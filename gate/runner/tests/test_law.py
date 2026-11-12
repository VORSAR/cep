"""gate/runner/law.py (S1.1c-1 design §2.3): the manifest reader and the pure helpers.

Every expected value is worked out by hand from the design's rules and texts, never from a run of the code.
"""

import json
import unittest

from gate.runner import law

GOOD = {
    "schema": 1,
    "note": "n",
    "paths": [
        {"path": "gate/registry.json", "kind": "registry"},
        {"path": "gate/runner/", "kind": "code"},
        {"path": "docs/language/SPEC.md", "kind": "doc", "flag": "SPEC changed — owner decision required"},
        {"path": "docs/architecture/adr/", "kind": "doc", "flag": "an ADR changed — owner decision required"},
    ],
    "instruments": [],
}


def text(doc: dict) -> str:
    return json.dumps(doc, ensure_ascii=False, indent=2) + "\n"


class LoadManifest(unittest.TestCase):
    def test_a_valid_manifest(self):
        m = law.load_manifest(text(GOOD))
        self.assertEqual(
            m.paths,
            (
                law.Path("gate/registry.json", "registry"),
                law.Path("gate/runner/", "code"),
                law.Path("docs/language/SPEC.md", "doc", "SPEC changed — owner decision required"),
                law.Path("docs/architecture/adr/", "doc", "an ADR changed — owner decision required"),
            ),
        )

    def assertFault(self, doc_text: str, needle: str) -> None:
        with self.assertRaises(law.ManifestError) as caught:
            law.load_manifest(doc_text)
        self.assertIn(needle, str(caught.exception))
        self.assertTrue(str(caught.exception).startswith("gate/law.json: "), str(caught.exception))

    def test_a_duplicate_key_is_refused(self):
        self.assertFault('{"schema": 1, "note": "n", "paths": [], "paths": []}', "duplicate key")

    def test_an_extra_top_level_key_is_named(self):
        self.assertFault(text({**GOOD, "extra": []}), "extra")

    def test_a_missing_instruments_key_is_named(self):
        doc = {k: v for k, v in GOOD.items() if k != "instruments"}
        self.assertFault(text(doc), "instruments")

    def test_a_missing_note_is_named(self):
        doc = {k: v for k, v in GOOD.items() if k != "note"}
        self.assertFault(text(doc), "note")

    def test_a_flag_on_a_non_doc_is_refused(self):
        doc = {**GOOD, "paths": [{"path": "gate/registry.json", "kind": "registry", "flag": "x"}]}
        self.assertFault(text(doc), "paths[0]")

    def test_a_doc_without_a_flag_is_refused(self):
        doc = {**GOOD, "paths": [{"path": "docs/language/SPEC.md", "kind": "doc"}]}
        self.assertFault(text(doc), "paths[0]")

    def test_dot_dot_in_a_path_is_refused(self):
        doc = {**GOOD, "paths": [{"path": "gate/../tools.json", "kind": "tools"}]}
        self.assertFault(text(doc), "paths[0]")

    def test_a_leading_slash_is_refused(self):
        doc = {**GOOD, "paths": [{"path": "/tools.json", "kind": "tools"}]}
        self.assertFault(text(doc), "paths[0]")

    def test_an_unknown_kind_is_refused(self):
        doc = {**GOOD, "paths": [{"path": "tools.json", "kind": "manifest"}]}
        self.assertFault(text(doc), "paths[0]")

    def test_a_repeated_path_is_refused(self):
        doc = {**GOOD, "paths": [{"path": "tools.json", "kind": "tools"}, {"path": "tools.json", "kind": "tools"}]}
        self.assertFault(text(doc), "paths[1]")

    def test_an_empty_paths_list_is_refused(self):
        self.assertFault(text({**GOOD, "paths": []}), "paths")

    def test_a_wrong_schema_is_refused(self):
        self.assertFault(text({**GOOD, "schema": 2}), "schema")


