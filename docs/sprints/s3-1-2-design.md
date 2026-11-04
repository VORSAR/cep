# S3.1-2 — design note: tables and membership, the ingest flag, the decoder, the purity exceptions (v2)

S3.1's items 3–6 ([card](sprint-plan.md#s31--the-clock-service-tables-and-the-decoder)); exit lines 4 and 5, and harness check 1.1's `in` assertion. The owner's answers: at the plan check, **P1–P8** ([sprint-plan §5, "S3.1-2's plan check"](sprint-plan.md#owner-decisions-2026-11-04--s31-2s-plan-check)); after the architecture review, **D1–D4** ([sprint-plan §5, "S3.1-2's design"](sprint-plan.md#owner-decisions-2026-11-04--s31-2s-design-after-the-architecture-review)). Routine (owner, all of S3.1): QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it.

v2 answers the architecture review ([s3-1-2-design-architecture.md](../reviews/s3-1-2-design-architecture.md): 3 blockers, 12 should-fix, 9 later) and QA's notes on the decoder tests (A1–A9); §9 places every finding. The stub commit carries the API with panicking bodies; MUST and MUST NOT below are the build's contract.

## 0. What the code says (read first)

1. **`expr.Build` types `in` / `not in`** (`internal/expr/expr.go`); the stub turned the old `later` node into a `member` node and gave `expr` the table seam (§2).
2. **`internal/event` imported only `cmp`, `errors`, `fmt`**; the decoder needs `strconv`, `unicode/utf8`, `unicode/utf16` (§4.5). It does not import `time` (D4).
3. **`archcheck` let `internal/event` and `internal/table` import `sync` and `sync/atomic` without limit**; ADR-0015 §2 names `sync.Pool` for event, one mutex and `atomic.Pointer` for table (§5).
4. **`cep.BuildInfo` is S3.3's** (the facade does not exist). This sprint lands the two values it reports as constants in `internal/text` (§3).
5. **No validator rule rejects an `optional` or `object` primary-key column**; the deploy error is S3.3's (D3).
6. **x/text v0.43.0** (2026-10-09; BSD-3; approved 2026-11-01) selects `tables17.0.0.go` under `go1.27`: `norm.Version` = `unicode.Version` = `17.0.0`. In `go.mod` and `tools/deps-allowlist.json`.
7. **`expr`'s float comparison treats NaN as equal to everything** (`cmpF` returns 0 when neither `<` nor `>` holds), and `cast('NaN' as float)` or a float overflow produces NaN or ±Inf (review F3). Fixed here (D2, §6).
8. **The reference executor has no tables**, so D1 needs no oracle change in this sprint; S3.2a's exit 2 tests it end to end.

## 1. `internal/table` (item 3; ADR-0012 §5, SEMANTICS §4)

