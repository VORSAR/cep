"""The two injected clocks, the real timer, and run ids (design note §4)."""

import queue
import re
import unittest

from gate import wait
from gate.runner import clock


class RunId(unittest.TestCase):
    def test_the_epoch(self):
        self.assertEqual(clock.run_id(0, "abc123"), "19700101T000000Z-abc123")

    def test_a_date_worked_out_with_date_1(self):
        # `date -u -j -f "%Y-%m-%dT%H:%M:%S" 2026-11-02T15:04:05 +%s` prints 1793631845.
        self.assertEqual(clock.run_id(1793631845999, "00ff00"), "20261102T150405Z-00ff00")

    def test_a_fresh_token_is_six_hex(self):
        self.assertTrue(re.fullmatch(r"\d{8}T\d{6}Z-[0-9a-f]{6}", clock.run_id(0)))


class RealClock(unittest.TestCase):
    def test_wall_is_epoch_ms(self):
        # After 2026-01-01 and before 2100: the value is milliseconds, not seconds or nanoseconds.
        self.assertTrue(1_767_225_600_000 < clock.REAL.wall_ms() < 4_102_444_800_000)

    def test_mono_never_goes_backwards(self):
        a = clock.REAL.mono_ns()
        self.assertGreaterEqual(clock.REAL.mono_ns(), a)


class RealTimers(unittest.TestCase):
    def test_fires_once_after_its_delay(self):
        q = queue.SimpleQueue()
        clock.RealTimers().start("t", 0, lambda: q.put("fired"))
        self.assertEqual(wait.get(q, "the timer to fire"), "fired")

    def test_a_cancelled_timer_does_not_fire(self):
        q = queue.SimpleQueue()
        timers = clock.RealTimers()
        timers.start("late", 3600, lambda: q.put("late")).cancel()
        timers.start("witness", 0, lambda: q.put("witness"))
        self.assertEqual(wait.get(q, "the witness timer to fire"), "witness")
        self.assertTrue(q.empty())


if __name__ == "__main__":
    unittest.main()
