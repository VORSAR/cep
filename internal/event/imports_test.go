package event

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestImportsAreCmpErrorsFmt is s1-1a-design-qa.md counter-example 39: the package's non-test files
// import only from {cmp, errors, fmt} (s1-1a-gate-design.md §1). That rules out sync and anything
// from this module (ARCHITECTURE.md §3 import rule 1: "internal/event imports nothing from this
// module").
func TestImportsAreCmpErrorsFmt(t *testing.T) {
	t.Parallel()
	const module = "github.com/vorsar/cep"
	allowed := map[string]bool{"cmp": true, "errors": true, "fmt": true}

	// go test runs a package's tests in the package's directory.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}
	fset := token.NewFileSet()
	files := 0
	for _, e := range entries {
		name := e.Name()
		// The files the go tool builds into the package: *.go, minus tests, minus "_" and "." names.
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parsing %s: %v", name, err)
			continue
		}
		files++
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Errorf("%s: unreadable import %s: %v", name, imp.Path.Value, err)
				continue
			}
			switch {
			case allowed[path]:
			case path == module || strings.HasPrefix(path, module+"/"):
				t.Errorf("%s imports %q; ARCHITECTURE §3 rule 1: internal/event imports nothing from this module", name, path)
			default:
				t.Errorf("%s imports %q; design §1: internal/event imports only cmp, errors and fmt", name, path)
			}
		}
	}
	// Zero files read would make this check vacuous (testing.md: zero is a failure).
	if files == 0 {
		t.Fatalf("found no non-test .go file in the package directory; the import check read nothing")
	}
}
