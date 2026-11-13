"""The vulnerability matcher and the copy's checks (S1.1c-4 design note §2.2, §2.4, §5; plan check §2 S1, §6 "The
scan"; owner 2026-11-13, S1.1c-4's plan check, decisions 1, 2 and 4): unit tables against gate/runner/vulndb.py, and
the agreement table over real database entries.

The agreement table's verdicts were measured once, outside the repository, with govulncheck v1.9.0 built by the
pinned go1.27.4, on 2026-11-13, in a scratch directory with its own GOPATH, GOMODCACHE and GOCACHE:

    go install golang.org/x/vuln/cmd/govulncheck@v1.9.0          # GOBIN=<scratch>/gopath/bin
    # query mode (x/vuln internal/scan/query.go: -mode query, one module@version per query), one planted database
    # per row: index/db.json {"modified":"2026-11-11T02:51:11Z"}, the entry's own index/vulns.json and
    # index/modules.json items copied from the real index, and ID/<id>.json the unmodified real entry (or, for the
    # rows marked PLANTED, the synthetic entry defined below):
    govulncheck -mode query -db file://<planted db> -format json <module>@<version>
    # verdict: affected when the output holds an {"osv": {"id": <entry id>}} message
    # source mode (x/vuln internal/vulncheck/vulncheck.go:138 skips withdrawn entries; GOVERSION read from go env):
    cd <scratch module importing net/http>; PATH=<go1.27.3 or go1.27.4>/bin:$PATH \
        govulncheck -db file://<db holding GO-2026-6859> -scan module -format json
    cd <scratch module requiring github.com/gorilla/sessions v1.2.2>; GOPROXY=off \
        govulncheck -db file://<db holding GO-2024-2730> -scan module -format json
    # verdict: affected when the output holds a {"finding": {"osv": <entry id>}} message

The real database copy read: vulndb.zip sha256 3b16a8d3ee29af03d9e8f78650c4286b9630569eb0542a93c4207e714c27ef11,
fetched 2026-11-13 (gate/runner/testdata/vulndb/NOTICE.md). No real entry has `introduced 0` for golang.org/x/exp or
events out of order (0 of 8,427 introduced and 5,916 fixed events today), so those two rows use PLANTED entries,
written below and measured the same way. Rows marked DIFFERS are query mode's three known differences (plan check
§0.18): there the matcher is right and govulncheck's query mode is not, as source mode (or x/vuln's own matcher on
the entry) shows.

Fixtures are built in code; nothing here reads the network or the real ~/.cache.
"""

import json
import os
import unittest
from datetime import UTC, datetime

from gate.runner import vulndb

TESTDATA = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "testdata", "vulndb")
NOW = datetime(2026, 11, 13, 12, 0, 0, tzinfo=UTC)
SHA = "ab" * 32
ROOT = "/home/dev/.cache/vorsar/cep/vulndb"  # the copy's directory, for the texts (design §2.4)


def real(entry_id: str) -> dict:
    with open(os.path.join(TESTDATA, f"{entry_id}.json"), "rb") as f:
        return json.loads(f.read())


def entry(entry_id: str, *blocks: tuple[str, list | None], withdrawn: str | None = None) -> dict:
    """A synthetic OSV entry: each block (package name, ranges or None for no `ranges` key)."""
    affected = []
    for name, ranges in blocks:
        block: dict = {"package": {"name": name, "ecosystem": "Go"}}
        if ranges is not None:
            block["ranges"] = ranges
        affected.append(block)
    out = {"schema_version": "1.3.1", "id": entry_id, "modified": "2026-11-01T00:00:00Z", "affected": affected}
    if withdrawn:
        out["withdrawn"] = withdrawn
    return out


def semver(*events: dict) -> list[dict]:
    return [{"type": "SEMVER", "events": list(events)}]


PLANTED_XEXP = entry("GO-2099-0001", ("golang.org/x/exp", semver({"introduced": "0"})))
PLANTED_OOO = entry("GO-2099-0002", ("example.com/ooo", semver({"fixed": "1.2.0"}, {"introduced": "1.0.0"})))
PLANTED_MUT = entry("GO-2099-0003", ("github.com/mutagen-io/mutagen", semver({"introduced": "0"})))
XEXP_PSEUDO = "v0.0.0-20261106192419-47ce03d25ef0"  # gate/proofs' pinned x/exp


