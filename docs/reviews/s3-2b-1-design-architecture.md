# S3.2b-1 design note v1: architecture review

> **Historical — S3.2b-1, the plan, replay through the runtime, rate limits; merged in PR #14.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-06--s32bs-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-2b-1-design.md) · [record](../sprints/s3-2b-1-record.md) · reviews: **design: architecture**, [combined build: architecture](s3-2b-1-combined-architecture.md), [break-it: QA](s3-2b-1-breakit-qa.md). **← [S3.2a-2](../sprints/s3-2a-2-record.md) · [S3.2b-2](../sprints/s3-2b-2-record.md) →**

Artifact: `docs/sprints/s3-2b-1-design.md` (v1), branch `s3-2b-1/integration` at 15d037c. This is a blind review: the QA branch was not read.

**Sources read:**
- The note.
- `docs/sprints/sprint-plan.md`: the S3.2b card, its "how it runs" note and readings; §5 "S3.2b's plan check" (Q1–Q7), "S3.2a-1's design" (D1–D7) and "S3.2a-2's design" (F5, F10, F14).
- SEMANTICS §2, §2.1, §2.2, §3, §4, §8, §9, §10, §11; SPEC §5, §6.
- ADR-0005 (Decision 1, 3), ADR-0006 §2 and §4 with their annotations, ADR-0010 §2 (annotation); ARCHITECTURE §5 and §6.
- Code at 15d037c:
  - `internal/runtime/{runtime,ingest,sync}.go`
  - `internal/partition/partition.go`
  - `internal/clock/{clock,rule}.go`
  - `internal/plan/plan.go`
  - `internal/playback/{driver,playback,plan,reader,encode,trace}.go`, plus the driver tests where they pin Step fields
  - `internal/expr/expr.go`
  - `internal/validate/{validate,exprs,resolve,bounds}.go`
  - `internal/model/model.go`
  - `tools/importcheck/rules.json`
  - examples 05, 07 and 09
- `docs/sprints/s3-2a-1-design.md` §3, where per-key marks are placed.

**Scratch checks.** All ran on pinned Go 1.27.3 with `GOTOOLCHAIN=local CGO_ENABLED=0`, in a detached worktree at 15d037c, which is now removed. Copies of every program are in `/private/tmp/claude-501/-Users-ramith-code-cep/b4697049-d09e-4976-8d26-f50c312c9baa/scratchpad/s3-2b-1/arch-scratch/`:
- `zz_arch_later_test.go`, run in `internal/expr`: F1. Also confirms §0.7 and MUST 10: a pointer row costs 0 allocations, a struct value 1.
- `zz_arch_o1_test.go`, `zz_arch_o1c_test.go`, `zz_arch_o1b_test.go`, run in `internal/runtime` on the real `Ingest` and `RuleClock`: F2. They simulate §3's on-arrival step, the design's O1 option 2, and the fix.
- `zz_arch_alloc_internal_test.go`, run in `internal/runtime`: F4.
- `zz_arch_order_test.go`, run in `internal/partition`: F10.
- `zz_arch_build_inputs_test.go`, run in `internal/validate`: F9.

## Summary

