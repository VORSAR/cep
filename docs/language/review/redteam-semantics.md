# Red-team review — SEMANTICS.md (L2)

> **Historical — language workstream, L2 red-team review, 2026-10-31.** Normative today: [SPEC.md](../SPEC.md) (v0.1, revised since; see its change log), the grammar, [errors.md](../errors.md) and the corpus; [SEMANTICS.md](../SEMANTICS.md) elaborates. Where they disagree with this file, they win. **Workstream:** [PLAN](../PLAN.md) · [DESIGN (L0)](../DESIGN.md) · [PRIOR-ART (L1)](../PRIOR-ART.md) · [SEMANTICS (L2)](../SEMANTICS.md) · [SYNTAX (L3)](../SYNTAX.md) · [SPEC (L4)](../SPEC.md) · [EVAL (L5)](../EVAL.md) · [REVIEW (L6)](../REVIEW.md).

Adversarial semantics review of [SEMANTICS.md](../SEMANTICS.md) against [DESIGN.md](../DESIGN.md)'s Fixed constraints, the L2 harness ([checks/](../checks/), 23/23 green, re-run 2026-10-31), and the Phase-1 flagship rule ([02-roadmap.md](../../product/02-roadmap.md)). Every finding carries a concrete timeline or rule shape; executor-reproducible ones are pinned in [experiments/exp_redteam.py](experiments/exp_redteam.py) (E1–E8, all reproduced — run from repo root: `python3 docs/language/review/experiments/exp_redteam.py`). Severity per ground rule 15: **blocker** = wrong/missed detection, lost state, contradiction; **should-fix** = wrong with a workaround or misleading; **later** = polish. No fixes proposed; findings only.

## Findings table

| ID | Severity | Section(s) | Claim |
|---|---|---|---|
| RT-01 | blocker | §2.1, §6.4 | Quiet approval stream stalls the min watermark; playback idleness is underivable for an empty stream, so the flagship absence is untestable in the conformance mode; an idle-resume event breaks the live/playback "never on *what*" guarantee |
| RT-02 | blocker | §2.1 vs §11.10 | `idle after D` is declared mandatory but absent from the §11.10 deploy-error list — a rule without it deploys and silently never fires |
| RT-03 | blocker | §6.4 vs harness | An approval between read and egress defeats the absence per §6.4's definition, but the "validated" executor ignores it and fires the kill switch (E1) |
| RT-04 | blocker | §6.5 | Skip-past-last's definition ("consumed events cannot join") contradicts its worked example and the executor, which kill partials holding only unconsumed events (E2) |
| RT-05 | blocker | §7 vs §8 | The two normative tables give two incompatible cron allow-lateness models (single delayed append fire vs on-close + re-fire changelog); §8's row self-contradicts; only one model is harness-covered (E6) |
| RT-06 | blocker | §9 vs §7/§8 | TTL purge destroys changelog (sort) window rows "silently" with no retractions — downstream state is corrupted; "window extent" is undefined for sort/cron so the §11.3 check is unimplementable |
| RT-07 | blocker | §9 vs §6.5/§8 | Purge+return re-arms at-most-once patterns (harness-pinned, unstated) and resets `output first every D`; §11.3 never checks the rate-limit interval, so §8's suppression guarantee is violated by a deploy-legal config (E5) |
| RT-08 | blocker | §9, §1.1 | Partition routing for a keyless second stream is undefined: `HumanApprovals` has no `agentId`, so the flagship rule either fails deploy or drops every approval as erroneous → false kill switches |
| RT-09 | blocker | §7, §2.1, §6.1 | No watermark/lateness is defined for derived streams; `delay` emits events behind the releasing watermark, so under the only stated clock the delayed stream is late-dropped by any downstream pattern (E4) |
| RT-10 | blocker | §6.3 vs §6.4 | At one watermark advance, partial-discard (`wm > t_first+D`) and absence-fire (`wm ≥ t_first+D`) both apply; priority is unstated and the discard-first reading misses every `within D` + `for D` (equal-bound) alert — the flagship shape |
| RT-11 | should-fix | §7 | Cron fire-times across DST transitions (repeated 01:30, nonexistent 02:30) are undefined; bucket boundaries become implementation-defined; harness takes precomputed boundaries, so untested |
| RT-12 | should-fix | §5.1, §10 | Normalization is not version-pinned: Cn (unassigned) stripping guarantees cross-Unicode-release divergence; the harness validates the *full* case fold, not the specced simple fold (E8) |
| RT-13 | should-fix | §5.1, §6.6, §3 | Forensic text identity unstated: whether evidence carries original or normalized text, whether `text_invisibles` metadata is in the match output, and when the flag is set (ingestion vs per-operator side effect — the latter contradicts §3 purity) |
| RT-14 | should-fix | §1.1 × §6.4 | Null-strict predicates inside absence steps invert the §1.1 rationale: an approval with a null `sessionId` cannot defeat the absence → false kill of an approved agent; lint covers only `!=` |
| RT-15 | should-fix | §3, §10 | "Event erroneous → stream error policy" has stream-wide blast radius as written: one rule's division-by-zero censors the event for every rule, falsely firing other rules' absences |
| RT-16 | should-fix | §8 × §7 | `output first every D` is keyed state, yet §8 defines it only "per partition key"; on an unpartitioned stateless query the key is undefined and §7's stateful-outside-partition ban is silent about it |
| RT-17 | should-fix | §7 | Sort-window tie eviction is unspecified; equal-valued arrivals either churn the changelog unboundedly or emit nothing — both readings fit the text (E3) |
| RT-18 | should-fix | §6.4, §6.6 | Conjunct absence with a positive event after the absence deadline: match legality is undefined and the output timestamp is ambiguous; executor emits `match_end` *earlier than a captured event* (E7) |
| RT-19 | should-fix | §10 | `${VAR}` substitution mechanism (textual vs token-level) is undefined; the textual reading lets a "parameter" change rule structure, breaking the Tweaker contract |
| RT-20 | later | §2.2, §6.4, §6.5 | "Stream id" in the tie-break order is never assigned; a B tied at `t_first` counts or not depending on it; earliest-started ties between partials are unbroken in the text |
| RT-21 | later | §7, §8 | Batch window output is undefined (emitted shape, event timestamp) — downstream mandatory-clock constructs are unsatisfiable; non-contiguous/redelivered batch ids fire the same id twice |
| RT-22 | later | §5.2 | "Maximal alphanumeric sequences" leaves *alphanumeric* undefined over Unicode; term-match results are implementation-defined (e.g. `²`, `_`) |

