package validate

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// expects is E-EXP-002 (design s3-3-2 §3.1): a mistake inside an expect block, at its own position, found before any
// block runs. It runs last, over every rule whose body and target resolved; it reports no other code.
func (c *checker) expects() {
	for _, r := range c.rules() {
		if c.poisoned[r] || c.noTarget[r] {
			continue
		}
		for i := range r.Expects {
			c.expectBlock(r, i+1, &r.Expects[i])
		}
	}
}

// expectBlock checks one block's sections in written order: a section whose name is unknown (X1), not the rule's
// (X7), stamped on arrival (X8) or given twice (X9) is not checked further.
func (c *checker) expectBlock(r *model.Rule, n int, b *model.Expect) {
	at := fmt.Sprintf("expect block %d of rule %s: ", n, r.Name)
	given := map[string]bool{}
	for gi := range b.Givens {
		g := &b.Givens[gi]
		if g.Table {
			t := c.tables[g.Name]
			switch {
			case t == nil:
				c.mistake(g.NamePos, at+c.unknownText("table", g.Name, tableNames(c.app.Tables)))
			case !readsTable(r, g.Name):
				c.mistake(g.NamePos, at+fmt.Sprintf("rule %s does not read table %s", r.Name, g.Name))
			default:
				for ri := range g.Records {
					c.givenRecord(at, g.Name, t.Attrs, "", &g.Records[ri])
				}
			}
			continue
		}
		s := c.streams[g.Name]
		switch {
		case s == nil:
			c.mistake(g.NamePos, at+c.unknownText("stream", g.Name, streamNames(c.app.Streams)))
		case !slices.Contains(read(r), g.Name):
			c.mistake(g.NamePos, at+fmt.Sprintf("rule %s does not read %s", r.Name, g.Name))
		case s.Time.Kind == model.TimeArrival:
			c.mistake(g.NamePos, at+fmt.Sprintf("%s is stamped on arrival; a block cannot give it events", g.Name))
		case given[g.Name]:
			c.mistake(g.NamePos, at+fmt.Sprintf("%s is given twice; give each stream one list", g.Name))
		default:
			given[g.Name] = true
			for ri := range g.Records {
				c.givenRecord(at, g.Name, s.Attrs, s.Time.Attr, &g.Records[ri])
			}
		}
	}
	th := &b.Then
	s := c.streams[th.Name]
	switch {
	case s == nil:
		c.mistake(th.NamePos, at+c.unknownText("stream", th.Name, streamNames(c.app.Streams)))
	case th.Name != r.InsertInto:
		c.mistake(th.NamePos, at+fmt.Sprintf("rule %s inserts into %s, not %s", r.Name, r.InsertInto, th.Name))
	default:
		for ri := range th.Records {
			c.thenRecord(at, th.Name, s.Attrs, &th.Records[ri])
		}
	}
}

// givenRecord checks one given or given-table record against its stream's or table's attributes (X2, X10, X6, X3,
// X4); timeAttr is the stream's time attribute ("" for a table), which is required whether or not it is optional.
func (c *checker) givenRecord(at, owner string, attrs []model.Attr, timeAttr string, rec *model.Record) {
	seen := map[string]bool{}
	for fi := range rec.Fields {
		f := &rec.Fields[fi]
		a := attrNamed(attrs, f.Name)
		switch {
		case a == nil:
			c.mistake(f.Pos, at+c.unknownText("attribute", f.Name, attrNames(attrs), owner+" has no"))
			continue
		case timeAttr != "" && f.Name == event.InvisiblesAttr:
			c.mistake(f.Pos, at+fmt.Sprintf("%s.%s is computed by the engine; a block cannot give it", owner, f.Name))
			continue
		case seen[f.Name]:
			c.mistake(f.Pos, at+fmt.Sprintf("field %s is written twice in one record", f.Name))
			continue
		}
		seen[f.Name] = true
		required := !a.Optional || a.Name == timeAttr
		switch {
		case f.Value.Kind == model.LitNull && required:
			c.mistake(f.ValuePos, at+fmt.Sprintf("%s.%s is required; the block gives it null", owner, f.Name))
		case f.Value.Kind != model.LitNull:
			c.blockValue(at, owner, a, f)
		}
	}
	for _, a := range attrs {
		if seen[a.Name] || (a.Optional && a.Name != timeAttr) || (timeAttr != "" && a.Name == event.InvisiblesAttr) {
			continue
		}
		c.mistake(rec.Pos, at+fmt.Sprintf("%s.%s is required; the block leaves it out", owner, a.Name))
	}
}

