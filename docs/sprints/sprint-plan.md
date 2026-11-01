# Sprint Plan — complex-events-engine

**Document ID:** SPRINT-PLAN-001
**Version:** 0.1 — 2026-10-31: the skeleton, written with the sprint process. No sprint is planned yet; Phase 1's decomposition is the next planning step. *(This document replaces the `docs/product/03-sprint-plan.md` named in the objectives and the roadmap.)*
**Status:** Active sprint schedule. Governed by [00-sprint-model.md](00-sprint-model.md), the development process every sprint runs. The objectives say **why** ([01-objectives](../product/01-objectives.md)), the roadmap says **in what order** ([02-roadmap](../product/02-roadmap.md)), and this plan says **which sprint runs next**, with its size, scope and dependencies.

---

## 1. Strategic frame

The roadmap's MVP strategy (owner, 2026-10-31): one real use case, **the Agent Kill Switch**, built as a vertical slice through every layer it needs, then breadth. Three rules from the objectives shape every sprint here:

- **Tests are the spec.** The gate, playback mode and the conformance harness come before the operators they test.
- **Work decomposes.** Each sprint is a set of lanes an agent can finish and a test can prove, with the owner's review at the seams.
- **Scope discipline is structural.** Anything not serving the current phase's exit criteria waits (the core / diff / later tiers).

---

## 2. Where we are today

> **Current status is the source + git, not this prose** (see the §Status line). This section is the forward gap list.

No code yet. The product documents are written: the [brainstorm](../product/00-product-brainstorm.md) with its decisions, the [objectives](../product/01-objectives.md) and the [roadmap](../product/02-roadmap.md). The development process is adopted: [00-sprint-model.md](00-sprint-model.md), [DEVELOPMENT.md](../development/DEVELOPMENT.md), [testing.md](../development/testing.md). The gate, the Go module and CI do not exist yet.

---

## 3. Phase 1 — MVP: the Agent Kill Switch slice

> **Exit criterion:** the roadmap's five ([roadmap — exit criteria](../product/02-roadmap.md#exit-criteria-definition-of-shipped)): (1) the demo runs end to end from the public quickstart in under 15 minutes; (2) conformance and unit tests green for every shipped construct, with detection → kill-switch latency measured and published; (3) docs for the shipped subset only, tagged `v0.1.0`, public repo; (4) the same detection authored in the DSL and the Go API, with identical behaviour proven by one test; (5) the attack gallery: ≥ 5 cited, incident-inspired examples, runnable via playback, green in CI. *(Annotated 2026-11-01: this line said "four" — criterion 5 was added to the roadmap on 2026-10-31.)*

