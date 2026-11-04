// Package table holds the engine's in-memory tables: keyed sets of rows read through a per-event snapshot
// (SPEC §4.3, SEMANTICS §4, ADR-0012 §5).
//
// Writers serialize on one mutex and publish a new, immutable Version through an atomic pointer; readers load the
// current Version once per event and never coordinate with writers. This package's sync and sync/atomic use is a
// named purity exception (ADR-0015 §2): one sync.Mutex and one atomic.Pointer, nothing else.
package table

import (
	"sync"
	"sync/atomic"

	"github.com/vorsar/cep/internal/event"
)

// Table is one declared table (design note §1).
type Table struct {
	name  string
	attrs []event.Attr
	key   []int // the primary-key columns, in `primary key (…)` order
	mu    sync.Mutex
	cur   atomic.Pointer[Version]
}

// Version is one immutable state of a table. Nothing writes it after it is published.
type Version struct{}

// New validates a table declaration (design note §1) and returns the table with an empty current Version.
func New(name string, attrs []event.Attr, key []string) (*Table, error) {
	panic("table.New: not built yet (S3.1-2 stub)")
}

// Name is the table's declared name.
func (t *Table) Name() string { panic("table.Name: not built yet (S3.1-2 stub)") }

// Snapshot is the current Version: never nil, one atomic load, no allocation.
func (t *Table) Snapshot() *Version { panic("table.Snapshot: not built yet (S3.1-2 stub)") }

// Upsert inserts or replaces rows by key, all or nothing (design note §1).
func (t *Table) Upsert(rows [][]event.Value) error {
	panic("table.Upsert: not built yet (S3.1-2 stub)")
}

// Delete removes the rows with these keys (each: the key columns' values, in key order), all or nothing.
func (t *Table) Delete(keys [][]event.Value) error {
	panic("table.Delete: not built yet (S3.1-2 stub)")
}

// Load replaces the whole content with rows, all or nothing.
func (t *Table) Load(rows [][]event.Value) error {
	panic("table.Load: not built yet (S3.1-2 stub)")
}

// Len is the number of rows.
func (v *Version) Len() int { panic("table.Version.Len: not built yet (S3.1-2 stub)") }

// Get returns the stored row with this key; the caller must not modify it.
func (v *Version) Get(key []event.Value) ([]event.Value, bool) {
	panic("table.Version.Get: not built yet (S3.1-2 stub)")
}

// Has reports whether a row whose single-column key equals k exists: false for a null k, a k of another type, and
// a table with a composite key. Equality is the expression engine's == for the key's type.
func (v *Version) Has(k event.Value) bool { panic("table.Version.Has: not built yet (S3.1-2 stub)") }
