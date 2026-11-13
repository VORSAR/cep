"""S1.1c-4's batched fix round, `make vulndb` (owner 2026-11-13, S1.1c-4's review and break-it, decisions 2 and 3):
the shared format check and a CRC pass before anything is replaced (CA-B1, CA-S2), the earlier copy kept whatever
fails (BK-S2, CA-L8), and any redirect off https refused (decision 3).

Every body is built in the test; every expected text is written from the review's text by hand, before the code.
"""

import contextlib
import io
import json
import os
import tempfile
import unittest
import urllib.request
import zipfile
from unittest import mock

from gate.toolchain import vulndb_fetch
from gate.toolchain.tests.test_vulndb_fetch import GOOD, NOW, NOW_TEXT, OLD_STAMP, OLD_ZIP, URL, database, serving

MODIFIED = "2026-11-11T02:51:11Z"
ENTRY = {
    "id": "GO-2099-0001",
    "modified": MODIFIED,
    "affected": [
        {
            "package": {"name": "example.com/a", "ecosystem": "Go"},
            "ranges": [{"type": "SEMVER", "events": [{"introduced": "0"}, {"fixed": "1.0.1"}]}],
        }
    ],
}


def members(**over: object) -> dict[str, bytes]:
    """A consistent one-entry database (design §2.4), each member's JSON text; `over` replaces or adds members."""
    out: dict[str, object] = {
        "index/db.json": {"modified": MODIFIED},
        "index/modules.json": [{"path": "example.com/a", "vulns": [{"id": "GO-2099-0001", "modified": MODIFIED}]}],
        "index/vulns.json": [{"id": "GO-2099-0001", "modified": MODIFIED, "aliases": []}],
        "ID/GO-2099-0001.json": ENTRY,
    }
    out.update(over)
    return {k: v if isinstance(v, bytes) else json.dumps(v).encode() for k, v in out.items()}


def stored(data: dict[str, bytes]) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_STORED) as z:
        for name, value in data.items():
            z.writestr(name, value)
    return buf.getvalue()


