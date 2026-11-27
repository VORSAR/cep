"""gate/runner/parts.py (S1.1c-5 design §3.5, §8 3.2-3.4; owner 2026-11-14, S1.1c-5's design after the reviews,
decision 1; DQ-B1, DQ-B3): the part-level check on tools/allowlists/smells.json's size exclusions, run by the aging row.

`forwards` is pure: the order, the marker, the tier, the merge-base state and the decision reader are injected, so no
case reads the real tree. ReadBase alone builds fixture git repositories (`git.merge_base`). Every expected text is
§3.5's, written by hand; a fault and a line both carry the aging row's "aging: " prefix, as aging.aged's lines do
(the row prints a fault "failed: aging: <text>").
"""

import json
import os
import unittest
from unittest import mock

from gate.runner import aging, allowlists, anchors, parts, sprintid
from gate.runner.tests import support

IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@example.com")
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
ORDER = sprintid.Order(
    tuple(
        sprintid.Row(id, closed)
        for id, closed in (
            ("S3.3", True),
            ("S1.1c", False),
            ("S1.1c-5", False),
            ("S1.1c-6", False),
            ("S1.1c-7", False),
            ("S1.1c-8", False),
            ("S1.1c-9", False),
            ("S1.1c-10", False),
            ("S4.1", False),
        )
    )
)
TARGET = "internal/runtime gocyclo"
D0 = "docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-design-after-the-reviews"
D1 = "docs/sprints/decisions.md#owner-decisions-2026-12-01--a-later-forward"
NEW = f"aging: smells.json: {TARGET}: a new size exclusion after S1.1c-5: clean the package instead (decision 1)"


def entry(target: str = TARGET, issue: str = "S1.1c-5-1", cap: int = 3) -> allowlists.Entry:
    return allowlists.Entry(
        target, "TEMP", "size family not yet cleaned", "S1.1c-5", expiry="2026-12-14", issue=issue, cap=cap
    )


def issue(id: str, forward: str, decision: str = D0) -> dict[str, object]:
    return {
        "id": id,
        "first_seen": "S1.1c-5",
        "forward": forward,
        "decision": decision,
        "correctness": False,
        "opened": "2026-11-14",
    }


def issues(*items: dict[str, object]) -> dict[str, dict[str, object]]:
    return {str(item["id"]): item for item in items}


def decided_by(*naming: tuple[str, str]):
    """A decision reader that resolves exactly the (id, decision) pairs given."""
    return lambda id, decision: (id, decision) in naming


NOTHING_DECIDED = decided_by()


def at_base(entries, known) -> parts.Base:
    return parts.Base("0123abcd", tuple(entries), known)


class FixtureBound(unittest.TestCase):
    """The move, cap and no-base rules are judged in ORDER's world, where S1.1c-7 and S1.1c-8 are printed: the bound
    is the fixture's S1.1c-8, as it was before S1.1c-5's close re-cut the parts to one (owner 2026-11-15). Rules
    tests the real bound."""

    def setUp(self) -> None:
        self.enterContext(mock.patch.object(parts, "LAST_PART", "S1.1c-8"))


class TheLiveBound(unittest.TestCase):
    """The real bound, read live; every other class judges in ORDER's world with a bound of its own, so a re-cut edits
    only this class."""

    def test_the_bound_is_the_part_that_cleans_the_last_size_issue(self):
        # S1.1c-14 cleans known issue S1.1c-5-3, the last size issue, after S4.2 (owner 2026-11-27, the names before
        # the pattern engine, decision 5; S1.1c-9 until then, owner 2026-11-20, S1.1c-7's plan check, decision 1).
        self.assertEqual(parts.LAST_PART, "S1.1c-14")

    def test_the_bound_is_a_printed_part_of_the_real_order(self):
        with open(os.path.join(REPO, "gate", "sprints.json"), encoding="utf-8") as f:
            order = sprintid.load_order(f.read())
        self.assertIs(order.reached_part(parts.LAST_PART, parts.LAST_PART), True)  # raises when not printed


