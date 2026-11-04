// Package partition holds one rule's per-key state on one shard (ADR-0006 §2, §3): its instances, the TTL purge and
// its re-arm, the state registry and the timer-derivability invariant. It drives the clock's walk for its rule and
// hands each key's items to the rule body. Pure: no goroutines, no locks, no I/O; one goroutine uses a Rule.
// Design: docs/sprints/s3-2a-1-design.md §3.
package partition

import (
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
	Key          event.Value     // the first event's key value
	Clock        *clock.Instance // the key's release buffer and timers
	Op           OpState         // nil until birth, and from a purge until rebirth
	LastActivity int64           // the event time of the key's last released event; math.MinInt64 before any
}

// Config describes one rule to its partitions.
type Config struct {
	Inputs  []int      // the stream ids the rule reads
	KeyType event.Type // the keys' one type (E-PART-003); TString for a key-less rule's pseudo-key ""
	TTL     int64      // ms, > 0; 0 only for a key-less rule, which is never purged
	Body    Body
}

// Stats are a rule's plain counters on its shard; the runtime sums them on scrape.
type Stats struct {
	ConstructLate uint64
	Purged        uint64
}

// InstanceState is what Rule.Walk yields for one instance (component "instance", key its Ordinal in decimal),
// before the instance's OpState parts.
type InstanceState struct {
	Key          event.Value
	LastActivity int64
	Events       []event.Event // buffered, in event.Compare order
	Timers       []clock.Timer // pending, in walk order
}

// Rule is one rule's instances on one shard, under one construct clock.
type Rule struct{}

// NewRule rejects an empty Inputs, a KeyType of TInvalid or TObject, a negative TTL and a nil Body.
func NewRule(cfg Config) (*Rule, error) { panic("partition: NewRule: not implemented (S3.2a-1 stub)") }

// Apply takes one stream's advance without walking (design note §1, step 1).
func (r *Rule) Apply(a clock.WMAdvance) (advanced bool) {
	panic("partition: Rule.Apply: not implemented (S3.2a-1 stub)")
}

// Offer buffers e under key (design note §1, step 2), creating the key's instance when it has none. It panics on a
// null key or a key of another type: the router checks both first.
func (r *Rule) Offer(key event.Value, e event.Event) (constructLate bool) {
	panic("partition: Rule.Offer: not implemented (S3.2a-1 stub)")
}

// Advance walks everything up to the clock (design note §1, step 3), then frees purged keys that hold nothing.
// tables is the snapshot the walk's timers read.
func (r *Rule) Advance(tables any) { panic("partition: Rule.Advance: not implemented (S3.2a-1 stub)") }

// Clock is the rule's construct clock.
func (r *Rule) Clock() int64 { panic("partition: Rule.Clock: not implemented (S3.2a-1 stub)") }

// Len is the number of resident instances.
func (r *Rule) Len() int { panic("partition: Rule.Len: not implemented (S3.2a-1 stub)") }

// Instances calls fn for each resident instance in Ordinal order.
func (r *Rule) Instances(fn func(in *Instance)) {
	panic("partition: Rule.Instances: not implemented (S3.2a-1 stub)")
}

// Walk yields each instance's InstanceState, then its OpState's parts, in Ordinal order: a state-registry holder.
func (r *Rule) Walk(fn func(component, key string, state any)) {
	panic("partition: Rule.Walk: not implemented (S3.2a-1 stub)")
}

// CheckTimers checks the derivability invariant: each instance's pending timers equal, as a multiset of (Kind, Due,
// Ref), the TTL timer and its OpState's Timers when born, and nothing when unborn. It names the first difference.
func (r *Rule) CheckTimers() error {
	panic("partition: Rule.CheckTimers: not implemented (S3.2a-1 stub)")
}

// Stats are the rule's counters.
func (r *Rule) Stats() Stats { panic("partition: Rule.Stats: not implemented (S3.2a-1 stub)") }

// Holder is anything with durable state (ADR-0006 §2(b)).
type Holder interface {
	Walk(fn func(component, key string, state any))
}

// Registry lists an app's state holders in registration order.
type Registry struct{}

// Register adds a holder; a duplicate name panics.
func (g *Registry) Register(name string, h Holder) {
	panic("partition: Registry.Register: not implemented (S3.2a-1 stub)")
}

// Walk yields every holder's state, holders in registration order.
func (g *Registry) Walk(fn func(holder, component, key string, state any)) {
	panic("partition: Registry.Walk: not implemented (S3.2a-1 stub)")
}
