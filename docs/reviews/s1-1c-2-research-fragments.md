> **S1.1c-2 plan check research, 2026-11-11 — the rule fragments SPEC and SEMANTICS quote, and participle sub-grammars (R-F).** A read-only research run before the reviews, in a scratch clone of `s1-1c-2/integration` @ fac4e64 (code `main` @ fd274e4); its findings are folded into [the plan check](../sprints/s1-1c-2-plan-check.md) §0. Paths below are the session scratchpad's.

# S1.1c-2 research: the rule-fragment test (inventory and sub-grammar proof)

Base: scratch clone of /Users/ramith/code/cep at fac4e64 (main fd274e4 plus docs). Go go1.27.3 (pinned), participle v2.1.4.
Nothing was changed in the repository. All experiments are in the scratch clone.
Scratch: /private/tmp/claude-501/-Users-ramith-code-cep/bcf47ef3-aff6-461b-85d0-fea8dd101151/scratchpad/s1-1c-2-research/fragments/
(cep/ is the clone; research-run.txt is the run log; tools/spans.py extracts spans; tools/classify.py holds the classification; spans.jsonl lists all 646 spans.)

## F1. Fences

SPEC.md has 9 fenced blocks. The 5 that are grammar input all parse today.
| lines | what it is | tag to give it | result today |
|---|---|---|---|
| 21-25 | two comments (`-- a comment`, a block comment) | `cep-trivia`, or `text` | lexAll gives 0 code tokens and only trivia: OK. (Parse would refuse it with E-PRAG-001, "holds no statement".) |
| 50-52 | `language cep/0.1;` (a whole program) | `cep` | Parse: OK |
| 60-71 | a whole program (pragma, param, 2 streams, rule) | `cep` | Parse: OK |
| 83-96 | the rule template: `[ … ]`, `…`, `<body>`, `D`, inline comments | `text` | LEX FAIL `frag:3:25: invalid input text "… ttl D]"` |
| 104-112 | rule UnknownAppConsent, whose streams are not declared | `cep-rule` | RuleDecl plus lowering: OK |
| 117-125 | a pattern body with within / after match / late events | `cep-body` | Body plus lowering: OK |
| 139-142 | a plain body `from Prompts[...]` | `cep-body` | Body: OK |
| 205-215 | the reserved-word list | `text` (an existing test already keeps it equal to the NAME terminal) | n/a |
| 226-253 | Go builder sketch | `go` (already tagged) | n/a |

SYNTAX.md is not normative. Its header says: "**Historical** — language workstream, L3 draft of the surface, 2026-10-31; superseded by SPEC." It has 5 fences:
- 11-15: a program (pragma and param). `cep`. Parse: OK.
- 22-32: stream, table and annotated-stream statements. `cep-statements`. Statement+: OK.
- 41-53: the rule-spine template. `text`. LEX FAIL `"| windowed input…"`.
- 61-80: AgentKillSwitch with a double-quoted intent over two lines. LEX FAIL `frag:2:10: invalid input text "\"Sensitive read,..."`. This is historical drift: a string must end on its line.
- 95-104: DailyDigest. RuleDecl: OK. Its `late events allow 10 min` parses; E-LATE-002 comes from the validator.

Recommendation: keep SYNTAX.md out of scope (tag its fences `text`, or exclude the file).

## F2. Inline spans (outside fences)

There are 646 spans: SPEC 432, SEMANTICS 214. Spans were extracted by matching backtick runs, so double-backtick spans are handled. Single identifiers, keywords, codes and paths are left out. Categories:
- L: literal rule text.
- P: valid syntax whose names look like placeholders.
- N: a documented counter-example.
- X: looks literal but is SEMANTICS's L2 pseudo-syntax or a shorthand.

Each span below was parsed through the entry point for its kind (F3).

