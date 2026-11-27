"""The naming tool against its golden table and on a real git listing (plan check §3, §6 #20-31; S1.1c-7 design W0,
Part B). Every expected verdict, name list and output line here was written by hand from the plan, before the matcher.
"""

import json
import os
import re
import subprocess
import sys
import tempfile
import unittest

from gate.runner import testnames
from gate.runner.tests import support

GOLDEN = os.path.join(support.GATE, "runner", "testdata", "testnames", "golden.json")
WORDS = os.path.join(support.GATE, "testnames.json")
TOOL = os.path.join(support.GATE, "runner", "testnames.py")
RULE_IDS = (
    "sprint id",
    "review step",
    "lane or wave",
    "review id with a digit",
    "review-role or sprint-shorthand prefix",
)
PREFIX = "review-role or sprint-shorthand prefix"
TOOL_CEILING_S = 120
GOOD_WORDS = {
    "schema": 1,
    "note": "a test word list",
    "rules": [{"id": "lane", "pattern": "lane_[a-z]"}, {"id": "wave", "pattern": "wave_?\\d+"}],
}


def golden() -> dict:
    with open(GOLDEN, encoding="utf-8") as f:
        return json.load(f)


def run_tool(cwd: str, *args: str) -> tuple[int, str, str]:
    """The tool as a process of its own, as S1.1c-13's row will start it; (exit code, stdout, stderr)."""
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        code = subprocess.run(
            [sys.executable, "-I", "-B", TOOL, *args],
            cwd=cwd,
            env=support.base_env(),
            stdin=subprocess.DEVNULL,
            stdout=out,
            stderr=err,
            timeout=TOOL_CEILING_S,
            start_new_session=True,
            check=False,
        ).returncode
        out.seek(0)
        err.seek(0)
        return code, out.read().decode("utf-8", errors="replace"), err.read().decode("utf-8", errors="replace")


