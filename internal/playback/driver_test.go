package playback_test

// The virtual-clock driver (design note v2 §5.3) and the recording records (owner D3; design §5.2, §4.3).

import (
	"fmt"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/table"
)

func ldStepAt(t *testing.T, w *ldWorld, line int) playback.Step {
	t.Helper()
	for _, s := range w.steps {
		if s.Line == line {
			return s
		}
	}
	t.Fatalf("no step at line %d", line)
	return playback.Step{}
}

func ldAdv(sid int, c int64) clock.WMAdvance {
	return clock.WMAdvance{StreamID: sid, Contribution: c, Moved: true}
}

// §5.3: Seq per stream from 1 in file order, a stream-late event included; Tables on every event; the advances in
// the order ObserveEvent → the virtual floor; a stream-late event moves nothing; the end raises everything to +∞.
//
// App: A (d 1 min), B (d 0). Seconds after 10:00:
//
//	L1 A@0    A seq 1. ObserveEvent: A's admission 0 − 60 = −60, moved. vnow 0: A's virtual floor −60 (no move),
//	          B's 0 − 0 = 0 (moved) → Advances [A −60, B 0], in that order (own advance first, design §5.3 step 5).
//	L2 B@-30  B seq 1. Admission −30 (B had none: not late); B's contribution max(−30, 0) = 0: no move; vnow stays 0
//	          → Advances []. A StepEvent (not stream-late).
//	L3 A@-61  A seq 2. A's admission −60 > −61: STREAM-LATE; Seq and Tables still set (steps 1, 2 come first).
//	L4 put T{p}
//	L5 A@30   A seq 3 (the late one counted). Admission −30, moved; vnow 30: A's floor −30 (no move), B's 30 (moved).
//	L6 B@40   B seq 2. Admission 40, moved; vnow 40: A's floor −20 (moved) → [B 40, A −20].
//	End (line 7): EndOfInput → [A +∞, B +∞], every contribution +∞.
func TestDriverSeqTablesAndAdvances(t *testing.T) {
	t.Parallel()
	app := &playback.App{Streams: []playback.Stream{ldStd(t, "A", ldMin, 0), ldStd(t, "B", 0, 0)}, Tables: []playback.Table{ldTableT()}}
	log := ldLog(
		ldE("A", ldAt(0), "a", "", ""),
		ldE("B", ldAt(-30), "a", "", ""),
		ldE("A", ldAt(-61), "a", "", ""),
		`{"table":"T","op":"put","row":{"x":"p"}}`,
		ldE("A", ldAt(30), "a", "", ""),
		ldE("B", ldAt(40), "a", "", ""),
	)
	w := ldRun(t, app, log)
	type want struct {
		kind playback.StepKind
		sid  int
		seq  uint64
		adv  []clock.WMAdvance
	}
	wants := []want{
		{playback.StepEvent, 0, 1, []clock.WMAdvance{ldAdv(0, ldAt(-60)), ldAdv(1, ldAt(0))}},
		{playback.StepEvent, 1, 1, nil},
		{playback.StepStreamLate, 0, 2, nil},
		{playback.StepTable, 0, 0, nil},
		{playback.StepEvent, 0, 3, []clock.WMAdvance{ldAdv(0, ldAt(-30)), ldAdv(1, ldAt(30))}},
		{playback.StepEvent, 1, 2, []clock.WMAdvance{ldAdv(1, ldAt(40)), ldAdv(0, ldAt(-20))}},
		{playback.StepEnd, 0, 0, []clock.WMAdvance{ldAdv(0, ldPosInf), ldAdv(1, ldPosInf)}},
	}
	if len(w.steps) != len(wants) {
		t.Fatalf("%d steps, want %d", len(w.steps), len(wants))
	}
	var prevTables *table.Snapshot
	for i, s := range w.steps {
		wt := wants[i]
		line := i + 1
		if s.Kind != wt.kind || s.Line != line {
			t.Errorf("step %d: kind %d line %d, want kind %d line %d", i, s.Kind, s.Line, wt.kind, line)
		}
		if len(s.Advances) != 0 || len(wt.adv) != 0 {
			if !slices.Equal(s.Advances, wt.adv) {
				t.Errorf("line %d: Advances %+v, want %+v", line, s.Advances, wt.adv)
			}
		}
		if s.Tables == nil {
			t.Errorf("line %d: Step.Tables nil: every step carries the snapshot current after it (design §5.3)", line)
		}
		if s.Kind == playback.StepEvent || s.Kind == playback.StepStreamLate {
			if s.Event.StreamID != wt.sid || s.Event.Seq != wt.seq {
				t.Errorf("line %d: stream %d seq %d, want stream %d seq %d (design §5.3 step 1)", line, s.Event.StreamID, s.Event.Seq, wt.sid, wt.seq)
			}
			snap, ok := s.Event.Tables.(*table.Snapshot)
			if !ok || snap == nil || snap != s.Tables {
				t.Errorf("line %d: Event.Tables %v, want the step's *table.Snapshot %p (design §2: set on every event)", line, s.Event.Tables, s.Tables)
			}
		}
		if s.Kind == playback.StepTable {
			if s.Tables == prevTables || !s.Tables.Table(0).Has(ldStr("p")) {
				t.Errorf("line %d: a put's step must carry the new snapshot, holding p", line)
			}
		}
		prevTables = s.Tables
	}
	if end := w.steps[len(w.steps)-1]; end.Line != 7 {
		t.Errorf("End's line %d, want the last line + 1 = 7", end.Line)
	}
	for sid := range app.Streams {
		if c := w.d.Registry().Contribution(sid); c != ldPosInf {
			t.Errorf("after a fixture's end, stream %d's contribution %d, want +∞ (design §5.2)", sid, c)
		}
	}
	// Driver.Walk: component "seq", key the stream name, in declaration order; the counters A 3, B 2.
	var got []string
	w.d.Walk(func(component, key string, state any) {
		got = append(got, fmt.Sprintf("%s/%s=%v", component, key, state))
	})
	if want := []string{"seq/A=3", "seq/B=2"}; !slices.Equal(got, want) {
		t.Errorf("Driver.Walk yielded %v, want %v (design §5.3, review F4)", got, want)
	}
}

