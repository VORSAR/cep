package clock

import (
	"math"

	"github.com/vorsar/cep/internal/event"
)

// The walk's phases at one instant: released events, then inclusive timers, then strict ones.
const (
	phaseEvents = iota
	phaseInclusive
	phaseStrict
)

type timer struct {
	due   int64
	kind  TimerKind
	ref   uint64
	seq   uint64 // schedule order
	in    *Instance
	index int // in the rule clock's timer heap
}

func (t *timer) phase() int {
	if t.kind.strict() {
		return phaseStrict
	}
	return phaseInclusive
}

// before orders timers by (due, phase, key ordinal, kind, ref, schedule order).
func (t *timer) before(u *timer) bool {
	switch {
	case t.due != u.due:
		return t.due < u.due
	case t.phase() != u.phase():
		return t.phase() < u.phase()
	case t.in.birth != u.in.birth:
		return t.in.birth < u.in.birth
	case t.kind != u.kind:
		return t.kind < u.kind
	case t.ref != u.ref:
		return t.ref < u.ref
	}
	return t.seq < u.seq
}

type buffered struct {
	e  event.Event
	in *Instance
}

type timerKey struct {
	kind TimerKind
	ref  uint64
}

// RuleClock is one rule's construct clock on one shard, shared by its keys' instances, with their release buffer and
// timers in two heaps.
type RuleClock struct {
	inputs   []int
	contrib  []int64
	clock    int64
	ordinals uint64
	births   uint64
	seq      uint64
	events   []buffered // a min-heap in event.Compare order
	timers   []*timer   // a min-heap by timer.before
	fires    []Timer    // reused by the walk
	refs     []uint64   // reused by the walk
}

// NewRuleClock makes a rule clock reading the given streams; every contribution starts at −∞.
func NewRuleClock(inputs []int) *RuleClock {
	rc := &RuleClock{inputs: append([]int(nil), inputs...), contrib: make([]int64, len(inputs)), clock: math.MinInt64}
	for i := range rc.contrib {
		rc.contrib[i] = math.MinInt64
	}
	return rc
}

// ApplyAdvance takes a stream's new contribution; the clock is the clamped min over the inputs and never retreats.
func (rc *RuleClock) ApplyAdvance(a WMAdvance) (clock int64, advanced bool) {
	if !a.Moved {
		return rc.clock, false
	}
	found := false
	for i, sid := range rc.inputs {
		if sid == a.StreamID {
			rc.contrib[i] = max(rc.contrib[i], a.Contribution)
			found = true
		}
	}
	if !found {
		return rc.clock, false
	}
	m := int64(math.MaxInt64)
	for _, c := range rc.contrib {
		m = min(m, c)
	}
	if m <= rc.clock {
		return rc.clock, false
	}
	rc.clock = m
	return rc.clock, true
}

// Clock is the construct clock.
func (rc *RuleClock) Clock() int64 { return rc.clock }

// NewInstance makes a key's instance; ordinals count up from 0 and are never reused.
func (rc *RuleClock) NewInstance() *Instance {
	in := &Instance{rc: rc, ordinal: rc.ordinals, timers: map[timerKey]*timer{}}
	rc.ordinals++
	return in
}

// Free drops a key's instance. It must be Empty: the shard keeps an instance while it holds an event or a timer.
func (rc *RuleClock) Free(in *Instance) {
	if !in.Empty() {
		panic("clock: Free of an instance that still holds events or timers")
	}
	in.freed = true
}

// Advance walks every pending item up to the clock, forward in event time (owner D1): by instant; within one,
// events in event.Compare order, then the inclusive timers due, then — once the clock is past the instant — the
// strict ones. A timer a callback schedules joins the walk in order.
//
// Keys at one instant go in birth order: a key is born at its first released event or first timer, and born again
// after its purge's CancelAll (the reference executor's order, SEMANTICS §9). A clock still at −∞ releases nothing.
func (rc *RuleClock) Advance(cb Callbacks) {
	if rc.clock == math.MinInt64 {
		return
	}
	for {
		evAt, evOK := rc.nextEvent()
		tm := rc.nextTimer()
		switch {
		case evOK && (tm == nil || evAt <= tm.due):
			b := rc.popEvent()
			b.in.buffered--
			b.in.bear()
			if cb.OnEvent != nil {
				cb.OnEvent(b.in, b.e)
			}
		case tm != nil:
			rc.fireGroup(tm, cb)
		default:
			return
		}
	}
}

