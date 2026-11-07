package cep

import (
	"fmt"

	"github.com/vorsar/cep/internal/event"
	cepio "github.com/vorsar/cep/io"
)

// row is the facade's RowWriter (design s3-3-1 §4): one per SendRow, Emit, Upsert row or Delete key. Its values
// are fresh and handed to the engine, which keeps them (runtime.Send); nothing is pooled (cep may not import sync),
// and a Set after fill returned panics (MUST 14).
type row struct {
	attrs    []event.Attr
	names    []string
	timeAttr int // the stream's time attribute; event.ArrivalTime for an arrival stream; -1 for a table
	values   []event.Value
	set      []bool
	err      error
	armed    bool
	keyOnly  []int // Delete: the key attributes' indexes; nil otherwise
	what     string
}

func newStreamRow(sch event.Schema, names []string) *row {
	attrs := make([]event.Attr, sch.NumAttrs())
	for i := range attrs {
		attrs[i] = sch.Attr(i)
	}
	return &row{attrs: attrs, names: names, timeAttr: sch.TimeAttr(), values: make([]event.Value, len(attrs)), set: make([]bool, len(attrs)), what: "the stream's"}
}

func newTableRow(attrs []event.Attr, names []string, keyOnly []int) *row {
	return &row{attrs: attrs, names: names, timeAttr: -1, values: make([]event.Value, len(attrs)), set: make([]bool, len(attrs)), keyOnly: keyOnly, what: "the table's"}
}

// run arms the row, calls fill, disarms it and checks what fill left (design s3-3-1 §4.3, rows 5).
func (w *row) run(fill func(cepio.RowWriter) error) error {
	w.armed = true
	err := fill(w)
	w.armed = false
	if err != nil {
		return err
	}
	if w.err != nil {
		return w.err
	}
	for i, a := range w.attrs {
		if w.set[i] {
			continue
		}
		if w.keyOnly != nil {
			continue // a delete key names its columns only; the unset ones are not part of it
		}
		if i == w.timeAttr {
			return fmt.Errorf("cep: the time attribute %q was not set", a.Name)
		}
		if !a.Optional {
			return fmt.Errorf("cep: attribute %q is not optional and was not set", a.Name)
		}
		w.values[i] = event.Value{Kind: a.Type, Null: true}
	}
	if w.keyOnly != nil {
		for _, k := range w.keyOnly {
			if !w.set[k] {
				return fmt.Errorf("cep: delete key column %q was not set", w.attrs[k].Name)
			}
		}
	}
	return nil
}

// put is every Set: the checks of design s3-3-1 §4.3 rows 1–4; the first error is kept for run.
func (w *row) put(attr int, kind event.Type, v event.Value, what string) {
	if !w.armed {
		panic(ErrRowUsedAfterFill)
	}
	if attr < 0 || attr >= len(w.values) {
		w.fail(fmt.Errorf("cep: attribute index %d is outside %s %d attributes", attr, w.what, len(w.values)))
		return
	}
	a := w.attrs[attr]
	if kind != event.TInvalid && kind != a.Type {
		w.fail(fmt.Errorf("cep: attribute %q is declared %v; %s was called", a.Name, a.Type, what))
		return
	}
	if w.keyOnly != nil && !contains(w.keyOnly, attr) {
		w.fail(fmt.Errorf("%w: %q", ErrNotAKeyColumn, a.Name))
		return
	}
	if v.Null && !a.Optional {
		w.fail(fmt.Errorf("cep: attribute %q is not optional; SetNull was called", a.Name))
		return
	}
	v.Kind = a.Type
	w.values[attr], w.set[attr] = v, true
}

func (w *row) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

func contains(xs []int, x int) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func (w *row) SetString(attr int, v string) {
	w.put(attr, event.TString, event.Value{Str: v}, "SetString")
}
func (w *row) SetInt(attr int, v int64)     { w.put(attr, event.TInt, event.Value{I: v}, "SetInt") }
func (w *row) SetFloat(attr int, v float64) { w.put(attr, event.TFloat, event.Value{F: v}, "SetFloat") }
func (w *row) SetBool(attr int, v bool)     { w.put(attr, event.TBool, event.Value{B: v}, "SetBool") }
func (w *row) SetTimestamp(attr int, ms int64) {
	w.put(attr, event.TTimestamp, event.Value{T: ms}, "SetTimestamp")
}
func (w *row) SetDuration(attr int, ms int64) {
	w.put(attr, event.TDuration, event.Value{D: ms}, "SetDuration")
}
func (w *row) SetObject(attr int, jsonText string) {
	w.put(attr, event.TObject, event.Value{Obj: jsonText}, "SetObject")
}
func (w *row) SetNull(attr int) { w.put(attr, event.TInvalid, event.Value{Null: true}, "SetNull") }

func (w *row) AttrIndex(name string) int { return attrIndex(w.names, name) }

func (w *row) AttrType(attr int) cepio.Type {
	if attr < 0 || attr >= len(w.attrs) {
		return cepio.Invalid
	}
	return cepio.Type(w.attrs[attr].Type) // the seven types number alike (event.TString == cepio.String, …)
}

func attrIndex(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}
