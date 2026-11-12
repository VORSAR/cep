"""The vet row's planted proof (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10: every row without a
planted proof gets one).

Planted: a package whose fmt.Printf gives a %d verb a string argument. It compiles, so only vet sees it; go1.27.3's
`go vet -json` exits 0 with the diagnostic in its output, and the vet row fails naming the file, line, column and
vet's message. internal/control beside it vets clean, so the reason names the plant alone.
"""

import os

from gate.proofs.drivers import support

PLANTED = "internal/planted/planted.go"
SOURCE = (
    "// Package planted is the vet row's planted defect: a Printf verb that does not match its argument.\n"
    "package planted\n"
    "\n"
    'import "fmt"\n'
    "\n"
    "// Show prints s with an integer verb.\n"
    "func Show(s string) {\n"
    '\tfmt.Printf("%d\\n", s)\n'
    "}\n"
)
MESSAGE = "fmt.Printf format %d has arg s of wrong type string"


class VetPlanted(support.DriverTest):
    def test_the_vet_row_catches_a_printf_verb_that_does_not_match_its_argument(self):
        self.world.copy_gate()
        self.world.copy_proofs_module("control")
        self.world.track(PLANTED, SOURCE)
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row("vet")))

        self.assertParityPassed(run, "lint")
        posn = f"{os.path.join(self.world.repo, PLANTED)}:8:14"
        self.assertRed(run, "lint", "vet", f"exit 1: {posn}: {MESSAGE}")
