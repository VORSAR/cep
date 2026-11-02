# QA & Test-Automation Strategy — preventing architectural debt

Owner condition at the 2026-11-01 architecture ratification; **revised 2026-11-01** after the foundation consistency review ([review](../architecture/review/foundation-consistency.md) — B3 scoped the metamorphic laws, B6/S7 fixed §4, S10 added the alloc gate, S11 corrected the review-only count) and the gate red-team. This document extends [testing.md](testing.md) (the four rules and the layers stand unchanged) and [ADR-0015](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md) (**the gate's single normative mechanism catalog** — where this file and that ADR differ, the ADR wins); it states the **strategy**: which kinds of assurance exist, what each one is for, and the doctrine that stops the suite itself from rotting. Mechanisms are synthesized from two of the owner's prior projects (studied 2026-11-01, verified first-hand; not named here by instruction).

## 0. Doctrine (binding)

1. **An invariant without an instrument is a claim, not a property.** Every normative sentence in ARCHITECTURE.md and SEMANTICS.md names the test/analyzer that proves it, or stands in ADR-0015 §3.6's review-only residue the owner accepted — no third state. (The §7 traceability table and ADR-0015's instrument table are the ledgers; a sentence without a row or a residue entry is a defect.)
2. **A check that can't fail isn't a check.** Every custom guard, analyzer, and oracle ships with a planted defect it must catch (with a pinned minimum of expectations — ADR-0015 §3.8), and every scan asserts its expected scope against a live expansion (ADR-0015 §3.3). Non-vacuity beats coverage theater.
3. **New tests are first seen failing** against the planted bug (testing.md), and a breaker's red test is committed as a regression — never left in a worktree. Coverage proves execution, never correctness: assertion strength comes from this rule plus the QA agent's independently written failing test at each plan check, with red-run evidence quoted in the PR (ADR-0015 §4).
4. **Agreement between agents is not verification.** Green means the coordinator read the real exit code of a completed run; claims are re-verified on the combined branch (sprint model).
5. **Flaky = broken.** Root-fixed immediately; never retried, never quarantined (ground rule 11).

## 1. The assurance stack (what proves what)

| Layer | Proves | Instrument | Cadence |
|---|---|---|---|
| Architecture analyzers | the layer charter, purity, clock discipline, API partition | ADR-0015 §2 table, each with teeth | T0–T2 |
| Law-change visibility | no gate/spec/architecture rule changes silently | the LAW Δ block (ADR-0015 §1.3), read by the owner at every merge | every tier |
| Conformance suite | SEMANTICS.md construct by construct | golden cases ported from `docs/language/checks/` (each Go case cites its Python source) + every corpus `expect` via playback, incl. `given table` seeding (SPEC §6.8 rev 2; ADR-0005 §4) | T2 |
| Grammar differential | SPEC grammar ≡ engine parser | lark vs goyacc over examples/ + invalid/ + eval/generated: accept/reject verdicts AND canonical operator-spine shapes (ADR-0002 §5, RC7), counts asserted | T4 |
| Determinism | byte-identical replay | same-log same-build byte diff; two-OS T4 job; planted map-iteration fixture must be caught | T3/T4 |
| Allocation gate | the zero-alloc hot path (ADR-0010 §1) | `allocgate`: AllocsPerRun per stage fails on > 0; freelist-warm spawn benchmark | T2 |
| Oracle & property tests | the pattern engine against an independent mind | a brute-force reference matcher (exhaustive over small windows) vs the engine on 200–300 seeded-random streams per shape; counters prove the interesting branches ran (`matches > N`, `defeats > N`); state-space caps recorded as skips, never silent | T3 |
| Metamorphic suite (CEP-specific) | semantics invariances no example can pin | see §2 — each law scoped to where the semantics actually promise it | T3 |
| Stress/race | concurrency seams | `-race` on T3; shuffle + `-count=N` + CPU-hog harness for new concurrency-sensitive tests, pass rate stated in the PR (testing.md) | T3 + per-PR |
| Fuzz | parser and decoder robustness | Go native fuzzing: grammar fuzzer (seeded from the corpus) on the parser; byte fuzzer on the JSON decoder (differential vs encoding/json) | nightly, advisory→blocking per ADR-0015 §3.9 aging |
| End-to-end | the shipped thing | the packaged binary drives the demo scenario; exit-code contract; gallery apps run via conformance | T3/T4 |
| Benchmarks | throughput/latency objectives | ADR-0010 §8: open-loop load, in-repo histograms, benchstat tripwires vs stored baselines | T4 + nightly |

## 2. The CEP metamorphic suite (cheap, high-yield, unique to this engine)

Each law reruns conformance scenarios and compares outputs mechanically — no new expected values to maintain. **Each law is scoped to the scenarios where the pinned semantics actually promise the invariance** (consistency B3 — the unscoped laws were provably false of a correct engine); a scenario excluded from a law is excluded by a tagged, counted rule, never silently:

