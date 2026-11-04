"""The lark side's shape mapping, pinned to tools/grammar-diff.md by hand-written expectations.

Runs under the pinned CPython with the pinned lark from the tool cache.
"""

import os
import unittest

from gate.differential import shapes

HEAD = """language cep/0.1;
stream S (a int, b int, c int, s string, t string, ts timestamp);
stream Out (a int);
table T (k string) primary key (k);
"""


def program(*filters: str, select: str = "a") -> str:
    rules = "".join(
        f"rule R{i} intent 'x' from S[{f}] select {select} insert into Out;\n" for i, f in enumerate(filters)
    )
    return HEAD + rules


class Shapes(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        with open(os.path.join(shapes.REPO, shapes.GRAMMAR), encoding="utf-8") as f:
            cls.grammar = shapes.Grammar(shapes.load_lark(), f.read())

    def shapes_of(self, *filters: str) -> list[str]:
        """The filters' shapes; each rule's `select a` contributes a trailing `name`, dropped here."""
        verdict, found = self.grammar.judge(program(*filters))
        self.assertEqual(verdict, "accept")
        self.assertEqual(found[1::2], ["name"] * len(filters))
        return found[::2]

    def test_precedence_and_associativity(self):
        cases = [
            ("a > 1 or b > 2 and c > 3", "(or (cmp > name literal) (and (cmp > name literal) (cmp > name literal)))"),
            ("(a > 1 or b > 2) and c > 3", "(and (or (cmp > name literal) (cmp > name literal)) (cmp > name literal))"),
            ("a > 1 or b > 2 or c > 3", "(or (or (cmp > name literal) (cmp > name literal)) (cmp > name literal))"),
            ("a - b - c > 0", "(cmp > (arith - (arith - name name) name) literal)"),
            ("a + b * c > 0", "(cmp > (arith + name (arith * name name)) literal)"),
            ("(a + b) * c > 0", "(cmp > (arith * (arith + name name) name) literal)"),
            ("a / b % c * 2 > 0", "(cmp > (arith * (arith % (arith / name name) name) literal) literal)"),
            ("-a * b > 0", "(cmp > (arith * (neg name) name) literal)"),
            ("not s in T", "(not (in name))"),
            ("not s not in T", "(not (not-in name))"),
            ("not a > 1 and b is null", "(and (not (cmp > name literal)) (is-null name))"),
            ("a + 1 is not null", "(is-not-null (arith + name literal))"),
        ]
        for text, want in cases:
            with self.subTest(text=text):
                self.assertEqual(self.shapes_of(text), [want])

    def test_leaves_calls_text_and_raw(self):
        cases = [
            ("s has_ci 'x'", "(text has_ci name (strings 1))"),
            ("s has_ci ${p}", "(text has_ci name param)"),
            ("s has_any_cs('x', 'y', 'z')", "(text has_any_cs name (strings 3))"),
            ("s has_any_cs(${p})", "(text has_any_cs name param)"),
            ("s has_any_cs(T)", "(text has_any_cs name name)"),
            ("s matches_ci '''x'''", "(text matches_ci name (strings 1))"),
            ("raw(s) has_cs 'x'", "(text has_cs (raw name) (strings 1))"),
            ("abs(a - b) > 1", "(cmp > (call abs (arith - name name)) literal)"),
            ("math:round(1.5) == 2.0", "(cmp == (call math:round literal) literal)"),
            ("count() > 1", "(cmp > (call count) literal)"),
            ("coalesce(`a`, 0) > 1", "(cmp > (call coalesce name literal) literal)"),
            ("ts > timestamp '2026-01-01T00:00:00Z' and a == true", "(and (cmp > name literal) (cmp == name literal))"),
            ("ts - 5 min > ts", "(cmp > (arith - name literal) name)"),
        ]
        for text, want in cases:
            with self.subTest(text=text):
                self.assertEqual(self.shapes_of(text), [want])

    def test_document_order_filters_then_select_items_per_rule(self):
        src = (
            HEAD + "rule P intent 'x' from every e = S[a > 1] -> f = S[f.a == e.a] and not S[b < 0] for 1 min "
            "within 5 min after match skip past last late events drop select e.a, 'x' as s insert into Out;\n"
            "rule W intent 'x' from S[c > 2] window sort(3, a, desc) emit on change late events drop "
            "select a + 1 as a insert into Out;\n"
            "rule N intent 'x' from S select a insert into Out;\n"
        )
        verdict, found = self.grammar.judge(src)
        self.assertEqual(verdict, "accept")
        self.assertEqual(
            found,
            [
                "(cmp > name literal)",
                "(cmp == qualified qualified)",
                "(cmp < name literal)",
                "qualified",
                "literal",
                "(cmp > name literal)",
                "(arith + name literal)",
                "name",
            ],
        )

    def test_a_grammar_reject_is_a_verdict_with_no_shapes(self):
        self.assertEqual(self.grammar.judge(program("a = 1")), ("reject", []))
        self.assertEqual(self.grammar.judge("not a program"), ("reject", []))


if __name__ == "__main__":
    unittest.main()
