# S3.3-1 design note v1 — architecture review (the architect, with the-fool)

**Reviewed:** `docs/sprints/s3-3-1-design.md` v1 at `ce272a4` in the read-only worktree `/Users/ramith/code/cep-wt/s3-3-1-panel`. Blind: no other review read.
**Code graph:** GitNexus reports its index **62 commits behind HEAD** (`mcp__gitnexus__query` staleness `behind`, `commitsBehind: 62`; the `gitnexus://repo/cep/context` resource tool is not available in this session, so `index.commit` could not be read). Every structural claim below is therefore from the source, with file:line.
**Memory:** three Hindsight searches (the batch-close recording, Start-after-Stop, the apipartition format) returned only the five generic pages; no recorded decision on any of the three — `memory: none applied`. The decisions cited below are from `docs/sprints/decisions.md` and the ADRs directly.
**Scratch programs** (all exit 0, under `/private/tmp/claude-501/-Users-ramith-code-cep/c32ea7c4-ab50-4044-8f33-e9ed338fa3bd/scratchpad/panel/architect/scratch/`): `closedchan` (receive from a closed channel yields the zero item for ever; send on it panics), `listing` (a go/parser export lister misses promoted and reachable methods that reflect shows), `pool` (a pooled RowWriter kept past `fill` aliases the next row), `stuckopen` (a cancelled `Open` that ignores ctx is ended only by the deadline, and its goroutine stays).

**Counts:** 2 blockers, 15 should-fix, 7 later. Three owner questions of my own (Q4–Q6) and a recommendation on the note's Q1–Q3.

**Steelmanned thesis (the-fool, step 1):** the note turns a whole-runtime `Start/Close` into a per-app `Add/Activate/Drain/Remove` through the router's intake order, so a thin facade can register, start, stop and remove apps while the engine runs; values cross only as `RowWriter`/`RowReader`; the public surface is listed by hand and diffed by `apidiff`; the batch-close signal is accepted but not recorded until something replays it. Every piece lands on code that exists today (`Drain`'s retry semantics, `TableWrite` through intake, `Recorder` off by default, the boundary checks in `Send`). The thesis is sound; the findings are where its contracts cannot hold as written.

---

## 1. Conformance with ADR-0001 §2, ADR-0008 §1–§2, ADR-0012 §3–§5 (brief item 1)

| # | ADR text (file:line) | Note (line) | Departure | Named in the note? | Finding |
|---|---|---|---|---|---|
| 1 | ADR-0008 §1, `ADR-0008-connectivity.md:13-15`: `Open(ctx, emit EmitFunc)`, `EmitFunc: func(fill func(RowWriter) error, batchID string) error`; `:28` "batch identity travels as EmitFunc's batchID string — no separate handle type"; ARCHITECTURE §3 `ARCHITECTURE.md:109` lists `EmitFunc`; the S3.3 card item 2 (`sprint-plan.md:113`) lists `EmitFunc` | `s3-3-1-design.md:189-198`: `Emitter` interface with `Emit(ctx, batchID, fill)` and `EndBatch(ctx, batchID)`; `Open(ctx, emit Emitter)` | A func type becomes a two-method interface; the parameter order flips (batchID before fill); `ctx` added | No. §5.3 calls the batch-close signal "the API Δ" but never says `EmitFunc` is gone | **A6** (should-fix) |
| 2 | ADR-0012 §3 impl. note, `ADR-0012:35`: `SendRow(fill func(cepio.RowWriter) error) error` | `:109`: `SendRow(ctx, batchID string, fill)` | `ctx` and `batchID` added | No | A6 |
| 3 | ADR-0008 §2, `ADR-0008:36`: the flusher "Retry with exponential backoff + cap; after max retries: log + `sink_failures` counter"; SEMANTICS §10 `SEMANTICS.md:196`: "Sink failures: retry with backoff per sink config" | `:200-204`: "an error is counted and the output is dropped (… the engine does not retry)" | Retry leaves the engine | Half: the note says "does not retry" and cites §3 (at-least-once), not §2, which places retry in the flusher. The as-built flusher already drops and counts (`runtime.go:1320-1329`), so the ADR has been behind the code since S3.2a-2 | **A7** (should-fix) |
| 4 | ADR-0012 §5, `ADR-0012:20`: `Upsert(fill)`, `Delete(key any)`, `Load(fills)` | `:114-116`: `Upsert(ctx, fill)`, `Delete(ctx, key func(RowWriter) error)`, `Load(ctx, fills)` | `key any` becomes a key fill; every method gains `ctx` (writes go through the intake, F14: `runtime.go:559-585`) | No | **A8** (should-fix, one sentence) |
| 5 | ADR-0001 §2, `ADR-0001:11`: `cep` holds "the public Event view"; Consequences `:23`: "public `cep.Event` wraps `internal/event.Event`" | `:118-124`: no `cep.Event`; outputs cross as `cepio.OutEvent{Row RowReader, Canonical}` (`:211-216`) | The public event view moves to `io` as a reader | No | **A9** (should-fix: ADR-0001 says "exactly") |
| 6 | ADR-0008 §5, `ADR-0008:39`: "the library user passes [`${param}` overrides and connector settings] via `cep.Config`" | `:90-92`: `WithParam`, `WithSource`, `WithSink` per deploy | Per-app options instead of runtime config | No | **A23** (later: name it; the per-app form is better) |
| 7 | ADR-0012 §3 annotation (owner D3, S3.1-1), `ADR-0012:16` end: "at Stop the ticks stop BEFORE the sources" | `:130`: Stop = sources, then `runtime.Drain` | No per-app "ticks stop" step; the ticker is one per runtime (`runtime.go:651-667`) and skips an app only once `drained` (`:815-817`) | No | **A20** (should-fix) |
| 8 | decisions.md:358 (owner F5, S3.2a-2): "S3.3's `Start` after `Stop` builds a fresh app" | `:39`: "activating a drained or removed app is an error" | A restart is refused, not rebuilt | No | **A5** (should-fix; owner Q4) |
| 9 | ADR-0012 §3, `ADR-0012:14`: Start = "connect sinks (retry) → load/connect tables → start shard delivery → start sources LAST" | `:130`: "there is no table loader in S3.3-1: tables start empty and `Load` fills them"; MUST 3 (`:56`) refuses `TableWrite` before `Activate` | The "load tables before sources" step cannot be performed through the public API | No | **A4** (should-fix) |
| 10 | ADR-0008 §1 `ADR-0008:24`: `SetObject(attr int, v any)` | `:152`: `SetObject(attr int, jsonText string)` | Narrowed | **Yes** (`:175`, B5) | held |
| 11 | ADR-0012 §3 `ADR-0012:13`: `Deploy(src) (AppHandle, error)` | `:87`: `Deploy(ctx, name, src, opts...) (*AppHandle, error)` | pointer, name, options | No | later, cosmetic |
| 12 | ADR-0012 §4, §3: `Input(stream) (cep.InputHandler, error)`, "both return backpressure errors rather than blocking past ctx" | `:100, :108-109` | matches | — | held |
| 13 | ADR-0008 §2 as amended `ADR-0008:36`: flushers owned by `internal/runtime`, called "through an internal function type that the `cep` facade adapts from `cepio.Sink`" | `:220`: the facade wraps `cepio.Sink` as `runtime.Sink` (`runtime.go:37`) | matches | — | held |

