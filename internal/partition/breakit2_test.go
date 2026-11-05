package partition_test

// S3.2a-1 break-it (QA): attacks on partition.Rule under owner D1. A red test names its defect; a guard test
// (TestBreakitGuard...) records an attack that held.

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// bkState is a body's per-key state: the timers it scheduled and has not seen fire.
type bkState struct{ timers []clock.Timer }

func (s *bkState) Walk(fn func(component, key string, state any)) { fn("op", "", len(s.timers)) }
func (s *bkState) Timers(dst []clock.Timer) []clock.Timer         { return append(dst, s.timers...) }

// bkBody logs every callback as text; keyText renders the instance's Key (sign of zero included).
type bkBody struct {
	log      []string
	complete int64 // > 0: schedule Completion at e.Ts+complete-1 (so 0 is "at the event's own instant")
	discard  int64 // > 0: schedule Discard at e.Ts+discard
	chain    bool  // on a fired Completion with an odd ref, schedule CronClose at the same instant
}

func bkKeyText(v event.Value) string {
	if v.Kind == event.TFloat {
		return fmt.Sprintf("f%v/neg=%v", v.F, math.Signbit(v.F))
	}
	return v.Str
}

func (b *bkBody) sched(in *partition.Instance, due int64, k clock.TimerKind, ref uint64) {
	in.Clock.Schedule(due, k, ref)
	st := in.Op.(*bkState)
	st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool { return x.Kind == k && x.Ref == ref })
	st.timers = append(st.timers, clock.Timer{Due: due, Kind: k, Ref: ref})
}

func (b *bkBody) NewState(in *partition.Instance) partition.OpState {
	b.log = append(b.log, "new "+bkKeyText(in.Key))
	return &bkState{}
}

func (b *bkBody) OnEvent(in *partition.Instance, e event.Event) {
	b.log = append(b.log, fmt.Sprintf("event %s s%d #%d @%d", bkKeyText(in.Key), e.StreamID, e.Seq, e.Ts))
	ref := uint64(e.StreamID)<<32 | e.Seq
	if b.complete > 0 {
		b.sched(in, e.Ts+b.complete-1, clock.Completion, ref)
	}
	if b.discard > 0 {
		b.sched(in, e.Ts+b.discard, clock.Discard, ref)
	}
}

func (b *bkBody) OnTimers(in *partition.Instance, fires []clock.Timer, _ any) {
	b.log = append(b.log, fmt.Sprintf("timers %s %v", bkKeyText(in.Key), fires))
	st := in.Op.(*bkState)
	st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool { return slices.Contains(fires, x) })
	if b.chain {
		for _, f := range fires {
			if f.Kind == clock.Completion && f.Ref%2 == 1 {
				b.sched(in, f.Due, clock.CronClose, f.Ref)
			}
		}
	}
}

func (b *bkBody) OnDiscard(in *partition.Instance, refs []uint64) {
	b.log = append(b.log, fmt.Sprintf("discard %s %v", bkKeyText(in.Key), refs))
	st := in.Op.(*bkState)
	st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool {
		return x.Kind == clock.Discard && slices.Contains(refs, x.Ref)
	})
}

func (b *bkBody) OnPurge(in *partition.Instance) {
	b.log = append(b.log, fmt.Sprintf("purge %s last=%d", bkKeyText(in.Key), in.LastActivity))
}

// bkMsg is one message of the step contract: advances, then an optional event under a key.
type bkMsg struct {
	adv   []clock.WMAdvance
	key   event.Value
	e     event.Event
	event bool
}

// bkRun drives one rule through msgs, walking after every message whose index is in walkAfter (and always at the
// end, after +inf on every input). It returns the body's log, the stats and every Offer's construct-late result.
func bkRun(t *testing.T, kt event.Type, ttl int64, mk func() *bkBody, inputs []int, msgs []bkMsg, walkAfter func(i int) bool) ([]string, partition.Stats, []bool) {
	t.Helper()
	b := mk()
	r, err := partition.NewRule(partition.Config{Inputs: inputs, KeyType: kt, TTL: ttl, Body: b})
	if err != nil {
		t.Fatal(err)
	}
	var late []bool
	walk := func() {
		r.Advance(nil)
		if err := r.CheckTimers(); err != nil {
			t.Fatalf("CheckTimers: %v", err)
		}
	}
	for i, m := range msgs {
		for _, a := range m.adv {
			r.Apply(a)
		}
		if m.event {
			late = append(late, r.Offer(m.key, m.e))
		}
		if walkAfter(i) {
			walk()
		}
	}
	for _, sid := range inputs {
		r.Apply(clock.WMAdvance{StreamID: sid, Contribution: math.MaxInt64, Moved: true})
	}
	walk()
	if r.Len() != 0 && ttl > 0 {
		t.Errorf("after +inf %d instances are resident; every key purges at the end of input (design §3, QA A9)", r.Len())
	}
	return b.log, r.Stats(), late
}