class Rules(unittest.TestCase):
    """Rules 1-4: the entry's issue, its forward part, the bound on forwards and the marker reaching the part, judged
    in ORDER's world with its bound S1.1c-9 (the real bound is TheLiveBound's)."""

    def setUp(self) -> None:
        self.enterContext(mock.patch.object(parts, "LAST_PART", "S1.1c-9"))

    def run_rules(self, known, marker="S1.1c-5", tier="T4", entries=None):
        entries = (entry(),) if entries is None else entries
        # The base equals now, so the move check (rule 5) has nothing to say.
        return parts.forwards(entries, known, ORDER, marker, tier, at_base(entries, known), NOTHING_DECIDED)

    def test_no_issue_is_red(self):
        faults, _ = self.run_rules({})
        self.assertEqual(faults, [f"aging: smells.json: {TARGET}: issue S1.1c-5-1 has no forward part"])

    def test_an_empty_forward_is_red(self):
        faults, _ = self.run_rules(issues(issue("S1.1c-5-1", "")))
        self.assertEqual(faults, [f"aging: smells.json: {TARGET}: issue S1.1c-5-1 has no forward part"])

    def test_an_unprinted_forward_is_red(self):
        faults, _ = self.run_rules(issues(issue("S1.1c-5-1", "S1.1c-4")))
        self.assertEqual(
            faults, [f"aging: smells.json: {TARGET}: forward S1.1c-4 is not a printed part of gate/sprints.json"]
        )

    def test_a_forward_past_the_bound_is_red(self):
        faults, _ = self.run_rules(issues(issue("S1.1c-5-1", "S1.1c-10")))  # printed in ORDER's world
        want = f"aging: smells.json: {TARGET}: forward S1.1c-10 is past S1.1c-9 (decision 1)"
        self.assertEqual(faults, [want])

    def test_a_forward_at_the_bound_is_silent_until_the_marker_reaches_it(self):
        faults, lines = self.run_rules(issues(issue("S1.1c-5-1", "S1.1c-9")))
        self.assertEqual((faults, lines), ([], []))

    def reached(self) -> str:
        return (
            f"aging: smells.json: {TARGET}: the marker S1.1c-6 has reached its part S1.1c-6: clean internal/runtime"
            " for gocyclo, or the owner moves the forward"
        )

    def test_reached_at_t4_is_red(self):
        faults, _ = self.run_rules(issues(issue("S1.1c-5-1", "S1.1c-6")), marker="S1.1c-6", tier="T4")
        self.assertEqual(faults, [self.reached()])

    def test_reached_at_nightly_is_red(self):
        faults, _ = self.run_rules(issues(issue("S1.1c-5-1", "S1.1c-6")), marker="S1.1c-6", tier="nightly")
        self.assertEqual(faults, [self.reached()])

    def test_reached_at_t3_is_a_line_not_red(self):
        faults, lines = self.run_rules(issues(issue("S1.1c-5-1", "S1.1c-6")), marker="S1.1c-6", tier="T3")
        self.assertEqual(faults, [])
        self.assertEqual(lines, [self.reached()])

    def test_not_reached_is_silent(self):
        # Witness first: the same entry at its part is red (test_reached_at_t4_is_red); one part earlier, nothing.
        faults, lines = self.run_rules(issues(issue("S1.1c-5-1", "S1.1c-6")), marker="S1.1c-5", tier="T4")
        self.assertEqual((faults, lines), ([], []))


