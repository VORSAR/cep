"""Records and atomic files (design note §4; plan check S2, case 11, C6)."""

import json
import os
import tempfile
import unittest

from gate.runner import records


class JsonBytes(unittest.TestCase):
    def test_the_pinned_form(self):
        got = records.json_bytes({"b": 1, "a": "é", "c": [1, 2]})
        self.assertEqual(got, '{"a":"é","b":1,"c":[1,2]}\n'.encode())


class WriteAtomic(unittest.TestCase):
    def test_writes_the_bytes_and_leaves_no_temporary(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "x.json")
            records.write_atomic(path, b"data")
            with open(path, "rb") as f:
                self.assertEqual(f.read(), b"data")
            self.assertEqual(sorted(os.listdir(d)), ["x.json"])

    def test_replaces_an_existing_file_whole(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "x.json")
            records.write_atomic(path, b"old")
            records.write_atomic(path, b"new")
            with open(path, "rb") as f:
                self.assertEqual(f.read(), b"new")

    def test_a_failed_write_keeps_the_old_file_and_no_temporary(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "x.json")
            records.write_atomic(path, b"old")

            def boom(tmp, final):
                raise OSError("rename failed")

            with self.assertRaises(OSError):
                records.write_atomic(path, b"new", rename=boom)
            with open(path, "rb") as f:
                self.assertEqual(f.read(), b"old")
            self.assertEqual(sorted(os.listdir(d)), ["x.json"])


class OneLine(unittest.TestCase):
    def test_every_control_character_becomes_a_space(self):
        self.assertEqual(records.one_line("a\nb\rc\td\x0be\x85f g"), "a b c d e f g")

    def test_long_text_is_truncated_and_marked(self):
        got = records.one_line("x" * 5000)
        self.assertEqual(len(got), records.REASON_MAX)
        self.assertTrue(got.endswith("…"))

    def test_no_limit_keeps_long_text_whole(self):
        self.assertEqual(records.one_line("x\n" * 400, limit=None), "x " * 400)

    def test_short_text_is_unchanged(self):
        self.assertEqual(records.one_line("exit 1"), "exit 1")


class Record(unittest.TestCase):
    def test_has_exactly_the_contract_fields(self):
        rec = records.make(
            run_id="r", commit="c", tier="T2", job="lint", row_id="gofmt", blocking=True, verdict="pass",
            reason="covered 1", exit_code=0, covered=1, expected=1, started_ms=5, ended_ms=9, duration_ms=4,
            scope_sha256="ab",
        )  # fmt: skip
        self.assertEqual(
            rec,
            {
                "schema": 1,
                "run_id": "r",
                "commit": "c",
                "tier": "T2",
                "job": "lint",
                "id": "gofmt",
                "blocking": True,
                "verdict": "pass",
                "reason": "covered 1",
                "exit_code": 0,
                "covered": 1,
                "expected": 1,
                "started_ms": 5,
                "ended_ms": 9,
                "duration_ms": 4,
                "scope_sha256": "ab",
            },
        )
        json.loads(records.json_bytes(rec))

    def test_a_multi_line_reason_stays_one_line(self):
        rec = records.make(
            run_id="r", commit="c", tier="T2", job="j", row_id="i", blocking=True, verdict="fail", reason="a\nb",
            exit_code=1, covered=0, expected=0, started_ms=0, ended_ms=0, duration_ms=0, scope_sha256="",
        )  # fmt: skip
        self.assertEqual(rec["reason"], "a b")

    def test_a_record_may_keep_a_long_reason_whole(self):
        # review BA5: parity's record lists every failing tool, however long the list is.
        rec = records.make(
            run_id="r", commit="c", tier="T2", job="j", row_id="parity", blocking=True, verdict="fail",
            reason="z" * 900, exit_code=1, covered=0, expected=0, started_ms=0, ended_ms=0, duration_ms=0,
            scope_sha256="", reason_max=None,
        )  # fmt: skip
        self.assertEqual(rec["reason"], "z" * 900)


if __name__ == "__main__":
    unittest.main()