---

## RT-01 (blocker) — quiet approval stream: the flagship absence stalls, and playback cannot reproduce it

**Spec text at fault (§2.1):** "Constructs reading multiple streams … advance on the **minimum** of their input watermarks." / "In **playback mode**, idleness is derived deterministically from event-time gaps in the log." / "Live and playback may therefore differ on *when* output appears, never on *what* the output is given the same admitted events; the conformance suite runs playback." Plus §6.4's claim: "detection latency is `d` past the bound, computable from the text."

**Counterexample A (empty stream — the flagship's canonical case).** Kill-switch rule reads `AgentActions` (busy) and `HumanApprovals`. The attack scenario *is* "no approval ever arrives": the `HumanApprovals` log is **empty**. An empty log has no event-time gaps, so the stated playback-idleness derivation produces nothing; `wm_HumanApprovals` never advances, the min watermark never passes `t_first + 5 min`, and the absence fires only at playback end (`wm → +∞`). Consequence: in the conformance mode the spec itself designates, the flagship alert's firing *time* is untestable — it always collapses to end-of-log — while §6.4 promises latency "computable from the text." The harness never exercises this: `test_semantics.py` feeds all three event types through **one** merged stream with **one** watermark (`_killswitch`, lines 39–47); §2.1's min-of-watermarks and idleness have zero executable coverage despite the "validated 23/23" status line.

**Counterexample B (idle-resume breaks "never on *what*").** Live mode, `HumanApprovals` declares `idle after 10 min`, `d = 2 min`. Read at 10:00, egress at 10:01 (match pending, absence due 10:05). No approval traffic since 09:50 → at wall 10:00+ the stream is idle and excluded from the min; `AgentActions` traffic advances the min past 10:05 → **alert fires**. At wall 10:20 a delayed approval with event time 10:04 arrives. It is the stream's newest event, `t ≥ wm_HumanApprovals`, so per §6.5 ("An event with `t < wm` **at its stream** is dropped") it is *not* late — it is admitted. Playback of the same log (approval present at 10:04) derives no idleness gap large enough → approval processed before the deadline → **no alert**. Same admitted events, different *what*: direct contradiction of §2.1's guarantee, and the already-emitted append-mode alert cannot be retracted. The spec must either drop admitted-but-overtaken events (contradicting §6.5's lateness definition) or process them after the absence fired (contradicting §6.4's "fires iff **no** qualifying B is processed … with `t_B ≤ t_first + D`"). Also unaddressed: when an idle stream re-enters the min, the construct-level minimum can *retreat* (its wm is behind the advanced min), violating "Monotonic (never retreats)" at the construct level.

## RT-02 (blocker) — "mandatory" idleness clause is not in the mandatory-clause list

**Spec text at fault:** §2.1 "**Idleness (mandatory for multi-stream constructs):** a stream declares `idle after D`." vs §11.10, the closed enumeration of missing-clause deploy errors: "(timestamp, watermark `d`, emission, lateness, TTL, text-operator case mode, cron tz)" — no `idle`.

**Counterexample.** Deploy the flagship rule with no `idle after` on `HumanApprovals`. Per §11 the deploy succeeds (nothing in the list is missing). Per §2.1 the multi-stream pattern needs idleness it doesn't have: with quiet approvals the min watermark stalls (RT-01) and the absence silently never fires in live mode — a missed detection produced by a configuration §11 declares legal. Internal contradiction between the two sections; §11's own purpose statement ("zero silent defaults", DESIGN Fixed list) is defeated.

## RT-03 (blocker) — approval between read and egress does not defeat the absence (spec vs harness)

**Spec text at fault (§6.4):** "fires iff **no** qualifying B is processed after the match start with `t_B ≤ t_first + D`"; §6.3: "All pattern bounds anchor at the match start." Roadmap flagship: "**no human-approval event arrives in between**."

**Counterexample (E1, reproduced).** Rule: `read → egress and not HumanApproval for 5 min, within 5 min`. Timeline (in arrival = event-time order): read 10:00, **approval 10:01**, egress 10:02, unrelated event 10:08. The approval is a qualifying B, processed after the match start, `t_B = 10:01 ≤ 10:05`. Per the quoted sentence: absence defeated, **0 alerts**. The reference executor (and the harness's own `_killswitch` modeling, which the 23/23 suite pins) emits **1 alert** — it consults the negated step only after the partial has captured the egress, so any approval arriving before the egress is invisible (`executor.py` `process()`: only `steps[p.idx]` is checked). A human pre-approves the action; the engine kills the agent anyway. Spec and validated harness directly contradict on the flagship scenario; one of them is wrong, and the roadmap's prose sides with the spec text against the harness.

## RT-04 (blocker) — skip-past-last: definition and worked example describe two different policies

**Spec text at fault (§6.5):** definition: "**skip-past-last** — events consumed by a completed match cannot join another match of the same rule in the same partition." Worked example: "Earliest wins: the A₁ match emits, B is consumed, **the A₂ partial is discarded**."

**Counterexample (E2, reproduced).** For-each pattern `e1=A → e2=B within 5 min`, skip-past-last. Timeline: A₁ 10:00, A₂ 10:01, B₁ 10:02, B₂ 10:03. Earliest-wins gives the match (A₁,B₁); its consumed events are {A₁,B₁}. A₂ and B₂ were consumed by nothing, so under the *definition* (A₂,B₂) must match: **2 alerts**. The executor implements the worked example's stronger rule — on emit it discards every partial with `first_ts ≤ match end` (`executor.py` `emit()`), killing the A₂ partial although none of its events were consumed: **1 alert**. These are the two classic distinct policies (no-event-reuse vs MATCH_RECOGNIZE's SKIP PAST LAST ROW); the spec states the first and implements the second. Under the definition reading the executor has a missed detection; under the example reading the normative sentence is false. Either way: contradiction in the artifact.

## RT-05 (blocker) — cron allowed-lateness: §7 and §8 specify incompatible behaviors

**Spec text at fault.** §7 cron row: "Fires: `wm ≥ bᵢ₊₁ + allowed_lateness`", emission "on close", lateness "drop / allow D". §8 cron row: "on close | n/a (single fire) | drop / allow D (**late firing re-emits full result, accumulating**) | append; **changelog if `allow` with re-fire**."

**Counterexample (E6, reproduced).** Bucket [10:00, 11:00), `allow 2 min`, `d = 0`. Arrival order: a(ts 10:59), c(ts 11:01) — wm passes the 11:00 boundary — then b(ts 10:59:30), then d(ts 11:03) — wm ≥ 11:02. **§7 model** (what the executor implements): the bucket stays open until `wm ≥ 11:02`, fires **once**, append, containing {a, b}. **§8 model:** the bucket fires on close (`wm ≥ 11:00`) with {a}; b's late arrival "re-emits full result, accumulating" → a second emission {a, b}, making the producer changelog-mode. The two models differ in firing count (1 vs 2), output mode (append vs changelog), and statically in sink legality: under §8, cron-with-allow into the MVP email/webhook sinks is a deploy error (§8: "append-only sinks … reject changelog producers"); under §7 it is legal. §8's own row also self-contradicts (accumulation "n/a (single fire)" next to "re-emits … accumulating"). The harness covers only the §7 reading, so "trigger × accumulation × lateness tabulated" (§12 checklist) is checked against a table that disagrees with itself.

## RT-06 (blocker) — TTL purge of a changelog window loses retractions; "window extent" undefined

**Spec text at fault (§9):** "Destruction drops partial matches and **window state silently**"; "deploy-time check: TTL must be ≥ every pattern bound (`within`/`for`) and **window extent** inside the block." §7/§8: sort output is changelog ("a consumer summing inserts without applying retractions double-counts — which is why sort's output is changelog-mode").

**Counterexample.** Partition block: `sort(2, score, desc)`, TTL 1 hour. Key k: arrivals score 5 (10:00) and 7 (10:05) → emits +5, +7. Key idle; at `wm ≥ 11:05` the instance is destroyed *silently* — **no −5/−7 retractions are emitted**. Key returns at 14:00 with scores 6, 4 → fresh instance emits +6, +4. Any downstream changelog consumer now holds {5, 7, 6, 4} as the "top 2": permanently corrupted derived state — exactly the double-count §7's own example warns about, mandated here by §9. Independently, the §11.3 check compares TTL to "window extent", which is **undefined for sort** (rows live until evicted — unbounded) and **unbounded/variable for cron** (`'0 0 29 2 *'` has multi-year buckets; DST buckets vary). Taken literally, no finite TTL is legal in a block containing a sort window, yet TTL is mandatory on every block and stateful constructs may not live outside one (§7) — the MVP sort window is either undeployable or deployed with a vacuous check. The spec defines "extent" nowhere.

## RT-07 (blocker) — purge + return resets rate-limit and at-most-once state; §11.3 doesn't guard it

**Spec text at fault.** §9: "runs every contained construct in isolation per key value: separate pattern state, window state, **rate-limit state**" + "Destruction drops **partial matches and window state** silently" (rate-limit and match-completion state unmentioned). §6.5: "the pattern matches **at most once per partition instance** (then stops)" — instance birth/lifetime defined nowhere. §8: "after an emission at `t_out`, **every output with `t ≤ t_out + D` is suppressed**." §9/§11.3: "TTL must be ≥ every pattern bound … and window extent" — the rate-limit interval `D` is not in the list.

**Counterexample (E5, reproduced for the at-most-once half).** Unmarked (at-most-once) pattern `A → B within 2 min`, TTL 5 min — deploy-legal (5 ≥ 2). Timeline: A 10:00, B 10:01 → match 1; key idle; other-key traffic advances wm past 10:06 → purge; A 10:11, B 10:12 → **match 2**. The executor emits both (`done.discard(k)` on purge, `executor.py` fire_timers) — so the 23/23-green harness *pins* a behavior ("at most once per TTL-idle period") that SEMANTICS.md never states; the normative sentence "then stops" is false as validated. **Rate-limit half:** kill-switch with `output first every 10 min`, pattern bounds 5 min, TTL 5 min — passes §11.3. Alert at 10:00 (`t_out`). Key idle 5 min → purge destroys per-key rate-limit state (it is per-instance state per §9's first sentence). Match at 10:08 → emits, violating §8's quoted guarantee (10:08 ≤ 10:00 + 10 min must be suppressed) — an attacker defeats alert-storm suppression by pacing bursts TTL apart. If instead rate-limit state *survives* purge, §9's "destroyed" instance retains state — contradiction in the other direction. The spec must pick; it doesn't, and §11.3's check cannot catch it because the rate-limit interval is outside its comparison set. (Harness gap: `rate_limit_first_every` is never composed with TTL purge in any check.)

## RT-08 (blocker) — the flagship's second stream has no partition key: routing undefined, approvals become errors

**Spec text at fault (§9):** "`partition by key of Stream` runs every contained construct in isolation per key value … Key expressions are evaluated per event; null key → event error policy (§1.1)." §1.1: "A null **partition key** … is an event-level error → … never a silent 'null partition'."

**Counterexample.** The Phase-1 rule partitions `with (agentId of AgentActions)` and consumes `HumanApprovals (sessionId string, approvedAction string)` inside the block — a schema with **no `agentId`**. What key does an approval event route to? The spec gives exactly two mechanisms, both fatal: (a) evaluate the key expression per event → `agentId` is an unknown attribute on `HumanApprovals` → §3 closed-catalog **deploy error**: the flagship rule does not compile; or (b) treat the key as null → §1.1 event-level error → **every approval is dropped** (`errored_events`), so no absence is ever defeated. Timeline for (b): read 10:00, egress 10:01, matching approval 10:02 (dropped as erroneous) → kill switch fires at 10:05 against an approved agent — systematic false detections. No third mechanism (broadcast-to-all-instances, join-key routing) exists anywhere in SEMANTICS.md. The harness dodges the question by stamping `agent="a"` onto its `HumanApproval` test events (`test_semantics.py` lines 88–112) — events the declared flagship schema cannot produce. Also interacts with §9 TTL: "activity = any event routed to that key" is undefined for a stream that routes nowhere.

## RT-09 (blocker) — derived streams have no watermark; `delay` output is late by construction

**Spec text at fault.** §2 defines clocks only via *stream declarations* ("Stream declaration must state one of: `timestamp = <attr>` …"); §2.1 defines watermarks only from per-stream `d` over arriving events. §6.1 *mandates* composition "by naming intermediate streams instead." §7: "delay re-times events, it never reorders them ahead of the watermark"; emitted "with timestamp `t + D`."

**Counterexample (E4, reproduced).** Source S (`d = 2 min`) → `delay(10 min)` → named intermediate stream → pattern. Events on S: 10:00, then 10:30. The 10:00 event (due 10:10) is held until the 10:30 arrival advances `wm` to 10:28 and releases it — stamped 10:10, i.e. **18 minutes behind the watermark that released it**. The spec never says what watermark the delayed stream carries. Every candidate fails: (a) inherit the upstream watermark → the event has `t < wm` on arrival downstream → §6.5 drops it (E4: `late=[1]`, 0 matches) — `delay` feeding a pattern silently discards nearly everything, since release only happens when `wm ≥ due` and generally `wm > due`; (b) declare a fresh `d` on the intermediate stream → no principled value exists and the mandatory-clause machinery (§11.10) says nothing about derived streams; (c) no watermark → the pattern's mandatory clock clause (§2) is unsatisfiable. The same hole swallows *every* intermediate stream the §6.1 flatness rule forces authors into, and pattern outputs too (RT-18 shows a match emitted with `match_end` behind the watermark). The composition story rests on an undefined construct.

## RT-10 (blocker) — discard vs absence-fire at the same watermark advance is unordered

**Spec text at fault.** §6.3: "A partial match is discarded when `wm > t_first + D`." §6.4: "The absence resolves when `wm ≥ t_first + D` — … with buffered events released *before* timers fire" (orders events vs timers only — never timers vs discards).

**Counterexample.** The flagship shape: `within 5 min` and `not … for 5 min` — equal bounds, both anchored at `t_first` (§6.3). Read 10:00, egress 10:03, no approval. Watermarks advance in jumps: the next event moves `wm` from 10:04 to 10:06 in one step. At that single advance, *both* conditions are true: `wm ≥ 10:05` (absence must fire) and `wm > 10:05` (partial must be discarded). The spec gives no priority. A conforming implementation that evaluates §6.3's discard first silently deletes the pending-absence partial and the alert never fires — for **every** rule where `within` equals the absence bound, which is exactly how the roadmap writes the kill switch (`for 5 min … within 5 min`). The executor happens to fire timers before processing the event that would discard (`drain()`), but SEMANTICS.md is the normative artifact and it permits the missed-detection reading. (Secondary gap: whether a completed-pending-absence state is still a "partial match" for §6.3's purposes is itself undefined.)

## RT-11 (should-fix) — cron fire-times across DST are undefined (repeated and nonexistent local times)

**Spec text at fault (§7):** "tumbling buckets whose boundaries are successive fire-times of the cron expression, evaluated in the **mandatory** IANA timezone `tz` … bucket = half-open `[bᵢ, bᵢ₊₁)`." No rule for local times that occur twice or never.

**Counterexample.** `cron('30 1 * * *', 'America/New_York')`. On 2026-11-01 (DST ends), 01:30 local occurs twice: 05:30Z (EDT) and 06:30Z (EST). Is the day's fire-time set {05:30Z}, {06:30Z}, or both? Event at 06:00Z: under "both", it falls in bucket [05:30Z, 06:30Z); under "first only", in bucket [05:30Z, *next day* 06:30Z) — different aggregate contents for the same log. Spring side: `cron('30 2 * * *', 'America/New_York')` on 2026-03-08 — 02:30 local never exists; cron libraries disagree (skip vs fire at 03:00), so the bucket silently spans ~48 h or doesn't, implementation-defined. This breaks the Reader promise ("what fires, when … from the text alone") and makes §10's byte-identical replay contingent on an unpinned cron-evaluation rule. Untested by construction: the harness's `cron_window` takes **precomputed boundary instants** ("cron parsing and tz evaluation happen upstream"), so the 23/23 suite exercises none of the tz semantics §7 specifies — including its own worked example.

## RT-12 (should-fix) — normalization is version-unpinned, and the harness validates the wrong fold

**Spec text at fault.** §5.1: stage 1 strips categories including "**Cn (unassigned)**"; stage 3 is the "Unicode **simple, locale-independent (ordinal) fold**." §10: "replaying the same event log (same timestamps, same per-stream order, same batch punctuations) reproduces byte-identical outputs" — Unicode table version is not part of the contract.

**Counterexample (E8, reproduced).** (a) *Version drift:* U+0378 is Cn today → stripped and flagged; every Unicode release assigns new code points, so a prompt containing a newly assigned character matches differently (and flips `text_invisibles`) on an engine built against newer tables. An auditor replaying last year's event log on this year's build gets different outputs; §10's determinism claim is false across builds and the spec never scopes it. (b) *Fold divergence:* the executor's `normalize()` uses `str.casefold` — the **full** fold (its own comment says "stand-in") — under which `substring("STRASSE" contains "straße", insensitive) = True` (ß→ss). Under the specced **simple** fold, ß stays ß and the match is **False**. A detection outcome flips between the normative text and the harness that "validates" it; whichever is intended, the 23/23 green proves the other.

## RT-13 (should-fix) — forensic text identity and the `text_invisibles` flag are underdefined; lazy flagging breaks purity

**Spec text at fault.** §6.6: evidence chain = "the ordered list of captured events" (so: original attributes — but never said against §5's "all text operators evaluate over the normalization pipeline"). §5.1: "stripped from the match text and flagged (per-event metadata `text_invisibles=true` …). Rules can test the flag." §3: "Filters and projections are **pure**: no side effects … Filter positions … are evaluated in written order."

**Counterexample.** SOC analyst disputes a kill switch triggered by keyword-set match on a prompt containing zero-width characters: the alert's evidence chain carries events, but the spec never states (a) that the *original* text is what the chain exposes, (b) what the *normalized* text that actually matched was, or (c) which normalization (version, stages, raw-vs-default per operator) produced the hit — the replaying auditor cannot reconstruct the match from the output. Sharper contradiction: when is `text_invisibles` set? If set as a side effect of evaluating a text operator, then `[prompt contains_i 'ignore' and text_invisibles]` and `[text_invisibles and prompt contains_i 'ignore']` differ (flag not yet set in the second, evaluated first per §3's written order) — a filter with side effects, contradicting §3's purity. If set at ingestion, over *which* fields of the event (all strings? only matched ones?) is unstated. No reading is given in the text.

## RT-14 (should-fix) — null-strict predicates inside absence steps flip the failure direction to false kills

**Spec text at fault (§1.1):** "Any comparison … or text operator with a null operand evaluates **false**", rationale "`x <> 5` silently excluding nulls is a **missed-detection** bug"; the only guard is "Deploy-time lint flags `!=` over attributes that are nullable per schema." §6.4: a B defeats the absence only if its predicate holds.

**Counterexample.** Kill switch: `not HumanApprovals[sessionId == e1.sessionId] for 5 min`. Timeline: read 10:00, egress 10:01, approval 10:02 with `sessionId = null` — precisely the telemetry drift §1.1 says must be tolerated. Predicate `null == e1.sessionId` → false → the approval does **not** qualify → absence holds → agent killed at 10:05 despite an approval event existing. Inside a negated step, null-strictness produces *false* detections, inverting the ratified rationale (safe-for-positive-match ≠ safe-for-absence); the engine's flagship construct sits in the inverted case. The decision itself stands (ratified); the defect is that §1.1 documents only the `!=` asymmetry and the lint covers only `!=` — `==` on a nullable attribute inside an absence predicate is the same hazard class, unflagged and unmentioned.

## RT-15 (should-fix) — expression-error blast radius: one rule's error censors the event for all rules

**Spec text at fault.** §3: "Expression runtime errors (failed cast, division by zero) make the *event* erroneous → **stream** error policy (§10)." §10: "an erroneous event … is **dropped**, logged with cause, and counted."

**Counterexample.** Two rules read `HumanApprovals`. R1 (unrelated hygiene rule) filters `[100 / riskScore > 2]`; R2 is the kill switch. Approval arrives with `riskScore = 0`: R1's evaluation divides by zero → per §3 *the event* becomes erroneous → per §10 it is dropped. Timeline: read 10:00, egress 10:01, that approval 10:02 → R2 never sees it → false kill switch at 10:05. As written, the error scope is the *stream* (the policy is declared per stream and the event "is dropped"), so deploying any rule with a fallible expression degrades every other rule's correctness on that stream — cross-rule interference the isolation story (§9) never admits. If the intent is per-rule-evaluation scope, the text says otherwise; either way the scope of "erroneous event" is undefined where it decides detections.

## RT-16 (should-fix) — rate limiting on unpartitioned queries: key undefined, §7's ban unstated

**Spec text at fault.** §8: "Rate limiting `first every D` (**per rule, per partition key**, event time)." §7: "Every stateful construct is partitioned — … outside one it is a deploy error unless explicitly marked global (**reserved**)."

**Counterexample.** Rule shape: `from S[severity == 'high'] select … output first every 10 min insert into T` — a stateless filter query, legal outside a partition block, with rate limiting attached (the roadmap lists `output first every T` as base FA1 surface with no partition caveat). The rate limiter must remember `t_out` per key — it *is* keyed state — but no partition key exists here. Either (a) it falls under §7's rule → deploy error (global is reserved in MVP) → the natural "suppress this noisy alert" use is inexpressible, which nothing states; or (b) it runs globally keyed → contradicts §7's blanket sentence and §8's "per partition key." §8 and §11 never resolve which; the §8 table's "— modifies any append producer" suggests (b) silently.

## RT-17 (should-fix) — sort-window ties: eviction unspecified; equal values churn the changelog

**Spec text at fault (§7):** "arrival inserts, overflow evicts the extreme; an arrival that would itself be the evicted extreme produces **no** change rows." No tie rule; with equal `attr` values both clauses describe the same arrival.

**Counterexample (E3, reproduced).** `sort(2, score, desc)`, arrivals score 5,5,5,5 (ts 10:00…10:03). The executor evicts the *oldest-inserted* tied row: output `+5₀ +5₁ +5₂ −5₀ +5₃ −5₁` — six change rows and unbounded churn for a stream of identical values. The equally textual reading "the arrival *is* (an) evicted extreme → no change rows" yields `+5₀ +5₁` and silence. Changelog consumers see entirely different retraction streams; which row evicts is also what decides §10's cross-implementation reproducibility and the DSL/Go-builder parity test surface. The spec text supports both.

## RT-18 (should-fix) — conjunct absence completed by a later positive event: legality and output timestamp ambiguous

**Spec text at fault.** §6.4: "(and the `and not B[pred] for D` conjunct form)… The emitted match's end timestamp is `t_first + D`." §6.6: "The output timestamp is the completing event's time (absence-completed matches: `t_anchor + D`)."

**Counterexample (E7, reproduced).** `read → egress and not Approval for 3 min, within 5 min`. Read 10:00, no approval by 10:03 (absence satisfied), egress 10:04 (inside `within`, after the absence deadline). Three questions the text can't answer: (a) is this a match at all (must the positive conjunct fall inside the absence window)? (b) if yes, is it "absence-completed" (output ts 10:03) or completed by the egress (output ts 10:04)? The two §6.6 clauses pick different answers, and the choice moves the §8 rate-limit anchor (`t_out`), i.e. which subsequent alerts are suppressed. (c) The executor answers (a) yes and (b) 10:03 — emitting `match_end = 10:03` **earlier than the captured egress at 10:04**, an evidence chain whose "end" precedes its own last event, released behind the watermark (see RT-09). None of this behavior is derivable from the spec text.

## RT-19 (should-fix) — `${VAR}` substitution mechanism undefined; textual reading breaks the Tweaker contract

**Spec text at fault (§10):** "substituted at deploy time, **before type-checking**, from the deployment environment … Values are typed by context (a `${LIMIT}` in a duration slot must parse as a duration literal)." Whether substitution is textual (into source) or token/value-level (into a single literal slot) is unstated — "before type-checking" permits before-*parsing*.

**Counterexample.** DESIGN.md defines the Tweaker as "changing parameters only — time bounds, keyword lists, thresholds, destinations — **never structure**." Rule: `… within ${BOUND}` with deployment value `BOUND = "5 min select * insert into Exfil"` (or a crafted keyword-list value closing a bracket). Under textual substitution the deployment *environment* rewrites rule structure — a reviewed, signed-off rule text no longer determines what runs, and the audience contract is void; "must parse as a duration literal" is only checkable if the slot boundary survives substitution, which is exactly the unstated part. Under token-level substitution the example is a deploy error. The spec must say which; security review of rule text is meaningless under one of the two readings.

## RT-20 (later) — tie-break "stream id" never assigned; partial-start ties unbroken

**Spec text at fault.** §2.2: "Total order for ties: `(event_time, stream id, per-stream ingestion sequence)`" — no rule anywhere for what a stream's id *is* (declaration order? name collation?). §6.4 makes detection hang on it: "a B tied at `t_first` counts iff ordered after the anchor" — so an approval at exactly the read's timestamp defeats the absence or not depending on an undefined id ordering; renaming a stream could flip a detection. §6.5's "earliest-started partial wins" is also unbroken for two partials with equal `first_ts` (two A's at one instant under for-each); the executor falls back to creation sequence (`(q.first_ts, q.seq)`), which the spec never licenses.