class Moves(FixtureBound):
    """Rule 5, per exclusion target: a later forward needs a new decision naming the issue (DQ-B1)."""

    def moved(self, old: str, new: str, base_id: str, id: str) -> str:
        return (
            f"aging: smells.json: {TARGET}: forward moved {old} → {new} (issue {base_id} → {id}) without a decision"
            f" that names {id} (decision 1: only the owner moves a forward)"
        )

    def test_a_forward_moved_without_a_decision_is_red(self):
        base = at_base((entry(),), issues(issue("S1.1c-5-1", "S1.1c-6")))
        now = issues(issue("S1.1c-5-1", "S1.1c-7"))
        faults, _ = parts.forwards((entry(),), now, ORDER, "S1.1c-5", "T4", base, NOTHING_DECIDED)
        self.assertEqual(faults, [self.moved("S1.1c-6", "S1.1c-7", "S1.1c-5-1", "S1.1c-5-1")])

    def test_moved_with_a_naming_decision_is_green(self):
        base = at_base((entry(),), issues(issue("S1.1c-5-1", "S1.1c-6")))
        now = issues(issue("S1.1c-5-1", "S1.1c-7", decision=D1))
        decided = decided_by(("S1.1c-5-1", D1))
        faults, _ = parts.forwards((entry(),), now, ORDER, "S1.1c-5", "T4", base, decided)
        self.assertEqual(faults, [])

    def test_moved_with_the_base_decision_is_red(self):
        # The decision names the id, but it is the decision the base already had: it did not move the forward.
        base = at_base((entry(),), issues(issue("S1.1c-5-1", "S1.1c-6")))
        now = issues(issue("S1.1c-5-1", "S1.1c-7"))
        decided = decided_by(("S1.1c-5-1", D0))
        faults, _ = parts.forwards((entry(),), now, ORDER, "S1.1c-5", "T4", base, decided)
        self.assertEqual(faults, [self.moved("S1.1c-6", "S1.1c-7", "S1.1c-5-1", "S1.1c-5-1")])

    def test_moved_earlier_is_green(self):
        base = at_base((entry(),), issues(issue("S1.1c-5-1", "S1.1c-7")))
        now = issues(issue("S1.1c-5-1", "S1.1c-6"))
        faults, _ = parts.forwards((entry(),), now, ORDER, "S1.1c-5", "T4", base, NOTHING_DECIDED)
        self.assertEqual(faults, [])

    def test_an_entry_repointed_to_a_later_issue_is_red(self):
        known = issues(issue("S1.1c-5-1", "S1.1c-6"), issue("S1.1c-5-2", "S1.1c-7"))
        base = at_base((entry(issue="S1.1c-5-1"),), known)
        faults, _ = parts.forwards((entry(issue="S1.1c-5-2"),), known, ORDER, "S1.1c-5", "T4", base, NOTHING_DECIDED)
        self.assertEqual(faults, [self.moved("S1.1c-6", "S1.1c-7", "S1.1c-5-1", "S1.1c-5-2")])

    def test_a_size_entry_absent_at_the_base_is_red_after_s1_1c_5(self):
        known = issues(issue("S1.1c-5-1", "S1.1c-7"))
        base = at_base((), known)
        faults, _ = parts.forwards((entry(),), known, ORDER, "S1.1c-6", "T4", base, NOTHING_DECIDED)
        self.assertEqual(faults, [NEW])

    def test_a_size_entry_absent_at_the_base_is_green_at_s1_1c_5(self):
        known = issues(issue("S1.1c-5-1", "S1.1c-7"))
        base = at_base((), known)
        faults, _ = parts.forwards((entry(),), known, ORDER, "S1.1c-5", "T4", base, NOTHING_DECIDED)
        self.assertEqual(faults, [])

    def test_a_removed_entry_is_green(self):
        known = issues(issue("S1.1c-5-1", "S1.1c-7"))
        removed = entry(target="internal/validate funlen")
        base = at_base((entry(), removed), known)
        faults, _ = parts.forwards((entry(),), known, ORDER, "S1.1c-6", "T4", base, NOTHING_DECIDED)
        self.assertEqual(faults, [])


