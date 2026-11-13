"""pin_tests.json's loader and its unmeasured pins (S1.1c-4 design §4.5; review DQ-S8; lane F1b's brief): the file lane
F1 writes is `{"schema": 1, "note": …, "entries": [{"id", "row", "pattern", "measured"}, …]}`; `load_pin_tests`
gives id → pattern and refuses a malformed file by name; `unmeasured` gives the ids marked "unmeasured", which the
sort names as an unmeasured pin wherever one of them fails. Expected values written by hand from the brief."""

import json
import unittest

from gate.canary.candidate import CanaryError

PIN = "gate.toolchain.tests.test_manifest.RealPins.test_ruff_version"
GO = "gate.toolchain.tests.test_manifest.RealPins.test_go_version"


def entry(id_: str, pattern: str, measured: str = "2026-11-13, ruff 0.17.5 + python build 20261110") -> dict:
    return {"id": id_, "row": "unit", "pattern": pattern, "measured": measured}


def doc(*entries: dict, **top: object) -> dict:
    out: dict = {"schema": 1, "note": "the pin reds of a full candidate", "entries": list(entries)}
    out.update(top)
    return out


GOOD = doc(
    entry(PIN, r"0\.17\.3"),
    entry("census.md", r"^census\.md: ruff 0\.17\.3", measured="2026-11-13, ruff 0.17.5"),
    entry(GO, r"go1\.27\.1", measured="unmeasured"),
)


def text(obj: object) -> str:
    return json.dumps(obj)


def record(row: str, failures: list[list[str]]) -> dict:
    # verdict.decide's reason for a row red by its own exit code; any other reason is a break (break-it BK-S3).
    return {"id": row, "job": "test", "verdict": "fail", "reason": "exit 1: x", "failures": failures}


class LoadPinTests(unittest.TestCase):
    def setUp(self) -> None:
        from gate.canary import sort

        self.sort = sort

    def refused(self, raw: str, *words: str) -> None:
        with self.assertRaises(CanaryError) as caught:
            self.sort.load_pin_tests(raw)
        message = str(caught.exception)
        self.assertTrue(message.startswith("pin_tests.json: "), message)
        for word in words:
            self.assertIn(word, message)

    def test_a_good_file_gives_id_to_pattern(self):
        got = self.sort.load_pin_tests(text(GOOD))
        self.assertEqual(got, {"census.md": r"^census\.md: ruff 0\.17\.3", GO: r"go1\.27\.1", PIN: r"0\.17\.3"})

    def test_not_json_is_refused(self):
        self.refused('{"schema": 1,', "not JSON")

    def test_not_an_object_is_refused(self):
        self.refused("[]", "not a JSON object")

    def test_a_duplicate_json_key_is_refused(self):
        self.refused('{"schema": 1, "schema": 1, "note": "", "entries": []}', "schema", "twice")

    def test_wrong_schema_is_refused(self):
        self.refused(text(doc(schema=2)), "schema", "2")

    def test_schema_true_is_not_1(self):
        self.refused(text(doc(schema=True)), "schema", "true")

    def test_a_missing_top_key_is_refused(self):
        bad = doc()
        del bad["entries"]
        self.refused(text(bad), "missing", "entries")

    def test_an_unknown_top_key_is_refused(self):
        self.refused(text(doc(extra=1)), "unknown", "extra")

    def test_entries_not_a_list_is_refused(self):
        self.refused(text(doc(entries={})), "entries", "not a list")

    def test_an_entry_not_an_object_is_refused(self):
        self.refused(text(doc("x")), "entry 1", "not a JSON object")

    def test_a_missing_entry_key_is_refused_by_name(self):
        bad = entry(PIN, r"0\.17\.3")
        del bad["pattern"]
        self.refused(text(doc(bad)), PIN, "missing", "pattern")

    def test_an_unknown_entry_key_is_refused(self):
        bad = entry(PIN, r"0\.17\.3")
        bad["file"] = "x"
        self.refused(text(doc(bad)), PIN, "unknown", "file")

    def test_a_non_string_value_is_refused(self):
        bad = entry(PIN, r"0\.17\.3")
        bad["row"] = 3
        self.refused(text(doc(bad)), PIN, "row", "not a non-empty string")

    def test_an_empty_id_is_refused(self):
        self.refused(text(doc(entry("", "x"))), "entry 1", "id", "not a non-empty string")

    def test_a_pattern_that_does_not_compile_is_refused(self):
        self.refused(text(doc(entry(PIN, "0\\.17\\.3("))), PIN, "does not compile")

    def test_a_pattern_matching_the_empty_text_is_refused(self):
        # It would make any failure of its test a pin.
        self.refused(text(doc(entry(PIN, "x*"))), PIN, "matches the empty text")

    def test_a_duplicate_id_is_refused(self):
        self.refused(text(doc(entry(PIN, "a"), entry(PIN, "b"))), PIN, "twice")

    # The W2 combine (2026-11-13): one test runs in two rows (pytests under 3.15, pyfloor under 3.12), so lane F1
    # measured the same pin red in both and wrote one entry per row; the sort keys pins by test id, so one id may sit
    # under several rows when its pattern is the same in each, and a pattern that differs is refused.
    def test_one_id_under_two_rows_with_one_pattern_loads_once(self):
        two_rows = doc(entry(PIN, "a"), {**entry(PIN, "a"), "row": "pyfloor"})
        self.assertEqual(self.sort.load_pin_tests(text(two_rows)), {PIN: "a"})

    def test_one_id_under_two_rows_with_two_patterns_is_refused(self):
        self.refused(text(doc(entry(PIN, "a"), {**entry(PIN, "b"), "row": "pyfloor"})), PIN, "two patterns")

    def test_the_committed_pin_tests_json_loads(self):
        with open("gate/canary/pin_tests.json", encoding="utf-8") as f:
            pins = self.sort.load_pin_tests(f.read())
        self.assertIn(
            "gate.toolchain.tests.test_manifest.Load.test_the_committed_tools_json_loads_in_file_order_for_both_platforms",
            pins,
        )


