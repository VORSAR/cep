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

### Phase 1 decomposition — v3, recalibrated to the 13-point cap (2026-11-02)

> **Status of this section.** v2 was approved by the owner on 2026-11-02. At the owner's request the same day, v3 recalibrates it: every card is re-estimated item by item and no card holds more than 13 points. **v3 changes only how the work is cut into sprints — no item was added or removed** except the two the owner added after v2's approval (coverage at 90% in S1.1, the parallel runner). **v3 is APPROVED** (owner, 2026-11-02 — §5 "Owner decisions, 2026-11-02 — recalibration (v3)"). Each sprint still passes its own G0.

**How the points are calibrated.** Story points measure **build work** — the code, its tests and its test material — estimated item by item (each item's estimate is in brackets) against two anchors: **`internal/event` = 1 point; the pattern engine's core (partials, absence watchers, completion timers, the skip rules) = 5 points.** The fixed process every sprint carries — plan check, reviews, break-it, fix round, owner's check, and the window sprints' design panels — is not in the points; the 2-week cap holds it. A sprint holds **at most 13 points** of estimated build work; each card shows its item total and its size rounded up to the Fibonacci scale. v2 sized cards by judgment; counted item by item, ten of v2's twenty cards were over 13, so v3 re-cuts them — splitting where a coherent half exists (new ids carry a letter, e.g. S1.1a/S1.1b, so references to v2's ids elsewhere stay valid) and otherwise moving an item to the neighbour it belongs with.

