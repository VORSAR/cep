# DESIGN.md — design record for the rule language

Everything under **Fixed** is a constraint all later phases work inside. A concrete conflict may be flagged with evidence for the owner to rule; fixed decisions are never reopened otherwise.

## Purpose

Rule apps describe temporal attack patterns over streams of AI-agent activity (sensitive read → unapproved egress, no human approval) and emit response events (kill switch, escalation). Authors are mostly LLMs/coding agents; reviewers and tweakers are security engineers. A new language because nothing existing combines temporal patterns + absence detection + free-form-text inspection, embeddable in Go, designed for LLM generation — Siddhi is JVM-bound and its dialect has known gaps ([00-product-brainstorm.md](../product/00-product-brainstorm.md) §3, §10).

## Audiences

- **Generator**: frontier LLMs (coding agents; users' copilots). A compile-and-repair loop is guaranteed — deploy-time validation with line/col errors is a core feature (FA1). Grammar-constrained decoding is **not** assumed. Spec sections + examples must fit a small prompt budget.
- **Reader**: security engineers / SOC analysts; SQL-literate, not streaming experts. From the text alone they must answer: *what fires, when, on which clock, with what evidence*.
- **Tweaker**: the same readers, changing parameters only — time bounds, keyword lists, thresholds, destinations — never structure.

## Unit of design

Whole application ("app"): stream/table definitions, continuous queries, partitions, sinks, `${VAR}` parameters in one deployable unit (Siddhi tradition; matches the MVP kill-switch rule).

## Compile target(s)

This engine's rule model (Go-native CEP runtime) — single target. Two authoring surfaces build the identical model: the DSL (this spec) and the generics-typed Go builder API; parity proven by test (O2). Spec is engine-specific with a Go-API mapping appendix.

## Fixed

- We are building this language (owner, 2026-10-31).
- Modernized Siddhi-inspired streaming SQL; **no** SiddhiQL compatibility promise; fix known gaps (decision #3).
- Expressions, operators, literals, comments: SQL verbatim (skill rule; PLAN L3).
- Go builder API is the headline interface, with generics-typed streams; the DSL is the shareable/ops format; guaranteed parity (decision #4; owner directive 2026-10-31).
- Mandatory explicit clauses for time model, lateness, emission, pattern bounds — zero silent defaults (PLAN L2/L3).
- First-class **deterministic** text operators in core: keyword-set match, contains/prefix, regex — most traffic is prompts. Semantic similarity and `llm:` judgment live in the optional FA14 tier (owner directive 2026-10-31).
- LLM constructs strictly optional, per-rule opt-in; every capability works with no LLM configured (decision #9).
- Spec depth staged: structural semantics for the whole language; normative syntax detail for the MVP subset first, rest marked *reserved* (PLAN).
- Every spec section carries an easy-to-understand, runnable example (owner directive 2026-10-31).
- Text operators are designed only after the L1 online literature review (detection-rule languages, full-text/regex predicates, prompt-inspection research) (owner directive 2026-10-31).
- ~~Case-insensitive keywords~~ **Reversed by owner 2026-11-01: keywords are lowercase-only** (non-lowercase spellings = E-KW-002; reserved-in-any-case retained so lookalikes can't become identifiers); time constants (`5 min`, `1 hour 30 min`); `${VAR}` substitution (FA1 core).
- Apache-2; public OSS (decisions #11, #12).
- Pattern surface: named steps + arrows (Siddhi-style `e1=… -> e2=…`, `every`, `not … for`), not MATCH_RECOGNIZE-style positional regex (owner ratified 2026-10-31).
- Event time is the only default clock; processing time is opt-in by keyword (owner ratified 2026-10-31).
- Text operators, all four in MVP: term match, substring (contains/prefix/suffix), keyword-set (any/all), safe regex (RE2-class, linear-time, explicit anchoring). Case sensitivity explicit per use — no global default (owner ratified 2026-10-31).
- Matching runs on normalized text by default (invisible characters stripped/flagged, ordinal case folding, stages specced and visible); raw matching is explicit opt-in (owner ratified 2026-10-31).
- Siddhi drop list approved: inline script functions, keywords-as-identifiers, current/expired/all events, expression windows, anonymous streams + grammar accidents (owner ratified 2026-10-31).

**Checklist waiver (2026-10-31):** the semantics checklist item "every window kind has a boundary rule + worked example (incl. hopping offset and session merge)" is satisfied for the four MVP windows only; tumbling/hopping/sliding/session examples are deferred to their P2 normative entry. Corpus examples (passing + violating per rule) are produced at L4 with the grammar.

## Open

Proposed defaults — the owner corrects or ratifies; none is settled until then.

- **Language name**: default *none for now* — docs say "the rule language"; naming follows Vorsar branding when needed (decision #5 logic applies). *(Only remaining open item; terminator `;`, lowercase canonical casing, and annotation style were all settled in SYNTAX.md, 2026-10-31.)*
- ~~**Statement terminator**~~ Settled at L3: mandatory `;` (SYNTAX.md).
- ~~**Sequence connector token**~~ **Ruled 2026-10-31 (L3):** surface style = **symbols** — `->` connector, bracket filters, `every` restart marker (owner chose over the sentence-style candidate, previews compared). Literate layer: **`intent` required on every rule; `expect` optional** (required for shipped gallery/pack rules).
- ~~**Source/sink attachment**~~ Settled at L3: typed `@source`/`@sink`/`@map` annotations, closed vocabulary (SYNTAX.md §2).
- ~~**Keyword casing**~~ Settled at L3: lowercase canonical, formatter-enforced (SYNTAX.md).

## Novelty ledger

Every non-SQL construct is priced here before it enters SYNTAX.md.

| Construct | Replaces (prior art) | What it buys | Cost / risk | Status |
|---|---|---|---|---|
| from-first rule spine (`rule … from … select … insert into`) | SQL select-first (Siddhi/EQL precedent) | dataflow reading order; named rules first-class | differs from SQL habit | **accepted (L3)** |
| followed-by `->` with `name = Stream[filter]` steps | Siddhi | temporal ordering, named captures, dense | symbols don't read aloud (owner chose over sentence style) | **accepted (L3, owner)** |
| absence `and not Stream[filter] for D` | Siddhi, FlinkCEP `notFollowedBy` | "the approval never came", bounded, continuous from match start | — | **accepted (L3)** |
| mandatory restart marker `every` / `once` | Siddhi's optional `every` | kills the silent-default restart trap | one extra word per pattern | **accepted (L3)** |
| `intent` required + `expect`/`todo` optional | literate-programming practice | reviewers read purpose first; executable examples; visible abstention | one extra line per rule | **accepted (L3, owner)** |
| case-suffixed text operators (`has_ci`/`has_cs`, …) + `raw()` | Kusto `_cs` suffixes | case mode explicit per use, no global default | more operator names | **accepted (L3)** |
| duration constants `5 min` form | Siddhi time constants | ratified FA1 form, reads aloud | longer than terse `5m` (skill default overridden) | **accepted (L3)** |
| mandatory `within` bound | Siddhi/Esper `within` (optional there) | no unbounded pattern state | more typing per rule | candidate (L2) |
| mandatory time/watermark clause on streams | ksqlDB `WITH(timestamp=…)` | clock explicit per stream | more typing | candidate (L2) |
| mandatory lateness clause | ksqlDB `GRACE PERIOD` | no silent late-drop | more typing | candidate (L2) |
| mandatory emission clause | ksqlDB `EMIT`, Timeplus | replaces Siddhi current/expired/all leak | more typing | candidate (L2) |
| partition block + idle-key TTL clause | Siddhi `partition with` + `@purge` | per-agent isolated state, safe cardinality | TTL clause is new surface | candidate (L3) |
| bounded repetition | Siddhi `<n:m>`, Kleene, M_R `{n,m}` | readable, always bounded | new wording | candidate (L3, P2) |
| after-match policy | MATCH_RECOGNIZE/Esper annotations | explicit multiplicity | reserved | candidate (reserved) |
| `in` table membership | SQL `IN` (list/subquery) | allowlist checks in filters | overloads SQL `IN` meaning | candidate (L2) |
| term-match text operator | Kusto `has`, Splunk `TERM()` | tokenized match, prefilter-friendly | tokenization rules must be specced | candidate (L2) |
| substring/prefix/suffix operators | Kusto `contains`, Sigma modifiers | literal substring, no analysis | scan cost; case stated per use | candidate (L2) |
| keyword-set operator (any/all) | ModSecurity `@pm`, Sigma lists, YARA `N of` | multi-keyword via Aho-Corasick | new surface | candidate (L2) |
| safe regex (explicit anchoring) | Go `regexp`/RE2; Esper-vs-Flink conflict | linear-time, ReDoS-immune | no backrefs/lookaround — document | candidate (L2) |
| text normalization pipeline | LLM Guard, UTS #39, ModSecurity transforms | Unicode evasion handled once | stages must be specced, visible | candidate (L2) |
| value-transformation modifiers | Sigma `base64offset`/`windash`, YARA | match encoded variants readably | reserved post-MVP | reserved |
| output rate limiting | Siddhi `output first every` | alert-storm suppression | trigger-vs-ratelimit semantics must be pinned | candidate (L2) |
| `${VAR}` substitution | env/config conventions (not in SiddhiQL) | portable rules, no secrets inline | substitution timing must be defined | candidate (L2) |
| evidence chain function | — (product requirement FA3/FA13) | forensic chain on every match | new function semantics | candidate (L2) |
| version pragma | common practice | spec evolution safety | one line per file | candidate (L4) |
| `llm:` namespace | — (FA14) | optional semantic judgment | reserved, opt-in | reserved |

## Decisions log

| Date | Decision | Rationale | Evidence / reference |
|---|---|---|---|
| 2026-10-31 | Modernized Siddhi-inspired dialect, no compat | proven ergonomics, fix gaps | brainstorm §10.3 |
| 2026-10-31 | Go API-first, generics-typed, DSL parity | embeddable-first, type safety | brainstorm §10.4; owner directive |
| 2026-10-31 | Deterministic text operators in core; similarity/`llm:` optional | prompt traffic; LLM-optional principle | owner directives; principle 7 |
| 2026-10-31 | Example per spec section (L4 gate) | readability + LLM generatability | owner directive |
| 2026-10-31 | L1 literature review before text-operator design | ground in prior art | owner directive |
| 2026-10-31 | Fixed list ratified; pattern surface = named steps + arrows; event time only default clock | L0 exit gate; LLM failure modes; replayable detections | owner ratification (AskUserQuestion) |
| 2026-10-31 | Primary language style: stream-query spine + composition-operator patterns | taxonomy recommendation; style mixing causes incoherence | PRIOR-ART.md §1 |
| 2026-10-31 | Dispositions: drop inline script functions, keywords-as-identifiers, current/expired/all events, expression windows, anonymous streams; annotations become closed+typed | validation, LLM error profile, Go engine | PRIOR-ART.md §§3–7 — **owner approved** |
| 2026-10-31 | Text operators: all four families in MVP; matching on normalized text by default, raw opt-in | lit-review lessons 1–7; principle 7 | PRIOR-ART.md §8 — owner ratified |
| 2026-10-31 | `sort`/`cron`/`delay`/`batch` windows added to MVP (roadmap amended); inner/fault streams stay P2; `limit`/`offset` take deploy-time-constant expressions | owner rulings on PRIOR-ART notes | AskUserQuestion batch; 02-roadmap.md amendment |
| 2026-10-31 | Null semantics: two-valued, null-strict comparisons (no SQL 3VL); `!=`-on-nullable lint | missed-detection trap beats SQL purity | SEMANTICS.md §1.1 — owner ratified |
| 2026-10-31 | After-match default: skip-past-last with earliest-wins on simultaneous completion — one burst, one alert | alert-volume sanity; overlap modes reserved opt-in | SEMANTICS.md §6.5 — owner ratified |
| 2026-10-31 | Interim adversarial review (owner-directed): 22 red-team findings (10 blocker) all verified and FIXED in SEMANTICS.md — headline fixes: continuous absence monitoring, propagated watermarks for derived streams, playback virtual clock, per-stream partition keys, cron single-fire lateness, purge destroys all per-key state w/ changelog retractions, true skip-past-last | reviewer-independent design review before syntax | review/redteam-semantics.md (dispositions inside) |
| 2026-10-31 | Attack-landscape validation (14 real 2025–26 attacks): kill-switch use case confirmed; roadmap amended — fleet/fan-out honestly scoped to P2–P3, count-over-rolling-window front-loaded in P2, anti-join + first-seen named, telemetry ingestion = detection precondition, delegation-depth non-goal | evidence-based roadmap validation | review/attack-validation.md (dispositions inside) |
| 2026-11-01 | Owner reversal: keywords lowercase-only (was case-insensitive, L0). Implemented + verified same day: grammar flags removed, any-case reservation kept, E-KW-002 added with counter-file; corpus/probes green. SPEC v0.1 rev 1 (pre-release) | one canonical spelling; crisp diagnostics beat silent case tolerance | SPEC §2.1; probes |
| 2026-11-01 | L6 closed: red team found 28 (8 blockers — incl. case-insensitivity unimplemented, `for` unreserved, `not in` null ambiguity on the flagship, corpus expects failing their own rules); 26 fixed, 2 accepted; SPEC v0.1 frozen; headline semantic pins: `not in` atomic null-strict, raw triple-quoted strings, idle-after on every timer-feeding stream, DAG stream graph, normative function catalog | the freeze gate did its job — four normative-artifact contradictions never reached the engine | REVIEW.md; review/redteam-spec.md |
| 2026-10-31 | L5 thresholds pre-registered BEFORE any generation run (skill defaults, Text2DSL/Anka range): parse ≥ 97%, static-clean ≥ 90%, one-round repair ≥ 95% of remainder; 12 held-out intents stratified across filter/text/window-kinds/patterns/params; spec-only prompt = SPEC.md + grammar.ebnf + errors.md + 3 tagged examples | grading criteria fixed before results exist | eval/tasks.md; evaluation-harness.md |
| 2026-10-31 | L4 grammar findings: two real ambiguities caught by the Earley check (raw-wrap vs call; true/false vs bare name) — fixed structurally by encoding the reserved-word list into the NAME terminal; rule count 79 vs ~60 target, justified (19 are alias alternatives of shared productions, not new constructs) | ambiguity eliminated by construction, not convention | grammar.lark; checker PASS |
| 2026-10-31 | L3 surface: symbol style — `->` connector, `[bracket]` filters, `every`/`once` restart markers (mandatory); `intent` clause required per rule, `expect` optional (required in shipped packs) | owner ruling (read-aloud previews compared); explicit restart marker kills Siddhi's `every` trap | AskUserQuestion; SYNTAX.md |
| 2026-10-31 | Six implementation ambiguities pinned at L2 close: admission wm = own events − d in both modes (virtual clock raises min-contributions only); pending completions exempt from `within`-discard (`for` may exceed `within`); skip boundary = last *captured positive* event; same-instant defeaters processed before completions finalize; cron extent counts full length of buckets starting in the horizon; partial-tie = spawn order | found by harness rebuild; zero silent defaults means pinning every one | SEMANTICS.md "pinned 2026-10-31" markers; 40/40 checks |

## Artefacts

| Phase | File | Status |
|---|---|---|
| 1 (L0) | DESIGN.md | ratified 2026-10-31 |
| 2 (L1) | PRIOR-ART.md | ratified 2026-10-31 (owner amendments applied) |
| 3 (L2) | SEMANTICS.md | **closed 2026-10-31** — adversarially reviewed (22 findings fixed), 40/40 harness checks green incl. RT-labeled regressions; 6 implementation ambiguities pinned |
| 4 (L3) | SYNTAX.md | drafted 2026-10-31 — style + literate-layer owner-ruled; read-aloud pass at L4 corpus review |
| 5 (L4) | SPEC.md, grammar.ebnf+.lark, errors.md (56 codes), examples/ (9) + invalid/ (22) | **frozen in v0.1** 2026-11-01 after the L6 round — `grammar_check` PASS; expect-runner lands with the engine |
| 6 (L5) | EVAL.md | **run 2026-10-31** — 12/12 parse, 12/12 static, 0 ambiguous (targets ≥97/≥90 met); 14 spec-gap reports harvested → L6 input; executor-agreement + human read-aloud planned (engine Phase 1 / pre-launch) |
| 7 (L6) | REVIEW.md | **closed 2026-11-01** — A–F checklist + independent red team (28 findings: 26 fixed, 2 accepted, 0 rejected); **SPEC v0.1 FROZEN**; engine Phase 1 unblocked |

*Spec rev 3 (2026-11-04, sprint S1.2): the owner's decisions of 2026-11-01/02 applied — `E-GRAPH-004`, `E-PART-003`, `E-TIME-003` added (60 codes); `late events allow` removed for v0.1; cron admission pinned to SPEC §6.1 (executor aligned, 41/41 harness checks); `text_invisibles` engine-owned; keyword sets over tables use the per-event snapshot; `time arrival` kept. No new design decisions.*

*Spec rev 4 (2026-11-06, sprint S3.2b-1): `E-NULL-003` and `E-PART-004` added (62 codes; invalid/ holds 24 files); harness 48/48 checks.*
