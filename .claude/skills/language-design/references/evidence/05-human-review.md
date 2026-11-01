# Evidence: human review of generated code, and the CEP reviewability gap

*Read when:* designing the literate layer (intent/expect), the read-aloud test, or justifying explicit clauses to reviewers (Phases 4, 6).

## 5. Human review of generated code

**Finding.** Verifiable Literate Programming (arXiv:2607.02333, July 2026): a literate layer that exposes the concrete semantics of generated code in a form readable without programming knowledge helps users validate and repair LLM-generated code more effectively than clarification-first or test-first human-in-the-loop methods; the bug study behind it found that the failures users need to see are semantic (wrong behaviour on edge cases), not syntactic.

**Finding.** Readability of LLM-generated code is comparable to human-written code on conventional metrics (Atlassian industrial study, arXiv:2501.11264; "The Readability Spectrum", arXiv:2605.13280), but readability metrics do not capture *semantic reviewability* — whether a reader can tell what the program will do.

**Rules.**
- Make the literate layer part of the language: an `intent` docstring and `expect` (given/when/then) blocks on every query. Reviewers read the examples, not the NFA.
- Design for *semantic* review: the reader must be able to answer "what fires, when, on what input" from the text. Explicit emission and lateness clauses are what make this possible.

## 7c. Human review of CEP specifically: what is and is not known

No published study measures non-experts *reading* textual CEP queries. The CEP-language survey (Eckert et al., "A CEP Babelfish", 2011) classifies five styles (composition operators, stream query languages, production rules, timed state machines, logic languages) and notes pattern definition is error-prone even for experts; the only user studies compare *visual* notations (Rapide vs a visual language; a semiotics-based visual DSML, arXiv:1708.05233) and find visual helps novices author rules. Transferable results from query-language usability: reverse-direction predicates are read slower and less accurately (SPARQL studies summarised in arXiv:2204.06277), so patterns should read forward in time. Treat the read-aloud and "what fires when" tests in the evaluation harness as new evidence you are producing, not as confirmation of prior work.
