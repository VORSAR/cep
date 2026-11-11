"""The sprint-id grammar, the printed order and the marker (S1.1c-1 lane D, plan check v2 section 2 S2 and
section 6 "The grammar, the order, the marker"; sprint-plan.md section 3's grammar S<stage>[.<part>][<letter>][-<n>]).

Pure: strings in, values out. The order tests read gate/sprints.json and docs/sprints/sprint-plan.md as they are.
"""

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
