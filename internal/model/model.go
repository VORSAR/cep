// Package model is the rule model: the one IR both front-ends produce (ADR-0003).
//
// The DSL parser and the Go builder each build an App and nothing else; the validator and the planner read only an
// App. The IR is plain data: no behaviour, no pointers to runtime objects, and no source position in its canonical
// form: positions ride in fields of type Pos, filled by the DSL front-end for diagnostics and skipped by
// MarshalCanonical (ADR-0003 §1 as annotated at S2.2b). Every field mirrors one SPEC clause by name, so a reviewer can diff an App's
// canonical JSON (MarshalCanonical) against the rule text by eye. Durations and timestamps are int64 milliseconds
// (SEMANTICS §1). A ${param} reference stays unresolved in its exact slot (ADR-0003 §5).
package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// Pos is a source position: line and column in runes, both 1-based. The DSL front-end fills it; a model built
// through the Go API leaves it zero. MarshalCanonical skips every field of type Pos or []Pos.
type Pos struct {
	Line, Col int
}

// App is one rule file (SPEC §3): its version pragma, then params, streams, tables and rules in declaration order.
// Order is kept: stream declaration order breaks event-time ties (SPEC §6.4).
type App struct {
	Language    string // the pragma's id, e.g. "cep/0.1" (SPEC §3.1)
	LanguagePos Pos    // the pragma's id
	Params      []Param
	Streams     []Stream
	Tables      []Table
	Rules       []Rule
}

// Param is `param name type = literal;` (SPEC §4.1). Default is the literal; a deployment may override it.
type Param struct {
	Name    string
	Type    event.Type
	Default Literal
	Pos     Pos // `param`
}

// Attr is one attribute of a stream or table: `name type [optional]` (SPEC §4.2).
type Attr struct {
	Name     string
	Type     event.Type
	Optional bool
	Pos      Pos // the attribute's name (owner, S3.2b-1's review: D3's error points at it)
}

// Stream is `stream Name (attrs) [time-clause];` with its annotations (SPEC §4.2). A derived stream has no
// time clause: Time.Kind is TimeNone.
type Stream struct {
	Annotations []Annotation
	Name        string
	Attrs       []Attr
	Time        TimeClause
	Pos         Pos // the declaration's first token
}

// TimeKind names the time clause's form (SPEC §4.2).
type TimeKind string

// The time clause's forms. TimeNone marks a derived stream.
const (
	TimeNone      TimeKind = "none"
	TimeBounded   TimeKind = "bounded"   // time <attr> watermark bounded <duration>
	TimeAscending TimeKind = "ascending" // time <attr> watermark ascending
	TimeArrival   TimeKind = "arrival"   // time arrival
)

// TimeClause is a source stream's time clause: its time attribute, watermark strategy and `idle after`.
type TimeClause struct {
	Kind      TimeKind
	Attr      string // empty for TimeArrival and TimeNone
	Bound     int64  // ms; TimeBounded only
	IdleAfter int64  // ms; 0 when absent
	Pos       Pos    // `time`
	AttrPos   Pos    // the time attribute
	IdlePos   Pos    // the `idle after` duration
}

// Annotation is `@name(item, …)` (SPEC §4.2: @source, @sink, @map).
type Annotation struct {
	Name  string
	Items []AnnotationItem
	Pos   Pos // `@`
}

// AnnotationItem is `key = literal`, `key = ${param}`, or a nested annotation (then Key is empty).
type AnnotationItem struct {
	Key      string
	Value    Literal
	Nested   []Annotation // one nested annotation, or none
	Pos      Pos          // the key, or the nested annotation's `@`
	ValuePos Pos          // the value
}

// Table is `table Name (attrs) primary key (cols);` (SPEC §4.3).
type Table struct {
	Name       string
	Attrs      []Attr
	PrimaryKey []string
	Pos        Pos // `table`
}

// LiteralKind names a literal's type (SPEC §2.4), or a ${param} reference standing in a literal slot.
type LiteralKind string

