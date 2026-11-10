"""The regen tool's classifier, its file format and its all-or-nothing write (design s3-3-2 §8.1, §8.3; QA T49).

Every case runs on synthetic baseline text built here, never on the committed baselines. An output is the canonical
JSON text playback.AppendEvent writes; each one here carries its `ts` both as an attribute and at the top level, so a
timestamp drift reads as one whichever of the two the classifier looks at. A value change moves `scope` only.

`main` is driven through three module-level seams, patched per test: `REPO` (the repository root it works in),
`run_tier1(repo) -> int` (step 1's gate exit code) and `fresh_run(repo, out_dir) -> int` (step 2: the fresh files
written into `out_dir`, and the `go test` exit code).
"""

import contextlib
import io
import os
import re
import shutil
import tempfile
import unittest
from unittest import mock

from gate.baseline import regen
from gate.baseline.regen import Baseline, Block, Verdict

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
SPEC = "docs/language/SPEC.md"
BASELINE_DIR = "conformance/baseline"

F10 = "10-grouped-keys.cep"
F09 = "09-params-and-quoting.cep"
R10 = "AdminScopeGrant"
R09 = "HighRisk"
SHA_A = "a" * 64
SHA_B = "b" * 64
REV = 9
ZERO = (("errored", 0), ("key_errors", 0), ("late", 0), ("panics", 0))
LATE = (("errored", 0), ("key_errors", 0), ("late", 1), ("panics", 0))
K1, K2, K3 = "1" * 64, "2" * 64, "3" * 64

# The seams `main` is driven through, with the signatures this file calls them by.
SEAMS = {
    "REPO": "REPO: str, the repository root main reads conformance/baseline/ and SPEC.md under",
    "run_tier1": "run_tier1(repo: str) -> int, step 1: the two-row gate's exit code",
    "fresh_run": "fresh_run(repo: str, out_dir: str) -> int, step 2: TestBaseline with CEP_BASELINE_WRITE=out_dir",
}


def out(user: str, ts: str = "2026-01-01T10:00:00Z", scope: str = "admin") -> str:
    attrs = f'{{"scope":"{scope}","tenantId":"t1","ts":"{ts}","userId":"{user}"}}'
    return f'{{"attrs":{attrs},"stream":"GrantAlerts","ts":"{ts}"}}'


A = out("alice", "2026-01-01T10:00:00Z")
B = out("bob", "2026-01-01T10:01:00Z")
C = out("carol", "2026-01-01T10:02:00Z")


def block(ordinal: int, key: str, *outputs: str, rule: str = R10, file: str = F10, counts=ZERO) -> Block:
    return Block(file=file, rule=rule, ordinal=ordinal, key=key, outputs=tuple(outputs), counts=counts)


def baseline(*blocks: Block, file: str = F10, sha: str = SHA_A, rev: int = REV, fmt: int = 1) -> Baseline:
    return Baseline(file=file, format=fmt, source_sha256=sha, spec_rev=rev, blocks=tuple(blocks))


def files(*baselines: Baseline) -> dict[str, Baseline]:
    return {b.file: b for b in baselines}


def verdict(b: Block, cls: str, recorded: bool) -> Verdict:
    return Verdict(file=b.file, rule=b.rule, ordinal=b.ordinal, key=b.key, cls=cls, recorded=recorded)


def counts_json(counts) -> str:
    return "{" + ",".join(f'"{name}":{value}' for name, value in counts) + "}"


def line(b: Block) -> str:
    """§8.1's block line: keys sorted, no spaces, each output as the line holds it."""
    outputs = ",".join(b.outputs)
    return (
        f'{{"block":{b.ordinal},"counts":{counts_json(b.counts)},"key":"{b.key}",'
        f'"outputs":[{outputs}],"rule":"{b.rule}"}}\n'
    )


def text(b: Baseline) -> str:
    """§8.1's file: the header, then one line per block, one `\\n` per line."""
    header = f'{{"file":"{b.file}","format":{b.format},"source_sha256":"{b.source_sha256}","spec_rev":{b.spec_rev}}}\n'
    return header + "".join(line(x) for x in b.blocks)


