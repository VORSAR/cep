"""Advisory aging, pure (S1.1c-3 plan check §0.15-0.17, §2 S2, §6 [3] "Aging" (6)-(11) and unit forms of the
driver's (1)-(4); owner 2026-11-12, S1.1c-2's plan check, decisions 5 and 6 as annotated; S1.1c-3's plan check,
decisions 8 and 9 and confirm (c)).

`aging.aged(entries, order, marker, *, today, read) -> list[str]` judges gate/known-issues.json's entries (dicts with
id, first_seen, forward, decision, correctness, opened) against the printed order, the sprint marker and an injected
date; `read` is the tracked-Markdown reader `knownissues.check` takes. One line per fault, naming the entry's id:

(1) more than two closed rows of the order follow the row of `first_seen` (that row not counted), and `decision` is
    empty, unresolved, or resolves to a section that does not name the id -> a line holding "closed rows since";
(2) the entry is open and `forward`'s row is closed, or `forward` is a part the marker has passed (`Order.passed`)
    -> a line holding "forwarded to";
(3) `correctness` is true and `opened` is more than 30 days before `today` -> a line holding "correctness issue open".

An empty `forward` with a naming decision stands at any age (BA9); with none, the entry ages by (1). The exact texts
are the design's (P2); these cases pin the three substrings and the id. Every case's world is a fixture: the order,
the decisions text and the date are injected. RealTree alone reads the real files.

design question (a forward alone): rule (1) as the plan writes it does not look at `forward`, so an entry forwarded
to an open row with no decision still ages after three closed rows; AgeRule's forward case pins that literal
reading. If the design reads decision 6 as "a forward is the clock", it changes.
design question (an id at a sentence's end): see SentenceEnd.
"""

import json
import os
import subprocess
import sys
import unittest
from datetime import UTC, date, datetime, timedelta

from gate.runner import aging, anchors, knownissues, sprintid

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
DECISIONS = "docs/sprints/decisions.md"
TODAY = date(2026, 11, 12)
AGE = "closed rows since"
FORWARD = "forwarded to"
CORRECTNESS = "correctness issue open"
NAMING_HEADING = "Owner decisions, 2026-11-12 — the known issues"
SILENT_HEADING = "Owner decisions, 2026-11-09 — nothing known"
SENTENCE_HEADING = "Owner decisions, 2026-11-08 — a sentence's end"
MARKDOWN = f"""# Decisions

## {NAMING_HEADING}

OLD1, OLD2 and CORR1 stay open; S2.2b-2 waits for the Go builder (S6.1); PART1 goes to a part.

## {SILENT_HEADING}

Nothing here names a known issue.

## {SENTENCE_HEADING}

The second finding is known issue S1.1c-3-2.
"""
NAMING = f"{DECISIONS}#{anchors.slug(NAMING_HEADING)}"
SILENT = f"{DECISIONS}#{anchors.slug(SILENT_HEADING)}"
SENTENCE_END = f"{DECISIONS}#{anchors.slug(SENTENCE_HEADING)}"
READ = {DECISIONS: MARKDOWN}.get


def order(*rows: tuple[str, bool]) -> sprintid.Order:
    return sprintid.Order(tuple(sprintid.Row(id, closed) for id, closed in rows))


# Two closed rows follow S2.2b; then three.
TWO_AFTER = order(("S2.2b", True), ("S3.1", True), ("S3.2a", True), ("S3.3", False), ("S1.1c", False), ("S6.1", False))
THREE_AFTER = order(("S2.2b", True), ("S3.1", True), ("S3.2a", True), ("S3.3", True), ("S1.1c", False), ("S6.1", False))
# Today's shape: S3.3 closed, S1.1c open, its parts in flight.
TODAYS = order(
    ("S1.1a-1", True),
    ("S2.2b", True),
    ("S3.1", True),
    ("S3.3", True),
    ("S1.1c", False),
    ("S4.1", False),
    ("S6.1", False),
    ("S8", False),
)
# S1.1c closed, the marker at S4.1.
S1_1C_CLOSED = order(("S1.1a-1", True), ("S3.3", True), ("S1.1c", True), ("S4.1", False), ("S6.1", False))


def entry(
    id: str,
    first_seen: str,
    *,
    forward: str = "",
    decision: str = "",
    correctness: bool = False,
    opened: str = "2026-11-01",
) -> dict[str, object]:
    return {
        "id": id,
        "first_seen": first_seen,
        "forward": forward,
        "decision": decision,
        "correctness": correctness,
        "opened": opened,
    }


