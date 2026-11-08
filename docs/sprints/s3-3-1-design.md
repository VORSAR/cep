# S3.3-1 — design note: the public Go API (v2, build-ready)

> **Historical — S3.3-1, the public Go API: `cep`, the connector contracts and the API gate; merged in PR #23.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-07--s33s-plan-check); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** **design** · [record](s3-3-1-record.md) · reviews: [design: breaker](../reviews/s3-3-1-design-breaker.md), [design: challenger](../reviews/s3-3-1-design-challenger.md), [design: architecture](../reviews/s3-3-1-design-architecture.md), [design: QA](../reviews/s3-3-1-design-qa.md), [combined build: architecture](../reviews/s3-3-1-combined-architecture.md), [break-it: QA](../reviews/s3-3-1-breakit-qa.md). **← [S3.2c](s3-2c-record.md) · [S3.3-2 (next)](sprint-plan.md#status) →**

**Status:** v2, after the blind panel and QA's testability review; the owner's decisions of 2026-11-07 applied. **Written so that a coding agent builds from this document alone:** every file is named, every signature is written out, every rule is a MUST with its test. Where this note and the code disagree, the code wins and the note is corrected (ground rule 21). v1 is in git (ce272a4) for the history; the panel's reports are [breaker](../reviews/s3-3-1-design-breaker.md), [challenger](../reviews/s3-3-1-design-challenger.md), [architecture](../reviews/s3-3-1-design-architecture.md), [QA](../reviews/s3-3-1-design-qa.md); §12 maps each finding to its section.