```go
type Table struct {
	name  string
	attrs []event.Attr
	key   []int // the primary-key columns, in `primary key (…)` order
	mu    sync.Mutex
	cur   atomic.Pointer[Version]
}
type Version struct{ /* rows by key; immutable once published */ }

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

- **`New` rejects:** an empty name; no attributes; an empty or duplicate attribute name (case-sensitive); a type outside `TString…TObject`; an empty key; a key column that is not declared, is named twice, or is `object`. It accepts an `optional` key column; a null key value is refused at write (D3: the deploy error for both is S3.3's).
- **Every write is all or nothing.** Every row (or key) is checked before anything changes; on error nothing is published, and the error names the table, the row's index and the attribute. A row MUST have one value per attribute; each value's `Kind` MUST equal its attribute's type; `Null` only where the attribute is `optional` and never in a key column; a `float` MUST be finite. A key passed to `Delete` follows the same rules for the key columns.
- **Key equality is per type, on the type's own field** (never `event.Value` struct equality): strings byte-exact; `int`, `timestamp`, `duration` by value; `float` by IEEE `==` on finite values, so `0.0` and `-0.0` are one key; `bool` by value. NaN and ±Inf are never stored, and `Has` of one is false.
- **`Upsert`:** a row whose key exists replaces it; within one call a later row wins over an earlier one with the same key (this design's choice, mirroring P4). **`Delete`:** a key with no row is not an error. **`Load`:** the new content is exactly `rows`, later wins; `Load` of no rows empties the table (QA A2, ruled at the build). An `Upsert` or `Delete` with no rows or keys publishes nothing; any other success publishes exactly one new Version, a `Delete` of absent keys included (QA A3).
- **The table copies every row it stores**; a caller may reuse its slices after the call.
- **Concurrency:** any number of goroutines may call `Snapshot` and read Versions while writers run; writers serialize on `mu`. A published Version MUST NOT change. `Snapshot`, `Has` and `Len` MUST NOT allocate; `Get` may.
- **Cost, stated:** each write copies the whole table (O(n)); snapshots held by buffered events (D1) keep old versions alive. Writes are rare (ADR-0012 §5); a large, often-written table is a later concern (review F22).
- **Not here:** `Walk` (S7), the public `cep.Table` and its `RowWriter` boundary (S3.3), table records in the replay log (S3.2a).

## 2. Membership: the `in` / `not in` node (item 3; SEMANTICS §1.1, §4 as annotated; SPEC §5.6)

```go
type Row interface {
	Value(slot, attr int) event.Value
	Invisibles() bool
	// Table is table id as it was at the ingest position of the event being evaluated (SEMANTICS §4, owner D1):
	// the runtime takes one snapshot of every table of the app per event, when the event is ingested, and every
	// rule and shard that evaluates the event reads that snapshot, however late the evaluation runs.
	Table(id int) *table.Version
}

