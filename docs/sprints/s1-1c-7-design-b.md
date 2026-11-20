# S1.1c-7 design note — B (internal/playback)

*S1.1c-7's design note for chunk B (internal/playback), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes.*


## The note

## S1.1c-7 design note, chunk B (`internal/playback`, lane B)

**What the note covers.**
- **Findings:** 24 (G10-18, G10-19, and the 22 G2s from G2-89 to G2-110).
- **Base:** `s1-1c-7/integration` at d5217259fa399824230a1eaa37df073d1e64ac67.
- **Files read:** `records.go`, `expect.go`, `driver.go`, `reader.go`, `recorder.go` (heads of `Table` and `Event`), `encode.go` (heads of the encoders), and the named sections of 21 test files. Outside the package I read `deploy.go`, `internal/runtime/{drain,router_in,build,sink,recorder,shutdown,sync}.go`, `internal/partition/partition.go`, `internal/validate/{bounds,fixround_test}.go`, `internal/plan/plan.go`, `internal/parser/lower.go`, `internal/model/model.go`, `internal/clock/clock.go`, `internal/event/{decode,scanner}.go`, `gate/tripwire.json`, `gate/proofs/drivers/test_{regen_baseline,tripwire_planted}.py`, and SPEC §4 rev 4/5.
- **GitNexus calls: 4.**
  - `context blUses`: one caller, `blKeys`.
  - `context Trace`: methods `Step`, `Released`, `ConstructLate`, `Fired`, `Discarded`, `Purged`, `Note`, `Bytes`. It has no late counter.
  - `impact recordKind` upstream: LOW, 3 symbols, all in Playback; process `record`.
  - `impact runBlock` upstream: LOW; d=1 `RunExpects`, d=2 `Runtime.Deploy` plus tests.
- **What I could not check:**
  - I could not read the `gitnexus://repo/cep/context` resource (no resource tool here). The brief states the index is fresh at d521725.
  - No runs were allowed, so every "old"/"new" outcome below is reasoned from the code. Three expectations carry a named risk with a stop rule: G2-92, G2-107 and G2-94's control.
  - Time box: every finding was reached.

### Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-18 | accepted | K1 | B-K1 | – | `has no %q key` → `takes no %q key` |
| G10-19 | accepted | K1 | B-K1 | – | runBlock strips the inner `playback: ` from a wrapped driver error and keeps it in the chain |
| G2-89 | accepted | K2e | B-K2 | model.go:199 comment → CD | blUses reads a window body's source and returns an error on an unhandled body kind |
| G2-90 | accepted | K2a | B-K2 | move to validate + merge → later part; rename → S1.1c-8 | pins validate's E-PART-004 refusal (SPEC rev 4); early returns removed |
| G2-91 | accepted | K2a | B-K2 | rename → S1.1c-8 | calls production `envelopeSchemas()` |
| G2-92 | accepted | K2a (risk named) | B-K2 | – | eqLive fails when stamps are left over |
| G2-93 | accepted | K6 | B-K6 | port to NewPlanDriver → later part | header comment re-states what the harness is; no "will" |
| G2-94 | accepted | K2a | B-K2 | partition's `world.advance` → E2 | step fails on `Stats().Panics != 0` and returns the CheckTimers error |
| G2-95 | accepted | K2-del | B-del | – | delete the test of bufio's contract |
| G2-96 | accepted | K2a (subtest) | B-K2 | – | "payload not an object" gets `decode: true`; the name-prefix exemption goes |
| G2-97 | accepted | K2c | B-K2 | – | wait for the 20 alerts by channel before the 300 ms drain |
| G2-98 | accepted | K2e (live data) | B-K2 | – | TestTripwire fails when a numeric-ceiling metric was never measured |
| G2-99 | accepted | K2a | B-K2 | – | asserts `queue full after record <Records>:` |
| G2-100 | accepted | K2e | B-K2 | – | one deferred release-and-close on every exit path |
| G2-101 | accepted | – | none | **carried in full to S1.1c-8** | rename only |
| G2-102 | accepted | K2e | B-K2 | – | enEntries looked up by name |
| G2-103 | accepted | K2a | B-K2 | – | `errors.Is(err, io.EOF)` |
| G2-104 | accepted | K2a | B-K2 | – | asserts the walked-stream counts; doc reworded |
| G2-105 | accepted | K2e (bench input) | B-bench | – | one checked call before `b.Loop()` in four benchmarks |
| G2-106 | accepted | K2e | B-K2 | partition's ttl_test/panic_test → E2 (other owner) | t.Parallel in 17 encode tests and TestBaseline; a reason on the AllocsPerRun test |
| G2-107 | accepted | K2a (risk named) | B-K2 | eqSync reuse and stream id in eqResult → later part | coverage guard requires `ConstructLate > 0` |
| G2-108 | accepted | K2a | B-K2 | – | the cleanup reports Close's error |
| G2-109 | accepted | K2e | B-K2 | – | guarded Stats reads at the four sites |
| G2-110 | accepted | K2e | B-K2 | rename of `d2` → S1.1c-8 | checks NewDriver's error |

---

### G10-18 — K1

1. **Verdict: accepted.**
   - `records.go:125-128` loops over the envelope keys. It fires when `!vals[i].Null && !rk.allowed[i]`: the key **is present** and the kind does not allow it. The message says the record "has no" key, which is the opposite of what the user wrote.
   - GitNexus `impact recordKind` upstream: LOW, process `record`.
2. **Kind: K1.**
   - R5: `recordKind` runs per line on the replay path, so it is per-event code.
   - The edit is to an error branch's text only, so it is exempt from K1h. This note names the exemption.
3. **Change.**
   - `records.go:127`: `r.fault("a %s record has no %q key", …)` → `r.fault("a %s record takes no %q key", …)`.
   - The test is the existing `TestReaderNamesTheFirstFaultOfALine` (`reader_refusals_internal_test.go:296`). Its two rows at `:306` and `:308` become `` `a table record takes no "stream" key` `` and `` `a tick record takes no "stream" key` ``.
   - The test was not renamed and no row name changes.
4. **Proof.**
   - **Revert plant:** `internal/playback/records.go`, find `"a %s record takes no %q key"`, replace with `"a %s record has no %q key"`.
   - `--expect` entry:
     ```json
     {"id":"G10-18","file":"internal/playback/records.go","find":"\"a %s record takes no %q key\"","replace":"\"a %s record has no %q key\"","test":"TestReaderNamesTheFirstFaultOfALine","subtest":"a table with a stream key names the key","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
     ```
   - Single-mode red line: `cnCheckFaults`'s message-mismatch report on that row.
   - Dry run of the old text: `git grep 'has no %q key\|record has no'` finds the production line, the test lines `:306` and `:308`, `docs/sprints/s1-1c-6-design.md:1095` (Historical; left alone), and the survey. Nothing else pins it.
5. **Commit:** B-K1.

### G10-19 — K1

1. **Verdict: accepted.**
   - `expect.go:49-51`: `fault` prefixes `playback: rule %s, expect block %d: `.
   - `:62` wraps `NewPlanDriver`'s error with `%w`.
   - Every error `newDriver` returns already starts `playback: `: `driver.go:43,53` and `reader.go:120,135,137,139`.
   - So Deploy prints `cep: deploy X: the expect sandbox: playback: rule R, expect block 1: playback: stream 0 has no schema` (`deploy.go:80-82`).
   - The panic case (`:56`) and the given-table case (`:66`, a `runtime`/`table` error) are not doubled.
   - GitNexus `impact runBlock`: d=1 `RunExpects`, d=2 `Runtime.Deploy`. R5: Deploy is not a per-event entry point; the edit is in an error branch.
