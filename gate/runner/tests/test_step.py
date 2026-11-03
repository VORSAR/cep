"""One step, end to end through real processes: each verdict slot, the environment, the row's files."""

import hashlib
import json
import os
import shutil
import signal
import sys

from gate.runner import records
from gate.runner.tests import support


class Slots(support.GateTest):
    """Design note §4's precedence, one real row per slot. Slot 2 is in test_interrupts."""

    def test_slot_1_parity_failure_stops_every_row_from_starting(self):
        marker = os.path.join(self.root, "marker")
        self.toolchain.reasons = ["go: version: want a, got b"]
        rows = [self.row("x", "envfile", "HOME", marker), self.row("y", "envfile", "HOME", marker)]
        result = self.run_rows(rows)
        self.assertEqual(result.exit_code, 1)
        self.assertEqual(self.reason("parity"), "go: version: want a, got b")
        self.assertEqual(self.reason("x"), "not run: parity failed")
        self.assertEqual(self.reason("y"), "not run: parity failed")
        self.assertFalse(os.path.exists(marker))

    def test_slot_1_lists_every_parity_reason_in_order(self):
        self.toolchain.reasons = ["python: a", "go: b", "ruff: c"]
        self.run_rows([self.row("x")])
        self.assertEqual(self.reason("parity"), "python: a; go: b; ruff: c")

    def test_slot_1_keeps_every_parity_reason_whole_in_the_record(self):
        # review BA5: two version reasons came to 311 characters, and the record cut the second one short.
        reasons = [f"{tool}: version: want {'a' * 70}, got {'b' * 70}: run make clean-tools" for tool in ("go", "ruff")]
        self.toolchain.reasons = reasons
        result = self.run_rows([self.row("x")])
        joined = "; ".join(reasons)
        self.assertGreater(len(joined), records.REASON_MAX)
        self.assertEqual(self.reason("parity"), joined)
        line = next(x for x in result.summary.splitlines() if " parity " in x)
        self.assertEqual(line.split(" | ", 4)[4], joined[: records.REASON_MAX - 1] + "…")

    def test_slot_3_timeout(self):
        h = self.start([self.row("x", "block", timeout_s=7)])
        self.timers.expect("watchdog:lint/x").fire()
        result = h.join()
        self.assertEqual(self.reason("x"), "timeout after 7s")
        self.assertEqual(result.exit_code, 1)

    def test_slot_3_a_child_ignoring_term_is_killed_after_the_grace_period(self):
        w = self.witness("start")
        h = self.start([self.row("x", "term", w.path)])
        w.started(1)
        self.timers.expect("watchdog:lint/x").fire()
        w.more(1)  # the row saw TERM and carried on: only now does the grace timer's KILL matter
        grace = self.timers.expect("grace:lint/x")
        self.assertEqual(grace.delay_s, 5)
        self.assertEqual(self.record_absent("x"), True)
        grace.fire()
        h.join()
        self.assertEqual(self.reason("x"), "timeout after 60s")
        w.gone()

    def record_absent(self, row_id):
        return not os.path.exists(os.path.join(self.run_dir, "records", "lint", f"{row_id}.json"))

    def test_slot_3_a_row_timing_out_while_its_scope_expands_never_starts(self):
        # The scope tool here is a fake `git` that holds; the row's own command must then never run.
        w = self.witness("start")
        bindir = os.path.join(self.root, "bin")
        self.script(
            "git",
            bindir,
            f"import os\nos.write(os.open({w.path!r}, os.O_WRONLY), b's')\nimport signal\nsignal.pause()\n",
        )
        self.toolchain.dirs = [bindir]
        marker = os.path.join(self.root, "marker")
        h = self.start([self.row("x", "envfile", "HOME", marker)])
        w.started(1)
        self.timers.expect("watchdog:lint/x").fire()
        h.join()
        self.assertEqual(self.reason("x"), "timeout after 60s")
        self.assertFalse(os.path.exists(marker))
        w.gone()

    def test_slot_4_could_not_start(self):
        self.run_rows([self.row("x", command=("no-such-command-xyz",))])
        self.assertRegex(self.reason("x"), r"^could not start: .*no-such-command-xyz")

    def test_slot_5_scope_error_runs_nothing(self):
        marker = os.path.join(self.root, "marker")
        self.run_rows([self.row("x", "envfile", "HOME", marker, expected_scope=("git", ":(badmagic)x"))])
        self.assertRegex(self.reason("x"), r"^scope error: git ls-files exit 128")
        self.assertFalse(os.path.exists(marker))

    def test_slot_5_zero_expected_is_a_failure(self):
        # plan check case 12: at T2 an empty expansion fails, whatever the command would print.
        self.run_rows([self.row("x", expected_scope=("git", "*.nothing"))])
        self.assertIn("zero is a failure", self.reason("x"))
        self.assertTrue(self.reason("x").startswith("scope error: "))

    def leaver(self, w, *leader):
        """A leader that exits only once its grandchild holds the witness, and stdout, and will never exit."""
        ready = self.fifo("ready")
        return self.good("x", *leader, "spawn", 5, "start", w.path, "start", ready, "block", "wait", ready)

    def test_slot_6_left_running_after_exit_0(self):
        # design-QA counter-example 18, plan check C4: the count is right and the leader exits 0.
        w = self.witness("start")
        h = self.start([self.leaver(w)])
        w.started(1)
        h.join()
        self.assertEqual(self.reason("x"), "left running processes")
        w.gone()

    def test_slot_6_beats_the_leaders_exit_1(self):
        w = self.witness("start")
        h = self.start([self.leaver(w, "status", 1)])
        w.started(1)
        h.join()
        self.assertEqual(self.reason("x"), "left running processes")
        self.assertEqual(self.record("lint", "x")["exit_code"], 1)
        w.gone()

    def test_slot_7_exit(self):
        self.run_rows([self.good("x", "status", 3)])
        self.assertEqual(self.reason("x"), "exit 3")
        self.assertEqual(self.record("lint", "x")["exit_code"], 3)

    def test_slot_7_names_the_rows_first_failure_line(self):
        # owner, 2026-11-03 (review BA3): the wrappers' `failed: ` line, coverage's `FAIL ` line, a crash's own line.
        for i, (printed, want) in enumerate((
            (("note", "failed: a.go: run gofmt -w a.go", "failed: b.go: run gofmt -w b.go"), "a.go: run gofmt -w a.go"),
            (("PASS ex.com/m/a 100.00% (2/2)", "FAIL ex.com/m/b 50.00% (1/2) < 90%"), "ex.com/m/b 50.00% (1/2) < 90%"),
            (("internal error: KeyError: 'x'", "GATE FAILED"), "internal error: KeyError: 'x'"),
            (("failed:no space", "FAILED", "fail: lower case"), ""),
        )):  # fmt: skip
            with self.subTest(printed):
                self.run_dir = os.path.join(self.root, f"run-{i}")
                os.mkdir(self.run_dir)
                actions = [x for line in printed for x in ("print", line)]
                self.run_rows([self.good("x", *actions, "status", 1)])
                self.assertEqual(self.reason("x"), f"exit 1: {want}" if want else "exit 1")

    def test_slot_7_killed_by_signal(self):
        self.run_rows([self.row("x", "killself", signal.SIGKILL)])
        self.assertEqual(self.reason("x"), "killed by signal 9")
        self.assertEqual(self.record("lint", "x")["exit_code"], -9)

    def test_slot_8_each_covered_problem(self):
        cases = (
            ("no-line", ("print", "hello"), "GATE_COVERED missing"),
            ("twice", ("print", "GATE_COVERED 1", "print", "GATE_COVERED 1"), "GATE_COVERED duplicated"),
            ("bad", ("print", "GATE_COVERED 03"), "GATE_COVERED malformed"),
            ("zero", ("print", "GATE_COVERED 0"), "GATE_COVERED 0"),
            ("cr", ("printhex", b"GATE_COVERED 1\r\n".hex()), "GATE_COVERED malformed"),
            ("nel", ("printhex", b"noise\x85GATE_COVERED 1\n".hex()), "GATE_COVERED missing"),
        )
        self.run_rows([self.row(name, *actions) for name, actions, _ in cases])
        for name, _, want in cases:
            with self.subTest(name):
                self.assertEqual(self.reason(name), want)

    def test_slot_9_covered_differs_from_expected(self):
        # plan check case 6: naming both numbers.
        self.run_rows([self.row("x", "print", "GATE_COVERED 3")])
        self.assertEqual(self.reason("x"), "covered 3 ≠ expected 1")
        rec = self.record("lint", "x")
        self.assertEqual((rec["covered"], rec["expected"]), (3, 1))

    def test_slot_10_below_min_wants(self):
        self.run_rows([self.row("x", min_wants=2)])
        self.assertEqual(self.reason("x"), "covered 1 < min_wants 2")

    def test_slot_11_pass(self):
        result = self.run_rows([self.row("x")])
        rec = self.record("lint", "x")
        self.assertEqual((rec["verdict"], rec["reason"], rec["exit_code"]), ("pass", "covered 1", 0))
        self.assertEqual((rec["covered"], rec["expected"]), (1, 1))
        self.assertEqual(result.exit_code, 0)


