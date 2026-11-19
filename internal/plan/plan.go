// Package plan turns a checked model (validate.Check's result) into an app's executable description (ARCHITECTURE §5
// step 1): the streams, the tables, and per rule its routing and its operator chain. It is pure: no goroutines, no
// I/O, no clock reads. Design: docs/sprints/s3-2b-1-design.md (v2).
package plan

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/partition"
)

// App is one app's executable description.
type App struct {
	Streams []Stream // declaration order: the index is the stream id (SEMANTICS §2.2)
	Tables  []Table  // declaration order: the index is the table id
	Rules   []Rule   // declaration order
	// Expects is every rule's expect blocks as values, rule by rule in declaration order, block by block in written
	// order (design s3-3-2 §4). On the app, not on Rule, so runtime.FromPlan never carries them into a running app.
	Expects []Expect
}

// Stream is one declared stream.
type Stream struct {
	Schema    event.Schema // a source: its time attribute, or event.ArrivalTime; a derived stream: event.ArrivalTime
	Delay     int64        // d, ms: the bound; 0 for ascending and arrival
	IdleAfter int64        // ms; 0: none
	Derived   bool         // no time clause, produced by a rule
	// the stream's @source annotation, if any (design s3-3-1 §3.4; the facade never reads the model)
	Source *Connector
	Sink   *Connector // its @sink annotation, if any
}

// Connector is a stream's connector annotation as the facade binds it: the type, every other key with its value as
// the rule language writes it (a string without its quotes; an int, float or bool as written; a duration as "<ms> ms";
// a timestamp as its unix milliseconds), and the nested @map's type, if any.
type Connector struct {
	Type string
	Keys map[string]string
	Map  string
}

// Table is one declared table.
type Table struct {
	Name  string
	Attrs []event.Attr
	Key   []string
}

// Rule is one rule: its routing (the runtime's routing table) and its body.
type Rule struct {
	Name      string
	Target    int          // the stream id it inserts into (design s3-3-2 §5.4: the sandbox prints then-records by it)
	Inputs    []int        // the stream ids it reads; exactly one when OnArrival
	KeyAttrs  [][]int      // per input, the key attributes' indexes in part order; nil: key-less (the pseudo-key "")
	KeyTypes  []event.Type // per part, the one type of that position; a key-less rule: {TString}
	TTL       int64        // ms; 0 for a key-less rule
	OnArrival bool         // a plain body: sees each event as its stream accepts it (owner Q1 of S3.2b)
	NewBody   func(emit func(event.Event)) partition.Body
}

// Stats are one rule's emission counters on one shard. Every body NewBody returns implements Counted.
type Stats struct {
	Suppressed uint64 // outputs the rate limit suppressed (suppressed_outputs, SEMANTICS §8)
	Errored    uint64 // events an expression error dropped for this rule (SEMANTICS §10)
}

// Counted is a body that reports its Stats.
type Counted interface{ Stats() Stats }

// ErrNotBuilt is a body kind, or an expression, this release cannot run yet: patterns (S4.1), windows (S8), text
// operators (S7), count (S8), chain (S4.1).
var ErrNotBuilt = errors.New("plan: this body kind is not built yet")

// Build describes app, which MUST be validate.Check's result (no findings but lints).
func Build(app *model.App) (*App, error) {
	p := &App{}
	ids := make(map[string]int, len(app.Streams))
	for i, s := range app.Streams {
		st, err := stream(s)
		if err != nil {
			return nil, err
		}
		p.Streams = append(p.Streams, st)
		ids[s.Name] = i
	}
	for _, t := range app.Tables {
		p.Tables = append(p.Tables, Table{Name: t.Name, Attrs: attrs(t.Attrs), Key: t.PrimaryKey})
	}
	for i := range app.Rules {
		r, err := rule(app, p, ids, &app.Rules[i])
		if err != nil {
			return nil, fmt.Errorf("plan: rule %s: %w", app.Rules[i].Name, err)
		}
		p.Rules = append(p.Rules, r)
	}
	x, err := expects(app, p, ids)
	if err != nil {
		return nil, err
	}
	p.Expects = x
	return p, nil
}

func attrs(as []model.Attr) []event.Attr {
	out := make([]event.Attr, len(as))
	for i, a := range as {
		out[i] = event.Attr{Name: a.Name, Type: a.Type, Optional: a.Optional}
	}
	return out
}

// connector projects one @source or @sink annotation (a checked model: its type is a string, its params resolved).
func connector(an *model.Annotation) *Connector {
	c := &Connector{Keys: map[string]string{}}
	for _, it := range an.Items {
		switch {
		case len(it.Nested) > 0:
			for _, n := range it.Nested {
				if n.Name == "map" {
					for _, m := range n.Items {
						if m.Key == "type" {
							c.Map = litText(m.Value)
						}
					}
				}
			}
		case it.Key == "type":
			c.Type = litText(it.Value)
		default:
			c.Keys[it.Key] = litText(it.Value)
		}
	}
	return c
}

// litText is a literal as the rule language writes it, for a connector's keys.
func litText(l model.Literal) string {
	switch l.Kind {
	case model.LitString:
		return l.Str
	case model.LitInt:
		return strconv.FormatInt(l.Int, 10)
	case model.LitFloat:
		return strconv.FormatFloat(l.Float, 'g', -1, 64)
	case model.LitBool:
		return strconv.FormatBool(l.Bool)
	case model.LitDuration:
		return strconv.FormatInt(l.Ms, 10) + " ms"
	case model.LitTimestamp:
		return strconv.FormatInt(l.Ms, 10)
	}
	return ""
}

