"""The grammar differential's proof (S2.2a exit line 1): a precedence swap planted in the engine's grammar is red.

Planted: in a world holding the gate, the engine's parser packages, the shape tool and the corpus, the participle
grammar's `+ -` and `* / %` levels are exchanged (internal/parser/grammar.go: SumRest and TermRest swap their
operator sets), so `a + b * c` parses as `(a + b) * c`. Verdicts do not change; shapes do, on every file mixing the
two levels. The expected t09 pair is written out by hand from the file's first filter:
    base + rate * secs > cap and cap - base > ${threshold} / 2
"""

import os

from gate.proofs.drivers import support

GRAMMAR = "internal/parser/grammar.go"
SUM_OPS, TERM_OPS = "@('+'|'-')", "@('*'|'/'|'%')"
T09 = "docs/language/eval/generated/torture/t09-mixed-arith-cmp.cep"
LARK = "(and (cmp > (arith + name (arith * name name)) name) (cmp > (arith - name name) (arith / param literal)))"
SWAPPED = "(and (cmp > (arith * (arith + name name) name) name) (cmp > (arith - name name) (arith / param literal)))"
SHAPE = "tools/shape/main.go"
GROUP_FORM = 'form = "group"'
EX10 = "docs/language/examples/10-grouped-keys.cep"
CORPUS = ("docs/language/examples", "docs/language/eval/generated", "docs/language/invalid")
ENGINE = ("internal/event", "internal/model", "internal/parser")


class DifferentialSwap(support.DriverTest):
    def engine_world(self) -> None:
        """The gate, the engine's production packages, the shape tool, the grammar and the whole corpus."""
        self.world.copy_gate()
        for rel in ("go.mod", "go.sum", "docs/language/grammar.lark", "tools/shape/main.go"):
            self.world.copy(rel)
        for prefix in ENGINE + CORPUS:
            for rel in self.world.tracked(prefix):
                if rel.endswith("_test.go") or "/testdata/" in rel or "/goldens/" in rel:
                    continue
                self.world.copy(rel)

    def test_a_swapped_precedence_level_is_red_by_shape(self):
        self.engine_world()
        path = os.path.join(self.world.repo, GRAMMAR)
        text = support.read_text(path)
        self.assertEqual((text.count(SUM_OPS), text.count(TERM_OPS)), (1, 1), "the plant's anchors moved")
        self.world.write(GRAMMAR, text.replace(SUM_OPS, "\0").replace(TERM_OPS, SUM_OPS).replace("\0", TERM_OPS))
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row("differential")), tier="T4")

        self.assertParityPassed(run, "test")
        record = run.record("test", "differential")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        stderr = run.row_file("test", "differential", "stderr")
        self.assertTrue(record["reason"].startswith("exit 1: docs/language/"), f"{record['reason']}\n{stderr}")
        self.assertEqual((record["covered"], record["expected"]), (73, 73), record)  # every file was judged
        failed = [line for line in run.row_stdout("test", "differential").split("\n") if line.startswith("failed: ")]
        self.assertIn(f"failed: {T09}: expression 1 differs: lark {LARK} / engine {SWAPPED}", failed)
        self.assertEqual([line for line in failed if "differs" not in line], [], failed)  # verdicts agree
        self.assertEqual([line for line in failed if "/invalid/" in line], [], failed)

    def test_a_grouped_key_shaped_as_a_single_key_is_red(self):
        # S3.2b-2 (review F8): the differential compares partition keys. Planted: the engine side shapes every group
        # `(a, b) of S` as `(key S a b)`; the lark side still says `(group S a b)`, so every grouped key differs.
        self.engine_world()
        path = os.path.join(self.world.repo, SHAPE)
        text = support.read_text(path)
        self.assertEqual(text.count(GROUP_FORM), 1, "the plant's anchor moved")
        self.world.write(SHAPE, text.replace(GROUP_FORM, 'form = "key"'))
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row("differential")), tier="T4")

        self.assertParityPassed(run, "test")
        record = run.record("test", "differential")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        failed = [line for line in run.row_stdout("test", "differential").split("\n") if line.startswith("failed: ")]
        self.assertIn(
            f"failed: {EX10}: expression 1 differs: "
            "lark (group Grants tenantId userId) / engine (key Grants tenantId userId)",
            failed,
        )
        self.assertEqual([line for line in failed if "differs" not in line], [], failed)  # verdicts agree
