# S3.2c break-it: overflow (QA, short)

> **Historical — S3.2c, the spec cleanup: the language documents agree, and overflow is an error; merged in PR #21.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-06--s32cs-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-2c-design.md) · [record](../sprints/s3-2c-record.md) · reviews: [combined build: architecture](s3-2c-combined-architecture.md), **break-it: QA**. **← [S3.2b-2](../sprints/s3-2b-2-record.md) · [S3.3 (next)](../sprints/sprint-plan.md#status) →**

Branch `s3-2c/breakit`, worktree `/Users/ramith/code/cep-wt/s3-2c-breakit`, commits 5796397..7fc4189 on top of 9b1b336.
GitNexus index reported 46 commits behind HEAD, so I confirmed every claim against the source with grep/read. No Hindsight page was used.

## Findings

| id | severity | what breaks | red test (file) | failing line | suggested fix |
|---|---|---|---|---|---|
| K1 | should-fix | A negative duration override in a bound slot deploys with no diagnostic. A zero one is refused (E-DUR-003, "use a positive duration"). `validate.Options.Overrides` is a `model.Literal` that the caller builds, so `output first every ${D}` with D = -5 sec passes. At runtime `plan` applies the rate limit only when `every > 0`, so it is silently off and every output is emitted. `partition.addSat` ("a + b for b >= 0") computes `math.MaxInt64 - b`, which wraps when b is negative. Only an internal Go caller can reach this today: source cannot write a negative duration, and no public deploy surface exists yet. | `TestBreakitANegativeDurationOverrideInABoundSlotIsRefused` in `internal/validate/s3_2c_breakit_test.go` | `s3_2c_breakit_test.go:31`: "no diagnostic; output first every is {Ms:-5000 …}" | `validate.zeroDurations` (bounds.go:155) should refuse `d.Ms <= 0`, not only `== 0`, for every bound slot (or E-PARAM-001 at resolve for a negative duration override). Then `addSat`'s precondition holds. |
| K2 | later | The smallest int, -9223372036854775808, is in range but cannot be written. `lowerer.signedLiteral` (param defaults) and `ExprNeg` over a number literal (expressions) parse the digits without the sign, and 9223372036854775808 alone is outside int64, so the source gets E-TYPE-001 "integer 9223372036854775808 is out of range". In expressions you can write `-9223372036854775807 - 1` instead. A param default has no source workaround. This predates S3.2c and is next to the decision, which names "the smallest int". | `TestBreakitTheSmallestIntIsAWritableLiteral` in `internal/parser/s3_2c_breakit_test.go` | `s3_2c_breakit_test.go:22` (param default) and `:34` (select literal): "E-TYPE-001: integer 9223372036854775808 is out of range" | In `lower.go`, parse a negated number literal with its sign (`ParseInt("-"+text)`): in `signedLiteral`, and in `atom` when `a.Neg` is a bare number literal, fold it to a literal. Otherwise state the literal range in SPEC. This is a language change, so the owner decides. |

No blockers. Nothing a rule can reach through `internal/expr` still produces a wrapped number, and no in-range result changed.

## Guards added (green)

Commit 5796397:
- `internal/expr/s3_2c_breakit_test.go`, `TestBreakitArithmeticAgreesWithBigInts`: 15 node forms run 3000 times each on seeded operands, biased towards ±2^63, ±2^62, ±2^31/2^32, ±3037000500 and small values, and compared with `math/big`. The forms are int + - * / %, -int, abs(int), duration ± duration, duration × int, int × duration, duration / int, -duration, timestamp ± duration. Results that fit are exact; results that don't fit are a `*expr.RuntimeError` with a null of the node's type; division by zero keeps its own error.
- Same file, `TestBreakitOverflowInsideACompositeNeverYieldsAValue`: 21 cases. An inner overflow is still an error when the outer operation would bring the wrapped value back into range: `(i + j) - j` at MaxInt64, `(i * j) * 0`, `(i * j) + i`, `(i * j) % 7`, `(d * i) / i`, `(t + d) - d`, `-(-i)`. None of comparison, `not`, `is [not] null`, `coalesce`, `minimum`, `maximum`, `cast`, `abs`, int-to-float arithmetic, a taken `ifThenElse` branch, `and` or `or` turns the error into a value. An untaken `ifThenElse` branch is not evaluated: no error.
- `internal/playback/s3_2c_breakit_test.go`, `TestBreakitDurationAndTimestampOverflowDropsTheEventForThatRuleOnly`: one stream with a duration attribute feeds four rules. Each rule overflows on a different event: `ts + d` in a filter, `-d` and `d * n` in selects, plus a plain witness rule. Each rule drops and counts only its own event (`RuleStats.Errored` 1/1/1/0). The boundary results that fit (`-MaxInt64 ms`, `MinInt64 ms`) are exact. Live and replay agree byte for byte, with the same counters.

## What held

- `addInt`/`subInt`/`mulInt`/`divInt`/`negInt` (nodes.go:396-424) and `abs` are correct on every bound I tried, including `-1 * MinInt64`, `MinInt64 * -1`, `MinInt64 / -1` and `MinInt64 % -1 = 0`.
- Overflow paths outside `arith`/`neg`/`abs`. The registry is `coalesce`, `minimum`, `maximum`, `abs`, `math:round`, `cast`, `ifThenElse`, plus `count`, which is not runnable yet; only `abs` does 64-bit arithmetic. Comparisons use `cmpI` and do not subtract. `cast` checks float to int exactly (`[-2^63, 2^63)`), and `ParseInt` refuses string to int out of range.
- Duration literals are checked at parse time (`lower.go` `duration`: `count > (MaxInt64-total)/unit` gives E-DUR-002). Event timestamps are limited to years 0000-9999 when decoded.
- Engine timing arithmetic saturates and does not wrap. That covers the rate limit (`plan.addSat(a.At, every)`), the TTL purge (`partition.addSat(ts, ttl)`), and the watermark and idle arithmetic (`clock.add`/`clock.sub`). With extreme event times these stay correct. The only wrap I found is the negative-b case in K1. `within`/`for` patterns are not built yet, so they are not reachable.
- The reference executor's `checked_int64` (`docs/language/checks/executor.py:84`) matches Go by inspection: truncating `/`, `%` keeps the dividend's sign, `MinInt64 / -1` is an error, `MinInt64 % -1` is 0, and results are range-checked. I did not run it against Go.

## Observed, not filed (outside the overflow decision or already decided)

- A timestamp sum that fits int64 but lies past year 9999 (or before 0000) is emitted, and `playback/encode.go` refuses to encode it (`errTimeRange`). The design note forwards this to S5.1.
- int × float comparisons and arithmetic, and the select widening of an int into a float target, convert through `float64` and lose precision above 2^53. This is SPEC §5.10's promotion, not overflow.
- `true or <overflow>` and `false and <overflow>` short-circuit with no error, as division by zero does. SPEC does not cover it.
- SPEC §5.6 says an expression error is "counted, logged". It is counted (`Stats.Errored`), but nothing in `internal/` logs. This dates from S3.2b-1 and also applies to division by zero.
