# DESIGN.md — design record for <language name>

The skill treats everything under **Fixed** as a constraint it works inside. It may flag a concrete conflict between a fixed decision and a principle (with evidence) and ask the designer to rule; it does not reopen fixed decisions on its own initiative.

## Purpose
<One paragraph: what programs in this language do, for whom, and why a new language rather than nothing.>

## Audiences
- **Generator**: <model class(es); is there a compile-and-repair loop; is grammar-constrained decoding available; typical prompt budget>
- **Reader**: <who reviews programs; their background; what they must be able to answer from the text ("what fires when")>
- **Tweaker**: <what they change: parameters only / clauses / structure>

## Unit of design
<single query | whole application (sources, streams, patterns, queries, sinks, params)>

## Compile target(s)
<engine(s); which is primary; whether the spec is target-agnostic with a mapping section>

## Fixed
- We are building this language.
- <e.g. Expressions, operators, literals and comments are SQL, verbatim.>
- <e.g. Event time is the only default clock; processing time is opt-in by keyword.>
- <...>

## Open
- <e.g. Pattern surface: keyword sequence vs MATCH_RECOGNIZE-style regex.>
- <e.g. Whether to support stream–stream joins in v1.>

## Novelty ledger
| Construct | Replaces (prior art) | What it buys | Cost / risk | Status |
|---|---|---|---|---|
| | | | | |

## Decisions log
| Date | Decision | Rationale | Evidence / reference |
|---|---|---|---|
| | | | |

## Artefacts
| Phase | File | Status |
|---|---|---|
| 1 | DESIGN.md | |
| 2 | PRIOR-ART.md | |
| 3 | SEMANTICS.md | |
| 4 | SYNTAX.md | |
| 5 | SPEC.md, grammar.ebnf, grammar.lark, errors.md, examples/ | |
| 6 | EVAL.md | |
| 7 | REVIEW.md | |
