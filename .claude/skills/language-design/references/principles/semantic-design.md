# Principles: semantic design decisions a spec must make (types, null, scoping, units, errors)

*Read when:* starting Phase 3; use as the list of decisions to write down before syntax.

## 3. Semantic design

Write these down before syntax. Each is a decision; "whatever the target does" is not a decision.
- **Types**: nominal stream schemas; scalar types; a real `duration` and `timestamp` type; what arithmetic is allowed across them.
- **Null**: SQL three-valued logic is a trap for both humans and models (`x <> 5` silently excludes nulls). Options: forbid nulls in event schemas; make null handling explicit (`is null`); use two-valued logic with explicit `coalesce`. Decide and document.
- **Evaluation order and purity**: filters and projections are pure; no side effects in expressions; no wall-clock functions unless the time model permits.
- **Scoping and name resolution**: which names are visible where; whether a query can reference another query's output by name; whether forward references are allowed.
- **Units**: durations typed, with literal syntax; no bare milliseconds.
- **Operational semantics sketch**: for each construct, a small-step description of what state it holds and when it emits. For stream constructs this is the Dataflow model; see the files under `references/semantics/`.
- **Error semantics**: what a runtime error in an expression does to the event (drop, route, fail the query).
