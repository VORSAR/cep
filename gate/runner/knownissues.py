"""The known-issues pin, pure (S1.1c-2 design §1, §4.7, §11.10): gate/known-issues.json and the table of
docs/product/known-issues.md agree.

The ids are equal both ways. Each table row's Found cell begins with the entry's `first_seen`, followed by a character
that cannot continue an id (`S1.1c-1's` counts; `S1.1c-10` does not match `S1.1c-1`). `first_seen` and a non-empty
`forward` are rows or parts of gate/sprints.json's order. A non-empty `decision` resolves to a heading of a tracked
Markdown file (`anchors.resolves`), and that heading's section — up to the next heading of the same or a higher
level — names the id as a whole word. The caller injects the order and the tracked-Markdown reader.

Each entry also carries `correctness` (a bool: a wrong or missed detection, a wrong emitted value, lost state, or a
replay that differs) and `opened` (`YYYY-MM-DD`, not after the run's `today`, and on or before a `(YYYY-MM-DD)` its
Found cell holds) (owner, 2026-11-12, S1.1c-3's plan check, decision 9; design §3.3). No new columns: a correctness
issue's Issue cell begins `**Correctness.**`, and only a correctness issue's does.
"""

import json
import re
from collections.abc import Callable
from datetime import date

from gate.runner import anchors, sprintid

JSON = "gate/known-issues.json"
TABLE = "docs/product/known-issues.md"
TOP_KEYS = ("schema", "note", "entries")
ENTRY_KEYS = ("id", "first_seen", "forward", "decision", "correctness", "opened")
TEXT_KEYS = ("id", "first_seen", "forward", "decision", "opened")
ID_CHAR = r"[A-Za-z0-9_.\-]"  # an id's characters: a sprint id holds "." and "-" (S3.3-1-3)
# After an id, a "." continues it only when an id character follows (design §3.2, §7 W0-5): "S1.1c-3-2." names it.
NOT_CONTINUED = r"(?!\.*[A-Za-z0-9_\-])"
ISO_DAY = re.compile(r"[0-9]{4}-[0-9]{2}-[0-9]{2}")
FOUND_DAY = re.compile(r"\(([0-9]{4}-[0-9]{2}-[0-9]{2})\)")
CORRECTNESS_WORD = "**Correctness.**"


class _Duplicate(ValueError):
    def __init__(self, key: str) -> None:
        super().__init__(f"duplicate key {key}")


def _pairs(pairs: list[tuple[str, object]]) -> dict[str, object]:
    """A JSON object whose keys are each written once (as allowlists.load reads a list, CA-S5)."""
    out: dict[str, object] = {}
    for key, value in pairs:
        if key in out:
            raise _Duplicate(key)
        out[key] = value
    return out


def _entries(text: str) -> list[dict]:
    """The entries, strictly: the keys, the fields, and each id once (BK-S5, CA-S5: S1.1c-3's aging keys on it)."""
    data = json.loads(text, object_pairs_hook=_pairs)
    if type(data) is not dict or sorted(data) != sorted(TOP_KEYS):
        raise ValueError(f"keys {', '.join(sorted(data)) if type(data) is dict else '-'}, want {', '.join(TOP_KEYS)}")
    if type(data["entries"]) is not list:
        raise ValueError("entries is not a list")
    out = []
    seen: set[str] = set()
    for i, item in enumerate(data["entries"]):
        if type(item) is not dict or sorted(item) != sorted(ENTRY_KEYS):
            raise ValueError(f"entries[{i}]: want the keys {', '.join(ENTRY_KEYS)}")
        if any(type(item[k]) is not str for k in TEXT_KEYS) or not item["id"]:
            raise ValueError(f"entries[{i}]: {', '.join(TEXT_KEYS)} are strings, the id non-empty")
        if type(item["correctness"]) is not bool:
            raise ValueError(f"entries[{i}]: {item['id']}: correctness is not a bool (true or false)")
        if item["id"] in seen:
            raise ValueError(f"entries[{i}]: id {item['id']} is listed twice")
        seen.add(item["id"])
        out.append(item)
    return out


def _cells(line: str) -> list[str]:
    return [cell.strip() for cell in line.strip().strip("|").split("|")]


def table_cells(md: str) -> dict[str, list[str]]:
    """The known-issues table, read outside fences: id → its cells after the id (Found, Issue, …). The table is the
    one headed `Id | Found`."""
    rows: dict[str, list[str]] = {}
    inside = False
    for _, line in anchors.prose(md.splitlines()):
        if not line.lstrip().startswith("|"):
            inside = False
            continue
        cells = _cells(line)
        if cells[:2] == ["Id", "Found"]:
            inside = True
            continue
        if inside and len(cells) >= 2 and not re.fullmatch(r":?-+:?", cells[0]):
            rows[cells[0]] = cells[1:]
    return rows


