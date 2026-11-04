"""The semantics row's proof (S3.1-1; owner 2026-11-04): a planted failing check turns the row red, naming it.

The world holds the gate and a real copy of the reference executor and its harness (docs/language/checks/). Its clean
run covers the harness; a check planted to fail — inserted before the harness's runner — turns the row red, and the
failure names that check and only that check.
"""

from gate.proofs.drivers import support

HARNESS = "docs/language/checks/test_semantics.py"
RUNNER = "\n# ---------------------------------------------------------------------- runner"
PLANT = '\n@check("planted", "a check planted to fail")\ndef _():\n    assert 1 == 2, "planted"\n\n'


class SemanticsRed(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        self.world.copy("docs/language/checks/executor.py")
        self.world.copy(HARNESS)
        self.world.commit()

    def run_semantics(self) -> support.Run:
        return self.world.run(registry=self.world.registry(support.registry_row("semantics")))

    def test_the_clean_harness_passes(self):
        run = self.run_semantics()
        self.assertParityPassed(run, "test")
        self.assertRow(run, "test", "semantics", True, "covered 1")

    def test_a_planted_failing_check_is_red_and_named(self):
        text = support.read_text(f"{self.world.repo}/{HARNESS}")
        self.assertEqual(text.count(RUNNER), 1, "the plant's anchor moved")
        self.world.track(HARNESS, text.replace(RUNNER, PLANT + RUNNER))
        self.world.commit()
        run = self.run_semantics()
        self.assertParityPassed(run, "test")
        failed = [line for line in run.row_stdout("test", "semantics").split("\n") if line.startswith("failed: ")]
        want = f"failed: {HARNESS}: planted · a check planted to fail · FAIL: AssertionError('planted')"
        self.assertEqual(failed, [want])
        self.assertRed(run, "test", "semantics", f"exit 1: {want.removeprefix('failed: ')}")
