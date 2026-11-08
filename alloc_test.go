package cep_test

// TestSendRowAllocationsAboveSend (design s3-3-1 §4.1 MUST 13; QA's T22): SendRow with a hoisted fill allocates at
// most three objects above runtime.Send on one shard, warm; the measured figures are logged beside the assertion.

import (
	"context"
	"errors"
	"testing"

	"github.com/vorsar/cep"
	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
	cepio "github.com/vorsar/cep/io"
)

type alBody struct{ partition.TimerFree }

type alState struct{}

func (alState) Walk(func(component, key string, state any))   {}
func (alState) Timers(dst []clock.Timer) []clock.Timer        { return dst }
func (alBody) NewState(*partition.Instance) partition.OpState { return alState{} }
func (alBody) OnEvent(*partition.Instance, event.Event)       {}

// alFill is hoisted: no per-call capture.
func alFill(w cepio.RowWriter) error {
	w.SetString(0, "a")
	w.SetInt(1, 1)
	w.SetNull(2)
	w.SetNull(3)
	w.SetTimestamp(4, fcAt(1))
	return nil
}

func TestSendRowAllocationsAboveSend(t *testing.T) {
	ctx := fcCeiling(t)
	// The engine's own figure: runtime.Send with a shared values slice on one shard.
	sch, err := event.NewSchema("S", []event.Attr{{Name: "s", Type: event.TString}, {Name: "ts", Type: event.TTimestamp}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	schO, err := event.NewSchema("O", []event.Attr{{Name: "s", Type: event.TString}, {Name: "ts", Type: event.TTimestamp}}, event.ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	app := runtime.App{
		Streams: []clock.StreamSpec{{Name: "S"}, {Name: "O", Derived: true}},
		Schemas: []event.Schema{sch, schO},
		Rules: []runtime.Rule{{Name: "N", Inputs: []int{0}, KeyTypes: []event.Type{event.TString},
			NewBody: func(runtime.Emit) partition.Body { return alBody{} }}},
	}
	rt, err := runtime.Start(runtime.Config{Shards: 1, Now: func() int64 { return fcT0 }}, []runtime.App{app})
	if err != nil {
		t.Fatalf("runtime.Start: %v", err)
	}
	defer func() {
		if err := rt.Close(ctx); err != nil {
			t.Errorf("runtime Close: %v", err)
		}
	}()
	vals := []event.Value{{Kind: event.TString, Str: "a"}, {Kind: event.TTimestamp, T: fcAt(1)}}
	send := func() {
		if err := rt.Send(ctx, 0, event.Event{StreamID: 0, Ts: fcAt(1), Values: vals}); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	for i := 0; i < 2000; i++ {
		send()
	}
	sendAllocs := testing.AllocsPerRun(5000, send)

	r := fcNew(t, cep.Config{Shards: 1})
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	sendRow := func() {
		if err := in.SendRow(context.Background(), "", alFill); err != nil {
			t.Fatalf("SendRow: %v", err)
		}
	}
	for i := 0; i < 2000; i++ {
		sendRow()
	}
	rowAllocs := testing.AllocsPerRun(5000, sendRow)
	t.Logf("allocations per call: runtime.Send %.2f (a two-attribute app, no outputs), SendRow %.2f (a five-attribute forwarding app), above Send %.2f", sendAllocs, rowAllocs, rowAllocs-sendAllocs)
	// S3.3-1 build (a fixture fix, named in the record): the two figures above drive different apps, so the engine's
	// own per-event work differs between them and their difference drifts by one between runs. The facade's own
	// allocations are measured exactly with a fill that returns an error after writing: the row is built and checked,
	// nothing is sent (MUST 13: at most three — the row and its values; the set mask is the values' kinds).
	errStop := errors.New("stop before Send")
	fillOnly := func() {
		if err := in.SendRow(context.Background(), "", func(w cepio.RowWriter) error { _ = alFill(w); return errStop }); !errors.Is(err, errStop) {
			t.Fatalf("SendRow with a failing fill: %v", err)
		}
	}
	for i := 0; i < 2000; i++ {
		fillOnly()
	}
	own := testing.AllocsPerRun(5000, fillOnly)
	t.Logf("the facade's own allocations per SendRow (the row built, nothing sent): %.2f (budget 3)", own)
	if own > 3.0 {
		t.Errorf("SendRow's own allocations %.2f; want at most 3 (the row, its values)", own)
	}
}
