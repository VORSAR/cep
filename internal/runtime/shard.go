package runtime

import (
	"fmt"
	"math"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/table"
)

// ---- shards ----

type shard struct {
	id      int
	mailbox chan message
	rules   [][]*partition.Rule // [app][rule]
	bodies  [][]partition.Body  // [app][rule]: for their plan.Counted
	arrival [][]bool            // [app][rule]: an on-arrival rule (owner Q1 and D1 of S3.2b)
	reads   [][][]int           // [app][stream]: the rules reading it
	panics  [][]int64           // [app][rule]
	outbox  []outItem
	parks   int64
	burst   [][]int64 // [app][rule]: outputs emitted by the message being processed
	outHigh [][]int64 // [app][rule]: the most outputs one message made the rule emit
	touched [][2]int  // the (app, rule) pairs that emitted during the message: reset after it
}

func newShard(id, mailbox int) *shard { return &shard{id: id, mailbox: make(chan message, mailbox)} }

// grow makes the shard's per-app slices hold slot app (nil slots for ids never installed here).
func (sh *shard) grow(app AppID) {
	for int(app) >= len(sh.rules) {
		sh.rules, sh.bodies, sh.arrival = append(sh.rules, nil), append(sh.bodies, nil), append(sh.arrival, nil)
		sh.reads, sh.panics = append(sh.reads, nil), append(sh.panics, nil)
		sh.burst, sh.outHigh = append(sh.burst, nil), append(sh.outHigh, nil)
	}
}

// install builds the shard's slot for app (design s3-3-1 §2.4: each shard builds its own state on mAdd); an error
// leaves the slot freed.
func (sh *shard) install(app AppID, a *App) (err error) {
	sh.grow(app)
	defer func() {
		if err != nil {
			sh.uninstall(app)
		}
	}()
	ai := int(app)
	rules := make([]*partition.Rule, len(a.Rules))
	bodies := make([]partition.Body, len(a.Rules))
	arrival := make([]bool, len(a.Rules))
	reads := make([][]int, len(a.Streams))
	sh.burst[ai], sh.outHigh[ai] = make([]int64, len(a.Rules)), make([]int64, len(a.Rules))
	for ri, rule := range a.Rules {
		streams, name := a.Streams, rule.Name
		emit := func(e event.Event) {
			if e.StreamID < 0 || e.StreamID >= len(streams) || !streams[e.StreamID].Derived {
				// a body's fault: partition recovers it and resets the key (owner F10); the router never sees it
				// (break-it B1)
				panic(fmt.Sprintf( //nolint:forbidigo,lll // PERMANENT reason="a body emitting on a stream that is not derived is a fault partition recovers, resetting the key (owner F10)" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-5
					"runtime: rule %q emitted on stream %d, which is not a derived stream of app %d", name, e.StreamID, app))
			}
			sh.outbox = append(sh.outbox, outItem{kind: oEvent, app: app, ev: e})
			if sh.burst[ai][ri]++; sh.burst[ai][ri] == 1 {
				sh.touched = append(sh.touched, [2]int{ai, ri})
			}
		}
		body, err := newBody(rule, emit)
		if err != nil {
			return fmt.Errorf("runtime: app %d rule %d (%q): %w", app, ri, rule.Name, err)
		}
		pr, err := partition.NewRule(partition.Config{Inputs: rule.Inputs, KeyTypes: rule.KeyTypes, TTL: rule.TTL, Body: body,
			OnArrival: rule.OnArrival})
		if err != nil {
			return fmt.Errorf("runtime: app %d rule %d (%q): %w", app, ri, rule.Name, err)
		}
		rules[ri], bodies[ri], arrival[ri] = pr, body, rule.OnArrival
		for _, sid := range rule.Inputs {
			reads[sid] = append(reads[sid], ri)
		}
	}
	sh.rules[ai], sh.bodies[ai], sh.arrival[ai], sh.reads[ai] = rules, bodies, arrival, reads
	sh.panics[ai] = make([]int64, len(rules))
	return nil
}

// uninstall frees the slot; indexes never shift (design s3-3-1 §2.4).
func (sh *shard) uninstall(app AppID) {
	sh.grow(app)
	ai := int(app)
	sh.rules[ai], sh.bodies[ai], sh.arrival[ai], sh.reads[ai] = nil, nil, nil, nil
	sh.panics[ai], sh.burst[ai], sh.outHigh[ai] = nil, nil, nil
}

