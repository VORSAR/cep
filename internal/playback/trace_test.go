package playback_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
)

// enTraceApp: stream 0 "S" (event time), stream 1 "A" (`time arrival`), table 0 "T". The trace names streams and
// tables by name (gap E3), resolved through the App given to NewTrace.
func enTraceApp(t testing.TB) *playback.App {
	return &playback.App{
		Streams: []playback.Stream{
			{Schema: enSchema(t, "S", 0, enReq("ts", event.TTimestamp), enReq("user", event.TString))},
			{Schema: enSchema(t, "A", event.ArrivalTime, enReq("v", event.TInt))},
		},
		Tables: []playback.Table{{Name: "T", Attrs: []event.Attr{enReq("k", event.TString)}, Key: []string{"k"}}},
	}
}

func enTraceBytes(t testing.TB, tr *playback.Trace) string {
	t.Helper()
	b, err := tr.Bytes()
	if err != nil {
		t.Fatalf("Bytes: unexpected error %v", err)
	}
	return string(b)
}

// enEntry is one entry written alone and the exact line §5.5 gives it: canonical JSON, keys sorted bytewise, "at" and
// "k" on every entry, times as canonical timestamps (§5.4), one line ending in "\n" (gap E4).
type enEntry struct {
	name  string
	write func(tr *playback.Trace)
	want  string
}

var enEntries = []enEntry{
	{"recording", func(tr *playback.Trace) {
		tr.Step(&playback.Step{Kind: playback.StepHeader, Line: 1, Time: enMs20261104T10})
	}, `{"at":1,"k":"recording","start":"2026-11-04T10:00:00.000Z"}`},
	{"stream_late", func(tr *playback.Trace) {
		tr.Step(&playback.Step{Kind: playback.StepStreamLate, Line: 3, Event: event.Event{StreamID: 0,
			Ts: enMs20261104T10_1_5, Seq: 2, Values: []event.Value{enTs(enMs20261104T10_1_5), enStr("u1")}}})
	}, `{"at":3,"k":"stream_late","seq":2,"stream":"S","ts":"2026-11-04T10:00:01.500Z"}`},
	{"table put", func(tr *playback.Trace) {
		tr.Step(&playback.Step{Kind: playback.StepTable, Line: 4, Table: 0, Op: playback.OpPut})
	}, `{"at":4,"k":"table","op":"put","table":"T"}`},
	{"table delete", func(tr *playback.Trace) {
		tr.Step(&playback.Step{Kind: playback.StepTable, Line: 9, Table: 0, Op: playback.OpDelete})
	}, `{"at":9,"k":"table","op":"delete","table":"T"}`},
	{"batch_end", func(tr *playback.Trace) {
		tr.Step(&playback.Step{Kind: playback.StepBatchEnd, Line: 5, Stream: 1, Batch: "b1"})
	}, `{"at":5,"batch":"b1","k":"batch_end","stream":"A"}`},
	{"tick", func(tr *playback.Trace) {
		tr.Step(&playback.Step{Kind: playback.StepTick, Line: 6, Time: enMs20261104T1001})
	}, `{"at":6,"k":"tick","time":"2026-11-04T10:01:00.000Z"}`},
	{"stop", func(tr *playback.Trace) {
		tr.Step(&playback.Step{Kind: playback.StepStop, Line: 7})
	}, `{"at":7,"k":"stop"}`},
	{"end", func(tr *playback.Trace) { // at: the last line + 1, which is StepEnd's Line
		tr.Step(&playback.Step{Kind: playback.StepEnd, Line: 8})
	}, `{"at":8,"k":"end"}`},
	{"event", func(tr *playback.Trace) {
		tr.Released(3, "r1", enStr("u1"), event.Event{StreamID: 0, Ts: enMs20261104T10, Seq: 1})
	}, `{"at":3,"k":"event","key":"u1","rule":"r1","seq":1,"stream":"S","ts":"2026-11-04T10:00:00.000Z"}`},
	{"construct_late", func(tr *playback.Trace) {
		tr.ConstructLate(4, "r1", enInt(42), event.Event{StreamID: 1, Ts: enMs20261104T10_1_5, Seq: 3})
	}, `{"at":4,"k":"construct_late","key":42,"rule":"r1","seq":3,"stream":"A","ts":"2026-11-04T10:00:01.500Z"}`},
	{"timer", func(tr *playback.Trace) {
		tr.Fired(5, "r2", enStr("u1"), clock.Timer{Due: enMs20261104T1001, Kind: clock.Completion, Ref: 7})
	}, `{"at":5,"due":"2026-11-04T10:01:00.000Z","k":"timer","key":"u1","kind":"completion","ref":7,"rule":"r2"}`},
	{"discard", func(tr *playback.Trace) {
		tr.Discarded(6, "r2", enStr("u1"), math.MaxUint64)
	}, `{"at":6,"k":"discard","key":"u1","ref":18446744073709551615,"rule":"r2"}`},
	{"purge", func(tr *playback.Trace) { // a null key is a canonical null
		tr.Purged(8, "r2", enNull(event.TString), enMs20261104T1001)
	}, `{"at":8,"due":"2026-11-04T10:01:00.000Z","k":"purge","key":null,"rule":"r2"}`},
	{"note", func(tr *playback.Trace) {
		tr.Note(5, "r3", enStr("u\"1"), "count", enFloat(0.75))
	}, `{"at":5,"k":"note","key":"u\"1","name":"count","rule":"r3","value":0.75}`},
	{"note, escaped rule and name, timestamp value", func(tr *playback.Trace) {
		tr.Note(2, "r\"x", enInt(-1), "w\u2028", enTs(enMsYear0Start))
	}, `{"at":2,"k":"note","key":-1,"name":"w\u2028","rule":"r\"x","value":"0000-01-01T00:00:00.000Z"}`},
}

