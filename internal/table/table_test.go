package table_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// ---- values ----

func s(v string) event.Value                   { return event.Value{Kind: event.TString, Str: v} }
func i(v int64) event.Value                    { return event.Value{Kind: event.TInt, I: v} }
func f(v float64) event.Value                  { return event.Value{Kind: event.TFloat, F: v} }
func b(v bool) event.Value                     { return event.Value{Kind: event.TBool, B: v} }
func ts(v int64) event.Value                   { return event.Value{Kind: event.TTimestamp, T: v} }
func dur(v int64) event.Value                  { return event.Value{Kind: event.TDuration, D: v} }
func obj(v string) event.Value                 { return event.Value{Kind: event.TObject, Obj: v} }
func null(t event.Type) event.Value            { return event.Value{Kind: t, Null: true} }
func row(vs ...event.Value) []event.Value      { return vs }
func rows(rs ...[]event.Value) [][]event.Value { return rs }

// hostsAttrs is the tests' usual table: Hosts(host string, owner optional string, port optional int, meta optional
// object) primary key (host).
var hostsAttrs = []event.Attr{
	{Name: "host", Type: event.TString},
	{Name: "owner", Type: event.TString, Optional: true},
	{Name: "port", Type: event.TInt, Optional: true},
	{Name: "meta", Type: event.TObject, Optional: true},
}

func host(h, owner string) []event.Value {
	return row(s(h), s(owner), null(event.TInt), null(event.TObject))
}

func newTable(t *testing.T, name string, attrs []event.Attr, key ...string) *table.Table {
	t.Helper()
	tb, err := table.New(name, attrs, key)
	if err != nil {
		t.Fatalf("New(%q): %v", name, err)
	}
	return tb
}

func hosts(t *testing.T) *table.Table { return newTable(t, "Hosts", hostsAttrs, "host") }

// keyed is a one-column-key table of type typ: Keyed(k typ, note optional string) primary key (k).
func keyed(t *testing.T, typ event.Type) *table.Table {
	return newTable(t, "Keyed", []event.Attr{{Name: "k", Type: typ}, {Name: "note", Type: event.TString, Optional: true}}, "k")
}