func bkFloat(f float64) event.Value { return event.Value{Kind: event.TFloat, F: f} }
func bkStr(s string) event.Value    { return event.Value{Kind: event.TString, Str: s} }
func bkAdv(sid int, c int64) clock.WMAdvance {
	return clock.WMAdvance{StreamID: sid, Contribution: c, Moved: true}
}

// TestBreakitNegZeroKeyFollowsWalkFrequency: -0.0 and +0.0 are one key (design §3), and "Instance.Key" is the first
// event's key value. Whether a returning +0.0 event finds the purged -0.0 instance still resident depends on whether
// a walk ran between the purge's due passing and the event's offer, so the Key a body (and a trace's "key") sees
// depends on walk frequency. Design §1 / §10: walking per message or per chunk MUST give the same callbacks (B1: only
// Ordinals may differ).
//
// Timeline (TTL 10, one input): -0.0 at 0; the clock to 1 releases it (TTL due 10); an advance alone to 15 passes
// the due; then +0.0 at 20; the clock to 21 releases it.
//   - walking per message: the walk at 15 purges and frees the key; +0.0 makes a new instance, Key +0.0;
//   - walking only at the last message: +0.0 is offered to the still-resident instance (Key -0.0), the walk purges it
//     (it holds the buffered event, so it stays) and reborn, its Key is still -0.0.
func TestBreakitNegZeroKeyFollowsWalkFrequency(t *testing.T) {
	negZero := math.Copysign(0, -1)
	msgs := []bkMsg{
		{key: bkFloat(negZero), e: event.Event{StreamID: 0, Ts: 0, Seq: 1}, event: true},
		{adv: []clock.WMAdvance{bkAdv(0, 1)}},
		{adv: []clock.WMAdvance{bkAdv(0, 15)}},
		{key: bkFloat(0), e: event.Event{StreamID: 0, Ts: 20, Seq: 2}, event: true},
		{adv: []clock.WMAdvance{bkAdv(0, 21)}},
	}
	mk := func() *bkBody { return &bkBody{} }
	perMsg, _, _ := bkRun(t, event.TFloat, 10, mk, []int{0}, msgs, func(int) bool { return true })
	chunked, _, _ := bkRun(t, event.TFloat, 10, mk, []int{0}, msgs, func(i int) bool { return i == 1 || i == 4 })
	if !slices.Equal(perMsg, chunked) {
		t.Errorf("the callbacks depend on walk frequency (design §1: walking per message or per chunk is the same):\n per message: %q\n per chunk:   %q", perMsg, chunked)
	}
}

