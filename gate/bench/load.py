"""A go test run under CPU hogs, and its per-test pass counts (S1.1c-6 design note §2.3, §3.4).

    python3.15 -I -B gate/bench/load.py [--hogs <n>] [--coverprofile <file>] [--wall <seconds>] -- <go test args>

Starts the hogs (default gate/runner/stress.py's hog_count()), runs `go test -json [-coverprofile=<file>] <args>`
through gorun with a wall-clock limit (default 3600 s), and stops the hogs whatever happened, a SIGTERM included.
For each top-level test it prints `<Test>: k/N under load (hogs <n>)`, k its pass actions and N its pass and fail
actions. Exit 0 when every test is N/N with N >= 1 and go test exited 0; 1 when a test failed or go test exited
non-zero; 2 when no test ran, the wall clock killed the run, or an internal error. The last line says which.
"""

import argparse
import collections
import json
import os
import signal
import sys
from collections.abc import Sequence
from typing import NoReturn

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate import term
from gate.bench import gorun
from gate.runner import stress

WALL_S = 3600.0
TAIL = 20


class Error(Exception):
    """Exit 2: the run cannot be judged."""


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise Error(f"usage: {message}")


def tally(text: str) -> dict[str, tuple[int, int]]:
    """Per top-level test (a Test with no `/`): its pass and fail actions. Lines that are not events are skipped."""
    passes: collections.Counter[str] = collections.Counter()
    fails: collections.Counter[str] = collections.Counter()
    seen: set[str] = set()
    for line in text.splitlines():
        try:
            record = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(record, dict):
            continue
        test, action = record.get("Test"), record.get("Action")
        if not isinstance(test, str) or "/" in test or action not in ("pass", "fail", "skip"):
            continue
        seen.add(test)
        passes[test] += action == "pass"
        fails[test] += action == "fail"
    return {test: (passes[test], fails[test]) for test in sorted(seen)}


def _parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = _Parser(prog="gate/bench/load.py")
    parser.add_argument("--hogs", type=int)
    parser.add_argument("--coverprofile")
    parser.add_argument("--wall", type=float, default=WALL_S)
    parser.add_argument("go_args", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    if args.go_args[:1] == ["--"]:
        args.go_args = args.go_args[1:]
    if not args.go_args:
        raise Error("no go test arguments after --")
    if args.hogs is not None and args.hogs < 0:
        raise Error("--hogs must not be negative")
    if args.wall <= 0:
        raise Error("--wall must be positive")
    return args


def _print(lines: Sequence[str]) -> None:
    for line in lines:
        print(line)
    sys.stdout.flush()


def load(args: argparse.Namespace) -> tuple[int, list[str]]:
    count = stress.hog_count() if args.hogs is None else args.hogs
    argv = ["go", "test", "-json"]
    if args.coverprofile:
        argv.append(f"-coverprofile={args.coverprofile}")
    argv += args.go_args
    with stress.hogs(count, sys.executable) as procs:
        _print([f"hogs: {count}, pids {' '.join(str(p) for p in sorted(p.pid for p in procs))}"])
        result = gorun.run(argv, cwd=os.getcwd(), timeout_s=args.wall)
    counts = tally(result.stdout)
    lines = [f"{test}: {k}/{k + f} under load (hogs {count})" for test, (k, f) in counts.items()]
    if args.coverprofile:
        lines.append(f"coverprofile: {args.coverprofile}")
    if result.timed_out:
        return 2, [*lines, f"error: go test killed by the wall clock after {args.wall:g}s"]
    if not any(k + f for k, f in counts.values()):
        tail = [f"  {line}" for line in result.stderr.splitlines()[-TAIL:]]
        return 2, [*lines, *tail, f"error: no test ran (go test exit {result.exit})"]
    below = sorted(test for test, (k, f) in counts.items() if f or not k)
    if below or result.exit != 0:
        tail = [f"  {line}" for line in result.stderr.splitlines()[-TAIL:]]
        return 1, [*lines, *tail, f"verdict: below N/N: {', '.join(below) or 'none'}; go test exit {result.exit}"]
    return 0, [*lines, f"verdict: every test N/N under load ({len(counts)} tests)"]


def main(argv: Sequence[str]) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    term.install()
    try:
        code, lines = load(_parse_args(argv))
    except Error as e:
        code, lines = 2, [f"error: {e}"]
    except BaseException as e:  # every other failure, a stop included, is exit 2 with its reason
        print(f"error: internal error: {type(e).__name__}: {e}", flush=True)
        raise SystemExit(2) from e
    _print(lines)
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
