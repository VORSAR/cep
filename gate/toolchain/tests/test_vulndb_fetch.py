"""`make vulndb`'s fetch (S1.1c-4 design note §2.3, §5 W0c; review DQ-S5): validated before it replaces anything.

Driven through the `fetch` seam with `get` and `now` injected, and `main` with its https-only `_get` patched: no
test reads the network or the real ~/.cache. Every body is built in the test; expected digests come from the body's
own bytes through hashlib, never from the code under test.
"""

import contextlib
import hashlib
import io
import json
import os
import tempfile
import unittest
import zipfile
from datetime import UTC, datetime
from unittest import mock

from gate.runner import vulndb
from gate.toolchain import vulndb_fetch

NOW = datetime(2026, 11, 13, 9, 30, 15, tzinfo=UTC)
NOW_TEXT = "2026-11-13T09:30:15Z"
URL = "https://vuln.go.dev/vulndb.zip"
OLD_ZIP = b"PK-the-earlier-copy"
OLD_STAMP = b'{"fetched":"2026-11-10T00:00:00Z","sha256":"00","url":"https://vuln.go.dev/vulndb.zip"}\n'


def database(members: dict[str, bytes]) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as z:
        for name, data in members.items():
            z.writestr(name, data)
    return buf.getvalue()


# A consistent one-entry database: the fetch runs the row's format check before it replaces anything (review CA-S2),
# so the indexes are non-empty and agree with the entry, as vuln.go.dev's do.
GOOD = database(
    {
        "index/db.json": b'{"modified": "2026-11-11T02:51:11Z"}',
        "index/modules.json": (
            b'[{"path": "example.com/a", "vulns": [{"id": "GO-2099-0001", "modified": "2026-11-11T02:51:11Z"}]}]'
        ),
        "index/vulns.json": b'[{"id": "GO-2099-0001", "modified": "2026-11-11T02:51:11Z", "aliases": []}]',
        "ID/GO-2099-0001.json": (
            b'{"id": "GO-2099-0001", "modified": "2026-11-11T02:51:11Z", "affected": [{"package": {"name": '
            b'"example.com/a", "ecosystem": "Go"}, "ranges": [{"type": "SEMVER", "events": [{"introduced": "0"}, '
            b'{"fixed": "1.0.1"}]}]}]}'
        ),
    }
)
NO_DB_JSON = database({"index/modules.json": b"[]", "ID/GO-2099-0001.json": b"{}"})


def serving(body: bytes, *, chunk: int = 1 << 20):
    """A `get` that streams `body` into the sink, chunk by chunk, after checking the URL."""
    asked: list[str] = []

    def get(url: str, sink) -> None:
        asked.append(url)
        for at in range(0, len(body), chunk):
            sink.write(body[at : at + chunk])

    return get, asked


def endless(sink_url_check: list[str]):
    """A `get` whose body never ends: 1 MiB chunks until the sink refuses one."""

    def get(url: str, sink) -> None:
        sink_url_check.append(url)
        block = b"\0" * (1 << 20)
        for _ in range((vulndb.FETCH_CAP >> 20) + 8):
            sink.write(block)

    return get


