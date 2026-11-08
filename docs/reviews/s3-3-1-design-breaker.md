# S3.3-1 design note v1 — the breaker's report

> **Historical — S3.3-1, the public Go API: `cep`, the connector contracts and the API gate; merged in PR #23.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-07--s33s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-3-1-design.md) · [record](../sprints/s3-3-1-record.md) · reviews: **design: breaker**, [design: challenger](s3-3-1-design-challenger.md), [design: architecture](s3-3-1-design-architecture.md), [design: QA](s3-3-1-design-qa.md), [combined build: architecture](s3-3-1-combined-architecture.md), [break-it: QA](s3-3-1-breakit-qa.md). **← [S3.2c](../sprints/s3-2c-record.md) · [S3.3-2 (next)](../sprints/sprint-plan.md#status) →**

Reviewed: `docs/sprints/s3-3-1-design.md` at ce272a4 (worktree `/Users/ramith/code/cep-wt/s3-3-1-panel`). Every file:line below is in that worktree. Scratch programs and their outputs are under `/private/tmp/claude-501/-Users-ramith-code-cep/c32ea7c4-ab50-4044-8f33-e9ed338fa3bd/scratchpad/panel/breaker/` (`apprace/`, `repo/internal/runtime/zz_breaker_pooled_row_test.go` + `repo/pooled.txt`, `apipart/`, `apidiff/`). Every exit code was read from a file, never through a pipe. GitNexus was not used: the panel worktree is a detached copy the index does not cover; the source was read directly. Hindsight: no search was needed (no finding disputes a recorded decision; S7 below feeds Q1 with evidence).

**Counts:** 2 blockers (B1, B2), 10 should-fix (S3–S12), 2 later (L13, L14).

---

## Blockers

### B1 — MUST 11's pooled row is a use-after-return: `runtime.Send` keeps `e.Values`; the next `SendRow` overwrites what the router, the shards and the recorder are still reading

**What breaks:** a data race and a wrong recording (replay then diverges from live: a lost/false detection on replay). The note (§4, "the row is pooled, `fill` writes in place"; MUST 11 "no allocation per `SendRow`") assumes the engine copies the values. It does not.

**Evidence.**
- `internal/runtime/runtime.go:508` (Send's doc): `// first (review F9) and takes e.Values: the caller MUST NOT change them afterwards (review F4).` and `:545`: `return r.enqueue(ctx, item{kind: iEvent, app: app, ev: e})` — the slice goes into the intake as is.
- The router reads and writes it on its own goroutine after `Send` returned: `internal/runtime/ingest.go:63` `e.Invisibles = text.IngestFlags(e.Values)` and `:65` `e.Values[g.flags[e.StreamID]] = …`; the shards read `m.ev.Values` in every body; the recorder goroutine reads it at `internal/runtime/recorder.go:183` `err = q.rec.Event(&it.ev, …)`.
- Scratch: `repo/internal/runtime/zz_breaker_pooled_row_test.go` — one `Send` with `row`, then `row[0].Str = "b"` (what the next pooled `fill` does), under `-race`. `go test -race` **exit 1**: `WARNING: DATA RACE — Read at … by goroutine 22: text.IngestFlags() ingest.go:31 ← (*Ingest).Event() ingest.go:63 ← (*router).event() runtime.go:882 ← (*router).ingest() runtime.go:798` vs the write at the test's line 41; and the recorder double logged `the recorder saw "b" for the first event (sent as "a")` (`repo/pooled.txt`).

**Fix (write this into §4 and MUST 11).** The values slice is the engine's from `Send` on (pattern state, the table snapshot's event, the recording all keep it); only the writer object is pooled. Replace MUST 11 with:

> MUST 11: `SendRow` allocates exactly one `[]event.Value` of `schema.NumAttrs()` per call and hands it to `runtime.Send`, which keeps it (runtime.go:508); the `RowWriter` object (its set-flags, its first error, its schema) is pooled. `allocgate`: `AllocsPerRun` on `SendRow` is `runtime.Send`'s + 1.

