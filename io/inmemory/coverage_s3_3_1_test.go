package inmemory_test

// S3.3-1's build: the lines QA's tests-first lane left uncovered in io/inmemory (the coverage row's 90% floor; design
// v2 §9: "the build adds a test only where QA's leave a line uncovered"). Prefix: cvi.

import (
	"context"
	"errors"
	"testing"

	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// cviRow is an OutEvent row over fixed values of every type, for the sink's per-type copies.
type cviRow struct{}

func (cviRow) String(int) (string, bool)    { return "s", true }
func (cviRow) Int(int) (int64, bool)        { return 1, true }
func (cviRow) Float(int) (float64, bool)    { return 1.5, true }
func (cviRow) Bool(int) (bool, bool)        { return true, true }
func (cviRow) Timestamp(int) (int64, bool)  { return 1000, true }
func (cviRow) Duration(int) (int64, bool)   { return 2000, true }
func (cviRow) Object(int) (string, bool)    { return `{"a":1}`, true }
func (cviRow) IsNull(attr int) bool         { return attr == 7 }
func (cviRow) AttrIndex(string) int         { return -1 }
func (cviRow) Len() int                     { return 8 }
func (cviRow) AttrType(attr int) cepio.Type { return cepio.Type(attr) } // 0 Invalid, 1 String … 7 Object

// TestCviSinkCopiesEveryType: Write reads each attribute by its type into a Go value; a null and an Invalid type
// stay nil.
func TestCviSinkCopiesEveryType(t *testing.T) {
	t.Parallel()
	var s inmemory.Sink
	if err := s.Write(context.Background(), cepio.OutEvent{Stream: "O", Timestamp: 1, Row: cviRow{}, Canonical: []byte("x")}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	recs := s.Drain()
	want := []any{nil, "s", int64(1), 1.5, true, int64(1000), int64(2000), nil}
	if len(recs) != 1 || len(recs[0].Values) != len(want) {
		t.Fatalf("records %+v", recs)
	}
	for i, w := range want {
		if recs[0].Values[i] != w {
			t.Errorf("Values[%d] = %v (%T); want %v (%T)", i, recs[0].Values[i], recs[0].Values[i], w, w)
		}
	}
	s.Reset()
	if n := len(s.Drain()); n != 0 {
		t.Errorf("%d records after Reset; want 0", n)
	}
}

// cviEmitter fails every Emit and EndBatch with errEmit.
type cviEmitter struct{}

var errEmit = errors.New("emit refused")

func (cviEmitter) Emit(context.Context, string, func(cepio.RowWriter) error) error { return errEmit }
func (cviEmitter) EndBatch(context.Context, string) error                          { return errEmit }

// TestCviSourceRefusalsAndContexts: a negative buffer is unbuffered; a nil fill and an empty batch id are refused;
// an expired context and a context ending while the pump does not take the row both return the context's error;
// an emit error ends Open with it and counts the row and the rest in Dropped.
func TestCviSourceRefusalsAndContexts(t *testing.T) {
	t.Parallel()
	ctx := imCeiling(t)
	s := inmemory.NewSource(-1)
	if err := s.Push(ctx, "", nil); err == nil {
		t.Error("Push with a nil fill: nil; want an error")
	}
	if err := s.EndBatch(ctx, ""); err == nil {
		t.Error("EndBatch with an empty batch id: nil; want an error")
	}
	expired, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Push(expired, "", imRow("a", 1)); !errors.Is(err, context.Canceled) {
		t.Errorf("Push with an expired context: %v; want context.Canceled", err)
	}
	short, cancelShort := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- s.Push(short, "", imRow("a", 1)) }() // unbuffered, no pump: blocks until short ends
	cancelShort()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Push while nothing pumps, its context ended: %v; want context.Canceled", err)
	}

	b := inmemory.NewSource(4)
	for i := 1; i <= 3; i++ {
		if err := b.Push(ctx, "", imRow("a", i)); err != nil {
			t.Fatalf("Push %d: %v", i, err)
		}
	}
	if err := b.Open(ctx, cviEmitter{}); !errors.Is(err, errEmit) {
		t.Errorf("Open with a refusing emitter: %v; want errEmit", err)
	}
	if got := b.Dropped(); got != 3 {
		t.Errorf("Dropped %d; want 3 (the refused row and the two behind it)", got)
	}
	b.Close()
	if err := b.Push(ctx, "", imRow("a", 4)); !errors.Is(err, inmemory.ErrClosed) {
		t.Errorf("Push after Close: %v; want ErrClosed", err)
	}
}