// §5.3 step 4: a stream-late event moves nothing — the registry is exactly as before it, bar the stream's last
// arrival (clock.StreamRegistry.ObserveEvent records it even for a late event, as the oracle does).
func TestDriverStreamLateMovesNothing(t *testing.T) {
	t.Parallel()
	app := &playback.App{Streams: []playback.Stream{ldStd(t, "A", 10*ldSec, 0), ldStd(t, "B", 0, 0)}}
	d, err := playback.NewDriver(app)
	if err != nil {
		t.Fatal(err)
	}
	recs, rerr := ldRecords(t, app, ldLog(ldE("A", ldAt(60), "a", "", ""), ldE("A", ldAt(49), "a", "", "")))
	if !ldEOF(rerr) || len(recs) != 2 {
		t.Fatalf("records %d, err %v", len(recs), rerr)
	}
	d.Step(recs[0])
	before := map[string]clock.StreamState{}
	d.Registry().Walk(func(_, key string, s any) { before[key] = s.(clock.StreamState) })
	// A's admission 60 − 10 = 50 > 49: stream-late.
	s := d.Step(recs[1])
	if s.Kind != playback.StepStreamLate || len(s.Advances) != 0 || s.Event.Seq != 2 {
		t.Fatalf("step %+v, want StepStreamLate, no Advances, Seq 2", s)
	}
	d.Registry().Walk(func(_, key string, st any) {
		got, want := st.(clock.StreamState), before[key]
		want.LastArrival = got.LastArrival
		if got != want {
			t.Errorf("stream %s moved on a stream-late event: %+v, was %+v", key, got, want)
		}
	})
}

// RT-01 (SEMANTICS §2.1; design §5.3 step 5): an empty stream never stalls a rule reading it — the virtual floor
// raises its contribution. A (d 0) carries events, B (d 0) none; the rule reads both.
//
//	L1 A@0  A 0; vnow 0 → B's floor 0 → the clock min(0, 0) = 0: E1 (0) not released (D1: 0 < 0 is false).
//	L2 A@60 A 60, B 60 → the clock 60 > 0: E1 released in line 2's walk — before the end of input.
//
// Without the virtual floor B stays −∞, the clock never moves, and E1 waits for the end (line 3).
func TestDriverEmptyStreamNeverStalls(t *testing.T) {
	t.Parallel()
	app := &playback.App{Streams: []playback.Stream{ldStd(t, "A", 0, 0), ldStd(t, "B", 0, 0)}}
	w := ldRun(t, app, ldLog(ldE("A", ldAt(0), "a", "", ""), ldE("A", ldAt(60), "a", "", "")),
		ldRuleSpec{Name: "R", Inputs: []int{0, 1}, Key: "k"})
	want := []ldEntry{
		{At: 2, K: "event", Rule: "R", Key: "a", Stream: 0, Seq: 1, Ts: ldAt(0)},
		{At: 3, K: "event", Rule: "R", Key: "a", Stream: 0, Seq: 2, Ts: ldAt(60)},
	}
	if got := ldFilter(w.log, "event"); !slices.Equal(got, want) {
		t.Fatalf("releases:%s\nwant:%s", ldEntries(got), ldEntries(want))
	}
}

