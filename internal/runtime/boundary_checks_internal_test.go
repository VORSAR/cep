package runtime

// The Go boundary's checks, case by case: Send's checks of one event and SendPayload's of one payload, on a built
// runtime (no goroutines), whose intake the test reads; and routeKey's parts. Prefix: bc.

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vorsar/cep/internal/event"
)

// bcAttrs is the source stream's attributes: k (string), o (optional string), f (optional float), ts (an optional
// timestamp, the time attribute unless the stream is `time arrival`).
var bcAttrs = []event.Attr{
	{Name: "k", Type: event.TString}, {Name: "o", Type: event.TString, Optional: true},
	{Name: "f", Type: event.TFloat, Optional: true}, {Name: "ts", Type: event.TTimestamp, Optional: true},
}

// bcBuild builds cgApp with the source stream s holding bcAttrs (time attribute timeAttr), on a runtime whose intake
// holds 4 items; rec, when set, records the app.
func bcBuild(t *testing.T, timeAttr int, rec Recorder) *Runtime {
	t.Helper()
	app := cgApp(t, func(event.Event) error { return nil })
	s, err := event.NewSchema("s", bcAttrs, timeAttr)
	if err != nil {
		t.Fatal(err)
	}
	app.Schemas[0], app.Recorder = s, rec
	r, err := build(Config{Shards: 1, Mailbox: 1, Intake: 4, SinkQueue: 1, RecordQueue: 4, Now: func() int64 {
		return cgT0
	}}, []App{app})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// bcTaken is the item the call queued, or false when it queued none.
func bcTaken(r *Runtime) (item, bool) {
	select {
	case it := <-r.intake:
		return it, true
	default:
		return item{}, false
	}
}

// bcEv is a valid event of stream s at cgT0+i: k, o and f set, ts its time.
func bcEv(i int) event.Event {
	ts := cgT0 + int64(i)
	return event.Event{StreamID: 0, Ts: ts, Values: []event.Value{
		{Kind: event.TString, Str: "k"}, {Kind: event.TString, Str: "o"}, {Kind: event.TFloat, F: 1.5},
		{Kind: event.TTimestamp, T: ts},
	}}
}

// bcSend sends e and checks the outcome: an error holding want (and nothing queued), or, with want "", queued.
func bcSend(t *testing.T, r *Runtime, e event.Event, want string) {
	t.Helper()
	err := r.Send(cgCtx(t), 0, e)
	_, queued := bcTaken(r)
	switch {
	case want == "" && (err != nil || !queued):
		t.Fatalf("Send: %v, queued %v; want nil and queued", err, queued)
	case want != "" && (err == nil || !strings.Contains(err.Error(), want) || queued):
		t.Fatalf("Send: %v, queued %v; want an error holding %q and nothing queued", err, queued, want)
	}
}

func TestSendRefusesEachNonSourceStream(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		stream int
		want   string
	}{
		"negative":     {-1, "is not a source stream"},
		"past the end": {2, "is not a source stream"},
		"derived":      {1, "is not a source stream"},
		"source":       {0, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := bcEv(1)
			e.StreamID = row.stream
			bcSend(t, bcBuild(t, 3, nil), e, row.want)
		})
	}
}

func TestSendRefusesAWrongValueCountOrKind(t *testing.T) {
	t.Parallel()
	r := bcBuild(t, 3, nil)
	short := bcEv(1)
	short.Values = short.Values[:3]
	bcSend(t, r, short, "3 values for 4 attributes")
	kind := bcEv(1)
	kind.Values[1] = event.Value{Kind: event.TInt, I: 1}
	bcSend(t, r, kind, `attribute "o" holds a int value`)
}

// A null value is refused when its attribute is required; allowed when optional.
func TestSendRefusesANullRequiredAttribute(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		attr int
		want string
	}{
		"required": {0, `attribute "k" is null, and it is not optional`},
		"optional": {1, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := bcEv(1)
			e.Values[row.attr].Null = true
			bcSend(t, bcBuild(t, 3, nil), e, row.want)
		})
	}
}

// A null time attribute is refused even when optional (the decoder agrees, event.DecodeNull); another null optional
// attribute is not.
func TestSendRefusesANullOptionalTimeAttribute(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		attr int
		want string
	}{
		"time attribute": {3, `the time attribute "ts" is null`},
		"other":          {2, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := bcEv(1)
			e.Values[row.attr].Null = true
			if row.attr == 3 {
				e.Values[3].T = e.Ts // the Ts check is not what refuses it
			}
			bcSend(t, bcBuild(t, 3, nil), e, row.want)
		})
	}
}

