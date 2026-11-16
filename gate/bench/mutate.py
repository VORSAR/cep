"""One mutation, its one designated test, and a tracked file never written (S1.1c-6 design note §2.3, §3.3).

    python3.15 -I -B gate/bench/mutate.py --file <repo-relative go file> --find <exact text> --replace <text> \\
        --test <TopLevelTestName> [--timeout 120s] [--race] [--cpu <list>] [--repeat N --load]
    python3.15 -I -B gate/bench/mutate.py --batch <mutations.json> [--timeout 120s] [--race] [--cpu <list>]

Run from the repo's top level. Refused (exit 2) before any go command: a file not tracked or differing from HEAD, a
find text occurring other than exactly once, a test name that is not a top-level Test identifier, a malformed batch
(every entry is checked before the first build). The mutant is written outside the tree and built through
`go test -overlay`, so the tracked file is never opened for writing. Outcomes, judged in order: timeout (go test's
own -timeout backstop), red (the line `--- FAIL: <Test> (`), green (exit 0 and `--- PASS: <Test> (`), invalid
(anything else). Only red counts (owner, 2026-11-16, G1 decision 4). Single mode exits 0 on red, 1 otherwise;
batch mode 0 when every entry is red; --repeat N --load runs N times under CPU hogs and exits 0 when k >= 1 of N.
Exit 2 on a refusal or an internal error; the last line says which.
"""

import argparse
import dataclasses
import difflib
import json
import os
import re
import signal
import sys
import tempfile
from collections.abc import Sequence
from typing import NoReturn

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate import term
from gate.bench import gorun
from gate.runner import stress

WALL_EXTRA_S = 120.0
LOOKUP_TIMEOUT_S = 60.0
TAIL = 20
OUTCOMES = ("red", "green", "timeout", "invalid")
ENTRY_KEYS = frozenset({"file", "find", "replace", "test", "note"})
REQUIRED_KEYS = frozenset({"file", "find", "replace", "test"})
_DURATION = re.compile(r"(?:[0-9]+(?:\.[0-9]*)?(?:ns|us|µs|ms|s|m|h))+")
_UNIT_S = {"ns": 1e-9, "us": 1e-6, "µs": 1e-6, "ms": 1e-3, "s": 1.0, "m": 60.0, "h": 3600.0}


class Refused(Exception):
    """Exit 2 before any go command."""


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise Refused(f"usage: {message}")


@dataclasses.dataclass(frozen=True)
class Mutation:
    file: str
    find: str
    replace: str
    test: str
    note: str
    original: str
    mutant: str


def seconds(duration: str) -> float:
    """A Go duration (`120s`, `2m30s`, `500ms`) in seconds."""
    if not _DURATION.fullmatch(duration):
        raise Refused(f"--timeout {duration!r} is not a Go duration")
    total = sum(float(n) * _UNIT_S[u] for n, u in re.findall(r"([0-9.]+)(ns|us|µs|ms|s|m|h)", duration))
    if total <= 0:
        raise Refused(f"--timeout {duration!r} is not positive")
    return total


def _git(*argv: str) -> gorun.Result:
    return gorun.run(["git", *argv], cwd=os.getcwd(), timeout_s=LOOKUP_TIMEOUT_S)


def _occurrences(text: str, find: str) -> int:
    """Overlapping occurrences: `aa` occurs twice in `aaa`."""
    count, at = 0, text.find(find)
    while at != -1:
        count += 1
        at = text.find(find, at + 1)
    return count


def check(file: str, find: str, replace: str, test: str, note: str = "") -> Mutation:
    """Every refusal of one mutation, before any go command."""
    if not test.isidentifier() or not test.startswith("Test"):
        raise Refused(f"--test {test!r} is not a top-level Test identifier")
    if not file or os.path.isabs(file) or ".." in file.split("/"):
        raise Refused(f"--file {file!r} is not a repo-relative path")
    if not find:
        raise Refused("--find is empty")
    if _git("ls-files", "--error-unmatch", "--", file).exit != 0:
        raise Refused(f"{file} is not tracked")
    if _git("diff", "--quiet", "HEAD", "--", file).exit != 0:
        raise Refused(f"{file} differs from HEAD")
    with open(file, "rb") as f:
        original = f.read().decode("utf-8", errors="surrogateescape")
    count = _occurrences(original, find)
    if count != 1:
        raise Refused(f"--find occurs {count} times in {file}, not exactly once")
    return Mutation(file, find, replace, test, note, original, original.replace(find, replace, 1))


