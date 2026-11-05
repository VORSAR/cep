# S3.2a-2 — design note: the runtime core (v1)

**What it covers:**
- S3.2a's item 3 ([card](sprint-plan.md#s32a--the-replay-format-and-the-runtime-core)) and exit line 3 (saturation).
- The forwards from S3.2a-1 (design v2 §10, under the S3.2a card).
- The owner's plan-check answers ([sprint-plan §5, "S3.2a-2's plan check"](sprint-plan.md#owner-decisions-2026-11-05--s32a-2s-plan-check)):
  - the drain barrier's mechanism is built here;
  - one shared ingest step serves the router and the replay driver.

**Routine (owner):** QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it.

v1 is for the architecture review and QA's tests. The stub commit carries every signature below with panicking bodies. MUST and MUST NOT below are the build's contract.

## 0. What the code says (read first)

1. **The pieces a shard drives already exist and are single-threaded.**
   - `partition.Rule` (Apply, Offer, Advance; one goroutine per Rule).
   - The clock finalizes an instant only once it is past it (owner D1 of S3.2a-1): a shard may walk after every message or once per chunk with the same result.
   - `table.Set.Snapshot` is single-goroutine.
   - `clock.StreamRegistry` is written by one goroutine.
2. **The ingest arithmetic lives in `playback.Driver` today** (Seq, the snapshot, `ObserveEvent`, the virtual floor, ticks, `Drain`, `EndOfInput`).
   - The import law lets `playback` import `runtime`, never the reverse (`tools/importcheck/rules.json`). So the shared ingest step lives in `internal/runtime`, and the driver calls it (owner, plan check).
3. **A partition key is an attribute.** The router reads `e.Values[attr]`.
   - `-0.0` and `+0.0` are one key (break-it K1). The router MUST hash the key with `-0.0` as `+0.0`, or one key would split across two shards.
4. **No rule body emits yet** (operators are S3.2b's plans, then S4.1). A body emits through an `Emit` function this package gives it, one per (rule, shard).
5. **Rules cannot read a derived stream in v0.1** (E-GRAPH-004), so an emitted event goes to its stream's sinks only, and never re-enters routing. ADR-0012 §3's fixpoint therefore completes in one round. The loop stays, for Phase 2.
6. **`archcheck` allows goroutines, channels, `sync` and the wall clock in `internal/runtime` only** (with `io/*` and `cmd`).
   - The race row and goleak are S3.2b's.
   - Exit 3's `-race` run is a recorded local run (S3.2a's plan-check reading 3).

## 1. The shared ingest step (owner)

```go
// Ingest is one app's ingest arithmetic, used by one goroutine: the live router, or the playback driver.
type Ingest struct{ /* reg *clock.StreamRegistry; set *table.Set; seq []uint64; vnow int64; virtual bool; buf []clock.WMAdvance */ }

// NewIngest: start is the deployment's start (wall ms); virtual is playback's fixture mode (the virtual floor after
// every admitted event); a recording and live run without it.
func NewIngest(specs []clock.StreamSpec, start int64, set *table.Set, virtual bool) *Ingest

// Event stamps e — Seq per stream from 1, a stream-late event included; Tables = Snapshot() — then judges it with the
// registry. Advances: its own stream's, then (virtual) the virtual floors. The slice is valid until the next call.
func (g *Ingest) Event(e *event.Event, arrival int64) (late bool, advances []clock.WMAdvance)
func (g *Ingest) Tick(now int64) []clock.WMAdvance // ObserveTick
func (g *Ingest) Drain() []clock.WMAdvance         // the registry's Drain: one past the max admitted event time
func (g *Ingest) End() []clock.WMAdvance           // EndOfInput: +∞
func (g *Ingest) Snapshot() *table.Snapshot        // the current snapshot (D5: what a walk this step triggers reads)
func (g *Ingest) Registry() *clock.StreamRegistry
func (g *Ingest) Walk(fn func(component, key string, state any)) // "seq" per stream: a state holder
```

- **The arithmetic is S3.2a-1 design v2 §5.3's, moved.** `playback.Driver` keeps its API and builds one `Ingest`: virtual in a fixture; rebuilt with the header's start, non-virtual, in a recording. Its steps call the methods above. S3.2a-1's playback tests are the regression proof.
- **The live router uses the same `Ingest`,** with `virtual = false`.

