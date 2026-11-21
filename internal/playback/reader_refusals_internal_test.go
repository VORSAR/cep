package playback

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// The reader nets' times: cnT0 is the recording's start, cnBefore one millisecond earlier.
const (
	cnT0     = "2026-01-01T10:00:00.000Z"
	cnBefore = "2026-01-01T09:59:59.999Z"
	cnHeader = `{"recording":{"start":"` + cnT0 + `"}}`
)

// cnReaderApp reads against S(k string, ts timestamp) timed by ts, A(k string) of `time arrival`, a derived D(k
// string), and the table T(k string, v int) keyed by k.
func cnReaderApp(t testing.TB) *App {
	t.Helper()
	k := event.Attr{Name: "k", Type: event.TString}
	return &App{
		Streams: []Stream{
			{Schema: cnSchema(t, "S", 1, k, event.Attr{Name: "ts", Type: event.TTimestamp})},
			{Schema: cnSchema(t, "A", event.ArrivalTime, k)},
			{Schema: cnSchema(t, "D", event.ArrivalTime, k), Derived: true},
		},
		Tables: []Table{{Name: "T", Attrs: []event.Attr{k, {Name: "v", Type: event.TInt}}, Key: []string{"k"}}},
	}
}

// cnFault reads log to its first fault and returns it as a *LoadError; the test fails if there is none.
func cnFault(t testing.TB, app *App, log string) *LoadError {
	t.Helper()
	rd, err := NewReader(strings.NewReader(log), app)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			t.Fatalf("log %q read to its end; want a fault", log)
		}
		if err != nil {
			le, ok := errors.AsType[*LoadError](err)
			if !ok {
				t.Fatalf("log %q: %v (%T); want a *LoadError", log, err, err)
			}
			return le
		}
	}
}

// cnRecords reads every record of log; the test fails at a fault.
func cnRecords(t testing.TB, app *App, log string) []Record {
	t.Helper()
	rd, err := NewReader(strings.NewReader(log), app)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var out []Record
	for {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("log %q: %v; want no fault", log, err)
		}
		out = append(out, rec)
	}
}

// cnFaultCase is a log and the fault it must stop at.
type cnFaultCase struct {
	name string
	log  string
	line int
	msg  string
}

func cnCheckFaults(t *testing.T, cases []cnFaultCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			le := cnFault(t, cnReaderApp(t), c.log)
			if le.Line != c.line || le.Msg != c.msg {
				t.Errorf("fault = line %d %q; want line %d %q", le.Line, le.Msg, c.line, c.msg)
			}
		})
	}
}

// TestNewReaderRefusesATableItCannotDecode: a table event.NewSchema refuses (no attributes) is refused with the
// schema's error (reader.go:122-125).
func TestNewReaderRefusesATableItCannotDecode(t *testing.T) {
	t.Parallel()
	_, err := NewReader(strings.NewReader(""), &App{Tables: []Table{{Name: "T"}}})
	const want = `playback: table "T": event: invalid schema for stream "T": no attributes declared (a stream declares at ` +
		`least one)`
	if err == nil || err.Error() != want {
		t.Errorf("NewReader(a table of no attributes) = %v; want %q", err, want)
	}
}

// TestNewReaderRefusesAKeyColumnNotDeclared: a key naming no column is refused (reader.go:135-137).
func TestNewReaderRefusesAKeyColumnNotDeclared(t *testing.T) {
	t.Parallel()
	app := &App{Tables: []Table{{Name: "T", Attrs: []event.Attr{{Name: "k", Type: event.TString}}, Key: []string{"z"}}}}
	_, err := NewReader(strings.NewReader(""), app)
	const want = `playback: table "T": key column "z" is not declared`
	if err == nil || err.Error() != want {
		t.Errorf("NewReader(key z) = %v; want %q", err, want)
	}
}

// TestNewReaderRefusesAKeyNamedTwice: a key naming one column twice builds a key schema with a duplicate attribute,
// which event.NewSchema refuses (reader.go:139-142).
func TestNewReaderRefusesAKeyNamedTwice(t *testing.T) {
	t.Parallel()
	app := &App{Tables: []Table{{Name: "T", Attrs: []event.Attr{{Name: "k", Type: event.TString}},
		Key: []string{"k", "k"}}}}
	_, err := NewReader(strings.NewReader(""), app)
	const want = `playback: table "T"'s key: event: invalid schema for stream "T": attribute 1 "k": duplicate attribute ` +
		`name, first declared as attribute 0 (names are case-sensitive)`
	if err == nil || err.Error() != want {
		t.Errorf("NewReader(key k, k) = %v; want %q", err, want)
	}
}