func ok(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// ---- New (design note §1) ----

func TestNewRejects(t *testing.T) {
	str := event.Attr{Name: "a", Type: event.TString}
	cases := []struct {
		name  string
		tname string
		attrs []event.Attr
		key   []string
	}{
		{"an empty name", "", []event.Attr{str}, []string{"a"}},
		{"no attributes (nil)", "T", nil, []string{"a"}},
		{"no attributes (empty)", "T", []event.Attr{}, []string{"a"}},
		{"an empty attribute name", "T", []event.Attr{str, {Name: "", Type: event.TInt}}, []string{"a"}},
		{"a duplicate attribute name", "T", []event.Attr{str, {Name: "a", Type: event.TInt}}, []string{"a"}},
		{"type TInvalid", "T", []event.Attr{str, {Name: "b", Type: event.TInvalid}}, []string{"a"}},
		{"a type above TObject", "T", []event.Attr{str, {Name: "b", Type: event.TObject + 1}}, []string{"a"}},
		{"a nil key", "T", []event.Attr{str}, nil},
		{"an empty key", "T", []event.Attr{str}, []string{}},
		{"an undeclared key column", "T", []event.Attr{str}, []string{"b"}},
		{"a key column differing only in case", "T", []event.Attr{str}, []string{"A"}},
		{"a key column named twice", "T", []event.Attr{str, {Name: "b", Type: event.TInt}}, []string{"a", "a"}},
		{"an object key column", "T", []event.Attr{{Name: "o", Type: event.TObject}}, []string{"o"}},
		{"an optional object key column", "T", []event.Attr{str, {Name: "o", Type: event.TObject, Optional: true}}, []string{"a", "o"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb, err := table.New(c.tname, c.attrs, c.key)
			if err == nil {
				t.Fatalf("New accepted it: %+v", tb)
			}
			if tb != nil {
				t.Fatalf("New returned a table with its error %v", err)
			}
		})
	}
}

func TestNewAccepts(t *testing.T) {
	cases := []struct {
		name  string
		attrs []event.Attr
		key   []string
	}{
		{"one column", []event.Attr{{Name: "a", Type: event.TString}}, []string{"a"}},
		{"names differing only in case", []event.Attr{{Name: "a", Type: event.TString}, {Name: "A", Type: event.TInt}}, []string{"A"}},
		{"an optional key column (its null is refused at write, D3)", []event.Attr{{Name: "a", Type: event.TString, Optional: true}}, []string{"a"}},
		{"a composite key in another order than declared", []event.Attr{{Name: "a", Type: event.TInt}, {Name: "b", Type: event.TString}}, []string{"b", "a"}},
		{"every non-object type as a key", []event.Attr{
			{Name: "s", Type: event.TString}, {Name: "i", Type: event.TInt}, {Name: "f", Type: event.TFloat},
			{Name: "b", Type: event.TBool}, {Name: "t", Type: event.TTimestamp}, {Name: "d", Type: event.TDuration},
			{Name: "o", Type: event.TObject, Optional: true},
		}, []string{"s", "i", "f", "b", "t", "d"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb, err := table.New("Tbl", c.attrs, c.key)
			if err != nil {
				t.Fatal(err)
			}
			if tb.Name() != "Tbl" {
				t.Fatalf("Name() = %q", tb.Name())
			}
			v := tb.Snapshot()
			if v == nil || v.Len() != 0 {
				t.Fatalf("a new table's Snapshot is %v, want an empty Version", v)
			}
		})
	}
}

// TestNewCopiesItsInputs: the declaration is the table's own; changing the caller's slices afterwards changes nothing.
func TestNewCopiesItsInputs(t *testing.T) {
	attrs := []event.Attr{{Name: "host", Type: event.TString}, {Name: "n", Type: event.TInt}}
	key := []string{"host"}
	tb, err := table.New("Hosts", attrs, key)
	ok(t, err)
	attrs[0] = event.Attr{Name: "host", Type: event.TInt}
	attrs[1] = event.Attr{Name: "n", Type: event.TString}
	key[0] = "n"
	ok(t, tb.Upsert(rows(row(s("a"), i(1)))))
	if !tb.Snapshot().Has(s("a")) {
		t.Fatal("the row is not found by its host after the caller changed its slices")
	}
}

// ---- writes: every rule, all or nothing (design note §1) ----

// wantWriteErr checks that err names the table, the row's index and (when attr != "") the attribute, and that the
// table published nothing.
func wantWriteErr(t *testing.T, tb *table.Table, before *table.Version, err error, index int, attr string) {
	t.Helper()
	if err == nil {
		t.Fatal("the write was accepted")
	}
	msg := err.Error()
	if !strings.Contains(msg, tb.Name()) {
		t.Errorf("the error does not name the table %q: %v", tb.Name(), err)
	}
	if !strings.Contains(msg, strconv.Itoa(index)) {
		t.Errorf("the error does not name the row index %d: %v", index, err)
	}
	if attr != "" && !strings.Contains(msg, attr) {
		t.Errorf("the error does not name the attribute %q: %v", attr, err)
	}
	if after := tb.Snapshot(); after != before {
		t.Errorf("a failed write published a new Version")
	}
}

func TestWriteRejects(t *testing.T) {
	// Each bad row sits at index 3, after three good rows that must not land.
	good := rows(host("good-a", "x"), host("good-b", "y"), host("good-c", "z"))
	cases := []struct {
		name string
		bad  []event.Value
		attr string
	}{
		{"too few values", row(s("h"), s("o"), null(event.TInt)), ""},
		{"too many values", row(s("h"), s("o"), null(event.TInt), null(event.TObject), s("extra")), ""},
		{"no values", row(), ""},
		{"an int in a string column", row(s("h"), i(1), null(event.TInt), null(event.TObject)), "owner"},
		{"a string in an int column", row(s("h"), s("o"), s("443"), null(event.TObject)), "port"},
		{"a zero Value", row(s("h"), event.Value{}, null(event.TInt), null(event.TObject)), "owner"},
		{"a string where the object goes", row(s("h"), s("o"), i(1), s("{}")), "meta"},
		{"an int key for a string key column", row(i(7), s("o"), null(event.TInt), null(event.TObject)), "host"},
		{"a null key", row(null(event.TString), s("o"), null(event.TInt), null(event.TObject)), "host"},
		{"a null of the wrong Kind", row(s("h"), null(event.TInt), null(event.TInt), null(event.TObject)), "owner"},
	}
	writes := map[string]func(*table.Table, [][]event.Value) error{
		"Upsert": (*table.Table).Upsert,
		"Load":   (*table.Table).Load,
	}
	for wname, write := range writes {
		for _, c := range cases {
			t.Run(wname+"/"+c.name, func(t *testing.T) {
				tb := hosts(t)
				ok(t, tb.Upsert(rows(host("seed", "s"))))
				before := tb.Snapshot()
				in := append(append([][]event.Value{}, good...), c.bad, host("good-d", "w"))
				err := write(tb, in)
				wantWriteErr(t, tb, before, err, 3, c.attr)
				v := tb.Snapshot()
				if v.Len() != 1 || !v.Has(s("seed")) || v.Has(s("good-a")) || v.Has(s("good-d")) {
					t.Fatalf("after a failed %s the table holds %d rows (seed %v, good-a %v, good-d %v)",
						wname, v.Len(), v.Has(s("seed")), v.Has(s("good-a")), v.Has(s("good-d")))
				}
			})
		}
	}
}

// TestNullOnlyWhereOptional: a null is refused in a non-optional column and in a key column, optional or not.
func TestNullOnlyWhereOptional(t *testing.T) {
	attrs := []event.Attr{
		{Name: "id", Type: event.TInt, Optional: true}, // an optional key column: New accepts it, writes refuse its null (D3)
		{Name: "need", Type: event.TString},
		{Name: "maybe", Type: event.TString, Optional: true},
	}
	tb := newTable(t, "Opt", attrs, "id")
	before := tb.Snapshot()
	wantWriteErr(t, tb, before, tb.Upsert(rows(row(i(1), s("x"), null(event.TString)), row(null(event.TInt), s("x"), s("y")))), 1, "id")
	wantWriteErr(t, tb, before, tb.Upsert(rows(row(i(1), null(event.TString), s("y")))), 0, "need")
	wantWriteErr(t, tb, before, tb.Delete(rows(row(i(1)), row(null(event.TInt)))), 1, "id")
	ok(t, tb.Upsert(rows(row(i(1), s("x"), null(event.TString)))))
	if r, found := tb.Snapshot().Get(row(i(1))); !found || !r[2].Null || r[2].Kind != event.TString {
		t.Fatalf("the optional column's null was not stored as a string null: %+v %v", r, found)
	}
}

// TestNonFiniteFloatsRefused: NaN and ±Inf are never stored, in a key column or any other (design note §1, §6).
func TestNonFiniteFloatsRefused(t *testing.T) {
	attrs := []event.Attr{{Name: "k", Type: event.TFloat}, {Name: "score", Type: event.TFloat, Optional: true}}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		name := strconv.FormatFloat(bad, 'g', -1, 64)
		t.Run("key "+name, func(t *testing.T) {
			tb := newTable(t, "Scores", attrs, "k")
			before := tb.Snapshot()
			wantWriteErr(t, tb, before, tb.Upsert(rows(row(f(1), f(1)), row(f(bad), f(1)))), 1, "k")
			wantWriteErr(t, tb, before, tb.Load(rows(row(f(bad), f(1)))), 0, "k")
			wantWriteErr(t, tb, before, tb.Delete(rows(row(f(2)), row(f(bad)))), 1, "k")
		})
		t.Run("value "+name, func(t *testing.T) {
			tb := newTable(t, "Scores", attrs, "k")
			before := tb.Snapshot()
			wantWriteErr(t, tb, before, tb.Upsert(rows(row(f(1), f(bad)))), 0, "score")
			wantWriteErr(t, tb, before, tb.Load(rows(row(f(1), f(1)), row(f(2), f(2)), row(f(3), f(bad)))), 2, "score")
		})
	}
}

