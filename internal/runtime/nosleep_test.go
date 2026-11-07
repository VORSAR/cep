package runtime_test

// TestNoSleepOrPollingInRuntime (design s3-3-1 §2.5 MUST NOT; QA's T8): the package's non-test files use no
// time.Sleep, time.After or time.Tick anywhere; tick() may use time.NewTicker. go/parser over the sources.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoSleepOrPollingInRuntime(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := map[string]bool{"Sleep": true, "After": true, "Tick": true, "AfterFunc": true}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		n++
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "time" && banned[sel.Sel.Name] {
				t.Errorf("%s: time.%s is a sleep or a poll (MUST NOT, design s3-3-1 §2.5)", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
	if n == 0 {
		t.Fatal("no non-test .go file parsed; the test selects nothing")
	}
}
