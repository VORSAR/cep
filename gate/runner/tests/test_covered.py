"""The GATE_COVERED grammar (design note §4; plan check C8; design-QA counter-example 22)."""

import unittest

from gate.runner import covered


class Parse(unittest.TestCase):
    def test_a_valid_line_gives_its_count(self):
        cases = (
            (b"GATE_COVERED 5\n", 5),
            (b"GATE_COVERED 5", 5),
            (b"noise\nGATE_COVERED 12\nmore\n", 12),
            (b"GATE_COVERED 999999999\n", 999999999),
            (b"GATE_COVERED 1\n", 1),
            (b"bad \xff bytes\nGATE_COVERED 7\n", 7),
        )
        for stdout, want in cases:
            with self.subTest(stdout):
                got = covered.parse(stdout)
                self.assertEqual((got.count, got.problem), (want, ""))

    def test_malformed_lines(self):
        cases = (
            b"GATE_COVERED 03\n",
            b"GATE_COVERED -1\n",
            b"GATE_COVERED 3 \n",
            b"GATE_COVERED\n",
            b"GATE_COVERED \n",
            b"GATE_COVERED 1180591620717411303424\n",
            b"GATE_COVERED 1000000000\n",
            b"GATE_COVERED +3\n",
            b"GATE_COVERED 0x3\n",
            b"GATE_COVERED  3\n",
            b"GATE_COVERED\t3\n",
            b"GATE_COVEREDX 3\n",
            "GATE_COVERED 1٣\n".encode(),
            "GATE_COVERED ４\n".encode(),
            b"GATE_COVERED 5\r\n",
            b"GATE_COVERED 5\r",
        )
        for stdout in cases:
            with self.subTest(stdout):
                got = covered.parse(stdout)
                self.assertEqual((got.count, got.problem), (0, "GATE_COVERED malformed"))

    def test_only_a_newline_ends_a_line(self):
        for stdout in (
            b"noise\x85GATE_COVERED 5\n",
            b"noise\x1cGATE_COVERED 5\n",
            b"noise\x0bGATE_COVERED 5\n",
            b"noise\rGATE_COVERED 5\n",
            "noise GATE_COVERED 5\n".encode(),
            b" GATE_COVERED 5\n",
        ):
            with self.subTest(stdout):
                got = covered.parse(stdout)
                self.assertEqual((got.count, got.problem), (0, "GATE_COVERED missing"))

    def test_missing(self):
        for stdout in (b"", b"\n", b"hello\n"):
            with self.subTest(stdout):
                self.assertEqual(covered.parse(stdout).problem, "GATE_COVERED missing")

    def test_zero_is_a_failure(self):
        got = covered.parse(b"GATE_COVERED 0\n")
        self.assertEqual((got.count, got.problem), (0, "GATE_COVERED 0"))

    def test_twice_is_a_failure_even_when_the_lines_agree(self):
        for stdout in (
            b"GATE_COVERED 1\nGATE_COVERED 1\n",
            b"GATE_COVERED 1\nGATE_COVERED 2\n",
            b"GATE_COVERED 1\nGATE_COVERED x\n",
        ):
            with self.subTest(stdout):
                got = covered.parse(stdout)
                self.assertEqual((got.count, got.problem), (0, "GATE_COVERED duplicated"))


if __name__ == "__main__":
    unittest.main()
