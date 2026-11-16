package validate

import (
	"slices"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// resolve is phase 3 (SPEC §4.1): each param's effective value is its override or its default, of its declared
// type, and every ${param} slot takes that value in the copy. A slot whose param does not resolve is poisoned, so
// no later phase reports it a second time.
func (c *checker) resolve() {
	c.defaults()
	c.overrides()
	for i := range c.app.Streams {
		c.resolveAnnotations(c.app.Streams[i].Annotations)
	}
	for _, r := range c.rules() {
		c.resolveRule(r)
	}
}

// defaults: each param's default is of its declared type and becomes its effective value; one that is not is bad.
func (c *checker) defaults() {
	for i := range c.app.Params {
		p := &c.app.Params[i]
		if !fits(p.Default, p.Type) {
			c.report("E-PARAM-001", p.Pos, "param %s is declared %s, but its default is a %s", p.Name, p.Type, kindName(
				p.Default))
			c.badParam[p.Name] = true
			continue
		}
		c.effective[p.Name] = as(p.Default, p.Type)
	}
}

// overrides: in name order, each deployment override names a declared param and is of its type, and then replaces
// the default; one that is not leaves the param bad, with no effective value. It runs after defaults.
func (c *checker) overrides() {
	names := make([]string, 0, len(c.opts.Overrides))
	for name := range c.opts.Overrides {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		v, p := c.opts.Overrides[name], c.params[name]
		switch {
		case p == nil:
			c.report("E-PARAM-001", model.Pos{}, "the deployment overrides %s, which no param declares", name)
		case !fits(v, p.Type):
			c.report("E-PARAM-001", p.Pos, "the deployment overrides %s (declared %s) with a %s", name, p.Type, kindName(v))
			c.badParam[name] = true
			delete(c.effective, name)
		default:
			c.effective[name] = as(v, p.Type)
		}
	}
}

// resolveRule resolves every duration slot of r, then every ${param} in its expressions.
func (c *checker) resolveRule(r *model.Rule) {
	for _, d := range durationSlots(r) {
		c.resolveDuration(d)
	}
	for _, root := range expressions(r) {
		c.resolveExpr(root, root)
	}
}

// fits says whether a literal can stand for a value of the declared type (an int literal fits a float): exactly
// the pairs model.LitValue converts.
func fits(l model.Literal, t event.Type) bool {
	_, ok := model.LitValue(l, t)
	return ok
}

// as is the literal as a value of type t: an int literal for a float param becomes a float.
func as(l model.Literal, t event.Type) model.Literal {
	if t == event.TFloat && l.Kind == model.LitInt {
		return model.Literal{Kind: model.LitFloat, Float: float64(l.Int)}
	}
	return l
}

func kindName(l model.Literal) string {
	if l.Kind == model.LitParam {
		return "${" + l.Param + "}"
	}
	return string(l.Kind) + " value"
}

// value is a param's effective value; an undeclared one is reported at the reference, a bad one was already
// reported at its declaration.
func (c *checker) value(name string, at model.Pos) (model.Literal, bool) {
	if v, ok := c.effective[name]; ok {
		return v, true
	}
	if !c.badParam[name] {
		c.report("E-PARAM-001", at, "${%s} is not a declared param", name)
	}
	return model.Literal{}, false
}

func (c *checker) resolveAnnotations(as []model.Annotation) {
	for i := range as {
		for j := range as[i].Items {
			it := &as[i].Items[j]
			c.resolveAnnotations(it.Nested)
			if it.Value.Kind == model.LitParam {
				if v, ok := c.value(it.Value.Param, it.ValuePos); ok {
					it.Value = v
				}
			}
		}
	}
}

// durationSlots are a rule's duration slots that may hold a param (grammar: duration_or_param).
func durationSlots(r *model.Rule) []*model.DurationOrParam {
	out := []*model.DurationOrParam{&r.Partition.TTL, &r.OutputFirstEvery}
	switch r.Body.Kind {
	case model.BodyPattern:
		p := &r.Body.Pattern
		out = append(out, &p.Within, &p.LateEvents.Allow)
		for i := range p.Elements {
			if p.Elements[i].Kind == model.ElemAbsence {
				out = append(out, &p.Elements[i].For)
			}
		}
	case model.BodyWindow:
		out = append(out, &r.Body.Window.Delay, &r.Body.Window.LateEvents.Allow)
	}
	return out
}

func (c *checker) resolveDuration(d *model.DurationOrParam) {
	if d.Param == "" {
		return
	}
	v, ok := c.value(d.Param, d.Pos)
	switch {
	case !ok:
		c.skipDur[d] = true
	case v.Kind != model.LitDuration:
		c.report("E-PARAM-001", d.Pos, "${%s} is a %s; this slot takes a duration", d.Param, kindName(v))
		c.skipDur[d] = true
	default:
		d.Ms, d.Param = v.Ms, ""
	}
}

// expressions are a rule's filters and select items, each a root a finding can poison.
func expressions(r *model.Rule) []*model.Expr {
	var out []*model.Expr
	for _, s := range sources(r) {
		if s.Filter.Kind != "" {
			out = append(out, &s.Filter)
		}
	}
	for i := range r.Select {
		out = append(out, &r.Select[i].Expr)
	}
	return out
}

// setOp says whether a text operator takes a keyword set (SPEC §5.3).
func setOp(op string) bool {
	for _, p := range []string{"has_any_", "has_all_", "contains_any_", "contains_all_"} {
		if strings.HasPrefix(op, p) {
			return true
		}
	}
	return false
}

func (c *checker) resolveExpr(n, root *model.Expr) {
	for i := range n.Args {
		c.resolveExpr(&n.Args[i], root)
	}
	switch {
	case n.Kind == model.ExprLiteral && n.Lit.Kind == model.LitParam:
		c.resolveLiteral(n, root)
	case n.Kind == model.ExprText && n.Operand.Param != "":
		c.resolveOperand(n, root)
	}
}

// resolveLiteral: a ${param} literal takes the param's value; one whose param does not resolve poisons root.
func (c *checker) resolveLiteral(n, root *model.Expr) {
	if v, ok := c.value(n.Lit.Param, n.Pos); ok {
		n.Lit = v
	} else {
		c.skipExpr[root] = true
	}
}

// resolveOperand: a text operator's ${param} operand takes the param's string, split at commas for a keyword-set
// operator; one that does not resolve, is not a string or holds an empty item poisons root.
func (c *checker) resolveOperand(n, root *model.Expr) {
	name, at := n.Operand.Param, n.Operand.Pos
	v, ok := c.value(name, at)
	if !ok {
		c.skipExpr[root] = true
		return
	}
	if v.Kind != model.LitString {
		c.report("E-PARAM-001", at, "${%s} is a %s; %s takes a string", name, kindName(v), n.Op)
		c.skipExpr[root] = true
		return
	}
	if !setOp(n.Op) {
		n.Operand.Strings, n.Operand.Param = []string{v.Str}, ""
		return
	}
	// Owner, 2026-11-04 (SPEC §4.1 as annotated): a keyword-set slot splits the string at commas.
	items := strings.Split(v.Str, ",")
	for i := range items {
		items[i] = strings.TrimSpace(items[i])
		if items[i] == "" {
			c.report("E-PARAM-001", at, "${%s} = %q: item %d of the comma-separated list is empty", name, v.Str, i+1)
			c.skipExpr[root] = true
			return
		}
	}
	n.Operand.Strings, n.Operand.Param = items, ""
}
