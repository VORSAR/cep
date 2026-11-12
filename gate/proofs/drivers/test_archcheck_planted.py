"""The archcheck row's planted proof (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10: every row
without a planted proof gets one).

Planted: internal/event, which tools/importcheck/rules.json lets import no module package, importing internal/model.
The world holds the real tools/archcheck, its rules and the allowlists, the root go.mod, and a clean internal/model
that imports internal/event as the rules allow. The archcheck row fails naming the planted file, its line and the
imports check; the clean file adds nothing to the reason.
"""

from gate.proofs.drivers import support

PLANTED = "internal/event/planted.go"
SOURCE = (
    "package event\n"
    "\n"
    'import _ "github.com/vorsar/cep/internal/model" // planted: rules.json lets internal/event import nothing\n'
)
CLEAN = "internal/model/model.go"
CLEAN_SOURCE = (
    "// Package model is the archcheck proof's clean file: it imports what the rules allow it.\n"
    "package model\n"
    "\n"
    'import _ "github.com/vorsar/cep/internal/event"\n'
)
REASON = (
    "exit 1: internal/event/planted.go:3: imports: internal/event may not import internal/model "
    "(ARCHITECTURE §3; tools/importcheck/rules.json)"
)


class ArchcheckPlanted(support.DriverTest):
    def test_the_archcheck_row_catches_an_import_the_rules_forbid(self):
        self.world.copy_gate()
        for rel in [*self.world.tracked("tools/archcheck"), *self.world.tracked("tools/allowlists")]:
            if not rel.endswith("_test.go"):
                self.world.copy(rel)
        for rel in ("tools/importcheck/rules.json", "go.mod", "go.sum"):
            self.world.copy(rel)
        self.world.track(CLEAN, CLEAN_SOURCE)
        self.world.track(PLANTED, SOURCE)
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row("archcheck")))

        self.assertParityPassed(run, "lint")
        self.assertRed(run, "lint", "archcheck", REASON)
        out = run.row_stdout("lint", "archcheck")
        self.assertEqual([line for line in out.split("\n") if line.startswith("failed: ")], [f"failed: {REASON[8:]}"])
