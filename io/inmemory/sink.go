package inmemory

import (
	"context"
	"sync"

	cepio "github.com/vorsar/cep/io"
)

// Sink collects outputs. Write keeps Canonical (the sink's own, a fresh slice per output) and reads Row's values into
// a Record before returning. Safe for concurrent use.
type Sink struct {
	mu   sync.Mutex
	recs []Record
}

// Record is one collected output. Values holds string, int64, float64, bool, int64 (unix ms), int64 (ms), string
// (JSON text) or nil, by the stream's attribute order.
type Record struct {
	Stream    string
	Timestamp int64
	Canonical []byte
	Values    []any
}

// Accepts is Append.
func (s *Sink) Accepts() cepio.Mode { return cepio.Append }

// Write collects e.
func (s *Sink) Write(ctx context.Context, e cepio.OutEvent) error {
	rec := Record{Stream: e.Stream, Timestamp: e.Timestamp, Canonical: e.Canonical, Values: make([]any, e.Row.Len())}
	for i := range rec.Values {
		if e.Row.IsNull(i) {
			continue
		}
		switch e.Row.AttrType(i) {
		case cepio.String:
			rec.Values[i], _ = e.Row.String(i)
		case cepio.Int:
			rec.Values[i], _ = e.Row.Int(i)
		case cepio.Float:
			rec.Values[i], _ = e.Row.Float(i)
		case cepio.Bool:
			rec.Values[i], _ = e.Row.Bool(i)
		case cepio.Timestamp:
			rec.Values[i], _ = e.Row.Timestamp(i)
		case cepio.Duration:
			rec.Values[i], _ = e.Row.Duration(i)
		case cepio.Object:
			rec.Values[i], _ = e.Row.Object(i)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, rec)
	return nil
}

// Drain returns a copy of the records; the sink keeps them.
func (s *Sink) Drain() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, len(s.recs))
	copy(out, s.recs)
	return out
}

// Reset forgets the records.
func (s *Sink) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = nil
}
