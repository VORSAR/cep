// Package table holds the engine's in-memory tables: keyed sets of rows read through a per-event snapshot
// (SPEC §4.3, SEMANTICS §4, ADR-0012 §5).
//
// Writers serialize on one mutex and publish a new, immutable Version through an atomic pointer; readers load the
// current Version once per event and never coordinate with writers. Which Version an event reads is the runtime's
// business: the one current at the event's ingest position (owner, 2026-11-04, S3.1-2 D1). This package's sync and
// sync/atomic use is a named purity exception (ADR-0015 §2): one sync.Mutex and one atomic.Pointer, nothing else.
package table

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/vorsar/cep/internal/event"
)

// Table is one declared table (design note s3-1-2 §1).
type Table struct {
	name  string
	attrs []event.Attr
	key   []int // the primary-key columns, in `primary key (…)` order
	isKey []bool
	mu    sync.Mutex
	cur   atomic.Pointer[Version]
}

// Version is one immutable state of a table. Nothing writes it after it is published.
type Version struct {
	rows    map[key][]event.Value
	keyKind event.Type // the first key column's type
	nkey    int
}

// key is a row's primary key. The first key column sits in the field of its own type, so a single-column lookup
// builds it without allocating; the remaining columns of a composite key are encoded into rest. The float field
// compares with IEEE ==, the expression engine's equality: 0.0 and -0.0 are one key.
type key struct {
	s    string
	n    int64 // int, timestamp, duration
	f    float64
	b    bool
	rest string
}

var (
	errEmptyName = errors.New("empty table name")
	errNoAttrs   = errors.New("no attributes declared")
	errNoKey     = errors.New("no primary key")
)

// New validates a table declaration and returns the table with an empty current Version. It rejects an empty
// name; no attributes; an empty or duplicate attribute name (case-sensitive); a type outside TString…TObject; an
// empty key; and a key column that is not declared, is named twice, or is an object. An optional key column is
// accepted; a null key value is refused when written (owner D3: the deploy error for both is S3.3's).
func New(name string, attrs []event.Attr, keyCols []string) (*Table, error) {
	if name == "" {
		return nil, fmt.Errorf("table: %w", errEmptyName)
	}
	if len(attrs) == 0 {
		return nil, fmt.Errorf("table %s: %w", name, errNoAttrs)
	}
	own := slices.Clone(attrs)
	at := make(map[string]int, len(own))
	for i, a := range own {
		switch {
		case a.Name == "":
			return nil, fmt.Errorf("table %s: attribute %d: empty name", name, i)
		case a.Type < event.TString || a.Type > event.TObject:
			return nil, fmt.Errorf("table %s: attribute %d %q: invalid type %v", name, i, a.Name, a.Type)
		}
		if first, dup := at[a.Name]; dup {
			return nil, fmt.Errorf("table %s: attribute %d %q: duplicate name, first declared as attribute %d", name, i, a.Name, first)
		}
		at[a.Name] = i
	}
	if len(keyCols) == 0 {
		return nil, fmt.Errorf("table %s: %w", name, errNoKey)
	}
	t := &Table{name: name, attrs: own, isKey: make([]bool, len(own))}
	for _, col := range keyCols {
		i, ok := at[col]
		switch {
		case !ok:
			return nil, fmt.Errorf("table %s: key column %q is not declared", name, col)
		case t.isKey[i]:
			return nil, fmt.Errorf("table %s: key column %q is named twice", name, col)
		case own[i].Type == event.TObject:
			return nil, fmt.Errorf("table %s: key column %q is an object: an object cannot be a key", name, col)
		}
		t.isKey[i] = true
		t.key = append(t.key, i)
	}
	t.cur.Store(&Version{rows: map[key][]event.Value{}, keyKind: own[t.key[0]].Type, nkey: len(t.key)})
	return t, nil
}

// Name is the table's declared name.
func (t *Table) Name() string { return t.name }

// Attrs is the table's columns, in declaration order; the caller MUST NOT change them.
func (t *Table) Attrs() []event.Attr { return t.attrs }

// Snapshot is the current Version: never nil, one atomic load, no allocation.
func (t *Table) Snapshot() *Version { return t.cur.Load() }

// Upsert inserts or replaces rows by key, all or nothing: every row is checked before anything changes. A row
// whose key exists replaces it; within one call a later row wins over an earlier one with the same key. The table
// stores copies. A call with no rows publishes nothing; any other success publishes exactly one new Version.
func (t *Table) Upsert(rows [][]event.Value) error {
	if len(rows) == 0 {
		return nil
	}
	stored, keys, err := t.prepare(rows)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	next := t.cur.Load().clone(len(rows))
	for i, r := range stored {
		next.rows[keys[i]] = r
	}
	t.cur.Store(next)
	return nil
}

