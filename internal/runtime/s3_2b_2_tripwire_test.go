package runtime_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

// twState is a burst body's state: no timers.
type twState struct{ partition.NoTimers }

func (twState) Walk(func(component, key string, state any)) {}

// twBurst emits n outputs on O for each event it sees: a runaway rule.
type twBurst struct {
	partition.TimerFree
	n    int
	emit runtime.Emit
}

func (b *twBurst) NewState(*partition.Instance) partition.OpState { return twState{} }
func (b *twBurst) OnEvent(_ *partition.Instance, e event.Event) {
	for range b.n {
		b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{rtStr("x"), {Kind: event.TTimestamp, T: e.Ts}}})
	}
}

func twApp(t *testing.T, n int, sink runtime.Sink) runtime.App {
	return runtime.App{
		Streams: []clock.StreamSpec{rtSrc("S", 0), rtDerived("O")},
		Schemas: []event.Schema{rtSchema(t, "S", rtA("k", event.TString)), rtSchema(t, "O", rtA("k", event.TString))},
		Rules: []runtime.Rule{{Name: "Burst", Inputs: []int{0}, KeyTypes: []event.Type{event.TString},
			NewBody: func(emit runtime.Emit) partition.Body { return &twBurst{n: n, emit: emit} }}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: sink}},
	}
}

// TestOutboxHighIsOneAdvancesEmissions (ADR-0006 §5: the tripwire fails a single Advance emitting more than the
// ceiling; S3.2b's exit 6; design s3-2b-2 §8): RuleStats.OutboxHigh is the most outputs one rule emitted from one
// message — 65,537 from one event, one over the default ceiling — not their sum over messages.
func TestOutboxHighIsOneAdvancesEmissions(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var got atomic.Int64
	r := rtStart(t, runtime.Config{Shards: 2}, twApp(t, 65537, func(event.Event) error { got.Add(1); return nil }))
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(2), rtStr("b")))
	rtDrain(t, ctx, r, 0)
	st := rtStats(t, ctx, r)
	if hi := st.Rules[0][0].OutboxHigh; hi != 65537 || got.Load() != 2*65537 {
		t.Errorf("OutboxHigh %d, %d outputs; want 65537 (one message's), 131074", hi, got.Load())
	}
}

// TestSinkHighIsTheLongestQueue (design s3-2b-2 §8; review F10: sink-depth needs a source): Stats.SinkHigh is the
// longest a binding's queue has been — a held sink lets it fill to its capacity.
func TestSinkHighIsTheLongestQueue(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	hold, entered := make(chan struct{}), make(chan struct{})
	var once sync.Once
	r := rtStart(t, runtime.Config{Shards: 1, SinkQueue: 4}, twApp(t, 1, func(event.Event) error {
		once.Do(func() { close(entered); <-hold })
		return nil
	}))
	// The key-less rule releases an event once the clock is past it (S3.2a-1's D1): the second event releases the first.
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(2), rtStr("a")))
	rtWait(t, ctx, entered, "the held sink's first call")
	for i := range 8 {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(3+i)), rtStr("a")))
	}
	close(hold)
	rtDrain(t, ctx, r, 0)
	st := rtStats(t, ctx, r)
	if len(st.SinkHigh) != 1 || len(st.SinkHigh[0]) != 1 || st.SinkHigh[0][0] != 4 {
		t.Errorf("SinkHigh %v; want [[4]]: the queue's capacity, filled while its sink was held", st.SinkHigh)
	}
}