## 2. The runtime

```go
type AppID int // dense, from 0, in Start's order

// Emit is a body's output: an event on a derived stream (e.StreamID); it goes to that stream's sinks.
type Emit func(e event.Event)

// Rule is one rule of an app, as S3.2b's planner will describe it.
type Rule struct {
	Name    string
	Inputs  []int      // the stream ids the rule reads
	KeyAttr []int      // per input, the key attribute's index in that stream; nil: key-less (the pseudo-key "")
	KeyType event.Type // the keys' type; TString for a key-less rule
	TTL     int64      // ms; 0 for a key-less rule
	NewBody func(emit Emit) partition.Body // one body per (rule, shard)
}

// Sink receives one derived stream's outputs on its own flusher goroutine (the facade adapts cepio.Sink in S3.3).
// An error is counted; retry and backoff are S5's.
type Sink func(e event.Event) error

type SinkBinding struct {
	Stream int // a derived stream's id
	Sink   Sink
}

type App struct {
	Streams []clock.StreamSpec // declaration order; their index is the stream id
	Tables  *table.Set         // nil: no tables
	Rules   []Rule
	Sinks   []SinkBinding
}

type Config struct {
	Shards    int               // 0: GOMAXPROCS (ADR-0006 §1)
	Mailbox   int               // per shard, in messages; 0: 1024
	Intake    int               // the router's intake queue; 0: 1024
	SinkQueue int               // per sink binding; 0: 1024
	TickEvery time.Duration     // live idle ticks from an internal ticker; 0: none (tests call Tick)
	Now       func() int64      // wall ms; nil: time.Now; injectable
}

// Start validates the apps, builds every shard's rules, and starts the router, the shards and the sink flushers.
func Start(cfg Config, apps []App) (*Runtime, error)
func (r *Runtime) Send(ctx context.Context, app AppID, e event.Event) error // thread-safe; blocks for backpressure
func (r *Runtime) Tick(now int64) error                                     // a wall tick into the intake
func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) // the drain barrier (§5)
func (r *Runtime) Stats(ctx context.Context) (Stats, error)                 // a consistent cut through the intake (§6)
func (r *Runtime) Close(ctx context.Context) error                          // stops every goroutine; drops the rest
```