# Owner 2026-11-14, S1.1c-5's review and break-it, decision 1 (BK-B1): only a docs/sprints/decisions.md section counts
# as a decision. The reader serves three tracked files whose heading names the id; only decisions.md's decides.
HEADING = "Owner decisions, 2026-12-01 — a later forward for S1.1c-5-1"
SECTION = f"# T\n\n## {HEADING}\n\nS1.1c-5-1 moves to S1.1c-7.\n"
RECORD = f"docs/sprints/s1-1c-5-record.md#{anchors.slug(HEADING)}"
REVIEW = f"docs/reviews/s1-1c-5-breakit-qa.md#{anchors.slug(HEADING)}"
IN_DECISIONS = f"docs/sprints/decisions.md#{anchors.slug(HEADING)}"
TRACKED = {path.split("#")[0]: SECTION for path in (RECORD, REVIEW, IN_DECISIONS)}


def real_decided(id: str, decision: str) -> bool:
    return aging.decided(id, decision, TRACKED.get)


class OnlyDecisionsMd(FixtureBound):
    def move(self, decision: str) -> list[str]:
        base = at_base((entry(),), issues(issue("S1.1c-5-1", "S1.1c-6")))
        now = issues(issue("S1.1c-5-1", "S1.1c-7", decision=decision))
        faults, _ = parts.forwards((entry(),), now, ORDER, "S1.1c-6", "T4", base, real_decided)
        return faults

    def test_a_forward_moved_with_a_decision_outside_decisions_md_is_red(self):
        want = (
            f"aging: smells.json: {TARGET}: forward moved S1.1c-6 → S1.1c-7 (issue S1.1c-5-1 → S1.1c-5-1) without a"
            " decision that names S1.1c-5-1 (decision 1: only the owner moves a forward)"
        )
        for ref in (RECORD, REVIEW):
            with self.subTest(ref=ref):
                self.assertEqual(self.move(ref), [want])

    def test_a_forward_moved_with_a_decision_in_decisions_md_is_green(self):
        self.assertEqual(self.move(IN_DECISIONS), [])


class Caps(FixtureBound):
    """Owner 2026-11-14, S1.1c-5's review and break-it, decision 2: a cap raised against the merge base needs a new
    decision naming the entry's issue, as a forward moved later does; a cap lowered is green."""

    def raised(self, old: int, new: int) -> str:
        return (
            f"aging: smells.json: {TARGET}: cap raised {old} → {new} (issue S1.1c-5-1) without a decision that names"
            " S1.1c-5-1 (decision 2: only the owner raises a cap)"
        )

    def judge(self, base_cap: int, cap: int, decision: str = D0, decided=NOTHING_DECIDED) -> list[str]:
        base = at_base((entry(cap=base_cap),), issues(issue("S1.1c-5-1", "S1.1c-7")))
        now = issues(issue("S1.1c-5-1", "S1.1c-7", decision=decision))
        faults, _ = parts.forwards((entry(cap=cap),), now, ORDER, "S1.1c-6", "T4", base, decided)
        return faults

    def test_a_cap_raised_without_a_decision_is_red(self):
        self.assertEqual(self.judge(3, 4), [self.raised(3, 4)])

    def test_a_cap_raised_with_the_base_decision_is_red(self):
        self.assertEqual(self.judge(3, 4, decided=decided_by(("S1.1c-5-1", D0))), [self.raised(3, 4)])

    def test_a_cap_raised_with_a_new_naming_decision_is_green(self):
        self.assertEqual(self.judge(3, 4, decision=D1, decided=decided_by(("S1.1c-5-1", D1))), [])

    def test_a_cap_raised_with_a_decision_outside_decisions_md_is_red(self):
        self.assertEqual(self.judge(3, 4, decision=RECORD, decided=real_decided), [self.raised(3, 4)])
        self.assertEqual(self.judge(3, 4, decision=IN_DECISIONS, decided=real_decided), [])  # witness

    def test_a_cap_kept_or_lowered_is_green(self):
        self.assertEqual(self.judge(3, 3), [])
        self.assertEqual(self.judge(3, 2), [])


