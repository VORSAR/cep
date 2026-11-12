# Sprint Planning & Documentation

This directory holds the sprint process, the forward plan, the owner's decisions and each sprint's documents. A sprint's reviews and break-it reports are in [`../reviews/`](../reviews/).

## The live documents

- **[`sprint-plan.md`](sprint-plan.md): what runs next.** The open sprint cards with size, scope and dependencies, the Phase 1 table, and the one status line (§Status). **Start here for "what's next."**
- **[`decisions.md`](decisions.md): every owner decision**, dated, oldest first, with an index. Read the sections a card or the previous record cites, not the whole file.
- **[`00-sprint-model.md`](00-sprint-model.md): how every sprint runs.** Roles, the full loop and the lighter routine, the gate tiers, break-it, the owner's check, done-done, releases.

**Records, read only for the why:** [`history.md`](history.md) holds the closed sprint cards and the old status lines; the [Records](#records) table below links every closed sprint's documents. Each of those documents opens with a "Historical" line linking its siblings and the sprints before and after it.

> **Current status is not narrated here.** What shipped is the code and git; what's next is [`sprint-plan.md`](sprint-plan.md#status). The records exist for their **decisions, measurements and rationale**, which code does not carry.

---

## The sprint lifecycle

Which documents a sprint produces, and when. Templates are in [`templates/`](templates/): copy one into `docs/sprints/` under the sprint's file name (its links are written for that location) and replace every `<…>`.

| When | Document | What it holds |
|---|---|---|
| The phase is decomposed | **A sprint card in [`sprint-plan.md`](sprint-plan.md)** ([template](templates/sprint-card.md)) | Goal, in and not in scope, entry criteria, exit criteria with the proof of each acceptance line, size, routine, dependencies, owner's check. Approved by the owner before G0. |
| Step 0, before the owner is asked | **The plan check:** a note under the card, "The plan check's readings" (since S1.1b); the first sprints wrote **`s<N>-plan-check.md`** ([template](templates/plan-check.md)) | Read-only, against `main` at a named commit, citing `file:line`: what the code says that the plan does not, then questions for the owner, recommendation first. |
| The owner answers (G0) | **A section in [`decisions.md`](decisions.md)**, a **"how it runs" note** under the card, and **`s<N>-record.md`** opened ([template](templates/record.md)) | The answers (dated, "≠ R" where they differ from the recommendation); lanes, waves and gates; the sprint's running record: decisions at each gate, measured results as numbers, scope changes, findings with their dispositions, done-done, retro. |
| Step 1 (full loop) | **`s<N>-design.md`** here; its review **`../reviews/s<N>-design-architecture.md`**; QA's tests first, in the code | The design note (v1, then v2 placing every finding) and its blind review. |
| Step 7 | **`../reviews/s<N>-combined-architecture.md`** and **`../reviews/s<N>-breakit-qa.md`** ([template](templates/breakit-report.md)) | The combined build's review and the break-it: base, branch, red-test commits, findings (where, scenario, evidence, honest likelihood), blockers and later. |
| Lighter routine, instead of a plan check and a record | **`s<N>-ledger.md`** ([template](templates/ledger.md)) | The short plan check, the PR record, deviations, the gate and reviews, the owner's check, the mini-retro. *(None so far.)* |
| A reusable how-to the sprint produced | **`s<N>-<topic>-procedure.md`** | A procedure people will run again (a soak, a benchmark, a replay capture). |
| Close-out (G5) | The record's done-done and retro; the card moves to [`history.md`](history.md); the [§Status](sprint-plan.md#status) line is replaced and the old one appended to history's log; a row in [Records](#records) | |

---

## Naming convention

**Sprint ids:** `S1`, `S2`, … in the order sprints are planned. A sprint split after planning takes sub-numbers (`S3.1`, `S3.2`) so later ids never move; a split at the plan check adds `-<n>` (`S3.2a-1`). **Ids are never renumbered**: a cut sprint keeps its row, struck through and annotated; the order in `sprint-plan.md` wins over the numbers.

**Files here:** `s{N}[-{sub}]-{doctype}.md`, lower case, dots as hyphens, `{doctype}` ∈ `plan-check` · `design` · `record` · `ledger` · `{topic}-procedure`. Examples: `s3-2a-1-design.md`, `s3-2a-1-record.md`.

**Reviews ([`../reviews/`](../reviews/)):** `s{N}[-{sub}]-{stage}-{reviewer}.md`, `{stage}` ∈ `plan-check` · `design` · `decisions` · `build` · `combined` · `breakit`, `{reviewer}` ∈ `architecture` · `qa`. A one-off question gets a topic (`s2-2a-expr-model-edge.md`); a phase-level review `phase{N}-{topic}-{reviewer}.md`.

**Renaming:** only when a name asserts something false. Never mass-rename for consistency: it breaks links and `git blame`.

**Cross-references:** relative links, with anchors where they help: `[S3.2b-1 record — findings](s3-2b-1-record.md#findings)`.

---

## Writing style for sprint documents

- **Plain language, short sentences.** The owner decides from these documents, so an option reads as what a rule author or operator would see, not as an implementation detail.
- **Numbers, not activity.** "p99 4.1 ms over 1 M events", not "performance looked good". A result names its measurement.
- **Mark inference.** A claim not proven by a run or a cited line says "inferred".
- **Mark who decided.** "(owner, 2026-11-02)" on an owner decision; "as recommended" or "≠ R" against the recommendation. The coordinator's own calls are named as such.
- **Annotate decisions, replace status.** A changed decision is struck through (`~~…~~`) with the new text and its date beside it. Status prose that went stale is rewritten; git keeps the old text.

---

## Records

One row per closed sprint, oldest first. The card each sprint was built from is in [`history.md`](history.md#closed-sprint-cards); its owner decisions are in [`decisions.md`](decisions.md#index).

| Sprint | Title | Plan check · design · record | Reviews | PR |
|---|---|---|---|---|
| — | Phase 1 decomposition | [sprint-plan §3](sprint-plan.md#phase-1-decomposition--v3-recalibrated-to-the-13-point-cap-2026-11-02) | [architecture](../reviews/phase1-decomposition-architecture.md) · [QA](../reviews/phase1-decomposition-qa.md) | — |
| S1.1a-1 | The local gate, coverage and the event kernel | [plan check (S1.1a)](s1-1a-plan-check.md) · [design §1–§11](s1-1a-gate-design.md) · [record](s1-1a-1-record.md) | plan check: [architecture](../reviews/s1-1a-plan-check-architecture.md), [QA](../reviews/s1-1a-plan-check-qa.md) · design: [architecture](../reviews/s1-1a-design-architecture.md), [QA](../reviews/s1-1a-design-qa.md), [re-check](../reviews/s1-1a-design-recheck.md) · [build](../reviews/s1-1a-1-build-architecture.md) · [break-it](../reviews/s1-1a-1-breakit-qa.md) | #1 |
| S1.1a-2 | Hooks and CI | [plan check](s1-1a-2-plan-check.md) · [design §12](s1-1a-gate-design.md#12-s11a-2-the-hooks-and-ci-2026-11-03-g1-approved-by-the-owner-2026-11-04-implement-this-fast-and-get-it-over) · [record](s1-1a-2-record.md) | plan check: [architecture](../reviews/s1-1a-2-plan-check-architecture.md), [QA](../reviews/s1-1a-2-plan-check-qa.md) | #2 |
| S1.1b | `archcheck` (slim) | [record](s1-1b-record.md) | — | #3 |
| S1.2 | The model kernel and spec rev 3 | [record](s1-2-record.md) | — | #4 |
| S2.1 | The parser (participle v2) | [record](s2-1-record.md) | [participle spike](../language/research/participle-spike.md) | #5 |
| S2.2a | The grammar differential and the expression engine | [record](s2-2a-record.md) | [the expr → model edge](../reviews/s2-2a-expr-model-edge.md) | #6 |
| S2.2b | The validator | [design](s2-2b-design.md) · [record](s2-2b-record.md) | [design](../reviews/s2-2b-design-architecture.md) | #7 |
| S3.1-1 | The clock service and the metrics counters | [design](s3-1-1-design.md) · [record](s3-1-1-record.md) | [design](../reviews/s3-1-1-design-architecture.md) · [owner's decisions](../reviews/s3-1-1-decisions-architecture.md) | #8 |
| S3.1-2 | Tables and membership, the ingest flag, the schema decoder | [design](s3-1-2-design.md) · [record](s3-1-2-record.md) | [design](../reviews/s3-1-2-design-architecture.md) · [break-it](../reviews/s3-1-2-breakit-qa.md) | #9 |
| S3.2a-1 | The replay format layer and partitions | [design](s3-2a-1-design.md) · [record](s3-2a-1-record.md) | [design](../reviews/s3-2a-1-design-architecture.md) · [break-it](../reviews/s3-2a-1-breakit-qa.md) | #10 |
| S3.2a-2 | The runtime core | [design](s3-2a-2-design.md) · [record](s3-2a-2-record.md) | [design](../reviews/s3-2a-2-design-architecture.md) · [break-it](../reviews/s3-2a-2-breakit-qa.md) | #11 |
| — | CI: the nightly's summary and the race row; fuzz rows blocking | [decisions](decisions.md#owner-decisions-2026-11-05--the-nightly-build-and-cis-coverage) | — | #12, #13 |
| S3.2b-1 | The plan, replay through the runtime, rate limits | [design](s3-2b-1-design.md) · [record](s3-2b-1-record.md) | [design](../reviews/s3-2b-1-design-architecture.md) · [combined](../reviews/s3-2b-1-combined-architecture.md) · [break-it](../reviews/s3-2b-1-breakit-qa.md) | #14 |
| S3.2b-2 | Multi-part keys, arrival time, the recorder, error counts, the gate's proofs | [design](s3-2b-2-design.md) · [record](s3-2b-2-record.md) | [design](../reviews/s3-2b-2-design-architecture.md) · [combined](../reviews/s3-2b-2-combined-architecture.md) · [break-it](../reviews/s3-2b-2-breakit-qa.md) | #19 |
| S3.2c | The spec cleanup: the language documents agree, and overflow is an error | [design](s3-2c-design.md) · [record](s3-2c-record.md) | [combined](../reviews/s3-2c-combined-architecture.md) · [break-it](../reviews/s3-2c-breakit-qa.md) | #21 |
| S3.3-1 | The public Go API: `cep`, the connector contracts and the API gate | [design](s3-3-1-design.md) · [record](s3-3-1-record.md) | [design: breaker](../reviews/s3-3-1-design-breaker.md) · [challenger](../reviews/s3-3-1-design-challenger.md) · [architecture](../reviews/s3-3-1-design-architecture.md) · [QA](../reviews/s3-3-1-design-qa.md) · [combined](../reviews/s3-3-1-combined-architecture.md) · [break-it](../reviews/s3-3-1-breakit-qa.md) | #23 |
| S3.3-2 | The expect sandbox, the conformance runner, the Tier-2 baselines and the clock freeze | [design](s3-3-2-design.md) · [record](s3-3-2-record.md) | [design: breaker](../reviews/s3-3-2-design-breaker.md) · [challenger](../reviews/s3-3-2-design-challenger.md) · [architecture](../reviews/s3-3-2-design-architecture.md) · [QA](../reviews/s3-3-2-design-qa.md) · [combined](../reviews/s3-3-2-combined-architecture.md) · [break-it](../reviews/s3-3-2-breakit-qa.md) | #28 |
| S1.1c-1 | The law manifest and the LAW Δ block, the current-sprint marker and the not-built expiry, the dependency checks, the error-code counts | [plan check](s1-1c-1-plan-check.md) · [design](s1-1c-1-design.md) · [record](s1-1c-1-record.md) | [scope: synthesis](../reviews/s1-1c-scope-synthesis.md) · [plan check: architecture](../reviews/s1-1c-1-plan-check-architecture.md) · [QA](../reviews/s1-1c-1-plan-check-qa.md) · [design: architecture](../reviews/s1-1c-1-design-architecture.md) · [QA](../reviews/s1-1c-1-design-qa.md) · [combined](../reviews/s1-1c-1-combined-architecture.md) · [break-it](../reviews/s1-1c-1-breakit-qa.md) | #31 (#30) |
| S1.1c-2 | The exemptions and the teeth | [plan check](s1-1c-2-plan-check.md) · [design](s1-1c-2-design.md) · [record](s1-1c-2-record.md) | [plan check: architecture](../reviews/s1-1c-2-plan-check-architecture.md) · [QA](../reviews/s1-1c-2-plan-check-qa.md) · [product](../reviews/s1-1c-2-plan-check-product.md) · [research: vuln and canary](../reviews/s1-1c-2-research-vuln-canary.md) · [fragments](../reviews/s1-1c-2-research-fragments.md) · [design: architecture](../reviews/s1-1c-2-design-architecture.md) · [QA](../reviews/s1-1c-2-design-qa.md) · [combined](../reviews/s1-1c-2-combined-architecture.md) · [break-it](../reviews/s1-1c-2-breakit-qa.md) | #35 (#34) |

---

**Last updated:** 2026-11-06
