# S3.3-2 — design note: the expect sandbox, the conformance runner, the baselines and the clock freeze (v2, build-ready)

**Status:** v2, after the blind panel and the testability review; the owner's decisions of 2026-11-09 applied. **Written so that a coding agent builds from this document alone:** every file is named, every signature written out, every rule a MUST with its test. Where this note and the code disagree, the code wins and the note is corrected (ground rule 21). v1 is in git (0a3114c); the reviews are [breaker](../reviews/s3-3-2-design-breaker.md), [challenger](../reviews/s3-3-2-design-challenger.md), [architecture](../reviews/s3-3-2-design-architecture.md), [testability](../reviews/s3-3-2-design-qa.md); §14 maps every finding to its section and disposition.

**What it covers:** items 3, 4, 6 and 7 of the [S3.3 card](sprint-plan.md#s33--the-deploy-pipeline-and-the-conformance-runner--clock-freeze), its forwards on the sandbox (M1, M2, L2, the unknown-attribute fault), exits 1, 2, 6, 7, 8, 9, and known issues S3.3-1-1 and S3.3-1-2 ([record](s3-3-2-record.md)). Not here: patterns (S4.1), text operators (S7), windows (S8, S9), `gate/law.json`, the LAW Δ block and the current-sprint marker (S1.1c).

**The owner's decisions this note carries out:**
- [S3.3-2's plan check](decisions.md#owner-decisions-2026-11-08--s33-2s-plan-check) (2026-11-08): 1, a block runs with the deployment's param values and a different result refuses the deploy (E-EXP-001), no warning; 2, a mistake inside a block is its own static code at its position, found before any block runs; 3, the full loop with the panel.
- [S3.3-2's design, after the panel](decisions.md#owner-decisions-2026-11-09--s33-2s-design-after-the-panel) (2026-11-09): 1, "not built yet" is `errors.ErrUnsupported`; 2, E-EXP-002 refuses every kind in §3's table; 3, E-EXP-001 points at the expected record that did not come out, or at `then` for an unexpected output; 4, a baseline recognises a block by its content; 5, after a SPEC revision only the named blocks' values are recorded; 6, the not-built expiry is S1.1c's; 7, `optional` on a time attribute is refused (E-TIME-003, SPEC rev 10); 8, one sprint, about 9.75 points.
- Earlier, binding: [S3.3's plan check](decisions.md#owner-decisions-2026-11-07--s33s-plan-check), 4 (lists interleave by time, each in its written order, ties to the list written first); S3.2b-1's M2 (a block passes only when outputs match the then-list in number as well as content) and L2 (`Deploy` never mutates or reuses the checked model after `plan.Build`); [S3.3-1's design](decisions.md#owner-decisions-2026-11-07--s33-1s-design-after-the-panel), 5 (the sandbox opens no connector; an unbuilt connector deploys in tests through a Go binding).

**Constraints from the gate that shape the code (read before writing a line):**
- The import law (`tools/importcheck/rules.json`): `conformance` imports only `""` (package `cep`), `rule`, `io`, `io/*` — the runner uses the public API only. `cep` never imports `internal/model` (S3.3-1 MUST 12). `internal/playback` may import `internal/plan`, `internal/runtime`, `internal/validate`, `internal/model`; its tests also `internal/parser`. `internal/plan` imports `internal/model` and `internal/event`.
- `archcheck`: a goroutine, a channel, `sync` or `sync/atomic` only in `internal/runtime`, `io/*`, `cmd` (and the named exceptions). The sandbox runs on the caller's goroutine through `runtime.Sync`, so `internal/playback` needs none; the `Close` gate (§9.1) lives in `internal/runtime`.
- The gate's go-test rows (`gate/runner/rows/gotest.py`, `unit.py`) judge a **skipped** test as a failure: no test in this sprint calls `t.Skip`.
- The pre-commit hook refuses unformatted Go; a test file written through a heredoc is checked with `gofmt -l` and for `\uXXXX` escapes.

## 1. Where it lands

| Path | What | Points |
|---|---|---|
| `internal/model/model.go`; `internal/parser/grammar.go`, `lower.go` | positions on expect blocks (§2) | 0.25 |
| `internal/validate/expects.go` (new), `validate.go`, `graph.go`, `catalog.go`, `static_test.go`; `testdata/e-exp-002-*.cep`, `testdata/e-time-003-optional-time.cep`; `docs/language/errors.md`, `SPEC.md` (rev 10) | E-EXP-002 and E-TIME-003's second reason (§3) | 1 |
| `internal/plan/expect.go` (new), `plan.go` | the plan carries the blocks as values (§4) | 0.25 |
| `internal/playback/expect.go` (new), `internal/runtime/sync.go` | the sandbox `RunExpects`, the alignment, the counts; `NewSync`'s queues (§5) | 2 |
| `deploy.go` | the expect stage; `errors.ErrUnsupported`; ctx before bind (§6) | 0.25 |
| `conformance/doc.go`, `conformance/corpus_test.go`, `conformance/factory_test.go`; `gate/registry.json` row `conformance`; `gate/proofs/drivers/test_expect_planted.py` | the runner, its row, the six planted proofs (§7) | 2 |
| `conformance/baseline/README.md`, `conformance/baseline/*.jsonl`; `internal/playback/baseline_test.go`; rows `baseline`, `determinism`, `determinism-macos`; `gate/baseline/__init__.py`, `gate/baseline/regen.py`, `gate/baseline/tests/test_regen.py`; `Makefile` target `regen-baseline`; `gate/proofs/drivers/test_regen_baseline.py` | Tier 2 (§8) | 2.25 |
| `internal/runtime/lifecycle.go`, `runtime.go` | S3.3-1-2: the in-flight gate (§9.1) | 0.75 |
| `tools/apipartition/main.go`, its tests; `api/exports.golden`, `api/clock.golden` | S3.3-1-1: `<Type>.<Method>` (§9.2) | 0.25 |
| the record; `docs/architecture/adr/ADR-0013-*.md` | the clock freeze declared at G5 (§10) | 0.25 |

About 9.75 points (owner, 2026-11-09, decision 8). One sprint, one PR, at most two agents at once.

## 2. Positions on expect blocks (`internal/model`, `internal/parser`)

**The model** (`internal/model/model.go`; every new field is of type `Pos`, so `MarshalCanonical` skips it and **no golden regenerates** — `model.go:447`, the precedent of S3.2b-2's `model.Attr.Pos`):

```go
// Expect is one `expect` block: its given sections in written order and its then section.
type Expect struct {
	Givens []Given
	Then   Then
	Pos    Pos // `expect`
}

// Given is `given [table] Name [records]`.
type Given struct {
	Table   bool
	Name    string
	Records []Record
	Pos     Pos // `given`
	NamePos Pos // the stream or table name
}

// Then is `then Name [records]`; no records asserts that nothing fires.
type Then struct {
	Name    string
	Records []Record
	Pos     Pos // `then`
	NamePos Pos // the stream name
}

// Record is `{field: value, …}` in written order.
type Record struct {
	Fields []Field
	Pos    Pos // `{`
}

// Field is `name: literal-or-null`.
type Field struct {
	Name     string
	Value    Literal
	Pos      Pos // the field's name
	ValuePos Pos // the value's first token: `-`, the literal, or `null`
}
```

**The parser** (`internal/parser/grammar.go`): `GivenSec` gains `Pos Position` and its `Name string` becomes `Name *NameTok`; `ThenSec` gains `Pos Position` and `Name *NameTok`; `Record` gains `Pos Position`; `Field` gains `Pos Position` and its two value fields become one `Value *FieldValue`:

```go
type GivenSec struct {
	Pos     Position
	Table   bool      `parser:"'given' @'table'?"`
	Name    *NameTok  `parser:"@@"`
	Records []*Record `parser:"'[' (@@ (',' @@)*)? ']'"`
}

type ThenSec struct {
	Pos     Position
	Name    *NameTok  `parser:"'then' @@"`
	Records []*Record `parser:"'[' (@@ (',' @@)*)? ']'"`
}

type Record struct {
	Pos    Position
	Fields []*Field `parser:"'{' @@ (',' @@)* '}'"`
}

type Field struct {
	Pos   Position
	Name  string      `parser:"@(Ident|Backticked) ':'"`
	Value *FieldValue `parser:"@@"`
}

type FieldValue struct {
	Pos  Position
	Null bool           `parser:"( @'null'"`
	Lit  *SignedLiteral `parser:"| @@ )"`
}
```

**The lowering** (`internal/parser/lower.go`): `expect(e *ExpectBlock, at Position)` is called with the enclosing `Extra.Pos` (the `expect` keyword) and fills every position above with `pos(…)`; `records` fills `Record.Pos`, `Field.Pos` and `Field.ValuePos` (`f.Value.Pos`). The grammar files (`grammar.ebnf`, `grammar.lark`) do not change; the parser goldens do not change.

**Test:** `TestExpectPositions` (`internal/parser/expect_pos_test.go`) — example 07's first block and a hand-written block with a `null` and a negative number: every position above by line and column. Red today (zero positions).

## 3. The static checks (`internal/validate`)

### 3.1 E-EXP-002 — a mistake inside an expect block

**Catalogue** (`catalog.go`): `"E-EXP-002": {Title: "Mistake inside an `expect` block", Fix: "name a stream or table the rule reads or writes, and give each field a value of its declared type."}`.

**`errors.md`**, after E-EXP-001:

> ### E-EXP-002 Mistake inside an `expect` block **[static]**
> Rule: SPEC §6.8 (rev 10) — a block names only streams and tables its rule reads or writes, each `given` stream once; every field is declared, written once and has its declared type; a required field — and a stream's time attribute, always — has a value; the engine-owned `text_invisibles` is never given. Checked before any block runs.
> Message names the block, the stream or table, the field and the expected type. Fix: name a stream or table the rule reads or writes, and give each field a value of its declared type.

**The check:** a new phase `c.expects()` in `internal/validate/expects.go`, run last in `checker.run` (after `c.expressions()`), over every rule that is neither `poisoned` nor `noTarget`. For each block `b` (ordinal `n`, 1-based within the rule), each section, each record, each field, in written order — every finding reported, at its own position:

| # | Mistake | Position | Message (after `expect block <n> of rule <R>: `) |
|---|---|---|---|
| X1 | `given` names no declared stream; `given table` no declared table; `then` no declared stream | `NamePos` | `unknown stream <S>` / `unknown table <T>`, with `; did you mean <S'>?` from `nearest` |
| X7 | `given` names a declared stream the rule does not read; `given table` a declared table the rule does not read; `then` a stream other than the rule's `insert into` target | `NamePos` | `rule <R> does not read <S>` / `rule <R> does not read table <T>` / `rule <R> inserts into <T>, not <S>` |
| X8 | `given` names a stream whose time clause is `time arrival` | `NamePos` | `<S> is stamped on arrival; a block cannot give it events` |
| X9 | `given` names a stream already given in this block | the second `NamePos` | `<S> is given twice; give each stream one list` |
| X2 | a field its stream or table does not declare | the field's `Pos` | `<S> has no attribute <a>` with `; did you mean <a'>?` |
| X10 | a `given` record names `text_invisibles` | the field's `Pos` | `<S>.text_invisibles is computed by the engine; a block cannot give it` |
| X6 | a field named twice in one record | the second field's `Pos` | `field <a> is written twice in one record` |
| X3 | a value that does not `fits` its field's type (`resolve.go:56`; an int literal fits a float) | `ValuePos` | `<S>.<a> is <type>; the block gives a <kind>` (`kindName`) |
| X4 | in a `given` or `given table` record: `null`, written or by omission, for the stream's **time attribute (always)** or for any other non-optional attribute or column — except `text_invisibles`, which the engine fills | `ValuePos` when written; the record's `Pos` when omitted | `<S>.<a> is required; the block gives it null` / `…; the block leaves it out` |
| X5 | in a `then` record: `null` for a non-optional attribute of the target stream | `ValuePos` | `<S>.<a> is never null; the block expects null` |

**Definitions.** The rule *reads* stream S when S is its body's source (`Body.Source.Stream`; S4.1 extends this to every pattern step's stream). The rule *reads* table T when one of its filter or select expressions, at any depth, is an `ExprIn` with `Table == T` or an `ExprText` with `Operand.Table == T`. A section that fails X1, X7, X8 or X9 is not checked further. Every finding goes through `c.report("E-EXP-002", …)`; the phase never reports any other code.

Because `expects()` runs inside `Check`, every E-EXP-002 refuses the deploy before `plan.Build`, so before any block runs (owner, 2026-11-08, 2; exit 8).

### 3.2 E-TIME-003's second reason — a time attribute declared `optional` (owner, 2026-11-09, 7)

`graph.go`'s time check (`:113-118`) gains a case after the type check: `case a.Optional:` → `c.report("E-TIME-003", s.Time.AttrPos, "the time attribute %s of %s is declared optional: every event needs a time", a.Name, s.Name)`. The catalogue title becomes `Time attribute is not a timestamp, or is optional` and its fix `declare the attribute `timestamp` without `optional`, or name the stream's timestamp attribute.`; `errors.md`'s E-TIME-003 heading, rule line and fix change to match (docsync (b) pins both ways). Test program `testdata/e-time-003-optional-time.cep`. No corpus file declares an optional time attribute (checked: `grep -rl 'timestamp optional'` finds only a table column in `breakit_s3_3_1_test.go:136` and a non-time stream attribute at `:537`).

### 3.3 E-EXP-001 in the catalogue

E-EXP-001 is raised by `Deploy`'s expect stage, not by `Check`. `Entry` gains `Deploy bool // raised by cep.Deploy's expect stage, not by Check; its tests are the facade's and the conformance runner's`; E-EXP-001 drops `Later: "S3.3"` and sets `Deploy: true`; `TestEveryStaticCodeIsAccountedFor` (`static_test.go:180-205`) gains `case e.Deploy && tested[code]: t.Errorf("%s is raised at deploy, not by Check, but has a test program", code)` and skips a `Deploy` code in the "no test program" case. `errors.md` E-EXP-001's message line becomes: "Message names the rule, the block and the first difference between the outputs and the then-list; it points at the expected record that did not come out, or at `then` for an output the then-list does not have."

### 3.4 SPEC rev 10

`SPEC.md`: the status line's rev 9 → rev 10 (2026-11-09…), Appendix E's change log gains rev 10's line, and §6.8 gains these sentences (each is what the code does; each rule fragment quoted is parsed first — sprint model §10):
- a block's tables are seeded before any event, wherever `given table` is written; each input stream is given at most one list; lists of different streams interleave by time, each in its written order, ties to the list written first;
- a block runs the rule alone, with the deployment's param values, and ends as playback ends (`wm → +∞`);
- a block passes only when the outputs match the then-list in number and order, each then-record matching on the attributes it names; a block fails when the rule panics;
- a mistake inside a block is `E-EXP-002`; a time attribute is never `optional` (`E-TIME-003`, §4.2).

**Tests:** the validator's programs (§11, T3–T15, T53); `TestExpectMistakeMessages` (`internal/validate/expects_test.go`, a table over every row: the message's names); the facade's "E-EXP-002 alone" (lane 1).

## 4. The plan carries the blocks (`internal/plan/expect.go`)

`Deploy` keeps no model (L2), and the sandbox lives in `internal/playback`, which reads plans; so `plan.Build` converts each rule's blocks into values once, from the checked model, whose params are resolved — decision 1 of the plan check holds by construction: the sandbox runs the plan `Deploy` built with the overrides, and a block's own fields are literals or `null` (`grammar.go:228-232`, never `${param}`).

```go
// Expect is one expect block of a rule, as values (SPEC §6.8). Build fills it from a checked model: every name is
// declared and every value has its field's type (E-EXP-002).
type Expect struct {
	Rule      int          // the owning rule's index in App.Rules
	Block     int          // 1-based within the rule
	Line, Col int          // `expect`
	ThenLine  int          // `then`
	ThenCol   int
	Tables    []GivenTable // in written order; seeded before any event
	Lists     []GivenList  // in written order; at most one per stream (E-EXP-002 X9)
	Then      []ThenRecord // the rule's outputs, in order; empty: nothing fires
}

// GivenTable is `given table T [rows]`: each row in the table's attribute order, an omitted optional column null.
type GivenTable struct {
	Table int
	Rows  [][]event.Value
}

// GivenList is `given S [records]`, each event's values in the stream's attribute order; an omitted optional
// attribute is null; the text_invisibles slot, if declared, is false (the ingest step computes it).
type GivenList struct {
	Stream int
	Events []GivenEvent
}

// GivenEvent is one given event; Ts is its time attribute's value (never null: E-EXP-002 X4, E-TIME-003).
type GivenEvent struct {
	Ts     int64
	Values []event.Value
}

// ThenRecord is one then-record: the attributes it names, in written order, and their values.
type ThenRecord struct {
	Line, Col int   // `{`
	Attrs     []int // indexes into the target stream's schema
	Values    []event.Value
}
```

`Rule` gains `Target int` (the stream id it inserts into; the sandbox prints a then-record by its schema). `App` gains `Expects []Expect` — on the app, **not** on `Rule`, so `runtime.FromPlan` (`sync.go:157`) never carries the blocks into a running app (A10). `Build` appends each rule's blocks in declaration order, then written order. A literal becomes a value by its field's type: `LitString` → `TString`; `LitInt` → `TInt`, or `TFloat` when the field is a float (as `validate.as`); `LitFloat`; `LitBool`; `LitDuration` → `D`; `LitTimestamp` → `T`; `LitNull` and an omitted field → `event.Value{Kind: t, Null: true}` (the decoder's null, `internal/event/decode.go:138`); the `text_invisibles` slot → `event.Value{Kind: event.TBool}`. `Build` never fails on a block: a contradiction (an unknown name, a type `fits` refuses) returns `fmt.Errorf("plan: rule %s, expect block %d: %s (Build is given only a checked model)", …)`, as `rule()`'s own checks do.

**Tests:** `TestBuildCarriesExpects` (`internal/plan/expect_test.go`: example 07's six blocks and 09's four — tables, lists, then attributes by index, the int-to-float widening of a then value, typed nulls, `Ts`, every position) and `TestFromPlanCarriesNoBlocks` (a reflect walk of `runtime.App` from `FromPlan(p, …)` reaches no `plan.Expect`).

## 5. The sandbox (`internal/playback/expect.go`)

### 5.1 Signatures

```go
// Counts are a block's drops, read from the sandbox's Stats after the end (SEMANTICS §10).
type Counts struct {
	Late      int64 // given events dropped as stream-late
	Errored   int64 // events an expression error dropped
	KeyErrors int64 // events a key error dropped
	Panics    int64 // the rule's recovered panics
}

// BlockResult is one expect block's run.
type BlockResult struct {
	Rule       string
	Block      int           // 1-based within the rule
	Outputs    []event.Event // the rule's outputs, in emission order
	Counts     Counts
	Divergence string // "" when the block passed; otherwise §5.4's text
	Line, Col  int    // where E-EXP-001 points (§5.4); 0, 0 when the block passed
}

// RunExpects runs every block of p, in p.Expects' order, each in a fresh sandbox: the owning rule alone, its own
// empty tables, no sinks, no recorder, no connectors. It runs on the caller's goroutine and is deterministic. The
// error is an engine fault (a sandbox that cannot be built, a table that refuses a row, a panic outside the rule's
// recovered body), never a block that failed.
func RunExpects(p *plan.App) ([]BlockResult, error)
```

### 5.2 One block, step by step

1. `sub := &plan.App{Streams: p.Streams, Tables: p.Tables, Rules: []plan.Rule{p.Rules[x.Rule]}}` — the owning rule alone (SPEC §6.8 checks *the rule's* outputs); the slices are shared read-only (`FromPlan` keeps what it needs; each body compiles its own trees through `NewBody`).
2. `d, err := NewPlanDriver(sub, func(e event.Event) { outs = append(outs, e) })`; an error is returned as `fmt.Errorf("playback: rule %s, expect block %d: %w", …)`. Fixture mode (no header record), so an event's arrival is its `Ts` (`driver.go:99-111`).
3. Every `GivenTable`, in written order, every row in order: `d.sync.TableWrite(t.Table, runtime.TablePut, [][]event.Value{slices.Clone(row)})`; an error is returned in step 2's form — never through `Driver.Step`'s panicking table path (`driver.go:75-77`).
4. The events, merged (owner, 2026-11-07, 4): keep an index per list; while any list has events left, take the list whose next event has the smallest `Ts`, ties to the list written first, and `d.Step(Record{Kind: RecEvent, Event: event.Event{StreamID: l.Stream, Ts: ev.Ts, Values: slices.Clone(ev.Values)}})` — a fresh slice per event, since the clock and the operators keep events (ADR-0010 §4). Each list keeps its written order: a later event with a smaller `Ts` waits its turn (example 07's arrival-order block). The number of lists is at most the rule's inputs (X9), so a linear scan of the heads is enough.
5. `d.End()` — the playback end of SEMANTICS §2 (line 47): every contribution to `+∞`, every timer fires, every key purges (`driver.go:112-117`, `sync.go:118-125`). Not `Stop`'s one-past-the-max: example 01's block 1 (S4.1) expects an absence alert at 10:05 after its last event at 10:03.
6. `st := d.Stats()`: `Counts{Late: Σ st.Streams[0][s].Late over the given streams, Errored: st.Rules[0][0].Errored, KeyErrors: st.Rules[0][0].KeyErrors, Panics: st.Rules[0][0].Panics}`.
7. Compare (§5.3, §5.4).

Steps 2–7 run inside a function with `defer func() { if v := recover(); v != nil { err = fmt.Errorf("playback: rule %s, expect block %d: a panic: %v", …) } }()`: a panic the shard did not recover becomes `RunExpects`' error, never the caller's panic (the runtime's precedent, `runtime.go:1318-1326`).

**`runtime.NewSync`** (`sync.go:30`) builds its runtime with `Config{Shards: 1, Mailbox: 1, Intake: 1, SinkQueue: 1, RecordQueue: 1, Now: …}`: a `Sync` delivers in place and never queues (`sync.go:36-40`), so the 1,024- and 65,536-slot defaults (`runtime.go:352-370`) were allocated for nothing — measured 483 KB → 17 KB and 52 µs → 8 µs per block (the challenger's C5). Every replay benefits.

### 5.3 The comparison

`o` = the outputs (n), `t` = the then-records (m). Output `i` **matches** then-record `j` when, for every attribute `t[j]` names, the values are equal. Two values are equal when both are null, or neither is and they have the same kind and the same value: `Str`, `I`, `B`, `T`, `D` by `==`; `F` by `==` **and** the same `math.Signbit` (−0.0 ≠ +0.0, as the canonical encoding writes them differently — `encode.go:44`). A then-record holds no object (X3).

The block **passes** iff `Counts.Panics == 0`, `n == m` and `o[i]` matches `t[i]` for every `i` (M2: number, order and the named attributes).

### 5.4 The first divergence: its text and its position

When the block does not pass, the first applicable case gives the `Divergence` and the position (owner, 2026-11-09, 3):

1. **A panic** (`Counts.Panics > 0`): `the rule panicked <k> time(s)` — at `expect` (`Line, Col`).
2. **Another order** — `n == m`, and giving each then-record in order the first unused output it matches gives every then-record an output, but not output `j` to then-record `j` for some `j`: let `j` be the first such then-record and `k` its output: `the outputs are in another order: then-record <j> matches output <k>` — at then-record `j`'s `{`.
3. **The alignment** — otherwise, align by a longest common subsequence. `L[i][j]` is the LCS length of `o[i:]` and `t[j:]` under *matches*. Walk from `(0, 0)`: when `o[i]` matches `t[j]` and `L[i][j] == 1 + L[i+1][j+1]`, pair them (`i++`, `j++`); else, when `j < m` and (`i == n` or `L[i][j+1] >= L[i+1][j]`), then-record `j` is the first edit; else output `i` is. The first edit gives:
   - then-record `j` unmatched, `i < n`: `then-record <j> has no output: expected {<a>: <v>, …}; the nearest output is <i>: <a> is <actual>, expected <expected>` — `<a>` the first attribute, in the then-record's written order, whose value differs from output `i`'s — at then-record `j`'s `{`;
   - then-record `j` unmatched, `i == n`: `then-record <j> has no output: expected {…}; the rule emitted <n>` — at then-record `j`'s `{`;
   - output `i` unmatched: `output <i> was not expected: {<every attribute of the output>}; the then-list has <m>` (or `; the then-list is empty`) — at `then`.
4. **The reason** — when any of `Late`, `Errored`, `KeyErrors` is non-zero, the text ends with ` (<k> given event(s) dropped as stream-late)`, ` (<k> dropped by an expression error)`, ` (<k> dropped by a key error)`, in that order, each only when non-zero.

Indexes in texts are 1-based. Values are printed in canonical form (`AppendValue`; a timestamp as its RFC 3339 text; a value `AppendValue` refuses, a non-finite float, with `%v`); a record's attributes in schema order for an output, written order for a then-record. Exit 2's six cases each read as themselves: wrong value (case 3, `<a> is …`), missing output (case 3, `the rule emitted`), extra output (case 3, `was not expected`), wrong order (case 2), an output under an empty `then` (case 3, `the then-list is empty`), wrong timestamp (case 3, `ts is …, expected …`). The rate-limit boundaries the corpus tests read as extra outputs, not as wrong values (the challenger's measurement on 10 without its rate limit: blocks 3 and 5 → `output 3 was not expected`, `output 2 was not expected`).

### 5.5 MUSTs (each with its test, `internal/playback/expect_test.go` unless named)

1. A fresh sandbox per block: nothing a block does is visible to the next. — `TestBlocksDoNotShareState`.
2. The owning rule alone. — `TestOtherRulesDoNotRun` (two rules on one stream; the second's filter divides by zero: `Counts.Errored == 0` and one rule in the sandbox's `Stats`).
3. Tables before events. — lane 1's facade test; `TestTablesSeedFirst`.
4. Lists merged by time, written order kept, ties to the first list. — `TestListsInterleaveByTime` (a synthetic two-input `plan.App`, three ties); `TestListOrderIsArrivalOrder` (07's block 6).
5. The end at `+∞`. — `TestBlockEndsAtPlaybackEnd` (the `End` step's advances reach `math.MaxInt64`).
6. Nothing live: `RunExpects` takes only a plan. — the facade's tests (§6).
7. Deterministic. — `TestRunExpectsIsDeterministic` (07, 09, 10 twice; `reflect.DeepEqual`).
8. A panic fails the block; a fault is an error, never a panic. — `TestPanicFailsADefeatPath` (a hand-built `plan.App` whose body panics on its event; then-list empty → `the rule panicked 1 time(s)`); `TestSandboxFaultIsAnError` (a hand-built plan whose `Expects` holds a null key row → an error, no panic).
9. The divergence texts and positions of §5.4. — `TestDivergenceTexts` (the six cases, each its text and `Line, Col`); `TestAlignedReportOnRateLimitTraps`; `TestMissingOutputNamesTheDrop` (10's block 4 expecting one alert → `(1 dropped by a key error)`; a stream-late given event; an overflow); `TestNegativeZeroIsNotZero`.
10. Light: `TestRunExpectsBytes` — `testing.Benchmark` over `RunExpects` on 07's plan, `AllocedBytesPerOp() < 200_000` (red at today's defaults, about 2.9 MB).

## 6. The facade: the expect stage in `Deploy` (`deploy.go`)

`Deploy` (design s3-3-1 §3.2) becomes, in order:
1. `Closed()` → `ErrClosed`; options; `parser.Parse`; the overrides; `validate.Check` (E-EXP-002 and E-TIME-003 here) — as today.
2. `p, err := plan.Build(checked)`; when `errors.Is(err, plan.ErrNotBuilt)`: `return nil, fmt.Errorf("cep: deploy %s: %w: %w", name, errors.ErrUnsupported, err)` (owner, 2026-11-09, 1); any other error as today.
3. **`results, err := playback.RunExpects(p)`**; an error → `fmt.Errorf("cep: deploy %s: the expect sandbox: %w", name, err)`. Every result with a `Divergence` becomes `Diagnostic{Code: "E-EXP-001", Line: r.Line, Col: r.Col, Message: fmt.Sprintf("rule %s, expect block %d: %s", r.Rule, r.Block, r.Divergence), Fix: "correct the rule or the example."}`, in result order; any → `&DeployError{Name: name, Diagnostics: …}`, no app registered.
4. `if err := ctx.Err(); err != nil { return nil, err }` — a cancelled deploy calls no connector factory (A9).
5. `bind`, `FromPlan`, the recording, the sinks' context, `r.rt.Add` — as today.

The stage sits **before `bind`**, so a refused deploy has called no factory and constructed no connector, and nothing it does reaches a sink, a recording or a live table. `Deploy`'s doc comment says: "a construct this release cannot run yet is refused with an error wrapping `errors.ErrUnsupported`; a failing expect block with E-EXP-001."

**Tests** (module root): QA lane 1 (exits 1, 2, 8, 9; nothing live); `TestRefusedDeployCallsNoFactory` (a counting `ConnectorFactory`, an annotated stream, a failing block → zero calls); `TestCancelledDeployCallsNoFactory`; `TestNotBuiltIsUnsupported` (example 01 → `errors.Is(err, errors.ErrUnsupported)`; example 07 with a typo → not).

## 7. The conformance runner (`conformance/`)

`conformance/doc.go`: `// Package conformance runs the language corpus through the public API: every example file deploys, its expect blocks passing, or is refused as not built yet.` and `package conformance`.

`conformance/corpus_test.go` (package `conformance_test`):

```go
const corpus = "../docs/language/examples"

// wantFiles is every corpus file: the ten examples and the two torture files. A corpus file added or removed moves
// it (and the differential swap proof's pinned count).
const wantFiles = 12

// notBuilt is every corpus file this release cannot deploy: the construct its error names and the last sprint the
// plan's corpus map gives it (sprint-plan.md, "Corpus examples → the sprint that turns each green"). Turning red
// once that sprint arrives is S1.1c's, with the current-sprint marker (owner, 2026-11-09, 6).
var notBuilt = map[string]struct{ construct, sprint string }{
	"01-agent-kill-switch.cep": {"a pattern body", "S4.1"},
	"08-session-flow.cep":      {"a pattern body", "S4.1"},
	"05-text-operators.cep":    {"…", "S7"},
	"06-once-and-group.cep":    {"…", "S7"},
	"03-sort-leaderboard.cep":  {"…", "S8"},
	"02-cron-digest.cep":       {"…", "S9.1"},
	"04-delay-and-batch.cep":   {"…", "S9.2"},
	"torture/comments.cep":     {"…", "S4.1"},
	"torture/lexical.cep":      {"…", "S9.2"},
}

func TestCorpus(t *testing.T)
```

Each `"…"` is filled at build step 8 with the text before `: plan: this body kind is not built yet` (or the `expr: … is not executable yet` construct) in that file's `Deploy` error, read by running it once. `TestCorpus` reads every `*.cep` under `corpus` and `corpus/torture`, sorted, and fails unless it read exactly `wantFiles`. One sub-test per file (name = the path under `corpus`): `cep.New(cep.Config{})`; `Deploy(ctx, file, src, cep.WithConnectors(corpusFactory{}))`; `Close`. A listed file must fail with `errors.Is(err, errors.ErrUnsupported)` **and** `strings.Contains(err.Error(), construct)` — another error, or another construct, is `t.Errorf` (move the entry); a listed file that deploys is `t.Errorf` (remove the entry); the sub-test passes with `t.Logf("not built until %s (%s)", sprint, construct)`. An unlisted file must deploy — every block passing is exit 1; a `*cep.DeployError` is `t.Errorf` with each diagnostic on its own line (`E-EXP-001 19:20 rule HighRisk, expect block 2: output 2 was not expected: {…}; the then-list has 1`).

`conformance/factory_test.go`: `corpusFactory` implements `cep.ConnectorFactory`, returning an `io/inmemory` source for every `@source` and sink for every `@sink`, whatever its type (S3.3-1's design decision 5: an unbuilt connector deploys in tests through a Go binding), so a file refused for a connector never reads as not built. `TestCorpusFactoryBindsEveryAnnotation`: a plain-rule program with an `@source(type = 'http')` and an `@sink(type = 'http')` deploys through it.

**The row** `conformance` (`gate/registry.json`): `{"id": "conformance", "job": "test", "tiers": ["T2", "T3", "T4"], "blocking": true, "timeout_s": 120, "needs": [], "scope": ["prod", "test"], "expected_scope": ["go", "./conformance"], "min_wants": 1, "lands_in": "S3.3-2", "exclusive": false, "command": ["python3.15", "-I", "-B", "-X", "utf8", "gate/runner/rows/gotest.py", "-run", "^TestCorpus$", "./conformance"]}`, then `make ci-workflow` (the `ci-fresh` row). `unit` runs it too; the row exists so a planted proof names it.

**The proofs (exit 2)** — `gate/proofs/drivers/test_expect_planted.py`, the `support.DriverTest` shape of `test_semantics_red.py`: the world copies the gate, the module (`go.mod`, `go.sum`, every package `conformance` imports, transitively) and `docs/language/examples/`. `test_the_clean_corpus_passes` (the row PASS, `covered 13`: `gotest.py` counts `TestCorpus` and its 12 sub-tests); six plants, each an edit of example 09 or 07 in the world (one line each, but the wrong order spans 07's two then lines 35–36: no single line can reorder), the anchor's count asserted 1, the row red, and the row's `test.json` holding `E-EXP-001` with §5.4's text for that case: `test_wrong_value`, `test_missing_output`, `test_extra_output`, `test_wrong_order`, `test_output_under_empty_then`, `test_wrong_timestamp`; and `test_empty_corpus_is_red` (the examples deleted in the world: red, `wantFiles`). `proofs` `min_wants` 19 → 27.

## 8. Tier 2: the baselines and `gate/regen-baseline`

**What is locked:** the full outputs and the drop counts of every expect block of every built corpus file — today 07 (6 blocks), 09 (4), 10 (5): 15. The blocks assert what their then-records name; the baseline locks everything else (an unasserted attribute, a `ts` a then-record omits, why a defeat path is silent).

### 8.1 The files

`conformance/baseline/<corpus file>.jsonl`, one per built file. Line 1, the header: `{"file":"07-consent-gate.cep","format":1,"source_sha256":"<hex SHA-256 of the corpus file's bytes>","spec_rev":10}`. Then one line per block, in `RunExpects` order: `{"block":<ordinal>,"counts":{"errored":0,"key_errors":0,"late":0,"panics":0},"key":"<k>","outputs":[<each output as playback.AppendEvent writes it>],"rule":"<R>"}` — keys sorted, no spaces, one `\n` per line.

**The key** (owner, 2026-11-09, 4: by content): `<k>` is the hex SHA-256 of the canonical JSON (`model.App.MarshalCanonical`) of a `model.App` holding the file's pragma, params, streams and tables, and the owning rule with its `Expects` replaced by this one block — so it changes when the block, its rule or a declaration the file makes changes, and never when the engine changes. Two blocks of one rule with equal keys take `#2`, `#3`, … in written order. The ordinal is for display.

`conformance/baseline/README.md` holds the class table of §8.3 in prose; the tool enforces it. *(The baseline files are law paths in ADR-0015 §1.3's sense; until S1.1c's LAW Δ block, the record lists every baseline change by hand.)*

### 8.2 The check — `internal/playback/baseline_test.go`

`TestBaseline`: for each corpus file not refused as not built (the same 12 files, sorted; a file whose `plan.Build` returns `plan.ErrNotBuilt` is skipped by the loop, not by `t.Skip`): `parser.Parse` (a test import), `validate.Check` with no overrides, `plan.Build`, `RunExpects`, the keys from the parsed model, the encoding of §8.1 — and compare with `conformance/baseline/<file>.jsonl` line by line, the header's `source_sha256` included and its `spec_rev` excluded. A difference is `t.Errorf` naming the file, the rule, the ordinal and the key, with both lines; a built file without a baseline file, or a baseline file without a built corpus file, is red. It also fails unless it ran exactly `wantBlocks = 15` blocks (S6: the public API cannot count blocks) — in compare mode only: a fresh run for the regen tool may hold an added block, which is the tool's to classify. With `CEP_BASELINE_WRITE=<dir>` set, it writes the files into `<dir>` instead, `spec_rev` read from SPEC.md's status line (`rev (\d+) \(`), and compares nothing (the precedent: `CEP_UPDATE_GOLDEN`).

**The rows:** `baseline` — `gotest.py -run ^TestBaseline$ ./internal/playback`, job `test`, T2–T4, `min_wants 1`, `lands_in S3.3-2`; and `determinism`, `determinism-macos` run `-run '^(TestDeterminismGolden|TestBaseline)$'` (A8: each OS asserts the same committed bytes), their `min_wants` 1 → 2.

### 8.3 The tool — `gate/baseline/regen.py`, run as `make regen-baseline [ACCEPT='…']`

Stdlib Python under the pinned interpreter, run **by path** — `python3.15 -I -B -X utf8 gate/baseline/regen.py [--accept '<tokens>']` from the repo root (`-I` keeps the working directory off `sys.path`, so `-m gate.baseline.regen` cannot import `gate`: the script inserts the repo root into `sys.path` before any `gate` import) — the card's `gate/regen-baseline`. Steps:
1. **Tier 1 first:** `gate/run.py --tier T2` on a two-row registry built from `gate/registry.json` (`conformance`, `semantics`) into a temporary run directory; anything but exit 0 → `refused: Tier 1 is red (run <dir>)`, exit 1, nothing written.
2. The fresh run: `go test -count=1 -run '^TestBaseline$' ./internal/playback` with `CEP_BASELINE_WRITE=<tmp>`; a non-zero exit → `refused: the fresh run failed`, exit 1.
3. Classify every block, keyed `(file, key)`, old against new; the first matching row wins:

| Class | When | Verdict |
|---|---|---|
| `format-change` | a baseline file's header `format` is not 1 | **refused** (`run regen with --migrate`: a declared step the record lists; no migration exists in format 1) |
| (none) | equal outputs and counts | — |
| `additive-new-block` | the key is new | recorded |
| `block-removed` | the key is gone and the file's `source_sha256` changed | recorded |
| `block-vanished` | the key is gone and the file's `source_sha256` did not change | **refused** (a tool fault: keys come from the source) |
| `count-change` | the number of outputs differs | **refused** |
| `ordering-change` | the same outputs as a multiset, in another order | **refused** |
| `timestamp-drift` | the same number, some output's `ts` differs | **refused** |
| `drop-change` | the drop counts differ (whatever the outputs' values) | **refused** |
| `spec-rev` | the same number, order and `ts`, other values differ, SPEC's rev is above the header's `spec_rev`, **and the block is named in `ACCEPT`** | recorded |
| `value-change` | any other value difference (incl. a would-be `spec-rev` not named) | **refused** (the tool prints its `ACCEPT` token) |

   `ACCEPT` is a space-separated list of `<file>:<rule>:<ordinal>` or `<file>` (owner, 2026-11-09, 5).
4. Any refused block: print every block with its class and exit 1, writing nothing. Otherwise copy the fresh files over `conformance/baseline/` (each header with the current `source_sha256` and `spec_rev`), print every recorded block with its class, exit 0. No difference: `no change`, exit 0.

**The Python interface** (`gate/baseline/regen.py`; what the pytests call):

```python
@dataclass(frozen=True)
class Block:
    file: str          # the corpus file's name, e.g. "07-consent-gate.cep"
    rule: str
    ordinal: int       # 1-based within the rule; display only
    key: str           # §8.1's content key, with its "#2" suffix if any
    outputs: tuple[str, ...]  # each output's canonical JSON text, as the line holds it
    counts: tuple[tuple[str, int], ...]  # sorted (name, value) pairs

@dataclass(frozen=True)
class Baseline:
    file: str
    format: int
    source_sha256: str
    spec_rev: int
    blocks: tuple[Block, ...]

@dataclass(frozen=True)
class Verdict:
    file: str
    rule: str
    ordinal: int       # the new run's ordinal; the old one's for a gone block
    key: str
    cls: str           # §8.3's class name
    recorded: bool

def parse(text: str) -> Baseline: ...                      # one .jsonl file; ValueError on a malformed line
def render(b: Baseline) -> str: ...                        # the inverse, byte for byte
def classify(old: dict[str, Baseline], new: dict[str, Baseline], spec_rev: int, accept: frozenset[str]) -> list[Verdict]: ...
def main(argv: list[str]) -> int: ...                      # steps 1–4; argv: ["--accept", "<tokens>"]
```

`classify` compares every file present in `old` or `new` (a file only in `new` gives `additive-new-block` for each block; a file only in `old` gives `block-removed` for each, since its source is gone); verdicts are sorted by `(file, ordinal, key)`; an `accept` token is `<file>` or `<file>:<rule>:<ordinal>` (the new ordinal). `main` writes nothing unless every verdict is `recorded` or there is none. **Its seams** (what the pytests stub): `REPO: str` (module level: the root `main` reads `conformance/baseline/*.jsonl` and `docs/language/SPEC.md` under; `.jsonl` files only, so the README is skipped); `run_tier1(repo: str) -> int` (step 1's gate exit code); `fresh_run(repo: str, out_dir: str) -> int` (step 2: `TestBaseline` with `CEP_BASELINE_WRITE=out_dir`, the go exit code). **What it prints** (the tests read it): every block of a refused run, the recorded ones too, each with its class as a bare token; a refused `value-change`'s `ACCEPT` token `<file>:<rule>:<ordinal>`; the literal lines `refused: Tier 1 is red`, `refused: the fresh run failed`, `no change`. SPEC's rev is read with `rev (\d+) \(`.

**Tests:** `gate/baseline/tests/test_regen.py` (the `pytests` row) — one case per class on synthetic files, plus: an unnamed `spec-rev` refused as `value-change`; equal keys taking `#2`; Tier 1 red refused (the gate runner stubbed). **Proofs (exit 6)** — `gate/proofs/drivers/test_regen_baseline.py`, in a world with the gate, the module, the corpus and the baseline. **The refused classes are planted in the stored baseline, not in the corpus** (QA lane 2's F1): a corpus edit changes the block's content key, so it reads as `block-removed` + `additive-new-block`, both recorded — by the owner's decision 4 a deliberate corpus edit is recorded, visible in the corpus diff; and no engine change can reach a refused class with Tier 1 green, since every then-record of 07, 09 and 10 names every attribute of its output. Editing the stored line models an engine change under an unchanged corpus: `count-change` (one output removed from a stored block line), `ordering-change` (two stored outputs swapped), `timestamp-drift` (one stored output's `ts` moved by 1 ms) — each refused, the baseline byte-identical afterwards; `additive-new-block` (a new rule with one block appended to 09 in the corpus) recorded. `proofs` `min_wants` 27 → 31.

## 9. The two known issues

### 9.1 S3.3-1-2: `Close` refuses new `Add`s and `Activate`s before it lists the apps — an in-flight gate

v1's barrier is replaced: a barrier queued on the intake is never answered while the router waits on a full sink (B2, A1: `runtime.go:805-829` reads no intake while a send is pending), and it left two windows (A3, S3: `Activate`'s caller-side tail after the router's answer; an `Add` still installing). The gate closes all three with one rule: **`Add` and `Activate` run inside a gate; `Close` closes the gate first and waits, bounded by its ctx, for the calls inside to return.** `Add` returns only after its install completed (its `requestQueued` reply comes from `installAck`), and `Activate` only after its tail (goroutines started, phase set), so once the gate is empty every app is fully added or fully refused, and every activation fully done.

`internal/runtime/lifecycle.go`:

```go
// inflightGate is the lifecycle's in-flight gate (S3.3-1-2): Add and Activate enter it; Close closes it and waits
// for the calls inside. sync.Mutex is internal/runtime's to use (archcheck). (Named so beside Runtime.gate, the
// per-app caller-side check of design s3-3-1 §2.5.) File: internal/runtime/inflight.go.
type inflightGate struct {
	mu      sync.Mutex
	closing bool
	n       int
	idle    chan struct{} // closed when closing and n reaches 0
}

// enter admits a call unless Close has begun.
func (g *inflightGate) enter() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return false
	}
	g.n++
	return true
}

// leave ends an admitted call.
func (g *inflightGate) leave() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n--
	if g.closing && g.n == 0 && g.idle != nil {
		close(g.idle)
		g.idle = nil
	}
}

// close refuses every later call; the channel is closed once the calls inside have left.
func (g *inflightGate) close() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closing = true
	ch := make(chan struct{})
	if g.n == 0 {
		close(ch)
	} else {
		g.idle = ch
	}
	return ch
}
```

`Runtime` gains `inflight inflightGate`. **`Add`**: `if r.isClosed() || !r.inflight.enter() { return 0, ErrClosed }; defer r.inflight.leave()`, then as today. **`Activate`**: the same two lines first, then as today (its tail runs before `leave`). **`Close`**, inside `closeOnce`:
1. `select { case <-r.inflight.close(): case <-ctx.Done(): first = ctx.Err() }`.
2. When `first == nil`: `apps := r.appList()` and the walk, in reverse id order: an app that is nil or removed is skipped; an app the router activated (`a.started.Load()`, not only `phaseActive` — `Drain` handles an activated app whose phase is still inactive, `lifecycle.go:174-188`) is drained (`Drain(ctx, id)`; a ctx error ends the walk with `first = err`; any other error is ignored, as today — e.g. `ErrDrained` after the app's own `Stop`); then `Remove(ctx, id)`: a ctx error ends the walk; `ErrRemoved` is ignored (an `Undeploy` got there first); any other error is collected and `errors.Join`ed into the result (A3: never dropped).
3. `close(r.stopped)`, the per-app `cancelSrc`/`onRemoved` loop and the wait on the group — as today (`lifecycle.go:343-364`). A ctx that ended at step 1 skips the walk and comes here at once: `Close` returns `ctx.Err()` at its deadline, whatever the router is waiting on.

The router needs no change: no barrier item, no refusal by `closing`; the existing `requestQueued` callers (`Add`, `Remove`, `Activate`) keep their reply-or-`stopped` wait, which `close(r.stopped)` ends with `ErrClosed` on the deadline path.

**MUSTs and tests** (`internal/runtime/close_gate_test.go`, `-race`): a `Deploy` and a `Start` refused with `ErrClosed` once `Close` has begun (lane 1's facade test + a `Start` case); `TestCloseReturnsAtDeadlineBehindABlockedSink` (a sink whose `Write` ignores its context, `SinkQueue: 1`, `Mailbox: 1`, rows sent until the router waits; `Close` with a 300 ms ctx returns `context.DeadlineExceeded` within the test's ceiling — red against a barrier awaited through `requestQueued`); `TestCloseDrainsAnAppActivatedBeforeItBegan` (`GOMAXPROCS=1`, 200 iterations of `Activate` racing `Close`: each ends with the app removed and `Close` returning nil); `TestCloseWaitsForAnAddInFlight` (an `Add` whose install is held — a test hook parking the shard's `mAdd` ack, as the saturation tests hold a sink — while `Close` begins: `Close` waits; once released, the `Add` returns its id and the app is drained and removed); `TestCloseReportsARemoveError` (a `Remove` that fails with a non-ctx error appears in `Close`'s result); `TestInflightGateRefusesAfterClose` (unit: `enter` false after `close`; `close`'s channel closed at once when empty, after the last `leave` otherwise).

### 9.2 S3.3-1-1: a method's entry is `<Type>.<Method>`

`tools/apipartition/main.go` (`fileSurface`, the `*ast.FuncDecl` case, `:185-191`): a method's `ident` is its receiver's type name — the receiver type with `*` and any type parameters removed (`*reader` → `reader`, `G[T]` → `G`) — `+ "." +` the method's name: `cep reader.AttrIndex func func (r *reader) AttrIndex(name string) int`. Two types' methods of one name are two keys, and an API Δ line names the type. Both goldens are rewritten with `--write` — the format only. **Tests:** lane 1's two tests (`method_entry_s3_3_2_test.go`), the fixture text `wantFixture` updated, `TestBreakitGenericTypeAndTagsArePrinted` updated for `G.Get`; and `TestGoldenRewriteIsFormatOnly`: for both `api/exports.golden` and `api/clock.golden`, every old entry (from `git show HEAD:<golden>` at the build's base) mapped through the new ident rule equals the `--write` output, line for line — the rewrite proven mechanical (A11). `apidiff` is untouched (it reads packages).

## 10. The clock freeze (item 7)

Nothing to build: `api/clock.golden` and the `clockgolden` row exist (S3.3-1); `gate/law.json` and the LAW Δ block are S1.1c's (exit 7 as annotated). At G5 the record declares: "the clock contract is frozen from `main` @ <merge sha>: a change to an exported `internal/clock` **signature** turns `clockgolden` red and needs an owner decision; a format-only rewrite of the golden by `tools/apipartition` (as §9.2's) is not a signature change." ADR-0013 is annotated with the same sentence. Exit 7's proof is the existing `clockgolden` proof.

## 11. The tests

The [testability review](../reviews/s3-3-2-design-qa.md) §1 is the test table (T1–T52); it stands with these amendments from v2: T6–T14 programs as §3.1's rows; T26 (`+∞`) as §5.5 MUST 5; T33/T35 positions as §5.4 (owner, 3); T41 `TestNotBuiltIsUnsupported` (owner, 1); T49's classes as §8.3's table; T51 as §9.1's tests (the gate, not the barrier); plus **T53** `testdata/e-time-003-optional-time.cep` (owner, 7) and **T54** `TestCorpusFactoryBindsEveryAnnotation`.

**QA lane 1 amendments** (each named in the record when made): `exRefusedDeploy`'s position check (`expect_s3_3_2_test.go:126`) and every `exWant` take §5.4's position (the then-record's `{`, or `then`) instead of `expect`'s; `TestCloseRefusesADeployOnceItHasBegun` gains a `Start` case.

**Ownership:** QA lane 2 (worktree `cep-wt/s3-3-2-qa`, branch `s3-3-2/qa`, continuing from lane 1) writes every `_test.go` file, test program, pytest and proof driver of T1–T54 not written by lane 1, red against the stubs of §12 step 1; the coordinator writes every production file, golden, baseline, registry entry and document, and edits QA's tests only to fit the real signatures, each edit named in the record.

## 12. The build, in order (the coordinator, main session; each step its own commit on `s3-3-2/integration`)

1. **Stubs** so every test compiles and fails: the model and parser fields (§2, positions zero); `plan.Expect` and its types, `App.Expects` empty; `playback.RunExpects` returning `nil, errors.New("not built")`, `Counts`, `BlockResult`; E-EXP-002 in the catalogue without a check, `Entry.Deploy`; `inflightGate` with every method panicking; `conformance/doc.go`. Commit; T0 green. Cherry-pick QA lanes 1 and 2.
2. §2 positions; `go test ./internal/parser ./internal/model` (goldens unchanged).
3. §3: `expects.go`, E-TIME-003's case, the catalogue, the programs, `errors.md`, SPEC rev 10; `go test ./internal/validate`.
4. §4 `plan/expect.go`; `go test ./internal/plan`.
5. §5 `playback/expect.go` and `NewSync`'s queues; `go test -race ./internal/playback ./internal/runtime`.
6. §6 `Deploy`; `go test -race .` (lane 1 green).
7. §9.1 the gate; `go test -race ./internal/runtime .`; the new timing tests stressed under load (`-count=50 -cpu 1,4`) before they count.
8. §7: the runner, `corpusFactory`, the `notBuilt` constructs read from one run, the row, `make ci-workflow`, the proof driver; `go test ./conformance`.
9. §8: `TestBaseline`, the first recording (`CEP_BASELINE_WRITE` into `conformance/baseline/`; the 15 blocks read by hand against the corpus and the breaker's table of worked blocks), the README, the rows, `regen.py`, its pytests, the proof driver, the `Makefile` target.
10. §9.2 `apipartition`, both goldens rewritten, `TestGoldenRewriteIsFormatOnly`.
11. T2 (`make check`), T3 (`make combine-check`): exit 0, read from the gate's own `exit:` line.
12. The record: measured results, the LAW changes by hand (both goldens, the baselines, the registry, SPEC rev 10, errors.md), the owner's check.

Then step 7 of the loop: the architecture review of the combined build and a QA break-it, one fix round (its changes under `-race` first), T4, the PR; the freeze declared at G5.

## 13. Size

About 9.75 points (owner, 2026-11-09, 8): the sandbox and its alignment 2, the static checks 1, positions and the plan 0.5, the facade 0.25, the runner and proofs 2, Tier 2 2.25, the gate 0.75, `apipartition` 0.25, the freeze 0.25, E-TIME-003 0.5 within the static checks.

## 14. The panel's findings → this note

Every finding verified by the coordinator against the source before it was taken (the reviews hold the evidence).

| Finding | Severity | Disposition |
|---|---|---|
| Breaker B1 — an omitted or null time attribute passes, stamped 1970 | blocker | fixed: §3.1 X4 (the time attribute always), §3.2 (E-TIME-003, owner 7) |
| Breaker B2 / Architect A1 — `Close`'s barrier hangs behind a blocked sink | blocker | fixed: §9.1 (the gate, waited on with ctx) |
| Breaker S1 / Challenger C6 / Architect A12 — `text_invisibles` | should-fix | fixed: §3.1 X4's exception, X10; §4 the slot left false |
| Breaker S2 / Architect A2 — a panic passes a defeat path; "missing" never says why | should-fix | fixed: §5.1 `Counts`, §5.4 cases 1 and 4, §8.1 `counts`, `drop-change` |
| Breaker S3 / Architect A3 — the barrier's windows (Activate's tail, an install in flight) | should-fix | fixed: §9.1 (the gate covers both; activated apps drained; Remove errors joined) |
| Breaker S4 / Challenger C4 — `spec-rev` records any drift | should-fix | fixed: §8.3 `ACCEPT` (owner 5) |
| Breaker S5 / Challenger C1 / Architect A7 — the ordinal key | should-fix | fixed: §8.1 the content key (owner 4); `block-removed`, `block-vanished`, `format` |
| Breaker S6 — the runner can pass empty | should-fix | fixed: §7 `wantFiles`, `test_empty_corpus_is_red`; §8.2 `wantBlocks` |
| Breaker S7 — ordering and timestamp refusals proven only synthetically | should-fix | fixed: §8.3's proofs through corpus edits of 10 |
| Breaker L1 — −0.0 | later | fixed: §5.3 signbit |
| Breaker L2 — a vacuous block passes | later | forwarded: the backlog (a lint or a rule for a block with no given event; the owner places it) |
| Breaker L3 — an unbuilt connector; one unbuilt rule hides a file's built rules | later | fixed: §7 `corpusFactory`; the second recorded as a limit of the runner until every body kind builds (S9.2) |
| Challenger C2 — positional "first divergence" misnames extra outputs | should-fix | fixed: §5.4 the alignment |
| Challenger C3 / Architect A5 — `notBuilt` labels, no construct check, no expiry | should-fix | fixed: §7 (labels from the plan's map, the construct checked); expiry forwarded to S1.1c (owner 6) |
| Challenger C5 / Architect A10 — unused queues; blocks carried into the runtime | later | fixed: §5.2 `NewSync`'s queues; §4 `App.Expects` |
| Architect A4 — the sandbox panics on the host's goroutine | should-fix | fixed: §5.1 the error, §5.2 the recover and `TableWrite` |
| Architect A6 — a stream given twice (SPEC §6.8: one per input stream) | should-fix | fixed: §3.1 X9; §5.5 MUST 4 over a synthetic two-input plan |
| Architect A8 — baselines checked on one OS | should-fix | fixed: §8.2 the determinism rows |
| Architect A9 — no ctx check before bind; an unbounded merge scan | later | fixed: §6 step 4; the merge bounded by X9 (heap rejected: at most the rule's inputs) |
| Architect A11 — the golden rewrite checked by hand; the freeze's wording | later | fixed: §9.2 `TestGoldenRewriteIsFormatOnly`; §10 "signature" |
| Architect Q1 — `errors.ErrUnsupported` | — | adopted (owner 1) |
| Architect Q3 — E-EXP-001 at the then-record | — | adopted (owner 3) |
| Testability Q-1 — X7's "reads a table" undefined | should-fix | fixed: §3.1 Definitions |
| Testability Q-2 — "the file changed in the same diff" needs a base | should-fix | fixed: §8.1 `source_sha256` |
