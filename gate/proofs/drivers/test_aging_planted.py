"""The aging row's planted proofs (S1.1c-3 plan check §2 S2, §6 [3] "Aging" driver (1)-(5); owner 2026-11-12,
S1.1c-2's plan check, decisions 5 and 6 as annotated; S1.1c-3's plan check, decisions 8 and 9).

The world holds the gate (with gate/known-issues.json, gate/sprints.json and gate/current-sprint) and every tracked
Markdown file under docs/, which the entries' decisions resolve against. Each case plants one entry in the world's
gate/known-issues.json and runs the real `aging` row from gate/registry.json at T4: an entry first seen in S1.1a-1
with no decision (red, rule 1: "closed rows since"); one whose decision resolves to a section that does not name it
(red, rule 1); one forwarded to S3.3, a closed row, still open (red, rule 2: "forwarded to"); a correctness issue
opened 35 days ago (red, rule 3: "correctness issue open"); and the real tree with no plant (green).

The green case's covered count is the files the row's expected_scope selects in the world, not "the file plus its
entries" (§2 S2): the runner passes a row only when covered equals that count (`gate/runner/verdict.py`, "covered N
≠ expected M"; `step.py` takes `expected` from the scope's expansion), so file-plus-entries would be red whenever
the list does not hold exactly three entries. Flagged to the design.

Nothing here is a calendar date, a count of existing entries or a hard-coded heading: the 35 days are counted back
from today in UTC, 5 days from the 30-day boundary, so a run across midnight UTC changes nothing; the marker, a
resolving decision and the entries are read from the world's own copies. Until lane E2 lands, every red case is red
on the W0 stub's `exit 1: aging: not built yet (S1.1c-3 lane E2)`, which names no planted id, and the green case is
red on the same line.
"""

import json
import os
from datetime import UTC, datetime, timedelta

from gate.proofs.drivers import support

ROW = "aging"
JOB = "lint"
TIER = "T4"
KNOWN = "gate/known-issues.json"
MARKER = "gate/current-sprint"
CORRECTNESS_DAYS = 35  # 5 days past the 30-day boundary
AGE = "closed rows since"
FORWARD = "forwarded to"
CORRECTNESS = "correctness issue open"


class AgingPlanted(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate()
        for rel in self.world.tracked("docs"):
            if rel.endswith(".md"):
                self.world.copy(rel)
        self.today = datetime.now(UTC).date()
        self.marker = support.read_text(os.path.join(self.world.repo, MARKER)).strip()
        self.known = json.loads(support.read_text(os.path.join(self.world.repo, KNOWN)))
        self.decision = next(e["decision"] for e in self.known["entries"] if e["decision"])
        self.assertTrue(self.marker and self.decision)

    # ---------------------------------------------------------------------------------------------------- the world

    def plant(self, id: str, first_seen: str, **fields: object) -> None:
        """One entry appended to the world's gate/known-issues.json, the two clock fields always present."""
        entry = {
            "id": id,
            "first_seen": first_seen,
            "forward": "",
            "decision": "",
            "correctness": False,
            "opened": (self.today - timedelta(days=5)).isoformat(),
            **fields,
        }
        self.assertNotIn(id, {e["id"] for e in self.known["entries"]})
        self.known["entries"].append(entry)
        self.world.write(KNOWN, json.dumps(self.known, indent=2, ensure_ascii=False) + "\n")

    def run_row(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row(ROW)), tier=TIER)

    def in_scope(self) -> list[str]:
        """The row's files, from the world by the row's own pathspecs: its covered count, never hard-coded."""
        row = support.registry_row(ROW)
        self.assertEqual(row["expected_scope"][0], "git")
        out = self.world.git("ls-files", "-z", "--", *row["expected_scope"][1:])
        return [p for p in out.split("\0") if p]

    def failed_lines(self, run: support.Run) -> list[str]:
        out = run.row_stdout(JOB, ROW)
        return [line.removeprefix("failed: ") for line in out.split("\n") if line.startswith("failed: ")]

    # ----------------------------------------------------------------------------------------------- the verdicts

    def assertAged(self, run: support.Run, id: str, needle: str) -> None:
        """Red for the plant: the reason names it, a failed line names it with the rule's words, and every failed
        line is about it (the world is otherwise green); parity passed in the same run."""
        self.assertGateFailed(run)
        record = run.record(JOB, ROW)
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertTrue(record["reason"].startswith("exit 1: "), record)
        self.assertIn(id, record["reason"], record)
        failed = self.failed_lines(run)
        self.assertTrue(any(id in line and needle in line for line in failed), failed)
        self.assertEqual([line for line in failed if id not in line], [], failed)
        self.assertParityPassed(run, JOB)

    # -------------------------------------------------------------------------------------------------- the cases

    def test_an_entry_first_seen_in_s1_1a_1_with_no_decision_is_red(self):
        self.plant("QA-AGING-1", "S1.1a-1")
        self.assertAged(self.run_row(), "QA-AGING-1", AGE)

    def test_a_decision_whose_section_does_not_name_the_entry_is_red(self):
        self.plant("QA-AGING-2", "S1.1a-1", decision=self.decision)
        self.assertAged(self.run_row(), "QA-AGING-2", AGE)

    def test_a_forward_to_a_closed_row_still_open_is_red(self):
        self.plant("QA-AGING-3", "S3.3-2", forward="S3.3")
        self.assertAged(self.run_row(), "QA-AGING-3", FORWARD)

    def test_a_correctness_issue_open_35_days_is_red(self):
        opened = (self.today - timedelta(days=CORRECTNESS_DAYS)).isoformat()
        self.plant("QA-AGING-4", self.marker, correctness=True, opened=opened)
        self.assertAged(self.run_row(), "QA-AGING-4", CORRECTNESS)

    def test_the_real_tree_ages_green(self):
        run = self.run_row()
        files = self.in_scope()
        self.assertIn(KNOWN, files)
        # A literal row id: the teeth count a pass only on one (teeth.py's strength rule).
        self.assertRow(run, JOB, "aging", True, f"covered {len(files)}")
        self.assertParityPassed(run, JOB)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)
