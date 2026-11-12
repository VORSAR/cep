"""gate/runner/teeth.py (S1.1c-2 design §4.2, §5 lane D): the strength rule over synthetic driver sources, and the
parse of archcheck's -cases listing.

`python_cases(source)` is pure (ast, no import): (case names, faults). A counted case must reach, in live code, a red
helper (assertRed, assertGateFailed, assertParityStops, assertRefusedAtLoad, assertHookRefused, assertRegenRefused,
assertRow with ok the literal False) or a pass on its row (assertRow with ok the literal True on a row other than
"parity"), through at most two helper levels; assertParityPassed alone never counts; a driver needs one red case, and
zero cases is a fault. Every planted case below was run through the coordinator's strength_scan.py logic, which
agrees with each expectation here.

design ambiguity: the fault texts. v1 named `teeth: <path>::<method> asserts no gate verdict`, `teeth: <path> plants
nothing red` and `teeth: <path> counts 0 cases`, but `python_cases` takes a source and no path; these tests assert the
method's name inside a case fault and the endings `plants nothing red` / `counts 0 cases` only.
design ambiguity: the case names' form (`test_x` or `Class.test_x`); asserted by count and by the method name.
design ambiguity: indirection through `support` cannot be resolved from the driver's source alone; these fixtures use
the driver's own methods only.
"""

import unittest

from gate.runner import teeth

HEAD = "from gate.proofs.drivers import support\n\n\nclass D(support.DriverTest):\n"
RED = "    def test_red(self):\n        run = self.run_row()\n        self.assertRed(run, 'lint', 'x', 'exit 1')\n\n"


def case(name: str, *body: str) -> str:
    return f"    def {name}(self):\n" + "".join(f"        {line}\n" for line in body) + "\n"


def method(name: str, *body: str) -> str:
    return f"    def {name}(self, run):\n" + "".join(f"        {line}\n" for line in body) + "\n"


class PythonCases(unittest.TestCase):
    def assertClean(self, source: str, n: int) -> None:
        cases, faults = teeth.python_cases(source)
        self.assertEqual((len(cases), faults), (n, []), cases)

    def assertOneFault(self, source: str, name: str) -> None:
        cases, faults = teeth.python_cases(source)
        self.assertEqual(len(faults), 1, faults)
        self.assertIn(name, faults[0])

    def test_each_red_helper_is_a_red_case(self):
        for helper in (
            "assertRed",
            "assertGateFailed",
            "assertParityStops",
            "assertRefusedAtLoad",
            "assertHookRefused",
            "assertRegenRefused",
        ):
            with self.subTest(helper=helper):
                self.assertClean(HEAD + case("test_a", "run = self.run_row()", f"self.{helper}(run, 'why')"), 1)

    def test_assert_row_false_positional_is_red(self):
        self.assertClean(
            HEAD + case("test_a", "run = self.run_row()", "self.assertRow(run, 'lint', 'x', False, 'r')"), 1
        )

    def test_assert_row_ok_false_by_keyword_is_red(self):
        src = HEAD + case("test_a", "run = self.run_row()", "self.assertRow(run, 'lint', 'x', ok=False, reason='r')")
        self.assertClean(src, 1)

    def test_a_pass_on_the_row_under_test_counts_beside_a_red_case(self):
        green = case("test_green", "run = self.run_row()", "self.assertRow(run, 'lint', 'docsync', True, 'covered 1')")
        self.assertClean(HEAD + RED + green, 2)

    def test_a_pass_on_parity_is_no_pass_on_the_row(self):
        green = case("test_parity_row", "run = self.run_row()", "self.assertRow(run, 'lint', 'parity', True, 'ok')")
        self.assertOneFault(HEAD + RED + green, "test_parity_row")

    def test_assert_parity_passed_alone_never_counts(self):
        # DQ-B4: a red assertion swapped for assertParityPassed must be red here.
        hollow = case("test_parity_only", "run = self.run_row()", "self.assertParityPassed(run)")
        self.assertOneFault(HEAD + RED + hollow, "test_parity_only")

    def test_a_verdict_after_return_is_dead(self):
        dead = case("test_after_return", "run = self.run_row()", "return", "self.assertRed(run, 'lint', 'x', 'r')")
        self.assertOneFault(HEAD + RED + dead, "test_after_return")

    def test_a_verdict_under_if_false_is_dead(self):
        dead = case("test_if_false", "run = self.run_row()", "if False:", "    self.assertRed(run, 'lint', 'x', 'r')")
        self.assertOneFault(HEAD + RED + dead, "test_if_false")

    def test_a_verdict_in_a_nested_def_never_called_is_dead(self):
        dead = case(
            "test_nested",
            "run = self.run_row()",
            "def later():",
            "    self.assertRed(run, 'lint', 'x', 'r')",
        )
        self.assertOneFault(HEAD + RED + dead, "test_nested")

    def test_a_nested_def_that_is_called_counts(self):
        live = case(
            "test_nested_called",
            "run = self.run_row()",
            "def now():",
            "    self.assertRed(run, 'lint', 'x', 'r')",
            "now()",
        )
        self.assertClean(HEAD + live, 1)

    def test_two_helper_levels_count(self):
        # the shape test_notbuilt_expired.py has: assertExpired -> assertFailReason -> assertGateFailed.
        src = (
            HEAD
            + case("test_two", "run = self.run_row()", "self.first(run)")
            + method("first", "self.second(run)")
            + method("second", "self.assertGateFailed(run, 'r')")
        )
        self.assertClean(src, 1)

    def test_three_helper_levels_are_too_deep(self):
        src = (
            HEAD
            + RED
            + case("test_three", "run = self.run_row()", "self.first(run)")
            + method("first", "self.second(run)")
            + method("second", "self.third(run)")
            + method("third", "self.assertGateFailed(run, 'r')")
        )
        self.assertOneFault(src, "test_three")

    def test_a_driver_with_only_green_cases_plants_nothing_red(self):
        green = case("test_green", "run = self.run_row()", "self.assertRow(run, 'lint', 'docsync', True, 'covered 1')")
        cases, faults = teeth.python_cases(HEAD + green)
        self.assertEqual(len(cases), 1)
        self.assertEqual(len(faults), 1, faults)
        self.assertTrue(faults[0].endswith("plants nothing red"), faults)

    def test_zero_cases_is_a_fault(self):
        cases, faults = teeth.python_cases(HEAD + method("helper", "self.assertRed(run, 'lint', 'x', 'r')"))
        self.assertEqual(cases, [])
        self.assertEqual(len(faults), 1, faults)
        self.assertTrue(faults[0].endswith("counts 0 cases"), faults)

    def test_a_helper_that_is_not_a_test_is_not_counted(self):
        src = HEAD + RED + method("helper_not_a_case", "self.assertParityPassed(run)")
        self.assertClean(src, 1)


