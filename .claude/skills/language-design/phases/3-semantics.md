# Phase 3 — Semantics before syntax

**Goal.** Write down what programs *mean* — time, windows, emission, state, patterns — before choosing any keyword. This is where LLM-generated streaming code fails; the language must close those holes here.

**Read now.**
- `references/evidence/03-streaming-failure-modes.md` (what has to be made explicit, with evidence).
- `references/principles/semantic-design.md` (the list of decisions every spec must make).
- `references/semantics/time-and-watermarks.md` (always; sources and every time construct depend on it).

**Read as the construct comes up.** Load each only when writing that section of SEMANTICS.md; unload mentally when done.
- Windows → `references/semantics/windows.md`.
- Emission, accumulation, lateness, output mode → `references/semantics/emission-and-lateness.md`.
- Aggregates, group-by, joins → `references/semantics/state-aggregations-joins.md`.
- Patterns (sequence, absence, repetition) → `references/semantics/patterns.md` then `references/evidence/08-formal-foundations.md` (give each operator a CEL meaning) and `references/evidence/06-temporal-logic-analogue.md` (keep nesting flat).
- Finalising → `references/semantics/ordering-determinism-performance.md`, then `references/semantics/checklist.md`.

**Procedure.**
1. Data model and null rule first (one paragraph each).
2. Time model: clock per source, watermark strategy, idleness. No defaults.
3. For each construct in scope, write its state, its bound, and its firing rule as an abstract-machine rule, plus one worked example that would fail if the rule were slightly wrong (boundary, late event, session merge, absence at the bound).
4. Decide the trigger × accumulation × lateness table and the output-mode derivation rule.
5. For patterns: selection strategy default, after-match default, restart semantics, absence firing on watermark, event-time ordering, late-event disposition, match output shape.
6. List forbidden constructs (unbounded state, processing time without keyword, wall-clock functions).
7. Run `references/semantics/checklist.md`; waive items only by writing the waiver into `DESIGN.md`.

**Output.** `SEMANTICS.md`.

**Exit check.** Every rule has a worked example; the checklist is ticked or waived. No surface syntax has been chosen (illustrative pseudo-syntax is fine if marked as such).

**Next.** `phases/4-syntax.md`.
