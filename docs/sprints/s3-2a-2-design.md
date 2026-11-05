# S3.2a-2 — design note: the runtime core (v2)

**What it covers:**
- S3.2a's item 3 ([card](sprint-plan.md#s32a--the-replay-format-and-the-runtime-core)) and exit line 3 (saturation).
- The forwards from S3.2a-1 (under the S3.2a card).

**The owner's answers:**
- at the plan check ([sprint-plan §5, "S3.2a-2's plan check"](sprint-plan.md#owner-decisions-2026-11-05--s32a-2s-plan-check)): the drain mechanism is built here, with one shared ingest step;
- after the architecture review ([§5, "S3.2a-2's design"](sprint-plan.md#owner-decisions-2026-11-05--s32a-2s-design-after-the-architecture-review)):
  - **F14:** table writes join the intake;
  - **F5:** a drain ends the app's input;
  - **F10:** a body's panic resets that key.

**Routine (owner):** QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it.

v2 answers the review ([s3-2a-2-design-architecture.md](../reviews/s3-2a-2-design-architecture.md): 2 blockers, 15 should-fix, 5 later) and QA's gaps Q1–Q9; §10 places each. MUST and MUST NOT lines are the build's contract.

## 0. What the code says (read first)

1. **The pieces a shard drives exist and are single-threaded:**
   - `partition.Rule`: one goroutine per Rule;
   - the clock: it finalizes an instant only once it is past it (D1 of S3.2a-1);
   - `table.Set.Snapshot` and `clock.StreamRegistry`: single goroutine.
2. **The ingest arithmetic lives in `playback.Driver`.** The import law lets `playback` import `runtime`, never the reverse, so the shared step lives here and the driver calls it.
3. **A partition key is an attribute**, and `-0.0` and `+0.0` are one key. The router hashes `-0.0` as `+0.0`.
4. **No rule body emits yet.** A body emits through an `Emit` this package gives it, one per (rule, shard).
5. **v0.1 rules cannot read a derived stream,** so an emitted event goes to sinks only, and the drain completes in one round.
6. **The race row and goleak are S3.2b's;** exit 3's `-race` run is a recorded local run.
7. **A body's panic inside a walk unwinds `partition.Rule.Advance`** and leaves the rest of the walk undone (review F1, confirmed by a scratch test). A panicking `OnPurge` leaves a born key with no TTL timer, which is never purged (F10, confirmed). → §1b.

## 1. The shared ingest step

As stubbed (`internal/runtime/ingest.go`): `NewIngest(specs, start, set, virtual)`, `Event`, `Tick`, `Drain`, `End`, `Snapshot`, `Registry`, `Walk`.
- **The arithmetic** is S3.2a-1 design v2 §5.3's, moved. `playback.Driver` keeps its API and calls an `Ingest`: virtual in a fixture; rebuilt with the header's start, non-virtual, in a recording.
- **The table set may be shared** with its owner (QA Q8): `Ingest` only takes snapshots.
- **`Ingest` is pure** (no goroutines, no clock reads). F20: `archcheck` cannot tell `ingest.go` from the rest of `internal/runtime`, so a test reads its imports instead.

## 1b. `partition` changes (owner F10; review F1, F10, F11)

**A body's panic is recovered inside `partition`**, in each callback (`NewState`, `OnEvent`, `OnTimers`, `OnDiscard`, `OnPurge`). That key is reset as a purge resets it:
- `CancelAll`, `Op = nil`, `LastActivity = math.MinInt64`;
- the key is marked for freeing;
- its buffered events stay and start it fresh;
- `Stats.Panics` counts the panic; `Purged` does not.

The walk then continues: every other key's due work is released in the same `Advance` (F1).

- **`OnPurge`'s own panic** still completes the purge (in a `defer`), so no key is left born without a TTL timer.
- **`CheckTimers` holds after any panic.**
- **`Stats` gains `Panics`.**
- **`func (r *Rule) Pending() int`**: the pending timers of every resident key, TTL timers excepted (F11; QA Q3), for `drained_pending`. It is on the cold path.

A panic in partition's own code, not a body's, is still recovered at the shard boundary (§4).

## 2. The runtime's API (stub v2)

- **`App` gains `Schemas []event.Schema`**, aligned with `Streams`. `Start` validates every rule against them (F9, F21):
  - its inputs are source streams in range;
  - each `KeyAttr` is in range, and its type is `KeyType`;
  - a keyed rule has `TTL > 0` (QA Q4; the planner's bug, refused);
  - a key-less rule has `TTL` 0;
  - each sink's stream is derived;
  - every `NewBody` is non-nil.
- **`Send(ctx, app, e)` checks in the caller's goroutine** (F9) and returns an error, never panicking the router:
  - the app is in range and not drained;
  - the stream is a source stream of the app;
  - `len(e.Values)` equals its schema's attribute count;
  - each value's `Kind` is its declared type.

  **Ownership (F4):** `Send` takes `e.Values`, and the caller MUST NOT change it afterwards.
- **`Tick(ctx, now) error`** (F7).
- **`TableWrite(ctx, app, table int, op TableOp, rows [][]event.Value) error`** (owner F14): an intake item. The router applies it to `app.Tables` in intake order (`Upsert`, or `Delete` with key values), and it returns the table's error once applied.
- **After `Close`,** every method returns `ErrClosed`, and a second `Close` returns nil (F7; QA Q6).
- **After an app's drain,** `Send`, `Tick` and `TableWrite` for it return `ErrDrained` (owner F5).
- **`Stats` gains:**
  - `SaturatedParks []int64`: per shard, the times it parked with a non-empty outbox while its mailbox was full (F16), the moment exit 3 must reach;
  - `AfterDrain [][]int64`: per app and stream, items refused after the drain by intake order (a `Send` racing its `Drain`).

## 3. Goroutines and their waits

| Goroutine | May wait on | Never waits on |
|---|---|---|
| a caller of `Send`, `Tick`, `TableWrite`, `Drain`, `Stats` | `select { intake send, stopped, ctx.Done }`; then its reply with the same `select`; `Drain` also on its flush markers (§5) | anything else |
| the router (one) | with nothing pending: `select { intake, handoff, stopped }`; with something pending: `select { the FIFO head's send, handoff, stopped }` | sinks, shards' state, callers' replies (reply channels are buffered, 1) |
| a shard | `select { mailbox, handoff send of a non-empty outbox }` | anything else |
| a sink flusher | its queue's receive; its own `Sink` call | anything else; a flush marker is a `close`, which never blocks (F3) |
| the ticker | `select { ticker, stopped }`, then the intake send under `select { …, stopped }` | anything else |

- **The intake channel is never closed** (F7): stopping is the `stopped` channel's close.
- **One shared handoff channel** with capacity `Shards` (R1). The FIFO is bounded in messages, and its bytes by ADR-0006 §5's per-`Advance` emit ceiling, which is S3.2b's tripwire item (F13).
- **The deadlock argument of v1 §3 stands**, with the caller and flusher rows above.
- **MUST NOT: a `Sink` calls back into the runtime** (F22). A sink that blocks on the runtime would deadlock its flusher against the router.
- **A `Sink` that panics** is recovered by its flusher and counted as a sink error, and the flusher continues (F8).

## 4. One event's path; ticks; table writes

**Wall stamps.** One monotone stamp serves arrivals and ticks alike: `stamp = max(last, Now())`. A tick's time is clamped to it, so a recording of what the router applied never has a tick before an arrival (F6).

**An event** of app `a` (stream `s`):
1. If `a` is drained, count `AfterDrain` and stop.
2. `stamp`; `late, adv := a.ingest.Event(&e, stamp)`. If `late`, count `stream_late`.
3. **Offers.** For each rule `R` of `a` reading `s`, the key is `e.Values[R.KeyAttr[i]]`, or `""` for a key-less rule.
   - A null or NaN key is a per-rule `key_errors` (a wrong type cannot pass `Start` and `Send`).
   - The shard is `fnv1a64(app, rule, canonical key) % Shards`, with `-0.0` hashed as `+0.0`; a key-less rule hashes `(app, rule)`.
4. **Messages.** One per shard with offers or a non-empty `adv`, appended to the FIFO in shard order: `{app, adv (copied), tables: e.Tables, ev: e, offers}`.

**A tick:** for every app not drained, `adv := a.ingest.Tick(stamp)`. A non-empty `adv` sends advance-only messages to every shard, with that app's current snapshot (D5). The ticker keeps running for the other apps after one drains (F2).

**A table write** (F14): applied by the router to `a.Tables` in intake order. `a.ingest.Snapshot()` then publishes the next snapshot at the next event or tick. Its reply carries the table's error.

**A shard's message**, for each rule of the app:
1. **Apply** the advances, then **offer** the event under its keys, counting construct-late.
2. **Advance** with the message's snapshot. A shard walks after every message (R2): per-chunk walking is equal only if a chunk never spans a snapshot change (F18), and it is a later, measured optimization.
3. **Under `recover`, as a last resort** (partition recovers bodies itself, §1b): a recovered panic is counted as `panics`, and `Advance` is called again until it returns normally. Each panic consumed one item, so this ends (F1).

**The outbox.** An `Emit` appends `{app, event}`; the runtime owns the event from then on (F4). A `Sink` receives events it MUST NOT change: several bindings may share one.

## 5. The drain barrier (owner: here; F5 terminal; F2, F3, F11)

**`Drain(ctx, app)`:**
1. **The caller.**
   - It sets the app's draining flag (atomic), so later `Send`s fail fast. A `Send` already past the check lands after the request in the intake and is refused there, counted in `AfterDrain`.
   - It enqueues the request.
2. **The router** (intake order):
   - `drained[a] = true`, so no later item of `a` (event, tick or write) reaches the shards (F2: no tick fires after the drain point);
   - `adv := a.ingest.Drain()`;
   - a `drain` message to every shard, with `adv`, the snapshot and a token.
3. **Each shard:** applies, walks, sums `Rule.Pending()` over the app's rules, appends `ack{token, pending}` to its outbox, and hands off. An empty outbox still hands off its ack.
4. **The router, on every shard's ack:**
   - **The order.** Every output a shard produced before its ack has already been appended to the FIFO (a handoff's items go in order).
   - **The flush markers.** It appends one per sink binding of `a` through the FIFO, behind those outputs, so they reach the queues in order.
   - **The reply.** It replies to the caller with the markers' `done` channels and the summed `Pending`.
5. **The caller** waits for every marker's `done`. A flusher `close`s it on reaching the marker, never blocking (F3). The caller then returns `DrainResult{Pending}`.
   - On `ctx`'s deadline it returns `ctx.Err()`, and the barrier still completes inside the runtime.

**What `Drain` returning means:** every output produced before the barrier has been through its `Sink`. Nothing after it can fire (R3, under the MUSTs of §7).

## 6. Stats and Close

**`Stats(ctx)`:** an intake item.
- **Shard counters** come back through the mailboxes and handoffs, a consistent cut at that position.
- **`sink_errors`** are counters the flushers increment atomically and `Stats` reads directly, outside the cut (F12; QA Q5).
- `Stats` waits behind a stuck sink like any intake item, and respects `ctx`.
- **The router's high-water marks:** the FIFO's length; each mailbox's length after each send (QA Q2).

**`Close(ctx)`** (F7):
1. It closes `stopped`. Callers and the ticker return `ErrClosed`.
2. **The router** abandons its FIFO and closes the mailboxes.
3. **Shards** finish their current message and exit.
4. **The router** waits for them, then closes the sink queues.
5. **Flushers** finish their current `Sink` call, then exit, dropping the rest.

`Close` waits on a `sync.WaitGroup` until `ctx`'s deadline, and returns `ctx.Err()` if a `Sink` never returns: that flusher is leaked, and the docs say so. It fires no pending timer (graceful or nothing, ADR-0012 §3).

## 7. MUST / MUST NOT

- **One writer, one reader:** the router alone writes mailboxes and sink queues, applies table writes, and reads the intake.
- **What may block:**
  - a shard blocks only in its parking `select`;
  - a flusher only on its queue and its sink;
  - the router never blocks without also receiving handoffs and watching `stopped`.
- **Order:**
  - per key, events reach the body in the clock's release order, equal to replay's for the same admitted events (SEMANTICS §2.1, §2.2);
  - every message carries its snapshot (D5);
  - an event's own advances ride with it.
- **Keys:** `-0.0` and `+0.0` reach one instance on one shard.
- **Ownership:** values given to `Send` and `Emit` are the runtime's; a `Sink` never changes an event; a `Sink` never calls the runtime.
- **No `time.Sleep`.** Every wait is on a state, with a deadline or the stop signal.

## 8. Tests (QA, written first)

**Written (red on the stub):**
- `TestSaturationProgress` (exit 3);
- `TestTimerTablesIndependentOfShards` (D5);
- `TestIngestMatchesDriver`;
- `TestLiveOrderPerKeyEqualsReplay`, in `internal/playback`, because runtime's tests cannot import playback (QA Q1, accepted).

**Revised and added for v2:**
- **Exit 3** also asserts `SaturatedParks > 0`, so the scenario was reached (F16).
- **Live = replay** uses a **recording** ending with a stop record, against a live run with no ticks and a `Drain`. The two are the same arithmetic, so every key's sequence must be equal, with 1 and with 4 shards (F15).
- **D5's test** orders its table writes with `TableWrite`, now in intake order (F14).
- **The rest:**
  - `TestKeyErrorsAndZeroKeys`;
  - `TestKeylessRuleOneInstance`;
  - `TestPanicResetsTheKey`: one key's body panics; its state resets, the panic is counted, and other keys and rules are unaffected in the same walk;
  - `TestDrainBarrier`: barrier outputs reach sinks first; later deadlines stay unfired and `Pending` counts non-TTL timers; after the drain, `Send`, `Tick` and `TableWrite` return `ErrDrained`, while other apps keep their ticks;
  - `TestTableWriteInIntakeOrder`;
  - `TestSendValidation`;
  - `TestSinkPanicCounted`;
  - `TestCloseStopsEverything`: polls the goroutine count to a deadline (F17); `ErrClosed`; a second `Close` returns nil;
  - `Start`'s validation.
- **`internal/partition`:** `TestBodyPanicResetsTheKey`, `TestPanicInOnPurgeStillPurges`, and `TestPendingExcludesTTL`.

The coordinator runs exit 3 50 times under `-race` with a CPU hog and records it.

## 9. Size

About 8.5 points:
- item 3: 6;
- the drain barrier: 1;
- the shared ingest step: 0.5;
- table writes through the intake: 0.5;
- partition's panic reset: 0.5.

## 10. Review and test-gap dispositions

| # | Severity | Disposition |
|---|---|---|
| F1 | blocker | Fixed (§1b, §4): partition recovers per callback; the shard re-calls `Advance` until it returns. |
| F2 | blocker | Fixed (§4, §5): a per-app drained mark by intake order; ticks continue for other apps. |
| F3 | should-fix | Fixed (§3, §5): a marker is a `close`; the `Drain` caller waits, not the router. |
| F4 | should-fix | Fixed (§2, §4, §7): ownership rules. |
| F5 | should-fix | Owner: a drain ends the app's input (§2, §5). |
| F6 | should-fix | Fixed (§4): one monotone wall stamp. |
| F7 | should-fix | Fixed (§2, §3, §6): `stopped`, never closing the intake; ctx everywhere; the `Close` contract. |
| F8 | should-fix | Fixed (§3): a flusher recovers a sink panic. |
| F9 | should-fix | Fixed (§2): `Send` and `Start` validate. |
| F10 | should-fix | Owner: reset the key (§1b). |
| F11 | should-fix | Fixed (§1b): `Rule.Pending()` excludes TTL. |
| F12 | should-fix | Fixed (§6): atomic sink-error counters; the documented wait. |
| F13 | should-fix | Annotated (ADR-0006 §5): depth metrics are `Stats`' high-water marks; the emit ceiling is S3.2b's tripwire. |
| F14 | should-fix | Owner: table writes join the intake (§2, §4). |
| F15 | should-fix | Fixed (§8): a recording against a live run with a drain. |
| F16 | should-fix | Fixed (§2, §8): `SaturatedParks`. |
| F17 | should-fix | Fixed (§8): poll to a deadline. |
| F18 | later | Noted (§4): chunking must cut at snapshot changes. |
| F19 | later | Forwarded to S5: one router for every app means one app's slow sink stalls the others. A known limit of the single-node MVP, in the docs. |
| F20 | later | §1: a test reads `ingest.go`'s imports. |
| F21 | later | §2: `Start`'s validation listed; a tick with empty advances sends nothing. |
| F22 | later | §3, §7: MUST NOT. |
| Q1–Q9 | QA | Q1 accepted; Q2 §6; Q3 §1b; Q4 §2; Q5 §6; Q6 §2; Q7 F1; Q8 §1; Q9 as tested. |
