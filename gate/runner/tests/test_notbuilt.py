"""The not-built rule (S1.1c-1 lane D; design note §3.3; plan check v2 §6 "The expiry"): a corpus file listed for a
sprint row the marker has reached is expired, named with its construct and the marker; an entry must name a row.

Pure, on fixtures: the order is a fixture table in the plan's printed order, the entries are today's nine as
listed in conformance/notbuilt.json, copied by hand in file order.
"""

import json
import unittest

from gate.runner import notbuilt, sprintid

ROWS = ("S3.3", "S1.1c", "S4.1", "S4.2", "S7", "S8", "S9.1", "S9.2", "S10")
ORDER = sprintid.load_order(
    json.dumps({"schema": 1, "note": "a fixture", "rows": [{"id": i, "closed": i == "S3.3"} for i in ROWS]})
)
ENTRIES = [
    ("01-agent-kill-switch.cep", "a pattern body", "S4.1"),
    ("08-session-flow.cep", "a pattern body", "S4.1"),
    ("05-text-operators.cep", "text operators (S7)", "S7"),
    ("06-once-and-group.cep", "a pattern body", "S7"),
    ("03-sort-leaderboard.cep", "a window body", "S8"),
    ("02-cron-digest.cep", "a window body", "S9.1"),
    ("04-delay-and-batch.cep", "a window body", "S9.2"),
    ("torture/comments.cep", "a pattern body", "S4.1"),
    ("torture/lexical.cep", "a pattern body", "S9.2"),
]


def listing(*entries: tuple[str, str, str], note: str | None = "a fixture") -> str:
    data = {"schema": 1, "entries": [{"file": f, "construct": c, "sprint": s} for f, c, s in entries]}
    if note is not None:
        data["note"] = note
    return json.dumps(data)


def line(file: str, construct: str, sprint: str, current: str) -> str:
    return f"{file}: not built ({construct}), listed for {sprint}, which is reached by {current}"


class LoadEntries(unittest.TestCase):
    def test_todays_nine_load_in_file_order(self):
        self.assertEqual(
            notbuilt.load_entries(listing(*ENTRIES)),
            tuple(notbuilt.Entry(f, c, s) for f, c, s in ENTRIES),
        )

    def test_an_empty_list_loads(self):
        self.assertEqual(notbuilt.load_entries(listing()), ())

    def test_a_part_is_refused_by_name(self):
        with self.assertRaises(ValueError) as caught:
            notbuilt.load_entries(listing(("01-agent-kill-switch.cep", "a pattern body", "S1.1c-2")))
        self.assertEqual(str(caught.exception), "conformance/notbuilt.json: entries name a row, not a part: S1.1c-2")

    def test_each_other_fault_is_named(self):
        cases = {
            "no note": (listing(("a.cep", "x", "S4.1"), note=None), "conformance/notbuilt.json: keys"),
            "a duplicate file": (
                listing(("a.cep", "x", "S4.1"), ("a.cep", "y", "S7")),
                "conformance/notbuilt.json: entries[1]: duplicate file a.cep",
            ),
            "an empty construct": (listing(("a.cep", "", "S4.1")), "conformance/notbuilt.json: entries[0]: construct"),
            "an unparseable sprint": (
                listing(("a.cep", "x", "s4.1")),
                "conformance/notbuilt.json: entries[0]: 's4.1': not a sprint id",
            ),
            "a missing key": (
                '{"schema": 1, "note": "", "entries": [{"file": "a.cep", "sprint": "S4.1"}]}',
                "conformance/notbuilt.json: entries[0]: keys",
            ),
            "an extra key": (
                '{"schema": 1, "note": "", "entries": [{"file": "a.cep", "construct": "x", "sprint": "S4.1", "k": 1}]}',
                "conformance/notbuilt.json: entries[0]: keys",
            ),
            "a schema 2": ('{"schema": 2, "note": "", "entries": []}', "conformance/notbuilt.json: schema 2, not 1"),
            "not a list": ('{"schema": 1, "note": "", "entries": {}}', "conformance/notbuilt.json: entries"),
        }
        for name, (text, needle) in cases.items():
            with self.subTest(name=name), self.assertRaises(ValueError) as caught:
                notbuilt.load_entries(text)
            self.assertIn(needle, str(caught.exception))


class Expired(unittest.TestCase):
    def expired(self, current: str, entries=ENTRIES) -> list[str]:
        loaded = notbuilt.load_entries(listing(*entries))
        return notbuilt.expired(loaded, ORDER, sprintid.parse(current))

    def test_the_marker_at_s4_1_expires_its_three_files_in_file_order(self):
        self.assertEqual(
            self.expired("S4.1"),
            [
                line("01-agent-kill-switch.cep", "a pattern body", "S4.1", "S4.1"),
                line("08-session-flow.cep", "a pattern body", "S4.1", "S4.1"),
                line("torture/comments.cep", "a pattern body", "S4.1", "S4.1"),
            ],
        )

    def test_the_marker_at_todays_part_expires_none(self):
        self.assertEqual(self.expired("S1.1c-1"), [])

    def test_the_marker_at_s4_2_expires_the_same_three(self):
        self.assertEqual(
            self.expired("S4.2"),
            [
                line("01-agent-kill-switch.cep", "a pattern body", "S4.1", "S4.2"),
                line("08-session-flow.cep", "a pattern body", "S4.1", "S4.2"),
                line("torture/comments.cep", "a pattern body", "S4.1", "S4.2"),
            ],
        )

    def test_the_marker_at_s9_2_expires_all_nine(self):
        self.assertEqual(self.expired("S9.2"), [line(f, c, s, "S9.2") for f, c, s in ENTRIES])

    def test_an_entry_for_the_markers_own_row_is_expired(self):
        # By design: the red the expiry exists for (review: architect R5, QA L5).
        self.assertEqual(
            self.expired("S1.1c-1", [("x.cep", "a window body", "S1.1c")]),
            [line("x.cep", "a window body", "S1.1c", "S1.1c-1")],
        )

    def test_an_entry_naming_no_row_is_the_error(self):
        with self.assertRaises(ValueError) as caught:
            self.expired("S1.1c-1", [("01-agent-kill-switch.cep", "a pattern body", "S4")])
        self.assertEqual(
            str(caught.exception),
            "conformance/notbuilt.json: 01-agent-kill-switch.cep: S4: not a row of gate/sprints.json",
        )