class EntryFor(unittest.TestCase):
    def setUp(self):
        self.m = law.load_manifest(text(GOOD))

    def test_an_exact_path(self):
        self.assertEqual(law.entry_for(self.m, "gate/registry.json"), law.Path("gate/registry.json", "registry"))

    def test_a_file_under_a_prefix(self):
        self.assertEqual(law.entry_for(self.m, "gate/runner/rows/x.py"), law.Path("gate/runner/", "code"))

    def test_the_longest_prefix_wins(self):
        doc = {**GOOD, "paths": [{"path": "gate/", "kind": "code"}, {"path": "gate/runner/", "kind": "workflow"}]}
        m = law.load_manifest(text(doc))
        self.assertEqual(law.entry_for(m, "gate/runner/x.py"), law.Path("gate/runner/", "workflow"))
        self.assertEqual(law.entry_for(m, "gate/x.py"), law.Path("gate/", "code"))

    def test_an_unlisted_file_is_none(self):
        self.assertIsNone(law.entry_for(self.m, "gate/registry.json.bak"))
        self.assertIsNone(law.entry_for(self.m, "tools.json"))


class Names(unittest.TestCase):
    def test_six_items_are_listed_whole(self):
        self.assertEqual(law.names(["a", "b", "c", "d", "e", "f"]), "a, b, c, d, e, f")

    def test_seven_items_show_three_and_an_ellipsis(self):
        self.assertEqual(law.names(["a", "b", "c", "d", "e", "f", "g"]), "a, b, c, …")

    def test_one_item(self):
        self.assertEqual(law.names(["a"]), "a")


class Line(unittest.TestCase):
    def test_with_a_one_liner(self):
        self.assertEqual(
            law.line("gate/current-sprint", 1, 1, "marker S1.1c-1 → S4.1"),
            "gate/current-sprint +1/−1 — marker S1.1c-1 → S4.1",
        )

    def test_without_a_one_liner(self):
        self.assertEqual(law.line("gate/run.py", 3, 0, ""), "gate/run.py +3/−0")


class Block(unittest.TestCase):
    def test_nothing_is_none(self):
        self.assertEqual(law.block("abc1234", [], {}), ["LAW Δ: none"])

    def test_data_lines_then_grouped_lines(self):
        data = ["gate/current-sprint +1/−1 — marker S1.1c-1 → S4.1", "tools.json +2/−2 — go: 1.27.3 → 1.27.4"]
        grouped = {"gate/runner/": (7, 120, 40), ".github/workflows/": (1, 3, 3)}
        self.assertEqual(
            law.block("abc1234", data, grouped),
            [
                "LAW Δ (10 files, vs abc1234)",
                "gate/current-sprint +1/−1 — marker S1.1c-1 → S4.1",
                "tools.json +2/−2 — go: 1.27.3 → 1.27.4",
                "gate/runner/: 7 files +120/−40",
                ".github/workflows/: 1 file +3/−3",
            ],
        )

    def test_grouped_only(self):
        self.assertEqual(
            law.block("abc1234", [], {"gate/run.py": (1, 1, 1)}),
            ["LAW Δ (1 file, vs abc1234)", "gate/run.py: 1 file +1/−1"],
        )


# The parsers, one per kind, on excerpt pairs of the real files (S1.1c-1 design §2.3; every item form).

