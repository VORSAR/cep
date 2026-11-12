> **S1.1c-3 plan check research, 2026-11-12 — the rule fragments the fragment check would turn red on landing (R-F2).** A read-only research run in a scratch clone of `main` @ c87942d, re-measuring [R-F](s1-1c-2-research-fragments.md) under the owner's rule of 2026-11-12 (decision 7) with errors.md in scope; its findings are folded into [the plan check](../sprints/s1-1c-3-plan-check.md) §0. Paths below are the session scratchpad's.

# S1.1c-3 plan-check research: what the rule-fragment check turns red when it lands (main @ c87942d)

Base: a scratch clone of /Users/ramith/code/cep at c87942d. Go go1.27.4 (pinned; the tools cache's go-go1.27.4-91a44a2b8a71/bin, GOTOOLCHAIN=local), participle v2.1.4. The repository was not touched (`git status` is clean).
Scratch: /private/tmp/claude-501/-Users-ramith-code-cep/33e77a3e-2c9e-4bc7-ac54-01eb79c6a3d0/scratchpad/s1-1c-3-research/fragments/
- tools/extract.py: extracts spans and fences, applies the binding table, runs detector R.
- tools/classify.py and tools/counts.py: classify the spans and count them.
- spans.jsonl, results.jsonl, classified.txt, counts.txt: the data.
- cep/: the clone, with the seven productions and the zz_frag/zz_entry harnesses.
- cep-clean/: a worktree at c87942d, for items 5 and 6.

`git diff --stat fd274e4 c87942d -- internal/parser docs/language internal/ast` prints nothing. The span totals match the prior run: SPEC 432, SEMANTICS 214. errors.md adds 159 spans. The 9 SPEC fences give the same verdicts as before. errors.md and SEMANTICS have no fences.

## 1. The detector and the binding table (exact)

**Lexing.** Lexing uses the lexer's own table:
- lexer.go `rules`, in the same order, as one first-match alternation;
- the keyword set read from keywords.go `keywordList`, matched exactly lowercase.

Two meta tokens are added: `Angle` = `<[A-Za-z][A-Za-z0-9_ '-]*>`, and `Bad` = any character the table refuses (`→ … | ± × − ≤ ≥ ⟨ ⟩ ₂`, or an unclosed `'`).

**Binding table.** The rules apply in this order, outside quoted strings and outside `<…>`:
1. `${…}` → `${p}`
2. `[…]` → `[x == 1]`
3. a leading `…` before `->` or `and` → `every a0 = A0`
4. `, …` before `)`, `]` or the end → removed
5. `= …` → `= A1`
6. any other `…` → ` A1 ` (a source)
7. `D` → `5 min`
8. `pred` and `filter` → `x == 1`
9. `Stream` → `Src`

A, B, S and T bind to themselves; they are already legal names.

**Rule-like (R).** A span is rule-like when all of these hold:
- it has at least one Keyword token in the original span;
- it has at least 2 tokens;
- it is not path-like (`^[\w.-]+(/[\w.*-]+)+/?$` with no space).

**Unbindable.** A span is unbindable when, after binding, it holds an Angle or Bad token, or an optional bracket (`[` followed only by keywords, then `]`, as in `[not]` or `[optional]`). Then `text` is legal.

**Keyword-only.** Every code token is a Keyword or `->`. Such a span gets `cep:` if it parses at some kind. Otherwise it is an exempt mention (`not in`, `idle after`, `emit on close`…).

**Every other R span must parse at some kind.** The kinds:
- program (Parse with lowering), trivia, `Statement+`, RuleDecl (+lowering), Body (+lowering), PatternBody, PatternChain;
- ChainElem, PosElem, Step, Source;
- WithinClause, AfterMatchClause, LateEventsClause, RateClause, ForBound;
- Partition, PartitionKeys, PartKey;
- Expr (+lowering), Duration (+lowering);
- TimeClause, WindowKind, SelectClause, Annotation, Extra, GivenSec, Record.

**The seven productions, in the clone only.** PatternChain, WithinClause, AfterMatchClause and LateEventsClause are embedded in PatternBody, and PartitionKeys in Partition. RateClause and ForBound are named pointers. lower.go has 2 one-line edits: `r.Rate.Every` and `c.For.D`. `go test ./internal/...` gives exit 0.

**New, not in the prior report: the ForBound pointer changes the error text.** Participle now says `expected ForBound` instead of `"for"`. invalid/e-pat-004 then fell to `E-SYN-001 … (expected ForBound)` until parseError's E-PAT-004 case also matched `expects("ForBound")`. Any named-pointer production needs the same audit of parseError's `expects(...)` strings. (RateClause is optional, so no corpus file showed it. That it never shows is inferred.)

**Sensitivity variants:**
- **R'** (a refinement I suggest): the same as R, but only structural keywords count. Type names, duration units, `true`/`false`/`null` and the text or set operators do not count.
- **R+** (for information only): spans with no keyword but an operator or call shape. The rule does not require markers on them.

## 2. Counts (marker forms per file)

| detector | file | `cep:` (a, literal) | `cep-meta:` (b, after binding) | `cep-invalid:` (e) | `text` (d) | exempt kw-only | **RED (c)** | rule-like total |
|---|---|---|---|---|---|---|---|---|
| R | SPEC | 22 | 9 | 1 (+1 that parses: `(k) of S`) | 24 | 19 | **12** | 88 |
| R | SEMANTICS | 15 | 10 | 0 | 16 | 8 | **21** | 70 |
| R | errors.md | 20 | 0 | 6 | 16 | 13 | **11** | 66 |
| R' | SPEC | 19 | 9 | 1+1 | 15 | 17 | **6** | 68 |
| R' | SEMANTICS | 13 | 10 | 0 | 8 | 8 | **14** | 53 |
| R' | errors.md | 15 | 0 | 3 | 15 | 13 | **10** | 56 |

- Under R, 44 spans are red on landing. 4 are the approved corrections, so **40 more**.
- Under R', 30 are red: **26 more**.
- R+ (no marker required) would add 39 spans that fail at every kind. They are operator lists, timestamps such as `wm = 10:02`, `x <> 5`, `has_*`, and similar.
- Markers to write under R: SPEC 88 − 19 = 69, SEMANTICS 62, errors.md 53. That is about 184, before the red spans are fixed.

## 3. The (c) list beyond the four approved corrections (R)

The approved four are confirmed red today, and each correction parses:
- SEM:98 → `text contains_any_ci('ignore previous')` parses as expr.
- SEM:128 → `and not HumanApprovals[sessionId == read.sessionId] for 5 min` parses as chainelem.
- SEM:136 → `late events drop` parses as late.
- SEM:180 → `output first every 10 min` parses as rate.

Each proposed correction marked "parses" was run.

### 3a. Real rule-fragment errors (the doc must change)

| file:line | span | binding | parser message (best kind) | proposed minimal correction | correction run |
|---|---|---|---|---|---|
| SEM:180 | `first every D` | D→5 min | aftermatch `1:1 unexpected token "first"` (no kind starts with `first`) | `output first every D` | parses (rate) |
| SEM:178 | `every D` (early firing, P2) | D→5 min | chain `1:7 unexpected token "5" (expected PosElem ChainElem*)` | `every <duration>` (a P2 construct, so text) | inferred |
| SEM:190 | `not … for 5 min` | …→A1 | expr `1:8 unexpected token "for"` (the incident class: an absence with no `and`) | `and not … for 5 min` | parses (chainelem) |
| SPEC:75 | `param name type = literal;` | none | statements `1:12 unexpected token "type" (expected ("string"…) …)` | `param <name> <type> = <literal>;` | text |
| SPEC:79 | `table Name (attrs) primary key (cols);` | none | statements `1:18 unexpected token ")" …` | `table <Name> (<attrs>) primary key (<cols>);` | text |
| SPEC:170 | `cron(expr, tz)` | none | windowkind `1:6 unexpected token "expr" (expected StrTok …)` | `cron(<expr>, <tz>)` | text |
| SPEC:172 | `sort(n, attr, dir)` | none | windowkind `1:6 unexpected token "n" (expected NumTok …)` | `sort(<n>, <attr>, <dir>)` | text |
| SPEC:179 | `raw()` | none | expr `1:5 unexpected token ")"` | `raw(attr)` | parses (expr) |
| SEM:93 | `any of`, `all of` (2 spans) | none | partkey `1:7 unexpected token "<EOF>"` | name the real operators: `has_any_*`/`contains_any_*`, `has_all_*`/`contains_all_*` (keyword-free, so no marker is needed) | inferred |
| errors:245 | `sort(n, …)` | `, …`→removed | windowkind `1:6 unexpected token "n"` | `sort(<n>, …)` | text |
| errors:131 | `and not …` | …→A1 | chainelem `1:11 unexpected token "<EOF>" (expected ForBound)` | `and not <Stream>[…] for <duration>` (as errors:26) | text; red only because of binding rule 6 |
| SPEC:38 | `timestamp '<ISO-8601>'` | none (placeholder inside a string) | grammar OK, then `LOWER E-TYPE-001 … not ISO-8601` | red only if the check lowers. Fix: treat `'<…>'` inside a string as unbindable, or drop the span (the next cell holds the real example) | |

### 3b. Grammar-valid, but no entry point exists

Each of these parses with a throwaway production (zz_entry_test, all `<nil>`).

| file:line | span | missing production |
|---|---|---|
| SEM:55 (2 spans) | `idle after D` | IdleClause. The alternative `time ts watermark bounded D idle after D` parses as timeclause. |
| errors:39, a **pinned Fix line** | `window cron('0 0 * * *', 'America/New_York')` | WindowClause (`'window' @@`) |
| errors:11, a **pinned Fix line** | `intent '<one sentence>'` | IntentClause, or treat `'<…>'` as unbindable |
| errors:116 | `(k, j) of S, (k2, j2) of T` | a bare key list. The alternative `partition by …` parses as partkeys. |
| errors:188 | `primary key (…)` | PrimaryKey, or write `primary key (<cols>)` as text |
| errors:35 (exempt today as keyword-only) | `emit on close` / `change` / `due` / `batch end` | EmitClause (all 4 parse with one) |

### 3c. Prose spans the literal rule catches

Every token in these is grammar, so `text` is red. None is a rule fragment.

**Type signatures:**
- SPEC:156 `duration / int`, `timestamp + timestamp`, `duration / duration`
- SPEC:277 `duration / int`
- SEM:13 `duration / int`
- errors:237 `timestamp + timestamp`

**Keyword or operator lists:**
- SPEC:134 `contains_ci contains_cs starts_with_* ends_with_*`
- SEM:15 `ms, sec, min, hour, day, week`

**Tuples, maths and flags:**
- SEM:11 `(stream, timestamp, attributes)`
- SEM:47 `wm_construct = max(previous wm_construct, min(input wms))`
- SEM:57 and SEM:128 `d = 2 min`
- SEM:61 `(event_time, stream id, per-stream ingestion sequence)`
- SEM:77 `text_invisibles=true`
- SEM:135 `(ts, stream id, seq)`
- SEM:190 `TTL 3 min < pattern bound 5 min`

**Quoted diagnostics:**
- errors:100 `part 2 of the partition key: …`
- errors:104 `to key it on both, write (k, j) of S, …`
- errors:116 `the partition keys of rule R have different numbers of parts: …`

**Others:**
- a lark tokenization note: SPEC:280 `stream S`
- a Go symbol: errors:119 `partition.NewRule`

What it takes to fix them:
- With R', 13 of the 23 drop out (the types/units/literals/text-ops refinement).
- The other 10 need one of these: un-backticking; splitting into single tokens; an unbindable character; a detector exclusion for dotted Go names (`^[a-z]+\.[A-Z]\w*$`) or quoted messages. Or the rule allows a reasoned prose escape.
- No mechanical position rule separates these 23 from the real errors. "Fails at the first token everywhere" would also exempt `first every 10 min`, which is approved correction 4.

### 3d. Documented counter-examples (`cep-invalid`)

| span | how it fails |
|---|---|
| SPEC:42 `select *`, errors:274 `select *` | fail at `*` |
| errors:62 `followed by` | fails |
| errors:209 `1.5 min`, `30 sec 5 min`, `5 min 5 min` | fail at **lowering** (E-DUR-002), not in the grammar, so this works only if the check lowers |
| SPEC:98 `(k) of S` (E-PART-005) | **parses**: the grammar cannot refuse it. As `cep-invalid` it would be red; as `text` it is red too, because every token is grammar. |
| errors:103 `k of S, j of S` (E-PART-004) | fails only because no key-list entry point exists; `partition by k of S, j of S ttl 1 hour` parses |

Validator-level counter-examples cannot be marked under this rule without a validator-backed kind or an exception. That makes 2 spans.

## 4. errors.md

- Rule-like: 66 spans.
- Literal `cep`: 20. Among them: `language cep/0.1;` ×3; `within 5 min`, `after match skip past last`, `for 5 min`, `late events drop` ×2, `5 min`, `1 hour 30 min` ×2, `250 ms`; `(k, j) of S` ×5, `k of S`; `attr is not null`, `a has_any_cs('x', 'y')`.
- `text`: 16. These are the `<duration>`, `<attr>`, `<Name>`, `<step>…`, `<key>` forms and `duration × float`, `is [not] null`.
- Exempt keyword-only: 13.
- `cep-invalid`: 6.
- RED: 11.

**Pinned Fix lines.** fixtext_test.go pins errors.md's Fix line verbatim to errors.go `fixes`, for every [parse] code and for E-TYPE-001, E-DUR-002, E-STR-002 and E-WIN-004. 21 rule-like spans sit on pinned Fix lines, on errors.md lines 7, 11, 15, 19, 23, 27, 31, 35, 39, 43, 47 and 272. So:
- A trailing `<!--cep:…-->` on those lines breaks TestFixTextsMatchTheCatalogue, unless the test strips markers.
- Changing those lines changes user-visible diagnostic text. Two of the red spans (errors:11 and :39) are on pinned lines.

## 5. Fences

SPEC:
- 21-25: trivia OK.
- 50-52: program OK.
- 60-71: program OK.
- 104-112: statements/rule OK.
- 117-125: body OK.
- 139-142: body OK.
- 83-96 (the template): LEX FAIL `frag:3:25 … "… ttl D]"`. It is unbindable, so text.
- 205-215 (the reserved list): fails. It is text, and a test already pins it.
- 226-253: Go.

None fails that needs correcting. SEMANTICS and errors.md have no fences.

## 6. Item 5: the whole-program incident on the real Parse

The program is progs/incident.cep: stream declarations, a partition, then `from every a = A[x == 1] -> and not B[x == 1] for 5 min`, then within, after match, late events, select, insert.

Today's code (cep-clean, c87942d, `PROGS=… go test ./internal/parser -run TestZZIncident -v`):
```
incident.cep:8:28: E-ORD-001: clauses are out of order: within comes before ->.
incident2.cep:8:45: E-ORD-001 … (two positive steps, then -> and not)
correct.cep => <nil>
raw participle: incident.cep:8:28: unexpected token "->" (expected "within" DurationOrParam ("after" "match" "skip" "past" "last") "late" "events" LateClause)
```
This confirms E-ORD-001 at the `->`: participle backtracks out of `ChainElem*`.

**The smallest change is a parseError branch, with no grammar change.** It goes in parse.go's switch, before the `cur.Value == "and" && expects("PosElem")` case:
```go
case cur.Value == "->" && tok(at+1).Value == "and" && tok(at+2).Value == "not":
    return mk(name, "E-PAT-005", tok(at+1).Pos, "the absence has a -> before it; write and not, with no ->")
```
Results:
```
incident.cep:8:31: E-PAT-005 …    incident2.cep:8:48: E-PAT-005 …
go test ./internal/parser/... -count=1 → ok, exit=0
TestInvalidCorpus: 28 of 28 subtests PASS (all codes kept)
go test ./internal/parser/... -count=20 → ok, exit=0
```
- No .cep file in the repo contains `-> and not`. The only other hit is a Go comment, internal/model/canonical_test.go:588.
- Observation: E-PAT-005's catalogue title and Fix ("pattern without a positive step"; "anchor the absence on a positive event…") do not describe this mistake. E-SYN-001 ("the message names the canonical form") may fit better. That choice is the owner's; this is only a note.

## 7. Item 6: map order in `clauseStart`

**By construction (from participle v2.1.4 error.go:51-58):** the message is `unexpected token %q (expected %s)`, so it holds exactly one `(expected `. A `Name: ` prefix from nodes.go:48 does not add another. The unexpected token is printed with %q, so a quote inside it is escaped and cannot fake `expected "w"`. `expects(w)` matches only when `"w"` comes straight after `expected ` or `expected (`, so at most one clause-start word can match.

**Empirical (cep-clean, zz_maporder_test.go, `MAPORDER=1`).** The input was 135 valid .cep files. The mutations deleted, swapped or duplicated each token, and deleted each clause-start word through to the next clause start:
```
files=135 mutants=54008 valid=422 oneHit=9966 multiHit=0 twoExpected=0
heads=map[after:224 emit:104 intent:4322 late:1383 ttl:2966 within:967]
```
I could not construct a message with two hits. `go test ./internal/parser/... -count=20` passes on pristine c87942d (exit 0) and with the branch (exit 0). The map order cannot change the code today. A sorted slice would still make that safe by construction rather than by participle's format (inferred, not tested).

## 8. Where this contradicts or extends the prior report

1. The prior report's "4 X fail" understates what lands red. With bindings and the machine rule:
   - 10 more real fragment errors (3a). SEM:178 `every D`, SEM:180 `first every D` and SEM:190 `not … for 5 min` become red only once bound. SPEC:75, :79, :170, :172 and :179 were "metasyntax" in the prior run; under the rule they are all-grammar, so red.
   - 6 or 7 entry-point gaps (3b).
   - 23 prose spans (3c).
2. Under binding, `Stream` → `Src`, so the prior run's 2 P failures (`from Stream[filter]`, `name = Stream[filter]`) now parse as `cep-meta`.
3. A named pointer (ForBound) changes participle's expected text and broke e-pat-004 until parseError was adjusted. The prior report called the change "a one-line change in lower.go".
4. errors.md's pinned Fix lines (fixtext_test.go) conflict with inline markers. This is new.
5. Prior counts were about 70 to 76 markers for SPEC and 65 for SEMANTICS. Measured under R: 69 + 62 + 53 = about 184 markers in the three files, plus 44 red spans to fix first.
