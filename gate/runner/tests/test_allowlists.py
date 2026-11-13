"""gate/runner/allowlists.py (S1.1c-2 design §2.2, §2.8): the list loader and the one set of field validators.

Every text of §2.8's `load` and `fields` rows is asserted verbatim. The context is built from fixtures here — a small
sprint order, a fixed day, a tier, a set of issue ids and a fake `resolve` — and never from gate/current-sprint,
gate/sprints.json, gate/known-issues.json or the clock (design §2.2, DQ-S9).
"""

import json
import unittest
from datetime import date, timedelta

from gate.runner import allowlists, sprintid

TODAY = date(2026, 11, 12)
ORDER = sprintid.Order(
    (
        sprintid.Row("S1.1a-1", True),
        sprintid.Row("S3.1", True),
        sprintid.Row("S3.3", True),
        sprintid.Row("S1.1c", False),
        sprintid.Row("S4.1", False),
    )
)
CURRENT = sprintid.parse("S1.1c-2")
ISSUES = frozenset({"S3.3-2-1", "BA9"})
GOOD_REF = "docs/sprints/decisions.md#title"
BAD_REF = "docs/sprints/decisions.md#nowhere"
BAD_WHY = "docs/sprints/decisions.md has no heading #nowhere"
REASON = "a reason long enough"  # 20 characters


def fake_resolve(ref: str) -> str:
    return "" if ref == GOOD_REF else BAD_WHY


def ctx(tier: str = "T2", today: date = TODAY) -> allowlists.Ctx:
    return allowlists.Ctx(today=today, tier=tier, order=ORDER, current=CURRENT, issues=ISSUES, resolve=fake_resolve)


def day(k: int) -> str:
    return (TODAY + timedelta(days=k)).isoformat()


def permanent(**over: str) -> dict[str, str]:
    entry = {"kind": "PERMANENT", "reason": REASON, "ref": GOOD_REF, "expiry": "", "issue": "", "sprint": "S1.1c-2"}
    entry.update(over)
    return entry


def temp(**over: str) -> dict[str, str]:
    entry = {"kind": "TEMP", "reason": REASON, "ref": "", "expiry": day(30), "issue": "S3.3-2-1", "sprint": "S1.1c-2"}
    entry.update(over)
    return entry


def fields(entry: dict[str, str], c: allowlists.Ctx | None = None) -> list[str]:
    return allowlists.fields(
        entry["kind"], entry["reason"], entry["ref"], entry["expiry"], entry["issue"], entry["sprint"], c or ctx()
    )


def list_text(**over: object) -> str:
    doc: dict[str, object] = {
        "schema": 1,
        "note": "a note",
        "rule": "purity",
        "entries": [
            {
                "target": "internal/table: sync.Mutex",
                "kind": "PERMANENT",
                "reason": REASON,
                "ref": GOOD_REF,
                "sprint": "S3.1-2",
            }
        ],
    }
    doc.update(over)
    return json.dumps(doc)


PREFIX = "tools/allowlists/purity.json: "


