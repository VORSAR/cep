# S3.1-1 owner decisions 1–4 (2026-11-04): architecture impact review

> **Historical — S3.1-1, the clock service and the metrics counters; merged in PR #8.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-04--s22b-closed-s31s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-1-1-design.md) · [record](../sprints/s3-1-1-record.md) · reviews: [design: architecture](s3-1-1-design-architecture.md), **owner's decisions: architecture**. **← [S2.2b](../sprints/s2-2b-record.md) · [S3.1-2](../sprints/s3-1-2-record.md) →**

Reviewed: the owner's four clock decisions for S3.1-1. They are read against ADR-0004, -0005, -0006, -0010, -0012, -0013 and -0014, ARCHITECTURE.md, SEMANTICS §2.1, §6.3–§6.5 and §8–§11, the reference executor, the cards S3.2a–S9.2, and QA's tests at `2edc3f2`. The reviewer did not propose these decisions, and accuracy is not re-argued here. The question is what the architecture must change to carry each decision, and whether anything makes one of them unsound. Severities follow ground rule 15.

**Summary.** Decisions 1, 2 and 4 bring the Go contract into line with the oracle. They close F1–F5 and F11 of the design review. Their cost is text and API: they change ADR-0004's callback shape and constructor before the S3.3 freeze, and they must be pinned in four places. Decision 3 is right in intent, and it matches ADR-0004's own Context and SEMANTICS §2.1:51. As worded ("the wall clock minus its watermark bound"), though, it brings absolute wall time into the event-time clock. That makes it unsound for any source that lags the engine's wall clock by more than `d`. It also fires pending absences when "quiet" really means "not yet read" (backpressure, an outage, or Stop). Both problems have fixes that keep the decision's intent (D3-1, D3-2). One pin is about determinism (D2-2) and goes across decisions 1 and 2: the order in which a shard walks the instances that share a clock.

---

## Decision 1: walk forward in time

### What must change

- **ADR-0004 §2** (ADR-0004:32–40), the normative callbacks, and the 2026-11-02 annotation at :39. The three-step shape cannot express a walk: "(a) … as ONE chunk", then (b), then (c). The new text must say:
  - `Advance` walks the instants `t` from the oldest pending instant up to `clock`, in increasing order.
  - At each `t` it does three things, in order: it releases the buffered events at `t` (a run of events can span several instants, up to the next pending timer instant), fires the `≥` kinds due at `t`, and then fires the strict kinds at `t`, only if `clock > t`.
  - Each `TimerKind` carries its strictness: Discard and TTLPurge are strict, every other kind is `≥` (design review F2).
  - Timers are taken from the heap one instant at a time, so a timer scheduled by a callback joins the walk in its place. This closes F10.
  - The Context line (:7, "events → completions/timers → discards") and :40 ("the only place the (a)(b)(c) order exists") are reworded to match.
- **SEMANTICS §6.4:130.** The normative list is now "at each instant, in increasing time order", not "at each watermark advance". The phrase "at each watermark advance" at :124 becomes "at each instant". RT-10 (equal bounds fire) still holds: a pending completion is exempt from the discard, and the discard at `t_first + D` runs last in its instant.
- **ADR-0013** §3:22 ("discard step (c)") becomes "the strict discard at `t_first + within`". §4:23 ("in one Advance") becomes "at one instant". §6:25 ("the whole released batch") becomes "each released run".
- **ARCHITECTURE** :228 (a)(b)(c), :274.
- **The executor:** `_PatternEngine.drain` (executor.py:437–452) moves the discards into the time walk, and `_fire` (:347–374) keeps TTL strict (`pd < upto`, already the case). All 40 checks re-run, and any check that moves goes to the owner. This is a spec change, so under ground rule 9 it lands first, before Go code relies on it.
- **QA's tests** at `2edc3f2` encode (a)(b)(c) and must be rewritten: timers_test.go:58 (`OneAdvanceOrder`), :160 (`CallbackScheduledTimerFiresInSameAdvance`) and :193 (`RT10`).

### Effects

- **Determinism: an improvement.** Results no longer depend on how far the clock jumps (F11), so SEMANTICS §2.1's live/playback guarantee is not broken by watermark granularity. This also makes ADR-0010 §2's batch-size invariance, and qa-strategy §2's batch-size law, true of a correct engine. Under (c)-at-end, a forced chunk size of 1 versus the whole log changes how far `Advance` moves, and with it a `once` result. Decision 3's periodic ticks need this decision (see "Across the decisions").
- **Hot path (ADR-0010 §2).** Chunks now break at every pending instant. An `every` pattern has one discard instant per live partial, so chunks shrink towards one event. The cost per event is a heap peek plus a callback for each run; there is no allocation if the run slice is reused.
- **Mailbox/broadcast.** No change. The walk is inside one `Advance` on one goroutine.