VERDICT = "self.assertRed(run, 'lint', 'x', 'r')"


class StrengthRuleTightened(unittest.TestCase):
    """S1.1c-2's fix round (BK-B3, BK-B4): a red call whose failure is swallowed or that cannot run is no red case; a
    driver may not define a verdict helper's name; a module shape that changes what unittest loads is a fault, and
    only a class whose bases reach unittest.TestCase or the support driver bases is counted."""

    def assertCaseFaulted(self, *body: str) -> None:
        source = HEAD + RED + case("test_a", "run = self.run_row()", *body) + method("first", VERDICT)
        cases, faults = teeth.python_cases(source, "d.py")
        self.assertEqual(cases, ["D.test_red", "D.test_a"])
        self.assertEqual(faults, ["teeth: d.py::D.test_a asserts no gate verdict"])

    def assertCounted(self, *body: str) -> None:
        source = HEAD + case("test_a", "run = self.run_row()", *body) + method("first", VERDICT)
        cases, faults = teeth.python_cases(source, "d.py")
        self.assertEqual((cases, faults), (["D.test_a"], []))

    def test_a_red_call_whose_failure_is_swallowed_does_not_count(self):
        for why, body in {
            "contextlib.suppress": ("with contextlib.suppress(AssertionError):", f"    {VERDICT}"),
            "assertRaises": ("with self.assertRaises(AssertionError):", f"    {VERDICT}"),
            "a context of the driver's own": ("with self.quietly():", f"    {VERDICT}"),
            "a plain resource beside a swallowing one": (
                "with open('f') as f, contextlib.suppress(AssertionError):",
                f"    {VERDICT}",
            ),
            "a bare except": ("try:", f"    {VERDICT}", "except:", "    pass"),
            "except AssertionError": ("try:", f"    {VERDICT}", "except AssertionError:", "    pass"),
            "except Exception": ("try:", f"    {VERDICT}", "except Exception:", "    pass"),
            "except BaseException": ("try:", f"    {VERDICT}", "except BaseException:", "    pass"),
            "a tuple holding AssertionError": (
                "try:",
                f"    {VERDICT}",
                "except (OSError, AssertionError):",
                "    pass",
            ),
            "a helper called under suppress": ("with contextlib.suppress(AssertionError):", "    self.first(run)"),
        }.items():
            with self.subTest(why=why):
                self.assertCaseFaulted(*body)

    def test_a_red_call_under_a_plain_resource_or_a_narrow_handler_counts(self):
        for why, body in {
            "open": ("with open('f') as f:", f"    {VERDICT}"),
            "subTest": ("with self.subTest(x=1):", f"    {VERDICT}"),
            "tempfile": ("with tempfile.TemporaryDirectory() as d:", f"    {VERDICT}"),
            "except OSError": ("try:", f"    {VERDICT}", "except OSError:", "    pass"),
            "finally": ("try:", f"    {VERDICT}", "finally:", "    pass"),
            "in the handler of a wide except": ("try:", "    pass", "except Exception:", f"    {VERDICT}"),
            "if True": ("if True:", f"    {VERDICT}"),
            "the else of if 0": ("if 0:", "    pass", "else:", f"    {VERDICT}"),
            "a loop over a non-empty literal": ("for _ in (1,):", f"    {VERDICT}"),
            "a helper called plainly": ("self.first(run)",),
        }.items():
            with self.subTest(why=why):
                self.assertCounted(*body)

    def test_a_red_call_that_cannot_run_does_not_count(self):
        for why, body in {
            "if None": ("if None:", f"    {VERDICT}"),
            "if an empty string": ("if '':", f"    {VERDICT}"),
            "if an empty tuple": ("if ():", f"    {VERDICT}"),
            "the else of if 1": ("if 1:", "    pass", "else:", f"    {VERDICT}"),
            "while False": ("while False:", f"    {VERDICT}"),
            "a loop over []": ("for _ in []:", f"    {VERDICT}"),
            "a loop over {}": ("for _ in {}:", f"    {VERDICT}"),
            "a loop over an empty string": ("for _ in '':", f"    {VERDICT}"),
            "after if/else that both return": ("if run:", "    return", "else:", "    return", VERDICT),
            "after if True: raise": ("if True:", "    raise SystemExit(0)", VERDICT),
            "after a with that returns": ("with open('f') as f:", "    return", VERDICT),
            "after a break in its loop": ("for _ in (1,):", "    break", f"    {VERDICT}"),
        }.items():
            with self.subTest(why=why):
                self.assertCaseFaulted(*body)

    def test_a_driver_may_not_define_a_verdict_helpers_name(self):
        for why, source in {
            "a method": HEAD + "    def assertGateFailed(self, run):\n        pass\n\n" + RED,
            "a class attribute": HEAD + "    assertRed = print\n\n" + RED,
            "an attribute set in a case": HEAD + case("test_b", "self.assertRow = print") + RED,
            "a module function": "def assertRed(*args):\n    pass\n\n\n" + HEAD + RED,
            "an import alias": "from os import path as assertRed\n" + HEAD + RED,
            "assertParityPassed": HEAD + "    def assertParityPassed(self, run, job):\n        pass\n\n" + RED,
        }.items():
            with self.subTest(why=why):
                _, faults = teeth.python_cases(source, "d.py")
                self.assertTrue(faults, f"{why}: a redefined verdict helper passes")

    def test_a_module_shape_that_changes_what_unittest_loads_is_a_fault(self):
        for why, tail in {
            "setattr": "\n\nsetattr(D, 'test_red', None)\n",
            "delattr": "\n\ndelattr(D, 'test_red')\n",
            "del": "\n\ndel D.test_red\n",
            "__test__": "\n\n__test__ = False\n",
            "load_tests assigned": "\n\nload_tests = None\n",
            "an attribute assigned": "\n\nD.test_red = None\n",
            "an if at module level": "\n\nif True:\n    D.test_red = None\n",
            "a call at module level": "\n\nprint(D)\n",
        }.items():
            with self.subTest(why=why):
                _, faults = teeth.python_cases(HEAD + RED + tail, "d.py")
                self.assertTrue(faults, f"{why}: the case is counted but may never run")

    def test_a_class_level_test_flag_is_a_fault(self):
        _, faults = teeth.python_cases(HEAD + "    __test__ = False\n\n" + RED, "d.py")
        self.assertTrue(faults)

    def test_a_class_whose_bases_reach_no_testcase_is_not_counted(self):
        for base in ("object", "support.World", "Mixin"):
            with self.subTest(base=base):
                source = HEAD.replace("support.DriverTest", base) + RED
                cases, faults = teeth.python_cases("class Mixin:\n    pass\n\n\n" + source, "d.py")
                self.assertEqual(cases, [])
                self.assertIn("teeth: d.py::D is not a unittest.TestCase: its cases never run", faults)

    def test_bases_that_reach_a_testcase_count(self):
        for head in (
            "import unittest\n\n\nclass D(unittest.TestCase):\n",
            HEAD.replace("support.DriverTest", "support.ParityDriver"),
            HEAD.replace("class D(support.DriverTest)", "class Base(support.DriverTest):\n    pass\n\n\nclass D(Base)"),
        ):
            with self.subTest(head=head):
                self.assertEqual(teeth.python_cases(head + RED, "d.py"), (["D.test_red"], []))

    def test_the_real_driver_shapes_stay_legal(self):
        source = (
            '"""A driver."""\n\nimport os\n\nfrom gate.proofs.drivers import support\n\nROW = "x"\n'
            "PLANT: str = 'a'\n\n\ndef helper(x):\n    return x\n\n\nclass D(support.DriverTest):\n" + RED
        )
        self.assertEqual(teeth.python_cases(source, "d.py"), (["D.test_red"], []))


