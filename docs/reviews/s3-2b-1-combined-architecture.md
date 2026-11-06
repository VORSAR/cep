# S3.2b-1 combined build: architecture review

> **Historical — S3.2b-1, the plan, replay through the runtime, rate limits; merged in PR #14.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-06--s32bs-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-2b-1-design.md) · [record](../sprints/s3-2b-1-record.md) · reviews: [design: architecture](s3-2b-1-design-architecture.md), **combined build: architecture**, [break-it: QA](s3-2b-1-breakit-qa.md). **← [S3.2a-2](../sprints/s3-2a-2-record.md) · [S3.2b-2 (next)](../sprints/sprint-plan.md#status) →**

**What was reviewed:** the combined build on `s3-2b-1/integration` at 8214be8 (`git diff a5dbc2d..8214be8`, 29 commits), checked against `docs/sprints/s3-2b-1-design.md` v2. Its §7 MUST/MUST NOT list is the contract. This is a blind review: the QA break-it branch and its worktree were not read.

**Sources read:**
- The design v2, §0–§12.
- The first review, `docs/reviews/s3-2b-1-design-architecture.md`.
- `docs/sprints/sprint-plan.md`: §5 "S3.2b's plan check" and "S3.2b-1's design", the S3.2b card (line 315), and the status line.
- `docs/sprints/s3-2b-1-record.md`.
- Code at 8214be8:
  - `internal/plan/plan.go`
  - `internal/runtime/{runtime,sync,ingest}.go`
  - `internal/partition/partition.go`
  - `internal/playback/{driver,playback}.go`
  - `internal/expr/{expr,registry,nodes}.go`
  - `internal/validate/{exprs,scope,validate}.go`
  - `internal/clock/{clock,rule}.go`
  - `tools/importcheck/rules.json`
  - the test fixtures for E-TYPE-001 and E-NULL-003
- The doc diffs:
  - SEMANTICS §8 and §9
  - SPEC §5.1, the code count and change log rev 4
  - errors.md
  - ARCHITECTURE §3 and §5
  - ADR-0005 §3
  - ADR-0006 §2

**Scratch checks.** All ran on pinned Go 1.27.3 with `GOTOOLCHAIN=local CGO_ENABLED=0`, in a detached worktree at 8214be8 that is now removed. Copies are in `/private/tmp/claude-501/-Users-ramith-code-cep/b4697049-d09e-4976-8d26-f50c312c9baa/scratchpad/s3-2b-1/arch2-scratch/`:
- `zz_arch2_alloc_test.go`: MUST 10 with a keyed rule.
- `mutations.txt`: probes M1–M7, each run against the real tests with the file restored afterwards.

**Baseline.** One run of `go test -count=1` over plan, runtime, playback, partition, validate, expr and clock: every package `ok`.

## Summary

| ID | Severity | Where | Title |
|---|---|---|---|
| C1 | should-fix | §7 MUST 3; `runtime.go:986`, `:991–998`, `:770` | MUST 3 is only half pinned. If on-arrival rules stop taking the drain's and the end's advances (M1), or a shard with no offer stops getting the admission (M4), every test still passes |
| C2 | should-fix | owner D3; `validate/exprs.go:301–304` | D3's E-TYPE-001 is reported at the stream's declaration, but the recorded decision says "at the attribute". A code comment credits that position to D3, and neither the design nor the record notes the change |
| C3 | should-fix | ARCHITECTURE §3 rule 4 (l.147); sprint-plan l.315 (S3.2b card item 6); record l.16 | Three docs still say rate anchors live in `partition.Instance`, with no annotation. Only ARCHITECTURE §5 step 7 was annotated |
| C4 | later | `sync.go:71–124` vs `runtime.go:676–717`; design §4 | `Sync` copies the router's tick, table-write and drain glue instead of sharing a compute/deliver split, so "only the delivery differs" holds by inspection, not by construction. S3.2b-2's router work could drift past it |
| C5 | later | `sync.go:129–130`, `driver.go:125–127`; ADR-0006 §2(b) | Neither the replay's `Walk` nor the driver's `Walk` reaches the rules' state (instances, rate anchors, QA G6's `("anchor", "", At)`), although the driver now owns rules |
| C6 | later | `partition.go:171–172`; ground rule 9 | The one-input check for on-arrival rules was not red first. M5 shows its test has teeth |
| C7 | later | `playback/plandriver_test.go:315–328` | MUST 10's test uses example 09, which is key-less, but its comment says "one keyed rule". The keyed routing path holds 0 allocations today (proven), but no test pins it |
| C8 | later | `expr/registry.go:21–31` | The default `NullRule` is the permissive one. When S8 adds `sum`/`avg`/`min`/`max`, a descriptor that leaves `Null` unset will understate nullability, and E-NULL-003 will miss cases (inferred) |
| C9 | later | `partition.go:27–45`, `plan.go:273–370`; design §2 | `NoTimers`/`TimerFree` are acceptable, but `TimerFree` fails silently. "Patterns and windows call the emission stage later" cannot work as written, because they cannot import `plan`; `plan` would have to wrap them |

