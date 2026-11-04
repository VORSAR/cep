# S3.1-2 break-it: tables and membership, NaN/±Inf, IngestFlags, the decoder, archcheck's exceptions (breaker QA)

**Base:** d22fbc1 (`s3-1-2/integration`). **Branch:** `s3-1-2/breakit` (worktree `/Users/ramith/code/cep-wt/s3-1-2-breakit`). Not pushed.
**Red-test commits:** 48d2c00 (B1, B2, B7), 6168cb8 (B1 blank-import variant, B3), 5b92cdb (B5; guards), 5c9b446 (B4, B6; guard), 73b7ada (guards only: decoder and IngestFlags).
**Test runs:** the 7 new red tests fail alone (`go test -run '^Name$' ./<pkg>/`, exit 1 each). With `-skip` over them, `./tools/archcheck/`, `./internal/expr/` (`-race`) and `./internal/table/` (`-race`) pass, and `./internal/event/ ./internal/text/` (`-race`) pass. `go vet` on all five packages: exit 0.

**Result:** 4 blockers, 3 should-fix, 0 later. Three of the blockers (B1-B3) are in this sprint's code. B4 is older than this sprint, but it is a blocker under rule 15.

Method: I read design note v2 (§1-§6, §9) and the code under attack adversarially. I wrote edge payloads against the existing §4.1 oracle (`oracleParse`/`oracle`/`disagreement`) over all 19 test schemas, plus exact syntax offsets that the differential does not check. Table key encoding got a randomized collision property test against a per-type reference. I probed archcheck with in-memory sources through `CheckSources`. I confirmed that Go 1.27.3 builds a file that imports the same package twice. The fuzz run is below. Not tried: whole-engine end-to-end (the runtime does not exist yet), and table writes vs. readers beyond the existing `-race` test.

## Findings (ranked)

