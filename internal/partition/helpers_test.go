package partition_test

// The test harness for internal/partition (design note s3-2a-1 §1, §3): a recording rule body whose state reports
// exactly the timers it scheduled (ADR-0006 §2(c)), and a world that drives one Rule through the step contract —
// apply every input's advance, offer the event, advance — checking the derivability invariant after every step.
// All times are int64 ms; 10:00 is 0.

import (
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

const (
	second int64 = 1000
	minute       = 60 * second
	negInf int64 = math.MinInt64 // LastActivity before any released event (design §3)
	posInf int64 = math.MaxInt64 // every contribution after the end of input
)

func str(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }

func ev(sid int, ts int64, seq uint64) event.Event {
	return event.Event{StreamID: sid, Ts: ts, Seq: seq}
}

// keyName renders a key value for the log; a float keeps its sign ("-0").
func keyName(v event.Value) string {
	switch v.Kind {
	case event.TString:
		return v.Str
	case event.TInt:
		return strconv.FormatInt(v.I, 10)
	case event.TFloat:
		return strconv.FormatFloat(v.F, 'g', -1, 64)
	}
	return "?" + v.Kind.String()
}

// rec is one body callback, in the order the rule made it.
type rec struct {
	Cb    string // "new", "event", "timers", "discard", "purge"
	Key   string
	Ord   uint64 // the instance's clock ordinal
	Ts    int64  // "event": the released event's time
	Fires []clock.Timer
	Refs  []uint64
	// "purge": what the body saw inside OnPurge, which comes before the reset (design §3)
	OpSet   bool
	Last    int64
	Pending []clock.Timer
}

func N(key string, ord uint64) rec { return rec{Cb: "new", Key: key, Ord: ord} }

func Ev(key string, ord uint64, ts int64) rec { return rec{Cb: "event", Key: key, Ord: ord, Ts: ts} }

func Tm(key string, ord uint64, fires ...clock.Timer) rec {
	return rec{Cb: "timers", Key: key, Ord: ord, Fires: fires}
}

func P(key string, ord uint64, last int64, pending ...clock.Timer) rec {
	return rec{Cb: "purge", Key: key, Ord: ord, OpSet: true, Last: last, Pending: pending}
}

func recEqual(a, b rec) bool {
	return a.Cb == b.Cb && a.Key == b.Key && a.Ord == b.Ord && a.Ts == b.Ts && slices.Equal(a.Fires, b.Fires) &&
		slices.Equal(a.Refs, b.Refs) && a.OpSet == b.OpSet && a.Last == b.Last && slices.Equal(a.Pending, b.Pending)
}

// state is the body's per-key state: it implies exactly the timers it scheduled and has not seen fire.
type state struct {
	key    string
	timers []clock.Timer
	hide   bool          // planted: Timers reports nothing
	extra  []clock.Timer // planted: reported but never scheduled
}

func (s *state) Walk(fn func(component, key string, state any)) { fn("op", s.key, len(s.timers)) }

func (s *state) Timers(dst []clock.Timer) []clock.Timer {
	if s.hide {
		return dst
	}
	dst = append(dst, s.timers...)
	return append(dst, s.extra...)
}

// body is the recording test double. schedule, when set, names the timers to schedule for a released event.
type body struct {
	t        *testing.T
	log      []rec
	schedule func(in *partition.Instance, e event.Event) []clock.Timer
	plant    func(key string, st *state) // planted faults, applied at birth
}

func (b *body) NewState(in *partition.Instance) partition.OpState {
	k := keyName(in.Key)
	b.log = append(b.log, N(k, in.Clock.Ordinal()))
	st := &state{key: k}
	if b.plant != nil {
		b.plant(k, st)
	}
	return st
}

func (b *body) OnEvent(in *partition.Instance, e event.Event) {
	b.log = append(b.log, Ev(keyName(in.Key), in.Clock.Ordinal(), e.Ts))
	if in.Op == nil {
		b.t.Errorf("OnEvent(%s, ts %d) with Op nil: the birth comes first (design §3)", keyName(in.Key), e.Ts)
		return
	}
	if in.LastActivity != e.Ts {
		b.t.Errorf("OnEvent(%s, ts %d): LastActivity %d, want the event's time (set before OnEvent, design §3)",
			keyName(in.Key), e.Ts, in.LastActivity)
	}
	if b.schedule == nil {
		return
	}
	st := in.Op.(*state)
	for _, tm := range b.schedule(in, e) {
		in.Clock.Schedule(tm.Due, tm.Kind, tm.Ref)
		st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool { return x.Kind == tm.Kind && x.Ref == tm.Ref })
		st.timers = append(st.timers, tm)
	}
}

