# S3.1-1 — design note: the clock service and the metrics counters (v2)

> **Historical — S3.1-1, the clock service and the metrics counters; merged in PR #8.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-04--s22b-closed-s31s-plan-check); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** **design** · [record](s3-1-1-record.md) · reviews: [design: architecture](../reviews/s3-1-1-design-architecture.md), [owner's decisions: architecture](../reviews/s3-1-1-decisions-architecture.md). **← [S2.2b](s2-2b-record.md) · [S3.1-2](s3-1-2-record.md) →**

The clock as the owner decided it after two architecture reviews ([design review](../reviews/s3-1-1-design-architecture.md), [impact review](../reviews/s3-1-1-decisions-architecture.md)); the decisions are [sprint-plan §5, "S3.1-1's clock"](decisions.md#owner-decisions-2026-11-04--s31-1s-clock-after-two-architecture-reviews), numbered **D1–D8** below. ADR-0004 is annotated to match. The stub in `internal/clock` and `internal/metrics` carries exactly this API with panicking bodies. §8 places every review finding.

## 1. The stream registry (per app; written by ingest or the playback driver only)

```go
type StreamSpec struct {
	Name      string
	Delay     int64 // d_s, ms: the watermark bound
	IdleAfter int64 // ms; 0: the stream declares no idleness and never goes quiet
	Derived   bool  // a rule's output: punctuations only (v0.1 routes none)
}

type WMAdvance struct {
	StreamID     int
	Contribution int64
	Moved        bool // false: no advance; the zero value is no advance
}

func NewStreamRegistry(specs []StreamSpec, start int64) *StreamRegistry // start: the deployment's start (wall ms)
func (r *StreamRegistry) ObserveEvent(sid int, ts, arrival int64) (streamLate bool, advance WMAdvance)
func (r *StreamRegistry) ObserveTick(now int64, buf []WMAdvance) []WMAdvance    // live: a wall tick from the intake
func (r *StreamRegistry) ObserveVirtual(now int64, buf []WMAdvance) []WMAdvance // playback: the log's time
func (r *StreamRegistry) ObservePunctuation(sid int, w int64) WMAdvance
func (r *StreamRegistry) EndOfInput(buf []WMAdvance) []WMAdvance                // every contribution to +∞
func (r *StreamRegistry) Contribution(sid int) int64
```

Per source stream `s`, every quantity is monotone, so a contribution never falls:

- **Admission** `adm(s) = max(ts seen) − d_s` (−∞ before the first event). **Stream-late** iff `ts < adm(s)` before this event: strict; a stream-late event moves nothing. Unchanged in both modes.
- **Quiet floor (live; D3, D5).** The stream is quiet once `now − lastArrival ≥ IdleAfter` at a tick. While quiet, `floor(s) = max(floor(s), lastTs + (now − lastArrival) − d_s)`: its own last event time, moved forward at wall speed. A stream with no event yet uses `lastTs = lastArrival = start`. Ticks come from the runtime's intake, behind the events already queued, so a tick never outruns unread events; the registry never reads a clock.
- **Virtual floor (playback).** `ObserveVirtual(now)`: `vfloor(s) = max(vfloor(s), now − d_s)` for every source stream of the app (ADR-0004: "EVERY stream"; the playback driver passes the log's time).
- **Contribution** `contrib(s) = max(adm(s), floor(s), vfloor(s))`; +∞ only after `EndOfInput`. `Moved` is true exactly when it rose. Subtractions saturate at `math.MinInt64`; additions at `math.MaxInt64`.
- **Punctuation (derived):** `contrib(s) = max(contrib(s), w)`; kept for Phase 2.
- `ObserveTick`, `ObserveVirtual` and `EndOfInput` append to the caller's `buf` only the advances that moved (no allocation on the clock path) and return it.
- *As built, after the break-it pass:* keys at one instant go in **birth order** — a key is born at its first released event or first timer, and born again after its purge's `CancelAll` (the executor's order; `Ordinal()` stays the creation label); `Empty()` is false while the key's seal is not yet behind the clock (freeing would forget it); a clock still at −∞ releases nothing, and +∞ (end of input) is past every due, strict ones included.
- *As built:* a quiet floor runs from the largest event time seen; any event, a stream-late one included, is an arrival and ends a quiet spell (both as the executor does). Instance ordinals are never reused; a timer scheduled with a due before the walk's position fires next.

## 2. The rule clock and its instances (per rule per shard; D2)

```go
func NewRuleClock(inputs []int) *RuleClock                     // the stream ids the rule reads
func (rc *RuleClock) ApplyAdvance(a WMAdvance) (clock int64, advanced bool)
func (rc *RuleClock) Clock() int64
func (rc *RuleClock) NewInstance() *Instance                   // a key born now; ordinals count up from 0
func (rc *RuleClock) Advance(cb Callbacks)                     // the walk (§3) up to the clock
func (rc *RuleClock) Free(in *Instance)                        // the shard drops a key: only when in.Empty()

func (in *Instance) Ordinal() uint64
func (in *Instance) Buffer(e event.Event) (constructLate bool)
func (in *Instance) Schedule(due int64, k TimerKind, ref uint64) // (k, ref) unique: a second Schedule replaces;
                                                               // a key has one TTLPurge, whatever its ref (ADR-0004 §5)
func (in *Instance) Cancel(k TimerKind, ref uint64) bool
func (in *Instance) CancelAll()                                // the TTL purge's reset (D1)
func (in *Instance) Empty() bool                               // no buffered event and no timer
```

- **The construct clock** is `max(clock, min over inputs of contrib)`, one per rule per shard, shared by every key; a key born late starts at the rule's current time. Every contribution only rises, so there is no idle exclusion and no retreat to guard.
- **Construct-late (D7):** `Buffer(e)` returns true and keeps nothing when `e.Ts < clock`, or when `e.Ts ≤ sealed`, where `sealed` is the latest instant at which this key's timers fired. Otherwise the event waits in the rule's release buffer, tagged with its instance.
- **Freeing (D1):** a key's instance is freed by the shard only when it is `Empty()`; the TTL purge resets the operator's state and calls `CancelAll`, but never drops events already buffered for the key — they start it fresh.

## 3. The walk (D1)

`Advance` processes every pending item with time `≤ clock`, in this order — the only place the order exists (ADR-0004 §2 as annotated):

1. by **instant** `t`, ascending;
2. within `t`: **events** with `Ts = t`, in `event.Compare` order across all keys, each through `OnEvent`; then **timers** due at `t` of the inclusive kinds, by (key ordinal, kind, ref, schedule order), each key's group through one `OnTimers` call; then, **only if `clock > t`**, the **strict** timers due at `t` — `TTLPurge` through `OnTimers`, then `Discard` through `OnDiscard` — by (key ordinal, kind, ref, schedule order), so a purge precedes a discard at one instant whatever their refs (the executor's order);
3. a timer scheduled by a callback with a due at or after the current position joins the walk in order; one with a due before it fires at the current position.