// TestDeleteKeyRules: a key passed to Delete follows the write rules for the key columns (design note §1).
func TestDeleteKeyRules(t *testing.T) {
	comp := newTable(t, "Pairs", []event.Attr{
		{Name: "tenant", Type: event.TString}, {Name: "port", Type: event.TInt}, {Name: "note", Type: event.TString, Optional: true},
	}, "port", "tenant")
	ok(t, comp.Upsert(rows(row(s("t1"), i(80), s("n")))))
	cases := []struct {
		name string
		key  []event.Value
		attr string
	}{
		{"too few key values", row(i(80)), ""},
		{"too many key values", row(i(80), s("t1"), s("n")), ""},
		{"the whole row instead of the key", row(s("t1"), i(80), s("n")), ""},
		{"key values in declaration order, not key order", row(s("t1"), i(80)), "port"},
		{"a wrong Kind", row(i(80), i(1)), "tenant"},
		{"a null key value", row(i(80), null(event.TString)), "tenant"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := comp.Snapshot()
			wantWriteErr(t, comp, before, comp.Delete(rows(row(i(1), s("x")), row(i(80), s("t1")), c.key)), 2, c.attr)
			if _, found := comp.Snapshot().Get(row(i(80), s("t1"))); !found {
				t.Fatal("a failed Delete removed a row")
			}
		})
	}
}