class Case(unittest.TestCase):
    def aged(self, entries, order=TODAYS, marker="S1.1c-3", today=TODAY) -> list[str]:
        return aging.aged(list(entries), order, marker, today=today, read=READ)

    def assertOneLine(self, lines: list[str], id: str, needle: str) -> str:
        """Exactly one line, naming the id and holding the rule's words."""
        self.assertEqual(len(lines), 1, lines)
        self.assertIn(id, lines[0])
        self.assertIn(needle, lines[0])
        return lines[0]


class AgeRule(Case):
    """Rule (1): §6 aging (6), (8); the driver's (1) and (2) in unit form."""

    def test_two_closed_rows_after_first_seen_are_green(self):
        # S2.2b is itself closed: counting it would make three.
        self.assertEqual(self.aged([entry("OLD9", "S2.2b")], TWO_AFTER, "S3.3"), [])

    def test_three_closed_rows_after_first_seen_are_red(self):
        self.assertOneLine(self.aged([entry("OLD9", "S2.2b")], THREE_AFTER, "S1.1c-3"), "OLD9", AGE)

    def test_a_part_ages_from_its_row(self):
        # S2.2b-1 is a part of S2.2b: the same two closed rows follow it, then three.
        self.assertEqual(self.aged([entry("OLD9", "S2.2b-1")], TWO_AFTER, "S3.3"), [])
        self.assertOneLine(self.aged([entry("OLD9", "S2.2b-1")], THREE_AFTER, "S1.1c-3"), "OLD9", AGE)

    def test_first_seen_in_s1_1a_1_with_no_decision_is_red(self):
        self.assertOneLine(self.aged([entry("OLD9", "S1.1a-1")]), "OLD9", AGE)

    def test_an_unresolved_decision_ages(self):
        lines = self.aged([entry("OLD1", "S2.2b", decision=f"{DECISIONS}#no-such-heading")], THREE_AFTER)
        self.assertOneLine(lines, "OLD1", AGE)

    def test_a_decision_in_an_untracked_file_ages(self):
        ref = f"docs/elsewhere.md#{anchors.slug(NAMING_HEADING)}"
        lines = self.aged([entry("OLD1", "S2.2b", decision=ref)], THREE_AFTER)
        self.assertOneLine(lines, "OLD1", AGE)

    def test_a_decision_whose_section_does_not_name_the_id_ages(self):
        self.assertOneLine(self.aged([entry("OLD1", "S2.2b", decision=SILENT)], THREE_AFTER), "OLD1", AGE)

    def test_a_decision_naming_a_longer_id_does_not_name_the_id(self):
        # OLD1's section is NAMING; OLD names a prefix of OLD1 and OLD2 only.
        self.assertOneLine(self.aged([entry("OLD", "S2.2b", decision=NAMING)], THREE_AFTER), "OLD", AGE)

    def test_a_naming_decision_stops_the_age_clock(self):
        self.assertEqual(self.aged([entry("OLD1", "S1.1a-1", decision=NAMING)]), [])

    def test_a_young_entry_with_a_bad_decision_is_not_agings_red(self):
        # (1) is a conjunction: the bad decision is the known-issues pin's red (T2), not aging's, until it is old.
        self.assertEqual(self.aged([entry("OLD1", "S2.2b", decision=SILENT)], TWO_AFTER, "S3.3"), [])

    def test_an_empty_forward_with_a_naming_decision_stands_at_any_age(self):
        self.assertEqual(self.aged([entry("OLD2", "S1.1a-1", forward="", decision=NAMING)]), [])

    def test_an_empty_forward_without_a_decision_ages_by_rule_one(self):
        self.assertEqual(self.aged([entry("NEW1", "S3.3-2")]), [])  # nothing closed after S3.3 yet: young
        self.assertOneLine(self.aged([entry("OLD9", "S1.1a-1", forward="")]), "OLD9", AGE)

    def test_a_forward_to_an_open_row_does_not_stop_the_age_clock(self):
        """The plan's rule (1) as written (the module docstring's design question): no decision, three closed rows
        since, so red although the forward's row S6.1 is open."""
        self.assertOneLine(self.aged([entry("OLD9", "S2.2b", forward="S6.1")], THREE_AFTER), "OLD9", AGE)


