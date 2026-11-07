# S3.3-1 design note v1 — the challenger's review (the alternatives not taken)

**Reviewed:** `docs/sprints/s3-3-1-design.md` v1 at ce272a4 (worktree `/Users/ramith/code/cep-wt/s3-3-1-panel`, read-only). Blind: no other review read.
**Code graph:** GitNexus (`mcp__gitnexus__query`, repo `cep`) reported `staleness: behind, commitsBehind: 62` — the index does not have S3.2b-2/S3.2c's code, so every code claim below is from the source at ce272a4 with file:line.
**Memory:** one Hindsight search ("public Go API facade Deploy functional options SendRow RowWriter design decisions") returned only the four generic pages; no recorded decision on any of the seven choices — nothing from memory is used. The decisions cited are read from `docs/sprints/decisions.md` and the ADRs directly.
**Prior art quoted from disk:** Siddhi at `/Users/ramith/code/siddhi` @ 98de1eb41 (v5.1.35-1, the checkout `docs/language/research/siddhi-surface.md` cites): `modules/siddhi-core/src/main/java/io/siddhi/core/{SiddhiAppRuntime.java, SiddhiAppRuntimeImpl.java, SiddhiManager.java, stream/input/InputHandler.java, stream/output/StreamCallback.java, query/processor/stream/window/BatchWindowProcessor.java}`. Esper 8 and Flink 1.x are from my own knowledge and marked "(from memory)" where I could not open a file.
**Scratch programs (all exit 0, read directly):** `/private/tmp/claude-501/-Users-ramith-code-cep/c32ea7c4-ab50-4044-8f33-e9ed338fa3bd/scratchpad/panel/challenger/repo/internal/playback/zz_challenger_test.go` and `zz_challenger2_test.go` (a copy of the worktree, the real `event.Value`, `runtime.Send`, `playback.AppendEvent`; logs `allocs.log`, `allocs2.log`, `bench.log` beside `repo/`), and `.../challenger/probe/main.go` with `.../challenger/fixture/fixture.go` (log `probe.log`). Go 1.27.3 from the pinned cache, `GOTOOLCHAIN=local`, darwin/arm64.

Severity per ground rule 15 and the brief: **blocker** = a contract that cannot hold, a lost output, a race; **should-fix** = wrong with a workaround or misleading; **later** = polish.

---

## The measurements (one table, cited below)

`testing.AllocsPerRun(2000, …)` over a 4-attribute row (string, int64, float64, timestamp), all shapes handing the finished `[]event.Value` to a retained sink (so escape analysis sees the hand-off the engine makes):

