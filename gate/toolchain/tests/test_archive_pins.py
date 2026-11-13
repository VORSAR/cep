"""install.archive_pins: the canary's hashes are the installer's (S1.1c-4 design §4.1; reviews A-L1, DA-B2, DQ-L5).

Lane F1's own tests, written before QA's equivalence test was on the branch. Each expected digest is derived by hand
from the archive's bytes, or from the tree the real installer published, never from archive_pins itself.
"""

import io
import os
import tempfile
import unittest
from unittest import mock

from gate.toolchain import install, manifest
from gate.toolchain.tests import support
from gate.toolchain.tests.support import Member

TOOL_BYTES = b"#!/bin/sh\necho demo 1.0\n"
HELPER_BYTES = b"helper\n"

MEMBERS = [
    Member("demo", kind="d", mode=0o755),
    Member("demo/bin", kind="d", mode=0o755),
    Member("demo/bin/demo", data=TOOL_BYTES, mode=0o755),
    Member("demo/bin/alias", kind="l", link="demo"),
    Member("demo/lib/nested/deeper/helper.txt", data=HELPER_BYTES, mode=0o644),
    Member("demo/lib/nested/tool", data=HELPER_BYTES, mode=0o750),
]


class ArchivePinsTest(support.ToolchainTest):
    def setUp(self) -> None:
        super().setUp()
        self.archive = support.write_archive(os.path.join(self.world, "demo.tar.gz"), MEMBERS)

    def installed_home(self) -> str:
        """Publish the archive with the real installer (file:// fetch, no network) and return the tool's tree."""
        entry = support.tool_entry(self.archive, exec_sha256={"bin/demo": support.sha256(TOOL_BYTES)})
        tools_json = support.write_tools(self.repo, [entry])
        out = io.StringIO()
        self.assertEqual(install.install(tools_json, self.cache, manifest.platform_key(), out), 0)
        self.assertEqual(out.getvalue(), "demo 1.0: installed\n")
        (tool,) = manifest.load(tools_json, manifest.platform_key())
        return manifest.tool_home(self.cache, tool)

    def test_archive_pins_equal_the_installers_published_tree(self) -> None:
        home = self.installed_home()
        self.assertTrue(os.path.isfile(os.path.join(home, ".complete")), "the installer's stamp is in its tree")
        pins = install.archive_pins(self.archive, "demo/", ["bin/demo", "lib/nested/tool"])
        self.assertEqual(pins.tree_sha256, manifest.tree_sha256(home))
        self.assertEqual(
            dict(pins.exec_sha256),
            {"bin/demo": support.sha256(TOOL_BYTES), "lib/nested/tool": support.sha256(HELPER_BYTES)},
        )

    def test_archive_pins_tree_hash_is_the_documented_line_digest(self) -> None:
        """The tree hash by hand from manifest.tree_sha256's documented lines: `<kind> <path> <payload>`, sorted."""
        lines = sorted(
            [
                b"d bin -",
                b"x bin/demo " + support.sha256(TOOL_BYTES).encode(),
                b"l bin/alias demo",
                b"d lib -",
                b"d lib/nested -",
                b"d lib/nested/deeper -",
                b"f lib/nested/deeper/helper.txt " + support.sha256(HELPER_BYTES).encode(),
                b"x lib/nested/tool " + support.sha256(HELPER_BYTES).encode(),
            ],
            key=lambda line: line.split(b" ")[1],
        )
        want = support.sha256(b"".join(line + b"\n" for line in lines))
        self.assertEqual(install.archive_pins(self.archive, "demo/", []).tree_sha256, want)

    def test_archive_pins_leaves_no_temporary_directory(self) -> None:
        made: list[str] = []
        real = tempfile.mkdtemp

        def recording(*args, **kwargs) -> str:
            made.append(real(*args, **kwargs))
            return made[-1]

        with mock.patch.object(install.tempfile, "mkdtemp", side_effect=recording):
            install.archive_pins(self.archive, "demo/", ["bin/demo"])
        self.assertEqual(len(made), 1, "archive_pins extracts into a directory of its own")
        self.assertFalse(os.path.lexists(made[0]))

    def test_rejected_archive_leaves_no_temporary_directory(self) -> None:
        hostile = support.write_archive(
            os.path.join(self.world, "hostile.tar.gz"),
            [Member("demo", kind="d", mode=0o755), Member("demo/bin/demo", data=TOOL_BYTES), Member("demo/../evil")],
        )
        made: list[str] = []
        real = tempfile.mkdtemp

        def recording(*args, **kwargs) -> str:
            made.append(real(*args, **kwargs))
            return made[-1]

        with (
            mock.patch.object(install.tempfile, "mkdtemp", side_effect=recording),
            self.assertRaisesRegex(install._Rejected, r"'demo/\.\./evil' refused: a `\.\.` component"),
        ):
            install.archive_pins(hostile, "demo/", ["bin/demo"])
        self.assertEqual(len(made), 1, "the extraction had started in its own directory")
        self.assertFalse(os.path.lexists(made[0]))

    def test_listed_executable_that_is_a_symlink_is_refused(self) -> None:
        with self.assertRaises(OSError):
            install.archive_pins(self.archive, "demo/", ["bin/alias"])


if __name__ == "__main__":
    unittest.main()