// TestBreakitGuardRandomWalkFrequencyInvariance: randomized logs of string keys over two inputs, ties at one
// instant, construct-late events, timers due at event instants (Completion at e.Ts), chained timers at the instant
// being walked, discards and TTL purges inside large jumps, then +inf: walking after every message, after random
// chunks, or only at the end gives the same callbacks, the same stats and the same construct-late verdicts.
func TestBreakitGuardRandomWalkFrequencyInvariance(t *testing.T) {
	for seed := uint64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewPCG(seed, 99))
		contrib := []int64{0, 0}
		var seq [2]uint64
		var msgs []bkMsg
		for range 40 {
			var m bkMsg
			for sid := range 2 {
				if rng.IntN(3) == 0 {
					step := int64(rng.IntN(3))
					if rng.IntN(8) == 0 {
						step = int64(5 + rng.IntN(30)) // a jump across TTLs and discards
					}
					contrib[sid] += step
					m.adv = append(m.adv, bkAdv(sid, contrib[sid]))
				}
			}
			if rng.IntN(4) != 0 {
				sid := rng.IntN(2)
				seq[sid]++
				lo := min(contrib[0], contrib[1])
				ts := lo - 1 + int64(rng.IntN(4)) // some construct-late, many at the clock itself
				m.event, m.key = true, bkStr(string(rune('a'+rng.IntN(4))))
				m.e = event.Event{StreamID: sid, Ts: ts, Seq: seq[sid]}
			}
			msgs = append(msgs, m)
		}
		mk := func() *bkBody { return &bkBody{complete: int64(1 + seed%3), discard: int64(2 + seed%4), chain: true} }
		ttl := int64(3 + seed%5)
		baseLog, baseStats, baseLate := bkRun(t, event.TString, ttl, mk, []int{0, 1}, msgs, func(int) bool { return true })
		chunk := rand.New(rand.NewPCG(seed, 7))
		for _, w := range []struct {
			name      string
			walkAfter func(int) bool
		}{
			{"chunks", func(int) bool { return chunk.IntN(3) == 0 }},
			{"end", func(int) bool { return false }},
		} {
			name, walkAfter := w.name, w.walkAfter
			log, stats, late := bkRun(t, event.TString, ttl, mk, []int{0, 1}, msgs, walkAfter)
			if !slices.Equal(baseLog, log) || baseStats != stats || !slices.Equal(baseLate, late) {
				t.Fatalf("seed %d, walking %s: differs from walking per message\n per message: %q %+v\n %s: %q %+v", seed, name, baseLog, baseStats, name, log, stats)
			}
		}
	}
}

// bkQuiet is a body that allocates nothing.
type bkQuiet struct{ st bkState }

func (q *bkQuiet) NewState(*partition.Instance) partition.OpState   { return &q.st }
func (q *bkQuiet) OnEvent(*partition.Instance, event.Event)         {}
func (q *bkQuiet) OnTimers(*partition.Instance, []clock.Timer, any) {}
func (q *bkQuiet) OnDiscard(*partition.Instance, []uint64)          {}
func (q *bkQuiet) OnPurge(*partition.Instance)                      {}

// TestBreakitGuardResidentKeyAllocatesNothing: design §3 MUST: Offer + Advance on a resident, born key with a body
// that allocates nothing allocates nothing (ADR-0010 §1), the TTL re-arm included, for a string and a float key.
func TestBreakitGuardResidentKeyAllocatesNothing(t *testing.T) {
	for _, key := range []event.Value{bkStr("k"), bkFloat(math.Copysign(0, -1))} {
		r, err := partition.NewRule(partition.Config{Inputs: []int{0, 1}, KeyType: key.Kind, TTL: 1000, Body: &bkQuiet{}})
		if err != nil {
			t.Fatal(err)
		}
		vals := []event.Value{bkStr("x")}
		var c int64
		step := func() {
			c++
			r.Apply(bkAdv(0, c))
			r.Apply(bkAdv(1, c))
			r.Offer(key, event.Event{StreamID: 0, Ts: c, Seq: uint64(c), Values: vals})
			r.Advance(nil)
		}
		for range 64 {
			step()
		}
		if n := testing.AllocsPerRun(500, step); n != 0 {
			t.Errorf("key %v: Offer + Advance on a resident key allocates %v per step, want 0 (design §3)", key, n)
		}
	}
}

// TestBreakitGuardOfferAfterEndOfInput: after +inf every key purged and was freed; an event offered then is
// construct-late, counted, and leaves no instance.
func TestBreakitGuardOfferAfterEndOfInput(t *testing.T) {
	r, err := partition.NewRule(partition.Config{Inputs: []int{0}, KeyType: event.TString, TTL: 5, Body: &bkBody{}})
	if err != nil {
		t.Fatal(err)
	}
	r.Offer(bkStr("a"), event.Event{Ts: 1, Seq: 1})
	r.Apply(bkAdv(0, math.MaxInt64))
	r.Advance(nil)
	if r.Len() != 0 || r.Stats().Purged != 1 {
		t.Fatalf("after +inf: Len %d, Purged %d; want 0 and 1", r.Len(), r.Stats().Purged)
	}
	if !r.Offer(bkStr("a"), event.Event{Ts: 100, Seq: 2}) || r.Len() != 0 || r.Stats().ConstructLate != 1 {
		t.Errorf("an offer after +inf: want construct-late, no instance; Len %d, stats %+v", r.Len(), r.Stats())
	}
	r.Advance(nil)
}
