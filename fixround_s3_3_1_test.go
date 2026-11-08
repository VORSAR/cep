package cep_test

// S3.3-1's fix round: the combined review's facade findings that QA's break-it did not already pin (docs/reviews/
// s3-3-1-combined-architecture.md, later: Deploy on a closed runtime). Prefix: fxc.

import (
	"errors"
	"testing"

	"github.com/vorsar/cep"
)

// TestDeployOnAClosedRuntimeIsErrClosed (§3.7's last row): Deploy after Close is ErrClosed, whatever the program —
// an invalid one included; the closed runtime is checked before the program is read.
func TestDeployOnAClosedRuntimeIsErrClosed(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r, err := cep.New(cep.Config{Now: fcNow})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(ctx); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{"valid": fcFwdSrc, "invalid": "language cep/0.1;\nrule R from Nowhere;\n"} {
		if _, err := r.Deploy(ctx, name, src); !errors.Is(err, cep.ErrClosed) {
			t.Errorf("Deploy of a %s program after Close: %v; want ErrClosed", name, err)
		}
	}
}
