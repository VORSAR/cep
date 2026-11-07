package validate_test

// S3.2c's fix round, around QA's break-it K1 (TestBreakitANegativeDurationOverrideInABoundSlotIsRefused): a bound
// slot takes a duration > 0 (errors.md E-DUR-003; SPEC §2.4). Source cannot write a negative duration, but a
// deployment override or a model built in Go can. A negative ttl is E-DUR-003's alone (not also below every bound,
// E-TTL-002), and a negative `idle after` is E-DUR-003 too. Prefix: ng.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/validate"
)

func ngCodes(diags []validate.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func TestANegativeBoundIsEDUR003(t *testing.T) {
	t.Run("a negative ttl override", func(t *testing.T) {
		src := "language cep/0.1;\n" +
			"param T duration = 1 hour;\n" +
			"stream S (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
			"stream O (k string, ts timestamp);\n" +
			"rule R intent 'Keyed.' partition by k of S ttl ${T} from S select k output first every 5 sec insert into O;\n"
		res, err := parser.Parse("ng-ttl.cep", src)
		if err != nil {
			t.Fatal(err)
		}
		_, diags := validate.Check(res.App, validate.Options{Overrides: map[string]model.Literal{"T": {Kind: model.LitDuration, Ms: -3_600_000}}})
		if got := ngCodes(diags); !slices.Equal(got, []string{"E-DUR-003"}) {
			t.Fatalf("codes %v (%v); want E-DUR-003 alone", got, diags)
		}
	})
	t.Run("a negative idle after in a model built in Go", func(t *testing.T) {
		src := "language cep/0.1;\n" +
			"stream S (k string, ts timestamp) time ts watermark bounded 1 min idle after 5 min;\n" +
			"stream O (k string, ts timestamp);\n" +
			"rule R intent 'Plain.' from S select k insert into O;\n"
		res, err := parser.Parse("ng-idle.cep", src)
		if err != nil {
			t.Fatal(err)
		}
		res.App.Streams[0].Time.IdleAfter = -300_000
		_, diags := validate.Check(res.App, validate.Options{})
		if got := ngCodes(diags); !slices.Equal(got, []string{"E-DUR-003"}) {
			t.Fatalf("codes %v (%v); want E-DUR-003", got, diags)
		}
	})
}
