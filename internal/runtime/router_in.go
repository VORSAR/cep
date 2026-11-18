package runtime

import (
	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

func (rt *router) ingest(it item) {
	r := rt.r
	switch it.kind {
	case iAdd:
		rt.add(it)
	case iRemove:
		rt.remove(it)
	case iActivate:
		a := rt.app(it.app)
		switch {
		case a == nil || a.ing == nil || a.removing:
			it.reply <- reply{err: ErrRemoved}
		case a.drained:
			it.reply <- reply{err: ErrDrained}
		case a.activated:
			it.reply <- reply{err: ErrStarted}
		default:
			a.activated = true
			a.started.Store(true)
			it.reply <- reply{}
		}
	case iQuiet:
		if a := rt.app(it.app); a != nil {
			a.quiet = true // the ticks stop before the sources (S3.1-1's D3; design s3-3-1 §2.6)
		}
	case iBatchEnd:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			return
		}
		if a.drained {
			a.after[it.stream]++
			return
		}
		a.batches[it.stream]++
		a.rec.put(recItem{kind: rBatchEnd, stream: it.stream, batch: it.batch})
	case iEvent:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			return
		}
		if a.drained {
			a.after[it.ev.StreamID]++
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
	case iReject:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			return
		}
		if a.drained {
			a.after[it.stream]++
			return
		}
		stamp := rt.now(r.cfg.Now())
		a.rec.put(recItem{kind: rRejected, stream: it.stream, arrival: stamp, fault: it.fault})
		a.ing.Reject(it.stream, stamp)
	case iTick:
		stamp := rt.now(it.now)
		for _, a := range r.appList() {
			// the clock starts at Start (owner, 2026-11-08)
			if a == nil || a.ing == nil || !a.activated || a.drained || a.quiet {
				continue
			}
			a.rec.put(recItem{kind: rTick, arrival: stamp}) // the raised stamp: what the ingest step uses (QA G6)
			rt.tickApp(a, stamp)
		}
	case iWrite:
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
	case iDrain:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			it.reply <- reply{err: ErrRemoved}
			return
		}
		if a.drained { // a retried Drain waits for the same flush (owner, 2026-11-05)
			if a.drainRes != nil {
				it.reply <- *a.drainRes
			} else {
				d := rt.drains[a.drainTok]
				d.replies = append(d.replies, it.reply)
			}
			return
		}
		if !a.activated { // never activated: nothing was delivered, nothing recorded (design s3-3-1 §2.4, MUST 8)
			a.drained = true // the queued header is never written: no recorder goroutine runs (a.rec is a caller's to read)
			res := reply{}
			a.drainRes = &res
			it.reply <- res
			return
		}
		a.drained = true
		rt.token++
		a.drainTok = rt.token
		// The drain waits until the stop is written (design s3-2b-2 §7), or, for a recording that ended, until the
		// records queued before its end are (owner D2 of S3.2b-2's design; review C3).
		dw := &drainWait{app: a, replies: []chan reply{it.reply}, recMarker: a.rec.drainStop()}
		rt.drains[rt.token] = dw
		rt.drainApp(a, rt.token)
	case iStats:
		rt.token++
		st := Stats{PendingHigh: rt.pendingHigh, MailboxHigh: append([]int(nil), rt.mailboxHigh...)}
		for _, a := range r.appList() {
			if a == nil { // never installed here
				st.Streams, st.Recording, st.Rules = append(st.Streams, nil), append(st.Recording, RecordingStats{}), append(
					st.Rules, nil)
				continue
			}
			st.Streams = append(st.Streams, streamStats(a)) // a removed app: zero rows of its length (MUST 2)
			st.Recording = append(st.Recording, a.rec.stats())
			rules := make([]RuleStats, len(a.app.Rules))
			for i := range rules {
				rules[i].KeyErrors = a.keyErr[i]
			}
			st.Rules = append(st.Rules, rules)
		}
		st.SaturatedParks = make([]int64, len(r.shards))
		rt.stats[rt.token] = &statsWait{stats: st, reply: it.reply}
		for sid := range r.shards {
			rt.push(send{shard: sid, msg: message{kind: mStats, token: rt.token}})
		}
	}
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
		rt.broadcast(a, mWalk, adv, a.ing.Snapshot(), 0, false)
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
