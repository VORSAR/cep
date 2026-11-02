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
                                    scope_error="", left_running=False, returncode=-9)  # fmt: skip
        self.assertEqual(verdict.decide(facts).reason, "killed by signal 9")

    def test_a_ceiling_breach_carries_its_own_reason(self):
        facts = dataclasses.replace(ALL_WRONG, parity_failed=False, stopped="run ceiling 420s exceeded")
        self.assertEqual(verdict.decide(facts).reason, "run ceiling 420s exceeded")

    def test_min_wants_equal_to_covered_passes(self):
        facts = verdict.Facts(cap_s=60, min_wants=4, returncode=0, covered=Covered(4, ""), expected=4)
        self.assertTrue(verdict.decide(facts).ok)


if __name__ == "__main__":
    unittest.main()