// nextEvent is the earliest buffered event's instant when it is released by the clock.
func (rc *RuleClock) nextEvent() (int64, bool) {
	if len(rc.events) == 0 || rc.events[0].e.Ts > rc.clock {
		return 0, false
	}
	return rc.events[0].e.Ts, true
}

// nextTimer is the earliest timer the clock has reached: an inclusive one at due ≤ clock, a strict one at due < clock.
func (rc *RuleClock) nextTimer() *timer {
	if len(rc.timers) == 0 {
		return nil
	}
	t := rc.timers[0]
	// a strict timer needs the clock past its due; +∞ (the end of input) is past every due
	if t.due > rc.clock || (t.kind.strict() && t.due == rc.clock && rc.clock != math.MaxInt64) {
		return nil
	}
	return t
}

// fireGroup fires the head timer and every timer of the same instant, phase, key and callback with it.
func (rc *RuleClock) fireGroup(head *timer, cb Callbacks) {
	in, due, phase, discard := head.in, head.due, head.phase(), head.kind == Discard
	rc.fires, rc.refs = rc.fires[:0], rc.refs[:0]
	for len(rc.timers) > 0 {
		t := rc.timers[0]
		if t.in != in || t.due != due || t.phase() != phase || (t.kind == Discard) != discard {
			break
		}
		rc.removeTimer(t)
		if discard {
			rc.refs = append(rc.refs, t.ref)
		} else {
			rc.fires = append(rc.fires, Timer{Due: t.due, Kind: t.kind, Ref: t.ref})
		}
	}
	if phase == phaseInclusive && (!in.hasSeal || due > in.sealed) { // owner D7: this key's instant is sealed
		in.sealed, in.hasSeal = due, true
	}
	switch {
	case discard && cb.OnDiscard != nil:
		cb.OnDiscard(in, rc.refs)
	case !discard && cb.OnTimers != nil:
		cb.OnTimers(in, rc.fires)
	}
}

// ---- the event heap ----

func (rc *RuleClock) pushEvent(b buffered) {
	rc.events = append(rc.events, b)
	i := len(rc.events) - 1
	for i > 0 {
		p := (i - 1) / 2
		if event.Compare(&rc.events[i].e, &rc.events[p].e) >= 0 {
			break
		}
		rc.events[i], rc.events[p] = rc.events[p], rc.events[i]
		i = p
	}
}

func (rc *RuleClock) popEvent() buffered {
	top := rc.events[0]
	last := len(rc.events) - 1
	rc.events[0] = rc.events[last]
	rc.events[last] = buffered{}
	rc.events = rc.events[:last]
	i := 0
	for {
		l, r, m := 2*i+1, 2*i+2, i
		if l < len(rc.events) && event.Compare(&rc.events[l].e, &rc.events[m].e) < 0 {
			m = l
		}
		if r < len(rc.events) && event.Compare(&rc.events[r].e, &rc.events[m].e) < 0 {
			m = r
		}
		if m == i {
			return top
		}
		rc.events[i], rc.events[m] = rc.events[m], rc.events[i]
		i = m
	}
}

// ---- the timer heap (indexed, so a timer can be cancelled) ----

func (rc *RuleClock) swapTimers(i, j int) {
	rc.timers[i], rc.timers[j] = rc.timers[j], rc.timers[i]
	rc.timers[i].index, rc.timers[j].index = i, j
}

func (rc *RuleClock) up(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if !rc.timers[i].before(rc.timers[p]) {
			return
		}
		rc.swapTimers(i, p)
		i = p
	}
}

