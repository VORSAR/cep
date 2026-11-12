"""The docsync row's planted proof, part (a) (S1.1c-2 lane D; design §4.1): a Go package ARCHITECTURE §3 does not
list is red, naming it.

The world holds the gate, the module's go.mod and go.sum, and docs/architecture/ARCHITECTURE.md. Planted: a new
package internal/newpkg (red), and, as the control, a file in internal/event, a package §3 lists (green). Until lane D
lands every case is red on the G0 stub's `exit 1: docsync: not built yet (lane D)`.
"""

from gate.proofs.drivers import support

ROW = "docsync"
JOB = "lint"
ARCHITECTURE = "docs/architecture/ARCHITECTURE.md"
NEWPKG = "internal/newpkg"
LISTED = "internal/event"
# design §4.1, exactly
NEWPKG_RED = f"docsync: {NEWPKG} is a Go package and ARCHITECTURE §3 does not list it"


class DocsyncPlanted(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate()
        for rel in ("go.mod", "go.sum", ARCHITECTURE):
            self.world.copy(rel)
        self.assertIn(f"\n{LISTED}/ ", support.read_text(f"{support.REPO}/{ARCHITECTURE}"), "the control moved")

    def run_row(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row(ROW)))

    def test_a_package_section_3_does_not_list_is_red(self):
        self.world.write(f"{LISTED}/x.go", "package event\n")
        self.world.write(f"{NEWPKG}/x.go", "package newpkg\n")
        run = self.run_row()
        self.assertRed(run, JOB, ROW, f"exit 1: {NEWPKG_RED}")
        out = run.row_stdout(JOB, ROW)
        failed = [x.removeprefix("failed: ") for x in out.split("\n") if x.startswith("failed: ")]
        self.assertEqual(failed, [NEWPKG_RED], out)

    def test_a_listed_package_is_green(self):
        self.world.write(f"{LISTED}/x.go", "package event\n")
        run = self.run_row()
        self.assertRow(run, JOB, ROW, True, "covered 1")
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)