**Phase entry** ([sprint model §9](00-sprint-model.md#9-phases-entry-and-exit)): the architecture baseline written and reviewed (architect + the-fool), ADRs in `docs/architecture/`; Phase 1 decomposed into sprint cards the owner approved. **Phase exit:** the five criteria above, plus a whole-engine break-it (`v0.1.0` is a release).

**Execution order:** enablers first. The gate and playback mode come before the pattern operators ([roadmap — the slice](../product/02-roadmap.md#everything-required-the-slice-by-feature-area)).

### Phase 1 decomposition — v2, APPROVED by the owner (2026-11-02)

> **Status of this section: APPROVED** (owner, 2026-11-02 — §5 "Owner decisions, 2026-11-02 — decomposition approval"); sprint model §9 phase entry 1 holds. Each card still passes its own G0 (plan check reviewed and answered) before its sprint starts. *(Until approval this section read "DRAFT v2 for owner approval"; annotated, not rewritten.)* v1 (2026-11-01) was reviewed blind by the architecture reviewer ([docs/reviews/phase1-decomposition-architecture.md](../reviews/phase1-decomposition-architecture.md), 22 findings) and the QA reviewer ([docs/reviews/phase1-decomposition-qa.md](../reviews/phase1-decomposition-qa.md), 35 findings); every finding is dispositioned in those files. The owner's scoping choices of 2026-11-01 and the six decisions of 2026-11-02 (§5) are applied below.

**What changed from v1, in one paragraph.** v1 had 14 sprints and 167 SP. The reviews found real work v1 left out — test material for seven of the nine examples, five shipped constructs nobody tested, six gate pieces with no sprint, a package cycle, and pieces placed before what they depend on — and showed that v1's split lines pushed work into cards that were already full. v2 therefore **splits every card whose fixed content exceeds the cap into its own sprints now**, before entry, as the sprint model requires: **20 sprints, 240 SP**. No new feature was added; the growth is work that was always required and is now visible.

**How to read the numbering.** ADR-0005 §1's stages keep their numbers; a stage that does not fit one sprint is split before entry (sprint model §3): **S1 = S1.1 + S1.2; S2 = S2.1 + S2.2; S3 = S3.1 + S3.2 + S3.3; S4 = S4.1 + S4.2; S5 = S5.1 + S5.2; S6 = S6.1 + S6.2; S9 = S9.1 + S9.2.** Every ADR reference to a stage ("lands with the parser in S2", "S3's exit") means the stage; the cards below name the exact sprint. The clock contract freezes at **S3.3's** G5 ("S3's exit", ADR-0005 §1). **Sprint-id grammar** (settled in S1.2, used by `todocheck`, `suppressions` and conformance skips): `S<stage>[.<part>]`, ordered by this table's row order; "current + 2" means two rows down.

**Sequencing principle (owner decision 2026-11-01).** Enablers first (S1.1–S3.3). Then the **kill-switch vertical**: patterns (S4.1, S4.2) make the flagship fire in playback; connectivity (S5.1) makes it fire live through `cepd`; the Go builder (S6.1) proves DSL/Go parity on it. Breadth follows: the formatter (S6.2), text (S7), windows (S8, S9.1, S9.2), the demo kit and published latency (S10), the attack gallery (S11), the release close-out (S12). **From S6.1 onward every construct ships in both front-ends in the sprint that ships it** (builder surface + a parity twin per example).

| Sprint | Title | SP | Routine | Depends on |
|---|---|---|---|---|
| S1.1 | The gate core and the event kernel | 13 | full | — |
| S1.2 | The model kernel, the remaining S1 gate rows and spec rev 3 | 13 | full | S1.1 |
| S2.1 | The parser and the grammar differential | 13 | full | S1.2 |
| S2.2 | The validator, expression typing and the callable registry | 13 | full | S2.1 |
| S3.1 | The clock service, tables and the playback format | 13 | full | S2.2 |
| S3.2 | The runtime core: partitions, shards, routing, live and replay | 13 | full | S3.1 |
| S3.3 | The Deploy pipeline, rate limiting and the conformance runner — clock freeze | 13 | full | S3.2 |
| S4.1 | The pattern engine — the kill switch fires in playback | 13 | full | S3.3 |
| S4.2 | The pattern engine against an independent oracle and the metamorphic laws | 8 | full | S4.1 |
| S5.1 | HTTP in, webhook out — the kill switch fires live | 13 | full | S4.2 |
| S5.2 | The other connectors, error logging and observability | 8 | full | S5.1 |
| S6.1 | The Go builder, DSL/Go parity and the CLI | 13 | full | S5.2 |
| S6.2 | `cep fmt` and the generated timing comment | 13 | full (language) | S6.1 |
| S7 | Text operators, incl. live table term lists | 13 | full | S6.2 |
| S8 | Windows I — sort, batch and `count` | 13 | full, three-reviewer panel | S7 |
| S9.1 | Windows II — cron | 13 | full, three-reviewer panel | S8 |
| S9.2 | Windows III — delay | 8 | full, three-reviewer panel | S9.1 |
| S10 | Demo kit, quickstart, benchmarks and the published latency | 13 | lighter | S9.2 |
| S11 | The attack gallery — seven examples | 13 | lighter | S10 |
| S12 | Release close-out — whole-engine break-it and `v0.1.0` | 8 | full + whole-engine break-it | S11 |

**Forecast (honest, re-forecast at named points).** 20 sprints, 240 SP. At 2–3 weeks wall-clock per sprint for a solo owner, Phase 1 is **40–60 weeks**; the 8-SP sprints should run shorter, so the lower half is plausible but unproven. No velocity has been measured: **re-forecast at S1.2's G5 and at S3.3's G5** from recorded points and durations (sprint model §3), and at any further split. Any 13-SP card the plan check sizes over the cap is split into its own two sprints (Sn.a / Sn.b) **before** G0 — never by moving work into a neighbouring card.

**Gate instruments by sprint** (ADR-0015 `lands_in`, mapped onto the split numbering — each lands with its first subject, fixture, `min_wants`, registry row and teeth pin in one commit, and scans the whole module retroactively).

| Sprint | Instruments landing |
|---|---|
| S1.1 | registry + runner + summary + `lawdelta`; parity + atomic installer; registry-lint; `importcheck`; `clockcheck`; `todocheck` (basic `TODO(owner\|S<n>\|#id)`); `smells`; `suppressions` + the one allowlist grammar; `testhygiene` (S1.1 already writes tests); teeth meta-test; hooks; CI + summary fixtures; `docsync` (a) one direction |
| S1.2 | the dependency-allowlist check (ADR-0009 §1); the licence check and `govulncheck` (T4); the build-tag law (ADR-0015 §3.4); the weekly canary (advisory, aging); the sprint-id grammar, a machine-readable current-sprint marker (`gate/current-sprint`, a law path) and the skip / `deferred(P<n>, ref=…)` forms in `todocheck`; `docsync` (d) **error-count consistency** (errors.md headings = SPEC Appendix C total = ARCHITECTURE §1/§5 counts) |
| S2.1 | `freshness`; `puritycheck`; `globalscheck`; `coverage-inclusion`; the grammar differential (T4); `docsync` (b) reserved words ↔ kwgen; **the nightly workflow** with one advisory, aging row per fuzz target |
| S2.2 | `docsync` (b) [static] codes ↔ `catalog.go`; `docsync` (c) Appendix F ↔ `expr.Registry`; `allocgate` first rows |
| S3.2 | race step + goleak proof; the **metric-tripwire step** (zero-config-is-failure; the outbox ceiling blocking at 65,536 — ADR-0006 §5, ADR-0015 §3.11); `allocgate` ingest → filter |
| S3.3 | `apipartition` (`api/exports.golden`: `cep` + `io`) + `apidiff` against the merge base; the two-OS determinism proof; `gate/regen-baseline` + Tier-2 baselines; the conformance runner's planted wrong-expect proofs; **`api/clock.golden`** (exported signatures of `internal/clock`, added to `gate/law.json` at the freeze — a later change is a LAW Δ) |
| S4.1, S8, S9.1, S9.2 | `allocgate` extended per hot-path stage; the walked-state / timer-derivability invariant (ADR-0006 §2) extended to each new state holder |
| S6.2 | `fmtcheck` (scope below) |
| S10 | benchmark tripwires (benchstat); the compose-kit nightly job |
| S12 | `docsync` (a) reverse direction; the ARCHITECTURE §7 file-existence check |

**Roadmap exit criteria → where each is proven.** (1) quickstart < 15 min → S10. (2) conformance + unit green for every construct → every sprint's exit; latency measured and published → S10. (3) docs for the shipped subset → S10, final pass S12; `v0.1.0` tag + public repo → S12. (4) DSL/Go parity proven by one test → S6.1 (twins extended S7–S9.2, S11). (5) attack gallery ≥ 5 → S11 (seven by owner decision).

**The 40 semantics-harness checks (`docs/language/checks/`) → the one sprint that ports each.** S2.2: the 1.1 null-strict asymmetry check (its scalar assertions; its `in` assertion is completed in S3.1), RT-14, "9 · TTL deploy check", RT-07's extended static check (pattern and rate parts) = 4. S3.3: the three §8 rate-limit checks = 3. S4.1: the eleven §6 checks, the 2.1/6.5 late-drop check, the four §2.1 checks other than RT-09 (all four run patterns), and the two pattern-dependent §9 purge checks = 18. S7: the seven §5 text checks = 7. S8: sort ×3, batch (RT-21), RT-06 changelog purge = 5. S9.1: cron half-open = 1, plus RT-07's window-extent assertions completing the check counted under S2.2. S9.2: delay = 1. **39 ported in Phase 1; RT-09 deferred to Phase 2** — it needs a rule consuming a derived stream, which v0.1 rejects (owner decision 2026-11-01); recorded as `deferred(P2, ref=sprint-plan §5 2026-11-01)` and ARCHITECTURE §7 is annotated. **S3.1 ports no harness check**: all §2.1 checks run patterns; S3.1 instead carries clock-level unit tests of the same arithmetic (admission strictness, the clamped min, per-stream virtual floors, idle exclusion), each citing the check whose arithmetic it pins. New harness checks (and-groups in S4.1; the cron admission rule in S1.2's rev 3) are written red-first in Python before any Go.

**Corpus examples → the sprint that turns each green.** 07-consent-gate, 09-params-and-quoting → S3.3. 01-agent-kill-switch, 08-session-flow → S4.1. 05-text-operators, 06-once-and-group → S7. 03-sort-leaderboard → S8. 02-cron-digest → S9.1. 04-delay-and-batch → its batch rule in S8, its delay rule and the whole file in S9.2. **Test material (owner decision 2026-11-02):** in the sprint that turns an example green, the QA agent adds to the example file **≥ 1 fire-path and ≥ 1 near-miss expect block, multi-key or multi-session wherever the rule correlates**, with expected outputs derived by hand from SEMANTICS or from the Python reference executor **before the engine runs** (first seen failing). Each addition is a spec-corpus change: `grammar_check` re-runs green and the LAW Δ block shows it. An output an expect block cannot express (03's `+`/`−` changelog rows, if SPEC §6.8 has no form for them) gets a `conformance/cases/<example>/` log beside it. Until its sprint, an example's cases are `skip: not-implemented(<sprint>)`; **a skip naming the current sprint is red at that sprint's own T4** (S1.2's current-sprint marker), and every skip still open at S12 is red.

---

#### S1.1 — The gate core and the event kernel

| | |
|---|---|
| **Goal** | A clean clone runs `make check` through the hostile gate's core — pinned toolchain, registry, runner, summary with the LAW Δ block first — and the event kernel `internal/event` exists, is tested, and is policed by it. |
| **Routine · size** | Full loop (the event kernel fixes the null representation and the total order every later output depends on; the gate is an owner ratification condition) · 13 SP |
| **Depends on** | — (first sprint; phase entry §9 must hold: architecture ratified and review-hardened ✔, this decomposition approved ☐) |
| **In scope** | 1. The Go module `github.com/vorsar/complex-events-engine`: `go.mod` with `go` and `toolchain` lines, `GOTOOLCHAIN=local`, Go ≥ 1.24. · 2. `tools.json` + one installer (`cache/tmp.$$`, atomic `mv` into the hash-keyed path) + the parity step, first at every tier. · 3. `gate/registry.json` (row schema = ADR-0015 §1.1), `gate/run.sh` (POSIX; per-step process groups; TERM-then-KILL watchdogs; one JSON record per step; `summary.md` + `run.json`), collect-all, zero-is-failure, `expected_scope` checked against the live expansion, the `exit:` line; `make check`, `make combine-check`, `make strict-gate`. · 4. `gate/law.json` + `lawdelta` + the LAW Δ block (ADR-0015 §1.3). · 5. registry-lint with its three planted problems and the §2 instrument-id pin. · 6. Analyzers in `tools/analyzers/` with `analysistest` fixtures and `min_wants`: `importcheck` (rules from ARCHITECTURE §3 **as it stands after the owner's `io` split of 2026-11-02** — no cycle), `clockcheck` (incl. a synthetic fixture for the `internal/window/cron.go` exception), `todocheck`, `testhygiene`; the teeth meta-test. · 7. golangci-lint strict config (`smells`) with the §2.1 scope classes; `nolintlint`; the `suppressions` checker and the one allowlist grammar under `tools/allowlists/`. · 8. `docsync` (a), one direction: every package directory that exists is listed in ARCHITECTURE §3. · 9. `gate/proofs/` as a separate Go module with the proofs for this sprint's instruments. · 10. `.githooks/` (T0, T1) and the CI workflow (actions pinned by SHA; the summary job `needs: all`, `if: always()`, the only required check) with summary fixture trees. · 11. **`internal/event`**: `Type`, `Attr`, `Schema`, `Value` (null = `Value{Null:true}` keeping the declared `Kind`), `Event`, the total-order comparator `(Ts, StreamID, Seq)`. It is the first layered subject `importcheck` and `clockcheck` police. |
| **Not in scope** | `internal/model` and spec rev 3 → S1.2. The dependency-allowlist, licence, vulnerability and build-tag checks → S1.2. Everything else per the instruments table. |
| **Entry criteria** | The standard ones, plus: this decomposition approved; the plan check answers the linter version, the Go version pin and hook wiring, settles **whether ARCHITECTURE §3's per-package import lists (rules 3, 4, 6) are exhaustive** and writes the answer into `rules.json` (architecture review DA-11 lists the edges that depend on it), places `conformance/` and `bench/` in the layer model, and names the Go specialist agent; the owner sets branch protection (required check = the summary job; no force-push to `main`). |
| **Exit criteria** | The done-done block (§13), plus: |
| | 1. A clean clone runs `make check` to `exit: 0`; the summary opens with the LAW Δ block — **proof:** CI `strict-gate` on the PR and the summary fixture tests classifying every planted tree. |
| | 2. A tool version that differs from `tools.json` fails before any other step and names the fix — **proof:** the parity proof. |
| | 3. An upward import from `internal/event` (into any module package) is red at T0 on the staged file — **proof:** `importcheck` fixture + a hook-level proof acting on `internal/event`. |
| | 4. `time.Now()` outside the allowlist is red; the synthetic `cron.go` fixture passes `time.LoadLocation` and fails `time.Now` — **proof:** `clockcheck` fixtures. |
| | 5. A bare `//nolint`, a PERMANENT suppression without a resolvable `ref=`, and an allowlist entry naming a non-existent package each fail — **proof:** `suppressions` tests. |
| | 6. Weakening a fixture below its `min_wants`, or deleting a §2 instrument's registry row, fails — **proof:** the teeth meta-test + registry-lint proofs. |
| | 7. A planted edit to `rules.json` appears in the LAW Δ block with its one-line semantic delta — **proof:** `lawdelta` test over a fixture repository. |
| | 8. A `time.Sleep` in a test outside the wait helper is red — **proof:** `testhygiene` fixture. |
| | 9. The event total order and the null representation are pinned — **proof:** `internal/event` table tests whose expected orderings are written by hand before the code. |
| **Owner's check** | *By running:* clone fresh, `make check`, read the summary; add a forbidden import on purpose and watch the commit refused. *By reading:* the LAW Δ block on the PR. *By hand:* branch protection set. |
| **Split line** | If over the cap: **S1.1a** = items 1–5, 10 (runner, parity, registry-lint, hooks, CI); **S1.1b** = items 6–9, 11 (analyzers, smells, suppressions, proofs, `internal/event`). |

#### S1.2 — The model kernel, the remaining S1 gate rows and spec rev 3

| | |
|---|---|
| **Goal** | `internal/model` holds the rule IR with a byte-stable canonical encoding, the remaining stage-1 gate rows are live, and spec rev 3 — the owner's language decisions of 2026-11-01 and 2026-11-02 — is applied and mechanically checked. |
| **Routine · size** | Full loop (the IR and a language revision) · 13 SP |
| **Depends on** | S1.1 |
| **In scope** | 1. **`internal/model`**: the IR of ADR-0003 (every field named after its SPEC clause; durations and timestamps `int64` ms; `ParamRef` unresolved) and `MarshalCanonical` (sorted keys, explicit defaults, float `'g'`/−1/64, RFC3339 UTC millis, NaN/±Inf unencodable). **The canonical JSON carries no source positions** — diagnostics keep positions in a side table (QA DQ-16; builder parity needs this too). · 2. The dependency-allowlist check, the licence check, `govulncheck`, the build-tag law, the weekly canary. · 3. The sprint-id grammar, `gate/current-sprint`, and the skip / `deferred(P<n>, ref=…)` forms in `todocheck`. · 4. `docsync` (d), error-count consistency. · 5. **Spec rev 3** (language-design skill; docs lane), containing exactly: (a) the derived-stream-consumption error code (owner 2026-11-01); (b) **the cross-stream partition-key type-mismatch code** (RC8.1; owner decision 2026-11-02: reject at deploy — the error names both streams and both types); (c) **`late events allow` removed for v0.1** (owner 2026-11-02): `allow` is rejected wherever it appears, with E-LATE-002's text revised to "late handling per window arrives in Phase 2; use `late events drop` and set the stream's watermark bound"; example 02 changes to `late events drop`; SEMANTICS §7's cron row annotated; (d) **cron admission pinned to SPEC §6.1**: an event behind its stream's admission watermark is dropped and counted even if its cron bucket is still open — the Python reference executor's `cron_window` is aligned to this and a new harness check pins it, written red-first (the executor admitted such events; SPEC is normative); (e) **`text_invisibles` is engine-owned** (owner 2026-11-02): the name is reserved; the engine always computes it; a value supplied in a payload is ignored, overwritten and counted; a stream may declare it as `bool` for readability; (f) **keyword sets over a table operand read the table's per-event snapshot like `in`** (owner 2026-11-02: supported live); (g) `time arrival` stays (owner 2026-11-02): playback logs carry the recorded arrival stamp as `ts`; (h) E-GRAPH-001 marked unreachable in v0.1 (every cycle needs a rule consuming a derived stream, which (a) rejects first) and kept for Phase 2, with `docsync`'s 1:1 rule accepting the marking; (i) errors.md, SPEC Appendix C and the change log updated. |
| **Not in scope** | Expect blocks for the examples → each example's own sprint. The SPEC §9 timing-comment sentence → S6.2 (with its format). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. `MarshalCanonical` is byte-stable and position-free — **proof:** a hand-derived canonical-JSON golden covering key order, −0, 1e21, timestamp millisecond truncation, and NaN/±Inf rejection (written before the code), plus a test that two models differing only in source positions encode identically. |
| | 2. An unapproved `go.mod` edit, a disallowed licence, a known-vulnerable module and a `//go:build !race` file are each red — **proof:** one planted proof each. |
| | 3. A `TODO` naming a sprint more than two rows ahead, and a skip naming the current sprint, are red — **proof:** `todocheck` fixtures driven by a fixture `gate/current-sprint`. |
| | 4. Spec rev 3 is applied: every decided code is present in errors.md with its SPEC § reference and fix text; the counts agree everywhere — **proof:** `docsync` (d) green, and red on a planted count mismatch; `grammar_check` PASS; the harness passes with its new cron-admission check, which was first seen failing against the unaligned executor. |
| **Owner's check** | *By reading:* the rev-3 change log entry and the revised E-LATE-002 text; the canonical JSON of one model. *By hand:* LAW Δ block (spec files changed). |
| **Split line** | **S1.2a** = items 1–4 (model + gate rows); **S1.2b** = item 5 (spec rev 3). |

#### S2.1 — The parser and the grammar differential

| | |
|---|---|
| **Goal** | `internal/parser` turns every corpus file into a `model.App` or into the exact catalogued parse error at the exact line and column, keeps every comment, and is proven equivalent to the normative lark grammar over the full corpus — by verdict and by parse shape. |
| **Routine · size** | Full loop (the language) · 13 SP |
| **Depends on** | S1.2 (rev 3 applied) |
| **In scope** | 1. The hand-written lexer (ADR-0002 §3), positions in runes, **every comment kept as positioned trivia in a side table reachable from the parse result** (S6.2's printer needs it). · 2. `tools/kwgen/` → `internal/parser/keywords.go` (generated header). · 3. `cep.y` hand-derived from `grammar.lark`; `parser_gen.go` committed (generated header). · 4. AST and lowering to `model.App`. · 5. The `yyError` mapping for all [parse] codes, with line, column and fix text. · 6. **Expected positions:** before the parser runs, the QA agent adds a hand-derived `@ <line>:<col>` to the first-line comment of each of the 24 files in `docs/language/invalid/` (comments only — `grammar_check` ignores them). · 7. The grammar-differential T4 row (ADR-0002 §5 as amended): pinned Python + `lark==1.3.1`; verdicts and canonical operator-spine S-expressions; ≥ 10 torture files in `docs/language/eval/generated/torture/`; corpus count asserted; a comment-torture file. · 8. The parser fuzz target; the nightly workflow. · 9. Instruments: `freshness`, `puritycheck`, `globalscheck`, `coverage-inclusion`, `docsync` (b) kwgen. |
| **Not in scope** | Static codes and expression typing → S2.2. The printer → S6.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. All 9 examples, the generated eval files and the torture files parse; each model's canonical JSON is golden-locked — **proof:** the parser corpus test. |
| | 2. Each invalid file yields exactly its [parse] code at exactly its `@ line:col` — **proof:** the invalid-corpus table test against the hand-derived positions. |
| | 3. Lexing and re-assembling every corpus, torture and comment-torture file from tokens plus trivia reproduces it byte for byte — **proof:** the lossless trivia round-trip test. |
| | 4. The differential is green with counts at or above the known minimum, and red on a planted precedence swap in `cep.y` — **proof:** the differential row + its proof. |
| | 5. A stale `parser_gen.go` and a forged generated header are each red — **proof:** `freshness` proofs. |
| | 6. The instruments land with fixtures and `min_wants`, and their retroactive scan of S1's code is green — **proof:** the teeth meta-test + T2. |
| **Owner's check** | *By running:* a deliberately broken rule file through the parser test helper (`go test ./internal/parser -run TestOwnerCheck -args <file>`, named in the plan check) — read code, line, column and fix text. *By reading:* the messages for E-KW-002, E-PAT-001 and E-STR-001. |
| **Split line** | **S2.1a** = items 1–6 (lexer, parser, positions); **S2.1b** = items 7–9 (differential, fuzz, instruments). |

#### S2.2 — The validator, expression typing and the callable registry

| | |
|---|---|
| **Goal** | Every corpus program validates clean, every reachable [static] code fires on its own minimal program with the catalogued fix text, every expression form in SPEC §5.10 is typed, and the scalar expression forms are compiled into executable trees. |
| **Routine · size** | Full loop · 13 SP |
| **Depends on** | S2.1 |
| **In scope** | 1. `internal/validate`: `catalog.go` (every [static] code with fix text); one check per code; `${param}` resolution; the connector annotation table `connectors.go`; **the stream-graph checks E-GRAPH-001/002/003 and the derived-stream-consumption code live here** (the catalog's owner; ADR-0012 §1 is annotated on approval and `plan` consumes the validated graph); E-TTL-002 for pattern bounds and rate intervals; the E-NULL lints; E-REGEX-001's pre-scan. Pinned at this sprint's plan check, each with a named testdata program: whether a stream with no `@source` punctuates (E-WIN-002 for 04's `LoadedRecords`); how a `string` param becomes a list (05's `${risk_terms}`). · 2. Static test material in `internal/validate/testdata/` — one minimal program per reachable [static] code, **including the derived-stream code**. · 3. `internal/expr` (ADR-0011): **typing for every SPEC §5.10 form** — scalar, `in`, text operators, `chain`, `count` — exported to `validate`; **executable nodes only for** scalar, arithmetic, comparison, boolean, null handling and the functions `coalesce`, `minimum`, `maximum`, `abs`, `math:round` (half-up, per Appendix F — not Go's `math.Round`), `cast`, `ifThenElse`. The `in` nodes land in S3.1, `chain` in S4.1, text nodes in S7, `count` in S8. · 4. Harness ports: the 1.1 null-strict check's scalar assertions, RT-14, "9 · TTL deploy check", RT-07 (pattern and rate parts). · 5. Instruments: `docsync` (b) catalog, `docsync` (c) registry, `allocgate`. |
| **Not in scope** | E-EXP-001 → S3.3 (needs the expect sandbox). E-TXT-002 → S7. E-WIN-003 and the cron part of E-TTL-002 → S9.1. Each is a visible `skip: not-implemented(<sprint>)`. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Each reachable [static] code fires on its testdata program with its fix text, line and column; no corpus example raises any code — **proof:** the validator table test + the corpus-clean test. |
| | 2. `docsync` (b) and (c) go red on a planted drift — **proof:** their fixtures. |
| | 3. Every E-TYPE-001 row of SPEC §5.10 has a table test — **proof:** `internal/expr` typing tests. |
| | 4. Every function evaluates per Appendix F: `math:round` at ±0.5 and ±1.5; `cast` success and failure per target (a failure is a counted runtime error, never a panic); integer `/` and `%`; `minimum`/`maximum` over timestamps and durations; `ifThenElse` with a null condition — **proof:** the per-function evaluation table, expected values written before the code. |
| | 5. Scalar evaluation allocates nothing, except nodes listed as counted exclusions (`cast` to string) — **proof:** `allocgate`. |
| **Owner's check** | *By reading:* the fix texts of five codes the owner picks. *By running:* one invalid program per picked code through the validator test helper. |
| **Split line** | **S2.2a** = `internal/expr` (item 3) + `docsync` (c) + `allocgate`; **S2.2b** = `internal/validate` (items 1, 2, 4) + `docsync` (b). |

#### S3.1 — The clock service, tables and the playback format

| | |
|---|---|
| **Goal** | The clock service implements ADR-0004 completely — every timer kind and the discard path exercised — tables give snapshot-per-event reads, and the playback log format (events, punctuations, batch ends, table records) loads deterministically, all proven by clock-level tests whose arithmetic matches the harness. |
| **Routine · size** | Full loop (event time, watermarks, ordering, lateness) · 13 SP |
| **Depends on** | S2.2 |
| **In scope** | 1. `internal/clock`, all of ADR-0004: `StreamRegistry`, `clock.Instance`, `WMAdvance`, `clock.Driver`, one `Advance` with events → completions → discards. **The discard path is designed here:** a partial's within-deadline is scheduled with a discard timer kind that `Advance` routes to `OnDiscard`, never to `OnTimers`. · 2. **A test-double operator** that schedules every timer kind (completion, absence resolution, cron close, delay due, TTL purge) and the discard kind, so the full contract is exercised before it freezes at S3.3. · 3. `internal/metrics` counters. · 4. `internal/table` and the expression `in` / `not in` membership nodes (null-strict), completing the 1.1 check's `in` assertion. · 5. `internal/text.IngestFlags` (invisible flagging only). · 6. `internal/event/decode.go`, the schema-driven JSON decoder, with its differential test against `encoding/json` and a fuzz target written before any optimization. · 7. `internal/playback` **format layer only**: the log format — events; punctuation records; **batch-end records** `{"batch_end":"<id>","stream":"<name>"}` (SEMANTICS §7: batch boundaries are part of the replay log contract); table records applied at their file position — the loader, the virtual-clock `Driver`, the canonical output encoder. The drive loop is the runtime itself (S3.2). · 8. The `puritycheck` named exceptions for `internal/table` and `internal/event` with fixtures. |
| **Not in scope** | The drive loop, shards and routing → S3.2. Harness ports → none here (see the mapping above). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Admission strictness (an equal timestamp is admitted), the clamped-monotonic construct clock, per-stream virtual floors and idle exclusion each match the arithmetic of the harness check that pins them — **proof:** `internal/clock` unit tests, each naming its source check. |
| | 2. Through the test-double operator, every timer kind fires in due order within (b), discards arrive only in (c), and a pending completion is never discarded — **proof:** the timer-contract test. |
| | 3. A two-key registry + `Instance` trace: one key silent, its construct clock still advances through the broadcast `WMAdvance` — **proof:** the quiet-key clock test (the live, sharded version is S4.1's). |
| | 4. Loading the same log twice yields a byte-identical **canonical trace** (released events, timer fires, stream-late and construct-late counters), and a planted map-iteration order turns it red — **proof:** the trace test + its proof. |
| | 5. The decoder agrees with `encoding/json` on accept/reject and values, and keeps `int64` above 2^53 exact — **proof:** the differential test; fuzz nightly. |
| | 6. A table record changes membership results from the next event onward, never for the current one — **proof:** a table + trace test. |
| **Owner's check** | *By running:* a ten-line JSONL log through the trace helper; read the trace and the late counters. |
| **Split line** | **S3.1a** = items 1–3 (clock + metrics); **S3.1b** = items 4–8 (tables, flags, decoder, playback format). |

#### S3.2 — The runtime core: partitions, shards, routing, live and replay

| | |
|---|---|
| **Goal** | A plain filter rule wired through `internal/plan` runs live (sharded) and in playback (the same runtime with one shard, stepped synchronously) with identical per-key output, and errors, arrival time and the invisible-character flag behave exactly per SEMANTICS. |
| **Routine · size** | Full loop (concurrency, partitioned state) · 13 SP |
| **Depends on** | S3.1 |
| **In scope** | 1. `internal/partition` (ADR-0006): `Instance`, `OpState`, key extraction via a compiled `expr.Expr`, the TTL purge timer and re-arm, `LastActivity`, the state registry, the timer-derivability invariant. · 2. `internal/runtime`: one ingest/router goroutine; shards with bounded mailboxes, per-key serial; the non-blocking outbox handoff; the router servicing every handoff while blocked; sink queues **and the sink flusher goroutines, owned by the runtime** (they call a sink through an internal function type the facade adapts — runtime never imports `io`); the key-less pseudo-key; panic recovery at the shard boundary; **an injectable idle-tick source for the live driver** (tests drive idleness without sleeping). · 3. **Playback's drive loop = this runtime with one shard and a synchronous step API** (ADR-0005 §3: playback runs through the real runtime). · 4. **`time arrival`** (owner 2026-11-02): live, the router stamps each event from the runtime's clock at ingest; playback replays the recorded stamp. · 5. **`text_invisibles` engine-owned** (owner 2026-11-02): the router always sets it; a payload value is ignored, overwritten and counted. · 6. **Error semantics** (SEMANTICS §10): per-rule expression errors; the stream error policy; null key; unknown attributes skipped and counted. · 7. `internal/plan` (minimal): the `StreamNode` graph consuming the validated graph, the routing table, filter-rule operator chains. · 8. Instruments: race step + goleak proof; the metric-tripwire step with the blocking outbox ceiling; `allocgate` ingest → filter. |
| **Not in scope** | Rate limiting, `cep.Deploy` and the conformance runner → S3.3. Patterns → S4.1. Connectors → S5.1. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The same log run live (N shards) and in playback (one shard) gives identical per-key output order; S3.1's trace and table lines re-run through the real runtime — **proof:** the live/playback equivalence test. |
| | 2. Progress with a full mailbox at a shard whose handoff is pending, plus a slow sink and a bursty source — **proof:** the saturation test under `-race` and a CPU hog, **50/50 passes**. |
| | 3. Two rules on one stream: one divides by an attribute that is 0 on one event; the other still fires on that event; per-rule `errored_events` counts it; a null key, a missing non-optional attribute, an unknown attribute and a failed `cast` are each counted where SEMANTICS §10 says — **proof:** the error-semantics table test. |
| | 4. A payload without `text_invisibles` gets the computed flag; a payload saying `false` over text containing a zero-width space yields `true` and increments the override counter — **proof:** the ingest-flag test. |
| | 5. A `time arrival` stream: live events are stamped at ingest in arrival order; replaying the recorded log reproduces the same output byte for byte — **proof:** the arrival-time test. |
| | 6. One TTL timer per instance, re-armed on activity, never one per event — **proof:** the conformance case guarding the Siddhi purge defect class. |
| | 7. A planted data race turns the race step red; a planted goroutine leak turns goleak red; a single `Advance` emitting past the outbox ceiling is red — **proof:** three proof rows. |
| | 8. Zero allocations per event from ingest through filter evaluation — **proof:** `allocgate`. |
| **Owner's check** | *By reading:* the saturation test's scenario and its pass rate; the error-semantics table. |
| **Split line** | **S3.2a** = items 1, 2, 7 and their proofs (partition, runtime, plan, race/goleak, saturation); **S3.2b** = items 3–6 and the tripwire step (replay, arrival, flags, errors). |

#### S3.3 — The Deploy pipeline, rate limiting and the conformance runner — clock freeze

| | |
|---|---|
| **Goal** | A filter rule deployed through the public `cep.Runtime.Deploy` runs with its expect blocks executed as a deploy stage, rate limiting works per SEMANTICS §8, the conformance runner provably fails on wrong expectations, and the clock contract is frozen under the gate. |
| **Routine · size** | Full loop (the public Go API; changes what rules emit) · 13 SP |
| **Depends on** | S3.2 |
| **In scope** | 1. `plan`'s emission stage with **rate limiting** (`output first every`: one anchor per (rule, key) in `partition.Instance`; suppressed outputs counted). · 2. The public facade `cep` (minimal): `Runtime`, `Config`, `Deploy` (parse → validate → plan → expect sandbox → register), `AppHandle` `Start` / `Stop` (drain barrier to a fixpoint, `drained_pending`) / `Undeploy`, **`Runtime.Close` (apps stopped in reverse deploy order)**, **multiple apps per runtime**, `Input` with **`SendRow` only** (the typed `Send` needs the builder's mapping — S6.1), the `Table` API, `BuildInfo`. · 3. **The connector contracts as a leaf package `io` (package `cepio`)** — `Source`, `Sink`, `Mapper`, `RowWriter`, `EmitFunc`, `OutEvent`, `Mode`, and **the batch-close signal** (designed here; an API Δ) — plus `io/inmemory` (owner decision 2026-11-02: built-ins are sub-packages that import `cep` and `io`; `cep` imports `io`, never the reverse). · 4. **The expect sandbox in `internal/playback`** (ARCHITECTURE §3), called by both `cep.Deploy` and `conformance/`: a fresh sandbox per block, `given table` seeding through table records. · 5. The conformance runner `conformance/`; E-EXP-001 lifted from S2.2's skip. · 6. **Test material for 07 and 09** (QA, before the engine runs): fire-path and near-miss expect blocks — 09 gains a second event inside its rate-limit interval. · 7. The three §8 harness checks ported. · 8. Instruments: `apipartition` + `apidiff`; the two-OS determinism proof; `gate/regen-baseline` + Tier-2 baselines; the planted wrong-expect proofs; `api/clock.golden` added to `gate/law.json`. · 9. **The clock contract freezes at this sprint's G5**: ADR-0004 annotated "frozen at S3.3 exit". |
| **Not in scope** | Patterns → S4.1. HTTP and the other connectors → S5.1, S5.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 07 and 09 pass their fire-path and near-miss expects through `Deploy` — **proof:** the conformance runner. |
| | 2. Each planted wrong expect — wrong value, missing output, extra output, wrong order, an output under an empty `then`, wrong timestamp — fails `Deploy` with E-EXP-001 naming the first divergence — **proof:** six proof rows in `gate/proofs/`. |
| | 3. A second output inside the rate-limit interval is suppressed and counted; a key-less rule uses one global anchor — **proof:** the three §8 ports. |
| | 4. Two apps in one runtime share no state; `Runtime.Close` stops them in reverse deploy order — **proof:** the multi-app isolation test. |
| | 5. Two `in` predicates on one table within one event see one snapshot while a concurrent `Table.Upsert` is held between them — **proof:** the live table-snapshot test. |
| | 6. An unlisted export turns `apipartition` red; `apidiff` reports a planted signature change against the merge base — **proof:** proof rows. |
| | 7. The two-OS byte diff is green and catches a planted map-iteration nondeterminism — **proof:** the T4 proof row. |
| | 8. A planted change to an exported `internal/clock` signature appears in the LAW Δ block — **proof:** the clock-golden proof. |
| **Owner's check** | *By running:* a 20-line Go program that embeds the engine, deploys 09 and feeds events through `SendRow`. *By reading:* `api/exports.golden` — the first public surface. |
| **Split line** | **S3.3a** = items 1–3, 7 (emission, facade, contracts); **S3.3b** = items 4–6, 8–9 (sandbox, runner, test material, instruments, freeze). |

#### S4.1 — The pattern engine: the kill switch fires in playback

| | |
|---|---|
| **Goal** | The flagship kill-switch rule fires and is defeated exactly per SEMANTICS §6 — including its full evidence chain, a second session, and a quiet key in live mode — and every pattern harness check passes in Go. |
| **Routine · size** | Full loop (pattern matching) · 13 SP |
| **Depends on** | S3.3 |
| **In scope** | 1. `internal/pattern` (ADR-0013): partials, continuous absence watchers, completion timers, skip-till-next, skip-past-last earliest-wins, `every` / `once`, the per-instance freelist, `Walk`. · 2. **Logical `and` groups** (SPEC §5.2; SEMANTICS §6.1, §6.5): order-free pairs; a for-each group restarts per group completion. Their semantics have no worked example, so **new Python harness checks are written red-first in the design step** — both arrival orders, equal timestamps, an absence conjunct, `every` per group, skip-past-last contention, `once` — and any ambiguity they expose goes to the owner. · 3. The `chain` factory. · 4. Plan wiring for pattern rules. · 5. The 18 harness ports (see the mapping). · 6. **Test material (QA, before the engine runs):** 01 gains an expect with two sessions in one agent, and its fire-path expect asserts the full `evidence` value (in the expect if SPEC §6.8's record syntax can express it, otherwise a conformance case); 08 gains fire-path and near-miss expects where `sessionId == a.sessionId` can be false. · 7. **The live quiet-key case (ADR-0004 notes):** two keys, the live driver, ≥ 2 shards, the injectable idle-tick source; one key quiet after its anchor. · 8. **`drained_pending`** with a real pending absence at `Stop`. · 9. The walked-state invariant extended to partials; `allocgate` spawn; partial-count ceilings for 01 and 08. |
| **Not in scope** | The oracle and the metamorphic suite → S4.2. Live connectors → S5.1. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 01's expects pass: fire at 10:05 with the asserted evidence chain (both captures, in order, original values); defeat → nothing; the second session neither fires nor defeats the first — **proof:** the conformance runner. |
| | 2. 08's expects pass — **proof:** the conformance runner. |
| | 3. The 18 ported checks and the new and-group checks pass in Go — **proof:** `conformance/semantics_test.go`. |
| | 4. Live, with the quiet key: its absence fires at exactly its deadline and its TTL purge happens at exactly its purge time — **proof:** the live quiet-key test (asserting times, not presence). |
| | 5. `Stop` with a pending absence counts it in `drained_pending` and does not fire it — **proof:** the drain test. |
| | 6. Spawning allocates nothing once the freelist is warm — **proof:** `allocgate`. |
| **Owner's check** | *By running:* `go test ./conformance -run TestKillSwitch` (named in the plan check), then edit the approval timestamp: at 10:05:00.000 it defeats, at 10:05:00.001 it fires. *By reading:* the evidence chain. |
| **Split line** | **S4.1a** = items 1, 3–5 and 01/08's material (the core engine); **S4.1b** = items 2, 7–9 (and-groups, the live quiet key, drain, freelist). |

#### S4.2 — The pattern engine against an independent oracle and the metamorphic laws

| | |
|---|---|
| **Goal** | The pattern engine agrees with an independently written oracle on hundreds of random streams per pattern shape, and all five metamorphic laws hold — each law proven able to fail. |
| **Routine · size** | Full loop (it can only find pattern-semantics bugs, whose fixes change detections) · 8 SP |
| **Depends on** | S4.1 |
| **In scope** | 1. The brute-force reference matcher, **written by an agent other than the pattern engine's author**, plus a differential against `docs/language/checks/executor.py` on the same seeds. **Shapes:** two- and three-step chains with cross-step references; absence before and after positives; `and` groups; `every` vs `once`; skip-past-last contention; a TTL purge mid-match; two streams with idle and virtual clocks. 200–300 seeded streams per shape; branch counters with floors (`matches > N`, `defeats > N`). · 2. The metamorphic suite (qa-strategy §2), five laws with their scopes, **a scenario floor per law that includes the oracle's random multi-key streams**, and **one planted proof per law**: a wall-clock leak (time shift), a shared cross-key map (key renaming), chunk-dependent logic (batch size), a buffer that releases on arrival (admissible reorder), a side effect of unfiltered events (irrelevant insertion). |
| **Not in scope** | The cron variant of time shift → S9.1. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Engine, oracle and executor agree on every stream of every shape, counters above their floors — **proof:** the oracle test. |
| | 2. Each law is green over at least its scenario floor and red on its planted defect — **proof:** the metamorphic suite + five proof rows. |
| **Owner's check** | *By reading:* the shape list and the counters from one run. |
| **Split line** | n/a (8 SP). |

#### S5.1 — HTTP in, webhook out: the kill switch fires live

| | |
|---|---|
| **Goal** | `cepd`, started with a JSON config, receives agent events over HTTP, detects the kill-switch pattern and calls the IAM webhook with the expected evidence, safely and with retries. |
| **Routine · size** | Full loop (sink delivery) · 13 SP |
| **Depends on** | S4.2 |
| **In scope** | 1. `io/http` source (JSON per event; an NDJSON body = one source batch, **closed with the batch-close signal when the body ends**) and sink (JSON, `{{attr}}` templates, coalesced batches, exponential backoff with a cap, `sink_failures`). · 2. `io/jsonmap`, writing through `RowWriter` via a **facade decode entry** (`cep` exposes the schema-driven decoder to mappers — a named API Δ; built-ins never import `internal/*`). · 3. `io/render.go`, the shared `{{attr}}` renderer. · 4. `cmd/cepd` (minimal): rule files + `cepd.json` (param overrides **type-checked on override, E-PARAM-001**; connector settings; secrets via params or environment), `import _ "time/tzdata"`. · 5. The live end-to-end tests drive idleness through S3.2's idle-tick source. |
| **Not in scope** | stdin, SMTP, log, text mapper, `@OnError`, `/metrics`, `/healthz`, latency hooks → S5.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 01's fire scenario POSTed to the packaged `cepd` produces one POST at the stub IAM listener whose body equals S4.1's expected evidence value; the defeat scenario produces none, asserted only after a witness that the key's construct clock passed 10:05 — **proof:** the `cepd` end-to-end test. |
| | 2. S4.1's quiet-key case, run over HTTP, fires at the same time — **proof:** the live quiet-key HTTP test. |
| | 3. The renderer keeps JSON valid for attribute values containing quotes, backslashes, CR/LF, Unicode and invisible characters — **proof:** the renderer table test. |
| | 4. A failing sink retries with backoff, then counts `sink_failures`; shards never block; sources feel the backpressure — **proof:** the sink-failure test + the saturation test over HTTP. |
| | 5. A param override of the wrong type is rejected with E-PARAM-001 at start-up — **proof:** the config test. |
| **Owner's check** | *By running:* `cepd` with the named example config; send the two kill-switch events with `curl`; **the webhook stub logs the hit about one minute later** (the stream's `idle after 1 min` — stated, not a bug). *By reading:* the example `cepd.json`. |
| **Split line** | **S5.1a** = items 1–3 (connectors, renderer); **S5.1b** = items 4–5 (`cepd`, live end-to-end). |

#### S5.2 — The other connectors, error logging and observability

| | |
|---|---|
| **Goal** | The stdin source, the SMTP and log sinks and the text mapper work and are safe, `@OnError` logs each error with its cause and rule, `cepd` serves metrics and health, and the engine records per-stage latency without contention. |
| **Routine · size** | Full loop (sink delivery; hot-path instrumentation) · 8 SP |
| **Depends on** | S5.1 |
| **In scope** | 1. `io/stdin` (NDJSON; one batch per `--batch-lines` or per EOF, closed with the batch-close signal). · 2. `io/smtp` (`net/smtp` + STARTTLS; one message per event; rendered subject and body; **header-safe**: CR/LF in an attribute is encoded or refused, never written raw). · 3. `io/log` (canonical JSON). · 4. `io/textmap`. · 5. `@OnError` log (SEMANTICS §10): cause and rule id. · 6. A facade metrics-exposition method (a named API Δ); `cepd` `/metrics` (Prometheus text) and `/healthz`. · 7. **Per-stage latency hooks and per-shard histograms** (ADR-0010 §8: no locks, merged on read) — built here under the full loop so S10 only consumes them. · 8. `allocgate` rows for decode and encode. |
| **Not in scope** | The benchmark suite and published numbers → S10. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Against an in-test mail catcher on port 0: one message per event with the rendered subject and body; an `agentId` containing CR/LF cannot add a header — **proof:** the SMTP test + the header-injection rows of the renderer table. |
| | 2. The log sink prints exactly the canonical JSON — **proof:** the log-sink golden. |
| | 3. stdin replaying a log gives the same output as playback — **proof:** the stdin-vs-playback test. |
| | 4. A division-by-zero error is logged with its cause and rule id — **proof:** the `@OnError` test. |
| | 5. Histograms read concurrently with shard writes stay race-free — **proof:** the histogram test under `-race`, 50/50. |
| | 6. `/metrics` exposes the counter set; `/healthz` answers — **proof:** the `cepd` test. |
| **Owner's check** | *By running:* the named example config with the email sink; read the email in the local mail catcher. |
| **Split line** | n/a (8 SP). |

#### S6.1 — The Go builder, DSL/Go parity and the CLI

| | |
|---|---|
| **Goal** | The kill-switch rule authored with the Go builder produces a byte-identical rule model and identical playback output to the DSL (roadmap exit criterion 4), and `cep validate` / `cep test` drive the public Deploy pipeline. |
| **Routine · size** | Full loop (the public Go API) · 13 SP |
| **Depends on** | S5.2 |
| **In scope** | 1. Package `rule` (ADR-0003 §4; SPEC Appendix D): `rule.Stream[T]` with tags; **the design step lays out the whole SPEC §4.4 type-state graph** (incl. the window, emit and late branches and text operands) so S7–S9.2 add states without restructuring; implemented for every construct shipped through S5.2; `rule.Param[T]`; `Build()` returning catalog codes. **`rule → validate`** is a named import edge (settled at S1.1's plan check). · 2. **Typed input**: the generic entry point that maps `T` to `SendRow` (e.g. `cep.InputFor[T]`), with its mapping owner named in the design (Go has no generic methods; ADR-0012's `Send(ev T)` note is annotated). · 3. The parity test (ADR-0003 §5, a–e) for 01, 07, 08, 09. · 4. `cmd/cep validate` and `cmd/cep test` (thin drivers of the public Deploy; `import _ "time/tzdata"`); the rule unit-test framework (O6). · 5. `apipartition`: the `rule` exports. |
| **Not in scope** | Builder surface for text and windows → S7, S8, S9.1, S9.2. The formatter → S6.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Parity holds for 01, 07, 08 and 09 — canonical JSON equal with `ParamRef`s, equal after resolution, identical playback outputs on their expect events — **proof:** the parity test (exit criterion 4). |
| | 2. For every mandatory clause in SPEC §5.4, a builder chain that omits it does not compile — **proof:** one compile-failure row per clause. |
| | 3. `cep test` runs every expect block and exits 0; on an example with a planted wrong expect it exits non-zero naming the divergence — **proof:** the CLI end-to-end test. |
| | 4. `cep validate` on an invalid file prints code, line, column and fix text, and returns the documented exit code — **proof:** the CLI validate test. |
| **Owner's check** | *By running:* write the kill switch in Go from the package example; run `cep test` and `cep validate` on a broken file. *By reading:* the `rule` godoc. |
| **Split line** | **S6.1a** = items 1, 2, 5 (builder + typed input); **S6.1b** = items 3–4 (parity + CLI). |

#### S6.2 — `cep fmt` and the generated timing comment

| | |
|---|---|
| **Goal** | `cep fmt` rewrites any rule file into SPEC §9's canonical form, preserving every user comment and keeping a formatter-owned comment on each rule that names the stream timing it inherits, and the gate refuses unformatted files. |
| **Routine · size** | Full loop (the formatter's output is part of the language; design step runs the language-design skill) · 13 SP |
| **Depends on** | S6.1 |
| **In scope** | 1. The canonical printer `internal/parser/printer.go` from S2.1's trivia; **explicit parentheses kept as written** (so torture files keep their meaning). · 2. A facade entry `cep.Format` (a named API Δ — L5 binaries import only public packages) and `cmd/cep fmt` (rewrite; `--check`). · 3. **The generated timing comment** (owner decisions 2026-11-01): its marker and text designed here, and **the SPEC §9 sentence landing with it** ("canonical form includes the generated timing comment"). · 4. **The corpus regenerated once** — `docs/language/examples/` (and any in-scope file below) — semantically neutral by construction. · 5. **`fmtcheck`** with expected scope `git ls-files '*.cep' ':!docs/language/invalid/**' ':!docs/language/eval/generated/**' ':!**/testdata/**' ':!gate/proofs/**' ':!.claude/**'` — the invalid corpus is unparseable by design; `eval/generated/` is recorded evaluation evidence (EVAL.md) and the torture set; testdata carry position expectations; proofs are deliberately bad. |
| **Not in scope** | Format-on-save or editor plugins (later). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. For every file in `examples/`, `eval/generated/` and the torture set, and for seeded layout variants of them (whitespace added or removed, line breaks moved, comments moved between legal positions): `parse → format → parse` yields the identical model, formatting is idempotent, the multiset of user comments is preserved, and every formatted variant also passes the lark side of the grammar differential — **proof:** the round-trip property test. |
| | 2. User comments survive at their position in every position class — between filter operands, inside expect records, between the parts of a duration, a trailing `--` at end of file with no newline, a multi-line block comment — and a user comment that looks like the generated marker is preserved, not swallowed — **proof:** the position-class table test. |
| | 3. Changing any field the timing comment lists (`watermark bounded`, `ascending`, `arrival`, `idle after`) and re-formatting rewrites the comment on exactly the rules that consume that stream, including a rule that consumes two streams — **proof:** the parameterized timing-comment test. |
| | 4. On the regeneration commit, every S2.1 model golden, every grammar-differential S-expression and every conformance output is byte-unchanged — **proof:** the regeneration-neutrality check run on that commit. |
| | 5. A stream-timing edit without re-formatting turns `fmtcheck` red — **proof:** the planted stale-comment proof. |
| **Owner's check** | *By running:* mangle a rule file's layout and run `cep fmt`; change a stream's watermark and read the diff. *By reading:* the timing comment on the flagship. |
| **Split line** | **S6.2a** = items 1–2 (printer + `cep fmt` without the timing comment); **S6.2b** = items 3–5. |

#### S7 — Text operators, incl. live table term lists

| | |
|---|---|
| **Goal** | All five text-operator families evaluate exactly per SEMANTICS §5 in both case modes and both front-ends, term lists stored in a table update live, and evidence keeps the original text. |
| **Routine · size** | Full loop (changes what a rule detects) · 13 SP |
| **Depends on** | S6.2 |
| **In scope** | 1. `internal/text` (ADR-0007): NFC, `FoldSimple`, `Tokens`, Aho-Corasick, regex wrappers incl. `(?i)` for `_ci`. · 2. The expression text nodes and `raw()`. · 3. **Table operands for keyword sets, live** (owner 2026-11-02): the matcher is rebuilt when the table's version changes — never per event, never shared mutably between shards; ADR-0007 §5 annotated. · 4. The validator's E-TXT-002 (lifting S2.2's skip); `validate → text` is a named edge (S1.1's plan check). · 5. The Unicode-version agreement check; `BuildInfo`. · 6. **Test material (QA, before the engine runs):** fire-path and near-miss expects for 05 and 06 (06 includes its `once` group); 05's include a payload that claims `text_invisibles: false`. · 7. Builder text surface + parity twins of 05 and 06. · 8. The seven §5 harness ports; fold-sensitive regex cases. · 9. `allocgate` rows for text (target zero; rebuild-on-update and any other allocating path listed as counted exclusions). |
| **Not in scope** | Locale-aware or full case folding (never). LLM judgement → Phase 4. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 05 and 06 pass their expects; 05's spoofed-flag case still flags — **proof:** the conformance runner. |
| | 2. The seven §5 ports and the fold-sensitive regex cases pass — **proof:** the conformance suite. |
| | 3. Table operands × all four set families × both case modes × terms/substrings evaluate correctly, and a table record between two events changes the result from the next event onward — **proof:** the table-operand matrix test. |
| | 4. A text rule's evidence carries the original text, including an invisible character and a case variant — **proof:** an evidence case. |
| | 5. Parity twins of 05 and 06 are green; no `skip: not-implemented(S7)` remains — **proof:** the parity test; the summary. |
| **Owner's check** | *By running:* a keyword-set rule over a prompt containing an invisible character and a case variant; then update the term table and send the same prompt again. *By reading:* the evidence. |
| **Split line** | **S7a** = items 1, 2, 4, 5, 8 (operators); **S7b** = items 3, 6, 7, 9 (table operands, material, twins). |

#### S8 — Windows I: sort, batch and `count`

| | |
|---|---|
| **Goal** | Sort and batch windows emit exactly per SEMANTICS §7–§9, `count` aggregates over a closed group, and batch punctuation works from live sources as well as from replay, in both front-ends. |
| **Routine · size** | Full loop with the full blind three-reviewer design panel (owner decision 2026-11-01) · 13 SP |
| **Depends on** | S7 |
| **In scope** | 1. `internal/window`'s `Kind` interface and `Out` (ADR-0014 §1). · 2. Sort: the tie rule, changelog output, `OnPurge` retractions in total order. · 3. Batch: grouping by `BatchID` **within the key**; firing on the batch-close signal delivered by the router as a control timer; one group event at the group's max event time; **the fired-id dedup set lives per instance** (ADR-0014 §5; ADR-0006 §2(b) annotated on approval), TTL-purged with it. · 4. **`count`** (registry `Aggregate` kind) evaluated over a closed group, respecting E-SEL-002; reused by cron in S9.1. · 5. **Test material (QA, before the engine runs):** fire-path and near-miss expects for 03 (its `+`/`−` rows in a conformance log if an expect cannot express them) and for 04's batch rule. · 6. Builder window surface for sort and batch + the twin of 03. · 7. The five harness ports (see the mapping). · 8. The walked-state invariant for window state; `allocgate` rows. |
| **Not in scope** | Cron → S9.1; delay → S9.2. The time-based window catalog → Phase 2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 03 and 04's batch rule pass their cases — **proof:** the conformance runner (+ 03's log). |
| | 2. The five ports pass — **proof:** the conformance suite. |
| | 3. An NDJSON body POSTed to `cepd`, and the same records through stdin, produce the playback result — **proof:** the live batch test. |
| | 4. A redelivered batch id emits nothing — **proof:** the RT-21 port. |
| | 5. The twin of 03 is green — **proof:** the parity test. |
| **Owner's check** | *By running:* a leaderboard rule with a tie. *By reading:* the changelog rows. |
| **Split line** | **S8a** = sort (items 1, 2, 03's material and twin); **S8b** = batch + `count` (items 3, 4, 04's material, the live batch test). |

#### S9.1 — Windows II: cron

| | |
|---|---|
| **Goal** | Cron windows emit exactly per SEMANTICS §7 with SPEC's dialect — including daylight saving — implemented in-repo, late events handled exactly as rev 3 pins them, and live firing driven by idleness, in both front-ends. |
| **Routine · size** | Full loop with the full blind three-reviewer design panel · 13 SP |
| **Depends on** | S8 |
| **In scope** | 1. `internal/window/cron.go` (ADR-0014 §3, §6): the 5-field Vixie parser; next-fire by field-wise search in the rule's timezone; `time.LoadLocation` over embedded tzdata. · 2. **Enumerated DST cases, expected UTC instants derived with Python `zoneinfo` before the Go code:** the spring gap and the autumn fold for `0 2 * * *` in America/New_York; Australia/Lord_Howe's 30-minute shift; a southern-hemisphere zone; a day-of-week-restricted expression across a transition. · 3. The `windowspec` helper for the validator: E-WIN-003 and the cron part of E-TTL-002 (lifting S2.2's skips) — **the 10-year extent scan reads its start instant from an injected clock, pinned in tests**, never the wall clock. · 4. Buckets with `count` (from S8), the close timer at `bᵢ₊₁`, a single fire — **no allowance** (rev 3). · 5. **Test material (QA, before the engine runs):** fire-path and near-miss expects for 02, including an event behind the admission watermark whose bucket is still open (dropped and counted, per rev 3). · 6. Live cron close through the idle-tick source. · 7. The time-shift law's cron variant. · 8. Builder cron surface + the twin of 02. · 9. The cron harness port; RT-07's window-extent assertions; the boundary property test; the walked-state invariant for buckets. |
| **Not in scope** | Delay → S9.2. `late events allow` → Phase 2 (owner 2026-11-02). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 02 passes its cases, incl. the admission-drop case — **proof:** the conformance runner. |
| | 2. Every enumerated DST case yields its pre-derived UTC instants; every rejected form returns E-WIN-003 — **proof:** the cron table tests. |
| | 3. The extent scan gives the same answer whatever day the test runs — **proof:** the scan test with two different injected start instants. |
| | 4. A live cron bucket closes through idleness at its boundary — **proof:** the live cron test. |
| | 5. The cron time-shift variant is green; the twin of 02 is green; no `skip: not-implemented(S9.1)` remains — **proof:** the metamorphic suite; the parity test; the summary. |
| **Owner's check** | *By running:* a daily-digest rule across a DST change in a named timezone. *By reading:* the bucket boundaries. |
| **Split line** | **S9.1a** = items 1–3 (parser, DST, validator); **S9.1b** = items 4–9 (windows, live, laws, twin). |

#### S9.2 — Windows III: delay

| | |
|---|---|
| **Goal** | Delay windows re-time events exactly per SEMANTICS §7, and 04 passes as a whole, in both front-ends. |
| **Routine · size** | Full loop with the full blind three-reviewer design panel · 8 SP |
| **Depends on** | S9.1 |
| **In scope** | 1. Delay: a due-ordered queue; each event re-stamped `t + D`. · 2. **Test material (QA):** fire-path and near-miss expects for 04's delay rule, plus a delay case across a TTL purge. · 3. Builder delay surface + the twin of 04. · 4. The delay harness port; the walked-state invariant for the queue; `allocgate`. |
| **Not in scope** | Rule-consumption of a delay's output (derived stream) → Phase 2 (RT-09). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 04 passes all its cases, incl. the TTL-purge case — **proof:** the conformance runner. |
| | 2. The delay port passes; the twin of 04 is green — **proof:** the conformance suite; the parity test. |
| **Owner's check** | *By running:* a delay rule with one out-of-order event. |
| **Split line** | n/a (8 SP). |

#### S10 — Demo kit, quickstart, benchmarks and the published latency

| | |
|---|---|
| **Goal** | A newcomer runs the kill-switch demo from the public quickstart in under 15 minutes, the demo is checked automatically, and detection latency is measured with an open-loop benchmark and published with its span, hardware and method. |
| **Routine · size** | Lighter (no change to what any rule detects or emits; the hot-path hooks were built under the full loop in S5.2) · 13 SP |
| **Depends on** | S9.2 |
| **In scope** | 1. The `bench/` suite (ADR-0010 §8), consuming S5.2's hooks: scenario generators in `bench/gen/`; an open-loop load generator; per-stage and end-to-end percentiles. · 2. `benchstat` baselines + CI tripwires. · 3. `demo/`: the compose kit (traffic generator sharing `bench/gen/`, stub IAM endpoint, mail catcher) **and a nightly job that brings it up and asserts both the stub and the mail catcher receive the kill switch**. · 4. The quickstart. · 5. Docs for the shipped subset only, with the honest limits (no restart survival; at-least-once; per-agent only; no `late events allow` until Phase 2). · 6. The published latency page. |
| **Not in scope** | Further optimization. Fleet-wide claims (Phase 2–3). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block (lighter), plus: |
| | 1. The owner completes the quickstart cold in under 15 minutes — **proof:** the timed run recorded in the sprint record (exit criterion 1). |
| | 2. Two spans published (exit criterion 2): **engine detection latency** — from the router accepting the event (or idle tick) that makes the match due to the kill-switch event entering the sink queue — with p50/p99/p99.9/max **and pass/fail against O3's sub-millisecond p99**; and **end-to-end** — through the IAM webhook's response — reported with hardware and method — **proof:** the bench report from the suite. |
| | 3. A planted 10× regression turns the benchmark tripwire red — **proof:** a gate proof. |
| | 4. Every documented construct links to a conformance case **whose rule model uses that construct** — **proof:** the docs-to-conformance check. |
| | 5. The compose-kit job is green and goes red when the stub IAM endpoint is removed — **proof:** the nightly job + its proof. |
| **Owner's check** | *By running:* the quickstart on a clean machine with a timer. *By reading:* the limits page and the latency page. |
| **Split line** | **S10a** = items 1, 2, 6 (benchmarks + latency); **S10b** = items 3–5 (demo, quickstart, docs). |

#### S11 — The attack gallery: seven examples

| | |
|---|---|
| **Goal** | Every attack the attack-landscape review classes as expressible in the MVP language — A2 EchoLeak, A4 AgentFlayer, A5 GitHub MCP toxic flow, A6(a) malicious MCP server (static form), A7 Nx s1ngularity, A10 CoPhish, A13 the third-party AI-tool breach — ships as a documented, cited, runnable rule app that provably detects its attack and provably ignores a near miss, in the DSL and the Go builder. |
| **Routine · size** | Lighter (consumes shipped constructs only) · 13 SP |
| **Depends on** | S10 |
| **In scope** | 1. `gallery/<nn>-<slug>/` × 7: a README (plain-English story; citation; **a class tag matching the review's tally**; an honest note on the Phase-3 telemetry real deployments need), a synthetic timeline, the rule, its Go twin, its playback log. Stories are written inside this sprint. · 2. **Per app, written from the README's timeline before the engine runs: ≥ 1 non-empty fire-path expect naming the attributes it asserts, and ≥ 1 benign near-miss expect.** · 3. The conformance runner executes every app; the parity test covers every twin; the gallery joins the grammar-differential corpus (ADR-0002 §5 annotated on approval) and `fmtcheck`. · 4. Attacks the review classes P2, P3 or inexpressible listed as "coming", never faked. · 5. **An engine defect found here** is not fixed under the lighter routine: it goes to a full-loop fix (the owner may open a sprint) or to known issues with the owner's consent. |
| **Not in scope** | Any construct not already shipped. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block (lighter), plus: |
| | 1. All seven apps pass their fire-path and near-miss expects in CI, each with a green twin, in the differential and `fmtcheck`-clean — **proof:** the conformance runner, the parity test, the differential, `fmtcheck`. |
| | 2. Every README's class tag matches the review's tally table — **proof:** a script checking the tags against `docs/language/review/attack-validation.md`'s expressibility tally. |
| | 3. Every citation link resolves to an in-repo anchor (no network in the gate) — **proof:** the in-repo link check. |
| **Owner's check** | *By reading:* the seven stories for accuracy and defensive framing. *By running:* one gallery app through `cep test`. |
| **Split line** | **S11.1** = four apps (A2, A4, A5, A10); **S11.2** = the other three. S11.1 alone does not meet exit criterion 5 (≥ 5), so S12 waits for S11.2. |

#### S12 — Release close-out: whole-engine break-it and `v0.1.0`

| | |
|---|---|
| **Goal** | The whole engine survives a whole-engine break-it with every finding dispositioned, all five roadmap exit criteria are proven by their named measurements, and the owner tags `v0.1.0`. |
| **Routine · size** | Full loop + the whole-engine break-it (sprint model §14) · 8 SP |
| **Depends on** | S11 |
| **In scope** | 1. The whole-engine break-it (QA + the-fool): out-of-order and late events, equal timestamps, `within` exactly at its bound, absence timers at the edge, partition-key explosion, malformed input, sink failures, races, invariant violations; the sampled constant-flip spot-check (ADR-0015 §4). · 2. One fix round; every breaker's red test committed. · 3. `docsync` (a) reverse direction and the §7 file-existence check switched on; every open skip red. · 4. The release workflow (tag build with provenance); `docs/releases/v0.1.0.md`. · 5. Docs final pass. · 6. Roadmap and this plan annotated: what shipped, what moved, why. · 7. The public-repo step (owner decision). |
| **Not in scope** | Any new construct. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Each of the five exit criteria is proven by its named measurement — **proof:** the exit-criteria table in the release notes, each row linking its proof. |
| | 2. Every break-it finding is fixed with a test, forwarded with the owner's consent, or rejected with a reason — **proof:** the break-it report. |
| | 3. T4 green on the tag candidate; main green after the merge — **proof:** CI runs. |
| | 4. The owner tags `v0.1.0` — **proof:** the tag and its release build. |
| **Owner's check** | *By running:* the quickstart against the tagged build. *By reading:* the release notes and the limits page. *By hand:* the tag. |
| **Split line** | n/a (8 SP). |

### Still open for the owner

- *Approval of the decomposition — **done** (2026-11-02). The plan-derived annotations it triggered are applied: ARCHITECTURE §3 and §7; ADR-0002 §5; ADR-0004; ADR-0005 §1; ADR-0006 §2(b); ADR-0012 §1 and its `Send(ev T)` note; ADR-0015 §2.*
- **At S1.1's plan check:** whether ARCHITECTURE §3's per-package import lists are exhaustive (and the edges listed in architecture review DA-11); the placement of `conformance/` and `bench/` in the layer model; the linter version, the Go version pin, hook wiring; the Go specialist agent; branch protection (owner-side).

*Each sprint gets a card in this section ([template](templates/sprint-card.md)). Once its plan check is answered, a short "how it runs" note follows the card: lanes, waves, gates.*

### Status

**Architecture baseline RATIFIED and review-hardened (2026-11-01). Phase 1 decomposition APPROVED (2026-11-02): 20 sprints, 240 SP, reviewed by architecture + QA (57 findings, all dispositioned) → next: S1.1's plan check (G0 for the first sprint).**

**This line is the single prose status surface for the project.** The README and the roadmap defer to it; everything finer-grained lives in the source and the git log, which are authoritative if they disagree with this.

---

## 4. Later phases

Each phase is decomposed into sprints when the phase before it nears its exit. The scope of each is in the [roadmap](../product/02-roadmap.md).

| Phase | Title | Unblocked by |
|---|---|---|
| 2 | Correctness & durability breadth (snapshots first) | Phase 1's engine skeleton |
| 3 | Detection pack v1 & agent telemetry | Phase 2's windows and joins (partly parallel) |
| 4 | LLM-in-the-loop (FA14) | Phase 2's async plumbing + Phase 3's pack |
| 5 | OSS launch & extensibility | SPI stability from Phases 2–4 |

---

## 5. Decisions

*Owner decisions that change this plan are recorded here as dated sections ("Owner decisions, YYYY-MM-DD — <occasion>"), numbered, with "≠ R" where the answer differs from the recommendation. A changed decision is annotated, never rewritten.*

### Owner decisions, 2026-11-01 — Phase 1 architecture ratification

1. **Architecture baseline ratified with conditions** (docs/architecture/, 14 ADRs *(annotation 2026-11-01: written before ADR-0015 was authored the same day; the baseline is 15 ADRs — ground rule 4, annotated not rewritten)*, three review rounds): the ratification binds three additions before S1 — (a) every ARCHITECTURE.md constraint and named code smell is **enforced at build time** (violation = failed build, starting at the pre-commit hook); (b) a QA and test-automation strategy that prevents architectural debt; (c) the build is **hostile** — effectively impossible to code against by hand, modeled on enforcement machinery the owner has proven in prior projects (studied, not named). Tracked as ADR-0015 + docs/development/qa-strategy.md.
2. **Dependencies approved:** `golang.org/x/tools` (goyacc, build-time), `golang.org/x/text` (runtime), `golang.org/x/perf` (benchstat, build-time). Everything else in-repo per ADR-0009.
3. **cepd config = JSON** (stdlib-only; roadmap's draft YAML amended).
4. **SPEC v0.1 rev 2 approved:** expect blocks gain `given table <Name> [rows]`; regex `_ci` defined as RE2 folding. Applied, grammar re-verified green.

### Owner decisions, 2026-11-01 — foundation adversarial review (gate red-team + consistency review)

Round-4 review of ADR-0015 + qa-strategy.md + post-ratification drift: 49 verified findings (15 blocker / 25 should-fix / 9 later), all dispositioned in [gate-redteam.md](../architecture/review/gate-redteam.md#dispositions-2026-11-01-post-review-revision--every-finding-per-ground-rule-16) and [foundation-consistency.md](../architecture/review/foundation-consistency.md#dispositions-2026-11-01-post-review-revision--every-finding-per-ground-rule-16); none rejected.

1. **Derived streams deferred:** v0.1 rejects a rule consuming another rule's output stream (silent-drop watermark bug otherwise; nothing in the MVP uses chaining). The min-over-live-instances merge is recorded as the Phase-2 design (ADR-0004 §6). The error code lands in the **spec rev-3 batch at S1's plan check**, together with the forwarded RC8.1 code (cross-stream partition-key type mismatch) — the one open item.
2. **Dependencies approved (≠ R):** `go.uber.org/goleak` (test-only, MIT) and `golang.org/x/exp/apidiff` (build-time, BSD-3); `x/tools` scope widened to go/analysis + analysistest. Recommendation was in-repo equivalents; owner chose the proven tools. ADR-0009 amended.
3. **Hostility bar reworded + residue accepted:** the bar is "every freestyle shortcut is either mechanically red or a named loud row at merge" (not "impossible to code against"); the seven review-only constraints are recorded verbatim in ADR-0015 §3.6 and accepted knowingly.
4. **LAW Δ adopted:** `gate/law.json` manifest + a mandatory LAW Δ block at the top of every gate summary + the standing owner-check line "LAW Δ block read and intended" (sprint model §8 step 10). Closes the review's root finding: gate law is agent-writable, so law changes must be structurally loud at merge.

### Owner decisions, 2026-11-01 — Phase 1 decomposition (scoping choices; the decomposition as a whole is not yet approved)

1. **`cep fmt` stays in Phase 1 (≠ R).** Recommendation was to move the formatter to the backlog and surface inherited stream timing through `cep validate` instead; the owner kept the full formatter. **Placed in S6 with the CLI it belongs to** (recommended, accepted) — S2.1 is at the cap; S2.1's lexer keeps comments as positioned trivia so the printer needs no lexer rework.
2. **The formatter always writes the generated timing comment** on every rule (recommended, accepted): the 9 corpus examples are regenerated once by `cep fmt` in S6 (a spec-corpus change, LAW Δ-visible); a new gate row `fmtcheck` makes a stream-timing edit without a re-format red. The SPEC §9 clarification joins the spec rev-3 batch at S1's plan check; the comment's exact format is designed in S6.
3. **Kill-switch vertical first** (recommended, accepted): S4 patterns → S5 live → S6 parity, then text and windows.
4. **Full three-reviewer design panel for every window sprint (≠ R).** Recommendation was one breaker for sort/batch/delay with the panel for cron only; the owner chose maximum scrutiny for all four kinds (S8, S9).
5. **Attack gallery: "six or more" (≠ R).** Recommendation was exactly five. Applied as **all seven attacks the attack review classes MVP-expressible** (A2, A4, A5, A6(a), A7, A10, A13) — *the coordinator's reading of "six or more"; the owner may correct it*. S11 grows to 13 SP with a split line.

*Annotations 2026-11-02 (decomposition v2, after the architecture and QA reviews — placement only; the decisions stand):* decision 1's "S6" is now **S6.2** (S6 split into S6.1 builder/CLI and S6.2 formatter). Decision 2's SPEC §9 sentence moves from the S1 rev-3 batch to **S6.2**, landing together with the comment's format and the corpus regeneration (architecture review DA-19); `fmtcheck`'s scope excludes the unparseable invalid corpus, recorded evaluation files, testdata and proofs (DA-06/DQ-12). Decision 3's "S4 → S5 → S6" is now S4.1/S4.2 → S5.1 → S6.1. Decision 4's "S8, S9" is now S8, S9.1 (cron) and S9.2 (delay).

### Owner decisions, 2026-11-02 — decomposition review (architecture + QA reviews of v1)

57 findings (architecture 22 — 7 blocker / 10 should-fix / 5 later; QA 35 — 15 / 17 / 3), all verified against the documents and dispositioned in [phase1-decomposition-architecture.md](../reviews/phase1-decomposition-architecture.md#dispositions-2026-11-02--every-finding-per-ground-rule-16) and [phase1-decomposition-qa.md](../reviews/phase1-decomposition-qa.md#dispositions-2026-11-02--every-finding-per-ground-rule-16). Six needed the owner; all six answers matched the recommendation.

1. **`io` split** (recommended, accepted): `io` (package `cepio`) becomes a leaf package holding only the connector contracts; each built-in connector is its own sub-package (`io/http`, `io/stdin`, `io/inmemory`, `io/smtp`, `io/log`, `io/jsonmap`, `io/textmap`) importing `cep` and `io`. `cep → io` is an edge; `io → cep` is not — the Go import cycle (DA-04) is gone. Sink flusher goroutines move into `internal/runtime`. ARCHITECTURE §3, ADR-0001 and ADR-0008 annotated.
2. **Example test material inside the example files** (recommended, accepted): each example gains fire-path and near-miss expect blocks, authored by QA in the sprint that turns it green, outputs derived independently before the engine runs; a spec-corpus change visible in LAW Δ.
3. **`late events allow` removed for v0.1** (recommended, accepted): digests drop late events like every rule; per-window late handling arrives in Phase 2. Spec rev 3 (S1.2) revises E-LATE-002 and example 02, and pins cron admission to SPEC §6.1 — the Python reference executor, which admitted late events into open buckets, is aligned and a red-first harness check added.
4. **Keyword sets over a table operand supported, live** (recommended, accepted): the matcher rebuilds when the table changes; the next event sees the new terms (S7). ADR-0007 §5 annotated.
5. **`text_invisibles` is engine-owned** (recommended, accepted): the engine always computes it; a payload value is ignored, overwritten and counted — closes the spoofing bypass without letting an attacker get events dropped. Spec rev 3 (S1.2); built and tested in S3.2, re-proven in S7.
6. **`time arrival` supported** (recommended, accepted): the engine stamps events at ingest; playback replays the recorded stamp (S3.2).

### Owner decisions, 2026-11-02 — decomposition approval

1. **Phase 1 decomposition v2 approved** (recommended, accepted): 20 sprints, 240 SP, forecast 40–60 weeks, re-forecast at S1.2's and S3.3's G5. Sprint model §9 phase entry 1 holds; each sprint still passes its own G0.
2. **Attack gallery = seven examples confirmed** (recommended, accepted): the coordinator's reading of 2026-11-01 decision 5 ("six or more") is confirmed — all seven MVP-expressible attacks.
3. **Partition keys of different types across streams are rejected at deploy** (recommended, accepted — pre-mortem RC8.1, the last open rev-3 item): the error names both streams and both types; a text `"5"` and a number `5` never silently form two instances. Lands in spec rev 3 (S1.2) and the validator (S2.2).
4. **Commit and push** the plan, the reviews and the annotations (recommended, accepted).
