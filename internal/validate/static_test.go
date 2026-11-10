package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/parser"
)

// The tests here read .cep programs through internal/parser: the one import test_imports allows this package's
// tests (owner, 2026-11-04, S2.2b A4).

const languageDocs = "../../docs/language"

// expectLine is the first line of a test program: `-- expect: <CODE> (<reason>) @ <line>:<col>` (S2.1's format).
var expectLine = regexp.MustCompile(`^-- expect: (E-[A-Z]+-\d{3}) \(.*\) @ (\d+):(\d+)$`)

// finding is one diagnostic as the tests compare it, from either the parser (a lowering code) or Check.
type finding struct {
	Code      string
	Line, Col int
	Msg, Fix  string
}

func (f finding) String() string { return fmt.Sprintf("%s at %d:%d: %s", f.Code, f.Line, f.Col, f.Msg) }

// findings runs a program the way the facade will: parse (its lowering's one error ends it), then Check.
func findings(t *testing.T, name, src string, opts Options) []finding {
	t.Helper()
	res, err := parser.Parse(name, src)
	if err != nil {
		var perr *parser.Error
		if !errors.As(err, &perr) {
			t.Fatalf("%s: parse error %T: %v", name, err, err)
		}
		return []finding{{perr.Code, perr.Line, perr.Col, perr.Msg, perr.Fix}}
	}
	_, diags := Check(res.App, opts)
	out := []finding{}
	for _, d := range diags {
		out = append(out, finding{d.Code, d.Line, d.Col, d.Msg, d.Fix})
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func programs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "*.cep"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no test programs in testdata/ (%v)", err)
	}
	return files
}

// TestStatic is S2.2b exit line 1's first half: each of QA's programs, written from the spec before this package,
// yields exactly its one finding — that code, at that line and column, with errors.md's fix text.
func TestStatic(t *testing.T) {
	t.Parallel()
	for _, path := range programs(t) {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".cep"), func(t *testing.T) {
			t.Parallel()
			src := readFile(t, path)
			m := expectLine.FindStringSubmatch(strings.SplitN(src, "\n", 2)[0])
			if m == nil {
				t.Fatalf("%s: the first line is not an expect line", path)
			}
			line, _ := strconv.Atoi(m[2])
			col, _ := strconv.Atoi(m[3])
			got := findings(t, path, src, Options{})
			if len(got) != 1 {
				t.Fatalf("want exactly one finding, %s at %d:%d; got %d: %v", m[1], line, col, len(got), got)
			}
			f := got[0]
			t.Logf("%s", f) // the owner's check: go test ./internal/validate -run 'TestStatic/<file>' -v
			if f.Code != m[1] || f.Line != line || f.Col != col {
				t.Fatalf("want %s at %d:%d; got %s", m[1], line, col, f)
			}
			if want := catalog[f.Code].Fix; f.Fix == "" || f.Fix != want {
				t.Fatalf("fix text %q; errors.md says %q", f.Fix, want)
			}
		})
	}
}

func glob(t *testing.T, patterns ...string) []string {
	t.Helper()
	var files []string
	for _, pattern := range patterns {
		found, err := filepath.Glob(filepath.Join(languageDocs, pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, found...)
	}
	return files
}

// TestCorpusClean is exit line 1's second half: no corpus example raises any code, lints included. The examples
// are the language's reference programs. (eval/generated is a dated evaluation record, pinned below; its torture
// subdirectory is grammar material for the differential, not valid programs.)
func TestCorpusClean(t *testing.T) {
	t.Parallel()
	files := glob(t, "examples/*.cep", "examples/torture/*.cep")
	if len(files) != 12 { // example 10 (grouped keys) joins at S3.2b-2
		t.Fatalf("found %d example programs, want 12", len(files))
	}
	for _, path := range files {
		t.Run(strings.TrimPrefix(path, languageDocs+"/"), func(t *testing.T) {
			t.Parallel()
			if got := findings(t, path, readFile(t, path), Options{}); len(got) != 0 {
				t.Fatalf("a corpus example raised %v", got)
			}
		})
	}
}

// TestGeneratedEvalFindings pins what the twelve programs of the 2026-11-01 language evaluation raise now. The
// evaluation audited them clean against the rules of that day; two later rules make four of them invalid: SPEC
// §4.2 asks every stream a window rule reads for `idle after`, and rev 3 removed `late events allow`. The record
// is not rewritten; this test keeps the difference visible and fixed.
func TestGeneratedEvalFindings(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"t04.cep": {"E-IDLE-001 at 5:3", "E-LATE-002 at 14:15"},
		"t05.cep": {"E-IDLE-001 at 5:3"},
		"t06.cep": {"E-IDLE-001 at 5:3"},
		"t07.cep": {"E-IDLE-001 at 5:3"},
	}
	files := glob(t, "eval/generated/*.cep")
	if len(files) != 12 {
		t.Fatalf("found %d generated programs, want 12", len(files))
	}
	for _, path := range files {
		var got []string
		for _, f := range findings(t, path, readFile(t, path), Options{}) {
			got = append(got, fmt.Sprintf("%s at %d:%d", f.Code, f.Line, f.Col))
		}
		if w := want[filepath.Base(path)]; strings.Join(got, "; ") != strings.Join(w, "; ") {
			t.Errorf("%s: got %v, want %v", path, got, w)
		}
	}
}

