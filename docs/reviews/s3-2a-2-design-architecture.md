# S3.2a-2 design note v1 — architecture review

> **Historical — S3.2a-2, the runtime core; merged in PR #11.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-05--s32a-2s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-2a-2-design.md) · [record](../sprints/s3-2a-2-record.md) · reviews: **design: architecture**, [break-it: QA](s3-2a-2-breakit-qa.md). **← [S3.2a-1](../sprints/s3-2a-1-record.md) · [S3.2b-1](../sprints/s3-2b-1-record.md) →**

Artifact: `docs/sprints/s3-2a-2-design.md` (v1), branch `s3-2a-2/integration` at ab15940. Blind review.
Sources read: the note; the S3.2a card, its "how it runs" note and "Forwarded to S3.2a-2" (`docs/sprints/sprint-plan.md`
l. 276–312); sprint-plan §5 "S3.1-1's clock" (D1–D8), "S3.2a-1's design" (D1–D7), "S3.2a-2's plan check"; ADR-0006 §1–§6
and its implementation notes; ADR-0012 §2, §3, §5, §6 and its implementation notes; ADR-0010 §1, §2, §4; SEMANTICS §2.1,
§2.2, §4, §9, §10; `internal/partition/partition.go`; `internal/clock/{clock,rule}.go`; `internal/table/table.go` (Set,
Snapshot); `internal/playback/driver.go`, `reader.go` (the stop and tick checks); `internal/event/event.go`, `decode.go`
(signatures); the stub `internal/runtime/{runtime,ingest}.go`; `tools/importcheck/rules.json`; `tools/archcheck/main.go`
(header, the clock and purity checks). Not read in full: ADR-0004 §3 (relied on through the note's and ADR-0006's
citations — marked "inferred" where it matters).

Scratch checks (pinned Go 1.27.3, `GOTOOLCHAIN=local CGO_ENABLED=0`), all under
`/private/tmp/claude-501/-Users-ramith-code-cep/2f0a267c-d842-4a04-b185-2e62e5703469/scratchpad/s3-2a-2/arch-scratch/`:
- `panic_scratch_test.go`, run in `internal/partition` through `go test -overlay` (the repository untouched): F1, F10.
- `tickrace/main.go`: F2 (a tick lands behind the drain request in 10 064 of 20 000 runs).
- `numg/main.go`: F17 (`runtime.NumGoroutine` above the baseline right after `wg.Wait` in 15 of 2 000 runs).

## Summary

