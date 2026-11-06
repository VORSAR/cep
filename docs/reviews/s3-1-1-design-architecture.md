# S3.1-1 design note: architecture review (G1)

> **Historical — S3.1-1, the clock service and the metrics counters; merged in PR #8.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-04--s22b-closed-s31s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-1-1-design.md) · [record](../sprints/s3-1-1-record.md) · reviews: **design: architecture**, [owner's decisions: architecture](s3-1-1-decisions-architecture.md). **← [S2.2b](../sprints/s2-2b-record.md) · [S3.1-2](../sprints/s3-1-2-record.md) →**

Reviewed: `docs/sprints/s3-1-1-design.md` (the note), `internal/clock/clock.go` and `internal/metrics/metrics.go` (the stub), at `8c51591`, against ADR-0004, ADR-0006, ADR-0013, ADR-0014, SEMANTICS §2.1/§6/§9 and the reference executor. The reviewer did not write the design.

**Method.** Each divergence claim below was run, not argued: a scratch probe imports `docs/language/checks/executor.py` unchanged and, for the ordering claims, a copy of `_PatternEngine.drain` changed to do the note's order ((a) every event, then (b) every timer, then (c) discards). The probes are P1–P7. Each finding quotes its probe's output.

**Summary.** The registry arithmetic in note §1 matches the executor on the paths the 2.1 checks cover: stream-late is judged against `adm` before the update, construct-late is strict, virtual floors are per stream, and the clamped min is right. The `Advance` contract in §2 does not match the executor. The executor's `drain` (executor.py:437–452) handles events and timers together in time order: before each event it fires the timers due strictly before it (`self._fire(ts, strict=True)`, :443). It does not release every event as one chunk before any timer. The executor also keeps one construct clock per rule (`wm_c`, :512), not one per key. Five blockers follow from these two facts. Three more come from the sentinel, strictness and TTL rules.

---

### F1 blocker: `Advance` releases every due event before any due timer, but the executor fires timers due before an event first; a TTL purge and the returning key then disagree

**Evidence.** Note §2 (s3-1-1-design.md:75) says "(a) every buffered event with `Ts ≤ clock` … as one `OnEvents` chunk; (b) every non-discard timer". The stub (clock.go:3) says the same. The executor does something else: `while self.buffer and self.buffer[0][0] <= wm: … self._fire(ts, strict=True); self._process(ev)` (executor.py:441–444). A TTL purge or completion due before event `e` fires before `e` is processed. SEMANTICS §6.4's three-step list (SEMANTICS.md:130) is written per watermark advance, and the executor is the binding arithmetic for it (ADR-0004 §4, "the conformance ports … are the binding arithmetic").

**P1** is check `9 · RT-07` (test_semantics.py:444) with `wm_delay = 6 min`, so the returning key's events and the purge fall in one drain. Executor: `[('a', 0), ('a', 660000)]`, two matches; the purge at 6:00 runs before A@11:00, so the `once` mark is gone. Note's order: `[('a', 0)]`, one match; A@11:00 is processed first, re-arms the TTL and keeps the `once` mark.

**Failure.** A missed detection. The purge never happens while the key stays active behind a lagging clock. The same order puts a sort window's purge retractions after the next arrival's change rows (RT-06), and leaves a cron bucket's next-bucket events ahead of its close.

**Fix.** Make `Advance` handle events and timers together, as the executor does:
1. Repeat: let `e` be the earliest buffered event with `e.Ts ≤ clock` (by `event.Compare`).
2. Fire, in heap order, every non-discard timer whose deadline is `< e.Ts`.
3. Release the run of events up to the next timer instant through `OnEvents`.
4. When no events are left, fire the timers that are due at `clock`.
5. Then run (c).