class Unmeasured(unittest.TestCase):
    def setUp(self) -> None:
        from gate.canary import sort

        self.sort = sort

    def test_the_unmeasured_ids_sorted(self):
        two = doc(*GOOD["entries"], entry("census.md2", "y", measured="unmeasured"))
        self.assertEqual(self.sort.unmeasured(text(two)), ["census.md2", GO])

    def test_none_unmeasured(self):
        self.assertEqual(self.sort.unmeasured(text(doc(entry(PIN, "a")))), [])

    def test_a_malformed_file_is_refused(self):
        with self.assertRaises(CanaryError):
            self.sort.unmeasured("{")

    def test_an_unmeasured_pin_matching_is_a_pin_named_unmeasured(self):
        pins = self.sort.load_pin_tests(text(GOOD))
        got = self.sort.sort(
            [record("unit", [[GO, "AssertionError: 'go1.27.2' != 'go1.27.1'"]])], pins, [], unmeasured=[GO]
        )
        self.assertEqual(list(map(str, got.pins)), [f"{GO} (gate/toolchain/tests/test_manifest.py; unmeasured pin)"])
        self.assertEqual(list(got.breaks), [])

    def test_an_unmeasured_pin_not_matching_is_a_break_named_unmeasured(self):
        pins = self.sort.load_pin_tests(text(GOOD))
        got = self.sort.sort([record("unit", [[GO, "AssertionError: go1.28.0"]])], pins, [], unmeasured=[GO])
        self.assertEqual(list(got.pins), [])
        self.assertEqual(list(map(str, got.breaks)), [f"unit: {GO}: AssertionError: go1.28.0 (unmeasured pin)"])


class PinAndBreakLines(unittest.TestCase):
    """The lines' text, as the summary prints them (design §4.6: `<test id> (<file>)`, `<row>: <reason>`)."""

    def setUp(self) -> None:
        from gate.canary import sort

        self.sort = sort
        self.pins = sort.load_pin_tests(text(GOOD))

    def test_a_test_pin_names_its_file(self):
        got = self.sort.sort([record("unit", [[PIN, "'0.17.5' != '0.17.3'"]])], self.pins, [])
        self.assertEqual(list(map(str, got.pins)), [f"{PIN} (gate/toolchain/tests/test_manifest.py)"])

    def test_a_non_test_pin_is_its_name(self):
        got = self.sort.sort([record("census", [["census.md", "census.md: ruff 0.17.3 is old"]])], self.pins, [])
        self.assertEqual(list(map(str, got.pins)), ["census.md"])

    def test_a_pin_failing_in_two_records_is_listed_once(self):
        red = [[PIN, "'0.17.5' != '0.17.3'"]]
        got = self.sort.sort([record("unit", red), record("unit-macos", red)], self.pins, [])
        self.assertEqual(len(list(got.pins)), 1)

    def test_a_break_is_one_line_with_its_row(self):
        tb = 'Traceback (most recent call last):\n  File "x.py", line 3\nAssertionError: moved\n'
        got = self.sort.sort([record("unit", [["a.b.C.test_d", tb]])], self.pins, [])
        self.assertEqual(
            list(map(str, got.breaks)),
            ['unit: a.b.C.test_d: Traceback (most recent call last): File "x.py", line 3 AssertionError: moved'],
        )

    def test_a_break_whose_id_is_its_row_names_it_once(self):
        got = self.sort.sort([record("ruff", [["ruff", "gate/x.py:3:1: E999 new rule"]])], self.pins, [])
        self.assertEqual(list(map(str, got.breaks)), ["ruff: gate/x.py:3:1: E999 new rule"])

    def test_structural_faults_first_then_the_records_sorted(self):
        got = self.sort.sort(
            [record("vet", [["vet", "b"]]), record("ruff", [["ruff", "a"]])], self.pins, ["canary: z", "canary: y"]
        )
        self.assertEqual(list(map(str, got.breaks)), ["canary: y", "canary: z", "ruff: a", "vet: b"])

    def test_a_structural_fault_names_the_pin_failures_as_breaks(self):
        got = self.sort.sort([record("unit", [[PIN, "'0.17.5' != '0.17.3'"]])], self.pins, ["canary: y"])
        self.assertEqual(list(map(str, got.breaks)), ["canary: y", f"unit: {PIN}: '0.17.5' != '0.17.3'"])


if __name__ == "__main__":
    unittest.main()
