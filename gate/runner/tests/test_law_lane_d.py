"""law.py's lane D additions (S1.1c-2 design §4.2, §4.5, §5 lane D): the `gomod` kind, `doc_line`'s three headings
and marker hunks, the `instruments` key of the manifest and its LAW Δ lines.

Every expected text is copied from the design (v2 §4.5; v1 §4.2, §4.6) by hand.

design ambiguity: where `(and <k> more)` and `+<n> markers` sit relative to the flag; the plan check's example is
`§5.2, §6.1, §9 (and 1 more)`, so the headings part is asserted as the line's start and `+<n> markers` by inclusion.
design ambiguity: numbered sub-headings (`5.2 x`) as `§5.2`; these fixtures use whole-number headings only.
design ambiguity: whether `// indirect` shows in `+require`; asserted here without it.
design ambiguity: an `instrument <id>: <field> <old> → <new>` line for a list field; asserted for scalar fields only.
"""

import json
import unittest

from gate.runner import law

FLAG = "SPEC changed — owner decision required"
SECTIONS = "".join(f"## {n}. S{n}\n\nold {n}\n\n" for n in (5, 6, 9, 10))


class GoMod(unittest.TestCase):
    BASE = (
        "module example.com/m\n\ngo 1.27.0\n\ntoolchain go1.27.3\n\n"
        "require example.com/single v1.0.0\n\n"
        "require (\n\texample.com/a v1.0.0\n\texample.com/b v1.2.0 // indirect\n)\n\n"
        "tool example.com/t/cmd\n"
    )

    def d(self, new: str) -> str:
        return law.delta("gomod", self.BASE, new)

    def test_gomod_is_a_kind(self):
        self.assertIn("gomod", law.KINDS)

    def test_unchanged(self):
        self.assertEqual(self.d(self.BASE), "")

    def test_a_single_require_bumped(self):
        self.assertEqual(
            self.d(self.BASE.replace("single v1.0.0", "single v1.1.0")), "example.com/single v1.0.0 → v1.1.0"
        )

    def test_a_block_require_bumped(self):
        self.assertEqual(self.d(self.BASE.replace("a v1.0.0", "a v2.0.0")), "example.com/a v1.0.0 → v2.0.0")

    def test_an_indirect_require_bumped(self):
        self.assertEqual(self.d(self.BASE.replace("b v1.2.0", "b v1.3.0")), "example.com/b v1.2.0 → v1.3.0")

    def test_a_require_added_in_the_block(self):
        new = self.BASE.replace(
            "\texample.com/a v1.0.0\n", "\texample.com/a v1.0.0\n\texample.com/c v0.1.0 // indirect\n"
        )
        self.assertEqual(self.d(new), "+require example.com/c v0.1.0")

    def test_a_require_removed(self):
        self.assertEqual(
            self.d(self.BASE.replace("require example.com/single v1.0.0\n\n", "")), "−require example.com/single"
        )

    def test_a_replace_added(self):
        self.assertEqual(self.d(self.BASE + "\nreplace example.com/a => ../a\n"), "+replace example.com/a => ../a")

    def test_a_tool_added(self):
        new = self.BASE + "tool golang.org/x/vuln/cmd/govulncheck\n"
        self.assertEqual(self.d(new), "+tool golang.org/x/vuln/cmd/govulncheck")

    def test_the_toolchain_line(self):
        self.assertEqual(self.d(self.BASE.replace("go1.27.3", "go1.27.4")), "toolchain go1.27.3 → go1.27.4")

    def test_the_go_line(self):
        self.assertEqual(self.d(self.BASE.replace("go 1.27.0", "go 1.27.1")), "go 1.27.0 → 1.27.1")


