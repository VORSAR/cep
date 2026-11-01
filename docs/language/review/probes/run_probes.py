#!/usr/bin/env python3
"""Red-team probes against grammar.lark. Each probe: (name, expect_parse, source)."""
import sys, pathlib
from lark import Lark

HERE = pathlib.Path(__file__).resolve().parent
GRAMMAR = (HERE.parent.parent / "grammar.lark").read_text()
parser = Lark(GRAMMAR, start="start", parser="lalr")

HDR = "language cep/0.1;\n"
S1 = "stream S (a string, n float, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n"
S2 = "stream R (a string, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n"
T  = "stream T (a string, ts timestamp);\n"
TB = "table Tab (a string) primary key (a);\n"

def pat(body_from, within="within 5 min", extra=""):
    return (HDR + S1 + S2 + T + TB +
            "rule P\n  intent 'x'\n  partition by a of S, a of R ttl 1 hour\n"
            f"  {body_from}\n  {within}\n  after match skip past last\n  late events drop\n{extra}"
            "  select e1.a\n  insert into T;\n")

PROBES = []
def probe(name, expect_ok, src): PROBES.append((name, expect_ok, src))

# P01 uppercase keywords (SPEC 2.1: keywords matched case-insensitively)
probe("P01-uppercase-keywords", True, HDR + S1 + T + "RULE X\n  INTENT 'x'\n  FROM S[a == 'x']\n  SELECT a\n  INSERT INTO T;\n")
# P01b uppercase-spelled keyword as identifier
probe("P01b-ident-spelled-Select", True, HDR + "stream Select (a string, ts timestamp) time ts watermark bounded 1 min;\n")
# P02 fractional duration (SPEC 2.4/EBNF: <int> <unit>)
probe("P02-fractional-duration", False, pat("from every e1 = S[a == 'x'] -> e2 = S[a == 'y']", within="within 1.5 min"))
# P02b exponent duration
probe("P02b-exponent-duration", False, pat("from every e1 = S[a == 'x'] -> e2 = S[a == 'y']", within="within 1e3 ms"))
# P03 ascending / repeated units (SPEC 2.4: descending)
probe("P03-ascending-units", False, pat("from every e1 = S[a == 'x'] -> e2 = S[a == 'y']", within="within 30 sec 5 min"))
probe("P03b-repeated-units", False, pat("from every e1 = S[a == 'x'] -> e2 = S[a == 'y']", within="within 5 min 5 min"))
# P04 unary minus (SPEC 2.5 'SQL verbatim')
probe("P04-unary-minus-filter", True, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[n < -1]\n  select a\n  insert into T;\n")
probe("P04b-negative-param-default", True, HDR + "param floor float = -1.0;\n")
# P05 absence as only element / absence first
probe("P05-absence-only", False, pat("from every and not R[a == 'y'] for 5 min"))
probe("P05b-absence-first-then-step", False, pat("from every not R[a == 'y'] for 5 min -> e1 = S[a == 'x']"))
# P06 duplicate step names
probe("P06-duplicate-step-name", True, pat("from every e1 = S[a == 'x'] -> e1 = S[a == 'y']"))
# P06b capture name shadowing stream name
probe("P06b-capture-shadows-stream", True, pat("from every R = S[a == 'x'] -> e1 = S[a == R.a]").replace("select e1.a", "select R.a"))
# P07 late events allow on a pattern (SEMANTICS 6.5: drop is the only pattern value)
probe("P07-pattern-late-allow", True, (HDR + S1 + S2 + T + TB +
    "rule P\n  intent 'x'\n  partition by a of S, a of R ttl 1 hour\n"
    "  from every e1 = S[a == 'x'] -> e2 = S[a == 'y']\n  within 5 min\n  after match skip past last\n  late events allow 10 min\n"
    "  select e1.a\n  insert into T;\n"))
# P08 wrong emit mode for window kind (SEMANTICS 8: illegal combo = deploy error; no code)
probe("P08-sort-emit-on-close", True, HDR + S1 + T + "rule W\n  intent 'x'\n  partition by a of S ttl 2 hour\n  from S window sort(2, n, desc)\n  emit on close\n  late events drop\n  select a\n  insert into T;\n")
probe("P08b-delay-late-allow", True, HDR + S1 + T + "rule W\n  intent 'x'\n  partition by a of S ttl 2 hour\n  from S window delay(10 min)\n  emit on due\n  late events allow 5 min\n  select a\n  insert into T;\n")
# P09 select *
probe("P09-select-star", False, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[a == 'x']\n  select *\n  insert into T;\n")
# P10 SQL IN-list
probe("P10-in-list", False, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[a in ('x', 'y')]\n  select a\n  insert into T;\n")
# P11 5min without space
probe("P11-no-space-duration", True, pat("from every e1 = S[a == 'x'] -> e2 = S[a == 'y']", within="within 5min"))
# P12 single = in filter (SQL equality)
probe("P12-sql-equals", False, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[a = 'x']\n  select a\n  insert into T;\n")
# P13 chained comparison
probe("P13-chained-comparison", False, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[0 < n < 1]\n  select a\n  insert into T;\n")
# P14 sort N fractional
probe("P14-sort-fractional-n", True, HDR + S1 + T + "rule W\n  intent 'x'\n  partition by a of S ttl 2 hour\n  from S window sort(2.5, n, desc)\n  emit on change\n  late events drop\n  select a\n  insert into T;\n")
# P15 two absences, order free
probe("P15-two-absences", True, pat("from every e1 = S[a == 'x'] and not R[a == 'y'] for 5 min -> e2 = S[a == 'z'] and not R[a == 'w'] for 5 min"))
# P16 self-referential insert (reads its own output)
probe("P16-self-insert", True, HDR + S1 + "stream Loop (a string, ts timestamp);\n" + "rule X\n  intent 'x'\n  from Loop[a == 'x']\n  select a\n  insert into Loop;\n")
# P16b insert into a SOURCE stream (has time clause)
probe("P16b-insert-into-source", True, HDR + S1 + S2 + "rule X\n  intent 'x'\n  from S[a == 'x']\n  select a\n  insert into R;\n")
# P17 unknown language version
probe("P17-wrong-version", True, "language cep/9.9;\n" + S1)
# P18 empty then-list in expect (negative test)
probe("P18-expect-empty-then", False, HDR + S1 + T +
    "rule X\n  intent 'x'\n  from S[a == 'x']\n"
    "  expect given S [ {a: 'x', ts: timestamp '2026-01-01T00:00:00Z'} ]\n         then T [ ]\n"
    "  select a\n  insert into T;\n")
# P18b expect with two given streams
probe("P18b-expect-two-givens", False, HDR + S1 + S2 + T +
    "rule X\n  intent 'x'\n  from S[a == 'x']\n"
    "  expect given S [ {a: 'x'} ] given R [ {a: 'y'} ] then T [ {a: 'x'} ]\n"
    "  select a\n  insert into T;\n")
# P18c null literal in expect record
probe("P18c-expect-null-literal", False, HDR + S1 + T +
    "rule X\n  intent 'x'\n  from S[a == 'x']\n"
    "  expect given S [ {a: null, ts: timestamp '2026-01-01T00:00:00Z'} ] then T [ {a: 'x'} ]\n"
    "  select a\n  insert into T;\n")
# P19 '--' inside a string
probe("P19-dashes-in-string", True, HDR + S1 + T + "rule X\n  intent 'a -- b'\n  from S[a == '--x']\n  select a\n  insert into T;\n")
# P20 BOM at file start
probe("P20-bom", True, "﻿" + HDR + S1)
# P22 min() aggregate (min is a reserved duration unit)
probe("P22-min-aggregate", False, HDR + S1 + "stream Agg (lo float, ts timestamp);\n" + "rule W\n  intent 'x'\n  partition by a of S ttl 2 day\n  from S window cron('0 0 * * *', 'UTC')\n  emit on close\n  late events drop\n  select min(n) as lo\n  insert into Agg;\n")
probe("P22b-max-aggregate", True, HDR + S1 + "stream Agg (hi float, ts timestamp);\n" + "rule W\n  intent 'x'\n  partition by a of S ttl 2 day\n  from S window cron('0 0 * * *', 'UTC')\n  emit on close\n  late events drop\n  select max(n) as hi\n  insert into Agg;\n")
# P23 param as cron arg
probe("P23-param-cron-tz", False, HDR + "param tz string = 'UTC';\n" + S1 + T + "rule W\n  intent 'x'\n  partition by a of S ttl 2 day\n  from S window cron('0 0 * * *', ${tz})\n  emit on close\n  late events drop\n  select a\n  insert into T;\n")
# P24 text operator on a number literal LHS
probe("P24-textop-on-number", True, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[5 has_ci 'x']\n  select a\n  insert into T;\n")
# P25 not (x in T) paren form
probe("P25-not-paren-in", True, HDR + S1 + T + TB + "rule X\n  intent 'x'\n  from S[not (a in Tab)]\n  select a\n  insert into T;\n")
# P26 backtick containing arrow
probe("P26-backtick-arrow", True, HDR + "stream `a->b` (a string, ts timestamp) time ts watermark bounded 1 min;\n")
# P27 zero durations
probe("P27-zero-duration", True, pat("from every e1 = S[a == 'x'] and not R[a == 'y'] for 0 ms", within="within 0 ms"))
# P28 unterminated block comment
probe("P28-unterminated-block-comment", False, HDR + S1 + "/* never closed\n")
# P30 window on derived stream
probe("P30-window-on-derived", True, HDR + S1 + T + "stream U (a string, ts timestamp);\n" + "rule W\n  intent 'x'\n  partition by a of T ttl 1 hour\n  from T window delay(1 min)\n  emit on due\n  late events drop\n  select a\n  insert into U;\n")
# P32 absence inside a group
probe("P32-absence-in-group", False, pat("from every (e1 = S[a == 'x'] and not R[a == 'y'] for 5 min)"))
# P33 group of three steps
probe("P33-group-of-three", False, pat("from every (e1 = S[a == 'x'] and e2 = R[a == 'y'] and e3 = S[a == 'z'])"))
# P34 ${...} inside a plain string (annotation style)
probe("P34-paramref-inside-string", True, HDR + "@sink(type = 'http', url = '${IAM_WEBHOOK}')\n" + T)
# P35 chain() with zero args
probe("P35-chain-zero-args", True, HDR + S1 + "stream U (evidence object, ts timestamp);\n" + "rule X\n  intent 'x'\n  from S[a == 'x']\n  select chain() as evidence\n  insert into U;\n")
# P36 duplicate select aliases
probe("P36-duplicate-aliases", True, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[a == 'x']\n  select a as a, 'y' as a\n  insert into T;\n")
# P37 two windows in one rule
probe("P37-two-windows", False, HDR + S1 + T + "rule W\n  intent 'x'\n  partition by a of S ttl 2 hour\n  from S window delay(1 min) window sort(2, n, desc)\n  emit on due\n  late events drop\n  select a\n  insert into T;\n")
# P38 chained text ops (a has_ci 'x' has_ci 'y')
probe("P38-chained-textop", False, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[a has_ci 'x' has_ci 'y']\n  select a\n  insert into T;\n")
# P39 'is null' on arbitrary expr + in-table LHS being is-null result
probe("P39-in-lhs-is-null", True, HDR + S1 + T + TB + "rule X\n  intent 'x'\n  from S[a is null in Tab]\n  select a\n  insert into T;\n")
# P40 arith between comparisons: a == b == c
probe("P40-chained-equality", False, HDR + S1 + T + "rule X\n  intent 'x'\n  from S[a == 'x' == 'y']\n  select a\n  insert into T;\n")

def main():
    failures = []
    for name, expect_ok, src in PROBES:
        try:
            parser.parse(src)
            ok = True
            err = None
        except Exception as e:
            ok = False
            err = f"{type(e).__name__}: {str(e).splitlines()[0][:110]}"
        verdict = "as-expected" if ok == expect_ok else "SURPRISE"
        print(f"{name:34s} parse={'OK ' if ok else 'ERR'} expected={'OK ' if expect_ok else 'ERR'} -> {verdict}" + (f"   [{err}]" if err and ok != expect_ok else ""))
        if ok != expect_ok:
            failures.append(name)
    print("\nSurprises (grammar disagrees with probe author's expectation):", failures or "none")

if __name__ == "__main__":
    main()
