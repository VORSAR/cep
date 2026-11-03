"""The `ci-fresh` row (gate design §12.6–12.7): the tracked workflow is exactly what the registry generates."""

import os
import sys

from gate.ci import generate
from gate.runner import registry
from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
WORKFLOW = ".github/workflows/gate.yml"


class CiFresh(support.GateTest):
    def setUp(self):
        super().setUp()
        rows = [self.row("x", tiers=("T2", "T4")), self.row("y", job="test", tiers=("T4",))]
        self.reg_path = self.registry_file(rows)
        os.makedirs(os.path.join(self.repo, "gate"))
        with open(self.reg_path, "rb") as f, open(os.path.join(self.repo, "gate", "registry.json"), "wb") as g:
            g.write(f.read())
        self.track(WORKFLOW, generate.render(registry.load(self.reg_path)))
        self.git("add", "--", "gate/registry.json")

    def ci_fresh(self):
        command = (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "ci_fresh.py"))
        self.run_rows([self.row("ci-fresh", command=command, expected_scope=("git", WORKFLOW))])
        return self.reason("ci-fresh")

    def test_a_fresh_workflow_passes(self):
        self.assertEqual(self.ci_fresh(), "covered 1")

    def test_a_hand_edit_fails_naming_the_regenerate_command(self):
        # Plan check N6.
        path = os.path.join(self.repo, WORKFLOW)
        with open(path, "a", encoding="utf-8") as f:
            f.write("# edited\n")
        self.assertEqual(
            self.ci_fresh(), f"exit 1: {WORKFLOW} is not what gate/registry.json generates: run make ci-workflow"
        )
