"""The coverage-inclusion row in the real runner, after the real unit row (design note §5; plan check S3d, S4).

The runner's live expansion of the row's scope is the expected count, so the instrument's own scope is checked
against it on every run (QA Q16). `needs: unit` orders the two rows and never stops this one: a unit row that wrote
nothing still lets it run, and it names unit (design-QA counter-example 34).
"""

import os

from gate.proofs.drivers import support

CONTROL = "PASS github.com/vorsar/cep/gate/proofs/internal/control 100.00% (4/4) >= 90%"
RULE_X = "PASS github.com/vorsar/cep/gate/proofs/rule/x 100.00% (1/1) >= 90%"


class Row(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        self.world.copy_proofs_module("control")

    def run_rows(self, *rows: dict) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(*rows))

    def test_the_count_equals_the_runners_own_expansion(self):
        run = self.run_rows(support.UNIT_ROW, support.COVERAGE_ROW)
        self.assertParityPassed(run, "test")
        self.assertRow(run, "test", "unit", True, "covered 1")
        self.assertRow(run, "test", "coverage-inclusion", True, "covered 1")
        self.assertEqual((run.tail(), run.code), (["GATE PASSED", "exit: 0"], 0))
        self.assertEqual(run.row_stdout("test", "coverage-inclusion").split("\n"), [CONTROL, "GATE_COVERED 1", ""])
        record = run.record("test", "coverage-inclusion")
        self.assertEqual((record["covered"], record["expected"]), (1, 1))

    def test_needs_unit_starts_this_row_only_after_unit_ends(self):
        trace = self.run_rows(support.UNIT_ROW, support.COVERAGE_ROW).trace()
        self.assertLess(trace.index(("end", "unit")), trace.index(("start", "coverage-inclusion")))

    def test_a_new_package_under_rule_joins_the_scope_with_no_edit(self):
        # C30, design-QA counter-example 3: `.`, ./rule/... and ./io/... matched nothing; now ./rule/... matches.
        self.world.write("rule/x/x.go", "package x\n\n// One is 1.\nfunc One() int { return 1 }\n")
        self.world.write(
            "rule/x/x_test.go",
            'package x\n\nimport "testing"\n\nfunc TestOne(t *testing.T) {\n\tif One() != 1 {\n\t\tt.Fail()\n\t}\n}\n',
        )
        run = self.run_rows(support.UNIT_ROW, support.COVERAGE_ROW)
        self.assertRow(run, "test", "coverage-inclusion", True, "covered 2")
        self.assertEqual(run.row_stdout("test", "coverage-inclusion").split("\n")[:2], [CONTROL, RULE_X])

    def test_a_unit_row_that_wrote_nothing_still_lets_this_row_run_and_it_names_unit(self):
        # design-QA counter-example 34: unit dies before writing cover.prof.
        unit = {**support.UNIT_ROW, "command": ["python3.15", "-I", "-B", "-c", "raise SystemExit(1)"]}
        run = self.run_rows(unit, support.COVERAGE_ROW)
        self.assertParityPassed(run, "test")
        self.assertRed(run, "test", "unit", "exit 1")
        self.assertRow(run, "test", "coverage-inclusion", False, "exit 1")
        unit_dir = os.path.join(run.run_dir, "rows", "test", "unit")
        self.assertEqual(
            run.row_stdout("test", "coverage-inclusion").split("\n"),
            [
                f"FAIL {unit_dir}/cover.prof is missing: the unit row did not write it",
                f"FAIL {unit_dir}/test.json is missing: the unit row did not write it",
                "GATE_COVERED 0",
                "",
            ],
        )
