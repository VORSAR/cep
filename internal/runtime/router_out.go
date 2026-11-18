package runtime

// take routes a handoff's items in order: outputs to their sinks, then the drain acks and stats replies.
func (rt *router) take(h handoff) {
	for _, it := range h.items {
		switch it.kind {
		case oEvent:
			rt.toSinks(it)
		case oAck:
			rt.ack(it)
		case oStats:
			rt.statsAck(h.shard, it)
		}
	}
}

// toSinks queues an output to every sink of its stream, unless its app is gone.
func (rt *router) toSinks(it outItem) {
	if a := rt.app(it.app); a != nil && a.ing != nil {
		for _, q := range a.sinks[it.ev.StreamID] {
			rt.push(send{shard: -1, sink: q, si: sinkItem{ev: it.ev}})
		}
	}
}

// ack is one shard's ack of an install or a drain.
func (rt *router) ack(it outItem) {
	if w := rt.installs[it.token]; w != nil {
		rt.installAck(w, it)
		return
	}
	rt.drainAck(it)
}

// drainAck is one shard's ack of a drain barrier; the last one queues a marker to every sink and answers each Drain.
func (rt *router) drainAck(it outItem) {
	r := rt.r
	d := rt.drains[it.token]
	d.acks++
	d.pending += it.pending
	if d.acks < len(r.shards) {
		return
	}
	delete(rt.drains, it.token)
	var markers []chan struct{}
	for _, q := range d.app.all {
		m := make(chan struct{})
		markers = append(markers, m)
		rt.push(send{shard: -1, sink: q, si: sinkItem{marker: m}})
	}
	res := reply{pending: d.pending, markers: markers, recMarker: d.recMarker}
	d.app.drainRes = &res
	for _, ch := range d.replies {
		ch <- res
	}
}
