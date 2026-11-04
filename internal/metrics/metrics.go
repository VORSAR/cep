// Package metrics holds the engine's counters (ADR-0006): one set per shard, plain int64s with no atomics on the
// hot path, summed on scrape. Design: docs/sprints/s3-1-1-design.md §4.
//
// STUB (S3.1-1, before G1): the API QA's tests are written against; every body panics until the build.
package metrics

const notBuilt = "metrics: S3.1-1 not built yet"

// ID is a counter's index in its registry.
type ID int

// Registry names the counters, in declaration order; it is frozen by the first NewShard.
type Registry struct{}

// Add declares a counter, once, at deploy; a duplicate name or a frozen registry is an error.
func (r *Registry) Add(name string) (ID, error) { panic(notBuilt) }

// Shard is one shard's counters.
type Shard struct{}

// NewShard makes a zeroed counter set for the registry's counters.
func NewShard(r *Registry) *Shard { panic(notBuilt) }

// Inc adds n to a counter.
func (s *Shard) Inc(id ID, n int64) { panic(notBuilt) }

// Get is a counter's value on this shard.
func (s *Shard) Get(id ID) int64 { panic(notBuilt) }

// Sample is one counter's name and value.
type Sample struct {
	Name  string
	Value int64
}

// Sum adds each counter across the shards, in declaration order.
func Sum(r *Registry, shards []*Shard) []Sample { panic(notBuilt) }
