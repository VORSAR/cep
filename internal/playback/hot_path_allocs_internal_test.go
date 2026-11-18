package playback

// The hot paths' exact allocation pins (S1.1c-6 design §2.4 and P.3's "Performance nets"; owner decision 1 of the
// plan check: "allocations equal (AllocsPerRun pins and -benchmem)"): the encoder, the recorder's table and event
// lines, the trace's steps, and the reader's Next and readLine per record kind, each pinned exactly and each
// allocation named where it comes from. TestAppendEventZeroAllocs (encode_test.go) and breakit2_test.go's
// TestBreakitGuardAppendEventAllocatesNothing stay as they are. Prefix: hp (the bench files' world).
//
// Not parallel: testing.AllocsPerRun counts every goroutine's allocations (runtime.MemStats.Mallocs), so a parallel
// test running beside a pin would add its own. Each pin runs while the package's sequential tests run, before any
// parallel one is released.

import (
	"io"
	"runtime/debug"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

const hpRuns = 200 // AllocsPerRun's runs; it calls the function once more, first, as a warm-up

// hpPin fails t unless f allocates exactly want times per call.
func hpPin(t *testing.T, what string, want float64, f func()) {
	t.Helper()
	if got := testing.AllocsPerRun(hpRuns, f); got != want {
		t.Errorf("%s: %v allocations per call; want exactly %v", what, got, want)
	}
}

// TestAppendValueAllocatesNothing: each kind, and a null, appends into a buffer with room without allocating:
// strconv's Append functions, appendString and time's AppendFormat write into dst, and an object's text is copied
// from its string.
func TestAppendValueAllocatesNothing(t *testing.T) {
	buf := make([]byte, 0, 128)
	for _, c := range hpValueCases() {
		var err error
		hpPin(t, "AppendValue "+c.name, 0, func() { buf, err = AppendValue(buf[:0], c.v) })
		if err != nil {
			t.Errorf("AppendValue %s: %v", c.name, err)
		}
	}
}

// TestAppendStringAllocatesNothing: quotes, a short escape, a \u00XX escape and a two-byte rune, written into a
// buffer with room, allocate nothing (every escape is appended byte by byte).
func TestAppendStringAllocatesNothing(t *testing.T) {
	buf := make([]byte, 0, 256)
	hpPin(t, "appendString", 0, func() { buf = appendString(buf[:0], hpEscapes) })
	if want := `"plain ascii text with \"quotes\" and \n and \u0001 and é"`; string(buf) != want {
		t.Errorf("appendString = %s; want %s", buf, want)
	}
}

// TestAppendEventIntoARoomyBufferAllocatesNothing: S(k string, n int, ts timestamp) with a batch allocates nothing:
// the name order is sorted in appendEvent's 64-slot stack array, never moved to the heap.
func TestAppendEventIntoARoomyBufferAllocatesNothing(t *testing.T) {
	s, e := hpSchema(t), hpEvent()
	buf := make([]byte, 0, 256)
	var err error
	hpPin(t, "appendEvent", 0, func() { buf, err = appendEvent(buf[:0], s, e, "") })
	if err != nil {
		t.Fatalf("appendEvent: %v", err)
	}
	if want := `{"attrs":{"k":"a","n":1,"ts":"1970-01-01T00:00:00.001Z"},"batch":"b","stream":"S"}`; string(buf) != want {
		t.Errorf("appendEvent = %s; want %s", buf, want)
	}
}

// hpRaceBuild says the test binary was built with -race, read from its build settings ("-race=true").
func hpRaceBuild(t *testing.T) bool {
	t.Helper()
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("the test binary has no build info")
	}
	for _, s := range bi.Settings {
		if s.Key == "-race" {
			return s.Value == "true"
		}
	}
	return false
}

// TestRecorderTableAllocatesNothingPerRow: a put and a delete of T(k string, v int) keyed by k write into the
// recorder's reused buffer; a delete's key columns are appended to a nil slice (recorder.go:161) whose 24-byte
// backing array the compiler places on the stack (design E4). Under -race the race detector's instrumentation keeps
// it off the stack: that append is then the delete's one allocation (measured, recorder.go:161, 24 B).
func TestRecorderTableAllocatesNothingPerRow(t *testing.T) {
	var deleteWant float64
	if hpRaceBuild(t) {
		deleteWant = 1
	}
	cases := []struct {
		name string
		op   runtime.TableOp
		row  []event.Value
		want float64
	}{
		{"put", runtime.TablePut, []event.Value{cnStr("a"), cnInt(1)}, 0},
		{"delete", runtime.TableDelete, []event.Value{cnStr("a")}, deleteWant},
	}
	for _, c := range cases {
		r := NewRecorder(io.Discard, cnTableApp())
		rows := [][]event.Value{c.row}
		var err error
		hpPin(t, "Recorder.Table "+c.name, c.want, func() { err = r.Table(0, c.op, rows) })
		if err != nil {
			t.Errorf("Recorder.Table %s: %v", c.name, err)
		}
	}
}

