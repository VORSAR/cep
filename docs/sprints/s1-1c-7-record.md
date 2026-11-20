# S1.1c-7 — sprint record

**Sprint:** S1.1c-7 — correctness and truthful tests: every behaviour defect the survey found, sprint ids in text users see, and every test that does not test what it claims · **Routine:** the full loop without the semantics panel · **Size:** about 39 SP (S1.1c-6's rate) against the 2-week cap · **Branch:** `s1-1c-7/integration`
**What to build and how each item is proven:** [the plan check §12](s1-1c-7-plan-check.md#12-the-card) · **How it runs:** [the plan check §1](s1-1c-7-plan-check.md#1-routine-shape-waves) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, every proof as it is made, the finding ledger, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan is [s1-1c-7-plan-check.md](s1-1c-7-plan-check.md) (v3), drafted read-only from `main` @ 6602ee3, then 5b4ddac.
- **The survey before it** ([summary](../reviews/s1-1c-7-survey.md), [every finding](../reviews/s1-1c-7-survey-findings.md)): 650 source files read in full by fresh `refactoring-specialist` agents with GitNexus; 1,442 findings in twelve groups. The three gate holes (G1) went up as PR #51 (`main` a2a63be, 5b4ddac). The committed findings list's sha256 is `b7fce96229f55e94c97d9f96750ee0bf17df717c1cb6d14d341554d61c069520`; the summary's is `d30b4f523c934b5677d93af2c962ca2e529ba4399a7a76820f144379b3193dc2`.
- **Three rounds of blind reviews** ([architecture](../reviews/s1-1c-7-plan-check-architecture.md), [QA](../reviews/s1-1c-7-plan-check-qa.md)):
  - v1: 2 blockers each;
  - v2: one blocker (QA) and 28 should-fix between them;
  - v3's narrow re-check: no blocker, seven should-fix, all written into v3.

  Every finding's disposition is in the plan check's §10. One was rejected, with its reason (PA3-L2).
- **What the plan did not know at first** (§0):
  - the 443 findings first placed in this sprint needed about 97 commits before a fix round;
  - `mutate.py` reads only top-level Go tests, with no expected outcomes and no Python mode;
  - the gate couples to test names, paths and texts in ten places;
  - 13 TEMP size exclusions expire on 2027-02-11, and 15 TEMP vulns entries on 2026-12-14.

## Decisions at the gates

**The survey — the owner, 2026-11-20** ([decisions](decisions.md#owner-decisions-2026-11-20--s11c-7s-survey)): every source file read by parallel agents as the sprint's first step, up to six agents for that read-only job only; everything wrong reported; the groups placed at the plan check; exported renames each need the owner's yes; the gate holes as a fix PR at once; both survey documents committed.

**The plan check — the owner, 2026-11-20** ([decisions](decisions.md#owner-decisions-2026-11-20--s11c-7s-plan-check); twelve answers, three not the recommendation):
1. **The re-cut into four:**
   - S1.1c-7 correctness (185 open findings);
   - S1.1c-8 names;
   - S1.1c-9 size and the exclusion machinery's retirement;
   - S1.1c-10 the rest.

   S1.1c-5-3's forward and `LAST_PART` move to S1.1c-9.
2. **No spike** (not the recommendation); about 39 SP from S1.1c-6's rate.
3. **The design notes decide undecided behaviour and user text** (not the recommendation), with dry runs, checked by the blind reviewers.
4. **Deletions with proof;** merges, splits and moves later.
5. **The wide naming rule** (not the recommendation): 429 names in 105 files.
6. **One Python naming tool,** registered as a row from T0 in S1.1c-8.
7. **The weighted, seeded spot-check.**
8. **S1.1c-7 renames no existing test.**
9. **G10-22 and G10-15 are internal,** so the design notes decide them.
10. **G2-24 is fixed in full,** replacing a 2026-11-14 backlog item.
11. **The 2026-12-14 vulns expiry:** an upgraded golangci-lint first, otherwise the owner renews.
12. **The card is approved;** the branch is pushed as it grows.

**The card — the owner, 2026-11-20:** approved, so the sprint passes G0.

## How it runs

At most two agents at once; the plan check §1 holds the order:
1. G0;
2. the design notes, their blind reviews and G1;
3. W0 (`mutate.py`'s contract, the bench defects, the proof tools' own tests, the naming tool);
4. W1 (A1, B) → spot-check → W2 (E1, A2) → spot-check → W3 (E2, CD) → spot-check;
5. combine.

## Measured results

*(Each proof is written here as it is made, with the commit it measured.)*

## The finding ledger

*(186 ids: G10's 23, G7a's 6, G2's 156 and G3-72; built through the sprint and checked by the coordinator's ledger checker, plan check §7.5. G10-6: fixed elsewhere, G1-1, PR #51, 5b4ddac.)*

## Test material

Each lane's design note holds every finding's kind, its exact change and its proof's material (plan check §5, §7); W0's counter-examples are in the plan check §6.

## Scope changes

*(none after G0)*

## Findings

*(review and break-it findings, with their dispositions)*

## Done-done

```
Sprint S1.1c-7 — correctness and truthful tests · routine: full · SP ≈ 39 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-20)
             ☑ dependencies merged, main green (S1.1c-6, PR #49; close-out #50; the gate-hole fix, PR #51, main 5b4ddac)
             ☑ fits the cap, lanes ≤ 8 days, commits projected ≈ 80 ≤ 95 (plan check §1)
             ☑ plan check reviewed (architect, QA; three rounds) and answered by the owner (2026-11-20)
             ☑ test material: each lane's design note, before G1 ☐ `gate/current-sprint` set, the first commit after the plan check's
Design (G1)  ☐ architecture + the-fool findings resolved ☐ QA testability findings resolved
             ☐ owner decisions recorded ☐ ADRs written for lasting decisions
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0 ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff ☐ QA break-it run ☐ every finding fixed, forwarded or rejected with reason
Verify (G4)  ☐ T4 strict-gate exit 0, run complete ☐ owner's check passed ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded ☐ the row closed in gate/sprints.json as in the plan table
```

## Retro

*(at G5)*