class CollectAll(support.GateTest):
    def test_one_failure_never_stops_the_run(self):
        # plan check case 8.
        rows = [self.row("a"), self.good("b", "status", 1), self.row("c"), self.good("d", "status", 2)]
        result = self.run_rows(rows)
        self.assertEqual(result.exit_code, 1)
        self.assertEqual([self.reason(r) for r in "abcd"], ["covered 1", "exit 1", "covered 1", "exit 2"])
        lines = result.summary.splitlines()
        self.assertIn("exit 1", lines[3])
        self.assertIn("exit 2", lines[5])


class RowOutput(support.GateTest):
    def test_ten_megabytes_of_invalid_utf8(self):
        # plan check case 11: the record stays valid JSON and the reason stays one line.
        self.run_rows([self.row("x", "flood", 10_000_000, "status", 1)])
        self.assertEqual(self.reason("x"), "exit 1")
        self.assertPinnedJson(os.path.join(self.run_dir, "records", "lint", "x.json"))
        self.assertGreater(len(self.row_file("lint", "x", "stdout")), 10_000_000)

    def test_stdout_and_stderr_are_kept_apart(self):
        code = "import sys; print('GATE_COVERED 1'); print('e', file=sys.stderr)"
        self.run_rows([self.row("x", command=(sys.executable, "-I", "-c", code))])
        self.assertEqual(self.row_file("lint", "x", "stdout"), b"GATE_COVERED 1\n")
        self.assertEqual(self.row_file("lint", "x", "stderr"), b"e\n")

    def test_a_multi_line_error_reason_is_one_line(self):
        self.track("f.txt", "x\n")
        self.run_rows([self.row("x", expected_scope=("git", ":(badmagic)x"))])
        self.assertNotIn("\n", self.reason("x"))


