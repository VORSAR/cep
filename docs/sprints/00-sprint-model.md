# Sprint Model & Development Process — complex-events-engine

**Document ID:** SPRINT-CEE-001
**Version:** 1.1 — 2026-10-31: explicit entry and exit criteria for sprints and phases, named gates G0–G5, sprint cards, and architecture and QA review at every stage (§3, §4, §6, §8, §9). *Amended 2026-11-03 (owner, S1.1a-1 retro): batched fix rounds (§6), lanes sized to one usage window (§11), the cheapest proving run (§10).*
**Status:** Authoritative. Governs **how** every sprint runs. [sprint-plan.md](sprint-plan.md) says **what** runs next and [decisions.md](decisions.md) records the owner's decisions; [02-roadmap.md](../product/02-roadmap.md) holds the arc; [DEVELOPMENT.md](../development/DEVELOPMENT.md) and [testing.md](../development/testing.md) hold the tools, the gate and the test rules. Sprint document formats: [README.md](README.md) and [templates/](templates/).

---

## 1. Principles

The [operating ground rules](../ground-rules.md) govern everything in this document; it is the procedure that carries them out, and where the two disagree the rules win and this document is fixed. The principles behind the procedure:

- **The owner decides; the coordinator coordinates; specialists build; reviewers try to prove them wrong.** Nothing ships the owner has not checked, and no owner decision is made for them.
- **Correctness first.** A detection engine that mis-evaluates a pattern once loses trust for good (O1). Options are ranked by **the correctness of what a rule detects and emits**, never by cost or delivery date.
- **Tests are the spec.** Agent-written code is only as trustworthy as the gates it passes. The conformance suite, deterministic replay and the rule unit-test framework are the development method: they are built before the operators they test, and nothing merges around them.
- **Review early, not only at the end.** Architecture and QA review the plan before the design, the design before the code, and the combined code before the merge. A defect found in a plan costs a sentence; found in a merge, a sprint.
- **Clear edges.** A sprint starts only when its entry criteria hold and ends only when its exit criteria are proven. Nothing is "mostly done".
- **Correct order over an early demo.** Enablers before the features that need them; the architecturally correct shape even when an intermediate step shows nothing.
- **Every change leaves the code more coherent.** Read the neighbouring code first; reuse an existing seam or refactor to a cleaner one; never bolt on.
- **The gate is the floor; trying to break it is the bar.** A green gate is where review starts, not where it ends.
- **Status lives in code and git.** Documents hold the plan and the reasons, never a second status narrative.

---

## 2. Roles

| Who | Does | Never |
|---|---|---|
| **The owner** (the product owner, the one human developer) | Sets direction; approves each sprint card and each phase's decomposition; decides at the gates from a plain-language recommendation; runs the owner's check (the demo, the quickstart, the detections and their evidence chains); merges every PR by hand. | — |
| **The coordinator** (the Claude Code session) | Writes the sprint cards and the plan check; runs the reviews; briefs the agents; combines their work; runs the gate on the combined branch; **verifies every agent claim against the code and the gate's real exit code**; writes the sprint record; keeps the documents and the project memory current, including a resume point whenever work pauses. | Labels anything an owner decision that the owner did not make; reports "green" from a log that merely looks fine; starts a sprint whose entry criteria do not hold. |
| **The architect reviewer** (+ the-fool) | Reviews structure at every stage (§8): seams and ownership in the plan check, contracts and failure modes in the design, drift in the combined diff, the base before a phase starts. | Writes the code it reviews. |
| **The QA agent** (+ the-fool) | Reviews testability at every stage (§8): the test plan and counter-examples in the plan check, a failing test for every design rule, then tries to break the combined build. | Writes the code it breaks; leaves a red test uncommitted. |
| **Specialist agents** | One build job each, from a short brief, in their own worktree. Roster (2026-10-31): project — architect reviewer, product manager, QA, refactoring; skills — language-design, architecture-designer, the-fool, the gitnexus set; user — code review, test automation, research, competitive analysis. **No Go specialist yet**: the first build sprint's plan check names one. *(Annotated 2026-11-02: the Go specialist is `go-engineer` (`.claude/agents/go-engineer.md`), which runs JetBrains' `use-modern-go` skill before editing Go; every agent inherits the session's model — owner decisions, sprint-plan §5.)* | Push, merge or commit outside their lane; add any Claude attribution. |