**Counts:** 0 blockers, 3 should-fix, 6 later.

**What holds** (checked; there is no finding):
- **MUST 1: imports.** `plan` imports neither `runtime` nor `playback`. The aliases are in place: `runtime.Rule`/`Emit` and `playback.App`/`Stream`/`Table`.
- **MUST 2 and the D1 arithmetic:**
  - A stream-late event delivers nothing (`runtime.go:745–748`).
  - Admission is monotone, and `ApplyAdvance` takes a max (`clock/rule.go:93`).
  - An accepted event's own advance leaves admission at or below its time, so a purge in the first walk precedes only an event the rate limit would emit anyway.
- **MUST 5: the first walk.** It runs only for on-arrival rules that received an offer. M6 (a first walk for buffered rules too) changes no test, which fits the design's "for buffered rules nothing changes".
- **MUST 7, 9 and 11:** each is held by the code and has a test.
- **MUST 8: the stamp clamp.** It is in `Ingest`, and M7 is caught.
- **F4: routing views.**
  - The live `send` copies `adv` and `offers` (`runtime.go:599–605`). `Sync` processes them in place.
  - An output's `Values` is allocated fresh for each emission, and `flush` clears the outbox. So `out` never sees a reused slice.
  - Escape analysis shows that `&it.ev` in `router.ingest` is not moved to the heap.
- **MUST 10 with a keyed rule:** 0 allocations per non-matching event for 8-, 33- and 36-byte string keys through `Sync`.
- **`plan`:**
  - Each body compiles its own trees: `NewBody` calls `spec.compile()` (`plan.go:168`).
  - The select binding matches the validator's: by `as`, or by the attribute's or raw's name. `ts` is refused by E-SEL-001 and skipped by `plan`.
  - Widening keeps null.
  - A key-less rule gets the pseudo-key, a TTL of 0, and one anchor.
- **`mayBeNull` matches what `expr`'s nodes actually do:**
  - compare, `in`/`not in`, `not`, `and`/`or` and `is [not] null` are two-valued in code (`nodes.go:53–160`, `expr.go:322–336`).
  - arith, `minimum`/`maximum`, `abs`, `round` and `cast` pass null through.
  - `coalesce` is null only when every argument is.
  - `ifThenElse` with a null condition is an error, not null.
  - `cast`'s target is a string literal.
  - Pattern elements are steps, pairs and absences, so a selectable capture is always bound. E-NULL-003 therefore has no capture false negative for S4.1.
- **The reader** refuses records after the stop (`reader.go:265`), so `Sync.Event`'s drained branch never reaches a replay.
- **Docs:** SEMANTICS §8/§9, SPEC §5.1 and rev 4, errors.md, ADR-0005 §3 and ADR-0006 §2 match what was built.

## C1 — MUST 3 is half pinned (should-fix)

