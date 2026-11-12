"""The sprint-id grammar and the sprint plan's printed order (S1.1c-1 lane D; design note §3.2).

A sprint id is S<stage>[.<part>][<letter>][-<n>] (sprint-plan.md §3); a row of the plan's table is an id without
-<n>, and a part such as S3.3-2 belongs to its row S3.3. The order of rows is the table's printed order, read from
gate/sprints.json (owner 2026-11-11, S1.1c-1's plan check, 7), never a numeric or string sort: row S3.3 precedes
row S1.1c, and S9.2 precedes S10. Pure: strings in, values out; the callers read the files.
"""

import dataclasses
import json
import re
from collections.abc import Sequence
from dataclasses import dataclass

GRAMMAR = re.compile(r"S(?P<stage>[1-9][0-9]*)(?:\.(?P<part>[1-9][0-9]*))?(?P<letter>[a-z])?(?:-(?P<n>[1-9][0-9]*))?")
HEADER = "| Sprint | Title | SP (items) | Routine | Depends on |"
SPRINTS = "gate/sprints.json"
MARKER = "gate/current-sprint"
PLAN = "sprint-plan.md"
TICK = "✓"
ORDER_KEYS = frozenset({"schema", "note", "rows"})
ROW_KEYS = frozenset({"id", "closed"})
# A plan row's id cell, stripped: struck (skipped), open, closed by a linked tick, closed by a tick and a parenthesis.
STRUCK = re.compile(r"~~.*~~")
OPEN = re.compile(r"(?P<id>\S+)")
LINKED = re.compile(r"(?P<id>\S+) \[✓\]\([^()\s]+\)")
TICKED = re.compile(r"(?P<id>\S+) ✓ \(.+\)")


@dataclass(frozen=True)
class ID:
    stage: int
    part: int  # 0 when absent
    letter: str  # "" when absent
    n: int  # 0 when absent


@dataclass(frozen=True)
class Row:
    id: str  # a printed row id ("S3.3", "S1.1a-1")
    closed: bool


def parse(text: str) -> ID:
    m = GRAMMAR.fullmatch(text)
    if m is None:
        raise ValueError(f"{text!r}: not a sprint id (S<stage>[.<part>][<letter>][-<n>])")
    return ID(int(m["stage"]), int(m["part"] or 0), m["letter"] or "", int(m["n"] or 0))


def canonical(id: ID) -> str:
    return f"S{id.stage}" + (f".{id.part}" if id.part else "") + id.letter + (f"-{id.n}" if id.n else "")


def row_of(id: ID) -> str:
    return canonical(dataclasses.replace(id, n=0))


@dataclass(frozen=True)
class Order:
    """The plan table's rows in printed order; a position is a row's index in that order."""

    rows: tuple[Row, ...]

    def index(self, text: str) -> int:
        """The position of the printed id, else of its row; a part is not validated by the table (pinned)."""
        id = parse(text)
        ids = [row.id for row in self.rows]
        for candidate in (canonical(id), row_of(id)):
            if candidate in ids:
                return ids.index(candidate)
        raise ValueError(f"{text}: not a row of {SPRINTS}")

    def reached(self, entry: str, current: str) -> bool:
        return self.index(entry) <= self.index(current)

    def passed(self, part: str, marker: str) -> bool:
        """Whether the marker is past the part: a later row, or a later part of the same row, parts compared as numbers.

        Positions are `index` (a printed part-row such as S1.1a-1 is a row of its own). At one position a part-less
        part reads n = 0, so S1.1c-1 has passed S1.1c; a part-less marker is not past a part of its own row (S1.1c-3
        design §3.1, §7 W0-3: aging names that case). A ValueError names a text that is not an id or not in the order.
        """
        at_part, at_marker = self.index(part), self.index(marker)
        if at_part != at_marker:
            return at_marker > at_part
        return parse(marker).n > parse(part).n

    def first_open(self) -> str:
        for row in self.rows:
            if not row.closed:
                return row.id
        raise ValueError(f"every row of {SPRINTS} is closed")


def _keys(value: object, keys: frozenset[str], where: str) -> dict:
    if not isinstance(value, dict):
        raise ValueError(f"{where}: not an object")
    if set(value) != keys:
        missing, extra = sorted(keys - set(value)), sorted(set(value) - keys)
        raise ValueError(f"{where}: keys {sorted(value)}, missing {missing}, unexpected {extra}")
    return value