class Golden(support.GateTest):
    def test_every_golden_name_gets_the_verdict_its_row_names(self) -> None:
        rules = testnames.load_rules(WORDS)
        rows = golden()["names"]
        self.assertTrue(any(row["rules"] for row in rows) and any(not row["rules"] for row in rows))
        for row in rows:
            with self.subTest(row=row["row"], name=row["name"]):
                self.assertEqual(testnames.match(rules, row["name"]), tuple(row["rules"]))

    def test_every_golden_source_yields_the_names_and_hits_its_row_names(self) -> None:
        rules = testnames.load_rules(WORDS)
        rows = golden()["sources"]
        self.assertTrue(any(row["path"].endswith(".go") for row in rows))
        self.assertTrue(any(row["path"].endswith(".py") for row in rows))
        for row in rows:
            with self.subTest(row=row["row"], path=row["path"]):
                got = testnames.names(row["path"], row["text"])
                self.assertEqual([[n.kind, n.name] for n in got], row["names"])
                self.assertEqual({n.path for n in got}, {row["path"]})
                hits = testnames.judge(rules, got)
                self.assertEqual([[h.name.kind, h.name.name, list(h.rules)] for h in hits], row["hits"])

    def test_every_golden_path_is_in_or_out_of_scope_as_its_row_names(self) -> None:
        rows = golden()["scope"]
        self.assertEqual({row["in_scope"] for row in rows}, {True, False})
        for row in rows:
            with self.subTest(row=row["row"], path=row["path"]):
                self.assertIs(testnames.in_scope(row["path"]), row["in_scope"])

    def test_the_golden_table_holds_every_counter_example_row(self) -> None:
        table = golden()
        self.assertEqual(set(table), {"schema", "note", "names", "sources", "scope"})
        self.assertEqual(table["schema"], 1)
        present = {row["row"] for part in ("names", "sources", "scope") for row in table[part]}
        missing = sorted({str(n) for n in range(20, 32)} - present, key=int)
        self.assertEqual(missing, [])
        # Every rule of the word list has a red row (plan check §3: the golden table is the spec).
        red = {rule for row in table["names"] for rule in row["rules"]}
        self.assertEqual(sorted(red), sorted(RULE_IDS))

    def test_tokens_split_on_separators_and_case_changes(self) -> None:
        cases = {
            "LaneA": "lane_a",
            "HTTPServer": "http_server",
            "s3_2b": "s3_2b",
            "x9Y": "x9_y",
            "a-b.c": "a_b_c",
            "wave2": "wave2",
        }
        for name, want in cases.items():
            with self.subTest(name=name):
                self.assertEqual(testnames.tokens(name), want)

    def test_the_word_list_loads_and_each_rule_compiles(self) -> None:
        rules = testnames.load_rules(WORDS)
        self.assertEqual(tuple(rule.id for rule in rules), RULE_IDS)
        for rule in rules:
            self.assertIsInstance(rule.rx, re.Pattern)

    def test_a_word_list_with_an_unknown_key_or_a_bad_pattern_is_refused(self) -> None:
        def words(name: str, data: object) -> str:
            path = os.path.join(self.root, name)
            with open(path, "w", encoding="utf-8") as f:
                f.write(data if isinstance(data, str) else json.dumps(data))
            return path

        good = testnames.load_rules(words("good.json", GOOD_WORDS))  # the positive witness: the base list loads
        self.assertEqual(tuple(rule.id for rule in good), ("lane", "wave"))
        lane, wave = GOOD_WORDS["rules"]
        bad = {
            "top-key.json": {**GOOD_WORDS, "extra": 1},
            "no-note.json": {"schema": 1, "rules": GOOD_WORDS["rules"]},
            "schema-2.json": {**GOOD_WORDS, "schema": 2},
            "schema-true.json": {**GOOD_WORDS, "schema": True},
            "rule-key.json": {**GOOD_WORDS, "rules": [{**lane, "flags": "i"}, wave]},
            "rule-no-pattern.json": {**GOOD_WORDS, "rules": [{"id": "lane"}, wave]},
            "same-id.json": {**GOOD_WORDS, "rules": [lane, {**wave, "id": "lane"}]},
            "bad-pattern.json": {**GOOD_WORDS, "rules": [lane, {**wave, "pattern": "wave_(\\d+"}]},
            "late-flag.json": {**GOOD_WORDS, "rules": [lane, {**wave, "pattern": "(?i)wave"}]},
            "no-rules.json": {**GOOD_WORDS, "rules": []},
            "not-a-list.json": {**GOOD_WORDS, "rules": {"lane": "lane_[a-z]"}},
            "twice.json": '{"schema": 1, "schema": 1, "note": "n", "rules": [{"id": "a", "pattern": "a"}]}',
            "not-json.json": "{",
        }
        for name, data in bad.items():
            with self.subTest(name=name):
                path = words(name, data)
                with self.assertRaises(ValueError) as caught:
                    testnames.load_rules(path)
                self.assertIn(path, str(caught.exception))

    def test_a_python_file_that_does_not_parse_is_a_fault_naming_it(self) -> None:
        path = "gate/x/tests/test_broken.py"
        good = testnames.py_names(path, "def test_holds():\n    pass\n")  # the positive witness: the path parses
        self.assertEqual([(n.kind, n.name) for n in good], [("file", "test_broken"), ("func", "test_holds")])
        with self.assertRaises(testnames.Fault) as caught:
            testnames.py_names(path, "def test_holds(:\n    pass\n")
        self.assertIn(path, str(caught.exception))
        # main reports the fault as its last line and exits 2, after a good file exits 0 the same way.
        self.write("pkg/test_good.py", "def test_holds():\n    pass\n")
        self.write("pkg/test_broken.py", "def test_holds(:\n    pass\n")
        code, out, err = run_tool(self.repo, "pkg/test_good.py")
        self.assertEqual((code, err), (0, ""), out)
        self.assertEqual(out, "testnames: 1 files, 2 names; 0 hits in 0 files (report only)\n")
        code, out, err = run_tool(self.repo, "pkg/test_good.py", "pkg/test_broken.py")
        self.assertEqual((code, err), (2, ""), out)
        self.assertRegex(out.splitlines()[-1], r"^error: pkg/test_broken\.py: ")

    def test_main_lists_only_tracked_test_files_outside_testdata(self) -> None:
        go = "package a\n\nfunc bkHelper() {}\n"
        py = "def bk_helper():\n    pass\n"
        self.track("a_test.go", go)
        self.track("testdata/b_test.go", go)
        self.track("c.go", go)
        self.track("pkg/test_d.py", py)
        self.track("pkg/helper.py", py)
        self.write("e_test.go", go)  # on disk, never added: git does not list it
        out_json = os.path.join(self.root, "hits.json")
        code, out, err = run_tool(self.repo, "--json", out_json)
        self.assertEqual((code, err), (0, ""), out)
        self.assertEqual(
            out.splitlines(),
            [
                f"a_test.go:3: func bkHelper: {PREFIX}",
                f"pkg/test_d.py:1: func bk_helper: {PREFIX}",
                "testnames: 2 files, 4 names; 2 hits in 2 files (report only)",
            ],
        )
        want = [
            {"kind": "func", "line": 3, "name": "bkHelper", "path": "a_test.go", "rules": [PREFIX]},
            {"kind": "func", "line": 1, "name": "bk_helper", "path": "pkg/test_d.py", "rules": [PREFIX]},
        ]
        with open(out_json, "rb") as f:
            data = f.read()
        pinned = json.dumps(want, sort_keys=True, ensure_ascii=False, separators=(",", ":")) + "\n"
        self.assertEqual(data, pinned.encode())
        self.assertEqual(sorted(os.listdir(self.root)), ["hits.json", "repo", "run"])  # no temporary file left


if __name__ == "__main__":
    unittest.main()
