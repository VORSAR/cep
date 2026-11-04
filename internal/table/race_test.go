package table_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// generation g's content is the rows k = 0 … size(g)-1, each with gen = g. Every successful write in the test
// publishes exactly one generation's content, so a Version is consistent iff it is one generation's content whole.
func size(g int64) int { return 1 + int(g%5) }

func genRows(g int64) [][]event.Value {
	out := make([][]event.Value, size(g))
	for k := range out {
		out[k] = []event.Value{{Kind: event.TInt, I: int64(k)}, {Kind: event.TInt, I: g}}
	}
	return out
}

// consistent reports why v is not one generation's whole content, or "" when it is. Failed writes use negative
// generations, so a Version showing one proves a failed write published something.
func consistent(v *table.Version) string {
	n := v.Len()
	if n == 0 {
		return "" // the empty Version New published
	}
	r0, ok := v.Get([]event.Value{{Kind: event.TInt, I: 0}})
	if !ok {
		return fmt.Sprintf("Len %d but no row 0", n)
	}
	g := r0[1].I
	if g < 0 {
		return fmt.Sprintf("generation %d comes from a failed write", g)
	}
	if n != size(g) {
		return fmt.Sprintf("generation %d has Len %d, want %d", g, n, size(g))
	}
	for k := range 6 {
		key := event.Value{Kind: event.TInt, I: int64(k)}
		has := v.Has(key)
		if has != (k < n) {
			return fmt.Sprintf("generation %d (Len %d): Has(%d) = %v", g, n, k, has)
		}
		if r, ok := v.Get([]event.Value{key}); ok && r[1].I != g {
			return fmt.Sprintf("generation %d: row %d is from generation %d", g, k, r[1].I)
		}
	}
	return ""
}

// TestConcurrentWritersAndReaders (design note §1 and §2; run under -race): readers load Versions while writers
// Load, Upsert and fail; every Version a reader sees is one write's whole result, it never changes after the reader
// loaded it, and a failed write is never seen.
func TestConcurrentWritersAndReaders(t *testing.T) {
	tb, err := table.New("Gens", []event.Attr{{Name: "k", Type: event.TInt}, {Name: "gen", Type: event.TInt}}, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	const writers, readers, writes = 4, 4, 300
	var done atomic.Bool
	var seen atomic.Int64
	errs := make(chan string, readers+writers)
	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range writes {
				g := int64(w*writes + j + 1)
				var err error
				switch j % 3 {
				case 0:
					err = tb.Load(genRows(g))
				case 1:
					// An Upsert of all five keys is a whole generation whatever the table held before.
					g = g*5 + 4
					err = tb.Upsert(genRows(g))
				default:
					// A failed write: good rows of a negative generation, then a bad row.
					bad := [][]event.Value{
						{{Kind: event.TInt, I: 0}, {Kind: event.TInt, I: -g}},
						{{Kind: event.TInt, I: 1}, {Kind: event.TInt, I: -g}},
						{{Kind: event.TInt, I: 9}, {Kind: event.TString, Str: "x"}},
					}
					if tb.Upsert(bad) == nil || tb.Load(bad) == nil {
						errs <- "a write with a bad row was accepted"
						return
					}
					err = tb.Upsert(nil) // publishes nothing
				}
				if err != nil {
					errs <- fmt.Sprintf("writer %d: %v", w, err)
					return
				}
			}
		}()
	}
	var rg sync.WaitGroup
	for range readers {
		rg.Add(1)
		go func() {
			defer rg.Done()
			for !done.Load() {
				v := tb.Snapshot()
				if v == nil {
					errs <- "Snapshot returned nil"
					return
				}
				if why := consistent(v); why != "" {
					errs <- why
					return
				}
				n := v.Len()
				first, _ := v.Get([]event.Value{{Kind: event.TInt, I: 0}})
				var g int64 = -1
				if first != nil {
					g = first[1].I
				}
				_ = tb.Snapshot().Len() // let writers move on, then re-read v
				if v.Len() != n {
					errs <- fmt.Sprintf("a loaded Version changed: Len %d then %d", n, v.Len())
					return
				}
				if again, _ := v.Get([]event.Value{{Kind: event.TInt, I: 0}}); first != nil && again[1].I != g {
					errs <- fmt.Sprintf("a loaded Version changed: generation %d then %d", g, again[1].I)
					return
				}
				seen.Add(1)
			}
		}()
	}
	wg.Wait()
	done.Store(true)
	rg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if why := consistent(tb.Snapshot()); why != "" || tb.Snapshot().Len() == 0 {
		t.Fatalf("the final Version: %q, Len %d", why, tb.Snapshot().Len())
	}
	if seen.Load() == 0 {
		t.Log("the readers loaded no Version while the writers ran") // not a failure: the scheduler decides
	}
}
