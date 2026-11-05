package playback_test

import (
	"bytes"
	"io"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
)

// TestAppendValueStringOracle is §5.4's string row against its named oracle (encoding/json, HTML escaping off, Go
// 1.27.3; review F11): every single byte 0x00-0xFF (0x80-0xFF alone are malformed UTF-8), and 3000 fixed-seed
// generated strings mixing every table row with malformed UTF-8.
func TestAppendValueStringOracle(t *testing.T) {
	var in []string
	for c := range 256 {
		in = append(in, string([]byte{byte(c)}), "x"+string([]byte{byte(c)})+"y")
	}
	in = append(in, "", "\u2028\u2029", "😀", "\xed\xa0\x80", "a\xffb")
	r := rand.New(rand.NewPCG(32, 1))
	for range 3000 {
		in = append(in, enRandString(r, true))
	}
	for _, s := range in {
		if got, want := enValue(t, enStr(s)), enJSONString(t, s); got != want {
			t.Fatalf("AppendValue(string %q) = %q; want %q (encoding/json, HTML off)", s, got, want)
		}
	}
}

// TestAppendValueStringTable is §5.4's escape table, written out literally.
func TestAppendValueStringTable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", `""`},
		{"abc", `"abc"`},
		{`"`, `"\""`},                  // " escaped \"
		{`\`, `"\\"`},                  // \ escaped \\
		{"\b\f\n\r\t", `"\b\f\n\r\t"`}, // the five short forms
		{"\x00", `"\u0000"`},           // every other byte below 0x20: \u00XX
		{"\x01\x0b\x0e\x1b\x1f", `"\u0001\u000b\u000e\u001b\u001f"`}, // lowercase hex
		{"\x7f", "\"\x7f\""},                       // DEL raw
		{"<>&'/", `"<>&'/"`},                       // HTML escaping off; / raw
		{"\u2028", `"\u2028"`},                     // U+2028 escaped
		{"\u2029", `"\u2029"`},                     // U+2029 escaped
		{"é€😀", `"é€😀"`},                           // every other rune raw, astral included
		{"\u0080\uFEFF", "\"\u0080\uFEFF\""},       // C1 and BOM runes raw
		{"\uFFFD", "\"\uFFFD\""},                   // U+FFFD itself raw
		{"\xff", "\"\uFFFD\""},                     // malformed UTF-8: a raw U+FFFD, not an escape
		{"a\xffb", "\"a\uFFFDb\""},                 // one per bad byte
		{"\xed\xa0\x80", "\"\uFFFD\uFFFD\uFFFD\""}, // an encoded surrogate: three bad bytes
		{"\xe2\x82", "\"\uFFFD\uFFFD\""},           // a truncated sequence: two bad bytes
	}
	for _, c := range cases {
		if got := enValue(t, enStr(c.in)); got != c.want {
			t.Errorf("AppendValue(string %q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// TestAppendValueScalars: int and duration decimal (duration in integer ms, P2), bool true/false.
func TestAppendValueScalars(t *testing.T) {
	cases := []struct {
		v    event.Value
		want string
	}{
		{enInt(0), "0"}, {enInt(-1), "-1"}, {enInt(42), "42"},
		{enInt(math.MaxInt64), "9223372036854775807"}, {enInt(math.MinInt64), "-9223372036854775808"},
		{enBool(true), "true"}, {enBool(false), "false"},
		{enDur(0), "0"}, {enDur(90000), "90000"}, {enDur(-1), "-1"},
		{enDur(math.MinInt64), "-9223372036854775808"},
	}
	for _, c := range cases {
		if got := enValue(t, c.v); got != c.want {
			t.Errorf("AppendValue(%+v) = %q; want %q", c.v, got, c.want)
		}
	}
}

// TestAppendValueFloats: strconv.FormatFloat(f, 'g', -1, 64), -0 kept (§5.4, as model.MarshalCanonical). The
// shortest 'g' form uses an exponent when the decimal exponent is below -4 or at least 6 (strconv's rule), so 1e20
// is "1e+20" and 123456 is "123456"; every output below was checked on the pinned toolchain.
func TestAppendValueFloats(t *testing.T) {
	cases := []struct {
		f    float64
		want string
	}{
		{math.Copysign(0, -1), "-0"},
		{0, "0"},
		{0.75, "0.75"},
		{-1.5, "-1.5"},
		{1e21, "1e+21"},
		{1e20, "1e+20"},
		{123456, "123456"},
		{1e6, "1e+06"},
		{0.0001, "0.0001"},
		{123456789, "1.23456789e+08"},
		{1e-7, "1e-07"},
		{math.SmallestNonzeroFloat64, "5e-324"},
		{math.MaxFloat64, "1.7976931348623157e+308"},
		{-math.MaxFloat64, "-1.7976931348623157e+308"},
	}
	for _, c := range cases {
		if got := enValue(t, enFloat(c.f)); got != c.want {
			t.Errorf("AppendValue(float %v) = %q; want %q", c.f, got, c.want)
		}
	}
}

// TestAppendValueFloatNonFinite: NaN and ±Inf are errors (§5.4).
func TestAppendValueFloatNonFinite(t *testing.T) {
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := playback.AppendValue(nil, enFloat(f)); err == nil {
			t.Errorf("AppendValue(float %v): no error; want one", f)
		}
	}
}

// TestAppendValueTimestamps: RFC 3339 UTC, exactly three fraction digits, the years 0000 and 9999 included.
func TestAppendValueTimestamps(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{0, `"1970-01-01T00:00:00.000Z"`},
		{-1, `"1969-12-31T23:59:59.999Z"`},
		{enMs20261104T10, `"2026-11-04T10:00:00.000Z"`},
		{enMs20261104T08_123, `"2026-11-04T08:00:00.123Z"`},
		{enMs20261104T10_1_5, `"2026-11-04T10:00:01.500Z"`},
		{enMsYear0Start, `"0000-01-01T00:00:00.000Z"`},
		{enMsYear9999End, `"9999-12-31T23:59:59.999Z"`},
	}
	for _, c := range cases {
		if got := enValue(t, enTs(c.ms)); got != c.want {
			t.Errorf("AppendValue(timestamp %d) = %q; want %q", c.ms, got, c.want)
		}
	}
}

// TestAppendValueTimestampOutOfRange: one millisecond before 0000-01-01 (year -1) or after 9999-12-31T23:59:59.999Z
// (year 10000) is an error, as are the int64 extremes.
func TestAppendValueTimestampOutOfRange(t *testing.T) {
	for _, ms := range []int64{enMsYear0Start - 1, enMsYear9999End + 1, math.MinInt64, math.MaxInt64} {
		if b, err := playback.AppendValue(nil, enTs(ms)); err == nil {
			t.Errorf("AppendValue(timestamp %d) = %q, no error; want one", ms, b)
		}
	}
}

// TestAppendValueObject: an object is its stored text (D4: already compact and valid UTF-8), escapes as written.
func TestAppendValueObject(t *testing.T) {
	for _, s := range enObjects {
		if got := enValue(t, enObj(s)); got != s {
			t.Errorf("AppendValue(object %q) = %q; want the stored text", s, got)
		}
	}
}

// TestAppendValueNulls: every type's null is null, whatever its other fields hold (a null's payload fields are
// never read: a stale NaN or out-of-range time under a null is not an error).
func TestAppendValueNulls(t *testing.T) {
	for _, ty := range []event.Type{event.TString, event.TInt, event.TFloat, event.TBool, event.TTimestamp,
		event.TDuration, event.TObject} {
		if got := enValue(t, enNull(ty)); got != "null" {
			t.Errorf("AppendValue(null %v) = %q; want null", ty, got)
		}
	}
	stale := []event.Value{
		{Kind: event.TFloat, Null: true, F: math.NaN()},
		{Kind: event.TTimestamp, Null: true, T: math.MaxInt64},
		{Kind: event.TString, Null: true, Str: "x"},
	}
	for _, v := range stale {
		if got := enValue(t, v); got != "null" {
			t.Errorf("AppendValue(%+v) = %q; want null", v, got)
		}
	}
}

// TestAppendValueAppends: AppendValue appends to dst and keeps what dst holds.
func TestAppendValueAppends(t *testing.T) {
	got, err := playback.AppendValue([]byte("x="), enStr("a\nb"))
	if err != nil || string(got) != `x="a\nb"` {
		t.Fatalf(`AppendValue("x=", "a\nb") = %q, %v; want "x=\"a\\nb\"", nil`, got, err)
	}
}

// enTrades is an event-time stream with attributes declared out of bytewise order, a null-able one included.
func enTrades(t testing.TB) event.Schema {
	return enSchema(t, "Trades", 1,
		enReq("user", event.TString), enReq("ts", event.TTimestamp), enReq("Qty", event.TInt),
		enReq("amount", event.TFloat), enReq("_x", event.TBool), enOpt("é", event.TString), enReq("b2", event.TDuration),
		enOpt("o", event.TObject))
}

func enTradesEvent() event.Event {
	return event.Event{StreamID: 0, Ts: enMs20261104T10, BatchID: "b7", Values: []event.Value{
		enStr("alice"), enTs(enMs20261104T10), enInt(3), enFloat(0.75), enBool(true), enNull(event.TString),
		enDur(90000), enObj(`{"a":[1]}`)}}
}

// TestAppendEventSortedKeys: the envelope's keys and the attributes' are sorted bytewise; every declared attribute
// is present, a null as null; batch is present when set; no ts, since Trades has a time attribute (D2). The order of
// the attribute names, by their bytes: "Qty" (0x51) < "_x" (0x5f) < "amount" (0x61) < "b2" (0x62) < "o" (0x6f) <
// "ts" (0x74) < "user" (0x75) < "é" (0xc3 0xa9).
func TestAppendEventSortedKeys(t *testing.T) {
	want := `{"attrs":{"Qty":3,"_x":true,"amount":0.75,"b2":90000,"o":{"a":[1]},"ts":"2026-11-04T10:00:00.000Z",` +
		`"user":"alice","é":null},"batch":"b7","stream":"Trades"}`
	if got := enEvent(t, enTrades(t), enTradesEvent()); got != want {
		t.Fatalf("AppendEvent:\n got  %s\n want %s", got, want)
	}
}

// TestAppendEventAllNull: every optional attribute null is still written, as null.
func TestAppendEventAllNull(t *testing.T) {
	s := enSchema(t, "N", event.ArrivalTime, enOpt("s", event.TString), enOpt("f", event.TFloat), enOpt("o", event.TObject))
	e := event.Event{Ts: 0, Values: []event.Value{enNull(event.TString), enNull(event.TFloat), enNull(event.TObject)}}
	want := `{"attrs":{"f":null,"o":null,"s":null},"stream":"N","ts":"1970-01-01T00:00:00.000Z"}`
	if got := enEvent(t, s, e); got != want {
		t.Fatalf("AppendEvent:\n got  %s\n want %s", got, want)
	}
}

// TestAppendEventOptionalKeys: batch only when set; ts exactly when the schema has no time attribute (D2), the
// event's Ts as a canonical timestamp; an event-time stream never carries ts, even when e.Ts were to differ.
func TestAppendEventOptionalKeys(t *testing.T) {
	arr := enSchema(t, "A", event.ArrivalTime, enReq("v", event.TInt))
	tr := enTrades(t)
	noBatch := enTradesEvent()
	noBatch.BatchID = ""
	cases := []struct {
		name string
		s    event.Schema
		e    event.Event
		want string
	}{
		{"arrival, no batch", arr, event.Event{Ts: enMs20261104T10_1_5, Values: []event.Value{enInt(1)}},
			`{"attrs":{"v":1},"stream":"A","ts":"2026-11-04T10:00:01.500Z"}`},
		{"arrival, batch", arr, event.Event{Ts: enMs20261104T10, BatchID: "b1", Values: []event.Value{enInt(1)}},
			`{"attrs":{"v":1},"batch":"b1","stream":"A","ts":"2026-11-04T10:00:00.000Z"}`},
		{"event time, no batch", tr, noBatch,
			`{"attrs":{"Qty":3,"_x":true,"amount":0.75,"b2":90000,"o":{"a":[1]},"ts":"2026-11-04T10:00:00.000Z",` +
				`"user":"alice","é":null},"stream":"Trades"}`},
	}
	for _, c := range cases {
		if got := enEvent(t, c.s, c.e); got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
}

// TestAppendEventErrors: a NaN or ±Inf float and a value whose Kind is not its attribute's declared type (a null of
// another type included) are errors naming the attribute; an unencodable time, in an attribute or in ts, is an error.
func TestAppendEventErrors(t *testing.T) {
	tr := enTrades(t)
	set := func(i int, v event.Value) event.Event {
		e := enTradesEvent()
		e.Values = append([]event.Value(nil), e.Values...)
		e.Values[i] = v
		return e
	}
	named := []struct {
		name, attr string
		e          event.Event
	}{
		{"NaN", "amount", set(3, enFloat(math.NaN()))},
		{"+Inf", "amount", set(3, enFloat(math.Inf(1)))},
		{"-Inf", "amount", set(3, enFloat(math.Inf(-1)))},
		{"int under string", "user", set(0, enInt(1))},
		{"string under int", "Qty", set(2, enStr("3"))},
		{"duration under int", "Qty", set(2, enDur(3))},
		{"zero Value (TInvalid) under bool", "_x", set(4, event.Value{})},
		{"int null under string", "é", set(5, enNull(event.TInt))},
	}
	for _, c := range named {
		_, err := playback.AppendEvent(nil, tr, c.e)
		if err == nil || !strings.Contains(err.Error(), c.attr) {
			t.Errorf("%s: AppendEvent error %v; want one naming %q", c.name, err, c.attr)
		}
	}
	if _, err := playback.AppendEvent(nil, tr, set(1, enTs(enMsYear9999End+1))); err == nil {
		t.Errorf("timestamp past 9999 in an attribute: no error; want one")
	}
	arr := enSchema(t, "A", event.ArrivalTime, enReq("v", event.TInt))
	if _, err := playback.AppendEvent(nil, arr, event.Event{Ts: enMsYear0Start - 1, Values: []event.Value{enInt(1)}}); err == nil {
		t.Errorf("ts before year 0000: no error; want one")
	}
}

// TestAppendEventEscapedNames: stream and attribute names are escaped like any string. The three attribute names
// sort the same raw and escaped (q" 0x22 < q\ 0x5c < q U+2028 0xe2, escaped \" < \\ < \u), so this case does not
// depend on which of the two is sorted (gap E2).
func TestAppendEventEscapedNames(t *testing.T) {
	s := enSchema(t, "S\"1", event.ArrivalTime,
		enReq("q\u2028", event.TInt), enReq(`q\`, event.TInt), enReq(`q"`, event.TInt))
	e := event.Event{Ts: 0, Values: []event.Value{enInt(3), enInt(2), enInt(1)}}
	want := `{"attrs":{"q\"":1,"q\\":2,"q\u2028":3},"stream":"S\"1","ts":"1970-01-01T00:00:00.000Z"}`
	if got := enEvent(t, s, e); got != want {
		t.Fatalf("AppendEvent:\n got  %s\n want %s", got, want)
	}
	s2 := enSchema(t, "T", event.ArrivalTime, enReq("a\tb<", event.TString))
	e2 := event.Event{Ts: 0, BatchID: "b\x01", Values: []event.Value{enStr("\x7f")}}
	want2 := "{\"attrs\":{\"a\\tb<\":\"\x7f\"},\"batch\":\"b\\u0001\",\"stream\":\"T\",\"ts\":\"1970-01-01T00:00:00.000Z\"}"
	if got := enEvent(t, s2, e2); got != want2 {
		t.Fatalf("AppendEvent:\n got  %s\n want %s", got, want2)
	}
}

