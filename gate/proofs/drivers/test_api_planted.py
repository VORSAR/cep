"""The API gate's planted defects (design s3-3-1 §6.5; QA's T34 and T35): an unlisted export turns apipartition
red with the delta naming it; one parameter more on a clock function turns clockgolden red naming it twice (one
entry removed, one added); a signature change without api/exports.golden changing turns apidiff red at T4; the same
change with the golden regenerated passes, the report printed in the row's output.

The world holds the gate, go.mod, go.sum, api/, the root package's files, io/, internal/ and tools/: the rows run
the world's own apipartition. The apidiff proofs give the world a `main` ref at the copied tree and one commit on
another branch after it, so the row's merge base is the copied tree.
"""

import os

from gate.proofs.drivers import support

PLANTED = "zz_planted.go"
FROBNICATE = "package cep\n\n// Frobnicate is the planted, unlisted export.\nfunc Frobnicate() {}\n"
CLOCK = "internal/clock/clock.go"
CLOCK_SIG = "func NewStreamRegistry(specs []StreamSpec, start int64) *StreamRegistry {"
CLOCK_PLANT = "func NewStreamRegistry(specs []StreamSpec, start int64, extra int) *StreamRegistry {"
APPHANDLE = "apphandle.go"
START_SIG = "func (a *AppHandle) Start(ctx context.Context) error {"
START_PLANT = "func (a *AppHandle) Start(ctx context.Context, extra int) error {"
GOLDEN = "api/exports.golden"
PACKAGES = (".", "./io", "./io/inmemory")
INCOMPATIBLE = "Incompatible changes:"


def pinned_go() -> str:
    go = support.tool("go")
    return os.path.realpath(os.path.join(support.home(support.real_cache(), "go"), go.path_dir, go.command))


class ApiGatePlanted(support.DriverTest):
    def engine_world(self) -> None:
        self.world.copy_gate()
        for rel in (
            "go.mod",
            "go.sum",
            "gate/proofs/go.mod",
            "gate/proofs/go.sum",
        ):  # the apidiff tool is gate/proofs's
            self.world.copy(rel)
        for prefix in (":(glob)*.go", "api", "io", "internal", "tools"):
            for rel in self.world.tracked(prefix):
                self.world.copy(rel)

    def plant(self, rel: str, old: str, new: str) -> None:
        path = os.path.join(self.world.repo, rel)
        text = support.read_text(path)
        self.assertEqual(text.count(old), 1, f"the plant's anchor in {rel} moved")
        self.world.write(rel, text.replace(old, new))

    def run_row(self, row_id: str, tier: str) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row(row_id)), tier=tier)

    def assertRedContaining(self, run: support.Run, job: str, row_id: str, *needles: str) -> None:
        """Red, with the record's reason (the row's first failed: line) holding every needle, and the summary's FAIL."""
        self.assertGateFailed(run)
        record = run.record(job, row_id)
        self.assertEqual(record["verdict"], "fail", run.stdout)
        for needle in needles:
            self.assertIn(needle, record["reason"], record)
        cells = run.summary(job, row_id)
        self.assertEqual(cells[2], "FAIL", run.stdout)

    def test_an_unlisted_export_turns_apipartition_red(self):
        self.engine_world()
        self.world.write(PLANTED, FROBNICATE)
        run = self.run_row("apipartition", "T2")
        self.assertRedContaining(run, "lint", "apipartition", "API Δ: +1/−0 (cep.Frobnicate)")

    def test_a_clock_signature_change_turns_clockgolden_red(self):
        self.engine_world()
        self.plant(CLOCK, CLOCK_SIG, CLOCK_PLANT)
        run = self.run_row("clockgolden", "T2")
        delta = "API Δ: +1/−1 (clock.NewStreamRegistry, clock.NewStreamRegistry)"
        self.assertRedContaining(run, "lint", "clockgolden", delta)

    def change_after_main(self, *, regenerate: bool) -> support.Run:
        """The copied tree as `main`; on a branch after it, Start with one parameter more, the golden rewritten by
        the tool when asked; the apidiff row at T4."""
        self.engine_world()
        self.world.commit()
        self.world.git("checkout", "-q", "-b", "change")
        self.world.git("branch", "-f", "main", "HEAD")
        self.plant(APPHANDLE, START_SIG, START_PLANT)
        if regenerate:
            out, err = os.path.join(self.world.root, "write.stdout"), os.path.join(self.world.root, "write.stderr")
            env = {**self.world.env, "GOTOOLCHAIN": "local", "GOFLAGS": "", "GOWORK": "off"}
            argv = [pinned_go(), "run", "./tools/apipartition", "--write", GOLDEN, *PACKAGES]
            code = support.run_process(argv, cwd=self.world.repo, env=env, out=out, err=err, cap_s=support.RUN_CAP_S)
            self.assertEqual(code, 0, f"--write: exit {code}: {support.read_text(out)}{support.read_text(err)}")
            golden = support.read_text(os.path.join(self.world.repo, GOLDEN))
            self.assertIn("Start(ctx context.Context, extra int) error", golden, "the golden was not regenerated")
        return self.run_row("apidiff", "T4")

    def test_a_signature_change_without_the_golden_is_red(self):
        run = self.change_after_main(regenerate=False)
        self.assertRedContaining(run, "lint", "apidiff", INCOMPATIBLE, "Start")
        out = run.row_stdout("lint", "apidiff")
        self.assertIn(INCOMPATIBLE, out)
        self.assertIn("Start", out)

    def test_with_the_golden_regenerated_it_passes_and_prints(self):
        run = self.change_after_main(regenerate=True)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)
        self.assertEqual(run.code, 0, run.stdout)
        self.assertRow(run, "lint", "apidiff", True, f"covered {len(PACKAGES)}")
        out = run.row_stdout("lint", "apidiff")
        self.assertIn(INCOMPATIBLE, out)
        self.assertIn("Start", out)
