"""Lane F1's own tests of the canary's archive seam (S1.1c-4 design §4.3): feeds.downloader checks the published
digest and keeps nothing it refused, and candidate.build names a real seam's DigestMismatch with the tool and the
platform. QA's W0c tests drive build through a fake that raises the named error itself; these cover the real seam.
Never the network: the fetch is injected."""

import copy
import os
import tempfile
import unittest

from gate.canary import candidate, feeds
from gate.canary.candidate import CanaryError, DigestMismatch
from gate.canary.tests import feedsupport as fs

BODY = b"an archive's bytes\n"
URL = "https://example.invalid/tool.tar.gz"


class Downloader(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory(prefix="canary-dl-")
        self.addCleanup(tmp.cleanup)
        self.dir = tmp.name
        self.fetched: list[str] = []

    def fetch(self, url, sink) -> None:
        self.fetched.append(url)
        sink.write(BODY)

    def test_matching_digest_gives_the_downloaded_file(self):
        path = feeds.downloader(self.dir, self.fetch)(URL, fs.sha(BODY))
        self.assertEqual(self.fetched, [URL])
        with open(path, "rb") as f:
            self.assertEqual(f.read(), BODY)
        self.assertEqual(os.path.dirname(path), self.dir)

    def test_wrong_digest_raises_and_keeps_nothing(self):
        with self.assertRaises(DigestMismatch):
            feeds.downloader(self.dir, self.fetch)(URL, "0" * 64)
        self.assertEqual(self.fetched, [URL], "the download ran")
        self.assertEqual(os.listdir(self.dir), [])

    def test_failed_fetch_keeps_nothing(self):
        def broken(url, sink) -> None:
            sink.write(b"part")
            raise OSError("connection reset")

        with self.assertRaisesRegex(OSError, "connection reset"):
            feeds.downloader(self.dir, broken)(URL, fs.sha(BODY))
        self.assertEqual(os.listdir(self.dir), [])


class BuildNamesTheSeamsErrors(unittest.TestCase):
    def setUp(self) -> None:
        self.pins = fs.pinned()
        self.archive = fs.FakeArchive(self, self.pins)

    def build_with(self, error: Exception) -> str:
        ruff = "https://github.com/astral-sh/ruff/releases/download/0.17.5/ruff-x86_64-unknown-linux-gnu.tar.gz"
        self.archive.fail[ruff] = error
        with self.assertRaises(CanaryError) as cm:
            candidate.build(copy.deepcopy(self.pins), fs.feeds(), self.archive)
        self.assertIn(ruff, self.archive.urls(), "build reached the ruff download")
        return str(cm.exception)

    def test_digest_mismatch_from_the_real_seam_is_named(self):
        text = self.build_with(DigestMismatch("some url: sha256 a is not the published b"))
        self.assertEqual(text, "ruff linux-amd64: the download's sha256 is not the published one")

    def test_a_failed_download_is_named(self):
        text = self.build_with(OSError("timed out"))
        self.assertTrue(text.startswith("ruff linux-amd64: the download failed: https://github.com/"), text)
        self.assertTrue(text.endswith(": timed out"), text)


if __name__ == "__main__":
    unittest.main()
