# PRIOR-ART.md — inherit/break dispositions

> **Historical — language workstream, L1 artifact, 2026-10-31.** Normative today: [SPEC.md](SPEC.md) (v0.1, revised since; see its change log), the grammar, [errors.md](errors.md) and the corpus; [SEMANTICS.md](SEMANTICS.md) elaborates. Where they disagree with this file, they win. **Workstream:** [PLAN](PLAN.md) · [DESIGN (L0)](DESIGN.md) · **PRIOR-ART (L1)** · [SEMANTICS (L2)](SEMANTICS.md) · [SYNTAX (L3)](SYNTAX.md) · [SPEC (L4)](SPEC.md) · [EVAL (L5)](EVAL.md) · [REVIEW (L6)](REVIEW.md).

L1 artifact ([PLAN.md](PLAN.md)). Computes the novelty budget: what we inherit verbatim, what we deliberately change, what we leave out — one line of reason each. The decision to build is fixed; this is not a feasibility review.

**Evidence base:** [research/siddhi-surface.md](research/siddhi-surface.md) (full SiddhiQL inventory: grammar `SiddhiQL.g4` @ v5.1.35-1 cross-checked against the [5.0 query guide](https://siddhi.io/en/v5.0/docs/query-guide/), verified 2026-10-31) · skill prior-art references (Siddhi, Esper, Flink MATCH_RECOGNIZE, ksqlDB/Beam) · text-operator literature review: **pending** (§8, agent running; L1 does not close without it).

**Disposition key:** **keep** = construct and name inherited · **modernize** = concept inherited, form/semantics changed (reason + replacement given; ledger entry) · **drop** = left out (reason) · **defer** = in the language's future, *reserved* in spec v0.1 (staged-depth rule). **Stage:** MVP / P2+ (roadmap phase) / res (reserved, no phase yet).

## 1. Primary language style (decision)

**Stream-query spine + composition-operator pattern sub-language.** SQL prior for everything expression-shaped; readable named-step sequences for patterns; no production-rule or logic-language features. This is the taxonomy's recommended combination for LLM generation + human review, and matches the ratified fixed decisions (named steps + arrows; SQL verbatim). Mixing styles is the documented source of semantic incoherence — rejected.

## 2. Inherit / break master table

| Concern | Inherit from | Form | Break / why |
|---|---|---|---|
| Expressions, literals, operators, comments | SQL | verbatim | never invent (fixed) |
| Stream schema | Siddhi | `define stream X (a int, …)` | **add mandatory time/watermark clause** — Siddhi bolts event time on via `@app:playback`/conventions |
| Query spine | Siddhi | `from … select … insert into` | keep from-first (dataflow reading order; Siddhi/EQL precedent); clause order fixed; add mandatory clauses |
| Window names | Beam/Flink/ksqlDB | `tumbling`, `hopping … every`, `session gap` | break from `#window.time(1 min)` — standard names carry the LLM prior; "sliding" defined exactly or omitted |
| Lateness | ksqlDB `GRACE PERIOD` | explicit lateness clause | **mandatory** — silent late-drop is a correctness trap |
| Emission | ksqlDB `EMIT CHANGES/FINAL`, Timeplus `emit` | explicit emission clause | **mandatory**; replaces Siddhi `current/expired/all events`, which leaks engine window-expiry internals |
| Pattern sequence | Siddhi | named steps + connector (`e1=… -> e2=…`) | ratified; connector token (`->` vs `then`) decided at L3 read-aloud |
| Absence | Siddhi `not … for` / FlinkCEP `notFollowedBy` | keyword absence with required bound | phrasing finalized at L3; bound mandatory (fixed) |
| Repetition | MATCH_RECOGNIZE `{n,m}` concept | readable bounded repetition | break from Siddhi `<2:5>` (unreadable, positional); no unbounded repeat without `within` |
| After-match policy | MATCH_RECOGNIZE / Esper annotations | explicit named policy | Siddhi has none; reserved, conservative default stated in L2 |
| Restart (`every`) | Siddhi/Esper | descriptive, pinned semantics | Siddhi `every`+parentheses is a semantic trap — L2 pins meaning; Esper `every-distinct` concept reserved |
| Match output | MATCH_RECOGNIZE MEASURES concept | named captures only (`e1.attr`) | no positional access |
| Partitioning | Siddhi | `partition with (key of Stream)` | keep; stateful-construct partitioning rules pinned at L2; idle-key TTL becomes a language clause |
| Application unit | Siddhi app | defs + queries + sinks in one file | add version pragma at top |

## 3. SiddhiQL dispositions — definitions & app structure

| Construct | Disposition | Stage | Reason / replacement |
|---|---|---|---|
| App unit (defs before queries) | keep | MVP | proven; + version pragma (ledger) |
| `@app:name` / `@app:description` | keep | MVP | trivial |
| `@app:statistics` | drop | — | runtime/config concern (Prometheus), not rule text |
| `@app:playback` | modernize | MVP | playback is an engine/test-harness mode (FA10); event time is declared per stream, not via app annotation |
| Generic annotation syntax (`@X(k='v')`) | modernize | MVP | keep shape; **closed, validated vocabulary + typed values** — Siddhi parses any name, values strings-only; unknown names must fail deploy with suggestion (identifier hallucination is the top LLM error) |
| `@info(name)` | modernize | MVP | named rules become first-class (form at L3) |
| `@Async` | drop | — | threading is runtime config, not rule semantics |
| `define stream` | keep | MVP | + mandatory time clause (break table) |
| `define table` | keep | MVP | in-memory, `@PrimaryKey`, programmatic load (MVP scope) |
| `define window` (named/shared) | defer | P2 | window catalog is Phase 2 |
| `define trigger` | defer | P2 | not in MVP slice; cron string gets typed form later |
| `define aggregation` (incremental) | defer | P3 | FA6 minimal rollups; fix `week` granularity + wildcard-string `within` when it lands |
| `define function` (inline script) | drop | — | no JVM scripting in a Go engine; opaque `{…}` bodies defeat validation; replacement: Go extension SPI, WASM later |
| Attribute types | modernize | MVP | add first-class `timestamp` (+ `decimal` consideration) per FA1 |
| Backtick identifiers | keep | MVP | SQL-ish escape hatch |

## 4. Dispositions — queries, projection, output

| Construct | Disposition | Stage | Reason / replacement |
|---|---|---|---|
| From-first query shape | keep | MVP | dataflow reading; ledger (differs from SQL select-first) |
| Filters `[expr]` | keep | MVP | pre/post-window positions pinned at L2 |
| Optional `#` before filter | drop | — | one way to write everything |
| Stream functions `#fn()` | modernize | MVP | closed catalog + namespaced extensions only |
| `#window.<name>()` attachment | modernize | MVP | explicit window clause with standard names (break table); MVP carries it for the four owner-added windows |
| Inner streams (`#Name`) | defer | P2+ | owner note "consider for MVP" → ruled 2026-10-31: stays Phase 2 (slice thinness) |
| Fault streams (`!Name`) | defer | P2 | owner note "consider for MVP" → ruled 2026-10-31: stays Phase 2, lands with the error model |
| Anonymous nested stream + `return` | drop | — | undocumented in Siddhi; replacement: named intermediate streams (already first-class); fixes "nested-query gap" honestly |
| `select`/aliases/`*` | keep | MVP | SQL verbatim |
| `group by` / `having` / `order by` | keep | P2 | SQL verbatim; post-MVP normative |
| `limit` / `offset` | modernize | P2 | owner ruling 2026-10-31: **deploy-time-constant expressions** (arithmetic, `${VAR}`); per-event values rejected — closes Siddhi's grammar/guide gap in the flexible direction |
| Aggregate functions | keep | P2 | closed catalog |
| `insert into` | keep | MVP | — |
| `current/expired/all events` | drop | — | replaced by the mandatory emission clause (break table) |
| Table `update`/`delete`/`update or insert` | defer | P2+ | MVP tables are programmatic |
| `return` output | drop | — | goes with anonymous streams |

## 5. Dispositions — windows, patterns, joins, partitions

| Construct | Disposition | Stage | Reason / replacement |
|---|---|---|---|
| time/timeBatch/length/lengthBatch/session/timeLength | modernize | P2 | standard names (tumbling/hopping/sliding/session); hopping added (Siddhi gap, decision #3) |
| externalTime/externalTimeBatch | drop | — | dissolved: event time is the default clock, so "external time" is just the time window |
| sort/cron/delay/batch | modernize | **MVP** | owner ruling 2026-10-31 ("need for MVP"): all four in the MVP engine; roadmap amended — pulls the window clause + their emission semantics into MVP scope |
| expression/expressionBatch | drop | — | string-embedded expressions defeat validation and LLM checking |
| frequent/lossyFrequent | drop | — | deprecated upstream |
| Followed-by (`->`) | keep | MVP | ratified; token form at L3 |
| `every` | keep | MVP | semantics pinned at L2 (restart per match-start; parenthesized groups defined); `every-distinct` reserved |
| Named refs (`e1=`) | keep | MVP | fixed |
| Logical `and` patterns | keep | MVP | two-stream limit stated in grammar, not prose |
| Logical `or` patterns | defer | P2 | roadmap |
| `not … and/or` (wait-for absence) | keep | P2 | core absence family |
| `not X for T` (timed absence) | keep | MVP | flagship; phrasing at L3 |
| Counting `<n:m>` | modernize | P2 | readable bounded repetition; no unbounded repeat without `within` |
| Indexed access (`e1[last]`) | modernize | P2 | lands with repetition; forms pinned (out-of-range = null is a silent default — L2 revisits) |
| Pattern `within` | keep | MVP | mandatory (fixed); trailing-vs-per-stage ambiguity resolved explicitly at L2 (grammar/guide disagree in Siddhi) |
| Sequences (`,` strict contiguity) | modernize | P2 | explicit keyword (e.g. `strictly then`), not punctuation |
| Kleene `* + ?` | modernize | P2 | bounded forms only |
| After-match policy | *new* | res | from MATCH_RECOGNIZE/Esper; explicit, conservative default |
| Join family (incl. `unidirectional`, aliasing) | defer | P2 | not in MVP; bare `outer join` **dropped** (grammar accident); `within`/`per` on non-aggregation joins pinned when joins land |
| Value partition `partition with (k of S) begin…end` | keep | MVP | wording review at L3 |
| Range partitions | defer | P2+ | — |
| `@purge` (partition TTL) | modernize | MVP | idle-key TTL is required MVP behavior — becomes an explicit clause, not a stringly annotation |

## 6. Dispositions — tables, aggregations, rate limiting, I/O

| Construct | Disposition | Stage | Reason / replacement |
|---|---|---|---|
| `@PrimaryKey` | keep | MVP | — |
| `@Index` | defer | P2 | — |
| `@store` (external) | defer | P3+ | adapter catalog = sibling repos |
| `@cache` | defer | P3+ | — |
| `in` table membership | keep | MVP | non-SQL twist (SQL `IN` takes list/subquery; ours takes table) — ledger |
| Store/on-demand queries | defer | P2+ | MVP tables queried only by rules + Go API |
| Aggregation `within`/`per` consumption | defer | P3 | typed ranges replace wildcard strings when FA6 lands |
| `output first every T` | keep | MVP | alert-storm suppression; L2 pins rate-limit vs trigger semantics (Siddhi conflates) |
| `output last/all every` | defer | P2 | — |
| `output snapshot every` | defer | P2 | — |
| `@source`/`@sink`/`@map` shape | keep | MVP | closed validated vocabulary + typed values (as §3 annotations); MVP set: http/inMemory/stdin sources; http/smtp/log sinks; json/text maps |
| `{{attr}}` sink templates | keep | MVP | — |
| `@attributes`/`@payload` | keep | MVP | minimal forms; rest deferred |
| `@distribution`/`@destination` | defer | P2+ | — |
| `@OnError(action='LOG')` | keep | MVP | STREAM/fault-stream action deferred to P2 |
| `on.error` sink option | modernize | MVP | retry/backoff is MVP; `WAIT` backpressure semantics pinned at P2 |
| `@source(ref=…)` | defer | P2 | — |

## 7. Dispositions — expressions & lexical

| Construct | Disposition | Stage | Reason / replacement |
|---|---|---|---|
| Literals (int/long/float/double/bool/string/triple-quoted) | keep | MVP | SQL-compatible; suffixes documented |
| Arithmetic/comparison/logical/`is null`/parens | keep | MVP | SQL verbatim (fixed) |
| Operator precedence | keep | MVP | SQL standard; `in` placement pinned at L4 |
| Time constants (`5 min`, `1 hour 30 min`) | keep | MVP | fixed; descending-unit rule documented |
| Keywords case-insensitive / identifiers case-sensitive | keep | MVP | fixed |
| Comments (`--`, `/* */`) | keep | MVP | documented normatively (Siddhi never did) |
| Keywords as identifiers (`keyword` escape rule) | drop | — | reserved keywords + fix-suggesting errors beat contextual-keyword ambiguity; kills dead surface like `RAW` |
| Function calls `(ns:)name(args)` | keep | MVP | closed catalog per deployment; unknown name = deploy error + suggestion |
| Built-in scalar/aggregate catalogs | keep | MVP | starter set per roadmap; catalog is spec appendix, versioned |
| `${VAR}` substitution | *new* | MVP | not in SiddhiQL grammar (verified); ours is a defined language feature — ledger |
| Evidence chain (`chain(e1,e2)` or similar) | *new* | MVP | forensic chains are a product requirement (FA3/FA13); form at L3 — ledger |
| `llm:` namespace | *new* | res | FA14, strictly optional; reserved in v0.1 |
| Text operators | *new* | MVP | **pending §8** |

## 8. Text operators — what the literature dictates

Evidence: [research/text-operators-litreview.md](research/text-operators-litreview.md) (4 families, 10 lessons, 55 primary-source references, gathered 2026-10-31; spot-verified against Kusto docs, Go `regexp`, and NVIDIA NeMo 0.21.0 docs). These are binding **constraints** for L2/L3; final semantics and names are decided there.

| Constraint | Evidence (lit review §5) |
|---|---|
| Term match and substring match are **distinct, named operators** — never one operator with hidden tokenization | Kusto `has` vs `contains` trap + perf cliff; Splunk `TERM()` silent failures; Elastic "avoid term query for text fields" (lesson 1) |
| Case sensitivity is **explicit per text predicate**; no global default | ecosystem defaults diverge (Sigma/Splunk insensitive vs YARA/Snort/EQL/LIKE sensitive); Sigma's global default forced slow `(?i)` wrapping on backends (lesson 2) |
| Regex anchoring (substring vs whole-string) is **named in the semantics** | Esper `regexp` = whole-string, Flink `REGEXP` = substring — same keyword family, opposite meanings (lesson 3) |
| Regex is **linear-time class only** (RE2; Go stdlib `regexp` already guarantees it); no backreferences/lookaround, documented | Cloudflare 2019 outage postmortem ("the only real solution"); OWASP ReDoS; PostgreSQL hostile-pattern warning (lesson 5) |
| **Keyword-set operator** (any/all of a list) is first-class, not sugar over regex | ModSecurity `@pm` "performs much better than a regular expression"; Snort/Suricata MPM; YARA atoms; Aho-Corasick/Hyperscan (lessons 4, 8) |
| Unicode evasion handled **once in the match pipeline** (strip/flag invisibles, ordinal case folding, confusables per UTS #39), not per rule | ASCII-smuggling via Tags block; Bad Characters attacks; LLM Guard InvisibleText; Turkish-I locale bypass (lesson 7) |
| **No stemming/stopword full-text semantics** as a detection primitive | SQL CONTAINS/tsquery/ES analysis lose precision invisibly; every security language chose literal semantics (lesson 9) |
| Text operators are **primitives that feed temporal correlation**, not standalone classifiers | keyword heuristics measured poor: NeMo 31.19% detection @ 7.44% FPR; NotInject: guards ~60% on benign trigger-word text (lesson 6) — this is design principle 7 ("CEP narrows") confirmed by measurement |
| Value-transformation modifiers (base64/UTF-16/dash-variants) are the proven encoded-variant mechanism — **reserved**, not MVP | Sigma `base64offset`/`windash`; YARA `xor`/`base64`/`wide` (lesson 8) |
| Spec defaults, escaping and anchoring **normatively** — streaming SQL's thin text predicates are the cautionary tale | Flink LIKE "escape character has not been supported yet"; ksqlDB LIKE documented as bare grammar (lesson 10) |

Candidate operator family for L2/L3 (dispositions): term match (*new*, MVP) · substring contains/prefix/suffix (*new*, MVP) · keyword-set any/all (*new*, MVP) · safe regex with explicit anchoring (*new*, MVP) · normalization pipeline (*new*, MVP, engine-level with language visibility) · transformation modifiers (*reserved*) · similarity/`llm:` (FA14, *reserved*).

## 9. What the other languages contribute

- **Esper**: vocabulary for consumption/multiplicity policies (suppress-overlapping, discard-partials), `every-distinct`, count-bounded `within` — concepts reserved for L2/L3, not syntax.
- **ksqlDB**: `GRACE PERIOD` and `EMIT CHANGES/FINAL` — the proof that lateness and emission can live visibly in the query; we inherit the phrasing direction and make both mandatory.
- **Beam/Dataflow**: the standard window/trigger/accumulation names and the semantic model L2 will be written against.
- **MATCH_RECOGNIZE/Flink**: after-match skip policies and bounded repetition concepts; the positional-regex surface itself was rejected by owner decision.
- **RisingWave**: separating event-time ordering from matching — worth copying in the engine spec later.

## 10. Exit status

- SiddhiQL constructs dispositioned: **all** (§§3–7 cover every row of the inventory; mismatch items from its §16 resolved: undocumented surface → drop or explicit; prose-only limits → grammar/spec; guide defects noted).
- Text-operator review: **cited** (§8); claims spot-verified against primary sources 2026-10-31.
- Ledger entries for every break/new: recorded in [DESIGN.md](DESIGN.md).
- Gate: owner ratification of the drop list and the text-operator constraints.
