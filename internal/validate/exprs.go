package validate

import (
	"errors"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// expressions is phase 6: every filter and select item is typed by expr (E-TYPE-001, E-FN-001, E-NAME-001) after
// the checks that must come first (E-TBL-001, a callable's placement); then the null lints, the regex pre-scan and
// the select binding.
func (c *checker) expressions() {
	for _, r := range c.rules() {
		if c.poisoned[r] {
			continue
		}
		sc := c.scopeOf(r)
		slot := 0
		switch r.Body.Kind {
		case model.BodyPattern:
			for i := range r.Body.Pattern.Elements {
				el := &r.Body.Pattern.Elements[i]
				for j := range el.Steps {
					c.filter(&el.Steps[j].Source.Filter, sc.at(slot), false)
					slot++
				}
				if el.Kind == model.ElemAbsence {
					c.filter(&el.Absence.Filter, sc.at(slot), true)
					slot++
				}
			}
			c.selects(r, sc.at(-1))
		default:
			c.filter(&sources(r)[0].Filter, sc, false)
			c.selects(r, sc)
		}
	}
}

func (c *checker) filter(root *model.Expr, sc *scope, inAbsence bool) {
	if root.Kind == "" || c.skipExpr[root] || c.before(root, false, false) {
		return
	}
	e, ok := c.build(root, sc)
	if !ok {
		return
	}
	if e.Type() != event.TBool {
		c.report("E-TYPE-001", root.Pos, "a filter is a boolean expression; this one is %s", e.Type())
		return
	}
	c.nulls(root, sc, inAbsence)
	c.regex(root)
}

// before reports the findings that must precede typing; true when it found one, so Build is skipped.
func (c *checker) before(n *model.Expr, count, chain bool) bool {
	switch n.Kind {
	case model.ExprIn:
		if t := c.tables[n.Table]; t != nil && len(t.PrimaryKey) > 1 {
			c.report("E-TBL-001", n.TablePos, "%s has a %d-column primary key; `in` compares against a single-column key",
				t.Name, len(t.PrimaryKey))
			return true
		}
	case model.ExprText:
		if t := c.tables[n.Operand.Table]; t != nil && len(t.Attrs) > 1 {
			c.report("E-TBL-001", n.Operand.Pos, "%s has %d columns; %s takes a one-column table", t.Name, len(t.Attrs), n.Op)
			return true
		}
	case model.ExprCall:
		full := n.Name
		if n.Namespace != "" {
			full = n.Namespace + ":" + n.Name
		}
		if d, ok := c.reg.Lookup(full); ok {
			if d.Kind == expr.Aggregate && !count {
				c.report("E-FN-001", n.Pos, "%s() is an aggregate: it belongs in a window rule's select", full)
				return true
			}
			if d.Kind == expr.PatternOnly && !chain {
				c.report("E-FN-001", n.Pos, "%s() belongs in a pattern rule's select", full)
				return true
			}
		}
	}
	for i := range n.Args {
		if c.before(&n.Args[i], count, chain) {
			return true
		}
	}
	return false
}

// build types one root through expr, reporting its one error.
func (c *checker) build(root *model.Expr, sc *scope) (expr.Expr, bool) {
	e, err := c.reg.Build(*root, sc)
	if err == nil {
		return e, true
	}
	var xe *expr.Error
	if !errors.As(err, &xe) {
		c.report("E-TYPE-001", root.Pos, "%v", err)
		return nil, false
	}
	at := model.Pos{Line: xe.Line, Col: xe.Col}
	if xe.Code != "E-NAME-001" {
		c.report(xe.Code, at, "%s", xe.Msg)
		return nil, false
	}
	var candidates []string
	switch xe.What {
	case "attribute":
		qualifier, _, found := strings.Cut(xe.Name, ".")
		if !found && sc.bare < 0 {
			// A bare name in a pattern select: the capture that has it, if one does.
			for _, cname := range sc.order {
				if attrOf(sc.slots[sc.captures[cname]], xe.Name) != nil {
					c.report("E-NAME-001", at, "unknown attribute %s; a pattern select names a capture's: did you mean %s.%s?",
						xe.Name, cname, xe.Name)
					return nil, false
				}
			}
		}
		if !found {
			qualifier = ""
		}
		candidates = sc.candidates(qualifier)
	case "function":
		candidates = c.reg.Names()
	case "table":
		candidates = tableNames(c.app.Tables)
	case "capture":
		candidates = sc.order
	}
	c.unknown(at, xe.What, xe.Name, candidates)
	return nil, false
}

func isAttr(n model.Expr) bool { return n.Kind == model.ExprAttr || n.Kind == model.ExprRaw }

func dotted(n model.Expr) string {
	if n.Qualifier != "" {
		return n.Qualifier + "." + n.Name
	}
	return n.Name
}

func walk(n *model.Expr, f func(*model.Expr)) {
	f(n)
	for i := range n.Args {
		walk(&n.Args[i], f)
	}
}
