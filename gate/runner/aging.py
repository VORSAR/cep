"""Aging over gate/known-issues.json (S1.1c-3 design §3.2; owner 2026-11-12, S1.1c-2's plan check, decisions 5 and 6
as annotated; S1.1c-3's plan check, decisions 8 and 9 and confirm (c)).

Every known issue ages. Three rules, one line per fault, each naming the entry's id:

1. More than two closed rows of the printed order follow `first_seen`'s position (`Order.index`: a printed part-row
   such as S1.1a-1 is a row of its own), and no decision names the id: `decision` is empty, does not resolve, or
   resolves to a section that does not name it. A forward does not stop this clock (§7 W0-8).
2. The entry is still listed, so still open, and `forward` is a row that has closed, or a part the marker has passed
   (`Order.passed`, decision 8). A part-less marker in the part's own row cannot say, and is named (§7 W0-3).
3. `correctness` is true and `opened` is more than 30 days before `today` (ground rule 10; decision 9).

Pure: the caller reads the files, the day (`rows.today()`, UTC) and passes the tracked-Markdown reader `anchors`
takes. What aging cannot place (a sprint outside the order, an unreadable day) is a named line, never a crash.
"""

from collections.abc import Callable
from datetime import date

from gate.runner import anchors, knownissues, sprintid

CLOSED_ROWS_ALLOWED = 2
CORRECTNESS_DAYS = 30

Reader = Callable[[str], str | None]


def _outside(sprint: str) -> str:
    return f"{sprint}, which is not a row or a part of a row of {sprintid.SPRINTS}"


def decided(id: str, decision: str, read: Reader) -> bool:
    """Whether the decision resolves to a heading of tracked Markdown whose section names the id."""
    if not decision or anchors.resolves(decision, read):
        return False
    path, anchor = anchors.split_ref(decision)
    return knownissues.names(knownissues.section(read(path) or "", anchor), id)


def _age(id: str, first_seen: str, decision: str, order: sprintid.Order, read: Reader) -> list[str]:
    """Rule 1."""
    try:
        start = order.index(first_seen)
    except ValueError:
        return [f"aging: {id} first seen {_outside(first_seen)}"]
    since = sum(1 for row in order.rows[start + 1 :] if row.closed)
    if since > CLOSED_ROWS_ALLOWED and not decided(id, decision, read):
        return [f"aging: {id} first seen {first_seen} — {since} closed rows since, and no decision names it"]
    return []


def _forward(id: str, forward: str, order: sprintid.Order, marker: str) -> list[str]:
    """Rule 2: a row by its printed id, else a part of a row."""
    if not forward:
        return []
    rows = {row.id: row for row in order.rows}
    if forward in rows:
        return [f"aging: {id} was forwarded to {forward}; it has closed; still open"] if rows[forward].closed else []
    try:
        at_part = order.index(forward)
    except ValueError:
        return [f"aging: {id} was forwarded to {_outside(forward)}"]
    try:
        at_marker = order.index(marker)
        passed = order.passed(forward, marker)
    except ValueError as e:
        return [f"aging: {id} was forwarded to {forward}; the marker cannot be placed: {e}"]
    if passed:
        return [f"aging: {id} was forwarded to {forward}; the marker is at {marker}; still open"]
    if at_marker == at_part and sprintid.parse(marker).n == 0:
        return [f"aging: {id} was forwarded to {forward}; the marker names no part"]
    return []


def _correctness(id: str, correctness: object, opened: object, today: date) -> list[str]:
    """Rule 3."""
    if correctness is not True:
        return []
    day = knownissues.day(opened) if type(opened) is str else None
    if day is None:
        return [f"aging: {id} is a correctness issue whose opened {opened!r} is not a day YYYY-MM-DD"]
    days = (today - day).days
    if days > CORRECTNESS_DAYS:
        return [
            f"aging: {id} is a correctness issue open {days} days (opened {opened}); "
            f"ground rule 10 allows {CORRECTNESS_DAYS}"
        ]
    return []


def aged(
    entries: list[dict[str, object]],
    order: sprintid.Order,
    marker: str,
    *,
    today: date,
    read: Reader,
) -> list[str]:
    """Every known issue past its clock, one line per fault naming its id, sorted; [] when none is."""
    lines: list[str] = []
    for e in entries:
        id, first_seen, forward, decision = (str(e[k]) for k in ("id", "first_seen", "forward", "decision"))
        lines += _age(id, first_seen, decision, order, read)
        lines += _forward(id, forward, order, marker)
        lines += _correctness(id, e["correctness"], e["opened"], today)
    return sorted(lines)
