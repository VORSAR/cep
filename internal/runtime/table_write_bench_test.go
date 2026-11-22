package runtime

// The table-write path's measure: BenchmarkTableWritePath keeps its name and world, so a later change to the write
// path is measured against the same thing.

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// BenchmarkTableWritePath times one TableWrite of one row (the same key each time, so the table keeps one row) on a
// live app with no recording (owner, 2026-11-22): the caller's checks, the intake, the router's write step and the
// reply.
func BenchmarkTableWritePath(b *testing.B) { benchTableWrite(b, nil) }

// BenchmarkTableWritePathRecorded is the same write with a recording that discards (igRec): the record per row is
// timed too. A recording that fell behind and failed would measure another path, so the run fails then; at one CPU
// it passes only because the router gives the recorder its turn (design s1-1c-7-design-starvation).
func BenchmarkTableWritePathRecorded(b *testing.B) { benchTableWrite(b, igRec{}) }

func benchTableWrite(b *testing.B, rec Recorder) {
	ctx := cgCtx(b)
	r, err := New(Config{Shards: 1, Now: func() int64 { return cgT0 }})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = r.Close(ctx) })
	tb, err := table.New("T", []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}}, []string{"k"})
	if err != nil {
		b.Fatal(err)
	}
	app := cgApp(b, func(event.Event) error { return nil })
	app.Sinks, app.Recorder = nil, rec
	app.Tables = table.NewSet([]*table.Table{tb})
	id, err := r.Add(ctx, app)
	if err != nil {
		b.Fatal(err)
	}
	if err := r.Activate(ctx, id); err != nil {
		b.Fatal(err)
	}
	rows := [][]event.Value{{{Kind: event.TString, Str: "k"}, {Kind: event.TInt, I: 1}}}
	b.ReportAllocs()
	for b.Loop() { // the timer stops when it returns false: the Stats read below is not timed
		if err := r.TableWrite(ctx, id, 0, TablePut, rows); err != nil {
			b.Fatal(err)
		}
	}
	if rec == nil {
		return
	}
	st, err := r.Stats(ctx)
	if err != nil {
		b.Fatal(err)
	}
	if rs := st.Recording[id]; rs.Failed {
		b.Fatalf("recording %+v: it fell behind, so the run measured another path", rs)
	}
}