type Scope interface {
	Attr(qualifier, name string) (slot, index int, typ event.Type, ok bool)
	Capture(name string) (slot int, ok bool)
	// Table resolves a table name: its id, the table's index in the app's declaration order (the index Row.Table
	// takes), and its single-column key's type. ok is false when the table is unknown or its key is composite; the
	// validator reports E-TBL-001 for a composite key before Build runs, so false reads as unknown only there.
	Table(name string) (id int, key event.Type, ok bool)
}
```

- `Build` of `ExprIn`: typing unchanged (E-NAME-001 at the table's position; E-TYPE-001 unless the operand's type is the key's and not `object`); it returns the `member` node: the operand, the id, `Negated`.
- `member.Eval`, in this order (QA A4): an operand error is returned as is; **a null operand gives `false` for `in` and for `not in`** (atomic, SPEC §5.6); `Row.Table(id) == nil` is a `RuntimeError` (a wiring fault, never read as an empty table); otherwise `in` is `Has(v)` and `not in` its negation. It MUST NOT allocate.
- **Only the runtime layer takes snapshots** (L3). An operator's row (pattern S4.1, window S8) embeds the runtime-provided `expr.Row`, which promotes `Table` without naming `table.Version`, so `pattern` and `window` need no `table` import (review F8). How the snapshot travels with the event through the release buffer, and which snapshot a timer-triggered evaluation reads, are S3.2a's (forwarded on its card).
- `rules.json` fences `expr → table` to `Version`: `expr` reads tables and never writes them (review F21). Its tests may also use `table.New` and `table.Table` to build fixtures, through a new `test_fences` key, the twin of `test_imports` (owner, 2026-11-04, ≠ R: the recommendation was to check fences on production files only).
- The validator's scope implements `Table` with the declaration index, and returns `ok == false` if the table is not in `App.Tables` (never id −1).

**Tests (QA).** Check 1.1's `in` assertion joins `TestNullStrict`: `x in T` and `x not in T` are both false when `x` is null. **Exit 4's proof, `TestMembershipSnapshot`** (`internal/expr`): a row pinned to Version v1 evaluates `x in T` and `x not in T`; an `Upsert` publishes v2 (also from inside the row's own `Value` call, mid-evaluation); the same row evaluating again gives v1's answers for both; a row pinned to the new `Snapshot()` gives v2's. In `internal/table`, a `-race` test with concurrent writers and readers: every Version a reader loads is internally consistent (`Len` and `Has` agree with what one write published), and a failed write publishes nothing. The allocgate test gains an `in` case.

## 3. `internal/text`: the ingest flag and the Unicode tables (item 4; ADR-0007 §2, §7; SEMANTICS §5.1)

```go
const (
	UnicodeTables = unicode.Version // the Go toolchain's tables: categories, so the invisible classes
	NormTables    = norm.Version    // x/text's tables: NFC (S7)
)
func IngestFlags(values []event.Value) bool
```

- **Invisible** = category `Cf` or `Co`; `Cc` other than TAB, LF and CR; or `Cn`, unassigned, which is **in none of `unicode.L, M, N, P, S, Z, Cc, Cf, Co, Cs`** (review F2: Go's `unicode.C` already contains every unassigned code point, so `C` must not be used whole). This is the reference executor's `_strip_invisibles`.
- `IngestFlags` reads non-null `TString` values only (an `object` value is not a string attribute); invalid UTF-8 reads as U+FFFD (category So), not invisible (P8). It strips nothing, allocates nothing and stops at the first invisible.
- **The agreement test (P5, P6):** fails unless `UnicodeTables == NormTables`, naming both.
- **The cross-check against the oracle (QA):** a fixture of the invisible ranges, generated from `executor.py`'s own function by a stdlib script under the pinned CPython 3.15.2 (Unicode 17.0.0) and committed with its generator and its Unicode version; a Go test compares `IngestFlags` on every code point (surrogates excluded) with it and asserts the fixture's version equals `UnicodeTables`.
- `cep.BuildInfo` (S3.3) reports `UnicodeTables`, `NormTables` and x/text's module version.

## 4. `internal/event/decode.go`: the schema decoder (item 5; ADR-0010 §3; P1–P4, P8, D4)

API as in the stub: `InvisiblesAttr`, `Decoder`, `NewDecoder(Schema)`, `Decoded{Ts, Unknown, Reserved}`, `Decode(b, dst) ([]Value, Decoded, error)`, `DecodeReason` {Syntax, Type, Missing, Null}, `*DecodeError{Reason, Attr, Offset, Msg}`.

### 4.1 The rule, stated as the oracle

The decoder MUST agree with this composition of `encoding/json` (Go 1.27.3's), which the differential test implements:

1. `json.Unmarshal(b, &m)` with `m map[string]json.RawMessage` fails, or `m == nil` (the document is `null`) → DecodeSyntax. Keys compare after unescaping, case-sensitively; **a repeated key keeps its last value** (P4), and only that value is decoded; earlier occurrences need only be valid JSON.
2. A key the schema does not declare → skipped, counted in `Unknown` (each occurrence). `InvisiblesAttr` → skipped, `Reserved` set, never counted in `Unknown`; a declared `text_invisibles` slot is `Value{Kind: TBool}` (false, non-null), for the caller (S3.2b, which gives the flag one home — review F6).
3. For each attribute in schema order: absent → `Value{Kind: t, Null: true}` if optional, else DecodeMissing; raw `null` → the same null if optional, else DecodeNull. **The time attribute is never null or absent** (SEMANTICS §1.1), optional or not.
4. Otherwise by type (an error → DecodeType; no coercion, SEMANTICS §1):

| type | JSON | the value |
|---|---|---|
| `string` | string | as `encoding/json` decodes it into `string`: each malformed UTF-8 byte and each lone surrogate escape becomes U+FFFD (P8) |
| `int` | number | as into `int64`: exact over the full range; a fraction, an exponent or an overflow is DecodeType |
| `float` | number | as into `float64`; `-0` stays `-0`; overflow is DecodeType; underflow gives ±0, not an error |
| `bool` | `true` / `false` | |
| `timestamp` | string | the strict RFC 3339 reading of §4.2, in ms (D4, P1) |
| `duration` | number | as `int`, in milliseconds (P2) |
| `object` | object or array | its exact bytes as a Go `string` in `Value.Obj`: escapes not unescaped, inner whitespace kept, outer whitespace not (P3) |

**Which fault is reported** (review F4, QA A2): a syntax fault anywhere in the document wins; otherwise the first faulting attribute in **schema** order. The decoder MUST NOT return early on a type, null or missing fault: it scans to the end, keeping each slot's outcome from its last occurrence, then reports.

**Values are exact** (QA A8): every field of a `Value` other than its type's own is zero, whatever `dst` held before. **On error** the returned slice is nil and `Decoded` is zero.

**Edges, written down** (review F23, QA A4): a leading BOM is DecodeSyntax; the depth limit counts the top-level object (a value nested 9,999 deep is accepted, 10,000 is DecodeSyntax); a key with malformed UTF-8 is compared after U+FFFD replacement; raw DEL (0x7F) is legal inside a string.

### 4.2 Timestamps: the strict RFC 3339 reader (D4)

The value is a JSON string whose decoded text matches RFC 3339 §5.6 exactly:

```
date-time  = full-date ("T" / "t") full-time
full-date  = 4DIGIT "-" 2DIGIT "-" 2DIGIT          ; month 01–12, day 01–(28|29|30|31) by month and leap year
full-time  = 2DIGIT ":" 2DIGIT ":" 2DIGIT ["." 1*DIGIT] time-offset
                                                    ; hour 00–23, minute 00–59, second 00–60
