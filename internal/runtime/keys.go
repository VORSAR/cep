package runtime

import (
	"encoding/binary"
	"hash/fnv"
	"math"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// routeKey is an event's key for a route (design s3-2b-2 §2.5): part i is the event's value of attrs[i]; ok is false
// when a part is null or a NaN float. A float zero part is stored as +0.0 (one key, one shard); the router is the only
// writer of a key's parts (MUST 16). A key-less route's key is the pseudo-key "". Rest is fresh per event: the
// message owns it (ADR-0010 §4).
func routeKey(attrs []int, values []event.Value) (partition.Key, bool) { //nolint:gocyclo,lll // PERMANENT reason="kept whole: its keyPart split costs +2.0 % median on BenchmarkRouteKey/float-zero (Mann-Whitney p=1.1e-05, 10 pairs; re-run +2.6 %, p=0.015), allocations 0 = 0; samples in s1-1c-6-record.md" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-6
	if attrs == nil {
		return partition.Single(event.Value{Kind: event.TString}), true
	}
	part := func(attr int) (event.Value, bool) {
		v := values[attr]
		if v.Null || (v.Kind == event.TFloat && math.IsNaN(v.F)) {
			return v, false
		}
		if v.Kind == event.TFloat && v.F == 0 {
			v.F = 0 // -0.0 is +0.0
		}
		return v, true
	}
	first, ok := part(attrs[0])
	if !ok {
		return partition.Key{}, false
	}
	key := partition.Single(first)
	if len(attrs) > 1 {
		key.Rest = make([]event.Value, len(attrs)-1)
		for i, attr := range attrs[1:] {
			if key.Rest[i], ok = part(attr); !ok {
				return partition.Key{}, false
			}
		}
	}
	return key, true
}

// shardOf is fnv-1a over (app, rule, the key's parts in order): a key-less rule hashes (app, rule). A single-part key
// hashes as it did before multi-part keys.
func (rt *router) shardOf(app AppID, rule int, keyed bool, key partition.Key) int {
	h := fnv.New64a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(app))
	h.Write(b[:])
	binary.LittleEndian.PutUint64(b[:], uint64(rule))
	h.Write(b[:])
	if keyed {
		for i := range key.Parts() { // inline: a hasher passed on as an interface escapes, one allocation per event
			key := key.Part(i)
			h.Write([]byte{byte(key.Kind)})
			switch key.Kind {
			case event.TString:
				h.Write([]byte(key.Str))
			case event.TInt:
				binary.LittleEndian.PutUint64(b[:], uint64(key.I))
				h.Write(b[:])
			case event.TTimestamp:
				binary.LittleEndian.PutUint64(b[:], uint64(key.T))
				h.Write(b[:])
			case event.TDuration:
				binary.LittleEndian.PutUint64(b[:], uint64(key.D))
				h.Write(b[:])
			case event.TFloat:
				binary.LittleEndian.PutUint64(b[:], math.Float64bits(key.F))
				h.Write(b[:])
			case event.TBool:
				if key.B {
					h.Write([]byte{1})
				} else {
					h.Write([]byte{0})
				}
			}
		}
	}
	return int(h.Sum64() % uint64(len(rt.r.shards)))
}