### L, 36 spans: all 36 parse
SPEC (23):
- :17 `from S` (body)
- :19 `-- to end of line`, `/* block */` (trivia)
- :35 `0.75` (expr)
- :37 `5 min`, `1 hour 30 min` (duration)
- :38 `timestamp '2026-01-01T10:00:00Z'` (expr)
- :54 and :279 `risk/2.5` (expr)
- :77, :183, :284 `time arrival` (time clause)
- :98 `partition by (src, dst) of Requests, (dst, src) of Responses` and `partition by (tenantId, userId) of Grants`. These are partition keys **without ttl**. No struct of that shape exists (Partition requires ttl). Both parse with an experimental PartitionKeys struct.
- :115 `after match skip past last` (clause)
- :127 `read.sessionId` (expr)
- :148 `destination not in Approved` and `coalesce(destination, 'unknown') not in Approved` (expr)
- :166, :284 `late events drop` (clause)
- :173 `batch()` (window kind)
- :266 `math:round(0.9)` (expr)
- :269 `count()` (expr)

SEMANTICS (13):
- :15 `1 hour 30 min`
- :34 `now()` (expr; the grammar accepts it, deploy refuses it)
- :35, :37 `time arrival`
- :120, :130 `within 5 min`
- :130 `for 5 min`
- :157 `cron('0 9 * * MON-FRI', 'America/New_York')`
- :159 `sort(2, score, desc)`
- :161 `delay(10 min)` (window kind)
- :184 `partition by (tenantId, userId) of Grants ttl 10 min` (Partition) and `output first every 5 min` (clause)
- :190 `3 min`

### P, 26 spans: 24 parse
SPEC (17):
- :75 `${name}`
- :98 `k of S`, `(k, j) of S`
- :102 `from Stream[filter]`: **FAIL**
- :127 `name = Stream[filter]`: **FAIL**, and `e = S`
- :135, :255 `${param}`
- :154 `x in T`
- :263 to :268 the function signatures: `coalesce(x, fallback)`, `minimum(a, b)`, `maximum(a, b)`, `abs(x)`, `math:round(x)`, `ifThenElse(cond, a, b)`
- :282, :283 `(k, j) of S`

The two failures give `unexpected token "Stream" (expected Source …)`. `Stream` is reserved in every letter case (SPEC §2.3), so these two are really metasyntax.

SEMANTICS (9):
- :21 `coalesce(x, 'unknown') not in T`
- :23 `x != 5`, `not (x == 5)`
- :93, :196 `${VAR}`
- :196 `${LIMIT}`
- :144 `e1.attr`, `chain(e1, e2)`
- :184 `(k, j) of S`

### N, 6 counter-examples: 5 fail as documented
- SPEC:17 and :280 `languagecep/0.1`: Parse gives `E-PRAG-001`.
- SPEC:42 `0 < n < 1`: `1:7 unexpected token "<"`.
- SPEC:42 `select *`: `1:8 unexpected token "*"`.
- SEM:27 `x <> 5`: `1:4 unexpected token ">"`.
- SPEC:98 `(k) of S` **parses**. The grammar accepts it; E-PART-005 comes from internal/validate, so a grammar-level fragment check cannot test it.

### X, 4 spans: all fail the grammar
These need a doc fix or a `text` marker.
- SEM:98 `any of ["ignore previous"]`: `1:5 unexpected token "of"`. The grammar spelling is `has_any_*(…)`.
- SEM:128 `not HumanApprovals for 5 min`: `1:1 unexpected token "not"`. This is incident-class: an absence with no `and` and no filter.
- SEM:136 `late: drop`: `1:5 unexpected token ":"`.
- SEM:180 `first every 10 min`: `1:1 unexpected token "first"`. The `output` is missing.

