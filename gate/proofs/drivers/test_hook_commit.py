"""The hook proof (exit line 5; design §12.1, §12.4; plan check §7, QA Q2, N8, N9, N27, N35).

The world holds real, byte-for-byte copies of the gate, the .githooks/ launchers, gate/hooks/ and the Makefile, and
turns the hooks on through its real `make setup`. Every commit is a real `git commit` in an isolated git: no
inherited GIT_* variable, its own HOME, no global or system config, no signing. An unformatted staged Go file is
refused under plain `commit`, `commit -a` and `commit <path>`, naming gofmt, the file and its fix, and a refused
commit leaves the index, the work tree and HEAD byte-unchanged. A pre-commit hook replaced by `exit 0` must turn the
refusal assertion red, so a no-op hook can never pass this proof.
"""

import os
import stat

from gate.proofs.drivers import support

BAD = "pkg/bad.go"
GOOD = "pkg/good.go"
UNFORMATTED = "package pkg\nfunc  F( ) {\n}\n"
FORMATTED = "package pkg\n\nfunc F() {}\n"
GUARD = "has unstaged changes: stage them or stash them"


class HookCommit(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate()
        support.copy_hooks(self.world)
        self.world.write(GOOD, FORMATTED)
        self.world.write("docs/note.md", "a note\n")
        self.world.commit()  # the base, before the hooks are on
        self.world.env = support.isolated_git_env(self.world)
        self.world.git("config", "user.name", "gate proofs")
        self.world.git("config", "user.email", "proofs@gate.invalid")
        self.world.git("config", "commit.gpgsign", "false")
        support.hooks_on(self.world)
        self.assertEqual(self.world.git("config", "core.hooksPath").strip(), ".githooks")
        self.calls = 0

    def git_commit(self, *args: str) -> tuple[int, str]:
        """A real `git commit` (hooks and all): its exit code and its whole output, never raising."""
        self.calls += 1
        out = os.path.join(self.world.root, f"commit{self.calls}.stdout")
        err = os.path.join(self.world.root, f"commit{self.calls}.stderr")
        argv = ["git", "commit", "-m", f"commit {self.calls}", *args]
        code = support.run_process(argv, cwd=self.world.repo, env=self.world.env, out=out, err=err, cap_s=900)
        return code, support.read_text(out) + support.read_text(err)

    def head(self) -> str:
        return self.world.git("rev-parse", "HEAD").strip()

    def snapshot(self) -> dict[str, bytes]:
        """HEAD, .git/index and every work-tree file outside .git/ and .gate/ (the runs), as bytes."""
        snap = {"HEAD": self.head().encode()}
        with open(os.path.join(self.world.repo, ".git", "index"), "rb") as f:
            snap[".git/index"] = f.read()
        for top, dirs, files in os.walk(self.world.repo):
            dirs[:] = sorted(d for d in dirs if not (top == self.world.repo and d in (".git", ".gate")))
            for name in files:
                path = os.path.join(top, name)
                with open(path, "rb") as f:
                    snap[os.path.relpath(path, self.world.repo)] = f.read()
        return snap

    def assertRefusedUnformatted(self, *args: str) -> None:
        before = self.snapshot()
        code, out = self.git_commit(*args)
        self.assertNotEqual(code, 0, out)
        self.assertIn("gofmt", out)
        self.assertIn(BAD, out)
        self.assertIn(f"gofmt -w {BAD}", out)
        self.assertEqual(self.snapshot(), before, "a refused commit changed the index, the work tree or HEAD")

    def test_an_unformatted_staged_file_is_refused(self):
        self.world.track(BAD, UNFORMATTED)
        self.assertRefusedUnformatted()

    def test_commit_a_refuses_an_unformatted_tracked_file(self):
        self.world.track(BAD, FORMATTED)
        self.world.git("commit", "--no-verify", "-q", "-m", "formatted")
        self.world.write(BAD, UNFORMATTED)
        self.assertRefusedUnformatted("-a")

    def test_commit_path_refuses_an_unformatted_tracked_file(self):
        self.world.track(BAD, FORMATTED)
        self.world.git("commit", "--no-verify", "-q", "-m", "formatted")
        self.world.write(BAD, UNFORMATTED)
        self.assertRefusedUnformatted("--", BAD)

    def test_a_formatted_file_commits(self):
        """The positive control: the same hooks let a clean commit through, and the gate ran to say so."""
        before = self.head()
        self.world.track("pkg/other.go", FORMATTED.replace("func F", "func G"))
        code, out = self.git_commit()
        self.assertEqual(code, 0, out)
        self.assertNotEqual(self.head(), before, out)
        self.assertIn("GATE PASSED", out)
        self.assertIn("exit: 0", out)

    def test_a_docs_only_commit_passes_with_rows_skipped(self):
        before = self.head()
        self.world.track("docs/note.md", "a changed note\n")
        code, out = self.git_commit()
        self.assertEqual(code, 0, out)
        self.assertNotEqual(self.head(), before, out)
        gofmt = [line for line in out.split("\n") if [c.strip() for c in line.split(" | ")][:2] == ["lint", "gofmt"]]
        self.assertEqual(len(gofmt), 1, out)
        cells = [c.strip() for c in gofmt[0].split(" | ", 4)]
        self.assertEqual(cells[2], "SKIP", out)
        self.assertTrue(cells[4].startswith("skip: nothing staged in scope ("), out)
        self.assertIn("GATE PASSED", out)

    def test_a_staged_file_with_unstaged_changes_is_refused_by_the_guard(self):
        self.world.track(BAD, FORMATTED)
        self.world.write(BAD, FORMATTED + "\n// more\n")
        before = self.snapshot()
        code, out = self.git_commit()
        self.assertNotEqual(code, 0, out)
        self.assertIn(f"{BAD} {GUARD}", out)
        self.assertNotIn("GATE PASSED", out)
        self.assertNotIn("GATE FAILED", out, "the guard refuses before any row runs")
        self.assertEqual(self.snapshot(), before)

    def test_a_no_op_pre_commit_hook_fails_this_proof(self):
        """N27: the refusal assertion is a real witness: with the launcher replaced by `exit 0` it must go red."""
        launcher = os.path.join(self.world.repo, ".githooks", "pre-commit")
        with open(launcher, "rb") as f, open(os.path.join(support.REPO, ".githooks", "pre-commit"), "rb") as g:
            self.assertEqual(f.read(), g.read(), "the world's launcher is the real one before the plant")
        with open(launcher, "w", encoding="utf-8") as f:
            f.write("#!/bin/sh\nexit 0\n")
        os.chmod(launcher, os.stat(launcher).st_mode | stat.S_IXUSR)
        self.world.track(BAD, UNFORMATTED)
        with self.assertRaises(AssertionError):
            self.assertRefusedUnformatted()
        self.assertEqual(self.world.git("ls-tree", "--name-only", "HEAD", "--", BAD).strip(), BAD)