## 2. Goroutine ownership (brief item 2)

Who starts, who stops, what each blocks on, and whether `Stop`/`Undeploy`/`Close` can deadlock or leak. "Today" = `runtime.go` at ce272a4; "note" = the design.

| Goroutine | Started by | Stopped by | Blocks on | Gap |
|---|---|---|---|---|
| router (`route`, `runtime.go:723-764`) | `Start` today (`:286`); `New` (note `:29`) | `Close` closes `r.stopped` (`:639`) | intake recv; mailbox send / sink-queue send while servicing handoff (`:733-751`) | none new; `Add`/`Remove` as intake items fit the discipline (ADR-0006 §5 table) |
| shard (`shard.run`, `:1167-1189`) | `Start`/`New` | `Close` | mailbox recv or handoff send | **A2**: the note's MUST 1 (`:54`) says the router "writes … shard per-app state" while the shard runs → a data race unless the shard itself builds on `mAdd`; and a build that fails on one shard after another succeeded has no rollback |
| sink flusher (`sinkQ.run`, `:1297-1317`) | `Start` today (`:292-295`); `Activate` (note MUST 4) | `Close` today; note: "its sink queues are closed once flushed" at `Remove` (`:44-46`) | queue recv, then **the sink's `Write`** (unbounded) | **A1**: closing `q.q` makes the as-built loop (`it := <-q.q`, no `ok`, `:1304-1312`) deliver zero events for ever and a pending send panic; **A13**: a `Write` that never returns holds the drain's marker (`:607-615`) and `Close` (`:636-647`) to the deadline and leaks the flusher — the note never says when the "app's lifetime context" it passes to `Write` (`:220`) is cancelled |
| recorder (`recQ.run`, `recorder.go:111-140`) | `Start` today (`:296-299`); `Activate` (note) | `Close` (`r.stopped`); note: "its recorder goroutine has returned" at `Remove` — by what signal? none named | queue recv, then the `Recorder` call (a stuck disk) | **A3**: `Drain` on an app whose recorder goroutine never started puts `rStop`+`rMarker` on the queue (`runtime.go:849-853`) and waits for the marker to the deadline (`:616-623`), then marks the recording failed; the markers to never-started flushers (`:1046-1050`) block the drain the same way |
| ticker (`tick`, `:651-667`) | `Start`/`New` if `TickEvery > 0` | `Close` | ticker channel / intake send | **A20** (per-app Stop has no tick-stop step) |
| source `Open` | the facade's `Start` (note `:219`: "the facade starts it") | the facade's `Stop` cancels its ctx and "waits for every `Open` to return" | its own I/O; `Emit` → `runtime.Send` → intake (`:464-476`) | **A10**: a `go` statement in package `cep` is red (`tools/archcheck/main.go:279`: `goAllowed` = `internal/runtime`, `io/*`, `cmd` only) — the facade cannot start it; **A12**: the wait is unbounded unless bounded by `Stop`'s ctx (scratch `stuckopen`) |
| `inmemory.Source.Push` caller | the embedder | — | the pump's channel | **A17**: if `Open` has not started (app not started) or has returned (after `Stop`), `Push` blocks for ever unless it selects on a closed/ctx signal; the note (`:222`) names neither |

**Deadlock check (what held):** `Add`/`Remove` as intake items with shard acks reuse the drain's path (`iDrain` → `mDrain` → `oAck`, `:834-855`, `:1037-1055`), which services handoff while pending (`:737-738`, `:747-748`), so the router never waits on a shard that is waiting on the router. `Stop` from inside a `Sink.Write` would self-deadlock to the deadline — document "never call the lifecycle from a connector callback" (later, **A22**).

## 3. The lifecycle state machine (brief item 3)

States of one app: `inactive` (after `Deploy`/`Add`), `active` (after `Start`/`Activate`), `draining/drained` (after `Stop`/`Drain`), `removed` (after `Undeploy`/`Remove`), and the runtime's `closed` (after `Close`, which wins: `appOf` checks `r.stopped` first, `runtime.go:454-462`). Cells in **bold** are undefined or contradictory in the note.

