"""Scheduling: parallelism, needs, exclusive, determinism (plan check §6, C1-C3; design-QA 25-27, 34, 35)."""

import json
import os
import sys
from unittest import mock

from gate.runner import clock, scheduler
from gate.runner.tests import support


class Rendezvous(support.GateTest):
    """Plan check case 1: X opens write then read, Y read then write, so they finish only side by side."""

    def rows(self):
        f1, f2 = self.fifo("f1"), self.fifo("f2")
        return [self.good("x", "openw", f1, "openr", f2), self.good("y", "openr", f1, "openw", f2)]

    def test_two_workers_finish_together(self):
        result = self.run_rows(self.rows(), workers=2)
        self.assertEqual(result.exit_code, 0)

    def test_one_worker_times_out_both(self):
        h = self.start(self.rows(), workers=1)
        h.saw("start", "x")
        self.timers.expect("watchdog:lint/x")
        self.assertEqual(h.events("start"), ["parity", "x"])  # the witness: x is alone, y has not started
        self.timers.expect("watchdog:lint/x").fire()
        h.saw("end", "x")
        self.timers.expect("watchdog:lint/y").fire()
        result = h.join()
        self.assertEqual(result.exit_code, 1)
        self.assertEqual(self.reason("x"), "timeout after 60s")
        self.assertEqual(self.reason("y"), "timeout after 60s")


class Needs(support.GateTest):
    def test_y_starts_only_after_x_ends(self):
        # C2: an independent row Z starts and ends while X is held: Y had its chance to start early.
        hold = self.fifo("hold")
        rows = [self.good("x", "wait", hold), self.row("y", needs=("x",)), self.row("z")]
        h = self.start(rows)
        h.saw("end", "z")
        self.assertNotIn("y", h.events("start"))
        support.release(hold)
        h.join()
        self.assertLess(h.seq("end", "x"), h.seq("start", "y"))

    def test_a_dependent_row_runs_after_its_need_failed_and_names_what_is_missing(self):
        # design-QA counter-example 34: the need is killed before writing its artifact.
        artifact = os.path.join(self.run_dir, "rows", "test", "unit", "cover.prof")
        rows = [
            self.good("unit", "killself", 9, job="test"),
            self.good("coverage", "needfile", artifact, job="test", needs=("unit",)),
        ]
        self.run_rows(rows)
        self.assertEqual(self.reason("unit", "test"), "killed by signal 9")
        self.assertEqual(self.reason("coverage", "test"), "exit 1")
        self.assertIn(b"cover.prof", self.row_file("test", "coverage", "stdout"))

    def test_a_needed_row_shares_its_directory_with_the_row_that_needs_it(self):
        # plan check S3d: the needed row's GATE_ROW_DIR is the sibling of this row's.
        code = (
            "import os\n"
            "open(os.path.join(os.environ['GATE_ROW_DIR'], 'artifact'), 'w').close()\n"
            "print('GATE_COVERED 1')\n"
        )
        reader = (
            "import os\n"
            "assert os.path.exists(os.path.join(os.path.dirname(os.environ['GATE_ROW_DIR']), 'x', 'artifact'))\n"
            "print('GATE_COVERED 1')\n"
        )
        python = sys.executable
        rows = [self.row("x", command=(python, "-I", "-c", code)),
                self.row("y", command=(python, "-I", "-c", reader), needs=("x",))]  # fmt: skip
        self.assertEqual(self.run_rows(rows).exit_code, 0)