# A file holding what the format must carry through: an empty output list, a non-zero count, a float, a null, a
# boolean and an int among an output's attributes.
ODD = '{"attrs":{"flag":true,"n":-3,"note":null,"risk":0.75,"ts":"2026-01-01T00:00:01.001Z"},"stream":"S"}'
SAMPLE = baseline(
    block(1, K1, A),
    block(2, K2),
    block(3, K3, ODD, B, counts=(("errored", 0), ("key_errors", 2), ("late", 0), ("panics", 0))),
)
SAMPLE_TEXT = (
    f'{{"file":"{F10}","format":1,"source_sha256":"{SHA_A}","spec_rev":9}}\n'
    f'{{"block":1,"counts":{{"errored":0,"key_errors":0,"late":0,"panics":0}},"key":"{K1}","outputs":[{A}],'
    f'"rule":"{R10}"}}\n'
    f'{{"block":2,"counts":{{"errored":0,"key_errors":0,"late":0,"panics":0}},"key":"{K2}","outputs":[],'
    f'"rule":"{R10}"}}\n'
    f'{{"block":3,"counts":{{"errored":0,"key_errors":2,"late":0,"panics":0}},"key":"{K3}","outputs":[{ODD},{B}],'
    f'"rule":"{R10}"}}\n'
)


class Format(unittest.TestCase):
    def test_the_sample_text_is_the_helpers_text(self):
        self.assertEqual(text(SAMPLE), SAMPLE_TEXT)  # a guard on this file's own helpers

    def test_render_writes_the_header_then_one_line_per_block(self):
        self.assertEqual(regen.render(SAMPLE), SAMPLE_TEXT)

    def test_parse_reads_the_header_and_every_block(self):
        self.assertEqual(regen.parse(SAMPLE_TEXT), SAMPLE)

    def test_parse_then_render_is_byte_for_byte(self):
        self.assertEqual(regen.render(regen.parse(SAMPLE_TEXT)), SAMPLE_TEXT)

    def test_a_malformed_line_is_a_value_error(self):
        header = SAMPLE_TEXT.split("\n")[0] + "\n"
        cases = {
            "a header that is not JSON": "not json\n" + SAMPLE_TEXT.split("\n", 1)[1],
            "a header without its format": f'{{"file":"{F10}","source_sha256":"{SHA_A}","spec_rev":9}}\n',
            "a block line cut short": header + '{"block":1,"counts":{"errored":0\n',
            "a block line without its key": header + line(block(1, K1, A)).replace(f'"key":"{K1}",', ""),
            "a block line that is a list": header + "[1,2]\n",
        }
        for name, bad in cases.items():
            with self.subTest(name), self.assertRaises(ValueError):
                regen.parse(bad)


