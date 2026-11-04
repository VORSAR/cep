# S2.2b design note — architecture review

Reviewer: architecture (adversarial, pre-build, G1). Subject: `docs/sprints/s2-2b-design.md` at `5dd8ee2`. The reviewer did not write the note. Line references are to the tree at that commit.

### AR-1 blocker: A1 retires a pinned law, and the note's proof leaves that out

Evidence: `internal/model/canonical_test.go:493-533` (`TestMarshalCanonicalNoPositions`) fails on any struct field reachable from `App` whose name is or ends in `pos`/`line`/`col`/`offset`, and its doc says "Two models that differ only in source positions therefore cannot exist". `internal/model/model.go:3-4`: "no source positions (diagnostics keep positions in a side table of their own)". ADR-0003:10: "a field with no SPEC clause is forbidden". `MarshalCanonical`'s documented rule (model.go:338-339) is "every one of its fields, always". The note (§2) offers only "the 39 goldens stay byte-identical" as proof.
Failure: as written, A1 turns an existing guard test red. It also amends an accepted ADR and the canonical encoding's written rule, and none of that is in front of the owner. After A1, any `reflect.DeepEqual` on `App` silently compares positions. That covers parity (ADR-0003 §5), S6.2's parse→print→parse round trip (a reformatted file moves every token), and Check's copy tests. canonical_test.go:451 already does this.
Fix: put the full cost into the owner question: an annotation on ADR-0003 §1, the encoding rule's exception, and the rewritten guard test. The new test should say: positions only in fields of type `model.Pos`; no position key in the encoding; two Apps that differ only in `Pos` encode byte-identically, with a red fixture. Add a rule: App equality is canonical JSON only, and `reflect.DeepEqual` on App is banned outside model's own tests.

### AR-2 blocker: codes raised in the lowering carry no fix text, and the table test cannot see them

Evidence: `parse.go:82-83` sets `Fix: fixes[code]`. `errors.go:20-44` holds `[parse]` codes only, and `fixtext_test.go:41-45` fails on any non-`[parse]` entry. The lowering already raises the `[static]` codes E-TYPE-001 (lower.go:392,398,436) and E-DUR-002 (lower.go:414). The note adds E-STR-002 and the unit-order part of E-DUR-002. `tools/shape/main.go:22` already lists E-TYPE-001 as a lowering code, but note §3 says the validator raises "every other" code.
Failure: E-DUR-002 and E-STR-002 reach the user with `Fix: ""`. That breaks errors.md:3 ("every diagnostic carries … fixes") and exit criterion 1 ("fires … with its fix text"). For these files the static table test in `internal/validate` gets a `*parser.Error`, not a `Diagnostic`, and note §9 does not say how such a file is judged.
Fix: the parser's fix table also holds the lowering codes (one exported list, shared with `tools/shape`). `fixtext_test` pins them to errors.md's `[static]` text. The table test takes a `*parser.Error` as the file's single finding. Note §3 names E-TYPE-001's lowering triggers (out-of-range literal, non-UTC timestamp).

### AR-3 blocker: E-WIN-004 cannot be decided from the model (missed detection)

Evidence: `lower.go:166`: `w.SortN, _ = strconv.ParseInt(k.SortN, 10, 64)`, and the grammar takes any `@Number` (grammar.go:129; lexer.go:29). `sort(2.5, …)` becomes 0. `sort(99999999999999999999, …)` becomes `MaxInt64`, because ParseInt saturates and the error is dropped. Every other out-of-range integer literal is E-TYPE-001 (lower.go:396-399).
Failure: the overflow case passes with no finding and the window is effectively unbounded. In the non-integer case the message cannot name what was written. This is the note's own E-DUR-002 argument ("the model keeps only …, so … cannot be seen after lowering"), but here it is not applied.
Fix: the lowering refuses a non-integer or overflowing `n` (E-WIN-004 at the number). The validator keeps `n ≥ 1`. Add a test for each trigger.

### AR-4 should-fix: `expr.Error` gaining `Pos model.Pos` breaks the expr→model fence

