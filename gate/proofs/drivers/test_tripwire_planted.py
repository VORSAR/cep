"""The metric tripwire's and the two-OS determinism proof's planted failures (S3.2b's exits 6 and 7; design s3-2b-2
§8, §9; ADR-0015 §3.11 and §4).

The world holds the gate, the engine with its tests, the corpus's example programs and gate/tripwire.json. Planted:
an outbox ceiling of 0, so the corpus's first output is one over it (a single Advance emitting past the ceiling); a
ceilings file with no entry for sink-depth (zero-config-is-failure); and, in the determinism test's own path, 64 keys
written in map iteration order, which no golden can match (the reviewer measured 3,000 orders in 3,000 runs at 64
keys; review F11).
"""

import json
import os

from gate.proofs.drivers import support

CORPUS_TEST = "internal/playback/s3_2b_2_corpus_test.go"
PLANT_SWITCH = 'os.Getenv("CEP_DETERMINISM_PLANT") == "maporder"'


class TripwireAndDeterminismPlanted(support.DriverTest):
    def engine_world(self) -> None:
        self.world.copy_gate()
        for rel in ("go.mod", "go.sum", "gate/tripwire.json"):
            self.world.copy(rel)
        for prefix in ("internal", "docs/language/examples"):
            for rel in self.world.tracked(prefix):
                self.world.copy(rel)

    def ceilings(self, edit) -> None:
        path = os.path.join(self.world.repo, "gate", "tripwire.json")
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        edit(data)
        self.world.write("gate/tripwire.json", json.dumps(data) + "\n")

    def run_row(self, job: str, row_id: str):
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row(row_id)), tier="T4")

    def test_an_advance_over_the_outbox_ceiling_is_red(self):
        self.engine_world()
        self.ceilings(lambda d: d["outbox"].update(ceiling=0))
        run = self.run_row("test", "tripwire")
        record = run.record("test", "tripwire")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertIn("TestTripwire", record["reason"], record)
        self.assertIn("over its blocking ceiling 0", run.row_file("test", "tripwire", "test.json"))

    def test_a_metric_without_a_ceiling_is_red(self):
        self.engine_world()
        self.ceilings(lambda d: d.pop("sink-depth"))
        run = self.run_row("test", "tripwire")
        record = run.record("test", "tripwire")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertIn("no ceiling configured for sink-depth", run.row_file("test", "tripwire", "test.json"))

    def test_a_map_order_nondeterminism_is_red_against_the_golden(self):
        self.engine_world()
        path = os.path.join(self.world.repo, CORPUS_TEST)
        text = support.read_text(path)
        self.assertEqual(text.count(PLANT_SWITCH), 1, "the plant's anchor moved")
        self.world.write(CORPUS_TEST, text.replace(PLANT_SWITCH, "true"))
        run = self.run_row("determinism", "determinism")
        record = run.record("determinism", "determinism")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertIn(
            "differs from testdata/determinism.golden", run.row_file("determinism", "determinism", "test.json")
        )
