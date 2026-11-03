"""The `hooks` row (gate design §12.7; plan check S4): `make check` refuses until the hooks are on."""

import os
import sys

from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
LAUNCHERS = (".githooks/pre-commit", ".githooks/pre-merge-commit", ".githooks/pre-push")
SCOPE = ("git", *LAUNCHERS)


class HooksRow(support.GateTest):
    def setUp(self):
        super().setUp()
        for rel in LAUNCHERS:
            self.write(rel, "print('hook')\n", mode=0o755)
        self.git("add", "--", ".githooks")

    def hooks_row(self):
        command = (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "hooks.py"))
        self.run_rows([self.row("hooks", command=command, expected_scope=SCOPE)])
        return self.reason("hooks")

    def test_hooks_off_fails_naming_make_setup(self):
        # Plan check N1 (exit line 10).
        self.assertEqual(self.hooks_row(), "exit 1: the hooks are off: run make setup")

    def test_the_relative_setting_passes(self):
        self.git("config", "core.hooksPath", ".githooks")
        self.assertEqual(self.hooks_row(), "covered 3")

    def test_the_absolute_setting_passes(self):
        # QA Q11, N29.
        self.git("config", "core.hooksPath", os.path.join(self.repo, ".githooks"))
        self.assertEqual(self.hooks_row(), "covered 3")

    def test_another_directory_fails(self):
        # Plan check N2.
        self.git("config", "core.hooksPath", os.path.join(self.root, "elsewhere"))
        self.assertEqual(self.hooks_row(), "exit 1: the hooks are off: run make setup")

    def test_a_launcher_that_cannot_run_fails_naming_it(self):
        self.git("config", "core.hooksPath", ".githooks")
        os.chmod(os.path.join(self.repo, ".githooks", "pre-push"), 0o644)
        self.assertEqual(
            self.hooks_row(), "exit 1: .githooks/pre-push is not executable: run git checkout -- .githooks"
        )
