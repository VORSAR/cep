package playback

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

func cnStr(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }

// cnTableApp is an app of one table T(k string, v int) keyed by k, and a second table U(a string, b int, c int) keyed
// by (c, a): a key written in another order than its columns.
func cnTableApp() *plan.App {
	return &plan.App{Tables: []plan.Table{
		{Name: "T", Attrs: []event.Attr{{Name: "k", Type: event.TString}, {Name: "v", Type: event.TInt}}, Key: []string{"k"}},
		{Name: "U", Attrs: []event.Attr{{Name: "a", Type: event.TString}, {Name: "b", Type: event.TInt}, {Name: "c",
			Type: event.TInt}}, Key: []string{"c", "a"}},
	}}
}

// TestRecorderTableRefusesAnUnknownTable: the operands of recorder.go:151's `table < 0 || table >= len(...)`.
func TestRecorderTableRefusesAnUnknownTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		table int
		want  string
	}{{-1, "playback: recorder: table -1 of 2"}, {2, "playback: recorder: table 2 of 2"}}
	for _, c := range cases {
		var buf bytes.Buffer
		err := NewRecorder(&buf, cnTableApp()).Table(c.table, runtime.TablePut, [][]event.Value{{cnStr("a"), cnInt(1)}})
		if err == nil || err.Error() != c.want {
			t.Errorf("Table(%d) = %v; want %q", c.table, err, c.want)
		}
		if buf.Len() != 0 {
			t.Errorf("Table(%d) wrote %q; want nothing", c.table, buf.String())
		}
	}
}

// TestRecorderTableWritesADeleteAsItsKeyColumns: a delete row holds the key columns in key order (recorder.go:156-164).
func TestRecorderTableWritesADeleteAsItsKeyColumns(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := NewRecorder(&buf, cnTableApp())
	if err := r.Table(0, runtime.TableDelete, [][]event.Value{{cnStr("a")}}); err != nil {
		t.Fatalf("Table(T, delete): %v", err)
	}
	if err := r.Table(1, runtime.TableDelete, [][]event.Value{{cnInt(3), cnStr("x")}}); err != nil {
		t.Fatalf("Table(U, delete): %v", err)
	}
	want := `{"op":"delete","row":{"k":"a"},"table":"T"}` + "\n" + `{"op":"delete","row":{"c":3,"a":"x"},"table":"U"}` + "\n"
	if buf.String() != want {
		t.Errorf("the delete lines = %q; want %q", buf.String(), want)
	}
}

// TestRecorderTableWritesEveryColumnOfAPut: a put row holds every column in declaration order, one line per row.
func TestRecorderTableWritesEveryColumnOfAPut(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := NewRecorder(&buf, cnTableApp()).Table(0, runtime.TablePut, [][]event.Value{{cnStr("a"), cnInt(1)},
		{cnStr("b"), cnInt(2)}})
	if err != nil {
		t.Fatalf("Table(T, put): %v", err)
	}
	want := `{"op":"put","row":{"k":"a","v":1},"table":"T"}` + "\n" + `{"op":"put","row":{"k":"b","v":2},"table":"T"}` + "\n"
	if buf.String() != want {
		t.Errorf("the put lines = %q; want %q", buf.String(), want)
	}
}

// TestRecorderTableRefusesARowOfTheWrongWidth: a row whose width is not its op's column count (recorder.go:167-170).
func TestRecorderTableRefusesARowOfTheWrongWidth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		op   runtime.TableOp
		row  []event.Value
		want string
	}{
		{runtime.TablePut, []event.Value{cnStr("a")}, `playback: recorder: table "T": a put row of 1 values for 2 columns`},
		{runtime.TableDelete, []event.Value{cnStr("a"), cnInt(1)},
			`playback: recorder: table "T": a delete row of 2 values for 1 columns`},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		err := NewRecorder(&buf, cnTableApp()).Table(0, c.op, [][]event.Value{c.row})
		if err == nil || err.Error() != c.want {
			t.Errorf("Table(op %v, %d values) = %v; want %q", c.op, len(c.row), err, c.want)
		}
		if buf.Len() != 0 {
			t.Errorf("Table(op %v) wrote %q; want nothing", c.op, buf.String())
		}
	}
}

// TestRecorderTableRefusesAnUnencodableValue: a column AppendValue refuses fails the call with the column named
// (recorder.go:182-184), and nothing is written.
func TestRecorderTableRefusesAnUnencodableValue(t *testing.T) {
	t.Parallel()
	app := &plan.App{Tables: []plan.Table{{Name: "F", Attrs: []event.Attr{{Name: "k", Type: event.TString},
		{Name: "f", Type: event.TFloat}}, Key: []string{"k"}}}}
	var buf bytes.Buffer
	err := NewRecorder(&buf, app).Table(0, runtime.TablePut, [][]event.Value{{cnStr("a"),
		{Kind: event.TFloat, F: math.NaN()}}})
	const want = `playback: recorder: table "F" column "f": a NaN or infinite float is not encodable`
	if err == nil || err.Error() != want || !errors.Is(err, errNonFinite) {
		t.Errorf("Table(NaN column) = %v; want %q wrapping errNonFinite", err, want)
	}
	if buf.Len() != 0 {
		t.Errorf("Table(NaN column) wrote %q; want nothing", buf.String())
	}
}

// TestRecorderTableStopsAtTheFirstFailedWrite: a failed write is the call's error, and no later row is tried
// (recorder.go:188-190).
func TestRecorderTableStopsAtTheFirstFailedWrite(t *testing.T) {
	t.Parallel()
	w := &cnFailWriter{err: errCnWrite}
	err := NewRecorder(w, cnTableApp()).Table(0, runtime.TablePut, [][]event.Value{{cnStr("a"), cnInt(1)},
		{cnStr("b"), cnInt(2)}})
	if !errors.Is(err, errCnWrite) {
		t.Errorf("Table(failing writer) = %v; want the writer's error", err)
	}
	if w.writes != 1 {
		t.Errorf("Table(failing writer) wrote %d times; want 1", w.writes)
	}
}

// TestRecorderTableStopsAtTheFirstBadRow: a good row, a row of the wrong width, a good row: exactly the first line is
// written and the call fails on the second.
func TestRecorderTableStopsAtTheFirstBadRow(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := NewRecorder(&buf, cnTableApp()).Table(0, runtime.TablePut, [][]event.Value{{cnStr("a"), cnInt(1)},
		{cnStr("b")}, {cnStr("c"), cnInt(3)}})
	const want = `playback: recorder: table "T": a put row of 1 values for 2 columns`
	if err == nil || err.Error() != want {
		t.Errorf("Table(good, bad, good) = %v; want %q", err, want)
	}
	if line := `{"op":"put","row":{"k":"a","v":1},"table":"T"}` + "\n"; buf.String() != line {
		t.Errorf("Table(good, bad, good) wrote %q; want exactly %q", buf.String(), line)
	}
}
