package model

import "github.com/vorsar/cep/internal/event"

// LitType is the type a literal of kind k has on its own: event.TInvalid for a null, a param or an unknown kind,
// which have none.
func LitType(k LiteralKind) event.Type {
	switch k {
	case LitString:
		return event.TString
	case LitInt:
		return event.TInt
	case LitFloat:
		return event.TFloat
	case LitBool:
		return event.TBool
	case LitDuration:
		return event.TDuration
	case LitTimestamp:
		return event.TTimestamp
	default:
		return event.TInvalid
	}
}

// LitValue is l as a value of type t: a literal of t's own type, or an int literal for a float, which widens. ok is
// false for every other pair (a null, a param, a string for an object, a mismatch); each caller says why in its own
// words.
func LitValue(l Literal, t event.Type) (event.Value, bool) {
	if l.Kind == LitInt && t == event.TFloat {
		return event.Value{Kind: t, F: float64(l.Int)}, true
	}
	if t == event.TInvalid || LitType(l.Kind) != t {
		return event.Value{}, false
	}
	return litPayload(l, t), true
}

// litPayload copies the one field of l that t reads; t is one of the six types LitType gives a literal kind.
func litPayload(l Literal, t event.Type) event.Value {
	v := event.Value{Kind: t}
	switch t {
	case event.TString:
		v.Str = l.Str
	case event.TInt:
		v.I = l.Int
	case event.TFloat:
		v.F = l.Float
	case event.TBool:
		v.B = l.Bool
	case event.TDuration:
		v.D = l.Ms
	case event.TTimestamp:
		v.T = l.Ms
	}
	return v
}
