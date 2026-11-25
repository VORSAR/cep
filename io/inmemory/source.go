// Package inmemory is the programmatic connector: a channel-fed source and a collecting sink for Go programs and
// tests. It imports the facade and the contracts only.
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

// ErrReopened is a second Open of one Source: one Source feeds one Open; a new Deploy takes a new Source.
var ErrReopened = errors.New("inmemory: the source was already opened: make a new Source for a new Deploy")

type item struct {
	batch string
	fill  func(cepio.RowWriter) error // nil: a batch-close signal
}

// Source is a channel-fed source for one Open (a redeploy takes a new Source). Push hands one row to the pump; the
// row's fill runs on the pump's goroutine (the one that called Open), so fill must not touch the pusher's state
// without synchronisation. The channel is never closed: Close signals quit, so no send can panic and no Push holds
// Close.
type Source struct {
	ch      chan item
	quit    chan struct{} // closed by Close
	once    sync.Once
	opened  atomic.Bool   // Open was called (once: break-it B4)
	done    chan struct{} // closed when Open returns
	dropped atomic.Int64
}

// NewSource makes a source whose buffer rows may wait for the pump; 0 or less: unbuffered.
func NewSource(buffer int) *Source {
	return &Source{ch: make(chan item, max(buffer, 0)), quit: make(chan struct{}), done: make(chan struct{})}
}

func (s *Source) push(ctx context.Context, it item) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select { // closed first: a Push after Close never races its row into the channel
	case <-s.quit:
		return ErrClosed
	default:
	}
	select {
	case s.ch <- it:
		return nil
	case <-s.quit:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Push blocks until the pump takes the row, Close is called or ctx ends; ErrClosed after Close.
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

// Close ends the pushes (a parked Push returns ErrClosed) and, once Open is running, returns only when the pump has
// emitted everything pushed before it or Open has returned — so "Close, then Stop" delivers every row. Before Open
// it returns at once (Open then drains what is buffered and returns nil).
func (s *Source) Close() {
	s.once.Do(func() { close(s.quit) })
	if s.opened.Load() {
		<-s.done
	}
}

// Dropped counts rows and signals pushed and never emitted (Open's ctx ended first, or an emit failed).
func (s *Source) Dropped() int64 { return s.dropped.Load() }

// Open pumps into emit until the source is closed and drained (nil) or ctx ends first (ctx.Err(); the rest counted
// in Dropped). An emit error ends Open with it, the rest counted in Dropped. A second Open returns ErrReopened.
func (s *Source) Open(ctx context.Context, emit cepio.Emitter) error {
	if !s.opened.CompareAndSwap(false, true) {
		return ErrReopened
	}
	defer close(s.done)
	for {
		select {
		case it := <-s.ch:
			if err := s.emitItem(ctx, emit, it); err != nil {
				return err
			}
		case <-s.quit: // closed: emit what is buffered, then end
			return s.drainBuffered(ctx, emit)
		case <-ctx.Done():
			s.dropped.Add(int64(len(s.ch)))
			return ctx.Err()
		}
	}
}

// drainBuffered emits what is buffered once the source is closed, then returns nil; an emit error ends it.
func (s *Source) drainBuffered(ctx context.Context, emit cepio.Emitter) error {
	for {
		select {
		case it := <-s.ch:
			if err := s.emitItem(ctx, emit, it); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

// emitItem hands one item to emit, a row or a batch end; an error counts it and everything still buffered in Dropped.
func (s *Source) emitItem(ctx context.Context, emit cepio.Emitter, it item) error {
	var err error
	if it.fill != nil {
		err = emit.Emit(ctx, it.batch, it.fill)
	} else {
		err = emit.EndBatch(ctx, it.batch)
	}
	if err != nil {
		s.dropped.Add(int64(1 + len(s.ch)))
	}
	return err
}
