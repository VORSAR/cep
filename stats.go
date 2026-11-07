package cep

// Stats is one app's counters, keyed by name.
type Stats struct {
	Streams   map[string]StreamStats // per source stream
	Rules     map[string]RuleStats
	Sinks     map[string][]SinkStats // per derived stream, its bindings in binding order
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
