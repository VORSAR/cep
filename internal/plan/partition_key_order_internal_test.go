package plan

import (
	"testing"

	"github.com/vorsar/cep/internal/model"
)

// TestBuildRefusesAnUnknownKeyAttributeInAnyPosition: a key attribute the source stream does not declare (E-NAME-001)
// is refused wherever the key writes it (SPEC §4.4): alone, first, or after an attribute whose index is not 0. The
// defect it guards: an unknown key attribute must keep its "not found" index wherever it is written; one that read
// index 0 instead would pass when nothing before it is index 0 and Build would accept a key the source does not have
// (TestBuildRefusesAnUncheckedPartition's row writes it after k, index 0, where it is refused as named twice with the
// same message). Added under the owner's decision of 2026-11-19 (one test-only commit, an exception to the frozen
// nets).
func TestBuildRefusesAnUnknownKeyAttributeInAnyPosition(t *testing.T) {
	t.Parallel()
	const want = "plan: rule R: stream S: key attribute nope is unknown or named twice (a checked model has neither)"
	for _, c := range []struct {
		name string
		key  model.PartitionKey
	}{
		{"the unknown attribute alone", ukKey("S", "nope")},
		{"the unknown attribute first", ukKey("S", "nope", "k")},
		{"the unknown attribute after a non-zero index", ukKey("S", "j", "nope")},
	} {
		p, err := Build(ukApp(c.key))
		if err == nil || err.Error() != want || p != nil {
			t.Errorf("%s: Build = %v, %v; want nil and %q", c.name, p, err, want)
		}
	}
}
