# S3.1-1 — sprint record (S2.2b's routine, plus a clock break-it)

**Sprint:** S3.1-1 — the clock service and the metrics counters (S3.1's items 1–2; the owner split S3.1 at its card's line) · **Routine:** by the owner (2026-11-04): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it of the finished clock · **Size:** 6.5 SP of S3.1's 13 · **Branch:** `s3-1-1/integration`

## Plan and decisions

- The owner's plan-check answers: [decisions](decisions.md#owner-decisions-2026-11-04--s22b-closed-s31s-plan-check).
- The design: [s3-1-1-design.md](s3-1-1-design.md) v2. v1's review ([s3-1-1-design-architecture.md](../reviews/s3-1-1-design-architecture.md): 8 blockers, 7 should-fix, 1 later) showed ADR-0004, the reference executor and SEMANTICS disagreeing on the clock's order, its owner and quiet streams; the owner chose the most accurate option each time, had the choices' architectural impact reviewed ([s3-1-1-decisions-architecture.md](../reviews/s3-1-1-decisions-architecture.md)), and settled what that raised: decisions D1–D8, [decisions](decisions.md#owner-decisions-2026-11-04--s31-1s-clock-after-two-architecture-reviews).
- The oracle changed first: the reference executor walks forward in event time; six new harness checks, red first; 47 pass.
- QA's tests were written first against v1, then revised by QA to v2 (26 tests) before the implementation was moved in; the implementation was written apart from them.

## Done-done

```
Sprint S3.1-1 — the clock service · routine: lighter (QA first + architecture review + break-it) · SP ~8.5 (6.5 + D1–D8) · cap 2 weeks
Entry (G0)   ☑ card complete and approved ☑ dependencies merged, main green (S2.2b's merge) ☑ fits the cap, no lanes
             ☑ plan check answered by the owner (2026-11-04) ☑ test material exists (QA's tests, first; revised to v2)
Design (G1)  ☑ architecture findings resolved (design v2 §8; two reviews) ☑ owner decisions recorded (D1–D8, sprint-plan §5)
             ☑ ADR-0004/0006/0012/0013 and SEMANTICS annotated ☑ the oracle changed first (47 checks, 6 new, red first)
Build (G2)   ☑ T2 (make check exit 0) ☑ T3 (in T4) ☑ tests on the real code (QA's 26 pass at the first run)
Review (G3)  ☑ QA break-it of the clock (4 findings) ☑ every finding fixed, with its test
Verify (G4)  ☑ T4 exit 0 ☑ CI (PR #8) ☑ owner's check (merged) ☑ every acceptance line met by its named proof
Exit (G5)    ☑ one PR (#8), merged by the owner ☑ main CI green on the merge commit
             ☑ record + retro ☑ known issues ☑ status line ☑ documents reconciled ☑ memory updated
```

## Measured results

- **Exit 1 — the harness's arithmetic at clock level:** `harness_test.go`, each test naming its check (`2.1/6.5` admission strictness and the late drop; `2.1` firing at the watermark; the `2.1` clamped min and construct-late idle resume, with ticks; `2.1 · RT-01` the empty stream; `2.1` playback admission equal to live admission; `6.4 · RT-10`). All pass.
- **Exit 2 — the timer contract:** `timers_test.go` drives the walk through a test-double operator; its `kindTable` (kind, rank, strict or inclusive, when it fires, its callback) is the owner's check. Every timer kind fires in its order within an instant, discards only through `OnDiscard`, a cancelled discard never arrives, a pending completion is never discarded (RT-10).
- **Exit 3 — the quiet key:** `quietkey_test.go`: two keys of one rule; the silent key's absence fires at due 300000 on the clock reaching it, its TTL purge once the clock passes 600000.
- **The decisions' own tests:** one clock per rule (a late key is born at the rule's time), the seal (D7), the purge keeping the buffer (D1-1), granularity (one advance, 1 ms steps and an irregular split give the same walk), quiet floors (D3, D5, D6).
- **Break-it (G3):** a fresh QA agent found 4 findings (5 red tests): a freed key forgot its seal; keys walked in arrival order, not the oracle's birth order (first release, and again after a purge); an event at `MinInt64` was construct-late on a fresh key; a strict timer at `MaxInt64` never fired after `EndOfInput`. All fixed, the red tests now green. It also kept, as guards, 3,000 random scenarios (the walk is the same however the advances split; every event released once; no timer early) and 2,000 registry runs against a direct model of the design. One guard written against creation order follows the findings' birth order.
- **The oracle:** the reference executor changed first (D1–D8); 47 harness checks pass, 6 of them new and red first.
- **Coverage:** `internal/clock` 98.2%, `internal/metrics` 100%.
- **The gate:** T4 green on the final tree, the `semantics` row included (`make strict-gate`: every row PASS, exit 0, wall 86.9 s; `proofs` 14).

## For the owner

- **The clock's design changed substantially from ADR-0004 as accepted**, by your D1–D8; the ADRs and SEMANTICS carry dated annotations, and the contract freezes at S3.3 (`api/clock.golden`).
- **The semantics harness is now a gate row** (owner, 2026-11-04): `semantics` runs its 47 checks at T2–T4; a failing check fails the row naming it, and every declared check must report OK. Its proof plants a failing check into a real copy of the harness (proofs: 14).
- **Forwarded:** where a batch close sits in the walk (S8); the ticks' source, their passage through the intake and their stop before the sources at Stop (S3.2a, S4.2).

**Closed (G5), 2026-11-04.** PR #8 merged by the owner (14 commits); CI green on the PR and on the merge commit. Points: about 8.5 of S3.1's 13 (6.5 carded + D1–D8); duration: one day.

## Retro

- **Worked:** the design review before code found the clock's three sources of truth (ADR-0004, the reference executor, SEMANTICS) disagreeing; asking the owner the questions with small examples, options ordered by accuracy, turned eight structural decisions into clear choices. The oracle changed first, red first, so the engine had an executable target. QA's tests (written, then revised, apart from the build) passed at the first run; the break-it found what they could not (cross-key order, the int64 extremes).
- **Hurt:** the first round of questions was too technical; the owner asked twice for simpler wording with examples. My decision-3 wording ("wall clock minus the bound") would have lost a lagging source's events; the impact review caught it.
- **Keep:** ask in plain words, a tiny timeline per option, most accurate option first; an impact review of the owner's choices before building on them.
- **Lasting rule proposed (for the owner):** decision questions to the owner come as a small timeline example with options ordered by accuracy (now the session's practice; recorded in memory).
