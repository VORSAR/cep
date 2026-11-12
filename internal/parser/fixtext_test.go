package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// markerComment is one of the four marker forms the rule-fragment test reads after a span (design note v2 §2.3):
// <!--cep:<kind>-->, <!--cep-meta:<kind>-->, <!--cep-invalid:<kind>-->, <!--text-->.
var markerComment = regexp.MustCompile(`<!--(?:(?:cep|cep-meta|cep-invalid):[a-z]+|text)-->`)

// stripMarkers removes the marker comments from a catalogue line: a fix text is compared as the reader sees it.
func stripMarkers(line string) string { return markerComment.ReplaceAllString(line, "") }

// TestStripMarkers: each of the four forms goes; any other comment, and the text around, stays.
func TestStripMarkers(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"add `within <duration>`<!--cep-meta:within--> after it.", "add `within <duration>` after it."},
		{"`within 5 min`<!--cep:within-->, `-> and not`<!--cep-invalid:chain-->", "`within 5 min`, `-> and not`"},
		{"`a ± b`<!--text--> is notation.", "`a ± b` is notation."},
		{"no marker here.", "no marker here."},
		{"`x`<!--cep-frob:x--> stays.", "`x`<!--cep-frob:x--> stays."},
	}
	for _, c := range cases {
		if got := stripMarkers(c.in); got != c.want {
			t.Errorf("stripMarkers(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// TestFixTextsMatchTheCatalogue pins every fix text here to docs/language/errors.md, both ways: every [parse] code
// in the catalogue, and every [static] code in LoweringCodes, has a fix here, the catalogue's, verbatim; and every
// fix here is one of those.
func TestFixTextsMatchTheCatalogue(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "errors.md"))
	if err != nil {
		t.Fatal(err)
	}
	heading := regexp.MustCompile(`^### (E-[A-Z]+-[0-9]{3}) .*\*\*\[(parse|static)\]\*\*`)
	lowering := map[string]bool{}
	for _, c := range LoweringCodes {
		lowering[c] = true
	}
	want := map[string]string{}
	code := ""
	for _, line := range strings.Split(string(data), "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			code = m[1]
			if m[2] == "static" && !lowering[code] {
				code = ""
			}
			continue
		}
		if strings.HasPrefix(line, "### ") {
			code = ""
		}
		if i := strings.Index(line, "Fix: "); code != "" && i >= 0 && (i == 0 || strings.HasSuffix(line[:i], ". ")) {
			want[code] = stripMarkers(line[i+len("Fix: "):])
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
			t.Errorf("%s has a fix here but is neither a [parse] code nor a lowering [static] code in the catalogue", code)
		}
	}
}
