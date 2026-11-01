# Semantics: clocks and watermarks

*Read when:* any streaming language; the source declaration and every time-dependent construct depend on this (Phase 3, first).

## 1. Time

Three clocks exist; the program must say which one each construct uses.
- **Event time**: the timestamp carried by the event, assigned at the source. The only clock under which results are deterministic and replayable.
- **Processing time**: the wall clock of the machine evaluating the operator. Non-deterministic, non-replayable, but needed for "alert if nothing arrives for 5 minutes of real time" and for latency-bounded output.
- **Ingestion time**: assigned on entry to the system. Rarely the right choice; treat as a degenerate event time.

Design rule: a source declaration names its event-time attribute (or declares itself processing-time) explicitly. There is no default. Every time-dependent construct inherits the clock of its input stream; mixing clocks in one query is a static error unless a construct explicitly converts.

Timestamps need a type (instant with time zone semantics decided: store UTC, display as given) and durations need a type with literals (`500ms`, `30s`, `5m`, `2h`, `1d`). Calendar units (months) are a trap; exclude or define exactly.

## 2. Watermarks

A watermark is the system's assertion "no event with timestamp < W will arrive again." It drives when windows close and when patterns time out. (Akidau et al., "Watermarks in Stream Processing Systems", VLDB 2021, is the precise comparative reference; "The Dataflow Model", VLDB 2015, is the origin.)

Strategies the language should name:
- **Bounded out-of-orderness**: `W = max(event_time seen) − d`. The common case; `d` must be stated.
- **Monotonic / ascending**: `d = 0`; only for sources that guarantee order.
- **Per-partition with min across partitions**: the watermark of a keyed/partitioned source is the minimum over partitions; one stalled partition stalls everything.
- **Idleness**: a source that stops emitting must be marked idle after a timeout or it holds back the global watermark forever. The language must expose this (`idle after 1m`).
- **Punctuated / source-provided**: the source emits watermark events itself.

What the reference semantics do (StreamReason-Bench's executor, matching Dataflow and Flink): after each arrival, `wm = max(event_time) − d`; a window fires and closes once `wm ≥ window_end + allowed_lateness`; an event whose every containing window is already closed is late and is dropped (or routed, if the program says so); at end of stream all open windows fire.
