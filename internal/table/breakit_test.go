package table_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// The S3.1-2 break-it pass over internal/table (design note docs/sprints/s3-1-2-design.md §1). Red tests name their
// finding; the rest are guards for what held.

// B5 (should-fix, design note §1: "The table copies every row it stores; a caller may reuse its slices after the
// call" and "A published Version MUST NOT change"): the copy is shallow. An object value's Obj is `any`, and the
// table accepts any dynamic type there (TestKeyEquality already passes a []byte), so a caller that reuses the byte
// slice it wrote, after Upsert or Load returned, rewrites the row inside a published Version.
func TestBreakitObjAliasingChangesPublishedVersion(t *testing.T) {
	for name, write := range map[string]func(*table.Table, [][]event.Value) error{
		"Upsert": (*table.Table).Upsert,
		"Load":   (*table.Table).Load,
	} {
		t.Run(name, func(t *testing.T) {
			tb := hosts(t)
			meta := []byte(`{"v":1}`)
			ok(t, write(tb, rows(row(s("a"), s("o"), null(event.TInt), event.Value{Kind: event.TObject, Obj: meta}))))
			v := tb.Snapshot()
			before, _ := v.Get(row(s("a")))
			was := fmt.Sprint(before[3].Obj)
			copy(meta, `{"v":2}`) // the caller reuses its buffer after the call returned
			after, _ := v.Get(row(s("a")))
			if now := fmt.Sprint(after[3].Obj); now != was {
				t.Fatalf("a published Version changed after the caller reused its object buffer: %s became %s", was, now)
			}
		})
	}
}

// Guard (held): composite-key encoding is injective per schema and keeps IEEE equality. Random keys over values
// chosen to collide (strings holding the separators and the encodings of the other types, -0 and +0, int, timestamp
// and duration values equal as numbers) are checked against a reference that compares each column on its type's
// own field.
func TestBreakitCompositeKeysNeverCollide(t *testing.T) {
	attrs := []event.Attr{
		{Name: "a", Type: event.TString}, {Name: "b", Type: event.TString}, {Name: "c", Type: event.TBool},
		{Name: "d", Type: event.TString}, {Name: "e", Type: event.TFloat}, {Name: "g", Type: event.TInt},
		{Name: "h", Type: event.TTimestamp}, {Name: "j", Type: event.TDuration}, {Name: "n", Type: event.TInt},
	}
	key := []string{"a", "b", "c", "d", "e", "g", "h", "j"}
	strs := []string{"", ";", ":", "1:", "1:;", "true;", "false;", "i1;", "t1;", "d1;", "f0;", "0:", "2:ab", "ab", "a", "b", "1:a1:b", "\x00", "f8000000000000000;"}
	floats := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, math.MaxFloat64, math.SmallestNonzeroFloat64}
	ints := []int64{0, 1, -1, 10, math.MinInt64, math.MaxInt64}
	rng := rand.New(rand.NewPCG(7, 11))
	pick := func() []event.Value {
		return row(s(strs[rng.IntN(len(strs))]), s(strs[rng.IntN(len(strs))]), b(rng.IntN(2) == 0),
			s(strs[rng.IntN(len(strs))]), f(floats[rng.IntN(len(floats))]), i(ints[rng.IntN(len(ints))]),
			ts(ints[rng.IntN(len(ints))]), dur(ints[rng.IntN(len(ints))]), i(0))
	}
	same := func(x, y []event.Value) bool {
		return x[0].Str == y[0].Str && x[1].Str == y[1].Str && x[2].B == y[2].B && x[3].Str == y[3].Str &&
			x[4].F == y[4].F && x[5].I == y[5].I && x[6].T == y[6].T && x[7].D == y[7].D
	}
	tb := newTable(t, "Comp", attrs, key...)
	var stored [][]event.Value
	for n := range 3000 {
		r := pick()
		r[8] = i(int64(n))
		ok(t, tb.Upsert(rows(r)))
		kept := stored[:0]
		for _, old := range stored {
			if !same(old, r) {
				kept = append(kept, old)
			}
		}
		stored = append(kept, r)
	}
	v := tb.Snapshot()
	if v.Len() != len(stored) {
		t.Fatalf("Len = %d, the reference holds %d distinct keys", v.Len(), len(stored))
	}
	for _, r := range stored {
		got, found := v.Get(r[:8])
		if !found || got[8].I != r[8].I {
			t.Fatalf("Get(%+v) = %+v %v, want the row tagged %d", r[:8], got, found, r[8].I)
		}
	}
}

// Guard (held): Has never allocates, for every key type (design note §1).
func TestBreakitHasAllocatesNothingForEveryKeyType(t *testing.T) {
	for _, c := range []struct {
		typ event.Type
		v   event.Value
	}{
		{event.TString, s("k")}, {event.TInt, i(3)}, {event.TFloat, f(-0.5)}, {event.TBool, b(true)},
		{event.TTimestamp, ts(9)}, {event.TDuration, dur(-9)},
	} {
		tb := keyed(t, c.typ)
		ok(t, tb.Upsert(rows(row(c.v, s("x")))))
		var got bool
		if allocs := testing.AllocsPerRun(500, func() { got = tb.Snapshot().Has(c.v) }); allocs != 0 || !got {
			t.Errorf("%s: Has = %v, %v allocations per run", c.typ, got, allocs)
		}
	}
}
