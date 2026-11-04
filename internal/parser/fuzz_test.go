package parser_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/parser"
)

// FuzzParse is the parser's fuzz target (S2.2a; the nightly fuzz row). Any input either parses, in which case its
// tokens and trivia reproduce it and its model encodes, or yields one catalogue error with a position. A panic,
// a hang or a wrong round trip is a crash. The corpus is the seed.
func FuzzParse(f *testing.F) {
	for _, pattern := range corpus {
		files, _ := filepath.Glob(pattern)
		for _, path := range files {
			if data, err := os.ReadFile(path); err == nil {
				f.Add(string(data))
			}
		}
	}
	f.Add("")
	f.Add("language cep/0.1;")
	f.Fuzz(func(t *testing.T, src string) {
		r, err := parser.Parse("fuzz", src)
		if err != nil {
			var perr *parser.Error
			if !errors.As(err, &perr) {
				t.Fatalf("not a *parser.Error: %v", err)
			}
			if perr.Code == "" || perr.Line < 1 || perr.Col < 1 {
				t.Fatalf("an error without a code or position: %+v", perr)
			}
			return
		}
		if got := r.Reassemble(); got != strings.TrimPrefix(src, "\xEF\xBB\xBF") {
			t.Fatalf("the round trip changed the bytes")
		}
		if _, err := r.App.MarshalCanonical(); err != nil {
			t.Fatalf("the model does not encode: %v", err)
		}
	})
}
