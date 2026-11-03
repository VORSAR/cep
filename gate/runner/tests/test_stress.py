"""gate/runner/stress.py (design note §9; QA DQ18; design-QA counter-examples 30, 31)."""

import json
import os
import subprocess
import sys

from gate.runner import stress
from gate.runner.tests import support

PLANTS = "gate.runner.testdata.stressplant"
STRESS = os.path.join(support.GATE, "runner", "stress.py")


class Harness(support.GateTest):
    def harness(self, *modules, runs=50):
        out_dir = os.path.join(self.root, "out")
        argv = [sys.executable, "-I", "-B", "-X", "utf8", STRESS, "--no-hogs", "--runs", str(runs), "--parallel", "4",
                "--out", out_dir, *modules]  # fmt: skip
        code, out, err = support.run_tool(argv, support.REPO)
        self.assertEqual(err, b"", err.decode(errors="replace"))
        counts = {}
        for line in out.decode().splitlines():
            parts = line.split()
            if len(parts) == 2 and "/" in parts[0]:
                counts[parts[1]] = int(parts[0].split("/")[0])
        return code, counts, out.decode(), out_dir

    def test_an_order_and_a_hash_dependent_test_score_below_50_of_50(self):
        # design-QA counter-example 31, with a clean control beside them. Each planted test must also pass
        # sometimes, or it would prove the harness broken rather than the test flaky.
        code, counts, out, _ = self.harness(f"{PLANTS}.plant_order", f"{PLANTS}.plant_hash", f"{PLANTS}.plant_clean")
        order = counts[f"{PLANTS}.plant_order.Order.test_needs_first"]
        seed = counts[f"{PLANTS}.plant_hash.Hash.test_even"]
        self.assertTrue(0 < order < 50, order)
        self.assertTrue(0 < seed < 50, seed)
        self.assertEqual(counts[f"{PLANTS}.plant_order.Order.test_first"], 50)
        self.assertEqual(counts[f"{PLANTS}.plant_clean.Clean.test_one"], 50)
        self.assertEqual(counts[f"{PLANTS}.plant_clean.Clean.test_two"], 50)
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "stress: 5 tests, 2 below 50/50")

    def test_a_test_below_the_line_shows_why(self):
        # A count alone cannot be fixed at its root: each failing test's first failure is printed, and every
        # traceback is kept in the run's file.
        _, _, out, out_dir = self.harness(f"{PLANTS}.plant_order", runs=4)
        test = f"{PLANTS}.plant_order.Order.test_needs_first"
        lines = [line for line in out.splitlines() if line.startswith(f"why {test}: run ")]
        self.assertEqual(len(lines), 1, out)
        self.assertIn("AssertionError: [] is not true", lines[0])
        seed = int(lines[0].split(": run ")[1].split(":")[0])
        with open(os.path.join(out_dir, f"run-{seed}.json"), "rb") as f:
            self.assertIn("Traceback", json.loads(f.read())["failures"][test])

    def test_a_clean_module_exits_0(self):
        code, counts, out, _ = self.harness(f"{PLANTS}.plant_clean", runs=3)
        self.assertEqual(code, 0, out)
        self.assertEqual(sorted(counts.values()), [3, 3])
        self.assertEqual(out.splitlines()[-1], "stress: 2 tests, 0 below 3/3")

    def test_each_run_prints_its_seed(self):
        _, _, out, _ = self.harness(f"{PLANTS}.plant_clean", runs=2)
        self.assertIn("run 0: seed 0", out)
        self.assertIn("run 1: seed 1", out)

    def test_a_leak_a_skip_and_an_expected_failure_are_not_passes(self):
        _, counts, _, _ = self.harness(f"{PLANTS}.plant_vacuous", runs=2)
        self.assertEqual(sorted(counts.values()), [0, 0, 0])

    def test_a_run_that_dies_is_reported_and_fails(self):
        code, _, out, _ = self.harness(f"{PLANTS}.plant_crash", f"{PLANTS}.plant_clean", runs=2)
        self.assertEqual(code, 1, out)
        self.assertIn("run 0: died", out)

    def test_zero_tests_is_a_failure(self):
        code, _, out, _ = self.harness("gate.runner.testdata.stressplant", runs=2)
        self.assertEqual(code, 1, out)
        self.assertIn("zero is a failure", out)


