"""The real pytests and pyfloor wrappers (design note §5; plan check C10; design-QA counter-example 14)."""

import os
import sys

from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
GOOD = "import unittest\n\n\nclass T(unittest.TestCase):\n    def test_a(self):\n        self.assertTrue(True)\n"
SKIP = "import unittest\n\n\n@unittest.skip('x')\nclass T(unittest.TestCase):\n    def test_a(self):\n        pass\n"
TESTS = ("git", ":(glob)gate/**/test_*.py", ":(glob,exclude)gate/proofs/**")


def wrapper(name):
    return (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, f"{name}.py"), "{files}")


class Base(support.GateTest):
    def setUp(self):
        super().setUp()
        self.toolchain.dirs = [os.path.dirname(sys.executable), support.tool_bin("python-floor")]
        self.track("gate/__init__.py", "")
        self.track("gate/x/__init__.py", "")

    def pytests(self):
        self.run_rows([self.row("pytests", command=wrapper("pytests"), expected_scope=TESTS, job="test")])
        return self.reason("pytests", "test")

    def out(self):
        return self.row_file("test", "pytests", "stdout").decode()


class Pytests(Base):
    def test_every_tracked_module_runs(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_b.py", GOOD)
        self.assertEqual(self.pytests(), "covered 2")

    def test_a_top_level_gate_module_is_in_scope_and_runs(self):
        # design-QA counter-example 14, as the `:(glob)` fix leaves it: this file now runs, so its failure is seen.
        self.track("gate/test_top.py", GOOD.replace("assertTrue(True)", "fail('x')"))
        self.track("gate/x/test_a.py", GOOD)
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/test_top.py", self.out())

    def test_proofs_are_outside_this_row(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/proofs/__init__.py", "")
        self.track("gate/proofs/drivers/__init__.py", "")
        self.track("gate/proofs/drivers/test_d.py", "raise SystemExit('never imported here')\n")
        self.assertEqual(self.pytests(), "covered 1")

    def test_a_module_that_fails_to_import_is_named(self):
        # C10 (d).
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_bad.py", "import no_such_module_xyz\n")
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/x/test_bad.py", self.out())
        self.assertIn("import error", self.out())

    def test_a_module_whose_tests_all_skip_is_named(self):
        # C10 (c).
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_skip.py", SKIP)
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/x/test_skip.py", self.out())
        self.assertIn("skipped", self.out())

    def test_a_module_with_no_tests_is_named(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_none.py", "x = 1\n")
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/x/test_none.py", self.out())
        self.assertIn("ran 0 tests", self.out())

    def test_a_module_that_exits_0_at_import_never_passes(self):
        # Exit 0 alone proves nothing: a module can end the process before a single test runs.
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_exit.py", "import sys\n\nsys.exit(0)\n")
        self.track("gate/x/test_hard.py", "import os\n\nos._exit(0)\n")
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/x/test_exit.py", self.out())
        self.assertIn("gate/x/test_hard.py", self.out())

    def test_a_module_in_a_directory_without_init_is_named(self):
        # C10 (a): namespace packages would import it silently, and the layout rule would be gone.
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/noinit/test_b.py", GOOD)
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/noinit/test_b.py", self.out())
        self.assertIn("__init__.py", self.out())

    def test_a_tracked_misnamed_module_is_named(self):
        # C10 (b), design-QA counter-example 14.
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/runner_test.py", GOOD)
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/x/runner_test.py", self.out())

    def test_an_untracked_misnamed_module_is_named(self):
        self.track("gate/x/test_a.py", GOOD)
        self.write("gate/x/other_test.py", GOOD)
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/x/other_test.py", self.out())

    def test_an_untracked_test_module_is_named(self):
        self.track("gate/x/test_a.py", GOOD)
        self.write("gate/x/test_new.py", GOOD)
        self.assertEqual(self.pytests(), "exit 1")
        self.assertIn("gate/x/test_new.py", self.out())
        self.assertIn("git add", self.out())

    def test_a_stray_file_under_tests_is_named(self):
        self.track("gate/tests/__init__.py", "")
        self.track("gate/tests/support.py", "")
        self.track("gate/tests/test_a.py", GOOD)
        self.track("gate/tests/helpers.py", "x = 1\n")
        self.assertEqual(self.pytests(), "exit 1")
        out = self.out()
        self.assertIn("gate/tests/helpers.py", out)
        self.assertNotIn("gate/tests/support.py", out)

    def test_the_named_helpers_under_tests_are_allowed(self):
        self.track("gate/tests/__init__.py", "")
        self.track("gate/tests/support.py", "x = 1\n")
        self.track("gate/tests/test_a.py", GOOD)
        self.assertEqual(self.pytests(), "covered 1")

    def test_a_row_narrowed_by_one_module_is_red(self):
        # plan check P5: the count is what ran, so a list one short is red, naming both numbers.
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_b.py", GOOD)
        short = os.path.join(self.root, "short.nul")
        with open(short, "wb") as f:
            f.write(b"gate/x/test_a.py\0")
        command = (*wrapper("pytests")[:-1], short)
        self.run_rows([self.row("pytests", command=command, expected_scope=TESTS, job="test")])
        self.assertEqual(self.reason("pytests", "test"), "covered 1 ≠ expected 2")

    def test_each_module_runs_in_its_own_process(self):
        # Module state does not leak: a global set by one module is unset in the other.
        body = GOOD.replace("assertTrue(True)", "assertNotIn('marker', vars(__import__('builtins')))")
        setter = "import builtins\n\nbuiltins.marker = 1\n" + GOOD
        self.track("gate/x/test_a.py", setter)
        self.track("gate/x/test_b.py", body)
        self.assertEqual(self.pytests(), "covered 2")

    def test_the_tests_run_under_the_pinned_interpreter(self):
        body = GOOD.replace("assertTrue(True)", "assertEqual(__import__('sys').version_info[:2], (3, 15))")
        self.track("gate/x/test_a.py", body)
        self.assertEqual(self.pytests(), "covered 1")

    def test_the_tests_run_with_warnings_as_errors(self):
        body = GOOD.replace("assertTrue(True)", "assertIn('error', __import__('sys').warnoptions)")
        self.track("gate/x/test_a.py", body)
        self.assertEqual(self.pytests(), "covered 1")


class Pyfloor(Base):
    def pyfloor(self, *paths):
        rows = [self.row("pyfloor", command=wrapper("pyfloor"), expected_scope=("git", *paths), job="lint")]
        self.run_rows(rows)
        return self.reason("pyfloor")

    def test_the_modules_run_under_the_floor(self):
        body = GOOD.replace("assertTrue(True)", "assertEqual(__import__('sys').version_info[:2], (3, 12))")
        self.track("gate/x/test_a.py", body)
        self.track("gate/x/test_b.py", body)
        self.assertEqual(self.pyfloor("gate/x/test_a.py", "gate/x/test_b.py"), "covered 2")

    def test_only_the_listed_modules_run(self):
        self.track("gate/x/test_a.py", GOOD)
        self.track("gate/x/test_other.py", "raise SystemExit('not listed')\n")
        self.assertEqual(self.pyfloor("gate/x/test_a.py"), "covered 1")

    def test_syntax_newer_than_the_floor_is_red(self):
        # 3.14's unparenthesised except, design note §8 test_pyfloor.
        body = GOOD.replace(
            "self.assertTrue(True)", "try:\n            pass\n        except ValueError, TypeError:\n            pass"
        )
        self.track("gate/x/test_a.py", body)
        self.assertEqual(self.pyfloor("gate/x/test_a.py"), "exit 1")
        out = self.row_file("lint", "pyfloor", "stdout").decode()
        self.assertIn("gate/x/test_a.py", out)
        self.assertIn("import error", out)

    def test_an_api_newer_than_the_floor_is_red(self):
        # copy.replace is 3.13, design note §8 test_pyfloor.
        body = GOOD.replace("assertTrue(True)", "assertTrue(__import__('copy').replace)")
        self.track("gate/x/test_a.py", body)
        self.assertEqual(self.pyfloor("gate/x/test_a.py"), "exit 1")

    def test_the_real_guard_module_passes_under_the_floor(self):
        # The one floor module on this branch: run for real, from the real repository, through the wrapper.
        scope = ("git", "gate/runner/tests/test_run_guard.py")
        rows = [self.row("pyfloor", command=wrapper("pyfloor"), expected_scope=scope)]
        self.run_rows(rows, repo=support.REPO)
        self.assertEqual(self.reason("pyfloor"), "covered 1")
