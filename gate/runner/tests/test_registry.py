"""Registry load and strict validation (design note §4; design-QA counter-examples 5, 24)."""

import json
import os
import tempfile
import unittest

from gate.runner import registry


def row(**over):
    base = {
        "id": "a",
        "job": "lint",
        "tiers": ["T2"],
        "blocking": True,
        "timeout_s": 60,
        "needs": [],
        "scope": ["tools"],
        "expected_scope": ["git", ":(glob)**/*.go"],
        "min_wants": 0,
        "lands_in": "S1.1a-1",
        "exclusive": False,
        "command": ["true"],
    }
    base.update(over)
    return base


def doc(*rows):
    return json.dumps({"schema": 1, "rows": list(rows)})


class Loads(unittest.TestCase):
    def test_a_good_row_parses_to_a_frozen_dataclass(self):
        rows = registry.loads(doc(row(), row(id="b", needs=["a"], tiers=["T2"])))
        self.assertEqual([r.id for r in rows], ["a", "b"])
        self.assertEqual(rows[1].needs, ("a",))
        self.assertEqual(rows[0].expected_scope, ("git", ":(glob)**/*.go"))
        self.assertEqual(rows[0].timeout_s, 60)
        with self.assertRaises(AttributeError):
            rows[0].id = "z"

    def test_timeout_boundaries_are_accepted(self):
        for cap in (1, 3600):
            with self.subTest(cap=cap):
                self.assertEqual(registry.loads(doc(row(timeout_s=cap)))[0].timeout_s, cap)

    def test_every_expected_scope_kind_is_accepted(self):
        for scope in (["git", "x"], ["go", "./..."], ["go-vet-units", "./..."]):
            with self.subTest(scope=scope):
                self.assertEqual(registry.loads(doc(row(expected_scope=scope)))[0].expected_scope, tuple(scope))

    def test_file_load_reads_utf8_and_names_the_path_on_error(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "r.json")
            with open(path, "w", encoding="utf-8") as f:
                f.write(doc(row()))
            self.assertEqual(len(registry.load(path)), 1)
            with self.assertRaises(registry.RegistryError) as caught:
                registry.load(os.path.join(d, "absent.json"))
            self.assertIn("absent.json", str(caught.exception))

    def test_a_file_that_is_not_utf8_is_refused(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "r.json")
            with open(path, "wb") as f:
                f.write(b'{"schema": 1, "rows": ["\xff"]}')
            with self.assertRaises(registry.RegistryError):
                registry.load(path)