func (b *body) OnTimers(in *partition.Instance, fires []clock.Timer, tables any) {
	b.log = append(b.log, Tm(keyName(in.Key), in.Clock.Ordinal(), slices.Clone(fires)...))
	for _, tm := range fires {
		if tm.Kind == clock.TTLPurge {
			b.t.Errorf("Body.OnTimers received a TTLPurge %+v: never (design §3)", tm)
		}
	}
	if st, ok := in.Op.(*state); ok {
		st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool { return slices.Contains(fires, x) })
	}
}

func (b *body) OnDiscard(in *partition.Instance, refs []uint64) {
	b.log = append(b.log, rec{Cb: "discard", Key: keyName(in.Key), Ord: in.Clock.Ordinal(), Refs: slices.Clone(refs)})
	if st, ok := in.Op.(*state); ok {
		st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool {
			return x.Kind == clock.Discard && slices.Contains(refs, x.Ref)
		})
	}
}

func (b *body) OnPurge(in *partition.Instance) {
	b.log = append(b.log, rec{Cb: "purge", Key: keyName(in.Key), Ord: in.Clock.Ordinal(), OpSet: in.Op != nil,
		Last: in.LastActivity, Pending: in.Clock.Timers(nil)})
}

func (b *body) count(cb string) int {
	n := 0
	for _, r := range b.log {
		if r.Cb == cb {
			n++
		}
	}
	return n
}

// world drives one Rule the way every driver must (design §1): apply, offer, advance; then CheckTimers.
type world struct {
	t       *testing.T
	r       *partition.Rule
	b       *body
	inputs  []int
	keyType event.Type
	noCheck bool
}

func newWorld(t *testing.T, ttl int64, b *body, inputs ...int) *world {
	t.Helper()
	return newWorldKeyed(t, event.TString, ttl, b, inputs...)
}

func newWorldKeyed(t *testing.T, kt event.Type, ttl int64, b *body, inputs ...int) *world {
	t.Helper()
	if b.t == nil {
		b.t = t
	}
	r, err := partition.NewRule(partition.Config{Inputs: inputs, KeyType: kt, TTL: ttl, Body: b})
	if err != nil {
		t.Fatalf("NewRule: %v", err)
	}
	return &world{t: t, r: r, b: b, inputs: inputs, keyType: kt}
}

// apply is step 1: every input's contribution rises to c, without walking.
func (w *world) apply(c int64) {
	for _, sid := range w.inputs {
		w.r.Apply(clock.WMAdvance{StreamID: sid, Contribution: c, Moved: true})
	}
}

func (w *world) advance() {
	w.t.Helper()
	w.r.Advance(nil)
	if w.noCheck {
		return
	}
	if err := w.r.CheckTimers(); err != nil {
		w.t.Fatalf("CheckTimers after Advance at clock %d: %v", w.r.Clock(), err)
	}
}

// arrive is one arrival under the step contract: apply (clock to c), offer e under key, advance.
func (w *world) arrive(c int64, key event.Value, e event.Event) (constructLate bool) {
	w.t.Helper()
	w.apply(c)
	constructLate = w.r.Offer(key, e)
	w.advance()
	return constructLate
}

// to is an advance with no event: steps 1 and 3.
func (w *world) to(c int64) {
	w.t.Helper()
	w.apply(c)
	w.advance()
}

func instances(r *partition.Rule) []*partition.Instance {
	var out []*partition.Instance
	r.Instances(func(in *partition.Instance) { out = append(out, in) })
	return out
}

// find is the resident instance whose key renders as k, or nil.
func find(r *partition.Rule, k string) *partition.Instance {
	for _, in := range instances(r) {
		if keyName(in.Key) == k {
			return in
		}
	}
	return nil
}

func wantLog(t *testing.T, b *body, want ...rec) {
	t.Helper()
	if !slices.EqualFunc(b.log, want, recEqual) {
		t.Fatalf("body log\n got %+v\nwant %+v", b.log, want)
	}
}

func ttl(due int64) clock.Timer { return clock.Timer{Due: due, Kind: clock.TTLPurge} }

func mustPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", what)
		}
	}()
	f()
}
