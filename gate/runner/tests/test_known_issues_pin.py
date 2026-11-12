"""The known-issues pin over fixtures (S1.1c-2 design §4.7, v1 §4.7, §5 lane D): gate/known-issues.json and the
Markdown table of docs/product/known-issues.md agree.

The ids are equal both ways; each table row's Found cell begins with its `first_seen` followed by a non-identifier
character (`S1.1c-1's` counts, `S1.1c-10` does not match `S1.1c-1`); `first_seen` and a non-empty `forward` are
rows or parts of gate/sprints.json's order; a non-empty `decision` resolves (`anchors.resolves`), and its section --
the heading's text up to the next heading of the same level -- names the id as a whole word. The fixtures live in
gate/runner/testdata/known_issues/; the order and the reader are injected, so nothing here reads the real law files.
The pin over the real files is lane D's to add with the module.

design ambiguity: the design names the test file but no pure function for it to call; these tests call
`knownissues.check(data, table, order, read) -> list[str]` in a new pure `gate/runner/knownissues.py`, `data` the
JSON text, `table` the Markdown text, `read` the tracked-Markdown reader `anchors.resolves` takes.
design ambiguity: the fault texts are not written; each red asserts a fault naming the planted id.

RealTree (lane D) runs the pin over the real files: gate/known-issues.json, docs/product/known-issues.md, the order in
gate/sprints.json, and the Markdown git tracks, so a drift between the data and the table is red at T2.
"""

import json
import os
import unittest
from datetime import date

from gate.runner import knownissues, rows, sprintid
from gate.runner.tests import support

TODAY = date(2026, 11, 12)
TESTDATA = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "testdata", "known_issues")
ORDER = sprintid.Order(
    tuple(
        sprintid.Row(id, closed) for id, closed in (("S1.1a-1", True), ("S1.1c", False), ("S3.3", True), ("S8", False))
    )
)


def fixture(name: str) -> str:
    with open(os.path.join(TESTDATA, name), encoding="utf-8") as f:
        return f.read()


class Pin(unittest.TestCase):
    def setUp(self):
        self.data = json.loads(fixture("known-issues.json"))
        self.table = fixture("known-issues.md")
        self.decisions = fixture("decisions.md")

    def entry(self, id: str) -> dict:
        return next(e for e in self.data["entries"] if e["id"] == id)

    def check(self) -> list[str]:
        docs = {"docs/sprints/decisions.md": self.decisions}
        return knownissues.check(json.dumps(self.data), self.table, ORDER, docs.get, today=TODAY)

    def assertNames(self, id: str) -> None:
        faults = self.check()
        self.assertTrue(faults, "the plant was not seen")
        self.assertTrue(any(id in f for f in faults), faults)

    def test_the_correct_fixture_is_green(self):
        self.assertEqual(self.check(), [])

    # BK-S5 / CA-S5: one entry per id, as allowlists.load refuses a target listed twice; S1.1c-3's aging keys on it.

    def test_an_id_listed_twice_is_red(self):
        first = self.data["entries"][0]
        self.data["entries"].append({**first, "first_seen": "S1.1c"})
        n = len(self.data["entries"]) - 1
        self.assertEqual(self.check(), [f"gate/known-issues.json: entries[{n}]: id {first['id']} is listed twice"])

    def test_a_duplicate_key_is_red(self):
        text = json.dumps(self.data).replace('"first_seen": ', '"first_seen": "S8", "first_seen": ', 1)
        docs = {"docs/sprints/decisions.md": self.decisions}
        got = knownissues.check(text, self.table, ORDER, docs.get, today=TODAY)
        self.assertEqual(got, ["gate/known-issues.json: duplicate key first_seen"])

    def test_a_sub_section_belongs_to_its_section(self):
        # BA13 is named under a ### heading inside the plan-check section: the section runs to the next "## ".
        self.data["entries"].append(
            {
                "id": "BA13",
                "first_seen": "S1.1a-1",
                "forward": "S1.1c-2",
                "decision": self.entry("QB10")["decision"],
                "correctness": False,
                "opened": "2026-11-01",
            }
        )
        self.table += "| BA13 | S1.1a-1 review | The shim's pass line. | Adversarial. | S1.1c-2 |\n"
        self.assertEqual(self.check(), [])

    def test_an_id_only_in_the_json_is_red(self):
        self.data["entries"].append(
            {"id": "BA77", "first_seen": "S1.1a-1", "forward": "", "decision": "", "correctness": False, "opened": ""}
        )
        self.assertNames("BA77")

    def test_an_id_only_in_the_table_is_red(self):
        self.table += "| BA78 | S1.1a-1 review | Planted. | None. | — |\n"
        self.assertNames("BA78")

    def test_a_found_cell_not_beginning_with_first_seen_is_red(self):
        self.entry("QB10")["first_seen"] = "S1.1c-1"
        self.assertNames("QB10")

    def test_s1_1c_10_does_not_match_s1_1c_1(self):
        self.table = self.table.replace("| BA9 | S1.1c-1's review |", "| BA9 | S1.1c-10 review |")
        self.assertNames("BA9")

    def test_a_first_seen_that_is_not_a_row_or_part_is_red(self):
        self.entry("QB10")["first_seen"] = "S9.9"
        self.table = self.table.replace("| QB10 | S1.1a-1 break-it |", "| QB10 | S9.9 break-it |")
        self.assertNames("QB10")

    def test_a_forward_that_is_not_a_row_or_part_is_red(self):
        self.entry("S3.3-1-3")["forward"] = "S7"
        self.assertNames("S3.3-1-3")

    def test_a_decision_that_does_not_resolve_is_red(self):
        self.entry("QB10")["decision"] = "docs/sprints/decisions.md#no-such-heading"
        self.assertNames("QB10")

    def test_a_decision_in_an_untracked_file_is_red(self):
        self.entry("QB10")["decision"] = "docs/sprints/elsewhere.md#owner-decisions-2026-11-12--the-plan-check"
        self.assertNames("QB10")

    def test_a_decision_section_that_does_not_name_the_id_is_red(self):
        # the 2026-11-08 section says QB10X and "the id": not QB10 as a whole word (the fixture's sentence no longer
        # ends in QB10, which names it once a sentence-final id counts, design §3.2, §7 W0-5).
        self.entry("QB10")["decision"] = "docs/sprints/decisions.md#owner-decisions-2026-11-08--earlier"
        self.assertNames("QB10")

    def test_the_next_section_does_not_count_for_the_id(self):
        # S3.3-1-3 is named in the 2026-11-09 section, not in the plan-check section above it.
        self.entry("S3.3-1-3")["decision"] = self.entry("QB10")["decision"]
        self.assertNames("S3.3-1-3")