class Classify(unittest.TestCase):
    def classify(self, old: dict, new: dict, *, rev: int = REV, accept: tuple[str, ...] = ()) -> list[Verdict]:
        return regen.classify(old, new, rev, frozenset(accept))

    def test_no_change_gives_no_verdict(self):
        old = files(baseline(block(1, K1, A, B), block(2, K2)))
        self.assertEqual(self.classify(old, dict(old)), [])

    def test_a_format_other_than_1_is_refused_as_format_change(self):
        new = baseline(block(1, K1, A))
        verdicts = self.classify(files(baseline(block(1, K1, A), fmt=2)), files(new))
        self.assertTrue(verdicts, "a stored format 2 gives a verdict even with every block equal")
        self.assertEqual({(v.file, v.cls, v.recorded) for v in verdicts}, {(F10, "format-change", False)})

    def test_a_new_key_is_recorded_as_additive(self):
        added = block(2, K2, C)
        verdicts = self.classify(files(baseline(block(1, K1, A))), files(baseline(block(1, K1, A), added, sha=SHA_B)))
        self.assertEqual(verdicts, [verdict(added, "additive-new-block", True)])

    def test_a_file_only_in_the_new_run_is_additive_for_each_block(self):
        first, second = block(1, K1, A, file=F09, rule=R09), block(2, K2, file=F09, rule=R09)
        new = files(baseline(block(1, K3, A)), baseline(first, second, file=F09))
        verdicts = self.classify(files(baseline(block(1, K3, A))), new)
        self.assertEqual(
            verdicts, [verdict(first, "additive-new-block", True), verdict(second, "additive-new-block", True)]
        )

    def test_a_gone_key_with_the_source_changed_is_recorded_as_removed(self):
        gone = block(2, K2, C)
        verdicts = self.classify(files(baseline(block(1, K1, A), gone)), files(baseline(block(1, K1, A), sha=SHA_B)))
        self.assertEqual(verdicts, [verdict(gone, "block-removed", True)])

    def test_a_file_only_in_the_old_baseline_is_removed_for_each_block(self):
        first, second = block(1, K1, A, file=F09, rule=R09), block(2, K2, B, file=F09, rule=R09)
        old = files(baseline(block(1, K3, A)), baseline(first, second, file=F09))
        verdicts = self.classify(old, files(baseline(block(1, K3, A))))
        self.assertEqual(verdicts, [verdict(first, "block-removed", True), verdict(second, "block-removed", True)])

    def test_a_gone_key_with_the_source_unchanged_is_refused_as_vanished(self):
        gone = block(2, K2, C)
        verdicts = self.classify(files(baseline(block(1, K1, A), gone)), files(baseline(block(1, K1, A))))
        self.assertEqual(verdicts, [verdict(gone, "block-vanished", False)])

    def test_another_number_of_outputs_is_refused_as_count_change(self):
        new = block(1, K1, A, B, C)
        verdicts = self.classify(files(baseline(block(1, K1, A, B))), files(baseline(new)))
        self.assertEqual(verdicts, [verdict(new, "count-change", False)])

    def test_the_same_outputs_in_another_order_are_refused_as_ordering_change(self):
        new = block(1, K1, B, A, C)
        verdicts = self.classify(files(baseline(block(1, K1, A, B, C))), files(baseline(new)))
        self.assertEqual(verdicts, [verdict(new, "ordering-change", False)])

    def test_a_moved_ts_is_refused_as_timestamp_drift(self):
        new = block(1, K1, A, out("bob", "2026-01-01T10:01:00.001Z"))
        verdicts = self.classify(files(baseline(block(1, K1, A, B))), files(baseline(new)))
        self.assertEqual(verdicts, [verdict(new, "timestamp-drift", False)])

    def test_the_same_outputs_with_other_counts_are_refused_as_drop_change(self):
        new = block(1, K1, A, counts=LATE)
        verdicts = self.classify(files(baseline(block(1, K1, A))), files(baseline(new)))
        self.assertEqual(verdicts, [verdict(new, "drop-change", False)])

    def spec_rev_case(self, accept: tuple[str, ...], *, rev: int = REV + 1) -> tuple[Block, list[Verdict]]:
        """Output 2 of block 2 changes `scope` only; SPEC's rev is `rev`, every header's REV."""
        new = block(2, K2, A, out("bob", "2026-01-01T10:01:00Z", scope="admin:all"))
        old = files(baseline(block(1, K1, A), block(2, K2, A, B)))
        return new, self.classify(old, files(baseline(block(1, K1, A), new)), rev=rev, accept=accept)

    def test_a_spec_rev_named_by_its_block_is_recorded(self):
        new, verdicts = self.spec_rev_case((f"{F09}", f"{F10}:{R10}:2"))
        self.assertEqual(verdicts, [verdict(new, "spec-rev", True)])

    def test_a_spec_rev_named_by_its_file_is_recorded(self):
        new, verdicts = self.spec_rev_case((F10,))
        self.assertEqual(verdicts, [verdict(new, "spec-rev", True)])

    def test_a_spec_rev_not_named_is_refused_as_value_change(self):
        new, verdicts = self.spec_rev_case(())
        self.assertEqual(verdicts, [verdict(new, "value-change", False)])

    def test_a_spec_rev_named_by_another_block_or_file_is_refused_as_value_change(self):
        for accept in [(f"{F10}:{R10}:1",), (f"{F10}:{R09}:2",), (F09,), (f"{F09}:{R10}:2",)]:
            with self.subTest(accept=accept):
                new, verdicts = self.spec_rev_case(accept)
                self.assertEqual(verdicts, [verdict(new, "value-change", False)])

    def test_a_value_change_with_an_unchanged_rev_is_refused_even_when_named(self):
        for accept in [(), (F10,), (f"{F10}:{R10}:2",)]:
            with self.subTest(accept=accept):
                new, verdicts = self.spec_rev_case(accept, rev=REV)
                self.assertEqual(verdicts, [verdict(new, "value-change", False)])

    def test_the_first_matching_row_wins(self):
        named = (F10,)
        drift_and_value = out("bob", "2026-01-01T10:01:00.001Z", scope="admin:all")
        cases = [
            ("count and drops: count-change", block(1, K1, A, B, C, counts=LATE), "count-change"),
            ("order and drops: ordering-change", block(1, K1, B, A, counts=LATE), "ordering-change"),
            (
                "ts, a named value and drops: timestamp-drift",
                block(1, K1, A, drift_and_value, counts=LATE),
                "timestamp-drift",
            ),
        ]
        for name, new, cls in cases:
            with self.subTest(name):
                verdicts = self.classify(
                    files(baseline(block(1, K1, A, B))), files(baseline(new)), rev=REV + 1, accept=named
                )
                self.assertEqual(verdicts, [verdict(new, cls, False)])
        with self.subTest("format 2 and a count change: format-change"):
            verdicts = self.classify(files(baseline(block(1, K1, A), fmt=2)), files(baseline(block(1, K1, A, B))))
            self.assertTrue(verdicts)
            self.assertEqual({(v.cls, v.recorded) for v in verdicts}, {("format-change", False)})

    def test_blocks_are_paired_by_the_full_key_text(self):
        # Two blocks of one rule with equal keys: the Go side writes the second as `<k>#2`; only that one changed.
        twin = K1 + "#2"
        new = block(2, twin, A, B)
        verdicts = self.classify(
            files(baseline(block(1, K1, A), block(2, twin, A))), files(baseline(block(1, K1, A), new))
        )
        self.assertEqual(verdicts, [verdict(new, "count-change", False)])

    def test_blocks_are_paired_by_key_never_by_ordinal(self):
        # A block written first shifts every ordinal by one; only the new key gives a verdict.
        added = block(1, K3, C)
        old = files(baseline(block(1, K1, A), block(2, K2, B)))
        new = files(baseline(added, block(2, K1, A), block(3, K2, B), sha=SHA_B))
        self.assertEqual(self.classify(old, new), [verdict(added, "additive-new-block", True)])

    def test_verdicts_are_sorted_by_file_then_ordinal_then_key(self):
        o10 = baseline(block(1, K1, A), block(2, K2, A))
        o09 = baseline(block(1, K1, A, file=F09, rule=R09), file=F09)
        n10 = baseline(block(1, K1, A, B), block(2, K2, B, A), sha=SHA_A)
        n09 = baseline(block(1, K1, A, file=F09, rule=R09), block(2, K3, C, file=F09, rule=R09), file=F09, sha=SHA_B)
        verdicts = self.classify(files(o10, o09), files(n10, n09))
        self.assertEqual(
            [(v.file, v.ordinal, v.key, v.cls) for v in verdicts],
            [(F09, 2, K3, "additive-new-block"), (F10, 1, K1, "count-change"), (F10, 2, K2, "count-change")],
        )


