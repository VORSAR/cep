// Package partition holds one rule's per-key state on one shard (ADR-0006 §2, §3): its instances, the TTL purge and
// its re-arm, the state registry and the timer-derivability invariant. It drives the clock's walk for its rule and
// hands each key's items to the rule body. Pure: no goroutines, no locks, no I/O; one goroutine uses a Rule.
// Design: docs/sprints/s3-2a-1-design.md §3.
package partition

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// OpState is a rule body's state for one key (ADR-0006 §2, extended for the derivability invariant).
type OpState interface {
	// Walk yields the state's parts: the Phase-2 snapshot seam.
	Walk(fn func(component, key string, state any))
	// Timers appends every timer this state implies, in any order (ADR-0006 §2(c)).
	Timers(dst []clock.Timer) []clock.Timer
}

// Body is a rule body's per-key logic: patterns (S4.1), windows (S8), test doubles now. A Body MUST NOT schedule
// clock.TTLPurge, and MUST NOT keep an Instance's clock past OnPurge.
type Body interface {
	// NewState is the key's state at its birth: its first released event, and again after a purge.
	NewState(in *Instance) OpState
	// OnEvent is a released event, after LastActivity and the TTL re-arm.
	OnEvent(in *Instance, e event.Event)
	// OnTimers is a group of inclusive timers, never TTLPurge; tables is the snapshot of the arrival whose walk
	// fired them (design note §2).
	OnTimers(in *Instance, fires []clock.Timer, tables any)
	// OnDiscard is a group of within-overrun discards.
	OnDiscard(in *Instance, refs []uint64)
	// OnPurge comes just before a TTL purge resets the key (a changelog window's retractions, S8).
	OnPurge(in *Instance)
}

// Instance is one key of one rule on one shard.
type Instance struct {
	Key          event.Value     // the first event's key value; a float zero is stored as +0.0 (break-it K1)
	Clock        *clock.Instance // the key's release buffer and timers
	Op           OpState         // nil until birth, and from a purge until rebirth
	LastActivity int64           // the event time of the key's last released event; math.MinInt64 before any
	Anchor       int64           // the rate limit's last emission time (SEMANTICS §8); meaningful when Anchored
	Anchored     bool            // cleared by every reset, a purge or a panic (design s3-2b-1 §3)

	pending bool // listed for freeing after a purge (once: review F10)
}

// Config describes one rule to its partitions.
type Config struct {
	Inputs  []int      // the stream ids the rule reads
	KeyType event.Type // the keys' one type (E-PART-003); TString for a key-less rule's pseudo-key ""
	TTL     int64      // ms, > 0; 0 only for a key-less rule, which is never purged
	Body    Body
	// OnArrival: the body sees each event at Offer, never buffered and never construct-late (owner Q1 of S3.2b;
	// design s3-2b-1 §3). Stub: not honoured yet.
	OnArrival bool
}

// Stats are a rule's plain counters on its shard; the runtime sums them on scrape.
type Stats struct {
	ConstructLate uint64
	Purged        uint64
	Panics        uint64 // a body's panics, each of which reset its key (owner F10 of S3.2a-2)
}

// InstanceState is what Rule.Walk yields, as a value, for one instance (component "instance", key its Ordinal in
// decimal), before the instance's OpState parts. Rule.Walk yields the rule clock first (component "clock", key "",
// a clock.RuleState).
type InstanceState struct {
	Key          event.Value
	LastActivity int64
	Birth        uint64        // the key's place in the walk's order across keys at one instant (clock.Instance.Birth)
	Born         bool          // false before its first released event or timer, and again after a purge
	Events       []event.Event // buffered, in event.Compare order
	Timers       []clock.Timer // pending, in walk order
}

// Rule is one rule's instances on one shard, under one construct clock.
type Rule struct {
	clock   *clock.RuleClock
	keyType event.Type
	ttl     int64
	body    Body
	byKey   map[keyT]*Instance
	byClock map[*clock.Instance]*Instance
	pending []*Instance // purged in a walk; freed after it once they hold nothing
	stats   Stats
	tables  any // the snapshot of the walk in progress (owner D5): what its timers read
	cb      clock.Callbacks
}

// keyT is a key value as a map key: the type's own field only, so a lookup allocates nothing, and float keys compare
// with ==, the expression engine's equality: -0.0 and +0.0 are one key.
type keyT struct {
	s string
	n int64
	f float64
	b bool
}

func keyOf(v event.Value) keyT {
	switch v.Kind {
	case event.TString:
		return keyT{s: v.Str}
	case event.TInt:
		return keyT{n: v.I}
	case event.TTimestamp:
		return keyT{n: v.T}
	case event.TDuration:
		return keyT{n: v.D}
	case event.TFloat:
		return keyT{f: v.F}
	case event.TBool:
		return keyT{b: v.B}
	}
	return keyT{}
}

