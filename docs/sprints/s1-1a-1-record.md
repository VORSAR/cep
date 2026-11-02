# S1.1a-1 — sprint record

**Sprint:** S1.1a-1 — The local gate, coverage and the event kernel · **Routine:** the full loop, no design panel · **Size:** 13 SP (items 11.5) against the 2-week cap · **Branch:** `s1-1a-1/integration`
**What to build and how each item is proven:** [sprint plan — S1.1a-1](sprint-plan.md#s11a-1--the-local-gate-coverage-and-the-event-kernel) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git. S1.1a-2 (hooks and CI) gets its own record at its G0.*

---

## Plan check (step 0)

The full draft is [s1-1a-plan-check.md](s1-1a-plan-check.md) (v2). It was drafted read-only against `main` @ `0a56632` for the original card S1.1a, which it split. Its §0 found 17 places where the plan as written did not match the binding documents, chiefly:
- the shell runner could not read JSON;
- the T0 format rule contradicted itself;
- "zero is a failure" blocked docs-only commits;
- no row ran the gate's own tests;
- the card's split line would have overloaded S1.1b.

Two blind reviews of v1 found 67 issues (architect 31: 3 blockers; QA 36: 4 blockers). All are dispositioned in the plan check's §10 and none rejected; two are forwarded:
- adding `Makefile`, `.githooks/`, `go.mod` and `ruff.toml` to `gate/law.json` → S1.1b's plan check;
- the stream declaration-order test → the sprint that assigns `StreamID` (inferred: S3.2b).

The answers below settle the owner-facing questions; the rest are design inputs the lanes carry.

## Decisions at the gates

**Plan check: the owner, 2026-11-02** (asked in plain language with pros and cons, recommendation first; "≠ R" where the answer differs from the recommendation; also recorded in [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-02--s11a-plan-check-g0)):

1. **The gate's tooling is stdlib Python, not shell and not Go (≠ R).** "Runner language: this can be python"; "I prefer python for any utility scripts than shell scripts". The recommendation was a Go runner with a POSIX bootstrap.
2. **CPython 3.15.2** ("go with latest and greatest for the python"); the alternative offered was 3.14.10.
3. **S1.1a is split into S1.1a-1 (this sprint) and S1.1a-2 (hooks and CI)** (as recommended).
4. **T0 refuses an unformatted Go file and prints the fix** (as recommended).
5. **At T0 only, an empty staged scope shows SKIP** (as recommended).
6. **No three-reviewer design panel** (as recommended).
7. **Agents write the gate's code; the coordinator reviews it, with a human gate wherever the coordinator is uncertain (≠ R:** the owner's wording). The recommendation was that the owner read the whole gate diff at merge.
8. **A new `python-engineer` agent** (as recommended).
9. **ruff 0.17.3 pinned;** the gate's Python has no coverage floor, accepted as residue (as recommended).
10. **The coordinator builds a minimal runner before wave 1 (≠ R), as a throwaway scaffold (≠ R).** The recommendations were stand-in checks for wave 1, then a seed that Lane A grows.
11. **Defaults.**
    - **Accepted as recommended:** Go 1.27.3; golangci-lint v2.15.0 in `tools.json` now; `go 1.27.0` in `go.mod`; no coverage waivers until S1.1b; the shared tool cache; `.gate/runs/<run_id>/`; the generated workflow; platforms darwin-arm64 + linux-amd64.
    - **Hooks (≠ R):** `make check` refuses until the hooks are on. The recommendation was `make setup` once.
    - **CI image (≠ R):** `ubuntu-latest`. The recommendation was a pinned `ubuntu-24.04`.
    - The last two land in S1.1a-2.

12. **The S1.1a-1 and S1.1a-2 cards approved** (as recommended). **G0 passed for S1.1a-1, 2026-11-02.**
13. **Commit the planning work as four commits on `s1-1a-1/integration`; nothing pushed** (as recommended).

**The design: the owner, YYYY-MM-DD** (after the design note and its review): *pending.*

## How it runs (revised after the plan check)

At most two agents at once. The coordinator reviews every gate-code diff and takes anything it is unsure of to the owner (decision 7).

| Step | Slot A | Slot B |
|---|---|---|
| Before the lanes | **Coordinator:** the design note `s1-1a-gate-design.md` → design review (architect + the-fool, QA) → owner decisions → **G1**. Then the integration branch's first commit: `go.mod`, `.gitignore`, `tools.json`, `gate/registry.json`, `ruff.toml`, the `internal/event` declaration stub, and **the throwaway runner scaffold** (decision 10). The scaffold is sequential and runs under the system `python3` with Go 1.27.3 checked against `tools.json`. It runs the registry's rows and prints the summary and `exit:` line, and `make check` points at it. | — |
| W1 | **Lane A — the runner** (python-engineer): items 3, 4, 7 | **Lane B — the toolchain** (python-engineer): item 2 |
| Combine W1 | **Coordinator:** cherry-pick; the real runner replaces the scaffold, **which is deleted**; the first real `make check` (T2) | — |
| W2 | **Lane C — coverage + proofs** (python-engineer): items 6, 8, 9 | **QA — the event tables** (red) → **Lane D — the event kernel** (go-engineer): item 5 |
| Close-out | combine → T3 → review + break-it → one fix round → T4 (`make strict-gate`, locally; CI arrives in S1.1a-2) → owner's check → **PR** | — |

A lane is done at the scaffold's `make check` in W1, and at the real `make check` from W2 on.

## Measured results

*(filled as lanes land)*

## Test material

*To be listed as it lands:*
- the `internal/event` tables (QA, written by hand before the code);
- the runner's fixture registries;
- the `file://` fixture archives;
- the coverage fixtures (`*.prof`, `*.json`);
- the planted packages under `gate/proofs/internal/`.

## Scope changes

None after G0. *(The split into S1.1a-1 / S1.1a-2 was decided at the plan check, before G0.)*

## Findings

*Plan-check findings: see the plan check's §10. Review and break-it findings of the combined build go here.*

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S1.1a-1 — The local gate, coverage and the event kernel · routine: full · SP 13 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (written from the split; approved by the owner 2026-11-02)
             ☑ dependencies merged: none, first sprint ☑ fits the cap (11.5), lanes ≤ 8 days (inferred)
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-02)
             ☑ test material: making it is an item (QA tables; proofs)
Design (G1)  ☐ architecture + the-fool findings resolved ☐ QA testability findings resolved
             ☐ owner decisions recorded ☐ ADRs written for lasting decisions
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main green on the merge commit (CI arrives in S1.1a-2: the local T4 stands in)
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
```

## Retro

*(at close-out)*
