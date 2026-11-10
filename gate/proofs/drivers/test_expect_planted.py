"""The conformance row's proofs (exit 2; design s3-3-2 §7, §5.4; QA T45, T46): a planted wrong expect is red, naming it.

The world holds the gate, the module (go.mod, go.sum and every package `go list -deps -test ./conformance` names in
the main module, each with the files directly in its directory) and the corpus, docs/language/examples/. Its clean
run passes; each plant is one anchored edit of example 09 or 07, its anchor found exactly once, and turns the row red
with E-EXP-001 and §5.4's text for its case in the row's test.json. Values in a text are AppendValue's: a float 1.0
is `1`, a string `"t1"`, a timestamp `"2026-01-01T10:00:00.000Z"`. A record's `{…}` is not pinned here, only the
text around it (the internal/playback tests own its form).

Every case below is worked out by hand from §5.4's rules:
- wrong value, 09 block 1: math:round(0.9) is 1.0; the then-record now says 2.0. n = m = 1, nothing matches, so not
  case 2; the alignment's first edit is then-record 1 with output 1 left (i < n), and `scaled` is the first
  attribute in written order that differs.
- missing output, 07 block 2: `crm` is registered, so nothing fires; the then-list now expects an alert. n = 0,
  m = 1: then-record 1 is the first edit with i == n.
- extra output, 07 block 5: the third grant is now tenant t3's, a new partition, so it fires: outputs t1@10:00,
  t2@10:02, t3@10:03 against then-records t1@10:00, t2@10:02. The first two pair; output 3 is the first edit.
- wrong order, 07 block 5: the two then-records swapped. n = m = 2; in order, then-record 1 takes output 2 and
  then-record 2 output 1, so case 2 with j = 1, k = 2. No one-line edit of 09 or 07 can do this: every block with
  two outputs has its then-list on two lines, and a watermark of 1 or 2 minutes keeps the outputs in `ts` order, so
  this one anchor spans the two then-record lines.
- an output under an empty then, 09 block 3: a risk of 0.76 crosses the 0.75 threshold under `then RiskAlerts [ ]`.
  n = 1, m = 0: output 1 is the first edit.
- wrong timestamp, 07 block 1: the alert is at 10:00:00; the then-record now says 10:00:01. As wrong value, with
  `ts` the first attribute that differs.
"""

import json
import os

from gate.proofs.drivers import support

EX07 = "docs/language/examples/07-consent-gate.cep"
EX09 = "docs/language/examples/09-params-and-quoting.cep"
EXAMPLES = "docs/language/examples"
PACKAGE = "github.com/vorsar/cep/conformance"
RED = f"exit 1: {PACKAGE}: TestCorpus"  # the sorted first failure: the parent test, before its sub-test
WANT_FILES = 12  # conformance/corpus_test.go's wantFiles: the ten examples and the two torture files
GO_ENV = {"GOTOOLCHAIN": "local", "GOENV": "off", "GOFLAGS": "", "GOWORK": "off", "CGO_ENABLED": "0"}
MAIN_DIRS = "{{if .Module}}{{if .Module.Main}}{{.Dir}}{{end}}{{end}}"

T1_1000 = "{tenantId: 't1', appId: 'rogue', ts: timestamp '2026-01-01T10:00:00Z'}"
T2_1002 = "{tenantId: 't2', appId: 'rogue', ts: timestamp '2026-01-01T10:02:00Z'}"
INDENT_07_36 = " " * 30  # 07's line 36, the second then-record of block 5


def go() -> str:
    tool = support.tool("go")
    return os.path.join(support.home(support.real_cache(), "go"), tool.path_dir, tool.command)