// TestNewReaderRefusesAnAppItCannotRead: validateApp's refusals in their order (reader.go:151-179), with both
// operands of each `||`.
func TestNewReaderRefusesAnAppItCannotRead(t *testing.T) {
	t.Parallel()
	s := func(delay, idle int64) Stream {
		return Stream{Schema: cnSchema(t, "S", event.ArrivalTime, event.Attr{Name: "k", Type: event.TString}),
			Delay: delay, IdleAfter: idle}
	}
	tbl := Table{Name: "T", Attrs: []event.Attr{{Name: "k", Type: event.TString}}, Key: []string{"k"}}
	cases := []struct {
		name string
		app  *App
		want string
	}{
		{"nil", nil, "playback: no app"},
		{"a zero stream", &App{Streams: []Stream{s(0, 0), {}}}, "playback: stream 1 has no schema"},
		{"a stream declared twice", &App{Streams: []Stream{s(0, 0), s(0, 0)}}, `playback: stream "S" is declared twice`},
		{"a negative delay", &App{Streams: []Stream{s(-1, 0)}}, `playback: stream "S" has a negative bound`},
		{"a negative idle", &App{Streams: []Stream{s(0, -1)}}, `playback: stream "S" has a negative bound`},
		{"a table with no name", &App{Tables: []Table{tbl, {}}}, "playback: table 1 has no name"},
		{"a table declared twice", &App{Tables: []Table{tbl, tbl}}, `playback: table "T" is declared twice`},
	}
	for _, c := range cases {
		if _, err := NewReader(strings.NewReader(""), c.app); err == nil || err.Error() != c.want {
			t.Errorf("%s: NewReader = %v; want %q", c.name, err, c.want)
		}
	}
	if _, err := NewReader(strings.NewReader(""), &App{Streams: []Stream{s(1, 1)}, Tables: []Table{tbl}}); err != nil {
		t.Errorf("NewReader(positive bounds) = %v; want nil", err)
	}
}

// cnEndless yields 'a' forever until it has yielded limit bytes, then fails with errCnEndless.
type cnEndless struct{ n, limit int }

var errCnEndless = errors.New("the endless reader ran out")

func (r *cnEndless) Read(p []byte) (int, error) {
	if r.n >= r.limit {
		return 0, errCnEndless
	}
	m := min(len(p), r.limit-r.n)
	for i := range m {
		p[i] = 'a'
	}
	r.n += m
	return m, nil
}

// TestReaderStopsAnEndlessLine: a line that never ends is refused once it passes maxLine+2 bytes
// (reader.go:220-222): a LoadError at line 1, never the reader's own error, which comes only after twice maxLine.
func TestReaderStopsAnEndlessLine(t *testing.T) {
	t.Parallel()
	rd, err := NewReader(&cnEndless{limit: 2 * maxLine}, cnReaderApp(t))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	_, err = rd.Next()
	le, ok := errors.AsType[*LoadError](err)
	if !ok || le.Line != 1 || le.Msg != "the line is longer than 16777216 bytes" {
		t.Fatalf("Next = %v; want the LoadError at line 1 \"the line is longer than 16777216 bytes\"", err)
	}
}

// cnFailing yields its text, then fails with errCnRead.
type cnFailing struct{ text string }

var errCnRead = errors.New("the test reader's failure")

func (r *cnFailing) Read(p []byte) (int, error) {
	if r.text == "" {
		return 0, errCnRead
	}
	n := copy(p, r.text)
	r.text = r.text[n:]
	return n, nil
}

// TestReaderReturnsAReadError: a read error that is not io.EOF is Next's error itself, not a LoadError
// (reader.go:231-233), after the lines before it, whether it cuts a line or comes at a line's end with nothing read
// (reader.go:228's io.EOF operand); Next returns it again.
func TestReaderReturnsAReadError(t *testing.T) {
	t.Parallel()
	line := `{"table":"T","op":"put","row":{"k":"a","v":1}}` + "\n"
	for _, text := range []string{line + `{"tab`, line} {
		rd, err := NewReader(&cnFailing{text: text}, cnReaderApp(t))
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		if rec, err := rd.Next(); err != nil || rec.Kind != RecTable {
			t.Fatalf("text %q: Next = %+v, %v; want the table record", text, rec, err)
		}
		for range 2 {
			_, err := rd.Next()
			if _, isLoad := errors.AsType[*LoadError](err); !errors.Is(err, errCnRead) || isLoad {
				t.Fatalf("text %q: Next = %v; want the reader's own error", text, err)
			}
		}
	}
}