| Call ↓ / state → | inactive | active | drained | removed | runtime closed |
|---|---|---|---|---|---|
| `Start` | → active (`:97`, `:130`) | **error, which? none listed (`:122`)** → A14 | **"an error" (`:39`); F5 says "builds a fresh app"** → A5 | error (`:39`) — `ErrUndeployed` presumably | `ErrClosed` |
| `Stop` | **undefined: `Drain` on an app with no flushers/recorder blocks to the deadline** → A3 | → drained; `StopReport` | same report, nil (runtime `:589`, `:836-843`); **the note is silent** → A14 | **undefined** (`ErrUndeployed`?) | `ErrClosed` |
| `Undeploy` | **Stop (undefined) + Remove; `Remove` refuses "not drained" (`:46`)** → A3 | Stop + Remove (`:130`) | Remove | `ErrUndeployed` ("removing twice is ErrRemoved", `:46`) | `ErrClosed` |
| `Input(s)`/`Table(n)` | handle returned; use → `ErrInactive` (MUST 3) | handle; use ok | handle; use → `ErrStopped` | **handle creation after Undeploy: undefined** | `ErrClosed` |
| `SendRow`/`EndBatch` | `ErrInactive` | ok | `ErrStopped` | `ErrUndeployed` | `ErrClosed` (`:455-457`) |
| `Table.Upsert/Delete/Load` | `ErrInactive` (MUST 3) — **contradicts "tables loaded before sources" (ADR-0012 §3)** → A4 | ok, in intake order | `ErrStopped` | `ErrUndeployed` | `ErrClosed` |
| `Stats` | zero rows | ok | ok (readable: F5) | "stays in Stats as empty rows" (`:46`) | `ErrClosed` |
| `Runtime.Close` | **"drains and removes every live app": drain of an inactive app burns the deadline** → A3 | drain, remove, in reverse id order | remove | skip | nil ("a second Close returns nil", `:636`) |
| `Deploy` | — | — | — | — | `ErrClosed` (`:239`) |
| `Deploy` twice, same `name` | **undefined** → A23 | | | | |

## 4. Failure modes (brief item 4)

