"""S1.1c-4 break-it (QA with the-fool, step 7): the vulns row on hostile copies and edge build lists.

Each case is red on the build at 1b06251 for the reason in its comment; the fix makes it green.
"""

import hashlib
import io
import json
import os
import zipfile
from datetime import timedelta

from gate.runner import vulndb
from gate.runner.rows import gomods, vulns
from gate.runner.tests import support
from gate.runner.tests import test_vulns_row as base
from gate.runner.tests.test_deps import entry, listing
from gate.runner.tests.test_vulns_row import NOW, loaded, stamp_time

MODIFIED = stamp_time(NOW - timedelta(days=3))


def osv(entry_id: str, module: str, fixed: str, **extra: object) -> dict:
    return {
        "id": entry_id,
        "modified": MODIFIED,
        "affected": [
            {
                "package": {"name": module, "ecosystem": "Go"},
                "ranges": [{"type": "SEMVER", "events": [{"introduced": "0"}, {"fixed": fixed}]}],
            }
        ],
        **extra,
    }


def db_members(*entries: dict, modified: str = MODIFIED) -> dict[str, object]:
    """A database whose index files agree with `entries` (design §2.4)."""
    by_module: dict[str, list[dict]] = {}
    for e in entries:
        for block in e["affected"]:
            by_module.setdefault(block["package"]["name"], []).append({"id": e["id"], "modified": MODIFIED})
    members: dict[str, object] = {
        "index/db.json": {"modified": modified},
        "index/modules.json": [{"path": p, "vulns": v} for p, v in sorted(by_module.items())],
        "index/vulns.json": [{"id": e["id"], "modified": MODIFIED, "aliases": []} for e in entries],
    }
    for e in entries:
        members[f"ID/{e['id']}.json"] = e
    return members


def zip_of(members: dict[str, object], compression: int = zipfile.ZIP_DEFLATED) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", compression) as z:
        for name, value in members.items():
            z.writestr(name, json.dumps(value))
    return buf.getvalue()


class BreakIt(support.GateTest):
    scan = base.Scan.scan  # the row with gomods.load faked (test_vulns_row), without re-running its cases

    def setUp(self) -> None:
        super().setUp()
        self.copy = os.path.join(self.root, "vulndb")
        os.makedirs(os.path.join(self.repo, "tools", "allowlists"))
        os.mkdir(self.copy)
        with open(os.path.join(self.repo, "tools", "deps-allowlist.json"), "w", encoding="utf-8") as f:
            f.write(listing(entry("example.com/dep")))
        with open(os.path.join(self.repo, vulns.WAIVERS), "w", encoding="utf-8") as f:
            json.dump({"schema": 1, "note": "the vulns row's exemptions", "rule": "vulns", "entries": []}, f)

    def install(self, body: bytes, fetched: str | None = None) -> None:
        """`body` as the copy, with a stamp that matches it (as make vulndb would write after its validation)."""
        with open(os.path.join(self.copy, "vulndb.zip"), "wb") as f:
            f.write(body)
        with open(os.path.join(self.copy, "stamp.json"), "w", encoding="utf-8") as f:
            json.dump(
                {
                    "fetched": fetched or stamp_time(NOW - timedelta(days=1)),
                    "sha256": hashlib.sha256(body).hexdigest(),
                    "url": vulndb.CANONICAL_URL,
                },
                f,
            )

    def test_bk_1_a_member_whose_crc_fails_is_a_named_fault_not_a_crash(self):
        # Red today: Copy.read catches only KeyError, so a member whose bytes fail the zip's CRC (a bit flipped in
        # transit, or on disk before the stamp was written; make vulndb's validation reads only index/db.json)
        # raises zipfile.BadZipFile out of check_db, open_copy and scan: the row dies with a traceback instead of
        # the design's "every fault one line" (design §2.5; `vulns: ... is not a zip: run make vulndb`).
        dep = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)
        members = db_members(osv("GO-2099-0200", "example.com/dep", "1.0.1"))
        body = bytearray(zip_of(members, zipfile.ZIP_STORED))
        at = body.rfind(b'"GO-2099-0200", "modified"')  # inside ID/GO-2099-0200.json, the last member stored
        self.assertGreater(at, 0)
        body[at + 1] = ord("X")
        self.assertTrue(zipfile.is_zipfile(io.BytesIO(bytes(body))))  # still a zip; make vulndb accepts it
        self.install(bytes(body))
        try:
            faults, _ = self.scan(loaded(dep))
        except zipfile.BadZipFile as e:  # the crash
            self.fail(f"scan raised {type(e).__name__}: {e}")
        self.assertTrue(faults, "a corrupt member must be a named fault")

    def test_bk_2_a_replacement_that_is_also_in_the_build_list_is_still_scanned(self):
        # Red today: _module_map keys the scan by path, so `example.com/a => example.com/z v1.0.0` beside
        # example.com/z v1.5.0 in the same build list (go accepts it: checkMultiplePaths keys by path AND version,
        # cmd/go/internal/modload/load.go:2036) keeps only one version of example.com/z. Sorted by path, a's
        # replacement is written first and z's own version overwrites it: z v1.0.0, which is built and vulnerable
        # (fixed in 1.5.0), is never matched and the row is green.
        a = gomods.Module("example.com/a", "v1.0.0", "", "example.com/z", "v1.0.0", True)
        z = gomods.Module("example.com/z", "v1.5.0", "", "", "", True)
        self.install(zip_of(db_members(osv("GO-2099-0201", "example.com/z", "1.5.0"))))
        faults, _ = self.scan(loaded(a, z))
        self.assertIn("vulns: GO-2099-0201 example.com/z v1.0.0: fixed in 1.5.0", faults)

    def test_bk_3_a_withdrawn_that_is_not_a_time_is_a_fault_not_a_skip(self):
        # Red today: findings skips any entry whose `withdrawn` is truthy, so `"withdrawn": true` (or "no", or 1)
        # silently drops an entry that affects a built module: no finding, no fault, green. OSV's withdrawn is an
        # RFC 3339 time; x/vuln decodes it into a *time.Time and refuses anything else. The rule says an entry it
        # cannot judge is a named fault, never a skip (review DA-S1).
        dep = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)
        self.install(zip_of(db_members(osv("GO-2099-0202", "example.com/dep", "1.0.1", withdrawn=True))))
        faults, _ = self.scan(loaded(dep))
        self.assertTrue(
            any("GO-2099-0202" in f for f in faults), f"the malformed entry vanished without a word: {faults}"
        )

    def test_bk_4_the_copy_age_counts_utc_calendar_days_for_a_stamp_with_an_offset(self):
        # Red today: _stamp_faults takes fetched.date() in the stamp's own offset, not in UTC. Fetched
        # 2026-11-06T01:00:00+05:00 is 2026-11-05T20:00Z, 8 UTC calendar days before NOW (2026-11-13), more than
        # COPY_MAX_DAYS; the row reads 2026-11-06 (7 days) and stays green. Design §2.4: "(UTC calendar days)".
        # The stamp's grammar accepts any offset (_RFC3339), so the age must be judged after astimezone(UTC).
        dep = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)
        body = zip_of(db_members(osv("GO-2099-0203", "example.com/other", "1.0.1"), modified="2026-11-04T00:00:00Z"))
        self.install(body, "2026-11-06T01:00:00+05:00")
        faults, _ = self.scan(loaded(dep))
        self.assertIn("vulns: the copy was fetched 8 days ago (2026-11-05), more than 7: run make vulndb", faults)