class SentenceEnd(Case):
    """An id that ends a sentence: decisions.md writes "known issues S1.1c-3-1 and S1.1c-3-2." style text.

    Expected: named. A sprint id never ends in "." (sprintid.GRAMMAR: a "." is always followed by digits) and a review
    id (QB10, BA10) holds no "." at all, so a "." followed by a space, a line end or the text's end cannot continue an
    id: it is the sentence's full stop. Today `knownissues.names` counts "." as an id character on either side
    (ID_CHAR), so "S1.1c-3-2." does not name S1.1c-3-2, and aging (which reuses it) would call a decision that names
    the id in plain prose silent. Red today on the stub and, for `names`, on behaviour.

    design question: test_known_issues_pin.py's fixture relies on today's reading - its 2026-11-08 section reads
    "QB10X is not QB10." and its case `test_a_decision_section_that_does_not_name_the_id_is_red` expects that section
    NOT to name QB10. Under the reading pinned here that fixture names QB10, so the lane changes the fixture's text
    (for example "QB10X is not the id") in the same commit as `names`.
    """

    def test_a_sentence_final_id_is_named_by_names(self):
        self.assertIs(knownissues.names("The second finding is known issue S1.1c-3-2.", "S1.1c-3-2"), True)
        self.assertIs(knownissues.names("Known issue S1.1c-3-2.\nNext line.", "S1.1c-3-2"), True)
        self.assertIs(knownissues.names("QB10 is fixed.", "QB10"), True)  # mid-text: green today, the control

    def test_a_dot_followed_by_an_id_character_still_continues_the_id(self):
        self.assertIs(knownissues.names("S1.1c-3-2.1 is not it", "S1.1c-3-2"), False)
        self.assertIs(knownissues.names("S3.3", "S3"), False)

    def test_a_decision_naming_the_id_at_a_sentences_end_stops_the_age_clock(self):
        lines = self.aged([entry("S1.1c-3-2", "S2.2b", decision=SENTENCE_END)], THREE_AFTER)
        self.assertEqual(lines, [])


class ForwardRule(Case):
    """Rule (2): §6 aging (7); the driver's (3) in unit form; decision 6 as annotated by decision 8."""

    def test_a_forward_whose_row_has_closed_is_red(self):
        self.assertOneLine(self.aged([entry("FWD1", "S3.3-2", forward="S3.3")]), "FWD1", FORWARD)

    def test_a_naming_decision_does_not_save_an_expired_forward(self):
        lines = self.aged([entry("OLD1", "S3.3-2", forward="S3.3", decision=NAMING)])
        self.assertOneLine(lines, "OLD1", FORWARD)

    def test_a_forward_to_an_open_row_with_a_naming_decision_is_green(self):
        # S2.2b-2 today: first seen long ago, forwarded to the Go builder's row, decided.
        self.assertEqual(self.aged([entry("S2.2b-2", "S2.2b", forward="S6.1", decision=NAMING)]), [])

    def test_a_part_forward_expires_when_the_marker_passes_the_part(self):
        lines = self.aged([entry("PART1", "S3.3-2", forward="S1.1c-3", decision=NAMING)], marker="S1.1c-4")
        self.assertOneLine(lines, "PART1", FORWARD)

    def test_a_part_forward_stands_while_the_marker_is_at_the_part(self):
        self.assertEqual(self.aged([entry("PART1", "S3.3-2", forward="S1.1c-3", decision=NAMING)]), [])

    def test_a_part_forward_stands_while_the_marker_is_before_the_part(self):
        lines = self.aged([entry("PART1", "S3.3-2", forward="S1.1c-4", decision=NAMING)], marker="S1.1c-3")
        self.assertEqual(lines, [])

    def test_parts_are_compared_as_numbers(self):
        later = entry("PART1", "S3.3-2", forward="S1.1c-10", decision=NAMING)
        self.assertEqual(self.aged([later], marker="S1.1c-3"), [])  # a string sort would call S1.1c-10 passed
        earlier = entry("PART1", "S3.3-2", forward="S1.1c-3", decision=NAMING)
        self.assertOneLine(self.aged([earlier], marker="S1.1c-10"), "PART1", FORWARD)

    def test_a_part_forward_expires_when_its_row_closes(self):
        lines = self.aged([entry("PART1", "S3.3-2", forward="S1.1c-3", decision=NAMING)], S1_1C_CLOSED, "S4.1")
        self.assertOneLine(lines, "PART1", FORWARD)

    def test_a_part_above_the_plan_is_named(self):
        """§6 aging (7): `forward: S1.1c-99` is red (above the plan), or it would wait for S1.1c's close unseen.

        design question: gate/sprints.json names rows only, and the row's expected_scope does not read
        sprint-plan.md, so the design names where "the plan's highest part" of a row comes from. The text is the
        design's; this case pins one line naming the id and the part."""
        lines = self.aged([entry("PART1", "S3.3-2", forward="S1.1c-99", decision=NAMING)])
        self.assertEqual(len(lines), 1, lines)
        self.assertIn("PART1", lines[0])
        self.assertIn("S1.1c-99", lines[0])

    def test_a_part_less_marker_where_a_part_is_needed_is_named(self):
        """§2 S2: the marker S1.1c cannot say whether S1.1c-3 has passed: a named red, never a silent pass (the
        conservative `Order.passed` answer, False, would leave the forward standing; test_sprintid_parts.py)."""
        lines = self.aged([entry("PART1", "S3.3-2", forward="S1.1c-3", decision=NAMING)], marker="S1.1c")
        self.assertEqual(len(lines), 1, lines)
        self.assertIn("PART1", lines[0])

    def test_a_part_less_marker_in_a_later_row_has_passed_the_part(self):
        lines = self.aged([entry("PART1", "S3.3-2", forward="S1.1c-3", decision=NAMING)], marker="S4.1")
        self.assertOneLine(lines, "PART1", FORWARD)


