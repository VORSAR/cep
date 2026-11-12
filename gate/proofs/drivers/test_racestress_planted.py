"""The race-stress row's planted proof (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10:
`race-stress` gets a planted proof of its own).

Planted: the race row's own racy package (gate/proofs/internal/racy, two goroutines writing one counter
unsynchronised; its test passes without the race detector), copied to internal/runtime, the one package the row
names. Run through the real race-stress row at the nightly tier, it fails naming the racy test, and go ran that test
20 times: the row's --repeat=20 reached go.
"""

import json

from gate.proofs.drivers import support

REASON = "exit 1: github.com/vorsar/cep/gate/proofs/internal/runtime: TestCount"


class RaceStressPlanted(support.DriverTest):
    def test_the_race_stress_row_catches_a_planted_race_in_internal_runtime(self):
        self.world.copy_gate()
        self.world.copy_proofs_module()
        for name in ("racy.go", "racy_test.go"):
            self.world.copy(f"gate/proofs/internal/racy/{name}", f"internal/runtime/{name}")
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row("race-stress")), tier="nightly")

        self.assertParityPassed(run, "nightly")
        self.assertRed(run, "nightly", "race-stress", REASON)
        events = [json.loads(line) for line in run.row_file("nightly", "race-stress", "test.json").splitlines()]
        runs = [e for e in events if e.get("Action") == "run" and e.get("Test") == "TestCount"]
        self.assertEqual(len(runs), 20, "the row's --repeat=20 did not reach go test's -count")