| Shape | allocs/op | Note |
|---|---|---|
| `runtime.Send` with a shared `Values` slice (isolates Send's own) | **0** (424 ns/op, 0 B) | violates runtime.go:507-508; the baseline MUST 11 names |
| `runtime.Send` with a fresh `Values` + writer per call (honest) | **2** (445 ns/op, 400 B) | 352 B slice + 48 B writer header |
| `SendRow(ctx, "", fill)` — fresh slice, top-level `fill` | 2 | |
| `SendRow` — fresh slice, closure capturing 4 locals, created per event | 2 | the closure does not allocate: a concrete method only calls it |
| `SendRow` — **pooled row written in place (the note's MUST 11)** | **0** | **unsafe**: the engine keeps the slice (below) |
| `Emitter.Emit` (interface) — same closure per event | 2 | devirtualised in this harness; a real source may pay +1 for the closure — not proven |
| `map[string]any` per event (Siddhi `StreamCallback.toMap` shape) | 4 | plus a name lookup per key |
| `[]any` positional (Siddhi `InputHandler.send(Object[] data)`) | 1 | interface data on the stack; positional = misuse-prone |
| typed struct + precomputed per-field setters (S6.1's `InputFor[T]`) | 3 | 1 with a pooled writer header |
| row-as-value: `r := h.NewRow(); r.Set…; h.Send(ctx, r)` | 1 | the caller still holds `r` after `Send` |

`go test -bench` (`-benchtime=200000x -benchmem`), canonical line of the same event with a batch id = **112 bytes**:

| Canonical | ns/op | B/op | allocs/op |
|---|---|---|---|
| `playback.AppendEvent` into the flusher's buffer (the note: aliased) | 249.6 | 0 | 0 |
| the same + one copy per output (the safe default) | 282.4 | 112 | 1 |
| `AppendEvent` into the **sink's own** buffer (`AppendCanonical`) | 244.0 | 0 | 0 |

`probe` over `internal/runtime` (24 files, imports 5 internal packages): **go/parser + go/printer: 51 exported lines in 2.19 ms; go/types with the source importer: 25 exported names in 1.10 s.** Fixture (an exported `Runtime` embedding an unexported `inner` with an exported method `Hidden`): the parser text prints `type Runtime struct { inner; secret int }` and no `Hidden`; `types.NewMethodSet(*Runtime)` lists `Runtime.Hidden func() int (promoted through an unexported embed: true)`.

---

## (1) `Deploy(ctx, name, src string, opts ...DeployOption)` vs a `DeployConfig` struct vs a builder

**As the note has it (§3):**
```go
func (r *Runtime) Deploy(ctx context.Context, name, src string, opts ...DeployOption) (*AppHandle, error)
type DeployOption func(*deployOptions)
func WithParam(name, literal string) DeployOption
func WithSource(stream string, s cepio.Source) DeployOption
func WithSink(stream string, s cepio.Sink) DeployOption
func WithRecording(w io.Writer) DeployOption
```
**Prior art.** Siddhi: `SiddhiManager.createSiddhiAppRuntime(String siddhiApp)` (SiddhiManager.java:95) takes no options; params and connectors are `@app`/`@source` annotations in the text, sinks by `addCallback(String streamId, StreamCallback)` after creation (SiddhiAppRuntime.java:116). Esper 8 (from memory): `EPDeploymentService.deploy(EPCompiled, DeploymentOptions)` — an options object. Flink: builders per connector (`KafkaSource.builder()…build()`), none for the job itself.
**The struct alternative:**
```go
type DeployConfig struct {
	Params    map[string]string          // ${param} overrides, each written as the rule language writes it
	Sources   map[string]cepio.Source    // by declared source stream
	Sinks     map[string][]cepio.Sink    // by derived stream, in binding order
	Recording io.Writer                  // nil: off
}
func (r *Runtime) Deploy(ctx context.Context, name, src string, cfg DeployConfig) (*AppHandle, error)
```
Buys: a repeated source or param binding is impossible by construction; one type to read (an embedder or an LLM sees the whole surface in one place); the zero value is the default; ADR-0008 §5's "the library user passes the same via `cep.Config`" (params) reads naturally. Costs: map iteration order is random, so the tool must sort stream names before reporting binding errors (else `DeployError.Diagnostics` order is nondeterministic); a `cepio.Source` is a Go value, not config, so `cmd/cepd` (ADR-0008 §5) binds by annotation either way and gains nothing from the struct. A builder buys nothing over four options and adds a type.
**Measurement:** none needed — a deploy-time cost, not a per-event one.
**Verdict: keep the functional options** (the Go idiom every LLM knows; a growing option set never breaks a call), **but the note leaves undefined what a repeated `WithSource("S", a)` / `WithParam("p", …)` / the same `cepio.Sink` value bound twice does — the struct would have settled it structurally.**
- **C1 · should-fix.** What breaks: two `WithSource` on one stream either both pump (duplicate events, nondeterministic interleaving) or the last silently wins; the note does not say. Evidence: §3.1 "A source stream may be bound to one `cepio.Source` (`WithSource`)" — "one" is stated, the second binding's fate is not. Fix — add after that sentence: *"A second `WithSource` for the same stream, a second `WithParam` for the same name, or a second `WithRecording` is a `*DeployError` naming the stream or param (a facade diagnostic, no catalog code); `WithSink` may repeat on a stream and binds in call order."*

## (2) `SendRow(ctx, batchID, fill func(RowWriter) error)` vs typed `Send(ev T)` vs `map[string]any` vs a row the engine returns

**As the note has it (§3, §4):**
```go
func (h *InputHandler) SendRow(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error
```
and §4: *"The facade's `row` implements both over a `[]event.Value` sized by the schema, reused per `InputHandler` (one per caller goroutine; `SendRow` is safe for concurrent use because each call takes a row from a `sync.Pool`)"* and **MUST 11**: *"no allocation per `SendRow` beyond the engine's own (the row is pooled, `fill` writes in place; `allocgate`'s `AllocsPerRun` on `SendRow` with a pooled row: 0 above `runtime.Send`'s)."*

**Prior art.** Siddhi `InputHandler.send(Object[] data)`, `send(long timestamp, Object[] data)`, `send(Event)`, `send(Event[])` (InputHandler.java:51-92) — positional, untyped, boxed; `StreamCallback.toMap(Event)` (StreamCallback.java:68) is the map shape on the way out. Esper 8 (from memory): `EPEventService.sendEventMap/ObjectArray/Bean/Json` plus `EventSender.sendEvent(Object)` from `getEventSender(String)` — the sender is pre-bound to a type, the nearest analogue of a schema-bound handler. Flink: typed `DataStream<T>` end to end, serialisers precomputed per type — the typed form, which ADR-0012's 2026-11-02 annotation defers to S6.1 because Go has no generic methods.

**What the measurement shows.** The fill form is the cheapest *safe* shape: 2 allocs/op (1 with a pooled writer header), equal to the typed struct's floor, better than the map (4) and as cheap as the engine-returned row (1) — and the engine-returned row leaves the caller holding `r` after `Send`, free to mutate a slice the engine owns, which the fill form forbids by scope. The `[]any` form's 1 alloc is bought with positional attributes and runtime type errors — Siddhi's shape, rightly not taken.

**But MUST 11 cannot hold as written.** `runtime.Send` *takes* the slice: runtime.go:507-508 "takes e.Values: the caller MUST NOT change them afterwards (review F4)"; the event travels through the intake channel (runtime.go:545 `r.enqueue(ctx, item{… ev: e})`) into shard and partition state; nothing copies `Values` on the way (the only copy is `copyRows` for table rows, runtime.go:829/1332). So a pooled row cannot be handed back to the pool after `Send` — the next `SendRow` would overwrite a slice the router or a shard is reading.
- **C2 · blocker.** What breaks: built as written (pooled row, `fill` in place, 0 allocs) it is a data race between the caller's next `SendRow` and the engine; built safely (a fresh schema-sized slice per call) the `allocgate` budget "0 above `runtime.Send`'s" is red — measured 2 allocs (400 B) against 0. Evidence: runtime.go:507-508, 545; the table above (pooled 0 / fresh 2 / Send's own 0); `allocs.log`, `allocs2.log`, `bench.log`, exit 0 each. Fix — replace MUST 11 with: *"**MUST 11:** one allocation per `SendRow` beyond `runtime.Send`'s own: the row's `[]event.Value`, sized by the schema, which `runtime.Send` takes (runtime.go:507). The writer header (`row`) is pooled; its `vals` is fresh per call and never returned to the pool. `allocgate`'s `AllocsPerRun` on `SendRow`: exactly 1 above `runtime.Send` with shared values."* And replace the §4 sentence "reused per `InputHandler` … takes a row from a `sync.Pool`" with: *"`SendRow` is safe for concurrent use: each call takes a writer header from a `sync.Pool`, gives it a fresh `[]event.Value`, and after `fill` returns disarms it (`vals = nil`), so a `RowWriter` kept by a closure and used later panics with `cep: RowWriter used after SendRow returned` instead of writing into an event the engine owns."*
- Keep `batchID` positional: Siddhi's `send(long timestamp, Object[])` is the same shape; `""` for none is one token.

## (3) `OutEvent{…, Canonical []byte}` valid until `Write` returns — vs a copied value

**As the note has it (§5):**
```go
type OutEvent struct {
	Stream    string
	Timestamp int64 // unix ms
	Row       RowReader
	Canonical []byte // valid until Write returns
}
```
and §5.2: *"`Canonical` by `playback.AppendEvent` into a buffer the flusher owns"*.

**Prior art.** Siddhi copies before the callback: `eventBuffer.add(new Event(complexEvent.getOutputData().length).copyFrom(complexEvent))` (StreamCallback.java:97) — the callback owns its `Event[]`. Esper's `EventBean` is immutable and may be retained (from memory). Flink's object reuse is **opt-in** (`ExecutionConfig.enableObjectReuse()`, from memory) because retained references broke sinks when it was on. All three default to "the sink may keep what it got"; the note makes Flink's opt-in hazard the default.

**Measured:** encoding is 250 ns and 112 B for a 4-attribute output; copying it is +1 alloc and +33 ns per output; encoding into the sink's own buffer is 244 ns, 0 allocs. Two more facts: the flusher encodes for every sink whether or not it reads the text (an http webhook renders `{{attr}}` from `Row`; `inmemory` copies anyway), and the note does not state `Row`'s validity at all — it reads `e.Values` through the same flusher-owned event.

**Verdict: take neither the alias nor the copy — move the bytes out of the struct into an `Append` method (the `strconv.AppendInt` / `time.AppendFormat` idiom).** The sink that wants the text appends it into a buffer it owns: 0 allocs, nothing to retain by accident, and sinks that do not want it pay nothing. The cost — k encodes for k text sinks on one stream, 250 ns each — is below the copy's cost for k ≤ 1, which is every example in the corpus.
- **C3 · should-fix.** What breaks: a third-party sink that keeps `e.Canonical` (a batching webhook, a buffered log) gets the next output's bytes under the previous output's name — a silent wrong output, the class ADR-0008's at-least-once promise does not cover. Evidence: §5's comment "valid until Write returns" and §5.2; StreamCallback.java:97 for the prior art; `bench.log`. Fix — replace the struct and the §5.2 sentence with:
```go
// OutEvent is one output. Row reads the engine's event and is valid until Write returns; a sink that keeps a value
// copies it (inmemory does). AppendCanonical appends the canonical JSON (ADR-0003/0005; the log sink prints exactly
// this text) to dst, which the sink owns, and returns the extended slice; it reads Row, so call it inside Write.
type OutEvent struct {
	Stream    string
	Timestamp int64 // unix ms
	Row       RowReader
	enc       func(dst []byte) ([]byte, error) // the facade's AppendEvent over the stream's schema
}
func (e OutEvent) AppendCanonical(dst []byte) ([]byte, error)
```
  §5.2 becomes: *"the facade wraps each `cepio.Sink` as a `runtime.Sink`: `func(e event.Event) error` builds the `OutEvent` (the `RowReader` over `e.Values`; `enc` closing over the schema and `playback.AppendEvent`) and calls `Write(ctx, out)`; nothing is encoded unless the sink calls `AppendCanonical`."* `inmemory.Sink` then keeps `AppendCanonical(nil)`'s result, which is its own.
  If the owner prefers a plain field, the safe field is the copy: +1 alloc, +33 ns per output, measured — still cheaper than a wrong output.

## (4) The batch-close signal as `Emitter.EndBatch` (accepted, counted, not recorded until S8) vs recording now vs "a batch id change implies the close"

**As the note has it (§5, §5.3):**
```go
type Emitter interface {
	Emit(ctx context.Context, batchID string, fill func(RowWriter) error) error
	EndBatch(ctx context.Context, batchID string) error
}
func (h *InputHandler) EndBatch(ctx context.Context, batchID string) error
```
*"In S3.3-1 the engine accepts, counts (`StreamStats.Batches`) and otherwise ignores it … the recording gains its record kind … when S8 first reads it."*

**Prior art.** Siddhi has no id and no close: the batch *is* the array handed to one `InputHandler.send(Event[] events)` (InputHandler.java:86), and `BatchWindowProcessor` groups by the chunk it is handed (BatchWindowProcessor.java:126-182) — so a batch can neither span two calls nor be empty, and two producers cannot interleave. Flink's punctuation is an explicit record in stream order (`Watermark`, emitted by the source; not checkpointed — regenerated on restore). Esper has no source punctuation.

**The id-change alternative** (no new call: the engine closes batch A when the stream's next event carries B ≠ A, and at the drain). It needs no API and no record — the recording already carries each event's batch id on both paths (recorder.go:86-89 for payloads; encode.go:174-176 for typed sends). It fails on the http source ADR-0008 §4 defines: "each POST body = one source batch", and a listener serves POSTs concurrently, so A1 B1 A2 on one stream would close A at B1 and put A2 in a closed batch; it also cannot express an empty batch. **The explicit signal is right;** Siddhi's implicit form only works because its batch is one synchronous call.

**Recording now vs later.** Recording now is one record kind, `{"arrival":…,"batch_end":"<id>","stream":"S"}` at its intake position (it is an intake item, so it has a stamp like a tick), about +0.5. What it buys: a recording made before S8 replays its closes once S8 reads them. What that is worth: nothing — no program before S8 has a batch window, so no detection can depend on a close, and the drain closes every batch. What it costs: a record designed without its consumer (where it sits relative to a tick with the same stamp, whether a close for an id no `Emit` carried is recorded, whether the sandbox's `given` lists carry one) — each a decision S8 would rather make with the window in hand.
**Verdict: agree with the note (Q1: not now)**, with the forward rule written down so S8 does not have to invent it.
- **C4 · later.** Add to §5.3: *"A recording without `batch_end` records replays with every batch closed at the drain; S8's reader treats their absence so. `EndBatch` for an id no `Emit` of that stream carried is accepted and counted, not an error."*

## (5) `apipartition` by `go/parser` + `go/printer` text vs `go/types` vs `apidiff` alone

**As the note has it (§6):** *"parses its non-test files with `go/parser` and prints one line per exported top-level declaration — `package ident kind signature` — the signature as `go/printer` prints the declaration's type … It never type-checks … `go/types` with source importing costs seconds per run."*

**Measured:** 2.19 ms (parser + printer, 51 lines) vs 1.10 s (`go/types`, source importer) over `internal/runtime` — the note's cost claim holds, with a number. `apidiff` alone is not a law file: it reports a *delta* against the merge base, so the owner's review surface (`api/exports.golden`, the card's "by reading") needs the list; the owner already decided both run (decisions 2026-11-07, 2 and 3).

**Two gaps the text mechanism has, both cheap to close syntactically:**
1. **Promoted methods.** The fixture shows the parser text lists `type Runtime struct { inner; secret int }` and never `Hidden`, while the method set of `*Runtime` has `Hidden func() int`. `apidiff` would report `Hidden` as *added* in the PR that adds it, but the law file would never name it — a public method the owner's "by reading" check cannot see. Every facade type in the note is `struct{ /* unexported */ }`, so the first embedded helper (`sync.Mutex` embedded instead of named — `Lock`/`Unlock` become public API) triggers it.
2. **Unexported fields in the text.** `go/printer` prints the whole struct type (`secret int` above); either the golden churns on every private field change or the tool filters — the note does not say which.
- **C5 · should-fix.** What breaks: an exported method reachable through an embedded field is absent from the law the owner reads; the golden churns on private fields. Evidence: `probe.log` (exit 0), the fixture. Fix — add to §6's `tools/apipartition` bullet: *"The tool prints an exported struct's exported fields only, and refuses (red, naming the type) an exported struct or interface with an embedded field, since promoted methods are invisible to the text; a type that needs embedding names the field instead. Unit-tested over the fixture package with both cases."* Keep `go/parser`.

## (6) `Table.Delete(ctx, key func(RowWriter) error)` vs `Delete(key any)` (ADR-0012 §5)

**As the note has it (§3):**
```go
func (t *Table) Upsert(ctx context.Context, fill func(cepio.RowWriter) error) error
func (t *Table) Delete(ctx context.Context, key func(cepio.RowWriter) error) error // key writes the key columns only
func (t *Table) Load(ctx context.Context, fills []func(cepio.RowWriter) error) error // one write, many rows
```
**The engine as built:** keys are composite (`table.New(name, attrs, keyCols []string)`, table.go:60; `checkKey` requires `len(k) == len(t.key)`, table.go:205-207); `Table.Delete(keys [][]event.Value)` and `Upsert(rows [][]event.Value)` are batched, all-or-nothing, one version per call (table.go:114, 134-153); `runtime.TableWrite(ctx, app, tableID, op, rows [][]event.Value)` is one intake item that returns once applied (runtime.go:559-585). Prior art: Siddhi's `TableInputHandler` (SiddhiAppRuntime.java:143) is insert-only; deletes are EPL `delete … on`. Esper tables are EPL `on-merge` (from memory). Neither has a typed programmatic delete to copy.

`Delete(key any)` cannot carry a composite key without a slice-or-struct convention and types it at runtime by reflection; the fill form reuses the one boundary, is composite-capable, and errors at the call. **The note's form wins over the ADR's wording.** What it loses against the engine is batching: deleting or upserting n rows is n intake round trips and n versions where the engine does one.
- **C6 · later.** Fix — replace the two signatures with:
```go
func (t *Table) Upsert(ctx context.Context, fills ...func(cepio.RowWriter) error) error // one write, many rows: one intake item, one version, one recording position
func (t *Table) Delete(ctx context.Context, keys ...func(cepio.RowWriter) error) error  // each key writes the key columns only; no keys: nothing
```
  `Load` keeps its slice form (`Load(ctx)` emptying the table, table.go:155-157, is a footgun as a variadic).

## (7) The lifecycle `inactive → active → draining → removed` with three errors vs Siddhi's `start`/`shutdown` vs a context-bound app

**As the note has it (§2 MUST 3, §3):** `ErrInactive` before `Start`, `ErrStopped`/`ErrDrained` after `Stop`, `ErrUndeployed`/`ErrRemoved` after `Undeploy`, `ErrClosed` after `Close`; *"Activating twice is an error; activating a drained or removed app is an error"*; `Start(ctx) error`, `Stop(ctx) (StopReport, error)`, `Undeploy(ctx) error`.

**Prior art.** Siddhi: `start()` on a running app logs a warning and returns (SiddhiAppRuntimeImpl.java:454-459) — silent idempotence; `send` on a non-running app throws `InterruptedException("… is not running, cannot send event.")` (InputHandler.java:51-57) — one message for every non-running state, in the wrong exception type; `shutdown()` tears down sources then executors with no drain report. Esper 8 (from memory): `EPRuntime.destroy()`, `EPDeploymentService.undeploy(id)`, `EPRuntimeDestroyedException` on a destroyed runtime — one state. A context-bound app (`Run(ctx) error`, the `http.Server`/`oklog/run` idiom) composes with `errgroup` but cannot return `StopReport.DrainedPending` or distinguish "the drain's deadline passed, recording incomplete" (runtime.go:616-622) from a cancelled lifetime.

**Verdict: the note's explicit states, three distinct sentinel errors and a `StopReport` are better than all three** for an engine whose stop is part of the replay contract. Two gaps:
- **C7 · later.** "Activating twice is an error" names no sentinel, so a program cannot branch on it. Fix — add `var ErrStarted = errors.New("cep: the app is already started")` to the `var` line and the sentence *"`Start` on a started app is `ErrStarted`; on a stopped app `ErrStopped`; on an undeployed app `ErrUndeployed`."*
- **C8 · should-fix.** A source's `Open` error has no home: §5.1 starts each `Open` on its own goroutine and `Start` returns before it; an `Open` that fails at once (a port in use, in S5.1) is lost unless something collects it. Siddhi separates `connect` (synchronous, fails `start`) from the pump. Fix without touching `cepio.Source`: *"`Stop` returns every `Open` error that is not the cancelled context's, by stream, in `StopReport`"* with
```go
type StopReport struct {
	DrainedPending int64            // timers left unfired by the barrier (drained_pending)
	SourceErrors   map[string]error // Open's non-nil return per bound stream, ctx.Canceled excluded
}
```
  and `Stats.Streams[name].SourceErr string` so a running app shows it before `Stop`.

---

## The changes I would make to the note, ranked

1. **C2 (blocker)** — MUST 11 and §4's pool sentence: the floor is one allocation (the schema-sized `[]event.Value` the engine takes, runtime.go:507); the writer header is pooled, the slice is not; the writer is disarmed after `fill`. Exact text above.
2. **C3 (should-fix)** — `OutEvent`: `Canonical []byte` → `AppendCanonical(dst []byte) ([]byte, error)`; state `Row`'s validity; §5.2 encodes nothing unprompted. Exact struct above; measured 244 ns / 0 allocs vs 250 / 0 aliased vs 282 / 1 copied.
3. **C5 (should-fix)** — `apipartition`: exported fields only; refuse embedded fields in exported types. Exact sentence above; the fixture proves the gap.
4. **C1 + C8 (should-fix)** — define a repeated `WithSource`/`WithParam`/`WithRecording` as a `*DeployError`; give `Open`'s error a home in `StopReport.SourceErrors`.
5. **C6 + C7 + C4 (later)** — variadic `Upsert`/`Delete`; `ErrStarted`; the forward rule for recordings without `batch_end`.

## Only the owner can decide

- **Q1 (record the close now?)** — recommendation: **not now**, as the note says, with C4's forward sentence written in. Plain words: nothing before S8 can tell the difference, and S8 will know where the record must sit.
- **Q2 (an annotation naming a connector not built yet)** — recommendation: **a deploy error naming the sprint**, as the note says. Plain words: a program that says `@source(type='http')` and quietly runs with no source would print nothing and look healthy; an error now, a working source in S5.1. The alternative (ignore the annotation, feed through `Input`) makes the same program mean two things in two sprints.
- **Q3 (`apidiff` red on every incompatible change, or only when the golden did not move?)** — recommendation: **only when the golden did not move**, as the note says. Plain words: before the first release every signature change is deliberate; the golden's diff is already the line the owner reads; a second red adds a step and no information. Tighten at S12 as planned.
- **New, from C3:** replacing the `Canonical` field with a method changes ADR-0008's implementation note ("OutEvent carries the canonical encoding") in form, not in substance — the owner's call whether a method "carries" it. Recommendation: the method.

## What held

- The Deploy pipeline's order (parse → validate → plan → bind → `runtime.Add`) and `WithParam` taking the rule language's own literal text through `parser.ParseLiteral` — the LLM-friendliest of the shapes (the same text in a rule and in Go).
- The fill-function over the map, the positional slice and the engine-returned row: measured 2 (1 pooled header) vs 4 vs 1-but-positional vs 1-but-caller-holds-the-row; the typed struct is S6.1's layer over it at the same floor.
- The explicit `EndBatch` over Siddhi's one-call-one-batch and over id-change inference, which ADR-0008 §4's concurrent POSTs would break.
- `go/parser` for the list: 2 ms against 1.1 s, with the two syntactic rules added.
- The explicit lifecycle with distinct sentinels over Siddhi's warn-and-return and over a context-bound `Run`.
- Sink flushers owned by `internal/runtime` and `cepio.Sink.Write` called from that goroutine only — the same deadlock-freedom argument as ADR-0006 §5; nothing in prior art does better (Siddhi's synchronous junction delivery is the rejected one).
- `Table.Delete` as a key fill over `key any`: composite keys (table.go:60, 205) decide it.

**Time box:** reached at the end of (7); nothing left unwritten that I measured.
