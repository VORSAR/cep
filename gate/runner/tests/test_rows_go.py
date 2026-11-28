"""The real gofmt, vet, unit and race wrappers over temporary Go modules, with the pinned go (design note §5).

Teeth (QA DQ22, design-QA counter-example 33): each wrapper's count comes from the tool's own work, so a narrowed
run is red, naming both numbers. Each narrowed case has a full control that passes.
"""

import json
import os
import sys
import types
import unittest

from gate.runner import registry
from gate.runner.rows import cpu as cpu_row
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
# The slowest recorded CI time of each nightly go row, in seconds, from the burn-in runs 46112427392, 46114653311,
# 46115572270 and 46116481578 (docs/sprints/s1-1c-11-record.md). cpu's is the row's whole time, an upper bound for
# each of its two go invocations. Each row's go -timeout must be at least 3 x its entry (RaceStressBudget).
SLOWEST_CI_S = {
    "cpu": 37.7,
    "race-stress": 229.0,
    "race-stress-inmemory": 17.7,
    "race-stress-partition": 21.9,
    "race-stress-playback": 976.4,
    "race-stress-root": 71.7,
}


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
    """The real nightly race-stress and cpu rows, read through the gate's registry loader: each row's timeout_s must
    hold go test's timeout, cmd/go's kill margin after it and the build-and-report headroom, and stay within the
    registry's hour (plan check S1.1c-11, counter-example 18: one pin per row). go's -timeout bounds one test binary,
    so each race-stress row names exactly one package, and the cpu row's two invocations each get their own. Each
    row's go timeout is also at least 3 x its slowest recorded CI run (SLOWEST_CI_S)."""

    def real_row(self, row_id):
        rows = {r.id: r for r in registry.load(os.path.join(support.GATE, "registry.json"))}
        self.assertIn(row_id, rows)
        return rows[row_id]

    def assertFits(self, row, need_s, why):
        self.assertLessEqual(need_s, row.timeout_s, f"{row.id}: {why} > timeout_s")
        self.assertLessEqual(row.timeout_s, 3600, f"{row.id}: timeout_s past the registry's hour")

    def stress_timeout_min(self, row):
        """A race-stress row's go timeout in minutes: its own --timeout, or the stress timeout without one."""
        named = [a.removeprefix("--timeout=") for a in row.command if a.startswith("--timeout=")]
        return int(named[0].removesuffix("m")) if named else race_row.STRESS_TIMEOUT_MIN

    def assertStressFits(self, row_id, package, *, own_timeout=True):
        self.assertStressRowFits(self.real_row(row_id), package, own_timeout=own_timeout)

    def assertStressRowFits(self, row, package, *, own_timeout=True):
        """A race-stress row: race.py, --repeat, one package (its expected scope too), and its go timeout — its own
        --timeout, or the stress timeout without one — with both margins inside its timeout_s."""
        self.assertIn("gate/runner/rows/race.py", row.command)
        after = row.command[row.command.index("gate/runner/rows/race.py") + 1 :]
        self.assertEqual(
            [a for a in after if not a.startswith("-")],
            [package],
            f"{row.id}: exactly one package, as go's -timeout bounds one test binary",
        )
        self.assertTrue(any(a.startswith("--repeat=") for a in row.command), row.command)
        self.assertEqual(
            ([a for a in row.command if not a.startswith("-")][-1:], tuple(row.expected_scope)),
            ([package], ("go", package)),
        )
        named = [a for a in row.command if a.startswith("--timeout=")]
        self.assertEqual(len(named), 1 if own_timeout else 0, row.command)
        minutes = self.stress_timeout_min(row)
        need_s = minutes * 60 + GO_KILL_MARGIN_S + STRESS_HEADROOM_S
        self.assertFits(row, need_s, f"go -timeout {minutes}m + {GO_KILL_MARGIN_S} s + {STRESS_HEADROOM_S} s")

    def test_race_stress_repeats_and_its_go_timeout_with_margins_fits_its_timeout_s(self):
        self.assertStressFits("race-stress", "./internal/runtime", own_timeout=False)

    def test_race_stress_inmemory_repeats_and_its_own_go_timeout_with_margins_fits_its_timeout_s(self):
        self.assertStressFits("race-stress-inmemory", "./io/inmemory")

    def test_race_stress_partition_repeats_and_its_own_go_timeout_with_margins_fits_its_timeout_s(self):
        self.assertStressFits("race-stress-partition", "./internal/partition")

    def test_race_stress_root_repeats_and_its_own_go_timeout_with_margins_fits_its_timeout_s(self):
        self.assertStressFits("race-stress-root", ".")

    def test_race_stress_playback_repeats_and_its_own_go_timeout_with_margins_fits_its_timeout_s(self):
        self.assertStressFits("race-stress-playback", "./internal/playback")

    def test_the_cpu_rows_two_go_timeouts_with_margins_fit_its_timeout_s(self):
        """go's -timeout bounds each test binary, not the invocation, so this sum does not bound the cpu row's wall
        when ./internal/... builds several binaries; the row's wall relies on the measured run (at most 38 s,
        SLOWEST_CI_S) against its 2040 s timeout_s."""
        row = self.real_row("cpu")
        self.assertIn("gate/runner/rows/cpu.py", row.command)
        need_s = 2 * (cpu_row.TIMEOUT_MIN * 60 + GO_KILL_MARGIN_S) + STRESS_HEADROOM_S
        self.assertFits(
            row,
            need_s,
            f"2 x (go -timeout {cpu_row.TIMEOUT_MIN}m, per test binary, + {GO_KILL_MARGIN_S} s) + headroom; the wall"
            f" relies on the measured run ({SLOWEST_CI_S['cpu']} s)",
        )

    def test_each_rows_go_timeout_is_at_least_3_x_its_slowest_recorded_ci_run(self):
        go_s = {
            row_id: self.stress_timeout_min(self.real_row(row_id)) * 60 for row_id in SLOWEST_CI_S if row_id != "cpu"
        }
        go_s["cpu"] = cpu_row.TIMEOUT_MIN * 60
        for row_id, slowest_s in sorted(SLOWEST_CI_S.items()):
            with self.subTest(row=row_id):
                self.assertGreaterEqual(
                    go_s[row_id], 3 * slowest_s, f"{row_id}: go timeout {go_s[row_id]} s < 3 x slowest CI {slowest_s} s"
                )

    def test_a_race_stress_row_naming_two_packages_is_refused(self):
        row = types.SimpleNamespace(
            id="two-packages",
            command=("python3.15", "gate/runner/rows/race.py", "--repeat=20", "--timeout=5m", "./a", "./b"),
            expected_scope=("go", "./b"),
            timeout_s=480,
        )
        with self.assertRaises(AssertionError) as caught:
            self.assertStressRowFits(row, "./b")
        self.assertIn("./a", str(caught.exception))


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

    def test_narrowed_to_one_of_two_packages_is_red(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.race("./a"), "covered 1 ≠ expected 2")

    def test_a_data_race_fails_naming_its_test(self):
        self.package("a")
        self.package("b", self.RACY)
        self.assertEqual(self.race(), "exit 1: ex.com/m/b: TestRacy")
        self.assertIn("ex.com/m/b: TestRacy", self.stdout("race", "race"))

    def go_calls(self, *args):
        """The row's reason and every go call it made, logged by a `go` ahead of the pinned one that then becomes it.
        The log exists, empty, before the row starts, so a row that never runs go reads as no calls."""
        log = os.path.join(self.root, "go.argv")
        with open(log, "x", encoding="utf-8"):
            pass
        real = os.path.join(support.tool_bin("go"), "go")
        logbin = os.path.join(self.root, "logbin")
        self.script("go", logbin, f"LOG, REAL = {log!r}, {real!r}\n{LOGGED_GO}")
        self.toolchain.dirs = [logbin, *self.toolchain.dirs]
        self.package("a")
        reason = self.race(*args)
        with open(log, encoding="utf-8") as f:
            return reason, [json.loads(line) for line in f]

    def go_test_argv(self, *args):
        """go test's arguments as the row gave them."""
        reason, calls = self.go_calls(*args)
        self.assertEqual(reason, "covered 1")
        tests = [c for c in calls if c[:1] == ["test"]]
        self.assertEqual(len(tests), 1, calls)
        return tests[0]

    def assertRefusedBeforeGo(self, reason, *args):
        """The row refuses `args` as a usage failure and never runs go test."""
        got, calls = self.go_calls(*args)
        self.assertEqual(got, reason)
        # The logged go was the row's go and had its chance: the runner's scope check (go list) went through it.
        self.assertIn(["list", "-e", "-json", "./..."], calls)
        self.assertEqual([c for c in calls if c[:1] == ["test"]], [], calls)
        self.assertFalse(os.path.exists(os.path.join(self.run_dir, "rows", "race", "race", "test.json")))

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

    # One grammar for N, covered.py's for a count: a whole number from 1 to 999999999, ASCII digits, no sign, space,
    # underscore or leading zero. Anything else is refused before go runs, with one message.
    USAGE_N = "exit 1: usage: --repeat=N with N a whole number from 1 to 999999999, not "

    def test_a_repeat_that_is_not_a_number_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_N + "'x'", "--repeat=x", "./...")

    def test_a_repeat_of_zero_is_a_usage_failure(self):
        # go test's -count=0 runs no test, which the row would read as "no test ran": a weak reason for a bad flag.
        self.assertRefusedBeforeGo(self.USAGE_N + "'0'", "--repeat=0", "./...")

    def test_a_negative_repeat_is_a_usage_failure(self):
        # go test refuses -count=-1, which the row would read as a failed package: a weak reason for a bad flag.
        self.assertRefusedBeforeGo(self.USAGE_N + "'-1'", "--repeat=-1", "./...")

    def test_a_repeat_with_a_plus_sign_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_N + "'+3'", "--repeat=+3", "./...")

    def test_a_repeat_with_a_leading_space_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_N + "' 3'", "--repeat= 3", "./...")

    def test_a_repeat_with_an_underscore_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_N + "'1_0'", "--repeat=1_0", "./...")

    def test_a_repeat_with_a_leading_zero_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_N + "'03'", "--repeat=03", "./...")

    def test_a_repeat_in_non_ascii_digits_is_a_usage_failure(self):
        # ARABIC-INDIC DIGIT THREE, which int() reads as 3; written as an escape, never as the raw character.
        self.assertRefusedBeforeGo(self.USAGE_N + "'\u0663'", "--repeat=\u0663", "./...")

    def test_a_repeat_past_go_tests_count_is_a_usage_failure(self):
        # 2^63: go test's -count cannot hold it and exits 2, which the row would read as a bare "go test exit 2".
        self.assertRefusedBeforeGo(self.USAGE_N + "'9223372036854775808'", "--repeat=9223372036854775808", "./...")

    USAGE_ARG = "exit 1: usage: {!r} is not a package pattern, --repeat=N or --timeout=Nm"

    def test_a_repeat_with_its_value_as_the_next_argument_is_a_usage_failure(self):
        # The other arguments are package patterns, and no package pattern starts with a dash.
        self.assertRefusedBeforeGo(self.USAGE_ARG.format("--repeat"), "--repeat", "20", "./...")

    def test_a_dash_argument_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_ARG.format("-x"), "-x", "./...")

    # --timeout=Nm (S1.1c-11, lane C): a row's own go test -timeout, N whole minutes from 1 to 999 in the count's
    # grammar (ASCII digits, no sign, space, underscore or leading zero) and the unit `m` alone; go's duration would
    # overflow at 999999999m, and the registry's longest row is an hour.
    USAGE_T = "exit 1: usage: --timeout=Nm with N a whole number of minutes from 1 to 999, not "

    def test_a_timeout_gives_go_test_that_timeout_in_place_of_the_stress_timeout(self):
        self.assertEqual(
            self.go_test_argv("--repeat=3", "--timeout=40m", "./..."),
            ["test", "-race", "-json", "-count=3", "-timeout", "40m", "./..."],
        )

    def test_a_timeout_before_the_repeat_still_wins(self):
        self.assertEqual(
            self.go_test_argv("--timeout=7m", "--repeat=3", "./..."),
            ["test", "-race", "-json", "-count=3", "-timeout", "7m", "./..."],
        )

    def test_a_timeout_without_a_repeat_gives_go_test_that_timeout(self):
        self.assertEqual(
            self.go_test_argv("--timeout=7m", "./..."),
            ["test", "-race", "-json", "-count=1", "-timeout", "7m", "./..."],
        )

    def test_the_longest_timeout_reaches_go_test(self):
        self.assertEqual(
            self.go_test_argv("--timeout=999m", "./..."),
            ["test", "-race", "-json", "-count=1", "-timeout", "999m", "./..."],
        )

    def test_a_timeout_without_its_unit_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_T + "'10'", "--timeout=10", "./...")

    def test_a_timeout_in_seconds_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_T + "'600s'", "--timeout=600s", "./...")

    def test_a_timeout_that_is_not_a_number_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_T + "'xm'", "--timeout=xm", "./...")

    def test_a_timeout_of_zero_is_a_usage_failure(self):
        # go test's -timeout 0 disables the timeout: the opposite of what the flag is for.
        self.assertRefusedBeforeGo(self.USAGE_T + "'0m'", "--timeout=0m", "./...")

    def test_a_timeout_with_a_leading_zero_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_T + "'05m'", "--timeout=05m", "./...")

    def test_a_timeout_with_a_sign_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_T + "'+5m'", "--timeout=+5m", "./...")

    def test_a_timeout_past_999_minutes_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_T + "'1000m'", "--timeout=1000m", "./...")

    def test_a_timeout_in_non_ascii_digits_is_a_usage_failure(self):
        # ARABIC-INDIC DIGIT FIVE, written as an escape, never as the raw character.
        self.assertRefusedBeforeGo(self.USAGE_T + "'\u0665m'", "--timeout=\u0665m", "./...")

    def test_a_timeout_with_its_value_as_the_next_argument_is_a_usage_failure(self):
        self.assertRefusedBeforeGo(self.USAGE_ARG.format("--timeout"), "--timeout", "7m", "./...")