2. **Kind: K1.**
3. **Choice, input and text.** The input is a sandbox `NewPlanDriver` refuses.
   - **Candidates:**
     - (a) TrimPrefix the text and drop `%w`. This loses the chain, which carries table's sentinels and runtime's build errors.
     - (b) Move the prefix out of `newDriver` and validateApp. This changes NewReader's and NewDriver's texts as well.
     - (c) A small unexported wrapper that trims the text and keeps the error in the chain.
   - **Chosen: (c).** It is the only option that changes one text and keeps `errors.Is`/`As`.
   - **New text:** `playback: rule R, expect block 1: stream 0 has no schema`.
   - **Code** (`expect.go`, add `"strings"` to its imports):
     ```go
     // inPlayback is a playback error wrapped by runBlock's fault, which already says "playback: ": its text without
     // that prefix, the error itself still in the chain.
     type inPlayback struct{ err error }

     func (e inPlayback) Error() string { return strings.TrimPrefix(e.err.Error(), "playback: ") }
     func (e inPlayback) Unwrap() error { return e.err }
     ```
     `:62` becomes `return BlockResult{}, fault("%w", inPlayback{err})`.
   - **Tests:**
     - `expect_engine_faults_test.go:34` want → `"playback: rule R, expect block 1: stream 0 has no schema"`.
     - New internal test `TestASandboxFaultKeepsTheDriversErrorInTheChain` in `expect_verdict_internal_test.go` (package playback):
       ```go
       func TestASandboxFaultKeepsTheDriversErrorInTheChain(t *testing.T) {
       	t.Parallel()
       	p := &plan.App{Streams: []plan.Stream{{}}, Rules: []plan.Rule{{Name: "R"}}, Expects: []plan.Expect{{Block: 1}}}
       	_, err := runBlock(p, &p.Expects[0])
       	var w inPlayback
       	if !errors.As(err, &w) || w.err.Error() != "playback: stream 0 has no schema" {
       		t.Fatalf("runBlock = %v; want the driver's own error in the chain", err)
       	}
       }
       ```
       Add imports as needed.
   - **Dry run of the old text:** `git grep 'expect block 1: playback:'` finds only `expect_engine_faults_test.go:34`.
     - `expect_given_table_fault_test.go:20` (prefix `…given table T: `) is unaffected.
     - `expect_test.go:329` checks `rule R, expect block 1`, which stays.
     - `reader_refusals_internal_test.go:150` pins NewReader's text, which is unchanged.
     - No root-package test pins `the expect sandbox:` (grep).
4. **Proof.**
   - **Revert plant** (keeps the type, which the internal test needs): find `fault("%w", inPlayback{err})`, replace with `fault("%w", err)`.
   - Entry:
     ```json
     {"id":"G10-19","file":"internal/playback/expect.go","find":"fault(\"%w\", inPlayback{err})","replace":"fault(\"%w\", err)","test":"TestRunExpectsReportsASandboxThatCannotBeBuilt","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
     ```
     The failing line is the test's `t.Errorf` at `:36`.
   - **The chain test's own plant:** find `func (e inPlayback) Unwrap() error { return e.err }`, replace with `func (e inPlayback) Unwrap() error { return nil }`. Expected red on `TestASandboxFaultKeepsTheDriversErrorInTheChain`; `errors.As` still finds `w`, but add `errors.Unwrap(err)` checks if the lane finds `As` unaffected.
     - **Correction for the build:** this plant does not fail that test, because `As` stops at `inPlayback` itself.
     - **Instead use:** find `return BlockResult{}, fault("%w", inPlayback{err})`, replace with `return BlockResult{}, fault("%v", inPlayback{err})`. That drops the chain; old (no test) n/a, new **red**.
5. **Commit:** B-K1. It is not K1h: the edit is an error branch on a Deploy-only path, named here.

### G2-89 — K2e

1. **Verdict: accepted.**
   - `parser/lower.go:186-205`: a window body sets `Body{Kind: BodyWindow, Window: w}` with `w.Source` and leaves `Body.Source` empty.
   - `baseline_test.go:231` reads only `r.Body.Source`.
   - The defect is latent today: a window file is `ErrNotBuilt` (`blRun:131`), so blUses never sees a window rule. It surfaces when windows build.
   - GitNexus: blUses' only caller is `blKeys`.
2. **Kind: K2e** (structure). No production plant can show it, because windows are not built (`plan.go:75`).
3. **Change** (`baseline_test.go`).
   - blUses gets the signature `func blUses(r *model.Rule) (map[string]bool, map[string]bool, error)`. Its body replaces the `if src := …` block with:
     ```go
     	var src *model.Source
     	switch r.Body.Kind {
     	case model.BodyPlain:
     		src = &r.Body.Source
     	case model.BodyWindow:
     		src = &r.Body.Window.Source
     	default: // a pattern's steps come with the pattern engine
     		return nil, nil, fmt.Errorf("a %q body: the block key does not read it yet", r.Body.Kind)
     	}
     	streams[src.Stream] = true
     	walk(&src.Filter)
     ```
     It ends with `return streams, tables, nil`.
   - blKeys: `streams, tables, err := blUses(&r); if err != nil { t.Fatalf("rule %s: %v", r.Name, err) }`.
   - The doc comment's "a plain or window body; S4.1 adds…" goes. The new comment holds no sprint id.
   - New test in the same file, `TestTheBlockKeyReadsAWindowRulesSourceAndItsFilterTables`, with `t.Parallel()`:
     - A rule `{InsertInto: "Out", Body: model.Body{Kind: model.BodyWindow, Window: model.Window{Source: model.Source{Stream: "S", Filter: model.Expr{Kind: model.ExprIn, Table: "T"}}}}}` gives streams `{Out, S}` and tables `{T}` (`maps.Equal`).
     - A rule with `Kind: model.BodyPattern` gives a non-nil error.
   - Existing keys are unchanged. For a plain body the old guard `src.Stream != ""` is equivalent: a `""` entry matches no stream. So TestBaseline stays green against the stored baselines.
4. **Proof.**
   - **K2e check:** `git grep -n -e 'case model.BodyWindow:' -e 'r.Body.Window.Source' internal/playback/baseline_test.go`. Old: no lines, exit 1 (red). New: 2 lines (green).
   - Then `go test ./internal/playback -run '^(TestBaseline|TestTheBlockKeyReadsAWindowRulesSourceAndItsFilterTables)$'` passes, which shows no stored key moved.
   - Coupling: `blWantBlocks = 15` stays exactly once (`test_regen_baseline.py:230-232`). No new import outside the main module. The proofs row runs alone afterwards.
5. **Commit:** B-K2.
   - **Other owner:** `internal/model/model.go:199` (`Source Source // plain and window`) is wrong in the same way. It is CD's file, so it is forwarded to the coordinator and not planned here.

### G2-90 — K2a

1. **Verdict: accepted.** `breakit_s3_2b_1_test.go:91-97` returns on refusal at either layer and ignores the diagnostics.
   - The behaviour is **decided**, not undecided: SPEC rev 4 (SPEC.md:283, :98) says "a stream named twice in a partition clause is refused (`E-PART-004`)".
   - Validate reports it at `bounds.go:125`, and plan backs it up at `plan.go:231`.
   - `internal/validate/fixround_test.go:23` already pins the same shape. So no choice is needed; the plan check's listing of G2-90 as undecided is answered by the SPEC.
2. **Kind: K2a.**
3. **Change.**
   - Replace `:91-107` with:
     ```go
     	app, diags := validate.Check(res.App, validate.Options{})
     	if app != nil || len(diags) != 1 || diags[0].Code != "E-PART-004" {
     		t.Fatalf("validate.Check = app %v, %v; want no app and one E-PART-004 (SPEC: a stream keyed twice in one partition clause is refused)", app != nil, diags)
     	}
     ```
   - Remove the now-unreachable Build and replay lines and any import that becomes unused (`fmt` and `plan` stay if other tests in the file use them: check `bkKeyedSrc` users).
   - The comment on `bkTwoKeysSrc` (`:62-63`, "The validator accepts it (no E-PART-* finding)") becomes "validate.Check refuses it: E-PART-004 at the second key".
   - The test's doc changes to match.
   - Removing the replay assertion is subsumed: it ran only when validate accepted, and the new pin is red whenever validate accepts (plant 2 below).
4. **Proof.**
   ```json
   {"id":"G2-90a","file":"internal/validate/bounds.go","find":"c.report(\"E-PART-004\", k.Pos,","replace":"c.report(\"E-PART-007\", k.Pos,","test":"TestBreakitPartitionKeyNotDropped","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   {"id":"G2-90b","file":"internal/validate/bounds.go","find":"f, twice := first[k.Stream]","replace":"f, twice := first[k.Stream]\n\t\ttwice = false","test":"TestBreakitPartitionKeyNotDropped","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   {"id":"G2-90-control","control":true,"test":"TestBreakitPartitionKeyNotDropped","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
   ```
   - In plant b, validate accepts; old: Build refuses at `plan.go:231`, so the test returns green.
