# S1.1c-7 design note — E2 (root package, partition, io/inmemory, tools)

*Built in W3 (2026-11-25 to 2026-11-26). Where the code differs from this note, the sprint record's W3 section says how and why ([record](s1-1c-7-record.md)).*

*S1.1c-7's design note for chunk E2 (root package, partition, io/inmemory, tools), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes.*


## The note

## S1.1c-7 design note: chunk E2 (the root package, internal/partition, io/inmemory, tools/apipartition, tools/shape)

### 1. Header

- **Chunk:** E2. **Findings:** 20 from the chunk file, plus G2-106's partition part, which the coordinator added. That makes 21 rows.
- **Base:** I read the tree at HEAD 1f4ebba. `git diff --stat d521725 37df879 -- '*.go'` is empty, so production code matches d521725, the commit the index was built at. I could not read the `gitnexus://repo/cep/context` resource because the tool is not available in this session. Every "no other caller" claim below was also checked with `git grep`.
- **Files read** (the cited ranges):
  - Root package: apphandle.go (all); deploy.go:17-27, :150-240; connectors.go (all); options.go:8-12, :60-95; diagnostic.go:15-30; row.go:36-100, :119-134; sink.go:12-45; cep.go:1-65; input.go:28-46; table.go:10-13, :82-84; stats.go:4-6; example_test.go:1-40; boundary_test.go:15-90, :170-235; deploy_test.go:50-75, :240-250, :280-318; table_test.go:20-40.
  - Read only for the G10-1 seam: internal/runtime lifecycle.go:46-170, requests.go:30-175, drain.go:1-80, transitions.go:1-110, router_in.go:1-115, recorder.go:60-115, messages.go, runtime.go:39-78.
  - internal/partition: partition.go:218-242, :348-385; panic_test.go:115-205; rule_test.go:203-240; ttl_test.go:119-122; objectkey_test.go; helpers_test.go (package-level vars only).
  - io/inmemory: source.go:15-120; breakit_s3_3_1_test.go:20-101; inmemory_test.go:155-175; coverage_s3_3_1_test.go:72-90.
  - tools/apipartition: main.go:322-356; main_test.go:45-188; breakit_s3_3_1_test.go:28-36.
  - tools/shape: main.go:50-66; the test function list in shape_test.go.
- **GitNexus calls: 5.**
  - `impact` upstream on `AppHandle.Start`: MEDIUM, exact. 6 callers at d=1, all in tests (`fcStart`, `imApp`, `Example_embed`, `TestLifecycleTable`, `TestCvConnectorFactory`, `TestBreakitWithTableRowsBadValueFailsTheDeploy`), 48 at d=2, no process.
  - `context` on `AppHandle.Start`: its outgoing calls list only `mapErr`, `newInputHandler` and `emitter`. The `a.rt.Activate`, `TableWrite` and `Run` calls on the runtime are not linked, so I confirmed them by reading the code.
  - `impact` upstream on `notBuilt`: LOW, exact, 2 direct callers (`defaultFactory.Source` and `.Sink`).
  - `impact` upstream on `partition.NewRule` (production only): LOW, exact, d=1 is `shard.install`. It sits in the processes of `shard.run` (internal/runtime/shard.go) and `runBlock` (playback/expect.go).
  - `context` on `router.activate` (internal/runtime/transitions.go:77-91): its only caller is `ingestLifecycle`.
- **What I could not check, or did not finish (time box):**
  - I ran nothing, so every expected outcome below is reasoned from the code.
  - **Not build-ready; each needs a short completion pass before G1:**
    - G2-83 and G2-84: the plants' exact text was not read (keyOf, CheckTimers' messages).
    - G2-143: the proof's form. The design is given.
    - G2-145: the plant's write site in main.go.
    - G2-146: the surface line's exact text.
    - G2-156: the four lowering inputs.
  - G2-2's runtime-side rows depend on lane E1's G7a-5 rewording of internal/runtime/boundary.go. I assume `attribute %q` survives it. The control entry checks this at E2's lane base.

### 2. Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-1 | accepted | K1 (facade); the runtime half is K1h, lane E1 | E2-1 root K1 | runtime seam → E1, to settle before W2 | starting rows travel in `runtime.App.Loads` and are applied in the activation step; Start loses its TableWrite loop |
| G10-23 | accepted | – | – | carried in full to S1.1c-10 | `print`'s error path is unreachable without a seam (bytes.Buffer, AST nodes only), so no red test exists today |
| G2-1 | accepted | K2a | E2-2 root K2 | – | count the sink's writes; exactly 1 |
| G2-2 | accepted | K2a | E2-2 root K2 | – | each `want` becomes the quoted name (`"\"s\""` …), `"99"` becomes `"index 99"` |
| G2-3 | accepted | K2a | E2-2 root K2 | – | assert `": 2 finding(s):"` via `fmt.Sprintf(": %d finding(s):", len(nonLint))` |
| G2-4 | accepted | K2a | E2-2 root K2 | – | `"\"k\""` at table_test.go:35 and deploy_test.go:315 |
| G2-81 | accepted (a choice: it counts) | K2a | E2-5 partition K2 | – | assert `Stats().Purged == 1`; the doc's "gap Q12" sentence replaced |
| G2-82 | accepted in part | K2a | E2-5 partition K2 | rename → S1.1c-8 | add the "nil key types" and "empty key types" rows |
| G2-83 | accepted | K2a | E2-5 partition K2 | – | assert the births order; add the wrong-type case to the doc *(plant text to complete)* |
| G2-84 | accepted | K2a | E2-5 partition K2 | – | a distinguishing fragment per fault *(fragments to complete)* |
| G2-106 (partition part) | accepted | K2e | E2-5 partition K2 | playback part → lane B | `t.Parallel()` in three panic tests; a "Runs serially:" reason on the allocation test |
| G2-141 | accepted in part | K2a | E2-6 io/inmemory K2 | rename → S1.1c-8 | assert `errors.Is(err, inmemory.ErrReopened)` on the second Open |
| G2-142 | accepted | – | – | carried in full to S1.1c-8 | rename to `TestEndBatchIsCountedByTheStream`, doc to match |
| G2-143 | accepted | K2b | E2-6 io/inmemory K2 | – | wait for Push to park before `cancelShort()` *(proof form to confirm)* |
| G2-144 | accepted | K2a | E2-7 apipartition K2 | – | new test: an embedded interface is refused |
| G2-145 | accepted | K2a | E2-7 apipartition K2 | – | read the golden and compare with "" *(plant site to complete)* |
| G2-146 | accepted | K2a | E2-7 apipartition K2 | – | assert B's full surface line with its iota marker *(text to complete)* |
| G2-156 | accepted | K2a | E2-8 shape K2 | – | one unit row per `parser.LoweringCodes` code *(inputs to complete)* |
| G7a-1 | accepted | K1 (error text only, not per-event) | E2-1 root K1 | – | "is not available in this release; bind a Go sink with WithSink"; `connectorSprint` deleted |
| G7a-2 | accepted, widened to the three public packages' rendered godoc | K6 | E2-3 public-godoc K6 | – | the jargon sweep over rendered godoc in `.`, `./io`, `./io/inmemory` and the Example's doc |
| G7a-4 | accepted | K1 (error-text exemption, named) | E2-4 partition K1 | – | `"partition: key types %v: a part of these types cannot key a partition"` |

**Counts:**
- 16 fixed or fixed in part.
- 2 carried in full to S1.1c-8: none of those two; see the next line.
- Carried in full: G2-142 → S1.1c-8; G10-23 → S1.1c-10.
- 6 findings have proof material still to complete: G2-83, -84, -143, -145, -146, -156.

### 3. Findings

#### G10-1: `AppHandle.Start` leaves an app half-started (apphandle.go:43-64)

**1. Verdict: accepted.**

The code:
- :44 `a.rt.Activate(ctx, a.id)`.
- :47-51, for each table: `a.rt.TableWrite(ctx, …)`, returning on the first error.
- :52-62: `Run` for the sources.

What a TableWrite failure leaves behind:
- `runtime.Activate` has already started the flushers and recorder and set `phaseActive` (lifecycle.go:84-104, `startAppGoroutines` :119-136).
- A second Start hits `activationRefusal(phaseActive)` and gets `ErrStarted` (lifecycle.go:110-120).
- So the sources can never open on that handle. The only way back is Stop, Undeploy and a new Deploy.

The realistic failure is the context ending: TableWrite → `request` → `enqueue` checks `ctx.Err()` first (requests.go:56-59), and `await` returns `ctx.Err()` (:78-86). `CheckRows` cannot fail at Start because deploy.go:217-219 runs the same check at Deploy. The other failures are concurrent Stop, Undeploy or Close (`ErrDrained`, `ErrRemoved`, `ErrClosed`).

GitNexus on `Start`: see the header. All callers are tests, so the method is not per-event.

**Choice (the note decides; plan §5).**

Input: an app deployed with `WithTableRows`, and a `Start` whose context ends after the activation is queued but before the table write is queued.

Candidates:
- **A. Roll back in the facade: Drain on failure.** Rejected, because it fails in exactly the common case.
  - With the expired ctx: `Drain` → `awaitRuns`/`queueDrain` → `enqueue` refuses → `revertDrain` puts the app back to active (drain.go:22-35, :70-80). So nothing is rolled back.
  - With `context.WithoutCancel`: the barrier waits on sink markers with no bound, so Start could hang forever behind a stuck sink.
- **B. The starting rows travel with the app, and the router applies them in the activation step.** Chosen.
  - Activation is one router step whose answer is awaited regardless of ctx once queued (`requestQueued`, lifecycle.go:64-75).
  - So Start either fails before anything changed (the app stays inactive, and a retry works) or activates with its rows applied.
  - After that, `Run` does not read ctx (lifecycle.go:145-169). It fails only on a concurrent Stop, Undeploy or Close, which is correct.
  - Bonus: no `Input(...).Send` racing Start can see the table without its starting rows. Today it can, because the phase is active before the TableWrite.
- **D. A resumable Start in the facade.** Fallback.
  - A mutex plus progress state in AppHandle; a second Start resumes the table writes.
  - Costs: it breaks the "immutable record" design of AppHandle (apphandle.go:12-13). A write applied after its ctx ended would be re-applied, so the recording gets duplicate table records and Records counts them twice.
- **E. Document the trap and pin it.** Last resort; it leaves the dead end.

**Reason for B:** it is the only option that removes the half-started state rather than describing it or working around it, and it keeps AppHandle stateless.

**The runtime seam. This is lane E1's change, to be settled before W2. It is not planned as mine.**
1. `internal/runtime/runtime.go`:
   - `type App` gains `Loads []TableLoad // each table's starting rows, applied by the router in Activate's step, before Activate returns; checked at Add`.
   - New type: `type TableLoad struct { Table int; Rows [][]event.Value }`.