Proposed facade code (package `cep`):
```go
type row struct {
	schema event.Schema
	values []event.Value // fresh per SendRow: the engine keeps it (runtime.Send, runtime.go:508)
	set    []bool        // pooled, cleared per call
	err    error         // the first wrong Set, reported by SendRow
}

func (h *InputHandler) SendRow(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error {
	w := h.pool.Get().(*row)
	defer func() { w.values = nil; w.err = nil; h.pool.Put(w) }()
	w.values = make([]event.Value, h.schema.NumAttrs()) // the one allocation; never returned to the pool
	clear(w.set)
	if err := fill(w); err != nil {
		return err
	}
	if w.err != nil {
		return w.err
	}
	if err := w.unsetCheck(); err != nil { // MUST 10's "left unset" checks
		return err
	}
	return h.rt.Send(ctx, h.app, event.Event{StreamID: h.stream, Ts: w.ts(), Values: w.values, BatchID: batchID})
}
```
The same holds for `Table.Upsert`/`Load`: `TableWrite` copies rows for the record (`runtime.go:1328–1336 copyRows`) and `table.Upsert` stores `prepare`'s result (`internal/table/table.go:114–132`) — not verified that `prepare` copies; the facade should allocate the table row per call too and say so.

**Test that catches it:** `cep.TestSendRowValuesAreNotReused` — deploy an app with `WithRecording` into a recorder double whose `Event` reads `e.Values[0]`; call `SendRow` twice with `"a"` then `"b"` on one handler; after `Stop`, assert the double saw `["a","b"]` in order. Run under `-race` (the gate's race row): red today's design (exit 1), green with the fix. Plus `TestSendRowAllocations`: `testing.AllocsPerRun(1000, …)` on `SendRow` equals `AllocsPerRun` on `runtime.Send` + 1.

### B2 — MUST 1 ("the router is the only goroutine that reads or writes `r.apps`") cannot hold: `appOf` reads `r.apps` on the caller's goroutine, and `Add` makes the router append to it

**What breaks:** a data race on the slice header (and a torn read: a caller indexes a slice whose backing array `append` just moved) on every `Send`/`SendPayload`/`TableWrite`/`Drain` concurrent with an `Add`. Today there is no race only because `r.apps` is complete before any goroutine starts (`runtime.go:370` `r.apps = append(r.apps, a)` inside `build`, before `Start` launches the router at `:286`).

**Evidence.** `internal/runtime/runtime.go:454–461`:
```go
func (r *Runtime) appOf(app AppID) (*appState, error) {
	if r.isClosed() {
		return nil, ErrClosed
	}
	if app < 0 || int(app) >= len(r.apps) {
		return nil, fmt.Errorf("runtime: no app %d", app)
	}
	return r.apps[app], nil
}
```
`Send` (`:509`), `TableWrite` (`:561`), `Drain` (`:590`), `SendPayload` (`boundary.go`) all call it on the caller's goroutine. Scratch `apprace/main.go` (a goroutine appending to `r.apps` while another indexes `r.apps[0]`): `go run -race` **exit 1**, `WARNING: DATA RACE` read at main.go:26 / write at main.go:20.

**Fix (replace MUST 1's first sentence; the rest of MUST 1 stands).**
> MUST 1: the router is the only goroutine that **writes** the app list, and it writes it copy-on-write: `r.apps` is an `atomic.Pointer[[]*appState]`; `iAdd` and `iRemove` store a fresh slice; callers `Load()` a snapshot in `appOf`. `shard` per-app state and the routes stay the router's/shard's only.

Proposed code:
```go
type Runtime struct {
	start int64
	cfg   Config
	apps  atomic.Pointer[[]*appState] // copy-on-write by the router (iAdd, iRemove); callers Load a snapshot
	// … unchanged
}

func (r *Runtime) appOf(app AppID) (*appState, error) {
	if r.isClosed() {
		return nil, ErrClosed
	}
	apps := *r.apps.Load()
	if app < 0 || int(app) >= len(apps) {
		return nil, fmt.Errorf("runtime: no app %d", app)
	}
	if apps[app] == nil {
		return nil, ErrRemoved
	}
	return apps[app], nil
}

// in (*router).ingest, case iAdd:
next := append(slices.Clone(*r.apps.Load()), a) // a: the built appState; its id is len(old)
r.apps.Store(&next)
// case iRemove, once every shard acked mRemove:
next := slices.Clone(*r.apps.Load())
next[it.app] = nil
r.apps.Store(&next)
```
Every `for _, a := range r.apps` in the router (`:814`, `:859`, `:1075`) becomes `for _, a := range *r.apps.Load()` (the router may also keep its own plain copy; S3 covers the nil slots).

**Test that catches it:** `runtime.TestAddWhileSending` — `New`, `Add`+`Activate` app 0; goroutine A loops `Send(ctx, 0, ev)` 10,000 times; main calls `Add` 100 times meanwhile; join; assert every `Send` returned nil and the test is race-free under `-race` (red with a plain slice: exit 1).

---

## Should-fix

### S3 — MUST 2's nil slot crashes the router in three loops the note does not name

**What breaks:** a nil-pointer panic on the router goroutine (the engine stops) at the first `Tick` or `Stats` after a `Remove`, unless each loop skips nil. The note says "`Stats` keeps the slot as zero rows" but names no loop.

**Evidence.** `runtime.go:812–818` (`case iTick: … for _, a := range r.apps { if a.drained {`), `:856–866` (`case iStats: … for _, a := range r.apps { st.Streams = append(st.Streams, streamStats(a))` → `streamStats` dereferences `a.app.Streams` at `:498–506`), `:1075–1086` (`take`, `oStats`: `for _, a := range r.apps { errs := make([]int64, len(a.all))`).

**Fix:** add to MUST 2: "the router's three per-app loops (`iTick`, `iStats`, `take`'s `oStats`) skip a nil slot; `iStats` and `take` append zero rows for it." Code for `iStats`:
```go
for _, a := range *r.apps.Load() {
	if a == nil { // removed: zero rows keep [app] indexes stable (MUST 2)
		st.Streams, st.Recording, st.Rules = append(st.Streams, nil), append(st.Recording, RecordingStats{}), append(st.Rules, nil)
		continue
	}
	…
}
```
and in `take`: `if a == nil { w.stats.SinkErrors = append(w.stats.SinkErrors, nil); w.stats.SinkHigh = append(w.stats.SinkHigh, nil); continue }`; in `iTick`: `if a == nil || a.drained { continue }`. The shard's `stats()` (`:1246–1262`) must keep an empty row for a removed app (`sh.rules[ai] = nil`, never `slices.Delete`), or `take`'s `it.stats.rules[ai]` indexing (`:1055`) shifts.

**Test:** `runtime.TestRemoveLeavesZeroRows` — Add 2 apps, Activate both, Drain and Remove app 0, then `Tick`, then `Stats`: no panic; `len(st.Streams) == 2 && len(st.Streams[0]) == 0 && len(st.Rules[1]) == <app 1's rules>`; `Send(ctx, 0, ev)` returns `ErrRemoved`.

### S4 — §2 `Add` and §3 MUST 6 contradict each other on when bodies are built, and `Add` has no rollback when a shard's `NewBody` fails

**Evidence.** §2 `Add`: "its schemas and rules are built on every shard"; §3 MUST 6: "the plan compiles its bodies at `Activate` (as `runtime.build` does today from `NewBody`)". Today `NewBody` runs in `newShard` (`runtime.go:1109–1150`, `:1131 body, err := newBody(rule, emit)`), i.e. at build, not at activation. With `mAdd` on each shard, shard 1's `NewBody` can fail after shard 0 built (`:1132–1134` returns an error).

**Fix:** keep §2 (build at `Add`, on each shard, in `process(mAdd)`), and rewrite MUST 6's clause to: "the plan's `NewBody` runs on each shard when it processes `mAdd`; the model is unreachable from the facade after `plan.Build`." Add to `Add`'s contract: "a shard whose `NewBody` fails acks `mAdd` with the error; the router then sends `mRemove` to every shard, replies the error, and the slot stays nil (the id is consumed)." Code sketch for the shard:
```go
case mAdd:
	if err := sh.install(m.app, m.add); err != nil { // builds rules/bodies/reads for the new slot
		sh.uninstall(m.app)
		sh.outbox = append(sh.outbox, outItem{kind: oAddAck, app: m.app, token: m.token, err: err})
		return
	}
	sh.outbox = append(sh.outbox, outItem{kind: oAddAck, app: m.app, token: m.token})
```
**Test:** `runtime.TestAddRollsBackOnBodyError` — `Config{Shards: 2}`, an app whose `NewBody` panics on its second call; `Add` returns an error containing `NewBody panicked`; `Stats` shows one zero-row slot; a second `Add` of a good app returns id 1 and works.

### S5 — `Remove`'s "sink queues are closed once flushed" makes today's flusher spin, calling the sink with empty events

**Evidence.** `runtime.go:1297–1311`:
```go
	select {
	case it := <-q.q:
		if r.stopping() { …return }
		if it.marker != nil { close(it.marker); continue }
		q.call(it.ev)
```
A receive from a closed channel returns the zero `sinkItem{}` at once, forever: `q.call(event.Event{})` runs in a tight loop until `Close`, and the sink gets an event with no values (a third-party `Write` on it panics → counted as an error, `:1317–1326`, thousands of times a second).

**Fix:** `case it, ok := <-q.q: if !ok { return } // Remove closed the queue after its last marker: the flusher's end`. The router is `q.q`'s only sender, so `close` after the drain's marker was pushed is safe; `Remove` waits for the marker before closing (it already does for the drain). State in §2 `Remove`: "the router closes each queue after the drain's marker; the flusher returns on the closed queue."

**Test:** `runtime.TestRemoveEndsTheFlusher` — a sink counting calls; Add/Activate/one Send/Drain/Remove; assert the count stays 1 after `Remove` returns and `goleak.VerifyNone` passes with the runtime still open (the flusher is gone, the router is not).

### S6 — `Close` "past the deadline … the goroutines are stopped anyway" cannot hold for a flusher inside a third-party `Write`

**Evidence.** `runtime.go:636–637` (today's `Close` doc): "A Sink that never returns keeps its flusher: Close then returns ctx's error." `close(r.stopped)` only makes loops return at their next `select` (`:1297–1311`); nothing interrupts `q.sink(e)` at `:1323`. MUST 5 is stated as "when `Close` returns without error" — right; §2 `Close`'s text and §3's `Close` comment are not.

**Fix:** §2 `Close`: "past the deadline the remaining apps are not drained; every goroutine is signalled and `Close` returns `ctx.Err()`; a `Write` that never returns keeps its goroutine (MUST 5 holds only for a nil return)." §5.2: say exactly when the `Write` context is cancelled: "the app's context is cancelled when `Undeploy` or `Close` gives up at its deadline, never before the drain's flush markers passed (an earlier cancel would drop outputs the barrier promised)."
```go
// facade, per app:
appCtx, cancelApp := context.WithCancel(context.Background())
// Stop: sources' cancel → wait Open → runtime.Drain (markers passed) → nothing cancels appCtx
// Undeploy/Close past the deadline: cancelApp()
```
**Test:** `cep.TestCloseWithAStuckSink` — a sink that blocks on a channel; `Close` with a 50 ms deadline returns `context.DeadlineExceeded`; release the sink; a second `Close` returns nil and goleak is clean.

### S7 — §5.3's "no format change lands before anything replays it" is wrong in the other direction: the recording format already has the `batch_end` record; what is missing is the runtime writing it

**Evidence.** `internal/playback/reader.go:38` `eBatchEnd: {Name: "batch_end", …}`, `:60` `{eBatchEnd, RecBatchEnd, "batch end", …}`, `:424–435` `batchEndRecord` (an empty batch id is a fault at `:432`); `internal/playback/driver.go:79–80` `case RecBatchEnd: st.Kind, st.Stream, st.Batch = StepBatchEnd, …`; `trace.go:99–102`; `testdata/log/exit1.jsonl:6` `{"batch_end":"b1","stream":"A"}`. The `runtime.Recorder` interface (`runtime.go:55–62`) has no `BatchEnd`, and `playback/recorder.go` (`:51–198`) writes none. So with this note, every recording made from S3.3-1 to S8 lacks the punctuations SEMANTICS §10 (line 197: "same batch punctuations") calls part of the replay contract, while the reader would accept them. The "+0.5, a new record kind" in Q1 overstates the cost: it is one interface method, one `recItem` kind and one encoder line, no format change.

**Fix (recommend to the owner for Q1: record now).**
```go
// internal/runtime/runtime.go, Recorder:
BatchEnd(stream int, batch string) error // a source's batch-close signal, in intake order (SEMANTICS §7, §10)
// internal/runtime: item kind iBatchEnd{app, stream, batch}; the router: if a.drained { a.after[stream]++; return }; a.ing.stamp raised as Reject does; a.rec.put(recItem{kind: rBatchEnd, stream, batch}); counts StreamStats.Batches
// internal/playback/recorder.go:
func (r *Recorder) BatchEnd(stream int, batch string) error {
	b := append(r.buf[:0], `{"batch_end":`...)
	b = appendString(b, batch)
	b = append(b, `,"stream":`...)
	b = appendString(b, r.streamName(stream)) // the same name Event writes at recorder.go:63–105
	return r.line(append(b, '}'))
}
```
And two contract lines in §5.3: `EndBatch(ctx, "")` is an error (`reader.go:432` refuses an empty id), and `Emit`/`SendRow` with `batchID == ""` means "no batch" (`encode.go:174` omits the key).

**Test:** `runtime.TestRecorderRecordsBatchEnd` — after one `Send` and one `EndBatch`, the double's log is `["header","event","batch_end","stop"]`; `playback.TestBatchEndRoundTrip` — a `NewRecorder` recording with one `BatchEnd("b1")` reads back through `Reader` as `Record{Kind: RecBatchEnd, Stream: 0, Batch: "b1"}`.

### S8 — `apipartition`'s `go/printer` text is not canonical: a gofmt-valid reflow is an "API Δ"

**Evidence.** Scratch `apipart/tool/main.go` (the §6 tool as written, 30 lines) over `apipart/fixture/a.go`, run on two gofmt-valid formattings of the same declarations: `diff out1.txt out2.txt` **exit 1** —
```
< fixture func func (o *opts) Apply(ctx context.Context) error
> fixture func func (o *opts) Apply(⏎	ctx context.Context,⏎) error
< fixture func func Map[T any, U comparable](xs []T, f func(T) U) []U
> fixture func func Map[T any, U comparable](xs []T,⏎	f func(T) U) []U
< fixture type Handle struct {⏎	*inner⏎	Name	string⏎	count	int⏎}
> fixture type Handle struct {⏎	*inner⏎⏎	Name	string⏎	count	int⏎}
```
`go/printer` keeps the source's line breaks and trailing commas from the nodes' positions. A reviewer would read a reflow as a signature change, and `--write` would then rewrite the law for nothing.

**Fix:** zero every position in the node before printing; `apipart/tool2/main.go` does it (`zero()` over `ast.Inspect`) and the two formattings then print identically: `diff out1b.txt out2b.txt` **exit 0**. One care: `ast.TypeSpec.Assign` must stay non-zero or the alias prints as `type Alias Handle` (the `=` is lost — seen in `out1b.txt`). Written out:
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
			if n.Assign.IsValid() { n.Assign = 1 } // keep the alias's "=": a valid, position-free marker
		}
		return true
	})
}
```
(Printing a `*ast.FuncDecl` with `Body = nil` and `Doc = nil` already drops comments and bodies — held in the scratch.) Add to §6: "the tool prints each declaration with its positions cleared, so only tokens matter."

**Test:** `apipartition.TestCanonicalAcrossReflow` — two fixture dirs with the same declarations formatted differently produce byte-identical output; `TestAliasKeepsEquals` — `type Alias = Handle` prints with ` = `.

### S9 — what the parser-only tool prints for methods is partly noise and partly blind

**Evidence** (`apipart/out1.txt`): it prints `func (o *opts) Apply(ctx context.Context) error` (an exported method on an unexported receiver: not API) and `func (i *inner) Promoted() int` (printed as `inner`'s; the API fact is `Handle.Promoted` through the embedded `*inner` — removing the embedding changes the API and the golden line stays). Generic functions and aliases print correctly (`Map[T any, U comparable](…)`, `Alias = Handle`).

**Fix:** skip methods whose receiver base type is unexported, and say in §6 that promoted methods are `apidiff`'s to catch (the golden does not list them):
```go
case *ast.FuncDecl:
	if !ast.IsExported(d.Name.Name) || (d.Recv != nil && !ast.IsExported(recvBase(d.Recv))) {
		continue
	}