class DocLine(unittest.TestCase):
    def test_four_sections_name_three_and_one_more(self):
        new = SECTIONS
        for n in (5, 6, 9, 10):
            new = new.replace(f"old {n}\n", f"new {n}\n")
        got = law.doc_line(SECTIONS, new, FLAG)
        self.assertTrue(got.startswith("§5, §6, §9 (and 1 more)"), got)
        self.assertIn(FLAG, got)
        self.assertNotIn("first of", got)

    def test_two_hunks_under_one_heading_name_it_once(self):
        old = "## 8. Eight\n\na\n\nb\n\nc\n\nd\n"
        got = law.doc_line(old, old.replace("a\n", "A\n").replace("d\n", "D\n"), FLAG)
        self.assertEqual(got, "§8 — " + FLAG)

    def test_a_heading_inside_a_fence_is_not_a_heading(self):
        old = "## 7. Seven\n\n```\n# not a heading\n```\n\nold line\n"
        self.assertEqual(law.doc_line(old, old.replace("old line", "new line"), FLAG), "§7 — " + FLAG)

    def test_marker_only_hunks_are_counted_not_named(self):
        old = "## 3. Three\n\nrule one.\n\n## 4. Four\n\nrule two.\n\nrule three.\n"
        new = (
            old.replace("rule one.", "rule one, corrected.")
            .replace("rule two.", "rule two. <!--cep:s4-r2-->")
            .replace("rule three.", "rule three. <!--cep:s4-r3-->")
        )
        got = law.doc_line(old, new, FLAG)
        self.assertTrue(got.startswith("§3"), got)
        self.assertNotIn("§4", got)
        self.assertIn("+2 markers", got)

    def test_the_marker_regex(self):
        for text in ("<!--cep:s4-r2-->", "<!--cep-->", "<!--text-->"):
            with self.subTest(text=text):
                self.assertTrue(law.MARKER_COMMENT.search(f"a rule {text} here"))
        self.assertIsNone(law.MARKER_COMMENT.search("a rule <!-- a plain comment --> here"))


INSTRUMENT_ROW = {
    "id": "docsync",
    "row": "docsync",
    "lands_in": "S1.1c-2",
    "code": ["gate/runner/rows/docsync.py"],
    "proofs": [{"path": "gate/proofs/drivers/test_docsync_planted.py", "cases": 2}],
    "tiers": ["T2", "T3", "T4"],
    "min_wants": 1,
    "expected_scope": ["go", "./..."],
}
INSTRUMENT_GO = {
    "id": "docsync-d",
    "row": "unit",
    "lands_in": "S1.1c-2",
    "code": ["internal/parser/errorcounts_test.go"],
    "proofs": [{"path": "internal/parser/errorcounts_test.go", "cases": 1, "test": "TestErrorCountsMatchTheCatalogue"}],
    "go_test": "internal/parser.TestErrorCountsMatchTheCatalogue",
}
PATHS = [{"path": "gate/runner/", "kind": "code"}]


def manifest(*instruments: dict) -> str:
    return json.dumps({"schema": 1, "note": "n", "paths": PATHS, "instruments": list(instruments)})


class Instruments(unittest.TestCase):
    def test_both_shapes_load(self):
        m = law.load_manifest(manifest(INSTRUMENT_ROW, INSTRUMENT_GO))
        row, go = m.instruments
        self.assertEqual((row.id, row.tiers, row.min_wants, row.go_test), ("docsync", ("T2", "T3", "T4"), 1, ""))
        self.assertEqual(row.proofs, (law.Proof("gate/proofs/drivers/test_docsync_planted.py", 2),))
        self.assertEqual((go.row, go.go_test, go.tiers, go.min_wants), ("unit", INSTRUMENT_GO["go_test"], (), -1))
        self.assertEqual(go.proofs[0].test, "TestErrorCountsMatchTheCatalogue")

    def test_an_empty_list_loads(self):
        self.assertEqual(law.load_manifest(manifest()).instruments, ())

    def test_proofs_and_exempt_together_are_refused(self):
        both = dict(INSTRUMENT_ROW, exempt="docsync")
        with self.assertRaises(law.ManifestError) as cm:
            law.load_manifest(manifest(both))
        self.assertTrue(str(cm.exception).startswith("gate/law.json: instruments[0]"), str(cm.exception))

    def law_text(self, *instruments: dict, with_key: bool = True) -> str:
        data = {"schema": 1, "note": "n", "paths": PATHS}
        if with_key:
            data["instruments"] = list(instruments)
        return json.dumps(data)

    def test_an_instrument_added_to_an_old_side_without_the_key(self):
        self.assertEqual(
            law.delta("law", self.law_text(with_key=False), self.law_text(INSTRUMENT_ROW)), "+instrument docsync"
        )

    def test_an_instrument_removed(self):
        self.assertEqual(law.delta("law", self.law_text(INSTRUMENT_ROW), self.law_text()), "−instrument docsync")

    def test_an_instrument_field_changed(self):
        lowered = dict(INSTRUMENT_ROW, min_wants=0)
        self.assertEqual(
            law.delta("law", self.law_text(INSTRUMENT_ROW), self.law_text(lowered)),
            "instrument docsync: min_wants 1 → 0",
        )


if __name__ == "__main__":
    unittest.main()
