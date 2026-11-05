package runtime

import (
	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// Ingest is one app's ingest arithmetic, used by one goroutine: the live router, or the playback driver (owner,
// S3.2a-2's plan check; design §1).
type Ingest struct{}

// NewIngest: start is the deployment's start (wall ms); virtual is playback's fixture mode (the virtual floor after
// every admitted event). A nil set means no tables.
func NewIngest(specs []clock.StreamSpec, start int64, set *table.Set, virtual bool) *Ingest {
	panic("runtime: NewIngest: not implemented (S3.2a-2 stub)")
}

// Event stamps e (Seq per stream from 1, a stream-late event included; Tables = the current snapshot), then judges it
// with the registry. The advances are its own stream's, then (virtual) the virtual floors; valid until the next call.
func (g *Ingest) Event(e *event.Event, arrival int64) (late bool, advances []clock.WMAdvance) {
	panic("runtime: Ingest.Event: not implemented (S3.2a-2 stub)")
}

// Tick is a live idle tick (ObserveTick).
func (g *Ingest) Tick(now int64) []clock.WMAdvance {
	panic("runtime: Ingest.Tick: not implemented (S3.2a-2 stub)")
}

// Drain is the registry's Drain: one past the largest admitted event time.
func (g *Ingest) Drain() []clock.WMAdvance {
	panic("runtime: Ingest.Drain: not implemented (S3.2a-2 stub)")
}

// End is the end of input: every contribution to +∞.
func (g *Ingest) End() []clock.WMAdvance {
	panic("runtime: Ingest.End: not implemented (S3.2a-2 stub)")
}

// Snapshot is the current table snapshot: what a walk this step triggers reads (owner D5 of S3.2a-1).
func (g *Ingest) Snapshot() *table.Snapshot {
	panic("runtime: Ingest.Snapshot: not implemented (S3.2a-2 stub)")
}

// Registry is the app's stream registry.
func (g *Ingest) Registry() *clock.StreamRegistry {
	panic("runtime: Ingest.Registry: not implemented (S3.2a-2 stub)")
}

// Walk yields the Seq counters (component "seq", key the stream name): a state holder.
func (g *Ingest) Walk(fn func(component, key string, state any)) {
	panic("runtime: Ingest.Walk: not implemented (S3.2a-2 stub)")
}
