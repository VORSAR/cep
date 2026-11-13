"""install.archive_pins against the installer (S1.1c-4 design §4.1; plan check §6 canary (11); reviews A-L1, DA-B2,
DQ-L5): over a small synthetic .tar.gz (an executable, a symlink, a nested directory) the installer's published tree
and archive_pins give the same tree hash and executable hashes; a rejected archive leaves no temp directory behind.
Red on the W0 stub by NotImplementedError."""

import io
import os
import tempfile
import unittest
from unittest import mock

from gate.toolchain import install, manifest
from gate.toolchain.tests import support
from gate.toolchain.tests.support import Member

MEMBERS = [
    Member("demo", "d", mode=0o755),
    Member("demo/bin/demo", data=b"#!tool\n", mode=0o775),
    Member("demo/bin/link", "l", link="demo"),
    Member("demo/lib/sub/deep.txt", data=b"deep\n"),
    Member("demo/lib/sub/run", data=b"#!run\n", mode=0o755),
]


class ArchivePins(support.ToolchainTest):
    def setUp(self) -> None:
        super().setUp()
        self.archive = support.write_archive(os.path.join(self.world, "demo.tar.gz"), MEMBERS)
        self.temp = os.path.join(self.world, "private-tmp")
        os.makedirs(self.temp)
        patch = mock.patch.object(tempfile, "tempdir", self.temp)  # archive_pins' mkdtemp() lands here
        patch.start()
        self.addCleanup(patch.stop)

    def install_tree(self) -> str:
        entry = support.tool_entry(self.archive, exec_sha256={"bin/demo": "0" * 64})
        tools_json = support.write_tools(self.repo, [entry])
        self.assertEqual(install.install(tools_json, self.cache, manifest.platform_key(), io.StringIO()), 0)
        digest = support.sha256_of(self.archive)
        return os.path.join(self.cache, f"demo-1.0-{digest[:12]}")

    def test_archive_pins_equal_the_installers_tree(self):
        home = self.install_tree()
        pins = install.archive_pins(self.archive, "demo/", ["bin/demo", "lib/sub/run"])
        self.assertEqual(pins.tree_sha256, manifest.tree_sha256(home))
        self.assertEqual(
            dict(pins.exec_sha256),
            {"bin/demo": support.sha256(b"#!tool\n"), "lib/sub/run": support.sha256(b"#!run\n")},
        )
        self.assertEqual(os.listdir(self.temp), [])

    def test_installer_stamp_unchanged(self):
        # Review DA-B2: the installer still writes .complete inside the tree before sealing; it is read-only.
        home = self.install_tree()
        stamp = os.path.join(home, ".complete")
        with open(stamp, encoding="ascii") as f:
            self.assertEqual(f.read(), support.sha256_of(self.archive) + "\n")
        self.assertEqual(os.stat(stamp).st_mode & 0o777, 0o444)
        self.assertEqual(
            install.archive_pins(self.archive, "demo/", ["bin/demo"]).tree_sha256, manifest.tree_sha256(home)
        )

    def test_rejected_archive_leaves_no_temp_dir(self):
        hostile = support.write_archive(
            os.path.join(self.world, "hostile.tar.gz"),
            [
                Member("demo", "d", mode=0o755),
                Member("demo/bin/demo", data=b"x"),
                Member("demo/escape", "l", link="../../etc"),
            ],
        )
        with self.assertRaises(Exception) as cm:
            install.archive_pins(hostile, "demo/", ["bin/demo"])
        self.assertNotIsInstance(cm.exception, NotImplementedError)
        self.assertIsInstance(cm.exception, (install.InstallError, install._Rejected))
        self.assertIn("demo/escape", str(cm.exception))
        self.assertEqual(os.listdir(self.temp), [])


if __name__ == "__main__":
    unittest.main()
