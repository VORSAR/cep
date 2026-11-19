"""Run a go command in a session of its own, and parse `go test -bench` lines (S1.1c-6 design note §3).

The child starts in a new session under term.critical(), so a first TERM never lands between its start and our
hold on it. Its output goes to files, never pipes; stdin is /dev/null. On the wall-clock limit the whole group gets
TERM, a grace, then KILL, and is reaped; a group left behind by a leader that exited is killed too. A TERM or an
interrupt of the caller kills the child's group before the stop goes on. The group's KILL is sent from a `finally`
with SIGINT and SIGTERM held, so neither a stop after the leader exits nor a second interrupt in the grace leaves
the group running; a stop held meanwhile is raised once the KILL is sent (owner, 2026-11-19, BK-L1, BK-L2).
"""

import contextlib
import dataclasses
import os
import re
import signal
import subprocess
import tempfile
from collections.abc import Mapping, Sequence
from fractions import Fraction

from gate import term

GRACE_S = 5.0


@dataclasses.dataclass(frozen=True)
class Result:
    exit: int
    stdout: str
    stderr: str
    timed_out: bool


class ParseError(Exception):
    """A Benchmark line with a numeric field that is not a well-formed result line."""


@dataclasses.dataclass(frozen=True)
class Bench:
    name: str
    iterations: int
    ns_per_op: Fraction
    bytes_per_op: int | None
    allocs_per_op: int | None


def _killpg(pgid: int, sig: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pgid, sig)


def _kill_group(pgid: int) -> None:
    """KILL to the group with SIGINT and SIGTERM held, so neither cuts it short; the caller's mask is restored after,
    and a stop held meanwhile is raised then."""
    held = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGINT, signal.SIGTERM})
    try:
        _killpg(pgid, signal.SIGKILL)
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, held)


def _stop(proc: subprocess.Popen[bytes], grace_s: float) -> None:
    """TERM to the group, the grace for the leader, then KILL to the group, from a finally, so a second interrupt in
    the grace cannot skip it; the leader is reaped."""
    try:
        _killpg(proc.pid, signal.SIGTERM)
        with contextlib.suppress(subprocess.TimeoutExpired):
            proc.wait(timeout=grace_s)
    finally:
        _kill_group(proc.pid)
        proc.wait()


def _read(path: str) -> str:
    with open(path, "rb") as f:
        return f.read().decode("utf-8", errors="replace")


def run(
    argv: Sequence[str], *, cwd: str, env: Mapping[str, str] | None = None, timeout_s: float, grace_s: float = GRACE_S
) -> Result:
    """Run argv in its own session with a wall-clock limit; the whole group is gone when this returns."""
    with tempfile.TemporaryDirectory(prefix="gorun-") as tmp:
        out_path, err_path = os.path.join(tmp, "stdout"), os.path.join(tmp, "stderr")
        with open(out_path, "wb") as out, open(err_path, "wb") as err:
            proc = None
            timed_out = False
            try:
                try:
                    with term.critical():  # a stop waits until proc is held, then lands in this try
                        proc = term.popen(
                            list(argv),
                            cwd=cwd,
                            env=None if env is None else dict(env),
                            stdin=subprocess.DEVNULL,
                            stdout=out,
                            stderr=err,
                        )
                    proc.wait(timeout=timeout_s)
                except subprocess.TimeoutExpired:
                    timed_out = True
                    _stop(proc, grace_s)
            except BaseException:
                if proc is not None:
                    _stop(proc, grace_s)
                raise
            finally:  # a grandchild left in the group by a leader that exited goes too, whatever ended the try
                if proc is not None:
                    _kill_group(proc.pid)
        return Result(exit=proc.returncode, stdout=_read(out_path), stderr=_read(err_path), timed_out=timed_out)


_NUMBER = re.compile(r"[0-9]+(?:\.[0-9]+)?")
_INT = re.compile(r"[0-9]+")
_SUFFIX = re.compile(r"-[0-9]+$")


def _bench(fields: list[str], cpu: int | None) -> Bench:
    name = fields[0]
    if cpu is None:
        name = _SUFFIX.sub("", name)
    elif cpu != 1:
        name = name.removesuffix(f"-{cpu}")
    if not _INT.fullmatch(fields[1]):
        raise ParseError("iterations are not an integer")
    rest = fields[2:]
    if len(rest) % 2:
        raise ParseError("a value without a unit")
    metrics: dict[str, str] = {}
    for value, unit in zip(rest[::2], rest[1::2], strict=True):
        if not _NUMBER.fullmatch(value):
            raise ParseError(f"{value!r} is not a number")
        metrics[unit] = value
    if "ns/op" not in metrics:
        raise ParseError("no ns/op")
    counts: list[int | None] = []
    for unit in ("B/op", "allocs/op"):
        value = metrics.get(unit)
        if value is not None and not _INT.fullmatch(value):
            raise ParseError(f"{unit} {value!r} is not an integer")
        counts.append(None if value is None else int(value))
    return Bench(name, int(fields[1]), Fraction(metrics["ns/op"]), counts[0], counts[1])


def parse_bench(text: str, *, cpu: int | None = None) -> list[Bench]:
    """Every result line of `go test -bench` output, in order.

    The -N GOMAXPROCS suffix is stripped from a name: any -N when `cpu` is None, exactly -<cpu> otherwise, and none
    at cpu 1 (Go omits it there, so a sub-benchmark's own trailing -4 stays). Custom metrics are ignored.
    """
    benches = []
    for line in text.splitlines():
        fields = line.split()
        if not fields or not fields[0].startswith("Benchmark") or len(fields) < 2:
            continue
        if not any(_NUMBER.fullmatch(f) for f in fields[1:]):
            continue
        try:
            benches.append(_bench(fields, cpu))
        except ParseError as e:
            raise ParseError(f"unparseable benchmark line {line!r}: {e}") from None
    return benches
