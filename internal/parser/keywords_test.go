package parser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// docsync (b) (ADR-0002 §3 as amended; S2.2a): the keyword table is pinned to grammar.lark's NAME exclusion
// list, the normative reserved-word list, by a test rather than a generator.

var nameExclusion = regexp.MustCompile(`NAME: /\(\?!\(\?i:([a-z_|]+)\)\\b\)`)

// grammarKeywords reads the NAME terminal's exclusion list out of grammar.lark's text, in its order.
func grammarKeywords(grammar []byte) ([]string, error) {
	m := nameExclusion.FindSubmatch(grammar)
	if m == nil {
		return nil, errors.New("grammar.lark: the NAME terminal's exclusion list was not found")
	}
	return strings.Split(string(m[1]), "|"), nil
}

// keywordDrift says how the grammar's list and the table differ, or "" when they are equal, in order.
func keywordDrift(grammar, table []string) string {
	if len(grammar) == 0 || len(grammar) != len(table) {
		return fmt.Sprintf("grammar.lark lists %d reserved words, keywords.go %d", len(grammar), len(table))
	}
	for i := range grammar {
		if grammar[i] != table[i] {
			return fmt.Sprintf("word %d: grammar.lark has %q, keywords.go has %q", i, grammar[i], table[i])
		}
	}
	return ""
}

func TestKeywordsEqualGrammarLark(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "grammar.lark"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := grammarKeywords(data)
	if err != nil {
		t.Fatal(err)
	}
	if d := keywordDrift(want, keywordList); d != "" {
		t.Fatal(d)
	}
	for k := range reservedFuture {
		if !isKeyword(k) {
			t.Errorf("reserved-future word %q is not in the keyword table", k)
		}
	}
}

// TestKeywordDriftIsSeen is exit line 5 of S2.2a for (b): a planted drift in either direction is named.
func TestKeywordDriftIsSeen(t *testing.T) {
	planted := []byte(`NAME: /(?!(?i:language|param|extra)\b)[A-Za-z_][A-Za-z0-9_]*/`)
	words, err := grammarKeywords(planted)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(words, ",") != "language,param,extra" {
		t.Fatalf("read %v from the fixture", words)
	}
	if d := keywordDrift(words, keywordList); !strings.HasPrefix(d, "grammar.lark lists 3 reserved words") {
		t.Fatalf("a fixture of three words against the table: %q", d)
	}
	swapped := append([]string(nil), keywordList...)
	swapped[0], swapped[1] = swapped[1], swapped[0]
	if d := keywordDrift(swapped, keywordList); !strings.HasPrefix(d, "word 0: ") {
		t.Fatalf("a swapped pair: %q", d)
	}
	if d := keywordDrift(keywordList, keywordList); d != "" {
		t.Fatalf("the table against itself: %q", d)
	}
	if _, err := grammarKeywords([]byte("NAME: /[A-Za-z_]+/")); err == nil {
		t.Fatal("a grammar without the exclusion list passed")
	}
}

// specReserved reads the reserved words SPEC Appendix B lists: the words of the first fenced block after its heading
// (S3.2c: a spec-only reader, an LLM generating rules, has the list without grammar.lark; the grouped-key evaluation's
// note).
func specReserved(spec string) ([]string, error) {
	i := strings.Index(spec, "## Appendix B.")
	if i < 0 {
		return nil, errors.New("SPEC.md: no Appendix B")
	}
	rest := spec[i:]
	if j := strings.Index(rest[1:], "\n## "); j >= 0 {
		rest = rest[:j+1]
	}
	open := strings.Index(rest, "```")
	if open < 0 {
		return nil, errors.New("SPEC.md Appendix B: no fenced list of the reserved words")
	}
	body := rest[open+3:]
	body = body[strings.IndexByte(body, '\n')+1:]
	end := strings.Index(body, "```")
	if end < 0 {
		return nil, errors.New("SPEC.md Appendix B: the fenced list is not closed")
	}
	return strings.Fields(body[:end]), nil
}

// reservedDrift says how SPEC's list and the grammar's differ as sets, or "" when they are equal.
func reservedDrift(spec, grammar []string) string {
	in := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	s, g := in(spec), in(grammar)
	var missing, extra []string
	for _, w := range grammar {
		if !s[w] {
			missing = append(missing, w)
		}
	}
	for _, w := range spec {
		if !g[w] {
			extra = append(extra, w)
		}
	}
	if len(missing)+len(extra) == 0 && len(spec) == len(s) {
		return ""
	}
	return fmt.Sprintf("SPEC Appendix B lacks %v, has extra %v, lists %d words for %d distinct", missing, extra, len(spec), len(s))
}

// TestSpecAppendixBListsEveryReservedWord (S3.2c, exit 2): SPEC Appendix B lists exactly grammar.lark's reserved words.
func TestSpecAppendixBListsEveryReservedWord(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	words, err := specReserved(string(spec))
	if err != nil {
		t.Fatal(err)
	}
	if d := reservedDrift(words, keywordList); d != "" {
		t.Fatal(d)
	}
	if !slices.IsSorted(words) {
		t.Fatal("SPEC Appendix B's reserved words are not in alphabetical order, as its text says (S3.2c review R5)")
	}
}

// TestSpecReservedDriftIsSeen: a word missing, a word extra and a word twice are each named.
func TestSpecReservedDriftIsSeen(t *testing.T) {
	g := []string{"rule", "stream", "within"}
	for name, spec := range map[string][]string{"missing": {"rule", "stream"}, "extra": {"rule", "stream", "within", "foo"}, "twice": {"rule", "stream", "within", "rule"}} {
		if reservedDrift(spec, g) == "" {
			t.Errorf("%s: no drift reported", name)
		}
	}
	if d := reservedDrift([]string{"within", "rule", "stream"}, g); d != "" {
		t.Errorf("the same words in another order: %q", d)
	}
}