```go
type TimerKind uint8

const ( // the order for one key's inclusive timers at one instant
	Completion TimerKind = iota + 1 // a match waiting out its absence windows (D4); wm ≥ due
	CronClose                       // a cron bucket's end; wm ≥ boundary
	DelayDue                        // a delayed event's due; wm ≥ t + D
	BatchClose                      // a batch's close punctuation (S8 places it; ADR-0014)
	TTLPurge                        // strict: fires once wm > due (due = last activity + TTL)
	Discard                         // strict: fires once wm > due (due = t_first + within); OnDiscard only
)

type Timer struct {
	Due  int64
	Kind TimerKind
	Ref  uint64
}

type Callbacks struct {
	OnEvent   func(in *Instance, e event.Event)
	OnTimers  func(in *Instance, fires []Timer)
	OnDiscard func(in *Instance, refs []uint64)
}
```

Operators pass SEMANTICS' deadline as the due (`t_first + within`, `t_last + TTL`); the kind carries the strictness, so no operator adds a millisecond. A match completes inside `OnEvent` when every absence deadline is behind the event (D4); only a match waiting out an absence schedules a `Completion`. TTL is re-armed when an event is released, not when it arrives. Skip-past-last compares in §2.2 order (D8) — an operator rule, recorded here because it is why the walk keeps events in `event.Compare` order.

