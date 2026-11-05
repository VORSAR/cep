package playback_test

// Exit lines 1 and 2 of S3.2a-1 (design note v2 §6: Exit 1, Exit 2), through the router double of
// helpers_log_test.go.

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/playback"
)

// ldExit1App: A (d 1 min), B (d 0), C (`time arrival`, d 30 s); table T(x, n) keyed by x.
func ldExit1App(t *testing.T) *playback.App {
	return &playback.App{
		Streams: []playback.Stream{ldStd(t, "A", ldMin, 0), ldStd(t, "B", 0, 0), ldArrival(t, "C", 30*ldSec, 0)},
		Tables:  []playback.Table{ldTableT()},
	}
}

// ldExit1Rules: R1 reads A,B (TTL 2 min; a Completion at +30 s and a Discard at +45 s per release); R2 reads B,C
// (TTL 1 min; a DelayDue at +10 s); R3 reads A,C (TTL 0; a Completion at +5 s; notes `x in T` through the event's
// own snapshot and the row count through the timer's).
func ldExit1Rules(t *testing.T) []ldRuleSpec {
	return []ldRuleSpec{
		{Name: "R1", Inputs: []int{0, 1}, Key: "k", TTL: 2 * ldMin, Sched: func(e event.Event, n uint64) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 30*ldSec, Kind: clock.Completion, Ref: n}, {Due: e.Ts + 45*ldSec, Kind: clock.Discard, Ref: 1000 + n}}
		}},
		{Name: "R2", Inputs: []int{1, 2}, Key: "k", TTL: ldMin, Sched: func(e event.Event, n uint64) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 10*ldSec, Kind: clock.DelayDue, Ref: n}}
		}},
		{Name: "R3", Inputs: []int{0, 2}, Key: "k", Sched: func(e event.Event, n uint64) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 5*ldSec, Kind: clock.Completion, Ref: n}}
		}, OnEvent: func(w *ldWorld, rule string, in *partition.Instance, e event.Event) {
			if e.StreamID != 0 { // only stream A has x (index 2); C's events have two values
				return
			}
			if x := e.Values[2]; !x.Null {
				w.note(rule, in, "x_in_T", event.Value{Kind: event.TBool, B: ldHasX(t, e.Tables, x.Str)})
			}
		}, OnTimers: func(w *ldWorld, rule string, in *partition.Instance, fires []clock.Timer, tables any) {
			ldHasX(t, tables, "p")
		}},
	}
}

