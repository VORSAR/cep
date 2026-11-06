# ADR-0006: Sharded runtime with per-key serial execution; all per-key state in one struct behind a snapshot seam

## Status
Accepted (owner, 2026-11-01 — ratified with the enforcement conditions implemented in ADR-0015 and docs/development/qa-strategy.md).

## Context
SEMANTICS §9: per-key isolation of ALL state (partials, completion marks, window state, rate anchors), instance lifecycle born-on-first-event / destroyed-on-TTL, purge emitting changelog retractions first. O2: unbounded key cardinality (agents, sessions) safe by default. O3: sub-millisecond hot path. Phase 2 adds snapshots — Phase 1 must not paint that in a corner (roadmap: "pattern state does NOT survive restart in MVP" is honest, but the state must be *walkable* later).

## Decision
1. The runtime is **sharded**: `shards = max(1, GOMAXPROCS)` by default (Config-overridable). A shard is one goroutine with one mailbox (bounded channel). Keys are assigned by `hash(ruleID, keyValue) % shards`. Everything for one (rule, key) happens on one shard — operator code runs single-threaded per key and holds **no locks**.
2. All per-instance state lives in ONE struct, `internal/partition.Instance` — **revised 2026-11-01 (review F1):** `partition` MUST NOT import operator packages, so operator state hangs off the instance behind an interface the OPERATOR implements and `partition` defines:
   ```go
   // internal/partition/instance.go — imports event and clock ONLY.
   type OpState interface {           // implemented by pattern.State and window.State
       Walk(fn func(component, key string, state any)) // Phase-2 snapshot seam
   }
   type Instance struct {
       Key          event.Value
       Clock        *clock.Instance      // ADR-0004 (buffer + timers live inside it)
       Op           OpState              // the rule body's state, created by plan's operator factory
       RateAnchors  map[int]int64        // ruleOutputID -> t_out
       OnceDone     bool                 // `once` completion mark
       LastActivity int64
   }
   ```
   The planner (L3) constructs `Op` via the operator's factory and hands the instance its callbacks — the layer arrows all point downward and import rule 5 holds. **Snapshot seam (widened per review F19):** Phase-1 ships (a) `OpState.Walk` on every stateful operator type, (b) a runtime-owned **state registry** in which plan-time construction registers every holder of durable state — instances (including each `clock.Instance`, which enumerates its ordered release buffer: admitted-unreleased events ARE durable state, round-2 F19), the stream registry (admission wms, Seq counters), each instance's batch-id dedup set *(annotated 2026-11-02: the dedup set lives per instance, as ADR-0014 §5 states, and is TTL-purged with it — it was listed under the stream registry here)*, and global rate anchors — and (c) the invariant, gate-tested, that **every timer is derivable from walked state** (absence deadlines from partials' t_first+D, TTL from LastActivity, cron from bucket boundaries), so a Phase-2 restore can rebuild the timer heap without serializing it. *(Annotated 2026-11-05, S3.2a-1 design v2 §3 ([decisions](../../sprints/decisions.md#owner-decisions-2026-11-05--s32a-1s-design-after-the-architecture-review)): `OpState` also has `Timers(dst)`, the timers its state implies, so `partition.Rule.CheckTimers` tests (c). Per-key marks — the `once` mark, rate anchors, the batch-id dedup set — live in the body's `OpState`, so a purge clears them and `Walk` yields them; `Instance` keeps `Key`, `Clock`, `Op`, `LastActivity`. The key arrives as a value (the router reads the key attribute); TTL, its re-arm and the purge are `partition`'s, through a `Body` interface. Holders: `partition.Rule` (its clock, then each instance), `clock.StreamRegistry`, and the ingest step's Seq counters.)* *(Annotated owner, 2026-11-06, S3.2b's plan check Q1 and S3.2b-1 D1 ([decisions](../../sprints/decisions.md#owner-decisions-2026-11-06--s32bs-plan-check)): a plain rule is `OnArrival` — one input, no release buffer, its body sees each accepted event at `Offer` — and its rule clock advances only on its stream's admission watermark, the drain and the end, so its keys' TTL never follows a floor. Its rate anchor is the body's per-key mark, as above.)*