// thenRecord checks one then-record against the target stream's attributes (X2, X6, X3, X5); an omitted attribute is
// unasserted (SPEC §6.8).
func (c *checker) thenRecord(at, owner string, attrs []model.Attr, rec *model.Record) {
	seen := map[string]bool{}
	for fi := range rec.Fields {
		f := &rec.Fields[fi]
		a := attrNamed(attrs, f.Name)
		switch {
		case a == nil:
			c.mistake(f.Pos, at+c.unknownText("attribute", f.Name, attrNames(attrs), owner+" has no"))
			continue
		case seen[f.Name]:
			c.mistake(f.Pos, at+fmt.Sprintf("field %s is written twice in one record", f.Name))
			continue
		}
		seen[f.Name] = true
		switch {
		case f.Value.Kind == model.LitNull && !a.Optional:
			c.mistake(f.ValuePos, at+fmt.Sprintf("%s.%s is never null; the block expects null", owner, f.Name))
		case f.Value.Kind != model.LitNull:
			c.blockValue(at, owner, a, f)
		}
	}
}

func (c *checker) mistake(p model.Pos, msg string) { c.report("E-EXP-002", p, "%s", msg) }

// blockValue is X3 for one written value: it fits the field's type — an object field takes a string holding one JSON
// object or array, as the Go boundary takes it (owner, 2026-11-10, S3.3-2's break-it decision 1).
func (c *checker) blockValue(at, owner string, a *model.Attr, f *model.Field) {
	switch {
	case a.Type == event.TObject && f.Value.Kind == model.LitString:
		if !objectText(f.Value.Str) {
			c.mistake(f.ValuePos, at+fmt.Sprintf("%s.%s is object; the block's string is not one JSON object or array", owner, f.Name))
		}
	case !fits(f.Value, a.Type):
		c.mistake(f.ValuePos, at+fmt.Sprintf("%s.%s is %s; the block gives %s", owner, f.Name, a.Type, article(kindName(f.Value))))
	}
}

// objectText says s is one JSON object or array an object value can hold: valid, an object or an array at the top,
// nested within the log's limit once in its stored form (event.CleanObject, event.ExactObject).
func objectText(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" || (t[0] != '{' && t[0] != '[') || !json.Valid([]byte(t)) {
		return false
	}
	return event.ExactObject(event.CleanObject([]byte(t)))
}

// unknownText is "unknown <what> <name>" — or, with a prefix, "<prefix> <what> <name>" — with the nearest candidate.
func (c *checker) unknownText(what, name string, candidates []string, prefix ...string) string {
	msg := "unknown " + what + " " + name
	if len(prefix) > 0 {
		msg = prefix[0] + " " + what + " " + name
	}
	if near := nearest(name, candidates); near != "" {
		msg += "; did you mean " + near + "?"
	}
	return msg
}

// readsTable: one of the rule's filter or select expressions, at any depth, is an `in` against t or a text operator
// whose operand is t (design s3-3-2 §3.1, Definitions).
func readsTable(r *model.Rule, t string) bool {
	var walk func(n *model.Expr) bool
	walk = func(n *model.Expr) bool {
		if (n.Kind == model.ExprIn && n.Table == t) || (n.Kind == model.ExprText && n.Operand.Table == t) {
			return true
		}
		for i := range n.Args {
			if walk(&n.Args[i]) {
				return true
			}
		}
		return false
	}
	return slices.ContainsFunc(expressions(r), walk)
}

func attrNamed(attrs []model.Attr, name string) *model.Attr {
	for i := range attrs {
		if attrs[i].Name == name {
			return &attrs[i]
		}
	}
	return nil
}

// article puts "a" or "an" before a kind's name ("an int value", "a string value").
func article(s string) string {
	if s != "" && slices.Contains([]byte("aeiou"), s[0]) {
		return "an " + s
	}
	return "a " + s
}