class Versions(unittest.TestCase):
    """semver 2.0 order, which is x/mod's semver.Compare (design §2.2)."""

    ASCENDING = (
        "0.0.0-20190101000000-aaaaaaaaaaaa",
        "0.0.0-20261106192419-47ce03d25ef0",  # a pseudo-version is a pre-release of 0.0.0
        "0.0.0",
        "0.1.0-0",
        "0.1.0-alpha",
        "0.1.0-alpha.1",
        "0.1.0-alpha.beta",
        "0.1.0-beta",
        "0.1.0-beta.2",
        "0.1.0-beta.11",  # numeric identifiers numerically
        "0.1.0-rc.1",
        "0.1.0",
        "0.9.0",
        "0.10.0",  # cores numerically, never as text
        "1.0.0",
        "1.27.0-0",
        "1.27.0-rc.1",
        "1.27.0",
        "2.7.0-beta.1",
        "2.7.0-rc.0",
        "2.7.0",
        "10.0.0",
    )

    def test_versions_order_as_semver_2(self):
        parsed = [vulndb._parse(v) for v in self.ASCENDING]
        for low, high, a, b in zip(self.ASCENDING, self.ASCENDING[1:], parsed, parsed[1:], strict=False):
            with self.subTest(f"{low} < {high}"):
                self.assertLess(a, b)
                self.assertGreater(b, a)

    def test_a_leading_v_and_build_metadata_are_dropped(self):
        for a, b in (
            ("v1.2.3", "1.2.3"),
            ("2.7.0-rc.0+incompatible", "2.7.0-rc.0"),
            ("v2.6.2+incompatible", "2.6.2"),
            ("1.0.0+build.5", "1.0.0"),
        ):
            with self.subTest(a):
                self.assertEqual(vulndb._parse(a), vulndb._parse(b))

    def test_numeric_identifiers_sort_before_alphanumeric_ones(self):
        self.assertLess(vulndb._parse("1.0.0-1"), vulndb._parse("1.0.0-a"))
        self.assertLess(vulndb._parse("1.0.0-99"), vulndb._parse("1.0.0-0a"))

    def test_an_invalid_version_raises_never_reads_as_lowest(self):
        for bad in (
            "",
            "0",
            "1",
            "1.2",
            "1.2.3.4",
            "v",
            "01.2.3",
            "1.02.3",
            "1.2.3-",
            "1.2.3-01",
            "1.2.3-a..b",
            "1.2.3+",
            "x1.2.3",
            "go1.27.4",
            "devel go1.28-abc",
            " 1.2.3",
        ):
            with self.subTest(repr(bad)), self.assertRaises(vulndb.InvalidVersion):
                vulndb._parse(bad)


class GoVersion(unittest.TestCase):
    """`go env GOVERSION` to the database's stdlib versions (design §2.2; x/vuln's GoTagToSemver, no `v`)."""

    def test_go_tags_convert(self):
        for tag, want in (
            ("go1.27.4", "1.27.4"),
            ("go1.27", "1.27.0"),
            ("go1.28rc1", "1.28.0-rc.1"),
            ("go1.28beta2", "1.28.0-beta.2"),
            ("go1.27.10", "1.27.10"),
        ):
            with self.subTest(tag):
                self.assertEqual(vulndb.go_version(tag), want)

    def test_anything_else_is_invalid(self):
        # Stricter than GoTagToSemver, which keeps the first field of "go1.27.4 X:boringcrypto": the design names
        # an experiment suffix as a red, never a silent strip (design §2.2).
        for tag in ("devel go1.28-0123abcd Thu Nov 12 00:00:00 2026 +0000", "go1.27.4 X:boringcrypto", "", "1.27.4"):
            with self.subTest(tag), self.assertRaises(vulndb.InvalidVersion):
                vulndb.go_version(tag)


class Affected(unittest.TestCase):
    """x/vuln v1.9.0 internal/semver/affects.go:13-90 (plan check §0.17; design §2.2)."""

    def test_no_ranges_affects_every_version(self):
        self.assertTrue(vulndb.affected("1.2.3", []))

    def test_no_semver_range_affects_every_version(self):
        self.assertTrue(vulndb.affected("1.2.3", [{"type": "GIT", "events": [{"introduced": "abc"}]}]))
        self.assertTrue(vulndb.affected("1.2.3", [{"type": "ECOSYSTEM", "events": [{"fixed": "1.0.0"}]}]))

    def test_introduced_zero_is_minus_infinity(self):
        ranges = semver({"introduced": "0"}, {"fixed": "0.1.0"})
        self.assertTrue(vulndb.affected(XEXP_PSEUDO, semver({"introduced": "0"})))
        self.assertTrue(vulndb.affected("0.0.0-20240101000000-abcdefabcdef", ranges))
        self.assertTrue(vulndb.affected("0.0.0", ranges))
        self.assertFalse(vulndb.affected("0.1.0", ranges))

    def test_fixed_is_exclusive_and_introduced_inclusive(self):
        ranges = semver({"introduced": "1.0.0"}, {"fixed": "1.2.0"})
        for version, want in (
            ("0.9.9", False),
            ("1.0.0-rc.1", False),
            ("1.0.0", True),
            ("1.1.9", True),
            ("1.2.0-rc.1", True),
            ("1.2.0", False),
        ):
            with self.subTest(version):
                self.assertEqual(vulndb.affected(version, ranges), want)

    def test_events_out_of_order_are_sorted_first(self):
        ranges = semver({"fixed": "1.2.0"}, {"introduced": "1.0.0"})
        for version, want in (("0.9.0", False), ("1.0.0", True), ("1.1.0", True), ("1.2.0", False)):
            with self.subTest(version):
                self.assertEqual(vulndb.affected(version, ranges), want)

    def test_two_ranges_either_holds(self):
        ranges = [*semver({"introduced": "0"}, {"fixed": "1.0.0"}), *semver({"introduced": "2.0.0"})]
        for version, want in (("0.5.0", True), ("1.5.0", False), ("2.0.0", True), ("9.0.0", True)):
            with self.subTest(version):
                self.assertEqual(vulndb.affected(version, ranges), want)

    def test_an_open_range_after_a_fix(self):
        ranges = semver({"introduced": "0"}, {"fixed": "0.0.0-20260617104213-10b7825c714c"}, {"introduced": "0.1.0"})
        self.assertTrue(vulndb.affected("0.0.0-20260101000000-aaaaaaaaaaaa", ranges))
        self.assertFalse(vulndb.affected("0.0.0-20260701000000-aaaaaaaaaaaa", ranges))
        self.assertTrue(vulndb.affected("0.1.0", ranges))

    def test_incompatible_is_dropped_on_both_sides(self):
        ranges = semver({"introduced": "0"}, {"fixed": "2.7.0-rc.0+incompatible"})
        self.assertTrue(vulndb.affected("2.6.2+incompatible", ranges))
        self.assertFalse(vulndb.affected("2.7.0-rc.0", ranges))
        self.assertFalse(vulndb.affected("2.7.0+incompatible", ranges))

    def test_an_unknown_event_key_is_a_named_red(self):
        for key in ("last_affected", "limit", "Fixed"):
            with self.subTest(key):
                with self.assertRaises(vulndb.UnknownEvent) as caught:
                    vulndb.affected("1.0.0", semver({"introduced": "0"}, {key: "2.0.0"}))
                self.assertIn(key, str(caught.exception))

    def test_an_invalid_version_on_either_side_raises(self):
        with self.assertRaises(vulndb.InvalidVersion):
            vulndb.affected("not-a-version", semver({"introduced": "0"}, {"fixed": "1.0.0"}))
        with self.assertRaises(vulndb.InvalidVersion):
            vulndb.affected("1.0.0", semver({"introduced": "0"}, {"fixed": "1.0"}))


