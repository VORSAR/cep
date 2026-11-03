"""The ruff proof of the format check (design note §8; re-check B3).

Planted: a temporary tracked gate/plant_format.py that ruff would reformat, in an otherwise clean copy of the gate.
The row's stdout carries ruff 0.17.3's own text, `File would be reformatted`, and the failure names only the plant.
"""

from gate.proofs.drivers import support

PLANT = "gate/plant_format.py"


class RuffFormat(support.DriverTest):
    def test_a_badly_formatted_file_is_red(self):
        self.world.copy_gate()
        self.world.commit()
        self.world.track(PLANT, "x  =  1\n")

        run = self.world.run(registry=self.world.registry(support.registry_row("ruff")))

        self.assertParityPassed(run, "lint")
        self.assertRed(run, "lint", "ruff", f"exit 1: {PLANT}: ruff format (exit 1)")
        out = run.row_stdout("lint", "ruff")
        self.assertIn("File would be reformatted", out)
        failed = [line for line in out.split("\n") if line.startswith("failed: ")]
        self.assertEqual(failed, [f"failed: {PLANT}: ruff format (exit 1)"])
