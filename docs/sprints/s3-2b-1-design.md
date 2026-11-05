# S3.2b-1 — design note: the plan, replay through the runtime, rate limits (v1)

**What it covers:** S3.2b's items 1, 2, 6 and 9 ([card](sprint-plan.md#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time)) and its exits 1 and 5, plus the owner's Q1: a plain rule sees each event as its stream accepts it.

**The owner's answers** at the plan check ([sprint-plan §5, "S3.2b's plan check"](sprint-plan.md#owner-decisions-2026-11-06--s32bs-plan-check)): Q1 (on arrival; a rate limit compares event times in arrival order), Q2 (S3.1's routine; this half of the split).

**Routine (owner):** QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it. MUST and MUST NOT lines (§7) are the build's contract.

## 0. What the code says (read first)

Read against `main` @ a5dbc2d.

1. **`validate.Check` returns the checked `*model.App`** (`internal/validate/validate.go:38`): a deep copy with every `${param}` slot resolved; a duration's effective value is in `Ms` and `Param` is cleared (`resolve.go:133`). There is no other graph type; `internal/plan` consumes this.
2. **`plan` may not import `runtime`; `runtime` and `playback` may import `plan`** (`tools/importcheck/rules.json`). `plan` may import `event`, `model`, `expr`, `partition`, `pattern`, `window`, `text`, `table`, `metrics` — not `clock`.
3. **`runtime.Rule` is already the routing table** (`internal/runtime/runtime.go:32`): `Inputs`, `KeyAttr` per input, `KeyType`, `TTL`, `NewBody(emit)`. A partition key is an attribute (`model.PartitionKey{Attr, Stream}`), so `KeyAttr` suffices.
4. **Every rule goes through partition's release buffer:** `Rule.Offer` buffers in the key's `clock.Instance` and `Rule.Advance` releases it once the clock is past the event (D1 of S3.2a-1). There is no on-arrival path.
5. **The shard step is already pure:** `shard.process(m)` applies advances, offers, walks, and appends to `sh.outbox` (`runtime.go:934`). The router's `event(a, e)` builds one message per shard and hands each to `rt.push` (`runtime.go:708`). It reads `cfg.Now()` once per event (`:714`); `Start` reads it once (`:254`).
6. **The playback driver owns its own `runtime.Ingest` and tables** and runs no rules (`internal/playback/driver.go:17`); its tests drive `partition.Rule`s by hand (`helpers_log_test.go:277`).
7. **`expr.Row` is an interface** (`internal/expr/expr.go:26`): a struct value passed to it is boxed and allocates; a pointer to a long-lived struct does not.
8. **A derived stream's `ts` is filled with the output timestamp** and a `select` may widen `int` to `float` (`internal/validate/exprs.go:276, 310`).

## 1. `internal/plan` (item 1)

```go
// Package plan turns a checked model (validate.Check's result) into an app's executable description
// (ARCHITECTURE §5 step 1): the streams, the tables, and per rule its routing and its operator chain.
package plan

// App is one app's executable description.
type App struct {
	Streams []Stream // declaration order: the index is the stream id (SEMANTICS §2.2)
	Tables  []Table  // declaration order: the index is the table id
	Rules   []Rule   // declaration order
}

// Stream is one declared stream.
type Stream struct {
	Schema    event.Schema // TimeAttr() == event.ArrivalTime for `time arrival`
	Delay     int64        // d, ms: the bound; 0 for ascending and arrival
	IdleAfter int64        // ms; 0: none
	Derived   bool         // no time clause, produced by a rule
}

// Table is one declared table.
type Table struct {
	Name  string
	Attrs []event.Attr
	Key   []string
}

// Rule is one rule: its routing (the runtime's routing table) and its body.
type Rule struct {
	Name      string
	Inputs    []int      // the stream ids it reads
	KeyAttr   []int      // per input, the key attribute's index; nil: key-less (the pseudo-key "")
	KeyType   event.Type // TString for a key-less rule
	TTL       int64      // ms; 0 for a key-less rule
	OnArrival bool       // a plain body: sees each event as its stream accepts it (owner Q1)
	NewBody   func(emit func(event.Event)) partition.Body
}

// Stats are one rule's emission counters on one shard. Every body NewBody returns implements Counted.
type Stats struct {
	Suppressed uint64 // outputs the rate limit suppressed (suppressed_outputs, SEMANTICS §8)
	Errored    uint64 // events an expression error dropped for this rule (SEMANTICS §10; exposed in S3.2b-2)
}

// Counted is a body that reports Stats.
type Counted interface{ Stats() Stats }

// ErrNotBuilt: the model holds a body kind this release cannot run yet (patterns S4.1, windows S8).
var ErrNotBuilt = errors.New("plan: this body kind is not built yet")

// Build describes app, which MUST be validate.Check's result (no findings but lints).
func Build(app *model.App) (*App, error)
```