func ldExit1Log(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/log/exit1.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Exit 1: the fixture testdata/log/exit1.jsonl — three streams, keys a, b, c, out-of-order and late events, table
// records and batch ends — replayed twice gives byte-identical traces.
//
// Hand derivation of the lateness (design §5.3; SEMANTICS §2.1; times as seconds after 10:00:00; vnow = the
// largest ts so far; a stream's contribution = max(own admission maxTs − d, vnow − d)):
//
//	L1 A@0    A −60, B 0, C −30             L5 A@20  A −40, B 20, C −10
//	L2 B@5    B 5, A −55, C −25             L7 B@2   B's admission is 5 > 2: STREAM-LATE
//	L3 C@3    C's admission −27: nothing    L8 C@60  C 30, A 0, B 60 → R1 0, R2 min(60,30)=30, R3 0
//	L9 B@10   admitted by B (10 ≥ 5); R1's clock 0 ≤ 10 admits it; R2's clock 30 > 10: CONSTRUCT-LATE for R2 only
//	L11 B@120 B 120, A 60, C 90             L12 A@90 A's admission 30 ≤ 90; R1, R3 at 60: admitted
//	L13 C@10  C's admission 30 > 10: STREAM-LATE
//	L14 A@180 A 120, B 180, C 150           L16 B@180 admitted everywhere
//
// So stream-late = lines {7, 13}; construct-late = [(R2, c, line 9)]. The end (line 17) raises everything to +∞:
// every resident key of R1 and R2 (TTL > 0) purges there (QA A9); R3 (TTL 0) never purges.
func TestTraceDeterministic(t *testing.T) {
	t.Parallel()
	log := ldExit1Log(t)
	var traces [][]byte
	var first *ldWorld
	for run := 0; run < 2; run++ {
		w := ldRun(t, ldExit1App(t), log, ldExit1Rules(t)...)
		b, err := w.tr.Bytes()
		if err != nil {
			t.Fatalf("run %d: Trace.Bytes: %v", run, err)
		}
		if len(b) == 0 {
			t.Fatalf("run %d: an empty trace", run)
		}
		traces = append(traces, b)
		if first == nil {
			first = w
		}
	}
	if string(traces[0]) != string(traces[1]) {
		t.Fatalf("two replays of one fixture gave different traces (exit 1):\n%s\n---\n%s", traces[0], traces[1])
	}
	// The fixture exercises what exit 1 names.
	var sl []int
	for _, e := range ldFilter(first.log, "stream_late") {
		sl = append(sl, e.At)
	}
	if !slices.Equal(sl, []int{7, 13}) {
		t.Errorf("stream-late lines %v, want [7 13]%s", sl, ldEntries(first.log))
	}
	cl := ldFilter(first.log, "construct_late")
	if len(cl) != 1 || cl[0].At != 9 || cl[0].Rule != "R2" || cl[0].Key != "c" {
		t.Errorf("construct-late %v, want exactly R2/c at line 9%s", cl, ldEntries(first.log))
	}
	kinds := map[playback.StepKind]int{}
	for _, s := range first.steps {
		kinds[s.Kind]++
	}
	want := map[playback.StepKind]int{playback.StepEvent: 10, playback.StepStreamLate: 2, playback.StepTable: 2, playback.StepBatchEnd: 2, playback.StepEnd: 1}
	for k, n := range want {
		if kinds[k] != n {
			t.Errorf("%d steps of kind %d, want %d", kinds[k], k, n)
		}
	}
	for _, k := range []string{"event", "timer", "discard", "purge", "note"} {
		if len(ldFilter(first.log, k)) == 0 {
			t.Errorf("the fixture produced no %q entry: it must exercise every entry kind%s", k, ldEntries(first.log))
		}
	}
	for _, e := range ldFilter(first.log, "purge") {
		if e.Rule == "R3" {
			t.Errorf("R3 has TTL 0 and never purges: %v", e)
		}
	}
}

// Exit 1's proof: the same harness, with a body that writes its notes by ranging over a Go map of 16 entries,
// gives at least two different traces within 20 replays. So TestTraceDeterministic can fail.
func TestTracePlantedMapOrderIsRed(t *testing.T) {
	t.Parallel()
	log := ldExit1Log(t)
	planted := map[string]int64{}
	for i := 0; i < 16; i++ {
		planted[fmt.Sprintf("m%02d", i)] = int64(i)
	}
	seen := map[string]bool{}
	for run := 0; run < 20 && len(seen) < 2; run++ {
		specs := ldExit1Rules(t)
		specs[0].OnEvent = func(w *ldWorld, rule string, in *partition.Instance, e event.Event) {
			for name, v := range planted { // planted: a Go map's order reaches the output
				w.note(rule, in, name, event.Value{Kind: event.TInt, I: v})
			}
		}
		w := ldRun(t, ldExit1App(t), log, specs...)
		b, err := w.tr.Bytes()
		if err != nil {
			t.Fatalf("run %d: Trace.Bytes: %v", run, err)
		}
		seen[string(b)] = true
	}
	if len(seen) < 2 {
		t.Fatal("20 replays with a map-ordered body gave one trace: the determinism test could not catch map order")
	}
}

// Exit 2 (design §2; owner D1 of S3.1-2, D5): a table record becomes visible from the next event, and a timer reads
// the snapshot of the step whose walk fired it.
//
// Put: L1 E1 (S, d 1 min) at 10:00; L2 put T{x1}; L3 E2 at 10:02.
//   - L1: S's admission 9:59 → the clock 9:59; E1 (10:00) is buffered, not released (D1: needs 10:00 < clock).
//   - L3: the clock 10:01 > 10:00 → E1 is released in L3's walk. E1.Tables is L1's snapshot (before the put):
//     x1 in T = false. Its Completion (10:00:30 < 10:01) fires in the same walk and reads Advance's tables = L3's
//     Step.Tables (after the put): true.
//   - End (line 4, +∞): E2 is released and reads its own snapshot (after the put): true; its Completion fires with
//     the End step's snapshot: true.
//
// Delete: L1 put T{x1}; L2 E1 at 10:00; L3 delete T{x1}; L4 E2 at 10:02 — the same timing one line later, every
// answer inverted: E1 true, its timer (L4's snapshot) false, E2 false, E2's timer false.
func TestTableRecordFromNextEvent(t *testing.T) {
	t.Parallel()
	put := `{"table":"T","op":"put","row":{"x":"x1"}}`
	del := `{"table":"T","op":"delete","row":{"x":"x1"}}`
	e1 := ldE("S", ldAt(0), "a", "x1", "")
	e2 := ldE("S", ldAt(120), "a", "x1", "")
	cases := []struct {
		name     string
		log      string
		e1Line   int // E1's line: its event's Tables must be that step's snapshot
		walkLine int // E2's line: the walk that releases E1 and fires its timer
		want     []ldEntry
	}{
		{"put", ldLog(e1, put, e2), 1, 3, []ldEntry{
			{At: 3, K: "note", Rule: "R", Key: "a", Name: "x_in_T", Val: "false"},
			{At: 3, K: "note", Rule: "R", Key: "a", Name: "timer_x_in_T", Val: "true"},
			{At: 4, K: "note", Rule: "R", Key: "a", Name: "x_in_T", Val: "true"},
			{At: 4, K: "note", Rule: "R", Key: "a", Name: "timer_x_in_T", Val: "true"},
		}},
		{"delete", ldLog(put, e1, del, e2), 2, 4, []ldEntry{
			{At: 4, K: "note", Rule: "R", Key: "a", Name: "x_in_T", Val: "true"},
			{At: 4, K: "note", Rule: "R", Key: "a", Name: "timer_x_in_T", Val: "false"},
			{At: 5, K: "note", Rule: "R", Key: "a", Name: "x_in_T", Val: "false"},
			{At: 5, K: "note", Rule: "R", Key: "a", Name: "timer_x_in_T", Val: "false"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			app := &playback.App{Streams: []playback.Stream{ldStd(t, "S", ldMin, 0)}, Tables: []playback.Table{ldTableT()}}
			var timerTables []any
			var e1Tables any
			spec := ldRuleSpec{Name: "R", Inputs: []int{0}, Key: "k", TTL: 10 * ldMin,
				Sched: func(e event.Event, n uint64) []clock.Timer {
					return []clock.Timer{{Due: e.Ts + 30*ldSec, Kind: clock.Completion, Ref: n}}
				},
				OnEvent: func(w *ldWorld, rule string, in *partition.Instance, e event.Event) {
					if e.Ts == ldAt(0) {
						e1Tables = e.Tables
					}
					w.note(rule, in, "x_in_T", event.Value{Kind: event.TBool, B: ldHasX(t, e.Tables, "x1")})
				},
				OnTimers: func(w *ldWorld, rule string, in *partition.Instance, fires []clock.Timer, tables any) {
					timerTables = append(timerTables, tables)
					w.note(rule, in, "timer_x_in_T", event.Value{Kind: event.TBool, B: ldHasX(t, tables, "x1")})
				},
			}
			w := ldRun(t, app, c.log, spec)
			if got := ldFilter(w.log, "note"); !slices.Equal(got, c.want) {
				t.Fatalf("notes:%s\nwant:%s\nall:%s", ldEntries(got), ldEntries(c.want), ldEntries(w.log))
			}
			// The carriers by identity: E1 holds its own step's snapshot; the timer of the walk at E2's line got that
			// step's Step.Tables (D5).
			byLine := map[int]playback.Step{}
			for _, s := range w.steps {
				byLine[s.Line] = s
			}
			if e1Tables != any(byLine[c.e1Line].Event.Tables) || e1Tables != any(byLine[c.e1Line].Tables) {
				t.Errorf("E1's Tables %p is not its own step's snapshot (event %v, step %p)", e1Tables, byLine[c.e1Line].Event.Tables, byLine[c.e1Line].Tables)
			}
			if len(timerTables) < 1 || timerTables[0] != any(byLine[c.walkLine].Tables) {
				t.Errorf("the timer fired in line %d's walk read %v, want that step's Tables %p (owner D5)", c.walkLine, timerTables, byLine[c.walkLine].Tables)
			}
		})
	}
}
