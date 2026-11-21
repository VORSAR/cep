"""The vulns row's judgement without a planted world (S1.1c-4 design note §2.5): `scan` with `gomods.load` faked.

Every world the driver can build imports x/mod, x/tools and x/exp, so the empty module set is proved here: the
loader returns no module and no error, and the row is red by name, `vulns: no module read`, never green on a scan
that read nothing. The positive witness first: the same copy and files with one module read is green.
"""

import errno
import hashlib
import io
import json
import os
import unittest
import zipfile
from datetime import UTC, datetime, timedelta
from unittest import mock

from gate.runner import vulndb
from gate.runner.rows import gomods, vulns
from gate.runner.tests import support
from gate.runner.tests.test_deps import entry, listing
from gate.toolchain import manifest

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

    def waive(self, *targets: str) -> None:
        entries = [
            {
                "target": target,
                "kind": "TEMP",
                "reason": "a planted exemption",
                "sprint": "S1.1c-5",
                "expiry": (NOW + timedelta(days=10)).date().isoformat(),
                "issue": "S1.1c-5-4",
            }
            for target in targets
        ]
        with open(os.path.join(self.repo, vulns.WAIVERS), "w", encoding="utf-8") as f:
            json.dump({"schema": 1, "note": "the vulns row's exemptions", "rule": "vulns", "entries": entries}, f)


class Exempt(Scan):
    """S1.1c-5 design §6, §8 row 5.5 (DQ-S9): an exemption matching no finding is red on a complete scan. The rule is
    today's (rows/vulns.py `scan`: `vulns: <target> matches nothing: remove it`, judged when `complete`); this test
    pins it for the module pairs and for a declared tool's pair."""

    def test_an_exemption_matching_no_finding_is_red(self):
        dep = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)
        hit = gomods.Module(AFFECTED, "v1.0.0", "", "", "", True)
        self.waive(f"{ENTRY_ID} {AFFECTED}")
        self.assertEqual(self.scan(loaded(hit))[0], [])  # witness: the exemption silences its finding
        for target in (f"{ENTRY_ID} {AFFECTED}", f"{ENTRY_ID} golangci-lint"):
            with self.subTest(target=target):
                self.waive(target)
                faults, _ = self.scan(loaded(dep))
                self.assertEqual(faults, [f"vulns: {target} matches nothing: remove it"])


class UnreadableCopyFile(Scan):
    """A file of the copy that exists but cannot be read (a directory in its place) is one named fault, never a
    traceback out of scan; an unreadable vulndb.zip or stamp.json leaves the copy unscanned."""

    def scan_faults(self, name: str) -> list[str]:
        path = os.path.join(self.copy, name)
        if os.path.exists(path):
            os.remove(path)
        os.mkdir(path)
        dep = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)
        try:
            faults, self.lines = self.scan(loaded(dep))
        except OSError as e:  # the row died with a traceback and no named fault
            self.fail(f"scan raised {e!r}")
        return faults

    def unreadable(self, name: str) -> str:
        return f"vulns: cannot read {os.path.join(self.copy, name)}: {os.strerror(errno.EISDIR)}: run make vulndb"

    def test_a_directory_in_place_of_the_zip_is_named(self):
        self.assertEqual(self.scan_faults("vulndb.zip"), [self.unreadable("vulndb.zip")])

    def test_a_directory_in_place_of_the_stamp_is_named_and_the_copy_is_not_scanned(self):
        self.assertEqual(self.scan_faults("stamp.json"), [self.unreadable("stamp.json")])
        self.assertFalse(any(" entries; " in x for x in self.lines), self.lines)

    def test_a_directory_in_place_of_the_fetch_error_is_named_and_the_copy_is_scanned(self):
        self.assertEqual(self.scan_faults("fetch-error.txt"), [self.unreadable("fetch-error.txt")])
        self.assertTrue(any("1 entries; 1 modules (1 built)" in x for x in self.lines), self.lines)


TESTDATA = os.path.join(support.REPO, "gate", "runner", "testdata", "buildinfo")
TOOL = "golangci-lint"
PIN = "v2.15.0"


def recorded(platform: str) -> str:
    with open(os.path.join(TESTDATA, f"golangci-lint.{platform}.txt"), "rb") as f:
        return f.read().decode("utf-8")


def minimal(*lines: str, go: str = "go1.27.0") -> str:
    head = [f"/c/x: {go}", "\tpath\tgithub.com/golangci/golangci-lint/v2/cmd/golangci-lint"]
    return "\n".join([*head, *lines, "\tbuild\tCGO_ENABLED=0", ""])


MOD = f"\tmod\tgithub.com/golangci/golangci-lint/v2\t{PIN}\t"
DEP = "\tdep\texample.com/a\tv1.0.0\th1:AAAA="


