package runtime

import (
	"context"
	goruntime "runtime"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// TestRecorderGetsATurnAtOneCPU (design s1-1c-7-design-starvation): on one P, a caller that feeds the router in a
// tight loop does not end the recording: the router gives the waiting recorder goroutine the processor at a full
// queue. Not parallel: it sets GOMAXPROCS for the whole process.
func TestRecorderGetsATurnAtOneCPU(t *testing.T) {
	defer goruntime.GOMAXPROCS(goruntime.GOMAXPROCS(1))
	const queue, n = 64, 64 * 1024
	row := [][]event.Value{{{Kind: event.TString, Str: "k"}, {Kind: event.TInt, I: 1}}}
	feeds := []struct {
		name  string
		sink  bool
		first bool // one on-time event before the loop, so the loop's events are late
		feed  func(ctx context.Context, r *Runtime, id AppID) error
	}{
		{"TableWrite", false, false, func(ctx context.Context, r *Runtime, id AppID) error {
			return r.TableWrite(ctx, id, 0, TablePut, row)
		}},
		{"BatchEnd", false, false, func(ctx context.Context, r *Runtime, id AppID) error {
			return r.BatchEnd(ctx, id, 0, "b")
		}},
		{"Tick", false, false, func(ctx context.Context, r *Runtime, _ AppID) error { return r.Tick(ctx, cgT0) }},
		{"late Send", true, true, func(ctx context.Context, r *Runtime, id AppID) error {
			return r.Send(ctx, id, cgEv(0))
		}},
	}
	for _, f := range feeds {
		t.Run(f.name, func(t *testing.T) {
			ctx := cgCtx(t)
			r, err := New(Config{Shards: 1, Intake: 1, RecordQueue: queue, Now: func() int64 { return cgT0 }})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = r.Close(ctx) })
			tb, err := table.New("T", []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}},
				[]string{"k"})
			if err != nil {
				t.Fatal(err)
			}
			app := cgApp(t, func(event.Event) error { return nil })
			if !f.sink {
				app.Sinks = nil
			}
			app.Recorder, app.Tables = igRec{}, table.NewSet([]*table.Table{tb})
			id, err := r.Add(ctx, app)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Activate(ctx, id); err != nil {
				t.Fatal(err)
			}
			want := int64(1 + n + 1) // the header, the loop's records, the stop
			if f.first {
				if err := r.Send(ctx, id, cgEv(n)); err != nil {
					t.Fatal(err)
				}
				want++
			}
			for range n {
				if err := f.feed(ctx, r, id); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.Drain(ctx, id); err != nil {
				t.Fatal(err)
			}
			st, err := r.Stats(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if rs := st.Recording[id]; rs.Failed || rs.Records != want {
				t.Fatalf("recording %+v, want %d records and no failure", rs, want)
			}
		})
	}
}

// rtHeld is a Recorder whose first Table call closes entered and whose every Table call waits for release; only the
// recorder goroutine calls it, so in needs no lock.
type rtHeld struct {
	igRec
	entered, release chan struct{}
	in               bool
}

func (h *rtHeld) Table(int, TableOp, [][]event.Value) error {
	if !h.in {
		h.in = true
		close(h.entered)
	}
	<-h.release
	return nil
}

// TestFullQueueWithABusyRecorderGetsNoTurn (design s1-1c-7-design-starvation; owner D2 of S3.2b-2's design): a
// recorder goroutine that is inside its Recorder is behind, not waiting for the processor, so at a full queue the
// router yields nothing and the recording ends at once. The witness that the full queue was met is Failed.
func TestFullQueueWithABusyRecorderGetsNoTurn(t *testing.T) {
	t.Parallel()
	const queue = 4
	ctx := cgCtx(t)
	r, err := New(Config{Shards: 1, RecordQueue: queue, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(ctx) })
	tb, err := table.New("T", []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}}, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	held := &rtHeld{entered: make(chan struct{}), release: make(chan struct{})}
	app := cgApp(t, func(event.Event) error { return nil })
	app.Sinks, app.Recorder, app.Tables = nil, held, table.NewSet([]*table.Table{tb})
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatal(err)
	}
	row := [][]event.Value{{{Kind: event.TString, Str: "k"}, {Kind: event.TInt, I: 1}}}
	if err := r.TableWrite(ctx, id, 0, TablePut, row); err != nil {
		t.Fatal(err)
	}
	select {
	case <-held.entered: // the goroutine is inside its Recorder, having taken a record: it is not waiting
	case <-ctx.Done():
		t.Fatal("the recorder never entered Table: the ceiling")
	}
	failed := false
	for range 4 * queue { // the queue is empty and holds 4: the fifth write finds it full
		if err := r.TableWrite(ctx, id, 0, TablePut, row); err != nil {
			t.Fatal(err)
		}
		st, err := r.Stats(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if failed = st.Recording[id].Failed; failed {
			break
		}
	}
	if !failed {
		t.Fatalf("the recording did not end after %d writes into a queue of %d behind a held Recorder", 4*queue, queue)
	}
	close(held.release)
	if err := r.Close(ctx); err != nil { // nil: every goroutine has returned, the router too, so turns is read safely
		t.Fatalf("Close: %v", err)
	}
	if got := r.appList()[id].rec.turns; got != 0 {
		t.Fatalf("the router yielded %d times at a full queue behind a busy Recorder, want 0", got)
	}
}