class ExpectPlanted(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate()
        self.copy_module()
        for rel in self.world.tracked(EXAMPLES):
            self.world.copy(rel)

    def copy_module(self) -> None:
        """go.mod, go.sum and every main-module package the conformance tests build, transitively."""
        self.world.copy("go.mod")
        self.world.copy("go.sum")
        out, err = os.path.join(self.world.root, "list.stdout"), os.path.join(self.world.root, "list.stderr")
        argv = [go(), "list", "-deps", "-test", "-f", MAIN_DIRS, "./conformance"]
        env = {**self.world.env, **GO_ENV}
        code = support.run_process(argv, cwd=support.REPO, env=env, out=out, err=err, cap_s=support.RUN_CAP_S)
        self.assertEqual(code, 0, support.read_text(err))
        repo = os.path.realpath(support.REPO)
        dirs = {os.path.relpath(os.path.realpath(d), repo) for d in support.read_text(out).split("\n") if d}
        self.assertIn("conformance", dirs)
        tracked = self.world.tracked(".")
        for d in sorted(dirs):
            here = "" if d == "." else d
            for rel in tracked:
                if os.path.dirname(rel) == here and (d == "conformance" or not rel.endswith("_test.go")):
                    self.world.copy(rel)

    def plant(self, rel: str, anchor: str, replacement: str) -> None:
        text = support.read_text(os.path.join(self.world.repo, rel))
        self.assertEqual(text.count(anchor), 1, "the plant's anchor moved")
        self.world.write(rel, text.replace(anchor, replacement))

    def run_conformance(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row("conformance")))

    def events(self, run: support.Run) -> list[dict]:
        text = run.row_file("test", "conformance", "test.json")
        return [json.loads(line) for line in text.split("\n") if line.startswith("{")]

    def assertDiverged(self, run: support.Run, corpus_file: str, rule: str, block: int, start: str, end: str):
        """The row red; only the planted file's sub-test failed; its E-EXP-001 line holds §5.4's text."""
        self.assertParityPassed(run, "test")
        self.assertRed(run, "test", "conformance", RED)
        events = self.events(run)
        failed = sorted({e["Test"] for e in events if e.get("Action") == "fail" and "Test" in e})
        self.assertEqual(failed, ["TestCorpus", f"TestCorpus/{corpus_file}"], run.stdout)
        output = "".join(e.get("Output", "") for e in events if e.get("Test") == f"TestCorpus/{corpus_file}")
        lines = [line.strip() for line in output.split("\n") if "E-EXP-001" in line]
        head = f"rule {rule}, expect block {block}: {start}"
        found = [line for line in lines if head in line and line.endswith(end)]
        self.assertEqual(len(found), 1, f"no E-EXP-001 line holding {head!r} … {end!r} in:\n{output}")

    def test_the_clean_corpus_passes(self):
        run = self.run_conformance()
        self.assertParityPassed(run, "test")
        # covered counts packages, as the unit row does (gotest.py, S3.3-2's build; QA's first form read 1 + 12 tests)
        self.assertRow(run, "test", "conformance", True, "covered 1")

    def test_wrong_value(self):
        self.plant(
            EX09,
            "scaled: 1.0, ts: timestamp '2026-01-01T00:00:01Z'} ]",
            "scaled: 2.0, ts: timestamp '2026-01-01T00:00:01Z'} ]",
        )
        run = self.run_conformance()
        self.assertDiverged(
            run,
            "09-params-and-quoting.cep",
            "HighRisk",
            1,
            "then-record 1 has no output: expected {",
            "}; the nearest output is 1: scaled is 1, expected 2",
        )

    def test_missing_output(self):
        self.plant(
            EX07,
            "then ConsentAlerts [ ]",
            "then ConsentAlerts [ {tenantId: 't1', appId: 'crm', ts: timestamp '2026-01-01T10:00:00Z'} ]",
        )
        run = self.run_conformance()
        self.assertDiverged(
            run,
            "07-consent-gate.cep",
            "UnknownAppConsent",
            2,
            "then-record 1 has no output: expected {",
            "}; the rule emitted 0",
        )

    def test_extra_output(self):
        self.plant(
            EX07,
            "{tenantId: 't1', appId: 'rogue', grantedBy: 'admin', ts: timestamp '2026-01-01T10:03:00Z'}",
            "{tenantId: 't3', appId: 'rogue', grantedBy: 'admin', ts: timestamp '2026-01-01T10:03:00Z'}",
        )
        run = self.run_conformance()
        self.assertDiverged(
            run,
            "07-consent-gate.cep",
            "UnknownAppConsent",
            5,
            "output 3 was not expected: {",
            "}; the then-list has 2",
        )

    def test_wrong_order(self):
        self.plant(
            EX07,
            f"{T1_1000},\n{INDENT_07_36}{T2_1002} ]",
            f"{T2_1002},\n{INDENT_07_36}{T1_1000} ]",
        )
        run = self.run_conformance()
        text = "the outputs are in another order: then-record 1 matches output 2"
        self.assertDiverged(run, "07-consent-gate.cep", "UnknownAppConsent", 5, text, text)

    def test_output_under_empty_then(self):
        self.plant(EX09, "risk: 0.75", "risk: 0.76")
        run = self.run_conformance()
        self.assertDiverged(
            run,
            "09-params-and-quoting.cep",
            "HighRisk",
            3,
            "output 1 was not expected: {",
            "}; the then-list is empty",
        )

    def test_wrong_timestamp(self):
        self.plant(
            EX07,
            f"then ConsentAlerts [ {T1_1000} ]",
            "then ConsentAlerts [ {tenantId: 't1', appId: 'rogue', ts: timestamp '2026-01-01T10:00:01Z'} ]",
        )
        run = self.run_conformance()
        self.assertDiverged(
            run,
            "07-consent-gate.cep",
            "UnknownAppConsent",
            1,
            "then-record 1 has no output: expected {",
            '}; the nearest output is 1: ts is "2026-01-01T10:00:00.000Z", expected "2026-01-01T10:00:01.000Z"',
        )

    def test_empty_corpus_is_red(self):
        # Every corpus file deleted, its directories kept: TestCorpus reads none and fails against wantFiles.
        removed = self.world.tracked(EXAMPLES)
        self.assertEqual(len([rel for rel in removed if rel.endswith(".cep")]), WANT_FILES)
        for rel in removed:
            os.remove(os.path.join(self.world.repo, rel))
        run = self.run_conformance()
        self.assertParityPassed(run, "test")
        self.assertRed(run, "test", "conformance", RED)
        events = self.events(run)
        ran = sorted({e["Test"] for e in events if e.get("Action") == "run" and "Test" in e})
        self.assertEqual(ran, ["TestCorpus"], "no corpus file, so no sub-test")
        output = "".join(e.get("Output", "") for e in events if e.get("Test") == "TestCorpus")
        self.assertIn(str(WANT_FILES), output)
