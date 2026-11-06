package runtime_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

// TestKeyErrorsAndZeroKeys (design v2 §4.3, §7): a null or NaN key is a per-rule key error, never offered; −0.0 and
// +0.0 reach one instance on one shard.
//
// Stream "in" (Delay 0; schema [f optional float, s string]). Rules f0..f3 keyed by f (TFloat), TTL 1 h; rule "s"
// keyed by s, TTL 1 h. Shards 16, and four float rules (each rule index hashes apart): a router that hashed −0.0 and
// +0.0 apart would put them on one shard of 16 for all four rules with probability (1/16)^4.
// Events: 1 s f = −0.0; 2 s f = null; 3 s f = NaN; 4 s f = +0.0; 5 s f = 1.5; all s = "a". Drain (5.001 s).
// The NaN event is refused by Send itself (fix round, 2026-11-05: NaN and infinities are never values, S3.1-2's D2),
// so it takes no Seq: the others are #1 @1 s, #2 @2 s, #3 @4 s, #4 @5 s.
// Expected: each f rule: key 0 (stored +0.0, break-it K1) released [s0#1 @1 s, s0#3 @4 s] with ONE birth; key 1.5
// [s0#4 @5 s]; nothing else; KeyErrors 1 (null). Rule s: key a gets all four; KeyErrors 0.
func TestKeyErrorsAndZeroKeys(t *testing.T) {
	ctx := rtCeiling(t)
	log, births := newRTLog(), newRTLog()
	var rules []runtime.Rule
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("f%d", i)
		rules = append(rules, runtime.Rule{Name: name, Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TFloat},
			TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: name, log: log, births: births})})
	}
	rules = append(rules, runtime.Rule{Name: "s", Inputs: []int{0}, KeyAttrs: [][]int{{1}}, KeyTypes: []event.Type{event.TString},
		TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: "s", log: log, births: births})})
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0)},
		Schemas: []event.Schema{rtSchema(t, "in", rtOpt("f", event.TFloat), rtA("s", event.TString))},
		Rules:   rules,
	}
	r := rtStart(t, runtime.Config{Shards: 16}, app)
	fs := []event.Value{rtFloat(math.Copysign(0, -1)), {Kind: event.TFloat, Null: true}, rtFloat(math.NaN()), rtFloat(0), rtFloat(1.5)}
	for i, f := range fs {
		e := rtEv(0, rtAt(float64(i+1)), f, rtStr("a"))
		if !f.Null && math.IsNaN(f.F) {
			if err := r.Send(ctx, 0, e); err == nil {
				t.Errorf("Send with a NaN key: nil, want an error (NaN is never a value)")
			}
			continue
		}
		rtSend(t, ctx, r, 0, e)
	}
	rtDrain(t, ctx, r, 0)

	want := map[string][]string{"s/a": {"ev s0#1 @1000", "ev s0#2 @2000", "ev s0#3 @4000", "ev s0#4 @5000"}}
	for i := 0; i < 4; i++ {
		want[fmt.Sprintf("f%d/0", i)] = []string{"ev s0#1 @1000", "ev s0#3 @4000"}
		want[fmt.Sprintf("f%d/1.5", i)] = []string{"ev s0#4 @5000"}
	}
	got := log.snapshot()
	for k, w := range want {
		if !rtEqual(got[k], w) {
			t.Errorf("%s: %v, want %v", k, got[k], w)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: %v, want nothing (a null or NaN key is never offered; −0.0 is key 0)", k, got[k])
		}
	}
	for i := 0; i < 4; i++ {
		k := fmt.Sprintf("f%d/0", i)
		if b := births.get(k); len(b) != 1 {
			t.Errorf("%s: births %v, want exactly one (one instance on one shard for −0.0 and +0.0)", k, b)
		}
	}
	st := rtStats(t, ctx, r)
	for i := 0; i < 4; i++ {
		if n := st.Rules[0][i].KeyErrors; n != 1 {
			t.Errorf("rule f%d: KeyErrors %d, want 1 (the null; the NaN never got past Send)", i, n)
		}
	}
	if n := st.Rules[0][4].KeyErrors; n != 0 {
		t.Errorf("rule s: KeyErrors %d, want 0", n)
	}
}

