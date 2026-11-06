package runtime

import (
	"context"
	"errors"
	"fmt"
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
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	if a.draining.Load() {
		return ErrDrained
	}
	if stream < 0 || stream >= len(a.app.Streams) || a.app.Streams[stream].Derived {
		return fmt.Errorf("runtime: app %d: stream %d is not a source stream", app, stream)
	}
	reject := func(fault *event.DecodeError) error {
		return r.enqueue(ctx, item{kind: iReject, app: app, stream: stream, fault: fault})
	}
	name := a.app.Schemas[stream].Stream()
	if len(payload) > payloadLimit(name) {
		return reject(&event.DecodeError{Reason: event.DecodeSyntax, Msg: fmt.Sprintf("the payload is longer than %d bytes, the most a recording line holds", payloadLimit(name))})
	}
	values, info, err := a.dec[stream].Decode(payload, nil)
	if err != nil {
		var de *event.DecodeError
		if !errors.As(err, &de) {
			de = &event.DecodeError{Reason: event.DecodeSyntax, Msg: err.Error()}
		}
		return reject(de)
	}
	if info.Depth > event.MaxDepth-1 {
		return reject(&event.DecodeError{Reason: event.DecodeSyntax, Msg: fmt.Sprintf("the payload is nested %d deep; a recording line holds %d", info.Depth, event.MaxDepth-1)})
	}
	it := item{kind: iEvent, app: app, ev: event.Event{StreamID: stream, Ts: info.Ts, Values: values}, info: info}
	if a.rec != nil {
		it.payload = []byte(event.CleanObject(payload)) // the payload as sent, the record's own (owner D3; review F6)
	}
	return r.enqueue(ctx, it)
}