---

## 3. Scoping a sprint

Every sprint is defined by a **sprint card** in [sprint-plan.md](sprint-plan.md) ([template](templates/sprint-card.md)) before it can start:

| Field | Rule |
|---|---|
| **Goal** | One sentence: what is true when the sprint is done. Every item in scope serves it; an item that does not belongs to another sprint. |
| **In scope** | The items, each small enough for one lane, each with its story id once the backlog exists. |
| **Not in scope** | The near-misses a reader would expect here, and where each goes (a later sprint, the backlog, "won't"). |
| **Entry criteria** | The standard ones (§4) plus any this sprint adds: a fixture corpus recorded, a decision taken, a dependency merged. |
| **Exit criteria** | The done-done block (§13) plus the sprint's **acceptance lines**: each a measurable outcome with **the named test or measurement that proves it**. A line no test can fail is rewritten until one can. |
| **Size, routine, dependencies** | Story points (Fibonacci: 1, 2, 3, 5, 8, 13); the routine (§5); the sprints that must be merged first. |
| **Owner's check** | What the owner will run, read and try at the end. |

**The shape:**
- **A 2-week cap.** A card that does not fit is split before entry (S3 → S3.1, S3.2), never mid-sprint.
- **Lanes of at most 8 days**, sized at the plan check.
- **One sprint = one PR**, linear history (cherry-picks, no merge commits) so "rebase and merge" works, and **at most 100 commits** (GitHub's rebase-and-merge limit). *(The owner may choose two PRs for one sprint at its plan check.)*
- **No standalone refactor waves.** Restructuring ships only with the feature that needs it, or when its trigger fires.
- **Velocity:** the sprint record notes points planned, points done, actual duration and surprises: the only velocity data kept.

---

## 4. Entry and exit criteria

**Entry — gate G0. A sprint starts only when all of these hold:**
1. Its card is complete (§3) and the owner approved it.
2. Every sprint it depends on is merged, and main's CI is green on the merge commit.
3. It fits the 2-week cap, with every lane sized at 8 days or less.
4. For the first sprint of a phase: the phase's entry criteria hold (§9).
5. The plan check is drafted, reviewed by the architect and the QA agent (full loop), and **answered by the owner**: no open question.
6. The test material for every acceptance line exists, or making it is an item in the sprint.

**Exit — gate G5. A sprint ends only when the done-done block (§13) is ticked with evidence:**
every acceptance line met by its named proof; T3 and T4 exit 0 from completed runs; every architecture and QA finding fixed, or forwarded to known issues with the owner's consent; the owner's check passed; merged by the owner; main green on the merge commit; the record and its retro written.

**Scope inside a sprint:**
- **Scope changes only by owner decision**, recorded in the sprint record with its date. An item added names the item that moves out, or the size change is recorded.
- **Finish-or-revert.** An item that cannot finish is reverted, not left half-built; a sprint never ends with two live owners of one job.
- **Discovered work goes out, not in.** A bug or idea found during the sprint goes to known issues or the backlog with a forward, unless it blocks an acceptance line.
- **Small fixes the owner points at during the check are fine**; another sprint's features are not.

---

## 5. Choosing the routine (at the plan check)

| Work | Routine |
|---|---|
| Event time, watermarks, ordering and lateness; pattern matching, windows, joins, aggregations; partitioned state, snapshots and restore; concurrency; the language (grammar and semantics) and the public Go API; sink delivery (retry, suppression); anything that changes what a rule detects or emits | **The full loop** (§6) |
| Docs, examples, the demo kit, CLI wording, build and tooling chores that change no behaviour | **The lighter routine** (§7) |
| A release close-out | Either, **plus a whole-engine break-it** |

The coordinator names the routine at the plan check and asks the owner before expanding a lighter sprint into the full loop. **A language change also runs the language-design skill** (semantics before syntax, then its evaluation) as its design step.

---

## 6. The full loop

| # | Step | Owner | Output |
|---|---|---|---|
| 0 | **Plan check.** Drafted read-only against `main`: the routine; **one owner for every shared helper and every cross-lane seam**; for a cross-lane split, the Go interface or signature and a stub agreed first; **the package that will own each new rule** (pure, no I/O; adapters stay thin); the lanes and their waves; for any semantic rule change, its counter-examples, each to become a conformance case; the test plan per lane; the test material for every decision the owner will judge; the owner's check. **Reviewed by the architect and the QA agent** (§8) before it goes to the owner. | Coordinator → reviewers → owner | The plan check ([template](templates/plan-check.md)) |
| | **G0 — Entry** (§4) | Owner | The sprint starts |
| 1 | **Design.** Semantics get the **blind three-reviewer panel**: a breaker re-implements from the sources (the papers, the SQL:2016 row-pattern spec, other engines' actual code and observed behaviour), a state-of-the-art challenger measures the alternatives, and the architect reviewer with the-fool checks contracts, concurrency and failure modes. None sees the others; every claim cites a source opened in that session. | Specialists | A design doc + the panel's reports |
| 2 | **Design review.** The architect reviewer and the-fool on structure; **the QA agent on testability**: every rule in the design has a test that can fail. | Reviewers | Findings, ranked |
| 3 | **Owner decisions**: plain-language options with pros and cons, the recommendation first, many at once as one list. Lasting decisions become ADRs. | Coordinator → owner | Decisions recorded in [decisions.md](decisions.md), the design, the record and the ADRs |
| | **G1 — Design approved**: review findings resolved, decisions recorded. **No code for the parts the design covers before G1**; stubs and unrelated lanes may proceed. | Coordinator | — |
| 4 | **Build in lanes.** Each lane in its own worktree and branch; **at most two agents at once** unless the owner says otherwise; tests on the real code; commit early. A lane is "done" at **T2 `make check`**, plus the conformance and replay suites for semantic lanes. | Specialists | Lane branches |
| 5 | **Combine, then gate once**: linear cherry-picks onto the integration branch; cross-lane conflicts re-resolved there; **T3 `make combine-check`** once on the combined branch. Never edit a worktree while a gate runs in it. | Coordinator | The integration branch, T3 green |
| | **G2 — Built**: every lane at T2, the combined branch at T3 exit 0. | Coordinator | — |
| 6 | **The slow real checks**: the demo scenario end to end, the quickstart timed, latency and throughput on the replay corpus, kill-and-resume once snapshots exist. | Coordinator + specialists | Measured results ("detection → kill switch p99 4.1 ms"), not activity |
| 7 | **Review and break-it.** Code review; **the architect reviewer on the combined diff** against the design (contracts held, no drift, ADRs current); then **the QA agent and the-fool try to break it**: out-of-order and late events, equal timestamps, `within` exactly at its bound, absence timers at the edge, partition-key explosion, malformed input, sink failures, races, invariant violations. All findings go into one list. | Reviewers | One findings list ([report template](templates/breakit-report.md)) |
| 8 | **One fix round**, split by file. Every breaker's failing test is committed as a regression test (a conformance case when it is semantic), never left in a worktree. | Specialists | Fixes on the integration branch |
| | **G3 — Reviewed**: every finding fixed, or forwarded to known issues with the owner's consent for a blocker. | Coordinator → owner | — |
| 9 | **The full gate**: **T4 `make strict-gate`** locally, or CI on the pushed branch when the owner says so. Green only from the real exit code of a run that completed (§10). | Coordinator | T4 green |
| 10 | **The owner's check**: a short checklist written from the sprint's diff, plus one standing line (owner, 2026-11-01): **"LAW Δ block read and intended"** — the gate summary's top block lists every rule-file change in the sprint (ADR-0015 §1.3). *(Until S1.1c builds the block, the record lists the sprint's rule-file changes by hand.)* *By running*: the demo, the quickstart, the CLI. *By reading*: deploy-time errors, emitted events and their evidence chains, the docs for the shipped subset. *By hand*: whatever the sprint changed that a person touches. | Owner | Pass / fail per item |
| | **G4 — Verified**: T4 exit 0 and the owner's check passed. | Owner | — |
| 11 | **Merge.** The owner merges by hand. **Main's CI run on the merge commit is the real gate**: a red main run is fixed forward at once, and nothing is tagged on a red main. | Owner; coordinator fixes forward | Main green |
| 12 | **Close-out.** The sprint record with a five-line retro; known issues updated; the [sprint-plan §Status](sprint-plan.md#status) line replaced, the old one appended to [history.md](history.md#the-status-log), and the closed card moved there; documents reconciled (plan and reasons only); memory updated; lane worktrees removed after the PR merged and the fix round is done. | Coordinator | — |
| | **G5 — Exit** (§4): done-done ticked with evidence. The next sprint that depends on this one may pass its G0. | Coordinator | The sprint closed |

**One fix round, not a loop.** Findings after the fix round that are not blockers go to known issues with a forward; the owner decides whether a blocker reopens the sprint.

**A fix round is batched** (owner, 2026-11-03): write every fix's test, run them all red once, make every fix, then verify once. Each fix still has its own failing test and, where files allow, its own commit.

---

## 7. The lighter routine

For docs, examples, the demo kit and chores that change no behaviour. The same entry and exit, with fewer steps between:

1. **G0:** a short plan check (the routine, the pieces, the owners) and a complete card.
2. One agent per piece, in parallel worktrees (the two-at-once rule still holds).
3. **G2:** the coordinator combines the pieces and runs **one** gate on the combined branch.
4. Code review only for logic or data changes. Docs are read against the code: they describe the shipped subset only.
5. Break-it only for behaviour-changing work.
6. **G4:** a few-minute owner check. **G5:** a five-line mini-retro in the ledger ([template](templates/ledger.md)).

---

## 8. Reviews: architecture and QA at every stage

| Stage | Architecture review (architect reviewer + the-fool) | QA review (QA agent + the-fool) |
|---|---|---|
| **Phase entry** (§9) | The base the phase builds on: package boundaries, the public API, concurrency model, state; blockers fixed before the first feature sprint | The test layers the phase needs exist (conformance, replay, the rig for its new kind of work) |
| **Plan check** (step 0) | Seams, ownership, the owning package per rule; that the scope is one coherent change | The test plan per lane, the counter-examples, the test material; every acceptance line has a proof that can fail |
| **Design** (steps 1–2) | Contracts, concurrency, failure modes, extensibility; the panel for semantics | Testability: a failing test exists or is planned for every rule in the design |
| **Combined build** (step 7) | The diff against the design: contracts held, no drift, no new coupling, ADRs current | Break-it: edge cases, races, boundaries, malformed input, interruption, invariant violations |
| **Release close-out** | — | The whole-engine break-it |

- **Reviewers work blind where it matters**: they see the artifact and the sources, not each other's findings or the author's hopes.
- **Every finding gets a disposition** in the record: *fixed* (with the commit and the test), *forwarded* (a known issue with a forward, the owner's consent for a blocker), or *rejected* (with the reason). A finding never just disappears.
- **Findings are ranked** on the ground rules' one scale (blocker / should-fix / later — [ground rule 15](../ground-rules.md)) so the owner decides on a short list.

---

## 9. Phases: entry and exit

A roadmap phase is a set of sprints with its own edges.

**Phase entry (before its first sprint's G0):**
1. The phase is **decomposed into sprint cards** (§3), each within the cap, in dependency order; the owner approved the decomposition.
2. **The architecture review of the base** the phase builds on passed (§8): the architecture and ADRs in `docs/architecture/` are current, and its blockers are fixed or scheduled as the phase's first sprint.
3. The enablers the phase needs (the gate, the test layers) exist or are its first sprints.

**Phase exit:**
1. The roadmap's exit criteria for the phase, each proven by its named measurement.
2. A **whole-engine break-it** when the phase closes a release.
3. The roadmap and the sprint plan annotated: what shipped, what moved, why.

---

## 10. The quality bar

**The gate.** One registry of checks drives every tier, so the owner's machine and CI cannot disagree ([DEVELOPMENT.md — The gate](../development/DEVELOPMENT.md#the-gate)):

| Tier | Command | When |
|---|---|---|
| T0 | pre-commit hook | every commit: staged files, format and fast lint |
| T1 | pre-push hook | every push: toolchain parity, format, lint, suppressions, posture guards |
| T2 | `make check` | a lane says "done" |
| T3 | `make combine-check` | before pushing an integration branch (G2) |
| T4 | `make strict-gate` = `make ci` = CI | the sprint's full gate (G4) |

*(All five tiers run since S1.1a-2; `gate/registry.json` is the tier map. Not yet: golangci lint and suppressions (S1.1c). The gate is an enabler (§16): it lands before the first feature that needs it.)*

- **One pinned toolchain** (Go and every gate tool at an exact version, in one manifest) in every checkout and worktree; T1 refuses a push otherwise. A hook is never bypassed.
- **Green means the real exit code** of a gate run that finished: read the `exit:` line, never a piped tail or a background task's wrapper. The same holds for an agent's "green".
- **The cheapest proving run** (owner, 2026-11-03): prove a claim with the cheapest run that proves it. An existing proof stands unless a later change touches what it covered; ask before a heavy re-run.
- **Prove a library claim first** ([owner, 2026-11-06](decisions.md#owner-decisions-2026-11-06--the-docs-restructure), from S3.1-2's retro): a design claim about how a library or a standard table behaves (a Unicode table, a parser's grammar, a conversion's range) is checked with a scratch program before the design goes to review.
- **A new CI workflow runs once by hand** (owner, 2026-11-06, from S3.2a-2's retro): the sprint that adds a workflow runs it once and reads its summary before the PR.
- **A guard test counts once it has been red** (owner, 2026-11-06, from S3.2b-1's retro): a test that pins wiring is shown failing with the line it guards removed before it counts.
- **Tests** follow [testing.md](../development/testing.md)'s four rules: time only ever breaks a hang; measure the component, never the machine; each test owns its world; every run has a ceiling. A rule is tested in the package that owns it; never through a mirror. A new timing- or concurrency-sensitive test is stressed under load before it merges.
- **A flaky test is fixed at its root immediately**: never quarantined, never retried.
- **A linter finding is fixed at its root**, never suppressed to get past it. Suppressions carry the metadata DEVELOPMENT.md requires.
- **Correctness.** Every shipped construct has conformance cases. Replay is deterministic: the same events and the same rules give the same output in the same order, every run. A detection that does not replay identically is a bug, not noise.
- **Honest docs.** Docs cover the shipped subset only; a limit (for example, pattern state not surviving a restart before snapshots) is stated, not hidden.
- **Data.** Fixtures are synthetic. Real agent telemetry, credentials or customer data never go into a test, a fixture or a break-it run.

---

## 11. Working with agents

**Every brief says:**
- the goal, what is already ruled out, and the files to read (the brief is the agent's only context);
- the agent's working directory (its own worktree) and **the files or packages it owns** (one owner per file when lanes run in parallel);
- read-only or write; never commit, push or merge unless the brief says so;
- the pinned toolchain first on `PATH`;
- the tier to run at "done" (T2, plus the conformance and replay suites for semantic work);
- **commit early** (usage limits kill agents; commits make the resume clean);
- tests on the real code; a breaker's red test is committed, not left behind;
- no Claude attribution anywhere (commits, PR text, documents, comments);
- never label anything an owner decision unless the owner made it;
- where its report goes (a file in the session scratchpad) and what its final message must contain.

**Running them:**
- A **fresh agent per job**, never one agent reused for several jobs (its context and cost grow without bound). A reviewer is never the agent that wrote what it reviews. Check an agent's `model:` frontmatter before choosing it.
- **At most two at once** unless the owner says otherwise.
- **A lane fits one usage window** (owner, 2026-11-03): size it to finish before a usage limit can stop it, or do the work in the main session.
- **Monitoring:** rely on the completion notification. Never judge an agent stalled by its transcript's size or modification time: it writes only between tool rounds. The stall ladder: check the deliverable → nudge → resume by raw agent ID → the coordinator does the step.
- **Resuming across sessions** works by the agent's raw ID, not its name. Active IDs are recorded only under the active sprint's resume point in memory, and deleted when the work closes.
- **Keep an agent's worktree until its sprint closes**: a fix round may need to resume it.
- **Verify, don't relay.** Check every agent claim on the combined branch; read the gate's exit code yourself.

---

## 12. Decisions and documents

- **Owner decisions** come as plain-language options with pros and cons and the recommendation first, in small batches. The answer is read literally, including free text. A gate is never passed while its question is unanswered. Where the owner's answer differs from the recommendation, the record marks it "≠ R".
- **Lasting technical decisions become ADRs** in `docs/architecture/` (the architecture-designer skill's template): the context, the options, the decision, the consequences.
- **Cross-cutting decisions** (a re-plan, a change to the language's semantics, a reversal of a recorded decision) use the reconciliation method: blind independent reviewers on the plan *and* the code → one merged, ranked synthesis → decision cards (options, pros and cons, what a rule author would see) → an impact map (what each answer changes in the plan) → a pre-mortem by the-fool → the owner decides card by card → the plan rewritten → a cross-check of every changed document against the decisions and each other.
- **Where things live:**
  - the open sprint cards, the forward schedule and the one prose status line: [sprint-plan.md](sprint-plan.md); the owner's decisions, dated: [decisions.md](decisions.md); closed cards and the old status lines: [history.md](history.md);
  - the product brainstorm and its decisions, the objectives and the arc: [docs/product/](../product/);
  - stories and acceptance: `docs/product/backlog.md`; open defects with forwards: `docs/product/known-issues.md` (each created with its first entry);
  - sprint plan checks, records, ledgers and break-it reports: `docs/sprints/`;
  - architecture and ADRs: `docs/architecture/`; design packages: `docs/design/<topic>/`;
  - reviews and panels: `docs/reviews/`;
  - release notes: `docs/releases/<tag>.md`.
- **A changed decision is annotated** ("amended YYYY-MM-DD: …") or marked **superseded**, never silently rewritten or deleted. This stops the same question being re-argued.
- Documents describe behaviour. They do not cite file paths or line numbers as status, and never carry commit hashes or counts as status. *(A plan check is the exception: it is a dated reading of one commit and cites `file:line` on that commit.)*

---

## 13. Done-done (the exit checklist)

Copied into each sprint record at G0 and ticked, with evidence, by G5. A lighter sprint marks the Design and review rows "n/a (lighter)".

```
Sprint <id> — <title> · routine: full | lighter · SP <n> · cap 2 weeks
Entry (G0)   ☐ card complete and approved (goal, in / not in scope, acceptance with named proofs)
             ☐ dependencies merged, main green ☐ fits the cap, lanes ≤ 8 days
             ☐ plan check reviewed (architect, QA) and answered by the owner
             ☐ test material exists, or making it is an item
Design (G1)  ☐ architecture + the-fool findings resolved ☐ QA testability findings resolved
             ☐ owner decisions recorded ☐ ADRs written for lasting decisions
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate / CI exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR (or the two the owner chose), ≤ 100 commits each, merged by the owner
             ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
```

---

## 14. Releases

- The owner decides when a release is due. A release is a **`v*` tag**: CI builds it from the tag and attaches build provenance; the release notes live in `docs/releases/<tag>.md`. The first is `v0.1.0`, Phase 1's exit ([roadmap](../product/02-roadmap.md#exit-criteria-definition-of-shipped)).
- **Every release close-out gets a whole-engine break-it.** The last sprint before a release carries it.
- Docs ship with the release and cover only what it contains.

---

## 15. Cadence

There are no fixed weekday rituals. Work runs in sessions:
- **Sprint start** = G0 with the owner, including any choice the card leaves open.
- **During the sprint**, the owner is asked only at the gates and for checks; progress is reported as measured results.
- **Pause and resume:** before any pause the coordinator records the resume point in memory: the state, the last gate passed, what is uncommitted, the next step, and the raw IDs of agents to resume.
- **Sprint end** = G5.

---

## 16. Sequencing

Enablers come before the features that depend on them (the gate, playback mode and the conformance harness before the first operator), and every sprint is independently done-done; an epic may span sprints. The order and each sprint's dependencies live in [sprint-plan.md](sprint-plan.md) and the [roadmap](../product/02-roadmap.md)'s phases, not here.

---

## Related documents

- [sprint-plan.md](sprint-plan.md): the open sprint cards, what runs next, and the one status line.
- [decisions.md](decisions.md): every owner decision, dated, with an index.
- [history.md](history.md): the closed sprint cards and the old status lines.
- [README.md](README.md): the sprint documents, their names and their templates.
- [02-roadmap.md](../product/02-roadmap.md): the phases and the MVP slice.
- [01-objectives.md](../product/01-objectives.md): the objectives and the operating model.
- [DEVELOPMENT.md](../development/DEVELOPMENT.md): the toolchain, the gate tiers, the code rules, suppressions.
- [testing.md](../development/testing.md): the four test rules, the test layers, how to write a waiting test.

---

**Last updated:** 2026-11-06
