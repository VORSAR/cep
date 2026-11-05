package runtime

import (
	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/table"
)

// Sync is the replay drive loop (ADR-0005 §3; design s3-2b-1 §4): one app on one shard, every step run to completion
// on the caller's goroutine, through the same router event step and shard step as the live runtime. It starts no
// goroutine.
type Sync struct{}

// NewSync builds app on one shard. A fixture starts virtual at 0 (start is then ignored); out receives every output,
// in emission order, during the step that emits it. App.Sinks is not used.
func NewSync(app App, start int64, virtual bool, out func(e event.Event)) (*Sync, error) {
	panic("runtime: NewSync is a stub (S3.2b-1)")
}

// Begin is a recording's header: live ingest from start. It is legal only before the first event.
func (s *Sync) Begin(start int64) { panic("runtime: Sync.Begin is a stub (S3.2b-1)") }

// Event is one event step: the shared ingest step, the offers, the walks; late says the stream judged it late.
func (s *Sync) Event(e event.Event, arrival int64) (late bool, adv []clock.WMAdvance) {
	panic("runtime: Sync.Event is a stub (S3.2b-1)")
}

// Tick is a recording's idle tick.
func (s *Sync) Tick(now int64) []clock.WMAdvance { panic("runtime: Sync.Tick is a stub (S3.2b-1)") }

// TableWrite applies one table record at its position.
func (s *Sync) TableWrite(tableID int, op TableOp, rows [][]event.Value) error {
	panic("runtime: Sync.TableWrite is a stub (S3.2b-1)")
}

// Drain is a recording's stop record: the drain barrier, run to its end.
func (s *Sync) Drain() ([]clock.WMAdvance, DrainResult) {
	panic("runtime: Sync.Drain is a stub (S3.2b-1)")
}

// End is a fixture's end: every contribution to +∞.
func (s *Sync) End() []clock.WMAdvance { panic("runtime: Sync.End is a stub (S3.2b-1)") }

// Snapshot is the current table snapshot.
func (s *Sync) Snapshot() *table.Snapshot { panic("runtime: Sync.Snapshot is a stub (S3.2b-1)") }

// Registry is the replay's stream registry.
func (s *Sync) Registry() *clock.StreamRegistry { panic("runtime: Sync.Registry is a stub (S3.2b-1)") }

// Walk yields the ingest's and the shard's state holders.
func (s *Sync) Walk(fn func(component, key string, state any)) {
	panic("runtime: Sync.Walk is a stub (S3.2b-1)")
}

// Stats is the replay's counters, in the live runtime's shape (one shard).
func (s *Sync) Stats() Stats { panic("runtime: Sync.Stats is a stub (S3.2b-1)") }

// FromPlan is the runtime's App for p, with its tables; sinks are bound by the caller.
func FromPlan(p *plan.App, tables *table.Set, sinks []SinkBinding) App {
	panic("runtime: FromPlan is a stub (S3.2b-1)")
}
