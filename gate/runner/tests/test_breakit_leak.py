"""Break-it of S1.1a-1's combined build (QA), QB4: a stopped row leaves its new-session descendants running forever.

When the runner stops a row (its watchdog, or an interrupt), it sends TERM, then KILL, to the row's process group and
to nothing else. A test module of the gate's own suite routinely starts helpers in sessions of their own: every
in-test Runner spawns its rows with start_new_session=True, and the toolchain tests' children do too. Each such test
cleans its helpers up through addCleanup, but TERM's default action ends the module's process before any cleanup
runs, so every helper outlives the row, the run, and the gate. Seen for real: one Ctrl-C during `make check`'s
pytests row left four rowfix.py `start ... block` fixtures and a fake `git ls-files` running, parented to launchd;
this machine also held about twenty such orphans of earlier runs, up to a day old.

Fixed (review BA1): the shim and the wrapper turn TERM into an exception, so cleanups run inside the grace.

The planted module below is that pattern in miniature: a helper in its own session, which holds a witness FIFO open,
an addCleanup that would kill it, and a test that blocks until it is stopped. The witness: the same helper in the
row's own process group is gone once the row is stopped, so the test's machinery sees "gone" when it happens.
"""

import contextlib
import os
import signal

from gate import wait
from gate.runner.tests import support
from gate.runner.tests.test_rows_py import TESTS, Base, wrapper

CAP_S = 20  # a killed process is gone within milliseconds
HOLD = """\
import os
import signal
import subprocess
import sys
import unittest

HELPER = (
    "import os, signal\\n"
    "with open(PIDFILE, 'w') as f:\\n"
    "    f.write(str(os.getpid()))\\n"
    "fd = os.open(WITNESS, os.O_WRONLY)\\n"
    "os.write(fd, b's')\\n"
    "while True:\\n"
    "    signal.pause()\\n"
)


class T(unittest.TestCase):
    def test_holds_a_helper(self):
        helper = subprocess.Popen(
            [sys.executable, "-I", "-c", HELPER], stdin=subprocess.DEVNULL, start_new_session=NEW_SESSION
        )
        self.addCleanup(helper.wait)
        self.addCleanup(helper.kill)
        signal.pause()
"""


class QB4StoppedRowLeavesNothing(Base):
    def plant(self, *, new_session: bool):
        """Track the holding module; return the witness whose one start byte says the helper is running."""
        w = self.witness("start")
        self.pidfile = os.path.join(self.root, "helper.pid")
        self.addCleanup(self.kill_helper)
        text = HOLD.replace("PIDFILE", repr(self.pidfile)).replace("WITNESS", repr(w.path))
        self.track("gate/x/test_hold.py", text.replace("NEW_SESSION", repr(new_session)))
        return w

    def kill_helper(self) -> None:
        """This test owns its world: a helper that outlived the run is killed here, whatever the verdict."""
        with contextlib.suppress(FileNotFoundError, ValueError), open(self.pidfile, encoding="ascii") as f:
            pid = int(f.read())
            with contextlib.suppress(ProcessLookupError):
                os.kill(pid, signal.SIGKILL)

    def start_pytests(self):
        return self.start([self.row("pytests", command=wrapper("pytests"), expected_scope=TESTS, job="test")])

    def assertAllGone(self, w) -> None:
        """End-of-file on the witness: every process that held it is gone. Only after its start byte was read."""
        self.assertEqual(w.expected, 1, "no positive witness: the helper's start byte was never read")

        def at_eof() -> bool:
            try:
                return os.read(w.fd, 64) == b""
            except BlockingIOError:
                return False

        try:
            wait.until("every process the stopped row started to be gone", at_eof, cap_s=CAP_S)
        except wait.WaitTimeout:
            with open(self.pidfile, encoding="ascii") as f:
                pid = f.read()
            self.fail(f"pid {pid}, started by the stopped pytests row, is still running {CAP_S}s after the run ended")

    def stop_by_watchdog(self, h, w) -> None:
        """TERM, then the grace's KILL. The fix runs cleanups on TERM, inside the grace (review BA1), so the helper
        must be gone before the grace ends: the KILL that follows can then only reach what TERM left."""
        self.timers.expect("watchdog:test/pytests").fire()
        self.assertAllGone(w)
        self.timers.expect("grace:test/pytests").fire()
        h.join()
        self.assertEqual(self.reason("pytests", "test"), "timeout after 60s")

    def test_qb4_witness_a_helper_in_the_rows_group_is_gone_after_a_timeout(self):
        w = self.plant(new_session=False)
        h = self.start_pytests()
        w.started(1)
        self.stop_by_watchdog(h, w)

    def test_qb4_a_helper_in_its_own_session_is_gone_after_a_timeout(self):
        w = self.plant(new_session=True)
        h = self.start_pytests()
        w.started(1)
        self.stop_by_watchdog(h, w)

    def test_qb4_a_helper_in_its_own_session_is_gone_after_an_interrupt(self):
        w = self.plant(new_session=True)
        h = self.start_pytests()
        w.started(1)
        h.runner.post_signal(signal.SIGINT)
        self.assertAllGone(w)
        self.timers.expect("interrupt-grace").fire()
        h.join()
        self.assertEqual(self.reason("pytests", "test"), "interrupted")

    def test_a_module_not_yet_started_never_starts_after_the_stop(self):
        # review BA1: the wrapper waits for its running modules and starts no other once it is stopped.
        w = self.plant(new_session=True)
        later = os.path.join(self.root, "later")
        self.track("gate/x/test_later.py", f"open({later!r}, 'w').close()\n")  # sorts after test_hold.py
        h = self.start([self.row("pytests", command=wrapper("pytests"), expected_scope=TESTS, job="test")],
                       environ={**support.base_env(), "VORSAR_CEP_WORKERS": "1"})  # fmt: skip
        w.started(1)
        self.stop_by_watchdog(h, w)
        self.assertFalse(os.path.exists(later))
