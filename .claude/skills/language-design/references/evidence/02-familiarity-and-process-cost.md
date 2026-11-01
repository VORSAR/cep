# Evidence: familiarity and process cost (the novelty budget)

*Read when:* any time you are about to introduce a construct that does not look like SQL/mainstream code (Phases 2, 4).

## 2. The counter-evidence: familiarity and process cost

**Finding.** "Leveraging Code to Improve In-context Learning for Semantic Parsing" (Bogin et al., arXiv:2311.09519): Python-based prompts consistently outperformed DSL-based prompts for the same semantic-parsing tasks, with and without domain descriptions, at every demonstration count; wrapping the DSL in Python syntax recovered most of the gap.

**Finding.** "The Best Programming Language for Tokenmaxxing" (Wu, Anderson, Guha, arXiv:2607.22807, July 2026): at comparable success rates, lower-resource languages cost agents significantly more tokens than Python; the cost comes from agents thrashing against an unfamiliar compiler, over-revising solutions that already pass, and switching to a familiar language to sidestep the unfamiliar one. Final-program length is not the cost driver; reasoning and revision are.

**Finding.** Dan Luu, "How does programming language affect token efficiency and correctness?" (danluu.com/pl-tokens, Aug 2026): the widely-cited claim that terse/dynamic languages are more token-efficient does not survive real tasks; absent the budget to fine-tune a model on your language, a mainstream language beats an obscure dense one.

**Finding.** Survey of LLM code generation for low-resource and domain-specific languages (Joel et al., arXiv:2410.03981): performance drops sharply with resource level; the effective mitigations are retrieval of similar examples, grammar/spec in context, and compiler feedback loops. Fine-tuning vs optimised RAG for DSLs (arXiv:2407.02742): with dynamically selected few-shot examples, RAG matched fine-tuning on quality and reduced hallucinated API names and parameters; hallucinated identifiers remain the dominant residual error.

**Reconciliation and rule.** Sections 1 and 2 are not in conflict once you separate *what is novel* from *how much is novel*. Anka and Text2DSL win because the DSL is small and encodes something mainstream syntax does not express well. Bogin et al. and Luu win where a DSL merely re-skins something Python or SQL already says. Therefore: **a novelty budget**. Every construct that does not look like SQL/mainstream code costs prior and process tokens. Spend it only where the mainstream prior is absent or wrong (for CEP: time semantics, lateness, emission, patterns with absence). Everything else — expressions, operators, literals, comments, projections, filters — must be SQL, verbatim. Also: identifier hallucination is the top residual error, so the spec must carry a *closed* vocabulary (functions, keywords, window kinds) and the compiler must reject unknown names with a fix suggestion.
