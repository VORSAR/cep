package validate

import (
	"slices"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

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
				"false, so the absence would fire; wrap it in coalesce(...) or test `is null` explicitly", strings.Join(offenders,
				", "))
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

// mayBeNull says whether n can evaluate to null (owner D4 of S3.2b-1; SEMANTICS §1.1), conservatively: an optional
// attribute (a capture's too) can; arithmetic, negation and a function call follow the function's own null rule in
// expr's registry (null in, null out by default); a literal, a param, a comparison, membership, a text operator, the
// boolean operators and `is [not] null` never can, being two-valued.
func (c *checker) mayBeNull(n *model.Expr, sc *scope, r *model.Rule) bool {
	switch n.Kind {
	case model.ExprLiteral:
		return n.Lit.Kind == model.LitNull
	case model.ExprAttr, model.ExprRaw:
		if n.Kind == model.ExprAttr && n.Name == event.InvisiblesAttr {
			return false // the engine's flag: a non-optional bool, declared or not (S3.2b's Q3)
		}
		slot, i, _, ok := sc.Attr(n.Qualifier, n.Name)
		if !ok {
			return true
		}
		st := sc.slots[slot]
		if !st.Attrs[i].Optional {
			return false
		}
		// A rule's own partition key is never null where the rule runs: a null key never reaches it (SEMANTICS §1.1,
		// §9; break-it L1 of S3.2b-1).
		return !slices.ContainsFunc(r.Partition.Keys, func(k model.PartitionKey) bool {
			return k.Stream == st.Name && k.Has(
				n.Name)
		})
	case model.ExprNeg, model.ExprArith:
		return slices.ContainsFunc(n.Args, func(x model.Expr) bool { return c.mayBeNull(&x, sc, r) })
	case model.ExprNot, model.ExprAnd, model.ExprOr, model.ExprCompare, model.ExprIsNull, model.ExprIn, model.ExprText:
		return false
	case model.ExprCall:
		name := n.Name
		if n.Namespace != "" {
			name = n.Namespace + ":" + n.Name
		}
		d, _ := c.reg.Lookup(name)
		args := n.Args
		switch d.Null {
		case expr.NullNever:
			return false
		case expr.NullIfAll:
			return !slices.ContainsFunc(args, func(x model.Expr) bool { return !c.mayBeNull(&x, sc, r) })
		case expr.NullIfBranch:
			args = args[1:]
		}
		return slices.ContainsFunc(args, func(x model.Expr) bool { return c.mayBeNull(&x, sc, r) })
	}
	return true
}

// exprText names a select item in a message: its dotted name when it is an attribute, else "the select item".
func exprText(n model.Expr) string {
	if isAttr(n) {
		return dotted(n)
	}
	return "the select item"
}
