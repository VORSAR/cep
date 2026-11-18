package playback

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// cnSchema builds a schema the test needs, failing the test if event.NewSchema refuses it.
func cnSchema(t testing.TB, stream string, timeAttr int, attrs ...event.Attr) event.Schema {
	t.Helper()
	s, err := event.NewSchema(stream, attrs, timeAttr)
	if err != nil {
		t.Fatalf("event.NewSchema(%q): %v", stream, err)
	}
	return s
}

func cnInt(i int64) event.Value { return event.Value{Kind: event.TInt, I: i} }

// TestAppendValueRefusesAValueOfNoType: AppendValue writes null only for a null of a real type (encode.go:32's
// `v.Kind != event.TInvalid` operand); a value of no type, null or not, and a type outside the seven, are refused with
// "a value of type <type> is not encodable" (encode.go:61), dst returned as it was.
func TestAppendValueRefusesAValueOfNoType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		v    event.Value
		want string
	}{
		{"no type", event.Value{Kind: event.TInvalid}, "a value of type invalid is not encodable"},
		{"a null of no type", event.Value{Kind: event.TInvalid, Null: true}, "a value of type invalid is not encodable"},
		{"type 99", event.Value{Kind: event.Type(99)}, "a value of type Type(99) is not encodable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := AppendValue([]byte("x"), c.v)
			if err == nil || err.Error() != c.want {
				t.Fatalf("AppendValue(%+v) error = %v; want %q", c.v, err, c.want)
			}
			if string(got) != "x" {
				t.Errorf("AppendValue(%+v) dst = %q; want %q unchanged", c.v, got, "x")
			}
		})
	}
}

// TestAppendEventRefusesAWrongValueCount: an event whose value count is not its schema's attribute count is refused
// before anything is written (encode.go:137-139).
func TestAppendEventRefusesAWrongValueCount(t *testing.T) {
	t.Parallel()
	s := cnSchema(t, "S", event.ArrivalTime, event.Attr{Name: "a", Type: event.TInt}, event.Attr{Name: "b", Type: event.TInt})
	for _, vals := range [][]event.Value{{cnInt(1)}, {cnInt(1), cnInt(2), cnInt(3)}} {
		got, err := appendEvent([]byte("x"), s, event.Event{Values: vals}, "")
		want := fmt.Sprintf(`playback: stream "S": an event with %d values for 2 attributes`, len(vals))
		if err == nil || err.Error() != want {
			t.Fatalf("appendEvent(%d values) error = %v; want %q", len(vals), err, want)
		}
		if string(got) != "x" {
			t.Errorf("appendEvent(%d values) dst = %q; want %q unchanged", len(vals), got, "x")
		}
	}
}

// TestAppendEventOrdersMoreThan64Attributes: past the 64-slot stack array the order slice comes from make
// (encode.go:142-144); 65 attributes declared in reverse name order are still written with their keys sorted, and the
// one allocation is that slice (design s1-1c-6 P.3: expected 1).
// Not parallel: testing.AllocsPerRun refuses to run in a parallel test.
func TestAppendEventOrdersMoreThan64Attributes(t *testing.T) {
	const n = 65
	attrs := make([]event.Attr, n)
	vals := make([]event.Value, n)
	for i := range n {
		attrs[i] = event.Attr{Name: fmt.Sprintf("x%02d", n-1-i), Type: event.TInt}
		vals[i] = cnInt(int64(i))
	}
	s := cnSchema(t, "S", event.ArrivalTime, attrs...)
	var want strings.Builder
	want.WriteString(`{"attrs":{`)
	for k := range n { // key x<k> is attribute n-1-k, which holds the value n-1-k
		if k > 0 {
			want.WriteByte(',')
		}
		fmt.Fprintf(&want, `"x%02d":%d`, k, n-1-k)
	}
	want.WriteString(`},"stream":"S","ts":"1970-01-01T00:00:00.000Z"}`)
	e := event.Event{Values: vals}
	got, err := appendEvent(nil, s, e, "")
	if err != nil || string(got) != want.String() {
		t.Fatalf("appendEvent(65 attributes) = %q, %v; want %q", got, err, want.String())
	}
	buf := make([]byte, 0, 4096)
	allocs := testing.AllocsPerRun(100, func() {
		buf, _ = appendEvent(buf[:0], s, e, "")
	})
	if allocs != 1 {
		t.Errorf("appendEvent(65 attributes) allocates %v per call; want 1 (the order slice)", allocs)
	}
}