1. **A sink `Write` that blocks for ever** — A13 above. Today: `Drain` returns `ctx.Err()` at `:612-613`; `Close` returns `ctx.Err()` at `:646` and the flusher goroutine is left (the code comment at `:636-637` says so). The note's MUST 5 ("every goroutine … has returned when `Close` returns without error") holds only in the sense that `Close` then returns *with* an error — state that, and state when `Write`'s ctx is cancelled.
2. **A source `Open` that never returns on cancel** — A12; scratch `stuckopen`: only the deadline ends the wait, the goroutine stays. The note must say whether `Stop` still drains after the deadline (recommend: yes — nothing in the intake is lost; the error names the stream).
3. **A `fill` that panics inside `SendRow`** — the panic surfaces on the caller's goroutine (the facade must not recover user code). With `defer pool.Put(row)` a half-written row returns to the pool: `Get` must clear it, or the next caller inherits a `Set` (later, **A24**). Inside `inmemory.Source.Push`, `fill` runs on the pump's goroutine (`:222`: "`Open` pumping into the `Emitter`"), so a panic there kills the process from a goroutine the embedder did not write — say so, or run `fill` on the pusher's goroutine into a row the pump then emits.
4. **Two apps whose stream names collide** — no issue: ids are per app (`AppID` + stream index, `runtime.go:27`, `:343-348`; ADR-0012 §2), every entry point goes through the handle, `Stats` is per app. Held. The only name the runtime shares is `Deploy`'s `name` (A23).
5. **`runtime.Add` failing on one shard after succeeding on another** — A2: undefined. Today the only build failure is a `NewBody` panic (`newBody`, `runtime.go:1154-1161`; plan's `NewBody` panics only when a compile that `Build` accepted fails again, `plan.go:179-186`), which is deterministic across shards, but the protocol must still roll back.

## 5. The import law and the row shape (brief item 5)

- `cep` (`""`) → `internal/parser, validate, plan, runtime, playback, event, table` + `io`: all in `tools/importcheck/rules.json:21`. **Held.** The note's list (`:64`) omits `internal/model`, which `validate.Options{Overrides map[string]model.Literal}` (`validate.go:31-33`) and `parser.ParseLiteral` (`:131`) force; it is in the law row already — **A18** (later: complete the sentence).
- `io` → nothing (`rules.json:23`); `io/inmemory` → `""`, `io` (`:24`). **Held.** `archcheck`'s file scope excludes `tools/**` (`gate/registry.json:94`), so `tools/apipartition` needs no `rules.json` row. **Held.**
- **Not held:** package `cep` may not contain a `go` statement (`tools/archcheck/main.go:279`) nor call `time.Now` (`:253-254`: `internal/runtime`, `io`, `internal/playback`, `cmd` only), yet the note has the facade start each source's `Open` goroutine (`:219`) and defaults `Config.Now` to `time.Now` (`:77`) — **A10**.
- **Row shape:** `expected_scope` has three kinds (`git`, `go`, `go-vet-units`; ADR-0015 §1.1 annotation `:52-55`) and the runner fails when the step's covered count differs from the live expansion (`:28-30`). The note's "scope `cep/`, `io/`, `api/exports.golden`" (`:226`) names neither kind nor what the tool prints as `GATE_COVERED`, nor `min_wants`. Propose `apipartition`: `["go", ".", "./io", "./io/inmemory"]`, the tool prints `GATE_COVERED 3` (packages), `min_wants 3`; `clockgolden`: `["go", "./internal/clock"]`, `min_wants 1`; the golden files are read by the command, not scope items. **A15** carries this.
- **Coverage:** the `io` package has zero statements; `gate/coverage/classify.py:114-117` passes a 0-statement package only when at least one test passed, and `:107` fails "no tests". The note plans no test for `io` → the `coverage-inclusion` row (`registry.json:322-328` already lists `./io/...`) goes red. **A16** (later).

## 6. The split with S3.3-2 (brief item 6)

`Deploy(ctx, name, src string, opts ...DeployOption) (*AppHandle, error)` with `*DeployError{Diagnostics}` absorbs the sandbox stage additively: E-EXP-001 is a `Diagnostic`, a `WithoutExpects()` option is a new variadic option, `given table` seeding is internal (ADR-0012 §3 `:13`). `cmd/cep validate|test` = `New` + `Deploy` + `Close` through the public path (ADR-0001 §2 `:11`). The clock golden lands now (`:229`). **Held.**

Two shapes the freeze should carry now, because S3.3-2 or the next sprints cannot add them without touching a frozen signature's meaning: (i) `Stats.Sinks map[string]SinkStats` cannot hold two sinks on one stream (`:124` vs `:136` "any number of sinks"; the runtime is `[app][binding]`, `runtime.go:131,133`) — **A11**; (ii) the table pre-load option — **A4**.

## 7. Pre-mortem (the-fool, "it is S5.1/S6.1 and we regret this")

**P1 — S5.1, the JSON mapper cannot type a number (likelihood high, impact high).** The http source arrives; `io/jsonmap` implements `Mapper.Decode(b []byte, w RowWriter)` (`:207`). A JSON `42` for attribute `amount` must become `SetInt`, `SetFloat`, `SetTimestamp` or `SetDuration`, but `RowWriter` (`:145-155`) offers `AttrIndex(name) int` and no type query; `RowReader` (`:157-168`) likewise, so the http sink's `{{attr}}` renderer tries each getter. S5.1 adds `AttrType(attr int) Type` to both interfaces and a `cepio.Type` enum: an added interface method is a breaking change in `apidiff`'s model, every third-party fake breaks, and the golden churns in the first sprint after the freeze. **Signature that would have to change:** `cepio.RowWriter`, `cepio.RowReader`. **Mitigation now (0.1 point):** add to both `AttrType(attr int) Type // TInvalid for an index outside the schema` and `type Type uint8` with `TString, TInt, TFloat, TBool, TTimestamp, TDuration, TObject` (the seven of ADR-0008 §1 R1).

**P2 — S5.1, `cepd` cannot bind `@source(type='http', port=…)` (likelihood high, impact medium).** `cmd/*` imports only public packages (`rules.json:25`; ADR-0001 §5). The annotations live in the model that `Deploy` hides (MUST 6). `cepd` must either parse the program a second time with a public parser that does not exist, or the facade must grow a binding hook. Meanwhile S3.3-1's rule "an annotation other than `inmemory` is a `DeployError`" (`:136`) is a behaviour S5.1 must reverse. **Signature that would have to change:** none breaks, but `Deploy`'s behaviour flips, and `cepd` ships a second parse. **Mitigation now (0.3 point):** `type ConnectorFactory interface { Source(typ string, keys map[string]string) (cepio.Source, error); Sink(typ string, keys map[string]string) (cepio.Sink, error) }` and `func WithConnectors(f ConnectorFactory) DeployOption`; S3.3-1's default factory knows `inmemory` and refuses the rest with the plain error the note proposes — which also answers the note's Q2 by one mechanism.

**P3 — S6.1, two deploy entry points (likelihood medium, impact medium).** The builder's `rule.App` carries a `*model.App`; `rule` may import `internal/model` only (`rules.json:22`) and `cep` does not import `rule`. `Deploy(ctx, name, src string, …)` takes text, so S6.1 adds `DeployApp(ctx, app *rule.App, opts …)` plus the `rules.json` edge `"" → rule`, and the option plumbing is duplicated for good; `WithParam(name, literal string)` re-parses text for a `rule.Param[T]` that is already typed. **Signature that would have to change:** none breaks (`DeployApp` is additive), but the two entry points are the regret. **Mitigation (owner's call, 0.2 point):** `type Program struct{ /* sealed */ }`, `func Text(name, src string) Program`, `Deploy(ctx, p Program, opts …)`; S6.1 adds `func Rules(a *rule.App) Program` in `cep` with the edge. Honest counter-argument: `Deploy(ctx, name, src string)` reads best for the 20-line owner's check; I would keep it and accept `DeployApp` later.

**Early warning signs:** the first `jsonmap` prototype in S5.1 that switches on `reflect`/JSON token kind to guess a setter (P1); a `parser` import in `cmd/cepd` or an `archcheck` suppression (P2); a second `deployOptions` struct (P3).

---

## Findings

Each: id, severity, what breaks, evidence, the fix written out, the test that catches it.

### A1 — blocker — closing a sink queue at `Remove` delivers phantom outputs and can panic
- **What breaks:** the note's `Remove` (`s3-3-1-design.md:44-46`): "its sink queues are closed once flushed". The as-built flusher reads `it := <-q.q` with no `ok` (`internal/runtime/runtime.go:1304-1312`); on a closed channel the receive returns the zero `sinkItem` at once, for ever: `it.marker == nil` so `q.call(it.ev)` runs with a zero `event.Event` in a tight loop — a sink receives outputs that never existed (wrong detection), and a core spins until `Close`. And the router's pending FIFO can still hold a `send` to that queue when the last `mRemove` ack arrives inside `take()` (`:737-738`, `:747-748` call `take` while `pending` is non-empty; a drain pushes its markers from inside `take`, `:1046-1050`): `head.sink.q <- head.si` on the closed channel panics "send on closed channel" on the router goroutine.
- **Evidence:** scratch `closedchan` (exit 0: four zero receives with `ok=false`, then `send on closed channel`).
- **Fix:** never close `q.q`. Add to `sinkQ` (`runtime.go:175-180`): `stop chan struct{}; returned chan struct{}` (both made in `build`/`Add`). The flusher: `defer close(q.returned)` and a third case `case <-q.stop: return`. `Remove`'s router step, after every shard's ack: push one `send{shard: -1, sink: q, si: sinkItem{marker: m}}` per queue as a drain does, then, when that marker is reached by the flusher, close `q.stop` from the flusher itself (`if it.marker != nil { close(it.marker); if it.last { return } }` with `sinkItem.last bool`). `Remove`'s reply carries the markers; `Remove` waits on each `q.returned` under its ctx. The recorder gets the same: `recItem{kind: rMarker, last: true}`.
- **Test:** `TestRemoveDeliversNothingAfterTheFlush` (`internal/runtime`): a sink counting `Write`s with a channel; `Add`, `Activate`, 100 events, `Drain`, `Remove`; assert the count equals the outputs before the drain and that no `Write` arrives for 100 ms after `Remove` returns (wait helper, ctx deadline); under `-race`. `TestRemoveWhileOutputsAreInFlight`: a rule emitting 1,000 outputs per event, `Remove` immediately after `Drain` returns; assert no panic (the router goroutine still answers `Stats`).