### Metasyntax that looks rule-like (to be excluded or marked `text`)
SPEC:
- :75 `param name type = literal;` (placeholders)
- :77 `stream Name (attr type [optional], …) time-clause;`, `time <attr> watermark bounded <duration>`, `time <attr> watermark ascending`, `idle after <duration>` (angle-bracket placeholders)
- :79 `table Name (attrs) primary key (cols);`
- :102 and :127 `from Stream[filter]`, `name = Stream[filter]` (reserved placeholder `Stream`)
- :102 and :181 `output first every D` (D is not a duration)
- :127 `(a = A[…] and b = B[…])` (ellipsis) and `and not Stream[filter] for D`
- :137 `raw(attr) <op> …`
- :146 and :171 `delay(D)`
- :148 `not(… in …)`
- :170 `cron(expr, tz)`
- :172 `sort(n, attr, dir)`
- :175 `within D`
- :183 and :285 `given table <Name> [ {row}, … ]` / `[rows]`
- :267 `cast(x, 'string'|'int'|'float'|'bool')` (alternation)
- :270 `chain(c1, c2, …)`
- :166 and :284 `late events allow` (incomplete, and refused since rev 3)
- :280 `stream S` (a lark tokenization note, not a statement)

SEMANTICS:
- :37 `time <attr> watermark …` (two spans)
- :49 and :188 `delay(D)`
- :55 `idle after D` (two spans)
- :69 `<expr> in <Table>`
- :93 `any of` / `all of` (L2 words)
- :110 `e1=A → e2=B[pred]` (a `→` arrow and placeholders)
- :114 and :117 `within D`, `for D`
- :124 `and not B[pred] for D` (the corrected incident sentence), `-> egress = …`, `and not HumanApprovals[…] for ${kill_window}` (ellipsis), and `→ not B for D` (the historical wrong form, quoted in the correction note; a lexer error)
- :140 `e1=A → e2=B within 5 min`
- :176 `output first every D`, :178 `every D`, :180 `first every D`
- :190 `not … for 5 min`

A crude keyword-and-shape detector flags 49 SPEC and 45 SEMANTICS spans outside the fragment set. Exempting keyword-only spans (`not in`, `idle after`, `->`, `and not`, `late events`, `given table`, `is [not] null`, `and or not`) leaves about 33 and 38. Those two numbers are inferred: I counted them by hand from the detector's list.

## F3. Participle sub-grammars

`participle.Build[T](participle.Lexer(definition{Definition: rules}), participle.UseLookahead(2))` succeeds (`err=<nil>`) for every struct tried: Program, Statement, RuleDecl, SelectClause, SelItem, Partition, PartKey, Body, PatternBody, ChainElem, PosElem, Step, Source, WindowKind, LateClause, Extra, Expr, Duration, DurationOrParam, TimeClause, StreamDecl. It also succeeds for an experimental embedded-struct production.

Fragments were fed through the same path Parse uses: `lexAll` → `lexer.Upgrade(&sliceLexer{…})` → `ParseFromLexer`. Trailing tokens are refused by default. Lookahead 2 was enough for every kind; no fragment misparsed.

Kind → entry point:
- program → Parse
- rule → RuleDecl
- statements → `Statement+`
- body → Body
- partition → Partition
- part key → PartKey
- step → Step
- expr → Expr
- duration → Duration
- time clause → TimeClause
- window kind → WindowKind
- select → SelectClause
- single absence → ChainElem

Kinds with no struct of their own:
- `within D`, `after match skip past last` and `late events drop` are fields of PatternBody. LateClause holds only what follows `late events`.
- `output first every D` is the RuleDecl.Rate field.
- `for D` is a field of ChainElem, inside an alternation.
- Partition keys without ttl have no struct.
- A pattern chain without its trailing clauses has no struct. PatternBody requires all three clauses: `every a = A within 5 min` gives `unexpected token "<EOF>" (expected ("after" "match" …)`.

The smallest honest entry point is to name these productions in grammar.go and have the real grammar use them. Test-only parallel structs would copy the keywords and keep passing if grammar.go changed, so they prove nothing.

This was proven in the clone. PatternBody was rewritten as embedded `PatternChain`, `WithinClause`, `AfterMatchClause` and `LateEventsClause`, with lower.go unchanged (the embedded fields keep their old names). The run gave `go vet ./internal/parser` clean, `go test ./internal/parser ./internal/...` all `ok`, and `exit: 0`. With those real structs as entry points:
`REAL WithinClause OK | AfterMatchClause OK | LateEventsClause OK | PatternChain OK | incident FAIL: frag:1:16: unexpected token "and" (expected PosElem) | within D FAIL … | empty FAIL`.