5. **Commit:** B-K2.
   - **Carried:** the move to `internal/validate` and the merge with `fixround_test.go:23` go to the later part (Q4).
   - **Rename** → S1.1c-8: `TestAPartitionClauseNamingTwoKeysOfOneStreamIsRefused`. "is refused" ← no app, one E-PART-004.

### G2-91 — K2a

1. **Verdict: accepted.** `encode_refusals_internal_test.go:149-162` builds its own copies of the schemas. Production is `envelopeSchemas()` at `reader.go:68-83`.
2. **Kind: K2a.**
3. **Change.** The body becomes:
   ```go
   	if _, _, _, err := envelopeSchemas(); err != nil {
   		t.Fatalf("envelopeSchemas: %v", err)
   	}
   ```
   The doc's stale line references `reader.go:97-110` now name `envelopeSchemas`. Drop the `event` import if it becomes unused.
4. **Proof.** Plant a duplicate attribute in the header schema:
   ```json
   {"id":"G2-91","file":"internal/playback/reader.go","find":"[]event.Attr{{Name: \"start\", Type: event.TTimestamp}}","replace":"[]event.Attr{{Name: \"start\", Type: event.TTimestamp}, {Name: \"start\", Type: event.TTimestamp}}","test":"TestEveryEnvelopeSchemaIsValid","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   ```
   Plus a control entry (green/green).
   - The find text occurs once in `reader.go` (grep: `:73`).
   - It relies on `event.NewSchema` refusing a duplicate attribute name. The lane confirms with the plant's new run reading red; if `NewSchema` accepts duplicates, plant `Name: ""` instead.
5. **Commit:** B-K2. **Rename** → S1.1c-8: `TestTheEnvelopeSchemasAreValid` ("are valid" ← `envelopeSchemas()` returns a nil error).

### G2-92 — K2a (risk named)

1. **Verdict: accepted.** The doc at `equivalence_test.go:192-193` says any other read fails the test, but `:207-218` and `:257-259` catch only reads past the end.
   - Reads are exactly 1 + events. `build.go:28-31` reads once at Start when it has apps. `router_in.go:60` (`ingestEvent`) reads once per event. Ticks pass their own time; the internal ticker is off (`TickEvery: 0`).
   - So on real code no stamps are left over.
2. **Kind: K2a.**
3. **Change.** After the `extra` check, inside the same `mu` section:
   ```go
   	if len(stamps) != 0 {
   		return nil, fmt.Errorf("Config.Now read %d times short of once at Start and once per event (design §7.8)", len(stamps))
   	}
   ```
4. **Proof.**
   - Plant: Start reads no stamp. File `internal/runtime/build.go`, find `	if len(apps) > 0 {\n\t\tstart = cfg.Now()\n\t}`, replace with `	if false && len(apps) > 0 {\n\t\tstart = cfg.Now()\n\t}`. Test `TestLiveEqualsReplay`, pkg `./internal/playback`, expect old green, new red, plus a control.
   - **Risk** (the-fool pre-mortem): the stamps are popped by position, so any fewer-read plant shifts every later arrival by one. Stream B's `idle after 1 min` reads arrivals, so the old comparison may already go red on some seed. That would make the entry's old outcome red, and then it is not K2a evidence.
   - **Stop rule:** the lane first runs this plant alone at `old_rev` (plain `mutate.py --file … --test TestLiveEqualsReplay --pkg ./internal/playback`).
     - If it reads red, the lane commits no G2-92 change and reports back.
     - The finding then narrows to arrival-neutral reads, and the coordinator decides between K2e (a grep that eqLive checks `len(stamps)`) and rejection.
   - The test runs goroutines, so a load run is needed: `python3.15 -I -B gate/bench/load.py -- -race -count=50 -run '^TestLiveEqualsReplay$' ./internal/playback`.
5. **Commit:** B-K2.

### G2-93 — K6 (comment only, test file)

1. **Verdict: accepted.**
   - `helpers_log_test.go:3-4` says the harness drives `partition.Rule` "the way S3.2a-2's router will".
   - The router exists: `driver.go:22` holds `*runtime.Sync`, and `NewPlanDriver` (`:33`) replays real rules.
   - `ldWorld.step` (`:329-370`) applies Advances and offers events itself.
   - Users (grep): `driver_test.go`, `exits_test.go`, `oracle_test.go` and this file.
2. **Kind: K6.** The commit changes comments only, so the K6 proof shape fits; no assertion moves.
3. **Change.** Lines 3-4 (through "…not a mirror of any logic under test.") become:
   ```
   // The log-side test harness for internal/playback: a test composition that drives playback.Replay through
   // partition.Rule with its own router double — it applies each step's Advances and offers its event to the rules
   // itself — so the tests built on it check playback's steps and the trace, not the runtime's routing (the runtime's
   // router is replayed through NewPlanDriver in plandriver_test.go and equivalence_test.go). For each Step:
   ```
   Lines 5-15 stay as they are.
4. **Proof.**
   - `go run ./tools/astequal <old> <new>` on `internal/playback/helpers_log_test.go` exits 0.
   - The file has no `// Output:`.
   - Jargon sweep (§7.1 pattern) over the added lines (`git diff -U0 | grep '^+'`): no hit. The removed text had the sprint id `S3.2a-2`, `s3-2a-1` and `owner D1–D6`.
5. **Commit:** B-K6.
   - **Carried:** porting `driver_test`/`exits_test` to `NewPlanDriver` (non-simple) goes to the later part. The tests that really check the runtime's routing stay where they are.

### G2-94 — K2a

1. **Verdict: accepted.**
   - partition's `recoverBody` (`partition.go:381-386`) counts each body panic and resets the key. `onPurge` counts too (`:371-377`).
   - The `ld` double's `OnEvent` appends its log entry **before** `in.Op.(*ldState)` (`helpers_log_test.go:225-230`). So a panic leaves the log intact.
   - `step` (`:362-367`) only calls `Errorf` on CheckTimers and returns nil.
   - No test in driver/exits/oracle/trace panics on purpose (grep `panic(`: none).
2. **Kind: K2a.**
3. **Change.** In `step`, the final loop becomes:
   ```go
   	for _, r := range w.rules {
   		r.r.Advance(s.Tables)
   		if n := r.r.Stats().Panics; n != 0 {
   			return fmt.Errorf("line %d, rule %s: %d body panics, each a silent key reset: the double or a spec hook panicked", s.Line, r.spec.Name, n)
   		}
   		if err := r.r.CheckTimers(); err != nil {
   			return fmt.Errorf("line %d, rule %s: %v", s.Line, r.spec.Name, err)
   		}
   	}
   	return nil
   ```
   - Its only caller is `replay` → `ldRun`, which already Fatals on the error (`:385`). `playback.Replay` stops on fn's error (`driver.go:149-151`).
   - Returning CheckTimers' error is red with or without the change; it only stops the replay at the first fault, so no plant can separate old from new for that part.
4. **Proof.**
   - Plant: the walk hands the body no state. `internal/partition/partition.go`, find `in.Op = r.body.NewState(in)\n\t}\n\tin.LastActivity = e.Ts`, replace with `in.Op = nil\n\t}\n\tin.LastActivity = e.Ts`. The find occurs once: `onEvent` only, since `arrive` uses `max(…)`.
   - Test `TestDriverEmptyStreamNeverStalls` (no Sched, TTL 0, so only the two "event" entries are compared). Pkg `./internal/playback`. Expect old green, new red, plus a control.
   - **Control risk:** if the control reads red, a body panic already happens on real code. That is a real defect: stop and report.
5. **Commit:** B-K2. **Other owner:** `internal/partition/helpers_test.go` `world.advance` has the same gap (E2, W3); named for the coordinator.

### G2-95 — K2-del

1. **Verdict: accepted.**
   - `reader_refusals_internal_test.go:274-293` claims to guard `n > 0` at `reader.go:234`. `reader.go` has 261 lines.
   - `readLine` (`:193-206`) has no such operand.
   - `trimLineEnd` (`:229-237`) guards `n > 0` itself.
   - So no defensive block relies on the contract; the test checks only stdlib bufio.
   - I chose delete over re-tying. The repo's "defensive block guarded by its contract" convention has nothing left to justify here.
