"""The ci-fresh row's planted proofs (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10: every row
without a planted proof gets one).

The world holds the gate with its real registry and generator, and both real workflows, which the registry generates
byte for byte. Planted: a hand edit to one line of .github/workflows/gate.yml; and, in a second world, nightly.yml
never tracked. The ci-fresh row fails naming the workflow and the fix.
"""

import os

from gate.proofs.drivers import support

GATE_YML = ".github/workflows/gate.yml"
NIGHTLY_YML = ".github/workflows/nightly.yml"
ANCHOR = "    runs-on: ubuntu-latest\n"
REGISTRY = "gate/registry.json"
FIX = "run make ci-workflow"


class CiFreshPlanted(support.DriverTest):
    def run_row(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row("ci-fresh")))

    def test_a_hand_edited_workflow_is_red(self):
        self.world.copy_gate()
        self.world.copy(NIGHTLY_YML)
        text = support.read_text(os.path.join(support.REPO, GATE_YML))
        self.assertIn(ANCHOR, text, "the plant's anchor moved")
        self.world.track(GATE_YML, text.replace(ANCHOR, "    runs-on: ubuntu-22.04\n", 1))

        run = self.run_row()

        self.assertParityPassed(run, "lint")
        self.assertRed(run, "lint", "ci-fresh", f"exit 1: {GATE_YML} is not what {REGISTRY} generates: {FIX}")

    def test_an_untracked_workflow_is_red(self):
        self.world.copy_gate()
        self.world.copy(GATE_YML)

        run = self.run_row()

        self.assertParityPassed(run, "lint")
        self.assertRed(run, "lint", "ci-fresh", f"exit 1: {NIGHTLY_YML} is not tracked: {FIX} and git add it")