## RT-21 (later) — batch window output undefined; duplicate batch ids double-fire

**Spec text at fault (§7/§8):** batch "groups events sharing one source delivery batch … fires at the batch boundary", output mode append. Never stated: *what* it emits (the group? per-event?) and with **what event timestamp** — yet §2 makes a clock clause mandatory on everything downstream, so a batch-fed construct is unspecifiable. §10 does put punctuations in the replay contract (credit), but "arrival-structured" output entering the event-time world has no defined lateness/order. Also: a source that redelivers or interleaves batch ids (retry per §10's at-least-once posture) re-opens a seen id — the executor fires the same batch id twice (`batch_window` splits on every id *change*); dedup/contiguity is nowhere required of sources.

## RT-22 (later) — term-match tokenization: "alphanumeric" undefined over Unicode

**Spec text at fault (§5.2):** "tokenized into **maximal alphanumeric sequences** (post-normalization)." Unicode has no single "alphanumeric": is U+00B2 (², category No) alphanumeric? U+005F (_)? The executor inherits Python's `str.isalnum()` (true for ², Nl, Nd, No…), so `term_match("curl²", "curl")` is false there but true under an ASCII-alnum or L*/Nd reading. Term-match results — the operator §5.2 designs specifically against silent-failure traps — are implementation-defined at the boundary, and the deploy-time "operand is a single token" check moves with the same definition.

---

## Count by severity

| Severity | Count | IDs |
|---|---|---|
| **blocker** | 10 | RT-01 … RT-10 |
| **should-fix** | 9 | RT-11 … RT-19 |
| **later** | 3 | RT-20 … RT-22 |
| total | 22 | |

Experiments: 8/8 reproduced (`E1–E8`, [experiments/exp_redteam.py](experiments/exp_redteam.py)); harness re-run 23/23 green before review. Recurring meta-observation, evidenced inside RT-01, RT-05, RT-07, RT-08, RT-11, RT-12: the "validated 23/23" status covers single-stream, single-construct slices only — multi-stream watermark min/idleness, cron tz evaluation, TTL×rate-limit composition, and the specced case fold have no executable coverage, and in RT-03/RT-04/RT-05/RT-07 the harness pins behavior the normative text contradicts or omits.

---

## Dispositions (coordinator, 2026-10-31) — per ground rule 16

All 22 findings verified first-hand (E1–E8 reproduced against the untouched harness before any fix). **All 22: FIXED in SEMANTICS.md on 2026-10-31**; none rejected, none forwarded unfixed. Harness regressions (RT-labeled checks) rebuilt to the amended spec — see `checks/` status in DESIGN.md.

| Finding | Fix (section) |
|---|---|
| RT-01 | §2.1 rewritten: playback virtual clock over the merged log (empty stream never stalls); construct-level clamped-monotonic min; construct-late drops counted; guarantee honestly re-scoped |
| RT-02 | `idle after` added to §11.10's mandatory-clause list |
| RT-03 | §6.4: absence monitored continuously from match start — defeat can precede later positive steps (spec text won; harness corrected) |
| RT-04 | §6.5 definition rewritten to true skip-past-last ("resumes after the match's last event; partials started at or before it are discarded"), matching the worked example |
| RT-05 | §7 model is normative (single delayed fire, append); §8 cron row rewritten to agree; re-fire/changelog variants explicitly P2 |
| RT-06 | §9: purge of a changelog window emits retractions first; window extents defined per kind (delay=D; cron=longest bucket over 10-yr horizon; sort/batch exempt) |
| RT-07 | §9: purge destroys all per-key state incl. rate-limit anchors and at-most-once marks; instance lifetime defined (re-arm after purge is normative); §11.3 check extended to rate-limit intervals (TTL ≥ D) |
| RT-08 | §9: partition clause must key every consumed stream (deploy error otherwise); flagship schema amended (HumanApprovals carries agentId) in 02-roadmap.md |
| RT-09 | §2.1: derived streams carry propagated watermark punctuations (forwarded after due emissions); re-declaring clauses on derived streams is a deploy error (§11.11) |
| RT-10 | §6.4: normative watermark-advance ordering — events, then completions/timers, then discards; equal-bound rules fire |
| RT-11 | §7 cron DST rule: ambiguous local times fire at first occurrence; nonexistent times at first instant after the gap |
| RT-12 | §5.1: Unicode tables pinned per engine release, recorded at deploy; §10 determinism scoped to engine release; simple fold confirmed normative (harness switched) |
| RT-13 | §5.1: `text_invisibles` computed once at ingestion over all string attributes (purity preserved); §6.6: evidence carries original text; auditor reproducibility via pinned tables |
| RT-14 | §1.1 + §11.12: nullable-attribute comparisons inside absence predicates are deploy errors without explicit null handling |
| RT-15 | §10: expression errors scope to the rule's evaluation only, never the stream |
| RT-16 | §8: unpartitioned rate limit = one global anchor per rule, explicitly exempt from §7 (bounded state) |
| RT-17 | §7: tie eviction = latest row in §2.2 order; tied newcomer emits nothing |
| RT-18 | §6.4 completion rule: match_end = max(last positive ts, absence deadlines); conjunct legality defined |
| RT-19 | §10: `${VAR}` is token-level substitution into a parsed literal slot; structural injection impossible |
| RT-20 | §2.2: stream id = declaration order; §6.5: partial-start ties broken by §2.2 order then spawn sequence |
| RT-21 | §7: batch emits one group event stamped max event time; fired ids deduplicated |
| RT-22 | §5.2: alphanumeric = Unicode L* + Nd exactly |

Meta-finding (coverage gaps of the 23/23 suite) accepted: the rebuilt harness adds multi-stream/idleness, TTL×rate-limit, fold, and RT-labeled regression checks.
