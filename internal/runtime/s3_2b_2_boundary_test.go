package runtime_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// bdApp is one event-time source S (k string, o object optional, ts timestamp; watermark bounded 0), a key-less rule
// that counts nothing, and a table T (k string, o object optional) keyed by k.
func bdApp(t *testing.T) runtime.App {
	t.Helper()
	sch, err := event.NewSchema("S", []event.Attr{rtA("k", event.TString), rtOpt("o", event.TObject), rtA("ts", event.TTimestamp)}, 2)
	if err != nil {
		t.Fatal(err)
	}
	tb, err := table.New("T", []event.Attr{rtA("k", event.TString), rtOpt("o", event.TObject)}, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	return runtime.App{
		Streams: []clock.StreamSpec{rtSrc("S", 0)}, Schemas: []event.Schema{sch}, Tables: table.NewSet([]*table.Table{tb}),
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyTypes: []event.Type{event.TString}, NewBody: rtBodies(rtBody{rule: "R", log: &rtLog{}})}},
	}
}

func bdEv(k string, o event.Value, ts, attrTs int64) event.Event {
	return event.Event{StreamID: 0, Ts: ts, Values: []event.Value{rtStr(k), o, {Kind: event.TTimestamp, T: attrTs}}}
}

var bdNoObj = event.Value{Kind: event.TObject, Null: true}

// TestSendRefusesWhatARecordingCannotCopy (owner D1 of S3.2b-2's design, ≠ R; review F1; MUST 14): Send refuses, with
// an error to its caller, every value a recording cannot copy exactly — and accepts the same event made exact.
func TestSendRefusesWhatARecordingCannotCopy(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 1}, bdApp(t))
	t0 := rtAt(0)
	obj := func(s string) event.Value { return event.Value{Kind: event.TObject, Obj: s} }
	for name, e := range map[string]event.Event{
		"Ts differs from the time attribute": bdEv("a", bdNoObj, t0+5000, t0),
		"malformed UTF-8 in a string":        bdEv("a\xff", bdNoObj, t0, t0),
		"an object with whitespace":          bdEv("a", obj(`{ "x": 1 }`), t0, t0),
		"an object that is not JSON":         bdEv("a", obj(`{"x":`), t0, t0),
		"an object given as bytes":           bdEv("a", event.Value{Kind: event.TObject, Obj: []byte(`{"x":1}`)}, t0, t0),
		"a timestamp after year 9999":        bdEv("a", bdNoObj, 253402300800000, 253402300800000),
		"a timestamp before year 0000":       bdEv("a", bdNoObj, -62167219200001, -62167219200001),
	} {
		if err := r.Send(ctx, 0, e); err == nil {
			t.Errorf("%s: Send accepted it; want an error to the caller (D1)", name)
		}
	}
	if err := r.Send(ctx, 0, bdEv("a", obj(`{"x":1}`), t0, t0)); err != nil {
		t.Errorf("an exact event: %v; want it accepted", err)
	}
}

// TestTableWriteRefusesWhatARecordingCannotCopy (owner D1; review F6): a table write is recorded too, so the same
// refusals hold for its rows; an object given as bytes is refused — the Go API takes an object as its JSON text.
func TestTableWriteRefusesWhatARecordingCannotCopy(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 1}, bdApp(t))
	for name, row := range map[string][]event.Value{
		"malformed UTF-8 in a key":  {rtStr("a\xff"), bdNoObj},
		"an object given as bytes":  {rtStr("a"), {Kind: event.TObject, Obj: []byte(`{"x":1}`)}},
		"an object with whitespace": {rtStr("a"), {Kind: event.TObject, Obj: `{ "x": 1 }`}},
	} {
		if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{row}); err == nil {
			t.Errorf("%s: TableWrite accepted it; want an error (D1)", name)
		}
	}
	if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{rtStr("a"), {Kind: event.TObject, Obj: `{"x":1}`}}}); err != nil {
		t.Errorf("an exact row: %v; want it written", err)
	}
}

// TestSendPayloadLimitsAndTargets (design s3-2b-2 §6; review F4, F18): a payload nested deeper than the recording
// format reads back (9,999 levels: the envelope adds one) or longer than a log line less the envelope is rejected —
// counted, not an error — whether or not the app records, so detection never depends on the recording setting; a
// derived or unknown stream is an error; after the drain SendPayload is ErrDrained.
func TestSendPayloadLimitsAndTargets(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 1}, bdApp(t))
	deep := `{"k":"a","ts":"2026-01-01T10:00:00Z","o":` + strings.Repeat("[", 9999) + strings.Repeat("]", 9999) + `}` // 10,000 deep: the decoder's limit, one over the line's
	long := `{"k":"` + strings.Repeat("x", event.MaxLogLine) + `","ts":"2026-01-01T10:00:00Z"}`
	for _, p := range []string{deep, long} {
		if err := r.SendPayload(ctx, 0, 0, []byte(p)); err != nil {
			t.Fatalf("SendPayload (%d bytes): %v; want nil (a rejection is counted)", len(p), err)
		}
	}
	if err := r.SendPayload(ctx, 0, 7, []byte(`{}`)); err == nil {
		t.Error("an unknown stream: nil; want an error")
	}
	rtDrain(t, ctx, r, 0)
	st := rtStats(t, ctx, r)
	if len(st.Streams) != 1 || len(st.Streams[0]) != 1 || st.Streams[0][0].Rejected != 2 {
		t.Errorf("Streams %+v; want S's Rejected 2 (depth, length)", st.Streams)
	}
	if err := r.SendPayload(ctx, 0, 0, []byte(`{}`)); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("SendPayload after the drain: %v; want ErrDrained", err)
	}
}