### A2 — blocker — MUST 1 makes the router write shard state the shard goroutine reads; no rollback for a partial `Add`
- **What breaks:** `s3-3-1-design.md:54`: "the router is the only goroutine that reads or writes `r.apps`, **shard per-app state** and the routes … the shards get a matching message (`mAdd`, `mRemove`) and ack". If the router writes `sh.rules[app]`, `sh.bodies`, `sh.arrival`, `sh.reads`, `sh.burst`, `sh.outHigh`, `sh.panics` (`runtime.go:1094-1107`, all appended by `newShard` `:1109-1151`) while the shard runs `process` (`:1194-1234`), that is a data race; the race row (`registry.json:438-466`) would catch it in T4. If instead the shard builds on `mAdd`, MUST 1's sentence is wrong and must say so. Either way, a `NewBody` failure on shard k after shards 0..k−1 built (`newBody`, `:1154-1161`) leaves a half-installed app with no defined reply.
- **Evidence:** file:line above; `plan.go:179-186` ("each body owns its trees: nothing is shared across shards") already makes the build per shard.
- **Fix:** rewrite MUST 1: "The router is the only goroutine that reads or writes `r.apps` and the routes. Each shard is the only goroutine that reads or writes its own per-app state. `Add` reaches every shard as `mAdd{app, rules, streams}`; the shard builds its state (`newShard`'s body, per app) and acks `oAck{token, err}`. The router installs the routes only when every ack is nil; on any non-nil ack it sends `mRemove` to every shard, waits for their acks, leaves the slot nil, and replies the first error. `Remove` is `mRemove` + acks, then the route drop." Signature unchanged.
- **Test:** `TestAddUnderRace` (nightly `race-stress` already repeats `internal/runtime` 20×, `registry.json:650-680`): `Add` + `Activate` + events + `Remove` in a loop of 50 apps under `-race`. `TestAddRollsBackWhenOneShardFails`: `Config{Shards: 4}`, a rule whose `NewBody` panics on its third call (a counter in the closure); assert `Add` returns an error naming the rule, `Stats` has no row for it, and a following `Add` of a good app gets the next id and runs.

