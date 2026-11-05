# S3.2a-1 — design note: the replay format layer and partitions (v2)

S3.2a's items 1–2 ([card](sprint-plan.md#s32a--the-replay-format-and-the-runtime-core)), exit lines 1, 2 and 4, and the forwards from S3.1-2:
- the table-snapshot carrier (owner D1);
- the timer snapshot (review F15);
- the encoder's malformed UTF-8 (review F11).

**The owner's answers:**
- at the plan check: [sprint-plan §5, "S3.2a's plan check"](sprint-plan.md#owner-decisions-2026-11-05--s32as-plan-check);
- after the architecture review: **D1–D7**, [sprint-plan §5, "S3.2a-1's design"](sprint-plan.md#owner-decisions-2026-11-05--s32a-1s-design-after-the-architecture-review).

**Routine (owner):** QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it.

v2 answers the architecture review ([s3-2a-1-design-architecture.md](../reviews/s3-2a-1-design-architecture.md): 1 blocker, 8 should-fix, 4 later) and QA's notes on the partition tests (A1–A9); §9 places every finding. The stub commit carries the API with panicking bodies. MUST and MUST NOT below are the build's contract.

## 0. What the code says (read first)

1. **A partition key is an attribute, not an expression.** `model.PartitionKey` is `{Attr, Stream}` (`internal/model/model.go`). One block keys every stream on one type (E-PART-003, `internal/validate/bounds.go`). The router (S3.2a-2) reads the key, and `partition` receives an `event.Value`.
2. **The validator accepts an `object` partition key**, and the language defines no equality for objects (D7: a deploy error in S3.3).
3. **The clock finalized an instant as soon as its clock reached it** (`internal/clock/rule.go`: `nextEvent` released `Ts ≤ clock`; `nextTimer` fired inclusive kinds at `due ≤ clock`).
   - **Why that is wrong.** A watermark W promises only that nothing *before* W will still arrive (SEMANTICS §2.1), so an event exactly at W can still come.
   - **What S3.1-1 did about it.** Its D7 seal made such an event construct-late: dropped, counted, and a deadline event lost (review F1; S3.1-1's review, D1-2).
   - **The fix.** D1: an instant is finalized only once the clock is past it (§4.1).
   - **The oracle agrees.** Measured on a scratch copy of the oracle with the D1 drain, 3 of its 47 checks change (§6). On a scratch copy of the clock, 20 of its 43 tests change: each asserts a release or a fire at exactly the clock.
4. **`clock.Instance.Schedule` allocates a new timer on every call, a replacement included**, and the TTL is re-armed on every released event. This is against ADR-0010 §1's zero-allocation budget; QA's test measures 1 allocation per re-arm. → §4.3.
5. **The clock exposes no view of a key's buffered events, pending timers, birth order, or the rule clock's contributions.** The state registry and the derivability invariant need them (ADR-0006 §2(b), (c); review F4). → §4.2.
6. **`event` imports nothing**, so the per-event table snapshot rides in a field `event` cannot type. → §2.
7. **The decoder kept an `object` value as its raw JSON text** (`internal/event/decode.go`: `Obj: string(raw)`), whitespace and malformed UTF-8 included. A pretty-printed object from a live source would break a log line (review F6). → D4, §5.6.
8. **The decoder reports unknown keys only as a count** (`Decoded.Unknown`), and for a repeated key the last one wins (P4). The log's line envelope is read with the same decoder (§5.2), so the codebase keeps one JSON reader, and `ts` uses the strict RFC 3339 reader (D4 of S3.1-2).

## 1. The step contract (how one arrival reaches the rules)

Every driver of partitions follows this: the playback driver's consumer now, the runtime's shard in S3.2a-2.

For each message — an event with the advances its arrival produced, or advances alone:
1. **Apply** every advance to every rule that reads the advanced stream (`partition.Rule.Apply`).
2. **Offer** the message's event, if any, to every rule that reads its stream, under its key (`partition.Rule.Offer`). A construct-late result counts for that rule.

Then **Advance** every rule (`partition.Rule.Advance`) — after each message or after a whole chunk; **D1 makes the two the same** (§4.1):
- A walk finalizes only instants below the clock.
- An admitted event's time is at or above the clock when it is offered.
- The clock never falls.
- So no event can ever join an instant that is already finalized, however late the walk runs. ADR-0010 §2's batch-size invariance therefore holds by construction (review F1 resolved).

**MUST: an event's own advances are applied before the event is offered** (the oracle's order: `run_pattern_multi` raises the construct watermark by the event's own stream and, in playback, by the virtual clock, before judging the event). In playback a merged-log event can fall behind another stream's virtual floor, and only this order judges it as the oracle does. In S3.2a-2, an event and its own advances share one mailbox message.

## 2. The table snapshot carrier (owner D1 of S3.1-2; D5)

**`internal/table` gains `Set` and `Snapshot`** (as stubbed):
```go
type Set struct { /* tables []*Table; last *Snapshot */ }
func NewSet(tables []*Table) *Set
func (s *Set) Len() int
func (s *Set) Table(id int) *Table
// Snapshot is every table's current Version; the same *Snapshot until a table publishes a write, so a steady-state
// ingest step allocates nothing. One goroutine only (the router, or the playback driver).
func (s *Set) Snapshot() *Snapshot
type Snapshot struct { /* versions []*Version */ }
func (s *Snapshot) Table(id int) *Version // never nil in range; panics out of range like a slice index
func (s *Snapshot) Len() int
```
- **`Snapshot` holds the Versions it was taken with**; later writes never change it.
- **Cost:** one atomic load per table per event (review F12: fine for a handful of tables; a set-wide generation counter is a later option).

**`event.Event.Tables any`** (as stubbed) holds the `*table.Snapshot` taken at the event's ingest position.
- **MUST:** ingest (the playback driver; S3.2a-2's router) sets it on every event before routing.
- **MAY:** a holder may set it to nil on an event it keeps after the event's release, because a released event's own evaluation is done (review F3: a kept event would otherwise pin a full table copy, since a write copies the whole map). S4.1 and S8 clear it when they capture an event.
- **MUST NOT:** nothing else writes it.

**A timer's snapshot (D5):** a timer sees the tables as at the arrival whose processing fired it. `Rule.Advance(tables)` takes the snapshot of the message that triggered the walk and hands it to `Body.OnTimers`; released events read their own `Tables`.
- In S3.2a-2, every message that can trigger a walk carries the router's current snapshot: an event, another shard's advance, a tick. So the shard count never changes what a timer reads (review F2; §10).
- In playback, `Step.Tables` is that snapshot.

## 3. `internal/partition` (item 2; ADR-0006 §2, §3)

`internal/partition` imports `event` and `clock` only. It is pure: no goroutines, no locks, no I/O. One `Rule` exists per (rule, shard), used by one goroutine. **The signatures are those of the stub**, `internal/partition/partition.go`. v2 changes only contracts, plus one field: `InstanceState` gains `Birth uint64` and `Born bool`.

**`NewRule`:**
- **Refusals:** empty `Inputs`, a `KeyType` of `TInvalid` or `TObject` (D7), a negative `TTL`, and a nil `Body`.
- **`TTL` 0** is accepted. The validator guarantees a keyed rule carries a TTL (E-TTL-001), and `partition` trusts the model (ADR-0006, implementation notes); QA A8.

**`Apply(a)`** reports whether **the rule's construct clock moved**, that is, the clamped min over its inputs (QA A7).

**`Offer(key, e)`:**
- **Key lookup.** It looks the key up by value: one comparable struct holding the type's own field, as `internal/table`'s key does. So `-0.0` and `+0.0` are one key, and the lookup allocates nothing.
- **Bad keys MUST panic:** a null key, a key of another type, or a NaN float key (QA A2). A NaN would make a new instance per event; the decoder never produces one, and the Go API refuses non-finite floats from S3.3. The router checks first, and S3.2a-2's shard boundary recovers a panic.
- **A missing key** gets a new `Instance`: `Clock` from `RuleClock.NewInstance`, `LastActivity` = `math.MinInt64`, `Op` nil.
- **Buffering.** It then calls `Clock.Buffer(e)`. On construct-late it counts `Stats.ConstructLate`. If that instance was created by this call, it frees the instance at once.

**`Advance(tables)`:** calls `RuleClock.Advance` with three callbacks built once in `NewRule`, so there is no closure per call.
- **`OnEvent(ci, e)`** for the instance `in` of `ci`:
  - if `in.Op == nil`, set `in.Op = Body.NewState(in)` (birth);
  - set `in.LastActivity = e.Ts`;
  - if `TTL > 0`, call `ci.Schedule(e.Ts + TTL, clock.TTLPurge, 0)`, saturating (this replaces the key's one TTL timer);
  - then `Body.OnEvent(in, e)`.
- **`OnTimers(ci, fires)`:**
  - **A purge.** When the group is `[TTLPurge]` (it fires alone in its group: a strict, non-discard kind), the purge runs:
    - `Body.OnPurge(in)`, with `in.Op` still set, `in.LastActivity` unchanged, the TTL timer already gone and the body's other timers still pending (QA A4: S8's retractions read them);
    - `ci.CancelAll()`;
    - `in.Op = nil`, `in.LastActivity = math.MinInt64`;
    - count `Stats.Purged`;
    - mark `in` pending free, **once**: a key purged and reborn several times in one walk is listed once (review F10).
  - **Otherwise** → `Body.OnTimers(in, fires, tables)`.
- **`OnDiscard`** → `Body.OnDiscard`.
- **After the walk:** each pending instance that is still unborn and `Clock.Empty()` is freed (`RuleClock.Free`, removed from both maps). One that was reborn leaves the list; one that still holds buffered events stays listed.
- **Why there is no purge test in `OnTimers`.** The due is `LastActivity + TTL`, re-armed on every release, and D1 fires it once the clock is past it, so it fires exactly when `clock − LastActivity > TTL` (SEMANTICS §9).
- **At the end of input (+∞), every resident key purges**, as SEMANTICS §2.1 has all open state expire then (QA A9).

**Per-key marks live in the body's `OpState`** (review F9): the `once` completion mark, rate anchors (S3.2b) and the batch-id dedup set (ADR-0014 §5). So a purge clears them (`Op = nil`) and `Walk` yields them. ADR-0006 §2's `Instance` fields `RateAnchors` and `OnceDone` move there, and the ADR is annotated at G1.

**`Walk(fn)`**, a state-registry holder:
- **First,** component `"clock"`, key `""`: a `clock.RuleState` value (§4.2).
- **Then, for each instance in `Ordinal` order:** component `"instance"`, key = the Ordinal in decimal, and an `InstanceState` **value** (QA A3) holding `Key`, `LastActivity`, `Birth` and `Born` (`clock.Instance.Birth`), `Events` (`Clock.Events`) and `Timers` (`Clock.Timers`).
- **Then** the instance's `Op.Walk` parts.

`Instances` also goes in `Ordinal` order. A Go map's order MUST NOT reach any output.

**`CheckTimers`:** for every instance, its pending timers MUST equal, as a multiset of `(Kind, Due, Ref)`:
- a born instance (`Op != nil`): `{TTLPurge, LastActivity + TTL, 0}` when `TTL > 0`, plus `Op.Timers()`;
- an unborn one: nothing.

Its error begins `partition: instance <ordinal>: ` and names the difference (QA A6).

**MUST / MUST NOT:**
- A `Body` MUST NOT schedule `TTLPurge` (`CheckTimers` reports it), and MUST NOT keep an instance's clock past `OnPurge`.
- `Offer` + `Advance` on a resident, born key, with a body that allocates nothing, MUST allocate nothing (ADR-0010 §1).

**The pseudo-key.** A key-less rule (ADR-0006 §4) is a `Rule` with `TTL` 0 whose router always offers key `""`. It has one instance and is never purged.

## 4. `internal/clock` changes (before the S3.3 freeze)

### 4.1 D1: finalize an instant only once the clock is past it

**The finalization rule:**
- **Events:** `RuleClock.Advance` releases an event only when `Ts < clock`.
- **Timers:** it fires a timer of **every** kind only when `due < clock`.
- **The two exceptions:** a clock at −∞ releases nothing; a clock at +∞ (end of input) is past every instant.

**Within one instant the order is unchanged:**
1. events, in `event.Compare` order across keys;
2. then the inclusive kinds (Completion, CronClose, DelayDue, BatchClose), by (key birth, kind, ref, schedule order);
3. then the strict kinds, by (key birth, kind, ref, schedule order): each key's TTLPurge before its Discard, keys in birth order (the executor's order; QA B2, G4). Discard goes through `OnDiscard` only.

"Inclusive" and "strict" now name only this order within an instant, not when a kind fires. A timer scheduled by a callback joins the walk in order, as before.

**The seal (S3.1-1's D7) is retired:**
- `Buffer(e)` is construct-late iff `e.Ts < clock`. Under D1 no instant at or above the clock has been finalized, so nothing else is needed.
- The seal fields go, and `Empty()` is "no buffered event and no timer".
- The seal's tests are rewritten to the D1 outcome: an event at the instant of a key's pending timer, arriving after the clock reached that instant, is admitted and released before the timer.

**For operator authors** (S4.1, S8, S9; ADR-0004 annotated): a timer due at `t` fires once the clock is past `t`.
- A SEMANTICS deadline `t_first + D` (inclusive window, `within`, `for`), a TTL `t_last + TTL`, and a discard `t_first + within` are scheduled at exactly that due. An event at the deadline is then processed first, and defeats or extends as SPEC §6.4 says.
- A condition "once the clock reaches `e`" (a half-open window's end) schedules `e − 1`.

**The oracle changes first.** In `docs/language/checks/executor.py`, `drain` releases an instant `< wm`, walks timers `< wm`, and at `wm = +∞` everything; `sealed_for` goes.

The three checks that change get new hand-derived expectations, red first:
- **"2.1 · seal"** becomes "an event at the instant of a pending timer, arriving after the clock reached it, is processed before the timer";
- **"6.4 · absence at the boundary"**: the alert releases when the watermark passes 10:05;
- **"2.1 · quiet · lagging source"**: the release waits for the floor to pass the instant.

Then the clock's tests are revised to D1, by hand, before the clock changes.

**Cost, accepted by the owner:** the newest instant waits for the clock to pass it — the next event on every input, or a quiet stream's idle tick. A match that completes on an event on a quiet stream waits up to `idle after`; timer-driven alerts are unaffected.

### 4.2 Views (the state seam; review F4)

```go
// Timers appends the key's pending timers in walk order.
func (in *Instance) Timers(dst []Timer) []Timer
// Events appends the key's buffered events in event.Compare order (cold path: scans the rule clock's buffer).
func (in *Instance) Events(dst []event.Event) []event.Event
// Birth is the key's place in the walk's order across keys at one instant; born is false before its first released
// event or timer, and again after CancelAll.
func (in *Instance) Birth() (birth uint64, born bool)
// RuleState is a rule clock's own state.
type RuleState struct {
	Inputs        []int
	Contributions []int64
	Clock         int64
}
func (rc *RuleClock) State() RuleState
// Walk yields each stream's state (component "stream", key its name): admission watermark, max and last event times,
// last arrival, floors, punctuation, contribution, ended.
func (r *StreamRegistry) Walk(fn func(component, key string, state any))
```

### 4.3 Allocation, and the stop drain

- **A replacement allocates nothing.** `Schedule` replacing an existing (kind, ref) — a TTL re-arm included — updates that timer in place: a new due, a new schedule order, and the heap fixed. It MUST NOT allocate. It is ordered as a fresh schedule (QA A5).
- **The stop drain:**
  ```go
  // Drain raises every stream's contribution to one past the largest event time admitted on any of the app's streams
  // (ADR-0012 §3's Stop, under D1: that instant is complete once the sources are stopped, so it finalizes; any deadline
  // after it stays unfired). Nothing admitted: no advance.
  func (r *StreamRegistry) Drain(buf []WMAdvance) []WMAdvance
  ```
  The recording's stop record (§5.2) and S3.3's live `Stop` both use it. ADR-0012 §3 is annotated: "to the max observed event time" becomes "past it".

## 5. `internal/playback`'s format layer (item 1; ADR-0005 §3)

`internal/playback` imports `event`, `clock` and `table` (and `partition` in its tests), plus `bufio`, `io`, `strconv`, `time` (formatting and parsing only), `unicode/utf8` and `math`. **The signatures are those of the stub**, with the v2 changes named below.

### 5.1 The app the log is read against

As stubbed: `App{Streams []Stream; Tables []Table}`.
- **`Stream`:** `{Schema, Delay, IdleAfter, Derived}`; a stream's index is its stream id. `IdleAfter` is the stream's `idle after D` (0 when it declares none); a recording's ticks use it.
- **`Table`:** `{Name, Attrs, Key}`; a table's index is its table id.

Names must be unique among streams and among tables, and `Delay ≥ 0`. S3.3's planner builds the `App`; tests build it by hand.

### 5.2 The log format (JSON Lines; one record per line)

A log is a **fixture** or a **recording** (D3).

**A fixture** — hand-written tests, expect blocks, generated corpora:

| Record | Form | Notes |
|---|---|---|
| event | `{"stream":"S","attrs":{…}}`, `"batch":"b1"` optional, and `"ts":"<RFC 3339>"` **only** on a stream with no time attribute (D2) | `attrs` is the payload; an event-time stream's time is its time attribute. |
| table | `{"table":"T","op":"put","row":{…}}` or `{"table":"T","op":"delete","row":{<key columns>}}` | Applied at its position (SEMANTICS §4, §10). |
| batch end | `{"batch_end":"b1","stream":"S"}` | A source batch boundary (SEMANTICS §7); nothing consumes it before S8. |

Time comes from the virtual clock (§5.3), and **the end of the log raises every contribution to +∞**: every open state fires or expires (SEMANTICS §2.1).

**A recording** — a live session, written by S3.2b's recorder — starts with a header and has three more forms:

| Record | Form | Notes |
|---|---|---|
| header | `{"recording":{"start":"<RFC 3339>"}}` | **Must be the first record.** The deployment's start (wall time): a stream with no event yet uses it (owner D5 of S3.1-1). |
| event | as a fixture's, plus `"arrival":"<RFC 3339>"` on an event-time stream | The intake's wall stamp. On a `time arrival` stream, `ts` *is* the arrival stamp, and `arrival` is not written. |
| tick | `{"tick":"<RFC 3339>"}` | A live idle tick at that wall time (owner D3, D5 of S3.1-1). |
| stop | `{"stop":true}` | The session ended by `Stop`: the drain (§4.3). Must be the last record. |

- In a recording, arrival and tick times never decrease in file order (the intake stamps them in order).
- A recording without a stop record ended without `Stop`. Its end adds nothing: no drain and no +∞, because MVP stop is graceful or nothing (ADR-0012 §3).
- A fixture with an `arrival`, `tick` or `stop` record, or a recording header anywhere but first, is a fault.

**No other record kind exists in v0.1.** Derived-stream punctuations are Phase 2; source punctuation is reserved (SEMANTICS §2.1).

**Lines:**
- A line that is empty or holds only JSON whitespace is skipped.
- Line numbers count every line from 1; CRLF line endings are accepted.
- A line longer than 16 MiB is a fault.
- File order is arrival order.

**Reading a line:**
- **The envelope.** It is decoded by an `event.Decoder` over the envelope schema: every key optional; `attrs`, `row` and `recording` as `object`; `ts`, `arrival` and `tick` as `timestamp`; `stop` as `bool`; the rest as `string`. An explicit `null` reads as absent.
- **The kind.** It is chosen by the first present key in this order: `recording`, `table`, `batch_end`, `tick`, `stop`, `stream`.
- **Every key** the kind does not allow is a fault; so is every key it requires that is missing.

**Faults (D6):** the first fault stops the reader at its line, with a `*LoadError`; the steps already delivered stand. A fault is any of:
- **in the envelope:** any fault the envelope decoder reports, a wrong-typed field included (`"stream":7`, a `ts` that is not RFC 3339; review F7); an unknown key (`Decoded.Unknown > 0`); a record with none of the kind keys; a forbidden or missing key;
- **in naming:** an unknown stream or table; a derived stream;
- **in time:** `ts` against D2; a missing `arrival` in a recording; a decreasing arrival or tick;
- **in a table record:** an `op` other than `put` or `delete`; a row that fails its schema (put: every attribute; delete: exactly the key columns, in a key-only schema); a null value in a key column, checked here so that the table never refuses a write (review F5);
- **in the payload:** `attrs` that fail the stream's schema. The `LoadError` then carries the `*event.DecodeError`. Live ingest keeps treating this as an event error, dropped and counted; a replay stops (D6, ≠ R).
- **in order:** a recording rule broken.

```go
type RecordKind uint8
const ( RecEvent RecordKind = iota + 1; RecTable; RecBatchEnd; RecHeader; RecTick; RecStop ) // v2 adds the last three

type Record struct {
	Line    int
	Kind    RecordKind
	Event   event.Event // RecEvent: StreamID, Ts, Values, BatchID (Seq and Tables are the driver's)
	Arrival int64       // RecEvent in a recording: the intake stamp (a `time arrival` stream: = Event.Ts)
	Table   int         // RecTable
	Op      TableOp
	Row     []event.Value
	Stream  int         // RecBatchEnd
	Batch   string
	Time    int64       // RecHeader: the start; RecTick: the tick's wall time
}                       // v2 drops Err: a payload fault is a LoadError (D6)

type LoadError struct {
	Line   int
	Msg    string
	Decode *event.DecodeError // a payload fault's cause; nil otherwise
}
```

Every slice in a `Record` is fresh: the clock and later the operators keep events, so a `Values` slice MUST NOT be reused (ADR-0010 §4).

### 5.3 The driver

```go
type StepKind uint8
const (
	StepEvent StepKind = iota + 1 // admitted: §1 with Advances
	StepHeader                    // a recording's header: recording mode from here
	StepStreamLate                // behind its stream's admission watermark: counted; nothing moves (not a fault)
	StepTable
	StepBatchEnd
	StepTick                      // a recording's tick: Advances from ObserveTick
	StepStop                      // a recording's stop: Advances from Drain
	StepEnd                       // a fixture: Advances raise everything to +∞; a recording: no Advances
)                                 // v2 drops StepErrored (D6)

// Step gains `Time int64`: StepHeader's start, StepTick's wall time (v2).
func NewDriver(app *App) (*Driver, error)
func (d *Driver) Step(rec Record) Step // v2: cannot fail; the Reader checks everything first
func (d *Driver) End() Step
func (d *Driver) Tables() *table.Set
func (d *Driver) Registry() *clock.StreamRegistry
func (d *Driver) Walk(fn func(component, key string, state any)) // v2: a holder: component "seq", key the stream name
func Replay(r *Reader, d *Driver, fn func(*Step) error) error    // streams: a step, then fn, line by line; stops at the first error
```

**Starting.** The driver starts in fixture mode with start 0. A header record switches it to recording mode and builds the registry with the header's start.

**An event record, in this order (MUST):**
1. **Stamp `Seq`** per stream, from 1, in file order. Every event that decodes gets one, a stream-late event included, as the live router stamps at intake (ADR-0006 §4). The order is the oracle's; only the numbers differ. The oracle numbers only buffered events, so oracle comparisons use line indexes (review R5).
2. **Set `Tables`** = `set.Snapshot()`.
3. **Observe the event.** `late, a := reg.ObserveEvent(sid, Ts, arrival)`: arrival = the record's `Arrival` in a recording, `Ts` in a fixture.
4. **If late** → `StepStreamLate`; nothing else moves.
5. **Otherwise** → `StepEvent` with `Advances`:
   - `[a if a.Moved]`;
   - then, in a fixture only, `vnow = max(vnow, Ts)` and `reg.ObserveVirtual(vnow)` appended.

**The other records:**
- **A table record** applies `Upsert` or `Delete`; the step's `Tables` is the new snapshot.
- **A tick** → `reg.ObserveTick(time)`.
- **A stop** → `reg.Drain`.
- **`End`** → in a fixture `reg.EndOfInput`; in a recording, nothing.

**The registry** is written by the driver only (ADR-0004 §1). `Step.Tables` is the snapshot current after the step: what this step's timers read (D5).

**Seq's home.** The Seq counters are durable state (ADR-0006 §2(b)), and the driver yields them as a holder (review F4). S3.2a-2 moves Seq, the snapshot, `ObserveEvent` and the virtual floor into one single-threaded ingest step that the router and the driver both call (review R4; §10).

### 5.4 The canonical output encoder

As stubbed: `AppendValue`, `AppendEvent`.
- **The envelope.** Keys are sorted bytewise and there is no whitespace. Every declared attribute appears, a null as `null`.
- **Optional keys.** `batch` appears only when set; `ts` exactly when the schema has no time attribute (D2). An output line never carries `arrival`: that is the recorder's (S3.2b).
- **`text_invisibles`** is S3.2b's.

**The values:**

| Type | Encoded as |
|---|---|
| string | `"` escaped `\"`, `\` escaped `\\`, then `\b \f \n \r \t` in short form, every other byte below 0x20 as `\u00XX` (lowercase hex), U+2028 and U+2029 as ` ` ` `; DEL and every other rune raw; malformed UTF-8 → U+FFFD. This is `encoding/json` with HTML escaping off on Go 1.27.3, kept as the test's oracle (review F11). |
| int | decimal |
| float | `strconv.FormatFloat(f, 'g', -1, 64)` (`-0` kept); NaN or ±Inf → an error naming the attribute |
| bool | `true` or `false` |
| timestamp | RFC 3339 UTC, exactly three fraction digits (`2026-01-01T10:00:00.000Z`); outside the years 0000–9999 → an error |
| duration | integer milliseconds (P2) |
| object | its stored text (D4: already compact, valid UTF-8) |

**The round trip (a test):** for every event a `Reader` accepts, `AppendEvent` gives a line the `Reader` reads back to equal `Values`, `Ts` and `BatchID`.

### 5.5 The canonical trace

As stubbed: `NewTrace`, `Step`, `Released`, `ConstructLate`, `Fired`, `Discarded`, `Purged`, `Note`, `Bytes`.
- **The lines.** One canonical JSON line per entry, in the order recorded.
- **The keys.** Every entry carries `"at"` (the line of the step whose processing produced it; for `end`, the last line + 1) and `"k"`:

| `k` | Other keys |
|---|---|
| `stream_late` | `stream`, `seq`, `ts` |
| `table` | `table`, `op` |
| `batch_end` | `stream`, `batch` |
| `recording` | `start` |
| `tick` | `time` |
| `stop` | — |
| `end` | — |
| `event` (released) | `rule`, `key`, `stream`, `seq`, `ts` |
| `construct_late` | `rule`, `key`, `stream`, `seq`, `ts` |
| `timer` | `rule`, `key`, `kind`, `due`, `ref` |
| `discard` | `rule`, `key`, `ref` |
| `purge` | `rule`, `key`, `due` |
| `note` | `rule`, `key`, `name`, `value` |

- **Encoding.** `key`, `value` and the times are canonical values (§5.4); a timer kind is its lowercase name.
- **The late counters** are the `stream_late` and `construct_late` entries' counts.
- **S3.2b's replay through the runtime** writes the same entries.

### 5.6 D4: the decoder cleans `object` values (`internal/event/decode.go`)

- **Compact form.** An `object` value is stored as its JSON text in compact form: every whitespace byte outside a string is removed, and member order and every token's text are kept. In particular, a string's escapes stay as written.
- **Valid UTF-8.** Any malformed UTF-8 inside a string becomes U+FFFD, as P8 does for `string` attributes.
- **What the change touches.** Only the `object` case changes. S3.1-2's differential oracle for `object` (the original text) becomes `json.Compact` of the text, with malformed UTF-8 replaced. QA's decoder tests change first.

## 6. Tests (QA, written first; each fails for its stated reason before the code changes)

**Wave A** (two agents):
- **D1:** the oracle's three checks, then the clock's tests, revised by hand.
- **The partition tests:** written against v1, revised to v2.

**Wave B:** playback and the decoder. Every timing claim states the instant, derived by hand in a comment. No test sleeps or reads the wall clock.

| Exit / rule | Test | Proof |
|---|---|---|
| **D1** (oracle) | `test_semantics.py`: the three checks of §4.1, plus a new one — two events at one millisecond from two streams, arriving in reverse declared order while the clock sits on that millisecond, are processed in declared order | red on the old drain, green on D1's |
| **D1** (clock) | `internal/clock`: the 20 tests that assume the old timing, revised; the seal tests rewritten (§4.1); a batch-invariance test: one sequence of advances and events walked after every message, and walked once at the end, gives the same callbacks | red on the old `nextEvent` / `nextTimer` |
| **Exit 4** | `TestOneTTLTimerPerInstance` (`internal/partition`) | Exactly one pending `TTLPurge` per key after every walk, due = `LastActivity + TTL`; a re-arm allocates 0. |
| TTL, keys, invariant, order | `internal/partition` (QA, revised to v2) | §3, including F10's repeated purge in one walk, a NaN key panicking, `InstanceState` as a value, the `"clock"` component, and `OnTimers` receiving `Advance`'s tables |
| **Exit 1** | `TestTraceDeterministic`, `TestTracePlantedMapOrderIsRed` (`internal/playback`) | A fixture with three streams, several keys, out-of-order and late events, table records and batch ends, replayed twice → byte-identical traces. The proof: a body that writes its notes by ranging over a Go map of ≥ 16 entries gives two different traces within 20 replays. |
| **Exit 2** | `TestTableRecordFromNextEvent` | E1 (`ts` 10:00, `d` 1 min), then `put {x}`, then E2 at 10:02, which releases E1: E1 reads `x not in T`; E2 reads `x in T`; a timer fired in E2's walk reads E2's snapshot (D5). The same for `delete`. |
| Recording | `internal/playback` | The S3.1-1 tick cases replayed from a hand-written recording give the clock arithmetic the live harness gives. A stop record drains to one past the max event time: a deadline at that time fires, and a later one does not. A recording without stop adds nothing at the end. Every recording fault. |
| Loader | `internal/playback` | One case per fault of §5.2 (its line, its message); a payload fault stops the replay with its `DecodeError` (D6); steps before a fault are delivered and the fault is the last thing `Replay` returns; `Seq` per stream with a stream-late event counted; whitespace lines skipped; CRLF accepted; a BOM is a fault at line 1. |
| Oracle fixtures | `internal/playback` (review F8) | (1) A fixed-seed corpus from `executor.run_pattern_multi` (playback, D1 drain) with a spec that opens no partials, against a body that schedules nothing: stream-late and construct-late line indexes match. (2) The oracle pattern `A, not X for D` (X never occurs) against a body that schedules `Completion` at `e.Ts + D` per released A: completions and lateness match. Both are committed fixtures. |
| Encoder | `internal/playback` | Strings against `encoding/json` (HTML off) on generated strings, malformed UTF-8 included; the §5.4 table literally; floats (`-0`, `1e+21`, the smallest subnormal); NaN and ±Inf errors; timestamps at year 0 and 9999, and one outside; the round trip. |
| D4 | `internal/event` | A pretty-printed object with newlines and tabs is stored compact; malformed UTF-8 inside is U+FFFD; a string's escapes are kept; the differential's object oracle is updated. |
| Snapshot, views | `internal/table`, `internal/clock` (QA, written) | `Set.Snapshot` reuse with 0 allocations; `Timers` and `Events` orders; a replacement `Schedule` allocates 0. Plus `Birth` and `RuleClock.State`, and `Drain`. |

## 7. Decisions

D1–D7 are the owner's, as recorded in [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-05--s32a-1s-design-after-the-architecture-review). D6 differs from the recommendation (≠ R).

## 8. Size

- **Carded:** 5.5 points.
- **Added by the owner's decisions:** about 3.5 points — D1 (the clock, the oracle, the revised tests: ~2), D3 (the recording records: ~1) and D4 (the decoder: ~0.5).
- **Total:** about 9 points, under the 13-point cap.

## 9. Review and test-gap dispositions

| # | Severity | Disposition |
|---|---|---|
| F1 | blocker | Resolved by D1 (§1, §4.1): walk timing cannot change a result. ADR-0010 §2 and ADR-0004 §2 annotated at G1. |
| F2 | should-fix | D5 (§2); S3.2a-2 forward (§10) with a shard-count-invariance test. |
| F3 | should-fix | Fixed (§2): a holder MAY clear `Tables` after release; forward to S4.1 and S8. |
| F4 | should-fix | Fixed (§3, §4.2, §5.3): birth order, the rule clock's state and the Seq counters are yielded. The seal is gone (D1). |
| F5 | should-fix | Superseded by D6 (§5.2): replay streams and stops at the first fault, and delivered steps stand. The null-key check moves into the reader, so `Driver.Step` cannot fail. |
| F6 | should-fix | D4 (§5.6). |
| F7 | should-fix | Fixed (§5.2): every envelope decoder fault is a fault. |
| F8 | should-fix | Fixed (§6): two oracle fixtures. |
| F9 | should-fix | Fixed (§3): per-key marks live in `OpState`; ADR-0006 §2 annotated at G1. |
| F10 | later | Fixed (§3): pending free once; a test. |
| F11 | later | Fixed (§5.4): the escape table is written out. |
| F12 | later | Noted (§2); no change. |
| F13 | later | Moot: the seal is retired (D1). |
| A1 | QA | Moot (D1). |
| A2 | QA | A NaN key panics (§3). |
| A3 | QA | `InstanceState` is yielded as a value. |
| A4 | QA | `OnPurge`'s view is stated (§3). |
| A5 | QA | As tested: a replacement is ordered as a fresh schedule. |
| A6 | QA | Fixed error prefix (§3). |
| A7 | QA | `Apply` reports the rule clock moving. |
| A8 | QA | TTL 0 is accepted; the validator guarantees keyed rules carry a TTL. |
| A9 | QA | Stated: at +∞ every resident key purges. |
| B1 | QA (v2) | Accepted: an Ordinal is a creation label. A key freed between two walks and seen again gets a new one, so `Walk`'s instance keys may differ with walk frequency while every callback is the same. A replay's walk schedule is fixed, so its output is deterministic. |
| B2, G4 | QA | Fixed in §4.1's wording: the strict kinds go per key in birth order, a key's purge before its discard. |
| B3 | QA | As tested: a refused key leaves no instance. |
| B4 | QA | As tested: a fresh rule's first-born key has Birth 0. |
| B5 | QA | `Drain` raises every stream, derived ones included, as `EndOfInput` does. Nothing arrives after it: a recording's stop is its last record, and a live `Stop` stops the sources first. |
| B6 | QA | Confirmed: under D1 a key is unborn, and holds no TTL timer, until its first event is released. Exit 4's "one TTL timer per instance" holds for every born instance after every walk. |
| G1 | QA (D1) | Forwarded to S9.1 and S9.2: the oracle's window functions (`cron`, `delay`) still finalize at `wm ≥ due`. Cron's close is unchanged if it schedules `e − 1`; whether delay re-times at `wm ≥ t + D` or `wm > t + D` is S9.2's design, oracle first. |
| G2 | QA (D1) | As tested: a due of +∞ fires at +∞. |
| G3, G5, G6 | QA (D1) | Noted: GranularityRandom's shared checks follow D1; `Empty()`'s doc is changed; SEMANTICS and ADR-0004 carry dated annotations, never rewrites (ground rule 4). |

## 10. Forwarded to S3.2a-2

- **The router:**
  - keys read from the attribute;
  - null-key, wrong-type and NaN keys as per-rule event errors;
  - `Tables` set from `table.Set.Snapshot`;
  - an event and its own advances in one message;
  - **every message that can trigger a walk carries the router's snapshot (D5)**, proven by a test that a table-reading timer gives the same result with 1 and with 4 shards.
- **One single-threaded ingest step** shared by the router and the playback driver (review R4): Seq, snapshot, `ObserveEvent`, the virtual floor or ticks.
- **Chunking** is free under D1: a shard may walk per message or per chunk.
