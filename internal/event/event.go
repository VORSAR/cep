// Package event holds the engine's canonical event, its schema, and the one total order
// in which every buffer releases events (ARCHITECTURE §4, SEMANTICS §1 and §2.2).
package event

import (
	"cmp"
	"errors"
	"fmt"
)

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
	switch t {
	case TInvalid:
		return "invalid"
	case TString:
		return "string"
	case TInt:
		return "int"
	case TFloat:
		return "float"
	case TBool:
		return "bool"
	case TTimestamp:
		return "timestamp"
	case TDuration:
		return "duration"
	case TObject:
		return "object"
	default:
		// The uint8 conversion keeps fmt off String, whatever the verb.
		return fmt.Sprintf("Type(%d)", uint8(t))
	}
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

// NewSchema validates and copies a stream declaration (design note §1). It rejects an empty
// stream name; zero attributes; an empty attribute name; a duplicate attribute name, compared
// case-sensitively; an attribute Type of TInvalid or above TObject; a timeAttr that is neither
// ArrivalTime nor an attribute index; and a timeAttr naming an attribute that is not a
// TTimestamp. An Optional time attribute is accepted: a null timestamp is an event-level error
// at ingest (SEMANTICS §1.1), not a schema error.
//
// When several rules are broken it reports the first, checking in this order: the stream name,
// the attribute count, each attribute in declaration order (its name, then its type), then
// timeAttr. Each rule has its own message, which names the offending attribute where there is
// one. On error it returns the zero Schema.
//
// It reads attrs and never writes it. The Schema keeps its own copy, so changing attrs
// afterwards changes nothing.
func NewSchema(stream string, attrs []Attr, timeAttr int) (Schema, error) {
	if stream == "" {
		return Schema{}, fmt.Errorf("event: invalid schema: %w", errEmptyStream)
	}
	if len(attrs) == 0 {
		return Schema{}, fmt.Errorf("event: invalid schema for stream %q: %w (a stream declares at least one)",
			stream, errNoAttrs)
	}
	// Check the copy, not the caller's slice, so the schema holds exactly what was checked.
	own := make([]Attr, len(attrs))
	copy(own, attrs)
	declaredAt := make(map[string]int, len(own))
	for i, a := range own {
		if a.Name == "" {
			return Schema{}, fmt.Errorf("event: invalid schema for stream %q: attribute %d: %w",
				stream, i, errEmptyAttrName)
		}
		if first, dup := declaredAt[a.Name]; dup {
			return Schema{}, fmt.Errorf("event: invalid schema for stream %q: attribute %d %q: %w, first declared as attribute %d (names are case-sensitive)",
				stream, i, a.Name, errDuplicateAttrName, first)
		}
		declaredAt[a.Name] = i
		if a.Type < TString || a.Type > TObject {
			return Schema{}, fmt.Errorf("event: invalid schema for stream %q: attribute %d %q: %w (%v); want one of %v, %v, %v, %v, %v, %v, %v",
				stream, i, a.Name, errInvalidAttrType, a.Type, TString, TInt, TFloat, TBool, TTimestamp, TDuration, TObject)
		}
	}
	if timeAttr != ArrivalTime {
		if timeAttr < 0 || timeAttr >= len(own) {
			return Schema{}, fmt.Errorf("event: invalid schema for stream %q: %w: index %d; want ArrivalTime (%d) or 0 to %d",
				stream, errTimeAttrRange, timeAttr, ArrivalTime, len(own)-1)
		}
		if a := own[timeAttr]; a.Type != TTimestamp {
			return Schema{}, fmt.Errorf("event: invalid schema for stream %q: attribute %d %q: %w (its type is %v)",
				stream, timeAttr, a.Name, errTimeAttrType, a.Type)
		}
	}
	return Schema{stream: stream, attrs: own, timeAttr: timeAttr}, nil
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
	// Tables is the app's table snapshot at this event's ingest position (a *table.Snapshot; SEMANTICS §4 as
	// annotated, owner D1 of S3.1-2). Set by ingest only, before the event is routed, and never written after.
	// Opaque here, since event imports nothing; the runtime's expr.Row reads it.
	Tables any
}

// Compare orders events by (Ts, StreamID, Seq) and returns -1, 0 or +1. Both arguments must be non-nil.
// Payload fields never affect it.
//
// Each key is compared with cmp.Compare and the first difference decides. It never subtracts,
// which would overflow at the int64, int and uint64 extremes. It returns early rather than
// using cmp.Or, which would evaluate all three comparisons on every call of a hot-path sort.
func Compare(a, b *Event) int {
	if c := cmp.Compare(a.Ts, b.Ts); c != 0 {
		return c
	}
	if c := cmp.Compare(a.StreamID, b.StreamID); c != 0 {
		return c
	}
	return cmp.Compare(a.Seq, b.Seq)
}