// recvBase: the identifier under *T, T, T[P], *T[P]
```
**Test:** `apipartition.TestSkipsUnexportedReceivers` — the fixture's `Apply` and `Promoted` are absent from the output; `Start` is present.

### S10 — the `apidiff` row as written does not run: `-w` takes one import path; the paths are wrong; red is in the report, not the exit code; the allowlist is 14 modules

**Evidence** (`apidiff/`, Go 1.27.3, `GOFLAGS=-mod=mod`): `go get -tool golang.org/x/exp/cmd/apidiff@latest` exit 0 — `go.mod` gains `tool golang.org/x/exp/cmd/apidiff` and four `// indirect` requires (`x/exp`, `x/tools v0.52.1-…`, `x/mod v0.42.0`, `x/sync v0.24.0`), `go.sum` 8 lines; `go tool apidiff -w base.export ./pkg` exit 0; same source: exit 0 and no output; after `Start()` gained a `ctx` parameter: **exit 0** with `Incompatible changes: - (*AppHandle).Start: changed from func() error to func(context.Context) error`; `go tool apidiff -w multi.export ./pkg ./pkg2` **exit 2** (usage: `apidiff -w FILE IMPORT_PATH`); `-m -w mod.export .` and `-m mod.export .` exit 0 (module mode, which also covers `internal/*`). `go mod graph` lists 14 modules in the tool's graph (`x/exp, x/tools, x/mod, x/sync, x/net, x/sys, x/telemetry, go-cmp, goldmark, x/tools/go/expect, x/tools/go/packages/packagestest, go, toolchain, the module`) — only the four are in `go.mod`.

