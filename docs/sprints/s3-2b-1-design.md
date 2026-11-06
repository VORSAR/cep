# S3.2b-1 — design note: the plan, replay through the runtime, rate limits (v2)

> **Historical — S3.2b-1, the plan, replay through the runtime, rate limits; merged in PR #14.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-06--s32bs-plan-check); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** **design** · [record](s3-2b-1-record.md) · reviews: [design: architecture](../reviews/s3-2b-1-design-architecture.md), [combined build: architecture](../reviews/s3-2b-1-combined-architecture.md), [break-it: QA](../reviews/s3-2b-1-breakit-qa.md). **← [S3.2a-2](s3-2a-2-record.md) · [S3.2b-2](s3-2b-2-record.md) →**

**What it covers:** S3.2b's items 1, 2, 6 and 9 ([card](history.md#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time)) and its exits 1 and 5, plus the owner's decisions that land here.

**The owner's answers:**
- at the plan check ([sprint-plan §5, "S3.2b's plan check"](decisions.md#owner-decisions-2026-11-06--s32bs-plan-check)): Q1 (a plain rule sees each event as its stream accepts it; a rate limit compares event times in arrival order), Q2 (S3.1's routine; this half of the split);
- after the architecture review ([§5, "S3.2b-1's design"](decisions.md#owner-decisions-2026-11-06--s32b-1s-design-after-the-architecture-review)): **D1** an on-arrival rule forgets a key on its own stream's admission watermark; **D2** example 07 is partitioned by tenant; **D3** a derived stream's `ts` declared other than `timestamp` is a deploy error with a code; **D4 (≠ R)** a select that can be null into a non-optional output attribute is a deploy error; **D5** D3 and D4 land in this sprint.

**Routine (owner):** QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it.

v2 answers the review ([s3-2b-1-design-architecture.md](../reviews/s3-2b-1-design-architecture.md): 2 blockers, 7 should-fix, 3 later) and QA's gaps G1–G7; §12 places each. MUST and MUST NOT lines (§7) are the build's contract.

## 0. What the code says (read first)

Read against `main` @ a5dbc2d.

1. **`validate.Check` returns the checked `*model.App`** (`internal/validate/validate.go:38`): a deep copy with every `${param}` slot resolved; a duration's effective value is in `Ms` and `Param` is cleared (`resolve.go:133`). `internal/plan` consumes it.
2. **`plan` may not import `runtime`; `runtime` and `playback` may import `plan`** (`tools/importcheck/rules.json`). `plan`'s and `playback`'s tests may read `.cep` programs through the parser (owner, 2026-11-06).
3. **`runtime.Rule` is already the routing table** (`internal/runtime/runtime.go:32`). A partition key is an attribute, so `KeyAttr` suffices.
4. **Every rule goes through partition's release buffer** (`Rule.Offer` buffers, `Rule.Advance` releases once the clock is past, D1 of S3.2a-1). There is no on-arrival path.
5. **The shard step is pure:** `shard.process(m)` applies, offers, walks, and appends to `sh.outbox` (`runtime.go:934`). The router's `event(a, e)` builds one message per shard with fresh copies of its advances and offers (`:744–746`, 1–2 allocations per event, measured by the review) and reads `cfg.Now()` once per event for an undrained app (`:714`); the internal ticker reads it once per tick (`:536`); `router.now` keeps a stamp monotone across apps (`:640`).
6. **`expr.Build` succeeds on nodes that cannot run yet** — text operators (S7), `count` (S8), `chain` (S4.1) — whose `Eval` always errors (`internal/expr/expr.go:204, 259, 281`; `registry.go:174`).
7. **A single-input rule's clock is its stream's contribution**, `max(admission, idle floor, virtual floor, punctuation, drain)` (`internal/clock/clock.go:85`), while stream lateness is judged on admission alone (`:100`). In a fixture with two or more streams the virtual floor lifts the contribution above admission (review F2, proven).
8. **Per-key marks — the `once` mark, rate anchors, the batch-id dedup set — live in the body's `OpState`** (ADR-0006 §2, annotated 2026-11-05 from S3.2a-1's design v2 §3), so a purge or a reset clears them.
9. **`math:round` takes and returns a `float`** (`internal/expr/registry.go:137`); example 09 widens nothing (QA G1).

## 1. `internal/plan` (item 1)

```go
// Package plan turns a checked model (validate.Check's result) into an app's executable description
// (ARCHITECTURE §5 step 1): the streams, the tables, and per rule its routing and its operator chain.
package plan

type App struct {
	Streams []Stream // declaration order: the index is the stream id (SEMANTICS §2.2)
	Tables  []Table  // declaration order: the index is the table id
	Rules   []Rule   // declaration order
}

type Stream struct {
	Schema    event.Schema // a source: its time attribute, or event.ArrivalTime for `time arrival`; a derived stream: event.ArrivalTime (D2 of S3.2a-1: its log line carries a top-level ts)
	Delay     int64        // d, ms: the bound; 0 for ascending and arrival
	IdleAfter int64        // ms; 0: none
	Derived   bool         // no time clause, produced by a rule
}

type Table struct {
	Name  string
	Attrs []event.Attr
	Key   []string
}

type Rule struct {
	Name      string
	Inputs    []int      // the stream ids it reads; exactly one when OnArrival
	KeyAttr   []int      // per input, the key attribute's index; nil: key-less (the pseudo-key "")
	KeyType   event.Type // TString for a key-less rule
	TTL       int64      // ms; 0 for a key-less rule
	OnArrival bool       // a plain body (owner Q1)
	NewBody   func(emit func(event.Event)) partition.Body
}

type Stats struct {
	Suppressed uint64 // suppressed_outputs (SEMANTICS §8)
	Errored    uint64 // events an expression error dropped for this rule (SEMANTICS §10)
}

// Counted is a body that reports its Stats; every body NewBody returns implements it.
type Counted interface{ Stats() Stats }

// ErrNotBuilt: a body kind or an expression this release cannot run yet.
var ErrNotBuilt = errors.New("plan: this body kind is not built yet")

// Build describes app, which MUST be validate.Check's result (no findings but lints).
func Build(app *model.App) (*App, error)
```

**One description, not three (review F6):** `runtime.Emit` becomes an alias, `type Emit = func(event.Event)`, and `runtime.Rule` an alias of `plan.Rule`; `playback.App`, `playback.Stream` and `playback.Table` become aliases of `plan.App`, `plan.Stream` and `plan.Table`, and `playback.FromPlan` is removed. `runtime.App` keeps its own shape (specs, schemas, tables, rules, sinks), made once by `runtime.FromPlan`.

**What `Build` does, in order:**
1. **Streams** as above (`Derived` = no time clause).
2. **Tables:** name, attributes, primary key.
3. **Rules.** A pattern or window body is `ErrNotBuilt`. A plain body:
   - every filter and select item is compiled with **`expr.BuildRunnable`**, a new `expr` function that returns a `*expr.NotRunnableError` (naming the construct and its sprint) when the tree holds a node this release cannot evaluate; `Build` wraps it in `ErrNotBuilt` (review F1). `plan` keeps no list of operator kinds.
   - `Inputs` = the source stream; `KeyAttr` = the partition key's index on it, or nil; `KeyType` (`TString` key-less); `TTL` = `Partition.TTL.Ms`; `OnArrival` = true.
   - The filter (none: always true). The projection: per target attribute other than `ts`, the select item bound to it, `int` widening to `float`. `ts`, if declared, holds the output timestamp.
   - The rate limit: `D` = `OutputFirstEvery.Ms` (0: none).
4. **The scope** given to `expr`: slot 0 is the source event; `Attr("", name)` its schema index; `Table(name)` the id and key type.
5. **`time arrival` sources** build; their live stamping is S3.2b-2's item 3.

**The validator's new rules (owner D3, D4; D5: this sprint)** — so `Build` is never given these cases:
- **D3:** a derived stream's attribute `ts` declared other than `timestamp` → **E-TYPE-001** at the attribute: "`<Stream>.ts` is filled with the output timestamp: declare it `timestamp`".
- **D4:** a select item that can be null bound to a non-optional attribute of its target → **E-NULL-003** (new) at the item: "`<item>` can be null and `<Stream>.<attr>` is not optional: wrap it in coalesce(...) or declare the attribute optional". "Can be null", conservatively (SEMANTICS §1.1): an `optional` attribute (a capture's included); arithmetic, `cast` and a function over a nullable argument (null propagates), except `coalesce` (nullable only when every argument is) and `ifThenElse` (nullable when either branch is); never a literal, a parameter, a comparison, `in` / `not in`, a text operator, `and` / `or` / `not`, `is [not] null`, `count()` or `chain(...)`. Whether a function propagates null is `expr`'s registry's to say, not a second list in `validate`.
- Each is a language change: errors.md, the catalogue, an invalid example, SPEC's change log. The corpus has no program either rule rejects (read 2026-11-06: every optional value selected is wrapped in `coalesce`).

## 2. The filter body and the emission stage (items 1, 6)

One body per (rule, shard), single-goroutine.

**`NewState(in)`** returns the key's state: `{anchor int64; anchored bool}` — the rate anchor lives here (§0.8, review F5). `Timers` yields nothing; `Walk` yields `("anchor", "", anchor)` when anchored (QA G6). A purge or a reset sets `Op = nil`, so the anchor goes with the key.

**`OnEvent(in, e)`:**
1. Copy `e` into the body's long-lived row (`b.row.ev = e`; `b.row.tables, _ = e.Tables.(*table.Snapshot)`); evaluate the filter on `&b.row` — a pointer, so nothing is boxed (proven by the review: 0 allocations by pointer, 1 by value).
2. An expression error: count `Errored`, return (this rule only). Null or false: return.
3. Project into freshly allocated `Values` (emission may allocate, ADR-0010 §1); widen `int` to `float`; `ts` = `e.Ts`. An error: count `Errored`, return. A null into a non-optional attribute cannot happen (D4).
4. Hand `event.Event{StreamID: target, Ts: e.Ts, Values: vals}` to the emission stage.

**The emission stage** (`plan`, generic; patterns and windows call it later with their own state's anchor):
```go
// Anchor is a rate limit's per-key mark; it lives in the body's OpState.
type Anchor struct{ At int64; Set bool }

// out applies `output first every D` (SEMANTICS §8): with D > 0, an output with Ts <= At + D (saturating) is
// suppressed and counted; otherwise it is emitted and At moves to its Ts.
func (m *emission) out(a *Anchor, e event.Event)
```
Ties are suppressed. A key-less rule has one instance, so one global anchor (RT-16). Under Q1 outputs arrive in arrival order; an output older than the anchor is inside the interval and suppressed.

## 3. `partition`: the on-arrival path (owner Q1, D1)

```go
type Config struct {
	// … as today …
	OnArrival bool // the body sees each event at Offer; nothing is buffered. Requires exactly one input.
}
```
`Instance` is unchanged (the stub's `Anchor` and `Anchored` fields are removed, review F5).

**`Offer` on an on-arrival rule:**
1. The same key checks (the router checks first).
2. The key's instance, created if absent with a `clock.Instance` for its TTL timer only, and entered in `byKey` and `byClock` **before** the body runs (review F11).
3. No construct-late test, no buffering: an accepted event is never dropped (Q1); `constructLate` is false.
4. Birth (`NewState`) if `Op == nil`; `LastActivity = max(LastActivity, e.Ts)`; a keyed rule re-arms its TTL timer at `LastActivity + TTL` (one per key, `Schedule` replaces); `body.OnEvent` under `recoverBody`.
5. `InstanceState.Born` is `Op != nil` for an on-arrival rule (a key-less one never schedules, so the clock never bears it; review F11).

**Which clock purges an on-arrival key (owner D1):** an on-arrival rule's clock is its stream's **admission watermark**, never a floor — except the drain level and +∞, after which nothing arrives. The shard applies to an on-arrival rule only `WMAdvance{StreamID: s, Contribution: admission(s), Moved: true}` for the event's own stream when its admission moved, and the drain's and end's advances; ticks and virtual floors never reach it. The registry gains `Admission(sid) int64` beside `Contribution` (an additive `clock` export, before S3.3's freeze).

**Why a purge now never changes an alert** (review F2): an accepted event has `t ≥` the admission before it, and its own advance gives an admission `≤ t`. A purge needs admission `> LastActivity + TTL ≥ Anchor + D` (SEMANTICS §11.3: TTL ≥ D), so it precedes only events the rate limit would emit anyway. Admission is identical live and in replay (SEMANTICS §2.1), so live and replay purge alike. **Cost, accepted (D1):** on a quiet stream keys stay until its next event or the stop — one small state per key.

**The shard step's order is apply → walk → offers → walk**, where the first walk runs only the on-arrival rules that receive an offer in the message (review F10). It fires a purge that fell due before the event, so the event starts a fresh instance. For buffered rules nothing changes. *(A key purged in that first walk is reborn as a new instance with a new ordinal; no output or counter differs.)*

## 4. The runtime with one shard, stepped synchronously (item 2)

```go
// Sync is the replay drive loop (ADR-0005 §3): one app on one shard, every step run to completion on the caller's
// goroutine through the live router's and shard's own steps. It starts no goroutine.
type Sync struct{ /* … */ }

// NewSync builds app on one shard. virtual: a fixture (the virtual floor; start ignored). out receives every output in
// emission order during the step that emits it; it MUST NOT call back into the Sync. App.Sinks is not used.
func NewSync(app App, start int64, virtual bool, out func(e event.Event)) (*Sync, error)

func (s *Sync) Begin(start int64)                                        // a recording's header: live ingest from start; before any event
func (s *Sync) Event(e *event.Event, arrival int64) (late bool, adv []clock.WMAdvance) // stamps e.Seq and e.Tables in place (review F3)
func (s *Sync) Tick(now int64) []clock.WMAdvance
func (s *Sync) TableWrite(table int, op TableOp, rows [][]event.Value) error
func (s *Sync) Drain() ([]clock.WMAdvance, DrainResult)                  // a recording's stop record
func (s *Sync) End() []clock.WMAdvance                                   // a fixture's end: +∞
func (s *Sync) Snapshot() *table.Snapshot
func (s *Sync) Registry() *clock.StreamRegistry
func (s *Sync) Walk(fn func(component, key string, state any))
func (s *Sync) Stats() Stats

func FromPlan(p *plan.App, tables *table.Set, sinks []SinkBinding) App
```

**The refactor:**
- **Routing fills reusable per-shard buffers** and calls `deliver(shard, view)` with views valid only during the call (review F4). The live `deliver` copies (a message crosses goroutines); `Sync`'s processes in place and flushes the outbox (`outbox[:0]`) to `out`. Ticks, table writes and drains get the same compute/deliver split.
- **The monotone stamp moves into `Ingest`** (per app): `Event` and `Tick` raise their stamp to at least the last one, so `Sync` and the router clamp alike (review F7). The router's cross-app clamp stays.
- `newShard` takes the app states, not `*Runtime`. `RuleStats` gains `Suppressed` and `Errored`, summed from each body's `Counted`.

**Playback over `Sync`:** the driver owns a `Sync` instead of its own `Ingest` and tables, so a replay has exactly one ingest, and `Driver.Step` keeps its contract (the stamped event, its advances, the snapshot). `NewDriver(app *App)` builds a `Sync` with no rules (the format layer's tests unchanged); `NewPlanDriver(p *plan.App, out)` one with the rules. A driver starts virtual and calls `Begin` at a recording's header (QA G7). `Driver.Stats()` returns the `Sync`'s (QA G4).

**The lines re-run through it:** S3.1's exit 4 and S3.2a's exits 1, 2 and 4, through `NewPlanDriver`.

## 5. Live against replay (exit 1)

1. A seeded generator writes a recording: 2–3 source streams (one with `idle after`; none `time arrival` until S3.2b-2), out-of-order times within each bound, some stream-late events, one table, ticks, a stop. Rules: a partitioned plain rule and a key-less plain rule with rate limits, an expression that can error (division by an attribute that can be 0), a nullable key, and S3.2a's buffered test-double body. **Each rule writes its own derived stream and projects its key**, so outputs are attributed per rule and key (review F8).
2. Live: `runtime.Start` with N shards, `TickEvery = 0`, `Config.Now` popping the recording's arrivals; table writes, events and ticks in file order; drain; every sink's outputs.
3. Replay: the same recording through `NewPlanDriver`.
4. Compare per rule and key the output sequences; across keys the multisets; and `StreamLate`, `ConstructLate`, `Suppressed`, `Errored`, `KeyErrors`, `Purged`.
5. N ∈ {1, 2, 4, 7}; 200 seeds in the `unit` row within its ceiling; a failing seed prints its recording.

**Teeth:** a body built from `NewBody`'s per-shard call order emits a shard-dependent value, which must fail the comparison for N > 1. The seams that are not shared are each pinned by a targeted test run both live and through `Sync`: a table record takes effect from the next event; the first walk precedes the offers; the router reads `Now` once per event (review F8).

## 6. Test material for 07 and 09 (item 9, QA, before the engine runs)

- **07 becomes per tenant (owner D2):** `partition by tenantId of ConsentGrants ttl 10 min`. Its blocks: an unregistered app (alert); a registered app via `given table` (no alert); a null `appId` (`'missing'`, alert); a second grant from the same tenant inside 5 min (suppressed); a second tenant's grant inside those 5 min (alert).
- **09:** a second event inside `${lookback}` (suppressed); below threshold; `env != ${label}`.
- Each change is a spec-corpus change: the parser's golden models, `grammar_check`, the grammar differential and the LAW Δ block. The blocks run green in S3.3.

## 7. MUST / MUST NOT

1. `plan` MUST NOT import `runtime` or `playback`; one description of an app (the aliases of §1).
2. An on-arrival rule MUST have exactly one input; its body MUST see each accepted event exactly once, at its arrival; its key's `clock.Instance` MUST NOT hold a buffered event; its `ConstructLate` MUST stay 0.
3. An on-arrival rule's clock MUST advance only on its stream's admission watermark, the drain and the end (D1).
4. The rate limit MUST suppress an output with `Ts <= At + D` (saturating; ties included), count it, and re-anchor only at an emitted output's `Ts`. The anchor MUST live in the body's `OpState`.
5. The shard step MUST run the on-arrival rules offered in a message through a walk before their offers.
6. `Sync` MUST NOT start a goroutine and MUST use the live router's and shard's steps (a review rule); `out` MUST NOT re-enter the `Sync`.
7. A replay MUST have exactly one `Ingest`: `Driver.Registry()` is the `Sync`'s.
8. `Config.Now` is read once by `Start`, once per event the router takes for an undrained app (stream-late included), and once per internal tick; nowhere else.
9. An expression error MUST drop the event for its own rule only, counted in that rule's `Errored`.
10. The replay path's event step MUST NOT allocate for a non-matching event of a resident key.
11. `Build` MUST return `ErrNotBuilt` for a pattern or window body and for any expression `expr.BuildRunnable` refuses; it MUST NOT be given a model with findings.

## 8. Tests

QA's 34 tests (red on the stub, on the integration branch) plus, written first in the build:
1. `expr.BuildRunnable` refuses a text operator, `count` and `chain`, and builds everything else.
2. The validator: E-TYPE-001 for a derived `ts` that is not a timestamp; E-NULL-003 for each nullable form into a non-optional attribute, and none for `coalesce`, a literal, a comparison, a non-optional attribute; the corpus stays clean.
3. D1: in a fixture with two streams, an event behind the rule's contribution but accepted by its stream — one key, `first every 5 min` — gives exactly the written rule's alerts (the review's scenario: [08:30], then a fresh alert only once the stream's own admission passes the TTL).
4. The re-runs of §4 not yet written (S3.2a exits 1 and 4 through `NewPlanDriver`).
5. `Sync` and live: a table record from the next event, and the first walk before offers, both live and through `Sync`; the monotone stamp clamps a direct `Sync` caller.
6. 07's tests follow D2 (its rule is keyed by tenant).

## 9. Annotations at G1

SEMANTICS §8 (a plain rule's outputs reach the rate limit in arrival order) and §9 (an on-arrival rule's activity is the largest event time seen, counted at arrival; its purge follows its stream's admission watermark); SPEC §5.1 and its change log (D3, D4); ARCHITECTURE §5 steps 5 and 7; ADR-0005 §3 (`Sync`); ADR-0006 §2 (on-arrival rules) and §4.

## 10. Size

Plan 2 · replay through the runtime 2 · rate limits and ports 2 · material 1 · the on-arrival path and D1 1 · D3, D4 in the validator 2 = **about 10 points** (was 7; the review's F1–F11 about +1, D1 +0.5, D3 and D4 +2).

## 11. Open points

None for the owner. Forwarded: whether a nullable value into a non-optional *table* or sink field needs the same rule — S3.3 (the facade's row boundary) and S5 (connectors).

## 12. Review and test-gap dispositions

| Id | Severity | Disposition |
|---|---|---|
| F1 | blocker | §1 step 3: `expr.BuildRunnable`; `Build` returns `ErrNotBuilt`; MUST 11; §8.1. 02 and 05 → `ErrNotBuilt` (QA). |
| F2 | blocker | Owner D1: on-arrival keys purge on their stream's admission (§3, MUST 3); §8.3. |
| F3 | should-fix | `Sync.Event` takes `*event.Event` (§4). |
| F4 | should-fix | Reusable per-shard routing buffers (§4); MUST 10; QA's whole-step allocation test stays here (G3). |
| F5 | should-fix | The anchor follows ADR-0006 §2's recorded placement: the body's `OpState` (§2); the stub's `Instance` fields go. |
| F6 | should-fix | Aliases (§1). |
| F7 | should-fix | MUST 8 restated; `TickEvery = 0` in §5; the stamp clamp in `Ingest` (§4); QA counts `Now` reads. |
| F8 | should-fix | §5: a derived stream per rule with its key; errors, null keys and their counters; seam tests live and through `Sync`. |
| F9 | should-fix | Owner D3 and D4 (§1, validator); a derived schema's `ArrivalTime` (§1); `time arrival` builds and is excluded from §5. |
| F10 | later | The first walk only for offered on-arrival rules; the ordinal effect stated (§3). |
| F11 | later | One input; maps before `OnEvent`; `Born`; saturating arithmetic; SEMANTICS §9 wording; tests in §8; MUST 6 and 7 as review rules with a pointer check; `out` never re-enters. |
| F12 | later | §10: about 10 points. |
| G1 | QA | `math:round` is float to float (§0.9); widening is tested on QA's own program. |
| G2 | QA | A derived stream's schema uses `ArrivalTime` (§1). |
| G3 | QA | The whole-step allocation test belongs here (F4). |
| G4 | QA | `Driver.Stats()` (§4). |
| G5 | QA | Partition's purge test drives `Apply` by hand; D1 changes what the shard applies, not partition's API. |
| G6 | QA | The body's `Walk` yields `("anchor", "", At)` (§2). |
| G7 | QA | A driver starts virtual and calls `Begin` at the header (§4). |

## 13. As built (annotated 2026-11-06, after the review and break-it)

- **E-PART-004** (break-it B1; owner): a stream named twice in a partition clause is refused at deploy, and `Build` refuses it too rather than keep the last key. Multi-part keys, `(k, j) of S` with no limit on the parts, are S3.2b-2's (owner).
- **D3's position:** the error names `ts` at the output stream's declaration; model attributes have no position. It moves to the attribute in S3.2b-2 (owner, ≠ R).
- **E-NULL-003 refinement** (break-it L1): a rule's own partition-key attribute is never null where the rule runs, so selecting it is no finding. Flow-sensitive cases (`ifThenElse(x is null, 0, x)`) stay refused: conservative by design, `coalesce` is the fix.
- **`Sync.Walk`** yields only the ingest step's Seq counters (combined review C5, later): the rules' state is walked through `partition.Rule`, the snapshot seam's concern in Phase 2.
- **The emission stage's reach** (C9): `pattern` and `window` cannot import `plan`; `plan` wraps their bodies (forwarded to S4.1). `partition.TimerFree.OnTimers` panics, so a stray timer is counted, never dropped.
- **Shared glue** (C4): the router and `Sync` share one tick, table-write and drain step.
- **Tests that guard without a red-first defect** (C1): the drain's and end's advances reaching on-arrival keys, and the admission-only message, each proven red by removing its line. The one-input check (C6) was written before its test; a mutation shows the test catches its removal.
