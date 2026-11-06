// Package validate is the static checker (S2.2b; design: docs/sprints/s2-2b-design.md): every [static] code of
// docs/language/errors.md that a model can show, each at its source position with the catalogue's fix text.
//
// Check runs in phases — pragma, names and connectors, params, graph and time, partition and bounds, expressions
// and select — and a finding poisons what it explains, so later phases skip it: one defect, one finding. The four
// [static] codes that need a token's text (E-TYPE-001's literals, E-DUR-002, E-STR-002, E-WIN-004) are raised by
// the parser's lowering instead (parser.LoweringCodes). Check never writes the App it is given: it resolves params
// in a deep copy, which it returns when no finding refuses the deploy.
package validate

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"

	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// Diagnostic is one finding: a catalogue code at a source position, with its message and the catalogue's fix.
type Diagnostic struct {
	Code      string // "E-TTL-002"
	Line, Col int    // runes, 1-based; 0, 0 for a model built through the Go API
	Msg       string // the names, numbers and nearest valid name errors.md says the message carries
	Fix       string // errors.md's fix text (docsync (b) pins catalog.go to it)
	Lint      bool   // E-NULL-002 only: reported, never refuses a deploy
}

// Options are a deployment's inputs to Check.
type Options struct {
	Overrides map[string]model.Literal // param overrides, already parsed by the caller
}

// Check validates app and resolves its params. It reports one finding per defect, ordered by line, column, code
// and message. The returned App is a deep copy of app with every ${param} slot holding its effective value; it is
// nil when any finding is not a lint.
func Check(app *model.App, opts Options) (*model.App, []Diagnostic) {
	return check(app, opts, builtin)
}

func check(app *model.App, opts Options, conn connectors) (*model.App, []Diagnostic) {
	cp := deepCopy(app)
	c := &checker{
		app: cp, opts: opts, conn: conn, reg: expr.Builtins(),
		streams: map[string]*model.Stream{}, tables: map[string]*model.Table{}, params: map[string]*model.Param{},
		effective: map[string]model.Literal{}, badParam: map[string]bool{},
		poisoned: map[*model.Rule]bool{}, noTarget: map[*model.Rule]bool{}, badTs: map[*model.Stream]bool{}, badKey: map[*model.PartitionKey]bool{},
		skipExpr: map[*model.Expr]bool{}, skipDur: map[*model.DurationOrParam]bool{},
	}
	c.run()
	slices.SortStableFunc(c.diags, func(a, b Diagnostic) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col), cmp.Compare(a.Code, b.Code), cmp.Compare(a.Msg, b.Msg))
	})
	for _, d := range c.diags {
		if !d.Lint {
			return nil, c.diags
		}
	}
	return cp, c.diags
}

type checker struct {
	app   *model.App
	opts  Options
	conn  connectors
	reg   *expr.Registry
	diags []Diagnostic

	streams map[string]*model.Stream
	tables  map[string]*model.Table
	params  map[string]*model.Param

	effective map[string]model.Literal // each param's value after overrides
	badParam  map[string]bool          // a param whose default or override is already reported

	poisoned map[*model.Rule]bool            // its from, a step or an absence names an unknown stream
	noTarget map[*model.Rule]bool            // its insert-into stream is unknown
	badTs    map[*model.Stream]bool          // a derived stream whose ts was reported (owner D3 of S3.2b-1)
	badKey   map[*model.PartitionKey]bool    // already reported: an unknown stream or attribute
	skipExpr map[*model.Expr]bool            // a filter or select item a finding already explains
	skipDur  map[*model.DurationOrParam]bool // a duration slot whose param did not resolve
}

func (c *checker) run() {
	if c.app.Language != "cep/0.1" {
		// SPEC §3.1: a later version is refused, never half-interpreted, so nothing else is reported.
		c.report("E-PRAG-002", c.app.LanguagePos, "language %s is not supported: this engine reads cep/0.1 only", c.app.Language)
		return
	}
	c.index()
	c.names()
	c.resolve()
	c.graph()
	c.bounds()
	c.expressions()
}

