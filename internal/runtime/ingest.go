package runtime

import (
	"math"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
	"github.com/vorsar/cep/internal/text"
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
	stamp   int64 // the monotone wall stamp of arrivals and ticks (design s3-2b-1 v2 §4, review F7)
	buf     []clock.WMAdvance
	counts  []streamCounts // per stream (design s3-2b-2 §6)
	flags   []int          // per stream: its declared text_invisibles slot, or -1; nil: none known (break-it B1)
}

// streamCounts are one stream's intake counters the ingest step keeps, live and in replay alike.
type streamCounts struct {
	rejected, unknown, overrides int64
}

// NewIngest: start is the deployment's start (wall ms); virtual is playback's fixture mode (the virtual floor after
// every admitted event). A nil set means no tables. The set may be shared with its owner: Ingest only takes snapshots.
// Live and in a recording the stamp starts at start, so no arrival or tick falls below it (review F5, MUST 15); a
// fixture's stamps are its events' times.
func NewIngest(specs []clock.StreamSpec, start int64, set *table.Set, virtual bool) *Ingest {
	if set == nil {
		set = table.NewSet(nil)
	}
	stamp := start
	if virtual {
		stamp = math.MinInt64
	}
	return &Ingest{
		specs: specs, reg: clock.NewStreamRegistry(specs, start), set: set, seq: make([]uint64, len(specs)),
		vnow: math.MinInt64, virtual: virtual, stamp: stamp, counts: make([]streamCounts, len(specs)),
	}
}

// Event stamps e (Seq per stream from 1, a stream-late event included; Tables = the current snapshot; the
// invisible-character flag, computed here and nowhere else: design s3-2b-2 §5, MUST 7), counts its decode report (an
// unknown key, a payload's own text_invisibles: §6), then judges it with the registry; arrival is first raised to the
// last stamp, as the router's is. The advances are its own stream's, then (virtual) the virtual floors; valid until
// the next call.
func (g *Ingest) Event(e *event.Event, arrival int64, info event.Decoded) (late bool, advances []clock.WMAdvance) {
	g.stamp = max(g.stamp, arrival)
	arrival = g.stamp
	c := &g.counts[e.StreamID]
	c.unknown += int64(info.Unknown)
	if info.Reserved {
		c.overrides++
	}
	e.Invisibles = text.IngestFlags(e.Values)
	if g.flags != nil && g.flags[e.StreamID] >= 0 { // a declared slot holds the engine's flag, so a key on it is the flag
		e.Values[g.flags[e.StreamID]] = event.Value{Kind: event.TBool, B: e.Invisibles}
	}
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

// Reject is a payload that failed its stream's schema (owner Q1 of S3.2b-2; design §6, MUST 8): it raises the stamp,
// as an arrival does, and is counted; it takes no Seq, reaches no clock and no rule.
func (g *Ingest) Reject(stream int, arrival int64) {
	g.stamp = max(g.stamp, arrival)
	g.counts[stream].rejected++
}

// Tick is a live idle tick (ObserveTick).
func (g *Ingest) Tick(now int64) []clock.WMAdvance {
	g.stamp = max(g.stamp, now)
	g.buf = g.reg.ObserveTick(g.stamp, g.buf[:0])
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

// withFlagSlots records each stream's declared text_invisibles slot (break-it B1): the ingest step writes the engine's
// flag there, so whatever reads the slot — a partition key, the recording's omission — sees the one flag, live and in
// replay alike.
func (g *Ingest) withFlagSlots(schemas []event.Schema) *Ingest {
	g.flags = make([]int, len(schemas))
	for sid, sch := range schemas {
		g.flags[sid] = -1
		for i := range sch.NumAttrs() {
			if sch.Attr(i).Name == event.InvisiblesAttr {
				g.flags[sid] = i
			}
		}
	}
	return g
}
