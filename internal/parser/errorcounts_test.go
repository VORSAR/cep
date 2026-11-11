package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestErrorCountsMatchTheCatalogue pins SPEC.md Appendix C's four counts (codes, parse-level, counter-example
// files, static) to docs/language/errors.md's headings and docs/language/invalid/, and every E- code SPEC mentions
// to a heading of the catalogue (S1.1c-1 plan check, section 6 "The counts"). It reads the first non-blank line
// after the `## Appendix C` heading, so a later revision of the appendix moves one line.
func TestErrorCountsMatchTheCatalogue(t *testing.T) {
	docs := filepath.Join("..", "..", "docs", "language")
	spec := readDoc(t, filepath.Join(docs, "SPEC.md"))
	catalogue := readDoc(t, filepath.Join(docs, "errors.md"))

	line := firstLineAfterHeading(t, spec, "## Appendix C")
	counts := regexp.MustCompile("— " + `(\d+) codes: (\d+) parse-level \(each with at least one counter-example file in \[invalid/\]\(invalid/\); (\d+) files\), (\d+) static`)
	m := counts.FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("SPEC.md Appendix C's first line does not carry the four counts: %q", line)
	}
	said := map[string]int{"codes": atoi(t, m[1]), "parse-level": atoi(t, m[2]), "files": atoi(t, m[3]), "static": atoi(t, m[4])}

	heading := regexp.MustCompile(`^### (E-[A-Z]+-[0-9]{3}) .*\*\*\[(parse|static)\]\*\*`)
	headings := map[string]bool{}
	parse, static := 0, 0
	for _, l := range strings.Split(catalogue, "\n") {
		if h := heading.FindStringSubmatch(l); h != nil {
			headings[h[1]] = true
			if h[2] == "parse" {
				parse++
			} else {
				static++
			}
		}
	}
	files, err := filepath.Glob(filepath.Join(docs, "invalid", "*.cep"))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]int{"codes": parse + static, "parse-level": parse, "files": len(files), "static": static}
	for _, key := range []string{"codes", "parse-level", "files", "static"} {
		if said[key] != have[key] {
			t.Errorf("SPEC.md Appendix C says %d %s; errors.md and invalid/ have %d", said[key], key, have[key])
		}
	}

	codes := map[string]bool{}
	for _, c := range regexp.MustCompile(`E-[A-Z]+-[0-9]{3}`).FindAllString(spec, -1) {
		codes[c] = true
	}
	mentioned := make([]string, 0, len(codes))
	for c := range codes {
		mentioned = append(mentioned, c)
	}
	sort.Strings(mentioned)
	for _, c := range mentioned {
		if !headings[c] {
			t.Errorf("SPEC.md mentions %s, which is no heading of errors.md", c)
		}
	}
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// firstLineAfterHeading gives the first non-blank line after the line that starts with `heading`.
func firstLineAfterHeading(t *testing.T, text, heading string) string {
	t.Helper()
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, heading) {
			continue
		}
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) != "" {
				return next
			}
		}
	}
	t.Fatalf("no non-blank line after a heading starting %q", heading)
	return ""
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
