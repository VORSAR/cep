"""The fuzz-decoder row's planted proof (S1.1c-2 W0; owner, 2026-11-12, S1.1c-2's plan check, decision 10: every row
without a planted proof gets one).

A seed alone cannot fail a correct target, so the plant is a pair: a decoder that panics on one payload, and that
payload committed as a seed of FuzzDecode under the package's testdata/fuzz/. `go test -fuzz` runs the seeds before
it fuzzes, so the row fails at once (the target's decodeSafe turns the panic into a disagreement), naming the fuzz
file and the target, and keeps the input under crashers/ beside the real committed seed. The target's own output must
name the planted seed: a build failure would give the row the same reason.

GATE_FUZZTIME is 3s (the row's default is 10m), so a plant that stopped failing costs seconds, not ten minutes.
"""

import os

from gate.proofs.drivers import support

FUZZ_FILE = "internal/event/fuzz_test.go"
TARGET = "FuzzDecode"
DECODE = "internal/event/decode.go"
ANCHOR = "func (d *Decoder) Decode(b []byte, dst []Value) ([]Value, Decoded, error) {\n"
DEFECT = '\tif string(b) == "gate-proof-plant" {\n\t\tpanic("planted")\n\t}\n'
SEED = f"internal/event/testdata/fuzz/{TARGET}/planted"
SEED_TEXT = 'go test fuzz v1\n[]byte("gate-proof-plant")\n'


class FuzzDecoderPlanted(support.DriverTest):
    def test_a_seed_that_crashes_the_decoder_is_red(self):
        self.world.copy_gate()
        for rel in ("go.mod", "go.sum"):
            self.world.copy(rel)
        for rel in self.world.tracked("internal"):
            self.world.copy(rel)
        text = support.read_text(os.path.join(self.world.repo, DECODE))
        self.assertEqual(text.count(ANCHOR), 1, "the plant's anchor moved")
        self.world.write(DECODE, text.replace(ANCHOR, ANCHOR + DEFECT))
        self.world.track(SEED, SEED_TEXT)
        self.world.commit()
        self.world.env["GATE_FUZZTIME"] = "3s"

        run = self.world.run(registry=self.world.registry(support.registry_row("fuzz-decoder")), tier="nightly")

        self.assertParityPassed(run, "nightly")
        self.assertRed(run, "nightly", "fuzz-decoder", f"exit 1: {FUZZ_FILE}: {TARGET} crashed: see crashers/")
        self.assertIn(f"{TARGET}/planted", run.row_file("nightly", "fuzz-decoder", f"{TARGET}.stdout"))
        self.assertEqual(run.row_file("nightly", "fuzz-decoder", "crashers", TARGET, "planted"), SEED_TEXT)