**Claim:** MUST 3 says an on-arrival rule's clock advances on its stream's admission, *the drain and the end*. Two parts of that wiring can be removed without any test failing:
- **(a)** `m.final`, which lets the drain's and the end's advances reach on-arrival rules.
- **(b)** The router's extra message to a shard when only the admission moved, `adm == before` in the skip test.

With (a) removed, plain-rule keys never purge at a fixture's end or at a stop. That contradicts `Sync.End`'s own doc ("every key purges"), and `Purged` is wrong in both live and replay, so the equivalence test cannot notice. With (b) removed, live with N > 1 shards and an idle floor above admission delays purges on shards that got no offer. Outputs are unchanged, because the next message carries the absolute admission, but `Purged` at a mid-stream stats cut and the resident state differ.

**Evidence:**
- `runtime.go:986` (`if !arrival[ri] || m.final`).
- `runtime.go:770` (`&& adm == before`).
- Mutations, from `mutations.txt`:
  - M1, `|| m.final` removed: runtime, playback, partition and plan all `ok`.
  - M4, the admission-only message dropped: runtime and playback `ok`.
- For contrast, M2 (the admission feed removed) is caught by `TestSyncWalksBeforeOffers` and `TestShardStepWalksBeforeOffers`. The playback D1 test does not catch M2: a plain body's purge is invisible in its outputs by D1's own argument, so the feed is observable only through `Purged` and the state.

**Fix:** two tests, each red with its mutation.
1. Through `Sync` and live: a keyed plain rule.
   - After `Drain`, a key with `LastActivity + TTL <` the drain level is purged (`Purged == 1`).
   - In a fixture, after `End` every key is purged (`Purged == keys`; no instance left in the rule).
2. Live, 2 shards, `TickEvery = 0`, `idle after` on the stream:
   - Tick past the idle bound so the floor rises above admission.
   - Send an event for key A on shard 0 that moves admission past key B's TTL, where B is on shard 1.
   - A `Stats` cut then shows B purged.

## C2 — D3's position differs from the recorded decision (should-fix)

**Claim:** sprint-plan §5 D3 records "E-TYPE-001 **at the attribute**", and design §1 says the same. The build reports it at the derived stream's declaration, because `model.Attr` has no position. That alone is a defensible engineering limit. Three things make it a finding:
- the code comment says "at its declaration (owner D3 of S3.2b-1)", crediting the owner with a position D3 does not state;
- neither the design nor the record notes the deviation;
- the invalid fixture pins `@ 7:1`.

Under ground rule 4, a recorded decision stands until the owner changes it.

**Evidence:**
- `validate/exprs.go:301–304` reports at `target.Pos`.
- `model/model.go:52–56`: `Attr` has `Name`, `Type`, `Optional` and no `Pos`.
- `validate/testdata/e-type-001-derived-ts-not-timestamp.cep`: `-- expect: E-TYPE-001 ... @ 7:1`.
- sprint-plan.md:966.

**Fix (recommended):** the owner annotates D3 to say "at the stream's declaration, naming `<Stream>.ts`", and the comment's attribution is corrected.

**Alternative:** add a `Pos` to `model.Attr`. That changes the model and the parser's golden JSON, a format change that needs the owner's consent. It costs about 1 point for a message that already names the attribute.

See owner question 1.

## C3 — Stale anchor placement in three docs (should-fix)

**Claim:** design v2 and the build keep the rate anchor in the body's `OpState` (`plan.go:273–293`). Three places still say `partition.Instance` with no annotation:
- ARCHITECTURE §3's import rule 4: "anchors live in `partition.Instance`";
- the S3.2b card's item 6 in sprint-plan;
- the record's item 6.

Only ARCHITECTURE §5 step 7 was annotated. A later agent reading §3 or the card would put S4.1's anchor back on `Instance`, which is the drift review F5 found.

**Evidence:**
- ARCHITECTURE.md:147.
- sprint-plan.md:315: "anchors in `partition.Instance`".
- s3-2b-1-record.md:16.
- ARCHITECTURE.md:229, which is annotated.

