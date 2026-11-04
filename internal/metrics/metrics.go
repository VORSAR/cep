// Package metrics holds the engine's counters (ADR-0006): one set per shard, plain int64s with no atomics on the
// hot path, summed on scrape. A live shard is scraped through its mailbox (the runtime, S3.2a); Sum is for a
// quiesced runtime and tests. Design: docs/sprints/s3-1-1-design.md §4.
package metrics

import "fmt"

// ID is a counter's index in its registry.
type ID int

// Registry names the counters, in declaration order; it is frozen by the first NewShard.
type Registry struct {
	names  []string
	frozen bool
}

// Add declares a counter, once, at deploy; a duplicate name or a frozen registry is an error.
func (r *Registry) Add(name string) (ID, error) {
	if r.frozen {
		return 0, fmt.Errorf("metrics: counter %q added after the first shard was made", name)
	}
	for _, n := range r.names {
		if n == name {
			return 0, fmt.Errorf("metrics: counter %q declared twice", name)
		}
	}
	r.names = append(r.names, name)
	return ID(len(r.names) - 1), nil
}

// Shard is one shard's counters.
type Shard struct {
	values []int64
}

// NewShard makes a zeroed counter set for the registry's counters, and freezes the registry.
func NewShard(r *Registry) *Shard {
	r.frozen = true
	return &Shard{values: make([]int64, len(r.names))}
}

// Inc adds n to a counter.
func (s *Shard) Inc(id ID, n int64) { s.values[id] += n }

// Get is a counter's value on this shard.
func (s *Shard) Get(id ID) int64 { return s.values[id] }

// Sample is one counter's name and value.
type Sample struct {
	Name  string
	Value int64
}

// Sum adds each counter across the shards, in declaration order.
func Sum(r *Registry, shards []*Shard) []Sample {
	out := make([]Sample, len(r.names))
	for i, n := range r.names {
		out[i].Name = n
		for _, s := range shards {
			out[i].Value += s.values[i]
		}
	}
	return out
}
