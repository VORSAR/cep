package playback_test

// S3.2a-1 break-it (QA): attacks on the reader, the driver, the encoder, the trace and Replay. A red test names its
// defect; a guard test (TestBreakitGuard...) records an attack that held. A JSON escape is written as bkBS+"u…" so
// no escape sequence is ever typed into this file.

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
)

const bkBS = "\\"

func bkSchema(t testing.TB, name string, timeAttr int, attrs ...event.Attr) event.Schema {
	t.Helper()
	s, err := event.NewSchema(name, attrs, timeAttr)
	if err != nil {
		t.Fatalf("NewSchema(%s): %v", name, err)
	}
	return s
}

// bkApp: stream A (ts timestamp, k string, x optional string), stream R (`time arrival`: k string), table T (x
// optional string, the key; n int) and table U (x string, the key; n optional int).
func bkApp(t testing.TB) *playback.App {
	t.Helper()
	return &playback.App{
		Streams: []playback.Stream{
			{Schema: bkSchema(t, "A", 0, event.Attr{Name: "ts", Type: event.TTimestamp}, event.Attr{Name: "k", Type: event.TString},
				event.Attr{Name: "x", Type: event.TString, Optional: true}), Delay: 1000, IdleAfter: 5000},
			{Schema: bkSchema(t, "R", event.ArrivalTime, event.Attr{Name: "k", Type: event.TString})},
		},
		Tables: []playback.Table{
			{Name: "T", Attrs: []event.Attr{{Name: "x", Type: event.TString, Optional: true}, {Name: "n", Type: event.TInt}}, Key: []string{"x"}},
			{Name: "U", Attrs: []event.Attr{{Name: "x", Type: event.TString}, {Name: "n", Type: event.TInt, Optional: true}}, Key: []string{"x"}},
		},
	}
}

// bkRead reads every record of log up to the first error (io.EOF at a clean end).
func bkRead(t testing.TB, app *playback.App, log string) ([]playback.Record, error) {
	t.Helper()
	r, err := playback.NewReader(strings.NewReader(log), app)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var recs []playback.Record
	for {
		rec, err := r.Next()
		if err != nil {
			return recs, err
		}
		recs = append(recs, rec)
	}
}

// bkFault asserts log faults with a *LoadError at line.
func bkFault(t *testing.T, app *playback.App, what, log string, line int) {
	t.Helper()
	_, err := bkRead(t, app, log)
	var le *playback.LoadError
	if !errors.As(err, &le) || le.Line != line {
		t.Errorf("%s: want a *LoadError at line %d, got %v", what, line, err)
	}
}

// bkClean asserts log reads to its end with n records.
func bkClean(t *testing.T, app *playback.App, what, log string, n int) []playback.Record {
	t.Helper()
	recs, err := bkRead(t, app, log)
	if err == nil || err.Error() != "EOF" || len(recs) != n {
		t.Errorf("%s: want %d records and EOF, got %d and %v", what, n, len(recs), err)
	}
	return recs
}

// TestBreakitTimestampOffsetOutsideEncodableYears: the reader accepts an RFC 3339 time whose offset puts the instant
// before 0000-01-01T00:00:00Z or after 9999-12-31T23:59:59.999Z, and the encoder refuses it. Design §5.4's round
// trip: "for every event a Reader accepts, AppendEvent gives a line the Reader reads back". The same event's trace
// entry ("ts") makes Trace.Bytes fail.
func TestBreakitTimestampOffsetOutsideEncodableYears(t *testing.T) {
	app := bkApp(t)
	for _, ts := range []string{"0000-01-01T00:00:00+00:01", "9999-12-31T23:59:59.999-00:01"} {
		line := `{"stream":"A","attrs":{"ts":"` + ts + `","k":"a"}}`
		recs, err := bkRead(t, app, line+"\n")
		if len(recs) != 1 {
			t.Logf("%s: the reader refuses it (%v): the round trip holds for this case", ts, err)
			continue
		}
		e := recs[0].Event
		out, err := playback.AppendEvent(nil, app.Streams[0].Schema, e)
		if err != nil {
			t.Errorf("ts %s: the Reader accepts the event (Ts %d ms) but AppendEvent refuses it: %v (design §5.4 round trip)", ts, e.Ts, err)
			continue
		}
		back, err := bkRead(t, app, string(out)+"\n")
		if len(back) != 1 || back[0].Event.Ts != e.Ts {
			t.Errorf("ts %s: round trip %s read back as %v, %v", ts, out, back, err)
		}
	}
}