// NaN and the infinities are never values; a null float and a finite one are.
func TestSendRefusesANonFiniteFloat(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		v    event.Value
		want string
	}{
		"NaN":    {event.Value{Kind: event.TFloat, F: math.NaN()}, "NaN and infinities are never values"},
		"+Inf":   {event.Value{Kind: event.TFloat, F: math.Inf(1)}, "NaN and infinities are never values"},
		"-Inf":   {event.Value{Kind: event.TFloat, F: math.Inf(-1)}, "NaN and infinities are never values"},
		"null":   {event.Value{Kind: event.TFloat, F: math.NaN(), Null: true}, ""},
		"finite": {event.Value{Kind: event.TFloat, F: -2.5}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := bcEv(1)
			e.Values[2] = row.v
			bcSend(t, bcBuild(t, 3, nil), e, row.want)
		})
	}
}

func TestSendRefusesAnUnrecordableValue(t *testing.T) {
	t.Parallel()
	e := bcEv(1)
	e.Values[1].Str = "a\xffb"
	bcSend(t, bcBuild(t, 3, nil), e, `attribute "o" holds malformed UTF-8`)
}

// An event-time stream's Ts is its time attribute; a `time arrival` stream's Ts is not checked.
func TestSendRefusesATsThatDiffersFromTheTimeAttribute(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		timeAttr int
		ts       int64
		want     string
	}{
		"arrival-time": {event.ArrivalTime, 7, ""},
		"mismatch":     {3, cgT0, "differs from the time attribute"},
		"match":        {3, cgT0 + 1, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := bcEv(1)
			e.Ts = row.ts
			bcSend(t, bcBuild(t, row.timeAttr, nil), e, row.want)
		})
	}
}

// An event whose recorded line could pass one log line is refused: a string of MaxLogLine/6+1 bytes, each escaped at
// worst to 6, passes it alone.
func TestSendRefusesAnEventOverOneRecordingLine(t *testing.T) {
	t.Parallel()
	e := bcEv(1)
	e.Values[1].Str = strings.Repeat("a", event.MaxLogLine/6+1)
	bcSend(t, bcBuild(t, 3, nil), e, "could not fit one recording line")
}

// bcPayload sends payload on stream and returns the item it queued; it fails when the call errs or queues nothing.
func bcPayload(t *testing.T, r *Runtime, stream int, payload []byte) item {
	t.Helper()
	if err := r.SendPayload(cgCtx(t), 0, stream, payload); err != nil {
		t.Fatalf("SendPayload: %v; want nil", err)
	}
	it, ok := bcTaken(r)
	if !ok {
		t.Fatal("SendPayload queued nothing; want one item")
	}
	return it
}

// bcRejected checks it is a rejection whose fault message holds want.
func bcRejected(t *testing.T, it item, want string) {
	t.Helper()
	if it.kind != iReject || it.fault == nil || !strings.Contains(it.fault.Msg, want) {
		t.Fatalf("the queued item: kind %d fault %+v; want a rejection holding %q", it.kind, it.fault, want)
	}
}

const bcOK = `{"k":"a","ts":"2026-01-01T10:00:00.000Z"}`

func TestSendPayloadRefusesEachNonSourceStream(t *testing.T) {
	t.Parallel()
	for name, stream := range map[string]int{"negative": -1, "past the end": 2, "derived": 1} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := bcBuild(t, 3, nil)
			err := r.SendPayload(cgCtx(t), 0, stream, []byte(bcOK))
			if _, queued := bcTaken(r); err == nil || !strings.Contains(err.Error(), "is not a source stream") || queued {
				t.Fatalf("SendPayload on stream %d: %v, queued %v; want refused, nothing queued", stream, err, queued)
			}
		})
	}
	r := bcBuild(t, 3, nil)
	if it := bcPayload(t, r, 0, []byte(bcOK)); it.kind != iEvent || it.ev.Ts != cgT0 || it.payload != nil {
		t.Fatalf("a valid payload on the source stream: %+v; want an event at cgT0, no payload kept (no Recorder)", it)
	}
}

func TestSendPayloadRejectsAPayloadLongerThanOneLine(t *testing.T) {
	t.Parallel()
	r := bcBuild(t, 3, nil)
	bcRejected(t, bcPayload(t, r, 0, bytes.Repeat([]byte(" "), payloadLimit("s")+1)), "is longer than")
}