// TestReaderReadsEachLineEnding: readLine's operands (reader.go:228-239) — io.EOF with nothing left; io.EOF after a
// last line with no newline; "\n" alone (an empty line); "\r\n"; a final "\r" with no "\n".
func TestReaderReadsEachLineEnding(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"x", []string{"x"}},
		{"\n", []string{""}},
		{"a\n\nb\r\n\r\nc\r", []string{"a", "", "b", "", "c"}},
		{"d\r\n", []string{"d"}},
	}
	for _, c := range cases {
		rd, err := NewReader(strings.NewReader(c.in), cnReaderApp(t))
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		var got []string
		eof := false
		for range len(c.want) + 2 { // bounded: a readLine that never says io.EOF fails here, not at go test's timeout
			line, err := rd.readLine()
			if errors.Is(err, io.EOF) {
				eof = true
				break
			}
			if err != nil {
				t.Fatalf("readLine(%q): %v", c.in, err)
			}
			got = append(got, string(line))
		}
		if !eof || strings.Join(got, "|") != strings.Join(c.want, "|") || len(got) != len(c.want) {
			t.Errorf("readLine(%q) lines = %q (io.EOF reached: %v); want %q, then io.EOF", c.in, got, eof, c.want)
		}
	}
}

// TestReaderNamesTheFirstFaultOfALine: two-fault lines in record's order (reader.go:262-294) — a decode fault, an
// unknown key, a record after the stop, no choosing key, a key the kind does not allow, then the kind's own checks.
func TestReaderNamesTheFirstFaultOfALine(t *testing.T) {
	t.Parallel()
	stop := `{"stop":true}`
	cnCheckFaults(t, []cnFaultCase{
		{"an unknown key after the stop names the unknown key", cnHeader + "\n" + stop + "\n" + `{"bogus":1}`, 3,
			"a key the log format does not define"},
		{"no choosing key after the stop names the stop", cnHeader + "\n" + stop + "\n" + `{"attrs":{}}`, 3,
			"a record after the stop record"},
		{"a table with a stream key names the key", `{"table":"T","op":"put","row":{"k":"a","v":1},"stream":"S"}`, 1,
			`a table record takes no "stream" key`},
		{"a tick with a stream key and no header names the key", `{"tick":"` + cnT0 + `","stream":"S"}`, 1,
			`a tick record takes no "stream" key`},
		{"no choosing key", `{"attrs":{}}`, 1,
			"a record names none of recording, table, batch_end, tick, stop, rejected or stream"},
	})
}

// cnRej is a rejected record on stream with the fault object fault, arriving at arrival.
func cnRej(stream, fault, arrival string) string {
	return `{"rejected":` + fault + `,"stream":"` + stream + `","arrival":"` + arrival + `"}`
}