// TestRev5EvalProgramsCheckClean: the five programs of SPEC rev 5's grouped-key evaluation (2026-11-06, S3.2b-2's
// design §1.6; docs/language/eval/rev5-grouped-keys.md) were audited clean by hand against errors.md before the
// validator knew grouped keys; the validator agrees.
func TestRev5EvalProgramsCheckClean(t *testing.T) {
	t.Parallel()
	files := glob(t, "eval/generated/rev5/*.cep")
	if len(files) != 5 {
		t.Fatalf("found %d rev 5 evaluation programs, want 5", len(files))
	}
	for _, path := range files {
		if got := findings(t, path, readFile(t, path), Options{}); len(got) != 0 {
			t.Errorf("%s: %v; want none", path, got)
		}
	}
}

// TestEveryStaticCodeIsAccountedFor: each [static] code has a test program, or is unreachable in v0.1 with a unit
// test of its check, or names the sprint that implements it. Nothing is skipped silently.
func TestEveryStaticCodeIsAccountedFor(t *testing.T) {
	t.Parallel()
	tested := map[string]bool{}
	for _, path := range programs(t) {
		if m := expectLine.FindStringSubmatch(strings.SplitN(readFile(t, path), "\n", 2)[0]); m != nil {
			tested[m[1]] = true
		}
	}
	codes := make([]string, 0, len(catalog))
	for code := range catalog {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		e := catalog[code]
		switch {
		case e.Later != "" && tested[code]:
			t.Errorf("%s has a test program but is marked for %s", code, e.Later)
		case e.Unreachable && tested[code]:
			t.Errorf("%s has a test program but is marked unreachable", code)
		case e.Deploy && tested[code]:
			t.Errorf("%s is raised at deploy, not by Check, but has a test program", code)
		case e.Later == "" && !e.Unreachable && !e.Deploy && !tested[code]:
			t.Errorf("%s has no test program in testdata/", code)
		}
	}
	for code := range tested {
		if _, ok := catalog[code]; !ok {
			t.Errorf("a test program expects %s, which is not a [static] code", code)
		}
	}
}

// staticEntries reads errors.md's [static] codes with their fix lines.
func staticEntries(text string) map[string]string {
	heading := regexp.MustCompile(`^### (E-[A-Z]+-[0-9]{3}) .*\*\*\[(parse|static)\]\*\*`)
	out := map[string]string{}
	code := ""
	for _, line := range strings.Split(text, "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			code = ""
			if m[2] == "static" {
				code = m[1]
			}
			continue
		}
		if strings.HasPrefix(line, "### ") {
			code = ""
		}
		if i := strings.Index(line, "Fix: "); code != "" && i >= 0 && (i == 0 || strings.HasSuffix(line[:i], ". ")) {
			out[code] = line[i+len("Fix: "):]
			code = ""
		}
	}
	return out
}

// catalogDrift lists how the catalogue text and catalog differ: missing either way, or a different fix text.
func catalogDrift(text string, cat map[string]Entry) []string {
	want := staticEntries(text)
	var out []string
	for code, fix := range want {
		e, ok := cat[code]
		switch {
		case !ok:
			out = append(out, code+" is [static] in errors.md but not in catalog.go")
		case e.Fix != fix:
			out = append(out, fmt.Sprintf("%s: errors.md's fix %q, catalog.go's %q", code, fix, e.Fix))
		}
	}
	for code := range cat {
		if _, ok := want[code]; !ok {
			out = append(out, code+" is in catalog.go but not [static] in errors.md")
		}
	}
	sort.Strings(out)
	return out
}

// TestDocsyncStaticCatalog is docsync (b) (S2.2b exit line 3): errors.md's [static] codes and catalog.go agree.
func TestDocsyncStaticCatalog(t *testing.T) {
	t.Parallel()
	if d := catalogDrift(readFile(t, filepath.Join(languageDocs, "errors.md")), catalog); len(d) != 0 {
		t.Fatalf("errors.md and catalog.go disagree:\n%s", strings.Join(d, "\n"))
	}
}

// TestDocsyncSeesPlantedDrift: each kind of drift, planted, is named.
func TestDocsyncSeesPlantedDrift(t *testing.T) {
	t.Parallel()
	const doc = "### E-AAA-001 One **[static]**\nRule: x.\nFix: do one.\n\n### E-BBB-001 Two **[static]**\nFix: do two.\n" +
		"### E-CCC-001 Three **[parse]**\nFix: not ours.\n"
	cat := map[string]Entry{"E-AAA-001": {Fix: "do one."}, "E-BBB-001": {Fix: "do two."}}
	if d := catalogDrift(doc, cat); len(d) != 0 {
		t.Fatalf("an agreeing fixture drifted: %v", d)
	}
	cases := map[string]struct {
		doc  string
		cat  map[string]Entry
		want string
	}{
		"a code only errors.md has":  {doc + "### E-DDD-001 Four **[static]**\nFix: four.\n", cat, "E-DDD-001 is [static] in errors.md but not in catalog.go"},
		"a code only catalog.go has": {doc, map[string]Entry{"E-AAA-001": {Fix: "do one."}, "E-BBB-001": {Fix: "do two."}, "E-ZZZ-001": {}}, "E-ZZZ-001 is in catalog.go but not [static] in errors.md"},
		"a fix that differs":         {strings.Replace(doc, "do two.", "do 2.", 1), cat, `E-BBB-001: errors.md's fix "do 2.", catalog.go's "do two."`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if d := catalogDrift(c.doc, c.cat); len(d) != 1 || d[0] != c.want {
				t.Fatalf("got %v, want [%s]", d, c.want)
			}
		})
	}
}
