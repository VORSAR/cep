package playback

import (
	"testing"
)

// TestTraceNamesAnUnknownTableByItsId: the operands of trace.go:95's `s.Table >= 0 && s.Table < len(...)` — a table
// id outside the app is written "#<id>", a known one by its name.
func TestTraceNamesAnUnknownTableByItsId(t *testing.T) {
	t.Parallel()
	tr := NewTrace(cnTableApp())
	tr.Step(&Step{Kind: StepTable, Line: 1, Table: -1, Op: OpPut})
	tr.Step(&Step{Kind: StepTable, Line: 2, Table: 1, Op: OpDelete})
	tr.Step(&Step{Kind: StepTable, Line: 3, Table: 2, Op: OpPut})
	got, err := tr.Bytes()
	want := `{"at":1,"k":"table","op":"put","table":"#-1"}` + "\n" + `{"at":2,"k":"table","op":"delete","table":"U"}` + "\n" +
		`{"at":3,"k":"table","op":"put","table":"#2"}` + "\n"
	if err != nil || string(got) != want {
		t.Errorf("the trace = %q, %v; want %q", got, err, want)
	}
}