class FilesList(support.GateTest):
    def test_the_token_becomes_a_nul_list_and_its_digest_is_recorded(self):
        self.track("sp ace.txt", "x\n")
        copy = os.path.join(self.root, "seen")
        code = "import sys, shutil; shutil.copy(sys.argv[1], sys.argv[2]); print('GATE_COVERED 2')"
        command = (sys.executable, "-I", "-c", code, "{files}", copy)
        self.run_rows([self.row("x", command=command, expected_scope=("git", "*.txt"))])
        want = b"one.txt\0sp ace.txt\0"
        with open(copy, "rb") as f:
            self.assertEqual(f.read(), want)
        self.assertEqual(self.record("lint", "x")["scope_sha256"], hashlib.sha256(want).hexdigest())
        self.assertEqual(self.row_file("lint", "x", "files.nul"), want)
        self.assertEqual(self.reason("x"), "covered 2")

    def test_the_go_kind_lists_import_paths(self):
        self.track("go.mod", "module ex.com/m\n\ngo 1.27.0\n")
        self.track("a/a.go", "package a\n")
        self.toolchain.dirs = [support.tool_bin("go")]
        self.run_rows([self.row("x", expected_scope=("go", "./..."))])
        self.assertEqual(self.row_file("lint", "x", "files.nul"), b"ex.com/m/a\0")


