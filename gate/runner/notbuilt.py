"""The not-built rule (S1.1c-1 lane D; design note §3.3; owner 2026-11-09, 6; 2026-11-11, the plan check, 2 and 6).

conformance/notbuilt.json lists every corpus file this release cannot deploy, each with the construct its refusal
names and the sprint-plan row that turns it green. Once gate/current-sprint reaches an entry's row, the entry has
expired: the file must deploy or the entry must move, and the notbuilt row turns red naming it. Entries name rows,
never parts: reached() is by row, so an entry for S1.1c-2 would expire under S1.1c-1. Pure: the row reads the files.
"""

import json
from collections.abc import Sequence
from dataclasses import dataclass

from gate.runner import sprintid

NOTBUILT = "conformance/notbuilt.json"
TOP_KEYS = frozenset({"schema", "note", "entries"})
ENTRY_KEYS = frozenset({"file", "construct", "sprint"})


@dataclass(frozen=True)
class Entry:
    file: str
    construct: str
    sprint: str


def _keys(value: object, keys: frozenset[str], where: str) -> dict:
    if not isinstance(value, dict):
        raise ValueError(f"{where}: not an object")
    if set(value) != keys:
        missing, extra = sorted(keys - set(value)), sorted(set(value) - keys)
        raise ValueError(f"{where}: keys {sorted(value)}, missing {missing}, unexpected {extra}")
    return value


def load_entries(text: str) -> tuple[Entry, ...]:
    """{"schema": 1, "note": str, "entries": [{"file", "construct", "sprint"}, ...]}: files unique, sprints rows."""
    try:
        data = json.loads(text)
    except ValueError as e:
        raise ValueError(f"{NOTBUILT}: {e}") from None
    data = _keys(data, TOP_KEYS, NOTBUILT)
    if data["schema"] != 1:
        raise ValueError(f"{NOTBUILT}: schema {data['schema']!r}, not 1")
    if not isinstance(data["note"], str):
        raise ValueError(f"{NOTBUILT}: note is not a string")
    if not isinstance(data["entries"], list):
        raise ValueError(f"{NOTBUILT}: entries is not a list")
    entries: list[Entry] = []
    for i, item in enumerate(data["entries"]):
        where = f"{NOTBUILT}: entries[{i}]"
        item = _keys(item, ENTRY_KEYS, where)
        for key in sorted(ENTRY_KEYS):
            if not isinstance(item[key], str) or not item[key]:
                raise ValueError(f"{where}: {key} is not a non-empty string")
        try:
            id = sprintid.parse(item["sprint"])
        except ValueError as e:
            raise ValueError(f"{where}: {e}") from None
        if id.n:
            raise ValueError(f"{NOTBUILT}: entries name a row, not a part: {item['sprint']}")
        if any(entry.file == item["file"] for entry in entries):
            raise ValueError(f"{where}: duplicate file {item['file']}")
        entries.append(Entry(item["file"], item["construct"], item["sprint"]))
    return tuple(entries)


def expired(entries: Sequence[Entry], order: sprintid.Order, current: sprintid.ID) -> list[str]:
    """One line per entry whose row the marker has reached, in file order; an entry naming no row is an error."""
    now = sprintid.canonical(current)
    order.index(now)  # the marker's own fault, not an entry's, if it names no row
    lines: list[str] = []
    for entry in entries:
        try:
            reached = order.reached(entry.sprint, now)
        except ValueError:
            raise ValueError(f"{NOTBUILT}: {entry.file}: {entry.sprint}: not a row of {sprintid.SPRINTS}") from None
        if reached:
            lines.append(
                f"{entry.file}: not built ({entry.construct}), listed for {entry.sprint}, which is reached by {now}"
            )
    return lines
