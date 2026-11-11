"""gate/runner/git.py (S1.1c-1 design §2.2): one temporary repository per test, isolated as §1 says.

Expected values are worked out by hand from git's documented output: `--numstat -z` records are
`added\\tremoved\\tpath\\0`; `--no-renames` reports a rename as a removal and an addition; a binary file's counts are
`-`, read as 0.
"""

import os
import unittest

from gate.runner import git
from gate.runner.tests import support

IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@example.com")


class GitTest(support.GateTest):
    def commit(self, message: str) -> None:
        self.git("add", "-A")
        self.git(*IDENTITY, "commit", "-q", "-m", message)

    def main_then_change(self) -> None:
        self.git("checkout", "-q", "-b", "main")
        self.track("a.txt", "a1\na2\n")
        self.track("b.txt", "b1\n")
        self.commit("base")
        self.git("checkout", "-q", "-b", "change")


class MergeBase(GitTest):
    def test_the_merge_base_with_main(self):
        self.main_then_change()
        base = self.git("rev-parse", "HEAD").decode().strip()
        self.write("a.txt", "a1\na2\na3\n")
        self.commit("change")
        self.assertEqual(git.merge_base(cwd=self.repo), base)

    def test_no_main_is_empty_never_a_guess(self):
        self.git("checkout", "-q", "-b", "work")
        self.commit("only")
        self.assertEqual(git.merge_base(cwd=self.repo), "")

    def test_head_is_the_full_sha(self):
        self.main_then_change()
        want = self.git("rev-parse", "HEAD").decode().strip()
        self.assertEqual(git.head(cwd=self.repo), want)
        self.assertEqual(len(want), 40)


class Numstat(GitTest):
    def test_two_files_in_gits_path_order(self):
        self.main_then_change()
        base = git.head(cwd=self.repo)
        self.write("b.txt", "b1\nb2\nb3\n")
        self.write("a.txt", "a2\n")
        self.commit("change")
        got = git.numstat(base, git.head(cwd=self.repo), ["a.txt", "b.txt"], cwd=self.repo)
        self.assertEqual(got, [("a.txt", 0, 1), ("b.txt", 2, 0)])

    def test_a_rename_is_a_removal_and_an_addition(self):
        self.main_then_change()
        base = git.head(cwd=self.repo)
        self.git("mv", "a.txt", "c.txt")
        self.commit("rename")
        got = git.numstat(base, git.head(cwd=self.repo), ["a.txt", "c.txt"], cwd=self.repo)
        self.assertEqual(got, [("a.txt", 0, 2), ("c.txt", 2, 0)])

    def test_a_binary_file_counts_as_zero(self):
        self.main_then_change()
        base = git.head(cwd=self.repo)
        with open(os.path.join(self.repo, "bin.dat"), "wb") as f:
            f.write(b"\x00\x01\x02\xff")
        self.commit("binary")
        got = git.numstat(base, git.head(cwd=self.repo), ["bin.dat"], cwd=self.repo)
        self.assertEqual(got, [("bin.dat", 0, 0)])

    def test_a_prefix_path_covers_the_files_under_it(self):
        self.main_then_change()
        base = git.head(cwd=self.repo)
        self.write("d/x.txt", "x\n")
        self.commit("under d")
        self.assertEqual(git.numstat(base, git.head(cwd=self.repo), ["d/"], cwd=self.repo), [("d/x.txt", 1, 0)])


class ShowAndListing(GitTest):
    def test_show_reads_a_file_at_a_revision(self):
        self.main_then_change()
        base = git.head(cwd=self.repo)
        self.write("a.txt", "changed\n")
        self.commit("change")
        self.assertEqual(git.show(base, "a.txt", cwd=self.repo), "a1\na2\n")
        self.assertEqual(git.show(git.head(cwd=self.repo), "a.txt", cwd=self.repo), "changed\n")

    def test_show_of_an_absent_file_is_none(self):
        self.main_then_change()
        self.assertIsNone(git.show(git.head(cwd=self.repo), "nope.txt", cwd=self.repo))

    def test_ls_files_lists_tracked_files_and_a_prefix(self):
        self.main_then_change()
        self.track("d/x.txt", "x\n")
        self.assertEqual(git.ls_files(["a.txt"], cwd=self.repo), ["a.txt"])
        self.assertEqual(git.ls_files(["d/"], cwd=self.repo), ["d/x.txt"])
        self.assertEqual(git.ls_files(["nope.txt"], cwd=self.repo), [])


class Dirty(GitTest):
    def test_a_clean_tree_is_zero(self):
        self.main_then_change()
        self.assertEqual(git.dirty(["a.txt", "b.txt"], cwd=self.repo), 0)

    def test_an_unstaged_and_a_staged_edit_count_two(self):
        self.main_then_change()
        self.write("a.txt", "edited\n")
        self.track("b.txt", "staged\n")
        self.assertEqual(git.dirty(["a.txt", "b.txt"], cwd=self.repo), 2)

    def test_an_edit_outside_the_paths_does_not_count(self):
        self.main_then_change()
        self.write("one.txt", "edited\n")
        self.assertEqual(git.dirty(["a.txt", "b.txt"], cwd=self.repo), 0)


if __name__ == "__main__":
    unittest.main()