// ---- key equality (design note §1): per type, on the type's own field ----

func TestKeyEquality(t *testing.T) {
	t.Run("strings are byte-exact", func(t *testing.T) {
		tb := keyed(t, event.TString)
		ok(t, tb.Upsert(rows(row(s("abc"), s("1")), row(s("é"), s("nfc")), row(s("é"), s("nfd")))))
		v := tb.Snapshot()
		if v.Len() != 3 {
			t.Fatalf("NFC and NFD é must be two keys: Len = %d", v.Len())
		}
		for k, want := range map[string]bool{"abc": true, "ABC": false, "abc ": false, "ab": false, "é": true, "é": true, "": false} {
			if got := v.Has(s(k)); got != want {
				t.Errorf("Has(%+q) = %v, want %v", k, got, want)
			}
		}
	})
	t.Run("0.0 and -0.0 are one key; later wins", func(t *testing.T) {
		tb := keyed(t, event.TFloat)
		ok(t, tb.Upsert(rows(row(f(0), s("zero")))))
		ok(t, tb.Upsert(rows(row(f(math.Copysign(0, -1)), s("negzero")))))
		v := tb.Snapshot()
		if v.Len() != 1 {
			t.Fatalf("0.0 and -0.0 made %d keys", v.Len())
		}
		if !v.Has(f(0)) || !v.Has(f(math.Copysign(0, -1))) {
			t.Fatal("Has misses 0.0 or -0.0")
		}
		if r, found := v.Get(row(f(0))); !found || r[1].Str != "negzero" {
			t.Fatalf("Get(0.0) = %+v %v, want the -0.0 row that replaced it", r, found)
		}
	})
	t.Run("NaN and Inf lookups are false", func(t *testing.T) {
		tb := keyed(t, event.TFloat)
		ok(t, tb.Upsert(rows(row(f(1.5), s("x")), row(f(math.MaxFloat64), s("max")))))
		v := tb.Snapshot()
		for _, k := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			if v.Has(f(k)) {
				t.Errorf("Has(%v) = true", k)
			}
			if _, found := v.Get(row(f(k))); found {
				t.Errorf("Get(%v) found a row", k)
			}
		}
		if !v.Has(f(1.5)) || !v.Has(f(math.MaxFloat64)) || v.Has(f(1.25)) {
			t.Fatal("finite float lookups are wrong")
		}
	})
	t.Run("int, timestamp, duration and bool by value", func(t *testing.T) {
		for _, c := range []struct {
			typ       event.Type
			in, other event.Value
		}{
			{event.TInt, i(math.MinInt64), i(math.MaxInt64)},
			{event.TTimestamp, ts(1_700_000_000_000), ts(1_700_000_000_001)},
			{event.TDuration, dur(-5), dur(5)},
			{event.TBool, b(true), b(false)},
		} {
			tb := keyed(t, c.typ)
			ok(t, tb.Upsert(rows(row(c.in, s("x")))))
			v := tb.Snapshot()
			if !v.Has(c.in) || v.Has(c.other) {
				t.Errorf("%s: Has(%+v) = %v, Has(%+v) = %v", c.typ, c.in, v.Has(c.in), c.other, v.Has(c.other))
			}
		}
	})
	t.Run("only the type's own field counts, never the whole Value", func(t *testing.T) {
		tb := keyed(t, event.TString)
		ok(t, tb.Upsert(rows(row(event.Value{Kind: event.TString, Str: "k", I: 99, F: 2.5}, s("x")))))
		v := tb.Snapshot()
		if !v.Has(s("k")) {
			t.Fatal("a clean lookup misses a key stored with stray fields")
		}
		if !v.Has(event.Value{Kind: event.TString, Str: "k", T: 7, B: true, Obj: []byte("not comparable")}) {
			t.Fatal("a lookup with stray fields misses the key")
		}
		ti := keyed(t, event.TInt)
		ok(t, ti.Upsert(rows(row(i(5), s("x")))))
		if !ti.Snapshot().Has(event.Value{Kind: event.TInt, I: 5, Str: "five"}) {
			t.Fatal("an int lookup with a stray Str misses")
		}
	})
}

