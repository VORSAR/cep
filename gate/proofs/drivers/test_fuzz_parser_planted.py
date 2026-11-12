"""The fuzz-parser row's planted proof (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10: every row
without a planted proof gets one).

A seed alone cannot fail a correct target, so the plant is a pair: a parser that panics on one input, and that input
committed as a seed of FuzzParse under the package's testdata/fuzz/. `go test -fuzz` runs the seeds before it
fuzzes, so the row fails at once, naming the fuzz file and the target, and keeps the input under crashers/. The
target's own output must name the planted seed: a build failure would give the row the same reason.

GATE_FUZZTIME is 3s (the row's default is 10m), so a plant that stopped failing costs seconds, not ten minutes.
"""

import os

from gate.proofs.drivers import support

FUZZ_FILE = "internal/parser/fuzz_test.go"
TARGET = "FuzzParse"
PARSE = "internal/parser/parse.go"
ANCHOR = "func Parse(name, src string) (*Result, error) {\n"
DEFECT = '\tif src == "gate-proof-plant" {\n\t\tpanic("planted")\n\t}\n'
SEED = f"internal/parser/testdata/fuzz/{TARGET}/planted"
SEED_TEXT = 'go test fuzz v1\nstring("gate-proof-plant")\n'


class FuzzParserPlanted(support.DriverTest):
    def test_a_seed_that_crashes_the_parser_is_red(self):
        self.world.copy_gate()
        for rel in ("go.mod", "go.sum"):
            self.world.copy(rel)
        for prefix in ("internal", "docs/language/examples"):
            for rel in self.world.tracked(prefix):
                self.world.copy(rel)
        text = support.read_text(os.path.join(self.world.repo, PARSE))
        self.assertEqual(text.count(ANCHOR), 1, "the plant's anchor moved")
        self.world.write(PARSE, text.replace(ANCHOR, ANCHOR + DEFECT))
        self.world.track(SEED, SEED_TEXT)
        self.world.commit()
        self.world.env["GATE_FUZZTIME"] = "3s"

        run = self.world.run(registry=self.world.registry(support.registry_row("fuzz-parser")), tier="nightly")

        self.assertParityPassed(run, "nightly")
        self.assertRed(run, "nightly", "fuzz-parser", f"exit 1: {FUZZ_FILE}: {TARGET} crashed: see crashers/")
        self.assertIn(f"{TARGET}/planted", run.row_file("nightly", "fuzz-parser", f"{TARGET}.stdout"))
        self.assertEqual(run.row_file("nightly", "fuzz-parser", "crashers", TARGET, "planted"), SEED_TEXT)
