package main

// Known issue S3.3-1-1 (design docs/sprints/s3-3-2-design.md v2, §9.2; A11): a method's entry is named
// <Type>.<Method>. QA's first form of this file proved the goldens' rewrite format-only once — every entry of the
// goldens at 4501d6c, mapped through the new rule, equalled the tool's --write output line for line (green at build
// step 10, both goldens; the record holds the run). That proof pinned a base, so the first later API change would
// have turned it red; the lasting rule below replaces it (fixture edit, named in the record): in both committed
// goldens, every method entry's name is its signature's receiver type and method name, and nothing else has a dot.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// grEntries splits a golden into its entries: an entry starts at a line that begins with neither a tab nor a closing
// brace or parenthesis (a multi-line signature's continuation lines do).
func grEntries(text string) []string {
	var out []string
	for _, l := range strings.SplitAfter(text, "\n") {
		if l == "" {
			continue
		}
		if len(out) > 0 && (strings.HasPrefix(l, "\t") || strings.HasPrefix(l, "}") || strings.HasPrefix(l, ")")) {
			out[len(out)-1] += l
			continue
		}
		out = append(out, l)
	}
	return out
}

// grMethod is a method entry: `<pkg> <ident> func func (<recv> <*?Type[...]?>) <Method>(…`.
var grMethod = regexp.MustCompile(`^\S+ (\S+) func func \((?:\w+ )?\*?(\w+)(?:\[[^\]]*\])?\) (\w+)[\[(]`)

func TestGoldenMethodsAreNamedByTheirReceiver(t *testing.T) {
	t.Parallel()
	for _, golden := range []string{"api/exports.golden", "api/clock.golden"} {
		t.Run(filepath.Base(golden), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../..", golden))
			if err != nil {
				t.Fatal(err)
			}
			methods := 0
			for _, e := range grEntries(string(data)) {
				ident := strings.Fields(e)[1]
				if m := grMethod.FindStringSubmatch(e); m != nil {
					methods++
					if want := m[2] + "." + m[3]; ident != want {
						t.Errorf("%s: a method entry named %q; want %q: %q", golden, ident, want, strings.TrimSpace(e))
					}
					continue
				}
				if strings.Contains(ident, ".") {
					t.Errorf("%s: %q is not a method but its name has a dot: %q", golden, ident, strings.TrimSpace(e))
				}
			}
			if methods == 0 {
				t.Fatalf("%s has no method entry: the rule checks nothing", golden)
			}
		})
	}
}
