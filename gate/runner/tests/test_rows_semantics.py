"""The semantics row wrapper (S3.1-1; owner 2026-11-04): the language's reference checks run on every gate.

A temporary repository holds a harness in the shape of docs/language/checks/test_semantics.py — `@check`
declarations, one `<section> · <desc> · OK` or `· FAIL: …` line per check — run under the pinned CPython. A harness
whose every declared check prints OK covers one; a failing check, a declared check that never reported, and a
harness that crashes each fail the row, naming the check or the file.
"""

import os
import sys

from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
HARNESS = """import sys
CHECKS = []
def check(section, desc):
    def deco(fn):
        CHECKS.append((section, desc, fn))
        return fn
    return deco

@check("1", "one holds")
def _():
    assert True

@check("2", "two holds")
def _():
    assert {two}

def main():
    failures = 0
    for section, desc, fn in CHECKS[:{run}]:
        try:
            fn()
        except Exception as exc:
            failures += 1
            print(f"{{section}} · {{desc}} · FAIL: {{exc!r}}")
        else:
            print(f"{{section}} · {{desc}} · OK")
    return 1 if failures else 0

if __name__ == "__main__":
    sys.exit(main())
"""
PATH = "docs/language/checks/test_semantics.py"


class Semantics(support.GateTest):
    def setUp(self):
        super().setUp()
        self.toolchain.dirs = [support.tool_bin("python")]

    def semantics(self):
        command = (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "semantics.py"), "{files}")
        self.run_rows([self.row("semantics", command=command, expected_scope=("git", PATH), job="test")])
        return self.reason("semantics", "test")

    def test_every_declared_check_ok_covers_one(self):
        self.track(PATH, HARNESS.format(two="True", run=2))
        self.assertEqual(self.semantics(), "covered 1")

    def test_a_failing_check_is_named(self):
        self.track(PATH, HARNESS.format(two="False", run=2))
        self.assertEqual(self.semantics(), f"exit 1: {PATH}: 2 · two holds · FAIL: AssertionError()")

    def test_a_declared_check_that_never_reported_fails(self):
        self.track(PATH, HARNESS.format(two="True", run=1))
        self.assertEqual(self.semantics(), f"exit 1: {PATH}: 1 of its 2 declared checks reported OK")

    def test_a_harness_that_crashes_fails(self):
        self.track(PATH, "raise SystemExit('planted crash')\n@check('x', 'y')\ndef _(): pass\n")
        self.assertEqual(self.semantics(), f"exit 1: {PATH}: exit 1: planted crash")