### Findings

- **D1-1 blocker: a TTL purge in the middle of a walk now always runs with later events still buffered.** F8 was optional under the old order; now it must be carried. The purge resets operator state and cancels its timers. The release buffer and the clock survive. The shard frees the instance only when its buffer is empty. Activity re-arms at release, not at routing. Text to change:
  - ADR-0006 §3:27, "(b) deletes the Instance from the shard map".
  - ADR-0006's implementation note :56, "LastActivity updates on every routed event".

  Without this the engine either drops admitted events (lost state) or misses RT-07's purge (P1).
- **D1-2 should-fix (forward to the owner): "events at t before timers at t" holds only within one walk.** Construct-late is strict (`ts < clock`). So an event at exactly `t == clock`, admitted after the `≥` timers at `t` have fired, runs after them. A §2.2-later defeater at an absence deadline then misses, and the alert fires. Live and playback can differ here with no counted drop, which breaks the scope of SEMANTICS §2.1:55. The oracle already behaves this way (`run_pattern` with `d = 0`). The decision's wording claims more than is true. Two options:
  - *Recommended:* a "sealed instant". Once the timers at `t` have fired, an event at `t` is construct-late and counted. The decision then holds absolutely, and §2.1's guarantee holds.
  - *Alternative:* write down the exception in SEMANTICS §6.4:126.
- **D1-3 should-fix (forward to S8): a batch close has no instant.** ADR-0014 §5:24 delivers batch close as a "control-timer". A walk ordered by event time has no place for a punctuation that follows arrival order, and the batch's events may still be in the release buffer (F9). S8 must place it. Recommended: the close is due at the group's max event time and waits for `clock ≥` that time. The alternative is that batch rules bypass the release buffer. Record this in the design note now.
- **D1-4 later: discard timers.** With the walk, a partial's discard can be seen in only two ways: the `once` spawn check and memory. The engine can apply it lazily (a partial is dead at event `ts` iff `t_first + within < ts`) and sweep with one per-instance timer. This removes a heap push for each spawn and the A3 `Cancel` whenever a partial turns pending. Keep the Discard kind in the frozen API, because TTL needs strict kinds anyway, and let S4.1 choose the mechanism against `allocgate`.

---

## Decision 2: one clock per rule (per shard), shared by its keys

### What must change

- **ADR-0004.** The title (:1, "per-instance construct clocks"). §1:13: the per-instance level keeps only the buffer and timers, and a new per-(rule, shard) level owns the contributions and the clamped clock. §2:26–30: `Instance` loses `contributions` and `clock`. §3:42: a shard applies `CtlAdvance` to each rule clock that reads `s`, whether or not any instance is resident, and then advances instances.
- **The API, before the S3.3 freeze:**
  - `NewRuleClock(inputs []int) *RuleClock` with `ApplyAdvance` and `Clock`.
  - `NewInstance(rc *RuleClock)`, or `Advance` taking the clock value.
  - `Buffer` judges construct-late against the rule clock.
  - QA's tests use `NewInstance(inputs)` (edges_test.go, helpers_test.go) and must change.
- **ADR-0006** §2:19 (the comment on `Clock *clock.Instance`) and :26 (the state registry). Rule clocks are durable state. Every shard applies the same FIFO sequence of advances, so the per-shard copies are replicas of one value. A Phase-2 snapshot stores one value per rule and restores it onto any shard count. Write that down; it keeps `shards` reconfigurable.
- **ARCHITECTURE** :226 and :356 ("per rule instance") become "per rule".

### Effects

- **Correctness: a fix.** It closes F5 (a key born after the clamp) and the case of a live key born after its stream went idle. It also simplifies D1-1, because freeing a purged instance now loses no clock.
- **Mailbox/broadcast.** It is unchanged and cheaper: one `ApplyAdvance` per (rule, shard), not one per resident instance.
- **Replay.** It matches the executor's single `wm_c` (executor.py:512).

### Findings

