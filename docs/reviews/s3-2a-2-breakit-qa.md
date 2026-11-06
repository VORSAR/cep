# S3.2a-2 break-it: the runtime core: router, shards, sink flushers, drain barrier, Stats, Close (breaker QA)

> **Historical — S3.2a-2, the runtime core; merged in PR #11.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-05--s32a-2s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-2a-2-design.md) · [record](../sprints/s3-2a-2-record.md) · reviews: [design: architecture](s3-2a-2-design-architecture.md), **break-it: QA**. **← [S3.2a-1](../sprints/s3-2a-1-record.md) · [S3.2b-1](../sprints/s3-2b-1-record.md) →**

**Base:** 1a86e88 (`s3-2a-2/integration`). **Branch:** `s3-2a-2/breakit` (worktree `/Users/ramith/code/cep-wt/s3-2a-2-breakit`). Not pushed.
**Commits:** c8e29fb (R1, B1, C1, V1 reds; 3 guards), e070198 (V2 red; 2 guards). One file: `internal/runtime/breakit_test.go`. No production code touched.
**Test runs (pinned Go 1.27.3, GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=):**
- I ran each of the 5 reds alone with `go test -race -count=1 -run '^Name$' ./internal/runtime/`. Each exits 1, for its stated reason.
- Every `TestBkGuard*` passes under `-race`: `-count=4 -cpu 1,4,8`, with a CPU hog on every core (exit 0); then again at `-count=3` and `-count=2` (exit 0).
- The pre-commit gate passed on both commits. The hog processes were stopped and checked gone.

**Result:** 1 blocker, 1 should-fix, 3 later.

**Method:**
- I read design v2 §2–§7 and all of `runtime.go` and `ingest.go` adversarially, plus `partition.go`, the playback driver's use of `Ingest`, `table.Set.Snapshot`, and the clock's `Drain`.
- Then I attacked through the real API only (Start, Send, Tick, TableWrite, Drain, Stats, Close):
  - tiny capacities (Mailbox 1, Intake 1, SinkQueue 1) with 3–7 shards and 4 apps;
  - bursty concurrent senders, with Stats and Ticks interleaved;
  - Sends racing Drain;
  - Close during traffic and during a drain;
  - every key type, across 1 and 5 shards;
  - panic storms in OnEvent and OnTimers, with sinks that panic and return errors;
  - expired contexts;
  - Start shapes and Send values.

**Not tried (time box):**
- A sink that calls back into the runtime (a documented MUST NOT).
- `Config.TickEvery` with idle streams (`IdleAfter > 0`).
- A randomized live-versus-replay differential on the same admitted events. Ordering held where I checked it: per key and per sink, across 1 and N shards.
- Body panics in OnDiscard, OnPurge and NewState through the runtime. The partition and runtime tests already cover these.

## Findings (ranked)

