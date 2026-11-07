package cep_test

// The value boundary (design s3-3-1 §4; QA's T21, T23, T24, T26 and §9's additions).

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// TestSendRowChecks (T21; MUST 15): one sub-test per check; each returns an error naming the attribute (or the index),
// nothing reaches the sink and the stream's stats stay unchanged.
func TestSendRowChecks(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	good := fcRow("ok", 1, fcAt(1))
	rows := []struct {
		name string
		fill func(cepio.RowWriter) error
		want string // in the error's text
	}{
		{"index out of range", func(w cepio.RowWriter) error { good(w); w.SetString(99, "x"); return nil }, "99"},
		{"wrong type", func(w cepio.RowWriter) error { good(w); w.SetInt(w.AttrIndex("s"), 1); return nil }, "s"},
		{"NaN", func(w cepio.RowWriter) error { good(w); w.SetFloat(w.AttrIndex("f"), math.NaN()); return nil }, "f"},
		{"+Inf", func(w cepio.RowWriter) error { good(w); w.SetFloat(w.AttrIndex("f"), math.Inf(1)); return nil }, "f"},
		{"malformed UTF-8", func(w cepio.RowWriter) error { good(w); w.SetString(w.AttrIndex("s"), "\xff"); return nil }, "s"},
		{"object not compact", func(w cepio.RowWriter) error { good(w); w.SetObject(w.AttrIndex("o"), "{ \"a\": 1 }"); return nil }, "o"},
		{"object not JSON", func(w cepio.RowWriter) error { good(w); w.SetObject(w.AttrIndex("o"), "[1,"); return nil }, "o"},
		{"timestamp out of range", func(w cepio.RowWriter) error { good(w); w.SetTimestamp(w.AttrIndex("t"), 253402300800001); return nil }, "t"},
		{"s unset", func(w cepio.RowWriter) error {
			w.SetInt(w.AttrIndex("i"), 1)
			w.SetTimestamp(w.AttrIndex("t"), fcAt(1))
			return nil
		}, "s"},
		{"t unset", func(w cepio.RowWriter) error {
			w.SetString(w.AttrIndex("s"), "x")
			w.SetInt(w.AttrIndex("i"), 1)
			return nil
		}, "t"},
		{"SetNull on a non-optional attribute", func(w cepio.RowWriter) error { good(w); w.SetNull(w.AttrIndex("s")); return nil }, "s"},
		{"fill's error", func(w cepio.RowWriter) error { good(w); return errors.New("the caller's own error") }, "the caller's own error"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			before := fcStats(t, ctx, h).Streams["S"]
			err := in.SendRow(ctx, "", row.fill)
			if err == nil || !strings.Contains(err.Error(), row.want) {
				t.Errorf("SendRow: %v; want an error naming %q", err, row.want)
			}
			if n := len(sink.Drain()); n != 0 {
				t.Errorf("%d outputs; want none", n)
			}
			if after := fcStats(t, ctx, h).Streams["S"]; after != before {
				t.Errorf("StreamStats changed %+v → %+v; want unchanged", before, after)
			}
		})
	}
	fcSend(t, ctx, in, good)
	fcStop(t, ctx, h)
	if n := len(sink.Drain()); n != 1 {
		t.Errorf("%d outputs from the good row; want 1", n)
	}
}

// TestRowWriterKeptPastFillPanics (§9, MUST 14): a Set on a writer kept past its fill panics with ErrRowUsedAfterFill.
func TestRowWriterKeptPastFillPanics(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	_, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	var kept cepio.RowWriter
	fcSend(t, ctx, fcInput(t, h, "S"), func(w cepio.RowWriter) error { kept = w; return fcRow("a", 1, fcAt(1))(w) })
	defer func() {
		p := recover()
		err, _ := p.(error)
		if !errors.Is(err, cep.ErrRowUsedAfterFill) {
			t.Errorf("Set after fill: panic %v; want ErrRowUsedAfterFill", p)
		}
	}()
	kept.SetString(0, "late")
}

// TestSendRowValuesAreTheEnginesOnceSent (T23; MUST 14): 10,000 rows in order, each its own values; the sink sees
// exactly 0…9999 in order (a reused slice would corrupt them, and -race would see it).
func TestSendRowValuesAreTheEnginesOnceSent(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	const n = 10000
	for i := 0; i < n; i++ {
		fcSend(t, ctx, in, fcRow(fmt.Sprint(i), int64(i), fcAt(float64(i))))
	}
	fcStop(t, ctx, h)
	recs := sink.Drain()
	if len(recs) != n {
		t.Fatalf("%d outputs; want %d", len(recs), n)
	}
	for i, rec := range recs {
		if rec.Values[0] != fmt.Sprint(i) {
			t.Fatalf("output %d is %v; want %d", i, rec.Values[0], i)
		}
	}
}

