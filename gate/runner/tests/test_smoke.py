"""The one real-clock test (design note §9, QA DQ28): the cap is small only on the row expected to time out."""

from gate.runner import clock
from gate.runner.tests import support


class RealClock(support.GateTest):
    def test_a_real_timer_kills_the_capped_row_and_the_others_pass(self):
        rows = [self.row("capped", "block", timeout_s=1), self.row("a", timeout_s=3600), self.row("b", timeout_s=3600)]
        result = self.run_rows(rows, clock=clock.REAL, timers=clock.RealTimers())
        self.assertEqual(result.exit_code, 1)
        self.assertEqual(self.reason("capped"), "timeout after 1s")
        self.assertEqual((self.reason("a"), self.reason("b")), ("covered 1", "covered 1"))
        self.assertGreaterEqual(self.record("lint", "capped")["duration_ms"], 1000)
        self.assertRegex(result.summary.splitlines()[-3], r"^wall \d+\.\ds · summed \d+\.\ds · saved -?\d+%$")
