package partition_test

// The engine's guard behind E-PART-008 (design s3-3-1 §7; QA's T37): NewRule refuses a key with an object part,
// alone or beside a scalar part, and leaves no rule. The validator's deploy error comes first; this stays.

import (
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

func TestNewRuleRefusesAnObjectKey(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		keys []event.Type
	}{
		{"object alone", []event.Type{event.TObject}},
		{"object after a string part", []event.Type{event.TString, event.TObject}},
		{"object before an int part", []event.Type{event.TObject, event.TInt}},
	} {
		cfg := partition.Config{Inputs: []int{0}, KeyTypes: c.keys, TTL: 60_000, Body: &body{t: t}}
		r, err := partition.NewRule(cfg)
		if err == nil || r != nil {
			t.Errorf("NewRule(%s): rule %v, err %v; want no rule and an error", c.name, r, err)
			continue
		}
		if !strings.Contains(err.Error(), "partition") || !strings.Contains(err.Error(), "key") {
			t.Errorf("NewRule(%s): %q does not say which key is refused", c.name, err)
		}
	}
	scalar := partition.Config{Inputs: []int{0}, KeyTypes: []event.Type{event.TString, event.TInt}, TTL: 60_000, Body: &body{t: t}}
	if _, err := partition.NewRule(scalar); err != nil {
		t.Errorf("NewRule of a two-part scalar key (the control): %v", err)
	}
}
