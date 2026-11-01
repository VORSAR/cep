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

> **Exit criterion:** the roadmap's four ([roadmap — exit criteria](../product/02-roadmap.md#exit-criteria-definition-of-shipped)): the demo runs end to end from the public quickstart in under 15 minutes; conformance and unit tests green for every shipped construct, with detection → kill-switch latency measured and published; docs for the shipped subset only, tagged `v0.1.0`; the same detection authored in the DSL and the Go API, with identical behaviour proven by one test.

**Phase entry** ([sprint model §9](00-sprint-model.md#9-phases-entry-and-exit)): the architecture baseline written and reviewed (architect + the-fool), ADRs in `docs/architecture/`; Phase 1 decomposed into sprint cards the owner approved. **Phase exit:** the four criteria above, plus a whole-engine break-it (`v0.1.0` is a release).

**Execution order:** enablers first. The gate and playback mode come before the pattern operators ([roadmap — the slice](../product/02-roadmap.md#everything-required-the-slice-by-feature-area)).

| Sprint | Title | SP | Routine | Depends on |
|---|---|---|---|---|
| — | *Not yet decomposed* | — | — | — |

*Each sprint gets a card under this table ([template](templates/sprint-card.md)): goal, in and not in scope, entry and exit criteria with the proof of each acceptance line, owner's check. Once its plan check is answered, a short "how it runs" note follows the card: lanes, waves, gates.*

### Status

**Architecture baseline RATIFIED and review-hardened (2026-11-01, docs/architecture/ — 15 ADRs; review round 4 = gate red-team + foundation consistency: 49 findings, all dispositioned, 4 owner decisions logged in §5; one forwarded item: the spec rev-3 error-code batch at S1's plan check) → next: Phase 1 decomposed into sprint cards for owner approval → S1's plan check.**

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
