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

ROW = "gofmt"  # the row its red cases land on: teeth.red_rows ties this proof to it (BK-S7)
# Planted in a package the import law lists, so archcheck (a T0 row since S1.1b) passes them and only gofmt judges.
BAD = "internal/event/bad.go"
GOOD = "internal/event/good.go"
UNFORMATTED = "package event\nfunc  F( ) {\n}\n"
FORMATTED = "package event\n\nfunc G() {}\n"
GUARD = "has unstaged changes: stage them or stash them"
# The gofmt row's reason and the guard's line, exactly (S1.1c-2 W0: a refusal is asserted by its reason).
GOFMT_REASON = f"exit 1: {BAD} is not formatted: fix it with gofmt -w {BAD}"
GUARD_LINE = f"{BAD} {GUARD}, so the check sees what you commit"


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

    def hook_run(self, code: int, out: str) -> support.Run:
        """The hook's own gate run, read from the run directory its summary names (`run: .gate/runs/<id>`)."""
        named = [line.removeprefix("run: ") for line in out.split("\n") if line.startswith("run: ")]
        self.assertEqual(len(named), 1, out)
        self.assertIn("LAW Δ", out)
        gate_out = out[out.index("LAW Δ") :]
        return support.Run(code=code, stdout=gate_out, run_dir=os.path.join(self.world.repo, named[0]))

    def head(self) -> str:
        return self.world.git("rev-parse", "HEAD").strip()

    def snapshot(self) -> dict[str, bytes]:
        """HEAD, the index's entries and every work-tree file outside .git/ and .gate/ (the runs), as bytes.

        The index's entries, not its raw bytes: `git commit` itself rewrites the index's cache data (its cache-tree
        extension) before the hook runs, whatever the hook does. What is staged must not change.
        """
        snap = {"HEAD": self.head().encode(), "index entries": self.world.git("ls-files", "-s", "-z").encode()}
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
        self.assertHookRefused((code, out), GOFMT_REASON)
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
        self.world.track("internal/event/other.go", FORMATTED.replace("func G", "func H"))
        code, out = self.git_commit()
        self.assertEqual(code, 0, out)
        self.assertNotEqual(self.head(), before, out)
        self.assertIn("GATE PASSED", out)
        self.assertIn("exit: 0", out)
        run = self.hook_run(code, out)
        self.assertRow(run, "lint", "gofmt", True, "covered 1")
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], out)

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
        # The skip is the scope's, not a dead row: the same hooks still refuse an unformatted file staged next
        # (S1.1c-2 design §4.2: a skip alone proves no row works).
        self.world.track(BAD, UNFORMATTED)
        self.assertRefusedUnformatted()

    def test_a_staged_file_with_unstaged_changes_is_refused_by_the_guard(self):
        self.world.track(BAD, FORMATTED)
        self.world.write(BAD, FORMATTED + "\n// more\n")
        before = self.snapshot()
        code, out = self.git_commit()
        self.assertNotEqual(code, 0, out)
        self.assertIn(f"{BAD} {GUARD}", out)
        self.assertNotIn("GATE PASSED", out)
        self.assertNotIn("| step", out, "the guard refuses before any row runs: no summary table")
        self.assertHookRefused((code, out), GUARD_LINE)
        self.assertEqual(self.snapshot(), before)

    def test_a_no_op_pre_commit_hook_fails_this_proof(self):
        """N27: the refusal assertion is a real witness: with the launcher replaced by `exit 0` it must go red."""
        launcher = os.path.join(self.world.repo, ".githooks", "pre-commit")
        with open(launcher, "rb") as f, open(os.path.join(support.REPO, ".githooks", "pre-commit"), "rb") as g:
            real = g.read()
            self.assertEqual(f.read(), real, "the world's launcher is the real one before the plant")
        with open(launcher, "w", encoding="utf-8") as f:
            f.write("#!/bin/sh\nexit 0\n")
        os.chmod(launcher, os.stat(launcher).st_mode | stat.S_IXUSR)
        self.world.track(BAD, UNFORMATTED)
        with self.assertRaises(AssertionError):
            self.assertRefusedUnformatted()
        self.assertEqual(self.world.git("ls-tree", "--name-only", "HEAD", "--", BAD).strip(), BAD)
        # The witness both ways: with the real launcher back, the next unformatted change is refused again. This is
        # the case's red verdict; the one under assertRaises above never counts (S1.1c-2's fix round, BK-B3).
        with open(launcher, "wb") as f:
            f.write(real)
        self.world.track(BAD, UNFORMATTED + "\n// staged again\n")
        self.assertRefusedUnformatted()
