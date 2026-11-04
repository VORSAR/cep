"""The fuzz row wrapper (S2.2a; owner 2026-11-04: the nightly tier): one Go fuzz target per listed file, for a time.

The real go, a temporary module, and GATE_FUZZTIME short enough to test: a clean target covers one; a target that
crashes fails naming it, and the crashing input lands in the row directory for the artifact.
"""

import os
import sys

from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
MOD = "module ex.com/m\n\ngo 1.27.0\n"
CLEAN = (
    'package p\n\nimport "testing"\n\n'
    'func FuzzOK(f *testing.F) {\n\tf.Add("a")\n\tf.Fuzz(func(t *testing.T, s string) { _ = len(s) })\n}\n'
)
CRASHING = (
    'package p\n\nimport "testing"\n\n'
    'func FuzzBad(f *testing.F) {\n\tf.Add("a")\n'
    "\tf.Fuzz(func(t *testing.T, s string) {\n"
    "\t\tif len(s) > 0 && s[0] == 'z' {\n\t\t\tt.Fatal(\"planted\")\n\t\t}\n"
    "\t})\n}\n"
)


class GoFuzz(support.GateTest):
    def setUp(self):
        super().setUp()
        self.toolchain.dirs = [support.tool_bin("go")]
        self.track("go.mod", MOD)
        self.track("p/p.go", "package p\n")

    def fuzz(self, *files):
        command = (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "gofuzz.py"), "{files}")
        environ = {**support.base_env(), "GATE_FUZZTIME": "3s"}
        self.run_rows(
            [self.row("fuzz", command=command, expected_scope=("git", *files), job="nightly")], environ=environ
        )
        return self.reason("fuzz", "nightly")

    def test_a_clean_target_covers_one(self):
        self.track("p/ok_fuzz_test.go", CLEAN)
        self.assertEqual(self.fuzz("p/ok_fuzz_test.go"), "covered 1")

    def test_a_crashing_target_fails_naming_it_and_keeps_the_input(self):
        self.track("p/bad_fuzz_test.go", CRASHING)
        self.assertEqual(self.fuzz("p/bad_fuzz_test.go"), "exit 1: p/bad_fuzz_test.go: FuzzBad crashed: see crashers/")
        crashers = os.listdir(os.path.join(self.run_dir, "rows", "nightly", "fuzz", "crashers"))
        self.assertTrue(crashers, "the crashing input was not kept")
