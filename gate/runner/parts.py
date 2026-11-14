"""The part-level check on size exclusions (S1.1c-5 design §3.5; owner 2026-11-14, S1.1c-5's design after the
reviews, decision 1): run by the aging row over tools/allowlists/smells.json, its known issues and the merge base.

Stub (W0b): lane B writes the check.
"""

from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass

from gate.runner import allowlists, sprintid

LAST_PART = "S1.1c-8"  # decision 1 of the plan check: never past S1.1c-8
LANDING = "S1.1c-5"  # the sprint whose own branch may add size entries without a decision
SMELLS = "tools/allowlists/smells.json"
KNOWN_ISSUES = "gate/known-issues.json"


@dataclass(frozen=True)
class Base:
    """The state at the merge base, or why there is none."""

    commit: str  # "" when git.merge_base() returned ""
    entries: Sequence[allowlists.Entry] | None  # smells.json at the base; None when absent there
    issues: Mapping[str, Mapping[str, object]] | None  # known-issues.json at the base; None when absent there


def read_base(cwd: str | None = None) -> Base:
    """git.merge_base(cwd); "" → Base("", None, None). Else `git show <commit>:<path>` for smells.json and
    known-issues.json; absent at the base → None for that file; a parse error → ValueError naming "<commit>:<path>"."""
    raise NotImplementedError("S1.1c-5 lane B: parts.read_base")


def forwards(
    entries: Sequence[allowlists.Entry],
    issues: Mapping[str, Mapping[str, object]],
    order: sprintid.Order,
    marker: str,
    tier: str,
    base: Base,
    decided: Callable[[str, str], bool],
) -> tuple[list[str], list[str]]:
    """(faults, lines): faults print "failed: aging: <text>", lines print plain (design §3.5 rules 1-6)."""
    raise NotImplementedError("S1.1c-5 lane B: parts.forwards")