**What it covers:** items 1, 2 and 5 of the [S3.3 card](sprint-plan.md#s33--the-deploy-pipeline-and-the-conformance-runner--clock-freeze) and the forwards on it that touch the facade, as split by the owner ([plan check](decisions.md#owner-decisions-2026-11-07--s33s-plan-check), 1). Not here: the expect sandbox, the conformance runner, the baselines (S3.3-2); patterns (S4.1); every connector but `inmemory` (S5.1, S5.2).

**The owner's decisions this note carries out:** [the plan check](decisions.md#owner-decisions-2026-11-07--s33s-plan-check) (1 the split; 2 the export check and the clock golden as gate rows now; 3 `apidiff` now, `golang.org/x/exp` approved; 4 expect lists interleave by time — S3.3-2's; 5 the full loop with the panel) and [the design](decisions.md#owner-decisions-2026-11-07--s33-1s-design-after-the-panel) (1 a stopped app stays stopped; 2 `WithTableRows`; 3 the destination retries, never the engine; 4 `OutEvent.Canonical` is a fresh copy per output; 5 an unbuilt connector is refused unless Go binds one; 6 the batch-close signal is recorded now; 7 `apidiff` is red only when the golden did not change). Earlier decisions that bind: S3.1-2's D2 (a non-finite float is refused at the boundary) and B5 (an object enters as its JSON text); S3.2a-1's D1 (`Stop`'s drain is `clock.StreamRegistry.Drain`) and D7 (an `object` partition key is a deploy error); S3.1-1's D3 (ticks stop before the sources at `Stop`); S3.2a-2's F5 (a drain ends the app's input; annotated by design decision 1); S3.2b-1's L2 (`Deploy` never mutates the checked model); S3.2b-2's D1 (the Go boundary refuses what a recording cannot copy) and D4 (recording off unless turned on).

**Constraints from the gate that shape the code (read before writing a line):**
- `tools/archcheck` (ADR-0015 §2.2): a `go` statement, a channel type, send or receive are allowed only in `internal/runtime`, `io/*` and `cmd` (`tools/archcheck/main.go:279`, `:310-317`); `sync` and `sync/atomic` may be imported only there, in `io`, and under the named exceptions `internal/event` (`sync.Pool`) and `internal/table` (`sync.Mutex`, `atomic.Pointer`) (`:76-82`); the wall clock (`time.Now` and the other `clockCalls`) only in `internal/runtime`, `io`, `internal/playback` and `cmd` (`:253`). **So package `cep` has no goroutine, no channel, no mutex, no pool, no atomic and no clock call: it holds no shared mutable state; every lifecycle step runs inside `internal/runtime`, and the facade only converts, binds and forwards.** `context.WithCancel` is allowed anywhere (no channel operation in the caller's code), and `cep` uses it for the sinks' context (§3.6).
- The import law `tools/importcheck/rules.json`: `""` (package `cep`) may import `internal/{event,model,metrics,clock,pattern,window,expr,text,table,partition,runtime,plan,validate,parser,playback}` and `io`; after this sprint it may **not** import `internal/model` (§8). `io` imports nothing of the module; `io/*` imports `""` and `io` only. `internal/*` never imports `cep`, `io` or `io/*`.
- The runtime's `Send` **keeps** the values slice it is given (`internal/runtime/runtime.go:507-508`: "takes e.Values: the caller MUST NOT change them afterwards"); the router, the shards and the recorder read it after `Send` returns. Every row handed to `Send` is a fresh slice (§4).

**Routine (owner):** QA's tests first (QA owns every `_test.go` file and the proof drivers; the coordinator owns every production file, golden and registry entry); the build in the main session; the architecture review of the combined build and a QA break-it; one fix round under the race detector; T4.

## 1. Where it lands

| Path | What | Points |
|---|---|---|
| `internal/runtime/runtime.go`, `recorder.go`, `sync.go`, new `lifecycle.go` | the per-app lifecycle: `New`, `Add`, `Activate`, `Run`, `Drain` (changed), `Remove`, `Close` (changed), `BatchEnd`; `StreamStats.Batches`; `Recorder.BatchEnd`; `Sync.BatchEnd` (§2) | 3 |
| `internal/playback/recorder.go` | `Recorder.BatchEnd` writes the `batch_end` record the reader already defines (§2.9) | incl. |
| `cep.go`, `config.go`, `runtime.go`, `deploy.go`, `options.go`, `connectors.go`, `apphandle.go`, `input.go`, `row.go`, `table.go`, `sink.go`, `stats.go`, `diagnostic.go` (module root, package `cep`) | the facade (§3, §4) | 2.5 |
| `io/cepio.go`, `io/cepio_test.go` (package `cepio`) | the contracts (§5) | 0.5 |
| `io/inmemory/source.go`, `io/inmemory/sink.go` | the programmatic connector (§5.5) | 1 |
| `internal/parser/literal.go`, `internal/validate/validate.go` | `parser.ParseLiteral`; `validate.Options.Override` (§3.3) | incl. |
| `tools/apipartition/main.go`, `api/exports.golden`, `api/clock.golden`, `gate/runner/rows/apidiff.py`, `gate/registry.json` (rows `apipartition`, `clockgolden`, `apidiff`; `proofs` min_wants 19), `gate/proofs/drivers/test_api_planted.py`, `go.mod` (`tool` line), `tools/deps-allowlist.json` | the API gate (§6) | 2 |
| `internal/validate/catalog.go`, `internal/validate/testdata/*.cep`, `docs/language/errors.md`, `SPEC.md` (rev 7), `invalid/e-tbl-002.cep`, `invalid/e-part-008.cep` | D3's and D7's codes (§7) | 0.5 |
| `tools/importcheck/rules.json` | the import law for the new packages and their tests (§8) | incl. |
| `docs/architecture/adr/ADR-0001.md` §2, `ADR-0008.md` §1–§2, `ADR-0012.md` §3, §5, `ARCHITECTURE.md` §3; `SEMANTICS.md` §10 | annotations naming each departure (§3.8) | incl. |

About 9.5 points (the panel's re-sizing of v1's 8). One sprint, one PR, at most two agents at once.

## 2. The runtime's per-app lifecycle (`internal/runtime`)

Today `Start(cfg, apps)` builds every app before the router runs and `Close` stops everything. The facade needs apps to come and go while the runtime runs (ADR-0012 §3). Every existing entry point keeps its signature and behaviour; `Start` becomes `New` + `Add` + `Activate` per app, so the existing test suite is the guard (it stays green unchanged).

### 2.1 Signatures (new or changed; everything else in `runtime.go` is unchanged)

```go
// New starts the router, the shards and, with TickEvery > 0, the ticker, with no apps. Close stops them.
func New(cfg Config) (*Runtime, error)

// Start is New, then Add and Activate for each app in order, with context.Background(). Unchanged behaviour.
func Start(cfg Config, apps []App) (*Runtime, error)

// Add registers app, inactive, and returns its id (dense; never reused). The router installs it between two
// intake items; every shard builds its own state for it and acks; on any shard's error nothing is installed, the id
// is consumed (its slot stays nil) and the error is returned. Nothing accepts the app's input until Activate.
func (r *Runtime) Add(ctx context.Context, app App) (AppID, error)

// Activate starts the app's sink flushers and, with a Recorder, its recorder goroutine (the header, queued at Add,
// is its first record), then opens its input: Send, SendPayload, TableWrite, BatchEnd accept the app from here.
// Errors: ErrStarted (already active), ErrDrained (after Drain), ErrRemoved (after Remove).
func (r *Runtime) Activate(ctx context.Context, app AppID) error

// Run starts each run on its own goroutine with the app's source context, which Drain cancels first. Only an active
// app: ErrInactive, ErrDrained, ErrRemoved otherwise. A run's return value is reported by Drain.
func (r *Runtime) Run(ctx context.Context, app AppID, runs ...func(ctx context.Context) error) error

// BatchEnd hands a source's batch-close signal for stream to the intake, behind the events before it (owner,
// 2026-11-07, design 6). An empty batch id is an error (the recording's reader refuses one). After the drain it is
// refused with ErrDrained and counted in AfterDrain, as an event is.
func (r *Runtime) BatchEnd(ctx context.Context, app AppID, stream int, batch string) error

// Drain (signature unchanged). Active app: (1) the ticks stop for the app (an iQuiet item; S3.1-1's D3), (2) the
// source context is cancelled and every Run goroutine awaited until ctx's deadline, (3) the barrier as today.
// If a run has not returned by the deadline, Drain returns the stuck count wrapped in ctx.Err() without the
// barrier; the app is draining (its input refused), and a repeated Drain with a fresh ctx runs the barrier without
// waiting for the runs again. Inactive app: marked drained at once, no markers, DrainResult{}; nothing is recorded
// (no header was written). A repeated Drain after a completed barrier returns the stored result, as today.
func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error)

// Remove unregisters a drained or inactive app: the router drops its routes, every shard frees its state, each sink
// flusher and the recorder goroutine return after their last marker (the queues are never closed), the slot is
// nil. Removing an active app is an error; removing twice is ErrRemoved.
func (r *Runtime) Remove(ctx context.Context, app AppID) error

// Close drains every active app and removes every app in reverse id order, then stops the router, the shards and
// the ticker. Past ctx's deadline it signals every goroutine and returns ctx.Err(); a Write or a run that never
// returns keeps its goroutine. A second Close returns nil.
func (r *Runtime) Close(ctx context.Context) error

var (
	ErrInactive = errors.New("runtime: the app is not activated")
	ErrStarted  = errors.New("runtime: the app is already activated")
	ErrRemoved  = errors.New("runtime: the app is removed")
	// ErrClosed, ErrDrained unchanged.
)

type DrainResult struct {
	Pending int64   // unchanged: body timers left unfired by the barrier (drained_pending)
	Runs    []error // per Run goroutine, in Run order: its return, with context.Canceled read as nil
}

type StreamStats struct {
	Late, AfterDrain, Rejected, Unknown, FlagOverrides int64 // unchanged
	Batches                                           int64 // batch-close signals accepted (design 6)
}

// App gains one field; the others are unchanged.
type App struct {
	// … Streams, Schemas, Tables, Rules, Sinks, Recorder unchanged …
	// OnRemoved, if set, is called once, on the caller's goroutine of Remove or Close, after the app's goroutines
	// were asked to stop: the facade cancels the sinks' context here (§3.6). It is never called twice.
	OnRemoved func()
}

// Recorder gains one method (design 6). The playback recorder implements it (§2.9).
type Recorder interface {
	// … Header, Event, Rejected, Tick, Table, Stop unchanged …
	BatchEnd(stream int, batch string) error // in intake order, like Tick
}

// Sync gains the replay side of the same record, so replay's StreamStats equal live's.
func (s *Sync) BatchEnd(stream int, batch string)
```

### 2.2 State (in `lifecycle.go`; the fields are added to the existing structs)

```go
type appPhase int32

const (
	phaseInactive appPhase = iota // after Add
	phaseActive                   // after Activate
	phaseDraining                 // Drain begun (a.drained is still the router's view, set by the drain item)
	phaseRemoved                  // after Remove: the slot is nil
)

// appState gains:
//   phase     atomic.Int32          // appPhase; the caller-side fast path, read before every entry point
//   start     int64                 // the router's stamp when it installed the app: the Ingest's and the header's
//   srcCtx    context.Context       // the source context, made at Add; Drain cancels it
//   cancelSrc context.CancelFunc
//   runMu     sync.Mutex            // guards runs
//   runs      []*runState           // Run's goroutines
//   quiet     bool                  // the router's: ticks skip the app (D3)
//   batches   []int64               // per stream: StreamStats.Batches (the router's)
//   onRemoved func()                // App.OnRemoved
//   started   atomic.Bool           // set by Activate; the router reads it (a drain of a never-activated app)
//   waited    bool                  // Drain's caller-side: the runs were awaited once (a repeated Drain skips the wait)
// runState: done chan struct{} (closed when the run returned); err error (written before done is closed).
// sinkQ gains:    returned chan struct{} // closed by the flusher when it returns
// sinkItem gains: last bool              // a marker after which the flusher returns
// recItem gains:  last bool              // the same for the recorder goroutine; kind rMarker
// recQ gains:     returned chan struct{} // closed by the recorder goroutine when it returns
// Runtime: apps becomes atomic.Pointer[[]*appState] (copy-on-write by the router; callers Load a snapshot).
```

### 2.3 Items and messages

New intake item kinds: `iAdd{app App, reply}`, `iRemove{app AppID, reply}`, `iQuiet{app}`, `iBatchEnd{app, stream, batch}`. New shard messages: `mAdd{app AppID, spec addSpec}` (the app's `Streams`, `Schemas`, `Rules` — what `newShard` builds from today), `mRemove{app}`. Shards ack both with the existing `oAck`, which gains `err error`. Like a drain, an add or remove is **pending on the router under a token**: the router sends the message to every shard, keeps servicing the intake and handoff, and completes the item in `take` when the last ack arrives (`runtime.go:1037-1055` is the pattern to copy).

### 2.4 The operations, step by step

**`New(cfg)`:** `build(cfg, nil)` (the existing builder with no apps), then the router, the shards and the ticker goroutines exactly as `Start` starts them today (`runtime.go:283-301`); no flusher, no recorder.

**`Add(ctx, app)`, caller's goroutine:** `validateApp(app)` as `build` does today (`runtime.go:396-412`; the panic-free part). Enqueue `iAdd` with a reply; wait on the reply or ctx. **Router, on `iAdd`:** `start := rt.now(r.cfg.Now())`; build the `appState` as `build` does for one app (`NewIngest(app.Streams, start, app.Tables, false)`, the decoders, `byStream`, the sink queues with their `returned` channels, `recQ` with the header `recItem{kind: rHeader, arrival: start}` queued — not yet written) with `phase = phaseInactive`, `srcCtx, cancelSrc = context.WithCancel(context.Background())`, `start = start`; assign `id = len(*r.apps.Load())`; publish a new slice with the app appended **but no routes yet**; send `mAdd{id, spec}` to every shard under a token; park the reply. **Shard, on `mAdd`:** build its per-app slot — what `newShard` builds today per app (`runtime.go:1109-1151`: `rules`, `bodies` via `newBody`, `reads`, `arrival`, `burst`, `outHigh`, `panics`) — appending at index `id` (the shard's per-app slices grow to `id+1` with nil slots in between if ever needed); on error, free the slot and ack `oAck{token, err}`; else ack nil. **Router, in `take`, on the last ack:** if any ack carried an error: send `mRemove{id}` to every shard under a new token, and when those acks arrive publish the slice with `apps[id] = nil`, set `phase = phaseRemoved`, reply the first error; else install the routes (`byStream`) and reply `id`.

**`Activate(ctx, app)`, caller's goroutine:** `a := appOf(app)`; `phase.CompareAndSwap(inactive, active)` else `ErrStarted` (active), `ErrDrained` (draining), `ErrRemoved`; `r.wg.Add` and start each `sinkQ.run` and, if `a.rec != nil`, `recQ.run` (the header is written first, being first in the queue). No intake item is needed: the phase is an atomic the entry points read.

**`Run(ctx, app, runs...)`:** phase must be active (`ErrInactive`, `ErrDrained`, `ErrRemoved`); under `runMu`, for each run: `rs := &runState{done: make(chan struct{})}`, append, `r.wg.Add(1)`, `go func() { defer r.wg.Done(); rs.err = run(a.srcCtx); close(rs.done) }()`.

**`BatchEnd(ctx, app, stream, batch)`:** `batch == ""` → error; `appOf`; phase inactive → `ErrInactive`; draining → `ErrDrained`; stream not a source stream → error; enqueue `iBatchEnd`. **Router:** `a.drained` → `a.after[stream]++`; else `a.batches[stream]++`, `a.rec.put(recItem{kind: rBatchEnd, stream, batch})`. The Ingest is not touched (no stamp: the record has none).

**`Drain(ctx, app)`:** `a := appOf(app)`. **Inactive app:** `CompareAndSwap(inactive, draining)`; enqueue `iDrain` as today; the router's `drainApp`, seeing `a.started.Load() == false`, sets `a.drained = true`, discards `recQ`'s queued header (nothing written: `Records 0`, not `Failed`), and replies `DrainResult{}` with no markers. **Active app:** `CompareAndSwap(active, draining)` (an app already draining skips to the barrier step below). Then, once per app (`a.waited`, set under `runMu`): enqueue `iQuiet{app}` (router: `a.quiet = true`; `iTick` skips it); `a.cancelSrc()`; under `runMu` copy `runs`; wait each `rs.done` or `ctx.Done()`; `runs[i] = rs.err` with `context.Canceled` read as nil; count the ones not done as `stuck`. If `stuck > 0`: return `DrainResult{Runs: runs}` and `fmt.Errorf("runtime: app %d: %d source goroutine(s) did not return before the deadline: %w", app, stuck, ctx.Err())` — **no barrier** (ctx is spent, and the intake refuses the app's input from here, so nothing new enters). Else, and on every repeated `Drain`: the existing drain path (`iDrain`, the markers, the recording's stop, `runtime.go:589-626`), with `Runs` attached to the result; a repeated `Drain` after a completed barrier returns the stored result, as today.

**`Remove(ctx, app)`:** `a := appOf(app)`; the phase read on the caller's goroutine must be inactive or draining (active → error "runtime: app %d is active: drain it first"; removed → `ErrRemoved`). Enqueue `iRemove` with a reply; the router serialises every check that needs its own view. **Router:** if the slot is already nil → reply `ErrRemoved`; if `a.phase` is draining and `!a.drained` (a drain begun but its barrier not run) → reply the error "runtime: app %d: the drain is not complete: drain it again"; else drop the app's entries from `byStream`; send `mRemove{id}` under a token; **in `take` on the last ack:** if `a.started`, push one last marker per sink queue (`sinkItem{marker: m, last: true}`) and to `recQ` (`recItem{kind: rMarker, last: true}`) exactly as a drain pushes its markers (through the router's pending FIFO, `runtime.go:1046-1050`); publish the slice with `apps[id] = nil`; reply with the markers. **Shard, on `mRemove`:** free the slot (set every per-app entry at `id` to nil/zero; never `slices.Delete`, so indexes stay); ack. **Caller:** on a nil reply, `phase.Store(removed)`; wait each `q.returned` and the recorder's `returned` (none for an app never started) or ctx; call `a.onRemoved()` once (whatever the wait's outcome); return `ctx.Err()` if the wait expired.

**Flusher (`sinkQ.run`, `runtime.go:1297-1317`):** `defer close(q.returned)` after `defer r.wg.Done()`; the marker case becomes `if it.marker != nil { close(it.marker); if it.last { return }; continue }`. **Recorder goroutine (`recQ.run`, `recorder.go:111-140`):** `defer close(q.returned)`; `handle` of an `rMarker` with `last` flushes and makes `run` return (a `q.done` bool checked after `handle`).

**`Close(ctx)`:** `closeOnce` guards the whole body. For `id` from the highest down: `a := apps[id]`, skip nil; phase active → `Drain(ctx, id)` then `Remove(ctx, id)`; phase inactive or draining → `Remove(ctx, id)`; stop at the first `ctx.Err()`. Then `close(r.stopped)` (the router, the shards, the ticker and every flusher and recorder goroutine return at their next select), call `onRemoved` once for every app not yet removed (their sinks' context ends), and wait `r.wg` under ctx as today. Return the first ctx error seen, else nil.

**`Stats`:** the three per-app loops (`iTick` `:812`, `iStats` `:856`, `take`'s `oStats` `:1075`) iterate `*r.apps.Load()` and skip a nil slot, appending zero rows in `iStats` and `take` so `[app]` indexes never shift; `iTick` also skips `a.quiet`. `StreamStats.Batches` is filled from `a.batches`. The shard's `stats()` keeps a zero row for a removed app.

### 2.5 MUSTs

- **MUST 1:** the router is the only goroutine that writes the app list (copy-on-write through `r.apps`) and the routes; each shard is the only goroutine that reads or writes its own per-app state; `appOf` loads a snapshot and returns `ErrRemoved` for a nil slot. Test: `TestAddRemoveSendUnderLoadHasNoRace` under `-race`.
- **MUST 2:** ids are dense and never reused; a removed app's slot is nil; `Stats` keeps zero rows for it; `Tick` and `Stats` after a `Remove` never panic. Test: `TestRemovedAppKeepsItsSlot`.
- **MUST 3:** the entry points read the phase on the caller's goroutine: inactive → `ErrInactive`; draining → `ErrDrained`; removed → `ErrRemoved`; closed runtime → `ErrClosed` first. Test: `TestAppStateErrors`.
- **MUST 4:** `Add` starts no goroutine; `Activate` starts the flushers and the recorder; `Run` starts the sources. Test: `TestAddStartsNoGoroutine`.
- **MUST 5:** `Close` drains and removes in reverse id order, and when it returns nil every goroutine the runtime started has returned (goleak). Test: `TestCloseStopsAppsInReverseAddOrder`.
- **MUST 6:** a sink queue is never closed; a flusher returns only on its last marker or `r.stopped`; a `Remove` never races a pending send, because its last markers travel through the router's pending FIFO behind every output queued before them. Test: `TestRemoveDeliversNothingAfterTheFlush`, `TestRemoveWhileOutputsAreInFlight`.
- **MUST 7:** an add that fails on any shard installs nothing, consumes the id, and a following `Add` works. Test: `TestAddRollsBackWhenOneShardFails`.
- **MUST 8:** `Drain` of a never-activated app returns at once with `DrainResult{}` and records nothing. Test: `TestCloseWithAnInactiveApp` (wall time under the ceiling with a 5 s deadline).
- **MUST 9:** `Drain` cancels the source context and awaits the runs before the barrier; past the deadline it returns the stuck count wrapped in `ctx.Err()` without the barrier, the app's input is refused from then on, and a repeated `Drain` runs the barrier without waiting again. Tests: `TestDrainCancelsTheSourcesBeforeTheBarrier`, `TestDrainWithARunThatIgnoresCancel` (the first `Drain` wraps `context.DeadlineExceeded` and names 1; `Send` is `ErrDrained`; a second `Drain` with a fresh ctx returns the barrier's result).
- **MUST 10:** an added app's `Ingest` start and its recording header carry the same stamp, taken by the router at install. Test: `TestAddedAppStartIsItsHeaders` (an injected `Now`: t0 at `New`, t0+1h at `Add`; the double's `Header` argument is t0+1h).
- **MUST 11:** `BatchEnd` is recorded in intake order as the `batch_end` record, counted in `StreamStats.Batches`, refused with an empty id, counted in `AfterDrain` after the drain; replaying the recording gives the same `Batches`. Tests: `TestRecorderRecordsBatchEnd` (the double's log is `header, event, batch_end, stop`), `TestBatchEndRoundTrip` (playback: a `NewRecorder` recording with one `BatchEnd(0, "b1")` reads back as `Record{Kind: RecBatchEnd, Stream: 0, Batch: "b1"}`), `TestBatchEndReplaysToTheSameStats`.
- **MUST NOT:** no sleep, no polling anywhere in the package (test `TestNoSleepOrPollingInRuntime`: `go/parser` over the non-test files, no `time.Sleep/After/Tick` selector outside `tick()`).

### 2.6 The ticks and the sources (D3)

A per-app stop quiets the app's ticks (`iQuiet`) before the sources are cancelled, so no idle tick advances a quiet stream between the last source event and the barrier; a shutdown never fabricates an absence (S3.1-1's D3, ADR-0012 §3's annotation). Until S4.1 no absence exists, so no detection changes; the contract is frozen now and `TestStopQuietsTicksBeforeSources` asserts the order through a recorder double (no `tick` record after the `quiet` point — the double counts ticks after `Drain` began).

### 2.7 `Start(cfg, apps)` as built

```go
func Start(cfg Config, apps []App) (*Runtime, error) {
	r, err := New(cfg)
	if err != nil { return nil, err }
	for _, app := range apps {
		id, err := r.Add(context.Background(), app)
		if err != nil { _ = r.Close(context.Background()); return nil, err }
		if err := r.Activate(context.Background(), id); err != nil { _ = r.Close(context.Background()); return nil, err }
	}
	return r, nil
}
```
The existing tests pass unchanged (the guard). One behaviour moves: `Close` with an active app now drains it (today `Close` drops queued outputs, `runtime.go:1305-1307`). Tests that `Close` with a held sink and an expired context (`breakit_test.go:134`, `coverage_test.go:370`) already expect `context.DeadlineExceeded` and keep it; any other existing test that changes verdict under the new `Close` is adjusted in its own commit with the reason, never silently.

### 2.8 `Sync` (replay)

`Sync.BatchEnd(stream, batch)` counts `s.a.batches[stream]++` (after the drain: `after[stream]++`), so `Sync.Stats().Streams[0][stream].Batches` equals live. Every loop that drives a `Sync` from `playback.Replay` (the S3.2b-2 equivalence tests in `internal/playback`; grep `StepTick` for the callers) gains `case playback.StepBatchEnd: s.BatchEnd(st.Stream, st.Batch)`.

### 2.9 `playback.Recorder.BatchEnd`

```go
// BatchEnd writes {"batch_end":"<batch>","stream":"<name>"} — the record reader.go:38, :60 and :424 already read.
func (r *Recorder) BatchEnd(stream int, batch string) error
```
written with the same `appendString` and `streamName` the `Event` record uses (`recorder.go:63-105`). No format change: the reader, the driver (`StepBatchEnd`) and the trace already handle the record.

## 3. The facade: package `cep` (module root)

The facade converts, binds and forwards. It holds **no shared mutable state** (§0's constraint): `Runtime` wraps the engine's runtime; `AppHandle`, `InputHandler` and `Table` are immutable records of ids and schemas whose methods call the runtime, which owns every goroutine, every phase and every counter. All of them are safe for concurrent use because the runtime's entry points are.

### 3.1 Files and signatures

```go
// cep.go
package cep

type Info struct{ Module, Version, GoVersion string }

// BuildInfo reads runtime/debug.ReadBuildInfo; Version is "(devel)" outside a module build.
func BuildInfo() Info

// The errors a caller sees: the facade's own sentinels (archcheck's globals rule allows errors.New sentinels only, so
// the engine's values cannot be aliased); mapErr(err) wraps the engine's ErrClosed, ErrInactive, ErrStarted,
// ErrDrained and ErrRemoved onto them at every call, so errors.Is works against these.
var (
	ErrClosed            = errors.New("cep: the runtime is closed")
	ErrInactive          = errors.New("cep: the app is deployed but not started")
	ErrStarted           = errors.New("cep: the app is already started")
	ErrStopped           = errors.New("cep: the app is stopped")           // after Stop (owner, design 1)
	ErrUndeployed        = errors.New("cep: the app is undeployed")
	ErrUnknownStream     = errors.New("cep: no such stream")
	ErrUnknownTable      = errors.New("cep: no such table")
	ErrNotASourceStream  = errors.New("cep: not a source stream")
	ErrNotADerivedStream = errors.New("cep: not a derived stream")
	ErrSinkMode          = errors.New("cep: the sink does not accept the stream's output mode")
	ErrConnectorNotBuilt = errors.New("cep: the connector named in the program is not built yet")
	ErrRepeatedOption    = errors.New("cep: a deploy option was given twice")
	ErrNotAKeyColumn     = errors.New("cep: a delete key may set primary-key columns only")
	ErrRowUsedAfterFill  = errors.New("cep: the RowWriter was used after its fill returned")
)

// config.go
type Config struct {
	Shards      int              // 0: GOMAXPROCS
	Mailbox     int              // per shard, in messages; 0: 1024
	Intake      int              // the router's intake queue; 0: 1024
	SinkQueue   int              // per sink binding; 0: 1024
	RecordQueue int              // each recording's queue; 0: 65,536
	TickEvery   time.Duration    // live idle ticks; 0: none
	Now         func() time.Time // the wall clock; nil: the engine's default (the facade never calls time.Now)
}
func (c Config) engine() runtime.Config // copies the ints; Now != nil → func() int64 { return c.Now().UnixMilli() }

// runtime.go
type Runtime struct{ rt *runtime.Runtime }
func New(cfg Config) (*Runtime, error)                    // runtime.New(cfg.engine())
func (r *Runtime) Close(ctx context.Context) error         // r.rt.Close(ctx): every app stopped in reverse deploy order

// deploy.go
// Deploy parses, validates and plans src, binds its connectors and registers the app, inactive. Every parse or
// static defect is returned at once as a *DeployError; lints are on the handle. Expect blocks are not run in
// S3.3-1 (S3.3-2 adds the sandbox stage here).
func (r *Runtime) Deploy(ctx context.Context, name, src string, opts ...DeployOption) (*AppHandle, error)

// diagnostic.go
type Diagnostic struct {
	Code      string // "E-TIME-001"
	Line, Col int    // 1-based; 0, 0 for a diagnostic with no position (a deployment override)
	Message   string
	Fix       string
	Lint      bool
}
type DeployError struct {
	Name        string
	Diagnostics []Diagnostic // validate's order: line, column, code, message; a parse error is one diagnostic
}
func (e *DeployError) Error() string // "cep: deploy <Name>: <n> finding(s):\n  <code> <line>:<col> <message>" per line

// options.go
type DeployOption func(*deployOptions) error
func WithParam(name, literal string) DeployOption            // a ${param} override, written as the rule language writes it
func WithSource(stream string, s cepio.Source) DeployOption   // one per source stream; replaces the program's @source
func WithSink(stream string, s cepio.Sink) DeployOption       // any number per derived stream, in call order
func WithTableRows(table string, rows ...func(cepio.RowWriter) error) DeployOption // applied at Start, before any source opens (owner, design 2)
func WithRecording(w cepio.Writer) DeployOption               // the recording, off unless given (owner D4 of S3.2b-2); cepio.Writer: any io.Writer (cep may not import io)
func WithConnectors(f ConnectorFactory) DeployOption          // binds the program's @source/@sink annotations (§3.4)

// connectors.go
// ConnectorFactory builds the connector a program's annotation names. keys are the annotation's items other than
// `type`, each value as the rule language writes it (a string without its quotes; any other literal in its canonical
// text). An unknown type returns an error wrapping ErrConnectorNotBuilt.
type ConnectorFactory interface {
	Source(typ string, keys map[string]string) (cepio.Source, error)
	Sink(typ string, keys map[string]string) (cepio.Sink, error)
}

// apphandle.go
type AppHandle struct { /* unexported: the runtime, the AppID, the plan's names and schemas, the bound sources, the checked table rows, the lints */ }
func (a *AppHandle) Name() string
func (a *AppHandle) Lints() []Diagnostic
func (a *AppHandle) Start(ctx context.Context) error
func (a *AppHandle) Stop(ctx context.Context) (StopReport, error)
func (a *AppHandle) Undeploy(ctx context.Context) error
func (a *AppHandle) Input(stream string) (*InputHandler, error) // ErrUnknownStream; ErrNotASourceStream
func (a *AppHandle) Table(name string) (*Table, error)          // ErrUnknownTable
func (a *AppHandle) Stats(ctx context.Context) (Stats, error)

type StopReport struct {
	DrainedPending int64            // body timers left unfired by the barrier (drained_pending)
	SourceErrors   map[string]error // per bound source stream: Open's non-nil return, context.Canceled excluded; absent when none
}

// input.go
type InputHandler struct { /* unexported: the runtime, the AppID, the stream id, its schema */ }
// SendRow allocates the row, calls fill, checks it (§4) and hands it to the engine, which keeps it. It blocks for
// backpressure and respects ctx. batchID names the source batch the row belongs to; "" for none.
func (h *InputHandler) SendRow(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error
// EndBatch is the batch-close signal (§5.3): recorded and counted; batchID must not be empty.
func (h *InputHandler) EndBatch(ctx context.Context, batchID string) error

// table.go
type Table struct { /* unexported: the runtime, the AppID, the table id, its attrs, its key positions */ }
func (t *Table) Upsert(ctx context.Context, fills ...func(cepio.RowWriter) error) error // one write, many rows: one intake item, one version, one record
func (t *Table) Delete(ctx context.Context, keys ...func(cepio.RowWriter) error) error  // each key sets primary-key columns only
func (t *Table) Load(ctx context.Context, fills []func(cepio.RowWriter) error) error    // a multi-row Upsert (a replace-all is S3.3-2's given table)

// stats.go
type Stats struct {
	Streams   map[string]StreamStats // per source stream
	Rules     map[string]RuleStats
	Sinks     map[string][]SinkStats // per derived stream, its bindings in WithSink order (the program's sink first)
	Recording RecordingStats
}
type StreamStats struct{ Late, AfterDrain, Rejected, Unknown, FlagOverrides, Batches int64 }
type RuleStats struct{ ConstructLate, Purged, KeyErrors, Panics, Suppressed, Errored, OutboxHigh int64 }
type SinkStats struct{ Errors int64; QueueHigh int }
type RecordingStats struct{ Records int64; Failed bool; Err string }
```

### 3.2 `Deploy`, step by step

1. Apply the options into `deployOptions{params map[string]string; sources map[string]cepio.Source; sinks map[string][]cepio.Sink; tableRows map[string][]func(cepio.RowWriter) error; recording cepio.Writer; connectors ConnectorFactory}`. A second `WithParam` for one name, a second `WithSource` for one stream, a second `WithRecording` or `WithConnectors` returns an error wrapping `ErrRepeatedOption` naming the option and the name (not a `DeployError`: it is the caller's Go code, not the program). `WithSink` and `WithTableRows` may repeat.
2. `res, err := parser.Parse(name, src)`: an error is a `*DeployError` with one `Diagnostic` from `parser.Error` (its code, line, column, message, fix; `Lint false`).
3. `var opts validate.Options`; for each param override: `lit, err := parser.ParseLiteral(literal)`; an error is a `Diagnostic{Code: "E-PARAM-001", Line: 0, Col: 0, Message: "the deployment overrides <name> with <literal>, which is not a literal: <err>", Fix: catalog's}`; else `opts.Override(pname, lit)`. `checked, diags := validate.Check(res.App, opts)`; the non-lint diagnostics → `*DeployError` (every one, validate's order, each converted field by field: `Code`, `Line`, `Col`, `Message`, `Fix`, `Lint`); the lints → the handle's `Lints()`.
4. `p, err := plan.Build(checked)`: an error is returned as a plain error wrapping the plan's (`fmt.Errorf("cep: deploy %s: %w", name, err)`), never a `DeployError`: `plan.Build` fails only on a model `validate.Check` accepted, which is a bug, not a program defect, and no new code is added for it.
5. Bind (§3.4): the sinks per derived stream (`[]runtime.SinkBinding`, the program's annotation-built sink first, then `WithSink` in order); the sources per source stream; the tables' starting rows. Errors here are plain errors (`ErrUnknownStream`, `ErrNotADerivedStream`, `ErrNotASourceStream`, `ErrSinkMode`, `ErrConnectorNotBuilt`), wrapped with the stream or table name.
6. `app := runtime.App{Streams, Schemas, Tables, Rules, Sinks, Recorder}` built from the plan the way `internal/playback` does it (read `driver.go`'s `newDriver` and `NewPlanDriver`, and `internal/runtime`'s `build`, for how a `*plan.App`'s streams, tables and rules become `clock.StreamSpec`s, `event.Schema`s, a `*table.Set` and `runtime.Rule`s; the facade repeats that mapping in `deploy.go`); `Recorder = playback.NewRecorder(w, p)` only with `WithRecording`; `OnRemoved = cancelSinks` (§3.6). `id, err := r.rt.Add(ctx, app)`.
7. Return `&AppHandle{…}` with the plan's names (stream name → id, table name → id, rule names) and schemas, the bound sources, the table rows and the lints.

**MUST 12 (L2):** `Deploy` never holds or names the model: package `cep` does not import `internal/model` (the law's `""` row drops it, §8, so a `model.` reference is `archcheck`-red); the override is built through `validate.Options.Override(name, lit)` with `parser.ParseLiteral`'s result, its type never named; no field reachable from `*Runtime` or `*AppHandle` is a `*model.App` (test `TestHandleReachesNoModel`: a reflect walk over every field, func values excluded, asserts no value whose type's `PkgPath` ends in `/internal/model`). The plan holds the checked model for the app's lifetime and compiles its bodies from it on each shard at `Add` (`plan.go:135`, `:177-185`); that is the plan's, not the facade's.

### 3.3 Two small engine entry points

```go
// internal/parser/literal.go
// ParseLiteral parses one literal as a param default is written (SPEC §2.4; a signed number, a string, a duration,
// a timestamp, true/false), with the parser's own lowering, so an override and a default never differ.
func ParseLiteral(text string) (model.Literal, error)

// internal/validate/validate.go
// Override records a deployment override for name; the facade calls it without naming model.Literal (MUST 12).
func (o *Options) Override(name string, lit model.Literal)
```

### 3.4 Binding connectors

- A program's `@source(type = T, …)` on a source stream, and `@sink(type = T, …)` on a derived stream, name a connector type. `Deploy` resolves each through the `ConnectorFactory` (`WithConnectors`, else the default factory). **The default factory refuses every type** with an error wrapping `ErrConnectorNotBuilt` that names the type and the sprint that builds it (`http`, `stdin`, `jsonmap`: S5.1; `smtp`, `log`, `textmap`: S5.2); `inmemory` is a Go-side connector with no annotation form, so it is refused too ("bind it with WithSource/WithSink").
- **A Go binding replaces the annotation** (owner, design 5): `WithSource(stream, s)` on an annotated source stream, or any `WithSink(stream, s)` on an annotated derived stream, means the factory is not consulted for that stream. An annotated stream with no Go binding and a refusing factory is a `Deploy` error. So a plain-rule program carrying example 01's `@sink(type = 'http', …)` deploys with `WithSink("O", &inmemory.Sink{})` and not without. *(Corrected in the build: example 01 itself cannot deploy before S4.1 — its pattern body is not built — so the test carries the annotation on a plain rule.)*
- A source stream may be fed by its bound source and by `Input` at once. A derived stream may have any number of sinks. `WithSource` on a derived stream is `ErrNotASourceStream`; `WithSink` on a source stream is `ErrNotADerivedStream`; an unknown name is `ErrUnknownStream`.
- `Sink.Accepts()` is checked against the stream's output mode at `Deploy`: every stream a plain rule produces is `cepio.Append` (SPEC §5.7); a sink answering `cepio.Changelog` on it is `ErrSinkMode` wrapped with the stream name (`E-SINK-001` is the validator's code for the program's own annotation and is not reused).
- `WithTableRows(table, rows...)` on an unknown table is `ErrUnknownTable`; the fills run at `Deploy` and the rows are checked as `Table.Upsert` checks them (§4.4), so a bad row fails the deploy, not the start; the checked rows are kept on the handle and written at `Start`.

### 3.5 The lifecycle: `Start`, `Stop`, `Undeploy`

**`Start(ctx)`:** (1) `rt.Activate(ctx, id)` — the flushers and the recorder (the header first); (2) for each table with starting rows, in table order: `rt.TableWrite(ctx, id, tableID, runtime.TablePut, rows)` — recorded after the header, before any event (ADR-0012 §3: tables before sources; owner, design 2); (3) `rt.Run(ctx, id, runs...)` with one run per bound source: `func(ctx context.Context) error { return s.Open(ctx, emitterFor(stream)) }`. An error from (1) is returned as is (`ErrStarted`, `ErrStopped`, `ErrUndeployed`); from (2) or (3) it is returned and the app stays started (the caller may `Stop`).

**`Stop(ctx)`:** `res, err := rt.Drain(ctx, id)` — the runtime quiets the ticks, cancels the sources, awaits them, runs the barrier (§2.4). `StopReport{DrainedPending: res.Pending, SourceErrors: m}` where `m[stream] = res.Runs[i]` for each non-nil run error, in the facade's run order (the bound sources' stream order). `err` is returned as is (`ctx.Err()` wrapped: a stuck source, in which case the barrier did not run and `DrainedPending` is 0 until a repeated `Stop` completes it; or a barrier past its deadline). The report is filled as far as the runtime got. `Stop` on a stopped app returns the same report, nil (the runtime's stored result). `Stop` on an inactive app returns `StopReport{}`, nil, and the app is stopped (§2.4: a drain of a never-activated app).

**`Undeploy(ctx)`:** `Stop` if the app is active (a `Stop` error past the deadline is returned, nothing removed), then `rt.Remove(ctx, id)`; the runtime calls `OnRemoved` (the sinks' cancel, §3.6). `Undeploy` on an undeployed app is `ErrUndeployed`.

**`Start` after `Stop` is `ErrStopped`** (owner, design 1): a restart is a new `Deploy`.

### 3.6 The sinks' context

`Deploy` makes `sinkCtx, cancelSinks := context.WithCancel(context.Background())` (no channel operation in `cep`); the sink adapter (§5.2) calls `Write(sinkCtx, out)`; `runtime.App.OnRemoved = cancelSinks`. So `Write`'s context is cancelled when the app is removed — at `Remove`, after the drain's barrier and before the flushers are awaited, so a `Write` still blocked on its context returns and the last marker passes (as built; the app is being discarded) — or when `Close` gives up at its deadline, never before the drain — an earlier cancel would drop outputs the barrier promised. `Undeploy` removes a stopped app without a second `Stop` (a repeated drain would wait on a stuck sink's marker again); on an active app it stops first. A `Write` that never returns holds `Stop` to its deadline (`ctx.Err()`, every other output delivered) and `Close` returns `ctx.Err()` with that flusher still running: MUST 5 holds for sinks that honour their context. `Sink.Write`'s doc says: MUST return when ctx is done.

### 3.7 The transition table (every cell defined; test `TestLifecycleTable`, one row per cell)

| Call ↓ / state → | deployed (inactive) | started (active) | stopped (drained) | undeployed (removed) | runtime closed |
|---|---|---|---|---|---|
| `Start` | → started | `ErrStarted` | `ErrStopped` | `ErrUndeployed` | `ErrClosed` |
| `Stop` | → stopped; `StopReport{}` | → stopped; the report | the same report, nil | `ErrUndeployed` | `ErrClosed` |
| `Undeploy` | → undeployed | stop, then → undeployed | → undeployed | `ErrUndeployed` | `ErrClosed` |
| `Input`, `Table` | the handle | the handle | the handle | the handle | the handle (its use fails) |
| `SendRow`, `EndBatch`, `Upsert`, `Delete`, `Load` | `ErrInactive` | ok | `ErrStopped` | `ErrUndeployed` | `ErrClosed` |
| `Stats` | zero rows | ok | ok | zero rows | `ErrClosed` |
| `Runtime.Close` | remove | stop, remove | remove | skip | a second Close: nil once every goroutine the first left running has returned, its ctx's error otherwise (as built, break-it S2) |
| `Deploy` | — | — | — | — | `ErrClosed` |
| `Deploy` twice with one `name` | allowed: `name` labels diagnostics and recordings, it is not a key | | | | |

The lifecycle MUST NOT be called from inside a connector's `Open` or `Write`: it deadlocks to the deadline (documented on both).

### 3.8 Departures from the ADRs, named (each annotated in the ADR in this PR)

- ADR-0008 §1: `EmitFunc` (a func, batch id second) becomes the two-method `Emitter` (batch id first, `ctx` added), because the batch-close signal needs a second entry point; ADR-0012 §3's `SendRow(fill)` gains `ctx` and `batchID` for the same reason; `SetObject(attr, v any)` is `SetObject(attr, jsonText string)` (B5).
- ADR-0008 §2 and SEMANTICS §10: the flusher counts and drops; retry is the destination's (owner, design 3).
- ADR-0012 §5: `Delete(key any)` is `Delete(ctx, keys ...func(RowWriter) error)` (a composite key cannot cross as `any`; the one boundary); every table call takes `ctx` because a write waits for the router (owner F14).
- ADR-0001 §2: `cep` exports no `Event`; the public event view is `cepio.OutEvent` with `RowReader`, so a connector and an embedder read outputs through one type.
- ADR-0008 §5: params and connectors are per-app `Deploy` options, not `cep.Config`.
- ADR-0012 §3 and S3.2a-2's F5: `Start` after `Stop` is an error; a restart is a new `Deploy` (owner, design 1).

## 4. The value boundary (`row.go`)

`cepio.RowWriter` and `cepio.RowReader` are the only way values cross the facade; `internal/event.Value` never does (ADR-0008 §1, round-2 N3). The facade's `row` implements both.

### 4.1 The row

```go
// row.go
type row struct {
	attrs    []event.Attr  // the stream's schema attributes, or the table's attributes (§4.4)
	names    []string      // attribute names, for AttrIndex (in attrs order; built once per handle, shared)
	timeAttr int           // the stream's time attribute index; event.ArrivalTime for an arrival stream; -1 for a table
	values   []event.Value // fresh per SendRow/Emit/Upsert: the engine keeps it (runtime.go:507)
	set      []bool        // which attributes fill set
	err      error         // the first wrong Set, reported by SendRow
	armed    bool          // true only while fill runs; a Set afterwards panics with ErrRowUsedAfterFill
	keyOnly  []int         // Delete: the key positions; nil otherwise
}
```

**MUST 13 (the allocations):** one `SendRow`/`Emit` allocates at most three objects — the `row` header, its `values` and its `set` — and nothing else; the budget is `runtime.Send`'s own plus three on one shard, warm, measured. No pool: `cep` may not import `sync` (§0), and a row can never be reused because the engine keeps `values`. Test `TestSendRowAllocationsAboveSend`: `testing.AllocsPerRun(5000, …)` on `SendRow` with a hoisted `fill` minus the same on `runtime.Send` with a shared slice (QA measured `Send` at 2.00 on one shard) is at most 3.0; the measured figures are logged beside the assertion. (A later sprint may move the header into `internal/event`'s `sync.Pool` exception; not now.)

**MUST 14 (the engine owns the values once sent):** `SendRow` hands `row.values` to `runtime.Send` and never touches it again; after `fill` returns, `armed = false`, and any `Set` on the writer panics with `ErrRowUsedAfterFill` (a closure that kept the writer cannot write into an event the engine owns). Test `TestSendRowValuesAreTheEnginesOnceSent` (10,000 rows `"0"…"9999"` on one handler, the sink sees them in order, under `-race`); `TestRowWriterKeptPastFillPanics`.

### 4.2 `RowWriter` (the facade's implementation)

```go
func (w *row) SetString(attr int, v string)      { w.put(attr, event.TString, event.Value{Kind: event.TString, Str: v}) }
func (w *row) SetInt(attr int, v int64)          { … TInt, I: v }
func (w *row) SetFloat(attr int, v float64)      { … TFloat, F: v }
func (w *row) SetBool(attr int, v bool)          { … TBool, B: v }
func (w *row) SetTimestamp(attr int, unixMs int64) { … TTimestamp, T: unixMs }
func (w *row) SetDuration(attr int, ms int64)    { … TDuration, D: ms }
func (w *row) SetObject(attr int, jsonText string) { … TObject, Obj: jsonText }  // B5: the JSON text, never any
func (w *row) SetNull(attr int)                  // Value{Kind: the attribute's type, Null: true}
func (w *row) AttrIndex(name string) int         // -1 if unknown
func (w *row) AttrType(attr int) cepio.Type      // cepio.Invalid outside the schema
func (w *row) put(attr int, kind event.Type, v event.Value) // the checks of §4.3, first error kept in w.err
```

### 4.3 The checks, in this order (each an error to the caller naming the attribute; MUST 15, test `TestSendRowChecks`, one sub-test per row)

1. `!armed` → panic `ErrRowUsedAfterFill` (never an error: it is a programming fault).
2. `attr < 0 || attr >= len(values)` → `err = fmt.Errorf("cep: attribute index %d is outside the stream's %d attributes", attr, n)`.
3. `kind != attrs[attr].Type` → `err = "cep: attribute %q is declared %v; Set%v was called"`. `SetNull` on a non-optional attribute → `"cep: attribute %q is not optional; SetNull was called"`.
4. A `Delete` key fill: `attr` not in `keyOnly` → `err` wrapping `ErrNotAKeyColumn` with the attribute's name.
5. After `fill` returns nil: `err != nil` → return it. Then the unset check: a non-optional attribute never set → `"cep: attribute %q is not optional and was not set"`; an event-time stream's time attribute never set → `"cep: the time attribute %q was not set"`. A `time arrival` stream has no time attribute among its values (`event.ArrivalTime`): its stamp is the engine's, and a declared attribute named `ts` is an ordinary attribute.
6. Then `runtime.Send(ctx, app, event.Event{StreamID, Ts: values[timeAttr].T (or 0 for arrival), Values: values, BatchID: batchID})` — the engine's own checks follow (`runtime.go:517-544`, `boundary.go:20-43`): a NaN or infinite float (S3.1-2's D2), malformed UTF-8, an object that is not one compact JSON value within the log's limits, a timestamp outside years 0000–9999, a line over the recording's limit; each returned to the caller unchanged, so the facade adds only the index, type, null and unset checks (rows 2–5).

`fill`'s panic is not recovered (the caller's code, the caller's goroutine). A `fill` error is returned as is, nothing sent.

### 4.4 Table rows

`Table.Upsert`/`Load` rows are built on a `row` over the table's attributes (`timeAttr = -1`; `names`, types and `Optional` from the attrs; the key columns must be set — the runtime's `TableWrite` refuses a null key, and its error is returned as is). `Delete`'s key fill: `keyOnly` holds the key attributes' indexes in key order; the facade builds the runtime's delete row from those positions only (`runtime.go:560`: a delete row holds key columns in key order). All rows of one call go in one `runtime.TableWrite` (one intake item, one table version, one record). `WithTableRows` is checked the same way at `Deploy`.

### 4.5 `RowReader` (over an output's values; `sink.go`)

```go
type reader struct { names []string; attrs []event.Attr; values []event.Value }
func (r *reader) String(attr int) (string, bool)   // false: null, or not this type, or outside the schema
func (r *reader) Int(attr int) (int64, bool)
func (r *reader) Float(attr int) (float64, bool)
func (r *reader) Bool(attr int) (bool, bool)
func (r *reader) Timestamp(attr int) (int64, bool)  // unix ms
func (r *reader) Duration(attr int) (int64, bool)   // ms
func (r *reader) Object(attr int) (string, bool)    // the JSON text
func (r *reader) IsNull(attr int) bool              // true outside the schema too
func (r *reader) AttrIndex(name string) int
func (r *reader) AttrType(attr int) cepio.Type
func (r *reader) Len() int
```
Test `TestRowReaderNullsAndTypes`.

## 5. The connector contracts: `io` (package `cepio`)

```go
// io/cepio.go
// Package cepio holds the connector contracts (ADR-0008 §1): what a source, a sink or a mapper implements, and
// the row boundary values cross. It imports nothing of the module and starts no goroutine.
package cepio

import "context"

// Type is an attribute's declared type (SPEC §2.3), as RowWriter.AttrType and RowReader.AttrType report it.
type Type uint8

const (
	Invalid Type = iota // outside the schema
	String
	Int
	Float
	Bool
	Timestamp
	Duration
	Object
)

// Mode is what a sink accepts (SPEC §5.7): an append stream, or a changelog.
type Mode uint8

const (
	Append Mode = iota + 1
	Changelog
)

// RowWriter writes one row's typed values into engine-owned storage. It is valid only during the fill that
// received it; a Set after that fill returned panics. AttrIndex resolves a name once; AttrType says what Set to call.
type RowWriter interface {
	SetString(attr int, v string)
	SetInt(attr int, v int64)
	SetFloat(attr int, v float64)
	SetBool(attr int, v bool)
	SetTimestamp(attr int, unixMs int64)
	SetDuration(attr int, ms int64)
	SetObject(attr int, jsonText string) // an object enters as its JSON text
	SetNull(attr int)
	AttrIndex(name string) int // -1 if unknown
	AttrType(attr int) Type    // Invalid outside the schema
}

// RowReader reads one output's typed values. It is valid until the Write that received it returns; a sink that
// keeps a value copies it (strings and JSON texts are immutable Go strings and may be kept).
type RowReader interface {
	String(attr int) (string, bool) // false: null, or not this type
	Int(attr int) (int64, bool)
	Float(attr int) (float64, bool)
	Bool(attr int) (bool, bool)
	Timestamp(attr int) (int64, bool) // unix ms
	Duration(attr int) (int64, bool)  // ms
	Object(attr int) (string, bool)   // the JSON text
	IsNull(attr int) bool
	AttrIndex(name string) int
	AttrType(attr int) Type
	Len() int
}

// Emitter is a source's way into one stream. Emit allocates the row, calls fill, checks it and ingests it; EndBatch
// closes batchID (every row of the batch was emitted). Both block for backpressure and return ctx's error when it
// ends first; both are safe for concurrent use. batchID "" on Emit means no batch; EndBatch("") is an error.
type Emitter interface {
	Emit(ctx context.Context, batchID string, fill func(RowWriter) error) error
	EndBatch(ctx context.Context, batchID string) error
}

// Source feeds a stream. Open runs on a goroutine the engine starts after the app's tables are loaded and returns
// when ctx is done (the engine cancels it first at Stop and waits for Open to return until Stop's deadline); its
// return value is reported in StopReport.SourceErrors (context.Canceled excluded). Open MUST return when ctx is
// done, and MUST NOT call the app's Start, Stop, Undeploy or the runtime's Close.
type Source interface {
	Open(ctx context.Context, emit Emitter) error
}

// Sink receives one derived stream's outputs. Write is called from the engine's flusher goroutine only, one call at
// a time, in output order. A non-nil return is counted (Stats.Sinks) and the output is dropped: the engine never
// retries (owner, 2026-11-07); a sink that promises delivery retries itself. ctx is cancelled when the app is
// undeployed or the runtime's Close gives up: Write MUST return then, and MUST NOT call the lifecycle.
type Sink interface {
	Accepts() Mode
	Write(ctx context.Context, e OutEvent) error
}

// Writer is where a recording goes: any io.Writer satisfies it (the facade may not import io, ADR-0015 §2.2).
type Writer interface {
	Write(p []byte) (int, error)
}

// Mapper decodes one payload into a row (the http and stdin sources' mappers, S5.1).
type Mapper interface {
	Decode(b []byte, w RowWriter) error
}

// OutEvent is one output. Row is valid until Write returns. Canonical is the output's canonical JSON (ADR-0003/0005;
// the log sink prints exactly this text), a fresh slice per output the sink may keep (owner, 2026-11-07, design 4).
type OutEvent struct {
	Stream    string
	Timestamp int64 // unix ms
	Row       RowReader
	Canonical []byte
}
```

`io/cepio_test.go` (package `cepio`): `TestModeAndTypeValues` asserting `Append == 1`, `Changelog == 2`, `String == 1`, `Object == 7` (the coverage row needs one passing test in a package with no statements).

### 5.1 Sources, in the facade (`apphandle.go`, `input.go`)

`Start` passes one run per bound source to `runtime.Run`: `func(ctx context.Context) error { return s.Open(ctx, &emitter{h: inputHandlerFor(stream)}) }`. `emitter.Emit(ctx, batchID, fill)` is `h.SendRow(ctx, batchID, fill)`; `emitter.EndBatch` is `h.EndBatch`. The runtime starts the goroutine, cancels it at `Drain` and awaits it (§2.4).

### 5.2 Sinks, in the facade (`sink.go`)

For each binding the facade builds `runtime.SinkBinding{Stream: id, Sink: adapt(sinkCtx, stream, schema, s)}` where

```go
func adapt(ctx context.Context, stream string, sch event.Schema, s cepio.Sink) runtime.Sink {
	return func(e event.Event) error {
		canon, err := playback.AppendEvent(nil, sch, e) // a fresh slice per output (design 4)
		if err != nil { return err }                     // counted as a sink error: the encoder refused the output (S5.1's forward)
		return s.Write(ctx, cepio.OutEvent{Stream: stream, Timestamp: e.Ts, Row: &reader{…e.Values}, Canonical: canon})
	}
}
```
Test `TestCanonicalIsTheRecordingsText` (`Canonical` equals `playback.AppendEvent(nil, schema, e)`), `TestKeptCanonicalSurvivesTheNextWrite`.

### 5.3 The batch-close signal

A source that batches calls `EndBatch(ctx, batchID)` after the batch's last `Emit`. The facade forwards to `runtime.BatchEnd(ctx, id, stream, batchID)`: counted in `StreamStats.Batches`, recorded as the `batch_end` record (§2.4, §2.9; owner, design 6), refused after `Stop` with `ErrStopped` and counted in `AfterDrain`. A batch id is the string the source gives; the engine does not interpret it; `EndBatch` for an id no `Emit` carried is accepted. A recording without `batch_end` records (made before this sprint) replays with every batch closed at the drain — S8's reader treats their absence so. Test `TestEndBatchIsRecordedAndCounted`.

### 5.4 `AttrType` is here now (the architect's P1)

S5.1's JSON mapper must choose `SetInt`/`SetFloat`/`SetTimestamp`/`SetDuration` for a JSON number by the attribute's declared type; adding an interface method after the freeze would break every third-party implementation. Both interfaces carry `AttrType` from the start.

### 5.5 `io/inmemory` (package `inmemory`; imports `cep` and `io` only)

```go
// source.go
// Source is a channel-fed source for Go programs and tests. Push hands one row to the pump; the row's fill runs on
// the pump's goroutine (the one that called Open), so fill must not touch the pusher's state without synchronisation.
type Source struct { /* unexported: ch chan item; closed, dropped atomics */ }
func NewSource(buffer int) *Source                                              // buffer rows may wait for the pump; 0: unbuffered
func (s *Source) Push(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error // blocks until the pump takes it or ctx ends; ErrClosed after Close
func (s *Source) EndBatch(ctx context.Context, batchID string) error            // queued like a row, in order
func (s *Source) Close()                                                         // no more pushes; Open returns once everything pushed was emitted
func (s *Source) Dropped() int64                                                 // rows pushed and never emitted (Open's ctx ended first)
func (s *Source) Open(ctx context.Context, emit cepio.Emitter) error             // returns nil when closed and drained; ctx.Err() when ctx ends first (the rest counted in Dropped)
var ErrClosed = errors.New("inmemory: the source is closed")

// sink.go
// Sink collects outputs. Write copies nothing more than the engine already gave it (Canonical is the sink's) and
// reads Row's values into a Record before returning.
type Sink struct { /* unexported: mu sync.Mutex; recs []Record */ }
type Record struct { Stream string; Timestamp int64; Canonical []byte; Values []any } // Values: string, int64, float64, bool, int64 (ms), int64 (ms), string (JSON), or nil
func (s *Sink) Accepts() cepio.Mode                       // Append
func (s *Sink) Write(ctx context.Context, e cepio.OutEvent) error
func (s *Sink) Drain() []Record                            // a copy; the sink keeps its records
func (s *Sink) Reset()
```
`Open`'s loop: `select { case it, ok := <-s.ch: if !ok { return nil }; err := it.apply(ctx, emit) (Emit or EndBatch); if err != nil { s.dropped.Add(1 + len(s.ch)); return err } case <-ctx.Done(): s.dropped.Add(int64(len(s.ch))); return ctx.Err() }`. `Close` closes `ch` under a `sync.Once`; `Push`/`EndBatch` after `Close` return `ErrClosed` (a closed flag checked before the send; a send on a closed channel is prevented by the flag and the mutex). Tests: `TestSourcePumpsUntilClosed`, `TestCloseThenStopDeliversEverything` (push 1,000, `Close`, `Stop`: the sink saw 1,000, `Dropped() == 0`), `TestStopWithoutCloseCounts` (`Dropped() + sink count == 1000`), `TestPushAfterCloseIsAnError`, `TestSinkCollectsCopies`, `TestSinkAcceptsAppend`.

## 6. The API gate

### 6.1 `tools/apipartition` (Go, stdlib only; outside `archcheck`'s scope by the registry's `tools/**` exclusion)

**Usage:** `go run ./tools/apipartition [--write] <golden> <import path>...` — e.g. `go run ./tools/apipartition api/exports.golden . ./io ./io/inmemory` and `go run ./tools/apipartition api/clock.golden ./internal/clock`.

**What it does:** for each package directory (resolved from the import path relative to the module root), parse every non-test `.go` file with `go/parser` (`parser.ParseFile`, mode `parser.SkipObjectResolution`), and print one line per exported declaration, sorted, as `<package> <ident> <kind> <signature>`:
- `func`: every function whose name is exported, and **every method whose name is exported, whatever its receiver's exportedness** (a method of an unexported type is reachable through an exported function's result; the reviewer reads the receiver); the signature is the `FuncDecl` with `Doc` and `Body` set to nil, printed by `go/printer`, e.g. `cep func func (a *AppHandle) Start(ctx context.Context) error`.
- `type`: every exported type; the `TypeSpec` printed with its type, **exported fields only** for a struct (unexported fields removed from the `FieldList` before printing), e.g. `cepio type type OutEvent struct {⏎	Stream string⏎	…⏎}` with real newlines kept (one golden entry may span lines; the file is a sequence of entries, each starting at column 0 with the package name, continuation lines indented by a tab).
- `var`, `const`: every exported name with its type or value as written, e.g. `cep var var ErrClosed = runtime.ErrClosed`, `cepio const const Append Mode = iota + 1` (the spec's text, one line per name).
- **Refused (exit 2, red, naming the type):** an exported struct or interface type with an embedded field — its promoted methods are invisible to this text (the challenger's C5, the architect's A15); name the field instead.
- **Positions cleared before printing** (the breaker's S8), so a gofmt-valid reflow never changes a line:
```go
func canon(n ast.Node) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident: n.NamePos = token.NoPos
		case *ast.FieldList: n.Opening, n.Closing = token.NoPos, token.NoPos
		case *ast.FuncType: n.Func = token.NoPos
		case *ast.StructType: n.Struct = token.NoPos
		case *ast.InterfaceType: n.Interface = token.NoPos
		case *ast.StarExpr: n.Star = token.NoPos
		case *ast.ArrayType: n.Lbrack = token.NoPos
		case *ast.MapType: n.Map = token.NoPos
		case *ast.ChanType: n.Begin, n.Arrow = token.NoPos, token.NoPos
		case *ast.Ellipsis: n.Ellipsis = token.NoPos
		case *ast.BasicLit: n.ValuePos = token.NoPos
		case *ast.IndexExpr: n.Lbrack, n.Rbrack = token.NoPos, token.NoPos
		case *ast.IndexListExpr: n.Lbrack, n.Rbrack = token.NoPos, token.NoPos
		case *ast.TypeSpec:
			if n.Assign.IsValid() { n.Assign = 1 } // keep an alias's "=": a valid, position-free marker
		}
		return true
	})
}
```
- **Compare:** the printed text against the golden file's text. Equal → `GATE_COVERED <n>` (n = packages listed) and exit 0. Different → **one** line `failed: API Δ: +<added>/−<removed> (<pkg>.<ident>, …); <golden> differs: run go run ./tools/apipartition --write <golden> <paths> and review the diff` listing every added then every removed entry's `<pkg>.<ident>` in sorted order (a changed signature is one removed and one added), then `GATE_COVERED <n>`, exit 1. *(As built: one `failed:` line, because the runner keeps a reason's first line; a refusal exits 2, which `go run` reports as 1 — the `failed:` text tells the two apart.)* The `−` is U+2212, as the summary's convention writes it (ADR-0015 §3's `apipartition` row).
- `--write`: rewrite the golden and exit 0 (editing law: reviewed in the PR).
- **Tests** (`tools/apipartition/main_test.go`, over `testdata/fixture/` — a package with an exported func, an exported type with one exported and one unexported method, an exported struct with an exported and an unexported field, an exported var and const, an unexported func, a `_test.go` with an exported func, doc comments, a type alias, a generic func — and `testdata/fixture_reflow/`, the same declarations formatted differently): `TestExportsOfFixturePackage` (the exact expected text, no doc, no bodies, nothing from `_test.go`, the unexported field absent, the unexported type's exported method present), `TestCanonicalAcrossReflow` (byte-identical output for the two fixtures), `TestAliasKeepsEquals`, `TestEmbeddedFieldIsRefused` (`testdata/embedded/`), `TestDeltaMessage` (a golden with one extra and one missing name → exactly `API Δ: +1/−1 (fixture.Added, fixture.Gone)`), `TestWriteRegenerates`.

### 6.2 The goldens (law files)

`api/exports.golden`: `.`, `./io`, `./io/inmemory`, written by `--write` once the build is complete and reviewed line by line in the PR. `api/clock.golden`: `./internal/clock`, written the same way; **the clock contract freezes at S3.3-2's G5** (the card's item 7) — the row is blocking from now, so the freeze is the row already in place.

### 6.3 The rows (`gate/registry.json`; the row shape is the existing one: `id, job, tiers, blocking, timeout_s, needs, scope, expected_scope, min_wants, lands_in, exclusive, command`)

```json
{"id": "apipartition", "job": "lint", "tiers": ["T2", "T3", "T4"], "blocking": true, "timeout_s": 60, "needs": [], "scope": ["prod"], "expected_scope": ["go", ".", "./io", "./io/inmemory"], "min_wants": 3, "lands_in": "S3.3-1", "exclusive": false, "command": ["go", "run", "./tools/apipartition", "api/exports.golden", ".", "./io", "./io/inmemory"]}
{"id": "clockgolden", "job": "lint", "tiers": ["T2", "T3", "T4"], "blocking": true, "timeout_s": 60, "needs": [], "scope": ["prod"], "expected_scope": ["go", "./internal/clock"], "min_wants": 1, "lands_in": "S3.3-1", "exclusive": false, "command": ["go", "run", "./tools/apipartition", "api/clock.golden", "./internal/clock"]}
{"id": "apidiff", "job": "lint", "tiers": ["T4"], "blocking": true, "timeout_s": 300, "needs": [], "scope": ["prod"], "expected_scope": ["go", ".", "./io", "./io/inmemory"], "min_wants": 3, "lands_in": "S3.3-1", "exclusive": true, "command": ["python3.15", "-I", "-B", "-X", "utf8", "gate/runner/rows/apidiff.py", ".", "./io", "./io/inmemory"]}
```
The `proofs` row's `min_wants` becomes 19. The `expected_scope` kind `go` and the covered count are checked by the runner as for `unit` (`gate/runner`'s scope expansion); the golden files are read by the command, not scope items. `apidiff` is `exclusive` because it adds a git worktree.

### 6.4 `apidiff` (`go.mod`: `tool golang.org/x/exp/cmd/apidiff`; row `gate/runner/rows/apidiff.py`)

`go get -tool golang.org/x/exp/cmd/apidiff@<pinned>` adds the `tool` line and four `// indirect` requires (`golang.org/x/exp`, `golang.org/x/mod`, `golang.org/x/sync`, `golang.org/x/tools`); the row runs `go tool apidiff`. **The row, step by step** (the shape of `gate/runner/rows/gotest.py`: `rows.start()`, `rows.run_tool`, `rows.covered`, `rows.finish`):
1. `base = git merge-base HEAD origin/main` (fall back to `main`); none (a one-commit clone) → the row **passes** with the reason `no merge base with main: nothing compared` and `GATE_COVERED <n>` (not a skip: ADR-0015 §1.2, "a skipped gate is never a passed gate" — this is a stated pass with nothing to compare, and the proof `test_no_merge_base` pins it).
2. `git worktree add --detach <row dir>/base <base>`; `defer git worktree remove --force`.
3. For each package `p` in the arguments: in the base worktree `go tool apidiff -w <row dir>/<name>.export <import path of p>` (one package per `-w`: the tool takes exactly one; the import path is `github.com/vorsar/cep` for `.`, `github.com/vorsar/cep/io` for `./io`, …); a package absent at the base (the first run of this sprint) → that package reports `new package` and counts as compatible.
4. Here: `go tool apidiff <row dir>/<name>.export <import path>`; capture stdout. **The exit code carries no verdict** (0 on an incompatible change); the verdict is the text: a line `Incompatible changes:` → incompatible.
5. `golden_changed = git diff --quiet <base> -- api/exports.golden` is non-zero.
6. Red when any package is incompatible and `!golden_changed`, with the reason `incompatible API change in <import path> without api/exports.golden changing: <the report on one line, its lines joined by "; ">` (the runner keeps a reason's first line only); red too when `apidiff` itself fails at the merge base or at HEAD (a package that does not build: its empty report must not read as no change); otherwise pass, with every report printed in the row's output (the reviewer reads it in the PR). `GATE_COVERED <packages compared>`. *(As built after QA's second lane, F2–F4.)*
7. **Tests** (`gate/runner/tests/test_apidiff.py`, pytest in the `pytests` row): `test_no_merge_base`, `test_incompatible_without_golden_is_red`, `test_incompatible_with_golden_passes`, `test_compatible_passes`, each on a fixture repo built in the test's temp dir with one commit on `main` and one change.

`tools/deps-allowlist.json` gains the four `go.mod` requires at scope `build-time (apidiff, T4 only)`, approved 2026-11-07 (the owner's plan-check decision 3), each with its pinned version from the `go get`, and one entry listing the seven graph-only modules `go mod graph` names (`golang.org/x/net`, `x/sys`, `x/telemetry`, `github.com/google/go-cmp`, `github.com/yuin/goldmark`, `golang.org/x/tools/go/expect`, `golang.org/x/tools/go/packages/packagestest`; as built — the review's count) with scope `module graph only, never built`. The `deps` row is S1.1c's; the file is law now (ADR-0015 §3).

### 6.5 The proofs (`gate/proofs/drivers/test_api_planted.py`; the `support.DriverTest` shape of `test_tripwire_planted.py`)

- `test_an_unlisted_export_turns_apipartition_red`: the world copies the gate, `go.mod`, `go.sum`, `api/`, `cep`'s files, `io/`, `internal/`; plants `func Frobnicate() {}` in a new `cep/zz_planted.go`; runs the `apipartition` row at T2; asserts red with the reason containing `API Δ: +1/−0 (cep.Frobnicate)`.
- `test_a_clock_signature_change_turns_clockgolden_red`: plants one extra parameter on an exported `internal/clock` function (`NewStreamRegistry`: read the file for its current signature); asserts the `clockgolden` row red with `API Δ: +1/−1 (clock.NewStreamRegistry, clock.NewStreamRegistry)`.
- `test_a_signature_change_without_the_golden_is_red`: a world with a `main` branch at the copied tree and one commit changing `AppHandle.Start` to `Start(ctx context.Context, extra int)`; the `apidiff` row at T4 red, the reason containing `Incompatible changes:` and `Start`.
- `test_with_the_golden_regenerated_it_passes_and_prints`: the same change plus `--write`; the row passes, its output containing the same report line.
The row's `min_wants` 19 counts this file.

## 7. D3's and D7's deploy errors (`internal/validate`, `docs/language`)

| Code | Title | Rule (errors.md) | Fix (catalog.go and errors.md, verbatim equal) | Test program (`internal/validate/testdata/`) | Invalid example |
|---|---|---|---|---|---|
| `E-TBL-002` | Primary-key column optional or object **[static]** | SPEC §4.3 — a `primary key (…)` column is neither `optional` nor `object`: a null or an object cannot key a row (S3.1-2's D3) | declare the key column non-optional with a scalar type, or key the table by another column. | `e-tbl-002-optional-key.cep` (`-- expect: E-TBL-002 (…) @ L:C`), `e-tbl-002-object-key.cep` | none: `docs/language/invalid/` holds parse-level counter-examples only (Appendix C); a static code's counter-example is its validator test program |
| `E-PART-008` | Partition key of type object **[static]** | SPEC §4.4 — a partition key part is a scalar attribute; an `object` attribute cannot key a partition (S3.2a-1's D7) | key the partition by a scalar attribute. | `e-part-008-object-key.cep` | none (as above) |

- `internal/validate/catalog.go`: two entries, in the table's order; `internal/validate`'s checker: the table check in the pass that reads `primary key` (E-TBL-001's), the key check in `bounds.go`'s `partition` after E-PART-003's type check; each one finding at the column's or the part's position.
- `docs/language/SPEC.md`: §4.3 and §4.4 gain one sentence each; Appendix C's count 65 → 67; **rev 7** in §0 and the change log: "v0.1 rev 7 (2026-11-07, pre-release; sprint S3.3-1): two deploy errors — `E-TBL-002` (S3.1-2's D3) and `E-PART-008` (S3.2a-1's D7); the grammar is unchanged." `DESIGN.md`'s log gains the line.
- `partition.NewRule`'s refusal of an object key stays as the engine's guard (`TestNewRuleRefusesAnObjectKey`).
- The existing harnesses make both red-first: `TestStatic` wants exactly one finding per program; the catalogue-coverage test wants a program per code; `TestDocsyncStaticCatalog` wants errors.md's entry; the grammar differential runs the two invalid files.

## 8. The import law (`tools/importcheck/rules.json`, exact edits)

- `"imports"`: the `""` row drops `"internal/model"` (MUST 12); every other row unchanged. `io/inmemory` matches the existing `io/*` row (`["", "io"]`).
- `"test_imports"` gains `"": ["io/*"]` — the facade's external tests (`package cep_test`) use `io/inmemory`; an internal `package cep` test cannot (an import cycle).
- `"test_external"` gains `"": ["go.uber.org/goleak"]` and `"io/*": ["go.uber.org/goleak"]`.
- `tools/archcheck` needs no change: `tools/**` is outside its scope by the registry's `expected_scope` exclusion.
- `tools/deps-allowlist.json`: §6.4.

## 9. The tests (QA first, red before the build)

[QA's review](../reviews/s3-3-1-design-qa.md) §1 is the test table (T1–T41: package, file, name, setup, assertion, how it is red first, the gate row, goleak). It stands, with these amendments from v2; the tests-first lane works from both documents:

| QA row | Amendment |
|---|---|
| T1–T9 (`internal/runtime`) | as written, plus the tests §2.5 names that QA's table lacks: `TestRemoveDeliversNothingAfterTheFlush`, `TestRemoveWhileOutputsAreInFlight` (MUST 6), `TestAddRollsBackWhenOneShardFails` (MUST 7), `TestCloseWithAnInactiveApp` (MUST 8), `TestDrainCancelsTheSourcesBeforeTheBarrier`, `TestDrainWithARunThatIgnoresCancel` (MUST 9), `TestAddedAppStartIsItsHeaders` (MUST 10), `TestRecorderRecordsBatchEnd`, `TestBatchEndReplaysToTheSameStats` (MUST 11), `TestStopQuietsTicksBeforeSources` (§2.6); and in `internal/playback`: `TestBatchEndRoundTrip`. T4's table gains `Run` before `Activate` → `ErrInactive`, `Remove` of an active app → error. |
| T12 | as QA's F3 fix: `TestHandleReachesNoModel` (reflect walk) + the `archcheck` row on the `""` row without `internal/model`. |
| T14 | `Start` after `Stop` asserts `ErrStopped` (owner, design 1); `Undeploy` without `Start` succeeds (§3.7). |
| T19 | `errors.Is(err, cep.ErrSinkMode)`; the annotation case is T20. |
| T20 | example 01 without a Go sink → `errors.Is(err, cep.ErrConnectorNotBuilt)`, the message naming `http` and `S5.1`; with `WithSink("AgentKillSwitch", …)` it deploys (owner, design 5). Plus `TestRepeatedOptionIsAnError` (`WithSource` twice on one stream, `WithParam` twice on one name, `WithRecording` twice → `ErrRepeatedOption`). |
| T21 | the arrival-stream row is dropped (§4.3: an arrival stream has no time attribute); rows added: `SetNull` on a non-optional attribute; a `Delete` key fill setting a non-key column → `ErrNotAKeyColumn`; a `RowWriter` kept past `fill` → panics `ErrRowUsedAfterFill` (`TestRowWriterKeptPastFillPanics`). |
| T22 | the budget is `runtime.Send`'s + 3 (MUST 13), the measured figures logged. |
| T25 | `Load` is a multi-row upsert (one version, one record); `Upsert`/`Delete` take several fills; one call = one `TableWrite`. |
| T28 | `Stats().Sinks["<stream>"][i].Errors`, per binding (two sinks on one stream, one failing). |
| T29 | `Canonical` is a fresh slice: `TestKeptCanonicalSurvivesTheNextWrite` (keep the slice from output 1, write output 2, the kept bytes are unchanged). |
| T30 | `EndBatch` is recorded (the recording holds `{"batch_end":"b1","stream":…}` after the batch's last event) and counted (`Streams[s].Batches == 2`); `EndBatch("")` is an error; `EndBatch` after `Stop` → `ErrStopped` and `AfterDrain` counts it (owner, design 6). |
| T31 | `inmemory`: the §5.5 tests (`TestCloseThenStopDeliversEverything`, `TestStopWithoutCloseCounts`, `TestPushAfterCloseIsAnError`, …). |
| T33 | the §6.1 tests (`TestCanonicalAcrossReflow`, `TestAliasKeepsEquals`, `TestEmbeddedFieldIsRefused`, the unexported type's exported method present, the unexported field absent). |
| T34–T35 | the exact reason texts of §6.1 and §6.4; `test_no_merge_base` asserts a pass with the stated reason. |
| T38 | the deterministic half in `internal/runtime` with a parked body; the facade half with the four rules and positive witnesses (QA's F11); no "facade table mutex". |
| new | `TestWithTableRowsAreVisibleToTheFirstEvent` (owner, design 2): a `not in` rule, `WithTableRows` with one row, a source whose `Open` emits at once; exactly the right output. `TestCloseCancelsTheSinksContext`: a sink that blocks until its ctx ends; `Stop` with a short deadline returns `DeadlineExceeded`, `Undeploy` returns nil and the sink saw ctx done (§3.6 as built). `TestOpenErrorIsReported`: a source whose `Open` returns an error at once → `StopReport.SourceErrors[stream]` holds it. `TestLifecycleTable` (§3.7). `TestModeAndTypeValues` (`io`). |

Ownership: QA writes every `_test.go` file above and the two proof/pytest files in the worktree `cep-wt/s3-3-1-qa` (branch `s3-3-1/qa`), red against the stubs the coordinator commits first (§10 step 1); the coordinator cherry-picks them onto the integration branch before the build and never edits them except to fix a compile error against the real signatures, each such edit named in the record.

## 10. The build, in order (the coordinator, main session; each step its own commit on `s3-3-1/integration`)

1. **Stubs for the compiler** so QA's tests compile and fail: the new runtime signatures returning `errors.New("not built")`; `cep`, `io`, `io/inmemory` with every exported name of §3–§5 and panicking bodies; `tools/apipartition` with `main` only; the two codes in `catalog.go` without a check. `rules.json` per §8. Commit; T0 (`archcheck`, `gofmt`) green.
2. **`internal/runtime`**: `lifecycle.go` (§2.2–2.4): `New`, `Add` and `mAdd` with rollback, `Activate`, `Run`, `Drain`'s new steps, `Remove` and `mRemove`, the flusher's and recorder's last markers, `Close`, the nil-slot loops, `BatchEnd`, `StreamStats.Batches`, `Recorder.BatchEnd`; `Start` rewritten (§2.7); `Sync.BatchEnd` (§2.8). `playback.Recorder.BatchEnd` (§2.9) and the replay loops' `StepBatchEnd` case. Run `go test -race ./internal/runtime ./internal/playback` (the existing suite is the guard; QA's T1–T9 go green).
3. **`internal/parser.ParseLiteral`, `validate.Options.Override`**, then the two codes with their checks, test programs, invalid examples, errors.md, SPEC rev 7 (§7).
4. **`io/cepio.go`** (§5, verbatim), its test.
5. **`cep`**: `config.go`, `runtime.go`, `diagnostic.go`, `options.go`, `connectors.go`, `deploy.go`, `row.go`, `input.go`, `table.go`, `sink.go`, `stats.go`, `apphandle.go`, `cep.go` (§3–§4). Then `io/inmemory` (§5.5). QA's `cep_test` and `inmemory` tests go green; `go test -race` on both.
6. **`tools/apipartition`** (§6.1) and its tests; `--write` the two goldens; review them line by line against §3–§5 (every name in the goldens is in this note, and the reverse).
7. **The rows** (§6.3), `go get -tool …` and the allowlist (§6.4), the `apidiff` row and its pytests, the proof driver (§6.5); `proofs` min_wants 19.
8. The ADR annotations (§3.8), `SEMANTICS §10`'s retry sentence, `ARCHITECTURE §3`'s `io/` line.
9. T2 (`make check`), T3 (`make combine-check`): exit 0, read from the gate's own line.
10. The record: measured results, the owner's check (the 20-line program of the card: `cep.New`, `Deploy` of example 09 with `WithSink`, `Start`, two `SendRow`s, `Stop`, print the sink's `Canonical`; committed as `Example_embed` with its `// Output:`).

Then step 7 of the loop (the combined review and the break-it), one fix round under `go test -race` on the touched packages, T4, the PR.

## 11. Size

About 9.5 points: the runtime 3; the facade 2.5; the contracts and `inmemory` 1.5; the API gate 2; the codes 0.5.

## 12. The panel's findings → this note

| Finding | Where |
|---|---|
| Breaker B1 / challenger C2 / QA F1 / architect A17 (the pooled row) | §4.1 MUST 13–14 |
| Breaker B2 (the app list race) | §2.2, §2.5 MUST 1 |
| Breaker S3 (nil slots) | §2.4 `Stats`, MUST 2 |
| Breaker S4 / architect A2 / A19 (bodies built at `Add`; rollback) | §2.4 `Add`, MUST 7, §3.2 MUST 12 |
| Breaker S5 / architect A1 (the closed queue) | §2.4 flusher, MUST 6 |
| Breaker S6 / architect A13 (a stuck `Write`; when the ctx ends) | §2.1 `Close`, §3.6 |
| Breaker S7 (the `batch_end` record exists) → owner's design 6 | §2.9, §5.3, MUST 11 |
| Breaker S8–S9 / challenger C5 / architect A15 (`apipartition`'s text) | §6.1 |
| Breaker S10 / QA F4, F8 (`apidiff` one package; the verdict in the text; no merge base) | §6.4 |
| Breaker S11 / architect A17 (`inmemory` drops) | §5.5 |
| Breaker S12 (an added app's start) | §2.4 `Add`, MUST 10 |
| Breaker L13 / QA F11 (exit 4's test) | §9 T38 |
| Breaker L14 (`Config.Now`) | §3.1 `config.go` |
| Challenger C1 (repeated options) | §3.2 step 1 |
| Challenger C3 → owner's design 4 (`Canonical` copied) | §5, §5.2 |
| Challenger C4 (recordings without `batch_end`) | §5.3 |
| Challenger C6 (variadic `Upsert`/`Delete`), C7 (`ErrStarted`), C8 (`SourceErrors`) | §3.1 |
| Architect A3 (a never-started app) | §2.4 `Drain`, MUST 8; §3.7 |
| Architect A4 → owner's design 2 (`WithTableRows`) | §3.1, §3.5 |
| Architect A5 → owner's design 1 (`Start` after `Stop`) | §3.5, §3.8 |
| Architect A6–A9, A23 (named departures) | §3.8 |
| Architect A10 (the facade may not start goroutines) | §0's constraints, §2.1 `Run`, §5.1 |
| Architect A11 / QA F6 (`Stats.Sinks` per binding) | §3.1 `stats.go` |
| Architect A12 (an `Open` that ignores cancel) | §2.4 `Drain`, MUST 9 |
| Architect A14 (the transition table) | §3.7 |
| Architect A16 (`io` has no test) | §5 |
| Architect A18 (the import list), QA F2 (the tests' imports) | §0's constraints, §8 |
| Architect A20 (ticks before sources) | §2.4 `Drain`, §2.6 |
| Architect A22 (the lifecycle from a callback) | §3.7, §5 |
| Architect A24 (a half-written row after a panic) | §4.3 (no pool: nothing to clear) |
| Architect A25 / QA F9 → owner's design 5 (unbuilt connectors) | §3.4 |
| Architect P1 (`AttrType`) | §5.4 |
| Architect P2 (`ConnectorFactory`) | §3.1, §3.4 |
| Architect P3 (`Deploy(text)` kept) | §3.1 |
| QA F3 (MUST 6's claim was false) | §3.2 MUST 12 |
| QA F5 (`Batches`' home) | §2.1 `StreamStats` |
| QA F7 (`Lints()`) | §3.1 |
| QA F10 (an arrival stream's `ts`) | §4.3 |
| QA F12 (the rows' fields and reason texts) | §6.1, §6.3, §6.4 |
| QA F13 (`Load`) | §3.1 |
| QA F14, F15 (`tools/**`; "sinks connected") | §8; §3.5 |

## 13. The fix round, as built (after the combined review and the break-it; the owner's decisions of 2026-11-08)

The reports: [combined review](../reviews/s3-3-1-combined-architecture.md), [break-it](../reviews/s3-3-1-breakit-qa.md); the decisions: [2026-11-08](decisions.md#owner-decisions-2026-11-08--s33-1s-review-and-break-it). Every change below has a test that failed first ([the record](s3-3-1-record.md) lists them).

- **The router decides activation** (review B1; break-it B2): `Activate` sends an `iActivate` item; the router, in intake order, refuses it after a drain (`ErrDrained`), a removal (`ErrRemoved`) or a first activation (`ErrStarted`), else sets its own `activated` flag and `started`, and only then does the caller start the flushers and the recorder. The router never writes a field a caller reads: a never-activated drain no longer clears `a.rec`; the removal tombstone keeps `a.dec` and `a.rec` and drops only the router's own fields (`ing`, `sinks`). `Drain` takes the phase it reverts to from the CAS that succeeded.
- **A removal is the router's once** (break-it B1): the router's `removing` flag, set when it accepts a removal, refuses a second `Remove` and a later `Activate` with `ErrRemoved`; two `Undeploy`s at once, or `Close` racing one, no longer wait on markers no flusher will close.
- **An app's clock starts at `Start`** (owner, decision 1; review S1): the router's ticks skip an app until it is activated.
- **`Add`, `Remove` and `Activate` wait for the router's answer once queued** (review S2): their context bounds the wait for the intake only, so an app is never installed, or removed, behind a caller that gave up.
- **A panicking source is not recovered** (owner, decision 2): its two crashes are fixed at their roots — `Deploy` refuses a `ConnectorFactory` that returns no connector and no error (break-it B3), and `inmemory.Source` takes one `Open` (a second returns `ErrReopened`, break-it B4).
- **Starting rows** get the engine's own checks at `Deploy` (`runtime.CheckRows`, shared with `TableWrite`: a non-finite float, a value a recording cannot copy, a row over a recording line; break-it B5), and are written at `Start` in table order; the bound sources run in stream order; unknown names and malformed params are reported in sorted order (break-it B6, L2; review S5).
- **A null time attribute is refused at the Go boundary** (`runtime.Send` and the facade's `SetNull`), as the decoder refuses it (break-it S1); a malformed override's message names the literal forms, not the version pragma (S5); a writer from a fill that panicked is disarmed (L1); `Deploy` on a closed runtime is `ErrClosed` before the program is read (review, later).
- **`inmemory.Source.Close`** never waits behind a parked `Push`: the channel is never closed, a quit signal ends both (break-it S6).
- **`apipartition`** prints an implicitly repeated const with its inherited type, value and `// iota N`, and an exported alias of an unexported type with that type's exported fields (break-it S3); both goldens regenerated (law, reviewed in the PR).
- **The `apidiff` row** reports a package removed at HEAD as a removal (red unless the golden changed; not counted as covered, since the row's scope is the packages at HEAD) and a renamed module as a stated incompatible change (break-it S4, L3).
- **A second `Close`** waits, until its own deadline, for the goroutines the first left running: nil once they have returned, ctx's error otherwise (break-it S2; the doc and §3.7 corrected, the behaviour kept).
- **Forwarded to S3.3-2** (known issues): `apipartition`'s method entries name the method without its receiver type (review, later); `Close` racing a `Deploy` or a `Start` in flight (review, later).
