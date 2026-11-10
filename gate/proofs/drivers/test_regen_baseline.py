"""The regen tool's proofs (exit 6; design s3-3-2 §8.3; QA T50): a refused class writes nothing, an addition records.

The world holds the gate, the module (go.mod, go.sum and every main-module package `go list -deps -test` names for
./conformance and ./internal/playback), the corpus, SPEC.md, the semantics harness (Tier 1's second row) and the
committed baselines, conformance/baseline/. The tool runs as a developer would run it, by path under the pinned
python from the world's root, with the pinned go first on PATH.

Where the plants go. A block's key is its content (§8.1; the owner's decision 4 of 2026-11-09 in
docs/sprints/decisions.md: "a new example is recorded as an addition; a removed one is recorded only when its corpus
file changed"). Any edit of a block's given or then list in the corpus is therefore a new key and a gone key in a
changed file: additive-new-block and block-removed, both recorded, exit 0. So §8.3's three refused plants, written as
corpus edits, could never be refused. A refused class needs the same key with other outputs: what an engine change
under an unchanged corpus gives. Each refused plant below edits the stored baseline instead, the old side of the
comparison, so the corpus (and so Tier 1, and every key) is untouched and the fresh run differs from it in exactly
the named way. The addition is a corpus edit, as §8.3 has it.
"""

import hashlib
import json
import os
import re

from gate.proofs.drivers import support

BASELINE = "conformance/baseline"
EX09 = "docs/language/examples/09-params-and-quoting.cep"
EX10 = "10-grouped-keys.cep"
SPEC = "docs/language/SPEC.md"
BASELINE_TEST = "internal/playback/baseline_test.go"
CLASSES = (
    "format-change",
    "additive-new-block",
    "block-removed",
    "block-vanished",
    "count-change",
    "ordering-change",
    "timestamp-drift",
    "drop-change",
    "spec-rev",
    "value-change",
)
GO_ENV = {"GOTOOLCHAIN": "local", "GOENV": "off", "GOFLAGS": "", "GOWORK": "off", "CGO_ENABLED": "0"}
MAIN_DIRS = "{{if .Module}}{{if .Module.Main}}{{.Dir}}{{end}}{{end}}"
WITH_TESTS = ("conformance", "internal/playback")  # the packages whose tests run: TestCorpus, TestBaseline

# A second rule for 09, appended after HighRisk: HighRisk's own shape with a staging filter, over the streams 09
# already declares, so it declares nothing new and HighRisk's four keys stay as they are.
# The new rule writes its own stream: a derived stream has one producer (E-GRAPH-003; fixture edit at T4, named in
# the record: the first form inserted into RiskAlerts, which Tier 1 refused).
STAGING_RULE = """

stream StagingAlerts (agentId string, scaled float, ts timestamp);

rule StagingRisk
  intent 'Alert when a scaled risk score crosses the configured threshold in staging.'
  from `agent-events`[risk * 100 / 100 > ${threshold} and env == 'staging']
  expect given `agent-events` [ {agentId: 'a3', risk: 0.9, env: 'staging', ts: timestamp '2026-01-01T00:00:01Z'} ]
         then StagingAlerts [ {agentId: 'a3', scaled: 1.0, ts: timestamp '2026-01-01T00:00:01Z'} ]
  select agentId, math:round(risk) as scaled
  output first every ${lookback}
  insert into StagingAlerts;"""
LAST_LINE_09 = "  insert into RiskAlerts;"


def go_bin() -> str:
    tool = support.tool("go")
    return os.path.join(support.home(support.real_cache(), "go"), tool.path_dir)


def canonical(value: dict) -> str:
    """§8.1's line form: keys sorted, no spaces. Equal to the stored text for 10's lines (ASCII strings only)."""
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def classes_in(text: str) -> set[str]:
    return {c for c in CLASSES if re.search(rf"(?<![\w-]){re.escape(c)}(?![\w-])", text)}


