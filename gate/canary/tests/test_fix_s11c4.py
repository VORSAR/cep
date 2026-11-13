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
from gate.canary.tests import test_commands as commands
from gate.runner import main
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


# unittest's header for a subtest given a message, alone and with parameters (break-it 2, BK2-S1), written by hand.
MSG_BLOCK = """\
======================================================================
FAIL: test_shape ({id}) [tools empty]
----------------------------------------------------------------------
Traceback (most recent call last):
  File "/x/gate/a/tests/test_b.py", line 9, in test_shape
    self.fail("x")
AssertionError: ManifestError not raised
"""
MSG_PARAMS_BLOCK = """\
======================================================================
ERROR: test_shape ({id}) [tools empty] (platform='linux-amd64')
----------------------------------------------------------------------
Traceback (most recent call last):
  File "/x/gate/a/tests/test_b.py", line 9, in test_shape
    raise KeyError("go")
KeyError: 'go'
"""
SHAPE_ID = "gate.a.tests.test_b.Shape.test_shape"


def counted(failures: int, errors: int = 0) -> str:
    """unittest's closing lines for a failed module (TextTestRunner, as pyshim runs it)."""
    said = ", ".join(f"{k}={n}" for k, n in (("failures", failures), ("errors", errors)) if n)
    return f"{'-' * 70}\nRan 9 tests in 0.1s\n\nFAILED ({said})\n"


class FailureBlocks(unittest.TestCase):
    """Break-it 2, BK2-S1: every failure block unittest prints is read, whatever follows its id; a module whose
    reported count is not the blocks read is a break naming both, so an unread block can never hide behind a pin."""

    def row(self, stderr: str) -> str:
        tmp = tempfile.TemporaryDirectory(prefix="fx-c-")
        self.addCleanup(tmp.cleanup)
        commands.write(
            os.path.join(tmp.name, "stdout"), "failed: gate/a/tests/test_b.py: pyshim: gate.a.tests.test_b: failed\n"
        )
        commands.write(os.path.join(tmp.name, "py", "gate.a.tests.test_b.stderr"), stderr)
        return tmp.name

    def test_a_subtest_header_with_a_message_is_read(self):
        # Red at 4695fe3: _FAILURE refused both headers, so neither block was read (the line itself was the failure).
        stderr = MSG_BLOCK.format(id=SHAPE_ID) + MSG_PARAMS_BLOCK.format(id=SHAPE_ID) + counted(1, 1)
        self.assertEqual(
            main._failures(self.row(stderr), "pytests"),
            [[SHAPE_ID, "AssertionError: ManifestError not raised"], [SHAPE_ID, "KeyError: 'go'"]],
        )

    def test_fewer_blocks_read_than_reported_is_a_break_naming_both(self):
        # Red at 4695fe3: an unreadable header (here one with no id) was skipped and the module's count never read.
        unreadable = MSG_BLOCK.format(id=SHAPE_ID).replace(f"test_shape ({SHAPE_ID}) [tools empty]", "test_shape")
        stderr = MSG_BLOCK.format(id=SHAPE_ID) + unreadable + counted(2)
        self.assertEqual(
            main._failures(self.row(stderr), "pytests"),
            [
                ["pytests", "gate.a.tests.test_b: 2 failures reported, 1 read"],
                [SHAPE_ID, "AssertionError: ManifestError not raised"],
            ],
        )

    def test_blocks_with_no_failed_count_are_a_break(self):
        # Red at 4695fe3: the blocks were taken as the module's whole story with no count to hold them to.
        stderr = MSG_BLOCK.format(id=SHAPE_ID)
        self.assertEqual(
            main._failures(self.row(stderr), "pytests"),
            [
                ["pytests", "gate.a.tests.test_b: no FAILED count reported, 1 read"],
                [SHAPE_ID, "AssertionError: ManifestError not raised"],
            ],
        )


class JobRows(unittest.TestCase):
    """Break-it 2, BK2-S2: each job's records are held to the registry's T4 rows for that job plus its parity row,
    as gate/runner/summarize.py holds CI's (an absent row is never a pass)."""

    setUp = commands.Summary.setUp
    summarize = commands.Summary.summarize
    record = commands.Summary.record

    def candidate_written(self) -> None:
        os.makedirs(self.candidate)
        commands.write(os.path.join(self.candidate, "tools.json"), commands.read(self.tools))
        commands.write(os.path.join(self.candidate, "upgrades.json"), '{"newer_lines":[],"upgrades":[]}\n')

    def registry_of(self, *ids: str) -> None:
        with open(os.path.join(REPO, "gate", "registry.json"), encoding="utf-8") as f:
            doc = json.load(f)
        doc["rows"] = [row for row in doc["rows"] if row["id"] in ids]
        self.assertEqual(sorted(row["id"] for row in doc["rows"]), sorted(ids))
        commands.write(os.path.join(self.repo, "gate", "registry.json"), json.dumps(doc))

    def test_a_row_missing_and_a_row_unknown_are_breaks(self):
        # Red at 4695fe3: canary_summary never read the registry, so the job's missing ruff row and its unknown
        # ghost row went unnamed: "Breaks: none", exit 0.
        self.registry_of("pyfloor", "ruff")
        self.candidate_written()
        self.record("lint", "parity", "pass", "", {})
        self.record("lint", "pyfloor", "pass", "", {})
        self.record("lint", "ghost", "pass", "", {})
        code, said = self.summarize({"candidate": {"result": "success"}, "lint": {"result": "success"}})
        self.assertIn(
            "Breaks: canary: job lint recorded ghost, not in the registry, canary: job lint recorded no ruff\n", said
        )
        self.assertEqual(code, 1, said)

    def test_every_row_recorded_is_no_break(self):
        # The positive witness for the test above: the same world with every row recorded and none unknown.
        self.registry_of("pyfloor", "ruff")
        self.candidate_written()
        for row in ("parity", "pyfloor", "ruff"):
            self.record("lint", row, "pass", "", {})
        code, said = self.summarize({"candidate": {"result": "success"}, "lint": {"result": "success"}})
        self.assertIn("Breaks: none\n", said)
        self.assertEqual(code, 0, said)

    def test_no_registry_is_the_commands_error(self):
        # Red at 4695fe3: the summary ran without the registry it must hold the jobs to (exit 0 here).
        path = os.path.join(self.repo, "gate", "registry.json")
        if os.path.exists(path):  # the world's own registry, if its setUp wrote one
            os.remove(path)
        self.candidate_written()
        self.record("lint", "parity", "pass", "", {})
        code, said = self.summarize({"candidate": {"result": "success"}, "lint": {"result": "success"}})
        self.assertEqual(code, 2, said)
        self.assertIn("registry.json", said)


if __name__ == "__main__":
    unittest.main()
