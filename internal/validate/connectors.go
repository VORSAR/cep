package validate

import (
	"slices"

	"github.com/vorsar/cep/internal/model"
)

// connector is one @source or @sink type of the closed annotation vocabulary (ADR-0008 decision 1: "a closed table
// in internal/validate/connectors.go; unknown names = E-NAME-001").
type connector struct {
	Name       string
	Keys       []string // every key but `type`
	Punctuates bool     // @source: it marks batch boundaries, so batch() may read it (E-WIN-002)
	Changelog  bool     // @sink: it accepts a changelog, so a sort window may feed it (E-SINK-001)
}

type connectors struct {
	Sources []connector
	Sinks   []connector
	Maps    []string // @map(type = …)
}

// builtin is ADR-0008's Phase 1 slice. Every source marks batches (owner, 2026-11-04: E-WIN-002 is unreachable in
// v0.1). The keys are provisional until the connectors are built: each is named by ADR-0008 or the corpus.
var builtin = connectors{
	Sources: []connector{
		{Name: "http", Keys: []string{"port", "path"}, Punctuates: true}, // §4: one listener; a POST body is one batch
		{Name: "stdin", Punctuates: true},                                // §4: one batch per --batch-lines or end of input
		{Name: "inmemory", Punctuates: true},                             // §4: the Go API's feed, its batch id from the caller
	},
	Sinks: []connector{
		{Name: "http", Keys: []string{"url", "template"}},        // FA8: a webhook, JSON, {{attr}} templates; example 01
		{Name: "email", Keys: []string{"to", "subject", "body"}}, // notes: SMTP, {{attr}} templates in subject and body
		{Name: "log", Changelog: true},                           // notes: prints the canonical JSON of each change
	},
	Maps: []string{"json", "text"},
}

func connectorNames(cs []connector) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func find(cs []connector, name string) *connector {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

// typeOf is an annotation's `type` value when it is written as a string literal ("" otherwise).
func typeOf(a *model.Annotation) (string, *model.AnnotationItem) {
	for i := range a.Items {
		if it := &a.Items[i]; it.Key == "type" && it.Value.Kind == model.LitString {
			return it.Value.Str, it
		}
	}
	return "", nil
}

// connectorOf is the stream's @source or @sink connector, when its type is a known string.
func (c *checker) connectorOf(s *model.Stream, annotation string) *connector {
	table := c.conn.Sources
	if annotation == "sink" {
		table = c.conn.Sinks
	}
	for i := range s.Annotations {
		if a := &s.Annotations[i]; a.Name == annotation {
			name, _ := typeOf(a)
			return find(table, name)
		}
	}
	return nil
}

// annotations checks a stream's annotations against the closed vocabulary (E-NAME-001 with the nearest name).
func (c *checker) annotations(s *model.Stream) {
	for i := range s.Annotations {
		a := &s.Annotations[i]
		var table []connector
		switch a.Name {
		case "source":
			table = c.conn.Sources
		case "sink":
			table = c.conn.Sinks
		default:
			c.unknown(a.Pos, "annotation", "@"+a.Name, []string{"@source", "@sink"})
			continue
		}
		name, typeItem := typeOf(a)
		kind := find(table, name)
		if typeItem != nil && kind == nil {
			c.unknown(typeItem.ValuePos, "@"+a.Name+" type", "'"+name+"'", quoted(connectorNames(table)))
		}
		for j := range a.Items {
			it := &a.Items[j]
			switch {
			case len(it.Nested) > 0:
				c.mapAnnotation(&it.Nested[0])
			case it.Key == "type":
			case kind != nil && !slices.Contains(kind.Keys, it.Key):
				c.unknown(it.Pos, "key of @"+a.Name+"(type = '"+kind.Name+"')", it.Key, append([]string{"type"}, kind.Keys...))
			}
		}
	}
}

func (c *checker) mapAnnotation(m *model.Annotation) {
	if m.Name != "map" {
		c.unknown(m.Pos, "nested annotation", "@"+m.Name, []string{"@map"})
		return
	}
	for _, it := range m.Items {
		switch {
		case len(it.Nested) > 0:
			c.unknown(it.Pos, "nested annotation", "@"+it.Nested[0].Name, nil)
		case it.Key != "type":
			c.unknown(it.Pos, "key of @map", it.Key, []string{"type"})
		case it.Value.Kind == model.LitString && !slices.Contains(c.conn.Maps, it.Value.Str):
			c.unknown(it.ValuePos, "@map type", "'"+it.Value.Str+"'", quoted(c.conn.Maps))
		}
	}
}

func quoted(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "'" + n + "'"
	}
	return out
}