Evidence: `tools/importcheck/rules.json` fences `internal/expr` → `internal/model` to `Expr*`, `Lit*`, `TextOperand`. `tools/archcheck/main.go:187-208` reports every `model.<Ident>` selector outside those prefixes, and `model.Pos` is one.
Failure: the design goes red at the gate, or the build quietly widens a fence the owner set on 2026-11-04 (a LAW Δ).
Fix: `expr.Error` gains `Line, Col int`, copied from `n.Pos.Line`/`n.Pos.Col`. A selector on a value is not fenced, and no rules.json change is needed. Say so in the note. Also name the plumbing: `compareNode`, `arithNode`, `literalNode` and the registry factories do not receive the node today (expr.go:140,146,104,221).

### AR-5 should-fix: the position principle does not decide positions, so QA and the build will disagree

Evidence: note §3 says a finding points at "the first token of the construct its fix changes — … the operator". But a node's `Pos` is its first token (ADR-0002:32; grammar.go:7). Binary nodes are left folds with no operator token (lower.go:253-333), so `a + 'x'` gives the position of `a`, not `+`. Several fixes *add* a construct that has no token yet: E-PART-001, E-PART-002, E-SEL-001 (missing attribute), E-TIME-001, E-IDLE-001.
Failure: QA writes positions "independently from the principle", the build writes them from node `Pos`, and the table test goes red on positions, not on behaviour.
Fix: replace the principle with a code → anchor table in the note, one line per reachable code, that both sides use. For example: E-PART-001 at `from`, E-SEL-001 at the `insert into` target, E-IDLE-001 at the stream's `time`. Where the anchor is an operator, binary `Expr` nodes need an `OpPos`.

### AR-6 should-fix: E-STR-002 positions in the lowering are not the string's

Evidence: `l.str` receives the rule's position for intent and `todo` (lower.go:121,139), the operator's `CmpTail.Pos` for text and set operands (lower.go:300,312; grammar.go:208), and `WindowKind.Pos` for cron strings (lower.go:163).
Failure: an unknown escape in an intent or a set-operand list is reported at `rule` or at the operator. That disagrees with any principle QA applies.
Fix: capture each string token with its own `Pos` (a small sub-struct), or report the escape's own column. Pin which one in the anchor table (AR-5).

### AR-7 should-fix: a failed param resolution reaches `expr` and fires E-PARAM-001 twice

Evidence: note §6 says "Resolution runs before typing, so `expr` never sees a param literal". But on the failure path (undeclared name, or an override that does not parse) the slot still holds `LitParam`, and `nodes.go:34-35` returns a second E-PARAM-001 ("reached the compiler unresolved").
Fix: an expression with an unresolved slot is not typed. Check skips `Build` for it and keeps only the resolution finding. Add a test: one undeclared `${x}` in a filter gives exactly one finding.

### AR-8 should-fix: collect-all has no suppression rule, so one defect gives many findings

Evidence: note §1 says "every finding", and §9 says "each file yields exactly one finding". No precedence rule is written. Cases where one defect fires two or more codes:
- a clockless source read by a window rule: E-TIME-001 + E-IDLE-001;
- a rule reading a derived stream: E-GRAPH-004 + E-IDLE-001 on that stream;
- a pattern rule with no partition: E-PART-001 + E-PART-002 per stream;
- `ttl 0 ms`: E-DUR-003 + E-TTL-002;
- `late events allow 0 ms`: E-LATE-002 + E-DUR-003;
- `!=` over an optional attribute inside `and not`: E-NULL-001 + E-NULL-002;
- an unknown `from` stream: E-NAME-001, then every attribute in its filter and select, then E-SEL-001.

Failure: QA's single-finding files go red, and users get cascades.
Fix: run the checks in phases (names → params → graph/time → partition → types/select). An entity a finding poisons (a stream, a rule, an expression) is skipped by later phases. Write a precedence list, one line per pair above, and add a test per pair.

### AR-9 should-fix: two definitions of "source", and A2 as written double-fires

