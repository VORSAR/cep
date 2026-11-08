# S3.2c — design note: an overflow is an expression error (v2)

> **Historical — S3.2c, the spec cleanup: the language documents agree, and overflow is an error; merged in PR #21.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** **design** · [record](s3-2c-record.md) · reviews: [combined build: architecture](../reviews/s3-2c-combined-architecture.md), [break-it: QA](../reviews/s3-2c-breakit-qa.md). **← [S3.2b-2](s3-2b-2-record.md) · [S3.3-1](s3-3-1-record.md) →**

**What it covers:** item 3 of the [S3.2c card](history.md#s32c--the-spec-cleanup-the-language-documents-agree-and-overflow-is-an-error) and its exits 3 and 4; items 1 and 2 are documentation and a test, done ([record](s3-2c-record.md)).

**The owner's decisions** ([2026-11-06](decisions.md#owner-decisions-2026-11-06--the-open-items-after-s32b-2), decision 3; [S3.2c's plan check](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check), decision 3): "Integer overflow is an expression error for that rule … the event is dropped for that rule only and counted; other rules see it. No rule acts on a wrong number." It covers "every 64-bit operation: ints, durations and timestamps alike".

**Routine (owner):** QA's tests first; this note and the build; one architecture review of the note and the code together; a short QA break-it; one fix round under the race detector before T4.

## 1. Where it lands

`internal/expr` only. Every operation evaluates in `nodes.go` (`arith.Eval`, `neg.Eval`, `abs.Eval`; `registry.go` only builds `abs`). An expression error there is already per-rule: `plan`'s body counts it in `Stats.Errored` and drops the event for that rule (SEMANTICS §10, built in S3.2b-1), and the runtime sums it into `RuleStats.Errored`. Nothing outside `expr` changes.

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
- **Timestamps:** `timestamp ± duration` overflowing int64 is this error. A sum that fits int64 but lies outside years 0000–9999 is not overflow: it is S5.1's forward (the sink's encoding). *(v2: "past year 9999" until the review's R3; owner, 2026-11-07.)*
- **Floats** are unchanged: a non-finite result is already an error (S2.2a).

## 3. The reference executor

`docs/language/checks/executor.py` gains the checked 64-bit helper QA's harness check names, raising the executor's expression-error type on a result outside `[-2^63, 2^63−1]` (Python's ints are unbounded, so the check is a range test), with division truncating toward zero as Go's does.

## 4. Tests

QA's, first and red against today's wrapping (exit 3: a table row per operation one step past its bound; exit 4: the in-range rows at each bound; a live test where one rule errors and another fires, live and in replay; the harness check). The build adds a test only where QA's leave a line uncovered.

## 5. Size

About 1.5 points, as carded.

## 6. The fix round (v2; the owner's decisions of 2026-11-07)

The review ([combined](../reviews/s3-2c-combined-architecture.md)) and the break-it ([QA](../reviews/s3-2c-breakit-qa.md)) found no blocker; the owner added two items ([decisions](decisions.md#owner-decisions-2026-11-07--s32cs-review-and-break-it)). Each has a test that failed first.

- **`timestamp − timestamp` → duration** (decision 1): one more `arith` kind, `arithTsTs`, typed duration, computed with `subInt`, so an overflow is this sprint's error. `timestamp + timestamp` stays E-TYPE-001.
- **The smallest int is a writable literal** (decision 3): `internal/parser/lower.go`. A param default's minus sign is parsed with its number (`ParseInt("-" + digits)`). In an expression, unary minus directly over the integer literal whose digits fit only with the sign (9223372036854775808) lowers to one int literal; unary minus over any other number keeps its `ExprNeg` form. The one model change elsewhere: a negative int param default's unused float field was −0 (the negated zero) and is now 0 — the `torture-lexical` golden. The grammar is unchanged.
- **A negative duration in a bound slot is E-DUR-003** (break-it K1): `internal/validate/bounds.go` refuses `Ms < 0` (and a written zero) for every bound slot and `idle after`; a negative ttl is not also compared against the bounds (E-TTL-002). Only an override or a model built in Go can carry one.
- **Documents** (review R1, R2, R4, R5, R7): SEMANTICS §6.4's absence form and both §5.2 references; SPEC §5.10 lists every accepted form; the plain-rule wording of SPEC §4.2 and SEMANTICS §2.1 follows errors.md; the stale counts go; Appendix B's order is tested.
- **Forwarded** (review R6, later): the overflow messages' units, with the logging of expression errors at S5.2. `neg.Eval` now takes its operand before assigning the result.
