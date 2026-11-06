package expr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// TestBuildRunnable: BuildRunnable refuses, with a *NotRunnableError naming the construct, an expression that holds
// a node this release cannot evaluate — text operators (S7), count (S8), chain (S4.1) — wherever it sits in the tree;
// it builds everything else exactly as Build does (S3.2b-1 design v2 §1, review F1).
func TestBuildRunnable(t *testing.T) {
	textOp := text("has_ci", a("s"), model.TextOperand{Strings: []string{"x"}})
	refused := []struct {
		name string
		node model.Expr
		what string
	}{
		{"a text operator", textOp, "text operators (S7)"},
		{"a text operator under `or`", or(textOp, cmp("==", a("n"), lit(1))), "text operators (S7)"},
		{"count", call("count"), "count (S8)"},
		{"count in arithmetic", ar("+", call("count"), lit(1)), "count (S8)"},
		{"chain", call("chain", a("read"), a("egress")), "chain (S4.1)"},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			if _, err := expr.Build(c.node, testScope); err != nil {
				t.Fatalf("Build: %v; the validator's typing must still accept it", err)
			}
			e, err := expr.BuildRunnable(c.node, testScope)
			nr, ok := errors.AsType[*expr.NotRunnableError](err)
			if !ok || e != nil {
				t.Fatalf("BuildRunnable = (%v, %v); want (nil, *NotRunnableError)", e, err)
			}
			if nr.What != c.what || !strings.Contains(err.Error(), c.what) {
				t.Errorf("NotRunnableError %q (%q); want What %q", nr.What, err, c.what)
			}
		})
	}
	runnable := []model.Expr{
		cmp("==", a("n"), lit(1)),
		and(cmp(">", a("n"), lit(0)), not(isNull(a("s"), false))),
		call("coalesce", a("s"), lit("x")),
	}
	for _, node := range runnable {
		if _, err := expr.BuildRunnable(node, testScope); err != nil {
			t.Errorf("BuildRunnable(%v) = %v; want it built", node.Kind, err)
		}
	}
	if _, err := expr.BuildRunnable(a("nope"), testScope); err == nil || errors.Is(err, nil) {
		t.Errorf("an unknown name: %v; want Build's own error", err)
	} else if _, ok := errors.AsType[*expr.NotRunnableError](err); ok {
		t.Errorf("an unknown name gave a NotRunnableError; want Build's own error")
	}
}
