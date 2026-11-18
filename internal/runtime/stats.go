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
