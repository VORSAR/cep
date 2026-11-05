package playback_test

// The log reader (design note v2 §5.1, §5.2; owner D2, D3, D6). Messages are not specified word for word: a fault
// case asserts its line, that Decode is set exactly for a payload fault, and — where the line has one — that the
// message names the offending name (a stream, a table, a key, an op).

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
)

// ldReaderApp: S (event time; d 1 min), R (`time arrival`), Out (derived); T(x, n) keyed by x; K(x) key-only.
func ldReaderApp(t *testing.T) *playback.App {
	out := ldStd(t, "Out", 0, 0)
	out.Derived = true
	return &playback.App{
		Streams: []playback.Stream{ldStd(t, "S", ldMin, 0), ldArrival(t, "R", 0, 0), out},
		Tables: []playback.Table{ldTableT(), {Name: "K", Attrs: []event.Attr{ldAttr("x", event.TString)}, Key: []string{"x"}},
			{Name: "M", Attrs: []event.Attr{ldAttr("x", event.TString), ldAttr("n", event.TInt)}, Key: []string{"x"}}},
	}
}

var (
	ldGood    = ldE("S", ldAt(0), "a", "", "")
	ldHeader  = fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0))
	ldArrS    = func(s float64) string { return fmt.Sprintf(`,"arrival":%q`, ldRFC(ldAt(s))) }
	ldTickAt  = func(s float64) string { return fmt.Sprintf(`{"tick":%q}`, ldRFC(ldAt(s))) }
	ldRecGood = ldE("S", ldAt(0), "a", "", ldArrS(0))
)