// A hand-written recording gives the live clock's arithmetic (owner D3; design §5.2, §5.3): the case of
// internal/clock's TestQuiet_NeverSeenStreamStartsFromStart_IdleAfterZeroNeverQuiet, replayed from a log.
// S0 (d 1 min, idle after 30 s), S1 (d 1 min, no idleness); start = ldT0 (seconds below are after it).
//
//	L1 header start          → StepHeader, Time = start; recording mode, no Advances.
//	L2 tick +10   10 < 30: S0 not quiet → no Advances.
//	L3 tick +30   30 ≥ 30: S0's floor = start + 30 − 60 = −30 → [S0 −30].
//	L4 tick +100  floor = start + 100 − 60 = +40 → [S0 +40]. S1 (IdleAfter 0) never moves.
//	L5 S0 event ts −500, arrival +100: admission −560 < the floor +40 → no advance; and no virtual floor in a
//	   recording → Advances [].
//	L6 tick +110  110 − 100 = 10 < 30: nothing.
//	L7 tick +130  30 ≥ 30: floor = max(+40, −500 + 30 − 60) = +40: no move.
//	End (line 8): a recording without stop adds nothing — no Advances, contributions unchanged (S0 +40, S1 −∞).
func TestRecordingTicksGiveTheLiveArithmetic(t *testing.T) {
	t.Parallel()
	app := &playback.App{Streams: []playback.Stream{ldStd(t, "S0", ldMin, 30*ldSec), ldStd(t, "S1", ldMin, 0)}}
	tick := func(s float64) string { return fmt.Sprintf(`{"tick":%q}`, ldRFC(ldAt(s))) }
	log := ldLog(
		fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0)),
		tick(10), tick(30), tick(100),
		ldE("S0", ldAt(-500), "a", "", fmt.Sprintf(`,"arrival":%q`, ldRFC(ldAt(100)))),
		tick(110), tick(130),
	)
	w := ldRun(t, app, log, ldRuleSpec{Name: "R", Inputs: []int{0, 1}, Key: "k", TTL: ldMin})
	wants := []struct {
		kind playback.StepKind
		time int64
		adv  []clock.WMAdvance
	}{
		{playback.StepHeader, ldT0, nil},
		{playback.StepTick, ldAt(10), nil},
		{playback.StepTick, ldAt(30), []clock.WMAdvance{ldAdv(0, ldAt(-30))}},
		{playback.StepTick, ldAt(100), []clock.WMAdvance{ldAdv(0, ldAt(40))}},
		{playback.StepEvent, 0, nil},
		{playback.StepTick, ldAt(110), nil},
		{playback.StepTick, ldAt(130), nil},
		{playback.StepEnd, 0, nil},
	}
	if len(w.steps) != len(wants) {
		t.Fatalf("%d steps, want %d", len(w.steps), len(wants))
	}
	for i, s := range w.steps {
		wt := wants[i]
		if s.Kind != wt.kind || s.Line != i+1 {
			t.Errorf("step %d: kind %d line %d, want kind %d line %d", i, s.Kind, s.Line, wt.kind, i+1)
		}
		if (s.Kind == playback.StepHeader || s.Kind == playback.StepTick) && s.Time != wt.time {
			t.Errorf("line %d: Time %d, want %d", s.Line, s.Time, wt.time)
		}
		if (len(s.Advances) != 0 || len(wt.adv) != 0) && !slices.Equal(s.Advances, wt.adv) {
			t.Errorf("line %d: Advances %+v, want %+v", s.Line, s.Advances, wt.adv)
		}
	}
	if got := w.steps[4].Event.Seq; got != 1 {
		t.Errorf("the event's Seq %d, want 1", got)
	}
	if c0, c1 := w.d.Registry().Contribution(0), w.d.Registry().Contribution(1); c0 != ldAt(40) || c1 != ldNegInf {
		t.Errorf("after a recording's end without stop: contributions (%d, %d), want (+40 s, −∞): its end adds nothing", c0, c1)
	}
	if got := ldFilter(w.log, "event"); len(got) != 0 {
		t.Errorf("a release with S1 at −∞: %v", got)
	}
}

