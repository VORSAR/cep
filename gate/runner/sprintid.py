"""The sprint-id grammar and the sprint plan's printed order (S1.1c-1 lane D; design note §3.2).

A sprint id is S<stage>[.<part>][<letter>][-<n>] (sprint-plan.md §3); a row of the plan's table is an id without
-<n>, and a part such as S3.3-2 belongs to its row S3.3. The order of rows is the table's printed order, read from
gate/sprints.json (owner 2026-11-11, S1.1c-1's plan check, 7), never a numeric or string sort: row S3.3 precedes
row S1.1c, and S9.2 precedes S10. Pure: strings in, values out; the callers read the files.
"""

import dataclasses
import json
import re
from dataclasses import dataclass

GRAMMAR = re.compile(r"S(?P<stage>[1-9][0-9]*)(?:\.(?P<part>[1-9][0-9]*))?(?P<letter>[a-z])?(?:-(?P<n>[1-9][0-9]*))?")
HEADER = "| Sprint | Title | SP (items) | Routine | Depends on |"
SPRINTS = "gate/sprints.json"
MARKER = "gate/current-sprint"
ORDER_KEYS = frozenset({"schema", "note", "rows"})
ROW_KEYS = frozenset({"id", "closed"})


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
