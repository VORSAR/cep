# S3.2c — design note: an overflow is an expression error (v1)

**What it covers:** item 3 of the [S3.2c card](sprint-plan.md#s32c--the-spec-cleanup-the-language-documents-agree-and-overflow-is-an-error) and its exits 3 and 4; items 1 and 2 are documentation and a test, done ([record](s3-2c-record.md)).

**The owner's decisions** ([2026-11-06](decisions.md#owner-decisions-2026-11-06--the-open-items-after-s32b-2), decision 3; [S3.2c's plan check](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check), decision 3): "Integer overflow is an expression error for that rule … the event is dropped for that rule only and counted; other rules see it. No rule acts on a wrong number." It covers "every 64-bit operation: ints, durations and timestamps alike".

**Routine (owner):** QA's tests first; this note and the build; one architecture review of the note and the code together; a short QA break-it; one fix round under the race detector before T4.

## 1. Where it lands

`internal/expr` only. Every operation evaluates in `nodes.go` (`arith.Eval`, `neg.Eval`) and `registry.go` (`abs`). An expression error there is already per-rule: `plan`'s body counts it in `Stats.Errored` and drops the event for that rule (SEMANTICS §10, built in S3.2b-1), and the runtime sums it into `RuleStats.Errored`. Nothing outside `expr` changes.

## 2. The checks (MUST hold exactly)

With `x, y int64` and `r` the wrapped result Go computes:

| Operation | Overflows when | Kinds it covers |
|---|---|---|
| `x + y` | `((x ^ r) & (y ^ r)) < 0` | int; duration ± duration; timestamp + duration |
| `x - y` | `((x ^ y) & (x ^ r)) < 0` | int; duration − duration; timestamp − duration |
| `x * y` | `x != 0 && (r/x != y || (x == -1 && y == math.MinInt64))` | int; duration × int; int × duration |
| `x / y` | `x == math.MinInt64 && y == -1` (division by zero stays its own error, checked first) | int; duration / int |
| `x % y` | never (`math.MinInt64 % -1` is 0 in Go, and 0 is right) | int |
| `-x` | `x == math.MinInt64` | int; duration |
| `abs(x)` | `x == math.MinInt64` | int (`abs` takes ints and floats only) |

- **An overflow returns** `event.Value{Kind: <the result kind>, Null: true}` and `runtimeErr("integer overflow: …")`, naming the operator and both operands, as division by zero does.
- **One helper per operation** (`addInt`, `subInt`, `mulInt`, `divInt`, `negInt`) in `nodes.go`, used by every kind it covers, so the rule is written once.
- **MUST:** no result that fits changes; an overflow never yields a value; the check adds no allocation on the path that does not overflow (`allocgate` holds — the error's message is built only when it overflows).
- **Timestamps:** `timestamp ± duration` overflowing int64 is this error. A sum that fits int64 but lies past year 9999 is not overflow: it is S5.1's forward (the sink's encoding).
- **Floats** are unchanged: a non-finite result is already an error (S2.2a).

## 3. The reference executor

`docs/language/checks/executor.py` gains the checked 64-bit helper QA's harness check names, raising the executor's expression-error type on a result outside `[-2^63, 2^63−1]` (Python's ints are unbounded, so the check is a range test), with division truncating toward zero as Go's does.

## 4. Tests

QA's, first and red against today's wrapping (exit 3: a table row per operation one step past its bound; exit 4: the in-range rows at each bound; a live test where one rule errors and another fires, live and in replay; the harness check). The build adds a test only where QA's leave a line uncovered.

## 5. Size

About 1.5 points, as carded.
