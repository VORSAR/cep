package validate

import (
	"errors"
	"fmt"
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
			c.report("E-TBL-001", n.TablePos, "%s has a %d-column primary key; `in` compares against a single-column key", t.Name, len(t.PrimaryKey))
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
					c.report("E-NAME-001", at, "unknown attribute %s; a pattern select names a capture's: did you mean %s.%s?", xe.Name, cname, xe.Name)
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

// nulls: a comparison over a schema-optional attribute is false when it is null (SEMANTICS §1.1). Inside an
// absence that inverts the detection, so every comparison is E-NULL-001; elsewhere `!=`, `in` and `not in` are
// the lint E-NULL-002. An attribute the filter tests with `is [not] null` is handled; one wrapped in coalesce is no
// direct operand.
func (c *checker) nulls(root *model.Expr, sc *scope, inAbsence bool) {
	handled := map[*model.Attr]bool{}
	walk(root, func(n *model.Expr) {
		if n.Kind == model.ExprIsNull && isAttr(n.Args[0]) {
			if a := sc.attr(&n.Args[0]); a != nil {
				handled[a] = true
			}
		}
	})
	walk(root, func(n *model.Expr) {
		blind := n.Kind == model.ExprIn || (n.Kind == model.ExprCompare && (inAbsence || n.Op == "!="))
		if !blind {
			return
		}
		var offenders []string
		for i := range n.Args {
			if !isAttr(n.Args[i]) {
				continue
			}
			if a := sc.attr(&n.Args[i]); a != nil && a.Optional && !handled[a] {
				offenders = append(offenders, dotted(n.Args[i]))
			}
		}
		switch {
		case len(offenders) == 0:
		case inAbsence:
			c.report("E-NULL-001", n.Pos, "comparison over the optional attribute %s inside an absence: a null makes it "+
				"false, so the absence would fire; wrap it in coalesce(...) or test `is null` explicitly", strings.Join(offenders, ", "))
		default:
			c.report("E-NULL-002", n.Pos, "%s is optional, and `%s` is false when it is null; wrap it in coalesce(...) "+
				"or test `is null` explicitly", strings.Join(offenders, ", "), opName(n))
		}
	})
}

func opName(n *model.Expr) string {
	switch {
	case n.Kind == model.ExprIn && n.Negated:
		return "not in"
	case n.Kind == model.ExprIn:
		return "in"
	}
	return n.Op
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

// regex: a regex operand is RE2-class (SEMANTICS §5.2): no backreference and no lookaround. It runs on the resolved
// operand, so a pattern from a param is scanned too.
func (c *checker) regex(root *model.Expr) {
	walk(root, func(n *model.Expr) {
		if n.Kind != model.ExprText || !(strings.HasPrefix(n.Op, "matches_") || strings.HasPrefix(n.Op, "contains_match_")) {
			return
		}
		for _, pattern := range n.Operand.Strings {
			if what, offset := unsupported(pattern); what != "" {
				c.report("E-REGEX-001", n.Operand.Pos, "the pattern %q uses %s at offset %d: regex is linear-time (RE2) only", pattern, what, offset)
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

// selects binds each select item to a target attribute (SPEC §5.8): by its alias, or by its name when it is an
// attribute; `ts` is filled with the output timestamp; every other target attribute is bound exactly once and
// takes its item's type. An aggregating window's items are the partition key, aggregates or constants (E-SEL-002).
func (c *checker) selects(r *model.Rule, sc *scope) {
	window, pattern := r.Body.Kind == model.BodyWindow, r.Body.Kind == model.BodyPattern
	var target *model.Stream
	if !c.noTarget[r] {
		target = c.streams[r.InsertInto]
	}
	bound := map[string]bool{}
	broken := false // an item a finding already explains: what it meant to bind is unknown
	for i := range r.Select {
		item := &r.Select[i]
		name := item.As
		if name == "" && isAttr(item.Expr) {
			name = item.Expr.Name
		}
		var typ event.Type
		typed := false
		if !c.skipExpr[&item.Expr] && !c.before(&item.Expr, window, pattern) {
			if e, ok := c.build(&item.Expr, sc); ok {
				typ, typed = e.Type(), true
			}
		}
		if !typed {
			broken = true
			continue
		}
		if target == nil {
			continue
		}
		switch a := attrOf(target, name); {
		case name == "":
			c.report("E-SEL-001", item.Pos, "select item %d binds no attribute of %s: add `as <attribute>`", i+1, target.Name)
		case name == "ts":
			c.report("E-SEL-001", item.Pos, "%s.ts is filled with the output timestamp: do not select it", target.Name)
		case a == nil:
			c.report("E-SEL-001", item.Pos, "%s has no attribute %s", target.Name, name)
		case bound[name]:
			c.report("E-SEL-001", item.Pos, "%s.%s is bound twice", target.Name, name)
		case !assignable(typ, a.Type):
			c.report("E-TYPE-001", item.Pos, "select item %s is %s; %s.%s is %s", name, typ, target.Name, name, a.Type)
		case window && aggregating(r.Body.Window.Kind):
			if attr := c.perEvent(r, &item.Expr); attr != "" {
				c.report("E-SEL-002", item.Pos, "%s is a per-event attribute; a %s window's select holds the partition key, aggregates or constants", attr, r.Body.Window.Kind)
			}
		}
		if name != "" {
			bound[name] = true
		}
	}
	if target == nil {
		return
	}
	for _, a := range target.Attrs {
		if broken {
			break
		}
		if a.Name != "ts" && !bound[a.Name] {
			c.report("E-SEL-001", r.SelectPos, "%s.%s is bound by no select item", target.Name, a.Name)
		}
	}
	if snk := c.connectorOf(target, "sink"); snk != nil && !snk.Changelog && window && r.Body.Window.Kind == model.WindowSort {
		c.report("E-SINK-001", r.InsertIntoPos, "a sort window emits a changelog, and %s's sink (%s) takes appends only", target.Name, snk.Name)
	}
}

func assignable(from, to event.Type) bool {
	return from == to || (from == event.TInt && to == event.TFloat)
}

func aggregating(k model.WindowKind) bool { return k == model.WindowCron || k == model.WindowBatch }

// perEvent is the first attribute an aggregating window's select item reads that is not a partition key of the
// window's stream ("" if none).
func (c *checker) perEvent(r *model.Rule, item *model.Expr) string {
	found := ""
	walk(item, func(n *model.Expr) {
		if found != "" || !isAttr(*n) {
			return
		}
		for _, k := range r.Partition.Keys {
			if k.Stream == r.Body.Window.Source.Stream && k.Attr == n.Name {
				return
			}
		}
		found = n.Name
	})
	return found
}
