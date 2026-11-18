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
		a.activated = true
		a.started.Store(true)
		it.reply <- reply{}
	}
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
		}
		// The slot keeps a tombstone: the phase removed, the state dropped, so Stats keeps zero rows of the app's
		// length and the id is never reused (design s3-3-1 MUST 2).
		// Only the router's own fields are dropped: a.dec and a.rec stay, because caller goroutines read them past the
		// gate (SendPayload's decoder, Activate's and Drain's recorder) (review S4).
		a.phase.Store(int32(phaseRemoved))
		a.ing, a.sinks, a.quiet = nil, nil, true
		if w.rollback {
			res.err = w.err
		}
		w.reply <- res
	case w.err != nil: // a shard failed: undo the others, keep the id
		a.phase.Store(int32(phaseRemoved))
		rt.token++
		rt.installs[rt.token] = &installWait{app: a, reply: w.reply, remove: true, rollback: true, err: w.err}
		for sid := range r.shards {
			rt.push(send{shard: sid, msg: message{kind: mRemove, app: a.id, token: rt.token}})
		}
	default:
		a.installRoutes() // the app is reachable from here
		w.reply <- reply{id: a.id}
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
	rt.broadcast(a, mDrain, adv, a.ing.Snapshot(), token, true)
	return adv
}