Two cases were not tested (inferred):
- RuleDecl.Rate is optional and ChainElem.For sits inside an alternation, so neither can be embedded. Each needs a named pointer (`Rate *RateClause parser:"@@?"`, `ForBound`) and a one-line change in lower.go.
- A PartitionKeys struct embedded in Partition. It is analogous to the tested embedding; the parallel test struct parsed both SPEC:98 spans.

Lowering:
- The lowerer methods take sub-trees, so a fragment can be lowered on its own. Proven: `30 min 1 hour` → `LOWER E-DUR-002: … units must be strictly descending and distinct`; `'bad \q escape'` → `LOWER E-STR-002`.
- Lowering only catches literal-level errors: the `l.fail` sites in lower.go raise E-WIN-004, E-TYPE-001, E-DUR-002 and E-STR-002.
- Semantic refusals (E-PART-005, E-LATE-002, names) live in internal/validate and need a whole app, so a fragment cannot reach them.

The incident class is grammar-level:
- `-> and not` → `unexpected token "and" (expected PosElem)`
- `→ not B for D` → lexer error `invalid input text "→ not B for D"`
- `not HumanApprovals for 5 min` → grammar error

The kind chosen affects the error:
- Through Body, `-> and not` fails at the `->` (`expected "within" …`) because participle backtracks out of `ChainElem*`. PatternChain or ChainElem reports the exact `and`.
- parseError's `case at == 0` maps any error at the first token to E-PRAG-001. Fragment errors should use participle's raw message, not parseError.

## F4. Today's state

Fences:
- SPEC: all 5 grammar-input fences parse (2 Parse, 1 RuleDecl, 2 Body), and the comment fence lexes clean.
- SYNTAX: 1 of 4 grammar-input fences fails (the multi-line string).

Inline (log `COUNTS map[L/ok:36 N/fail:5 N/ok:1 P/fail:2 P/ok:24 X/fail:4]`):
- L: 36 of 36 parse.
- P: 24 of 26 parse (the 2 `Stream[filter]` spans are really metasyntax).
- N: 5 of 6 fail as documented; `(k) of S` passes the grammar.
- X: all 4 fail.

The incident forms:
- `every a = A -> and not B[x == 1] for 5 min` (chain) → `FAIL: frag:1:16: unexpected token "and" (expected PosElem)`
- `-> and not B[pred] for 5 min` (ChainElem) → FAIL at `and`
- `every a = A[x == 1] -> b = A[y == 2] and not B[z == 3] for 5 min` → OK
- `and not B[pred] for 5 min` → OK
- the correct body with trailing clauses → OK

The incident span itself, from commit ae25f5d, was metasyntax: `` `… -> and not B[pred] for D` ``. With `…`→`every a = A`, `B[pred]`→`B[x == 1]` and `D`→`5 min` substituted, it fails at `and` as shown above. As written, though, it would have been marked `text` and never parsed. See risk 1.

A whole program with `-> and not` (pragma, partition, …, select, insert) → `E-ORD-001: clauses are out of order: within comes before ->`, at the `->`. That message is misleading. No file under docs/language/invalid/ or examples/ contains `-> and not` (grep). This is a should-fix candidate to forward; it is not this lane's work.

## F5. Marker convention

Rendered with GitHub's own renderer (`gh api markdown -f mode=gfm`):

(a) An HTML comment **before** the span works mid-line: `<p>(a) mid-line: the clause <code class="notranslate">within 5 min</code> bounds the match.</p>`. At the start of a line or list item it is **broken**: the comment opens an HTML block and the span comes out as raw backticks. Output: `` `from S` at the start of a line. `` and `<li>\n`x != 5` in a list item.\n</li>`. Spans that start a line include SEM:110 and SEM:124.