| ID | Severity | Where | Title |
|---|---|---|---|
| F1 | blocker | §4 (the outbox, panics), §5 step 3, §8 Panics | A recovered panic abandons the rest of that rule's walk: other keys' due work is deferred to another snapshot, or lost at the drain |
| F2 | blocker | §5 step 1, §3 (ticker row) | "Drain stops the ticker first" races (a tick lands behind the drain) and is global: a Stop can fabricate an absence, and every other app loses its idle ticks for good |
| F3 | should-fix | §3, §5 steps 4–5 | The flush marker's signal is outside the normative wait table; the obvious flusher→router send deadlocks; the completion condition is over-specified |
| F4 | should-fix | §2 (Send's contract, Emit, Sink) | No ownership rule for `e.Values`: a reusing decoder (`Decode(b, dst)`), a body reusing its buffer or a mutating sink is a data race and a wrong key |
| F5 | should-fix | §5 ("Drain does not stop the runtime"), §8 Drain | Work after a Drain is unrecordable (the reader faults on "a record after the stop record") and silently construct-late — owner decision |
| F6 | should-fix | §4 step 1, §4 (a tick) | Tick times are not clamped to the arrival stamps: a recording of what the router applied faults ("a tick before the previous arrival or tick") |
| F7 | should-fix | §2, §6 Close | Shutdown contract gaps: closing the intake under concurrent `Send` panics; callers and the ticker do not wait on a stop signal; `Tick` has no ctx; Close's deadline path leaks every goroutine; "drops the rest" contradicts "finishes its FIFO" |
| F8 | should-fix | §2 Sink, §3 flusher row | A panicking `Sink` crashes the process, or (recovered without care) stalls every app and hangs `Drain` |
| F9 | should-fix | §2 Send's contract | `Send`'s preconditions are unchecked: a bad app id, stream id, derived stream or short `Values` panics the router and kills every app |
| F10 | should-fix | §4 (panic paragraph) | "The rule's clock stays consistent" — but partition's invariants do not: a panicking `OnPurge` leaves a born key with no TTL timer, never purged |
| F11 | should-fix | §5 step 3 | `drained_pending` counts every live key's TTL timer, and partition has no API to count pending timers |
| F12 | should-fix | §6 Stats | `Stats` is unavailable exactly when a sink is stuck, and `sink_errors` (flusher-owned) has no defined read path |
| F13 | should-fix | §3, §6; ADR-0006 §5 | ADR-0006 §5's accounting (outbox and queue depths) and its per-Advance emit ceiling are dropped without an annotation |
| F14 | should-fix | §0.1, §4 step 2; SEMANTICS §4 | The snapshot is taken at the router's dequeue, not at `Send`; table writes have no intake position — owner decision |
| F15 | should-fix | §8 `TestLiveOrderPerKeyEqualsReplay` | As written it compares a fixture (virtual floors, +∞ end) with live (no virtual floor, drain at max+1): red or vacuous |
| F16 | should-fix | §8 `TestSaturationProgress` | The exit-3 test can pass without ever reaching "a full mailbox at a shard whose handoff is pending" |
| F17 | should-fix | §8 `TestCloseStopsEverything` | `runtime.NumGoroutine` right after `Close` is flaky (verified); it must poll to a deadline |
| F18 | later | §0.1, §4 (walk per message), R2; ADR-0010 §2 | Each watermark-moving event costs `Shards` messages and `Shards × rules` walks; per-chunk walking or advance coalescing must cut at snapshot changes (D5) |
| F19 | later | §3 | One router and one FIFO for all apps: one app's slow sink (and S5's retry/backoff inside a flusher) stalls every app |
| F20 | later | §7 ("No `time.Sleep`"), §1 | Neither "no `time.Sleep`" nor `Ingest`'s purity is gate-enforced in `internal/runtime` |
| F21 | later | §2 Start, §4 (a tick), §5 | Start's validation list, empty-advance tick messages, and the drain of an app with no rules or no sinks are unspecified |
| F22 | later | §2 Sink | A `Sink` that calls back into the `Runtime` (`Send`, `Drain`, `Close`) deadlocks; say MUST NOT |

Counts: 2 blockers, 15 should-fix, 5 later.

## Answers to §9

- **R1 (one shared handoff channel, capacity `Shards`).** Keep it. One channel gives the router a fixed three-case `select`
  with no `reflect.Select`; one shard's sequential sends are received in order, so per-shard (and so per-key) output order
  holds; a shard can fill several slots, which is harmless because the router's every wait includes the receive. The bound
  argument is sound **in messages**: the router reads the intake only with an empty FIFO, so at any time the FIFO holds at
  most one intake item's mailbox sends (≤ `Shards`) plus the outputs of at most `Shards × (Mailbox + 1)` messages (the `+1`
  is the message each shard holds). It is **not** a bound in bytes: one message's outputs are bounded only by what its walk
  releases, and a drain or a tick after a long idle releases a whole shard's buffered state. ADR-0006 §5's per-Advance ceiling
  is what bounds that, and the note drops it (F13). Say the bound that way.
- **R2 (walk after every message, against per chunk).** Keep per message for this sprint. D1 makes per-chunk walking equal
  for the clock, but D5 makes the snapshot part of the result: a chunk may be walked once only if every message in it carries
  the same snapshot pointer (`table.Set.Snapshot` returns the same pointer while nothing was published, so this is cheap to
  test). ADR-0010 §2's D1 annotation says "the same result" without that caveat (F18). The real cost is the fan-out, not the
  walk (F18).