func (rc *RuleClock) down(i int) {
	for {
		l, r, m := 2*i+1, 2*i+2, i
		if l < len(rc.timers) && rc.timers[l].before(rc.timers[m]) {
			m = l
		}
		if r < len(rc.timers) && rc.timers[r].before(rc.timers[m]) {
			m = r
		}
		if m == i {
			return
		}
		rc.swapTimers(i, m)
		i = m
	}
}

func (rc *RuleClock) pushTimer(t *timer) {
	t.index = len(rc.timers)
	rc.timers = append(rc.timers, t)
	rc.up(t.index)
}

func (rc *RuleClock) removeTimer(t *timer) {
	i, last := t.index, len(rc.timers)-1
	if i != last {
		rc.swapTimers(i, last)
	}
	rc.timers[last] = nil
	rc.timers = rc.timers[:last]
	if i != last {
		rc.down(i)
		rc.up(i)
	}
	delete(t.in.timers, keyOf(t.kind, t.ref))
	t.index = -1
}

// keyOf is a timer's identity on its key: (kind, ref), except the TTL timer, of which a key has exactly one
// (ADR-0004 §5).
func keyOf(k TimerKind, ref uint64) timerKey {
	if k == TTLPurge {
		return timerKey{kind: TTLPurge}
	}
	return timerKey{kind: k, ref: ref}
}

// Instance is one key's release buffer and timers under its rule clock.
type Instance struct {
	rc       *RuleClock
	ordinal  uint64
	birth    uint64 // the walk's order across keys; valid while born
	born     bool
	sealed   int64 // owner D7: the latest instant this key's inclusive timers fired, when hasSeal
	hasSeal  bool
	buffered int
	timers   map[timerKey]*timer
	freed    bool
}

// Ordinal is the instance's creation order on this rule clock: a stable label, never reused.
func (in *Instance) Ordinal() uint64 { return in.ordinal }

// bear gives an unborn key its place in the walk's order across keys.
func (in *Instance) bear() {
	if !in.born {
		in.birth, in.born = in.rc.births, true
		in.rc.births++
	}
}

// Buffer holds an admitted event until the walk releases it, or reports it construct-late: behind the rule's clock,
// or at or before an instant sealed for this key (owner D7). A construct-late event is not kept.
func (in *Instance) Buffer(e event.Event) (constructLate bool) {
	if e.Ts < in.rc.clock || (in.hasSeal && e.Ts <= in.sealed) {
		return true
	}
	in.rc.pushEvent(buffered{e: e, in: in})
	in.buffered++
	return false
}

// Schedule adds a timer; a second Schedule of the same (kind, ref) — of any TTLPurge — replaces it.
func (in *Instance) Schedule(due int64, k TimerKind, ref uint64) {
	key := keyOf(k, ref)
	if old := in.timers[key]; old != nil {
		in.rc.removeTimer(old)
	}
	in.bear()
	in.rc.seq++
	t := &timer{due: due, kind: k, ref: ref, seq: in.rc.seq, in: in}
	in.timers[key] = t
	in.rc.pushTimer(t)
}

// Cancel removes a pending timer; false when there was none.
func (in *Instance) Cancel(k TimerKind, ref uint64) bool {
	t := in.timers[keyOf(k, ref)]
	if t == nil || (k != TTLPurge && t.ref != ref) {
		return false
	}
	in.rc.removeTimer(t)
	return true
}

// CancelAll removes every pending timer of the key (the TTL purge's reset); its buffered events stay, and its next
// released event or timer is its new birth (a returning key starts fresh, SEMANTICS §9).
func (in *Instance) CancelAll() {
	for _, t := range in.timers {
		in.rc.removeTimer(t)
	}
	in.born = false
}

// Empty says the key holds nothing the shard would lose by freeing it: no buffered event, no timer, and no seal
// the clock has not yet passed (a sealed instant still drops a late event of the key, owner D7).
func (in *Instance) Empty() bool {
	return in.buffered == 0 && len(in.timers) == 0 && !(in.hasSeal && in.sealed >= in.rc.clock)
}
