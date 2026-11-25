package runtime

import (
	"context"
	goruntime "runtime"
	"strings"
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

// TestARecorderBlockedInItsRecorderEndsTheRecordingAfterTwoTurns (design s1-1c-7-design-starvation; owner D2 of
// S3.2b-2's design; owner, 2026-11-22, the starvation fix under load): at a full queue the router yields at most twice,
// whatever the recorder goroutine is doing, and offers the record again. A goroutine blocked inside its Recorder is
// not runnable and takes nothing, so at the first full queue the router yields exactly twice and the recording ends;
// every write returns, so detection goes on. The witness that the full queue was met is Failed. The bound is the
// owner's, written here, not read from recTurns, so a changed constant turns this red.
func TestARecorderBlockedInItsRecorderEndsTheRecordingAfterTwoTurns(t *testing.T) {
	t.Parallel()
	const queue, turns = 4, 2 // turns: the owner's "at most twice" (2026-11-22)
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
	case <-held.entered: // the goroutine has taken the header and this record and is blocked in Table: the queue is empty
	case <-ctx.Done():
		t.Fatal("the recorder never entered Table: the ceiling")
	}
	writes := 0
	for range 4 * queue { // the queue is empty and holds 4: the fifth write finds it full
		if err := r.TableWrite(ctx, id, 0, TablePut, row); err != nil { // returns: detection goes on
			t.Fatal(err)
		}
		writes++
		st, err := r.Stats(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.Recording[id].Failed {
			break
		}
	}
	if writes != queue+1 {
		t.Fatalf("the recording ended at write %d into a queue of %d behind a blocked Recorder, want write %d",
			writes, queue, queue+1)
	}
	close(held.release)
	if err := r.Close(ctx); err != nil { // nil: every goroutine has returned, the router too, so turns is read safely
		t.Fatalf("Close: %v", err)
	}
	if got := r.appList()[id].rec.turns; got != turns {
		t.Fatalf("the router yielded %d times at a full queue behind a blocked Recorder, want %d", got, turns)
	}
}

// TestATurnThatQueuesTheRecordCountsIt: a record turn queues is counted as put's own are, so the queue-full text that
// ends the recording names the records queued before it. Run on the caller's goroutine with no recorder goroutine: the
// one-slot queue is empty for the first offer and stays full for the second, whose overflow signal has its one slot.
func TestATurnThatQueuesTheRecordCountsIt(t *testing.T) {
	t.Parallel()
	q := newRecQ(igRec{}, 1)
	if !q.turn(recItem{}) {
		t.Fatal("turn at an empty one-slot queue: false; want true")
	}
	if q.queued != 1 || q.turns != 1 {
		t.Fatalf("after turn queued the record: queued %d, turns %d; want 1 and 1", q.queued, q.turns)
	}
	if q.put(recItem{}) {
		t.Fatal("put at a full queue with no recorder goroutine: true; want false")
	}
	if st := q.stats(); !st.Failed || !strings.Contains(st.Err, "after record 1:") {
		t.Fatalf("the recording after a full queue: failed %v, %q; want true and the text naming record 1", st.Failed,
			st.Err)
	}
}
