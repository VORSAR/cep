"""The smells row's planted proofs (S1.1c-5 design note §2, §8 row 1.13 and "the 30 cases").

Each case is one world: the gate's real files, a go.mod with no requirement, the planted Go files, and a registry
holding the `smells` row in §2.5's shape (ROW_DEF, copied here until the coordinator stages the real row). The world's
tool cache links every pinned tool, so the row's lint cache (`<tool cache>/golangci-lint-cache`) is new per world.
One real gate run per case at T2: the real pinned golangci-lint under linux and darwin. A red case names the plant
past the bound by file and line (`smells: <file>:<line>:`) and does not name the one at the bound; a green part of a
case is shown by the same run's other, named finding (the run had its chance).
"""

import json
import os

from gate.proofs.drivers import support

ROW = "smells"
JOB = "lint"
ROW_DEF = {
    "id": "smells",
    "job": "lint",
    "tiers": ["T0", "T1", "T2", "T3", "T4"],
    "blocking": True,
    "timeout_s": 600,
    "needs": [],
    "scope": ["prod", "test", "tools", "generated"],
    "expected_scope": ["git", ":(glob)**/*.go", ":(glob,exclude)gate/proofs/**"],
    "min_wants": 1,
    "lands_in": "S1.1c-5",
    "exclusive": False,
    "command": ["python3.15", "-I", "-B", "-X", "utf8", "gate/runner/rows/smells.py", "{files}"],
}
MODULE = "planted.example"
GO_MOD = f"module {MODULE}\n\ngo 1.27.0\n"
LIB = "internal/planted"
HEAD = "package planted\n\n"


def lines(*body: str) -> str:
    return "".join(f"{line}\n" for line in body)


def func(name: str, interior: int, *, comments: int = 0) -> str:
    """A function whose body spans `interior` lines between its `func` line and its `}`, `comments` of them comments."""
    body = ["\t// a counted comment line"] * comments + ["\t_ = 0"] * (interior - comments)
    return lines(f"func {name}() {{", *body, "}")


def branches(name: str, n: int) -> str:
    """Cyclomatic complexity n + 1: n `if`s."""
    body = [f"\tif x == {i} {{\n\t\treturn {i}\n\t}}" for i in range(n)]
    return lines(f"func {name}(x int) int {{", *body, "\treturn -1", "}")


