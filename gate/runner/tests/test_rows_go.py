"""The real gofmt, vet and unit wrappers over temporary Go modules, with the pinned go (design note §5).

Teeth (QA DQ22, design-QA counter-example 33): each wrapper's count comes from the tool's own work, so a narrowed
run is red, naming both numbers. Each narrowed case has a full control that passes.
"""

import os
import sys

from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
MOD = "module ex.com/m\n\ngo 1.27.0\n"


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
        self.assertEqual(self.gofmt(), "exit 1")
        out = self.stdout("gofmt")
        self.assertIn("p/bad.go", out)
        self.assertIn("gofmt -w p/bad.go", out)

    def test_a_file_gofmt_cannot_parse_fails_naming_it(self):
        self.track("p/broken.go", "package p\nfunc {\n")
        self.assertEqual(self.gofmt(), "exit 1")
        self.assertIn("p/broken.go", self.stdout("gofmt"))

    def test_257_files_cross_the_batch_boundary(self):
        self.go_files(257)
        self.assertEqual(self.gofmt(), "covered 257")
        names = os.listdir(os.path.join(self.run_dir, "rows", "lint", "gofmt"))
        self.assertEqual(sorted(n for n in names if n.startswith("gofmt.") and n.endswith(".stdout")),
                         ["gofmt.0.stdout", "gofmt.1.stdout"])  # fmt: skip

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
        self.assertEqual(self.vet(), "exit 1")
        self.assertIn("c/c.go:5", self.stdout("vet"))

    def test_a_compile_error_prints_no_object_and_still_fails(self):
        self.track("a/a.go", "package a\n")
        self.track("b/b.go", 'package b\n\nvar x int = "s"\n')
        self.assertEqual(self.vet(), "exit 1")
        self.assertIn("b/b.go", self.stdout("vet"))

    def test_a_compile_error_only_in_a_test_file_fails(self):
        # re-check S5: `go build` is green here; vet's non-zero exit is the only signal.
        self.track("a/a.go", "package a\n")
        self.track("a/a_test.go", 'package a\n\nvar x int = "s"\n')
        self.assertEqual(self.vet(), "exit 1")


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
        self.assertEqual(self.unit(), "exit 1")
        out = self.stdout("unit", "test")
        self.assertIn("ex.com/m/b", out)
        self.assertIn("TestBad", out)

    def test_a_skipped_test_fails_naming_it(self):
        # Owner, 2026-11-03 (break-it QB7): a skipped Go test fails the row, as a skipped Python test fails pytests.
        skip = self.GOOD + '\nfunc TestLater(t *testing.T) { t.Skip("flaky") }\n'
        self.package("a", skip)
        self.assertEqual(self.unit(), "exit 1")
        out = self.stdout("unit", "test")
        self.assertIn("ex.com/m/a: TestLater: skipped", out)

    def test_no_test_at_all_fails(self):
        self.track("a/a.go", "package a\n")
        self.track("b/b.go", "package b\n")
        self.assertEqual(self.unit(), "exit 1")
        self.assertIn("no test ran", self.stdout("unit", "test"))

    def test_a_test_build_failure_fails(self):
        self.package("a", 'package p\n\nvar x int = "s"\n')
        self.assertEqual(self.unit(), "exit 1")

    def test_narrowed_to_one_of_two_packages_is_red(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.unit("./a"), "covered 1 ≠ expected 2")
