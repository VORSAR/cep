package main

// Known issue S3.3-1-1, fixed in S3.3-2: a method's entry is named `<Type>.<Method>`, the receiver's type without
// its `*`, so an API Δ line says which type's method changed (`cep reader.AttrIndex func …`, not `cep AttrIndex
// func …`). The entry's signature text is unchanged.

import (
	"os"
	"strings"
	"testing"
)

// TestMethodEntryNamesItsReceiverType: the fixture's methods, on an exported and on an unexported type, and a
// value receiver beside a pointer receiver, each named <Type>.<Method>; no bare method name is left.
func TestMethodEntryNamesItsReceiverType(t *testing.T) {
	t.Parallel()
	got := surfaceOf(t, fixture)
	for _, want := range []string{
		"fixture Handle.Name func func (h *Handle) Name() string\n",
		"fixture priv.Reach func func (p *priv) Reach() int\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("surface of %s:\n%s\nwant it to contain %q", fixture, got, want)
		}
	}
	for _, bare := range []string{"fixture Name func", "fixture Reach func"} {
		if strings.Contains(got, bare) {
			t.Errorf("surface of %s still names a method bare (%q):\n%s", fixture, bare, got)
		}
	}
	two := surfaceOf(t, pkgDir(t, "package p\n\ntype A struct{}\n\ntype B struct{}\n\nfunc (a A) Get() int { return 0 }\n\nfunc (b *B) Get() int { return 1 }\n"))
	for _, want := range []string{"p A.Get func func (a A) Get() int\n", "p B.Get func func (b *B) Get() int\n"} {
		if !strings.Contains(two, want) {
			t.Errorf("surface:\n%s\nwant it to contain %q (value and pointer receivers alike)", two, want)
		}
	}
	if !strings.Contains(two, "p A type type A struct {\n}\n") {
		t.Errorf("surface:\n%s\nwant the type's own entry unchanged", two)
	}
}

// TestDeltaNamesAMethodByItsType: a changed method signature reads `<pkg>.<Type>.<Method>` in the API Δ line, once
// removed and once added.
func TestDeltaNamesAMethodByItsType(t *testing.T) {
	t.Parallel()
	golden := write(t, surfaceOf(t, pkgDir(t, "package p\n\ntype T struct{}\n\nfunc (t *T) M() int { return 0 }\n")))
	after := pkgDir(t, "package p\n\ntype T struct{}\n\nfunc (t *T) M() string { return \"\" }\n")
	code, out := run(t, golden, after)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	const delta = "API Δ: +1/−1 (p.T.M, p.T.M)"
	if line := failedLine(t, out); !strings.HasPrefix(line, delta+";") {
		t.Errorf("failed: line %q does not start with %q", line, delta)
	}
	if b, _ := os.ReadFile(golden); !strings.Contains(string(b), "p T.M func") {
		t.Errorf("golden:\n%s\nwant the method entry named T.M", b)
	}
}