// ---- Upsert, Delete, Load (design note §1) ----

func TestUpsert(t *testing.T) {
	tb := hosts(t)
	ok(t, tb.Upsert(rows(host("a", "first"), host("b", "b1"))))
	ok(t, tb.Upsert(rows(host("a", "second"))))
	v := tb.Snapshot()
	if r, found := v.Get(row(s("a"))); v.Len() != 2 || !found || r[1].Str != "second" {
		t.Fatalf("a row whose key exists was not replaced: Len %d, %+v %v", v.Len(), r, found)
	}
	ok(t, tb.Upsert(rows(host("c", "early"), host("d", "d1"), host("c", "late"))))
	v = tb.Snapshot()
	if r, found := v.Get(row(s("c"))); v.Len() != 4 || !found || r[1].Str != "late" {
		t.Fatalf("within one call the later row must win: Len %d, %+v %v", v.Len(), r, found)
	}
	if r, _ := v.Get(row(s("b"))); r[1].Str != "b1" {
		t.Fatalf("an untouched row changed: %+v", r)
	}
	full := row(s("e"), s("o"), i(443), obj(`{"k": [1, 2]}`))
	ok(t, tb.Upsert(rows(full)))
	r, found := tb.Snapshot().Get(row(s("e")))
	if !found || len(r) != 4 || r[0] != full[0] || r[1] != full[1] || r[2] != full[2] || r[3].Kind != event.TObject || r[3].Obj != full[3].Obj {
		t.Fatalf("Get returned %+v, want %+v", r, full)
	}
}

func TestDelete(t *testing.T) {
	tb := hosts(t)
	ok(t, tb.Upsert(rows(host("a", "1"), host("b", "2"), host("c", "3"))))
	before := tb.Snapshot()
	ok(t, tb.Delete(rows(row(s("b")), row(s("nope")))))
	v := tb.Snapshot()
	if v.Len() != 2 || v.Has(s("b")) || !v.Has(s("a")) || !v.Has(s("c")) {
		t.Fatalf("Delete(b, nope): Len %d", v.Len())
	}
	if v == before {
		t.Fatal("a successful Delete published nothing")
	}
	before = tb.Snapshot()
	if err := tb.Delete(rows(row(s("absent")))); err != nil {
		t.Fatalf("a key with no row is not an error: %v", err)
	}
	if v := tb.Snapshot(); v.Len() != 2 {
		t.Fatalf("Delete of an absent key changed the content: Len %d", v.Len())
	}
	if tb.Snapshot() == before {
		// design note §1: any success with keys publishes exactly one new Version, even when nothing matched
		t.Fatal("Delete of an absent key published no new Version")
	}

	comp := newTable(t, "Pairs", []event.Attr{{Name: "tenant", Type: event.TString}, {Name: "port", Type: event.TInt}}, "port", "tenant")
	ok(t, comp.Upsert(rows(row(s("t1"), i(80)), row(s("t2"), i(80)), row(s("t1"), i(443)))))
	ok(t, comp.Delete(rows(row(i(80), s("t1")))))
	cv := comp.Snapshot()
	_, gone := cv.Get(row(i(80), s("t1")))
	_, kept1 := cv.Get(row(i(80), s("t2")))
	_, kept2 := cv.Get(row(i(443), s("t1")))
	if cv.Len() != 2 || gone || !kept1 || !kept2 {
		t.Fatalf("a composite Delete (key order port, tenant): Len %d, gone %v, kept %v %v", cv.Len(), gone, kept1, kept2)
	}
}

