# Evidence: semantic-parsing lessons (retrieval, plan-then-write, typed holes)

*Read when:* designing the example corpus for retrieval or the prompt protocol for evaluation (Phases 5, 6).

## 7. Semantic-parsing lessons worth keeping

- Retrieval of structurally similar examples is the strongest single lever for in-context generation (MageSQL, VLDB 2025 workshop; ACL 2025 demonstration-retrieval work). Build the example corpus to be retrievable: tag examples by construct used.
- Chain-of-thought / plan-then-write helps on compositional DSL tasks (Program-of-Thought variants in text-to-SQL surveys, arXiv:2410.06011). The language can support this by making the *plan* expressible as named sub-queries.
- Typed holes / static context (Blinn et al., "Statically Contextualizing LLMs with Typed Holes", OOPSLA 2024): feeding the model the expected type at the generation point reduces errors. A mandatory-clause grammar gives the model this for free: after `window`, the next thing *must* be a lateness clause.
