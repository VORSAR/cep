"""Verdict precedence: the first match gives the reason (design note §4, slots 1-11)."""

import dataclasses
import unittest

from gate.runner import verdict
from gate.runner.covered import Covered

ALL_WRONG = verdict.Facts(
    cap_s=60,
    min_wants=9,
    parity_failed=True,
    stopped="interrupted",
    timed_out=True,
    spawn_error="[Errno 2] no such file",
    scope_error="git ls-files exit 128",
    left_running=True,
    returncode=3,
    covered=Covered(0, "GATE_COVERED malformed"),
    expected=4,
)


class Precedence(unittest.TestCase):
    def test_each_slot_beats_every_slot_after_it(self):
        # Every condition holds at first; each step clears the winner, so the next slot must win.
        slots = (
            ("parity_failed", False, "not run: parity failed"),
            ("stopped", "", "interrupted"),
            ("timed_out", False, "timeout after 60s"),
            ("spawn_error", "", "could not start: [Errno 2] no such file"),
            ("scope_error", "", "scope error: git ls-files exit 128"),
            ("left_running", False, "left running processes"),
            ("returncode", 0, "exit 3"),
            ("covered", Covered(3, ""), "GATE_COVERED malformed"),
            ("expected", 3, "covered 3 ≠ expected 4"),
            ("min_wants", 0, "covered 3 < min_wants 9"),
        )
        facts = ALL_WRONG
        for field, cleared, reason in slots:
            with self.subTest(reason):
                got = verdict.decide(facts)
                self.assertEqual((got.ok, got.reason), (False, reason))
            facts = dataclasses.replace(facts, **{field: cleared})
        got = verdict.decide(facts)
        self.assertEqual((got.ok, got.reason), (True, "covered 3"))

    def test_a_signal_death_reads_killed_by_signal(self):
        facts = dataclasses.replace(ALL_WRONG, parity_failed=False, stopped="", timed_out=False, spawn_error="",
                                    scope_error="", left_running=False, returncode=-9)  # fmt: skip  # aligned keywords
        self.assertEqual(verdict.decide(facts).reason, "killed by signal 9")

    def test_an_exit_carries_the_rows_first_failure_line(self):
        # owner, 2026-11-03 (review BA3): a red row's summary shows its first failure line, not a bare "exit 1".
        clear = dataclasses.replace(ALL_WRONG, parity_failed=False, stopped="", timed_out=False, spawn_error="",
                                    scope_error="", left_running=False)  # fmt: skip  # aligned keywords
        named = dataclasses.replace(clear, returncode=1, first_failure="internal/event 85.10% (40/47) < 90%")
        self.assertEqual(verdict.decide(named).reason, "exit 1: internal/event 85.10% (40/47) < 90%")
        self.assertEqual(verdict.decide(dataclasses.replace(named, first_failure="")).reason, "exit 1")
        self.assertEqual(verdict.decide(dataclasses.replace(named, returncode=-9)).reason, "killed by signal 9")
        self.assertEqual(verdict.decide(dataclasses.replace(named, timed_out=True)).reason, "timeout after 60s")

    def test_a_t0_skip_comes_after_parity_and_before_everything_else(self):
        # gate design §12.3: "skip: nothing staged in scope" sits between rules 1 and 2.
        skipped = dataclasses.replace(ALL_WRONG, parity_failed=False, skipped=3)
        got = verdict.decide(skipped)
        self.assertEqual((got.ok, got.skip, got.reason), (True, True, "skip: nothing staged in scope (3 tracked)"))
        self.assertEqual(
            verdict.decide(dataclasses.replace(skipped, parity_failed=True)).reason, "not run: parity failed"
        )
        self.assertFalse(verdict.decide(dataclasses.replace(skipped, skipped=-1)).skip)

    def test_a_ceiling_breach_carries_its_own_reason(self):
        facts = dataclasses.replace(ALL_WRONG, parity_failed=False, stopped="run ceiling 420s exceeded")
        self.assertEqual(verdict.decide(facts).reason, "run ceiling 420s exceeded")

    def test_min_wants_equal_to_covered_passes(self):
        facts = verdict.Facts(cap_s=60, min_wants=4, returncode=0, covered=Covered(4, ""), expected=4)
        self.assertTrue(verdict.decide(facts).ok)


if __name__ == "__main__":
    unittest.main()


class LeftRunning(unittest.TestCase):
    def test_the_detail_rides_the_reason(self):
        facts = verdict.Facts(cap_s=60, left_running=True, left_detail="the group still exists: 123 S python3.15 x.py")
        self.assertEqual(
            verdict.decide(facts).reason, "left running processes: the group still exists: 123 S python3.15 x.py"
        )

    def test_without_a_detail_the_text_is_unchanged(self):
        self.assertEqual(verdict.decide(verdict.Facts(cap_s=60, left_running=True)).reason, "left running processes")
