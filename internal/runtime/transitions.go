package runtime

import (
	"fmt"

	"github.com/vorsar/cep/internal/clock"
)

// ---- the router's side ----

// ingestLifecycle takes an app's lifecycle item: add, remove, activate, quiet or drain.
func (rt *router) ingestLifecycle(it item) {
	switch it.kind {
	case iAdd:
		rt.add(it)
	case iRemove:
		rt.remove(it)
	case iActivate:
		rt.activate(it)
	case iQuiet:
		if a := rt.app(it.app); a != nil {
			a.quiet = true // the ticks stop before the sources (S3.1-1's D3; design s3-3-1 §2.6)
		}
	case iDrain:
		rt.beginDrain(it)
	}
}

// add installs an app (design s3-3-1 §2.4): the state built with the router's stamp, published without routes, then
// mAdd to every shard under a token; take completes it.
func (rt *router) add(it item) {
	r := rt.r
	if r.addHold != nil { // the package's tests only
		r.addHold()
	}
	if r.stopping() { // Close stopped the runtime meanwhile: install nothing (the break-it's Q-L3)
		it.reply <- reply{err: ErrClosed}
		return
	}
	start := rt.now(r.cfg.Now())
	apps := r.appList()
	a := r.newAppState(AppID(len(apps)), *it.add, start)
	next := append(append([]*appState(nil), apps...), a)
	r.apps.Store(&next)
	rt.token++
	rt.installs[rt.token] = &installWait{app: a, reply: it.reply}
	for sid := range r.shards {
		rt.push(send{shard: sid, msg: message{kind: mAdd, app: a.id, token: rt.token, add: &a.app}})
	}
}

// remove unregisters an app: the drain must be complete; the routes are dropped at once, mRemove goes to every shard
// under a token, and take pushes the last markers once every shard acked.
func (rt *router) remove(it item) {
	r := rt.r
	a := rt.app(it.app)
	if a == nil || a.ing == nil || a.removing {
		it.reply <- reply{err: ErrRemoved} // a second removal, or one racing Close (break-it B1)
		return
	}
	// An activated app is removed only once its drain is complete, so no event, tick or drain walk of the app follows
	// this step. Two releases rely on that: the recorder's release of its queue at its stop (no such walk's record is
	// queued after the stop), and the tables' rows being unreachable when Remove returns (no snapshot of them is taken
	// after this step, so every walk that holds one is ahead of mRemove on its shard). Release's safety does not rely
	// on it (completeRemove says why).
	if a.activated && !(a.drained && a.drainRes != nil) {
		it.reply <- reply{err: fmt.Errorf("%w (app %d)", ErrDrainIncomplete, it.app)}
		return
	}
	a.removing = true // the router's: a later Activate or Remove of the app is refused (break-it B1, B2)
	for sid := range a.byStream {
		a.byStream[sid] = nil
	}
	rt.token++
	rt.installs[rt.token] = &installWait{app: a, reply: it.reply, remove: true}
	for sid := range r.shards {
		rt.push(send{shard: sid, msg: message{kind: mRemove, app: a.id, token: rt.token}})
	}
}

// activate starts an app's clock and delivery, or answers why it cannot.
func (rt *router) activate(it item) {
	a := rt.app(it.app)
	switch {
	case a == nil || a.ing == nil || a.removing:
		it.reply <- reply{err: ErrRemoved}
	case a.drained:
		it.reply <- reply{err: ErrDrained}
	case a.activated:
		it.reply <- reply{err: ErrStarted}
	default:
		if err := rt.applyLoads(a); err != nil { // checked at Add: only a broken invariant reaches here
			it.reply <- reply{err: err}
			return
		}
		a.activated = true
		a.started.Store(true)
		it.reply <- reply{}
	}
}

// applyLoads puts the app's starting rows (App.Loads) table by table in load order and records them as table
// writes; Add checked them, so it returns nil unless that check and the table disagree.
func (rt *router) applyLoads(a *appState) error {
	for _, l := range a.app.Loads {
		if err := a.applyWrite(l.Table, TablePut, l.Rows); err != nil {
			return err
		}
	}
	return nil
}

