# Phase 5 — Spec artefacts

**Goal.** Turn SYNTAX.md and SEMANTICS.md into machine-checkable, model-consumable artefacts.

**Read now.** `references/spec-template.md` (structure of SPEC.md and the companion files).

**Read only if needed.**
- Writing `grammar.lark` → `references/principles/grammar-engineering.md`; `assets/starter/` for a working shape.
- Writing `errors.md` → `references/principles/errors-evolution-implementation.md` and `references/evidence/04-validation-tooling.md` (why the fix line matters).
- Designing the example corpus for retrieval → `references/evidence/09-semantic-parsing-lessons.md`.

**Procedure.**
1. `grammar.lark` first (LALR), then `grammar.ebnf` from it, not the other way round; keep them equivalent.
2. `examples/` with `intent` + program + `expect` per file, tagged by construct; `examples/invalid/` with the expected error code on line 1. Start with the worked examples from SEMANTICS.md.
3. Run `python scripts/grammar_check.py grammar.lark examples --invalid examples/invalid --ambiguity`. Fix every failure and every ambiguity before writing prose.
4. `SPEC.md` per the template; `errors.md` with a code for every static rule; `feature-matrix.md` per target.
5. Execute `expect` blocks against `scripts/ref_executor.py` (extend it with tests if a construct is missing).

**Output.** `SPEC.md`, `grammar.ebnf`, `grammar.lark`, `errors.md`, `examples/`, `feature-matrix.md`.

**Exit check.** Checker passes; every error code has an invalid example; rule count within the cap or justified.

**Next.** `phases/6-evaluation.md`.
