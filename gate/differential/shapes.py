"""The lark side of the grammar differential (ADR-0002 §5 as amended; tools/grammar-diff.md).

Grammar runs the normative grammar (docs/language/grammar.lark) through lark's LALR parser, as the language-design
checker does, and renders every expression of an accepted program as its canonical operator-spine S-expression,
in document order. The engine side is tools/shape; neither reads the other's output. lark itself is the pinned
python-package of tools.json, imported from the tool cache (load_lark), never from the host.
"""

import os
import sys

from gate.toolchain import manifest

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GRAMMAR = os.path.join("docs", "language", "grammar.lark")

# The tree nodes that are one expression in the grammar; each holds its expression as its first child.
HOLDERS = frozenset({"filter", "sel_item"})
# A partition key: `a of S` (part_key) and `(a, b, ...) of S` (part_group), shaped `(key S a)` and `(group S a b)`
# (S3.2b-2, review F8: the differential compares keys, not only accept/reject).
KEYS = frozenset({"part_key", "part_group"})
# The literal rules (the ?literal alternatives that are not a bare STRING token).
LITERALS = frozenset({"number", "duration", "true_lit", "false_lit", "ts_lit"})
TEXT_OPS = frozenset({"text_op", "set_op", "regex_op"})


def load_lark():
    """Import the pinned lark from the tool cache; the first import binds it for the process."""
    cache = manifest.cache_root(os.environ)
    tools = manifest.load(os.path.join(REPO, "tools.json"), manifest.platform_key())
    for path in reversed(manifest.python_paths(cache, tools)):
        if path not in sys.path:
            sys.path.insert(0, path)
    import lark  # from the cache; this import is the point of the function

    return lark


class Grammar:
    """The normative grammar, loaded once."""

    def __init__(self, lark_module, grammar_text: str) -> None:
        self.lark = lark_module
        self.parser = lark_module.Lark(grammar_text, parser="lalr", start="start")

    def judge(self, src: str) -> tuple[str, list[str]]:
        """("accept", the shapes in document order) or ("reject", [])."""
        try:
            tree = self.parser.parse(src)
        except self.lark.exceptions.LarkError:
            return "reject", []
        return "accept", [shape(node) for node in expressions(tree)]


def is_tree(node) -> bool:
    return hasattr(node, "data")


def expressions(tree) -> list:
    """Every partition key, filter and select item, in document order (a pre-order walk; holders never nest)."""
    out: list = []

    def walk(node) -> None:
        if not is_tree(node):
            return
        if node.data in HOLDERS:
            out.append(node.children[0])
            return
        if node.data in KEYS:
            out.append(node)
            return
        for child in node.children:
            walk(child)

    walk(tree)
    return out


def cname(node) -> str:
    return str(node.children[0]).strip("`")


def shape(node) -> str:
    if not is_tree(node):  # a token: the only one an expression position holds is a STRING literal
        return "literal" if node.type == "STRING" else f"?token:{node.type}"
    d, c = node.data, node.children
    if d in KEYS:  # the attributes, then the stream after `of`
        names = [cname(n) for n in c]
        return f"({'group' if d == 'part_group' else 'key'} {names[-1]} {' '.join(names[:-1])})"
    if d in LITERALS:
        return "literal"
    if d == "param_ref":
        return "param"
    if d == "bare":
        return "name"
    if d == "qualified":
        return "qualified"
    if d == "raw_wrap":
        return f"(raw {shape(c[0])})"
    if d == "neg":
        return f"(neg {shape(c[0])})"
    if d == "not_op":
        return f"(not {shape(c[0])})"
    if d == "and_op":
        return f"(and {shape(c[0])} {shape(c[1])})"
    if d == "or_op":
        return f"(or {shape(c[0])} {shape(c[1])})"
    if d == "in_table":
        return f"(in {shape(c[0])})"
    if d == "not_in_table":
        return f"(not-in {shape(c[0])})"
    if d == "cmp":
        return f"(cmp {c[1]} {shape(c[0])} {shape(c[2])})"
    if d == "is_null":
        return f"(is-null {shape(c[0])})"
    if d == "is_not_null":
        return f"(is-not-null {shape(c[0])})"
    if d in TEXT_OPS:
        return f"(text {c[1]} {shape(c[0])} {operand(c[2])})"
    if d == "arith":
        return f"(arith {c[1]} {shape(c[0])} {shape(c[2])})"
    if d == "call":
        return "(" + " ".join(["call", cname(c[0]), *arguments(c[1:])]) + ")"
    if d == "ns_call":
        return "(" + " ".join(["call", f"{cname(c[0])}:{cname(c[1])}", *arguments(c[2:])]) + ")"
    return f"?{d}"


def arguments(rest: list) -> list[str]:
    """A call's arg_list, if it has one."""
    return [shape(arg) for arg in rest[0].children] if rest else []


def operand(node) -> str:
    """text_operand: STRING | param_ref; set_list: STRING+; set_operand: param_ref | cname."""
    if node.data == "set_list":
        return f"(strings {len(node.children)})"
    child = node.children[0]
    if is_tree(child):
        return "param" if child.data == "param_ref" else "name"
    return "(strings 1)"
