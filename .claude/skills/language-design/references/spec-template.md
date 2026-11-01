# Language specification template

Use this structure for `SPEC.md`. The order follows the Go and WebAssembly specs (lexical → syntax → static semantics → dynamic semantics), which readers and models both know. Every normative statement is numbered so error messages and tests can cite it. Keep examples in canonical form and keep each example also in `examples/` so it is executed.

```
# <Language name> Specification, version <x.y>

## 0. Status and scope
- Version, date, compatibility promise, what is normative vs informative.
- One-paragraph purpose; audiences (generator, reader, tweaker); compile targets.

## 1. Notation
- EBNF conventions used in this document (ISO 14977 style); how terminals are written.
- Pointer to `grammar.ebnf` (normative) and `grammar.lark` (machine-checkable, must be equivalent).

## 2. Lexical structure
2.1 Source encoding (UTF-8), line endings, case sensitivity rule.
2.2 Comments (`--`, `/* */`).
2.3 Identifiers (charset, quoting with backticks, reserved words list — the full closed vocabulary).
2.4 Literals: integer, decimal, string (escapes), boolean, duration (`500ms 30s 5m 2h 1d`), timestamp (ISO-8601), null (if allowed).
2.5 Operators and punctuation (SQL set, verbatim).
2.6 Keywords reserved for future use.

## 3. Program structure
3.1 Version pragma.
3.2 Declarations in order: params, sources, streams, patterns, queries, sinks.
3.3 Name resolution and scoping; forward references; uniqueness rules.

## 4. Grammar
Full EBNF, one production per construct, grouped by section. Each production followed by its canonical English rendering (used by the paraphrase generator) and one example.

## 5. Types and static semantics
5.1 Type system: scalar types, `duration`, `timestamp`, stream types (nominal schemas), optional fields.
5.2 Null rule.
5.3 Typing of expressions (SQL rules; duration arithmetic; comparisons).
5.4 Mandatory-clause rules (each with its error code).
5.5 Boundedness rules (every stateful construct has a bound) with error codes.
5.6 Determinism rules (processing-time, now(), lookups).
5.7 Output-mode derivation (append / changelog / upsert) and sink compatibility.
5.8 Parameter rules.

## 6. Dynamic semantics
6.1 Data model and event ordering.
6.2 Time: clocks, watermark computation, idleness.
6.3 Windows: boundary rules per kind, alignment, per-key; worked examples (hopping offsets; session merge; late bridging).
6.4 Triggers, accumulation, lateness: the allowed-combinations table; firing rule `wm ≥ end + allowed_lateness`; late-event disposition.
6.5 Aggregations: distributive/algebraic/holistic classification; what is allowed where.
6.6 Joins.
6.7 Patterns: selection strategy, after-match policy, restart semantics, absence firing rule, `within` measured from first matched event, event-time ordering and late-event handling, match output; a CEL/timed-CEL rendering of each operator.
6.8 Expression evaluation: purity, errors (what happens to the event).
6.9 `expect` block semantics (how examples are executed and compared; tolerance rules).

## 7. Diagnostics
Pointer to `errors.md`; format of a diagnostic; guarantee that every static rule in §5 has a code.

## 8. Compile-target mapping
Per target (e.g. Siddhi, Flink SQL/CEP): a construct × feature matrix (native / emulated / unsupported), semantic caveats (e.g. Flink's greedy-tail restriction; Siddhi's `every` restart), and which `expect` blocks are target-independent.

## 9. Canonical formatting
Formatter rules; a before/after example.

## 10. Conformance
What a conforming implementation must do; how the example corpus is used as a conformance suite.

## Appendix A. Complete grammar (EBNF)
## Appendix B. Reserved words
## Appendix C. Error catalogue summary (codes and one-line descriptions)
## Appendix D. Change log
```

## Companion artefacts (same directory)

- `grammar.ebnf` — ISO-style EBNF, human-oriented.
- `grammar.lark` — Lark grammar (LALR preferred; Earley with `ambiguity="explicit"` used only for the ambiguity test). Must accept exactly the examples in `examples/` and reject exactly the counter-examples in `examples/invalid/`.
- `errors.md` — the catalogue. Entry format:

```
### E-<AREA>-<NNN> <short title>
Rule: SPEC §x.y
Message: <what is wrong, naming the construct, not the token>
Fix: <one to three concrete rewrites>
Example (invalid → valid):
```

- `examples/` — numbered, each file: `intent`, program, `expect` blocks; tagged in a front-matter comment with the constructs exercised (for retrieval during generation). `examples/invalid/` holds counter-examples with the expected error code in a comment on line 1.
- `paraphrase/` (optional but recommended) — the English rendering rules, one per production, as a table; this is what a paraphrase generator or a reviewer uses.
- `feature-matrix.md` — construct × target.

## Writing rules for the spec
- Normative sentences use "must / must not"; informative sentences are marked *Note*.
- Each rule has at least one example that satisfies it and one that violates it (and the violating one appears in `examples/invalid/` with its error code).
- Semantics are stated as rules over an abstract machine (watermark, window state, partial matches), not as "whatever engine X does".
- Where the target engine deviates, the deviation is in §8, never silently in §6.