// The literal kinds.
const (
	LitString    LiteralKind = "string"
	LitInt       LiteralKind = "int"
	LitFloat     LiteralKind = "float"
	LitBool      LiteralKind = "bool"
	LitDuration  LiteralKind = "duration"  // Ms
	LitTimestamp LiteralKind = "timestamp" // Ms since the Unix epoch, UTC
	LitNull      LiteralKind = "null"      // only in expect records (SPEC §6.8)
	LitParam     LiteralKind = "param"     // ${Param}, unresolved
)

// Literal is one literal value, or a ${param} in its slot.
type Literal struct {
	Kind  LiteralKind
	Str   string
	Int   int64
	Float float64
	Bool  bool
	Ms    int64
	Param string
}

// DurationOrParam is a duration slot that may hold ${param} (the grammar's duration-or-param). Ms 0 with no Param
// means the optional clause is absent: every duration slot requires > 0 (E-DUR-003).
type DurationOrParam struct {
	Ms    int64
	Param string
	Pos   Pos // the duration's first token, or the ${param}
}

// Rule is one `rule` statement, its clauses in SPEC §4.4's fixed order.
type Rule struct {
	Name             string
	Intent           string
	Partition        Partition // no Keys: unpartitioned
	Body             Body
	Expects          []Expect
	Todos            []string
	Select           []SelectItem
	OutputFirstEvery DurationOrParam // absent: Ms 0, no Param
	InsertInto       string
	Pos              Pos   // `rule`
	SelectPos        Pos   // `select`
	InsertIntoPos    Pos   // the target's name
	TodoPos          []Pos // each `todo`, in order
}

// Partition is `partition by k of S, … ttl D`, a key being `k` or `(k, j, …)` (SPEC §4.4, rev 5).
type Partition struct {
	Keys []PartitionKey
	TTL  DurationOrParam
	Pos  Pos // `partition`
}

// PartitionKey is `k of S` or `(k, j, …) of S` (SPEC §4.4, rev 5): one stream's key attributes, in written order.
type PartitionKey struct {
	Attrs     []KeyAttr // ≥ 1; one when written without brackets
	Grouped   bool      // written in brackets, even with one attribute (E-PART-005)
	Stream    string
	Pos       Pos // the key's first token: the attribute, or `(`
	StreamPos Pos // the stream after `of`
}

// KeyAttr is one attribute of a partition key.
type KeyAttr struct {
	Name string
	Pos  Pos
}

// Has reports whether attr is one of the key's attributes.
func (k PartitionKey) Has(attr string) bool {
	for _, a := range k.Attrs {
		if a.Name == attr {
			return true
		}
	}
	return false
}

// BodyKind names the rule body's form (SPEC §5).
type BodyKind string

// The body forms.
const (
	BodyPlain   BodyKind = "plain"
	BodyPattern BodyKind = "pattern"
	BodyWindow  BodyKind = "window"
)

// Body is the rule's `from` clause: a plain source, a pattern or a window. Only the fields of its Kind are set.
type Body struct {
	Kind    BodyKind
	Source  Source  // plain and window
	Pattern Pattern // pattern
	Window  Window  // window
	Pos     Pos     // `from`
}

// Source is `Stream[filter]`; a zero Filter (Kind "") means no filter.
type Source struct {
	Stream string
	Filter Expr
	Pos    Pos // the stream's name
}

// Pattern is a pattern body (SPEC §5.2), its mandatory elements in order.
type Pattern struct {
	Restart    string // "every" or "once"
	Elements   []PatternElement
	Within     DurationOrParam
	AfterMatch string // "skip past last"
	LateEvents LateEvents
}

// ElementKind names one pattern element.
type ElementKind string

// The pattern elements: a step, an order-free pair of steps, or a timed absence. The first element is always a
// step or a pair; each later step or pair follows `->`.
const (
	ElemStep    ElementKind = "step"    // name = Stream[filter]
	ElemPair    ElementKind = "pair"    // (a = A[…] and b = B[…])
	ElemAbsence ElementKind = "absence" // and not Stream[filter] for D
)

