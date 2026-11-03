"""The ruff proof of the sleep ban (design note §8; QA DQ6): an inline `noqa` waives nothing.

Planted: a temporary tracked gate/plant_sleep.py holding `time.sleep(0)  # noqa: TID251`, in an otherwise clean copy
of the gate. The row runs `ruff check --ignore-noqa`, so ruff.toml is the only waiver and the plant is red.
"""

from gate.proofs.drivers import support

PLANT = "gate/plant_sleep.py"


class RuffSleep(support.DriverTest):
    def test_a_sleep_with_an_inline_noqa_is_red(self):
        self.world.copy_gate()
        self.world.commit()
        self.world.track(PLANT, "import time\n\ntime.sleep(0)  # noqa: TID251\n")

        run = self.world.run(registry=self.world.registry(support.registry_row("ruff")))

        self.assertParityPassed(run, "lint")
        self.assertRed(run, "lint", "ruff", "exit 1")
        out = run.row_stdout("lint", "ruff")
        self.assertIn("ruff> TID251 `time.sleep` is banned", out)
        failed = [line for line in out.split("\n") if line.startswith("failed: ")]
        self.assertEqual(failed, [f"failed: {PLANT}: ruff check (exit 1)"])