def scan(modules: dict[str, str], go: str, entries: list[dict], never_built: frozenset[str] = frozenset()) -> list:
    """findings() where no entry is bad: its faults asserted empty, its findings returned."""
    found, faults = vulndb.findings(modules, never_built, go, entries)
    if faults:
        raise AssertionError(f"unexpected faults: {faults}")
    return found


class Findings(unittest.TestCase):
    """design §2.2 findings(modules, never_built, go_version, entries) -> (findings, faults)."""

    def test_withdrawn_entries_are_skipped(self):
        withdrawn = entry(
            "GO-2099-0010", ("example.com/a", semver({"introduced": "0"})), withdrawn="2026-01-01T00:00:00Z"
        )
        self.assertEqual(scan({"example.com/a": "v1.0.0"}, "go1.27.4", [withdrawn]), [])

    def test_two_blocks_for_one_module_give_one_finding(self):
        e = entry(
            "GO-2099-0011",
            ("example.com/a", semver({"introduced": "0"}, {"fixed": "1.1.0"})),
            ("example.com/a", semver({"introduced": "0"}, {"fixed": "1.2.0"})),
        )
        got = scan({"example.com/a": "v1.0.0"}, "go1.27.4", [e])
        self.assertEqual([(f.id, f.module) for f in got], [("GO-2099-0011", "example.com/a")])

    def test_one_entry_naming_two_modules_gives_a_finding_each(self):
        e = entry(
            "GO-2099-0012",
            ("example.com/b", semver({"introduced": "0"}, {"fixed": "2.0.0"})),
            ("example.com/a", semver({"introduced": "0"}, {"fixed": "2.0.0"})),
        )
        got = scan({"example.com/a": "v1.0.0", "example.com/b": "v1.5.0"}, "go1.27.4", [e])
        self.assertEqual(
            got,
            [
                vulndb.Finding("GO-2099-0012", "example.com/a", "v1.0.0", "2.0.0", True),
                vulndb.Finding("GO-2099-0012", "example.com/b", "v1.5.0", "2.0.0", True),
            ],
        )

    def test_a_never_built_module_gives_a_finding_not_built(self):
        e = entry("GO-2099-0013", ("golang.org/x/net", semver({"introduced": "0"}, {"fixed": "0.62.0"})))
        got = scan({"golang.org/x/net": "v0.61.0"}, "go1.27.4", [e], frozenset({"golang.org/x/net"}))
        self.assertEqual(got, [vulndb.Finding("GO-2099-0013", "golang.org/x/net", "v0.61.0", "0.62.0", False)])

    def test_stdlib_and_toolchain_are_read_at_the_go_version(self):
        e = entry(
            "GO-2099-0014",
            ("stdlib", semver({"introduced": "0"}, {"fixed": "1.27.5"})),
            ("toolchain", semver({"introduced": "0"}, {"fixed": "1.27.5"})),
        )
        got = scan({}, "go1.27.4", [e])
        self.assertEqual(
            [(f.module, f.version, f.fixed, f.built) for f in got],
            [
                ("stdlib", "go1.27.4", "1.27.5", True),
                ("toolchain", "go1.27.4", "1.27.5", True),
            ],
        )
        self.assertEqual(scan({}, "go1.27.5", [e]), [])

    def test_fixed_is_the_smallest_fix_above_the_version_in_the_holding_range(self):
        e = entry(
            "GO-2099-0015",
            (
                "example.com/a",
                semver({"introduced": "0"}, {"fixed": "1.1.0"}, {"introduced": "1.5.0"}, {"fixed": "1.7.0"}),
            ),
        )
        got = scan({"example.com/a": "v1.6.0"}, "go1.27.4", [e])
        self.assertEqual([f.fixed for f in got], ["1.7.0"])
        open_ = entry("GO-2099-0016", ("example.com/a", semver({"introduced": "0"})))
        self.assertEqual([f.fixed for f in scan({"example.com/a": "v1.6.0"}, "go1.27.4", [open_])], [""])

    def test_findings_are_sorted_by_module_then_id(self):
        es = [
            entry("GO-2099-0022", ("example.com/b", semver({"introduced": "0"}))),
            entry("GO-2099-0021", ("example.com/b", semver({"introduced": "0"}))),
            entry("GO-2099-0023", ("example.com/a", semver({"introduced": "0"}))),
        ]
        got = scan({"example.com/a": "v1.0.0", "example.com/b": "v1.0.0"}, "go1.27.4", es)
        self.assertEqual(
            [(f.module, f.id) for f in got],
            [("example.com/a", "GO-2099-0023"), ("example.com/b", "GO-2099-0021"), ("example.com/b", "GO-2099-0022")],
        )

    def test_a_module_not_read_and_another_ecosystem_are_ignored(self):
        other = entry("GO-2099-0024", ("example.com/a", semver({"introduced": "0"})))
        other["affected"][0]["package"]["ecosystem"] = "npm"
        absent = entry("GO-2099-0025", ("example.com/zzz", semver({"introduced": "0"})))
        self.assertEqual(scan({"example.com/a": "v1.0.0"}, "go1.27.4", [other, absent]), [])

    def test_an_unknown_event_is_a_fault_naming_the_entry(self):
        e = entry("GO-2099-0026", ("example.com/a", semver({"introduced": "0"}, {"last_affected": "1.0.0"})))
        found, faults = vulndb.findings({"example.com/a": "v1.0.0"}, frozenset(), "go1.27.4", [e])
        self.assertEqual(found, [])
        self.assertEqual(
            faults, [("GO-2099-0026", "example.com/a", "vulns: GO-2099-0026: unknown range event last_affected")]
        )

    def test_an_invalid_version_is_a_fault_naming_the_entry_and_module(self):
        e = entry("GO-2099-0027", ("example.com/a", semver({"introduced": "0"}, {"fixed": "1.0"})))
        found, faults = vulndb.findings({"example.com/a": "v0.5.0"}, frozenset(), "go1.27.4", [e])
        self.assertEqual(found, [])
        self.assertEqual(
            faults,
            [("GO-2099-0027", "example.com/a", "vulns: GO-2099-0027 example.com/a: version 1.0 cannot be ordered")],
        )
        ok = entry("GO-2099-0028", ("example.com/a", semver({"introduced": "0"})))
        found, faults = vulndb.findings({"example.com/a": "banana"}, frozenset(), "go1.27.4", [ok])
        self.assertEqual(found, [])
        self.assertEqual(
            faults,
            [("GO-2099-0028", "example.com/a", "vulns: GO-2099-0028 example.com/a: version banana cannot be ordered")],
        )

    def test_a_bad_entry_between_two_good_ones_never_stops_the_walk(self):
        # Review DA-S1: the walk goes on past a bad entry; both good entries give their findings, the bad one a fault.
        first = entry("GO-2099-0031", ("example.com/a", semver({"introduced": "0"}, {"fixed": "2.0.0"})))
        bad = entry("GO-2099-0032", ("example.com/a", semver({"introduced": "0"}, {"limit": "3.0.0"})))
        last = entry("GO-2099-0033", ("example.com/b", semver({"introduced": "0"})))
        found, faults = vulndb.findings(
            {"example.com/a": "v1.0.0", "example.com/b": "v1.0.0"}, frozenset(), "go1.27.4", [first, bad, last]
        )
        self.assertEqual(
            found,
            [
                vulndb.Finding("GO-2099-0031", "example.com/a", "v1.0.0", "2.0.0", True),
                vulndb.Finding("GO-2099-0033", "example.com/b", "v1.0.0", "", True),
            ],
        )
        self.assertEqual(faults, [("GO-2099-0032", "example.com/a", "vulns: GO-2099-0032: unknown range event limit")])

    def test_faults_are_sorted_by_module_then_id(self):
        es = [
            entry("GO-2099-0042", ("example.com/b", semver({"introduced": "0"}, {"fixed": "1"}))),
            entry("GO-2099-0041", ("example.com/b", semver({"introduced": "0"}, {"fixed": "1"}))),
            entry("GO-2099-0043", ("example.com/a", semver({"introduced": "0"}, {"fixed": "1"}))),
        ]
        found, faults = vulndb.findings(
            {"example.com/a": "v1.0.0", "example.com/b": "v1.0.0"}, frozenset(), "go1.27.4", es
        )
        self.assertEqual(found, [])
        self.assertEqual(
            [(m, i) for i, m, _ in faults],
            [("example.com/a", "GO-2099-0043"), ("example.com/b", "GO-2099-0041"), ("example.com/b", "GO-2099-0042")],
        )

    def test_an_invalid_go_version_is_a_fault_never_a_finding_or_a_crash(self):
        # design §2.2: go_version() raises InvalidVersion for a tag it cannot convert; findings() converts it once,
        # before the walk, and collects one fault (the tag as given), the walk going on (review CA-L9).
        e = entry("GO-2099-0029", ("stdlib", semver({"introduced": "0"}, {"fixed": "1.27.5"})))
        found, faults = vulndb.findings({}, frozenset(), "devel go1.28-0123abcd", [e])
        self.assertEqual(found, [])
        self.assertEqual(
            faults,
            [
                (
                    "",
                    "",
                    "vulns: the go version devel go1.28-0123abcd cannot be ordered: stdlib and toolchain not judged",
                )
            ],
        )

    def test_the_cap_constants(self):
        self.assertEqual((vulndb.FETCH_CAP, vulndb.MEMBER_CAP), (64 << 20, 4 << 20))  # design §2.3, §2.4
        self.assertTrue(issubclass(vulndb.UnknownEvent, ValueError) and issubclass(vulndb.MemberTooLarge, ValueError))


