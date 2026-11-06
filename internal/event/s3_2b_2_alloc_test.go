package event_test

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// TestDecodeAllocationBudget is S3.1-2's F12, pinned (design s3-2b-2 §8; review F15): a decode into a slice with room
// allocates once per string value of two or more bytes (one-byte strings are the Go runtime's interned ones) and
// twice per object value — nothing else.
func TestDecodeAllocationBudget(t *testing.T) {
	s, err := event.NewSchema("S", []event.Attr{
		{Name: "a", Type: event.TString}, {Name: "b", Type: event.TString}, {Name: "n", Type: event.TInt},
		{Name: "o", Type: event.TObject}, {Name: "ts", Type: event.TTimestamp},
	}, 4)
	if err != nil {
		t.Fatal(err)
	}
	d := event.NewDecoder(s)
	payload := []byte(`{"a":"alpha","b":"beta","n":7,"o":{"x":[1,2]},"ts":"2026-01-01T10:00:00Z"}`)
	dst := make([]event.Value, 0, 8)
	allocs := testing.AllocsPerRun(200, func() {
		if _, _, err := d.Decode(payload, dst); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 4 {
		t.Errorf("%v allocations per decode; want 4: two strings, one object (two)", allocs)
	}
}
