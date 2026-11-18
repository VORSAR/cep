package runtime

// take routes a handoff's items in order: outputs to their sinks, then the drain acks and stats replies.
func (rt *router) take(h handoff) {
	r := rt.r
	for _, it := range h.items {
		switch it.kind {
		case oEvent:
			if a := rt.app(it.app); a != nil && a.ing != nil {
				for _, q := range a.sinks[it.ev.StreamID] {
					rt.push(send{shard: -1, sink: q, si: sinkItem{ev: it.ev}})
				}
			}
		case oAck:
			if w := rt.installs[it.token]; w != nil {
				rt.installAck(w, it)
				continue
			}
			d := rt.drains[it.token]
			d.acks++
			d.pending += it.pending
			if d.acks < len(r.shards) {
				continue
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
		case oStats:
			w := rt.stats[it.token]
			w.acks++
			for ai := range it.stats.rules {
				for ri, ps := range it.stats.rules[ai] {
					rs := &w.stats.Rules[ai][ri]
					rs.ConstructLate += int64(ps.ConstructLate)
					rs.Purged += int64(ps.Purged)
					rs.Panics += int64(ps.Panics) + it.stats.panics[ai][ri]
					rs.Suppressed += int64(it.stats.emits[ai][ri].Suppressed)
					rs.Errored += int64(it.stats.emits[ai][ri].Errored)
					rs.OutboxHigh = max(rs.OutboxHigh, it.stats.outHigh[ai][ri])
				}
			}
			w.stats.SaturatedParks[h.shard] = it.stats.parks
			if w.acks < len(r.shards) {
				continue
			}
			delete(rt.stats, it.token)
			for _, a := range r.appList() {
				if a == nil || a.ing == nil {
					w.stats.SinkErrors, w.stats.SinkHigh = append(w.stats.SinkErrors, nil), append(w.stats.SinkHigh, nil)
					continue
				}
				errs := make([]int64, len(a.all))
				for i, q := range a.all {
					errs[i] = q.errors.Load()
				}
				w.stats.SinkErrors = append(w.stats.SinkErrors, errs)
				high := make([]int, len(a.all))
				for i, q := range a.all {
					high[i] = q.high
				}
				w.stats.SinkHigh = append(w.stats.SinkHigh, high)
			}
			w.reply <- reply{stats: w.stats}
		}
	}
}
