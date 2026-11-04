# S3.2a-1 — design note: the replay format layer and partitions (v1)

S3.2a's items 1–2 ([card](sprint-plan.md#s32a--the-replay-format-and-the-runtime-core)); exit lines 1, 2 and 4; the forwards from S3.1-2 (the table-snapshot carrier, owner D1; the timer snapshot, review F15; the encoder's malformed UTF-8, review F11). The owner's plan-check answers: [sprint-plan §5, "S3.2a's plan check"](sprint-plan.md#owner-decisions-2026-11-05--s32as-plan-check). Routine (owner): QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it.

v1 is for the architecture review and for QA's tests. §7 lists the questions for the owner after the review, each with a recommendation; the tests follow the recommendation until the owner answers. The stub commit carries every signature below with panicking bodies. MUST and MUST NOT below are the build's contract.

## 0. What the code says (read first)

1. **A partition key is an attribute, not an expression.** `model.PartitionKey` is `{Attr, Stream}` (`internal/model/model.go`); one block keys every stream on one type (E-PART-003, `internal/validate/bounds.go`). The router (S3.2a-2) reads the key; `partition` receives an `event.Value` (plan-check reading 1).
2. **The validator accepts an `object` partition key**, and object equality is not defined (SPEC §5.10: objects take `is null` only). → §7 O5.
3. **`clock.Instance.Schedule` allocates a new timer on every call, a replacement included** (`internal/clock/rule.go`, `Schedule`). The TTL is re-armed on every released event (ADR-0004 §5 as annotated), so as written each event allocates, against ADR-0010 §1's zero-allocation budget. → §4: a replacement reuses the timer.
4. **The clock exposes no view of a key's buffered events or pending timers.** The state registry and the timer-derivability invariant (ADR-0006 §2(b), (c)) need one. → §4.
5. **The order of one arrival is fixed by the oracle.** In `docs/language/checks/executor.py` (`run_pattern_multi`, playback mode), an event first raises the construct watermark by its own stream's watermark and the virtual clock, is then judged construct-late or buffered, and only then is the buffer drained. Walking before the event is buffered would fire the key's timers at the event's instant first and seal it (owner D7 of S3.1-1), so the event would be wrongly construct-late. → §1, the step contract.
6. **`event` imports nothing**, so the per-event table snapshot rides in a field `event` cannot type (plan-check reading 5). → §2.
7. **The decoder keeps an `object` value as its raw JSON text**, `Obj: string(raw)` (`internal/event/decode.go`), malformed UTF-8 and whitespace included; a string value's malformed UTF-8 becomes U+FFFD (P8). → §7 O3.
8. **The decoder reports unknown keys only as a count** (`Decoded.Unknown`); repeated keys: the last wins (P4). The log's line envelope is read with the same decoder (§5.2), so the codebase keeps one JSON reader, and the strict RFC 3339 reader (D4) reads `ts`.

## 1. The step contract (how one arrival reaches the rules)

This is the one order every driver of partitions follows: the playback driver's consumer now, the runtime's shard in S3.2a-2.

For an admitted event `e` on stream `s`, with the advances its arrival produced (its own stream's, then the virtual floors' in playback):
1. **Apply** every advance to every rule that reads the advanced stream (`partition.Rule.Apply`), without walking.
2. **Offer** `e` to every rule that reads `s`, under its key (`partition.Rule.Offer`). A construct-late result is counted for that rule.
3. **Advance** every rule once (`partition.Rule.Advance`): released events, then timers, in the clock's walk.

An advance with no event (playback's end; in live, a tick or another shard's event) is steps 1 and 3.

- An event's own advances MUST travel with it to its shard. In S3.2a-2 they share one mailbox message, and the shard MUST NOT walk between applying them and buffering the event.
- ADR-0004 §3 (advances fan out to every shard) still holds for the other shards.
- This annotates ADR-0004 §3 at G1.