// One case per fault of design §5.2. Each log is good up to the faulty line.
func TestReaderFaults(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		log    string
		line   int
		phrase string // "" when the line names nothing in particular
		decode bool
	}{
		// in the envelope
		{"syntax", ldLog(ldGood, `{"stream":"S",`), 2, "", false},
		{"not an object", ldLog(ldGood, `[1,2]`), 2, "", false},
		{"wrong-typed stream (review F7)", ldLog(ldGood, `{"stream":7,"attrs":{"ts":"2026-01-01T10:00:00Z","k":"a"}}`), 2, "stream", false},
		{"ts not RFC 3339 (review F7)", ldLog(`{"stream":"R","ts":"10:00","attrs":{"k":"a"}}`), 1, "ts", false},
		{"unknown key", ldLog(ldGood, ldE("S", ldAt(1), "a", "", `,"foo":1`)), 2, "", false},
		{"no kind key", ldLog(ldGood, `{"attrs":{"k":"a"}}`), 2, "", false},
		{"empty object", ldLog(`{}`), 1, "", false},
		{"forbidden key on a table record", ldLog(`{"table":"T","op":"put","row":{"x":"p"},"stream":"S"}`), 1, "stream", false},
		{"forbidden batch on a batch end's sibling kind", ldLog(`{"table":"T","op":"put","row":{"x":"p"},"batch":"b"}`), 1, "batch", false},
		{"missing attrs", ldLog(`{"stream":"S"}`), 1, "attrs", false},
		{"missing row", ldLog(`{"table":"T","op":"put"}`), 1, "row", false},
		{"missing op", ldLog(`{"table":"T","row":{"x":"p"}}`), 1, "op", false},
		{"batch end missing stream", ldLog(`{"batch_end":"b1"}`), 1, "stream", false},
		{"wrong-typed batch", ldLog(ldE("S", ldAt(0), "a", "", `,"batch":5`)), 1, "batch", false},
		// in naming
		{"unknown stream", ldLog(ldGood, ldE("Nope", ldAt(1), "a", "", "")), 2, "Nope", false},
		{"unknown table", ldLog(`{"table":"Nope","op":"put","row":{"x":"p"}}`), 1, "Nope", false},
		{"derived stream", ldLog(ldE("Out", ldAt(0), "a", "", "")), 1, "Out", false},
		{"batch end on an unknown stream", ldLog(`{"batch_end":"b1","stream":"Nope"}`), 1, "Nope", false},
		// in time (owner D2)
		{"ts on an event-time stream (D2)", ldLog(ldGood, `{"stream":"S","ts":"2026-01-01T10:00:00Z","attrs":{"ts":"2026-01-01T10:00:00Z","k":"a"}}`), 2, "ts", false},
		{"no ts on a time-arrival stream (D2)", ldLog(`{"stream":"R","attrs":{"k":"a"}}`), 1, "ts", false},
		// a fixture with recording records (D3)
		{"arrival in a fixture", ldLog(ldGood, ldE("S", ldAt(1), "a", "", ldArrS(1))), 2, "arrival", false},
		{"tick in a fixture", ldLog(ldGood, ldTickAt(1)), 2, "tick", false},
		{"stop in a fixture", ldLog(ldGood, `{"stop":true}`), 2, "stop", false},
		{"header not first", ldLog(ldGood, ldHeader), 2, "", false},
		// a recording's rules
		{"missing arrival in a recording", ldLog(ldHeader, ldRecGood, ldE("S", ldAt(1), "a", "", "")), 3, "arrival", false},
		{"arrival on a time-arrival stream", ldLog(ldHeader, ldEA("R", ldAt(1), "a", "", ldArrS(1))), 2, "arrival", false},
		{"decreasing arrival", ldLog(ldHeader, ldE("S", ldAt(0), "a", "", ldArrS(5)), ldE("S", ldAt(1), "a", "", ldArrS(4))), 3, "", false},
		{"decreasing tick", ldLog(ldHeader, ldTickAt(10), ldTickAt(9)), 3, "", false},
		{"tick before the last arrival", ldLog(ldHeader, ldE("S", ldAt(0), "a", "", ldArrS(10)), ldTickAt(9)), 3, "", false},
		{"time-arrival ts before the last tick", ldLog(ldHeader, ldTickAt(10), ldEA("R", ldAt(9), "a", "", "")), 3, "", false},
		{"a record after stop", ldLog(ldHeader, ldRecGood, `{"stop":true}`, ldE("S", ldAt(1), "a", "", ldArrS(1))), 4, "", false},
		{"two headers", ldLog(ldHeader, ldHeader), 2, "", false},
		{"tick not RFC 3339", ldLog(ldHeader, `{"tick":"soon"}`), 2, "tick", false},
		{"stop not a bool", ldLog(ldHeader, `{"stop":"yes"}`), 2, "stop", false},
		// in a table record
		{"op upsert", ldLog(`{"table":"T","op":"upsert","row":{"x":"p"}}`), 1, "upsert", false},
		{"put missing a column", ldLog(`{"table":"M","op":"put","row":{"x":"p"}}`), 1, "n", false},
		{"put wrong type", ldLog(`{"table":"T","op":"put","row":{"x":"p","n":"one"}}`), 1, "n", false},
		{"put null key", ldLog(`{"table":"T","op":"put","row":{"x":null,"n":1}}`), 1, "x", false},
		{"delete with a non-key column", ldLog(`{"table":"T","op":"delete","row":{"x":"p","n":1}}`), 1, "", false},
		{"delete missing its key", ldLog(`{"table":"K","op":"delete","row":{}}`), 1, "x", false},
		{"delete null key (review F5)", ldLog(`{"table":"K","op":"delete","row":{"x":null}}`), 1, "x", false},
		// in the payload (owner D6): the cause rides in Decode
		{"payload wrong type", ldLog(ldGood, `{"stream":"S","attrs":{"ts":"2026-01-01T10:00:01Z","k":5}}`), 2, "", true},
		{"payload missing its time", ldLog(`{"stream":"S","attrs":{"k":"a"}}`), 1, "", true},
		{"payload not an object", ldLog(`{"stream":"S","attrs":[1]}`), 1, "", false},
		// lines
		{"BOM at line 1", "\ufeff" + ldLog(ldGood), 1, "", false},
		{"a line over 16 MiB", ldLog(ldGood, ldE("S", ldAt(1), strings.Repeat("a", 16<<20+16), "", "")), 2, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			recs, err := ldRecords(t, ldReaderApp(t), c.log)
			le := ldLoadErr(t, err)
			if le.Line != c.line {
				t.Errorf("LoadError at line %d (%q), want line %d", le.Line, le.Msg, c.line)
			}
			if len(recs) != c.line-1 {
				t.Errorf("%d records before the fault, want %d (every line before it is good)", len(recs), c.line-1)
			}
			if le.Msg == "" {
				t.Error("an empty message")
			}
			if c.phrase != "" && !strings.Contains(le.Msg, c.phrase) {
				t.Errorf("message %q does not name %q", le.Msg, c.phrase)
			}
			if c.decode {
				if le.Decode == nil {
					t.Errorf("a payload fault without its *event.DecodeError (design §5.2, D6): %q", le.Msg)
				}
			} else if le.Decode != nil && !strings.HasPrefix(c.name, "payload") {
				t.Errorf("Decode %v set on a non-payload fault", le.Decode)
			}
			if !strings.Contains(le.Error(), le.Msg) {
				t.Errorf("Error() %q does not carry the message %q", le.Error(), le.Msg)
			}
			// Next after a fault returns the same error.
			r := ldReader(t, ldReaderApp(t), c.log)
			var first error
			for i := 0; i <= c.line; i++ {
				if _, first = r.Next(); first != nil {
					break
				}
			}
			_, again := r.Next()
			le1, le2 := ldLoadErr(t, first), ldLoadErr(t, again)
			if le1.Line != le2.Line || le1.Msg != le2.Msg {
				t.Errorf("Next after a fault: %v, want the same %v", again, first)
			}
		})
	}
}

