"""The coverage fixtures reach a clean clone (plan check C33, QA Q28): tracked, never ignored, never `*.out`.

.gitignore hides `*.out`, so a fixture named that way passes in the worktree that made it and is absent everywhere
else; a test that skipped a missing fixture would then pass vacuously.
"""

import os

from gate.coverage.tests import support

REL = os.path.relpath(support.TESTDATA, support.REPO)


class Fixtures(support.CoverageTest):
    def git(self, *args: str) -> tuple[int, str]:
        env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
        for name in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"):
            env.pop(name, None)
        code, out, _ = support.run(["git", *args], support.REPO, env, self.root, "git")
        return code, out

    def on_disk(self) -> list[str]:
        names = sorted(os.listdir(support.TESTDATA))
        self.assertTrue(names, "no fixtures: nothing was checked")
        return names

    def test_every_fixture_on_disk_is_tracked(self):
        code, out = self.git("ls-files", "-z", "--", REL)
        self.assertEqual(code, 0)
        tracked = sorted(os.path.basename(p) for p in out.split("\0") if p)
        self.assertEqual(self.on_disk(), tracked)

    def test_no_fixture_matches_an_ignore_rule(self):
        for name in self.on_disk():
            with self.subTest(name=name):
                code, out = self.git("check-ignore", "--no-index", "-v", "--", f"{REL}/{name}")
                self.assertEqual(code, 1, f"ignored: {out.strip()}")

    def test_no_fixture_is_named_out(self):
        self.assertEqual([n for n in self.on_disk() if n.endswith(".out")], [])

    def test_the_fixture_reader_refuses_out(self):
        self.write("x.out", "mode: set\n")
        with self.assertRaisesRegex(AssertionError, r"\.gitignore hides \*\.out"):
            support.fixture("x.out")
