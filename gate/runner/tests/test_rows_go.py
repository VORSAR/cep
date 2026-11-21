"""The real gofmt, vet and unit wrappers over temporary Go modules, with the pinned go (design note §5).

Teeth (QA DQ22, design-QA counter-example 33): each wrapper's count comes from the tool's own work, so a narrowed
run is red, naming both numbers. Each narrowed case has a full control that passes.
"""

import json
import os
import sys
import unittest

from gate.runner import registry
from gate.runner.rows import race as race_row
from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
MOD = "module ex.com/m\n\ngo 1.27.0\n"
# A `go` that logs its arguments, one JSON line per call, then becomes the pinned go (Race.go_test_argv).
LOGGED_GO = """\
import json, os, sys
with open(LOG, "a", encoding="utf-8") as log:
    log.write(json.dumps(sys.argv[1:]) + "\\n")
os.execv(REAL, [REAL, *sys.argv[1:]])
"""
GO_KILL_MARGIN_S = 60  # cmd/go kills a test binary a minute after its -timeout (cmd/go/internal/test, testKillTimeout)
# Chosen, not measured on CI: building the race-instrumented test binary comes before both of go's clocks start, and
# the row judges test.json and writes its record after go exits; two minutes covers a cold build of one package.
STRESS_HEADROOM_S = 120


def wrapper(name, *args):
    return (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, f"{name}.py"), *args)


class GoRows(support.GateTest):
    def setUp(self):
        super().setUp()
        self.toolchain.dirs = [support.tool_bin("go")]
        self.track("go.mod", MOD)

    def go_files(self, count):
        for i in range(count):
            self.write(f"p/f{i:03}.go", "package p\n")
        self.git("add", "--", "p")

    def stdout(self, row_id, job="lint"):
        return self.row_file(job, row_id, "stdout").decode("utf-8", errors="replace")


class Gofmt(GoRows):
    SCOPE = ("git", ":(glob)**/*.go")

    def gofmt(self, *args):
        self.run_rows([self.row("gofmt", command=wrapper("gofmt", *(args or ("{files}",))), expected_scope=self.SCOPE)])
        return self.reason("gofmt")

    def test_formatted_files_pass(self):
        self.go_files(3)
        self.assertEqual(self.gofmt(), "covered 3")

    def test_an_unformatted_file_fails_naming_it_and_the_fix(self):
        self.go_files(2)
        self.track("p/bad.go", "package p\nvar   x = 1\n")
        self.assertEqual(self.gofmt(), "exit 1: p/bad.go is not formatted: fix it with gofmt -w p/bad.go")
        out = self.stdout("gofmt")
        self.assertIn("p/bad.go", out)
        self.assertIn("gofmt -w p/bad.go", out)

    def test_a_file_gofmt_cannot_parse_fails_naming_it(self):
        self.track("p/broken.go", "package p\nfunc {\n")
        self.assertEqual(self.gofmt(), "exit 1: gofmt exit 2: p/broken.go:2:6: expected 'IDENT', found '{'")
        self.assertIn("p/broken.go", self.stdout("gofmt"))

    def test_257_files_cross_the_batch_boundary(self):
        self.go_files(257)
        self.assertEqual(self.gofmt(), "covered 257")
        names = os.listdir(os.path.join(self.run_dir, "rows", "lint", "gofmt"))
        self.assertEqual(sorted(n for n in names if n.startswith("gofmt.") and n.endswith(".stdout")),
                         ["gofmt.0.stdout", "gofmt.1.stdout"])  # fmt: skip  # aligned arguments

    def test_dropping_the_last_batch_is_red(self):
        # design-QA counter-example 33: 257 expected, and the wrapper handed only the first 256.
        self.go_files(257)
        names = sorted(self.git("ls-files", "-z", "--", "*.go").split(b"\0"))[1:257]
        short = os.path.join(self.root, "short.nul")
        with open(short, "wb") as f:
            f.write(b"".join(n + b"\0" for n in names))
        self.assertEqual(self.gofmt(short), "covered 256 ≠ expected 257")

    def test_odd_file_names(self):
        self.track("p/sp ace.go", "package p\n")
        self.track("p/-dash.go", "package p\n")
        self.assertEqual(self.gofmt(), "covered 2")