class Load(unittest.TestCase):
    def assertRefused(self, text: str, want: str, name: str = "purity") -> None:
        with self.assertRaises(ValueError) as caught:
            allowlists.load(text, name)
        self.assertEqual(str(caught.exception), want)

    def test_rules(self):
        # "vulns": S1.1c-4 (owner, 2026-11-13, S1.1c-4's design, after the reviews).
        self.assertEqual(
            allowlists.RULES,
            frozenset({"purity", "clock", "teeth", "ruff", "testhygiene", "generated", "fragments", "vulns"}),
        )

    def test_a_good_list_loads(self):
        got = allowlists.load(list_text(), "purity")
        want = allowlists.AllowList(
            "purity",
            "purity",
            (allowlists.Entry("internal/table: sync.Mutex", "PERMANENT", REASON, "S3.1-2", ref=GOOD_REF),),
        )
        self.assertEqual(got, want)

    def test_a_temp_entry_loads_with_its_optional_keys(self):
        entry = {
            "target": "x.go: time",
            "kind": "TEMP",
            "reason": REASON,
            "expiry": "2026-12-01",
            "issue": "BA9",
            "sprint": "S1.1c-2",
        }
        got = allowlists.load(list_text(rule="clock", entries=[entry]), "clock")
        self.assertEqual(
            got.entries,
            (allowlists.Entry("x.go: time", "TEMP", REASON, "S1.1c-2", expiry="2026-12-01", issue="BA9"),),
        )

    def test_not_json(self):
        with self.assertRaises(ValueError) as caught:
            allowlists.load("{not json", "purity")
        self.assertTrue(str(caught.exception).startswith(PREFIX + "not JSON: "), str(caught.exception))
        self.assertGreater(len(str(caught.exception)), len(PREFIX + "not JSON: "))

    def test_duplicate_key(self):
        text = '{"schema": 1, "note": "n", "rule": "purity", "rule": "purity", "entries": []}'
        self.assertRefused(text, PREFIX + "duplicate key rule")

    def test_keys(self):
        # design ambiguity: §2.8 writes `keys <got>, want schema, note, rule, entries` without saying how <got> is
        # written; the test pins the frame and that every key present is named, not its separator.
        for doc in ({"schema": 1, "note": "n", "rule": "purity"}, json.loads(list_text()) | {"extra": 1}):
            with self.subTest(keys=sorted(doc)):
                with self.assertRaises(ValueError) as caught:
                    allowlists.load(json.dumps(doc), "purity")
                text = str(caught.exception)
                self.assertTrue(text.startswith(PREFIX + "keys "), text)
                self.assertTrue(text.endswith(", want schema, note, rule, entries"), text)
                got = text.removeprefix(PREFIX + "keys ").removesuffix(", want schema, note, rule, entries")
                for key in doc:
                    self.assertIn(key, got)

    def test_schema(self):
        self.assertRefused(list_text(schema=2), PREFIX + "schema 2, want 1")

    def test_note(self):
        self.assertRefused(list_text(note=5), PREFIX + "note is not a string")

    def test_rule_unknown(self):
        # design ambiguity: <sorted RULES> is read as the sorted names joined by ", " (law.py's KINDS text).
        want = PREFIX + "rule nope is not one of " + ", ".join(sorted(allowlists.RULES))
        self.assertRefused(list_text(rule="nope"), want)

    def test_rule_against_the_file_name(self):
        self.assertRefused(list_text(rule="clock"), PREFIX + "rule clock but the file is purity.json")

    def test_entries_not_a_list(self):
        self.assertRefused(list_text(entries={}), PREFIX + "entries is not a list")

    def test_an_unknown_entry_key(self):
        entry = json.loads(list_text())["entries"][0] | {"owner": "me"}
        self.assertRefused(list_text(entries=[entry]), PREFIX + "entries[0]: unknown key owner")

    def test_a_missing_entry_key(self):
        for key in ("target", "kind", "reason", "sprint"):
            with self.subTest(key=key):
                entry = json.loads(list_text())["entries"][0]
                del entry[key]
                self.assertRefused(list_text(entries=[entry]), PREFIX + f"entries[0]: missing {key}")

    def test_an_entry_value_not_a_string(self):
        entry = json.loads(list_text())["entries"][0] | {"reason": 5}
        self.assertRefused(list_text(entries=[entry]), PREFIX + "entries[0]: reason is not a string")

    def test_a_target_listed_twice(self):
        entry = json.loads(list_text())["entries"][0]
        want = PREFIX + "entries[1]: target internal/table: sync.Mutex is listed twice"
        self.assertRefused(list_text(entries=[entry, dict(entry)]), want)


