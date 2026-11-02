"""The CLI (plan check S4; design note §4): exit codes, the last two lines, the run directory, parity's wiring."""

import contextlib
import io
import json
import os
import re
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

    def test_usage_errors(self):
        reg = self.reg()
        for argv, needle in (
            ((), "--tier"),
            (("--tier", "T9", "--registry", reg), "T9"),
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

    def test_a_keyboard_interrupt_before_the_run_starts_is_exit_2(self):
        def boom(repo, environ):
            raise KeyboardInterrupt

        self.assertRefused(self.cli("--tier", "T2", "--registry", self.reg(), toolchain=boom), "KeyboardInterrupt")

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

    def test_t1_rows_run_at_t1(self):
        reg = self.reg(self.row("x", tiers=("T1", "T2")))
        self.assertEqual(self.cli("--tier", "T1", "--registry", reg, "--run-dir", self.run_dir), 0)
        self.assertFalse(self.toolchain.calls[0][1])


class RealParts(support.GateTest):
    """The defaults main wires up, each run against the real thing it reads."""

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