class Child(support.GateTest):
    def child(self, seed, *modules):
        out = os.path.join(self.root, f"out-{seed}-{self.counter}.json")
        self.counter += 1
        env = stress.scrubbed(support.base_env(), seed)
        argv = [*stress.child_argv(sys.executable, STRESS), str(seed), out, support.REPO, *modules]
        with open(os.devnull, "wb") as null:
            subprocess.run(argv, env=env, cwd=self.root, stdin=subprocess.DEVNULL, stdout=null, stderr=null, check=True)
        with open(out, "rb") as f:
            return json.loads(f.read())

    def test_the_same_seed_gives_the_same_hash(self):
        # design-QA counter-example 30: under -I the seed is ignored and this differs.
        module = f"{PLANTS}.plant_clean"
        self.assertEqual(self.child(7, module)["hash_probe"], self.child(7, module)["hash_probe"])

    def test_different_seeds_give_different_hashes(self):
        module = f"{PLANTS}.plant_clean"
        self.assertGreater(len({self.child(seed, module)["hash_probe"] for seed in range(4)}), 1)

    def test_the_flags_are_the_design_notes(self):
        flags = self.child(0, f"{PLANTS}.plant_clean")["flags"]
        self.assertEqual(flags, {"isolated": 0, "ignore_environment": 0, "no_user_site": 1, "safe_path": True,
                                 "dont_write_bytecode": 1, "dev_mode": True, "warn_error": True})  # fmt: skip

    def test_the_shuffle_is_seeded_and_crosses_modules(self):
        modules = (f"{PLANTS}.plant_clean", f"{PLANTS}.plant_order")
        orders = {seed: self.child(seed, *modules)["order"] for seed in range(8)}
        self.assertEqual(orders[0], self.child(0, *modules)["order"])
        self.assertGreater(len({tuple(o) for o in orders.values()}), 1)
        names = sorted(orders[0])
        self.assertEqual(sorted(orders[1]), names)

        def crossings(order):
            kinds = [name.split(".")[4] for name in order]
            return sum(1 for a, b in zip(kinds, kinds[1:], strict=False) if a != b)

        self.assertGreater(max(crossings(o) for o in orders.values()), 1)


class Stopping(support.GateTest):
    def test_a_run_dequeued_after_the_stop_does_not_start(self):
        # A worker can take the next run before the stop's shutdown cancels it (seen under T4's load, 2026-11-04):
        # a run that starts after the stop returns at once, running nothing.
        stress.stopping.set()
        self.addCleanup(stress.stopping.clear)
        out_dir = os.path.join(self.root, "out")
        os.mkdir(out_dir)
        self.assertIsNone(stress._one_run(0, STRESS, support.REPO, [PLANTS], out_dir))
        self.assertEqual(os.listdir(out_dir), [])


class Environment(support.GateTest):
    def test_scrubbed_keeps_only_the_allow_list_and_sets_the_seed(self):
        base = {"PATH": "/p", "HOME": "/h", "PYTHONPATH": "/evil", "PYTHONHOME": "/evil", "GOFLAGS": "-x", "X": "1",
                "VORSAR_CEP_TOOLS_CACHE": "/c", "GOCACHE": "/g"}  # fmt: skip
        self.assertEqual(stress.scrubbed(base, 7), {"PATH": "/p", "HOME": "/h", "VORSAR_CEP_TOOLS_CACHE": "/c",
                                                    "GOCACHE": "/g", "PYTHONHASHSEED": "7"})  # fmt: skip

    def test_no_child_flag_reads_isolated(self):
        argv = stress.child_argv("py", "s.py")
        self.assertEqual(argv, ["py", "-s", "-B", "-P", "-X", "dev", "-W", "error", "s.py", "--child"])


class Hogs(support.GateTest):
    def test_hogs_run_inside_the_block_and_are_gone_after_it(self):
        with stress.hogs(2, sys.executable) as procs:
            self.assertEqual(len(procs), 2)
            self.assertEqual([p.poll() for p in procs], [None, None])
        self.assertEqual([p.returncode for p in procs], [-9, -9])

    def test_the_default_is_twice_the_cores(self):
        self.assertEqual(stress.hog_count(lambda: 6), 12)
        self.assertEqual(stress.hog_count(lambda: None), 2)