// PatternElement is one element of a pattern; only the fields of its Kind are set.
type PatternElement struct {
	Kind    ElementKind
	Steps   []Step          // one for a step, two for a pair
	Absence Source          // absence
	For     DurationOrParam // absence
}

// Step is `name = Stream[filter]`.
type Step struct {
	Name   string
	Source Source
	Pos    Pos // the capture's name
}

// LateEvents is `late events drop` or `late events allow D` (the grammar's late-clause).
type LateEvents struct {
	Mode  string          // "drop" or "allow"
	Allow DurationOrParam // allow only
	Pos   Pos             // `drop` or `allow`
}

// WindowKind names the window (SPEC §6.3).
type WindowKind string

// The window kinds.
const (
	WindowSort  WindowKind = "sort"
	WindowCron  WindowKind = "cron"
	WindowDelay WindowKind = "delay"
	WindowBatch WindowKind = "batch"
)

// Window is a windowed body: `from Source window kind(…) emit on … late events …`. Only the fields of its Kind are
// set.
type Window struct {
	Source      Source
	Kind        WindowKind
	SortN       int64  // sort
	SortAttr    string // sort
	SortDir     string // sort: "asc" or "desc"
	CronExpr    string // cron
	CronTZ      string // cron
	Delay       DurationOrParam
	Emit        string // "close", "change", "due" or "batch end"
	LateEvents  LateEvents
	SortAttrPos Pos // sort: the attribute
	EmitPos     Pos // the emit mode
}

// SelectItem is `expr [as name]`.
type SelectItem struct {
	Expr Expr
	As   string
	Pos  Pos // the item's first token
}

// Expect is one `expect` block (SPEC §6.8): given sections, then the expected outputs.
type Expect struct {
	Givens []Given
	Then   Then
}

// Given is `given [table] Name [records]`.
type Given struct {
	Table   bool
	Name    string
	Records []Record
}

// Then is `then Name [records]`; no records asserts that nothing fires.
type Then struct {
	Name    string
	Records []Record
}

// Record is `{field: value, …}` in written order.
type Record struct {
	Fields []Field
}

// Field is `name: literal-or-null`.
type Field struct {
	Name  string
	Value Literal
}

// ExprKind names an expression node (the grammar's expressions, SPEC §2.5).
type ExprKind string

// The expression nodes. Args holds the operands in source order.
const (
	ExprLiteral ExprKind = "literal" // Lit (a ${param} is a Literal of kind param)
	ExprAttr    ExprKind = "attr"    // Qualifier.Name, or Name; Qualifier is a capture name
	ExprRaw     ExprKind = "raw"     // raw(attr)
	ExprCall    ExprKind = "call"    // [Namespace:]Name(Args)
	ExprNeg     ExprKind = "neg"     // -Args[0]
	ExprNot     ExprKind = "not"     // not Args[0]
	ExprAnd     ExprKind = "and"
	ExprOr      ExprKind = "or"
	ExprCompare ExprKind = "compare" // Args[0] Op Args[1]; Op one of == != < <= > >=
	ExprArith   ExprKind = "arith"   // Args[0] Op Args[1]; Op one of + - * / %
	ExprIsNull  ExprKind = "isnull"  // Args[0] is [not] null; Negated for "is not null"
	ExprIn      ExprKind = "in"      // Args[0] [not] in Table; Negated for "not in"
	ExprText    ExprKind = "text"    // Args[0] Op Operand; Op a text, set or regex operator
)

// Expr is one expression node. Only the fields of its Kind are set; a zero Expr (Kind "") is "no expression".
type Expr struct {
	Kind      ExprKind
	Op        string
	Lit       Literal
	Qualifier string
	Name      string
	Namespace string
	Negated   bool
	Table     string
	Operand   TextOperand
	Args      []Expr
	Pos       Pos // the expression's first token
	TablePos  Pos // in: the table's name
}

// TextOperand is a text operator's right side: one string or ${param} for a text or regex operator; for a set
// operator, a list of strings, one ${param}, or a one-column table's name.
type TextOperand struct {
	Strings []string
	Param   string
	Table   string
	Pos     Pos // the operand's first token
}

