package cep

import (
	"context"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
	cepio "github.com/vorsar/cep/io"
)

// Table writes a declared table (design s3-3-1 §3.1, §4.4). Each call is one write: one intake item, one table
// version, one record; rows are checked as the engine's TableWrite checks them.
type Table struct {
	rt     *runtime.Runtime
	app    runtime.AppID
	id     int
	attrs  []event.Attr
	names  []string
	keyPos []int // the primary-key columns' attribute indexes, in key order
}

func tableFor(a *AppHandle, tid int) *Table {
	t := a.plan.Tables[tid]
	names := make([]string, len(t.Attrs))
	for i, at := range t.Attrs {
		names[i] = at.Name
	}
	keyPos := make([]int, len(t.Key))
	for i, k := range t.Key {
		keyPos[i] = attrIndex(names, k)
	}
	return &Table{rt: a.rt, app: a.id, id: tid, attrs: t.Attrs, names: names, keyPos: keyPos}
}

// rows builds whole rows from fills.
func (t *Table) rows(fills []func(cepio.RowWriter) error) ([][]event.Value, error) {
	out := make([][]event.Value, 0, len(fills))
	for _, fill := range fills {
		w := newTableRow(t.attrs, t.names, nil)
		if err := w.run(fill); err != nil {
			return nil, err
		}
		out = append(out, w.values)
	}
	return out, nil
}

// keys builds delete rows: the key columns' values in key order.
func (t *Table) keys(fills []func(cepio.RowWriter) error) ([][]event.Value, error) {
	out := make([][]event.Value, 0, len(fills))
	for _, fill := range fills {
		w := newTableRow(t.attrs, t.names, t.keyPos)
		if err := w.run(fill); err != nil {
			return nil, err
		}
		key := make([]event.Value, len(t.keyPos))
		for i, p := range t.keyPos {
			key[i] = w.values[p]
		}
		out = append(out, key)
	}
	return out, nil
}

// Upsert writes the rows the fills describe: one write, many rows.
func (t *Table) Upsert(ctx context.Context, fills ...func(cepio.RowWriter) error) error {
	rows, err := t.rows(fills)
	if err != nil {
		return err
	}
	return mapErr(t.rt.TableWrite(ctx, t.app, t.id, runtime.TablePut, rows))
}

// Delete removes the rows the keys name; each key fill sets primary-key columns only.
func (t *Table) Delete(ctx context.Context, keys ...func(cepio.RowWriter) error) error {
	rows, err := t.keys(keys)
	if err != nil {
		return err
	}
	return mapErr(t.rt.TableWrite(ctx, t.app, t.id, runtime.TableDelete, rows))
}

// Load is a multi-row Upsert (a replace-all is S3.3-2's given table).
func (t *Table) Load(ctx context.Context, fills []func(cepio.RowWriter) error) error {
	return t.Upsert(ctx, fills...)
}
