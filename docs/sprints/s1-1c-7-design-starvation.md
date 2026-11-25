# S1.1c-7 design note: the recorder's starvation at one CPU

*The design note for the recorder's starvation fix, by a fresh agent on 2026-11-22, as given; `S/` is its scratch folder. The owner approved the chosen design the same day ([decisions](decisions.md#owner-decisions-2026-11-22--pr-2s-review)): at most two yields; the big-write case of Q2 becomes a known issue; Q3's sentence goes into `Config.RecordQueue`'s doc.*

*(annotated 2026-11-22: candidate 2 was built in the end. Under CPU load candidate 1's waiting flag read a recorder paused mid-record as busy and its test failed 34–37 of 50 runs, while candidate 2 passed 50 of 50, with and without `-race`; [the owner's decision](decisions.md#owner-decisions-2026-11-22--the-starvation-fix-under-load). §3.1's `waiting` field and its two stores per record are gone, and §3.4's second test asserts exactly two turns for a blocked Recorder.)*

Status: design only, nothing built. Base `730fa8e3` (`s1-1c-7/integration`), pinned Go 1.27.4.

**Read this first**

- **Cause confirmed, with two corrections to the hypothesis.** At one CPU the caller and the router pass the processor to each other through Go's `runnext` slot, so the recorder goroutine sits in the local run queue and is not run at all.
  - Waiting for a reply is not needed: `BatchEnd`, `Tick` and late `Send` starve it too (run with `Intake: 1`).
  - "One record per `select`" is not a factor: the recorder had taken nothing when the queue filled.
- **Chosen fix:** at a full queue, if the recorder goroutine is waiting for the processor, the router yields (`runtime.Gosched`) at most twice and offers the record again. Nothing is added to the router's per-event path while the queue has room.
- **Prototype at one CPU:** 0 red of 1,800 runs with two yields; 1,152 red of 1,800 with one yield.
- **Not measured:** the fix adds two atomic stores per record on the recorder's goroutine. I have no before/after timing; `compare.py` must judge it (rule 13a).
- **Found on the way, outside this fix:** one `TableWrite` of more rows than the queue ends the recording at any CPU count (Q2).

## 1. The cause, proven at one CPU

Scratch tests: package `runtime`, a live app, the discarding recorder `igRec`, `Shards: 1`, a fixed clock, `-test.cpu=1`. "q=64" is `RecordQueue: 64` with 4,096 calls; "default" is `RecordQueue` 65,536 with 400,000 calls.

### 1.1 Red on main

`RecordQueue: 64`, 4,096 synchronous `TableWrite` calls, then `Stats`:

```
GOMAXPROCS=1 n=4096 1144 ns/op recording={Records:64 Failed:true Err:runtime: recording queue full after record 64: the recording stopped; detection goes on}
--- FAIL: TestZZTableWrite (0.01s)
```

- 50 of 50 runs red.
- "Full after record 64" with 64 slots means the recorder took nothing before the queue filled.
- Default queue, 400,000 calls: `Records:147080 Failed:true`, full at record 147,080. At about 1.4 µs per call, the recorder got no turn for about 92 ms.

### 1.2 What makes it pass, and what does not (q=64, one CPU)

**The one experiment that shows the cause:** the caller calls `runtime.Gosched()` after each `TableWrite`, nothing else changed. The test passes with `Records:4097`. The recorder is runnable; it only lacks a turn.

| Scratch change | Result | What it shows |
| --- | --- | --- |
| Caller yields after each `TableWrite` | pass, `Records:4097` | The recorder is runnable and unscheduled. |
| Router yields after `applyWrite`, before the reply | pass, `Records:4097` | Same, from the router's side. |
| Router sends the reply first, the records after | red, `Records:64` | The order of wake-ups is not the lever. |
| At a full queue: one yield, then retry | red in 1,152 of 1,800 | One yield is often handed straight back. |
| At a full queue: two yields, then retry | 0 red of 1,800 | Two yields reach the recorder. |
| `GOGC=off`, default queue, `TableWrite` | pass, 393,403 records | See 1.3. |
| `GOGC=off`, default queue, `BatchEnd` | red, full at 65,536 | See 1.3. |

A shell mistake of mine turned five intended fix runs into base runs: 6,000 of 6,000 red across six tests (`TableWrite`, `BatchEnd`, `Tick`, late `Send`, two `Send` configurations). Those binaries carried the flag's stores but no yield.

### 1.3 The scheduler's part

Proven by the table: the recorder is runnable and unscheduled; a yield by either partner fixes it; one yield is not enough and two are.

Read in the pinned Go's `src/runtime/proc.go` and consistent with the runs, but **reasoned, not proven**:

- A wake-up puts the woken goroutine in `runnext`, displacing the previous one to the run-queue tail (`runqput`, line 7529).
- A parking goroutine's successor is taken from `runnext` first, so caller and router alternate without the run queue being served.
- A goroutine that yields goes to the global queue. Every 61st scheduling tick the global queue is served first (line 3458), which hands the processor straight back to the router. That would explain why one yield fails and two do not.
- The only other breaker is the forced preemption after 10 ms on one tick.
  - Fast paths (220 to 380 ns per call) fill 65,536 slots in 14 to 25 ms, inside that interval, even with GC off.
  - The slow `TableWrite` path survives with GC off and starves with GC on. My explanation, that GC workers' scheduling resets that clock without serving the run queue, is not proven.
- Under `-race` the scheduler uses `runnext` half the time (line 7541). Base was red in only 20 of 120 race runs, so the red proof must be a non-race run.

### 1.4 Siblings

| Path | At one CPU | Evidence |
| --- | --- | --- |
| Recorder via `TableWrite` | starved | **proven**, q=64 and default |
| Recorder via `BatchEnd` (`Intake: 1`) | starved | **proven**, q=64 and default (`Records:0` at the failure) |
| Recorder via `Tick` (`Intake: 1`) | starved | **proven**, q=64 and default |
| Recorder via late `Send` (`Intake: 1`) | starved | **proven**, q=64 and default (late=400000) |
| On-time `Send`, rule and sink, default queues | keeps its recording | **proven**: 400,000 events, not Failed |
| On-time `Send`, q=64, default `Intake` and `Mailbox` | starved | **proven**: the router handles a backlog bigger than the queue without parking |
| On-time `Send`, q=64, one-slot queues | keeps its recording | **proven** |
| Recorder via rejected payloads | starved, expected | reasoned only: `ingestReject` has the shape of `ingestBatchEnd` |
| The proven paths at the default `Intake` 1024 | starved, expected | reasoned only, not run |
| `Drain`'s stop record after such a burst | lost with the recording | reasoned only: same `put` |
| Sinks' queues, shards' mailboxes | no loss | reasoned only: `router.sendHead` blocks on them, so the router parks and the run queue is served |
| `Stats` loop | no loss | reasoned only: it puts no record and goes through the shards |
| `Activate` | no loss | reasoned only: the queue is sized for the starting rows (`app.go:75`) |

- The recording queue is the only lossy hand-off among the files I read.
- `Send` is safe at default sizes because the shard and sink goroutines park with `runnext` empty, which serves the run queue.

Two more findings, both present on main:

- **Small queues at 2 or more CPUs.** q=64 `TableWrite` at 2 CPUs is red on main (`Records:3119`). Across six tests at 2 and 14 CPUs, base was red in 550 of 1,200. An early prototype was red in 405 of 1,200, but that run had the flag starting false, so it says little about the final fix. A 64-slot queue cannot absorb a thread wake-up, so the red-first test must pin one CPU itself.
- **One big write.** One `TableWrite` of 262,144 rows with the default queue ends the recording at 14 CPUs (full at 74,538) and at one CPU. The router's put loop outruns even a discarding recorder. The fix does not change it (Q2).

### 1.5 Everything not proven, in one list

- The scheduler mechanism of 1.3, including the GC explanation.
- The sibling rows marked "reasoned only" in 1.4.
- Unread `default:` branches in non-test code: `drain.go:108,135`, `boundary.go:59,81`, `transitions.go:86,125`. A reviewer should confirm none is a lossy hand-off.
- The cost of the recorder's two atomic stores per record.
- Behaviour when many runnable goroutines sit ahead of the recorder (no load run was allowed).
- The red-first test at its final length and with its exact record count; the second test; plants 3 to 5 (§3.4, §3.5).
- That `Close` waits for the router goroutine (§3.4).

## 2. Candidates, ordered by accuracy and code health, and the choice

1. **Yield at a full queue only when the recorder goroutine is waiting; at most two yields. (Chosen.)**
   - Pro: the router yields only when the recorder has not had the processor, so a slow or stuck `Recorder` still ends the recording at once. Measured: 2 µs per record, 14 CPUs, 0 yields, Failed as on main.
   - Pro: no operation added to the router's path while the queue has room.
   - Con: one new field with an invariant, and two atomic stores per record on the recorder's goroutine (untimed).
   - Con: "two" is tied to the scheduler's fairness check.
2. **Yield at any full queue, at most twice, no flag.**
   - Pro: simplest; no cost on either goroutine while the queue has room; 0 red of 1,800.
   - Con: with a `Recorder` that is slow but moving, the retry can succeed and the router is then paced by the recorder, against D2. Measured once at 2 µs per record: 3 yields, then ended. The pacing case for faster recorders is reasoned only.
3. **Router yields every N records, full or not.**
   - Pro: no flag.
   - Con: yields when the recorder is keeping up, adding a scheduling pause to detection for nothing; N must track `RecordQueue` and rows per write.
4. **Callers yield after a reply.**
   - Con: does not cover `BatchEnd`, `Tick` or `Send`; slows every `TableWrite` (scratch reading 1,524 against 1,144 ns/op, single readings).
5. **A bigger queue.**
   - Con: not a fix; the pair ran unbroken for 92 ms, and any size fills.
6. **Router blocks until the recorder has taken a record.**
   - Con: exact for the recording but breaks D2. Rejected.

Why candidate 1: it is the only one that keeps D2 exactly as written while fixing every proven path. Candidate 2 is the fallback if the stores fail rule 13a.

## 3. The chosen design, build-ready

### 3.1 `internal/runtime/recorder.go`

Imports (lines 3 to 8) become:

```go
import (
	"fmt"
	goruntime "runtime"
	"sync"
	"sync/atomic"

	"github.com/vorsar/cep/internal/event"
)
```

After the `recQ` type (after line 66) add:

```go
// recTurns is how many times the router gives the processor away at a full queue whose goroutine is waiting for it
// (design s1-1c-7-design-starvation §1.3): one yield can be handed straight back by the scheduler's fairness check,
// the second reaches the run queue. More would make detection wait for the recording (owner D2 of S3.2b-2's design).
const recTurns = 2
```

In `recQ`, after `isSettled` (line 59), add:

```go
	// the goroutine's: true from the queue's making, and whenever it is at its select having taken no record since;
	// false while it handles one. The router reads it at a full queue only.
	waiting atomic.Bool
	turns   int64 // the router's: the yields it made at a full queue
```

`newRecQ` (lines 68 to 71) becomes:

```go
func newRecQ(rec Recorder, capacity int) *recQ {
	q := &recQ{q: make(chan recItem, capacity), rec: rec, overflow: make(chan int64, 1), stop: make(chan struct{}),
		returned: make(chan struct{}), settled: make(chan struct{})}
	q.waiting.Store(true) // not yet started is waiting too: a goroutine that has never run has taken nothing
	return q
}
```

In `put`, the `default:` branch (line 103) gains three lines at its top:

```go
	default:
		if q.turn(it) {
			return true
		}
		q.ended = true
		// ... the rest unchanged
```

New method after `put`:

```go
// turn is put's step at a full queue: while the goroutine is waiting for the processor, having taken nothing, the
// router yields it and offers the record again, at most recTurns times. A goroutine that is handling a record gets no
// yield: its Recorder is behind, and the recording ends (owner D2 of S3.2b-2's design).
func (q *recQ) turn(it recItem) bool {
	for i := 0; i < recTurns && q.waiting.Load(); i++ {
		q.turns++
		goruntime.Gosched()
		select {
		case q.q <- it:
			q.queued++
			return true
		default:
		}
	}
	return false
}
```

In `run`, the loop head (lines 161 to 164) becomes:

```go
	for {
		q.waiting.Store(true)
		select {
		case it := <-q.q:
			q.waiting.Store(false)
			q.handle(it)
```

- The scratch had this logic inline in `put`, without the `turn` helper and without the `turns` counter's name (it counted yields the same way).
- The start value matters: with `waiting` starting false, 295 of 300 runs were red, because a recorder that has never run was read as busy.

### 3.2 Cost

- Router, queue has room: 0 operations added per event.
- Router, queue full: one atomic load, then up to two `Gosched` calls and two channel offers. Measured after the fix at one CPU, default queue: 2 to 6 yields per 400,000 calls.
- Recorder goroutine: two atomic stores per record. Not timed.
  - `waiting` sits beside `mu` and `records`, which that goroutine already writes per record, so no new cache line is shared with the router.
  - That sharing would only show at more than one CPU, which the one-CPU method does not see.

### 3.3 The law path, `TestNoSleepOrPollingInRuntime`

- The test parses every non-test file of `internal/runtime`. It fails on `time.Sleep`, `time.After`, `time.Tick` or `time.AfterFunc` under any import name, and on a dot import of `time`. `time.NewTicker` is not banned.
- The fix uses `runtime.Gosched` and `sync/atomic` and no `time` call, so it passes the test as written.
- In spirit: the loop is bounded at two iterations and entered only at a full queue, so it is not a polling loop. The reviewers should still judge that, since the test cannot.

### 3.4 The red-first test

File `internal/runtime/recorder_turn_internal_test.go`, package `runtime`.

```go
package runtime

import (
	"context"
	goruntime "runtime"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// TestRecorderGetsATurnAtOneCPU (design s1-1c-7-design-starvation): on one P, a caller that feeds the router in a
// tight loop does not end the recording: the router gives the waiting recorder goroutine the processor at a full
// queue. Not parallel: it sets GOMAXPROCS for the whole process.
func TestRecorderGetsATurnAtOneCPU(t *testing.T) {
	defer goruntime.GOMAXPROCS(goruntime.GOMAXPROCS(1))
	const queue, n = 64, 64 * 1024
	row := [][]event.Value{{{Kind: event.TString, Str: "k"}, {Kind: event.TInt, I: 1}}}
	feeds := []struct {
		name  string
		sink  bool
		first bool // one on-time event before the loop, so the loop's events are late
		feed  func(ctx context.Context, r *Runtime, id AppID) error
	}{
		{"TableWrite", false, false, func(ctx context.Context, r *Runtime, id AppID) error {
			return r.TableWrite(ctx, id, 0, TablePut, row)
		}},
		{"BatchEnd", false, false, func(ctx context.Context, r *Runtime, id AppID) error {
			return r.BatchEnd(ctx, id, 0, "b")
		}},
		{"Tick", false, false, func(ctx context.Context, r *Runtime, _ AppID) error { return r.Tick(ctx, cgT0) }},
		{"late Send", true, true, func(ctx context.Context, r *Runtime, id AppID) error {
			return r.Send(ctx, id, cgEv(0))
		}},
	}
	for _, f := range feeds {
		t.Run(f.name, func(t *testing.T) {
			ctx := cgCtx(t)
			r, err := New(Config{Shards: 1, Intake: 1, RecordQueue: queue, Now: func() int64 { return cgT0 }})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = r.Close(ctx) })
			tb, err := table.New("T", []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}},
				[]string{"k"})
			if err != nil {
				t.Fatal(err)
			}
			app := cgApp(t, func(event.Event) error { return nil })
			if !f.sink {
				app.Sinks = nil
			}
			app.Recorder, app.Tables = igRec{}, table.NewSet([]*table.Table{tb})
			id, err := r.Add(ctx, app)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Activate(ctx, id); err != nil {
				t.Fatal(err)
			}
			want := int64(1 + n + 1) // the header, the loop's records, the stop
			if f.first {
				if err := r.Send(ctx, id, cgEv(n)); err != nil {
					t.Fatal(err)
				}
				want++
			}
			for range n {
				if err := f.feed(ctx, r, id); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.Drain(ctx, id); err != nil {
				t.Fatal(err)
			}
			st, err := r.Stats(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if rs := st.Recording[id]; rs.Failed || rs.Records != want {
				t.Fatalf("recording %+v, want %d records and no failure", rs, want)
			}
		})
	}
}
```

Why it is deterministic at one CPU with no sleep:

- Before the fix, nothing in the loop yields, and 64 slots fill in well under one forced-preemption interval. Red in 6,000 of 6,000 base runs.
- After the fix, every full queue is met with a turn. Green in 1,800 of 1,800.
- The loop is 65,536 calls, not 4,096, so the "one yield" plant meets the fairness check about a thousand times.

**Not run in this exact form.** The scratch asserted `Failed` only, with 4,096 calls and no `Drain`. The exact `Records` count and the 65,536 length are new and uncompiled. If a count is off for a reason outside the fix, the builder reports it and does not loosen the assertion.

**A second test guards D2:** `TestFullQueueWithABusyRecorderGetsNoTurn`, same file. Not written or run by me; this is its specification, corrected from my first copy.

- A `Recorder` whose `Table` closes an `entered` channel on its first call and then blocks on a `release` channel; both belong to the test. `RecordQueue: 4`.
- The test makes one `TableWrite`, then waits on `entered`. Only then is the recorder known to be inside its `Recorder`, with `waiting` false.
- It then calls `TableWrite` until `Stats` shows `Failed`, closes `release`, calls `Close`, reads `r.appList()[id].rec.turns` and wants 0.
- Why the wait on `entered` is needed: without it the queue can fill before the recorder has woken, the router then rightly gives it a turn, and the test flakes. I measured that wake-up gap on main at q=64 and 2 CPUs.
- Assumption to confirm: `Close` waits for the router goroutine (`r.wg`), which makes the read of `turns` safe. I did not read `shutdown.go`.

### 3.5 Revert plants

| # | File | Find | Replace | Must turn red |
| --- | --- | --- | --- | --- |
| 1 | `recorder.go` | `goruntime.Gosched()` in `turn` | delete the line | `TestRecorderGetsATurnAtOneCPU`, all four |
| 2 | `recorder.go` | `q.waiting.Store(true) // not yet started` | delete the line | `TestRecorderGetsATurnAtOneCPU`; builder records which subtests |
| 3 | `recorder.go` | `const recTurns = 2` | `const recTurns = 1` | `TestRecorderGetsATurnAtOneCPU`; confirm with 20 runs |
| 4 | `recorder.go` | `q.waiting.Store(true)` at the loop head in `run` | delete the line | `TestRecorderGetsATurnAtOneCPU/TableWrite` |
| 5 | `recorder.go` | `q.waiting.Store(false)` in `run` | delete the line | `TestFullQueueWithABusyRecorderGetsNoTurn` |

Plants 1 and 2 match scratch runs (6,000 of 6,000, and 295 of 300 at the short length). Plants 3 to 5 are unrun at the final test length.

### 3.6 The benchmark change

`internal/runtime/table_write_bench_test.go`: the header comment keeps its text; the function is replaced by:

```go
// BenchmarkTableWritePath times one TableWrite of one row (the same key each time, so the table keeps one row) on a
// live app with no recording (owner, 2026-11-22): the caller's checks, the intake, the router's write step and the
// reply.
func BenchmarkTableWritePath(b *testing.B) { benchTableWrite(b, nil) }

// BenchmarkTableWritePathRecorded is the same write with a recording that discards (igRec): the record per row is
// timed too. A recording that fell behind and failed would measure another path, so the run fails then; at one CPU
// it passes only because the router gives the recorder its turn (design s1-1c-7-design-starvation).
func BenchmarkTableWritePathRecorded(b *testing.B) { benchTableWrite(b, igRec{}) }

func benchTableWrite(b *testing.B, rec Recorder) {
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
	app.Sinks, app.Recorder = nil, rec
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
	for b.Loop() { // the timer stops when it returns false: the Stats read below is not timed
		if err := r.TableWrite(ctx, id, 0, TablePut, rows); err != nil {
			b.Fatal(err)
		}
	}
	if rec == nil {
		return
	}
	st, err := r.Stats(ctx)
	if err != nil {
		b.Fatal(err)
	}
	if rs := st.Recording[id]; rs.Failed {
		b.Fatalf("recording %+v: it fell behind, so the run measured another path", rs)
	}
}
```

- Uncompiled in this form.
- Only this file names `BenchmarkTableWritePath` among the repo's `.go`, `.py` and `.json` files; I did not search the docs.

### 3.7 Runs for the coordinator

- **Red:** the new test on the test-only commit, non-race, read the exit code. Then green on the fix commit, plain and `-race`, and with `-cpu 1,4`.
- **Recorded benchmark:** `BenchmarkTableWritePathRecorded` at `-test.cpu=1 -test.benchtime=1s`, three runs; all must pass. On the parent they fail.
- **Timings, rule 13a:** `compare.py`, base `730fa8e3` against the fix, on `BenchmarkSendPath`, `BenchmarkSendPayloadPath` and `BenchmarkTableWritePath`. I did not read `compare.py`; use its usual one-CPU method.
  - The recorded variant has no base to compare with: on the base it fails.
  - If a recorded path is 5 % or more slower, confirmed by a re-run, the fix stops there. Candidate 2 is the fallback to put to the owner.
- **Extra, informational:** the recorded Send world at the default CPU count, before and after, for the cache-line concern in 3.2.
- **Load:** the usual load run over `internal/runtime`, plus the new test under load. Watch for the recorder sitting behind many runnable goroutines, where two yields may not reach it. The recording then ends as it does today.

### 3.8 Doc text

- `runtime.go` lines 88 to 90, `Config.RecordQueue`: "RecordQueue is each recording's queue, in records, plus one slot per starting row of the app (App.Loads); 0: 65,536. A full queue ends the recording (owner D2 of S3.2b-2's design), unless the recorder's goroutine has not had the processor: the router then gives it up, at most twice, and offers the record again."
- `recorder.go` lines 40 to 44, `recQ`: change "puts records on without ever blocking" to "puts records on without ever blocking (at a full queue it yields the processor at most recTurns times to a goroutine that is waiting for it, and no more)".
- `recorder.go` lines 89 to 90, `put`: "A full queue ends the recording, with the record it could not take named, once turn has not made room."
- `recorder.go` lines 154 to 156, `run`: add "It marks itself waiting at its select, so the router can tell a goroutine without the processor from a Recorder that is behind."
- `docs/sprints/s1-1c-7-record.md`: the proofs, by the coordinator.

### 3.9 Commits, one kind each

1. `test(runtime)`: `TestRecorderGetsATurnAtOneCPU` and `TestFullQueueWithABusyRecorderGetsNoTurn`, red.
2. `fix(runtime)`: `recorder.go` as in 3.1, with its comments.
3. `test(runtime)`: the benchmark split of 3.6.
4. `docs`: `Config.RecordQueue`'s text, this note, the record's proofs.

The second test reads `turns`, which commit 1 does not have. Either move that test to commit 2, or add `turns` in commit 1 as an unused counter.

### 3.10 Pre-mortem

- **"Two" is scheduler lore.** A Go upgrade may change the fairness check. The one-CPU test is the alarm; plant 3 keeps it honest.
- **A yield is a wait in disguise.** It is bounded at two, taken only when the recorder is not in its `Recorder`, and measured at 2 to 6 per 400,000 calls.
- **Saturation.** With many runnable goroutines ahead of the recorder, two yields may miss it. The outcome is today's, not worse. Unproven.
- **The stores' cost.** Untimed; `compare.py` decides.
- **Process-wide `GOMAXPROCS(1)` in a test.** Precedent: `close_gate_test.go:226`.

## 4. Questions for the owner

**Q1. How long may the router keep yielding at a full queue whose recorder is waiting?**
- Recommended: at most twice, then the recording ends as today. Detection's pause stays bounded and D2 stays as written.
- Alternative: keep yielding until the recorder has run. No recording is ever lost to scheduling, but detection pauses for as long as the scheduler takes, which on a saturated machine is open-ended.

**Q2. One `TableWrite` of more rows than the queue ends the recording at any CPU count. Fix now or forward?**
- Recommended: record it as a known issue and forward it (rule 7). It is old, separate from the starvation, and its fix is a design choice of its own (refuse such a write, or record a write as one record).
- Alternative: widen this fix, which changes the recording's format or `TableWrite`'s contract inside PR 2.

**Q3. May `Config.RecordQueue`'s doc say that small queues are fragile?**
- Recommended: yes, one sentence: the queue must be large enough to cover the recorder's wake-up, and values far below the default can end a recording under bursts at any CPU count. Measured: q=64 at 2 CPUs is red on main.
- Alternative: a minimum enforced in `build.go`, which is a configuration change users can see.

## Appendix A. Method and limits

- `ns/op` figures are single `time.Since` readings, not benchmarks.
- No gate tiers, `load.py`, `stress.py` or `compare.py` were run.
- Index freshness not checked: this session has no tool to read `gitnexus://repo/cep/context`. I cross-checked callers by reading the files at `730fa8e3`.
- memory: none used.
- Skills read: `gitnexus-impact-analysis` (first 120 of 167 lines), `architecture-designer`, `the-fool`. The-fool's mode question needs `AskUserQuestion`, which a subagent lacks; I ran a pre-mortem (§3.10).

## Appendix B. GitNexus blast radius

| Symbol | Risk | Direct | Note |
| --- | --- | --- | --- |
| `recQ.put` (`recorder.go:91`) | MEDIUM, lower-bound | 6 | 32 in total; process `route`. One call site dropped by the index: `tickApps`, `router_in.go:92`, found by reading. |
| `recQ.run` (`recorder.go:157`) | LOW, lower-bound | 1 | 46 unresolved `run` sites by name. Text search of `build.go`, `lifecycle.go`, `transitions.go`, `app.go` finds one caller: `lifecycle.go:138`. |
| `recQ.handle` | LOW, exact | 2 | not changed |
| `newRecQ` | LOW, exact | 1 | caller `newAppState`, `app.go:75` |
| `appState.applyWrite` | LOW, exact | 1 | not changed |
| `router.writeTable` | LOW, exact | 1 | not changed |
| `Runtime.TableWrite` | MEDIUM, lower-bound | 6 | not changed |
| `Runtime.enqueue` | MEDIUM, exact | 10 | not changed |

- No HIGH, CRITICAL or UNKNOWN.
- `put`'s callers, by reading: `newAppState` (header), `drainStop` (twice), `applyWrite`, `ingestBatchEnd`, `ingestEvent`, `ingestReject`, `tickApps`.
- `detect_changes` on the scratch worktree, before its removal: risk "high", 8 changed symbols, 11 affected flows, all `Run → …` through `handle`. That describes my scratch edits; the real fix touches `recQ`, `newRecQ`, `put` and `run` and will read much the same.
