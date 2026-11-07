package inmemory_test

// The programmatic connector (design s3-3-1 §5.5; QA's T31 as §9 amends it), through the facade only (the import law:
// cep and io, never internal/*).

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

const imSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

stream O (k string, ts timestamp);

rule Fwd
  intent 'Forward every row.'
  from S
  select k
  insert into O;
`

var imT0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

func imCeiling(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func imRow(k string, i int) func(cepio.RowWriter) error {
	return func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), k)
		w.SetTimestamp(w.AttrIndex("ts"), imT0+int64(i)*1000)
		return nil
	}
}

// imApp deploys imSrc with src bound to S and a fresh sink on O, started; the runtime closes at the test's end.
func imApp(t testing.TB, ctx context.Context, src *inmemory.Source) (*cep.AppHandle, *inmemory.Sink) {
	t.Helper()
	r, err := cep.New(cep.Config{Now: func() time.Time { return time.UnixMilli(imT0).UTC() }})
	if err != nil {
		t.Fatalf("cep.New: %v", err)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := r.Close(cctx); err != nil {
			t.Errorf("Close at cleanup: %v", err)
		}
	})
	sink := &inmemory.Sink{}
	h, err := r.Deploy(ctx, "im", imSrc, cep.WithSource("S", src), cep.WithSink("O", sink))
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if err := h.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return h, sink
}

// TestSourcePumpsUntilClosed: Push ×3, Close; Stop: 3 records at the sink, Open returned nil (no source error).
func TestSourcePumpsUntilClosed(t *testing.T) {
	t.Parallel()
	ctx := imCeiling(t)
	src := inmemory.NewSource(0)
	h, sink := imApp(t, ctx, src)
	for i := 1; i <= 3; i++ {
		if err := src.Push(ctx, "", imRow("a", i)); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
	}
	src.Close()
	rep, err := h.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(rep.SourceErrors) != 0 {
		t.Errorf("SourceErrors %v; want none (Open returned nil when closed and drained)", rep.SourceErrors)
	}
	if n := len(sink.Drain()); n != 3 {
		t.Errorf("%d records; want 3", n)
	}
	if src.Dropped() != 0 {
		t.Errorf("Dropped %d; want 0", src.Dropped())
	}
}

// TestCloseThenStopDeliversEverything: push 1,000, Close, Stop: the sink saw 1,000, Dropped 0.
func TestCloseThenStopDeliversEverything(t *testing.T) {
	t.Parallel()
	ctx := imCeiling(t)
	src := inmemory.NewSource(16)
	h, sink := imApp(t, ctx, src)
	for i := 0; i < 1000; i++ {
		if err := src.Push(ctx, "", imRow("a", i)); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
	}
	src.Close()
	if _, err := h.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := len(sink.Drain()); n != 1000 || src.Dropped() != 0 {
		t.Errorf("%d records, Dropped %d; want 1000 and 0", n, src.Dropped())
	}
}

// TestStopWithoutCloseCounts: a Stop while rows are still buffered: Dropped + the sink's count == 1000.
func TestStopWithoutCloseCounts(t *testing.T) {
	t.Parallel()
	ctx := imCeiling(t)
	src := inmemory.NewSource(1000)
	h, sink := imApp(t, ctx, src)
	for i := 0; i < 1000; i++ {
		if err := src.Push(ctx, "", imRow("a", i)); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
	}
	if _, err := h.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := len(sink.Drain()); int64(n)+src.Dropped() != 1000 {
		t.Errorf("%d records + Dropped %d; want 1000", n, src.Dropped())
	}
}

// TestPushAfterCloseIsAnError: Push and EndBatch after Close are ErrClosed.
func TestPushAfterCloseIsAnError(t *testing.T) {
	t.Parallel()
	ctx := imCeiling(t)
	src := inmemory.NewSource(1)
	src.Close()
	if err := src.Push(ctx, "", imRow("a", 1)); !errors.Is(err, inmemory.ErrClosed) {
		t.Errorf("Push after Close: %v; want ErrClosed", err)
	}
	if err := src.EndBatch(ctx, "b"); !errors.Is(err, inmemory.ErrClosed) {
		t.Errorf("EndBatch after Close: %v; want ErrClosed", err)
	}
	src.Close() // twice: no panic
}

// TestEndBatchIsQueuedInOrder: EndBatch is queued like a row; the stream counts it.
func TestEndBatchIsQueuedInOrder(t *testing.T) {
	t.Parallel()
	ctx := imCeiling(t)
	src := inmemory.NewSource(0)
	h, sink := imApp(t, ctx, src)
	if err := src.Push(ctx, "b1", imRow("a", 1)); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if err := src.EndBatch(ctx, "b1"); err != nil {
		t.Fatalf("EndBatch: %v", err)
	}
	src.Close()
	if _, err := h.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, err := h.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Streams["S"].Batches != 1 || len(sink.Drain()) != 1 {
		t.Errorf("Batches %d, %d records; want 1 and 1", st.Streams["S"].Batches, len(sink.Drain()))
	}
}

// TestSinkCollectsCopies: Drain returns a copy (mutating it leaves the next Drain unchanged); concurrent Drain is
// safe under -race; Reset forgets.
func TestSinkCollectsCopies(t *testing.T) {
	t.Parallel()
	ctx := imCeiling(t)
	src := inmemory.NewSource(0)
	h, sink := imApp(t, ctx, src)
	for i := 1; i <= 3; i++ {
		if err := src.Push(ctx, "", imRow("a", i)); err != nil {
			t.Fatalf("Push: %v", err)
		}
	}
	src.Close()
	if _, err := h.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	first := sink.Drain()
	if len(first) != 3 {
		t.Fatalf("%d records; want 3", len(first))
	}
	first[0].Stream = "mutated"
	first[0].Canonical[0] = 'X'
	first[0].Values[0] = "mutated"
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = sink.Drain() }()
	}
	wg.Wait()
	second := sink.Drain()
	if second[0].Stream != "O" || second[0].Canonical[0] != '{' || second[0].Values[0] != "a" {
		t.Errorf("the second Drain saw the mutation: %+v", second[0])
	}
	sink.Reset()
	if n := len(sink.Drain()); n != 0 {
		t.Errorf("%d records after Reset; want 0", n)
	}
}

// TestSinkAcceptsAppend: the sink's mode is Append.
func TestSinkAcceptsAppend(t *testing.T) {
	t.Parallel()
	if m := (&inmemory.Sink{}).Accepts(); m != cepio.Append {
		t.Errorf("Accepts() %v; want Append", m)
	}
	_ = os.Getpid
}