class TableSide(unittest.TestCase):
    """The two fields' Markdown side (S1.1c-3 design §3.3, §7 W0-6; DQ-S5, DQ-S15): no new columns. A correctness
    issue's Issue cell begins `**Correctness.**`, and only a correctness issue's; a Found cell's `(YYYY-MM-DD)`, when
    present, is on or after `opened` (an issue can be found later than the commit that opened it)."""

    setUp = Pin.setUp
    entry = Pin.entry
    check = Pin.check
    assertNames = Pin.assertNames

    def issue_cell(self, id: str, text: str) -> None:
        old = {"QB10": "| Two contrived module shapes can be shadowed. |"}[id]
        self.assertIn(old, self.table)
        self.table = self.table.replace(old, f"| {text} |")

    def found_cell(self, text: str) -> None:
        self.assertIn("| S3.3-1-3 | S3.3-2 review |", self.table)
        self.table = self.table.replace("| S3.3-1-3 | S3.3-2 review |", f"| S3.3-1-3 | {text} |")

    def test_a_correctness_issue_opening_its_cell_with_the_word_is_green(self):
        self.entry("QB10")["correctness"] = True
        self.issue_cell("QB10", "**Correctness.** Two contrived module shapes can be shadowed.")
        self.assertEqual(self.check(), [])

    def test_a_correctness_issue_without_the_word_is_red(self):
        self.entry("QB10")["correctness"] = True
        self.assertNames("QB10")

    def test_the_word_elsewhere_in_the_cell_does_not_count(self):
        self.entry("QB10")["correctness"] = True
        self.issue_cell("QB10", "Two contrived module shapes. **Correctness.**")
        self.assertNames("QB10")

    def test_the_word_on_an_issue_that_is_not_a_correctness_issue_is_red(self):
        self.issue_cell("QB10", "**Correctness.** Two contrived module shapes can be shadowed.")
        self.assertNames("QB10")

    def test_a_found_date_on_or_after_opened_is_green(self):
        for found in ("2026-11-09", "2026-11-10"):
            with self.subTest(found=found):
                self.setUp()
                self.found_cell(f"S3.3-2 review ({found})")
                self.assertEqual(self.check(), [])

    def test_a_found_date_before_opened_is_red_naming_opened(self):
        self.found_cell("S3.3-2 review (2026-11-08)")
        faults = self.check()
        self.assertTrue(any("S3.3-1-3" in f and "opened" in f for f in faults), faults)


class RealTree(unittest.TestCase):
    def read_repo(self, rel: str) -> str:
        with open(os.path.join(support.REPO, rel), encoding="utf-8") as f:
            return f.read()

    def test_the_real_files_agree(self):
        code, out, err = support.run_tool(["git", "ls-files", "-z", "--", "*.md"], support.REPO)
        self.assertEqual(code, 0, err)
        tracked = {p for p in out.decode("utf-8", errors="replace").split("\0") if p}
        self.assertIn(knownissues.TABLE, tracked)  # the witness: the reader sees the table it pins

        def read(path: str) -> str | None:
            return self.read_repo(path) if path in tracked else None

        data = self.read_repo(knownissues.JSON)
        self.assertTrue(json.loads(data)["entries"], "the pin needs entries to compare")
        order = sprintid.load_order(self.read_repo(sprintid.SPRINTS))
        table = self.read_repo(knownissues.TABLE)
        self.assertEqual(knownissues.check(data, table, order, read, today=rows.today()), [])  # design §3.4, W0-7


if __name__ == "__main__":
    unittest.main()
