"""The hooks-off proof (exit line 10; design §12.7; plan check S4, N1, N2, N29).

The real registry's `hooks` row, in a world holding real copies of the gate and the launchers: with the hooks off,
or `core.hooksPath` pointing anywhere but this repository's .githooks, the row fails naming `make setup`. With the
hooks on, by the relative path `make setup` writes or by the absolute path, it passes (QA Q11: no false red).
"""

import os

from gate.proofs.drivers import support

OFF = "the hooks are off: run make setup"


class HooksOff(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate()
        support.copy_hooks(self.world)
        self.world.commit()
        self.registry = self.world.registry(support.registry_row("hooks"))

    def assertHooksRed(self, reason: str = OFF) -> None:
        run = self.world.run(registry=self.registry)
        self.assertParityPassed(run, "lint")
        self.assertGateFailed(run)
        record = run.record("lint", "hooks")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertIn(reason, record["reason"], run.stdout)
        cells = run.summary("lint", "hooks")
        self.assertEqual(cells[2], "FAIL", run.stdout)
        self.assertIn(reason, cells[4], run.stdout)

    def assertHooksGreen(self) -> None:
        run = self.world.run(registry=self.registry)
        self.assertParityPassed(run, "lint")
        self.assertEqual(run.record("lint", "hooks")["verdict"], "pass", run.stdout)
        self.assertEqual(run.summary("lint", "hooks")[2], "PASS", run.stdout)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)

    def test_hooks_off_is_red_naming_make_setup(self):
        self.assertHooksRed()

    def test_hooks_path_elsewhere_is_red(self):
        os.makedirs(os.path.join(self.world.repo, "other"))
        self.world.git("config", "core.hooksPath", "other")
        self.assertHooksRed()

    def test_a_launcher_that_is_not_executable_is_red(self):
        self.world.git("config", "core.hooksPath", ".githooks")
        launcher = os.path.join(self.world.repo, ".githooks", "pre-commit")
        os.chmod(launcher, 0o644)
        self.assertHooksRed(".githooks/pre-commit is not executable: run git checkout -- .githooks")

    def test_the_relative_hooks_path_of_make_setup_is_green(self):
        support.hooks_on(self.world)
        self.assertEqual(self.world.git("config", "core.hooksPath").strip(), ".githooks")
        self.assertHooksGreen()

    def test_the_absolute_hooks_path_is_green(self):
        self.world.git("config", "core.hooksPath", os.path.join(self.world.repo, ".githooks"))
        self.assertHooksGreen()
