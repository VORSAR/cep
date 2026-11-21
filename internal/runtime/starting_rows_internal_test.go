package runtime

// An app's starting rows (App.Loads): checked at Add, refused by build, put in the router's activation step, and
// recorded in a queue sized for them.

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// startingRowsTable is a table name (k string, the key; v int) for the starting-rows tests.
func startingRowsTable(t testing.TB, name string) *table.Table {
	t.Helper()
	tb, err := table.New(name, []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}}, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

// startingRows is n rows of T: k1 → 1, k2 → 2, ….
func startingRows(n int) [][]event.Value {
	rows := make([][]event.Value, n)
	for i := range rows {
		rows[i] = []event.Value{{Kind: event.TString, Str: fmt.Sprintf("k%d", i+1)}, {Kind: event.TInt, I: int64(i + 1)}}
	}
	return rows
}

// The starting rows an App carries are put in the router's activation step: none after Add, all of them when the
// activation is answered (the activateHold seam, before the flushers and the recorder start).
func TestActivateAppliesTheStartingRowsInItsStep(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	tb := startingRowsTable(t, "T")
	app := cgApp(t, func(event.Event) error { return nil })
	app.Tables = table.NewSet([]*table.Table{tb})
	app.Loads = []TableLoad{{Table: 0, Rows: startingRows(2)}}
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if n := tb.Snapshot().Len(); n != 0 {
		t.Fatalf("rows in T after Add: %d; want 0 (nothing is put before the activation step)", n)
	}
	answered := -1
	r.activateHold = func() { answered = tb.Snapshot().Len() }
	if err := r.Activate(ctx, id); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if answered != 2 {
		t.Errorf("rows in T when the activation was answered: %d; want 2 (put in the activation step)", answered)
	}
	if row, ok := tb.Snapshot().Get([]event.Value{{Kind: event.TString, Str: "k1"}}); !ok || row[1].I != 1 {
		t.Errorf("T's row k1 after Activate: %v, %v; want [k1 1]", row, ok)
	}
}

// Add refuses starting rows the activation step could not put, before anything is installed: a table the app does
// not have, a row the table refuses, a row a recording cannot copy.
func TestAddRefusesStartingRowsItCouldNotApply(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	bad := "a\xffb"
	if utf8.ValidString(bad) {
		t.Fatal("the malformed input is valid UTF-8")
	}
	for name, c := range map[string]struct {
		load TableLoad
		want string
	}{
		"no table":     {TableLoad{Table: 1, Rows: startingRows(1)}, "starting rows for table 1: the app has no such table"},
		"negative":     {TableLoad{Table: -1, Rows: startingRows(1)}, "starting rows for table -1: the app has no such table"},
		"short row":    {TableLoad{Rows: [][]event.Value{{{Kind: event.TString, Str: "k"}}}}, "table T: row 0: 1 values for 2 attributes"},
		"null key":     {TableLoad{Rows: [][]event.Value{{{Kind: event.TString, Null: true}, {Kind: event.TInt, I: 1}}}}, `table T: row 0: attribute 0 "k": a key value cannot be null`},
		"unrecordable": {TableLoad{Rows: [][]event.Value{{{Kind: event.TString, Str: bad}, {Kind: event.TInt, I: 1}}}}, "holds malformed UTF-8"},
	} {
		app := cgApp(t, func(event.Event) error { return nil })
		app.Tables = table.NewSet([]*table.Table{startingRowsTable(t, "T")})
		app.Loads = []TableLoad{c.load}
		if _, err := r.Add(ctx, app); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Add: %v; want an error holding %q", name, err, c.want)
		}
	}
	if n := len(r.appList()); n != 0 {
		t.Errorf("%d app slots after the refused Adds; want 0 (refused before the router)", n)
	}
}

// A built app is never activated, so it takes no starting rows (a replay's come from its recording).
func TestABuiltAppTakesNoStartingRows(t *testing.T) {
	t.Parallel()
	app := cgApp(t, func(event.Event) error { return nil })
	app.Tables = table.NewSet([]*table.Table{startingRowsTable(t, "T")})
	app.Loads = []TableLoad{{Table: 0, Rows: startingRows(1)}}
	if _, err := build(Config{Shards: 1}, []App{app}); err == nil || !strings.Contains(err.Error(), "a built app is never activated") {
		t.Fatalf("build with starting rows: %v; want the refusal", err)
	}
}

// With starting rows, an app's recording queue is RecordQueue plus one slot per row: at RecordQueue 4, two tables of
// 3 rows each (the header and 6 rows, 7 records queued before the recorder starts) record in full, and every line is
// counted (the header, the 6 rows, the stop).
func TestStartingRowsBeyondRecordQueueRecordInFull(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r, err := New(Config{Shards: 1, SinkQueue: 4, RecordQueue: 4, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ldClose(t, r) })
	h := newHeldFlushRecorder(nil)
	h.let()
	ta, tb := startingRowsTable(t, "A"), startingRowsTable(t, "B")
	app := cgApp(t, func(event.Event) error { return nil })
	app.Sinks, app.Recorder = nil, h
	app.Tables = table.NewSet([]*table.Table{ta, tb})
	app.Loads = []TableLoad{{Table: 0, Rows: startingRows(3)}, {Table: 1, Rows: startingRows(3)}}
	id := ldStarted(t, ctx, r, app)
	if _, err := r.Drain(ctx, id); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rs := st.Recording[id]; rs.Failed || rs.Records != 8 {
		t.Errorf("recording %+v; want not Failed and 8 records (the header, 6 starting rows, the stop)", rs)
	}
	if na, nb := ta.Snapshot().Len(), tb.Snapshot().Len(); na != 3 || nb != 3 {
		t.Errorf("rows in A %d, B %d; want 3 and 3", na, nb)
	}
}
