"""The race-stress-partition row's planted proof (S1.1c-11).

Planted: the race row's own racy package (gate/proofs/internal/racy, two goroutines writing one counter
unsynchronised; its test passes without the race detector), copied to internal/partition, the one package the row
names. Run through the real race-stress-partition row at the nightly tier, it fails naming the racy test, and go ran
that test 20 times: the row's --repeat=20 reached go.
"""

import json

from gate.proofs.drivers import support

ROW = "race-stress-partition"
REASON = "exit 1: github.com/vorsar/cep/gate/proofs/internal/partition: TestCount"


class RaceStressPartitionPlanted(support.DriverTest):
    def test_the_race_stress_partition_row_catches_a_planted_race_in_internal_partition(self):
        self.world.copy_gate()
        self.world.copy_proofs_module()
        for name in ("racy.go", "racy_test.go"):
            self.world.copy(f"gate/proofs/internal/racy/{name}", f"internal/partition/{name}")
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row(ROW)), tier="nightly")

        self.assertParityPassed(run, ROW)
        self.assertRed(run, ROW, ROW, REASON)
        events = [json.loads(line) for line in run.row_file(ROW, ROW, "test.json").splitlines()]
        runs = [e for e in events if e.get("Action") == "run" and e.get("Test") == "TestCount"]
        self.assertEqual(len(runs), 20, "the row's --repeat=20 did not reach go test's -count")