func TestSendPayloadRejectsWithTheDecodersFault(t *testing.T) {
	t.Parallel()
	r := bcBuild(t, 3, nil)
	it := bcPayload(t, r, 0, []byte(`{"ts":"2026-01-01T10:00:00.000Z"}`))
	if it.kind != iReject || it.fault == nil || it.fault.Reason != event.DecodeMissing || it.fault.Attr != "k" {
		t.Fatalf("a payload without k: %+v; want the decoder's DecodeMissing of k", it.fault)
	}
}

// bcGrows is a payload within payloadLimit whose cleaned form (each malformed UTF-8 byte a 3-byte U+FFFD) is past it.
func bcGrows(extra string) []byte {
	n := payloadLimit("s") / 2
	return []byte(`{"k":"` + strings.Repeat("\xff", n) + `","ts":"2026-01-01T10:00:00.000Z"` + extra + `}`)
}

// bcDeep is a member nesting an array so the payload's depth (the object counted 1) is d.
func bcDeep(d int) string { return `,"x":` + strings.Repeat("[", d-1) + strings.Repeat("]", d-1) }

func TestSendPayloadRejectsAPayloadThatGrowsPastOneLineWhenCleaned(t *testing.T) {
	t.Parallel()
	p := bcGrows("")
	if len(p) > payloadLimit("s") || event.CleanLen(p) <= payloadLimit("s") {
		t.Fatalf("the fixture: %d bytes, %d cleaned; want within %d raw, past it cleaned", len(p), event.CleanLen(p),
			payloadLimit("s"))
	}
	bcRejected(t, bcPayload(t, bcBuild(t, 3, nil), 0, p), "as a recording keeps it")
}

// Nesting: MaxDepth-1 is the most a recording line holds; one more is rejected.
func TestSendPayloadRejectsAPayloadNestedPastOneLine(t *testing.T) {
	t.Parallel()
	r := bcBuild(t, 3, nil)
	ok := bcPayload(t, r, 0, []byte(`{"k":"a","ts":"2026-01-01T10:00:00.000Z"`+bcDeep(event.MaxDepth-1)+`}`))
	if ok.kind != iEvent {
		t.Fatalf("a payload nested %d deep: %+v; want an event", event.MaxDepth-1, ok.fault)
	}
	bcRejected(t, bcPayload(t, r, 0, []byte(`{"k":"a","ts":"2026-01-01T10:00:00.000Z"`+bcDeep(event.MaxDepth)+`}`)),
		"nested 10000 deep")
}

// Both faults of the recorded form: the cleaned length is checked first, so it is the fault named.
func TestSendPayloadNamesTheCleanedLengthBeforeTheDepth(t *testing.T) {
	t.Parallel()
	bcRejected(t, bcPayload(t, bcBuild(t, 3, nil), 0, bcGrows(bcDeep(event.MaxDepth))), "as a recording keeps it")
}

// With a Recorder the payload is kept, cleaned, as the record's own.
func TestSendPayloadKeepsTheCleanedPayloadForTheRecording(t *testing.T) {
	t.Parallel()
	r := bcBuild(t, 3, igRec{})
	p := `{"k": "a",  "ts":"2026-01-01T10:00:00.000Z"}`
	if it := bcPayload(t, r, 0, []byte(p)); it.kind != iEvent || string(it.payload) != bcOK {
		t.Fatalf("a recorded app's payload: %q; want the cleaned %q", it.payload, bcOK)
	}
}

// The missing operand of routeKey's float-zero normalisation: a non-zero float part is kept as it is.
func TestRouteKeyKeepsANonZeroFloatPart(t *testing.T) {
	t.Parallel()
	values := []event.Value{{Kind: event.TFloat, F: -1.25}, {Kind: event.TFloat, F: 2.5}}
	if k, ok := routeKey([]int{0, 1}, values); !ok || k.First.F != -1.25 || k.Rest[0].F != 2.5 {
		t.Fatalf("routeKey(-1.25, 2.5) = %+v, %v; want the parts kept", k, ok)
	}
}

// routeKey's key-error condition, each operand alone: a null part (of any kind) and a NaN float are key errors; a
// float that is not NaN and a string are parts.
func TestRouteKeyRefusesANullOrNaNPart(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		v  event.Value
		ok bool
	}{
		"null string":  {event.Value{Kind: event.TString, Null: true}, false},
		"null float":   {event.Value{Kind: event.TFloat, F: 1, Null: true}, false},
		"NaN float":    {event.Value{Kind: event.TFloat, F: math.NaN()}, false},
		"finite float": {event.Value{Kind: event.TFloat, F: 3}, true},
		"string":       {event.Value{Kind: event.TString, Str: "s"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, attrs := range [][]int{{0}, {1, 0}} {
				values := []event.Value{row.v, {Kind: event.TString, Str: "x"}}
				if _, ok := routeKey(attrs, values); ok != row.ok {
					t.Fatalf("routeKey(%v) over %+v: ok %v; want %v", attrs, row.v, ok, row.ok)
				}
			}
		})
	}
}

