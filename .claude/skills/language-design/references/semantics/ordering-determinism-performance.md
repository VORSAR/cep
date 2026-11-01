# Semantics: ordering, partitioning, determinism and performance affordances

*Read when:* finalising Phase 3, and in Phase 7 when hunting unbounded-state and non-determinism paths.

## 9. Ordering, partitioning, parallelism

- Order is only meaningful per partition key. Say so. Global order is a scaling hazard; a construct that needs it (unpartitioned pattern, unpartitioned session window) should be a warning or an error.
- `partition by` is the unit of parallelism; it should be visible in every stateful construct so a reader can see the scaling model.
- Repartitioning (changing the key between queries) implies a shuffle; the language may make this implicit but the spec must say when it happens.

## 10. Determinism and replay

A program is deterministic iff it uses only event time, pure expressions, and no external lookups. Deterministic programs can be replayed from a log to reproduce outputs, tested against a reference executor, and debugged. Design rules: processing-time constructs are opt-in with a keyword that makes their non-determinism visible; wall-clock functions (`now()`) are banned from filters and projections unless the query is declared processing-time; external lookups are marked.

## 11. Performance affordances to design out

- Patterns without `within` (unbounded partial-match state).
- Kleene closures without a cap under skip-till-any-match (exponential partial matches).
- Unpartitioned stateful operators.
- Holistic aggregates over unbounded scope.
- Stream–stream joins without time bounds.
- Session windows with very large gaps on high-cardinality keys (state held for gap duration per key).
- Early-firing triggers with short periods on wide windows (output amplification).
- Regular expressions over strings in hot predicates (make it a declared function so cost is visible).
- Watermark delays large relative to window size (latency is `d + allowed_lateness`; the reader should be able to compute output latency from the text).
