"""The time-and-allocation verdict between two prebuilt test binaries (S1.1c-6 design note §2.4, §3.1).

    python3.15 -I -B gate/bench/compare.py --before <test binary> --after <test binary> --bench <regex> \\
        [--dir <package dir>] [--pairs 10] [--benchtime 1s] [--alloc-iters 20000] \\
        [--before-label <text>] [--after-label <text>]

Time: `pairs` ABBA pairs (before then after when the pair's index is even, after then before when odd) at
-test.cpu=1 -test.count=1, one ns/op sample per benchmark per run. Allocations: each binary twice at a fixed
-test.benchtime=<alloc-iters>x -test.benchmem (alloc-iters is a plain count; the x is added here); allocs/op compared
exactly, and a side whose own two runs disagree on it is unjudgeable; B/op compared exactly when each side's two runs
agree, else each side's range is printed and B/op is left out of that benchmark's verdict (scheduling-dependent
batching moves bytes, never counts; owner, 2026-11-17, S1.1c-6 lane R decision 2). The statistic is the exact two-sided
Mann-Whitney p conditional on the observed ties. A benchmark costs if its allocations differ, or if p < 0.05 and
median(after) > 1.02 x median(before). Exit 0 no cost, 1 costs, 2 an error; the last line says which.
"""

import argparse
import dataclasses
import math
import os
import shutil
import signal
import statistics
import sys
from collections.abc import Sequence
from fractions import Fraction
from typing import NoReturn

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate import term
from gate.bench import gorun

MAX_PER_SIDE = 20
ALPHA = Fraction(1, 20)
MARGIN = Fraction(102, 100)
RUN_TIMEOUT_S = 1800.0
LOOKUP_TIMEOUT_S = 60.0
STDERR_TAIL = 10


class Error(Exception):
    """Exit 2: the comparison cannot be judged."""


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise Error(f"usage: {message}")


def mann_whitney_p(before: Sequence[Fraction | int], after: Sequence[Fraction | int]) -> Fraction:
    """The exact two-sided p, conditional on the ties: a dynamic program over the pooled doubled midranks."""
    n_after, pooled = len(after), sorted([*before, *after])
    n = len(pooled)
    doubled: dict[Fraction | int, int] = {}
    i = 0
    while i < n:
        j = i
        while j + 1 < n and pooled[j + 1] == pooled[i]:
            j += 1
        doubled[pooled[i]] = (i + 1) + (j + 1)
        i = j + 1
    observed = sum(doubled[v] for v in after)
    mean = n_after * (n + 1)
    counts: list[dict[int, int]] = [{0: 1}] + [{} for _ in range(n_after)]
    for seen, value in enumerate(pooled):
        rank = doubled[value]
        for k in range(min(seen + 1, n_after), 0, -1):
            row = counts[k]
            for total, ways in counts[k - 1].items():
                row[total + rank] = row.get(total + rank, 0) + ways
    extreme = abs(observed - mean)
    hits = sum(ways for total, ways in counts[n_after].items() if abs(total - mean) >= extreme)
    return Fraction(hits, math.comb(n, n_after))


def _parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = _Parser(prog="compare.py", add_help=False)
    parser.add_argument("--before", required=True)
    parser.add_argument("--after", required=True)
    parser.add_argument("--bench", required=True)
    parser.add_argument("--dir", default=".")
    parser.add_argument("--pairs", default="10")
    parser.add_argument("--benchtime", default="1s")
    parser.add_argument("--alloc-iters", default="20000")
    parser.add_argument("--before-label", default="before")
    parser.add_argument("--after-label", default="after")
    args = parser.parse_args(list(argv))
    for name in ("pairs", "alloc_iters"):
        value = getattr(args, name)
        if not value.isdecimal() or int(value) < 1:
            raise Error(f"--{name.replace('_', '-')} takes a positive count, got {value!r}")
        setattr(args, name, int(value))
    if args.pairs > MAX_PER_SIDE:
        raise Error(f"--pairs {args.pairs} is more than {MAX_PER_SIDE} samples per side")
    for side in ("before", "after"):
        path = getattr(args, side)
        if not (os.path.isfile(path) and os.access(path, os.X_OK)):
            raise Error(f"--{side} {path} is not an executable file")
    return args


