"""load.py: a go test run under CPU hogs, its per-test pass counts from -json, and hogs gone whatever happened."""

import contextlib
import faulthandler
import json
import os
import signal
import subprocess
import sys
import tempfile
import unittest

from gate import wait
from gate.bench.tests import support

CAP_S = 60
MODULE = "module example.com/l\n\ngo 1.27\n"
SOURCE = "package l\n\nfunc One() int { return 1 }\n"
TESTS = """\
package l

import "testing"

func TestOK(t *testing.T) {
\tif One() != 1 {
\t\tt.Fatal("One() != 1")
\t}
}
"""


def event(action: str, test: str | None = None, output: str | None = None) -> str:
    record = {"Action": action, "Package": "example.com/l"}
    if test is not None:
        record["Test"] = test
    if output is not None:
        record["Output"] = output
    return json.dumps(record) + "\n"


def died_in_a_test() -> str:
    """TestOK passes; the binary dies under TestSlow (go test's -timeout panic): test2json gives TestSlow a run action
    and no result, a fail for the package only, and the panic as output events, followed by more goroutine lines
    than the tail holds."""
    lines = [event("run", "TestOK"), event("output", "TestOK", "=== RUN   TestOK\n")]
    lines += [event("output", "TestOK", "--- PASS: TestOK (0.00s)\n"), event("pass", "TestOK")]
    lines += [event("run", "TestSlow"), event("output", "TestSlow", "=== RUN   TestSlow\n")]
    lines += [event("output", None, "panic: test timed out after 3s\n"), event("output", None, "\trunning tests:\n")]
    lines += [event("output", None, "\t\tTestSlow (3s)\n")]
    lines += [event("output", None, f"goroutine {i} [sleep]:\n") for i in range(30)]
    lines += [event("output", None, "FAIL\texample.com/l\t3.012s\n"), event("fail")]
    return "".join(lines)


def count3_flaky() -> str:
    """-count=3 over TestOK (passes every run) and TestFlaky (fails its second run), with a subtest of TestOK."""
    lines = []
    for run in (1, 2, 3):
        lines += [event("run", "TestOK"), event("run", "TestOK/sub"), event("pass", "TestOK/sub")]
        lines += [event("pass", "TestOK"), event("run", "TestFlaky")]
        lines.append(event("fail" if run == 2 else "pass", "TestFlaky"))
    lines += ["FAIL\n", event("fail")]
    return "".join(lines)


