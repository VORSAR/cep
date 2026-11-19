package plan

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// BenchmarkBodyOnEvent is a built body's OnEvent called directly, as partition calls it, one path per sub-benchmark
// (design S1.1c-6 Q.3, DA-S7; the runtime's Send benchmark never reaches OnEvent): filtered-out stops at the filter
// (x 3 > 5 is false), projected emits an int into an int attribute, widened emits an int widened into a float one.
// The emit function drops the output.
func BenchmarkBodyOnEvent(b *testing.B) {
	for _, c := range []struct {
		name, rule string
		x          event.Value
	}{
		{"filtered-out", "FilteredOut", event.Value{Kind: event.TInt, I: 3}},
		{"projected", "Projected", boNullInt()},
		{"widened", "Widened", boNullInt()},
	} {
		b.Run(c.name, func(b *testing.B) {
			body, in := boBody(b, c.rule, func(event.Event) {})
			e := boEvent(c.x, boNullBool())
			b.ReportAllocs()
			for b.Loop() {
				body.OnEvent(in, e)
			}
		})
	}
}
