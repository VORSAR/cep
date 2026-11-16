package validate

import (
	"testing"

	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// TestMayBeNullIsConservative: a node mayBeNull cannot place may be null — an attribute the scope cannot resolve, a
// kind it does not know, the null literal. None is typeable through Check (design s1-1c-6 §5 §3.1), so it is called
// directly.
func TestMayBeNullIsConservative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		n    model.Expr
	}{
		{"an attribute the scope cannot resolve", model.Expr{Kind: model.ExprAttr, Name: "nope"}},
		{"an unknown kind", model.Expr{Kind: "bogus"}},
		{"the null literal", model.Expr{Kind: model.ExprLiteral, Lit: model.Literal{Kind: model.LitNull}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ch := &checker{reg: expr.Builtins()}
			if !ch.mayBeNull(&c.n, &scope{bare: -1}, &model.Rule{}) {
				t.Fatalf("mayBeNull(%+v) = false, want true", c.n)
			}
		})
	}
}
