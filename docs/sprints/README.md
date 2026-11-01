# Sprint Planning & Documentation

This directory holds the sprint process, the forward sprint schedule, and each sprint's records: its plan check, its record, its ledgers and its break-it reports.

## The two authoritative docs

- **[`sprint-plan.md`](sprint-plan.md): the forward sprint schedule.** What runs next and in what order, with size, scope and dependencies, and the one prose status line (§Status). **Start here for "what's next."**
- **[`00-sprint-model.md`](00-sprint-model.md): the process every sprint runs.** Roles, the full loop and the lighter routine, the gate tiers, break-it, the owner's check, done-done, releases. The *how*, not the *what-next*.

> **Current status is not narrated here.** What shipped is the source + git; what's next is [`sprint-plan.md`](sprint-plan.md#status). The records exist for their **decisions, measurements and rationale**, which code does not carry.

---

## The sprint lifecycle

Which documents a sprint produces, and when. Templates are in [`templates/`](templates/): copy one into `docs/sprints/` under the sprint's file name (its links are written for that location) and replace every `<…>`.

| When | Document | What it holds |
|---|---|---|
| The phase is decomposed | **A sprint card in [`sprint-plan.md`](sprint-plan.md)** ([template](templates/sprint-card.md)) | Goal, in and not in scope, entry criteria, exit criteria with the proof of each acceptance line, size, routine, dependencies, owner's check. Approved by the owner before G0. |
| Step 0, before the owner is asked | **`s<N>-plan-check.md`** ([template](templates/plan-check.md)) | A read-only draft against `main` at a named commit, citing `file:line` on it. What the code says that the plan does not (read first); routine, shape and waves; seams with signatures and stubs; the package that owns each new rule; one owner per file; design notes and the panel's questions; counter-examples; the test plan per lane; risks and order; lane briefs; **questions for the owner, recommendation first**. Not edited after the owner answers: the answers go in the record. |
| The owner answers the plan check (G0) | **A short "how it runs" note under the card** | Lanes (Lane · Items · Agent · Owns · Seams), waves, gates, as decided. |
| The owner answers the plan check | **`s<N>-record.md`** opened ([template](templates/record.md)) | The sprint's running record: decisions at each gate (dated, "≠ R" where they differ from the recommendation), how it runs, measured results as numbers, test material, scope changes, findings with their dispositions, done-done, retro. Filled as the sprint runs; closed at close-out. |
| Step 1 (full loop) | **`docs/design/<topic>/README.md`**, panel reports in `docs/reviews/<topic>/` | The design and its blind reviews. A design that outlives the sprint lives under `docs/design/`, not here. |
| Step 7 | **`s<N>-breakit-<scope>.md`** ([template](templates/breakit-report.md)) | One report per breaker: base, branch, red-test commits, method, findings (where, scenario, evidence, honest likelihood, fix area), split into blockers and later. |
| Lighter routine, instead of a plan check and a record | **`s<N>-ledger.md`** ([template](templates/ledger.md)) | The short plan check, the PR record, deviations, the gate and reviews, the owner's check, the mini-retro. |
| A reusable how-to the sprint produced | **`s<N>-<topic>-procedure.md`** | A procedure people will run again (a soak, a benchmark, a replay capture). |

At close-out, the record's done-done block is ticked, the five-line retro written, and the [§Status](sprint-plan.md#status) line updated.

---

## Naming convention

**Sprint ids:** `S1`, `S2`, … in the order sprints are planned. A sprint split after planning takes sub-numbers (`S3.1`, `S3.2`) so later ids never move. **Ids are never renumbered**: a cut sprint keeps its row, struck through and annotated; the order in `sprint-plan.md` wins over the numbers.

**Sprint files:** `s{N}[-{sub}]-{doctype}.md`, lower case, dots as hyphens.
- `{doctype}` ∈ `plan-check` · `record` · `ledger` · `breakit-{scope}` · `{topic}-design` (a design owned by one sprint) · `{topic}-findings` · `{topic}-procedure`.
- Examples: `s1-plan-check.md`, `s1-record.md`, `s3-2-breakit-patterns.md`, `s4-ledger.md`, `s2-replay-capture-procedure.md`.

**Cross-sprint docs:** `{topic}-{design|plan}.md`, for work no single sprint owns.

**Renaming:** only when a name asserts something false. Never mass-rename for consistency: it breaks links and `git blame`.

**Cross-references:** relative links, with anchors where they help: `[S1 record — decisions](s1-record.md#decisions-at-the-gates)`.

---

## Writing style for sprint documents

- **Plain language, short sentences.** The owner decides from these documents, so an option reads as what a rule author or operator would see, not as an implementation detail.
- **Numbers, not activity.** "p99 4.1 ms over 1 M events", not "performance looked good". A result names its measurement.
- **Mark inference.** A claim not proven by a run or a cited line says "inferred".
- **Mark who decided.** "(owner, 2026-11-02)" on an owner decision; "as recommended" or "≠ R" against the recommendation. The coordinator's own calls are named as such.
- **Annotate, don't rewrite.** A changed line is struck through (`~~…~~`) with the new text and its date beside it.

---

## Records

*None yet. Each sprint adds one line here at its close-out: its id, its title, its documents and its PR.*

---

**Last updated:** 2026-10-31