// TestReaderRefusesAMalformedRejectedRecord: every refusal of rejectedRecord (reader.go:315-348), with both operands of
// `!present(eStream) || !present(eArrival)`, and the valid forms.
func TestReaderRefusesAMalformedRejectedRecord(t *testing.T) {
	t.Parallel()
	h := cnHeader + "\n"
	const carries = "a rejected record carries stream and arrival"
	cnCheckFaults(t, []cnFaultCase{
		{"no header", cnRej("S", `{"reason":"syntax"}`, cnT0), 1, "a rejected record in a log without a recording header"},
		{"no stream", h + `{"rejected":{"reason":"syntax"},"arrival":"` + cnT0 + `"}`, 2, carries},
		{"no arrival", h + `{"rejected":{"reason":"syntax"},"stream":"S"}`, 2, carries},
		{"neither", h + `{"rejected":{"reason":"syntax"}}`, 2, carries},
		{"an unknown stream", h + cnRej("Z", `{"reason":"syntax"}`, cnT0), 2, `unknown stream "Z"`},
		{"a derived stream", h + cnRej("D", `{"reason":"syntax"}`, cnT0), 2,
			`stream "D" is a rule's output; a log holds source streams only`},
		{"a nested decode fault", h + cnRej("S", `{"reason":1}`, cnT0), 2,
			`the rejected record: attribute "reason" (string) at offset 10: want a string, found a number "1"`},
		{"a nested unknown key", h + cnRej("S", `{"reason":"syntax","x":1}`, cnT0), 2,
			"the rejected record has a key other than reason and attr"},
		{"a bogus reason", h + cnRej("S", `{"reason":"bogus"}`, cnT0), 2,
			`the rejected record's reason "bogus" is none of syntax, type, missing, null`},
		{"syntax with an attr", h + cnRej("S", `{"attr":"k","reason":"syntax"}`, cnT0), 2,
			"a syntax fault names no attribute"},
		{"type without an attr", h + cnRej("S", `{"reason":"type"}`, cnT0), 2, "a type fault names its attribute"},
		{"an arrival before the start", h + cnRej("S", `{"attr":"k","reason":"type"}`, cnBefore), 2,
			"an arrival before the previous arrival or tick"},
	})
	ms := func(s string) int64 { // the header's start, read the reader's own way
		recs := cnRecords(t, cnReaderApp(t), `{"recording":{"start":"`+s+`"}}`)
		return recs[0].Time
	}
	t0 := ms(cnT0)
	cases := []struct {
		fault  string
		reason event.DecodeReason
		attr   string
	}{
		{`{"reason":"syntax"}`, event.DecodeSyntax, ""},
		{`{"attr":"k","reason":"type"}`, event.DecodeType, "k"},
		{`{"attr":"k","reason":"missing"}`, event.DecodeMissing, "k"},
		{`{"attr":"k","reason":"null"}`, event.DecodeNull, "k"},
	}
	for _, c := range cases {
		recs := cnRecords(t, cnReaderApp(t), h+cnRej("A", c.fault, cnT0))
		want := Record{Line: 2, Kind: RecRejected, Stream: 1, Arrival: t0, Reason: c.reason, Attr: c.attr}
		if len(recs) != 2 || recs[1].Line != want.Line || recs[1].Kind != want.Kind || recs[1].Stream != want.Stream ||
			recs[1].Arrival != want.Arrival || recs[1].Reason != want.Reason || recs[1].Attr != want.Attr {
			t.Errorf("rejected %s = %+v; want %+v", c.fault, recs, want)
		}
	}
}

// TestReaderNamesTheFirstFaultOfARejectedRecord: one two-fault case per adjacent boundary of the rejected record's
// checks (design s1-1c-6 P.2: the stream, then rejectedFault's decode fault, unknown key, reason, and its two
// attribute rules, then advance).
func TestReaderNamesTheFirstFaultOfARejectedRecord(t *testing.T) {
	t.Parallel()
	h := cnHeader + "\n"
	cnCheckFaults(t, []cnFaultCase{
		{"an unknown stream before a decode fault", h + cnRej("Z", `{"reason":1}`, cnT0), 2, `unknown stream "Z"`},
		{"a decode fault before an unknown key", h + cnRej("S", `{"reason":1,"x":1}`, cnT0), 2,
			`the rejected record: attribute "reason" (string) at offset 10: want a string, found a number "1"`},
		{"an unknown key before a bogus reason", h + cnRej("S", `{"reason":"bogus","x":1}`, cnT0), 2,
			"the rejected record has a key other than reason and attr"},
		{"a bogus reason before the attribute rule", h + cnRej("S", `{"attr":"k","reason":"bogus"}`, cnT0), 2,
			`the rejected record's reason "bogus" is none of syntax, type, missing, null`},
		{"syntax with an attr before an early arrival", h + cnRej("S", `{"attr":"k","reason":"syntax"}`, cnBefore), 2,
			"a syntax fault names no attribute"},
		{"type without an attr before an early arrival", h + cnRej("S", `{"reason":"type"}`, cnBefore), 2,
			"a type fault names its attribute"},
	})
}

// TestReaderRefusesAPutRowWithAnUndeclaredColumn: a put row with a column the table does not declare
// (reader.go:398-403, the put side).
func TestReaderRefusesAPutRowWithAnUndeclaredColumn(t *testing.T) {
	t.Parallel()
	cnCheckFaults(t, []cnFaultCase{{"undeclared column", `{"table":"T","op":"put","row":{"k":"a","v":1,"z":2}}`, 1,
		`table "T": the row has a column the table does not declare`}})
}

// TestReaderRefusesAnEventWithAnEmptyBatch: a present but empty batch id is refused (reader.go:485-486).
func TestReaderRefusesAnEventWithAnEmptyBatch(t *testing.T) {
	t.Parallel()
	cnCheckFaults(t, []cnFaultCase{{"empty batch", `{"stream":"S","attrs":{"k":"a","ts":"` + cnT0 + `"},"batch":""}`, 1,
		"an empty batch id"}})
}

