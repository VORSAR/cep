package main

// QA's break-it on S3.3-1's API gate (step 7): declarations whose API meaning the printed text can lose.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pkgDir writes one Go file into a fresh package directory and gives the directory.
func pkgDir(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestBreakitIotaRepetitionKeepsTypeAndValue (design s3-3-1 §6.1: "var, const: every exported name with its type or
// value as written"): a const block with iota and implicit repetition prints each name with its type and value, so
// swapping two repeated names (their values change: cepio.String would no longer equal event.TString) changes the
// surface. Today a repeated name prints as a bare `const B`, and the swap leaves the text, and the gate, unchanged.
func TestBreakitIotaRepetitionKeepsTypeAndValue(t *testing.T) {
	t.Parallel()
	before := surfaceOf(t, pkgDir(t, "package p\n\ntype T uint8\n\nconst (\n\tA T = iota\n\tB\n\tC\n)\n"))
	after := surfaceOf(t, pkgDir(t, "package p\n\ntype T uint8\n\nconst (\n\tA T = iota\n\tC\n\tB\n)\n"))
	if before == after {
		t.Errorf("swapping B and C (B 1→2, C 2→1) left the surface unchanged:\n%s\nwant B's and C's lines to carry their type and value", before)
	}
	if !strings.Contains(before, "p B const const B T") {
		t.Errorf("surface:\n%s\nwant B printed with its type T (and its value), not a bare `const B`", before)
	}
}

// TestBreakitAliasOfAnUnexportedTypeShowsItsFields: an exported alias of an unexported struct exposes that struct's
// exported fields; changing one's type changes the API. Today the alias prints as `type Public = hidden` and the
// unexported struct is skipped, so the change is invisible to the surface.
func TestBreakitAliasOfAnUnexportedTypeShowsItsFields(t *testing.T) {
	t.Parallel()
	before := surfaceOf(t, pkgDir(t, "package p\n\ntype Public = hidden\n\ntype hidden struct{ X int }\n"))
	after := surfaceOf(t, pkgDir(t, "package p\n\ntype Public = hidden\n\ntype hidden struct{ X string }\n"))
	if before == after {
		t.Errorf("changing the exported field X of the type behind the exported alias Public left the surface unchanged:\n%s", before)
	}
}

// TestBreakitGenericTypeAndTagsArePrinted (guard): an exported generic type with a method prints its type
// parameters on both; a struct tag change changes the surface.
func TestBreakitGenericTypeAndTagsArePrinted(t *testing.T) {
	t.Parallel()
	got := surfaceOf(t, pkgDir(t, "package p\n\ntype G[T any] struct{ V T }\n\nfunc (g *G[T]) Get() T { return g.V }\n"))
	for _, want := range []string{"p G type type G[T any] struct {\n\tV T\n}\n", "p Get func func (g *G[T]) Get() T\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("surface:\n%s\nwant it to contain %q", got, want)
		}
	}
	a := surfaceOf(t, pkgDir(t, "package p\n\ntype S struct {\n\tX int `json:\"x\"`\n}\n"))
	b := surfaceOf(t, pkgDir(t, "package p\n\ntype S struct {\n\tX int `json:\"y\"`\n}\n"))
	if a == b {
		t.Errorf("a struct tag change left the surface unchanged:\n%s", a)
	}
}
