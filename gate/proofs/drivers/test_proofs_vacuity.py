"""The proofs row's planted proofs (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10: `proofs` gets a
planted proof of its own).

The world holds the gate, the real gate/proofs/row.py and a clean driver that passes. Planted, one per run: a driver
that runs no test; a driver whose one test is skipped; a driver whose one test fails. Each run's proofs row fails
naming the planted driver and the shim's cause, while the clean driver passes in the same run.
"""

from gate.proofs.drivers import support

GOOD = "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_a(self):\n        self.assertTrue(True)\n"
SKIPPED = "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_a(self):\n        self.skipTest('planted')\n"
FAILING = "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_a(self):\n        self.fail('planted')\n"
CLEAN = "gate.proofs.drivers.test_clean"


class ProofsVacuity(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        for rel in ("gate/proofs/__init__.py", "gate/proofs/row.py", "gate/proofs/drivers/__init__.py"):
            self.world.copy(rel)
        self.world.track("gate/proofs/drivers/test_clean.py", GOOD)

    def assertNamesOnly(self, rel: str, text: str, *, cause: str) -> None:
        self.world.track(rel, text)
        self.world.commit()
        run = self.world.run(registry=self.world.registry(support.PROOFS_ROW), tier="T4")
        self.assertParityPassed(run, "proofs")
        line = f"{rel}: pyshim: {rel.removesuffix('.py').replace('/', '.')}: {cause}"
        out = run.row_stdout("proofs", "proofs")
        self.assertEqual([x for x in out.split("\n") if x.startswith("failed: ")], [f"failed: {line}"], out)
        self.assertRed(run, "proofs", "proofs", f"exit 1: {line}")
        clean = run.row_file("proofs", "proofs", "py", f"{CLEAN}.stdout")
        self.assertIn(f"pyshim: {CLEAN}: passed 1 tests", clean)

    def test_a_driver_that_runs_no_test(self):
        self.assertNamesOnly("gate/proofs/drivers/test_empty.py", "x = 1\n", cause="ran 0 tests")

    def test_a_driver_whose_every_test_skips(self):
        self.assertNamesOnly("gate/proofs/drivers/test_skipped.py", SKIPPED, cause="skipped: 1")

    def test_a_driver_that_fails(self):
        self.assertNamesOnly("gate/proofs/drivers/test_failing.py", FAILING, cause="failed: 1 failures, 0 errors")
