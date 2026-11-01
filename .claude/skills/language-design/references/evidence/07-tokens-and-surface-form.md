# Evidence: tokens, whitespace and surface form

*Read when:* someone argues for terse syntax or significant whitespace, or when reporting token cost (Phases 4, 6).

## 6. Tokens and surface form

**Finding.** Significant whitespace is hostile to tokenisation; indentation can be 8–10% of tokens in C-family code, and models are poor at emitting exact whitespace counts (D blog, Aug 2026; Akita, Feb 2026; ImJasonH's LLM-language experiments). SimPy (arXiv:2404.16333) showed ~10% token reduction from a de-sugared Python grammar; Token Sugar (arXiv:2512.08266) ~40% with trained shorthands — but both require training, and §2 shows process cost dominates surface cost.

**Rules.**
- No layout sensitivity. Braces or keyword terminators.
- Do not optimise for token count. Optimise for first-pass validity and for the number of repair iterations. Measure tokens only relative to a baseline (e.g. the same intent expressed in the target engine's native language).
