package parser_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/parser"
)

// corpusRoot is docs/language, seen from this package's directory.
const corpusRoot = "../../docs/language"

// expectLine matches the first line of an invalid-corpus file, for example
//
//	-- expect: E-PAT-001 (every pattern states `within <duration>`) @ 9:3
//
// The code and the hand-derived line:col are the oracle; the parser is not.
var expectLine = regexp.MustCompile(`^-- expect: (E-[A-Z]+-\d{3}) \(.*\) @ (\d+):(\d+)$`)

// TestInvalidCorpus proves S2.1 exit line 2: each invalid file yields exactly its [parse] code at
// exactly its @ line:col, with fix text.
func TestInvalidCorpus(t *testing.T) {
	t.Parallel()
	for _, path := range glob(t, filepath.Join(corpusRoot, "invalid", "*.cep")) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			code, line, col := expectation(t, path)
			app, err := parser.ParseFile(path)
			if err == nil {
				t.Fatalf("ParseFile(%s) accepted an invalid file (app=%v); want %s at %d:%d", path, app, code, line, col)
			}
			var perr *parser.Error
			if !errors.As(err, &perr) {
				t.Fatalf("ParseFile(%s) error is %T (%v); want *parser.Error", path, err, err)
			}
			if perr.Code != code || perr.Line != line || perr.Col != col {
				t.Errorf("ParseFile(%s) = %s at %d:%d; want %s at %d:%d (error: %v)",
					path, perr.Code, perr.Line, perr.Col, code, line, col, err)
			}
			if strings.TrimSpace(perr.Fix) == "" {
				t.Errorf("ParseFile(%s) %s carries no fix text", path, perr.Code)
			}
		})
	}
}

// TestValidCorpus proves the parse half of S2.1 exit line 1: every example, torture and generated
// eval file parses with no error.
func TestValidCorpus(t *testing.T) {
	t.Parallel()
	var files []string
	for _, pattern := range []string{
		filepath.Join(corpusRoot, "examples", "*.cep"),
		filepath.Join(corpusRoot, "examples", "torture", "*.cep"),
		filepath.Join(corpusRoot, "eval", "generated", "*.cep"),
	} {
		files = append(files, glob(t, pattern)...)
	}
	for _, path := range files {
		rel, err := filepath.Rel(corpusRoot, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			t.Parallel()
			app, err := parser.ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile(%s): %v", path, err)
			}
			if app == nil {
				t.Fatalf("ParseFile(%s) returned no error and no App", path)
			}
		})
	}
}

// glob returns the files matching pattern and fails when there are none: a run that selects zero
// files proves nothing.
func glob(t *testing.T, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no corpus files match %s", pattern)
	}
	return files
}

// expectation reads the code and position from an invalid file's first line.
func expectation(t *testing.T, path string) (code string, line, col int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(data), "\n")
	first = strings.TrimSuffix(first, "\r")
	m := expectLine.FindStringSubmatch(first)
	if m == nil {
		t.Fatalf("%s: first line %q is not `-- expect: E-XXX-NNN (...) @ line:col`", path, first)
	}
	line, err = strconv.Atoi(m[2])
	if err != nil {
		t.Fatal(err)
	}
	col, err = strconv.Atoi(m[3])
	if err != nil {
		t.Fatal(err)
	}
	return m[1], line, col
}