class RedRows(unittest.TestCase):
    """BK-S7: the rows a driver's red cases land on: the literal row id passed to the red helper or to a helper of
    the driver's own (its `row_id` parameter), a module constant (`ROW`) named there, or the module's ROW for a red
    helper that takes no row."""

    def test_where_the_row_comes_from(self):
        row_helper = method("assertRedOn", "self.assertGateFailed(run)").replace("(self, run)", "(self, run, row_id)")
        for why, (source, rows) in {
            "a literal": (HEAD + RED, {"x"}),
            "assertRow with ok False": (
                HEAD + case("test_a", "run = 1", "self.assertRow(run, 'j', 'y', False, 'r')"),
                {"y"},
            ),
            "the ROW constant": (
                "ROW = 'docsync'\n" + HEAD + case("test_a", "run = 1", "self.assertRed(run, 'j', ROW, 'r')"),
                {"docsync"},
            ),
            "a helper's row_id": (
                HEAD + case("test_a", "run = 1", "self.assertRedOn(run, 'apidiff')") + row_helper,
                {"apidiff"},
            ),
            "ROW for a red helper without a row": (
                "ROW = 'gofmt'\n" + HEAD + case("test_a", "self.assertHookRefused((1, ''), 'r')"),
                {"gofmt"},
            ),
            "no ROW for a red helper without a row": (
                HEAD + case("test_a", "self.assertHookRefused((1, ''), 'r')"),
                set(),
            ),
            "a pass is no red row": (
                HEAD + RED + case("test_b", "run = 1", "self.assertRow(run, 'j', 'z', True, 'r')"),
                {"x"},
            ),
            "a swallowed red call": (
                HEAD
                + RED
                + case(
                    "test_b",
                    "run = 1",
                    "with contextlib.suppress(AssertionError):",
                    "    self.assertRed(run, 'j', 'w', 'r')",
                ),
                {"x"},
            ),
        }.items():
            with self.subTest(why=why):
                self.assertEqual(teeth.red_rows(source), frozenset(rows))


