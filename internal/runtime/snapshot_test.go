package runtime_test

// TestOneSnapshotPerEventUnderAConcurrentWrite (S3.3's exit 4; QA's T38, the deterministic half; design s3-3-1 §9):
// a body reads the event's table snapshot twice around a hold while the test writes the table through the router;
// both reads are equal and are the value at ingest — the write succeeded meanwhile (TableWrite returned nil while the
// body was parked), so a live read would differ.

import (
	"sync/atomic"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// snBody reads whether key k is in table 0 of the event's snapshot, parks, reads again.
type snBody struct {
	partition.TimerFree
	entered chan struct{}
	release chan struct{}
	first   atomic.Bool
	second  atomic.Bool
	ran     atomic.Bool
}

func (b *snBody) NewState(*partition.Instance) partition.OpState { return lcState{} }
func (b *snBody) OnEvent(_ *partition.Instance, e event.Event) {
	if b.ran.Swap(true) {
		return
	}
	snap, _ := e.Tables.(*table.Snapshot)
	has := func() bool {
		if snap == nil {
			return false
		}
		return snap.Table(0).Has(rtStr("k"))
	}
	b.first.Store(has())
	close(b.entered)
	<-b.release
	b.second.Store(has())
}

func TestOneSnapshotPerEventUnderAConcurrentWrite(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	tb, err := table.New("T", []event.Attr{rtA("k", event.TString)}, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	body := &snBody{entered: make(chan struct{}), release: make(chan struct{})}
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("S", 0), rtDerived("O")},
		Schemas: []event.Schema{rtSchema(t, "S", rtA("k", event.TString)), rtSchema(t, "O", rtA("k", event.TString))},
		Tables:  table.NewSet([]*table.Table{tb}),
		Rules: []runtime.Rule{{Name: "Snap", Inputs: []int{0}, KeyTypes: []event.Type{event.TString},
			NewBody: func(runtime.Emit) partition.Body { return body }}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(event.Event) error { return nil }}},
	}
	r := lcNew(t, runtime.Config{Shards: 1})
	id := lcStarted(t, ctx, r, app)
	if err := r.TableWrite(ctx, id, 0, runtime.TablePut, [][]event.Value{{rtStr("k")}}); err != nil {
		t.Fatalf("TableWrite(put k): %v", err)
	}
	rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("k")))
	rtWait(t, ctx, body.entered, "the body to park between its two reads")
	if err := r.TableWrite(ctx, id, 0, runtime.TableDelete, [][]event.Value{{rtStr("k")}}); err != nil {
		t.Fatalf("TableWrite(delete k) while the body is parked: %v; want nil (the router is not the shard)", err)
	}
	close(body.release)
	rtDrain(t, ctx, r, id)
	if !body.first.Load() || !body.second.Load() {
		t.Errorf("reads (%v, %v); want (true, true): one snapshot per event, the value at ingest", body.first.Load(), body.second.Load())
	}
}