## 4. The metrics counters (ADR-0006: per shard, summed on scrape, no atomics)

```go
type ID int
type Registry struct{ /* names in declaration order; frozen at the first NewShard */ }
func (r *Registry) Add(name string) (ID, error) // error: a duplicate name, or a registry already frozen
type Shard struct{ /* one int64 per counter */ }
func NewShard(r *Registry) *Shard
func (s *Shard) Inc(id ID, n int64)
func (s *Shard) Get(id ID) int64
type Sample struct { Name string; Value int64 }
func Sum(r *Registry, shards []*Shard) []Sample // declaration order; for a quiesced runtime and tests
```

The clock returns facts (`streamLate`, `constructLate`) and counts nothing; the runtime counts, and scrapes a live shard through its mailbox (S3.2a).

## 5. The reference executor and SEMANTICS change first

The harness is the oracle, so it changes before the build: `executor.py`'s drain walks instants (discards and purges per instant, strict); quiet streams take wall ticks with the D5 floor; the instant seals per key; skip-past-last compares in §2.2 order. The `2.1` multi-stream check gets explicit ticks. New checks, red first: an unrelated event never changes a result (granularity); a TTL purge due inside a jump precedes a later event of the key; every input quiet, the absence fires on time; a sealed instant drops a late same-instant event; a same-instant partial survives a match that completes by timer. SEMANTICS §2.1, §6.4, §6.5 and §9, ADR-0004, ADR-0006 §3, ADR-0012 §3 (ticks stop before the sources at Stop) and ADR-0013 §3–§4 carry dated annotations.

## 6. Tests

- QA's tests (written first against v1) are revised by QA to v2: the walk replaces the one-chunk order; the per-rule clock replaces per-instance clocks; quiet floors replace the +∞ exclusion; the edge "all inputs quiet, the clock stops" becomes "all inputs quiet, the clock moves at wall speed".
- **Exit 1:** the harness's `2.1` arithmetic at clock level, each test naming its check. **Exit 2:** the walk through a test-double operator; the table of kinds, strictness and order is the owner's check. **Exit 3:** the quiet key: two keys of one rule; the silent key's absence fires at the right time and its TTL purge at the right time.
- A determinism test: the same input, split into different advance sizes, gives the same callback sequence.

## 7. Size

D1–D8 add about 2 points: S3.1-1 is about 8.5 of S3.1's 13, under the cap; no re-split. S3.2a gains about 1–2 (ticks through the intake, the per-rule walk across instances is here already).

## 8. Review dispositions

| finding | disposition |
|---|---|
| F1, F11 | fixed: the walk (D1) |
| F2 | fixed: strict kinds (§3) |
| F3, F10 | fixed: per-instant order (§3) |
| F4 | fixed: D4; ADR-0013 §3 annotated |
| F5 | fixed: one clock per rule (D2) |
| F6, F7, F12 | dissolved: monotone floors (D3, D5); +∞ only at end of input |
| F8 | fixed: purge keeps the buffer, re-arm at release (§2, §3) |
| F9 | fixed: kinds per ADR-0013/0014 (§3) |
| F13 | fixed: caller buffers; typed heaps |
| F14 | fixed: `Add` errors; frozen at first shard |
| F15 | per ADR-0004: app-wide virtual floors |
| F16 | fixed: `(kind, ref)` unique |
| D1-1 | fixed: §2 freeing |
| D1-2 | fixed: D7 |
| D1-3 | forwarded to S8: where a batch close sits in the walk |
| D2-1 | fixed: the rule clock walks its instances (§3); its index is internal |
| D2-2 | fixed: key ordinals, declaration order (§3) |
| D3-1, D3-2 | fixed: D5, ticks through the intake |
| D3-3 | forwarded to S3.2a/S4.2: ticks stop before the sources; ADR-0012 annotated |
| D3-4 | fixed: injectable tick source (S3.2a); ADR-0004 §4 annotated |
| D4-1 | fixed: per match (§3) |
| D4-2 | fixed: D8 |
