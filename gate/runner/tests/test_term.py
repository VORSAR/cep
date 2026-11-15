"""gate/term.py: the first SIGTERM raises Terminated, so cleanups run; a later one changes nothing (review BA1)."""

import os
import signal
import subprocess
import sys
import unittest
from unittest import mock

from gate import term


class Install(unittest.TestCase):
    def setUp(self):
        self.addCleanup(signal.signal, signal.SIGTERM, signal.getsignal(signal.SIGTERM))

    def test_the_first_term_runs_the_hook_then_raises_and_a_second_changes_nothing(self):
        calls = []
        term.install(lambda: calls.append("hook"))
        with self.assertRaises(term.Terminated):
            os.kill(os.getpid(), signal.SIGTERM)
            signal.pause()  # reached only if the signal were not yet handled; the handler's raise ends it
        self.assertEqual(calls, ["hook"])
        os.kill(os.getpid(), signal.SIGTERM)  # a cleanup in progress is never cut short by a second TERM
        self.assertEqual(calls, ["hook"])

    def test_an_except_exception_cannot_swallow_the_stop(self):
        self.assertFalse(issubclass(term.Terminated, Exception))
        self.assertTrue(issubclass(term.Terminated, BaseException))


class Critical(unittest.TestCase):
    """A TERM inside `term.critical()` is raised when the block ends, never in its middle (S1.1c-5's T4: a stop in
    the spawn-and-hold window lost the child; 19/20 under the stress harness, 2026-11-15)."""

    def setUp(self):
        self.addCleanup(signal.signal, signal.SIGTERM, signal.getsignal(signal.SIGTERM))

    def test_a_term_inside_a_critical_block_is_raised_when_the_block_ends(self):
        calls, reached = [], []
        term.install(lambda: calls.append("hook"))
        with self.assertRaises(term.Terminated), term.critical():
            os.kill(os.getpid(), signal.SIGTERM)
            reached.append("after the kill")  # runs: the handler has run (calls holds the hook), the stop waits
        self.assertEqual((calls, reached), (["hook"], ["after the kill"]))

    def test_a_critical_block_without_a_term_raises_nothing(self):
        term.install()
        with term.critical():
            pass

    def test_a_term_after_a_critical_block_is_raised_at_once(self):
        term.install()
        with term.critical():
            pass
        with self.assertRaises(term.Terminated):
            os.kill(os.getpid(), signal.SIGTERM)
            signal.pause()  # reached only if the signal were not yet handled


class Popen(unittest.TestCase):
    def test_a_stop_inside_popens_own_start_up_kills_the_child_it_had_forked(self):
        # The S1.1b load race: the first TERM raises wherever the program is, and that can be inside Popen's own
        # start-up after the fork. The caller then never holds the child, so popen itself must kill it.
        forked = []
        real = subprocess.Popen._execute_child

        def interrupted(self, *args, **kwargs):
            real(self, *args, **kwargs)
            forked.append(self.pid)
            raise term.Terminated("planted: the TERM landed here")

        with mock.patch.object(subprocess.Popen, "_execute_child", interrupted), self.assertRaises(term.Terminated):
            term.popen(
                [sys.executable, "-I", "-c", "import signal\nwhile True: signal.pause()"], stdin=subprocess.DEVNULL
            )
        self.assertEqual(len(forked), 1)
        with self.assertRaises(ProcessLookupError):  # killed and reaped, so its pid is free
            os.kill(forked[0], 0)

    def test_the_child_runs_in_a_session_of_its_own(self):
        proc = term.popen([sys.executable, "-I", "-c", "import os; print(os.getsid(0))"], stdout=subprocess.PIPE)
        out, _ = proc.communicate()
        self.assertEqual(int(out), proc.pid)


if __name__ == "__main__":
    unittest.main()
