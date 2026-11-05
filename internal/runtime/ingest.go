package runtime

import (
	"math"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// Ingest is one app's ingest arithmetic, used by one goroutine: the live router, or the playback driver (owner,
// S3.2a-2's plan check; design §1). It is pure: no goroutines, no clock reads.
type Ingest struct {
	specs   []clock.StreamSpec
	reg     *clock.StreamRegistry
	set     *table.Set
	seq     []uint64
	vnow    int64
	virtual bool
	buf     []clock.WMAdvance
}

// NewIngest: start is the deployment's start (wall ms); virtual is playback's fixture mode (the virtual floor after
// every admitted event). A nil set means no tables. The set may be shared with its owner: Ingest only takes snapshots.
func NewIngest(specs []clock.StreamSpec, start int64, set *table.Set, virtual bool) *Ingest {
	if set == nil {
		set = table.NewSet(nil)
	}
	return &Ingest{
		specs: specs, reg: clock.NewStreamRegistry(specs, start), set: set, seq: make([]uint64, len(specs)),
		vnow: math.MinInt64, virtual: virtual,
	}
}

// Event stamps e (Seq per stream from 1, a stream-late event included; Tables = the current snapshot), then judges it
// with the registry. The advances are its own stream's, then (virtual) the virtual floors; valid until the next call.
func (g *Ingest) Event(e *event.Event, arrival int64) (late bool, advances []clock.WMAdvance) {
	g.seq[e.StreamID]++
	e.Seq = g.seq[e.StreamID]
	e.Tables = g.set.Snapshot()
	late, a := g.reg.ObserveEvent(e.StreamID, e.Ts, arrival)
	g.buf = g.buf[:0]
	if late {
		return true, g.buf
	}
	if a.Moved {
		g.buf = append(g.buf, a)
	}
	if g.virtual {
		g.vnow = max(g.vnow, e.Ts)
		g.buf = g.reg.ObserveVirtual(g.vnow, g.buf)
	}
	return false, g.buf
}

// Tick is a live idle tick (ObserveTick).
func (g *Ingest) Tick(now int64) []clock.WMAdvance {
	g.buf = g.reg.ObserveTick(now, g.buf[:0])
	return g.buf
}

// Drain is the registry's Drain: one past the largest admitted event time.
func (g *Ingest) Drain() []clock.WMAdvance {
	g.buf = g.reg.Drain(g.buf[:0])
	return g.buf
}

// End is the end of input: every contribution to +∞.
func (g *Ingest) End() []clock.WMAdvance {
	g.buf = g.reg.EndOfInput(g.buf[:0])
	return g.buf
}

// Snapshot is the current table snapshot: what a walk this step triggers reads (owner D5 of S3.2a-1).
func (g *Ingest) Snapshot() *table.Snapshot { return g.set.Snapshot() }

// Registry is the app's stream registry.
func (g *Ingest) Registry() *clock.StreamRegistry { return g.reg }

// Walk yields the Seq counters (component "seq", key the stream name), in declaration order: a state holder.
func (g *Ingest) Walk(fn func(component, key string, state any)) {
	for i, s := range g.specs {
		fn("seq", s.Name, g.seq[i])
	}
}