2. `validateApp` (called by `Add`, lifecycle.go:51) checks each load: `0 <= Table < app.Tables.Len()` and `CheckRows(tb.Name(), tb.Attrs(), l.Rows)`. That way the router's apply cannot fail.
3. `router.activate` (transitions.go:77-91), default branch, before `a.activated = true`: apply each load as `writeTable` does today (router_in.go:108-113): `a.write(l.Table, TablePut, l.Rows)`, then one `rTable` record per row.
   - Factor the apply-and-record body into one helper shared by `writeTable` and `activate`, so the two record the same way.
   - On an error (unreachable after step 2): reply with it and leave the app inactive.
4. `item` (messages.go:27-42) stays unchanged, so the per-event intake message does not grow.
5. Activate's doc says the rows are applied in its step.

**The runtime part's kind:** K1h. `router.activate` is reached from the router loop through `ingestLifecycle` (GitNexus), which is an R5 entry point. Proof: K1's proof plus `compare.py` on the path benchmarks.

**Its red test (E1 owns it):** at the `activateHold` seam (lifecycle.go:99-101, package tests only), the table already holds the loads. The revert plant keeps `App.Loads` and `TableLoad` and removes the apply loop from `activate`.

**Risk to name for E1 (from the-fool's pre-mortem):**
- The rows' records are now put before the recorder goroutine starts (`startAppGoroutines` runs after the router replies). So the header plus all tables' starting rows must fit `RecordQueue` (default 65,536, build.go:72-73), or the recording ends with "queue full" (recorder.go:91-110).
- Today the same bound already applies per table: `writeTable` puts a whole table's rows in one router step, non-blocking. So B widens it from "per table" to "all tables together".
- E1's note should pin this corner, or make the starting rows' records exempt.

**2. Kind (facade half):** K1. Start and Deploy are not per-event: the GitNexus upstream is tests only, and R5's entry points are not reached.

**3. The exact change (E2, after E1's seam lands):**
- deploy.go:203-225: build `loads = append(loads, runtime.TableLoad{Table: tid, Rows: rows})` and set `Loads: loads` on the `runtime.App` passed to `r.rt.Add` at deploy.go:109. Keep the `CheckRows` at :217-219 (the facade's own error text).
- Remove `tableLoad` (deploy.go:236-240) and `AppHandle.tableRows` (apphandle.go:24).
- apphandle.go:47-51: delete the loop.
- Start's doc (:41-42) becomes:

  `// Start activates the app: its sinks' flushers, its recorder (the header first) and its tables' starting rows, in one step of the engine; then it opens the sources, each on a goroutine the engine starts and stops. An error before the activation leaves the app as it was, and Start may be called again; once activated, Start fails only when a Stop, Undeploy or Close runs at the same time.`

  This comment is jargon-swept in this commit.

New test in lifecycle_test.go (root package):

```go
// endsAfterFirstCheck is a context whose first Err returns nil and every later Err context.Canceled; Done closes at
// the second Err. Inside Start it ends the context after the activation was queued and before anything later.
type endsAfterFirstCheck struct {
	context.Context
	mu     sync.Mutex
	checks int
	done   chan struct{}
}

func (c *endsAfterFirstCheck) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks++
	switch c.checks {
	case 1:
		return nil
	case 2:
		close(c.done)
	}
	return context.Canceled
}

func (c *endsAfterFirstCheck) Done() <-chan struct{} { return c.done }

// TestAStartWhoseContextEndsMidwayLeavesTheAppStartable: Start either completes, starting rows included, or leaves
// the app startable; the starting row is in T before the first event either way.
func TestAStartWhoseContextEndsMidwayLeavesTheAppStartable(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	row := func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), "a")
		w.SetInt(w.AttrIndex("v"), 1)
		return nil
	}
	h := fcDeploy(t, ctx, r, "tbl", fcTableSrc, bind, cep.WithTableRows("T", row))
	sctx := &endsAfterFirstCheck{Context: context.Background(), done: make(chan struct{})}
	if first := h.Start(sctx); first != nil {
		if err := h.Start(ctx); err != nil {
			t.Fatalf("Start after a Start that failed with %v: %v; want nil (a failed Start leaves the app startable)", first, err)
		}
	}
	in := fcInput(t, h, "S")
	fcSend(t, ctx, in, fcKey("a", fcAt(1)))
	fcSend(t, ctx, in, fcKey("b", fcAt(2)))
	fcStop(t, ctx, h)
	if recs := sink.Drain(); len(recs) != 1 || recs[0].Values[0] != "b" {
		t.Errorf("outputs %+v; want exactly b (a was in T before the first event)", recs)
	}
}
```

How the two codes behave under this test:
- **Old code:** Activate's `enqueue` makes check #1 (nil) and queues; the TableWrite's `enqueue` makes check #2 (Canceled). The second Start then returns `ErrStarted`, so the test goes red at the `t.Fatalf` line.
- **New code:** Start returns nil, because only Activate's `enqueue` reads ctx and the app has no sources.
- **If a future Activate checks ctx twice:** the first Start fails before queuing, the retry succeeds, and the test stays green. So it is not coupled to a call count in a way that could pass vacuously.
- **Assumption to confirm at build:** `fcTableSrc` has the input stream "S" and fcKey fills, as deploy_test.go:280-313 uses them.

**4. Proof (K1).**
- Test: `TestAStartWhoseContextEndsMidwayLeavesTheAppStartable`, lifecycle_test.go.
- Revert plant (`--patch`):
  - re-add `tableRows []tableLoad` to AppHandle, `type tableLoad`, and the Start loop;
  - deploy.go appends to `h.tableRows` and stops setting `Loads`.
  - It keeps E1's `runtime.App.Loads` and `TableLoad`, which are new declarations the facade needs.
- Expected: **red** at the `t.Fatalf("Start after a Start that failed …")` line.

**5. Commit:** E2-1, root K1. It depends on E1's seam commit.

#### G10-23: `print` swallows errors (tools/apipartition/main.go:351-357)

**Verdict: accepted, carried in full to S1.1c-10.**
- `print` writes to a `bytes.Buffer`, which never fails, through `printer.Config.Fprint`.
- Fprint fails only on a writer error or an unsupported node type, and `fileSurface` passes only declarations.
- So the error branch cannot be reached without a seam (a writer or printer variable), and K1 needs a red test.
- The fix (return the error, exit 2) plus its seam is a non-simple change.
- GitNexus (per the survey): `print` ← `fileSurface`. My grep agrees.

#### G2-1: `TestRowReaderNullsAndTypes` passes with zero writes (boundary_test.go:175-235)

**1. Verdict: accepted.** Every check sits inside the sink's `write` closure (:182-215) and only appends to `fails`. If Write never runs, `fails` is empty and `Errors` is 0 (:225), so the test passes. The facade's `reader` is built only by `adapter` (sink.go:27).

**2. Kind:** K2a.

**3. The change:**
- Add `var writes int` beside `fails`.
- Add `writes++` as the first statement after `mu.Lock(); defer mu.Unlock()` in `write`.
- After `mu.Lock(); defer mu.Unlock()` at the end (before the `fails` loop), add:

```go
if writes != 1 {
	t.Errorf("the sink's Write ran %d times; want 1 (every check runs inside it)", writes)
}
```

**4. Proof (`--expect` entries):**

```json
{"id":"G2-1","file":"sink.go","find":"canon, err := playback.AppendEvent(nil, sch, e)","replace":"canon, err := playback.AppendEvent(nil, sch, e); if err == nil { return nil }","test":"TestRowReaderNullsAndTypes","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-1-control","control":true,"test":"TestRowReaderNullsAndTypes","pkg":".","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

The plant drops every output as a silent success. The new test is red on `writes != 1`.

**Load run:** the test runs the engine's goroutines, so `load.py -- -race -count=50 -run '^TestRowReaderNullsAndTypes$'` must show 50/50.

**5. Commit:** E2-2, root K2.

#### G2-2: one-letter `want`s (boundary_test.go:40-62 and :75)

**1. Verdict: accepted.** `strings.Contains(err.Error(), row.want)` with "s", "f", "o", "t" passes on almost any text.

The error sites quote the name with `%q`:
- row.go:57 (time attribute not set), :60 (not optional, not set), :83 (wrong type), :91 (SetNull).
- internal/runtime/boundary.go:199, :211, :221 (value checks), whose texts E1's G7a-5 rewrites.

GitNexus (per the survey): `put` is called by all eight Set* methods.

**2. Kind:** K2a, with per-subtest entries.

**3. The change:** in the `rows` table, the wants become:

| Row | Old want | New want |
|---|---|---|
| index out of range | `"99"` | `"index 99"` |
| wrong type | `"s"` | `"\"s\""` |
| NaN, +Inf | `"f"` | `"\"f\""` |
| malformed UTF-8 | `"s"` | `"\"s\""` |
| object not compact, object not JSON | `"o"` | `"\"o\""` |
| timestamp out of range | `"t"` | `"\"t\""` |
| s unset | `"s"` | `"\"s\""` |
| t unset | `"t"` | `"\"t\""` |
| SetNull | `"s"` | `"\"s\""` |
| fill's error | unchanged | unchanged |

The doc at :5-6 drops "(T21; MUST 15)" in K6? No: this is a test-file comment, so it stays for S1.1c-8. The assertion line is unchanged.

**4. Proof:** one entry per facade site (the runtime sites' text is E1's after W2, so the control entry covers them):

```json
{"id":"G2-2-type","file":"row.go","find":"fmt.Errorf(\"cep: attribute %q is declared %v; %s was called\", a.Name, a.Type, what)","replace":"fmt.Errorf(\"cep: an attribute is declared %v; %s was called\", a.Type, what)","test":"TestSendRowChecks","subtest":"wrong type","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-2-unset","file":"row.go","find":"fmt.Errorf(\"cep: attribute %q is not optional and was not set\", a.Name)","replace":"fmt.Errorf(\"cep: an attribute is not optional and was not set\")","test":"TestSendRowChecks","subtest":"s unset","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-2-time","file":"row.go","find":"fmt.Errorf(\"cep: the time attribute %q was not set\", a.Name)","replace":"fmt.Errorf(\"cep: the time attribute was not set\")","test":"TestSendRowChecks","subtest":"t unset","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-2-null","file":"row.go","find":"fmt.Errorf(\"cep: attribute %q is not optional; SetNull was called\", a.Name)","replace":"fmt.Errorf(\"cep: an attribute is not optional; SetNull was called\")","test":"TestSendRowChecks","subtest":"SetNull on a non-optional attribute","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-2-control","control":true,"test":"TestSendRowChecks","pkg":".","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Why the old test stays green on each plant:
- "wrong type": the planted text still contains "s" ("is", "attribute").
- "t unset": it contains "t" ("the", "attribute").
- The other two keep "s" for the same reason.

**Load run:** `load.py -- -race -count=50 -run '^TestSendRowChecks$'` must show 50/50.

**5. Commit:** E2-2.

#### G2-3: the count is never checked (deploy_test.go:68)

**1. Verdict: accepted.** `!Contains(e,"2 finding(s)") && !Contains(e,"finding(s)")` is false whenever "finding(s)" appears, so the count is never tested. The text comes from diagnostic.go:25: `"cep: deploy %s: %d finding(s):"`.

**2. Kind:** K2a.

**3. The change:**

```go
if want := fmt.Sprintf(": %d finding(s):", len(nonLint)); !strings.Contains(de.Error(), want) {
	t.Errorf("Error() %q; want %q", de.Error(), want)
}
```

The surrounding colons keep "12 finding(s)" from matching.

**4. Proof:**

```json
{"id":"G2-3","file":"diagnostic.go","find":"%d finding(s):\", e.Name, len(e.Diagnostics))","replace":"%d finding(s):\", e.Name, len(e.Diagnostics)+1)","test":"TestDeployReportsEveryDiagnosticAtOnce","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-3-control","control":true,"test":"TestDeployReportsEveryDiagnosticAtOnce","pkg":".","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

**5. Commit:** E2-2.

#### G2-4: `Contains(err.Error(), "k")` (table_test.go:35; deploy_test.go:315)

**1. Verdict: accepted.**
- table_test.go:35: Upsert with `k` unset. The error is row.go:60's `attribute "k" is not optional and was not set`.
- deploy_test.go:315: the same check through Deploy; the text is wrapped as `WithTableRows("T"): …`.

**2. Kind:** K2a.

**3. The change:** both sites assert `strings.Contains(err.Error(), "\"k\"")`. Error messages read "want an error naming \"k\"".

**4. Proof:**

```json
{"id":"G2-4-table","file":"row.go","find":"fmt.Errorf(\"cep: attribute %q is not optional and was not set\", a.Name)","replace":"fmt.Errorf(\"cep: a key attribute is not optional and was not set\")","test":"TestTableRowsAreCheckedAsTableWriteChecksThem","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-4-deploy","file":"row.go","find":"fmt.Errorf(\"cep: attribute %q is not optional and was not set\", a.Name)","replace":"fmt.Errorf(\"cep: a key attribute is not optional and was not set\")","test":"TestWithTableRowsAreVisibleToTheFirstEvent","pkg":".","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Plus two control entries, one per test. The old tests stay green because "key" contains "k".

**Load run:** both tests run goroutines, so each needs `load.py … -count=50` showing 50/50.

**5. Commit:** E2-2.

#### G2-81: Purged after a panicking OnPurge (internal/partition/panic_test.go:168-196)

**1. Verdict: accepted.** partition.go:354-355 runs `r.stats.Purged++` before `r.onPurge(in)`, and `onPurge` recovers and counts `Panics` (:371-377).

**Choice:**
- Input: key a's TTL purge fires, and OnPurge panics.
- Candidates: (a) it counts as a purge; (b) it does not.
- **Chosen: (a).** The purge completes (the instance is reset and freed; the test pins this). The panic is a separate fault, counted in `Panics`. The code already behaves this way, and no design says otherwise.

**2. Kind:** K2a.

**3. The change:** after the `Panics` check (:182-184):

```go
if st := r.Stats(); st.Purged != 1 {
	t.Errorf("Purged %d, want 1: a purge whose OnPurge panicked still completed", st.Purged)
}
```

The doc's last sentence ("(Purged is not asserted: … gap Q12.)") becomes "Purged is 1: the purge completed, and its OnPurge panic counts in Panics as well."

**4. Proof:**

```json
{"id":"G2-81","file":"internal/partition/partition.go","patch":"--- a/internal/partition/partition.go\n+++ b/internal/partition/partition.go\n@@\n-\t\tr.stats.Purged++\n-\t\tr.onPurge(in)\n+\t\tp := r.stats.Panics\n+\t\tr.onPurge(in)\n+\t\tif r.stats.Panics == p {\n+\t\t\tr.stats.Purged++\n+\t\t}\n","test":"TestPanicInOnPurgeStillPurges","pkg":"./internal/partition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Plus a control entry. The build agent writes the hunk header with real line numbers (`Purged++` is exact-once at :354, by grep).

**5. Commit:** E2-5.

#### G2-82: `TestNewRuleRefuses` lacks the empty-key-types case (rule_test.go:205-239)

**1. Verdict: accepted in part.** `errNoKeyTypes` (partition.go:223, :235-236) has no case anywhere in partition's tests (grep: its only uses are those two lines).

**2. Kind:** K2a. The rename is carried.

**3. The change:** add two rows to the case table:

```go
{"nil key types", func(c *partition.Config) { c.KeyTypes = nil }},
{"empty key types", func(c *partition.Config) { c.KeyTypes = []event.Type{} }},
```

**Carried to S1.1c-8:** the rename to `TestNewRuleRefusesABadConfigAndAcceptsEveryScalarKeyType`.
- "Refuses a bad config" is backed by the refusal rows.
- "Accepts every scalar key type" is backed by the loop over six types × TTL {0, minute}.

**4. Proof:**

```json
{"id":"G2-82","file":"internal/partition/partition.go","find":"case len(cfg.KeyTypes) == 0:","replace":"case len(cfg.KeyTypes) < 0:","test":"TestNewRuleRefuses","pkg":"./internal/partition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Plus a control entry. If NewRule panics on the empty key later in the code, that is still a red FAIL.

**5. Commit:** E2-5.

#### G2-83 (later): multi-part key births (multipart_test.go:41, :46-47)

**Verdict: accepted, K2a, E2-5.** Not completed: I did not read keyOf, oaBody's log, or the doc at :46-47.

The design:
- Assert the body's log shows "new" only at offers 1, 2 and 4.
- Two plants: drop the string length prefix in `keyOf`, and stop folding -0. Each must be red on the new test and green on the old `Len()==3`, which holds only when both faults are planted together. So the plants are (a) both faults together (old green, new red) and (b) each fault alone (new red).
- Add "a wrong-typed part" to the doc of `TestMultiPartOfferPanicsOnAMalformedKey`. That doc edit is comment-only inside a test file and rides in the K2 commit.

#### G2-84 (later): CheckTimers fault cases (rule_test.go:244-298)

**Verdict: accepted, K2a, E2-5.** Not completed: I did not read the four messages CheckTimers produces.

The design: each case asserts its own fragment after "partition: instance 3: ". The plant swaps two of CheckTimers' messages, so the old test stays green and the new one goes red.

#### G2-106 (partition part, added by the coordinator)

**1. Verdict: accepted.** Four top-level tests have neither `t.Parallel()` nor a stated reason:
- ttl_test.go:122 `TestOneTTLTimerPerInstance_ReArmAllocatesNothing`;
- panic_test.go:123 `TestBodyPanicResetsTheKey`;
- panic_test.go:173 `TestPanicInOnPurgeStillPurges`;
- panic_test.go:204 `TestPendingExcludesTTL`.

The panic tests share no package state: each builds its own `pnBody` and Rule (`pnRule`, :79-87), and the package-level names in helpers_test.go are constants.

**2. Kind:** K2e.

**3. The change:**
- `t.Parallel()` becomes the first statement of the three panic tests.
- The allocation test's doc gains the line `// Runs serially: testing.AllocsPerRun counts every goroutine's allocations, so a parallel test would add its own.`

**4. Proof (the mechanical check):** for each `func TestX(t *testing.T) {` in `internal/partition/*_test.go`, the first statement must be `t.Parallel()`, or the doc comment must contain `Runs serially:`. Any other test is listed.
- Command:

  ```
  python3 -I -c "import re,glob,sys; bad=[f'{p}:{i+1}' for p in sorted(glob.glob('internal/partition/*_test.go')) for L in [open(p).read().split('\n')] for i,l in enumerate(L) if re.match(r'func Test\w+\(t \*testing\.T\) \{$',l) and L[i+1].strip()!='t.Parallel()' and not any('Runs serially:' in L[j] for j in range(max(0,i-12),i) )]; print('\n'.join(bad)); sys.exit(1 if bad else 0)"
  ```

- On the old tree it is red and lists at least the four lines above. The build agent quotes the full list; any extra test gets the same treatment in this commit, or is reported.
- On the new tree it exits 0.

**5. Commit:** E2-5. In the record: "G2-106's partition part".

#### G2-141: `ErrReopened` is never asserted (io/inmemory/breakit_s3_3_1_test.go:86-101)

**1. Verdict: accepted in part.** source.go:92-95 returns `ErrReopened` on a second Open. `git grep ErrReopened` finds only source.go.

**2. Kind:** K2a. The rename is carried.

**3. The change:** capture the second Open's return as `err2 = src.Open(cctx, bkNopEmitter{})` inside the recover closure. After the panic check:

```go
if !errors.Is(err2, inmemory.ErrReopened) {
	t.Errorf("the second Open: %v; want inmemory.ErrReopened", err2)
}
```

**Carried to S1.1c-8:** the rename to `TestASecondOpenReturnsErrReopened`, backed by the `errors.Is` assertion.

**4. Proof:**

```json
{"id":"G2-141","file":"io/inmemory/source.go","find":"\t\treturn ErrReopened","replace":"\t\treturn nil","test":"TestBreakitOpenTwice","pkg":"./io/inmemory","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Plus a control entry.

**5. Commit:** E2-6.

#### G2-142: `TestEndBatchIsQueuedInOrder` (inmemory_test.go:157-175)

**Verdict: accepted, carried in full to S1.1c-8.** The test asserts Batches == 1 and one record; nothing in it checks order. A real order check needs a recording reader, which `imApp` does not offer. So the honest fix is the finding's alternative: rename to `TestEndBatchIsCountedByTheStream`, with the doc to match.
- "Counted" is backed by `Batches == 1`.
- "ByTheStream" is backed by the stream's stats entry.

#### G2-143 (later): the "context ends while blocked" case races (coverage_s3_3_1_test.go:80-85)

**1. Verdict: accepted.** `cancelShort()` runs right after the `go` statement. Push's `ctx.Err()` check (source.go:44) can see the context already cancelled, so the case can take the expired path tested just above it.

**2. Kind:** K2b: the input sometimes misses the promised path (the blocked `case <-ctx.Done()` at source.go:57-58).

**3. The change:** wrap `short` in a park context like `bkParkCtx` (breakit_s3_3_1_test.go:22-35, same package `inmemory_test`). Its Done signals `parked` and returns `short.Done()`. Then:

```go
go func() { done <- s.Push(pctx, "", imRow("a", 1)) }()
<-pctx.parked
cancelShort()
```

Push reads Done only in the blocking select (source.go:52-58), after the Err check, so "parked" proves the blocked path.

**4. Proof:**
- Plant: source.go:57-58, `case <-ctx.Done():\n\t\treturn ctx.Err()` → `case <-ctx.Done():\n\t\treturn nil`.
- The new test is red deterministically.
- The old test's outcome on the plant is nondeterministic. So the coordinator should accept an `old` of "green with k<50 reds" from `mutate.py --repeat 50 --load` instead of a single green. **To confirm before G1.**
- `bkParkCtx`'s exact shape must be read at build.
- Load run: `load.py -- -race -count=50` on the test, 50/50.

**5. Commit:** E2-6.

#### G2-144: the embedded-interface refusal is untested (tools/apipartition/main.go:325-331)

**1. Verdict: accepted.** By grep, the message "embeds an interface" appears only in main.go:328.

**2. Kind:** K2a, with a new test.

**3. The change:** in main_test.go:

```go
// TestEmbeddedInterfaceIsRefused: exit 2, the failed: line naming the type and the fix, and the golden untouched.
func TestEmbeddedInterfaceIsRefused(t *testing.T) {
	t.Parallel()
	golden := write(t, "")
	code, out := run(t, golden, pkgDir(t, "package p\n\ntype R interface{ Read() }\n\ntype W interface {\n\tR\n\tWrite()\n}\n"))
	if code != 2 {
		t.Errorf("exit %d, want 2 (refused):\n%s", code, out)
	}
	line := failedLine(t, out)
	for _, want := range []string{"type W", "embeds an interface", "list the methods instead"} {
		if !strings.Contains(line, want) {
			t.Errorf("failed: line %q lacks %q", line, want)
		}
	}
}
```

`pkgDir` is used by breakit_s3_3_1_test.go:28 in the same package. Its signature must be confirmed at build.

**4. Proof:**

```json
{"id":"G2-144","file":"tools/apipartition/main.go","find":"if len(m.Names) == 0 {","replace":"if len(m.Names) < 0 {","old_test":"TestEmbeddedFieldIsRefused","new_test":"TestEmbeddedInterfaceIsRefused","pkg":"./tools/apipartition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Plus a control entry for the new test. The `find` text must be confirmed exact-once at build; the struct branch uses another variable name.

**5. Commit:** E2-7.

#### G2-145: "golden touched" checks only that it exists (main_test.go:184-186)

**1. Verdict: accepted.**

**2. Kind:** K2a.

**3. The change:**

```go
if b, err := os.ReadFile(golden); err != nil || string(b) != "" {
	t.Errorf("the golden after a refused run: %q, %v; want it still empty", b, err)
}
```

**4. Proof:** the plant writes the golden before the refusal returns. **Site to complete:** the `--write`/golden write in main.go was not read.

**5. Commit:** E2-7.

#### G2-146 (later): the iota marker (breakit_s3_3_1_test.go:33)

**Verdict: accepted, K2a, E2-7.**

The design: assert B's full surface line, as `surfaceOf` prints it, including its value marker. **Text to complete:** the exact line was not read. The survey's `const B T = iota // iota 1` has not been checked.

The plant: drop the value marker from the surface printer. The old assertion stays green; the new one goes red.

#### G2-156: the shape tool's "lowering" stage is untested (tools/shape/main.go:59-61)

**1. Verdict: accepted.** `parser.LoweringCodes = {"E-DUR-002","E-STR-002","E-TYPE-001","E-WIN-004"}` (internal/parser/errors.go:59). Per the survey's grep, the corpus golden has no `"stage":"lowering"` row.

**2. Kind:** K2a, with a new test.

**3. The change:** new test `TestJudgeReportsEachLoweringCodeAtTheLoweringStage` in shape_test.go.
- One row per code: a grammar-valid source that fails with that code.
- Each row asserts `r.Verdict == "reject"`, `r.Code == code` and `r.Stage == "lowering"`.
- **Inputs to complete:** take each from that code's invalid example in the conformance tree or docs/language, byte-exact.

**4. Proof:**

```json
{"id":"G2-156","file":"tools/shape/main.go","find":"if slices.Contains(parser.LoweringCodes, r.Code) {","replace":"if false && slices.Contains(parser.LoweringCodes, r.Code) {","old_test":"TestShapeCorpusGolden","new_test":"TestJudgeReportsEachLoweringCodeAtTheLoweringStage","pkg":"./tools/shape","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Plus a control entry.

**5. Commit:** E2-8.

#### G7a-1: connector errors name sprint ids (connectors.go:18-38)

**1. Verdict: accepted.**
- connectors.go:22-25 maps types to "S5.1"/"S5.2".
- :36 prints "arrives in %s; bind a Go %s with With%s for now".
- GitNexus: `notBuilt` ← `defaultFactory.Source` and `.Sink`, exact, LOW.
- `connectorSprint` is read only at :32 (grep).

**2. Kind:** K1. The change is to text only, at Deploy time, so it is not per-event.

**3. The change (the choice):**
- Input: `@sink(type = "http")` with no Go sink bound.
- Candidates:
  - (a) keep the sprint names;
  - (b) `@sink(type = "http") is not available in this release; bind a Go sink with WithSink`;
  - (c) a generic "not built".
- **Chosen: (b).** It carries no process ids, does not go stale, and still tells the user what to do. The inmemory branch (:28-31) is unchanged.

New code:

```go
func notBuilt(kind, typ string) error {
	if typ == "inmemory" { /* unchanged */ }
	return fmt.Errorf("%w: @%s(type = %q) is not available in this release; bind a Go %s with With%s", ErrConnectorNotBuilt,
		kind, typ, kind, title(kind))
}
```

- `connectorSprint` is deleted.
- The comment at :18-19 becomes `// defaultFactory refuses every annotation-bound type: this release builds none; a Go binding (WithSource, WithSink) replaces the annotation.`

**Dry run** (`git grep -n 'arrives in\|"S5\.[12]"\|ErrConnectorNotBuilt' -- '*_test.go'`):
- deploy_test.go:246-247 pins "http" and "S5.1". It becomes `strings.Contains(err.Error(), "@sink(type = \"http\") is not available in this release; bind a Go sink with WithSink")`.
- coverage_s3_3_1_test.go:84-85 pins "S5.1". It becomes `strings.Contains(err.Error(), "is not available in this release")`.
- coverage_s3_3_1_test.go:98-99 pins "WithSource" for the inmemory branch. Unchanged.
- No non-test file outside docs/reviews and docs/sprints pins "arrives in S".

**4. Proof (K1):**
- Revert plant (`--patch`): restore `connectorSprint` and the old `notBuilt` body.
- Expected: **red** at deploy_test.go:246's `t.Errorf` (and at coverage :84).

**5. Commit:** E2-1, root K1.

#### G7a-2: process ids in public godoc (options.go:72, :80; example_test.go:14)

**1. Verdict: accepted, widened.**
- The finding's reason is that these are rendered public docs.
- `apidiff` covers `.`, `./io` and `./io/inmemory`.
- The plan's K6 carries no per-package rule, so one K6 commit covers the rendered godoc of all three packages.
- My sweep (the §7.1 pattern over comments in non-test files of the three packages) found 49 comment lines. These are the ones that render in `go doc`:
  - **Root:** cep.go:1-4 (also drops "Stubs until the build's step 5.", which is false today); apphandle.go:12-13, :66-67, :82-84; deploy.go:19-20; input.go:29-30 (also "(MUST 14)"), :44; options.go:9-10, :71-72, :80, :94; stats.go:5; table.go:11, :83; example_test.go:14-15.
  - **io:** cepio.go:1, :40, :78-79, :92, :97-98.
  - **io/inmemory:** source.go:1, :28, :79-80.
  - **Left alone:** apphandle.go:41-42 is rewritten by G10-1's K1 instead. Unexported and in-body comments (row.go, sink.go, deploy.go:25, :45 and others) are not rendered and not touched.

**2. Kind:** K6.

**3. The change:** each parenthetical or clause the sweep flags is removed; the behaviour text stays. Examples:
- options.go:71-72: `// WithTableRows gives a table its starting rows, written at Start after the recording's header and before any source opens. The fills run at Deploy.`
- options.go:80: `// WithRecording turns the app's recording on, written to w. Recording is off unless this option is given.`
- example_test.go:14-15: `// Example_embed deploys example 09, feeds it two rows, stops it and prints the one alert as its canonical JSON.`
- apphandle.go:66-67: `… A stopped app stays stopped: a restart is a new Deploy. …`
- table.go:83: `// Load is a multi-row Upsert.`
- cepio.go:92: `// Mapper decodes one payload into a row.`
- cepio.go:97-98: `Canonical is the output's canonical JSON, a fresh slice per output the sink may keep.`
- Package docs: drop "(ADR-… ; design s3-3-1 §…)".

**4. Proof (K6):**
- `go run ./tools/astequal <parent> <new>` exits 0 on every touched file.
- `git diff -U0 -- example_test.go | grep -c '^[-+].*// Output:'` prints 0.
- The jargon sweep over the touched comments shows no hit. Shown red on options.go:72's old text, which hits `\bowner\b`, the parenthesised-date rule and `\bdesign \d`. Also red on options.go:80 (`owner`, `D4`, `S3.2b-2`) and example_test.go:14 (`owner`, `S3.3`, `T40`). Green on the new texts.
- ADR references left elsewhere (cepio.go:87) are not hits under the pattern.

**5. Commit:** E2-3, public-godoc K6. It comes after E2-1, so Start's comment is already new.

#### G7a-4: process ids in NewRule's error (internal/partition/partition.go:238-240)

**1. Verdict: accepted.** The error text ends "(owner D7 of S3.2a-1: object keys are a deploy error from S3.3)". GitNexus: `NewRule` ← `shard.install` (LOW, exact).

**2. Kind:** K1. R5's error-text exemption applies, named here: NewRule sits in `shard.run`'s process, but only an error branch's text changes.

**3. The change (the choice):**
- Candidates: (a) the survey's "an object or invalid part cannot key a partition"; (b) the runtime's own sibling text from internal/runtime/build.go:146, "key types %v: a part of these types cannot key a partition".
- **Chosen: (b),** with the "partition:" prefix: `return nil, fmt.Errorf("partition: key types %v: a part of these types cannot key a partition", cfg.KeyTypes)`. It matches the engine's existing wording and is exact for every type the case refuses.

**Dry run:**
- objectkey_test.go:30 pins "partition" and "key". Both are kept.
- No test pins "owner D7" or "S3.3" (`git grep`).
- docs/language/errors.md:119 cites the guard in prose without quoting the text.

New assertion in objectkey_test.go's loop:

```go
if !strings.HasSuffix(err.Error(), ": a part of these types cannot key a partition") {
	t.Errorf("NewRule(%s): %q; want it to end in the refusal, with no process reference", c.name, err)
}
```

**4. Proof (K1):** revert plant: restore the two-line old format string. Expected **red** at the new `HasSuffix` line.

**5. Commit:** E2-4, partition K1.

### 4. Commit plan (lane E2, W3)

1. **E2-1, root K1:** G10-1 (facade half) and G7a-1. Needs E1's seam commit to land first.
   - Proofs: two revert plants, both red.
   - T2 green.
2. **E2-2, root K2:** G2-1, G2-2, G2-3, G2-4.
   - `--expect` batch: 8 plant entries and 5 controls.
   - Load runs for the four engine tests.
3. **E2-3, public-godoc K6 (`.`, `./io`, `./io/inmemory`):** G7a-2.
   - astequal, the `// Output:` count, and the sweep.
4. **E2-4, partition K1:** G7a-4. One revert plant.
5. **E2-5, partition K2:** G2-81, G2-82, G2-83, G2-84, and G2-106's partition part.
   - `--expect` entries.
   - The G2-106 check, red then green.
6. **E2-6, io/inmemory K2:** G2-141, G2-143.
7. **E2-7, apipartition K2:** G2-144, G2-145, G2-146.
8. **E2-8, shape K2:** G2-156.

**Count: 8 commits,** against the lane's line of 9 (ceiling 11). Plant batches go to `docs/sprints/s1-1c-7-plants/E2.json` in the coordinator's proof docs commit.

### 5. Choices made (each "chosen by the design note")

| Id | Input | Candidates | Chosen | Reason | Dry run |
|---|---|---|---|---|---|
| G10-1 | Start's ctx ends between the activation and a table write | A Drain rollback; B rows in the activation step; D resumable Start; E document only | B (runtime seam, E1) | the only option that removes the half state; A fails on an expired ctx (drain.go:22-35); D breaks immutability and duplicates records | behaviour only; lifecycle_test.go:49's ErrStarted after a successful Start still holds |
| G2-81 | OnPurge panics during a TTL purge | counts / does not count | counts | the purge completed; the panic is counted in Panics; the code already does this (partition.go:354) | none |
| G7a-1 | `@sink(type="http")` with no Go sink | keep sprint ids / "not available in this release; bind a Go sink with WithSink" / generic | the second | no process ids, never stale, still tells the user what to do | deploy_test.go:246-247, coverage_s3_3_1_test.go:84-85 |
| G7a-4 | NewRule given an object or invalid key type | survey's wording / runtime/build.go:146's wording | build.go:146's, with the "partition:" prefix | matches the engine's sibling text and is exact for every refused type | objectkey_test.go:30 (kept) |

**Public API (apidiff on `.`, `./io`, `./io/inmemory`):**
- No exported signature changes.
- Embedders see two behaviour changes:
  - G10-1: a Start that fails before the activation leaves the app startable, and the starting rows are applied with the activation.
  - G7a-1: `ErrConnectorNotBuilt`'s message text; the wrapped sentinel is unchanged.
- The godoc text changes (G7a-2).
- `runtime.App.Loads` is in an internal package, outside apidiff.

### 6. Couplings and risks

- **Plan §0 item 5's anchors:** none touched. No `-run` pins, corpus, baseline, fuzz or notbuilt files, and nothing in `gate/`.
- **Per-event code (R5):**
  - E2-1's facade edits (Start, Deploy, notBuilt) are not per-event; GitNexus upstream is tests or Deploy only.
  - G7a-4: error-text exemption, named above.
  - The G10-1 runtime half is K1h, in lane E1.
  - Plants are proof-only.
- **Frozen benchmarks (R4):** none touched.
- **Frame tables and line-anchored mutants (§0 item 4):** none in my files. G10-1's new test depends on Activate reading ctx once before queuing (requests.go:57); if that changes, the test stays meaningful (see G10-1).
- **Other owners' files I need:**
  - internal/runtime (E1): the G10-1 seam, to settle before W2.
  - E1's G7a-5 rewording of internal/runtime/boundary.go must keep `attribute %q` for G2-2's NaN, +Inf, UTF-8, object and timestamp rows. E2's control entry checks this at its lane base.
- **Pre-mortem (the-fool) on G10-1:** the risky part is the recording queue. Starting rows put before the recorder goroutine starts must fit `RecordQueue` in total, not per table. E1's note should pin it.

### 7. Carried items

**Renames → S1.1c-8:**

| Finding | Proposed name | Backed by |
|---|---|---|
| G2-82 | `TestNewRuleRefusesABadConfigAndAcceptsEveryScalarKeyType` | the refusal rows; the accept loop |
| G2-141 | `TestASecondOpenReturnsErrReopened` | the `errors.Is(err2, ErrReopened)` assertion |
| G2-142 (carried in full) | `TestEndBatchIsCountedByTheStream` | `Batches == 1`; the stream's stats |

**→ S1.1c-10:**
- G10-23, carried in full: `print`'s error path needs a seam to be provable.
- New finding from lane B's note (not a survey finding): `world.advance` in internal/partition/helpers_test.go ignores body panics, as playback's `ldWorld.step` does (G2-94). No fix in this chunk touches it.

**Open items to finish before G1:** the plant or input text for G2-83, -84, -145, -146 and -156, and the coordinator's decision on G2-143's proof form.


## Addendum: the six completed proof items

## E2 design note: completion pass (replacement sections only)

All six open items are now build-ready. Nothing was run: every expected outcome is reasoned from the code. The control entries are what check my computed texts against the real code.

- **Base:** HEAD 1f4ebba.
- **GitNexus calls this pass:** 0. A hook reported the graph DB held by another GitNexus process. Every claim below comes from reading the code, with file:line.
- **Exact-once check:** every plant `find` was counted with `python3 -I -c` over the file and occurs exactly once (counts quoted per entry).
- **One thing differs from the brief, for G2-156:** neither conformance/ nor docs/language/invalid/ (29 files) has an example for any of the four lowering codes. That is why the shape golden has no lowering row. The four inputs are therefore taken from the parser's own `TestLoweringCodes` (internal/parser/lowering_test.go:16-45), which already proves each one reaches its code through `parser.Parse`.

### Replacement summary rows

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G2-83 | accepted | K2a | E2-5 partition K2 | – | assert the body's exact log (births at offers 1, 2, 4); doc names the wrong-type case |
| G2-84 | accepted | K2a | E2-5 partition K2 | – | each fault case asserts CheckTimers' exact error text |
| G2-143 | accepted | K2b (the coordinator's three-part proof) | E2-6 io/inmemory K2 | – | a park-signalling context; cancel only after the Push reached its select |
| G2-145 | accepted | K2a | E2-7 apipartition K2 | – | the golden's content must still be "" after a refused run |
| G2-146 | accepted | K2a | E2-7 apipartition K2 | – | assert B's full line `p B const const B T = iota // iota 1` |
| G2-156 | accepted | K2a | E2-8 shape K2 | – | new test, one row per `parser.LoweringCodes` code, stage "lowering" |

### G2-83 (internal/partition/multipart_test.go:41-43, :46-47)

**Code read.**
- `keyOf` (partition.go:160-193) builds the map key from the first part plus a byte string over `Rest`. For a string it writes the kind byte, `binary.AppendUvarint(len)` (:168), then the bytes (:169). For a float it folds -0.0 into +0.0 (:177-180).
- `oaBody` (onarrival_test.go:27-47) logs `"new "+keyName(First)` in NewState and `"ev "+keyName(First)+" @"+ts` in OnEvent.
- With `OnArrival: true`, Offer births the key and calls OnEvent at once (`oaWant(t, b, "new a", "ev a @0")`, onarrival_test.go:81).
- The test offers five keys whose first part is always "k", with `ev(0, i+1, i+1)`, so ts = 1…5.

**Expected log on the real code:**
- offer 1: new key;
- offer 2: new key (the length prefix keeps it apart);
- offer 3: offer 1's key again;
- offer 4: new key;
- offer 5: offer 4's key (-0.0 is +0.0).

**The two faults:**
- Without the length prefix, offers 1 and 2 encode identically: `K 'a' K K 'c' F bits` in both cases.
- Without the fold, offer 5 becomes a fourth key.
- With both faults, `Len()` is still 3.

**The change** (test file only). After the `r.Len() != 3` check (:41-43), add:

```go
	// births only at offers 1, 2 and 4: a lost length prefix merges offers 1 and 2; an unfolded -0.0 splits 4 and 5
	oaWant(t, b, "new k", "ev k @1", "new k", "ev k @2", "ev k @3", "new k", "ev k @4", "ev k @5")
```

`oaWant` (onarrival_test.go:65-70) is in the same package, `partition_test`, and calls Fatalf on a mismatch.

Doc of `TestMultiPartOfferPanicsOnAMalformedKey` (:46-47). Old:
```
// TestMultiPartOfferPanicsOnAMalformedKey (design s3-2b-2 §2.6): the router checks first, so Offer panics on a part
// count other than the rule's and on a null part, as on a single null key.
```
New:
```
// TestMultiPartOfferPanicsOnAMalformedKey (design s3-2b-2 §2.6): the router checks first, so Offer panics on a part
// count other than the rule's, on a null part (as on a single null key) and on a part of a type other than the rule's.
```

**Proof (`--expect` entries).** Each plant `find` occurs once in partition.go: :168 and :179. The hunk line numbers were read from partition.go:166-181.

```json
{"id":"G2-83-both","patch":"--- a/internal/partition/partition.go\n+++ b/internal/partition/partition.go\n@@ -167,3 +167,3 @@\n \t\t\tcase event.TString:\n-\t\t\t\tbuf = binary.AppendUvarint(buf, uint64(len(v.Str)))\n+\t\t\t\t_ = len(v.Str)\n \t\t\t\tbuf = append(buf, v.Str...)\n@@ -178,3 +178,3 @@\n \t\t\t\tif f == 0 {\n-\t\t\t\t\tf = 0 // -0.0 is +0.0\n+\t\t\t\t\tf = v.F // -0.0 kept\n \t\t\t\t}\n","test":"TestMultiPartKeysAreOneInstancePerTuple","pkg":"./internal/partition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-83-prefix","file":"internal/partition/partition.go","find":"buf = binary.AppendUvarint(buf, uint64(len(v.Str)))","replace":"_ = len(v.Str)","test":"TestMultiPartKeysAreOneInstancePerTuple","pkg":"./internal/partition","expect":{"old":"red","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-83-negzero","file":"internal/partition/partition.go","find":"f = 0 // -0.0 is +0.0","replace":"f = v.F // -0.0 kept","test":"TestMultiPartKeysAreOneInstancePerTuple","pkg":"./internal/partition","expect":{"old":"red","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-83-control","control":true,"test":"TestMultiPartKeysAreOneInstancePerTuple","pkg":"./internal/partition","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

What each plant does:
- **both:** offers 1 and 2 merge and offers 4 and 5 split, so `Len()` is 3. The old test stays green; the new log is "new k, ev k @1, ev k @2, ev k @3, new k, ev k @4, new k, ev k @5", so the new test is red.
- **prefix alone:** gives 2 instances, so old and new are both red.
- **negzero alone:** gives 4 instances, so old and new are both red.

Both replacements build: `binary` is still used at :171 and below, and `f = v.F` is a no-op that keeps -0.

**Load run:** the test starts no goroutines, so none is needed.

### G2-84 (internal/partition/rule_test.go:240-297)

**Code read.** `CheckTimers` (partition.go:465-490) has two messages, not four:
- (1) :476-477 `"partition: instance %d: its state implies a TTLPurge, which only partition schedules"`;
- (2) :485-486 `"partition: instance %d: pending timers %v, but its state implies %v"`, over `got` and `want`, both sorted by `cmpTimer` (Kind, then Due, then Ref, :492-494).

**How `%v` renders the timers.**
- `clock.Timer` is `{Due int64; Kind TimerKind; Ref uint64}` (clock.go:235-239).
- `TimerKind` values: Completion=1, CronClose=2, DelayDue=3, BatchClose=4, TTLPurge=5, Discard=6 (:223-230).
- `grep ') String() string' internal/clock/` finds nothing, so `%v` prints `{Due Kind Ref}` as numbers.

**What happens to key d** (ordinal 3, as the passing prefix check shows):
- `second` = 1000 (helpers_test.go:23).
- d's event has ts 3000.
- On release, partition sets `LastActivity` = 3000 and re-arms the TTL at 3000 + 300000 = 303000, before the body's OnEvent (partition.go:340-345).
- The body's `Schedule` with the same kind and ref replaces the existing timer (clock/rule.go:350-358).

**The four messages:**

| Case | Message | Why |
|---|---|---|
| hidden timer | `partition: instance 3: pending timers [{63000 1 1} {303000 5 0}], but its state implies [{303000 5 0}]` | got = TTL plus a Completion at 3000+60000, Ref 1; the hidden state implies none |
| phantom timer | `partition: instance 3: pending timers [{303000 5 0}], but its state implies [{540000 3 2} {303000 5 0}]` | the state reports a DelayDue at 540000, Ref 2, which sorts before the TTL by kind |
| body TTLPurge, reported | `partition: instance 3: its state implies a TTLPurge, which only partition schedules` | the state reports a TTLPurge |
| body TTLPurge, moved | `partition: instance 3: pending timers [{543000 5 0}], but its state implies [{303000 5 0}]` | the body's Schedule moves the TTL to 543000; the state is hidden |

**The change** (test file only):
- The case struct gains a fourth field, `want string`, after `plant`.
- The rows become (positional, as today):

```go
		{"hidden timer", completion, func(st *state) { st.hide = true },
			"partition: instance 3: pending timers [{63000 1 1} {303000 5 0}], but its state implies [{303000 5 0}]"},
		{"phantom timer", nil, func(st *state) { st.extra = []clock.Timer{{Due: 9 * minute, Kind: clock.DelayDue, Ref: 2}} },
			"partition: instance 3: pending timers [{303000 5 0}], but its state implies [{540000 3 2} {303000 5 0}]"},
		{"body TTLPurge, reported", func(in *partition.Instance, e event.Event) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 5*minute, Kind: clock.TTLPurge}}
		}, nil, "partition: instance 3: its state implies a TTLPurge, which only partition schedules"},
		{"body TTLPurge, moved", func(in *partition.Instance, e event.Event) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 9*minute, Kind: clock.TTLPurge}}
		}, func(st *state) { st.hide = true },
			"partition: instance 3: pending timers [{543000 5 0}], but its state implies [{303000 5 0}]"},