class Vet(GoRows):
    SCOPE = ("go-vet-units", "./...")

    def three_units(self):
        # By hand from design note §4: a 1 unit, b 1 unit, c 1 unit.
        for name in "abc":
            self.track(f"{name}/x.go", f"package {name}\n")

    def vet(self, *args):
        rows = [self.row("vet", command=wrapper("vet", *args), expected_scope=self.SCOPE)]
        self.run_rows(rows)
        return self.reason("vet")

    def test_the_real_count_equals_the_formula_on_every_shape(self):
        # design-QA counter-example 1: notests 1, intests 1, xtests 2, both 2, onlyx 1 = 7.
        self.track("notests/a.go", "package notests\n")
        self.track("intests/a.go", "package intests\n")
        self.track("intests/a_test.go", "package intests\n")
        self.track("xtests/a.go", "package xtests\n")
        self.track("xtests/a_test.go", "package xtests_test\n")
        self.track("both/a.go", "package both\n")
        self.track("both/a_test.go", "package both\n")
        self.track("both/b_test.go", "package both_test\n")
        self.track("onlyx/a_test.go", "package onlyx_test\n")
        self.assertEqual(self.vet(), "covered 7")

    def test_narrowed_to_two_of_three_units_is_red(self):
        self.three_units()
        self.assertEqual(self.vet("./a", "./b"), "covered 2 ≠ expected 3")

    def test_the_full_control_passes(self):
        self.three_units()
        self.assertEqual(self.vet(), "covered 3")

    def test_a_diagnostic_fails_naming_its_position_even_though_vet_exits_0(self):
        # design-QA counter-example 2.
        self.track("c/c.go", 'package c\n\nimport "fmt"\n\nvar S = fmt.Sprintf("%d", "x")\n')
        position = os.path.join(self.repo, "c", "c.go:5:22")
        self.assertEqual(self.vet(), f'exit 1: {position}: fmt.Sprintf format %d has arg "x" of wrong type string')
        self.assertIn("c/c.go:5", self.stdout("vet"))

    def test_a_compile_error_prints_no_object_and_still_fails(self):
        self.track("a/a.go", "package a\n")
        self.track("b/b.go", 'package b\n\nvar x int = "s"\n')
        self.assertRegex(self.vet(), r"^exit 1: go vet exit 1: .* b/b\.go:3:13: cannot use ")
        self.assertIn("b/b.go", self.stdout("vet"))

    def test_a_compile_error_only_in_a_test_file_fails(self):
        # re-check S5: `go build` is green here; vet's non-zero exit is the only signal.
        self.track("a/a.go", "package a\n")
        self.track("a/a_test.go", 'package a\n\nvar x int = "s"\n')
        self.assertRegex(self.vet(), r"^exit 1: go vet exit 1: .* a/a_test\.go:3:13: cannot use ")


class Unit(GoRows):
    SCOPE = ("go", "./...")
    GOOD = 'package p\n\nimport "testing"\n\nfunc TestOk(t *testing.T) {}\n'
    BAD = 'package p\n\nimport "testing"\n\nfunc TestBad(t *testing.T) { t.Fatal("no") }\n'

    def unit(self, *args):
        rows = [self.row("unit", command=wrapper("unit", *args), expected_scope=self.SCOPE, job="test")]
        self.run_rows(rows)
        return self.reason("unit", "test")

    def package(self, name, test=GOOD):
        self.track(f"{name}/a.go", "package p\n\nfunc F() int { return 1 }\n")
        self.track(f"{name}/a_test.go", test)

    def test_passing_packages(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.unit(), "covered 2")

    def test_a_package_without_tests_still_counts(self):
        self.package("a")
        self.track("b/b.go", "package b\n")
        self.assertEqual(self.unit(), "covered 2")

    def test_the_artifacts_the_next_row_needs_are_in_the_row_directory(self):
        self.package("a")
        self.unit()
        self.assertIn(b'"Action":"pass"', self.row_file("test", "unit", "test.json"))
        self.assertTrue(self.row_file("test", "unit", "cover.prof").startswith(b"mode: "))

    def test_a_failing_test_fails_naming_it(self):
        self.package("a")
        self.package("b", self.BAD)
        self.assertEqual(self.unit(), "exit 1: ex.com/m/b: TestBad")
        out = self.stdout("unit", "test")
        self.assertIn("ex.com/m/b", out)
        self.assertIn("TestBad", out)

    def test_a_skipped_test_fails_naming_it(self):
        # Owner, 2026-11-03 (break-it QB7): a skipped Go test fails the row, as a skipped Python test fails pytests.
        skip = self.GOOD + '\nfunc TestLater(t *testing.T) { t.Skip("flaky") }\n'
        self.package("a", skip)
        self.assertEqual(self.unit(), "exit 1: ex.com/m/a: TestLater: skipped")
        out = self.stdout("unit", "test")
        self.assertIn("ex.com/m/a: TestLater: skipped", out)

    def test_no_test_at_all_fails(self):
        self.track("a/a.go", "package a\n")
        self.track("b/b.go", "package b\n")
        self.assertEqual(self.unit(), "exit 1: no test ran")
        self.assertIn("no test ran", self.stdout("unit", "test"))

    def test_a_test_build_failure_fails(self):
        self.package("a", 'package p\n\nvar x int = "s"\n')
        self.assertEqual(self.unit(), "exit 1: ex.com/m/a: package failed")

    def test_narrowed_to_one_of_two_packages_is_red(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.unit("./a"), "covered 1 ≠ expected 2")