// MarshalCanonical encodes the App as its canonical JSON (ADR-0003 §2). The rules, exactly:
//
//   - Every struct is a JSON object holding every one of its fields, always, zero values included ("explicit
//     defaults"), except the source positions: a field of type Pos or []Pos is never encoded. A key is the Go field name in lowerCamelCase: its first letter lowercased, or a whole leading
//     initialism (OutputFirstEvery is "outputFirstEvery", TTL is "ttl", CronTZ is "cronTZ"). Keys are sorted
//     bytewise. There is no whitespace and no trailing newline.
//   - A slice is an array in its order; a nil slice is []. A string is a JSON string escaped as encoding/json
//     does with HTML escaping off. A bool is true or false. An int64 is its decimal digits.
//   - An event.Type is its name: "invalid", "string", "int", "float", "bool", "timestamp", "duration", "object".
//   - A float64 is strconv.FormatFloat(f, 'g', -1, 64): 0.75, 1e+21, -0. NaN and ±Inf are not encodable: an
//     error names the slot.
//   - In a Literal of kind "timestamp", the key "ms" holds the time as an RFC 3339 UTC string with exactly three
//     fraction digits ("2026-01-01T10:00:00.000Z"); everywhere else "ms" is an integer.
func (a App) MarshalCanonical() ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, reflect.ValueOf(a), "app"); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// timestampLayout is RFC 3339 in UTC with exactly three fraction digits.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// errNotEncodable is wrapped by MarshalCanonical's error for a value JSON cannot hold.
var errNotEncodable = errors.New("model: not encodable")

func encode(buf *bytes.Buffer, v reflect.Value, path string) error {
	if t, ok := v.Interface().(event.Type); ok {
		return encodeString(buf, t.String())
	}
	switch v.Kind() {
	case reflect.Struct:
		return encodeStruct(buf, v, path)
	case reflect.Slice:
		buf.WriteByte('[')
		for i := range v.Len() {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encode(buf, v.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case reflect.String:
		return encodeString(buf, v.String())
	case reflect.Bool:
		buf.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int64:
		buf.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Float64:
		f := v.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("%w: %s is %v", errNotEncodable, path, f)
		}
		buf.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
	default:
		return fmt.Errorf("%w: %s has kind %s", errNotEncodable, path, v.Kind())
	}
	return nil
}

func encodeStruct(buf *bytes.Buffer, v reflect.Value, path string) error {
	type field struct {
		key   string
		value reflect.Value
	}
	fields := make([]field, 0, v.NumField())
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Type == reflect.TypeFor[Pos]() || f.Type == reflect.TypeFor[[]Pos]() {
			continue // positions are never canonical (ADR-0003 §1 as annotated)
		}
		fields = append(fields, field{lowerCamel(f.Name), v.Field(i)})
	}
	slices.SortFunc(fields, func(a, b field) int { return strings.Compare(a.key, b.key) })
	stamp := v.Type() == reflect.TypeFor[Literal]() && v.Interface().(Literal).Kind == LitTimestamp
	buf.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encodeString(buf, f.key); err != nil {
			return err
		}
		buf.WriteByte(':')
		var err error
		if stamp && f.key == "ms" {
			err = encodeString(buf, time.UnixMilli(f.value.Int()).UTC().Format(timestampLayout))
		} else {
			err = encode(buf, f.value, path+"."+f.key)
		}
		if err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

// lowerCamel lowercases a field name's first letter, or its whole leading initialism: TTL is "ttl".
func lowerCamel(name string) string {
	n := 0
	for n < len(name) && 'A' <= name[n] && name[n] <= 'Z' {
		n++
	}
	switch {
	case n == len(name):
		return strings.ToLower(name)
	case n > 1:
		n-- // the last capital starts the next word: URLPath is "urlPath"
	}
	return strings.ToLower(name[:n]) + name[n:]
}

// encodeString writes s as encoding/json does with HTML escaping off.
func encodeString(buf *bytes.Buffer, s string) error {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // Encode ends with a newline
	return nil
}