```

The existing per-row comments stay above each row.

The `HasPrefix` check (:292-294) becomes:

```go
			if err.Error() != c.want {
				t.Errorf("CheckTimers: %q, want %q", err, c.want)
			}
```

The doc's third line (`begins "partition: instance 3: "`) becomes `names the fault exactly`.

**Proof.** Both plant `find`s occur once.

```json
{"id":"G2-84-message","file":"internal/partition/partition.go","find":"its state implies a TTLPurge, which only partition schedules\"","replace":"pending timers differ from what its state implies\"","test":"TestCheckTimers_PlantedFaults","subtest":"body TTLPurge, reported","pkg":"./internal/partition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-84-swap-hidden","file":"internal/partition/partition.go","find":"in.Clock.Ordinal(), got,\n\t\t\t\twant)","replace":"in.Clock.Ordinal(), want,\n\t\t\t\tgot)","test":"TestCheckTimers_PlantedFaults","subtest":"hidden timer","pkg":"./internal/partition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-84-swap-phantom","file":"internal/partition/partition.go","find":"in.Clock.Ordinal(), got,\n\t\t\t\twant)","replace":"in.Clock.Ordinal(), want,\n\t\t\t\tgot)","test":"TestCheckTimers_PlantedFaults","subtest":"phantom timer","pkg":"./internal/partition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-84-swap-moved","file":"internal/partition/partition.go","find":"in.Clock.Ordinal(), got,\n\t\t\t\twant)","replace":"in.Clock.Ordinal(), want,\n\t\t\t\tgot)","test":"TestCheckTimers_PlantedFaults","subtest":"body TTLPurge, moved","pkg":"./internal/partition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-84-control","control":true,"test":"TestCheckTimers_PlantedFaults","pkg":"./internal/partition","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Under both plants the prefix still holds, so the old test stays green.
- **message plant:** the TTLPurge fault reads as a mismatch.
- **swap plant:** got and want are swapped in the text, so each mismatch case names the wrong side.

The control entry checks my computed texts against the real code. If the control is red, the build agent stops and reports the real text; it does not edit the test to match.

### G2-143 (io/inmemory/coverage_s3_3_1_test.go:80-85)

**Code read.**
- `push` (source.go:43-59) checks `ctx.Err()` at :44.
- A non-blocking quit check follows (:47-51).
- `ctx.Done()` is read only in the blocking select (:52-58), so a Done read proves the :44 check has already passed.
- The branch's first body statement is :58, `return ctx.Err()`.
- `bkParkCtx` (breakit_s3_3_1_test.go:22-33) signals `parked` on the first Done, but returns `nil`: a context that never ends. So it cannot be cancelled, and a new type is needed.

**The change** (test file only).

Add `"sync"` to coverage_s3_3_1_test.go's imports, and add:

```go
// parkSignalCtx is its Context with one change: the first Done read closes parked. Source.push reads Done only in its
// blocking select, after its expired-context check, so parked means the Push waits there with a live context.
type parkSignalCtx struct {
	context.Context
	once   sync.Once
	parked chan struct{}
}

func (c *parkSignalCtx) Done() <-chan struct{} {
	c.once.Do(func() { close(c.parked) })
	return c.Context.Done()
}
```

Replace lines :80-85 (from `short, cancelShort := context.WithCancel(ctx)` through the closing `}` of the `<-done` check) with:

```go
	short, cancelShort := context.WithCancel(ctx)
	pctx := &parkSignalCtx{Context: short, parked: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- s.Push(pctx, "", imRow("a", 1)) }() // unbuffered, no pump: blocks until short ends
	select {
	case <-pctx.parked: // past the expired-context check: the Push waits in its select
	case <-ctx.Done():
		t.Fatalf("ceiling hit before the Push reached its select: %v", ctx.Err())
	}
	cancelShort()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Push while nothing pumps, its context ended: %v; want context.Canceled", err)
	}
```

- `ctx` is the test's `imCeiling(t)` (:68).
- If the Fatalf fires, the Push goroutine still returns when the ceiling cancels `short`.

**The plant.** Its `find` occurs once in source.go (counted). Open's own `ctx.Done()` case at :106-108 is indented one tab deeper and has a line between, so it does not match.
- find: `"\n\tcase <-ctx.Done():\n\t\treturn ctx.Err()\n"`
- replace: `"\n\tcase <-ctx.Done():\n\t\treturn nil\n"`

**Proof (the coordinator's three parts):**
1. **Old test, k of 50.** At the lane base:

   `python3.15 gate/bench/mutate.py --file io/inmemory/source.go --find $'\n\tcase <-ctx.Done():\n\t\treturn ctx.Err()\n' --replace $'\n\tcase <-ctx.Done():\n\t\treturn nil\n' --test TestCviSourceRefusalsAndContexts --pkg ./io/inmemory --repeat 50 --load`

   Record whatever k is. k < 50 shows the old test sometimes takes the expired path at :44-45, where the plant has no effect.
2. **New test, 50 of 50.** The same command at the commit. The new test must be red 50/50: the Push parks, the plant returns nil, and `errors.Is(nil, context.Canceled)` fails.
3. **Coverage, twice.** At the commit, run twice under the pinned toolchain:

   `go test -count=1 -run '^TestCviSourceRefusalsAndContexts$' -coverprofile=<scratch>/g2-143-run1.out ./io/inmemory`

   and again with `-run2.out`. In each profile, the block of `io/inmemory/source.go` that starts on line 57 and ends on line 58 (the `case <-ctx.Done():` body) must have a count > 0. Quote both lines.

**Load run** (the test runs a goroutine): `python3.15 gate/bench/load.py -- -race -count=50 -run '^TestCviSourceRefusalsAndContexts$' ./io/inmemory` must show 50/50.

### G2-145 (tools/apipartition/main_test.go:184-186)

**Code read.**
- The golden is written only on the `--write` path (main.go:66-72).
- A refusal exits at main.go:52-55: `fmt.Printf("failed: %v\n", err)`, `GATE_COVERED`, `os.Exit(2)`.
- `TestEmbeddedFieldIsRefused` runs without `--write` (`run(t, golden, embed)`, :171). So the plant writes the golden inside that refusal branch.

**The change.** Replace :184-186 with:

```go
	if b, err := os.ReadFile(golden); err != nil || string(b) != "" {
		t.Errorf("the golden after a refused run: %q, %v; want it still empty", b, err)
	}
```

**Proof.** The `find` occurs once in main.go (:53).

```json
{"id":"G2-145","file":"tools/apipartition/main.go","find":"fmt.Printf(\"failed: %v\\n\", err)","replace":"_ = os.WriteFile(golden, []byte(\"x\"), 0o644); fmt.Printf(\"failed: %v\\n\", err)","test":"TestEmbeddedFieldIsRefused","pkg":"./tools/apipartition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-145-control","control":true,"test":"TestEmbeddedFieldIsRefused","pkg":"./tools/apipartition","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

The old `os.Stat` passes on the written file, so the old test stays green. The new content is "x", not "", so the new test is red.

### G2-146 (tools/apipartition/breakit_s3_3_1_test.go:33-35)

**Code read.**
- `fileSurface` (main.go:236-280) gives an implicitly repeated const its block's last type and values (:249-256).
- It prints a one-name `GenDecl` (`const B T = iota`) and appends `" // iota %d"` with the spec's index in its block (:269-271). `specIndex` resets per declaration (:227) and increments per spec (:275).
- `entry.line()` is `pkg + " " + ident + " " + kind + " " + sig` (:36).
- Lines are sorted, so in `before` B's line follows A's.
- B's full line: `p B const const B T = iota // iota 1`.

**The change.** Replace :33-35 with:

```go
	if want := "\np B const const B T = iota // iota 1\n"; !strings.Contains(before, want) {
		t.Errorf("surface:\n%s\nwant B's line %q: its type T and its value, iota 1", before, want)
	}
```

**Proof.** The `find` occurs once in main.go (:270).

```json
{"id":"G2-146","file":"tools/apipartition/main.go","find":"sig += fmt.Sprintf(\" // iota %d\", specIndex)","replace":"sig += fmt.Sprintf(\" // iota %d\", specIndex+1)","test":"TestBreakitIotaSwapChangesTheSurface","pkg":"./tools/apipartition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-146-control","control":true,"test":"TestBreakitIotaSwapChangesTheSurface","pkg":"./tools/apipartition","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

**The test name in these entries is a placeholder.** I did not read the enclosing function's name above :28. The build agent replaces it with that name. The finding's evidence names only the file and line.

Why the plant (an off-by-one value) passes the old test:
- before: B "// iota 2", C "// iota 3"; after: C "// iota 2", B "// iota 3". So `before != after` still holds.
- "p B const const B T" is still present.
- So the old test stays green, and the new one is red because "// iota 1" is missing.

A plant that drops the marker entirely would already be red on the old test, through `before == after`, so it is not used.

### G2-156 (tools/shape/main.go:59-61)

**Code read.**
- `judge` (main.go:50-66) sets `Stage = "lowering"` when `parser.Parse` fails with a code in `parser.LoweringCodes` = `{"E-DUR-002","E-STR-002","E-TYPE-001","E-WIN-004"}` (internal/parser/errors.go:59).
- Where the parser raises each code:
  - E-DUR-002: literal.go:135-138, a count that is not a whole number.
  - E-STR-002: literal.go:228, `\%c is not an escape`.
  - E-TYPE-001: literal.go:110-113, an integer out of range.
  - E-WIN-004: lower.go:213-217, a sort size < 1.
- **internal/validate/testdata's e-type-001 files are not usable.** They are validator-stage faults (a derived `ts` type, text operators), which Parse accepts.
- **Source of the inputs:** the parser's own `TestLoweringCodes` rows ("a fractional count", "an unknown escape", "an integer out of range", "a zero sort size"; lowering_test.go:31, :38, :40, :44). Written out in full here.

**The change.** shape_test.go (package `main`) adds the import `"github.com/vorsar/cep/internal/parser"` (`slices` and `strings` are already imported) and:

```go
// TestJudgeReportsEachLoweringCodeAtTheLoweringStage: a grammar-valid file the lowering refuses is a reject at the
// "lowering" stage with its code, one row per parser.LoweringCodes code (inputs as the parser's TestLoweringCodes).
func TestJudgeReportsEachLoweringCodeAtTheLoweringStage(t *testing.T) {
	t.Parallel()
	const head = "language cep/0.1;\n" +
		"stream S (k string, n int, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
		"stream Out (k string, ts timestamp);\n" +
		"rule R intent 'x' partition by k of S ttl 1 hour from S"
	rows := []struct{ code, src string }{
		{"E-DUR-002", head + " select k output first every 1.5 min insert into Out;\n"},
		{"E-STR-002", head + "[k has_cs 'C:\u005cq'] select k insert into Out;\n"},
		{"E-TYPE-001", head + "[n > 99999999999999999999] select k insert into Out;\n"},
		{"E-WIN-004", head + " window sort(0, n, desc) emit on change late events drop select k insert into Out;\n"},
	}
	if !strings.Contains(rows[1].src, "'C:\\q'") {
		t.Fatal("test setup: the E-STR-002 input must hold a backslash before q")
	}
	var codes []string
	for _, row := range rows {
		codes = append(codes, row.code)
		r := judge("lowering.cep", row.src)
		if r.Verdict != "reject" || r.Code != row.code || r.Stage != "lowering" {
			t.Errorf("judge(the %s input): verdict %q, code %q, stage %q; want reject, %s, lowering", row.code, r.Verdict,
				r.Code, r.Stage, row.code)
		}
	}
	if !slices.Equal(codes, parser.LoweringCodes) {
		t.Errorf("the rows cover %v; want one per parser.LoweringCodes %v, in its order", codes, parser.LoweringCodes)
	}
}
```

Each `src` is byte-identical to the parser test's `program(...)` expansion, for example `rate("1.5 min")` = head + `" select k" + " output first every 1.5 min" + " insert into Out;\n"`. The setup check confirms that the `\u005c` input holds a real backslash.

**Proof.** The `find` occurs once in main.go.

```json
{"id":"G2-156","file":"tools/shape/main.go","find":"if slices.Contains(parser.LoweringCodes, r.Code) {","replace":"if false && slices.Contains(parser.LoweringCodes, r.Code) {","old_test":"TestShapeCorpusGolden","new_test":"TestJudgeReportsEachLoweringCodeAtTheLoweringStage","pkg":"./tools/shape","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-156-control","control":true,"test":"TestJudgeReportsEachLoweringCodeAtTheLoweringStage","pkg":"./tools/shape","expect":{"old":"invalid","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

The control's old outcome is "invalid" because the test does not exist at the lane base. The corpus golden has no lowering row, so `TestShapeCorpusGolden` stays green under the plant.

### Commit plan

Unchanged: G2-83, G2-84 and G2-106's partition part in E2-5; G2-143 in E2-6; G2-145 and G2-146 in E2-7; G2-156 in E2-8. The lane stays at 8 commits.

E2-1 still depends on lane E1's G10-1 seam (`App.Loads`, `TableLoad`, the apply in `router.activate`), which E1 is adding to its note.


## Revision 1, after the blind reviews

## E2 design note: revision after the blind reviews (replacement sections and change list)

Same rules as before: read-only, nothing run. One thing differs from what the review and your request assume:

- **DA-S3's example cannot reach Deploy's row check.** An optional key column is refused by the validator before any starting row is built (E-TBL-002, internal/validate/names.go:72-83). Every other refusal in the table's own check is already a Deploy refusal in the facade. So no Deploy input reaches E1's Add check.
- So I did not choose the recommended facade call to the table's Check, and the "deterministic table-refusal trigger" is not one of B's reasons. Details and the test I added instead are in the G10-1 section below.

### Change list

1. **DA-S3:**
   - The claim at :78 is corrected.
   - The review's trigger is shown unreachable, with file:line evidence.
   - The facade adds no table Check call; my reason is in "Starting rows the table refuses" below.
   - A new facade test pins the facade's wording for the reachable null-key input. Its plant shows the test goes red if Add's runtime wording ever becomes reachable.
   - Dry run: no existing pin moves.
2. **DA-L1:** a facade-only candidate (F) joins G10-1's table, with the reasons B is still chosen. The table-refusal trigger is not used, because it is unreachable.
3. **DA-S6:** G2-106 uses the repo's `Not parallel: <reason>` marker, and CD1's G2-73 §4 script is the check.
4. **DQ-L4:** G7a-1's commit also fixes deploy_test.go:232-233's doc, plus the failure messages at deploy_test.go:247 and coverage_s3_3_1_test.go:85.
5. **DQ-L5:** a per-entry `timeout` of `"300s"` on every entry in the root package (`.`) and io/inmemory, both of which link the whole engine, and on G2-143's repeat runs. Partition, apipartition and shape entries keep the default.
6. **The recording queue:** the risk paragraph is replaced. E1's DA-S1 sizes each app's recording queue at Add, so starting rows no longer end a recording.
7. **G2-146:** the real test name, `TestBreakitIotaRepetitionKeepsTypeAndValue` (breakit_s3_3_1_test.go:26), in the entry, its control and the test edit.

### G10-1: `AppHandle.Start` leaves an app half-started (apphandle.go:43-64). Replaces the whole section.

**1. Verdict: accepted.**

The code:
- :44 `Activate`.
- :47-51, for each table: `TableWrite(ctx, …)`, returning on the first error.
- :52-62: `Run` for the sources.

After a TableWrite failure the app is active, because `startAppGoroutines` (lifecycle.go:119-136) set `phaseActive`. A second Start gets `ErrStarted` (`activationRefusal`, lifecycle.go:110-120). The sources never open; Stop, Undeploy and a new Deploy are the only way back.

**How Start's TableWrite can fail (this paragraph corrects my earlier claim at :78).**
- **The context ending.** `enqueue` checks `ctx.Err()` first (requests.go:56-59), and `await` returns `ctx.Err()` (:78-86).
- **A concurrent Stop, Undeploy or Close:** `ErrDrained`, `ErrRemoved`, `ErrClosed`.
- **Not the rows' values.** TableWrite reaches the table's own check (internal/table/table.go:240-259, through the router's `a.write`). Each of its refusals is already a Deploy refusal:
  - **Wrong kind:** cannot happen. The facade's RowWriter sets each value's kind from the declaration (row.go:97).
  - **A null or unset non-optional value:** refused by the RowWriter (row.go:58-60 unset, :90-92 `SetNull`).
  - **A null key:** a key column is never optional. The validator refuses such a program first with E-TBL-002 (validate/names.go:72-83; fixture internal/validate/testdata/e-tbl-002-optional-key.cep). So a key falls under the previous rule.
  - **A non-finite float:** refused by `runtime.CheckRows` at Deploy (deploy.go:217-219; boundary.go:242-243).
  - **An object:** stored as its JSON text by `SetObject` (row.go:131-132), which the table accepts.
  - My earlier sentence, "CheckRows cannot fail at Start because Deploy runs the same check", named only one of these checks. The full reason is the list above.

**Choice (the note decides; plan §5).**

Input: an app with `WithTableRows`, and a Start whose context ends after the activation is queued but before a table write is queued.

| Candidate | What it does | Verdict |
|---|---|---|
| A. Roll back in the facade (Drain on failure) | undo the activation | **Rejected.** With the expired ctx, `Drain` reverts to active (drain.go:22-35, :70-80). With `context.WithoutCancel`, the barrier can wait forever on a stuck sink. |
| **B. Rows travel in `runtime.App.Loads`; the router applies them in the activation step** | activation and starting rows are one router step, awaited regardless of ctx once queued (`requestQueued`, lifecycle.go:64-75) | **Chosen** |
| F. Facade only: the starting-row writes run under `context.WithoutCancel(ctx)`, and the rows are checked at Deploy with the table's own check | no runtime change and no new internal API | **Rejected** (reasons below) |
| D. Resumable Start | mutex and progress state in AppHandle | Fallback; breaks the handle's immutability and duplicates table records |
| E. Document only | – | Last resort; the dead end stays |

**Why B over F:**
1. **F leaves a race open; B closes it by construction.** Under F the app is active (`gate` passes, requests.go:37-55) before the writes, so a concurrent `Input(...).Send` or `Table` write is accepted against a table without its starting rows. B applies the rows in the step that makes the app active.
2. **F makes Start ignore its ctx.** Under F, TableWrite's `enqueue` blocks on `r.intake <- it` while the intake is backpressured (requests.go:60-62), and `await` then waits only for the router or Close. So a Start with a deadline can block past it, while every other facade call respects ctx. B's only blocking step, the activation's enqueue, still respects ctx.
3. **F's Deploy-time table check would be dead code.** As shown above, every refusal of the table's check is already a Deploy refusal, so no input reaches that branch and no test can prove it (ground rule 9).

**The table-refusal trigger is not a reason for B,** because it is unreachable (see the corrected paragraph).

**B's cost, for the owner at G1:** it moves responsibility from the facade to the runtime across packages. That means three commits across three agents (E1's seam, `table.Check`, E2-1); new internal API (`App.Loads`, `TableLoad`, `table.Check`); three new runtime texts; and a public behaviour change (below).

**The runtime seam:** lane E1, as its addendum specifies (`App.Loads`, `TableLoad`, Condition A's check at Add, the apply in `router.activate`). E1's DA-S1 sizes each app's recording queue at Add, so starting rows never end a recording. My earlier queue-risk paragraph is withdrawn.

**2. Kind (facade half):** K1. Start and Deploy are not per-event: the GitNexus upstream of Start is tests only.

**3. The exact change (E2-1, after E1's seam lands):**
- deploy.go:203-225: build `[]runtime.TableLoad{Table: tid, Rows: rows}` in table order (the loop at :209 is already in table order) and set `Loads` on the `runtime.App` passed to `r.rt.Add` at :109. Keep the RowWriter and `CheckRows` checks and their facade wording (:212-214, :219-221).
- Remove `tableLoad` (deploy.go:236-240) and `AppHandle.tableRows` (apphandle.go:24).
- Delete the loop at apphandle.go:47-51.
- Start's doc (:41-42) becomes:

  `// Start activates the app: its sinks' flushers, its recorder (the header first) and its tables' starting rows, in one step of the engine; then it opens the sources, each on a goroutine the engine starts and stops. An error before the activation leaves the app as it was, and Start may be called again; once activated, Start fails only when a Stop, Undeploy or Close runs at the same time.`

- New test in lifecycle_test.go: `endsAfterFirstCheck` and `TestAStartWhoseContextEndsMidwayLeavesTheAppStartable`, unchanged from the note.
- TestBreakitTableRowsAreWrittenInTableOrder (breakit_s3_3_1_test.go:219) still holds: the loads are applied, and their records written, in table order.

**4. Proof (K1):**
- The revert plant (`--patch`) re-adds `tableRows`/`tableLoad` and the Start loop, and stops setting `Loads`. It keeps E1's declarations.
- Expected: **red** at the second-Start `t.Fatalf`.
- Per-entry `"timeout": "300s"`.

**5. Commit:** E2-1, root K1.

#### Starting rows the table refuses (DA-S3): the choice and its test

Input: a starting row that the table's own check would refuse.

Candidates:
- (i) wrap Add's error at deploy.go:109-113;
- (ii) call `(*table.Table).Check(rows)` in the facade before Add, worded `cep: deploy %s: WithTableRows(%q): %w`;
- (iii) no new facade call: Deploy's existing checks refuse every such row in the facade's own wording, and E1's Add check stays as a runtime-side guard that no Deploy input reaches.

**Chosen: (iii).**
- The superset argument above shows (i) and (ii) would add a branch no Deploy input reaches, with no red test and an uncovered line.
- (i) would also need a way to tell the rows' error apart from Add's other errors.
- What the review cares about is that Deploy's wording stays the facade's. That is pinned by a test that goes red exactly when the runtime wording becomes reachable (the plant below).
- If the coordinator or owner prefers (ii) anyway: put it after deploy.go:221, wrapping the table's text: `cep: deploy tbl: WithTableRows("T"): table T: row 0: attribute 0 "k": a key value cannot be null`. The same test and plant still apply.

**Dry run:** no Deploy text changes, so no pin moves.
- deploy_test.go:315 ("k" unset; G2-4 makes it `"\"k\""`) is unchanged.
- breakit_s3_3_1_test.go:169 accepts any Deploy refusal ("refused at Deploy: the contract").
- No test pins "runtime: app" through Deploy (`git grep -n 'WithTableRows(' -- '*_test.go'`: six uses, none on Add's text).

**New test in deploy_test.go** (in E2-1, beside G10-1's test):

```go
// TestAStartingRowWithANullKeyFailsDeployWithTheFacadesText: a null key in a starting row is refused by Deploy, in
// the facade's WithTableRows wording, before the engine sees the row.
func TestAStartingRowWithANullKeyFailsDeployWithTheFacadesText(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	_, bind := fcSink("O")
	nullKey := func(w cepio.RowWriter) error {
		w.SetNull(w.AttrIndex("k"))
		w.SetInt(w.AttrIndex("v"), 1)
		return nil
	}
	_, err := r.Deploy(ctx, "tbl", fcTableSrc, bind, cep.WithTableRows("T", nullKey))
	want := `cep: deploy tbl: WithTableRows("T"): cep: attribute "k" is not optional; SetNull was called`
	if err == nil || err.Error() != want {
		t.Errorf("Deploy with a null key in a starting row: %v; want %q", err, want)
	}
}
```

Where the expected text comes from: deploy.go:212-214 wraps `t.rows`'s error (table.go:36-46); `row.run` returns the first `fail` (row.go:36-47, :104-108), here row.go:91.

**Proof.** The `find` occurs once in row.go (counted).

```json
{"id":"G10-1-deploy-wording","file":"row.go","find":"if v.Null && !a.Optional {","replace":"if v.Null && !a.Optional && false {","old_test":"TestWithTableRowsAreVisibleToTheFirstEvent","new_test":"TestAStartingRowWithANullKeyFailsDeployWithTheFacadesText","pkg":".","timeout":"300s","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G10-1-deploy-wording-control","control":true,"test":"TestAStartingRowWithANullKeyFailsDeployWithTheFacadesText","pkg":".","timeout":"300s","expect":{"old":"invalid","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

Why the outcomes are as stated:
- **The new test is red under the plant.** The null key passes the RowWriter and `CheckRows`, so it reaches Add. E1's Condition A check refuses it in runtime wording, or, without that check, Start fails later and Deploy returns nil. Either way the assertion fails.
- **The old test stays green.** Its bad row leaves k unset, which row.go:58-60 refuses, and its main row sets k.

### G7a-1: connector errors name sprint ids. Replaces item 3's test list.

Item 3 adds these test-file edits to E2-1:
- deploy_test.go:232-233, the doc. Old:
  ```
  // TestUnbuiltConnectorAnnotation (T20, as §9 amends it; owner's design 5): example 01's @sink(type = 'http') is
  // refused naming http and S5.1 without a Go sink, and deploys with WithSink on that stream.
  ```
  New:
  ```
  // TestUnbuiltConnectorAnnotation (T20, as §9 amends it; owner's design 5): example 01's @sink(type = 'http') is
  // refused without a Go sink, naming http as not available in this release, and deploys with WithSink on that stream.
  ```
- deploy_test.go:246-247:
  ```go
  	if want := `@sink(type = "http") is not available in this release; bind a Go sink with WithSink`; !errors.Is(err, cep.ErrConnectorNotBuilt) || !strings.Contains(err.Error(), want) {
  		t.Errorf("an @sink(type = 'http') program without a Go sink: %v; want ErrConnectorNotBuilt with %q", err, want)
  ```
- coverage_s3_3_1_test.go:84-85:
  ```go
  	if _, err := r.Deploy(ctx, "ann", src); !errors.Is(err, cep.ErrConnectorNotBuilt) || !strings.Contains(err.Error(), "is not available in this release") {
  		t.Errorf("default factory: %v; want ErrConnectorNotBuilt saying the connector is not available in this release", err)
  ```

**Not touched:** conformance/factory_test.go:27 ("factory until S5.1") is CD's file and a statement about the plan, not about the text. sink.go:25 is an unexported comment the change does not reach.

**Proof:** the revert plant is unchanged, with per-entry `"timeout": "300s"`.

### G2-106 (partition part). Replaces items 3 and 4.

**3. The change:**
- `t.Parallel()` becomes the first statement of `TestBodyPanicResetsTheKey` (panic_test.go:123), `TestPanicInOnPurgeStillPurges` (:173) and `TestPendingExcludesTTL` (:204). Each builds its own body and Rule; no package-level mutable state exists.
- `TestOneTTLTimerPerInstance_ReArmAllocatesNothing` (ttl_test.go:122) gains a last doc line, `// Not parallel: testing.AllocsPerRun counts the whole process's allocations.`, the repo's marker with the wording of rfc3339_operands_test.go:86.

**4. Proof (K2e):** CD1's G2-73 §4 script, the one shared check. Run with `python3.15 -I <script> <pairs>` on these pairs:
- `internal/partition/ttl_test.go:TestOneTTLTimerPerInstance_ReArmAllocatesNothing`
- `internal/partition/panic_test.go:TestBodyPanicResetsTheKey`
- `internal/partition/panic_test.go:TestPanicInOnPurgeStillPurges`
- `internal/partition/panic_test.go:TestPendingExcludesTTL`

Expected: old **4**, exit 1; new **0**, exit 0.

**5. Commit:** E2-5.

### G2-146. Replaces the test edit and entries.

The test edit is in `TestBreakitIotaRepetitionKeepsTypeAndValue` (tools/apipartition/breakit_s3_3_1_test.go:26); the replaced lines are :33-35, as before.

```json
{"id":"G2-146","file":"tools/apipartition/main.go","find":"sig += fmt.Sprintf(\" // iota %d\", specIndex)","replace":"sig += fmt.Sprintf(\" // iota %d\", specIndex+1)","test":"TestBreakitIotaRepetitionKeepsTypeAndValue","pkg":"./tools/apipartition","expect":{"old":"green","new":"red"},"old_rev":"<lane base>","new_rev":"<this commit>"}
{"id":"G2-146-control","control":true,"test":"TestBreakitIotaRepetitionKeepsTypeAndValue","pkg":"./tools/apipartition","expect":{"old":"green","new":"green"},"old_rev":"<lane base>","new_rev":"<this commit>"}
```

### Timeouts (DQ-L5)

Add `"timeout": "300s"` (a Go duration string, as mutate.py's `--timeout`, default "120s" at :218) to every E2 entry whose `pkg` is `.` or `./io/inmemory`:
- G10-1, G7a-1, G2-1, G2-2 (×4), G2-3, G2-4 (×2), G10-1-deploy-wording, and their controls;
- G2-141, G2-143, and their controls.

G2-143's repeat commands add `--timeout 300s`. All entries in `./internal/partition`, `./tools/apipartition` and `./tools/shape` keep the default.

These are estimates from package size; no run was made.

### Replacement rows for §5 (choices) and §6 (risks)

| Id | Input | Candidates | Chosen | Reason | Dry run |
|---|---|---|---|---|---|
| G10-1 | Start's ctx ends between the activation and a table write | A, B, F, D, E (above) | B (E1's seam) | closes the concurrent-Send window by construction; Start keeps respecting ctx; F's Deploy-time check would be unreachable | behaviour only; lifecycle_test.go:49's ErrStarted after a successful Start still holds |
| G10-1 (DA-S3) | a starting row the table would refuse | (i) wrap Add's error; (ii) facade table Check; (iii) none, with the wording pinned | (iii) | the facade already refuses every such row (RowWriter, CheckRows, E-TBL-002); (i) and (ii) would add an unreachable branch | no pin moves |

§6, the recording-queue risk: withdrawn. E1's DA-S1 sizes each app's recording queue at Add.

---

### Coordinator corrections after the second QA pass (2026-11-20; evidence: review-design-qa2.md)

1. **G2-156's control (DQ2-L2).** Give it `old_test: TestShapeCorpusGolden` and `new_test: TestJudgeReportsEachLoweringCodeAtTheLoweringStage`, expecting green/green, the same shape as the plant entry. A control shows the real code green at both sides.
2. **G2-143's commands (DQ2-L6).** Both `mutate.py` and `load.py` run under `python3.15 -I -B` (mutate.py:3-5), never bare `python3.15`.
3. **G2-146:** the real test name `TestBreakitIotaRepetitionKeepsTypeAndValue` is already in this revision (pre-check 2 confirmed green under it).