func TestLoad(t *testing.T) {
	tb := hosts(t)
	ok(t, tb.Upsert(rows(host("a", "1"), host("b", "2"))))
	ok(t, tb.Load(rows(host("c", "early"), host("d", "4"), host("c", "late"))))
	v := tb.Snapshot()
	if v.Len() != 2 || v.Has(s("a")) || v.Has(s("b")) || !v.Has(s("d")) {
		t.Fatalf("Load must replace everything: Len %d, a %v, b %v", v.Len(), v.Has(s("a")), v.Has(s("b")))
	}
	if r, _ := v.Get(row(s("c"))); r[1].Str != "late" {
		t.Fatalf("within one Load the later row must win: %+v", r)
	}
}

// ---- publishing (design note §1) ----

func TestPublishing(t *testing.T) {
	tb := hosts(t)
	empty := tb.Snapshot()
	if tb.Snapshot() != empty {
		t.Fatal("two Snapshots with no write between gave two Versions")
	}
	for name, call := range map[string]func() error{
		"Upsert(nil)":   func() error { return tb.Upsert(nil) },
		"Upsert(empty)": func() error { return tb.Upsert([][]event.Value{}) },
		"Delete(nil)":   func() error { return tb.Delete(nil) },
		"Delete(empty)": func() error { return tb.Delete([][]event.Value{}) },
	} {
		ok(t, call())
		if tb.Snapshot() != empty {
			t.Fatalf("%s published a Version", name)
		}
	}
	ok(t, tb.Upsert(rows(host("a", "1"), host("b", "2"), host("c", "3"))))
	v1 := tb.Snapshot()
	if v1 == empty || v1.Len() != 3 || empty.Len() != 0 {
		t.Fatalf("a successful Upsert: new %v, Len %d, the empty Version's Len %d", v1 != empty, v1.Len(), empty.Len())
	}
	ok(t, tb.Upsert(nil))
	if tb.Snapshot() != v1 {
		t.Fatal("an empty Upsert published a Version over a non-empty table")
	}
	if err := tb.Upsert(rows(host("d", "4"), row(s("bad")))); err == nil {
		t.Fatal("a short row was accepted")
	}
	if tb.Snapshot() != v1 {
		t.Fatal("a failed write changed Snapshot()")
	}
	ok(t, tb.Load(rows(host("z", "26"))))
	if v := tb.Snapshot(); v == v1 || v.Len() != 1 {
		t.Fatal("a successful Load did not publish its Version")
	}
}

// TestCopiesRows: the table copies every row it stores; a caller may reuse its slices (design note §1).
func TestCopiesRows(t *testing.T) {
	for name, write := range map[string]func(*table.Table, [][]event.Value) error{
		"Upsert": (*table.Table).Upsert,
		"Load":   (*table.Table).Load,
	} {
		t.Run(name, func(t *testing.T) {
			tb := hosts(t)
			r := host("a", "orig")
			in := rows(r)
			ok(t, write(tb, in))
			r[0], r[1] = s("zzz"), s("mutated")
			in[0] = host("other", "o")
			v := tb.Snapshot()
			if !v.Has(s("a")) || v.Has(s("zzz")) || v.Has(s("other")) {
				t.Fatal("changing the caller's slices changed the stored keys")
			}
			if got, _ := v.Get(row(s("a"))); got[1].Str != "orig" {
				t.Fatalf("changing the caller's row changed the stored row: %+v", got)
			}
		})
	}
	t.Run("a Delete key slice reused", func(t *testing.T) {
		tb := hosts(t)
		ok(t, tb.Upsert(rows(host("a", "1"), host("b", "2"))))
		k := row(s("a"))
		ok(t, tb.Delete(rows(k)))
		k[0] = s("b")
		if v := tb.Snapshot(); v.Has(s("a")) || !v.Has(s("b")) {
			t.Fatal("reusing a Delete key changed the content")
		}
	})
}

