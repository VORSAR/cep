package runtime

import (
	"fmt"

	"github.com/vorsar/cep/internal/clock"
)

// ---- the router's side ----

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

// drainApp ends the app's input and sends every shard the drain barrier's message, shared by the router and Sync.
func (rt *router) drainApp(a *appState, token uint64) []clock.WMAdvance {
	a.drained = true
	adv := a.ing.Drain()
	rt.broadcast(a, mDrain, adv, a.ing.Snapshot(), token, true)
	return adv
}
