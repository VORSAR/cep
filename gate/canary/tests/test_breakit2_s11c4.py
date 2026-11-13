"""S1.1c-4 second break-it (QA): the canary's surface the first pass did not reach. Each test is red on the code it
was written against, for the reason its comment states; never the network."""

import os
import unittest

from gate.canary.tests import support as canary_support
from gate.canary.tests import test_commands as commands

# A real subtest in the pin test's own module: test_manifest.py's `with self.subTest(name):` (a positional msg).
SUBTEST_ID = "gate.toolchain.tests.test_manifest.Load.test_the_top_level_shape_is_checked"
SUBTEST_BLOCK = """\
======================================================================
FAIL: test_the_top_level_shape_is_checked ({id}) [tools empty]
----------------------------------------------------------------------
Traceback (most recent call last):
  File "/x/gate/toolchain/tests/test_manifest.py", line 224, in test_the_top_level_shape_is_checked
    self.assert_refused(doc)
AssertionError: ManifestError not raised
"""


class BreakIt2(unittest.TestCase):
    # test_commands.Summary's world (a repo with tools.json and pin_tests.json, the artifacts, the candidate), without
    # running its own tests a second time.
    setUp = commands.Summary.setUp
    summarize = commands.Summary.summarize
    record = commands.Summary.record

    def test_bk2_1_a_subtest_failure_with_a_message_beside_a_pin_red_is_still_a_break(self):
        # Red today: main._FAILURE accepts only `FAIL: <name> (<id>)` optionally followed by ` (<params>)`. unittest
        # prints a subtest with a message as `FAIL: <name> (<id>) [<msg>]` (and `[<msg>] (<params>)`), so the block
        # is never read; the module's other blocks are the pin's, the record holds pin failures only, and the sort
        # files a real failure under "Pins to update" with "Breaks: none" and exit 0.
        os.makedirs(self.candidate)
        commands.write(os.path.join(self.candidate, "tools.json"), commands.read(self.tools))
        commands.write(os.path.join(self.candidate, "upgrades.json"), '{"newer_lines":[],"upgrades":[]}\n')
        blocks = "".join(
            commands.FAIL_BLOCK.format(id=commands.MANIFEST_ID, platform=p) for p in canary_support.PLATFORMS
        )
        blocks += SUBTEST_BLOCK.format(id=SUBTEST_ID) + commands.TAIL.replace("failures=2", "failures=3")
        self.record(
            "lint",
            "pyfloor",
            "fail",
            "failed: gate/toolchain/tests/test_manifest.py: pyshim: gate.toolchain.tests.test_manifest: failed 3\n",
            {"gate.toolchain.tests.test_manifest": blocks},
        )
        code, said = self.summarize({"candidate": {"result": "success"}, "lint": {"result": "failure"}})
        self.assertIn(SUBTEST_ID, said, "the subtest's failure is named nowhere")
        self.assertNotIn("Breaks: none\n", said)
        self.assertEqual(code, 1, said)

    def test_bk2_2_a_job_whose_artifact_lacks_most_of_its_rows_is_not_pins_only(self):
        # Red today: canary_summary reads whatever records a job's artifact holds and never checks them against the
        # registry's T4 rows for that job (the gate's own summarize marks each absent row "missing: no job recorded
        # it"). The runner writes a record as each step ends (scheduler.py), so a lint job whose runner died after its
        # pyfloor step (the candidate's own CPython is the runner's interpreter) uploads one pin-red record, ends
        # failure, and the canary prints "Breaks: none", exit 0, for seventeen rows that never ran.
        with open(os.path.join(commands.REPO, "gate", "registry.json"), encoding="utf-8") as f:
            commands.write(os.path.join(self.repo, "gate", "registry.json"), f.read())
        os.makedirs(self.candidate)
        commands.write(os.path.join(self.candidate, "tools.json"), commands.read(self.tools))
        commands.write(os.path.join(self.candidate, "upgrades.json"), '{"newer_lines":[],"upgrades":[]}\n')
        blocks = "".join(
            commands.FAIL_BLOCK.format(id=commands.MANIFEST_ID, platform=p) for p in canary_support.PLATFORMS
        )
        self.record(
            "lint",
            "pyfloor",
            "fail",
            "failed: gate/toolchain/tests/test_manifest.py: pyshim: gate.toolchain.tests.test_manifest: failed 2\n",
            {"gate.toolchain.tests.test_manifest": blocks + commands.TAIL},
        )
        code, said = self.summarize({"candidate": {"result": "success"}, "lint": {"result": "failure"}})
        self.assertNotIn("Breaks: none\n", said)
        self.assertIn("ruff", said.split("Breaks:", 1)[-1], "the lint job's unrecorded rows are named nowhere")
        self.assertEqual(code, 1, said)


if __name__ == "__main__":
    unittest.main()
