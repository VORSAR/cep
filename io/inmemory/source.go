// Package inmemory is the programmatic connector (ADR-0008 §4; design s3-3-1 §5.5): a channel-fed source and a
// collecting sink for Go programs and tests. It imports the facade and the contracts only.
package inmemory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	cepio "github.com/vorsar/cep/io"
)

// ErrClosed is Push or EndBatch after Close.
var ErrClosed = errors.New("inmemory: the source is closed")

type item struct {
	batch string
	fill  func(cepio.RowWriter) error // nil: a batch-close signal
}

// Source is a channel-fed source. Push hands one row to the pump; the row's fill runs on the pump's goroutine (the
// one that called Open), so fill must not touch the pusher's state without synchronisation.
type Source struct {
	mu      sync.Mutex // guards closed against a send on the closed channel
	ch      chan item
	closed  bool
	dropped atomic.Int64
	once    sync.Once
}

// NewSource makes a source whose buffer rows may wait for the pump; 0: unbuffered.
func NewSource(buffer int) *Source {
	if buffer < 0 {
		buffer = 0
	}
	return &Source{ch: make(chan item, buffer)}
}

func (s *Source) push(ctx context.Context, it item) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	select {
	case s.ch <- it:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Push blocks until the pump takes the row or ctx ends; ErrClosed after Close.
func (s *Source) Push(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error {
	if fill == nil {
		return errors.New("inmemory: Push with a nil fill")
	}
	return s.push(ctx, item{batch: batchID, fill: fill})
}

// EndBatch is queued like a row, in order.
func (s *Source) EndBatch(ctx context.Context, batchID string) error {
	if batchID == "" {
		return errors.New("inmemory: EndBatch with an empty batch id")
	}
	return s.push(ctx, item{batch: batchID})
}

// Close ends the pushes; Open returns once everything pushed was emitted.
func (s *Source) Close() {
	s.once.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closed = true
		close(s.ch)
	})
}

// Dropped counts rows and signals pushed and never emitted (Open's ctx ended first, or an emit failed).
func (s *Source) Dropped() int64 { return s.dropped.Load() }

// Open pumps into emit until the source is closed and drained (nil) or ctx ends first (ctx.Err(); the rest counted
// in Dropped). An emit error ends Open with it, the rest counted in Dropped.
func (s *Source) Open(ctx context.Context, emit cepio.Emitter) error {
	for {
		select {
		case it, ok := <-s.ch:
			if !ok {
				return nil
			}
			var err error
			if it.fill != nil {
				err = emit.Emit(ctx, it.batch, it.fill)
			} else {
				err = emit.EndBatch(ctx, it.batch)
			}
			if err != nil {
				s.dropped.Add(int64(1 + len(s.ch)))
				return err
			}
		case <-ctx.Done():
			s.dropped.Add(int64(len(s.ch)))
			return ctx.Err()
		}
	}
}
