"""gate/term.py: the first SIGTERM raises Terminated, so cleanups run; a later one changes nothing (review BA1)."""

import os
import signal
import unittest

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


if __name__ == "__main__":
    unittest.main()
