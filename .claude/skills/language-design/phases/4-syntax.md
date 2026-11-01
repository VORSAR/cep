# Phase 4 — Concrete syntax

**Goal.** Design the surface, construct by construct, under the rules, recording the novelty cost of each decision.

**Read now.** `references/syntax-rules.md` in full (18 rules + exit checklist). It is the only mandatory read for this phase.

**Read only if needed.**
- A rule's rationale is challenged → the evidence file it names (`references/evidence/0n-*.md`).
- Choosing keyword names → `references/principles/cognitive-load-and-notation.md` (naming, English-like trap, Cognitive Dimensions).
- Deciding grammar class, terminators, literals → `references/principles/grammar-engineering.md`.
- Pattern surface still open → `references/prior-art/match-recognize-flink.md` (regex) vs `references/prior-art/siddhi.md` (keyword sequence).
- Someone wants terse syntax → `references/evidence/07-tokens-and-surface-form.md`.

**Procedure.**
1. Fix the statement spine and clause order once; every statement in the language follows it.
2. For each construct in SEMANTICS.md: write the syntax, one canonical example, its one-sentence English rendering, and its ledger entry (what it replaces, what it buys, what it costs).
3. Mark every hazard clause from SEMANTICS.md as mandatory in the grammar sketch.
4. Add the literate and tooling constructs: `intent`, `expect`, `param`, `todo`, version pragma.
5. Run the exit checklist at the end of `references/syntax-rules.md`.

**Output.** `SYNTAX.md`; ledger updated in `DESIGN.md`.

**Exit check.** No positional access; no optional hazard clause; one way per thing; every construct has an English rendering; grammar sketch has fixed order and terminators.

**Next.** `phases/5-spec-artifacts.md`.
