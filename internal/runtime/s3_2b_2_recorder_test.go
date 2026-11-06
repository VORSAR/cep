package runtime_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// rrDouble is a runtime.Recorder that logs each call's kind; it fails its failAt-th call (0: never), blocks every call
// while hold is open, and counts Flush.
type rrDouble struct {
	mu      sync.Mutex
	calls   []string
	failAt  int
	hold    chan struct{}
	flushed int
}

func (d *rrDouble) add(kind string) error {
	if d.hold != nil {
		<-d.hold
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, kind)
	if d.failAt > 0 && len(d.calls) == d.failAt {
		return errors.New("rrDouble: the disk is full")
	}
	return nil
}

func (d *rrDouble) log() []string { d.mu.Lock(); defer d.mu.Unlock(); return slices.Clone(d.calls) }

func (d *rrDouble) Header(int64) error                                { return d.add("header") }
func (d *rrDouble) Event(*event.Event, int64, []byte) error           { return d.add("event") }
func (d *rrDouble) Rejected(int, int64, *event.DecodeError) error     { return d.add("rejected") }
func (d *rrDouble) Tick(int64) error                                  { return d.add("tick") }
func (d *rrDouble) Table(int, runtime.TableOp, [][]event.Value) error { return d.add("table") }
func (d *rrDouble) Stop() error                                       { return d.add("stop") }
func (d *rrDouble) Flush() error                                      { d.mu.Lock(); d.flushed++; d.mu.Unlock(); return nil }

// rrApp is one event-time source S (k string, ts timestamp), a key-less rule emitting each event on O, a table T
// (k string) and the recorder.
func rrApp(t *testing.T, rec runtime.Recorder, sink runtime.Sink) runtime.App {
	t.Helper()
	tb, err := table.New("T", []event.Attr{rtA("k", event.TString)}, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	a := twApp(t, 1, sink)
	a.Tables, a.Recorder = table.NewSet([]*table.Table{tb}), rec
	return a
}

const rrGood = `{"k":"a","ts":"2026-01-01T10:00:01Z"}`

// TestRecorderRecordsTheIntake (design s3-2b-2 §7, MUST 9): the recorder goroutine gets the header, then one call per
// intake item in intake order — an event, a rejected payload, a table write that succeeded, a tick — and the stop at
// the drain; Records counts every line; a drained app's later payloads count in AfterDrain, recorded or not.
func TestRecorderRecordsTheIntake(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	d := &rrDouble{}
	r := rtStart(t, runtime.Config{Shards: 2}, rrApp(t, d, func(event.Event) error { return nil }))
	for _, p := range []string{rrGood, `{"ts":"2026-01-01T10:00:02Z"}`} {
		if err := r.SendPayload(ctx, 0, 0, []byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{rtStr("x")}}); err != nil {
		t.Fatal(err)
	}
	if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{rtInt(1)}}); err == nil {
		t.Error("a wrong-typed row was written; want an error (and no record)")
	}
	if err := r.Tick(ctx, rtAt(100)); err != nil {
		t.Fatal(err)
	}
	rtDrain(t, ctx, r, 0)
	if err := r.SendPayload(ctx, 0, 0, []byte(rrGood)); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("SendPayload after the drain: %v; want ErrDrained", err)
	}
	if want := []string{"header", "event", "rejected", "table", "tick", "stop"}; !slices.Equal(d.log(), want) {
		t.Errorf("calls %v; want %v", d.log(), want)
	}
	st := rtStats(t, ctx, r)
	if rs := st.Recording[0]; rs.Failed || rs.Records != 6 {
		t.Errorf("Recording %+v; want 6 records, not failed", rs)
	}
	if ss := st.Streams[0][0]; ss.Rejected != 1 {
		t.Errorf("Streams %+v; want S's Rejected 1", ss)
	}
}