- **R3 (the drain ack through the outbox, the marker through the sink queues).** Yes, every pre-barrier output reaches its
  sink before `Drain` returns, under five conditions the note must state as MUSTs: (1) the marker is appended to the
  **pending FIFO** behind the outputs already routed, never sent directly to a sink queue (§5 step 4 says "to each of the app's
  sink queues", which reads as a direct send); (2) the router handles one handoff's items in order (outputs before the ack);
  (3) every shard's drain walk runs to completion even when a body panics (F1); (4) the marker's signal never blocks a flusher
  and needs no router wait (F3); (5) no tick is applied to the app after the drain request (F2). A panicking sink (F8) is a
  sixth way to break it.
- **R4 (consistent stats through the mailboxes).** Right for the shard counters: a cut at one intake position, no atomics on
  the hot path (ADR-0006 implementation notes). "Plain counters read after a barrier" would need a barrier that does not move
  the watermark, and `Drain` does (F5). Two gaps: `Stats` waits behind backpressure, so it is unavailable under a stuck sink,
  and the flushers' `sink_errors` are not reachable through a mailbox (F12).
- **R5 (allocations).** Nothing in the router → shard → sink path is structurally unfixable: a live event has at most one
  advance (`virtual = false` returns only its own stream's), so a message can carry one inline advance plus an overflow slice;
  offers can be an inline small array; one advance copy can be shared by all `Shards` messages (immutable); handed-off outboxes
  can be recycled through the next mailbox message; the FIFO is a ring. The one structural per-event allocation is the
  event's own `Values`, retained by reference in release buffers: ADR-0010 §4 forbids pooling events, ADR-0010 §1 lists
  "ingest decode" as zero-allocation, and `Send` must own a fresh slice (F4). That tension is ADR-0010's to settle, not this
  note's; `allocgate`'s ingest benchmark should name the `Values` slice as the allowed allocation.

## Findings

### F1 — blocker — A recovered panic abandons the rest of that rule's walk

**Where:** §4 ("A panic in a rule's `Offer` or `Advance` … the shard carries on with the next rule and message"), §5 step 3,
§8 `TestPanicIsAPerRuleError`.

**Problem.** `partition.Rule.Advance` runs `clock.RuleClock.Advance`, which walks every key of the rule in one loop
(`internal/clock/rule.go` `Advance`, `fireGroup`). A panic in one key's callback unwinds the whole walk. "Carries on with the
next rule and message" means every other key of that rule whose work was due in this walk is not processed until the rule's
next `Advance` — on the next message of that app that reaches this shard, which carries a different snapshot (D5), or never
before `Drain` returns. ADR-0012 §6: a panic "never skips other consumers' deliveries". Verified: in
`panic_scratch_test.go` (`TestScratchPanicMidWalkSkipsOtherKeys`), keys A and B both at ts 10, clock 11, the body panics on A:
after the first `Advance` B is not released; a second `Advance` with the same clock releases it.

**Failing scenario (missed detection at Stop).**
1. Router: drain request for app a → `Ingest.Drain()` = max + 1 → drain message to shard 0.
2. Shard 0: applies the advance; `Advance` of rule R starts; key A's absence timer fires; A's body panics; recovered.
3. Key B's completion due at max − 5 (same rule, later in walk order) is never reached in this walk.
4. Shard 0 counts `drained_pending` (B's timer included), appends the ack, hands off.
5. `Drain` returns; S3.3's `Stop` detaches. B's alert is never emitted. Without a drain, B's alert fires one message later
   reading another snapshot: its result depends on another key's panic.

**Fix.** After a recovered panic in a rule's `Advance`, the shard calls that rule's `Advance` again with the same snapshot,
until it returns normally. It always terminates: the walk removes each item before its callback (verified in
`rule.go`: `popEvent` / `removeTimer` precede `OnEvent` / `OnTimers`), so every retry makes progress. The same for `Offer`
(no retry needed: one key). Make it a MUST in §4 and §7. Extend `TestPanicIsAPerRuleError`: a second key **of the same rule**
due at the same instant is still delivered in the same message, and a drain variant shows its output before `Drain` returns.

### F2 — blocker — "Drain stops the ticker first" races, and is global

**Where:** §5 step 1 ("It stops the internal ticker first (owner D3 of S3.1-1 …)"), §3 (the ticker row), §2 (`TickEvery`).

**Problem.** Three defects in one line.
1. **The race.** Signalling the ticker goroutine to stop and then enqueuing the drain does not order them: a ticker blocked
   in (or entering) `select { intake <- tick; <-stop }` with both cases ready picks one at random. Verified: `tickrace/main.go`
   puts a tick behind the drain request in 10 064 of 20 000 runs. The router then applies that tick after the drain's
   max + 1: a quiet stream's floor rises past the max event time, and a deadline after it fires. ADR-0012 §3 (D3 of S3.1-1):
   "at Stop the ticks stop BEFORE the sources, so a shutdown never fabricates an absence."
2. **Global.** The ticker serves every app (§4: "`adv := a.ingest.Tick(now)` for every app"). `Drain(a)` stopping it stops
   idle advancement for apps b, c…: their quiet-stream deadlines (D3, D6: fail-closed, "the alert fires on time") stop firing.
3. **Never restarted.** §5 says `Drain` does not stop the runtime, yet the ticker stays stopped; and a `Drain` whose ctx
   expires before the request is enqueued has already stopped it.

**Failing scenario.**
1. App a: stream Q quiet for 10 min (`idle after 1m`, `d = 0`); a pattern awaits an absence on Q due at max + 30 s.
2. Caller: `Drain(a)` → `close(tickerStop)`.
3. Ticker: had just read `ticker.C`; enters `select`; both cases ready; sends the tick.
4. Caller: enqueues the drain behind it — or (the race's other half) ahead of it.
5. Router: drain (max + 1) → drain messages; FIFO empties; reads the tick → `ObserveTick` raises Q's floor to about
   max + 10 min → advance messages → the absence fires → its alert goes to the sink. The same input, a different outcome per
   scheduling.

**Fix.** Gate ticks in the router, by intake order, not by stopping a goroutine: when the router dequeues app a's drain
request it marks a "drained"; from then on it skips `a.ingest.Tick` (for the internal ticker and for `Tick()`). The ticker
stays global and running; `Close` alone stops it. This also settles defect 3, and fits F5's options (a "drained" app either
refuses later sends or clears the mark on its next `Send`). Test: a drain with a tick enqueued after it (call `Tick` right
after the drain request is in the intake) fires nothing past max; a second app's quiet-stream timer still fires after the
first app's drain.

### F3 — should-fix — The flush marker's signal is outside the wait table

**Where:** §3 (the table: the router "may wait on" the intake, the FIFO head with the handoff, the ticker), §5 steps 4–5.

**Problem.** "Each flusher signals its marker once it reaches it", and "the round is complete" when every marker is
signalled — but the normative table gives the router no edge to receive such a signal, and gives the flusher no edge to send
one. The obvious build (a flusher sends on a channel the router reads) deadlocks.

**Failing scenario.**
1. Router: FIFO head = an output for sink queue Q (full); waits on `Q <- out` and the handoff.
2. Flusher of Q: reaches the marker; sends `markerDone <- token` — the router is not receiving it.
3. Flusher blocks → Q never drains → the router never completes its send → every app stalls; `Drain` hangs to its deadline.

Also, step 5's "the app's mailbox work and pending FIFO are empty" is unnecessary: the acks prove the app's pre-barrier
mailbox work is done, and the markers prove the outputs reached the sinks. The FIFO is global, so the condition couples one
app's drain to another app's traffic.

**Fix.** A marker carries a `chan struct{}` the flusher closes on reaching it (`close` never blocks and is never lost). When
the last ack arrives, the router appends the markers to the FIFO and puts their channels and the summed `Pending` into the
caller's reply (a buffered channel of 1: a non-blocking send). The **`Drain` caller**, not the router, waits on every marker
channel with ctx and the runtime's stop signal. No router edge is added. Drop the FIFO-empty condition; keep the fixpoint loop
in the router (a round that re-ingests — never in v0.1 — sends another drain round before the markers). An app with no sink
binding completes at its last ack.

### F4 — should-fix — No ownership rule for `e.Values`

**Where:** §2 (`Send`'s contract, `Emit`, `Sink`, `SinkBinding`).

**Problem.** An event's `Values` slice is read by the router (keys), retained by shards in release buffers, handed to bodies,
then to every sink binding of its stream on separate flusher goroutines. The note never says who owns it. `event.Decoder`'s
signature is `Decode(b []byte, dst []Value) ([]Value, …)`: a zero-allocation source loop (ADR-0010 §1, "ingest decode")
passes the same `dst` every time.

**Failing scenario.**
1. Source goroutine: `vals, _, _ = dec.Decode(line1, vals)`; `Send(ctx, a, Event{Values: vals, …})` returns.
2. Shard 2: holds that event in key "u1"'s release buffer.
3. Source: `Decode(line2, vals)` overwrites the same backing array with "u7"'s values.
4. Shard 2 releases the event: the body sees u7's attributes under key u1. A data race under `-race`; a wrong detection
   without it.
The same with a body that reuses its output buffer after `Emit`, or a sink that mutates `e` while a second binding of the
same stream reads it.

**Fix.** §2 MUSTs: `Send` takes ownership of `e.Values` — the caller MUST NOT write it after the call (decode into a fresh
slice per event); an emitted event belongs to the runtime from `Emit` on; a `Sink` MUST NOT modify `e` (it is shared across
bindings). A `-race` test: two bindings on one stream, a sink that only reads. The allocation consequence is R5's.

### F5 — should-fix — Work after a Drain is unrecordable and silently construct-late (owner decision)

**Where:** §5 ("`Drain` does not stop the runtime: later `Send`s of the app are processed normally"), §8 `TestDrainBarrier`
("`Send` after `Drain` still works").

**Problem.** `StreamRegistry.Drain` raises every contribution permanently to max + 1 (`clock.go`: "every quantity only
rises"). After it, an event at or below max on any stream is construct-late for every rule, though its stream admits it
(e.g. `d = 5 s`, max = 100 s, a later event at 98 s). And the recording format the owner chose (S3.2a-1 D3: "a final stop
record") cannot hold it: `reader.go` faults on "a record after the stop record". A session that drains and continues cannot
be recorded or replayed, so live and replay can differ by more than counted drops.

**Owner decision — options, most accurate first:**
- **A (recommended): a drain is terminal for the app.** After its drain request, `Send` of that app returns an error; S3.3's
  `Stop` = stop sources → `Drain` → detach is unchanged, the format holds, F2's gate is a plain flag. Tests use `Stats` (an
  intake barrier that moves no watermark) where they need a mid-run barrier.
- **B: keep "Send after Drain works"**, document that it is construct-late at or below the drained instant, and give the
  format a mid-log drain record (a format change, ground rule 3).
- C: leave it unstated — not recommended.

### F6 — should-fix — Tick times are not clamped to the arrival stamps

**Where:** §4 step 1 (`arrival = max(lastArrival, Now())` at dequeue), §4 (a tick: `Tick(now)`), §2 (`Tick(now int64)`).

**Problem.** An event's arrival is stamped when the router dequeues it; a tick's `now` is stamped when it is enqueued (by the
ticker or the caller). A tick queued behind events therefore carries an older time than their arrivals. Harmless to
`ObserveTick` (a negative elapsed time moves nothing), but S3.2b's recorder writes what the router applied, and
`reader.go`'s `tickRecord` faults on "a tick before the previous arrival or tick".

**Failing scenario.** Intake: e1, tick(now = 1000). Router at wall 1005: e1's arrival = 1005; then tick 1000 → the recording
holds `arrival 1005`, then `tick 1000` → the replay faults at that line.

**Fix.** One monotone wall stamp in the router across arrivals and ticks: a tick's time is `max(stamp, now)`, and it
advances the stamp. State it in §4, and add it to `TestIngestMatchesDriver`'s comparison.

### F7 — should-fix — Shutdown contract gaps

**Where:** §2 (`Send`, `Tick`, `Drain`, `Stats`, `Close`), §3 (callers' row), §6 Close.

**Problem.**
1. "Stops … the intake": if that means closing the intake channel, a concurrent `Send` panics ("send on closed channel").
2. A `Send` blocked on a full intake, a `Drain`/`Stats` caller waiting on its reply, and the ticker blocked on the intake all
   need the runtime's stop signal in their `select`; the callers' row lists only "the intake; its own reply channel".
   Requests left in the intake are dropped by Close, so their callers would wait for their ctx.
3. `Tick(now int64) error` has no ctx: it blocks on the intake with no deadline, against §7's "every wait … with a deadline
   or a stop signal".
4. Close's deadline path is unspecified: with a sink stuck in its call, "the router finishes its pending FIFO" never ends, so
   the router, every shard (mailboxes never closed) and every flusher leak.
5. §2 says Close "drops the rest"; §6 says the router finishes its FIFO and flushers drain their queues. And ADR-0012 §3:
   "`Runtime.Close(ctx)` = Stop all apps in reverse deploy order, then stop shards" — this Close does not drain.
6. A second `Close`, or `Close` concurrent with `Drain`, is unspecified.

**Failing scenario (1).** Caller A: `Send` → `select { intake <- m … }` ready. Caller B: `Close` → `close(intake)`. A's send
executes after the close → panic in A, the process dies.

**Fix.** Never close the intake: one `done` channel closed once (`sync.Once`); every caller wait, the ticker and the router
select on it; `Tick(ctx, now)`. Close: (a) mark closed, (b) the router drops the intake's remaining items and replies to their
callers with an error, (c) delivers what is already in the FIFO, (d) closes mailboxes; shards hand off and exit; the router
closes sink queues; (e) on ctx's deadline the router abandons the FIFO, closes mailboxes and sink queues anyway, so only a
goroutine inside a stuck `Sink` call can remain — documented. State that the facade's `Runtime.Close` (S3.3) drains each app
first per ADR-0012 §3. Tests: concurrent `Send` and `Close` under `-race`; `Close` with a sink stuck forever returns ctx's
error and leaves only that flusher.

### F8 — should-fix — A panicking `Sink`

**Where:** §2 (`Sink`: "An error is counted"), §3 (flusher row).

**Problem.** A `Sink` is user code from S3.3 on (the `cepio.Sink` adapter). A panic in it, unrecovered, kills the process.
Recovered by `defer` at the flusher's top level, the flusher exits: its queue never drains, the router blocks on that queue
forever (every app stalls), and the markers behind it are never signalled (`Drain` hangs).

**Fix.** The flusher recovers **each call**, counts a panic as `sink_errors`, and carries on; markers are signalled whatever
the sink did. ADR-0012 §6's isolation, applied at delivery. Test: a sink that panics on the first event; the next events and
the drain marker still arrive.

### F9 — should-fix — `Send`'s preconditions are unchecked

**Where:** §2 ("`e.StreamID` is a source stream of the app, and `e.Values` is aligned to its schema").

**Problem.** Stated as preconditions, checked nowhere. An out-of-range app id or stream id indexes past
`StreamRegistry.streams` (`clock.go` `ObserveEvent`: `&r.streams[sid]`) on the router goroutine, which has no `recover`:
the process dies with every app. A derived stream id would move a derived stream's watermark from an event (ADR-0004 §6:
punctuations only — inferred from `clock.go`'s `Derived` comment). A short `Values` panics the key read.

**Fix.** `Send` checks, on the caller's goroutine, in O(1): the app id, the stream id, not derived, `len(Values)` equals the
schema's. It returns an error. A test per case.

### F10 — should-fix — A panic leaves partition's invariants broken

**Where:** §4 ("The rule's clock stays consistent, because the walk removed the item before its callback").

**Problem.** True for the clock, not for `partition.Rule`. `purge` calls `body.OnPurge(in)` before `CancelAll`, `Op = nil`
and the counter (`partition.go` `purge`). If `OnPurge` panics, the TTL timer is already popped and the rest never runs: the
key stays born, with no TTL timer, and is never purged. Verified (`TestScratchPanicInOnPurge`): `CheckTimers` reports
"pending timers [], but its state implies [{110 5 0}]", and after the clock reaches 2^40 the key is still resident, purged 0.
One leaked key per such panic: unbounded growth under a buggy body, and a broken timer-derivability invariant (ADR-0006 §2(c)).
Likewise a panic in `OnEvent` after a partial state change leaves `OpState.Timers` out of step with the scheduled timers.

**Fix.** In `partition` (pure Go; `defer` is no purity breach): `purge` completes `CancelAll`, `Op = nil`, `LastActivity` and
`Purged++` in a `defer`, so a panicking `OnPurge` still purges. A test in `internal/partition` (a file owned by the closed
S3.2a-1: the build touches it with a test first).
**Owner question (recommendation first):** after a body panic on a key, **reset the key** (as a purge does: its state and
timers go, its buffered events stay) so the invariant holds — recommended; or leave the key's state as the panic left it
(ADR-0012 §6 read literally). The runtime cannot tell which key panicked without help from `partition`, so the reset, if
chosen, lives there.

### F11 — should-fix — `drained_pending` counts TTL timers, and has no API

**Where:** §5 step 3 ("counts the app's pending timers as `drained_pending`").

**Problem.** Every born key holds one TTL timer (`partition.go` `onEvent` schedules it on each release), due after the drain
level for any recently active key. Counting "pending timers" makes `Pending` ≈ the number of live keys plus the real deadlines.
ADR-0012 §3 defines it as the deadlines "left unfired, honestly dropped" — detections that will not happen. And `partition`
exposes no count: the build would walk `Instances` and allocate `Clock.Timers(nil)` per key, or add an API to a closed
sprint's package.

**Fix.** Define it: the pending inclusive timers (`Completion`, `CronClose`, `DelayDue`, `BatchClose`); not `TTLPurge` or
`Discard` (strict kinds, which drop no output). Name the counting function (a `partition.Rule.PendingDeadlines()` test-first,
or the runtime's walk over `Instances` — cold path, allocation acceptable). `TestDrainBarrier` asserts the exact number with
several live keys.

### F12 — should-fix — Stats under a stuck sink; `sink_errors` has no read path

**Where:** §6 Stats.

**Problem.** `Stats` is an intake item, and the router reads the intake only with an empty FIFO. With a sink stuck, the FIFO
head never completes, so `Stats` times out exactly when an operator needs `sink_errors` and the FIFO high-water mark.
Separately, `sink_errors` is counted by flushers, which have no mailbox; the note does not say how the cut reads them.

**Fix.** State that `Stats` is a cut that waits behind backpressure. Give `sink_errors` (and per-queue depth, F13) a
flusher-owned atomic counter read directly by `Stats` — the error path, not the hot path, so ADR-0006's "no atomics on the
hot path" holds — and say it is outside the cut. A later, non-blocking health read (S5) can reuse it.

### F13 — should-fix — ADR-0006 §5's accounting and emit ceiling are dropped

**Where:** §3, §6; ADR-0006 §5 ("Accounting: outbox depth and per-queue depth are metrics; … a gate tripwire fails if a
single Advance emits more than the configured ceiling (default 65,536)").

**Problem.** The note's counters omit outbox depth and sink-queue depth, and nothing enforces the per-Advance ceiling — the
only bound on the "unbounded-but-accounted" outbox (R1).

**Fix.** Either build them (outbox high-water per shard, queue high-water per binding, and a per-Advance emission counter
checked against a `Config` ceiling, its overflow counted and tested), or forward them by name to S3.2b / S5 with an annotation
on ADR-0006 §5. Recommended: build the high-waters and the counted ceiling now (small), since exit 3's test needs them (F16).

### F14 — should-fix — The snapshot is taken at dequeue; table writes have no intake position (owner decision)

**Where:** §0.1, §4 step 2 (`Ingest.Event` sets `Tables = Snapshot()` on the router); SEMANTICS §4 as annotated ("the table
state at the event's ingest position — intake order live … a change made after the event arrived never decides it").

**Problem.** Table writes go straight to `table.Table` (mutex, atomic pointer); they never enter the intake. The router takes
an event's snapshot when it dequeues the event, which under backpressure can be seconds after `Send` returned.

**Failing scenario.**
1. Embedder goroutine: `Send(e)` (user u1 not yet blocked) returns — e is in the intake behind 1 000 others.
2. Same goroutine: `table.Upsert(blocked u1)`.
3. Router dequeues e later: `Snapshot()` already includes u1 → e is decided by a change made after it arrived.
The D5 test (`TestTimerTablesIndependentOfShards`) is flaky for the same reason unless it orders writes against sends.
Also (inferred): S3.2b's recorder must write table records at their position among events, but the router never sees the
writes, only new snapshot pointers.

**Owner decision — options, most accurate first:**
- **A: table writes enter the intake** (a write is applied by the router, or published through it), so one order holds for
  events and writes, and the recorder sees each write in place. Most accurate; a change to S3.1-2's table write path.
- **B (recommended for this sprint):** define the live ingest position as the router's dequeue, annotate SEMANTICS §4 so,
  document that a caller orders a write after an event with a `Stats` call (an intake barrier), and forward A's recording
  question to S3.2b.
- C: snapshot in `Send` — not recommended: `Set.Snapshot` is single-goroutine (its cache), and per-caller snapshots break
  intake order anyway.
In every case the D5 test separates writes and sends with `Stats`.

### F15 — should-fix — `TestLiveOrderPerKeyEqualsReplay` compares two different clocks

**Where:** §8 ("A log replayed through `playback` … and sent live … Live ticks are off; the drain stands in for the end").

**Problem.** A fixture replay raises every source stream's virtual floor to `vnow − d` after each event and ends at +∞
(`driver.go` `event`, `End`). Live has no virtual floor and ends at the drain's max + 1. A multi-stream rule's clock moves
earlier in the fixture (different construct-late drops), and every deadline after max fires in replay but not live. Either
the test is red, or it uses only cases where the difference cannot show — vacuous.

**Fix.** Replay a **recording** (header, arrivals, recorded ticks, a final stop record): the driver is then non-virtual and
ends with `reg.Drain` (`driver.go` `RecStop`). Live: `Now` returns the recorded arrivals, `Tick` replays the recorded ticks
at their positions, and `Drain` stands for the stop record. Include a multi-stream rule, a deadline after max (unfired in
both), and keys spread over shards.

### F16 — should-fix — The saturation test can pass without its scenario

**Where:** §8 `TestSaturationProgress`; exit 3 ("progress with a full mailbox at a shard whose handoff is pending").

**Problem.** "A shard's mailbox is full while its handoff is pending" is an internal state; a run where scheduling never
reaches it still passes ("every output still arrives"). 50/50 passes then prove nothing about the counterexample (ADR-0006
§5, round-2 N1).

**Fix.** Make the state observable: the shard counts each park whose `select` has the handoff case enabled while
`len(mailbox) == cap(mailbox)`; expose it through `Stats` (or an `export_test.go` hook). The test asserts the count > 0 and
`MailboxHigh == Mailbox`, plus a sink-queue high-water at capacity (F13), alongside the progress assertions.

### F17 — should-fix — `NumGoroutine` right after `Close` is flaky

**Where:** §8 `TestCloseStopsEverything` ("counted by `runtime.NumGoroutine` before and after, with a ceiling").

**Problem.** `Close` returns when every goroutine called `wg.Done`, which a goroutine does (deferred) before it has exited.
Verified: `numg/main.go` sees `NumGoroutine` above the baseline right after `wg.Wait` in 15 of 2 000 runs. Ground rule 11:
a flaky test stops the line.

**Fix.** Poll `NumGoroutine` until it is at or below the baseline, with a deadline (yielding with `runtime.Gosched`, no
`time.Sleep`, testing.md rule 1). Name it in §8.

### F18 — later — Per-event fan-out; chunking must respect D5

**Where:** §4 step 4 ("Every shard gets the advances"), §4 (walk per message), R2; ADR-0010 §2's D1 annotation.

**Problem.** Live, nearly every in-order event moves its stream's contribution (`ObserveEvent` raises admission with each new
max), so each event becomes `Shards` mailbox sends and `Shards × (the app's rules)` `Apply`+`Advance` calls. Throughput
cannot grow with shards: the router's per-event work grows with them. The future fix — coalescing advance-only messages per
shard, or walking per chunk (ADR-0010 §2) — is safe under D1 only if it never spans a snapshot change (D5), and only if a
target shard receives its deferred advances before an offer.

**Fix (later, measured).** Record in §4 the two conditions any coalescing must keep: cut at every snapshot-pointer change;
flush a shard's deferred advances before its next offer and when the intake is empty (no timer waits, ADR-0010 §2). Annotate
ADR-0010 §2's D1 note with the D5 caveat. Nothing in this design blocks it.

### F19 — later — One router for all apps couples their sinks

**Where:** §3. One FIFO holds every app's mailbox and sink sends; its head blocks everything behind it. One app's slow sink
stalls all apps; S5's retry and backoff inside a flusher would make a sink outage a runtime outage. ADR-0006 §4 chose one
router per runtime, so this is accepted by the ADR — but ADR-0012 §2 sells "deployment isolation". State it in §3 and forward
the isolation question to S5 (retry must never hold a queue's receive side for longer than one attempt).

### F20 — later — Two §7 rules are not gate-enforced

**Where:** §7 ("No `time.Sleep` anywhere"), §1. `archcheck` allows every `clockCalls` entry, `Sleep` included, in
`internal/runtime` (`tools/archcheck/main.go` `clockCalls`, the clock check). `Ingest` now lives in a package allowed
goroutines, channels, `sync` and the wall clock, so its purity (the replay arithmetic) is no longer guarded. Fix: a test that
scans `internal/runtime`'s non-test files for `time.Sleep`, and one that `ingest.go` uses no `time`, `sync` or channel; or an
`archcheck` row (S1.1c).

### F21 — later — Unspecified edges

**Where:** §2 Start, §4 (a tick), §5.
- Start's validation: `len(KeyAttr) == len(Inputs)` (or nil), key attributes in range and of `KeyType`, no duplicate input,
  sink bindings on derived streams only, `Shards ≥ 1` after the default.
- A tick whose `adv` is empty should send nothing (§4 says "advance-only messages to every shard").
- The drain of an app with no rules: complete at once (or send acks anyway); with no sink binding: complete at the last ack.
- Concurrent drains of one app: one token each, both complete.

### F22 — later — A sink calling back into the runtime deadlocks

**Where:** §2 Sink. A `Sink` that calls `Send` (the host wiring one app's output to another's input, ADR-0012 §2's "cross-app
wiring = the host program moving events") blocks on the intake while the router is blocked on that sink's queue; `Drain` or
`Close` from a sink waits on its own flusher. State: a `Sink` MUST NOT call the `Runtime`; cross-app forwarding goes through
the host's own queue.
