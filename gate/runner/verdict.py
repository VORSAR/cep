"""Verdict precedence (design note §4): the first match gives the reason, so one run has one story."""

from dataclasses import dataclass, field

from gate.runner.covered import Covered


@dataclass(frozen=True)
class Facts:
    """What happened to one row. No judgement lives here; `decide` is the only place reasons are composed."""

    cap_s: int
    min_wants: int = 0
    parity_failed: bool = False
    stopped: str = ""  # the reason unfinished rows carry: "interrupted", or the ceiling's reason
    timed_out: bool = False
    spawn_error: str = ""
    scope_error: str = ""
    left_running: bool = False
    returncode: int = 0  # Popen's convention: negative means killed by that signal
    first_failure: str = ""  # the row's own first failure line, if it printed one (step.first_failure)
    covered: Covered = field(default_factory=Covered)
    expected: int = 0


@dataclass(frozen=True)
class Verdict:
    ok: bool
    reason: str


def decide(f: Facts) -> Verdict:
    if f.parity_failed:
        return Verdict(False, "not run: parity failed")
    if f.stopped:
        return Verdict(False, f.stopped)
    if f.timed_out:
        return Verdict(False, f"timeout after {f.cap_s}s")
    if f.spawn_error:
        return Verdict(False, f"could not start: {f.spawn_error}")
    if f.scope_error:
        return Verdict(False, f"scope error: {f.scope_error}")
    if f.left_running:
        return Verdict(False, "left running processes")
    if f.returncode < 0:
        return Verdict(False, f"killed by signal {-f.returncode}")
    if f.returncode > 0:
        # owner, 2026-11-03 (review BA3): a red row's reason shows its first failure line, not a bare exit code.
        return Verdict(False, f"exit {f.returncode}: {f.first_failure}" if f.first_failure else f"exit {f.returncode}")
    if f.covered.problem:
        return Verdict(False, f.covered.problem)
    if f.covered.count != f.expected:
        return Verdict(False, f"covered {f.covered.count} ≠ expected {f.expected}")
    if f.covered.count < f.min_wants:
        return Verdict(False, f"covered {f.covered.count} < min_wants {f.min_wants}")
    return Verdict(True, f"covered {f.covered.count}")