// TestAppendEventLeavesOutTheSkippedAttribute: the operands of encode.go:155's `skip != "" && a.Name == skip` — no
// skip writes every attribute; a skip naming an attribute leaves out exactly that one; a skip naming none of them
// leaves out nothing.
func TestAppendEventLeavesOutTheSkippedAttribute(t *testing.T) {
	t.Parallel()
	s := cnSchema(t, "S", event.ArrivalTime, event.Attr{Name: "a", Type: event.TInt}, event.Attr{Name: "b", Type: event.TInt},
		event.Attr{Name: "c", Type: event.TInt})
	e := event.Event{Values: []event.Value{cnInt(1), cnInt(2), cnInt(3)}}
	cases := []struct{ skip, want string }{
		{"", `{"attrs":{"a":1,"b":2,"c":3},"stream":"S","ts":"1970-01-01T00:00:00.000Z"}`},
		{"b", `{"attrs":{"a":1,"c":3},"stream":"S","ts":"1970-01-01T00:00:00.000Z"}`},
		{"z", `{"attrs":{"a":1,"b":2,"c":3},"stream":"S","ts":"1970-01-01T00:00:00.000Z"}`},
	}
	for _, c := range cases {
		got, err := appendEvent(nil, s, e, c.skip)
		if err != nil || string(got) != c.want {
			t.Errorf("appendEvent(skip %q) = %q, %v; want %q", c.skip, got, err, c.want)
		}
	}
}

// TestAppendEventRefusesAnArrivalTsOutOfRange: on a `time arrival` stream the event's ts is written, so a ts outside
// the years 0000-9999 is refused, on either side (encode.go:181-184).
func TestAppendEventRefusesAnArrivalTsOutOfRange(t *testing.T) {
	t.Parallel()
	s := cnSchema(t, "A", event.ArrivalTime, event.Attr{Name: "v", Type: event.TInt})
	const want = `playback: stream "A": ts: a timestamp outside the years 0000-9999 is not encodable in RFC 3339`
	for _, ts := range []int64{minTimestamp - 1, maxTimestamp + 1} {
		_, err := appendEvent(nil, s, event.Event{Ts: ts, Values: []event.Value{cnInt(1)}}, "")
		if err == nil || err.Error() != want || !errors.Is(err, errTimeRange) {
			t.Errorf("appendEvent(ts %d) = %v; want %q", ts, err, want)
		}
	}
	for _, ts := range []int64{minTimestamp, maxTimestamp} {
		if _, err := appendEvent(nil, s, event.Event{Ts: ts, Values: []event.Value{cnInt(1)}}, ""); err != nil {
			t.Errorf("appendEvent(ts %d) = %v; want nil", ts, err)
		}
	}
}

// TestEveryEnvelopeSchemaIsValid guards three blocks no test can reach (owner G1 decision 3, "defensive: guaranteed
// by <the named contract>"): NewReader's error returns after event.NewSchema on its three constant schemas
// (reader.go:98-100 the envelope, :103-105 the header, :108-110 the rejected record). Contract: each of those
// constant schemas is valid. This test builds the same three schemas and fails if one stops being valid.
func TestEveryEnvelopeSchemaIsValid(t *testing.T) {
	t.Parallel()
	if _, err := event.NewSchema("log", envelopeAttrs[:], event.ArrivalTime); err != nil {
		t.Errorf("the envelope schema (reader.go:97): %v", err)
	}
	if _, err := event.NewSchema("recording", []event.Attr{{Name: "start", Type: event.TTimestamp}},
		event.ArrivalTime); err != nil {
		t.Errorf("the header schema (reader.go:101): %v", err)
	}
	if _, err := event.NewSchema("rejected", []event.Attr{{Name: "attr", Type: event.TString, Optional: true}, {
		Name: "reason", Type: event.TString}}, event.ArrivalTime); err != nil {
		t.Errorf("the rejected record's schema (reader.go:106): %v", err)
	}
}

// TestNewSchemaRefusesAStreamWithNoAttributes guards the redundant operand at reader.go:159
// (`s.Schema.NumAttrs() == 0`, which never decides alone): a schema with a stream name always has attributes, since
// event.NewSchema refuses an empty attribute list (event.go:100), and the zero Schema has neither. Contract: NewSchema
// refuses zero attributes. It fails if that refusal goes.
func TestNewSchemaRefusesAStreamWithNoAttributes(t *testing.T) {
	t.Parallel()
	for _, attrs := range [][]event.Attr{nil, {}} {
		if s, err := event.NewSchema("S", attrs, event.ArrivalTime); err == nil {
			t.Errorf("event.NewSchema(%v attributes) = %+v, nil; want an error", len(attrs), s)
		}
	}
	var zero event.Schema
	if zero.Stream() != "" || zero.NumAttrs() != 0 {
		t.Errorf("the zero schema: stream %q, %d attributes; want \"\" and 0", zero.Stream(), zero.NumAttrs())
	}
}

// cnFailWriter fails every Write with err and counts the calls.
type cnFailWriter struct {
	err    error
	writes int
}

func (w *cnFailWriter) Write([]byte) (int, error) { w.writes++; return 0, w.err }

var errCnWrite = errors.New("the test writer's failure")
