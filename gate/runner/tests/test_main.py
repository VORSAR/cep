"""The CLI (plan check S4; design note §4): exit codes, the last two lines, the run directory, parity's wiring."""

import contextlib
import hashlib
import io
import json
import os
import re
import shutil
import sys
from unittest import mock

from gate.runner import main
from gate.runner.tests import support


class Cli(support.GateTest):
    def cli(self, *argv, **deps):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = main.main(list(argv), deps=self.deps(**deps))
        self.output = out.getvalue()
        return code

    def record_at(self, rel, job, row_id):
        with open(os.path.join(self.repo, rel, "records", job, f"{row_id}.json"), "rb") as f:
            return json.loads(f.read())

    def assertRefused(self, code, needle):
        self.assertEqual(code, 2, self.output)
        self.assertEqual(self.output.splitlines()[-2:], ["GATE FAILED", "exit: 2"], self.output)
        self.assertIn(needle, self.output)

    def reg(self, *rows):
        return self.registry_file(list(rows) or [self.row("x")])

    def test_a_passing_run_ends_with_the_contract_lines(self):
        code = self.cli("--tier", "T2", "--registry", self.reg(), "--run-dir", self.run_dir)
        self.assertEqual(code, 0)
        self.assertEqual(self.output.splitlines()[-2:], ["GATE PASSED", "exit: 0"])

    def test_a_failing_run_ends_with_the_contract_lines(self):
        reg = self.reg(self.good("x", "status", 1))
        code = self.cli("--tier", "T2", "--registry", reg, "--run-dir", self.run_dir)
        self.assertEqual(code, 1)
        self.assertEqual(self.output.splitlines()[-2:], ["GATE FAILED", "exit: 1"])

    def test_stdout_is_exactly_the_summary_file(self):
        self.cli("--tier", "T2", "--registry", self.reg(), "--run-dir", self.run_dir)
        with open(os.path.join(self.run_dir, "summary.md"), encoding="utf-8") as f:
            self.assertEqual(self.output, f.read())

    def run_json(self) -> dict:
        with open(os.path.join(self.run_dir, "run.json"), "rb") as f:
            return json.loads(f.read())

    def test_a_registry_other_than_the_default_is_named_and_recorded(self):
        # review BA8: an "exit: 0" from a fixture registry must not pass for a run of the repository's law.
        reg = self.reg()
        self.cli("--tier", "T2", "--registry", reg, "--run-dir", self.run_dir)
        self.assertEqual(self.output.splitlines()[-5:-3], [f"registry: {reg}", f"run: {self.run_dir}"])
        with open(reg, "rb") as f:
            self.assertEqual(self.run_json()["registry_sha256"], hashlib.sha256(f.read()).hexdigest())

    def test_the_default_registry_is_recorded_and_not_named(self):
        default = os.path.join(self.repo, "gate", "registry.json")
        os.mkdir(os.path.dirname(default))
        shutil.copyfile(self.reg(), default)
        for argv in ((), ("--registry", default)):
            with self.subTest(argv):
                run_dir = os.path.join(self.root, f"run-{len(argv)}")
                self.assertEqual(self.cli("--tier", "T2", *argv, "--run-dir", run_dir), 0, self.output)
                self.assertFalse([x for x in self.output.splitlines() if x.startswith("registry:")], self.output)
                with open(os.path.join(run_dir, "run.json"), "rb") as f, open(default, "rb") as g:
                    self.assertEqual(json.loads(f.read())["registry_sha256"], hashlib.sha256(g.read()).hexdigest())

    def test_usage_errors(self):
        reg = self.reg()
        for argv, needle in (
            ((), "--tier"),
            (("--tier", "T9", "--registry", reg), "invalid choice"),
            (("--tier", "T2", "--registry", reg, "--nonsense"), "nonsense"),
            (("--tier", "T2", "--registry", reg, "stray"), "stray"),
            (("summarize",), "usage"),
        ):
            with self.subTest(argv):
                self.assertRefused(self.cli(*argv), needle)

    def test_a_broken_registry_exits_2_before_anything_runs(self):
        path = os.path.join(self.root, "bad.json")
        with open(path, "w", encoding="utf-8") as f:
            f.write('{"schema": 1, "rows": [{"id": "x"}]}')
        self.assertRefused(self.cli("--tier", "T2", "--registry", path), "missing field")
        self.assertEqual(self.toolchain.calls, [])

    def test_a_missing_registry(self):
        self.assertRefused(self.cli("--tier", "T2", "--registry", os.path.join(self.root, "no.json")), "no.json")

    def test_the_default_registry_is_the_repositorys(self):
        self.assertRefused(self.cli("--tier", "T2"), os.path.join("gate", "registry.json"))

    def test_zero_rows_at_the_tier_is_refused(self):
        # plan check case 10.
        self.assertRefused(self.cli("--tier", "T4", "--registry", self.reg()), "zero")

    def test_an_unknown_job_is_refused(self):
        self.assertRefused(self.cli("--tier", "T2", "--job", "nope", "--registry", self.reg()), "nope")

    def test_bad_workers_are_refused(self):
        env = dict(support.base_env(), VORSAR_CEP_WORKERS="0")
        self.assertRefused(self.cli("--tier", "T2", "--registry", self.reg(), environ=env), "VORSAR_CEP_WORKERS")

    def test_workers_reach_the_scheduler(self):
        f1, f2 = self.fifo("f1"), self.fifo("f2")
        rows = (self.good("x", "openw", f1, "openr", f2), self.good("y", "openr", f1, "openw", f2))
        env = dict(support.base_env(), VORSAR_CEP_WORKERS="2")
        code = self.cli("--tier", "T2", "--registry", self.reg(*rows), "--run-dir", self.run_dir, environ=env)
        self.assertEqual(code, 0)

    def test_every_exception_is_exit_2_with_the_contract_lines(self):
        def boom(repo, environ):
            raise RuntimeError("planted")

        self.assertRefused(self.cli("--tier", "T2", "--registry", self.reg(), toolchain=boom), "planted")

    def test_a_scheduler_crash_is_exit_2(self):
        with mock.patch.object(main.scheduler.Runner, "run", side_effect=ValueError("planted")):
            self.assertRefused(self.cli("--tier", "T2", "--registry", self.reg(), "--run-dir", self.run_dir), "planted")

    def test_a_keyboard_interrupt_before_the_run_starts_is_an_interrupt_not_an_internal_error(self):
        # break-it QB8: a Ctrl-C before the scheduler's own handlers is the user's, so it reads as the run's would.
        def boom(repo, environ):
            raise KeyboardInterrupt

        code = self.cli("--tier", "T2", "--registry", self.reg(), toolchain=boom)
        self.assertEqual((code, self.output.splitlines()), (1, ["interrupted", "GATE FAILED", "exit: 1"]))

    def test_the_job_filter(self):
        rows = (self.row("a", job="lint"), self.row("b", job="test"))
        code = self.cli("--tier", "T2", "--job", "test", "--registry", self.reg(*rows), "--run-dir", self.run_dir)
        self.assertEqual(code, 0)
        self.assertTrue(os.path.exists(os.path.join(self.run_dir, "records", "test", "b.json")))
        self.assertFalse(os.path.exists(os.path.join(self.run_dir, "records", "lint")))

    def test_the_default_run_directory(self):
        code = self.cli("--tier", "T2", "--registry", self.reg())
        self.assertEqual(code, 0)
        path = os.path.join(self.repo, ".gate", "runs", support.RUN_ID, "run.json")
        self.assertPinnedJson(path)

    def test_the_summary_names_the_run_directory(self):
        # owner, 2026-11-03 (review BA3): inside the repository relative to it, elsewhere absolute.
        self.cli("--tier", "T2", "--registry", self.reg())
        self.assertIn(f"run: .gate/runs/{support.RUN_ID}", self.output.splitlines())
        self.cli("--tier", "T2", "--registry", self.reg(), "--run-dir", self.run_dir)
        self.assertIn(f"run: {self.run_dir}", self.output.splitlines())

    def test_a_real_run_id_is_made_from_the_wall_clock(self):
        code = self.cli("--tier", "T2", "--registry", self.reg(), new_run_id=main.clock.run_id)
        self.assertEqual(code, 0)
        names = os.listdir(os.path.join(self.repo, ".gate", "runs"))
        self.assertEqual(len(names), 1)
        self.assertRegex(names[0], r"^19700101T000000Z-[0-9a-f]{6}$")

    def test_a_used_run_directory_is_refused(self):
        with open(os.path.join(self.run_dir, "stale.json"), "w", encoding="utf-8") as f:
            f.write("{}")
        self.assertRefused(self.cli("--tier", "T2", "--registry", self.reg(), "--run-dir", self.run_dir), "not empty")

    def test_a_new_run_directory_is_made(self):
        target = os.path.join(self.root, "a", "b")
        self.assertEqual(self.cli("--tier", "T2", "--registry", self.reg(), "--run-dir", target), 0)
        self.assertPinnedJson(os.path.join(target, "run.json"))

    def test_commit_and_run_id_can_be_given(self):
        # gate design §12.3: a push names its sha; every CI job of one run shares one id.
        sha = "a" * 40
        code = self.cli("--tier", "T2", "--registry", self.reg(), "--commit", sha, "--run-id", "77-1")
        self.assertEqual(code, 0, self.output)
        self.assertIn("run: .gate/runs/77-1", self.output.splitlines())
        self.assertEqual(self.record_at(".gate/runs/77-1", "lint", "x")["commit"], sha)

    def test_a_bad_commit_or_run_id_is_refused(self):
        for argv, needle in ((("--commit", "abc"), "--commit"), (("--run-id", "a/b"), "--run-id")):
            with self.subTest(argv):
                self.assertRefused(self.cli("--tier", "T2", "--registry", self.reg(), *argv), needle)

    def test_t1_rows_run_at_t1(self):
        reg = self.reg(self.row("x", tiers=("T1", "T2")))
        self.assertEqual(self.cli("--tier", "T1", "--registry", reg, "--run-dir", self.run_dir), 0)
        self.assertFalse(self.toolchain.calls[0][1])


