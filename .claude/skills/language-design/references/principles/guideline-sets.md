# Principles: the canonical guideline sets (Hoare, Tennent, Karsai, Fowler, Voelter, Parr)

*Read when:* Phase 2 (framing) and Phase 4 (as checklists while choosing constructs).

## 1. Canonical guideline sets (use as checklists)

### Hoare, "Hints on Programming Language Design" (1973)
Still the shortest useful list. Rank in this order: **simplicity** (a language a user can hold in their head), **security** (the compiler catches the mistakes it can), **fast translation**, **efficient object code**, **readability**. For an LLM-generated language, "security" becomes "the compiler catches what the model gets wrong" — the single most valuable property you can design for.

### Tennent's principles (1977)
- *Abstraction*: every syntactic category that can be named should be nameable (if you can write a pattern inline you should be able to name it).
- *Correspondence*: declaration and parameter mechanisms should mirror each other.
- *Data type completeness*: no second-class types (durations, timestamps and keys are types, not conventions).

### Karsai et al., "Design Guidelines for Domain Specific Languages" (OOPSLA DSM workshop, 2009)
Twenty-six guidelines; the ones that bite in practice:
- Language purpose: identify the uses and the users early; question *each* feature against a use.
- Realisation: reuse existing language definitions and type systems; compose or extend before you invent.
- Content: **limit the number of language elements**; avoid conceptual redundancy (two ways to say one thing); avoid unnecessary generality; separate abstract and concrete syntax and keep them aligned.
- Concrete syntax: adopt existing notations users know; use descriptive notations; make elements visually distinguishable; use syntactic sugar sparingly; permit comments; provide organisational structure (modules, named units); balance compactness and comprehensibility; be consistent everywhere.
- Abstract syntax: align it with the domain model, not with the parser's convenience.

### Fowler, *Domain-Specific Languages* (2010)
The *semantic model* is the point; the syntax is a façade over it. Design the model (your Phase 3) so that multiple syntaxes could populate it. Keep parsing separate from model population. Distinguishes external DSL (own grammar, your case), internal DSL, and language workbench approaches.

### Voelter, *DSL Engineering* (2013, free at dslbook.org)
Language modularisation and composition; static semantics as first-class (type systems, constraints, name binding); IDE support as part of the language, not an afterthought. The chapters on "expressivity vs. complexity" and "coverage vs. generality" frame the novelty-budget trade-off.

### Parr, *Language Implementation Patterns* (2010)
Practical grammar-to-implementation patterns: which parser class for which grammar, symbol tables, scoping, tree walking. Use when estimating implementation cost of a syntax choice.

### Classic slogans that are actually rules
- **Orthogonality** (Algol 68): features combine without special cases; a window applies the same way to every query form.
- **Principle of least astonishment**: a construct does what its name says and nothing else.
- **One obvious way** (Python's Zen): redundancy is extraneous load for readers and a branching point for generators.
- **Make illegal states unrepresentable**: if a query without a lateness policy is unsafe, the grammar should not accept it.
- **Local reasoning**: the meaning of a statement is determined by its text plus the declarations it names. No global mode switches.
- **Pit of success**: the easiest thing to write is the correct thing. Defaults are design decisions; a dangerous default is a bug in the language, not in the user.