class CorrectnessRule(Case):
    """Rule (3): §6 aging (9); the driver's (4) in unit form; ground rule 10; decision 9."""

    def corr(self, days: int, *, correctness: bool = True, today: date = TODAY, **kw) -> list[str]:
        opened = (today - timedelta(days=days)).isoformat()
        # first seen at the marker's own row and forwarded nowhere: rules (1) and (2) cannot fire.
        return self.aged([entry("CORR1", "S1.1c-3", correctness=correctness, opened=opened, **kw)], today=today)

    def test_thirty_one_days_is_red(self):
        self.assertOneLine(self.corr(31), "CORR1", CORRECTNESS)

    def test_thirty_days_is_green(self):
        self.assertEqual(self.corr(30), [])

    def test_opened_today_is_green(self):
        self.assertEqual(self.corr(0), [])

    def test_not_a_correctness_issue_at_three_hundred_days_is_green(self):
        self.assertEqual(self.corr(300, correctness=False), [])

    def test_a_naming_decision_and_an_open_forward_do_not_stop_the_thirty_days(self):
        self.assertOneLine(self.corr(31, decision=NAMING, forward="S6.1"), "CORR1", CORRECTNESS)

    def test_the_days_are_calendar_days_across_a_year(self):
        self.assertOneLine(self.corr(31, today=date(2027, 1, 15)), "CORR1", CORRECTNESS)  # opened 2026-12-15
        self.assertEqual(self.corr(30, today=date(2027, 1, 15)), [])

    def test_the_date_is_injected_never_read(self):
        # The same entry, judged on two injected days: only `today` moves the verdict.
        e = entry("CORR1", "S1.1c-3", correctness=True, opened="2026-11-12")
        self.assertEqual(self.aged([e], today=date(2026, 12, 12)), [])
        self.assertOneLine(self.aged([e], today=date(2026, 12, 13)), "CORR1", CORRECTNESS)


