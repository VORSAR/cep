"""S1.1c-4's batched fix round, the vulns row and the deps row's tidy line (owner 2026-11-13, S1.1c-4's review and
break-it, decision 2): a member that cannot be read (BK-B1, CA-B1), the build list's (path, version) pairs (BK-B2),
a malformed `withdrawn` (BK-S1), the shared format check (CA-S2), UTC calendar days (BK-L1), "no stamp" beside a zip
(CA-L5), one fault for an unparseable Go version (CA-L9), the tidy line keyed on the exit code (CA-L3).

Every expected text is written from the review's or the break-it's text by hand, before the code.
"""

import hashlib
import io
import json
import os
import zipfile
import zlib
from datetime import UTC, datetime, timedelta
from unittest import mock

from gate.runner import vulndb
from gate.runner.rows import deps as deps_row
from gate.runner.rows import gomods, vulns
from gate.runner.tests import support
from gate.runner.tests import test_vulns_row as base
from gate.runner.tests.test_breakit_s11c4 import db_members, osv, zip_of
from gate.runner.tests.test_deps import entry, listing
from gate.runner.tests.test_deps_rows import FakeRun
from gate.runner.tests.test_vulndb import NOW, ROOT, SHA, member_db, semver, stamp
from gate.runner.tests.test_vulndb import entry as osv_entry
from gate.runner.tests.test_vulns_row import loaded, stamp_time

DEP = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)


class _Raising:
    """A zip whose every `open` raises `error` (the member's bytes as zipfile would fail on them)."""

    def __init__(self, error: BaseException) -> None:
        self.error = error

    def open(self, name: str):
        raise self.error

    def namelist(self) -> list[str]:
        return []


class MemberUnreadable(support.GateTest):
    """BK-B1 / CA-B1: a member whose read fails is `vulns: member <name> cannot be read: <why>`, never a crash."""

    def test_copy_read_turns_every_read_failure_into_member_unreadable(self):
        for error in (
            zipfile.BadZipFile("Bad CRC-32 for file 'ID/GO-2099-0200.json'"),
            zlib.error("Error -3 while decompressing data: invalid code lengths set"),
            EOFError("Compressed file ended before the end-of-stream marker was reached"),
            RuntimeError("File 'ID/GO-2099-0200.json' is encrypted, password required for extraction"),
            NotImplementedError("That compression method is not supported"),
            ValueError("bad"),
            OSError("I/O error"),
        ):
            with self.subTest(type(error).__name__):
                copy = vulns.Copy("/x", zip=_Raising(error))
                with self.assertRaises(vulndb.MemberUnreadable) as caught:
                    copy.read("ID/GO-2099-0200.json")
                self.assertEqual(caught.exception.name, "ID/GO-2099-0200.json")
                self.assertIn(str(error), caught.exception.why)

    def test_a_missing_member_is_still_none_the_witness(self):
        copy = vulns.Copy("/x", zip=_Raising(KeyError("ID/GO-2099-0200.json")))
        self.assertIsNone(copy.read("ID/GO-2099-0200.json"))

    def test_check_db_names_an_unreadable_member(self):
        members = member_db()

        def read(name: str) -> bytes | None:
            if name == "ID/GO-2099-0100.json":
                raise vulndb.MemberUnreadable(name, "Bad CRC-32 for file 'ID/GO-2099-0100.json'")
            return members.get(name)

        faults = vulndb.check_db(read, sorted(members), stamp(), root=ROOT, zip_sha256=SHA, now=NOW)
        self.assertEqual(
            faults,
            ["vulns: member ID/GO-2099-0100.json cannot be read: Bad CRC-32 for file 'ID/GO-2099-0100.json'"],
        )

    def test_an_unreadable_db_json_is_named_once(self):
        members = member_db()

        def read(name: str) -> bytes | None:
            if name == vulndb.DB_JSON:
                raise vulndb.MemberUnreadable(name, "Error -3 while decompressing data")
            return members.get(name)

        faults = vulndb.check_db(read, sorted(members), stamp(), root=ROOT, zip_sha256=SHA, now=NOW)
        self.assertEqual(faults, ["vulns: member index/db.json cannot be read: Error -3 while decompressing data"])

    def test_entries_of_marks_the_scan_incomplete(self):
        body = zip_of(db_members(osv("GO-2099-0200", "example.com/dep", "1.0.1")))
        copy = vulns.Copy("/x", zip=zipfile.ZipFile(io.BytesIO(body)))
        entries, complete = vulns.entries_of(copy)
        self.assertEqual((len(entries), complete), (1, True))  # the witness: the same copy read whole
        real = copy.zip.open
        copy.cache.clear()

        def open_(name: str):
            if name == "ID/GO-2099-0200.json":
                raise zlib.error("Error -3 while decompressing data: invalid code lengths set")
            return real(name)

        copy.zip = mock.Mock(open=open_)
        self.assertEqual(vulns.entries_of(copy), ([], False))


