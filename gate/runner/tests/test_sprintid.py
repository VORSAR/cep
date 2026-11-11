"""The sprint-id grammar, the printed order and the marker (S1.1c-1 lane D, plan check v2 section 2 S2 and
section 6 "The grammar, the order, the marker"; sprint-plan.md section 3's grammar S<stage>[.<part>][<letter>][-<n>]).

Pure: strings in, values out. The order tests read gate/sprints.json and docs/sprints/sprint-plan.md as they are.
"""

import json
import os
import unittest

from gate.runner import sprintid

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
SPRINTS = os.path.join(REPO, "gate", "sprints.json")
PLAN = os.path.join(REPO, "docs", "sprints", "sprint-plan.md")
HEADER = "| Sprint | Title | SP (items) | Routine | Depends on |"
TICK = "✓"
CROSS = "✗"
VALID = ["S3.3-2", "S1.1a-1", "S2.2a", "S10", "S3-1", "S1.1c-1", "S9.2", "S12", "S1.2"]
INVALID = [
    "s3.3",
    "S3.3-",
    "S3.3-2x",
    "S03.3",
    "S3.03",
    "S3.3-0",
    "S3.3-01",
    "S3.3A",
    "S3.3ab",
    "S0",
    "S3.0",
    "",
    " S1.1c-1",
    "S1.1c-1 ",
]


def read(path: str) -> str:
    with open(path, encoding="utf-8") as f:
        return f.read()


def order() -> sprintid.Order:
    return sprintid.load_order(read(SPRINTS))


class Grammar(unittest.TestCase):
    def test_parse_and_row_of(self):
        table = [
            ("S3.3-2", sprintid.ID(3, 3, "", 2), "S3.3"),
            ("S1.1a-1", sprintid.ID(1, 1, "a", 1), "S1.1a"),
            ("S2.2a", sprintid.ID(2, 2, "a", 0), "S2.2a"),
            ("S10", sprintid.ID(10, 0, "", 0), "S10"),
            ("S1.1c-1", sprintid.ID(1, 1, "c", 1), "S1.1c"),
        ]
        for text, want, row in table:
            with self.subTest(text=text):
                parsed = sprintid.parse(text)
                self.assertEqual(parsed, want)
                self.assertEqual(sprintid.row_of(parsed), row)

    def test_errors_name_the_text(self):
        for text in INVALID:
            with self.subTest(text=text), self.assertRaises(ValueError) as caught:
                sprintid.parse(text)
            if text:
                self.assertIn(text, str(caught.exception))

    def test_canonical_round_trips(self):
        for text in VALID:
            with self.subTest(text=text):
                self.assertEqual(sprintid.canonical(sprintid.parse(text)), text)

    def test_a_stage_with_a_part_number_and_no_part_is_valid(self):
        # The grammar admits S3-1 (no part, a split); pinned as valid, its row S3 (section 6 grammar (3)).
        self.assertEqual(sprintid.parse("S3-1"), sprintid.ID(3, 0, "", 1))
        self.assertEqual(sprintid.row_of(sprintid.parse("S3-1")), "S3")


class TheOrder(unittest.TestCase):
    def test_load_order_reads_todays_table(self):
        rows = order().rows
        self.assertEqual(len(rows), 26)
        self.assertEqual(rows[0], sprintid.Row("S1.1a-1", True))
        self.assertEqual(rows[12], sprintid.Row("S1.1c", False))
        self.assertEqual(rows[-1], sprintid.Row("S12", False))

    def test_load_order_refuses_a_bad_table(self):
        bad = {
            "a duplicate": '{"schema": 1, "rows": [{"id": "S4.1", "closed": false}, {"id": "S4.1", "closed": false}]}',
            "an unparseable id": '{"schema": 1, "rows": [{"id": "s4.1", "closed": false}]}',
            "no rows": '{"schema": 1, "rows": []}',
        }
        for name, text in bad.items():
            with self.subTest(name=name), self.assertRaises(ValueError):
                sprintid.load_order(text)

    def test_index_by_printed_id_first_then_by_row(self):
        o = order()
        self.assertEqual(o.index("S1.1a-1"), 0)
        self.assertEqual(o.index("S1.1a-2"), 1)
        with self.assertRaises(ValueError):
            o.index("S1.1a")  # the struck row is not in sprints.json
        self.assertEqual(o.index("S3.3"), 11)
        self.assertEqual(o.index("S3.3-2"), 11)
        self.assertEqual(o.index("S3.3-7"), 11)  # parts are not validated by the table (pinned, said so)
        with self.assertRaises(ValueError):
            o.index("S99")

    def test_reached_is_by_the_table_not_by_numbers(self):
        o = order()
        self.assertTrue(o.reached("S3.3-2", "S1.1c-1"))  # row S3.3 precedes row S1.1c in the table
        self.assertFalse(o.reached("S10", "S9.2"))  # a string compare says the opposite
        self.assertTrue(o.reached("S9.2", "S10"))
        self.assertTrue(o.reached("S4.1", "S4.2"))
        self.assertFalse(o.reached("S4.2", "S4.1"))
        self.assertTrue(o.reached("S4.1", "S4.1"))
        self.assertTrue(o.reached("S1.1c-2", "S1.1c-1"))  # by row: an entry must name a row, never a part

    def test_first_open(self):
        self.assertEqual(order().first_open(), "S1.1c")
        closed = '{"schema": 1, "rows": [{"id": "S4.1", "closed": true}, {"id": "S4.2", "closed": true}]}'
        with self.assertRaises(ValueError):
            sprintid.load_order(closed).first_open()