3. **TTL purge** is a clock timer (ADR-0004): on firing, if `clock − LastActivity > ttl`, the shard (a) asks a changelog window for retraction events and emits them, (b) deletes the Instance from the shard map, (c) counts `purged_instances`. Re-arming after purge is normative (SEMANTICS §9). *(Annotated owner, 2026-11-04, S3.1-1; [decisions](../../sprints/decisions.md#owner-decisions-2026-11-04--s31-1s-clock-after-two-architecture-reviews), D1: the purge falls at its instant in the clock's walk, so later events of the key may still be buffered; it resets the key's operator state and timers (`CancelAll`) and keeps those events, which start the key fresh. The shard frees an instance only when it holds no event and no timer (`Empty`). Activity is counted at release.)*
4. **Ingest and routing** (`internal/runtime`): **exactly ONE ingest/router goroutine per runtime** (round-2 N4) — sources call a thread-safe `emit` that enqueues into the router's intake; the router alone validates, stamps Seq (plain per-stream counters — single writer, no atomics), computes Invisibles, updates the `clock.StreamRegistry`, evaluates key expressions, and writes mailboxes/sink queues (it is their only writer). Null key → per-rule event error (counted), not routed. **Key-less (unpartitioned) rules** (round-2 F14): the planner assigns a constant pseudo-key `""` so the rule runs as one instance on shard `hash(appID, ruleID) % shards`; its global rate anchor is that instance's anchor — no special path exists.
5. **Backpressure (rewritten 2026-11-01 — review F4 found the old text self-contradictory and deadlock-prone):** one rule — **only the ingest/router stage ever blocks; a shard goroutine never blocks on anything.**
   - Each shard owns an unbounded-but-accounted **outbox** (slice it appends to during `Advance`); after each `Advance` the shard offers it to the **router** with a **non-blocking try-send** on its handoff channel — on failure the shard RETAINS the outbox, merges further emissions into it, and re-offers; an otherwise-idle shard parks on `select{ mailbox receive, handoff send of the retained outbox }`, so a drained handoff wakes it (round-2 N1).
   - The router delivers outbox contents: sink-bound events to sink queues (bounded; if full, the ROUTER waits — sources feel backpressure, shards do not), derived-stream events to their sink and table bindings only — **v0.1 rules cannot consume derived streams** (owner decision 2026-11-01, ADR-0004 §6), so no derived event re-enters routing in v0.1; the Phase-2 re-ingestion path (key-extract → shard mailboxes, then a min-merged punctuation) is designed in ADR-0004 §6. **While blocked on any mailbox or queue send, the router simultaneously services all shard handoff channels** (one select over the pending send plus every handoff), so the shard→router edge can always complete (round-2 N1).
   - **Wait-discipline table (normative; the deadlock argument, complete this time):**

     | Goroutine class | May wait on | Never waits on |
     |---|---|---|
     | source reader | its own I/O; the router's intake queue | shards, sinks, other sources |
     | ingest/router (ONE per runtime — round-2 N4) | shard mailbox sends and sink-queue sends, **while servicing every shard handoff** | nothing else |
     | shard (one per shard) | `select{mailbox recv, retained-outbox handoff send}` | sinks, other shards, I/O, tables |
     | sink flusher (one per binding) | its queue recv; its own I/O | everything else |

     Every blocking edge in the table points source-ward: sources → router-intake; router → shards/sink-queues with handoffs serviced; shards park only on edges the router (its sole counterpart) always eventually completes. The stream graph is a DAG with single producers (E-GRAPH-001/003). Therefore the wait-for graph is acyclic — and the saturation test includes round-2 N1's exact counterexample (full mailbox at a shard whose handoff is pending) plus slow-sink + bursty-source + a derived stream bound to a sink, asserting progress (a derived-*chain* case — rule consuming rule — joins the test with the Phase-2 feature, ADR-0004 §6).
   - Accounting: outbox depth and per-queue depth are metrics; "unbounded" outbox is bounded in practice by one Advance's emissions — a gate tripwire fails if a single Advance emits more than the configured ceiling (default 65,536) to catch runaway rules. *(Annotated 2026-11-05, S3.2a-2 design v2 ([decisions](../../sprints/decisions.md#owner-decisions-2026-11-05--s32a-2s-design-after-the-architecture-review); review F13): every shard hands off through **one shared channel**, of capacity = shards, which the router receives on in every wait. The depths are `runtime.Stats`' high-water marks (the router's pending FIFO, each mailbox) and `SaturatedParks`, the times a shard parked with its mailbox full and a handoff pending. The per-Advance emit ceiling's tripwire is S3.2b's item 7.)*
6. **Determinism:** playback runs with `shards = 1` and a no-op ingest interleaving (merged-log order), making output byte-identical (ADR-0005). Live sharding affects only interleaving across keys — per-key order is identical in both modes because routing and per-key serialization are identical.

## Alternatives considered
- **Goroutine per key** — rejected: unbounded key cardinality (the product's core assumption) = unbounded goroutines; shards bound concurrency independently of keys.
- **Lock-based shared state** — rejected: per-key serial execution removes whole bug classes agent-written code is prone to; the no-locks rule is lintable.
- **Deterministic live mode (global ordering)** — rejected: global order is a scaling hazard (SEMANTICS §2.2 — order is only meaningful per key); the live/playback contract (SEMANTICS §2.1) already scopes the difference honestly.

## Consequences
- Positive: operator authors (agents) write single-threaded code against one struct; the race detector (T3) guards the seams only; snapshots have a prepared seam.
- Negative: hot keys serialize on one shard — accepted for MVP (single-node, vertical scale); shard-count config is the pressure valve.

## Implementation notes for coding agents
- The shard map is `map[instanceKeyString]*partition.Instance` private to the shard goroutine; instanceKey = appID + "\x00" + ruleID + "\x00" + canonical key encoding (AppID-scoped — two apps may share rule ids; round-2 N5.6). Canonical key encoding = the ADR-0003 canonical JSON of the key value.
- `LastActivity` updates on every routed event (any input stream — SEMANTICS §9).
- The TTL static check (TTL ≥ bounds/extents/rate intervals, E-TTL-002) is the validator's job (deploy time), NOT the runtime's; the runtime trusts the model.
- Metrics counters (`internal/metrics`) are per-shard, summed on scrape — no atomic contention on the hot path; per-shard hot structs are cache-line padded (perf study §2.6, measured false-sharing effect in Go).
- Conformance case "one TTL timer per instance" guards the verified Siddhi purge defect class (siddhi-core study §7): timers scheduled at birth, re-armed on activity, never one-per-event.
