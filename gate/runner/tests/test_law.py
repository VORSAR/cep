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
        self.assertFault(text({**GOOD, "instruments": []}), "instruments")

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
                ".github/workflows/: 1 files +3/−3",
            ],
        )

    def test_grouped_only(self):
        self.assertEqual(
            law.block("abc1234", [], {"gate/run.py": (1, 1, 1)}),
            ["LAW Δ (1 files, vs abc1234)", "gate/run.py: 1 files +1/−1"],
        )


if __name__ == "__main__":
    unittest.main()
