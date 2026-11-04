# S3.1-1 — design note: the clock service and the metrics counters

A dated reading of `main` at the S2.2b merge (2026-11-04), for the architecture review before the build (G1). The card is [sprint-plan §3, S3.1](sprint-plan.md), items 1–2 (the owner split S3.1 at its card's line). The design is [ADR-0004](../architecture/adr/ADR-0004-clock-service.md); this note fixes what ADR-0004 leaves open and marks **A1–A4**, each with a recommendation. A1–A3 add to ADR-0004's normative interface and go to the owner after the review. The stub in `internal/clock` and `internal/metrics` carries exactly the API below, with bodies that panic, so QA's tests compile before the build.

## 1. The stream registry (per app; written by ingest or the playback driver only)

```go
type StreamSpec struct {
	Name    string
	Delay   int64 // d_s, ms: the watermark bound (0 for ascending and arrival streams)
	Derived bool  // a rule's output: no admission, punctuations only (v0.1 routes none; ADR-0004 §6)
}

// WMAdvance is one stream's new contribution to the construct clocks that read it.
type WMAdvance struct {
	StreamID     int
	Contribution int64
	Moved        bool // A1: false is "no advance"; the zero value is no advance
}

func NewStreamRegistry(specs []StreamSpec) *StreamRegistry // stream id = index in specs (SPEC §6.4's declaration order)
func (r *StreamRegistry) ObserveEvent(sid int, ts int64) (streamLate bool, advance WMAdvance)
func (r *StreamRegistry) ObserveIdle(sid int) WMAdvance              // live: the stream's `idle after` elapsed
func (r *StreamRegistry) ObservePunctuation(sid int, w int64) WMAdvance // a derived stream's punctuation
func (r *StreamRegistry) ObserveVirtual(now int64) []WMAdvance       // playback: every source stream's floor to now − d_s
func (r *StreamRegistry) Contribution(sid int) int64
```

The arithmetic, per source stream `s`, from SEMANTICS §2.1, ADR-0004 §4 and the reference executor (`run_pattern_multi`):

- **Admission watermark** `adm(s) = max(event times seen on s) − d_s`; −∞ (`math.MinInt64`) before the first event. It is judged on the stream's own events in both modes; the virtual floor never touches it.
- **Stream-late** iff `ts < adm(s)` as it stood *before* this event: strictly; an equal timestamp is admitted. A stream-late event does not move `adm(s)`.
- **Contribution** `contrib(s) = max(adm(s), virtual floor(s))`, where the virtual floor (playback only) is `virtualNow − d_s` and `virtualNow` is the largest `now` passed to `ObserveVirtual` so far.
- **Idle (live):** `ObserveIdle(s)` sets `contrib(s) = +∞` (`math.MaxInt64`): the stream leaves every min. Its next event clears the idle mark and its contribution falls back to `adm(s)` — the clamped construct clock does not retreat (ADR-0004 §4).
- **Punctuation (derived):** `contrib(s) = max(contrib(s), w)`; kept for Phase 2, unused by v0.1's routing.
- An observation returns `Moved: true` exactly when `contrib(s)` changed. Subtractions saturate: `ts − d_s` and `now − d_s` never wrap below `math.MinInt64`.

## 2. The instance (per rule × key; used by its shard only)

```go
type TimerKind uint8

const ( // (b)'s order for timers due at the same instant, then by schedule order
	Completion TimerKind = iota + 1 // a pending completion resolves
	AbsenceResolution
	CronClose
	DelayDue
	TTLPurge
	Discard // a partial's `within` deadline: routed to OnDiscard in (c), never to OnTimers
)

type Timer struct {
	Due  int64
	Kind TimerKind
	Ref  uint64
}

type AdvanceCallbacks struct {
	OnEvents  func(batch []event.Event) // (a) released ≤ clock, in event.Compare order, as one chunk
	OnTimers  func(fires []Timer)       // (b) due ≤ clock, by (due, kind, schedule order)
	OnDiscard func(overrun []uint64)    // (c) Discard refs due ≤ clock, by (due, schedule order)
}

func NewInstance(inputs []int) *Instance // the stream ids it reads; every contribution starts at −∞
func (in *Instance) Clock() int64
func (in *Instance) Buffer(e event.Event) (constructLate bool)  // A2
func (in *Instance) ApplyAdvance(a WMAdvance) (newClock int64, advanced bool)
func (in *Instance) Advance(cb AdvanceCallbacks)
func (in *Instance) Schedule(due int64, k TimerKind, ref uint64)
func (in *Instance) Cancel(k TimerKind, ref uint64) bool       // A3
```

- **The construct clock** is clamped-monotonic: `clock = max(clock, min over inputs of contrib)`, where an input at +∞ (idle) is left out; when every input is idle the clock does not move (the executor's `if active:`). `ApplyAdvance` ignores a stream the instance does not read and an advance with `Moved: false`.
- **Construct-late** (A2): `Buffer(e)` returns true and keeps nothing when `e.Ts < clock`, strictly — counted by the caller, separately from stream-late (SEMANTICS §2.1). Otherwise the event waits in the release buffer.
- **`Advance` is the one ordered sequence** (ADR-0004 §2): (a) every buffered event with `Ts ≤ clock`, in `event.Compare` order, as one `OnEvents` chunk; (b) every non-discard timer with `Due ≤ clock`, by (due, kind, schedule order) — a timer scheduled by a callback during (a) or (b) and already due fires in the same `Advance`, in a later `OnTimers` call; (c) every `Discard` timer with `Due ≤ clock`, by (due, schedule order), in one `OnDiscard` call. A callback that has nothing to receive is not called.
- **A pending completion is never discarded** by construction: the operator that turns a partial into a pending completion cancels its `Discard` timer (A3) and schedules a `Completion`. Timers that share a due instant fire in (b) before any discard in (c) (RT-10).
- **TTL** (ADR-0004 §5): `Schedule(due, TTLPurge, _)` *replaces* the instance's one TTL timer, so there is exactly one, re-armed on activity.
- **Timers** live in a binary heap per instance behind an unexported type; the per-shard facade of ADR-0004 §5 is the runtime's (S3.2a).

## 3. The metrics counters (card item 2; ADR-0006: per shard, summed on scrape, no atomics)

```go
type ID int
type Registry struct{ /* counter names in declaration order */ }
func (r *Registry) Add(name string) ID   // once per counter, at deploy
type Shard struct{ /* one int64 per counter */ }
func NewShard(r *Registry) *Shard
func (s *Shard) Inc(id ID, n int64)
func (s *Shard) Get(id ID) int64
type Sample struct { Name string; Value int64 }
func Sum(r *Registry, shards []*Shard) []Sample // in declaration order
```

`internal/clock` returns facts (`streamLate`, `constructLate`) and counts nothing itself; the runtime increments the counters (S3.2a), which also makes a scrape safe (A4).

## 4. Open questions

- **A1 — the "no advance" value.** ADR-0004 says a `WMAdvance`'s zero value is "no advance", but stream 0 advancing to time 0 *is* that zero value. Recommended: a `Moved` field; the zero value stays "no advance". *Alternative:* an `ok bool` second return on every `Observe…`.
- **A2 — construct-late needs a voice.** ADR-0004's `Buffer` returns nothing, so the runtime cannot count a construct-late drop (harness 2.1, the idle-resume check). Recommended: `Buffer` returns `constructLate bool`.
- **A3 — cancelling a timer.** A partial that becomes a pending completion must lose its `Discard` timer, or exit line 2's "a pending completion is never discarded" holds only by operator discipline. Recommended: `Cancel(kind, ref) bool`.
- **A4 — scraping per-shard counters.** A plain `int64` read from another goroutine is a data race. Recommended: no atomics (ADR-0006); the runtime scrapes a shard through its mailbox (S3.2a), and this sprint's `Sum` is for a quiesced runtime and tests. Not an owner question unless the review disagrees.

## 5. Tests (QA's first, from the spec)

- **Exit 1 — the harness's arithmetic at clock level**, each test naming its source check: admission strictness and the late drop (`2.1/6.5`); the clamped min and the construct-late idle-resume event (`2.1` multi-stream); per-stream virtual floors and the empty stream (`2.1 · RT-01`); playback admission equal to live admission, construct-late counted (`2.1` playback admission); firing at the watermark (`2.1` absence due 10:01:30).
- **Exit 2 — the timer contract**, through a test-double operator that schedules every kind and the discard kind: due order within (b), discards only in (c), a cancelled discard never arrives, one TTL timer per instance. Its table of kinds and their order is the owner's check.
- **Exit 3 — the quiet key:** a two-key registry and two instances; one key never receives an event after its anchor; the broadcast `WMAdvance` still moves its clock, its absence fires at the right event time, and its TTL purge happens at the right time (ADR-0004's mandated case).
- **Metrics:** sums across shards in declaration order.
