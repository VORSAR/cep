package runtime

// The table-write path's measure: BenchmarkTableWritePath keeps its name and world, so a later change to the write
// path is measured against the same thing.

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// BenchmarkTableWritePath times one TableWrite of one row (the same key each time, so the table keeps one row) on a
// live app with a recording: the caller's checks, the intake, the router's write step, the record per row and the
// reply. The recorder discards (igRec); a recording that fell behind and failed would measure another path, so the
// run fails then.
func BenchmarkTableWritePath(b *testing.B) {
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
	app.Sinks, app.Recorder = nil, igRec{}
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
	for b.Loop() { // the timer starts at its first call and stops when it returns false: the Stats read below is not timed
		if err := r.TableWrite(ctx, id, 0, TablePut, rows); err != nil {
			b.Fatal(err)
		}
	}
	st, err := r.Stats(ctx)
	if err != nil {
		b.Fatal(err)
	}
	if rs := st.Recording[id]; rs.Failed {
		b.Fatalf("recording %+v: it fell behind, so the run measured another path", rs)
	}
}