**Why:** the oracle's order (§0.5). A walk between steps 1 and 2 would seal the key's instant before its own event arrives.

## 2. The table snapshot carrier (owner D1 of S3.1-2; review F15)

**`internal/table` gains `Set` and `Snapshot`:**
```go
// Set is an app's tables in declaration order (a table's index is its id: expr.Scope.Table's id).
type Set struct { /* tables []*Table; last *Snapshot */ }

func NewSet(tables []*Table) *Set
func (s *Set) Len() int
func (s *Set) Table(id int) *Table

// Snapshot is every table's current Version. When no table has published a Version since the last call, it returns
// the same *Snapshot, so a steady-state ingest step allocates nothing. One goroutine only (the router, or the
// playback driver): it takes no lock.
func (s *Set) Snapshot() *Snapshot

// Snapshot is one immutable read view of an app's tables.
type Snapshot struct { /* versions []*Version */ }

func (s *Snapshot) Table(id int) *Version // never nil for 0 ≤ id < Len; panics out of range like a slice index
func (s *Snapshot) Len() int
```

**`event.Event` gains one field:**
```go
	// Tables is the app's table snapshot at this event's ingest position (*table.Snapshot; SEMANTICS §4 as annotated,
	// owner D1 of S3.1-2). Set by ingest only, before the event is routed; never written after. Opaque here: event
	// imports nothing. The runtime's expr.Row reads it.
	Tables any
```
- Storing a pointer in an `any` does not allocate.
- MUST: ingest (the playback driver; S3.2a-2's router) sets `Tables` on every event it routes.
- MUST NOT: anything else writes it.
- `event.Compare` ignores it.

**A timer's snapshot (F15; §7 O4):** a timer that fires in the walk of step 3 reads the snapshot current at the arrival that triggered the walk. `Rule.Advance` takes that snapshot as an argument and hands it to `Body.OnTimers`. Released events read their own `Tables`.

## 3. `internal/partition` (item 2; ADR-0006 §2, §3)

`internal/partition` imports `event` and `clock` only. It is pure: no goroutines, no locks, no I/O. One `Rule` per (rule, shard), used by one goroutine.

```go
package partition

// OpState is a rule body's state for one key (ADR-0006 §2, extended for the derivability invariant).
type OpState interface {
	Walk(fn func(component, key string, state any)) // the Phase-2 snapshot seam
	Timers(dst []clock.Timer) []clock.Timer         // every timer this state implies, appended, any order
}

// Body is a rule body's per-key logic: patterns (S4.1), windows (S8), test doubles now.
type Body interface {
	NewState(in *Instance) OpState                    // at the key's birth: its first released event, and again after a purge
	OnEvent(in *Instance, e event.Event)              // a released event, after LastActivity and the TTL re-arm
	OnTimers(in *Instance, fires []clock.Timer, tables any) // inclusive timers; never TTLPurge
	OnDiscard(in *Instance, refs []uint64)
	OnPurge(in *Instance)                             // just before a TTL purge resets the key (S8's retractions)
}

// Instance is one key of one rule on one shard.
type Instance struct {
	Key          event.Value     // the first event's key value
	Clock        *clock.Instance // the key's release buffer and timers
	Op           OpState         // nil until birth, and from a purge until rebirth
	LastActivity int64           // the event time of the key's last released event; math.MinInt64 before any
}

// Config describes one rule to its partitions.
type Config struct {
	Inputs  []int      // the stream ids the rule reads
	KeyType event.Type // the keys' one type; TString for a key-less rule's pseudo-key ""
	TTL     int64      // ms, > 0; 0 only for a key-less rule, which is never purged
	Body    Body
}

// Stats are a rule's plain counters on its shard; the runtime sums them on scrape.
type Stats struct{ ConstructLate, Purged uint64 }

type Rule struct{ /* clock *clock.RuleClock; byKey map[keyT]*Instance; byClock map[*clock.Instance]*Instance; pending []*Instance; ttl; body; stats */ }

func NewRule(cfg Config) (*Rule, error)
func (r *Rule) Apply(a clock.WMAdvance) (advanced bool)
func (r *Rule) Offer(key event.Value, e event.Event) (constructLate bool)
func (r *Rule) Advance(tables any)
func (r *Rule) Clock() int64
func (r *Rule) Len() int                                   // resident instances
func (r *Rule) Instances(fn func(in *Instance))            // in Ordinal order
func (r *Rule) Walk(fn func(component, key string, state any)) // a state-registry holder
func (r *Rule) CheckTimers() error                         // the derivability invariant
func (r *Rule) Stats() Stats

// Holder is anything with durable state (ADR-0006 §2(b)).
type Holder interface{ Walk(fn func(component, key string, state any)) }

// Registry lists an app's state holders in registration order.
type Registry struct{ /* names []string; holders []Holder */ }

func (g *Registry) Register(name string, h Holder) // a duplicate name panics
func (g *Registry) Walk(fn func(holder, component, key string, state any))

// InstanceState is what Rule.Walk yields for one instance (component "instance", key its Ordinal in decimal),
// before the instance's OpState parts.
type InstanceState struct {
	Key          event.Value
	LastActivity int64
	Events       []event.Event // buffered, in event.Compare order
	Timers       []clock.Timer // pending, in walk order
}
```

**`NewRule` rejects** an empty `Inputs`, a `KeyType` of `TInvalid` or `TObject` (§0.2, §7 O5), a negative `TTL` and a nil `Body`.

**`Offer`:**
- **Key lookup.** It looks the key up by value: one comparable struct holding the type's own field, as `internal/table`'s key does. So `-0.0` and `+0.0` are one key, and the lookup allocates nothing.
- **Bad keys.** It MUST panic on a null key or a key of another type: the router checks both first (SEMANTICS §9: a null key is a per-rule event error), and S3.2a-2's shard boundary recovers a panic.
- **A missing key** gets a new `Instance`: `Clock` from `RuleClock.NewInstance`, `LastActivity` = `math.MinInt64`, `Op` nil.
- **Buffering.** It then calls `Clock.Buffer(e)`. On construct-late it counts `Stats.ConstructLate`. If that instance was created by this call, it frees the instance at once, so a stray key holds no memory.

**`Advance(tables)`:** calls `RuleClock.Advance` with three callbacks built once in `NewRule` (no closure per call):
- **`OnEvent(ci, e)`** for the instance `in` of `ci`:
  - if `in.Op == nil`, set `in.Op = Body.NewState(in)` (birth);
  - set `in.LastActivity = e.Ts`;
  - if `TTL > 0`, call `ci.Schedule(e.Ts + TTL, clock.TTLPurge, 0)`, saturating at `MaxInt64` (this replaces the key's one TTL timer);
  - then `Body.OnEvent(in, e)`.
- **`OnTimers(ci, fires)`:**
  - **A TTL purge.** `TTLPurge` is the only strict kind that `OnTimers` receives, and it fires alone in its group. When the group is `[TTLPurge]`, the purge runs:
    - `Body.OnPurge(in)`;
    - `ci.CancelAll()`;
    - `in.Op = nil`, `in.LastActivity = math.MinInt64`;
    - count `Stats.Purged`;
    - append `in` to `pending`.
  - **Otherwise** → `Body.OnTimers(in, fires, tables)`.
- **`OnDiscard`** → `Body.OnDiscard`.

**After the walk, freeing:**
- Each pending instance that is still unborn and has `Clock.Empty()` is freed: `RuleClock.Free`, then removed from both maps.
- A pending instance that was reborn in the walk leaves `pending`.
- Any other pending instance stays pending: it still holds buffered events, or an unpassed seal.

The purge test is not repeated in `OnTimers`. The timer's due is `LastActivity + TTL`, it is strict, and it is re-armed on every release, so it fires exactly when `clock − LastActivity > TTL` (SEMANTICS §9).

**MUST / MUST NOT:**
- A `Body` MUST NOT schedule `TTLPurge`; `CheckTimers` reports one.
- A `Body` MUST NOT keep a `*clock.Instance` past `OnPurge`.
- `Instances` and `Walk` MUST go in `Ordinal` order. A Go map's order never reaches any output (exit 1).
- `Offer` and `Advance` on a resident, born key with a body that allocates nothing MUST allocate nothing (`testing.AllocsPerRun` = 0; ADR-0010 §1).

**`CheckTimers`** checks every instance. Its pending timers (`Clock.Timers`) MUST equal, as a multiset of `(Kind, Due, Ref)`:
- a born instance (`Op != nil`): `{TTLPurge, LastActivity + TTL, 0}` when `TTL > 0`, plus `Op.Timers()`;
- an unborn one: nothing.

It returns the first difference, naming the instance's ordinal. Tests in this sprint and every later operator sprint call it after each step.

**The pseudo-key.** A key-less rule (ADR-0006 §4) is a `Rule` with `TTL` 0 whose router always offers key `""`. It has one instance and is never purged.

## 4. `internal/clock` additions (before the S3.3 freeze)

```go
// Timers appends the key's pending timers in walk order.
func (in *Instance) Timers(dst []Timer) []Timer
// Events appends the key's buffered events in event.Compare order.
func (in *Instance) Events(dst []event.Event) []event.Event
// Walk yields each stream's state (component "stream", key its name): the registry is a state holder (ADR-0006 §2(b)).
func (r *StreamRegistry) Walk(fn func(component, key string, state any))
```
- **`Schedule` replacing an existing (kind, ref)** — a TTL re-arm included — updates that timer in place (new due, new schedule order, heap fixed) and MUST NOT allocate. The walk order is unchanged: a replacement is still ordered as a fresh schedule.
- **Cost.** `Events` scans the rule clock's heap: it is on the cold path, for the registry and tests only.

## 5. `internal/playback`'s format layer (item 1; ADR-0005 §3)

`internal/playback` imports `event`, `clock`, `table` and `partition` (tests), plus `bufio`, `io`, `strconv`, `time` (formatting only) and `math`.

### 5.1 The app the log is read against

```go
// Stream is one declared stream; its index in App.Streams is its stream id (SEMANTICS §2.2's declaration order).
type Stream struct {
	Schema  event.Schema // name, attributes, time attribute (event.ArrivalTime for `time arrival`)
	Delay   int64        // d_s, ms
	Derived bool         // a rule's output: never in a log (v0.1)
}
// Table is one declared table; its index in App.Tables is its table id.
type Table struct {
	Name  string
	Attrs []event.Attr
	Key   []string
}
type App struct {
	Streams []Stream
	Tables  []Table
}
```
S3.3's planner builds an `App` from the model; tests build it by hand. Names must be unique among streams and among tables, and `Delay ≥ 0`.

### 5.2 The log format (JSON Lines; one record per line)

| Record | Form | Notes |
|---|---|---|
| event | `{"stream":"S","attrs":{…}}` · `"batch":"b1"` optional · `"ts":"<RFC 3339>"` only on a `time arrival` stream (§7 O1) | `attrs` is the payload. An event-time stream's time is its time attribute inside `attrs`. |
| table | `{"table":"T","op":"put","row":{…}}` · `{"table":"T","op":"delete","row":{<key columns>}}` | Applied at its position: visible to the next event on (SEMANTICS §4, §10). |
| batch end | `{"batch_end":"b1","stream":"S"}` | A source's batch boundary (SEMANTICS §7). Nothing consumes it before S8. |

- **No other record kind exists in v0.1** (§7 O2). Derived-stream punctuations are Phase 2, and source punctuation is reserved (SEMANTICS §2.1).
- **The end of the log is `wm → +∞`** for every stream (SEMANTICS §2.1).
- **Lines.** A line that is empty or holds only JSON whitespace is skipped. Line numbers count every line from 1. A line longer than 16 MiB is a load error.
- **File order is arrival order.**

**Reading a line** (one envelope decoder, §0.8):
- **The envelope** is decoded by an `event.Decoder` over the envelope schema: every key optional; `attrs` and `row` as `object`, `ts` as `timestamp`, the rest as `string`. An explicit `null` reads as absent.
- **The record is classified** by its keys:
  - `table` present → a table record;
  - else `batch_end` present → a batch-end record;
  - else `stream` present → an event record.
- **A load error** (`*LoadError`, which refuses the whole log) is any of:
  - an envelope syntax fault;
  - an unknown key (`Decoded.Unknown > 0`);
  - none of the three keys;
  - a key the kind forbids, or a missing key the kind requires;
  - an unknown stream or table;
  - a derived stream;
  - `ts` on an event-time stream, or missing on an arrival stream;
  - `op` other than `put` or `delete`;
  - a table `row` that fails the table's schema (put: every attribute; delete: exactly the key columns, in a key-only schema), or that the table refuses (a null key value).
- **An event error** (§7 O6): the payload `attrs` failing its stream's schema is not a load error. The record is an event error — dropped and counted — as live ingest would do with the same payload (SEMANTICS §10).

```go
type RecordKind uint8
const ( RecEvent RecordKind = iota + 1; RecTable; RecBatchEnd )
type TableOp uint8
const ( OpPut TableOp = iota + 1; OpDelete )

// Record is one log line, decoded. Every slice in it is fresh: the clock and later the operators keep events, so
// a Values slice MUST NOT be reused (ADR-0010 §4).
type Record struct {
	Line   int
	Kind   RecordKind
	Event  event.Event        // RecEvent: StreamID, Ts, Values, BatchID; Seq and Tables are the driver's
	Err    *event.DecodeError // RecEvent whose payload failed its schema: an event error; Values nil
	Table  int                // RecTable
	Op     TableOp
	Row    []event.Value      // put: the row in attribute order; delete: the key in key order
	Stream int                // RecBatchEnd
	Batch  string
}

type LoadError struct{ Line int; Msg string }
func (e *LoadError) Error() string // "playback: line <n>: <msg>"

type Reader struct{ /* … */ }
func NewReader(r io.Reader, app *App) (*Reader, error) // error: an invalid App
func (r *Reader) Next() (Record, error)                 // io.EOF after the last record
```

### 5.3 The virtual-clock driver

```go
type StepKind uint8
const (
	StepEvent      StepKind = iota + 1 // admitted: the step contract §1 with Advances
	StepStreamLate                     // behind its stream's admission watermark: dropped, counted; nothing moves
	StepErrored                        // its payload failed its schema: dropped, counted
	StepTable                          // a table record, applied
	StepBatchEnd
	StepEnd                            // the log's end: Advances raise every contribution to +∞; §1 steps 1 and 3
)

type Step struct {
	Kind     StepKind
	Line     int               // the record's line; StepEnd: the last line + 1
	Event    event.Event       // StepEvent, StepStreamLate: Seq and Tables set; StepErrored: StreamID only
	Err      *event.DecodeError
	Advances []clock.WMAdvance // StepEvent, StepEnd; valid until the next call
	Tables   *table.Snapshot   // the snapshot current after this step: what this step's timers read (§2)
	Table    int               // StepTable
	Op       TableOp
	Stream   int               // StepBatchEnd
	Batch    string
}

type Driver struct{ /* app; reg *clock.StreamRegistry; set *table.Set; seq []uint64; vnow int64; buf []clock.WMAdvance */ }

func NewDriver(app *App) (*Driver, error)                     // fresh, empty tables: a replay is a sandbox (SPEC §6.8)
func (d *Driver) Step(rec Record) (Step, error)               // a *LoadError when a table refuses a write
func (d *Driver) End() Step
func (d *Driver) Tables() *table.Set
func (d *Driver) Registry() *clock.StreamRegistry
// Replay runs every record of r through d, then End, calling fn on each step; it stops at the first error.
func Replay(r *Reader, d *Driver, fn func(*Step) error) error
```

**An event record, in this order (MUST):**
1. Stamp `Seq`: per stream, from 1, in file order. Every event that decodes gets one, a stream-late event included, as the live router stamps at intake before the registry judges lateness (ADR-0006 §4). An errored event gets none.
2. Set `Tables = set.Snapshot()`.
3. `late, a := reg.ObserveEvent(sid, Ts, Ts)`. Replay passes the event time as the arrival time; no tick ever reads it.
4. If `late` → `StepStreamLate`; the virtual clock does not move, as in the oracle.
5. Otherwise: `vnow = max(vnow, Ts)`; `Advances = [a if a.Moved] + reg.ObserveVirtual(vnow)` → `StepEvent`.

**The other steps:**
- **A table record** applies `Upsert([][]Value{Row})` or `Delete([][]Value{Row})`, and its step's `Tables` is the new snapshot.
- **`End`** = `reg.EndOfInput`.
- **The registry** is written by the driver only (ADR-0004 §1: "the ingest goroutine (live) or the playback driver").

### 5.4 The canonical output encoder

```go
// AppendValue appends v's canonical JSON. Error: a NaN or ±Inf float; a timestamp outside 0000–9999.
func AppendValue(dst []byte, v event.Value) ([]byte, error)
// AppendEvent appends e as one log line, no newline: {"attrs":{…},"batch":"…","stream":"…","ts":"…"}.
func AppendEvent(dst []byte, s event.Schema, e event.Event) ([]byte, error)
```
- **Keys.** They are sorted bytewise, the envelope's and the attributes' alike. There is no whitespace.
- **Every declared attribute appears**, a null as `null`. `text_invisibles` is engine-owned and is S3.2b's.
- **The envelope.** `batch` appears only when set. `ts` appears exactly when the schema has no time attribute (`time arrival`, or a derived output stream: §7 O1).

**The values:**

| Type | Encoded as |
|---|---|
| string | escaped as `encoding/json` with HTML escaping off (malformed UTF-8 → `�`; U+2028/2029 escaped) |
| int | decimal |
| float | `strconv.FormatFloat(f, 'g', -1, 64)` (`-0` kept) |
| bool | `true` or `false` |
| timestamp | RFC 3339 UTC with exactly three fraction digits (`2026-01-01T10:00:00.000Z`, ADR-0003's form) |
| duration | integer milliseconds (P2) |
| object | its stored JSON text, byte for byte (§7 O3) |

**The round trip (a test):** for every event a `Reader` accepts, `AppendEvent` gives a line the `Reader` reads back to equal `Values`, `Ts` and `BatchID`.

### 5.5 The canonical trace

```go
type Trace struct{ /* app; buf []byte; err error */ }
func NewTrace(app *App) *Trace
func (t *Trace) Step(s *Step)                                                   // the driver's entries
func (t *Trace) Released(at int, rule string, key event.Value, e event.Event)
func (t *Trace) ConstructLate(at int, rule string, key event.Value, e event.Event)
func (t *Trace) Fired(at int, rule string, key event.Value, tm clock.Timer)
func (t *Trace) Discarded(at int, rule string, key event.Value, ref uint64)
func (t *Trace) Purged(at int, rule string, key event.Value, due int64)
func (t *Trace) Note(at int, rule string, key event.Value, name string, v event.Value) // a body's own observation
func (t *Trace) Bytes() ([]byte, error)                                         // the first encoding error, if any
```
- **One canonical JSON line per entry**, keys sorted, in the order recorded.
- **Every entry carries `"at"`**, the line of the step whose processing produced it, and `"k"`, its kind:

| `k` | Other keys |
|---|---|
| `stream_late` | `stream`, `seq`, `ts` |
| `errored` | `stream`, `reason`, `attr` |
| `table` | `table`, `op` |
| `batch_end` | `stream`, `batch` |
| `end` | — |
| `event` (released) | `rule`, `key`, `stream`, `seq`, `ts` |
| `construct_late` | `rule`, `key`, `stream`, `seq`, `ts` |
| `timer` | `rule`, `key`, `kind`, `due`, `ref` |
| `discard` | `rule`, `key`, `ref` |
| `purge` | `rule`, `key`, `due` |
| `note` | `rule`, `key`, `name`, `value` |

- **Encoding.** `key` and `value` are canonical values (§5.4); a timer kind is its lowercase name.
- **The late counters** are these entries' counts, so a trace can be summed.
- **The trace is a test and debugging artifact.** S3.2b's replay through the runtime writes the same entries.

## 6. Tests (QA, written first; each fails on the stub for its stated reason)

| Exit / rule | Test | Proof |
|---|---|---|
| **Exit 1** | `TestTraceDeterministic`, `TestTracePlantedMapOrderIsRed` (`internal/playback`) | A log with three streams, several keys, out-of-order and late events, table records and batch ends; replayed twice through the §1 harness with a recording test body → byte-identical traces. The proof: the same harness with a body that writes its notes by ranging over a Go map gives two different traces within 20 replays. |
| **Exit 2** | `TestTableRecordFromNextEvent` | E1 (`ts` 10:00, `d` 1 min), then `put {x}`, then E2 at 10:02 (which releases E1). E1 reads `x not in T` through its own `Tables`; E2 reads `x in T`. A timer that fires in E2's walk reads E2's snapshot (O4). The same for `delete`. |
| **Exit 4** | `TestOneTTLTimerPerInstance` (`internal/partition`) | 10,000 events for one key → after every walk exactly one pending `TTLPurge` for it, due = `LastActivity + TTL`. 100 keys → 100 TTL timers. `AllocsPerRun` of a re-arm = 0. |
| TTL purge | `internal/partition` | The purge falls at its instant, before the key's later buffered events, which start it fresh (a new `NewState`, a new TTL). An event at exactly `LastActivity + TTL` keeps the key. A purged key with nothing left is freed, and one with a seal is freed once the clock passes it. A construct-late first event leaves no instance. `TTL` 0 never purges. |
| Keys | `internal/partition` | `-0.0` and `+0.0` are one key; `int` 1 and `float` 1.0 cannot both arrive (one `KeyType`); a null key or a wrong type panics; `KeyType` object is refused. |
| Invariant | `internal/partition` | `CheckTimers` after every step of every partition test, plus a planted body that schedules a timer its state does not imply → an error naming it. |
| Step contract | `internal/playback` | An event at the instant of its key's pending timer is released before the timer, never construct-late (§0.5). |
| Loader | `internal/playback` | One case per load-error form of §5.2 (line number, message); each payload fault → `StepErrored` with its reason; `Seq` per stream with a stream-late event counted; whitespace lines skipped; CRLF accepted; a BOM is a load error at line 1. |
| Virtual clock | `internal/playback` | The oracle's playback lateness: for a fixed-seed corpus generated from `executor.run_pattern_multi` (playback mode) with a body that schedules nothing, stream-late and construct-late indexes match (a committed fixture, like S3.1-2's invisibles fixture). RT-01: an empty stream does not stall the clock. |
| Encoder | `internal/playback` | Strings against `encoding/json` (HTML off) on generated strings, malformed UTF-8 included; floats (`-0`, `1e+21`, the smallest subnormal); NaN and ±Inf → an error naming the attribute; timestamps at year 0 and 9999, and one outside → error; the round trip. |
| Snapshot reuse | `internal/table` | `Set.Snapshot` returns the same pointer until a write, then a new one; 0 allocations when nothing changed. |
| Clock views | `internal/clock` | `Timers` in walk order, `Events` in `event.Compare` order; a replacement `Schedule` allocates 0 and keeps the walk's order. |

Each timing claim states the instant and is derived by hand in the test's comment. No test sleeps or reads the wall clock.

## 7. For the owner, after the review (recommendation first)

1. **O1 — where an event's time lives in the log.** → **`ts` appears exactly when the stream has no time attribute** (a `time arrival` source, or a derived output). An event-time stream's time is only its attribute, so a line can never carry two times that disagree. *Other option:* `ts` on every line, which must equal the attribute. It is easier to read and sort, but has two sources of one fact.
2. **O2 — punctuation records in v0.1.** → **None.** The log has events, table records and batch ends; anything else is a load error; the end of the log is +∞. Derived-stream punctuations are Phase 2. *Other option:* a "time passes to T" record. It would let a log stop time short of +∞, but nothing in v0.1 needs it (expect blocks end at +∞).
3. **O3 — malformed UTF-8 inside an `object` value (F11).** → **Replace it with U+FFFD when decoding**, as P8 does for strings. Every reader — rules, sinks, the encoder — then sees valid UTF-8, and the encoder copies the text as stored (whitespace kept). This is a small change to S3.1-2's decoder. *Other options:* replace only when encoding (each sink must remember to), or copy the bytes (the output may not be valid JSON).
4. **O4 — which tables a timer sees (F15).** → **The tables as at the arrival whose advance fired the timer.** *Example:* E1 at 10:00 starts an absence until 10:05; `put {x}` arrives; E2 at 10:06 moves the clock past 10:05. The completion that fires then sees `x` in the table. *Other option:* the tables as at the anchor event (E1). That is stable per match, but it reads a state older than what has arrived.
5. **O5 — an `object` partition key.** → **A deploy error with a new code in S3.3, beside D3's table-key error;** until then `partition` refuses it when a rule is created. Object equality is not defined, so two sessions could never be told apart.
6. **O6 — a bad line in a log.** → **A malformed line (envelope, table record) refuses the whole log with its line number; a payload that fails its stream's schema is an event error** (dropped, counted, in the trace), as live does. A replay then counts exactly what live counted. *Other option:* refuse the log on any fault, which is stricter but differs from live.

## 8. For the review

- **R1:** the step contract (§1). Does it hold for S3.2a-2's mailboxes? Note ADR-0010 §2's chunking: walking once per chunk instead of once per event changes results when a key's timer sits at an event's instant (the seal). Chunking must walk per event, or the invariance claim must be scoped.
- **R2:** `OpState.Timers` and `TTLPurge` owned by `partition`: the right seam for every later operator?
- **R3:** `event.Event.Tables any` against the alternatives (an epoch index into a runtime-held ring; a context on `clock.Instance.Buffer`).
- **R4:** two writers of the ingest arithmetic (the driver in replay, the router live). Should S3.2a-2 extract one single-threaded ingest step that both call?
- **R5:** `Seq` for stream-late events (stamped, unlike the oracle). The order is identical either way; only the numbers differ.

## 9. Forwarded to S3.2a-2

- The router: keys read from the attribute, null-key and wrong-type event errors, `Tables` set live from `table.Set.Snapshot`, and an event's own advances riding with it to its shard (§1).
- The chunking question (R1).

## 10. Gate and size

- **The gate:** no new row; the `unit` row runs every test.
- **Import law:** `internal/partition` stays on `event` and `clock`; `internal/playback`'s rules list already allows its imports.
- **Size:** about 5.5 points, as carded.
  - Moved out: key evaluation, to S3.2a-2.
  - Added: `table.Set`, the clock views and the allocation fix (about 0.5 point).
