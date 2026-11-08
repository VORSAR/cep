"""QA's break-it on S3.3-1's apidiff row (step 7): a package removed at HEAD, and a module renamed.

The fixtures are test_apidiff's: a throwaway module carrying the real go.mod's require and tool lines and go.sum;
`main` at the base commit, the change on another branch.
"""

from gate.runner.tests import support
from gate.runner.tests.test_apidiff import BASE, GOLDEN, IDENTITY, real, throwaway_module, wrapper


class ApidiffBreakit(support.GateTest):
    def setUp(self):
        super().setUp()
        self.toolchain.dirs = [support.tool_bin("go")]

    def commit(self, message: str) -> None:
        self.git("add", "-A")
        self.git(*IDENTITY, "commit", "-q", "-m", message)

    def base(self, *, with_q: bool = False) -> None:
        self.track("go.mod", throwaway_module())
        self.track("go.sum", real("go.sum"))
        self.track("p/p.go", BASE)
        golden = "p F func func F()\n"
        if with_q:
            self.track("q/q.go", "package q\n\n// H is q's one function.\nfunc H() {}\n")
            golden += "q H func func H()\n"
        self.track(GOLDEN, golden)
        self.commit("base")
        self.git("checkout", "-q", "-b", "change")
        self.git("branch", "-f", "main", "HEAD")

    def apidiff_of(self, *pkgs: str) -> str:
        row = self.row(
            "apidiff",
            command=wrapper(*pkgs),
            expected_scope=("go", *pkgs),
            scope=("prod",),
            timeout_s=300,
            exclusive=True,
        )
        self.run_rows([row])
        return self.reason("apidiff")

    def stdout(self) -> str:
        return self.row_file("lint", "apidiff", "stdout").decode("utf-8", errors="replace")

    def test_removed_package_with_golden_changed_passes(self):
        # A new package is handled ("new package (absent at the merge base): compatible"); a removed one is not: the
        # HEAD side's apidiff cannot load it and the row is red with "apidiff at HEAD failed", even though the law
        # (api/exports.golden) changed with the removal. Want: the removal reported as incompatible, and the row
        # passing because the golden changed, as an incompatible signature change with the golden changed does.
        self.base(with_q=True)
        self.git("rm", "-q", "q/q.go")
        self.write(GOLDEN, "p F func func F()\n")
        self.commit("remove q")
        reason = self.apidiff_of("./p", "./q")
        self.assertEqual(reason, "covered 2", f"{reason}\n--- stdout:\n{self.stdout()}")

    def test_removed_package_without_golden_names_the_removal(self):
        # The same removal with the golden unchanged is red, and its reason says the package was removed, not that a
        # tool failed.
        self.base(with_q=True)
        self.git("rm", "-q", "q/q.go")
        self.commit("remove q")
        reason = self.apidiff_of("./p", "./q")
        self.assertTrue(reason.startswith("exit 1:"), reason)
        self.assertNotIn("failed with exit", reason)
        self.assertIn("removed", reason)

    def test_renamed_module_is_compared_not_a_tool_failure(self):
        # The module line renamed (every import path changes; the golden, which names packages, does not): the row
        # runs `apidiff -w` at the merge base with HEAD's import path, which the base module does not have, and is red
        # with "apidiff -w at the merge base failed" on every run of the branch. Want: a stated verdict on the rename
        # (each import path removed and added: incompatible), not a tool failure.
        self.base()
        self.write("go.mod", throwaway_module().replace("module ex.com/m\n", "module ex.com/n\n", 1))
        self.commit("rename the module")
        reason = self.apidiff_of("./p")
        self.assertNotIn("at the merge base failed", reason, f"{reason}\n--- stdout:\n{self.stdout()}")
