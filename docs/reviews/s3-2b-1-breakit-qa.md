> **Status (2026-11-06):** open — B1 awaits a fix with this red test, or the owner's forwarding.

# S3.2b-1 break-it — the plan, replay through the runtime, rate limits (breaker: QA, with the fool's lens)

**Base:** 8214be8 (`s3-2b-1/integration`). **Branch:** `s3-2b-1/breakit`, not pushed.
**Red-test commits:** 195fff8 (B1).
**Guard commits:** 195fff8 (also holds G1), c737662 (G2), dff55b9 (G3).
**`go test ./internal/playback -count=1`:** 135 tests and subtests ran (134 passed). Exit 1. The only failure is the new break-it test `TestBreakitPartitionKeyNotDropped`.

**Result:** 1 blocker, 0 should-fix, 2 later (observations, no red test).

Method:
- Read adversarially: design v2 §1–§7, the architecture review, sprint-plan §5 (Q1 and D1–D5), and SEMANTICS §8 and §9 with their 2026-11-06 annotations.
- Read the code under test: `plan.go`, `runtime/{runtime,sync,ingest}.go` (the router's `event`, `broadcast` and `shard.process`), `partition.go` (`Offer`, `arrive`, `reset`, `free`), `validate/exprs.go` (`selects`, `mayBeNull`), `validate/bounds.go` (`partition`), and `expr/{expr,registry,nodes}.go`. I checked each node's `Eval` for paths that give null or always fail.
- Ran a throwaway harness (not committed) that sent about 30 edge programs through `parser.Parse` → `validate.Check` → `plan.Build`. The programs covered `raw(x)`, `raw(x) as y`, int into float, an object passed through, an object, optional, float or `ts` partition key, a target with no `ts`, a target `ts` declared optional, insert into a source stream, a rule reading a derived stream, a partition on another stream, TTL < D (literal and params), D = 0, a huge D and TTL, a `time arrival` source, `text_invisibles`, two producers of one stream, `-n`, `ifThenElse` with a null branch or a null condition, a `null` literal, `not in` on an optional value, and two keys of one stream.
- Committed tests drive the real rule text through `NewPlanDriver`, through `runtime.Sync` directly, and through `runtime.Start` with 7 shards under `-race`.

## Blockers

### B1 — a partition clause with two keys of one stream passes the validator, and `plan.Build` keeps only the last (a missed alert)

- **Where:** `internal/plan/plan.go` `rule()`. The loop over `r.Partition.Keys` overwrites `pr.KeyAttr`, `pr.KeyType` and `pr.TTL` for every key of the source stream, so the last key wins. `internal/validate/bounds.go` `partition()` checks that every read stream has a key and that the key types agree. It never refuses a second key of the same stream.
- **Scenario:** `partition by k of S, j of S ttl 10 min`, `from S select k, j output first every 5 min`. Replay S{k:a, j:x}@10:00, then S{k:b, j:x}@10:01. The partitions (a,x) and (b,x) are different values, each with its own rate-limit anchor (SEMANTICS §9). The engine partitions by `j` alone, so b's alert is suppressed by a's anchor.
- **Evidence:** `TestBreakitPartitionKeyNotDropped` in `internal/playback/breakit_s3_2b_1_test.go` gives `rule keys [1] of S (k dropped): 1 alerts [...], want 2`. The test also passes if the program is refused at deploy, by a validator finding or a `Build` error.
- **Honest likelihood:** low to moderate. An author who wants a composite key ("per tenant and app") will write exactly this, since the clause takes a list and nothing refuses it. When it happens, alerts are lost silently, with no lint and no counter. Keys merged across `k` also share a TTL. No corpus program does this.
- **Fix area:** a language decision for the owner, in order of accuracy:
  1. Support a composite key: one key value per stream made of several attributes. This touches routing (`KeyAttr` per input becomes a list), `partition`'s `keyT` and E-PART-003.
  2. Refuse a second key of one stream at deploy, with a new code or an E-PART-002/003 extension. This is a language change, so it needs errors.md, the catalogue and an invalid example.

  Either way, `plan.Build` should not silently drop a declared key: an error is the safe floor.

## Later

### L1 — E-NULL-003 refuses selects that can never be null at run time (conservative false positives)

- **Scenario:** `select ifThenElse(n is null, 0, n) as n` into a non-optional `n` gets E-NULL-003. So does selecting a partitioned rule's own `optional` key into a non-optional attribute, which the router never lets through as null (a null key is a key error). The equivalence program met this and declared its output attribute optional.
- **Evidence:** the throwaway harness (`ifelse_nullbranch` → E-NULL-003). No red test: the design §1 rule says "ifThenElse (nullable when either branch is)" and "an optional attribute", so this is the stated contract, not a defect against it.
- **Likelihood / fix area:** the message already offers `coalesce(...)` as a workaround. A sharper analysis (a guard such as `x is null` in the condition, or a partition key of the rule) belongs in `validate.mayBeNull` if the owner wants it. Otherwise it is backlog polish.

### L2 — `plan.App` is not self-contained: `NewBody` compiles from the checked model at `Start`

- **Where:** `plan.go` `rule()` keeps `scope{app}` and `model.Expr` copies whose `Args` slices share the model's arrays. `NewBody` compiles from them when each shard is built.
- **Scenario:** a caller that reuses or mutates the checked `*model.App` after `Build` changes what the deployed rules run. Nothing in the tree does this today; the S3.3 facade will own that model.
- **Evidence:** read only, no test (outside the time box).
- **Fix area:** compile once in `Build` into a self-contained spec (deep-copy the nodes), or document that `Build`'s input must not change while the plan is in use.

## Tried and held

- **G1 (committed, 195fff8), MUST 10:** a non-matching event of a resident key allocates nothing on the `Sync` path even when every event is 1 ms later, so S's admission and a second stream's virtual floor move each step. QA's own test held time still.
- **G2 (committed, c737662):** `runtime.Start` on 7 shards ran a keyed `not in` rule and a key-less rate-limited rule. Four concurrent senders (8,000 events), a goroutine doing 200 table puts, ticks and `Stats` scrapes, and a drain all ran under `-race`, three runs. It was race-free, with no panic, construct-late, key error or expression error, and with outputs and suppressions present.
- **G3 (committed, dff55b9):** the rate limit on `Sync`:
  - a tie at t_out is suppressed;
  - an output older than the anchor but accepted by its stream is suppressed and does not re-anchor;
  - exactly t_out + D is suppressed, and +1 ms is emitted and re-anchors;
  - the next exact t_out + D is suppressed again;
  - another key is independent;
  - Suppressed counts 4.
- **`plan.Build` on validator-accepted programs:** `raw(x)` with or without `as`, int into float, an object passed through, a target without `ts`, a `ts` partition key, a float key on an ascending stream, huge D and TTL (MaxInt64 ms), `time arrival`, `text_invisibles` and `-n` all build. The validator refuses insert into a source stream (E-TIME-002), a rule reading a derived stream (E-GRAPH-004), two producers (E-GRAPH-003), TTL < D with literals and with params (E-TTL-002), and D = 0 (E-DUR-003).
- **Object partition key:** `Build` accepts it (KeyType object) and `partition.NewRule` refuses it at `Start` or `NewSync` with an error, not a panic. This matches the recorded S3.2a-1 D7 (object keys become a deploy error in S3.3).
- **E-NULL-003 soundness (read):** apart from `coalesce`, every node that can give null does so only from a null input. A null `ifThenElse` condition, a division by zero, a non-finite float and a failed cast are errors, not nulls. Comparisons, `in`/`not in`, `not`/`and`/`or` and `is null` are two-valued. Live `Send` refuses a null in a non-optional attribute. No false negative found.
- **`expr.BuildRunnable` (read):** the only nodes whose `Eval` always errors are the `later` nodes (text operators, `count`, `chain`), and `BuildRunnable` catches all three.
- **Owner D1 (read):** an on-arrival rule's clock is fed only `m.adm` for its own stream, plus advances marked `final` (drain and end). Ticks and virtual floors are filtered in `shard.process`. A shard with no offers still gets a message whenever the admission moves (`adm == before` guard), so live and `Sync` purge at the same event.
- **Pre-existing, not this sprint's:** int arithmetic, `-n` and `abs` wrap silently on overflow (`abs(MinInt64)` < 0). SEMANTICS and SPEC do not define overflow; worth a backlog question.

**Not tried (time box):**
- a live-vs-replay comparison built for this attack: I relied on QA's 200-seed equivalence and its teeth;
- output order around a drain;
- a recording with ticks then a late-but-admitted event, through both paths;
- `Purged` at a fixture's `End` with a saturating TTL (a due at MaxInt64 may never fire, which would break "every key purges" at End; counter only);
- many keys / map growth;
- a body panic through a plan body (none can panic on a checked program);
- a D3 derived `ts` declared `timestamp optional`, which is accepted and harmless as far as I read;
- a red test for L2.
