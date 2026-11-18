package runtime

import (
	"context"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/vorsar/cep/internal/event"
)

// The Go boundary (owner D1 of S3.2b-2's design, ≠ R; review F1; design §6, MUST 14): Send and TableWrite refuse, with
// an error to the caller, every value a recording cannot copy exactly, so a recording always replays to the live
// run's alerts. SendPayload's values come from the decoder and need no such check; its payload is held to the
// format's limits instead (review F4), recording or not, so detection never depends on the recording setting.

// recordable says v can be written to a log line and read back as itself: a string of well-formed UTF-8, an object
// as its exact JSON text (event.ExactObject; never bytes), a timestamp RFC 3339 can write. Kind, null and NaN are
// Send's own checks.
func recordable(v event.Value) error {
	if v.Null {
		return nil
	}
	switch v.Kind {
	case event.TString:
		if !utf8.ValidString(v.Str) {
			return errors.New("holds malformed UTF-8, which a recording would replace with U+FFFD")
		}
	case event.TObject:
		text, ok := v.Obj.(string)
		if !ok {
			return fmt.Errorf("is an object given as %T; give its JSON text", v.Obj)
		}
		if !event.ExactObject(text) {
			return errors.New("is not one valid JSON object or array in compact form, nested within the log's limit")
		}
	case event.TTimestamp:
		if !event.TimestampInRange(v.T) {
			return errors.New("is a timestamp outside the years 0000-9999, which RFC 3339 cannot write")
		}
	}
	return nil
}

// lineBound is a cap on e's recorded line: every string at its worst-case escaped length, every other value at the
// longest text of its kind, the names and the envelope. An event above the log-line limit is refused.
func lineBound(sch event.Schema, e event.Event) int {
	n := 256 + len(sch.Stream())
	for i, v := range e.Values {
		n += len(sch.Attr(i).Name) + 4
		switch {
		case v.Kind == event.TString:
			n += 6*len(v.Str) + 2
		case v.Kind == event.TObject:
			if s, ok := v.Obj.(string); ok {
				n += len(s)
			}
		default:
			n += 40
		}
	}
	return n + 6*len(e.BatchID) + 12 // the batch id, escaped at worst (break-it L1)
}

// rowBound is a cap on one table record's line (review C2; break-it B3): {"op":…,"row":{…},"table":…}, every string at
// its worst-case escaped length.
func rowBound(table string, attrs []event.Attr, row []event.Value) int {
	n := 256 + 6*len(table)
	for i, v := range row {
		if i < len(attrs) {
			n += len(attrs[i].Name) + 4
		}
		switch {
		case v.Kind == event.TString:
			n += 6*len(v.Str) + 2
		case v.Kind == event.TObject:
			if s, ok := v.Obj.(string); ok {
				n += len(s)
			}
		default:
			n += 40
		}
	}
	return n
}

// payloadLimit is the longest payload whose recording line fits a log line: the line's limit less the envelope.
func payloadLimit(stream string) int { return event.MaxLogLine - 256 - len(stream) }

// SendPayload hands one source event of app, as a JSON payload, to the intake (design s3-2b-2 §6). The payload is
// decoded on the caller's goroutine with the stream's decoder; a payload that fails its stream's schema, or the
// format's limits, is not an error to the caller: it enters the intake as a rejected item, counted and recorded
// (SEMANTICS §10; owner Q1 of S3.2b-2). It blocks for backpressure and respects ctx; the caller may reuse payload
// once it returns.
func (r *Runtime) SendPayload(ctx context.Context, app AppID, stream int, payload []byte) error {
	a, err := r.gate(app)
	if err != nil {
		return err
	}
	if stream < 0 || stream >= len(a.app.Streams) || a.app.Streams[stream].Derived {
		return fmt.Errorf("runtime: app %d: stream %d is not a source stream", app, stream)
	}
	reject := func(fault *event.DecodeError) error {
		return r.enqueue(ctx, item{kind: iReject, app: app, stream: stream, fault: fault})
	}
	name := a.app.Schemas[stream].Stream()
	if len(payload) > payloadLimit(name) {
		return reject(&event.DecodeError{Reason: event.DecodeSyntax, Msg: fmt.Sprintf(
			"the payload is longer than %d bytes, the most a recording line holds", payloadLimit(name))})
	}
	values, info, err := a.dec[stream].Decode(payload, nil)
	if err != nil {
		return reject(asDecodeError(err))
	}
	if fault := recordedFault(name, payload, info); fault != nil {
		return reject(fault)
	}
	it := item{kind: iEvent, app: app, ev: event.Event{StreamID: stream, Ts: info.Ts, Values: values}, info: info}
	if a.rec != nil {
		it.payload = []byte(event.CleanObject(payload)) // the payload as sent, the record's own (owner D3; review F6)
	}
	return r.enqueue(ctx, it)
}

// asDecodeError is the decoder's fault as a rejected item carries it: every decoder error is an *event.DecodeError
// (defensive conversion otherwise).
func asDecodeError(err error) *event.DecodeError {
	var de *event.DecodeError
	if !errors.As(err, &de) {
		de = &event.DecodeError{Reason: event.DecodeSyntax, Msg: err.Error()}
	}
	return de
}

// recordedFault is a decoded payload's fault in its recorded form, the cleaned length first, then the depth; nil: none.
func recordedFault(name string, payload []byte, info event.Decoded) *event.DecodeError {
	if n := event.CleanLen(payload); n > payloadLimit(name) { // the recorded form: malformed UTF-8 grows (review C1)
		return &event.DecodeError{Reason: event.DecodeSyntax, Msg: fmt.Sprintf(
			"the payload is %d bytes as a recording keeps it; a recording line holds %d", n, payloadLimit(name))}
	}
	if info.Depth > event.MaxDepth-1 {
		return &event.DecodeError{Reason: event.DecodeSyntax, Msg: fmt.Sprintf(
			"the payload is nested %d deep; a recording line holds %d", info.Depth, event.MaxDepth-1)}
	}
	return nil
}

