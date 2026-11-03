"""The CI workflow, generated from the registry (gate design §12.6)."""

import re
import unittest

from gate.ci import generate
from gate.runner.registry import Row


def row(row_id, job, tiers=("T2", "T3", "T4")):
    fields = {"blocking": True, "timeout_s": 60, "needs": (), "scope": ("tools",), "expected_scope": ("git", "x")}
    fields.update(min_wants=0, lands_in="S1.1a-2", exclusive=False, command=("true",))
    return Row(id=row_id, job=job, tiers=tiers, **fields)


ROWS = [row("gofmt", "lint"), row("unit", "test"), row("proofs", "proofs", ("T4",)), row("t0only", "early", ("T0",))]


class Render(unittest.TestCase):
    def setUp(self):
        self.text = generate.render(ROWS)

    def test_the_same_registry_gives_the_same_bytes(self):
        self.assertEqual(generate.render(list(ROWS)), self.text)

    def test_one_job_per_registry_job_at_t4_in_registry_order(self):
        jobs = re.findall(r"^  ([a-z][a-z0-9-]*):$", self.text[self.text.index("\njobs:\n") :], re.M)
        self.assertEqual(jobs, ["lint", "test", "proofs", "strict-gate"])

    def test_strict_gate_needs_every_job_and_always_runs(self):
        block = self.text[self.text.index("  strict-gate:") :]
        self.assertIn("needs: [lint, test, proofs]", block)
        self.assertEqual(block.count("if: always()"), 2)  # the job, and the summarize step

    def test_every_action_is_pinned_by_a_full_sha(self):
        uses = re.findall(r"uses: (\S+)", self.text)
        self.assertTrue(uses)
        for ref in uses:
            self.assertRegex(ref, r"^actions/(checkout|cache|upload-artifact|download-artifact)@[0-9a-f]{40}$")

    def test_the_tool_cache_is_where_a_developer_has_it(self):
        # First CI run (2026-11-04): a cache inside the checkout put the project's go.mod above every tool, so go
        # followed its toolchain line and a parity witness passed differently on CI than on a developer's machine.
        self.assertNotIn("VORSAR_CEP_TOOLS_CACHE", self.text)
        self.assertNotIn(".tools-cache", self.text)
        self.assertIn("path: ~/.cache/vorsar/cep/tools", self.text)

    def test_the_posture(self):
        needles = (
            "runs-on: ubuntu-latest",
            "permissions:\n  contents: read",
            "persist-credentials: false",
            "cancel-in-progress: ${{ github.event_name == 'pull_request' }}",
            "make setup",
            "--run-id ${{ github.run_id }}-${{ github.run_attempt }}",
            "--commit ${{ github.sha }}",
        )
        for needle in needles:
            with self.subTest(needle):
                self.assertIn(needle, self.text)
        self.assertNotIn("paths:", self.text)
