"""BA13's proofs (S1.1c-2 lane D; design §4.3; ADR-0015 §3.6): the pytests row trusts only the report file the shim
writes, never a module's stdout.

The world holds the gate and one planted test module. The stdout forge prints the shim's pass line and ends the
process with os._exit(0) before the shim reports: red, the shim made no report. The file forge writes that line to
the report file itself, sys.argv[3], and exits 0: ADR-0015 §3.6 names it as the review-only residue (every test
module is a law path, so such a module is a named LAW Δ line), and it is documented here as green, so a change that
closes it, or one that breaks the report file, is seen.

Before lane D's fix both cases fail: the stdout forge passes the row (BA13 itself), and the file forge is an import
error, the shim taking two arguments (sys.argv[3] does not exist).
"""

from gate.proofs.drivers import support

ROW = "pytests"
JOB = "test"
PLANT = "gate/runner/tests/test_planted_forge.py"
NAME = "gate.runner.tests.test_planted_forge"
PASS_LINE = f"pyshim: {NAME}: passed 1 tests"
STDOUT_FORGE = f"import os\nimport sys\n\nprint({PASS_LINE!r})\nsys.stdout.flush()\nos._exit(0)\n"
FILE_FORGE = (
    "import os\nimport sys\n\n"
    'with open(sys.argv[3], "w", encoding="utf-8") as f:\n'
    f"    f.write({PASS_LINE!r} + " + '"\\n")\n'
    "os._exit(0)\n"
)
NO_REPORT = f"{PLANT}: exit 0, and the shim made no report"


class ShimForgedPass(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate()
        self.world.copy("gate/runner/tests/__init__.py")

    def run_row(self, module: str) -> support.Run:
        self.world.write(PLANT, module)
        self.world.commit()
        # The real row, its floor set to the world's one module: gate/registry.json's min_wants counts the real
        # tree's modules (82 since S1.1c-2's instruments); gate/law.json's instruments pin that floor, not this proof.
        row = dict(support.registry_row(ROW), min_wants=1)
        return self.world.run(registry=self.world.registry(row))

    def test_a_pass_line_forged_on_stdout_is_red(self):
        run = self.run_row(STDOUT_FORGE)
        self.assertRed(run, JOB, ROW, f"exit 1: {NO_REPORT}")
        self.assertIn(PASS_LINE, run.row_file(JOB, ROW, "py", f"{NAME}.stdout"), "the forge printed its line")

    def test_a_pass_line_forged_into_the_report_file_is_the_named_residue(self):
        # ADR-0015 §3.6's residue (owner, 2026-11-12, design 2): a module that writes the report path from
        # sys.argv[3] itself passes; review catches it, since every test module is a law path (a LAW Δ line).
        run = self.run_row(FILE_FORGE)
        self.assertRow(run, "test", "pytests", True, "covered 1")
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)