class Base(FixtureBound):
    """Rule 6: no merge base (DQ-B3); a base file absent at a resolved base is not "no base"."""

    NO_BASE = "aging: smells.json: no merge base with origin/main or main: forward moves not checked (decision 1)"
    known = issues(issue("S1.1c-5-1", "S1.1c-7"))

    def test_no_merge_base_at_t4_is_red(self):
        faults, _ = parts.forwards(
            (entry(),), self.known, ORDER, "S1.1c-5", "T4", parts.Base("", None, None), NOTHING_DECIDED
        )
        self.assertEqual(faults, [self.NO_BASE])

    def test_no_merge_base_at_nightly_is_red(self):
        faults, _ = parts.forwards(
            (entry(),), self.known, ORDER, "S1.1c-5", "nightly", parts.Base("", None, None), NOTHING_DECIDED
        )
        self.assertEqual(faults, [self.NO_BASE])

    def test_no_merge_base_below_t4_is_a_line(self):
        faults, lines = parts.forwards(
            (entry(),), self.known, ORDER, "S1.1c-5", "T3", parts.Base("", None, None), NOTHING_DECIDED
        )
        self.assertEqual(faults, [])
        self.assertEqual(lines, ["aging: no merge base; forward moves not checked"])

    def test_the_base_file_absent_is_judged_as_new_entries(self):
        for base in (parts.Base("0123abcd", None, self.known), parts.Base("0123abcd", (), None)):
            with self.subTest(base=base):
                faults, _ = parts.forwards((entry(),), self.known, ORDER, "S1.1c-6", "T4", base, NOTHING_DECIDED)
                self.assertEqual(faults, [NEW])


def smells_json(*entries: dict[str, str]) -> str:
    return json.dumps({"schema": 1, "note": "fixture", "rule": "smells", "entries": list(entries)}) + "\n"


def known_issues_json(*items: dict[str, object]) -> str:
    return json.dumps({"schema": 1, "note": "fixture", "entries": list(items)}) + "\n"


ENTRY = {
    "target": TARGET,
    "kind": "TEMP",
    "reason": "size family not yet cleaned",
    "sprint": "S1.1c-5",
    "expiry": "2026-12-14",
    "issue": "S1.1c-5-1",
    "cap": 3,
}


class ReadBase(support.GateTest):
    """`read_base` in a temporary repository: main holds the base, HEAD a branch off it."""

    def commit(self, message: str) -> str:
        self.git("add", "-A")
        self.git(*IDENTITY, "commit", "-q", "--allow-empty", "-m", message)
        return self.git("rev-parse", "HEAD").decode().strip()

    def test_a_fixture_repo_with_a_base(self):
        self.git("checkout", "-q", "-b", "main")
        self.track("tools/allowlists/smells.json", smells_json(ENTRY))
        self.track("gate/known-issues.json", known_issues_json(issue("S1.1c-5-1", "S1.1c-6")))
        base = self.commit("base")
        self.git("checkout", "-q", "-b", "work")
        self.write("tools/allowlists/smells.json", smells_json())
        self.commit("change")
        got = parts.read_base(cwd=self.repo)
        self.assertEqual(got.commit, base)
        self.assertEqual(tuple(got.entries), (entry(),))
        self.assertEqual(dict(got.issues["S1.1c-5-1"]), issue("S1.1c-5-1", "S1.1c-6"))

    def test_without_a_base(self):
        self.git("checkout", "-q", "-b", "other")
        self.track("tools/allowlists/smells.json", smells_json(ENTRY))
        self.commit("no main anywhere")
        self.assertEqual(parts.read_base(cwd=self.repo), parts.Base("", None, None))

    def test_with_the_file_absent_at_the_base(self):
        self.git("checkout", "-q", "-b", "main")
        self.track("gate/known-issues.json", known_issues_json(issue("S1.1c-5-1", "S1.1c-6")))
        base = self.commit("base without smells.json")
        self.git("checkout", "-q", "-b", "work")
        self.track("tools/allowlists/smells.json", smells_json(ENTRY))
        self.commit("adds smells.json")
        got = parts.read_base(cwd=self.repo)
        self.assertEqual(got.commit, base)
        self.assertIsNone(got.entries)
        self.assertEqual(sorted(got.issues), ["S1.1c-5-1"])


if __name__ == "__main__":
    unittest.main()