class FirstLine(support.GateTest):
    """The LAW delta block is the summary's first line, end to end (S1.1c-1 design 2.6; review QA-S2). The fixture is
    a git repository with a one-entry manifest (gate/law.json itself) and the real lawdelta row script."""

    LAW = os.path.join("gate", "law.json")
    SCRIPT = os.path.join(support.REPO, "gate", "runner", "rows", "lawdelta.py")
    IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@example.com")

    def setUp(self):
        super().setUp()
        self.git("checkout", "-q", "-b", "main")
        self.track(
            self.LAW, '{"schema": 1, "note": "a fixture", "paths": [{"path": "gate/law.json", "kind": "law"}]}\n'
        )
        self.git(*self.IDENTITY, "commit", "-q", "-m", "main")

    def rows(self, trivial_job="lint"):
        command = (sys.executable, "-I", "-B", "-X", "utf8", self.SCRIPT)
        law = self.row(
            "lawdelta", tiers=("T1", "T2", "T3", "T4"), command=command, expected_scope=("git", self.LAW), min_wants=1
        )
        return self.registry_file([self.row("trivial", tiers=("T0", "T1", "T2"), job=trivial_job), law])

    def first_line(self, *argv):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            main.main(
                [*argv, "--run-dir", os.path.join(self.root, f"run-{len(os.listdir(self.root))}")], deps=self.deps()
            )
        self.output = out.getvalue()
        return self.output.split("\n")[0]

    def test_t0_says_not_run_at_t0(self):
        self.assertEqual(
            self.first_line("--tier", "T0", "--registry", self.rows()),
            "LAW Δ: not run — the lawdelta row is not in this registry at T0",
        )

    def test_t1_and_t2_on_main_say_head_is_main(self):
        for tier in ("T1", "T2"):
            with self.subTest(tier):
                self.assertEqual(
                    self.first_line("--tier", tier, "--registry", self.rows()),
                    "LAW Δ: HEAD is main — read the PR run",
                )

    def test_a_branch_with_a_non_law_commit_says_none(self):
        self.git("checkout", "-q", "-b", "change")
        self.track("notes.txt", "not a law path\n")
        self.git(*self.IDENTITY, "commit", "-q", "-m", "change")
        self.assertEqual(self.first_line("--tier", "T2", "--registry", self.rows()), "LAW Δ: none")
        self.assertEqual(self.output.split("\n")[1], "")

    def test_a_job_without_the_row_says_not_run_at_the_job(self):
        line = self.first_line("--tier", "T2", "--job", "test", "--registry", self.rows(trivial_job="test"))
        self.assertEqual(line, "LAW Δ: not run — the lawdelta row is not in this registry at T2, job test")