// TestAppendEventAppends: AppendEvent appends to a non-empty dst, keeping its bytes, with no newline.
func TestAppendEventAppends(t *testing.T) {
	arr := enSchema(t, "A", event.ArrivalTime, enReq("v", event.TInt))
	got, err := playback.AppendEvent([]byte("prefix\n"), arr, event.Event{Ts: 0, Values: []event.Value{enInt(1)}})
	want := "prefix\n" + `{"attrs":{"v":1},"stream":"A","ts":"1970-01-01T00:00:00.000Z"}`
	if err != nil || string(got) != want {
		t.Fatalf("AppendEvent = %q, %v; want %q, nil", got, err, want)
	}
}

// TestAppendEventZeroAllocs: a typical event encoded into a reused buffer allocates nothing (ADR-0010 §1).
func TestAppendEventZeroAllocs(t *testing.T) {
	s := enTrades(t)
	e := enTradesEvent()
	buf := make([]byte, 0, 1024)
	var err error
	allocs := testing.AllocsPerRun(200, func() {
		buf, err = playback.AppendEvent(buf[:0], s, e)
	})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if allocs != 0 {
		t.Fatalf("AppendEvent into a reused buffer: %v allocations per run; want 0", allocs)
	}
}

// TestAppendEventRoundTrip is §5.4's round trip: for 400 fixed-seed generated events, as a Reader would produce
// them (valid UTF-8 strings, finite floats, times in 0000-9999, compact objects, nulls on optional attributes),
// AppendEvent's line, read back by playback.NewReader as a one-line fixture, gives equal Values, Ts and BatchID.
func TestAppendEventRoundTrip(t *testing.T) {
	trades := enSchema(t, "Trades", 0, enReq("ts", event.TTimestamp), enOpt("s", event.TString),
		enOpt("i", event.TInt), enOpt("f", event.TFloat), enOpt("b", event.TBool), enOpt("t2", event.TTimestamp),
		enOpt("d", event.TDuration), enOpt("o", event.TObject), enOpt("q\"k\u2028", event.TString))
	clicks := enSchema(t, "Clicks", event.ArrivalTime, enReq("s", event.TString), enOpt("n", event.TInt))
	app := &playback.App{Streams: []playback.Stream{{Schema: trades}, {Schema: clicks}}}
	r := rand.New(rand.NewPCG(54, 1))
	randTime := func() int64 { return enMsYear0Start + r.Int64N(enMsYear9999End-enMsYear0Start+1) }
	floats := []float64{0, math.Copysign(0, -1), 0.75, 1e21, 1e-7, math.SmallestNonzeroFloat64, math.MaxFloat64, -1.5}
	batches := []string{"", "", "b1", "b \"q\"", "é\u2028"}
	for n := range 400 {
		sid := r.IntN(2)
		s := app.Streams[sid].Schema
		var e event.Event
		e.StreamID = sid
		e.BatchID = batches[r.IntN(len(batches))]
		for i := range s.NumAttrs() {
			a := s.Attr(i)
			if a.Optional && r.IntN(4) == 0 {
				e.Values = append(e.Values, enNull(a.Type))
				continue
			}
			var v event.Value
			switch a.Type {
			case event.TString:
				v = enStr(enRandString(r, false))
			case event.TInt:
				v = enInt(int64(r.Uint64()))
			case event.TFloat:
				if r.IntN(2) == 0 {
					v = enFloat(floats[r.IntN(len(floats))])
				} else {
					v = enFloat(math.Float64frombits(r.Uint64()&^(0x7ff<<52) | uint64(r.IntN(0x7ff))<<52))
				}
			case event.TBool:
				v = enBool(r.IntN(2) == 0)
			case event.TTimestamp:
				v = enTs(randTime())
			case event.TDuration:
				v = enDur(int64(r.Uint64()))
			case event.TObject:
				v = enObj(enObjects[r.IntN(len(enObjects))])
			}
			e.Values = append(e.Values, v)
		}
		if ta := s.TimeAttr(); ta != event.ArrivalTime {
			e.Ts = e.Values[ta].T
		} else {
			e.Ts = randTime()
		}
		line := enEvent(t, s, e)
		rd, err := playback.NewReader(bytes.NewReader([]byte(line+"\n")), app)
		if err != nil {
			t.Fatalf("event %d: NewReader: %v", n, err)
		}
		rec, err := rd.Next()
		if err != nil {
			t.Fatalf("event %d: line %s: Next: %v", n, line, err)
		}
		if rec.Kind != playback.RecEvent || rec.Event.StreamID != sid || rec.Event.Ts != e.Ts ||
			rec.Event.BatchID != e.BatchID || len(rec.Event.Values) != len(e.Values) {
			t.Fatalf("event %d: line %s: read back %+v; want kind %d, stream %d, Ts %d, batch %q, %d values",
				n, line, rec, playback.RecEvent, sid, e.Ts, e.BatchID, len(e.Values))
		}
		for i := range e.Values {
			if !enSameValue(rec.Event.Values[i], e.Values[i]) {
				t.Fatalf("event %d: line %s: value %d read back %+v; want %+v", n, line, i, rec.Event.Values[i], e.Values[i])
			}
		}
		if _, err := rd.Next(); err != io.EOF {
			t.Fatalf("event %d: second Next: %v; want io.EOF", n, err)
		}
	}
}
