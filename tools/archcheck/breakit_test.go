package main

import (
	"strings"
	"testing"
)

// The S3.1-2 break-it pass over archcheck's named purity exceptions and the expr -> table fence (design note
// docs/sprints/s3-1-2-design.md §2 and §5; ADR-0015 §2). Each red test names its finding.

// wantFindingAll fails unless some finding holds every needle.
func wantFindingAll(t *testing.T, got []string, needles ...string) {
	t.Helper()
	for _, f := range got {
		all := true
		for _, n := range needles {
			all = all && strings.Contains(f, n)
		}
		if all {
			return
		}
	}
	t.Fatalf("want a finding containing all of %q, got %q", needles, got)
}

// B1 (blocker, design note §5; ADR-0015 §2): internal/table may use only sync.Mutex and atomic.Pointer, and
// internal/event only sync.Pool. Go accepts a second import of the same path under another name, and namedSync
// reads only the first import's local name (localName returns the first match; a blank first import skips the
// package outright), so every selector on the second name goes unchecked: sync.WaitGroup, sync.RWMutex and
// atomic.Int64 pass the gate.
func TestBreakitNamedSyncSecondImportName(t *testing.T) {
	cases := []struct{ path, src, banned string }{
		{"internal/table/x.go", "package table\nimport (\n\"sync\"\ns2 \"sync\"\n)\ntype T struct {\nmu sync.Mutex\nwg s2.WaitGroup\n}\n", "WaitGroup"},
		{"internal/table/x.go", "package table\nimport (\n\"sync/atomic\"\na2 \"sync/atomic\"\n)\ntype V struct{}\ntype T struct {\np atomic.Pointer[V]\nn a2.Int64\n}\n", "Int64"},
		{"internal/table/x.go", "package table\nimport (\n_ \"sync\"\ns \"sync\"\n)\ntype T struct{ wg s.WaitGroup }\n", "WaitGroup"},
		{"internal/event/x.go", "package event\nimport (\n\"sync\"\ns2 \"sync\"\n)\nvar pool = sync.Pool{}\ntype T struct{ mu s2.RWMutex }\n", "RWMutex"},
	}
	for _, c := range cases {
		t.Run(c.path+"/"+c.banned, func(t *testing.T) {
			wantFindingAll(t, check(t, c.path, c.src), "purity:", c.banned)
		})
	}
}

// B2 (blocker, design note §2; review F21; rules.json "fences"): internal/expr's production code may use
// table.Version only. A second import of internal/table under another name is not fenced (fences reads only the
// first import's local name), so expr's production code can build and write a table.
func TestBreakitExprFenceSecondImportName(t *testing.T) {
	src := "package expr\nimport (\n\"" + mod + "internal/table\"\nt2 \"" + mod + "internal/table\"\n)\n" +
		"func f(v *table.Version) error {\n_ = v.Len()\ntb, _ := t2.New(\"T\", nil, nil)\nreturn tb.Upsert(nil)\n}\n"
	wantFindingAll(t, check(t, "internal/expr/x.go", src), "fence:", "New")
}

// B7 (should-fix; pre-dates S3.1-2, the same root as B1 and B2; ADR-0015 §2.2's clock rule): a package that may
// import time but not read the wall clock (here internal/plan) reads it through a second import name: clock() looks
// up only the first local name of "time".
func TestBreakitClockSecondImportName(t *testing.T) {
	src := "package plan\nimport (\n\"time\"\nt2 \"time\"\n)\nvar _ = time.Duration(0)\nfunc f() int64 { return t2.Now().UnixNano() }\n"
	wantFindingAll(t, check(t, "internal/plan/x.go", src), "clock:", "Now")
}

// B3 (blocker, design note §2; review F21; rules.json "fences"): a dot-import of a fenced package is not fenced.
// fences() looks for selectors whose qualifier is the local name, and a dot-import has none, so internal/expr's
// production code calls table.New and writes the table with no finding. The expr -> model fence (owner,
// 2026-11-04) has the same hole: model.App passes as App.
func TestBreakitFenceDotImport(t *testing.T) {
	t.Run("expr dot-imports table", func(t *testing.T) {
		src := "package expr\nimport . \"" + mod + "internal/table\"\nfunc f() error {\ntb, _ := New(\"T\", nil, nil)\nreturn tb.Upsert(nil)\n}\n"
		wantFindingAll(t, check(t, "internal/expr/x.go", src), "internal/table")
	})
	t.Run("expr dot-imports model", func(t *testing.T) {
		src := "package expr\nimport . \"" + mod + "internal/model\"\nvar _ = App{}\n"
		wantFindingAll(t, check(t, "internal/expr/x.go", src), "internal/model")
	})
}