class ThePlanTable(unittest.TestCase):
    def test_plan_rows_equal_todays_sprints_json(self):
        self.assertEqual(sprintid.plan_rows(read(PLAN)), list(order().rows))

    def test_a_changed_header_is_an_error_naming_the_line(self):
        text = read(PLAN)
        self.assertEqual(text.count(HEADER), 1, "the header anchor moved")
        with self.assertRaises(ValueError) as caught:
            sprintid.plan_rows(text.replace(HEADER, HEADER.replace("Title", "Name")))
        self.assertIn("Name", str(caught.exception))

    def test_a_fourth_mark_is_an_error_naming_the_line(self):
        text = read(PLAN)
        anchor = "| S4.1 | The pattern engine"
        self.assertEqual(text.count(anchor), 1, "the row anchor moved")
        with self.assertRaises(ValueError) as caught:
            sprintid.plan_rows(text.replace(anchor, f"| S4.1 {CROSS} | The pattern engine"))
        self.assertIn("S4.1", str(caught.exception))

    def test_a_tick_outside_the_id_cell_is_an_error_naming_the_line(self):
        # Owner 2026-11-11 (the design, 6): a tick anywhere but a plan row's id cell is an error.
        text = read(PLAN)
        anchor = "| S4.1 | The pattern engine"
        self.assertEqual(text.count(anchor), 1, "the row anchor moved")
        line = next(n for n, row in enumerate(text.splitlines(), 1) if row.startswith(anchor))
        with self.assertRaises(ValueError) as caught:
            sprintid.plan_rows(text.replace(anchor, f"| S4.1 | The pattern engine {TICK}"))
        self.assertEqual(str(caught.exception), f"sprint-plan.md line {line}: {TICK} outside the id cell")

    def test_a_missing_header_is_an_error(self):
        with self.assertRaises(ValueError) as caught:
            sprintid.plan_rows("# A plan\n\n| Other | Table |\n|---|---|\n| S4.1 | x |\n")
        self.assertEqual(str(caught.exception), "sprint-plan.md: the §3 table header was not found")

    def test_the_three_tick_forms_are_recognised(self):
        text = read(PLAN)
        for form in (f"S1.1a-1 [{TICK}](s1-1a-1-record.md)", f"S3.1 {TICK} ([S3.1-1]", f"S3.2c {TICK} ([record]"):
            self.assertIn(form, text, "the plan's tick forms changed")
        closed = [row.id for row in sprintid.plan_rows(text) if row.closed]
        self.assertEqual(closed[:3], ["S1.1a-1", "S1.1a-2", "S1.1b"])
        self.assertIn("S3.1", closed)
        self.assertIn("S3.2c", closed)
        self.assertNotIn("S1.1a", [row.id for row in sprintid.plan_rows(text)])  # struck: skipped


class TheMarker(unittest.TestCase):
    def test_one_line_one_lf(self):
        self.assertEqual(sprintid.read_marker("S1.1c-1\n"), sprintid.ID(1, 1, "c", 1))

    def test_every_other_form_is_an_error(self):
        for text in ("S1.1c-1", "S1.1c-1\n\n", "\r\n", "", "S1.1c-1\nS4.1\n"):
            with self.subTest(text=text), self.assertRaises(ValueError):
                sprintid.read_marker(text)


def table(*rows: tuple[str, bool]) -> str:
    return json.dumps({"schema": 1, "note": "a fixture", "rows": [{"id": i, "closed": c} for i, c in rows]})


