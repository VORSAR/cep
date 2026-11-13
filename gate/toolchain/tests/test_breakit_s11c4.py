"""S1.1c-4 break-it (QA with the-fool, step 7): `make vulndb` interrupted after it replaced the zip.

Red on the build at 1b06251 for the reason in its comment; the fix makes it green.
"""

import os
import tempfile
import unittest
from unittest import mock

from gate.toolchain import vulndb_fetch
from gate.toolchain.tests.test_vulndb_fetch import GOOD, NOW, OLD_STAMP, OLD_ZIP, serving


class BreakIt(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory(prefix="breakit-")
        self.addCleanup(tmp.cleanup)
        self.root = tmp.name
        with open(os.path.join(self.root, vulndb_fetch.ZIP), "wb") as f:
            f.write(OLD_ZIP)
        with open(os.path.join(self.root, vulndb_fetch.STAMP), "wb") as f:
            f.write(OLD_STAMP)

    def read(self, name: str) -> bytes | None:
        try:
            with open(os.path.join(self.root, name), "rb") as f:
                return f.read()
        except FileNotFoundError:
            return None

    def test_bk_5_a_stamp_write_that_fails_leaves_the_earlier_zip_and_stamp_as_they_were(self):
        # Red today: fetch os.replace()s vulndb.zip and only then writes stamp.json, outside its failure handler.
        # A stamp write that fails (a full disk, ENOSPC; or Ctrl-C in that window) leaves the NEW zip beside the
        # OLD stamp, writes no fetch-error.txt, and the OSError escapes main as a traceback, not `vulndb: <error>`
        # and exit 2. Design §2.3: "On any failure: ... the old copy and stamp left as they were, fetch-error.txt
        # written". The row then reds with an integrity fault ("the zip's sha256 is not the stamp's") that the
        # `vulndb` waiver may not silence, until a later fetch succeeds.
        get, _ = serving(GOOD)
        real = vulndb_fetch._write_json

        def write_json(root: str, name: str, value: object) -> None:
            if name == vulndb_fetch.STAMP:
                raise OSError(28, "No space left on device")
            real(root, name, value)

        # today: OSError, so main crashes
        with mock.patch.object(vulndb_fetch, "_write_json", write_json), self.assertRaises(vulndb_fetch.FetchError):
            vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertEqual((self.read(vulndb_fetch.ZIP), self.read(vulndb_fetch.STAMP)), (OLD_ZIP, OLD_STAMP))
        self.assertIsNotNone(self.read(vulndb_fetch.ERROR), "fetch-error.txt must name the failure")