// TestTraceEntries writes each entry kind once, alone, and compares the exact line.
func TestTraceEntries(t *testing.T) {
	for _, c := range enEntries {
		t.Run(c.name, func(t *testing.T) {
			tr := playback.NewTrace(enTraceApp(t))
			c.write(tr)
			if got := enTraceBytes(t, tr); got != c.want+"\n" {
				t.Fatalf("got  %q\nwant %q", got, c.want+"\n")
			}
		})
	}
}

// TestTraceTimerKinds: a timer kind is its lowercase name (§5.5). For the multi-word kinds this reads "lowercase
// name" as the Go name lowercased, with no separator (gap E1).
func TestTraceTimerKinds(t *testing.T) {
	kinds := []struct {
		k    clock.TimerKind
		name string
	}{
		{clock.Completion, "completion"}, {clock.CronClose, "cronclose"}, {clock.DelayDue, "delaydue"},
		{clock.BatchClose, "batchclose"}, {clock.TTLPurge, "ttlpurge"}, {clock.Discard, "discard"},
	}
	for _, c := range kinds {
		tr := playback.NewTrace(enTraceApp(t))
		tr.Fired(2, "r", enInt(1), clock.Timer{Due: 0, Kind: c.k, Ref: 1})
		want := `{"at":2,"due":"1970-01-01T00:00:00.000Z","k":"timer","key":1,"kind":"` + c.name + `","ref":1,"rule":"r"}` + "\n"
		if got := enTraceBytes(t, tr); got != want {
			t.Errorf("kind %d: got %q; want %q", c.k, got, want)
		}
	}
}

// TestTraceStepEventWritesNothing: Trace.Step records the driver's own entries only; an admitted event's line is
// the rules' (Released), so a StepEvent adds no entry (gap E5).
func TestTraceStepEventWritesNothing(t *testing.T) {
	tr := playback.NewTrace(enTraceApp(t))
	tr.Step(&playback.Step{Kind: playback.StepEvent, Line: 2, Event: event.Event{StreamID: 0, Ts: enMs20261104T10, Seq: 1}})
	if got := enTraceBytes(t, tr); got != "" {
		t.Fatalf("StepEvent wrote %q; want nothing", got)
	}
}

