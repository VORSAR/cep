package playback_test

// Tier 2 (qa-strategy §3; design s3-3-2 §8.1–§8.2): the full outputs and drop counts of every expect block of every
// built corpus file, locked in conformance/baseline/. TestBaseline compares a fresh run with the stored files; with
// CEP_BASELINE_WRITE=<dir> it writes the fresh run there instead (gate/baseline/regen.py's step 2; the gate never
// sets it). Prefix: bl.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/validate"
)

const (
	blCorpus   = "../../docs/language/examples"
	blBaseline = "../../conformance/baseline"
	blSpec     = "../../docs/language/SPEC.md"
	blFormat   = 1
	// blWantFiles and blWantBlocks are the corpus's files and the blocks of its built files (07: 6, 09: 4, 10: 5). A
	// corpus change moves them (design s3-3-2 §8.2; the breaker's S6: the public API cannot count blocks).
	blWantFiles  = 12
	blWantBlocks = 15
)

// blHeader is a baseline file's first line.
type blHeader struct {
	File         string `json:"file"`
	Format       int    `json:"format"`
	SourceSHA256 string `json:"source_sha256"`
	SpecRev      int    `json:"spec_rev"`
}

// TestBaseline locks Tier 2: every built corpus file's blocks, run fresh, equal their stored lines byte for byte (the
// header's spec_rev aside, which only the regen tool writes).
func TestBaseline(t *testing.T) {
	out := os.Getenv("CEP_BASELINE_WRITE")
	files := blFiles(t)
	if len(files) != blWantFiles {
		t.Fatalf("read %d corpus files, want %d: %v", len(files), blWantFiles, files)
	}
	built, blocks := map[string]bool{}, 0
	for _, name := range files {
		lines, n, ok := blRun(t, name)
		if !ok {
			continue // not built yet: the conformance runner checks the construct
		}
		built[name+".jsonl"], blocks = true, blocks+n
		path := filepath.Join(blBaseline, filepath.Base(name)+".jsonl")
		if out != "" {
			lines[0] = blHeaderLine(t, name, blSpecRev(t))
			if err := os.WriteFile(filepath.Join(out, filepath.Base(name)+".jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		blCompare(t, name, path, lines)
	}
	if out != "" {
		return // a fresh run for the regen tool: an added block is its to classify, not this test's to refuse
	}
	if blocks != blWantBlocks {
		t.Errorf("ran %d blocks in the built corpus files, want %d", blocks, blWantBlocks)
	}
	stored, err := filepath.Glob(filepath.Join(blBaseline, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range stored {
		if !built[filepath.Base(p)] {
			t.Errorf("%s has no built corpus file", p)
		}
	}
}

// blFiles is every *.cep under the corpus and its torture directory, relative to the corpus, sorted.
func blFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"", "torture"} {
		paths, err := filepath.Glob(filepath.Join(blCorpus, dir, "*.cep"))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			rel, err := filepath.Rel(blCorpus, p)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, filepath.ToSlash(rel))
		}
	}
	slices.Sort(out)
	return out
}

// blRun runs one corpus file's blocks and returns its baseline lines (the header first, its spec_rev 0), the number
// of blocks, and false when the file is not built yet.
func blRun(t *testing.T, name string) ([]string, int, bool) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(blCorpus, name))
	if err != nil {
		t.Fatal(err)
	}
	res, err := parser.Parse(name, string(src))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	checked, diags := validate.Check(res.App, validate.Options{})
	if checked == nil {
		t.Fatalf("%s: %v", name, diags)
	}
	p, err := plan.Build(checked)
	if errors.Is(err, plan.ErrNotBuilt) {
		return nil, 0, false
	}
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	results, err := playback.RunExpects(p)
	if err != nil {
		t.Fatalf("%s: RunExpects: %v", name, err)
	}
	keys := blKeys(t, res.App)
	if len(results) != len(keys) {
		t.Fatalf("%s: %d block results for %d blocks", name, len(results), len(keys))
	}
	lines := []string{blHeaderLine(t, name, 0)}
	for i, r := range results {
		var outs []json.RawMessage
		for _, e := range r.Outputs {
			b, err := playback.AppendEvent(nil, p.Streams[e.StreamID].Schema, e)
			if err != nil {
				t.Fatalf("%s: rule %s block %d: an output cannot be encoded: %v", name, r.Rule, r.Block, err)
			}
			outs = append(outs, b)
		}
		line := map[string]any{
			"block": r.Block,
			"counts": map[string]int64{
				"errored": r.Counts.Errored, "key_errors": r.Counts.KeyErrors, "late": r.Counts.Late, "panics": r.Counts.Panics,
			},
			"key":     keys[i],
			"outputs": outs,
			"rule":    r.Rule,
		}
		if outs == nil {
			line["outputs"] = []json.RawMessage{}
		}
		lines = append(lines, blJSON(t, line))
	}
	return lines, len(results), true
}

