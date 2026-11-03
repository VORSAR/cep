"""The hooks' own rules (gate design §12.1, §12.4): the commit guard and the push plan."""

import unittest

from gate.hooks import pre_commit, pre_push

ZERO = "0" * 40
A, B, C = "a" * 40, "b" * 40, "c" * 40


class Guard(unittest.TestCase):
    def test_a_staged_file_with_unstaged_changes_is_named(self):
        self.assertEqual(pre_commit.mixed([b"a.go", b"b.go"], [b"b.go", b"c.go"]), ["b.go"])

    def test_nothing_in_both_is_nothing(self):
        self.assertEqual(pre_commit.mixed([b"a.go"], [b"c.go"]), [])

    def test_the_refusal_names_the_fix(self):
        self.assertEqual(
            pre_commit.refusal(["b.go"]),
            ["b.go has unstaged changes: stage them or stash them, so the check sees what you commit"],
        )


class PushPlan(unittest.TestCase):
    def plan(self, lines, head=A, clean=True):
        return pre_push.plan(lines, head=head, clean=clean, peel=lambda sha: sha)

    def test_a_clean_head_is_checked(self):
        self.assertEqual(self.plan([f"refs/heads/m {A} refs/heads/m {B}"]), [("refs/heads/m", A, "check")])

    def test_a_deletion_is_skipped(self):
        # Plan check N3.
        self.assertEqual(self.plan([f"(delete) {ZERO} refs/heads/m {B}"]), [])

    def test_two_refs_at_one_sha_check_once(self):
        # Plan check N4.
        got = self.plan([f"refs/heads/m {A} refs/heads/m {B}", f"refs/heads/n {A} refs/heads/n {ZERO}"])
        self.assertEqual(got, [("refs/heads/m", A, "check")])

    def test_anything_but_a_clean_head_is_left_to_ci(self):
        # Plan check N5 (v3).
        self.assertEqual(self.plan([f"refs/heads/o {C} refs/heads/o {B}"]), [("refs/heads/o", C, "ci")])
        self.assertEqual(self.plan([f"refs/heads/m {A} refs/heads/m {B}"], clean=False), [("refs/heads/m", A, "ci")])

    def test_a_tag_is_peeled_to_its_commit(self):
        got = pre_push.plan([f"refs/tags/v1 {C} refs/tags/v1 {ZERO}"], head=A, clean=True, peel=lambda sha: A)
        self.assertEqual(got, [("refs/tags/v1", A, "check")])


if __name__ == "__main__":
    unittest.main()