func (c *checker) index() {
	for i := range c.app.Streams {
		if s := &c.app.Streams[i]; c.streams[s.Name] == nil {
			c.streams[s.Name] = s
		}
	}
	for i := range c.app.Tables {
		if t := &c.app.Tables[i]; c.tables[t.Name] == nil {
			c.tables[t.Name] = t
		}
	}
	for i := range c.app.Params {
		if p := &c.app.Params[i]; c.params[p.Name] == nil {
			c.params[p.Name] = p
		}
	}
}

func (c *checker) report(code string, at model.Pos, format string, args ...any) {
	c.diags = append(c.diags, Diagnostic{
		Code: code, Line: at.Line, Col: at.Col, Msg: fmt.Sprintf(format, args...),
		Fix: catalog[code].Fix, Lint: code == "E-NULL-002",
	})
}

// unknown reports E-NAME-001 for name, suggesting the nearest of candidates (in their given order).
func (c *checker) unknown(at model.Pos, what, name string, candidates []string) {
	msg := "unknown " + what + " " + name
	if near := nearest(name, candidates); near != "" {
		msg += "; did you mean " + near + "?"
	}
	c.report("E-NAME-001", at, "%s", msg)
}

// deepCopy copies every struct and slice of app, so resolving params in the copy never writes the original.
func deepCopy(app *model.App) *model.App {
	out := reflect.New(reflect.TypeFor[model.App]()).Elem()
	copyValue(out, reflect.ValueOf(*app))
	cp := out.Interface().(model.App)
	return &cp
}

func copyValue(dst, src reflect.Value) {
	switch src.Kind() {
	case reflect.Struct:
		for i := range src.NumField() {
			copyValue(dst.Field(i), src.Field(i))
		}
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		s := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
		for i := range src.Len() {
			copyValue(s.Index(i), src.Index(i))
		}
		dst.Set(s)
	default:
		dst.Set(src)
	}
}

// rules gives a pointer to each rule, in order.
func (c *checker) rules() []*model.Rule {
	out := make([]*model.Rule, len(c.app.Rules))
	for i := range c.app.Rules {
		out[i] = &c.app.Rules[i]
	}
	return out
}

// sources gives a rule's stream reads in source order: the from source, or each step and absence of its pattern.
func sources(r *model.Rule) []*model.Source {
	switch r.Body.Kind {
	case model.BodyPattern:
		var out []*model.Source
		for i := range r.Body.Pattern.Elements {
			el := &r.Body.Pattern.Elements[i]
			for j := range el.Steps {
				out = append(out, &el.Steps[j].Source)
			}
			if el.Kind == model.ElemAbsence {
				out = append(out, &el.Absence)
			}
		}
		return out
	case model.BodyWindow:
		return []*model.Source{&r.Body.Window.Source}
	}
	return []*model.Source{&r.Body.Source}
}

// read lists the distinct streams a rule's body reads, in source order.
func read(r *model.Rule) []string {
	var out []string
	for _, s := range sources(r) {
		if !slices.Contains(out, s.Stream) {
			out = append(out, s.Stream)
		}
	}
	return out
}

func hasAnnotation(s *model.Stream, name string) bool {
	return slices.ContainsFunc(s.Annotations, func(a model.Annotation) bool { return a.Name == name })
}

func attrOf(s *model.Stream, name string) *model.Attr {
	for i := range s.Attrs {
		if s.Attrs[i].Name == name {
			return &s.Attrs[i]
		}
	}
	return nil
}

func attrNames(attrs []model.Attr) []string {
	out := make([]string, len(attrs))
	for i, a := range attrs {
		out[i] = a.Name
	}
	return out
}

// declared lists the names of a declaration list, in order.
func streamNames(ss []model.Stream) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name
	}
	return out
}

func tableNames(ts []model.Table) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}

// present says whether an optional duration slot was written: the model holds 0 ms for an absent one, so a
// written zero is told apart by its position (a model from the Go API has none; known issue S2.2b-2).
func present(d model.DurationOrParam) bool {
	return d.Ms != 0 || d.Param != "" || d.Pos != (model.Pos{})
}
