package runtime_test

// Exit 3 of S3.2a (design v2 §2, §3, §8): progress with a full mailbox at a shard whose handoff is pending, plus a
// slow sink and a bursty source. The coordinator runs it 50 times under -race with a CPU hog.

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
// The world: G = 4 source streams s0..s3 (Delay 0, no idleness; schema [k string]) and a derived stream "out" (id G;
// schema [k string, stream int, seq int, j int]). One rule "burst" reads all four, keyed by k, TTL 24 h (no purge:
// every event lies within 210 s). Its body emits Burst = 8 outputs on "out" per released event: [key, stream, seq,
// j], j = 0..7. One sink records each output under its key; the test can arm it to hold its next call on a channel
// (the slow sink: never a sleep). Config: Shards 2, Mailbox 1, Intake 1, SinkQueue 1; the handoff channel holds
// Shards = 2 outboxes (design §3). The rule's clock is min over its inputs' contributions (Delay 0: each stream's max
// ts); every event raises its stream's contribution, so every event sends a message to BOTH shards (design §4.4).
//
// Phase 1 — the saturated park, forced (F16). Round 1 sends ts 0 s on s0..s3 (keys w0..w3), round 2 ts 1 s (same
// keys): round 2's s3 event moves the clock to 1 s and releases round 1. The body of w0, on its first release, emits
// its burst and then HOLDS until the test releases it (state: bodyHeld). Shard X (w0's) is then stuck mid-message
// with a non-empty outbox. The test sends E1, E2, E3 = s0, s1, s2 @ 2 s, key x, from its own goroutine. With Intake 1,
// Send E3 can return only once the router has taken E2 from the intake, and the router reads the intake only with
// nothing pending (design §3), so every message of E1 was sent — including E1's to X, which sits in X's mailbox: it is
// full (capacity 1, X is held). (This reads Send as returning once enqueued: v1 §2 "waits on the intake queue only";
// gap Q10.) The test releases the body: X finishes with a non-empty outbox and a full mailbox, and parks:
// SaturatedParks[X] ≥ 1. This is the exit-3 moment: a naive shard blocked on its handoff and a naive router blocked
// on X's mailbox would deadlock here.
// Phase 2 — the slow sink. The test arms the sink hold and sends s3 @ 2 s (key x): the clock moves to 2 s and
// releases round 2 (four keys at 1 s, ≥ 16 outputs from at least one shard in one handoff); the sink's next call
// holds (state: entered). The router's FIFO head is now a sink-queue send that cannot complete; it must keep
// receiving handoffs.
// Phase 3 — the bursty source. G goroutines; goroutine g sends M = 200 events on stream g, ts 3 s + i s, key
// s<g>-k<i mod 5>. One sender per stream keeps each stream monotone: no event is stream-late or construct-late (Q9).
// The test releases the sink, joins the senders, then Drains: max ts + 1 ms releases every event.
//
// Expected, by hand:
//   - every Send returns nil; Drain returns nil within the ceiling;
//   - outputs: (4 + 4 + 4 + G × M) × 8 = 812 × 8 = 6496, all at the sink when Drain returns (the barrier);
//   - per key, in order: w<g>: (s<g>#1 j0..7), (s<g>#2 j0..7); x: (s0#3), (s1#3), (s2#3), (s3#3), each j0..7 (one
//     instant, released in (Ts, StreamID, Seq) order); s<g>-k<c>: for i = c, c+5, … < 200, (s<g>#(4+i) j0..7);
//   - Stats: no stream-late, key error, panic, construct-late or sink error; MailboxHigh: 2 entries, each ≤ 1;
//     SaturatedParks: 2 entries, their sum ≥ 1 (phase 1); PendingHigh ≥ Burst − 2 = 6 (phase 2: a handoff of ≥ 8
//     outputs behind the held sink; at most 2 leave the FIFO, one in the held call and one in the queue).
func TestSaturationProgress(t *testing.T) {
	const (
		G     = 4
		M     = 200
		K     = 5
		Burst = 8
	)
	ctx := rtCeiling(t)
	out := G

	var streams []clock.StreamSpec
	var schemas []event.Schema
	inputs := make([]int, G)
	keyAttrs := make([][]int, G)
	for g := 0; g < G; g++ {
		keyAttrs[g] = []int{0}
		name := fmt.Sprintf("s%d", g)
		streams = append(streams, rtSrc(name, 0))
		schemas = append(schemas, rtSchema(t, name, rtA("k", event.TString)))
		inputs[g] = g
	}
	streams = append(streams, rtDerived("out"))
	schemas = append(schemas, rtSchema(t, "out", rtA("k", event.TString), rtA("stream", event.TInt), rtA("seq", event.TInt), rtA("j", event.TInt)))

	got := newRTLog()
	hold := newRTHold()
	// round1 closes when round 1's G × Burst outputs have reached the sink. Phase 2 arms the hold only then, so the hold
	// catches a round-2 output: arming earlier could catch a round-1 output still in flight, and the router (rightly)
	// reads no intake while its sink send is held, so the test's own Send in phase 2 would wait on a hold only the test
	// releases (coordinator, 2026-11-05: the first real run hung here).
	round1 := make(chan struct{})
	var round1Once sync.Once
	sink := func(e event.Event) error {
		hold.at()
		got.add(e.Values[0].Str, fmt.Sprintf("s%d#%d j%d", e.Values[1].I, e.Values[2].I, e.Values[3].I))
		if got.count() == G*Burst {
			round1Once.Do(func() { close(round1) })
		}
		return nil
	}
	bodyHeld := make(chan struct{})
	bodyRelease := make(chan struct{})
	var holdOnce, releaseOnce sync.Once
	freeBody := func() { releaseOnce.Do(func() { close(bodyRelease) }) }

	body := rtBodies(rtBody{rule: "burst", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
		for j := 0; j < Burst; j++ {
			b.emit(event.Event{StreamID: out, Ts: e.Ts, Values: []event.Value{
				in.Key.First, rtInt(int64(e.StreamID)), rtInt(int64(e.Seq)), rtInt(int64(j)),
			}})
		}
		if in.Key.First.Str == "w0" {
			holdOnce.Do(func() {
				close(bodyHeld)
				<-bodyRelease // the held shard: stuck mid-message with a non-empty outbox
			})
		}
	}})
	app := runtime.App{
		Streams: streams, Schemas: schemas,
		Rules: []runtime.Rule{{
			Name: "burst", Inputs: inputs, KeyAttrs: keyAttrs, KeyTypes: []event.Type{event.TString}, TTL: 24 * 3600 * rtSec,
			NewBody: body,
		}},
		Sinks: []runtime.SinkBinding{{Stream: out, Sink: sink}},
	}
	r := rtStart(t, runtime.Config{Shards: 2, Mailbox: 1, Intake: 1, SinkQueue: 1}, app)
	t.Cleanup(func() { freeBody(); hold.free() }) // runs before rtStart's Close: a failed wait never leaves a hold

	// Phase 1.
	for round := 0; round < 2; round++ {
		for g := 0; g < G; g++ {
			rtSend(t, ctx, r, 0, rtEv(g, rtAt(float64(round)), rtStr(fmt.Sprintf("w%d", g))))
		}
	}
	rtWait(t, ctx, bodyHeld, "w0's body held mid-message (round 1 released)")
	for g := 0; g < 3; g++ {
		rtSend(t, ctx, r, 0, rtEv(g, rtAt(2), rtStr("x"))) // E1..E3; E3's return: E1's message is in X's full mailbox
	}
	freeBody()

	// Phase 2.
	rtWait(t, ctx, round1, "round 1's outputs at the sink")
	hold.arm()
	rtSend(t, ctx, r, 0, rtEv(3, rtAt(2), rtStr("x")))
	rtWait(t, ctx, hold.entered, "the sink's held call (round 2 released behind it)")

	// Phase 3.
	var wg sync.WaitGroup
	errs := make(chan error, G)
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < M; i++ {
				e := rtEv(g, rtAt(float64(3+i)), rtStr(fmt.Sprintf("s%d-k%d", g, i%K)))
				if err := r.Send(ctx, 0, e); err != nil {
					errs <- fmt.Errorf("goroutine %d, Send %d: %w", g, i, err)
					return
				}
			}
		}(g)
	}
	hold.free()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("bursty source: %v (state: every Send admitted under the ceiling)", err)
	}

	rtDrain(t, ctx, r, 0)

	if n, want := got.count(), (12+G*M)*Burst; n != want {
		t.Fatalf("outputs at the sink after Drain: %d, want %d", n, want)
	}
	burst := func(dst []string, g, seq int) []string {
		for j := 0; j < Burst; j++ {
			dst = append(dst, fmt.Sprintf("s%d#%d j%d", g, seq, j))
		}
		return dst
	}
	var wantX []string
	for g := 0; g < G; g++ {
		wantX = burst(wantX, g, 3)
	}
	if have := got.get("x"); !rtEqual(have, wantX) {
		t.Errorf("key x: outputs %v,\nwant %v", have, wantX)
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
				want = burst(want, g, 4+i)
			}
			if have := got.get(key); !rtEqual(have, want) {
				t.Errorf("key %s: %d outputs, want %d in order:\n%v\nwant\n%v", key, len(have), len(want), have, want)
			}
		}
	}

	st := rtStats(t, ctx, r)
	for sid, ss := range st.Streams[0] {
		n := ss.Late
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
	if len(st.MailboxHigh) != 2 || len(st.SaturatedParks) != 2 {
		t.Fatalf("MailboxHigh %v, SaturatedParks %v: want one entry per shard (2)", st.MailboxHigh, st.SaturatedParks)
	}
	for i, h := range st.MailboxHigh {
		if h < 0 || h > 1 {
			t.Errorf("shard %d: MailboxHigh %d, want within [0, Mailbox = 1]", i, h)
		}
	}
	if st.SaturatedParks[0]+st.SaturatedParks[1] < 1 {
		t.Errorf("SaturatedParks %v, want a sum ≥ 1: phase 1 parked w0's shard with a non-empty outbox and a full mailbox (F16)", st.SaturatedParks)
	}
	if st.PendingHigh < Burst-2 {
		t.Errorf("PendingHigh %d, want ≥ %d: phase 2 held ≥ Burst − 2 outputs in the FIFO behind the held sink", st.PendingHigh, Burst-2)
	}
}
