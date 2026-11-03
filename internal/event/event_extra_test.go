package event

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Sources cited below:
//   design §1 r<n> - s1-1a-gate-design.md §1, NewSchema rule n (numbered as in schema_test.go), and its
//                    2026-11-03 clarification: on error the zero Schema; each rule's message is distinct
//                    and names the offending attribute where there is one
//   NewSchema doc  - event.go's NewSchema comment: when several rules are broken, the one reported is
//                    the first in this order: r1, r2, then each attribute in declaration order (its name,
//                    r3 and r4, then its type, r5), then r6, r7. Design §1 fixes no order; the doc does.
//   ADR-0010 §1    - the steady-state hot path allocates nothing per event; every buffer that releases
//                    events sorts them with Compare (ARCHITECTURE §4)

// ruleSentinels holds each rule's sentinel, r1 at index 0 through r7 at index 6.
func ruleSentinels() []error {
	return []error{
		errEmptyStream, errNoAttrs, errEmptyAttrName, errDuplicateAttrName,
		errInvalidAttrType, errTimeAttrRange, errTimeAttrType,
	}
}

// rulesReported lists the rule numbers (1 to 7) whose sentinel err wraps, in rule order.
func rulesReported(err error) []int {
	var rules []int
	for i, s := range ruleSentinels() {
		if errors.Is(err, s) {
			rules = append(rules, i+1)
		}
	}
	return rules
}

// TestNewSchemaReportsTheBrokenRule: each input breaks exactly one rule, and NewSchema's error wraps
// that rule's sentinel and no other, so the message names the rule that fired, not a neighbour.
func TestNewSchemaReportsTheBrokenRule(t *testing.T) {
	t.Parallel()
	ts := []Attr{{Name: "ts", Type: TTimestamp}}
	cases := []struct {
		name    string
		in      schemaInput
		rule    int
		mention string // text the message must contain: the offending attribute, or "" for none
	}{
		// design §1 r1.
		{"r1/empty stream name", schemaInput{"", ts, 0}, 1, ""},
		// design §1 r2: nil and empty are both zero attributes.
		{"r2/nil attributes", schemaInput{"orders", nil, ArrivalTime}, 2, ""},
		{"r2/empty attributes", schemaInput{"orders", []Attr{}, ArrivalTime}, 2, ""},
		// design §1 r3: the attribute has no name, so the clarification's "names the offending
		// attribute" can only mean its position.
		{"r3/first attribute", schemaInput{"orders", []Attr{{Name: "", Type: TInt}}, ArrivalTime}, 3, "attribute 0"},
		{"r3/second attribute", schemaInput{"orders", []Attr{{Name: "id", Type: TInt}, {Name: "", Type: TString}}, ArrivalTime}, 3, "attribute 1"},
		// design §1 r4: the duplicate is named.
		{"r4/not adjacent", schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TInt}, {Name: "mid", Type: TString}, {Name: "qz_dup", Type: TInt}}, ArrivalTime}, 4, "qz_dup"},
		// design §1 r4; SPEC §2.1 (case-sensitive): only the exact repeat is a duplicate, and it is the one named.
		{"r4/after a case variant", schemaInput{"orders", []Attr{{Name: "qz_a", Type: TInt}, {Name: "QZ_A", Type: TInt}, {Name: "qz_a", Type: TInt}}, ArrivalTime}, 4, "qz_a"},
		// design §1 r5: both ends of the invalid range.
		{"r5/TInvalid", schemaInput{"orders", []Attr{{Name: "qz_kind", Type: TInvalid}}, ArrivalTime}, 5, "qz_kind"},
		{"r5/Type(8)", schemaInput{"orders", []Attr{{Name: "qz_kind", Type: Type(8)}}, ArrivalTime}, 5, "qz_kind"},
		// design §1 r6: past the end, and negatives that are not ArrivalTime.
		{"r6/one past the end", schemaInput{"orders", ts, 1}, 6, ""},
		{"r6/-2", schemaInput{"orders", ts, -2}, 6, ""},
		{"r6/MinInt", schemaInput{"orders", ts, math.MinInt}, 6, ""},
		// design §1 r7: the time attribute is the duration, not the timestamp beside it.
		{"r7/duration", schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}, {Name: "qz_when", Type: TDuration}}, 1}, 7, "qz_when"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSchema(c.in.stream, c.in.attrs, c.in.timeAttr)
			if err == nil {
				t.Fatalf("NewSchema(%q, %+v, %d) returned no error; want r%d", c.in.stream, c.in.attrs, c.in.timeAttr, c.rule)
			}
			if got, want := rulesReported(err), []int{c.rule}; !slices.Equal(got, want) {
				t.Errorf("error %q reports rules %v; want exactly %v", err, got, want)
			}
			if c.mention != "" && !strings.Contains(err.Error(), c.mention) {
				t.Errorf("error %q does not contain %q", err, c.mention)
			}
		})
	}
}

