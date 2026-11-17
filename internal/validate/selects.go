package validate

import (
	"fmt"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// selects binds each select item to a target attribute (SPEC §5.8): by its alias, or by its name when it is an
// attribute; `ts` is filled with the output timestamp; every other target attribute is bound exactly once and
// takes its item's type. An aggregating window's items are the partition key, aggregates or constants (E-SEL-002).
func (c *checker) selects(r *model.Rule, sc *scope) {
	var target *model.Stream
	if !c.noTarget[r] {
		target = c.streams[r.InsertInto]
	}
	bound, broken := c.bindItems(r, sc, target)
	if target == nil {
		return
	}
	c.outputTs(target)
	if !broken {
		c.unboundAttrs(r, target, bound)
	}
	c.sortSink(r, target)
}

// bindItems types each select item and, when the rule has a target, binds it: at most one finding per item, its
// binding's first, else its value's. It returns the names bound and whether an item failed to type (broken: what it
// meant to bind is unknown).
func (c *checker) bindItems(r *model.Rule, sc *scope, target *model.Stream) (map[string]bool, bool) {
	bound := map[string]bool{}
	broken := false // an item a finding already explains: what it meant to bind is unknown
	for i := range r.Select {
		item := &r.Select[i]
		typ, typed := c.itemType(r, sc, item)
		if !typed {
			broken = true
			continue
		}
		if target == nil {
			continue
		}
		name := itemName(item)
		if msg := binding(target, name, i, bound); msg != "" {
			c.report("E-SEL-001", item.Pos, "%s", msg)
		} else {
			c.itemValue(r, sc, item, typ, target)
		}
		if name != "" {
			bound[name] = true
		}
	}
	return bound, broken
}

// itemType is a select item's type, after the checks that must precede typing; false when it has none (a finding,
// here or earlier, explains it).
func (c *checker) itemType(r *model.Rule, sc *scope, item *model.SelectItem) (event.Type, bool) {
	window, pattern := r.Body.Kind == model.BodyWindow, r.Body.Kind == model.BodyPattern
	if c.skipExpr[&item.Expr] || c.before(&item.Expr, window, pattern) {
		return event.TInvalid, false
	}
	e, ok := c.build(&item.Expr, sc)
	if !ok {
		return event.TInvalid, false
	}
	return e.Type(), true
}

// itemName is the target attribute a select item binds: its alias, or its name when it is an attribute; "" when
// it has neither.
func itemName(item *model.SelectItem) string {
	name := item.As
	if name == "" && isAttr(item.Expr) {
		name = item.Expr.Name
	}
	return name
}

// binding is E-SEL-001's text when an item named name (the i-th, from 0) cannot bind an attribute of target: no
// name, the output timestamp, no such attribute, or one already bound; "" when it binds.
func binding(target *model.Stream, name string, i int, bound map[string]bool) string {
	switch a := attrOf(target, name); {
	case name == "":
		return fmt.Sprintf("select item %d binds no attribute of %s: add `as <attribute>`", i+1, target.Name)
	case name == "ts":
		return fmt.Sprintf("%s.ts is filled with the output timestamp: do not select it", target.Name)
	case a == nil:
		return fmt.Sprintf("%s has no attribute %s", target.Name, name)
	case bound[name]:
		return fmt.Sprintf("%s.%s is bound twice", target.Name, name)
	}
	return ""
}

// itemValue checks a bound item's value against its attribute, the first finding only: its type (E-TYPE-001), a
// null into a non-optional attribute (E-NULL-003), a per-event attribute in an aggregating window (E-SEL-002).
func (c *checker) itemValue(r *model.Rule, sc *scope, item *model.SelectItem, typ event.Type, target *model.Stream) {
	window := r.Body.Kind == model.BodyWindow
	name := itemName(item)
	a := attrOf(target, name)
	switch {
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
}

// outputTs is E-TYPE-001 for a target without event time whose ts is not a timestamp, once per stream across rules.
func (c *checker) outputTs(target *model.Stream) {
	if ts := attrOf(target, "ts"); ts != nil && target.Time.Kind == model.TimeNone && ts.Type != event.TTimestamp &&
		!c.badTs[target] {
		c.badTs[target] = true // once per stream, at the attribute itself (owner, S3.2b-1's review: D3's position)
		c.report("E-TYPE-001", ts.Pos, "%s.ts is filled with the output timestamp: declare it timestamp, not %s", target.Name,
			ts.Type)
	}
}

// unboundAttrs is E-SEL-001 for each attribute of target, other than ts, that no select item binds.
func (c *checker) unboundAttrs(r *model.Rule, target *model.Stream, bound map[string]bool) {
	for _, a := range target.Attrs {
		if a.Name != "ts" && !bound[a.Name] {
			c.report("E-SEL-001", r.SelectPos, "%s.%s is bound by no select item", target.Name, a.Name)
		}
	}
}

// sortSink is E-SINK-001 for a sort window, which emits a changelog, into a sink that takes appends only.
func (c *checker) sortSink(r *model.Rule, target *model.Stream) {
	window := r.Body.Kind == model.BodyWindow
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
