package main

import (
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
	got, err := CheckSources(testRules(t), path, files)
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
		wantNone(t, check(t, "internal/window/cron.go", "package window\nimport \"time\"\nfunc f() (*time.Location, error) { return time.LoadLocation(\"UTC\") }\n"))
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
		wantNone(t, check(t, "internal/event/x.go", "package event\nimport _ \"sync\"\n"))
		wantNone(t, check(t, "internal/table/x.go", "package table\nimport _ \"sync/atomic\"\n"))
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
		wantNone(t, check(t, "internal/table/x.go", src))
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
				wantOneAll(t, check(t, "internal/table/x.go", src), "purity:", ident, "Mutex", "Pointer", adr)
			})
		}
	})
	t.Run("table: an allowed and a banned identifier in one file give one finding", func(t *testing.T) {
		src := "package table\nimport \"sync\"\ntype T struct {\nmu sync.Mutex\nrw sync.RWMutex\n}\n"
		wantOneAll(t, check(t, "internal/table/x.go", src), "purity:", "RWMutex")
	})
	t.Run("table: an aliased import is still read", func(t *testing.T) {
		wantOneAll(t, check(t, "internal/table/x.go", "package table\nimport s \"sync\"\ntype T struct{ mu s.RWMutex }\n"), "purity:", "RWMutex")
		wantNone(t, check(t, "internal/table/x.go", "package table\nimport s \"sync\"\ntype T struct{ mu s.Mutex }\n"))
	})
	t.Run("event may use sync.Pool", func(t *testing.T) {
		src := "package event\nimport \"sync\"\nvar pool = sync.Pool{New: func() any { return new([]byte) }}\nfunc get() any { return pool.Get() }\n"
		wantNone(t, check(t, "internal/event/x.go", src))
	})
	t.Run("event: any other sync identifier is red", func(t *testing.T) {
		for ident, src := range map[string]string{
			"Mutex":     "package event\nimport \"sync\"\ntype T struct{ mu sync.Mutex }\n",
			"Once":      "package event\nimport \"sync\"\nfunc f() { var o sync.Once; o.Do(f) }\n",
			"WaitGroup": "package event\nimport \"sync\"\nfunc f() { var wg sync.WaitGroup; wg.Wait() }\n",
		} {
			t.Run(ident, func(t *testing.T) {
				wantOneAll(t, check(t, "internal/event/x.go", src), "purity:", ident, "Pool", adr)
			})
		}
	})
	t.Run("event may not import sync/atomic", func(t *testing.T) {
		wantOneAll(t, check(t, "internal/event/x.go", "package event\nimport _ \"sync/atomic\"\n"), "purity:", "sync/atomic", adr)
	})
	t.Run("a dot-import of sync or sync/atomic is red in table and event", func(t *testing.T) {
		wantOneAll(t, check(t, "internal/table/x.go", "package table\nimport . \"sync\"\ntype T struct{ mu Mutex }\n"), "purity:", adr)
		wantOneAll(t, check(t, "internal/table/x.go", "package table\nimport . \"sync/atomic\"\ntype V struct{}\ntype T struct{ p Pointer[V] }\n"), "purity:", adr)
		wantOneAll(t, check(t, "internal/event/x.go", "package event\nimport . \"sync\"\nvar pool = Pool{}\n"), "purity:", adr)
		// sync/atomic is banned in event twice over (the import and the dot); one or two findings, all purity.
		got := check(t, "internal/event/x.go", "package event\nimport . \"sync/atomic\"\ntype T struct{ n Int64 }\n")
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
		wantOne(t, check(t, "internal/pattern/x.go", "package pattern\nimport \"sync\"\ntype T struct{ mu sync.Mutex }\n"), "purity: internal/pattern may not import sync")
		wantOne(t, check(t, "internal/text/x.go", "package text\nimport \"sync/atomic\"\ntype V struct{}\ntype T struct{ p atomic.Pointer[V] }\n"), "purity: internal/text may not import sync/atomic")
		wantOne(t, check(t, "internal/expr/x.go", "package expr\nimport \"sync\"\nvar _ = sync.Pool{}\n"), "purity: internal/expr may not import sync")
	})
	t.Run("tests in table and event are not held to it", func(t *testing.T) {
		wantNone(t, check(t, "internal/table/x_test.go", "package table_test\nimport \"sync\"\nfunc f() { var wg sync.WaitGroup; wg.Wait() }\n"))
		wantNone(t, check(t, "internal/event/x_test.go", "package event_test\nimport \"sync/atomic\"\nfunc f() { var n atomic.Int64; n.Add(1) }\n"))
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