Evidence: note §8 says "a stream with no producing rule is a source". errors.md:219 and SPEC §3.3 say a source "has a time clause". Under §8's definition, A2's E-GRAPH-002 case (time clause, inserted into, read by a rule) makes the stream derived, so the reader also gets E-GRAPH-004.
Fix: E-GRAPH-002, E-GRAPH-004 and E-IDLE-001 classify by the time clause. E-TIME-001, E-TIME-002 and E-GRAPH-003 classify by producers. State that a time-clause stream is never "derived" for E-GRAPH-004.

### AR-10 should-fix: overrides as source text need a second literal parser inside `internal/validate`

Evidence: `Options.Overrides map[string]string // "5 min", "'x'", "3"` (note §1). `internal/validate` may not import `internal/parser` (rules.json). The literal grammar (durations with integrality and unit order, escapes, numbers, timestamps) lives in lower.go:360-484.
Failure: a second literal parser that drifts from the language, so an override such as `1.5 min` or `'\q'` gets a different verdict from the same text in source. Three more things are undefined:
- the position of a failed override (it has no source position);
- an override naming no declared param: silently ignored, so the deploy runs on the default while the operator believes otherwise;
- iteration order over a Go map.

Fix: `Options.Overrides map[string]model.Literal`. The facade parses override text with a new `parser.ParseLiteral`, and the validator checks kind against the declared type. A failed or unknown override is E-PARAM-001 at the param's declaration, or 0:0 for an unknown name, and the message names the override. Iterate the keys sorted.

### AR-11 should-fix: the findings order is not total, and nearest-name ties are open

Evidence: note §1 says "sorted by line, column, code". A Go-API model puts every finding at 0:0 (note §1), so same-code findings tie. Candidate sources include maps: `Overrides`, and the expr registry `descriptors` (expr.go:203).
Failure: message order and suggested names vary from run to run. Ground rule 11 treats non-determinism as stop-the-line.
Fix: order by (line, col, code, msg) with a stable sort over deterministically generated findings. Nearest name: a stated distance (e.g. Damerau-Levenshtein ≤ 2, or ≤ ⌊len/3⌋), ties broken by declaration order then bytewise, no suggestion past the cutoff. Registry candidates come from a sorted `Names()`.

### AR-12 should-fix: `expr`'s E-NAME-001 cannot carry the nearest name

Evidence: errors.md:107 says "Message carries the nearest valid name". `expr.Error` is `{Code, Msg}` (expr.go:46-49). Its E-NAME-001 messages (expr.go:108,160,176,205,231) carry none. `Scope` has no way to list candidates (expr.go:36-43).
Failure: the validator can suggest only by parsing `Msg`, or not at all.
Fix: `expr.Error` gains structured `Name` and `What` (attribute | function | table | capture). The validator builds the suggestion from its own scope and the registry's names.

### AR-13 should-fix: E-TBL-001 and composite tables go through `expr` as "unknown table"

Evidence: `expr.go:158-161` and `174-177` treat `TableKey` ok=false as E-NAME-001 "unknown table". The note's scope (§5) says a composite key is E-TBL-001 at the `in`. It does not say what `TableKey` returns for one. A set operator's table operand must be "a one-column table" (model.go:300-301), and no code or check is designed for a two-column one.
Failure: a composite-key `in` reports E-NAME-001 (wrong code), or is accepted.
Fix: the validator walks each expression for `ExprIn` and set operands against composite or multi-column tables, reports E-TBL-001 (or the code the owner picks for set operands), and skips `Build` for that expression.

### AR-14 should-fix: E-REGEX-001 (card item 1, "pre-scan") is missing from the note

Evidence: the card's in-scope item 1 says "E-REGEX-001's pre-scan [5]". The note never names the code. errors.md:117-119 says "Message names the construct and its position".
Failure: the following are all undesigned: placement, position (inside the pattern string?), running after param resolution (a regex from `${p}`), and the overlap with E-STR-002 (`'\d+'` is an unknown escape before it is a regex, so the pre-scan only sees triple-quoted patterns).
Fix: add a section covering it, and the reason the scan must run on resolved operands.

### AR-15 should-fix: E-PRAG-002 must stop the check, not join collect-all

