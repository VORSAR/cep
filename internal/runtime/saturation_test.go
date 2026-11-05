package runtime_test

// Exit 3 of S3.2a (design §3, §8): progress with a full mailbox at a shard whose handoff is pending, plus a slow sink
// and a bursty source. The coordinator runs it 50 times under -race with a CPU hog.

import (
	"fmt"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

// TestSaturationProgress.
//
// The world: G = 4 source streams s0..s3 (Delay 0, no idleness) and one derived stream "out" (id G). One rule
// "burst" reads all four streams, keyed by attribute 0 (a string), TTL 24 h (no purge: every event lies within
// 210 s). Its body emits Burst = 8 outputs on "out" for each released event: Values [key, stream, seq, j], j = 0..7.
// One sink binding on "out" records each output under its key; its FIRST call blocks until the test closes
// `release` (the slow sink: a hold on a channel the test controls, never a sleep).
//
// The config is the narrowest the design allows: Shards 2, Mailbox 1, Intake 1, SinkQueue 1; the shared handoff
// channel then holds Shards = 2 outboxes (design §3). So:
//   - the router's FIFO head is a sink-queue send that cannot complete while the sink is held, and every shard
//     keeps producing outboxes from the messages already in its mailbox: the router must keep receiving handoffs
//     while blocked on that send (design §3, step 1);
//   - once the sink runs, each event becomes a message to BOTH shards (its own stream's advance goes to every shard,
//     design §4 step 4), each message releases a burst, and a shard's mailbox (capacity 1) is full whenever the
//     router has queued one more message while the shard is still handing off: a naive router (a blocking mailbox
//     send) and a naive shard (a blocking handoff send) deadlock here once the handoff channel holds 2 outboxes.
//     The test cannot observe that moment through the API; it asserts that the run completes, under a ceiling.
//
// Phase 1 (one goroutine, in order): round 1 sends ts = 0 s on s0..s3 with keys w0..w3; round 2 sends ts = 1 s on
// s0..s3 with the same keys. The rule's clock is min over its four inputs' contributions (Delay 0: each stream's
// max ts), so it stays at 0 s until round 2's s3 event, then moves to 1 s and releases round 1's four events
// (ts 0 s < 1 s): 32 outputs, and the sink's first call holds. The test waits for that call (state: the sink is
// held), so from here the router's FIFO head is blocked on the sink queue.
// Phase 2 (bursty): G goroutines, goroutine g sends M = 200 events on stream g, ts = 2 s + i s, key s<g>-k<i mod 5>.
// One sender per stream keeps each stream monotone, so no event is stream-late, and none is construct-late (an
// event at ts on stream g has ts ≥ its stream's previous max ≥ the rule's clock). The test then releases the sink.
// Then Drain: Ingest.Drain raises every stream to max ts + 1 ms = 2 s + 199 s + 1 ms, so every event (ts ≤ max) is
// released.
//
// Expected, by hand:
//   - every Send returns nil; Drain returns nil within the ceiling (Pending 0: the body schedules no timer, and
//     the TTL timers are the partition's: see TestDrainBarrier's note on what Pending counts — this rule's purge
//     timers are pending too, so Pending is not asserted here);
//   - outputs: G × (2 + M) events × 8 = 4 × 202 × 8 = 6464, all at the sink when Drain returns (the barrier);
//   - per key, in order: key w<g>: (s<g>#1 j0..j7), (s<g>#2 j0..j7); key s<g>-k<c>: for i = c, c+5, … < 200, in
//     order, (s<g>#(3+i) j0..j7) — a key's events come from one stream in increasing ts, so the clock releases them
//     in Send order, its outputs leave one shard in order, and the router and the sink queue are FIFOs;
//   - Stats: no stream-late, no key error, no panic, no construct-late, no sink error; MailboxHigh has one entry
//     per shard, each ≤ Mailbox = 1 (a channel of capacity 1 never holds 2); PendingHigh ≥ Burst − 2 = 6 (phase 1:
//     one handoff carries ≥ 8 outputs of one release while the sink holds its first; at most 2 of them can leave
//     the FIFO — one in the flusher's held call, one in the queue — so the FIFO held ≥ 6 at once).
func TestSaturationProgress(t *testing.T) {
	const (
		G     = 4
		M     = 200
		K     = 5
		Burst = 8
	)
	ctx := rtCeiling(t)
	out := G // the derived stream's id

	streams := make([]clock.StreamSpec, 0, G+1)
	inputs := make([]int, G)
	keyAttr := make([]int, G)
	for g := 0; g < G; g++ {
		streams = append(streams, rtSrc(fmt.Sprintf("s%d", g), 0))
		inputs[g] = g
		keyAttr[g] = 0
	}
	streams = append(streams, rtDerived("out"))

	got := newRTLog()
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls int
	var callsMu sync.Mutex
	sink := func(e event.Event) error {
		callsMu.Lock()
		calls++
		first := calls == 1
		callsMu.Unlock()
		if first {
			close(entered)
			<-release // the slow sink: held until the test releases it
		}
		got.add(e.Values[0].Str, fmt.Sprintf("s%d#%d j%d", e.Values[1].I, e.Values[2].I, e.Values[3].I))
		return nil
	}

	body := rtBodies(rtBody{rule: "burst", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
		for j := 0; j < Burst; j++ {
			b.emit(event.Event{StreamID: out, Ts: e.Ts, Values: []event.Value{
				in.Key, rtInt(int64(e.StreamID)), rtInt(int64(e.Seq)), rtInt(int64(j)),
			}})
		}
	}})
	app := runtime.App{
		Streams: streams,
		Rules: []runtime.Rule{{
			Name: "burst", Inputs: inputs, KeyAttr: keyAttr, KeyType: event.TString, TTL: 24 * 3600 * rtSec,
			NewBody: body,
		}},
		Sinks: []runtime.SinkBinding{{Stream: out, Sink: sink}},
	}
	r := rtStart(t, runtime.Config{Shards: 2, Mailbox: 1, Intake: 1, SinkQueue: 1}, app)
	released := false
	t.Cleanup(func() { // registered after rtStart's Close, so it runs first: a failed wait never leaves the sink held
		if !released {
			close(release)
		}
	})

	// Phase 1.
	for round := 0; round < 2; round++ {
		for g := 0; g < G; g++ {
			rtSend(t, ctx, r, 0, rtEv(g, rtAt(float64(round)), rtStr(fmt.Sprintf("w%d", g))))
		}
	}
	rtWait(t, ctx, entered, "the sink's first call (phase 1's release reached the held sink)")

	// Phase 2: the bursty source, then the release.
	var wg sync.WaitGroup
	errs := make(chan error, G)
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < M; i++ {
				e := rtEv(g, rtAt(float64(2+i)), rtStr(fmt.Sprintf("s%d-k%d", g, i%K)))
				if err := r.Send(ctx, 0, e); err != nil {
					errs <- fmt.Errorf("goroutine %d, Send %d: %w", g, i, err)
					return
				}
			}
		}(g)
	}
	close(release)
	released = true
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("bursty source: %v (state: every Send admitted under the ceiling)", err)
	}

	rtDrain(t, ctx, r, 0)

	// Every output, at the sink when Drain returns.
	if n, want := got.count(), G*(2+M)*Burst; n != want {
		t.Fatalf("outputs at the sink after Drain: %d, want %d (G × (2 + M) × Burst)", n, want)
	}
	burst := func(dst []string, g, seq int) []string {
		for j := 0; j < Burst; j++ {
			dst = append(dst, fmt.Sprintf("s%d#%d j%d", g, seq, j))
		}
		return dst
	}
	for g := 0; g < G; g++ {
		key := fmt.Sprintf("w%d", g)
		want := burst(burst(nil, g, 1), g, 2)
		if have := got.get(key); !rtEqual(have, want) {
			t.Errorf("key %s: outputs %v,\nwant %v", key, have, want)
		}
		for c := 0; c < K; c++ {
			key := fmt.Sprintf("s%d-k%d", g, c)
			var want []string
			for i := c; i < M; i += K {
				want = burst(want, g, 3+i)
			}
			if have := got.get(key); !rtEqual(have, want) {
				t.Errorf("key %s: %d outputs, want %d in order; first difference shown in full:\n%v\nwant\n%v", key, len(have), len(want), have, want)
			}
		}
	}

	st := rtStats(t, ctx, r)
	for sid, n := range st.StreamLate[0] {
		if n != 0 {
			t.Errorf("stream %d: stream_late %d, want 0 (one sender per stream, monotone)", sid, n)
		}
	}
	if rs := st.Rules[0][0]; rs != (runtime.RuleStats{}) {
		t.Errorf("rule burst: %+v, want all zero", rs)
	}
	if n := st.SinkErrors[0][0]; n != 0 {
		t.Errorf("sink_errors %d, want 0", n)
	}
	if len(st.MailboxHigh) != 2 {
		t.Fatalf("MailboxHigh %v: want one entry per shard (2)", st.MailboxHigh)
	}
	for i, h := range st.MailboxHigh {
		if h < 0 || h > 1 {
			t.Errorf("shard %d: MailboxHigh %d, want within [0, Mailbox = 1]", i, h)
		}
	}
	if st.PendingHigh < Burst-2 {
		t.Errorf("PendingHigh %d, want ≥ %d: phase 1 held ≥ Burst − 2 outputs in the FIFO behind the held sink", st.PendingHigh, Burst-2)
	}
}