class TheDate(unittest.TestCase):
    """§6 aging (9): "23:30 at UTC-5 -> judged by UTC". `aged` takes a date; the one `today()` the row reads
    (`gate/runner/rows/__init__.py`, §2 S2) gives the UTC date whatever the local zone. Red today: there is no
    `rows.today`.

    The child runs under two POSIX zones, UTC+14 and UTC-12: at every instant at least one of them is on a different
    calendar day from UTC (UTC+14 from 10:00 UTC, UTC-12 before 12:00 UTC), which the test asserts as its witness, so
    a `today()` reading the local date cannot pass both. The UTC date is read before and after in the child, so a run
    across midnight UTC changes nothing.
    """

    CHILD = (
        "import sys; sys.path.insert(0, sys.argv[1]); from datetime import UTC, date, datetime; "
        "before = datetime.now(UTC).date(); from gate.runner import rows; got = rows.today(); "
        "after = datetime.now(UTC).date(); print(got.isoformat(), before.isoformat(), after.isoformat(), "
        "date.today().isoformat(), type(got).__name__)"
    )

    def child(self, tz: str) -> list[str]:
        env = {k: v for k, v in os.environ.items() if not k.startswith("PYTHON")}
        env["TZ"] = tz
        argv = [sys.executable, "-I", "-B", "-c", self.CHILD, REPO]
        done = subprocess.run(argv, env=env, capture_output=True, text=True, timeout=60, check=False)
        self.assertEqual(done.returncode, 0, done.stderr)
        return done.stdout.split()

    def test_today_is_the_utc_date_in_any_zone(self):
        differs = []
        for tz in ("AAA-14", "BBB+12"):  # POSIX: UTC+14, UTC-12
            got, before, after, local, kind = self.child(tz)
            self.assertEqual(kind, "date", tz)
            self.assertIn(got, {before, after}, tz)
            differs.append(local not in {before, after})
        self.assertTrue(any(differs), "the witness: one zone's local date differs from UTC's")


class Faults(Case):
    """§6 aging (10), (11): what aging cannot judge is named, never a crash or a silent pass."""

    def test_an_empty_list_is_green(self):
        self.assertEqual(self.aged([]), [])

    def test_only_the_red_entry_is_named(self):
        lines = self.aged([entry("OLD1", "S1.1a-1", decision=NAMING), entry("OLD9", "S1.1a-1")])
        self.assertOneLine(lines, "OLD9", AGE)
        self.assertFalse(any(knownissues.names(line, "OLD1") for line in lines), lines)

    def test_each_red_entry_has_its_own_line(self):
        lines = self.aged([entry("OLD8", "S1.1a-1"), entry("OLD9", "S1.1a-1")])
        self.assertEqual(len(lines), 2, lines)
        self.assertEqual({knownissues.names(line, "OLD8") for line in lines}, {True, False}, lines)
        self.assertEqual({knownissues.names(line, "OLD9") for line in lines}, {True, False}, lines)

    def test_a_first_seen_not_in_the_order_is_named(self):
        lines = self.aged([entry("BAD1", "S99")])
        self.assertEqual(len(lines), 1, lines)
        self.assertIn("BAD1", lines[0])
        self.assertIn("S99", lines[0])

    def test_a_forward_not_in_the_order_is_named(self):
        lines = self.aged([entry("BAD1", "S3.3-2", forward="S99", decision=NAMING)])
        self.assertEqual(len(lines), 1, lines)
        self.assertIn("BAD1", lines[0])
        self.assertIn("S99", lines[0])


class RealTree(unittest.TestCase):
    """§0.16: the row lands green. The real gate/known-issues.json, gate/sprints.json, gate/current-sprint and the
    Markdown git tracks, judged today (UTC). Red today twice over: the entries carry no `correctness` or `opened`
    yet (decision 9's fields, lane E2's knownissues change), and `aged` is the stub."""

    def read_repo(self, rel: str) -> str:
        with open(os.path.join(REPO, rel), encoding="utf-8") as f:
            return f.read()

    def test_the_real_tree_ages_green(self):
        done = subprocess.run(
            ["git", "ls-files", "-z", "--", "*.md"], cwd=REPO, capture_output=True, timeout=60, check=False
        )
        self.assertEqual(done.returncode, 0, done.stderr)
        tracked = {p for p in done.stdout.decode("utf-8", errors="replace").split("\0") if p}
        self.assertIn(DECISIONS, tracked)  # the witness: the reader sees the decisions the entries cite

        def read(path: str) -> str | None:
            return self.read_repo(path) if path in tracked else None

        entries = json.loads(self.read_repo(knownissues.JSON))["entries"]
        self.assertTrue(entries, "the case needs entries to judge")
        for e in entries:
            with self.subTest(id=e["id"]):
                self.assertIs(type(e.get("correctness")), bool, e)
                self.assertRegex(str(e.get("opened")), r"^\d{4}-\d{2}-\d{2}$", e)
        order = sprintid.load_order(self.read_repo(sprintid.SPRINTS))
        marker = sprintid.canonical(sprintid.read_marker(self.read_repo(sprintid.MARKER)))
        today = datetime.now(UTC).date()
        self.assertEqual(aging.aged(entries, order, marker, today=today, read=read), [])


if __name__ == "__main__":
    unittest.main()