// TestObjectRoundTripsAsText (T24; B5): an object enters and leaves as its JSON text.
func TestObjectRoundTripsAsText(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	var got string
	var isNull bool
	var mu sync.Mutex
	sink := &fcFuncSink{mode: cepio.Append, write: func(_ context.Context, e cepio.OutEvent) error {
		mu.Lock()
		defer mu.Unlock()
		o := e.Row.AttrIndex("o")
		got, _ = e.Row.Object(o)
		isNull = e.Row.IsNull(o)
		return nil
	}}
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, cep.WithSink("O", sink))
	fcStart(t, ctx, h)
	fcSend(t, ctx, fcInput(t, h, "S"), func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("s"), "a")
		w.SetInt(w.AttrIndex("i"), 1)
		w.SetNull(w.AttrIndex("f"))
		w.SetObject(w.AttrIndex("o"), `{"a":[1,2]}`)
		w.SetTimestamp(w.AttrIndex("t"), fcAt(1))
		return nil
	})
	fcStop(t, ctx, h)
	mu.Lock()
	defer mu.Unlock()
	if got != `{"a":[1,2]}` || isNull {
		t.Errorf("Object(o) = %q, IsNull %v; want the JSON text, false", got, isNull)
	}
}

// TestRowReaderNullsAndTypes (T26; §4.5): a null reads as (zero, false) and IsNull; a wrong type is false; Len is
// the schema's; an unknown name is -1; outside the schema is Invalid and null.
func TestRowReaderNullsAndTypes(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	var mu sync.Mutex
	var fails []string
	sink := &fcFuncSink{mode: cepio.Append, write: func(_ context.Context, e cepio.OutEvent) error {
		mu.Lock()
		defer mu.Unlock()
		row := e.Row
		f, s, i := row.AttrIndex("f"), row.AttrIndex("s"), row.AttrIndex("i")
		if v, ok := row.Float(f); v != 0 || ok {
			fails = append(fails, fmt.Sprintf("Float(f) = %v, %v; want 0, false", v, ok))
		}
		if !row.IsNull(f) {
			fails = append(fails, "IsNull(f) false; want true")
		}
		if v, ok := row.Int(s); v != 0 || ok {
			fails = append(fails, fmt.Sprintf("Int(s) = %v, %v; want 0, false (wrong type)", v, ok))
		}
		if v, ok := row.String(s); v != "a" || !ok {
			fails = append(fails, fmt.Sprintf("String(s) = %q, %v; want a, true", v, ok))
		}
		if v, ok := row.Int(i); v != 7 || !ok {
			fails = append(fails, fmt.Sprintf("Int(i) = %v, %v; want 7, true", v, ok))
		}
		if row.Len() != 5 {
			fails = append(fails, fmt.Sprintf("Len() = %d; want 5", row.Len()))
		}
		if row.AttrIndex("zzz") != -1 {
			fails = append(fails, "AttrIndex(zzz) != -1")
		}
		if row.AttrType(s) != cepio.String || row.AttrType(f) != cepio.Float || row.AttrType(99) != cepio.Invalid {
			fails = append(fails, "AttrType wrong for s, f or 99")
		}
		if !row.IsNull(99) {
			fails = append(fails, "IsNull(99) false; want true outside the schema")
		}
		if v, ok := row.Timestamp(row.AttrIndex("ts")); v != fcAt(1) || !ok {
			fails = append(fails, fmt.Sprintf("Timestamp(ts) = %v, %v; want %d, true", v, ok, fcAt(1)))
		}
		return nil
	}}
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, cep.WithSink("O", sink))
	fcStart(t, ctx, h)
	fcSend(t, ctx, fcInput(t, h, "S"), fcRow("a", 7, fcAt(1)))
	fcStop(t, ctx, h)
	mu.Lock()
	defer mu.Unlock()
	for _, f := range fails {
		t.Error(f)
	}
	if st := fcStats(t, ctx, h); st.Sinks["O"][0].Errors != 0 {
		t.Errorf("sink errors %d; want 0", st.Sinks["O"][0].Errors)
	}
	_ = inmemory.Sink{}
}
