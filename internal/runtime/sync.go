package runtime

import (
	"fmt"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/table"
)

// Sync is the replay drive loop (ADR-0005 §3; design s3-2b-1 v2 §4): one app on one shard, every step run to
// completion on the caller's goroutine, through the live router's event step and the live shard's step — only the
// delivery differs. It starts no goroutine.
type Sync struct {
	r       *Runtime // built, never started
	rt      *router
	a       *appState
	sh      *shard
	out     func(e event.Event)
	pending int64 // the drain barrier's drained_pending, summed from the shard's ack
}

// NewSync builds app on one shard. A fixture starts virtual at 0 (start is then ignored); out receives every output,
// in emission order, during the step that emits it, and MUST NOT call back into the Sync. App.Sinks is not used.
func NewSync(app App, start int64, virtual bool, out func(e event.Event)) (*Sync, error) {
	if virtual {
		start = 0
	}
	r, err := build(Config{Shards: 1, Now: func() int64 { return start }}, []App{app})
	if err != nil {
		return nil, err
	}
	s := &Sync{r: r, a: r.apps[0], sh: r.shards[0], out: out}
	s.a.ing = NewIngest(app.Streams, start, app.Tables, virtual)
	s.rt = newRouter(r)
	s.rt.deliver = func(_ int, m message) {
		s.sh.process(m)
		s.flush()
	}
	return s, nil
}

// flush hands the shard's outbox to out in order and keeps a drain's ack.
func (s *Sync) flush() {
	for _, it := range s.sh.outbox {
		switch it.kind {
		case oEvent:
			s.out(it.ev)
		case oAck:
			s.pending += it.pending
		}
	}
	clear(s.sh.outbox)
	s.sh.outbox = s.sh.outbox[:0]
}

// Begin is a recording's header: live ingest from start. It is legal only before the first event.
func (s *Sync) Begin(start int64) {
	s.a.ing = NewIngest(s.a.app.Streams, start, s.a.app.Tables, false)
}

// Event is one event step: the shared ingest step (stamping e's Seq and Tables in place), the offers, the walks;
// late says its stream judged it late. adv is valid until the next step.
func (s *Sync) Event(e *event.Event, arrival int64) (late bool, adv []clock.WMAdvance) {
	if s.a.drained {
		s.a.after[e.StreamID]++
		return false, nil
	}
	return s.rt.event(s.a, e, arrival)
}

// Reject is a recording's rejected record (owner Q1 of S3.2b-2; design §6): it raises the stamp and counts the
// rejection, nothing else. Stub: not built yet (a no-op).
func (s *Sync) Reject(stream int, arrival int64, reason event.DecodeReason) {}

// Tick is a recording's idle tick.
func (s *Sync) Tick(now int64) []clock.WMAdvance {
	if s.a.drained {
		return nil
	}
	return s.rt.tickApp(s.a, now)
}

// TableWrite applies one table record at its position: the next event reads it, never the one before (F14).
func (s *Sync) TableWrite(tableID int, op TableOp, rows [][]event.Value) error {
	if s.a.drained {
		return ErrDrained
	}
	return s.a.write(tableID, op, rows)
}

// Drain is a recording's stop record: the drain barrier, run to its end; the app's input ends (owner F5).
func (s *Sync) Drain() ([]clock.WMAdvance, DrainResult) {
	if s.a.drained {
		return nil, DrainResult{Pending: s.pending}
	}
	adv := s.rt.drainApp(s.a, 0)
	return adv, DrainResult{Pending: s.pending}
}

// End is a fixture's end: every contribution to +∞, so every timer fires and every key purges.
func (s *Sync) End() []clock.WMAdvance {
	adv := s.a.ing.End()
	if len(adv) > 0 {
		s.rt.broadcast(s.a, mWalk, adv, s.a.ing.Snapshot(), 0, true)
	}
	return adv
}

// Snapshot is the current table snapshot.
func (s *Sync) Snapshot() *table.Snapshot { return s.a.ing.Snapshot() }

// Registry is the replay's stream registry: the one ingest step's (MUST 7).
func (s *Sync) Registry() *clock.StreamRegistry { return s.a.ing.Registry() }

// Walk yields the ingest step's Seq counters (component "seq"), as the driver's Walk always has.
func (s *Sync) Walk(fn func(component, key string, state any)) { s.a.ing.Walk(fn) }

// Stats is the replay's counters, in the live runtime's shape (one shard).
func (s *Sync) Stats() Stats {
	a := s.a
	rules := make([]RuleStats, len(a.app.Rules))
	sh := s.sh.stats()
	for ri := range rules {
		ps, es := sh.rules[0][ri], sh.emits[0][ri]
		rules[ri] = RuleStats{
			ConstructLate: int64(ps.ConstructLate), Purged: int64(ps.Purged), KeyErrors: a.keyErr[ri],
			Panics: int64(ps.Panics) + sh.panics[0][ri], Suppressed: int64(es.Suppressed), Errored: int64(es.Errored),
		}
	}
	return Stats{
		Streams: [][]StreamStats{streamStats(a)}, Recording: []RecordingStats{{}}, OutboxHigh: []int{0},
		StreamLate: [][]int64{append([]int64(nil), a.late...)}, AfterDrain: [][]int64{append([]int64(nil), a.after...)},
		Rules: [][]RuleStats{rules}, SinkErrors: [][]int64{make([]int64, len(a.all))},
		PendingHigh: 0, MailboxHigh: []int{0}, SaturatedParks: []int64{sh.parks},
	}
}

// FromPlan is the runtime's App for p; tables nil builds p's tables empty. Sinks are bound by the caller.
func FromPlan(p *plan.App, tables *table.Set, sinks []SinkBinding) App {
	a := App{Rules: p.Rules, Sinks: sinks, Tables: tables}
	for _, st := range p.Streams {
		a.Streams = append(a.Streams, clock.StreamSpec{Name: st.Schema.Stream(), Delay: st.Delay, IdleAfter: st.IdleAfter, Derived: st.Derived})
		a.Schemas = append(a.Schemas, st.Schema)
	}
	if tables == nil && len(p.Tables) > 0 {
		var ts []*table.Table
		for _, t := range p.Tables {
			tb, err := table.New(t.Name, t.Attrs, t.Key)
			if err != nil {
				panic(fmt.Sprintf("runtime: FromPlan: table %s: %v (a plan holds only checked tables)", t.Name, err))
			}
			ts = append(ts, tb)
		}
		a.Tables = table.NewSet(ts)
	}
	return a
}
