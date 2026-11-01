# Operating ground rules

Binding on the owner, the coordinator and every agent. They override other process docs; the [sprint model](sprints/00-sprint-model.md) is the procedure. Changed only by the owner.

**Decisions**
1. The owner decides scope and trade-offs. Recommend; never decide or label an owner decision.
2. Ask plainly, recommendation first, in batches. Unsure whether it's a decision? Ask. Brainstorm in simple, non-technical language, pros and cons stated clearly, with a recommendation.
3. Ask before anything irreversible or public: push, merge, tag, delete, new dependency, API/language/format change.
4. Recorded decisions stand until the owner changes them; annotate, never rewrite.

**Scope**
5. One sprint at a time. Start only at entry (G0); end only at proven exit (G5).
6. Sprint ≤ 2 weeks, lane ≤ 8 days. Split before starting, never mid-sprint.
7. Scope changes need the owner. New work goes to the backlog unless it blocks acceptance. Finish or revert.
8. Spikes are time-boxed and produce findings, not production code.

**Quality**
9. Tests are the spec: no behaviour change without a test that failed first.
10. Correctness beats features and dates. No correctness bug open past 30 days.
11. Stop the line for red main, flaky tests, non-deterministic replay or correctness bugs. Fix the root; never retry, quarantine or suppress.
12. Green = the real exit code of a completed run, read yourself.
13. The gate (pinned toolchain, same checks locally and in CI) comes before the first feature.

**Review**
14. Architecture and QA review the plan, design and combined build; releases add a whole-engine break-it. Reviewers never review their own work.
15. Severity: **blocker** = wrong/missed detection, lost state, crash, security flaw, broken contract; **should-fix** = wrong with a workaround or misleading; **later** = polish.
16. Every finding ends fixed (with a test), forwarded (owner consents for blockers) or rejected (with reason).

**Agents and git**
17. Verify agent claims; never relay them. Fresh agent per job, one owner per file, ≤ 2 agents at once.
18. Agents never push, merge or tag. Only the owner merges to `main`; `main` is never rewritten.
19. One sprint = one PR, linear history, ≤ 100 commits.
20. No AI attribution in git. No secrets or real telemetry in the repo. New dependencies are few, pinned and Apache-2-compatible.

**Docs**
21. Status lives in code, git and the sprint plan's one status line. Docs hold plan and reasons; if docs and code disagree, the code wins.
22. Docs describe what is built and stay lean. Lasting retro changes go into these rules or the sprint model.