class Fields(unittest.TestCase):
    def test_a_good_permanent_and_a_good_temp_are_green(self):
        self.assertEqual(fields(permanent()), [])
        self.assertEqual(fields(temp()), [])

    def test_kind(self):
        self.assertIn("kind must be PERMANENT or TEMP", fields(permanent(kind="SOMETIMES")))

    def test_reason_twelve_characters_green_eleven_red(self):
        self.assertEqual(fields(permanent(reason="x" * 12)), [])
        self.assertEqual(fields(permanent(reason="  " + "x" * 12 + "  ")), [])
        self.assertEqual(fields(permanent(reason="x" * 11)), ["reason must be at least 12 characters"])
        self.assertEqual(fields(permanent(reason="  " + "x" * 11 + "   ")), ["reason must be at least 12 characters"])

    def test_permanent_needs_a_ref(self):
        self.assertEqual(fields(permanent(ref="")), ["PERMANENT needs ref=<path>#<anchor>"])

    def test_permanent_takes_no_expiry_or_issue(self):
        self.assertIn("PERMANENT takes no expiry or issue", fields(permanent(expiry=day(10))))
        self.assertIn("PERMANENT takes no expiry or issue", fields(permanent(issue="BA9")))

    def test_permanent_ref_must_resolve(self):
        # design ambiguity: a ref that does not resolve is reported with resolve's why verbatim (§2.8 resolves row).
        self.assertEqual(fields(permanent(ref=BAD_REF)), [BAD_WHY])

    def test_temp_needs_expiry_and_issue(self):
        want = "TEMP needs expiry=YYYY-MM-DD and issue=<known issue id>"
        self.assertEqual(fields(temp(expiry="")), [want])
        self.assertEqual(fields(temp(issue="")), [want])

    def test_temp_takes_no_ref(self):
        self.assertEqual(fields(temp(ref=GOOD_REF)), ["TEMP takes no ref"])

    def test_expiry_form(self):
        for bad in ("2026-13-01", "26-12-01", "2026-12-1", "tomorrow"):
            with self.subTest(expiry=bad):
                self.assertEqual(fields(temp(expiry=bad)), ["expiry must be YYYY-MM-DD"])

    def test_exactly_ninety_days_green_ninety_one_red(self):
        self.assertEqual(fields(temp(expiry=day(90))), [])
        self.assertEqual(fields(temp(expiry=day(91))), [f"expiry {day(91)} is more than 90 days ahead"])
        self.assertEqual(day(91), "2027-02-11")

    def test_today_is_not_expired_at_any_tier(self):
        for tier in ("T2", "T3", "T4", "nightly"):
            with self.subTest(tier=tier):
                self.assertEqual(fields(temp(expiry=day(0)), ctx(tier)), [])

    def test_expired_green_at_t3_red_at_t4_and_nightly(self):
        for tier in ("T1", "T2", "T3"):
            with self.subTest(tier=tier):
                self.assertEqual(fields(temp(expiry=day(-1)), ctx(tier)), [])
        for tier in ("T4", "nightly"):
            with self.subTest(tier=tier):
                self.assertEqual(fields(temp(expiry=day(-1)), ctx(tier)), [f"expired {day(-1)}"])

    def test_issue_must_be_known(self):
        self.assertEqual(fields(temp(issue="QB99")), ["issue QB99 is not in gate/known-issues.json"])

    def test_sprint_not_a_row(self):
        self.assertEqual(fields(permanent(sprint="S2.9")), ["sprint S2.9 is not a row of gate/sprints.json"])

    def test_sprint_after_the_current_one(self):
        self.assertEqual(fields(permanent(sprint="S4.1")), ["sprint S4.1 is after the current sprint S1.1c-2"])
        self.assertEqual(fields(permanent(sprint="S1.1c-9")), ["sprint S1.1c-9 is after the current sprint S1.1c-2"])

    def test_sprint_before_or_at_the_current_one_is_green(self):
        for sprint in ("S3.1-2", "S1.1a-1", "S3.3", "S1.1c-1", "S1.1c-2"):
            with self.subTest(sprint=sprint):
                self.assertEqual(fields(permanent(sprint=sprint)), [])


