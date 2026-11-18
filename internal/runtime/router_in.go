package runtime

import (
	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

func (rt *router) ingest(it item) {
	switch it.kind {
	case iAdd, iRemove, iActivate, iQuiet, iDrain:
		rt.ingestLifecycle(it)
	case iBatchEnd:
		rt.ingestBatchEnd(it)
	case iEvent:
		rt.ingestEvent(it)
	case iReject:
		rt.ingestReject(it)
	case iTick:
		rt.tickApps(it.now)
	case iWrite:
		rt.writeTable(it)
	case iStats:
		rt.statsCut(it.reply)
	}
}

// inputApp is the app a stream input reaches: nil if the app is gone, and nil, the input counted as after the drain,
// if the router has drained it. It reads the router's own a.drained, never the caller's phase, so a batch end queued
// between Stop and the drain item is still counted and recorded (known issue S3.3-1-3).
func (rt *router) inputApp(id AppID, stream int) *appState {
	a := rt.app(id)
	if a == nil || a.ing == nil {
		return nil
	}
	if a.drained {
		a.after[stream]++
		return nil
	}
	return a
}

// ingestBatchEnd counts a batch end of a live app and queues its record.
func (rt *router) ingestBatchEnd(it item) {
	a := rt.inputApp(it.app, it.stream)
	if a == nil {
		return
	}
	a.batches[it.stream]++
	a.rec.put(recItem{kind: rBatchEnd, stream: it.stream, batch: it.batch})
}

// ingestEvent stamps an event of a live app, runs the ingest step, and queues its record.
func (rt *router) ingestEvent(it item) {
	r := rt.r
	a := rt.inputApp(it.app, it.ev.StreamID)
	if a == nil {
		return
	}
	stamp := rt.now(r.cfg.Now())
	if a.app.Schemas[it.ev.StreamID].TimeAttr() == event.ArrivalTime {
		it.ev.Ts = stamp // live stamping at ingest (item 3; design s3-2b-2 §4, MUST 6)
	}
	rt.event(a, &it.ev, stamp, it.info)
	// queued after the ingest step, still in intake order: the step writes the flag into a declared text_invisibles
	// slot, and the channel hand-off orders that write before the recorder goroutine reads the event (a race the
	// race row caught at T4)
	a.rec.put(recItem{kind: rEvent, ev: it.ev, arrival: stamp, payload: it.payload})
}

// ingestReject records a rejected input of a live app and counts it in the ingest step.
func (rt *router) ingestReject(it item) {
	r := rt.r
	a := rt.inputApp(it.app, it.stream)
	if a == nil {
		return
	}
	stamp := rt.now(r.cfg.Now())
	a.rec.put(recItem{kind: rRejected, stream: it.stream, arrival: stamp, fault: it.fault})
	a.ing.Reject(it.stream, stamp)
}

// tickApps ticks every activated app the router still drives, in app order.
func (rt *router) tickApps(now int64) {
	r := rt.r
	stamp := rt.now(now)
	for _, a := range r.appList() {
		// the clock starts at Start (owner, 2026-11-08)
		if a == nil || a.ing == nil || !a.activated || a.drained || a.quiet {
			continue
		}
		a.rec.put(recItem{kind: rTick, arrival: stamp}) // the raised stamp: what the ingest step uses (QA G6)
		rt.tickApp(a, stamp)
	}
}

// writeTable applies a table write to a live app and records each row written.
func (rt *router) writeTable(it item) {
	a := rt.app(it.app)
	if a == nil || a.ing == nil {
		it.reply <- reply{err: ErrRemoved}
		return
	}
	if a.drained {
		it.reply <- reply{err: ErrDrained}
		return
	}
	err := a.write(it.table, it.op, it.rows)
	if err == nil {
		for _, row := range copyRows(it.rows) { // one record per row: Records counts lines (break-it S1)
			a.rec.put(recItem{kind: rTable, table: it.table, op: it.op, rows: [][]event.Value{row}})
		}
	}
	it.reply <- reply{err: err}
}

// event is design §4: the ingest step (stamping e in place), the offers by key, and one message per shard, each a
// view delivered before the next step (design s3-2b-1 v2 §4). The caller checks the app is not drained and gives the
// arrival stamp. adv is the ingest step's, valid until its next call.
func (rt *router) event(a *appState, e *event.Event, arrival int64, info event.Decoded) (late bool,
	adv []clock.WMAdvance) {
	reg := a.ing.Registry()
	before := reg.Admission(e.StreamID)
	late, adv = a.ing.Event(e, arrival, info)
	if late {
		a.late[e.StreamID]++
		return true, adv
	}
	adm := reg.Admission(e.StreamID)
	tables, _ := e.Tables.(*table.Snapshot) // the event's own snapshot: its walk's timers read it too (D5)
	for i := range rt.offers {
		rt.offers[i] = rt.offers[i][:0]
	}
	for _, rr := range a.byStream[e.StreamID] {
		key, ok := routeKey(rr.attrs, e.Values)
		if !ok {
			a.keyErr[rr.rule]++ // a null or NaN part: this rule's key error only (SEMANTICS §9, §10)
			continue
		}
		sid := rt.shardOf(a.id, rr.rule, rr.attrs != nil, key)
		rt.offers[sid] = append(rt.offers[sid], offer{rule: rr.rule, key: key})
	}
	for sid := range rt.r.shards {
		if len(rt.offers[sid]) == 0 && len(adv) == 0 && adm == before {
			continue
		}
		m := message{kind: mWalk, app: a.id, adv: adv, tables: tables, admStream: e.StreamID, adm: adm}
		if len(rt.offers[sid]) > 0 {
			m.ev, m.offers = *e, rt.offers[sid]
		}
		rt.deliver(sid, m)
	}
	return false, adv
}

// tickApp is one app's idle tick, shared by the router and Sync (combined review C4 of S3.2b-1).
func (rt *router) tickApp(a *appState, stamp int64) []clock.WMAdvance {
	adv := a.ing.Tick(stamp)
	if len(adv) > 0 {
		rt.broadcast(message{kind: mWalk, app: a.id, adv: adv, tables: a.ing.Snapshot()})
	}
	return adv
}

// copyRows is a table write's rows as the record's own: the caller may reuse its slices once TableWrite returns.
func copyRows(rows [][]event.Value) [][]event.Value {
	out := make([][]event.Value, len(rows))
	for i, row := range rows {
		out[i] = append([]event.Value(nil), row...)
	}
	return out
}
