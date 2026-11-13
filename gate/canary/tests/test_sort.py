"""sort.sort (S1.1c-4 design §4.5; plan check §6 canary (10), (13); review DQ-S8). gate/canary/sort.py does not exist
at W0c: every test here is red by ModuleNotFoundError until lane F1 adds it.

The shapes as these tests read them (the design leaves them to F1's measured run — W0c report, ambiguity 3): a record
is the runner's record (`id`, `job`, `verdict`, `reason`) with `failures`, a list of `[test id, failure text]` pairs
(a non-test red, such as census.md or a go.mod toolchain line, is a pair whose id names it); `pin_tests` maps a test id
to a regular-expression string its failure text must match (re.search); `Sorted.pins` and `Sorted.breaks` are
sequences whose items name the test id or the row and failure. Assertions read their str().
"""

import unittest

PIN = "gate.toolchain.tests.test_manifest.RealPins.test_ruff_version"
PIN_TESTS = {PIN: r"0\.17\.3", "census.md": r"^census\.md: ruff 0\.17\.3"}
STRUCTURAL = ["canary: ruff.path_dir changed: . → bin"]


def record(row: str, failures: list[list[str]], verdict: str = "fail") -> dict:
    # A red record's reason is verdict.decide's for a row red by its own exit code (`exit N: …`): any other reason
    # makes the record a break whatever its failures (break-it BK-S3; test_fix_s11c4.ReasonBreaks).
    said = "; ".join(f"{test}: {text}" for test, text in failures)
    reason = "ok" if verdict == "pass" else f"exit 1: {said}" if said else "exit 1"
    return {"id": row, "job": "lint", "verdict": verdict, "reason": reason, "failures": failures}


def text(items) -> str:
    return "\n".join(str(item) for item in items)


class Sort(unittest.TestCase):
    def setUp(self) -> None:
        from gate.canary import sort  # red by ModuleNotFoundError until F1 adds it

        self.sort = sort.sort

    def test_pin_failure_on_its_version_is_a_pin(self):
        got = self.sort([record("unit", [[PIN, "AssertionError: '0.17.5' != '0.17.3'"]])], PIN_TESTS, [])
        self.assertIn(PIN, text(got.pins))
        self.assertEqual(list(got.breaks), [])

    def test_pin_test_failing_on_a_hash_is_a_break(self):
        # Case (10): the same test, red on something its pattern does not name.
        got = self.sort([record("unit", [[PIN, "AssertionError: tree_sha256 4bda7c != 9079ac"]])], PIN_TESTS, [])
        self.assertEqual(list(got.pins), [])
        self.assertIn("tree_sha256 4bda7c", text(got.breaks))

    def test_record_with_pin_and_other_failure_is_a_break(self):
        # Review DQ-S8: every failure in a record must be a pin for the record to be one.
        other = "gate.runner.tests.test_rows.Ruff.test_format"
        got = self.sort(
            [record("unit", [[PIN, "AssertionError: '0.17.5' != '0.17.3'"], [other, "AssertionError: E501 moved"]])],
            PIN_TESTS,
            [],
        )
        self.assertEqual(list(got.pins), [])
        self.assertIn(other, text(got.breaks))
        self.assertIn("E501 moved", text(got.breaks))

    def test_any_other_red_is_a_break_named(self):
        got = self.sort([record("ruff", [["ruff", "gate/x.py:3:1: E999 new rule"]])], PIN_TESTS, [])
        self.assertEqual(list(got.pins), [])
        self.assertIn("ruff", text(got.breaks))
        self.assertIn("E999 new rule", text(got.breaks))

    def test_non_test_pin_red_is_a_pin(self):
        got = self.sort(
            [record("census", [["census.md", "census.md: ruff 0.17.3 is not tools.json's 0.17.5"]])], PIN_TESTS, []
        )
        self.assertIn("census.md", text(got.pins))
        self.assertEqual(list(got.breaks), [])

    def test_a_structural_fault_makes_every_pin_a_break(self):
        # Case (13): a moved path_dir is a named break, never "pins to update".
        got = self.sort([record("unit", [[PIN, "AssertionError: '0.17.5' != '0.17.3'"]])], PIN_TESTS, STRUCTURAL)
        self.assertEqual(list(got.pins), [])
        self.assertIn("canary: ruff.path_dir changed: . → bin", text(got.breaks))

    def test_a_structural_fault_alone_is_a_break(self):
        got = self.sort([record("unit", [], verdict="pass")], PIN_TESTS, STRUCTURAL)
        self.assertIn("canary: ruff.path_dir changed: . → bin", text(got.breaks))

    def test_green_records_sort_to_nothing(self):
        got = self.sort([record("unit", [], verdict="pass"), record("ruff", [], verdict="pass")], PIN_TESTS, [])
        self.assertEqual((list(got.pins), list(got.breaks)), ([], []))

    def test_a_red_record_with_no_failures_read_is_a_break(self):
        red = record("deps", [])
        red["reason"] = "deps: timed out after 180 s"
        got = self.sort([red], PIN_TESTS, [])
        self.assertEqual(list(got.pins), [])
        self.assertIn("deps", text(got.breaks))
        self.assertIn("timed out", text(got.breaks))


if __name__ == "__main__":
    unittest.main()
