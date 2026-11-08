"""The apidiff row over fixture modules (design s3-3-1 §6.4 step 7; QA's T35): the verdict is read from the tool's
report text, never its exit code; no merge base is a stated pass, not a skip; an incompatible change is red only
while api/exports.golden is unchanged against the merge base; a compatible change passes with its report printed.

Each test owns a repository: one commit, `main` at it, then one change committed on another branch. The module is a
throwaway (`ex.com/m`) carrying the real go.mod's require and tool lines and the real go.sum, so the module's own
`go tool apidiff` builds from the module cache; its one package is `p`.
"""

import os
import sys

from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
BASE = "package p\n\n// F is the base's one function.\nfunc F() {}\n"
INCOMPATIBLE = "package p\n\n// F gained a parameter.\nfunc F(extra int) {}\n"
COMPATIBLE = BASE + "\n// G is new.\nfunc G() {}\n"
GOLDEN = "api/exports.golden"
NO_MERGE_BASE = "apidiff: no merge base with main: nothing compared"
IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@t.invalid", "-c", "commit.gpgsign=false")


def wrapper(*args):
    return (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "apidiff.py"), *args)


def real(rel: str) -> str:
    with open(os.path.join(support.REPO, rel), encoding="utf-8") as f:
        return f.read()


def throwaway_module() -> str:
    text = real("go.mod")
    assert text.startswith("module github.com/vorsar/cep\n"), text
    return text.replace("module github.com/vorsar/cep\n", "module ex.com/m\n", 1)


class Apidiff(support.GateTest):
    def setUp(self):
        super().setUp()
        self.toolchain.dirs = [support.tool_bin("go")]

    def commit(self, message: str) -> None:
        self.git("add", "-A")
        self.git(*IDENTITY, "commit", "-q", "-m", message)

    def base(self) -> None:
        self.track("go.mod", throwaway_module())
        self.track("go.sum", real("go.sum"))
        self.track("p/p.go", BASE)
        self.track(GOLDEN, "p F func func F()\n")
        self.commit("base")

    def change(self, source: str, golden: str | None = None) -> None:
        self.git("checkout", "-q", "-b", "change")
        self.git("branch", "-f", "main", "HEAD")
        self.write("p/p.go", source)
        if golden is not None:
            self.write(GOLDEN, golden)
        self.commit("change")

    def apidiff(self) -> str:
        row = self.row(
            "apidiff",
            command=wrapper("./p"),
            expected_scope=("go", "./p"),
            scope=("prod",),
            timeout_s=300,
            exclusive=True,
        )
        self.run_rows([row])
        return self.reason("apidiff")

    def stdout(self) -> str:
        return self.row_file("lint", "apidiff", "stdout").decode("utf-8", errors="replace")

    def test_no_merge_base(self):
        # One commit, the branch named neither main nor tracking an origin: nothing to compare, and a stated pass.
        self.base()
        self.git("branch", "-M", "work")
        self.write("p/p.go", INCOMPATIBLE)
        self.commit("a change nothing is compared against")
        self.assertEqual(self.apidiff(), "covered 1")
        self.assertIn(NO_MERGE_BASE, self.stdout())
        self.assertNotIn("Incompatible", self.stdout())

    def test_incompatible_without_golden_is_red(self):
        self.base()
        self.change(INCOMPATIBLE)
        reason = self.apidiff()
        self.assertTrue(reason.startswith("exit 1: incompatible API change"), reason)
        self.assertIn(f"without {GOLDEN} changing", reason)
        self.assertIn("Incompatible changes:", reason)
        self.assertIn("F: changed from func() to func(int)", self.stdout())

    def test_incompatible_with_golden_passes(self):
        self.base()
        self.change(INCOMPATIBLE, golden="p F func func F(extra int)\n")
        self.assertEqual(self.apidiff(), "covered 1")
        self.assertIn("Incompatible changes:", self.stdout())
        self.assertIn("F: changed from func() to func(int)", self.stdout())

    def test_compatible_passes(self):
        self.base()
        self.change(COMPATIBLE)
        self.assertEqual(self.apidiff(), "covered 1")
        self.assertIn("Compatible changes:", self.stdout())
        self.assertIn("G: added", self.stdout())
        self.assertNotIn("Incompatible", self.stdout())
