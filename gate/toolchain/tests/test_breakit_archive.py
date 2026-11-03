"""Break-it of S1.1a-1's combined build (QA), QB5: the installer publishes a truncated archive as a finished install.

tarfile ends its iteration without a word when a header after the first is cut short (TruncatedHeaderError at a
non-zero offset is treated as the end of the archive). So a .tar.gz whose gzip stream is whole but whose tar stream
stops inside the second member's header installs the first member only, stamps .complete, prints "installed" and
exits 0; every later run takes the partial tree as present. The archive's pinned digest makes this need a broken
pin, so it is rare; but the design treats every archive as hostile, and this one is half a tool, silently.
The witness: the same two members, whole, install both.
"""

import gzip
import io
import os
import tarfile

from gate.toolchain import manifest
from gate.toolchain.tests import support
from gate.toolchain.tests.test_install import InstallTest

BLOCK = 512


def tar_bytes(*members: tuple[str, bytes]) -> bytes:
    """An uncompressed ustar stream of regular files, so every header is exactly one block."""
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w", format=tarfile.USTAR_FORMAT) as tf:
        for name, data in members:
            info = tarfile.TarInfo(name)
            info.size, info.mode = len(data), 0o755
            tf.addfile(info, io.BytesIO(data))
    return buffer.getvalue()


class QB5TruncatedArchive(InstallTest):
    MEMBERS = (("demo/bin/demo", b"A" * BLOCK), ("demo/bin/second", b"B" * BLOCK))

    def pin_bytes(self, data: bytes) -> str:
        """Pin exactly these archive bytes; return the tool's final directory."""
        with open(self.archive, "wb") as f:
            f.write(data)
        entry = support.tool_entry(self.archive)
        support.write_tools(self.repo, [entry])
        digest = entry["platforms"][manifest.platform_key()]["archive_sha256"]
        return os.path.join(self.cache, f"{entry['name']}-{entry['version']}-{digest[:12]}")

    def test_qb5_witness_the_whole_archive_installs_both_members(self):
        home = self.pin_bytes(gzip.compress(tar_bytes(*self.MEMBERS)))
        code, lines = self.run_main("install")
        self.assertEqual((code, lines), (0, ["demo 1.0: installed"]))
        self.assertEqual(sorted(os.listdir(os.path.join(home, "bin"))), ["demo", "second"])

    def test_qb5_a_tar_stream_cut_inside_a_header_is_refused(self):
        whole = tar_bytes(*self.MEMBERS)
        self.assertEqual(whole[2 * BLOCK : 2 * BLOCK + 15], b"demo/bin/second")  # the second header starts here
        home = self.pin_bytes(gzip.compress(whole[: 2 * BLOCK + 200]))
        code, lines = self.run_main("install")
        installed = sorted(os.listdir(os.path.join(home, "bin"))) if os.path.isdir(home) else None
        self.assertEqual(code, 2, f"a truncated archive was published: {lines}, bin/ holds {installed}")
        self.assertEqual(lines[-2:], ["GATE FAILED", "exit: 2"])
        self.assertFalse(os.path.lexists(home))


del InstallTest  # a base only: unittest must not collect it from this module
