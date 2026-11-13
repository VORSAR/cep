> **Historical — S1.1c-3, the documents and the clocks; merged in PR #38.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-3-plan-check.md) · [design](../sprints/s1-1c-3-design.md) · [record](../sprints/s1-1c-3-record.md) · reviews: [research fragments](s1-1c-3-research-fragments.md), [research fragments angle](s1-1c-3-research-fragments-angle.md), [research vuln canary](s1-1c-3-research-vuln-canary.md), [plan check architecture](s1-1c-3-plan-check-architecture.md), [plan check qa](s1-1c-3-plan-check-qa.md), [design architecture](s1-1c-3-design-architecture.md), [design qa](s1-1c-3-design-qa.md), [combined architecture](s1-1c-3-combined-architecture.md), [breakit qa](s1-1c-3-breakit-qa.md). **← [S1.1c-2](../sprints/s1-1c-2-record.md) · [S1.1c-4 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-3 plan check research, 2026-11-12 — the fragment rule re-measured with angle placeholders bound (R-F3).** A read-only run in a scratch clone of `main` @ c87942d, asked for by the plan check's blind reviews (A-B2, QA-B1, QA-B2): the rule as decided (R), angle placeholders bound from a closed vocabulary, `text` legal only for characters the grammar has no token for, the incident's shape checked everywhere, and each documented-invalid span's failing stage; folded into [the plan check](../sprints/s1-1c-3-plan-check.md) §0.2. Paths below are the session scratchpad's.

# S1.1c-3 plan-check research: the rule-fragment check under the strict angle/Bad rule (main @ c87942d)