EDGES_OLD = (
    '{"schema": 1, "module": "github.com/vorsar/cep", "note": "n",\n'
    ' "imports": {"internal/event": [], "internal/model": ["internal/event"]},\n'
    ' "external": {"internal/text": ["golang.org/x/text/"]},\n'
    ' "fences": {"internal/expr": {"internal/model": ["Expr", "Lit"]}},\n'
    ' "test_imports": {"internal/validate": ["internal/parser"]}, "test_fences": {},\n'
    ' "test_external": {"internal/runtime": ["go.uber.org/goleak"]}}\n'
)
REGISTRY_OLD = (
    '{"schema": 1, "rows": [{"id": "gofmt", "job": "lint", "tiers": ["T0", "T1"], "blocking": true,\n'
    ' "timeout_s": 60, "needs": [], "scope": ["prod"], "expected_scope": ["git", "x"], "min_wants": 0,\n'
    ' "lands_in": "S1.1a-1", "exclusive": false, "command": ["true"]}]}\n'
)
REGISTRY_ROW = (
    '{"id": "notbuilt", "job": "test", "tiers": ["T4"], "blocking": true, "timeout_s": 60, "needs": [],'
    ' "scope": ["tools"], "expected_scope": ["git", "conformance/notbuilt.json"], "min_wants": 1,'
    ' "lands_in": "S1.1c-1", "exclusive": false, "command": ["python3.15"]}'
)
EXPORTS_OLD = (
    "cep BuildInfo func func BuildInfo() Info\n"
    "cep Config type type Config struct {\n"
    "\tShards      int\n"
    "\tMailbox     int\n"
    "}\n"
    "clock BatchClose const const BatchClose TimerKind = iota + 1 // iota 3\n"
)
DEPS_OLD = (
    '{"schema": 1, "note": "n", "modules": [{"module": "github.com/alecthomas/participle/v2", "version": "v2.1.4",'
    ' "scope": "runtime", "license": "MIT", "approved": "2026-11-04", "why": "the engine parser"}]}\n'
)
CEILINGS_OLD = (
    '{"outbox": {"ceiling": 65536, "blocking": true}, "drained_pending": {"ceiling": 0, "rules": {"qa-eq": 8}},\n'
    ' "partial-count": {"ceiling": "none", "reason": "no pattern state until S4.1"}}\n'
)
BASELINE_HEAD = (
    '{"file":"07-consent-gate.cep","format":1,'
    '"source_sha256":"e24f0cb6fd68624f10afcaa50f3e0bb90d4cafcb83d9b8b3fe05a1c4771c11d3","spec_rev":10}\n'
)
BASELINE_BLOCK = (
    '{"block":1,"counts":{"errored":0,"key_errors":0,"late":0,"panics":0},"key":"8972e3c3e21a812f82dfd6051009e8dd77c48745655fd1a0f05277d3d5d6faa3",'
    '"outputs":[{"attrs":{"appId":"rogue","tenantId":"t1","ts":"2026-01-01T10:00:00.000Z"},"stream":"ConsentAlerts","ts":"2026-01-01T10:00:00.000Z"}],"rule":"UnknownAppConsent"}\n'
)
NOTBUILT_OLD = (
    '{"schema": 1, "note": "n", "entries": ['
    '{"file": "01-agent-kill-switch.cep", "construct": "a pattern body", "sprint": "S4.1"},\n'
    ' {"file": "08-session-flow.cep", "construct": "a pattern body", "sprint": "S4.1"}]}\n'
)
TOOLS_OLD = (
    '{"schema": 1, "tools": [{"command": "go", "name": "go", "path_dir": "bin", "version": "1.27.3",\n'
    ' "platforms": {"darwin-arm64": {"archive_sha256": "aa", "exec_sha256": {"bin/go": "bb"}}}},\n'
    ' {"command": "ruff", "name": "ruff", "path_dir": "", "version": "0.17.3", "platforms": {}}]}\n'
)
SPRINTS_OLD = (
    '{"schema": 1, "note": "n", "rows": [{"id": "S1.1a-1", "closed": true}, {"id": "S1.1c", "closed": false},'
    ' {"id": "S4.1", "closed": false}]}\n'
)
LAW_OLD = (
    '{"schema": 1, "note": "n", "paths": [{"path": "gate/run.py", "kind": "code"},\n'
    ' {"path": "docs/language/SPEC.md", "kind": "doc", "flag": "SPEC changed — owner decision required"}]}\n'
)