class Findings(support.GateTest):
    def test_bk_b2_one_path_at_two_versions_gives_a_finding_each(self):
        e = osv_entry("GO-2099-0301", ("example.com/z", semver({"introduced": "0"}, {"fixed": "2.0.0"})))
        found, faults = vulndb.findings({"example.com/z": {"v1.5.0", "v1.0.0"}}, frozenset(), "go1.27.4", [e])
        self.assertEqual(faults, [])
        self.assertEqual(
            found,
            [
                vulndb.Finding("GO-2099-0301", "example.com/z", "v1.0.0", "2.0.0", True),
                vulndb.Finding("GO-2099-0301", "example.com/z", "v1.5.0", "2.0.0", True),
            ],
        )

    def test_bk_s1_withdrawn_is_skipped_only_as_an_rfc_3339_time(self):
        live = osv_entry("GO-2099-0302", ("example.com/a", semver({"introduced": "0"})))
        found, faults = vulndb.findings(
            {"example.com/a": "v1.0.0"}, frozenset(), "go1.27.4", [{**live, "withdrawn": None}]
        )
        self.assertEqual((len(found), faults), (1, []))  # null means live: the witness
        found, faults = vulndb.findings(
            {"example.com/a": "v1.0.0"}, frozenset(), "go1.27.4", [{**live, "withdrawn": "2026-01-01T00:00:00Z"}]
        )
        self.assertEqual((found, faults), ([], []))
        for value, shown in ((True, "true"), ("yesterday", '"yesterday"'), (1, "1"), ("2026-01-01", '"2026-01-01"')):
            with self.subTest(shown):
                found, faults = vulndb.findings(
                    {"example.com/a": "v1.0.0"}, frozenset(), "go1.27.4", [{**live, "withdrawn": value}]
                )
                self.assertEqual(found, [])
                self.assertEqual(
                    faults,
                    [("GO-2099-0302", "", f"vulns: GO-2099-0302: withdrawn {shown} is not an RFC 3339 time")],
                )

    def test_ca_l9_an_unparseable_go_version_is_one_fault(self):
        entries = [
            osv_entry(f"GO-2099-03{n}", (module, semver({"introduced": "0"})))
            for n, module in (("10", "stdlib"), ("11", "toolchain"), ("12", "stdlib"))
        ]
        found, faults = vulndb.findings({}, frozenset(), "go1.27.4 X:nodwarf5", entries)
        self.assertEqual(found, [])
        self.assertEqual(
            faults,
            [("", "", "vulns: the go version go1.27.4 X:nodwarf5 cannot be ordered: stdlib and toolchain not judged")],
        )


