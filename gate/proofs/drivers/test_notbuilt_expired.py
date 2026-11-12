"""The notbuilt row's proofs (S1.1c-1, plan check v2 section 6 "The expiry"; ADR-0005 section 4 as annotated): a
corpus file listed for a sprint the marker has reached is red at T4 naming the file, its construct and the sprint;
the marker at today's row passes; a missing marker, a marker naming no row, an entry naming no row and an entry
naming a part are each a FAIL, never a pass.

The world holds the gate (with gate/sprints.json and gate/current-sprint) and conformance/notbuilt.json: no engine,
seconds (review QA (d)). The row runs at T4, the tier the registry gives it.
"""

import json
import os

from gate.proofs.drivers import support

ROW = "notbuilt"  # the row its red cases land on: teeth.red_rows ties this proof to it (BK-S7)
MARKER = "gate/current-sprint"
NOTBUILT = "conformance/notbuilt.json"
SPRINTS = "gate/sprints.json"
S4_1 = {
    "01-agent-kill-switch.cep": "a pattern body",
    "08-session-flow.cep": "a pattern body",
    "torture/comments.cep": "a pattern body",
}


class NotBuiltExpired(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        self.world.copy(NOTBUILT)

    def marker(self, text: str) -> None:
        self.world.write(MARKER, text)

    def entries(self, edit) -> None:
        path = os.path.join(self.world.repo, NOTBUILT)
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        edit(data["entries"])
        self.world.write(NOTBUILT, json.dumps(data, indent=2, ensure_ascii=False) + "\n")

    def run_row(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row("notbuilt")), tier="T4")

    def failed_lines(self, run: support.Run) -> list[str]:
        out = run.row_stdout("test", "notbuilt")
        return [line.removeprefix("failed: ") for line in out.split("\n") if line.startswith("failed: ")]

    def assertFailReason(self, run: support.Run, *needles: str) -> dict:
        self.assertGateFailed(run)
        record = run.record("test", "notbuilt")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        for needle in needles:
            self.assertIn(needle, record["reason"], record)
        return record

    def assertExpired(self, run: support.Run, sprint: str, expired: dict[str, str]) -> None:
        """Red; one failed line per expired file naming it, its construct and the marker's sprint; no other file."""
        self.assertGateFailed(run)  # directly, one helper level from the test (S1.1c-2 design §4.2)
        record = self.assertFailReason(run)
        self.assertTrue(any(name in record["reason"] for name in expired), record)
        failed = self.failed_lines(run)
        for name, construct in expired.items():
            named = [line for line in failed if name in line]
            self.assertEqual(len(named), 1, (name, failed))
            self.assertIn(construct, named[0])
            self.assertIn(sprint, named[0])
        with open(os.path.join(support.REPO, NOTBUILT), encoding="utf-8") as f:
            others = {e["file"] for e in json.load(f)["entries"]} - set(expired)
        for name in others:
            self.assertEqual([line for line in failed if name in line], [], failed)

    def test_the_marker_at_s4_1_expires_its_three_files(self):
        self.marker("S4.1\n")
        self.assertExpired(self.run_row(), "S4.1", S4_1)

    def test_the_marker_at_todays_row_passes(self):
        self.marker("S1.1c-1\n")
        run = self.run_row()
        self.assertRow(run, "test", "notbuilt", True, "covered 1")
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)

    def test_the_marker_at_s4_2_expires_the_same_three(self):
        self.marker("S4.2\n")
        self.assertExpired(self.run_row(), "S4.2", S4_1)

    def test_no_marker_is_a_fail(self):
        os.remove(os.path.join(self.world.repo, MARKER))
        self.assertFailReason(self.run_row(), f"{MARKER}: missing")

    def test_a_marker_naming_no_row_is_a_fail(self):
        self.marker("S99\n")
        self.assertFailReason(self.run_row(), "S99", SPRINTS)

    def test_an_entry_naming_no_row_is_a_fail(self):
        def stage_only(entries):
            entries[0]["sprint"] = "S4"

        self.marker("S1.1c-1\n")
        self.entries(stage_only)
        self.assertFailReason(self.run_row(), NOTBUILT, "01-agent-kill-switch.cep")

    def test_an_entry_naming_a_part_is_a_fail(self):
        def a_part(entries):
            entries[0]["sprint"] = "S1.1c-2"

        self.marker("S1.1c-1\n")
        self.entries(a_part)
        self.assertFailReason(self.run_row(), "entries name a row, not a part")
