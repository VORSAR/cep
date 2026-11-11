package playback_test

// The build's own tests for S3.2b-2 (design v2 §15): what QA's tests leave to the review's fixes — the stamp floor
// (F5), a recording that falls behind (F2, owner D2) and the suppressed match's zero allocations (F7, S3.2b's Q4).

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// bdSteps is a Config.Now that hands out the given stamps in order, then repeats the last.
func bdSteps(stamps ...int64) func() int64 {
	var mu sync.Mutex
	i := 0
	return func() int64 {
		mu.Lock()
		defer mu.Unlock()
		v := stamps[min(i, len(stamps)-1)]
		i++
		return v
	}
}

// TestRecordingStampsNeverFallBelowTheStart (review F5; MUST 15): the deployment's start floors every stamp, so a
// tick below it and a wall clock that steps back after Start record no line the reader refuses; the replay of the
// recording gives the live outputs.
func TestRecordingStampsNeverFallBelowTheStart(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-arrival.cep", rcArrivalSrc)
	w := &rcRec{}
	start := ldT0 + 10*ldSec
	live := q2Start(t, p, 1, bdSteps(start, start-ldSec, start-2*ldSec, start+ldSec), playback.NewRecorder(w, p))
	if err := live.r.Tick(live.ctx, start-5*ldSec); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	for _, k := range []string{"a", "b", "c"} {
		if err := live.r.Send(live.ctx, 0, rcR(k, 0)); err != nil {
			t.Fatalf("Send %s: %v", k, err)
		}
	}
	res := live.finish()
	replay := q2Replay(t, p, w.text())
	o := q2Stream(t, p, "O")
	if strings.Join(res.outs[o], "\n") != strings.Join(replay.outs[o], "\n") || len(res.outs[o]) != 3 {
		t.Errorf("live %v\nreplay %v\nrecording:\n%s", res.outs[o], replay.outs[o], w.text())
	}
}

// bdStuck is a writer whose second Write blocks until release is closed.
type bdStuck struct {
	mu      sync.Mutex
	n       int
	release chan struct{}
}

func (w *bdStuck) Write(b []byte) (int, error) {
	w.mu.Lock()
	w.n++
	n := w.n
	w.mu.Unlock()
	if n == 2 {
		<-w.release
	}
	return len(b), nil
}

// TestARecordingThatFallsBehindStops (owner D2 of S3.2b-2's design; review F2; MUST 13): a writer that blocks never
// slows detection — the recorder writes on its own goroutine behind a bounded queue, a full queue ends the recording,
// counted and reported, and every alert still goes out. The drain waits for the stalled recording's queued records
// only until its own deadline (owner, 2026-11-06, decision 1), then succeeds.
func TestARecordingThatFallsBehindStops(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-recorder.cep", rcSrc)
	w := &bdStuck{release: make(chan struct{})}
	var mu sync.Mutex
	outs := 0
	ok := q2Stream(t, p, "OK")
	var sinks []runtime.SinkBinding
	for i, s := range p.Streams {
		if s.Derived {
			sinks = append(sinks, runtime.SinkBinding{Stream: i, Sink: func(e event.Event) error {
				if e.StreamID == ok {
					mu.Lock()
					outs++
					mu.Unlock()
				}
				return nil
			}})
		}
	}
	app := runtime.FromPlan(p, pbTables(t, p), sinks)
	app.Recorder = playback.NewRecorder(w, p)
	r, err := runtime.Start(runtime.Config{Shards: 1, Now: q2NewClock(ldT0, ldSec).now, RecordQueue: 4}, []runtime.App{app})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer func() {
		close(w.release)
		if err := r.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	for i := range 20 {
		if err := r.SendPayload(ctx, 0, 0, []byte(rcS(int64(i)*2*ldMin, "a", 4, 2, ""))); err != nil {
			t.Fatalf("SendPayload %d: %v", i, err)
		}
	}
	dctx, dcancel := context.WithTimeout(ctx, 300*time.Millisecond) // the drain's deadline: the stalled recording holds it no longer
	defer dcancel()
	if _, err := r.Drain(dctx, 0); err != nil {
		t.Fatalf("Drain: %v (a stalled recording holds the drain only until its deadline)", err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := outs
	mu.Unlock()
	if got != 20 {
		t.Errorf("Keyed gave %d alerts; want 20 (detection never waits for the recording)", got)
	}
	if len(st.Recording) != 1 || !st.Recording[0].Failed || !strings.Contains(st.Recording[0].Err, "queue full") {
		t.Errorf("Recording %+v; want Failed with a queue-full message", st.Recording)
	}
}

// TestSuppressedMatchAllocatesNothing (S3.2b's Q4: zero allocations per event through the rate limit; review F7): a
// matching event the rate limit suppresses allocates nothing on the one-thread replay path.
func TestSuppressedMatchAllocatesNothing(t *testing.T) {
	p := pbBuild(t, "bk-keyed.cep", bkKeyedSrc) // partition by k of S ttl 10 min; from S[n > 100]; output first every 5 min
	outs := 0
	s := pbSync(t, p, func(event.Event) { outs++ })
	ts := ldT0
	e := bkS("a", 500, ts) // matches and emits: the anchor is set
	pbSyncEvent(s, &e, ts)
	vals := []event.Value{ldStr("a"), {Kind: event.TInt, I: 500}, {Kind: event.TTimestamp}}
	allocs := testing.AllocsPerRun(500, func() {
		ts++ // 1 ms later: inside the interval, so the output is suppressed
		vals[2].T = ts
		e = event.Event{StreamID: 0, Ts: ts, Values: vals}
		pbSyncEvent(s, &e, ts)
	})
	if allocs != 0 || outs != 1 {
		t.Errorf("%v allocations per suppressed match (want 0); %d outputs (want 1)", allocs, outs)
	}
}