time-offset = "Z" / "z" / ("+" / "-") 2DIGIT ":" 2DIGIT   ; hour 00–23, minute 00–59 ("-00:00" is UTC)
```

- The instant is the local date-time minus the offset, in ms since 1970-01-01T00:00:00Z. **Fraction digits beyond the third are dropped**, which rounds down (toward −∞) because the fraction is always added (P1).
- **A leap second** (`:60`) reads as that minute's `:59.999` (the last millisecond before the next minute), whatever its fraction.
- Anything else — a comma fraction, a one-digit field, `+24:00`, a missing seconds field, a space separator, surrounding spaces — is DecodeType. Years 0000–9999 only (four digits).
- The reader is hand-written in `internal/event`; tests check it against `time.Parse(time.RFC3339Nano, …)` on every form both accept, and against the grammar on the forms they disagree on.

### 4.3 Shape and messages

- One pass over `b` (a second over a failing payload is allowed): no `map[string]any`, no reflection, no `encoding/json` in production code.
- **Offset** (QA A3): DecodeType and DecodeNull, the first byte of the offending (last) value; a syntax fault, the offending byte, or `len(b)` for empty or cut-off input; DecodeMissing, the offset of the object's closing `}`.
- **Msg** names the attribute, the wanted and the found JSON kind, and the offset. Payload bytes appear only quoted with Go's `%q` (control and non-UTF-8 bytes escaped), at most 64 of them before quoting (review F13: no log forging).
- **Allocations, stated** (review F12): one per non-empty `string` value, two per `object` value, none otherwise. No allocation gate on the decoder in this sprint; the budget and ADR-0010 §1's annotation are S3.2b's (forwarded on its card).

### 4.4 Tests (QA, written first)

`TestDecodeDifferential` (exit 5's proof: fixed-seed payloads over schemas covering every type, optionality and both time clauses, 27 mutation classes, compared on accept/reject, reason, attribute and every value), `TestDecodeCases` (named cases with literal expectations), `TestDecodeReusesDst`, `TestDecodeConcurrent`, and `FuzzDecode` (`internal/event/fuzz_test.go`; nightly row `fuzz-decoder`, P7). For D4 the oracle's timestamp step becomes an independent strict RFC 3339 reference in the test.

### 4.5 The import law for `internal/event`

Production files may import `{cmp, errors, fmt, math, strconv, unicode/utf8, unicode/utf16}`, plus `sync` for `sync.Pool` only (none yet). Never `time` (D4), `encoding/json`, `reflect`, `sync/atomic` or a module package. The tests use `encoding/json` and `time` as oracles.

## 5. `archcheck`: the named purity exceptions (item 6; ADR-0015 §2)

- `internal/table`: may import `sync` and `sync/atomic`, and use only `sync.Mutex` and `atomic.Pointer`.
- `internal/event`: may import `sync` only, and use only `sync.Pool`.
- Any other identifier from them there (`sync.RWMutex`, `sync.Once`, `sync.WaitGroup`, `atomic.Int64`, `atomic.Value`, …), `sync/atomic` in `internal/event`, and a dot-import of either package in either directory (review F20: a dot-import hides the identifiers) are `purity` findings naming the allowed identifiers and ADR-0015 §2. The list sits in `archcheck`'s code beside today's `syncAllowed`; moving it to `tools/allowlists/` is S1.1c's (ADR-0015 §3.10).
- **Fixtures** (`archcheck_test.go`): each allowed use passes; each banned identifier, the event `sync/atomic` import and each dot-import is red with its message; any other package importing `sync` stays red, as today.

## 6. NaN and ±Inf (D2; ADR-0005 §3; review F3)

- In `internal/expr`, **every node that can produce a float** — `+ - * /` over floats (and int × float), `cast(… as float)` from a string, and any function returning a float — returns a `RuntimeError` when its result is NaN or ±Inf ("float result is not finite"), so the event is dropped for that rule and counted (SEMANTICS §10). `cast('NaN' as float)`, `cast('Inf' as float)`, `1e308 * 10` and `-1e308 - 1e308` are errors; finite results are unchanged.
- Literals cannot be NaN (the model refuses them); the decoder cannot produce one (JSON has none; overflow is DecodeType); tables refuse them. The Go API's row boundary (S3.3) MUST refuse non-finite floats too (forwarded with D3).
- Tests (QA): each case above red first; the comparison operators over finite values unchanged.

## 7. The gate

- `fuzz-decoder`: a nightly row, a copy of `fuzz-parser` with `expected_scope` `internal/event/fuzz_test.go`, `lands_in` `S3.1-2` (P7).
- Every other new test runs in `unit` (P6). Coverage stays above each package's floor.

## 8. Size

Items 3 [2], 4 [1], 5 [3], 6 [0.5] = 6.5; D2 (NaN) +0.5; D4 (the RFC 3339 reader) +0.5: **7.5 points**, under the cap; no lanes.

## 9. Review dispositions

| finding | disposition |
|---|---|
| F1 (blocker) | fixed: D1; `Row.Table`'s contract (§2); SEMANTICS §4 and ADR-0012 §5 annotated; the carrier forwarded to S3.2a (card annotated) |
| F2 (blocker) | fixed: §3's Cn rule |
| F3 (blocker) | fixed in this sprint: D2, §6 |
| F4 | fixed: §4.1 "which fault is reported" |
| F5 | fixed: D4, §4.2 |
| F6, F7 | forwarded to S3.2b item 4 (card annotated): one home for the flag; the undeclared and non-`bool` cases are validator questions for the owner |
| F8 | fixed: §2, only L3 takes snapshots; operator rows embed the runtime's row |
| F9 | fixed: §1 key equality; §6 |
| F10 | fixed: §2's `TestMembershipSnapshot` and the `-race` test |
| F11 | forwarded to S3.2a (the canonical encoder): an object's text keeps malformed UTF-8 verbatim; the encoder must say what it emits |
| F12 | stated in §4.3; the gate and ADR-0010 §1's annotation forwarded to S3.2b (card annotated) |
| F13 | fixed: §4.3 Msg |
| F14 | D3: the deploy error in S3.3 (card annotated); `New` as §1 meanwhile |
| F15 | forwarded to S3.2a (card annotated) with the review's recommendation |
| F16 | fixed: §2, the validator never returns id −1 |
| F17 | fixed: §2, a nil snapshot is a `RuntimeError` |
| F18 | kept `Get` (the tests and S3.3's sandbox read rows by key); it may allocate (§1) |
| F19 | fixed: §1, per-type field equality |
| F20 | fixed: §5, dot-imports |
| F21 | fixed: §2, the `expr → table` fence |
| F22 | stated in §1; later |
| F23 | fixed: §4.1 edges |
| F24 | fixed: §1 wording |
| QA A1 | D4 |
| QA A2, A3 | fixed: §4.1 and §4.3 |
| QA A4, A5, A8 | fixed: written into §4.1 |
| QA A6, A7 | A6 with F7 (S3.2b); A7 as QA assumed (§4.1 step 2) |
| QA A9 | fixed: §4.1, nil and zero on error |