class RegenBaseline(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        self.copy_module()
        for prefix in ("docs/language/examples", "docs/language/checks", "conformance", "internal/playback"):
            for rel in self.world.tracked(prefix):
                self.world.copy(rel)
        self.world.copy(SPEC)

    def copy_module(self) -> None:
        """go.mod, go.sum and the files directly in every main-module package the two test packages build."""
        self.world.copy("go.mod")
        self.world.copy("go.sum")
        out, err = os.path.join(self.world.root, "list.stdout"), os.path.join(self.world.root, "list.stderr")
        argv = [os.path.join(go_bin(), "go"), "list", "-deps", "-test", "-f", MAIN_DIRS, "./conformance"]
        argv += ["./internal/playback"]
        env = {**self.world.env, **GO_ENV}
        code = support.run_process(argv, cwd=support.REPO, env=env, out=out, err=err, cap_s=support.RUN_CAP_S)
        self.assertEqual(code, 0, support.read_text(err))
        repo = os.path.realpath(support.REPO)
        dirs = {os.path.relpath(os.path.realpath(d), repo) for d in support.read_text(out).split("\n") if d}
        tracked = self.world.tracked(".")
        for d in sorted(dirs):
            here = "" if d == "." else d
            for rel in tracked:
                if os.path.dirname(rel) == here and (d in WITH_TESTS or not rel.endswith("_test.go")):
                    self.world.copy(rel)

    # -------------------------------------------------------------------------------------------- the baselines

    def stored_path(self, corpus_file: str) -> str:
        path = os.path.join(self.world.repo, BASELINE, corpus_file + ".jsonl")
        self.assertTrue(os.path.isfile(path), f"{BASELINE}/{corpus_file}.jsonl: no baseline recorded yet")
        return path

    def snapshot(self) -> dict[str, bytes]:
        found = {}
        root = os.path.join(self.world.repo, BASELINE)
        for name in sorted(os.listdir(root)):
            with open(os.path.join(root, name), "rb") as f:
                found[name] = f.read()
        return found

    def stored_lines(self, corpus_file: str) -> list[str]:
        text = support.read_text(self.stored_path(corpus_file))
        self.assertTrue(text.endswith("\n"), "one \\n per line")
        return text.split("\n")[:-1]

    def replant(self, corpus_file: str, rule: str, ordinal: int, change) -> None:
        """The stored line of one block, its outputs passed through `change`; every other byte as stored."""
        lines = self.stored_lines(corpus_file)
        parsed = [json.loads(line) for line in lines]
        hits = [i for i, d in enumerate(parsed) if i > 0 and (d["rule"], d["block"]) == (rule, ordinal)]
        self.assertEqual(len(hits), 1, f"{corpus_file}: rule {rule} block {ordinal}")
        i = hits[0]
        self.assertEqual(canonical(parsed[i]), lines[i], "the stored line is not in the form this plant writes")
        parsed[i]["outputs"] = change(parsed[i]["outputs"], parsed)
        lines[i] = canonical(parsed[i])
        self.world.write(os.path.join(BASELINE, corpus_file + ".jsonl"), "\n".join(lines) + "\n")

    # ------------------------------------------------------------------------------------------------ the tool

    def regen(self, *accept: str) -> tuple[int, str]:
        self.world.commit()
        argv = [support.pinned_python(), "-I", "-B", "-X", "utf8", os.path.join("gate", "baseline", "regen.py")]
        if accept:
            argv += ["--accept", " ".join(accept)]
        env = {**self.world.env, "VORSAR_CEP_TOOLS_CACHE": support.real_cache()}
        env["PATH"] = os.pathsep.join([go_bin(), env.get("PATH", "")])
        out, err = os.path.join(self.world.root, "regen.stdout"), os.path.join(self.world.root, "regen.stderr")
        code = support.run_process(argv, cwd=self.world.repo, env=env, out=out, err=err, cap_s=support.RUN_CAP_S)
        return code, support.read_text(out) + support.read_text(err)

    def assertRefused(self, cls: str) -> None:
        """Tier 1 green, the fresh run done, exactly the named class printed, exit 1, the baseline untouched."""
        before = self.snapshot()
        code, printed = self.regen()
        self.assertNotIn("refused: Tier 1 is red", printed)
        self.assertNotIn("refused: the fresh run failed", printed)
        self.assertEqual(classes_in(printed), {cls}, printed)
        self.assertEqual(code, 1, printed)
        self.assertEqual(self.snapshot(), before, "a refused regen writes nothing")

    # ------------------------------------------------------------------------------------------------ the tests

    def test_count_change_is_refused(self):
        # 10's block 2: a read-scope grant fails `scope == 'admin'`, so the fresh run emits nothing. The stored block
        # now holds one alert, block 1's (t1, alice, admin, 10:00): the old side had one output more. Tier 1 is
        # green, the corpus being untouched; the key and source_sha256 are unchanged, the header's format is 1, and
        # the number of outputs differs (1 stored, 0 fresh): count-change, the first row that matches.
        def one_more(outputs, parsed):
            self.assertEqual(outputs, [], "10's block 2 emits nothing")
            block_1 = [d for d in parsed[1:] if (d["rule"], d["block"]) == ("AdminScopeGrant", 1)]
            return [block_1[0]["outputs"][0]]

        self.replant(EX10, "AdminScopeGrant", 2, one_more)
        self.assertRefused("count-change")

    def test_ordering_change_is_refused(self):
        # 10's block 3 emits (t1, alice) 10:00, (t1, bob) 10:01, (t2, alice) 10:03 (the third alice grant is
        # suppressed). The stored block holds its first two the other way round, as an engine emitting a 10:01
        # alert before a 10:00 one would have recorded (the breaker's S7). Tier 1 green, the corpus untouched; same
        # key, same number of outputs, the same outputs as a multiset in another order: ordering-change, which
        # comes before timestamp-drift (each moved output's ts differs from its position's too).
        def swapped(outputs, _):
            self.assertEqual(len(outputs), 3, "10's block 3 emits three alerts")
            return [outputs[1], outputs[0], outputs[2]]

        self.replant(EX10, "AdminScopeGrant", 3, swapped)
        self.assertRefused("ordering-change")

    def test_timestamp_drift_is_refused(self):
        # 10's block 3's third alert, (t2, alice) at 10:03:00.000, alone in its partition, so no rate limit is near
        # it. The stored one is 1 ms later, 10:03:00.001, as an engine stamping the alert 1 ms late would have
        # recorded. Tier 1 green, the corpus untouched; same key, three outputs each side, not a permutation (the
        # moved alert is in no fresh output), and an output's ts differs: timestamp-drift.
        def drifted(outputs, _):
            self.assertEqual(len(outputs), 3, "10's block 3 emits three alerts")
            last = outputs[2]
            self.assertEqual((last["attrs"]["tenantId"], last["attrs"]["ts"]), ("t2", "2026-01-01T10:03:00.000Z"))
            last["attrs"]["ts"] = "2026-01-01T10:03:00.001Z"
            if "ts" in last:  # a top-level ts, written for an arrival-time stream, moves with it
                self.assertEqual(last["ts"], "2026-01-01T10:03:00.000Z")
                last["ts"] = "2026-01-01T10:03:00.001Z"
            return outputs

        self.replant(EX10, "AdminScopeGrant", 3, drifted)
        self.assertRefused("timestamp-drift")

    def test_additive_new_block_is_recorded(self):
        # 09 gains a second rule, StagingRisk, writing its own new stream StagingAlerts, with one block: a3's 0.9 in
        # staging passes `risk * 100 / 100 > 0.75 and env == 'staging'`, math:round(0.9) is 1.0, the rate limit has
        # nothing before it: one alert (a3, 1.0, 00:00:01), as its then-list says, so Tier 1 stays green. HighRisk's
        # four keys hold the file's pragma and params, the streams and tables HighRisk uses and HighRisk itself, none
        # of which changed (StagingAlerts is not one HighRisk uses): no verdict. The new block's key is new:
        # additive-new-block, recorded. TestBaseline counts one block more, so the world's blWantBlocks moves 15 -> 16,
        # as a developer adding a block would move it.
        stored_09 = self.stored_lines("09-params-and-quoting.cep")
        path = os.path.join(self.world.repo, EX09)
        text = support.read_text(path)
        self.assertEqual(text.count(LAST_LINE_09), 1, "the plant's anchor moved")
        self.world.write(EX09, text.replace(LAST_LINE_09, LAST_LINE_09 + STAGING_RULE))
        test = support.read_text(os.path.join(self.world.repo, BASELINE_TEST))
        # The constant is blWantBlocks (the file's bl prefix; fixture edit at T4, named in the record).
        want = re.compile(r"\bblWantBlocks(\s*)=(\s*)15\b")
        self.assertEqual(len(want.findall(test)), 1, "the wantBlocks anchor moved")
        self.world.write(BASELINE_TEST, want.sub(r"blWantBlocks\1=\g<2>16", test))
        before = self.snapshot()

        code, printed = self.regen()

        self.assertEqual(classes_in(printed), {"additive-new-block"}, printed)
        self.assertEqual(code, 0, printed)
        after = self.snapshot()
        name_09 = "09-params-and-quoting.cep.jsonl"
        self.assertEqual(
            {k: v for k, v in after.items() if k != name_09}, {k: v for k, v in before.items() if k != name_09}
        )
        lines = self.stored_lines("09-params-and-quoting.cep")
        self.assertEqual(len(lines), len(stored_09) + 1, "the header, HighRisk's four blocks and the new one")
        self.assertEqual(lines[1:-1], stored_09[1:], "HighRisk's blocks byte for byte")
        header, new = json.loads(lines[0]), json.loads(lines[-1])
        with open(path, "rb") as f:
            self.assertEqual(header["source_sha256"], hashlib.sha256(f.read()).hexdigest())
        self.assertEqual((new["rule"], new["block"], len(new["outputs"])), ("StagingRisk", 1, 1))
        self.assertEqual(new["counts"], {"errored": 0, "key_errors": 0, "late": 0, "panics": 0})
