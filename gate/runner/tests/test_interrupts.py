"""Interrupts and the run ceiling (design note §4; plan check case 9, C5, C6; design-QA 19-21)."""

import json
import os
import signal
import threading

from gate import wait
from gate.runner.tests import support


def valid_files(test, run_dir, rows):
    for job, row_id in rows:
        test.assertPinnedJson(os.path.join(run_dir, "records", job, f"{row_id}.json"))
    test.assertPinnedJson(os.path.join(run_dir, "run.json"))
    with open(os.path.join(run_dir, "summary.md"), encoding="utf-8") as f:
        test.assertEqual(f.read().splitlines()[-2:], ["GATE FAILED", "exit: 1"])


class Signals(support.GateTest):
    def test_two_held_rows_die_and_are_recorded(self):
        # design-QA counter-example 19: neither row ends on its own, and nothing is released but the signal.
        w = self.witness("start")
        h = self.start([self.row("x", "start", w.path, "block"), self.row("y", "start", w.path, "block")])
        w.started(2)
        h.runner.post_signal(signal.SIGINT)
        result = h.join()
        self.assertEqual(result.exit_code, 1)
        self.assertEqual((self.reason("x"), self.reason("y")), ("interrupted", "interrupted"))
        w.gone()

    def test_a_grandchild_dies_with_its_group(self):
        # plan check case 9: nothing outlives the runner.
        w = self.witness("start")
        h = self.start([self.row("x", "spawn", 3, "start", w.path, "block", "start", w.path, "block")])
        w.started(2)
        h.runner.post_signal(signal.SIGTERM)
        h.join()
        w.gone()

    def test_a_row_ignoring_term_dies_when_the_grace_period_ends(self):
        w = self.witness("start")
        h = self.start([self.row("x", "term", w.path)])
        w.started(1)
        h.runner.post_signal(signal.SIGINT)
        w.more(1)  # the row saw TERM and carried on
        grace = self.timers.expect("interrupt-grace")
        self.assertEqual(grace.delay_s, 5)
        grace.fire()
        h.join()
        self.assertEqual(self.reason("x"), "interrupted")
        w.gone()

    def test_a_second_signal_kills_at_once(self):
        # design-QA counter-example 20: the grace timer is never released.
        w = self.witness("start")
        h = self.start([self.row("x", "term", w.path)])
        w.started(1)
        h.runner.post_signal(signal.SIGINT)
        w.more(1)
        grace = self.timers.expect("interrupt-grace")
        h.runner.post_signal(signal.SIGINT)
        h.join()
        self.assertTrue(grace.cancelled)
        self.assertEqual(self.reason("x"), "interrupted")
        w.gone()

    def test_no_row_starts_after_the_signal(self):
        hold = self.fifo("hold")
        w = self.witness("start")
        rows = [self.row("x", "start", w.path, "wait", hold), self.row("y"), self.row("z", needs=("x",))]
        h = self.start(rows, workers=1)
        w.started(1)
        h.runner.post_signal(signal.SIGINT)
        h.join()
        self.assertEqual(h.events("start"), ["parity", "x"])
        self.assertEqual(h.events("not-run"), ["y", "z"])
        self.assertEqual((self.reason("y"), self.reason("z")), ("interrupted", "interrupted"))

    def test_a_row_finished_before_the_signal_keeps_its_verdict(self):
        w = self.witness("start")
        h = self.start([self.row("done"), self.row("x", "start", w.path, "block", needs=("done",))])
        w.started(1)
        h.runner.post_signal(signal.SIGINT)
        h.join()
        self.assertEqual(self.reason("done"), "covered 1")
        self.assertEqual(self.reason("x"), "interrupted")

    def test_interrupt_beats_timeout(self):
        # Slot 2 before slot 3.
        w = self.witness("start")
        h = self.start([self.row("x", "term", w.path)])
        w.started(1)
        self.timers.expect("watchdog:lint/x").fire()
        w.more(1)
        h.runner.post_signal(signal.SIGINT)
        self.timers.expect("interrupt-grace").fire()
        h.join()
        self.assertEqual(self.reason("x"), "interrupted")