// TestKeylessRuleOneInstance (design v2 §2, §4.3): a key-less rule (KeyAttr nil, KeyType TString, TTL 0) runs as one
// instance under the pseudo-key "", on one shard, never purged. Shards 4. Stream "in" (Delay 0; schema [v int]).
// Events at 1, 2, 3, 4, 5 s and 10000 s (a gap no TTL would survive), v = their index. Drain (10000.001 s).
// Expected: key "" released all six, s0#1..#6, in order; one birth; Purged 0; no other key.
func TestKeylessRuleOneInstance(t *testing.T) {
	ctx := rtCeiling(t)
	log, births := newRTLog(), newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0)},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("v", event.TInt))},
		Rules: []runtime.Rule{{Name: "K", Inputs: []int{0}, KeyTypes: []event.Type{event.TString},
			NewBody: rtBodies(rtBody{rule: "K", log: log, births: births})}},
	}
	r := rtStart(t, runtime.Config{Shards: 4}, app)
	tss := []float64{1, 2, 3, 4, 5, 10000}
	var want []string
	for i, s := range tss {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(s), rtInt(int64(i))))
		want = append(want, fmt.Sprintf("ev s0#%d @%d", i+1, int64(s*1000)))
	}
	rtDrain(t, ctx, r, 0)
	got := log.snapshot()
	if !rtEqual(got["K/"], want) || len(got) != 1 {
		t.Errorf("released %v, want only K/ = %v", got, want)
	}
	if b := births.get("K/"); len(b) != 1 {
		t.Errorf("births %v, want one", b)
	}
	if n := rtStats(t, ctx, r).Rules[0][0].Purged; n != 0 {
		t.Errorf("Purged %d, want 0 (a key-less rule is never purged)", n)
	}
}

// TestPanicResetsTheKey (owner F10; design v2 §1b, §4): a body that panics on one key is counted, that key is reset,
// and other keys and rules are unaffected IN THE SAME WALK.
//
// Stream "in" (Delay 10 s, so nothing releases before the drain; schema [k string]). Rules p and q keyed by k, TTL 1 h;
// p's body records, then panics when the key is "boom"; q records. Events: ok1 @1 s, boom @2 s, ok2 @3 s, boom @4 s,
// ok3 @5 s. Drain (5.001 s) walks once per rule: p releases ok1, boom@2 (panic: boom reset), ok2, boom@4 (a fresh
// state: a second birth; panic again), ok3 — all in that one walk (F1).
// Expected: p/ok1 [s0#1], p/ok2 [s0#3], p/ok3 [s0#5], p/boom [s0#2, s0#4] with two births; q gets every event
// under its key; Stats: p Panics 2, Purged 0; q Panics 0.
func TestPanicResetsTheKey(t *testing.T) {
	ctx := rtCeiling(t)
	log, births := newRTLog(), newRTLog()
	panicky := rtBody{rule: "p", log: log, births: births, onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
		if in.Key.First.Str == "boom" {
			panic("a body's panic on key boom")
		}
	}}
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 10*rtSec)},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
		Rules: []runtime.Rule{
			{Name: "p", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtBodies(panicky)},
			{Name: "q", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
				NewBody: rtBodies(rtBody{rule: "q", log: log})},
		},
	}
	r := rtStart(t, runtime.Config{Shards: 4}, app)
	keys := []string{"ok1", "boom", "ok2", "boom", "ok3"}
	for i, k := range keys {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(i+1)), rtStr(k)))
	}
	rtDrain(t, ctx, r, 0)
	ev := func(i int) string { return fmt.Sprintf("ev s0#%d @%d", i, int64(i)*rtSec) }
	want := map[string][]string{
		"p/ok1": {ev(1)}, "p/ok2": {ev(3)}, "p/ok3": {ev(5)}, "p/boom": {ev(2), ev(4)},
		"q/ok1": {ev(1)}, "q/ok2": {ev(3)}, "q/ok3": {ev(5)}, "q/boom": {ev(2), ev(4)},
	}
	got := log.snapshot()
	for k, w := range want {
		if !rtEqual(got[k], w) {
			t.Errorf("%s: %v, want %v", k, got[k], w)
		}
	}
	if b := births.get("p/boom"); len(b) != 2 {
		t.Errorf("p/boom births %v, want 2 (reset after its first panic, reborn at boom@4 s)", b)
	}
	st := rtStats(t, ctx, r)
	if p := st.Rules[0][0]; p.Panics != 2 || p.Purged != 0 {
		t.Errorf("rule p: %+v, want Panics 2, Purged 0", p)
	}
	if q := st.Rules[0][1]; q.Panics != 0 {
		t.Errorf("rule q: Panics %d, want 0", q.Panics)
	}
}