// addSat is a + b for b ≥ 0, saturating at MaxInt64.
func addSat(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

var (
	errNoInputs = errors.New("partition: a rule reads at least one stream")
	errNilBody  = errors.New("partition: a rule needs a body")
	errNegTTL   = errors.New("partition: negative TTL")
)

// NewRule rejects an empty Inputs, a KeyType of TInvalid or TObject, a negative TTL and a nil Body. A TTL of 0 is
// accepted: the validator guarantees a keyed rule carries one (E-TTL-001), and a key-less rule is never purged.
func NewRule(cfg Config) (*Rule, error) {
	switch {
	case len(cfg.Inputs) == 0:
		return nil, errNoInputs
	case cfg.KeyType < event.TString || cfg.KeyType >= event.TObject:
		return nil, fmt.Errorf("partition: key type %v cannot key a partition (owner D7 of S3.2a-1: object keys are a deploy error from S3.3)", cfg.KeyType)
	case cfg.TTL < 0:
		return nil, errNegTTL
	case cfg.Body == nil:
		return nil, errNilBody
	}
	r := &Rule{
		clock: clock.NewRuleClock(cfg.Inputs), keyType: cfg.KeyType, ttl: cfg.TTL, body: cfg.Body,
		byKey: map[keyT]*Instance{}, byClock: map[*clock.Instance]*Instance{},
	}
	r.cb = clock.Callbacks{OnEvent: r.onEvent, OnTimers: r.onTimers, OnDiscard: r.onDiscard}
	return r, nil
}

// Apply takes one stream's advance without walking (design note §1, step 1); advanced says the rule's construct
// clock moved.
func (r *Rule) Apply(a clock.WMAdvance) (advanced bool) {
	_, advanced = r.clock.ApplyAdvance(a)
	return advanced
}

// Offer buffers e under key (design note §1, step 2), creating the key's instance when it has none. It panics on a
// null key, a key of another type or a NaN key: the router checks first.
func (r *Rule) Offer(key event.Value, e event.Event) (constructLate bool) {
	switch {
	case key.Null:
		panic("partition: Offer: a null key (the router makes it a per-rule event error)")
	case key.Kind != r.keyType:
		panic(fmt.Sprintf("partition: Offer: a %v key for a rule keyed by %v", key.Kind, r.keyType))
	case key.Kind == event.TFloat && key.F != key.F:
		panic("partition: Offer: a NaN key")
	}
	k := keyOf(key)
	in, resident := r.byKey[k]
	if !resident {
		if key.Kind == event.TFloat && key.F == 0 {
			key.F = 0 // -0.0 and +0.0 are one key, stored as +0.0, whichever arrives first (break-it K1)
		}
		in = &Instance{Key: key, Clock: r.clock.NewInstance(), LastActivity: math.MinInt64}
	}
	if in.Clock.Buffer(e) {
		r.stats.ConstructLate++
		if !resident {
			r.clock.Free(in.Clock)
		}
		return true
	}
	if !resident {
		r.byKey[k] = in
		r.byClock[in.Clock] = in
	}
	return false
}

// Advance walks everything before the clock (design note §1, step 3), then frees purged keys that hold nothing.
// tables is the snapshot the walk's timers read (owner D5).
func (r *Rule) Advance(tables any) {
	r.tables = tables
	r.clock.Advance(r.cb)
	r.tables = nil
	r.free()
}

func (r *Rule) onEvent(ci *clock.Instance, e event.Event) {
	in := r.byClock[ci]
	defer r.recoverBody(in)
	if in.Op == nil {
		in.Op = r.body.NewState(in)
	}
	in.LastActivity = e.Ts
	if r.ttl > 0 {
		ci.Schedule(addSat(e.Ts, r.ttl), clock.TTLPurge, 0)
	}
	r.body.OnEvent(in, e)
}

func (r *Rule) onTimers(ci *clock.Instance, fires []clock.Timer) {
	in := r.byClock[ci]
	// A key has one TTL timer, the only strict kind OnTimers receives, so it fires alone in its group.
	if fires[0].Kind == clock.TTLPurge {
		if len(fires) != 1 {
			panic("partition: a TTLPurge grouped with other timers")
		}
		r.stats.Purged++
		r.onPurge(in)
		r.reset(in)
		return
	}
	defer r.recoverBody(in)
	r.body.OnTimers(in, fires, r.tables)
}

func (r *Rule) onDiscard(ci *clock.Instance, refs []uint64) {
	in := r.byClock[ci]
	defer r.recoverBody(in)
	r.body.OnDiscard(in, refs)
}

// onPurge lets the body see a purged key first, with its state and its other timers (S8's retractions). A panic there
// is counted, and the purge still completes (owner F10 of S3.2a-2).
func (r *Rule) onPurge(in *Instance) {
	defer func() {
		if recover() != nil {
			r.stats.Panics++
		}
	}()
	r.body.OnPurge(in)
}

// recoverBody, deferred around a body call, turns the body's panic into a reset of its key (owner F10 of S3.2a-2):
// counted, and the walk goes on for every other key (review F1). The key's buffered events stay and start it fresh.
func (r *Rule) recoverBody(in *Instance) {
	if recover() != nil {
		r.stats.Panics++
		r.reset(in)
	}
}

// reset drops a key's state and timers, as a purge does (SEMANTICS §9); its buffered events stay and start it fresh,
// and it is freed after the walk if it holds nothing.
func (r *Rule) reset(in *Instance) {
	in.Clock.CancelAll()
	in.Op = nil
	in.LastActivity = math.MinInt64
	if !in.pending {
		in.pending = true
		r.pending = append(r.pending, in)
	}
}

// free drops each purged key that holds nothing; a reborn one leaves the list, one with buffered events stays.
func (r *Rule) free() {
	keep := r.pending[:0]
	for _, in := range r.pending {
		switch {
		case in.Op != nil:
			in.pending = false
		case in.Clock.Empty():
			in.pending = false
			r.clock.Free(in.Clock)
			delete(r.byKey, keyOf(in.Key))
			delete(r.byClock, in.Clock)
		default:
			keep = append(keep, in)
		}
	}
	clear(r.pending[len(keep):])
	r.pending = keep
}

// Clock is the rule's construct clock.
func (r *Rule) Clock() int64 { return r.clock.Clock() }

// Len is the number of resident instances.
func (r *Rule) Len() int { return len(r.byKey) }

// ordered is the resident instances in Ordinal order: a Go map's order never reaches an output.
func (r *Rule) ordered() []*Instance {
	ins := make([]*Instance, 0, len(r.byKey))
	for _, in := range r.byKey {
		ins = append(ins, in)
	}
	slices.SortFunc(ins, func(a, b *Instance) int { return cmp.Compare(a.Clock.Ordinal(), b.Clock.Ordinal()) })
	return ins
}

// Instances calls fn for each resident instance in Ordinal order.
func (r *Rule) Instances(fn func(in *Instance)) {
	for _, in := range r.ordered() {
		fn(in)
	}
}

// Walk yields the rule clock (component "clock", key "", a clock.RuleState), then, for each instance in Ordinal
// order, its InstanceState (component "instance", key its Ordinal in decimal) and its OpState's parts: a
// state-registry holder.
func (r *Rule) Walk(fn func(component, key string, state any)) {
	fn("clock", "", r.clock.State())
	for _, in := range r.ordered() {
		birth, born := in.Clock.Birth()
		fn("instance", strconv.FormatUint(in.Clock.Ordinal(), 10), InstanceState{
			Key: in.Key, LastActivity: in.LastActivity, Birth: birth, Born: born,
			Events: in.Clock.Events(nil), Timers: in.Clock.Timers(nil),
		})
		if in.Op != nil {
			in.Op.Walk(fn)
		}
	}
}

// CheckTimers checks the derivability invariant: each instance's pending timers equal, as a multiset of (Kind, Due,
// Ref), the TTL timer and its OpState's Timers when born, and nothing when unborn. It names the first difference.
func (r *Rule) CheckTimers() error {
	for _, in := range r.ordered() {
		got := in.Clock.Timers(nil)
		var want []clock.Timer
		if in.Op != nil {
			if r.ttl > 0 {
				want = append(want, clock.Timer{Due: addSat(in.LastActivity, r.ttl), Kind: clock.TTLPurge})
			}
			implied := in.Op.Timers(nil)
			for _, t := range implied {
				if t.Kind == clock.TTLPurge {
					return fmt.Errorf("partition: instance %d: its state implies a TTLPurge, which only partition schedules", in.Clock.Ordinal())
				}
			}
			want = append(want, implied...)
		}
		slices.SortFunc(got, cmpTimer)
		slices.SortFunc(want, cmpTimer)
		if !slices.Equal(got, want) {
			return fmt.Errorf("partition: instance %d: pending timers %v, but its state implies %v", in.Clock.Ordinal(), got, want)
		}
	}
	return nil
}

func cmpTimer(a, b clock.Timer) int {
	return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Due, b.Due), cmp.Compare(a.Ref, b.Ref))
}