Events at an instant still come before timers at that instant (§6.4's same-instant rule). Add P1 as a timer-contract test that names its source check.

### F2 blocker: firing every kind at `Due ≤ clock` makes the `within` discard and the TTL purge one instant early; adding 1 to the due does not work once events and timers are handled together

**Evidence.** Note §2 (:75) fires every timer at `Due ≤ clock`, and (c) fires Discard at `Due ≤ clock`. SEMANTICS sets different strictness per construct:
- completion: `wm ≥ deadline` (§6.4)
- discard: `wm > t_first + D` (§6.3, SEMANTICS.md:116; executor.py:452 keeps a partial while `wm <= p.first_ts + within`)
- TTL: `wm − t_last > TTL` (§9; executor.py:368 `if pd < upto`)

**P5** (playback, S1 and S2 with `d = 0`): A@0 on S1, X@5:00 on S2, X@5:00 on S1, B@5:00 on S2. Executor: 1 match. The note with Discard scheduled at `t_first + within`: the clock reaches 5:00, (c) discards the partial, and B@5:00 is then admitted (`5:00 ≥ clock`) with nothing to extend. 0 matches: §6.3's worked example ("egress at exactly 10:05:00.000 → match") fails.

Scheduling at `deadline + 1` does not fix TTL once events and timers are handled together. For an event at `pd + 1`, the executor purges first (`pd < pd+1`, :443), but `Due = pd+1 < e.Ts` is false, so the event goes first.

**Failure.** Missed detections at the `within` boundary. The TTL purge happens one instant early, or never at the boundary.

**Fix.** The kind carries the strictness, and operators pass SEMANTICS' deadline unchanged:
- Discard and TTLPurge fire when `clock > deadline`. All other kinds fire when `clock ≥ deadline`.
- Before any event `e`, every kind fires when `deadline < e.Ts`. This is exactly the executor's `strict=True` and `strict=False` pair.

Write this in the stub's doc comment for `TimerKind`. Add a test for each kind at `deadline`, `deadline + 1` and an event at `deadline + 1`.

### F3 blocker: tie order "(due, kind, schedule order)" is not the executor's `(due, first_ts, seq)`; two pending completions at one instant fire in the wrong order

**Evidence.** Note §2 (:43, :60, :75). The executor sorts ripe completions by `(self._due(p), p.first_ts, p.seq)` (executor.py:363); seq is spawn order (SEMANTICS §6.5, RT-20). A pending completion is scheduled when its last positive matches, so schedule order is the order in which partials became pending, not the order in which they were spawned.

**P2**: A(x=1)@0 (spawned first), A(x=2)@0, B(x=2)@1:00 (the second partial becomes pending first), B(x=1)@2:00, `not C for 5 min`, so both are due at 5:00. The executor fires x=1, and its skip-past-last discards x=2. Schedule order fires x=2 and discards x=1.

**Failure.** A wrong detection, with the wrong evidence chain in the alert.

**Fix.** Order by `(deadline, kind, Ref, schedule seq)`. Write down that `Ref` is the operator's ordinal (a pattern passes the partial's spawn seq). Within one pattern, equal completion deadlines imply equal `first_ts` (the deadline is `first_ts + maxAbsenceBound`, ADR-0013 §3), so this reproduces executor.py:363. Add P2 to the timer-contract test.

### F4 blocker: ADR-0013 §3 completes a pure-positive pattern "through a timer due ≤ clock in the same Advance"; under any timer order this loses a later partial spawned at the same instant

**Evidence.** ADR-0013 §3 (ADR-0013:22) and note §2 (:75, "already due fires in the same Advance"). The executor completes a pure-positive match inside `_process`, at the completing event (executor.py:415–431). The skip-past-last therefore runs before the next event at the same instant is processed.

**P3** (`d = 5 min`, one drain): A@0, B@2:00, A@2:00 (later in §2.2 order), B@3:00. Executor: `[(0, 120000), (120000, 180000)]`, two matches. If the completion is a timer due 2:00, A@2:00 spawns first. The timer then fires, and its skip discards that partial (`first_ts 2:00 ≤ last 2:00`). One match. F1's fix does not help, because a timer at an instant fires after that instant's events.