class Fetch(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = os.path.join(tmp.name, "vulndb")
        os.mkdir(self.root)

    def path(self, name: str) -> str:
        return os.path.join(self.root, name)

    def read(self, name: str) -> bytes:
        with open(self.path(name), "rb") as f:
            return f.read()

    def plant_old_copy(self) -> None:
        with open(self.path("vulndb.zip"), "wb") as f:
            f.write(OLD_ZIP)
        with open(self.path("stamp.json"), "wb") as f:
            f.write(OLD_STAMP)

    def assertRefused(self, get, why: str) -> None:
        """FetchError naming `why`; the old zip and stamp byte-identical; fetch-error.txt written; no temp file."""
        self.plant_old_copy()
        with self.assertRaises(vulndb_fetch.FetchError) as caught:
            vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertIn(why, str(caught.exception))
        self.assertEqual(self.read("vulndb.zip"), OLD_ZIP)
        self.assertEqual(self.read("stamp.json"), OLD_STAMP)
        error = json.loads(self.read("fetch-error.txt"))
        self.assertEqual(set(error), {"at", "error"})
        self.assertEqual(error["at"], NOW_TEXT)
        self.assertIn(why, error["error"])
        self.assertNotIn("\n", error["error"])
        self.assertEqual(sorted(os.listdir(self.root)), ["fetch-error.txt", "stamp.json", "vulndb.zip"])

    def test_a_body_that_is_not_a_zip_replaces_nothing(self):
        get, asked = serving(b"<html>a captive portal</html>\n")
        self.assertRefused(get, "not a zip")
        self.assertEqual(asked, [URL])

    def test_a_body_past_the_cap_replaces_nothing(self):
        asked: list[str] = []
        self.assertRefused(endless(asked), "larger than 64 MiB")
        self.assertEqual(asked, [URL])

    def test_a_zip_without_db_json_replaces_nothing(self):
        get, _ = serving(NO_DB_JSON)
        self.assertRefused(get, "index/db.json is missing or malformed")

    def test_a_network_failure_replaces_nothing(self):
        def get(url: str, sink) -> None:
            sink.write(GOOD[:10])
            raise OSError("connection reset by peer")

        self.assertRefused(get, "connection reset by peer")

    def test_a_success_installs_the_zip_and_its_stamp_and_clears_the_error(self):
        self.plant_old_copy()
        with open(self.path("fetch-error.txt"), "w", encoding="utf-8") as f:
            f.write('{"at":"2026-11-12T00:00:00Z","error":"an earlier failure"}\n')
        get, asked = serving(GOOD, chunk=7)
        line = vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        digest = hashlib.sha256(GOOD).hexdigest()
        self.assertEqual(asked, [URL])
        self.assertEqual(self.read("vulndb.zip"), GOOD)
        self.assertEqual(json.loads(self.read("stamp.json")), {"fetched": NOW_TEXT, "sha256": digest, "url": URL})
        self.assertTrue(self.read("stamp.json").endswith(b"}\n"))
        self.assertEqual(sorted(os.listdir(self.root)), ["stamp.json", "vulndb.zip"])
        self.assertEqual(
            line, f"vulndb: fetched {len(GOOD)} bytes, sha256 {digest[:12]}, modified 2026-11-11T02:51:11Z"
        )

    def test_the_first_fetch_makes_the_root(self):
        os.rmdir(self.root)
        get, _ = serving(GOOD)
        vulndb_fetch.fetch(self.root, get=get, now=lambda: NOW)
        self.assertEqual(self.read("vulndb.zip"), GOOD)


class Main(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = os.path.join(tmp.name, "copy")
        self.environ = {"VORSAR_CEP_VULNDB": self.root, "HOME": tmp.name}

    def main(self, body: bytes) -> tuple[int, str]:
        get, _ = serving(body)
        out = io.StringIO()
        with mock.patch.object(vulndb_fetch, "_get", get), contextlib.redirect_stdout(out):  # decision 3's getter
            code = vulndb_fetch.main([], self.environ)
        return code, out.getvalue()

    def test_a_refused_body_exits_2_naming_it(self):
        code, out = self.main(b"not a zip at all")
        self.assertEqual(code, 2)
        self.assertTrue(out.startswith("vulndb: "), out)
        self.assertIn("not a zip", out)
        self.assertTrue(os.path.exists(os.path.join(self.root, "fetch-error.txt")))

    def test_a_good_body_exits_0(self):
        code, out = self.main(GOOD)
        self.assertEqual(code, 0, out)
        self.assertTrue(out.startswith(f"vulndb: fetched {len(GOOD)} bytes, sha256 "), out)

    def test_an_argument_is_a_usage_error(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertEqual(vulndb_fetch.main(["--now"], self.environ), 2)


class CopyRoot(unittest.TestCase):
    def test_the_override_is_used_and_made_absolute(self):
        self.assertEqual(vulndb_fetch.copy_root({"VORSAR_CEP_VULNDB": "/x/db", "HOME": "/h"}), "/x/db")
        relative = vulndb_fetch.copy_root({"VORSAR_CEP_VULNDB": "rel/db", "HOME": "/h"})
        self.assertTrue(os.path.isabs(relative) and relative.endswith(os.path.join("rel", "db")), relative)

    def test_the_default_is_under_home(self):
        self.assertEqual(vulndb_fetch.copy_root({"HOME": "/home/dev"}), "/home/dev/.cache/vorsar/cep/vulndb")


if __name__ == "__main__":
    unittest.main()
