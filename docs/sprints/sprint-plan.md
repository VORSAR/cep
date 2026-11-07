# Sprint Plan — complex-events-engine

**Document ID:** SPRINT-PLAN-001
**Version:** 2026-11-06 — the live plan only: closed sprint cards and the old status lines moved to [history.md](history.md), owner decisions to [decisions.md](decisions.md), word for word. *(Earlier: the skeleton, 2026-10-31; Phase 1's decomposition v3, approved 2026-11-02. This document replaces the `docs/product/03-sprint-plan.md` named in the objectives and the roadmap.)*
**Status:** Active sprint schedule. Governed by [00-sprint-model.md](00-sprint-model.md), the development process every sprint runs. The objectives say **why** ([01-objectives](../product/01-objectives.md)), the roadmap says **in what order** ([02-roadmap](../product/02-roadmap.md)), and this plan says **which sprint runs next**, with its size, scope and dependencies.

---

## 1. Strategic frame

The roadmap's MVP strategy (owner, 2026-10-31): one real use case, **the Agent Kill Switch**, built as a vertical slice through every layer it needs, then breadth. Three rules from the objectives shape every sprint here:

- **Tests are the spec.** The gate, playback mode and the conformance harness come before the operators they test.
- **Work decomposes.** Each sprint is a set of lanes an agent can finish and a test can prove, with the owner's review at the seams.
- **Scope discipline is structural.** Anything not serving the current phase's exit criteria waits (the core / diff / later tiers).

---

## 2. Where we are today

> **The one status line is [§Status](#status); the code and git are authoritative.** Closed sprints: the [records index](README.md#records) and their cards in [history.md](history.md). What is left: the open cards in §3, from S3.3 on, and the items in [Still open for the owner](#still-open-for-the-owner).

---

## 3. Phase 1 — MVP: the Agent Kill Switch slice

> **Exit criterion:** the roadmap's five ([roadmap — exit criteria](../product/02-roadmap.md#exit-criteria-definition-of-shipped)): (1) the demo runs end to end from the public quickstart in under 15 minutes; (2) conformance and unit tests green for every shipped construct, with detection → kill-switch latency measured and published; (3) docs for the shipped subset only, tagged `v0.1.0`, public repo; (4) the same detection authored in the DSL and the Go API, with identical behaviour proven by one test; (5) the attack gallery: ≥ 5 cited, incident-inspired examples, runnable via playback, green in CI. *(Annotated 2026-11-01: this line said "four" — criterion 5 was added to the roadmap on 2026-10-31.)*

**Phase entry** ([sprint model §9](00-sprint-model.md#9-phases-entry-and-exit)): the architecture baseline written and reviewed (architect + the-fool), ADRs in `docs/architecture/`; Phase 1 decomposed into sprint cards the owner approved. **Phase exit:** the five criteria above, plus a whole-engine break-it (`v0.1.0` is a release).

**Execution order:** enablers first. The gate and playback mode come before the pattern operators ([roadmap — the slice](../product/02-roadmap.md#everything-required-the-slice-by-feature-area)).

### Phase 1 decomposition — v3, recalibrated to the 13-point cap (2026-11-02)

> **Status of this section.** v2 was approved by the owner on 2026-11-02. At the owner's request the same day, v3 recalibrates it: every card is re-estimated item by item and no card holds more than 13 points. **v3 changes only how the work is cut into sprints — no item was added or removed** except the two the owner added after v2's approval (coverage at 90% in S1.1, the parallel runner). **v3 is APPROVED** (owner, 2026-11-02 — §5 "Owner decisions, 2026-11-02 — recalibration (v3)"). Each sprint still passes its own G0.

**How the points are calibrated.** Story points measure **build work** — the code, its tests and its test material — estimated item by item (each item's estimate is in brackets) against two anchors: **`internal/event` = 1 point; the pattern engine's core (partials, absence watchers, completion timers, the skip rules) = 5 points.** The fixed process every sprint carries — plan check, reviews, break-it, fix round, owner's check, and the window sprints' design panels — is not in the points; the 2-week cap holds it. A sprint holds **at most 13 points** of estimated build work; each card shows its item total and its size rounded up to the Fibonacci scale. v2 sized cards by judgment; counted item by item, ten of v2's twenty cards were over 13, so v3 re-cuts them — splitting where a coherent half exists (new ids carry a letter, e.g. S1.1a/S1.1b, so references to v2's ids elsewhere stay valid) and otherwise moving an item to the neighbour it belongs with.

**How to read the numbering.** ADR-0005 §1's stages keep their numbers; a stage that does not fit one sprint is split before entry (sprint model §3): **S1 = S1.1a + S1.1b + S1.2; S2 = S2.1 + S2.2a + S2.2b; S3 = S3.1 + S3.2a + S3.2b + S3.3; S4 = S4.1 + S4.2; S5 = S5.1 + S5.2; S6 = S6.1 + S6.2; S9 = S9.1 + S9.2.** Every ADR reference to a stage means the stage; the cards name the exact sprint. The clock contract freezes at **S3.3's** G5 ("S3's exit", ADR-0005 §1). **Sprint-id grammar** (settled in S1.2, used by `todocheck`, `suppressions` and conformance skips): `S<stage>[.<part>][<letter>][-<n>]` (the `-<n>` suffix only when a plan check applies a card's split line), ordered by this table's row order; "current + 2" means two rows down. *(Annotated 2026-11-02: S1.1a's plan check applied a split — S1.1a = S1.1a-1 + S1.1a-2 — so S1 = S1.1a-1 + S1.1a-2 + S1.1b + S1.2.)*

**Sequencing principle (owner decision 2026-11-01).** Enablers first (S1.1a–S3.3). Then the **kill-switch vertical**: patterns (S4.1, S4.2) make the flagship fire in playback; connectivity (S5.1) makes it fire live through `cepd`; the Go builder (S6.1) proves DSL/Go parity on it. Breadth follows: the formatter (S6.2), text (S7), windows (S8, S9.1, S9.2), the demo kit and published latency (S10), the attack gallery (S11), the release close-out (S12). **From S6.1 onward every construct ships in both front-ends in the sprint that ships it** (builder surface + a parity twin per example).

*✓ = merged and closed: the link is its record; its card is in [history.md](history.md#closed-sprint-cards).*

| Sprint | Title | SP (items) | Routine | Depends on |
|---|---|---|---|---|
| ~~S1.1a~~ | ~~The gate runner, CI, coverage and the event kernel~~ *split at its plan check, 2026-11-02 (owner, §5): re-sized for the owner's Python decision to about 15 points → S1.1a-1 + S1.1a-2* | ~~13 (12.5)~~ | full | — |
| S1.1a-1 [✓](s1-1a-1-record.md) | The local gate, coverage and the event kernel | 13 (11.5) | full | — |
| S1.1a-2 [✓](s1-1a-2-record.md) | Hooks and CI | 5 (3.5) | full | S1.1a-1 |
| S1.1b [✓](s1-1b-record.md) | The analyzers and the law | 13 (13) | full | S1.1a-2 *(was S1.1a; annotated 2026-11-02)* |
| S1.2 [✓](s1-2-record.md) | The model kernel, the remaining gate rows and spec rev 3 | 13 (11) | full | S1.1b |
| S2.1 [✓](s2-1-record.md) | The parser | 13 (13) | full | S1.2 |
| S2.2a [✓](s2-2a-record.md) | The grammar differential and the expression engine | 13 (10.5) | full | S2.1 |
| S2.2b [✓](s2-2b-record.md) | The validator | 13 (11.5) | full | S2.2a |
| S3.1 ✓ ([S3.1-1](s3-1-1-record.md), [S3.1-2](s3-1-2-record.md)) | The clock service, tables and the decoder | 13 (13) | full | S2.2b |
| S3.2a ✓ ([S3.2a-1](s3-2a-1-record.md), [S3.2a-2](s3-2a-2-record.md)) | The replay format and the runtime core *(split at its plan check, 2026-11-05 (owner, §5): S3.2a-1 = items 1, 2, the format layer and partitions; S3.2a-2 = item 3, the runtime core)* | 13 (11.5) | full *(S3.1's routine, owner 2026-11-05)* | S3.1 |
| S3.2b ✓ ([S3.2b-1](s3-2b-1-record.md), [S3.2b-2](s3-2b-2-record.md)) | One runtime, live and replay — rate limits, errors, arrival time *(split at its plan check, 2026-11-06 (owner, §5): S3.2b-1 = items 1, 2, 6, 9, the plan, the replay loop, rate limits and the material; S3.2b-2 = items 3–5, 7, 8 and the recorder)* | 13 (12.5) | full *(S3.1's routine, owner 2026-11-06)* | S3.2a |
| S3.2c ✓ ([record](s3-2c-record.md)) | The spec cleanup: the five contradictions in the language documents (known issue S2.2b-1), the grouped-key evaluation's two notes, integer overflow as an expression error (S3.2b-1-1) *(placed here by the owner, 2026-11-06: [decisions](decisions.md#owner-decisions-2026-11-06--the-open-items-after-s32b-2))* | 3 (2.5) *(4 (3.5), owner, 2026-11-07)* | tests first, one review, a break-it *(owner, 2026-11-06)* | S3.2b |
| S3.3 | The Deploy pipeline and the conformance runner — clock freeze | 13 (13) | full | S3.2b, S3.2c *(owner, 2026-11-06: the cleanup first)* |
| S1.1c | The gate extras moved from S1.1b and S1.2 *(placed here by the owner, 2026-11-05, §5: after the clock freeze, before the pattern engine)* *(and the one-module check of ADR-0001 §1: [owner, 2026-11-06](decisions.md#owner-decisions-2026-11-06--the-docs-restructure))* *(and a test that parses every rule fragment SEMANTICS and SPEC quote, about 0.5: [owner, 2026-11-07](decisions.md#owner-decisions-2026-11-07--s32cs-close-out))* | sized at its G0 | at its G0 | S3.3 |
| S4.1 | The pattern engine — the kill switch fires in playback | 13 (11) | full | S3.3 |
| S4.2 | The pattern engine completed and independently proven | 13 (13) | full | S4.1 |
| S5.1 | HTTP in, webhook out — the kill switch fires live | 13 (11.5) | full | S4.2 |
| S5.2 | The other connectors, error logging and observability | 13 (9) | full | S5.1 |
| S6.1 | The Go builder, DSL/Go parity and the CLI | 13 (13) | full | S5.2 |
| S6.2 | `cep fmt` and the generated timing comment | 13 (12.5) | full (language) | S6.1 |
| S7 | Text operators, incl. live table term lists | 13 (13) | full | S6.2 |
| S8 | Windows I — sort, batch and `count` | 13 (12) | full, three-reviewer panel | S7 |
| S9.1 | Windows II — cron | 13 (12) | full, three-reviewer panel | S8 |
| S9.2 | Windows III — delay, and the cron/delay builder twins | 8 (6.5) | full, three-reviewer panel | S9.1 |
| S10 | Demo kit, quickstart, benchmarks and the published latency | 13 (10) | lighter | S9.2 |
| S11 | The attack gallery — seven examples | 13 (12) | lighter | S10 |
| S12 | Release close-out — docs, whole-engine break-it and `v0.1.0` | 13 (12.5) | full + whole-engine break-it | S11 |

**Forecast (honest, re-forecast at named points).** **23 sprints; about 270 points of estimated build work** (294 in Fibonacci card sizes). *(Annotated 2026-11-02: S1.1a's plan check split it into S1.1a-1 and S1.1a-2 → **24 sprints, about 272.5 points** (299 in card sizes).)* At 2–3 weeks wall-clock per sprint for a solo owner, Phase 1 is **46–69 weeks**. Several cards sit well under 13 (S1.2, S2.2a, S5.2, S9.2, S10), so the lower half of the range is plausible but unproven. No velocity has been measured: **re-forecast at S1.2's G5 and at S3.3's G5** from recorded points and durations (sprint model §3). A card the plan check sizes over 13 is split into its own two sprints **before** G0 — never by moving work into a neighbouring card that is already full. *(Re-forecast, owner 2026-11-05, early data: nine sprints, S1.1a-1 to S3.1-2, closed in three calendar days, 2026-11-02 to 2026-11-04, for about 84 recorded points — about 28 points a day. About 195 points remain: 185 on the cards from S3.2a to S12, and about 12 moved to S1.1c from S1.1b and S1.2, still unsized. At the measured pace that is about 7 days; at a third of it about 3 weeks, allowing for the concurrent runtime, the pattern engine, the connectors, the window sprints' three-reviewer panels and longer owner's checks. **Phase 1 forecast: about 1–4 weeks from 2026-11-05**, against v3's 46–69 weeks. The slim sprints' points (S1.1b, S1.2, S2.1) are estimates, and no sprint so far has a live path or a heavy break-it. The next re-forecast stays at S3.3's G5.)*

**Gate instruments by sprint** (ADR-0015 `lands_in`, mapped onto the split numbering — each lands with its first subject, fixture, `min_wants`, registry row and teeth pin in one commit, and scans the whole module retroactively).

| Sprint | Instruments landing |
|---|---|
| ~~S1.1a~~ | ~~registry + **parallel** runner + summary; parity + atomic installer; the `gofmt` and `go vet` rows; hooks; CI (concurrent jobs, cached) + summary fixtures; `gate/proofs/`; **`coverage-inclusion`** — every package tested + the **90%** per-package floor (owner 2026-11-02)~~ *split 2026-11-02 into the two rows below; the gate's tooling is stdlib Python (owner, §5)* |
| S1.1a-1 | registry + **parallel** runner + summary (Python, CPython 3.15.2 pinned); the bootstrap installer + parity (every listed executable re-hashed every run); the `gofmt`, `vet` and `unit` rows; the gate's Python checked by `ruff` and `pyfloor` and tested by `pytests`; `gate/proofs/`; **`coverage-inclusion`** at **90%** |
| S1.1a-2 | hooks (T0 refuses unformatted files; `make check` refuses until hooks are on); CI (concurrent jobs on `ubuntu-latest`, cached; the workflow generated from `registry.json` + `ci-fresh`) + `summarize` and its fixture trees; the required `strict-gate` check |
| S1.1b | `lawdelta` + `gate/law.json`; registry-lint; `importcheck`; `clockcheck`; `puritycheck`; `globalscheck` (their first subject, `internal/event`, exists from S1.1a); `todocheck` (basic); `testhygiene`; teeth meta-test; `smells`; `suppressions` + the one allowlist grammar |
| S1.2 | `docsync` (a) one direction; the dependency-allowlist check; the licence check and `govulncheck`; the build-tag law; the weekly canary; the sprint-id grammar, `gate/current-sprint` and the skip / `deferred(P<n>, ref=…)` forms in `todocheck`; `docsync` (d) error-count consistency |
| S2.1 | `freshness` *(annotated 2026-11-04: moot for the parser; participle)* |
| S2.2a | the grammar differential (T4); `docsync` (b) reserved words ↔ kwgen; the nightly workflow (one advisory, aging row per fuzz target); `docsync` (c) Appendix F ↔ `expr.Registry`; `allocgate` first rows *(annotated 2026-11-04: (b) compares the parser's keyword table with `grammar.lark` directly)* |
| S2.2b | `docsync` (b) [static] codes ↔ `catalog.go` |
| S3.2b | race step + goleak proof; the metric-tripwire step (outbox ceiling blocking at 65,536); `allocgate` ingest → filter; the two-OS determinism proof |
| S3.3 | `apipartition` + `apidiff` (merge base); `gate/regen-baseline` + Tier-2 baselines; the planted wrong-expect proofs; `api/clock.golden` (law path, at the freeze) |
| S4.1, S8, S9.1, S9.2 | `allocgate` extended per hot-path stage; the walked-state / timer-derivability invariant extended to each new state holder |
| S6.2 | `fmtcheck` |
| S10 | benchmark tripwires (`exclusive` rows); the compose-kit nightly job |
| S12 | `docsync` (a) reverse direction; the ARCHITECTURE §7 file-existence check; the docs-to-conformance check |

**Roadmap exit criteria → where each is proven.** (1) quickstart < 15 min → S10. (2) conformance + unit green for every construct → every sprint's exit; latency measured and published → S10. (3) docs for the shipped subset, `v0.1.0`, public repo → S12. (4) DSL/Go parity proven by one test → S6.1 (twins extended S7–S9.2, S11). (5) attack gallery ≥ 5 → S11 (seven by owner decision).

**The 40 semantics-harness checks → the one sprint that ports each.** S2.2a: the 1.1 null-strict check's scalar assertions (its `in` assertion is completed in S3.1) = 1. S2.2b: RT-14, "9 · TTL deploy check", RT-07's extended static check (pattern and rate parts) = 3. S3.2b: the three §8 rate-limit checks = 3. S4.1: the eleven §6 checks, the 2.1/6.5 late-drop check, the four §2.1 checks other than RT-09, and the two pattern-dependent §9 purge checks = 18. S7: the seven §5 text checks = 7. S8: sort ×3, batch (RT-21), RT-06 changelog purge = 5. S9.1: cron half-open = 1, plus RT-07's window-extent assertions. S9.2: delay = 1. **39 ported; RT-09 deferred to Phase 2** (it needs a rule consuming a derived stream, which v0.1 rejects) as `deferred(P2, ref=sprint-plan §5 2026-11-01)`. S3.1 ports none: it carries clock-level unit tests of the same arithmetic, each citing its source check. New harness checks (and-groups in S4.2; the cron admission rule in S1.2's rev 3) are written red-first in Python before any Go.

**Corpus examples → the sprint that turns each green.** 07-consent-gate, 09-params-and-quoting → S3.3 (their test material is written in S3.2b). 01-agent-kill-switch, 08-session-flow → S4.1. 05-text-operators, 06-once-and-group → S7. 03-sort-leaderboard → S8. 02-cron-digest → S9.1 (its parity twin in S9.2). 04-delay-and-batch → its batch rule in S8, the whole file in S9.2. **Test material (owner decision 2026-11-02):** in the sprint that owns an example, the QA agent adds to the example file **≥ 1 fire-path and ≥ 1 near-miss expect block, multi-key or multi-session wherever the rule correlates**, with expected outputs derived by hand from SEMANTICS or from the Python reference executor **before the engine runs**. Each addition is a spec-corpus change: `grammar_check` re-runs green and the LAW Δ block shows it. An output an expect block cannot express (03's `+`/`−` rows, if SPEC §6.8 has no form for them) gets a `conformance/cases/<example>/` log beside it. Until its sprint, an example's cases are `skip: not-implemented(<sprint>)`; **a skip naming the current sprint is red at that sprint's own T4**, and every skip still open at S12 is red.

---

*Closed cards (S1.1a to S3.2c) are in [history.md](history.md#closed-sprint-cards), word for word. Each sprint gets a card in this section ([template](templates/sprint-card.md)). Once its plan check is answered, a short "how it runs" note follows the card: lanes, waves, gates. At close-out the card moves to history.md.*

#### S3.3 — The Deploy pipeline and the conformance runner — clock freeze

| | |
|---|---|
| **Goal** | A filter rule deployed through the public `cep.Runtime.Deploy` runs with its expect blocks executed as a deploy stage, the conformance runner provably fails on wrong expectations, and the clock contract is frozen under the gate. |
| **Routine · size** | Full loop (the public Go API) · 13 SP (items 13) |
| **Depends on** | S3.2b |
| **In scope** | 1. The public facade `cep` (minimal): `Runtime`, `Config`, `Deploy` (parse → validate → plan → expect sandbox → register), `AppHandle` `Start` / `Stop` (drain barrier to a fixpoint, `drained_pending`) / `Undeploy`, `Runtime.Close` (reverse deploy order), multiple apps per runtime, `Input` with **`SendRow` only**, the `Table` API, `BuildInfo` [4]. · 2. **The connector contracts as the leaf package `io` (package `cepio`)** — `Source`, `Sink`, `Mapper`, `RowWriter`, `EmitFunc`, `OutEvent`, `Mode`, and **the batch-close signal** (designed here; an API Δ) — plus `io/inmemory` [1.5]. · 3. **The expect sandbox in `internal/playback`**, called by `cep.Deploy` and `conformance/`: a fresh sandbox per block, `given table` seeding through table records [2]. · 4. The conformance runner `conformance/` (public API only); E-EXP-001 lifted from S2.2b's skip; the six planted wrong-expect proofs [2]. · 5. `apipartition` (`api/exports.golden`: `cep` + `io`) + `apidiff` against the merge base [1.5]. · 6. `gate/regen-baseline` + Tier-2 baselines [1.5]. · 7. `api/clock.golden` added to `gate/law.json`; **the clock contract freezes at this sprint's G5** [0.5]. |
| **Not in scope** | Patterns → S4.1. HTTP and the other connectors → S5.1, S5.2. |
| *Forwarded* | *(Annotated 2026-11-04, from S3.1-2, owner D3: an `optional` or `object` primary-key column is a deploy error with a new code, with its SPEC v0.1 change-log entry and an invalid example; owner D2: the Go API's row boundary refuses a non-finite float; break-it B5: an object value enters as its JSON text.)* *(Annotated 2026-11-05, from S3.2a-1, owner D7: an `object` partition key is a deploy error with a new code beside D3's, with its SPEC v0.1 change-log entry and an invalid example; until then `partition.NewRule` refuses it. And under D1, `Stop`'s drain uses `clock.StreamRegistry.Drain`: one past the max observed event time.)* *(Annotated 2026-11-05, from S3.2a-1's break-it: the replay reader skips an unknown attribute in an event's `attrs`, as live ingest does, so a typo in a hand-written expect block reads as null; the conformance runner should make an unknown attribute in an expect block a fault.)* *(Annotated 2026-11-05, owner at S3.2a-2's plan check: the drain barrier's mechanism moves to S3.2a-2; item 1's `Stop` wraps it. Item 1 shrinks by about 1 point (13 → 12).)* *(Annotated 2026-11-06, from S3.2b-1: the expect sandbox must define what a block's given lists mean as arrival — a list's order is its arrival order, and how several lists interleave (material gap M1; example 07's arrival-order block depends on it) — and that a block passes only when the outputs match the then-list in number as well as content (M2). `Deploy` must not mutate or reuse the checked model after `plan.Build`: a built plan compiles its bodies from it at `Start` (break-it L2).)* *(Annotated 2026-11-06, from S3.2b-2's design ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-design-after-the-architecture-review)), owner: `Deploy` has a recording setting, **off unless turned on**; when on, it attaches `playback.NewRecorder` to the app (`runtime.App.Recorder`).)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 07 and 09 pass their fire-path and near-miss expects through `Deploy` — **proof:** the conformance runner. |
| | 2. Each planted wrong expect — wrong value, missing output, extra output, wrong order, an output under an empty `then`, wrong timestamp — fails `Deploy` with E-EXP-001 naming the first divergence — **proof:** six proof rows. |
| | 3. Two apps in one runtime share no state; `Runtime.Close` stops them in reverse deploy order — **proof:** the multi-app isolation test. |
| | 4. Two `in` predicates on one table within one event see one snapshot while a concurrent `Table.Upsert` is held between them — **proof:** the live table-snapshot test. |
| | 5. An unlisted export turns `apipartition` red; `apidiff` reports a planted signature change — **proof:** proof rows. |
| | 6. `gate/regen-baseline` refuses to record a run that changes timestamps, ordering or counts, and records an additive one — **proof:** regen-baseline proofs. |
| | 7. A planted change to an exported `internal/clock` signature appears in the LAW Δ block — **proof:** the clock-golden proof. |
| **Owner's check** | *By running:* a 20-line Go program that embeds the engine, deploys 09 and feeds events through `SendRow`. *By reading:* `api/exports.golden` — the first public surface. |
| **Split line** | **S3.3-1** = items 1, 2, 5 (facade, contracts, API gate); **S3.3-2** = items 3, 4, 6, 7. |

#### S4.1 — The pattern engine: the kill switch fires in playback

| | |
|---|---|
| **Goal** | The flagship kill-switch rule fires and is defeated exactly per SEMANTICS §6 — including its full evidence chain and a second session — and every pattern harness check passes in Go. |
| **Routine · size** | Full loop (pattern matching) · 13 SP (items 11) |
| **Depends on** | S3.3 |
| **In scope** | 1. `internal/pattern` (ADR-0013): partials, continuous absence watchers, completion timers, skip-till-next, skip-past-last earliest-wins, `every` / `once`, the per-instance freelist, `Walk` [5]. · 2. The `chain` factory [1]. · 3. Plan wiring for pattern rules [1]. · 4. The 18 harness ports (see the mapping) [2]. · 5. **Test material (QA, before the engine runs):** 01 gains an expect with two sessions in one agent, and its fire-path expect asserts the full `evidence` value (in the expect if SPEC §6.8 can express it, otherwise a conformance case); 08 gains fire-path and near-miss expects where `sessionId == a.sessionId` can be false [1]. · 6. The walked-state invariant extended to partials; `allocgate` spawn; partial-count ceilings for 01 and 08 [1]. |
| **Not in scope** | `and` groups, the live quiet-key case, the drain case, the oracle and the metamorphic suite → S4.2. Live connectors → S5.1. |
| *Forwarded* | *(Annotated 2026-11-05, from S3.2a-1: an operator that keeps an event after its release (a capture) sets its `Tables` to nil, so kept events do not pin table copies (review F3). Under owner D1 a timer due at `t` fires once the clock is past `t`: a deadline is scheduled at exactly `t_first + D`, and an event at the deadline is processed first (SPEC §6.4). Per-key marks (`once`) live in the body's `OpState`; `partition.Rule.CheckTimers` runs after every step of the operator's tests.)* *(Annotated 2026-11-06, from S3.2b-1 (combined review C9): `pattern` and `window` cannot import `plan`, so they cannot call its emission stage; `plan` wraps their bodies and applies the rate limit, the anchor living in the wrapped body's per-key state (ADR-0006 §2). E-NULL-003 already applies to pattern selects; a capture's own partition key counts as never null.)* *(Annotated 2026-11-06, from S3.2b-2 (QA G1): multi-part keys over two streams — the design's worked examples 3 (`(src, dst) of Requests, (dst, src) of Responses`) and 4 (types by position) — are proven statically in S3.2b-2; their engine tests come with the first multi-stream rule here.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 01's expects pass: fire at 10:05 with the asserted evidence chain (both captures, in order, original values); defeat → nothing; the second session neither fires nor defeats the first — **proof:** the conformance runner. |
| | 2. 08's expects pass — **proof:** the conformance runner. |
| | 3. The 18 ported checks pass in Go — **proof:** `conformance/semantics_test.go`. |
| | 4. Spawning allocates nothing once the freelist is warm — **proof:** `allocgate`. |
| **Owner's check** | *By running:* the kill-switch conformance test (command named in the plan check), then edit the approval timestamp: at 10:05:00.000 it defeats, at 10:05:00.001 it fires. *By reading:* the evidence chain. |
| **Split line** | None needed (11). |

#### S4.2 — The pattern engine completed and independently proven

| | |
|---|---|
| **Goal** | Logical `and` groups work per SEMANTICS, a quiet key's absence fires on time in live mode, and the pattern engine agrees with an independently written oracle on hundreds of random streams while all five metamorphic laws hold — each law proven able to fail. |
| **Routine · size** | Full loop (pattern matching; can only find bugs whose fixes change detections) · 13 SP (items 13) |
| **Depends on** | S4.1 |
| **In scope** | 1. **Logical `and` groups** (SPEC §5.2; SEMANTICS §6.1, §6.5): new Python harness checks written red-first in the design step — both arrival orders, equal timestamps, an absence conjunct, `every` per group, skip-past-last contention, `once` — and any ambiguity they expose goes to the owner [3]. · 2. **The live quiet-key case (ADR-0004 notes):** two keys, the live driver, ≥ 2 shards, the injectable idle-tick source [1.5]. · 3. **`drained_pending`** with a real pending absence at `Stop` [0.5]. · 4. The brute-force reference matcher, **written by an agent other than the pattern engine's author**, plus a differential against `docs/language/checks/executor.py` on the same seeds; shapes: two- and three-step chains with cross-step references, absence before and after positives, `and` groups, `every` vs `once`, skip-past-last contention, a TTL purge mid-match, two streams with idle and virtual clocks; 200–300 seeded streams per shape; branch counters with floors [5]. · 5. The metamorphic suite (qa-strategy §2): five laws with their scopes, a scenario floor per law incl. the oracle's random multi-key streams, one planted proof per law [3]. |
| **Not in scope** | The cron variant of time shift → S9.1. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The new and-group checks pass in Python and in Go — **proof:** the harness + `conformance/semantics_test.go`. |
| | 2. Live, with the quiet key: its absence fires at exactly its deadline and its TTL purge happens at exactly its purge time — **proof:** the live quiet-key test (asserting times, not presence). |
| | 3. `Stop` with a pending absence counts it in `drained_pending` and does not fire it — **proof:** the drain test. |
| | 4. Engine, oracle and executor agree on every stream of every shape, counters above their floors — **proof:** the oracle test. |
| | 5. Each law is green over at least its scenario floor and red on its planted defect — **proof:** the metamorphic suite + five proof rows. |
| **Owner's check** | *By reading:* the and-group harness checks; the oracle's shape list and one run's counters. |
| **Split line** | **S4.2-1** = items 1–3 (and-groups, live quiet key, drain); **S4.2-2** = items 4–5 (oracle, metamorphic). |

#### S5.1 — HTTP in, webhook out: the kill switch fires live

| | |
|---|---|
| **Goal** | `cepd`, started with a JSON config, receives agent events over HTTP, detects the kill-switch pattern and calls the IAM webhook with the expected evidence, safely and with retries. |
| **Routine · size** | Full loop (sink delivery) · 13 SP (items 11.5) |
| **Depends on** | S4.2 |
| **In scope** | 1. `io/http` source (JSON per event; an NDJSON body = one source batch, closed with the batch-close signal when the body ends) and sink (JSON, `{{attr}}` templates, coalesced batches, exponential backoff with a cap, `sink_failures`) [4]. · 2. `io/jsonmap` via a **facade decode entry** (a named API Δ; built-ins never import `internal/*`) [1.5]. · 3. `io/render.go`, the shared renderer, and its injection-safety table [1.5]. · 4. `cmd/cepd` (minimal): rule files + `cepd.json` (param overrides type-checked, E-PARAM-001; connector settings; secrets via params or environment), `import _ "time/tzdata"` [2]. · 5. The live end-to-end tests, driving idleness through the idle-tick source, incl. S4.2's quiet-key case over HTTP [1.5]. · 6. The sink-failure test and the saturation test over HTTP [1]. |
| **Not in scope** | stdin, SMTP, log, text mapper, `@OnError`, `/metrics`, `/healthz`, latency hooks → S5.2. |
| *Forwarded* | *(Annotated 2026-11-06, from S3.2c: a timestamp past year 9999 reaching an output cannot be written in RFC 3339; the sink's encoding decides what that is — an error counted for its rule, as S3.2c makes overflow.)* *(Annotated 2026-11-07, owner ([decisions](decisions.md#owner-decisions-2026-11-07--s32cs-review-and-break-it), 2): a timestamp outside years 0000–9999 — `timestamp − duration` can land before year 0000, which the encoder writes as a negative year, not RFC 3339 either.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 01's fire scenario POSTed to the packaged `cepd` produces one POST at the stub IAM listener whose body equals S4.1's expected evidence; the defeat scenario produces none, asserted only after a witness that the key's construct clock passed 10:05 — **proof:** the `cepd` end-to-end test. |
| | 2. The quiet-key case over HTTP fires at the same time as in S4.2 — **proof:** the live quiet-key HTTP test. |
| | 3. The renderer keeps JSON valid for values containing quotes, backslashes, CR/LF, Unicode and invisible characters — **proof:** the renderer table test. |
| | 4. A failing sink retries with backoff, then counts `sink_failures`; shards never block; sources feel the backpressure — **proof:** the sink-failure + saturation tests. |
| | 5. A param override of the wrong type is rejected with E-PARAM-001 at start-up — **proof:** the config test. |
| **Owner's check** | *By running:* `cepd` with the named example config; send the two kill-switch events with `curl`; the webhook stub logs the hit about one minute later (the stream's `idle after 1 min`). *By reading:* the example `cepd.json`. |
| **Split line** | **S5.1-1** = items 1–3; **S5.1-2** = items 4–6. |

#### S5.2 — The other connectors, error logging and observability

| | |
|---|---|
| **Goal** | The stdin source, the SMTP and log sinks and the text mapper work and are safe, `@OnError` logs each error with its cause and rule, `cepd` serves metrics and health, and the engine records per-stage latency without contention. |
| **Routine · size** | Full loop (sink delivery; hot-path instrumentation) · 13 SP (items 9) |
| **Depends on** | S5.1 |
| **In scope** | 1. `io/stdin` (NDJSON; one batch per `--batch-lines` or per EOF, closed with the batch-close signal) [1]. · 2. `io/smtp` (`net/smtp` + STARTTLS; one message per event; header-safe rendering) [2]. · 3. `io/log` (canonical JSON) [0.5]. · 4. `io/textmap` [0.5]. · 5. `@OnError` log: cause and rule id [1]. · 6. A facade metrics-exposition method (named API Δ); `cepd` `/metrics` and `/healthz` [1.5]. · 7. Per-stage latency hooks and per-shard histograms (no locks, merged on read) [2]. · 8. `allocgate` rows for decode and encode [0.5]. |
| **Not in scope** | The benchmark suite and published numbers → S10. |
| *Forwarded* | *(Annotated 2026-11-07, from S3.2c's review, R6: an integer overflow is an expression error like division by zero, so exit 4's logging covers it too; its message then prints durations with a unit and timestamps in RFC 3339 — today some print raw milliseconds.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Against an in-test mail catcher on port 0: one message per event with the rendered subject and body; an `agentId` containing CR/LF cannot add a header — **proof:** the SMTP test + the header-injection rows. |
| | 2. The log sink prints exactly the canonical JSON — **proof:** the log-sink golden. |
| | 3. stdin replaying a log gives the same output as playback — **proof:** the stdin-vs-playback test. |
| | 4. A division-by-zero error is logged with its cause and rule id — **proof:** the `@OnError` test. |
| | 5. Histograms read concurrently with shard writes stay race-free — **proof:** the histogram test under `-race`, 50/50. |
| | 6. `/metrics` exposes the counter set; `/healthz` answers — **proof:** the `cepd` test. |
| **Owner's check** | *By running:* the named example config with the email sink; read the email in the local mail catcher. |
| **Split line** | None needed (9). |

#### S6.1 — The Go builder, DSL/Go parity and the CLI

| | |
|---|---|
| **Goal** | The kill-switch rule authored with the Go builder produces a byte-identical rule model and identical playback output to the DSL (roadmap exit criterion 4), and `cep validate` / `cep test` drive the public Deploy pipeline. |
| **Routine · size** | Full loop (the public Go API) · 13 SP (items 13) |
| **Depends on** | S5.2 |
| **In scope** | 1. Package `rule` (ADR-0003 §4; SPEC Appendix D): `rule.Stream[T]` with tags; **the design step lays out the whole SPEC §4.4 type-state graph** so S7–S9.2 add states without restructuring; implemented for every construct shipped through S5.2; `rule.Param[T]`; `Build()` returning catalog codes; `rule → validate` is a listed import edge [6]. · 2. **Typed input**: the generic entry point mapping `T` to `SendRow` (e.g. `cep.InputFor[T]`), with its mapping owner named in the design [1.5]. · 3. The parity test (ADR-0003 §5, a–e) for 01, 07, 08, 09 [2]. · 4. `cmd/cep validate` and `cmd/cep test` (`import _ "time/tzdata"`); the rule unit-test framework (O6) [2]. · 5. One compile-failure row per mandatory clause [1]. · 6. `apipartition`: the `rule` exports [0.5]. |
| **Not in scope** | Builder surface for text and windows → S7, S8, S9.2. The formatter → S6.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Parity holds for 01, 07, 08 and 09 — canonical JSON equal with `ParamRef`s, equal after resolution, identical playback outputs — **proof:** the parity test (exit criterion 4). |
| | 2. For every mandatory clause in SPEC §5.4, a builder chain that omits it does not compile — **proof:** the compile-failure rows. |
| | 3. `cep test` runs every expect block and exits 0; on a planted wrong expect it exits non-zero naming the divergence — **proof:** the CLI end-to-end test. |
| | 4. `cep validate` on an invalid file prints code, line, column and fix text with the documented exit code — **proof:** the CLI validate test. |
| **Owner's check** | *By running:* write the kill switch in Go from the package example; run `cep test` and `cep validate` on a broken file. *By reading:* the `rule` godoc. |
| **Split line** | **S6.1-1** = items 1, 2, 5, 6; **S6.1-2** = items 3, 4. |

#### S6.2 — `cep fmt` and the generated timing comment

| | |
|---|---|
| **Goal** | `cep fmt` rewrites any rule file into SPEC §9's canonical form, preserving every user comment and keeping a formatter-owned comment on each rule that names the stream timing it inherits, and the gate refuses unformatted files. |
| **Routine · size** | Full loop (the formatter's output is part of the language; the design step runs the language-design skill) · 13 SP (items 12.5) |
| **Depends on** | S6.1 |
| **In scope** | 1. The canonical printer `internal/parser/printer.go` from S2.1's trivia, explicit parentheses kept as written [4]. · 2. A facade entry `cep.Format` (named API Δ) and `cmd/cep fmt` (rewrite; `--check`) [1]. · 3. The generated timing comment — marker and text designed here — and the SPEC §9 sentence landing with it [1.5]. · 4. The corpus regenerated once (`docs/language/examples/`), with the regeneration-neutrality check [1]. · 5. `fmtcheck` with expected scope `git ls-files '*.cep' ':!docs/language/invalid/**' ':!docs/language/eval/generated/**' ':!**/testdata/**' ':!gate/proofs/**' ':!.claude/**'` and its stale-comment proof [1]. · 6. The round-trip property test with the seeded layout-variant generator, comment-multiset property and the lark side of the differential [2]. · 7. The comment position-class table [1]. · 8. The parameterized timing-comment test [1]. |
| **Not in scope** | Format-on-save or editor plugins (later). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. For every file in `examples/`, `eval/generated/` and the torture set, and for seeded layout variants of them: `parse → format → parse` yields the identical model, formatting is idempotent, the multiset of user comments is preserved, and every formatted variant passes the lark side of the differential — **proof:** the round-trip property test. |
| | 2. User comments survive at their position in every position class, and a user comment that looks like the generated marker is preserved — **proof:** the position-class table test. |
| | 3. Changing any listed timing field and re-formatting rewrites the comment on exactly the rules that consume that stream, incl. a two-stream rule — **proof:** the parameterized timing-comment test. |
| | 4. On the regeneration commit, every model golden, differential S-expression and conformance output is byte-unchanged — **proof:** the neutrality check. |
| | 5. A stream-timing edit without re-formatting turns `fmtcheck` red — **proof:** the stale-comment proof. |
| **Owner's check** | *By running:* mangle a rule file's layout and run `cep fmt`; change a stream's watermark and read the diff. *By reading:* the timing comment on the flagship. |
| **Split line** | **S6.2-1** = items 1, 2, 6, 7 (printer + `cep fmt`); **S6.2-2** = items 3–5, 8. |

#### S7 — Text operators, incl. live table term lists

| | |
|---|---|
| **Goal** | All five text-operator families evaluate exactly per SEMANTICS §5 in both case modes and both front-ends, term lists stored in a table update live, and evidence keeps the original text. |
| **Routine · size** | Full loop (changes what a rule detects) · 13 SP (items 13) |
| **Depends on** | S6.2 |
| **In scope** | 1. `internal/text` (ADR-0007): NFC, `FoldSimple`, `Tokens`, Aho-Corasick, regex wrappers incl. `(?i)` for `_ci` [5]. · 2. The expression text nodes and `raw()` [1.5]. · 3. **Table operands for keyword sets, live** (owner 2026-11-02): the matcher rebuilds when the table's version changes — never per event, never shared mutably between shards [2]. · 4. The validator's E-TXT-002 (lifting S2.2b's skip); `validate → text` is a listed edge [0.5]. · 5. **Test material (QA, before the engine runs):** fire-path and near-miss expects for 05 and 06 (06's `and` group from S4.2 and its `once`); 05's include a payload claiming `text_invisibles: false` [1]. · 6. Builder text surface + parity twins of 05 and 06 [1.5]. · 7. The seven §5 harness ports; fold-sensitive regex cases [1]. · 8. `allocgate` rows for text (target zero; rebuild-on-update and other allocating paths listed as counted exclusions) [0.5]. |
| **Not in scope** | Locale-aware or full case folding (never). LLM judgement → Phase 4. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 05 and 06 pass their expects; 05's spoofed-flag case still flags — **proof:** the conformance runner. |
| | 2. The seven §5 ports and the fold-sensitive regex cases pass — **proof:** the conformance suite. |
| | 3. Table operands × all four set families × both case modes × terms/substrings evaluate correctly, and a table record between two events changes the result from the next event onward — **proof:** the table-operand matrix test. |
| | 4. A text rule's evidence carries the original text, incl. an invisible character and a case variant — **proof:** an evidence case. |
| | 5. Parity twins of 05 and 06 are green; no `skip: not-implemented(S7)` remains — **proof:** the parity test; the summary. |
| **Owner's check** | *By running:* a keyword-set rule over a prompt containing an invisible character and a case variant; then update the term table and send the same prompt again. *By reading:* the evidence. |
| **Split line** | **S7-1** = items 1, 2, 4, 7, 8 (operators); **S7-2** = items 3, 5, 6. |

#### S8 — Windows I: sort, batch and `count`

| | |
|---|---|
| **Goal** | Sort and batch windows emit exactly per SEMANTICS §7–§9, `count` aggregates over a closed group, and batch punctuation works from live sources as well as from replay, in both front-ends. |
| **Routine · size** | Full loop with the full blind three-reviewer design panel (owner decision 2026-11-01; the panel's days sit outside the points) · 13 SP (items 12) |
| **Depends on** | S7 |
| **In scope** | 1. `internal/window`'s `Kind` interface and `Out` (ADR-0014 §1) [1]. · 2. Sort: the tie rule, changelog output, `OnPurge` retractions in total order [2.5]. · 3. Batch: grouping by `BatchID` within the key; firing on the batch-close signal delivered by the router as a control timer; one group event at the group's max event time; the fired-id dedup set per instance, TTL-purged with it [2.5]. · 4. **`count`** (registry `Aggregate` kind) over a closed group, respecting E-SEL-002 [1]. · 5. **Test material (QA, before the engine runs):** fire-path and near-miss cases for 03 (its `+`/`−` rows in a conformance log if an expect cannot express them) and for 04's batch rule [1]. · 6. Builder sort and batch surface + the twin of 03 [1.5]. · 7. The five harness ports [1]. · 8. The live batch test (NDJSON over HTTP and the same records over stdin) [1]. · 9. The walked-state invariant for window state; `allocgate` [0.5]. |
| **Not in scope** | Cron → S9.1; delay → S9.2. The time-based window catalog → Phase 2. |
| *Forwarded* | *(Annotated 2026-11-05, from S3.2a-1: windows keep events past release, so they clear `Tables` on capture (review F3). Under owner D1 a timer fires once the clock is past its due: a close that must run once the clock reaches `e` is scheduled at `e − 1`. The batch-id dedup set lives in the body's `OpState`, so a purge clears it. Batch ends arrive as `StepBatchEnd` in replay.)* *(Annotated 2026-11-06, from S3.2b-1 (combined review C8): every callable added — aggregates included — states its `expr.NullRule`; the default (null in, null out) is wrong for an aggregate over an empty group, and E-NULL-003 reads it.)* |
| *Forwarded* | *(Annotated 2026-11-06, from S3.2c ([decisions](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check)): an aggregate (`sum`, `count`) that overflows is an expression error for its rule, as every 64-bit operation is from S3.2c; the aggregates use `internal/expr`'s checked arithmetic.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 03 and 04's batch rule pass their cases — **proof:** the conformance runner (+ 03's log). |
| | 2. The five ports pass — **proof:** the conformance suite. |
| | 3. An NDJSON body POSTed to `cepd`, and the same records through stdin, produce the playback result — **proof:** the live batch test. |
| | 4. A redelivered batch id emits nothing — **proof:** the RT-21 port. |
| | 5. The twin of 03 is green — **proof:** the parity test. |
| **Owner's check** | *By running:* a leaderboard rule with a tie. *By reading:* the changelog rows. |
| **Split line** | **S8-1** = sort (items 1, 2, 03's material and twin); **S8-2** = batch + `count` (items 3, 4, 04's material, the live batch test). |

#### S9.1 — Windows II: cron

| | |
|---|---|
| **Goal** | Cron windows emit exactly per SEMANTICS §7 with SPEC's dialect — including daylight saving — implemented in-repo, late events handled exactly as rev 3 pins them, and live firing driven by idleness. |
| **Routine · size** | Full loop with the full blind three-reviewer design panel · 13 SP (items 12) |
| **Depends on** | S8 |
| **In scope** | 1. `internal/window/cron.go` (ADR-0014 §3, §6): the 5-field Vixie parser; next-fire by field-wise search in the rule's timezone; `time.LoadLocation` over embedded tzdata [4]. · 2. **Enumerated DST cases, expected UTC instants derived with Python `zoneinfo` before the Go code:** the spring gap and autumn fold for `0 2 * * *` in America/New_York; Australia/Lord_Howe's 30-minute shift; a southern-hemisphere zone; a day-of-week-restricted expression across a transition [1.5]. · 3. The `windowspec` helper for the validator: E-WIN-003 and the cron part of E-TTL-002 (lifting S2.2b's skips) — the 10-year extent scan reads its start instant from an injected clock, pinned in tests [1.5]. · 4. Buckets with `count` (from S8), the close timer at `bᵢ₊₁`, a single fire, no allowance (rev 3), incl. the admission-drop case [1.5]. · 5. **Test material (QA, before the engine runs):** fire-path and near-miss expects for 02, incl. an event behind the admission watermark whose bucket is still open [1]. · 6. Live cron close through the idle-tick source [1]. · 7. The time-shift law's cron variant [0.5]. · 8. The cron harness port; RT-07's window-extent assertions; the boundary property test; the walked-state invariant for buckets [1]. |
| **Not in scope** | The builder's cron surface and the twin of 02 → S9.2. Delay → S9.2. `late events allow` → Phase 2. |
| *Forwarded* | *(Annotated 2026-11-05, from S3.2a-1 (owner D1; QA G1): a timer fires once the clock is past its due, so a cron close that must run once the clock reaches a bucket's end `e` is scheduled at `e − 1`; the oracle's `cron` function is changed to D1 first.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 02 passes its cases, incl. the admission-drop case — **proof:** the conformance runner. |
| | 2. Every enumerated DST case yields its pre-derived UTC instants; every rejected form returns E-WIN-003 — **proof:** the cron table tests. |
| | 3. The extent scan gives the same answer whatever day the test runs — **proof:** the scan test with two different injected start instants. |
| | 4. A live cron bucket closes through idleness at its boundary — **proof:** the live cron test. |
| | 5. The cron time-shift variant is green; no `skip: not-implemented(S9.1)` remains — **proof:** the metamorphic suite; the summary. |
| **Owner's check** | *By running:* a daily-digest rule across a DST change in a named timezone. *By reading:* the bucket boundaries. |
| **Split line** | **S9.1-1** = items 1–3 (parser, DST, validator); **S9.1-2** = items 4–8. |

#### S9.2 — Windows III: delay, and the cron/delay builder twins

| | |
|---|---|
| **Goal** | Delay windows re-time events exactly per SEMANTICS §7, 04 passes as a whole, and cron and delay rules can be authored with the Go builder with proven parity. |
| **Routine · size** | Full loop with the full blind three-reviewer design panel · 8 SP (items 6.5) |
| **Depends on** | S9.1 |
| **In scope** | 1. Delay: a due-ordered queue; each event re-stamped `t + D` [2]. · 2. **Test material (QA):** fire-path and near-miss expects for 04's delay rule, plus a delay case across a TTL purge [1]. · 3. Builder cron and delay surface + the parity twins of 02 and 04 [2.5]. · 4. The delay harness port; the walked-state invariant for the queue; `allocgate` [1]. |
| **Not in scope** | Rule-consumption of a delay's output (derived stream) → Phase 2 (RT-09). |
| *Forwarded* | *(Annotated 2026-11-05, from S3.2a-1 (owner D1; QA G1): the oracle's `delay` still releases at `wm ≥ t + D`; under D1, whether delay re-times at `wm ≥ t + D` (scheduled at `t + D − 1`) or once the clock is past `t + D` is this sprint's design, oracle first, and an owner decision.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 04 passes all its cases, incl. the TTL-purge case — **proof:** the conformance runner. |
| | 2. The delay port passes; the twins of 02 and 04 are green; no `skip: not-implemented(S9.2)` remains — **proof:** the conformance suite; the parity test; the summary. |
| **Owner's check** | *By running:* a delay rule with one out-of-order event; the cron digest authored in Go. |
| **Split line** | None needed (6.5). |

#### S10 — Demo kit, quickstart, benchmarks and the published latency

| | |
|---|---|
| **Goal** | A newcomer runs the kill-switch demo from the public quickstart in under 15 minutes, the demo is checked automatically, and detection latency is measured with an open-loop benchmark and published with its span, hardware and method. |
| **Routine · size** | Lighter (no change to what any rule detects or emits; the hot-path hooks were built under the full loop in S5.2) · 13 SP (items 10) |
| **Depends on** | S9.2 |
| **In scope** | 1. The `bench/` suite (ADR-0010 §8), consuming S5.2's hooks: scenario generators in `bench/gen/`; an open-loop load generator; per-stage and end-to-end percentiles [4]. · 2. `benchstat` baselines + CI tripwires, registered as **`exclusive`** rows [1.5]. · 3. `demo/`: the compose kit (traffic generator sharing `bench/gen/`, stub IAM endpoint, mail catcher) and a nightly job that brings it up and asserts both the stub and the mail catcher receive the kill switch [3]. · 4. The quickstart [1]. · 5. The published latency page [0.5]. |
| **Not in scope** | The full docs for the shipped subset → S12. Further optimization. Fleet-wide claims (Phase 2–3). |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block (lighter), plus: |
| | 1. The owner completes the quickstart cold in under 15 minutes — **proof:** the timed run recorded in the sprint record (exit criterion 1). |
| | 2. Two spans published (exit criterion 2): **engine detection latency** — from the router accepting the event (or idle tick) that makes the match due to the kill-switch event entering the sink queue — with p50/p99/p99.9/max and pass/fail against O3's sub-millisecond p99; and **end-to-end** through the IAM webhook's response, with hardware and method — **proof:** the bench report. |
| | 3. A planted 10× regression turns the benchmark tripwire red — **proof:** a gate proof. |
| | 4. The compose-kit job is green and goes red when the stub IAM endpoint is removed — **proof:** the nightly job + its proof. |
| **Owner's check** | *By running:* the quickstart on a clean machine with a timer. *By reading:* the latency page. |
| **Split line** | None needed (10). |

#### S11 — The attack gallery: seven examples

| | |
|---|---|
| **Goal** | Every attack the attack-landscape review classes as expressible in the MVP language — A2 EchoLeak, A4 AgentFlayer, A5 GitHub MCP toxic flow, A6(a) malicious MCP server (static form), A7 Nx s1ngularity, A10 CoPhish, A13 the third-party AI-tool breach — ships as a documented, cited, runnable rule app that provably detects its attack and provably ignores a near miss, in the DSL and the Go builder. |
| **Routine · size** | Lighter (consumes shipped constructs only) · 13 SP (items 12) |
| **Depends on** | S10 |
| **In scope** | 1. `gallery/<nn>-<slug>/` × 7: a README (plain-English story; citation; a class tag matching the review's tally; an honest note on the Phase-3 telemetry real deployments need), a synthetic timeline, the rule, its Go twin, its playback log; stories written inside this sprint [9]. · 2. Per app, written from the README's timeline before the engine runs: ≥ 1 non-empty fire-path expect naming the attributes it asserts, and ≥ 1 benign near-miss expect [1.5]. · 3. The conformance runner executes every app; the parity test covers every twin; the gallery joins the grammar-differential corpus and `fmtcheck` [0.5]. · 4. The class-tag script and the in-repo link check [1]. · 5. Attacks the review classes P2, P3 or inexpressible listed as "coming", never faked [incl.]. · 6. An engine defect found here goes to a full-loop fix (the owner may open a sprint) or to known issues with the owner's consent — never fixed under the lighter routine [incl.]. |
| **Not in scope** | Any construct not already shipped. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block (lighter), plus: |
| | 1. All seven apps pass their fire-path and near-miss expects in CI, each with a green twin, in the differential and `fmtcheck`-clean — **proof:** the conformance runner, the parity test, the differential, `fmtcheck`. |
| | 2. Every README's class tag matches the review's tally table — **proof:** the class-tag script. |
| | 3. Every citation link resolves to an in-repo anchor (no network in the gate) — **proof:** the in-repo link check. |
| **Owner's check** | *By reading:* the seven stories for accuracy and defensive framing. *By running:* one gallery app through `cep test`. |
| **Split line** | **S11.1** = four apps (A2, A4, A5, A10); **S11.2** = the other three. S11.1 alone does not meet exit criterion 5 (≥ 5), so S12 waits for S11.2. |

#### S12 — Release close-out: docs, whole-engine break-it and `v0.1.0`

| | |
|---|---|
| **Goal** | The docs cover exactly the shipped subset, the whole engine survives a whole-engine break-it with every finding dispositioned, all five roadmap exit criteria are proven by their named measurements, and the owner tags `v0.1.0`. |
| **Routine · size** | Full loop + the whole-engine break-it (sprint model §14) · 13 SP (items 12.5) |
| **Depends on** | S11 |
| **In scope** | 1. **Docs for the shipped subset** (moved here from v2's S10, so they cover the gallery and every construct): the language reference for shipped constructs, the Go API, the connectors, and the honest limits — no restart survival; at-least-once delivery; per-agent detection only; no `late events allow` until Phase 2 — with the docs-to-conformance check (each documented construct links to a conformance case whose rule model uses it) [4]. · 2. The whole-engine break-it (QA + the-fool): out-of-order and late events, equal timestamps, `within` exactly at its bound, absence timers at the edge, partition-key explosion, malformed input, sink failures, races, invariant violations; the sampled constant-flip spot-check (ADR-0015 §4) [3]. · 3. One fix round; every breaker's red test committed [3]. · 4. `docsync` (a) reverse direction and the §7 file-existence check switched on; every open skip red [0.5]. · 5. The release workflow (tag build with provenance); `docs/releases/v0.1.0.md`; the roadmap and this plan annotated — what shipped, what moved, why; the public-repo step (owner decision) [2]. |
| **Not in scope** | Any new construct. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Every documented construct links to a conformance case whose rule model uses it — **proof:** the docs-to-conformance check (exit criterion 3). |
| | 2. Each of the five exit criteria is proven by its named measurement — **proof:** the exit-criteria table in the release notes, each row linking its proof. |
| | 3. Every break-it finding is fixed with a test, forwarded with the owner's consent, or rejected with a reason — **proof:** the break-it report. |
| | 4. T4 green on the tag candidate; main green after the merge; the owner tags `v0.1.0` — **proof:** CI runs; the tag and its release build. |
| **Owner's check** | *By running:* the quickstart against the tagged build. *By reading:* the release notes and the limits page. *By hand:* the tag. |
| **Split line** | **S12-1** = item 1 (docs); **S12-2** = items 2–5. |

### Still open for the owner

- *None open (2026-11-06): the retro's two rules joined the sprint model; the spec cleanup is S3.2c, with integer overflow decided.*

*Resolved items move to [history.md](history.md#resolved-owner-items).*

### Status

**Phase 1 in progress: S1.1a-1 to S3.2c merged (PRs #1–#21, the last on 2026-11-07). S3.2c closed: SPEC rev 6 — an overflow of any 64-bit arithmetic is an expression error for its rule, `timestamp − timestamp`, the smallest int's literal, every reserved word in Appendix B, the language documents agreeing → next: S3.3's plan check (the Deploy pipeline and the conformance runner — clock freeze).**


*Earlier lines, 2026-11-01 to 2026-11-06: [the status log](history.md#the-status-log). At each step, replace this line and append the old one to the log.*

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

Every owner decision is in **[decisions.md](decisions.md)**: dated, oldest first, with an index. New decisions are appended there.
