# S3.3-2 — design note: the expect sandbox, the conformance runner, the baselines and the clock freeze (v1, for the panel)

**Status:** v1, written for the blind panel (breaker, challenger, the architect with the-fool) and QA's testability review. Not yet build-ready: §11 lists the questions the panel and the owner settle before v2. Where this note and the code disagree, the code wins and the note is corrected (ground rule 21).

**What it covers:** items 3, 4, 6 and 7 of the [S3.3 card](sprint-plan.md#s33--the-deploy-pipeline-and-the-conformance-runner--clock-freeze), its forwards on the sandbox (M1, M2, L2, the unknown-attribute fault), exits 1, 2, 6, 7, 8, 9, and known issues S3.3-1-1 and S3.3-1-2 ([record](s3-3-2-record.md)). Not here: patterns (S4.1), windows (S8), text operators (S7), `gate/law.json` and the LAW Δ block (S1.1c).

**The owner's decisions this note carries out:** [S3.3-2's plan check](decisions.md#owner-decisions-2026-11-08--s33-2s-plan-check) — 1: a block runs with the deployment's param values, a different result refuses the deploy (E-EXP-001), no warning; 2: a mistake inside a block is its own static code, at the mistake's position, found before any block runs (one `errors.md` entry, a SPEC change-log line, no grammar change); 3: the full loop with the panel. [S3.3's plan check](decisions.md#owner-decisions-2026-11-07--s33s-plan-check), 4: a block's lists interleave by time, each in its written order, ties to the list written first. Earlier, binding: S3.2b-1's M2 (a block passes only when the outputs match the then-list in number as well as content) and L2 (`Deploy` never mutates or reuses the checked model after `plan.Build`); [S3.3-1's design](decisions.md#owner-decisions-2026-11-07--s33-1s-design-after-the-panel), 5 (the sandbox never opens connectors and ignores the annotations); S3.2a-1's break-it forward (an unknown attribute in a hand-written block must be a fault, not a null).

**The sources a reviewer opens:** SPEC §6.8 (expect blocks), §5.1 as annotated (a plain rule emits on arrival), §4.2 (streams, `optional`, `time arrival`); SEMANTICS §2 (line 47: "End of stream (and playback end): `wm → +∞`, all open state fires/expires"); `errors.md` E-EXP-001; qa-strategy §3 (Tier 1 and Tier 2); [gate red-team GR-23](../architecture/review/gate-redteam.md#gr-23-later--wording-rot); the code: `internal/parser/grammar.go` (`ExpectBlock`, `GivenSec`, `ThenSec`, `Record`, `Field`), `internal/parser/lower.go` (`expect`, `records`), `internal/model/model.go` (`Expect`, `Given`, `Then`, `Record`, `Field`), `internal/validate` (`check`, `fits`, `as`, the catalogue), `internal/plan/plan.go` (`Build`, `rule`), `internal/playback/driver.go` (`NewPlanDriver`, `Step`, `End`), `internal/runtime/sync.go` (`FromPlan`, `Sync.End`), `deploy.go` (`Deploy`), `internal/runtime/lifecycle.go` (`Add`, `Close`), `tools/apipartition/main.go`.

**Constraints that shape the code:**
- The import law (`tools/importcheck/rules.json`): `conformance` imports only `""` (package `cep`), `rule`, `io`, `io/*` — the runner uses the public API only. `cep` never imports `internal/model` (S3.3-1 MUST 12). `internal/playback` may import `internal/plan`, `internal/runtime`, `internal/validate`; its tests may import `internal/parser`.
- `archcheck`: no goroutine, channel or `sync` outside `internal/runtime`, `io/*`, `cmd`. The sandbox runs on the caller's goroutine through `runtime.Sync` (one shard stepped in place), so it needs none.
- The gate's go-test rows (`gate/runner/rows/gotest.py`, `unit.py`) judge a **skipped** test as a failure: the runner never calls `t.Skip`.

## 1. Where it lands

| Path | What | Points |
|---|---|---|
| `internal/parser/grammar.go`, `lower.go`; `internal/model/model.go` | positions on expect blocks (§2) | 0.25 |
| `internal/validate/expects.go` (new), `validate.go`, `catalog.go`, `testdata/e-exp-002-*.cep`; `docs/language/errors.md`, `SPEC.md` (rev 10) | E-EXP-002 (§3) | 0.5 |
| `internal/plan/plan.go`, new `expect.go` | the plan carries each rule's blocks as values (§4) | 0.25 |
| `internal/playback/expect.go` (new) | the sandbox: `RunExpects` (§5) | 1.5 |
| `deploy.go`, `cep.go` | the expect stage in `Deploy`; `ErrNotBuilt` (§6) | 0.25 |
| `conformance/doc.go`, `conformance/corpus_test.go`; row `conformance`; `gate/proofs/drivers/test_expect_planted.py` | the runner and the six planted proofs (§7) | 2 |
| `conformance/baseline/` (README + one `.jsonl` per built corpus file), `internal/playback/baseline_test.go`, row `baseline`, `gate/baseline/regen.py`, `make regen-baseline`, its pytests and proofs | Tier 2 (§8) | 1.5 |
| `internal/runtime/lifecycle.go`, `runtime.go` | S3.3-1-2: `Close` refuses `Add` and `Activate` before its snapshot (§9.1) | 0.5 |
| `tools/apipartition/main.go`, `api/exports.golden`, `api/clock.golden` | S3.3-1-1: a method's entry `<Type>.<Method>` (§9.2) | 0.25 |
| the record, ADR-0013 annotation | the clock freeze declared at G5 (§10) | 0.25 |

About 7.5 points, as sized at the plan check. One sprint, one PR.

## 2. Positions on expect blocks (parser, model)

Today `model.Expect`, `Given`, `Then`, `Record` and `Field` carry no position, so neither E-EXP-002 nor E-EXP-001 can point at anything. Add (every field of type `model.Pos`, so `MarshalCanonical` skips them and **no golden regenerates** — the precedent of S3.2b-2's `model.Attr.Pos`):

```go
type Expect struct {
	Givens []Given
	Then   Then
	Pos    Pos // `expect`
}
type Given struct {
	Table   bool
	Name    string
	Records []Record
	Pos     Pos // `given`
	NamePos Pos // the stream or table name
}
type Then struct {
	Name    string
	Records []Record
	Pos     Pos // `then`
	NamePos Pos // the stream name
}
type Record struct {
	Fields []Field
	Pos    Pos // `{`
}
type Field struct {
	Name     string
	Value    Literal
	Pos      Pos // the field's name
	ValuePos Pos // the value's first token (`-`, the literal, or `null`)
}
```

Parser: `GivenSec` and `ThenSec` gain `Pos Position` and their `Name` becomes `*NameTok`; `Record` gains `Pos Position`; `Field` gains `Pos Position` and its value becomes `Value *FieldValue` with `FieldValue{Pos Position; Null bool "@'null'"; Lit *SignedLiteral "| @@"}`. `lower.expect` fills `Expect.Pos` from the enclosing `Extra.Pos` (the `expect` keyword). The grammar (`grammar.ebnf`, `grammar.lark`) does not change; the parser goldens do not change (positions are skipped). **Test:** `TestExpectPositions` (parser) — 07's first block: every position above, by line and column.

## 3. E-EXP-002 — a mistake inside an expect block (validator)

**Catalogue:** `"E-EXP-002": {Title: "Mistake inside an `expect` block", Fix: "name a stream or table the rule reads or writes, and give each field its declared type."}`. `errors.md`, after E-EXP-001:

> ### E-EXP-002 Mistake inside an `expect` block **[static]**
> Rule: SPEC §6.8 (rev 10) — every name in an `expect` block is declared, and every value has its field's type; checked before any block runs.
> Message names the block, the field and the expected type. Fix: name a stream or table the rule reads or writes, and give each field its declared type.

**The check** (`internal/validate/expects.go`, a new phase `c.expects()` run last in `checker.run`, over every rule not poisoned), per block, per section, per record, per field — each finding at its own position, every finding reported (one defect, one finding):

| # | Mistake | Position | Owner's decision 2 |
|---|---|---|---|
| X1 | `given` names no declared stream; `given table` no declared table; `then` no declared stream | `NamePos` | yes |
| X2 | a field its stream or table does not have (incl. `ts` on a stream with no `ts` attribute) | the field's `Pos` | yes |
| X3 | a value that does not `fits` the field's type (`fits` and `as` of `resolve.go`: an int literal fits a float) | `ValuePos` | yes |
| X4 | `null`, written or by omission, for a non-optional attribute or column of a `given` record (the time attribute and every primary-key column are non-optional) | `ValuePos` when written; the record's `Pos` when omitted | **proposed** (Q2) — *needed for soundness*: the table refuses a row with a null in a non-optional column (`Driver.Step` panics on a refused `RecTable`), and an event with a null in a non-optional attribute reaches expressions typed as never null |
| X5 | `null` in a `then` record for a non-optional attribute of the rule's output (it can never match) | `ValuePos` | **proposed** (Q2) |
| X6 | a field named twice in one record | the second field's `Pos` | **proposed** (Q2) |
| X7 | `given` names a declared stream the rule does not read; `given table` a declared table the rule's expressions do not read; `then` a stream other than the rule's `insert into` target | `NamePos` | **proposed** (Q2) — the sandbox runs the rule alone, so such a section is silently ignored and the block passes vacuously or fails confusingly |
| X8 | `given` names a `time arrival` stream | `NamePos` | **proposed** (Q6) — a block has no arrival time to stamp |

A finding is reported once per position; a section whose name fails X1 or X7 is not checked further (its fields would all be noise). Messages (the catalogue's fix is the `Fix`):
- X1: `expect block <n> of rule <R>: unknown stream <S>[; did you mean <S'>?]` (`nearest`, as E-NAME-001 does).
- X2: `expect block <n> of rule <R>: stream <S> has no attribute <a>[; did you mean <a'>?]`.
- X3: `expect block <n> of rule <R>: <S>.<a> is <type>; the block gives a <kind> value`.
- X4/X5: `expect block <n> of rule <R>: <S>.<a> is not optional; the block gives it null` / `…leaves it out, which is null`.
- X6: `…: field <a> is named twice in one record`. X7: `…: rule <R> does not read <S>` / `does not write <S>; it inserts into <T>`. X8: `…: <S> is stamped on arrival; a block cannot give it events`.

`<n>` is the block's 1-based ordinal within its rule. Since `expects()` runs inside `Check`, every E-EXP-002 refuses the deploy before `plan.Build`, so before any block runs (decision 2; exit 8).

**E-EXP-001 in the catalogue:** it is raised by `Deploy`'s expect stage, not by `Check`. `Entry` gains `Deploy bool // raised by cep.Deploy's expect stage; its tests are the facade's and the conformance runner's`, E-EXP-001 drops `Later: "S3.3"` and sets `Deploy: true`, and `TestEveryCodeHasATestProgram` (`static_test.go`) accepts a `Deploy` code without a program and refuses one with a program.

**SPEC rev 10** (change-log line and §6.8): E-EXP-002; a block's tables are seeded before any event, wherever `given table` is written; several `given` lists interleave by time, each in its written order, ties to the list written first; a block runs the rule alone with the deployment's param values and ends as playback ends (`wm → +∞`); a block passes only when the outputs match the then-list in number, order and the named attributes. (Each sentence is what the code does; the rule fragments quoted are parsed first — sprint model §10.)

**Tests:** the validator's test programs `testdata/e-exp-002-<kind>.cep`, one per row of the table that the owner confirms (QA's tests-first lane writes X1–X3 now), each with the harness's `-- expect:` line; plus `TestExpectChecksRunBeforeBlocks` at the facade (a program with an E-EXP-002 mistake and a block that would fail reports E-EXP-002 only).

## 4. The plan carries the blocks (`internal/plan/expect.go`)

`Deploy` keeps no model (L2), and the sandbox lives in `internal/playback`, which reads plans. So `plan.Build` converts each rule's blocks into values once, from the checked model (whose params are resolved — so decision 1 holds by construction: the sandbox runs the plan `Deploy` built with the overrides):

```go
// Expect is one expect block of a rule, as values (SPEC §6.8). Build fills it from a checked model, so every name
// is declared and every value has its field's type (E-EXP-002).
type Expect struct {
	Line, Col int          // `expect`
	Tables    []GivenTable // in written order; seeded before any event
	Lists     []GivenList  // in written order
	Then      []ThenRecord // the rule's outputs, in order; empty: nothing fires
}

// GivenTable is `given table T [rows]`: each row in the table's attribute order, an omitted column null.
type GivenTable struct {
	Table int
	Rows  [][]event.Value
}

// GivenList is `given S [records]`: each event's values in the stream's attribute order, an omitted attribute null;
// Ts is the time attribute's value.
type GivenList struct {
	Stream int
	Events []GivenEvent
}

type GivenEvent struct {
	Ts     int64
	Values []event.Value
}

// ThenRecord is one then-record: the attributes it names, in written order, and their values.
type ThenRecord struct {
	Attrs  []int // indexes into the target stream's schema
	Values []event.Value
}
```

`Rule` gains `Expects []Expect`. A literal becomes a value by its field's type: string, int, float (an int literal widens, as `validate.as`), bool, duration, timestamp; `null` and an omitted field become `event.Value{Kind: t, Null: true}` (the decoder's null, `internal/event/decode.go:138`). `Build` never fails on a block (`Check` refused every bad one); a contradiction is a panic naming the rule, as `rule()`'s "Build is given only a checked model" errors are.

## 5. The sandbox (`internal/playback/expect.go`)

```go
// BlockResult is one expect block's run.
type BlockResult struct {
	Rule       string
	Block      int // 1-based within the rule
	Line, Col  int // `expect`
	Outputs    []event.Event // the rule's outputs, in emission order
	Divergence string        // "" when the block passed; otherwise the first divergence (§5.3)
}

// RunExpects runs every expect block of p, rule by rule in declaration order, block by block in written order, each
// in a fresh sandbox: the owning rule alone, its own empty tables, no sinks, no recorder, no connectors. It is
// deterministic and runs on the caller's goroutine.
func RunExpects(p *plan.App) []BlockResult
```

### 5.1 One block, step by step

1. `sub := &plan.App{Streams: p.Streams, Tables: p.Tables, Rules: []plan.Rule{p.Rules[ri]}}` — the owning rule alone (SPEC §6.8 checks *the rule's* outputs); the slices are shared read-only (`runtime.FromPlan` copies what it keeps; each body compiles its own trees through `NewBody`).
2. `d, err := NewPlanDriver(sub, func(e event.Event) { outs = append(outs, e) })` — fresh tables (`newDriver` makes them), fixture mode (no header record), so an event's arrival is its `ts` (`Driver.event`). An error here is a panic naming the rule (a built plan always drives).
3. Every `GivenTable`, in written order, every row in order: `d.Step(Record{Kind: RecTable, Table: t, Op: OpPut, Row: <a fresh copy>})` — before any event, wherever the section is written (SPEC §6.8).
4. The events, merged (decision 4): while any list has events left, take the head with the smallest `Ts`; on a tie, the list written first. Each list keeps its written order (a later event with a smaller `ts` waits its turn — example 07's arrival-order block). Each: `d.Step(Record{Kind: RecEvent, Event: event.Event{StreamID: s, Ts: ev.Ts, Values: <a fresh copy>}})` — a fresh slice per event, since the clock and the operators keep events (ADR-0010 §4). A stream-late event is dropped and counted, as live ingest drops it.
5. `d.End()` — the playback end of SEMANTICS §2: every contribution to `+∞`, every timer fires, every key purges. *(The plan check's reading 3 said the block "drains as `Stop` does (one past the max event time)"; `Driver.End` in a fixture is `+∞`, which is what SEMANTICS §2 says of playback end. For plain rules, which emit on arrival and hold no timer, the two agree; they differ for S4.1's absence timers. The panel is asked to confirm `+∞` — Q5.)*
6. Compare `outs` with `Then` (§5.3).

### 5.2 MUSTs

1. A fresh sandbox per block: nothing a block does is visible to the next (tables, rate-limit anchors, keys, clocks). — `TestBlocksDoNotShareState` (a rate-limited rule whose second block passes only with a fresh anchor; a table seeded in block 1 empty in block 2).
2. The owning rule alone: another rule of the app neither runs nor contributes outputs. — `TestOtherRulesDoNotRun` (two rules on one stream; a block of the first passes only when the second does not run: the second rule's expression errors are not counted, `Stats` shows one rule).
3. Tables before events, wherever written. — `TestGivenTableWrittenAfterEvents`.
4. Interleave by time, written order within a list, ties to the list written first. — `TestListsInterleaveByTime` (two lists of one stream; three ties), `TestListOrderIsArrivalOrder` (07's last block).
5. Ends at `+∞`. — `TestBlockEndsAtPlaybackEnd` (a plain rule cannot show it; the test pins `Driver.End`'s advance to `+∞` through `Step.Advances` and is extended by S4.1 with an absence).
6. Nothing live: `RunExpects` takes only a plan; it opens no connector, writes no recording and touches no table of a deployment. — the facade's tests (§6).
7. Deterministic: the same plan gives the same results, byte for byte. — `TestRunExpectsIsDeterministic` (run 07, 09, 10 twice; compare `BlockResult`s deeply).

### 5.3 The comparison and the first divergence

A block passes iff `len(outs) == len(Then)` and, for every `i`, every attribute `Then[i]` names equals `outs[i]`'s (M2: number, order and named attributes; a then-record asserts nothing about an attribute it omits). Values are equal when both are null, or neither is and they have the same kind and the same value (`==` on `Str`, `I`, `F`, `B`, `T`, `D`; a then-record can hold no object — X3). Walk `i` from 1; the first difference is the divergence:

- a named attribute differs: `output <i>: <a> is <actual>, expected <expected>`;
- fewer outputs: `output <i> is missing: expected {<then-record as named>}; the rule emitted <k>`;
- more outputs: `output <i> was not expected: {<the output's attributes>}; the then-list has <m>` (an empty then-list says `the then-list is empty`).

Values are printed in their canonical form (`AppendValue`; a timestamp as its RFC 3339 text); a value `AppendValue` refuses (a non-finite float) is printed with `%v`. A wrong order shows as a value divergence at the first position where the orders differ; a wrong timestamp as `ts is …, expected …`. These are exit 2's six cases: wrong value, missing output, extra output, wrong order, an output under an empty `then`, wrong timestamp.

## 6. The facade: the expect stage in `Deploy`

`Deploy` (design s3-3-1 §3.2) becomes: parse → overrides → `validate.Check` (E-EXP-002 here) → `plan.Build` → **`playback.RunExpects(p)`** → bind → `FromPlan` → `Add`. The stage sits **before `bind`**, so a refused deploy has constructed no connector, and nothing it does reaches a sink, a recording or a live table (MUST 6). Every block with a `Divergence` becomes one `Diagnostic{Code: "E-EXP-001", Line, Col, Message: "rule <R>, expect block <n>: <divergence>", Fix: catalog fix}`, in result order (rule declaration order, then written order); any failing block refuses the deploy with a `*DeployError` holding them all; no app is registered.

`plan.ErrNotBuilt` is mapped to a new public sentinel, **`cep.ErrNotBuilt`** (`errors.New("cep: this release cannot run the construct yet")`, the plan's message kept after it), so the conformance runner can tell "not built yet" from any other failure through the public API — **an API change (Q1)**: one line in `api/exports.golden`.

**Tests (the facade):** QA's tests-first lane (exits 1, 2, 8, 9 through `Deploy`; MUST 6: a Go-bound source and sink see no call during `Deploy`, a recording holds nothing from the blocks, a block's `given table` row is absent from the deployed app's table, a refused deploy leaves no app). Plus `TestNotBuiltIsPublic` (example 01 → `errors.Is(err, cep.ErrNotBuilt)`).

## 7. The conformance runner (`conformance/`)

`conformance/doc.go` (package `conformance`, a doc comment only: the corpus runs through the public API) and `conformance/corpus_test.go` (package `conformance_test`):

```go
// notBuilt is every corpus file this release cannot deploy, with the sprint that builds it. A file listed here that
// deploys turns the runner red (remove its line); a file not listed that does not deploy turns it red.
var notBuilt = map[string]string{
	"01-agent-kill-switch.cep": "S4.1", "06-once-and-group.cep": "S4.1", "08-session-flow.cep": "S4.1",
	"02-cron-digest.cep": "S8", "03-sort-leaderboard.cep": "S8", "04-delay-and-batch.cep": "S8",
	"05-text-operators.cep": "S7",
	"torture/comments.cep": "S4.1", "torture/lexical.cep": "S4.1",
}

func TestCorpus(t *testing.T) // one sub-test per file of docs/language/examples/ and examples/torture/, sorted
```

Each sub-test: `cep.New(cep.Config{})`, `Deploy(ctx, file, src)`, `Close`. A listed file must fail with `errors.Is(err, cep.ErrNotBuilt)` and the sub-test **passes** with `t.Logf("not built until %s", sprint)` (a skip is red in the gate); an unlisted file must deploy — every block passing is exit 1; a `*DeployError` is reported as `t.Errorf` with each diagnostic on its own line (`E-EXP-001 17:3 rule HighRisk, expect block 1: output 1: scaled is 1, expected 2`).

**The row** `conformance`: `gotest.py -run ^TestCorpus$ ./conformance`, job `test`, T2–T4, blocking, `min_wants 1`, `lands_in S3.3-2`. (`unit` runs it too; the row exists so a planted proof names it.)

**The six planted proofs (exit 2)** — `gate/proofs/drivers/test_expect_planted.py`, the `support.DriverTest` shape of `test_semantics_red.py`: the world holds the gate, the module and the corpus; a clean run passes; each of the six plants is a one-line edit of a corpus block in the world (anchored by its exact text, the anchor's count asserted 1), and the `conformance` row goes red naming that file, its test output holding `E-EXP-001` at the block's line and the divergence of §5.3. `proofs` `min_wants` 19 → 26 (six plants and the clean control).

## 8. Tier 2: the baselines and `gate/regen-baseline`

**What is locked:** the full outputs of every expect block of every built corpus file — today 07 (6 blocks), 09 (4), 10 (5): 15 blocks. The expect blocks assert what their then-records name; the baseline locks everything else (an unasserted attribute, a `ts` a then-record omits).

**The files:** `conformance/baseline/<corpus file name>.jsonl`, one per built file: the first line `{"file":"07-consent-gate.cep","spec_rev":10}`, then one line per block, rule by rule in declaration order and block by block in written order: `{"block":1,"outputs":[<each output as playback.AppendEvent writes it>],"rule":"UnknownAppConsent"}` (keys sorted, no spaces). `conformance/baseline/README.md` is the prose table of the classes. *(The baseline files are law paths in ADR-0015 §1.3's sense; until S1.1c's LAW Δ block, the record lists a baseline change by hand.)*

**The check** — `internal/playback/baseline_test.go`, `TestBaseline`: for each built corpus file, parse (a test import), `validate.Check`, `plan.Build`, `RunExpects`, encode, and compare line by line with its baseline file, ignoring the header's `spec_rev`; a difference is `t.Errorf` naming the file, rule and block and showing both lines; a built file without a baseline, or a baseline without a built file, is red. With `CEP_BASELINE_WRITE=<dir>` set it writes the files there instead (the regen tool's step; the precedent is `CEP_UPDATE_GOLDEN`), with `spec_rev` read from SPEC.md's status line. **The row** `baseline`: `gotest.py -run ^TestBaseline$ ./internal/playback`, job `test`, T2–T4, `min_wants 1`.

**The tool** — `gate/baseline/regen.py` (stdlib Python under the pinned interpreter; the card's `gate/regen-baseline`), run as `make regen-baseline`:
1. **Tier 1 first:** runs the gate (`gate/run.py --tier T2`) on a two-row registry, `conformance` and `semantics`; anything but exit 0 → `refused: Tier 1 is red` and exit 1, nothing written.
2. Writes the fresh run to a temporary directory (`CEP_BASELINE_WRITE`).
3. Classifies each block, keyed `(file, rule, block)`, old against new:

| Class | When | Verdict |
|---|---|---|
| (none) | the outputs are equal | — |
| `additive-new-rule` | the key is new (a new file, rule or block) | recorded |
| `count-change` | the number of outputs differs; or the key is gone (Q4) | **refused** |
| `ordering-change` | the same outputs as a multiset, in another order | **refused** |
| `timestamp-drift` | the same number, some output's `ts` differs | **refused** |
| `spec-rev` | the same number, order and `ts`, other values differ, and SPEC's rev is above the baseline's `spec_rev` | recorded |
| `value-change` | the same, with SPEC's rev unchanged | **refused** |

   The first matching row from the top wins for a block.
4. If any block is refused: print every block with its class, write nothing, exit 1. Otherwise copy the fresh files over the baseline (with the current `spec_rev`), print every recorded block with its class, exit 0. No difference: `no change`, exit 0.

**Proofs (exit 6):** the classifier's pytests (`gate/baseline/tests/test_regen.py`, run by the `pytests` row: one case per class, on synthetic files); and `gate/proofs/drivers/test_regen_baseline.py`: in a world, a corpus edit that changes a block's output count (its given list and then-list both extended, so Tier 1 stays green) is refused as `count-change` and the baseline is unchanged byte for byte; a new rule with a block added to a corpus file is recorded as `additive-new-rule`. (A count or order change cannot reach the classifier through the engine alone: the blocks assert both, so Tier 1 refuses it first. A `ts` or an unasserted value can.)

## 9. The two known issues

### 9.1 S3.3-1-2: `Close` refuses `Add` and `Activate` before its snapshot

Today `Close` reads `r.appList()` and walks it; an `Add` the router accepts after that read is never drained or removed. Change (`internal/runtime`):
1. `Runtime` gains `closing atomic.Bool`. `Close` stores `true` **first**, inside `closeOnce`.
2. The router answers an `iAdd` or an `iActivate` with `ErrClosed` when `closing` is set.
3. `Close` then queues a barrier item (`iBarrier`, answered by the router in intake order) and waits for its answer before reading `appList()`: every `Add` the router accepted is in the list, and every later one is refused. If `Close`'s ctx ends before the barrier is queued, it reads the list as today (the deadline is already its result).

**Tests:** QA's tests-first lane (a `Close` held mid-drain by a blocked sink; a `Deploy`, then a `Start`, in that window → `ErrClosed`; no app left unremoved). `TestCloseBarrierOrdersAnAddQueuedBefore` (an `Add` queued before the barrier is drained and removed).

### 9.2 S3.3-1-1: a method's entry is `<Type>.<Method>`

`tools/apipartition`: a method's `ident` is its receiver's type name (no `*`, no type parameters) + `.` + the method's name — `cep reader.AttrIndex func func (r *reader) AttrIndex(name string) int` — so two types' methods of one name are two distinct keys and an API Δ line names the type. Both goldens (`api/exports.golden`, `api/clock.golden`) are rewritten with `--write`, the format change only (reviewed line by line: every entry's signature unchanged); the record lists both law files as changed. **Test:** QA's (the tool's new form); `apidiff` is untouched (it reads packages, not the golden).

## 10. The clock freeze (item 7)

Nothing to build: `api/clock.golden` and the `clockgolden` row exist (S3.3-1); `gate/law.json` and the LAW Δ block are S1.1c's (exit 7 as annotated). At G5 the record declares the freeze ("the clock contract is frozen from main @ <merge sha>; a change to an exported `internal/clock` signature turns `clockgolden` red and needs an owner decision"), and ADR-0013 is annotated with the same line. Exit 7's proof is the existing `clockgolden` proof, which this sprint does not touch (§9.2's format change re-runs it).

## 11. Open questions (for the panel, then the owner)

- **Q1 (API):** a public `cep.ErrNotBuilt` so the runner tells "not built yet" from other failures (recommended); or the runner matches the error text; or `conformance` may import `internal/plan` (an import-law change).
- **Q2 (decision 2's reach):** are X4 (null for a non-optional field, written or omitted — needed so the sandbox cannot be handed a row a table refuses), X5, X6 and X7 E-EXP-002? Recommended: all four.
- **Q3 (the message):** E-EXP-001 at the block's `expect`, one diagnostic per failing block, the first divergence in §5.3's words — or at the diverging then-record?
- **Q4 (the baselines):** a corpus edit that changes a block's outputs is recordable only under a SPEC revision (`spec-rev`) or as new blocks (`additive-new-rule`); a removed block counts as `count-change` and is refused. Is that the intended discipline, or does a deliberate corpus edit need its own recordable class?
- **Q5 (the end of a block):** `+∞` (SEMANTICS §2, `Driver.End`), not `Stop`'s one-past-the-max (the plan check's reading 3).
- **Q6:** a `time arrival` stream cannot be given events in a block (X8) — or each given event is stamped with the block's latest `ts` so far.
- **Q7 (later):** an `object` attribute cannot be written in a block (v0.1 has no object literal), so a stream with a non-optional `object` attribute cannot carry a block. Forward?

## 12. The tests (QA first) and the build, in order

QA's tests-first lane (worktree `cep-wt/s3-3-2-qa`, branch `s3-3-2/qa`) writes, from the card, SPEC and the owner's decisions only: the facade's tests for exits 1, 2, 8, 9 and MUST 6; the E-EXP-002 programs X1–X3; S3.3-1-2's and S3.3-1-1's tests. After v2, a second QA lane writes the sandbox's unit tests (§5.2), the runner's proofs (§7), the classifier's pytests and the regen proofs (§8).

The build (the coordinator, the main session, one commit per step on `s3-3-2/integration`): 1. stubs so every test compiles and fails (the new types and `RunExpects` panicking, `cep.ErrNotBuilt`, E-EXP-002 in the catalogue without a check); cherry-pick QA's lane. 2. §2 positions. 3. §3 E-EXP-002, the programs, errors.md, SPEC rev 10. 4. §4 the plan. 5. §5 the sandbox. 6. §6 the stage in `Deploy`. 7. §7 the runner, the row, the proofs. 8. §8 the baselines, the tool, the row, the proofs (the 15 blocks recorded once, read by hand against the corpus). 9. §9 the known issues. 10. T2, T3. Then the combined review and the break-it, one fix round under the race detector, T4, the PR; the freeze declared at G5.
