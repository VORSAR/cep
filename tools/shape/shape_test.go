package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
)

var update = flag.Bool("update", false, "rewrite testdata/corpus.golden from the engine side")

// corpusFiles is the differential's corpus (tools/grammar-diff.md): docs/language/examples/**,
// docs/language/eval/generated/** and docs/language/invalid/*, sorted.
func corpusFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..", "docs", "language")
	var files []string
	for _, dir := range []string{"examples", filepath.Join("eval", "generated")} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".cep") {
				files = append(files, p)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	invalid, err := filepath.Glob(filepath.Join(root, "invalid", "*.cep"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, invalid...)
	slices.Sort(files)
	return files
}

// TestShapeCorpusGolden pins every corpus file's report (design s1-1c-5 §7.5, DQ-S11): a characterisation of the
// engine side before its refactor. The differential row compares the same shapes against lark's independent
// rendering (gate/differential/shapes.py), so the golden is the agreed shape, not the engine's alone.
func TestShapeCorpusGolden(t *testing.T) {
	t.Parallel()
	var got bytes.Buffer
	enc := json.NewEncoder(&got)
	files := corpusFiles(t)
	if len(files) < 70 {
		t.Fatalf("found %d corpus files, want the whole corpus (74 at S1.1c-5)", len(files))
	}
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := enc.Encode(judge(filepath.ToSlash(p), string(src))); err != nil {
			t.Fatal(err)
		}
	}
	golden := filepath.Join("testdata", "corpus.golden")
	if *update {
		if err := os.WriteFile(golden, got.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		gl, wl := strings.Split(got.String(), "\n"), strings.Split(string(want), "\n")
		for i := range min(len(gl), len(wl)) {
			if gl[i] != wl[i] {
				t.Fatalf("line %d differs:\n got %s\nwant %s", i+1, gl[i], wl[i])
			}
		}
		t.Fatalf("got %d lines, want %d", len(gl), len(wl))
	}
}

func attr(q string) model.Expr { return model.Expr{Kind: model.ExprAttr, Name: "a", Qualifier: q} }

func lit() model.Expr {
	return model.Expr{Kind: model.ExprLiteral, Lit: model.Literal{Kind: model.LitInt}}
}

// TestShapeEachConstructKind has one row per arm of shape's switch (and of each if inside an arm), expected values
// from tools/grammar-diff.md's mapping.
func TestShapeEachConstructKind(t *testing.T) {
	t.Parallel()
	a, b := attr(""), lit()
	for _, tc := range []struct {
		name string
		e    model.Expr
		want string
	}{
		{"param", model.Expr{Kind: model.ExprLiteral, Lit: model.Literal{Kind: model.LitParam}}, "param"},
		{"literal", b, "literal"},
		{"name", a, "name"},
		{"qualified", attr("e1"), "qualified"},
		{"raw", model.Expr{Kind: model.ExprRaw, Qualifier: "e1"}, "(raw qualified)"},
		{"call", model.Expr{Kind: model.ExprCall, Name: "f", Args: []model.Expr{a, b}}, "(call f name literal)"},
		{"call namespaced", model.Expr{Kind: model.ExprCall, Namespace: "ns", Name: "f"}, "(call ns:f)"},
		{"neg", model.Expr{Kind: model.ExprNeg, Args: []model.Expr{a}}, "(neg name)"},
		{"not", model.Expr{Kind: model.ExprNot, Args: []model.Expr{a}}, "(not name)"},
		{"and", model.Expr{Kind: model.ExprAnd, Args: []model.Expr{a, b}}, "(and name literal)"},
		{"or", model.Expr{Kind: model.ExprOr, Args: []model.Expr{b, a}}, "(or literal name)"},
		{"compare", model.Expr{Kind: model.ExprCompare, Op: "<=", Args: []model.Expr{a, b}}, "(cmp <= name literal)"},
		{"arith", model.Expr{Kind: model.ExprArith, Op: "%", Args: []model.Expr{a, b}}, "(arith % name literal)"},
		{"is null", model.Expr{Kind: model.ExprIsNull, Args: []model.Expr{a}}, "(is-null name)"},
		{"is not null", model.Expr{Kind: model.ExprIsNull, Negated: true, Args: []model.Expr{a}}, "(is-not-null name)"},
		{"in", model.Expr{Kind: model.ExprIn, Args: []model.Expr{a}}, "(in name)"},
		{"not in", model.Expr{Kind: model.ExprIn, Negated: true, Args: []model.Expr{a}}, "(not-in name)"},
		{"text param", model.Expr{Kind: model.ExprText, Op: "contains", Args: []model.Expr{a},
			Operand: model.TextOperand{Param: "p"}}, "(text contains name param)"},
		{"text table", model.Expr{Kind: model.ExprText, Op: "in", Args: []model.Expr{a},
			Operand: model.TextOperand{Table: "T"}}, "(text in name name)"},
		{"text strings", model.Expr{Kind: model.ExprText, Op: "in", Args: []model.Expr{a},
			Operand: model.TextOperand{Strings: []string{"x", "y"}}}, "(text in name (strings 2))"},
		{"unknown kind", model.Expr{Kind: "mystery"}, "?mystery"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shape(tc.e); got != tc.want {
				t.Fatalf("shape = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestShapesEachBodyKind has one row per arm of shapes' switch, with keys of both forms and an unfiltered source.
func TestShapesEachBodyKind(t *testing.T) {
	t.Parallel()
	f := func(e model.Expr) model.Source { return model.Source{Stream: "S", Filter: e} }
	keys := model.Partition{Keys: []model.PartitionKey{
		{Stream: "S", Attrs: []model.KeyAttr{{Name: "a"}}},
		{Stream: "T", Grouped: true, Attrs: []model.KeyAttr{{Name: "a"}, {Name: "b"}}},
	}}
	sel := []model.SelectItem{{Expr: lit()}}
	for _, tc := range []struct {
		name string
		rule model.Rule
		want []string
	}{
		{"pattern", model.Rule{Partition: keys, Select: sel, Body: model.Body{Kind: model.BodyPattern,
			Pattern: model.Pattern{Elements: []model.PatternElement{
				{Steps: []model.Step{{Source: f(attr(""))}, {Source: model.Source{Stream: "S"}}}},
				{Kind: model.ElemAbsence, Absence: f(attr("e1"))},
			}}}},
			[]string{"(key S a)", "(group T a b)", "name", "qualified", "literal"}},
		{"window", model.Rule{Select: sel, Body: model.Body{Kind: model.BodyWindow,
			Window: model.Window{Source: f(attr(""))}}}, []string{"name", "literal"}},
		{"plain", model.Rule{Select: sel, Body: model.Body{Source: f(lit())}}, []string{"literal", "literal"}},
		{"plain unfiltered", model.Rule{Select: sel, Body: model.Body{Source: model.Source{Stream: "S"}}},
			[]string{"literal"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shapes(&model.App{Rules: []model.Rule{tc.rule}}); !slices.Equal(got, tc.want) {
				t.Fatalf("shapes = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunExitCodes(t *testing.T) {
	t.Parallel()
	good := filepath.Join("..", "..", "docs", "language", "examples", "torture", "precedence.cep")
	files := corpusFiles(t)
	if !slices.Contains(files, good) {
		good = files[0]
	}
	var out, errs bytes.Buffer
	if code := run([]string{good}, &out, &errs); code != 0 || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("run(%s) = %d with %q, want 0 and one line", good, code, out.String())
	}
	out.Reset()
	missing := filepath.Join(t.TempDir(), "missing.cep")
	if code := run([]string{good, missing}, &out, &errs); code != 2 || !strings.Contains(errs.String(), "missing.cep") {
		t.Fatalf("run with a missing file = %d, stderr %q; want 2 naming it", code, errs.String())
	}
}

// TestJudgeReportsEachLoweringCodeAtTheLoweringStage: a grammar-valid file the lowering refuses is a reject at the
// "lowering" stage with its code, one row per parser.LoweringCodes code (inputs as the parser's TestLoweringCodes).
func TestJudgeReportsEachLoweringCodeAtTheLoweringStage(t *testing.T) {
	t.Parallel()
	const head = "language cep/0.1;\n" +
		"stream S (k string, n int, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
		"stream Out (k string, ts timestamp);\n" +
		"rule R intent 'x' partition by k of S ttl 1 hour from S"
	rows := []struct{ code, src string }{
		{"E-DUR-002", head + " select k output first every 1.5 min insert into Out;\n"},
		{"E-STR-002", head + "[k has_cs 'C:\\q'] select k insert into Out;\n"},
		{"E-TYPE-001", head + "[n > 99999999999999999999] select k insert into Out;\n"},
		{"E-WIN-004", head + " window sort(0, n, desc) emit on change late events drop select k insert into Out;\n"},
	}
	if !strings.Contains(rows[1].src, "'C:\\q'") {
		t.Fatal("test setup: the E-STR-002 input must hold a backslash before q")
	}
	var codes []string
	for _, row := range rows {
		codes = append(codes, row.code)
		r := judge("lowering.cep", row.src)
		if r.Verdict != "reject" || r.Code != row.code || r.Stage != "lowering" {
			t.Errorf("judge(the %s input): verdict %q, code %q, stage %q; want reject, %s, lowering", row.code, r.Verdict,
				r.Code, r.Stage, row.code)
		}
	}
	if !slices.Equal(codes, parser.LoweringCodes) {
		t.Errorf("the rows cover %v; want one per parser.LoweringCodes %v, in its order", codes, parser.LoweringCodes)
	}
}
