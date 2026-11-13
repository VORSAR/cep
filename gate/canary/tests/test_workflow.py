"""The canary's workflow as generate.targets renders it (S1.1c-4 design §4.7; plan check §6 canary (9), (14), (15);
reviews A-S4, A-S5, DA-S7, DA-S8). F2 builds render_canary in W3 and pins the same in test_generate.py; these are W0c's
red-first statements over the real registry. Red today: targets() renders no canary.yml (KeyError)."""

import os
import re
import unittest

from gate.ci import generate
from gate.runner import registry

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
CANARY = ".github/workflows/canary.yml"


def jobs_of(text: str) -> dict[str, str]:
    body = text.split("\njobs:\n", 1)[1]
    parts = re.split(r"^  ([a-z][a-z0-9-]*):\n", body, flags=re.M)
    return dict(zip(parts[1::2], parts[2::2], strict=True))


def runs_on(body: str) -> str:
    return re.search(r"^    runs-on: (.+)$", body, flags=re.M).group(1)


class Workflow(unittest.TestCase):
    def setUp(self) -> None:
        self.rows = registry.load(os.path.join(REPO, "gate", "registry.json"))
        rendered = dict(generate.targets(self.rows))
        self.gate = rendered[generate.WORKFLOW]
        self.canary = rendered[CANARY]

    def test_t4_jobs_equal_the_gates_on_the_same_runners(self):
        # Case (14), G1-Q2: the T4 jobs exactly as the gate runs them, plus candidate and summary.
        t4 = list(dict.fromkeys(r.job for r in registry.select(self.rows, "T4", None)))
        gate, canary = jobs_of(self.gate), jobs_of(self.canary)
        self.assertEqual(set(canary), {*t4, "candidate", "summary"})
        for job in t4:
            with self.subTest(job=job):
                self.assertEqual(runs_on(canary[job]), runs_on(gate[job]))
                self.assertIn("needs: candidate", canary[job])

    def test_its_own_concurrency_group_never_cancelled(self):
        self.assertIn("  group: canary-${{ github.ref }}\n  cancel-in-progress: false\n", self.canary)

    def test_triggers_and_permissions(self):
        head = self.canary.split("\njobs:\n", 1)[0]
        self.assertIn("    - cron: '0 6 * * 1'", head)
        self.assertIn("  workflow_dispatch:", head)
        self.assertIn("gate/canary/**", head)
        self.assertIn("permissions:\n  contents: read\n", head)

    def test_token_only_on_the_feeds_step(self):
        # Case (15), reviews A-S5, DA-S7.
        self.assertEqual(self.canary.count("GITHUB_TOKEN"), 1, self.canary)
        step = next(s for s in self.canary.split("\n      - ") if "GITHUB_TOKEN" in s)
        self.assertIn("canary-feeds", step)
        self.assertNotIn("canary-candidate", step)

    def test_caches_restored_never_saved(self):
        self.assertNotRegex(self.canary, r"uses: actions/cache@")
        self.assertIn("actions/cache/restore@", self.canary)

    def test_summary_reads_needs_and_runs_always(self):
        summary = jobs_of(self.canary)["summary"]
        self.assertIn("if: always()", summary)
        self.assertIn("${{ toJSON(needs) }}", summary)

    def test_ci_fresh_checks_the_canary(self):
        # Case (9): the workflow differing from generate's output is ci-fresh's red only if ci-fresh reads it.
        row = next(r for r in self.rows if r.id == "ci-fresh")
        self.assertIn(CANARY, row.expected_scope)
        self.assertEqual(row.min_wants, 3)


if __name__ == "__main__":
    unittest.main()
