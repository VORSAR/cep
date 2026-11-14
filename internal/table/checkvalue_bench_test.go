package table

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// checkValueWorld is one table with a column of each checked kind and a valid row for it (design s1-1c-5 §7.5,
// DQ-S12: checkValue is on the runtime's table-write path, appState.write -> Table.Upsert/Delete -> prepare/checkKey).
func checkValueWorld(tb testing.TB) (*Table, []event.Value) {
	tb.Helper()
	tab, err := New("t", []event.Attr{
		{Name: "id", Type: event.TString},
		{Name: "n", Type: event.TInt},
		{Name: "f", Type: event.TFloat},
		{Name: "o", Type: event.TObject, Optional: true},
		{Name: "b", Type: event.TBool, Optional: true},
	}, []string{"id"})
	if err != nil {
		tb.Fatal(err)
	}
	row := []event.Value{
		{Kind: event.TString, Str: "k"},
		{Kind: event.TInt, I: 7},
		{Kind: event.TFloat, F: 1.5},
		{Kind: event.TObject, Obj: `{"a":1}`},
		{Kind: event.TBool, Null: true},
	}
	return tab, row
}

// Not parallel: testing.AllocsPerRun refuses to run in a parallel test.
func TestCheckValueAllocs(t *testing.T) {
	tab, row := checkValueWorld(t)
	var failed error
	// Pinned at today's value: a value that passes builds no error, so the check allocates nothing.
	got := testing.AllocsPerRun(100, func() {
		for j, v := range row {
			if err := tab.checkValue(j, v); err != nil {
				failed = err
			}
		}
	})
	if failed != nil {
		t.Fatalf("a valid row was refused: %v", failed)
	}
	if got != 0 {
		t.Fatalf("checkValue allocated %v times per row, want 0", got)
	}
}

func BenchmarkTableCheckValue(b *testing.B) {
	tab, row := checkValueWorld(b)
	b.ReportAllocs()
	for b.Loop() {
		for j, v := range row {
			if err := tab.checkValue(j, v); err != nil {
				b.Fatal(err)
			}
		}
	}
}
