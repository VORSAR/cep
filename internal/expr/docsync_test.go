package expr_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/expr"
)

// TestDocsyncAppendixF is docsync (c) (ADR-0015 §2.2): SPEC Appendix F's function table and the registry name the
// same callables with the same kinds, so neither can drift from the other.
func TestDocsyncAppendixF(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	start := strings.Index(spec, "## Appendix F.")
	if start < 0 {
		t.Fatal("SPEC.md has no Appendix F")
	}
	appendix := spec[start:]
	if end := strings.Index(appendix, "\n## "); end > 0 {
		appendix = appendix[:end]
	}
	rowRe := regexp.MustCompile("(?m)^\\| `([^`]+)`(?: / `([^`]+)`)? \\| (scalar|aggregate|pattern-only) \\|")
	kinds := map[string]expr.Kind{}
	for _, m := range rowRe.FindAllStringSubmatch(appendix, -1) {
		for _, cell := range m[1:3] {
			if cell == "" {
				continue
			}
			name := cell[:strings.IndexAny(cell+"(", "(")]
			kinds[name] = expr.Kind(m[3])
		}
	}
	if len(kinds) == 0 {
		t.Fatal("no function rows found in Appendix F")
	}
	reg := expr.Builtins()
	for _, name := range reg.Names() {
		kind, ok := kinds[name]
		if !ok {
			t.Errorf("%s is registered but not in Appendix F", name)
			continue
		}
		if d, _ := reg.Lookup(name); d.Kind != kind {
			t.Errorf("%s: Appendix F says %s, the registry says %s", name, kind, d.Kind)
		}
		delete(kinds, name)
	}
	for name := range kinds {
		t.Errorf("%s is in Appendix F but not registered", name)
	}
}
