package table_test

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// TestCheckRefusesWhatUpsertRefusesAndChangesNothing: for each kind of row Upsert and Load refuse, Check returns
// their error word for word, naming the first refused row; and no Check, refusing or accepting, publishes a Version.
func TestCheckRefusesWhatUpsertRefusesAndChangesNothing(t *testing.T) {
	t.Parallel()
	// Checks(id optional string, n int, x optional float, meta optional object) primary key (id): New accepts an
	// optional key column, and a write refuses its null; n is a column that is not optional.
	attrs := []event.Attr{
		{Name: "id", Type: event.TString, Optional: true},
		{Name: "n", Type: event.TInt},
		{Name: "x", Type: event.TFloat, Optional: true},
		{Name: "meta", Type: event.TObject, Optional: true},
	}
	clean := func(id string) []event.Value { return row(s(id), i(1), null(event.TFloat), obj(`{"a":1}`)) }
	newChecks := func(t *testing.T) *table.Table {
		t.Helper()
		tb := newTable(t, "Checks", attrs, "id")
		ok(t, tb.Upsert(rows(clean("seed"))))
		return tb
	}
	writes := []struct {
		name  string
		write func(*table.Table, [][]event.Value) error
	}{
		{"Upsert", (*table.Table).Upsert},
		{"Load", (*table.Table).Load},
	}

	// Each refused row sits at index 1, after a clean row and before a row with no values, which is refused in other
	// words: the error must name the first refused row. Each wanted text is Upsert's wording, written out by hand from
	// its format strings.
	cases := []struct {
		name string
		bad  []event.Value
		want string
	}{
		{
			"a wrong value count",
			row(s("b"), i(1)),
			"table Checks: row 1: 2 values for 4 attributes",
		},
		{
			"a wrong kind",
			row(s("b"), s("1"), null(event.TFloat), null(event.TObject)),
			`table Checks: row 1: attribute 1 "n": want int, got string`,
		},
		{
			"a null key",
			row(null(event.TString), i(1), null(event.TFloat), null(event.TObject)),
			`table Checks: row 1: attribute 0 "id": a key value cannot be null`,
		},
		{
			"a null in a column that is not optional",
			row(s("b"), null(event.TInt), null(event.TFloat), null(event.TObject)),
			`table Checks: row 1: attribute 1 "n": null for an attribute that is not optional`,
		},
		{
			"a float that is not finite",
			row(s("b"), i(1), f(math.Inf(1)), null(event.TObject)),
			`table Checks: row 1: attribute 2 "x": the float +Inf is not finite`,
		},
		{
			"an object held as an int",
			row(s("b"), i(1), null(event.TFloat), event.Value{Kind: event.TObject, Obj: 42}),
			`table Checks: row 1: attribute 3 "meta": an object value is its JSON text, a string or []byte; got int`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			tb := newChecks(t)
			in := rows(clean("a"), c.bad, row())
			for _, w := range writes {
				if err := w.write(tb, in); err == nil || err.Error() != c.want {
					t.Fatalf("%s: got %v, want %q", w.name, err, c.want)
				}
			}
			before := tb.Snapshot()
			err := tb.Check(in)
			if after := tb.Snapshot(); after != before {
				t.Errorf("Check published a new Version")
			}
			if err == nil || err.Error() != c.want {
				t.Errorf("Check: got %v, want Upsert's %q", err, c.want)
			}
		})
	}

	t.Run("a clean batch", func(t *testing.T) {
		t.Parallel()
		tb := newChecks(t)
		in := rows(
			clean("a"),
			row(s("b"), i(2), f(1.5), event.Value{Kind: event.TObject, Obj: []byte(`[1]`)}),
			row(s("c"), i(3), null(event.TFloat), null(event.TObject)),
		)
		before := tb.Snapshot()
		if err := tb.Check(in); err != nil {
			t.Fatalf("Check refused a clean batch: %v", err)
		}
		if after := tb.Snapshot(); after != before {
			t.Fatalf("Check of a clean batch published a new Version")
		}
		// The witness that the batch could change the table: Upsert of the same rows publishes them.
		ok(t, tb.Upsert(in))
		if v := tb.Snapshot(); v == before || v.Len() != 4 {
			t.Fatalf("Upsert of the clean batch: same Version %v, %d rows; want a new Version with 4 rows",
				v == before, v.Len())
		}
	})
}