// Delete removes the rows with these keys (each: the key columns' values, in key order), all or nothing. A key
// with no row is not an error. A call with no keys publishes nothing.
func (t *Table) Delete(keys [][]event.Value) error {
	if len(keys) == 0 {
		return nil
	}
	ks := make([]key, len(keys))
	for i, k := range keys {
		if err := t.checkKey(i, k); err != nil {
			return err
		}
		ks[i] = t.keyFrom(k)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	next := t.cur.Load().clone(0)
	for _, k := range ks {
		delete(next.rows, k)
	}
	t.cur.Store(next)
	return nil
}

// Load replaces the whole content with rows, all or nothing; a later row wins over an earlier one with the same
// key. Every success publishes one new Version: loading no rows empties the table.
func (t *Table) Load(rows [][]event.Value) error {
	stored, keys, err := t.prepare(rows)
	if err != nil {
		return err
	}
	next := t.cur.Load().empty(len(rows))
	for i, r := range stored {
		next.rows[keys[i]] = r
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cur.Store(next)
	return nil
}

// prepare checks every row and returns the copies to store with their keys.
func (t *Table) prepare(rows [][]event.Value) ([][]event.Value, []key, error) {
	stored := make([][]event.Value, len(rows))
	keys := make([]key, len(rows))
	for i, r := range rows {
		if err := t.checkRow(i, r); err != nil {
			return nil, nil, err
		}
		stored[i] = slices.Clone(r)
		for j := range stored[i] {
			if v := &stored[i][j]; v.Kind == event.TObject && !v.Null {
				if b, isBytes := v.Obj.([]byte); isBytes {
					v.Obj = string(b) // the caller may reuse its buffer: the stored row holds its own text
				}
			}
		}
		keys[i] = t.keyOf(stored[i])
	}
	return stored, keys, nil
}

func (t *Table) checkRow(i int, r []event.Value) error {
	if len(r) != len(t.attrs) {
		return fmt.Errorf("table %s: row %d: %d values for %d attributes", t.name, i, len(r), len(t.attrs))
	}
	for j, v := range r {
		if err := t.checkValue(j, v); err != nil {
			return fmt.Errorf("table %s: row %d: %w", t.name, i, err)
		}
	}
	return nil
}

func (t *Table) checkKey(i int, k []event.Value) error {
	if len(k) != len(t.key) {
		return fmt.Errorf("table %s: key %d: %d values for %d key columns", t.name, i, len(k), len(t.key))
	}
	for c, v := range k {
		if err := t.checkValue(t.key[c], v); err != nil {
			return fmt.Errorf("table %s: key %d: %w", t.name, i, err)
		}
	}
	return nil
}

// checkValue holds one value to attribute j's declaration: its type, null only where optional and never in a key
// column, a float finite (NaN and ±Inf never enter the engine — ADR-0005 §3), and an object its JSON text (P3; the
// table stores it as a string, so no caller can change a published row — S3.1-2 break-it B5).
func (t *Table) checkValue(j int, v event.Value) error {
	a := t.attrs[j]
	switch {
	case v.Kind != a.Type:
		return fmt.Errorf("attribute %d %q: want %v, got %v", j, a.Name, a.Type, v.Kind)
	case v.Null && t.isKey[j]:
		return fmt.Errorf("attribute %d %q: a key value cannot be null", j, a.Name)
	case v.Null && !a.Optional:
		return fmt.Errorf("attribute %d %q: null for an attribute that is not optional", j, a.Name)
	case !v.Null && v.Kind == event.TFloat && (math.IsNaN(v.F) || math.IsInf(v.F, 0)):
		return fmt.Errorf("attribute %d %q: the float %v is not finite", j, a.Name, v.F)
	case !v.Null && v.Kind == event.TObject:
		switch v.Obj.(type) {
		case string, []byte:
		default:
			return fmt.Errorf("attribute %d %q: an object value is its JSON text, a string or []byte; got %T", j, a.Name, v.Obj)
		}
	}
	return nil
}

// keyOf is a checked row's key.
func (t *Table) keyOf(row []event.Value) key {
	k := scalar(row[t.key[0]])
	for _, c := range t.key[1:] {
		k.rest = appendValue(k.rest, row[c])
	}
	return k
}

// keyFrom is a checked key's key.
func (t *Table) keyFrom(vals []event.Value) key {
	k := scalar(vals[0])
	for _, v := range vals[1:] {
		k.rest = appendValue(k.rest, v)
	}
	return k
}

// scalar places a value in its type's own field; every other field stays zero.
func scalar(v event.Value) key {
	switch v.Kind {
	case event.TString:
		return key{s: v.Str}
	case event.TInt:
		return key{n: v.I}
	case event.TTimestamp:
		return key{n: v.T}
	case event.TDuration:
		return key{n: v.D}
	case event.TFloat:
		return key{f: v.F}
	case event.TBool:
		return key{b: v.B}
	}
	return key{}
}

// appendValue encodes a later key column: its type's own field, length-prefixed for strings, with -0 written as 0
// so the encoding keeps IEEE equality.
func appendValue(dst string, v event.Value) string {
	switch v.Kind {
	case event.TString:
		return dst + strconv.Itoa(len(v.Str)) + ":" + v.Str
	case event.TInt:
		return dst + "i" + strconv.FormatInt(v.I, 10) + ";"
	case event.TTimestamp:
		return dst + "t" + strconv.FormatInt(v.T, 10) + ";"
	case event.TDuration:
		return dst + "d" + strconv.FormatInt(v.D, 10) + ";"
	case event.TFloat:
		f := v.F
		if f == 0 {
			f = 0 // -0 and +0 are one key
		}
		return dst + "f" + strconv.FormatUint(math.Float64bits(f), 16) + ";"
	case event.TBool:
		return dst + strconv.FormatBool(v.B) + ";"
	}
	return dst
}

func (v *Version) clone(extra int) *Version {
	next := v.empty(len(v.rows) + extra)
	for k, r := range v.rows {
		next.rows[k] = r
	}
	return next
}

func (v *Version) empty(size int) *Version {
	return &Version{rows: make(map[key][]event.Value, size), keyKind: v.keyKind, nkey: v.nkey}
}

// Len is the number of rows.
func (v *Version) Len() int { return len(v.rows) }

// Get returns the stored row with this key (the key columns' values, in key order); ok is false for a key that is
// malformed (wrong count, a type, a null) or has no row. The caller must not modify the row. It may allocate for a
// composite key.
func (v *Version) Get(k []event.Value) ([]event.Value, bool) {
	if len(k) != v.nkey || len(k) == 0 || k[0].Kind != v.keyKind {
		return nil, false
	}
	for _, x := range k {
		if x.Null {
			return nil, false
		}
	}
	kk := scalar(k[0])
	for _, x := range k[1:] {
		kk.rest = appendValue(kk.rest, x)
	}
	row, ok := v.rows[kk]
	return row, ok
}

// Has reports whether a row whose single-column key equals k exists: false for a null k, a k of another type, and
// a table with a composite key. Equality is the expression engine's == for the key's type; it never allocates.
func (v *Version) Has(k event.Value) bool {
	if v.nkey != 1 || k.Null || k.Kind != v.keyKind {
		return false
	}
	_, ok := v.rows[scalar(k)]
	return ok
}

// Set is an app's tables in declaration order: a table's index is its id, the id expr.Scope.Table gives (design
// note s3-2a-1 §2).
type Set struct {
	tables []*Table
	last   *Snapshot
}

// NewSet holds tables in declaration order.
func NewSet(tables []*Table) *Set { return &Set{tables: append([]*Table(nil), tables...)} }

// Len is the number of tables.
func (s *Set) Len() int { return len(s.tables) }

// Table is table id.
func (s *Set) Table(id int) *Table { return s.tables[id] }

// Snapshot is every table's current Version (SEMANTICS §4, owner D1 of S3.1-2). When no table has published a
// Version since the last call it returns the same *Snapshot, so a steady-state ingest step allocates nothing. One
// goroutine only — the router or the playback driver: it takes no lock.
func (s *Set) Snapshot() *Snapshot {
	if last := s.last; last != nil && s.unchanged(last) {
		return last
	}
	versions := make([]*Version, len(s.tables))
	for i, t := range s.tables {
		versions[i] = t.cur.Load()
	}
	s.last = &Snapshot{versions: versions}
	return s.last
}

// unchanged says no table has published a Version since snap was taken.
func (s *Set) unchanged(snap *Snapshot) bool {
	for i, t := range s.tables {
		if t.cur.Load() != snap.versions[i] {
			return false
		}
	}
	return true
}

// Snapshot is one immutable read view of an app's tables.
type Snapshot struct {
	versions []*Version
}

// Table is table id's Version in this view: never nil for an id in range; an id out of range panics like a slice
// index.
func (s *Snapshot) Table(id int) *Version { return s.versions[id] }

// Len is the number of tables in the view.
func (s *Snapshot) Len() int { return len(s.versions) }
