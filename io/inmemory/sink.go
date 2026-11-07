package inmemory

import (
	"context"

	cepio "github.com/vorsar/cep/io"
)

// Sink collects outputs. Write keeps Canonical (the sink's) and reads Row's values into a Record before returning.
type Sink struct{ _ struct{} }

// Record is one collected output. Values holds string, int64, float64, bool, int64 (ms), int64 (ms), string (JSON
// text) or nil, by the stream's attribute order.
type Record struct {
	Stream    string
	Timestamp int64
	Canonical []byte
	Values    []any
}

// Accepts is Append.
func (s *Sink) Accepts() cepio.Mode { return cepio.Append }

// Write collects e.
func (s *Sink) Write(ctx context.Context, e cepio.OutEvent) error { return errNotBuilt }

// Drain returns a copy of the records; the sink keeps them.
func (s *Sink) Drain() []Record { return nil }

// Reset forgets the records.
func (s *Sink) Reset() {}
