package playback

import (
	"fmt"
	"slices"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

// Counts are a block's drops, read from the sandbox's Stats after the end (SEMANTICS §10; design s3-3-2 §5.1).
type Counts struct {
	Late      int64 // given events dropped as stream-late
	Errored   int64 // events an expression error dropped
	KeyErrors int64 // events a key error dropped
	Panics    int64 // the rule's recovered panics
}

// BlockResult is one expect block's run.
type BlockResult struct {
	Rule       string
	Block      int           // 1-based within the rule
	Outputs    []event.Event // the rule's outputs, in emission order
	Counts     Counts
	Divergence string // "" when the block passed; otherwise design s3-3-2 §5.4's text
	Line, Col  int    // where E-EXP-001 points (§5.4); 0, 0 when the block passed
}

// RunExpects runs every block of p, in p.Expects' order, each in a fresh sandbox: the owning rule alone, its own
// empty tables, no sinks, no recorder, no connectors. It runs on the caller's goroutine and is deterministic. The
// error is an engine fault (a sandbox that cannot be built, a table that refuses a row, a panic outside the rule's
// recovered body), never a block that failed.
func RunExpects(p *plan.App) ([]BlockResult, error) {
	out := make([]BlockResult, 0, len(p.Expects))
	for i := range p.Expects {
		r, err := runBlock(p, &p.Expects[i])
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// runBlock is one block in a fresh sandbox (design s3-3-2 §5.2).
func runBlock(p *plan.App, x *plan.Expect) (res BlockResult, err error) {
	rule := p.Rules[x.Rule]
	fault := func(format string, args ...any) error {
		return fmt.Errorf("playback: rule %s, expect block %d: "+format, append([]any{rule.Name, x.Block}, args...)...)
	}
	defer func() {
		if v := recover(); v != nil { // never the caller's panic (the runtime's precedent)
			res, err = BlockResult{}, fault("a panic: %v", v)
		}
	}()
	sub := &plan.App{Streams: p.Streams, Tables: p.Tables, Rules: []plan.Rule{rule}}
	var outs []event.Event
	d, err := NewPlanDriver(sub, func(e event.Event) { outs = append(outs, e) })
	if err != nil {
		return BlockResult{}, fault("%w", inPlayback{err})
	}
	if tid, err := seedGivenTables(d, x); err != nil {
		return BlockResult{}, fault("given table %s: %w", p.Tables[tid].Name, err)
	}
	feedGivenLists(d, x)
	d.End() // the playback end: every contribution to +∞ (SEMANTICS §2)
	res = BlockResult{Rule: rule.Name, Block: x.Block, Outputs: outs}
	res.Counts = blockCounts(d.Stats(), x)
	res.Divergence, res.Line, res.Col = diverge(p.Streams[rule.Target].Schema, x, outs, res.Counts)
	return res, nil
}

// inPlayback is a playback error wrapped by runBlock's fault, which already says "playback: ": its text without
// that prefix, the error itself still in the chain.
type inPlayback struct{ err error }

func (e inPlayback) Error() string { return strings.TrimPrefix(e.err.Error(), "playback: ") }
func (e inPlayback) Unwrap() error { return e.err }

// seedGivenTables writes every given table's rows into the sandbox, in written order; the table that refuses a row and
// TableWrite's own error, 0 and nil when every row is written.
func seedGivenTables(d *Driver, x *plan.Expect) (table int, err error) {
	for _, gt := range x.Tables { // before any event, wherever the section is written (SPEC §6.8)
		for _, row := range gt.Rows {
			if err := d.sync.TableWrite(gt.Table, runtime.TablePut, [][]event.Value{slices.Clone(row)}); err != nil {
				return gt.Table, err
			}
		}
	}
	return 0, nil
}

// feedGivenLists steps every given list's events into the sandbox, merged by time.
func feedGivenLists(d *Driver, x *plan.Expect) {
	// The lists merged by time, each in its written order, ties to the list written first (owner, 2026-11-07, 4).
	next := make([]int, len(x.Lists))
	for {
		best := nextGiven(x, next)
		if best < 0 {
			break
		}
		l := &x.Lists[best]
		ev := l.Events[next[best]]
		next[best]++
		d.Step(Record{Kind: RecEvent, Event: event.Event{StreamID: l.Stream, Ts: ev.Ts, Values: slices.Clone(ev.Values)}})
	}
}

// nextGiven is the list whose next event is the earliest, ties to the list written first; -1 when every list is
// stepped to its end.
func nextGiven(x *plan.Expect, next []int) int {
	best := -1
	for li, l := range x.Lists {
		if next[li] < len(l.Events) && (best < 0 || l.Events[next[li]].Ts < x.Lists[best].Events[next[best]].Ts) {
			best = li
		}
	}
	return best
}

// blockCounts is the block's drops: each given stream's stream-late count once, and the rule's errored, key-error
// and panic counts.
func blockCounts(st runtime.Stats, x *plan.Expect) (c Counts) {
	given := map[int]bool{}
	for _, l := range x.Lists {
		if !given[l.Stream] {
			given[l.Stream] = true
			c.Late += st.Streams[0][l.Stream].Late
		}
	}
	rs := st.Rules[0][0]
	c.Errored, c.KeyErrors, c.Panics = rs.Errored, rs.KeyErrors, rs.Panics
	return c
}
