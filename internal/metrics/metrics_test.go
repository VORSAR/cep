package metrics_test

// Design note s3-1-1-design.md §3 (ADR-0006): counters declared once in order, one zeroed int64 set per shard,
// Inc per shard, Sum across shards in declaration order.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/metrics"
)

func TestCounters_IncPerShardSumInDeclarationOrder(t *testing.T) {
	t.Parallel()
	r := &metrics.Registry{}
	// Declared out of alphabetical order, so a sorted Sum is caught.
	streamLate := r.Add("stream_late_events")
	constructLate := r.Add("construct_late_events")
	purges := r.Add("ttl_purges")
	if streamLate != 0 || constructLate != 1 || purges != 2 {
		t.Fatalf("Add returned %d, %d, %d; want the declaration indices 0, 1, 2", streamLate, constructLate, purges)
	}

	s1, s2, s3 := metrics.NewShard(r), metrics.NewShard(r), metrics.NewShard(r)
	for i, s := range []*metrics.Shard{s1, s2, s3} {
		for _, id := range []metrics.ID{streamLate, constructLate, purges} {
			if got := s.Get(id); got != 0 {
				t.Fatalf("new shard %d: Get(%d) = %d, want 0", i+1, id, got)
			}
		}
	}

	s1.Inc(streamLate, 3)
	s1.Inc(streamLate, 4) // 3 + 4 = 7
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
			t.Errorf("%s.Get(%d) = %d, want %d (each shard counts its own)", c.name, c.id, got, c.want)
		}
	}

	// stream_late 7 + 5 + 0 = 12; construct_late 0 + 2 + 0 = 2; ttl_purges 0 + 0 + 1 = 1.
	want := []metrics.Sample{{Name: "stream_late_events", Value: 12}, {Name: "construct_late_events", Value: 2}, {Name: "ttl_purges", Value: 1}}
	if got := metrics.Sum(r, []*metrics.Shard{s1, s2, s3}); !slices.Equal(got, want) {
		t.Fatalf("Sum = %v, want %v", got, want)
	}
	// Summing reads; it does not reset or move counts.
	if got := s1.Get(streamLate); got != 7 {
		t.Fatalf("after Sum: s1.Get(stream_late) = %d, want 7", got)
	}
	if got := metrics.Sum(r, []*metrics.Shard{s1, s2, s3}); !slices.Equal(got, want) {
		t.Fatalf("second Sum = %v, want %v", got, want)
	}
	// No shards: every declared counter, in order, at 0.
	zero := []metrics.Sample{{Name: "stream_late_events"}, {Name: "construct_late_events"}, {Name: "ttl_purges"}}
	if got := metrics.Sum(r, nil); !slices.Equal(got, zero) {
		t.Fatalf("Sum over no shards = %v, want %v", got, zero)
	}
}
