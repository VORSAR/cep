package runtime

import (
	"context"

	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
)

// streamStats is a's per-stream counters: the router's (late, after the drain) and the ingest step's.
func streamStats(a *appState) []StreamStats {
	out := make([]StreamStats, len(a.app.Streams))
	if a.ing == nil { // removed: zero rows of the app's length (design s3-3-1 MUST 2)
		return out
	}
	for i := range out {
		c := a.ing.counts[i]
		out[i] = StreamStats{Late: a.late[i], AfterDrain: a.after[i], Rejected: c.rejected, Unknown: c.unknown,
			FlagOverrides: c.overrides, Batches: a.batches[i]}
	}
	return out
}

// Stats is a consistent cut of the counters, taken through the intake (design §6).
func (r *Runtime) Stats(ctx context.Context) (Stats, error) {
	if r.isClosed() {
		return Stats{}, ErrClosed
	}
	rep, err := r.request(ctx, item{kind: iStats})
	return rep.stats, err
}

// statsCut begins a Stats cut: the router's counters now, then one mStats to every shard; take completes it.
func (rt *router) statsCut(rep chan reply) {
	r := rt.r
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
	rt.stats[rt.token] = &statsWait{stats: st, reply: rep}
	for sid := range r.shards {
		rt.push(send{shard: sid, msg: message{kind: mStats, token: rt.token}})
	}
}

// statsAck folds one shard's part of a Stats cut; the last part completes it.
func (rt *router) statsAck(shard int, it outItem) {
	r := rt.r
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
	w.stats.SaturatedParks[shard] = it.stats.parks
	if w.acks < len(r.shards) {
		return
	}
	delete(rt.stats, it.token)
	rt.completeStats(w)
}

// completeStats adds each app's sink counters to a cut every shard has answered, and replies with it.
func (rt *router) completeStats(w *statsWait) {
	r := rt.r
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

// stats is the shard's counters for a Stats cut.
func (sh *shard) stats() *shardStats {
	st := &shardStats{parks: sh.parks}
	for ai, rules := range sh.rules {
		ps := make([]partition.Stats, len(rules))
		es := make([]plan.Stats, len(rules))
		for ri, pr := range rules {
			ps[ri] = pr.Stats()
			if c, ok := sh.bodies[ai][ri].(plan.Counted); ok {
				es[ri] = c.Stats()
			}
		}
		if rules == nil { // a removed app: zero rows keep [app] indexes stable
			st.rules, st.emits, st.panics, st.outHigh = append(st.rules, nil), append(st.emits, nil), append(st.panics, nil),
				append(st.outHigh, nil)
			continue
		}
		st.rules = append(st.rules, ps)
		st.emits = append(st.emits, es)
		st.panics = append(st.panics, append([]int64(nil), sh.panics[ai]...))
		st.outHigh = append(st.outHigh, append([]int64(nil), sh.outHigh[ai]...))
	}
	return st
}
