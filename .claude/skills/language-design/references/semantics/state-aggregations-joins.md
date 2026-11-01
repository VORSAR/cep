# Semantics: aggregations, state bounds and joins

*Read when:* the language has aggregates, group-by over unbounded streams, or joins (Phase 3).

## 6. Aggregations and state

Classify by state cost (Gray et al. 1997):
- **Distributive** (sum, count, min, max): constant state per window per key.
- **Algebraic** (avg, variance): constant-size intermediate state.
- **Holistic** (median, percentile, distinct count, mode): state proportional to input; must be bounded by a window or approximated (HLL, t-digest). The language should either exclude holistic aggregates over unbounded scopes or require an explicit approximation.

Every stateful construct must have a visible bound: a window, a `within`, a TTL, or a count cap. A construct with no bound is a static error. Non-windowed `group by` over an unbounded stream is the classic unbounded-state query; allow it only with a declared state TTL and changelog output.

## 7. Joins

- **Window join**: both sides windowed by the same window; join within the window.
- **Interval join**: `a.t between b.t − d1 and b.t + d2`; state bounded by the interval; the most useful stream–stream join.
- **Temporal / versioned join** (event-time lookup into a slowly changing table as of the event's time): deterministic; needs a versioned table with a primary key and event-time attribute.
- **Lookup join** (processing-time lookup into an external table): non-deterministic, non-replayable; mark it as such in the language or exclude it.
- Unbounded stream–stream joins: forbid without an interval or TTL.
