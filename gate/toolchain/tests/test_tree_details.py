"""A tree's details digest (proof-world speed-up design note §3.1; owner, 2026-11-14, the speed-up's measurement,
decision 2: a tree is unchanged when every entry's name, type, mode, size, inode, modification and change times are
what the outer run saw when it hashed it).

`manifest.tree_details(root)` is the sha256 of one line per entry, root itself as `.`, from os.lstat, `.complete`
included, the lines sorted bytewise. The expected digest here is written out from os.lstat in the documented line
format, never taken from the code under test (QB-S6).
"""

import hashlib
import os
import unittest

from gate.toolchain import manifest
from gate.toolchain.tests import support

ENTRIES = (".", ".complete", "README", "bin", "bin/tool", "link")


def line(root: str, rel: str) -> bytes:
    """One entry's line, in the design note's format: path, mode, size, dev, inode, mtime and ctime in ns."""
    st = os.lstat(root if rel == "." else os.path.join(root, rel))
    fields = (st.st_mode, st.st_size, st.st_dev, st.st_ino, st.st_mtime_ns, st.st_ctime_ns)
    return os.fsencode(rel) + b"".join(b"\0%d" % value for value in fields) + b"\n"


class TreeDetails(support.ToolchainTest):
    def tree(self, name: str) -> str:
        """A directory, a plain file, an executable, a symlink and `.complete`."""
        root = os.path.join(self.world, name)
        os.makedirs(os.path.join(root, "bin"))
        for rel, data, mode in (
            ("README", b"read me\n", 0o644),
            ("bin/tool", b"#!/bin/false\n", 0o755),
            (".complete", b"done\n", 0o444),
        ):
            path = os.path.join(root, rel)
            with open(path, "wb") as f:
                f.write(data)
            os.chmod(path, mode)
        os.symlink("README", os.path.join(root, "link"))
        return root

    def test_tree_details_is_the_sha256_of_exactly_these_lines(self):
        root = self.tree("tree")
        want = hashlib.sha256(b"".join(sorted(line(root, rel) for rel in ENTRIES))).hexdigest()
        self.assertEqual(manifest.tree_details(root), want)

    def test_hashing_the_tree_leaves_its_details(self):
        # The witness the next test's negatives need: reading every file's content (atime aside) changes no detail,
        # so the outer run's hash between its details and a nested run's details moves nothing by itself.
        root = self.tree("tree")
        before = manifest.tree_details(root)
        manifest.tree_sha256(root)
        self.assertEqual(manifest.tree_details(root), before)

    def test_each_change_moves_the_details(self):
        def chmod(root: str) -> None:
            os.chmod(os.path.join(root, "README"), 0o600)

        def same_size_same_mtime(root: str) -> None:
            path = os.path.join(root, "README")
            st = os.lstat(path)
            with open(path, "r+b") as f:
                f.write(b"READ ME\n")
            os.utime(path, ns=(st.st_atime_ns, st.st_mtime_ns), follow_symlinks=False)
            after = os.lstat(path)
            # Only the change time is left to tell (the anchor, design note §3.1).
            self.assertEqual(
                (after.st_mode, after.st_size, after.st_ino, after.st_mtime_ns),
                (st.st_mode, st.st_size, st.st_ino, st.st_mtime_ns),
            )

        def added(root: str) -> None:
            with open(os.path.join(root, "bin", "new"), "wb") as f:
                f.write(b"new\n")

        def removed(root: str) -> None:
            os.unlink(os.path.join(root, "bin", "tool"))

        def retargeted(root: str) -> None:
            link = os.path.join(root, "link")
            os.symlink("bin/xy", link + ".tmp")  # the same length as README: the size cannot tell
            os.replace(link + ".tmp", link)

        # Each change, and the entry whose own change time it moves: the file, or the directory gaining or losing one.
        for change, rel in (
            (chmod, "README"),
            (same_size_same_mtime, "README"),
            (added, "bin"),
            (removed, "bin"),
            (retargeted, "link"),
        ):
            with self.subTest(change=change.__name__):
                root = self.tree(change.__name__)
                before = manifest.tree_details(root)
                planted = os.path.join(root, rel)
                ctime = support.past_the_ctime_of(planted, os.path.join(self.world, "probe"))
                change(root)
                self.assertGreater(os.lstat(planted).st_ctime_ns, ctime, "the witness: the change moved its ctime")
                self.assertNotEqual(manifest.tree_details(root), before)

    def test_a_missing_root_raises_oserror(self):
        with self.assertRaises(OSError):
            manifest.tree_details(os.path.join(self.world, "absent"))


if __name__ == "__main__":
    unittest.main()