class Fix(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = os.path.join(tmp.name, "vulndb")
        os.mkdir(self.root)

    def read(self, name: str) -> bytes | None:
        try:
            with open(os.path.join(self.root, name), "rb") as f:
                return f.read()
        except FileNotFoundError:
            return None

    def plant_old_copy(self) -> None:
        with open(os.path.join(self.root, "vulndb.zip"), "wb") as f:
            f.write(OLD_ZIP)
        with open(os.path.join(self.root, "stamp.json"), "wb") as f:
            f.write(OLD_STAMP)

    def assertRefused(self, get, why: str) -> str:
        """FetchError holding `why`; the old zip and stamp byte-identical; fetch-error.txt naming it; no temp file."""
        self.plant_old_copy()
        with self.assertRaises(vulndb_fetch.FetchError) as caught:
            vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertIn(why, str(caught.exception))
        self.assertEqual((self.read("vulndb.zip"), self.read("stamp.json")), (OLD_ZIP, OLD_STAMP))
        error = json.loads(self.read("fetch-error.txt"))
        self.assertEqual(error, {"at": NOW_TEXT, "error": str(caught.exception)})
        self.assertEqual(sorted(os.listdir(self.root)), ["fetch-error.txt", "stamp.json", "vulndb.zip"])
        return str(caught.exception)

    def test_the_good_body_is_accepted_the_witness(self):
        with zipfile.ZipFile(io.BytesIO(GOOD)) as z:  # the module's GOOD is this consistent database
            self.assertEqual({name: z.read(name) for name in z.namelist()}, members())
        get, _ = serving(stored(members()))
        vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertEqual(sorted(os.listdir(self.root)), ["stamp.json", "vulndb.zip"])

    # ------------------------------------------------------------------ CA-B1: every member read before replacing

    def test_a_member_failing_its_crc_replaces_nothing(self):
        body = bytearray(stored(members()))
        at = body.rfind(b'"GO-2099-0001", "modified"')  # inside ID/GO-2099-0001.json, stored last
        self.assertGreater(at, 0)
        body[at + 1] = ord("X")
        self.assertTrue(zipfile.is_zipfile(io.BytesIO(bytes(body))))
        get, _ = serving(bytes(body))
        self.assertRefused(get, "member ID/GO-2099-0001.json cannot be read: ")

    def test_a_broken_deflate_stream_replaces_nothing(self):
        good = database(members())
        info = zipfile.ZipFile(io.BytesIO(good)).getinfo("ID/GO-2099-0001.json")
        start = info.header_offset + 30 + len(info.filename) + len(info.extra)
        body = bytearray(good)
        body[start : start + 4] = b"\xff\xff\xff\xff"
        get, _ = serving(bytes(body))
        self.assertRefused(get, "member ID/GO-2099-0001.json cannot be read: ")

    # ------------------------------------------------------------------ CA-S2: an upstream format change refused

    def test_an_extra_db_json_key_replaces_nothing_naming_it(self):
        get, _ = serving(database(members(**{"index/db.json": {"modified": MODIFIED, "schema": 2}})))
        self.assertRefused(get, "index/db.json is missing or malformed: unknown key schema")

    def test_a_new_index_file_replaces_nothing_naming_it(self):
        get, _ = serving(database(members(**{"index/aliases.json": b"[]"})))
        self.assertRefused(get, "member index/aliases.json is not in the database's layout")

    def test_an_empty_index_replaces_nothing(self):
        get, _ = serving(database(members(**{"index/vulns.json": []})))
        self.assertRefused(get, "index/vulns.json is missing, malformed or empty")

    # ------------------------------------------------------------------ decision 3: no redirect off https

    def test_a_redirect_off_https_replaces_nothing_naming_it(self):
        def get(url: str, sink) -> None:
            sink.write(GOOD[:10])
            raise vulndb_fetch.Redirected(url, "http://mirror.example/vulndb.zip")

        text = self.assertRefused(get, "")
        self.assertEqual(text, f"{URL} redirected to a non-https URL http://mirror.example/vulndb.zip: refused")

    def test_the_redirect_handler_refuses_http_and_follows_https(self):
        handler = vulndb_fetch._HttpsOnly()
        req = urllib.request.Request(URL)
        follow = handler.redirect_request(req, io.BytesIO(), 302, "Found", {}, "https://mirror.example/vulndb.zip")
        self.assertEqual(follow.full_url, "https://mirror.example/vulndb.zip")  # the witness: https is followed
        for target in ("http://mirror.example/vulndb.zip", "ftp://mirror.example/vulndb.zip"):
            with self.subTest(target), self.assertRaises(vulndb_fetch.Redirected) as caught:
                handler.redirect_request(req, io.BytesIO(), 302, "Found", {}, target)
            self.assertEqual(str(caught.exception), f"{URL} redirected to a non-https URL {target}: refused")

    def test_main_fetches_through_the_https_only_get(self):
        get, asked = serving(GOOD)
        out = io.StringIO()
        environ = {"VORSAR_CEP_VULNDB": self.root, "HOME": self.root}
        with mock.patch.object(vulndb_fetch, "_get", get), contextlib.redirect_stdout(out):
            self.assertEqual(vulndb_fetch.main([], environ), 0, out.getvalue())
        self.assertEqual(asked, [URL])
        opener = vulndb_fetch._opener()
        self.assertTrue(any(isinstance(h, vulndb_fetch._HttpsOnly) for h in opener.handlers), opener.handlers)
        self.assertFalse(any(type(h) is urllib.request.HTTPRedirectHandler for h in opener.handlers), opener.handlers)

    # ------------------------------------------------------------------ BK-S2 / CA-L8: whatever fails, nothing lost

    def test_a_stamp_write_that_fails_with_no_earlier_copy_leaves_no_zip(self):
        get, _ = serving(GOOD)
        real = vulndb_fetch._write_json

        def write_json(root: str, name: str, value: object) -> None:
            if name == vulndb_fetch.STAMP:
                raise OSError(28, "No space left on device")
            real(root, name, value)

        with mock.patch.object(vulndb_fetch, "_write_json", write_json), self.assertRaises(vulndb_fetch.FetchError):
            vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertEqual(sorted(os.listdir(self.root)), ["fetch-error.txt"])
        self.assertIn("No space left on device", json.loads(self.read("fetch-error.txt"))["error"])

    def test_an_interrupted_download_leaves_no_temp_file(self):
        def get(url: str, sink) -> None:
            sink.write(GOOD[:10])
            raise KeyboardInterrupt

        self.plant_old_copy()
        with self.assertRaises(KeyboardInterrupt):
            vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertEqual(sorted(os.listdir(self.root)), ["stamp.json", "vulndb.zip"])
        self.assertEqual((self.read("vulndb.zip"), self.read("stamp.json")), (OLD_ZIP, OLD_STAMP))

    def test_an_interrupt_at_the_stamp_write_restores_the_earlier_zip(self):
        get, _ = serving(GOOD)
        real = vulndb_fetch._write_json

        def write_json(root: str, name: str, value: object) -> None:
            if name == vulndb_fetch.STAMP:
                raise KeyboardInterrupt
            real(root, name, value)

        self.plant_old_copy()
        with mock.patch.object(vulndb_fetch, "_write_json", write_json), self.assertRaises(KeyboardInterrupt):
            vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertEqual(sorted(os.listdir(self.root)), ["stamp.json", "vulndb.zip"])
        self.assertEqual((self.read("vulndb.zip"), self.read("stamp.json")), (OLD_ZIP, OLD_STAMP))

    def test_a_success_leaves_no_kept_copy_behind(self):
        self.plant_old_copy()
        get, _ = serving(GOOD)
        vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertEqual(sorted(os.listdir(self.root)), ["stamp.json", "vulndb.zip"])
        self.assertEqual(self.read("vulndb.zip"), GOOD)


if __name__ == "__main__":
    unittest.main()