**What `Build` does, in order:**
1. **Streams:** `event.NewSchema(name, attrs, timeAttr)` — `timeAttr` is the time attribute's index, or `event.ArrivalTime`. `Delay` = the bound (0 for ascending and arrival); `Derived` = no time clause.
2. **Tables:** name, attributes, primary key.
3. **Rules, plain bodies only** (any other kind: `ErrNotBuilt`):
   - `Inputs` = the body's source stream; `KeyAttr` = the partition key's attribute index on it, or nil; `KeyType` its type (`TString` when key-less); `TTL` = `Partition.TTL.Ms`.
   - `OnArrival` = true.
   - The filter: `expr.Build(source.Filter, scope)`; no filter means "always true".
   - The projection: per attribute of the target stream other than `ts`, the select item bound to it (by `as`, or by the attribute's own name), compiled with `expr.Build`; `int` into `float` widens.
   - `ts`: if the target declares an attribute `ts`, it holds the output timestamp.
   - The rate limit: `D` = `OutputFirstEvery.Ms` (0: none).
4. **The scope** `plan` gives `expr.Build`: slot 0 is the source event; `Attr("", name)` is its schema index; `Table(name)` is the table id and its key type; `Capture` is never asked (no captures in a plain body).

## 2. The filter body and the emission stage (items 1, 6)

One body per (rule, shard), made by `NewBody(emit)`, single-goroutine.

**`OnEvent(in, e)`** — the only hook that does work; `NewState` returns a stateless `OpState` (no timers, `Walk` yields nothing):
1. Copy `e` into the body's own row (`b.row.ev = e`; `b.row.tables, _ = e.Tables.(*table.Snapshot)`) and evaluate the filter on `&b.row` — a pointer to a long-lived struct, so nothing is boxed (§0.7).
2. An expression error: count `Errored`, return (this rule only; SEMANTICS §10). Null or false: return.
3. Project: allocate the output `Values` (emission may allocate, ADR-0010 §1); evaluate each select item; widen `int` to `float`; set `ts` to `e.Ts`. An error: count `Errored`, return.
4. Hand `event.Event{StreamID: target, Ts: e.Ts, Values: vals}` to the emission stage.

**The emission stage** (`plan`, generic: patterns and windows will call it too):
```go
// out applies the rate limit (SEMANTICS §8): with D > 0, an output with Ts <= anchor + D is suppressed and counted;
// otherwise it is emitted and the anchor moves to its Ts.
func (m *emission) out(in *partition.Instance, e event.Event)
```
- The anchor lives on `partition.Instance` (ARCHITECTURE §5 step 7), so it is born, purged and reset with the key (SEMANTICS §9).
- Ties are suppressed (`<=`). A key-less rule has one instance, so one global anchor (RT-16).
- Under Q1 outputs reach the stage in arrival order; an output older than the anchor is inside the interval and suppressed.

## 3. `partition`: the on-arrival path and the anchor (owner Q1)

```go
type Config struct {
	// … as today …
	OnArrival bool // the body sees each event at Offer; nothing is buffered (owner Q1 of S3.2b)
}

type Instance struct {
	// … as today …
	Anchor   int64 // the rate limit's last emission time (SEMANTICS §8); meaningful when Anchored
	Anchored bool
}
```

**`Offer` on an on-arrival rule:**
1. The same key checks (a null, wrong-type or NaN key panics; the router checks first).
2. The key's instance, created if absent, with a `clock.Instance` for its TTL timer only.
3. No construct-late test and no buffering: an accepted event is never dropped (Q1). `constructLate` is always false.
4. Birth (`NewState`) if `Op == nil`; `LastActivity = max(LastActivity, e.Ts)`; the TTL timer is re-armed at `LastActivity + TTL` (one per key, `Schedule` replaces); `body.OnEvent` under `recoverBody`.

**The shard step's order becomes apply → walk → offers → walk** (was apply → offers → walk). The first walk fires what fell due before the event arrived — a TTL purge in particular — so an on-arrival event never meets a key that should already have been purged. For buffered rules the extra walk changes nothing: under D1 when a walk runs never changes a result (ADR-0010 §2 as annotated).

**The anchor and resets:** `reset` (purge, panic) clears `Anchor` and `Anchored`; `Walk` yields `("anchor", key, Anchor)` when set (the snapshot seam).

**Why a purge never changes an alert** (SEMANTICS §9 + §11.3's TTL ≥ D): a purge needs `clock > LastActivity + TTL`; any event accepted afterwards has `t ≥` its stream's admission watermark, which, without idleness, is at least the clock, so `t > LastActivity + TTL ≥ Anchor + D`: it would have been emitted anyway. *The one exception is open point O1 (§11).*

## 4. The runtime with one shard, stepped synchronously (item 2)

ADR-0005 §3: replay runs "single-threaded through the real runtime". ARCHITECTURE §6: playback owns no goroutine.

```go
// Sync is the replay drive loop: one app on one shard, every step run to completion on the caller's goroutine,
// through the same router event step and shard step as the live runtime. It starts no goroutine.
type Sync struct{ /* … */ }

// NewSync builds app on one shard. A fixture starts virtual at 0 (start ignored); out receives every output, in
// emission order, during the step that emits it. App.Sinks is not used.
func NewSync(app App, start int64, virtual bool, out func(e event.Event)) (*Sync, error)

func (s *Sync) Begin(start int64)                                       // a recording's header: live ingest from start; before any event
func (s *Sync) Event(e event.Event, arrival int64) (late bool, adv []clock.WMAdvance)
func (s *Sync) Tick(now int64) []clock.WMAdvance
func (s *Sync) TableWrite(table int, op TableOp, rows [][]event.Value) error
func (s *Sync) Drain() ([]clock.WMAdvance, DrainResult)                 // a recording's stop record
func (s *Sync) End() []clock.WMAdvance                                  // a fixture's end: +∞
func (s *Sync) Snapshot() *table.Snapshot
func (s *Sync) Registry() *clock.StreamRegistry
func (s *Sync) Walk(fn func(component, key string, state any))
func (s *Sync) Stats() Stats

// FromPlan is the runtime's App for p; sinks are bound by the caller.
func FromPlan(p *plan.App, tables *table.Set, sinks []SinkBinding) App
```

**The refactor:**
- `router.event(a, e)` becomes `routeEvent(a, e, arrival, deliver func(shard int, m message))`. Live: `arrival = rt.now(cfg.Now())`, `deliver` = `rt.push`. `Sync`: the arrival is the step's, `deliver` = the one shard's `process`, then the outbox is flushed to `out` in order.
- Ticks, table writes and drains get the same split: one function computes, the caller delivers.
- `newShard` takes the app states, not `*Runtime`, so both build shards the same way.
- `RuleStats` gains `Suppressed` and `Errored`, summed from each body's `Counted`.

**Playback over `Sync`:** the driver keeps its record-to-step contract (`Driver.Step` returns the same `Step`, advances included) but owns a `Sync` instead of its own `Ingest` and tables, so a replay has exactly one ingest. `NewDriver(app *App)` builds a `Sync` with no rules (the format layer's tests stay unchanged); `NewPlanDriver(p *plan.App, out func(event.Event))` builds one with the rules; `FromPlan(p *plan.App) *App` gives the format layer's view.

**The lines re-run through it** (the card's "re-running S3.1's and S3.2a's lines"): S3.1's exit 4 (a table write is visible to the next evaluation, never the current one) and S3.2a's exits 1 (byte-identical canonical trace, red on a planted map order), 2 (a table record takes effect from the next event) and 4 (one TTL timer per instance) — each through `NewPlanDriver` with a real filter rule or the test-double body. Exit 3 (saturation) is live-only.

## 5. Live against replay (exit 1)

**The test** (in `internal/playback`'s tests, which may use both `runtime` and `playback`):
1. A seeded generator writes a **recording** (header, events with arrival stamps, ticks, table records, a stop): 2–3 source streams (one with `idle after`), out-of-order times within each bound, some stream-late events, one table. Rules: a partitioned plain rule with a rate limit, a key-less plain rule with a rate limit, and S3.2a's buffered test-double body (timers).
2. **Live:** `runtime.Start` with N shards; `Config.Now` pops the recording's next arrival (the router reads it once at `Start` and once per event, §0.5 — a MUST, §7); the test sends table writes, events and ticks in file order, then drains, and collects every sink's outputs.
3. **Replay:** the same recording through `NewPlanDriver`.
4. **Compare:** per rule and key, the output sequence (values and timestamp) is identical; across keys the output multisets are equal; `StreamLate`, `ConstructLate`, `Suppressed` and `Purged` are equal.
5. N ∈ {1, 2, 4, 7}; 200 seeds in the `unit` row, inside its time ceiling; a failing seed prints its recording.

**Teeth:** a planted divergence — a body whose output depends on its shard (passed in through a test-only hook) — must turn the test red for N > 1.

## 6. Test material for 07 and 09 (item 9, QA, before the engine runs)

Per the owner's 2026-11-02 rule: in each example, at least one fire-path and one near-miss expect block, multi-key wherever the rule correlates; expected outputs derived by hand from SEMANTICS before the engine runs.
- **09:** a second event inside its `${lookback}` interval (suppressed); below threshold; `env != ${label}`.
- **07:** an unregistered app (alert), a registered app via `given table` (no alert), a null `appId` (`'missing'`, alert), a second grant inside 5 min (suppressed — see O2).
- Each addition is a spec-corpus change: `grammar_check` re-runs green and the LAW Δ block shows it. The blocks run green in S3.3 (the conformance runner).

## 7. MUST / MUST NOT

1. `plan` MUST NOT import `runtime` or `playback`; both build from `plan.App`.
2. A plain rule's body MUST see each accepted event exactly once, at its arrival; its key's `clock.Instance` MUST NOT hold a buffered event, and its `ConstructLate` MUST stay 0.
3. The rate limit MUST suppress an output with `Ts <= Anchor + D` (ties included), count it, and re-anchor only at an emitted output's `Ts`.
4. The anchor MUST live on `partition.Instance` and MUST be cleared by every reset (purge and panic).
5. The shard step MUST walk before and after its offers.
6. `Sync` MUST NOT start a goroutine, and MUST use the live router's event step and the live shard step — no copy of either.
7. A replay MUST have exactly one `Ingest`: `Sync`'s.
8. The router MUST read `Config.Now` exactly once at `Start` and once per event, and nowhere else.
9. An expression error MUST drop the event for its own rule only, counted in that rule's `Errored`.
10. Evaluating the filter on an event that does not match MUST NOT allocate (the body's row is a long-lived struct passed by pointer).
11. `Build` MUST refuse a pattern or window body with `ErrNotBuilt`; it MUST NOT be given a model with findings.

## 8. Tests (QA, written first, red on the stub)

1. **`plan.Build`** over examples 07 and 09: streams, schemas, delays, `Derived`; the rule's `Inputs`, `KeyAttr`, `TTL`, `OnArrival`; 09's `D` = 1 h 30 min from `${lookback}`; 01 (a pattern) → `ErrNotBuilt`.
2. **The filter body:** true, false and null filters (null-strict); 09's `int` → `float` widening; `ts` filled; a division by zero drops the event for that rule only — a second rule on the same stream still fires.
3. **The three §8 ports**, each naming its source check: (a) outputs at 0, 4 min, 10:00.000 with `first every 10 min` → [0], 2 suppressed, and 10:00.001 emits; (b) 0, 0, 10 min, 10 min + 1 ms → [0, 10 min + 1 ms], 2 suppressed; (c) key-less, 0, 4 min, 11 min → [0, 11 min], [4 min] suppressed.
4. **Q1's cases:** a lone event on a quiet stream with no `idle after` is output at its step, with no later event and no end; arrivals 11 min then 0 (bound 15 min, `first every 10 min`) → [11 min], 0 suppressed; an event behind the construct clock still reaches the body.
5. **Per key:** keys a and b at 0 both emit; a purge then a return → a fresh anchor; the step order — a key whose purge fell due before an arriving event is purged first (`Purged` counted).
6. **`Sync`:** no goroutine started (a goroutine count before and after; goleak arrives in S3.2b-2); outputs equal the live runtime's with one shard; §4's re-run lines.
7. **Live against replay** (§5), with its teeth.
8. **Allocation:** filter evaluation on a non-matching event on the `Sync` path, 0 allocations (S3.2b-2's exit 8 builds on it).

## 9. Annotations at G1

SEMANTICS §8 (a plain rule's outputs reach the rate limit in arrival order) and §9 (an on-arrival rule's activity is its arrival); SPEC §5.1; ARCHITECTURE §5 steps 5 and 7; ADR-0005 §3 (`Sync`); ADR-0006 §2 (the anchor on `Instance`).

## 10. Size

Plan 2 · replay through the runtime 1.5 · rate limits and ports 2 · material 1 · the on-arrival path 0.5 = **7 points**.

## 11. Open points for the review

- **O1 — an event far behind an idle-raised clock.** Idleness raises a quiet stream's contribution at wall speed; an accepted event whose time is more than `TTL` behind that clock (its source lagging beyond its lateness bound after a quiet spell) gets its alert, then the walk purges the key at once, so a second such event inside `D` alerts again: a duplicate alert, never a missed one. Options: accept and state it; or due-date an on-arrival key's TTL from `max(LastActivity, clock)`.
- **O2 — example 07's rate limit is one global anchor.** `UnknownAppConsent` is unpartitioned, so a second tenant's unknown-app grant within 5 min of the first tenant's is suppressed. This is SPEC §5.1 as written; whether the example should partition by `tenantId` is an owner question (a spec-corpus change).
- **O3 — a derived `ts` of another type.** The validator forbids selecting `ts` but does not check that a derived stream's `ts` is a `timestamp`; `Build` would write a timestamp into it.
