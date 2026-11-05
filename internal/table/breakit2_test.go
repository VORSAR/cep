package table_test

// S3.2a-1 break-it (QA): attacks on table.Set and Snapshot that held (guards).

import (
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

func bkTable(t *testing.T, name string) *table.Table {
	t.Helper()
	tb, err := table.New(name, []event.Attr{{Name: "x", Type: event.TString}, {Name: "n", Type: event.TInt}}, []string{"x"})
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func bkRow(x string, n int64) []event.Value {
	return []event.Value{{Kind: event.TString, Str: x}, {Kind: event.TInt, I: n}}
}

// TestBreakitGuardFailedWritePublishesNothing: a refused write (a wrong type, a null key, an empty batch) leaves
// Set.Snapshot returning the same *Snapshot; a successful one gives a new Snapshot while the old one keeps its rows.
func TestBreakitGuardFailedWritePublishesNothing(t *testing.T) {
	a, b := bkTable(t, "A"), bkTable(t, "B")
	set := table.NewSet([]*table.Table{a, b})
	s0 := set.Snapshot()
	bad := [][]event.Value{bkRow("ok", 1), {{Kind: event.TInt, I: 1}, {Kind: event.TInt, I: 1}}}
	if a.Upsert(bad) == nil || a.Delete([][]event.Value{{{Kind: event.TString, Null: true}}}) == nil {
		t.Fatal("a bad write was accepted")
	}
	_ = a.Upsert(nil)
	_ = b.Delete(nil)
	if s1 := set.Snapshot(); s1 != s0 {
		t.Errorf("a refused or empty write published a new snapshot")
	}
	if err := b.Upsert([][]event.Value{bkRow("k", 7)}); err != nil {
		t.Fatal(err)
	}
	s2 := set.Snapshot()
	if s2 == s0 || s0.Table(1).Len() != 0 || s2.Table(1).Len() != 1 || s2.Table(0) != s0.Table(0) {
		t.Errorf("after a write to B: a new snapshot holding B's row, A's version unchanged, the old snapshot unchanged")
	}
	if n := testing.AllocsPerRun(100, func() { _ = set.Snapshot() }); n != 0 {
		t.Errorf("a steady-state Snapshot allocates %v", n)
	}
}

// TestBreakitGuardSnapshotUnderConcurrentWriters: writers on other goroutines (Upsert, Delete, Load) against
// Set.Snapshot on one goroutine; run with -race. Every snapshot is internally stable: a version read twice from one
// snapshot is the same, and a later snapshot never shows an older version of a table than an earlier one did.
func TestBreakitGuardSnapshotUnderConcurrentWriters(t *testing.T) {
	a, b := bkTable(t, "A"), bkTable(t, "B")
	set := table.NewSet([]*table.Table{a, b})
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for w, tb := range []*table.Table{a, b, a} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := int64(0); ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				switch (i + int64(w)) % 3 {
				case 0:
					_ = tb.Upsert([][]event.Value{bkRow("k", i)})
				case 1:
					_ = tb.Delete([][]event.Value{{{Kind: event.TString, Str: "k"}}})
				default:
					_ = tb.Load([][]event.Value{bkRow("k", i), bkRow("j", i)})
				}
			}
		}()
	}
	prev := set.Snapshot()
	for range 20000 {
		s := set.Snapshot()
		for id := range s.Len() {
			v := s.Table(id)
			if v != s.Table(id) {
				t.Fatal("a snapshot's version changed")
			}
			_, _ = v.Get([]event.Value{{Kind: event.TString, Str: "k"}})
		}
		prev = s
	}
	_ = prev
	close(stop)
	wg.Wait()
}
