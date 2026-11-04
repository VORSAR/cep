package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/parser"
)

var corpus = []string{
	filepath.Join("..", "..", "docs", "language", "examples", "*.cep"),
	filepath.Join("..", "..", "docs", "language", "examples", "torture", "*.cep"),
	filepath.Join("..", "..", "docs", "language", "eval", "generated", "*.cep"),
	filepath.Join("..", "..", "docs", "language", "eval", "generated", "torture", "*.cep"),
}

func corpusFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range corpus {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 21 {
		t.Fatalf("expected the corpus, found %d files", len(files))
	}
	return files
}

// TestTokensAndTriviaReproduceEveryCorpusFile is exit line 3: lexing and re-assembling every corpus file from its
// tokens plus trivia gives the file back byte for byte, comments and all.
func TestTokensAndTriviaReproduceEveryCorpusFile(t *testing.T) {
	for _, path := range corpusFiles(t) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := strings.TrimPrefix(string(data), "\xEF\xBB\xBF")
		r, err := parser.Parse(path, src)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if got := r.Reassemble(); got != src {
			t.Errorf("%s: the round trip changed the bytes", path)
		}
		comments := 0
		for _, tr := range r.Trivia {
			if strings.HasPrefix(tr.Value, "--") || strings.HasPrefix(tr.Value, "/*") {
				comments++
			}
		}
		if strings.Contains(src, "--") && comments == 0 {
			t.Errorf("%s: no comment was kept as trivia", path)
		}
	}
}
