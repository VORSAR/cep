# ADR-0010: Performance model — zero-alloc steady state, chunked pipeline, schema-driven decoding, measured everything

## Status
Accepted (owner, 2026-11-01 — conditions in ADR-0015/qa-strategy.md; origin: owner directive "throughput and latency are of utmost importance"). Evidence base: [research/perf-techniques.md](../research/perf-techniques.md) (43 sources; measured/claimed/folklore labeled).

## Context
Sub-millisecond hot path is O3; the owner elevated throughput+latency to a first-class driver. The literature's transferable lessons for a single-node Go engine: tail latency is dominated by allocation/GC jitter and scheduler wakeups, throughput by per-event overhead amortization; and none of it is real unless measured with open-loop load and honest percentiles (coordinated omission). Three tensions with existing rules were identified and are resolved here, not smuggled past: dependency policy vs fast-JSON (resolved: in-repo decoder), determinism vs pooling (resolved: scratch-buffers only), embeddability vs process tuning (resolved: the library never touches runtime knobs).

## Decision
1. **Allocation budget (gate-enforced):** steady-state hot path allocates **zero** per event — ingest decode, routing, clock advance, filter/pattern/window evaluation, rate limiting, canonical encode into a reused buffer. A T2 gate row runs `testing.AllocsPerRun` benchmarks per stage and fails on > 0; exceptions are impossible by policy (fix the code). Match emission MAY allocate (matches are rare relative to events); its budget is one tripwire-tracked benchmark. **Partial spawning (round-2 N8):** `every`-spawn is per-qualifying-event work and MUST NOT fail the zero-alloc gate dishonestly — the sanctioned mechanism is a **per-instance freelist** for partials and their capture slices (single-threaded per key ⇒ deterministic; release points are explicit: defeat, discard, emission — unlike cross-key event pooling, which stays rejected); the spawn benchmark asserts zero allocations once the freelist is warm, plus a tripwire on cold-path growth.
2. **Chunked pipeline (natural batching, no flush timers):** the shard loop drains its whole mailbox per wakeup and runs the batch through `Advance`/operators as one chunk (Disruptor batch effect [measured]; Siddhi chunk precedent). **Latency is never traded for batching**: no timer waits for a batch to fill. **Batch-size invariance is a conformance property**: the suite reruns golden cases with forced chunk sizes {1, 2, 7, whole-log} and asserts byte-identical outputs — chunk boundaries MUST NOT leak into semantics.
3. **Schema-driven ingest decoding (in-repo):** `internal/event/decode.go` — a single-pass JSON scanner that writes directly into `[]event.Value` by schema index: no `map[string]any`, no reflection, no intermediate strings beyond the value bytes, `int64` full-range safe. Differential-tested against `encoding/json` over a generated corpus (same accept/reject + values) and fuzzed (Go native fuzzing). sonic/simdjson-class dependencies are rejected (ADR-0009 §6; conformance failures + platform/JIT constraints per the study).
4. **Pooling: scratch buffers only.** Leveled, size-capped `sync.Pool`s for encode/decode scratch, never held across I/O or stored in state (VictoriaMetrics rules [measured]). **Event objects are NOT pooled** in Phase 1: windows and partials retain events, so no provable release point exists; a use-after-return here = nondeterministic wrong detections, the worst bug class we can create (determinism tension resolved conservatively). Revisit only with an ownership analysis in a future ADR.
5. **Identifiers:** stream/rule/attr names resolve to integer ids at deploy (already ADR-0003/0004); `unique.Handle` (stdlib, Go ≥ 1.23) interns recurring metadata strings (keys in evidence, stream names in outputs); payload values are never interned.
6. **Concurrency hygiene:** per-shard hot structs cache-line padded; no shared atomics/mutexes on the hot path (review discipline + race detector at T3); mailboxes stay Go channels behind the mailbox seam — an in-repo SPSC ring replaces a channel ONLY where a benchmark shows the channel is the measured bottleneck (study §1/§2.6: channel-vs-ring gaps are real but µs-order; prove need first).
7. **The library never tunes the process:** no GOGC/GOMEMLIMIT writes, no LockOSThread, no busy-spin — `cepd` MAY document deployment guidance, and a future opt-in "dedicated mode" (busy-poll wait strategy, pinned threads) is named as reserved, never default (embeddability tension resolved).
8. **Measurement discipline (the gate makes 1–7 real):**
   - Benchmark suite `bench/` with fixed scenario generators (kill-switch mix, text-heavy mix, high-cardinality keys); open-loop load generation (arrival schedule independent of completion — coordinated omission [Tene]); in-repo HDR-style histogram recording p50/p99/p99.9/max end-to-end (ingest→sink-queue) and per-stage.
   - CI: `benchstat` comparisons against the stored baseline, ≥10 runs, tripwire at the testing.md ≥10× rule for correctness-adjacent regressions and a 20% advisory band for throughput/latency; baselines updated only by an explicit commit (`perf: rebaseline <reason>`).
   - Published numbers (roadmap exit criterion 2: detection→kill-switch latency) come from this suite, with hardware + method stated.

## Alternatives considered
- Fast-JSON dependency (sonic/simdjson-go) — rejected (study: conformance failures, JIT/asm/platform constraints, Go-version coupling); revisit only if the in-repo decoder measures short of need.
- Event pooling now — rejected on determinism risk (above).
- Disruptor-style ring everywhere — deferred behind the mailbox seam pending measurements; semantics identical either way.

## Consequences
- Positive: performance is enforced by gates and benchmarks, not aspiration; every technique adopted has a measured citation; the three rule-tensions are decided on the record.
- Negative: the in-repo decoder and histogram are maintenance surface (bounded, heavily tested); zero-alloc discipline constrains API shapes (caller-provided buffers) and will occasionally cost agent time — the budget is the point.

## Implementation notes for coding agents
- Decoder: write the fuzz target and the differential test BEFORE optimizing; keep a `decodeSlow` (encoding/json-based) reference implementation in the test package only.
- Every benchmark uses `b.ReportAllocs()` and fixed seeds; scenario generators live in `bench/gen/` and are shared with the demo kit.
- The histogram: power-of-two bucketing, ~2% resolution, int64 ns; `Record(d)`, `Percentile(p)`, `Max()`; no locks (per-shard, merged on read).
