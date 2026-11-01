# Adversarial review checklist for a grammar and spec

Phase 7. Apply every item to `grammar.lark`, `SPEC.md`, `errors.md` and the corpus. Record each finding in `REVIEW.md` as *item → finding → fix or justified exception*. The review targets the design; the decision to build the language is out of scope.

## A. Ambiguity and parsing
- A1. Run `scripts/grammar_check.py --ambiguity` over the whole corpus. Zero ambiguous parses.
- A2. LALR build succeeds with no shift/reduce or reduce/reduce conflicts (or every conflict is documented with the chosen resolution and a test).
- A3. Prefix overlap: can two different statements start with the same two tokens? If so, is the third token decisive?
- A4. Optional-separator traps: any place where a missing comma/semicolon still parses as something else?
- A5. Keyword–identifier collision: can any reserved word appear as an identifier without quoting? (Must not.)
- A6. Literal overlap: `5m` vs `5 m` vs identifier `m`; `1d` vs hex; strings with embedded quotes.
- A7. Precedence: every binary operator has a precedence and associativity stated; no reliance on "obvious".
- A8. Error recovery: with a statement deleted or truncated, does the parser resynchronise at the next `;`/`}` so one error does not cascade?

## B. Hallucination affordances (what a model is likely to invent)
- B1. For each construct, list the three most plausible *invalid* variants a model might emit (synonym keyword, reordered clause, omitted clause, SQL-ism that does not exist here). Confirm each produces a specific diagnostic with a fix, not a generic parse error. Add them to `examples/invalid/`.
- B2. Identifier hallucination: unknown function, window kind, stream or field name → diagnostic lists valid candidates (closest-match).
- B3. Does any construct accept a free-form string where a keyword from a closed set was intended? Replace with the closed set.
- B4. Are there two constructs so similar that a model would confuse them (`then` vs `strictly then`, `emit every` vs `window hopping every`)? Either make them visually distinct or merge.
- B5. SQL habits that must be rejected explicitly: `ORDER BY` on an unbounded stream, `LIMIT`, correlated subqueries, `DISTINCT` on unbounded scope, `NOW()` in event-time queries, `GROUP BY` without a window or TTL.

## C. Semantic traps
- C1. For each window kind, is there an `expect` example that would fail if the boundary were off by one (inclusive/exclusive) or misaligned (hopping on multiples of size instead of hop)?
- C2. For session windows: merge example and late-bridging example present and executed.
- C3. For each pattern operator: an example where skip-till-next and skip-till-any would differ, with the expected output under the chosen default.
- C4. Absence: example where the bound elapses (fires) and where B arrives at exactly the bound (define inclusive/exclusive).
- C5. After-match: an input burst that yields exactly one match under the default policy; the alternative policies' counts stated.
- C6. Late data: an example with an event that is late for one window but on time for an overlapping hopping window.
- C7. Watermark idleness: an example with a stalled partition.
- C8. Null: an example that would silently drop rows under SQL three-valued logic; confirm the spec'd behaviour.
- C9. Processing time: confirm it cannot be used without the keyword.
- C10. Unbounded state: confirm every path to unbounded state is a static error with a code.

## D. Reviewability
- D1. Read-aloud: every construct in the corpus rendered as one English sentence by the paraphrase rules; a reader unfamiliar with the engine can answer "what fires when" from the sentence.
- D2. Hidden dependencies: changing one declaration (a source's watermark delay, a param) — list every query whose behaviour changes; confirm the change is visible from the query text or its named dependencies only.
- D3. Viscosity: renaming a stream, changing a threshold, adding a field — count edits required; each should be local.
- D4. Diffuseness: longest construct in the corpus in tokens; justify anything over ~12 lines.
- D5. Role-expressiveness: can a reader tell clause roles from keywords alone with all identifiers blanked out?

## E. Spec hygiene
- E1. Every static rule in SPEC §5 has an error code; every error code has an invalid example; every invalid example fails with that exact code.
- E2. `grammar.ebnf` and `grammar.lark` are equivalent on the corpus (both accept all valid, both reject all invalid).
- E3. The feature matrix covers every construct for every target; any "emulated" cell has a documented semantic caveat.
- E4. Version pragma, reserved-word list, null rule, and canonical-format rules all appear in the first two pages.
- E5. Novelty ledger in DESIGN.md matches the constructs actually in the grammar.

## F. Generation dry run
- F1. Prompt a model with SPEC.md + 10 examples only; ask for programs for 10 held-out intents (including two with absence and two with sessions). Record parse rate, compile rate (static checks), and executor agreement rate. Anything under the targets in `evaluation-harness.md` is a finding.
- F2. Repair loop: feed each failure's diagnostic back once; record how many are fixed in one round. A diagnostic that does not lead to a fix in one round is a finding against `errors.md`.