class Exclusive(support.GateTest):
    def test_case_3_nothing_overlaps_an_exclusive_row(self):
        holds = {name: self.fifo(name) for name in ("a", "b", "e")}
        rows = [
            self.good("a", "wait", holds["a"]),
            self.good("b", "wait", holds["b"]),
            self.row("gate-open"),
            self.good("e", "wait", holds["e"], exclusive=True, needs=("gate-open",)),
        ]
        h = self.start(rows, workers=8)
        h.saw("end", "gate-open")  # E is now ready, with A and B running
        self.assertNotIn("e", h.events("start"))
        support.release(holds["a"])
        h.saw("end", "a")
        self.assertNotIn("e", h.events("start"))
        support.release(holds["b"])
        h.saw("start", "e")
        support.release(holds["e"])
        h.join()
        self.assertLess(h.seq("end", "a"), h.seq("start", "e"))
        self.assertLess(h.seq("end", "b"), h.seq("start", "e"))

    def test_c3_a_row_ready_later_cannot_starve_a_waiting_exclusive_row(self):
        # DQ27: C sits AHEAD of E in the registry and becomes ready while E waits. C must not jump the queue.
        holds = {name: self.fifo(name) for name in ("w", "w2", "a", "e")}
        rows = [
            self.good("w", "wait", holds["w"]),
            self.good("w2", "wait", holds["w2"]),
            self.good("a", "wait", holds["a"]),
            self.row("c", needs=("w2",)),
            self.good("e", "wait", holds["e"], exclusive=True, needs=("w",)),
        ]
        h = self.start(rows, workers=8)
        support.release(holds["w"])
        h.saw("end", "w")  # E is ready and waiting on w2 and a
        support.release(holds["w2"])
        h.saw("end", "w2")  # C is ready now: the witness that it had its chance
        self.assertNotIn("c", h.events("start"))
        support.release(holds["a"])
        h.saw("start", "e")
        self.assertNotIn("c", h.events("start"))
        support.release(holds["e"])
        h.join()
        self.assertLess(h.seq("end", "e"), h.seq("start", "c"))

    def test_an_exclusive_row_ready_at_the_start_runs_before_rows_ahead_of_it(self):
        # "When an exclusive row is ready, no other row starts."
        hold = self.fifo("e")
        rows = [self.row("a"), self.good("e", "wait", hold, exclusive=True), self.row("b")]
        h = self.start(rows, workers=8)
        h.saw("start", "e")
        self.assertEqual(h.events("start"), ["parity", "e"])
        support.release(hold)
        h.join()
        self.assertLess(h.seq("end", "e"), h.seq("start", "a"))

    def test_nothing_starts_while_an_exclusive_row_dies_slowly(self):
        # The one event that can arrive while an exclusive row runs is its own watchdog. It wakes the scheduler,
        # and the row, ignoring TERM, lives on through its grace period: still running, still alone.
        w = self.witness("start")
        rows = [self.good("e", "term", w.path, exclusive=True), self.row("after")]
        h = self.start(rows, workers=8)
        w.started(1)
        self.timers.expect("watchdog:lint/e").fire()
        w.more(1)
        self.timers.expect("grace:lint/e").fire()
        h.join()
        self.assertLess(h.seq("end", "e"), h.seq("start", "after"))

    def test_two_exclusive_rows_run_one_at_a_time(self):
        e1, e2 = self.fifo("e1"), self.fifo("e2")
        rows = [self.good("e1", "wait", e1, exclusive=True), self.good("e2", "wait", e2, exclusive=True)]
        h = self.start(rows, workers=8)
        h.saw("start", "e1")
        self.assertNotIn("e2", h.events("start"))
        support.release(e1)
        support.release(e2)
        h.join()
        self.assertLess(h.seq("end", "e1"), h.seq("start", "e2"))


class Workers(support.GateTest):
    def test_never_more_rows_than_workers(self):
        holds = [self.fifo(f"h{i}") for i in range(3)]
        rows = [self.good(f"r{i}", "wait", holds[i]) for i in range(3)]
        h = self.start(rows, workers=2)
        h.saw("start", "r1")
        self.assertEqual(h.events("start"), ["parity", "r0", "r1"])
        support.release(holds[0])
        h.saw("start", "r2")
        support.release(holds[1])
        support.release(holds[2])
        self.assertEqual(h.join().exit_code, 0)

    def test_c1_one_worker_and_four_give_the_same_summary(self):
        # ADR-0015 §1.1: parallelism never changes a verdict. Frozen clock, so the bytes match in full.
        rows = [self.row("a"), self.good("b", "status", 1), self.row("c", needs=("a",)), self.row("d", min_wants=2)]
        summaries = []
        for workers in (1, 4):
            run_dir = os.path.join(self.root, f"run-{workers}")
            os.mkdir(run_dir)
            summaries.append(support.masked(self.run_rows(rows, workers=workers, run_dir=run_dir).summary, run_dir))
        self.assertEqual(summaries[0], summaries[1])
        self.assertIn("exit 1", summaries[0])


class Deadlock(support.GateTest):
    def test_each_pending_row_fails_naming_the_deadlock(self):
        # Validation makes this unreachable, so the rows skip it: a cycle in rows built by hand.
        rows = [self.row("ok"), self.row("x", needs=("y",)), self.row("y", needs=("x",))]
        result = self.run_rows(rows)
        self.assertEqual(result.exit_code, 1)
        self.assertEqual(self.reason("ok"), "covered 1")
        self.assertEqual(self.reason("x"), "not run: dependency deadlock")
        self.assertEqual(self.reason("y"), "not run: dependency deadlock")


