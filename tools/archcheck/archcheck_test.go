package main

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func testRules(t *testing.T) *Rules {
	t.Helper()
	r, err := LoadRules("../importcheck/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// check runs every rule over one file, given as source; other maps more files of the same package.
func check(t *testing.T, path, src string, other ...string) []string {
	t.Helper()
	files := map[string]string{path: src}
	for i := 0; i+1 < len(other); i += 2 {
		files[other[i]] = other[i+1]
	}
	return checkWith(t, testLists(t), path, files)
}

// testLists is the repository's own tools/allowlists/ (design §3: tests pass filepath.Join("..", "..")).
func testLists(t *testing.T) *Lists {
	t.Helper()
	l, err := LoadLists(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// adrNamed are ADR-0015 §2.2's two named purity exceptions (S3.1-2 design note §5) as purity.json targets.
var adrNamed = []string{"internal/event: sync.Pool", "internal/table: sync.Mutex", "internal/table: sync/atomic.Pointer"}

// checkNamed is check over a fixture list holding adrNamed, not the repository's own: the named-exception tests read
// the mechanism, and the real list may carry only the exceptions some code uses (BK-S3: an unused target is red).
func checkNamed(t *testing.T, path, src string) []string {
	t.Helper()
	return checkWith(t, loadAt(t, listRoot(t, adrNamed, nil, nil)), path, map[string]string{path: src})
}

func checkWith(t *testing.T, lists *Lists, path string, files map[string]string) []string {
	t.Helper()
	got, err := CheckSources(testRules(t), lists, path, files)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func wantOne(t *testing.T, got []string, needle string) {
	t.Helper()
	if len(got) != 1 || !strings.Contains(got[0], needle) {
		t.Fatalf("want one finding containing %q, got %q", needle, got)
	}
}

func wantNone(t *testing.T, got []string) {
	t.Helper()
	if len(got) != 0 {
		t.Fatalf("want no finding, got %q", got)
	}
}

const mod = "github.com/vorsar/cep/"

func TestImports(t *testing.T) {
	t.Run("an upward import from event is red", func(t *testing.T) {
		got := check(t, "internal/event/x.go", "package event\nimport _ \""+mod+"internal/model\"\n")
		wantOne(t, got, "imports: internal/event may not import internal/model")
	})
	t.Run("a listed import is fine", func(t *testing.T) {
		wantNone(t, check(t, "internal/model/x.go", "package model\nimport _ \""+mod+"internal/event\"\n"))
	})
	t.Run("operators never import each other beyond the exception table", func(t *testing.T) {
		got := check(t, "internal/pattern/x.go", "package pattern\nimport _ \""+mod+"internal/window\"\n")
		wantOne(t, got, "may not import internal/window")
		wantNone(t, check(t, "internal/expr/x.go", "package expr\nimport _ \""+mod+"internal/text\"\n"))
	})
	t.Run("the engine never imports its own facade", func(t *testing.T) {
		wantOne(t, check(t, "internal/runtime/x.go", "package runtime\nimport _ \""+mod+"\"\n"), "may not import cep")
	})
	t.Run("connectors import cep and io only", func(t *testing.T) {
		wantNone(t, check(t, "io/http/x.go", "package http\nimport (\n_ \""+mod+"\"\n_ \""+mod+"io\"\n)\n"))
		wantOne(t, check(t, "io/http/x.go", "package http\nimport _ \""+mod+"internal/runtime\"\n"), "io/http may not import")
	})
	t.Run("nothing imports cmd", func(t *testing.T) {
		wantOne(t, check(t, "cmd/cep/x.go", "package main\nimport _ \""+mod+"cmd/cepd\"\n"), "may not import cmd/cepd")
	})
	t.Run("a package not in the rules is red", func(t *testing.T) {
		wantOne(t, check(t, "internal/newthing/x.go", "package newthing\n"), "internal/newthing is not in")
	})
	t.Run("third-party imports only where listed", func(t *testing.T) {
		wantOne(t, check(t, "internal/expr/x.go", "package expr\nimport _ \"golang.org/x/text/unicode/norm\"\n"), "third-party")
		wantNone(t, check(t, "internal/text/x.go", "package text\nimport _ \"golang.org/x/text/unicode/norm\"\n"))
	})
	t.Run("expr may use only the model's expression types", func(t *testing.T) {
		// Owner, 2026-11-04 (S2.2a): the edge expr -> model is allowed for the IR's expression node only.
		wantNone(t, check(t, "internal/expr/x.go", "package expr\nimport m \""+mod+"internal/model\"\nfunc f(e m.Expr, l m.Literal, k m.ExprKind, o m.TextOperand) { _ = m.ExprAnd; _ = m.LitInt }\n"))
		wantOne(t, check(t, "internal/expr/x.go", "package expr\nimport m \""+mod+"internal/model\"\nfunc f(a m.App) {}\n"), "fence: internal/expr may use model.Expr")
		wantOne(t, check(t, "internal/expr/x.go", "package expr\nimport \""+mod+"internal/model\"\nvar _ = model.Param{}\n"), "fence: internal/expr may use model.Expr")
	})
	t.Run("validate's tests may import the parser, its production code may not", func(t *testing.T) {
		// Owner, 2026-11-04 (S2.2b, A4): test_imports, test files only, this one edge only.
		parser := "import _ \"" + mod + "internal/parser\"\n"
		wantNone(t, check(t, "internal/validate/x_test.go", "package validate_test\n"+parser))
		wantOne(t, check(t, "internal/validate/x.go", "package validate\n"+parser), "internal/validate may not import internal/parser")
		wantOne(t, check(t, "internal/expr/x_test.go", "package expr_test\n"+parser), "internal/expr may not import internal/parser")
		runtime := "import _ \"" + mod + "internal/runtime\"\n"
		wantOne(t, check(t, "internal/validate/x_test.go", "package validate_test\n"+runtime), "may not import internal/runtime")
	})
	t.Run("plan's and playback's tests may read .cep programs, their production code may not", func(t *testing.T) {
		// Owner, 2026-11-06 (S3.2b-1): test_imports, test files only — plan's tests through the parser and the
		// validator, playback's through the parser (it may import the validator already).
		parser := "import _ \"" + mod + "internal/parser\"\n"
		validate := "import _ \"" + mod + "internal/validate\"\n"
		wantNone(t, check(t, "internal/plan/x_test.go", "package plan_test\n"+parser))
		wantNone(t, check(t, "internal/plan/x_test.go", "package plan_test\n"+validate))
		wantNone(t, check(t, "internal/playback/x_test.go", "package playback_test\n"+parser))
		wantOne(t, check(t, "internal/plan/x.go", "package plan\n"+parser), "internal/plan may not import internal/parser")
		wantOne(t, check(t, "internal/plan/x.go", "package plan\n"+validate), "internal/plan may not import internal/validate")
		wantOne(t, check(t, "internal/playback/x.go", "package playback\n"+parser), "internal/playback may not import internal/parser")
		runtime := "import _ \"" + mod + "internal/runtime\"\n"
		wantOne(t, check(t, "internal/plan/x_test.go", "package plan_test\n"+runtime), "may not import internal/runtime")
	})
	t.Run("runtime's and playback's tests may import goleak, their production code may not", func(t *testing.T) {
		// Owner, 2026-11-06 (S3.2b's plan check Q6): goleak is test-only — test_external, test files only.
		goleak := "import _ \"go.uber.org/goleak\"\n"
		wantNone(t, check(t, "internal/runtime/x_test.go", "package runtime_test\n"+goleak))
		wantNone(t, check(t, "internal/playback/x_test.go", "package playback_test\n"+goleak))
		wantOne(t, check(t, "internal/runtime/x.go", "package runtime\n"+goleak), "third-party")
		wantOne(t, check(t, "internal/playback/x.go", "package playback\n"+goleak), "third-party")
		wantOne(t, check(t, "internal/plan/x_test.go", "package plan_test\n"+goleak), "third-party")
	})
	t.Run("expr's tests may also build tables, its production code may not", func(t *testing.T) {
		// Owner, 2026-11-04 (S3.1-2, ≠ R): test_fences, test files only, these identifiers only.
		imp := "import \"" + mod + "internal/table\"\n"
		build := "func f() (*table.Table, error) { return table.New(\"T\", nil, nil) }\n"
		wantNone(t, check(t, "internal/expr/x_test.go", "package expr_test\n"+imp+build))
		wantOne(t, check(t, "internal/expr/x.go", "package expr\n"+imp+"var _ = table.New\nvar _ *table.Version\n"), "fence: internal/expr may use table.Version")
		wantOne(t, check(t, "internal/expr/x_test.go", "package expr_test\n"+imp+"var _ = table.Walk\n"), "fence: internal/expr may use table.Version")
		model := "import \"" + mod + "internal/model\"\nvar _ = model.Param{}\n"
		wantOne(t, check(t, "internal/expr/x_test.go", "package expr_test\n"+model), "fence: internal/expr may use model.Expr")
	})
	t.Run("an external test package may import its own package", func(t *testing.T) {
		wantNone(t, check(t, "internal/event/x_test.go", "package event_test\nimport _ \""+mod+"internal/event\"\n"))
	})
}

func TestClock(t *testing.T) {
	t.Run("a wall-clock call outside the allowed packages is red", func(t *testing.T) {
		got := check(t, "internal/clock/x.go", "package clock\nimport \"time\"\nvar _ = time.Now\n")
		wantOne(t, got, "clock: time.Now")
	})
	t.Run("an aliased import is still caught", func(t *testing.T) {
		got := check(t, "internal/model/x.go", "package model\nimport tm \"time\"\nfunc f() { tm.Sleep(1) }\n")
		wantOne(t, got, "clock: time.Sleep")
	})
	t.Run("runtime may use the wall clock", func(t *testing.T) {
		wantNone(t, check(t, "internal/runtime/x.go", "package runtime\nimport \"time\"\nfunc f() { _ = time.Now() }\n"))
	})
	t.Run("an operator package may not import time, except window/cron.go", func(t *testing.T) {
		wantOne(t, check(t, "internal/table/x.go", "package table\nimport \"time\"\nfunc f(d time.Duration) time.Duration { return d }\n"), "clock: internal/table may not import time")
		// cron.go's exception now comes from tools/allowlists/clock.json: see TestListsClockMembership (S1.1c-2).
	})
	t.Run("time types and arithmetic are fine elsewhere", func(t *testing.T) {
		wantNone(t, check(t, "internal/clock/x.go", "package clock\nimport \"time\"\nfunc f(d time.Duration) time.Duration { return 2 * d }\n"))
	})
}

func TestPurity(t *testing.T) {
	t.Run("a go statement outside runtime, io/* and cmd is red", func(t *testing.T) {
		wantOne(t, check(t, "internal/plan/x.go", "package plan\nfunc f() { go f() }\n"), "purity: go statement")
		wantNone(t, check(t, "internal/runtime/x.go", "package runtime\nfunc f() { go f() }\n"))
	})
	t.Run("channel operations are red outside them", func(t *testing.T) {
		got := check(t, "internal/expr/x.go", "package expr\nfunc f(c chan int) { c <- 1 }\n")
		if len(got) != 2 {
			t.Fatalf("want the chan type and the send, got %q", got)
		}
	})
	t.Run("sync only where named", func(t *testing.T) {
		wantOne(t, check(t, "internal/model/x.go", "package model\nimport _ \"sync\"\n"), "purity: internal/model may not import sync")
		wantNone(t, checkNamed(t, "internal/event/x.go", "package event\nimport _ \"sync\"\n"))
		wantNone(t, checkNamed(t, "internal/table/x.go", "package table\nimport _ \"sync/atomic\"\n"))
	})
	t.Run("os, net and io only at the boundary", func(t *testing.T) {
		wantOne(t, check(t, "internal/parser/x.go", "package parser\nimport _ \"os\"\n"), "purity: internal/parser may not import os")
		wantOne(t, check(t, "internal/model/x.go", "package model\nimport _ \"net/http\"\n"), "may not import net/http")
		wantNone(t, check(t, "internal/playback/x.go", "package playback\nimport _ \"os\"\n"))
	})
	t.Run("unsafe and go:linkname are red everywhere", func(t *testing.T) {
		wantOne(t, check(t, "internal/runtime/x.go", "package runtime\nimport _ \"unsafe\"\n"), "purity: unsafe")
		wantOne(t, check(t, "cmd/cep/x.go", "package main\n//go:linkname f runtime.f\nfunc f()\n"), "purity: //go:linkname")
	})
	t.Run("tests may use os", func(t *testing.T) {
		wantNone(t, check(t, "internal/model/x_test.go", "package model\nimport _ \"os\"\n"))
	})
}

func TestGlobals(t *testing.T) {
	t.Run("an error sentinel is fine", func(t *testing.T) {
		wantNone(t, check(t, "internal/model/x.go", "package model\nimport \"errors\"\nvar errX = errors.New(\"x\")\n"))
	})
	t.Run("a mutable package-level var is red", func(t *testing.T) {
		wantOne(t, check(t, "internal/model/x.go", "package model\nvar count int\n"), "globals: count")
	})
	t.Run("a lookup table never written is fine, and one written anywhere in the package is red", func(t *testing.T) {
		table := "package model\nvar names = map[int]string{1: \"a\"}\n"
		wantNone(t, check(t, "internal/model/x.go", table))
		got := check(t, "internal/model/x.go", table, "internal/model/y.go", "package model\nfunc f() { names[2] = \"b\" }\n")
		wantOne(t, got, "globals: names")
	})
	t.Run("a compiled regexp and a blank assertion are fine", func(t *testing.T) {
		wantNone(t, check(t, "internal/model/x.go", "package model\nimport \"regexp\"\nvar re = regexp.MustCompile(\"a\")\nvar _ error = nil\n"))
	})
	t.Run("a Must-constructed value is fine, a plainly built one is not", func(t *testing.T) {
		wantNone(t, check(t, "internal/parser/x.go", "package parser\nimport \"p\"\nvar g = p.MustBuild[int](1)\nvar r = p.MustSimple(2)\n"))
		wantOne(t, check(t, "internal/parser/x.go", "package parser\nimport \"p\"\nvar g = p.Build(1)\n"), "globals: g")
	})
	t.Run("tests may hold package-level state", func(t *testing.T) {
		wantNone(t, check(t, "internal/model/x_test.go", "package model\nvar seen int\n"))
	})
}

// wantOneAll: exactly one finding, containing every needle.
func wantOneAll(t *testing.T, got []string, needles ...string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("want one finding containing %q, got %q", needles, got)
	}
	for _, n := range needles {
		if !strings.Contains(got[0], n) {
			t.Fatalf("want one finding containing %q, got %q", needles, got)
		}
	}
}

// TestPurityNamedExceptions: S3.1-2 design note §5 (ADR-0015 §2): internal/table may use only sync.Mutex and
// atomic.Pointer, internal/event only sync.Pool; any other identifier of sync or sync/atomic there, sync/atomic in
// internal/event, and a dot-import of either in either package are purity findings naming the allowed identifiers
// and ADR-0015 §2. Every other package stays as before.
func TestPurityNamedExceptions(t *testing.T) {
	const adr = "ADR-0015 §2"
	t.Run("table may use sync.Mutex and atomic.Pointer", func(t *testing.T) {
		src := "package table\nimport (\n\"sync\"\n\"sync/atomic\"\n)\ntype V struct{}\ntype T struct {\nmu sync.Mutex\ncur atomic.Pointer[V]\n}\nfunc (t *T) f() *V { t.mu.Lock(); defer t.mu.Unlock(); return t.cur.Load() }\n"
		wantNone(t, checkNamed(t, "internal/table/x.go", src))
	})
	t.Run("table: each other sync or sync/atomic identifier is red", func(t *testing.T) {
		for ident, src := range map[string]string{
			"RWMutex":   "package table\nimport \"sync\"\ntype T struct{ mu sync.RWMutex }\n",
			"Once":      "package table\nimport \"sync\"\nfunc f() { var o sync.Once; o.Do(f) }\n",
			"WaitGroup": "package table\nimport \"sync\"\nfunc f() { var wg sync.WaitGroup; wg.Wait() }\n",
			"Pool":      "package table\nimport \"sync\"\ntype T struct{ p sync.Pool }\n",
			"Int64":     "package table\nimport \"sync/atomic\"\ntype T struct{ n atomic.Int64 }\n",
			"Value":     "package table\nimport \"sync/atomic\"\ntype T struct{ v atomic.Value }\n",
			"AddInt64":  "package table\nimport \"sync/atomic\"\nfunc f(p *int64) { atomic.AddInt64(p, 1) }\n",
		} {
			t.Run(ident, func(t *testing.T) {
				wantOneAll(t, checkNamed(t, "internal/table/x.go", src), "purity:", ident, "Mutex", "Pointer", adr)
			})
		}
	})
	t.Run("table: an allowed and a banned identifier in one file give one finding", func(t *testing.T) {
		src := "package table\nimport \"sync\"\ntype T struct {\nmu sync.Mutex\nrw sync.RWMutex\n}\n"
		wantOneAll(t, checkNamed(t, "internal/table/x.go", src), "purity:", "RWMutex")
	})
	t.Run("table: an aliased import is still read", func(t *testing.T) {
		wantOneAll(t, checkNamed(t, "internal/table/x.go", "package table\nimport s \"sync\"\ntype T struct{ mu s.RWMutex }\n"), "purity:", "RWMutex")
		wantNone(t, checkNamed(t, "internal/table/x.go", "package table\nimport s \"sync\"\ntype T struct{ mu s.Mutex }\n"))
	})
	t.Run("event may use sync.Pool", func(t *testing.T) {
		src := "package event\nimport \"sync\"\nvar pool = sync.Pool{New: func() any { return new([]byte) }}\nfunc get() any { return pool.Get() }\n"
		wantNone(t, checkNamed(t, "internal/event/x.go", src))
	})
	t.Run("event: any other sync identifier is red", func(t *testing.T) {
		for ident, src := range map[string]string{
			"Mutex":     "package event\nimport \"sync\"\ntype T struct{ mu sync.Mutex }\n",
			"Once":      "package event\nimport \"sync\"\nfunc f() { var o sync.Once; o.Do(f) }\n",
			"WaitGroup": "package event\nimport \"sync\"\nfunc f() { var wg sync.WaitGroup; wg.Wait() }\n",
		} {
			t.Run(ident, func(t *testing.T) {
				wantOneAll(t, checkNamed(t, "internal/event/x.go", src), "purity:", ident, "Pool", adr)
			})
		}
	})
	t.Run("event may not import sync/atomic", func(t *testing.T) {
		wantOneAll(t, checkNamed(t, "internal/event/x.go", "package event\nimport _ \"sync/atomic\"\n"), "purity:", "sync/atomic", adr)
	})
	t.Run("a dot-import of sync or sync/atomic is red in table and event", func(t *testing.T) {
		wantOneAll(t, checkNamed(t, "internal/table/x.go", "package table\nimport . \"sync\"\ntype T struct{ mu Mutex }\n"), "purity:", adr)
		wantOneAll(t, checkNamed(t, "internal/table/x.go", "package table\nimport . \"sync/atomic\"\ntype V struct{}\ntype T struct{ p Pointer[V] }\n"), "purity:", adr)
		wantOneAll(t, checkNamed(t, "internal/event/x.go", "package event\nimport . \"sync\"\nvar pool = Pool{}\n"), "purity:", adr)
		// sync/atomic is banned in event twice over (the import and the dot); one or two findings, all purity.
		got := checkNamed(t, "internal/event/x.go", "package event\nimport . \"sync/atomic\"\ntype T struct{ n Int64 }\n")
		if len(got) == 0 || len(got) > 2 {
			t.Fatalf("want one or two purity findings, got %q", got)
		}
		for _, f := range got {
			if !strings.Contains(f, "purity:") {
				t.Fatalf("want purity findings only, got %q", got)
			}
		}
	})
	t.Run("any other package importing sync stays red", func(t *testing.T) {
		wantOne(t, checkNamed(t, "internal/pattern/x.go", "package pattern\nimport \"sync\"\ntype T struct{ mu sync.Mutex }\n"), "purity: internal/pattern may not import sync")
		wantOne(t, checkNamed(t, "internal/text/x.go", "package text\nimport \"sync/atomic\"\ntype V struct{}\ntype T struct{ p atomic.Pointer[V] }\n"), "purity: internal/text may not import sync/atomic")
		wantOne(t, checkNamed(t, "internal/expr/x.go", "package expr\nimport \"sync\"\nvar _ = sync.Pool{}\n"), "purity: internal/expr may not import sync")
	})
	t.Run("tests in table and event are not held to it", func(t *testing.T) {
		wantNone(t, checkNamed(t, "internal/table/x_test.go", "package table_test\nimport \"sync\"\nfunc f() { var wg sync.WaitGroup; wg.Wait() }\n"))
		wantNone(t, checkNamed(t, "internal/event/x_test.go", "package event_test\nimport \"sync/atomic\"\nfunc f() { var n atomic.Int64; n.Add(1) }\n"))
	})
}

// TestExprTableFence: S3.1-2 design note §2 (review F21): internal/expr reads tables through table.Version only and
// never builds or writes one.
func TestExprTableFence(t *testing.T) {
	imp := "import \"" + mod + "internal/table\"\n"
	t.Run("table.Version is allowed", func(t *testing.T) {
		wantNone(t, check(t, "internal/expr/x.go", "package expr\n"+imp+"func f(v *table.Version) int { return v.Len() }\n"))
	})
	t.Run("table.New is red", func(t *testing.T) {
		wantOne(t, check(t, "internal/expr/x.go", "package expr\n"+imp+"func f() { _, _ = table.New(\"T\", nil, nil) }\n"), "fence: internal/expr may use table.Version")
	})
	t.Run("table.Table is red", func(t *testing.T) {
		wantOne(t, check(t, "internal/expr/x.go", "package expr\n"+imp+"func f(t *table.Table) error { return t.Upsert(nil) }\n"), "fence: internal/expr may use table.Version")
	})
}

// ---- S1.1c-2 lane C (design §3): archcheck reads tools/allowlists/; the -tests fence; Stale.

const (
	cronSrc = "package window\nimport \"time\"\nfunc f() (*time.Location, error) { return time.LoadLocation(\"UTC\") }\n"
	fence   = ": wait for a state, cap it with a context, yield on every pass of a poll (testing.md rule 1)"
)

// listRoot writes a fixture root holding tools/allowlists/{purity,clock,testhygiene}.json, each a list of targets
// in the grammar's shape (the extra fields are read by the Python row, not by archcheck — A-L2), and an empty
// generated.json (the fix round's BK-S6 list; a test that lists a generated file rewrites it with writeList).
func listRoot(t *testing.T, purity, clock, tests []string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "tools", "allowlists")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, targets := range map[string][]string{"purity": purity, "clock": clock, "testhygiene": tests, "generated": nil} {
		writeList(t, root, name, name, targets)
	}
	return root
}

func writeList(t *testing.T, root, name, rule string, targets []string) {
	t.Helper()
	entries := make([]string, len(targets))
	for i, target := range targets {
		entries[i] = `{"target": "` + target + `", "kind": "PERMANENT", "reason": "a reason long enough", "ref": "docs/a.md#b", "sprint": "S1.1c-2"}`
	}
	text := `{"schema": 1, "note": "n", "rule": "` + rule + `", "entries": [` + strings.Join(entries, ", ") + "]}\n"
	if err := os.WriteFile(filepath.Join(root, "tools", "allowlists", name+".json"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadAt(t *testing.T, root string) *Lists {
	t.Helper()
	l, err := LoadLists(root)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func touch(t *testing.T, root, rel string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListsPurity(t *testing.T) {
	t.Run("the targets are read into the purity map", func(t *testing.T) {
		l := loadAt(t, listRoot(t, []string{"internal/event: sync.Pool", "internal/table: sync.Mutex", "internal/table: sync/atomic.Pointer"}, nil, nil))
		want := map[string]map[string][]string{
			"internal/event": {"sync": {"Pool"}},
			"internal/table": {"sync": {"Mutex"}, "sync/atomic": {"Pointer"}},
		}
		if len(l.Purity) != len(want) {
			t.Fatalf("purity %v, want %v", l.Purity, want)
		}
		for pkg, imports := range want {
			for imp, idents := range imports {
				if !slices.Equal(l.Purity[pkg][imp], idents) {
					t.Fatalf("purity[%s][%s] = %v, want %v", pkg, imp, l.Purity[pkg][imp], idents)
				}
			}
		}
	})
	t.Run("a list without internal/table: sync.Mutex makes table's mutex red", func(t *testing.T) {
		// The mutex is a field: a package-level `var mu sync.Mutex` is a globals finding whatever the lists hold
		// (ADR-0015 §2.2), which would hide what this case tests.
		src := "package table\nimport \"sync\"\ntype T struct{ mu sync.Mutex }\nfunc (t *T) f() { t.mu.Lock() }\n"
		with := loadAt(t, listRoot(t, []string{"internal/table: sync.Mutex"}, nil, nil))
		wantNone(t, checkWith(t, with, "internal/table/x.go", map[string]string{"internal/table/x.go": src}))
		without := loadAt(t, listRoot(t, []string{"internal/event: sync.Pool"}, nil, nil))
		wantOne(t, checkWith(t, without, "internal/table/x.go", map[string]string{"internal/table/x.go": src}), "purity: internal/table may not import sync")
	})
	t.Run("a purity target that does not split is an error", func(t *testing.T) {
		for _, bad := range []string{"internal/table sync.Mutex", "internal/table: syncMutex", "internal/table: sync.", ": sync.Mutex"} {
			root := listRoot(t, []string{bad}, nil, nil)
			if _, err := LoadLists(root); err == nil {
				t.Fatalf("target %q: want an error, got none", bad)
			}
		}
	})
	t.Run("a list whose rule does not match its file is an error", func(t *testing.T) {
		root := listRoot(t, nil, nil, nil)
		writeList(t, root, "clock", "purity", nil)
		if _, err := LoadLists(root); err == nil {
			t.Fatal("clock.json with rule purity: want an error, got none")
		}
	})
	t.Run("the repository's own lists load", func(t *testing.T) {
		l := testLists(t)
		if !slices.Equal(l.Purity["internal/table"]["sync"], []string{"Mutex"}) {
			t.Fatalf("purity[internal/table][sync] = %v, want [Mutex]", l.Purity["internal/table"]["sync"])
		}
	})
}

func TestListsClockMembership(t *testing.T) {
	files := map[string]string{"internal/window/cron.go": cronSrc}
	t.Run("cron.go importing time is red with an empty clock.json", func(t *testing.T) {
		got := checkWith(t, loadAt(t, listRoot(t, nil, nil, nil)), "internal/window/cron.go", files)
		wantOne(t, got, "clock: internal/window may not import time")
	})
	t.Run("and green with the entry internal/window/cron.go: time", func(t *testing.T) {
		got := checkWith(t, loadAt(t, listRoot(t, nil, []string{"internal/window/cron.go: time"}, nil)), "internal/window/cron.go", files)
		wantNone(t, got)
	})
	t.Run("the entry covers its file only", func(t *testing.T) {
		l := loadAt(t, listRoot(t, nil, []string{"internal/window/cron.go: time"}, nil))
		got := checkWith(t, l, "internal/window/other.go", map[string]string{"internal/window/other.go": strings.Replace(cronSrc, "cron", "other", 1)})
		wantOne(t, got, "clock: internal/window may not import time")
	})
}

func TestStale(t *testing.T) {
	t.Run("purity, clock and testhygiene targets that do not exist", func(t *testing.T) {
		root := listRoot(t,
			[]string{"internal/event: sync.Pool", "internal/gone: sync.Mutex"},
			[]string{"internal/window/cron.go: time"},
			[]string{"io/inmemory/x_test.go", "io/gone_test.go"})
		touch(t, root, "internal/event/x.go")
		touch(t, root, "io/inmemory/x_test.go")
		got := Stale(loadAt(t, root), root)
		want := []string{
			"tools/allowlists/clock.json: internal/window/cron.go: time: does not exist (a stale target)",
			"tools/allowlists/purity.json: internal/gone: sync.Mutex: does not exist (a stale target)",
			"tools/allowlists/testhygiene.json: io/gone_test.go: does not exist (a stale target)",
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("Stale = %q, want %q", got, want)
		}
	})
	t.Run("the repository's own lists have no stale target", func(t *testing.T) {
		if got := Stale(testLists(t), filepath.Join("..", "..")); len(got) != 0 {
			t.Fatalf("Stale = %q, want none", got)
		}
	})
}

func TestCheckTest(t *testing.T) {
	empty := func(t *testing.T) *Lists { return loadAt(t, listRoot(t, nil, nil, nil)) }
	const path = "internal/x/x_test.go"
	for _, banned := range []string{"Sleep", "After", "AfterFunc", "Tick", "NewTimer", "NewTicker"} {
		t.Run("time."+banned+" is red", func(t *testing.T) {
			src := "package x\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\nfunc TestX(t *testing.T) {\n\t_ = time." + banned + "\n}\n"
			got := CheckTest(empty(t), path, src)
			// design ambiguity: §3 prints "failed: " before each finding; CheckTest's items are taken without it,
			// as CheckSources' are (main adds the prefix).
			want := []string{path + ":9: testhygiene: time." + banned + " in a test" + fence}
			if !slices.Equal(got, want) {
				t.Fatalf("CheckTest = %q, want %q", got, want)
			}
		})
	}
	t.Run("an aliased import is still caught", func(t *testing.T) {
		src := "package x\n\nimport tm \"time\"\n\nfunc f() { tm.Sleep(1) }\n"
		want := []string{path + ":5: testhygiene: time.Sleep in a test" + fence}
		if got := CheckTest(empty(t), path, src); !slices.Equal(got, want) {
			t.Fatalf("CheckTest = %q, want %q", got, want)
		}
	})
	t.Run("a dot-import of time is red", func(t *testing.T) {
		src := "package x\n\nimport . \"time\"\n\nfunc f() { Sleep(1) }\n"
		want := []string{path + ":3: testhygiene: a dot-import of time hides the calls the fence reads"}
		if got := CheckTest(empty(t), path, src); !slices.Equal(got, want) {
			t.Fatalf("CheckTest = %q, want %q", got, want)
		}
	})
	t.Run("time.Duration and context.WithTimeout are fine", func(t *testing.T) {
		src := "package x\n\nimport (\n\t\"context\"\n\t\"time\"\n)\n\nfunc f() {\n\tctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)\n\tdefer cancel()\n\tvar d time.Duration\n\t_, _ = ctx, d\n}\n"
		if got := CheckTest(empty(t), path, src); len(got) != 0 {
			t.Fatalf("CheckTest = %q, want none", got)
		}
	})
	t.Run("a file listed in testhygiene.json is skipped", func(t *testing.T) {
		l := loadAt(t, listRoot(t, nil, nil, []string{path}))
		src := "package x\n\nimport \"time\"\n\nfunc f() { time.Sleep(1) }\n"
		if got := CheckTest(l, path, src); len(got) != 0 {
			t.Fatalf("CheckTest on a listed file = %q, want none", got)
		}
	})
}

func TestTestsModeRefusesANonTestFile(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "archcheck")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	list := filepath.Join(t.TempDir(), "files.nul")
	if err := os.WriteFile(list, []byte("tools/archcheck/main.go\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-tests", list)
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("-tests on a non-test file: want exit 2, got %v\n%s", err, out)
	}
}

// ---- S1.1c-2 lane C's own cases: LoadLists' faults, the fence's edges, and cli (main's body) end to end in-process.

func TestLoadListsFaults(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ file, text, want string }{
		"bad JSON":                 {"purity", "{", "tools/allowlists/purity.json: "},
		"a clock target, no colon": {"clock", `{"rule": "clock", "entries": [{"target": "internal/window/cron.go"}]}`, `tools/allowlists/clock.json: target "internal/window/cron.go": want "<file>: time"`},
		"a clock target, not time": {"clock", `{"rule": "clock", "entries": [{"target": "internal/window/cron.go: sync"}]}`, `tools/allowlists/clock.json: target "internal/window/cron.go: sync": want "<file>: time"`},
		"a clock target, no file":  {"clock", `{"rule": "clock", "entries": [{"target": ": time"}]}`, `tools/allowlists/clock.json: target ": time": want "<file>: time"`},
		"an empty testhygiene one": {"testhygiene", `{"rule": "testhygiene", "entries": [{"target": ""}]}`, `tools/allowlists/testhygiene.json: an empty target: want "<file>"`},
		"a rule mismatch":          {"testhygiene", `{"rule": "clock", "entries": []}`, `tools/allowlists/testhygiene.json: rule "clock", want "testhygiene"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := listRoot(t, nil, nil, nil)
			if err := os.WriteFile(filepath.Join(root, "tools", "allowlists", c.file+".json"), []byte(c.text), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadLists(root)
			if err == nil || !strings.HasPrefix(err.Error(), c.want) {
				t.Fatalf("LoadLists: %v, want an error starting %q", err, c.want)
			}
		})
	}
	t.Run("a missing list file is an error naming it", func(t *testing.T) {
		t.Parallel()
		root := listRoot(t, nil, nil, nil)
		if err := os.Remove(filepath.Join(root, "tools", "allowlists", "clock.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLists(root); err == nil || !strings.HasPrefix(err.Error(), "tools/allowlists/clock.json: ") {
			t.Fatalf("LoadLists: %v, want an error naming tools/allowlists/clock.json", err)
		}
	})
}

func TestCheckTestEdges(t *testing.T) {
	t.Parallel()
	empty := loadAt(t, listRoot(t, nil, nil, nil))
	const path = "internal/x/x_test.go"
	t.Run("a second import name of time is read too", func(t *testing.T) {
		t.Parallel()
		src := "package x\n\nimport (\n\t\"time\"\n\tt2 \"time\"\n)\n\nvar _ = time.Second\n\nfunc f() { t2.Sleep(1) }\n"
		want := []string{path + ":10: testhygiene: time.Sleep in a test" + fence}
		if got := CheckTest(empty, path, src); !slices.Equal(got, want) {
			t.Fatalf("CheckTest = %q, want %q", got, want)
		}
	})
	t.Run("a blank import of time and a Sleep on another package are fine", func(t *testing.T) {
		t.Parallel()
		src := "package x\n\nimport (\n\t_ \"time\"\n\t\"other\"\n)\n\nfunc f() { other.Sleep(1) }\n"
		if got := CheckTest(empty, path, src); len(got) != 0 {
			t.Fatalf("CheckTest = %q, want none", got)
		}
	})
	t.Run("a file that does not parse is a finding, never a pass", func(t *testing.T) {
		t.Parallel()
		got := CheckTest(empty, path, "package x\n\nfunc (\n")
		if len(got) != 1 || !strings.HasPrefix(got[0], path+": testhygiene: the fence cannot read it: ") {
			t.Fatalf("CheckTest = %q, want one finding saying the fence cannot read %s", got, path)
		}
	})
}

// cliRoot is a fixture root with the lists, the import law and the named files written.
func cliRoot(t *testing.T, purity, clock, tests []string, files map[string]string) string {
	t.Helper()
	root := listRoot(t, purity, clock, tests)
	rules, err := os.ReadFile(filepath.Join("..", "importcheck", "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	files["tools/importcheck/rules.json"] = string(rules)
	for rel, src := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// runCLI writes the NUL-separated list of names and runs cli at root; it gives cli's exit code and output.
func runCLI(t *testing.T, root string, tests bool, names ...string) (int, string) {
	t.Helper()
	list := filepath.Join(t.TempDir(), "files.nul")
	var text string
	for _, n := range names {
		text += n + "\x00"
	}
	if err := os.WriteFile(list, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{list}
	if tests {
		args = []string{"-tests", list}
	}
	var out strings.Builder
	code := cli(root, args, &out)
	return code, out.String()
}

func TestCLI(t *testing.T) {
	t.Parallel()
	const (
		sleepy = "internal/x/a_test.go"
		calm   = "internal/x/b_test.go"
	)
	sleepySrc := "package x\n\nimport \"time\"\n\nfunc f() { time.Sleep(1) }\n"
	twoTests := func() map[string]string { return map[string]string{sleepy: sleepySrc, calm: "package x\n"} }
	cases := []struct {
		name  string
		root  func(t *testing.T) string
		tests bool
		files []string
		code  int
		out   string
	}{
		{
			name:  "-tests: each finding after failed:, then the test files read",
			root:  func(t *testing.T) string { return cliRoot(t, nil, nil, nil, twoTests()) },
			tests: true, files: []string{sleepy, calm}, code: 1,
			out: "failed: " + sleepy + ":5: testhygiene: time.Sleep in a test" + fence + "\nGATE_COVERED 2\n",
		},
		{
			name:  "-tests: a listed file is read and counted, never judged",
			root:  func(t *testing.T) string { return cliRoot(t, nil, nil, []string{sleepy}, twoTests()) },
			tests: true, files: []string{sleepy, calm}, code: 0,
			out: "GATE_COVERED 2\n",
		},
		{
			name: "-tests: a non-test file is an internal error naming it",
			root: func(t *testing.T) string {
				return cliRoot(t, nil, nil, nil, map[string]string{"internal/x/a.go": "package x\n"})
			},
			tests: true, files: []string{"internal/x/a.go"}, code: 2,
			out: "internal error: internal/x/a.go: -tests reads _test.go files only\n",
		},
		{
			name: "-tests: a stale testhygiene target is red, a stale purity one is the normal mode's",
			root: func(t *testing.T) string {
				return cliRoot(t, []string{"internal/gone: sync.Mutex"}, nil, []string{"io/gone_test.go"}, map[string]string{})
			},
			tests: true, code: 1,
			out: "failed: tools/allowlists/testhygiene.json: io/gone_test.go: does not exist (a stale target)\nGATE_COVERED 0\n",
		},
		{
			name: "normal mode: a finding after failed:, then the files read",
			root: func(t *testing.T) string {
				return cliRoot(t, nil, nil, nil, map[string]string{"internal/event/planted.go": "package event\n\nimport _ \"" + mod + "internal/model\"\n"})
			},
			files: []string{"internal/event/planted.go"}, code: 1,
			out: "failed: internal/event/planted.go:3: imports: internal/event may not import internal/model (ARCHITECTURE §3; tools/importcheck/rules.json)\nGATE_COVERED 1\n",
		},
		{
			name: "normal mode: stale purity and clock targets are red, a stale testhygiene one is -tests'",
			root: func(t *testing.T) string {
				return cliRoot(t, []string{"internal/gone: sync.Mutex"}, []string{"internal/window/cron.go: time"}, []string{"io/gone_test.go"},
					map[string]string{"internal/model/x.go": "package model\n"})
			},
			files: []string{"internal/model/x.go"}, code: 1,
			out: "failed: tools/allowlists/clock.json: internal/window/cron.go: time: does not exist (a stale target)\n" +
				"failed: tools/allowlists/purity.json: internal/gone: sync.Mutex: does not exist (a stale target)\nGATE_COVERED 1\n",
		},
		{
			// The fix round (BK-S3) changed the fixture: a present target must also be used, or it waives nothing.
			name: "normal mode: a clean file and present, used targets pass",
			root: func(t *testing.T) string {
				return cliRoot(t, []string{"internal/model: sync.Mutex"}, nil, nil, map[string]string{"internal/model/x.go": "package model\n\nimport \"sync\"\n\ntype T struct{ mu sync.Mutex }\n"})
			},
			files: []string{"internal/model/x.go"}, code: 0,
			out: "GATE_COVERED 1\n",
		},
		{
			name: "a missing list file is an internal error naming it",
			root: func(t *testing.T) string {
				root := cliRoot(t, nil, nil, nil, map[string]string{"internal/model/x.go": "package model\n"})
				if err := os.Remove(filepath.Join(root, "tools", "allowlists", "purity.json")); err != nil {
					t.Fatal(err)
				}
				return root
			},
			files: []string{"internal/model/x.go"}, code: 2,
			out: "internal error: tools/allowlists/purity.json: ",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			code, out := runCLI(t, c.root(t), c.tests, c.files...)
			match := out == c.out
			if c.code == 2 {
				match = strings.HasPrefix(out, c.out) && strings.Count(out, "\n") == 1
			}
			if code != c.code || !match {
				t.Fatalf("cli: exit %d, output %q; want exit %d, output %q", code, out, c.code, c.out)
			}
		})
	}
	t.Run("usage", func(t *testing.T) {
		t.Parallel()
		for _, args := range [][]string{nil, {"-tests"}, {"-x", "files.nul"}, {"a", "b"}, {"-cases"}} {
			var out strings.Builder
			if code := cli(t.TempDir(), args, &out); code != 2 || out.String() != usage+"\n" {
				t.Fatalf("cli %q: exit %d, output %q; want exit 2 and the usage line", args, code, out.String())
			}
		}
	})
}

// TestCases is -cases' table rules (S1.1c-2 design §4.2, DA-S7's Cases(path, src string) (cases, faults []string)):
// a case per top-level Test and per t.Run literal, nested names joined with "/", a range over an identifier-bound or
// inline composite literal counted by its elements (a [:n] slice counts n), anything else a fault; a counted case
// that never reaches Fatal, Fatalf, Error, Errorf or FailNow is a fault (DA-S12).
//
// design ambiguity: Cases returns the names without the "case " prefix and the faults without "fault ".
// design ambiguity: the names of a range-counted table's elements; asserted by count under the test's prefix.
// design ambiguity: a parent whose only verdicts sit in its t.Run closures counts as calling Fatal; read as yes.
// design ambiguity: the no-verdict fault's text; asserted as one fault beginning with the test's name.
func TestCases(t *testing.T) {
	const head = "package x\n\nimport \"testing\"\n\n"
	count := func(cases []string, prefix string) int {
		n := 0
		for _, c := range cases {
			if strings.HasPrefix(c, prefix) {
				n++
			}
		}
		return n
	}
	t.Run("literal subtests and nesting", func(t *testing.T) {
		src := head + `func TestA(t *testing.T) {
	t.Run("one", func(t *testing.T) {
		t.Run("two", func(t *testing.T) { t.Fatal("x") })
	})
	t.Run("three", func(t *testing.T) { t.Error("x") })
}
`
		cases, faults := Cases("x_test.go", src)
		if len(faults) != 0 {
			t.Fatalf("faults %q", faults)
		}
		want := []string{"TestA", "TestA/one", "TestA/one/two", "TestA/three"}
		slices.Sort(cases)
		if !slices.Equal(cases, want) {
			t.Fatalf("cases %q, want %q", cases, want)
		}
	})
	t.Run("an identifier bound to a literal", func(t *testing.T) {
		src := head + `func TestT(t *testing.T) {
	table := []struct{ name string }{{"a"}, {"b"}, {"c"}}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) { t.Fatal(tc.name) })
	}
}
`
		cases, faults := Cases("x_test.go", src)
		if len(faults) != 0 || count(cases, "TestT/") != 3 || !slices.Contains(cases, "TestT") {
			t.Fatalf("cases %q faults %q, want TestT and 3 elements", cases, faults)
		}
	})
	t.Run("an inline literal", func(t *testing.T) {
		src := head + `func TestI(t *testing.T) {
	for name, want := range map[string]int{"a": 1, "b": 2} {
		t.Run(name, func(t *testing.T) { t.Errorf("%d", want) })
	}
}
`
		cases, faults := Cases("x_test.go", src)
		if len(faults) != 0 || count(cases, "TestI/") != 2 {
			t.Fatalf("cases %q faults %q, want 2 elements", cases, faults)
		}
	})
	t.Run("a constant slice of a bound literal", func(t *testing.T) {
		src := head + `func TestS(t *testing.T) {
	table := []string{"a", "b", "c", "d"}
	for _, name := range table[:2] {
		t.Run(name, func(t *testing.T) { t.Fatal(name) })
	}
}
`
		cases, faults := Cases("x_test.go", src)
		if len(faults) != 0 || count(cases, "TestS/") != 2 {
			t.Fatalf("cases %q faults %q, want 2 elements", cases, faults)
		}
	})
	t.Run("a range over a function call is a fault", func(t *testing.T) {
		src := head + `func names() []string { return []string{"a"} }

func TestF(t *testing.T) {
	for _, name := range names() {
		t.Run(name, func(t *testing.T) { t.Fatal(name) })
	}
}
`
		_, faults := Cases("x_test.go", src)
		want := []string{"TestF: t.Run over a table -cases cannot count"}
		if !slices.Equal(faults, want) {
			t.Fatalf("faults %q, want %q", faults, want)
		}
	})
	t.Run("a case that never fails is a fault", func(t *testing.T) {
		src := head + `func TestGood(t *testing.T) { fail(t) }

func fail(t *testing.T) { t.Helper(); t.FailNow() }

func TestNoVerdict(t *testing.T) {
	_ = 1 + 1
	t.Log("never fails")
}
`
		cases, faults := Cases("x_test.go", src)
		if len(faults) != 1 || !strings.HasPrefix(faults[0], "TestNoVerdict") {
			t.Fatalf("faults %q, want one naming TestNoVerdict", faults)
		}
		if !slices.Contains(cases, "TestGood") {
			t.Fatalf("cases %q: a same-file helper's FailNow counts", cases)
		}
	})
}

// ---- S1.1c-2 lane D2's own cases: Cases' names, its scope rules and its helper depth; the -cases listing.

// usage is the one usage line every mode shares (lane D2 adds -cases).
const usage = "usage: go run ./tools/archcheck [-tests] <files.nul> | -cases <file>..."

// TestCasesNames pins Cases' exact output (design §11's seam): a range-counted table's elements are named
// <ident>[<i>], <ident> being the table's identifier, or for an inline literal the loop's first named variable.
// Every expectation below was written by hand from design §4.2 and §11 before Cases existed.
func TestCasesNames(t *testing.T) {
	t.Parallel()
	const head = "package x\n\nimport \"testing\"\n\n"
	for _, c := range []struct {
		name, src     string
		cases, faults []string
	}{
		{
			name: "an identifier bound to a slice literal",
			src: `func TestT(t *testing.T) {
	table := []struct{ name string }{{"a"}, {"b"}}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) { t.Fatal(tc.name) })
	}
}
`,
			cases: []string{"TestT", "TestT/table[0]", "TestT/table[1]"},
		},
		{
			name: "a map literal bound by var",
			src: `func TestM(t *testing.T) {
	var want = map[string]int{"a": 1, "b": 2, "c": 3}
	for k, v := range want {
		t.Run(k, func(t *testing.T) { t.Errorf("%d", v) })
	}
}
`,
			cases: []string{"TestM", "TestM/want[0]", "TestM/want[1]", "TestM/want[2]"},
		},
		{
			name: "[:2] of a bound literal",
			src: `func TestS(t *testing.T) {
	table := []string{"a", "b", "c"}
	for _, n := range table[:2] {
		t.Run(n, func(t *testing.T) { t.Error(n) })
	}
}
`,
			cases: []string{"TestS", "TestS/table[0]", "TestS/table[1]"},
		},
		{
			name: "a nested t.Run inside a range, over an inline literal",
			src: `func TestN(t *testing.T) {
	for _, n := range []string{"a", "b"} {
		t.Run(n, func(t *testing.T) {
			t.Run("inner", func(t *testing.T) { t.FailNow() })
		})
	}
}
`,
			cases: []string{"TestN", "TestN/n[0]", "TestN/n[0]/inner", "TestN/n[1]", "TestN/n[1]/inner"},
		},
		{
			name: "a table bound in the test, ranged inside a subtest",
			src: `func TestO(t *testing.T) {
	cases := [...]string{"a", "b"}
	t.Run("group", func(t *testing.T) {
		for _, c := range cases {
			t.Run(c, func(t *testing.T) { t.Error(c) })
		}
	})
}
`,
			cases: []string{"TestO", "TestO/group", "TestO/group/cases[0]", "TestO/group/cases[1]"},
		},
		{
			name: "a helper that calls t.Fatalf counts",
			src: `func must(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatalf("not ok")
	}
}

func TestH(t *testing.T) { must(t, true) }
`,
			cases: []string{"TestH"},
		},
		{
			name: "a helper two levels deep does not count",
			src: `func outer(t *testing.T) { inner(t) }

func inner(t *testing.T) { t.Fatal("x") }

func TestDeep(t *testing.T) { outer(t) }
`,
			cases:  []string{"TestDeep"},
			faults: []string{"TestDeep: calls no Fatal, Fatalf, Error, Errorf or FailNow"},
		},
		{
			name: "a helper's verdict counts only when the case passes it its T",
			src: `func fail(t *testing.T, other *testing.T) { other.Fatal("x") }

func TestWrongT(t *testing.T) { fail(t, nil) }
`,
			cases:  []string{"TestWrongT"},
			faults: []string{"TestWrongT: calls no Fatal, Fatalf, Error, Errorf or FailNow"},
		},
		{
			name: "a subtest that never fails is faulted by its own name; its parent counts",
			src: `func TestQ(t *testing.T) {
	t.Run("loud", func(t *testing.T) { t.Error("x") })
	t.Run("quiet", func(t *testing.T) { t.Log("x") })
}
`,
			cases:  []string{"TestQ", "TestQ/loud", "TestQ/quiet"},
			faults: []string{"TestQ/quiet: calls no Fatal, Fatalf, Error, Errorf or FailNow"},
		},
		{
			name: "non-test functions, benchmarks, examples, fuzz targets and methods are ignored",
			src: `func helper(t *testing.T) {}

func Testify(t *testing.T) {}

func TestMain(m *testing.M) {}

func BenchmarkX(b *testing.B) {
	for b.Loop() {
	}
}

func ExampleX() {}

func FuzzX(f *testing.F) {}

type s struct{}

func (s) TestY(t *testing.T) {}

func Test(t *testing.T) { t.Fatal("x") }

func Test_z(t *testing.T) { t.Fatal("x") }
`,
			cases: []string{"Test", "Test_z"},
		},
		{
			name: "a three-clause for over t.Run cannot be counted",
			src: `func TestL(t *testing.T) {
	for i := 0; i < 2; i++ {
		t.Run(string(rune('a'+i)), func(t *testing.T) { t.Fatal(i) })
	}
}
`,
			cases:  []string{"TestL"},
			faults: []string{"TestL: t.Run over a table -cases cannot count"},
		},
		{
			name: "a table grown after its binding cannot be counted",
			src: `func TestG(t *testing.T) {
	table := []string{"a"}
	table = append(table, "b")
	for _, n := range table {
		t.Run(n, func(t *testing.T) { t.Fatal(n) })
	}
}
`,
			cases:  []string{"TestG"},
			faults: []string{"TestG: t.Run over a table -cases cannot count"},
		},
		{
			name: "a package-level table and a parameter are not bound in the function",
			src: `var names = []string{"a"}

func TestP(t *testing.T) {
	for _, n := range names {
		t.Run(n, func(t *testing.T) { t.Fatal(n) })
	}
	run := func(names []string) {
		for _, n := range names {
			t.Run(n, func(t *testing.T) { t.Fatal(n) })
		}
	}
	run(nil)
}
`,
			cases:  []string{"TestP"},
			faults: []string{"TestP: t.Run over a table -cases cannot count", "TestP: t.Run over a table -cases cannot count"},
		},
		{
			name: "a parameter shadowing a bound table is not the table",
			src: `func TestShadow(t *testing.T) {
	names := []string{"a", "b"}
	run := func(names []string) {
		for _, n := range names {
			t.Run(n, func(t *testing.T) { t.Fatal(n) })
		}
	}
	run(names)
}
`,
			cases:  []string{"TestShadow"},
			faults: []string{"TestShadow: t.Run over a table -cases cannot count"},
		},
		{
			name: "a three-clause for inside a counted range cannot be counted",
			src: `func TestFor(t *testing.T) {
	for _, s := range []string{"a", "b"} {
		for i := 0; i < 2; i++ {
			t.Run(s, func(t *testing.T) { t.Fatal(i) })
		}
	}
}
`,
			cases:  []string{"TestFor"},
			faults: []string{"TestFor: t.Run over a table -cases cannot count"},
		},
		{
			name: "nested ranges over t.Run cannot be counted",
			src: `func TestR(t *testing.T) {
	for _, a := range []string{"a", "b"} {
		for _, b := range []string{"c", "d"} {
			t.Run(a+b, func(t *testing.T) { t.Fatal(a, b) })
		}
	}
}
`,
			cases:  []string{"TestR"},
			faults: []string{"TestR: t.Run over a table -cases cannot count"},
		},
		{
			name: "a non-literal t.Run outside any range cannot be counted",
			src: `func TestV(t *testing.T) {
	name := "a"
	t.Run(name, func(t *testing.T) { t.Fatal(name) })
}
`,
			cases:  []string{"TestV"},
			faults: []string{"TestV: t.Run over a table -cases cannot count"},
		},
		{
			name: "a keyed or fixed-length array literal",
			src: `func TestK(t *testing.T) {
	for _, n := range [3]string{"a"} {
		t.Run(n, func(t *testing.T) { t.Fatal(n) })
	}
	for _, n := range []string{2: "c"} {
		t.Run(n, func(t *testing.T) { t.Fatal(n) })
	}
}
`,
			cases:  []string{"TestK", "TestK/n[0]", "TestK/n[1]", "TestK/n[2]"},
			faults: []string{"TestK: t.Run over a table -cases cannot count"},
		},
		{
			name: "a subtest named by a same-file function is read",
			src: `func sub(t *testing.T) { t.Error("x") }

func quiet(t *testing.T) {}

func TestNamed(t *testing.T) {
	t.Run("sub", sub)
	t.Run("quiet", quiet)
	t.Fatal("x")
}
`,
			cases:  []string{"TestNamed", "TestNamed/sub", "TestNamed/quiet"},
			faults: []string{"TestNamed/quiet: calls no Fatal, Fatalf, Error, Errorf or FailNow"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cases, faults := Cases("x_test.go", head+c.src)
			if !slices.Equal(cases, c.cases) || !slices.Equal(faults, c.faults) {
				t.Fatalf("Cases = %q, %q\nwant %q, %q", cases, faults, c.cases, c.faults)
			}
		})
	}
	t.Run("a file that does not parse is one fault naming no test", func(t *testing.T) {
		t.Parallel()
		const src = "package x\n\nfunc (\n"
		_, want := parser.ParseFile(token.NewFileSet(), "x_test.go", src, parser.SkipObjectResolution)
		cases, faults := Cases("x_test.go", src)
		if len(cases) != 0 || want == nil || !slices.Equal(faults, []string{": " + want.Error()}) {
			t.Fatalf("Cases = %q, %q; want no case and the fault %q", cases, faults, ": "+want.Error())
		}
	})
}

// TestCLICases is -cases end to end in-process: per file in argument order, its cases then its faults, each line
// "<path>: case <name>" or "<path>: fault <name>: <why>"; a file that does not parse or cannot be read is one
// "<path>: fault : <error>" line; the exit is 0 whatever the listing holds (design §11; the Python side decides).
func TestCLICases(t *testing.T) {
	t.Parallel()
	const (
		a    = "internal/x/a_test.go"
		b    = "internal/x/b_test.go"
		gone = "internal/x/gone_test.go"
	)
	const bSrc = "package x\n\nfunc (\n"
	root := t.TempDir()
	for rel, src := range map[string]string{
		a: `package x

import "testing"

func TestA(t *testing.T) {
	t.Run("quiet", func(t *testing.T) { t.Log("x") })
	for _, n := range []string{"p", "q"} {
		t.Run(n, func(t *testing.T) { t.Error(n) })
	}
}

func TestB(t *testing.T) {
	for _, n := range names() {
		t.Run(n, func(t *testing.T) { t.Error(n) })
	}
}

func names() []string { return nil }
`,
		b: bSrc,
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, parseErr := parser.ParseFile(token.NewFileSet(), b, bSrc, parser.SkipObjectResolution)
	_, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(gone)))
	if parseErr == nil || readErr == nil {
		t.Fatalf("fixture: want a parse error and a read error, got %v, %v", parseErr, readErr)
	}
	want := a + ": case TestA\n" +
		a + ": case TestA/quiet\n" +
		a + ": case TestA/n[0]\n" +
		a + ": case TestA/n[1]\n" +
		a + ": case TestB\n" +
		a + ": fault TestA/quiet: calls no Fatal, Fatalf, Error, Errorf or FailNow\n" +
		a + ": fault TestB: t.Run over a table -cases cannot count\n" +
		gone + ": fault : " + readErr.Error() + "\n" +
		b + ": fault : " + parseErr.Error() + "\n"
	var out strings.Builder
	if code := cli(root, []string{"-cases", a, gone, b}, &out); code != 0 || out.String() != want {
		t.Fatalf("cli -cases: exit %d, output\n%s\nwant exit 0, output\n%s", code, out.String(), want)
	}
}

// ---- S1.1c-2's fix round (docs/reviews/s1-1c-2-breakit-qa.md): BK-B4, BK-S1, BK-S3 and BK-S6 beyond the break-it's
// own reds in breakit_s11c2_test.go.

// constrained is Cases' one fault for a file go test does not always compile (BK-B4); -cases prints it as
// "<path>: fault : …", the line shape of a file it cannot read (design §11).
const constrained = ": a build-constrained proof file is not always compiled"

// TestFixCasesBuildConstraints (BK-B4): a constraint line, a GOOS or GOARCH file-name suffix, or a name go ignores
// makes a proof file one go test may never compile; Cases counts none of its cases and gives the one fault.
func TestFixCasesBuildConstraints(t *testing.T) {
	t.Parallel()
	const body = "package x\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { t.Fatal(\"x\") }\n"
	for _, c := range []struct{ name, path, src string }{
		{"//go:build ignore", "x_test.go", "//go:build ignore\n\n" + body},
		{"//go:build on a real platform", "x_test.go", "//go:build linux\n\n" + body},
		{"// +build never", "x_test.go", "// +build never\n\n" + body},
		{"//+build never, no space", "x_test.go", "//+build never\n\n" + body},
		{"a GOOS suffix", "internal/x/x_linux_test.go", body},
		{"a GOARCH suffix", "internal/x/x_amd64_test.go", body},
		{"a GOOS_GOARCH suffix", "internal/x/x_windows_arm64_test.go", body},
		{"a name go ignores", "internal/x/_x_test.go", body},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cases, faults := Cases(c.path, c.src)
			if len(cases) != 0 || !slices.Equal(faults, []string{constrained}) {
				t.Fatalf("Cases = %q, %q; want no case and the one fault %q", cases, faults, constrained)
			}
		})
	}
	for _, c := range []struct{ name, path string }{
		{"a platform before the first underscore is no suffix", "internal/x/linux_test.go"},
		{"a word that names no platform", "internal/x/x_gate_test.go"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cases, faults := Cases(c.path, body)
			if !slices.Equal(cases, []string{"TestA"}) || len(faults) != 0 {
				t.Fatalf("Cases = %q, %q; want [TestA] and no fault", cases, faults)
			}
		})
	}
	t.Run("-cases prints it on the file's one fault line", func(t *testing.T) {
		t.Parallel()
		const path = "internal/x/x_test.go"
		root := t.TempDir()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("//go:build ignore\n\n"+body), 0o644); err != nil {
			t.Fatal(err)
		}
		want := path + ": fault : a build-constrained proof file is not always compiled\n"
		var out strings.Builder
		if code := cli(root, []string{"-cases", path}, &out); code != 0 || out.String() != want {
			t.Fatalf("cli -cases: exit %d, output %q; want exit 0, output %q", code, out.String(), want)
		}
	})
}

// TestFixCasesLiveVerdicts (BK-S1): a verdict counts only in live code, as the Python strength rule's: not under an
// if (or for) whose condition folds to false from literals, not in the else of one that folds to true, not in a
// range over an empty literal, not after a terminating statement in its list (a return, a branch, a panic, or a
// Fatal, FailNow or Skip on the T), not in a closure bound to a name (or _) that no live call calls or passes on.
func TestFixCasesLiveVerdicts(t *testing.T) {
	t.Parallel()
	const head = "package x\n\nimport \"testing\"\n\n"
	test := func(body string) string { return head + "func TestA(t *testing.T) {\n\t" + body + "\n}\n" }
	dead := []struct{ name, body string }{
		{"under if !true", "if !true {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"under if 0 == 1", "if 0 == 1 {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"under if (\"a\" == \"b\")", "if (\"a\" == \"b\") {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"under if false && x", "x := t.Name() != \"\"\n\tif false && x {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"under if x && 1 > 2", "x := t.Name() != \"\"\n\tif x && 1 > 2 {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"in the else of if true", "if true {\n\t\tt.Log(\"x\")\n\t} else {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"under for false", "for false {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"in a range over an empty literal", "for range []int{} {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"after a return", "return\n\tt.Fatal(\"x\")"},
		{"after a panic", "panic(\"x\")\n\tt.Fatal(\"x\")"},
		{"after t.Skip", "t.Skip(\"x\")\n\tt.Fatal(\"x\")"},
		{"after t.SkipNow, in a block", "{\n\t\tt.SkipNow()\n\t\tt.Error(\"x\")\n\t}"},
		{"after an if whose branches both return", "if t.Name() == \"\" {\n\t\treturn\n\t} else {\n\t\treturn\n\t}\n\tt.Fatal(\"x\")"},
		{"after a break", "for {\n\t\tbreak\n\t\tt.Fatal(\"x\")\n\t}"},
		{"in a closure bound to a name never called", "f := func() { t.Fatal(\"x\") }\n\t_ = f"},
		{"in a closure bound by var, never called", "var f = func() { t.Fatal(\"x\") }\n\t_ = f"},
		{"in a closure whose only call is dead", "f := func() { t.Fatal(\"x\") }\n\tif false {\n\t\tf()\n\t}"},
	}
	for _, c := range dead {
		t.Run("dead: "+c.name, func(t *testing.T) {
			t.Parallel()
			cases, faults := Cases("x_test.go", test(c.body))
			if !slices.Equal(cases, []string{"TestA"}) || !slices.Equal(faults, []string{"TestA: " + noVerdict}) {
				t.Fatalf("Cases = %q, %q; want [TestA] and its no-verdict fault: the verdict %s counted", cases, faults, c.name)
			}
		})
	}
	live := []struct{ name, body string }{
		{"under if true", "if true {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"under if 1 == 1", "if 1 == 1 {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"under a condition that is not constant", "if t.Name() == \"\" {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"in the else of if false", "if false {\n\t\tt.Log(\"x\")\n\t} else {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"before a return", "t.Fatal(\"x\")\n\treturn"},
		{"after an if that returns on one branch", "if t.Name() == \"\" {\n\t\treturn\n\t}\n\tt.Fatal(\"x\")"},
		{"after a goto, at its label", "goto L\nL:\n\tt.Fatal(\"x\")"},
		{"in a range over a literal with an element", "for range []int{1} {\n\t\tt.Fatal(\"x\")\n\t}"},
		{"in a closure called at once", "func() { t.Fatal(\"x\") }()"},
		{"in a deferred closure", "defer func() { t.Fatal(\"x\") }()"},
		{"in a closure passed to t.Cleanup", "t.Cleanup(func() { t.Error(\"x\") })"},
		{"in a closure bound to a name, then called", "f := func() { t.Fatal(\"x\") }\n\tf()"},
		{"in a closure bound to a name, then passed", "f := func() { t.Error(\"x\") }\n\tt.Cleanup(f)"},
	}
	for _, c := range live {
		t.Run("live: "+c.name, func(t *testing.T) {
			t.Parallel()
			cases, faults := Cases("x_test.go", test(c.body))
			if !slices.Equal(cases, []string{"TestA"}) || len(faults) != 0 {
				t.Fatalf("Cases = %q, %q; want [TestA] and no fault: the verdict %s is live", cases, faults, c.name)
			}
		})
	}
	t.Run("a helper whose only verdict is dead does not count", func(t *testing.T) {
		t.Parallel()
		src := head + "func check(t *testing.T) {\n\tif false {\n\t\tt.Fatal(\"x\")\n\t}\n}\n\nfunc TestA(t *testing.T) { check(t) }\n"
		cases, faults := Cases("x_test.go", src)
		if !slices.Equal(cases, []string{"TestA"}) || !slices.Equal(faults, []string{"TestA: " + noVerdict}) {
			t.Fatalf("Cases = %q, %q; want [TestA] and its no-verdict fault", cases, faults)
		}
	})
	t.Run("a subtest in dead code is listed and is a no-verdict fault", func(t *testing.T) {
		t.Parallel()
		src := test("t.Error(\"parent\")\n\tif false {\n\t\tt.Run(\"s\", func(t *testing.T) { t.Fatal(\"x\") })\n\t}")
		cases, faults := Cases("x_test.go", src)
		if !slices.Equal(cases, []string{"TestA", "TestA/s"}) || !slices.Equal(faults, []string{"TestA/s: " + noVerdict}) {
			t.Fatalf("Cases = %q, %q; want [TestA TestA/s] and TestA/s's no-verdict fault", cases, faults)
		}
	})
}

// TestFixUnusedTargets (BK-S3): each mode reports a target of its own lists whose file or package exists but which
// waives nothing — "failed: tools/allowlists/<name>.json: <target>: waives nothing (an unused target)" — and leaves the
// other mode's lists alone, as Stale's scoping does.
func TestFixUnusedTargets(t *testing.T) {
	t.Parallel()
	const (
		model   = "internal/model/x.go"
		cron    = "internal/window/cron.go"
		calm    = "internal/x/a_test.go"
		unused  = ": waives nothing (an unused target)\n"
		mutexed = "package model\n\nimport \"sync\"\n\ntype T struct{ mu sync.Mutex }\n"
	)
	cases := []struct {
		name  string
		root  func(t *testing.T) string
		tests bool
		files []string
		code  int
		out   string
	}{
		{
			name: "normal mode: a purity target whose package never uses the identifier",
			root: func(t *testing.T) string {
				return cliRoot(t, []string{"internal/model: sync.Mutex"}, nil, nil, map[string]string{model: "package model\n"})
			},
			files: []string{model}, code: 1,
			out: "failed: tools/allowlists/purity.json: internal/model: sync.Mutex" + unused + "GATE_COVERED 1\n",
		},
		{
			name: "normal mode: a purity target used only by a test file, or another identifier used",
			root: func(t *testing.T) string {
				return cliRoot(t, []string{"internal/model: sync.Mutex", "internal/model: sync.Pool"}, nil, nil, map[string]string{
					model:                      "package model\n\nimport \"sync\"\n\nvar _ = sync.Pool{}\n",
					"internal/model/x_test.go": mutexed,
				})
			},
			files: []string{model}, code: 1,
			out: "failed: tools/allowlists/purity.json: internal/model: sync.Mutex" + unused + "GATE_COVERED 1\n",
		},
		{
			name: "normal mode: a clock target whose file does not import time",
			root: func(t *testing.T) string {
				return cliRoot(t, nil, []string{cron + ": time"}, nil, map[string]string{cron: "package window\n"})
			},
			files: []string{cron}, code: 1,
			out: "failed: tools/allowlists/clock.json: " + cron + ": time" + unused + "GATE_COVERED 1\n",
		},
		{
			name: "normal mode: a clock target whose file imports time is used",
			root: func(t *testing.T) string {
				return cliRoot(t, nil, []string{cron + ": time"}, nil, map[string]string{cron: cronSrc})
			},
			files: []string{cron}, code: 0,
			out: "GATE_COVERED 1\n",
		},
		{
			name: "normal mode: an unused testhygiene target is -tests'",
			root: func(t *testing.T) string {
				return cliRoot(t, nil, nil, []string{calm}, map[string]string{model: "package model\n", calm: "package x\n"})
			},
			files: []string{model}, code: 0,
			out: "GATE_COVERED 1\n",
		},
		{
			name: "-tests: a testhygiene target whose file calls no banned name",
			root: func(t *testing.T) string {
				return cliRoot(t, nil, nil, []string{calm}, map[string]string{calm: "package x\n\nimport \"time\"\n\nvar _ = time.Second\n"})
			},
			tests: true, files: []string{calm}, code: 1,
			out: "failed: tools/allowlists/testhygiene.json: " + calm + unused + "GATE_COVERED 1\n",
		},
		{
			name: "-tests: unused purity and clock targets are the normal mode's",
			root: func(t *testing.T) string {
				return cliRoot(t, []string{"internal/model: sync.Mutex"}, []string{cron + ": time"}, nil,
					map[string]string{model: "package model\n", cron: "package window\n", calm: "package x\n"})
			},
			tests: true, files: []string{calm}, code: 0,
			out: "GATE_COVERED 1\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			code, out := runCLI(t, c.root(t), c.tests, c.files...)
			if code != c.code || out != c.out {
				t.Fatalf("cli: exit %d, output %q; want exit %d, output %q", code, out, c.code, c.out)
			}
		})
	}
	t.Run("the repository's own lists have no unused target", func(t *testing.T) {
		t.Parallel()
		got, err := Unused(testLists(t), filepath.Join("..", ".."))
		if err != nil || len(got) != 0 {
			t.Fatalf("Unused = %q, %v; want none", got, err)
		}
	})
}

// TestFixGenerated (BK-S6; owner, 2026-11-12, S1.1c-2's review and break-it 3: "archcheck and the coverage tool accept
// `// Code generated … DO NOT EDIT.` only before the package clause (Go's own rule), and every generated file must be
// listed by name in a new list in the one grammar (empty today)"): tools/allowlists/generated.json, rule generated;
// only a listed file with the marker skips the globals check; a marked file not listed gets every check and a finding
// naming it; a listed file without the marker waives nothing; a listed file that is gone is stale.
func TestFixGenerated(t *testing.T) {
	t.Parallel()
	const (
		path   = "internal/model/x.go"
		marker = "// Code generated by gen. DO NOT EDIT.\n\n"
		global = "package model\n\nvar state = map[string]int{}\n\nfunc f() { state[\"a\"] = 1 }\n"
	)
	parse := func(t *testing.T, src string) bool {
		t.Helper()
		file, err := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		return generated(file)
	}
	t.Run("the marker before the package clause, after a licence comment too, marks the file", func(t *testing.T) {
		t.Parallel()
		if !parse(t, marker+global) || !parse(t, "// Licence.\n\n"+marker+global) {
			t.Fatal("a marker before the package clause does not mark the file")
		}
	})
	withGenerated := func(t *testing.T, targets []string, files map[string]string) string {
		t.Helper()
		root := cliRoot(t, nil, nil, nil, files)
		writeList(t, root, "generated", "generated", targets)
		return root
	}
	t.Run("the list is read into Lists.Generated", func(t *testing.T) {
		t.Parallel()
		l := loadAt(t, withGenerated(t, []string{path}, map[string]string{}))
		if !l.Generated[path] || len(l.Generated) != 1 {
			t.Fatalf("Generated = %v, want {%s: true}", l.Generated, path)
		}
	})
	t.Run("a listed, marked file skips the globals check", func(t *testing.T) {
		t.Parallel()
		root := withGenerated(t, []string{path}, map[string]string{path: marker + global})
		code, out := runCLI(t, root, false, path)
		if code != 0 || out != "GATE_COVERED 1\n" {
			t.Fatalf("cli: exit %d, output %q; want exit 0, output %q", code, out, "GATE_COVERED 1\n")
		}
	})
	t.Run("a marked file not listed gets every check and a finding naming it", func(t *testing.T) {
		t.Parallel()
		got := checkWith(t, loadAt(t, listRoot(t, nil, nil, nil)), path, map[string]string{path: marker + global})
		want := path + ":1: generated: the file says it is generated but tools/allowlists/generated.json does not list it"
		if len(got) != 2 || got[0] != want || !strings.Contains(got[1], "globals: state is a mutable package-level var") {
			t.Fatalf("CheckSources = %q; want %q and the globals finding on state", got, want)
		}
	})
	t.Run("a listed file without the marker waives nothing", func(t *testing.T) {
		t.Parallel()
		root := withGenerated(t, []string{path}, map[string]string{path: "package model\n"})
		code, out := runCLI(t, root, false, path)
		want := "failed: tools/allowlists/generated.json: " + path + ": waives nothing (an unused target)\nGATE_COVERED 1\n"
		if code != 1 || out != want {
			t.Fatalf("cli: exit %d, output %q; want exit 1, output %q", code, out, want)
		}
	})
	t.Run("a listed file that is gone is stale in the normal mode, not in -tests", func(t *testing.T) {
		t.Parallel()
		const gone = "internal/model/gone.go"
		root := withGenerated(t, []string{gone}, map[string]string{path: "package model\n", "internal/x/a_test.go": "package x\n"})
		code, out := runCLI(t, root, false, path)
		want := "failed: tools/allowlists/generated.json: " + gone + ": does not exist (a stale target)\nGATE_COVERED 1\n"
		if code != 1 || out != want {
			t.Fatalf("cli: exit %d, output %q; want exit 1, output %q", code, out, want)
		}
		if code, out := runCLI(t, root, true, "internal/x/a_test.go"); code != 0 || out != "GATE_COVERED 1\n" {
			t.Fatalf("cli -tests: exit %d, output %q; want exit 0, output %q", code, out, "GATE_COVERED 1\n")
		}
	})
	for name, c := range map[string]struct{ text, want string }{
		"an empty target": {`{"rule": "generated", "entries": [{"target": ""}]}`, `tools/allowlists/generated.json: an empty target: want "<file>"`},
		"a rule mismatch": {`{"rule": "clock", "entries": []}`, `tools/allowlists/generated.json: rule "clock", want "generated"`},
		"a missing list":  {"", "tools/allowlists/generated.json: "},
	} {
		t.Run("LoadLists: "+name, func(t *testing.T) {
			t.Parallel()
			root := listRoot(t, nil, nil, nil)
			file := filepath.Join(root, "tools", "allowlists", "generated.json")
			var err error
			if c.text == "" {
				err = os.Remove(file)
			} else {
				err = os.WriteFile(file, []byte(c.text), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLists(root); err == nil || !strings.HasPrefix(err.Error(), c.want) {
				t.Fatalf("LoadLists: %v, want an error starting %q", err, c.want)
			}
		})
	}
}
