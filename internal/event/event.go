// Package event holds the engine's canonical event, its schema, and the one total order
// in which every buffer releases events (ARCHITECTURE §4, SEMANTICS §1 and §2.2).
//
// This file is the S1.1a-1 declaration stub: the surface is final (design note §1), the
// bodies of String, NewSchema and Compare are placeholders until Lane D implements them
// against QA's tables.
package event

import "errors"

// Type is an attribute's declared type. The zero value, TInvalid, marks an unset slot:
// it is never a declared type and never a null (a null keeps its declared type).
type Type uint8

// The attribute types of SEMANTICS §1, after TInvalid.
const (
	TInvalid Type = iota
	TString
	TInt
	TFloat
	TBool
	TTimestamp
	TDuration
	TObject
)

// String names the type: "invalid", "string", …, "object"; otherwise "Type(<n>)".
func (t Type) String() string {
	return "" // TODO(S1.1a-1): Lane D implements.
}

// Attr is one declared attribute of a stream.
type Attr struct {
	Name     string
	Type     Type
	Optional bool
}

// ArrivalTime is the TimeAttr of a stream declared `time arrival`.
const ArrivalTime = -1

// Schema is a stream's validated declaration. Its fields are private so that NewSchema is
// the only way to build a usable one; the zero Schema has no attributes.
type Schema struct {
	stream   string
	attrs    []Attr
	timeAttr int
}

var errNotImplemented = errors.New("event: not implemented")

// The rule each NewSchema error wraps, one per rule of design note §1, r1 to r7. They stay
// unexported: the surface is fixed, and a caller needs the message, not the rule's identity.
var (
	errEmptyStream       = errors.New("empty stream name")
	errNoAttrs           = errors.New("no attributes declared")
	errEmptyAttrName     = errors.New("empty attribute name")
	errDuplicateAttrName = errors.New("duplicate attribute name")
	errInvalidAttrType   = errors.New("invalid attribute type")
	errTimeAttrRange     = errors.New("time attribute out of range")
	errTimeAttrType      = errors.New("time attribute is not a timestamp")
)

// NewSchema validates and copies a stream declaration (design note §1 lists the rules).
func NewSchema(stream string, attrs []Attr, timeAttr int) (Schema, error) {
	_, _, _ = stream, attrs, timeAttr
	return Schema{}, errNotImplemented // TODO(S1.1a-1): Lane D implements.
}

// Stream is the stream's name.
func (s Schema) Stream() string { return s.stream }

// NumAttrs is the number of declared attributes.
func (s Schema) NumAttrs() int { return len(s.attrs) }

// Attr returns a copy of attribute i; an out-of-range i panics like any slice index.
func (s Schema) Attr(i int) Attr { return s.attrs[i] }

// TimeAttr is the index of the event-time attribute, or ArrivalTime.
func (s Schema) TimeAttr() int { return s.timeAttr }

// Value is a tagged union. A null is Value{Kind: <declared type>, Null: true}; there is no null Kind.
type Value struct {
	Kind Type
	Str  string
	I    int64
	F    float64
	B    bool
	T    int64
	D    int64
	Obj  any
	Null bool
}

// Event is one ingested event; Values are aligned to its stream's Schema attributes.
type Event struct {
	StreamID   int
	Ts         int64
	Seq        uint64
	Values     []Value
	Invisibles bool
	BatchID    string
}

// Compare orders events by (Ts, StreamID, Seq) and returns -1, 0 or +1. Both arguments must be non-nil.
func Compare(a, b *Event) int {
	_, _ = a, b
	return 0 // TODO(S1.1a-1): Lane D implements.
}