class Warn(support.GateTest):
    def test_a_non_blocking_failure_warns_and_the_run_passes(self):
        # design-QA counter-example 35.
        result = self.run_rows([self.row("a"), self.good("b", "status", 1, blocking=False)])
        self.assertEqual(result.exit_code, 0)
        line = next(x for x in result.summary.splitlines() if " b " in x)
        self.assertEqual(line.split(" | ")[2].strip(), "WARN")
        self.assertEqual(result.summary.splitlines()[-2:], ["GATE PASSED", "exit: 0"])
        self.assertEqual(self.record("lint", "b")["verdict"], "fail")

    def test_every_record_says_whether_its_row_blocks(self):
        # review BA12 (owner, 2026-11-03): a WARN's record reads "fail", so the record must say it does not block.
        self.run_rows([self.row("a"), self.good("b", "status", 1, blocking=False)])
        self.assertEqual((self.record("lint", "a")["blocking"], self.record("lint", "b")["blocking"]), (True, False))
        self.assertTrue(self.record("lint", "parity")["blocking"])
        with open(os.path.join(self.run_dir, "run.json"), "rb") as f:
            run = json.loads(f.read())
        got = [(r["id"], r["blocking"]) for r in run["records"]]
        self.assertEqual(got, [("parity", True), ("a", True), ("b", False)])

    def test_a_blocking_failure_beside_it_still_fails_the_run(self):
        result = self.run_rows([self.good("a", "status", 1), self.good("b", "status", 1, blocking=False)])
        self.assertEqual(result.exit_code, 1)

    def test_a_non_blocking_pass_is_a_plain_pass(self):
        result = self.run_rows([self.row("b", blocking=False)])
        self.assertIn("PASS", next(x for x in result.summary.splitlines() if " b " in x))


class CompletionOrder(support.GateTest):
    def run_in_order(self, first, second, clock_=support.FROZEN, observer=None):
        run_dir = os.path.join(self.root, f"run-{first}")
        os.mkdir(run_dir)
        holds = {name: self.fifo(f"{name}-{first}") for name in ("a", "b")}
        rows = [self.good("a", "wait", holds["a"]), self.good("b", "wait", holds["b"])]
        h = self.start(rows, run_dir=run_dir, clock=clock_, observer=observer)
        support.release(holds[first])
        h.saw("end", first)
        support.release(holds[second])
        result = h.join()
        self.assertLess(h.seq("end", first), h.seq("end", second))
        return result, run_dir

    def test_frozen_clock_summaries_are_byte_identical_across_orders(self):
        # design-QA counter-example 25; the frozen clock leaves nothing to mask but the run's own directory.
        ab, ab_dir = self.run_in_order("a", "b")
        ba, ba_dir = self.run_in_order("b", "a")
        self.assertEqual(support.masked(ab.summary, ab_dir).encode(), support.masked(ba.summary, ba_dir).encode())
        for rel in ("summary.md", "run.json", "records/lint/a.json", "records/lint/b.json"):
            with open(os.path.join(ab_dir, rel), "rb") as f, open(os.path.join(ba_dir, rel), "rb") as g:
                mine, theirs = f.read(), g.read()
            if rel == "summary.md":
                mine = support.masked(mine.decode(), ab_dir).encode()
                theirs = support.masked(theirs.decode(), ba_dir).encode()
            self.assertEqual(mine, theirs, rel)

    def test_the_summary_lists_rows_in_registry_order_whatever_finished_first(self):
        result, _ = self.run_in_order("b", "a")
        steps = [x.split(" | ")[1].strip() for x in result.summary.splitlines()[1:4]]
        self.assertEqual(steps, ["parity", "a", "b"])

    def test_scripted_per_row_times_survive_either_order(self):
        # design-QA counter-example 25: A 1500 ms and B 150 ms in both orders.
        for first, second in (("a", "b"), ("b", "a")):
            with self.subTest(first=first):
                script = ScriptedClock({"parity": (0, 0), "a": (0, 1500), "b": (0, 150)})
                result, _ = self.run_in_order(first, second, script.clock, script.observe)
                table = result.summary.splitlines()
                self.assertEqual(table[2].split(" | ")[3].strip(), "1.5s")
                self.assertEqual(table[3].split(" | ")[3].strip(), "0.1s")
                # By hand: wall 1500, summed 1650; (1650 - 1500) * 100 // 1650 = 9.
                self.assertEqual(table[-3], "wall 1.5s · summed 1.6s · saved 9%")


class ScriptedClock:
    """A per-row clock: the scheduler's trace says which row is being stamped (design note §4)."""

    def __init__(self, times):
        self.times = times  # row -> (start ms, duration ms)
        self.ns = 0
        self.clock = clock.Clock(wall_ms=lambda: 0, mono_ns=lambda: self.ns)

    def observe(self, entry):
        start, duration = self.times.get(entry["id"], (0, 0))
        if entry["event"] == "start":
            self.ns = start * clock.NS_PER_MS
        elif entry["event"] == "end":
            self.ns = (start + duration) * clock.NS_PER_MS