class Delta(unittest.TestCase):
    """Each case: (kind, old text, new text, the exact one-liner)."""

    def check(self, kind: str, cases: list[tuple[str | None, str | None, str]]) -> None:
        for i, (old, new, want) in enumerate(cases):
            with self.subTest(kind=kind, case=i):
                self.assertEqual(law.delta(kind, old, new), want)

    def test_edges(self):
        self.check(
            "edges",
            [
                (
                    EDGES_OLD,
                    EDGES_OLD.replace(
                        '"internal/event": [],', '"internal/event": [], "internal/planted": ["internal/event"],'
                    ),
                    "+1 edge internal/planted → internal/event",
                ),
                (
                    EDGES_OLD,
                    EDGES_OLD.replace('"internal/event": [],', '"internal/event": [], "a": ["b", "c"],'),
                    "+2 edges (a → b, a → c)",
                ),
                (
                    EDGES_OLD,
                    EDGES_OLD.replace('"internal/model": ["internal/event"]', '"internal/model": []'),
                    "−1 edge internal/model → internal/event",
                ),
                (
                    EDGES_OLD,
                    EDGES_OLD.replace('["Expr", "Lit"]', '["Expr", "Lit", "TextOperand"]'),
                    "fence internal/expr ← internal/model: Expr, Lit → Expr, Lit, TextOperand",
                ),
                (
                    EDGES_OLD,
                    EDGES_OLD.replace('"github.com/vorsar/cep"', '"example.com/cep"').replace(
                        '"note": "n"', '"note": "m"'
                    ),
                    "module github.com/vorsar/cep → example.com/cep; note changed",
                ),
                (
                    EDGES_OLD,
                    EDGES_OLD.replace(
                        '"test_imports": {"internal/validate": ["internal/parser"]}', '"test_imports": {}'
                    ),
                    "−1 edge internal/validate →test internal/parser",
                ),
                (
                    EDGES_OLD,
                    EDGES_OLD.replace('"internal/text": ["golang.org/x/text/"]', '"internal/text": []'),
                    "−1 edge internal/text → ext:golang.org/x/text/",
                ),
                (
                    EDGES_OLD,
                    EDGES_OLD.replace('"internal/runtime": ["go.uber.org/goleak"]', '"internal/runtime": []'),
                    "−1 edge internal/runtime →test ext:go.uber.org/goleak",
                ),
                (EDGES_OLD, "{not json", "unparseable (new)"),
                ("{not json", EDGES_OLD, "unparseable (old)"),
                (None, EDGES_OLD, "file added"),
                (EDGES_OLD, None, "file removed"),
            ],
        )

    def test_registry(self):
        self.check(
            "registry",
            [
                (
                    REGISTRY_OLD,
                    REGISTRY_OLD.replace('"command": ["true"]}]}', '"command": ["true"]}, ' + REGISTRY_ROW + "]}"),
                    "+1 row notbuilt (T4, test)",
                ),
                (REGISTRY_OLD, '{"schema": 1, "rows": []}\n', "−1 row gofmt"),
                (
                    REGISTRY_OLD,
                    REGISTRY_OLD.replace('["T0", "T1"]', '["T0", "T1", "T2"]').replace(
                        '"timeout_s": 60', '"timeout_s": 90'
                    ),
                    "row gofmt: tiers T0, T1 → T0, T1, T2; timeout_s 60 → 90",
                ),
                (
                    REGISTRY_OLD,
                    REGISTRY_OLD.replace('"blocking": true', '"blocking": false'),
                    "row gofmt: blocking true → false",
                ),
                (REGISTRY_OLD, '{"schema": 1, "rows": [], "rows": []}\n', "unparseable (new)"),
            ],
        )

    def test_exports(self):
        self.check(
            "exports",
            [
                (
                    EXPORTS_OLD,
                    EXPORTS_OLD.replace("\tMailbox     int\n", "\tMailbox     int64\n"),
                    "1 signature changed (cep.Config)",
                ),
                (EXPORTS_OLD, EXPORTS_OLD + "cep New func func New() *Config\n", "+1 export (cep.New)"),
                (
                    EXPORTS_OLD,
                    EXPORTS_OLD.replace("clock BatchClose const const BatchClose TimerKind = iota + 1 // iota 3\n", ""),
                    "−1 export (clock.BatchClose)",
                ),
                (
                    EXPORTS_OLD,
                    EXPORTS_OLD.replace(
                        "cep BuildInfo func func BuildInfo() Info\n", "cep BuildInfo func func BuildInfo() *Info\n"
                    ).replace("}\n", "}\ncep A func func A()\ncep B func func B()\n"),
                    "+2 exports (cep.A, cep.B); 1 signature changed (cep.BuildInfo)",
                ),
            ],
        )

    def test_deps(self):
        self.check(
            "deps",
            [
                (
                    DEPS_OLD,
                    DEPS_OLD.replace('"version": "v2.1.4"', '"version": "v2.2.0"'),
                    "github.com/alecthomas/participle/v2: v2.1.4 → v2.2.0",
                ),
                (
                    DEPS_OLD,
                    DEPS_OLD.replace('"license": "MIT"', '"license": "ISC"'),
                    "github.com/alecthomas/participle/v2: license MIT → ISC",
                ),
                (
                    DEPS_OLD,
                    DEPS_OLD.replace(
                        '"why": "the engine parser"}',
                        '"why": "the engine parser"}, {"module": "example.com/x", "version": "v1.0.0", "scope": "s",'
                        ' "license": "MIT", "approved": "d", "why": "w"}',
                    ),
                    "+1 module example.com/x v1.0.0 (MIT)",
                ),
                (
                    DEPS_OLD,
                    '{"schema": 1, "note": "m", "modules": []}\n',
                    "−1 module github.com/alecthomas/participle/v2; note changed",
                ),
            ],
        )

    def test_ceilings(self):
        self.check(
            "ceilings",
            [
                (
                    CEILINGS_OLD,
                    CEILINGS_OLD.replace('"ceiling": 65536, "blocking": true', '"ceiling": 32768, "blocking": false'),
                    "outbox: ceiling 65536 → 32768; outbox: blocking true → false",
                ),
                (CEILINGS_OLD, CEILINGS_OLD.replace('"qa-eq": 8', '"qa-eq": 4'), "drained_pending[qa-eq]: 8 → 4"),
                (
                    CEILINGS_OLD,
                    CEILINGS_OLD.replace('"reason": "no pattern state until S4.1"', '"reason": "none yet"'),
                    "partial-count: reason changed",
                ),
                (
                    CEILINGS_OLD,
                    CEILINGS_OLD.replace('{"outbox"', '{"sink-depth": {"ceiling": 1024}, "outbox"'),
                    "+metric sink-depth",
                ),
                (
                    CEILINGS_OLD,
                    CEILINGS_OLD.replace('"outbox": {"ceiling": 65536, "blocking": true}, ', ""),
                    "−metric outbox",
                ),
            ],
        )

    def test_baseline(self):
        old = BASELINE_HEAD + BASELINE_BLOCK
        second = (
            BASELINE_BLOCK.replace('"block":1', '"block":2')
            .replace('"key":"8972', '"key":"ffff')
            .replace('"rule":"UnknownAppConsent"', '"rule":"Other"')
        )
        self.check(
            "baseline",
            [
                (
                    old,
                    BASELINE_HEAD + BASELINE_BLOCK.replace('"stream":"ConsentAlerts"', '"stream":"ConsentAlerts2"'),
                    "1 block re-recorded (rule UnknownAppConsent, block 1)",
                ),
                (old, old + second, "+1 block (rule Other, block 2)"),
                (old + second, old, "−1 block (rule Other, block 2)"),
                (
                    old,
                    old.replace('"spec_rev":10', '"spec_rev":11').replace(
                        '"source_sha256":"e24f', '"source_sha256":"0000'
                    ),
                    "source changed; spec_rev 10 → 11",
                ),
                (old, old.replace('"format":1', '"format":2'), "format 1 → 2"),
                (None, old, "file added: 1 blocks"),
                (old, None, "file removed"),
                (old, "not jsonl\n", "unparseable (new)"),
            ],
        )

    def test_notbuilt(self):
        self.check(
            "notbuilt",
            [
                (
                    NOTBUILT_OLD,
                    NOTBUILT_OLD.replace(
                        '"sprint": "S4.1"}]}',
                        '"sprint": "S4.1"}, {"file": "x.cep", "construct": "c", "sprint": "S7"}]}',
                    ),
                    "+1 entry x.cep (S7)",
                ),
                (
                    NOTBUILT_OLD,
                    NOTBUILT_OLD.replace(
                        ',\n {"file": "08-session-flow.cep", "construct": "a pattern body", "sprint": "S4.1"}', ""
                    ),
                    "−1 entry 08-session-flow.cep",
                ),
                (
                    NOTBUILT_OLD,
                    NOTBUILT_OLD.replace(
                        '"construct": "a pattern body", "sprint": "S4.1"},\n',
                        '"construct": "a pattern body", "sprint": "S4.2"},\n',
                    ),
                    "01-agent-kill-switch.cep: S4.1 → S4.2",
                ),
                (
                    NOTBUILT_OLD,
                    NOTBUILT_OLD.replace(
                        '{"file": "08-session-flow.cep", "construct": "a pattern body"',
                        '{"file": "08-session-flow.cep", "construct": "a window"',
                    ),
                    "08-session-flow.cep: construct changed",
                ),
            ],
        )

    def test_tools(self):
        self.check(
            "tools",
            [
                (TOOLS_OLD, TOOLS_OLD.replace('"version": "1.27.3"', '"version": "1.27.4"'), "go: 1.27.3 → 1.27.4"),
                (TOOLS_OLD, TOOLS_OLD.replace('"bin/go": "bb"', '"bin/go": "cc"'), "go: hashes changed"),
                (
                    TOOLS_OLD,
                    TOOLS_OLD.replace(
                        ',\n {"command": "ruff", "name": "ruff", "path_dir": "", "version": "0.17.3", "platforms": {}}',
                        "",
                    ),
                    "−tool ruff",
                ),
                (
                    TOOLS_OLD,
                    TOOLS_OLD.replace(
                        '"platforms": {}}]}',
                        '"platforms": {}}, {"command": "x", "name": "x", "path_dir": "", "version": "1",'
                        ' "platforms": {}}]}',
                    ),
                    "+tool x",
                ),
            ],
        )

    def test_marker(self):
        self.check("marker", [("S1.1c-1\n", "S4.1\n", "marker S1.1c-1 → S4.1")])

    def test_sprints(self):
        self.check(
            "sprints",
            [
                (
                    SPRINTS_OLD,
                    SPRINTS_OLD.replace('{"id": "S1.1c", "closed": false}', '{"id": "S1.1c", "closed": true}'),
                    "S1.1c closed",
                ),
                (
                    SPRINTS_OLD,
                    SPRINTS_OLD.replace('{"id": "S1.1a-1", "closed": true}', '{"id": "S1.1a-1", "closed": false}'),
                    "S1.1a-1 reopened",
                ),
                (
                    SPRINTS_OLD,
                    SPRINTS_OLD.replace(
                        '{"id": "S4.1", "closed": false}]',
                        '{"id": "S4.1", "closed": false}, {"id": "S4.2", "closed": false}]',
                    ),
                    "+1 row S4.2 at 3",
                ),
                (SPRINTS_OLD, SPRINTS_OLD.replace(', {"id": "S4.1", "closed": false}', ""), "−1 row S4.1"),
                (
                    SPRINTS_OLD,
                    SPRINTS_OLD.replace(
                        '{"id": "S1.1c", "closed": false}, {"id": "S4.1", "closed": false}',
                        '{"id": "S4.1", "closed": false}, {"id": "S1.1c", "closed": false}',
                    ),
                    "order changed",
                ),
            ],
        )

    def test_law(self):
        self.check(
            "law",
            [
                (
                    LAW_OLD,
                    LAW_OLD.replace(
                        '{"path": "gate/run.py", "kind": "code"},',
                        '{"path": "gate/run.py", "kind": "code"}, {"path": "tools.json", "kind": "tools"},',
                    ),
                    "+1 path tools.json (tools)",
                ),
                (LAW_OLD, LAW_OLD.replace('{"path": "gate/run.py", "kind": "code"},\n', ""), "−1 path gate/run.py"),
                (LAW_OLD, LAW_OLD.replace('"kind": "code"', '"kind": "workflow"'), "gate/run.py: kind code → workflow"),
                (
                    LAW_OLD,
                    LAW_OLD.replace("SPEC changed — owner decision required", "the spec changed"),
                    "docs/language/SPEC.md: flag changed",
                ),
            ],
        )

    def test_code_and_workflow_have_no_one_liner(self):
        self.check("code", [("a\n", "b\n", "")])
        self.check("workflow", [("a\n", "b\n", "")])