- **D2-1 should-fix (S3.2a scope; it becomes a blocker for O2 once decision 3 lands): only instances with due work should run.** ADR-0004 §3 runs `Advance` on every resident instance, which costs O(resident keys) per advance. Decision 3 makes advances periodic, so an idle runtime with a million keys would walk a million instances on every tick. With a shared clock the shard can keep a due index per (rule, shard): a heap of instances keyed by their next pending instant (the minimum of the buffer head and the timer head), with only those `≤ clock` advanced. This replaces ADR-0004 §5's "per-shard facade `clock.Timers`" (:46). S3.1-1 must define the hook before the freeze, for example `Instance.NextDue() int64` or a scheduler type. S3.2a item 3 grows by about 1–2 SP.
- **D2-2 blocker (non-deterministic replay, ground rule 11): the order of the walk across instances and rules is unpinned.** "When a rule's clock advances" now advances many instances on one shard. Playback is byte-identical (ADR-0005 §3, ADR-0006 §6) only if they run in a stated order. If they run in Go map order, the output order changes from run to run. The executor walks every key in one time-ordered pass and fires timers by `str(key)` (executor.py:352).
  - Pin: walk the rule's due instances in order of `(instant, key ordinal)`, where the key ordinal is the instance's birth order or its canonical key encoding. Walk rules in declaration order.
  - Make the executor's `_fire` order match whichever is chosen.
  - S3.2a exit 1's "planted map-iteration order" proof must cover this path.
- **D2-3 later: construct-late events should not create instances.** Check the rule clock before creating an instance for a first event that is construct-late. Otherwise every such event creates an instance and leaves it to the TTL (O2).

---

## Decision 3: timers fire on time when every input is quiet

### What must change

