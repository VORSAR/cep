"""Leak proof (ADR-0015 §4 "the leak step (goleak, owner-approved) catches a planted leak"; S3.2b's item 7, owner Q6).

Planted: gate/proofs/internal/leaky, whose test starts a goroutine that never ends and passes, under a TestMain that
runs goleak.VerifyTestMain, as internal/runtime's and internal/playback's do; beside internal/control. The unit row
fails naming the leaky package.
"""

from gate.proofs.drivers import support

LEAKY = "github.com/vorsar/cep/gate/proofs/internal/leaky"


class LeakPlanted(support.DriverTest):
    def test_the_unit_row_catches_a_planted_goroutine_leak(self):
        self.world.copy_gate()
        self.world.copy_proofs_module("leaky", "control")
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.UNIT_ROW), tier="T4")

        record = run.record("test", "unit")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertIn(LEAKY, record["reason"], record)
        self.assertRed(run, "test", "unit", f"exit 1: {LEAKY}: package failed")  # the exact reason (S1.1c-2 W0)
