"""Classification from `go test -json` events plus the profile (plan check S6 as amended by design note §5).

Every expected line below was derived by hand from the S6 table, before the code existed.
"""

from gate.coverage import classify, coverprofile
from gate.coverage.classify import EventsError, Package
from gate.coverage.tests import support

P = "ex.com/m/internal/"
NAMES = (
    "allgen",
    "build",
    "c29",
    "crashed",
    "eightnine",
    "full",
    "gen",
    "ninety",
    "notests",
    "skipped",
    "zero",
    "zerofail",
)
# By hand, floor 90 (classes.prof and classes.json):
CLASSES = [
    "PASS ex.com/m/internal/allgen n/a (0 statements)",  # every block is in a generated file
    "FAIL ex.com/m/internal/build build failed",  # C28 (b)
    "FAIL ex.com/m/internal/c29 89.99% (17999/20000) < 90%",  # C29: 89.995, truncated, never 90.00
    "FAIL ex.com/m/internal/crashed missing from the profile",  # a test passed, then the binary died
    "FAIL ex.com/m/internal/eightnine 88.88% (8/9) < 90%",  # the boundary below
    "PASS ex.com/m/internal/full 100.00% (3/3) >= 90%",  # a skipped subtest beside a pass
    "PASS ex.com/m/internal/gen 100.00% (1/1) >= 90%",  # gen.go's 5 statements excluded from both sides
    "PASS ex.com/m/internal/ninety 90.00% (9/10) >= 90%",  # the boundary on the floor
    "FAIL ex.com/m/internal/notests no tests",  # C28 (c): package action pass, no run event
    "FAIL ex.com/m/internal/skipped no test ran",  # C31
    "PASS ex.com/m/internal/zero n/a (0 statements)",  # C28 (a)
    "FAIL ex.com/m/internal/zerofail n/a (0 statements), but no test passed",
]
GENERATED = {"gen": frozenset({"gen.go"}), "allgen": frozenset({"g.go"})}


def packages(*names: str) -> list[Package]:
    return [Package(P + name, GENERATED.get(name, frozenset())) for name in names]


def lines(verdicts: list[classify.Verdict]) -> list[str]:
    return [v.line() for v in verdicts]


class Classes(support.CoverageTest):
    def evaluate(self, names=NAMES, floor=90):
        blocks = coverprofile.parse(support.fixture("classes.prof"))
        events = classify.parse_events(support.fixture("classes.json"))
        return classify.evaluate(packages(*names), blocks, events, floor)

    def test_every_class_of_the_table(self):
        self.assertEqual(lines(self.evaluate()), CLASSES)

    def test_the_verdicts_follow_the_lines(self):
        self.assertEqual([v.ok for v in self.evaluate()], [line.startswith("PASS") for line in CLASSES])

    def test_packages_come_out_sorted_whatever_order_they_go_in(self):
        self.assertEqual(lines(self.evaluate(tuple(reversed(NAMES)))), CLASSES)

    def test_only_the_packages_asked_for_are_evaluated(self):
        # The profile also holds ex.com/m/outside; it is not in scope, so it is never judged.
        self.assertEqual(lines(self.evaluate(("ninety",))), ["PASS ex.com/m/internal/ninety 90.00% (9/10) >= 90%"])

    def test_a_package_with_no_events_at_all_has_no_tests(self):
        self.assertEqual(lines(self.evaluate(("absent",))), ["FAIL ex.com/m/internal/absent no tests"])

    def test_without_its_generated_list_a_package_counts_every_block(self):
        blocks = coverprofile.parse(support.fixture("classes.prof"))
        events = classify.parse_events(support.fixture("classes.json"))
        (verdict,) = classify.evaluate([Package(P + "gen")], blocks, events, 90)
        self.assertEqual(verdict.line(), "FAIL ex.com/m/internal/gen 16.66% (1/6) < 90%")

    def test_the_floor_is_a_parameter(self):
        self.assertEqual(lines(self.evaluate(("eightnine",), floor=88)), [f"PASS {P}eightnine 88.88% (8/9) >= 88%"])
        self.assertEqual(lines(self.evaluate(("ninety",), floor=91)), [f"FAIL {P}ninety 90.00% (9/10) < 91%"])

    def test_duplicated_blocks_count_once_with_the_higher_count(self):
        blocks = coverprofile.parse(support.fixture("dupes.prof"))
        events = classify.parse_events(support.fixture("dupes.json"))
        (verdict,) = classify.evaluate([Package("ex.com/m/internal/d")], blocks, events, 90)
        self.assertEqual(verdict.line(), "FAIL ex.com/m/internal/d 50.00% (5/10) < 90%")


class Percent(support.CoverageTest):
    def test_truncated_never_rounded_up(self):
        # By hand: covered * 10000 // total, shown as <int>.<2 digits>%.
        cases = {
            (17999, 20000): "89.99%",
            (8, 9): "88.88%",
            (9, 10): "90.00%",
            (2, 3): "66.66%",
            (1, 1): "100.00%",
            (0, 7): "0.00%",
            (1, 10000): "0.01%",
            (1, 10001): "0.00%",
        }
        for (covered, total), shown in cases.items():
            with self.subTest(covered=covered, total=total):
                self.assertEqual(classify.percent(covered, total), shown)

    def test_the_verdict_is_integer_arithmetic_on_the_floor(self):
        # 17999/20000 shows 89.99% and fails; 18000/20000 is exactly 90% and passes.
        events = [{"Action": "run", "Package": "p/q", "Test": "T"}, {"Action": "pass", "Package": "p/q", "Test": "T"}]
        for covered, ok in ((17999, False), (18000, True)):
            blocks = [
                coverprofile.Block("p/q/a.go", 1, 1, 2, 1, covered, 1),
                coverprofile.Block("p/q/a.go", 3, 1, 4, 1, 20000 - covered, 0),
            ]
            with self.subTest(covered=covered):
                (verdict,) = classify.evaluate([Package("p/q")], blocks, events, 90)
                self.assertIs(verdict.ok, ok)


class Events(support.CoverageTest):
    def test_one_object_per_line(self):
        self.assertEqual(len(classify.parse_events(support.fixture("classes.json"))), 59)

    def test_an_empty_stream_has_no_events(self):
        self.assertEqual(classify.parse_events(b""), [])

    def test_anything_but_an_object_per_line_is_refused_naming_the_line(self):
        cases = {
            "not JSON": b'{"Action":"run"}\nnot json\n',
            "an array": b"[1]\n",
            "two objects on a line": b'{"Action":"run"} {"Action":"pass"}\n',
        }
        for why, data in cases.items():
            with self.subTest(why=why), self.assertRaises(EventsError) as caught:
                classify.parse_events(data)
            self.assertRegex(str(caught.exception), r"^line [0-9]+: ")
