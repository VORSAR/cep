# Principles: error design, implementation complexity, performance, evolution

*Read when:* Phase 5 (errors.md, feature matrix) and whenever estimating the cost of a construct.

## 5. Error design (for humans and repair loops)

- Each diagnostic has a stable code, the rule it violates (by spec section), the location, and a concrete fix. Elm and Rust set the standard for human-facing messages; for an LLM repair loop the *fix suggestion* is the part that moves the number.
- Prefer static checks over runtime failures: mandatory clauses, type checks on durations, bounded-state checks, unknown-name checks.
- Parse errors should name the construct being parsed, not the token ("expected a lateness clause after the window clause", not "unexpected `select`").

## 6. Implementation complexity and performance implications

Estimate before committing to a construct:
- **Parser**: LL(1)/LALR(1) → generator-friendly, cheap. Context-sensitive lexing (e.g. keywords usable as identifiers) → expensive.
- **Static semantics**: type checking with a handful of types is cheap; type inference across joins is not.
- **Semantic gap to the target**: every construct must map to target features. Keep a feature matrix (construct × target) in the spec. A construct that needs emulation on the target is a maintenance liability and a performance hazard.
- **Performance affordances to design out**: unbounded patterns (no `within`), skip-till-any-match on long sequences (NFA state explosion), Kleene closures with no cap, unkeyed global state, regex over strings in hot filters, cross-stream joins without time bounds.
- **Determinism as performance**: processing-time constructs are both non-deterministic and non-replayable; forbidding them simplifies testing and recovery.

## 7. Evolution

- A `language version` pragma at the top of every program so that semantics can change without silent breakage.
- Additive grammar changes only within a major version; new keywords must come from the reserved set.
- Keep a canonical formatter from day one; a single canonical layout is better for diffs, for review, and for the model (one surface form per program).
