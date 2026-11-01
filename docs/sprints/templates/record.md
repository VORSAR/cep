# S<N> — sprint record

**Sprint:** S<N> — <title> · **Routine:** <the full loop | the lighter routine> · **Size:** <SP> as sized against the 2-week cap · **Branch:** `s<N>/integration`
**What to build and how each item is proven:** [sprint plan — S<N>](sprint-plan.md#<anchor>) · **How it runs:** [sprint plan — S<N>, how it runs](sprint-plan.md#<anchor>) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full draft is [s<N>-plan-check.md](s<N>-plan-check.md), drafted read-only from `main` @ <commit>. Its §0 found <n> places where the plan as written did not match the code (<one clause each>). The answers below settle the owner-facing ones; the rest are design inputs the lanes carry.

## Decisions at the gates

**Plan check — the owner, YYYY-MM-DD** (asked in plain language with options; "≠ R" where the answer differs from the recommendation):

1. **<the decision in one bold sentence>.** <what it means for the sprint>.

**Decided as recommended, unless the owner objects:** <the lower-stakes answers, in one paragraph>.

**The design — the owner, YYYY-MM-DD** (after the design note, its panel and the author's revision):

1. **<decision>.** <reason, with the source it rests on>.

**During the build — the owner, YYYY-MM-DD:**
- **<decision>** (<what prompted the question>).

## How it runs (revised after the plan check)

At most two agents at once.

| Step | Slot A | Slot B |
|---|---|---|
| Before the lanes | **Coordinator:** the integration branch's first commit (new packages, shared types, stubs) | — |
| W1 | **Lane <n> — <name>** (<agent>): <scope> | **Lane <m> — <name>** (<agent>): <scope> |
| Close-out | **PR:** combine → T3 → slow checks → review + break-it → one fix round → T4 → owner's check → **PR** | — |

## Measured results

*(filled as lanes land: the measurement named by each acceptance line, as a number)*

- **<what was measured> (<where, when>):** <the number and what it shows>.
- **The combined check (T3):** <steps passed>/<steps>, exit <code> (<test counts>).
- **The fix round (YYYY-MM-DD)**: review + break-it gave <n> findings; <n> fixed, <n> to known issues with forwards.

## Test material

*The fixtures, corpora and recorded event streams the sprint used, what each proves, and the gaps: anything an owner-judged decision needed but no fixture covers.*

## Scope changes

*Each change to the card after G0: the date, what moved in or out, and the owner's decision that allowed it. "None" if none.*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| <id> | architect / QA / review / owner's check | blocker / should-fix / later | <one line> | fixed (<commit>, <test>) / forwarded (<known issue>) / rejected (<reason>) |

## Done-done

*Copied at G0 from [00-sprint-model.md §13](00-sprint-model.md#13-done-done-the-exit-checklist); each box ticked with its evidence (the exit line, the PR number, the count). An unticked box says why.*

## Retro

1. **Worked — <what>.** <the evidence>.
2. **Worked — <what>.** <the evidence>.
3. **Missed — <what>.** <what it cost, and when it could have been seen>.
4. **Friction — <what>.** <tooling, process or code>.
5. **Change next sprint.** <concrete changes; the lasting ones go into the ground rules or the sprint model at close-out>.
