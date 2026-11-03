"""The pytests proof (design note §8; plan check C10; QA DQ8, DQ17): five ways a test module could pass vacuously.

Five runs, one plant each: an import error; every test skipped; no tests; an untracked test_x.py; a tracked
runner_test.py. Each run also holds a clean module, which passes in the same run, and a sixth run holds the clean
module alone, which passes the row. Each red names its file, and only its file.
"""

from gate.proofs.drivers import support

GOOD = "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_a(self):\n        self.assertTrue(True)\n"
SKIPPED = "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_a(self):\n        self.skipTest('planted')\n"
CLEAN = "gate.plants.test_clean"
BROKEN = "import no_such_module_planted\n"


class PytestsVacuity(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        self.world.track("gate/plants/__init__.py", "")
        self.world.track("gate/plants/test_clean.py", GOOD)
        self.world.commit()

    def run_pytests(self) -> support.Run:
        return self.world.run(registry=self.world.registry(support.registry_row("pytests")))

    def assertNamesOnly(self, rel: str, text: str, *, tracked: bool, cause: str) -> None:
        (self.world.track if tracked else self.world.write)(rel, text)
        run = self.run_pytests()
        self.assertParityPassed(run, "test")
        self.assertRed(run, "test", "pytests", "exit 1")
        out = run.row_stdout("test", "pytests")
        failed = [line for line in out.split("\n") if line.startswith("failed: ")]
        self.assertEqual(len(failed), 1, out)
        self.assertTrue(failed[0].startswith(f"failed: {rel}: "), failed)
        self.assertIn(cause, failed[0])
        clean = run.row_file("test", "pytests", "py", f"{CLEAN}.stdout")
        self.assertIn(f"pyshim: {CLEAN}: passed 1 tests", clean)

    def test_a_module_that_fails_to_import(self):
        self.assertNamesOnly("gate/plants/test_import.py", BROKEN, tracked=True, cause="import error")

    def test_a_module_whose_every_test_skips(self):
        self.assertNamesOnly("gate/plants/test_skipped.py", SKIPPED, tracked=True, cause="skipped: 1")

    def test_a_module_with_no_tests(self):
        self.assertNamesOnly("gate/plants/test_empty.py", "x = 1\n", tracked=True, cause="ran 0 tests")

    def test_an_untracked_test_module(self):
        self.assertNamesOnly("gate/plants/test_x.py", GOOD, tracked=False, cause="untracked, so it never runs")

    def test_a_tracked_misnamed_module(self):
        self.assertNamesOnly("gate/plants/runner_test.py", GOOD, tracked=True, cause="misnamed")

    def test_the_clean_control_passes(self):
        run = self.run_pytests()
        self.assertParityPassed(run, "test")
        self.assertRow(run, "test", "pytests", True, "covered 1")
        self.assertEqual((run.tail(), run.code), (["GATE PASSED", "exit: 0"], 0))