**How to read the numbering.** ADR-0005 §1's stages keep their numbers; a stage that does not fit one sprint is split before entry (sprint model §3): **S1 = S1.1a + S1.1b + S1.2; S2 = S2.1 + S2.2a + S2.2b; S3 = S3.1 + S3.2a + S3.2b + S3.3; S4 = S4.1 + S4.2; S5 = S5.1 + S5.2; S6 = S6.1 + S6.2; S9 = S9.1 + S9.2.** Every ADR reference to a stage means the stage; the cards name the exact sprint. The clock contract freezes at **S3.3's** G5 ("S3's exit", ADR-0005 §1). **Sprint-id grammar** (settled in S1.2, used by `todocheck`, `suppressions` and conformance skips): `S<stage>[.<part>][<letter>][-<n>]` (the `-<n>` suffix only when a plan check applies a card's split line), ordered by this table's row order; "current + 2" means two rows down. *(Annotated 2026-11-02: S1.1a's plan check applied a split — S1.1a = S1.1a-1 + S1.1a-2 — so S1 = S1.1a-1 + S1.1a-2 + S1.1b + S1.2.)*

**Sequencing principle (owner decision 2026-11-01).** Enablers first (S1.1a–S3.3). Then the **kill-switch vertical**: patterns (S4.1, S4.2) make the flagship fire in playback; connectivity (S5.1) makes it fire live through `cepd`; the Go builder (S6.1) proves DSL/Go parity on it. Breadth follows: the formatter (S6.2), text (S7), windows (S8, S9.1, S9.2), the demo kit and published latency (S10), the attack gallery (S11), the release close-out (S12). **From S6.1 onward every construct ships in both front-ends in the sprint that ships it** (builder surface + a parity twin per example).

| Sprint | Title | SP (items) | Routine | Depends on |
|---|---|---|---|---|
| ~~S1.1a~~ | ~~The gate runner, CI, coverage and the event kernel~~ *split at its plan check, 2026-11-02 (owner, §5): re-sized for the owner's Python decision to about 15 points → S1.1a-1 + S1.1a-2* | ~~13 (12.5)~~ | full | — |
| S1.1a-1 | The local gate, coverage and the event kernel | 13 (11.5) | full | — |
| S1.1a-2 | Hooks and CI | 5 (3.5) | full | S1.1a-1 |
| S1.1b | The analyzers and the law | 13 (13) | full | S1.1a-2 *(was S1.1a; annotated 2026-11-02)* |
| S1.2 | The model kernel, the remaining gate rows and spec rev 3 | 13 (11) | full | S1.1b |
| S2.1 | The parser | 13 (13) | full | S1.2 |
| S2.2a | The grammar differential and the expression engine | 13 (10.5) | full | S2.1 |
| S2.2b | The validator | 13 (11.5) | full | S2.2a |
| S3.1 | The clock service, tables and the decoder | 13 (13) | full | S2.2b |
| S3.2a | The replay format and the runtime core | 13 (11.5) | full | S3.1 |
| S3.2b | One runtime, live and replay — rate limits, errors, arrival time | 13 (12.5) | full | S3.2a |
| S3.3 | The Deploy pipeline and the conformance runner — clock freeze | 13 (13) | full | S3.2b |
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

**Forecast (honest, re-forecast at named points).** **23 sprints; about 270 points of estimated build work** (294 in Fibonacci card sizes). *(Annotated 2026-11-02: S1.1a's plan check split it into S1.1a-1 and S1.1a-2 → **24 sprints, about 272.5 points** (299 in card sizes).)* At 2–3 weeks wall-clock per sprint for a solo owner, Phase 1 is **46–69 weeks**. Several cards sit well under 13 (S1.2, S2.2a, S5.2, S9.2, S10), so the lower half of the range is plausible but unproven. No velocity has been measured: **re-forecast at S1.2's G5 and at S3.3's G5** from recorded points and durations (sprint model §3). A card the plan check sizes over 13 is split into its own two sprints **before** G0 — never by moving work into a neighbouring card that is already full.

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

#### S1.1a — The gate runner, CI, coverage and the event kernel

| | |
|---|---|
| **Goal** | A clean clone runs `make check` — pinned toolchain, parallel registry runner, deterministic summary — locally and in CI, with coverage enforced at 90% on the first Go package, `internal/event`. |
| **Routine · size** | Full loop (the event kernel fixes the null representation and the total order every later output depends on; the gate is an owner ratification condition) · 13 SP (items 12.5) |
| **Depends on** | — (first sprint; phase entry §9 holds: architecture ratified and review-hardened ✔, decomposition approved ✔) |
| **In scope** | 1. The Go module `github.com/vorsar/complex-events-engine` *(amended 2026-11-02, owner: `github.com/vorsar/cep`)*: `go.mod` with `go` and `toolchain` lines, `GOTOOLCHAIN=local` [0.5]. · 2. `tools.json` + one installer (`cache/tmp.$$`, atomic `mv` into the hash-keyed path) + the parity step, first at every tier [2]. · 3. `gate/registry.json` (row schema = ADR-0015 §1.1), `gate/run.sh` (POSIX; per-step process groups; TERM-then-KILL watchdogs; one JSON record per step; `summary.md` + `run.json`), collect-all, zero-is-failure, `expected_scope` checked against the live expansion, the `exit:` line; `make check`, `make combine-check`, `make strict-gate`; **parallel by default** (owner 2026-11-02; ADR-0015 §1.1): rows run concurrently up to a worker limit, each starting when its `needs` finish; `exclusive` rows run alone; the summary stays in registry order and prints wall time beside summed step time [4]. · 4. The `gofmt` and `go vet` rows [0.5]. · 5. `.githooks/` (T0, T1) and the CI workflow (actions pinned by SHA; rows grouped into **concurrent jobs** by their `job` field, each restoring the Go module and build cache keyed by `go.sum` + the `tools.json` hash; the summary job `needs: all`, `if: always()`, the only required check) with summary fixture trees [2]. · 6. `gate/proofs/` as a separate Go module, with this sprint's proofs [1.5]. · 7. **`internal/event`**: `Type`, `Attr`, `Schema`, `Value` (null = `Value{Null:true}` keeping the declared `Kind`), `Event`, the total-order comparator `(Ts, StreamID, Seq)` [1]. · 8. **`coverage-inclusion`** (owner 2026-11-02): every `internal/*` and public package has tests (or a reasoned allowlist entry), and ≥ **90%** statement coverage per package from T2's coverprofile, scope per ADR-0015 §2.2 (generated files excluded), no waiver within scope [1]. |
| **Not in scope** | The architecture analyzers, LAW Δ, registry-lint, suppressions → S1.1b. `internal/model` and spec rev 3 → S1.2. |
| **Entry criteria** | The standard ones, plus (answered by the owner 2026-11-02, §5): toolchain = the newest stable Go that the pinned golangci-lint supports — exact versions recorded in the plan check on day one; the Go specialist is `.claude/agents/go-engineer.md` with JetBrains' `use-modern-go` skill; every agent `model: inherit`; `main` has force-push and deletion blocked before G0 ✔ (ruleset `protect-main`, 2026-11-02), and the summary check becomes a required status check in that ruleset once this sprint's CI has run (before it merges). Hook wiring is ADR-0015 §3.1's (`core.hooksPath .githooks`). |
| **Exit criteria** | The done-done block (§13), plus: |
| | 1. A clean clone runs `make check` to `exit: 0`; CI shows the same rows — **proof:** CI `strict-gate` on the PR and the summary fixture tests classifying every planted tree. |
| | 2. A tool version that differs from `tools.json` fails before any other step and names the fix — **proof:** the parity proof. |
| | 3. The runner really runs in parallel, and safely: two independent fixture rows that each wait for the other's marker file finish only if they run at the same time (run one after the other, the watchdog kills them — red); a row never starts before its `needs` finish; an `exclusive` row never overlaps another; the summary is byte-identical whatever order rows finish in — **proof:** runner fixture tests (no sleeps — marker files and watchdog ceilings). |
| | 4. The parallel saving is measured — **proof:** the summary footer's wall time vs summed step time on the PR's CI run, recorded in the sprint record. |
| | 5. An unformatted Go file is refused at T0 — **proof:** a hook-level proof. |
| | 6. A package below 90% statement coverage, and a package with no tests, each turn the gate red naming the package and its number; `internal/event` is at or above 90% — **proof:** two planted proofs + the T2 run. |
| | 7. The event total order and the null representation are pinned — **proof:** `internal/event` table tests whose expected orderings are written by hand before the code. |
| **Owner's check** | *By running:* clone fresh, `make check`, read the summary and its timing footer; commit an unformatted file and watch the hook refuse it. *By hand:* add the required check on GitHub once CI has run. |
| **Split line** | None needed (12.5); if the plan check sizes it over, CI caching and the timing footer move to S1.1b. *(Annotated 2026-11-02: this split line could not be used: S1.1b is full, and §3's rule forbids moving work into a full neighbour. The plan check re-sized the card for the owner's Python decision at about 15 points, and the owner split it into its own two sprints: S1.1a-1 and S1.1a-2 below. This card stays as the approved original; the plan check is [s1-1a-plan-check.md](s1-1a-plan-check.md).)* |

#### S1.1a-1 — The local gate, coverage and the event kernel

| | |
|---|---|
| **Goal** | A clean clone runs `make check` to `exit: 0` with the pinned toolchain, the parallel registry runner and a deterministic summary; coverage is enforced at 90% on `internal/event`; and every instrument, including the checks on the gate's own Python, is proven able to fail. |
| **Routine · size** | Full loop, no design panel (owner 2026-11-02) · 13 SP (items 11.5) |
| **Depends on** | — (first sprint) |
| **In scope** | All gate tooling is stdlib Python under CPython 3.15.2, pinned (owner 2026-11-02). 1. `go.mod` (`go 1.27.0`, `toolchain go1.27.3`) [0.5]. · 2. `tools.json` (Go 1.27.3, golangci-lint v2.15.0, CPython 3.15.2, ruff 0.17.3; archive and per-executable sha256) + `gate/bootstrap.py` (any Python ≥ 3.9 *(superseded 2026-11-02 by owner decision 9: the installer needs ≥ 3.12, verified under a pinned 3.12.16; the 3.9-syntax launcher only refuses)*; safe extraction; atomic rename into the shared cache) + parity (the hard stop; every listed executable re-hashed every run) [2.5]. · 3. `gate/registry.json` + the runner `gate/run.py` / `gate/runner/` (parallel, `needs`, `exclusive`, watchdogs over process groups, collect-all, zero-is-failure, live expected scope, records keyed by job, a summary deterministic under an injected clock, the timing footer) + `make check|combine-check|strict-gate|ci` [4]. · 4. The `gofmt`, `vet` and `unit` rows [0.5]. · 5. `internal/event` [1]. · 6. `coverage-inclusion` at 90%, classified from `go test -json` events; no waivers until S1.1b [1]. · 7. The `pytests` row, so the gate runs its own tests [0.5]. · 8. The `ruff` and `pyfloor` rows (the gate's Python formatted and linted; the bootstrap parses at 3.9 *(as built: `pyfloor` runs the floor modules' tests under 3.12.16; ruff's `py39` target holds `bootstrap.py`)*) [0.5]. · 9. `gate/proofs/`: parity, coverage ×2, `pytests`, `ruff` [1]. A coordinator-built throwaway runner scaffold serves wave 1 and is deleted when the real runner lands (owner 2026-11-02). |
| **Not in scope** | Hooks and CI → S1.1a-2. The analyzers, LAW Δ, registry-lint, the allowlist grammar → S1.1b. |
| **Entry criteria** | The standard ones; the plan check answered (owner 2026-11-02); `.claude/agents/python-engineer.md` exists. |
| **Exit criteria** | The done-done block, plus (numbering kept from S1.1a): |
| | 1. A clean clone with an empty tool cache runs `make check` to `exit: 0` — **proof:** the coordinator's fresh-clone run, recorded. |
| | 2. A tool whose version or executable hash differs from `tools.json` fails before any other row starts, naming the fix — **proof:** the parity proof (three separate cases: version, not installed, path). |
| | 3. Runner behaviour as on the S1.1a card; the summary is byte-identical whatever order rows finish in — **proof:** runner fixture tests (FIFO-held rows, an injected clock, no sleeps). |
| | 6. A package below 90% and a package with no tests each turn the gate red, naming the package and its number; `internal/event` ≥ 90% — **proof:** two planted proofs + the T2 run. |
| | 7. The event total order and the null representation are pinned — **proof:** `internal/event` tables written by QA, by hand, red before the code. |
| | 8. The gate's own tests run in the gate: a test module that is untracked, fails to import, skips everything or holds no test turns `pytests` red — **proof:** the `pytests` proof. |
| | 9. Unformatted gate Python, and a `time.sleep` outside the wait helper, are red — **proof:** the `ruff` proof. |
| **Owner's check** | *By running:* clone fresh, `make check`, read the summary and its timing footer. *By reading:* the decisions the coordinator flagged as uncertain (owner 2026-11-02). |
| **Split line** | None needed (11.5). |

#### S1.1a-2 — Hooks and CI

| | |
|---|---|
| **Goal** | Every commit and push runs its tier through the same runner, `make check` refuses to run until the hooks are on, and CI runs the registry's rows as concurrent jobs with one required check. |
| **Routine · size** | Full loop · 5 SP (items 3.5) *(re-sized 2026-11-03 by the owner at G0: about 10 SP, items 5.5, then about 8 SP, items 4.5, on 2026-11-04; see the [plan check](s1-1a-2-plan-check.md) §0 and §5 decisions)* |
| **Depends on** | S1.1a-1 |
| **In scope** | 1. `.githooks/pre-commit` (T0 over an index snapshot; refuses an unformatted file and prints the fix; SKIP when nothing is staged) and `.githooks/pre-push` (T1 over the pushed tip), both Python; `make setup`; a row that makes `make check` refuse until the hooks are on (CI sets `core.hooksPath`) [1]. · 2. `gate/ci/`, which generates `.github/workflows/gate.yml` from `registry.json` (`ubuntu-latest` per the owner, actions pinned by SHA, concurrent jobs, module and build caches), plus the `ci-fresh` row [1]. · 3. `summarize` (expected rows taken from the registry) + its fixture trees [1]. · 4. The hook proof and the summary proofs [0.5]. |
| **Not in scope** | The analyzers and LAW Δ → S1.1b. |
| **Entry criteria** | The standard ones (S1.1a-1 merged). |
| **Exit criteria** | The done-done block, plus (numbering kept from S1.1a): |
| | 1. CI shows the same rows as a local `make strict-gate` — **proof:** CI `strict-gate` on the PR, the summary fixture trees, and a diff of the two row lists. |
| | 4. The parallel saving is measured — **proof:** the PR's CI timing footer, recorded. |
| | 5. An unformatted Go file is refused at T0, including under `git commit -a` and `git commit <path>` — **proof:** the hook proof. |
| | 10. `make check` with the hooks off fails, naming `make setup` — **proof:** a proof row. |
| **Owner's check** | *By running:* commit an unformatted file and watch the refusal; `make check` in a clone without `make setup`. *By hand:* add `strict-gate` as a required status check in `protect-main` once CI has run. |
| **Split line** | None needed (3.5). |

#### S1.1b — The analyzers and the law

| | |
|---|---|
| **Goal** | Every architectural rule that can be checked from stage 1's code is mechanically red when broken — imports, clock use, goroutines, globals, TODOs, test hygiene, lint smells, suppressions — each analyzer proven able to fail, and every change to the gate's own rules shows in the LAW Δ block. |
| **Routine · size** | Full loop · 13 SP (items 13) |
| **Depends on** | S1.1a *(annotated 2026-11-02: now S1.1a-2, after the split)* |
| **In scope** | 1. `gate/law.json` + `lawdelta` + the LAW Δ block at the top of the summary (ADR-0015 §1.3) [2]. · 2. registry-lint with its three planted problems and the §2 instrument-id pin [1]. · 3. `importcheck` with `tools/importcheck/rules.json` encoding ARCHITECTURE §3's import lists as **exhaustive** (owner 2026-11-02) and the post-`io`-split edges; the docsync comparison of parsed edges; the edge-count ceiling [2]. · 4. `clockcheck`, incl. a synthetic fixture for the `internal/window/cron.go` exception [1]. · 5. `puritycheck` (module-wide) and `globalscheck` (const-like defined in the analyzer) [2]. · 6. `todocheck` (basic `TODO(owner\|S<n>\|#id)`) and `testhygiene` [1]. · 7. The teeth meta-test (`min_wants` per analyzer) [1]. · 8. golangci-lint strict config (`smells`, `forbidigo`, the §2.1 scope classes) + `nolintlint` [1]. · 9. The `suppressions` checker and the one allowlist grammar under `tools/allowlists/` [2]. Each analyzer ships with its `analysistest` fixture and scans S1.1a's code retroactively on landing. |
| **Not in scope** | `docsync`, the dependency/licence/vulnerability/build-tag rows and the sprint-id grammar → S1.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. A planted edit to `rules.json` appears in the LAW Δ block with its one-line semantic delta — **proof:** `lawdelta` test over a fixture repository. |
| | 2. An upward import from `internal/event` is red at T0 on the staged file — **proof:** `importcheck` fixture + a hook-level proof. |
| | 3. `time.Now()` outside the allowlist is red; the synthetic `cron.go` fixture passes `time.LoadLocation` and fails `time.Now` — **proof:** `clockcheck` fixtures. |
| | 4. A `go` statement outside the allowed packages, an `unsafe` import, and a mutable package-level `var` are each red; an error sentinel is not — **proof:** `puritycheck` and `globalscheck` fixtures. |
| | 5. A bare `//nolint`, a PERMANENT suppression without a resolvable `ref=`, and an allowlist entry naming a non-existent package each fail — **proof:** `suppressions` tests. |
| | 6. Weakening a fixture below its `min_wants`, or deleting a §2 instrument's registry row, fails; registry-lint reports its three planted problems — **proof:** the teeth meta-test + registry-lint proofs. |
| | 7. A `time.Sleep` in a test outside the wait helper is red — **proof:** `testhygiene` fixture. |
| | 8. Every analyzer's retroactive scan of S1.1a's code is green — **proof:** the T2 run. |
| **Owner's check** | *By running:* add a forbidden import on purpose and watch the commit refused. *By reading:* the LAW Δ block on the PR. |
| **Split line** | **S1.1b-1** = items 1–5 (law, imports, clock, purity, globals); **S1.1b-2** = items 6–9. |

#### S1.2 — The model kernel, the remaining gate rows and spec rev 3

| | |
|---|---|
| **Goal** | `internal/model` holds the rule IR with a byte-stable canonical encoding, the remaining stage-1 gate rows are live, and spec rev 3 — the owner's language decisions of 2026-11-01 and 2026-11-02 — is applied and mechanically checked. |
| **Routine · size** | Full loop (the IR and a language revision) · 13 SP (items 11) |
| **Depends on** | S1.1b |
| **In scope** | 1. **`internal/model`**: the IR of ADR-0003 (every field named after its SPEC clause; durations and timestamps `int64` ms; `ParamRef` unresolved) and `MarshalCanonical` (sorted keys, explicit defaults, float `'g'`/−1/64, RFC3339 UTC millis, NaN/±Inf unencodable); **the canonical JSON carries no source positions** — diagnostics keep positions in a side table [3]. · 2. The dependency-allowlist check, the licence check, `govulncheck`, the build-tag law, the weekly canary [2]. · 3. The sprint-id grammar (`S<stage>[.<part>][<letter>][-<n>]` (the `-<n>` suffix only when a plan check applies a card's split line)), `gate/current-sprint`, and the skip / `deferred(P<n>, ref=…)` forms in `todocheck` [1]. · 4. `docsync` (a) one direction (every package directory that exists is listed in ARCHITECTURE §3) and `docsync` (d) error-count consistency [1]. · 5. **Spec rev 3** (language-design skill; docs lane), containing exactly: (a) the derived-stream-consumption error code (owner 2026-11-01); (b) **the cross-stream partition-key type-mismatch code** (RC8.1; owner 2026-11-02: reject at deploy — the error names both streams and both types); (c) **`late events allow` removed for v0.1** (owner 2026-11-02): `allow` is rejected wherever it appears, with E-LATE-002's text revised to "late handling per window arrives in Phase 2; use `late events drop` and set the stream's watermark bound"; example 02 changes to `late events drop`; SEMANTICS §7's cron row annotated; (d) **cron admission pinned to SPEC §6.1**: an event behind its stream's admission watermark is dropped and counted even if its cron bucket is still open — the Python reference executor's `cron_window` is aligned and a new harness check pins it, written red-first; (e) **`text_invisibles` is engine-owned** (owner 2026-11-02): the name is reserved; the engine always computes it; a payload value is ignored, overwritten and counted; a stream may declare it as `bool` for readability; (f) **keyword sets over a table operand read the table's per-event snapshot like `in`** (owner 2026-11-02); (g) `time arrival` stays (owner 2026-11-02): playback logs carry the recorded arrival stamp as `ts`; (h) E-GRAPH-001 marked unreachable in v0.1 and kept for Phase 2, with `docsync`'s 1:1 rule accepting the marking; (i) errors.md, SPEC Appendix C and the change log updated [4]. *(Added 2026-11-02 by owner decision at S1.1a-1's design review: (j) a stream's `timestamp = <attr>` must name a `timestamp`-typed attribute, as a coded deploy error with fix text [incl.]. The item total stays about 11; the plan check re-sizes it.)* |
| **Not in scope** | Expect blocks for the examples → each example's own sprint. The SPEC §9 timing-comment sentence → S6.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. `MarshalCanonical` is byte-stable and position-free — **proof:** a hand-derived canonical-JSON golden (key order, −0, 1e21, millisecond truncation, NaN/±Inf rejection) written before the code, plus a test that two models differing only in source positions encode identically. |
| | 2. An unapproved `go.mod` edit, a disallowed licence, a known-vulnerable module and a `//go:build !race` file are each red — **proof:** one planted proof each. |
| | 3. A `TODO` naming a sprint more than two rows ahead, and a skip naming the current sprint, are red — **proof:** `todocheck` fixtures driven by a fixture `gate/current-sprint`. |
| | 4. Spec rev 3 is applied: every decided code is present in errors.md with its SPEC § reference and fix text; the counts agree everywhere — **proof:** `docsync` (d) green and red on a planted count mismatch; `grammar_check` PASS; the harness passes with its new cron-admission check, first seen failing against the unaligned executor. |
| **Owner's check** | *By reading:* the rev-3 change log entry and the revised E-LATE-002 text; the canonical JSON of one model. *By hand:* the LAW Δ block (spec files changed). |
| **Split line** | **S1.2-1** = items 1–4; **S1.2-2** = item 5. |

#### S2.1 — The parser

| | |
|---|---|
| **Goal** | `internal/parser` turns every corpus file into a `model.App` or into the exact catalogued parse error at the exact line and column, and keeps every comment. |
| **Routine · size** | Full loop (the language) · 13 SP (items 13) |
| **Depends on** | S1.2 (rev 3 applied) |
| **In scope** | 1. The hand-written lexer (ADR-0002 §3), positions in runes, **every comment kept as positioned trivia in a side table reachable from the parse result** (S6.2's printer needs it), with its lossless round-trip test and a comment-torture file [3]. · 2. `tools/kwgen/` → `internal/parser/keywords.go` (generated header) [1]. · 3. `cep.y` hand-derived from `grammar.lark`; `parser_gen.go` committed (generated header); the AST; lowering to `model.App` [5]. · 4. The `yyError` mapping for all [parse] codes, with line, column and fix text [2]. · 5. **Expected positions:** before the parser runs, the QA agent adds a hand-derived `@ <line>:<col>` to the first-line comment of each of the 24 files in `docs/language/invalid/` [1]. · 6. `freshness` (goyacc + kwgen; scratch-dir regeneration; header-manifest anti-forgery) [1]. *(Annotated 2026-11-04, owner: built on participle v2 — items 1, 3, 4, 5 as read for participle (the grammar as struct tags, no `cep.y`, no `parser_gen.go`); item 2 becomes a pinning test; item 6 is moot. See §5, 2026-11-04.)* |
| **Not in scope** | The grammar differential, the parser fuzz target and the nightly workflow → S2.2a. Static codes and typing → S2.2a/S2.2b. The printer → S6.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. All 9 examples, the generated eval files and the torture files parse; each model's canonical JSON is golden-locked — **proof:** the parser corpus test. |
| | 2. Each invalid file yields exactly its [parse] code at exactly its `@ line:col` — **proof:** the invalid-corpus table test. |
| | 3. Lexing and re-assembling every corpus, torture and comment-torture file from tokens plus trivia reproduces it byte for byte — **proof:** the lossless trivia test. |
| | 4. A stale `parser_gen.go` and a forged generated header are each red — **proof:** `freshness` proofs. *(annotated 2026-11-04: moot, no generated file; participle)* |
| **Owner's check** | *By running:* a deliberately broken rule file through the parser test helper (command named in the plan check) — read code, line, column and fix text. *By reading:* the messages for E-KW-002, E-PAT-001 and E-STR-001. |
| **Split line** | **S2.1-1** = items 1, 2, 5 (lexer, keywords, positions); **S2.1-2** = items 3, 4, 6. |

#### S2.2a — The grammar differential and the expression engine

| | |
|---|---|
| **Goal** | The engine's parser is proven equivalent to the normative lark grammar over the full corpus — by verdict and by parse shape — and every expression form in SPEC §5.10 is typed, with the scalar forms compiled into executable trees. |
| **Routine · size** | Full loop (the language) · 13 SP (items 10.5) |
| **Depends on** | S2.1 |
| **In scope** | 1. The grammar-differential T4 row (ADR-0002 §5 as amended): pinned Python + `lark==1.3.1`; verdicts and canonical operator-spine S-expressions; ≥ 10 torture files in `docs/language/eval/generated/torture/`; corpus count asserted [3]. · 2. The parser fuzz target and the nightly workflow (one advisory, aging row per fuzz target) [1]. · 3. `docsync` (b) reserved words ↔ kwgen [0.5]. · 4. `internal/expr` (ADR-0011): **typing for every SPEC §5.10 form** — scalar, `in`, text operators, `chain`, `count` — exported to `validate`; **executable nodes for** scalar, arithmetic, comparison, boolean, null handling and the functions `coalesce`, `minimum`, `maximum`, `abs`, `math:round` (half-up, per Appendix F — not Go's `math.Round`), `cast`, `ifThenElse`; the `Registry` with all nine Appendix F callables (the `in` nodes land in S3.1, `chain` in S4.1, text nodes in S7, `count` in S8) [4]. · 5. The per-function evaluation table [1]. · 6. `docsync` (c) Appendix F ↔ `expr.Registry`; `allocgate` first rows [1]. · 7. The 1.1 null-strict check's scalar assertions ported [incl.]. |
| **Not in scope** | The validator → S2.2b. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The differential is green with counts at or above the known minimum, and red on a planted precedence swap in `cep.y` — **proof:** the differential row + its proof. *(annotated 2026-11-04: the planted swap goes in the participle grammar structs)* |
| | 2. Every E-TYPE-001 row of SPEC §5.10 has a table test — **proof:** `internal/expr` typing tests. |
| | 3. Every function evaluates per Appendix F: `math:round` at ±0.5 and ±1.5; `cast` success and failure per target (a counted runtime error, never a panic); integer `/` and `%`; `minimum`/`maximum` over timestamps and durations; `ifThenElse` with a null condition — **proof:** the per-function table, expected values written before the code. |
| | 4. Scalar evaluation allocates nothing except listed exclusions (`cast` to string) — **proof:** `allocgate`. |
| | 5. `docsync` (b) and (c) go red on a planted drift — **proof:** their fixtures. |
| **Owner's check** | *By reading:* one differential S-expression pair for a precedence torture file. |
| **Split line** | None needed (10.5). |

#### S2.2b — The validator

| | |
|---|---|
| **Goal** | Every corpus program validates clean, and every reachable [static] code fires on its own minimal program with the catalogued fix text, line and column. |
| **Routine · size** | Full loop · 13 SP (items 11.5) |
| **Depends on** | S2.2a |
| **In scope** | 1. `internal/validate`: `catalog.go` (every [static] code with fix text, incl. rev 3's); one check per code; E-TTL-002 for pattern bounds and rate intervals; the E-NULL lints; E-REGEX-001's pre-scan [5]. · 2. Static test material in `internal/validate/testdata/` — one minimal program per reachable [static] code, **including the derived-stream and key-type codes** [1]. · 3. The connector annotation table `connectors.go` (E-NAME-001; `punctuates`; E-SINK-001; E-WIN-002) [1]. · 4. **The stream-graph checks** E-GRAPH-001/002/003, the derived-stream rejection and the cross-stream key-type check live here (ADR-0012 §1 as annotated) [1.5]. · 5. `${param}` resolution [1]. · 6. Pinned at the plan check, each with a named testdata program: whether a stream with no `@source` punctuates (E-WIN-002 for 04's `LoadedRecords`); how a `string` param becomes a list (05's `${risk_terms}`) [0.5]. · 7. Harness ports: RT-14, "9 · TTL deploy check", RT-07 (pattern and rate parts) [1]. · 8. `docsync` (b) [static] codes ↔ `catalog.go` [0.5]. |
| **Not in scope** | E-EXP-001 → S3.3. E-TXT-002 → S7. E-WIN-003 and the cron part of E-TTL-002 → S9.1. Each is a visible `skip: not-implemented(<sprint>)`. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Each reachable [static] code fires on its testdata program with its fix text, line and column; no corpus example raises any code — **proof:** the validator table test + the corpus-clean test. |
| | 2. A rule reading a derived stream, and a partition block whose key is text in one stream and a number in another, are each rejected naming what the error says — **proof:** their testdata programs. |
| | 3. `docsync` (b) goes red on a planted catalog drift — **proof:** its fixture. |
| **Owner's check** | *By reading:* the fix texts of five codes the owner picks. *By running:* one invalid program per picked code through the validator test helper. |
| **Split line** | None needed (11.5). |

#### S3.1 — The clock service, tables and the decoder

| | |
|---|---|
| **Goal** | The clock service implements ADR-0004 completely — every timer kind and the discard path exercised — tables give snapshot-per-event reads, and JSON decodes by schema with full integer range, all proven by unit tests whose arithmetic matches the harness. |
| **Routine · size** | Full loop (event time, watermarks, ordering, lateness) · 13 SP (items 13) |
| **Depends on** | S2.2b |
| **In scope** | 1. `internal/clock`, all of ADR-0004, incl. **the discard path** (a partial's `within` deadline is a discard timer kind routed to `OnDiscard`, never `OnTimers`) and **a test-double operator** that schedules every timer kind and the discard kind; clock-level unit tests of admission strictness, the clamped min, per-stream virtual floors and idle exclusion, each citing its source harness check [6]. · 2. `internal/metrics` counters [0.5]. · 3. `internal/table` and the `in` / `not in` membership nodes (null-strict), completing the 1.1 check's `in` assertion [2]. · 4. `internal/text.IngestFlags` (invisible flagging only) and the Unicode-version agreement check; `BuildInfo`'s Unicode fields [1]. · 5. `internal/event/decode.go`, the schema-driven JSON decoder, with its differential test against `encoding/json` and a fuzz target written before any optimization [3]. · 6. The `puritycheck` named exceptions for `internal/table` and `internal/event` with fixtures [0.5]. |
| **Not in scope** | The replay log format and the runtime → S3.2a. Harness ports → none here. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The clock's arithmetic matches each cited harness check — **proof:** `internal/clock` unit tests, each naming its source check. |
| | 2. Through the test-double operator, every timer kind fires in due order within (b), discards arrive only in (c), and a pending completion is never discarded — **proof:** the timer-contract test. |
| | 3. A two-key registry + `Instance` trace: one key silent, its construct clock still advances through the broadcast `WMAdvance` — **proof:** the quiet-key clock test. |
| | 4. A table write is visible to the next evaluation, never the current one — **proof:** the table snapshot test. |
| | 5. The decoder agrees with `encoding/json` on accept/reject and values, and keeps `int64` above 2^53 exact — **proof:** the differential test; fuzz nightly. |
| **Owner's check** | *By reading:* the timer-contract test's table of timer kinds and their order. |
| **Split line** | **S3.1-1** = items 1, 2 (clock); **S3.1-2** = items 3–6. |

#### S3.2a — The replay format and the runtime core

| | |
|---|---|
| **Goal** | The engine's moving parts exist and hold under load: a deterministic replay-log format, per-key partitions with TTL, and sharded per-key execution with non-blocking routing. |
| **Routine · size** | Full loop (concurrency, partitioned state) · 13 SP (items 11.5) |
| **Depends on** | S3.1 |
| **In scope** | 1. `internal/playback`'s **format layer**: the log format — events; punctuation records; batch-end records `{"batch_end":"<id>","stream":"<name>"}`; table records applied at their file position — the loader, the virtual-clock `Driver`, the canonical output encoder, and a **canonical trace** (released events, timer fires, late counters) [2.5]. · 2. `internal/partition` (ADR-0006): `Instance`, `OpState`, key extraction via a compiled `expr.Expr`, the TTL purge timer and re-arm, `LastActivity`, the state registry, the timer-derivability invariant [3]. · 3. `internal/runtime`: one ingest/router goroutine; shards with bounded mailboxes, per-key serial; the non-blocking outbox handoff; the router servicing every handoff while blocked; sink queues and the runtime-owned sink flusher goroutines (calling a sink through an internal function type the facade later adapts); the key-less pseudo-key; panic recovery at the shard boundary; **an injectable idle-tick source** for the live driver [6]. |
| **Not in scope** | Rule plans, the replay drive loop and rule semantics → S3.2b. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Loading the same log twice yields a byte-identical canonical trace, and a planted map-iteration order turns it red — **proof:** the trace test + its proof. |
| | 2. A table record changes membership from the next event onward, never for the current one — **proof:** a table + trace test. |
| | 3. Progress with a full mailbox at a shard whose handoff is pending, plus a slow sink and a bursty source — **proof:** the saturation test (test-double operators) under `-race` and a CPU hog, 50/50 passes. |
| | 4. One TTL timer per instance, re-armed on activity, never one per event — **proof:** the case guarding the Siddhi purge defect class. |
| **Owner's check** | *By reading:* the saturation scenario and its pass rate. |
| **Split line** | **S3.2a-1** = items 1, 2; **S3.2a-2** = item 3. |

#### S3.2b — One runtime, live and replay: rate limits, errors, arrival time

| | |
|---|---|
| **Goal** | A plain filter rule wired through `internal/plan` runs live (sharded) and in replay (the same runtime, one shard, stepped synchronously) with identical per-key output, and rate limiting, errors, arrival time and the invisible-character flag behave exactly per SEMANTICS. |
| **Routine · size** | Full loop (concurrency; changes what rules emit) · 13 SP (items 12.5) |
| **Depends on** | S3.2a |
| **In scope** | 1. `internal/plan` (minimal): the `StreamNode` graph consuming the validated graph, the routing table, filter-rule operator chains [2]. · 2. **The replay drive loop = the runtime with one shard and a synchronous step API** (ADR-0005 §3), re-running S3.1's and S3.2a's lines through it [1.5]. · 3. **`time arrival`** (owner 2026-11-02): live stamping at ingest; replay of the recorded stamp [1]. · 4. **`text_invisibles` engine-owned** (owner 2026-11-02): always computed; a payload value ignored, overwritten and counted [0.5]. · 5. **Error semantics** (SEMANTICS §10): per-rule expression errors; the stream error policy; null key; unknown attributes skipped and counted [1.5]. · 6. The emission stage with **rate limiting** (`output first every`; anchors in `partition.Instance`; suppressed outputs counted) and the three §8 harness ports [2]. · 7. Race step + goleak proof; the metric-tripwire step (outbox ceiling blocking); `allocgate` ingest → filter [2]. · 8. The two-OS determinism proof [1]. · 9. **Test material for 07 and 09** (QA, before the engine runs): fire-path and near-miss expect blocks — 09 gains a second event inside its rate-limit interval; they run green in S3.3 [1]. |
| **Not in scope** | `cep.Deploy`, the connector contracts and the conformance runner → S3.3. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The same log run live (N shards) and in replay (one shard) gives identical per-key output order — **proof:** the live/replay equivalence test. |
| | 2. Two rules on one stream: one divides by an attribute that is 0 on one event; the other still fires on that event; per-rule `errored_events` counts it; a null key, a missing non-optional attribute, an unknown attribute and a failed `cast` are each counted where SEMANTICS §10 says — **proof:** the error-semantics table test. |
| | 3. A payload without `text_invisibles` gets the computed flag; a payload saying `false` over text containing a zero-width space yields `true` and increments the override counter — **proof:** the ingest-flag test. |
| | 4. A `time arrival` stream: live events are stamped at ingest in arrival order; replaying the recorded log reproduces the output byte for byte — **proof:** the arrival-time test. |
| | 5. A second output inside the rate-limit interval is suppressed and counted; a key-less rule uses one global anchor — **proof:** the three §8 ports. |
| | 6. A planted data race, a planted goroutine leak, and a single `Advance` emitting past the outbox ceiling are each red — **proof:** three proof rows. |
| | 7. The two-OS byte diff is green and catches a planted map-iteration nondeterminism — **proof:** the T4 proof row. |
| | 8. Zero allocations per event from ingest through filter evaluation — **proof:** `allocgate`. |
| **Owner's check** | *By reading:* the error-semantics table and the live/replay equivalence scenario. |
| **Split line** | **S3.2b-1** = items 1, 2, 6, 9 (plan, replay loop, rate limits, material); **S3.2b-2** = items 3–5, 7, 8. |

#### S3.3 — The Deploy pipeline and the conformance runner — clock freeze

| | |
|---|---|
| **Goal** | A filter rule deployed through the public `cep.Runtime.Deploy` runs with its expect blocks executed as a deploy stage, the conformance runner provably fails on wrong expectations, and the clock contract is frozen under the gate. |
| **Routine · size** | Full loop (the public Go API) · 13 SP (items 13) |
| **Depends on** | S3.2b |
| **In scope** | 1. The public facade `cep` (minimal): `Runtime`, `Config`, `Deploy` (parse → validate → plan → expect sandbox → register), `AppHandle` `Start` / `Stop` (drain barrier to a fixpoint, `drained_pending`) / `Undeploy`, `Runtime.Close` (reverse deploy order), multiple apps per runtime, `Input` with **`SendRow` only**, the `Table` API, `BuildInfo` [4]. · 2. **The connector contracts as the leaf package `io` (package `cepio`)** — `Source`, `Sink`, `Mapper`, `RowWriter`, `EmitFunc`, `OutEvent`, `Mode`, and **the batch-close signal** (designed here; an API Δ) — plus `io/inmemory` [1.5]. · 3. **The expect sandbox in `internal/playback`**, called by `cep.Deploy` and `conformance/`: a fresh sandbox per block, `given table` seeding through table records [2]. · 4. The conformance runner `conformance/` (public API only); E-EXP-001 lifted from S2.2b's skip; the six planted wrong-expect proofs [2]. · 5. `apipartition` (`api/exports.golden`: `cep` + `io`) + `apidiff` against the merge base [1.5]. · 6. `gate/regen-baseline` + Tier-2 baselines [1.5]. · 7. `api/clock.golden` added to `gate/law.json`; **the clock contract freezes at this sprint's G5** [0.5]. |
| **Not in scope** | Patterns → S4.1. HTTP and the other connectors → S5.1, S5.2. |
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

- *Approval of the v3 recalibration — **done** (2026-11-02); the sprint references in ADR-0005 §1, ADR-0012 §1, ADR-0015 and qa-strategy §4 are updated to v3's ids.*
- *Owner action — **done** (2026-11-02, verified via the GitHub API): ruleset `protect-main` is active on the default branch with deletion and force-push (non-fast-forward) blocked and no bypass actors. S1.1a adds the summary job as a required status check to the same ruleset once its CI has run.*

*Each sprint gets a card in this section ([template](templates/sprint-card.md)). Once its plan check is answered, a short "how it runs" note follows the card: lanes, waves, gates.*

### Status

**Architecture baseline RATIFIED and review-hardened (2026-11-01). Phase 1 decomposition APPROVED (v2, 2026-11-02) and recalibrated the same day (v3, APPROVED: 23 sprints, every card ≤ 13 points by an item-by-item estimate, ~270 points, forecast 46–69 weeks) → next: S1.1a's plan check (G0 for the first sprint).** *(Annotated 2026-11-02: S1.1a's plan check is answered, and S1.1a is split into S1.1a-1 + S1.1a-2 (24 sprints) → next: S1.1a-1's design note and G1.)* *(Annotated 2026-11-02: S1.1a-1 passed G1 → next: the coordinator's first commit, then wave 1.)* *(Annotated 2026-11-03: S1.1a-1 merged and closed (G5): the local gate, coverage and the event kernel are on main → next: S1.1a-2's plan check (G0), hooks and CI.)* *(Annotated 2026-11-03: S1.1a-2 passed G0 → next: its design (G1), then the build in the main session.)* *(Annotated 2026-11-04: S1.1a-2 built; CI green on Linux; PR #2 awaits the owner's check and merge → next: S1.1b.)* *(Annotated 2026-11-04: S1.1a-2 merged; S1.1b slimmed by the owner to the engine-facing checks (`archcheck`); the rest → S1.1c → next: S1.2, the model kernel.)* *(Annotated 2026-11-04: S1.2 merged (the model kernel, spec rev 3) → S2.1, the parser, on participle v2 by the owner's decision.)* *(Annotated 2026-11-04: S2.1 built: every corpus file parses to a locked model, every invalid file to its exact code and position → PR for the owner → next: S2.2a.)* *(Annotated 2026-11-04: S2.2a built on top of S2.1's branch: the grammar differential is green over all 63 corpus files and red on a planted precedence swap; `internal/expr` types every §5.10 form and runs the scalar forms without allocating; the nightly tier, the fuzz row and the `python-package` kind are in → PR after #5 merges → next: S2.2b, the validator.)* *(Annotated 2026-11-04: S2.1 and S2.2a merged and closed → S2.2b passed G0 (slim with an architecture review, by the owner) → next: its design note, reviewed, then the build.)* *(Annotated 2026-11-04: S2.2b built: every reachable [static] code fires on QA's independent programs at its position with its fix text, the examples are clean, T4 green → PR for the owner → next: S3.1.)* *(Annotated 2026-11-04: S2.2b merged and closed; S3.1 split by the owner into S3.1-1 (the clock) and S3.1-2 → S3.1-1 passed G0 → next: its design note, reviewed, with QA's tests first.)*

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
2. **Dependencies approved:** `golang.org/x/tools` (goyacc, build-time), `golang.org/x/text` (runtime), `golang.org/x/perf` (benchstat, build-time). Everything else in-repo per ADR-0009. *(annotated 2026-11-04: goyacc dropped for participle v2, approved by the owner at S2.1; `x/tools` not yet used)*
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

### Owner decisions, 2026-11-02 — answered ahead of S1.1's plan check

1. **Import rules are strict** (recommended, accepted): the per-package import lists in ARCHITECTURE §3 (rules 3, 4, 6 and the exception table) are exhaustive — only listed connections are allowed; every new edge is a `rules.json` change visible in the LAW Δ block (architecture review DA-11's known edges are added in the sprints that need them).
2. **`conformance/` uses the public API only; `bench/` may import internal packages** (recommended, accepted): nothing imports either. Conformance proves the real product path; benchmarks can time internal stages for `allocgate` and ADR-0010 §8.
3. **Toolchain = newest stable** (recommended, accepted): the newest stable Go release that the pinned golangci-lint supports, verified and pinned on S1.1's first day; later bumps are deliberate commits that also fix what the new version finds.
4. **Go specialist = our own agent + the JetBrains skill** (recommended, accepted): `.claude/agents/go-engineer.md` carries the project's rules and runs JetBrains' `use-modern-go` skill (the `modern-go-guidelines` plugin the owner installed 2026-10-31 — a style skill, not an agent) before editing any Go file; where the two conflict, the gate wins.
5. **Agents always inherit the session's model** (owner's own answer, replacing both offered options): every agent definition uses `model: inherit`; no cheaper model for any lane.
6. **Branch protection: now, plus the required check in S1.1** (recommended, accepted): the owner blocks force-pushes and deletion of `main` now; once S1.1's CI has run once, the gate's summary check is added as required before S1.1 merges.
7. **Code coverage starts in S1.1 at a 90% floor (≠ R on the level).** Recommendation was to move `coverage-inclusion` into S1.1 (accepted) at the ratified 85%; the owner raised the per-package floor to **90%** (amends the 2026-11-01 ratification's 85% — ADR-0015 §2 and §4 and qa-strategy §4 annotated). Scope is unchanged (`internal/*` + the public packages incl. the `io/*` built-ins; generated files excluded; no waiver within scope). S1.1 grows; its plan check confirms the cap or applies the split line.
8. **Build and tests run in parallel wherever they can** (owner requirement): the gate runs independent rows concurrently (respecting `needs`), with only timing-sensitive rows marked `exclusive`; Go builds and tests parallelise across and within packages over a shared cache; CI runs its jobs concurrently with cached modules and builds. The summary stays deterministic (registry order) and reports the time saved. ADR-0015 §1.1 (new `exclusive` row field) and DEVELOPMENT.md amended; proven in S1.1 (exit lines 11–12).

### Owner decisions, 2026-11-02 — recalibration (v3)

1. **v3 approved** (recommended, accepted): every card re-estimated item by item against two anchors (`internal/event` = 1 point; the pattern-engine core = 5) with the fixed per-sprint process outside the points; at most 13 points per sprint. Ten of v2's twenty cards were over the cap; v3 has **23 sprints, ~270 points, forecast 46–69 weeks**, re-forecast at S1.2's and S3.3's G5. No item added or removed. Also moved by the first-subject rule: `puritycheck` and `globalscheck` to S1.1b (their first subject, `internal/event`, exists from S1.1a); the full docs for the shipped subset to S12 (so they cover every construct and the gallery).
2. **Commit and push** (recommended, accepted).

### Owner decisions, 2026-11-02 — S1.1a plan check (G0)

The plan check ([s1-1a-plan-check.md](s1-1a-plan-check.md), v2) was reviewed blind by the architect reviewer (31 findings) and QA (36), all dispositioned there, none rejected. Asked in plain language with pros and cons; recommendation first.

1. **The gate's tooling is stdlib Python, not shell and not Go (≠ R:** a Go runner was recommended). This covers the runner, summary, installer, parity, coverage, hooks and CI generator. A system `python3` (3.9 or newer) only bootstraps the pinned interpreter. The owner's standing preference is Python over shell for every utility script.
2. **The pinned interpreter is CPython 3.15.2** (owner: "latest and greatest"), the python-build-standalone build of 2026-10-26.
3. **S1.1a is split into S1.1a-1 and S1.1a-2** (recommended, accepted). Re-sized for Python, it was about 15 points. The card's split line would have overloaded S1.1b, which is full.
4. **The pre-commit hook refuses an unformatted file and prints the fix** (recommended, accepted). This annotates ADR-0015 §3.1 and DEVELOPMENT.md's T0 row, which said auto-fix.
5. **At T0 only, a check whose staged scope is empty shows SKIP** (recommended, accepted). Docs-only commits pass; everywhere else, zero is a failure.
6. **No three-reviewer design panel for S1.1a** (recommended, accepted). The design note gets the architect + the-fool and QA review.
7. **Build agents write the gate's code; the coordinator reviews it, with a human gate wherever the coordinator is uncertain (≠ R:** the owner's own wording). The recommendation was that the owner read the whole gate diff in place of LAW Δ. This annotates ADR-0015 §1.4 for this sprint.
8. **A Python build agent, `.claude/agents/python-engineer.md`** (`model: inherit`; recommended, accepted).
9. **ruff 0.17.3 pinned** (MIT, build-time) to format and lint the gate's Python, with a ban on `time.sleep` outside the wait helper. The gate's Python has no statement-coverage floor, accepted as residue (recommended, accepted; ADR-0015 §3.6, ADR-0009 annotated).
10. **The coordinator builds a minimal runner before wave 1 (≠ R), as a throwaway scaffold (≠ R).** The recommendations were stand-in checks for wave 1, then a seed that Lane A grows. The scaffold is deleted when Lane A's runner lands.
11. **Defaults.**
    - **Accepted as recommended:** Go 1.27.3 and golangci-lint v2.15.0 (in `tools.json` now); `go.mod` says `go 1.27.0`; no coverage waivers until S1.1b; the shared tool cache `~/.cache/complex-events-engine/tools`; run output in a git-ignored `.gate/runs/<run_id>/`; the workflow generated from `registry.json`; platforms darwin-arm64 and linux-amd64.
    - **Hooks (≠ R):** `make check` refuses to run until the hooks are on; CI turns them on too. The recommendation was `make setup` once.
    - **CI image (≠ R):** `ubuntu-latest`. The recommendation was a pinned `ubuntu-24.04`. *Consequence, stated:* GitHub moves `latest` on its own schedule, so a CI change can arrive with no commit of ours; the summary records the image version on every run.
12. **The S1.1a-1 and S1.1a-2 cards approved** (recommended, accepted); S1.1a-1 passed G0.

### Owner decisions, 2026-11-02 — the Vorsar name (during S1.1a-1's design step)

1. **The module path is `github.com/vorsar/cep` (owner: "package names should reflect vorsar").** Asked with three options; the owner chose the shorter branded path, then the exact path as recommended. This reverses the brainstorm's decision 5 for the module and repository names, and the brainstorm, ADR-0001 §1 and ARCHITECTURE §3 are annotated. Go package names stay short: `cep` at the root, `rule`, `cepio`, `internal/*`.
2. **The GitHub repository `VORSAR/complex-events-engine` is renamed `VORSAR/cep`, using the `gh` CLI** (the owner chose the CLI over the web settings). GitHub redirects the old URL, and the `protect-main` ruleset stays with the repository.
3. **The gate's local names follow it** (as recommended): the tool cache is `~/.cache/vorsar/cep/tools`, overridable by `VORSAR_CEP_TOOLS_CACHE`, and every gate environment variable has the prefix `VORSAR_CEP_`. This amends item 11's cache path above. **Done 2026-11-02:** the repository is `VORSAR/cep`, the ruleset `protect-main` is still active, and the local remote is updated.

### Owner decisions, 2026-11-02 — S1.1a-1 design review (toward G1)

The design note ([s1-1a-gate-design.md](s1-1a-gate-design.md)) was reviewed blind. The architect + the-fool found 24 issues (4 blocker / 14 should-fix / 6 later); QA's review was still running when the owner answered. Asked with options, pros and cons; every answer as recommended.

1. **(b+) for ARCHITECTURE §4's two zero-value traps.** `TInvalid` becomes the zero `Type`. `Schema`'s fields become private behind one validating constructor, `NewSchema`, read through accessors. §4 is annotated at G1.
2. **"The time attribute must be a timestamp" becomes a coded deploy error in S1.2's spec rev 3** (new item (j); scope added to S1.2 by owner decision). `NewSchema` enforces the same rule internally meanwhile.
3. **Wave 1's scaffold runs its own short check list** of plain commands the coordinator writes, under the pinned Python with process-group cleanup. The real registry arrives with Lane A's runner at combine-W1, and the scaffold and its list are deleted.
4. **`make check` (and every gate target) installs any missing pinned tools first.** Exit line 1 holds as written.
5. **The gate's Python never silences ruff inline.** ruff runs with `--ignore-noqa`, and the only exceptions are per-file entries in the coordinator-owned `ruff.toml`.
6. **Unpinned residue accepted:** git, make, and the system `python3` that only starts the installer. `CGO_ENABLED=0` for every row, so Go never calls a C compiler; S3.2b's race-detector row turns it on for that row alone. This is added to ADR-0015 §3.6 as residue item 9.
7. **A focused re-check before G1:** one fresh agent probes the revised sections and the blocker fixes against the real tools.
8. **The coordinator renamed the repository with `gh`** (done and verified, above).
9. **The installer's minimum Python is 3.12, verified by a pinned CPython 3.12.16** (recommended, accepted; asked after QA's review showed that the `ast` 3.9 check accepts newer syntax, and that no pinned 3.9 build exists). Apple's built-in 3.9.6 is refused, with the fix text. The bootstrap's own tests run under the pinned 3.12.16, the fifth pinned tool.
10. **G1 approved for S1.1a-1** (recommended, accepted) on design note v2.1, after the focused re-check. That re-check found 3 more blockers, which were fixed and re-verified.
11. **Parity re-hashes whole tool trees at T2 and above, and executables only at commit or push** (recommended, accepted), through a pinned `tree_sha256` per tool.

### Owner decisions, 2026-11-03 — S1.1a-1 review and break-it (step 7)

The combined build was reviewed blind:
- **Architect + the-fool:** 0 blocker / 4 should-fix / 10 later.
- **QA break-it:** 2 blocker / 2 should-fix / 6 later. Red tests are committed for 6 of them.

The coordinator re-verified the blockers' red tests. All four answers below match the recommendations.

1. **Fix-round scope:**
   - Fix both blockers (QB1: test modules no row runs; QB3: a FIFO in the cache hangs parity) and every should-fix item.
   - Also fix the cheap later items: a truncated archive accepted; a parity record cut at 300 characters; the registry used not recorded; Ctrl-C before the scheduler starts; the `make` exit-code doc note.
   - Move later-sprint items to their sprints: the nested `go.mod` check and the scope-class map → S1.1b; the hook scopes → S1.1a-2. The rest go to known issues with reasons.
   - Leftover-process risk that the fix cannot remove (e.g. a SIGKILL of the runner itself) is recorded as accepted residue.
2. **A red row's summary shows its first failure line,** and the summary prints the run's directory.
3. **A skipped Go test fails the `unit` row,** as Python skips already fail `pytests` ("a skip is never green"). S1.2's skip grammar may later admit only the named `not-implemented(S<n>)` form.
4. **Every record carries a `blocking` field.**
5. **The coordinator makes the fix round itself, in the main session** (later the same day; ≠ the plan check's "agents write gate code; the coordinator reviews"). The fix agents had stalled after rate limits. Each fix is test-first, with one commit per finding.
6. **No separate review of the fix round** (≠ R: a fresh QA reviewer was recommended, because the coordinator wrote the fixes and ground rule 14 says nobody reviews their own work). The fixes stand on their tests, each seen red first, and on the three green tiers.

### Owner decisions, 2026-11-03 — S1.1a-2 plan check (G0)

The plan check ([s1-1a-2-plan-check.md](s1-1a-2-plan-check.md), v2) was reviewed blind by the architect (2 blocker, 6 should-fix, 6 later) and QA (2 blocker, 13 should-fix, 1 later). Both reviews' blockers changed the design: a commit is checked by its own gate, in a tree outside the work tree. All four answers match the recommendations.

1. **The card is re-sized from 5 to about 10 points** (items 3.5 → 5.5): T0/T1 support in the runner, the checked tree, a `pre-merge-commit` hook and CI's record checks. It stays one sprint, under the cap.
2. **The coordinator builds the sprint in the main session.** Fresh, time-boxed agents write QA's red tests first and do the step-7 review.
3. **CI may use four GitHub Actions, and only these:** `checkout`, `cache`, `upload-artifact`, `download-artifact`, each pinned by commit SHA (MIT-licensed). They are the project's first outside dependencies.
4. **A commit runs only the fast checks on its staged files** (parity's executables, `gofmt`, `ruff`); `pyfloor` runs at push.
5. **The hooks check in place, with a guard** (2026-11-04, as recommended in a follow-up question; replaces plan v2's separate checked tree as "overkill" for a convenience check). A staged file with unstaged changes is refused; a push of anything but a clean HEAD is left to CI. The size falls to about 8 SP.
6. **G1: "implement this fast and get it over"** (2026-11-04): the owner approved the design (gate design §12) and asked for speed.
7. **No step-7 review or break-it for this sprint** (2026-11-04; ≠ item 2 and the sprint model's step 7): "we don't need to break it as per say. there are utility stuff." The stopped reviewer had reported nothing. CI's green run and the proofs stand in.

### Owner decisions, 2026-11-04 — S1.1b slimmed (toward engine code)

1. **S1.1b keeps only the checks engine code needs from its first line** (as recommended; the owner wants engine code fast): the import law, the clock rule, goroutines/channels/sync/I-O/unsafe, and package-level vars. Built as one standard-library Go program, `tools/archcheck`, with `tools/importcheck/rules.json` and one gate row, `archcheck` (no new dependency). About 5 points.
2. **The rest of S1.1b moves to a later sprint, S1.1c,** sized at its own G0: `law.json`/`lawdelta` and the LAW Δ block, registry-lint, `todocheck`, `testhygiene`, the teeth meta-test, golangci-lint `smells` + `nolintlint`, and the suppressions checker with the allowlist grammar. Until then, a package-level var outside the four allowed forms has no allowlist: it is red.
3. **"Don't make this overly complicated":** one plan in the record, no plan reviews, no step-7 review (utility code, as for S1.1a-2).

### Owner decisions, 2026-11-04 — S1.2 cut to the model kernel and spec rev 3

1. **S1.2 = `internal/model` (the IR and its canonical JSON) + spec rev 3** (≠ R: the recommendation was the model kernel alone, with spec rev 3 just before the validator sprint). The owner kept spec rev 3 beside the model because the model's fields are named after spec clauses.
2. **The gate extras move to S1.1c:** the dependency-allowlist, licence and `govulncheck` checks, the build-tag law, the weekly canary, the sprint-id grammar with `gate/current-sprint` and the `todocheck` forms, and `docsync` (a) and (d).

### Owner decisions, 2026-11-04 — S2.1, the parser: participle v2 (≠ ADR-0002's goyacc)

1. **The engine parser is built on `github.com/alecthomas/participle/v2` v2.1.4** (MIT; two helper modules in its graph), the project's first library dependency, after a time-boxed spike the owner asked for ([participle-spike.md](../language/research/participle-spike.md)): the whole grammar in about 250 lines of struct tags, the corpus exact (21 valid, 24 invalid) at lookahead 1, positions in runes, trivia round-tripping, 22 of 24 error codes mapped with one misreported position to repair. Chosen over a hand-written parser (no dependency, exact errors, 4–5× the code) and goyacc (a generated file, `x/tools`, the hardest error path). ADR-0002 is annotated, not rewritten: no `.y` grammar, no generated parser, no `freshness` row for it; the keyword table is pinned to `grammar.lark` by a test instead of a generator.
2. **S2.1 runs slim**, like S1.1b and S1.2: the card's items 1, 3, 4 and 5 (lexer with trivia, grammar and lowering, the error map with positions and fix text, QA's hand-derived `@ line:col` on the invalid corpus); item 2 (`kwgen`) becomes the pinning test; item 6 (`freshness`) is moot.

### Owner decisions, 2026-11-04 — S2.2a runs in full

1. **S2.2a as planned, all seven items (10.5 points)** (≠ R: the recommendation was the expression engine plus the grammar differential, about 7 points, with the fuzz target and nightly workflow, `docsync` (b) and (c) and `allocgate` deferred). The owner chose the full card.

### Owner decisions, 2026-11-04 — S2.2a's gate shapes

1. **`tools.json` gains a `python-package` kind** (as recommended): an entry with no command, for a pure-Python library the gate's Python side needs. Parity checks its version by importing it under the pinned CPython, and hashes its files and whole tree as for any tool. First use: `lark` 1.3.1 (MIT, the normative grammar's engine) for the grammar differential.
2. **The registry gains a `nightly` tier** (as recommended): rows marked `nightly` run only in a second generated workflow, scheduled and runnable by hand, never on pull requests; `summarize` judges them as it judges T4. First use: the parser fuzz row.
3. **Advisory aging for the fuzz row, for now** (as recommended): a crash makes the nightly run red and uploads the crashing input; the owner commits it as a regression test, which blocks every run from then on; the two-sprint clock of ADR-0015 §3.9 is a note in known-issues until the gate has persistent advisory state (planned with S1.1c's LAW Δ work).
4. **The `expr → model` import edge is allowed, fenced** (as recommended, after the adversarial review the owner asked for: [s2-2a-expr-model-edge.md](../reviews/s2-2a-expr-model-edge.md)). ADR-0011's design has `expr.Build` take a `model.Expr`, which its own import list forgot; the edge points downward (both are L0 data) and every alternative is a converter or mirror tree of 160–400 lines. The exception-table row reads "expr reads `model.Expr` only"; `rules.json` fences the edge to the model's expression types (`Expr`, `ExprKind`, `Literal`, `LiteralKind`, `TextOperand`) and `archcheck` enforces the fence; Build's contract: names come only from the planner's scope, `${param}` is resolved before Build, tables are handed in by the planner.

### Owner decisions, 2026-11-04 — S2.2a closed; S2.2b's plan check

1. **S2.2a's two deviations are accepted** (as recommended): `docsync` (c) and `allocgate` stay Go tests run by the `unit` row; the differential's corpus count is 63.
2. **S2.2b runs slim, with an architecture review** (≠ R: the recommendation was slim with QA's tests first and no reviews): QA writes the test programs first and independently, from the spec and not from the build; an architecture reviewer reviews the design note before the build; the coordinator builds in the main session; no build lanes.
3. **A stream with no `@source` can mark batches** (as recommended): its connector is chosen at deployment, and the replay log carries batch boundaries (SEMANTICS §10). `E-WIN-002` fires only when `@source` names a connector type that cannot mark batches. Example 04 and eval file t07 stay as written.
4. **A `string` param in a keyword-set slot is a comma-separated list** (as recommended): split at each comma, trim surrounding spaces; an empty item is `E-PARAM-001`. A term containing a comma needs a table operand. SPEC §4.1 gains this one-line clarification.

### Owner decisions, 2026-11-04 — S2.2b's design

After the architecture review and QA's test programs ([s2-2b-design.md](s2-2b-design.md) v2):

1. **Positions live on the model nodes** (A1, as recommended; the model's doc had said "in a side table"): fields of type `model.Pos`, filled by the lowering and skipped by `MarshalCanonical`, so the canonical JSON and the goldens do not change. `TestMarshalCanonicalNoPositions` is rewritten to pin that; ADR-0003 §1 is annotated; Apps compare by canonical JSON only.
2. **Which code fires for a stream with a time clause that a rule inserts into depends on its use** (A2, as recommended): read by a rule or carrying `@source` → `E-GRAPH-002` at the insert target; otherwise → `E-TIME-002` at its `time`. Never both.
3. **Test files of `internal/validate` may import `internal/parser`** (A4, as recommended): `rules.json` gains `test_imports`, enforced by `archcheck`; production code stays forbidden.
4. **`E-WIN-002` is unreachable in v0.1** (as recommended): every built-in source punctuates; errors.md says so, and a unit test over a planted entry proves the check.
5. **`E-TBL-001` also covers a keyword-set operand naming a table with more than one column** (as recommended).

### Owner decisions, 2026-11-04 — S2.2b closed; S3.1's plan check

1. **S3.1 runs as S2.2b did, plus a QA break-it of the clock** (≠ the card's full loop; as recommended): QA's tests first and independent; an architecture review of the design note; the build in the main session; then one QA agent tries to break the finished clock.
2. **S3.1 is split along its card's line** (≠ R: the recommendation was one sprint, one PR): **S3.1-1** = items 1, 2 (the clock service and the metrics counters); **S3.1-2** = items 3–6 (tables and membership, the text flags, the decoder, the purity exceptions). Two sprints, two PRs, one at a time.

### Owner decisions, 2026-11-04 — S3.1-1's clock (after two architecture reviews)

The design review ([s3-1-1-design-architecture.md](../reviews/s3-1-1-design-architecture.md)) showed ADR-0004's interface, the reference executor and SEMANTICS disagreeing; the owner chose, each time, the most accurate option, then had their architectural impact reviewed ([s3-1-1-decisions-architecture.md](../reviews/s3-1-1-decisions-architecture.md)) and settled what that review raised:

1. **Walk forward in time.** When a rule's clock advances, its pending work is done in event-time order: everything at an instant before anything later; within one instant, released events, then the timers due then, then the strict deadlines (the `within` discard, the TTL purge) once the clock is past it. Results no longer depend on how far one advance jumps. Replaces ADR-0004 §2's three steps; the reference executor is corrected (it dropped stale partials only at the end of a jump).
2. **One clock per rule (per shard), shared by its keys**, so a key first seen late starts at the rule's current time (ADR-0004 §1 gave each key its own clock from −∞).
3. **Timers fire on time when every input is quiet** (SEMANTICS §2.1's pin of 2026-11-01 holds; ADR-0004 §4's "an idle stream leaves the min" goes): a quiet stream's contribution moves forward.
4. **A match completes at its last event** when every absence deadline is already behind that event; only a match waiting out an absence window completes by timer (ADR-0013 §3).
5. **A quiet stream's clock runs from its own last event time at wall-clock speed** (≠ "the wall clock minus its bound", which loses a lagging source's events after every quiet spell): its floor = last event time + wall time elapsed since that event arrived − its bound, monotone; a stream that never sent anything starts from the deployment's start time. Wall time enters only through idle ticks that queue behind the events in the runtime's intake.
6. **Fail-closed when every source is down** (product decision): silence is silence, the alert fires on time; a per-source "no data" counter shows which source went quiet.
7. **Seal the instant:** once a key's timers at an instant have fired, a later-arriving event at that instant for that key is construct-late — dropped and counted — so live and replay differ only as counted drops (SEMANTICS §2.1).
8. **Skip-past-last compares in §2.2 order** `(ts, stream id, seq)`, not by timestamp alone (SEMANTICS §6.5): a partial that began after the completed match's last event survives, however the match completed.

*Id mapping for the records above (annotation, 2026-11-02):* v2's S1.1 → v3's S1.1a (runner, CI, coverage, event kernel) + S1.1b (analyzers, LAW Δ) — so "S1.1" in the coverage and parallel-run decisions above means S1.1a, and their exit lines are S1.1a's exits 3, 4 and 6; v2's S2.1 differential → S2.2a; v2's S2.2 → S2.2a (expressions) + S2.2b (validator); v2's S3.1 playback format → S3.2a; v2's S3.2 → S3.2a (runtime core) + S3.2b (replay loop, errors, arrival, flags); v2's rate limiting → S3.2b; v2's S4.1 and-groups, live quiet key and drain → S4.2.
