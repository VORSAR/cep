# S3.2b-2 — sprint record (S3.1's routine)

**Sprint:** S3.2b-2 — arrival time, the recorder, the ingest flag, error semantics, the gate's leak, tripwire and two-OS proofs, multi-part partition keys and D3's position (S3.2b's items 3–5, 7, 8 and the owner's additions) · **Routine:** S3.1's, by the owner (2026-11-06): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it · **Size:** about 12.5 SP · **Branch:** `s3-2b-2/integration`
**What to build and how each item is proven:** [sprint plan — S3.2b](sprint-plan.md#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time), its "how it runs" note and S3.2b-2's plan-check readings · **Process:** [00-sprint-model.md](00-sprint-model.md) · **Before:** [S3.2b-1](s3-2b-1-record.md)

## Plan and decisions

- **The plan check (owner, 2026-11-06):** three answers, all as recommended ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-plan-check)):
  - **Q1:** a recording keeps a rejected live event as a "rejected" line; replay counts it and skips it, so a recording's replay gives the live alerts and the live error counts;
  - **Q2:** when the recording cannot be written, detection goes on; the recording stops there, and the failure is counted and reported;
  - **Q3:** grouped partition keys have one spelling: `k of S` alone, `(k, j, …) of S` for two or more different attributes, the same number of parts on every stream; anything else is a deploy error showing the valid form.
- **Binding from S3.2b's plan check** ([decisions](decisions.md#owner-decisions-2026-11-06--s32bs-plan-check), Q3–Q7):
  - **Q3:** `text_invisibles` is readable on every stream; a declaration other than a non-optional `bool` is E-TYPE-001 at the declaration (SPEC change log, an invalid example);
  - **Q4:** exit 8's zero allocations cover every per-event stage on the one-thread replay path; the live router-to-shard hand-off gets a measured per-event budget as a tripwire, and ADR-0010 §1 a dated note;
  - **Q5:** the second OS is macOS;
  - **Q6:** `go.uber.org/goleak` v1.3.0 enters `go.mod`, test-only; its own test graph enters `tools/deps-allowlist.json` (testify v1.8.0, go-spew v1.1.1, go-difflib v1.0.0, yaml.v3 v3.0.1);
  - **Q7:** advisory ceilings — partial-count `none` until S4.1, sink-depth 1,024, drained_pending 0; the outbox ceiling blocking at 65,536.
- **Binding from S3.2b-1's review and break-it** ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-1s-review-and-break-it)):
  - **multi-part partition keys** (≠ R): each combination of parts is its own key (its own rate limit, state and expiry); a null in any part drops the event for that rule, counted; parts may differ in type, matched position by position across streams; no limit on the number of parts (≠ R) — a single-part key stays allocation-free, a multi-part key may allocate; E-PART-004 stays for a stream named twice and its message points to `(k, j) of S`;
  - **D3's error at the `ts` attribute itself** (≠ R): model attributes gain a source position, and the stored model goldens are regenerated.
- **In this sprint:**
  - **Item 3 — `time arrival`:** live events stamped at ingest; replay of the recorded stamp; **the recorder** (S3.2a-1's D3) writes the start, each event with its arrival, every idle tick, table writes and the stop, plus Q1's rejected line, under Q2's failure rule.
  - **Item 4 — `text_invisibles` engine-owned:** always computed in the shared ingest step; a payload's own value ignored, overwritten and counted; the expression node reading the flag; Q3's validator rule.
  - **Item 5 — error semantics** (SEMANTICS §10): per-stream counts of rejected payloads (a missing non-optional attribute, a wrong type, a null timestamp), unknown attributes and overridden flags, through an internal payload intake; per-rule expression errors and null keys as built in S3.2b-1.
  - **Item 7:** goleak and its planted-leak proof; the outbox high-water mark and the T4 metric-tripwire step with its ceilings and a planted proof; `allocgate` ingest → filter, the hand-off budget, S3.1-2's decode budget (F12) and ADR-0010 §1's note.
  - **Item 8:** the two-OS determinism proof on macOS, with a planted map-iteration nondeterminism.
  - **Multi-part partition keys** with the language-design skill's pass first, and **D3's position**.
- **The design (G1, 2026-11-06):** [s3-2b-2-design.md](s3-2b-2-design.md) v2.
  - **v1's review** ([s3-2b-2-design-architecture.md](../reviews/s3-2b-2-design-architecture.md)) found 2 blockers, 10 should-fix and 8 later, each behaviour claim proven by a scratch program (the coordinator re-ran F1, F3–F7 on a fresh copy: all reproduce):
    - F1: typed `Send` accepted five kinds of value a recording cannot copy, so replay differed from live or stopped;
    - F2: a recorder writing on the router goroutine could stall intake for every app.
  - **QA's tests first** (`s3-2b-2/qa`, 7 commits: 14 Go tests and 7 counter-files red on the stub, a Python harness check shown red by narrowing its key, example 10; every other test green but the five corpus tests example 10 reaches) raised gaps G1–G13.
  - **The language evaluation** ([rev5-grouped-keys.md](../language/eval/rev5-grouped-keys.md)): one spec-only generator, five held-out tasks: parse 5/5, static 5/5, semantic 5/5.
  - **The owner's decisions** ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-design-after-the-architecture-review)): **D1 (≠ R)** the Go entry points refuse every value a recording cannot copy exactly; **D2** a recording that falls behind stops, never delaying detection, and a product setting turns recording off; **D3 (≠ R)** a recording keeps each payload as sent; **D4 (≠ R)** recording is off unless turned on (forwarded to S3.3's `Deploy`); **D5** one sprint, about 12.5 points.
  - **Annotated:** SPEC rev 5 (§0, §4.4, §5.8, Appendices C and E), grammar.lark and .ebnf, errors.md, SEMANTICS §9 and §10, SYNTAX, DESIGN's ledger and log; ADR-0005 §3, ADR-0006 §5, ADR-0015 §3.11; the S3.3 and S4.1 cards; known issue S2.2b-1.
