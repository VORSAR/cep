"""The race-stress-root row's planted proof (S1.1c-11).

Planted: the race row's own racy package (gate/proofs/internal/racy, two goroutines writing one counter
unsynchronised; its test passes without the race detector), copied to the world's root, the one package the row names
(the world's root holds go.mod and no other Go file, so package racy is the root package). Run through the real
race-stress-root row at the nightly tier, it fails naming the racy test, and go ran that test 20 times: the row's
--repeat=20 reached go.
"""

import json

from gate.proofs.drivers import support

ROW = "race-stress-root"
REASON = "exit 1: github.com/vorsar/cep/gate/proofs: TestCount"


class RaceStressRootPlanted(support.DriverTest):
    def test_the_race_stress_root_row_catches_a_planted_race_in_the_root_package(self):
        self.world.copy_gate()
        self.world.copy_proofs_module()
        for name in ("racy.go", "racy_test.go"):
            self.world.copy(f"gate/proofs/internal/racy/{name}", name)
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row(ROW)), tier="nightly")

        self.assertParityPassed(run, ROW)
        self.assertRed(run, ROW, ROW, REASON)
        events = [json.loads(line) for line in run.row_file(ROW, ROW, "test.json").splitlines()]
        runs = [e for e in events if e.get("Action") == "run" and e.get("Test") == "TestCount"]
        self.assertEqual(len(runs), 20, "the row's --repeat=20 did not reach go test's -count")
