"""gorun: a child in its own session, its group killed on the limit or a stop, and go test -bench lines parsed."""

import faulthandler
import os
import signal
import subprocess
import sys
import tempfile
import unittest
from fractions import Fraction

from gate import wait
from gate.bench import gorun

CAP_S = 20.0  # a killed process is gone within milliseconds

# A child that starts a grandchild in its own process group, writes both pids, then waits forever.
PARENT = """\
import os, signal, subprocess, sys
g = subprocess.Popen([sys.executable, "-I", "-c", "import signal\\nwhile True:\\n    signal.pause()"])
with open(sys.argv[1] + ".tmp", "w") as f:
    f.write(f"{os.getpid()} {g.pid}")
os.rename(sys.argv[1] + ".tmp", sys.argv[1])
while True:
    signal.pause()
"""


def gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    return False


def read_pids(path: str) -> list[int]:
    with open(path, encoding="ascii") as f:
        return [int(p) for p in f.read().split()]


class Run(unittest.TestCase):
    def setUp(self) -> None:
        faulthandler.dump_traceback_later(60, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.dir = tmp.name

    def kill_all(self, path: str) -> None:
        if os.path.exists(path):
            for pid in read_pids(path):
                if not gone(pid):
                    os.kill(pid, signal.SIGKILL)

    def test_a_child_past_the_limit_is_killed_with_its_grandchild(self):
        pids = os.path.join(self.dir, "pids")
        self.addCleanup(self.kill_all, pids)
        result = gorun.run([sys.executable, "-I", "-c", PARENT, pids], cwd=self.dir, timeout_s=2.0)
        self.assertTrue(result.timed_out)
        # The positive witness: both processes existed (their pids were written before the limit).
        child, grandchild = read_pids(pids)
        wait.until("the timed-out child and its grandchild gone", lambda: gone(child) and gone(grandchild), cap_s=CAP_S)

    def test_exit_code_and_output_come_through(self):
        code = "import sys; print('out ü'); print('err', file=sys.stderr); sys.exit(3)"
        result = gorun.run([sys.executable, "-I", "-c", code], cwd=self.dir, timeout_s=30.0)
        self.assertEqual(result, gorun.Result(exit=3, stdout="out ü\n", stderr="err\n", timed_out=False))

    def test_a_stopped_parent_kills_the_child_group_first(self):
        pids = os.path.join(self.dir, "pids")
        self.addCleanup(self.kill_all, pids)
        driver = (
            "import sys\n"
            f"sys.path.insert(0, {REPO!r})\n"
            "from gate import term\n"
            "from gate.bench import gorun\n"
            "term.install()\n"
            f"gorun.run([sys.executable, '-I', '-c', {PARENT!r}, {pids!r}], cwd={self.dir!r}, timeout_s=600)\n"
        )
        with open(os.path.join(self.dir, "driver.stderr"), "wb") as err:
            proc = subprocess.Popen(
                [sys.executable, "-I", "-B", "-c", driver], stdin=subprocess.DEVNULL, stdout=err, stderr=err
            )
        self.addCleanup(proc.wait)
        self.addCleanup(lambda: proc.poll() is None and proc.kill())
        wait.until("the child's pid file", lambda: os.path.exists(pids), cap_s=CAP_S)
        child, grandchild = read_pids(pids)
        proc.send_signal(signal.SIGTERM)
        self.assertNotEqual(proc.wait(timeout=CAP_S), 0)
        wait.until("the stopped run's child and grandchild gone", lambda: gone(child) and gone(grandchild), cap_s=CAP_S)


REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

# Real `go test -bench` output (Go 1.27, darwin/arm64), hand-copied in its exact layout: tabs between fields.
REAL = (
    "goos: darwin\n"
    "goarch: arm64\n"
    "pkg: example.com/b\n"
    "cpu: Apple M2\n"
    "BenchmarkDecode-8   \t 1000000\t      1052 ns/op\t     128 B/op\t       2 allocs/op\n"
    "BenchmarkSmall     \t12345678\t        12.53 ns/op\n"
    "BenchmarkSub/size=10-8         \t  500000\t      2400 ns/op\t   3.20 MB/s\t       0 B/op\t       0 allocs/op\n"
    "BenchmarkSub/case_a        \t 20000\t     51234.5 ns/op\t      7.000 widgets/op\n"
    "PASS\n"
    "ok  \texample.com/b\t3.210s\n"
)


class ParseBench(unittest.TestCase):
    def test_real_output_in_every_form(self):
        self.assertEqual(
            gorun.parse_bench(REAL),
            [
                gorun.Bench("BenchmarkDecode", 1000000, Fraction(1052), 128, 2),
                gorun.Bench("BenchmarkSmall", 12345678, Fraction("12.53"), None, None),
                gorun.Bench("BenchmarkSub/size=10", 500000, Fraction(2400), 0, 0),
                gorun.Bench("BenchmarkSub/case_a", 20000, Fraction("51234.5"), None, None),
            ],
        )

    def test_at_cpu_1_no_suffix_is_stripped(self):
        # Go omits the -N suffix at GOMAXPROCS 1, so a name's own trailing -4 stays.
        line = "BenchmarkSub/n-4 \t 100\t 1234 ns/op\n"
        self.assertEqual(
            gorun.parse_bench(line, cpu=1), [gorun.Bench("BenchmarkSub/n-4", 100, Fraction(1234), None, None)]
        )
        self.assertEqual(gorun.parse_bench(line)[0].name, "BenchmarkSub/n")

    def test_a_numeric_benchmark_line_that_does_not_parse_is_an_error(self):
        for bad in (
            "BenchmarkX \t 100\n",
            "BenchmarkX \t 1.5\t 12 ns/op\n",
            "BenchmarkX \t 100\t 12 B/op\n",
            "BenchmarkX \t 100\t abc ns/op\n",
            "BenchmarkX \t 100\t 12 ns/op\t 3\n",
            "BenchmarkX \t 100\t 12 ns/op\t 1.5 allocs/op\n",
        ):
            with self.subTest(bad=bad), self.assertRaises(gorun.ParseError):
                gorun.parse_bench(bad)

    def test_non_result_lines_are_skipped(self):
        text = "BenchmarkX\n--- FAIL: BenchmarkY\nBenchmarks are fun\nFAIL\n"
        self.assertEqual(gorun.parse_bench(text), [])


if __name__ == "__main__":
    unittest.main()
