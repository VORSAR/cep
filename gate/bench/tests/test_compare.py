"""compare.py: ABBA time pairs, the exact tie-conditional Mann-Whitney p, and the fixed-iteration allocation pass."""

import contextlib
import faulthandler
import itertools
import math
import os
import shutil
import subprocess
import tempfile
import unittest
from fractions import Fraction

from gate.bench import compare
from gate.bench.tests import support
from gate.runner import env as gate_env


def brute_p(before: list[int], after: list[int]) -> Fraction:
    """Every split of the pooled samples, by hand: doubled midranks, |S - mean| at least the observed."""
    pooled = sorted(before + after)
    n = len(pooled)
    doubled = {v: pooled.index(v) + 1 + n - pooled[::-1].index(v) for v in pooled}
    ranks = [doubled[v] for v in before + after]
    k = len(after)
    mean = Fraction(k * (n + 1))
    observed = abs(sum(ranks[len(before) :]) - mean)
    splits = list(itertools.combinations(range(n), k))
    extreme = sum(1 for split in splits if abs(sum(ranks[i] for i in split) - mean) >= observed)
    return Fraction(extreme, len(splits))


class Statistic(unittest.TestCase):
    def test_the_program_equals_brute_force_with_ties(self):
        cases = [
            ([1, 2, 3], [4, 5, 6]),
            ([1, 2, 2, 3], [2, 3, 3, 4]),
            ([5, 5, 5], [5, 5, 5]),
            ([1, 1, 2], [2, 2, 3, 3]),
            ([10, 20, 30, 40], [15, 25]),
            ([7, 7, 8, 9, 9], [7, 8, 8, 9, 10]),
            ([3, 1, 4, 1, 5], [9, 2, 6, 5, 3, 5]),
        ]
        for before, after in cases:
            with self.subTest(before=before, after=after):
                self.assertEqual(compare.mann_whitney_p(before, after), brute_p(before, after))

    def test_all_values_tied_is_p_1(self):
        self.assertEqual(compare.mann_whitney_p([4] * 6, [4] * 6), Fraction(1))

    def test_ten_against_ten_fully_separated(self):
        p = compare.mann_whitney_p(list(range(10)), list(range(10, 20)))
        self.assertEqual(p, Fraction(2, 184756))
        self.assertEqual(f"{float(p):.4e}", "1.0825e-05")


def run_out(ns: int, bench: str = "BenchmarkA") -> dict:
    return {"out": support.output({bench: (ns,)})}


def alloc_out(b: int = 64, allocs: int = 2, bench: str = "BenchmarkA") -> dict:
    return {"out": support.output({bench: (500, b, allocs)})}