**Fix:** annotate each, without rewriting: "(annotated 2026-11-06, S3.2b-1 design v2, review F5: the anchor lives in the body's `OpState`, ADR-0006 §2)". The record's line can carry the same parenthesis.

## C4 — `Sync` copies the router's tick, write and drain glue (later)

**Claim:** design §4 says "Ticks, table writes and drains get the same compute/deliver split". Today:
- `Sync.Event` shares `router.event`, as the design asks;
- `Sync.Tick`, `Drain` and `End` call `ing.X` and `rt.broadcast` themselves;
- `Sync.TableWrite` re-implements upsert and delete, adding a table-id bounds check the live path lacks.

The two paths are equivalent today, by inspection. But S3.2b-2 changes the router's ingest, with stream-level event errors (SEMANTICS §10) and the live hand-off's allocation budget. Any of that work placed in `router.ingest` rather than in `Ingest` or `router.event` skips `Sync` silently, and only the equivalence test's seeds would catch it.

**Evidence:**
- `sync.go:71–124`.
- `runtime.go:676–717`.
- The `Sync` doc comment, "only the delivery differs" (`sync.go:12–14`).

**Fix:** factor `rt.tick(a, stamp)`, `rt.write(a, table, op, rows)` and `rt.drain(a, token)` so both paths call them. Or, at minimum, add a review rule for S3.2b-2: every change to `router.ingest` names its `Sync` counterpart, or says why there is none.

## C5 — The replay's `Walk` misses the rules (later)

**Claim:** the driver now owns the rules, but `Driver.Walk` and `Sync.Walk` yield only the ingest step's Seq counters. ADR-0006 §2(b)'s holders include each `partition.Rule`, meaning its clock and then each instance, and therefore the anchor that QA G6 made walkable. Nothing in the replay can reach them. The doc is accurate about what `Walk` yields, but the driver's "a state holder (ADR-0006 §2(b))" now under-describes what it holds. This is the coordinator's known deviation; I judge it **later**, because the live runtime has no `Walk` either.

**Evidence:**
- `sync.go:129–130`.
- `driver.go:125–127`.
- `partition.go:366–375`: `Rule.Walk` exists and yields `instance` state.

**Fix:** `Sync.Walk` also walks each rule on its shard under a `rule/<name>` component, which takes a few lines. Or forward it to the state-registry and snapshot work by name.

## C6 — The one-input check was not red first (later)

**Claim:** the coordinator's known deviation. The guard at `partition.go:171–172` came in the feature commit together with its test (b48b790). Ground rule 9 asks for a test that failed first.

**Evidence:** M5, the guard disabled, fails `TestOnArrivalReadsOneStream`, so the test has teeth. `plan` always builds exactly one input (`plan.go:156`), so no program reaches the guard today.

**Fix:** record it in the record's Findings table as a process deviation, with M5 as the proof. No code change.

## C7 — MUST 10's test is key-less, though its comment says keyed (later)

**Claim:** `TestSyncNoMatchAllocatesNothing` builds example 09, which is unpartitioned (`09-params-and-quoting.cep`: no `partition`, `output first every ${lookback}`), while its comment says "one keyed rule (09's shape)". So the router's keyed path is not pinned: `shardOf` hashing a string key with `h.Write([]byte(key.Str))`. It holds 0 allocations today only because the compiler devirtualizes `fnv` and keeps the conversion on the stack. A toolchain bump or a change to `shardOf` could add an allocation for keys over 32 bytes, such as UUID tenant ids, and nothing would fail.

**Evidence:** `zz_arch2_alloc_test.go`, a keyed `partition by k of S` rule, a non-matching event, through `runtime.Sync`:
```
key len 8: 0 allocs per non-matching event, 0 outputs
key len 36: 0 allocs per non-matching event, 0 outputs
key len 33: 0 allocs per non-matching event, 0 outputs
```

**Fix:** add the keyed case to the test, using 07's rule or this one with a 36-byte key, and correct the comment.

