package cep

import (
	"context"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
	cepio "github.com/vorsar/cep/io"
)

// adapter wraps a cepio.Sink as the engine's sink (design s3-3-1 §5.2): each output's canonical JSON is a fresh
// slice the sink may keep (owner, 2026-11-07, design 4); Row reads the output's values until Write returns; ctx is
// the app's sink context, cancelled at Undeploy or at Close's deadline (§3.6).
func adapter(stream string, sch event.Schema, s cepio.Sink) func(ctx context.Context) runtime.Sink {
	attrs := make([]event.Attr, sch.NumAttrs())
	for i := range attrs {
		attrs[i] = sch.Attr(i)
	}
	ns := names(sch)
	return func(ctx context.Context) runtime.Sink {
		return func(e event.Event) error {
			canon, err := playback.AppendEvent(nil, sch, e)
			if err != nil {
				return err // the encoder refused the output: counted as a sink error (S5.1's forward)
			}
			return s.Write(ctx, cepio.OutEvent{Stream: stream, Timestamp: e.Ts, Row: &reader{attrs: attrs, names: ns, values: e.Values}, Canonical: canon})
		}
	}
}

// reader is the facade's RowReader over an output's values.
type reader struct {
	attrs  []event.Attr
	names  []string
	values []event.Value
}

func (r *reader) get(attr int, kind event.Type) (event.Value, bool) {
	if attr < 0 || attr >= len(r.values) {
		return event.Value{}, false
	}
	v := r.values[attr]
	if v.Null || v.Kind != kind {
		return event.Value{}, false
	}
	return v, true
}

func (r *reader) String(attr int) (string, bool) {
	v, ok := r.get(attr, event.TString)
	return v.Str, ok
}
func (r *reader) Int(attr int) (int64, bool)     { v, ok := r.get(attr, event.TInt); return v.I, ok }
func (r *reader) Float(attr int) (float64, bool) { v, ok := r.get(attr, event.TFloat); return v.F, ok }
func (r *reader) Bool(attr int) (bool, bool)     { v, ok := r.get(attr, event.TBool); return v.B, ok }
func (r *reader) Timestamp(attr int) (int64, bool) {
	v, ok := r.get(attr, event.TTimestamp)
	return v.T, ok
}
func (r *reader) Duration(attr int) (int64, bool) {
	v, ok := r.get(attr, event.TDuration)
	return v.D, ok
}
func (r *reader) Object(attr int) (string, bool) {
	v, ok := r.get(attr, event.TObject)
	if !ok {
		return "", false
	}
	s, isText := v.Obj.(string)
	return s, isText
}
func (r *reader) IsNull(attr int) bool {
	return attr < 0 || attr >= len(r.values) || r.values[attr].Null
}
func (r *reader) AttrIndex(name string) int { return attrIndex(r.names, name) }
func (r *reader) AttrType(attr int) cepio.Type {
	if attr < 0 || attr >= len(r.attrs) {
		return cepio.Invalid
	}
	return cepio.Type(r.attrs[attr].Type)
}
func (r *reader) Len() int { return len(r.values) }