// TestOldSnapshotNeverChanges: a published Version is immutable whatever is written later (design note §1, F10).
func TestOldSnapshotNeverChanges(t *testing.T) {
	tb := hosts(t)
	ok(t, tb.Upsert(rows(host("a", "1"), host("b", "2"))))
	v1 := tb.Snapshot()
	ok(t, tb.Upsert(rows(host("a", "changed"), host("c", "3"))))
	ok(t, tb.Delete(rows(row(s("b")))))
	ok(t, tb.Load(rows(host("x", "x"))))
	_ = tb.Upsert(rows(host("y", "y"), row(s("bad"))))
	if v1.Len() != 2 || !v1.Has(s("a")) || !v1.Has(s("b")) || v1.Has(s("c")) || v1.Has(s("x")) {
		t.Fatalf("v1 changed: Len %d", v1.Len())
	}
	if r, _ := v1.Get(row(s("a"))); r[1].Str != "1" {
		t.Fatalf("v1's row a changed: %+v", r)
	}
	if v := tb.Snapshot(); v.Len() != 1 || !v.Has(s("x")) {
		t.Fatalf("the current Version: Len %d", v.Len())
	}
}

// TestHasEdges: Has is false for a composite key, a null, a wrong Kind and TInvalid.
func TestHasEdges(t *testing.T) {
	comp := newTable(t, "Pairs", []event.Attr{{Name: "tenant", Type: event.TString}, {Name: "host", Type: event.TString}}, "tenant", "host")
	ok(t, comp.Upsert(rows(row(s("a"), s("a")), row(s("b"), s("x")))))
	cv := comp.Snapshot()
	for _, k := range []event.Value{s("a"), s("b"), s("x")} {
		if cv.Has(k) {
			t.Errorf("Has(%q) on a composite-key table = true", k.Str)
		}
	}
	if _, found := cv.Get(row(s("a"), s("a"))); !found {
		t.Error("Get by the composite key misses")
	}
	if _, found := cv.Get(row(s("a"))); found {
		t.Error("Get with half a composite key found a row")
	}

	tb := keyed(t, event.TInt)
	ok(t, tb.Upsert(rows(row(i(0), s("zero")), row(i(1), s("one")))))
	v := tb.Snapshot()
	for name, k := range map[string]event.Value{
		"a null of the key's type":      null(event.TInt),
		"a null of the key's type, I 1": {Kind: event.TInt, I: 1, Null: true},
		"a string":                      s("1"),
		"a float of equal value":        f(1),
		"a timestamp of equal value":    ts(1),
		"a duration of equal value":     dur(1),
		"the zero Value":                {},
	} {
		if v.Has(k) {
			t.Errorf("Has(%s) = true", name)
		}
	}
	if !v.Has(i(0)) || !v.Has(i(1)) {
		t.Error("Has misses a stored int")
	}
}

// TestReadsAllocateNothing: Snapshot, Has and Len are allocation-free (design note §1).
func TestReadsAllocateNothing(t *testing.T) {
	str := keyed(t, event.TString)
	flt := keyed(t, event.TFloat)
	var many [][]event.Value
	for n := range 100 {
		many = append(many, row(s("host-"+strconv.Itoa(n)), s("x")))
	}
	ok(t, str.Upsert(many))
	ok(t, flt.Upsert(rows(row(f(1.5), s("x")))))
	hit, miss, fhit := s("host-42"), s("nope"), f(1.5)
	var got bool
	var n int
	checks := map[string]func(){
		"Snapshot":          func() { _ = str.Snapshot() },
		"Len":               func() { n = str.Snapshot().Len() },
		"Has (string hit)":  func() { got = str.Snapshot().Has(hit) },
		"Has (string miss)": func() { got = str.Snapshot().Has(miss) },
		"Has (float hit)":   func() { got = flt.Snapshot().Has(fhit) },
		"Has (wrong Kind)":  func() { got = str.Snapshot().Has(fhit) },
	}
	for name, fn := range checks {
		if allocs := testing.AllocsPerRun(1000, fn); allocs != 0 {
			t.Errorf("%s allocated %v times per run", name, allocs)
		}
	}
	_, _ = got, n
}