class Environment(support.GateTest):
    def seen(self, name, environ=None):
        path = os.path.join(self.root, f"env-{name}")
        over = {} if environ is None else {"environ": environ}
        self.run_rows([self.row("x", "envfile", name, path, "print", "GATE_COVERED 1")], **over)
        with open(path, encoding="utf-8") as f:
            return f.read()

    def test_the_row_variables(self):
        for name, want in (("GATE_TIER", "T2"), ("GATE_ROW", "x"), ("GATE_RUN_DIR", self.run_dir),
                           ("GATE_ROW_DIR", os.path.join(self.run_dir, "rows", "lint", "x"))):  # fmt: skip
            with self.subTest(name):
                self.assertEqual(self.seen(name), want)
                self.reset_run()

    def reset_run(self):
        shutil.rmtree(self.run_dir)
        os.mkdir(self.run_dir)

    def test_the_pinned_go_settings_win_over_the_caller(self):
        env = dict(support.base_env(), GOFLAGS="-mod=mod", GOTOOLCHAIN="go1.27.2", CGO_ENABLED="1", GOROOT="/nope",
                   PYTHONPATH="/evil", GIT_LITERAL_PATHSPECS="1")  # fmt: skip
        self.assertEqual(self.seen("GOFLAGS", env), "")
        self.reset_run()
        self.assertEqual(self.seen("GOTOOLCHAIN", env), "local")
        self.reset_run()
        self.assertEqual(self.seen("CGO_ENABLED", env), "0")
        self.reset_run()
        self.assertEqual(self.seen("GOROOT", env), "<unset>")
        self.reset_run()
        self.assertEqual(self.seen("PYTHONPATH", env), "<unset>")
        self.reset_run()
        self.assertEqual(self.seen("GIT_LITERAL_PATHSPECS", env), "<unset>")

    def test_the_cache_bin_wins_over_a_decoy_on_the_inherited_path(self):
        # design-QA counter-example 40: a command resolves through the runner's PATH.
        cache, decoy = os.path.join(self.root, "cache"), os.path.join(self.root, "decoy")
        marker = os.path.join(self.root, "decoy-ran")
        self.script("tool", cache, "print('GATE_COVERED 1')\n")
        self.script("tool", decoy, f"open({marker!r}, 'w').close()\nprint('GATE_COVERED 1')\n")
        self.toolchain.dirs = [cache]
        env = dict(support.base_env(), PATH=f"{decoy}:{support.base_env()['PATH']}")
        self.run_rows([self.row("x", command=("tool",))], environ=env)
        self.assertEqual(self.reason("x"), "covered 1")
        self.assertFalse(os.path.exists(marker))

    def test_a_row_runs_in_the_repository(self):
        code = "import os; print(f'GATE_COVERED {len(os.listdir(\".git\")) > 0:d}')"
        self.run_rows([self.row("x", command=(sys.executable, "-I", "-c", code))])
        self.assertEqual(self.reason("x"), "covered 1")

    def test_parity_gets_the_clean_environment_without_row_variables(self):
        env = dict(support.base_env(), GOTOOLCHAIN="go1.27.2", PYTHONPATH="/evil")
        self.run_rows([self.row("x")], environ=env)
        seen, _ = self.toolchain.calls[0]
        self.assertEqual(seen["GOTOOLCHAIN"], "local")
        self.assertFalse("PYTHONPATH" in seen or "GATE_ROW" in seen)  # not assertNotIn: it would print the environment
        self.assertTrue(seen["PATH"].startswith(self.toolchain.dirs[0]))


class TreeDigest(support.GateTest):
    def test_full_tree_only_at_t2_and_above(self):
        # design note §3, the owner's G1 decision.
        for tier, want in (("T0", False), ("T1", False), ("T2", True), ("T3", True), ("T4", True)):
            with self.subTest(tier):
                self.toolchain.calls.clear()
                run_dir = os.path.join(self.root, f"run-{tier}")
                os.mkdir(run_dir)
                self.run_rows([self.row("x", tiers=(tier,))], tier=tier, run_dir=run_dir)
                self.assertEqual(self.toolchain.calls[0][1], want)


