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

// unsupported finds the first construct outside RE2 in a pattern: a backreference or a lookaround. An escaped
// character is skipped, so `\(?=` is no lookahead.
func unsupported(p string) (string, int) {
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' && i+1 < len(p) {
			if w := backreference(p[i+1:]); w != "" {
				return w, i
			}
			i++
			continue
		}
		if w := lookaround(p[i:]); w != "" {
			return w, i
		}
	}
	return "", 0
}

// backreference names the backreference an escape's rest (the text after a backslash) starts with: a digit 1-9 or
// a named `k<`; "" when it is none.
func backreference(rest string) string {
	if c := rest[0]; c >= '1' && c <= '9' {
		return fmt.Sprintf("the backreference \\%c", c)
	}
	if strings.HasPrefix(rest, "k<") {
		return "a named backreference"
	}
	return ""
}

// lookaround names the lookahead or lookbehind s starts with; "" when it is none.
func lookaround(s string) string {
	switch {
	case strings.HasPrefix(s, "(?=") || strings.HasPrefix(s, "(?!"):
		return "a lookahead"
	case strings.HasPrefix(s, "(?<=") || strings.HasPrefix(s, "(?<!"):
		return "a lookbehind"
	}
	return ""
}
