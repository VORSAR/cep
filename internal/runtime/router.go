package runtime

import "github.com/vorsar/cep/internal/clock"

// ---- the router ----

type send struct {
	shard int // -1: a sink send
	msg   message
	sink  *sinkQ
	si    sinkItem
}

type drainWait struct {
	app     *appState
	acks    int
	pending int64
	replies []chan reply // the Drain calls waiting for this drain's result
	// closed once the recording's stop is written, or an ended recording's queued records are; nil: no recording
	recMarker chan struct{}
}

type statsWait struct {
	acks  int
	stats Stats
	reply chan reply
}

// installWait is a pending mAdd or mRemove (design s3-3-1 §2.3): acked by every shard, then completed in take.
type installWait struct {
	app      *appState
	acks     int
	err      error      // the first shard error of an add
	reply    chan reply // the caller's
	remove   bool       // an mRemove (or an add's rollback)
	rollback bool       // the remove undoes a failed add: reply err, not nil
}

type router struct {
	r           *Runtime
	pending     []send
	pendingHigh int
	mailboxHigh []int
	stamp       int64
	token       uint64
	drains      map[uint64]*drainWait
	installs    map[uint64]*installWait
	stats       map[uint64]*statsWait
	offers      [][]offer // per shard, reused: a message's offers are a view into it until delivered (review F4)
	// deliver hands one shard its message; the message's adv and offers are views valid only during the call. Live:
	// send, which copies them; Sync: the shard's step, in place.
	deliver func(shard int, m message)
}

func newRouter(r *Runtime) *router {
	return &router{
		r: r, mailboxHigh: make([]int, len(r.shards)), stamp: r.start, drains: map[uint64]*drainWait{},
		installs: map[uint64]*installWait{}, stats: map[uint64]*statsWait{}, offers: make([][]offer, len(r.shards)),
	}
}

// send is the live deliver: the message crosses goroutines, so its views are copied.
func (rt *router) send(shard int, m message) {
	m.adv = append([]clock.WMAdvance(nil), m.adv...)
	if len(m.offers) > 0 {
		m.offers = append([]offer(nil), m.offers...)
	}
	rt.push(send{shard: shard, msg: m})
}

func (r *Runtime) route(rt *router) {
	defer r.wg.Done()
	for {
		if r.stopping() {
			return
		}
		if len(rt.pending) > 0 {
			if !rt.sendHead() {
				return
			}
			continue
		}
		select {
		case it := <-r.intake:
			rt.ingest(it)
		case h := <-r.handoff:
			rt.take(h)
		case <-r.stopped:
			return
		}
	}
}

// sendHead offers the pending head to its shard's mailbox, or its sink's queue, taking a handoff while that is full
// (each select whole, its priority unchanged); false: the runtime stopped.
func (rt *router) sendHead() bool {
	r := rt.r
	head := &rt.pending[0]
	if head.shard >= 0 {
		mb := r.shards[head.shard].mailbox
		select {
		case mb <- head.msg:
			rt.mailboxHigh[head.shard] = max(rt.mailboxHigh[head.shard], len(mb))
			rt.pop()
		case h := <-r.handoff:
			rt.take(h)
		case <-r.stopped:
			return false
		}
	} else {
		select {
		case head.sink.q <- head.si:
			head.sink.high = max(head.sink.high, len(head.sink.q))
			rt.pop()
		case h := <-r.handoff:
			rt.take(h)
		case <-r.stopped:
			return false
		}
	}
	return true
}

func (rt *router) push(s send) {
	rt.pending = append(rt.pending, s)
	rt.pendingHigh = max(rt.pendingHigh, len(rt.pending))
}

func (rt *router) pop() {
	rt.pending[0] = send{}
	rt.pending = rt.pending[1:]
	if len(rt.pending) == 0 {
		rt.pending = nil
	}
}

// now is the monotone wall stamp shared by arrivals and ticks (review F6).
func (rt *router) now(t int64) int64 {
	rt.stamp = max(rt.stamp, t)
	return rt.stamp
}

// broadcast delivers one advance-only (or drain) message to every shard, with the app's snapshot (owner D5 of
// S3.2a-1); m.final marks a drain or the end, whose advances on-arrival rules take too (owner D1 of S3.2b-1). It
// carries no admission: broadcast sets m.admStream to -1 itself.
func (rt *router) broadcast(m message) {
	m.admStream = -1
	for sid := range rt.r.shards {
		rt.deliver(sid, m)
	}
}

// app is the router's view of a slot: nil for a removed app.
func (rt *router) app(id AppID) *appState {
	apps := rt.r.appList()
	if int(id) < len(apps) {
		return apps[id]
	}
	return nil
}
