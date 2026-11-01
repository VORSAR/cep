#!/usr/bin/env python3
"""Second probe batch: reserved-word list gaps, annotation params, expect/extras, groups."""
import pathlib
from lark import Lark

HERE = pathlib.Path(__file__).resolve().parent
GRAMMAR = (HERE.parent.parent / "grammar.lark").read_text()
parser = Lark(GRAMMAR, start="start", parser="lalr")

HDR = "language cep/0.1;\n"
S1 = "stream S (a string, n float, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n"
S2 = "stream R (a string, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n"
T  = "stream T (a string, ts timestamp);\n"

PROBES = [
 # 'for' is used as a keyword (absence bound) but missing from NAME's reserved lookahead.
 ("P41-stream-named-for",      HDR + "stream for (a string, ts timestamp) time ts watermark bounded 1 min;\n"),
 ("P41b-attr-named-for",       HDR + "stream S2 (for string, ts timestamp) time ts watermark bounded 1 min;\n"),
 ("P41c-filter-on-attr-for",   HDR + S1 + T + "rule X\n  intent 'x'\n  from S[for == 'x']\n  select for\n  insert into T;\n"),
 # annotation value as a structural param ref (not inside a string)
 ("P47-annotation-param-ref",  HDR + "@sink(type = 'http', url = ${WEBHOOK})\n" + T),
 # two expect blocks in one rule
 ("P50-two-expects",           HDR + S1 + T + "rule X\n  intent 'x'\n  from S[a == 'x']\n"
   "  expect given S [ {a: 'x'} ] then T [ {a: 'x'} ]\n"
   "  expect given S [ {a: 'y'} ] then T [ {a: 'y'} ]\n"
   "  select a\n  insert into T;\n"),
 # two sibling and-groups in one pattern
 ("P57-two-groups",            HDR + S1 + S2 + T + "rule P\n  intent 'x'\n  partition by a of S, a of R ttl 1 hour\n"
   "  from every (e1 = S[a == 'x'] and e2 = R[a == 'y']) -> (e3 = S[a == 'z'] and e4 = R[a == 'w'])\n"
   "  within 5 min\n  after match skip past last\n  late events drop\n  select e1.a\n  insert into T;\n"),
 # timestamp literal in a comparison
 ("P45-timestamp-comparison",  HDR + S1 + T + "rule X\n  intent 'x'\n  from S[ts > timestamp '2026-01-01T00:00:00Z']\n  select a\n  insert into T;\n"),
 # unknown escape inside a TRIPLE-quoted string (E-STR-002 scope ambiguity)
 ("P49-tripleq-weird-escape",  HDR + S1 + T + "rule X\n  intent '''uses \\q escape'''\n  from S[a == 'x']\n  select a\n  insert into T;\n"),
 # duration literal arithmetic in a filter (ts - 5 min style)
 ("P58-duration-arith",        HDR + S1 + T + "rule X\n  intent 'x'\n  from S[ts + 5 min > timestamp '2026-01-01T00:00:00Z']\n  select a\n  insert into T;\n"),
 # rate limit with zero-length: output first every with param
 ("P59-pattern-no-filter-step", HDR + S1 + S2 + T + "rule P\n  intent 'x'\n  partition by a of S, a of R ttl 1 hour\n"
   "  from every e1 = S -> e2 = R\n  within 5 min\n  after match skip past last\n  late events drop\n  select e1.a\n  insert into T;\n"),
 # emit on batch end over cron (another illegal combo that parses)
 ("P60-cron-emit-batch-end",   HDR + S1 + T + "rule W\n  intent 'x'\n  partition by a of S ttl 2 day\n  from S window cron('0 0 * * *', 'UTC')\n  emit on batch end\n  late events drop\n  select a\n  insert into T;\n"),
]

for name, src in PROBES:
    try:
        parser.parse(src)
        print(f"{name:32s} parse=OK")
    except Exception as e:
        print(f"{name:32s} parse=ERR  [{type(e).__name__}: {str(e).splitlines()[0][:100]}]")
