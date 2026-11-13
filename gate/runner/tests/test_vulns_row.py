"""The vulns row's judgement without a planted world (S1.1c-4 design note §2.5): `scan` with `gomods.load` faked.

Every world the driver can build imports x/mod, x/tools and x/exp, so the empty module set is proved here: the
loader returns no module and no error, and the row is red by name, `vulns: no module read`, never green on a scan
that read nothing. The positive witness first: the same copy and files with one module read is green.
"""

import hashlib
import io
import json
import os
import zipfile
from datetime import UTC, datetime, timedelta
from unittest import mock

from gate.runner import vulndb
from gate.runner.rows import gomods, vulns
from gate.runner.tests import support
from gate.runner.tests.test_deps import entry, listing

NOW = datetime(2026, 11, 13, 12, 0, 0, tzinfo=UTC)
ENTRY_ID = "GO-2099-0100"
AFFECTED = "example.com/not-in-the-tree"


def stamp_time(moment: datetime) -> str:
    return moment.strftime("%Y-%m-%dT%H:%M:%SZ")


def clean_db() -> bytes:
    """One entry, affecting a module no fake build list selects; the index files agreeing with it (§2.4)."""
    modified = stamp_time(NOW - timedelta(days=3))
    osv = {
        "id": ENTRY_ID,
        "modified": modified,
        "affected": [
            {
                "package": {"name": AFFECTED, "ecosystem": "Go"},
                "ranges": [{"type": "SEMVER", "events": [{"introduced": "0"}, {"fixed": "1.0.1"}]}],
            }
        ],
    }
    members = {
        "index/db.json": {"modified": modified},
        "index/modules.json": [{"path": AFFECTED, "vulns": [{"id": ENTRY_ID, "modified": modified}]}],
        "index/vulns.json": [{"id": ENTRY_ID, "modified": modified, "aliases": []}],
        f"ID/{ENTRY_ID}.json": osv,
    }
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as z:
        for name, value in members.items():
            z.writestr(name, json.dumps(value))
    return buf.getvalue()


def loaded(*modules: gomods.Module) -> gomods.Loaded:
    return gomods.Loaded(
        modules=modules,
        versions={".": {m.path: m.version for m in modules}, "gate/proofs": {}},
        graph=frozenset(m.path for m in modules),
        go_version="go1.27.4",
        errors=(),
    )


class Scan(support.GateTest):
    def setUp(self) -> None:
        super().setUp()
        self.copy = os.path.join(self.root, "vulndb")
        os.makedirs(os.path.join(self.repo, "tools", "allowlists"))
        os.mkdir(self.copy)
        with open(os.path.join(self.repo, "tools", "deps-allowlist.json"), "w", encoding="utf-8") as f:
            f.write(listing(entry("example.com/dep")))
        with open(os.path.join(self.repo, vulns.WAIVERS), "w", encoding="utf-8") as f:
            json.dump({"schema": 1, "note": "the vulns row's exemptions", "rule": "vulns", "entries": []}, f)
        body = clean_db()
        with open(os.path.join(self.copy, "vulndb.zip"), "wb") as f:
            f.write(body)
        with open(os.path.join(self.copy, "stamp.json"), "w", encoding="utf-8") as f:
            stamp = {
                "fetched": stamp_time(NOW - timedelta(days=1)),
                "sha256": hashlib.sha256(body).hexdigest(),
                "url": vulndb.CANONICAL_URL,
            }
            json.dump(stamp, f)

    def scan(self, result: gomods.Loaded) -> tuple[list[str], list[str]]:
        with mock.patch.object(gomods, "load", lambda row_dir, env: result):
            return vulns.scan(self.run_dir, {}, repo=self.repo, copy_root=self.copy, now=NOW)

    def test_one_module_read_and_no_finding_is_green(self):
        # The positive witness: this copy, register and waivers list give no fault once a module is read.
        dep = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)
        faults, lines = self.scan(loaded(dep))
        self.assertEqual(faults, [])
        self.assertTrue(any("1 entries; 1 modules (1 built); stdlib and toolchain at go1.27.4" in x for x in lines))

    def test_an_empty_module_set_is_red_by_name(self):
        # Design §2.5: an empty module set is `vulns: no module read`, never a clean scan of nothing.
        faults, _ = self.scan(loaded())
        self.assertEqual(faults, ["vulns: no module read"])
