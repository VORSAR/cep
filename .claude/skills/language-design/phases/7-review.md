# Phase 7 — Adversarial review of the grammar

**Goal.** Red-team the design (not the decision to design). Find ambiguity, hallucination affordances, semantic traps, reviewability failures, spec gaps.

**Read now.** `references/grammar-review-checklist.md`.

**Read only if needed.**
- Hunting unbounded state / non-determinism → `references/semantics/ordering-determinism-performance.md`.
- Hidden-dependency and viscosity items → `references/principles/cognitive-load-and-notation.md`.

**Procedure.** Apply every item A–F; record *item → finding → fix or justified exception* in `REVIEW.md`; feed fixes back into Phase 4/5 artefacts and re-run the checker and `expect` blocks.

**Output.** `REVIEW.md`; updated artefacts.

**Exit.** Report to the designer: findings count by section, fixes applied, exceptions awaiting a ruling.