class Format(support.GateTest):
    """CA-S2: one format check, shared by the fetch and the row; the db.json fault names what is wrong."""

    def check(self, members: dict[str, bytes]) -> list[str]:
        return vulndb.check_format(members.get, sorted(members))

    def test_a_clean_database_has_no_format_fault(self):
        self.assertEqual(self.check(member_db()), [])

    def test_the_db_json_fault_names_what_is_wrong(self):
        for db, why in (
            ({"modified": "2026-11-11T02:51:11Z", "y": 1}, "unknown key y"),
            ({"modified": "2026-11-11T02:51:11Z", "a": 1, "b": 2}, "unknown keys a, b"),
            ({}, "no modified"),
            ({"modified": "2026-11-11"}, 'modified "2026-11-11" is not an RFC 3339 time'),
            ({"modified": 5}, "modified 5 is not an RFC 3339 time"),
            ([], "not a JSON object"),
            (b"not json", "not JSON"),
        ):
            with self.subTest(why):
                self.assertIn(
                    f"vulns: index/db.json is missing or malformed: {why}",
                    self.check(member_db(**{"index/db.json": db})),
                )

    def test_an_absent_db_json_keeps_its_text(self):
        self.assertIn("vulns: index/db.json is missing or malformed", self.check(member_db(**{"index/db.json": None})))

    def test_the_layout_and_the_index_shapes_are_format(self):
        faults = self.check({**member_db(**{"index/modules.json": []}), "index/new.json": b"{}"})
        self.assertIn("vulns: member index/new.json is not in the database's layout", faults)
        self.assertIn("vulns: index/modules.json is missing, malformed or empty", faults)

    def test_an_unreadable_member_is_format(self):
        members = member_db()

        def read(name: str) -> bytes | None:
            if name == "ID/GO-2099-0100.json":
                raise vulndb.MemberUnreadable(name, "Bad CRC-32")
            return members.get(name)

        self.assertIn(
            "vulns: member ID/GO-2099-0100.json cannot be read: Bad CRC-32",
            vulndb.check_format(read, sorted(members)),
        )


class UtcDays(support.GateTest):
    """BK-L1: fetched, modified and now each counted as UTC calendar days."""

    def check(self, db_modified: str, fetched: str, now: datetime) -> list[str]:
        members = member_db(**{"index/db.json": {"modified": db_modified}})
        return vulndb.check_db(members.get, sorted(members), stamp(fetched=fetched), root=ROOT, zip_sha256=SHA, now=now)

    def test_the_database_age_counts_utc_days_for_an_offset(self):
        # 2026-10-14T01:00:00+05:00 is 2026-10-13T20:00Z: 31 UTC calendar days before 2026-11-13.
        self.assertEqual(self.check("2026-10-14T00:00:00Z", "2026-11-12T08:00:00Z", NOW), [])  # 30: the witness
        self.assertIn(
            "vulns: the database has not changed since 2026-10-13 (31 days, more than 30): check vuln.go.dev, or add"
            " a dated vulndb exemption naming a known issue",
            self.check("2026-10-14T01:00:00+05:00", "2026-11-12T08:00:00Z", NOW),
        )

    def test_now_with_an_offset_counts_its_utc_day(self):
        # now 2026-11-13T01:00:00+05:00 is 2026-11-12T20:00Z: a copy fetched 2026-11-05 is 7 UTC days old.
        now = datetime.fromisoformat("2026-11-13T01:00:00+05:00")
        self.assertEqual(self.check("2026-11-01T00:00:00Z", "2026-11-05T12:00:00Z", now), [])
        late = datetime(2026, 11, 13, 0, 0, 1, tzinfo=UTC)
        self.assertIn(  # the witness: the same copy at a UTC now one day on is red
            "vulns: the copy was fetched 8 days ago (2026-11-05), more than 7: run make vulndb",
            self.check("2026-11-01T00:00:00Z", "2026-11-05T12:00:00Z", late),
        )


class _Row(support.GateTest):
    """The row with gomods.load faked (test_vulns_row), a register, and the waivers in WAIVERS."""

    scan = base.Scan.scan
    WAIVERS: tuple[dict, ...] = ()

    def setUp(self) -> None:
        super().setUp()
        self.copy = os.path.join(self.root, "vulndb")
        os.makedirs(os.path.join(self.repo, "tools", "allowlists"))
        os.mkdir(self.copy)
        with open(os.path.join(self.repo, "tools", "deps-allowlist.json"), "w", encoding="utf-8") as f:
            f.write(listing(entry("example.com/dep")))
        with open(os.path.join(self.repo, vulns.WAIVERS), "w", encoding="utf-8") as f:
            listed = {"schema": 1, "note": "the vulns row's exemptions", "rule": "vulns", "entries": list(self.WAIVERS)}
            json.dump(listed, f)


