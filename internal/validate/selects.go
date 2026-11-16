package validate

import (
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

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
		case !a.Optional && c.mayBeNull(&item.Expr, sc, r):
			c.report("E-NULL-003", item.Pos,
				"%s can be null and %s.%s is not optional: wrap it in coalesce(...) or declare %s.%s optional",
				exprText(item.Expr), target.Name, name, target.Name, name)
		case window && aggregating(r.Body.Window.Kind):
			if attr := c.perEvent(r, &item.Expr); attr != "" {
				c.report("E-SEL-002", item.Pos,
					"%s is a per-event attribute; a %s window's select holds the partition key, aggregates or constants", attr,
					r.Body.Window.Kind)
			}
		}
		if name != "" {
			bound[name] = true
		}
	}
	if target == nil {
		return
	}
	if ts := attrOf(target, "ts"); ts != nil && target.Time.Kind == model.TimeNone && ts.Type != event.TTimestamp &&
		!c.badTs[target] {
		c.badTs[target] = true // once per stream, at the attribute itself (owner, S3.2b-1's review: D3's position)
		c.report("E-TYPE-001", ts.Pos, "%s.ts is filled with the output timestamp: declare it timestamp, not %s", target.Name,
			ts.Type)
	}
	for _, a := range target.Attrs {
		if broken {
			break
		}
		if a.Name != "ts" && !bound[a.Name] {
			c.report("E-SEL-001", r.SelectPos, "%s.%s is bound by no select item", target.Name, a.Name)
		}
	}
	if snk := c.connectorOf(target, "sink"); snk != nil && !snk.Changelog && window &&
		r.Body.Window.Kind == model.WindowSort {
		c.report("E-SINK-001", r.InsertIntoPos, "a sort window emits a changelog, and %s's sink (%s) takes appends only",
			target.Name, snk.Name)
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
			if k.Stream == r.Body.Window.Source.Stream && k.Has(n.Name) {
				return
			}
		}
		found = n.Name
	})
	return found
}