class RaceStressBudget(unittest.TestCase):
    """The real race-stress row, read through the gate's registry loader: its own timeout_s must hold go test's
    stress timeout, cmd/go's kill margin after it and the build-and-report headroom."""

    def test_race_stress_repeats_and_its_go_timeout_with_margins_fits_its_timeout_s(self):
        rows = {r.id: r for r in registry.load(os.path.join(support.GATE, "registry.json"))}
        row = rows["race-stress"]
        self.assertIn("gate/runner/rows/race.py", row.command)
        self.assertTrue(any(a.startswith("--repeat=") for a in row.command), row.command)
        need_s = race_row.STRESS_TIMEOUT_MIN * 60 + GO_KILL_MARGIN_S + STRESS_HEADROOM_S
        self.assertLessEqual(
            need_s,
            row.timeout_s,
            f"go -timeout {race_row.STRESS_TIMEOUT} + {GO_KILL_MARGIN_S} s + {STRESS_HEADROOM_S} s > timeout_s",
        )


class Race(GoRows):
    """The race row (owner, 2026-11-05: pulled forward from S3.2b): `go test -race -json -count=1`, with cgo on for
    that row alone (ADR-0015 §3.6, residue item 9). A data race fails the test it happened in, naming it."""

    SCOPE = ("go", "./...")
    GOOD = Unit.GOOD
    RACY = (
        'package p\n\nimport (\n\t"sync"\n\t"testing"\n)\n\n'
        "func TestRacy(t *testing.T) {\n\tn := 0\n\tvar wg sync.WaitGroup\n\tfor range 2 {\n\t\twg.Add(1)\n"
        "\t\tgo func() { defer wg.Done(); n++ }()\n\t}\n\twg.Wait()\n\t_ = n\n}\n"
    )

    def race(self, *args):
        rows = [self.row("race", command=wrapper("race", *args), expected_scope=self.SCOPE, job="race")]
        self.run_rows(rows)
        return self.reason("race", "race")

    def package(self, name, test=GOOD):
        self.track(f"{name}/a.go", "package p\n\nfunc F() int { return 1 }\n")
        self.track(f"{name}/a_test.go", test)

    def test_passing_packages(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.race(), "covered 2")

    def test_a_data_race_fails_naming_its_test(self):
        self.package("a")
        self.package("b", self.RACY)
        self.assertEqual(self.race(), "exit 1: ex.com/m/b: TestRacy")
        self.assertIn("ex.com/m/b: TestRacy", self.stdout("race", "race"))

    def go_test_argv(self, *args):
        """go test's arguments as the row gave them, logged by a `go` ahead of the pinned one that then becomes it."""
        log = os.path.join(self.root, "go.argv")
        real = os.path.join(support.tool_bin("go"), "go")
        logbin = os.path.join(self.root, "logbin")
        self.script("go", logbin, f"LOG, REAL = {log!r}, {real!r}\n{LOGGED_GO}")
        self.toolchain.dirs = [logbin, *self.toolchain.dirs]
        self.package("a")
        self.assertEqual(self.race(*args), "covered 1")
        with open(log, encoding="utf-8") as f:
            calls = [json.loads(line) for line in f]
        tests = [c for c in calls if c[:1] == ["test"]]
        self.assertEqual(len(tests), 1, calls)
        return tests[0]

    def test_a_repeat_gives_go_test_the_stress_timeout(self):
        # The nightly race-stress row: go test's default -timeout 10m covers all N runs in one test binary, and 20
        # runs of ./internal/runtime took 519.8 s on CI one night and 601 s (timed out) the next.
        self.assertEqual(
            self.go_test_argv("--repeat=20", "./..."),
            ["test", "-race", "-json", "-count=20", "-timeout", race_row.STRESS_TIMEOUT, "./..."],
        )

    def test_a_repeat_of_one_still_gives_go_test_the_stress_timeout(self):
        # The rule is any --repeat, not a repeat above one.
        self.assertEqual(
            self.go_test_argv("--repeat=1", "./..."),
            ["test", "-race", "-json", "-count=1", "-timeout", race_row.STRESS_TIMEOUT, "./..."],
        )

    def test_no_repeat_leaves_go_tests_default_timeout(self):
        self.assertEqual(self.go_test_argv("./..."), ["test", "-race", "-json", "-count=1", "./..."])

    def test_a_repeat_runs_every_test_that_many_times(self):
        # The nightly race-stress row: --repeat=N is go test's -count=N; a pass needs every run to pass.
        self.package("a")
        self.assertEqual(self.race("--repeat=3", "./..."), "covered 1")
        passes = b'"Action":"pass","Package":"ex.com/m/a","Test":"TestOk"'
        self.assertEqual(self.row_file("race", "race", "test.json").count(passes), 3)
