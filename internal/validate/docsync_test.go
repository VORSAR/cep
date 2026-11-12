package validate

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// docsync (b)'s catalogue half, in its own file (S1.1c-2 design §4.2): an exact gate/law.json entry, so the
// instrument pin counts its cases and a change to it is a named LAW Δ line. Moved unchanged from static_test.go.

// staticEntries reads errors.md's [static] codes with their fix lines.
func staticEntries(text string) map[string]string {
	heading := regexp.MustCompile(`^### (E-[A-Z]+-[0-9]{3}) .*\*\*\[(parse|static)\]\*\*`)
	out := map[string]string{}
	code := ""
	for _, line := range strings.Split(text, "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			code = ""
			if m[2] == "static" {
				code = m[1]
			}
			continue
		}
		if strings.HasPrefix(line, "### ") {
			code = ""
		}
		if i := strings.Index(line, "Fix: "); code != "" && i >= 0 && (i == 0 || strings.HasSuffix(line[:i], ". ")) {
			out[code] = line[i+len("Fix: "):]
			code = ""
		}
	}
	return out
}

// catalogDrift lists how the catalogue text and catalog differ: missing either way, or a different fix text.
func catalogDrift(text string, cat map[string]Entry) []string {
	want := staticEntries(text)
	var out []string
	for code, fix := range want {
		e, ok := cat[code]
		switch {
		case !ok:
			out = append(out, code+" is [static] in errors.md but not in catalog.go")
		case e.Fix != fix:
			out = append(out, fmt.Sprintf("%s: errors.md's fix %q, catalog.go's %q", code, fix, e.Fix))
		}
	}
	for code := range cat {
		if _, ok := want[code]; !ok {
			out = append(out, code+" is in catalog.go but not [static] in errors.md")
		}
	}
	sort.Strings(out)
	return out
}

// TestDocsyncStaticCatalog is docsync (b) (S2.2b exit line 3): errors.md's [static] codes and catalog.go agree.
func TestDocsyncStaticCatalog(t *testing.T) {
	t.Parallel()
	if d := catalogDrift(readFile(t, filepath.Join(languageDocs, "errors.md")), catalog); len(d) != 0 {
		t.Fatalf("errors.md and catalog.go disagree:\n%s", strings.Join(d, "\n"))
	}
}

// TestDocsyncSeesPlantedDrift: each kind of drift, planted, is named.
func TestDocsyncSeesPlantedDrift(t *testing.T) {
	t.Parallel()
	const doc = "### E-AAA-001 One **[static]**\nRule: x.\nFix: do one.\n\n### E-BBB-001 Two **[static]**\nFix: do two.\n" +
		"### E-CCC-001 Three **[parse]**\nFix: not ours.\n"
	cat := map[string]Entry{"E-AAA-001": {Fix: "do one."}, "E-BBB-001": {Fix: "do two."}}
	if d := catalogDrift(doc, cat); len(d) != 0 {
		t.Fatalf("an agreeing fixture drifted: %v", d)
	}
	cases := map[string]struct {
		doc  string
		cat  map[string]Entry
		want string
	}{
		"a code only errors.md has":  {doc + "### E-DDD-001 Four **[static]**\nFix: four.\n", cat, "E-DDD-001 is [static] in errors.md but not in catalog.go"},
		"a code only catalog.go has": {doc, map[string]Entry{"E-AAA-001": {Fix: "do one."}, "E-BBB-001": {Fix: "do two."}, "E-ZZZ-001": {}}, "E-ZZZ-001 is in catalog.go but not [static] in errors.md"},
		"a fix that differs":         {strings.Replace(doc, "do two.", "do 2.", 1), cat, `E-BBB-001: errors.md's fix "do 2.", catalog.go's "do two."`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if d := catalogDrift(c.doc, c.cat); len(d) != 1 || d[0] != c.want {
				t.Fatalf("got %v, want [%s]", d, c.want)
			}
		})
	}
}
