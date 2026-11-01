# Evidence: natural language to temporal logic (nearest analogue to pattern generation)

*Read when:* designing pattern/absence syntax, deciding on nesting depth, or adding paraphrase and abstention features (Phases 3, 4).

## 7b. The nearest analogue: natural language to temporal logic

Absence, sequence and "within" patterns are temporal-logic formulas in a different coat. The NL→LTL/STL literature is therefore the closest measured analogue to NL→CEP, and it is humbling.

**Finding.** On complex, multi-domain requirements, NeuroNL2LTL (arXiv:2605.22874, May 2026) reaches 27.8% semantic equivalence, beating LLM baselines by 12–20 points; Lang2LTL scores 2.1% because it only handles flat goal sequences and cannot represent nested temporal structure. nl2spec (Cosler et al., CAV 2023) is the interactive-disambiguation baseline; LTLGuard (arXiv:2603.05728) adds a reasoning-based refinement loop. Standard mitigations in this line: decomposition, dynamic prompting, grammar-constrained decoding ("Grammar-forced translation", arXiv:2512.16814), fine-tuning, conformal abstention (ConformalNL2LTL, arXiv:2504.21022).

**Finding.** AeroReq2LTL (arXiv:2604.21715, Apr 2026) beat direct translation on industrial requirements by inserting a *Templated Natural Language* stage: NL → TNL (fixed sentence templates that force implicit timing and conditions into the open) → deterministic rules → LTL. NeuroNL2LTL also generates grounded natural-language explanations from the formula so domain experts validate without reading logic.

**Rules.**
- The surface syntax should *be* the templated natural language: fixed keywords in English sentence order, one sentence per construct, every timing and condition slot mandatory. This is the evidence behind the "read-aloud" rule.
- Keep temporal structure flat. Nested temporal operators are where models collapse; compose by naming patterns and referencing them, not by nesting.
- Specify a deterministic English rendering for every construct (a paraphrase generator) so the compiler can hand reviewers prose next to code. Round-trip (NL → program → NL) is a validation signal.
- Offer abstention: a generator should be able to emit "ambiguous: X or Y?" rather than guess, and the language should have a syntactic place for an unresolved question (e.g. a `todo` clause that fails compilation).
