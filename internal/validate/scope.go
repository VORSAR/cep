package validate

import (
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// scope is a rule's expr.Scope (design §6): slot i is one stream the rule reads. A plain or window rule reads one
// stream, slot 0. A pattern rule has a slot per step and per absence, in element order; inside an element's filter
// a bare name is that element's stream, and capture.attr names a capture's. In a pattern select only capture.attr
// resolves (bare is -1).
type scope struct {
	c        *checker
	slots    []*model.Stream
	captures map[string]int
	order    []string // capture names in declaration order, for suggestions
	bare     int
}

func (c *checker) scopeOf(r *model.Rule) *scope {
	s := &scope{c: c, captures: map[string]int{}, bare: 0}
	if r.Body.Kind != model.BodyPattern {
		s.slots = []*model.Stream{c.streams[sources(r)[0].Stream]}
		return s
	}
	for _, el := range r.Body.Pattern.Elements {
		for _, st := range el.Steps {
			if _, dup := s.captures[st.Name]; !dup {
				s.captures[st.Name] = len(s.slots)
				s.order = append(s.order, st.Name)
			}
			s.slots = append(s.slots, c.streams[st.Source.Stream])
		}
		if el.Kind == model.ElemAbsence {
			s.slots = append(s.slots, c.streams[el.Absence.Stream])
		}
	}
	return s
}

// at gives the same scope with bare names reading slot i (-1: none).
func (s *scope) at(i int) *scope {
	cp := *s
	cp.bare = i
	return &cp
}

func (s *scope) Attr(qualifier, name string) (int, int, event.Type, bool) {
	slot := s.bare
	if qualifier != "" {
		var ok bool
		if slot, ok = s.captures[qualifier]; !ok {
			return 0, 0, event.TInvalid, false
		}
	}
	if slot < 0 {
		return 0, 0, event.TInvalid, false
	}
	for i, a := range s.slots[slot].Attrs {
		if a.Name == name {
			return slot, i, a.Type, true
		}
	}
	return 0, 0, event.TInvalid, false
}

func (s *scope) Capture(name string) (int, bool) {
	slot, ok := s.captures[name]
	return slot, ok
}

// TableKey is a single-column key's type; a composite key never reaches Build (E-TBL-001 comes first).
func (s *scope) TableKey(table string) (event.Type, bool) {
	t := s.c.tables[table]
	if t == nil || len(t.PrimaryKey) != 1 {
		return event.TInvalid, false
	}
	for _, a := range t.Attrs {
		if a.Name == t.PrimaryKey[0] {
			return a.Type, true
		}
	}
	return event.TInvalid, false
}

// attr resolves an attribute reference to its declaration, or nil.
func (s *scope) attr(n *model.Expr) *model.Attr {
	slot, i, _, ok := s.Attr(n.Qualifier, n.Name)
	if !ok {
		return nil
	}
	return &s.slots[slot].Attrs[i]
}

// candidates are the names an unknown attribute could have meant, in declaration order.
func (s *scope) candidates(qualifier string) []string {
	if qualifier != "" {
		if slot, ok := s.captures[qualifier]; ok {
			var out []string
			for _, a := range s.slots[slot].Attrs {
				out = append(out, qualifier+"."+a.Name)
			}
			return out
		}
	} else if s.bare >= 0 {
		return attrNames(s.slots[s.bare].Attrs)
	}
	var out []string // an unknown qualifier, or a pattern select: every capture's qualified names
	for _, cname := range s.order {
		for _, a := range s.slots[s.captures[cname]].Attrs {
			out = append(out, cname+"."+a.Name)
		}
	}
	return out
}
