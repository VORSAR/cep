# S3.3-2 design note v1 — the breaker's report

**This sprint:** [design](../sprints/s3-3-2-design.md) · [record](../sprints/s3-3-2-record.md) · reviews: **design: breaker** · [design: challenger](s3-3-2-design-challenger.md) · [design: architecture](s3-3-2-design-architecture.md) · [design: QA (testability)](s3-3-2-design-qa.md)

*The agent's own write of this report was refused by the harness; the coordinator saved its final message verbatim.*

**Reviewed:** `docs/sprints/s3-3-2-design.md` (v1), read-only checkout `/Users/ramith/code/cep-wt/s3-3-2-panel` (detached at `0a3114c`).
**Scratch:** a `git archive` copy at `/private/tmp/claude-501/-Users-ramith-code-cep/14b3d567-7a0b-4919-ab4a-e8203f9bb87a/scratchpad/panel/breaker/repo`: `internal/playback/zz_breaker_test.go` (v1 §5.1 and §5.3 rebuilt on the real `parser`, `validate.Check`, `plan.Build` and `NewPlanDriver`: the owning rule alone, tables first, lists merged by `Ts` with ties to the list written first, `Driver.End()`, compared by number, order and named attributes; `TestBreakerCorpus` runs the 15 corpus blocks); `zz_breaker2_test.go`, `zz_breaker3_test.go` (adversarial programs A1–A12); `zz_breaker4_test.go` with `testdata_breaker_10_reordered.cep` (an ordering-change plant); `internal/runtime/zz_breaker_close_test.go` (an intake item queued while the router is held behind a blocked sink, under `-race`). Outputs in `…/panel/breaker/{corpus,adv,more,order,close}.out`. **Tools:** pinned go1.27.3 (`GOTOOLCHAIN=local`), `go test -v` and `-race`, grep, sed; every exit code read on its own line; every run exited 0, the verdicts are `t.Logf` lines.

**Counts: 2 blockers · 7 should-fix · 3 later.**

---

### B1 (blocker) — a block can omit or null the time attribute; live refuses that event, the sandbox stamps it at 1970, and the block passes

**What breaks.** X4's rationale says "the time attribute and every primary-key column are non-optional" (design line 85). True for key columns (E-TBL-002 refuses an optional key, run A3); not for the time attribute: the validator checks only its type (`internal/validate/graph.go:117`, E-TIME-003), so `stream S (id string, ts timestamp optional) time ts …` passes `Check`. X4 refuses null only for a "non-optional attribute", so `ts: null` or an omitted `ts` gets through, and §4 builds `GivenEvent.Ts = 0`. Both live paths refuse this event whatever `optional` says: `internal/event/decode.go:135-147` (`if a.Optional && !isTime {…Null}`, else `DecodeMissing`/`DecodeNull`) and `row.go:92` ("a null time attribute is refused, as the decoder refuses it").
**Evidence (A2b).** `given S [ {id: 'b'} ]` with `then Out [ {id: 'b', ts: timestamp '1970-01-01T00:00:00Z'} ]` → `R block 1: PASS; late=0; outs=1: ts=0[b T0 ]` — a false pass on an output live can never produce. In A2 the null event comes second and is dropped silently as stream-late (`late=1`); the block fails with "output 2 is missing".
**Fix (§3, X4):**
> X4 | `null`, written or by omission, for the stream's **time attribute, whether or not it is declared `optional`** (live ingest refuses it: `internal/event/decode.go:135-147`, `row.go:92`), or for any other non-optional attribute or column of a `given` record | `ValuePos` when written; the record's `Pos` when omitted | proposed (Q2)

§4: "`Ts` is never null: X4 refuses a null or missing time attribute." Add `testdata/e-exp-002-null-time.cep`. (An owner option, not part of this fix: refuse `optional` on a time attribute in E-TIME-003 — a SPEC change.)

### B2 (blocker) — Close's barrier waits forever behind a router held by a blocked sink

**What breaks.** §9.1 step 3 (design line 260) bounds only queuing the barrier by ctx; not waiting for its answer. (1) `intake` is buffered (`runtime.go:373`), so the barrier queues at once. (2) While a send is pending the router reads no intake: `runtime.go:805-829` selects only on mailbox-or-sink, `handoff` and `stopped`; intake is read only at `runtime.go:832` (`internal/runtime/saturation_test.go:81`: "the router (rightly) reads no intake while its sink send is held"). (3) `requestQueued` (`lifecycle.go:60-73`) awaits "whatever ctx does", unblocked only by `r.stopped`, which Close itself closes later (`lifecycle.go:343`); the sink's context is cancelled only by Remove or after `stopped`. So with a sink blocked in `Write` when Close starts, Close never returns — breaking its contract (`lifecycle.go:320-322`, "Past ctx's deadline … returns ctx.Err()"), which today's Close keeps.
**Evidence (`-race`, exit 0).** Sink held, its queue full, an `Add` with a 300 ms ctx queued: `Add has NOT returned 2s after start, 1.7s past its 300ms deadline`; `after the sink was freed, Add returned after 2.001964416s: <nil>`. An intake-order barrier behaves the same, and inside Close nobody frees the sink.
**Fix (§9.1 step 3):**
> `Close` queues `iBarrier` with `enqueue(ctx, …)` and waits with `select { case <-reply: case <-ctx.Done(): }`, never through `requestQueued`. On ctx's end at either point, Close takes `ctx.Err()` as its result, skips the walk, and goes straight to `close(r.stopped)` and the per-app `cancelSrc`/`onRemoved` loop (`lifecycle.go:343-352`).

