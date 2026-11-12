"""The testhygiene row's planted proofs (S1.1c-2 lane C; design §3; testing.md rule 1).

The world holds the gate, the module's go.mod and go.sum, archcheck's sources with the import law it reads, and
tools/allowlists/ (its testhygiene list). Planted: a `_test.go` under internal/runtime/ calling time.Sleep (red with
design §3's text, naming the file and the line); the same call in bench/x_test.go, which the row's scope excludes and
so never reads; and a test that waits only on a context.WithTimeout cap (green). Until lane C lands and the
coordinator points the row at `go run ./tools/archcheck -tests {files}`, every case is red on the G0 stub's
`exit 1: testhygiene: not built yet (lane C)`.
"""

from gate.proofs.drivers import support

ROW = "testhygiene"
JOB = "lint"
SLEEP_TEST = "internal/runtime/planted_sleep_test.go"
SLEEP_LINE = 9
BENCH_TEST = "bench/x_test.go"
CAP_TEST = "internal/runtime/planted_cap_test.go"
SLEEP_SRC = (
    'package runtime\n\nimport (\n\t"testing"\n\t"time"\n)\n\n'
    "func TestPlantedSleep(t *testing.T) {\n\ttime.Sleep(1)\n}\n"
)
BENCH_SRC = SLEEP_SRC.replace("package runtime", "package bench")
CAP_SRC = (
    'package runtime\n\nimport (\n\t"context"\n\t"testing"\n\t"time"\n)\n\n'
    "func TestPlantedCap(t *testing.T) {\n"
    "\tctx, cancel := context.WithTimeout(context.Background(), time.Second)\n"
    "\tdefer cancel()\n"
    "\tselect {\n\tcase <-ctx.Done():\n\t}\n}\n"
)
# design §3, exactly (archcheck prints it after `failed: `)
SLEEP_RED = (
    f"{SLEEP_TEST}:{SLEEP_LINE}: testhygiene: time.Sleep in a test: wait for a state, cap it with a context, "
    "yield on every pass of a poll (testing.md rule 1)"
)


class TestHygienePlanted(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate()
        for rel in ("go.mod", "go.sum", "tools/importcheck/rules.json"):
            self.world.copy(rel)
        for prefix in ("tools/archcheck", "tools/allowlists"):
            for rel in self.world.tracked(prefix):
                if not rel.endswith("_test.go"):
                    self.world.copy(rel)
        self.assertEqual(SLEEP_SRC.split("\n")[SLEEP_LINE - 1], "\ttime.Sleep(1)", "the plant's line moved")

    def run_row(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row(ROW)))

    def in_scope(self) -> list[str]:
        """The row's {files}, from the world by the row's own pathspecs: its covered count, never hard-coded."""
        row = support.registry_row(ROW)
        self.assertEqual(row["expected_scope"][0], "git")
        out = self.world.git("ls-files", "-z", "--", *row["expected_scope"][1:])
        return [p for p in out.split("\0") if p]

    def test_a_sleep_in_a_runtime_test_is_red_naming_its_line(self):
        self.world.write(SLEEP_TEST, SLEEP_SRC)
        self.world.write(BENCH_TEST, BENCH_SRC)
        run = self.run_row()
        self.assertRed(run, JOB, ROW, f"exit 1: {SLEEP_RED}")
        out = run.row_stdout(JOB, ROW)
        self.assertNotIn(BENCH_TEST, out, "bench/ is outside the row's scope: never read")
        failed = [x.removeprefix("failed: ") for x in out.split("\n") if x.startswith("failed: ")]
        self.assertEqual(failed, [SLEEP_RED], out)

    def test_bench_is_not_read_and_a_context_cap_is_green(self):
        self.world.write(BENCH_TEST, BENCH_SRC)
        self.world.write(CAP_TEST, CAP_SRC)
        run = self.run_row()
        files = self.in_scope()
        self.assertIn(CAP_TEST, files)
        self.assertNotIn(BENCH_TEST, files)
        self.assertRow(run, "lint", "testhygiene", True, f"covered {len(files)}")
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)
