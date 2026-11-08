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
NIGHTLY = [*ROWS, row("fuzz-parser", "nightly", ("nightly",))]


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

    def test_the_job_running_apidiff_checks_out_mains_history(self):
        # S3.3-1: the apidiff row compares against the merge base with main; a shallow checkout has no main, so the
        # row would pass on every CI run with nothing compared. Only that job pays for the full history.
        text = generate.render([*ROWS, row("apidiff", "lint", ("T4",))])
        lint = text[text.index("  lint:") : text.index("  test:")]
        self.assertIn("          fetch-depth: 0", lint)
        self.assertNotIn("fetch-depth", text[text.index("  test:") :])

    def test_the_tool_cache_is_where_a_developer_has_it(self):
        # First CI run (2026-11-04): a cache inside the checkout put the project's go.mod above every tool, so go
        # followed its toolchain line and a parity witness passed differently on CI than on a developer's machine.
        self.assertNotIn("VORSAR_CEP_TOOLS_CACHE", self.text)
        self.assertNotIn(".tools-cache", self.text)
        self.assertIn("path: ~/.cache/vorsar/cep/tools", self.text)

    def test_a_nightly_tier_gives_a_second_scheduled_workflow_with_only_its_rows(self):
        # Owner, 2026-11-04 (S2.2a): the nightly workflow is generated like the gate's, from the registry.
        targets = dict(generate.targets(NIGHTLY))
        self.assertEqual(sorted(targets), [".github/workflows/gate.yml", ".github/workflows/nightly.yml"])
        gate, nightly = targets[".github/workflows/gate.yml"], targets[".github/workflows/nightly.yml"]
        self.assertEqual(gate, generate.render(ROWS))  # the pull-request workflow never sees a nightly row
        jobs = re.findall(r"^  ([a-z][a-z0-9-]*):$", nightly[nightly.index("\njobs:\n") :], re.M)
        self.assertEqual(jobs, ["nightly", "strict-gate"])
        for needle in ("name: nightly", "schedule:", "workflow_dispatch:", "--tier nightly --job nightly",
                       "summarize --registry gate/registry.json --tier nightly"):  # fmt: skip
            with self.subTest(needle):
                self.assertIn(needle, nightly)
        self.assertNotIn("pull_request", nightly)

    def test_the_summary_downloads_each_jobs_records_by_name_into_its_own_directory(self):
        # The nightly's first scheduled run (2026-11-05) failed: download-artifact with a pattern extracts a single
        # match straight into its path, but each of several into a subdirectory, so the one-job nightly's records
        # never reached summarize's records-<job>-<attempt>/ layout. One download per job, by name, into its own
        # directory, gives that layout whatever the job count.
        nightly = dict(generate.targets(NIGHTLY))[".github/workflows/nightly.yml"]
        for name, text, jobs in (("gate", self.text, ["lint", "test", "proofs"]), ("nightly", nightly, ["nightly"])):
            block = text[text.index("  strict-gate:") :]
            with self.subTest(name):
                self.assertNotIn("pattern:", block)
                for job in jobs:
                    self.assertIn(f"name: records-{job}-${{{{ github.run_attempt }}}}", block)
                    self.assertIn(f"path: .gate/artifacts/records-{job}-${{{{ github.run_attempt }}}}", block)
                self.assertEqual(block.count("download-artifact@"), len(jobs))

    def test_the_macos_job_runs_on_macos_and_every_other_on_linux(self):
        # S3.2b-2 (S3.2b's item 8; owner Q5: the second OS of the determinism proof is macOS).
        text = generate.render([*ROWS, row("determinism-macos", "determinism-macos", ("T4",))])
        runners = dict(re.findall(r"^  ([a-z][a-z0-9-]*):\n    runs-on: (\S+)$", text, re.M))
        self.assertEqual(runners["determinism-macos"], "macos-latest")
        self.assertEqual(
            {j: r for j, r in runners.items() if j != "determinism-macos"},
            {
                "lint": "ubuntu-latest",
                "test": "ubuntu-latest",
                "proofs": "ubuntu-latest",
                "strict-gate": "ubuntu-latest",
            },
        )

    def test_without_nightly_rows_there_is_one_workflow(self):
        self.assertEqual([p for p, _ in generate.targets(ROWS)], [".github/workflows/gate.yml"])

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