def names(text: str, id: str) -> bool:
    """Whether the text names the id as a whole word: no id character before it, and none after it but a sentence's
    full stop (a "." that no id character follows)."""
    return re.search(rf"(?<!{ID_CHAR}){re.escape(id)}{NOT_CONTINUED}", text) is not None


def day(text: str) -> date | None:
    """A `YYYY-MM-DD` calendar day, else None."""
    if not ISO_DAY.fullmatch(text):
        return None
    try:
        return date.fromisoformat(text)
    except ValueError:
        return None


def _level(line: str) -> int:
    stripped = line.lstrip(" ")
    return len(stripped) - len(stripped.lstrip("#"))


def section(md: str, anchor: str) -> str:
    """The text of the heading with this anchor, up to the next heading of the same or a higher level; "" if none."""
    lines = md.splitlines()
    seen: dict[str, int] = {}
    start = level = 0
    body: list[str] = []
    for number, line in anchors.prose(lines):
        m = anchors.HEADING.match(line)
        if start:
            if m and m.group(1) and _level(line) <= level:
                break
            body.append(line)
            continue
        if m and m.group(1):
            base = anchors.slug(m.group(1))
            n = seen.get(base, 0)
            seen[base] = n + 1
            if (base if n == 0 else f"{base}-{n}") == anchor:
                start, level = number, _level(line)
                body.append(line)
    return "\n".join(body)


def _sprint_fault(where: str, sprint: str, order: sprintid.Order) -> str:
    try:
        order.index(sprint)
    except ValueError:
        return f"{where} {sprint} is not a row or a part of a row of gate/sprints.json"
    return ""


def _row_faults(id: str, e: dict, cells: list[str]) -> list[str]:
    """The table row against its entry: the Found cell's first_seen, its date against opened, the Issue cell's word."""
    found, issue = cells[0], cells[1] if len(cells) > 1 else ""
    faults = []
    if not re.match(rf"{re.escape(e['first_seen'])}(?!{ID_CHAR})", found) or not e["first_seen"]:
        faults.append(f"{id}: the Found cell {found!r} does not begin with first_seen {e['first_seen']}")
    found_days = sorted(d for d in (day(text) for text in FOUND_DAY.findall(found)) if d is not None)
    opened = day(e["opened"])
    if opened is not None and found_days and opened > found_days[0]:
        faults.append(f"{id}: opened {e['opened']} is after the date {found_days[0]} its Found cell gives")
    if e["correctness"] and not issue.startswith(CORRECTNESS_WORD):
        faults.append(f"{id}: a correctness issue's Issue cell begins {CORRECTNESS_WORD}")
    if not e["correctness"] and issue.startswith(CORRECTNESS_WORD):
        faults.append(f"{id}: the Issue cell begins {CORRECTNESS_WORD}, and correctness is false")
    return faults


def _opened_fault(id: str, opened: str, today: date) -> str:
    """ "" when opened is a day not after today; else why, naming the field."""
    d = day(opened)
    if d is None:
        return f"{id}: opened {opened!r} is not a day YYYY-MM-DD"
    if d > today:
        return f"{id}: opened {opened} is after today {today.isoformat()}"
    return ""


def check(data: str, table: str, order: sprintid.Order, read: Callable[[str], str | None], *, today: date) -> list[str]:
    """Every disagreement, one line each naming the id; [] when the file and the table agree. `today` is the run's
    UTC day (`rows.today()`), a required keyword so no caller skips the opened check (design §3.3, DQ-S14)."""
    try:
        entries = _entries(data)
    except ValueError as e:
        return [f"{JSON}: {e}"]
    rows = table_cells(table)
    by_id = {e["id"]: e for e in entries}
    faults = [f"{id}: in {JSON} and not in the table of {TABLE}" for id in sorted(set(by_id) - set(rows))]
    faults += [f"{id}: in the table of {TABLE} and not in {JSON}" for id in sorted(set(rows) - set(by_id))]
    for id in sorted(set(by_id) & set(rows)):
        faults += _row_faults(id, by_id[id], rows[id])
    for id in sorted(by_id):
        e = by_id[id]
        why = _opened_fault(id, e["opened"], today)
        if why:
            faults.append(why)
        for where in ("first_seen", "forward"):
            if where == "forward" and not e[where]:
                continue
            why = _sprint_fault(where, e[where], order)
            if why:
                faults.append(f"{id}: {why}")
        if not e["decision"]:
            continue
        why = anchors.resolves(e["decision"], read)
        if why:
            faults.append(f"{id}: decision {why}")
            continue
        path, anchor = anchors.split_ref(e["decision"])
        if not names(section(read(path) or "", anchor), id):
            faults.append(f"{id}: decision {e['decision']}: the section does not name {id}")
    return faults