// A stop record drains to one past the largest admitted event time (design §4.3, §5.2; ADR-0012 §3 as annotated):
// a deadline at that time fires, a later one does not. And a recording without stop adds nothing at its end.
//
// S0 (d 1 min, no idleness). L1 header start 10:00; L2 a@10:00 (arrival 10:00); L3 b@10:01 (arrival 10:01).
// After L3 the clock is 10:01 − 60 s = 10:00: nothing released (D1: 10:00 < 10:00 is false). The body schedules, per
// released event e, Completion ref 1 at e.Ts and ref 2 at e.Ts + 1 ms; TTL 1 h (no purge before +∞).
//
//	L4 stop: Drain → max admitted 10:01 → the clock 10:01:00.001. The walk: a@10:00 released, its timers at 10:00
//	   and 10:00:00.001 fire; b@10:01 released, its ref 1 at 10:01 fires (10:01 < 10:01:00.001); its ref 2 at
//	   10:01:00.001 does not (not below the clock). End (line 5): a recording — nothing more.
//
// Without the stop record: nothing is released at all, and End has no Advances.
func TestRecordingStopDrainsPastTheMaxEventTime(t *testing.T) {
	t.Parallel()
	app := &playback.App{Streams: []playback.Stream{ldStd(t, "S0", ldMin, 0)}}
	arr := func(s float64) string { return fmt.Sprintf(`,"arrival":%q`, ldRFC(ldAt(s))) }
	head := []string{
		fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0)),
		ldE("S0", ldAt(0), "a", "", arr(0)),
		ldE("S0", ldAt(60), "b", "", arr(60)),
	}
	spec := ldRuleSpec{Name: "R", Inputs: []int{0}, Key: "k", TTL: 60 * ldMin, Sched: func(e event.Event, n uint64) []clock.Timer {
		return []clock.Timer{{Due: e.Ts, Kind: clock.Completion, Ref: 1}, {Due: e.Ts + 1, Kind: clock.Completion, Ref: 2}}
	}}
	t.Run("stop", func(t *testing.T) {
		t.Parallel()
		w := ldRun(t, app, ldLog(append(head, `{"stop":true}`)...), spec)
		want := []ldEntry{
			{At: 4, K: "event", Rule: "R", Key: "a", Stream: 0, Seq: 1, Ts: ldAt(0)},
			{At: 4, K: "timer", Rule: "R", Key: "a", Kind: clock.Completion, Due: ldAt(0), Ref: 1},
			{At: 4, K: "timer", Rule: "R", Key: "a", Kind: clock.Completion, Due: ldAt(0) + 1, Ref: 2},
			{At: 4, K: "event", Rule: "R", Key: "b", Stream: 0, Seq: 2, Ts: ldAt(60)},
			{At: 4, K: "timer", Rule: "R", Key: "b", Kind: clock.Completion, Due: ldAt(60), Ref: 1},
		}
		if !slices.Equal(w.log, want) {
			t.Fatalf("log:%s\nwant:%s", ldEntries(w.log), ldEntries(want))
		}
		stop := ldStepAt(t, w, 4)
		if stop.Kind != playback.StepStop || !slices.Equal(stop.Advances, []clock.WMAdvance{ldAdv(0, ldAt(60)+1)}) {
			t.Errorf("stop step %+v, want StepStop with [S0 10:01:00.001]", stop)
		}
		if end := ldStepAt(t, w, 5); end.Kind != playback.StepEnd || len(end.Advances) != 0 {
			t.Errorf("end step %+v, want StepEnd with no Advances (a recording)", end)
		}
	})
	t.Run("no stop", func(t *testing.T) {
		t.Parallel()
		w := ldRun(t, app, ldLog(head...), spec)
		if len(w.log) != 0 {
			t.Fatalf("a recording without stop released or fired:%s", ldEntries(w.log))
		}
		if end := ldStepAt(t, w, 4); end.Kind != playback.StepEnd || len(end.Advances) != 0 {
			t.Errorf("end step %+v, want StepEnd with no Advances", end)
		}
		if c := w.d.Registry().Contribution(0); c != ldAt(0) {
			t.Errorf("contribution %d after the end, want 10:00 (unchanged)", c)
		}
	})
}

// A `time arrival` stream's ts is its arrival (design §5.2): its recorded line carries "ts" and no "arrival", and
// the record's Arrival equals its Event.Ts; the registry's last arrival is that time.
func TestRecordingArrivalStreamTsIsItsArrival(t *testing.T) {
	t.Parallel()
	app := &playback.App{Streams: []playback.Stream{ldArrival(t, "R", 0, 0)}}
	log := ldLog(fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0)), ldEA("R", ldAt(5), "a", "", ""))
	recs, err := ldRecords(t, app, log)
	if !ldEOF(err) || len(recs) != 2 {
		t.Fatalf("records %+v, err %v", recs, err)
	}
	if r := recs[1]; r.Kind != playback.RecEvent || r.Event.Ts != ldAt(5) || r.Arrival != ldAt(5) {
		t.Fatalf("record %+v, want an event with Ts = Arrival = 10:00:05", r)
	}
	w := ldRun(t, app, log)
	w.d.Registry().Walk(func(_, key string, s any) {
		if st := s.(clock.StreamState); st.LastArrival != ldAt(5) || st.MaxTs != ldAt(5) {
			t.Errorf("stream %s: %+v, want LastArrival = MaxTs = 10:00:05", key, st)
		}
	})
}