## C8 — The default null rule is the permissive one (later, inferred)

**Claim:** `NullRule`'s zero value is `NullIfAny`: "null only when an argument can be". That is right for the scalars. For an aggregate over a group that can be empty it is wrong: `avg`, `min` or `max` of an empty bucket is null whatever its argument. Only `count` exists today, and it sets `NullNever` explicitly. When S8 adds the reserved aggregates, a descriptor that leaves `Null` unset will make E-NULL-003 miss a null bound to a non-optional attribute. That is the run-time hole D4 closed.

**Evidence:** `expr/registry.go:21–31` (`NullIfAny NullRule = iota // ... the zero value is the default`) and `:107–115`. SPEC: aggregates beyond `count` are reserved. No current program is affected.

**Fix:** forward to S8 a registry test that every `Aggregate`-kind descriptor sets `Null` deliberately. Or make the zero value "may be null" and mark the scalars `NullIfAny` explicitly.

## C9 — `NoTimers`/`TimerFree`, and the emission stage's path to patterns and windows (later)

**Claim:** the embeddables are an acceptable Go pattern, like default no-op methods. They keep the import law intact: `plan` may not import `clock`, so it cannot schedule timers. The `CheckTimers` invariant backs this up, because a timer that walked state does not imply is caught. Two weaknesses:
- **`TimerFree.OnTimers` fails silently.** A body that embeds it and later gains a timer would have that timer fire into a no-op.
- **Design §2 overstates how patterns and windows will reach the emission stage.** It says "patterns and windows call it later with their own state's anchor". The stage is an unexported method on `plan`'s plain body, and `pattern`/`window` cannot import `plan`: `plan` imports them, so it would be a cycle. What does work is `plan` decorating their `Body` and `OpState`, keeping the anchor in its wrapper state and intercepting `emit` during each call.

**Evidence:**
- `partition.go:27–45`.
- `plan.go:273–370` (`body.out`).
- `tools/importcheck/rules.json`: `internal/pattern` and `internal/window` list no `internal/plan`.

**Fix:** have `TimerFree.OnTimers` panic ("a TimerFree body got a timer"). Partition turns that into a key reset and `Panics++`, so the failure is loud. Forward to S4.1's design: the emission stage reaches pattern bodies by `plan` wrapping them, not by them calling it.

## Pre-mortem

It is S3.3, and a replay of a production recording disagrees with live. The likeliest causes, in order:
1. **C4.** S3.2b-2 put stream-level event-error accounting or the `time arrival` stamp into `router.ingest`, and `Sync` never ran it. Live drops and counts an event that replay processes, or the reverse.
2. **C1.** A refactor of `shard.process` dropped `m.final` for on-arrival rules. Plain-rule state then grows without bound until restart, and `Purged` reads 0 in both live and replay, so the comparison is silent.
3. **C8.** S8's `avg` emitted null into a non-optional sink field that E-NULL-003 had approved.
4. **C3.** S4.1 put the pattern's rate anchor on `partition.Instance` because ARCHITECTURE §3 said so. A purge then left it behind, which is review F5 again.

The D1 arithmetic, the routing views and the per-body expression trees all held under every probe.

**Not covered in the time box:**
- the 200-seed equivalence generator's coverage: whether it actually produces stream-late events, null keys and division errors on every N;
- MUST 8's `Now`-count test, which I did not read;
- the parser goldens and grammar differential for 07 and 09;
- `go vet`, `-race` and the gate's `make check`, which are not mine to run.

## Owner questions

1. **Where the "ts must be a timestamp" error points (C2).** When an output stream declares `ts` as text or a number, the engine now reports the error at the line where the stream is declared, and the message names `Stream.ts`. Your decision said it should point at the `ts` field itself. Pointing at the field needs a small change to the program model and its saved test outputs.
   - **Recommended:** keep the stream's line and annotate your decision. The message already names the field, and it costs nothing.
   - **Alternative:** point at the field, at about 1 point and a format change.
