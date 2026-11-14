package playback

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

// TestRecordsDoNotAlias (S1.1c-5 design §7.1, R-4): the Recorder reuses one buffer for every line, so each record
// must start from the buffer emptied, whatever it holds. Between records the test changes the reused buffer (its
// length and its bytes); every record after the change is exactly its own line, and the records before it are
// unchanged. Each of the six sites that start a line from the reused buffer (Header, Event, Rejected, Tick, Table,
// BatchEnd) writes one record after a change. The expected lines are the recording format (design s3-2b-2 §7),
// written out by hand.
func TestRecordsDoNotAlias(t *testing.T) {
	t.Parallel()
	schema, err := event.NewSchema("S", []event.Attr{{Name: "k", Type: event.TString}, {Name: "ts", Type: event.TTimestamp}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	app := &plan.App{
		Streams: []plan.Stream{{Schema: schema}},
		Tables:  []plan.Table{{Name: "T", Attrs: []event.Attr{{Name: "k", Type: event.TString}}, Key: []string{"k"}}},
	}
	var out bytes.Buffer
	r := NewRecorder(&out, app)
	change := func() { r.buf = append(r.buf[:0], "GARBAGE"...) }
	const t0 = 1767261600000 // 2026-01-01T10:00:00.000Z
	steps := []struct {
		name string
		do   func() error
	}{
		{"Header", func() error { return r.Header(t0) }},
		{"Event", func() error {
			e := event.Event{StreamID: 0, Ts: t0}
			return r.Event(&e, t0, []byte(`{"k":"a","ts":"2026-01-01T10:00:00.000Z"}`))
		}},
		{"Rejected", func() error {
			return r.Rejected(0, t0, &event.DecodeError{Reason: event.DecodeMissing, Attr: "k"})
		}},
		{"Tick", func() error { return r.Tick(t0 + 1000) }},
		{"Table", func() error {
			return r.Table(0, runtime.TablePut, [][]event.Value{{{Kind: event.TString, Str: "a"}}})
		}},
		{"BatchEnd", func() error { return r.BatchEnd(0, "b1") }},
	}
	for i, s := range steps {
		if i > 0 {
			change()
		}
		if err := s.do(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
	}
	want := strings.Join([]string{
		`{"recording":{"start":"2026-01-01T10:00:00.000Z"}}`,
		`{"arrival":"2026-01-01T10:00:00.000Z","attrs":{"k":"a","ts":"2026-01-01T10:00:00.000Z"},"stream":"S"}`,
		`{"arrival":"2026-01-01T10:00:00.000Z","rejected":{"attr":"k","reason":"missing"},"stream":"S"}`,
		`{"tick":"2026-01-01T10:00:01.000Z"}`,
		`{"op":"put","row":{"k":"a"},"table":"T"}`,
		`{"batch_end":"b1","stream":"S"}`,
	}, "\n") + "\n"
	if got := out.String(); got != want {
		t.Errorf("the recording after a change to the reused buffer between records:\n got %s\nwant %s", got, want)
	}
}