// stream is a stream's schema and clock facts; a derived stream's schema has no time attribute (its log line
// carries a top-level ts, D2 of S3.2a-1).
func stream(s model.Stream) (Stream, error) {
	timeAttr := event.ArrivalTime
	st := Stream{IdleAfter: s.Time.IdleAfter, Derived: s.Time.Kind == model.TimeNone}
	switch s.Time.Kind {
	case model.TimeBounded, model.TimeAscending:
		for i, a := range s.Attrs {
			if a.Name == s.Time.Attr {
				timeAttr = i
			}
		}
		if s.Time.Kind == model.TimeBounded {
			st.Delay = s.Time.Bound
		}
	}
	for i := range s.Annotations {
		switch an := &s.Annotations[i]; an.Name {
		case "source":
			st.Source = connector(an)
		case "sink":
			st.Sink = connector(an)
		}
	}
	sch, err := event.NewSchema(s.Name, attrs(s.Attrs), timeAttr)
	if err != nil {
		return Stream{}, fmt.Errorf("plan: stream %s: %w", s.Name, err)
	}
	st.Schema = sch
	return st, nil
}

// rule describes one rule; only a plain body is built in this release.
func rule(app *model.App, p *App, ids map[string]int, r *model.Rule) (Rule, error) {
	if r.Body.Kind != model.BodyPlain {
		return Rule{}, fmt.Errorf("a %s body: %w", r.Body.Kind, ErrNotBuilt)
	}
	src, ok := ids[r.Body.Source.Stream]
	if !ok {
		return Rule{}, fmt.Errorf("unknown stream %s (Build is given only a checked model)", r.Body.Source.Stream)
	}
	target, ok := ids[r.InsertInto]
	if !ok {
		return Rule{}, fmt.Errorf("unknown stream %s (Build is given only a checked model)", r.InsertInto)
	}
	sc := &scope{app: app, schema: p.Streams[src].Schema}
	spec := bodySpec{target: target, tsIdx: -1, every: r.OutputFirstEvery.Ms, scope: sc}
	if r.Body.Source.Filter.Kind != "" {
		spec.filter = r.Body.Source.Filter
	}
	out := p.Streams[target].Schema
	spec.nattrs = out.NumAttrs()
	for i := range out.NumAttrs() {
		a := out.Attr(i)
		if a.Name == "ts" {
			spec.tsIdx = i
			continue
		}
		item := selected(r.Select, a.Name)
		if item == nil {
			return Rule{}, fmt.Errorf("%s.%s is bound by no select item (Build is given only a checked model)", out.Stream(),
				a.Name)
		}
		spec.sel = append(spec.sel, projSpec{idx: i, node: *item, widen: a.Type == event.TFloat})
	}
	if _, err := spec.compile(); err != nil { // fail at deploy, not per event (review F1)
		return Rule{}, err
	}
	pr := Rule{Name: r.Name, Target: target, Inputs: []int{src}, KeyTypes: []event.Type{event.TString}, OnArrival: true}
	for _, k := range r.Partition.Keys {
		if k.Stream != r.Body.Source.Stream {
			continue
		}
		if pr.KeyAttrs != nil { // a second key of one stream is E-PART-004: never drop a declared key (break-it B1)
			return Rule{}, fmt.Errorf("stream %s is keyed twice (E-PART-004; Build is given only a checked model)", k.Stream)
		}
		attrs, types := make([]int, len(k.Attrs)), make([]event.Type, len(k.Attrs))
		for p, ka := range k.Attrs {
			attrs[p] = -1
			for i := range sc.schema.NumAttrs() {
				if a := sc.schema.Attr(i); a.Name == ka.Name {
					attrs[p], types[p] = i, a.Type
				}
			}
			// E-NAME-001, E-PART-006: Build is given only a checked model
			if attrs[p] < 0 || slices.Index(attrs[:p], attrs[p]) >= 0 {
				return Rule{}, fmt.Errorf("stream %s: key attribute %s is unknown or named twice (a checked model has neither)",
					k.Stream, ka.Name)
			}
		}
		pr.KeyAttrs, pr.KeyTypes, pr.TTL = [][]int{attrs}, types, r.Partition.TTL.Ms
	}
	pr.NewBody = func(emit func(event.Event)) partition.Body {
		b, err := spec.compile() // each body owns its trees: nothing is shared across shards
		if err != nil {
			panic(fmt.Sprintf("plan: rule %s: an expression Build accepted failed to compile again: %v", r.Name, err)) //nolint:forbidigo,lll // PERMANENT reason="an expression Build accepted compiles again the same way; a failure is a programming fault" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-5
		}
		b.emit = emit
		return b
	}
	return pr, nil
}

// selected is the select item bound to attribute name: by `as`, or by its own attribute name (validate's binding).
func selected(items []model.SelectItem, name string) *model.Expr {
	for i := range items {
		it := &items[i]
		bound := it.As
		if bound == "" && (it.Expr.Kind == model.ExprAttr || it.Expr.Kind == model.ExprRaw) {
			bound = it.Expr.Name
		}
		if bound == name {
			return &it.Expr
		}
	}
	return nil
}
