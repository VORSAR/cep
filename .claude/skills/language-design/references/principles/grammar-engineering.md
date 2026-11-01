# Principles: grammar engineering (class, ambiguity, keywords, terminators, layout)

*Read when:* Phase 4 and Phase 5 when writing grammar.lark; Phase 7 ambiguity review.

## 4. Grammar engineering

- **Grammar class**: aim for a grammar an LL(1) or LALR(1) generator accepts without conflicts. Conflicts are usually ambiguities users will feel as surprises. LL(1)-friendly grammars are also cheap for grammar-constrained decoding.
- **Ambiguity**: test with an Earley parser in ambiguity-reporting mode on the whole corpus (`scripts/grammar_check.py`). Classic sources: dangling else, optional separators, prefix overlap between constructs, operator precedence left to "the obvious reading".
- **Keywords vs symbols**: keywords are self-describing and tokenise well; symbols are dense and overload easily. Use SQL's symbol set for expressions and keywords for everything structural.
- **Reserved words**: reserve generously up front (including words you plan to use later). Un-reserving is free; reserving later breaks programs.
- **Case sensitivity**: SQL is case-insensitive for keywords; most modern languages are case-sensitive. Pick one and apply it everywhere; document it in the first page of the spec.
- **Terminators**: a mandatory statement terminator (`;`) or block braces removes a whole class of ambiguity and makes partial programs parseable for error recovery.
- **Comments**: `--` line comments as in SQL, plus `/* */`. Comments are where humans leave review notes; do not make them awkward.
- **Layout**: never significant.
- **Lexical specifics to decide**: identifier charset and quoting, string escapes, numeric literal forms, duration and timestamp literals, whether identifiers can shadow keywords (no).