2. **Kind: K2-del.** A per-assertion plant is impossible: the test calls only `bufio`.
3. **Change:** delete the function and its doc comment. Drop `bufio` and `io` from the imports if they become unused.
4. **Proof (covered set).**
   - At `old_rev` and `new_rev`, run twice each: `go test ./internal/playback -count=1 -coverpkg=./... -coverprofile=<f>`.
   - Compare `awk 'NR>1 && $NF>0 {print $1}' <f> | sort -u`. Expected: equal across all four.
   - Supplementary: at `old_rev`, the covered set of `-run '^TestReadSliceEndsInItsDelimiterWhenItReturnsNoError$'` equals that of `-run '^$'` (init only).
   - If the package set differs between two runs of one revision (live-runtime nondeterminism), the supplementary check is the evidence and the record says so.
5. **Commit:** B-del.

### G2-96 — K2a (subtest)

1. **Verdict: accepted, and sharper than the survey stated.** `reader_test.go:120-123` exempts names starting with `payload` from "Decode set on a non-payload fault".
   - The only row this changes is "payload not an object" (`:99`, `decode: false`).
   - `[1]` passes the envelope: TObject accepts `[` (`event/decode.go:287-291`).
   - `scanner.document` then returns a syntax `*DecodeError` (`scanner.go:57-58`). `eventPayload` stores it in `Decode` (`records.go:388-393`).
   - So that row's Decode **is** set and the row's `false` is wrong; the exemption hides it.
2. **Kind: K2a.**
3. **Change.**
   - `:99` gets `decode: true`.
   - `:122` becomes `} else if le.Decode != nil {`.
4. **Proof.**
   ```json
   {"id":"G2-96","file":"internal/playback/records.go","find":"\t\terrors.As(err, &de)\n\t\treturn values, info, &LoadError{","replace":"\t\terrors.As(err, &de)\n\t\tif de != nil && de.Reason == event.DecodeSyntax {\n\t\t\tde = nil\n\t\t}\n\t\treturn values, info, &LoadError{","test":"TestReaderFaults","subtest":"payload not an object","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   ```
   Plus a control. The lane confirms the find occurs once in `records.go`.
5. **Commit:** B-K2.

### G2-97 — K2c

1. **Verdict: accepted.** `Drain` (`runtime/drain.go:25-46`) runs `awaitRuns`, `queueDrain`, `await`, `awaitMarkers` and then `awaitRecordingStop` on one ctx. So the 300 ms `dctx` (`s3_2b_2_build_test.go:117`) also bounds delivering 20 events.
2. **Kind: K2c.**
3. **Change.**
   - Add `all := make(chan struct{})`. In the sink: `outs++; if outs == 20 { close(all) }`, under `mu`.
   - After the send loop:
     ```go
     	// every alert out, waited for by state under ctx's 60 s cap: the 300 ms below then bounds only what it is about,
     	// how long the drain waits for the stalled recording
     	select {
     	case <-all:
     	case <-ctx.Done():
     		t.Fatalf("waiting for Keyed's 20 alerts: %v", ctx.Err())
     	}
     ```
   - The recording is already failed by then: the router's `put` fails synchronously (`runtime/recorder.go:99-108`) before the events' outputs.
4. **Proof.**
   - **Slowdown plant** (`--patch`, two hunks) on `internal/runtime/sink.go`:
     ```
     --- a/internal/runtime/sink.go
     +++ b/internal/runtime/sink.go
     @@ -3,4 +3,5 @@
      import (
      	"sync/atomic"
     +	"time"
      
      	"github.com/vorsar/cep/internal/event"
     @@ -58,3 +59,4 @@
      	}()
     +	time.Sleep(25 * time.Millisecond)
      	if err := q.sink(e); err != nil {
      		q.errors.Add(1)
     ```
     20 × 25 ms = 500 ms, which is past 300 ms.
   - Entry with `"patch"`, test `TestARecordingThatFallsBehindStops`, pkg `./internal/playback`, expect old **red** (Drain hits `dctx` in `awaitMarkers`), new **green**.
   - Load run: `python3.15 -I -B gate/bench/load.py -- -race -count=50 -run '^TestARecordingThatFallsBehindStops$' ./internal/playback` → 50/50.
5. **Commit:** B-K2.

### G2-98 — K2e (live data)

1. **Verdict: accepted.** `twTracked` (`s3_2b_2_corpus_test.go:197`) lists `partial-count`, but `check` is never called for it. `gate/tripwire.json` gives it `"ceiling": "none"`.
2. **Kind: K2e.** A plant in `tripwire.json` cannot go through mutate's overlay, because the test reads the file with `os.ReadFile`. So this is the live-data check.
3. **Change.**
   - Declare `measured := map[string]bool{}` before `check`.
   - First line of `check`: `measured[metric] = true`.
   - After the corpus loop:
     ```go
     	for _, m := range twTracked {
     		if c, ok := ceilings[m]; ok && c.Ceiling != "none" && !measured[m] {
     			t.Errorf("%s: a numeric ceiling and no measurement: the tripwire measures nothing for it (%s)", m, path)
     		}
     	}
     ```
4. **Proof.**
   - In a detached scratch worktree at each revision, set `gate/tripwire.json`'s `partial-count` to `{"ceiling": 0}`, then run `go test ./internal/playback -count=1 -run '^TestTripwire$'`. Old: PASS (red for the check). New: FAIL naming `partial-count` (green).
   - Unchanged real file: both PASS.
   - **Couplings:**
     - `PLANT_SWITCH` `const determinismPlant = false` (`:274`) is untouched and stays exactly once.
     - `test_tripwire_planted.py`'s plants still produce their asserted texts: the `outbox` 0 plant is measured; the popped `sink-depth` is absent from `ceilings`, so it is skipped.
     - The proofs row runs alone afterwards.
5. **Commit:** B-K2.

### G2-99 — K2a

1. **Verdict: accepted.** The doc (`s3_2b_2_fixround_test.go:33-35`) says the message names the last record queued; the test checks only `"queue full"`.
   - The production text is `runtime: recording queue full after record %d: …` with `q.queued` (`runtime/recorder.go:105-106`).
   - The header goes through `put` (`app.go:76`), and `records++` counts each write (`:214`). So once every queued record is written, `Records == queued`.
2. **Kind: K2a.**
3. **Change.** Add `|| !strings.Contains(rs.Err, fmt.Sprintf("queue full after record %d:", rs.Records))` to the condition at `:67`, with `the message naming record Records` in its text.
4. **Proof.**
   ```json
   {"id":"G2-99","file":"internal/runtime/recorder.go","find":"detection goes on\",\n\t\t\tq.queued))","replace":"detection goes on\",\n\t\t\tq.queued-1))","test":"TestARecordingThatOverflowsStaysReadable","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   ```
   Plus a control. Load run (goroutines): `load.py -- -race -count=50 -run '^TestARecordingThatOverflowsStaysReadable$' ./internal/playback`.
5. **Commit:** B-K2.

### G2-100 — K2e

1. **Verdict: accepted.** `close(w.gate)` (`:53`) and `r.Close` (`:61`) run only on the success path. Compare `s3_2b_2_build_test.go:106-111`.
2. **Kind: K2e** (structure). Old and new are both red on an early Fatal; only goleak's extra report differs.
3. **Change.** After `defer cancel()`:
   ```go
   	released, closed := false, false
   	defer func() { // a failure before the end still releases the writer, then closes the runtime: goleak sees no leak
   		if !released {
   			close(w.gate)
   		}
   		if !closed {
   			if err := r.Close(ctx); err != nil {
   				t.Errorf("Close: %v", err)
   			}
   		}
   	}()
   ```
   - `:53` becomes `close(w.gate); released = true`.
   - `:61-63` becomes `err = r.Close(ctx); closed = true; if err != nil { t.Fatal(err) }`.
4. **Proof.**
   - K2e check: `awk '/^func TestARecordingThatOverflowsStaysReadable/,/^}/' internal/playback/s3_2b_2_fixround_test.go | grep -c 'defer func()'`. Old: 0 (red). New: 1 (green).
   - Load run as for G2-99 (the same run covers both).