class Verdict(unittest.TestCase):
    def setUp(self) -> None:
        faulthandler.dump_traceback_later(120, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.dir = tmp.name
        self.log = os.path.join(self.dir, "log")

    def pair(self, before_time, after_time, before_alloc=None, after_alloc=None):
        before = support.fake(self.dir, "before", self.log, time=before_time, alloc=before_alloc or [alloc_out()])
        after = support.fake(self.dir, "after", self.log, time=after_time, alloc=after_alloc or [alloc_out()])
        return before, after

    def compare(self, before, after, *extra: str) -> tuple[int, str]:
        argv = ["--before", before, "--after", after, "--bench", ".", "--dir", self.dir, *extra]
        return support.compare(argv, self.dir)

    def test_plus_2_percent_exactly_is_no_cost(self):
        code, out = self.compare(*self.pair([run_out(1000)], [run_out(1020)]))
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "verdict: no cost")
        self.assertIn("p 1.083e-05", out)

    def test_plus_2_1_percent_significant_costs(self):
        code, out = self.compare(*self.pair([run_out(1000)], [run_out(1021)]))
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "verdict: costs (BenchmarkA)")

    def test_a_significant_1_percent_shift_is_no_cost(self):
        code, out = self.compare(*self.pair([run_out(1000)], [run_out(1010)]))
        self.assertEqual(code, 0, out)

    def test_an_allocation_difference_costs_whatever_the_times(self):
        before, after = self.pair([run_out(1000)], [run_out(900)], [alloc_out(64, 2)], [alloc_out(64, 3)])
        code, out = self.compare(before, after)
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "verdict: costs (BenchmarkA)")
        before, after = self.pair([run_out(1000)], [run_out(1000)], [alloc_out(64, 2)], [alloc_out(72, 2)])
        self.assertEqual(self.compare(before, after)[0], 1)

    def test_an_a_a_run_is_no_cost(self):
        runs = [run_out(ns) for ns in (1000, 1004, 997, 1010, 1001, 999, 1003, 1006)]
        binary = support.fake(self.dir, "same", self.log, time=runs, alloc=[alloc_out()])
        code, out = self.compare(binary, binary)
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "verdict: no cost")

    def test_abba_order_and_the_flags(self):
        before, after = self.pair([run_out(1000)], [run_out(1000)])
        code, out = self.compare(before, after, "--pairs", "4", "--benchtime", "2s", "--alloc-iters", "1000")
        self.assertEqual(code, 0, out)
        log = support.read_log(self.log)
        order = [side for side, kind, _ in log if kind == "time"]
        self.assertEqual(order, ["before", "after", "after", "before", "before", "after", "after", "before"])
        self.assertEqual([side for side, kind, _ in log if kind == "alloc"], ["before", "before", "after", "after"])
        time_argv = ["-test.run=^$", "-test.bench=.", "-test.cpu=1", "-test.benchtime=2s", "-test.count=1"]
        alloc_argv = ["-test.run=^$", "-test.bench=.", "-test.cpu=1", "-test.benchtime=1000x", "-test.benchmem"]
        self.assertEqual({tuple(a) for _, kind, a in log if kind == "time"}, {tuple(time_argv)})
        self.assertEqual({tuple(a) for _, kind, a in log if kind == "alloc"}, {tuple(alloc_argv)})

    def test_errors_are_exit_2(self):
        missing_b = {"out": support.output({"BenchmarkA": (1000,), "BenchmarkB": (10,)})}
        cases = {
            "no benchmark matched": ([{"out": support.HEADER + "PASS\n"}], [{"out": support.HEADER + "PASS\n"}]),
            "BenchmarkB": ([missing_b, run_out(1000)], [missing_b]),
            "panic: boom": ([run_out(1000)], [{"out": "", "err": "panic: boom\n", "exit": 2}]),
        }
        for reason, (before_time, after_time) in cases.items():
            with self.subTest(reason=reason):
                with contextlib.suppress(FileNotFoundError):
                    os.remove(self.log)
                code, out = self.compare(*self.pair(before_time, after_time))
                self.assertEqual(code, 2, out)
                self.assertTrue(out.splitlines()[-1].startswith("error: "), out)
                self.assertIn(reason, out.splitlines()[-1])

    def test_a_self_disagreeing_allocation_side_is_unjudgeable(self):
        before, after = self.pair([run_out(1000)], [run_out(1000)], [alloc_out(64, 2), alloc_out(64, 3)])
        code, out = self.compare(before, after)
        self.assertEqual(code, 2, out)
        self.assertIn("unjudgeable", out.splitlines()[-1])
        self.assertIn("BenchmarkA", out.splitlines()[-1])

    def test_unstable_bytes_are_left_out_and_the_counts_still_judge(self):
        # owner, 2026-11-17, S1.1c-6 lane R decision 2: scheduling-dependent batching moves B/op, never allocs/op
        before, after = self.pair(
            [run_out(1000)], [run_out(1000)], [alloc_out(2082, 3), alloc_out(2140, 3)], [alloc_out(2100, 3)]
        )
        code, out = self.compare(before, after)
        self.assertEqual(code, 0, out)
        self.assertIn("allocs/op 3/3, B/op unstable (before 2082-2140, after 2100-2100), left out: no cost", out)
        self.assertEqual(out.splitlines()[-1], "verdict: no cost", out)

    def test_unstable_bytes_never_hide_a_changed_allocation_count(self):
        before, after = self.pair(
            [run_out(1000)], [run_out(1000)], [alloc_out(2082, 3), alloc_out(2140, 3)], [alloc_out(2100, 4)]
        )
        code, out = self.compare(before, after)
        self.assertEqual(code, 1, out)
        self.assertIn("allocs/op 3/4", out)
        self.assertEqual(out.splitlines()[-1], "verdict: costs (BenchmarkA)", out)

    def test_stable_bytes_are_still_compared_exactly(self):
        before, after = self.pair([run_out(1000)], [run_out(1000)], [alloc_out(64, 2)], [alloc_out(65, 2)])
        code, out = self.compare(before, after)
        self.assertEqual(code, 1, out)
        self.assertIn("allocs/op 2/2, B/op 64/65: costs", out)

    def test_bad_arguments_are_exit_2(self):
        before, after = self.pair([run_out(1000)], [run_out(1000)])
        for extra in (["--pairs", "21"], ["--alloc-iters", "1000x"], ["--pairs", "0"]):
            with self.subTest(extra=extra):
                code, out = self.compare(before, after, *extra)
                self.assertEqual(code, 2, out)
                self.assertTrue(out.splitlines()[-1].startswith("error: "), out)
        code, out = self.compare(os.path.join(self.dir, "absent.test"), after)
        self.assertEqual(code, 2, out)
        self.assertIn("absent.test", out.splitlines()[-1])
        self.assertFalse(os.path.exists(self.log), "no binary runs when an argument is wrong")


