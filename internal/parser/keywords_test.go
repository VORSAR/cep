package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestKeywordsEqualGrammarLark pins the keyword table to grammar.lark's NAME exclusion list, the normative
// reserved-word list (ADR-0002 §3 as amended: a test, not a generator).
func TestKeywordsEqualGrammarLark(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "grammar.lark"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`NAME: /\(\?!\(\?i:([a-z_|]+)\)\\b\)`).FindSubmatch(data)
	if m == nil {
		t.Fatal("grammar.lark: the NAME terminal's exclusion list was not found")
	}
	want := strings.Split(string(m[1]), "|")
	if len(want) == 0 || len(want) != len(keywordList) {
		t.Fatalf("grammar.lark lists %d reserved words, keywords.go %d", len(want), len(keywordList))
	}
	for i := range want {
		if want[i] != keywordList[i] {
			t.Fatalf("word %d: grammar.lark has %q, keywords.go has %q", i, want[i], keywordList[i])
		}
	}
	for k := range reservedFuture {
		if !isKeyword(k) {
			t.Errorf("reserved-future word %q is not in the keyword table", k)
		}
	}
}
