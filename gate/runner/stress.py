"""The stress harness (design note §9; QA DQ18): many runs of test modules, each with its own hash seed and order.

    python3.15 -I -B -X utf8 gate/runner/stress.py [--runs 50] [--parallel N] [--no-hogs] [--out DIR] <module>...

Each run is a child process WITHOUT -I or -E, since either makes CPython ignore PYTHONHASHSEED, in a scrubbed
environment carrying the seed, with `-s -B -P -X dev -W error`. It runs every test of every module in one process,
in an order shuffled from the same seed, so one test can break another across modules. A CPU hog per half-core
loads the machine meanwhile. The harness prints per-test pass counts; below runs/runs it exits 1.
"""

import argparse
import collections
import concurrent.futures
import contextlib
import gc
import json
import os
import random
import signal
import subprocess
import sys
import tempfile
import threading
import unittest
from collections.abc import Callable, Iterator, Mapping, Sequence

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate import term

RUNS = 50
SCRUBBED_KEEP = (
    "PATH",
    "HOME",
    "TMPDIR",
    "USER",
    "LOGNAME",
    "VORSAR_CEP_TOOLS_CACHE",
    "GOCACHE",
    "GOMODCACHE",
    "GOPATH",
)
HOG = "while True:\n    pass\n"
CHILD_FLAGS = ("-s", "-B", "-P", "-X", "dev", "-W", "error")
PASS = "pass"
FAIL = "fail"


def scrubbed(base: Mapping[str, str], seed: int) -> dict[str, str]:
    """Only what the tests need to find the tools, and the seed. Nothing else reaches the child."""
    env = {name: base[name] for name in SCRUBBED_KEEP if name in base}
    env["PYTHONHASHSEED"] = str(seed)
    return env


def child_argv(python: str, script: str) -> list[str]:
    return [python, *CHILD_FLAGS, script, "--child"]


def hog_count(cpu_count: Callable[[], int | None] = os.process_cpu_count) -> int:
    return 2 * (cpu_count() or 1)


@contextlib.contextmanager
def hogs(count: int, python: str) -> Iterator[list[subprocess.Popen[bytes]]]:
    """Busy processes for the length of the block, each its own group, all gone after it."""
    procs: list[subprocess.Popen[bytes]] = []
    try:
        for _ in range(count):
            procs.append(term.popen([python, "-I", "-c", HOG], stdin=subprocess.DEVNULL))
        yield procs
    finally:
        for proc in procs:
            os.killpg(proc.pid, signal.SIGKILL)
        for proc in procs:
            proc.wait()


class _Outcomes(unittest.TestResult):
    """One outcome per test. Only a clean pass is a pass: a skip proves nothing, and a leak is a defect."""

    def __init__(self) -> None:
        super().__init__()
        self.outcomes: dict[str, str] = {}
        self.why: dict[str, str] = {}
        self.current = ""

    def startTest(self, test: unittest.TestCase) -> None:
        super().startTest(test)
        self.current = test.id()
        self.outcomes.setdefault(self.current, PASS)

    def stopTest(self, test: unittest.TestCase) -> None:
        gc.collect()  # a leak's destructor runs here, so it is charged to the test that leaked
        super().stopTest(test)

    def _bad(self, test: unittest.TestCase, outcome: str) -> None:
        self.outcomes[test.id()] = outcome

    def _failed(self, test: unittest.TestCase, err) -> None:
        self._bad(test, FAIL)
        self.why.setdefault(test.id(), self._exc_info_to_string(err, test))

    def addError(self, test, err) -> None:
        super().addError(test, err)
        self._failed(test, err)

    def addFailure(self, test, err) -> None:
        super().addFailure(test, err)
        self._failed(test, err)

    def addSkip(self, test, reason) -> None:
        super().addSkip(test, reason)
        self._bad(test, "skip")

    def addExpectedFailure(self, test, err) -> None:
        super().addExpectedFailure(test, err)
        self._bad(test, "expected failure")

    def addUnexpectedSuccess(self, test) -> None:
        super().addUnexpectedSuccess(test)
        self._bad(test, "unexpected success")

    def addSubTest(self, test, subtest, err) -> None:
        super().addSubTest(test, subtest, err)
        if err is not None:
            self._failed(test, err)

    def on_unraisable(self, info: object) -> None:
        test = self.current or "<between tests>"
        self.outcomes[test] = "leak"
        self.why.setdefault(test, f"leak: {info.exc_type.__name__}: {info.exc_value}")


def _flatten(suite: unittest.TestSuite) -> Iterator[unittest.TestCase]:
    for item in suite:
        if isinstance(item, unittest.TestSuite):
            yield from _flatten(item)
        else:
            yield item