| ID | Severity | Where | Title |
|---|---|---|---|
| F1 | blocker | §1, §7 MUST 11 | `Build` accepts a plain rule whose filter holds a node that cannot run yet (text operators until S7). Every event becomes an expression error, so the rule never fires, and an empty-`then` expect block passes on nothing |
| F2 | blocker | §3 ("why a purge never changes an alert"), §11 O1 | O1 is wider than the note says. In any fixture with two or more source streams, the virtual floor puts the rule clock past an accepted event's TTL, so the key is purged at once and a second event inside D alerts again. O1's option 2 does not close it |
| F3 | should-fix | §4 (`Sync.Event`), `sync.go:25` | `Sync.Event` takes the event by value, so the driver loses `Step.Event.Seq` and `Tables`. "The format layer's tests stay unchanged" is false |
| F4 | should-fix | §4 (the refactor), §8.8; owner Q4 | Sharing `router.event` as it is costs 1 to 2 allocations per event on the replay path (measured), which S3.2b-2's exit 8 forbids |
| F5 | should-fix | §2, §3, §7 MUST 4, §9 | The anchor's home reverses ADR-0006 §2's 2026-11-05 annotation (rate anchors live in the body's `OpState`) without saying so |
| F6 | should-fix | §1, §4 (`FromPlan` ×2) | Three descriptions of one app: `plan.App` and `playback.App` match field for field, and `plan.Rule` and `runtime.Rule` match except for `Emit`. They will drift in S3.2b-2 |
| F7 | should-fix | §5 step 2, §7 MUST 8 | MUST 8 is false as written: the ticker reads `Config.Now`, and a drained app's events do not. Sync also skips the router's monotone stamp |
| F8 | should-fix | §5, §8.7 | The equivalence test cannot tell which rule or key an output belongs to, and its teeth plant only in shared code, so a broken seam stays green |
| F9 | should-fix | §1, §2 step 3, §11 O3 | The validator accepts plain-rule inputs `Build` leaves unspecified: a nullable value into a non-optional field, `ts` of another type (the encoder then refuses the output), a derived schema's time attribute, `time arrival` |
| F10 | later | §3 (step order) | The extra walk does not change outputs or counters, but it does change the state view: a key purged in walk 1 comes back as a new instance. Every message also walks every rule twice |
| F11 | later | §2, §3, §7, §8 | Unspecified edges and missing test rows |
| F12 | later | §10 | 7 points is optimistic by about 1 (inferred) |

**Counts:** 2 blockers, 7 should-fix, 3 later.

**O1–O3, answered:**
- O1: wider than stated, and option 2 does not fix it (F2). The proven fix: purge an on-arrival rule's keys on its stream's admission watermark. This goes to the owner.
- O2: an owner question (below). Partitioning 07 loses no corpus coverage, because 05 and 09 already carry the unpartitioned rate limit. It would also give the corpus its only partitioned plain rule (F11).
- O3: `Build` must refuse it with a plain error naming the stream (F9). A validator code is a language change for the owner.

---

### F1 — blocker — `Build` builds a rule that cannot run, and it fails silently

**Claim.** §1 step 3 builds every plain body, and MUST 11 refuses only pattern and window bodies. `expr.Build` succeeds on text operators (S7), `count` (S8) and `chain` (S4.1): it returns a `later` node whose `Eval` always returns a `RuntimeError`.

Under §2 step 2, each such error counts `Errored` and drops the event. So a plain rule using a text operator deploys, counts every event as errored, and never alerts. Example 05's `SuspiciousPromptText` is such a rule, and the validator accepts it.

The `or` in its filter does not save it. An event whose `text_invisibles == true` branch is true still errors, because the left operand errors first.

The damage goes past the rule. S3.3's conformance runner relies on "not built" to record `skip: not-implemented(S7)`. Without that signal, an expect block with an empty `then` (the defeat path) passes vacuously: a green gate for a construct that does not exist yet.

**Evidence.**
- `internal/expr/expr.go:204` (`&later{… "text operators (S7)"}`), `:259` (chain), `registry.go:174` (count), `expr.go:281–283` (`later.Eval` always errors).
- Example 05 lines 13–24.
- Scratch `TestArchLaterNodeInPlainFilter`, with filter `text has_ci 'override' or flag == true`: "Build ok, type bool; Eval = {… Null:true}, err = text operators (S7) is not executable yet". The result is the same for an event whose flag is true.

**Fix.**
- MUST 11 becomes: `Build` MUST return `ErrNotBuilt` for a pattern or window body, and for any rule whose filter or select contains a node this release cannot evaluate. That means text operators until S7, `count` until S8 and `chain` until S4.1.
- `expr` exposes the check, for example `expr.Runnable(Expr) bool`, or `Build` returning a sentinel. `plan` must not keep its own list of operator kinds, because that list would drift.
- Add §8.1 cases: 05 → `ErrNotBuilt`, and 02 (a cron window) → `ErrNotBuilt`.

### F2 — blocker — "A purge never changes an alert" fails in fixtures; O1 is not only about idleness

**Claim.** §3 argues that any event accepted after a purge has `t ≥` its stream's admission watermark, "which, without idleness, is at least the clock". That holds only in live mode without idleness.

A single-input rule's clock is its stream's contribution:

`max(admission, idle floor, virtual floor, punctuation, drain)`

In a fixture, the virtual floor is `virtual_now − d_s`, and `virtual_now` is the largest event time on any stream (SEMANTICS §2.1; `ObserveVirtual`). When another stream runs ahead, stream S's contribution sits above its admission watermark. S then accepts events behind the rule clock, because stream lateness is judged on admission alone. The on-arrival path gives them to the body by design.

What follows for such an event:
1. Its key is born, or re-armed, with a TTL due of `ts + TTL`, which is already behind the clock.
2. The second walk purges the key and clears the anchor.
3. The next event of that key inside D alerts again.

The written rule (SEMANTICS §8: `t ≤ t_out + D` is suppressed) gives one alert. The engine gives two.

The owner's Q1 record states "the deploy check (TTL ≥ every rate-limit interval) means a purge never changes an alert". The note's argument does not hold in replay, which is the product's main path (ADR-0005: "replayed single-threaded through the real runtime with the virtual clock"). Live idleness, the note's O1, is the second case.

O1's option 2 ("due-date … from `max(LastActivity, clock)`") narrows the gap but does not close it. When the other stream keeps moving, the clock passes the new due date, and the next event inside D alerts again. It also breaks `CheckTimers`, which derives the TTL timer as `LastActivity + TTL` (`partition.go:345`), unless `LastActivity` is redefined.

**Evidence.**
- `internal/clock/clock.go:83–95` (`raise`: the contribution is the maximum of the floors), `:100–111` (`ObserveEvent` judges against `adm` only), `:131–143` (`ObserveVirtual`: every stream).
- `internal/runtime/ingest.go:49–52`: a fixture raises the virtual floor after every event.
- Scratch, real `runtime.Ingest` (fixture) and `clock.RuleClock`. Stream S is bounded at 60 min and stream T at 0. One plain rule on S is keyed by k, with TTL 10 min and `first every 5 min`. The log is T@10:00, then S{a}@08:30, then S{a}@08:32.
  - `TestArchO1VirtualFloor` follows §3: "ts 510 min on S: accepted; rule clock 540 min; TTL due 520 min", and "alerts at [510 512]; purges 2". That is two alerts where the written rule gives one.
  - `TestArchO1DueFromClock` is O1 option 2, with T@10:20 then S{a}@08:33 added: "alerts at [510 513]; purges 1". It still duplicates.
  - `TestArchO1AdmissionClock` is the fix below: 08:32 and 08:33 suppressed, then a fresh alert at S{a}@10:00 once S's own admission passes 08:33 + 10 min. Result "alerts at [510 600]", which is the written rule's answer. Its final assertion still says "want 1" from the shorter scenario; the printed result is the evidence.

**Fix (recommended; the owner decides, because Q1 says "the idle-key purge stays on the clock").** An on-arrival rule's TTL clock is its stream's **admission watermark**, never a floor. The exceptions are the drain level and +∞, after which nothing arrives.

Why this is exact:
- An accepted event has `t ≥ adm_before`.
- Its own advance gives `adm_after = max(adm_before, maxTs − d) ≤ t`.
- So a purge at `adm > LastActivity + TTL ≥ Anchor + D` can only precede events the rate limit would have emitted anyway.
- Admission is the same live and in replay (SEMANTICS §2.1), so equivalence holds.

The cost: a quiet stream keeps its keys until its next event or the stop. A plain rule's per-key state is one anchor, and a quiet stream brings no new keys.

Mechanics: the event message carries the stream's admission (from `Ingest`; the registry already holds it as `StreamState.Admission`), and the shard applies it to on-arrival rules instead of `m.adv`. Ticks never move it.

If the owner instead accepts duplicates, §3's argument and SEMANTICS §8 and §9 must say that a purge can re-open a suppression interval in replay of multi-stream logs and in live under idleness. They must not claim the guarantee.

### F3 — should-fix — `Sync.Event` drops the stamped event; the driver's contract breaks

**Claim.** §4 says the driver "keeps its record-to-step contract (`Driver.Step` returns the same `Step`)" and owns only a `Sync`. But `Sync.Event(e event.Event, arrival int64) (late bool, adv []clock.WMAdvance)` takes `e` by value. The ingest step stamps `Seq` and `Tables` on Sync's copy (`Ingest.Event(&e, …)`), and nothing returns them.

So `Step.Event.Seq` and `Step.Event.Tables`, which the driver sets today and the format layer's tests pin, can only be recovered by a second Seq counter in the driver. That is a second ingest, which MUST 7 forbids.

**Evidence.**
- `internal/runtime/sync.go:25`; `internal/runtime/ingest.go:37–40` (stamps through the pointer).
- `internal/playback/driver.go:85–95` today returns the stamped `e`.
- Tests that pin it: `driver_test.go:91–96` (Seq, and `Event.Tables` is the step's snapshot), `:142`, `:228`; `breakit2_test.go:232`; `exits_test.go:232`; `ingest_test.go:95`.

**Fix.** `Event(e *event.Event, arrival int64) (late bool, adv []clock.WMAdvance)`, mirroring `Ingest.Event`, or return the stamped event. Add to §7: "Sync's event step returns the event as ingest stamped it (Seq, Tables)".

### F4 — should-fix — The shared router step allocates on the replay path

**Claim.** §4 makes `routeEvent` shared, with `deliver` choosing between the live FIFO and Sync's shard. Today the step builds each message with fresh copies, which live needs because a message crosses goroutines:
- `adv: append([]clock.WMAdvance(nil), adv...)`
- `m.offers = append([]offer(nil), rt.offers[sid]...)`

Shared as it is, every event on Sync costs 1 allocation, and 2 when the watermark moves.

Owner Q4 requires zero allocations for every per-event stage "(the ingest step, the key, the clock, the filter, the rate limit), measured on the one-thread replay path". That is S3.2b-2's exit 8. §8.8 measures "filter evaluation" only, so it can pass while the path it claims to prepare is not alloc-free.

**Evidence.**
- `internal/runtime/runtime.go:744–746`.
- Scratch `TestArchRouterEventAllocs`: one shard, one keyed route, the real `router.event`, with the FIFO reset between runs. Results: "watermark still: 1.0 allocs/event", "watermark moving: 2.0 allocs/event", "shardOf alone: 0.0 allocs".

**Fix.**
- `routeEvent` fills per-shard buffers it reuses and calls `deliver(shard, view)` with views valid only during the call.
- The live `deliver` copies, as it must. Sync's `deliver` processes in place.
- Sync's outbox flush reuses `outbox[:0]`.
- §8.8 measures a whole `Sync.Event` on a non-matching event of a resident key (0 allocations), not the filter alone.
- Add a MUST: "the replay path's event step MUST NOT allocate for a non-matching event of a resident key".

### F5 — should-fix — The anchor's placement reverses a recorded annotation silently

**Claim.**
- ADR-0006 §2, annotated 2026-11-05 from S3.2a-1's design v2 §3 (review F9): "Per-key marks — the `once` mark, **rate anchors**, the batch-id dedup set — live in the body's `OpState`, so a purge clears them and `Walk` yields them; `Instance` keeps `Key`, `Clock`, `Op`, `LastActivity`."
- S3.2a-1's design v2 says the same: "rate anchors (S3.2b) … ADR-0006 §2's `Instance` fields `RateAnchors` and `OnceDone` move there".

This note puts `Anchor` and `Anchored` back on `partition.Instance`. It cites the older card text and ARCHITECTURE §5 step 7, and plans a §9 annotation of ADR-0006 §2 ("the anchor on `Instance`") without saying it reverses the later one. Ground rule 4 says to annotate, never rewrite.

The engineering trade-off between the two placements:
- **`OpState`:** a reset clears the anchor for free (`Op = nil`), so MUST 4 disappears, and `partition` stays free of a plan concept.
- **`Instance`:** one generic slot for the emission stage.

**Evidence.**
- `docs/architecture/adr/ADR-0006-partitioning-state-concurrency.md:26` (the annotation); `docs/sprints/s3-2a-1-design.md:109`.
- `internal/partition/partition.go:49–50` (stub fields); note §3 and §9.

**Fix.** Either follow the recorded placement (the filter body's `OpState` holds the anchor, and the emission stage takes a `*Anchor`), or state the reversal and its reason, and bring it to the owner (owner question 2). The recommendation is the recorded placement.

### F6 — should-fix — One app, three descriptions, two `FromPlan`s

**Claim.**
- `plan.Stream {Schema, Delay, IdleAfter, Derived}` and `plan.Table {Name, Attrs, Key}` are field-for-field `playback.Stream` and `playback.Table`.
- `plan.Rule` is `runtime.Rule` except that `NewBody` takes `func(event.Event)` where runtime takes the named type `Emit`. The two func types are therefore not assignable, and `runtime.FromPlan` must wrap every `NewBody`.

S3.2b-2 adds per-stream facts: the stream error policy (item 5) and arrival stamping (item 3). Each must be added in three places and copied twice, and a missed copy is silent.

`playback` may import `plan` (`rules.json`), and `runtime` may import `plan`.

**Evidence.**
- `internal/plan/plan.go:15–45`; `internal/playback/playback.go:14–33`; `internal/runtime/runtime.go:29–41`.
- `internal/runtime/sync.go:60`; `internal/playback/plan.go:9`.

**Fix.**
- `type Emit = func(event.Event)`, an alias, then `type Rule = plan.Rule` in runtime. The runtime tests' `func(emit runtime.Emit) partition.Body` literals still compile.
- `type App = plan.App`, `Stream = plan.Stream` and `Table = plan.Table` in playback, which drops `playback.FromPlan`.
- `runtime.App` keeps its `Specs`, `Schemas`, `Tables` and `Sinks`, made once by `runtime.FromPlan`.
- If the aliases are refused, add a test that walks both structs' fields by reflection and fails on a mismatch.
- This also answers the review question "one description or three?": one, if this fix is taken.

### F7 — should-fix — MUST 8 is false as written; Sync bypasses the router's stamp

**Claim.**
1. The internal ticker calls `cfg.Now()` on every tick. If the §5 live run ever sets `TickEvery`, the popping `Now` consumes arrivals out of order.
2. An event for a drained app returns before reading `Now`, so "once per event" is false after a drain.
3. `router.now` clamps every arrival and tick to a monotone stamp shared across apps. That clamp sits outside the step §4 shares, so a direct `Sync` caller (S3.3's runner, a test) feeding a non-monotone stamp diverges from live. Replay of a file is safe only because the reader refuses non-monotone stamps.
4. MUST 8 has no test that can fail.

**Evidence.**
- `internal/runtime/runtime.go:536` (ticker), `:712–716` (drained app, then the read), `:640–643` (the clamp).
- `internal/playback/reader.go:310, 389, 447` (the reader's monotone checks).
- Note §5 step 2, §7 MUST 8.

**Fix.**
- MUST 8 reads: "`Config.Now` is read once by `Start`, once per event the router takes for an undrained app (stream-late included), and by the internal ticker once per tick; nowhere else".
- §5 step 2 says the live run uses `TickEvery = 0`.
- Add a test that counts `Now` calls against events sent.
- Move the monotone clamp into the shared step (Ingest, or `routeEvent`'s caller-independent part) so Sync applies it too, or make `Sync.Event` and `Sync.Tick` refuse a stamp below the last one.

### F8 — should-fix — The live/replay test cannot attribute outputs, and its teeth miss the seams

**Claim.**
- **(a) Attribution.** Sinks receive bare `event.Event`s with no rule and no key. "Per rule and key, the output sequence is identical" is implementable only if each rule writes its own derived stream and projects its key. The note does not require either.
- **(b) The teeth.** A body whose output depends on its shard proves the comparator compares. But live and replay share the body, the router step and the shard step, so a planted fault there changes both sides alike. Real divergence can only come from the parts that are not shared: arrival and tick plumbing, the monotone stamp (F7), table-write position, Sync's `deliver`, `Begin` and `Drain`, and a forgotten walk 1 in Sync's deliver. No tooth targets them.
- **(c) Coverage.** The §10 paths, expression errors and null keys, are not generated, and `Errored` and `KeyErrors` are not compared.
- **(d) `time arrival`.** A `time arrival` stream must be excluded until S3.2b-2. The live router does not stamp it yet.

**Evidence.**
- `internal/runtime/runtime.go:800–803` (an output carries only `StreamID`), `:865–878` (`NewBody` is called once per shard, in shard order, so a test closure can number shards with no production hook).
- Note §5 steps 1, 4 and "Teeth".

**Fix.**
- §5 requires one derived stream per rule, with the key projected.
- Generate an expression that errors (division by an attribute that can be 0) and a nullable key, and compare `Errored` and `KeyErrors`.
- Teeth at the seams: Sync applying a table record one event late, Sync's deliver skipping walk 1, and live reading `Now` twice per event. Each must turn red.
- Build the shard-dependent body from the existing per-shard `NewBody` call order, not a new hook in engine code.

### F9 — should-fix — Inputs the validator accepts that `Build` leaves unspecified

**Claim.** Each of these programs passes `validate.Check` with zero findings, and §1 or §2 does not say what `Build` or the body does with it.
1. **A nullable value into a non-optional derived attribute** (`select x` with `x string optional` into `O.x string`). `assignable` ignores optionality. §2 step 3 says nothing about the null, and dropping it would drop an alert.
2. **O3: a derived `ts` that is not a timestamp.** `Build` writes a timestamp into a `string` slot. Live sinks receive a mistyped value, and the replay encoder refuses the output: "attribute "ts" holds a timestamp value; it is declared string".
3. **A derived stream's schema.** §1 step 1 gives `timeAttr` only for source streams. Under D2 (S3.2a-1), a derived output's line carries a top-level `ts`, so its schema MUST use `event.ArrivalTime`. Using the `ts` attribute's index would drop the top-level `ts`.
4. **A `time arrival` source** builds, but nothing stamps its time live until S3.2b-2.
5. **A derived stream without `ts`.** This is handled by §1's "if the target declares", which is fine; it is listed for completeness.

**Evidence.**
- Scratch `TestArchBuildInputs`: "nullable into non-optional findings: []", "derived ts not a timestamp (O3) findings: []", "partitioned plain rule findings: []", "time arrival source findings: []", "object projected findings: []", "derived without ts findings: []".
- `internal/validate/exprs.go:286–301` and `:302–304` (`assignable`).
- `internal/playback/encode.go:146–147` and `:166–172`.

**Fix.**
- §1 states that a derived stream's schema uses `ArrivalTime` (D2).
- `Build` returns a plain error, not `ErrNotBuilt`, for a derived `ts` that is not a `timestamp`. The validator code for it goes to the owner as a language change.
- §2 step 3 states the null case. The recommendation is to emit it, because an alert is never dropped for a field, and to forward a lint to the owner.
- §1 states that a `time arrival` source builds, its stamping is S3.2b-2's item 3, and the §5 test excludes it.

### F10 — later — The extra walk changes the state view, and costs a second pass over every rule

**Claim.**

§3 says "for buffered rules the extra walk changes nothing". For outputs and counters that is true under D1. For the state registry it is not: a key whose TTL falls due before an offer is now purged and freed in walk 1, and its event creates a new instance with a new ordinal. Before, the purge ran in the walk after the offer and the key kept its instance. No output or trace reads ordinals today. Phase 2's snapshot seam does.

Separately, every message now walks every rule of the app twice (`Shards × rules × 2` `Advance` calls per advancing event). Walk 1 is needed only for on-arrival rules that receive an offer in that message.

**Evidence.**
- `internal/partition/partition.go:277–295` (`free`), `:179–186` (a new `Instance` on a non-resident key).
- Scratch `TestArchStepOrderOrdinal`, with key a at 0 and TTL 10, then key a at 20 with the clock at 15:
  - old order: "released [0 20], purged 1, instance ordinals [0]"
  - new order: "released [0 20], purged 1, instance ordinals [1]"
- `internal/runtime/runtime.go:960–962`.

**Fix.** Restate §3 as "changes no output or counter; a key purged before an offer is reborn as a new instance". Run walk 1 only for the on-arrival rules offered in the message, or record the cost.

### F11 — later — Unspecified edges and missing test rows

- **One input.** `OnArrival` MUST require exactly one input (`validateApp` and `partition.NewRule`). §3's argument, and F2's fix, assume one.
- **New instances.** A new on-arrival instance MUST be in `byKey` and `byClock` before `OnEvent` runs, so a panic's reset and `free` find it (`partition.go:194–197, 258–295`).
- **`Born`.** A key-less on-arrival instance is never "born": with TTL 0 nothing calls `Schedule`, and so nothing calls `bear` (`clock/rule.go:321–327, 343–346`). `Walk` then yields `Born: false` for a live key with an anchor. Define `Born` for on-arrival keys, or call it out.
- **Anchor arithmetic.** `Anchor + D` should saturate. Durations have no upper bound in the validator (`bounds.go`); `partition.addSat` exists.
- **SEMANTICS §9 wording.** "An on-arrival rule's activity is its arrival" reads as the wall stamp. It should say "the largest event time seen, counted at arrival".
- **Tests:**
  - §8.5 has no case where a panic reset clears the anchor (MUST 4's second half).
  - §8.1 has no window body (02) → `ErrNotBuilt`, and no partitioned plain rule. The corpus has none, so add an inline program, or take owner question 3's option A.
  - §8.6's goroutine check should assert "no increase", not equality. Goroutines from earlier runtime tests may still be exiting; see S3.2a-2's review F17.
- **MUST 6 and MUST 7** cannot fail a test. Say they are review rules. For MUST 7, add a pointer-identity check: `Driver.Registry()` is the `Sync`'s.
- **Re-entrancy.** `out` MUST NOT call back into the `Sync`.

### F12 — later — Size

The 1.5 points for "replay through the runtime" cover two things:
- making the router's event, tick, write, drain and stats paths and `newShard` independent of `*Runtime`;
- the §5 generator and live harness (popping `Now`, four shard counts, 200 seeds, teeth).

With F1–F9, the total is about 8 points, still inside the lane. This estimate is inferred, not measured.

---

## Pre-mortem: three sprints from now, this design has failed. The two most likely reasons

1. **"One runtime" was narrower than it looked.** The router step and the shard step are shared. But arrival and tick plumbing, the monotone stamp, `Config.Now`'s coupling, table-write position and the per-shard message copies all sit outside them (F4, F7). The equivalence test's teeth were planted only in shared code (F8), so a seam drift stays green. It surfaces when a recorded incident replays to different alerts than fired live (S3.3, S5), or when exit 8 forces a late rewrite of the "shared" step.
2. **One key, two clocks.** The on-arrival path lets plain rules accept events behind the construct clock, while their purge stays on that clock (F2). Duplicate alerts appear in multi-stream replays and under live idleness. Hand-derived expected outputs disagree with the engine, and the fix lands as special cases once patterns and windows share the emission stage. Close behind: `Build` accepts anything the validator accepts (F1, F9), and a green expect block hides a rule that has never fired.

## Owner questions

1. **When should a plain rule forget a key?** (F2, O1)

   Example, with alerts at most once per 5 minutes per account. Another feed has already reached 10:00 in the replay when a grant for account A stamped 08:30 arrives: alert. The engine then forgets A at once, because "the clock" is already past 08:40. A second grant for A stamped 08:32 arrives: a second alert, though the rule says one per 5 minutes.

   Options, most accurate first:
   - **(A) Recommended.** Forget a key only when that rule's own feed has moved past it. Always one alert. Cost: a quiet feed keeps its keys a little longer, which is a small amount of memory.
   - **(B)** Start the forgetting time from the clock, as the note's option 2 does. Fewer duplicates, but some remain.
   - **(C)** Keep it as is, and document that an extra alert can appear in this corner.

2. **Where is a key's "last alert time" kept?** (F5)

   On 2026-11-05 it was recorded that a rule keeps such per-key marks in its own per-key memory. This design puts it on the shared key record instead. Both give the same alerts.
   - **Keep the 2026-11-05 placement (recommended).** Clearing a key, after expiry or a crash, clears the mark automatically.
   - **Accept the reversal.** It is simpler for the code that will serve patterns later, but the 2026-11-05 record must be annotated.

3. **Should example 07 limit alerts per tenant?** (O2)

   Today, tenant A's unknown-app grant at 10:00 silences tenant B's at 10:03.
   - **(A) Recommended.** Partition 07 by tenant, so each tenant gets its alert. No coverage is lost (05 and 09 still show the shared limit), and the corpus gains its only per-key plain rule.
   - **(B)** Keep one shared limit.

4. **Should two quiet mistakes in an output stream become deploy errors?** (F9, O3)
   - A derived stream's `ts` declared as something other than a timestamp: make it a deploy error. Recommended; today the engine would write a wrong-typed value that replay refuses to print.
   - A possibly-empty value placed in a field declared must-have: send the alert with the field empty, and add a warning at deploy. Recommended over dropping the alert.