(a') An HTML comment **after** the span, `` `from S`<!--cep:body--> ``, is clean in all three positions:
- `<p><code class="notranslate">from S</code> at the start of a line.</p>`
- `<li><code class="notranslate">x != 5</code> in a list item.</li>`
- mid-line OK, and `<!--text-->` is also invisible.

(b) A kind prefix inside the backticks shows to the reader (`<code>cep:body from S</code>`) and breaks copy-paste.

(c) A link form renders as a hyperlink to a dead anchor (`<a href="#cep-body"><code>from S</code></a>`). It misleads readers.

Finding the markers: for (a'), a test matches a backtick run, the span, the same run, then `<!--(cep:[a-z-]+|cep-invalid:[a-z-]+|text)-->`, outside fences. The span extractor here handled all 646 spans.

Catching unmarked spans: a must-be-marked rule works with a rule-like detector (the F2 keyword-and-shape list, with keyword-only spans exempt). It is heuristic, so it will have some false positives.

Constraints on the kind names: no `--` inside the comment (CommonMark), and no space between the span and the comment.

Recommendation: (a'), the trailing HTML comment.

Markers needed:
- SPEC: 38 `cep:<kind>` (23 L and 15 P that parse), 5 `cep-invalid:<kind>` (4 checkable by the grammar), and about 33 `text` if the must-be-marked rule applies. About 76 in all.
- SEMANTICS: 22 `cep:<kind>`, 1 `cep-invalid`, 4 X spans (fix them or mark them `text`), and about 38 `text`. About 65 in all.
- Fragment markers alone come to 70 (43 SPEC + 27 SEMANTICS). Marking only the 36 L spans is the lighter choice.

## Sizing

- Fences: SPEC has 5 grammar-input fences out of 9, and all 5 pass. SYNTAX is historical.
- Inline: 72 rule-like literal or near-literal spans. 36 L, all passing. 26 P: 24 pass and 2 are really metasyntax. 6 N: 5 fail as documented and 1 needs the validator. 4 X fail today.
- Entry points: about 15 fragment kinds.
  - 10 have an existing struct: Program/Parse, RuleDecl, Statement, Body, Partition, PartKey, Step, Expr, Duration, TimeClause, WindowKind, SelectClause, ChainElem.
  - 7 need a new named production: PatternChain, WithinClause, AfterMatchClause, LateEventsClause and PartitionKeys (embedded, so lower.go is unchanged), plus RateClause and ForBound (named pointers, small lower.go edits).
- Failing today: 0 L. 4 X in SEMANTICS (:98, :128, :136, :180). 1 SYNTAX fence.

## Risks

1. **The incident itself was metasyntax** (`… -> and not B[pred] for D`, ae25f5d). A test that parses only literal fragments would not have caught it unless:
   - doc authors write such sentences as literal examples; or
   - metasyntax spans get a placeholder-binding check (fixed bindings, e.g. `D`→`5 min`, `pred`→`x == 1`, `…`→a step), which the substitution above proves would catch it.

   The owner rejected "a wrapping template (placeholders defeat it)". Whether binding placeholders inside the fragment is a different thing is the owner's call.
2. SEMANTICS's header says "every code-looking snippet is illustrative pseudo-syntax". That contradicts marking its spans as grammar fragments. The header needs an edit, and docs/language/** is a law file, so the lane needs explicit ownership.
3. The rule-like detector is heuristic. The must-be-marked rule costs about 70 `text` markers, and the detector will have misses and false hits.
4. Refactoring grammar.go touches the grammar mapping (ADR-0002's struct per production; lark/EBNF equivalence). The parser tests pass. Whether a gate check compares the grammar shape was not verified (inferred unknown).
5. Error mapping: the existing parseError maps an error at the first token to E-PRAG-001, so fragment errors need participle's raw message or a fragment-aware mapper.
6. Validator-level negatives (`(k) of S`) and validator-level refusals in positive fragments cannot be checked per fragment.
7. Diagnostic finding: a whole-program `-> and not` reports E-ORD-001 "within comes before ->", which is misleading, and no invalid-corpus file covers it. Forward it.
8. Fences must be found by their tag, not by line number. SYNTAX.md should be explicitly out of scope.
