"""The known issues' two new fields (S1.1c-3 plan check §2 S2 "Under Q9", §6 [3] aging (10); owner 2026-11-12,
S1.1c-3's plan check, decision 9): each entry of gate/known-issues.json gains `correctness` (a JSON bool: is it a
correctness bug - a wrong or missed detection, a wrong emitted value, lost state, or a replay that differs) and
`opened` (the UTC day it was opened, `YYYY-MM-DD`), and the known-issues pin (`knownissues.check`) refuses a value it
cannot trust, naming the field: a non-bool `correctness`; an `opened` that is malformed, in the future, or later than
a date the entry's Found cell names (S3.3-1-3's cell reads "S3.3-2, the flaky-test fix (2026-11-09)").

Red today on behaviour: `knownissues.ENTRY_KEYS` holds four keys, so every entry carrying the fields is refused with
the generic "want the keys id, first_seen, forward, decision". Each refusal case asserts the field's own name and
NOT that generic text, so a pin that only learns the two keys stays red here.

The date (design §3.3; DQ-S14): `knownissues.check(..., *, today)` takes `today` as a required keyword, so every
case here passes it; the real-tree pin and the row pass `rows.today()`.
The table side (design §3.3, §7 W0-6; DQ-S15): no new columns; a correctness issue's Issue cell begins
`**Correctness.**` (test_known_issues_pin.py's TableSide pins it both ways); `opened` is on or before a Found cell's
`(YYYY-MM-DD)` (DQ-S5).
"""

import copy
import json
import unittest
from datetime import date, timedelta

from gate.runner import anchors, knownissues, sprintid

TODAY = date(2026, 11, 12)
DECISIONS = "docs/sprints/decisions.md"
HEADING = "Owner decisions, 2026-11-12 — the plan check"
REF = f"{DECISIONS}#{anchors.slug(HEADING)}"
GENERIC = "want the keys"
ORDER = sprintid.Order(
    tuple(
        sprintid.Row(id, closed) for id, closed in (("S1.1a-1", True), ("S3.3", True), ("S1.1c", False), ("S8", False))
    )
)
DATA = {
    "schema": 1,
    "note": "A fixture for test_known_issues_fields.py (S1.1c-3 plan check §2 S2, decision 9).",
    "entries": [
        {
            "id": "QB10",
            "first_seen": "S1.1a-1",
            "forward": "S1.1c-3",
            "decision": REF,
            "correctness": False,
            "opened": "2026-11-01",
        },
        {
            "id": "S3.3-1-3",
            "first_seen": "S3.3-2",
            "forward": "S8",
            "decision": REF,
            "correctness": False,
            "opened": "2026-11-09",
        },
    ],
}
TABLE = """# Known issues

| Id | Found | Issue | Why not fixed yet | Forward |
|---|---|---|---|---|
| QB10 | S1.1a-1 break-it | Two contrived module shapes can be shadowed. | Contrived. | S1.1c-3 |
| S3.3-1-3 | S3.3-2, the flaky-test fix (2026-11-09) | A misleading counter. | Later. | S8 |
"""
MARKDOWN = f"""# Decisions

## {HEADING}

QB10 goes to S1.1c-3; S3.3-1-3 stays forwarded to S8.
"""


class Fields(unittest.TestCase):
    def setUp(self):
        self.data = copy.deepcopy(DATA)
        self.table = TABLE

    def entry(self, id: str) -> dict:
        return next(e for e in self.data["entries"] if e["id"] == id)

    def check(self, today: date = TODAY) -> list[str]:
        return knownissues.check(json.dumps(self.data), self.table, ORDER, {DECISIONS: MARKDOWN}.get, today=today)

    def assertRefused(self, field: str, faults: list[str]) -> None:
        """Refused, the field named, and not by the generic keys refusal."""
        self.assertTrue(faults, "the plant was not seen")
        self.assertTrue(any(field in f for f in faults), faults)
        self.assertFalse(any(GENERIC in f for f in faults), faults)

    def test_entries_carrying_both_fields_pass(self):
        self.assertEqual(self.check(), [])

    def test_a_correctness_bug_passes(self):
        self.entry("QB10")["correctness"] = True
        self.table = TABLE.replace("| Two contrived", "| **Correctness.** Two contrived")  # design §3.3
        self.assertEqual(self.check(), [])

    def test_correctness_not_a_bool_is_refused_naming_the_field(self):
        for value in ("true", "false", 1, 0, None, ""):
            with self.subTest(value=value):
                self.entry("QB10")["correctness"] = value
                self.assertRefused("correctness", self.check())

    def test_a_malformed_opened_is_refused_naming_the_field(self):
        for value in ("2026-13-01", "2026-02-30", "2026-1-05", "20261105", "2026-11-05T00:00", " 2026-11-05", "", 3):
            with self.subTest(value=value):
                self.entry("QB10")["opened"] = value
                self.assertRefused("opened", self.check())

    def test_an_opened_in_the_future_is_refused_naming_the_field(self):
        self.entry("QB10")["opened"] = (TODAY + timedelta(days=5)).isoformat()
        faults = self.check(TODAY)
        self.assertRefused("opened", faults)
        self.assertTrue(any("QB10" in f for f in faults), faults)

    def test_an_opened_of_today_passes(self):
        self.entry("QB10")["opened"] = TODAY.isoformat()
        self.assertEqual(self.check(TODAY), [])

    def test_an_opened_later_than_the_found_cells_date_is_refused_naming_the_field(self):
        self.entry("S3.3-1-3")["opened"] = "2026-11-10"
        faults = self.check()
        self.assertRefused("opened", faults)
        self.assertTrue(any("S3.3-1-3" in f for f in faults), faults)

    def test_an_opened_on_or_before_the_found_cells_date_passes(self):
        for value in ("2026-11-09", "2026-11-08"):
            with self.subTest(value=value):
                self.entry("S3.3-1-3")["opened"] = value
                self.assertEqual(self.check(), [])

    def test_a_missing_field_is_refused_naming_it(self):
        for field in ("correctness", "opened"):
            with self.subTest(field=field):
                self.setUp()
                del self.entry("QB10")[field]
                faults = self.check()
                self.assertTrue(any(field in f for f in faults), faults)


if __name__ == "__main__":
    unittest.main()