// blKeys is each block's content key, in RunExpects' order (design s3-3-2 §8.1 as built): the SHA-256 of the canonical
// form of the file's params, the declarations the owning rule uses — the streams it reads, its target, the tables it
// reads — and the rule holding only this block; equal keys in one rule take #2, #3, …. A declaration the rule does not
// use (a new rule's own stream) moves no key, so an engine drift in the other blocks is still judged by the classes.
func blKeys(t *testing.T, app *model.App) []string {
	t.Helper()
	var keys []string
	for _, r := range app.Rules {
		streams, tables := blUses(&r)
		one := model.App{Language: app.Language, Params: app.Params}
		for _, s := range app.Streams {
			if streams[s.Name] {
				one.Streams = append(one.Streams, s)
			}
		}
		for _, tb := range app.Tables {
			if tables[tb.Name] {
				one.Tables = append(one.Tables, tb)
			}
		}
		seen := map[string]int{}
		for _, b := range r.Expects {
			rule := r
			rule.Expects = []model.Expect{b}
			one.Rules = []model.Rule{rule}
			text, err := one.MarshalCanonical()
			if err != nil {
				t.Fatalf("rule %s: %v", r.Name, err)
			}
			sum := sha256.Sum256(text)
			k := hex.EncodeToString(sum[:])
			seen[k]++
			if n := seen[k]; n > 1 {
				k += fmt.Sprintf("#%d", n)
			}
			keys = append(keys, k)
		}
	}
	return keys
}

// blUses is the streams a rule reads or writes and the tables its expressions read (an `in` operand, a text
// operator's table).
func blUses(r *model.Rule) (map[string]bool, map[string]bool) {
	streams, tables := map[string]bool{r.InsertInto: true}, map[string]bool{}
	var walk func(e *model.Expr)
	walk = func(e *model.Expr) {
		if e.Kind == model.ExprIn && e.Table != "" {
			tables[e.Table] = true
		}
		if e.Kind == model.ExprText && e.Operand.Table != "" {
			tables[e.Operand.Table] = true
		}
		for i := range e.Args {
			walk(&e.Args[i])
		}
	}
	if src := &r.Body.Source; src.Stream != "" { // a plain or window body; S4.1 adds a pattern's steps here
		streams[src.Stream] = true
		walk(&src.Filter)
	}
	for i := range r.Select {
		walk(&r.Select[i].Expr)
	}
	return streams, tables
}

func blHeaderLine(t *testing.T, name string, specRev int) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(blCorpus, name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(src)
	return blJSON(t, blHeader{File: filepath.Base(name), Format: blFormat, SourceSHA256: hex.EncodeToString(sum[:]), SpecRev: specRev})
}

// blJSON is v as one line: sorted keys, no spaces, no HTML escaping (the outputs' canonical text is kept as written).
func blJSON(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// blSpecRev is SPEC.md's revision, from its status line.
func blSpecRev(t *testing.T) int {
	t.Helper()
	text, err := os.ReadFile(blSpec)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`rev (\d+) \(`).FindSubmatch(text)
	if m == nil {
		t.Fatalf("%s: no `rev N (` in its status line", blSpec)
	}
	rev, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	return rev
}

// blCompare compares a fresh run with its stored file: the header without spec_rev, then every block line.
func blCompare(t *testing.T, name, path string, fresh []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: no baseline at %s (record one with make regen-baseline)", name, path)
		return
	}
	stored := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	var got, want blHeader
	if err := json.Unmarshal([]byte(fresh[0]), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(stored[0]), &want); err != nil {
		t.Errorf("%s: its header does not parse: %v", path, err)
		return
	}
	got.SpecRev, want.SpecRev = 0, 0
	if got != want {
		t.Errorf("%s: header %+v, stored %+v", name, got, want)
	}
	for i := 1; i < max(len(fresh), len(stored)); i++ {
		switch {
		case i >= len(stored):
			t.Errorf("%s: block line %d is new:\n  fresh  %s", name, i, fresh[i])
		case i >= len(fresh):
			t.Errorf("%s: block line %d is gone:\n  stored %s", name, i, stored[i])
		case fresh[i] != stored[i]:
			t.Errorf("%s: block line %d differs:\n  fresh  %s\n  stored %s", name, i, fresh[i], stored[i])
		}
	}
}
