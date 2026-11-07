package parser_test

// S3.2c's fix round (owner, 2026-11-07, S3.2c's review and break-it, decision 3; docs/sprints/decisions.md): "A minus
// sign written directly before an integer literal belongs to it where the number fits only with its sign — a param
// default and an expression alike; the grammar is unchanged." These are the guards around QA's
// TestBreakitTheSmallestIntIsAWritableLiteral: nothing else the lowering refused becomes writable, and a negated
// literal that fits on its own keeps its form. Prefix: mi.

import (
	"math"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
)

func miRule(sel string) string {
	return "language cep/0.1;\n" +
		"stream S (n int, ts timestamp) time ts watermark bounded 1 min;\n" +
		"stream O (n int, ts timestamp);\n" +
		"rule R intent 'A literal.' from S select " + sel + " as n insert into O;\n"
}

// TestTheSmallestIntIsWrittenWithItsSignOnly: one past the smallest int, the smallest int's digits in brackets or
// after a binary minus, are still E-TYPE-001; the smallest int lowers to one literal; `-5` keeps its form.
func TestTheSmallestIntIsWrittenWithItsSignOnly(t *testing.T) {
	refused := map[string]string{
		"param one past":         "language cep/0.1;\nparam N int = -9223372036854775809;\n",
		"select one past":        miRule("-9223372036854775809"),
		"select in brackets":     miRule("-(9223372036854775808)"),
		"select binary minus":    miRule("0 - 9223372036854775808"),
		"select the bare digits": miRule("9223372036854775808"),
	}
	for name, src := range refused {
		t.Run(name, func(t *testing.T) {
			if _, err := parser.Parse("mi.cep", src); err == nil || !strings.Contains(err.Error(), "E-TYPE-001") {
				t.Fatalf("Parse: %v; want E-TYPE-001 (out of range)", err)
			}
		})
	}
	t.Run("the smallest int is one literal", func(t *testing.T) {
		res, err := parser.Parse("mi.cep", miRule("-9223372036854775808"))
		if err != nil {
			t.Fatal(err)
		}
		got := res.App.Rules[0].Select[0].Expr
		if got.Kind != model.ExprLiteral || got.Lit.Kind != model.LitInt || got.Lit.Int != math.MinInt64 {
			t.Fatalf("select item %+v; want the int literal %d", got, int64(math.MinInt64))
		}
	})
	t.Run("a negated literal that fits keeps its form", func(t *testing.T) {
		res, err := parser.Parse("mi.cep", miRule("-5"))
		if err != nil {
			t.Fatal(err)
		}
		if got := res.App.Rules[0].Select[0].Expr; got.Kind != model.ExprNeg {
			t.Fatalf("select item %+v; want unary minus over 5, as before", got)
		}
	})
}
