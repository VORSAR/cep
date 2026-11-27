package table_test

// table.Set and table.Snapshot (design note s3-2a-1 §2; owner D1 of S3.1-2): the per-event snapshot carrier. The same
// *Snapshot until a table publishes a Version, then a new one; no allocation when nothing changed; a Snapshot is
// immutable — it keeps the Versions it was taken with. Release empties the tables and keeps their declarations, so the
// rows go once no earlier Snapshot is held (owner, 2026-11-27; PR 3's review PR3A-2).

import (
	"runtime"
	"slices"
	"testing"
	"weak"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

func newKeyTable(t *testing.T, name string) *table.Table {
	t.Helper()
	tb, err := table.New(name, []event.Attr{{Name: "k", Type: event.TString}}, []string{"k"})
	if err != nil {
		t.Fatalf("table.New(%s): %v", name, err)
	}
	return tb
}

func keyRow(k string) [][]event.Value { return [][]event.Value{{{Kind: event.TString, Str: k}}} }

// keyRows is one row per key, in order.
func keyRows(ks ...string) [][]event.Value {
	rows := make([][]event.Value, 0, len(ks))
	for _, k := range ks {
		rows = append(rows, []event.Value{{Kind: event.TString, Str: k}})
	}
	return rows
}

func TestSetSnapshotReuseAndRenewal(t *testing.T) {
	t.Parallel()
	t0, t1 := newKeyTable(t, "T0"), newKeyTable(t, "T1")
	set := table.NewSet([]*table.Table{t0, t1})
	if set.Len() != 2 || set.Table(0) != t0 || set.Table(1) != t1 {
		t.Fatalf("Set: Len %d; want 2 holding T0, T1 by id", set.Len())
	}
	s1 := set.Snapshot()
	if s1 == nil || s1.Len() != 2 {
		t.Fatalf("Snapshot: %v, want 2 tables", s1)
	}
	if s1.Table(0) != t0.Snapshot() || s1.Table(1) != t1.Snapshot() {
		t.Fatal("Snapshot's Versions are not the tables' current ones")
	}
	if s2 := set.Snapshot(); s2 != s1 {
		t.Fatal("Snapshot with no write since the last: want the same pointer")
	}
	// A refused write publishes nothing (Upsert is all or nothing; a null key is refused): still the same pointer.
	if err := t1.Upsert([][]event.Value{{{Kind: event.TString, Null: true}}}); err == nil {
		t.Fatal("Upsert of a null key: want an error")
	}
	if s := set.Snapshot(); s != s1 {
		t.Fatal("Snapshot after a refused write: want the same pointer")
	}
	old1 := t1.Snapshot()
	if err := t1.Upsert(keyRow("x")); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	s3 := set.Snapshot()
	if s3 == s1 {
		t.Fatal("Snapshot after T1 published a write: want a new pointer")
	}
	if s3.Table(1) != t1.Snapshot() || s3.Table(1) == old1 || s3.Table(0) != s1.Table(0) {
		t.Error("new Snapshot: want T1's new Version and T0's unchanged one")
	}
	// s1 is immutable: it still reads T1 as it was.
	if s1.Table(1) != old1 || s1.Table(1).Has(event.Value{Kind: event.TString, Str: "x"}) {
		t.Error("the earlier Snapshot changed after a later write")
	}
	if !s3.Table(1).Has(event.Value{Kind: event.TString, Str: "x"}) {
		t.Error("the new Snapshot does not see the write")
	}
	if s4 := set.Snapshot(); s4 != s3 {
		t.Error("Snapshot after no further write: want the same pointer as the last")
	}
	// A write to T0, then a delete in T1: each is a new pointer.
	if err := t0.Upsert(keyRow("y")); err != nil {
		t.Fatal(err)
	}
	s5 := set.Snapshot()
	if s5 == s3 || s5.Table(0) != t0.Snapshot() || s5.Table(1) != s3.Table(1) {
		t.Error("after T0's write: want a new Snapshot with T0's new Version and T1's kept")
	}
	if err := t1.Delete(keyRow("x")); err != nil {
		t.Fatal(err)
	}
	if s6 := set.Snapshot(); s6 == s5 || s6.Table(1).Has(event.Value{Kind: event.TString, Str: "x"}) {
		t.Error("after T1's delete: want a new Snapshot without x")
	}
}

// Snapshot allocates nothing when no table has published since the last call (design §2: a steady-state ingest step).
func TestSetSnapshotAllocatesNothingWhenUnchanged(t *testing.T) {
	t0, t1 := newKeyTable(t, "T0"), newKeyTable(t, "T1")
	set := table.NewSet([]*table.Table{t0, t1})
	if err := t0.Upsert(keyRow("a")); err != nil {
		t.Fatal(err)
	}
	set.Snapshot()
	var s *table.Snapshot
	if got := testing.AllocsPerRun(1000, func() { s = set.Snapshot() }); got != 0 {
		t.Errorf("Snapshot with nothing changed: %v allocations, want 0", got)
	}
	if s.Len() != 2 {
		t.Errorf("Len %d, want 2", s.Len())
	}
}

// Snapshot.Table panics out of range, like a slice index; an empty Set gives an empty, non-nil Snapshot.
func TestSnapshotRangeAndEmptySet(t *testing.T) {
	t.Parallel()
	set := table.NewSet([]*table.Table{newKeyTable(t, "T0")})
	s := set.Snapshot()
	for _, id := range []int{-1, 1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Snapshot.Table(%d) of 1 table: want a panic", id)
				}
			}()
			s.Table(id)
		}()
	}
	empty := table.NewSet(nil)
	if es := empty.Snapshot(); es == nil || es.Len() != 0 || empty.Len() != 0 {
		t.Error("an empty Set: want a non-nil Snapshot of 0 tables")
	}
}

