package validate

import (
	"fmt"
	"strings"

	"github.com/vorsar/cep/internal/model"
)

// regex: a regex operand is RE2-class (SEMANTICS §5.2): no backreference and no lookaround. It runs on the resolved
// operand, so a pattern from a param is scanned too.
func (c *checker) regex(root *model.Expr) {
	walk(root, func(n *model.Expr) {
		if n.Kind != model.ExprText || !(strings.HasPrefix(n.Op, "matches_") || strings.HasPrefix(n.Op, "contains_match_")) {
			return
		}
		for _, pattern := range n.Operand.Strings {
			if what, offset := unsupported(pattern); what != "" {
				c.report("E-REGEX-001", n.Operand.Pos, "the pattern %q uses %s at offset %d: regex is linear-time (RE2) only",
					pattern, what, offset)
				return
			}
		}
	})
}

// unsupported finds the first construct outside RE2 in a pattern: a backreference or a lookaround.
func unsupported(p string) (string, int) {
	for i := 0; i < len(p); i++ {
		switch {
		case p[i] == '\\' && i+1 < len(p):
			if c := p[i+1]; c >= '1' && c <= '9' {
				return fmt.Sprintf("the backreference \\%c", c), i
			}
			if strings.HasPrefix(p[i+1:], "k<") {
				return "a named backreference", i
			}
			i++
		case strings.HasPrefix(p[i:], "(?=") || strings.HasPrefix(p[i:], "(?!"):
			return "a lookahead", i
		case strings.HasPrefix(p[i:], "(?<=") || strings.HasPrefix(p[i:], "(?<!"):
			return "a lookbehind", i
		}
	}
	return "", 0
}