// TestRecorderErrorEndsTheRecording (owner Q2 of S3.2b-2; MUST 9): the recorder's first error ends the recording —
// it is never called again — and the app goes on.
func TestRecorderErrorEndsTheRecording(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	d := &rrDouble{failAt: 2}
	var outs atomic.Int64
	r := rtStart(t, runtime.Config{Shards: 1}, rrApp(t, d, func(event.Event) error { outs.Add(1); return nil }))
	for i := range 4 {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(1+i)), rtStr("a")))
	}
	rtDrain(t, ctx, r, 0) // every output through its sink when it returns
	if n := outs.Load(); n != 4 {
		t.Errorf("%d outputs; want 4: detection goes on after the recording failed", n)
	}
	if got := d.log(); len(got) != 2 {
		t.Errorf("calls %v; want 2: the header, then the failing event", got)
	}
	if rs := rtStats(t, ctx, r).Recording[0]; !rs.Failed || rs.Records != 1 || !strings.Contains(rs.Err, "disk is full") {
		t.Errorf("Recording %+v; want Failed, Records 1, the error", rs)
	}
}

// TestRecorderQueueFullEndsTheRecording (owner D2 of S3.2b-2's design; MUST 13): a recorder that does not keep up
// never holds the router: a full queue ends the recording, named, and Close still stops its goroutine once the
// recorder returns; Close flushes a recorder that has Flush.
func TestRecorderQueueFullEndsTheRecording(t *testing.T) {
	t.Parallel()
	d := &rrDouble{hold: make(chan struct{})}
	r, err := runtime.Start(runtime.Config{Shards: 1, Now: rtNow, RecordQueue: 2}, []runtime.App{rrApp(t, d, func(event.Event) error { return nil })})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for i := range 6 {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(1+i)), rtStr("a")))
	}
	st := rtStats(t, ctx, r)
	if rs := st.Recording[0]; !rs.Failed || !strings.Contains(rs.Err, "recording queue full at record") {
		t.Errorf("Recording %+v; want Failed: queue full", rs)
	}
	close(d.hold)
	if err := r.Close(ctx); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.flushed != 0 {
		t.Errorf("Flush called %d times after the recording failed; want 0", d.flushed)
	}
	d2 := &rrDouble{}
	r2, err := runtime.Start(runtime.Config{Shards: 1, Now: rtNow}, []runtime.App{rrApp(t, d2, func(event.Event) error { return nil })})
	if err != nil {
		t.Fatal(err)
	}
	rtSend(t, ctx, r2, 0, rtEv(0, rtAt(1), rtStr("a")))
	if err := r2.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if d2.flushed != 1 {
		t.Errorf("Flush called %d times at Close without a drain; want 1", d2.flushed)
	}
}

// TestSyncReplaysARecordingsSteps (design s3-2b-2 §6, §7): the replay loop takes a recording's steps — the header's
// start, events with their decode report, a rejected record, a tick, a table write, the stop — and counts as live
// ingest does; a drained Sync counts what follows in AfterDrain.
func TestSyncReplaysARecordingsSteps(t *testing.T) {
	t.Parallel()
	var outs []event.Event
	app := rrApp(t, nil, nil)
	app.Sinks = nil
	s, err := runtime.NewSync(app, rtAt(0), false, func(e event.Event) { outs = append(outs, e) })
	if err != nil {
		t.Fatal(err)
	}
	s.Begin(rtAt(0))
	e := rtEv(0, rtAt(1), rtStr("a"))
	s.Event(&e, rtAt(1), event.Decoded{Unknown: 2, Reserved: true})
	s.Reject(0, rtAt(2))
	s.Tick(rtAt(3))
	if err := s.TableWrite(0, runtime.TablePut, [][]event.Value{{rtStr("x")}}); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot() == nil {
		t.Error("no table snapshot")
	}
	walked := 0
	s.Walk(func(string, string, any) { walked++ })
	s.Drain()
	s.Reject(0, rtAt(4))
	e2 := rtEv(0, rtAt(5), rtStr("b"))
	s.Event(&e2, rtAt(5), event.Decoded{})
	if err := s.TableWrite(0, runtime.TablePut, [][]event.Value{{rtStr("y")}}); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("TableWrite after the drain: %v; want ErrDrained", err)
	}
	s.End()
	ss := s.Stats().Streams[0][0]
	if ss.Unknown != 2 || ss.FlagOverrides != 1 || ss.Rejected != 1 || ss.AfterDrain != 2 || walked == 0 || len(outs) != 1 {
		t.Errorf("Streams %+v, walked %d, outputs %d; want Unknown 2, FlagOverrides 1, Rejected 1, AfterDrain 2, a walk, 1 output", ss, walked, len(outs))
	}
	_ = clock.StreamSpec{}
}