// TestNewSchemaFirstBrokenRuleWins: when one declaration breaks several rules, NewSchema reports one
// error, the first broken rule in the NewSchema doc's order, so the same input always gets the same message.
func TestNewSchemaFirstBrokenRuleWins(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   schemaInput
		rule int
	}{
		// Breaks r1, r2 and r6: the stream name comes first.
		{"r1 before r2 and r6", schemaInput{"", nil, 5}, 1},
		// Breaks r2 and r6 (index 0 of no attributes): the attribute count comes before the index.
		{"r2 before r6", schemaInput{"s", nil, 0}, 2},
		// Breaks r3, r5 and r6 on one attribute: its name comes before its type and the index.
		{"r3 before r5 and r6", schemaInput{"s", []Attr{{Name: "", Type: TInvalid}}, 3}, 3},
		// Breaks r4, r5 and r7 on attribute 1: its name comes before its type and the time attribute.
		{"r4 before r5 and r7", schemaInput{"s", []Attr{{Name: "a", Type: TInt}, {Name: "a", Type: TInvalid}}, 0}, 4},
		// Breaks r5 on attribute 0 and r3 on attribute 1: declaration order comes before rule order.
		{"attribute 0's r5 before attribute 1's r3", schemaInput{"s", []Attr{{Name: "a", Type: TInvalid}, {Name: "", Type: TInt}}, ArrivalTime}, 5},
		// Breaks r5 and r6: every attribute comes before the index.
		{"r5 before r6", schemaInput{"s", []Attr{{Name: "a", Type: TInt}, {Name: "b", Type: Type(9)}}, 7}, 5},
		// Breaks r5 and r7 on the same attribute: its type comes before the time attribute's type.
		{"r5 before r7", schemaInput{"s", []Attr{{Name: "a", Type: TInvalid}}, 0}, 5},
		// Breaks r4 and r6: every attribute comes before the index.
		{"r4 before r6", schemaInput{"s", []Attr{{Name: "a", Type: TTimestamp}, {Name: "a", Type: TTimestamp}}, 2}, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewSchema(c.in.stream, c.in.attrs, c.in.timeAttr)
			if err == nil {
				t.Fatalf("NewSchema(%q, %+v, %d) returned no error; want r%d", c.in.stream, c.in.attrs, c.in.timeAttr, c.rule)
			}
			if rules, want := rulesReported(err), []int{c.rule}; !slices.Equal(rules, want) {
				t.Errorf("error %q reports rules %v; want exactly %v", err, rules, want)
			}
			// design §1 clarification: on any error, the zero Schema.
			if !reflect.DeepEqual(got, Schema{}) {
				t.Errorf("NewSchema returned %#v with its error; want the zero Schema", got)
			}
		})
	}
}

// TestCompareDoesNotAllocate: ADR-0010 §1, Compare runs per event on the hot path, inside every
// release buffer's sort, so neither a comparison nor a sort with it may allocate.
//
// Not parallel: testing.AllocsPerRun counts the whole process's allocations, so another test running
// at the same time would be counted against Compare.
func TestCompareDoesNotAllocate(t *testing.T) {
	a := &Event{Ts: 5, StreamID: 1, Seq: 3, BatchID: "b", Values: []Value{{Kind: TInt, I: 1}}}
	b := &Event{Ts: 5, StreamID: 1, Seq: 4}
	sum := 0
	if n := testing.AllocsPerRun(1000, func() { sum += Compare(a, b) + Compare(b, a) }); n != 0 {
		t.Errorf("Compare allocated %v times per run; want 0 (ADR-0010 §1)", n)
	}
	_ = sum

	src := sevenInOrder()
	slices.Reverse(src)
	buf := make([]*Event, len(src))
	if n := testing.AllocsPerRun(1000, func() {
		copy(buf, src)
		slices.SortFunc(buf, Compare)
	}); n != 0 {
		t.Errorf("sorting %d events with Compare allocated %v times per run; want 0 (ADR-0010 §1)", len(src), n)
	}
	// The measured sort did sort: sevenInOrder's BatchIDs run "g" down to "a" in the hand-written order.
	got := make([]string, len(buf))
	for i, e := range buf {
		got[i] = e.BatchID
	}
	if want := []string{"g", "f", "e", "d", "c", "b", "a"}; !slices.Equal(got, want) {
		t.Errorf("the measured sort left BatchIDs %v; want %v", got, want)
	}
}
