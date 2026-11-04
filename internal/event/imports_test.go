package event

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestImportsAreTheAllowedStdlib is the import law for internal/event (design note s3-1-2 v2 §4.5, superseding
// s1-1a-gate-design.md §1's {cmp, errors, fmt}): the package's non-test files import only cmp, errors, fmt, math,
// strconv, unicode/utf8 and unicode/utf16, plus sync for sync.Pool alone (archcheck holds the identifier; this test
// only allows the import). time stays out (owner D4: the timestamp reader is hand-written); encoding/json and
// reflect stay out of production code (ADR-0010 §3: no reflection, the decoder is in-repo; the tests use
// encoding/json and time as oracles); sync/atomic stays out (ADR-0015 §2); and nothing from this module comes in
// (ARCHITECTURE.md §3 import rule 1).
func TestImportsAreTheAllowedStdlib(t *testing.T) {
	t.Parallel()
	const module = "github.com/vorsar/cep"
	allowed := map[string]bool{
		"cmp": true, "errors": true, "fmt": true, "math": true, "strconv": true,
		"unicode/utf8": true, "unicode/utf16": true, "sync": true,
	}
	banned := map[string]string{
		"time":          "design §4.5 and owner D4: the RFC 3339 reader is hand-written; time is the tests' oracle only",
		"encoding/json": "design §4.5 and ADR-0010 §3: the decoder is in-repo; encoding/json is the tests' oracle only",
		"reflect":       "design §4.5 and ADR-0010 §3: no reflection in the decoder",
		"sync/atomic":   "design §5 and ADR-0015 §2: internal/event may use sync.Pool only, never sync/atomic",
	}

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
			switch why, isBanned := banned[path]; {
			case isBanned:
				t.Errorf("%s imports %q; %s", name, path, why)
			case allowed[path]:
			case path == module || strings.HasPrefix(path, module+"/"):
				t.Errorf("%s imports %q; ARCHITECTURE §3 rule 1: internal/event imports nothing from this module", name, path)
			default:
				t.Errorf("%s imports %q; design s3-1-2 §4.5: internal/event imports only cmp, errors, fmt, math, strconv, unicode/utf8, unicode/utf16 and sync (sync.Pool)", name, path)
			}
		}
	}
	// Zero files read would make this check vacuous (testing.md: zero is a failure).
	if files == 0 {
		t.Fatalf("found no non-test .go file in the package directory; the import check read nothing")
	}
}