5. **Commit:** B-K2.

### G2-101 — carried in full to S1.1c-8

**Verdict: accepted.** GitNexus shows `Trace` has no late field or method.
- I chose not to delete it. It is the only test that writes one kind more than once: `stream_late` ×2 and `construct_late` ×3 (`trace_test.go:199-200`).
- So it guards something `TestTraceEntries` and `TestTraceOrder` do not.
- **Rename:** `TestTraceWritesOneEntryPerLateEvent`. "writes one entry per late event" ← counts of 2 `stream_late` and 3 `construct_late` lines after two and three writes.
- No commit and no proof row. G2-102 edits its index list, which is a body change.

### G2-102 — K2e

1. **Verdict: accepted.** `trace_test.go:144` and `:199` index `enEntries` by position.
2. **Kind: K2e** (fragile fixture).
3. **Change.** Add:
   ```go
   // enByName is the enEntries entry named name.
   func enByName(t testing.TB, name string) enEntry {
   	t.Helper()
   	for _, e := range enEntries {
   		if e.name == name {
   			return e
   		}
   	}
   	t.Fatalf("no enEntries entry %q", name)
   	return enEntry{}
   }
   ```
   - `:144`: `order := []string{"recording", "event", "stream_late", "construct_late", "table put", "timer", "note", "discard", "batch_end", "purge", "tick", "stop", "end"}`. The loop uses `e := enByName(t, name)`.
   - `:199`: `[]string{"recording", "stream_late", "construct_late", "event", "stream_late", "construct_late", "timer", "construct_late", "end"}`.
   - Both lists are the old index lists mapped through the current table (0 recording, 1 stream_late, 2 table put, 4 batch_end, 5 tick, 6 stop, 7 end, 8 event, 9 construct_late, 10 timer, 11 discard, 12 purge, 13 note).
4. **Proof.** K2e: `git grep -nE 'enEntries\[[a-z0-9]+\]' internal/playback/trace_test.go`. Old: 3 lines (`:148`, `:149`, `:200`), red. New: none, green. Both tests stay green.
5. **Commit:** B-K2.

### G2-103 — K2a

1. **Verdict: accepted.** `breakit2_test.go:80` uses `err.Error() != "EOF"`.
2. **Kind: K2a.**
3. **Change:** `if !errors.Is(err, io.EOF) || len(recs) != n {` and add the `io` import.
4. **Proof.**
   ```json
   {"id":"G2-103","file":"internal/playback/reader.go","find":"return Record{}, io.EOF","replace":"return Record{}, errors.New(\"EOF\")","test":"TestBreakitGuardReaderEdges","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   ```
   Plus a control. The find occurs once (`reader.go:169`).
5. **Commit:** B-K2.

### G2-104 — K2a

1. **Verdict: accepted.** `driver_test.go:139` and `:146` (and `:313`) assert only inside `Walk` callbacks. `clock.StreamRegistry.Walk` (`clock.go:209-217`) yields every declared stream.
2. **Kind: K2a.**
3. **Change.**
   - In `TestDriverStreamLateMovesNothing`, count the second Walk's calls (`n++`) and add `if len(before) != 2 || n != 2 { t.Fatalf("Walk yielded %d then %d streams; want A and B each time", len(before), n) }`.
   - In `TestRecordingArrivalStreamTsIsItsArrival`, count and require `n == 1`.
   - Doc at `:299`: "its recorded line carries…" → "a log line on a time-arrival stream carries \"ts\" and no \"arrival\"…".
4. **Proof.** Plant `internal/clock/clock.go`, find `	for i := range r.streams {\n\t\ts := &r.streams[i]\n\t\tfn("stream"`, replace with `	for i := range r.streams[:0] {\n\t\ts := &r.streams[i]\n\t\tfn("stream"`. Two entries, one per test, each with expect old green, new red, plus a control.
5. **Commit:** B-K2.

### G2-105 — K2e (bench input, its own commit, R4)

1. **Verdict: accepted.** Errors are discarded at `encode_bench_test.go:78`, `105`, `124` and `144`.
2. **Kind: K2e.** Benchmark names are unchanged (frozen). The checks sit outside `b.Loop()`, so the measured loop is unchanged.
3. **Change.** Before each `for b.Loop()`:
   - BenchmarkAppendValue: `if _, err := AppendValue(buf[:0], c.v); err != nil { b.Fatalf("AppendValue(%s): %v", c.name, err) }`.
   - BenchmarkAppendEvent: the same with `appendEvent(buf[:0], s, e, "")`.
   - BenchmarkRecorderTable: `if err := r.Table(0, c.op, rows); err != nil { b.Fatalf(...) }`.
   - BenchmarkRecordOneEvent: `if err := r.Event(&e, hpT0, c.payload); err != nil { b.Fatalf(...) }`.
4. **Proof.** K2e: a one-iteration run under an overlay plant, one per benchmark: `go test ./internal/playback -count=1 -run '^$' -bench '^<Name>$' -benchtime=1x -overlay=<o.json>`. Old: exit 0 (red: the drift is silent). New: exit 1 with the Fatalf (green).
   - **Plants:**
     - `recorder.go` `if table < 0 || table >= len(r.app.Tables) {` → `if table >= 0 || table >= len(r.app.Tables) {` (RecorderTable).
     - `recorder.go` `if e.StreamID < 0 || e.StreamID >= len(r.app.Streams) {` → `if e.StreamID >= 0 || …` (RecordOneEvent).
     - `encode.go` `if len(e.Values) != n {` → `if len(e.Values) == n {` (AppendEvent; occurs once).
     - `encode.go` `\treturn append(dst, "null"...), nil` → `\treturn dst, fmt.Errorf("planted")` (AppendValue/null; occurs once).
   - Real code: each of the four runs with `-benchtime=1x` exits 0.
5. **Commit:** B-bench.

### G2-106 — K2e

1. **Verdict: accepted.**
   - Of `encode_test.go`'s 18 tests, none calls `t.Parallel`. `TestAppendEventZeroAllocs` (`:323`) uses `AllocsPerRun`; `go doc` says it sets GOMAXPROCS to 1 during measurement.
   - `TestBaseline` (`baseline_test.go:55`) has no shared state: it reads the corpus, and `-baseline-write` writes to its own directory. It is pinned `-run ^TestBaseline$`, where it runs alone anyway.
2. **Kind: K2e.**
3. **Change.**
   - `t.Parallel()` is the first statement of the 17 other encode tests and of TestBaseline.
   - On TestAppendEventZeroAllocs's doc, add the line: `It runs serially: testing.AllocsPerRun sets GOMAXPROCS to 1 and must not run in a parallel test.`
4. **Proof.** K2e check:
   ```
   python3.15 -I -c "import re,sys
   for f in sys.argv[1:]:
    s=open(f).read()
    for m in re.finditer(r'((?://[^\n]*\n)*)func (Test\w+)\(t \*testing\.T\) \{\n\t([^\n]*)',s):
     if m.group(3)!='t.Parallel()' and 'runs serially' not in m.group(1): print(f,m.group(2))" internal/playback/encode_test.go internal/playback/baseline_test.go
   ```
   - Old: prints 19 names (red). New: prints nothing (green).
   - Load run: `load.py -- -race -count=50 -run '^(TestAppendValue|TestAppendEvent|TestBaseline)' ./internal/playback` → 50/50.
5. **Commit:** B-K2. **Fixed in part:** `internal/partition/ttl_test.go:122` and `panic_test.go:123,173,204` are E2's files; routed to the coordinator.

### G2-107 — K2a (risk named)

1. **Verdict: accepted.** The header (`equivalence_test.go:6`) lists construct-late among the compared counts, and `:348` compares them. But the coverage guard (`:398`) never requires any construct-late drop.
   - Plain rules are on-arrival, so they are never construct-late (`partition.go:332-334`). Only the buffered `Double` (`:114`) can be.
2. **Kind: K2a.**
3. **Change.**
   - `:390-395`: add `total.ConstructLate += rs.ConstructLate`.
   - `:398`: add `|| total.ConstructLate == 0`.
