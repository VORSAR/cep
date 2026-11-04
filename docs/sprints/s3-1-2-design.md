# S3.1-2 — design note: tables and membership, the ingest flag, the decoder, the purity exceptions (v1)

S3.1's items 3–6 ([card](sprint-plan.md#s31--the-clock-service-tables-and-the-decoder)); exit lines 4 and 5, and harness check 1.1's `in` assertion. The owner's answers at the plan check are [sprint-plan §5, "S3.1-2's plan check"](sprint-plan.md#owner-decisions-2026-11-04--s31-2s-plan-check), numbered **P1–P8** below. Routine (owner, all of S3.1): QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it. Written against `s3-1-2/integration` @ `ffeb537`.

The stub commit carries exactly the API below with panicking bodies (the interface changes in §2 are real, so the tree builds and QA's tests compile red).

## 0. What the code says (read first)

1. **`expr.Build` types `in` / `not in` already** and returns a `later` node (`internal/expr/expr.go:171-182`); its `Scope.TableKey` gives the key's type but no handle, and `Row` has no way to reach a table (`expr.go:24-44`). → §2.
2. **`internal/event` may import only `cmp`, `errors`, `fmt`** (`internal/event/imports_test.go`, S1.1a-1's design). The decoder needs `strconv`, `unicode/utf8`, `unicode/utf16` and `time`. → §4.4.
3. **`archcheck` lets `internal/event` and `internal/table` import `sync` and `sync/atomic` without limit** (`tools/archcheck/main.go:67, 253`); ADR-0015 §2 names exactly `sync.Pool` for event and one mutex plus `atomic.Pointer` for table. → §5.
4. **`cep.BuildInfo` is S3.3's** (the facade does not exist). This sprint lands the two values it reports as constants in `internal/text`; S3.3 reads them. → §3.
5. **No validator rule rejects an `optional` or an `object` primary-key column** (`errors.md` has E-TBL-001 only, for membership against a composite key). → §1, review question R3.
6. **x/text v0.43.0** (latest, 2026-10-09; BSD-3; approved 2026-11-01, ADR-0009 §6) selects `tables17.0.0.go` under `go1.27`: `norm.Version` = `unicode.Version` = `17.0.0`. Its `go.mod` asks for Go 1.26. Added to `go.mod` and `tools/deps-allowlist.json`.

## 1. `internal/table` (item 3; ADR-0012 §5, SEMANTICS §4)

```go
package table

// Table is one declared table: a keyed set of rows (SPEC §4.3). Writers serialize on one mutex and publish a new,
// immutable Version through an atomic pointer; readers load the current Version once per event (SEMANTICS §4).
type Table struct {
	name  string
	attrs []event.Attr
	key   []int // the primary-key columns, in `primary key (…)` order
	mu    sync.Mutex
	cur   atomic.Pointer[Version]
}

// Version is one immutable state of a table. Nothing writes it after it is published.
type Version struct{ /* rows by key */ }

func New(name string, attrs []event.Attr, key []string) (*Table, error)
func (t *Table) Name() string
func (t *Table) Snapshot() *Version                    // the current version: never nil (empty after New); one atomic load
func (t *Table) Upsert(rows [][]event.Value) error     // insert or replace by key
func (t *Table) Delete(keys [][]event.Value) error     // each key: the key columns' values, in key order
func (t *Table) Load(rows [][]event.Value) error       // replace the whole content
func (v *Version) Len() int
func (v *Version) Get(key []event.Value) (row []event.Value, ok bool) // the stored row; the caller MUST NOT modify it
func (v *Version) Has(k event.Value) bool              // single-column key: a row whose key == k exists
```

- **`New` rejects:** an empty name; no attributes; an empty or duplicate attribute name (case-sensitive); a type outside `TString…TObject`; an empty key; a key column that is not declared, is named twice, or is `object`. *(R3: whether `optional` key columns and `object` key columns are deploy errors is a spec gap; `New` refuses `object` keys and accepts `optional` ones, and a null key value is refused at write time.)*
- **Every write is all or nothing.** Each row (or key) is checked before anything changes; on error nothing is published and the error names the table, the row's index and the attribute. A row MUST have one value per attribute; each value's `Kind` MUST equal its attribute's type; `Null` only where the attribute is `optional` and never in a key column; a `float` MUST be finite (NaN and ±Inf never enter the engine — ADR-0005's canonical encoding cannot write them). A key passed to `Delete` follows the same rules for the key columns.
- **Key equality is the expression engine's `==`** for the key's type: strings byte-exact; `int`, `timestamp`, `duration` by value; `float` by IEEE `==`, so `0.0` and `-0.0` are one key; `bool` by value.
- **`Upsert`:** a row whose key exists replaces it; within one call a later row wins over an earlier one with the same key (as P4). **`Delete`:** a key with no row is not an error. **`Load`:** the new content is exactly `rows` (later wins on a repeated key). A successful call with no rows or keys publishes nothing; any other success publishes exactly one new Version.
- **The table copies every row it stores**; a caller may reuse its slices after the call returns.
- **Concurrency:** any number of goroutines may call `Snapshot` and read Versions while writers run; writers serialize on `mu`. A published Version MUST NOT change. `Snapshot`, `Has`, `Len` and `Get` MUST NOT allocate.
- **Not here:** `Walk` (S7 needs it for table keyword sets; S3.3's sandbox seeds through `Upsert`), the public `cep.Table` and its `RowWriter` boundary (S3.3), table records in the replay log (S3.2a).

## 2. Membership: the `in` / `not in` node (item 3; SEMANTICS §1.1, §4; SPEC §5.6)

`expr` gains the table seam (`expr → table` is a listed edge in `rules.json`):

```go
type Row interface {
	Value(slot, attr int) event.Value
	Invisibles() bool
	// Table is the snapshot of table id that this evaluation reads. The caller takes every snapshot once per event,
	// before the first expression of that event runs, and returns the same *table.Version for every call until the
	// next event (SEMANTICS §4: all predicates evaluated for one event read the same table state).
	Table(id int) *table.Version
}

type Scope interface {
	Attr(qualifier, name string) (slot, index int, typ event.Type, ok bool)
	Capture(name string) (slot int, ok bool)
	// Table resolves a table name: its id, which is the table's index in the app's declaration order (the index
	// Row.Table takes), and its single-column key's type; ok false when the table is unknown or its key is composite.
	Table(name string) (id int, key event.Type, ok bool) // replaces TableKey
}
```

- `Build` of `ExprIn`: unchanged typing (E-NAME-001 at the table's position; E-TYPE-001 unless the operand's type is the key's and not `object`); it returns a membership node holding the operand, the id and `Negated`.
- `Eval`: an operand error is returned as is; **a null operand gives `false` for `in` and for `not in`** (atomic, SPEC §5.6); otherwise `in` is `Row.Table(id).Has(v)` and `not in` its negation. It MUST NOT allocate (the allocgate test gains an `in` case).
- The validator's scope implements `Table` (the id is the table's index in `App.Tables`); its text-operand check uses it in place of `TableKey`. The runtime's rows (S3.2a), and the pattern's (S4.1), take the snapshots; **forwarded to S3.2a:** where exactly "once per event" is taken across a shard's rules.
- `TestNullStrict` gains check 1.1's `in` assertion: `x in T` and `x not in T` are both false when `x` is null.

## 3. `internal/text`: the ingest flag and the Unicode tables (item 4; ADR-0007 §2, §7; SEMANTICS §5.1)

```go
package text

const (
	UnicodeTables = unicode.Version // the Go toolchain's tables: categories, so the invisible classes
	NormTables    = norm.Version    // x/text's tables: NFC (S7)
)

// IngestFlags reports whether any non-null string value holds an invisible code point (SEMANTICS §5.1 stage 1).
func IngestFlags(values []event.Value) bool
```

- **Invisible** = general category `Cf`, `Co` or `Cn` (unassigned: in no category of `unicode.L, M, N, P, S, Z, C`), or `Cc` other than TAB, LF and CR — the reference executor's `_strip_invisibles`. Values of other kinds and nulls are skipped; `object` text is not a string attribute and is not read. Invalid UTF-8 reads as U+FFFD (category So), not invisible (P8: S7 decides). It strips nothing, allocates nothing and stops at the first invisible.
- **The agreement check (P5, P6):** a test in `internal/text` fails unless `UnicodeTables == NormTables`, naming both. ADR-0007 §2 is annotated "exact".
- **BuildInfo's Unicode fields:** `cep.BuildInfo` (S3.3) reports `UnicodeTables`, `NormTables` and x/text's module version from the build info. Nothing else in the engine reads a Unicode table version.
- **The cross-check against the oracle:** the reference executor (Python 3.15.2's `unicodedata`, also Unicode 17.0.0) classifies every code point; a fixture of the invisible ranges is generated from `executor.py`'s own function by a stdlib script under the pinned interpreter, records its Unicode version, and is committed with its generator. A Go test compares `IngestFlags` on each code point (surrogates excluded: no Go string holds one) with the fixture, and asserts the fixture's version equals `UnicodeTables`.

## 4. `internal/event/decode.go`: the schema decoder (item 5; ADR-0010 §3; P1–P4, P8)

```go
// InvisiblesAttr is the engine-owned attribute name (SPEC §4.2, rev 3).
const InvisiblesAttr = "text_invisibles"

// Decoder decodes one JSON object per call into one stream's values. It is immutable after NewDecoder and safe
// for concurrent use.
type Decoder struct{ /* the schema, a name → index map, the time attribute */ }

func NewDecoder(s Schema) *Decoder

// Decoded is what a successful decode reports besides the values.
type Decoded struct {
	Ts       int64 // the event time from the schema's time attribute; 0 for a `time arrival` stream
	Unknown  int   // payload keys the schema does not declare, each occurrence: skipped (ARCHITECTURE §5 step 2)
	Reserved bool  // the payload carried InvisiblesAttr: ignored (SPEC §4.2: overwritten and counted)
}

// Decode appends the schema's NumAttrs values to dst[:0], in attribute order, and returns them.
func (d *Decoder) Decode(b []byte, dst []Value) ([]Value, Decoded, error)

type DecodeReason uint8

const (
	DecodeSyntax  DecodeReason = iota + 1 // not one JSON object, trailing data, nesting deeper than encoding/json's 10000
	DecodeType                            // a value of the wrong JSON type or out of range for its attribute
	DecodeMissing                         // a non-optional attribute, or the time attribute, absent
	DecodeNull                            // a non-optional attribute, or the time attribute, null
)

type DecodeError struct {
	Reason DecodeReason
	Attr   string // the attribute's name; "" for DecodeSyntax
	Offset int    // the byte offset in b where the fault starts
	Msg    string
}
```

### 4.1 The rule, stated as the oracle

The decoder MUST agree with this composition of `encoding/json`, which the differential test implements:

1. `json.Unmarshal(b, &m)` with `m map[string]json.RawMessage` fails → reject (DecodeSyntax). `m == nil` (the document is `null`) → reject (DecodeSyntax). Keys are compared after unescaping, case-sensitively; **a repeated key keeps its last value** (P4), and only that value is decoded.
2. A key the schema does not declare → skipped, counted in `Unknown`. `InvisiblesAttr` → skipped, `Reserved` set; a declared `text_invisibles` slot is `Value{Kind: TBool}` (false), for the caller to overwrite with the flag.
3. For each attribute in order: absent → `Value{Kind: t, Null: true}` if optional, else DecodeMissing; raw `null` → the same null if optional, else DecodeNull. **The time attribute is never null or absent** (SEMANTICS §1.1), optional or not.
4. Otherwise by type, each `json.Unmarshal(raw, &x)` into the Go type named, an error → DecodeType:

| type | JSON | Go target | the value |
|---|---|---|---|
| `string` | string | `string` | as decoded (malformed UTF-8 and lone surrogate escapes become U+FFFD — P8) |
| `int` | number | `int64` | exact over the full range; a fraction, exponent or overflow is DecodeType |
| `float` | number | `float64` | `strconv.ParseFloat(…, 64)`; `-0` stays `-0`; overflow is DecodeType |
| `bool` | `true` / `false` | `bool` | |
| `timestamp` | string | `string`, then `time.Parse(time.RFC3339, s)` | `t.UnixMilli()`: UTC, rounded down to the ms (P1) |
| `duration` | number | `int64` | milliseconds (P2) |
| `object` | object or array | `json.RawMessage` | its exact bytes, as a `string` in `Value.Obj` (P3) |

A JSON value of another type for an attribute (a number for a `string`, `"5"` for an `int`, a string for an `object`) is DecodeType: no coercion (SEMANTICS §1).

### 4.2 Shape

- One pass over `b` with no `map[string]any`, no reflection and no `encoding/json` in production code; string values are the only allocations beyond `dst`'s growth (ADR-0010 §3). The fuzz target comes before any speed work (card); there is no allocation gate on the decoder in this sprint.
- Earlier occurrences of a repeated key are checked for JSON syntax only (their type does not matter: only the last is decoded).
- `Offset` points into `b`; `Msg` names the attribute and what was wanted, never echoes more than 64 bytes of the payload.

### 4.3 Tests

- **The differential (exit 5):** a deterministic generator (fixed seed) builds payloads over schemas covering every type and optionality — valid ones, every single-step mutation class (type swaps, nulls, absences, repeats, unknown keys, `text_invisibles`, escapes, malformed UTF-8, int64 and float extremes, 2^53 + 1, RFC 3339 edge forms, nesting, trailing data, whitespace) — and compares accept/reject, the reason class where both reject, and every value (floats bitwise) with §4.1's oracle. Plus hand-written cases for each table row.
- **The fuzz target** `FuzzDecode` (`internal/event/fuzz_test.go`): any bytes, over a fixed set of schemas: the decoder never panics and agrees with the oracle. Seeds: the differential's hand-written cases. Its nightly row is `fuzz-decoder` (P7).

### 4.4 The import law for `internal/event`

`imports_test.go`'s list becomes `{cmp, errors, fmt, strconv, time, unicode/utf8, unicode/utf16}` plus `sync` in a file that uses only `sync.Pool` (none yet). `time` is for `time.Parse` and `Time.UnixMilli` only; `archcheck`'s clock check still bans the wall-clock calls there. `encoding/json` and `reflect` stay out of production code; the tests use `encoding/json` as the oracle.

## 5. `archcheck`: the named purity exceptions (item 6; ADR-0015 §2)

- `internal/table`: may import `sync` and `sync/atomic`, and use from them only `sync.Mutex` and `atomic.Pointer`.
- `internal/event`: may import `sync` only (not `sync/atomic`), and use from it only `sync.Pool`.
- Any other identifier from those packages there (`sync.RWMutex`, `sync.Once`, `sync.WaitGroup`, `atomic.Int64`, `atomic.Value`, …), and `sync/atomic` in `internal/event`, is a `purity` finding naming the allowed identifiers and ADR-0015 §2. The exceptions sit in `archcheck`'s code next to today's `syncAllowed`; moving them to `tools/allowlists/` is S1.1c's (ADR-0015 §3.10).
- **Fixtures** (`archcheck_test.go`): each allowed use passes; each banned identifier is red with its message; `sync/atomic` in event is red; a package outside the two is red for any `sync` import, as today.

## 6. The gate

- `fuzz-decoder`: a nightly row, a copy of `fuzz-parser` with `expected_scope` `internal/event/fuzz_test.go`, `lands_in` `S3.1-2`.
- The Unicode agreement test and the table, membership, flag and decoder tests run in `unit` (P6). Coverage stays above each package's floor.

## 7. Review questions (for the architecture reviewer)

- **R1.** `Row.Table(id)` puts the per-event snapshot on the caller. Is that the right owner, and is `id` = declaration index a stable enough seam for the planner (S3.2a)?
- **R2.** `Scope.Table` replaces `TableKey`: one method returning id and key type. Any reason to keep two?
- **R3.** The spec gap on `optional` and `object` key columns (§0.5): reject both at deploy (a new static rule), or as §1 does?
- **R4.** `time` in `internal/event` for `time.Parse` (§4.4), against ARCHITECTURE §2's "L0 knows nothing about time". The alternative is a hand-written RFC 3339 reader, tested against `time.Parse`.
- **R5.** `Decoded.Reserved` and `Unknown` as returned facts, the event package counting nothing (as the clock does).

## 8. Size

Items 3 [2], 4 [1], 5 [3], 6 [0.5]: 6.5 points, under the cap; no lanes.
