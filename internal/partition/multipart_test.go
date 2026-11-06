package partition_test

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// mpKey is a two-or-more-part key of strings and floats, as the router builds it (design s3-2b-2 §2.5).
func mpKey(first event.Value, rest ...event.Value) partition.Key {
	return partition.Key{First: first, Rest: rest}
}

func mpFloat(f float64) event.Value { return event.Value{Kind: event.TFloat, F: f} }

// TestMultiPartKeysAreOneInstancePerTuple (design s3-2b-2 §1.1, §2.6, MUST 4): two keys share an instance iff their
// parts are equal position by position. (k, "a"+K, "c") and (k, "a", K+"c"), K the string kind's byte, are two keys —
// the map key's strings are length-prefixed, so a part ending in a kind byte cannot pass for the next part's start —
// and a float part compares as a single key does: -0.0 and +0.0 are one part.
func TestMultiPartKeysAreOneInstancePerTuple(t *testing.T) {
	t.Parallel()
	b := &oaBody{}
	r, err := partition.NewRule(partition.Config{Inputs: []int{0}, KeyTypes: []event.Type{event.TString, event.TString, event.TString, event.TFloat},
		TTL: 60_000, Body: b, OnArrival: true})
	if err != nil {
		t.Fatal(err)
	}
	kb := string([]byte{byte(event.TString)})
	offers := []partition.Key{
		mpKey(str("k"), str("a"+kb), str("c"), mpFloat(1)),
		mpKey(str("k"), str("a"), str(kb+"c"), mpFloat(1)), // not (k, a+K, c): a different key
		mpKey(str("k"), str("a"+kb), str("c"), mpFloat(1)), // (k, a+K, c, 1) again
		mpKey(str("k"), str("a"+kb), str("c"), mpFloat(0)),
		mpKey(str("k"), str("a"+kb), str("c"), mpFloat(math.Copysign(0, -1))), // -0.0 is +0.0
	}
	for i, k := range offers {
		r.Offer(k, ev(0, int64(i+1), uint64(i+1)))
	}
	if r.Len() != 3 {
		t.Errorf("%d instances, want 3: (k, a+K, c, 1), (k, a, K+c, 1), (k, a+K, c, 0)", r.Len())
	}
}

// TestMultiPartOfferPanicsOnAMalformedKey (design s3-2b-2 §2.6): the router checks first, so Offer panics on a part
// count other than the rule's and on a null part, as on a single null key.
func TestMultiPartOfferPanicsOnAMalformedKey(t *testing.T) {
	t.Parallel()
	for name, key := range map[string]partition.Key{
		"one part for two":   partition.Single(str("a")),
		"three for two":      mpKey(str("a"), str("b"), str("c")),
		"a null second part": mpKey(str("a"), event.Value{Kind: event.TString, Null: true}),
		"a wrong-typed part": mpKey(str("a"), mpFloat(1)),
	} {
		t.Run(name, func(t *testing.T) {
			r, err := partition.NewRule(partition.Config{Inputs: []int{0}, KeyTypes: []event.Type{event.TString, event.TString},
				TTL: 60_000, Body: &oaBody{}, OnArrival: true})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if recover() == nil {
					t.Errorf("Offer(%v) did not panic", key)
				}
			}()
			r.Offer(key, ev(0, 1, 1))
		})
	}
}