// installAck is one shard's ack of an mAdd or mRemove; the last one completes the item.
func (rt *router) installAck(w *installWait, it outItem) {
	r := rt.r
	w.acks++
	if it.err != nil && w.err == nil {
		w.err = it.err
	}
	if w.acks < len(r.shards) {
		return
	}
	delete(rt.installs, it.token)
	a := w.app
	switch {
	case w.remove:
		rt.completeRemove(w)
	case w.err != nil: // a shard failed: undo the others, keep the id
		rt.rollBackAdd(w)
	default:
		a.installRoutes() // the app is reachable from here
		w.reply <- reply{id: a.id}
	}
}

// completeRemove completes an mRemove (or an add's rollback) all shards acked: the flushers' and recorder's markers,
// then the tombstone, then the reply.
func (rt *router) completeRemove(w *installWait) {
	a := w.app
	res := reply{}
	if a.activated {
		for _, q := range a.all {
			m := make(chan struct{})
			res.markers = append(res.markers, m)
			rt.push(send{shard: -1, sink: q, si: sinkItem{marker: m, last: true}})
		}
		if a.rec != nil {
			close(a.rec.stop)
			res.recMarker = a.rec.returned
		}
	} else if a.rec != nil {
		// Never activated: the queue is released here (owner, 2026-11-25; PR 2's review PR2A-3, PR2Q-10), where an
		// activated app's recorder goroutine releases it at its stop. Safe: no recorder goroutine ever ran
		// (startAppGoroutines follows the activation step, which refuses a removing app), the header was this
		// goroutine's own put at Add, and nothing reads a removing app's queue (the drain step refuses it; a.ing is
		// nil below, so every later put is refused; Stats reads the counters under mu).
		a.rec.q = nil
	}
	// The slot keeps a tombstone: the phase removed, the state dropped, so Stats keeps zero rows of the app's
	// length and the id is never reused (design s3-3-1 MUST 2).
	// Only the router's own fields are dropped: a.dec and a.rec stay, because caller goroutines read them past the
	// gate (SendPayload's decoder, Activate's and Drain's recorder) (review S4). The starting rows go too: only the
	// activation step read them, and it refuses a removing app (owner, 2026-11-25; PR 2's review PR2A-3, PR2Q-10).
	// The tables' rows go, but not their Set: TableWrite's check reads its declarations past the gate, so Release
	// empties each table and keeps them (owner, 2026-11-27; PR 3's review PR3A-2). Safe: Release runs on the router,
	// the tables' only writer and only snapshot taker, and Versions are immutable. The rows are unreachable when
	// Remove returns because every shard dropped the app's walks at mRemove, acked before here.
	a.phase.Store(int32(phaseRemoved))
	a.ing, a.sinks, a.quiet = nil, nil, true
	a.app.Loads = nil
	if a.app.Tables != nil {
		a.app.Tables.Release()
	}
	if w.rollback {
		res.err = w.err
	}
	w.reply <- res
}

// rollBackAdd undoes an add a shard failed: the app removed from every shard, its id kept.
func (rt *router) rollBackAdd(w *installWait) {
	r := rt.r
	a := w.app
	a.phase.Store(int32(phaseRemoved))
	rt.token++
	rt.installs[rt.token] = &installWait{app: a, reply: w.reply, remove: true, rollback: true, err: w.err}
	for sid := range r.shards {
		rt.push(send{shard: sid, msg: message{kind: mRemove, app: a.id, token: rt.token}})
	}
}

// beginDrain starts an app's drain: the barrier to every shard, or the answer of a drain already begun or not needed.
func (rt *router) beginDrain(it item) {
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
}

// drainApp ends the app's input and sends every shard the drain barrier's message, shared by the router and Sync.
func (rt *router) drainApp(a *appState, token uint64) []clock.WMAdvance {
	a.drained = true
	adv := a.ing.Drain()
	rt.broadcast(message{kind: mDrain, app: a.id, adv: adv, tables: a.ing.Snapshot(), token: token, final: true})
	return adv
}