**Fix** for §6's `apidiff` paragraph: (a) one `-w` per package: `go tool apidiff -w base-cep.export .`, `… base-io.export ./io`, `… base-inmemory.export ./io/inmemory` (the facade is the module root, §1: `cep/ (module root)`, so its import path is `.`, not `./cep`; `apipartition`'s row scope `cep/` has the same slip — the root's `*.go`); (b) the row's red is read from the report text (`Incompatible changes:` present and the golden unchanged), never the exit code, which is 0; (c) `tools/deps-allowlist.json` pins the four `go.mod` requires at build-time scope and lists the ten transitive ones as "module graph only, never built" (the owner approved `golang.org/x/exp`, decisions 2026-11-07; the graph's `x/net`, `x/sys`, `x/telemetry`, `goldmark`, `go-cmp` are new names and the owner should see them).

**Test:** the planted proof (b) in §6 as written catches a row that reads the exit code: it would stay green.

### S11 — `Stop`'s ordering holds for a conforming source, but `inmemory.Source` drops what was `Push`ed and not yet pumped, counted nowhere

**Evidence.** `runtime.go:464–474` `enqueue` checks `ctx.Err()` first, then selects on the intake and `ctx.Done()`; an event admitted in the same instant the `Open` context is cancelled sits in the intake ahead of `Stop`'s later `iDrain` (`Stop` waits for `Open` to return before `Drain`, §3 MUST 8) — processed, not `AfterDrain`, not lost. That part stands. But §5.4's channel-fed `Push(ctx, batchID, fill)`: items in the channel when `Open`'s ctx is cancelled are never emitted and never counted (`AfterDrain` counts only what reached `Send`). §8's "Deploy → Start → events → Stop" test is then non-deterministic.

**Fix** (§5.4): "`Open` returns when the source's channel is closed **and empty**, or when ctx is done; `Close()` closes the channel; `Push` after `Close()` returns `cepio.ErrSourceClosed`; events pushed and not pumped when ctx is cancelled are counted in `Source.Dropped() int64`. A test calls `Close()` before `Stop`." Code:
```go
func (s *Source) Open(ctx context.Context, emit cepio.Emitter) error {
	for {
		select {
		case it, ok := <-s.ch:
			if !ok { return nil } // Close(): everything pushed before it was emitted
			if err := emit.Emit(ctx, it.batch, it.fill); err != nil { s.dropped.Add(int64(1 + len(s.ch))); return err }
		case <-ctx.Done():
			s.dropped.Add(int64(len(s.ch)))
			return ctx.Err()
		}
	}
}
```
**Test:** `inmemory.TestCloseThenStopDeliversEverything` — push 1,000, `Close()`, `Stop`; the sink saw 1,000 and `Dropped() == 0`; `TestStopWithoutCloseCounts` — push 1,000 into a slow-emitting app, `Stop` at once; `Dropped() + sink count == 1000`.

### S12 — an added app's `start` is unspecified; the header and the `Ingest` must share one value

**Evidence.** Today `build` takes `start := cfg.Now()` once (`runtime.go:333–335`), uses it for every app's `NewIngest(app.Streams, start, …)` (`:340`) and the header `recItem{kind: rHeader, arrival: start}` (`:354`); the router's stamp floor is the same `r.start` (`:703`). With `Add` hours later, the note says neither which `start` the app's `Ingest` gets nor what the header carries (MUST 4 only moves the header to `Activate`). If they differ, replay's floor is not live's (`sync.go:60–62` `Begin(start)` rebuilds the `Ingest` from the header's start).

