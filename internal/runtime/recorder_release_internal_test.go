package runtime

// A removed app's recording queue and starting rows are released at Remove (owner, 2026-11-25, PR 2's review PR2A-3
// and PR2Q-10): the queue, sized at Add for the starting rows and kept for the app's life, is dropped by the recorder
// goroutine on its way out, and Remove returns only after it has; the rows the router kept for the activation step
// are dropped in its remove step. The recording's counts stay, so Stats still answers for the removed app. And the
// tables' rows (owner, 2026-11-27, PR3A-2): the remove step empties the app's tables and keeps their declarations.

import (
	"context"
	goruntime "runtime"
	"testing"
	"weak"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// rrRecorded is a recorded app (igRec) with one table of three starting rows, so its queue is sized above the
// configured RecordQueue: the header and the three rows queued before the recorder starts.
func rrRecorded(t *testing.T) App {
	t.Helper()
	app := cgApp(t, func(event.Event) error { return nil })
	app.Recorder = igRec{}
	app.Tables = table.NewSet([]*table.Table{startingRowsTable(t, "T")})
	app.Loads = []TableLoad{{Table: 0, Rows: startingRows(3)}}
	return app
}

// rrRuntime is a one-shard runtime whose recording queues hold 8 records plus the starting rows: every record a test
// here puts (the header, 3 rows, 2 events, the stop and the drain's marker: 8) fits whatever the recorder's pace, so
// no recording ends at a full queue. Its cleanup closes it.
func rrRuntime(t *testing.T) *Runtime {
	t.Helper()
	r, err := New(Config{Shards: 1, SinkQueue: 4, RecordQueue: 8, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ldClose(t, r) })
	return r
}

// rrRemoved adds, activates, sends two events to, drains and removes a recorded app with starting rows, and returns
// its state (taken before Remove: appOf refuses a removed app) and its id. The recording holds the header, the three
// starting rows, the two events and the stop: 7 records.
func rrRemoved(t *testing.T, ctx context.Context, r *Runtime) (*appState, AppID) {
	t.Helper()
	id := ldStarted(t, ctx, r, rrRecorded(t))
	a, err := r.appOf(id)
	if err != nil {
		t.Fatalf("appOf: %v", err)
	}
	for i := range 2 {
		if err := r.Send(ctx, id, cgEv(i)); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	if _, err := r.Drain(ctx, id); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if err := r.Remove(ctx, id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	return a, id
}

// rrStats checks the removed app's recording in Stats: not failed, records given.
func rrStats(t *testing.T, ctx context.Context, r *Runtime, id AppID, records int64) {
	t.Helper()
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats after Remove: %v", err)
	}
	if rs := st.Recording[id]; rs.Failed || rs.Records != records {
		t.Errorf("the removed app's recording in Stats: %+v; want not Failed and %d records", rs, records)
	}
}

// Once Remove has returned, the app's recording queue is gone (the recorder goroutine dropped it on its way out, and
// Remove waited for the goroutine's return, so the read is ordered after the drop), and Stats still counts the
// recording's 7 records (the header, 3 starting rows, 2 events, the stop).
func TestARemovedAppsRecordingQueueIsReleasedOnceItsRecorderReturns(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := rrRuntime(t)
	a, id := rrRemoved(t, ctx, r)
	if a.rec.q != nil {
		t.Errorf("the recording queue after Remove: %d slots kept; want nil (released once the recorder returned)",
			cap(a.rec.q))
	}
	rrStats(t, ctx, r, id, 7)
}

// Once Remove has returned, the router has dropped the app's starting rows (App.Loads), which only its activation
// step read.
func TestARemovedAppsStartingRowsAreReleased(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := rrRuntime(t)
	a, _ := rrRemoved(t, ctx, r)
	if a.app.Loads != nil {
		t.Errorf("the starting rows after Remove: %d loads kept; want nil (dropped in the router's remove step)",
			len(a.app.Loads))
	}
}

// A recorded app removed before it was activated: no recorder goroutine ever ran, so the router's remove step closes
// no stop and Remove waits on nothing; it returns, the remove step itself has released the queue and the rows, and
// Stats answers for the app with the recording not failed and nothing recorded (the queued header is never written:
// design s3-3-1 §2.4, MUST 8).
func TestRemovingANeverActivatedRecordedAppReturnsAndStatsAnswers(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := rrRuntime(t)
	id, err := r.Add(ctx, rrRecorded(t))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	a, err := r.appOf(id)
	if err != nil {
		t.Fatalf("appOf: %v", err)
	}
	if err := r.Remove(ctx, id); err != nil {
		t.Fatalf("Remove of a never-activated app: %v; want nil", err)
	}
	select {
	case <-a.rec.returned:
		t.Error("the recorder's returned is closed; want open (no recorder goroutine ran for a never-activated app)")
	default:
	}
	if a.rec.q != nil {
		t.Errorf("the recording queue after Remove: %d slots kept; want nil (released in the router's remove step)",
			cap(a.rec.q))
	}
	if a.app.Loads != nil {
		t.Errorf("the starting rows after Remove: %d loads kept; want nil (dropped in the router's remove step)",
			len(a.app.Loads))
	}
	rrStats(t, ctx, r, id, 0)
}

// Once Remove has returned, no stored row of the app's tables is reachable, and the tables' declarations stay for
// TableWrite's check, which reads them past the gate. The control: the row is held while the app is live.
func TestARemovedAppsTableRowsAreReleased(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := rrRuntime(t)
	id := ldStarted(t, ctx, r, rrRecorded(t))
	a, err := r.appOf(id)
	if err != nil {
		t.Fatalf("appOf: %v", err)
	}
	wp := trWeakRow(t, a, "k1")
	for i := range 2 {
		if err := r.Send(ctx, id, cgEv(i)); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	goruntime.GC()
	if wp.Value() == nil {
		t.Fatal("the stored row k1 while the app is live: freed; want held (by its table)")
	}
	if _, err := r.Drain(ctx, id); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if err := r.Remove(ctx, id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	goruntime.GC()
	if wp.Value() != nil {
		t.Error("the stored row k1 after Remove: held; want freed (the remove step empties the app's tables)")
	}
	if a.app.Tables == nil || a.app.Tables.Len() != 1 {
		t.Fatalf("the tables after Remove: %v; want the Set kept with its 1 table (TableWrite's check reads it)",
			a.app.Tables)
	}
	if tb := a.app.Tables.Table(0); tb.Name() != "T" || tb.Snapshot().Len() != 0 {
		t.Errorf("table 0 after Remove: %s with %d rows; want T with 0", tb.Name(), tb.Snapshot().Len())
	}
}

// trWeakRow is a weak pointer to the stored row k of a's table 0, read from its current Version. It is taken here, so
// no slot of the test's own frame keeps the row alive.
func trWeakRow(t *testing.T, a *appState, k string) weak.Pointer[event.Value] {
	t.Helper()
	row, ok := a.app.Tables.Table(0).Snapshot().Get([]event.Value{{Kind: event.TString, Str: k}})
	if !ok {
		t.Fatalf("row %s: not stored", k)
	}
	return weak.Make(&row[0])
}
