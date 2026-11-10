package plan

import "github.com/vorsar/cep/internal/event"

// Expect is one expect block of a rule, as values (SPEC §6.8; design s3-3-2 §4). Build fills it from a checked
// model: every name is declared and every value has its field's type (E-EXP-002).
type Expect struct {
	Rule      int // the owning rule's index in App.Rules
	Block     int // 1-based within the rule
	Line, Col int // `expect`
	ThenLine  int // `then`
	ThenCol   int
	Tables    []GivenTable // in written order; seeded before any event
	Lists     []GivenList  // in written order; at most one per stream (E-EXP-002 X9)
	Then      []ThenRecord // the rule's outputs, in order; empty: nothing fires
}

// GivenTable is `given table T [rows]`: each row in the table's attribute order, an omitted optional column null.
type GivenTable struct {
	Table int
	Rows  [][]event.Value
}

// GivenList is `given S [records]`, each event's values in the stream's attribute order; an omitted optional
// attribute is null; the text_invisibles slot, if declared, is false (the ingest step computes it).
type GivenList struct {
	Stream int
	Events []GivenEvent
}

// GivenEvent is one given event; Ts is its time attribute's value (never null: E-EXP-002 X4, E-TIME-003).
type GivenEvent struct {
	Ts     int64
	Values []event.Value
}

// ThenRecord is one then-record: the attributes it names, in written order, and their values.
type ThenRecord struct {
	Line, Col int   // `{`
	Attrs     []int // indexes into the target stream's schema
	Values    []event.Value
}