def _run(binary: str, flags: list[str], cwd: str) -> list[gorun.Bench]:
    result = gorun.run([binary, *flags], cwd=cwd, timeout_s=RUN_TIMEOUT_S)
    if result.timed_out:
        raise Error(f"{binary} ran past {RUN_TIMEOUT_S:.0f}s")
    if result.exit != 0:
        tail = " | ".join(result.stderr.splitlines()[-STDERR_TAIL:])
        raise Error(f"{binary} exited {result.exit}: {tail}")
    try:
        benches = gorun.parse_bench(result.stdout, cpu=1)
    except gorun.ParseError as e:
        raise Error(f"{binary}: {e}") from None
    names = [b.name for b in benches]
    if len(set(names)) != len(names):
        raise Error(f"{binary} reported a benchmark twice: {', '.join(sorted(names))}")
    return benches


def _check_names(names: set[str], benches: list[gorun.Bench], what: str) -> None:
    got = {b.name for b in benches}
    if not got:
        raise Error(f"no benchmark matched in {what}")
    if got != names:
        missing = sorted(names.symmetric_difference(got))
        raise Error(f"benchmark {', '.join(missing)} missing in some runs (first seen differently in {what})")


def _time_pass(args: argparse.Namespace) -> tuple[list[str], dict[str, dict[str, list[Fraction]]]]:
    flags = ["-test.run=^$", f"-test.bench={args.bench}", "-test.cpu=1", f"-test.benchtime={args.benchtime}"]
    flags.append("-test.count=1")
    samples: dict[str, dict[str, list[Fraction]]] = {"before": {}, "after": {}}
    names: set[str] | None = None
    for i in range(args.pairs):
        order = ("before", "after") if i % 2 == 0 else ("after", "before")
        for side in order:
            benches = _run(getattr(args, side), flags, args.dir)
            if names is None:
                names = {b.name for b in benches}
            _check_names(names, benches, f"{side}'s time run {i + 1}")
            for b in benches:
                samples[side].setdefault(b.name, []).append(b.ns_per_op)
    assert names is not None
    return sorted(names), samples


@dataclasses.dataclass(frozen=True)
class Allocs:
    """One side's allocation pass for one benchmark: allocs/op, which both runs must agree on, and B/op's range over
    the runs (lo == hi when they agree; otherwise the bytes are left out of the verdict, owner 2026-11-17)."""

    count: int
    bytes_lo: int
    bytes_hi: int

    @property
    def stable(self) -> bool:
        return self.bytes_lo == self.bytes_hi


def _alloc_pass(args: argparse.Namespace, names: list[str]) -> dict[str, dict[str, Allocs]]:
    flags = ["-test.run=^$", f"-test.bench={args.bench}", "-test.cpu=1", f"-test.benchtime={args.alloc_iters}x"]
    flags.append("-test.benchmem")
    allocs: dict[str, dict[str, Allocs]] = {}
    for side in ("before", "after"):
        runs = []
        for k in range(2):
            benches = _run(getattr(args, side), flags, args.dir)
            _check_names(set(names), benches, f"{side}'s allocation run {k + 1}")
            runs.append({b.name: (b.allocs_per_op, b.bytes_per_op) for b in benches})
        for name in names:
            first, second = runs[0][name], runs[1][name]
            if None in first or None in second:
                raise Error(f"{side}'s allocation run printed no allocs/op or B/op for {name}")
            if first[0] != second[0]:
                raise Error(
                    f"unjudgeable: {side}'s two allocation runs disagree on {name}: "
                    f"allocs/op {first[0]} and {second[0]}, B/op {first[1]} and {second[1]}"
                )
        allocs[side] = {
            name: Allocs(
                runs[0][name][0], min(runs[0][name][1], runs[1][name][1]), max(runs[0][name][1], runs[1][name][1])
            )
            for name in names
        }
    return allocs


