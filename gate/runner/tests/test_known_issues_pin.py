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
"""

import json
import os
import unittest

from gate.runner import knownissues, sprintid

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
        return knownissues.check(json.dumps(self.data), self.table, ORDER, docs.get)

    def assertNames(self, id: str) -> None:
        faults = self.check()
        self.assertTrue(faults, "the plant was not seen")
        self.assertTrue(any(id in f for f in faults), faults)

    def test_the_correct_fixture_is_green(self):
        self.assertEqual(self.check(), [])

    def test_a_sub_section_belongs_to_its_section(self):
        # BA13 is named under a ### heading inside the plan-check section: the section runs to the next "## ".
        self.data["entries"].append(
            {"id": "BA13", "first_seen": "S1.1a-1", "forward": "S1.1c-2", "decision": self.entry("QB10")["decision"]}
        )
        self.table += "| BA13 | S1.1a-1 review | The shim's pass line. | Adversarial. | S1.1c-2 |\n"
        self.assertEqual(self.check(), [])

    def test_an_id_only_in_the_json_is_red(self):
        self.data["entries"].append({"id": "BA77", "first_seen": "S1.1a-1", "forward": "", "decision": ""})
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
        # the 2026-11-08 section says QB10X: not QB10 as a whole word.
        self.entry("QB10")["decision"] = "docs/sprints/decisions.md#owner-decisions-2026-11-08--earlier"
        self.assertNames("QB10")

    def test_the_next_section_does_not_count_for_the_id(self):
        # S3.3-1-3 is named in the 2026-11-09 section, not in the plan-check section above it.
        self.entry("S3.3-1-3")["decision"] = self.entry("QB10")["decision"]
        self.assertNames("S3.3-1-3")


if __name__ == "__main__":
    unittest.main()
