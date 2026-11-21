package runtime_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoSleepOrPollingInRuntime (design s3-3-1 §2.5 MUST NOT; QA's T8): the package's non-test files use no
// time.Sleep, time.After, time.Tick or time.AfterFunc anywhere, under whatever name they import "time" (a dot import
// is refused); tick() may use time.NewTicker. go/parser over the sources.
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
		names := map[string]bool{}
		for _, im := range f.Imports {
			if im.Path.Value != `"time"` {
				continue
			}
			switch {
			case im.Name == nil:
				names["time"] = true
			case im.Name.Name == ".":
				t.Errorf("%s: a dot import of time hides its calls from this check", name)
			case im.Name.Name != "_":
				names[im.Name.Name] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && names[x.Name] && banned[sel.Sel.Name] {
				t.Errorf("%s: time.%s is a sleep or a poll (MUST NOT, design s3-3-1 §2.5)", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
	if n == 0 {
		t.Fatal("no non-test .go file parsed; the test selects nothing")
	}
}
