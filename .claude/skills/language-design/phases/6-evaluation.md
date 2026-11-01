# Phase 6 — Evaluation

**Goal.** Numbers, not opinions. Run what can be run; plan the rest concretely.

**Read now.** `references/evaluation-harness.md`.

**Read only if needed.**
- Setting targets or ablations → `references/evidence/01-learning-a-dsl-from-spec.md`.
- Designing the human test → `references/evidence/05-human-review.md` (and note the gap it documents).
- Cognitive Dimensions pass → `references/principles/cognitive-load-and-notation.md`.

**Procedure.**
1. Spec-only generation: parse rate, static-check rate, one-round repair rate, by construct stratum.
2. Executor agreement on generated event streams across the easy/medium/hard ladder.
3. Target conformance if an engine is available.
4. Write the human read-aloud / "what fires when" protocol; run it if readers are available.
5. Report in `EVAL.md` with the table, the failure taxonomy, and the three most frequent failures verbatim.

**Output.** `EVAL.md`.

**Next.** `phases/7-review.md`.