// Release frees the rows of every table: a row stored before it, in T0 or in T1, is unreachable once no Snapshot taken
// before it is held, the Set's own cached one included. The control: each row is held before Release (by its table's
// current Version and the cached Snapshot).
func TestReleaseFreesTheRowsOnceNoEarlierSnapshotIsHeld(t *testing.T) {
	t.Parallel()
	t0, t1 := newKeyTable(t, "T0"), newKeyTable(t, "T1")
	set := table.NewSet([]*table.Table{t0, t1})
	if err := t0.Upsert(keyRows("k1", "k2", "k3")); err != nil {
		t.Fatal(err)
	}
	if err := t1.Upsert(keyRows("m1", "m2")); err != nil {
		t.Fatal(err)
	}
	wp0, wp1 := weakRow(t, set, 0, "k1"), weakRow(t, set, 1, "m1")
	runtime.GC()
	if wp0.Value() == nil || wp1.Value() == nil {
		t.Fatalf("before Release: T0's row k1 held %v, T1's row m1 held %v; want both held", wp0.Value() != nil,
			wp1.Value() != nil)
	}
	set.Release()
	runtime.GC()
	if wp0.Value() != nil {
		t.Error("T0's row k1 after Release: held; want freed (no Snapshot taken before Release is held)")
	}
	if wp1.Value() != nil {
		t.Error("T1's row m1 after Release: held; want freed (Release empties every table, not only the first)")
	}
	runtime.KeepAlive(set) // the Set itself stays reachable throughout: Release frees the rows, not the Set
}

// weakRow takes set's Snapshot, which fills the Set's cache, and returns a weak pointer to the stored row k of table
// id. It is taken here, so no slot of the test's own frame keeps the row alive.
func weakRow(t *testing.T, set *table.Set, id int, k string) weak.Pointer[event.Value] {
	t.Helper()
	row, ok := set.Snapshot().Table(id).Get([]event.Value{{Kind: event.TString, Str: k}})
	if !ok {
		t.Fatalf("table %d row %s: not stored", id, k)
	}
	return weak.Make(&row[0])
}

// Release changes no declaration and no earlier Snapshot: Len, Table, Name and Attrs answer as before; the current view
// is a new one with every table empty; a Snapshot taken before it still reads its rows (3 in T0, 2 in T1); and a table
// stays writable and keyed: a row put after Release is found by its key.
func TestReleaseKeepsTheDeclarationsAndAnEarlierSnapshot(t *testing.T) {
	t.Parallel()
	t0, t1 := newKeyTable(t, "T0"), newKeyTable(t, "T1")
	set := table.NewSet([]*table.Table{t0, t1})
	if err := t0.Upsert(keyRows("k1", "k2", "k3")); err != nil {
		t.Fatal(err)
	}
	if err := t1.Upsert(keyRows("m1", "m2")); err != nil {
		t.Fatal(err)
	}
	before := set.Snapshot()
	set.Release()
	if set.Len() != 2 || set.Table(0) != t0 || set.Table(1) != t1 {
		t.Fatalf("Set after Release: Len %d; want 2 holding T0, T1 by id", set.Len())
	}
	attrs := []event.Attr{{Name: "k", Type: event.TString}}
	for i, name := range []string{"T0", "T1"} {
		if tb := set.Table(i); tb.Name() != name || !slices.Equal(tb.Attrs(), attrs) {
			t.Errorf("table %d after Release: %s %v; want %s %v", i, tb.Name(), tb.Attrs(), name, attrs)
		}
	}
	now := set.Snapshot()
	if now == before {
		t.Error("Snapshot after Release: the earlier pointer; want a new one")
	}
	for i := range 2 {
		if n := now.Table(i).Len(); n != 0 {
			t.Errorf("table %d after Release: %d rows; want 0", i, n)
		}
	}
	for i, want := range []int{3, 2} {
		if n := before.Table(i).Len(); n != want {
			t.Errorf("the Snapshot taken before Release: %d rows in table %d; want %d", n, i, want)
		}
	}
	for i, k := range []string{"k1", "m1"} {
		if row, ok := before.Table(i).Get([]event.Value{{Kind: event.TString, Str: k}}); !ok || row[0].Str != k {
			t.Errorf("the Snapshot taken before Release: table %d Get(%s) = %v, %v; want the row %s", i, k, row, ok, k)
		}
	}
	if err := t0.Upsert(keyRow("k4")); err != nil {
		t.Fatalf("Upsert after Release: %v; want nil", err)
	}
	after := set.Snapshot().Table(0)
	if n := after.Len(); n != 1 {
		t.Errorf("T0 after Release and one Upsert: %d rows; want 1", n)
	}
	k4 := event.Value{Kind: event.TString, Str: "k4"}
	if row, ok := after.Get([]event.Value{k4}); !ok || row[0].Str != "k4" || !after.Has(k4) {
		t.Errorf("T0 after Release and one Upsert: Get(k4) = %v, %v, Has(k4) %v; want the row k4 found by its key", row,
			ok, after.Has(k4))
	}
}
