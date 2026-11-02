"""With SIGCHLD ignored, subprocess reports a dead child as exit 0: a false green (design-QA counter-example 12).

The row exits 1 and prints a correct count, so a reported exit 0 would be a pass. This must run in-process: on
darwin an ignored SIGCHLD inherited across exec does not show the bug (DQ5). Banned everywhere but this file.
"""

import signal

from gate.runner import main
from gate.runner.tests import support


class FalseGreen(support.GateTest):
    def test_a_failing_row_stays_failed_with_sigchld_ignored_at_entry(self):
        before = signal.getsignal(signal.SIGCHLD)
        self.addCleanup(signal.signal, signal.SIGCHLD, before)
        signal.signal(signal.SIGCHLD, signal.SIG_IGN)
        registry = self.registry_file([self.good("x", "status", 1)])
        code = main.main(["--tier", "T2", "--registry", registry, "--run-dir", self.run_dir], deps=self.deps())
        self.assertEqual(self.reason("x"), "exit 1")
        self.assertEqual(code, 1)
        self.assertEqual(signal.getsignal(signal.SIGCHLD), signal.SIG_DFL)