class Refuses(unittest.TestCase):
    """One row per rule; every message names the rule's subject."""

    CASES = (
        ("timeout_s is a bool", row(timeout_s=True), "timeout_s"),
        ("timeout_s below the range", row(timeout_s=0), "timeout_s"),
        ("timeout_s above the range", row(timeout_s=3601), "timeout_s"),
        ("timeout_s is a float", row(timeout_s=1.0), "timeout_s"),
        ("min_wants is a bool", row(min_wants=False), "min_wants"),
        ("min_wants is negative", row(min_wants=-1), "min_wants"),
        ("blocking is an int", row(blocking=1), "blocking"),
        ("exclusive is a string", row(exclusive="no"), "exclusive"),
        ("id is empty", row(id=""), "id"),
        ("id has a capital", row(id="Ab"), "id"),
        ("id starts with a digit", row(id="1a"), "id"),
        ("id ends in a newline", row(id="a\n"), "id"),
        ("id is the built-in parity", row(id="parity"), "parity"),
        ("job has an underscore", row(job="a_b"), "job"),
        ("job is the star label", row(job="*"), "job"),
        ("tiers is empty", row(tiers=[]), "tiers"),
        ("tiers has an unknown tier", row(tiers=["T5"]), "T5"),
        ("tiers has a duplicate", row(tiers=["T2", "T2"]), "tiers"),
        ("tiers is a string", row(tiers="T2"), "tiers"),
        ("scope has an unknown class", row(scope=["vendor"]), "vendor"),
        ("scope has a duplicate", row(scope=["tools", "tools"]), "scope"),
        ("expected_scope has an unknown kind", row(expected_scope=["svn", "x"]), "svn"),
        ("expected_scope has no argument", row(expected_scope=["git"]), "expected_scope"),
        ("expected_scope is empty", row(expected_scope=[]), "expected_scope"),
        ("expected_scope holds a non-string", row(expected_scope=["git", 1]), "expected_scope"),
        ("command is empty", row(command=[]), "command"),
        ("command holds a non-string", row(command=["go", 1]), "command"),
        ("command is a string", row(command="go vet"), "command"),
        ("command embeds {files}", row(command=["x", "--files={files}"]), "{files}"),
        ("lands_in is empty", row(lands_in=""), "lands_in"),
        ("an unknown field", dict(row(), timout_s=1), "timout_s"),
    )

    def test_each_bad_row_exits_at_load(self):
        for name, bad, needle in self.CASES:
            with self.subTest(name):
                with self.assertRaises(registry.RegistryError) as caught:
                    registry.loads(doc(bad))
                self.assertIn(needle, str(caught.exception))

    def test_every_field_is_required(self):
        for field in row():
            bad = row()
            del bad[field]
            with self.subTest(field):
                with self.assertRaises(registry.RegistryError) as caught:
                    registry.loads(doc(bad))
                self.assertIn(field, str(caught.exception))

    def test_a_duplicate_key_in_a_row(self):
        text = doc(row()).replace('"id": "a"', '"id": "a", "id": "b"')
        with self.assertRaises(registry.RegistryError) as caught:
            registry.loads(text)
        self.assertIn("duplicate key", str(caught.exception))
        self.assertIn("id", str(caught.exception))

    def test_a_duplicate_key_at_the_top_level(self):
        with self.assertRaises(registry.RegistryError):
            registry.loads('{"schema": 1, "schema": 1, "rows": []}')

    def test_nan_is_not_json(self):
        with self.assertRaises(registry.RegistryError):
            registry.loads(doc(row()).replace('"timeout_s": 60', '"timeout_s": NaN'))

    def test_broken_json(self):
        with self.assertRaises(registry.RegistryError):
            registry.loads("{")

    def test_the_top_level_shape(self):
        for text in ("[]", '{"rows": []}', '{"schema": 2, "rows": []}', '{"schema": true, "rows": []}',
                     '{"schema": 1}', '{"schema": 1, "rows": {}}', '{"schema": 1, "rows": [], "x": 1}',
                     '{"schema": 1, "rows": [1]}'):  # fmt: skip
            with self.subTest(text), self.assertRaises(registry.RegistryError):
                registry.loads(text)

    def test_duplicate_ids(self):
        with self.assertRaises(registry.RegistryError) as caught:
            registry.loads(doc(row(), row()))
        self.assertIn("duplicate id", str(caught.exception))

    def test_needs_an_unknown_id(self):
        with self.assertRaises(registry.RegistryError) as caught:
            registry.loads(doc(row(needs=["ghost"])))
        self.assertIn("ghost", str(caught.exception))

    def test_needs_a_cycle_is_refused_by_the_topological_rule(self):
        with self.assertRaises(registry.RegistryError) as caught:
            registry.loads(doc(row(id="x", needs=["y"]), row(id="y", needs=["x"])))
        self.assertIn("'y'", str(caught.exception))

    def test_needs_itself(self):
        with self.assertRaises(registry.RegistryError):
            registry.loads(doc(row(needs=["a"])))

    def test_needs_a_later_row(self):
        with self.assertRaises(registry.RegistryError):
            registry.loads(doc(row(id="x", needs=["y"]), row(id="y")))

    def test_needs_another_job(self):
        with self.assertRaises(registry.RegistryError) as caught:
            registry.loads(doc(row(id="x", job="lint"), row(id="y", job="test", needs=["x"])))
        self.assertIn("same job", str(caught.exception))

    def test_a_needed_row_must_cover_this_rows_tiers(self):
        rows = doc(row(id="x", tiers=["T3", "T4"]), row(id="y", tiers=["T2", "T3"], needs=["x"]))
        with self.assertRaises(registry.RegistryError) as caught:
            registry.loads(rows)
        self.assertIn("T2", str(caught.exception))

    def test_a_duplicate_need(self):
        with self.assertRaises(registry.RegistryError):
            registry.loads(doc(row(id="x"), row(id="y", needs=["x", "x"])))


class Select(unittest.TestCase):
    def setUp(self):
        self.rows = registry.loads(
            doc(
                row(id="a", job="lint", tiers=["T1", "T2"]),
                row(id="b", job="test", tiers=["T2"]),
                row(id="c", job="lint", tiers=["T4"]),
            )
        )

    def test_by_tier_keeps_registry_order(self):
        self.assertEqual([r.id for r in registry.select(self.rows, "T2", None)], ["a", "b"])

    def test_by_tier_and_job(self):
        self.assertEqual([r.id for r in registry.select(self.rows, "T2", "lint")], ["a"])

    def test_an_unknown_job_is_refused(self):
        with self.assertRaises(registry.RegistryError) as caught:
            registry.select(self.rows, "T2", "lnt")
        self.assertIn("lnt", str(caught.exception))

    def test_zero_rows_is_refused(self):
        with self.assertRaises(registry.RegistryError) as caught:
            registry.select(self.rows, "T3", None)
        self.assertIn("zero", str(caught.exception))


if __name__ == "__main__":
    unittest.main()
