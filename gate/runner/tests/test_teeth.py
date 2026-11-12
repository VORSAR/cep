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


class GoCases(unittest.TestCase):
    # design ambiguity: whether `-cases` prefixes each line with its file when given several files (§4.2: one run
    # over every file); these listings are one file's, and the faults are taken as the text after "fault ".
    def test_case_lines(self):
        listing = "case TestA\ncase TestA/one\ncase TestA/one/two\ncase TestB\n"
        self.assertEqual(teeth.go_cases(listing), (["TestA", "TestA/one", "TestA/one/two", "TestB"], []))

    def test_fault_lines(self):
        listing = "case TestA\nfault TestT: t.Run over a table -cases cannot count\n"
        self.assertEqual(teeth.go_cases(listing), (["TestA"], ["TestT: t.Run over a table -cases cannot count"]))

    def test_an_empty_listing(self):
        self.assertEqual(teeth.go_cases(""), ([], []))


if __name__ == "__main__":
    unittest.main()