4. **Proof.** Plant `internal/partition/partition.go`, find `r.stats.ConstructLate++`, replace with `r.stats.ConstructLate += 0`. Live and replay read the same counter (`stats.go:67`, `sync.go:147`), so they stay equal. Test `TestLiveEqualsReplay`; expect old green, new red, plus a control.
   - **Risk:** if the control reads red, the 200-seed generator gives no construct-late at all. The lane then stops and reports; the fix becomes either a generator change (later part) or narrowing the header's claim (choice for the coordinator).
   - Load run as for G2-92.
5. **Commit:** B-K2. **Carried** to the later part (§7.2's test-internal-refactor rule): eqCompare returning the Sync result, and the stream id kept in eqResult instead of `k[1]-'0'`.

### G2-108 — K2a

1. **Verdict: accepted.** `helpers_s3_2b_2_test.go:119` has `_ = r.Close(cctx)`. No q2Start user closes `live.r` itself (grep).
2. **Kind: K2a.**
3. **Change:** `if err := r.Close(cctx); err != nil { t.Errorf("Close: %v", err) }`.
4. **Proof.**
   ```json
   {"id":"G2-108","file":"internal/runtime/shutdown.go","find":"\treturn r.awaitGoroutines(ctx, first, failed)","replace":"\tif err := r.awaitGoroutines(ctx, first, failed); err != nil {\n\t\treturn err\n\t}\n\treturn ErrClosed","test":"TestRecordingStampsNeverFallBelowTheStart","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   ```
   Plus a control. The find occurs once (`shutdown.go:63`).
   - Load run: the full package under `-race` at hand-back. All 24 q2Start users now report Close; a red there is a real defect (stop and report).
5. **Commit:** B-K2.

### G2-109 — K2e

1. **Verdict: accepted.** Unguarded indexing at:
   - `ingest_test.go:321`, in `TestLiveOrderPerKeyEqualsReplay`;
   - `plandriver_test.go:222`, in `TestPlanDriverErrorIsPerRule`;
   - `s3_2b_2_corpus_test.go:256` (TestTripwire);
   - `s3_2b_2_fixround_test.go:66`.
2. **Kind: K2e.**
3. **Change.**
   - `:321` → `q2StreamStats(t, st, 0).Late != 1 || q2StreamStats(t, st, 1).Late != 0`.
   - `:222` → `q2RuleStats(t, st, 0).Errored` and `q2RuleStats(t, st, 1).Errored`, in the condition and the message.
   - Corpus: before `b := 0`, add `if len(st.SinkHigh) == 0 { t.Fatalf("Stats.SinkHigh %v has no app 0", st.SinkHigh) }`. In the loop, add `if b >= len(st.SinkHigh[0]) { t.Fatalf("Stats.SinkHigh[0] has %d sinks; derived stream %s has none", len(st.SinkHigh[0]), s.Schema.Stream()) }`.
   - Fixround: before `rs :=`, add `if len(st.Recording) != 1 { t.Fatalf("Stats.Recording %+v; want one recording", st.Recording) }`.
4. **Proof.** K2e:
   - `git grep -nE 'st\.(Streams|Rules)\[0\]\[[0-9]+\]' -- internal/playback/ingest_test.go internal/playback/plandriver_test.go`. Old: 2 lines (red). New: 0.
   - `git grep -c -e 'has no app 0' -e 'want one recording' -- internal/playback/s3_2b_2_corpus_test.go internal/playback/s3_2b_2_fixround_test.go`. Old: 0 (red). New: 1 each (green).
   - The TestTripwire edit is body only; `PLANT_SWITCH` is untouched.
5. **Commit:** B-K2.

### G2-110 — K2e

1. **Verdict: accepted.** `reader_test.go:243` has `d2, _ := playback.NewDriver(app)`.
2. **Kind: K2e.**
3. **Change:** `d2, err := playback.NewDriver(app); if err != nil { t.Fatalf("NewDriver: %v", err) }`, using the in-scope `err`.
4. **Proof.** K2e: `git grep -n ', _ := playback.NewDriver' internal/playback/reader_test.go`. Old: 1 line (red). New: 0.
5. **Commit:** B-K2.
   - **Rename** → S1.1c-8: the proposed `failingDriver` misdescribes it. The driver does not fail; fn's error stops Replay. A suggested name is `stoppedDriver` ("stopped" ← `errors.Is(rerr, stop)` after lines `[1 2]`). Local variables are outside the naming tool's scope, so this is recorded only.

---

### Commit plan (lane B; lane line 2, ceiling 4)

1. **B-K1** `fix(playback)`: G10-18, G10-19.
   - Proofs: the G10-18 and G10-19 revert entries (red), plus G10-19's chain plant (red).
   - Not K1h: both are error-branch text on the replay or Deploy path, named here.
2. **B-K2** `test(playback)`: G2-89, -90, -91, -92, -94, -96, -97, -98, -99, -100, -102, -103, -104, -106 (playback part), -107, -108, -109, -110. That is 18 findings.
   - I do not take the optional split by file, to stay inside the ceiling.
   - Proofs: the `--expect` batch `docs/sprints/s1-1c-7-plants/B.json` (every K2a entry plus controls); the K2c patch and load runs; the K2e commands.
   - Load runs:
     - `TestLiveEqualsReplay`
     - `TestARecordingThatFallsBehindStops`
     - `TestARecordingThatOverflowsStaysReadable`
     - the encode/baseline set
   - **Order inside the lane:** run the G2-92 and G2-107 controls and plants first, because of their stop rules.
3. **B-K6** `docs(playback tests)`: G2-93. Proof: astequal plus the sweep.
4. **B-del** `test(playback)`: G2-95. Proof: the covered set. It is counted in the shared K2-del pool.
5. **B-bench** `test(playback)`: G2-105. Proof: the four one-iteration overlay runs (R4). It is counted in the bench-input pool.

Lane commits: 3 of a ceiling of 4. Pool commits: 2. **Chunk total: 5.**

### Choices made

1. **G10-18 user text.**
   - Input: a record holding a key its kind does not allow, for example `{"table":"T",…,"stream":"S"}`.
   - Candidates: "has no", "takes no", "cannot have".
   - Chosen: **"a table record takes no \"stream\" key"**. It is short, true, and keeps the sentence shape.
   - Dry run: `reader_refusals_internal_test.go:306` and `:308` (changed in the commit); `docs/sprints/s1-1c-6-design.md:1095` is Historical and left alone.
2. **G10-19 user text and behaviour.**
   - Input: a sandbox `NewPlanDriver` refuses.
   - Candidates: (a), (b), (c) above.
   - Chosen: **(c)**, new text `playback: rule R, expect block 1: stream 0 has no schema`, chain kept.
   - Dry run: `expect_engine_faults_test.go:34` only.
3. **G2-90.** No choice is made: SPEC rev 4 decides E-PART-004. Listed so the reviewers can check that the plan's "undecided" label is answered by the SPEC.
4. **G2-95.** Delete rather than re-tie, for the reason in its entry.
5. **G2-101.** Keep and carry the rename rather than delete, for the reason in its entry.
6. **G2-106.** TestBaseline gets `t.Parallel()` rather than a reason, because nothing it does conflicts with other tests.

### Couplings and risks

- **§0 item 5.**
  - **CORPUS_TEST / PLANT_SWITCH:** G2-98 and G2-109 edit TestTripwire's body only. `const determinismPlant = false` stays exactly once. `test_tripwire_planted.py`'s two plants keep their asserted texts. The registry pin `^TestTripwire$` is unchanged.
  - **BASELINE_TEST:** G2-89 and G2-106 edit the body and add one test. `blWantBlocks = 15` stays exactly once. `^TestBaseline$` is unchanged.
  - The proofs row runs alone at T4 after this lane (coupled files).
- **R5 per-event code.** The only production edits are G10-18 (`recordKind`, replay per line; text-only exemption) and G10-19 (`runBlock`, Deploy-only error branch). Every plant touches per-event code (partition, runtime sink/recorder, reader), but plants are never committed. Nothing is K1h.
- **R4.** G2-105 is in its own commit with names unchanged. No other edit touches a benchmark.
- **§0 item 4.** No frame tables or line-anchored mutants are in this chunk's files.
- **Other owners' files (not planned; for the coordinator):**
  - `internal/model/model.go:199` comment (CD; G2-89);
  - `internal/partition/{ttl_test,panic_test}.go` (E2; G2-106);
  - `internal/partition/helpers_test.go` `world.advance` (E2; G2-94's twin).
- **Risks named:**
  - G2-92: the old outcome may be red. Stop rule.
  - G2-107: the control may be red. Stop rule.
  - G2-94 and G2-108: a red control means a real defect. Stop and report.
  - G2-91: the plant depends on `NewSchema` refusing duplicate names; the fallback plant is named.
  - G2-97: the residual drain round-trip under its 300 ms is small and accepted, since the deadline is the behaviour.
  - **G10-19's first-written chain plant is wrong;** use the corrected `%v` plant.

### Carried items

- **Renames → S1.1c-8 (recorded, not made):**

  | Id | New name | Assertion behind the verb |
  |---|---|---|
  | G2-90 | `TestAPartitionClauseNamingTwoKeysOfOneStreamIsRefused` | no app and one E-PART-004 |
  | G2-91 | `TestTheEnvelopeSchemasAreValid` | `envelopeSchemas()` returns a nil error |
  | G2-101 | `TestTraceWritesOneEntryPerLateEvent` (the finding's only fix) | 2 and 3 lines of each late kind |
  | G2-110 | local `d2` → `stoppedDriver` (the survey proposed `failingDriver`) | — |

- **Later part:**
  - G2-90: the move to `internal/validate` and the merge with `fixround_test.go:23`.
  - G2-93: porting `driver_test`/`exits_test` to `NewPlanDriver`.
  - G2-107: eqCompare returning the Sync result, and the stream id in eqResult.


## Revision 1, after the blind reviews, with the pre-check's corrections

## Chunk B design note, revision 1: replacement sections

I made all four requested changes; no other finding's verdict or kind changed. One thing to know before you read on: GitNexus rates `q2Start` **HIGH** risk (25 symbols upstream), and my G2-108 change touches it, so I flag that under G2-108 below.

The checks I ran for this revision were read-only:
- `git grep` for `Not parallel:` found 12 files.
- I read `cnCheckFaults` at `reader_refusals_internal_test.go:85-96`.
- I confirmed `reader_test.go:96`.
- The toolchain's `testing/allocs.go:21-22` panics with "AllocsPerRun called during parallel test".
- I read W0's `PLAIN_KEYS` and `read_batch` (design-W0 A.1 and A.2) and CD1's G2-73 §4 script.
- One GitNexus call: `impact q2Start` upstream with tests, cross-checked by grep.

### Change list

1. **DA-S5 / DQ-S1.** The G10-18 and G10-19 K1 proofs move to a plain batch, `docs/sprints/s1-1c-7-plants/B-K1.json`. It runs on the B-K1 commit, every entry expects red, and the failing lines are quoted.
   - The withdrawn chain plant (`Unwrap` returning nil) is deleted.
   - The corrected chain plant (`%w` → `%v`) is the batch's third entry.
   - The "first-written chain plant is wrong" risk line in Couplings is deleted.
2. **DA-S6.** G2-106 now uses the repo's marker `Not parallel: <reason>`, with a reason checked against the toolchain source.
   - Its check is CD1's G2-73 §4 script, the one shared check, run on B's 19 `file:Test` pairs.
   - My regex check is withdrawn.
3. **DQ-S5.** G2-108 gains a 50/50 load run over the 22 top-level tests that reach `q2Start`, and keeps the stop rule.
   - The review counted 24. GitNexus finds 21 direct callers: 18 tests and 3 helpers. The helpers add 4 tests at depth 2, so there are 22 tests. 24 is likely the count of call sites.
4. **DQ-L5.** No B batch entry needs more than 60 s.
   - The playback package ran in about 2.5 s at S3.2b-1 (`docs/sprints/s3-2b-1-record.md:48`).
   - The two `TestLiveEqualsReplay` entries (G2-92, G2-107) are the slowest and unmeasured, so they carry `"timeout": "300s"` as margin.
   - Every `load.py` run now names go test's `-timeout`. `-count=50` runs in one test binary, so go test's default 10-minute limit would otherwise cap the whole run; `50m` stays inside load.py's 3600 s wall.
5. **Two corrections the review raised.**
   - G2-96's row is `reader_test.go:96`, not `:99`.
   - In the K2 `--expect` entries, `"<lane base>"` means the B-K2 commit's parent, which is B-K1. Production files do not change in B-K2, so every K2 find text exists at both revisions.

---

### G10-18, item 4 (replaces it)

The proof is a plain K1 batch entry, run from a checkout of the B-K1 commit (tracked files equal HEAD). It expects **red**.

```json
{"id":"G10-18","file":"internal/playback/records.go","find":"\"a %s record takes no %q key\"","replace":"\"a %s record has no %q key\"","test":"TestReaderNamesTheFirstFaultOfALine","subtest":"a table with a stream key names the key","pkg":"./internal/playback","note":"revert plant: the old text"}
```

- **Failing line:** `reader_refusals_internal_test.go:92`, `t.Errorf("fault = line %d %q; want line %d %q", …)`.
- **Expected output:** `fault = line 1 "a table record has no \"stream\" key"; want line 1 "a table record takes no \"stream\" key"`, under `--- FAIL: TestReaderNamesTheFirstFaultOfALine/a_table_with_a_stream_key_names_the_key (`.
- **Single-run form of the same proof:**
  ```
  python3.15 -I -B gate/bench/mutate.py --file internal/playback/records.go --find '"a %s record takes no %q key"' --replace '"a %s record has no %q key"' --test TestReaderNamesTheFirstFaultOfALine --subtest 'a table with a stream key names the key' --pkg ./internal/playback
  ```
  → red

### G10-19, item 4 (replaces it, including both earlier chain plants)

These are two plain K1 batch entries in `B-K1.json`, run on the B-K1 commit. Each expects **red**.

```json
{"id":"G10-19","file":"internal/playback/expect.go","find":"fault(\"%w\", inPlayback{err})","replace":"fault(\"%w\", err)","test":"TestRunExpectsReportsASandboxThatCannotBeBuilt","pkg":"./internal/playback","note":"revert plant: the doubled prefix"}
{"id":"G10-19-chain","file":"internal/playback/expect.go","find":"fault(\"%w\", inPlayback{err})","replace":"fault(\"%v\", inPlayback{err})","test":"TestASandboxFaultKeepsTheDriversErrorInTheChain","pkg":"./internal/playback","note":"the chain plant: same text, no chain"}
```

**Entry G10-19 (the revert plant).**
- The plant keeps `inPlayback`, which the internal test needs. An unused type still compiles.
- **Failing line:** `expect_engine_faults_test.go:36`, `t.Errorf("RunExpects = %v, %v; want nil, %q", …)`.
- **Expected output:** `RunExpects = [], playback: rule R, expect block 1: playback: stream 0 has no schema; want nil, "playback: rule R, expect block 1: stream 0 has no schema"`.

**Entry G10-19-chain (the chain plant).**
- `%v` gives the same text, but `errors.As` no longer finds an `inPlayback`.
- **Failing line:** the new test's `t.Fatalf("runBlock = %v; want the driver's own error in the chain", err)`.
- **Expected output:** `runBlock = playback: rule R, expect block 1: stream 0 has no schema; want the driver's own error in the chain`.

**Running both entries.**
- Each find text occurs exactly once in `expect.go` at B-K1. The two entries are separate mutations of the same line.
- **Batch command:** `python3.15 -I -B gate/bench/mutate.py --batch docs/sprints/s1-1c-7-plants/B-K1.json`. It exits 0 only when all three entries read red (G10-18's and these two).

### Commit plan, line 1 (replaces it)

1. **B-K1** `fix(playback)`: G10-18 and G10-19.
   - **Proof:** the plain batch `B-K1.json` (three entries, each red, failing lines as quoted above), run from a checkout of B-K1.
   - Not K1h: both changes are text on an error branch, on the replay and Deploy paths, as named in the note.

### G2-106, items 3 and 4 (replace them)

**3. Change.**
- `t.Parallel()` becomes the first statement of the 17 other encode tests and of `TestBaseline`.
- `TestAppendEventZeroAllocs`'s doc comment gets this last line, using the repo's marker and keeping the GOMAXPROCS reason:
  ```
  // Not parallel: testing.AllocsPerRun sets GOMAXPROCS to 1 while it measures and panics if a parallel test is running.
  ```
  - The toolchain confirms both halves: `testing/allocs.go:21-22`, `panic("testing: AllocsPerRun called during parallel test")`, and the go doc's GOMAXPROCS sentence.
  - The same package already uses this marker at `encode_refusals_internal_test.go:72` and `hot_path_allocs_internal_test.go:9`.

**4. Proof (K2e).** Run CD1's G2-73 §4 script, the one shared check; the coordinator commits its text beside `CD.json`. Run it on B's 19 pairs:

```
python3.15 -I <CD1's G2-73 script> internal/playback/encode_test.go:TestAppendValueStringOracle internal/playback/encode_test.go:TestAppendValueStringTable internal/playback/encode_test.go:TestAppendValueScalars internal/playback/encode_test.go:TestAppendValueFloats internal/playback/encode_test.go:TestAppendValueFloatNonFinite internal/playback/encode_test.go:TestAppendValueTimestamps internal/playback/encode_test.go:TestAppendValueTimestampOutOfRange internal/playback/encode_test.go:TestAppendValueObject internal/playback/encode_test.go:TestAppendValueNulls internal/playback/encode_test.go:TestAppendValueAppends internal/playback/encode_test.go:TestAppendEventSortedKeys internal/playback/encode_test.go:TestAppendEventAllNull internal/playback/encode_test.go:TestAppendEventOptionalKeys internal/playback/encode_test.go:TestAppendEventErrors internal/playback/encode_test.go:TestAppendEventEscapedNames internal/playback/encode_test.go:TestAppendEventAppends internal/playback/encode_test.go:TestAppendEventZeroAllocs internal/playback/encode_test.go:TestAppendEventRoundTrip internal/playback/baseline_test.go:TestBaseline
```

- **Old:** `19 without t.Parallel or a reason`, exit 1 (red).
- **New:** `0 …`, exit 0 (green).
- The script matches `func <name>(` with the parenthesis included, so `TestAppendEvent…` names cannot match one another.
- **Load run:**
  ```
  python3.15 -I -B gate/bench/load.py -- -race -count=50 -timeout 50m -run '^(TestAppendValue|TestAppendEvent|TestBaseline)' ./internal/playback
  ```
  → every test 50/50.

### G2-108 (replaces the whole section)

1. **Verdict: accepted.**
   - `helpers_s3_2b_2_test.go:119` discards Close's error with `_ = r.Close(cctx)`.
   - No `q2Start` user closes `live.r` itself (grep).
   - `Runtime.Close` returns a ctx error or the removals that failed (`runtime/shutdown.go:54-63`).
   - **GitNexus** `impact q2Start` upstream, with tests, rates it **HIGH** risk. Its 25 symbols are 21 direct callers (18 tests plus the helpers `qeFlagRun`, `mkLiveRecording` and `mkLiveRecords`) and 4 tests reached through those helpers. That makes **22 top-level tests**, confirmed by grep. All 22 change with this fix, which is why the load run below covers every one of them.
2. **Kind: K2a.**
3. **Change:** `if err := r.Close(cctx); err != nil { t.Errorf("Close: %v", err) }`.
4. **Proof.**
   ```json
   {"id":"G2-108","file":"internal/runtime/shutdown.go","find":"\treturn r.awaitGoroutines(ctx, first, failed)","replace":"\tif err := r.awaitGoroutines(ctx, first, failed); err != nil {\n\t\treturn err\n\t}\n\treturn ErrClosed","test":"TestRecordingStampsNeverFallBelowTheStart","pkg":"./internal/playback","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
   ```
   - Add a control entry (green/green). The find text occurs once (`shutdown.go:63`).
   - **Load run** (the tests start goroutines, plan §7.2), over all 22 tests:
     ```
     python3.15 -I -B gate/bench/load.py -- -race -count=50 -timeout 50m -run '^(TestBreakitConcurrentPayloadsReplay|TestBreakitDepthLimitsRoundTrip|TestBreakitInvisiblesSlotAsKeyPartReplays|TestBreakitMalformedUTF8PayloadStaysRecordable|TestBreakitRecordsCountsEveryLine|TestBreakitSendBatchIDBeyondTheLine|TestBreakitTableWriteBeyondTheLineIsRefused|TestBreakitTickBeyondYear9999StaysReadable|TestRecordingStampsNeverFallBelowTheStart|TestErrorSemanticsTable|TestSendPayloadRefusesAWrongTarget|TestIngestFlagDeclaredStream|TestIngestFlagUndeclaredStream|TestMultiPartKeysLiveEqualsReplay|TestMultiPartKeysWorkedExamples|TestArrivalRecordingReplaysByteForByte|TestArrivalStampsLiveEventsInArrivalOrder|TestRecorderCallOrder|TestRecorderFailureLeavesDetectionRunning|TestRecorderReplayGivesTheLiveRun|TestBreakitDurationAndTimestampOverflowDropsTheEventForThatRuleOnly|TestOverflowDropsTheEventForThatRuleOnly)$' ./internal/playback
     ```
     → every line `<Test>: 50/50 under load`, exit 0.
   - **Stop rule (kept):** a control that reads red, or any k < 50 in this run, means Close really failed in that test. The lane does not retry or weaken the check; it stops and reports to the coordinator.
5. **Commit:** B-K2.

### Timeouts (DQ-L5): replacement lines

- **G2-92 and G2-107:** each `TestLiveEqualsReplay` entry, and its control, gains `"timeout": "300s"` (W0's per-entry key). Every other B entry runs well under 60 s, so it has no key and uses mutate.py's default.
- **G2-92 / G2-107 load run** (replaces it):
  ```
  python3.15 -I -B gate/bench/load.py -- -race -count=50 -timeout 50m -run '^TestLiveEqualsReplay$' ./internal/playback
  ```
- **G2-97 load run** (replaces it):
  ```
  python3.15 -I -B gate/bench/load.py -- -race -count=50 -timeout 50m -run '^TestARecordingThatFallsBehindStops$' ./internal/playback
  ```
- **G2-99 / G2-100 load run** (replaces it):
  ```
  python3.15 -I -B gate/bench/load.py -- -race -count=50 -timeout 50m -run '^TestARecordingThatOverflowsStaysReadable$' ./internal/playback
  ```

### G2-96, item 3 (correction only)

- `reader_test.go:96` ("payload not an object") gets `decode: true`.
- `:122` becomes `} else if le.Decode != nil {`.
- Everything else in the section stands.

---

### Coordinator corrections after the pre-check (2026-11-20; evidence: precheck-1.md)

1. **G2-92: the plant is replaced.** The note's build.go plant never runs for the live runtime: `runtime.Start` → `New` → `build(cfg, nil)` (lifecycle.go:24), so build.go:30 runs only through `NewSync`. The live read at Start is `router.add` (transitions.go:40). Use instead:
   - file `internal/runtime/transitions.go`;
   - find `⧵tstart := rt.now(r.cfg.Now())⧵n` (⧵ = backslash; one tab before `start`, a newline after);
   - replace `⧵tstart := rt.now(rt.stamp)⧵n`;
   - test `TestLiveEqualsReplay`, pkg `internal/playback`.

   Pre-checked: old **green**. With the note's eqLive `len(stamps) != 0` check it is **red** ("live: Config.Now read 1 times short of once at Start and once per event (design §7.8)", seed 0, 1 shard). The stop rule does not fire.
2. **G2-107: the stop rule fired.** With the note's guard change, the real code fails: ConstructLate is 0 over all 200 seeds (`equivalence_test.go:404`). **Coordinator's choice** (a test-internal matter, plan §7.2):
   - commit no guard change;
   - narrow the header's claim instead (equivalence_test.go:6 and the compared-counts list), so it no longer says construct-late drops are exercised, only that the count is compared;
   - carry the generator change (making the 200 seeds produce construct-late through the buffered `Double`) to S1.1c-10;
   - ledger outcome "fixed in part (generator → S1.1c-10)".

   The header edit rides B-K2 as a comment-only change in a test file.
3. **G2-94 and G2-108:** pre-checked. Real code green with the note's changes; red under the plants, with the expected messages. The stop rules do not fire.