Base: R-F2's scratch clone (c87942d, the seven productions, zz harnesses), copied to fragments-angle/ (originals untouched). Go go1.27.4 pinned, GOTOOLCHAIN=local. docs/language in the clone = the repo's (diff: only __pycache__). Repo untouched.
Scratch: /private/tmp/claude-501/-Users-ramith-code-cep/33e77a3e-2c9e-4bc7-ac54-01eb79c6a3d0/scratchpad/s1-1c-3-research/fragments-angle/
- tools/extract_angle.py (R-F2's extractor + the vocabulary, glued-ellipsis rule, strict text rule, incident flag); tools/classify_angle.py, tools/counts_angle.py; tools/corr_angle.py (corrections)
- spans.jsonl, results.jsonl, classified.txt, counts.txt, corr-out.jsonl, corr2-out.jsonl
- cep/internal/parser/zz_entry2_test.go (throwaway entry points), cep/internal/zzcounter/zz_test.go (counter-examples in whole programs: Parse + validate.Check)
All harness runs exit 0. Headline is **R** (per the coordinator's change of base); R' is the second column.

## 1. The rule as applied

1. **Angle placeholders bound** (outside quotes) before R-F2's binding rules 1-9. A placeholder glued to an ellipsis (`<step>…`) means "one or more" and binds as one. Inside quotes a placeholder stays a string (`'<one sentence>'`, `'<sentinel>'`, `'<ISO-8601>'`).
2. **`text` legal only when a Bad character survives binding** (`→ … | ± × − ≤ ≥ ⟨ ⟩ ∞ ₂`, unclosed `'`). Optional brackets (`[not]`, `[optional]`) no longer license `text`. A placeholder outside the vocabulary = "unknown placeholder".
3. Incident shape `(->|→)\s*and\b` checked on every span and fence.
4. Else as R-F2 (R: ≥1 keyword incl. types/units/literals/text-ops, ≥2 tokens, not path-like; R': structural keywords only).

**Placeholders found in backticked spans** (15 distinct, 35 occurrences): `<attr>` 11 (SPEC 4, SEM 3, errors 4), `<duration>` 10 (2/1/7), `<Name>` 3 (2/0/1), `<sentinel>` 3 (errors, in quotes), `<Stream>` 2, `<key>` 2, `<step>` 2 (errors), and 1 each: `<ISO-8601>` (SPEC, in quotes), `<count>`, `<unit>`, `<op>` (SPEC), `<Table>`, `<expr>` (SEM), `<ThatStream>`, `<one sentence>` (errors, in quotes).

**Vocabulary used:** `<duration>`→`5 min`; `<Name>`→`Nm`; `<name>`→`nm`; `<Stream>`→`Src`; `<ThatStream>`→`Src2`; `<Table>`→`Tbl`; `<type>`→`string`; `<literal>`→`1`; `<expr>`/`<pred>`/`<cond>`→`x == 1`; `<attr>`/`<attrs>`/`<cols>`→`a`; `<key>`→`k`; `<count>`→`5`; `<unit>`→`min`; `<op>`→`==`; `<step>`→`a = A`; plus for the corrections `<n>`→`10`, `<dir>`→`asc`, `<tz>`→`'UTC'`, `<time-clause>`→`time arrival` (last one used only in §4's run).
**Unknown placeholders: 0.**
**Two vocabulary collisions (run):** `<expr>` as a boolean makes `cron(<expr>, <tz>)` fail (cron wants a string); `<attrs>`→`a` makes `table <Name> (<attrs>) …` fail (a table needs `a <type>`). A single stand-in per name does not fit every position.

## 2. Counts

| file | cep | cep-meta | cep-invalid | text | exempt (kw-only) | **RED R** | total R | RED R' (2nd col) | total R' |
|---|---|---|---|---|---|---|---|---|---|
| SPEC | 22 | 12 | 1 (+1 that parses: `(k) of S`) | 13 | 19 | **20** | 88 | 14 | 68 |
| SEMANTICS | 15 | 13 | 0 | 12 | 8 | **22** | 70 | 14 | 53 |
| errors.md | 20 | 5 | 6 | 1 | 13 | **21** | 66 | 20 | 56 |
| all | 57 | 30 | 8 | 26 | 40 | **63** | 224 | 48 | 177 |

R' per file (cep/meta/invalid/text/exempt/RED): SPEC 19/12/1+1/4/17/14; SEM 13/13/0/5/8/14; errors 15/5/3/0/13/20.

- **R: 63 red, 59 beyond the four approved** (R-F2 had 44/40). R': 48 red, 44 beyond.
- The +19 against R-F2 are all spans R-F2 let through as `text`: 5 optional-bracket spans (`is [not] null` ×4, SPEC:77's stream form), 5 placeholder spans that do not parse once bound (SPEC:183, SPEC:285, errors:26, :92, :96), 6 entry-point gaps (SPEC:77 and errors:88 `idle after`, errors:43 `ttl`, errors:11 `rule <Name>`, errors:23 ×2), 2 partial time clauses (SPEC:284, errors:83) and SEM:37's quoted pre-v0.1 form.
- 11 former `text` spans now parse as `cep-meta`:
  - SPEC:77 ×2 time clauses, SPEC:137 `raw(attr) <op> …`
  - SEM:37 ×2, SEM:69 `<expr> in <Table>`
  - errors:15 `within <duration>`, :26 and :27 `for <duration>`, :75 ×2
- `text` falls from 56 to 26.

## 3. The red list beyond the four approved (R; "R-only" = drops under R')

The approved four are still red and their corrections parse (run): SEM:98 → `text contains_any_ci('ignore previous')` (expr); SEM:128 → `and not HumanApprovals[sessionId == read.sessionId] for 5 min` (chainelem); SEM:136 → `late events drop` (late); SEM:180 → `output first every 10 min` (rate).

### 3a. Real fragment errors (the doc must change); corrections run unless marked

| file:line | span | binding | parser message (best kind) | proposed minimal correction | run |
|---|---|---|---|---|---|
| SPEC:75 | `param name type = literal;` | none | statements 1:12 unexpected "type" | `param <name> <type> = <literal>;` | parses (statements) |
| SPEC:77 | `stream Name (attr type [optional], …) time-clause;` | `, …`→removed | statements 1:19 unexpected "type" | `stream <Name> (<attr> <type> optional, …) <time-clause>;` (needs `<time-clause>` in the vocabulary; writing `optional` without brackets drops the sign that the word may be left out) | parses (statements) |
| SPEC:79 | `table Name (attrs) primary key (cols);` | none | statements 1:18 unexpected ")" | `table <Name> (<attr> <type>, …) primary key (<cols>);` | parses (statements) |
| SPEC:170 | `cron(expr, tz)` | none | windowkind 1:6 unexpected "expr" (expected StrTok) | `cron('<expr>', '<tz>')` | parses (windowkind); deploy-time cron check would refuse `'<expr>'` (inferred) |
| SPEC:172 | `sort(n, attr, dir)` | none | windowkind 1:6 unexpected "n" | `sort(<n>, <attr>, <dir>)` | parses (windowkind) |
| SPEC:179 | `raw()` | none | expr 1:5 unexpected ")" | `raw(attr)` | parses (expr) |
| SPEC:183 | `given table <Name> [ {row}, … ]` | Nm; `, …` removed | givensec 1:18 unexpected "{" | `given table <Name> [ {<attr>: <literal>}, … ]` | parses (givensec) |
| SPEC:285 | `given table <Name> [rows]` | Nm | givensec 1:17 unexpected "rows" | `given table <Name> [ {<attr>: <literal>}, … ]` | parses (givensec; run as `[{<attr>: <literal>}]` and as 183's form) |
| SPEC:42, SPEC:42, SPEC:156, errors:237 | `is [not] null` (4 spans) | none (optional bracket no longer licenses text) | aftermatch 1:1 unexpected "is" | `<attr> is null` / `<attr> is not null` (two spans each) | both parse (expr). `is null` alone fails everywhere (entry point zIsNull parses) |
| SEM:93 ×2 | `any of`, `all of` | none | partkey 1:7 unexpected EOF | name the real operators (`has_any_*`/`contains_any_*`, `has_all_*`/`contains_all_*`); single tokens are not rule-like | run: `has_any_ci` alone fails everywhere, but 1 token → unchecked |
| SEM:178 | `every D` (early firing, P2, "forbidden now") | D→5 min | chain 1:7 unexpected "5" | no `text` escape left: mark `cep-invalid` (it fails at grammar), or un-backtick | `every <duration>` fails (chain) |
| SEM:180 | `first every D` | D→5 min | aftermatch 1:1 unexpected "first" | `output first every <duration>` | parses (rate) |
| SEM:190 | `not … for 5 min` | …→A1 | expr 1:8 unexpected "for" | `and not … for 5 min` | parses (chainelem) |
| errors:26 | `and not <Stream>[…]` | Src, […]→[x == 1] | chainelem 1:21 EOF (expected ForBound) | `cep-invalid` (E-PAT-004, see §5), or `and not <Stream>[…] for <duration>` | correction parses (chainelem) |
| errors:131 | `and not …` | …→A1 | chainelem 1:11 EOF (expected ForBound) | `and not <Stream>[…] for <duration>` | parses (chainelem) |
| errors:92 | `partition by <key> of <Stream> … ttl <duration>` | `…`→ A1 (rule 6) | partition 1:23 unexpected "A1" (expected "ttl") | `partition by <key> of <Stream>, <key> of <ThatStream> ttl <duration>` | parses (partition); `, … ttl` still fails |
| errors:96 | `, <key> of <ThatStream>` | k, Src2 | aftermatch 1:1 unexpected "," | `<key> of <ThatStream>` (sentence keeps "add … to the clause") | parses (partkey) |
| errors:245 | `sort(n, …)` | `, …`→removed | windowkind 1:6 unexpected "n" | `sort(<n>, <attr>, <dir>)` | parses; `sort(<n>, …)` fails (1:10 unexpected ")") |
| SPEC:284 | `time <attr>` (changelog) | a | timeclause 1:7 EOF (expected "watermark") | a partial clause: `time <attr> watermark bounded <duration>` widens the meaning; else an entry point or un-backtick | full clause parses (timeclause) |
| errors:83 | `time <attr>` | a | same | same as SPEC:284 | same |

### 3b. Grammar-valid, no entry point (each parses with a throwaway production; zz_entry2_test.go and R-F2's zz_entry_test.go, all `<nil>`)

| file:line | span | missing production | pinned Fix line? |
|---|---|---|---|
| SPEC:77, SEM:55 ×2, errors:88 | `idle after <duration>` / `idle after D` | IdleClause | no |
| errors:43 | `ttl <duration>` | TTL clause | **yes** |
| errors:11 | `rule <Name>` | rule head | **yes** |
| errors:11 | `intent '<one sentence>'` | IntentClause | **yes** |
| errors:23 ×2 | `from every <step>…`, `from once <step>…` | FromClause (`'from' PatternChain`) | **yes**. `every <step>…` alone parses as chain, but that changes pinned text |
| errors:39 | `window cron('0 0 * * *', 'America/New_York')` | WindowClause | **yes** |
| errors:188 | `primary key (…)` | PrimaryKey | no |
| errors:116 | `(k, j) of S, (k2, j2) of T` | bare key list | no |

Four pinned Fix lines (11, 23, 39, 43) hold six red spans: editing them changes errors.go `fixes` and user-visible text; entry points avoid that.

### 3c. Prose spans

**Under both R and R':**
- tuples: SEM:11 `(stream, timestamp, attributes)`, SEM:61 `(event_time, stream id, per-stream ingestion sequence)`, SEM:135 `(ts, stream id, seq)`
- quoted diagnostics: errors:100, errors:104, errors:116 `the partition keys of rule R …`
- a lark note: SPEC:280 `stream S`
- a Go symbol: errors:119 `partition.NewRule`

**R-only (drop under R'), 15 spans:**
- SPEC:38 `timestamp '<ISO-8601>'`. The placeholder stays a string, so lowering fails: `LOWER E-TYPE-001 … not ISO-8601`.
- SPEC:134 `contains_ci contains_cs starts_with_* ends_with_*`
- SPEC:156 `duration / int`, `timestamp + timestamp`, `duration / duration`
- SPEC:277 `duration / int`
- SEM:13 `duration / int`
- SEM:15 `ms, sec, min, hour, day, week`
- SEM:47 `wm_construct = max(…)`
- SEM:57 and SEM:128 `d = 2 min`
- SEM:77 `text_invisibles=true`
- SEM:190 `TTL 3 min < pattern bound 5 min`
- errors:237 `timestamp + timestamp`
- SEM:37 `timestamp = <attr>` (it is also a counter-example; see §5)

**Count check, R:**

| group | spans |
|---|---|
| approved | 4 |
| 3a | 24 |
| 3b | 12 |
| 3c | 23 (8 + 15 R-only) |
| **total** | **63** |

## 4. R-F2's `<angle>` corrections, now bound

| R-F2 correction | bound | verdict |
|---|---|---|
| SPEC:75 `param <name> <type> = <literal>;` | `param nm string = 1 ;` | **parses** (statements) |
| SPEC:79 `table <Name> (<attrs>) primary key (<cols>);` | `table Nm ( a ) primary key ( a );` | **fails** statements 1:14 unexpected ")" (`<attrs>` needs a type); `table <Name> (<attr> <type>, …) primary key (<cols>);` parses |
| SPEC:170 `cron(<expr>, <tz>)` | `cron( x == 1 , 'UTC' )` | **fails** windowkind 1:7 unexpected "x" (expected StrTok); `cron('<expr>', '<tz>')` parses |
| SPEC:172 `sort(<n>, <attr>, <dir>)` | `sort( 10 , a , asc )` | **parses** (windowkind) |
| errors:245 `sort(<n>, …)` | `sort( 10 )` | **fails** 1:10 unexpected ")"; the full `sort(<n>, <attr>, <dir>)` parses |
| errors:131 `and not <Stream>[…] for <duration>` | `and not Src [x == 1] for 5 min` | **parses** (chainelem) |
| SEM:178 `every <duration>` | `every 5 min` | **fails** chain 1:7 (P2, not in the grammar) |
| errors:188 `primary key (<cols>)` | `primary key ( a )` | **fails** at every kind (entry-point gap) |
| errors:11 `intent '<one sentence>'` | unchanged (string) | **fails** at every kind (entry-point gap) |

## 5. Documented-invalid spans (`cep-invalid` candidates) and where they fail

Each was embedded in a whole program (examples 01/02 with one substitution) and run through `parser.Parse` then `validate.Check`. Both baselines are clean.

| span | where it fails | evidence |
|---|---|---|
| SPEC:42 `select *`, errors:274 `select *` | **grammar** | `p.cep:15:10: E-SYN-004` |
| errors:62 `followed by` | **grammar** | `p.cep:30:5: E-SYN-001 … canonical form is ->` |
| errors:209 `1.5 min`, `30 sec 5 min`, `5 min 5 min` | **lowering** | `E-DUR-002` at 5:29 (lower.go); the fragment harness's `duration` kind also fails only at LOWER |
| SPEC:98 `(k) of S` | **validator only** (parses, lowers) | `E-PART-005` (internal/validate/bounds.go:104) |
| errors:103 `k of S, j of S` | **validator only** | `E-PART-004` (bounds.go:78). As a bare fragment it fails only because no key-list entry point exists |
| *new candidate* errors:26 `and not <Stream>[…]` | **grammar** | `p.cep:33:3: E-PAT-004` |
| *new candidate* SEM:37 `timestamp = <attr>` (the quoted pre-v0.1 form) | **grammar** | fails at every fragment kind; not run as a whole program (inferred) |
| *new candidate* SEM:178 `every D` (P2, forbidden) | **grammar** | fails at every fragment kind |

- The counts table carries R-F2's 8.
- With the three new candidates there are 11 under R and 7 under R' (R' drops the three errors:209 durations and SEM:37).
- A grammar-only or grammar+lowering `cep-invalid` check would call the 2 validator-only spans red. They need a whole-program, validator-backed kind or an exception.

## 6. Keyword-only spans containing `->`, and the incident shape

**Keyword-only with `->`.** There are 4 spans, each the single token `->`: SPEC:127, SEM:124 ×2, errors:62.
- Each fails at every kind; a bare `->` is no production.
- None is rule-like under R. They have 1 token, and `->` is an Arrow, not a Keyword.
- No keyword-only span of 2 or more tokens contains `->`.
- If the rule required these 4 to parse or be `cep-invalid`, all 4 would be red. They are mentions of the connector.

**The incident shape `(->|→)\s*and\b`.** No span or fence matches, in all three files.

Two spans show the related absence-without-`and` shape, which the regex misses:
- SEM:124 `→ not B for D`. It is text-legal through `→`, and quoted in a correction note as the wrong form.
- SEM:190 `not … for 5 min`. It is RED in §3a.

Widening the shape to `(->|→)\s*(and\s+)?not\b` would catch SEM:124. This is a recommendation, not run as a rule.
