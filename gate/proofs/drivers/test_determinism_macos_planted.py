"""The determinism-macos row's planted proof (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10:
`determinism-macos` gets a planted proof of its own).

The row runs the determinism row's command on the second OS (owner Q5), so it is planted as that row's proof plants
it (test_tripwire_planted.py): in TestDeterminismGolden's own path, 64 keys written in map iteration order, which no
golden can match (review F11). The world holds the gate, the engine with its tests, the corpus's example programs,
the conformance baseline and SPEC.md, so the row's other test, TestBaseline, passes beside the plant. Run through the
real determinism-macos row at T4, the row fails naming the golden test and only it.
"""

import os

from gate.proofs.drivers import support

CORPUS_TEST = "internal/playback/s3_2b_2_corpus_test.go"
PLANT_SWITCH = 'os.Getenv("CEP_DETERMINISM_PLANT") == "maporder"'
PACKAGE = "github.com/vorsar/cep/internal/playback"
REASON = f"exit 1: {PACKAGE}: TestDeterminismGolden"
FAILED = [f"failed: {PACKAGE}: TestDeterminismGolden", f"failed: {PACKAGE}: package failed", "failed: go test exit 1"]


class DeterminismMacosPlanted(support.DriverTest):
    def test_a_map_order_nondeterminism_is_red_against_the_golden(self):
        self.world.copy_gate()
        for rel in ("go.mod", "go.sum", "gate/tripwire.json"):
            self.world.copy(rel)
        self.world.copy("docs/language/SPEC.md")
        for prefix in ("internal", "docs/language/examples", "conformance/baseline"):
            for rel in self.world.tracked(prefix):
                self.world.copy(rel)
        text = support.read_text(os.path.join(self.world.repo, CORPUS_TEST))
        self.assertEqual(text.count(PLANT_SWITCH), 1, "the plant's anchor moved")
        self.world.write(CORPUS_TEST, text.replace(PLANT_SWITCH, "true"))
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row("determinism-macos")), tier="T4")

        self.assertParityPassed(run, "determinism-macos")
        self.assertRed(run, "determinism-macos", "determinism-macos", REASON)
        out = run.row_stdout("determinism-macos", "determinism-macos")
        self.assertEqual([line for line in out.split("\n") if line.startswith("failed: ")], FAILED, out)
        self.assertIn(
            "differs from testdata/determinism.golden",
            run.row_file("determinism-macos", "determinism-macos", "test.json"),
        )
