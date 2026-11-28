"""The cpu row's planted proof (S1.1c-11, lane C; plan check counter-example 20: a test that fails only at one
GOMAXPROCS value).

Planted: internal/clock, a package whose test calls t.Fatal when runtime.GOMAXPROCS(0) is 1, beside the passing
internal/control. Run through the real cpu row at the nightly tier, it fails naming -cpu 1 and that test, and no
-cpu 4 failure line: the row's two invocations each name the value they ran at. The -cpu 4 pass is shown, not
inferred from a missing line: the row's test.cpu4.json holds a pass for the planted test, test.cpu1.json its fail.
"""

import json

from gate.proofs.drivers import support

PLANTED = "github.com/vorsar/cep/gate/proofs/internal/clock: TestPlantedOnlyAtOneCPU"
PROCS_TEST = (
    'package clock\n\nimport (\n\t"runtime"\n\t"testing"\n)\n\n'
    "func TestPlantedOnlyAtOneCPU(t *testing.T) {\n"
    '\tif runtime.GOMAXPROCS(0) == 1 {\n\t\tt.Fatal("planted: fails only at GOMAXPROCS 1")\n\t}\n}\n'
)


class CpuPlanted(support.DriverTest):
    def test_the_cpu_row_names_a_test_that_fails_only_at_one_cpu(self):
        self.world.copy_gate()
        self.world.copy_proofs_module("control")
        self.world.track("internal/clock/clock.go", "package clock\n")
        self.world.track("internal/clock/procs_test.go", PROCS_TEST)
        self.world.commit()

        run = self.world.run(registry=self.world.registry(support.registry_row("cpu")), tier="nightly")

        self.assertParityPassed(run, "cpu")
        self.assertRed(run, "cpu", "cpu", f"exit 1: -cpu 1: {PLANTED}")
        out = run.row_stdout("cpu", "cpu")
        self.assertIn(f"-cpu 1: {PLANTED}", out)
        self.assertNotIn("-cpu 4:", out)
        self.assertEqual(self.actions(run, 1), ["fail"], "-cpu 1 did not fail the planted test")
        self.assertEqual(self.actions(run, 4), ["pass"], "-cpu 4 did not run the planted test to a pass")

    def actions(self, run, cpu):
        """The pass/fail/skip events go test -json gave the planted test in the row's invocation at `cpu`."""
        events = [json.loads(line) for line in run.row_file("cpu", "cpu", f"test.cpu{cpu}.json").splitlines()]
        test = PLANTED.rpartition(": ")[2]
        return [e["Action"] for e in events if e.get("Test") == test and e.get("Action") in ("pass", "fail", "skip")]