// TestBreakitGuardEndAtTheLastRecordsLine: break-it K3 (later), dispositioned by the design: the end entry's "at" is
// the last record's line + 1 (design §5.3, §5.5 as annotated), not the last line's: trailing blank lines are not
// steps. A log whose last lines are blank ends at its record's line + 1.
func TestBreakitGuardEndAtTheLastRecordsLine(t *testing.T) {
	app := bkApp(t)
	log := `{"stream":"R","ts":"2026-01-01T10:00:00Z","attrs":{"k":"a"}}` + "\n\n  \n"
	r, err := playback.NewReader(strings.NewReader(log), app)
	if err != nil {
		t.Fatal(err)
	}
	d, err := playback.NewDriver(app)
	if err != nil {
		t.Fatal(err)
	}
	var end int
	if err := playback.Replay(r, d, func(s *playback.Step) error {
		if s.Kind == playback.StepEnd {
			end = s.Line
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if end != 2 {
		t.Errorf("a 3-line log (one record at line 1, two blank lines): end at %d, want 2 (the last record's line + 1)", end)
	}
}

// TestBreakitGuardReaderEdges: line-level and envelope edges of design §5.2 that held.
func TestBreakitGuardReaderEdges(t *testing.T) {
	app := bkApp(t)
	ev := `{"stream":"R","ts":"2026-01-01T10:00:00Z","attrs":{"k":"a"}}`
	hdr := `{"recording":{"start":"2026-01-01T10:00:00Z"}}`
	bkFault(t, app, "a BOM in the middle", ev+"\n\xef\xbb\xbf"+ev+"\n", 2)
	bkFault(t, app, "a NUL byte", ev+"\n"+`{"stream":"R","ts":"2026-01-01T10:00:00Z","attrs":{"k":"a`+"\x00"+`"}}`+"\n", 2)
	bkFault(t, app, "a NUL between records", ev+"\x00\n", 1)
	bkFault(t, app, "a lone CR between two records", ev+"\r"+ev+"\n", 1)
	bkClean(t, app, "CR CR LF", ev+"\r\r\n", 1)
	bkClean(t, app, "a last line with no newline, CR-ended", ev+"\n"+ev+"\r", 2)
	bkFault(t, app, `"stop":false`, hdr+"\n"+`{"stop":false}`+"\n", 2)
	bkFault(t, app, `"stop":"true"`, hdr+"\n"+`{"stop":"true"}`+"\n", 2)
	bkFault(t, app, "a record after stop", hdr+"\n"+`{"stop":true}`+"\n\n"+`{"tick":"2026-01-01T10:00:01Z"}`+"\n", 4)
	bkClean(t, app, "blank lines after stop", hdr+"\n"+`{"stop":true}`+"\n \n\t\n", 2)
	bkClean(t, app, "a header after blank lines", "\n \n"+hdr+"\n", 1)
	bkFault(t, app, "a second header", hdr+"\n"+hdr+"\n", 2)
	recs := bkClean(t, app, "an arrival equal to the previous tick", hdr+"\n"+`{"tick":"2026-01-01T10:00:05Z"}`+"\n"+
		`{"stream":"A","arrival":"2026-01-01T10:00:05Z","attrs":{"ts":"2026-01-01T09:00:00Z","k":"a"}}`+"\n"+
		`{"tick":"2026-01-01T10:00:05Z"}`+"\n", 4)
	if len(recs) == 4 && recs[2].Arrival != recs[1].Time {
		t.Errorf("arrival %d, want the tick's %d", recs[2].Arrival, recs[1].Time)
	}
	bkFault(t, app, "an arrival before the previous tick", hdr+"\n"+`{"tick":"2026-01-01T10:00:05Z"}`+"\n"+
		`{"stream":"A","arrival":"2026-01-01T10:00:04.999Z","attrs":{"ts":"2026-01-01T09:00:00Z","k":"a"}}`+"\n", 3)
	recs = bkClean(t, app, "repeated keys: the last wins", `{"stream":"A","stream":"R","ts":"2026-01-01T10:00:00Z","attrs":{"ts":"bad"},"attrs":{"k":"z"}}`+"\n", 1)
	if len(recs) == 1 && (recs[0].Event.StreamID != 1 || recs[0].Event.Values[0].Str != "z") {
		t.Errorf("repeated keys: got stream %d values %v, want R and k z", recs[0].Event.StreamID, recs[0].Event.Values)
	}
	bkFault(t, app, "a put with its optional key column null", `{"table":"T","op":"put","row":{"x":null,"n":1}}`+"\n", 1)
	bkFault(t, app, "a put with its optional key column absent", `{"table":"T","op":"put","row":{"n":1}}`+"\n", 1)
	bkFault(t, app, "a delete with its key null", `{"table":"T","op":"delete","row":{"x":null}}`+"\n", 1)
	bkFault(t, app, "a delete with a non-key column", `{"table":"U","op":"delete","row":{"x":"a","n":1}}`+"\n", 1)
	bkFault(t, app, "attrs as an array", `{"stream":"R","ts":"2026-01-01T10:00:00Z","attrs":["k"]}`+"\n", 1)
	bkFault(t, app, "a header as an array", `{"recording":["2026-01-01T10:00:00Z"]}`+"\n", 1)
	bkFault(t, app, "a header start with a second key", `{"recording":{"start":"2026-01-01T10:00:00Z","x":1}}`+"\n", 1)
	bkFault(t, app, "a stream and a table key", `{"table":"U","op":"put","row":{"x":"a"},"stream":"R"}`+"\n", 1)
}

// TestBreakitGuardLongLines: a line of exactly 16 MiB (its CRLF excluded) is read; one byte more is a fault at its
// line; the fault stops the reader and Next keeps returning it.
func TestBreakitGuardLongLines(t *testing.T) {
	if testing.Short() {
		t.Skip("16 MiB lines")
	}
	app := bkApp(t)
	head, tail := `{"stream":"R","ts":"2026-01-01T10:00:00Z","attrs":{"k":"`, `"}}`
	const maxLine = 16 << 20
	exact := head + strings.Repeat("a", maxLine-len(head)-len(tail)) + tail
	bkClean(t, app, "exactly 16 MiB, CRLF", "\n"+exact+"\r\n", 1)
	over := head + strings.Repeat("a", maxLine+1-len(head)-len(tail)) + tail
	r, err := playback.NewReader(strings.NewReader("\n"+over+"\n"+exact+"\n"), app)
	if err != nil {
		t.Fatal(err)
	}
	_, e1 := r.Next()
	_, e2 := r.Next()
	var le *playback.LoadError
	if !errors.As(e1, &le) || le.Line != 2 || e1 != e2 {
		t.Errorf("16 MiB + 1: want one *LoadError at line 2, twice; got %v then %v", e1, e2)
	}
}

// TestBreakitGuardRecordsAreFresh: design §5.2 MUST: every slice in a Record is fresh (no aliasing between records).
func TestBreakitGuardRecordsAreFresh(t *testing.T) {
	app := bkApp(t)
	log := `{"stream":"A","attrs":{"ts":"2026-01-01T10:00:00Z","k":"a","x":"p"}}` + "\n" +
		`{"stream":"A","attrs":{"ts":"2026-01-01T10:00:01Z","k":"b","x":"q"}}` + "\n" +
		`{"table":"U","op":"put","row":{"x":"r","n":1}}` + "\n" + `{"table":"U","op":"put","row":{"x":"s","n":2}}` + "\n"
	recs := bkClean(t, app, "four records", log, 4)
	if len(recs) != 4 {
		return
	}
	recs[0].Event.Values[1].Str = "MUT"
	recs[2].Row[0].Str = "MUT"
	if recs[1].Event.Values[1].Str != "b" || recs[3].Row[0].Str != "s" {
		t.Errorf("a record's slice aliases another's: %v %v", recs[1].Event.Values, recs[3].Row)
	}
}

// TestBreakitGuardDriverTableEdges: a delete of a missing key and a put that replaces are table steps that publish
// a new snapshot; an event read after them carries it.
func TestBreakitGuardDriverTableEdges(t *testing.T) {
	app := bkApp(t)
	log := `{"table":"U","op":"delete","row":{"x":"nobody"}}` + "\n" + `{"table":"U","op":"put","row":{"x":"a"}}` + "\n" +
		`{"table":"U","op":"put","row":{"x":"a","n":2}}` + "\n" + `{"stream":"R","ts":"2026-01-01T10:00:00Z","attrs":{"k":"a"}}` + "\n"
	r, _ := playback.NewReader(strings.NewReader(log), app)
	d, _ := playback.NewDriver(app)
	var steps []playback.Step
	if err := playback.Replay(r, d, func(s *playback.Step) error { steps = append(steps, *s); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 || steps[3].Event.Tables != any(steps[2].Tables) || steps[0].Tables == steps[1].Tables {
		t.Fatalf("steps %d; the event's snapshot must be the last table step's", len(steps))
	}
	row, ok := steps[3].Tables.Table(1).Get([]event.Value{{Kind: event.TString, Str: "a"}})
	if !ok || row[1].I != 2 {
		t.Errorf("U[a] = %v %v, want n 2", row, ok)
	}
}

// TestBreakitGuardReplayEdges: fn's error stops Replay at that step and is returned; a recording with no event
// drains nothing at its stop, and its End adds nothing.
func TestBreakitGuardReplayEdges(t *testing.T) {
	app := bkApp(t)
	ev := `{"stream":"R","ts":"2026-01-01T10:00:00Z","attrs":{"k":"a"}}`
	boom := errors.New("boom")
	r, _ := playback.NewReader(strings.NewReader(ev+"\n"+ev+"\n"+ev+"\n"), app)
	d, _ := playback.NewDriver(app)
	n := 0
	if err := playback.Replay(r, d, func(*playback.Step) error {
		n++
		if n == 2 {
			return boom
		}
		return nil
	}); err != boom || n != 2 {
		t.Errorf("fn's error at step 2: Replay returned %v after %d steps", err, n)
	}
	r, _ = playback.NewReader(strings.NewReader(`{"recording":{"start":"2026-01-01T10:00:00Z"}}`+"\n"+`{"stop":true}`+"\n"), app)
	d, _ = playback.NewDriver(app)
	var kinds []playback.StepKind
	if err := playback.Replay(r, d, func(s *playback.Step) error {
		kinds = append(kinds, s.Kind)
		if len(s.Advances) != 0 {
			t.Errorf("step %v: advances %v, want none (nothing admitted, no +inf in a recording)", s.Kind, s.Advances)
		}
		return nil
	}); err != nil || len(kinds) != 3 || kinds[2] != playback.StepEnd {
		t.Errorf("an empty recording: %v %v", kinds, err)
	}
}

// bkRandValue writes a random JSON value of type ty that the decoder accepts.
func bkRandValue(rng *rand.Rand, ty event.Type) string {
	switch ty {
	case event.TString:
		var b strings.Builder
		b.WriteByte('"')
		for range rng.IntN(8) {
			switch rng.IntN(10) {
			case 0:
				b.WriteString(bkBS + "u00" + fmt.Sprintf("%02x", rng.IntN(0x20)))
			case 1:
				b.WriteString(bkBS + "ud83d" + bkBS + "ude00") // a surrogate pair
			case 2:
				b.WriteString(bkBS + "ud800") // a lone surrogate
			case 3:
				b.WriteString(string(rune(0x2028)) + string(rune(0x2029)) + "\x7f")
			case 4:
				b.WriteByte(byte(0x80 + rng.IntN(0x80))) // malformed UTF-8
			case 5:
				b.WriteString(bkBS + `"` + bkBS + bkBS + bkBS + "/" + bkBS + "b" + bkBS + "t")
			default:
				b.WriteByte(byte('a' + rng.IntN(26)))
			}
		}
		b.WriteByte('"')
		return b.String()
	case event.TInt, event.TDuration:
		switch rng.IntN(4) {
		case 0:
			return strconv.FormatInt(math.MaxInt64, 10)
		case 1:
			return strconv.FormatInt(math.MinInt64, 10)
		}
		return strconv.FormatInt(rng.Int64()>>rng.IntN(64), 10)
	case event.TFloat:
		for {
			f := math.Float64frombits(rng.Uint64())
			if rng.IntN(5) == 0 {
				f = []float64{math.Copysign(0, -1), 5e-324, math.MaxFloat64, 1e21, 1e-7, 123456789012345678}[rng.IntN(6)]
			}
			if !math.IsNaN(f) && !math.IsInf(f, 0) {
				return strconv.FormatFloat(f, []byte{'g', 'e', 'f'}[rng.IntN(3)], -1, 64)
			}
		}
	case event.TBool:
		return []string{"true", "false"}[rng.IntN(2)]
	case event.TTimestamp:
		return `"` + []string{"0000-01-01T00:00:00Z", "9999-12-31T23:59:59.999Z", "2026-06-30T23:59:60Z",
			"2026-01-01T10:00:00.123456789+05:30", "1969-12-31T23:59:59.9995-00:00", "2024-02-29t01:02:03.5z"}[rng.IntN(6)] + `"`
	case event.TObject:
		return []string{`{ "a" : [ 1 , 2.5e3 , { } ] , "b" : "x  y` + bkBS + `"\t" }`, "[ ]", `{"s":"` + "\xff\xe2\x82" + `"}`,
			`[` + strings.Repeat("[ ", 50) + strings.Repeat("] ", 50) + `]`, `{"k":"` + bkBS + "u00e9" + `" ,"n":null}`}[rng.IntN(5)]
	}
	panic(ty)
}

// TestBreakitGuardRoundTripRandom: design §5.4's round trip over randomized lines of every type, nulls, escapes,
// malformed UTF-8, float and timestamp edges and messy objects: the Reader's event, encoded, reads back equal, and
// encoding is a fixed point (the second encoding is byte-identical).
func TestBreakitGuardRoundTripRandom(t *testing.T) {
	types := []event.Type{event.TString, event.TInt, event.TFloat, event.TBool, event.TTimestamp, event.TDuration, event.TObject}
	attrs := []event.Attr{{Name: "t0", Type: event.TTimestamp}}
	for i, ty := range types {
		attrs = append(attrs, event.Attr{Name: "a" + strconv.Itoa(i), Type: ty, Optional: true})
	}
	app := &playback.App{Streams: []playback.Stream{{Schema: bkSchema(t, "S", 0, attrs...)}}}
	sch := app.Streams[0].Schema
	rng := rand.New(rand.NewPCG(5, 5))
	for n := range 3000 {
		var b strings.Builder
		b.WriteString(`{"stream":"S"`)
		if rng.IntN(3) == 0 {
			b.WriteString(`,"batch":"b` + bkRandValue(rng, event.TString)[1:]) // never empty: an empty batch id is a fault
		}
		b.WriteString(`,"attrs":{"t0":` + bkRandValue(rng, event.TTimestamp))
		for i, ty := range types {
			switch rng.IntN(4) {
			case 0: // absent
			case 1:
				b.WriteString(`,"a` + strconv.Itoa(i) + `":null`)
			default:
				b.WriteString(`,"a` + strconv.Itoa(i) + `":` + bkRandValue(rng, ty))
			}
		}
		b.WriteString("}}\n")
		recs, err := bkRead(t, app, b.String())
		if len(recs) != 1 {
			t.Fatalf("#%d: the reader refused a generated line %q: %v", n, b.String(), err)
		}
		e := recs[0].Event
		out, err := playback.AppendEvent(nil, sch, e)
		if err != nil {
			t.Fatalf("#%d: AppendEvent: %v (line %q)", n, err, b.String())
		}
		back, err := bkRead(t, app, string(out)+"\n")
		if len(back) != 1 {
			t.Fatalf("#%d: the encoded line %q does not read back: %v", n, out, err)
		}
		f := back[0].Event
		if f.Ts != e.Ts || f.BatchID != e.BatchID || len(f.Values) != len(e.Values) {
			t.Fatalf("#%d: %q -> %q: Ts/BatchID differ", n, b.String(), out)
		}
		for i := range e.Values {
			x, y := e.Values[i], f.Values[i]
			same := x.Kind == y.Kind && x.Null == y.Null && x.Str == y.Str && x.I == y.I && x.B == y.B && x.T == y.T &&
				x.D == y.D && math.Float64bits(x.F) == math.Float64bits(y.F) && x.Obj == y.Obj
			if !same {
				t.Fatalf("#%d: attribute %d: %+v read back as %+v (line %q, encoded %q)", n, i, x, y, b.String(), out)
			}
		}
		again, _ := playback.AppendEvent(nil, sch, f)
		if !bytes.Equal(out, again) {
			t.Fatalf("#%d: encoding is not a fixed point: %q then %q", n, out, again)
		}
	}
}

// TestBreakitGuardAppendEventAllocatesNothing: AppendEvent into a buffer with room allocates nothing (its doc).
func TestBreakitGuardAppendEventAllocatesNothing(t *testing.T) {
	app := bkApp(t)
	e := event.Event{StreamID: 0, Ts: 1, BatchID: "b", Values: []event.Value{{Kind: event.TTimestamp, T: 1},
		{Kind: event.TString, Str: "k" + string(rune(0x2028)) + "\x01\xff"}, {Kind: event.TString, Null: true}}}
	buf := make([]byte, 0, 512)
	if n := testing.AllocsPerRun(200, func() { _, _ = playback.AppendEvent(buf[:0], app.Streams[0].Schema, e) }); n != 0 {
		t.Errorf("AppendEvent allocates %v", n)
	}
	if _, err := playback.AppendValue(nil, event.Value{Kind: event.TObject, Obj: []byte("{}")}); err == nil {
		t.Errorf("an object value that is not a string encodes without an error")
	}
}