**`Send`'s contract:**
- `e.StreamID` is a source stream of the app, and `e.Values` is aligned to its schema (the decoder's output).
- `e.Ts` is the event time. A `time arrival` stream's live stamping is S3.2b's item 3; here `Ts` is taken as given.
- `Seq` and `Tables` are the router's.
- `Send` after `Close` is an error.
- It waits on the intake queue only, and respects `ctx`.

## 3. Goroutines and their waits (ADR-0006 §5, made concrete)

| Goroutine | Owns | May wait on | Never waits on |
|---|---|---|---|
| a caller of `Send` / `Tick` / `Drain` / `Stats` | — | the intake queue (send); its own reply channel | anything else |
| the router (one) | every app's `Ingest`; the routing index; the pending-send FIFO | the intake receive (only with nothing pending); the head of the pending FIFO (a mailbox or a sink-queue send) **together with** the shared handoff receive; the ticker | shards' state, sinks |
| a shard (`Shards` of them) | its `partition.Rule`s (every app's rules); its outbox; its counters | `select { mailbox receive, handoff send of a non-empty outbox }` | sinks, other shards, the router's state, I/O, tables |
| a sink flusher (one per binding) | its queue's receive side | its queue receive; its own `Sink` call | everything else |
| the ticker (when `TickEvery > 0`) | — | its `time.Ticker`; the intake send | everything else |

**The handoff.** One channel shared by every shard, with capacity `Shards`, carrying `handoff{shard, items}`.
- A shard with a non-empty outbox includes the handoff send in its parking `select`, so it never blocks on the handoff alone. A handed-off outbox belongs to the router from then on, and the shard starts a new one.
- The router's every wait includes the handoff receive. That is ADR-0006 §5's "the router services every handoff while blocked". With one shared channel, it needs no reflection.

**The router's loop:**
1. **While the pending FIFO is non-empty:** `select` on the head's send (a shard's mailbox or a sink queue), the handoff receive, and the stop signal.
   - A received handoff's items are routed: outputs to their stream's sink queues, appended to the FIFO; a drain ack is recorded (§5).
2. **Otherwise:** `select` on the intake, the handoff, and the stop signal.
   - An intake item is an event, a tick, a drain request or a stats request. It turns into mailbox sends appended to the FIFO, in shard order.

The router never reads the intake while anything is pending. That is the backpressure: sources feel it, shards never do. The FIFO stays bounded by one intake item's messages plus the outputs of the mailboxes' in-flight work.

**Why it cannot deadlock** (ADR-0006 §5's argument, kept):
- every edge a shard waits on is completed by the router, which always also receives handoffs;
- a flusher waits only on its own sink;
- the router waits on a mailbox or a sink queue only while also receiving handoffs.

The saturation test (exit 3) is the proof.

## 4. One event's path (the step contract, live; S3.2a-1 design §1, D5)

For an intake event `e` of app `a`:
1. `arrival = max(lastArrival, Now())` (monotone).
2. `late, adv := a.ingest.Event(&e, arrival)`.
   - If `late`, count `stream_late` for the stream; stop.
3. **Offers.** For each rule `R` of `a` reading `e.StreamID` (the routing index):
   - **The key** is `e.Values[R.KeyAttr[i]]`, or `""` for a key-less rule.
   - **A bad key** — null, NaN, or not `R.KeyType` — is a per-rule event error, counted for `R`; nothing is offered.
   - **The shard** is `fnv1a64(appID, ruleIndex, canonicalKey) % Shards`: `-0.0` hashed as `+0.0`; a key-less rule hashes `(appID, ruleIndex)`.
4. **One message per shard,** in shard order, appended to the pending FIFO, when it has offers or `adv` is non-empty: `{app, kind: event, adv (copied), tables: e.Tables, ev: e, offers: [(rule, key)…] for this shard}`.
   - The event's own advances ride with it (S3.2a-1 §1), and every message carries the snapshot (D5).
   - Every shard gets the advances, because a rule's clock moves on every shard, with or without keys (ADR-0004 §3; S3.1-1 D2).

**A tick:** `adv := a.ingest.Tick(now)` for every app; advance-only messages to every shard, each with that app's current snapshot (D5).

**A shard's message** (§3: one goroutine; each rule's work under `recover`):
1. **Apply** each advance to each of the app's rules that reads its stream.
2. **Offer** the event to each listed rule under its key; a construct-late result counts for that rule.
3. **Advance** every rule of the app with the message's snapshot.

A shard walks after every message. D1 makes per-chunk walking equal, and a chunked walk is a later, measured optimization.

**The outbox.**
- An `Emit` appends `{app, event}` to the shard's outbox.
- A panic in a rule's `Offer` or `Advance`, a body included, is recovered at the shard boundary: it is counted as a per-rule event error (`panics`), and the shard carries on with the next rule and message (ADR-0012 §6). The rule's clock stays consistent, because the walk removed the item before its callback.

## 5. The drain barrier (owner; ADR-0012 §3)

**`Drain(ctx, app)`:**
1. **The request.** An intake item, so it follows every earlier `Send` of the app; Stop stops sources first (S3.3).
   - It stops the internal ticker first (owner D3 of S3.1-1: the ticks stop before the sources).
2. **The router's part:**
   - `adv := a.ingest.Drain()`, so instants up to the max admitted event time finalize, and nothing later fires;
   - a `drain` message to every shard with `adv`, the app's snapshot and a token.
3. **Each shard:** applies, walks, counts the app's pending timers as `drained_pending`, appends an ack `{token, pending}` to its outbox, and hands off. A shard with nothing else still hands off its ack.
4. **The router, with every shard's ack in:**
   - it has received, and queued in order, every output the shards produced before the ack;
   - it appends a flush marker behind them to each of the app's sink queues;
   - each flusher signals its marker once it reaches it.
5. **Completion.** When every marker is signalled and the app's mailbox work and pending FIFO are empty, the round is complete.
   - A second round follows only if a round added work; in v0.1 none can.
   - `Drain` returns `DrainResult{Pending}` (the summed `drained_pending`).
   - On `ctx`'s deadline it returns `ctx.Err()`, and the barrier still completes inside the runtime.

`Drain` does not stop the runtime: later `Send`s of the app are processed normally. S3.3's `Stop` = stop sources → `Drain` → detach.

## 6. Stats and Close

**`Stats(ctx)`:** an intake item.
- The router asks every shard through its mailbox. Each shard's reply rides its handoff, so the counters are a consistent cut at that intake position, and there are no atomics on the hot path (ADR-0006, implementation notes).
- **Counters:**
  - per app and stream: `stream_late`;
  - per app and rule: `construct_late`, `purged`, `key_errors`, `panics`;
  - per sink binding: `sink_errors`;
  - the router's high-water marks: the pending FIFO's length, and each mailbox's length.

**`Close(ctx)`:**
1. Stops the ticker and the intake.
2. The router finishes its pending FIFO, then closes the mailboxes.
3. Shards hand off their last outboxes and exit.
4. The router routes them, closes the sink queues and exits.
5. Flushers drain their queues and exit.

`Close` waits for every goroutine (a `sync.WaitGroup`) until `ctx`'s deadline. It fires no pending timer (MVP stop is graceful or nothing, ADR-0012 §3). A test then sees no goroutine left; goleak is S3.2b's.

## 7. MUST / MUST NOT

- The router alone writes mailboxes and sink queues, and alone reads the intake (ADR-0006 §4).
- A shard MUST NOT block on anything but its parking `select`; a flusher, on anything but its queue and its sink.
- Per key, events reach the body in the order the clock releases them, which equals replay's for the same admitted events (SEMANTICS §2.1, §2.2). Across keys, live order is free.
- A `-0.0` key and a `+0.0` key reach one instance on one shard.
- A table-reading timer reads the snapshot of the message whose walk fires it, so its result does not depend on `Shards` (D5).
- No `time.Sleep` anywhere. Every wait is on a state, with a deadline or a stop signal (testing.md rule 1 applied to production code).

## 8. Tests (QA, written first; each red on the stub for its stated reason)

| Line | Test | Proof |
|---|---|---|
| **Exit 3** | `TestSaturationProgress` (`internal/runtime`) | Shards 2, mailbox 1, a body that emits a burst per event, a sink that blocks until the test releases it, a bursty source. The scenario: a shard's mailbox is full while its handoff is pending. Every output still arrives and `Drain` completes, bounded by a ceiling. Run 50/50 under `-race` with a CPU hog, recorded by the coordinator. |
| Live = replay per key | `TestLiveOrderPerKeyEqualsReplay` | A log replayed through `playback` (the trace harness) and sent live with 1 and with 4 shards: each key's released events and timer fires are identical. Live ticks are off; the drain stands in for the end. |
| D5 | `TestTimerTablesIndependentOfShards` | A table-reading timer gives the same note with 1 and with 4 shards. |
| Keys | `TestKeyErrorsAndZeroKeys` | Null, NaN and wrong-type keys are counted as per-rule errors and never offered; `-0.0` and `+0.0` reach one instance. |
| Pseudo-key | `TestKeylessRuleOneInstance` | A key-less rule runs as one instance, never purged. |
| Panics | `TestPanicIsAPerRuleError` | A body that panics on one key: counted; other rules and keys of the same event are unaffected. |
| Drain | `TestDrainBarrier` | Outputs produced by the barrier's walk reach the sinks before `Drain` returns; deadlines after the max event time stay unfired and are counted; `Send` after `Drain` still works; a ctx deadline returns its error. |
| Ingest | `internal/playback`'s existing tests, plus `TestIngestMatchesDriver` | The driver on `Ingest` passes every S3.2a-1 test; the router's Seq, snapshot and advances equal the driver's on one log. |
| Close | `TestCloseStopsEverything` | No goroutine of the runtime is left (counted by `runtime.NumGoroutine` before and after, with a ceiling); `Send` after `Close` errors. |

## 9. For the review

- **R1:** one shared handoff channel with capacity `Shards`, against one per shard. Is the FIFO's bound argument sound?
- **R2:** walking after every message, against D1's per-chunk option.
- **R3:** the drain ack through the outbox, and the flush marker through the sink queues. Does every output produced before the barrier reach its sink before `Drain` returns?
- **R4:** consistent stats through the mailboxes, against plain counters read after a barrier.
- **R5:** messages allocate per event (advances copied, offers slices). The allocation budget for ingest → shard is S3.2b's `allocgate`; is anything here structurally unfixable later?

## 10. Size

About 7.5 points:
- item 3: 6;
- the drain barrier: 1;
- the shared ingest step: 0.5.
