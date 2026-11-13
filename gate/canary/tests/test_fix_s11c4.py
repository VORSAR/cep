"""S1.1c-4's fix round, lane FX-C: the canary's sort on a record red for a reason other than its exit code (break-it
BK-S3, review CA-L6's reason half), a Go patch candidate's pins (review CA-S1) and the candidate module's docstring
(review CA-L7). Each test was red on the build at 4695fe3 for the reason in its comment; every expected value is
written by hand from the review, design §4.5 and the toolchain-line test's message.
"""

import json
import os
import shutil
import tempfile
import unittest

from gate.canary import candidate, sort
from gate.toolchain.tests import test_toolchain_lines as lines

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
MANIFEST_PIN = (
    "gate.toolchain.tests.test_manifest.Load.test_the_committed_tools_json_loads_in_file_order_for_both_platforms"
)
MANIFEST_FILE = "gate/toolchain/tests/test_manifest.py"
LINES = "gate.toolchain.tests.test_toolchain_lines.ToolchainLines."
LINE_NAMES = (
    "test_a_missing_toolchain_line_is_named",
    "test_a_stale_toolchain_line_is_named_with_both_versions",
    "test_both_toolchain_lines_name_tools_jsons_go",
)
LINES_FILE = "gate/toolchain/tests/test_toolchain_lines.py"
# The manifest pin's first text shape (pin_tests.json), as unittest shortens a long list's repr.
MANIFEST_TEXT = (
    "AssertionError: Lists differ: [('py[93 chars]bin'), ('go', 'go1.27.5', 'go', 'bin'), ('go[84 chars]'.')] "
    "!= [('py[93 chars]bin'), ('go', 'go1.27.4', 'go', 'bin'), ('go[84 chars]'.')]"
)
STALE = "update the toolchain line with the bump"


def pin_tests_text() -> str:
    with open(os.path.join(REPO, "gate", "canary", sort.FILE), encoding="utf-8") as f:
        return f.read()


def red(row: str, reason: str, *failures: tuple[str, str]) -> dict:
    return {"id": row, "job": "test", "verdict": "fail", "reason": reason, "failures": [list(f) for f in failures]}


def exception_line(trace: str) -> str:
    """A formatted failure's exception line, as the canary's reader takes it (main._failure_blocks): the first line
    that is neither indented nor the traceback's header."""
    said = [line for line in trace.splitlines() if line and not line[0].isspace()]
    return next(line for line in said if line != "Traceback (most recent call last):")


class ReasonBreaks(unittest.TestCase):
    """Break-it BK-S3, review CA-L6 (the reason half): verdict.decide ranks a stop, a timeout, left-running processes,
    a kill by signal and a coverage fault above the row's exit code, so a red record whose reason is not `exit …` is
    a break by its reason, whatever failed lines it printed; its pins are not listed as pins."""

    PIN_TESTS = {MANIFEST_PIN: r"AssertionError: Lists differ: \[(?:\('py|'/c/py)"}

    def test_a_record_red_by_its_exit_code_with_only_pins_is_pins(self):
        # The positive witness for the next test: the same failure under the exit-code kind is a pin.
        got = sort.sort([red("pytests", "exit 1: failed: x", (MANIFEST_PIN, MANIFEST_TEXT))], self.PIN_TESTS, [])
        self.assertEqual(got, sort.Sorted((f"{MANIFEST_PIN} ({MANIFEST_FILE})",), ()))

    def test_a_record_red_for_another_reason_is_a_break_by_its_reason_and_lists_no_pin(self):
        # Red at 4695fe3: sort judged a record with failures by its failures only (pins (the manifest pin), no break).
        for reason in (
            "timeout after 900s",
            "killed by signal 9",
            "left running processes: pid 4242 (python3.15)",
            "covered 81 ≠ expected 82",
        ):
            with self.subTest(reason=reason):
                got = sort.sort([red("pytests", reason, (MANIFEST_PIN, MANIFEST_TEXT))], self.PIN_TESTS, [])
                self.assertEqual(got, sort.Sorted((), (f"pytests: {reason}",)))

    def test_a_record_red_for_another_reason_still_names_its_other_failures(self):
        # Red at 4695fe3: the timeout went unnamed beside the failure no pattern found.
        other = ("gate.runner.tests.test_rows.Ruff.test_format", "AssertionError: E501 moved")
        got = sort.sort(
            [red("pytests", "timeout after 900s", (MANIFEST_PIN, MANIFEST_TEXT), other)], self.PIN_TESTS, []
        )
        self.assertEqual(
            got,
            sort.Sorted(
                (),
                (
                    "pytests: gate.runner.tests.test_rows.Ruff.test_format: AssertionError: E501 moved",
                    "pytests: timeout after 900s",
                ),
            ),
        )