class DocLine(unittest.TestCase):
    FLAG = "SPEC changed — owner decision required"
    SPEC = "# SPEC\n\n## 8. Other\n\ntext\n\n## 9. Canonical form\n\nold line\n\nmore\n"

    def test_a_change_under_a_numbered_heading(self):
        self.assertEqual(
            law.doc_line(self.SPEC, self.SPEC.replace("old line", "new line"), self.FLAG), "§9 — " + self.FLAG
        )

    def test_a_change_under_a_text_heading(self):
        adr = "# ADR\n\n## Decision\n\nwe do x\n"
        self.assertEqual(
            law.doc_line(adr, adr.replace("we do x", "we do y"), "an ADR changed — owner decision required"),
            "Decision — an ADR changed — owner decision required",
        )

    def test_two_hunks_under_two_headings_name_both(self):
        # S1.1c-2 design §4.5, §11.13: the first three distinct headings replace v1's "(first of <n> hunks)".
        new = self.SPEC.replace("text", "text changed").replace("more", "more changed")
        self.assertEqual(law.doc_line(self.SPEC, new, self.FLAG), "§8, §9 — " + self.FLAG)

    def test_no_heading_above_the_change(self):
        self.assertEqual(law.doc_line("a\nb\n", "a\nc\n", self.FLAG), self.FLAG)

    def test_a_pure_deletion_reads_the_old_text(self):
        self.assertEqual(law.doc_line(self.SPEC, self.SPEC.replace("\nmore\n", "\n"), self.FLAG), "§9 — " + self.FLAG)

    def test_added_and_removed_files(self):
        self.assertEqual(law.doc_line(None, self.SPEC, self.FLAG), "(file added) — " + self.FLAG)
        self.assertEqual(law.doc_line(self.SPEC, None, self.FLAG), "(file removed) — " + self.FLAG)


if __name__ == "__main__":
    unittest.main()
