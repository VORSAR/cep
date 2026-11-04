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