class Cpu(GoRows):
    """The cpu row (S1.1c-11, lane C): `go test -json -count=1 -cpu=1`, then `-cpu=4`, two invocations so that each
    failure line names the -cpu value it failed at (one `-cpu 1,4` run names a test the same at both)."""

    SCOPE = ("go", "./...")
    GOOD = Unit.GOOD
    BAD = Unit.BAD
    # Fails only when the test runs on one OS thread for Go code (plan check counter-example 20).
    ONE = (
        'package p\n\nimport (\n\t"runtime"\n\t"testing"\n)\n\n'
        'func TestProcs(t *testing.T) {\n\tif runtime.GOMAXPROCS(0) == 1 {\n\t\tt.Fatal("GOMAXPROCS 1")\n\t}\n}\n'
    )
    FOUR = ONE.replace("== 1", "== 4").replace("GOMAXPROCS 1", "GOMAXPROCS 4")

    def package(self, name, test=GOOD):
        self.track(f"{name}/a.go", "package p\n\nfunc F() int { return 1 }\n")
        self.track(f"{name}/a_test.go", test)

    def cpu_row(self, *args):
        return self.row("cpu", command=wrapper("cpu", *args), expected_scope=self.SCOPE, job="cpu")

    def cpu(self, *args):
        self.run_rows([self.cpu_row(*args)])
        return self.reason("cpu", "cpu")

    def test_passing_packages(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.cpu(), "covered 2")

    def test_each_invocation_runs_every_test_once(self):
        self.package("a")
        self.assertEqual(self.cpu(), "covered 1")
        passes = b'"Action":"pass","Package":"ex.com/m/a","Test":"TestOk"'
        for value in (1, 4):
            self.assertEqual(self.row_file("cpu", "cpu", f"test.cpu{value}.json").count(passes), 1, value)

    def test_a_test_failing_only_at_one_proc_fails_naming_cpu_1_and_the_unit_row_passes_it(self):
        # Counter-example 20: the unit row runs at the machine's GOMAXPROCS and passes; the cpu row names -cpu 1.
        self.assertGreater(
            os.cpu_count() or 1,
            1,
            "the unit-row half needs more than one CPU: on one, GOMAXPROCS is 1 and the unit row fails TestProcs too",
        )
        self.package("a", self.ONE)
        unit_row = self.row("unit", command=wrapper("unit"), expected_scope=self.SCOPE, job="test")
        self.run_rows([unit_row, self.cpu_row()])
        self.assertEqual(self.reason("unit", "test"), "covered 1")
        self.assertEqual(self.reason("cpu", "cpu"), "exit 1: -cpu 1: ex.com/m/a: TestProcs")
        out = self.stdout("cpu", "cpu")
        self.assertIn("failed: -cpu 1: ex.com/m/a: TestProcs", out)
        self.assertNotIn("-cpu 4: ex.com/m/a", out)

    def test_a_test_failing_only_at_four_procs_fails_naming_cpu_4(self):
        self.package("a", self.FOUR)
        self.assertEqual(self.cpu(), "exit 1: -cpu 4: ex.com/m/a: TestProcs")
        self.assertNotIn("-cpu 1: ex.com/m/a", self.stdout("cpu", "cpu"))

    def test_a_test_failing_at_both_values_is_named_at_each(self):
        self.package("a", self.BAD)
        self.assertEqual(self.cpu(), "exit 1: -cpu 1: ex.com/m/a: TestBad")
        out = self.stdout("cpu", "cpu")
        self.assertIn("failed: -cpu 1: ex.com/m/a: TestBad", out)
        self.assertIn("failed: -cpu 4: ex.com/m/a: TestBad", out)

    def test_no_test_at_all_fails_at_each_value(self):
        self.track("a/a.go", "package a\n")
        self.assertEqual(self.cpu(), "exit 1: -cpu 1: no test ran")
        self.assertIn("failed: -cpu 4: no test ran", self.stdout("cpu", "cpu"))

    def test_narrowed_to_one_of_two_packages_is_red(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.cpu("./a"), "covered 1 ≠ expected 2")

    def test_go_test_gets_each_cpu_value_and_the_rows_timeout_in_order(self):
        log = os.path.join(self.root, "go.argv")
        with open(log, "x", encoding="utf-8"):
            pass
        real = os.path.join(support.tool_bin("go"), "go")
        logbin = os.path.join(self.root, "logbin")
        self.script("go", logbin, f"LOG, REAL = {log!r}, {real!r}\n{LOGGED_GO}")
        self.toolchain.dirs = [logbin, *self.toolchain.dirs]
        self.package("a")
        self.assertEqual(self.cpu("./..."), "covered 1")
        with open(log, encoding="utf-8") as f:
            tests = [c for c in map(json.loads, f) if c[:1] == ["test"]]
        timeout = ["-timeout", cpu_row.TIMEOUT]
        self.assertEqual(
            tests,
            [
                ["test", "-json", "-count=1", "-cpu=1", *timeout, "./..."],
                ["test", "-json", "-count=1", "-cpu=4", *timeout, "./..."],
            ],
        )

    def test_a_dash_argument_is_a_usage_failure(self):
        self.package("a")
        self.assertEqual(self.cpu("-x", "./..."), "exit 1: usage: '-x' is not a package pattern")
        self.assertFalse(os.path.exists(os.path.join(self.run_dir, "rows", "cpu", "cpu", "test.cpu1.json")))
