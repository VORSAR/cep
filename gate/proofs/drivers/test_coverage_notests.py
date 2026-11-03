"""Coverage proof 2 of 2, inclusion (design note §8; plan check C28 (c); QA Q15, Q16).

Planted: gate/proofs/internal/notests, code with no _test.go at all, beside the fully covered internal/control (so
the unit row has a test to run and passes). go test reports the package as passed with no `run` event; only the
coverage-inclusion row can fail, and it names the package.
"""

from gate.proofs.drivers import support

CONTROL = "PASS github.com/vorsar/cep/gate/proofs/internal/control 100.00% (4/4) >= 90%"
NOTESTS = "FAIL github.com/vorsar/cep/gate/proofs/internal/notests no tests"


class CoverageNotests(support.DriverTest):
    def test_a_package_without_tests_fails_inclusion(self):
        self.world.copy_gate()
        self.world.copy_proofs_module("notests", "control")
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.UNIT_ROW, support.COVERAGE_ROW))

        self.assertParityPassed(run, "test")
        self.assertRow(run, "test", "unit", True, "covered 2")
        self.assertRed(run, "test", "coverage-inclusion", "exit 1")
        self.assertEqual(run.row_stdout("test", "coverage-inclusion").split("\n")[:2], [CONTROL, NOTESTS])
        record = run.record("test", "coverage-inclusion")
        self.assertEqual((record["covered"], record["expected"]), (2, 2))
