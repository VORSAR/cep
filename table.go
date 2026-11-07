package cep

import (
	"context"

	cepio "github.com/vorsar/cep/io"
)

// Table writes a declared table. Each call is one write: one intake item, one table version, one record.
type Table struct{ _ struct{} }

// Upsert writes the rows the fills describe.
func (t *Table) Upsert(ctx context.Context, fills ...func(cepio.RowWriter) error) error {
	return errNotBuilt
}

// Delete removes the rows the keys name; each key fill sets primary-key columns only.
func (t *Table) Delete(ctx context.Context, keys ...func(cepio.RowWriter) error) error {
	return errNotBuilt
}

// Load is a multi-row Upsert.
func (t *Table) Load(ctx context.Context, fills []func(cepio.RowWriter) error) error {
	return errNotBuilt
}
