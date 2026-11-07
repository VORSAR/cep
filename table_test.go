package cep_test

// Table rows through the facade (design s3-3-1 §4.4; QA's T25 as §9 amends it).

import (
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
)

func fcTRow(k string, v int64) func(cepio.RowWriter) error {
	return func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), k)
		w.SetInt(w.AttrIndex("v"), v)
		return nil
	}
}

// TestTableRowsAreCheckedAsTableWriteChecksThem: the key unset, a wrong type and a non-key column in a delete key
// each name the attribute; Load is a multi-row upsert (two Loads leave three keys); one call is one write.
func TestTableRowsAreCheckedAsTableWriteChecksThem(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "tbl", fcTableSrc, bind)
	fcStart(t, ctx, h)
	tb, err := h.Table("T")
	if err != nil {
		t.Fatalf("Table(T): %v", err)
	}
	if err := tb.Upsert(ctx, func(w cepio.RowWriter) error { w.SetInt(w.AttrIndex("v"), 1); return nil }); err == nil || !strings.Contains(err.Error(), "k") {
		t.Errorf("Upsert with k unset: %v; want an error naming k", err)
	}
	if err := tb.Upsert(ctx, func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), "a")
		w.SetString(w.AttrIndex("v"), "x")
		return nil
	}); err == nil || !strings.Contains(err.Error(), "v") {
		t.Errorf("Upsert with SetString on v: %v; want an error naming v", err)
	}
	if err := tb.Delete(ctx, func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), "a")
		w.SetInt(w.AttrIndex("v"), 1)
		return nil
	}); !errors.Is(err, cep.ErrNotAKeyColumn) || !strings.Contains(err.Error(), "v") {
		t.Errorf("Delete whose key sets v: %v; want ErrNotAKeyColumn naming v", err)
	}
	if err := tb.Load(ctx, []func(cepio.RowWriter) error{fcTRow("k1", 1), fcTRow("k2", 2)}); err != nil {
		t.Fatalf("Load of 2: %v", err)
	}
	if err := tb.Load(ctx, []func(cepio.RowWriter) error{fcTRow("k3", 3)}); err != nil {
		t.Fatalf("Load of 1: %v", err)
	}
	in := fcInput(t, h, "S")
	for i, k := range []string{"k1", "k2", "k3", "k4"} {
		fcSend(t, ctx, in, fcKey(k, fcAt(float64(i+1))))
	}
	if err := tb.Delete(ctx, func(w cepio.RowWriter) error { w.SetString(w.AttrIndex("k"), "k1"); return nil }); err != nil {
		t.Fatalf("Delete(k1): %v", err)
	}
	fcSend(t, ctx, in, fcKey("k1", fcAt(5)))
	fcStop(t, ctx, h)
	recs := sink.Drain()
	var got []string
	for _, rec := range recs {
		got = append(got, rec.Values[0].(string))
	}
	if want := "k4,k1"; strings.Join(got, ",") != want {
		t.Errorf("outputs %v; want %s (Load is a multi-row upsert: k1..k3 stay; k4 was never in T; k1 after its delete)", got, want)
	}
}