def child(argv: list[str]) -> int:
    """One run: every test, in one process, in this seed's order; the outcomes go to a file."""
    seed, out_path, root, *modules = argv
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    sys.path.insert(0, root)
    loader = unittest.TestLoader()
    tests = sorted(
        (t for name in modules for t in _flatten(loader.loadTestsFromName(name))),
        key=lambda test: test.id(),
    )
    random.Random(int(seed)).shuffle(tests)
    result = _Outcomes()
    sys.unraisablehook = result.on_unraisable
    unittest.TestSuite(tests).run(result)
    gc.collect()
    report = {
        "seed": int(seed),
        "hash_probe": hash("abc"),
        "order": [test.id() for test in tests],
        "outcomes": result.outcomes,
        "failures": result.why,
        "flags": {
            "isolated": sys.flags.isolated,
            "ignore_environment": sys.flags.ignore_environment,
            "no_user_site": sys.flags.no_user_site,
            "safe_path": sys.flags.safe_path,
            "dont_write_bytecode": sys.flags.dont_write_bytecode,
            "dev_mode": sys.flags.dev_mode,
            "warn_error": "error" in sys.warnoptions,
        },
    }
    with open(f"{out_path}.tmp", "w", encoding="utf-8") as f:
        json.dump(report, f, sort_keys=True, ensure_ascii=False, separators=(",", ":"))
    os.rename(f"{out_path}.tmp", out_path)
    return 0


# Set by the first SIGTERM (review BA1): a run the pool dequeues after the stop must not start, even though the
# stop's shutdown cancels only the runs still queued.
stopping = threading.Event()


def _one_run(seed: int, script: str, root: str, modules: Sequence[str], out_dir: str) -> dict | None:
    if stopping.is_set():
        return None
    path = os.path.join(out_dir, f"run-{seed}.json")
    argv = [*child_argv(sys.executable, script), str(seed), path, root, *modules]
    with open(f"{path}.stdout", "wb") as out, open(f"{path}.stderr", "wb") as err:
        subprocess.run(
            argv, env=scrubbed(os.environ, seed), cwd=out_dir, stdin=subprocess.DEVNULL, stdout=out, stderr=err
        )
    if not os.path.exists(path):
        return None
    with open(path, "rb") as f:
        return json.loads(f.read())


def stress(modules: Sequence[str], *, runs: int, parallel: int, out_dir: str) -> int:
    script = os.path.abspath(__file__)
    root = os.path.dirname(os.path.dirname(os.path.dirname(script)))
    # Runs are submitted by this thread only, `parallel` at a time, as earlier ones finish: a pre-filled queue let a
    # worker start the next run on its own in the moment between a stop's arrival and this thread acting on it
    # (seen under T4's load, 2026-11-04). Stopped, the running runs finish and nothing else is ever submitted.
    pool = concurrent.futures.ThreadPoolExecutor(max_workers=parallel)
    reports: list[dict | None] = [None] * runs
    try:
        seeds = iter(range(runs))
        in_flight: dict[concurrent.futures.Future, int] = {}
        while True:
            for seed in seeds:
                in_flight[pool.submit(_one_run, seed, script, root, modules, out_dir)] = seed
                if len(in_flight) >= parallel:
                    break
            if not in_flight:
                break
            done, _ = concurrent.futures.wait(in_flight, return_when=concurrent.futures.FIRST_COMPLETED)
            for future in done:
                reports[in_flight.pop(future)] = future.result()
    finally:
        pool.shutdown(wait=True, cancel_futures=True)

    passes: collections.Counter[str] = collections.Counter()
    why: dict[str, tuple[int, str]] = {}
    seen: set[str] = set()
    died = False
    for seed, report in enumerate(reports):
        if report is None:
            died = True
            print(f"run {seed}: died: see {os.path.join(out_dir, f'run-{seed}.json.stderr')}")
            continue
        print(f"run {seed}: seed {report['seed']}")
        for test, outcome in report["outcomes"].items():
            seen.add(test)
            passes[test] += outcome == PASS
            if outcome != PASS and test not in why:
                text = report["failures"].get(test, outcome).strip().splitlines()
                why[test] = (seed, text[-1] if text else outcome)
    if not seen:
        print("stress: no test ran: zero is a failure")
        return 1
    for test in sorted(seen):
        print(f"{passes[test]}/{runs} {test}")
    for test in sorted(why):
        seed, line = why[test]
        print(f"why {test}: run {seed}: {line} (the traceback is in run-{seed}.json)")
    below = sum(1 for test in seen if passes[test] < runs)
    print(f"stress: {len(seen)} tests, {below} below {runs}/{runs}")
    return 1 if below or died else 0


def main(argv: list[str]) -> int:
    if argv[:1] == ["--child"]:
        return child(argv[1:])
    term.install(
        stopping.set
    )  # review BA1: a TERM ends the run through `hogs`'s finally, never leaving the hogs behind
    parser = argparse.ArgumentParser(prog="gate/runner/stress.py")
    parser.add_argument("--runs", type=int, default=RUNS)
    parser.add_argument("--parallel", type=int, default=1)
    parser.add_argument("--no-hogs", action="store_true")
    parser.add_argument("--out")
    parser.add_argument("modules", nargs="+")
    args = parser.parse_args(argv)
    with tempfile.TemporaryDirectory(prefix="stress-") as scratch:
        out_dir = args.out or scratch
        os.makedirs(out_dir, exist_ok=True)
        with hogs(0 if args.no_hogs else hog_count(), sys.executable):
            return stress(args.modules, runs=args.runs, parallel=args.parallel, out_dir=out_dir)


if __name__ == "__main__":
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    sys.exit(main(sys.argv[1:]))