| id | sev | contract broken | red test | repro (one line) |
|---|---|---|---|---|
| B1 | blocker (crash; likelihood low until S3.2b: the only producer is planner-built body code. The owner may rate it should-fix) | design §4 "The outbox", §7; owner F10 (a body's fault is contained to its key) | `TestBkEmitOnUnknownStreamCrashesProcess` | a body calls `emit(event.Event{StreamID: 7})` in an app with 2 streams: `panic: index out of range [7] with length 2` in `(*router).take` (runtime.go:768) kills the process |
| R1 | should-fix (arguably blocker as a broken contract: the owner's call) | design §2 "After an app's drain, `Send` … return `ErrDrained`" (owner F5) | `TestBkFailedRedrainReopensSend` | `Drain(ctx,0)` succeeds; `Drain(expiredCtx,0)` fails with DeadlineExceeded; then `Send(ctx,0,e)` returns nil, and `e` is silently counted in `AfterDrain` |
| C1 | later | design §6 Close step 5: "Flushers finish their current `Sink` call, then exit, dropping the rest" | `TestBkCloseDeliversAfterStop` | hold a sink with 18 items queued behind it; `Close(expired)`; release: the sink is called again after stop (7–12 extra calls over 12 tries) |
| V1 | later | design §2: `Send` checks each value against its schema (review F9); the decoder refuses this (`event.DecodeNull`) | `TestBkSendAdmitsNullInRequiredAttribute` | `Send(ctx,0,{k: Value{Kind:TString, Null:true}})` with `k` non-optional: nil, want an error |
| V2 | later | design §2: Start validates every rule's inputs (F9, F21) | `TestBkDuplicateInputDoublesEvents` | a rule with `Inputs: [0,0], KeyAttr: [0,0]`: Start accepts it, and the body releases every event twice |

## Blocker

### B1: a body's Emit on a stream outside the app kills the process

- **Where:**
  - `router.take`, `case oEvent`: `r.apps[it.app].sinks[it.ev.StreamID]` is indexed without a range check. It runs on the router goroutine, outside any `recover`.
  - The emit closure in `newShard` (runtime.go:833) appends without checking either.
- **Effect:**
  - The whole engine process dies, every app with it.
  - A related case, with no red test: an Emit on an in-range *source* stream finds no sinks, and the output is dropped silently, with no counter.
- **Why it matters:**
  - Owner F10 makes a body's panic a reset of its key. A wrong output stream is the same class of body fault, and it crashes everything instead.
- **Likelihood:**
  - Low now: only test bodies emit.
  - From S3.2b the planner generates the emits, and one wrong stream id in generated code takes the process down.
- **Fix area:**
  - Check `e.StreamID` in the emit closure, on the shard: in range and derived. Two options:
    - panic inside the body call, which partition already recovers and counts as a key reset;
    - drop the event and count it.
  - Either way the router never sees a bad id. A test then asserts the process survives and the fault is counted.

## Should-fix

### R1: a failed second Drain reopens a drained app's Send fast path

- **Where:** in `Drain`, `a.draining.Store(false)` on an enqueue error (the QA Q15 fix) also runs when an earlier Drain has already drained the app.
- **Effect:**
  - From then on `Send` for that app returns nil instead of `ErrDrained`.
  - The router refuses the event and counts it in `AfterDrain`. So it is not lost without trace, but the caller is told it was accepted.
  - `TableWrite` is unaffected: the router answers `ErrDrained` itself.
  - The same reset happens when two Drains race and the one whose context expired loses.
- **Likelihood:** medium. Retrying a Drain under a short deadline, the natural lifecycle reaction to a timeout, triggers it.
- **Fix area:** clear the flag only if this call set it:
  - `if !a.draining.Swap(true) { … on failure Store(false) }`;
  - or keep a separate "drain queued" atomic that only the router's `drained` path sets.

  A guard: Drain → failed Drain → Send must still be `ErrDrained`.

## Later

### C1: Close keeps delivering queued outputs after stop

- **Where:**
  - The flusher's `select { case it := <-q.q: …; case <-r.stopped: }` is a random choice when both are ready.
  - Shards (`select { mailbox, handoff, stopped }`) and the router behave the same way, but only the sink side is visible to the outside.
- **Effect:** after `stopped` closes, a flusher with items queued keeps calling its Sink about half the time per item, against §6's "drops the rest".
- **Not affected:** nothing runs after `Close` returns nil, because the WaitGroup holds it.
- **Fix area:** check `stopped` first in the flusher loop, with a non-blocking `select { case <-r.stopped: return; default: }` before the receive. Do the same in the shard loop.

### V1: Send admits a null in a non-optional attribute

- **What happens:** Send checks only `v.Kind`. A null in a required attribute reaches the bodies. A null key on a required key attribute becomes a `key_error`, where it should be refused.
- **Likelihood:** low. Live connectors decode through `event.Decode`, which refuses it, so this is defence in depth.
- **Related, no red test:** Send also admits NaN and ±Inf in non-key float attributes. ADR-0005 §3 calls these expression errors that can never be encoded. A NaN key is a designed `key_error` (design §4), so this one may be intended.
- **Fix area:** in Send, refuse `v.Null && !sch.Attr(i).Optional`.

### V2: a rule may list one input stream twice

- **What happens:** `validateApp` does not refuse duplicate `Inputs`. `byStream` then gets two routes for one rule, the event is offered twice under one key, and the body releases it twice: a doubled detection.
- **Likelihood:** low. It is a planner bug, refused nowhere.
- **Fix area:** in `validateApp`, refuse a stream id repeated in `rule.Inputs`.

## Observations not filed (no red test; for the coordinator)

- **A timed-out Drain cannot be waited on again.** Drain's context expires while it waits on the markers. It returns `ctx.Err()`, and the barrier completes inside the runtime. A second `Drain` then returns `ErrDrained` at once, possibly before the first barrier's outputs have been through their sinks. The caller has no way to learn when the flush finished.
  - Design §2 does not say what a second Drain returns. That is a question for the owner, not a defect.
  - My recommendation: a second Drain on an app whose barrier is in flight should wait on the same markers.
- **Start does not compare names:** it does not check that `Schemas[i].Stream()` equals `Streams[i].Name`. A misaligned schema passes, and Send's error messages then name the wrong stream.
- **A `NewBody` that panics** panics the caller of `Start`; it is not turned into an error.
- **A body that calls `runtime.Goexit`** (for example a test helper's `t.FailNow`) ends its shard goroutine without a panic. The router then blocks on that mailbox. Exotic, and not filed.

## Tried and held

- **Liveness under tiny capacities.**
  - Setup: Mailbox 1, Intake 1, SinkQueue 1; 7 shards, 4 apps × 300 events over 9 keys; concurrent Stats and Tick callers; a sink that yields.
  - Held: every Drain returned. At its return, each app's sink had seen exactly 300 outputs, per key in seq order. AfterDrain and StreamLate were 0.
  - Runs: `-race`, `-cpu 1,4,8`, CPU hog. `TestBkGuardTinyCapacityDrainFlushesEverything`.
- **Send racing Drain.**
  - 40 tries, Mailbox 1, Intake 2, SinkQueue 1.
  - Held: every Send returning nil is accounted for, as an output through the sink by Drain's return or as `AfterDrain`. Nothing was lost silently. `TestBkGuardSendRacingDrainNeverLost`.
- **Close during traffic and during a drain, with a ticker.**
  - Held: every method after Close returns `ErrClosed`, and a second Close returns nil.
  - Held: the goroutine count returns to the baseline after 20 runtimes. `TestBkGuardAfterCloseAndLeaks`.
- **Every key type across 1 and 5 shards,** plus a key-less rule. Types: string, int, float with -0.0 and +0.0, bool, timestamp, duration.
  - Held: each key value is born exactly once, and -0.0/+0.0 is one key. The per-key release records are identical for 1 and 5 shards. `TestBkGuardEveryKeyTypeOneInstance`.
- **A panic storm.** OnEvent panics on every 3rd seq, after emitting and scheduling a timer. OnTimers panics on every other timer. The sink panics or errors on some outputs. Tiny capacities.
  - Held: there was no crash, and Drain returned. Every released event's output reached the sink: 300 of 300. Panics and SinkErrors were counted. `TestBkGuardPanicStorm`.
- **Held on reading** (no separate test):
  - the deadlock argument: the router always receives handoffs while blocked, and a shard always drains a full mailbox;
  - marker ordering behind every pre-ack output (a handoff's items in order; acks after outputs in each outbox);
  - ticks skip drained apps;
  - TableWrite on a drained app is refused by the router;
  - `rt.now` clamping;
  - copies of `adv` and offers per message;
  - the handoff slice not aliased (`sh.outbox = nil` after the send);
  - the table snapshot published by the router only;
  - `Pending` excluding TTL only (Discard timers are body-scheduled, so they count).

## Commits

- c8e29fb: test(runtime): break-it reds: R1, B1, C1, V1. Guards: AfterCloseAndLeaks, TinyCapacityDrainFlushesEverything, SendRacingDrainNeverLost.
- e070198: test(runtime): break-it red: V2. Guards: EveryKeyTypeOneInstance, PanicStorm.