class GoCases(unittest.TestCase):
    # design §11's seam (it settles the ambiguity QA marked here): `-cases` prints `<path>: case <name>` and
    # `<path>: fault <TestX>: <why>` per file in argument order, and go_cases groups the listing by path.
    P = "internal/parser/keywords_test.go"
    Q = "internal/expr/docsync_test.go"

    def test_case_lines(self):
        listing = (
            f"{self.P}: case TestA\n{self.P}: case TestA/one\n{self.P}: case TestA/one/two\n{self.P}: case TestB\n"
        )
        self.assertEqual(teeth.go_cases(listing), {self.P: (["TestA", "TestA/one", "TestA/one/two", "TestB"], [])})

    def test_fault_lines(self):
        listing = f"{self.P}: case TestA\n{self.P}: fault TestT: t.Run over a table -cases cannot count\n"
        self.assertEqual(
            teeth.go_cases(listing), {self.P: (["TestA"], ["TestT: t.Run over a table -cases cannot count"])}
        )

    def test_two_files_are_grouped_by_path(self):
        listing = f"{self.P}: case TestA\n{self.Q}: case TestB\n{self.Q}: fault : expected 'package'\n"
        self.assertEqual(
            teeth.go_cases(listing), {self.P: (["TestA"], []), self.Q: (["TestB"], [": expected 'package'"])}
        )

    def test_an_empty_listing(self):
        self.assertEqual(teeth.go_cases(""), {})


if __name__ == "__main__":
    unittest.main()
