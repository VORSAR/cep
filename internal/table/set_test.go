package table_test

// table.Set and table.Snapshot (design note s3-2a-1 §2; owner D1 of S3.1-2): the per-event snapshot carrier. The same
// *Snapshot until a table publishes a Version, then a new one; no allocation when nothing changed; a Snapshot is
// immutable — it keeps the Versions it was taken with.

import (
	"testing"

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