class TheOrdersFaults(unittest.TestCase):
    """load_order's faults with the note present, so each case fails on its own fault and the text names it."""

    def test_each_fault_is_named(self):
        cases = {
            "a duplicate": (table(("S4.1", False), ("S4.1", False)), "gate/sprints.json: rows[1]: duplicate id S4.1"),
            "an unparseable id": (table(("s4.1", False)), "gate/sprints.json: rows[0]: 's4.1': not a sprint id"),
            "no rows": (table(), "gate/sprints.json: rows is not a non-empty list"),
            "no note": ('{"schema": 1, "rows": [{"id": "S4.1", "closed": false}]}', "gate/sprints.json: keys"),
            "an extra row key": (
                '{"schema": 1, "note": "", "rows": [{"id": "S4.1", "closed": false, "x": 1}]}',
                "gate/sprints.json: rows[0]: keys",
            ),
            "a schema 2": ('{"schema": 2, "note": "", "rows": [{"id": "S4.1", "closed": false}]}', "schema 2, not 1"),
            "not json": ("{", "gate/sprints.json: "),
        }
        for name, (text, needle) in cases.items():
            with self.subTest(name=name), self.assertRaises(ValueError) as caught:
                sprintid.load_order(text)
            self.assertIn(needle, str(caught.exception))

    def test_every_row_closed_is_first_opens_error(self):
        with self.assertRaises(ValueError) as caught:
            sprintid.load_order(table(("S4.1", True), ("S4.2", True))).first_open()
        self.assertEqual(str(caught.exception), "every row of gate/sprints.json is closed")


class ThePinHelpers(unittest.TestCase):
    """compare_rows and first_open_mismatch on fixtures (design note section 3.5); the pins run them on the files."""

    def test_equal_tables_compare_empty(self):
        plan = [sprintid.Row("S3.3", True), sprintid.Row("S1.1c", False), sprintid.Row("S4.1", False)]
        self.assertEqual(sprintid.compare_rows(plan, sprintid.load_order(table(*((r.id, r.closed) for r in plan)))), "")

    def test_a_flipped_mark_names_the_row_both_ways(self):
        plan = [sprintid.Row("S3.3", True), sprintid.Row("S1.1c", False), sprintid.Row("S4.1", False)]
        order = sprintid.load_order(table(("S3.3", True), ("S1.1c", True), ("S4.1", False)))
        self.assertEqual(
            sprintid.compare_rows(plan, order),
            "gate/sprints.json row 2 is S1.1c (closed); sprint-plan.md §3 row 2 is S1.1c (open)",
        )

    def test_a_different_id_names_the_first_difference_only(self):
        plan = [sprintid.Row("S3.3", True), sprintid.Row("S1.1c", False), sprintid.Row("S4.1", False)]
        order = sprintid.load_order(table(("S3.3", True), ("S4.1", False), ("S4.2", False)))
        self.assertEqual(
            sprintid.compare_rows(plan, order),
            "gate/sprints.json row 2 is S4.1 (open); sprint-plan.md §3 row 2 is S1.1c (open)",
        )

    def test_a_row_missing_on_either_side_is_absent(self):
        plan = [sprintid.Row("S3.3", True), sprintid.Row("S1.1c", False)]
        longer = sprintid.load_order(table(("S3.3", True), ("S1.1c", False), ("S4.1", False)))
        shorter = sprintid.load_order(table(("S3.3", True)))
        self.assertEqual(
            sprintid.compare_rows(plan, longer),
            "gate/sprints.json row 3 is S4.1 (open); sprint-plan.md §3 row 3 is absent",
        )
        self.assertEqual(
            sprintid.compare_rows(plan, shorter),
            "gate/sprints.json row 2 is absent; sprint-plan.md §3 row 2 is S1.1c (open)",
        )

    def test_the_marker_at_the_first_open_rows_part_matches(self):
        order = sprintid.load_order(table(("S3.3", True), ("S1.1c", False), ("S4.1", False)))
        self.assertEqual(sprintid.first_open_mismatch(sprintid.parse("S1.1c-1"), order), "")
        self.assertEqual(sprintid.first_open_mismatch(sprintid.parse("S1.1c"), order), "")

    def test_a_marker_past_or_before_the_first_open_row_is_named(self):
        order = sprintid.load_order(table(("S3.3", True), ("S1.1c", False), ("S4.1", False)))
        self.assertEqual(
            sprintid.first_open_mismatch(sprintid.parse("S4.1"), order),
            "gate/current-sprint S4.1 is not the first open row S1.1c",
        )
        self.assertEqual(
            sprintid.first_open_mismatch(sprintid.parse("S3.3-2"), order),
            "gate/current-sprint S3.3-2 is not the first open row S1.1c",
        )