# (module, version, entry id, verdict, mode) — every verdict measured as the header says. The stdlib rows give the
# go tag findings() reads; query mode was asked at its GoTagToSemver form (go1.27rc1 -> v1.27.0-rc.1).
AFFECTED, NOT = "affected", "not affected"
AGREEMENT = (
    ("golang.org/x/exp", XEXP_PSEUDO, "GO-2099-0001", AFFECTED, "query; PLANTED introduced 0, no real x/exp entry"),
    ("golang.org/x/net", "v0.61.0", "GO-2020-0014", NOT, "query"),
    ("golang.org/x/net", "v0.0.0-20190101000000-aaaaaaaaaaaa", "GO-2020-0014", AFFECTED, "query"),
    ("github.com/octo-sts/app", "v0.0.0-20240101000000-abcdefabcdef", "GO-2024-2833", AFFECTED, "query"),
    ("github.com/octo-sts/app", "v0.1.0", "GO-2024-2833", NOT, "query"),
    ("example.com/ooo", "v0.9.0", "GO-2099-0002", NOT, "query; PLANTED events out of order"),
    ("example.com/ooo", "v1.0.0", "GO-2099-0002", AFFECTED, "query; PLANTED events out of order"),
    ("example.com/ooo", "v1.1.0", "GO-2099-0002", AFFECTED, "query; PLANTED events out of order"),
    ("example.com/ooo", "v1.2.0", "GO-2099-0002", NOT, "query; PLANTED events out of order"),
    ("github.com/mutagen-io/mutagen", "v0.16.5", "GO-2023-1758", AFFECTED, "query; fixed - 1"),
    ("github.com/mutagen-io/mutagen", "v0.16.6-rc.1", "GO-2023-1758", AFFECTED, "query; fixed - 1 (pre-release)"),
    ("github.com/mutagen-io/mutagen", "v0.16.6", "GO-2023-1758", NOT, "query; fixed"),
    ("github.com/mutagen-io/mutagen", "v0.16.7", "GO-2023-1758", NOT, "query; between the ranges"),
    ("github.com/mutagen-io/mutagen", "v0.17.0-rc.1", "GO-2023-1758", NOT, "query; introduced - 1"),
    ("github.com/mutagen-io/mutagen", "v0.17.0", "GO-2023-1758", AFFECTED, "query; introduced"),
    ("github.com/mutagen-io/mutagen", "v0.17.1-rc.1", "GO-2023-1758", AFFECTED, "query; fixed - 1 (pre-release)"),
    ("github.com/mutagen-io/mutagen", "v0.17.1", "GO-2023-1758", NOT, "query; fixed"),
    ("github.com/mutagen-io/mutagen", "v0.18.0", "GO-2023-1758", NOT, "query"),
    ("github.com/docker/distribution", "v2.6.2+incompatible", "GO-2021-0072", AFFECTED, "query"),
    ("github.com/docker/distribution", "v2.7.0-beta.1+incompatible", "GO-2021-0072", AFFECTED, "query"),
    ("github.com/docker/distribution", "v2.7.0-rc.0+incompatible", "GO-2021-0072", NOT, "query"),
    ("github.com/docker/distribution", "v2.7.0-rc.0", "GO-2021-0072", NOT, "query"),
    ("github.com/docker/distribution", "v2.7.0+incompatible", "GO-2021-0072", NOT, "query"),
    ("stdlib", "go1.26.8", "GO-2026-6617", AFFECTED, "query"),
    ("stdlib", "go1.26.9", "GO-2026-6617", NOT, "query"),
    ("stdlib", "go1.27rc1", "GO-2026-6617", AFFECTED, "query; introduced 1.27.0-0"),
    ("stdlib", "go1.27.1", "GO-2026-6617", AFFECTED, "query"),
    ("stdlib", "go1.27.2", "GO-2026-6617", NOT, "query"),
    ("stdlib", "go1.28rc1", "GO-2026-6617", NOT, "query"),
    ("golang.org/x/net", "v0.59.0", "GO-2026-6617", AFFECTED, "query; the entry's second module"),
    ("golang.org/x/net", "v0.60.0", "GO-2026-6617", NOT, "query; the entry's second module"),
    ("golang.org/x/net", "v0.61.0", "GO-2026-6617", NOT, "query; the entry's second module"),
    ("golang.org/x/net", "v0.61.0", "GO-2026-6860", AFFECTED, "query"),
    ("golang.org/x/net", "v0.62.0", "GO-2026-6860", NOT, "query"),
    ("stdlib", "go1.27.3", "GO-2026-6859", AFFECTED, "query; source with go1.27.3 on PATH: GOVERSION go1.27.3"),
    ("stdlib", "go1.27.4", "GO-2026-6859", NOT, "query; source with go1.27.4 on PATH: GOVERSION go1.27.4"),
    # query mode's three differences: the matcher is right, query mode is not
    ("github.com/gorilla/sessions", "v1.2.2", "GO-2024-2730", NOT, "source; DIFFERS: query reports a withdrawn entry"),
    (
        "github.com/mutagen-io/mutagen",
        "v0.16.7",
        "GO-2023-1758",
        NOT,
        "query over GO-2023-1758 + PLANTED GO-2099-0003; DIFFERS: query returns the unfiltered entries, both",
    ),
    (
        "github.com/mutagen-io/mutagen",
        "v0.17.0",
        "GO-2023-1758",
        AFFECTED,
        "query with the modules.json item's fixed planted at 0.16.6; DIFFERS: query's pre-filter drops it",
    ),
)
REAL = (
    "GO-2020-0014",
    "GO-2021-0072",
    "GO-2023-1758",
    "GO-2024-2730",
    "GO-2024-2833",
    "GO-2026-6617",
    "GO-2026-6859",
    "GO-2026-6860",
)
PLANTED = {e["id"]: e for e in (PLANTED_XEXP, PLANTED_OOO, PLANTED_MUT)}