// newBody calls a rule's NewBody; its panic is Start's error, never its caller's panic.
func newBody(rule Rule, emit Emit) (b partition.Body, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("NewBody panicked: %v", p)
		}
	}()
	return rule.NewBody(emit), nil
}

// run is a shard's loop: it waits only on its mailbox and, with a non-empty outbox, the handoff (design §3).
func (sh *shard) run(r *Runtime) {
	defer r.wg.Done()
	for {
		if r.stopping() {
			return
		}
		var ho chan handoff
		if len(sh.outbox) > 0 {
			ho = r.handoff
			if len(sh.mailbox) == cap(sh.mailbox) {
				sh.parks++ // the moment exit 3 must reach: a full mailbox while the handoff is pending (review F16)
			}
		}
		select {
		case m := <-sh.mailbox:
			sh.process(m)
		case ho <- handoff{shard: sh.id, items: sh.outbox}:
			sh.outbox = nil
		case <-r.stopped:
			return
		}
	}
}

// process is design §4's shard step: apply, walk the on-arrival rules offered, offer, then walk every rule of the app
// with the message's snapshot (design s3-2b-1 v2 §3). An on-arrival rule's clock takes only its stream's admission
// watermark, and a drain's or the end's advances (owner D1 of S3.2b-1).
func (sh *shard) process(m message) {
	defer sh.foldBursts()
	switch m.kind {
	case mStats:
		sh.outbox = append(sh.outbox, outItem{kind: oStats, token: m.token, stats: sh.stats()})
		return
	case mAdd:
		err := sh.install(m.app, m.add)
		sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token, err: err})
		return
	case mRemove:
		sh.uninstall(m.app)
		sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token})
		return
	}
	rules, arrival := sh.rules[m.app], sh.arrival[m.app]
	if rules == nil { // a message for a slot this shard freed (a remove raced a drain's last advances): nothing to walk
		if m.kind == mDrain {
			sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token})
		}
		return
	}
	for _, a := range m.adv {
		for _, ri := range sh.reads[m.app][a.StreamID] {
			if !arrival[ri] || m.final {
				rules[ri].Apply(a)
			}
		}
	}
	if m.admStream >= 0 && m.adm != math.MinInt64 {
		adv := clock.WMAdvance{StreamID: m.admStream, Contribution: m.adm, Moved: true}
		for _, ri := range sh.reads[m.app][m.admStream] {
			if arrival[ri] {
				rules[ri].Apply(adv)
			}
		}
	}
	for _, o := range m.offers {
		if arrival[o.rule] { // a purge that fell due before this event comes first (design s3-2b-1 v2 §3)
			sh.advance(m.app, o.rule, m.tables)
		}
	}
	for _, o := range m.offers {
		sh.offer(m.app, o, m.ev)
	}
	for ri := range rules {
		sh.advance(m.app, ri, m.tables)
	}
	if m.kind == mDrain {
		var pending int64
		for _, pr := range rules {
			pending += int64(pr.Pending())
		}
		sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token, pending: pending})
	}
}

// foldBursts ends a message: each rule's outputs from it become its high-water mark if higher (ADR-0006 §5).
func (sh *shard) foldBursts() {
	for _, p := range sh.touched {
		sh.outHigh[p[0]][p[1]] = max(sh.outHigh[p[0]][p[1]], sh.burst[p[0]][p[1]])
		sh.burst[p[0]][p[1]] = 0
	}
	sh.touched = sh.touched[:0]
}

func (sh *shard) offer(app AppID, o offer, e event.Event) {
	defer func() {
		if recover() != nil {
			sh.panics[app][o.rule]++
		}
	}()
	sh.rules[app][o.rule].Offer(o.key, e)
}

// advance walks one rule; a panic partition did not catch is the shard boundary's last resort: counted, and the walk
// is resumed until it returns (each panic consumed one item; review F1), with a cap against a panic that consumes none.
func (sh *shard) advance(app AppID, ri int, tables *table.Snapshot) {
	for range 1 << 16 {
		if sh.tryAdvance(app, ri, tables) {
			return
		}
	}
}

func (sh *shard) tryAdvance(app AppID, ri int, tables *table.Snapshot) (done bool) {
	defer func() {
		if recover() != nil {
			sh.panics[app][ri]++
		}
	}()
	sh.rules[app][ri].Advance(tables)
	return true
}