def load_order(text: str) -> Order:
    """gate/sprints.json: {"schema": 1, "note": str, "rows": [{"id": str, "closed": bool}, ...]}, ids unique."""
    try:
        data = json.loads(text)
    except ValueError as e:
        raise ValueError(f"{SPRINTS}: {e}") from None
    data = _keys(data, ORDER_KEYS, SPRINTS)
    if data["schema"] != 1:
        raise ValueError(f"{SPRINTS}: schema {data['schema']!r}, not 1")
    if not isinstance(data["note"], str):
        raise ValueError(f"{SPRINTS}: note is not a string")
    if not isinstance(data["rows"], list) or not data["rows"]:
        raise ValueError(f"{SPRINTS}: rows is not a non-empty list")
    rows: list[Row] = []
    for i, item in enumerate(data["rows"]):
        where = f"{SPRINTS}: rows[{i}]"
        item = _keys(item, ROW_KEYS, where)
        if not isinstance(item["id"], str):
            raise ValueError(f"{where}: id is not a string")
        try:
            parse(item["id"])
        except ValueError as e:
            raise ValueError(f"{where}: {e}") from None
        if not isinstance(item["closed"], bool):
            raise ValueError(f"{where}: closed is not a bool")
        if any(row.id == item["id"] for row in rows):
            raise ValueError(f"{where}: duplicate id {item['id']}")
        rows.append(Row(item["id"], item["closed"]))
    return Order(tuple(rows))


def read_marker(text: str) -> ID:
    """gate/current-sprint: exactly one line ending in one newline, no other whitespace."""
    body = text[:-1]
    if not text.endswith("\n") or not body or any(ch.isspace() for ch in body):
        raise ValueError(f"{MARKER}: one line, one trailing newline, got {text!r}")
    return parse(body)


def _plan_cell(n: int, cell: str) -> Row | None:
    """A plan row's id cell: None for a struck row, else its Row; the three shapes are pinned here."""
    if STRUCK.fullmatch(cell):
        return None
    for shape, closed in ((OPEN, False), (LINKED, True), (TICKED, True)):
        m = shape.fullmatch(cell)
        if m is not None:
            try:
                parse(m["id"])
            except ValueError as e:
                raise ValueError(f"{PLAN} line {n}: {e}") from None
            return Row(m["id"], closed)
    raise ValueError(f"{PLAN} line {n}: unknown row shape {cell!r}")


def plan_rows(markdown: str) -> list[Row]:
    """The plan's §3 table (used only by the T2 pin test): one Row per printed row, struck rows skipped."""
    lines = markdown.split("\n")
    if HEADER not in lines:
        for n, line in enumerate(lines, 1):
            if line.startswith("| Sprint"):
                raise ValueError(f"{PLAN} line {n}: the §3 table header reads {line!r}, not {HEADER!r}")
        raise ValueError(f"{PLAN}: the §3 table header was not found")
    rows: list[Row] = []
    first = lines.index(HEADER) + 2  # the header's line, then the separator row, both skipped
    for n, line in enumerate(lines[first:], first + 1):
        if not line.startswith("| "):
            break
        cells = line.split("|")
        if any(TICK in cell for cell in cells[2:]):
            raise ValueError(f"{PLAN} line {n}: {TICK} outside the id cell")
        row = _plan_cell(n, cells[1].strip())
        if row is not None:
            rows.append(row)
    return rows


def _state(row: Row | None) -> str:
    return "absent" if row is None else f"{row.id} ({'closed' if row.closed else 'open'})"


def compare_rows(plan: Sequence[Row], order: Order) -> str:
    """ "" when gate/sprints.json equals the plan's table; else the first difference, named both ways."""
    for i in range(max(len(plan), len(order.rows))):
        b = plan[i] if i < len(plan) else None
        a = order.rows[i] if i < len(order.rows) else None
        if a != b:
            return f"{SPRINTS} row {i + 1} is {_state(a)}; {PLAN} §3 row {i + 1} is {_state(b)}"
    return ""


def first_open_mismatch(marker: ID, order: Order) -> str:
    """ "" when the marker's row is the first open row; else the mismatch, naming both."""
    if row_of(marker) == order.first_open():
        return ""
    return f"{MARKER} {canonical(marker)} is not the first open row {order.first_open()}"
