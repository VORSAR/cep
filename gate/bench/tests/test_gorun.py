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

# A child that starts a grandchild in its own process group, writes both pids, then waits forever. It unblocks
# SIGTERM only after the write: started with SIGTERM blocked (the limit case), it holds a TERM that comes during its
# start-up until both processes exist, and the grandchild, started before the unblock, keeps TERM blocked.
PARENT = """\
import os, signal, subprocess, sys
g = subprocess.Popen([sys.executable, "-I", "-c", "import signal\\nwhile True:\\n    signal.pause()"])
with open(sys.argv[1] + ".tmp", "w") as f:
    f.write(f"{os.getpid()} {g.pid}")
os.rename(sys.argv[1] + ".tmp", sys.argv[1])
signal.pthread_sigmask(signal.SIG_UNBLOCK, {signal.SIGTERM})
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
        # The limit never races the child's start-up: the child starts with SIGTERM blocked (a signal mask passes
        # through fork and exec), so the limit's TERM, early or late, kills it only once both pids are written. The
        # grace is the cap on that, ended by the child's death. A 0 s limit comes during start-up, a 2 s one usually
        # while the child waits; both end the same way.
        for timeout_s in (0.0, 2.0):
            with self.subTest(timeout_s=timeout_s):
                self.check_the_limit_kills_the_child_and_its_grandchild(timeout_s)

    def check_the_limit_kills_the_child_and_its_grandchild(self, timeout_s: float) -> None:
        pids = os.path.join(self.dir, f"pids-{timeout_s}")
        self.addCleanup(self.kill_all, pids)
        blocked = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
        try:
            argv = [sys.executable, "-I", "-c", PARENT, pids]
            result = gorun.run(argv, cwd=self.dir, timeout_s=timeout_s, grace_s=CAP_S)
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, blocked)
        self.assertTrue(result.timed_out)
        # The positive witness: the child died of the limit's TERM, which it takes only after it started the grandchild
        # and wrote both pids; a KILL at the grace's end, before that, would be -SIGKILL.
        self.assertEqual(result.exit, -signal.SIGTERM, result)
        child, grandchild = read_pids(pids)
        # The grandchild blocks TERM: only the KILL to the group ends it.
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

    def test_a_stop_while_the_child_starts_kills_the_child_group_first(self):
        # The stop lands inside run's critical() block, deterministically: the driver's term.popen stand-in starts the
        # real child, waits for its pid file (the child and its grandchild exist), sends the driver its own TERM and
        # returns. critical() holds that TERM and raises it as the block ends. The stand-in exits 4 unless the TERM is
        # held, so the window is a witness, not a hope; the driver exits 3 when the stop comes out of run as Terminated.
        pids = os.path.join(self.dir, "pids")
        self.addCleanup(self.kill_all, pids)
        driver = (
            "import os, signal, sys\n"
            f"sys.path.insert(0, {REPO!r})\n"
            "from gate import term, wait\n"
            "from gate.bench import gorun\n"
            "real_popen = term.popen\n"
            "def popen_then_stop(argv, **kwargs):\n"
            "    proc = real_popen(argv, **kwargs)\n"
            f"    wait.until('the child pid file', lambda: os.path.exists({pids!r}), cap_s={CAP_S!r})\n"
            "    os.kill(os.getpid(), signal.SIGTERM)\n"
            "    if not term._held['pending']:\n"
            "        os._exit(4)\n"
            "    return proc\n"
            "signal.signal(signal.SIGCHLD, signal.SIG_DFL)\n"
            "term.install()\n"
            "term.popen = popen_then_stop\n"
            "try:\n"
            f"    gorun.run([sys.executable, '-I', '-c', {PARENT!r}, {pids!r}], cwd={self.dir!r}, timeout_s=600)\n"
            "except term.Terminated:\n"
            "    sys.exit(3)\n"
        )
        err_path = os.path.join(self.dir, "driver.stderr")
        with open(err_path, "wb") as err:
            proc = subprocess.Popen(
                [sys.executable, "-I", "-B", "-c", driver], stdin=subprocess.DEVNULL, stdout=err, stderr=err
            )
        self.addCleanup(proc.wait)
        self.addCleanup(lambda: proc.poll() is None and proc.kill())
        code = proc.wait(timeout=CAP_S)
        with open(err_path, "rb") as f:
            self.assertEqual(code, 3, f.read().decode("utf-8", errors="replace"))
        # The positive witness: the child ran and started its grandchild before the stop (both pids written).
        child, grandchild = read_pids(pids)
        wait.until(
            "the child and grandchild of a run stopped while starting gone",
            lambda: gone(child) and gone(grandchild),
            cap_s=CAP_S,
        )


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
