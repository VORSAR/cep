# S1.1c-7 design note — E1 (internal/runtime)

*S1.1c-7's design note for chunk E1 (internal/runtime), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes.*


## The note

## S1.1c-7 design note: chunk E1 (`internal/runtime`)

### 1. Header

- **Chunk:** E1. **Findings:** 25: G10-20, G10-21, G7a-5, and G2-111 to G2-132.
- **Read in full or in the relevant part:**
  - production: drain.go, app.go, lifecycle.go, requests.go, router_in.go, router.go (broadcast), sync.go, boundary.go, runtime.go (errors), build.go (checkRuleInputs), keys.go, recorder.go (run, put, drainStop, cut), shutdown.go (Remove, Close, shutDown), transitions.go (remove, activate);
  - tests: every test file the findings name, plus helpers_test.go, close_gate_test.go, lifecycle_races_internal_test.go, boundary_checks_internal_test.go and build_validate_internal_test.go:76;
  - outside the package: docs/sprints/s3-3-1-design.md (§2, MUST 11, T30), known issue S3.3-1-3, internal/playback/records.go:271-281, and the root batch_test.go and helpers_test.go (read only).
- **GitNexus calls: 7.** impact BatchEnd, context queueDrain, impact checkValue, context broadcast, a cypher query on who accesses `waited`, impact Drain, context shardOf.
- **What I could not check:**
  - The MCP resource `gitnexus://repo/cep/context` could not be read (that tool is not available here). I relied on the brief's statement that the index is fresh at d521725. Every "no caller" claim below is also backed by grep.
  - I did not run anything: no tests, mutate.py, load.py or compare.py (the brief forbids it). Every outcome below is a prediction from reading the code.
  - G2-115's note that breakit_test.go:388 "may have the same blind spot" was not checked; it is forwarded.
  - I went past the 40-minute box. Every finding is covered.

