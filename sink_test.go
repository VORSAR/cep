package cep_test

// Sinks and emitters (design s3-3-1 §5, §5.2; QA's T27, T28, T29 and §9's amendments).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// TestEmitIsSafeForConcurrentUse (T27): 8 goroutines × 1000 Emit inside one Open; 8000 outputs.
func TestEmitIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	emitted := make(chan struct{}) // S3.3-1 build: the test waits for the emits before stopping (MUST 3: a row emitted
	// after Stop began is refused); a fixture fix, named in the record
	src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
		var wg sync.WaitGroup
		var first atomic.Value
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				for i := 0; i < 1000; i++ {
					// S3.3-1 build: one timestamp for every row — interleaved times from 8 goroutines are stream-late on a
					// bounded watermark (SEMANTICS §2), which is the language's rule, not the Emitter's (a fixture fix, named)
					if err := emit.Emit(sctx, "", fcRow(fmt.Sprint(g), int64(i), fcAt(1))); err != nil {
						first.CompareAndSwap(nil, err)
						return
					}
				}
			}(g)
		}
		wg.Wait()
		close(emitted)
		if err, _ := first.Load().(error); err != nil {
			return err
		}
		<-sctx.Done()
		return nil
	})
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind, cep.WithSource("S", src))
	fcStart(t, ctx, h)
	select {
	case <-emitted:
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for the source's emits: %v", ctx.Err())
	}
	rep := fcStop(t, ctx, h)
	if len(rep.SourceErrors) != 0 {
		t.Errorf("SourceErrors %v; want none", rep.SourceErrors)
	}
	if n := len(sink.Drain()); n != 8000 {
		t.Errorf("%d outputs; want 8000", n)
	}
}

// TestEmitRespectsCtx (T27): an expired ctx on SendRow is its error, nothing sent.
func TestEmitRespectsCtx(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{Intake: 1, SinkQueue: 1})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	expired, cancel := context.WithDeadline(ctx, time.Unix(0, 0))
	defer cancel()
	if err := fcInput(t, h, "S").SendRow(expired, "", fcRow("a", 1, fcAt(1))); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("SendRow with an expired ctx: %v; want DeadlineExceeded", err)
	}
	fcStop(t, ctx, h)
	if n := len(sink.Drain()); n != 0 {
		t.Errorf("%d outputs; want 0", n)
	}
}

// TestWriteIsSerialInOrderAndCounted (T28, as §9 amends it): two sinks on one stream; Write is never re-entered, the
// seqs ascend with no gap, every output is written exactly once, the failing binding's errors are counted per binding.
func TestWriteIsSerialInOrderAndCounted(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	const n = 300
	var busy atomic.Bool
	var calls, reentered atomic.Int64
	var mu sync.Mutex
	var seqs []int64
	failing := &fcFuncSink{mode: cepio.Append, write: func(_ context.Context, e cepio.OutEvent) error {
		if !busy.CompareAndSwap(false, true) {
			reentered.Add(1)
		}
		defer busy.Store(false)
		c := calls.Add(1)
		v, _ := e.Row.Int(e.Row.AttrIndex("i"))
		mu.Lock()
		seqs = append(seqs, v)
		mu.Unlock()
		if c%3 == 0 {
			return errors.New("every third write fails")
		}
		return nil
	}}
	good, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind, cep.WithSink("O", failing))
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	for i := 0; i < n; i++ {
		fcSend(t, ctx, in, fcRow("a", int64(i), fcAt(float64(i))))
	}
	fcStop(t, ctx, h)
	if reentered.Load() != 0 {
		t.Errorf("Write re-entered %d times; want 0", reentered.Load())
	}
	if calls.Load() != n {
		t.Errorf("%d Write calls; want %d (each output once, no retry)", calls.Load(), n)
	}
	mu.Lock()
	for i, s := range seqs {
		if s != int64(i) {
			t.Errorf("seq %d at %d; want ascending with no gap", s, i)
			break
		}
	}
	mu.Unlock()
	if m := len(good.Drain()); m != n {
		t.Errorf("the good sink has %d; want %d", m, n)
	}
	st := fcStats(t, ctx, h)
	if s := st.Sinks["O"]; len(s) != 2 || s[0].Errors != 0 || s[1].Errors != n/3 {
		t.Errorf("Sinks[O] %+v; want [{0 …} {%d …}] in WithSink order", s, n/3)
	}
}