// TestTraceOrder: entries come out in the order recorded, the driver's and the rules' interleaved; Bytes is the
// trace so far, so a mid-way Bytes is a prefix of the final one. An empty trace is empty.
func TestTraceOrder(t *testing.T) {
	tr := playback.NewTrace(enTraceApp(t))
	if got := enTraceBytes(t, tr); got != "" {
		t.Fatalf("empty trace: %q; want empty", got)
	}
	// The driver's and the rules' entries interleaved, in an order sorted neither by at nor by k: the trace keeps the
	// recorded order.
	order := []int{0, 8, 1, 9, 2, 10, 13, 11, 4, 12, 5, 6, 7}
	var want strings.Builder
	var mid string
	for i, idx := range order {
		enEntries[idx].write(tr)
		want.WriteString(enEntries[idx].want + "\n")
		if i == 5 {
			mid = enTraceBytes(t, tr)
			if mid != want.String() {
				t.Fatalf("mid-way Bytes:\n got  %q\n want %q", mid, want.String())
			}
		}
	}
	got := enTraceBytes(t, tr)
	if got != want.String() {
		t.Fatalf("trace:\n got  %q\n want %q", got, want.String())
	}
	if !strings.HasPrefix(got, mid) {
		t.Fatalf("final trace does not extend the mid-way one")
	}
}

// TestTraceBytesFirstError: Bytes returns the first encoding error. A NaN note value is unencodable, and so is a
// timestamp past 9999. Trace a holds the NaN alone; b the NaN then the bad time; c the bad time alone. b's error is
// a's (the first), and differs from c's.
func TestTraceBytesFirstError(t *testing.T) {
	nan := func(tr *playback.Trace) { tr.Note(2, "r", enInt(1), "nan", enFloat(math.NaN())) }
	late := func(tr *playback.Trace) { tr.Note(3, "r", enInt(1), "far", enTs(enMsYear9999End+1)) }
	ok := func(tr *playback.Trace) { tr.Note(4, "r", enInt(1), "fine", enInt(5)) }
	run := func(fs ...func(*playback.Trace)) error {
		tr := playback.NewTrace(enTraceApp(t))
		for _, f := range fs {
			f(tr)
		}
		_, err := tr.Bytes()
		return err
	}
	errA, errB, errC := run(ok, nan), run(ok, nan, late, ok), run(ok, late)
	if errA == nil || errB == nil || errC == nil {
		t.Fatalf("errors %v, %v, %v; want three", errA, errB, errC)
	}
	if errB.Error() != errA.Error() {
		t.Errorf("NaN then bad time: %q; want the first error, %q", errB, errA)
	}
	if errC.Error() == errA.Error() {
		t.Errorf("the NaN and the bad time give one message %q: the test cannot tell which came first", errA)
	}
	if err := run(func(tr *playback.Trace) { tr.Purged(2, "r", enInt(1), math.MaxInt64) }); err == nil {
		t.Errorf("purge due at MaxInt64: no error; want one (an unencodable time)")
	}
}

// TestTraceLateCounters: the late counters are the stream_late and construct_late entries' counts (§5.5).
func TestTraceLateCounters(t *testing.T) {
	tr := playback.NewTrace(enTraceApp(t))
	for _, idx := range []int{0, 1, 9, 8, 1, 9, 10, 9, 7} { // stream_late x2, construct_late x3
		enEntries[idx].write(tr)
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSuffix(enTraceBytes(t, tr), "\n"), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %q is not JSON: %v", line, err)
		}
		k, _ := m["k"].(string)
		counts[k]++
	}
	if counts["stream_late"] != 2 || counts["construct_late"] != 3 {
		t.Fatalf("counts %v; want stream_late 2, construct_late 3", counts)
	}
}