def read_batch(path: str) -> list[Mutation]:
    try:
        with open(path, "rb") as f:
            entries = json.loads(f.read().decode("utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as e:
        raise Refused(f"batch {path}: {e}") from None
    if not isinstance(entries, list) or not entries:
        raise Refused(f"batch {path}: not a non-empty list of entries")
    mutations = []
    for index, entry in enumerate(entries, 1):
        if not isinstance(entry, dict):
            raise Refused(f"batch entry {index}: not an object")
        keys = set(entry)
        if keys - ENTRY_KEYS or REQUIRED_KEYS - keys:
            extra, missing = sorted(keys - ENTRY_KEYS), sorted(REQUIRED_KEYS - keys)
            raise Refused(f"batch entry {index}: extra keys {extra}, missing keys {missing}")
        if not all(isinstance(value, str) for value in entry.values()):
            raise Refused(f"batch entry {index}: every value must be a string")
        try:
            mutations.append(
                check(entry["file"], entry["find"], entry["replace"], entry["test"], entry.get("note", ""))
            )
        except Refused as e:
            raise Refused(f"batch entry {index}: {e}") from None
    return mutations


def diff(m: Mutation) -> list[str]:
    lines = difflib.unified_diff(
        m.original.splitlines(keepends=True), m.mutant.splitlines(keepends=True), f"a/{m.file}", f"b/{m.file}"
    )
    return [line.rstrip("\n") for line in lines]


def judge(result: gorun.Result, test: str) -> str:
    text = f"{result.stdout}\n{result.stderr}"
    if "panic: test timed out after" in text:
        return "timeout"
    if re.search(rf"^--- FAIL: {re.escape(test)} \(", text, re.MULTILINE):
        return "red"
    if result.exit == 0 and not result.timed_out and re.search(rf"^--- PASS: {re.escape(test)} \(", text, re.MULTILINE):
        return "green"
    return "invalid"


def tail(result: gorun.Result) -> list[str]:
    lines = []
    for name, text in (("stderr", result.stderr), ("stdout", result.stdout)):
        kept = text.splitlines()[-TAIL:]
        if kept:
            lines += [f"{name} tail:", *(f"  {line}" for line in kept)]
    if result.timed_out:
        lines.append("killed by the wall clock")
    return lines


def run_one(m: Mutation, args: argparse.Namespace, scratch: str) -> tuple[str, gorun.Result]:
    """The mutant outside the tree, built through -overlay; the tracked file is only ever read."""
    top = os.getcwd()
    mutant = os.path.join(scratch, "mutant", m.file)
    os.makedirs(os.path.dirname(mutant), exist_ok=True)
    with open(mutant, "wb") as f:
        f.write(m.mutant.encode("utf-8", errors="surrogateescape"))
    overlay = os.path.join(scratch, "overlay.json")
    with open(overlay, "w", encoding="utf-8") as f:
        json.dump({"Replace": {os.path.join(top, m.file): mutant}}, f, sort_keys=True, ensure_ascii=False)
        f.write("\n")
    package = "./" + (os.path.dirname(m.file) or ".")
    argv = ["go", "test", f"-overlay={overlay}", "-count=1", "-v", "-run", f"^{m.test}$", "-timeout", args.timeout]
    if args.race:
        argv.append("-race")
    if args.cpu:
        argv += ["-cpu", args.cpu]
    argv.append(package)
    # PWD names the real directory, so go's working directory matches the overlay's key.
    env = {**os.environ, "PWD": top}
    result = gorun.run(argv, cwd=top, env=env, timeout_s=seconds(args.timeout) + WALL_EXTRA_S)
    return judge(result, m.test), result


def record() -> list[str]:
    head = _git("rev-parse", "HEAD")
    version = gorun.run(["go", "version"], cwd=os.getcwd(), timeout_s=LOOKUP_TIMEOUT_S)
    return [f"head: {head.stdout.strip()}", f"go: {version.stdout.strip() or version.stderr.strip()}"]


def _parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = _Parser(prog="gate/bench/mutate.py")
    parser.add_argument("--file")
    parser.add_argument("--find")
    parser.add_argument("--replace")
    parser.add_argument("--test")
    parser.add_argument("--batch")
    parser.add_argument("--timeout", default="120s")
    parser.add_argument("--race", action="store_true")
    parser.add_argument("--cpu")
    parser.add_argument("--repeat", type=int)
    parser.add_argument("--load", action="store_true")
    args = parser.parse_args(argv)
    single = (args.file, args.find, args.replace, args.test)
    if args.batch is not None:
        if any(value is not None for value in single) or args.repeat is not None or args.load:
            raise Refused("--batch takes no --file, --find, --replace, --test, --repeat or --load")
    elif any(value is None for value in single):
        raise Refused("--file, --find, --replace and --test are all required without --batch")
    if (args.repeat is None) != (not args.load):
        raise Refused("--repeat N and --load go together")
    if args.repeat is not None and args.repeat < 1:
        raise Refused("--repeat must be at least 1")
    seconds(args.timeout)
    return args


def _print(lines: Sequence[str]) -> None:
    for line in lines:
        print(line)
    sys.stdout.flush()


def single(m: Mutation, args: argparse.Namespace, scratch: str) -> tuple[int, list[str]]:
    _print([f"test: {m.test}", *diff(m), *record()])
    outcome, result = run_one(m, args, scratch)
    lines = tail(result) if outcome == "invalid" else []
    return (0 if outcome == "red" else 1), [*lines, f"outcome: {outcome}"]


def repeat(m: Mutation, args: argparse.Namespace, scratch: str) -> tuple[int, list[str]]:
    _print([f"test: {m.test}", *diff(m), *record()])
    caught = 0
    with stress.hogs(stress.hog_count(), sys.executable) as procs:
        _print([f"load: {len(procs)} hogs, pids {' '.join(str(p) for p in sorted(p.pid for p in procs))}"])
        for n in range(1, args.repeat + 1):
            outcome, _ = run_one(m, args, scratch)
            caught += outcome == "red"
            _print([f"run {n}/{args.repeat}: {outcome}"])
    return (0 if caught else 1), [f"caught {caught}/{args.repeat}"]


def batch(mutations: list[Mutation], args: argparse.Namespace, scratch: str) -> tuple[int, list[str]]:
    _print(record())
    counts = dict.fromkeys(OUTCOMES, 0)
    for index, m in enumerate(mutations, 1):
        _print([f"entry {index}", f"note: {m.note}", f"test: {m.test}", *diff(m)])
        with tempfile.TemporaryDirectory(dir=scratch) as own:
            outcome, result = run_one(m, args, own)
        counts[outcome] += 1
        _print([*(tail(result) if outcome == "invalid" else []), f"entry {index}: {outcome}"])
    summary = ", ".join(f"{name} {counts[name]}" for name in OUTCOMES)
    return (0 if counts["red"] == len(mutations) else 1), [f"counts: {summary}"]


def mutate(args: argparse.Namespace) -> tuple[int, list[str]]:
    if args.batch is not None:
        mutations = read_batch(args.batch)
    else:
        mutations = [check(args.file, args.find, args.replace, args.test)]
    with tempfile.TemporaryDirectory(prefix="mutate-") as scratch:
        if args.batch is not None:
            return batch(mutations, args, scratch)
        if args.repeat is not None:
            return repeat(mutations[0], args, scratch)
        return single(mutations[0], args, scratch)


def main(argv: Sequence[str]) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    term.install()
    try:
        code, lines = mutate(_parse_args(argv))
    except Refused as e:
        code, lines = 2, [f"error: refused: {e}"]
    except BaseException as e:  # every other failure, a stop included, is exit 2 with its reason
        print(f"error: internal error: {type(e).__name__}: {e}", flush=True)
        raise SystemExit(2) from e
    _print(lines)
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