class BuildInfo(unittest.TestCase):
    """S1.1c-5 design §6, §8 row 5.4: `go version -m <tool>` read; every refusal is named and gives no info."""

    def assert_red(self, code: int, text: str, *needles: str, pinned: str = PIN) -> None:
        info, faults = vulns.build_info(TOOL, pinned, code, text)
        self.assertIsNone(info)
        self.assertTrue(faults)
        for needle in (TOOL, *needles):
            self.assertTrue(any(needle in f for f in faults), (needle, faults))

    def test_exit_nonzero_is_red(self):
        self.assertIsNotNone(vulns.build_info(TOOL, PIN, 0, minimal(MOD, DEP))[0])  # witness
        self.assert_red(1, minimal(MOD, DEP), "exit 1")

    def test_no_go_line_is_red(self):
        for first in ("", "/c/x:", "/c/x: not-a-version", "\tpath\tx"):
            with self.subTest(first=first):
                self.assert_red(0, "\n".join([first, MOD, DEP, ""]))

    def test_devel_go_is_red(self):
        self.assert_red(0, minimal(MOD, DEP, go="devel go1.28-0123abcd"), "devel go1.28-0123abcd")

    def test_an_experiment_suffix_is_parsed_or_red(self):
        # vulndb.go_version refuses the suffix (test_vulndb's GoVersion pins it): never a silent strip.
        info, faults = vulns.build_info(TOOL, PIN, 0, minimal(MOD, DEP, go="go1.27.0 X:boringcrypto"))
        self.assertIsNone(info)
        self.assertEqual(
            faults, [f"vulns: {TOOL}: go version -m: the Go version 'go1.27.0 X:boringcrypto' cannot be ordered"]
        )

    def test_no_mod_line_is_red(self):
        self.assert_red(0, minimal(DEP), "mod")

    def test_an_empty_main_module_sum_is_accepted(self):
        for platform in ("darwin-arm64", "linux-amd64"):
            with self.subTest(platform=platform):
                info, faults = vulns.build_info(TOOL, PIN, 0, recorded(platform))
                self.assertEqual(faults, [])
                self.assertEqual((info.go_version, info.main_version, len(info.deps)), ("go1.27.0", PIN, 207))
                self.assertEqual(info.deps["golang.org/x/tools"], "v0.52.0")

    def test_mod_version_not_the_pin_is_red(self):
        self.assert_red(0, recorded("darwin-arm64"), "v2.15.0", "v2.15.1", pinned="v2.15.1")

    def test_zero_deps_is_red(self):
        self.assert_red(0, minimal(MOD), "dep")

    def test_a_replacement_is_applied(self):
        # The engine scan's precedent (rows/vulns.py `_module_map`, DA-S2): the replacement's path at its version.
        text = minimal(MOD, DEP, "\tdep\texample.com/b\tv1.0.0\th1:BBBB=", "\t=>\texample.com/c\tv2.0.0\th1:CCCC=")
        info, faults = vulns.build_info(TOOL, PIN, 0, text)
        self.assertEqual(faults, [])
        self.assertEqual(info.deps, {"example.com/a": "v1.0.0", "example.com/c": "v2.0.0"})

    def test_the_pair_names_the_tool(self):
        info = vulns.BuildInfo("go1.27.0", PIN, {"example.com/a": "v1.0.0"})
        entries = [
            {
                "id": "GO-2099-0201",
                "affected": [{"package": {"name": "stdlib", "ecosystem": "Go"}, "ranges": SEMVER_FIX}],
            },
            {
                "id": "GO-2099-0202",
                "affected": [{"package": {"name": "example.com/a", "ecosystem": "Go"}, "ranges": SEMVER_FIX}],
            },
            {
                "id": "GO-2099-0203",
                "affected": [{"package": {"name": "toolchain", "ecosystem": "Go"}, "ranges": SEMVER_FIX}],
            },
        ]
        found, faults = vulns.binary_findings(TOOL, info, entries)
        self.assertEqual(faults, [])
        self.assertEqual([pair for pair, _ in found], [f"GO-2099-0202 {TOOL}", f"GO-2099-0201 {TOOL}"])
        self.assertEqual([f.module for _, f in found], ["example.com/a", "stdlib"])

    def test_covered_counts_tools_json_once_whatever_it_declares(self):
        # Owner, 2026-11-14, during S1.1c-5's W1, decision 2: the six database and module files plus tools.json,
        # against the scope's seven, however many binaries tools.json declares; tools.json unread is not counted.
        def tool(name: str, go_built: bool) -> manifest.Tool:
            return manifest.Tool(
                name, "1", name, ".", (name,), name, "u", "a" * 64, "s/", {}, "t" * 64, go_built=go_built
            )

        self.assertEqual(vulns.COVERED, 6)
        for declared in ([], [TOOL], [TOOL, "other"]):
            with self.subTest(declared=declared):
                tools = [tool("ruff", False), *(tool(name, True) for name in declared)]
                self.assertEqual(sum(t.go_built for t in tools), len(declared))  # witness: as many as named
                self.assertEqual(vulns.covered(tools), 7)
        self.assertEqual(vulns.covered(None), 6)


SEMVER_FIX = [{"type": "SEMVER", "events": [{"introduced": "0"}, {"fixed": "99.0.0"}]}]