class RealParts(support.GateTest):
    """The defaults main wires up, each run against the real thing it reads."""

    def test_a_worlds_git_runs_no_background_maintenance(self):
        # Forwarded item (i): git 2.54's commit starts `git maintenance run --auto --detach`, which outlives the
        # command and its group and writes into .git while the world is being removed. The world turns it off.
        for key, want in (("maintenance.auto", "false"), ("gc.auto", "0")):
            code, out, _ = support.run_tool(["git", "config", "--get", key], self.repo)
            self.assertEqual((code, out.strip()), (0, want.encode()), key)

    def test_the_commit_is_head(self):
        self.git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "x")
        want = self.git("rev-parse", "HEAD").decode().strip()
        got = main.read_commit(self.repo, support.base_env(), self.run_dir, "T2")
        self.assertEqual(got, want)
        self.assertTrue(re.fullmatch(r"[0-9a-f]{40}", got))

    def test_the_commit_at_t0_is_the_index(self):
        self.assertEqual(main.read_commit(self.repo, support.base_env(), self.run_dir, "T0"), "index")

    def test_a_repository_with_no_head_is_refused(self):
        with self.assertRaises(main.UsageError) as caught:
            main.read_commit(self.repo, support.base_env(), self.run_dir, "T2")
        self.assertIn("HEAD", str(caught.exception))

    def test_the_real_defaults_are_the_real_modules(self):
        deps = main.default_deps()
        self.assertIs(deps.clock, main.clock.REAL)
        self.assertIsInstance(deps.timers, main.clock.RealTimers)
        self.assertIs(deps.toolchain, main.RealToolchain)
        self.assertEqual(deps.repo, support.REPO)
        self.assertTrue(deps.record_go_env)

    def test_the_real_toolchain_calls_the_seam(self):
        # The seam of gate.toolchain, faked at its own boundary: the names and arguments Lane B's code must honour.
        calls = {}

        def check(tools, cache, env, *, full_tree):
            calls["check"] = (tools, cache, env, full_tree)
            return ["go: x"]

        with (
            mock.patch.object(main.manifest, "platform_key", return_value="p"),
            mock.patch.object(main.manifest, "load", return_value=["t1", "t2"]) as load,
            mock.patch.object(main.manifest, "cache_root", return_value="/c"),
            mock.patch.object(main.manifest, "path_dirs", return_value=["/c/t1/bin"]) as path_dirs,
            mock.patch.object(main.parity, "check", check),
        ):
            real = main.RealToolchain("/repo", {"A": "1"})
            self.assertEqual(real.path_dirs(), ["/c/t1/bin"])
            self.assertEqual(real.tool_count(), 2)
            self.assertEqual(real.parity({"E": "1"}, full_tree=True), ["go: x"])
        load.assert_called_once_with(os.path.join("/repo", "tools.json"), "p")
        path_dirs.assert_called_once_with("/c", ["t1", "t2"])
        self.assertEqual(calls["check"], (["t1", "t2"], "/c", {"E": "1"}, True))

    def test_a_manifest_error_is_exit_2(self):
        out = io.StringIO()

        def bad(repo, environ):
            raise main.manifest.ManifestError("no entry for this platform")

        with contextlib.redirect_stdout(out):
            code = main.main(["--tier", "T2", "--registry", self.registry_file([self.row("x")])],
                             deps=self.deps(toolchain=bad))  # fmt: skip
        self.assertEqual(code, 2)
        self.assertIn("no entry for this platform", out.getvalue())
        self.assertNotIn("internal error", out.getvalue())

    def test_the_run_json_is_valid(self):
        with contextlib.redirect_stdout(io.StringIO()):
            main.main(["--tier", "T2", "--registry", self.registry_file([self.row("x")]), "--run-dir", self.run_dir],
                      deps=self.deps())  # fmt: skip
        with open(os.path.join(self.run_dir, "run.json"), "rb") as f:
            self.assertEqual(json.loads(f.read())["run_id"], support.RUN_ID)
