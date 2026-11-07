# S3.3-1 — design note: the public Go API (v1)

**What it covers:** items 1, 2 and 5 of the [S3.3 card](sprint-plan.md#s33--the-deploy-pipeline-and-the-conformance-runner--clock-freeze) and the forwards on it that touch the facade, as split by the owner ([decisions, 2026-11-07](decisions.md#owner-decisions-2026-11-07--s33s-plan-check), 1): the facade `cep`, the connector contracts `io` (package `cepio`) with `io/inmemory`, the `Table` API, the API gate (`apipartition`, `apidiff`, the clock golden), D3's and D7's deploy errors, `Deploy`'s recording setting, `Deploy` never mutating the checked model. Not here: the expect sandbox, the conformance runner, the baselines (S3.3-2); patterns (S4.1); every other connector (S5.1, S5.2).

**Sources this note rests on:** [ADR-0001](../architecture/adr/ADR-0001-module-and-package-layout.md) §2 (the three public packages), [ADR-0008](../architecture/adr/ADR-0008-connectivity.md) §1–§2 (the SPI, the value boundary, sinks' flushers), [ADR-0012](../architecture/adr/ADR-0012-streamnode-and-app-lifecycle.md) §2–§5 (identity, the lifecycle, the ingest facade, tables), [ADR-0015](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md) §3's `apipartition` row and §4 (planted proofs), [ARCHITECTURE §3](../architecture/ARCHITECTURE.md) rules 5–7 (the `cep → io` edge; the engine never imports its facade), SEMANTICS §4 (one table snapshot per event) and §7 (batch punctuations), and the code at `main` @ a8a63ad: `internal/runtime` (`Start`, `Send`, `SendPayload`, `TableWrite`, `Drain`, `Stats`, `Close`; `App`, `Config`, `Recorder`), `internal/plan.Build`, `internal/validate.Check`, `internal/parser.Parse`, `internal/playback.AppendEvent`/`NewRecorder`, `internal/table`.

**Routine (owner):** QA's tests first; this note; the blind three-reviewer panel, then QA on testability; the owner's decisions; the build in the main session; the combined review and a break-it; one fix round under the race detector; T4.

## 1. Where it lands

| Path | What | Points |
|---|---|---|
| `internal/runtime` | the per-app lifecycle: a runtime that starts empty, apps added, activated, drained and removed while it runs; `Close` in reverse order (§2) | 1.5 |
| `cep/` (module root, package `cep`) | `Runtime`, `Config`, `New`, `Deploy` and its options, `AppHandle`, `InputHandler`, `Table`, `Diagnostic`, the errors, `BuildInfo` (§3, §4) | 2.5 |
| `io/` (package `cepio`) | `Source`, `Sink`, `Mapper`, `RowWriter`, `RowReader`, `Emitter`, `OutEvent`, `Mode`; the batch-close signal (§5) | 1 |
| `io/inmemory` | the programmatic source and sink over the facade (§5.4) | 0.5 |
| `tools/apipartition`, `api/exports.golden`, `api/clock.golden`, gate rows `apipartition`, `clockgolden`, `apidiff`, their proofs, `go.mod`'s `tool` line, `tools/deps-allowlist.json` | the API gate (§6) | 1.5 |
| `internal/validate`, `docs/language` | D3's and D7's codes, SPEC rev 7's change-log entry, two invalid examples, `errors.md` (§7) | 0.5 |
| `tools/importcheck/rules.json` | `tools/apipartition` is outside `archcheck`'s scope; `io/inmemory` matches `io/*`; nothing else changes | incl. |

About 8 points. One sprint, one PR, at most two agents at once.

## 2. The runtime's per-app lifecycle (`internal/runtime`)

Today `Start(cfg, apps)` builds every app before the router runs, and `Close` stops everything. The facade needs apps to come and go while the runtime runs (ADR-0012 §3). The change keeps every existing entry point (`Start` stays as the convenience the tests use: `New` + `Add` + `Activate` per app) and adds four:

```go
// New starts the router and the shards with no apps. Close stops them.
func New(cfg Config) (*Runtime, error)

// Add registers app, inactive: its schemas and rules are built on every shard, its id is assigned (dense, never
// reused), and nothing accepts its input yet. It is the router that installs the app, between two intake items, so
// no event is ever routed to a half-installed app.
func (r *Runtime) Add(ctx context.Context, app App) (AppID, error)

// Activate starts the app's sink flushers and, with a Recorder, its recorder goroutine (the header is its first
// record), then opens its input: Send, SendPayload and TableWrite accept the app from here. Activating twice is an
// error; activating a drained or removed app is an error.
func (r *Runtime) Activate(ctx context.Context, app AppID) error

// Drain is unchanged: the barrier, then quiescence as a fixpoint; with a recording, its stop line (until the deadline).

// Remove unregisters a drained app: the router drops its routes, every shard frees its state, its sink queues are
// closed once flushed, its recorder goroutine has returned. Its id stays in Stats as empty rows. Removing an app that
// is not drained is an error; removing twice is ErrRemoved.
func (r *Runtime) Remove(ctx context.Context, app AppID) error

// Close drains and removes every live app in reverse id order (a drain already done is not repeated), then stops the
// router and the shards. It honours ctx as Drain does; past the deadline the remaining apps are not drained and the
// goroutines are stopped anyway.
func (r *Runtime) Close(ctx context.Context) error
```

- **MUST 1:** the router is the only goroutine that reads or writes `r.apps`, `shard` per-app state and the routes. `Add` and `Remove` are intake items (`iAdd`, `iRemove`) with a reply; the shards get a matching message (`mAdd`, `mRemove`) and ack through the outbox, as a drain does, so a shard never sees an event for an app it has not built or has freed.
- **MUST 2:** an app's `AppID` is its index in `r.apps`; a removed app leaves a nil slot. `Stats` keeps the slot as zero rows so `[app]` indexes never shift. `appOf` returns `ErrRemoved` for a nil slot.
- **MUST 3:** before `Activate`, `Send`/`SendPayload`/`TableWrite` return `ErrInactive`; after `Drain`, `ErrDrained` (unchanged); after `Remove`, `ErrRemoved`. These are checked on the caller's goroutine from an atomic state (`inactive → active → draining → removed`), as `draining` is today.
- **MUST 4:** sink flushers and the recorder goroutine start at `Activate`, not at `Add`, so a deployed-but-not-started app holds no goroutine but its shard state (ADR-0012 §3: "register (inactive)").
- **MUST 5:** `Close` stops apps in reverse order of `Add` (exit 3), and every goroutine the runtime started has returned when `Close` returns without error (goleak holds in the tests).
- **MUST NOT:** no sleep, no polling; every wait is a reply channel or an ack, as today.
- `Config`, `App`, `Recorder`, `Stats` are unchanged. `Start(cfg, apps)` becomes `New` + `Add` + `Activate` in order and keeps its behaviour, so every existing test holds unchanged (the guard: the runtime's test suite green with `Start` rewritten).

## 3. The facade: `cep`

The facade is thin: it converts, binds and forwards. It holds no engine logic and no goroutine of its own (the engine's goroutines are the runtime's; a connector's are the connector's). `cep` imports `internal/parser`, `internal/validate`, `internal/plan`, `internal/runtime`, `internal/playback`, `internal/event`, `internal/table` and `io` (ARCHITECTURE §3 rule 5; `rules.json`'s `""` row already lists them). `internal/*` never imports `cep` (rule 7).

```go
package cep

// Config is the runtime's configuration; zero values take the engine's defaults (internal/runtime.Config).
type Config struct {
	Shards      int           // 0: GOMAXPROCS
	Mailbox     int           // per shard, in messages; 0: 1024
	Intake      int           // the router's intake queue; 0: 1024
	SinkQueue   int           // per sink binding; 0: 1024
	RecordQueue int           // each recording's queue; 0: 65,536
	TickEvery   time.Duration // live idle ticks; 0: none
	Now         func() time.Time // the wall clock; nil: time.Now
}

type Runtime struct{ /* unexported */ }

func New(cfg Config) (*Runtime, error)
func (r *Runtime) Close(ctx context.Context) error // Stop every app in reverse deploy order, then the engine

// Deploy parses, validates and plans src, binds its connectors, and registers the app inactive. It returns every
// diagnostic at once as a *DeployError. Expect blocks are not run in S3.3-1 (S3.3-2 adds the sandbox stage here).
func (r *Runtime) Deploy(ctx context.Context, name, src string, opts ...DeployOption) (*AppHandle, error)

type DeployOption func(*deployOptions)
func WithParam(name, literal string) DeployOption          // a ${param} override, written as the rule language writes it
func WithSource(stream string, s cepio.Source) DeployOption // binds a source to a declared source stream
func WithSink(stream string, s cepio.Sink) DeployOption     // binds a sink to a derived stream
func WithRecording(w io.Writer) DeployOption                // the recording, off unless given (owner D4 of S3.2b-2)

type AppHandle struct{ /* unexported */ }
func (a *AppHandle) Name() string
func (a *AppHandle) Start(ctx context.Context) error          // sinks, tables, delivery, then sources last
func (a *AppHandle) Stop(ctx context.Context) (StopReport, error) // sources first, the drain barrier, quiescence
func (a *AppHandle) Undeploy(ctx context.Context) error       // Stop (if not stopped) + unregister
func (a *AppHandle) Input(stream string) (*InputHandler, error)
func (a *AppHandle) Table(name string) (*Table, error)
func (a *AppHandle) Stats(ctx context.Context) (Stats, error)

type StopReport struct{ DrainedPending int64 } // timers left unfired by the barrier (drained_pending)

type InputHandler struct{ /* unexported */ }
// SendRow allocates the row, calls fill, checks it (§4) and hands it to the engine; it blocks for backpressure and
// respects ctx. batchID names the source batch the row belongs to ("" for none).
func (h *InputHandler) SendRow(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error
// EndBatch is the batch-close signal (§5.3).
func (h *InputHandler) EndBatch(ctx context.Context, batchID string) error

type Table struct{ /* unexported */ }
func (t *Table) Upsert(ctx context.Context, fill func(cepio.RowWriter) error) error
func (t *Table) Delete(ctx context.Context, key func(cepio.RowWriter) error) error // key writes the key columns only
func (t *Table) Load(ctx context.Context, fills []func(cepio.RowWriter) error) error // one write, many rows

type Diagnostic struct{ Code string; Line, Col int; Message, Fix string; Lint bool }
type DeployError struct{ Name string; Diagnostics []Diagnostic } // Error() lists them, one per line
type Info struct{ Module, Version, GoVersion string }
func BuildInfo() Info // from runtime/debug.ReadBuildInfo
var ErrClosed, ErrInactive, ErrStopped, ErrUndeployed, ErrUnknownStream, ErrUnknownTable error

type Stats struct { Streams map[string]StreamStats; Rules map[string]RuleStats; Recording RecordingStats; Sinks map[string]SinkStats }
```

- **Deploy's pipeline (ADR-0012 §3):** `parser.Parse(name, src)` → `validate.Check(app, validate.Options{Overrides})` → `plan.Build(checked)` → bind (§3.1) → `runtime.Add`. A parse error, any non-lint diagnostic, a binding error or a `plan.Build` error is a `*DeployError`; lints are returned on the handle (`AppHandle.Lints()`), never an error.
- **MUST 6 (L2):** `Deploy` keeps neither the parsed nor the checked `*model.App`. After `plan.Build` the model is unreachable from the facade; the plan compiles its bodies at `Activate` (as `runtime.build` does today from `NewBody`). A test deploys, mutates nothing it can reach, and shows a second `Deploy` of the same source gives an app with the same behaviour.
- **MUST 7 (D4):** without `WithRecording`, `runtime.App.Recorder` is nil. With it, `playback.NewRecorder(w, plan)`.
- **MUST 8:** `Deploy` returns the app inactive; `Start` is `runtime.Activate` after the sinks are connected (§5.2) and the tables loaded (there is no table loader in S3.3-1: tables start empty and `Load` fills them; a `given table` is S3.3-2's); sources are opened last (§5.1). `Stop` closes the sources first (cancels their `Open` contexts and waits for them to return), then `runtime.Drain`, and reports `DrainedPending`. `Undeploy` is `Stop` if not stopped, then `runtime.Remove`.
- **MUST 9:** `WithParam`'s literal is parsed by the parser's own literal lowering, exported for this as `parser.ParseLiteral(text string) (model.Literal, error)`, so an override is written exactly as a param default is (the smallest int included, SPEC rev 6 §2.4); a literal that does not parse is a `DeployError` with E-PARAM-001.
- `Stats` is keyed by name (streams, rules, sink streams), converted from `runtime.Stats` by the app's index maps; `OutboxHigh` and the queue marks are included as they are.

### 3.1 Binding connectors

A source stream may be bound to one `cepio.Source` (`WithSource`) or fed through `Input`; both are allowed on one stream. A derived stream may be bound to any number of sinks (`WithSink`), each a `runtime.SinkBinding`. A `@source`/`@sink` annotation in the program names a built-in type (`inmemory` here; `http` etc. later): in S3.3-1 an annotation other than `inmemory` is a `DeployError` naming the sprint that builds it (E-NAME-001 is not the right code: the name is known, the connector is not built — a facade error, `ErrConnectorNotBuilt`, listed as a diagnostic with code `E-CONN-001`? **No new code:** the facade reports it as a plain error, not a diagnostic; the owner's question Q2 below). `WithSink` checks `Sink.Accepts()` against the stream's mode: a plain rule's stream is `Append`; a `Changelog`-only sink on it is `E-SINK-001`'s situation reversed and is refused with the same code's text.

## 4. The value boundary

`cepio.RowWriter` and `cepio.RowReader` are the only way values cross the facade; `internal/event.Value` never does (ADR-0008 §1, round-2 N3).

```go
package cepio

type RowWriter interface {
	SetString(attr int, v string)
	SetInt(attr int, v int64)
	SetFloat(attr int, v float64)
	SetBool(attr int, v bool)
	SetTimestamp(attr int, unixMs int64)
	SetDuration(attr int, ms int64)
	SetObject(attr int, jsonText string) // an object enters as its JSON text (S3.1-2's B5)
	SetNull(attr int)
	AttrIndex(name string) int // -1 if unknown
}

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
	Len() int
}
```

The facade's `row` implements both over a `[]event.Value` sized by the schema, reused per `InputHandler` (one per caller goroutine; `SendRow` is safe for concurrent use because each call takes a row from a `sync.Pool`), with the schema's attribute names for `AttrIndex`.

- **MUST 10 (the checks, in this order, each an error to the caller naming the attribute):** a `Set` on an index outside the schema; a `Set` of the wrong type for the attribute (recorded at the call, reported by `SendRow`); a non-finite float (S3.1-2's D2); a string that is not well-formed UTF-8, an object that is not one compact JSON value within the log's limits, a timestamp outside years 0000–9999 (S3.2b-2's D1: `runtime.Send` already refuses these — the facade lets `Send`'s error through unchanged); a non-optional attribute left unset; an event-time stream's time attribute unset. A `time arrival` stream's `ts` is stamped by the engine, and a `Set` on it is an error.
- **MUST 11:** no allocation per `SendRow` beyond the engine's own (the row is pooled, `fill` writes in place; `allocgate`'s `AllocsPerRun` on `SendRow` with a pooled row: 0 above `runtime.Send`'s).
- **Objects:** `SetObject` takes JSON text, never `any` (the ADR's `SetObject(attr int, v any)` narrowed by B5: "an object value enters as its JSON text"). A caller with a Go value marshals it first.
- **`Table.Upsert`'s row** is checked as `TableWrite` checks a row today (type, key not null); `Delete`'s `key` fill may set key columns only (any other `Set` is an error).

## 5. The connector contracts: `io` (package `cepio`)

```go
package cepio

type Mode uint8
const ( Append Mode = iota + 1; Changelog )

// Emitter is what a Source gets: the engine's intake for one stream. Emit allocates the row, calls fill, checks it
// and ingests it; EndBatch closes batchID (§5.3). Both block for backpressure and respect ctx; both are safe for
// concurrent use.
type Emitter interface {
	Emit(ctx context.Context, batchID string, fill func(RowWriter) error) error
	EndBatch(ctx context.Context, batchID string) error
}

type Source interface {
	// Open feeds the stream until ctx is done, then returns; the engine opens sources last at Start and cancels
	// their ctx first at Stop, waiting for Open to return.
	Open(ctx context.Context, emit Emitter) error
}

type Sink interface {
	Accepts() Mode
	// Write is called from the sink's own flusher goroutine only, in output order; an error is counted and the
	// output is dropped (at-least-once is the connectors' concern, ADR-0008 §3; the engine does not retry).
	Write(ctx context.Context, e OutEvent) error
}

type Mapper interface { Decode(b []byte, w RowWriter) error } // the contract only; io/jsonmap is S5.1's

// OutEvent is one output: the derived stream's name, its event time, the row and its canonical JSON (ADR-0003/0005:
// the log sink prints exactly this text, so demo output diffs against expect fixtures).
type OutEvent struct {
	Stream    string
	Timestamp int64 // unix ms
	Row       RowReader
	Canonical []byte // valid until Write returns
}
```

- **5.1 Sources:** the facade adapts `Emitter` over the stream's `InputHandler`. `Start` opens each bound source on its own goroutine (the connector's, ARCHITECTURE §3 rule 6: `io/*` may own I/O goroutines; the facade starts it, the connector runs it) with a context cancelled by `Stop`; `Stop` waits for every `Open` to return before draining, so no event arrives after the barrier (the ones that do are `AfterDrain`, counted).
- **5.2 Sinks:** the facade wraps each `cepio.Sink` as a `runtime.Sink`: `func(e event.Event) error` builds the `OutEvent` (the `RowReader` over `e.Values`; `Canonical` by `playback.AppendEvent` into a buffer the flusher owns) and calls `Write(ctx, out)` with the app's lifetime context. The flusher goroutine is the runtime's (ADR-0008 §2 as amended). `Accepts()` is checked at `Deploy` (§3.1).
- **5.3 The batch-close signal (the API Δ this sprint designs, ADR-0008 §1):** a source that batches calls `EndBatch(ctx, batchID)` after the batch's last `Emit`. In S3.3-1 the engine **accepts, counts (`StreamStats.Batches`) and otherwise ignores** it: batch windows are S8's, and the recording gains its record kind (SEMANTICS §7, §10: punctuations are part of the replay contract) when S8 first reads it, so no format change lands before anything replays it. A batch id is the string the source gives; the engine does not interpret it. **Owner question Q1** asks whether to record the signal now instead.
- **5.4 `io/inmemory`:** `inmemory.Source{}` — a channel-fed source: `Push(ctx, batchID, fill)` and `Close()` for the embedder, `Open` pumping into the `Emitter`; `inmemory.Sink{}` — collects `OutEvent`s (copying `Canonical`) behind a mutex with `Drain() []Record`; `Accepts()` is `Append`. Both import `cep` and `io` only (rule `io/* → cep, io`). This is what tests and the owner's check use.

## 6. The API gate

- **`tools/apipartition` (Go, stdlib only):** for each package named, parses its non-test files with `go/parser` and prints one line per exported top-level declaration — `package ident kind signature` — the signature as `go/printer` prints the declaration's type (no doc comments, no bodies), sorted. It never type-checks, so it needs no dependency and runs in under a second. Two files, both law: `api/exports.golden` (`cep`, `io`, `io/inmemory`) and `api/clock.golden` (`internal/clock`). The row compares the tool's output to the file and fails with `API Δ: +n/−m (names)`; regenerating a golden is editing law (`--write`, reviewed in the PR). Rows `apipartition` (scope `cep/`, `io/`, `api/exports.golden`) and `clockgolden` (scope `internal/clock/`, `api/clock.golden`) at T2–T4, blocking. **Why `go/parser` and not `go/types`:** the export list is law written by hand (ADR-0015 GR-03); a change to a signature's text is what a reviewer reads; type identity adds nothing the text does not show, and `go/types` with source importing costs seconds per run. `apidiff` (next) is the typed check.
- **`apidiff` (owner's decision 3):** `go.mod` gains `tool golang.org/x/exp/cmd/apidiff` (Go 1.24+'s tool directive; pinned, with its module graph, in `tools/deps-allowlist.json`, scope build-time). Row `apidiff` at T4: a Python row (`gate/runner/rows/apidiff.py`) adds a detached worktree at the merge base with `main` (as `lawdelta` will), runs `go tool apidiff -w base.export ./cep ./io ./io/inmemory` there, then `go tool apidiff base.export <same>` here, and prints the report in the row's reason. **Red when:** the report lists an incompatible change and `api/exports.golden` is unchanged against the merge base (a signature changed without the law changing with it). A compatible change, or an incompatible one with the golden updated, passes with the report printed. Before the first release every change is allowed when deliberate; at S12 the policy tightens to a tag. The row skips (not fails) when there is no merge base (a fresh clone with one commit) and says so.
- **Proofs (exit 5):** `gate/proofs/drivers/test_api_planted.py`: (a) an exported function added to `cep` and not to the golden → `apipartition` red naming it; (b) a parameter added to `AppHandle.Start` with the golden unchanged → `apidiff` red, its report naming `Start`; (c) the same with the golden regenerated → `apidiff` passes and prints the change; (d) an exported `internal/clock` signature changed → `clockgolden` red. The `proofs` row's `min_wants` rises to 19.
- **The clock contract freezes at S3.3-2's G5** (the card's item 7); `api/clock.golden` lands here so S3.3-2's freeze is one row's `blocking` already true.

## 7. D3's and D7's deploy errors

- **E-TBL-002 Primary-key column optional or object [static]:** SPEC §4.3; a `primary key (…)` column declared `optional` or `object` is refused at deploy (S3.1-2's D3). Invalid example `invalid/e-tbl-002.cep`.
- **E-PART-008 Partition key of type object [static]:** SPEC §4.4; an `object` attribute as a key part is refused at deploy (S3.2a-1's D7); `partition.NewRule`'s refusal stays as the engine's guard. Invalid example `invalid/e-part-008.cep`.
- Both: `internal/validate/catalog.go`, `errors.md`, the static-error test programs (`TestStatic` needs a program per code), SPEC v0.1 **rev 7** change-log entry ("two deploy errors; the grammar is unchanged"), Appendix C's count 67. No grammar change, so the language-design skill is not needed; the differential corpus gains the two invalid files.

## 8. Tests (QA first, red before the build)

Exit 3: two apps deployed in one `Runtime` from the same source feed the same events; each sees only its own outputs and tables, their `Stats` are separate, and `Close` stops them in reverse deploy order (an `inmemory.Sink` records the order of its last `Write` per app; a source's `Open` returns in the order their contexts are cancelled). Exit 4: a rule `a in T and b in T` over one event while a goroutine upserts `T` between the two reads (a sink or a `Table` hook that blocks the first `in`? — no hook exists; the test holds the `Upsert` behind the facade's table mutex from a second goroutine and asserts, over 10,000 events, that no output ever saw one `in` true and the other false for the same key set: both old or both new). Exit 5: the proofs. The lifecycle: `Deploy` → `Start` → events → `Stop` → `Undeploy` with goleak; `Send` before `Start` is `ErrInactive`; after `Stop`, `ErrStopped`; after `Undeploy`, `ErrUndeployed`; `Deploy` after `Close` is `ErrClosed`. The boundary (MUST 10): one row per check. `apipartition`'s own unit tests over a fixture package. D3 and D7: a program each, red until the codes exist. The recording option: a deploy with `WithRecording` replays to the live outputs through `playback.Replay` (S3.2b-2's equivalence test, through the facade). L2: MUST 6's test. `SendRow`'s allocation budget.

## 9. Owner questions (asked after the panel)

- **Q1:** record the batch-close signal now (a new record kind in the recording format, about +0.5) or when S8 first reads it (this note)?
- **Q2:** a program whose `@source`/`@sink` names a connector not yet built: a deploy error now (this note), or deploy with the annotation ignored and the stream fed through `Input` only?
- **Q3:** `apidiff` red only when an incompatible change comes without a golden change (this note), or red on every incompatible change until S12 (every signature change then needs an owner-approved override in the row)?

## 10. Size

About 8 points as carded: runtime 1.5, facade 2.5, contracts and inmemory 1.5, the API gate 1.5, the codes 0.5, D4 and L2 0.5.