### 2. Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-20 | accepted (a real data race) | K1c | C5 | – | `waited` is read and the drain reverted under `runMu` through a new helper `revertUnlessQuieted` |
| G10-21 | accepted in part | K1h | C6 | the counting rule is forwarded to S3.3-1-3's fix (needs the owner) | the drained path queues a BatchEnd only for a source stream; two comments corrected |
| G7a-5 | accepted | K1 (error-text exemption, named) | C4 | – | 11 messages lose their process ids; a new exact-text test |
| G2-111 | accepted | K2a | C2 | renames → S1.1c-8; merge and delete → S1.1c-10 | the Activate race checks the error identities and "never both" |
| G2-112 | accepted | K2a | C2 | rename and file rename → S1.1c-8; move → S1.1c-10 | `t.Logf` becomes `errors.Is(a.err, ErrClosed)` |
| G2-113 | accepted | K2-del | C1 | – | delete TestBkDuplicateInputDoublesEvents |
| G2-114 | accepted | K2b | C3 | – | the event is built with rtEv (3 values); the refusal text is checked |
| G2-115 | accepted | K2a | C3 | – | `Shards: 16` → `Shards: 3`; the probability sentence replaced |
| G2-116 | accepted | K2a | C2 | – | assert the recording is not Failed; a timed-out `cgUntil` fails the test |
| G2-117 | accepted | K2c | C2 | – | the 300 ms timeout becomes a cancel through `recordingWait` |
| G2-118 | accepted | K2a | C2 | – | after Drain and Stats, `cgUntil(ldRecorderIdle(self))` |
| G2-119 | accepted | – | – | carried in full → S1.1c-8 | rename only (the doc already names the known issue) |
| G2-120 | accepted | K2a | C3 | – | new test drives Sync.End (broadcast's third caller); doc updated |
| G2-121 | accepted | – | – | carried in full → S1.1c-8 | rename plus a doc comment |
| G2-122 | accepted | K2e | C3 | – | `&rtLog{}` → `newRTLog()` |
| G2-123 | accepted | – | – | carried in full → S1.1c-10 (a split) | – |
| G2-124 | accepted | – | – | carried in full → S1.1c-10 | – |
| G2-125 | accepted | K2a | C3 | – | the NaN refusal checks its text |
| G2-126 | accepted | – | – | carried in full → S1.1c-10 (helper refactor, about 20 callers) | – |
| G2-127 | accepted | K2a | C3 | – | resolve `time` by import path; doc attached and names AfterFunc |
| G2-128 | accepted | K2a | C2 | – | `err == nil` → `!errors.Is(err, context.Canceled)` |
| G2-129 | accepted | K2-del | C1 | – | delete the "no value" row |
| G2-130 | accepted | – | – | carried in full → S1.1c-10 | – |
| G2-131 | accepted, widened to a third site | K2e | C2 | – | `cgApp(t, …)` built before the `go` statement, at three sites |
| G2-132 | accepted | K2d | C3 | – | `context.Background()` → `ctx` |

### 3. Findings

#### G10-20: `appState.waited` read without `runMu`

**1. Verdict: accepted. It is a real data race, not just a wrong comment.**
- **What the code says:**
  - app.go:35: `runMu sync.Mutex // guards runs and waited`.
  - The only write is drain.go:119 `a.waited = true`, inside awaitRuns, which holds runMu (:112-113).
  - Unlocked reads: drain.go:28 `if !a.waited {` in Drain, after awaitRuns has returned and unlocked; and drain.go:80 in queueDrain.
- **GitNexus:** the cypher query on accesses to `waited` returned queueDrain (read), awaitRuns (write, read, read). It does not show Drain's read at :28; that one is grep-confirmed (drain.go:28). `context queueDrain` shows Drain as its only caller (exact).
- **The interleaving:**
  1. Drain A has an already-ended ctx; Drain B is live; same activated app.
  2. A locks runMu. `enqueue` returns `ctx.Err()` first (requests.go:57), so `waited` stays false. A unlocks.
  3. B locks runMu and writes `waited = true` (:119).
  4. A reads `a.waited` at :28 with no lock.
- **Why the race detector sees it:** A's unlock happens before B's lock, but A's read comes after that unlock. Nothing orders A's read against B's write.
  - In one ordering there *is* an order: A's revert CAS on `phase` (after the read) is acquired by B's markDraining CAS. That needs A to revert before B moves the phase.
  - So the race shows when B's markDraining comes before A's revert and A takes runMu first. That is reachable through the public Drain alone.
- **:80 is the same race, harder to reach:** A read `started` false at :25, an in-flight Activate sets `started`, and B awaits.

**2. Kind: K1c (a data race).** Drain is not per-event: impact on Drain upstream gives 1 direct caller on the Close path, plus 1 dropped by receiver typing (the facade's Stop). None of R5's entry points reaches it.

**3. The change** (production: drain.go only).

New helper after `revertDrain` (drain.go:70):
```go
// revertUnlessQuieted moves the app back to was unless a Drain has queued the quiet item. It reads waited under
// runMu, which awaitRuns holds while it queues that item and sets waited, so the read and the revert never race it.
func (a *appState) revertUnlessQuieted(was appPhase) {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	if !a.waited {
		a.revertDrain(was)
	}
}
```
- drain.go:28-30 `if !a.waited { // the quiet item … \n a.revertDrain(was)\n }` becomes `a.revertUnlessQuieted(was) // the quiet item was never queued: live, as before this Drain`.
- drain.go:80-82 becomes `a.revertUnlessQuieted(was)`.
- No lock-order risk: runMu is only ever taken before `inflight.mu` (Run, lifecycle.go:150-153). The helper takes runMu alone, and neither caller holds it.
- The comment at app.go:35 becomes true, and stays unchanged.

New test in lifecycle_races_internal_test.go:
```go
// Two Drains of one active app at once, one whose ctx has already ended: the ended one returns context.Canceled, the
// other drains the app, and input is refused after both, whichever Drain takes the run lock first.
func TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range ldRounds {
		id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
		live, ended := ldRace(t, ctx, func() error { _, err := r.Drain(ctx, id); return err },
			func() error { _, err := r.Drain(ldExpired(), id); return err })
		if live != nil {
			t.Fatalf("the live Drain: %v; want nil", live)
		}
		if !errors.Is(ended, context.Canceled) {
			t.Fatalf("the Drain whose ctx had ended: %v; want context.Canceled", ended)
		}
		if err := r.Send(ctx, id, cgEv(1)); !errors.Is(err, ErrDrained) {
			t.Fatalf("Send after both Drains: %v; want ErrDrained", err)
		}
	}
}
```
Every ordering ends with Send refused: either the phase is draining, or the `draining` flag B set refuses it (requests.go:50).

**4. The proof:**
- **Revert plant `g10-20-revert`:** a `--patch` with two hunks restoring the old inline code. The helper stays declared.
  ```
  --- a/internal/runtime/drain.go
  +++ b/internal/runtime/drain.go
  @@
   		if err != nil {
  -			a.revertUnlessQuieted(was) // the quiet item was never queued: live, as before this Drain
  +			if !a.waited { // the quiet item was never queued: live, as before this Drain
  +				a.revertDrain(was)
  +			}
   			return DrainResult{Runs: runs}, err
  @@
   			a.draining.Store(false)
  -			a.revertUnlessQuieted(was)
  +			if !a.waited {
  +				a.revertDrain(was)
  +			}
   		}
  ```
- **Red:** `mutate.py --race --repeat 50 --load --patch <plant> --test '^TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained$' --pkg ./internal/runtime`. Expected: k ≥ 1 red runs out of 50, from the race report "WARNING: DATA RACE" between drain.go:119 (write) and the restored read. Record k.
- No hold is used: a hold would create the ordering that hides the race.
- **Green:** `load.py -- -race -count=50 -run '^TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained$' ./internal/runtime`, 50/50.
- **Pre-mortem:** k could be 0 if B's markDraining rarely comes before A's revert. That is why the test runs 64 rounds (`ldRounds`). If k = 0, the lane reports it and does not switch to a hold.

**5. Commit:** C5 (K1c).

**Residual, not fixed (a new item for the backlog, rule 7):** the logic window stays. A Drain whose quiet item failed can still put the phase back to active after another Drain has passed markDraining. The `draining` flag still refuses input, so no input is lost.

#### G10-21: a drained BatchEnd is counted, a drained Send is not

**1. Verdict: accepted in part.**
- **What the code does:**
  - lifecycle.go:177-183: a BatchEnd that `gate` refuses with ErrDrained is still queued as `iBatchEnd`.
  - router_in.go:36-37 (`inputApp`) counts it in `after` once the router has drained.
  - Send (boundary.go:152-155) returns at `gate` and queues nothing.
  - The two comments are wrong: lifecycle.go:172 "as an event is" and :178 "as a late event is". A refused Send is never counted. Only an event that passed `gate` and reached the router after the drain is counted.
- **The Derived gap is real and worse than the survey says:**
  - The drained branch (:179) has no Derived check.
  - Between Stop and the router's drain item, a queued derived-stream batch end is taken as live: counted in `Batches` and written as `rBatchEnd` (router_in.go:49-50).
  - The playback reader refuses that record: records.go:277 `"stream %q is a rule's output; a log holds source streams only"`. So the recording can no longer be replayed. That is a correctness defect.
- **GitNexus:** impact on BatchEnd upstream: 3 direct callers, epistemic exact. BatchEnd is an R5 entry point (the facade's BatchEnd).

**2. Kind: K1h** (per-event entry point).

**3. The choice (undecided behaviour):**
- **Input:** a BatchEnd after the app's drain.
- **The candidates:**
  - **(A) Count refused Sends too.** The Send path would queue a count item. Without a new item kind, a refused event that beats the drain item would be processed as live, which is a correctness bug. It also adds work to Send's refused path. Rejected.
  - **(B) Stop queuing any drained BatchEnd.** One rule for every input kind, and it closes known issue S3.3-1-3. But it contradicts design s3-3-1 MUST 11 (s3-3-1-design.md:190, "counted in AfterDrain after the drain") and T30 (:777, "EndBatch after Stop → ErrStopped and AfterDrain counts it (owner, design 6)"). It also breaks five pins:
    - root batch_test.go:47;
    - root helpers_test.go:57-75 (`fcAwaitDrainRan`, whose own doc says S3.3-1-3's fix must give it a new signal);
    - internal/runtime lifecycle_test.go:724 and :759;
    - S3.3-1-3's row in gate/known-issues.json (the coordinator's file), whose target is S8.
  - **(C) Keep the designed counting** for source streams, fix the comments, and stop queuing a derived stream's drained BatchEnd.
- **My choice: (C).**
  - The counting rule is decided by a design and an owner-tagged test row, so changing it needs the owner.
  - (B) needs E2's root files and the coordinator's file, which would break the ownership table (§4).
  - (C) fixes the part no design decides (a derived stream takes no batch end) and the user-visible defect (a recording that cannot be read).
  - **Recommendation for the owner:** adopt (B) as S3.3-1-3's fix when that is scheduled.
- **Error for a derived stream after the drain:** stays ErrDrained. The design's order at :166 puts the draining check before the stream check, and `gate` is "every entry point's caller-side check".

**The code** (lifecycle.go:170-183):
```go
// BatchEnd hands a source's batch-close signal for stream to the intake, behind the events before it (owner,
// 2026-11-07, design 6). An empty batch id is an error (the recording's reader refuses one). Once the app's drain has
// begun it is refused with ErrDrained; a source stream's is still queued, and the router counts it in AfterDrain once
// it has run the drain. A refused Send is not queued, so it is not counted.
func (r *Runtime) BatchEnd(ctx context.Context, app AppID, stream int, batch string) error {
	…
	a, err := r.gate(app)
	if errors.Is(err, ErrDrained) { // refused; a source stream's is queued, and the router counts it once drained
		if a, err2 := r.appOf(app); err2 == nil && stream >= 0 && stream < len(a.app.Streams) &&
			!a.app.Streams[stream].Derived {
			_ = r.enqueue(ctx, item{kind: iBatchEnd, app: app, stream: stream, batch: batch})
		}
		return err
	}
```
The accepted path is byte-unchanged. `gate`, `ingest` and sync.go are not touched.

**The red test**, in lifecycle_test.go after TestRecorderRecordsBatchEnd. lcApp's stream 1 is derived (:713). The intake is FIFO, so Stats comes after both BatchEnds.
```go
// A derived stream takes no batch end: after the drain one is refused with ErrDrained, as every input is, and
// AfterDrain stays 0 on that stream; the source stream's refused batch end is still counted there.
func TestABatchEndOnADerivedStreamAfterTheDrainIsNotCounted(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 1})
	id := lcStarted(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
	rtDrain(t, ctx, r, id)
	if err := r.BatchEnd(ctx, id, 1, "d1"); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("BatchEnd on the derived stream after the drain: %v; want ErrDrained", err)
	}
	if err := r.BatchEnd(ctx, id, 0, "s1"); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("BatchEnd on the source stream after the drain: %v; want ErrDrained", err)
	}
	st := rtStats(t, ctx, r).Streams[id]
	if st[1].AfterDrain != 0 || st[0].AfterDrain != 1 {
		t.Errorf("AfterDrain derived %d, source %d; want 0 and 1", st[1].AfterDrain, st[0].AfterDrain)
	}
}
```

**4. The proof:**
- **Revert plant `g10-21-revert`:** file `internal/runtime/lifecycle.go`.
  - find `"stream < len(a.app.Streams) &&\n\t\t\t!a.app.Streams[stream].Derived {"`
  - replace `"stream < len(a.app.Streams) {"`
  - Expected: **red** at the AfterDrain `t.Errorf` ("AfterDrain derived 1, source 1; want 0 and 1").
- **Timing on mains power** (sprint model §10):
  1. `pmset -g batt` must read "AC Power".
  2. Build the parent's test binary in a detached worktree (`go test -c -o <tmp>/before.test ./internal/runtime`) and the commit's (`<tmp>/after.test`).
  3. Run `python3.15 -I -B gate/bench/compare.py --before <tmp>/before.test --after <tmp>/after.test --bench '^(BenchmarkSendPath|BenchmarkSendPayloadPath)$' --dir internal/runtime --pairs 10`.
  4. `pmset -g batt` again. A session that ran on battery is discarded.
  - Expected: exit 0, "no cost" (Send's path is not edited).
  - Exit 1: ground rule 13a applies and the lane stops.
- **T2 green.** Existing pins stay green: lifecycle_test.go:724 and :759, root batch_test.go:47, and fcAwaitDrainRan (source streams only).

**5. Commit:** C6 (K1h).

#### G7a-5: error text with process ids

**1. Verdict: accepted.** Every site was re-read:
- runtime.go:84;
- boundary.go:177-180, :183-185, :199, :214, :221, :243, :246, :250;
- build.go:159;
- requests.go:117-119.
- GitNexus: impact on checkValue upstream reaches checkValues, then checkEvent and Send (3, LOW); Playback is in the affected modules.

**2. Kind: K1.** This is the named R5 exemption: only the text of error branches changes in checkEvent, checkValues, checkValue, CheckRows, Tick, the ErrDrained sentinel and checkRuleInputs. No condition or control flow changes.

**3. The new wording** (verbs and arguments unchanged unless shown):

| Site | New text |
|---|---|
| runtime.go:84 | `"runtime: the app is drained: its input has ended"` |
| boundary.go:177-180 | `"runtime: app %d stream %q: Ts %d differs from the time attribute %q, %d: an event-time stream's time is its attribute"` |
| :183-185 | `"runtime: app %d stream %q: the event could not fit one recording line of %d bytes"` |
| :199 | `"runtime: app %d stream %q: attribute %q %v"` |
| :214 | `"runtime: app %d stream %q: attribute %q is null, and it is not optional"` |
| :221 | `"runtime: app %d stream %q: attribute %q is %v: NaN and infinities are never values"` |
| :243 | `"row %d: attribute %s: the float %v is not finite"` |
| :246 | `"row %d: attribute %s %v"` |
| :250 | `"row %d could not fit one recording line of %d bytes"` |
| build.go:159 | `"input %d is listed twice"` |
| requests.go:117-119 | `"runtime: Tick at %d ms: outside the years 0000-9999, which a recording cannot write"` |

**The dry run.** I grepped every `*.go`, `*.py`, `*.json` and `*.md` outside docs/sprints and docs/reviews for each old fragment. No test pins a parenthetical. These pins match only the part that stays, so they remain green:
- boundary_checks_internal_test.go:111, :153-155, :184, :202, :353;
- build_validate_internal_test.go:76 ("input 0 is listed twice").

No `// Output:` line and no doc quotes these texts.

**The test** (boundary_checks_internal_test.go; add imports `fmt` and `unicode/utf8`):
```go
// Each refusal at the Go boundary states its rule and nothing else: no sprint, review or decision reference.
func TestBoundaryRefusalsStateTheirRuleWithNoProcessReference(t *testing.T) {
	t.Parallel()
	bad, long := "a\xffb", strings.Repeat("a", event.MaxLogLine/6+1)
	if utf8.ValidString(bad) {
		t.Fatal("the malformed input is valid UTF-8")
	}
	send := func(edit func(e *event.Event)) error {
		e := bcEv(1)
		edit(&e)
		return bcBuild(t, 3, nil).Send(cgCtx(t), 0, e)
	}
	str := []event.Attr{{Name: "x", Type: event.TString}}
	twice := cgApp(t, func(event.Event) error { return nil })
	twice.Rules[0].Inputs = []int{0, 0}
	u8 := " holds malformed UTF-8, which a recording would replace with U+FFFD"
	for name, c := range map[string]struct {
		err    error
		want   string
		suffix bool
	}{
		"drained": {ErrDrained, "runtime: the app is drained: its input has ended", false},
		"ts": {send(func(e *event.Event) { e.Ts = cgT0 }), fmt.Sprintf(`runtime: app 0 stream "s": Ts %d differs from the time attribute "ts", %d: an event-time stream's time is its attribute`, cgT0, cgT0+1), false},
		"line": {send(func(e *event.Event) { e.Values[1].Str = long }), fmt.Sprintf(`runtime: app 0 stream "s": the event could not fit one recording line of %d bytes`, event.MaxLogLine), false},
		"unrecordable": {send(func(e *event.Event) { e.Values[1].Str = bad }), `runtime: app 0 stream "s": attribute "o"` + u8, false},
		"required": {send(func(e *event.Event) { e.Values[0] = event.Value{Kind: event.TString, Null: true} }), `runtime: app 0 stream "s": attribute "k" is null, and it is not optional`, false},
		"NaN": {send(func(e *event.Event) { e.Values[2] = event.Value{Kind: event.TFloat, F: math.NaN()} }), `runtime: app 0 stream "s": attribute "f" is NaN: NaN and infinities are never values`, false},
		"row float": {CheckRows("T", []event.Attr{{Name: "x", Type: event.TFloat}}, [][]event.Value{{{Kind: event.TFloat, F: math.Inf(1)}}}), `row 0: attribute 0 "x": the float +Inf is not finite`, false},
		"row unrecordable": {CheckRows("T", str, [][]event.Value{{{Kind: event.TString, Str: bad}}}), `row 0: attribute 0 "x"` + u8, false},
		"row line": {CheckRows("T", str, [][]event.Value{{{Kind: event.TString, Str: long}}}), fmt.Sprintf("row 0 could not fit one recording line of %d bytes", event.MaxLogLine), false},
		"tick": {bcBuild(t, 3, nil).Tick(cgCtx(t), math.MaxInt64), "runtime: Tick at 9223372036854775807 ms: outside the years 0000-9999, which a recording cannot write", false},
		"input twice": {validateApp(0, &twice), ": input 0 is listed twice", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			switch {
			case c.err == nil:
				t.Fatalf("no error; want %q", c.want)
			case c.suffix && !strings.HasSuffix(c.err.Error(), c.want), !c.suffix && c.err.Error() != c.want:
				t.Errorf("error %q; want %q", c.err.Error(), c.want)
			}
		})
	}
}
```
- If cgApp with `Inputs [0, 0]` is refused earlier by another check, use build_validate_internal_test.go:76's fixture and mutation instead; that row already reaches "input 0 is listed twice".
- The checks go through these values in order. Each input differs from the valid `bcEv(1)` in one value only, so it reaches the intended branch.

**4. The proof:**
- **Revert plant `g7a-5-revert`:** a `--patch` restoring the 11 old strings in runtime.go, boundary.go, build.go and requests.go. Its hunks are the exact inverse of the fix's diff.
- Expected: **red**. Every subtest fails at `t.Errorf("error %q; want %q", …)`. Quote the "drained" subtest's line.
- **T2 green.**

**5. Commit:** C4 (K1).

#### G2-111: the Activate race checks only "not both nil"

**1. Verdict: accepted.**
- breakit_s3_3_1_test.go:52-79 checks only `aerr == nil && rerr == nil`.
- Its doc promises ErrActive / ErrRemoved.
- The real refusals:
  - Activate: ErrRemoved (requests.go:30-31, lifecycle.go:116, transitions.go:80-81);
  - Remove: ErrActive (shutdown.go:24) or ErrDrainIncomplete (transitions.go:61-62).
- These match lifecycle_races_internal_test.go:41-59.

**2. Kind: K2a.**

**3. The change.** After the existing both-nil check (:73-75), add:
```go
		if aerr != nil && !errors.Is(aerr, runtime.ErrRemoved) {
			t.Fatalf("iteration %d: Activate racing Remove: %v; want nil or ErrRemoved", i, aerr)
		}
		if rerr != nil && !errors.Is(rerr, runtime.ErrActive) && !errors.Is(rerr, runtime.ErrDrainIncomplete) {
			t.Fatalf("iteration %d: Remove racing Activate: %v; want nil, ErrActive or ErrDrainIncomplete", i, rerr)
		}
		if aerr != nil && rerr != nil {
			t.Fatalf("iteration %d: both refused: Activate %v, Remove %v", i, aerr, rerr)
		}
```
The doc's "Remove refuses it with ErrActive" becomes "Remove refuses it with ErrActive, or ErrDrainIncomplete from the router once it activated the app first".

**4. The proof:**
```json
{"id":"g2-111","file":"internal/runtime/transitions.go","find":"a.removing:\n\t\tit.reply <- reply{err: ErrRemoved}","replace":"a.removing:\n\t\tit.reply <- reply{err: ErrStarted}","test":"TestBreakitActivateRacesRemove","pkg":"./internal/runtime","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
{"id":"g2-111-control","control":true,"test":"TestBreakitActivateRacesRemove","pkg":"./internal/runtime","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
```
- Why red is near certain: the router path is taken whenever iRemove is queued before iActivate, which happens in roughly 40 % of the 60 iterations.
- **Load:** `load.py -- -race -count=50 -run '^TestBreakitActivateRacesRemove$' ./internal/runtime`, 50/50.

**5. Commit:** C2.

**Carried:**
- To S1.1c-8:
  - `TestBreakitTwoRemovesAtOnce` → `TestTwoRemovesOfADrainedAppAtOnceReturnNilOrErrRemoved` ("return nil or ErrRemoved" ← the errs loop, :38-42);
  - `TestBreakitActivateRacesRemove` → `TestActivateAndRemoveOfAnInactiveAppAtOnceNeverBothSucceedOrBothRefuse` ("never both succeed" ← the both-nil check; "or both refuse" ← the new check);
  - the file name.
- To S1.1c-10: the merge into lifecycle_races_internal_test.go and the deletion of the weaker copy.

#### G2-112: the released Add is only logged

**1. Verdict: accepted.**
- breakit_s3_3_2_test.go:52 is a `t.Logf`, though the doc (:16) says the Add returns an error.
- close_deadline_internal_test.go:73 pins ErrClosed for the same release after Close has stopped the runtime.

**2. Kind: K2a.**

**3. The change.** Replace :52 with:
```go
	if !errors.Is(a.err, ErrClosed) {
		t.Errorf("the Add released after Close gave up: id %d, %v; want ErrClosed", a.id, a.err)
	}
```
- Also move `cgApp` out of the goroutine at :32 (see G2-131): `app := cgApp(t, func(event.Event) error { return nil })` before `go func() { id, err := r.Add(ctx, app); add <- added{id, err} }()`.

**4. The proof:**
```json
{"id":"g2-112","file":"internal/runtime/lifecycle.go","find":"\trep, err := r.requestQueued(ctx, item{kind: iAdd, add: &app})\n\tif err != nil {\n\t\treturn 0, err\n\t}","replace":"\trep, err := r.requestQueued(ctx, item{kind: iAdd, add: &app})\n\tif err != nil {\n\t\treturn 0, nil\n\t}","test":"TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled","pkg":"./internal/runtime","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}}
```
- Plus a control entry for the same test: green, green.
- **Load:** `load.py -- -race -count=50 -run '^TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled$' ./internal/runtime`, 50/50.

**5. Commit:** C2.

**Carried:**
- To S1.1c-8: `TestAnAddReleasedAfterCloseGaveUpIsNotLeftInstalled` ("released after Close gave up" ← the hold and Close's DeadlineExceeded; "is not left installed" ← the appList loop), plus the file's `_internal` name.
- To S1.1c-10: the move into close_gate_internal_test.go.

#### G2-113: a duplicate-input test that never reaches its check

**1. Verdict: accepted.**
- build.go:158-160 refuses a rule that lists one input twice, so breakit_test.go:478-480 always returns early.
- No plant can redden it: with the refusal removed, the body sees 2 events, which is exactly what the test accepts.
- build_validate_internal_test.go:76 pins the refusal and its text.

**2. Kind: K2-del.**

**3. The change:** delete breakit_test.go:463-485: the "---- red (later) …" banner, the doc and the function.

**4. The proof (covered set):**
- Run `go test -count=1 -coverpkg=./... -coverprofile=<p> ./internal/runtime` twice at the lane base and twice at C1.
- The set of covered blocks (count > 0) must be equal across all four profiles.
- C1 holds both deletions (G2-113 and G2-129), so one comparison covers both.

**5. Commit:** C1.

#### G2-114: the NaN test never reaches the NaN check

**1. Verdict: accepted.**
- `rtSchema` adds a `ts` attribute (helpers_test.go:73), so the schema has 3 attributes while the event at fixround_test.go:122 has 2 values.
- checkEvent refuses on the count (boundary.go:169-171) before checkValue.

**2. Kind: K2b.**

**3. The change.** At :122-125:
```go
		e := rtEv(0, rtAt(1), rtStr("a"), event.Value{Kind: event.TFloat, F: f})
		if err := r.Send(ctx, 0, e); err == nil || !strings.Contains(err.Error(), "NaN and infinities are never values") {
			t.Errorf("Send of the float %v: %v; want the non-finite refusal", f, err)
		}
```
Add `strings` to the imports if it is missing.

**4. The proof:**
- K2a entry: file `internal/runtime/boundary.go`, find `"\tcase nonFinite(v):\n"`, replace `"\tcase false && nonFinite(v):\n"`; test `TestSendRefusesNonFiniteFloat`; expect old green, new red. Plus a control entry: green, green.
- **Coverage:** the first statement of the branch body is boundary.go:221 (`return fmt.Errorf(`). The plan's ":222 / :221" is one line off at d521725.
  - Command: `go test -count=1 -run '^TestSendRefusesNonFiniteFloat$' -coverprofile=<p> ./internal/runtime`.
  - The block starting at boundary.go:221 has count 0 at the lane base and more than 0 at the commit. Quote both.
- **Load:** `load.py -- -race -count=50 -run '^TestSendRefusesNonFiniteFloat$' ./internal/runtime`, 50/50.

**5. Commit:** C3.

#### G2-115: 16 shards hide the −0.0 / +0.0 defect

**1. Verdict: accepted, and recomputed.**
- shardOf (keys.go:48-84) is FNV-1a over 8 bytes of app, 8 bytes of rule, the kind byte (TFloat = 3) and the float's bits.
- For app 0 and rules 0-3, the raw −0.0 and +0.0 land on these shard pairs:
  - at 16 shards: (2,2), (5,5), (12,12), (15,15), together for every rule;
  - at 3 shards: (0,1) for every rule, apart.
- GitNexus: shardOf's only caller is router.event (exact).

**2. Kind: K2a.**

**3. The change** (keys_test.go):
- :36 `runtime.Config{Shards: 16}` → `runtime.Config{Shards: 3}`.
- Replace the sentence at :17-19 with: "Shards 3: a router that hashed the raw −0.0 and +0.0 would put them on shards 0 and 1 for each of the four float rules, so the one-birth check catches it. At a power-of-two shard count up to 128, FNV-1a keeps the two zeros on one shard and the check could not."

**4. The proof:** a `--patch` plant that hashes the raw value and stores the normalised key.
```
--- a/internal/runtime/router_in.go
+++ b/internal/runtime/router_in.go
@@
-		sid := rt.shardOf(a.id, rr.rule, rr.attrs != nil, key)
+		sid := rt.shardOf(a.id, rr.rule, rr.attrs != nil, rawKey(rr.attrs, e.Values, key))
--- a/internal/runtime/keys.go
+++ b/internal/runtime/keys.go
@@
 	return key, true
 }
 
+func rawKey(attrs []int, values []event.Value, key partition.Key) partition.Key {
+	if attrs == nil {
+		return key
+	}
+	return partition.Single(values[attrs[0]])
+}
+
 // shardOf is fnv-1a over (app, rule, the key's parts in order): a key-less rule hashes (app, rule). A single-part key
```
- Test `TestKeyErrorsAndZeroKeys`: expect old green, new red (two births of key 0). Plus a control entry: green, green.
- **Load:** `load.py -- -race -count=50 -run '^TestKeyErrorsAndZeroKeys$' ./internal/runtime`, 50/50.

**5. Commit:** C3.

#### G2-116: Close's "complete drain" is never checked

**1. Verdict: accepted.**
- awaitRecordingStop (drain.go:91-107) returns nothing, and Drain returns nil either way.
- lifecycle_deadlines_internal_test.go:584-587 drops `cgUntil`'s result.

**2. Kind: K2a.**

**3. The change:**
- :579 becomes `id := ldStarted(t, ctx, r, app)`.
- :586 becomes `if !cgUntil(ctx, func() bool { _, s, _ := h.counts(); return s == 1 && ldRecorderIdle(self) }) { t.Error("the recorder never wrote the stop and parked: the ceiling") }`.
- After the Close check (:590-592), add:
  ```go
  	if rs := r.appList()[id].rec.stats(); rs.Failed {
  		t.Errorf("recording %+v; want it kept (the stop was written as the deadline passed)", rs)
  	}
  ```
- The doc's "the drain is complete" becomes "the drain is complete (the recording is kept, not Failed)".
- The frame table (`ldRecorderIdle`, `ldParked`) is untouched.

**4. The proof:**
- K2a entry: file `internal/runtime/drain.go`, find `"\tr.awaitRecordingStop(ctx, a, rep.recMarker)\n"`, replace `"\tr.awaitRecordingStop(ctx, a, rep.recMarker)\n\ta.rec.cut()\n"`. `cut` sets `failed` unconditionally (recorder.go:124-137).
- Test `TestCloseRemovesAnAppWhoseDeadlinePassedAsItsDrainEnded`: expect old green, new red. Plus a control entry: green, green.
- **Load:** `load.py -- -race -count=50 -run '^TestCloseRemovesAnAppWhoseDeadlinePassedAsItsDrainEnded$' ./internal/runtime`, 50/50.

**5. Commit:** C2.

#### G2-117: a 300 ms timer decides the branch

**1. Verdict: accepted.**
- recorder_drain_test.go:116 sets a 300 ms timeout.
- GitNexus/grep: `recordingWait` is read only at drain.go:93-95.

**2. Kind: K2c.**

**3. The change.** Replace :116 with:
```go
	dctx, cancel := context.WithCancel(ctx) // the drain's deadline is cancel, called by recordingWait after the barrier
	defer cancel()
	r.recordingWait = cancel
```
Then, after the Drain call, set `r.recordingWait = nil`. Drop the `"time"` import (:12): it is the file's only `time.` use.

**4. The proof:**
- **Slowdown plant:** a `--patch` on drain.go with two hunks: add `"time"` to the imports, and insert `time.Sleep(400 * time.Millisecond)` before `ch, err := r.queueDrain(ctx, a, app, was)`. The old ctx expires there, and enqueue checks `ctx.Err()` first, so the old test fails deterministically.
- Test `TestDrainMarksAStoppedRecordingIncompleteAtItsDeadline`: expect old red ("Drain: context deadline exceeded"), new green.
- **Load:** `load.py -- -race -count=50 -run '^TestDrainMarksAStoppedRecordingIncompleteAtItsDeadline$' ./internal/runtime`, 50/50.

**5. Commit:** C2.

#### G2-118: "stays live" is one non-blocking peek

**1. Verdict: accepted.** recorder_stop_internal_test.go:67-71 peeks once at `q.returned`. recQ.run loops after settle (recorder.go:168-171); `close(returned)` runs only on return.

**2. Kind: K2a.**

**3. The change:**
- Add `self := rsSelfID()` as the test's second statement. rdOverflowed activates on the test goroutine.
- After the Stats call, add:
  ```go
  	if !cgUntil(ctx, func() bool { return ldRecorderIdle(self) }) {
  		t.Error("the recorder goroutine is not parked in its select after the drain; want it live until its stop")
  	}
  ```
- The peek stays.

**4. The proof:**
- K2a entry: file `internal/runtime/recorder.go`.
  - find `"\t\t\tq.settle() // the output is final: nothing is put after a full queue; a drain waiting for it goes on\n"`
  - replace the same line followed by `"\t\t\t<-q.stop\n\t\t\tq.drainQueue()\n\t\t\tq.flush()\n\t\t\treturn\n"`. The goroutine leaves its select at the overflow.
- Test `TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles`: expect old green, new red. Plus a control entry: green, green.
  - Old is green because the peek passes, a drain after an overflow queues nothing to the recorder (drainStop returns `q.settled`, recorder.go:86), and the cleanup's Remove still stops it through `q.stop`.
  - New is red at `cgCeiling` (60 s), so mutate's timeout must exceed that.
- **Load:** `load.py -- -race -count=50 -run '^TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles$' ./internal/runtime`, 50/50.

**5. Commit:** C2.

#### G2-119: a known issue's wrong behaviour under a neutral name

**Verdict: accepted. Carried in full to S1.1c-8.**
- router_intake_internal_test.go:292-293's doc already says "Known issue S3.3-1-3 as it stands". The only fix left is the name.
- Proposed name: `TestRouterStillRecordsABatchEndQueuedBetweenStopAndItsDrainItem` ("still records" ← the record and Batches assertions in its body).
- Plan §5's rule (a known issue's wrong behaviour never becomes a contract) is met once the name says so.
- G10-21's choice (C) does not change this test.

#### G2-120: broadcast's third caller is not covered

**1. Verdict: accepted.** GitNexus `context broadcast` lists callers tickApp, drainApp and Sync.End (exact). The doc at :519-523 names only two.

**2. Kind: K2a.**

**3. The change:**
- The doc's "ingest's iTick (tickApp) and iDrain (drainApp)" becomes "ingest's iTick (tickApp) and iDrain (drainApp) here, and Sync.End in TestSyncEndBroadcastsAWalkWithNoAdmission".
- New test in router_intake_internal_test.go:
  ```go
  // Sync.End, replay's final walk, is broadcast's third caller: its one shard's message carries no admission either,
  // so the on-arrival rule's clock stays MinInt64.
  func TestSyncEndBroadcastsAWalkWithNoAdmission(t *testing.T) {
  	t.Parallel()
  	w := igBuild(t, 1, 1, nil)
  	s, err := NewSync(w.a.app, cgT0, false, func(event.Event) {})
  	if err != nil {
  		t.Fatal(err)
  	}
  	var adm []int
  	deliver := s.rt.deliver
  	s.rt.deliver = func(sid int, m message) { adm = append(adm, m.admStream); deliver(sid, m) }
  	if adv := s.End(); len(adv) == 0 {
  		t.Fatal("End advanced nothing: no walk to check")
  	}
  	if !slices.Equal(adm, []int{-1}) {
  		t.Errorf("End's messages' admStream %v; want [-1]", adm)
  	}
  	if clk := s.sh.rules[0][0].Clock(); clk != math.MinInt64 {
  		t.Errorf("the on-arrival rule's clock %d after End; want MinInt64", clk)
  	}
  }
  ```

**4. The proof:**
- K2a entry: file `internal/runtime/sync.go`.
  - find `"\t\ts.rt.broadcast(message{kind: mWalk, app: s.a.id, adv: adv, tables: s.a.ing.Snapshot(), final: true})"`
  - replace `"\t\ts.rt.deliver(0, message{kind: mWalk, app: s.a.id, adv: adv, tables: s.a.ing.Snapshot(), final: true})"`
  - This is a new test, so: `old_test` = TestBroadcastCarriesNoAdmission (green), `new_test` = TestSyncEndBroadcastsAWalkWithNoAdmission (red). Plus a control entry: green, green.
- No goroutines (Sync runs in place), so no load run.

**5. Commit:** C3.

#### G2-121: the proposed name claims a refusal that never happens

**Verdict: accepted. Carried in full to S1.1c-8.**
- The survey's correction holds: only iWrite is tested for a drained app (:181-184).
- Proposed name: `TestRouterRefusesAGoneAppsWriteOrDrainAndADrainedAppsWrite` ("a gone app's write or drain" ← the igGone loop's ErrRemoved; "a drained app's write" ← the ErrDrained check).
- The doc comment the finding asks for goes with the rename.

#### G2-122: a broken fixture

**1. Verdict: accepted.** s3_2b_2_boundary_test.go:28 has `log: &rtLog{}`, a nil `per` map. That is the only `&rtLog{}` in the package (grep).

**2. Kind: K2e** (broken fixture).

**3. The change:** `log: &rtLog{}` → `log: newRTLog()`.

**4. The proof (mechanical check):** `git grep -n -F '&rtLog{}' -- internal/runtime` gives 1 hit at the lane base (red) and 0 at the commit (green). Plus `load.py -- -race -count=50 -run '^TestSendRefusesWhatARecordingCannotCopy$' ./internal/runtime`, 50/50.

**5. Commit:** C3.

#### G2-123: two behaviours in one test

**Verdict: accepted. Carried in full to S1.1c-10** (a split, §11 Q4). Proposed: `TestCloseFlushesARecorderThatWasNeverDrained`.

#### G2-124: goroutine counts read from the whole process

**Verdict: accepted. Carried in full to S1.1c-10.** Moving three tests (breakit_test.go:167, sync_test.go:107, api_test.go:155) onto the `gr*` helpers changes how they measure. No plant shows an outcome difference today (§7.2's refactor rule).

#### G2-125: the NaN branch accepts any error

**1. Verdict: accepted** (keys_test.go:45-49).

**2. Kind: K2a.**

**3. The change:** at :46, `err == nil` becomes `err == nil || !strings.Contains(err.Error(), "NaN and infinities are never values")`. The message becomes `"Send with a NaN key: %v; want the non-finite refusal"`, err. This text survives G7a-5.

**4. The proof:**
- K2a entry: file `internal/runtime/boundary.go`, find `"\tcase v.Kind != at.Type:\n"`, replace `"\tcase v.Kind != at.Type || (v.Kind == event.TFloat && v.F != v.F):\n"` (NaN refused for the wrong reason).
- Test `TestKeyErrorsAndZeroKeys`: expect old green, new red. Plus a control entry: green, green.
- The load run is shared with G2-115.

**5. Commit:** C3.

#### G2-126: no cleanup on failure paths

**Verdict: accepted. Carried in full to S1.1c-10.** Making ldRuntime and rdOverflowed register cleanups touches about 20 callers, some of which already register `ldClose`. It changes no outcome on real code (§7.2).

#### G2-127: the no-sleep guard misses an aliased import

**1. Verdict: accepted.**
- nosleep_test.go:40 matches only an identifier named `time`.
- The doc (:3-4) floats unattached before the imports and omits AfterFunc, which the code bans (:22).

**2. Kind: K2a.**

**3. The change:**
- Move the doc onto `func TestNoSleepOrPollingInRuntime`, with "time.Sleep, time.After, time.Tick or time.AfterFunc".
- After parsing, collect the local names of import path `"time"`:
  ```go
  		names := map[string]bool{}
  		for _, im := range f.Imports {
  			if im.Path.Value != `"time"` {
  				continue
  			}
  			switch {
  			case im.Name == nil:
  				names["time"] = true
  			case im.Name.Name == ".":
  				t.Errorf("%s: a dot import of time hides its calls from this check", name)
  			case im.Name.Name != "_":
  				names[im.Name.Name] = true
  			}
  		}
  ```
- The condition at :40 becomes `names[x.Name]`.

**4. The proof:**
- A `--patch` plant on drain.go with two hunks: add `tm "time"` to the import block, and append `var _ = tm.Sleep` at the end of the file.
- Test `TestNoSleepOrPollingInRuntime`: expect old green, new red. Plus a control entry: green, green.
- **Coupling:** nosleep_test.go is a law path, so LAW Δ shows this edit (§6).

**5. Commit:** C3.

#### G2-128: any error passes a given-up Close

**1. Verdict: accepted.** recorder_stop_internal_test.go:43. `ldExpired()` is an already-cancelled ctx (lifecycle_deadlines_internal_test.go:100-104).

**2. Kind: K2a.**

**3. The change:** `if err := r.Close(ldExpired()); !errors.Is(err, context.Canceled) { t.Fatalf("Close with an expired ctx: %v; want context.Canceled", err) }`. Add the `context` and `errors` imports.

**4. The proof:**
- K2a entry: file `internal/runtime/shutdown.go`, find `"\treturn r.awaitGoroutines(ctx, first, failed)\n}"`, replace `"\tif err := r.awaitGoroutines(ctx, first, failed); err != nil {\n\t\treturn ErrClosed\n\t}\n\treturn nil\n}"`.
- Test `TestCloseStopsAnIdleRecorderItGaveUpOn`: expect old green, new red. Plus a control entry: green, green.
- **Load:** `load.py -- -race -count=50 -run '^TestCloseStopsAnIdleRecorderItGaveUpOn$' ./internal/runtime`, 50/50.

**5. Commit:** C2.

#### G2-129: a row that cannot fail

**1. Verdict: accepted.** Applying an admission of MinInt64 to a fresh rule leaves its clock at MinInt64 whether or not shard.go:184's guard is there.

**2. Kind: K2-del.**

**3. The change:** delete the row `"no value":  {0, math.MinInt64, false},` (router_intake_internal_test.go:392). `math` stays imported (it is used elsewhere in the file).

**4. The proof:** the covered set in C1 (see G2-113). The "no stream" row takes the same guard statement.

**5. Commit:** C1.

#### G2-130: the "tombstone" fixture is not a real tombstone

**Verdict: accepted. Carried in full to S1.1c-10.** The real tombstone (transitions.go completeRemove) only adds fields to the fixture's `ing = nil`. No production defect distinguishes the two, so this is a fidelity refactor (§7.2).

#### G2-131: `cgApp(t, …)` called on a spawned goroutine

**1. Verdict: accepted, with a third site.** cgApp calls `t.Fatal` (close_gate_test.go:54, :58). The sites:
- router_keeps_routing_internal_test.go:37 and lifecycle_deadlines_internal_test.go:674 (from the finding);
- breakit_s3_3_2_test.go:32, found during the re-check.

**2. Kind: K2e.**

**3. The change at each site:** build `app := cgApp(t, func(event.Event) error { return nil })` before the `go` statement and pass `app` to `r.Add`. cgApp itself is unchanged (R4). The frame tables are unaffected: they match production frames, not test lines.

**4. The proof (mechanical check):**
- A throwaway `go/ast` script in the lane's scratch, never committed, over the three files.
- Rule: inside every `*ast.GoStmt` (its call and, for a `FuncLit`, its whole body), no `*ast.CallExpr` has an argument that is the identifier `t`, or a local alias assigned from `t`.
- Result: 3 hits at the lane base (red), 0 at the commit (green). Any other hit the script reports is fixed the same way and named in the record.
- **Load:** `load.py -- -race -count=50 -run '^(TestRouterKeepsRouting.*|<the test at :674>)$'`, using the exact names of the two tests that contain the sites, 50/50.

**5. Commit:** C2.

#### G2-132: `Tick` with no ceiling

**1. Verdict: accepted** (s3_2b_2_fixround_test.go:53).

**2. Kind: K2d.**

**3. The change:** `r.Tick(context.Background(), rtAt(5))` → `r.Tick(ctx, rtAt(5))`.

**4. The proof:**
- K2d entry: file `internal/runtime/requests.go`, find `"\treturn r.enqueue(ctx, item{kind: iTick, now: now})\n"`, replace `"\t<-ctx.Done()\n\treturn ctx.Err()\n"`. The earlier out-of-range Ticks return before this line.
- Test `TestTickRefusesATimeARecordingCannotWrite`: expect old timeout, new red at rtCeiling (60 s, "Tick at an ordinary time: context deadline exceeded"). mutate's timeout must be over 60 s.
- **Load:** `load.py -- -race -count=50 -run '^TestTickRefusesATimeARecordingCannotWrite$' ./internal/runtime`, 50/50.

**5. Commit:** C3.

### 4. Commit plan

The K2 commits come first, so every K2 plant's production text is the same at the lane base and at its commit. The K1 commits follow.

| # | Kind (package `internal/runtime`) | Findings | Proofs |
|---|---|---|---|
| C1 | K2-del | G2-113, G2-129 | covered set, four profiles equal |
| C2 | K2 part 1 (lifecycle, recorder and close-gate files) | G2-112, G2-116, G2-117, G2-118, G2-128, G2-131 | `--expect` entries plus controls; the slowdown plant; the AST check; load runs |
| C3 | K2 part 2 (entry points, keys, routing, fixtures, guard) | G2-111, G2-114, G2-115, G2-120, G2-122, G2-125, G2-127, G2-132 | `--expect` entries plus controls; the coverage blocks; the grep check; the timeout→red; load runs |
| C4 | K1 | G7a-5 | revert plant red; T2 |
| C5 | K1c | G10-20 | `--race --repeat 50 --load` k ≥ 1; load 50/50 |
| C6 | K1h | G10-21 | revert plant red; compare.py exit 0 on mains power |

- **6 commits.** The plan budgets E1 at 4 lane commits (ceiling 6), plus K2-del from the shared pool. Here: 5 lane commits plus 1 K2-del.
- C2 and C3 together hold 14 findings, so the split by file is allowed (over 12).
- The plants go in `docs/sprints/s1-1c-7-plants/E1.json`.

### 5. Choices made

1. **G10-21** (§3): chosen (C). (A) is rejected on correctness. (B) is recommended as S3.3-1-3's fix and needs the owner, because it contradicts design s3-3-1 MUST 11 and T30 "(owner, design 6)". The derived-stream error stays ErrDrained, by the design's check order (:166). The two comments are rewritten. No other test pins the old comment text.
2. **G7a-5 text:** the 11 strings in §3. Every candidate keeps the rule and drops only the parenthetical. Dry run: no pinned parenthetical; the substring pins listed there stay green.
3. **G10-20:** fix the locking (K1c), not only the comment. The race is real.
4. **G2-113:** delete (covered set), not rewrite. A rewrite would keep a false name until S1.1c-8, and build_validate already pins the refusal and its text.
5. **G2-120:** add a test for the third caller, not just narrow the doc.

### 6. Couplings and risks

- **Per-event code (R5):**
  - G10-21 is K1h (BatchEnd).
  - G7a-5's edits are text only (named exemption).
  - The K2 plants touching per-event code (router_in.go:140, boundary.go, requests.go) are plants only.
  - `gate`, `ingest` and sync.go are not edited.
- **Frozen benchmarks (R4):** hotpath_bench_test.go is untouched. cgApp is untouched; G2-131 moves call sites only.
- **Frame tables and line-anchored mutants:** `ldParked` and `ldRecorderIdle` (lifecycle_deadlines_internal_test.go) and `rsSelectAt` (route_stop_internal_test.go) are untouched. No production function is renamed. `case mb <- head.msg:` is not edited.
- **Law path:** nosleep_test.go (G2-127) shows in LAW Δ. **The coordinator decides** whether it needs a coordinator commit or a proofs-row run; otherwise it stays in C3.
- **Other owners' files:** none are edited. Choice (B) for G10-21 would need root batch_test.go and helpers_test.go (E2) and gate/known-issues.json with docs/product/known-issues.md (coordinator). That is why it is forwarded, not built.
- **Ledger words:** G2-123, -124, -126 and -130 are "carried in full to S1.1c-10". The ledger's outcome list (plan §7.5) has no word for a whole finding carried to the later part. **The coordinator should add one.**
- **Pre-mortem:**
  - G10-20's k may be 0 (§3).
  - G2-118's and G2-132's new reds arrive only at the 60 s ceiling.
  - The covered-set proof assumes coverage is stable across runs. If it differs between two runs at the same revision, the lane reports it and falls back to plants where one exists.
- **Forwarded:** G2-115's possible second instance at breakit_test.go:388 (unchecked); G10-20's leftover logic window.

### 7. Carried items

**Renames → S1.1c-8:**
- G2-111's two test names and its file;
- G2-112's name and its file's `_internal` suffix;
- G2-119 (in full);
- G2-121 (in full, with its doc comment).

The proposed names and the assertion behind each verb are in §3.

**Later part (S1.1c-10):**
- G2-111's merge and deletion of the duplicate;
- G2-112's move into close_gate_internal_test.go;
- G2-123 (split);
- G2-124;
- G2-126;
- G2-130.


## Addendum: G10-1's runtime half

## E1 note addition: G10-1's runtime half (E2's option B)

Short answer: **option B works, but not exactly as specified.** Item 2 as written does not guarantee the activation step's apply cannot fail. Two conditions (A and B below) make it work. One of them is a one-function change in `internal/table`, which is CD's file, so you need to rule on it. Only reading and GitNexus were used here; no test, mutate, load or compare run was made, so every outcome below is a prediction.

### Is option B sound?

**What the code says:**
- Today `AppHandle.Start` (apphandle.go:43-52) calls Activate and then one `TableWrite` per table. If a write fails or ctx ends part-way, the app is active with some of its rows written. That is G10-1.
- `router.activate` (transitions.go:77-91) is called only from `ingestLifecycle` (GitNexus `context`, exact). `writeTable` (router_in.go:98-115) is called only from `ingest` (exact).
- `validateApp` is called by `Add` and `build`; `build` is reached from `New` and `NewSync`. On the playback path that covers 7 RunBlock processes (GitNexus `impact` upstream, exact, LOW).
- Rows travel inside `App`: `newAppState` stores `a.app = app` (app.go:60-69), so `item` does not change.
- Once Activate's item is queued, the router's answer is awaited whatever ctx does (`requestQueued`, lifecycle.go:65-76). So a Start either fails before the item is queued (nothing changed) or gets the router's final answer.
- `recQ.overflow` has room for one value (recorder.go:69). So a full queue during the activation step does not block the router, even though the recorder goroutine has not started yet.

**Condition A: Add's check must be the table's own check. Item 2 as specified falls short here.**
- `CheckRows` (boundary.go:235-255) checks only three things: floats are finite, values can be recorded, and the row fits one line.
- `table.Upsert` → `prepare` → `checkRow`/`checkValue` (internal/table/table.go:193-259) also refuses:
  - a wrong number of values;
  - a wrong kind;
  - a null in a key column;
  - a null in a column that is not optional;
  - an object held as anything but a string or `[]byte`.
- So with `CheckRows` alone, the router's apply can fail after earlier loads were already applied.
- The runtime cannot repeat the table's check itself. `Table` exposes no key columns (its exported methods are Name, Attrs, Snapshot, Upsert, Delete and Load). `markKey` (table.go:105-121) accepts an optional key column, which `checkValue` (:244) then refuses when it holds a null.
- **The fix needs a change outside E1's files.** `internal/table/table.go` gains one exported function, owned by CD:
  ```go
  // Check reports the first row Upsert or Load would refuse, worded as they word it, and changes nothing.
  func (t *Table) Check(rows [][]event.Value) error {
  	for i, r := range rows {
  		if err := t.checkRow(i, r); err != nil {
  			return err
  		}
  	}
  	return nil
  }
  ```
  - Its test: `TestCheckRefusesWhatUpsertRefusesAndChangesNothing`. Each row Upsert refuses gives Upsert's own error text, and `Snapshot()` returns the same `*Version` pointer before and after.
  - Its revert plant: `return nil` as the function's first statement; expected red.
  - CD works in W3, after E1. So this must land first, as a coordinator commit, or be handed to E1 for that one commit. **You need to rule on this.**
- **Without Condition A, option B does not hold**, and E2 falls back to option D.

**Condition B: a built app must refuse starting rows.**
- `build` (build.go:15-45) marks every app activated directly and never runs `router.activate`. So rows given to `runtime.Start` or `NewSync` would be ignored without a word.
- A replay must also never carry them: the recording already holds the table records, so they would be applied twice.
- `FromPlan` sets no starting rows, so playback is not affected.

**Corner pinned (E2's pre-mortem):** the rows' records are queued before the recorder goroutine starts.
- **The choice:** pin it with a test rather than exempt the starting rows.
- **The input:** a recording with RecordQueue n, where the header plus all the starting rows come to more than n records.
- **The candidates and why I rejected them:**
  - Exempting the rows would need either a router that blocks on its recorder (the router never waits on one) or an unbounded queue.
  - Refusing at Add would turn a recording-capacity limit into a deploy failure. That goes against owner D2 of S3.2b-2: a full queue ends the recording and detection goes on.
  - Starting the recorder before the router's decision would start goroutines for an app the router may still refuse.
- **What happens instead:** the recording ends with "queue full", the rows are applied in full, and detection goes on. The rule is documented on `Config.RecordQueue` and `App.Loads`.
- **How it differs from today:** today each `TableWrite` races a recorder that is already running. Now the header and all tables' starting rows share the queue together.

**Other results:**
- `ErrClosed` from `startAppGoroutines` after the router has activated is unchanged. Close is ending the runtime, so there is nothing to retry.
- Ticks skip an app that is not activated (router_in.go:89), and the rows are applied before `a.activated = true`. So the first tick and the first event see the rows.
- The recording order stays: header, then the table rows, then events. The bytes match what today's Start writes.

### Finding: G10-1 (runtime half)

**1. Verdict:** accepted, given Conditions A and B.

**2. Kind: K1h.**
- `router.activate` is reached from the router loop through `ingestLifecycle`.
- `writeTable` is reached from the facade's `Upsert` and `Delete` (R5 entry points) through `TableWrite`.

**3. The exact change.** All in E1's files; the steps are ordered.

1. **`internal/runtime/runtime.go`.** Add to `App`, after `OnRemoved`:
   ```go
   	// Loads are the app's starting rows, put table by table in load order by the router's activation step
   	// (Activate), before the app takes any input; Add checks each, so applying them cannot fail. The runtime keeps
   	// the rows: the caller must not change them after Add. With a Recorder, the header and every starting row's
   	// record queue before the recorder starts, so together they must fit Config.RecordQueue, or the recording ends at
   	// once (queue full) while the rows are still applied. A built app (Start, NewSync) takes none.
   	Loads []TableLoad
   ```
   The new type, after `App`:
   ```go
   // TableLoad is one table's starting rows: whole rows put into table Table of App.Tables at activation.
   type TableLoad struct {
   	Table int
   	Rows  [][]event.Value
   }
   ```
   `Config.RecordQueue`'s doc gains: "An app's header and starting rows are queued before its recorder starts, so they must fit it together."

2. **`internal/runtime/build.go`.**
   - `validateApp`'s last line `return checkSinks(i, a)` becomes:
     ```go
     	if err := checkSinks(i, a); err != nil {
     		return err
     	}
     	return checkLoads(i, a)
     ```
   - A new function after `checkSinks`:
     ```go
     // checkLoads checks each starting load against its table and a recording, so the activation step cannot fail.
     func checkLoads(i int, a *App) error {
     	for _, l := range a.Loads {
     		if a.Tables == nil || l.Table < 0 || l.Table >= a.Tables.Len() {
     			return fmt.Errorf("runtime: app %d: starting rows for table %d: the app has no such table", i, l.Table)
     		}
     		tb := a.Tables.Table(l.Table)
     		if err := tb.Check(l.Rows); err != nil {
     			return fmt.Errorf("runtime: app %d: starting rows for table %d: %w", i, l.Table, err)
     		}
     		if err := CheckRows(tb.Name(), tb.Attrs(), l.Rows); err != nil {
     			return fmt.Errorf("runtime: app %d: starting rows for table %d: %w", i, l.Table, err)
     		}
     	}
     	return nil
     }
     ```
   - In `build`'s first loop, after the `validateApp` call:
     ```go
     		if len(apps[i].Loads) > 0 { // applied by the activation step, which a built app never takes
     			return nil, fmt.Errorf("runtime: app %d: starting rows are applied at Activate, and a built app is never activated", i)
     		}
     ```
   - `Check` reads only `t.attrs`, `t.isKey` and `t.key`, which never change after `New`. So calling it on Add's caller goroutine is race-free.

3. **`internal/runtime/router_in.go`.** One helper shared by `writeTable` and the activation step:
   ```go
   // applyWrite applies one table write to a and queues one record per row written (Records counts lines: break-it
   // S1); a write that fails records nothing.
   func (a *appState) applyWrite(table int, op TableOp, rows [][]event.Value) error {
   	if err := a.write(table, op, rows); err != nil {
   		return err
   	}
   	for _, row := range copyRows(rows) {
   		a.rec.put(recItem{kind: rTable, table: table, op: op, rows: [][]event.Value{row}})
   	}
   	return nil
   }
   ```
   `writeTable`'s tail, from `err := a.write(…)` to the reply, becomes `it.reply <- reply{err: a.applyWrite(it.table, it.op, it.rows)}`.

4. **`internal/runtime/transitions.go`.**
   - `activate`'s default branch:
     ```go
     	default:
     		if err := rt.applyLoads(a); err != nil { // checked at Add: only a broken invariant reaches here
     			it.reply <- reply{err: err}
     			return
     		}
     		a.activated = true
     		a.started.Store(true)
     		it.reply <- reply{}
     ```
   - The new method:
     ```go
     // applyLoads puts the app's starting rows (App.Loads) table by table in load order and records them as table
     // writes; Add checked them, so it returns nil unless that check and the table disagree.
     func (rt *router) applyLoads(a *appState) error {
     	for _, l := range a.app.Loads {
     		if err := a.applyWrite(l.Table, TablePut, l.Rows); err != nil {
     			return err
     		}
     	}
     	return nil
     }
     ```
   - `a.app.Loads` is never cleared. Callers read `a.app` concurrently, and the facade keeps the same rows today anyway.

5. **`internal/runtime/lifecycle.go`.** Add to Activate's doc (:77-83), after "accept the app from here,": "The router puts the app's starting rows (App.Loads) in the same step, before it answers, so the app's first tick and first event read them and their table records follow the header."

6. **New file `internal/runtime/starting_rows_internal_test.go`** (package `runtime`). Imports: `fmt`, `strings`, `testing`, `unicode/utf8`, `internal/event`, `internal/table`. No new file-topic prefix.
   ```go
   // startingRowsTable is a table T (k string, the key; v int) for the starting-rows tests.
   func startingRowsTable(t *testing.T) *table.Table {
   	t.Helper()
   	tb, err := table.New("T", []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}}, []string{"k"})
   	if err != nil {
   		t.Fatal(err)
   	}
   	return tb
   }

   // startingRows is n rows of T: k1 → 1, k2 → 2, ….
   func startingRows(n int) [][]event.Value {
   	rows := make([][]event.Value, n)
   	for i := range rows {
   		rows[i] = []event.Value{{Kind: event.TString, Str: fmt.Sprintf("k%d", i+1)}, {Kind: event.TInt, I: int64(i + 1)}}
   	}
   	return rows
   }

   // The starting rows an App carries are put in the router's activation step: none after Add, all of them when the
   // activation is answered (the activateHold seam, before the flushers and the recorder start).
   func TestActivateAppliesTheStartingRowsInItsStep(t *testing.T) {
   	t.Parallel()
   	ctx := cgCtx(t)
   	r := ldRuntime(t)
   	t.Cleanup(func() { ldClose(t, r) })
   	tb := startingRowsTable(t)
   	app := cgApp(t, func(event.Event) error { return nil })
   	app.Tables = table.NewSet([]*table.Table{tb})
   	app.Loads = []TableLoad{{Table: 0, Rows: startingRows(2)}}
   	id, err := r.Add(ctx, app)
   	if err != nil {
   		t.Fatalf("Add: %v", err)
   	}
   	if n := tb.Snapshot().Len(); n != 0 {
   		t.Fatalf("rows in T after Add: %d; want 0 (nothing is put before the activation step)", n)
   	}
   	answered := -1
   	r.activateHold = func() { answered = tb.Snapshot().Len() }
   	if err := r.Activate(ctx, id); err != nil {
   		t.Fatalf("Activate: %v", err)
   	}
   	if answered != 2 {
   		t.Errorf("rows in T when the activation was answered: %d; want 2 (put in the activation step)", answered)
   	}
   	if row, ok := tb.Snapshot().Get([]event.Value{{Kind: event.TString, Str: "k1"}}); !ok || row[1].I != 1 {
   		t.Errorf("T's row k1 after Activate: %v, %v; want [k1 1]", row, ok)
   	}
   }

   // Add refuses starting rows the activation step could not put, before anything is installed: a table the app does
   // not have, a row the table refuses, a row a recording cannot copy.
   func TestAddRefusesStartingRowsItCouldNotApply(t *testing.T) {
   	t.Parallel()
   	ctx := cgCtx(t)
   	r := ldRuntime(t)
   	t.Cleanup(func() { ldClose(t, r) })
   	bad := "a\xffb"
   	if utf8.ValidString(bad) {
   		t.Fatal("the malformed input is valid UTF-8")
   	}
   	for name, c := range map[string]struct {
   		load TableLoad
   		want string
   	}{
   		"no table":     {TableLoad{Table: 1, Rows: startingRows(1)}, "starting rows for table 1: the app has no such table"},
   		"negative":     {TableLoad{Table: -1, Rows: startingRows(1)}, "starting rows for table -1: the app has no such table"},
   		"short row":    {TableLoad{Rows: [][]event.Value{{{Kind: event.TString, Str: "k"}}}}, "table T: row 0: 1 values for 2 attributes"},
   		"null key":     {TableLoad{Rows: [][]event.Value{{{Kind: event.TString, Null: true}, {Kind: event.TInt, I: 1}}}}, `table T: row 0: attribute 0 "k": a key value cannot be null`},
   		"unrecordable": {TableLoad{Rows: [][]event.Value{{{Kind: event.TString, Str: bad}, {Kind: event.TInt, I: 1}}}}, "holds malformed UTF-8"},
   	} {
   		app := cgApp(t, func(event.Event) error { return nil })
   		app.Tables = table.NewSet([]*table.Table{startingRowsTable(t)})
   		app.Loads = []TableLoad{c.load}
   		if _, err := r.Add(ctx, app); err == nil || !strings.Contains(err.Error(), c.want) {
   			t.Errorf("%s: Add: %v; want an error holding %q", name, err, c.want)
   		}
   	}
   	if n := len(r.appList()); n != 0 {
   		t.Errorf("%d app slots after the refused Adds; want 0 (refused before the router)", n)
   	}
   }

   // A built app is never activated, so it takes no starting rows (a replay's come from its recording).
   func TestABuiltAppTakesNoStartingRows(t *testing.T) {
   	t.Parallel()
   	app := cgApp(t, func(event.Event) error { return nil })
   	app.Tables = table.NewSet([]*table.Table{startingRowsTable(t)})
   	app.Loads = []TableLoad{{Table: 0, Rows: startingRows(1)}}
   	if _, err := build(Config{Shards: 1}, []App{app}); err == nil || !strings.Contains(err.Error(), "a built app is never activated") {
   		t.Fatalf("build with starting rows: %v; want the refusal", err)
   	}
   }

   // The header and every starting row queue before the recorder starts: at RecordQueue 4 the header and 3 rows fit,
   // and a 4th row ends the recording (queue full) while all 4 rows are still put.
   func TestTheHeaderAndTheStartingRowsShareTheRecordQueue(t *testing.T) {
   	t.Parallel()
   	for name, c := range map[string]struct {
   		rows   int
   		failed bool
   	}{
   		"fit":      {3, false},
   		"one over": {4, true},
   	} {
   		t.Run(name, func(t *testing.T) {
   			t.Parallel()
   			ctx := cgCtx(t)
   			r, err := New(Config{Shards: 1, SinkQueue: 4, RecordQueue: 4, Now: func() int64 { return cgT0 }})
   			if err != nil {
   				t.Fatal(err)
   			}
   			t.Cleanup(func() { ldClose(t, r) })
   			h := newHeldFlushRecorder(nil)
   			h.let()
   			tb := startingRowsTable(t)
   			app := cgApp(t, func(event.Event) error { return nil })
   			app.Sinks, app.Recorder = nil, h
   			app.Tables = table.NewSet([]*table.Table{tb})
   			app.Loads = []TableLoad{{Table: 0, Rows: startingRows(c.rows)}}
   			id := ldStarted(t, ctx, r, app)
   			st, err := r.Stats(ctx)
   			if err != nil {
   				t.Fatal(err)
   			}
   			if rs := st.Recording[id]; rs.Failed != c.failed || (c.failed && !strings.Contains(rs.Err, "queue full after record 4")) {
   				t.Errorf("recording %+v with %d starting rows at RecordQueue 4; want Failed %v", rs, c.rows, c.failed)
   			}
   			if n := tb.Snapshot().Len(); n != c.rows {
   				t.Errorf("rows in T: %d; want %d (put whatever the recording does)", n, c.rows)
   			}
   		})
   	}
   }
   ```

**4. The proof's material.** Plants are named `g10-1-*` in `docs/sprints/s1-1c-7-plants/E1.json`.
- **Revert plant `g10-1-apply-revert`.** It keeps `App.Loads`, `TableLoad`, `applyLoads`, `applyWrite` and `checkLoads`, and removes only the apply.
  - File `internal/runtime/transitions.go`.
  - find `"\t\tif err := rt.applyLoads(a); err != nil { // checked at Add: only a broken invariant reaches here\n\t\t\tit.reply <- reply{err: err}\n\t\t\treturn\n\t\t}\n"`, replace `""`.
  - Expected **red** on `TestActivateAppliesTheStartingRowsInItsStep`, at "rows in T when the activation was answered: 0; want 2".
  - Expected **red** on `TestTheHeaderAndTheStartingRowsShareTheRecordQueue/one_over`, at Failed false and 0 rows.
- **Revert plant `g10-1-check-revert`.**
  - File `internal/runtime/build.go`, find `"\treturn checkLoads(i, a)\n"`, replace `"\treturn nil\n"`.
  - Expected **red** on `TestAddRefusesStartingRowsItCouldNotApply`, at the "no table: Add: <nil>" line.
- **Revert plant `g10-1-build-revert`.**
  - File `internal/runtime/build.go`.
  - find `"\t\tif len(apps[i].Loads) > 0 { // applied by the activation step, which a built app never takes\n"`, replace `"\t\tif false {\n"`.
  - Expected **red** on `TestABuiltAppTakesNoStartingRows`.
- **Load runs, 50/50 each:** `load.py -- -race -count=50 -run '^TestActivateAppliesTheStartingRowsInItsStep$' ./internal/runtime`. The same for `TestAddRefusesStartingRowsItCouldNotApply` and `TestTheHeaderAndTheStartingRowsShareTheRecordQueue`.
- **Timing on mains power:**
  1. `pmset -g batt` reads "AC Power".
  2. Build the parent's test binary in a detached worktree (`go test -c -o <tmp>/before.test ./internal/runtime`) and the commit's (`<tmp>/after.test`).
  3. Run `python3.15 -I -B gate/bench/compare.py --before <tmp>/before.test --after <tmp>/after.test --bench '^(BenchmarkSendPath|BenchmarkSendPayloadPath)$' --dir internal/runtime --pairs 10`.
  4. `pmset -g batt` again. A session that ran on battery is discarded.
  - Expected: exit 0, "no cost". Send's path is not edited, and `writeTable`'s body moves into `applyWrite` with the same calls.
  - Exit 1: ground rule 13a applies and the lane stops.
- **T2 green.** `TestValidateAppNamesTheFirstFault` and `TestValidateAppRefusesEachMalformedRule` keep their order, because `checkLoads` runs last and their apps carry no starting rows.

**5. Commit:** C7, K1h. It lands after the `table.Check` commit (Condition A).

**User text chosen** (new runtime errors; no existing test pins them):
- `runtime: app %d: starting rows for table %d: the app has no such table`
- `runtime: app %d: starting rows for table %d: %w`, wrapping the table's or CheckRows' text
- `runtime: app %d: starting rows are applied at Activate, and a built app is never activated`

**For E2's note (not E1's files):**
- The facade sets `app.Loads` from `h.tableRows` before `Add` (deploy.go:109) and deletes the `TableWrite` loop at apphandle.go:47-51.
- A bad row now fails at Deploy (Add) rather than at Start. That matches the design's §3.4 intent ("a row TableWrite would refuse fails the deploy, not the start").
- The facade's `CheckRows` at deploy.go:221 becomes redundant. It can stay as an earlier refusal with the facade's own wording.
- E2's commit must come after E1's C7 (W3 runs after W2).

### Updated commit plan (`internal/runtime`)

| # | Kind | Findings | Proofs |
|---|---|---|---|
| C1 | K2-del | G2-113, G2-129 | covered set, four profiles equal |
| C2 | K2 part 1 | G2-112, G2-116, G2-117, G2-118, G2-128, G2-131 | `--expect` entries with controls; the slowdown plant; the AST check; load runs |
| C3 | K2 part 2 | G2-111, G2-114, G2-115, G2-120, G2-122, G2-125, G2-127, G2-132 | `--expect` entries with controls; coverage blocks; the grep check; timeout → red; load runs |
| C4 | K1 | G7a-5 | revert plant red; T2 |
| C5 | K1c | G10-20 | `--race --repeat 50 --load` k ≥ 1; load 50/50 |
| C6 | K1h | G10-21 | revert plant red; compare.py exit 0 on mains power |
| C7 | K1h | G10-1 (runtime half) | three revert plants red; load 50/50; compare.py exit 0 on mains power |

- **7 commits:** 6 lane commits (C2 to C7) and 1 K2-del (C1) from the shared pool.
- The lane is exactly at its ceiling of 6 (its line of 4 plus 2). Nothing is spare for a fix round, so any further E1 work needs the coordinator's re-projection.
- **The `table.Check` function is not an E1 commit.** It is a coordinator or CD commit in `internal/table`, and it must land before C7. Without it, option B does not hold and E2 falls back to option D.


## Revision 1, after the blind reviews

## E1 note: revision after the blind reviews

All ten review items are handled; nothing was rejected. Two changes go beyond what was asked:
- **G10-21's test now catches the defect that breaks a recording**, not just the counter (DQ-L2). The playback reader cannot be imported from package `runtime`, so the derived stream's `Batches` counter is the proof instead.
- **C7's new table-write benchmark lands inside C7**, with its own test file. The parent's test binary gets the same file through `-overlay`, so no extra commit is needed.

Nothing was run: every expected outcome below is read from the code.

### Change list

| Review item | Section replaced | Change |
|---|---|---|
| DA-S1 | G10-1 runtime half (C7) | The app's recording queue is sized at Add to `max(cfg, 1 + Σrows + (cfg − 1))`, which equals cfg + Σrows. The "one over" test row is replaced by "two tables beyond RecordQueue record in full". New plant `g10-1-size-revert`. The App.Loads and RecordQueue docs are reworded. |
| DA-S7 | G10-1 (C7), G10-21 (C6) | New benchmark `BenchmarkTableWritePath` (not frozen), timed parent against commit through `-overlay`. C6's Send/SendPayload timing is stated as a collateral-cost check. |
| DA-L2 | G10-21 | The source-stream half of the test is dropped. The Stats reply on the single intake is now the witness. |
| DQ-L2 | G10-21 | The test now covers the window between Stop and the drain item: `Batches == 0` on the derived stream means no `rBatchEnd` record was queued. |
| DA-L8 | G10-20 | The bounded wait on `runMu` is stated. |
| DQ-L1 | G10-20 | The mutate command uses `--test` with a bare test name. |
| DQ-S3 | G2-113, G2-129 | Each deletion gets a deterministic subset proof. The "no stream" row is shown to cover the block at shard.go:182-184 on its own. |
| DQ-L3 | G2-131 | The load run names all three tests and `./internal/runtime`. |
| DQ-L5 | G2-118, G2-132 | Each entry gets `"timeout": 180`. |
| item 10 | commit plan | `table.Check` is its own commit before W2, outside E1. |

### Replacement sections

#### G10-20 (replaces item 4's command; adds the bounded wait)

*(annotated 2026-11-22: PR 2's fix round replaced `markDraining`'s snapshot and `revertUnlessQuieted` by `holdDraining` and `releaseDraining`, which still read `waited` under `runMu`: [the Drain design note](s1-1c-7-design-drain.md). G10-20's plant is re-cut as `E1-G10-20-revert-2.diff`.)*

**4. The proof:**
- **Red** (the revert plant `g10-20-revert` is unchanged):
  `python3.15 -I -B gate/bench/mutate.py --race --repeat 50 --load --patch <g10-20-revert> --test TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained --pkg ./internal/runtime`
  Expect k ≥ 1 red out of 50, from a data-race report between drain.go:119 (the write) and the restored unlocked read. Record k.
- **Green:** `load.py -- -race -count=50 -run '^TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained$' ./internal/runtime`, 50/50.

**The wait on `runMu` is bounded.**
- `revertUnlessQuieted` takes `runMu`. So a Drain whose ctx has already ended can wait behind another Drain's `awaitRuns`.
- `awaitRuns` holds `runMu` while it waits for the runs (drain.go:112-139). That wait ends when every run returns, or at that other Drain's own deadline (`case <-ctx.Done()`, :136).
- So the expired Drain waits at most until that other Drain's deadline.
- At :28 this wait is not new. The expired Drain already took `runMu` in its own `awaitRuns` (:112), so the only addition is re-taking it after the unlock.
- At :80 it is new, for a Drain that skipped `awaitRuns` because `started` was false.
- Neither caller holds a lock while it waits, so there is no deadlock. The lock order is unchanged: `runMu` is taken before `inflight.mu` (Run).

#### G10-21 (replaces item 3's test, item 4, and the timing statement)

**3. The red test**, in internal/runtime/lifecycle_test.go after TestRecorderRecordsBatchEnd. It replaces the earlier version, and the source-stream half is gone. Add `context` to the imports if it is missing.

```go
// A derived stream takes no batch end. One sent once Stop has begun is refused with ErrDrained and never reaches the
// router: not taken while the drain item is still to come (no Batches count, so no batch_end record, which a
// recording's reader refuses on a derived stream), and not counted in AfterDrain after the drain. The Stats reply,
// queued behind each BatchEnd on the one intake, is the witness that the router has taken everything before it.
func TestABatchEndOnADerivedStreamOnceStopBeganIsNeitherTakenNorCounted(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 1})
	id := lcStarted(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
	cancelled, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(sync.OnceFunc(func() { close(release) }))
	if err := r.Run(ctx, id, func(rctx context.Context) error { <-rctx.Done(); close(cancelled); <-release; return nil }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	drained := make(chan error, 1)
	go func() { _, err := r.Drain(ctx, id); drained <- err }()
	select {
	case <-cancelled: // Drain has moved the phase and waits for the run: its drain item is still to come
	case <-ctx.Done():
		t.Fatal("Drain never cancelled the run: the ceiling")
	}
	if err := r.BatchEnd(ctx, id, 1, "mid"); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("BatchEnd on the derived stream once Stop began: %v; want ErrDrained", err)
	}
	if ss := rtStats(t, ctx, r).Streams[id][1]; ss.Batches != 0 || ss.AfterDrain != 0 {
		t.Errorf("derived stream %+v before the drain item; want Batches 0 (no batch_end record) and AfterDrain 0", ss)
	}
	close(release)
	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("Drain: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("Drain never returned: the ceiling")
	}
	if err := r.BatchEnd(ctx, id, 1, "after"); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("BatchEnd on the derived stream after the drain: %v; want ErrDrained", err)
	}
	if ss := rtStats(t, ctx, r).Streams[id][1]; ss.AfterDrain != 0 {
		t.Errorf("derived stream's AfterDrain after the drain: %d; want 0", ss.AfterDrain)
	}
}
```

Notes for the builder:
- The cleanup and the explicit release use one `OnceFunc`. Store it in a variable and call that instead of `close(release)`: `rel := sync.OnceFunc(func() { close(release) }); t.Cleanup(rel)`, then `rel()` at the release point.
- **Why `cancelled` is a sound witness:** Drain moves the phase (`markDraining`) before `awaitRuns` cancels the source context, and the context cancel orders the two. Drain then waits for the run inside `awaitRuns`, so its drain item has not been queued yet.
- **What the old code does here:** it queues `iBatchEnd` on stream 1. The router has not drained, so `ingestBatchEnd` counts `Batches[1]` and queues `rBatchEnd` in the same two lines (router_in.go:49-50).
- **DQ-L2:** `Batches == 0` therefore means no `rBatchEnd` was queued. The playback reader (records.go:277) cannot be imported from package `runtime`, because playback imports runtime. So this counter is the proof of the recording side.
- **DA-L2:** the source-stream checks stay where they already are, in lifecycle_test.go:724 and :759.

**4. The proof:**
- **Revert plant `g10-21-revert`**, unchanged:
  - file `internal/runtime/lifecycle.go`;
  - find `"stream < len(a.app.Streams) &&\n\t\t\t!a.app.Streams[stream].Derived {"`;
  - replace `"stream < len(a.app.Streams) {"`.
- **Expected red** at the first Stats check: "derived stream {… Batches:1} before the drain item…".
- **Load:** `load.py -- -race -count=50 -run '^TestABatchEndOnADerivedStreamOnceStopBeganIsNeitherTakenNorCounted$' ./internal/runtime`, 50/50.

**Timing (DA-S7):**
- C6 edits only BatchEnd's refused branch: lifecycle.go:178-183, which runs only once Stop has begun.
- BatchEnd's accepted path is byte-unchanged. No benchmark exercises the refused branch, and neither Send nor SendPayload calls BatchEnd.
- So the compare.py run on `^(BenchmarkSendPath|BenchmarkSendPayloadPath)$` is a **collateral-cost check** on the same package binary. Its command and the mains-power rule are as before. Expect exit 0.

#### G2-113 and G2-129: the K2-del proof (replaces item 4 of both; C1)

The whole-package covered-set check stays. Each deletion now also gets a deterministic subset check.

1. **The deleted test's own blocks**, at the lane base:
   - G2-113: `go test -count=1 -run '^TestBkDuplicateInputDoublesEvents$' -coverpkg=./... -coverprofile=<d113> ./internal/runtime`.
   - G2-129: `go test -count=1 -run '^TestShardAppliesAnAdmissionOnlyWithAStreamAndAValue$/^no_value$' -coverpkg=./... -coverprofile=<d129> ./internal/runtime`.
2. **The remaining suite**, at C1, run twice: `go test -count=1 -coverpkg=./... -coverprofile=<rN> ./internal/runtime` for r1 and r2.
3. **The check**, with a stdlib script kept in the lane's scratch space and not committed:
   - Parse each profile line `file:a.b,c.d stmts count` into a block key `file:a.b,c.d`.
   - A block is covered when any of its lines has count > 0.
   - The check passes when `covered(d113) ∪ covered(d129) ⊆ covered(r1) ∩ covered(r2)`.
   - This only checks containment, so it cannot be upset by blocks that differ between runs of concurrent code.
4. **G2-129's guard:** at C1, `go test -count=1 -run '^TestShardAppliesAnAdmissionOnlyWithAStreamAndAValue$/^no_stream$' -coverprofile=<ns> ./internal/runtime`.
   - Expected: the block that runs from `applyAdmission`'s opening brace through the guard `if stream >= 0 && adm != math.MinInt64 {` (shard.go:182-184) has count > 0.
   - So the "no stream" row alone covers the statement the deleted row covered.
   - The if-body (shard.go:185-190) is covered by the "both" row.
   - Quote the block's line from the profile.

#### G2-118 and G2-132: per-entry timeouts (adds to item 4)

| Entry | Timeout | Why it needs more than 60 s |
|---|---|---|
| g2-118 | `"timeout": 180` | the new test goes red only when `cgUntil` reaches the 60 s `cgCeiling` |
| g2-132 | `"timeout": 180` | the new test goes red at the 60 s `rtCeilingDur`; the old test hangs, so its outcome must read `timeout` at 180 s |

180 s is the 60 s ceiling plus build time, plus margin for a loaded machine. These are W0's new per-entry `timeout` batch key.

#### G2-131: the load run (replaces item 4's load line)

`load.py -- -race -count=50 -run '^(TestRouterKeepsRoutingAfterAHeadSend|TestActivateIsErrClosedOnceCloseHasBegun|TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled)$' ./internal/runtime`, 50/50.

The three sites are router_keeps_routing_internal_test.go:37, lifecycle_deadlines_internal_test.go:674 and breakit_s3_3_2_test.go:32.

#### G10-1 runtime half (replaces the addendum's Condition B corner, step 1's docs, step 6's last test, and item 4; C7)

**The recording queue is sized at Add (DA-S1).** This is a choice the design note makes; the owner is informed at G1.

- **The input:** an app with a Recorder and starting rows. The header plus all the rows come to more than `Config.RecordQueue` records, and all of them are queued before the recorder starts.
- **The formula:** `newRecQ(app.Recorder, max(cfg, 1 + Σrows + headroom))`.
- **The headroom chosen:** `cfg − 1`, so the formula equals **cfg + Σrows**.
- **Why this headroom:**
  - With no starting rows the queue is exactly today's size. Every queue-full test still pins the same bound. Examples: rdOverflowed's "header + 64" in recorder_drain_test.go, and TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles' 65 lines.
  - With starting rows, once the header and rows are queued, the app has the same free room for later records (cfg − 1) as an app without rows has after its header.
  - A smaller headroom would leave a loaded app less room than a loadless one. A larger one would change the configured bound.
- **A full queue still ends the recording** for later records (owner D2 of S3.2b-2).
- **The cost:** a buffered channel allocates all its slots up front, so the queue grows by Σrows slots of `recItem`. That is proportional to the rows the app already holds in its tables.

**The code** (internal/runtime/app.go:74-75):
```go
	if app.Recorder != nil {
		a.rec = newRecQ(app.Recorder, recordQueueFor(r.cfg.RecordQueue, app.Loads))
```
And after `newAppState`:
```go
// recordQueueFor is an app's recording queue: the configured size plus one slot per starting row, which is
// max(configured, 1 + rows + (configured - 1)). The header and every starting row are queued before the recorder
// starts, so they always fit and leave the room an app without starting rows has after its header; a full queue
// still ends the recording (owner D2 of S3.2b-2's design).
func recordQueueFor(configured int, loads []TableLoad) int {
	n := configured
	for _, l := range loads {
		n += len(l.Rows)
	}
	return n
}
```
- `r.cfg.RecordQueue` has its default applied by then (build.go:72-73).
- `newAppState` runs in the router's install step, and in `build`, where loads are refused. It does not run per event.

**Docs** (they replace the addendum's step-1 wording).
- `App.Loads`:
  ```go
  	// Loads are the app's starting rows, put table by table in load order by the router's activation step
  	// (Activate), before the app takes any input; Add checks each, so applying them cannot fail. The runtime keeps
  	// the rows: the caller must not change them after Add. With a Recorder, the app's recording queue is
  	// Config.RecordQueue plus one slot per starting row, so the header and the starting rows, queued before the
  	// recorder starts, always fit and leave the configured room for later records. A built app (Start, NewSync)
  	// takes none.
  	Loads []TableLoad
  ```
  *(annotated 2026-11-22: the code reads "A built app (NewSync) takes none." `runtime.Start` is New, then Add and Activate per app (runtime.go:186-205), so it applies the rows; only `NewSync` goes through `build`. The same holds for line 870's "`runtime.Start` or `NewSync`". C7, e0f0e13.)*
- `Config.RecordQueue`:
  ```go
  	// RecordQueue is each recording's queue, in records, plus one slot per starting row of the app (App.Loads); 0:
  	// 65,536. A full queue ends the recording (owner D2 of S3.2b-2's design).
  ```

**Test changes in starting_rows_internal_test.go:**
- `startingRowsTable` takes `(t testing.TB, name string)`. The earlier tests pass `"T"`.
- `TestTheHeaderAndTheStartingRowsShareTheRecordQueue` is replaced by:
```go
// With starting rows, an app's recording queue is RecordQueue plus one slot per row: at RecordQueue 4, two tables of
// 3 rows each (the header and 6 rows, 7 records queued before the recorder starts) record in full, and every line is
// counted (the header, the 6 rows, the stop).
func TestStartingRowsBeyondRecordQueueRecordInFull(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r, err := New(Config{Shards: 1, SinkQueue: 4, RecordQueue: 4, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ldClose(t, r) })
	h := newHeldFlushRecorder(nil)
	h.let()
	ta, tb := startingRowsTable(t, "A"), startingRowsTable(t, "B")
	app := cgApp(t, func(event.Event) error { return nil })
	app.Sinks, app.Recorder = nil, h
	app.Tables = table.NewSet([]*table.Table{ta, tb})
	app.Loads = []TableLoad{{Table: 0, Rows: startingRows(3)}, {Table: 1, Rows: startingRows(3)}}
	id := ldStarted(t, ctx, r, app)
	if _, err := r.Drain(ctx, id); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rs := st.Recording[id]; rs.Failed || rs.Records != 8 {
		t.Errorf("recording %+v; want not Failed and 8 records (the header, 6 starting rows, the stop)", rs)
	}
	if na, nb := ta.Snapshot().Len(), tb.Snapshot().Len(); na != 3 || nb != 3 {
		t.Errorf("rows in A %d, B %d; want 3 and 3", na, nb)
	}
}
```
- `Records` counts every line written (s3_2b_2_recorder_test.go:70's doc; `handle` increments it for each successful call, recorder.go:192-215).
- Drain waits for the stop's marker, so the stop is written before Stats.

**The new benchmark (DA-S7).** File `internal/runtime/table_write_bench_test.go`. It is self-contained, so it also compiles in C7's parent:
```go
package runtime

// The table-write path's measure: BenchmarkTableWritePath keeps its name and world, so a later change to the write
// path is measured against the same thing.

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// BenchmarkTableWritePath times one TableWrite of one row (the same key each time, so the table keeps one row) on a
// live app with a recording: the caller's checks, the intake, the router's write step, the record per row and the
// reply. The recorder discards (igRec); a recording that fell behind and failed would measure another path, so the
// run fails then.
func BenchmarkTableWritePath(b *testing.B) {
	ctx := cgCtx(b)
	r, err := New(Config{Shards: 1, Now: func() int64 { return cgT0 }})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = r.Close(ctx) })
	tb, err := table.New("T", []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}}, []string{"k"})
	if err != nil {
		b.Fatal(err)
	}
	app := cgApp(b, func(event.Event) error { return nil })
	app.Sinks, app.Recorder = nil, igRec{}
	app.Tables = table.NewSet([]*table.Table{tb})
	id, err := r.Add(ctx, app)
	if err != nil {
		b.Fatal(err)
	}
	if err := r.Activate(ctx, id); err != nil {
		b.Fatal(err)
	}
	rows := [][]event.Value{{{Kind: event.TString, Str: "k"}, {Kind: event.TInt, I: 1}}}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := r.TableWrite(ctx, id, 0, TablePut, rows); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	st, err := r.Stats(ctx)
	if err != nil {
		b.Fatal(err)
	}
	if rs := st.Recording[id]; rs.Failed {
		b.Fatalf("recording %+v: it fell behind, so the run measured another path", rs)
	}
}
```
It is not frozen and is not in hotpath_bench_test.go. `cgCtx` and `cgApp` take `testing.TB` (close_gate_test.go:23, :49). `igRec` is the package's no-op Recorder (router_intake_internal_test.go:268).

**4. The proof (replaces the addendum's item 4):**
- **`g10-1-apply-revert`**, unchanged. Expected red on:
  - TestActivateAppliesTheStartingRowsInItsStep ("…answered: 0; want 2");
  - TestStartingRowsBeyondRecordQueueRecordInFull (Records 2, tables 0).
- **`g10-1-size-revert`** (new):
  - file `internal/runtime/app.go`;
  - find `"newRecQ(app.Recorder, recordQueueFor(r.cfg.RecordQueue, app.Loads))"`;
  - replace `"newRecQ(app.Recorder, r.cfg.RecordQueue)"`.
  - Expected red on TestStartingRowsBeyondRecordQueueRecordInFull: Failed true, "queue full after record 4".
- **`g10-1-check-revert` and `g10-1-build-revert`**, unchanged. Expected red on TestAddRefusesStartingRowsItCouldNotApply and TestABuiltAppTakesNoStartingRows.
- **Load runs**, 50/50 each, via `load.py -- -race -count=50 -run '^<Test>$' ./internal/runtime` for:
  - TestActivateAppliesTheStartingRowsInItsStep;
  - TestAddRefusesStartingRowsItCouldNotApply;
  - TestStartingRowsBeyondRecordQueueRecordInFull.
- **Timing on mains power** (DA-S7). The parent's binary gets the benchmark through an overlay:
  1. `git worktree add --detach <tmp>/before <C7^>`
  2. `git show <C7>:internal/runtime/table_write_bench_test.go > <tmp>/bench_test.go`
  3. Write `<tmp>/overlay.json` = `{"Replace":{"<tmp>/before/internal/runtime/table_write_bench_test.go":"<tmp>/bench_test.go"}}`, with absolute paths.
  4. In `<tmp>/before`: `go test -c -overlay <tmp>/overlay.json -o <tmp>/before.test ./internal/runtime`.
  5. At C7: `go test -c -o <tmp>/after.test ./internal/runtime`.
  6. `pmset -g batt` must read "AC Power".
  7. `python3.15 -I -B gate/bench/compare.py --before <tmp>/before.test --after <tmp>/after.test --bench '^(BenchmarkTableWritePath|BenchmarkSendPath|BenchmarkSendPayloadPath)$' --dir internal/runtime --pairs 10`
  8. `pmset -g batt` again. A session that ran on battery is discarded.
  - Expected exit 0. `applyWrite` makes the same calls that `writeTable` made, so allocs/op should match exactly.
  - Exit 1 on BenchmarkTableWritePath triggers ground rule 13a: the helper stays unsplit, `writeTable` keeps its own body, and an architecture review of the area follows.
- **T2 green.**

**Conditions unchanged from the addendum:**
- Condition A, `table.Check`, becomes its own commit before W2, outside E1. Item 10 says the owner's six-agent lift settles DA-S2.
- Condition B, a built app takes no starting rows, is in C7.
- Steps 2 to 5 of the addendum's change are unchanged.

### Updated commit plan (`internal/runtime`)

| # | Kind | Findings | Proofs |
|---|---|---|---|
| (before W2, not E1's) | coordinator or CD | `table.Check` in internal/table | its own test, and the revert plant `return nil` reads red |
| C1 | K2-del | G2-113, G2-129 | each deleted test's blocks ⊆ the remaining suite's two runs; "no stream" covers shard.go:182-184; package covered set equal |
| C2 | K2 part 1 | G2-112, G2-116, G2-117, G2-118 (timeout 180), G2-128, G2-131 | `--expect` entries with controls; the slowdown plant; the AST check; load runs (G2-131's names all three tests) |
| C3 | K2 part 2 | G2-111, G2-114, G2-115, G2-120, G2-122, G2-125, G2-127, G2-132 (timeout 180) | `--expect` entries with controls; coverage blocks; the grep check; timeout → red; load runs |
| C4 | K1 | G7a-5 | revert plant red; T2 |
| C5 | K1c | G10-20 | `mutate.py --race --repeat 50 --load --test TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained`, k ≥ 1; load 50/50 |
| C6 | K1h | G10-21 | revert plant red (the stretch between Stop and the drain item, and after the drain); load 50/50; compare.py on Send/SendPayload as a collateral-cost check, exit 0 |
| C7 | K1h | G10-1 runtime half, plus the new `BenchmarkTableWritePath` | four revert plants red; load 50/50; compare.py on the table-write, Send and SendPayload benchmarks, parent with overlay against commit, exit 0 |

**Seven E1 commits:** six lane commits (C2 to C7), which is exactly the ceiling of 4 + 2, plus one K2-del (C1) from the shared pool. C7 comes after the `table.Check` commit; C1 to C6 do not depend on it. Because the lane is at its ceiling, there is no spare commit for a fix round without the coordinator's re-projection.