// TestCanonicalIsTheRecordingsText (T29): Canonical equals playback.AppendEvent over the output's schema and values.
func TestCanonicalIsTheRecordingsText(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	fcSend(t, ctx, fcInput(t, h, "S"), fcRow("a", 7, fcAt(1)))
	fcStop(t, ctx, h)
	recs := sink.Drain()
	if len(recs) != 1 {
		t.Fatalf("%d outputs; want 1", len(recs))
	}
	sch, err := event.NewSchema("O", []event.Attr{
		{Name: "s", Type: event.TString}, {Name: "i", Type: event.TInt}, {Name: "f", Type: event.TFloat, Optional: true},
		{Name: "o", Type: event.TObject, Optional: true}, {Name: "ts", Type: event.TTimestamp}}, event.ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	e := event.Event{StreamID: 1, Ts: fcAt(1), Values: []event.Value{
		{Kind: event.TString, Str: "a"}, {Kind: event.TInt, I: 7}, {Kind: event.TFloat, Null: true},
		{Kind: event.TObject, Null: true}, {Kind: event.TTimestamp, T: fcAt(1)}}}
	want, err := playback.AppendEvent(nil, sch, e)
	if err != nil {
		t.Fatal(err)
	}
	if string(recs[0].Canonical) != string(want) || recs[0].Stream != "O" || recs[0].Timestamp != fcAt(1) {
		t.Errorf("Canonical %q (stream %s, ts %d); want %q, O, %d", recs[0].Canonical, recs[0].Stream, recs[0].Timestamp, want, fcAt(1))
	}
}

// TestKeptCanonicalSurvivesTheNextWrite (§9, T29; owner's design 4): Canonical is a fresh slice per output.
func TestKeptCanonicalSurvivesTheNextWrite(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	var mu sync.Mutex
	var kept []byte
	var keptText string
	sink := &fcFuncSink{mode: cepio.Append, write: func(_ context.Context, e cepio.OutEvent) error {
		mu.Lock()
		defer mu.Unlock()
		if kept == nil {
			kept, keptText = e.Canonical, string(e.Canonical)
		}
		return nil
	}}
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, cep.WithSink("O", sink))
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	fcSend(t, ctx, in, fcRow("first", 1, fcAt(1)))
	fcSend(t, ctx, in, fcRow("second-and-longer", 2, fcAt(2)))
	fcStop(t, ctx, h)
	mu.Lock()
	defer mu.Unlock()
	if string(kept) != keptText {
		t.Errorf("the kept Canonical changed to %q; want %q unchanged", kept, keptText)
	}
}

// TestInmemorySinkCopiesCanonical (T29): after 1000 outputs every record's Canonical still parses and matches its row.
func TestInmemorySinkCopiesCanonical(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	for i := 0; i < 1000; i++ {
		fcSend(t, ctx, in, fcRow(fmt.Sprint(i), int64(i), fcAt(float64(i))))
	}
	fcStop(t, ctx, h)
	recs := sink.Drain()
	if len(recs) != 1000 {
		t.Fatalf("%d records; want 1000", len(recs))
	}
	for i, rec := range recs {
		var m map[string]any
		if err := json.Unmarshal(rec.Canonical, &m); err != nil {
			t.Fatalf("record %d: Canonical %q does not parse: %v", i, rec.Canonical, err)
		}
		attrs, _ := m["attrs"].(map[string]any) // S3.3-1 build: Canonical is the whole canonical line (ADR-0003/0005), the attrs inside it (a fixture fix, named)
		if attrs["s"] != fmt.Sprint(i) || rec.Values[0] != fmt.Sprint(i) {
			t.Fatalf("record %d: Canonical s %v, Values[0] %v; want %d", i, attrs["s"], rec.Values[0], i)
		}
	}
	_ = inmemory.Record{}
}