class GoPatchCandidate(unittest.TestCase):
    """Review CA-S1: a candidate that moves only Go's patch release reds the manifest's file-order pin (pytests and
    pyfloor) and the toolchain-line tests (pytests: pyfloor's registry row runs five files, not that one); all are
    "pins to update", the toolchain-line ones named unmeasured (design §4.5)."""

    def test_a_go_only_candidate_sorts_to_pins_and_no_breaks(self):
        # Red at 4695fe3: pin_tests.json named no toolchain-line test, so all three were breaks.
        text = pin_tests_text()
        pins = sort.load_pin_tests(text)
        stale = (
            "go.mod says toolchain go1.27.4, tools.json pins go1.27.5: " + STALE + "; "
            "gate/proofs/go.mod says toolchain go1.27.4, tools.json pins go1.27.5: " + STALE
        )
        records = [
            red(
                "pytests",
                "exit 1: failed: gate/toolchain/tests/test_manifest.py: 1 failure",
                (MANIFEST_PIN, MANIFEST_TEXT),
                (LINES + LINE_NAMES[0], f"AssertionError: the witness, an unedited copy: {stale}"),
                (LINES + LINE_NAMES[1], f"AssertionError: the witness, an unedited copy: {stale}"),
                (LINES + LINE_NAMES[2], f"AssertionError: the repository: {stale}"),
            ),
            red(
                "pyfloor",
                "exit 1: failed: gate/toolchain/tests/test_manifest.py: 1 failure",
                (MANIFEST_PIN, MANIFEST_TEXT),
            ),
            {"id": "vulns", "job": "test", "verdict": "pass", "reason": "ok", "failures": []},
        ]
        got = sort.sort(records, pins, [], unmeasured=sort.unmeasured(text))
        want_pins = (
            f"{MANIFEST_PIN} ({MANIFEST_FILE})",
            *(f"{LINES}{name} ({LINES_FILE}; unmeasured pin)" for name in LINE_NAMES),
        )
        self.assertEqual(got, sort.Sorted(want_pins, ()))

    def test_the_toolchain_line_tests_fail_under_a_go_move_only_by_their_pinned_pattern(self):
        # Red at 4695fe3: the tests read the repository itself and hard-coded go1.27.4, so a candidate's tools.json
        # could not be put under them, and pin_tests.json had no pattern for them.
        patterns = sort.load_pin_tests(pin_tests_text())
        with tempfile.TemporaryDirectory(prefix="fx-c-") as tmp:
            source = os.path.join(tmp, "candidate")
            for rel in ("tools.json", *lines.GOMODS):
                os.makedirs(os.path.dirname(os.path.join(source, rel)), exist_ok=True)
                shutil.copyfile(os.path.join(REPO, rel), os.path.join(source, rel))
            path = os.path.join(source, "tools.json")
            with open(path, encoding="utf-8") as f:
                doc = json.load(f)
            go = next(tool for tool in doc["tools"] if tool["name"] == "go")
            self.assertNotEqual(go["version"], "go1.99.0")
            go["version"] = "go1.99.0"
            with open(path, "w", encoding="utf-8") as f:
                json.dump(doc, f)

            class Candidate(lines.ToolchainLines):
                SOURCE = source

            result = unittest.TestResult()
            unittest.defaultTestLoader.loadTestsFromTestCase(Candidate).run(result)
        self.assertEqual(result.errors, [])
        failed = {test._testMethodName: exception_line(trace) for test, trace in result.failures}
        self.assertEqual(sorted(failed), list(LINE_NAMES), "each test had its chance under the candidate and failed")
        for name, said in sorted(failed.items()):
            with self.subTest(test=name):
                self.assertIn("tools.json pins go1.99.0", said)
                self.assertRegex(said, patterns[LINES + name])

    def test_the_note_names_the_test_that_reads_the_toolchain_lines(self):
        # Red at 4695fe3: the note said no row reads go.mod's toolchain lines (lane E2b's test does, in pytests).
        note = json.loads(pin_tests_text())["note"]
        self.assertNotIn("no row reads go.mod's toolchain lines", note)
        self.assertIn("test_toolchain_lines", note)


class CandidateDoc(unittest.TestCase):
    def test_the_module_doc_says_what_touches_the_disk(self):
        # Review CA-L7; red at 4695fe3: the doc called the module pure, but _platform_entry calls
        # install.archive_pins, which extracts and hashes each archive on disk.
        doc = candidate.__doc__ or ""
        self.assertNotIn("Pure over", doc)
        self.assertIn("install.archive_pins", doc)


if __name__ == "__main__":
    unittest.main()