| id | sev | contract broken | red test | repro (one line) |
|---|---|---|---|---|
| B1 | blocker | design §5, ADR-0015 §2: in table only `sync.Mutex`/`atomic.Pointer`, in event only `sync.Pool`; any other identifier is a purity finding | `tools/archcheck/breakit_test.go` `TestBreakitNamedSyncSecondImportName` | `import ("sync"; s2 "sync")` in internal/table, then `s2.WaitGroup`: no finding. Same for `a2.Int64`, event's `s2.RWMutex`, and `import (_ "sync"; s "sync")` |
| B2 | blocker | design §2, review F21, rules.json `fences`: expr production code uses `table.Version*` only | `TestBreakitExprFenceSecondImportName` | internal/expr imports `.../internal/table` twice (`t2` second), calls `t2.New(...)` and `Upsert`: no finding |
| B3 | blocker | the same fence; also the owner's expr to model fence (2026-11-04) | `TestBreakitFenceDotImport` | internal/expr `import . ".../internal/table"`, then `New("T",nil,nil)` and `Upsert`: no finding. `import . ".../internal/model"` with `App{}`: no finding |
| B4 | blocker (older than this sprint) | SPEC App. F "failed cast = runtime error"; ground rule 11 (non-deterministic replay) | `internal/expr/breakit_test.go` `TestBreakitCastOutOfRangeFloatToInt` | `cast(1e19 as int)` gives MaxInt64 on arm64 with no error (Go's `int64(f)` is implementation-defined; amd64 gives MinInt64). Also 2^63, ±MaxFloat64, -2^63-2048 |
| B5 | should-fix | design §1: "copies every row it stores; a caller may reuse its slices" and "A published Version MUST NOT change" | `internal/table/breakit_test.go` `TestBreakitObjAliasingChangesPublishedVersion` | Upsert (or Load) a row with `Obj: []byte("{\"v\":1}")`, then `copy(buf, "{\"v\":2}")`: the published Version's row now reads v:2 |
| B6 | should-fix (older than this sprint) | SPEC App. F: `math:round` is half-up | `TestBreakitRoundHalfUpExact` | `math:round(0.49999999999999994)` = 1 (want 0); `math:round(4503599627370497.0)` = ...498 (want ...497); its negative gives ...496 |
| B7 | should-fix (older than this sprint; same root as B1/B2) | ADR-0015 §2.2 clock rule | `TestBreakitClockSecondImportName` | internal/plan `import ("time"; t2 "time")`, then `t2.Now()`: no clock finding |

## Blockers

### B1: a second import name escapes archcheck's named sync exceptions (the gate misses banned sync identifiers)

- **Where:** `tools/archcheck/main.go` `namedSync` (l.314-331) uses `localName(p)` (l.231-241), which returns only the first import spec's name. If that name is `_`, `namedSync` skips the package entirely (l.317).
- **Scenario:** a production file in internal/table imports `"sync"` and also `s2 "sync"` (or `_ "sync"` and then `s "sync"`) and uses `s2.WaitGroup`, `s2.RWMutex`, `a2.Int64`, and so on. Go 1.27.3 compiles this. I checked with `go build` in a scratch module.
- **Evidence:** `TestBreakitNamedSyncSecondImportName` has 4 subtests (table WaitGroup, table Int64, table blank-first WaitGroup, event RWMutex). Each returns `[]` findings where design §5 wants a purity finding.
- **Honest likelihood:** this takes a deliberate double import, which no one writes by accident. But the check exists to keep agents inside ADR-0015 §2, and an agent working around a red purity finding could land on it. When it happens, nothing in the gate notices.
- **Fix area:** `localName` should return every local name the file gives the path, and `namedSync`, `fences` and `clock` should check selectors on all of them. A simpler alternative is to make importing the same path twice a finding of its own.

### B2: a second import name escapes the expr to table fence (expr production code can build and write tables)

- **Where:** `tools/archcheck/main.go` `fences` (l.203-229), through the same `localName`.
- **Scenario:** an internal/expr production file imports `.../internal/table` and also `t2 ".../internal/table"`, then calls `t2.New(...)` and `tb.Upsert(nil)`.
- **Evidence:** `TestBreakitExprFenceSecondImportName` gets `[]` findings.
- **Honest likelihood:** same as B1. It defeats review F21's "expr reads tables and never writes them".
- **Fix area:** the same as B1.

### B3: a dot-import escapes every fence (expr to table, expr to model)

- **Where:** `tools/archcheck/main.go` `fences`. `local` is `"."` for a dot-import, and no `SelectorExpr` has `"."` as its qualifier, so nothing is ever checked. Design §5 bans dot-imports only for sync and sync/atomic in table and event (F20). The fences have no such rule.
- **Scenario:** internal/expr has `import . "github.com/vorsar/cep/internal/table"`, then `tb, _ := New("T", nil, nil); tb.Upsert(nil)`. Or it dot-imports internal/model and uses `App{}`.
- **Evidence:** `TestBreakitFenceDotImport` (2 subtests) gets `[]` findings for both.
- **Honest likelihood:** this is more plausible than B1/B2, because a dot-import is a single ordinary line. The fence then means nothing.
- **Fix area:** in `fences`, report a dot-import of any fenced package, as F20 already does for sync. Or resolve the bare identifiers against the package's exports.

### B4: `cast(f as int)` of a whole float outside int64 returns a value that depends on the platform, with no error

- **Where:** `internal/expr/nodes.go` `cast.Eval`, the `TInt` from `TFloat` branch (l.534-538). `v.F != math.Trunc(v.F)` passes for every whole float, including those ≥ 2^63 or < -2^63, and then `int64(v.F)` runs. The Go spec leaves that result implementation-defined.
- **Scenario:** an event with `f = 1e19`, and a rule `cast(f as int) > 0`. On arm64 the cast gives MaxInt64 and the rule fires. On amd64 it gives MinInt64 and the rule does not fire. The same log replays differently across machines.
- **Evidence:** `TestBreakitCastOutOfRangeFloatToInt` has 7 subtests failing on this arm64 Mac, e.g. `cast(1e+19 as int) = {I:9223372036854775807}, <nil>`. The guard subtest `-2^63` (a real int64) passes.
- **Honest likelihood:** this needs a float of magnitude ≥ 9.2e18 cast to int. That input is rare, but the result is silently wrong and not deterministic across architectures, which is a stop-the-line class under rule 11. It is older than S3.1-2 and outside this sprint's design. I am reporting it under rule 10/11 so it can be forwarded.
- **Fix area:** in `cast.Eval`, return a RuntimeError when `v.F < -0x1p63 || v.F >= 0x1p63`.

## Should-fix

### B5: the table's row copy is shallow, so a caller's reused `Obj` buffer changes a published Version

- **Where / scenario:** `internal/table/table.go` `prepare` (l.177, `slices.Clone(r)`). `Value.Obj` is `any`, and `checkValue` accepts any dynamic type. The existing `TestKeyEquality` already passes `[]byte` as Obj. If a caller writes `Obj: buf` and later reuses `buf`, the row inside the published Version changes.
- **Evidence:** `TestBreakitObjAliasingChangesPublishedVersion`: for both Upsert and Load, the row's Obj changes from `{"v":1}` to `{"v":2}` after the write returned.
- **Honest likelihood:** today's only Obj producer, the decoder, stores a Go string, so nothing triggers this yet. S3.3's Go API row boundary will. It breaks a MUST, but there is a workaround (pass strings), so I rate it should-fix.
- **Fix area:** either `checkValue` requires a `string` Obj for TObject, or S3.3's RowWriter converts. The design note should name which one.

### B6: `math:round` is not half-up at the edges

- **Where:** `internal/expr/nodes.go` `round.Eval` (l.498). `math.Floor(v.F + 0.5)` rounds during the addition.
- **Evidence:** `TestBreakitRoundHalfUpExact`: 0.49999999999999994 gives 1, 4503599627370497 gives 4503599627370498, and -4503599627370497 gives -4503599627370496.
- **Honest likelihood:** deterministic, and only within 1 ulp of .5 or for |x| ≥ 2^52. It is older than this sprint. The reference executor does not implement round, so there is no oracle cross-check.
- **Fix area:** `t := math.Floor(x); if x-t >= 0.5 { t++ }`, which returns x unchanged when |x| ≥ 2^52.

### B7: the clock rule has the same second-import hole

- **Where / evidence:** `tools/archcheck/main.go` `clock` (l.255, `localName("time")`). `TestBreakitClockSecondImportName`: in internal/plan, `t2.Now()` gets no finding. This is older than this sprint, but the fix is the same as B1's.

## Tried and held

- **Tables:**
  - Composite keys: 3,000 random keys over strings holding `;`, `:`, `1:`, `true;`, `i1;`, `f0;` and NUL, plus bool, ±0, int/timestamp/duration with equal numbers, MinInt64 and MaxInt64. No collision against a per-type reference, and Get finds every row (`TestBreakitCompositeKeysNeverCollide`).
  - Get with a later column of the wrong Kind cannot collide: each kind's encoding starts with a distinct character class.
  - `Has` does not allocate for any of the six key types (`TestBreakitHasAllocatesNothingForEveryKeyType`).
  - All-or-nothing: `prepare` and `checkKey` run before the lock.
  - Load vs Upsert ordering under the lock is linearizable.
  - Delete of absent keys publishes a Version.
- **Membership:**
  - A null operand is false for `in` and `not in` even with a nil snapshot, and an operand error beats a nil snapshot (`TestBreakitMembershipPrecedence`).
  - -0 finds 0. A NaN cannot reach `in`: every float source is checked.
- **NaN/±Inf (D2):**
  - neg, abs, round, minimum/maximum, coalesce and ifThenElse cannot make a non-finite value from finite inputs.
  - Casts from int and bool are finite or an error. String-to-float underflow gives 0, not an error.
  - Float literals: the parser refuses Inf (`parser/lower.go:435`) and the model refuses NaN/Inf (`model.go:411`).
  - The existing `finite_test.go` already covers every arithmetic case on the brief's list.
- **IngestFlags:**
  - C1 controls, DEL, U+FEFF, Tags block, noncharacters (U+FDD0, U+FFFE, U+1FFFE, U+FFFFE, U+10FFFF), and private-use planes 15/16 all flag.
  - NBSP, U+2028 and U+FFFD do not flag.
  - Overlong, CESU-surrogate, past-U+10FFFF and truncated sequences shaped like invisible code points all read as U+FFFD and do not flag.
  - A flag in the last value is found. Null strings, objects and non-strings are skipped. 0 allocations (`TestBreakitIngestFlagsEdges`).
- **Decoder:** about 120 edge payloads × 19 schemas agree with §4.1's oracle on accept/reject, reason, attribute, Offset and every value, and §4.3's message shape holds (`TestBreakitDecodeEdgesAgreeWithOracle`). They cover:
  - escaped keys (`"t\u0073"`, `"\u0074ext_invisibles"`, 40 escaped chars past the 64-byte buffer) and keys with malformed UTF-8;
  - duplicate keys whose earlier value is wrong-typed or null;
  - strings over 128 bytes with and without escapes;
  - every surrogate shape (pair, lone high, lone low, reversed, high-high-low, escaped backslash, upper-case hex), plus raw CESU and past-max bytes;
  - nesting exactly at 9,999 and 10,000 inside an unknown key and inside a declared object;
  - the numbers `-0`, `1E400`, `0.000…1e400` (400 zeros), `1e-400`, `±9223372036854775808/9`, a 40-digit int, and float min/max subnormals;
  - RFC 3339: years 0000 and 9999 with ±23:59 offsets, Feb 29 in 1900/2000/2100/2023/2024, `:60` with `+05:30` and with a fraction, a 20-digit fraction, the instant 0, -1 ms and -0.1 ms, escaped `Z` and `-`, a trailing or leading space, NBSP, full-width digits, `+01:00:00`, `+1:00`, `-23:60`, month 00/13, day 00, Apr 31, sec 61, min 60, and a `+` year.
- **Decoder offsets and concurrency:**
  - 30 syntax offsets are exact: the offending byte, or len(b) for cut-off input (`TestBreakitSyntaxOffsets`).
  - One Decoder shared by 8 goroutines under `-race` matches the serial results (`TestBreakitDecoderSharedConcurrently`).
- **archcheck:**
  - An allowed type with a banned type argument (`atomic.Pointer[sync.Once]`), embedding `sync.RWMutex`, and a type alias of `sync.WaitGroup` are all caught.
  - A non-test file cannot use `test_fences` (`c.test` comes from the `_test.go` suffix).
  - `import "C"` gets no archcheck finding, but the gate pins `CGO_ENABLED=0` (`gate/runner/env.py`), so it does not build.
  - Not tested: laundering through an exported alias in another allowed package (e.g. event exporting `type Pool = sync.Pool` for table). No such alias exists today.

## Fuzz run

`go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 5m ./internal/event/` (pinned go1.27.3, from the worktree): **exit 0**, 9,618,936 execs, 328 new interesting inputs (1,205 total in the cache), no crasher. Nothing was added to `testdata/fuzz`.

## Commits

48d2c00, 6168cb8, 5b92cdb, 5c9b446, 73b7ada (on `s3-1-2/breakit`, base d22fbc1).
