package expr_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/expr"
)

// docsync (c) (ADR-0015 §2.2; S2.2a): SPEC Appendix F's function table and the registry name the same callables
// with the same kinds, so neither can drift from the other. appendixF reads the table; drift compares.

var functionRow = regexp.MustCompile("(?m)^\\| `([^`]+)`(?: / `([^`]+)`)? \\| (scalar|aggregate|pattern-only) \\|")

// appendixF gives every callable Appendix F lists, by name, with its kind.
func appendixF(spec string) (map[string]expr.Kind, error) {
	start := strings.Index(spec, "## Appendix F.")
	if start < 0 {
		return nil, fmt.Errorf("SPEC.md has no Appendix F")
	}
	appendix := spec[start:]
	if end := strings.Index(appendix, "\n## "); end > 0 {
		appendix = appendix[:end]
	}
	kinds := map[string]expr.Kind{}
	for _, m := range functionRow.FindAllStringSubmatch(appendix, -1) {
		for _, cell := range m[1:3] {
			if cell == "" {
				continue
			}
			name := cell[:strings.IndexAny(cell+"(", "(")]
			kinds[name] = expr.Kind(m[3])
		}
	}
	if len(kinds) == 0 {
		return nil, fmt.Errorf("no function rows found in Appendix F")
	}
	return kinds, nil
}

// drift lists every way the registry and the table disagree; none means they are in sync.
func drift(reg *expr.Registry, kinds map[string]expr.Kind) []string {
	var found []string
	left := map[string]expr.Kind{}
	for name, kind := range kinds {
		left[name] = kind
	}
	for _, name := range reg.Names() {
		kind, ok := left[name]
		if !ok {
			found = append(found, name+" is registered but not in Appendix F")
			continue
		}
		if d, _ := reg.Lookup(name); d.Kind != kind {
			found = append(found, fmt.Sprintf("%s: Appendix F says %s, the registry says %s", name, kind, d.Kind))
		}
		delete(left, name)
	}
	for name := range left {
		found = append(found, name+" is in Appendix F but not registered")
	}
	sort.Strings(found)
	return found
}

func TestDocsyncAppendixF(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	kinds, err := appendixF(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if found := drift(expr.Builtins(), kinds); len(found) != 0 {
		t.Fatalf("SPEC Appendix F and expr.Builtins disagree:\n%s", strings.Join(found, "\n"))
	}
}

// TestDocsyncSeesPlantedDrift is exit line 5 of S2.2a for (c): each kind of drift, planted in a fixture, is named.
func TestDocsyncSeesPlantedDrift(t *testing.T) {
	const fixture = "## Appendix F. Functions\n\n| Function | Kind | Notes |\n|---|---|---|\n" +
		"| `abs(x)` | scalar | |\n| `cast(x, t)` | scalar | |\n| `chain(a, b…)` | pattern-only | |\n" +
		"| `coalesce(x, y)` | scalar | |\n| `count()` | aggregate | |\n| `ifThenElse(c, a, b)` | scalar | |\n" +
		"| `math:round(x)` | scalar | |\n| `maximum(x, y)` / `minimum(x, y)` | scalar | |\n"
	kinds, err := appendixF(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if found := drift(expr.Builtins(), kinds); len(found) != 0 {
		t.Fatalf("the fixture mirrors the registry, yet: %v", found)
	}
	cases := []struct {
		name string
		spec string
		want string
	}{
		{"a function only the spec has", fixture + "| `median(x)` | aggregate | |\n", "median is in Appendix F but not registered"},
		{"a function only the registry has", strings.Replace(fixture, "| `abs(x)` | scalar | |\n", "", 1), "abs is registered but not in Appendix F"},
		{"a kind that differs", strings.Replace(fixture, "| `count()` | aggregate |", "| `count()` | scalar |", 1), "count: Appendix F says scalar, the registry says aggregate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kinds, err := appendixF(c.spec)
			if err != nil {
				t.Fatal(err)
			}
			found := drift(expr.Builtins(), kinds)
			if len(found) != 1 || found[0] != c.want {
				t.Fatalf("got %v, want [%s]", found, c.want)
			}
		})
	}
	if _, err := appendixF("no appendix here"); err == nil {
		t.Fatal("a spec without Appendix F passed")
	}
	if _, err := appendixF("## Appendix F. Functions\n\nno table\n"); err == nil {
		t.Fatal("an empty table passed")
	}
}