class Agreement(unittest.TestCase):
    """The matcher agrees with govulncheck v1.9.0 on every row (design §5; plan check §2 S1)."""

    def test_the_testdata_holds_exactly_the_real_entries_and_their_notice(self):
        self.assertEqual(sorted(os.listdir(TESTDATA)), sorted([*(f"{i}.json" for i in REAL), "NOTICE.md"]))
        for entry_id in REAL:
            self.assertEqual(real(entry_id)["id"], entry_id)
        self.assertEqual(len(AGREEMENT), 39)  # the row count the law model pins (design §1)

    def test_every_row_agrees(self):
        for module, version, entry_id, verdict, mode in AGREEMENT:
            e = PLANTED[entry_id] if entry_id in PLANTED else real(entry_id)
            with self.subTest(f"{module}@{version} {entry_id} ({mode})"):
                got = scan({}, version, [e]) if module == "stdlib" else scan({module: version}, "go1.27.4", [e])
                hit = any(f.id == entry_id and f.module == module for f in got)
                self.assertEqual(AFFECTED if hit else NOT, verdict)

    def test_a_withdrawn_real_entry_still_matches_by_range(self):
        # GO-2024-2730 is skipped only because it is withdrawn: its range holds every version.
        e = real("GO-2024-2730")
        self.assertTrue(e["withdrawn"])
        self.assertTrue(vulndb.affected("v1.2.2", e["affected"][0]["ranges"]))


