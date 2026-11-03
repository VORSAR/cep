# S1.1a-2 — sprint record

**Sprint:** S1.1a-2 — Hooks and CI · **Routine:** the full loop, no design panel · **Size:** about 8 SP (items 4.5) against the 2-week cap · **Branch:** `s1-1a-2/integration`
**What to build and how each item is proven:** [sprint plan — S1.1a-2](sprint-plan.md#s11a-2--hooks-and-ci) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan check is [s1-1a-2-plan-check.md](s1-1a-2-plan-check.md) (v2), drafted read-only from `main` @ 5166ce4. Its §0 found 11 places where the plan as written did not match the code: no T0 support in the runner; the runner takes its law from its own tree; git finds the real repository from any folder under the work tree; the hooks outside the Python checks; records that already carry `blocking`; no `make setup`; the first Linux run; no required check yet; the first outside dependencies; the owner's image-version commitment; the size. Both blind reviews ([architecture](../reviews/s1-1a-2-plan-check-architecture.md), [QA](../reviews/s1-1a-2-plan-check-qa.md)) are answered in its §10; nothing was rejected.

## Decisions at the gates

**Plan check — the owner, 2026-11-03** (asked in plain language with options; every answer as recommended; also in [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-03--s11a-2-plan-check-g0)):

1. **The card is re-sized to about 10 SP** (items 3.5 → 5.5), still one sprint.
2. **The coordinator builds in the main session;** fresh, time-boxed agents write QA's red tests and do the step-7 review.
3. **CI may use four GitHub Actions, and only these** (`checkout`, `cache`, `upload-artifact`, `download-artifact`), pinned by commit SHA.
4. **A commit runs only the fast checks on its staged files;** `pyfloor` runs at push.

5. **The hooks check in place, with a guard** (2026-11-04; asked as a follow-up: the separate checked tree was "overkill" for a convenience check). The size falls to about 8 SP.

**Decided as recommended, unless the owner objects:** the hooks are three short launchers in `.githooks/` that find a Python ≥ 3.12 (the cached pinned one first) and run the hook code in `gate/hooks/`.

**The design — the owner, 2026-11-04:** G1 approved: "implement this fast and get it over".

## Measured results

- **The build** (2026-11-04, the coordinator in the main session; QA's 35 red tests first, by a time-boxed agent). Local T4 `make strict-gate`: 11 rows PASS (`proofs` covered 12, `pytests` 51 modules), `wall 73.1s · summed 165.8s · saved 55%`, `exit: 0`.
- **The hooks, used for real:**
  - The sprint's own commit ran T0: parity PASS, `gofmt` SKIP (nothing staged in scope), `ruff` covered 32. `wall 0.3s`.
  - The push ran T1 on HEAD: 5 rows PASS. `wall 4.4s`.
- **The first CI run** (run 41697195516, Linux, `ubuntu26 20261026.180.1`): `lint` and `proofs` passed; `test` failed.
  - **The cause:** one parity witness test. CI's tool cache sat inside the checkout, so `go` found the project's `go.mod` above every tool and followed its toolchain line.
  - **The fix:** the cache moved to the developer's path, `~/.cache/vorsar/cep/tools`, pinned by a generator test (fd6e661).
  - `summarize` judged that run correctly: one named FAIL, every other row PASS, `exit: 1`.
- **The second CI run** (run 41697865717): all four jobs succeeded; `strict-gate` `GATE PASSED`, `exit: 0`.
- **Exit 1, CI shows the same rows as a local strict-gate:** CI's 13 rows (parity in each of 3 jobs, plus 10 rows) equal the local T4 run's 13 `(job, id)` records, none only on one side. A test pins the same on real runner output (`test_summarize_local.py`).
- **Exit 4, the parallel saving on CI:** `wall 94.3s · summed 136.2s · saved 30%`, over three concurrent jobs.
- **Exits 5 and 10:** the hook proof (7 tests) and the hooks-off proof (5) pass inside the `proofs` row, locally and on CI.

**During the build — the coordinator, 2026-11-04** (QA's red tests named eight ambiguities in design §12; the ones that change behaviour, settled here for the owner to overrule):
- **Two attempts downloaded together ("re-run all jobs"): only the current attempt is judged** (QA ambiguity 7; plan check N33 read this way). Records of an older attempt count only as "stale" when they are all a row has (N32). Otherwise every full re-run would be red.
- **`summarize` reads every job's folder in every artifact,** so a record filed in another job's artifact is a duplicate, never invisible.
- **The composite run id `<run_id>-<attempt>` is what `summarize` compares** (ambiguity 1); RED reasons start with their class word (ambiguity 4); `summarize` writes its own `run.json` and `summary.md` under `<artifacts>/strict-gate/` (ambiguity 3).

## Test material

- the hook proof's isolated git worlds (built at test time, never committed unformatted files);
- the `summarize` fixture trees (planted CI artifacts: missing, duplicate, malformed, stale, wrong tier, unknown row, inconsistent, foreign registry, two attempts);
- the hooks-off proof's world.

## Scope changes

None after G0. *(The re-size to about 10 SP was decided at G0.)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S1.1a-2 — Hooks and CI · routine: full · SP ≈ 10 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (re-sized by the owner, 2026-11-03)
             ☑ dependencies merged (S1.1a-1, PR #1), main green (local T4) ☑ fits the cap, lanes ≤ 8 days
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-03) — G0 passed
             ☑ test material: making it is an item (the hook, hooks-off and summary proofs)
Design (G1)  ☑ architecture + the-fool findings resolved ☑ QA testability findings resolved (plan check §10)
             ☑ owner decisions recorded ☑ ADRs: residue noted in design §12.8 — G1 passed 2026-11-04
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate / CI exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
```

## Retro

*(at close-out)*