class RunFiles(support.GateTest):
    def test_run_json_holds_the_records_in_registry_order_with_the_totals(self):
        self.run_rows([self.row("b", job="test"), self.row("a", job="lint")])
        with open(os.path.join(self.run_dir, "run.json"), "rb") as f:
            run = json.loads(f.read())
        self.assertEqual([(r["job"], r["id"]) for r in run["records"]],
                         [("test", "parity"), ("lint", "parity"), ("test", "b"), ("lint", "a")])  # fmt: skip
        self.assertEqual((run["wall_ms"], run["summed_ms"]), (0, 0))
        self.assertEqual((run["schema"], run["run_id"], run["tier"]), (1, support.RUN_ID, "T2"))

    def test_parity_has_one_record_per_job_and_one_summary_line(self):
        # design note §4: labelled `*` in an all-jobs run.
        result = self.run_rows([self.row("a", job="lint"), self.row("b", job="test"), self.row("c", job="lint")])
        self.assertEqual(self.record("lint", "parity")["job"], "lint")
        self.assertEqual(self.record("test", "parity")["job"], "test")
        lines = result.summary.splitlines()
        self.assertEqual(lines[1].split(" | ")[:2], ["*   ", "parity"])
        self.assertEqual(sum(1 for line in lines if "parity" in line), 1)

    def test_a_single_job_run_labels_parity_with_its_job(self):
        result = self.run_rows([self.row("a", job="lint")], job="lint")
        self.assertEqual(result.summary.splitlines()[1].split(" | ")[0].strip(), "lint")

    def test_parity_counts_its_tools(self):
        self.run_rows([self.row("a")])
        rec = self.record("lint", "parity")
        self.assertEqual((rec["covered"], rec["expected"], rec["exit_code"]), (5, 5, 0))

    def test_every_file_in_pinned_json_and_the_summary_on_disk(self):
        result = self.run_rows([self.row("a")])
        for rel in ("run.json", "trace.json", "records/lint/a.json", "records/lint/parity.json"):
            self.assertPinnedJson(os.path.join(self.run_dir, rel))
        with open(os.path.join(self.run_dir, "summary.md"), encoding="utf-8") as f:
            self.assertEqual(f.read(), result.summary)

    def test_no_temporary_file_survives(self):
        self.run_rows([self.row("a")])
        names = [n for _, _, files in os.walk(self.run_dir) for n in files]
        self.assertEqual([n for n in names if n.endswith(".tmp")], [])

    def test_the_record_carries_the_run_identity(self):
        self.run_rows([self.row("a")])
        rec = self.record("lint", "a")
        self.assertEqual((rec["run_id"], rec["commit"], rec["tier"]), (support.RUN_ID, support.COMMIT, "T2"))

    def test_go_env_goes_into_the_run_directory_once(self):
        self.toolchain.dirs = [support.tool_bin("go")]
        self.run_rows([self.row("a"), self.row("b")], record_go_env=True)
        with open(os.path.join(self.run_dir, "go-env.json"), "rb") as f:
            env = json.loads(f.read())
        self.assertEqual(env["GOTOOLCHAIN"], "local")
        self.assertEqual(env["CGO_ENABLED"], "0")

    def test_a_go_env_that_cannot_start_is_noted_and_is_never_a_verdict(self):
        # Parity decides whether `go` is good; this record only describes the run.
        bindir = os.path.join(self.root, "bin")
        os.mkdir(bindir)
        os.symlink(shutil.which("git"), os.path.join(bindir, "git"))
        self.toolchain.dirs = [bindir]
        result = self.run_rows([self.row("a")], environ=dict(support.base_env(), PATH=""), record_go_env=True)
        self.assertEqual(result.exit_code, 0)
        with open(os.path.join(self.run_dir, "go-env.stderr"), encoding="utf-8") as f:
            self.assertIn("go env", f.read())