class NoStampBesideAZip(_Row):
    """CA-L5: with a zip present "no stamp" is an integrity fault the `vulndb` waiver never silences."""

    WAIVERS = (
        {
            "target": "vulndb",
            "kind": "TEMP",
            "reason": "vuln.go.dev is down",
            "sprint": "S1.1c-4",
            "expiry": "2026-12-01",
            "issue": "S1.1c-4-9",
        },
    )

    def test_no_zip_and_no_stamp_is_exempted_the_witness(self):
        faults, lines = self.scan(loaded(DEP))
        self.assertEqual(faults, [])
        self.assertTrue(any(line.startswith("vulns: not scanned: vulns: no vulndb.zip at ") for line in lines), lines)

    def test_a_zip_with_no_stamp_is_red_under_the_vulndb_waiver(self):
        with open(os.path.join(self.copy, "vulndb.zip"), "wb") as f:
            f.write(zip_of(db_members(osv("GO-2099-0400", "example.com/other", "1.0.1"))))
        faults, _ = self.scan(loaded(DEP))
        self.assertIn(f"vulns: no stamp.json at {self.copy}: run make vulndb", faults)


class CorruptMemberRow(_Row):
    """BK-B1 through the row: a deflate stream broken inside a member is one named line and an incomplete scan."""

    def test_a_broken_deflate_stream_is_a_named_line(self):
        members = db_members(osv("GO-2099-0500", "example.com/dep", "1.0.1"))
        name = "ID/GO-2099-0500.json"
        good = zip_of(members)
        body = bytearray(good)
        info = zipfile.ZipFile(io.BytesIO(good)).getinfo(name)
        start = info.header_offset + 30 + len(info.filename) + len(info.extra)
        body[start : start + 4] = b"\xff\xff\xff\xff"  # the member's deflate stream, its central entry intact
        with open(os.path.join(self.copy, "vulndb.zip"), "wb") as f:
            f.write(bytes(body))
        with open(os.path.join(self.copy, "stamp.json"), "w", encoding="utf-8") as f:
            json.dump(
                {
                    "fetched": stamp_time(NOW - timedelta(days=1)),
                    "sha256": hashlib.sha256(bytes(body)).hexdigest(),
                    "url": vulndb.CANONICAL_URL,
                },
                f,
            )
        faults, _ = self.scan(loaded(DEP))
        self.assertTrue(any(f.startswith(f"vulns: member {name} cannot be read: ") for f in faults), faults)


class TidyExitCode(support.GateTest):
    """CA-L3: "could not run" keyed on the exit code, never on what stderr holds."""

    def tidy(self, outputs: dict[str, tuple[str, str, int]]) -> list[str]:
        fake = FakeRun({mod: (out, err) for mod, (out, err, _) in outputs.items()})

        def run_status(row_dir, env, cwd, name, argv, *, ok=(0,)):
            return outputs[cwd][2], fake(row_dir, env, cwd, name, argv, ok=ok)

        with mock.patch.object(gomods, "run_status", run_status):
            return deps_row.tidy_lines(self.run_dir, {})

    def test_exit_0_with_a_stderr_warning_is_clean(self):
        warning = "go: warning: ignoring go.mod in system temp root\n"
        self.assertEqual(self.tidy({".": ("", warning, 0), "gate/proofs": ("", warning, 0)}), [])

    def test_exit_1_with_no_header_and_only_downloads_is_could_not_run(self):
        self.assertEqual(
            self.tidy({".": ("", "", 0), "gate/proofs": ("", "go: downloading example.com/a v1.0.0\n", 1)}),
            [
                "deps: go mod tidy could not run offline in gate/proofs: exit 1 and no diff header: run go mod"
                " download in gate/proofs"
            ],
        )

    def test_exit_1_quotes_the_last_stderr_line_not_a_download(self):
        err = "go: downloading example.com/a v1.0.0\ngo: first\ngo: last\ngo: downloading example.com/b v1.0.0\n"
        self.assertEqual(
            self.tidy({".": ("", err, 1), "gate/proofs": ("", "", 0)}),
            ["deps: go mod tidy could not run offline in .: go: last: run go mod download in ."],
        )

    def test_gomods_run_status_returns_the_code_and_stdout(self):
        code, out = gomods.run_status(
            self.run_dir, dict(os.environ), self.root, "probe", ["git", "--no-such-flag"], ok=(0, 129)
        )
        self.assertEqual((code, out), (129, ""))
        code, out = gomods.run_status(self.run_dir, dict(os.environ), self.root, "probe", ["git", "--version"])
        self.assertEqual(code, 0)
        self.assertTrue(out.startswith("git version "), out)