class Footer(support.GateTest):
    def footer_of(self, times, rows):
        script = ScriptedClock(times)
        result = self.run_rows(rows, clock=script.clock, observer=script.observe)
        return result.summary.splitlines()[-3], result

    def test_overlapping_rows_save_time(self):
        # design-QA counter-example 26: 150 ms and 150 ms inside a 200 ms span.
        script = ScriptedClock({"parity": (0, 0), "a": (0, 150), "b": (50, 150)})
        result = self.run_rows([self.row("a"), self.row("b")], clock=script.clock, observer=script.observe)
        lines = result.summary.splitlines()
        self.assertEqual(lines[2].split(" | ")[3].strip(), "0.1s")
        self.assertEqual(lines[3].split(" | ")[3].strip(), "0.1s")
        self.assertEqual(lines[-3], "wall 0.2s · summed 0.3s · saved 33%")

    def test_a_gap_gives_a_negative_saving(self):
        # design-QA counter-example 26: 1000 ms of work in a 1155 ms span. Floor division gives -16.
        line, _ = self.footer_of({"parity": (0, 0), "a": (155, 1000)}, [self.row("a")])
        self.assertEqual(line, "wall 1.1s · summed 1.0s · saved -16%")

    def test_the_run_json_totals_match(self):
        self.footer_of({"parity": (0, 100), "a": (100, 400)}, [self.row("a")])
        with open(os.path.join(self.run_dir, "run.json"), "rb") as f:
            run = json.loads(f.read())
        self.assertEqual((run["wall_ms"], run["summed_ms"]), (500, 500))

    def test_wall_and_started_times_come_from_their_own_clocks(self):
        # Records carry the wall clock; durations carry the monotonic one (design note §4).
        script = ScriptedClock({"parity": (0, 0), "a": (0, 250)})
        wall = iter(range(1000, 2000, 10))
        split = clock.Clock(wall_ms=lambda: next(wall), mono_ns=script.clock.mono_ns)
        self.run_rows([self.row("a")], clock=split, observer=script.observe)
        rec = self.record("lint", "a")
        self.assertEqual(rec["duration_ms"], 250)
        self.assertGreater(rec["started_ms"], 999)
        self.assertGreater(rec["ended_ms"], rec["started_ms"])


class Ceiling(support.GateTest):
    def test_the_ceiling_is_the_sum_of_the_caps_plus_sixty(self):
        h = self.start([self.row("a", timeout_s=10), self.row("b", timeout_s=20)])
        self.assertEqual(self.timers.expect("ceiling").delay_s, 90)
        h.join()

    def test_only_selected_rows_count(self):
        runner = self.runner([self.row("a", timeout_s=10)])
        self.assertEqual(runner.ceiling_s, 70)


class Trace(support.GateTest):
    def test_the_trace_uses_the_schedulers_own_sequence_numbers(self):
        self.run_rows([self.row("a")])
        with open(os.path.join(self.run_dir, "trace.json"), "rb") as f:
            trace = json.loads(f.read())
        self.assertEqual([e["seq"] for e in trace], list(range(len(trace))))
        self.assertEqual([(e["event"], e["id"]) for e in trace],
                         [("start", "parity"), ("end", "parity"), ("start", "a"), ("end", "a")])  # fmt: skip

    def test_a_parity_failure_leaves_only_parity_started(self):
        self.toolchain.reasons = ["go: version: want a, got b"]
        self.run_rows([self.row("a")])
        with open(os.path.join(self.run_dir, "trace.json"), "rb") as f:
            trace = json.loads(f.read())
        self.assertEqual([e["id"] for e in trace if e["event"] == "start"], ["parity"])
        self.assertEqual([e["id"] for e in trace if e["event"] == "not-run"], ["a"])


class Crash(support.GateTest):
    def test_a_worker_crash_is_an_internal_error_never_a_hang(self):
        # A bug in the executor must surface as exit 2, not as a run that waits forever.
        def boom(*args, **kwargs):
            raise RuntimeError("planted executor bug")

        with mock.patch.object(scheduler.step, "execute", boom), mock.patch("threading.excepthook"):
            result = self.run_rows([self.row("a")])
        self.assertEqual(result.exit_code, 2)
        self.assertEqual(result.summary.splitlines()[-2:], ["GATE FAILED", "exit: 2"])
        self.assertIn("planted executor bug", self.reason("a"))

    def test_a_parity_crash_is_an_internal_error(self):
        def boom(env, *, full_tree):
            raise RuntimeError("planted parity bug")

        self.toolchain.parity = boom
        with mock.patch("threading.excepthook"):
            result = self.run_rows([self.row("a")])
        self.assertEqual(result.exit_code, 2)
        self.assertIn("planted parity bug", self.reason("parity"))
