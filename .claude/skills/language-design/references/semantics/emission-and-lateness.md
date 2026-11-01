# Semantics: triggers, emission, accumulation, lateness and output modes

*Read when:* the language has windows or non-windowed aggregation; this is where "what fires when" is decided (Phase 3).

## 4. Triggers, emission and lateness

The Dataflow questions: *what* is computed (the query), *where* in event time (the window), **when** in processing time (the trigger), **how** refinements relate (accumulation). Most languages hide the last two; that is the hazard.

Trigger/emission policies to name explicitly:
- `on watermark` (default-looking, but must still be written): emit once when the window closes.
- `every <duration>` / `every <n> events` (early firing): emit partial results periodically before closing. Requires an accumulation mode.
- `on each event`: emit after every input (per-event sliding semantics).
- `on close with late firings up to <duration>`: emit at watermark, then re-emit when admitted late events change the result.

Accumulation modes when a window can fire more than once:
- **Discarding**: each emission contains only new contributions since the last. Consumers must sum.
- **Accumulating**: each emission is the full current result. Consumers must overwrite.
- **Accumulating and retracting**: each emission carries a retraction of the previous result plus the new one. Consumers get exactly-once-style updates. This is a changelog output (see §5).

Lateness policy per windowed query (mandatory clause):
- `drop` late events (and ideally count them in a metric).
- `allow <duration>`: keep window state open that much longer after the watermark passes the end; late events within the bound are admitted and trigger late firings.
- `route to <stream>`: side-output late events.

The combination (trigger × accumulation × lateness) determines what a consumer sees. The spec must contain a table of allowed combinations and forbid the nonsensical ones (e.g. early firing without an accumulation mode).

## 5. Output semantics

- **Append stream**: each output row is a new fact; never updated. Natural for pattern matches and single-fire windows.
- **Changelog / retract stream**: rows carry `+I` (insert), `−U/+U` (update before/after) or `−D` (delete). Needed for multi-fire windows with accumulation and for non-windowed aggregations.
- **Upsert stream**: keyed; a new row for a key replaces the old. Needs a declared primary key.

The language should make the output mode derivable from the query (a static rule: "a query that can fire more than once per window produces a changelog"), and should make sinks declare what they accept. A sink that accepts only appends fed by a changelog query is a static error.