class Main(unittest.TestCase):
    """Steps 1-4 on a temporary repository holding SPEC.md and conformance/baseline/, the gate and Go stubbed."""

    def setUp(self):
        missing = [signature for name, signature in SEAMS.items() if not hasattr(regen, name)]
        if missing:
            self.fail("gate/baseline/regen.py has no seam for the tests: " + "; ".join(missing))
        tmp = tempfile.TemporaryDirectory(prefix="regen-")
        self.addCleanup(tmp.cleanup)
        self.repo = os.path.realpath(tmp.name)
        os.makedirs(os.path.join(self.repo, "docs", "language"))
        shutil.copy(os.path.join(REPO, SPEC), os.path.join(self.repo, SPEC))
        with open(os.path.join(REPO, SPEC), encoding="utf-8") as f:
            self.rev = int(re.search(r"rev (\d+) \(", f.read()).group(1))
        self.dir = os.path.join(self.repo, BASELINE_DIR)
        os.makedirs(self.dir)
        self.tier1, self.fresh, self.fresh_calls = 0, {}, []
        self.fresh_code = 0
        self.enterContext(mock.patch.object(regen, "REPO", self.repo))
        self.enterContext(mock.patch.object(regen, "run_tier1", lambda repo: self.tier1))
        self.enterContext(mock.patch.object(regen, "fresh_run", self.fake_fresh_run))

    def fake_fresh_run(self, repo: str, out_dir: str) -> int:
        self.fresh_calls.append(repo)
        for name, body in self.fresh.items():
            with open(os.path.join(out_dir, name), "w", encoding="utf-8") as f:
                f.write(body)
        return self.fresh_code

    def store(self, *baselines: Baseline) -> None:
        for b in baselines:
            with open(os.path.join(self.dir, b.file + ".jsonl"), "w", encoding="utf-8") as f:
                f.write(text(b))
        with open(os.path.join(self.dir, "README.md"), "w", encoding="utf-8") as f:
            f.write("the class table\n")

    def fresh_files(self, *baselines: Baseline) -> None:
        self.fresh = {b.file + ".jsonl": text(b) for b in baselines}

    def snapshot(self) -> dict[str, bytes]:
        found = {}
        for name in sorted(os.listdir(self.dir)):
            with open(os.path.join(self.dir, name), "rb") as f:
                found[name] = f.read()
        return found

    def main(self, *argv: str) -> tuple[int, str]:
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
            code = regen.main(list(argv))
        return code, buf.getvalue()

    def old_and_new(self) -> tuple[Baseline, Baseline]:
        return baseline(block(1, K1, A), rev=self.rev), baseline(block(1, K1, A, B), rev=self.rev)

    def test_tier1_red_is_refused_before_the_fresh_run(self):
        old, new = self.old_and_new()
        self.store(old)
        self.fresh_files(new)
        self.tier1 = 1
        before = self.snapshot()
        code, printed = self.main()
        self.assertEqual(code, 1, printed)
        self.assertIn("refused: Tier 1 is red", printed)
        self.assertEqual(self.fresh_calls, [], "the fresh run starts only after Tier 1 is green")
        self.assertEqual(self.snapshot(), before)

    def test_a_failed_fresh_run_is_refused(self):
        old, _ = self.old_and_new()
        self.store(old)
        self.fresh_code = 1
        before = self.snapshot()
        code, printed = self.main()
        self.assertEqual(code, 1, printed)
        self.assertIn("refused: the fresh run failed", printed)
        self.assertEqual(self.snapshot(), before)

    def test_one_refused_block_writes_nothing(self):
        # 09 gains a block (recorded on its own), 10's block 1 gains an output (refused): nothing is written.
        o09 = baseline(block(1, K2, A, file=F09, rule=R09), file=F09, rev=self.rev)
        n09 = baseline(block(1, K2, A, file=F09, rule=R09), block(2, K3, C, file=F09, rule=R09), file=F09, sha=SHA_B)
        old, new = self.old_and_new()
        self.store(old, o09)
        self.fresh_files(new, baseline(*n09.blocks, file=F09, sha=SHA_B, rev=self.rev))
        before = self.snapshot()
        code, printed = self.main()
        self.assertEqual(code, 1, printed)
        self.assertIn("count-change", printed)
        self.assertIn("additive-new-block", printed, "every block is printed with its class, the recorded ones too")
        self.assertEqual(self.snapshot(), before)

    def test_every_block_recorded_copies_the_fresh_files(self):
        old = baseline(block(1, K1, A), rev=self.rev)
        new = baseline(block(1, K1, A), block(2, K2, C), sha=SHA_B, rev=self.rev)
        self.store(old)
        self.fresh_files(new)
        readme = self.snapshot()["README.md"]
        code, printed = self.main()
        self.assertEqual(code, 0, printed)
        self.assertIn("additive-new-block", printed)
        self.assertEqual(self.snapshot(), {F10 + ".jsonl": text(new).encode(), "README.md": readme})

    def test_no_difference_prints_no_change(self):
        old = baseline(block(1, K1, A, B), block(2, K2), rev=self.rev)
        self.store(old)
        self.fresh_files(old)
        before = self.snapshot()
        code, printed = self.main()
        self.assertEqual(code, 0, printed)
        self.assertIn("no change", printed)
        self.assertEqual(self.snapshot(), before)

    def test_a_spec_rev_is_recorded_only_when_accepted(self):
        old = baseline(block(1, K1, A, B), rev=self.rev - 1)
        new = baseline(block(1, K1, A, out("bob", "2026-01-01T10:01:00Z", scope="admin:all")), rev=self.rev)
        self.store(old)
        self.fresh_files(new)
        before = self.snapshot()
        token = f"{F10}:{R10}:1"
        with self.subTest("not named: refused, its ACCEPT token printed"):
            code, printed = self.main()
            self.assertEqual(code, 1, printed)
            self.assertIn("value-change", printed)
            self.assertIn(token, printed)
            self.assertEqual(self.snapshot(), before)
        with self.subTest("named: recorded"):
            code, printed = self.main("--accept", token)
            self.assertEqual(code, 0, printed)
            self.assertIn("spec-rev", printed)
            self.assertEqual(self.snapshot()[F10 + ".jsonl"], text(new).encode())


if __name__ == "__main__":
    unittest.main()