- **Time-shift invariance:** shift every input timestamp by Δ ⇒ every output shifts by exactly Δ. **Scope:** scenarios with no cron window and no absolute `timestamp '…'` literal (cron buckets are absolute wall-calendar fire times, SEMANTICS §7 — shifted inputs legitimately land in different buckets). Cron scenarios run the variant law instead: shift by a whole multiple of every cron period in the app, outside DST folds. Catches wall-clock leaks and absolute-time bugs instantly.
- **Key-renaming invariance:** bijectively rename partition-key values ⇒ outputs identical under the same renaming. Unscoped — sound as stated. Catches cross-key state bleed.
- **Irrelevant-event insertion:** inject events matching no filter of any rule, **each injected timestamp ≤ its stream's running max at the injection point** (admission watermarks are pre-filter `max(Ts seen) − d`, SEMANTICS §2.1 — a new-maximum injection legitimately advances the watermark and changes later events' lateness) ⇒ byte-identical outputs (except counters). Catches skip-till-next violations and buffer bugs.
- **Batch-size invariance:** rerun with forced chunk sizes {1, 2, 7, whole-log} ⇒ byte-identical (ADR-0010 §2's property, executed here). Unscoped — sound as stated.
- **Admissible-reorder invariance:** permute arrival order within each stream's watermark bound ⇒ identical outputs. **"Admissible" is defined** (B3): under the permuted arrival order, no event changes lateness class against the recomputed watermark trajectory, AND equal-timestamp order within a stream is preserved (per-stream `Seq` is derived from log order — ADR-0005 — and ties break on it, SEMANTICS §2.2). The generator proves admissibility before the run; a permutation that fails the test is a bug, not noise. Exercises the ordering buffer exactly at its contract.

## 3. Golden corpus discipline (two tiers)

- **Tier 1 — invariants:** asserted fresh every run, never recorded (ordering, bounds, null rules — the ported harness checks).
- **Tier 2 — descriptive baselines:** the conformance outputs of the corpus and gallery, baseline-locked; any diff turns the suite red; regeneration happens only via `gate/regen-baseline`, which re-runs Tier 1 first and **refuses to record** a run that violates any invariant, and which **classifies the diff mechanically** (gate red-team GR-23): `additive-new-rule` and `spec-rev` classes may record (with the class printed); `timestamp-drift`, `ordering-change` and `count-change` classes are **refused** — stop and investigate. The prose table in the baseline directory explains the classes; the tool enforces them. The baseline's git history is the changelog of behavior.

## 4. Debt tripwires (drift that fails the build, not the retro)

- Doc-sync: error catalog ↔ validator table; Appendix F ↔ expr registry; ARCHITECTURE package list ↔ directories; reserved words ↔ kwgen (ADR-0015 §2, per sub-check `lands_in`).
- API: the hand-written export list (`api/exports.golden`, a law file) + `apidiff` at T4 against merge-base/last-tag (ADR-0015).
- Coverage: the binary inclusion rule (allowlist-waivable with a recorded reason) + the **90%** per-package statement floor *(owner, 2026-11-02: raised from 85%; enforced from sprint S1.1a, the first Go code)*, **unwaivable within its stated scope** — `internal/*` + the public packages (`cep`, `rule`, `io` and the `io/*` built-ins), generated files excluded (ADR-0015 §2.2; the old "no waiver path" sentence conflated the two rules).
- Dependency allowlist (`tools/deps-allowlist.json`) — an unapproved `go.mod` edit is red.
- Suppression + allowlist accountability: one grammar (ADR-0015 §3.10), expiry enforcement, liveness, counts + deltas in the summary.
- Generated-code freshness (parser, keywords) regenerate-and-diff, scratch-dir only.
- Runtime-metric tripwires are **zero-config-is-failure** (ADR-0015 §3.11): the **outbox ceiling is blocking** (default 65,536, ADR-0006 §5); `partial-count`, `sink-depth` and `drained_pending` are advisory per-rule ceilings set at the plan check — an unset ceiling fails the step; a deliberate opt-out is an explicit `ceiling: none # reason` entry. Advisory findings age into blocking after 2 sprints (ADR-0015 §3.9).

## 5. Review process hooks (people and agents)

Per the sprint model §8: QA writes the failing test at the plan check; the architect reviews each combined diff against ARCHITECTURE §2b rules **5–8 and the rest of ADR-0015 §3.6's review-only residue** (seven items, accepted by the owner 2026-11-01; *an eighth, the gate's Python having no coverage floor, accepted 2026-11-02* — rule 5 is review-only too: hand-computed watermarks use no banned time API, so clockcheck cannot see them); the owner reads the **LAW Δ block** at every merge (sprint model §8 step 10); the break-it phase before each release commits its red tests and may run the sampled constant-flip spot-check (ADR-0015 §4). Every sprint card's exit criteria enumerate the conformance cases it adds — a construct without cases does not ship (`skip: not-implemented(S<n>)` is visible, never green, and fails T4 once its sprint closes).

**Last updated:** 2026-11-01 (created at the architecture ratification; revised same day after the foundation adversarial review).
