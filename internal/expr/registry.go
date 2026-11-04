package expr

import (
	"fmt"
	"sort"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// Kind is a callable's kind (ADR-0011 §3): where it may appear.
type Kind string

// The kinds of SPEC Appendix F.
const (
	Scalar      Kind = "scalar"
	Aggregate   Kind = "aggregate"    // window rules only (count)
	PatternOnly Kind = "pattern-only" // chain
)

// Descriptor is one callable: its name, kind, arity, and how a call to it is typed and built.
type Descriptor struct {
	Name    string // "coalesce" or "math:round"
	Kind    Kind
	MinArgs int
	MaxArgs int
	// Factory types the built arguments and gives the node; it reports E-TYPE-001 or E-FN-001 (nil for the
	// kinds that are typed here and built in a later sprint).
	Factory func(args []Expr, call model.Expr) (Expr, error)
}

func (d *Descriptor) arity() string {
	switch {
	case d.MinArgs == d.MaxArgs:
		return fmt.Sprintf("%d argument(s)", d.MinArgs)
	case d.MaxArgs == maxArgs:
		return fmt.Sprintf("at least %d argument(s)", d.MinArgs)
	}
	return fmt.Sprintf("%d to %d arguments", d.MinArgs, d.MaxArgs)
}

const maxArgs = 1 << 20

// Registry maps every callable name to its descriptor (ADR-0011 §3): one registration API for the built-ins
// and for extensions to come. Its content is SPEC Appendix F, which docsync (c) pins.
type Registry struct {
	descriptors map[string]*Descriptor
}

// NewRegistry gives an empty registry.
func NewRegistry() *Registry { return &Registry{descriptors: map[string]*Descriptor{}} }

// Register adds d; a second registration of a name is an error.
func (r *Registry) Register(d Descriptor) error {
	if _, dup := r.descriptors[d.Name]; dup {
		return fmt.Errorf("expr: %s is already registered", d.Name)
	}
	copy := d
	r.descriptors[d.Name] = &copy
	return nil
}

// Names lists the registered callables, sorted.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.descriptors))
	for name := range r.descriptors {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Lookup gives a callable's descriptor.
func (r *Registry) Lookup(name string) (Descriptor, bool) {
	d, ok := r.descriptors[name]
	if !ok {
		return Descriptor{}, false
	}
	return *d, true
}

// Builtins is the registry holding exactly the nine callables of SPEC Appendix F.
func Builtins() *Registry {
	r := NewRegistry()
	for _, d := range builtins {
		if err := r.Register(d); err != nil {
			panic(err) // the table below is wrong, which no input can cause
		}
	}
	return r
}

var builtins = []Descriptor{
	{Name: "coalesce", Kind: Scalar, MinArgs: 2, MaxArgs: 2, Factory: buildCoalesce},
	{Name: "minimum", Kind: Scalar, MinArgs: 2, MaxArgs: 2, Factory: buildExtreme(false)},
	{Name: "maximum", Kind: Scalar, MinArgs: 2, MaxArgs: 2, Factory: buildExtreme(true)},
	{Name: "abs", Kind: Scalar, MinArgs: 1, MaxArgs: 1, Factory: buildAbs},
	{Name: "math:round", Kind: Scalar, MinArgs: 1, MaxArgs: 1, Factory: buildRound},
	{Name: "cast", Kind: Scalar, MinArgs: 2, MaxArgs: 2, Factory: buildCast},
	{Name: "ifThenElse", Kind: Scalar, MinArgs: 3, MaxArgs: 3, Factory: buildIfThenElse},
	{Name: "count", Kind: Aggregate, MinArgs: 0, MaxArgs: 0, Factory: buildCount},
	{Name: "chain", Kind: PatternOnly, MinArgs: 1, MaxArgs: maxArgs},
}

func buildCoalesce(args []Expr, _ model.Expr) (Expr, error) {
	at := common(args[0].Type(), args[1].Type())
	if at == event.TInvalid {
		return nil, typeErr("coalesce needs both arguments of one type, got %s and %s", args[0].Type(), args[1].Type())
	}
	return &coalesce{x: args[0], y: args[1], typ: at}, nil
}

func buildExtreme(max bool) func([]Expr, model.Expr) (Expr, error) {
	name := "minimum"
	if max {
		name = "maximum"
	}
	return func(args []Expr, _ model.Expr) (Expr, error) {
		at := common(args[0].Type(), args[1].Type())
		switch at {
		case event.TInt, event.TFloat, event.TTimestamp, event.TDuration:
			return &extreme{x: args[0], y: args[1], typ: at, max: max}, nil
		}
		return nil, typeErr("%s needs two arguments of one numeric, timestamp or duration type, got %s and %s", name, args[0].Type(), args[1].Type())
	}
}

func buildAbs(args []Expr, _ model.Expr) (Expr, error) {
	if !numeric(args[0].Type()) {
		return nil, typeErr("abs needs a numeric argument, got %s", args[0].Type())
	}
	return &abs{x: args[0], typ: args[0].Type()}, nil
}

func buildRound(args []Expr, _ model.Expr) (Expr, error) {
	if args[0].Type() != event.TFloat {
		return nil, typeErr("math:round needs a float, got %s", args[0].Type())
	}
	return &round{x: args[0]}, nil
}

var castTargets = map[string]event.Type{
	"string": event.TString, "int": event.TInt, "float": event.TFloat, "bool": event.TBool,
}

func buildCast(args []Expr, call model.Expr) (Expr, error) {
	target := call.Args[1]
	if target.Kind != model.ExprLiteral || target.Lit.Kind != model.LitString {
		return nil, fnErr("cast's second argument is one of 'string', 'int', 'float', 'bool'")
	}
	to, ok := castTargets[target.Lit.Str]
	if !ok {
		return nil, fnErr("cast's second argument is one of 'string', 'int', 'float', 'bool', not %q", target.Lit.Str)
	}
	switch args[0].Type() {
	case event.TString, event.TInt, event.TFloat, event.TBool:
		return &cast{x: args[0], to: to}, nil
	}
	return nil, typeErr("cast takes a string, int, float or bool, got %s", args[0].Type())
}

func buildIfThenElse(args []Expr, _ model.Expr) (Expr, error) {
	if args[0].Type() != event.TBool {
		return nil, typeErr("ifThenElse needs a boolean condition, got %s", args[0].Type())
	}
	at := common(args[1].Type(), args[2].Type())
	if at == event.TInvalid {
		return nil, typeErr("ifThenElse needs both branches of one type, got %s and %s", args[1].Type(), args[2].Type())
	}
	return &ifThenElse{cond: args[0], a: args[1], b: args[2], typ: at}, nil
}

func buildCount(_ []Expr, _ model.Expr) (Expr, error) {
	return &later{typ: event.TInt, what: "count (S8)"}, nil
}
