"""The two pins (S1.1c-1 lane D; design note §3.5; owner 2026-11-10, S1.1c's scope, 8; 2026-11-11, the plan check, 7;
2026-11-27, the names before the pattern engine, 6).

gate/sprints.json is the plan's §3 table as printed, with its closed marks; gate/current-sprint is that table's first
open row (or a part of it), or, between sprints, the closed row printed just before it until that row's sprint record
exists in docs/sprints/. All read the real files: a forgotten marker bump once the sprint's record is opened, or a
table that drifts from the plan, is red at T2 naming the row both ways.
"""

import os
import unittest

from gate.runner import sprintid

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
SPRINTS = os.path.join(REPO, "gate", "sprints.json")
PLAN = os.path.join(REPO, "docs", "sprints", "sprint-plan.md")
MARKER = os.path.join(REPO, "gate", "current-sprint")
RECORDS = os.path.join(REPO, "docs", "sprints")


def read(path: str) -> str:
    with open(path, encoding="utf-8") as f:
        return f.read()


def records() -> list[str]:
    return sorted(name for name in os.listdir(RECORDS) if name.endswith("-record.md"))


class ThePins(unittest.TestCase):
    def test_sprints_json_equals_the_plan_table(self):
        plan = sprintid.plan_rows(read(PLAN))
        order = sprintid.load_order(read(SPRINTS))
        self.assertEqual(sprintid.compare_rows(plan, order), "")

    def test_the_marker_is_the_first_open_row(self):
        marker = sprintid.read_marker(read(MARKER))
        order = sprintid.load_order(read(SPRINTS))
        self.assertEqual(sprintid.first_open_mismatch(marker, order, records()), "")

    def test_every_sprint_record_belongs_to_a_printed_row(self):
        # The record guard finds a sprint's record by its row's name; a record no row claims would never arm it.
        order = sprintid.load_order(read(SPRINTS))
        found = records()
        self.assertTrue(found, "no *-record.md in docs/sprints")  # witness: the scan sees the records
        orphans = [name for name in found if not any(sprintid.has_record(row.id, (name,)) for row in order.rows)]
        self.assertEqual(orphans, [])
