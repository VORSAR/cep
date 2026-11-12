package parser_test

// The register's validator stage, re-run (S1.1c-3 design note v2 §2.3; DA-S7, DQ-S9): each entry of
// tools/allowlists/fragments.json whose reason begins `validator: <code> <pkg>.<Test>/<case>` is wrapped in a fixed
// program for its kind, parsed, and validated; the validator must give the listed code. A validator entry is never
// looked up by its test's name alone.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/validate"
)

// validatorPrograms wraps a bound span of each kind in a whole program: a partition clause for a key, a filter for
// an expression. S carries the attributes the stand-ins name (k, j, ts, n).
var validatorPrograms = map[string]func(span string) string{
	"partkey": partitionProgram,
	"keylist": partitionProgram,
	"expr": func(span string) string {
		return "language cep/0.1;\n" +
			"stream S (k string, j string, n int, ts timestamp) time ts watermark bounded 1 min;\n" +
			"stream Out (k string, ts timestamp);\n" +
			"rule R intent 'x' from S[" + span + "] select k insert into Out;\n"
	},
}

func partitionProgram(span string) string {
	return "language cep/0.1;\n" +
		"stream S (k string, j string, n int, ts timestamp) time ts watermark bounded 1 min;\n" +
		"stream Out (k string, ts timestamp);\n" +
		"rule R intent 'x' partition by " + span + " ttl 10 min from S select k output first every 5 min insert into Out;\n"
}

func TestTheRegistersValidatorEntriesFailAtTheValidator(t *testing.T) {
	t.Parallel()
	cases, err := parser.FragmentValidatorCases("../..", "../../tools/allowlists/fragments.json")
	if err != nil {
		t.Fatalf("the register tools/allowlists/fragments.json: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("the register lists no validator entry; the design lists three (E-PART-005, E-PART-004, E-TYPE-001)")
	}
	runValidatorCases(t, cases)
}

// runValidatorCases wraps, parses and validates each case, wanting its listed code.
func runValidatorCases(t *testing.T, cases []parser.FragmentValidatorCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.File+": "+c.Span, func(t *testing.T) {
			t.Parallel()
			wrap, ok := validatorPrograms[c.Kind]
			if !ok {
				t.Fatalf("no fixed program for kind %s", c.Kind)
			}
			src := wrap(c.Bound)
			res, err := parser.Parse("fragment.cep", src)
			if err != nil {
				t.Fatalf("fails at grammar, listed at validator: %v\n%s", err, src)
			}
			_, diags := validate.Check(res.App, validate.Options{})
			codes := make([]string, 0, len(diags))
			for _, d := range diags {
				codes = append(codes, d.Code)
			}
			if !slices.Contains(codes, c.Code) {
				t.Errorf("validator gives %v, listed %s\n%s", codes, c.Code, src)
			}
		})
	}
}
