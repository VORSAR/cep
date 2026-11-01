# Red-team review — SPEC.md, grammar.lark/ebnf, errors.md, corpus (pre-v0.1-freeze)

Adversarial review of the normative language artifacts: [SPEC.md](../SPEC.md), [grammar.lark](../grammar.lark), [grammar.ebnf](../grammar.ebnf), [errors.md](../errors.md), [examples/](../examples/), [invalid/](../invalid/), with [SEMANTICS.md](../SEMANTICS.md) as binding context. Baseline `grammar_check.py grammar.lark examples --invalid invalid --ambiguity` re-run: **PASS** — every claim below is about what the checker does *not* check. Grammar claims are proven by executable probes in [probes/run_probes.py](probes/run_probes.py) and [probes/run_probes2.py](probes/run_probes2.py) (lark 1.3.1, LALR, same grammar file; probe IDs P01–P60 below refer to those scripts). The items pinned 2026-10-31 in the "already fixed" round are not re-reported; where a finding is residual to one of those fixes it says so. Severity per ground rule 15: **blocker** = accepts wrong programs, rejects right ones, contradiction between normative artifacts, or a trap producing wrong detections; **should-fix** = misleading/missing with a workaround; **later** = polish. Findings only; no fixes applied.

## Findings table

| ID | Severity | Artifact · section | Claim (one line) |
|---|---|---|---|
| RS-01 | blocker | SPEC §2.1 vs grammar.lark/ebnf | Keywords are "matched case-insensitively" per SPEC; both grammars are case-sensitive and reject `FROM`/`SELECT` (P01), and accept `Select` as a plain identifier (P01b) |
| RS-02 | blocker | SPEC §2.4/§5.3, errors.md E-STR-002 vs examples/05, 06 | The exhaustive escape list makes the regex operands in two valid-corpus files (`\s`, `\.`) and SPEC's own §5.3 example E-STR-002 violations |
| RS-03 | blocker | grammar.lark vs grammar.ebnf vs SPEC §2.4 | lark accepts `1.5 min`, `1e3 ms`, and non-descending/repeated unit pairs; EBNF (`integer`) and SPEC (`<int> <unit>`, descending) forbid them — the "machine-checked equivalent" grammars are not equivalent (P02/P03) |
| RS-04 | blocker | grammar.lark NAME vs SPEC §2.3 + App. B | `for` (the §5.2 absence-bound keyword) is missing from the normative reserved-word lookahead: `stream for (…)`, attribute `for`, `select for` all parse with no E-KW-001 (P41) |
| RS-05 | blocker | SEMANTICS §1.1 × SPEC §2.5 × examples/01 | `not in` null semantics unpinned: under §1.1's membership-null-false the flagship's `destination not in ApprovedDestinations` (destination optional) silently misses null-destination egress **and** example 01's own expect block fails; under the `not(in)` reading it passes — two normative readings, opposite detections |
| RS-06 | blocker | examples/09 vs SPEC §6.8/§8 | Example 09's expect asserts `scaled: 0.9` while the rule selects `math:round(risk)` of 0.9 (= 1 under any rounding) — a valid-corpus file fails its own normative expect (E-EXP-001), so conformance §8(2) is unsatisfiable |
| RS-07 | blocker | SPEC §5.8 (E-SEL-002) vs examples/03, 04 | The pinned window-select rule (partition key, aggregate, or constant only) outlaws `select … score` (ex03, sort) and `select … body` (ex04, delay) — two valid-corpus files violate a static rule; "window rule" scope undefined |
| RS-08 | blocker | examples/01 vs SEMANTICS §10 / SPEC §4.1 / grammar | `url = '${IAM_WEBHOOK}'` relies on textual interpolation inside a string: annotations structurally reject param refs (P47), `IAM_WEBHOOK` is undeclared, and §10 pins substitution "token-level, never textual" — either the sink URL is the literal text `${IAM_WEBHOOK}` or the Tweaker contract is broken |
| RS-09 | should-fix | grammar vs SPEC §6.1/SEMANTICS §6.5 | `late events allow D` on a pattern parses; patterns are drop-only per both spec docs; no error code, no invalid file (P07) |
| RS-10 | should-fix | grammar vs SEMANTICS §8 | Illegal emit-mode × window-kind combos parse (`sort … emit on close`, `cron … emit on batch end`, `delay … late events allow`); §8 calls them deploy errors but no code exists (P08, P08b, P60) |
| RS-11 | should-fix | SPEC §2.5 "SQL verbatim" vs both grammars | No unary minus anywhere: `[n < -1]` and `param floor float = -1.0` are parse errors; negative values are inexpressible in filters, param defaults, sort thresholds and expect records (P04) |
| RS-12 | should-fix | SPEC §5.2/§5.5 vs grammar | Pure-absence patterns ("no heartbeat for 5 min") are grammatically impossible (first element must be positive, P05) but no SPEC sentence says so, no code, no invalid file — the single most likely CEP ask dies with a generic token error |
| RS-13 | should-fix | SPEC §5.2/§3.3 | Duplicate step names (`e1` twice) and capture names shadowing stream names parse; no uniqueness or scoping rule, no error code — `e1.attr` is ambiguous (P06, P06b) |
| RS-14 | should-fix | grammar expect_block, SPEC §6.8 vs examples/01 | Expect cannot state the defeat path: exactly one `given` stream, `then` requires ≥ 1 record (no "then nothing"), no null literal, and omitted-attribute semantics are undefined — ex01's then-record omits `evidence`, contradicting §6.8's "attribute-wise" equality (P18/P18b/P18c) |
| RS-15 | should-fix | SPEC §3.3/App. B, SEMANTICS §1/§3 | The built-in function catalog ("closed") is never enumerated — `coalesce count chain math:round cast convert ifThenElse` appear across docs with no signatures; `min` is unwritable as an aggregate (reserved as a unit) while `max(n)` parses (P22/P22b); `chain()` with zero args parses (P35) |
| RS-16 | should-fix | SPEC §5.6, SEMANTICS §1 | No expression typing rules and no type-error code: `5 has_ci 'x'` and `(a is null) in Tab` parse and no catalogued diagnostic covers them; `%` on floats, ordering of timestamps, equality on `object` all unstated (P24, P39) |
| RS-17 | should-fix | errors.md header/§7 vs invalid/ | E-PAT-004 is marked **[parse]** but has no counter-file, falsifying "every [parse] code has a counter-example"; conversely E-RES-001 is marked **[static]** yet its counter-file is rejected by the grammar, so the catalogued code can never be the one emitted |
| RS-18 | should-fix | SPEC App. C, App. E | Counts wrong: "34 codes: 14 parse-level, 20 static" — actual 38 (16 parse, 22 static); "12 invalid corpus files" — actual 16 |
| RS-19 | should-fix | examples/03, 04, 06 | Corpus teaches wrong shapes: ex03's intent says "per tenant" but partitions by `agentId` (no tenant attribute exists); ex04's "one summary per delivery batch" partitions by `recId`, making `count()` ≈ 1 per record; ex06 ships `todo` in the *valid* corpus although E-TODO-001 "always fails compile" |
| RS-20 | should-fix | SPEC §4.2 "inert" vs §6.1 | A single-stream pattern with an absence (`every e=S[…] and not S[…] for D`) stalls in live mode: `idle after` is pinned inert on single-stream rules and no construct watermark is defined for one input, so "a quiet stream never stalls detection" does not hold — residual after the admission/contribution fix |
| RS-21 | should-fix | SPEC §4.1 vs errors.md E-PARAM-001 | Param model incoherent: the grammar requires `= literal` on every param, yet E-PARAM-001 handles "unset" parameters and directs users to "set it in the deployment environment" — whether deployment silently overrides the reviewed default (Tweaker contract) is unstated |
| RS-22 | should-fix | checklist B1 sweep | Top LLM hallucinations die generically with no catalogued code or invalid file: SQL `=`/`<>`, IN-lists `a in ('x','y')`, `select *`, chained comparisons, wrong units (`5m`, `mins`), unterminated `/*`, and `language cep/9.9;` (unsupported version **parses**, no code) (P09/P10/P12/P13/P17/P28) |
| RS-23 | should-fix | errors.md E-STR-002 vs SPEC §2.4 | E-STR-002's fix says "use a triple-quoted **raw** string" but no normative text defines triple-quoted strings as raw; whether `\q`/`\s` are escape-checked inside `'''…'''` is unspecified (P49) — this decides what regex text the engine sees |
| RS-24 | should-fix | SPEC §6.3, errors.md E-WIN-003 | `sort(2.5, …)` parses in both grammars with no static integer/positivity rule (P14); the cron dialect is unpinned (field count, seconds, macros, DOW 0 vs 7) though E-WIN-003 claims deploy validation — two conforming engines can disagree on validity and buckets |
| RS-25 | should-fix | SPEC §3/§4.2 | Stream-graph legality unstated: a rule reading its own output (`from Loop … insert into Loop`), cycles through two rules, two rules inserting into one stream, and `insert into` a *source* stream (one with a time clause) all pass every catalogued check; watermark propagation on a cycle is undefined (P16, P16b) |
| RS-26 | later | errors.md, grammar.lark, grammar.ebnf | Hygiene drift: E-EXP-001 cites nonexistent "SPEC §6.9"; grammar.lark header says `examples/invalid/` and cites "S-PART-001"; EBNF never defines `pragma-id`; BACKTICKED admits `->`, spaces and `${…}` in names; a BOM-prefixed UTF-8 file is rejected with no spec statement (P20, P26) |
| RS-27 | later | SPEC §2.4/§4.4/§5.2 | Unstated accepted forms: `5min` without a space, zero durations (`for 0 ms`, `delay(0 ms)`, `output first every 0 ms`), filterless steps (`e1 = S`), multiple `expect` blocks, and two sibling and-groups vs §5.2's "**one** parenthesized … pair" (P11/P27/P50/P57/P59) |
| RS-28 | later | SPEC §4.2 note / checklist D | Hidden dependency: a rule's detection latency, lateness drops and absence fire-times change when a consumed stream's watermark `d` or `idle after` changes — invisible from the rule text; nothing requires surfacing consumed time clauses at review time |

Counts: **8 blocker · 17 should-fix · 3 later.**

---

## Proofs

### RS-01 (blocker) — keyword case-insensitivity is specified but not implemented

SPEC §2.1 (normative): "keywords are written in lowercase in canonical form and **matched case-insensitively**; identifiers are case-sensitive."

grammar.lark uses case-sensitive anonymous terminals (`"rule"`, `"from"`, …) and a case-sensitive NAME lookahead; grammar.ebnf likewise. Probe P01:

```
language cep/0.1;
stream S (a string, n float, ts timestamp) time ts watermark bounded 1 min idle after 1 min;
stream T (a string, ts timestamp);
RULE X
  INTENT 'x'
  FROM S[a == 'x']
  SELECT a
  INSERT INTO T;
```

→ `UnexpectedToken: Token('NAME', 'RULE') at line 4` — the grammar **rejects a program SPEC §2.1 accepts**. Worse, P01b: `stream Select (a string, ts timestamp) …` **parses** — `Select` lexes as an ordinary NAME, so under §2.1's case-insensitive keyword matching the same spelling is simultaneously a keyword and an unquoted identifier. The combination "case-insensitive keywords + case-sensitive identifiers + reserved-word exclusion" is incoherent as written: either §2.1 drops the case-insensitivity claim (and the invariant "an identifier spelled like a keyword must be backtick-quoted" is weakened to exact-lowercase spellings), or both grammars and the E-KW-001 check must become case-insensitive. Conformance §8(1) ("accepts exactly the programs the normative grammar accepts") currently contradicts §2.1 directly.

### RS-02 (blocker) — the exhaustive escape list outlaws the corpus's own regexes

SPEC §2.4 (normative): "escapes, exhaustively: `\\ \' \" \n \t \r \uXXXX` (4 hex digits) — **any other `\x` is `E-STR-002`**." errors.md E-STR-002: "the escape set is exhaustive."

Valid corpus, [examples/05-text-operators.cep](../examples/05-text-operators.cep):

```
or text contains_match_ci 'wire\s+transfer'
```

`\s` is not in the list → E-STR-002. [examples/06-once-and-group.cep](../examples/06-once-and-group.cep):

```
version matches_cs 'v[0-9]+\.[0-9]+'
```

`\.` is not in the list → E-STR-002. SPEC §5.3's own inline example repeats `'wire\s+transfer'`. Two normative-corpus files (and the spec's example) violate a normative static rule — and the resolution is semantics-bearing: if authors must write `\\s`, every existing regex in the corpus matches the wrong text; if regex operands are exempt, "exhaustively" is false and the exemption (which operand positions? detected how, pre-type-check?) must be specified. Related to, but not covered by, the already-fixed escape-enumeration item: the enumeration itself now contradicts the corpus.

### RS-03 (blocker) — duration lexing: lark ≠ EBNF ≠ SPEC

SPEC §2.4: duration = "one or more `<int> <unit>` pairs, units … **descending**". grammar.ebnf: `duration = integer, dur-unit, { integer, dur-unit } ;  (* descending units *)`. grammar.lark: `dur_part: NUMBER DUR_UNIT` where `NUMBER: /[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?/`.

Probes (all **parse OK** under grammar.lark):

- P02 `within 1.5 min` — fractional, forbidden by EBNF/SPEC.
- P02b `within 1e3 ms` — exponent duration.
- P03 `within 30 sec 5 min` — ascending units.
- P03b `within 5 min 5 min` — repeated unit.

Four programs the normative EBNF rejects and the machine-checked lark accepts, falsifying SPEC §0/App. A's "machine-checked equivalent" claim (grammar_check only runs lark against the corpus — equivalence is asserted, not checked, for anything outside the corpus). No static code exists for non-descending/duplicate units either, so even a "fix statically" resolution needs a new code + invalid files. What does `1.5 min` mean if accepted — 90 sec, or error? Nothing says.

### RS-04 (blocker) — `for` is a keyword but not a reserved word

grammar.lark line 180's NAME lookahead ("This lookahead list **IS** the normative reserved-word list (SPEC Appendix B mirrors it)") enumerates every keyword **except `for`**, which is the absence-bound keyword of §5.2 (`and not S[…] for 5 min`, E-PAT-004). SPEC Appendix B: "all keywords of §§2–5" — `for` is a keyword of §5. Probes (all parse):

```
stream for (a string, ts timestamp) time ts watermark bounded 1 min;     -- P41
stream S2 (for string, ts timestamp) time ts watermark bounded 1 min;    -- P41b
from S[for == 'x'] … select for …                                        -- P41c
```

So the two halves of Appendix B contradict each other (the prose says reserved; the pointer-to-lark says not), E-KW-001 never fires for `for`, and a program using `for` as an identifier is accepted by one normative artifact and forbidden by another. EBNF defers to the lark list, inheriting the hole.

### RS-05 (blocker) — `not in` over a nullable attribute: two readings, opposite flagship detections, and the corpus expect decides neither

SEMANTICS §1.1 (binding): "Any comparison (`== != < <= > >=`), **membership (`in`)**, or text operator with a null operand evaluates **false**." SPEC §2.5 treats "`in` / `not in` (table membership)" as one operator family; the grammar makes `not_in_table` an atomic node. Nothing states whether `x not in T` with null `x` is **false** (atomic membership operator, per §1.1's pattern) or **true** (sugar for `not (x in T)`, per §1.1's documented `not` asymmetry: "`x != 5` is false but `not (x == 5)` is true").

This is not hypothetical — it is the flagship. [examples/01-agent-kill-switch.cep](../examples/01-agent-kill-switch.cep) declares `destination string optional` and gates the egress step on:

```
-> egress = AgentActions[sessionId == read.sessionId
                         and destination not in ApprovedDestinations]
```

- Atomic reading: a null-destination egress event evaluates **false** → the kill switch silently never fires on events missing `destination` — a missed detection of exactly the §1.1-rationale kind ("silently excluding nulls is a missed-detection bug in a security engine").
- Negation reading: null → `in` false → `not in` **true** → fires.

The corpus's own expect proves the ambiguity is live: the given `'send'` event carries **no `destination` field at all**, yet the then-list asserts the alert fires at 10:05. If an omitted optional attribute is null (also unstated — see RS-14) the expect **passes only under the negation reading** and **fails (E-EXP-001) under the atomic reading** that SEMANTICS §1.1's wording implies. Two binding artifacts, opposite outcomes, on the flagship rule. E-NULL-002 lints only `!=`; E-NULL-001 covers only absence predicates — no guard exists for `not in` in positive steps. (Distinct from the fixed absence-side null item: this is the positive-step/`not in` axis plus the corpus contradiction.)

### RS-06 (blocker) — example 09's expect asserts a value its own select cannot produce

[examples/09-params-and-quoting.cep](../examples/09-params-and-quoting.cep):

```
expect given `agent-events` [ {agentId: 'a1', risk: 0.9, env: 'prod', ts: timestamp '2026-01-01T00:00:01Z'} ]
       then RiskAlerts [ {agentId: 'a1', scaled: 0.9, ts: timestamp '2026-01-01T00:00:01Z'} ]
select agentId, math:round(risk) as scaled
```

`math:round(0.9)` is 1 under every rounding convention (half-up, half-even, floor/ceil give 0 or 1 — never 0.9). The asserted `scaled: 0.9` is unreachable, so under SPEC §6.8 ("the rule's outputs must equal the then-list") this valid-corpus file fails with E-EXP-001, and conformance §8(2) ("reproduces the corpus expect outputs under playback byte-identically") is unsatisfiable. Either the select drops `math:round`, the expect asserts `1.0`, or `math:round` means something no document defines (see RS-15: the function catalog is unspecified, so this cannot even be adjudicated).

### RS-07 (blocker) — the pinned window-select rule outlaws two valid-corpus files

SPEC §5.8 (pinned this round): "In a **window rule**, a select item must be the partition key, an aggregate over the window, or a constant (`E-SEL-002`…)."

- [examples/03-sort-leaderboard.cep](../examples/03-sort-leaderboard.cep): window rule (`window sort(2, score, desc)`), `select agentId, score` — `score` is a per-event attribute, not the partition key (`agentId`), not an aggregate, not a constant → E-SEL-002.
- [examples/04-delay-and-batch.cep](../examples/04-delay-and-batch.cep), rule `DebounceNotifications`: window rule (`window delay(10 min)`), `select userId, body` — `body` likewise → E-SEL-002.

Yet per-row output is the entire point of sort (changelog rows) and delay (re-timed events). As written the rule can only be about *aggregating* windows (cron, batch), but "window rule" is undefined and the §5.4 summary lists E-SEL-002 unconditionally. Normative contradiction: the grammar+corpus accept what §5.8's static rule forbids. Residual of the fixed "window select legality" item — the fix's wording contradicts the corpus it froze alongside.

### RS-08 (blocker) — `'${IAM_WEBHOOK}'`: textual interpolation that the binding semantics forbid

SEMANTICS §10 (binding): "`${VAR}` substitution is **token-level, never textual**: … A value can therefore never alter rule structure — reviewed rule text fully determines what runs (the Tweaker contract)." SPEC §4.1: substitution goes "into one literal slot after parsing."

grammar.lark: `ann_item: cname "=" literal` — `literal` has no `param_ref` alternative, and probe P47 confirms `@sink(url = ${WEBHOOK})` is a parse error. The only way [examples/01](../examples/01-agent-kill-switch.cep) can parameterize its sink is what it does:

```
@sink(type = 'http', url = '${IAM_WEBHOOK}', @map(type = 'json'))
```

— a `${…}` **inside a string literal** (P34: parses as a plain STRING). Now either (a) strings are inert, the sink URL is the literal nine characters `${IAM_WEBHOOK}` and the flagship example is broken as shipped; or (b) strings interpolate textually, directly contradicting §10's "never textual" and opening value-driven structure changes (a deployment value becomes part of a URL/regex/keyword-set with no slot type-check — E-PARAM-001 cannot even apply, since `IAM_WEBHOOK` is not a declared `param` and no doc says annotation `${…}` resolves against the param namespace or the deployment environment). Neither reading is written down. The spec must pin: where `${…}` is structural (grammar slots), whether `${…}` inside any string is ever substituted (recommend: never, with an annotation-value `param_ref` alternative added instead), and which namespace annotation references resolve against.

### RS-09 (should-fix) — `late events allow` on patterns: parses, forbidden, uncoded

SPEC §6.1: late events on patterns "are dropped and counted"; SEMANTICS §6.5: "`late: drop` is the **only** MVP value for patterns." Grammar: `late_clause` is shared by pattern and windowed bodies, so probe P07 —

```
from every e1 = S[a == 'x'] -> e2 = S[a == 'y']
within 5 min
after match skip past last
late events allow 10 min
```

— **parses**. No code (an E-LATE-002) and no invalid file cover it, although errors.md's charter says every static rule has a stable code. A generator that has just seen `late events allow 10 min` legal on the cron example will emit it on patterns; today the rejection text is implementation-folklore.

### RS-10 (should-fix) — emit-mode × window-kind mismatches have no code

SEMANTICS §8: "MVP legal combinations (**anything else is a deploy error**)". The grammar accepts any `emit_mode` with any `window_kind`: P08 `window sort(2, n, desc) emit on close`, P60 `window cron(…) emit on batch end`, P08b `window delay(10 min) … late events allow 5 min` all parse. errors.md has only E-EMIT-001 (missing mode). E-EMIT-001's own fix line ("add **the kind's mode**: close (cron) | change (sort) | due (delay) | batch end (batch)") shows the mode is 1:1 with the kind — making the mismatch the single most likely wrong edit, with no catalogued diagnostic and no invalid files. (If the mode is truly determined by the kind, the cleaner v0.1 fix is noting that redundancy explicitly; either way a code is needed.)

### RS-11 (should-fix) — no unary minus anywhere; "SQL verbatim" misleads

SPEC §2.5: "Operators: **SQL verbatim** — `+ - * / %` …". SQL's `-` includes unary minus; neither grammar has one (`PLUS_MINUS` is binary-only; `NUMBER` is unsigned; EBNF `sum = [sum, ("+"|"-")], term`). Probes:

- P04 `from S[n < -1]` → `UnexpectedToken PLUS_MINUS '-'`.
- P04b `param floor float = -1.0;` → same.

Negative thresholds, deltas, and expect-record values are inexpressible (literal → unsigned number also rules them out of `expect` records and sort/param slots). Either add unary minus (or signed numeric literals) to both grammars and §2.4, or state the exclusion loudly in §2.4/§2.5 and catalog the diagnostic — and stop saying "SQL verbatim" (see also RS-22: SQL's `=`/`<>` are likewise not verbatim).

### RS-12 (should-fix) — absence-only patterns: inexpressible, but nowhere stated

Grammar: `body: "from" restart first_elem chain_elem* …` with `first_elem: pos_elem` — an absence can be neither the first nor the only element. Probes P05 (`from every and not R[a == 'y'] for 5 min …`) and P05b (absence before the first positive) both fail to parse. Correct per the design (absences need an anchor), but:

- No SPEC sentence states "every pattern has at least one positive step; a pure absence ('no heartbeat for D') is not expressible in v0.1 — anchor it on a positive event or use a cron window." §5.5's guidance ("a pattern whose only positive step is the anchor…") *presupposes* the rule without stating it.
- The heartbeat/dead-man rule is among the most common CEP requests; a generator will emit it and get an uncatalogued `UnexpectedToken`, with no invalid-corpus file to teach the repair loop.

Needs: a normative sentence in §5.2, a catalogued code (or an E-SYN-001 message binding), and an invalid file.

### RS-13 (should-fix) — step-name uniqueness and capture/stream scoping are unspecified

P06: `from every e1 = S[a == 'x'] -> e1 = S[a == 'y']` parses; after it, `select e1.a` has two candidate bindings. No SPEC rule requires unique capture names and no error code covers the collision. P06b: `from every R = S[…] -> e1 = S[a == R.a]` parses — the capture `R` shadows stream `R`; `attr_ref: cname "." cname` cannot distinguish capture.attr from a (future) stream-qualified name, and §3.3's closed-catalog name resolution never defines the pattern-local scope, shadowing order, or whether a capture may collide with a stream/table/param name. One sentence plus one code (e.g. E-PAT-005 duplicate capture; E-NAME-002 shadowing) and two invalid files close it.

### RS-14 (should-fix) — `expect` cannot express the defeat path; record matching is underspecified

Grammar: `expect_block: "given" cname "[" record ("," record)* "]" "then" cname "[" record ("," record)* "]"` — exactly one given stream, one then stream, each with **at least one** record. Probes: P18 `then T [ ]` → parse error (asserting *no output* is impossible); P18b two `given` sections → parse error (events for the approval stream cannot be injected, so the kill-switch's defining negative case — approval arrives, no alert — is untestable); P18c `{a: null, …}` → parse error (no null literal; an optional attribute cannot be explicitly null).

Underspecification on top: SPEC §6.8 says outputs "must equal the then-list (attribute-wise…)", yet ex01's then-record omits the target's `evidence` attribute entirely — if "attribute-wise" means all attributes, the flagship expect fails; if omitted attributes are unchecked, §6.8 must say so (and say what an omitted *given* attribute means — null? error? — which RS-05 shows is detection-relevant). For a language whose §6.8 declares expects "a safety net… rules shipped in packs must carry one," the net cannot catch the one failure mode the flagship exists to prevent.

### RS-15 (should-fix) — the "closed" function catalog is not in any normative artifact

§3.3 resolves names against "closed catalogs: … built-in functions"; E-NAME-001 depends on the catalog's contents; conformance §8 requires implementations to agree. But no artifact lists the functions. Observed across the docs: `coalesce` (§5.1), `count()` (ex02/04), `chain(…)` (§6.6, ex01/08), `math:round` (ex09), `cast`/`convert` (SEMANTICS §1), `ifThenElse` (SEMANTICS §1.1). Consequences proven:

- P22: `select min(n) as lo` is a **parse error** (`min` is reserved as a duration unit) while P22b `max(n)` parses — the most common SQL aggregate pair is asymmetrically broken, the backtick workaround `` `min`(n) `` is nowhere documented, and no diagnostic explains it.
- P35: `chain()` with zero args parses; arity/argument kinds (capture names only?) are unspecified.
- RS-06 is unadjudicable because `math:round`'s semantics exist nowhere.

v0.1 needs a normative Appendix: function names (and namespaces), signatures, null behavior, and which are aggregate-only (`count`) vs scalar vs pattern-only (`chain`).

### RS-16 (should-fix) — no typing rules, no type-error code

The grammar is untyped and no SPEC section gives expression typing judgments; errors.md has no E-TYPE code. Proven affordances: P24 `from S[5 has_ci 'x']` parses (text operator on a number — the brief's canonical case); P39 `from S[a is null in Tab]` parses (a boolean fed to table membership — a legal consequence of the pinned precedence, never type-rejected). Unstated: `%` on floats (SEMANTICS §1 says only "int/float as usual"), `<` on timestamps/strings/bools, `==` on `object` (declared "opaque; only passed through or null-checked" — so is `==` an error?), duration comparison `5 min < 1 hour`. §5.6 routes *runtime* errors per-rule, but static type errors have no catalogued code, although errors.md promises specific diagnostics for everything the validator rejects.

### RS-17 (should-fix) — the [parse]/[static] bookkeeping is false in both directions

errors.md header: "Codes marked **[parse]** are enforced by the grammar itself (**each has a counter-example in [invalid/](../invalid/)**)"; SPEC §7 repeats it. Count: 16 [parse] codes, but invalid/ holds counter-files for only 15 of them — **E-PAT-004 (absence without `for`) has no invalid file**. Conversely **E-RES-001 is marked [static]** ("deploy-time checks after parsing") yet its counter-file `e-res-001-group-by.cep` is rejected *by the grammar* (the checker log shows "[invalid] rejected by grammar"; `group` cannot lex as NAME) — so the pipeline can never reach the deploy-time check that owns the code, and the diagnostic actually emitted for the counter-example is not the catalogued one. Reserved-word misuse needs a defined parse-level story (which code does `group by` actually produce?), and E-PAT-004 needs its file.

### RS-18 (should-fix) — Appendix counts are wrong

SPEC Appendix C: "34 codes: 14 parse-level, 20 static." Actual errors.md census: **38 codes — 16 parse** (PRAG-001, INT-001, PAT-001..004, LATE-001, EMIT-001, WIN-001, TTL-001, DUR-001, TXT-001, STR-001, ORD-001, SYN-001, KW-001) **+ 22 static** (TIME-001/002, IDLE-001, PART-001/002, TTL-002, NAME-001, NULL-001/002, REGEX-001, TXT-002, SINK-001, PARAM-001, TODO-001, EXP-001, STR-002, SEL-001/002, TBL-001, WIN-002/003, RES-001). Appendix E: "9 valid / **12 invalid** corpus files" — invalid/ holds **16**. All three numbers in the freeze record are stale; for a spec that sells reviewability-by-counting, the counts must be regenerated, not hand-maintained.

### RS-19 (should-fix) — the corpus teaches wrong shapes

The corpus is normative (§0) and is the generators' few-shot set; three files mislead:

- **ex03**: `intent 'Keep the two riskiest agents per tenant; emit every change.'` but `partition by agentId of RiskScores` over a stream with **no tenant attribute** — the rule actually keeps each agent's own top-2 *scores*. Intent and body describe different detections; a reader using intent as the contract (the stated reader model) is deceived.
- **ex04** `SummarizeLoads`: intent "One summary event per source delivery batch", but `partition by recId of LoadedRecords` makes each record its own partition — `count()` per batch *per recId* ≈ 1, every time. The honest content (per-batch global aggregation is inexpressible while global partitions are reserved) is hidden behind a key choice that silently degenerates. (The batch×partition *spec wording* was fixed this round; the example still demonstrates the degenerate shape.)
- **ex06** carries `todo '…'` — per SPEC §4.4 a clause that "always fails compile" (E-TODO-001) — inside the *valid* corpus. So "valid" means parse-only, yet §8(2) requires corpus playback reproduction, which ex06 can never participate in; and generators learn that finished rules may ship `todo`.

### RS-20 (should-fix) — single-input absence patterns stall under "inert" idleness

SPEC §4.2 (pinned wording): `idle after` "on a stream consumed only by single-stream rules … is legal but **inert**." SPEC §6.1: idleness "raise[s] only its *contribution* to the min" — defined only for multi-stream rules — and "a quiet stream never stalls detection."

A single-stream pattern with an absence is legal (probe family P15; e.g. `from every e1 = S[sev == 'hi'] and not S[sev == 'ok'] for 5 min …`). Its deadline fires at `wm ≥ t_first + 5 min`, but the only stated watermark for a one-input construct is the stream's own `max(event time) − d`, which **freezes when the stream goes quiet** — precisely the absence scenario. With idle pinned inert there, nothing advances the clock: the alert waits for the next unrelated event or end-of-stream, i.e. unbounded live detection latency on the construct whose whole purpose is detecting silence. Residual after the admission/contribution fix (which addressed the multi-stream min): either define a construct watermark for single-input patterns to which idleness contributes (making §4.2's "inert" false), or state the stall as a scoped limitation and lint absence-bearing single-stream rules whose stream lacks `idle after`.

### RS-21 (should-fix) — the param lifecycle contradicts itself

Grammar (both): `param_decl: "param" cname type "=" literal ";"` — a default is mandatory. errors.md E-PARAM-001: "**Unset** or ill-typed parameter … Fix: **set it in the deployment environment**, or correct the type." If every param carries a declared literal, "unset" is unreachable — unless deployment values override declared defaults, which no artifact states. If overrides exist, SEMANTICS §10's Tweaker contract ("reviewed rule text fully determines what runs") is false as experienced by a reviewer reading `param kill_window duration = 5 min` while deployment runs 5 hours; if they don't, E-PARAM-001's first clause and fix are dead text. Pin: default vs deployment precedence, whether overrides are visible/recorded at deploy, and whether `= literal` is a default or the value.

### RS-22 (should-fix) — B1 sweep: the likeliest LLM emissions die with generic errors

Per checklist B1, the most plausible invalid variants per construct were parsed; those below are **uncaught by any catalogued code and have no invalid-corpus file** (all die as bare `UnexpectedToken`/`UnexpectedCharacters`, exactly the "unexpected token" outcome the error catalogue exists to prevent):

| Emission | Probe | Note |
|---|---|---|
| `from S[a = 'x']` (SQL equality) | P12 | §2.5 claims "SQL verbatim" yet `=`/`<>` are not operators; the message should name `==` (E-SYN-001 candidates, but nothing binds them) |
| `a in ('x', 'y')` (SQL IN-list) | P10 | §2.5's "(table membership)" hint is the only trace; IN-lists are the single most common SQL habit |
| `select *` | P09 | nothing anywhere says projection is explicit-only |
| `0 < n < 1` (chained comparison) | P13 | correctly rejected, silently |
| `within 5m` / `5 mins` / `1h` | — | unknown units are NAMEs; E-DUR-001 covers only bare numbers |
| unterminated `/* …` | P28 | swallows the file tail; no code, no resync possible |
| `language cep/9.9;` | P17 | **parses** — no code exists for an unsupported version; E-PRAG-001 covers only absence. A v0.1 engine's required behavior on cep/0.2 is unstated |

Each deserves either a catalogued code with a canonical-form message (the E-SYN-001 mechanism exists for exactly this) plus an invalid file, or an explicit SPEC sentence designed to land in generator context windows.

### RS-23 (should-fix) — "triple-quoted raw string" is an undefined term

errors.md E-STR-002 Fix: "use a listed escape, **or a triple-quoted raw string**." SPEC §2.4 defines triple-quoted strings only as "may span lines and contain quotes" — nothing about escape processing. The lark terminal `/'''(.|\n)*?'''/` does no escape validation (P49: `intent '''uses \q escape'''` parses), but whether E-STR-002 (a *static* check) applies inside triple quotes, and whether `\n` inside `'''…'''` is two characters or one, is stated nowhere. This is load-bearing: the sanctioned escape-free home for regexes (the natural fix for RS-02) only works if triple-quoted strings are normatively raw. One sentence in §2.4 ("triple-quoted strings are raw: backslash has no special meaning and E-STR-002 does not apply") settles it.

### RS-24 (should-fix) — sort-N and the cron dialect are unpinned

- Both grammars type sort's first argument as `number`: P14 `window sort(2.5, n, desc)` parses; `sort(0, …)` likewise. No static rule or code requires a positive integer N (E-WIN-003 is cron-only). What `sort(0, …)` or `sort(2.5, …)` means is undefined.
- E-WIN-003 says "expr must be a valid cron schedule" — valid per **which dialect**? Field count (5 vs 6-with-seconds vs 7), names/macros (`@daily`, `MON`), day-of-week 0-vs-7, `L/W/#` extensions are all unstated, yet bucket boundaries feed detection timing, the §5.5 TTL extent check, and §8's byte-identical replay. ex02 uses `MON-FRI` names — so *some* name support is normative by corpus, dialect still unnamed. Pin the dialect (and version, like the Unicode tables of SEMANTICS §5.1) or two conforming engines will disagree on both validity and semantics.

### RS-25 (should-fix) — stream-graph legality: cycles, fan-in, and inserting into sources

All of the following pass the grammar and every catalogued static rule:

- P16: `rule X … from Loop[a == 'x'] … insert into Loop;` — a rule consuming its own output. Watermark punctuations on `Loop` (§6.2: forwarded only after emitting everything due) are circularly defined; nothing forbids the cycle or defines its semantics. Same for a two-rule cycle (A→B, B→A).
- Two rules inserting into one derived stream: legal by silence; the merged stream's propagated watermark (min of producers? what if one producer is a changelog?) is unstated. §5.7 mode compatibility is only checked against *sinks*, not against co-producers or downstream consumers.
- P16b: `insert into R` where `R` carries `time ts watermark bounded 1 min` — inserting into a *source* stream. E-TIME-002 rejects time clauses on derived streams, but nothing rejects deriving into a time-claused stream, which creates a stream with two clocks (declared + propagated) and a mixed admission story.

Needs: a §3.3-level statement (recommend: the rule→stream graph is a DAG; each derived stream has exactly one producer, or defined merge semantics; a stream is source xor derived) with a code for each violation.

### RS-26 (later) — reference and lexical hygiene

- errors.md E-EXP-001 cites "SPEC §6.9"; expect is §6.8 (SPEC §6 ends at 6.8).
- grammar.lark header: "every file in **examples/invalid/** must fail" — the directory is `invalid/`; line 49 cites "static rule **S-PART-001**" — the code is E-PART-001.
- grammar.ebnf never defines `pragma-id` even as a comment-terminal (lark pins `/[a-z][a-z0-9_]*\/[0-9]+\.[0-9]+/`); `00.1` minor versions are accepted.
- `BACKTICKED: /`[^`\n]+`/` admits names like `` `a->b` ``, `` `x ${y}` `` (P26 parses) — legal identifiers indistinguishable from operators/param-refs in review diffs; EBNF's `any-but-backtick` doesn't even exclude newlines, a (benign) third drift.
- P20: a UTF-8 BOM at file start is an `UnexpectedCharacters` error at 1:1; SPEC §2.1 ("Source is UTF-8") should say whether a leading BOM is accepted (stripped) or rejected — BOM-prefixed files are common from Windows tooling.

### RS-27 (later) — accepted-but-unstated forms

All parse (probes cited); none are mentioned by SPEC, so the formatter/canonical-form contract (§9) is their only witness:

- `within 5min` — no whitespace between count and unit (P11).
- Zero durations: `for 0 ms`, `within 0 ms` (P27), `delay(0 ms)`, `output first every 0 ms`, `ttl` vs zero — semantics of a zero-width absence window or zero rate-limit never stated.
- Filterless steps: `e1 = S` with no `[…]` (P59) — legal, matches every event; worth one sentence.
- Multiple `expect` blocks per rule (P50) — §4.4's grammar sketch shows one optional; `extra*` allows many (and interleaved `todo`s); fine, but say so.
- Two sibling and-groups (P57): `(a and b) -> (c and d)` parses while SPEC §5.2 says "**one** parenthesized … pair" and SEMANTICS §6.1 "beyond one parenthesized logical group" — "one group per pattern" vs "no nesting" are different rules; pick one and say which.

### RS-28 (later) — hidden dependency: the stream's `d` silently retunes every rule

Checklist D probe: blanking all identifiers in ex01 leaves clause roles fully readable (keyword-anchored grammar — pass). But the second D question fails: change `AgentActions`' `watermark bounded 2 min` to `bounded 10 min` and, with **no edit to any rule text**, every consuming rule's absence fire-time, detection latency (§4.2's own note: latency = `d` past the bound), late-drop population, and `expect` feasibility change. The rule text answers "what fires, when, on which clock" (§0's reader contract) only jointly with declarations possibly hundreds of lines away. Recommend: the canonical formatter (§9) renders consumed streams' time clauses as a generated comment on each rule, or review tooling is required to show them; at minimum SPEC §4.2's note should state the blast radius of editing `d` on a shared stream.

---

## Probe inventory

- [probes/run_probes.py](probes/run_probes.py) — P01–P40: 48 assertions; 8 grammar-vs-spec surprises, all reported above (P01, P02, P02b, P03, P03b, P04, P04b, P20).
- [probes/run_probes2.py](probes/run_probes2.py) — P41–P60: reserved-word gap (`for`), annotation param-refs, multi-expect, sibling groups, triple-quote escapes, illegal emit combos.

Both run standalone: `python3 docs/language/review/probes/run_probes.py` (requires `lark`; parses `../../grammar.lark` with LALR, same configuration as the conformance checker).

---

## Dispositions (coordinator, 2026-11-01) — per ground rule 16

All 28 findings verified first-hand (probes re-run before any fix). **26 FIXED, 2 accepted with documented justification; none rejected.** Full verification suite re-run after the round: grammar_check PASS (9 valid, 22 invalid, 0 ambiguity), semantics harness 40/40.

Blockers — all fixed: RS-01 grammar made case-insensitive end-to-end (keywords `"…"i`, NAME lookahead `(?i:…)`; uppercase programs parse, `Select`/`FOR` as identifiers rejected); RS-02+RS-23 triple-quoted strings pinned **raw** normatively, corpus/spec regexes moved into them; RS-03 EBNF/SPEC/lark aligned (lexically numbers; `E-DUR-002` static: integer, strictly descending, distinct); RS-04 `for` added to the reserved lookahead; RS-05 `not in` pinned atomic null-strict, flagship rewritten with `coalesce(destination,'unknown')`, lint E-NULL-002 extended to membership; RS-06 expect corrected (`math:round(0.9)` = 1.0, now defined in Appendix F); RS-07 §5.8 split into aggregating (cron/batch) vs per-row (sort/delay) select rules; RS-08 annotation values accept `${param}` tokens, `${…}` inside strings pinned inert, flagship sink parameterized properly.

Should-fix — fixed: RS-09 (E-LATE-002), RS-10 (E-EMIT-002), RS-11 (unary minus + signed literals in grammar+EBNF; "SQL verbatim" reworded with named differences), RS-12 (≥1-positive-step sentence + E-PAT-005 + counter-file), RS-13 (capture uniqueness/shadowing: E-PAT-006, E-NAME-002), RS-14 (expect grammar: multi-given, empty then = defeat path, `null` literal; partial then-record matching pinned; flagship gained a defeat expect), RS-15 (Appendix F normative function catalog; `minimum`/`maximum` naming; E-FN-001), RS-16 (§5.10 typing rules + E-TYPE-001), RS-17 (E-PAT-004 counter-file added; E-RES-001 reclassified [parse] with code-binding note; 22 parse codes ↔ 22 files, 1:1), RS-18 (counts regenerated: 56 codes = 22 parse + 34 static; 9 valid / 22 invalid), RS-19 (ex03 intent corrected; ex04 partitions by source; ex06 todo removed), RS-20 (single-input timer clock defined via contributions; `idle after` required on all timer-feeding streams, E-IDLE-001 widened; ex02/03/04 updated), RS-21 (param defaults vs deployment overrides pinned; overrides recorded in deploy metadata), RS-22 (E-SYN-002/003/004 + E-CMT-001 + E-PRAG-002 with counter-files where parse-level), RS-24 (E-WIN-004 sort positive integer; cron dialect pinned: 5-field Vixie, names, DOW 0, no seconds/macros/L-W-#), RS-25 (DAG + source-xor-derived + single producer: E-GRAPH-001/002/003).

Later: RS-26 fixed (E-EXP cite, lark header, EBNF pragma-id, BACKTICKED charset tightened, BOM ignored + documented); RS-27 **partially fixed / accepted**: zero durations now E-DUR-003, filterless steps + sibling groups + multi-expect made explicitly legal in SPEC; `5min` without space stays accepted (formatter canonicalizes — documented exception, REVIEW A6). RS-28 **accepted with mitigation**: blast-radius note added to §4.2 and the canonical formatter is required to render consumed time clauses on each rule; the dependency itself is inherent to shared declarations.

Residual risk recorded: probe scripts assert pre-fix behavior and are retained as the review record (not maintained); EBNF↔lark equivalence remains corpus-tested until the Phase 1 grammar-differential job.