// A payload fault's Decode is the decoder's own error: the reason and the attribute (owner D6).
func TestReaderPayloadFaultCarriesDecode(t *testing.T) {
	t.Parallel()
	_, err := ldRecords(t, ldReaderApp(t), ldLog(`{"stream":"S","attrs":{"ts":"2026-01-01T10:00:00Z","k":5}}`))
	le := ldLoadErr(t, err)
	if le.Decode == nil || le.Decode.Reason != event.DecodeType || le.Decode.Attr != "k" {
		t.Fatalf("Decode %+v, want DecodeType on attribute k", le.Decode)
	}
}

// What a good log reads to (design §5.2): kinds, ids, fields; whitespace-only lines skipped and counted; CRLF;
// an explicit null reads as absent; io.EOF at the end; fresh Values slices (ADR-0010 §4).
func TestReaderRecords(t *testing.T) {
	t.Parallel()
	log := strings.Join([]string{
		"",
		" \t ",
		ldE("S", ldAt(0), "a", "p", `,"batch":"b1"`),
		`{"table":"T","op":"put","row":{"n":2,"x":"p"}}`,
		"  ",
		`{"table":"K","op":"delete","row":{"x":"q"}}`,
		`{"batch_end":"b1","stream":"S"}`,
		ldEA("R", ldAt(3), "b", "", `,"batch":null`),
		ldE("S", ldAt(1), "c", "", ""),
	}, "\r\n") + "\r\n"
	recs, err := ldRecords(t, ldReaderApp(t), log)
	if !ldEOF(err) {
		t.Fatalf("err %v, want io.EOF", err)
	}
	if len(recs) != 6 {
		t.Fatalf("%d records, want 6: %+v", len(recs), recs)
	}
	e := recs[0]
	if e.Line != 3 || e.Kind != playback.RecEvent || e.Event.StreamID != 0 || e.Event.Ts != ldAt(0) || e.Event.BatchID != "b1" ||
		len(e.Event.Values) != 4 || e.Event.Values[1].Str != "a" || e.Event.Values[2].Str != "p" || !e.Event.Values[3].Null {
		t.Errorf("line 3: %+v", e)
	}
	if p := recs[1]; p.Line != 4 || p.Kind != playback.RecTable || p.Table != 0 || p.Op != playback.OpPut ||
		len(p.Row) != 2 || p.Row[0].Str != "p" || p.Row[1].I != 2 {
		t.Errorf("line 4 (a put: the row in attribute order): %+v", p)
	}
	if d := recs[2]; d.Line != 6 || d.Kind != playback.RecTable || d.Table != 1 || d.Op != playback.OpDelete || len(d.Row) != 1 || d.Row[0].Str != "q" {
		t.Errorf("line 6 (a delete: the key in key order): %+v", d)
	}
	if b := recs[3]; b.Line != 7 || b.Kind != playback.RecBatchEnd || b.Stream != 0 || b.Batch != "b1" {
		t.Errorf("line 7: %+v", b)
	}
	if a := recs[4]; a.Line != 8 || a.Kind != playback.RecEvent || a.Event.StreamID != 1 || a.Event.Ts != ldAt(3) || a.Event.BatchID != "" {
		t.Errorf("line 8 (time arrival; batch null = absent): %+v", a)
	}
	if recs[5].Line != 9 {
		t.Errorf("line 9: %+v", recs[5])
	}
	recs[0].Event.Values[1].Str = "clobbered"
	if recs[5].Event.Values[1].Str != "c" {
		t.Error("two records share a Values slice: every slice in a Record is fresh (ADR-0010 §4)")
	}
	r := ldReader(t, ldReaderApp(t), log)
	for i := 0; i < 6; i++ {
		_, _ = r.Next()
	}
	if _, err := r.Next(); !ldEOF(err) {
		t.Errorf("Next at the end: %v, want io.EOF", err)
	}
	if _, err := r.Next(); !ldEOF(err) {
		t.Errorf("Next after io.EOF: %v, want io.EOF again", err)
	}
}

