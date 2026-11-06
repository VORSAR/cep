package runtime_test

// S3.2b-2's QA break-it (design note docs/sprints/s3-2b-2-design.md v2): the runtime's recorder, the Go boundary
// and the per-stream counters under inputs and timings the acceptance tests did not try. Prefix: bk2.

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

// bk2Panicker is a Recorder whose Event panics, as a recorder over a broken io.Writer may (a nil buffer, a closed
// compressor); every other call succeeds.
type bk2Panicker struct{ rrDouble }

func (d *bk2Panicker) Event(*event.Event, int64, []byte) error {
	panic("bk2Panicker: the writer broke")
}

// TestBreakitRecorderPanicLeavesDetectionRunning (owner Q2 of S3.2b-2: "when the recording cannot be written,
// detection goes on"; MUST 9): a live run whose recorder panics on its first event keeps every alert and reports the
// recording failed. Red first in a child process (the panic ended the process); in-process since the fix round, which
// recovers it.
func TestBreakitRecorderPanicLeavesDetectionRunning(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var outs atomic.Int64
	r := rtStart(t, runtime.Config{Shards: 1}, rrApp(t, &bk2Panicker{}, func(event.Event) error { outs.Add(1); return nil }))
	for i := range 3 {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(1+i)), rtStr("a")))
	}
	rtDrain(t, ctx, r, 0)
	rs := rtStats(t, ctx, r).Recording[0]
	if outs.Load() != 3 || !rs.Failed || !strings.Contains(rs.Err, "panicked") {
		t.Errorf("outputs %d, Recording %+v; want 3 outputs and a recording failed by the panic", outs.Load(), rs)
	}
}

// bk2TimerApp: S keyed by k; each key's first event schedules a timer one second on, whose firing emits one output.
func bk2TimerApp(t *testing.T, sink runtime.Sink) runtime.App {
	return runtime.App{
		Streams: []clock.StreamSpec{rtSrc("S", 0), rtDerived("O")},
		Schemas: []event.Schema{rtSchema(t, "S", rtA("k", event.TString)), rtSchema(t, "O", rtA("k", event.TString))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R",
				sched: func(e event.Event, n uint64) []clock.Timer {
					if n > 1 {
						return nil
					}
					return []clock.Timer{{Due: e.Ts + rtSec, Kind: clock.Completion, Ref: n}}
				},
				onTimers: func(b *rtBody, in *partition.Instance, fires []clock.Timer, _ any) {
					for _, f := range fires {
						b.emit(event.Event{StreamID: 1, Ts: f.Due, Values: []event.Value{in.Key.First, {Kind: event.TTimestamp, T: f.Due}}})
					}
				}})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: sink}},
	}
}

// TestBreakitOutboxHighReplaysAcrossShards (MUST 10: "Replaying a recording MUST give the live run's ... RuleStats";
// design §16.1: OutboxHigh is a RuleStats field, "the most outputs one rule emitted from one message on one shard"):
// eight keys' timers fire on one watermark advance. Live on four shards the advance is one message per shard, each
// emitting its own keys' outputs; replay runs one shard, where the same advance is one message emitting all eight. The
// outputs agree, the RuleStats do not: OutboxHigh depends on the shard count, so the property cannot hold for it (and
// the tripwire's ceiling means different things live and in replay). No program body schedules timers before S4.1,
// so this is latent today; the test double shows it.
func TestBreakitOutboxHighReplaysAcrossShards(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var liveOuts atomic.Int64
	r := rtStart(t, runtime.Config{Shards: 4}, bk2TimerApp(t, func(event.Event) error { liveOuts.Add(1); return nil }))
	var evs []event.Event
	for i := range 8 {
		evs = append(evs, rtEv(0, rtAt(1), rtStr(fmt.Sprintf("k%d", i))))
	}
	evs = append(evs, rtEv(0, rtAt(100), rtStr("z"))) // the watermark passes every k's timer
	for _, e := range evs {
		rtSend(t, ctx, r, 0, e)
	}
	rtDrain(t, ctx, r, 0)
	live := rtStats(t, ctx, r).Rules[0][0]
	var replayOuts int64
	s, err := runtime.NewSync(bk2TimerApp(t, func(event.Event) error { return nil }), rtT0, false, func(event.Event) { replayOuts++ })
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		s.Event(&e, rtT0, event.Decoded{})
	}
	s.Drain()
	replay := s.Stats().Rules[0][0]
	if liveOuts.Load() != replayOuts {
		t.Fatalf("outputs: live %d, replay %d", liveOuts.Load(), replayOuts)
	}
	// Owner, 2026-11-06 (S3.2b-2's review and break-it, 2): OutboxHigh is a capacity mark, per worker, left out of the
	// live-equals-replay check; every detection counter must still match. A guard from here on.
	live.OutboxHigh, replay.OutboxHigh = 0, 0
	if live != replay {
		t.Errorf("RuleStats live (4 shards) %+v vs replay (1 shard) %+v: the detection counters must match (MUST 10)", live, replay)
	}
}
