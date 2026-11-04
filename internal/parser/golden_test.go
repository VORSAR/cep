package parser_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/parser"
)

// TestEveryExampleParsesToItsGoldenModel is exit line 1: each example's model, as canonical JSON, is locked under
// testdata/golden/. A golden is written only when CEP_UPDATE_GOLDEN=1 is set by hand; the gate never sets it.
func TestEveryExampleParsesToItsGoldenModel(t *testing.T) {
	update := os.Getenv("CEP_UPDATE_GOLDEN") == "1"
	for _, path := range corpusFiles(t) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		r, err := parser.Parse(path, string(data))
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		got, err := r.App.MarshalCanonical()
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		rel := strings.TrimSuffix(filepath.Base(path), ".cep")
		switch {
		case strings.Contains(path, "generated") && strings.Contains(path, "torture"):
			rel = "eval-torture-" + rel
		case strings.Contains(path, "generated"):
			rel = "eval-" + rel
		case strings.Contains(path, "torture"):
			rel = "torture-" + rel
		}
		golden := filepath.Join("testdata", "golden", rel+".json")
		if update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, append(got, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Errorf("%s: no golden at %s (run once with CEP_UPDATE_GOLDEN=1 and review the diff)", path, golden)
			continue
		}
		if !bytes.Equal(bytes.TrimSuffix(want, []byte("\n")), got) {
			t.Errorf("%s: the model differs from its golden %s", path, golden)
		}
	}
}