def _alloc_verdict(before: Allocs, after: Allocs) -> tuple[bool, str]:
    """Whether the allocations differ, and their text: counts always exactly; bytes exactly when both sides' runs agree,
    else each side's range, left out of the verdict."""
    if before.stable and after.stable:
        differs = (before.count, before.bytes_lo) != (after.count, after.bytes_lo)
        return differs, f"allocs/op {before.count}/{after.count}, B/op {before.bytes_lo}/{after.bytes_lo}"
    text = (
        f"allocs/op {before.count}/{after.count}, B/op unstable (before {before.bytes_lo}-{before.bytes_hi}, "
        f"after {after.bytes_lo}-{after.bytes_hi}), left out"
    )
    return before.count != after.count, text


def _lookup(argv: list[str], cwd: str) -> str:
    """Best effort, for the record only: the command's output, or `unavailable: <first stderr line>`."""
    go = shutil.which("go")
    if go is None:
        return "unavailable: go not found on PATH"
    try:
        result = gorun.run([go, *argv], cwd=cwd, timeout_s=LOOKUP_TIMEOUT_S)
    except OSError as e:
        return f"unavailable: {e}"
    if result.exit != 0 or result.timed_out:
        first = (result.stderr.splitlines() or ["(no stderr)"])[0]
        return f"unavailable: {first}"
    return " ".join(result.stdout.split())


def _num(value: Fraction) -> str:
    return f"{float(value):.6g}"


def judge(args: argparse.Namespace) -> tuple[int, list[str]]:
    names, samples = _time_pass(args)
    for side in ("before", "after"):
        for name in names:
            if len(samples[side][name]) > MAX_PER_SIDE:
                raise Error(f"{name}: more than {MAX_PER_SIDE} samples on the {side} side")
    allocs = _alloc_pass(args, names)
    lines, costs = [], []
    for name in names:
        before, after = samples["before"][name], samples["after"][name]
        m_before, m_after = statistics.median(before), statistics.median(after)
        p = mann_whitney_p(before, after)
        alloc_differs, alloc_text = _alloc_verdict(allocs["before"][name], allocs["after"][name])
        slower = p < ALPHA and m_after > MARGIN * m_before
        verdict = "costs" if alloc_differs or slower else "no cost"
        if verdict == "costs":
            costs.append(name)
        ratio = m_after / m_before if m_before else Fraction(0)
        lines.append(
            f"{name}: median before {_num(m_before)} ns/op, after {_num(m_after)} ns/op, ratio {float(ratio):.4f}, "
            f"p {float(p):.4g}, {alloc_text}: {verdict}"
        )
    for name in names:
        for side in ("before", "after"):
            lines.append(f"samples {name} {side}: {' '.join(_num(v) for v in samples[side][name])}")
    for side in ("before", "after"):
        binary = getattr(args, side)
        lines.append(f"{side} label: {getattr(args, side + '_label')}")
        lines.append(f"{side} binary: {binary}")
        lines.append(f"go version {side}: {_lookup(['version', binary], args.dir)}")
        lines.append(f"go buildid {side}: {_lookup(['tool', 'buildid', binary], args.dir)}")
    if costs:
        lines.append(f"verdict: costs ({', '.join(costs)})")
        return 1, lines
    lines.append("verdict: no cost")
    return 0, lines


def main(argv: Sequence[str]) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    term.install()
    try:
        code, lines = judge(_parse_args(argv))
    except Error as e:
        code, lines = 2, [f"error: {e}"]
    except BaseException as e:  # every other failure, a stop included, is exit 2 with its reason
        print(f"error: internal error: {type(e).__name__}: {e}", flush=True)
        raise SystemExit(2) from e
    for line in lines:
        print(line)
    sys.stdout.flush()
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