// A null time attribute that is also required is named as not optional: the null-required check comes first.
func TestSendNamesANullRequiredTimeAttributeAsNotOptional(t *testing.T) {
	t.Parallel()
	r, err := build(Config{Shards: 1, Intake: 4, Now: func() int64 { return cgT0 }}, []App{
		cgApp(t, func(event.Event) error { return nil })})
	if err != nil {
		t.Fatal(err)
	}
	e := cgEv(1)
	e.Values[1] = event.Value{Kind: event.TTimestamp, T: e.Ts, Null: true}
	bcSend(t, r, e, `attribute "ts" is null, and it is not optional`)
}

// Each refusal at the Go boundary states its rule and nothing else: no sprint, review or decision reference.
func TestBoundaryRefusalsStateTheirRuleWithNoProcessReference(t *testing.T) {
	t.Parallel()
	bad, long := "a\xffb", strings.Repeat("a", event.MaxLogLine/6+1)
	if utf8.ValidString(bad) {
		t.Fatal("the malformed input is valid UTF-8")
	}
	send := func(edit func(e *event.Event)) error {
		e := bcEv(1)
		edit(&e)
		return bcBuild(t, 3, nil).Send(cgCtx(t), 0, e)
	}
	str := []event.Attr{{Name: "x", Type: event.TString}}
	twice := cgApp(t, func(event.Event) error { return nil })
	twice.Rules[0].Inputs = []int{0, 0}
	u8 := " holds malformed UTF-8, which a recording would replace with U+FFFD"
	for name, c := range map[string]struct {
		err    error
		want   string
		suffix bool
	}{
		"drained":          {ErrDrained, "runtime: the app is drained: its input has ended", false},
		"ts":               {send(func(e *event.Event) { e.Ts = cgT0 }), fmt.Sprintf(`runtime: app 0 stream "s": Ts %d differs from the time attribute "ts", %d: an event-time stream's time is its attribute`, cgT0, cgT0+1), false},
		"line":             {send(func(e *event.Event) { e.Values[1].Str = long }), fmt.Sprintf(`runtime: app 0 stream "s": the event could not fit one recording line of %d bytes`, event.MaxLogLine), false},
		"unrecordable":     {send(func(e *event.Event) { e.Values[1].Str = bad }), `runtime: app 0 stream "s": attribute "o"` + u8, false},
		"required":         {send(func(e *event.Event) { e.Values[0] = event.Value{Kind: event.TString, Null: true} }), `runtime: app 0 stream "s": attribute "k" is null, and it is not optional`, false},
		"NaN":              {send(func(e *event.Event) { e.Values[2] = event.Value{Kind: event.TFloat, F: math.NaN()} }), `runtime: app 0 stream "s": attribute "f" is NaN: NaN and infinities are never values`, false},
		"row float":        {CheckRows("T", []event.Attr{{Name: "x", Type: event.TFloat}}, [][]event.Value{{{Kind: event.TFloat, F: math.Inf(1)}}}), `row 0: attribute 0 "x": the float +Inf is not finite`, false},
		"row unrecordable": {CheckRows("T", str, [][]event.Value{{{Kind: event.TString, Str: bad}}}), `row 0: attribute 0 "x"` + u8, false},
		"row line":         {CheckRows("T", str, [][]event.Value{{{Kind: event.TString, Str: long}}}), fmt.Sprintf("row 0 could not fit one recording line of %d bytes", event.MaxLogLine), false},
		"tick":             {bcBuild(t, 3, nil).Tick(cgCtx(t), math.MaxInt64), "runtime: Tick at 9223372036854775807 ms: outside the years 0000-9999, which a recording cannot write", false},
		"input twice":      {validateApp(0, &twice), ": input 0 is listed twice", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			switch {
			case c.err == nil:
				t.Fatalf("no error; want %q", c.want)
			case c.suffix && !strings.HasSuffix(c.err.Error(), c.want), !c.suffix && c.err.Error() != c.want:
				t.Errorf("error %q; want %q", c.err.Error(), c.want)
			}
		})
	}
}