Test: `TestCloseReturnsAtDeadlineBehindABlockedSink`.

---

### S1 — X4 refuses a block that omits `text_invisibles`; a block that names it is silently ignored
SPEC §4.2 (`SPEC.md:77`): `text_invisibles` is engine-owned (a payload value "ignored, overwritten and counted"; declared only as a non-optional bool). The decoder never needs it (`decode.go:132`, `if i == d.reserved { return Value{Kind: TBool}, nil }`); ingest recomputes it (`runtime/ingest.go:63`). X4 would require every given record on such a stream to name it — refusing what SPEC allows — and naming it has no effect: A12 (omitted) `PASS` in the sandbox (the refusal would be X4's alone); A11 (`text_invisibles: true`, filter `[text_invisibles]`) `FAIL: output 1 is missing`; A11b `flag is …B:false, expected …B:true`.
**Fix (§3):** X4 "except `text_invisibles`, which the engine computes (SPEC §4.2)"; new X9: a `given` record names `text_invisibles`, at the field's `Pos`, `…: text_invisibles is computed by the engine; a block cannot set it`.

### S2 — "missing output" hides why the output is missing
§5.3's only message, `output <i> is missing …; the rule emitted <k>`, reads the same for a given event dropped as stream-late (§5.1 step 4), an expression error, a key error or a panic: A5 `PASS; late=1` (vacuous); A5b `FAIL: output 3 is missing; the rule emitted 2; late=1`; A7 (`1e308 * 10.0`, an expression error) only `FAIL: output 1 is missing`.
**Fix (§5):** `BlockResult` gains `Late, Errored, KeyErrors, Panics int64` from `Driver.Stats()` after `End`; §5.3: when any is non-zero the divergence ends with `; <n> given event(s) were stream-late and dropped` / `; <n> dropped by an expression error` / `; <n> key error(s)` / `; <n> panic(s)`.

### S3 — the barrier is answered before earlier installs complete; `Start` in the window is not reliably `ErrClosed`
1. The router answers intake right after `router.add` (`lifecycle.go:381`) publishes the app; the shards' acks complete the install later. So Close can `Remove` an app whose install is pending (`a.ing` set in `newAppState`, `runtime.go:410`): `remove` sets `removing` and clears the routes (`lifecycle.go:408-410`), then the add's `installAck` runs `a.installRoutes()` (`lifecycle.go:463`), re-installing routes on the app being removed, and replies success to `Add`.
2. `Activate` checks `appOf` and the phase before reaching the router (`lifecycle.go:84-95`, `runtime.go:528`): a `Start` in the window returns `ErrRemoved`, and `ErrClosed` only after `stopped`; QA's planned test (design line 262) would depend on timing.
3. `Add`'s caller-side check reads `stopped` (`lifecycle.go:47`), not `closing`.
**Fix (§9.1):** step 2 — `Add` and `Activate` check `closing.Load()` caller-side first and return `ErrClosed` without queuing (the router keeps its check for the race); step 3 — the router answers `iBarrier` only when `rt.installs` holds no pending add, else parks the reply and sends it from `installAck`; `installAck` skips `installRoutes` when `a.removing` is set. Test: `TestCloseBarrierWaitsForAPendingInstall`.

### S4 — `spec-rev` records any value drift once SPEC's rev has moved, for any reason
§8 (design line 245) records `spec-rev` whenever "SPEC's rev is above the baseline's `spec_rev`". The rev moves for unrelated reasons (revs 7–10 within days, `SPEC.md:276-283`); after any bump the first regen records, exit 0, every drift in an attribute no then-record names — what Tier 2 is for (qa-strategy §3:43).
**Fix (§8, `spec-rev`):** "**and the run names the block** (`make regen-baseline ACCEPT=spec-rev:<file>:<rule>:<block>`)"; an unnamed block of this class is refused as `value-change`; step 4 prints the current rev's change-log line. Pytest: a rev bump with an unnamed value change is refused.

### S5 — blocks keyed by ordinal make an insert, delete or rename refuse unrelated blocks
Key `(file, rule, 1-based ordinal)` (design lines 229, 236): inserting a block shifts every later ordinal, each later block compared with its predecessor's outputs and refused; a renamed rule is "gone" (refused) + "new". Q4 covers only removal.
**Fix (§8):** key `(file, rule, hash of the block's canonical text)`, ordinal for display; a gone key and a new key in the same rule with identical outputs are classed `moved` and recorded; any other gone key stays refused (Q4).

### S6 — the `conformance` row can pass with no block run
`gotest.py:4-5` counts passed tests against `min_wants`; v1 sets 1 (design line 221). `TestCorpus` passes with zero sub-tests (a wrong path, no `docs/` in the checkout), or with every file in `notBuilt` — the silent-pass shape of the owner's lasting rule (decisions.md, 2026-11-08 S3.3-1 close-out, item 3).
**Fix (§7):** `TestCorpus` fails when it reads fewer than `wantFiles = 12` files (10 examples + 2 torture, counted) or the deployed files run fewer than `wantBlocks = 15` blocks; every corpus change moves both; `min_wants` = `1 + wantFiles`. Proof: an empty examples directory in a world turns the row red.

### S7 — exit 6's ordering and timestamp refusals are proven only on synthetic files, though a corpus edit reaches both
v1: "A count or order change cannot reach the classifier through the engine alone" (design line 251). A corpus edit of the same kind as its own count proof does: swapping the first two given events and the first two then-records of 10's block 3 → `AdminScopeGrant block 3: PASS; late=0; outs=3: [t1 bob 10:01] [t1 alice 10:00] [t2 alice 10:03]` — Tier 1 green, the same multiset in a new order. (The same swap in 07's block 5 becomes a late drop, `late=1`: the plant must use 10.)
**Fix (§8, Proofs):** in the world this plant is refused as `ordering-change`; moving one given `ts` and its then-record `ts` by 1 ms is refused as `timestamp-drift`; after each, the baseline unchanged byte for byte; the `proofs` `min_wants` raised to match.

