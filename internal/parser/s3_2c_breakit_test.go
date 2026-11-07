package parser_test

// S3.2c break-it (QA, short): the smallest int, -9223372036854775808, is an in-range int64 — the owner's overflow
// decision names it ("the smallest int divided by −1", docs/sprints/decisions.md, 2026-11-06) — yet a rule cannot
// write it: the lowering parses the digits without the sign, and 9223372036854775808 alone is outside int64, so the
// source is refused with E-TYPE-001 "integer 9223372036854775808 is out of range". A param default (a signed literal,
// no expression allowed) has no workaround in the source. Prefix: ox.

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/parser"
)

// TestBreakitTheSmallestIntIsAWritableLiteral: a param default of -9223372036854775808 lowers to math.MinInt64, and
// the same literal in a select lowers without error.
func TestBreakitTheSmallestIntIsAWritableLiteral(t *testing.T) {
	t.Run("param default", func(t *testing.T) {
		res, err := parser.Parse("ox-param.cep", "language cep/0.1;\nparam N int = -9223372036854775808;\n")
		if err != nil {
			t.Fatalf("Parse: %v; want the smallest int64, which is in range, accepted", err)
		}
		if got := res.App.Params[0].Default.Int; got != math.MinInt64 {
			t.Fatalf("default %d; want %d", got, int64(math.MinInt64))
		}
	})
	t.Run("select literal", func(t *testing.T) {
		src := "language cep/0.1;\n" +
			"stream S (n int, ts timestamp) time ts watermark bounded 1 min;\n" +
			"stream O (n int, ts timestamp);\n" +
			"rule R intent 'The smallest int.' from S select -9223372036854775808 as n insert into O;\n"
		if _, err := parser.Parse("ox-select.cep", src); err != nil {
			t.Fatalf("Parse: %v; want the smallest int64, which is in range, accepted", err)
		}
	})
}
