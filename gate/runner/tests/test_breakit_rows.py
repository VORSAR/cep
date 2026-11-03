"""Break-it of S1.1a-1's combined build (QA): test modules that no row runs, and a floor that shrinks unseen.

QB1. The pytests row excludes gate/proofs/** and the proofs row covers only gate/proofs/drivers/test_*.py, so a
tracked test module anywhere else under gate/proofs/ (gate/proofs/tests/, gate/proofs/, a directory below drivers/)
is run by no row. The pytests wrapper's search for test files "that would silently never run" does not look there
either, so the module's failing test never runs and the row passes. The witness: the same failing module where the
row does look turns it red.

QB2. The pyfloor row names its four test modules by literal path, and its min_wants is 0. `git ls-files` drops a
pathspec that matches nothing without a word, so renaming one of the four (here test_manifest.py, whose subject
install.py imports) shrinks the row to three modules, covered 3 = expected 3, and it passes: the 3.12 floor of the
renamed module's subject is no longer checked by anything. The witness: the four modules as registered pass.
"""

import dataclasses
import os

from gate.runner import registry
from gate.runner.tests import support
from gate.runner.tests.test_rows_py import GOOD, Base, wrapper

FAILING = GOOD.replace("assertTrue(True)", "fail('planted: this test must run and fail')")
NO_ROW = "no row runs it: move it into the pytests scope or directly into gate/proofs/drivers/"


class QB1TestModulesNoRowRuns(Base):
    def assertRedNaming(self, rel: str, why: str = NO_ROW) -> None:
        self.assertEqual(self.pytests(), f"exit 1: {rel}: {why}", self.out())

    def test_qb1_witness_the_failing_module_is_red_where_pytests_looks(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_planted.py", FAILING)
        self.assertRedNaming("gate/x/test_planted.py", "pyshim: gate.x.test_planted: failed: 1 failures, 0 errors")

    def test_qb1_a_test_module_in_a_tests_directory_beside_the_proofs_row(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/proofs/__init__.py", "")
        self.track("gate/proofs/tests/__init__.py", "")
        self.track("gate/proofs/tests/test_row.py", FAILING)
        self.assertRedNaming("gate/proofs/tests/test_row.py")

    def test_qb1_a_test_module_at_the_top_of_gate_proofs(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/proofs/__init__.py", "")
        self.track("gate/proofs/test_row.py", FAILING)
        self.assertRedNaming("gate/proofs/test_row.py")

    def test_qb1_a_test_module_one_directory_below_the_drivers(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/proofs/__init__.py", "")
        self.track("gate/proofs/drivers/__init__.py", "")
        self.track("gate/proofs/drivers/more/__init__.py", "")
        self.track("gate/proofs/drivers/more/test_driver.py", FAILING)
        self.assertRedNaming("gate/proofs/drivers/more/test_driver.py")


class QB2PyfloorShrinksUnseen(Base):
    def floor(self) -> registry.Row:
        """The pyfloor row exactly as gate/registry.json has it; only its wrapper is the real one by absolute path."""
        rows = registry.load(os.path.join(support.GATE, "registry.json"))
        (row,) = [r for r in rows if r.id == "pyfloor"]
        return dataclasses.replace(row, command=wrapper("pyfloor"))

    def plant(self, row: registry.Row, renamed: str = "") -> dict:
        """Track every floor module as a trivial passing test; the one named `renamed` under a new name."""
        paths = row.expected_scope[1:]
        for path in paths:
            parts = path.split("/")[:-1]
            for depth in range(1, len(parts) + 1):
                init = "/".join([*parts[:depth], "__init__.py"])
                if not os.path.exists(os.path.join(self.repo, init)):
                    self.track(init, "")
            self.track(path.replace("/test_", "/test_moved_") if path == renamed else path, GOOD)
        self.run_rows([row])
        return self.record("lint", "pyfloor")

    def test_qb2_witness_the_four_registered_floor_modules_pass(self):
        row = self.floor()
        self.assertEqual(len(row.expected_scope[1:]), 4, row.expected_scope)
        record = self.plant(row)
        self.assertEqual((record["verdict"], record["reason"]), ("pass", "covered 4"))

    def test_qb2_a_renamed_floor_module_does_not_pass_unseen(self):
        row = self.floor()
        renamed = "gate/toolchain/tests/test_manifest.py"
        self.assertIn(renamed, row.expected_scope)
        record = self.plant(row, renamed)
        self.assertEqual(record["verdict"], "fail", f"pyfloor passed without {renamed}: {record}")