class Faults(unittest.TestCase):
    def test_sprint_fault(self):
        self.assertEqual(allowlists.sprint_fault("S3.1-2", ctx()), "")
        self.assertEqual(
            allowlists.sprint_fault("S1.1c-9", ctx()), "sprint S1.1c-9 is after the current sprint S1.1c-2"
        )
        self.assertEqual(allowlists.sprint_fault("S2.9", ctx()), "sprint S2.9 is not a row of gate/sprints.json")

    def test_a_sprint_that_does_not_parse_is_not_a_row(self):
        # design ambiguity: §2.8 has no text for an unparseable sprint; "not a row" is the nearer of its two texts.
        self.assertEqual(allowlists.sprint_fault("next", ctx()), "sprint next is not a row of gate/sprints.json")

    def test_ref_fault_is_resolves_why(self):
        self.assertEqual(allowlists.ref_fault(GOOD_REF, ctx()), "")
        self.assertEqual(allowlists.ref_fault(BAD_REF, ctx()), BAD_WHY)


class RefIsARecordedDecision(unittest.TestCase):
    """CA-S3 (owner, 2026-11-12, S1.1c-2's review and break-it 2): "A PERMANENT exemption's `ref` resolves only into
    `docs/architecture/adr/` or `docs/sprints/decisions.md`"; ref_fault is the one place both rows use."""

    WHY = "ref {} is not a recorded decision: cite docs/architecture/adr/… or docs/sprints/decisions.md#…"

    def everywhere(self) -> allowlists.Ctx:
        """A resolver that finds every heading: only the path rule can say no."""
        return allowlists.Ctx(today=TODAY, tier="T2", order=ORDER, current=CURRENT, issues=ISSUES, resolve=lambda r: "")

    def test_a_ref_into_any_other_document_is_red(self):
        for path in (
            "docs/development/testing.md",
            "docs/sprints/s1-1a-gate-design.md",
            "docs/architecture/ARCHITECTURE.md",
            "docs/architecture/adr.md",
            "docs/sprints/decisions.md.md",
            "decisions.md",
            "docs/a.md",
        ):
            with self.subTest(path=path):
                self.assertEqual(allowlists.ref_fault(f"{path}#title", self.everywhere()), self.WHY.format(path))

    def test_an_adr_or_decisions_ref_is_resolved(self):
        for ref in ("docs/architecture/adr/ADR-0015-x.md#title", "docs/sprints/decisions.md#title"):
            with self.subTest(ref=ref):
                self.assertEqual(allowlists.ref_fault(ref, self.everywhere()), "")
        self.assertEqual(allowlists.ref_fault(BAD_REF, ctx()), BAD_WHY)  # and the resolver still says no

    def test_a_permanent_entry_citing_another_document_is_red(self):
        self.assertEqual(fields(permanent(ref="docs/a.md#title")), [self.WHY.format("docs/a.md")])


class Check(unittest.TestCase):
    def test_each_entrys_faults_are_prefixed_with_the_file_index_and_target(self):
        lst = allowlists.AllowList(
            "purity",
            "purity",
            (
                allowlists.Entry("internal/a: sync.Pool", "PERMANENT", REASON, "S3.1-2", ref=GOOD_REF),
                allowlists.Entry("internal/b: sync.Once", "PERMANENT", "too short", "S3.1-2", ref=GOOD_REF),
                allowlists.Entry("internal/c: sync.Map", "PERMANENT", REASON, "S4.1", ref=GOOD_REF),
            ),
        )
        prefix = "tools/allowlists/purity.json: entries"
        self.assertEqual(
            allowlists.check(lst, ctx()),
            [
                f"{prefix}[1] (internal/b: sync.Once): reason must be at least 12 characters",
                f"{prefix}[2] (internal/c: sync.Map): sprint S4.1 is after the current sprint S1.1c-2",
            ],
        )

    def test_a_good_list_is_green(self):
        self.assertEqual(allowlists.check(allowlists.load(list_text(), "purity"), ctx()), [])


class Expiring(unittest.TestCase):
    def test_the_window(self):
        items = [("in14", day(14)), ("in15", day(15)), ("today", day(0)), ("yesterday", day(-1)), ("far", day(80))]
        got = allowlists.expiring(items, TODAY)
        self.assertEqual(sorted(got), sorted([("in14", 14), ("today", 0), ("yesterday", -1)]))

    def test_nothing_in_the_window(self):
        self.assertEqual(allowlists.expiring([("x", day(15))], TODAY), [])
        self.assertEqual(allowlists.expiring([], TODAY), [])


if __name__ == "__main__":
    unittest.main()
