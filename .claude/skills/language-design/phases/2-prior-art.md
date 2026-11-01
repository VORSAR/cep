# Phase 2 — Prior-art harvest

**Goal.** Compute the novelty budget: what is inherited verbatim, what is deliberately changed, what is left out — and why, in one line each. Not a feasibility review; the decision to build is fixed.

**Read now.**
- `references/evidence/02-familiarity-and-process-cost.md` (why every deviation costs).
- `references/prior-art/inherit-break-table.md` (the starting table; copy it).
- `references/prior-art/formal-languages-and-taxonomy.md` (pick one primary language style).

**Read only if relevant.**
- `references/prior-art/siddhi.md` — Siddhi is a target or known to readers.
- `references/prior-art/match-recognize-flink.md` — Flink is a target, or pattern surface is open.
- `references/prior-art/esper.md` — you need consumption/multiplicity vocabulary.
- `references/prior-art/ksqldb-streaming-dbs-beam.md` — non-Flink targets, or precedent for emission/lateness clauses.
- `references/principles/guideline-sets.md` — the user wants the design grounded in named guidelines.

**Procedure.**
1. For each language in scope (targets + what readers know), fill inherit / break / leave-out rows. "Inherit" means copy the construct and its name; "break" needs the reason and the replacement; "leave out" needs the reason.
2. Choose the primary language style (stream-query spine + composition-operator patterns is the default recommendation) and record it as a decision.
3. Write the first ledger entries in `DESIGN.md` for every "break".

**Output.** `PRIOR-ART.md`; ledger entries in `DESIGN.md`.

**Exit check.** Every construct the user has mentioned appears in the table. Every break has a replacement and a reason.

**Next.** `phases/3-semantics.md`.
