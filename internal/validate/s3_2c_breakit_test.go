package validate_test

// S3.2c break-it (QA, short): a bound slot's duration must be positive (E-DUR-003 refuses zero: "use a positive
// duration"), and a duration the source can write is never negative — but a deployment override is a model.Literal
// the caller builds (Options.Overrides), and a negative one passes. `output first every ${D}` with D = -5 sec deploys
// with no diagnostic; plan's rate limit only runs for every > 0, so it is silently off and every output is emitted, and
// partition's addSat (a + b "for b >= 0") computes math.MaxInt64 - b, which wraps for a negative b. Prefix: ox.

import (
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/validate"
)

// TestBreakitANegativeDurationOverrideInABoundSlotIsRefused: the rate-limit interval overridden to -5 sec is a
// deploy diagnostic, as zero is.
func TestBreakitANegativeDurationOverrideInABoundSlotIsRefused(t *testing.T) {
	src := "language cep/0.1;\n" +
		"param D duration = 5 sec;\n" +
		"stream S (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
		"stream O (k string, ts timestamp);\n" +
		"rule R intent 'At most one output per D.' from S select k output first every ${D} insert into O;\n"
	res, err := parser.Parse("ox-negative.cep", src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	app, diags := validate.Check(res.App, validate.Options{Overrides: map[string]model.Literal{"D": {Kind: model.LitDuration, Ms: -5000}}})
	if len(diags) == 0 {
		t.Fatalf("no diagnostic; output first every is %+v — want a negative bound refused at deploy, as a zero one is (E-DUR-003)", app.Rules[0].OutputFirstEvery)
	}
}
