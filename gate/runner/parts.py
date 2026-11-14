"""The part-level check on size exclusions (S1.1c-5 design §3.5; owner 2026-11-14, S1.1c-5's design after the
reviews, decision 1): run by the aging row over tools/allowlists/smells.json, its known issues and the merge base.

Every size exclusion is TEMP and names a known issue whose `forward` is a printed part, never past S1.1c-8; when the
marker reaches that part the exclusion is red at T4 and nightly and a countdown line at T3. A forward only the owner
moves: per exclusion target, an effective forward later than at the merge base needs a new decision naming the issue,
and a target absent at the base is red after S1.1c-5's own branch. With no merge base the move check cannot run, red
at T4 and nightly. `forwards` is pure (the order, the marker, the tier, the base and the decision reader injected);
`read_base` reads git.
"""

from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass

from gate.runner import allowlists, git, knownissues, sprintid

LAST_PART = "S1.1c-8"  # decision 1 of the plan check: never past S1.1c-8
LANDING = "S1.1c-5"  # the sprint whose own branch may add size entries without a decision
SMELLS = "tools/allowlists/smells.json"
KNOWN_ISSUES = "gate/known-issues.json"
RULE = "smells"


@dataclass(frozen=True)
class Base:
    """The state at the merge base, or why there is none."""

    commit: str  # "" when git.merge_base() returned ""
    entries: Sequence[allowlists.Entry] | None  # smells.json at the base; None when absent there
    issues: Mapping[str, Mapping[str, object]] | None  # known-issues.json at the base; None when absent there


def _at(commit: str, path: str, cwd: str | None) -> str | None:
    """The file's text at the commit, None when git cannot show it there (absent at the base)."""
    code, text = git.git("show", f"{commit}:{path}", cwd=cwd)
    return text if code == 0 else None


def _unprefixed(error: ValueError, where: str) -> str:
    text = str(error)
    return text.removeprefix(f"{where}: ")


def read_base(cwd: str | None = None) -> Base:
    """git.merge_base(cwd); "" → Base("", None, None). Else `git show <commit>:<path>` for smells.json and
    known-issues.json; absent at the base → None for that file; a parse error → ValueError naming "<commit>:<path>"."""
    commit = git.merge_base(cwd)
    if not commit:
        return Base("", None, None)
    entries: tuple[allowlists.Entry, ...] | None = None
    issues: dict[str, Mapping[str, object]] | None = None
    text = _at(commit, SMELLS, cwd)
    if text is not None:
        try:
            entries = allowlists.load(text, RULE).entries
        except ValueError as e:
            raise ValueError(f"{commit}:{SMELLS}: {_unprefixed(e, SMELLS)}") from None
    text = _at(commit, KNOWN_ISSUES, cwd)
    if text is not None:
        try:
            issues = {str(item["id"]): item for item in knownissues._entries(text)}
        except ValueError as e:
            raise ValueError(f"{commit}:{KNOWN_ISSUES}: {e}") from None
    return Base(commit, entries, issues)


def _size(entry: allowlists.Entry) -> tuple[str, str] | None:
    """(package dir, rule) of a TEMP entry in the one target form "<package dir> <rule>"; the form's other faults are
    the smells row's (design §3.1)."""
    fields = entry.target.split(" ")
    if entry.kind != "TEMP" or len(fields) != 2 or not all(fields):
        return None
    return fields[0], fields[1]


def _forward(issues: Mapping[str, Mapping[str, object]] | None, id: str) -> tuple[str, str] | None:
    """(forward, decision) of the issue, None when the issue is absent or has no forward."""
    item = (issues or {}).get(id)
    if item is None:
        return None
    forward, decision = item.get("forward", ""), item.get("decision", "")
    if type(forward) is not str or not forward:
        return None
    return forward, decision if type(decision) is str else ""


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
    faults: list[str] = []
    lines: list[str] = []
    red = tier in allowlists.RED_EXPIRED
    judged: list[tuple[allowlists.Entry, str, str]] = []  # (entry, forward, decision) past rules 1-3
    size = sorted((e for e in entries if _size(e) is not None), key=lambda e: e.target)
    for entry in size:
        where = f"aging: smells.json: {entry.target}"
        package, rule = _size(entry) or ("", "")
        now = _forward(issues, entry.issue)
        if now is None:
            faults.append(f"{where}: issue {entry.issue} has no forward part")
            continue
        forward, decision = now
        try:
            order.reached_part(forward, forward)
        except ValueError:
            faults.append(f"{where}: forward {forward} is not a printed part of {sprintid.SPRINTS}")
            continue
        if order.index(forward) > order.index(LAST_PART):
            faults.append(f"{where}: forward {forward} is past {LAST_PART} (decision 1)")
            continue
        try:
            reached = order.reached_part(forward, marker)
        except ValueError as e:
            faults.append(f"{where}: the marker cannot be placed: {e}")
            continue
        if reached:
            text = (
                f"{where}: the marker {marker} has reached its part {forward}: clean {package} for {rule},"
                " or the owner moves the forward"
            )
            (faults if red else lines).append(text)
        judged.append((entry, forward, decision))
    if not size:
        return faults, lines
    if not base.commit:  # rule 6
        if red:
            faults.append(
                "aging: smells.json: no merge base with origin/main or main: forward moves not checked (decision 1)"
            )
        else:
            lines.append("aging: no merge base; forward moves not checked")
        return faults, lines
    faults.extend(_moves(judged, order, marker, base, decided))
    return faults, lines


def _moves(
    judged: Sequence[tuple[allowlists.Entry, str, str]],
    order: sprintid.Order,
    marker: str,
    base: Base,
    decided: Callable[[str, str], bool],
) -> list[str]:
    """Rule 5, per exclusion target: the effective forward now against the one at the base."""
    then = {e.target: e.issue for e in base.entries or ()} if base.issues is not None else {}
    faults: list[str] = []
    for entry, forward, decision in judged:
        where = f"aging: smells.json: {entry.target}"
        base_id = then.get(entry.target)
        old = _forward(base.issues, base_id) if base_id is not None else None
        try:
            moved = old is not None and order.index(forward) > order.index(old[0])
        except ValueError:
            old = None
        if old is None:
            if marker != LANDING:
                faults.append(f"{where}: a new size exclusion after {LANDING}: clean the package instead (decision 1)")
            continue
        if moved and not (decided(entry.issue, decision) and decision != old[1]):
            faults.append(
                f"{where}: forward moved {old[0]} → {forward} (issue {base_id} → {entry.issue}) without a decision"
                f" that names {entry.issue} (decision 1: only the owner moves a forward)"
            )
    return faults
