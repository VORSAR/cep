"""Coverage proof 1 of 2, the floor (design note §8; plan check §7 lane C; QA Q16).

Planted: gate/proofs/internal/below, nine statements with a test covering eight, beside internal/control, which is
fully covered. They sit at the root of this world's module, so unit and coverage-inclusion run exactly as registered
and the instrument's real default scope (./internal/...) finds them. By hand: 8 * 10000 // 9 = 8888, so 88.88%.
"""

from gate.proofs.drivers import support

BELOW = "FAIL github.com/vorsar/cep/gate/proofs/internal/below 88.88% (8/9) < 90%"
CONTROL = "PASS github.com/vorsar/cep/gate/proofs/internal/control 100.00% (4/4) >= 90%"


class CoverageBelow(support.DriverTest):
    def test_eight_of_nine_statements_fail_the_floor(self):
        self.world.copy_gate()
        self.world.copy_proofs_module("below", "control")
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.UNIT_ROW, support.COVERAGE_ROW))

        self.assertParityPassed(run, "test")
        self.assertRow(run, "test", "unit", True, "covered 2")
        self.assertRed(run, "test", "coverage-inclusion", f"exit 1: {BELOW.removeprefix('FAIL ')}")
        self.assertEqual(run.row_stdout("test", "coverage-inclusion").split("\n")[:2], [BELOW, CONTROL])
        record = run.record("test", "coverage-inclusion")
        self.assertEqual((record["covered"], record["expected"]), (2, 2))