**Failure.** A missed detection (S4.1, 08's flows).

**Fix.** The clock contract states that a completion with no remaining absence deadline is synchronous inside `OnEvents`, and only absence-bounded completions are timers. ADR-0013 §3's last sentence needs an annotation, which is the owner's call (ground rule 4). The note should say so now, because the timer-contract test's table is the owner's check.

### F5 blocker: a new instance starts with every contribution at −∞, but the executor has one clamped construct clock per rule; a key born after a clamp admits events the oracle drops

**Evidence.** Note §2 (:64) has `NewInstance(inputs) // every contribution starts at −∞`. ADR-0004 §3 applies advances only "to every resident instance". The executor keeps one `wm_c` per rule across all keys (executor.py:512, 521–541).

**P7** (live): A@0 k=a on S1, X@0 k=a on S2, S2 idle, X@10:00 k=a on S1 (`wm_c = 10:00`), then A@1:00 and B@2:00 for a new key b on S2, then X@12:00 on S1. Executor: `construct_late [4, 5]`, no match. The note: b's instance is born at −∞, so both events are admitted. When S1 reaches 12:00, b's clock becomes `min(12:00, 2:00)`, releases them, and b matches. Seeding the new instance with the current contributions is not enough either: it gives `min = 1:00`, which still admits A@1:00. Only the rule's clamped clock matches the oracle. A live key born after its stream went idle is worse: it never hears the idle advance, which was sent before it existed, so its clock waits for that stream's next event.

**Failure.** Extra or missed detections and wrong construct-late counts. Live and playback outputs also differ in ways SEMANTICS §2.1's confined guarantee does not allow.

**Fix.** Keep the construct clock per rule per shard: a `RuleClock{contributions, clock}` that applies every `CtlAdvance` whether or not any instance is resident. An instance holds only its release buffer and timers and reads its clock from the `RuleClock`. Alternatively, `NewInstance(rc *RuleClock)` copies both values. Either way the constructor's signature changes before the S3.3 freeze. Add P7 as a clock test (two keys, the second born after the clamp).

### F6 blocker: a stream-late event on an idle stream, while every other input is idle, moves the note's clock but not the executor's

**Evidence.** Note §1 (:34, :36): the next event clears the idle mark, and `Moved: true` "exactly when contrib(s) changed". The executor clears `idle[name]` before the lateness check (executor.py:531–532) and then `continue`s without recomputing `wm_c` (:533–535).

**P4** (live): X@100 on S1, A@50 on S2, S1 and S2 idle, X@90 on S1 (stream-late), B@60 on S2. Executor: `matches 1, stream_late [4], construct_late []`. The note: the late X@90 makes S1's contribution go from +∞ to 100 with `Moved: true`. The instance's min over non-idle inputs is 100, so its clock jumps to 100 and B@60 is construct-late. 0 matches.

Making the late event not clear the idle mark diverges the other way: S1 stays out of later mins that the executor caps at 100.

**Failure.** A missed detection after a fully quiet period.

**Fix (owner decision; it changes the oracle).** Recommended: change executor.py:533–535 to recompute `wm_c` on a stream-late arrival too, as the note's model does. The difference is confined to a counted construct-late race, and the harness re-run shows whether any of the 40 checks moves (none of the 2.1 checks has a stream-late event during idleness). Alternative: the registry reports a stream-late resume as an advance that updates the stored contribution without recomputing the clock. That is precise, but it is a special case in the frozen contract. Either way, add P4 as a clock test.

### F7 blocker: `math.MaxInt64` means both "idle" and "+∞"; playback end cannot fire open state, and a `d = 0` stream's virtual floor at MaxInt64 reads as idle

**Evidence.** Note §1 (:34) has an idle stream at `contrib = +∞ (math.MaxInt64)`. Note §2 (:73) leaves +∞ inputs out of the min, and "when every input is idle the clock does not move". SEMANTICS §2.1 (SEMANTICS.md:46) says "End of stream (and playback end): `wm → +∞`, all open state fires/expires"; the executor's `eng.drain(math.inf)` is at executor.py:548–549. The note has no end-of-input operation. The obvious encoding, `ObserveVirtual(MaxInt64)`, gives every `d = 0` stream a contribution of MaxInt64. The instance reads that as idle, and if every input has `d = 0` the clock does not move. A Phase-2 punctuation `w = MaxInt64` collides the same way.

**Failure.** Lost detections at the end of a log: the expect sandbox (S3.3) and conformance runs end without firing pending absences, cron closes or delay dues.

**Fix.** Make idleness a flag, not a value: add `Idle bool` to `WMAdvance`, or a kind field next to A1's `Moved`. Add an explicit end-of-input operation, `(*StreamRegistry).ObserveEnd() []WMAdvance`, whose advances carry `Contribution = MaxInt64` with `Idle = false`. Write down that `Advance` at `clock = MaxInt64` fires everything due. This is an exported-API change, so make it before S3.3.

### F8 blocker: TTL "re-armed on activity" does not say when activity counts, and a purge in the middle of `Advance` deletes the release buffer that ADR-0006 keeps inside the instance

**Evidence.** Note §2 (:77) has `Schedule(due, TTLPurge, _)` replacing the one TTL timer, re-armed "on activity". ADR-0006 §2 says `LastActivity` is updated "on every routed event" (ADR-0006:56), which is arrival. The executor sets `last_act` at release (`_process`, executor.py:380), and SEMANTICS §9 judges `wm − t_last_activity`. ADR-0006 §3 then has the purge "delete the Instance from the shard map", and that Instance holds `Clock *clock.Instance` with its buffer (ADR-0006:19, 27). Under F1's fix the purge fires before a later buffered event in the same `Advance` (P1). If the shard deletes the instance there, admitted, unreleased events are dropped (ADR-0006 F19 calls them durable state). If activity counts at arrival instead, there is no purge and P1's divergence returns.

**Failure.** Lost state (dropped admitted events) or a missed detection, depending on which reading S3.2a takes.

**Fix.** The contract states three things:
1. TTL re-arms on release, in `OnEvents`.
2. A TTL purge resets operator state and cancels every operator timer (add `CancelAll()` or a `Reset`). The clock, contributions and release buffer survive, and the fresh lifetime continues in the same `Advance`.
3. The shard frees a purged instance only when its buffer is empty.

Exit 3's quiet-key test should also assert that a purge with a non-empty buffer keeps the buffered events and that they match in the fresh lifetime.

Question 4: `Schedule` replacing the timer expresses "exactly one timer". It does not express "re-armed" until points 1–2 are written down. ADR-0006 §3's lazy "check on fire, reschedule if still active" is the cheaper equivalent (one heap operation per TTL period instead of per event) and should be the stated mechanism.

### F9 should-fix: the kinds table, which is the owner's check, lists a kind no operator uses and leaves out one a window needs

**Evidence.** `AbsenceResolution` (clock.go:56) has no user: ADR-0013 §3 schedules one completion timer per partial, and its alternatives rejected "timer-per-absence-watcher" (ADR-0013:30). ADR-0014 §5 says batch "fires at the batch punctuation (delivered as a control-timer by the router)" (ADR-0014:24), and that kind is not in the list. The card says "every timer kind" (sprint-plan S3.1 item 1).

**Failure.** The owner signs off a table that is wrong both ways. Removing a kind after the S3.3 freeze is a breaking change; adding one is only additive.

**Fix.** Drop `AbsenceResolution` now. For batch, either add `BatchClose`, stating how it is ordered against events still in the release buffer (a batch's events may not be released when its close arrives), or state that batch bypasses the release buffer and the clock. The batch decision can wait for S8, but the note should record it.

### F10 should-fix: delivering a batch in (b), with "already due fires in a later `OnTimers` call", breaks due order and makes `Cancel` misleading

**Evidence.** Note §2 (:60, :75). A callback in the middle of a batch can schedule a timer due before a later timer already in that batch (a cron close at 1:00 schedules the next close at 2:00 while a TTL at 2:30 is already in the batch), and the new timer fires after it. A completion's skip-past-last cancels sibling completions that are already in the delivered slice, so `Cancel` returns false and the stale fire still arrives. ADR-0014's window interface takes one `OnTimer(t clock.Timer, …)` at a time (ADR-0014:14).

**Fix.** Pop timers from the heap one at a time, or one instant at a time, re-checking the heap's minimum after each callback. State that a timer scheduled during a callback must not be due before the instant being processed (panic if it is), and that calling `Advance` from inside a callback is forbidden.

### F11 should-fix (forward to the owner): the oracle's result depends on how finely the watermark advances; an irrelevant event changes a `once` detection

**Evidence.** Discards run only at the end of a drain (executor.py:449–452), and the `once` spawn check reads `not self.partials.get(k)` (:435), which still sees an overrun partial. Probe: `once A→B within 10`: `A@0 A@20 B@25` gives 0 matches, and `A@0 X@15 A@20 B@25` gives 1. SEMANTICS §6.2 says "irrelevant events between are ignored". Live and playback advance the watermark in different steps, so SEMANTICS §2.1's guarantee breaks too.

**Failure.** The note's (c)-at-end reproduces the oracle only if the Go `Advance` steps exactly as the executor drains. ADR-0004 §3 sends the event and its own `CtlAdvance` as two mailbox messages. If the advance goes first, the overrun partial is discarded before the event and the result diverges.

**Fix.** Until the owner rules, pin the call order for each arrival: `Buffer` the event, then `ApplyAdvance`, then one `Advance`. The router enqueues the event before its own advance. Ask the owner whether discards should instead take part in the time ordering (the partial is dead at `t_first + within`). That changes the oracle and SEMANTICS §6.4's wording.

### F12 should-fix (forward to the owner): "every input idle → the clock does not move" contradicts SEMANTICS §2.1's single-input pin

**Evidence.** Note §2 (:73) and executor.py:521–523 (`if active:`) agree with each other. SEMANTICS §2.1 (SEMANTICS.md:50) says "a quiet stream cannot stall its own rule's timers in live mode". Under exclusion, a single-input rule, or the flagship with both streams quiet, never fires its pending absence until traffic returns. ADR-0004's context also speaks of an idle *floor* value (ADR-0004:7).

**Failure.** The live kill switch (S4.2, S5.1) stays silent through a fully quiet period.

**Fix.** The design follows the binding oracle, so it is not wrong here, but the law contradicts itself. The owner should choose between an idle floor value (a wall-derived floor the registry computes) and amending the §2.1 pin. The API can carry either. F7's `Idle` flag keeps the choice open past the freeze.

### F13 should-fix: the API forces allocation on the clock path (ADR-0010 §1)

**Evidence.** `ObserveVirtual(now) []WMAdvance` (clock.go:45) returns a fresh slice on every playback event. A heap built on `container/heap` boxes each `Timer` into `any` on `Push`. `AdvanceCallbacks` closures built per call allocate. ADR-0010 §1 lists "clock advance" among the zero-allocation stages and prefers caller-provided buffers (ADR-0010:29).

**Fix.** Use `ObserveVirtual(now int64, dst []WMAdvance) []WMAdvance` (append into the caller's buffer). Use a typed, hand-written heap. Document that the callbacks' slices are owned by the clock until the call returns, and build the callbacks once per instance. Add an `allocgate` row for `Buffer`, `ApplyAdvance`, `Advance`, `Schedule` and `Cancel` once warm. The signature change must come before S3.3.

### F14 should-fix: the metrics registry's lifetime is unstated; an `Add` after `NewShard` breaks shards that already exist

**Evidence.** metrics.go:16 has `Add` "once per counter, at deploy", and :22 has `NewShard`. S3.3 runs several apps per runtime and deploys while running, and shards live for the runtime (ADR-0006 §1). A counter added at a later deploy indexes past an existing `Shard`'s slice. Duplicate names are not specified.

**Fix.** Choose one registry per app, sealed before its shards are made: `Add` after `NewShard` panics, and a duplicate name panics. Or let `Shard.Inc` grow to the ID's size, but only off the hot path. Pin one of these in the stub's comments and add a test.

### F15 should-fix: `virtualNow`'s scope is not pinned

**Evidence.** Note §1 (:33) defines `virtualNow` as "the largest `now` passed to `ObserveVirtual`" on a per-app registry, so it covers the whole app's log. The executor's `virtual_now` covers only the construct's input streams (executor.py:528 over `arrivals`). SEMANTICS says "over the merged log" (SEMANTICS.md:52).

**Failure.** An event on a stream the rule does not read advances the rule's virtual floors, which can add playback construct-late drops (counted, but different from the oracle) when an expect block mixes streams.

**Fix.** State the scope as the whole app log and add a sentence to the note. If the owner wants the executor's scope, the driver needs per-rule virtual clocks. Ask in the A-batch.

### F16 later: smaller pins for the build

- `Cancel` and replace identity: make `(kind, ref)` unique per instance, so that `Schedule` of an existing pair replaces it. TTL's replace rule then becomes ordinary, with ref 0. The lookup must not allocate: use an index stored in each heap entry, not a map per timer. No map iteration should appear in any output order.
- `ApplyAdvance` should say what happens with duplicate `inputs`, or reject them in `NewInstance`.
- Layering is clean. `internal/clock` imports `event` (and `metrics` is allowed but unused, which is fine). The stub has no `sync`, goroutines, channels, `time` or mutable globals (archcheck main.go:65–67, 252–286, 400–425). `sync/atomic` is barred in `internal/metrics` by archcheck (`syncAllowed`, main.go:67, :260), so A4's "no atomics" is the gate's rule, not just a preference.

---

## A1–A4 verdicts

- **A1 (a `Moved` field): agree, with an addition.** The zero-value collision is real: stream 0 advancing to time 0 is the zero value. Also separate idleness from the contribution value (`Idle bool`) and add an end-of-input advance (F7). Without that, a `Moved: true` advance at `MaxInt64` is still ambiguous.
- **A2 (`Buffer` returns `constructLate`): agree.** The count belongs to the runtime. The value is only correct once F5 is fixed: a newborn instance at −∞ reports false for events the oracle drops.
- **A3 (`Cancel(kind, ref) bool`): agree, with pins.** Make `(kind, ref)` unique per instance, with `Schedule` replacing an existing pair. `Cancel` of a timer already in a delivered batch returns false, so deliver one instant at a time (F10). Add `CancelAll`/`Reset` for the TTL purge (F8). The lookup must not allocate (F13).
- **A4 (scrape through the mailbox, no atomics): agree.** archcheck already bars `sync/atomic` in `internal/metrics`. Forward to S3.2a: a scrape must not block behind a full, bounded mailbox, because a saturated shard is exactly when the metrics matter. Use a non-blocking request with the last published snapshot as the fallback.

## Probe outputs (executor.py unchanged; the note's order simulated by a modified `drain`)

```
P1 TTL interleave  executor, design-order: ([('a', 0), ('a', 660000)], [('a', 0)])
P2 equal-due tie: executor fires x= [1] (schedule order would fire x=2 first)
P3 same-instant spawn after a completion: executor [(0, 120000), (120000, 180000)]
P4 executor: matches 1 stream_late [4] construct_late []
P5 executor: matches 1 late [] []
P7 executor: matches [] stream_late [] construct_late [4, 5]
once A@0 A@20 B@25: 0    once A@0 X@15 A@20 B@25: 1    (F11)
```
