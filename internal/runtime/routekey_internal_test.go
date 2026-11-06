package runtime

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// TestRouteKeyParts (design s3-2b-2 §2.5; QA G3: a NaN part cannot reach the router through Send or SendPayload, so
// this guard is tested here): part i is the event's value of attrs[i]; a null or NaN in any part is a key error; a
// float zero part is stored as +0.0; a key-less route is the pseudo-key ""; each key's Rest is its own.
func TestRouteKeyParts(t *testing.T) {
	t.Parallel()
	s := func(v string) event.Value { return event.Value{Kind: event.TString, Str: v} }
	f := func(v float64) event.Value { return event.Value{Kind: event.TFloat, F: v} }
	values := []event.Value{s("a"), f(math.Copysign(0, -1)), {Kind: event.TString, Null: true}, f(math.NaN()), s("b")}

	if k, ok := routeKey([]int{0, 1, 4}, values); !ok || k.Parts() != 3 || k.First != s("a") || math.Signbit(k.Rest[0].F) || k.Rest[1] != s("b") {
		t.Errorf("routeKey(a, -0.0, b) = %+v, %v; want (a, +0.0, b), true", k, ok)
	}
	if _, ok := routeKey([]int{0, 2}, values); ok {
		t.Error("a null second part made a key; want a key error")
	}
	if _, ok := routeKey([]int{3, 0}, values); ok {
		t.Error("a NaN first part made a key; want a key error")
	}
	if k, ok := routeKey(nil, values); !ok || k.Parts() != 1 || k.First != (event.Value{Kind: event.TString}) {
		t.Errorf("a key-less route: %+v, %v; want the pseudo-key \"\"", k, ok)
	}
	k1, _ := routeKey([]int{0, 4}, values)
	k2, _ := routeKey([]int{0, 4}, values)
	if &k1.Rest[0] == &k2.Rest[0] {
		t.Error("two keys share one Rest; each event's Rest is its own (ADR-0010 §4)")
	}
}
