"""gate/runner/pyshim.py: a Python test module passes only by really running (design note §5; DQ17)."""

import os
import signal
import subprocess
import sys

from gate import wait
from gate.runner.tests import support

SHIM = os.path.join(support.GATE, "runner", "pyshim.py")
FLAGS = ("-I", "-B", "-X", "dev", "-W", "error")
HEAD = "import unittest\n\n\nclass T(unittest.TestCase):\n"


class Shim(support.GateTest):
    def shim(self, body, python=sys.executable):
        self.write("pkg/__init__.py", "")
        self.write("pkg/test_m.py", body)
        return support.run_tool([python, *FLAGS, SHIM, self.repo, "pkg.test_m"], self.repo)

    def assertBad(self, body, needle):
        code, out, err = self.shim(body)
        self.assertEqual(code, 1, (out, err))
        self.assertIn(needle, out.decode())
        self.assertIn("pkg.test_m", out.decode())

    def test_a_real_test_passes_and_reports_it(self):
        code, out, err = self.shim(HEAD + "    def test_ok(self):\n        self.assertTrue(True)\n")
        self.assertEqual(code, 0, (out, err))
        self.assertEqual(out.decode().splitlines()[-1], "pyshim: pkg.test_m: passed 1 tests")

    def test_a_stopped_module_runs_its_cleanups_and_starts_no_later_test(self):
        # review BA1, break-it QB4: TERM ends the test where it is, its cleanups run, and no later test starts.
        w = self.witness("start")
        cleaned, later = os.path.join(self.root, "cleaned"), os.path.join(self.root, "later")
        body = (
            "import os\nimport signal\n"
            + HEAD
            + (
                "    def test_a_holds(self):\n"
                f"        self.addCleanup(lambda: open({cleaned!r}, 'w').close())\n"
                f"        os.write(os.open({w.path!r}, os.O_WRONLY), b's')\n"
                "        signal.pause()\n"
                "\n"
                "    def test_b_later(self):\n"
                f"        open({later!r}, 'w').close()\n"
            )
        )
        self.write("pkg/__init__.py", "")
        self.write("pkg/test_m.py", body)
        out = os.path.join(self.root, "shim.stdout")
        with open(out, "wb") as f:
            proc = subprocess.Popen([sys.executable, *FLAGS, SHIM, self.repo, "pkg.test_m"], cwd=self.repo,
                                    env=support.base_env(), stdin=subprocess.DEVNULL,
                                    stdout=f, stderr=f)  # fmt: skip  # aligned arguments
        self.addCleanup(proc.wait)
        self.addCleanup(proc.kill)
        w.started(1)
        proc.send_signal(signal.SIGTERM)
        self.assertEqual(proc.wait(timeout=wait.DEFAULT_CAP_S), 1)
        w.gone()
        with open(out, "rb") as f:
            last = f.read().decode().splitlines()[-1]
        self.assertEqual(last, "pyshim: pkg.test_m: stopped by SIGTERM; failed: 0 failures, 1 errors")
        self.assertEqual((os.path.exists(cleaned), os.path.exists(later)), (True, False))

    def test_exit_0_at_import_is_not_a_pass(self):
        self.assertBad("import sys\n\nsys.exit(0)\n", "import error")

    def test_an_exception_at_import(self):
        self.assertBad("raise RuntimeError('x')\n", "import error")

    def test_no_tests(self):
        self.assertBad("import unittest\n", "ran 0 tests")

    def test_an_empty_test_class(self):
        self.assertBad(HEAD + "    pass\n", "ran 0 tests")

    def test_an_import_error(self):
        self.assertBad("import no_such_module_xyz\n", "import error")

    def test_a_syntax_error(self):
        self.assertBad("def f(:\n", "import error")

    def test_a_skip(self):
        self.assertBad(HEAD + "    @unittest.skip('x')\n    def test_a(self):\n        pass\n", "skipped")

    def test_a_skip_beside_a_pass(self):
        body = HEAD + "    def test_a(self):\n        pass\n\n    def test_b(self):\n        self.skipTest('x')\n"
        self.assertBad(body, "skipped")

    def test_a_class_level_skip(self):
        body = HEAD.replace("class T", "@unittest.skip('x')\nclass T") + "    def test_a(self):\n        pass\n"
        self.assertBad(body, "skipped")

    def test_an_expected_failure(self):
        # design-QA counter-example 28: a skip in disguise.
        body = HEAD + "    @unittest.expectedFailure\n    def test_a(self):\n        self.fail('x')\n"
        self.assertBad(body, "expected failure")

    def test_an_unexpected_success(self):
        body = HEAD + "    @unittest.expectedFailure\n    def test_a(self):\n        pass\n"
        self.assertBad(body, "unexpected success")

    def test_a_failure(self):
        self.assertBad(HEAD + "    def test_a(self):\n        self.fail('x')\n", "failed")

    def test_a_leaked_file(self):
        # design-QA counter-example 29: a ResourceWarning raised in a destructor reaches the unraisable hook.
        body = HEAD + f"    def test_a(self):\n        open({SHIM!r}).read()\n"
        self.assertBad(body, "leak")

    def test_a_leaked_process(self):
        body = (
            "import subprocess\nimport sys\n" + HEAD + "    def test_a(self):\n"
            "        subprocess.Popen([sys.executable, '-c', 'pass'])\n"
        )
        self.assertBad(body, "leak")

    def test_a_warning_is_an_error(self):
        body = "import warnings\n" + HEAD + "    def test_a(self):\n        warnings.warn('x')\n"
        self.assertBad(body, "failed")

    def test_sigchld_is_reset_for_the_tests(self):
        body = (
            "import signal\n" + HEAD + "    def test_a(self):\n"
            "        self.assertEqual(signal.getsignal(signal.SIGCHLD), signal.SIG_DFL)\n"
        )
        self.assertEqual(self.shim(body)[0], 0)

    def test_runs_under_the_floor_interpreter(self):
        floor = os.path.join(support.tool_bin("python-floor"), "python3.12")
        body = (
            HEAD
            + "    def test_a(self):\n        import sys\n        self.assertEqual(sys.version_info[:2], (3, 12))\n"
        )
        code, out, err = self.shim(body, python=floor)
        self.assertEqual(code, 0, (out, err))