class Base(unittest.TestCase):
    def setUp(self) -> None:
        faulthandler.dump_traceback_later(600, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = tmp.name
        self.log = os.path.join(self.tmp, "go.log")

    def fake(self, stdout: str, exit_code: int = 0, *, hang: bool = False, log: bool = True) -> dict[str, str]:
        """A fake go that logs its pid and argv, prints `stdout`, then exits or blocks until stopped. With log=False it
        blocks before logging: a go the wall clock stops before its first line, as on a loaded machine."""
        body = (
            "import os, signal, sys\n"
            f"while not {log!r}:\n"
            "    signal.pause()\n"
            f"with open({self.log!r}, 'a', encoding='utf-8') as f:\n"
            "    f.write(str(os.getpid()) + ' ' + ' '.join(sys.argv[1:]) + '\\n')\n"
            f"sys.stdout.write({stdout!r})\n"
            "sys.stdout.flush()\n"
            f"while {hang!r}:\n"
            "    signal.pause()\n"
            f"sys.exit({exit_code})\n"
        )
        return {**os.environ, "PATH": support.fake_go(self.tmp, body) + os.pathsep + os.environ["PATH"]}

    def load(self, argv: list[str], env: dict[str, str], cwd: str | None = None) -> tuple[int, str]:
        return support.tool("load", argv, cwd or self.tmp, env, os.path.join(self.tmp, "load.stdout"))

    def assertHogsGone(self, out: str) -> None:
        pids = support.pids_line(out, "hogs:")
        self.assertEqual(len(pids), 2, out)  # the witness: the hogs ran
        self.assertEqual([p for p in pids if support.alive(p)], [], out)

    def logged(self) -> list[str]:
        with open(self.log, encoding="utf-8") as f:
            return f.read().splitlines()

    def has_logged(self) -> bool:
        """The fake wrote its line (the file exists from its open, before the write)."""
        return os.path.exists(self.log) and bool(self.logged())

    def running_fakes(self) -> list[str]:
        """The processes still running this test's fake go, found by its path: covers one stopped before it logged."""
        fake = os.path.join(self.tmp, "fakebin", "go")
        ps = subprocess.run(["ps", "-axww", "-o", "pid=,command="], capture_output=True, text=True, check=True)
        return [line.strip() for line in ps.stdout.splitlines() if fake in line]


class Fake(Base):
    def test_a_passing_run_stops_the_hogs(self):
        code, out = self.load(["--hogs", "2", "--", "-count=2", "./..."], self.fake(event("pass", "TestOK") * 2))
        self.assertEqual(code, 0, out)
        self.assertIn("TestOK: 2/2 under load (hogs 2)", out)
        self.assertHogsGone(out)
        self.assertEqual(self.logged()[0].split(" ", 1)[1], "test -json -count=2 ./...")

    def test_pass_counts_are_parsed_per_top_level_test_and_a_failing_run_stops_the_hogs(self):
        code, out = self.load(["--hogs", "2", "--", "-count=3", "./..."], self.fake(count3_flaky(), 1))
        self.assertEqual(code, 1, out)
        self.assertIn("TestFlaky: 2/3 under load (hogs 2)", out)
        self.assertIn("TestOK: 3/3 under load (hogs 2)", out)
        self.assertNotIn("TestOK/sub", out)
        self.assertLess(out.index("TestFlaky:"), out.index("TestOK:"))  # sorted
        self.assertHogsGone(out)

    def test_a_run_killed_by_the_wall_clock_is_exit_2_and_stops_the_hogs(self):
        code, out = self.load(["--hogs", "2", "--wall", "2", "--", "./..."], self.fake("", hang=True))
        self.assertEqual(code, 2, out)
        self.assertIn("killed", out)
        self.assertHogsGone(out)
        self.assertEqual(self.running_fakes(), [], out)
        # on a loaded machine the wall clock may stop the fake before it logs (the next test): its pid, when logged
        if self.has_logged():
            self.assertFalse(support.alive(int(self.logged()[0].split()[0])), out)

    def test_a_go_stopped_by_the_wall_clock_before_it_logs_is_exit_2_and_leaves_no_go(self):
        code, out = self.load(["--hogs", "2", "--wall", "2", "--", "./..."], self.fake("", log=False))
        self.assertEqual(code, 2, out)
        self.assertIn("killed", out)
        self.assertHogsGone(out)
        self.assertEqual(self.running_fakes(), [], out)
        self.assertFalse(os.path.exists(self.log), out)  # the witness: stopped before its first line

    def test_a_test_the_binary_died_under_counts_as_failed_and_the_panic_is_shown(self):
        # Owner 2026-11-19 (BK-S1): a run action with no result is a failed run of that test, and a red run's tail
        # comes from the output events, where -json puts the binary's panic: the panic: and FAIL lines, the panic
        # line kept although 30 goroutine lines follow it. Go test's stderr is empty here, as under -json.
        code, out = self.load(["--hogs", "2", "--", "-count=1", "./..."], self.fake(died_in_a_test(), 1))
        self.assertEqual(code, 1, out)
        self.assertIn("TestOK: 1/1 under load (hogs 2)\n", out)
        self.assertIn("TestSlow: 0/1 under load (hogs 2)\n", out)
        self.assertIn("\n  panic: test timed out after 3s\n  FAIL\texample.com/l\t3.012s\nverdict: ", out)
        self.assertTrue(out.endswith("verdict: below N/N: TestSlow; go test exit 1\n"), out)
        self.assertNotIn("goroutine", out)
        self.assertHogsGone(out)

    def test_help_prints_the_usage_and_exits_0_before_any_go_command(self):
        code, out = self.load(["--help"], self.fake(""))
        self.assertEqual(code, 0, out)
        self.assertTrue(out.startswith("usage: gate/bench/load.py "), out)
        self.assertNotIn("error:", out)
        self.assertFalse(os.path.exists(self.log), "a go command ran")

    def test_no_test_ran_is_exit_2(self):
        code, out = self.load(["--hogs", "2", "--", "./..."], self.fake(event("pass")))
        self.assertEqual(code, 2, out)
        self.assertHogsGone(out)

    def test_a_sigterm_leaves_no_hog_and_no_go(self):
        env = self.fake("", hang=True)
        script = os.path.join(support.REPO, "gate", "bench", "load.py")
        out_path = os.path.join(self.tmp, "load.stdout")
        with open(out_path, "wb") as out:
            proc = subprocess.Popen(
                [sys.executable, "-I", "-B", script, "--hogs", "2", "--", "./..."],
                cwd=self.tmp,
                env=env,
                stdin=subprocess.DEVNULL,
                stdout=out,
                stderr=out,
            )
        self.addCleanup(proc.wait)
        self.addCleanup(self.kill, proc)
        wait.until("the fake go to log its start", self.has_logged, cap_s=CAP_S)
        go_pid = int(self.logged()[0].split()[0])
        self.assertTrue(support.alive(go_pid))  # the witness
        self.assertNotEqual(self.running_fakes(), [])  # running_fakes sees a live fake
        proc.send_signal(signal.SIGTERM)
        self.assertEqual(proc.wait(timeout=CAP_S), 2)
        with open(out_path, encoding="utf-8") as f:
            text = f.read()
        self.assertHogsGone(text)
        self.assertFalse(support.alive(go_pid), text)

    @staticmethod
    def kill(proc: subprocess.Popen[bytes]) -> None:
        with contextlib.suppress(ProcessLookupError):
            proc.kill()


class Real(Base):
    def setUp(self) -> None:
        super().setUp()
        self.module = os.path.join(self.tmp, "mod")
        support.write(self.module, {"go.mod": MODULE, "l.go": SOURCE, "l_test.go": TESTS})

    def test_count_5_is_5_of_5_under_load(self):
        code, out = self.load(["--hogs", "2", "--", "-count=5", "-run", "^TestOK$", "."], os.environ, self.module)
        self.assertEqual(code, 0, out)
        self.assertIn("TestOK: 5/5 under load (hogs 2)", out)
        self.assertHogsGone(out)

    def test_the_coverprofile_is_kept(self):
        profile = os.path.join(self.tmp, "cover.prof")
        argv = ["--hogs", "2", "--coverprofile", profile, "--", "-count=1", "-run", "^TestOK$", "."]
        code, out = self.load(argv, os.environ, self.module)
        self.assertEqual(code, 0, out)
        with open(profile, encoding="utf-8") as f:
            self.assertTrue(f.read().startswith("mode: "))
        self.assertIn(f"coverprofile: {profile}", out)


# TestOK passes; TestSlow outlives go test's -timeout, so the binary panics ("panic: test timed out after …") while
# TestSlow runs: the package fails, and the panic text reaches go test -json's stdout as output events.
TIMEOUT_TESTS = """\
package l

import (
\t"testing"
\t"time"
)

func TestOK(t *testing.T) {
\tif One() != 1 {
\t\tt.Fatal("One() != 1")
\t}
}

func TestSlow(t *testing.T) {
\ttime.Sleep(time.Minute)
}
"""


class DiedOutsideATest(Base):
    """Break-it (S1.1c-6 combined build, BK-S1): load.py's verdict on a test binary that dies outside a test's own
    result."""

    def setUp(self) -> None:
        super().setUp()
        self.module = os.path.join(self.tmp, "mod")
        support.write(self.module, {"go.mod": MODULE, "l.go": SOURCE, "l_test.go": TIMEOUT_TESTS})

    def test_a_binary_that_times_out_names_the_test_or_the_cause(self):
        # The verdict must say why the run is red: the test that was running (below N/N) or the panic's line. Today
        # the tail printed is stderr's, which is empty under -json, so the verdict reads "below N/N: none; go test
        # exit 1" with no cause, as it did for internal/event under -race -count=4 -cpu 1,4 and 28 hogs.
        code, out = self.load(["--hogs", "1", "--", "-count=1", "-timeout=3s", "."], os.environ, self.module)
        self.assertEqual(code, 1, out)
        self.assertIn("TestSlow: 0/1 under load (hogs 1)\n", out)
        self.assertIn("\n  panic: test timed out after 3s\n", out)
        self.assertTrue(out.endswith("verdict: below N/N: TestSlow; go test exit 1\n"), out)


if __name__ == "__main__":
    unittest.main()
