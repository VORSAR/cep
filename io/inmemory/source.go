// Package inmemory is the programmatic connector (ADR-0008 §4; design s3-3-1 §5.5): a channel-fed source and a
// collecting sink for Go programs and tests. Stubs until the build's step 5.
package inmemory

import (
	"context"
	"errors"

	cepio "github.com/vorsar/cep/io"
)

// ErrClosed is Push or EndBatch after Close.
var ErrClosed = errors.New("inmemory: the source is closed")

var errNotBuilt = errors.New("inmemory: not built yet (S3.3-1)")

// Source is a channel-fed source. Push hands one row to the pump; the row's fill runs on the pump's goroutine (the
// one that called Open), so fill must not touch the pusher's state without synchronisation.
type Source struct{ _ struct{} }

// NewSource makes a source whose buffer rows may wait for the pump; 0: unbuffered.
func NewSource(buffer int) *Source { return &Source{} }

// Push blocks until the pump takes the row or ctx ends; ErrClosed after Close.
func (s *Source) Push(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error {
	return errNotBuilt
}

// EndBatch is queued like a row, in order.
func (s *Source) EndBatch(ctx context.Context, batchID string) error { return errNotBuilt }

// Close ends the pushes; Open returns once everything pushed was emitted.
func (s *Source) Close() {}

// Dropped counts rows pushed and never emitted (Open's ctx ended first).
func (s *Source) Dropped() int64 { return 0 }

// Open pumps into emit until the source is closed and drained (nil) or ctx ends first (ctx.Err(); the rest counted
// in Dropped).
func (s *Source) Open(ctx context.Context, emit cepio.Emitter) error { return errNotBuilt }