// TestReaderChecksAnEventsKeys: every operand of eventRecord's seven-case switch (reader.go:471-487), refused and
// accepted.
func TestReaderChecksAnEventsKeys(t *testing.T) {
	t.Parallel()
	h := cnHeader + "\n"
	sEv := `{"stream":"S","attrs":{"k":"a","ts":"` + cnT0 + `"}`
	aEv := `{"stream":"A","attrs":{"k":"a"}`
	cnCheckFaults(t, []cnFaultCase{
		{"an arrival stream without ts", aEv + `}`, 1, "stream \"A\" is `time arrival`: its events carry ts"},
		{"an attribute stream with ts", sEv + `,"ts":"` + cnT0 + `"}`, 1,
			`ts on stream "S", whose time is its attribute "ts"`},
		{"arrival with no header", sEv + `,"arrival":"` + cnT0 + `"}`, 1, "arrival in a log without a recording header"},
		{"arrival on an arrival stream in a recording", h + aEv + `,"ts":"` + cnT0 + `","arrival":"` + cnT0 + `"}`, 2,
			"arrival on `time arrival` stream \"A\": its ts is the arrival"},
		{"a recording's attribute-time event without arrival", h + sEv + `}`, 2, "an event in a recording carries arrival"},
		{"a present but empty batch", sEv + `,"batch":""}`, 1, "an empty batch id"},
	})
	for _, log := range []string{
		aEv + `,"ts":"` + cnT0 + `"}`,                      // an arrival stream with ts
		sEv + `}`,                                          // an attribute stream without ts
		sEv + `,"batch":"b"}`,                              // a non-empty batch
		h + aEv + `,"ts":"` + cnT0 + `"}`,                  // a recording's arrival-stream event
		h + sEv + `,"arrival":"` + cnT0 + `","batch":"b"}`, // a recording's attribute-time event with arrival
	} {
		recs := cnRecords(t, cnReaderApp(t), log)
		last := recs[len(recs)-1]
		wantBatch := ""
		if strings.Contains(log, `"batch":"b"`) {
			wantBatch = "b"
		}
		if last.Kind != RecEvent || last.Event.BatchID != wantBatch {
			t.Errorf("log %q: last record %+v; want an event with batch %q", log, last, wantBatch)
		}
	}
}

// TestReaderNamesTheFirstFaultOfAnEvent: an event's checks keep their written order (design s1-1c-6 P.2) — one
// two-fault case per adjacent boundary of the stream check, eventKeys, arrivalKeys, the payload and the arrival.
func TestReaderNamesTheFirstFaultOfAnEvent(t *testing.T) {
	t.Parallel()
	h := cnHeader + "\n"
	cnCheckFaults(t, []cnFaultCase{
		{"an unknown stream before no attrs", `{"stream":"Z"}`, 1, `unknown stream "Z"`},
		{"no attrs before an empty batch", `{"stream":"S","batch":""}`, 1, "an event needs attrs"},
		{"no attrs before an arrival stream without ts", `{"stream":"A"}`, 1, "an event needs attrs"},
		{"no ts before arrival with no header", `{"stream":"A","attrs":{"k":"a"},"arrival":"` + cnT0 + `"}`, 1,
			"stream \"A\" is `time arrival`: its events carry ts"},
		{"ts on an attribute stream before arrival with no header",
			`{"stream":"S","attrs":{"k":"a","ts":"` + cnT0 + `"},"ts":"` + cnT0 + `","arrival":"` + cnT0 + `"}`, 1,
			`ts on stream "S", whose time is its attribute "ts"`},
		{"arrival with no header before an empty batch",
			`{"stream":"S","attrs":{"k":"a","ts":"` + cnT0 + `"},"arrival":"` + cnT0 + `","batch":""}`, 1,
			"arrival in a log without a recording header"},
		{"arrival on an arrival stream before an empty batch",
			h + `{"stream":"A","attrs":{"k":"a"},"ts":"` + cnT0 + `","arrival":"` + cnT0 + `","batch":""}`, 2,
			"arrival on `time arrival` stream \"A\": its ts is the arrival"},
		{"no arrival in a recording before an empty batch",
			h + `{"stream":"S","attrs":{"k":"a","ts":"` + cnT0 + `"},"batch":""}`, 2, "an event in a recording carries arrival"},
		{"an empty batch before a payload fault", `{"stream":"A","attrs":{"k":null},"ts":"` + cnT0 + `","batch":""}`, 1,
			"an empty batch id"},
		{"a payload fault before an early arrival",
			h + `{"stream":"A","attrs":{"k":null},"ts":"` + cnBefore + `"}`, 2,
			`the payload fails stream "A"'s schema: attribute "k" (string) at offset 5 is null`},
	})
}