---

### L1 — −0.0 equals +0.0 in a block but encodes differently in the baseline
§5.3 compares floats with `==`, so A6 passes a then-record of `-0.0` (the output `-0`, signbit set); `encode.go:44` writes `-0` and `0` differently, so Tier 2 sees a change Tier 1 does not. **Fix (§5.3):** float equality is `==` and the same `math.Signbit`.

### L2 — vacuous blocks pass
`given S [ ] then Out [ ]` passes (A8), as does a block whose only given events are dropped as late (A5). SPEC §6.8: rules in packs must carry a block; these satisfy that without testing anything. **Fix (§3):** propose a lint, or an X10, for a block with no given event; the owner places it.

### L3 — two cases the runner cannot express
A file refused only for an unbuilt connector fails at bind with an error that is not `ErrNotBuilt` (S3.3-1's design decision 5), so it can be neither listed nor deployed; one unbuilt rule fails `plan.Build` for the whole file (`plan.go:92-97`), so its built rules' blocks never run. No current built file is affected (07, 09, 10 have no annotations). **Fix (§7):** record both as known limits until S4.1 and S5.1; map a not-built connector to `cep.ErrNotBuilt`.

---

### Blocks worked by hand
From SEMANTICS §8 and §9 as annotated (a plain rule emits on arrival; per key, an output is suppressed when `ts ≤ anchor + D`, the end included; a null key part is a key error; the watermark is max ts − d), then run through v1's algorithm on the real engine (`TestBreakerCorpus`, exit 0). **All 15 agree; `late=0` in every block.**

| File | Block | What it shows | v1's verdict | Engine outputs |
|---|---|---|---|---|
| 07 | 1 | `rogue` not in the table → fires | pass | 1: t1 rogue 10:00 |
| 07 | 2 | `crm` in the table → nothing | pass | 0 |
| 07 | 3 | `coalesce(null)` → 'missing' fires | pass | 1: t1 missing 10:00 |
| 07 | 4 | per tenant: 10:04, 10:05 suppressed; 10:05:00.001 fires | pass | 2 |
| 07 | 5 | t2 its own key; t1 10:03 suppressed | pass | 2: t1 10:00, t2 10:02 |
| 07 | 6 | app-b 10:00:30 fires; app-a 10:00 not late (wm 09:59:30), suppressed | pass | 1: app-b |
| 09 | 1 | 0.9 > 0.75; `round` → 1.0 | pass | 1 |
| 09 | 2 | 1 h 30 min: 00:30, 01:30:01 suppressed; 01:30:01.001 fires | pass | 2 |
| 09 | 3 | 0.75 is not > 0.75 | pass | 0 |
| 09 | 4 | staging filtered; prod fires | pass | 1: 00:10 |
| 10 | 1 | fire path | pass | 1 |
| 10 | 2 | `read` filtered | pass | 0 |
| 10 | 3 | alice, bob, (alice 10:02 suppressed), t2-alice | pass | 3 |
| 10 | 4 | null `userId` → key error | pass | 0 |
| 10 | 5 | 10:05:00.000 suppressed; 10:05:00.001 fires | pass | 2 |

**Checked and holding:** `plan.Rules` lines up index by index with `model.Rules` (`plan.go:92-97`); the output `ts` is `e.Ts` at the target attribute named `ts` (`plan.go:207-209, 432-434`); pre-1970 and year-0001 events admitted (A1, A1b); an optional key refused by E-TBL-002 (A3); a given on a derived stream cannot arise (E-GRAPH-004 refuses the rule first, A4); `MarshalCanonical` skips `Pos` fields (`model.go:447`); without X6 a duplicate field's last value wins (A9 passed with output `b`), which supports X6.
