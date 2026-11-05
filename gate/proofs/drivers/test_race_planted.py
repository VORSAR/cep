"""Race proof (owner, 2026-11-05: the race row pulled forward from S3.2b; ADR-0015 §3.4 "the race step catches a
planted race").

Planted: gate/proofs/internal/racy, two goroutines writing one counter unsynchronised, whose test passes without the
race detector, beside internal/control. The unit row runs both green; the race row, with -race and cgo on for itself,
fails naming the racy test.
"""

from gate.proofs.drivers import support

RACY = "github.com/vorsar/cep/gate/proofs/internal/racy: TestCount"


class RacePlanted(support.DriverTest):
    def test_the_race_row_catches_a_planted_race_the_unit_row_passes(self):
        self.world.copy_gate()
        self.world.copy_proofs_module("racy", "control")
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.UNIT_ROW, support.registry_row("race")), tier="T4")

        self.assertRow(run, "test", "unit", True, "covered 2")
        self.assertRed(run, "race", "race", f"exit 1: {RACY}")