MODULE = "module example.com/b\n\ngo 1.27\n"
BENCH = """\
package b

import "testing"

var sink []int

func BenchmarkGrow(b *testing.B) {
\tfor i := 0; i < b.N; i++ {
\t\tsink = make([]int, 8)
\t}
}
"""


class RealGo(unittest.TestCase):
    def test_a_real_test_binary_against_itself(self):
        faulthandler.dump_traceback_later(300, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        env = {**gate_env.clean_env(os.environ, []), "GOPROXY": "off"}
        go = shutil.which("go", path=env["PATH"])
        self.assertIsNotNone(go, "the pinned go is on PATH")
        for name, text in (("go.mod", MODULE), ("b_test.go", BENCH)):
            with open(os.path.join(tmp.name, name), "w", encoding="utf-8") as f:
                f.write(text)
        binary = os.path.join(tmp.name, "b.test")
        subprocess.run([go, "test", "-c", "-o", binary], cwd=tmp.name, env=env, check=True, capture_output=True)
        argv = ["--before", binary, "--after", binary, "--bench", "Grow", "--dir", tmp.name, "--pairs", "2"]
        argv += ["--benchtime", "100x", "--alloc-iters", "1000", "--before-label", "c^", "--after-label", "c"]
        code, out = support.compare(argv, tmp.name, env)
        self.assertEqual(code, 0, out)
        lines = out.splitlines()
        self.assertEqual(lines[-1], "verdict: no cost")
        self.assertTrue(lines[0].startswith("BenchmarkGrow: "), out)
        self.assertIn("allocs/op 1/1", lines[0])
        self.assertIn("B/op 64/64", lines[0])
        self.assertIn("before label: c^", lines)
        self.assertIn("after label: c", lines)
        version = [line for line in lines if line.startswith("go version before: ")]
        self.assertEqual(len(version), 1, out)
        self.assertIn("go1.27", version[0])
        self.assertEqual(len([line for line in lines if line.startswith("go buildid after: ")]), 1, out)
        self.assertEqual(math.comb(4, 2), 6)  # at pairs 2 the smallest two-sided p is 2/6 = 1/3


if __name__ == "__main__":
    unittest.main()
