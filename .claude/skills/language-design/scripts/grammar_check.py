#!/usr/bin/env python3
"""Check a Lark grammar against an example corpus.

Usage:
  python grammar_check.py GRAMMAR.lark EXAMPLES_DIR [--invalid INVALID_DIR] [--ambiguity] [--start START]

What it does:
  1. Builds the grammar with the LALR parser and reports conflicts (the build fails if there are any).
  2. Parses every *.txt / *.cep / *.dsl / *.q file under EXAMPLES_DIR; every file must parse.
  3. With --invalid, parses every file under INVALID_DIR; every file must FAIL to parse (or fail a
     static check you plug in below). First-line comment of the form `-- expect: E-XXX-NNN` is
     reported next to the result so you can compare with your checker's output.
  4. With --ambiguity, re-parses the corpus with the Earley parser in ambiguity='explicit' mode
     and reports any file whose parse tree contains an `_ambig` node.
  5. Prints the number of rules and terminals so you can track grammar size against the cap.

Requires: pip install lark
Exit code 0 if all checks pass, 1 otherwise.
"""
import argparse
import pathlib
import sys

try:
    from lark import Lark, Tree
    from lark.exceptions import LarkError
except ImportError:  # pragma: no cover
    print("lark is not installed: pip install lark", file=sys.stderr)
    sys.exit(2)

EXTS = {".txt", ".cep", ".dsl", ".q", ".lang"}


def corpus(d: pathlib.Path):
    return sorted(p for p in d.rglob("*") if p.suffix in EXTS and p.is_file())


def has_ambig(t):
    if isinstance(t, Tree):
        if t.data == "_ambig":
            return True
        return any(has_ambig(c) for c in t.children)
    return False


def grammar_size(parser: Lark):
    rules = {r.origin.name for r in parser.rules}
    terms = {t.name for t in parser.terminals}
    return len(rules), len(terms)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("grammar")
    ap.add_argument("examples")
    ap.add_argument("--invalid")
    ap.add_argument("--ambiguity", action="store_true")
    ap.add_argument("--start", default="start")
    a = ap.parse_args()

    src = pathlib.Path(a.grammar).read_text()
    ok = True

    # 1. LALR build (conflicts raise here)
    try:
        lalr = Lark(src, parser="lalr", start=a.start, propagate_positions=True)
    except Exception as e:  # GrammarError on conflicts
        print(f"[LALR] grammar does not build: {e}")
        return 1
    nr, nt = grammar_size(lalr)
    print(f"[size] {nr} rules, {nt} terminals (target: < ~60 rules)")

    # 2. valid corpus
    files = corpus(pathlib.Path(a.examples))
    if not files:
        print(f"[corpus] no example files under {a.examples}")
        ok = False
    for f in files:
        try:
            lalr.parse(f.read_text())
            print(f"[valid] OK   {f}")
        except LarkError as e:
            ok = False
            msg = str(e).splitlines()[0]
            print(f"[valid] FAIL {f}: {msg}")

    # 3. invalid corpus
    if a.invalid:
        for f in corpus(pathlib.Path(a.invalid)):
            text = f.read_text()
            first = text.splitlines()[0] if text else ""
            expect = first.split("expect:", 1)[1].strip() if "expect:" in first else "?"
            try:
                lalr.parse(text)
                # parsed: acceptable only if a static check is meant to catch it
                print(f"[invalid] PARSED (needs static check {expect}) {f}")
            except LarkError:
                print(f"[invalid] rejected by grammar (expected {expect}) {f}")

    # 4. ambiguity
    if a.ambiguity:
        earley = Lark(src, parser="earley", ambiguity="explicit", start=a.start)
        for f in files:
            try:
                t = earley.parse(f.read_text())
            except LarkError:
                continue
            if has_ambig(t):
                ok = False
                print(f"[ambig] AMBIGUOUS {f}")
        print("[ambig] done")

    print("[result]", "PASS" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
