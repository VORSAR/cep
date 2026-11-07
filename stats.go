package cep

import "github.com/vorsar/cep/internal/runtime"

// Stats is one app's counters, keyed by name (design s3-3-1 §3.1).
type Stats struct {
	Streams   map[string]StreamStats // per source stream
	Rules     map[string]RuleStats
	Sinks     map[string][]SinkStats // per derived stream, its bindings in binding order (the program's sink first)
	Recording RecordingStats
}

// StreamStats are one source stream's counters.
type StreamStats struct{ Late, AfterDrain, Rejected, Unknown, FlagOverrides, Batches int64 }

// RuleStats are one rule's counters.
type RuleStats struct{ ConstructLate, Purged, KeyErrors, Panics, Suppressed, Errored, OutboxHigh int64 }

// SinkStats are one sink binding's counters.
type SinkStats struct {
	Errors    int64
	QueueHigh int
}

// RecordingStats is the app's recording.
type RecordingStats struct {
	Records int64
	Failed  bool
	Err     string
}

// convert is the app's slice of the engine's counters, by name; a removed app's rows are zero.
func (a *AppHandle) convert(st runtime.Stats) Stats {
	out := Stats{Streams: map[string]StreamStats{}, Rules: map[string]RuleStats{}, Sinks: map[string][]SinkStats{}}
	id := int(a.id)
	if id < len(st.Streams) {
		for sid, s := range st.Streams[id] {
			if a.plan.Streams[sid].Derived {
				continue
			}
			out.Streams[a.plan.Streams[sid].Schema.Stream()] = StreamStats{Late: s.Late, AfterDrain: s.AfterDrain, Rejected: s.Rejected, Unknown: s.Unknown, FlagOverrides: s.FlagOverrides, Batches: s.Batches}
		}
	}
	if id < len(st.Rules) {
		for ri, r := range st.Rules[id] {
			out.Rules[a.plan.Rules[ri].Name] = RuleStats{ConstructLate: r.ConstructLate, Purged: r.Purged, KeyErrors: r.KeyErrors, Panics: r.Panics, Suppressed: r.Suppressed, Errored: r.Errored, OutboxHigh: r.OutboxHigh}
		}
	}
	if id < len(st.Recording) {
		rs := st.Recording[id]
		out.Recording = RecordingStats{Records: rs.Records, Failed: rs.Failed, Err: rs.Err}
	}
	for stream, idxs := range a.sinkRows {
		rows := make([]SinkStats, len(idxs))
		for i, bi := range idxs {
			if id < len(st.SinkErrors) && bi < len(st.SinkErrors[id]) {
				rows[i].Errors = st.SinkErrors[id][bi]
			}
			if id < len(st.SinkHigh) && bi < len(st.SinkHigh[id]) {
				rows[i].QueueHigh = st.SinkHigh[id][bi]
			}
		}
		out.Sinks[stream] = rows
	}
	return out
}
