# S<N> — plan check (draft, YYYY-MM-DD; the owner's answers are recorded in [the sprint record](s<N>-record.md))

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ <commit>. Every code claim cites `file:line` on that commit; "inferred" marks a reading not proven by a run. Nothing here is an owner decision until the owner answers §11.*

```
Sprint S<N> — <title> · routine: full | lighter (+ whole-engine break-it if it closes a release) · SP ≈ <n> as sized · cap 2 weeks
Entry (G0)   ☐ card complete and approved ☐ dependencies merged, main green
             ☐ fits the cap, lanes ≤ 8 days ☐ test material exists, or making it is an item
             ☐ `gate/current-sprint` set to S<N> as the first commit (§4 item 7)
             ☐ reviewed by the architect (§10) ☐ reviewed by QA (§10) ☐ answered by the owner (§11)
```
*At G0 the full done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)) is copied into the sprint record.*

SP as sized: <per item, and which are inferred>.

## 0. What the code says that the plan does not (read first)

*Numbered findings from reading the code against the sprint card and the roadmap. Each one: the fact with its citation, then "→" where it lands (a seam in §2, a panel question in §5, an owner question in §11). The last item is usually size.*

1. **<the mismatch in one bold sentence>.** <the evidence, cited>. → <§2 S1 / §5 P2 / §11 Q3>.

## 1. Routine, shape, waves

**<Full loop | lighter routine>** (why: which row of [§5](00-sprint-model.md#5-choosing-the-routine-at-the-plan-check) it falls under). **Waves (≤ 2 agents):** W1 lane <a> + lane <b> · W2 … *Changes from the sprint card:* <any split or reorder, and why>. *If two PRs:* <where the cut falls>.

## 2. Seams — signatures and stubs

*One subsection per seam, named S1, S2, …: the owner lane → the reading lanes, the Go signature or interface, and the stub plan (what the owner lane's first commit lands so readers can build against it).*

**S1 — <name> (lane <a> → lanes <b>, <c>).**
```go
// package <pkg>
type <Type> interface { … }
```
*Stub:* <what the first commit provides, and the fixture readers use until real values land>.

## 3. Packages — where each new rule lives

| Rule | Package (pure, no I/O) | Extends |
|---|---|---|
| <the rule in words> | <package> | <existing code it builds on, cited> |

## 4. Ownership — one owner per file per wave

| Lane | Owns |
|---|---|
| **<n>** | <files and packages; shared files name the wave in which this lane owns them> |
| **Coordinator** | `go.mod`, the gate registry, docs; new packages created empty before the lanes start so lanes never edit shared build files |

## 5. Design notes and the panel (before code)

*Which design note is written, by whom, and which parts wait for the blind three-reviewer panel. Then the questions the panel must answer:*

- **P1** <the question, with the facts it turns on>.

## 6. Counter-examples (listed before coding; each becomes a test)

*Per rule being added or changed: the cases a breaker would try first, numbered, each with its expected outcome. Semantic cases become conformance cases.*

**<rule>.** (1) <input> → <expected>. (2) …

## 7. Test plan per lane (the rig proves the line)

| Lane | Acceptance → rig |
|---|---|
| <n> | <each acceptance line> → **<the suite that proves it>** (conformance / replay / unit / benchmark / the demo) |

*Verifiers at "done":* <what each lane runs beyond T2>.

## 8. Risks and order

1. **<risk>.** *Mitigation:* <what the plan does about it>.

**Order:** <the wave order and what drives it>.

## 9. Lane briefs (skeletons; the coordinator adds paths, worktree and the report file)

**Common to all:** your own worktree and branch; the pinned toolchain first; write only to your owned files (§4); commit early; never push or merge; tests on the real code (never a mirror); a breaker's red test is committed; no Claude attribution anywhere; never call anything an owner decision unless the owner made it; "done" = T2 `make check` exit 0 (read the `exit:` line) plus your verifiers; report to the session scratchpad, `s<N>/lane-<n>-report.md`, and your final message gives the commits, the measurements and the open items.

**Lane <n> — <name>** (<agent>). *Goal:* … *Stories:* … *Read:* … *Provides:* <seams>. *Order:* … *Accept:* <measurable lines>. *Done:* <tier + verifiers>. *Not in this lane:* …

## 10. Reviews of this plan check

*Full loop: the architect reviewer and the QA agent each read this draft blind (not each other's findings) before it goes to the owner ([§8](00-sprint-model.md#8-reviews-architecture-and-qa-at-every-stage)). Ranked on the [ground rules' scale](../ground-rules.md); each finding gets a disposition here.*

| # | Reviewer | Severity | Finding | Disposition |
|---|---|---|---|---|
| A1 | architect | blocker / should-fix / later | <seams, ownership, the owning package, whether the scope is one coherent change> | fixed in §<n> / to §11 Q<n> / rejected: <reason> |
| Q1 | QA | … | <the test plan, the counter-examples, the test material; an acceptance line with no proof that can fail> | … |

## 11. Questions for the owner (recommendation first)

- **Q1 <the question in plain words>?** → **<the recommendation>**: <why, in one or two sentences, and what the other option would mean>.
- **Owner's check additions** (refined from the diff at the end): …
