# S3.2c — sprint record (tests first, one review, a break-it)

> **Historical — S3.2c, the spec cleanup: the language documents agree, and overflow is an error; merged in PR #21.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** [design](s3-2c-design.md) · **record** · reviews: [combined build: architecture](../reviews/s3-2c-combined-architecture.md), [break-it: QA](../reviews/s3-2c-breakit-qa.md). **← [S3.2b-2](s3-2b-2-record.md) · [S3.3-1](s3-3-1-record.md) →**

**Sprint:** S3.2c — the spec cleanup: the language documents agree, and overflow is an error · **Routine:** by the owner (2026-11-06): QA's tests first; a one-page design note and the build in the main session; one architecture review of the design and the code together; a short QA break-it on overflow; one fix round, under the race detector before T4 · **Size:** 3 SP (items 2.5); 4 SP (items 3.5) after the owner's fix-round additions · **Branch:** `s3-2c/integration`
**What to build and how each item is proven:** [the S3.2c card](history.md#s32c--the-spec-cleanup-the-language-documents-agree-and-overflow-is-an-error), its "how it runs" note and the plan check's readings · **Process:** [00-sprint-model.md](00-sprint-model.md) · **Before:** [S3.2b-2](s3-2b-2-record.md)

## Plan and decisions

- **Placed by the owner** before S3.3 (2026-11-06, [decisions](decisions.md#owner-decisions-2026-11-06--the-open-items-after-s32b-2)), with integer overflow decided: an expression error for that rule, counted, as division by zero is.
- **The plan check (owner, 2026-11-06):** three answers, all as recommended ([decisions](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check)): the card as written; QA's tests first, one review, a break-it; the overflow rule over every 64-bit operation — ints, durations, timestamps.
- **In this sprint:** item 1, the five contradictions (known issue S2.2b-1); item 2, the evaluation's two notes (E-IDLE-001's wording; every reserved word in SPEC Appendix B, test-pinned); item 3, overflow as an expression error, SPEC rev 6.
- **Exits:** 1, the contradictions resolved with no verdict changed; 2, Appendix B equal to `grammar.lark`'s list; 3, every 64-bit overflow an expression error for its rule only; 4, no in-range result changed.

## Scope changes

- **Added by the owner, 2026-11-07** ([decisions](decisions.md#owner-decisions-2026-11-07--s32cs-review-and-break-it)), after the review and the break-it: item 4, **`timestamp − timestamp` → duration**, built and checked for overflow, with SPEC §5.10 listing every arithmetic form the engine accepts (review R2; ≠ R); item 5, **the smallest int, `-9223372036854775808`, a writable literal** (break-it K2; ≠ R). About 1 point; exits 5 and 6 on the card. Both change a verdict on purpose (each was refused before), as does K1's fix (a negative bound override, accepted before); exit 1's "no verdict changed" is the five contradictions'.

## Measured results

- **The build (9b1b336):** QA's tests red first on the wrapping arithmetic — the `expr` table (82 rows: 40 overflow, 42 in range, each in-range row allocation-free), the live-and-replay test, the harness check; green on the build. T2 and T3 exit 0, each about 98 s of gate wall time.
- **An observation, not a defect:** the T2 and T3 commands took 30–60 min of wall clock for that 98 s of gate time. The machine's load average was 16–25 (Spotlight indexing, three orphaned gate-toolchain processes from an earlier day), and a few of the gate's own test processes lingered after their rows; the gate waited for them, and the exit codes stand.
- **The review** found the code agreeing with `math/big` over 6.3 million evaluations and Python's `checked_int64` with Go on 866,112 cases; **the break-it's** guards ran each form 3000 times on seeded operands at the bounds, 21 composite cases, and a four-rule live-and-replay test.
- **The fix round:** every new behaviour's test red once (`TestTimestampMinusTimestampIsADuration`, QA's `TestBreakitTheSmallestIntIsAWritableLiteral` and `TestBreakitANegativeDurationOverrideInABoundSlotIsRefused`, `TestANegativeBoundIsEDUR003`, the smallest-int row of `TestTheSmallestIntIsWrittenWithItsSignOnly`), then green; the race detector on `internal/expr`, `internal/parser` and `internal/validate` exit 0. One golden changed, by design: `torture-lexical`, a negative int param default's unused float field, −0 before, 0 now.
- **T4 (`make strict-gate`, at 5c52a20):** exit 0 — 19 rows PASS (race, determinism on Linux and macOS, tripwire, proofs 18, pytests 57, differential 69, semantics, coverage inclusion among them); 172 s of gate wall time, about 3 min on the clock, with the machine's load average at 20–26.

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| R1 | combined review | should-fix | SEMANTICS §6.4's corrected absence form put `->` before `and not`, which the grammar refuses; both §6.4 and §6.5 notes cited SPEC §5.3 for §5.2 | fixed: 13e3a7e (the form as SPEC writes it; §5.2) |
| R2 | combined review | should-fix | SPEC §5.10 lacked `int × duration` and `duration / int`, which the engine accepts; SEMANTICS §1 promised `timestamp − timestamp`, which it refused | fixed by the owner's decision 1 (≠ R): 48c8da3 (built, test c74f739 red first), 13e3a7e (§5.10, §1) |
| R3 | combined review | should-fix | the S5.1 forward named only timestamps past year 9999; before year 0000 is as unwritable | fixed by the owner's decision 2: the decision annotated, the card and S5.1's forward annotated, the design note (5c52a20) |
| R4 | combined review | later | SPEC §4.2 and SEMANTICS §2.1 said a plain rule "holds none"; a partitioned one arms a ttl timer on its stream's admission watermark | fixed: 13e3a7e (errors.md's wording) |
| R5 | combined review | later | SPEC §6's "40 checks" (50 now); Appendix B's count and order untested; DESIGN's "no new code" | fixed: 13e3a7e (the count dropped, the order tested, "no new error code") |
| R6 | combined review | later | the overflow messages print some durations and timestamps as raw milliseconds; `neg` printed its operand after overwriting it | the operand fixed (48c8da3, no behaviour change); the units forwarded to S5.2, where expression errors are first logged (5c52a20) |
| R7 | combined review | later | the design note placed `abs`'s evaluation in registry.go | fixed: the design note v2 (5c52a20) |
| K1 | break-it | should-fix | a negative duration override in a bound slot deployed with no diagnostic: `output first every` silently off, `partition.addSat` would wrap | fixed: 669ff8e (E-DUR-003 for any bound slot and `idle after`; a negative ttl not also E-TTL-002); QA's test f0b287c and `TestANegativeBoundIsEDUR003` (c8653d5) red first |
| K2 | break-it | later | the smallest int could not be written as a literal | fixed by the owner's decision 3 (≠ R): a59cc01; QA's test ec299ea red first, the guards 665de2f |
| — | break-it, observed | — | `true or <overflow>` short-circuits with no error, as with division by zero; SPEC is silent | rejected as a defect: deterministic, and no rule acts on a value it never computed (the review agrees, its "checked" 5); noted here |
| — | break-it, observed | — | SPEC §5.6 says an expression error is "logged"; nothing logs yet | rejected for this sprint: S5.2's exit 4 builds the logging (with R6's units) |

## Done-done

```
Sprint S3.2c — the spec cleanup · routine: tests first, one review, a break-it (owner) · SP 3 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-06) ☑ dependencies merged, main green (S3.2b's PRs #14, #19; main 53facc5, the gate green)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner ☑ test material: QA's tests first, an item
Design (G1)  ☑ the design note (33d450a; v2 5c52a20) ☑ owner decisions recorded
Build (G2)   ☑ T2 ☑ T3 ☑ tests on the real code
Review (G3)  ☑ one architecture review (design and code; 0 blockers) ☑ QA break-it on overflow (0 blockers) ☑ every finding fixed, forwarded or rejected (table above)
Verify (G4)  ☑ the fix round under the race detector ☑ T4 exit 0 ☑ owner's check (merged) ☑ exits 1–6 met by their proofs
Exit (G5)    ☑ one PR (#21, 20 commits), merged by the owner ☐ main CI green on the merge commit (f31f0a2: the owner reports) ☑ record + retro
             ☑ known issues S2.2b-1, S3.2b-1-1 closed (none new) ☑ status line ☑ documents reconciled ☑ memory updated ☑ worktrees removed ☑ points and duration
```

## For the owner (the owner's check)

*By reading* (each change is annotated where it stands):
- [SEMANTICS](../language/SEMANTICS.md): §1 (arithmetic, `timestamp − timestamp`), §2 (the time clause), §2.1 (idleness heading), §6.4 (the absence form), §6.5 (`once`), §10 and §11.10 — search for "2026-11-07".
- [SPEC](../language/SPEC.md): §0 and the rev 6 change log (Appendix E), §2.4 (the number row), §4.2 (`idle after`), §5.6 (overflow), §5.10 (the arithmetic list), Appendix B (the reserved words).
- [errors.md](../language/errors.md): E-SEL-001, E-IDLE-001, E-DUR-003, E-TYPE-001.

*By running* (the pinned Go on `PATH`, `GOTOOLCHAIN=local`):
```
go test -count=1 -run 'Overflow|TimestampMinusTimestamp|SmallestInt|NegativeBound|NegativeDuration|AppendixB' ./internal/expr ./internal/parser ./internal/validate ./internal/playback
```
Every test passes. To see one fail, change `return r, (x^r)&(y^r) >= 0` in `addInt` (internal/expr/nodes.go) to `return r, true` and run it again; the nine `+` overflow rows (int, duration, timestamp) fail.

## Retro

- **Worked:** QA's tests, written first and red against the wrapping arithmetic, specified the build; the review's scratch programs (6.3 million evaluations against `math/big`, 866,112 Go-against-Python cases) showed the code correct, so every finding was in the documents; the owner's two additions went into the one fix round test-first; the race detector on the fixed packages before T4, the new rule, and T4 passed on its first run.
- **Hurt:** my correction of SEMANTICS §6.4 brought in a new wrong form (`->` before `and not`) and cited the wrong SPEC section twice — a contradiction fix written from memory, not checked against the grammar; the design note's "no other model changes" was wrong by one golden (an int default's unused float field, −0 before); T2 and T3 took 30–60 min of wall clock under the machine's load for about 98 s of gate time.
- **Keep:** re-running each agent's red test and scratch proof before acting; one red run and one verification for the whole fix round; a mutation of the build to show the owner's check can fail.
- **Change next sprint:** parse every syntax form a language document quotes before writing it in; regenerate the goldens on a scratch run before claiming a change leaves them alone.
- **Lasting-rule proposal for the owner:** a test parses each pattern and clause form that SEMANTICS and SPEC quote in backticks (the review's optional doc test, R1), so a document cannot show a form the grammar refuses. *(Adopted by the owner, 2026-11-07 ([decisions](decisions.md#owner-decisions-2026-11-07--s32cs-close-out)): the rule is in the sprint model; the test joins S1.1c.)*

**Closed (G5), 2026-11-07.** PR #21 merged by the owner (20 commits, rebased onto main as f31f0a2); its CI run passed all seven jobs (lint, test, race, proofs, determinism on Linux and macOS, strict-gate). Points: about 4 (3 carded, 1 added by the owner in the fix round); duration: about half a day (plan check 2026-11-06 evening, G0 to merge 2026-11-07 10:49–14:33, with T2 and T3 slowed by the machine's load).
