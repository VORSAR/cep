package conformance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep"
)

// corpus is the language's valid corpus (SPEC's header: normative).
const corpus = "../docs/language/examples"

// wantFiles is every corpus file: the ten examples and the two torture files. A corpus file added or removed moves
// it (and the differential swap proof's pinned count).
const wantFiles = 12

// notBuiltEntry is one entry of conformance/notbuilt.json (a law path): the construct a refusal names and the plan row
// that turns the file green. The expiry — red at T4 once gate/current-sprint reaches the row — is the gate's notbuilt
// row (S1.1c-1); this test only reads the list, and never loosely.
type notBuiltEntry struct {
	File      string `json:"file"`
	Construct string `json:"construct"`
	Sprint    string `json:"sprint"`
}

type notBuiltFile struct {
	Schema  int             `json:"schema"`
	Note    string          `json:"note"`
	Entries []notBuiltEntry `json:"entries"`
}

// notBuiltPath is the list, relative to this package's directory (go test runs a package in its own directory).
const notBuiltPath = "notbuilt.json"

var sprintRow = regexp.MustCompile(`^S[1-9][0-9]*(\.[1-9][0-9]*)?[a-z]?$`) // a row, never a part (no -<n>)

// parseNotBuilt is pure: the file's bytes to the map, or the first fault.
func parseNotBuilt(data []byte) (map[string]notBuiltEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f notBuiltFile
	if err := dec.Decode(&f); err != nil {
		return nil, err
	}
	var keys map[string]json.RawMessage // every top-level key must be present (design §1); unknown ones failed above
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	for _, key := range []string{"schema", "note", "entries"} {
		if _, ok := keys[key]; !ok {
			return nil, fmt.Errorf("missing key %q", key)
		}
	}
	if f.Schema != 1 {
		return nil, fmt.Errorf("schema %d, want 1", f.Schema)
	}
	out := make(map[string]notBuiltEntry, len(f.Entries))
	for i, e := range f.Entries {
		if e.File == "" || e.Construct == "" || e.Sprint == "" {
			return nil, fmt.Errorf("entries[%d]: file, construct and sprint must be non-empty", i)
		}
		if _, dup := out[e.File]; dup {
			return nil, fmt.Errorf("entries[%d]: duplicate file %q", i, e.File)
		}
		if !sprintRow.MatchString(e.Sprint) {
			return nil, fmt.Errorf("entries[%d]: %q names a part, not a row", i, e.Sprint)
		}
		out[e.File] = e
	}
	return out, nil
}

// loadNotBuilt reads conformance/notbuilt.json and fails the test on any fault.
func loadNotBuilt(t *testing.T) map[string]notBuiltEntry {
	t.Helper()
	data, err := os.ReadFile(notBuiltPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseNotBuilt(data)
	if err != nil {
		t.Fatalf("%s: %v", notBuiltPath, err)
	}
	return m
}

// TestCorpus is the conformance runner (design s3-3-2 §7): every corpus file deploys through the public API with its
// expect blocks passing (exit 1), or is refused as not built yet with the construct its entry names. It never skips:
// the gate judges a skip red.
func TestCorpus(t *testing.T) {
	notBuilt := loadNotBuilt(t)
	files := corpusFiles(t)
	if len(files) != wantFiles {
		t.Fatalf("read %d corpus files under %s, want %d: %v", len(files), corpus, wantFiles, files)
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(corpus, name))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // the ceiling: Deploy never waits
			defer cancel()
			r, err := cep.New(cep.Config{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := r.Close(ctx); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			_, err = r.Deploy(ctx, name, string(src), cep.WithConnectors(corpusFactory{}))
			want, listed := notBuilt[name]
			switch {
			case listed && err == nil:
				t.Errorf("%s deploys: remove its notBuilt entry (%s, %s)", name, want.Construct, want.Sprint)
			case listed && !errors.Is(err, errors.ErrUnsupported):
				t.Errorf("%s: %v; want a not-built refusal (errors.ErrUnsupported) naming %q", name, err, want.Construct)
			case listed && !strings.Contains(err.Error(), want.Construct):
				t.Errorf("%s: %v; want it to stop on %q: move its notBuilt entry", name, err, want.Construct)
			case listed:
				t.Logf("not built until %s (%s)", want.Sprint, want.Construct)
			case err != nil:
				t.Errorf("%s does not deploy:%s", name, describe(err))
			}
		})
	}
}

// corpusFiles is every *.cep under corpus and corpus/torture, as paths relative to corpus, sorted.
func corpusFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"", "torture"} {
		paths, err := filepath.Glob(filepath.Join(corpus, dir, "*.cep"))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			rel, err := filepath.Rel(corpus, p)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, filepath.ToSlash(rel))
		}
	}
	slices.Sort(out)
	return out
}

// describe is a refusal as the runner prints it: a *cep.DeployError one diagnostic per line, anything else as is.
func describe(err error) string {
	var de *cep.DeployError
	if !errors.As(err, &de) {
		return " " + err.Error()
	}
	var b strings.Builder
	for _, d := range de.Diagnostics {
		fmt.Fprintf(&b, "\n  %s %d:%d %s", d.Code, d.Line, d.Col, d.Message)
	}
	return b.String()
}