// TestRecorderEventAllocatesFourForATypedEventAndNoneForAPayload: a typed event's line is encoded by appendEvent into
// a nil buffer (recorder.go:80) that grows four times for its 82-byte line (backing arrays of 16, 32, 64 and 128
// bytes); a payload is copied into the recorder's reused buffer, allocating nothing.
func TestRecorderEventAllocatesFourForATypedEventAndNoneForAPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    float64
	}{
		{"typed", nil, 4},
		{"payload", []byte(hpPayload), 0},
	}
	const line = `{"arrival":"2026-01-01T10:00:00.000Z","attrs":{"k":"a","n":7,"ts":"2026-01-01T10:00:00.000Z"},` +
		`"batch":"b","stream":"S"}` + "\n"
	for _, c := range cases {
		var out hpLastLine
		r := NewRecorder(&out, hpRecordingApp(t))
		e := hpRecordedEvent()
		var err error
		hpPin(t, "Recorder.Event "+c.name, c.want, func() { err = r.Event(&e, hpT0, c.payload) })
		if err != nil {
			t.Fatalf("Recorder.Event %s: %v", c.name, err)
		}
		if string(out.last) != line {
			t.Errorf("Recorder.Event %s wrote %q; want %q", c.name, out.last, line)
		}
	}
}

// hpLastLine keeps the last line written to it, in a buffer it reuses, so writing allocates nothing once warm.
type hpLastLine struct{ last []byte }

func (w *hpLastLine) Write(p []byte) (int, error) {
	w.last = append(w.last[:0], p...)
	return len(p), nil
}

// TestTraceStepAllocatesNothing: every step kind writes into the trace's reused buffer, allocating nothing; a table
// step's "#<id>" fallback name is built on the stack (design E5). StepEvent and StepRejected write no entry.
func TestTraceStepAllocatesNothing(t *testing.T) {
	steps := []Step{
		{Kind: StepEvent, Line: 1},
		{Kind: StepHeader, Line: 1, Time: hpT0},
		{Kind: StepStreamLate, Line: 2, Event: event.Event{Seq: 3, StreamID: 0, Ts: hpT0}},
		{Kind: StepTable, Line: 3, Op: OpDelete, Table: 0},
		{Kind: StepBatchEnd, Line: 4, Batch: "b", Stream: 0},
		{Kind: StepTick, Line: 5, Time: hpT0},
		{Kind: StepStop, Line: 6},
		{Kind: StepEnd, Line: 7},
		{Kind: StepRejected, Line: 8},
	}
	tr := NewTrace(cnReaderApp(t))
	for i := range steps {
		hpPin(t, "Trace.Step kind "+string(rune('0'+steps[i].Kind)), 0, func() {
			tr.buf = tr.buf[:0]
			tr.Step(&steps[i])
		})
	}
	if tr.err != nil {
		t.Fatalf("the trace: %v", tr.err)
	}
}

// TestReaderNextAllocatesAFixedCountPerKind: Next over a line repeated forever (after the header for a recording
// kind). The envelope decodes into the reader's reused envBuf, and a one-byte string (stream "S", table "T", batch
// "b", the row key "a") comes from the runtime's static one-byte strings, costing nothing. What allocates
// (internal/event/decode.go):
//   - an object key's text (attrs, row, rejected): cleanObject's string(raw) (:209) and its boxing into Value.Obj
//     (:196), 2;
//   - that text converted back to bytes for its own decoder (reader.go:488, :394, :326), 1;
//   - the inner decode's values, appended to a nil dst (:103): one growth per value (a Value is 88 bytes), 2 for two
//     values, 1 for one;
//   - a string of two bytes or more: decodeString's string(text) (:405), 1 each — the event key "ab", the op "put" or
//     "delete", the reason "type".
//
// So: fixture event 2+1+2+1 = 6; `time arrival` event (one value) 2+1+1+1 = 5; recording event 6 (the arrival is a
// timestamp: no allocation); table put 2+1+2+1 ("put") = 6; table delete 2+1+1+1 ("delete") = 5; batch end and tick
// 0; rejected 2+1+2+1 ("type") = 6.
func TestReaderNextAllocatesAFixedCountPerKind(t *testing.T) {
	want := map[string]float64{
		"fixture_event": 6, "arrival_event": 5, "recording_event": 6, "table_put": 6, "table_delete": 5,
		"batch_end": 0, "tick": 0, "rejected": 6,
	}
	for _, c := range hpLines() {
		rd := hpReader(t, c)
		var rec Record
		var err error
		hpPin(t, "Next "+c.name, want[c.name], func() { rec, err = rd.Next() })
		if err != nil || rec.Kind != c.kind {
			t.Errorf("Next %s: kind %v, %v; want kind %v", c.name, rec.Kind, err, c.kind)
		}
	}
}

// TestReadLineAllocatesNothing: each kind's line is returned as a slice of the bufio buffer, its ending trimmed in
// place; nothing is copied for a line shorter than the buffer.
func TestReadLineAllocatesNothing(t *testing.T) {
	for _, c := range hpLines() {
		rd := hpReader(t, c)
		var line []byte
		var err error
		hpPin(t, "readLine "+c.name, 0, func() { line, err = rd.readLine() })
		if err != nil || string(line) != c.line {
			t.Errorf("readLine %s = %q, %v; want %q", c.name, line, err, c.line)
		}
	}
}