**Fix** (§2 `Add`): "the router takes `start := rt.now(r.cfg.Now())` when it installs the app and uses it for both `NewIngest(app.Streams, start, app.Tables, false)` and the header's `rHeader{arrival: start}` put at `Activate`."

**Test:** `runtime.TestAddedAppStartIsItsHeaders` — `Now` injected: t0 at `New`, t1 = t0+1h at `Add`; the recorder double's `Header` argument is t1; the first event's recorded arrival ≥ t1.

---

## Later

### L13 — §8's exit-4 test needs no "facade table mutex"
`ingest.go:71` `e.Tables = g.set.Snapshot()` pins one snapshot per event before routing, and `TableWrite` is applied by the router in intake order (`runtime.go:826–836`); the test is simply 10,000 events with a concurrent `Upsert` loop and the assertion as stated. Drop "holds the `Upsert` behind the facade's table mutex".

### L14 — `cep.Config.Now func() time.Time` vs `runtime.Config.Now func() int64`
Fine; write the conversion in §3 (`func() int64 { return cfg.Now().UnixMilli() }`) so an agent does not pass the facade's func through.

---

## Sizing (§10)

Runtime 1.5 is light: copy-on-write `apps` (B2), `mAdd`/`mRemove` with rollback (S4), the nil-slot loops (S3), the queue close (S5), the app start (S12) — about 2.5. `apipartition`'s canonical form (S8–S9) +0.25; recording `batch_end` now (S7) +0.25. About 9 as a whole; still one sprint.

