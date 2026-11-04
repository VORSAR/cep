package metrics_test

// Design note s3-1-1-design.md v2 §4 (ADR-0006): counters declared once, in order, before the first shard; one
// zeroed int64 set per shard; Inc per shard; Sum across shards in declaration order.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/metrics"
)

func TestCounters_IncPerShardSumInDeclarationOrder(t *testing.T) {
	t.Parallel()
	r := &metrics.Registry{}
	add := func(name string) metrics.ID {
		t.Helper()
		id, err := r.Add(name)
		if err != nil {
			t.Fatalf("Add(%q): %v", name, err)
		}
		return id
	}
	// Declared out of alphabetical order, so a sorted Sum is caught.
	streamLate, constructLate, purges := add("stream_late_events"), add("construct_late_events"), add("ttl_purges")
	if streamLate != 0 || constructLate != 1 || purges != 2 {
		t.Fatalf("IDs %d, %d, %d; want the declaration indices 0, 1, 2", streamLate, constructLate, purges)
	}
	if _, err := r.Add("construct_late_events"); err == nil {
		t.Fatalf("Add of a duplicate name: no error")
	}

	s1, s2, s3 := metrics.NewShard(r), metrics.NewShard(r), metrics.NewShard(r)
	if _, err := r.Add("late_addition"); err == nil {
		t.Fatalf("Add after the first NewShard: no error (the registry is frozen)")
	}
	for i, s := range []*metrics.Shard{s1, s2, s3} {
		for _, id := range []metrics.ID{streamLate, constructLate, purges} {
			if got := s.Get(id); got != 0 {
				t.Fatalf("new shard %d: Get(%d) = %d, want 0", i+1, id, got)
			}
		}
	}
	s1.Inc(streamLate, 3)
	s1.Inc(streamLate, 4) // 7
	s2.Inc(streamLate, 5)
	s2.Inc(constructLate, 2)
	s3.Inc(purges, 1)
	for _, c := range []struct {
		shard *metrics.Shard
		name  string
		id    metrics.ID
		want  int64
	}{
		{s1, "s1", streamLate, 7}, {s1, "s1", constructLate, 0}, {s1, "s1", purges, 0},
		{s2, "s2", streamLate, 5}, {s2, "s2", constructLate, 2}, {s2, "s2", purges, 0},
		{s3, "s3", streamLate, 0}, {s3, "s3", constructLate, 0}, {s3, "s3", purges, 1},
	} {
		if got := c.shard.Get(c.id); got != c.want {
			t.Errorf("%s.Get(%d) = %d, want %d", c.name, c.id, got, c.want)
		}
	}
	// 7 + 5 + 0 = 12; 0 + 2 + 0 = 2; 0 + 0 + 1 = 1. The failed Adds declared nothing.
	want := []metrics.Sample{{Name: "stream_late_events", Value: 12}, {Name: "construct_late_events", Value: 2}, {Name: "ttl_purges", Value: 1}}
	if got := metrics.Sum(r, []*metrics.Shard{s1, s2, s3}); !slices.Equal(got, want) {
		t.Fatalf("Sum = %v, want %v", got, want)
	}
	if got := s1.Get(streamLate); got != 7 {
		t.Fatalf("after Sum: s1 stream_late = %d, want 7 (Sum only reads)", got)
	}
	zero := []metrics.Sample{{Name: "stream_late_events"}, {Name: "construct_late_events"}, {Name: "ttl_purges"}}
	if got := metrics.Sum(r, nil); !slices.Equal(got, zero) {
		t.Fatalf("Sum over no shards = %v, want %v", got, zero)
	}
}