Evidence: errors.md:170-171: "later versions are refused, never half-interpreted". Note §1 makes Check collect-all.
Fix: Check returns E-PRAG-002 alone when `Language != "cep/0.1"`. Add a test that a `cep/0.2` file with a second defect gives one finding.

### AR-16 should-fix: the resolved copy must be deep, or Check writes into the caller's App

Evidence: note §1 says "The returned app is a copy". `App` is a value tree of slices (`Rules`, `Select`, `Args`), so a shallow copy shares them.
Failure: resolution overwrites the parsed, unresolved model. That model is what ADR-0003 §5(c) compares and what the reviewer read (ADR-0003:28). The loss is silent.
Fix: deep copy. Add a test that the input's canonical JSON is unchanged after a Check with overrides.

### AR-17 should-fix: A5's "reachable when a sibling adapter registers one" cannot happen

Evidence: ADR-0008:35 makes the vocabulary "a closed table in `internal/validate/connectors.go`". Nothing registers into it. L3 must not know source and sink internals (ARCHITECTURE.md:79). ADR-0012 §1 puts `Punctuates` on `plan.StreamNode`, and `internal/plan` may not import `internal/validate` (rules.json).
Failure: E-WIN-002 is permanently unreachable in v0.1, but the note implies a path. How the planner learns `Punctuates` (and a sink's append or changelog mode) is undefined.
Fix: state "unreachable in v0.1" and add an errors.md annotation like E-GRAPH-001's (errors.md:215-216). That is an owner text change. State that the planner re-derives these facts from the App plus the connectors at Deploy, or that the facade hands them over.

### AR-18 later: the facade seam has two error types, and collect-all starts only after a clean lowering

Evidence: `parser.Parse` returns one `*parser.Error`, first error only (lower.go:25-29). Check returns `[]Diagnostic`. The planner cannot import validate (rules.json).
Fix: write down that the facade maps both to one public diagnostic type, carrying `Lint`. Also that a file failing in the lowering reports one finding only. This is acceptable, but say it.

### AR-19 later: the param-slot list includes a slot the grammar does not have, and `IdleAfter` has no position

Evidence: note §6 lists `idle after` as a param duration slot. `TimeClause.Idle` is `*Duration` only (grammar.go:45), and `TimeClause.IdleAfter` is a plain int64 (model.go:73). E-DUR-003 for `idle after 0 ms` needs the duration's position, but A1's node list gives `TimeClause`'s position only.
Fix: drop `idle after` from §6 and give the idle duration its own position.

## A1–A5 verdicts

- **A1 — agree, with conditions.** Positions on model nodes is the simplest working design. It also gives S6.2's printer something to attach comments to, and lets the Go builder record call sites later. The side table's path format is the worse coupling. The conditions:
  - put the real cost in the owner question (AR-1);
  - keep `expr` inside its fence (AR-4);
  - App equality by canonical JSON only.
  
  The alternative I would offer the owner is not the path table. It is "positions on nodes, but the guard test rewritten, not deleted".
- **A2 — agree with the intent, disagree with the wording.** Choosing the code by whether the stream is used as a source is a sound tie-break. But it needs AR-9's split: time clause versus producer, and a time-clause stream never counts as derived for E-GRAPH-004. Without it the reader also gets E-GRAPH-004.
- **A3 — agree.** errors.md:113 calls it a lint. The corpus-clean test still holds lints to zero, and the only `!=` in the corpus is on a non-optional attribute (eval t08:18). Add the E-NULL-001-over-E-NULL-002 precedence (AR-8).
- **A4 — agree.** Either option changes the law. `test_imports` is the smaller code change if it is limited to `_test.go` files, holds only `internal/validate → internal/parser`, and comes with an archcheck red fixture. Do not allow a general wildcard.
- **A5 — agree on substance, disagree on the stated path.** All three built-ins mark batches (ADR-0008:38), so `punctuates: true`. But the code is unreachable in v0.1, not "until an adapter registers one" (AR-17). Test it as a unit test over a planted table entry, and annotate errors.md the way E-GRAPH-001 is annotated.
