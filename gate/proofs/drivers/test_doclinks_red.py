"""The doclinks row's proof (owner, 2026-11-06): a planted broken link and a planted broken anchor turn the row red.

The world holds a copy of every file the repository tracks, so the real docs' links resolve there as they do here.
Its clean run covers every Markdown file the row's real scope lists; a link to a file that does not exist, planted in
docs/README.md, turns the row red naming it, and so does a link to an anchor no heading makes.
"""

from gate.proofs.drivers import support

README = "docs/README.md"


class DoclinksRed(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.tracked = self.world.tracked(".")
        for rel in self.tracked:
            self.world.copy(rel)
        self.world.commit()

    def run_doclinks(self) -> support.Run:
        return self.world.run(registry=self.world.registry(support.registry_row("doclinks")))

    def plant(self, link: str) -> str:
        """Append `link` to the README as its own line; give the failure the row must report for it."""
        text = support.read_text(f"{self.world.repo}/{README}")
        line = text.count("\n") + 2
        self.world.track(README, f"{text}\n{link}\n")
        self.world.commit()
        return f"{README}:{line}: {link.split('](')[1].rstrip(')')}"

    def test_the_clean_docs_pass(self):
        docs = [
            p for p in self.tracked if (p.startswith("docs/") and p.endswith(".md")) or p in ("CLAUDE.md", "README.md")
        ]
        run = self.run_doclinks()
        self.assertParityPassed(run, "lint")
        self.assertRow(run, "lint", "doclinks", True, f"covered {len(docs)}")

    def test_a_planted_missing_file_is_red_and_named(self):
        want = self.plant("[planted](planted-missing.md)") + " (missing file)"
        run = self.run_doclinks()
        self.assertParityPassed(run, "lint")
        failed = [line for line in run.row_stdout("lint", "doclinks").split("\n") if line.startswith("failed: ")]
        self.assertEqual(failed, [f"failed: {want}"])
        self.assertRed(run, "lint", "doclinks", f"exit 1: {want}")

    def test_a_planted_missing_anchor_is_red_and_named(self):
        want = self.plant("[planted](sprints/sprint-plan.md#planted-anchor)") + " (missing anchor)"
        run = self.run_doclinks()
        self.assertParityPassed(run, "lint")
        self.assertRed(run, "lint", "doclinks", f"exit 1: {want}")
