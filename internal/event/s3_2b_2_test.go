package event_test

import (
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// TestDecodeReportsTheDeepestNesting (design s3-2b-2 §6, review F4): Decoded.Depth is the payload's deepest nesting,
// the top-level object counted 1, so live intake can refuse what a recording line, one level deeper, cannot hold.
func TestDecodeReportsTheDeepestNesting(t *testing.T) {
	t.Parallel()
	s, err := event.NewSchema("S", []event.Attr{{Name: "o", Type: event.TObject, Optional: true}}, event.ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	d := event.NewDecoder(s)
	for payload, want := range map[string]int{
		`{}`:                    1,
		`{"o":{}}`:              2,
		`{"o":[[1],{"a":[2]}]}`: 4,
		`{"x":[[[["deep"]]]]}`:  5,
		`{"x":"[[[not nested"}`: 1,
		`{"o":` + strings.Repeat("[", 9998) + strings.Repeat("]", 9998) + `}`: 9999,
	} {
		_, info, err := d.Decode([]byte(payload), nil)
		if err != nil || info.Depth != want {
			t.Errorf("Decode(%.40s…): depth %d, %v; want %d", payload, info.Depth, err, want)
		}
	}
}

// TestExactObject (owner D1 of S3.2b-2's design): an object value a log line holds as it is — valid, compact, its
// UTF-8 well formed, nested within the line's limit — and nothing else.
func TestExactObject(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]bool{
		`{"a":1}`:          true,
		`[1,{"b":[]}]`:     true,
		`{ "a": 1 }`:       false,
		`{"a":`:            false,
		`{"a":1}x`:         false,
		`"a string"`:       false,
		"{\"a\":\"\xff\"}": false,
		``:                 false,
		strings.Repeat("[", 9998) + strings.Repeat("]", 9998): true,
		strings.Repeat("[", 9999) + strings.Repeat("]", 9999): false,
	} {
		if got := event.ExactObject(text); got != want {
			t.Errorf("ExactObject(%.30q): %v, want %v", text, got, want)
		}
	}
}