class Points(support.GateTest):
    """C6: an interrupt at each scheduler point leaves complete, valid files."""

    def test_during_parity(self):
        self.toolchain.hold = threading.Event()
        self.addCleanup(self.toolchain.hold.set)
        h = self.start([self.row("x")])
        wait.until("parity to be running", lambda: self.toolchain.calls)
        h.runner.post_signal(signal.SIGINT)
        result = h.join()
        self.assertEqual(result.exit_code, 1)
        self.assertEqual(self.reason("parity"), "interrupted")
        self.assertEqual(self.reason("x"), "interrupted")
        self.assertEqual(h.events("start"), ["parity"])
        valid_files(self, self.run_dir, [("lint", "parity"), ("lint", "x")])

    def test_between_rows(self):
        hold = self.fifo("hold")
        w = self.witness("start")
        h = self.start([self.row("a"), self.row("b", "start", w.path, "wait", hold, needs=("a",)),
                        self.row("c", needs=("b",))], workers=1)  # fmt: skip  # aligned arguments
        w.started(1)
        h.runner.post_signal(signal.SIGINT)
        h.join()
        valid_files(self, self.run_dir, [("lint", "parity"), ("lint", "a"), ("lint", "b"), ("lint", "c")])

    def test_with_every_row_running(self):
        w = self.witness("start")
        h = self.start([self.row(f"r{i}", "start", w.path, "block") for i in range(4)])
        w.started(4)
        h.runner.post_signal(signal.SIGINT)
        h.join()
        valid_files(self, self.run_dir, [("lint", f"r{i}") for i in range(4)])
        w.gone()


class CeilingBreach(support.GateTest):
    def test_the_interrupt_path_with_its_own_reason(self):
        # design-QA counter-example 21.
        w = self.witness("start")
        rows = [self.row("x", "start", w.path, "block"), self.row("y", needs=("x",))]
        h = self.start(rows, workers=1)
        w.started(1)
        self.timers.expect("ceiling").fire()
        result = h.join()
        self.assertEqual(result.exit_code, 1)
        want = "run ceiling 180s exceeded"
        self.assertEqual((self.reason("x"), self.reason("y")), (want, want))
        w.gone()
        with open(os.path.join(self.run_dir, "trace.json"), "rb") as f:
            self.assertIn("ceiling", [e["event"] for e in json.loads(f.read())])

    def test_a_signal_after_the_ceiling_kills_at_once(self):
        w = self.witness("start")
        h = self.start([self.row("x", "term", w.path)])
        w.started(1)
        self.timers.expect("ceiling").fire()
        w.more(1)
        h.runner.post_signal(signal.SIGINT)
        h.join()
        self.assertEqual(self.reason("x"), "run ceiling 120s exceeded")


class RealSignals(support.GateTest):
    """The handlers themselves: a real signal, delivered while the scheduler runs on the main thread."""

    def run_on_main_thread(self, signum):
        w = self.witness("start")
        runner = self.runner([self.row("x", "spawn", 3, "start", w.path, "block", "start", w.path, "block")],
                             install_signals=True)  # fmt: skip  # aligned arguments
        before = signal.getsignal(signum)

        def driver():
            w.started(2)
            os.kill(os.getpid(), signum)

        thread = threading.Thread(target=driver, name="signal-driver", daemon=True)
        thread.start()
        result = runner.run()
        thread.join()
        self.assertEqual(result.exit_code, 1)
        self.assertEqual(self.reason("x"), "interrupted")
        self.assertEqual(signal.getsignal(signum), before)
        w.gone()

    def test_a_signal_caught_by_another_thread_still_interrupts(self):
        # The kernel may hand a process's signal to any thread. The C-level handler then runs there, and the
        # scheduler, blocked on its queue, would not run the Python-level one until something else woke it: an
        # interrupt lost until a row ends (architect DA5). Directing the signal at the driver makes that certain.
        w = self.witness("start")
        runner = self.runner([self.row("x", "start", w.path, "block")], install_signals=True)
        lost = []

        def driver():
            w.started(1)
            signal.pthread_kill(threading.get_ident(), signal.SIGINT)
            try:
                w.gone()
            except wait.WaitTimeout as e:
                lost.append(str(e))
                runner.post_signal(signal.SIGINT)
                runner.post_signal(signal.SIGINT)

        thread = threading.Thread(target=driver, name="signal-driver", daemon=True)
        thread.start()
        runner.run()
        thread.join()
        self.assertEqual(lost, [])
        self.assertEqual(self.reason("x"), "interrupted")

    def test_sigint(self):
        self.run_on_main_thread(signal.SIGINT)

    def test_sigterm(self):
        # C5: the same outcome as Ctrl-C.
        self.run_on_main_thread(signal.SIGTERM)

    def test_sighup(self):
        # DA22: SIGHUP is treated like SIGTERM.
        self.run_on_main_thread(signal.SIGHUP)
