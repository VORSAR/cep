"""The proofs row (design note §5, §8; QA DQ4, C38): each driver in its own process, and at least ten of them.

The row counts the drivers that passed. Its expected scope is a live `git` expansion, so deleting a driver lowers
both sides alike; min_wants 10 is what turns that red, "covered 9 < min_wants 10". Stand-in drivers keep this fast:
the real ten run only in the proofs row itself. The row is T4-only, so this test runs the runner at T4; its stand-ins
never start a runner of their own.
"""

import os

from gate.proofs.drivers import support

GOOD = "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_a(self):\n        self.assertTrue(True)\n"
BAD = GOOD.replace("assertTrue(True)", "fail('planted')")


class ProofsRow(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        self.world.copy("gate/proofs/__init__.py")
        self.world.copy("gate/proofs/row.py")
        self.world.write("gate/proofs/drivers/__init__.py", "")
        for i in range(10):
            self.world.write(f"gate/proofs/drivers/test_d{i:02}.py", GOOD)
        self.world.commit()

    def run_proofs(self) -> support.Run:
        return self.world.run(registry=self.world.registry(support.PROOFS_ROW), tier="T4")

    def test_ten_passing_drivers_pass(self):
        run = self.run_proofs()
        self.assertParityPassed(run, "proofs")
        self.assertRow(run, "proofs", "proofs", True, "covered 10")
        self.assertEqual((run.tail(), run.code), (["GATE PASSED", "exit: 0"], 0))

    def test_deleting_one_driver_is_red(self):
        # C38 and QA DQ4: expected and covered both fall to 9, so only the pinned minimum can see it.
        self.world.git("rm", "-q", "--", "gate/proofs/drivers/test_d07.py")
        run = self.run_proofs()
        self.assertParityPassed(run, "proofs")
        self.assertRed(run, "proofs", "proofs", "covered 9 < min_wants 10")
        record = run.record("proofs", "proofs")
        self.assertEqual((record["covered"], record["expected"]), (9, 9))

    def test_a_failing_driver_is_red_naming_it(self):
        self.world.track("gate/proofs/drivers/test_d03.py", BAD)
        run = self.run_proofs()
        self.assertParityPassed(run, "proofs")
        failed = "gate/proofs/drivers/test_d03.py: pyshim: gate.proofs.drivers.test_d03: failed: 1 failures, 0 errors"
        self.assertRed(run, "proofs", "proofs", f"exit 1: {failed}")
        out = run.row_stdout("proofs", "proofs")
        self.assertIn(f"failed: {failed}", out)
        self.assertEqual(out.count("failed: gate/proofs/drivers/"), 1, out)

    def test_a_driver_outside_the_drivers_directory_is_not_counted(self):
        # The scope is :(glob)gate/proofs/drivers/test_*.py, and * never crosses a /.
        self.world.track("gate/proofs/drivers/deeper/__init__.py", "")
        self.world.track("gate/proofs/drivers/deeper/test_d10.py", GOOD)
        self.assertRow(self.run_proofs(), "proofs", "proofs", True, "covered 10")

    def test_a_crash_in_the_wrapper_is_exit_2_never_exit_1(self):
        # Exit 1 reads as "a driver failed"; a broken wrapper must not pass for that.
        row_dir = os.path.join(self.world.root, "row")
        os.mkdir(row_dir)
        argv = [support.pinned_python(), "-I", "-B", "-X", "utf8", os.path.join(self.world.repo, "gate/proofs/row.py")]
        out, err = os.path.join(row_dir, "out"), os.path.join(row_dir, "err")
        env = {**self.world.env, "GATE_ROW_DIR": row_dir}
        missing = os.path.join(row_dir, "no-such-files.nul")
        code = support.run_process([*argv, missing], cwd=self.world.repo, env=env, out=out, err=err, cap_s=60)
        lines = support.read_text(out).split("\n")[:-1]
        self.assertEqual(code, 2)
        self.assertTrue(lines[0].startswith("internal error: FileNotFoundError"), lines)
        self.assertEqual(lines[-2:], ["GATE FAILED", "exit: 2"])