// Whitespace-only lines count in line numbers, so a fault after them is at its file line.
func TestReaderFaultLineCountsBlankLines(t *testing.T) {
	t.Parallel()
	_, err := ldRecords(t, ldReaderApp(t), ldLog("", ldGood, "   ", "", `{"stream":"Nope","attrs":{}}`))
	if le := ldLoadErr(t, err); le.Line != 5 {
		t.Fatalf("fault at line %d, want 5", le.Line)
	}
}

// D6: Replay delivers every step before a fault, then returns the fault; End is not delivered. fn's own error
// stops it the same way.
func TestReplayStopsAtTheFirstFault(t *testing.T) {
	t.Parallel()
	app := ldReaderApp(t)
	log := ldLog(ldGood, `{"table":"T","op":"put","row":{"x":"p"}}`, `{"stream":"S","attrs":{"ts":"2026-01-01T10:00:01Z","k":5}}`, ldE("S", ldAt(2), "a", "", ""))
	d, err := playback.NewDriver(app)
	if err != nil {
		t.Fatal(err)
	}
	var lines []int
	rerr := playback.Replay(ldReader(t, app, log), d, func(s *playback.Step) error { lines = append(lines, s.Line); return nil })
	le := ldLoadErr(t, rerr)
	if le.Line != 3 || le.Decode == nil {
		t.Errorf("Replay returned %v, want the payload fault at line 3 with its DecodeError", rerr)
	}
	if fmt.Sprint(lines) != "[1 2]" {
		t.Errorf("steps delivered at lines %v, want [1 2]: every step before the fault, nothing after (no End)", lines)
	}
	stop := errors.New("planted")
	d2, _ := playback.NewDriver(app)
	lines = nil
	rerr = playback.Replay(ldReader(t, app, ldLog(ldGood, ldGood, ldGood)), d2, func(s *playback.Step) error {
		lines = append(lines, s.Line)
		if s.Line == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(rerr, stop) || fmt.Sprint(lines) != "[1 2]" {
		t.Errorf("fn's error: Replay returned %v after lines %v, want the planted error after [1 2]", rerr, lines)
	}
}

// §5.1: an invalid App is an error, not a panic: duplicate stream names, duplicate table names, a negative delay.
func TestNewReaderRejectsAnInvalidApp(t *testing.T) {
	t.Parallel()
	s := ldStd(t, "S", 0, 0)
	neg := ldStd(t, "N", 0, 0)
	neg.Delay = -1
	for name, app := range map[string]*playback.App{
		"duplicate stream": {Streams: []playback.Stream{s, s}},
		"duplicate table":  {Streams: []playback.Stream{s}, Tables: []playback.Table{ldTableT(), ldTableT()}},
		"negative delay":   {Streams: []playback.Stream{neg}},
	} {
		if _, err := playback.NewReader(strings.NewReader(""), app); err == nil {
			t.Errorf("%s: NewReader accepted it", name)
		}
		if _, err := playback.NewDriver(app); err == nil {
			t.Errorf("%s: NewDriver accepted it", name)
		}
	}
}