- **Exits** (S3.2b's lines 2, 3, 4, 6, 7, 8; 9–11 are the owner's additions, written at G0):
  2. Two rules on one stream: one divides by an attribute that is 0 on one event; the other still fires on that event; per-rule `errored_events` counts it; a null key, a missing non-optional attribute, an unknown attribute and a failed `cast` are each counted where SEMANTICS §10 says — the error-semantics table test.
  3. A payload without `text_invisibles` gets the computed flag; a payload saying `false` over text containing a zero-width space yields `true` and increments the override counter — the ingest-flag test.
  4. A `time arrival` stream: live events are stamped at ingest in arrival order; replaying the recorded log reproduces the output byte for byte — the arrival-time test.
  6. A planted data race, a planted goroutine leak, and a single `Advance` emitting past the outbox ceiling are each red — three proof rows.
  7. The two-OS byte diff is green and catches a planted map-iteration nondeterminism — the T4 proof row.
  8. Zero allocations per event from ingest through filter evaluation — `allocgate`.
  9. Multi-part keys: events equal in one part and different in another are different keys (each with its own rate limit); a null in any part is a counted key error; parts of different types match by position across streams; the same log live and in replay gives identical per-key output — the plan and runtime tests, the live/replay equivalence test over multi-part keys, and a harness check in the Python reference executor.
  10. Each wrong spelling of a grouped key (Q3), a part-count mismatch and a type mismatch at one position gives its code at its position with a message showing the valid form; D3's E-TYPE-001 points at the `ts` attribute — `validate`'s static-error table and the invalid corpus.
  11. A live run with a rejected payload, recorded and replayed, gives the same outputs and the same counters (Q1); a recording writer that fails leaves detection running and counts the failure (Q2) — the recorder tests.

## Scope changes

- **2026-11-06 (owner, plan check):** Q1's rejected-line record kind (+0.5). Re-estimated item by item at the plan check: item 7 +0.5 (the tripwire step is a new gate instrument, not only a test), Q3 of S3.2b's validator rule +0.5. About 9.5 → about 11 points, under the cap; nothing moved out.
- **2026-11-06 (owner, design):** D1's refusals at the Go boundary (+0.5), D2's recorder goroutine and queue (+0.5), the review's fixes (the differential's key shapes, the sink high-water and the tripwire corpus, the in-path determinism plant, the projection buffer, the test migration: about +1.5); D3's position shrank (no golden changes: −0.75). About 11 → about 12.5 points, under the cap; the owner kept one sprint (D5); nothing moved out.

## Measured results

*(filled as the work lands)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S3.2b-2 — arrival, recorder, flag, errors, gate proofs, multi-part keys · routine: S3.1's (QA first + architecture review + break-it) · SP ~11 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2b's, v3; the owner's additions of 2026-11-06, exits 9–11 written at G0)
             ☑ dependencies merged, main green (S3.2b-1's PR #14; main f32fd41, the gate workflow green) ☑ fits the cap, no lanes
             ☑ plan check answered by the owner (2026-11-06) ☑ test material: QA's tests first, an item
Design (G1)  ☑ architecture review resolved (design v2 §15) ☑ QA testability (14 tests red first; G1–G13 placed) ☑ owner decisions recorded (D1–D5)
             ☑ ADRs, SEMANTICS and SPEC annotated ☑ language evaluation 5/5
Build (G2)   ☐ T2 (make check exit 0) ☐ T3 (exit 0) ☐ conformance cases for every construct changed ☐ tests on the real code
Review (G3)  ☐ architecture review of the combined diff ☐ QA break-it ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ T4 exit 0, run complete ☐ owner's check passed ☐ exits 2–4 and 6–11 met by their proofs
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration
```

## For the owner (the owner's check)

*(written from the sprint's diff before the PR)*

## Retro

*(at close-out)*