class SmellsPlanted(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate()
        self.world.copy(".golangci.toml")
        self.world.write("go.mod", GO_MOD)
        self.cache = self.world.link_cache(*(t.name for t in support.tools()))

    # ---------------------------------------------------------------------------------------------------- the world

    def go(self, rel: str, text: str) -> None:
        self.world.write(rel, text)

    def entry(self, target: str) -> dict:
        return {
            "target": target,
            "kind": "TEMP",
            "reason": "size family not yet cleaned (planted)",
            "sprint": "S1.1c-5",
            "expiry": "2099-01-01",
            "issue": "S1.1c-5-1",
        }

    def smells_list(self, *targets: str) -> None:
        doc = {"schema": 1, "note": "planted", "rule": "smells", "entries": [self.entry(t) for t in targets]}
        self.world.write("tools/allowlists/smells.json", json.dumps(doc) + "\n")

    def generated_list(self, *targets: str) -> None:
        entries = [
            {"target": t, "kind": "PERMANENT", "reason": "a generated file, listed", "sprint": "S1.1c-2", "ref": "x"}
            for t in targets
        ]
        doc = {"schema": 1, "note": "planted", "rule": "generated", "entries": entries}
        self.world.write("tools/allowlists/generated.json", json.dumps(doc) + "\n")

    def config(self, old: str, new: str) -> None:
        path = os.path.join(self.world.repo, ".golangci.toml")
        text = support.read_text(path)
        self.assertIn(old, text)
        self.world.write(".golangci.toml", text.replace(old, new, 1))

    def run_row(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(ROW_DEF), cache_dir=self.cache)

    # ----------------------------------------------------------------------------------------------- the verdicts

    def failed(self, run: support.Run) -> list[str]:
        out = run.row_stdout(JOB, ROW)
        return [x.removeprefix("failed: smells: ") for x in out.split("\n") if x.startswith("failed: smells: ")]

    def assertSmellsRed(self, run: support.Run, named: list[str], unnamed: list[str] = ()) -> list[str]:
        """Red; each of `named` starts some failed line; none of `unnamed` starts any."""
        self.assertParityPassed(run, JOB)
        self.assertGateFailed(run)
        self.assertEqual(run.record(JOB, ROW)["verdict"], "fail", run.stdout)
        failed = self.failed(run)
        for want in named:
            self.assertTrue([x for x in failed if x.startswith(want)], (want, failed))
        for not_want in unnamed:
            self.assertFalse([x for x in failed if x.startswith(not_want)], (not_want, failed))
        return failed

    # -------------------------------------------------------------------------------------------------- the cases

    def test_function_60_lines_green_61_red_with_comments_counted(self):
        self.go(f"{LIB}/p.go", HEAD + func("A", 60, comments=59) + func("B", 61, comments=60))
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:65:"], [f"{LIB}/p.go:3:"])

    def test_cyclomatic_10_green_11_red(self):
        self.go(f"{LIB}/p.go", HEAD + branches("A", 9) + branches("B", 10))
        b_line = 3 + (1 + 9 * 3 + 2)  # A spans its func line, nine 3-line ifs, a return and its }
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:{b_line}:"], [f"{LIB}/p.go:3:"])

    def test_file_500_green_501_red(self):
        self.go(f"{LIB}/a.go", HEAD + lines(*["// a line"] * 498))
        self.go(f"{LIB}/b.go", HEAD + lines(*["// a line"] * 499))
        self.assertSmellsRed(self.run_row(), [f"{LIB}/b.go:501:"], [f"{LIB}/a.go:"])

    def test_params_5_green_6_red(self):
        self.go(f"{LIB}/p.go", HEAD + lines("func A(a, b, c, d, e int) {}", "func B(a, b, c, d, e, f int) {}"))
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:4:"], [f"{LIB}/p.go:3:"])

    def test_line_120_green_121_red_a_tab_one_column(self):
        self.go(f"{LIB}/p.go", HEAD + lines("\t// " + "x" * 116, "\t// " + "x" * 117))
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:4:"], [f"{LIB}/p.go:3:"])

    def test_a_test_file_is_exempt_from_size_and_lll_not_from_panic(self):
        self.go(f"{LIB}/p.go", HEAD)
        body = func("A", 61) + branches("B", 10) + lines("func C(a, b, c, d, e, f int) {}", "// " + "x" * 130)
        self.go(f"{LIB}/p_test.go", HEAD + body + lines("func D() {", '\tpanic("x")', "}"))
        panic_line = 3 + 63 + 33 + 2 + 1  # after A (63 lines), B (33), C and the long comment, D's func line
        failed = self.assertSmellsRed(self.run_row(), [f"{LIB}/p_test.go:{panic_line}:"])
        self.assertEqual([x for x in failed if x.startswith(f"{LIB}/")], [x for x in failed if "[panic]" in x], failed)

    def test_println_print_fmt_println_red_in_a_library(self):
        self.go(
            f"{LIB}/p.go",
            HEAD + lines('import "fmt"', "", "func F() {", "\tprintln(1)", "\tprint(1)", "\tfmt.Println(1)", "}"),
        )
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:{n}:" for n in (6, 7, 8)])

    def test_fmtprint_green_in_a_main_tool_red_in_a_non_main_tool(self):
        self.go(
            "tools/m/main.go", lines("package main", "", 'import "fmt"', "", "func main() {", "\tfmt.Println(1)", "}")
        )
        self.go("tools/n/n.go", lines("package n", "", 'import "fmt"', "", "func F() {", "\tfmt.Println(1)", "}"))
        self.assertSmellsRed(self.run_row(), ["tools/n/n.go:6:"], ["tools/m/main.go:"])

    def test_a_fourth_panic_beside_three_waived_is_red(self):
        waived = '\tpanic("x") //nolint:forbidigo // PERMANENT reason="a planted waiver"'
        self.go(f"{LIB}/p.go", HEAD + lines("func F() {", waived, waived, waived, '\tpanic("x")', "}"))
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:7:"], [f"{LIB}/p.go:{n}:" for n in (4, 5, 6)])

    def test_reflect_red_in_l2_green_in_l3_green_in_a_test(self):
        # ADR-0015 §2 smells: forbidigo bans `reflect` in L0-L2, which the §2.1 config's path-except rule does; the
        # design's §8 case name (red in L3, green in L2) is the other way round, reported to the coordinator.
        use = lines('import "reflect"', "", "func F() {", "\t_ = reflect.TypeOf(0)", "}")
        self.go(f"{LIB}/p.go", HEAD + use)
        self.go("internal/event/e.go", "package event\n\n" + use)
        self.go("internal/event/e_test.go", "package event\n\n" + use.replace("func F", "func G"))
        unnamed = [f"{LIB}/p.go:", "internal/event/e_test.go:"]
        self.assertSmellsRed(self.run_row(), ["internal/event/e.go:6:"], unnamed)

    def test_getenv_lookupenv_environ_red_green_under_cmd_and_io(self):
        use = lines(
            'import "os"',
            "",
            "func F() {",
            '\t_ = os.Getenv("A")',
            '\t_, _ = os.LookupEnv("A")',
            "\t_ = os.Environ()",
            "}",
        )
        self.go(f"{LIB}/p.go", HEAD + use)
        self.go("cmd/c/c.go", "package c\n\n" + use)
        self.go("io/x/x.go", "package x\n\n" + use)
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:{n}:" for n in (6, 7, 8)], ["cmd/c/c.go:", "io/x/x.go:"])

    def test_blank_unsafe_with_linkname_is_red(self):
        link = f"//go:linkname f {MODULE}/{LIB}.f"
        self.go(f"{LIB}/p.go", HEAD + lines('import _ "unsafe"', "", link, "func f() {}"))
        self.assertSmellsRed(self.run_row(), [f"{MODULE}/{LIB}: imports unsafe"])

    def test_unsafe_in_an_xtest_is_red(self):
        self.go(f"{LIB}/p.go", HEAD)
        self.go(
            f"{LIB}/x_test.go", lines("package planted_test", "", 'import "unsafe"', "", "var _ = unsafe.Sizeof(0)")
        )
        self.assertSmellsRed(self.run_row(), [f"{MODULE}/{LIB}: imports unsafe"])

    def test_findings_in_an_example_function_are_red(self):
        self.go(f"{LIB}/p.go", HEAD)
        body = lines('import "os"', "", "func ExampleF() {", '\t_ = os.Getenv("A")', '\tpanic("x")', "}")
        self.go(f"{LIB}/p_test.go", HEAD + body)
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p_test.go:6:", f"{LIB}/p_test.go:7:"])

    def test_unchecked_close_red_written_out_exclusion_green(self):
        body = lines(
            'import (\n\t"bytes"\n\t"fmt"\n\t"os"\n)',
            "",
            "func F(rt *os.File, b *bytes.Buffer) {",
            "\trt.Close()",
            '\tb.WriteString("x")',
            '\tfmt.Fprintln(os.Stdout, "x")',
            "}",
        )
        self.go(f"{LIB}/p.go", HEAD + body)
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:10:", f"{LIB}/p.go:12:"], [f"{LIB}/p.go:11:"])

    def test_gocritic_append_assign_is_red(self):
        body = lines("func F(y []int) []int {", "\tvar x []int", "\tx = append(y, 1)", "\treturn x", "}")
        self.go(f"{LIB}/p.go", HEAD + body)
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:5:"])

    def test_a_waiver_on_the_wrong_line_is_red_and_the_finding_stays(self):
        # The line just above: golangci-lint v2.15.0 widens the //nolint (column 8) to the next line's node that starts
        # at column 8, the "x" literal, so the panic is silenced and nolintlint is quiet; the row reads the nolint
        # filter's record and names the panic (coordinator, during S1.1c-5's W1).
        waiver = '\t_ = 1 //nolint:forbidigo // PERMANENT reason="on the wrong line"'
        self.go(f"{LIB}/p.go", HEAD + lines("func F() {", waiver, '\tpanic("x")', "}"))
        silenced = f"{LIB}/p.go:5:2: forbidigo: silenced by the //nolint on line 4; a //nolint waives only its own line"
        self.assertSmellsRed(self.run_row(), [silenced])

    def test_nolint_spellings_golangci_honours_are_red(self):
        forms = ["// nolint:forbidigo // a reason", "//nolint", "//nolint:forbidigo"]
        body = lines("func F() {", *(f'\tpanic("x") {form}' for form in forms), "}")
        self.go(f"{LIB}/p.go", HEAD + body)
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:{n}:" for n in (4, 5, 6)])

    def test_a_typecheck_error_in_an_excluded_package_is_red_beside_an_in_scope_finding(self):
        self.go("internal/q/q.go", lines("package q", "", 'var X int = "s"'))
        self.go(f"{LIB}/p.go", HEAD + lines("func F() {", '\tpanic("x")', "}"))
        self.smells_list("internal/q gocyclo")
        # golangci-lint reports a package that does not type-check at its line 1 and then reports no other finding
        # (measured): the in-scope panic is hidden, and the run is red on the typecheck issue, never dropped.
        failed = self.assertSmellsRed(self.run_row(), ["internal/q/q.go:1:0: typecheck: "])
        self.assertFalse([x for x in failed if x.startswith("internal/q/") and "typecheck" not in x], failed)

    def test_a_generated_header_not_in_generated_json_is_linted(self):
        gen = "// Code generated by planted. DO NOT EDIT.\n\n" + HEAD + lines("func F() {", '\tpanic("x")', "}")
        self.go(f"{LIB}/a.go", gen)
        self.go(f"{LIB}/b.go", gen.replace("func F", "func G"))
        self.generated_list(f"{LIB}/b.go")
        self.assertSmellsRed(self.run_row(), [f"{LIB}/a.go:6:"], [f"{LIB}/b.go:"])

    def test_two_findings_on_one_line_are_both_red(self):
        self.go(f"{LIB}/p.go", HEAD + lines('import "os"', "", "func F() {", '\tpanic(os.Getenv("A"))', "}"))
        failed = self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:6:"])
        self.assertEqual(len([x for x in failed if x.startswith(f"{LIB}/p.go:6:")]), 2, failed)

    def test_fifty_one_findings_of_one_linter_are_all_red(self):
        self.go(f"{LIB}/p.go", HEAD + lines("func F() {", *['\tpanic("x")'] * 51, "}"))
        failed = self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:4:", f"{LIB}/p.go:54:"])
        self.assertEqual(len([x for x in failed if "[panic]" in x]), 51, failed)

    def test_a_linux_only_and_a_darwin_only_finding_are_both_red(self):
        self.go(f"{LIB}/p.go", HEAD)

        def panic(name: str) -> str:
            return lines(f"func {name}() {{", '\tpanic("x")', "}")

        self.go(f"{LIB}/p_linux.go", HEAD + panic("L"))
        self.go(f"{LIB}/p_darwin.go", HEAD + panic("D"))
        self.go(f"{LIB}/p_nocgo.go", "//go:build !cgo\n\n" + HEAD + panic("N"))
        named = [f"{LIB}/p_linux.go:4:", f"{LIB}/p_darwin.go:4:", f"{LIB}/p_nocgo.go:6:"]
        self.assertSmellsRed(self.run_row(), named)

    def test_unclaimed_files_are_red(self):
        self.go(f"{LIB}/p.go", HEAD)
        self.go("_dir/x.go", "package x\n")
        self.go(f"{LIB}/i.go", "//go:build ignore\n\n" + HEAD)
        self.go(f"{LIB}/c.go", "//go:build cgo\n\n" + HEAD)
        self.go("nested/go.mod", "module nested.example\n\ngo 1.27.0\n")
        self.go("nested/n.go", "package nested\n")
        self.go(f"{LIB}/testdata/t.go", "package t\n")
        named = [
            "_dir/x.go: no package under linux or darwin claims it",
            f"{LIB}/i.go: excluded by build constraints under linux and darwin",
            f"{LIB}/c.go: excluded by build constraints under linux and darwin",
            "nested/go.mod: a nested module is never linted",
            "nested/n.go: no package under linux or darwin claims it",
        ]
        self.assertSmellsRed(self.run_row(), named, [f"{LIB}/testdata/"])

    def test_a_hiding_config_edit_is_refused_before_lint(self):
        self.config("[issues]\n", '[issues]\nnew-from-rev = "HEAD"\n')
        self.go(f"{LIB}/p.go", HEAD + lines("func F() {", '\tpanic("x")', "}"))
        run = self.run_row()
        self.assertSmellsRed(run, [".golangci.toml: issues.new-from-rev: not pinned (remove it)"], [f"{LIB}/"])
        for goos in ("linux", "darwin"):
            self.assertFalse(os.path.exists(os.path.join(run.run_dir, "rows", JOB, ROW, f"smells-{goos}.json")))

    def test_a_float_argument_limit_is_red(self):
        self.config("arguments = [5]\n", "arguments = [5.0]\n")
        self.go(f"{LIB}/p.go", HEAD)
        named = [".golangci.toml: linters.settings.revive.rules[1].arguments[0]: 5.0 ≠ pinned 5"]
        self.assertSmellsRed(self.run_row(), named)

    def test_an_excluded_package_hides_only_its_rule(self):
        self.go(f"{LIB}/p.go", HEAD + branches("A", 10) + func("B", 61))
        self.smells_list(f"{LIB} gocyclo")
        b_line = 3 + (1 + 10 * 3 + 2)
        self.assertSmellsRed(self.run_row(), [f"{LIB}/p.go:{b_line}:"], [f"{LIB}/p.go:3:"])

    def test_an_exclusion_that_waives_nothing_is_red(self):
        self.go(f"{LIB}/p.go", HEAD + func("A", 3))
        self.smells_list(f"{LIB} funlen")
        named = [
            f"tools/allowlists/smells.json: {LIB} funlen waives nothing: the package is clean for funlen; remove it"
        ]
        self.assertSmellsRed(self.run_row(), named)

    def test_a_new_package_not_clean_is_red(self):
        panic = lines("func F() {", '\tpanic("x")', "}")
        dirs = ("examples/e", "third_party/t", "Godeps/g", "builtin/b", "internal/fresh")
        for d in dirs:
            name = d.rsplit("/", 1)[1]
            self.go(f"{d}/{name}.go", f"package {name}\n\n" + panic)
        self.assertSmellsRed(self.run_row(), [f"{d}/{d.rsplit('/', 1)[1]}.go:4:" for d in dirs])

    def test_a_line_directive_cannot_move_a_finding_into_a_main_tool(self):
        body = lines('import "fmt"', "", "func F() {", "//line ../tools/t/main.go:3", "\tfmt.Println(1)", "}")
        self.go("gen/g.go", "package gen\n\n" + body)
        self.assertSmellsRed(self.run_row(), ["tools/t/main.go:3: forbidigo: reported at a file no package claims"])
