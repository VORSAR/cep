package validate

import (
	"errors"
	"testing"

	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// TestABuildErrorWithoutACodeIsETYPE001: an error from the builder that is not an *expr.Error carries no code, so
// build reports it as E-TYPE-001 at the root with the error's text. Every builtin returns an *expr.Error, so the
// error comes from a callable registered here (design s1-1c-6 §5 §3.1).
func TestABuildErrorWithoutACodeIsETYPE001(t *testing.T) {
	t.Parallel()
	reg := expr.Builtins()
	boom := func([]expr.Expr, model.Expr) (expr.Expr, error) { return nil, errors.New("boom") }
	if err := reg.Register(expr.Descriptor{Name: "boom", Kind: expr.Scalar, Factory: boom}); err != nil {
		t.Fatal(err)
	}
	c := &checker{reg: reg}
	root := &model.Expr{Kind: model.ExprCall, Name: "boom", Pos: model.Pos{Line: 3, Col: 7}}
	e, ok := c.build(root, &scope{bare: -1})
	if e != nil || ok {
		t.Fatalf("build = %v, %v; want nil, false", e, ok)
	}
	want := Diagnostic{Code: "E-TYPE-001", Line: 3, Col: 7, Msg: "boom", Fix: catalog["E-TYPE-001"].Fix}
	if len(c.diags) != 1 || c.diags[0] != want {
		t.Fatalf("got %+v, want [%+v]", c.diags, want)
	}
}
