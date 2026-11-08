package inmemory_test

// QA's break-it on S3.3-1's combined build (step 7): the channel-fed source's Close and Open.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// bkNopEmitter accepts every row and batch signal without running the fill.
type bkNopEmitter struct{}

func (bkNopEmitter) Emit(context.Context, string, func(cepio.RowWriter) error) error { return nil }
func (bkNopEmitter) EndBatch(context.Context, string) error                          { return nil }

// bkParkCtx is a context that never ends and signals parked the first time Done is read: Source.push reads Done in
// its select, after it took the source's mutex, so parked means the pusher holds the mutex and is waiting to send.
type bkParkCtx struct {
	context.Context
	once   sync.Once
	parked chan struct{}
}

func (c *bkParkCtx) Done() <-chan struct{} {
	c.once.Do(func() { close(c.parked) })
	return nil
}

// TestBreakitCloseUnblocksAParkedPush (Source.Close's doc: "Before Open it returns at once"; Push's doc: "ErrClosed
// after Close"): a Push parked on an unbuffered source with no pump, then Close; Close returns at once and the Push
// returns ErrClosed. Today push holds s.mu while it waits to send (source.go:48-56), and Close takes s.mu
// (source.go:83), so Close waits until a pump takes the row: with no Open, forever.
func TestBreakitCloseUnblocksAParkedPush(t *testing.T) {
	t.Parallel()
	src := inmemory.NewSource(0)
	pctx := &bkParkCtx{Context: context.Background(), parked: make(chan struct{})}
	pushed := make(chan error, 1)
	go func() { pushed <- src.Push(pctx, "", imRow("a", 1)) }()
	select {
	case <-pctx.parked:
	case <-time.After(10 * time.Second):
		t.Fatal("the Push never reached its send (state: the pusher waiting on the channel)")
	}
	closed := make(chan struct{})
	go func() { src.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Errorf("Close with a Push parked and no Open did not return in 5 s; want it to return at once and the Push to get ErrClosed")
		// Release both: a pump takes the parked row, Close then closes the channel and the pump returns.
		go func() { _ = src.Open(context.Background(), bkNopEmitter{}) }()
		<-closed
	}
	if err := <-pushed; err != nil && !errors.Is(err, inmemory.ErrClosed) {
		t.Errorf("the parked Push returned %v; want ErrClosed (or nil had a pump taken it)", err)
	}
}

// TestBreakitOpenTwice: a Source given to a second Deploy (a restart is a new Deploy, owner design 1) is opened a
// second time. Today Open's `defer close(s.done)` (source.go:101) closes the channel twice: the second Open to
// return panics, and on the engine's run goroutine nothing recovers it, so the process dies. Want: the second Open
// returns (an error or nil), it does not panic.
func TestBreakitOpenTwice(t *testing.T) {
	t.Parallel()
	src := inmemory.NewSource(1)
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := src.Open(cctx, bkNopEmitter{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("first Open with an ended ctx: %v; want context.Canceled", err)
	}
	var p any
	func() {
		defer func() { p = recover() }()
		_ = src.Open(cctx, bkNopEmitter{})
	}()
	if p != nil {
		t.Errorf("the second Open panicked: %v; want it to return (an error naming the reuse, or nil)", p)
	}
	src.Close()
}
