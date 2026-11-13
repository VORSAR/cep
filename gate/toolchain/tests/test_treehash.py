"""The tree child, `gate/toolchain/treehash.py` (proof-world speed-up design note §3.2).

    treehash.py <root>                    realpath, its details digest, then its tree hash: three lines
    treehash.py --details <root> [...]    for each root in order, its realpath and its details digest

Both read one resolved path (the realpath, taken once) so the details and the hash describe the same tree, and an
OSError exits 3 (`UNREADABLE`) in either form. The child is run here as parity runs it, a process of its own.
"""

import os
import sys
import unittest

from gate.toolchain import manifest
from gate.toolchain.tests import support

TREEHASH = os.path.join(support.REPO, "gate", "toolchain", "treehash.py")
UNREADABLE = 3


class TreeChild(support.ToolchainTest):
    def tree(self, name: str, content: bytes = b"content\n") -> str:
        root = os.path.join(self.world, name)
        os.makedirs(os.path.join(root, "bin"))
        with open(os.path.join(root, "bin", "tool"), "wb") as f:
            f.write(content)
        os.symlink("bin/tool", os.path.join(root, "alias"))
        return root

    def child(self, *args: str) -> support.Child:
        return support.run_child(
            [sys.executable, "-I", "-B", TREEHASH, *args],
            env={"PATH": os.defpath},
            cwd=self.world,
            out_dir=os.path.join(self.world, "out"),
        )

    def test_the_child_prints_realpath_details_then_digest(self):
        real = self.tree("tree")
        link = os.path.join(self.world, "home")
        os.symlink(real, link)
        got = self.child(link)
        self.assertEqual((got.code, got.stderr), (0, ""))
        self.assertNotEqual(link, real)  # the witness: the root given is not its own realpath
        self.assertEqual(got.lines, [real, manifest.tree_details(real), manifest.tree_sha256(real)])

    def test_details_form_prints_two_lines_per_root_in_order(self):
        first, second = self.tree("b-first", b"one\n"), self.tree("a-second", b"two\n")
        link = os.path.join(self.world, "linked")
        os.symlink(first, link)
        got = self.child("--details", first, second, link)
        self.assertEqual((got.code, got.stderr), (0, ""))
        want = [first, manifest.tree_details(first), second, manifest.tree_details(second)]
        self.assertEqual(got.lines, [*want, first, manifest.tree_details(first)])
        self.assertNotEqual(want[1], want[3])  # the witness: the two trees' lines cannot stand in for each other

    def test_an_unreadable_tree_exits_3_in_both_forms(self):
        good = self.tree("good")
        bad = self.tree("bad")
        os.chmod(os.path.join(bad, "bin"), 0o000)  # its listing fails: a walk of details alone still meets it
        for argv in ((bad,), ("--details", bad), ("--details", good, bad)):
            with self.subTest(argv=argv[0] if len(argv) == 1 else " ".join(argv[:2])):
                got = self.child(*argv)
                self.assertEqual(got.code, UNREADABLE, got.stderr)
                self.assertIn("Permission denied", got.stderr)


if __name__ == "__main__":
    unittest.main()
