package main

import (
	"slices"
	"testing"
)

// QA's break-it of S1.1c-2 (BK-S1): -cases counts a Go case as asserting when a Fatal, Error or FailNow on its T
// appears anywhere in its body, live or not. The Python strength rule (gate/runner/teeth.py) refuses a red helper
// after a return or under if False; the Go rule (design §4.2, DA-S12) has no live-code clause, so a pinned Go proof
// case hollowed by an if false or by a closure it never calls keeps its count and passes the pin. (A return at the
// top is the vet row's: unreachable code.)
func TestBreakitCasesDeadVerdicts(t *testing.T) {
	t.Parallel()
	const head = "package x\n\nimport \"testing\"\n\n"
	for _, c := range []struct{ name, src string }{
		{"the Fatal under if false", "func TestA(t *testing.T) {\n\tif false {\n\t\tt.Fatal(\"never\")\n\t}\n}\n"},
		{"the Fatal in a closure never called", "func TestA(t *testing.T) {\n\t_ = func() { t.Fatal(\"never\") }\n}\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cases, faults := Cases("x_test.go", head+c.src)
			if !slices.Equal(cases, []string{"TestA"}) {
				t.Fatalf("cases %q, want [TestA]", cases)
			}
			if !slices.Contains(faults, "TestA: "+noVerdict) {
				t.Errorf("faults %q: %s counted as a verdict", faults, c.name)
			}
		})
	}
}