- **ADR-0004 §4:44.** It now says "excluded from the min … +∞". The new text: contribution = `max(adm, idle floor)`, every contribution is monotone, and there is no idle value. §2:22 changes `ObserveIdle(sid)`: it takes a time, or becomes `ObserveWall(now, dst) []WMAdvance` (appending into the caller's buffer, F13). `StreamSpec` gains `IdleAfter`, unless the runtime decides idleness. F7's `Idle` flag is no longer needed. F7's end-of-input operation still is, because +∞ now means only "end". The Context (:7) already describes an "idle floor", so the ADR becomes consistent with itself.
- **SEMANTICS.** §2.1:55 must say what the floor is (see D3-1). §6.4:128 ("detection latency is `d` past the bound, computable from the text") gains "plus the tick period" in live mode. §11.10:210 still lists `idle after` only for multi-stream constructs, while §2.1:51, SPEC §4.2 and the validator (`internal/validate/graph.go:164`) require it on every stream that feeds a timer. Decision 3 relies on the wider rule, so fix the text (later).
- **ARCHITECTURE :332.** "Idleness timing … only affect *when*" is no longer true. A quiet period can now decide *whether* an absence fires, with any later defeater counted as construct-late. Reword.
- **The executor's live mode** (executor.py:516–525, 539–541). The `("idle", s)` markers carry a wall time. P4/F6 dissolves: with no exclusion, a stream-late event changes nothing. F12 is resolved.
- **QA's tests** assert the opposite of decision 3 and must change: edges_test.go:10 `TestEdge_AllInputsIdleLeavesTheClock`, and harness_test.go:137 (idle-resume), which needs the wall input.

### Where wall time enters, and whether there is still a single ordering domain

The archcheck ban covers only operator packages (`tools/archcheck/main.go:233`), and `internal/clock` imports no `time`. So wall time must be read by the live `clock.Driver` (ARCHITECTURE :83) in `internal/runtime`, as a monotonic clock, and passed as int64 ms. ADR-0004 §7 is kept.

The single ordering domain (ADR-0004 Alternatives :54; §1:12, a registry with one writer) is kept **only if** two things hold:
- a tick is a message in the router's intake FIFO, handled by the router like an event;
- the resulting `CtlAdvance` travels through the mailboxes.

A ticker goroutine that writes the registry directly, or that bypasses the intake, breaks both.

### Findings

- **D3-1 blocker: absolute `wall − d_s` mixes wall time into event time; anchor the floor in the stream's own event time instead.**
  - **Who loses events.** Any source whose event times lag the engine's wall clock by more than `d_s`. That includes transport latency on a `d = 0` ascending stream, producer clock skew, clients that buffer, and a backfill through the live API.
  - **What happens.** After every idle period, that source's next events are construct-late. The floor is monotone and stays in force after the stream resumes, so for a source with a steady lag the drops continue until event time catches up. For a source that is permanently behind, that is never. The first event after a quiet spell is often the flagship's anchor (the sensitive read), so these are missed detections. They are counted, but they are missed.
  - **What it changes in the language.** The meaning of `d` changes silently, from "bounded out-of-orderness" (SEMANTICS §2.1:44) to "also the maximum lag behind the engine's wall clock". That is a language change (ground rule 3).
  - **Fix, keeping the decision's intent and its playback analogy.** Playback's `virtual_now` is in event time, the max event time of the merged log, not the wall. The live analogue is `floor(s) = adm(s) + (wallNow − wallAt(adm(s) last raised))`, measured on the monotonic clock.
    - Only latency *jitter* above `d` causes a drop, which is `d`'s existing meaning.
    - Constant skew is harmless.
    - NTP steps cannot move the floor.
    - The time-shift law stays meaningful for live tests.
  - **The empty stream needs a pinned rule** because it has no anchor. Recommended: anchor at the app's highest admission watermark when the stream is declared idle.
- **D3-2 blocker (ordering under load) plus an owner question (outages): "quiet" is not "not yet read".**
  - **Under load.** Under ADR-0006 §5 the router blocks on a full mailbox while a stream's events wait in the intake. If idleness is judged from the router's processing time, the stream looks idle. A tick then raises the floor, fires pending absences, and drops the queued events as construct-late. Wrong detections appear exactly when the system is saturated.
  - **Fix (required).** Ticks travel through the intake FIFO, and idleness and the floor are computed from arrival stamps, never from the router's processing time.
  - **Owner question (fail-open or fail-closed).** A disconnected source, a rebalance or a suspended process also looks quiet. Before this decision, a rule whose every input was quiet stopped. Now every pending absence fires: the kill switch fires on a total input outage. If that is not wanted, the `cepio.Source` contract (S3.3 item 2) needs a signal for "disconnected, not idle" that suppresses the floor.
- **D3-3 should-fix (conflicts with ADR-0012 §3:15 and its rejected alternative at :25): Stop must freeze the floor.** `Stop` stops sources first and then drains. If ticks keep raising the floors of streams that were already idle, the drain fires absences that are due in those seconds. ADR-0012 rejected exactly that ("a shutdown must not fabricate detections"). Pin: the tick source stops, freezing every floor, before the sources stop. S4.2 exit 3 (the `drained_pending` test) must include a stream that is already idle.
- **D3-4 should-fix: the tick is now periodic.** Its period becomes a config knob and part of detection latency. S3.2a item 3's "injectable idle-tick source" becomes an injectable monotonic clock plus a tick schedule, which tests drive with no sleeps. Decide whether `cep.Config` exposes it to embedders, which would be an API change at S3.3. The broadcast cost, O(shards) per tick for each idle stream, is fine only with D2-1.

---

## Decision 4: a pattern completes at its last event

### What must change

- **ADR-0013 §3:22.** Delete the last sentence ("its 'timer' is due ≤ current clock and fires in the same Advance"). New text: a match completes during the processing of its completing event when every absence deadline is strictly before that event (executor.py:411). Otherwise it schedules a Completion due at `firstTs + maxAbsenceBound`.
- **ADR-0004 §2:40** ("pending completions … are timers") stays true.
- **The exit-2 table in the design note:** the Completion kind is used only by absence-pending matches. Add P3 as a test.

### Effects

There is one fewer heap push and pop per match (ADR-0010). Emission happens inside `OnEvents`, so the outbox and the rate limiter (S3.2b) see inline matches in event order. The tie order of F3 is still needed for pending completions. S4.1 keeps its size and its risk goes down.

**Checked, and it holds:** §6.5's earliest-wins rule survives the split between the inline and timer paths. Within one pattern, every bound anchors at `t_first`, so a completion deadline is `t_first + maxAbsenceBound`, monotone in `t_first`. A match that completes inline at instant `t` must therefore have started before any match still pending with a deadline `≥ t`. Write this in ADR-0013 §4, so that nobody later adds per-step absence anchors without revisiting it.

### Findings

- **D4-1 blocker (wording): the rule is per match, not per pattern.** "A pattern with no absence completes inside event processing" reads as a property of the pattern. An absence pattern whose windows all closed before its last positive, built from that wording, schedules a timer due now. P3's lost same-instant partial comes back. ADR-0013 must state the per-match rule: all deadlines `< ts`.
- **D4-2 should-fix (forward to the owner; conflicts with SEMANTICS §6.5:135): the decision's stated purpose holds only on the inline path.** The purpose is that a same-instant partial is not wiped by skip-past-last. On the timer path the skip compares times (executor.py:340–341 `q.first_ts > p.last_pos_ts`; ADR-0013 §4:23 "firstTs ≤ lastPositiveTs"). It therefore still kills a partial anchored at the last capture's instant but later in §2.2 order. SEMANTICS §6.5's literal "first event … at or before the … last event" supports killing on both paths, so it contradicts the decision too.
  - *Recommended:* compare the anchor's `(ts, stream id, seq)` with that of the last captured event. Both paths then agree with decision 4's intent. This changes executor :340–341 and §6.5's wording, and it needs no new state, because both events are in `captures`.
  - *Alternative:* record the asymmetry. Left unpinned, it becomes a blocker at S4.1, because an agent following §6.5 literally rebuilds P3.

---

## Across the decisions

- **3 depends on 1.** Periodic ticks produce many small advances. Only decision 1's independence from step size keeps tick timing out of the results.
- **2 plus 3 makes D2-1 mandatory**, and with it D2-2's ordering pin.
- **1 and 4 compose.** At one instant: inline completions (during the events), then Completion timers, then strict discards and the TTL. RT-10 and §6.3's boundary example (P5) hold.

**The S3.3 freeze (`api/clock.golden`).** Every change below is to an exported signature and must land in S3.1-1:
- `RuleClock` and the constructor;
- the walk callbacks;
- strictness per kind;
- `ObserveIdle`/`ObserveWall` with a time and a destination buffer;
- `ObserveEnd`;
- the due-index hook;
- `StreamSpec.IdleAfter` (if the registry decides idleness).

**Sprint cards.**

| Card | Change | Size and risk |
|---|---|---|
| S3.1 (sprint-plan:263, :268–269) | Item 1's "(c)" and "idle exclusion" wording; exit 2 "within (b) … only in (c)" becomes per-instant order and strictness per kind; exit 3's "`Instance` trace" becomes a `RuleClock` trace; add P1, P3, P5 and P7 | +1–2 SP, plus the oracle change first. Within S3.1-1 only if the owner consents; ground rule 6 says split before starting |
| S3.2a (:282, :289) | Item 3: the due index (D2-1), ticks through the intake (D3-2), an injectable monotonic clock (D3-4); item 2/exit 4: purge keeps the buffer, activity at release (D1-1) | +1–2 SP; concurrency risk up (tick ordering) |
| S3.2b (:300, :304) | Exit 1's equivalence test must use the pinned instance order (D2-2) | none |
| S3.3 (:322) | The Stop sequence freezes ticks (D3-3); the `cepio.Source` health signal if the owner chooses fail-open (D3-2) | small API Δ |
| S4.1 (:343) | Inline completion per match (D4-1); the skip comparison (D4-2); lazy discard option (D1-4) | risk down |
| S4.2 (:361, :366–367) | The live quiet-key test uses the fake wall clock; the drain test covers an already-idle stream; the oracle's "idle" shape carries wall times | none |
| S8 (:475) | Where the batch close sits in the walk (D1-3) | design-panel item |
| S9.1 (:494, exit 4) | A live cron close "through idleness" now fires at `wall − …`, anchored per D3-1 | none if D3-1 is adopted |
| S9.2 (:513) | Delay dues and the TTL purge use the per-instant order; the TTL-purge case runs with a non-empty buffer (D1-1) | none |

## Can the four be carried as decided?

**Yes, with the listed changes.** Decision 3 needs one correction to how it is realised.

- Decisions 1, 2 and 4 are sound. They need the API and text changes above, plus three pins:
  - D1-1: a purge keeps the buffer.
  - D2-2: a deterministic walk across instances.
  - D4-1: completion is decided per match.
- Decision 3 is sound only if two things hold. The idle floor must be anchored in the stream's own event time, extrapolated at wall rate (D3-1), not in absolute `wall − d_s`. And ticks must ride the intake FIFO, stamped at arrival (D3-2). As literally worded it causes missed detections whenever a source lags the engine's wall clock by more than `d`, and false absence fires under backpressure.
- The two owner questions do not block the build if the recommended defaults are taken:
  - D1-2: seal the instant.
  - D4-2: compare in §2.2 order.
- One question needs a product answer: is the kill switch fail-open or fail-closed when every input source is down (D3-2)?
