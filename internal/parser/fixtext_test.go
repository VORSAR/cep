package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestFixTextsMatchTheCatalogue pins every [parse] code's fix text to docs/language/errors.md, both ways: every
// [parse] code in the catalogue has a fix here, and every fix here is the catalogue's, verbatim.
func TestFixTextsMatchTheCatalogue(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "errors.md"))
	if err != nil {
		t.Fatal(err)
	}
	heading := regexp.MustCompile(`^### (E-[A-Z]+-[0-9]{3}) .*\*\*\[parse\]\*\*`)
	want := map[string]string{}
	code := ""
	for _, line := range strings.Split(string(data), "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			code = m[1]
			continue
		}
		if strings.HasPrefix(line, "### ") {
			code = ""
		}
		if i := strings.Index(line, "Fix: "); code != "" && i >= 0 && (i == 0 || strings.HasSuffix(line[:i], ". ")) {
			want[code] = line[i+len("Fix: "):]
		}
	}
	if len(want) == 0 {
		t.Fatal("no [parse] code with a Fix line found in errors.md")
	}
	for code, fix := range want {
		if fixes[code] != fix {
			t.Errorf("%s: fix text differs\n catalogue: %s\n parser:    %s", code, fix, fixes[code])
		}
	}
	for code := range fixes {
		if _, ok := want[code]; !ok {
			t.Errorf("%s has a fix here but is not a [parse] code in the catalogue", code)
		}
	}
}
