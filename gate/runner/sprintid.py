"""The sprint-id grammar and the sprint plan's printed order (S1.1c-1 lane D; design note §3.2).

A sprint id is S<stage>[.<part>][<letter>][-<n>] (sprint-plan.md §3); a row of the plan's table is an id without
-<n>, and a part such as S3.3-2 belongs to its row S3.3. The order of rows is the table's printed order, read from
gate/sprints.json (owner 2026-11-11, S1.1c-1's plan check, 7), never a numeric or string sort: row S3.3 precedes
row S1.1c, and S9.2 precedes S10. Pure: strings in, values out; the callers read the files.
"""

import dataclasses
import re
from dataclasses import dataclass

GRAMMAR = re.compile(r"S(?P<stage>[1-9][0-9]*)(?:\.(?P<part>[1-9][0-9]*))?(?P<letter>[a-z])?(?:-(?P<n>[1-9][0-9]*))?")
HEADER = "| Sprint | Title | SP (items) | Routine | Depends on |"


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
