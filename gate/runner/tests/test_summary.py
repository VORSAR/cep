"""The summary, byte-exact (design note §4; design-QA counter-examples 25-27).

Every expected value here is worked out by hand from the design note's rules.
"""

import unittest

from gate.runner import summary
from gate.runner.summary import Line


class Tenths(unittest.TestCase):
    def test_truncated_never_rounded(self):
        for ms, want in ((0, "0.0s"), (99, "0.0s"), (100, "0.1s"), (150, "0.1s"), (199, "0.1s"), (999, "0.9s"),
                         (1500, "1.5s"), (1999, "1.9s"), (60000, "60.0s"), (31000, "31.0s")):  # fmt: skip
            with self.subTest(ms):
                self.assertEqual(summary.tenths(ms), want)


class Footer(unittest.TestCase):
    def test_design_examples(self):
        self.assertEqual(summary.footer(200, 300), "wall 0.2s · summed 0.3s · saved 33%")
        self.assertEqual(summary.footer(31000, 42700), "wall 31.0s · summed 42.7s · saved 27%")

    def test_a_negative_saving_uses_floor_division(self):
        # (1000 - 1155) * 100 // 1000 = -15500 // 1000 = -16 (truncation would say -15).
        self.assertEqual(summary.footer(1155, 1000), "wall 1.1s · summed 1.0s · saved -16%")

    def test_nothing_summed_saves_zero(self):
        self.assertEqual(summary.footer(0, 0), "wall 0.0s · summed 0.0s · saved 0%")


class Render(unittest.TestCase):
    def test_widths_follow_the_longest_value_or_header(self):
        lines = [
            Line("*", "parity", "PASS", 200, "covered 5"),
            Line("lint", "gofmt", "PASS", 1500, "covered 12"),
            Line("test", "coverage-inclusion", "FAIL", 100, "exit 1"),
            Line("proofs", "proofs", "WARN", 60000, "covered 8 < min_wants 10"),
        ]
        want = (
            "job    | step               | verdict | time  | reason\n"
            "*      | parity             | PASS    | 0.2s  | covered 5\n"
            "lint   | gofmt              | PASS    | 1.5s  | covered 12\n"
            "test   | coverage-inclusion | FAIL    | 0.1s  | exit 1\n"
            "proofs | proofs             | WARN    | 60.0s | covered 8 < min_wants 10\n"
            "wall 60.2s · summed 61.8s · saved 2%\n"
            "GATE FAILED\n"
            "exit: 1\n"
        )
        self.assertEqual(summary.render(lines, wall_ms=60200, summed_ms=61800, exit_code=1), want)

    def test_short_values_never_shrink_a_column_below_its_header(self):
        lines = [Line("a", "b", "PASS", 0, "c")]
        want = (
            "job | step | verdict | time | reason\n"
            "a   | b    | PASS    | 0.0s | c\n"
            "wall 0.0s · summed 0.0s · saved 0%\n"
            "GATE PASSED\n"
            "exit: 0\n"
        )
        self.assertEqual(summary.render(lines, wall_ms=0, summed_ms=0, exit_code=0), want)

    def test_the_last_two_lines_follow_the_exit_code(self):
        for code, word in ((0, "GATE PASSED"), (1, "GATE FAILED"), (2, "GATE FAILED")):
            with self.subTest(code):
                last = summary.render([], wall_ms=0, summed_ms=0, exit_code=code).splitlines()[-2:]
                self.assertEqual(last, [word, f"exit: {code}"])

    def test_no_line_has_trailing_whitespace(self):
        text = summary.render([Line("a", "b", "FAIL", 0, "")], wall_ms=0, summed_ms=0, exit_code=1)
        for line in text.splitlines():
            self.assertEqual(line, line.rstrip())


if __name__ == "__main__":
    unittest.main()
