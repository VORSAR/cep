package playback

import (
	"io"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

// hpT0 is 2026-01-01T10:00:00.000Z in Unix milliseconds: every fixed timestamp of the recording-path nets.
const hpT0 int64 = 1767261600000

// hpValueCase is one of AppendValue's fixed inputs.
type hpValueCase struct {
	name string
	v    event.Value
}

// hpValueCases are AppendValue's fixed inputs, one per kind and a null (design s1-1c-6 P.3's table): the string holds
// quotes, U+2028 and a malformed byte, so each of appendString's slow branches runs.
func hpValueCases() []hpValueCase {
	return []hpValueCase{
		{"string", event.Value{Kind: event.TString, Str: "say \"hi\" \xff"}},
		{"int", event.Value{Kind: event.TInt, I: -1234567}},
		{"float", event.Value{Kind: event.TFloat, F: 3.25}},
		{"bool", event.Value{Kind: event.TBool, B: true}},
		{"timestamp", event.Value{Kind: event.TTimestamp, T: hpT0}},
		{"duration", event.Value{Kind: event.TDuration, D: 90000}},
		{"object", event.Value{Kind: event.TObject, Obj: `{"a":1}`}},
		{"null", event.Value{Kind: event.TInt, Null: true}},
	}
}

// hpEscapes is appendString's mixed input: plain ASCII, quotes, a short escape, a \u00XX escape and a two-byte rune.
const hpEscapes = "plain ascii text with \"quotes\" and \n and \x01 and é"

// hpPlain is appendString's fast path: 64 bytes of ASCII with nothing to escape.
const hpPlain = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"

// hpSchema is S(k string, n int, ts timestamp), timed by ts.
func hpSchema(t testing.TB) event.Schema {
	t.Helper()
	return cnSchema(t, "S", 2, event.Attr{Name: "k", Type: event.TString}, event.Attr{Name: "n", Type: event.TInt},
		event.Attr{Name: "ts", Type: event.TTimestamp})
}

// hpEvent is S's event {k: "a", n: 1, ts: 1} in batch "b".
func hpEvent() event.Event {
	return event.Event{Ts: 1, BatchID: "b", Values: []event.Value{cnStr("a"), cnInt(1),
		{Kind: event.TTimestamp, T: 1}}}
}

// hpRecordingApp is a plan of the stream S (hpSchema) and the table T(k string, v int) keyed by k.
func hpRecordingApp(t testing.TB) *plan.App {
	t.Helper()
	app := cnTableApp()
	app.Streams = []plan.Stream{{Schema: hpSchema(t)}}
	return app
}

// hpRecordedEvent is the event the recording-path nets write: S's {k: "a", n: 7, ts: hpT0} in batch "b".
func hpRecordedEvent() event.Event {
	return event.Event{Ts: hpT0, BatchID: "b", Values: []event.Value{cnStr("a"), cnInt(7),
		{Kind: event.TTimestamp, T: hpT0}}}
}

// hpPayload is hpRecordedEvent's attrs as SendPayload would hand them to the recorder.
const hpPayload = `{"k":"a","n":7,"ts":"2026-01-01T10:00:00.000Z"}`

func BenchmarkAppendValue(b *testing.B) {
	for _, c := range hpValueCases() {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			buf := make([]byte, 0, 128)
			if _, err := AppendValue(buf[:0], c.v); err != nil {
				b.Fatalf("AppendValue(%s): %v", c.name, err)
			}
			for b.Loop() {
				buf, _ = AppendValue(buf[:0], c.v)
			}
		})
	}
}

func BenchmarkAppendStringPlain(b *testing.B) {
	b.ReportAllocs()
	buf := make([]byte, 0, 256)
	for b.Loop() {
		buf = appendString(buf[:0], hpPlain)
	}
}

func BenchmarkAppendStringEscapes(b *testing.B) {
	b.ReportAllocs()
	buf := make([]byte, 0, 256)
	for b.Loop() {
		buf = appendString(buf[:0], hpEscapes)
	}
}

func BenchmarkAppendEvent(b *testing.B) {
	b.ReportAllocs()
	s, e := hpSchema(b), hpEvent()
	buf := make([]byte, 0, 256)
	if _, err := appendEvent(buf[:0], s, e, ""); err != nil {
		b.Fatalf("appendEvent: %v", err)
	}
	for b.Loop() {
		buf, _ = appendEvent(buf[:0], s, e, "")
	}
}

func BenchmarkRecorderTable(b *testing.B) {
	cases := []struct {
		name string
		op   runtime.TableOp
		row  []event.Value
	}{
		{"put", runtime.TablePut, []event.Value{cnStr("a"), cnInt(1)}},
		{"delete", runtime.TableDelete, []event.Value{cnStr("a")}},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			r := NewRecorder(io.Discard, cnTableApp())
			rows := [][]event.Value{c.row}
			if err := r.Table(0, c.op, rows); err != nil {
				b.Fatalf("Table(%s): %v", c.name, err)
			}
			for b.Loop() {
				_ = r.Table(0, c.op, rows)
			}
		})
	}
}

func BenchmarkRecordOneEvent(b *testing.B) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"typed", nil},
		{"payload", []byte(hpPayload)},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			r := NewRecorder(io.Discard, hpRecordingApp(b))
			e := hpRecordedEvent()
			if err := r.Event(&e, hpT0, c.payload); err != nil {
				b.Fatalf("Event(%s): %v", c.name, err)
			}
			for b.Loop() {
				_ = r.Event(&e, hpT0, c.payload)
			}
		})
	}
}

func BenchmarkTraceStepTable(b *testing.B) {
	b.ReportAllocs()
	tr := NewTrace(cnReaderApp(b))
	s := &Step{Kind: StepTable, Line: 7, Op: OpDelete, Table: 0}
	for b.Loop() {
		tr.buf = tr.buf[:0]
		tr.Step(s)
	}
}
