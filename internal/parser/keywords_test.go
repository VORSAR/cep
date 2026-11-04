package parser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