// Stats are the rule's counters.
func (r *Rule) Stats() Stats { return r.stats }

// Pending is the number of pending timers over every resident key, TTL timers excepted: what a drain leaves unfired
// (drained_pending; design s3-2a-2 §1b). Cold path.
func (r *Rule) Pending() int {
	n := 0
	for _, in := range r.byKey { // a count: the map's order cannot show
		for _, t := range in.Clock.Timers(nil) {
			if t.Kind != clock.TTLPurge {
				n++
			}
		}
	}
	return n
}

// Holder is anything with durable state (ADR-0006 §2(b)).
type Holder interface {
	Walk(fn func(component, key string, state any))
}

// Registry lists an app's state holders in registration order.
type Registry struct {
	names   []string
	holders []Holder
}

// Register adds a holder; a duplicate name panics.
func (g *Registry) Register(name string, h Holder) {
	if slices.Contains(g.names, name) {
		panic("partition: Registry.Register: duplicate holder " + strconv.Quote(name))
	}
	g.names = append(g.names, name)
	g.holders = append(g.holders, h)
}

// Walk yields every holder's state, holders in registration order.
func (g *Registry) Walk(fn func(holder, component, key string, state any)) {
	for i, h := range g.holders {
		name := g.names[i]
		h.Walk(func(component, key string, state any) { fn(name, component, key, state) })
	}
}