// Send hands one source event of app to the intake; it blocks for backpressure and respects ctx. It checks the event
// first (review F9) and takes e.Values: the caller MUST NOT change them afterwards (review F4).
func (r *Runtime) Send(ctx context.Context, app AppID, e event.Event) error {
	a, err := r.gate(app)
	if err != nil {
		return err
	}
	if err := checkEvent(app, &a.app, e); err != nil {
		return err
	}
	return r.enqueue(ctx, item{kind: iEvent, app: app, ev: e})
}

// checkEvent is Send's check of e, in order: a source stream, one value per attribute, each value (checkValues), an
// event-time stream's Ts equal to its time attribute, and one recording line.
func checkEvent(app AppID, a *App, e event.Event) error {
	if e.StreamID < 0 || e.StreamID >= len(a.Streams) || a.Streams[e.StreamID].Derived {
		return fmt.Errorf("runtime: app %d: stream %d is not a source stream", app, e.StreamID)
	}
	sch := a.Schemas[e.StreamID]
	if len(e.Values) != sch.NumAttrs() {
		return fmt.Errorf("runtime: app %d stream %q: %d values for %d attributes", app, sch.Stream(), len(e.Values),
			sch.NumAttrs())
	}
	if err := checkValues(app, sch, e.Values); err != nil {
		return err
	}
	if ta := sch.TimeAttr(); ta != event.ArrivalTime && e.Ts != e.Values[ta].T {
		return fmt.Errorf(
			"runtime: app %d stream %q: Ts %d differs from the time attribute %q, %d: an event-time stream's time is its "+
				"attribute (S3.2a-1's D2; owner D1 of S3.2b-2's design)",
			app, sch.Stream(), e.Ts, sch.Attr(ta).Name, e.Values[ta].T)
	}
	if lineBound(sch, e) > event.MaxLogLine {
		return fmt.Errorf(
			"runtime: app %d stream %q: the event could not fit one recording line of %d bytes (owner D1 of S3.2b-2's design)",
			app, sch.Stream(), event.MaxLogLine)
	}
	return nil
}

// checkValues checks each value in order: its kind, null and finiteness (checkValue), then that a recording can copy
// it; the first fault is the error.
func checkValues(app AppID, sch event.Schema, values []event.Value) error {
	for i, v := range values {
		at := sch.Attr(i)
		if err := checkValue(app, sch, i, at, v); err != nil {
			return err
		}
		if err := recordable(v); err != nil {
			return fmt.Errorf("runtime: app %d stream %q: attribute %q %v (owner D1 of S3.2b-2's design)", app, sch.Stream(),
				at.Name, err)
		}
	}
	return nil
}

// checkValue is value i's first fault, in order: a kind other than its attribute's, a null in a required attribute, a
// null time attribute, a float that is not finite; nil: none.
func checkValue(app AppID, sch event.Schema, i int, at event.Attr, v event.Value) error {
	switch {
	case v.Kind != at.Type:
		return fmt.Errorf("runtime: app %d stream %q: attribute %q holds a %v value; it is declared %v", app, sch.Stream(),
			at.Name, v.Kind, at.Type)
	case v.Null && !at.Optional:
		return fmt.Errorf("runtime: app %d stream %q: attribute %q is null, and it is not optional (break-it V1)", app,
			sch.Stream(), at.Name)
	case v.Null && i == sch.TimeAttr():
		// the decoder refuses a null time attribute (event.DecodeNull); the Go boundary agrees, so no event enters
		// at timestamp 0 (S3.3-1's break-it S1)
		return fmt.Errorf("runtime: app %d stream %q: the time attribute %q is null", app, sch.Stream(), at.Name)
	case nonFinite(v):
		return fmt.Errorf("runtime: app %d stream %q: attribute %q is %v: NaN and infinities are never values (S3.1-2's D2)",
			app, sch.Stream(), at.Name, v.F)
	}
	return nil
}

// nonFinite says v is a float NaN or infinity, never a value (S3.1-2's D2).
func nonFinite(v event.Value) bool {
	return v.Kind == event.TFloat && !v.Null && (math.IsNaN(v.F) || math.IsInf(v.F, 0))
}

// CheckRows is TableWrite's per-row check, shared with the facade so a deploy's starting rows are refused at Deploy,
// not at Start (S3.3-1's break-it B5): every value a recording can copy exactly (owner D1 of S3.2b-2's design), every
// row within one recording line.
func CheckRows(table string, attrs []event.Attr, rows [][]event.Value) error {
	for ri, row := range rows {
		for ci, v := range row {
			name := fmt.Sprint(ci)
			if ci < len(attrs) {
				name = fmt.Sprintf("%d %q", ci, attrs[ci].Name)
			}
			if v.Kind == event.TFloat && !v.Null && (math.IsNaN(v.F) || math.IsInf(v.F, 0)) {
				return fmt.Errorf("row %d: attribute %s: the float %v is not finite (S3.1-2's D2)", ri, name, v.F)
			}
			if err := recordable(v); err != nil {
				return fmt.Errorf("row %d: attribute %s %v (owner D1 of S3.2b-2's design)", ri, name, err)
			}
		}
		if rowBound(table, attrs, row) > event.MaxLogLine {
			return fmt.Errorf("row %d could not fit one recording line of %d bytes (owner D1 of S3.2b-2's design)", ri,
				event.MaxLogLine)
		}
	}
	return nil
}
