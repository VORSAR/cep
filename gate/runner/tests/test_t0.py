"""T0 in place (gate design §12.3): scopes cut to the staged paths, and a row with none staged skips."""

from gate.runner.tests import support

IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@t.invalid", "-c", "commit.gpgsign=false")


class StagedScope(support.GateTest):
    def setUp(self):
        super().setUp()
        self.track("two.txt", "2\n")
        self.git(*IDENTITY, "commit", "-qm", "base")
        self.track("one.txt", "1 changed\n")  # staged; two.txt is tracked but unchanged

    def rows(self):
        return [self.row("a", expected_scope=("git", "one.txt")), self.row("b", expected_scope=("git", "two.txt"))]

    def test_a_row_with_nothing_staged_skips_and_t0_passes(self):
        result = self.run_rows(self.rows(), tier="T0")
        self.assertEqual(result.exit_code, 0, result.summary)
        self.assertEqual(self.reason("a"), "covered 1")
        self.assertEqual(self.reason("b"), "skip: nothing staged in scope (1 tracked)")
        self.assertEqual(self.record("lint", "b")["verdict"], "skip")
        line = next(x for x in result.summary.splitlines() if x.startswith("lint | b "))
        self.assertEqual(line.split(" | ")[2].strip(), "SKIP")

    def test_the_same_rows_at_t2_never_skip(self):
        result = self.run_rows(self.rows(), tier="T2")
        self.assertEqual((self.reason("a"), self.reason("b")), ("covered 1", "covered 1"))
        self.assertEqual(result.exit_code, 0)

    def test_parity_never_skips(self):
        self.run_rows(self.rows(), tier="T0")
        self.assertEqual(self.record("lint", "parity")["verdict"], "pass")

    def test_a_staged_deletion_is_not_in_scope(self):
        self.git("rm", "-q", "--cached", "two.txt")
        self.run_rows([self.row("b", expected_scope=("git", "two.txt"))], tier="T0")
        self.assertEqual(self.reason("b"), "skip: nothing staged in scope (0 tracked)")

    def test_a_staged_defect_is_never_skipped(self):
        # QA N12: the row still runs, and its own failure stands.
        self.run_rows([self.row("a", "status", 1, expected_scope=("git", "one.txt"))], tier="T0")
        self.assertEqual(self.reason("a"), "exit 1")


class Unborn(support.GateTest):
    def test_before_the_first_commit_every_staged_path_counts(self):
        # setUp staged one.txt in a repository with no commit yet (QA N14).
        result = self.run_rows([self.row("a", expected_scope=("git", "one.txt"))], tier="T0")
        self.assertEqual((self.reason("a"), result.exit_code), ("covered 1", 0))