### A3 — should-fix — `Stop`/`Undeploy`/`Close` on a deployed-but-never-started app blocks to the deadline
- **What breaks:** MUST 4 (`:57`) starts flushers and the recorder at `Activate`; `Drain` is "unchanged" (`:41`); `Close` "drains and removes every live app" (`:49`). A drain of an inactive app pushes a marker to each sink queue (`runtime.go:1046-1050`) that no flusher will ever close, and `rStop`/`rMarker` on a recorder queue no goroutine reads (`:849-853`); `Drain` then waits at `:607-615` to the deadline and returns `ctx.Err()`, and `:616-623` marks the recording failed. `Close` of a runtime holding one such app always returns `ctx.Err()`, so MUST 5's "when `Close` returns without error" never happens; `Undeploy` without `Start` cannot reach `Remove` ("removing an app that is not drained is an error", `:46`).
- **Fix:** add the transition `inactive → removed`: "`Remove` accepts an inactive app: no barrier is needed, nothing was ever delivered. `Drain` of an inactive app marks it drained and returns `DrainResult{}` at once, without markers; with a recording nothing is written (no header was). `Close` treats an inactive app as `Remove` only." Facade: `Undeploy` on a never-started handle = `runtime.Remove`; `Stop` on it returns `StopReport{}`, nil, and the app moves to stopped.
- **Test:** `TestCloseWithAnInactiveApp` (`internal/runtime`): `New`, `Add` (no `Activate`), `Close(ctx)` with a 5 s deadline returns nil, wall time under 1 s (measured with the test's clock, no sleep), goleak clean. Facade: `TestUndeployWithoutStart`: `Deploy`, `Undeploy` → nil; `SendRow` → `ErrUndeployed`.

### A4 — should-fix — tables cannot be loaded before the sources open
- **What breaks:** ADR-0012 §3 (`:14`) orders tables before sources at `Start`; the note's MUST 3 (`:56`) refuses `TableWrite` before `Activate`, and MUST 8 (`:130`) says "tables start empty and `Load` fills them" — but `Load` can only be called after `Start` has returned, by which time the sources are open (`:97` "sources last"), so the first events of an `x in T` rule read an empty table: a missed detection (ground rule 15). A pre-`Start` write cannot be recorded either, since the recorder's header is the first record at `Activate` (`:35-36`).
- **Fix (signature to shape now):** `func WithTableRows(table string, rows []func(cepio.RowWriter) error) DeployOption // applied by Start after the recorder's header and before any source opens`. In `Start`: `Activate` (header first) → for each option, `runtime.TableWrite(ctx, id, TablePut, rows)` → open sources. Alternatively allow `Table.Load` in `inactive` and have `Activate` apply the queued loads after the header; the option is simpler and matches the ADR's step.
- **Test:** `TestTableRowsAreVisibleToTheFirstEvent` (facade): rule `from E where id in T insert into Out`, `WithTableRows("T", one row)`, an `inmemory.Source` whose `Open` emits the matching event immediately; `Stop`; assert exactly one `OutEvent`.

### A5 — should-fix — `Start` after `Stop` departs from owner F5 without saying so
- **What breaks:** `decisions.md:358` (owner, 2026-11-05): "S3.3's `Start` after `Stop` builds a fresh app." The note (`:39`): "activating a drained or removed app is an error", and the facade's error list (`:122`) has no name for it.
- **Fix:** state one of: (a) "`Start` after `Stop` returns `ErrStopped`; a restart is a new `Deploy` (annotates F5: the fresh app is the caller's `Deploy`)" — my recommendation (the kept `plan` and a reused recording writer make an in-place rebuild ambiguous); or (b) implement F5: `Start` on a stopped handle `Add`s a new app from the kept plan (new `AppID`, empty tables, the same bindings) — then `Stats`' old slot and the new one both exist. Owner question Q4.
- **Test:** (a) `TestStartAfterStopIsErrStopped`; (b) `TestStartAfterStopIsAFreshApp` asserting a new id and empty tables.

### A6 — should-fix — `EmitFunc → Emitter` and `SendRow`'s new parameters are unnamed departures
- **Evidence:** table row 1–2 above (`ADR-0008:13-15, :28`; `ARCHITECTURE.md:109`; `sprint-plan.md:113`; `ADR-0012:35`; note `:109, :189-198`).
- **Fix:** add to §5.3: "The batch-close signal needs a second entry point beside `Emit`, so ADR-0008 §1's `EmitFunc` (a func, batch id second) becomes the two-method `Emitter` (batch id first, `ctx` added, as `SendRow` has); ADR-0012's `SendRow(fill)` gains `ctx` and `batchID` for the same reason. ADR-0008 §1, ADR-0012 §3's note and ARCHITECTURE §3's `io/` line are annotated in this PR." The `apidiff` and golden rows then describe exactly this surface.
- **Test:** the export golden itself (exit 5 proof (a)).

### A7 — should-fix — "the engine does not retry" contradicts ADR-0008 §2 and SEMANTICS §10 unannotated
- **Evidence:** `ADR-0008:36` (flusher retries with backoff, `sink_failures`); `SEMANTICS.md:196`; note `:202-204`; as built `runtime.go:1320-1329` counts and drops.
- **Fix:** name the departure in §5.2 and annotate ADR-0008 §2 in the PR: "retry with backoff is the connector's (S5.1's http sink), not the flusher's; the engine counts (`SinkStats.Errors`) and drops; at-least-once holds through the connector". Owner question Q5 (the alternative — the flusher retries at S5.1 — changes nothing public now, but `Write`'s doc comment is public text a third party will rely on).
- **Test:** `TestSinkErrorIsCountedAndDropped` already exists in spirit (`review F8`); add the doc sentence to the golden's signature comment? No — the golden has no prose; the contract test is the facade's `TestSinkWriteErrorCountsOnce`.

### A8 — should-fix — the `Table` API's `ctx` and the `Delete` key fill are unnamed departures from ADR-0012 §5
- **Evidence:** `ADR-0012:20` (`Delete(key any)`, no ctx); note `:114-116`; writes go through the intake (`runtime.go:559-585`, owner F14 `decisions.md:357`).
- **Fix:** one sentence in §4: "ADR-0012 §5's `Delete(key any)` would cross an `internal/event.Value`; the key is written through the same `RowWriter`, and every table call takes `ctx` because a write waits for the router (F14)." Also define how `Delete`'s `RowWriter` maps indexes: the runtime's delete row holds key columns in key order (`runtime.go:560`), while `AttrIndex` names table attributes — say the facade maps attribute index → key position and refuses a non-key `Set` with `ErrNotAKeyColumn`-style text naming the attribute (later **A21** folded here).
- **Test:** `TestDeleteRefusesANonKeyColumn`.

### A9 — should-fix — no `cep.Event`; ADR-0001 §2 says the facade holds "the public Event view"
- **Evidence:** `ADR-0001:11, :23`; note `:118-124`, `:211-216`.
- **Fix:** name it: "the public event view is `cepio.OutEvent` with `RowReader`, so a connector and an embedder read outputs through one type; `cep` exports no `Event`"; annotate ADR-0001 §2 in the PR. Also state `OutEvent.Row`'s validity (`Canonical` is "valid until `Write` returns", `:215`; `Row` is over the output's own values — `plan.go:386` clones them — so it is safe to retain; say so, or say "copy both").
- **Test:** `TestInmemorySinkRecordsSurviveTheNextWrite`.

### A10 — should-fix — the facade may not start goroutines or read the wall clock, and the note has it do both
- **What breaks:** `s3-3-1-design.md:219` "the facade starts it [`Open`'s goroutine]" and `:77` "`Now` … nil: time.Now". `tools/archcheck/main.go:279`: `go` statements are allowed in `internal/runtime`, `io/*`, `cmd` only; `:253-254`: wall-clock calls in `internal/runtime`, `io`, `internal/playback`, `cmd` only. Package `cep` (dir `""`) is in neither list: T0 red on the first commit.
- **Fix:** move the source goroutines into the runtime, which owns every engine goroutine (ADR-0008 §2 as amended): `func (r *Runtime) Activate(ctx context.Context, app AppID, run ...func(ctx context.Context)) error` — each `run` starts on its own goroutine with a per-app context after the sinks, tables and recorder; `Drain` cancels that context first and waits for every `run` to return, bounded by `ctx` (this is also A12's bound and ADR-0012 §3's "stop sources FIRST" in the place the drain lives); the facade passes `func(ctx) { _ = s.Open(ctx, emitter) }`. `Now`: the facade converts only a non-nil `Config.Now` (`rc.Now = func() int64 { return cfg.Now().UnixMilli() }`) and leaves nil to the runtime's default (`runtime.go:323-325`).
- **Test:** the `archcheck` row at T0 on the facade's first file; `TestDrainCancelsTheSourcesBeforeTheBarrier` (`internal/runtime`): a `run` that records the order "ctx done" before the drain's ack.

### A11 — should-fix — `Stats.Sinks map[string]SinkStats` cannot hold several sinks on one stream
- **Evidence:** note `:124` vs `:136` ("any number of sinks"); runtime `SinkHigh [][]int // [app][binding]`, `SinkErrors [][]int64` (`runtime.go:131, :133`).
- **Fix:** `Sinks map[string][]SinkStats // per derived stream, its bindings in WithSink order` with `type SinkStats struct{ Errors int64; QueueHigh int }`. Shape now: S5.1's `/metrics` needs it and a map value type change is breaking under `apidiff`.
- **Test:** `TestStatsHoldOneRowPerSinkBinding`: two `inmemory.Sink`s on one stream, one returning an error; assert `len(Sinks["out"]) == 2` and `Sinks["out"][1].Errors == n`.

### A12 — should-fix — `Stop`'s wait for `Open` is unbounded unless the note says otherwise
- **Evidence:** note `:130` "waits for them to return"; `:196` "the engine … cancels their ctx first at Stop, waiting for Open to return"; scratch `stuckopen` (exit 0): cancel cannot end an `Open` that ignores ctx; the goroutine stays.
- **Fix:** "`Stop` waits for every `Open` until `ctx`'s deadline. Past it, `Stop` still drains — nothing already in the intake is lost — and returns `StopReport` with `fmt.Errorf("cep: source of stream %q did not return from Open before the deadline: %w", stream, ctx.Err())`; MUST 5 then cannot hold for that app, and the embedder's goleak must expect it." (With A10 this lives in `Drain`.)
- **Test:** `TestStopWithASourceThatIgnoresCancel`: assert the error wraps `context.DeadlineExceeded`, the drain ran (`AfterDrain` counts a late `Emit`), and `Undeploy` still succeeds.

### A13 — should-fix — a `Write` that never returns: say what `Stop`, `Undeploy`, `Close` do and when `Write`'s ctx ends
- **Evidence:** note `:220` ("the app's lifetime context"); `runtime.go:607-615`, `:636-647` (comment: "A Sink that never returns keeps its flusher: Close then returns ctx's error").
- **Fix:** "`Write`'s ctx is cancelled at `Remove` after the drain, and at `Close`'s deadline. A `Write` that does not return holds `Stop` to its deadline (`ctx.Err()`, every other output delivered), and `Close` returns `ctx.Err()` with that flusher still running: MUST 5 holds only for sinks that honour ctx." Document in `Sink.Write`'s comment: "MUST return when ctx is done."
- **Test:** `TestStopWithAStuckSink`: `Stop` returns `context.DeadlineExceeded`; `Stats.Sinks["out"][0].QueueHigh ≥ 1`; goleak with `IgnoreTopFunction` for the planted sink.

### A14 — should-fix — the transition table has undefined cells
- **Evidence:** §3 above: `Start` twice (no error name in `:122`), `Stop` twice, `Undeploy` on removed, `Input`/`Table` after `Undeploy`, `Deploy` with a duplicate `name`, every call after `Close` (today `ErrClosed` wins, `runtime.go:455-457`).
- **Fix:** put the §3 table into the note with these values: `Start` on active → `ErrStarted` (new sentinel) ; `Stop` on drained → the same `StopReport`, nil; `Undeploy` on removed → `ErrUndeployed`; `Input`/`Table` on any state → the handle (use fails with the state's error); after `Close` → `ErrClosed` everywhere; duplicate `name` → allowed (a label for diagnostics, not a key).
- **Test:** one table-driven `TestLifecycleTable` (facade) with a row per cell.

### A15 — should-fix — `apipartition` over `go/parser` under-states the surface; the row's scope and `min_wants` are unspecified
- **Evidence:** scratch `listing` (exit 0): `Handle.Promoted` (through an embedded unexported type) and `New().Reachable` (an exported method of an unexported type returned by an exported func) are callable API the exported-top-level lister never prints, while reflect shows both. §5 above for the row shape (ADR-0015 §1.1 `:27-31, :52-55`; `registry.json` rows carry `expected_scope` kind + `min_wants`).
- **Fix:** the lister prints every method whose name is exported regardless of its receiver's exportedness (`cep hidden.Reachable method func (*hidden) Reachable() string`), every embedded field of an exported struct, and every exported field; the review reads the receiver. Row `apipartition`: `"expected_scope": ["go", ".", "./io", "./io/inmemory"]`, `GATE_COVERED 3`, `"min_wants": 3`, `scope: ["prod"]`, tiers T2–T4; `clockgolden`: `["go", "./internal/clock"]`, `min_wants 1`. Proof (a) gains a planted exported method on an unexported type.
- **Test:** `tools/apipartition`'s unit test over a fixture package containing exactly the `listing` scratch's `fx`.

### A16 — later — `io` has zero statements and no planned test; the coverage row fails "no tests"
- **Evidence:** `gate/coverage/classify.py:107` (FAIL "no tests"), `:114-117` (0 statements passes only with ≥ 1 passing test); `registry.json:322-328` lists `./io/...`.
- **Fix:** `io/cepio_test.go` with `TestModeValues` asserting `Append == 1`, `Changelog == 2`.

### A17 — should-fix — the `RowWriter` is valid only during `fill`, and `inmemory.Push` runs `fill` on the pump's goroutine
- **Evidence:** scratch `pool` (exit 0): a writer kept past `fill` aliases the next caller's row; note `:171` (pooled rows), `:222` (`Push(ctx, batchID, fill)` with `Open` pumping).
- **Fix:** `RowWriter`'s doc: "valid only during the `fill` that received it; a copy kept past it writes into another caller's row". `inmemory.Source`: "`fill` runs on the goroutine that called `Open`; it must not touch the pusher's state without synchronisation" — or run `fill` on the pusher's goroutine into a buffered row the pump emits (one allocation per `Push`; acceptable off the hot path). `Push` after `Close()`/after `Open` returned → `ErrStopped`; `Push` before `Open` is called → blocks on ctx only.
- **Test:** `TestPushAfterStopReturnsAnError`; `TestFillRunsOnThePusherGoroutine` (goroutine id via a channel of `t.Name()`-tagged values is not available; assert through a mutex-free counter under `-race`).

### A18 — later — the `cep` import list omits `internal/model`
- **Evidence:** `validate.go:31-33`; note `:64, :131`; `rules.json:21` has it.
- **Fix:** add `internal/model` to the sentence.

### A19 — later — "built on every shard" at `Add` vs "compiles its bodies at `Activate`"
- **Evidence:** note `:32` vs `:128`; today the build is at `Start` (`runtime.go:1131`), and plan compiles once at `Build` to fail at deploy (`plan.go:154`).
- **Fix:** pick `Add` (a compile failure is then a `DeployError`, as `:127` promises) and fix MUST 6's wording.

### A20 — should-fix — a per-app `Stop` has no "ticks stop before the sources" step (owner D3, S3.1-1)
- **Evidence:** `ADR-0012:16` (D3 annotation); note `:130`; the ticker is runtime-wide (`runtime.go:651-667`) and skips only drained apps (`:815-817`).
- **Fix:** either an intake item `iQuiet{app}` that `Stop` enqueues before cancelling the sources (ticks skip a quiet app; events are still accepted; a recording writes a `quiet` record? — no: ticks simply stop appearing), or an annotation that for a per-app stop the window between source cancel and the drain keeps ticking and why that fabricates nothing (every event a source still holds reaches the intake before `Open` returns, and the drain follows). No absence exists before S4.1, so no detection changes now; the contract is frozen now. Owner question Q6.
- **Test:** `TestStopTicksBeforeSources` (S4.1 can fill it; the hook exists now).

### A21 — folded into A8.

### A22 — later — "never call the lifecycle from a connector callback"
- **Fix:** one sentence in `Sink.Write`'s and `Source.Open`'s docs: `Stop`/`Undeploy`/`Close` from inside them deadlocks to the deadline.

### A23 — later — `Deploy`'s `name` is not a key; per-app options vs ADR-0008 §5's `cep.Config`
- **Fix:** say both (table rows 6 and the last cell of §3).

### A24 — later — a half-written pooled row after a `fill` panic
- **Fix:** `clear(row.vals)` on `Get` (or on `Put`); `SendRow` does not recover `fill`'s panic (documented).

### A25 — later — the note's §3.1 argues with itself ("E-CONN-001? No new code:")
- **Fix:** state one behaviour (Q2) and move the argument to §9.

---

## What held

- The import law: `cep`'s and `io/inmemory`'s lists are inside `rules.json:21, :23-24`; `tools/apipartition` is outside `archcheck`'s scope by the row's own excludes (`registry.json:94`).
- `Add`/`Remove` as intake items with shard acks follow ADR-0006 §5's wait table exactly as `Drain` does today (`runtime.go:834-855`, `:1037-1055`); the router services handoff while blocked (`:733-751`), so no new deadlock class appears.
- `Drain` twice returning the same result (`:589`, `:836-843`) gives `Stop` its retry-after-timeout for free (owner, `decisions.md:336`).
- The value boundary's checks (MUST 10) land on checks `Send` already makes (`runtime.go:517-544`, `boundary.go:20-43`), so the facade adds the index/type/unset checks only.
- `Deploy` keeping no model (MUST 6) is consistent with `validate.Check` returning a deep copy (`validate.go:35-61`) and `plan.Build` consuming it (`plan.go:66-88`).
- Two apps' stream names colliding is harmless by construction (per-app ids, `runtime.go:343-348`).
- The S3.3-2 sandbox fits `Deploy`'s signature additively (§6).
- The clock golden and `apidiff`'s "red only when the golden is unchanged" rule are coherent with ADR-0015 §2's `apipartition` row (`:103`) and the owner's decision 3 (`decisions.md:498`).
- `cepio` as a leaf with no `go` statements satisfies `puritycheck`'s `syncOK` for `io` (`archcheck/main.go:280`).

## Questions only the owner can answer (recommendation first)

- **Q4 — `Start` after `Stop`.** Recommend: an error (`ErrStopped`); a restart is a new `Deploy`; annotate F5 (`decisions.md:358`). Alternative: `Start` rebuilds a fresh app from the kept plan under a new id (more code, ambiguous recording and `Stats`).
- **Q5 — Sink retry.** Recommend: out of the engine for good — the flusher counts and drops (as built), the http sink retries (S5.1); annotate ADR-0008 §2 and SEMANTICS §10. Alternative: the flusher retries with backoff at S5.1 (no public change now, but `Write`'s public doc must not promise "dropped" today).
- **Q6 — Tables before sources.** Recommend: `WithTableRows` now (A4; ADR-0012 §3's step, about 0.3 point). Alternative: document that the first events after `Start` may read an empty table (a missed detection the embedder must design around).
- **On the note's Q1–Q3:** Q1 record the batch-close when S8 reads it (the note's choice; a new record kind is additive for the reader, D6 faults only malformed lines). Q2 an error now, through the `ConnectorFactory` hook of P2 so S5.1 is additive. Q3 the note's rule (red only when the golden did not move), plus the typed `apidiff` report always printed.