def member_db(**over: object) -> dict[str, bytes]:
    """A small clean database: one x/net entry, its index lines, db.json modified two days before NOW."""
    e = entry("GO-2099-0100", ("golang.org/x/net", semver({"introduced": "0"}, {"fixed": "0.62.0"})))
    members: dict[str, object] = {
        "index/db.json": {"modified": "2026-11-11T02:51:11Z"},
        "index/modules.json": [
            {
                "path": "golang.org/x/net",
                "vulns": [{"id": "GO-2099-0100", "modified": "2026-11-01T00:00:00Z", "fixed": "0.62.0"}],
            }
        ],
        "index/vulns.json": [{"id": "GO-2099-0100", "modified": "2026-11-01T00:00:00Z", "aliases": []}],
        "ID/GO-2099-0100.json": e,
    }
    members.update(over)
    return {k: v if isinstance(v, bytes) else json.dumps(v).encode() for k, v in members.items() if v is not None}


def stamp(**over: object) -> dict:
    return {"fetched": "2026-11-12T08:00:00Z", "sha256": SHA, "url": vulndb.CANONICAL_URL, **over}


class CheckDb(unittest.TestCase):
    """design §2.4: every fault one line, all collected."""

    def check(
        self,
        members: dict[str, bytes],
        st: dict | None,
        *,
        now: datetime = NOW,
        sha: str = SHA,
        names: list[str] | None = None,
        too_large: frozenset[str] = frozenset(),
    ) -> list[str]:
        """check_db over `members` as the zip: `names` its member list (the members' own by default); `read` as the
        row's, raising MemberTooLarge for a name in `too_large`."""

        def read(name: str) -> bytes | None:
            if name in too_large:
                raise vulndb.MemberTooLarge(name)
            return members.get(name)

        listed = sorted(members) if names is None else names
        return vulndb.check_db(read, listed, st, root=ROOT, zip_sha256=sha, now=now)

    def assertFault(self, faults: list[str], text: str) -> None:
        self.assertTrue(any(text in f for f in faults), f"no fault holding {text!r} in {faults}")

    def test_a_clean_copy_has_no_fault(self):
        self.assertEqual(self.check(member_db(), stamp()), [])

    def test_no_stamp(self):
        self.assertIn(f"vulns: no stamp.json at {ROOT}: run make vulndb", self.check(member_db(), None))

    def test_a_malformed_stamp(self):
        for bad in (
            [],
            {"fetched": "2026-11-12T08:00:00Z", "sha256": SHA},
            {**stamp(), "extra": "x"},
            stamp(sha256=5),
            stamp(fetched="yesterday"),
            stamp(fetched="2026-11-12"),
        ):
            with self.subTest(repr(bad)):
                self.assertFault(self.check(member_db(), bad), "vulns: stamp.json is malformed: ")

    def test_the_zip_is_not_the_stamps(self):
        self.assertIn(
            "vulns: the zip's sha256 is not the stamp's: run make vulndb",
            self.check(member_db(), stamp(), sha="cd" * 32),
        )

    def test_the_url_is_not_canonical(self):
        faults = self.check(member_db(), stamp(url="https://example.com/vulndb.zip"))
        self.assertIn(f"vulns: the copy came from https://example.com/vulndb.zip, not {vulndb.CANONICAL_URL}", faults)

    def test_fetched_in_the_future(self):
        faults = self.check(member_db(), stamp(fetched="2026-11-13T12:00:01Z"))
        self.assertIn("vulns: the copy's fetched time 2026-11-13T12:00:01Z is in the future", faults)

    def test_the_copy_age_counts_utc_calendar_days_7_ok_8_red(self):
        db = member_db(**{"index/db.json": {"modified": "2026-11-01T00:00:00Z"}})
        self.assertEqual(self.check(db, stamp(fetched="2026-11-06T00:00:00Z")), [])
        self.assertIn(
            "vulns: the copy was fetched 8 days ago (2026-11-05), more than 7: run make vulndb",
            self.check(db, stamp(fetched="2026-11-05T23:59:59Z")),
        )

    def test_one_second_across_midnight_is_one_day(self):
        db = member_db(**{"index/db.json": {"modified": "2026-11-01T00:00:00Z"}})
        late = datetime(2026, 11, 13, 0, 0, 1, tzinfo=UTC)
        self.assertIn(  # 7 days and 2 seconds elapsed, 8 calendar days
            "vulns: the copy was fetched 8 days ago (2026-11-05), more than 7: run make vulndb",
            self.check(db, stamp(fetched="2026-11-05T23:59:59Z"), now=late),
        )
        early = datetime(2026, 11, 13, 23, 59, 59, tzinfo=UTC)
        self.assertEqual(self.check(db, stamp(fetched="2026-11-06T00:00:00Z"), now=early), [])  # 7 d 23:59:59, 7 days

    def test_db_json_missing_or_malformed(self):
        for bad in (None, b"not json", {"modified": 5}, {"modified": "2026-11-11"}, {"modified": "x", "y": 1}, []):
            with self.subTest(repr(bad)):
                self.assertFault(
                    self.check(member_db(**{"index/db.json": bad}), stamp()),
                    "vulns: index/db.json is missing or malformed",
                )

    def test_modified_after_fetched(self):
        faults = self.check(member_db(**{"index/db.json": {"modified": "2026-11-12T09:00:00Z"}}), stamp())
        self.assertFault(
            faults,
            "vulns: the database's modified 2026-11-12T09:00:00Z is after the copy's fetched 2026-11-12T08:00:00Z",
        )

    def test_modified_in_the_future(self):
        faults = self.check(
            member_db(**{"index/db.json": {"modified": "2026-11-13T12:00:01Z"}}), stamp(fetched="2026-11-13T11:00:00Z")
        )
        self.assertFault(faults, "in the future")

    def test_the_database_age_30_ok_31_red(self):
        self.assertEqual(self.check(member_db(**{"index/db.json": {"modified": "2026-10-14T00:00:00Z"}}), stamp()), [])
        self.assertIn(
            "vulns: the database has not changed since 2026-10-13 (31 days, more than 30): check vuln.go.dev, or add a"
            " dated vulndb exemption naming a known issue",
            self.check(member_db(**{"index/db.json": {"modified": "2026-10-13T23:59:59Z"}}), stamp()),
        )

    def test_modules_json_missing_malformed_or_empty(self):
        for bad in (None, b"{", [], {"path": "x"}, [{"path": "golang.org/x/net"}], [{"vulns": []}]):
            with self.subTest(repr(bad)):
                self.assertFault(
                    self.check(member_db(**{"index/modules.json": bad}), stamp()),
                    "vulns: index/modules.json is missing, malformed or empty",
                )

    def test_vulns_json_missing_malformed_or_empty(self):
        for bad in (None, b"[", [], [{"modified": "2026-11-01T00:00:00Z"}]):
            with self.subTest(repr(bad)):
                self.assertFault(
                    self.check(member_db(**{"index/vulns.json": bad}), stamp()),
                    "vulns: index/vulns.json is missing, malformed or empty",
                )

    def test_a_listed_id_without_its_entry(self):
        faults = self.check(member_db(**{"ID/GO-2099-0100.json": None}), stamp())
        self.assertIn("vulns: GO-2099-0100 is listed in index/vulns.json but ID/GO-2099-0100.json is missing", faults)

    def test_modules_json_and_the_entry_disagree(self):
        extra = [
            {
                "path": "golang.org/x/net",
                "vulns": [{"id": "GO-2099-0100", "modified": "2026-11-01T00:00:00Z", "fixed": "0.62.0"}],
            },
            {"path": "golang.org/x/sys", "vulns": [{"id": "GO-2099-0100", "modified": "2026-11-01T00:00:00Z"}]},
        ]
        self.assertIn(
            "vulns: index/modules.json and ID/GO-2099-0100.json disagree on golang.org/x/sys",
            self.check(member_db(**{"index/modules.json": extra}), stamp()),
        )
        e = entry(
            "GO-2099-0100",
            ("golang.org/x/net", semver({"introduced": "0"}, {"fixed": "0.62.0"})),
            ("golang.org/x/text", semver({"introduced": "0"})),
        )
        self.assertIn(
            "vulns: index/modules.json and ID/GO-2099-0100.json disagree on golang.org/x/text",
            self.check(member_db(**{"ID/GO-2099-0100.json": e}), stamp()),
        )

    def test_a_member_past_4_mib_is_red(self):
        # The row's read raises MemberTooLarge past MEMBER_CAP decompressed bytes (design §2.4, §2.5); check_db names
        # the member and goes on.
        for name in ("index/modules.json", "ID/GO-2099-0100.json"):
            with self.subTest(name):
                faults = self.check(member_db(), stamp(), too_large=frozenset({name}))
                self.assertIn(f"vulns: member {name} exceeds 4 MiB", faults)
        self.assertEqual(vulndb.MEMBER_CAP, 4 * 1024 * 1024)

    def test_the_layout_is_an_exact_list(self):
        # Reviews DA-S10, DA-L3: index/db.json, index/modules.json, index/vulns.json and ID/<GO-YYYY-NNNN…>.json only.
        for name in (
            "README",
            "index/",
            "ID/",
            "index/extra.json",
            "index/db.json.bak",
            "ID/GO-2099-0100.json.bak",
            "ID/notes.txt",
            "ID/GO-2099-100.json",  # three digits: the number is four or more
            "ID/GO-99-0100.json",
            "ID/go-2099-0100.json",
            "ID/GHSA-xxxx-yyyy-zzzz.json",
            "ID/sub/GO-2099-0101.json",
            "../GO-2099-0101.json",
            "/ID/GO-2099-0101.json",
            "id/GO-2099-0101.json",
        ):
            with self.subTest(name):
                db = member_db()
                faults = self.check(db, stamp(), names=[*sorted(db), name])
                self.assertIn(f"vulns: member {name} is not in the database's layout", faults)

    def test_a_long_id_is_in_the_layout(self):
        e = entry("GO-2099-12345", ("golang.org/x/net", semver({"introduced": "0"}, {"fixed": "0.62.0"})))
        db = member_db(
            **{
                "index/modules.json": [
                    {
                        "path": "golang.org/x/net",
                        "vulns": [
                            {"id": "GO-2099-0100", "modified": "2026-11-01T00:00:00Z", "fixed": "0.62.0"},
                            {"id": "GO-2099-12345", "modified": "2026-11-01T00:00:00Z", "fixed": "0.62.0"},
                        ],
                    }
                ],
                "index/vulns.json": [
                    {"id": "GO-2099-0100", "modified": "2026-11-01T00:00:00Z", "aliases": []},
                    {"id": "GO-2099-12345", "modified": "2026-11-01T00:00:00Z", "aliases": []},
                ],
                "ID/GO-2099-12345.json": e,
            }
        )
        self.assertEqual(self.check(db, stamp()), [])

    def test_every_fault_is_collected_never_the_first_only(self):
        faults = self.check(
            member_db(**{"index/db.json": {"modified": "2026-10-01T00:00:00Z"}, "ID/GO-2099-0100.json": None}),
            stamp(fetched="2026-11-01T00:00:00Z", url="https://example.com/x.zip"),
            sha="cd" * 32,
        )
        for text in (
            "the zip's sha256 is not the stamp's",
            "the copy came from https://example.com/x.zip",
            "the copy was fetched 12 days ago (2026-11-01)",
            "the database has not changed since 2026-10-01 (43 days, more than 30)",
            "GO-2099-0100 is listed in index/vulns.json but ID/GO-2099-0100.json is missing",
        ):
            self.assertFault(faults, text)

    def test_the_limits_are_the_decided_ones(self):
        self.assertEqual((vulndb.COPY_MAX_DAYS, vulndb.SOURCE_MAX_DAYS), (7, 30))  # decision 2
        self.assertEqual(vulndb.CANONICAL_URL, "https://vuln.go.dev/vulndb.zip")


if __name__ == "__main__":
    unittest.main()