## What held

- The `tool` directive with `golang.org/x/exp/cmd/apidiff` under Go 1.27.3: `go get -tool`, `-w`, compare, and the incompatible-change report all work (`apidiff/out.txt`, exits 0); the report text names the method and both signatures.
- `apipartition` over `go/parser`+`go/printer` prints generic functions, aliases, vars and consts correctly and drops doc comments and bodies without extra work; with positions cleared it is byte-stable across reflows.
- `Stop`'s order (sources cancelled and awaited, then `Drain`) loses no event of a conforming source: `enqueue`'s `ctx.Err()` precheck and intake FIFO order put any event admitted at cancel time ahead of the drain item.
- `Close` in reverse id order is implementable on today's `Drain`: a repeated drain returns the stored result (`runtime.go:842–849`, `drainRes`).
- MUST 3's atomic state is today's `draining` pattern (`runtime.go:513–515`) extended; nothing blocks a four-state atomic checked before `appOf`'s snapshot.
- D3/D7: `validateApp` already refuses object key parts at the engine (`runtime.go:402–403`); the deploy codes are additive.

## Questions only the owner can answer (plain language, recommendation first)

1. **Q1 (batch-close):** record the signal now. The recording format already reads `batch_end` lines (S7); writing them costs one method and one line, not a format change. Pro: every recording from here on carries the punctuations the replay contract names. Con: a quarter point in a full sprint.
2. **The `apidiff` dependency surface:** the approved `golang.org/x/exp` brings ten more modules into the graph (none built, none vendored). Recommend: pin all four `go.mod` requires in the allowlist and list the other ten as "graph only"; say so in the PR. Alternative: skip the `tool` directive and run `apidiff` from `go run golang.org/x/exp/cmd/apidiff@<pinned>` in the row — same download, no `go.mod` change.
3. **MUST 11's budget:** accept "one allocation per `SendRow`" (the values slice the engine keeps) instead of zero. Recommend yes; the alternative is an engine-side copy, which only moves the allocation.
