# Owner decisions — complex-events-engine

**What this is:** the dated record of every owner decision that changed the plan, the scope or a design, oldest first. Moved word for word from [sprint-plan.md](sprint-plan.md) §5 on 2026-11-06; the section headings are unchanged, so their anchors are too. A sprint's record says which decisions it carried out, and its design note applies them. A changed decision is annotated in place, never rewritten ([ground rule 4](../ground-rules.md)).

**Read only what you need.** Use the index to jump to one section. Before a sprint's plan check, read the sections its card and the previous sprint's record cite, not the whole file. In the moved text, "§5" means this file, a card anchor means [sprint-plan.md](sprint-plan.md) (open cards) or [history.md](history.md) (closed cards).

**Adding a decision:** append a section "Owner decisions, YYYY-MM-DD — <occasion>" at the end, numbered, with "≠ R" where the answer differs from the recommendation, and add its row to the index.

## Index

| Date | Occasion |
|---|---|
| 2026-11-01 | [Phase 1 architecture ratification](#owner-decisions-2026-11-01--phase-1-architecture-ratification) |
| 2026-11-01 | [foundation adversarial review (gate red-team + consistency review)](#owner-decisions-2026-11-01--foundation-adversarial-review-gate-red-team--consistency-review) |
| 2026-11-01 | [Phase 1 decomposition (scoping choices; the decomposition as a whole is not yet approved)](#owner-decisions-2026-11-01--phase-1-decomposition-scoping-choices-the-decomposition-as-a-whole-is-not-yet-approved) |
| 2026-11-02 | [decomposition review (architecture + QA reviews of v1)](#owner-decisions-2026-11-02--decomposition-review-architecture--qa-reviews-of-v1) |
| 2026-11-02 | [decomposition approval](#owner-decisions-2026-11-02--decomposition-approval) |
| 2026-11-02 | [answered ahead of S1.1's plan check](#owner-decisions-2026-11-02--answered-ahead-of-s11s-plan-check) |
| 2026-11-02 | [recalibration (v3)](#owner-decisions-2026-11-02--recalibration-v3) |
| 2026-11-02 | [S1.1a plan check (G0)](#owner-decisions-2026-11-02--s11a-plan-check-g0) |
| 2026-11-02 | [the Vorsar name (during S1.1a-1's design step)](#owner-decisions-2026-11-02--the-vorsar-name-during-s11a-1s-design-step) |
| 2026-11-02 | [S1.1a-1 design review (toward G1)](#owner-decisions-2026-11-02--s11a-1-design-review-toward-g1) |
| 2026-11-03 | [S1.1a-1 review and break-it (step 7)](#owner-decisions-2026-11-03--s11a-1-review-and-break-it-step-7) |
| 2026-11-03 | [S1.1a-2 plan check (G0)](#owner-decisions-2026-11-03--s11a-2-plan-check-g0) |
| 2026-11-04 | [S1.1b slimmed (toward engine code)](#owner-decisions-2026-11-04--s11b-slimmed-toward-engine-code) |
| 2026-11-04 | [S1.2 cut to the model kernel and spec rev 3](#owner-decisions-2026-11-04--s12-cut-to-the-model-kernel-and-spec-rev-3) |
| 2026-11-04 | [S2.1, the parser: participle v2 (≠ ADR-0002's goyacc)](#owner-decisions-2026-11-04--s21-the-parser-participle-v2--adr-0002s-goyacc) |
| 2026-11-04 | [S2.2a runs in full](#owner-decisions-2026-11-04--s22a-runs-in-full) |
| 2026-11-04 | [S2.2a's gate shapes](#owner-decisions-2026-11-04--s22as-gate-shapes) |
| 2026-11-04 | [S2.2a closed; S2.2b's plan check](#owner-decisions-2026-11-04--s22a-closed-s22bs-plan-check) |
| 2026-11-04 | [S2.2b's design](#owner-decisions-2026-11-04--s22bs-design) |
| 2026-11-04 | [S2.2b closed; S3.1's plan check](#owner-decisions-2026-11-04--s22b-closed-s31s-plan-check) |
| 2026-11-04 | [S3.1-1's clock (after two architecture reviews)](#owner-decisions-2026-11-04--s31-1s-clock-after-two-architecture-reviews) |
| 2026-11-04 | [S3.1-2's plan check](#owner-decisions-2026-11-04--s31-2s-plan-check) |
| 2026-11-04 | [S3.1-2's design (after the architecture review)](#owner-decisions-2026-11-04--s31-2s-design-after-the-architecture-review) |
| 2026-11-04 | [S3.1-2's break-it](#owner-decisions-2026-11-04--s31-2s-break-it) |
| 2026-11-05 | [S3.2a's plan check](#owner-decisions-2026-11-05--s32as-plan-check) |
| 2026-11-05 | [S3.2a-1's design (after the architecture review)](#owner-decisions-2026-11-05--s32a-1s-design-after-the-architecture-review) |
| 2026-11-05 | [S3.2a-1's break-it](#owner-decisions-2026-11-05--s32a-1s-break-it) |
| 2026-11-05 | [S3.2a-2's plan check](#owner-decisions-2026-11-05--s32a-2s-plan-check) |
| 2026-11-05 | [S3.2a-2's break-it](#owner-decisions-2026-11-05--s32a-2s-break-it) |
| 2026-11-05 | [the nightly build and CI's coverage](#owner-decisions-2026-11-05--the-nightly-build-and-cis-coverage) |
| 2026-11-05 | [branch protection](#owner-decisions-2026-11-05--branch-protection) |
| 2026-11-05 | [S3.2a-2's design (after the architecture review)](#owner-decisions-2026-11-05--s32a-2s-design-after-the-architecture-review) |
| 2026-11-06 | [S3.2b's plan check](#owner-decisions-2026-11-06--s32bs-plan-check) |
| 2026-11-06 | [S3.2b-1's design (after the architecture review)](#owner-decisions-2026-11-06--s32b-1s-design-after-the-architecture-review) |
| 2026-11-06 | [S3.2b-1's review and break-it](#owner-decisions-2026-11-06--s32b-1s-review-and-break-it) |
| 2026-11-06 | [the docs restructure](#owner-decisions-2026-11-06--the-docs-restructure) |
| 2026-11-06 | [the code graph re-indexes itself](#owner-decisions-2026-11-06--the-code-graph-re-indexes-itself) |
| 2026-11-06 | [S3.2b-2's plan check](#owner-decisions-2026-11-06--s32b-2s-plan-check) |
| 2026-11-06 | [S3.2b-2's design (after the architecture review)](#owner-decisions-2026-11-06--s32b-2s-design-after-the-architecture-review) |
| 2026-11-06 | [S3.2b-2's review and break-it](#owner-decisions-2026-11-06--s32b-2s-review-and-break-it) |
| 2026-11-06 | [the open items after S3.2b-2](#owner-decisions-2026-11-06--the-open-items-after-s32b-2) |
| 2026-11-06 | [S3.2c's plan check](#owner-decisions-2026-11-06--s32cs-plan-check) |
| 2026-11-07 | [S3.2c's review and break-it](#owner-decisions-2026-11-07--s32cs-review-and-break-it) |
| 2026-11-07 | [S3.2c's close-out](#owner-decisions-2026-11-07--s32cs-close-out) |
| 2026-11-07 | [S3.3's plan check](#owner-decisions-2026-11-07--s33s-plan-check) |
| 2026-11-07 | [S3.3-1's design (after the panel)](#owner-decisions-2026-11-07--s33-1s-design-after-the-panel) |

## The decisions

*Owner decisions that change this plan are recorded here as dated sections ("Owner decisions, YYYY-MM-DD — <occasion>"), numbered, with "≠ R" where the answer differs from the recommendation. A changed decision is annotated, never rewritten.*

### Owner decisions, 2026-11-01 — Phase 1 architecture ratification

1. **Architecture baseline ratified with conditions** (docs/architecture/, 14 ADRs *(annotation 2026-11-01: written before ADR-0015 was authored the same day; the baseline is 15 ADRs — ground rule 4, annotated not rewritten)*, three review rounds): the ratification binds three additions before S1 — (a) every ARCHITECTURE.md constraint and named code smell is **enforced at build time** (violation = failed build, starting at the pre-commit hook); (b) a QA and test-automation strategy that prevents architectural debt; (c) the build is **hostile** — effectively impossible to code against by hand, modeled on enforcement machinery the owner has proven in prior projects (studied, not named). Tracked as ADR-0015 + docs/development/qa-strategy.md.
2. **Dependencies approved:** `golang.org/x/tools` (goyacc, build-time), `golang.org/x/text` (runtime), `golang.org/x/perf` (benchstat, build-time). Everything else in-repo per ADR-0009. *(annotated 2026-11-04: goyacc dropped for participle v2, approved by the owner at S2.1; `x/tools` not yet used)*
3. **cepd config = JSON** (stdlib-only; roadmap's draft YAML amended).
4. **SPEC v0.1 rev 2 approved:** expect blocks gain `given table <Name> [rows]`; regex `_ci` defined as RE2 folding. Applied, grammar re-verified green.

### Owner decisions, 2026-11-01 — foundation adversarial review (gate red-team + consistency review)

Round-4 review of ADR-0015 + qa-strategy.md + post-ratification drift: 49 verified findings (15 blocker / 25 should-fix / 9 later), all dispositioned in [gate-redteam.md](../architecture/review/gate-redteam.md#dispositions-2026-11-01-post-review-revision--every-finding-per-ground-rule-16) and [foundation-consistency.md](../architecture/review/foundation-consistency.md#dispositions-2026-11-01-post-review-revision--every-finding-per-ground-rule-16); none rejected.

1. **Derived streams deferred:** v0.1 rejects a rule consuming another rule's output stream (silent-drop watermark bug otherwise; nothing in the MVP uses chaining). The min-over-live-instances merge is recorded as the Phase-2 design (ADR-0004 §6). The error code lands in the **spec rev-3 batch at S1's plan check**, together with the forwarded RC8.1 code (cross-stream partition-key type mismatch) — the one open item.
2. **Dependencies approved (≠ R):** `go.uber.org/goleak` (test-only, MIT) and `golang.org/x/exp/apidiff` (build-time, BSD-3); `x/tools` scope widened to go/analysis + analysistest. Recommendation was in-repo equivalents; owner chose the proven tools. ADR-0009 amended.
3. **Hostility bar reworded + residue accepted:** the bar is "every freestyle shortcut is either mechanically red or a named loud row at merge" (not "impossible to code against"); the seven review-only constraints are recorded verbatim in ADR-0015 §3.6 and accepted knowingly.
4. **LAW Δ adopted:** `gate/law.json` manifest + a mandatory LAW Δ block at the top of every gate summary + the standing owner-check line "LAW Δ block read and intended" (sprint model §8 step 10). Closes the review's root finding: gate law is agent-writable, so law changes must be structurally loud at merge.

### Owner decisions, 2026-11-01 — Phase 1 decomposition (scoping choices; the decomposition as a whole is not yet approved)

1. **`cep fmt` stays in Phase 1 (≠ R).** Recommendation was to move the formatter to the backlog and surface inherited stream timing through `cep validate` instead; the owner kept the full formatter. **Placed in S6 with the CLI it belongs to** (recommended, accepted) — S2.1 is at the cap; S2.1's lexer keeps comments as positioned trivia so the printer needs no lexer rework.
2. **The formatter always writes the generated timing comment** on every rule (recommended, accepted): the 9 corpus examples are regenerated once by `cep fmt` in S6 (a spec-corpus change, LAW Δ-visible); a new gate row `fmtcheck` makes a stream-timing edit without a re-format red. The SPEC §9 clarification joins the spec rev-3 batch at S1's plan check; the comment's exact format is designed in S6.
3. **Kill-switch vertical first** (recommended, accepted): S4 patterns → S5 live → S6 parity, then text and windows.
4. **Full three-reviewer design panel for every window sprint (≠ R).** Recommendation was one breaker for sort/batch/delay with the panel for cron only; the owner chose maximum scrutiny for all four kinds (S8, S9).
5. **Attack gallery: "six or more" (≠ R).** Recommendation was exactly five. Applied as **all seven attacks the attack review classes MVP-expressible** (A2, A4, A5, A6(a), A7, A10, A13) — *the coordinator's reading of "six or more"; the owner may correct it*. S11 grows to 13 SP with a split line.

*Annotations 2026-11-02 (decomposition v2, after the architecture and QA reviews — placement only; the decisions stand):* decision 1's "S6" is now **S6.2** (S6 split into S6.1 builder/CLI and S6.2 formatter). Decision 2's SPEC §9 sentence moves from the S1 rev-3 batch to **S6.2**, landing together with the comment's format and the corpus regeneration (architecture review DA-19); `fmtcheck`'s scope excludes the unparseable invalid corpus, recorded evaluation files, testdata and proofs (DA-06/DQ-12). Decision 3's "S4 → S5 → S6" is now S4.1/S4.2 → S5.1 → S6.1. Decision 4's "S8, S9" is now S8, S9.1 (cron) and S9.2 (delay).

### Owner decisions, 2026-11-02 — decomposition review (architecture + QA reviews of v1)

57 findings (architecture 22 — 7 blocker / 10 should-fix / 5 later; QA 35 — 15 / 17 / 3), all verified against the documents and dispositioned in [phase1-decomposition-architecture.md](../reviews/phase1-decomposition-architecture.md#dispositions-2026-11-02--every-finding-per-ground-rule-16) and [phase1-decomposition-qa.md](../reviews/phase1-decomposition-qa.md#dispositions-2026-11-02--every-finding-per-ground-rule-16). Six needed the owner; all six answers matched the recommendation.

1. **`io` split** (recommended, accepted): `io` (package `cepio`) becomes a leaf package holding only the connector contracts; each built-in connector is its own sub-package (`io/http`, `io/stdin`, `io/inmemory`, `io/smtp`, `io/log`, `io/jsonmap`, `io/textmap`) importing `cep` and `io`. `cep → io` is an edge; `io → cep` is not — the Go import cycle (DA-04) is gone. Sink flusher goroutines move into `internal/runtime`. ARCHITECTURE §3, ADR-0001 and ADR-0008 annotated.
2. **Example test material inside the example files** (recommended, accepted): each example gains fire-path and near-miss expect blocks, authored by QA in the sprint that turns it green, outputs derived independently before the engine runs; a spec-corpus change visible in LAW Δ.
3. **`late events allow` removed for v0.1** (recommended, accepted): digests drop late events like every rule; per-window late handling arrives in Phase 2. Spec rev 3 (S1.2) revises E-LATE-002 and example 02, and pins cron admission to SPEC §6.1 — the Python reference executor, which admitted late events into open buckets, is aligned and a red-first harness check added.
4. **Keyword sets over a table operand supported, live** (recommended, accepted): the matcher rebuilds when the table changes; the next event sees the new terms (S7). ADR-0007 §5 annotated.
5. **`text_invisibles` is engine-owned** (recommended, accepted): the engine always computes it; a payload value is ignored, overwritten and counted — closes the spoofing bypass without letting an attacker get events dropped. Spec rev 3 (S1.2); built and tested in S3.2, re-proven in S7.
6. **`time arrival` supported** (recommended, accepted): the engine stamps events at ingest; playback replays the recorded stamp (S3.2).

### Owner decisions, 2026-11-02 — decomposition approval

1. **Phase 1 decomposition v2 approved** (recommended, accepted): 20 sprints, 240 SP, forecast 40–60 weeks, re-forecast at S1.2's and S3.3's G5. Sprint model §9 phase entry 1 holds; each sprint still passes its own G0.
2. **Attack gallery = seven examples confirmed** (recommended, accepted): the coordinator's reading of 2026-11-01 decision 5 ("six or more") is confirmed — all seven MVP-expressible attacks.
3. **Partition keys of different types across streams are rejected at deploy** (recommended, accepted — pre-mortem RC8.1, the last open rev-3 item): the error names both streams and both types; a text `"5"` and a number `5` never silently form two instances. Lands in spec rev 3 (S1.2) and the validator (S2.2).
4. **Commit and push** the plan, the reviews and the annotations (recommended, accepted).

### Owner decisions, 2026-11-02 — answered ahead of S1.1's plan check

1. **Import rules are strict** (recommended, accepted): the per-package import lists in ARCHITECTURE §3 (rules 3, 4, 6 and the exception table) are exhaustive — only listed connections are allowed; every new edge is a `rules.json` change visible in the LAW Δ block (architecture review DA-11's known edges are added in the sprints that need them).
2. **`conformance/` uses the public API only; `bench/` may import internal packages** (recommended, accepted): nothing imports either. Conformance proves the real product path; benchmarks can time internal stages for `allocgate` and ADR-0010 §8.
3. **Toolchain = newest stable** (recommended, accepted): the newest stable Go release that the pinned golangci-lint supports, verified and pinned on S1.1's first day; later bumps are deliberate commits that also fix what the new version finds.
4. **Go specialist = our own agent + the JetBrains skill** (recommended, accepted): `.claude/agents/go-engineer.md` carries the project's rules and runs JetBrains' `use-modern-go` skill (the `modern-go-guidelines` plugin the owner installed 2026-10-31 — a style skill, not an agent) before editing any Go file; where the two conflict, the gate wins.
5. **Agents always inherit the session's model** (owner's own answer, replacing both offered options): every agent definition uses `model: inherit`; no cheaper model for any lane.
6. **Branch protection: now, plus the required check in S1.1** (recommended, accepted): the owner blocks force-pushes and deletion of `main` now; once S1.1's CI has run once, the gate's summary check is added as required before S1.1 merges.
7. **Code coverage starts in S1.1 at a 90% floor (≠ R on the level).** Recommendation was to move `coverage-inclusion` into S1.1 (accepted) at the ratified 85%; the owner raised the per-package floor to **90%** (amends the 2026-11-01 ratification's 85% — ADR-0015 §2 and §4 and qa-strategy §4 annotated). Scope is unchanged (`internal/*` + the public packages incl. the `io/*` built-ins; generated files excluded; no waiver within scope). S1.1 grows; its plan check confirms the cap or applies the split line.
8. **Build and tests run in parallel wherever they can** (owner requirement): the gate runs independent rows concurrently (respecting `needs`), with only timing-sensitive rows marked `exclusive`; Go builds and tests parallelise across and within packages over a shared cache; CI runs its jobs concurrently with cached modules and builds. The summary stays deterministic (registry order) and reports the time saved. ADR-0015 §1.1 (new `exclusive` row field) and DEVELOPMENT.md amended; proven in S1.1 (exit lines 11–12).

### Owner decisions, 2026-11-02 — recalibration (v3)

1. **v3 approved** (recommended, accepted): every card re-estimated item by item against two anchors (`internal/event` = 1 point; the pattern-engine core = 5) with the fixed per-sprint process outside the points; at most 13 points per sprint. Ten of v2's twenty cards were over the cap; v3 has **23 sprints, ~270 points, forecast 46–69 weeks**, re-forecast at S1.2's and S3.3's G5. No item added or removed. Also moved by the first-subject rule: `puritycheck` and `globalscheck` to S1.1b (their first subject, `internal/event`, exists from S1.1a); the full docs for the shipped subset to S12 (so they cover every construct and the gallery).
2. **Commit and push** (recommended, accepted).

### Owner decisions, 2026-11-02 — S1.1a plan check (G0)

The plan check ([s1-1a-plan-check.md](s1-1a-plan-check.md), v2) was reviewed blind by the architect reviewer (31 findings) and QA (36), all dispositioned there, none rejected. Asked in plain language with pros and cons; recommendation first.

1. **The gate's tooling is stdlib Python, not shell and not Go (≠ R:** a Go runner was recommended). This covers the runner, summary, installer, parity, coverage, hooks and CI generator. A system `python3` (3.9 or newer) only bootstraps the pinned interpreter. The owner's standing preference is Python over shell for every utility script.
2. **The pinned interpreter is CPython 3.15.2** (owner: "latest and greatest"), the python-build-standalone build of 2026-10-26.
3. **S1.1a is split into S1.1a-1 and S1.1a-2** (recommended, accepted). Re-sized for Python, it was about 15 points. The card's split line would have overloaded S1.1b, which is full.
4. **The pre-commit hook refuses an unformatted file and prints the fix** (recommended, accepted). This annotates ADR-0015 §3.1 and DEVELOPMENT.md's T0 row, which said auto-fix.
5. **At T0 only, a check whose staged scope is empty shows SKIP** (recommended, accepted). Docs-only commits pass; everywhere else, zero is a failure.
6. **No three-reviewer design panel for S1.1a** (recommended, accepted). The design note gets the architect + the-fool and QA review.
7. **Build agents write the gate's code; the coordinator reviews it, with a human gate wherever the coordinator is uncertain (≠ R:** the owner's own wording). The recommendation was that the owner read the whole gate diff in place of LAW Δ. This annotates ADR-0015 §1.4 for this sprint.
8. **A Python build agent, `.claude/agents/python-engineer.md`** (`model: inherit`; recommended, accepted).
9. **ruff 0.17.3 pinned** (MIT, build-time) to format and lint the gate's Python, with a ban on `time.sleep` outside the wait helper. The gate's Python has no statement-coverage floor, accepted as residue (recommended, accepted; ADR-0015 §3.6, ADR-0009 annotated).
10. **The coordinator builds a minimal runner before wave 1 (≠ R), as a throwaway scaffold (≠ R).** The recommendations were stand-in checks for wave 1, then a seed that Lane A grows. The scaffold is deleted when Lane A's runner lands.
11. **Defaults.**
    - **Accepted as recommended:** Go 1.27.3 and golangci-lint v2.15.0 (in `tools.json` now); `go.mod` says `go 1.27.0`; no coverage waivers until S1.1b; the shared tool cache `~/.cache/complex-events-engine/tools`; run output in a git-ignored `.gate/runs/<run_id>/`; the workflow generated from `registry.json`; platforms darwin-arm64 and linux-amd64.
    - **Hooks (≠ R):** `make check` refuses to run until the hooks are on; CI turns them on too. The recommendation was `make setup` once.
    - **CI image (≠ R):** `ubuntu-latest`. The recommendation was a pinned `ubuntu-24.04`. *Consequence, stated:* GitHub moves `latest` on its own schedule, so a CI change can arrive with no commit of ours; the summary records the image version on every run.
12. **The S1.1a-1 and S1.1a-2 cards approved** (recommended, accepted); S1.1a-1 passed G0.

### Owner decisions, 2026-11-02 — the Vorsar name (during S1.1a-1's design step)

1. **The module path is `github.com/vorsar/cep` (owner: "package names should reflect vorsar").** Asked with three options; the owner chose the shorter branded path, then the exact path as recommended. This reverses the brainstorm's decision 5 for the module and repository names, and the brainstorm, ADR-0001 §1 and ARCHITECTURE §3 are annotated. Go package names stay short: `cep` at the root, `rule`, `cepio`, `internal/*`.
2. **The GitHub repository `VORSAR/complex-events-engine` is renamed `VORSAR/cep`, using the `gh` CLI** (the owner chose the CLI over the web settings). GitHub redirects the old URL, and the `protect-main` ruleset stays with the repository.
3. **The gate's local names follow it** (as recommended): the tool cache is `~/.cache/vorsar/cep/tools`, overridable by `VORSAR_CEP_TOOLS_CACHE`, and every gate environment variable has the prefix `VORSAR_CEP_`. This amends item 11's cache path above. **Done 2026-11-02:** the repository is `VORSAR/cep`, the ruleset `protect-main` is still active, and the local remote is updated.

### Owner decisions, 2026-11-02 — S1.1a-1 design review (toward G1)

The design note ([s1-1a-gate-design.md](s1-1a-gate-design.md)) was reviewed blind. The architect + the-fool found 24 issues (4 blocker / 14 should-fix / 6 later); QA's review was still running when the owner answered. Asked with options, pros and cons; every answer as recommended.

1. **(b+) for ARCHITECTURE §4's two zero-value traps.** `TInvalid` becomes the zero `Type`. `Schema`'s fields become private behind one validating constructor, `NewSchema`, read through accessors. §4 is annotated at G1.
2. **"The time attribute must be a timestamp" becomes a coded deploy error in S1.2's spec rev 3** (new item (j); scope added to S1.2 by owner decision). `NewSchema` enforces the same rule internally meanwhile.
3. **Wave 1's scaffold runs its own short check list** of plain commands the coordinator writes, under the pinned Python with process-group cleanup. The real registry arrives with Lane A's runner at combine-W1, and the scaffold and its list are deleted.
4. **`make check` (and every gate target) installs any missing pinned tools first.** Exit line 1 holds as written.
5. **The gate's Python never silences ruff inline.** ruff runs with `--ignore-noqa`, and the only exceptions are per-file entries in the coordinator-owned `ruff.toml`.
6. **Unpinned residue accepted:** git, make, and the system `python3` that only starts the installer. `CGO_ENABLED=0` for every row, so Go never calls a C compiler; S3.2b's race-detector row turns it on for that row alone. This is added to ADR-0015 §3.6 as residue item 9.
7. **A focused re-check before G1:** one fresh agent probes the revised sections and the blocker fixes against the real tools.
8. **The coordinator renamed the repository with `gh`** (done and verified, above).
9. **The installer's minimum Python is 3.12, verified by a pinned CPython 3.12.16** (recommended, accepted; asked after QA's review showed that the `ast` 3.9 check accepts newer syntax, and that no pinned 3.9 build exists). Apple's built-in 3.9.6 is refused, with the fix text. The bootstrap's own tests run under the pinned 3.12.16, the fifth pinned tool.
10. **G1 approved for S1.1a-1** (recommended, accepted) on design note v2.1, after the focused re-check. That re-check found 3 more blockers, which were fixed and re-verified.
11. **Parity re-hashes whole tool trees at T2 and above, and executables only at commit or push** (recommended, accepted), through a pinned `tree_sha256` per tool.

### Owner decisions, 2026-11-03 — S1.1a-1 review and break-it (step 7)

The combined build was reviewed blind:
- **Architect + the-fool:** 0 blocker / 4 should-fix / 10 later.
- **QA break-it:** 2 blocker / 2 should-fix / 6 later. Red tests are committed for 6 of them.

The coordinator re-verified the blockers' red tests. All four answers below match the recommendations.

1. **Fix-round scope:**
   - Fix both blockers (QB1: test modules no row runs; QB3: a FIFO in the cache hangs parity) and every should-fix item.
   - Also fix the cheap later items: a truncated archive accepted; a parity record cut at 300 characters; the registry used not recorded; Ctrl-C before the scheduler starts; the `make` exit-code doc note.
   - Move later-sprint items to their sprints: the nested `go.mod` check and the scope-class map → S1.1b; the hook scopes → S1.1a-2. The rest go to known issues with reasons.
   - Leftover-process risk that the fix cannot remove (e.g. a SIGKILL of the runner itself) is recorded as accepted residue.
2. **A red row's summary shows its first failure line,** and the summary prints the run's directory.
3. **A skipped Go test fails the `unit` row,** as Python skips already fail `pytests` ("a skip is never green"). S1.2's skip grammar may later admit only the named `not-implemented(S<n>)` form.
4. **Every record carries a `blocking` field.**
5. **The coordinator makes the fix round itself, in the main session** (later the same day; ≠ the plan check's "agents write gate code; the coordinator reviews"). The fix agents had stalled after rate limits. Each fix is test-first, with one commit per finding.
6. **No separate review of the fix round** (≠ R: a fresh QA reviewer was recommended, because the coordinator wrote the fixes and ground rule 14 says nobody reviews their own work). The fixes stand on their tests, each seen red first, and on the three green tiers.

### Owner decisions, 2026-11-03 — S1.1a-2 plan check (G0)

The plan check ([s1-1a-2-plan-check.md](s1-1a-2-plan-check.md), v2) was reviewed blind by the architect (2 blocker, 6 should-fix, 6 later) and QA (2 blocker, 13 should-fix, 1 later). Both reviews' blockers changed the design: a commit is checked by its own gate, in a tree outside the work tree. All four answers match the recommendations.

1. **The card is re-sized from 5 to about 10 points** (items 3.5 → 5.5): T0/T1 support in the runner, the checked tree, a `pre-merge-commit` hook and CI's record checks. It stays one sprint, under the cap.
2. **The coordinator builds the sprint in the main session.** Fresh, time-boxed agents write QA's red tests first and do the step-7 review.
3. **CI may use four GitHub Actions, and only these:** `checkout`, `cache`, `upload-artifact`, `download-artifact`, each pinned by commit SHA (MIT-licensed). They are the project's first outside dependencies.
4. **A commit runs only the fast checks on its staged files** (parity's executables, `gofmt`, `ruff`); `pyfloor` runs at push.
5. **The hooks check in place, with a guard** (2026-11-04, as recommended in a follow-up question; replaces plan v2's separate checked tree as "overkill" for a convenience check). A staged file with unstaged changes is refused; a push of anything but a clean HEAD is left to CI. The size falls to about 8 SP.
6. **G1: "implement this fast and get it over"** (2026-11-04): the owner approved the design (gate design §12) and asked for speed.
7. **No step-7 review or break-it for this sprint** (2026-11-04; ≠ item 2 and the sprint model's step 7): "we don't need to break it as per say. there are utility stuff." The stopped reviewer had reported nothing. CI's green run and the proofs stand in.

### Owner decisions, 2026-11-04 — S1.1b slimmed (toward engine code)

1. **S1.1b keeps only the checks engine code needs from its first line** (as recommended; the owner wants engine code fast): the import law, the clock rule, goroutines/channels/sync/I-O/unsafe, and package-level vars. Built as one standard-library Go program, `tools/archcheck`, with `tools/importcheck/rules.json` and one gate row, `archcheck` (no new dependency). About 5 points.
2. **The rest of S1.1b moves to a later sprint, S1.1c,** sized at its own G0: `law.json`/`lawdelta` and the LAW Δ block, registry-lint, `todocheck`, `testhygiene`, the teeth meta-test, golangci-lint `smells` + `nolintlint`, and the suppressions checker with the allowlist grammar. Until then, a package-level var outside the four allowed forms has no allowlist: it is red.
3. **"Don't make this overly complicated":** one plan in the record, no plan reviews, no step-7 review (utility code, as for S1.1a-2).

### Owner decisions, 2026-11-04 — S1.2 cut to the model kernel and spec rev 3

1. **S1.2 = `internal/model` (the IR and its canonical JSON) + spec rev 3** (≠ R: the recommendation was the model kernel alone, with spec rev 3 just before the validator sprint). The owner kept spec rev 3 beside the model because the model's fields are named after spec clauses.
2. **The gate extras move to S1.1c:** the dependency-allowlist, licence and `govulncheck` checks, the build-tag law, the weekly canary, the sprint-id grammar with `gate/current-sprint` and the `todocheck` forms, and `docsync` (a) and (d).

### Owner decisions, 2026-11-04 — S2.1, the parser: participle v2 (≠ ADR-0002's goyacc)

1. **The engine parser is built on `github.com/alecthomas/participle/v2` v2.1.4** (MIT; two helper modules in its graph), the project's first library dependency, after a time-boxed spike the owner asked for ([participle-spike.md](../language/research/participle-spike.md)): the whole grammar in about 250 lines of struct tags, the corpus exact (21 valid, 24 invalid) at lookahead 1, positions in runes, trivia round-tripping, 22 of 24 error codes mapped with one misreported position to repair. Chosen over a hand-written parser (no dependency, exact errors, 4–5× the code) and goyacc (a generated file, `x/tools`, the hardest error path). ADR-0002 is annotated, not rewritten: no `.y` grammar, no generated parser, no `freshness` row for it; the keyword table is pinned to `grammar.lark` by a test instead of a generator.
2. **S2.1 runs slim**, like S1.1b and S1.2: the card's items 1, 3, 4 and 5 (lexer with trivia, grammar and lowering, the error map with positions and fix text, QA's hand-derived `@ line:col` on the invalid corpus); item 2 (`kwgen`) becomes the pinning test; item 6 (`freshness`) is moot.

### Owner decisions, 2026-11-04 — S2.2a runs in full

1. **S2.2a as planned, all seven items (10.5 points)** (≠ R: the recommendation was the expression engine plus the grammar differential, about 7 points, with the fuzz target and nightly workflow, `docsync` (b) and (c) and `allocgate` deferred). The owner chose the full card.

### Owner decisions, 2026-11-04 — S2.2a's gate shapes

1. **`tools.json` gains a `python-package` kind** (as recommended): an entry with no command, for a pure-Python library the gate's Python side needs. Parity checks its version by importing it under the pinned CPython, and hashes its files and whole tree as for any tool. First use: `lark` 1.3.1 (MIT, the normative grammar's engine) for the grammar differential.
2. **The registry gains a `nightly` tier** (as recommended): rows marked `nightly` run only in a second generated workflow, scheduled and runnable by hand, never on pull requests; `summarize` judges them as it judges T4. First use: the parser fuzz row.
3. **Advisory aging for the fuzz row, for now** (as recommended): a crash makes the nightly run red and uploads the crashing input; the owner commits it as a regression test, which blocks every run from then on; the two-sprint clock of ADR-0015 §3.9 is a note in known-issues until the gate has persistent advisory state (planned with S1.1c's LAW Δ work). *(Annotated 2026-11-05, owner: the CI review found both fuzz rows registered non-blocking, so a crash read WARN and the nightly stayed green, against this decision. They are now blocking, and a test holds every nightly-only row blocking; nightly rows never run on a pull request, so only the nightly's verdict changes.)*
4. **The `expr → model` import edge is allowed, fenced** (as recommended, after the adversarial review the owner asked for: [s2-2a-expr-model-edge.md](../reviews/s2-2a-expr-model-edge.md)). ADR-0011's design has `expr.Build` take a `model.Expr`, which its own import list forgot; the edge points downward (both are L0 data) and every alternative is a converter or mirror tree of 160–400 lines. The exception-table row reads "expr reads `model.Expr` only"; `rules.json` fences the edge to the model's expression types (`Expr`, `ExprKind`, `Literal`, `LiteralKind`, `TextOperand`) and `archcheck` enforces the fence; Build's contract: names come only from the planner's scope, `${param}` is resolved before Build, tables are handed in by the planner.

### Owner decisions, 2026-11-04 — S2.2a closed; S2.2b's plan check

1. **S2.2a's two deviations are accepted** (as recommended): `docsync` (c) and `allocgate` stay Go tests run by the `unit` row; the differential's corpus count is 63.
2. **S2.2b runs slim, with an architecture review** (≠ R: the recommendation was slim with QA's tests first and no reviews): QA writes the test programs first and independently, from the spec and not from the build; an architecture reviewer reviews the design note before the build; the coordinator builds in the main session; no build lanes.
3. **A stream with no `@source` can mark batches** (as recommended): its connector is chosen at deployment, and the replay log carries batch boundaries (SEMANTICS §10). `E-WIN-002` fires only when `@source` names a connector type that cannot mark batches. Example 04 and eval file t07 stay as written.
4. **A `string` param in a keyword-set slot is a comma-separated list** (as recommended): split at each comma, trim surrounding spaces; an empty item is `E-PARAM-001`. A term containing a comma needs a table operand. SPEC §4.1 gains this one-line clarification.

### Owner decisions, 2026-11-04 — S2.2b's design

After the architecture review and QA's test programs ([s2-2b-design.md](s2-2b-design.md) v2):

1. **Positions live on the model nodes** (A1, as recommended; the model's doc had said "in a side table"): fields of type `model.Pos`, filled by the lowering and skipped by `MarshalCanonical`, so the canonical JSON and the goldens do not change. `TestMarshalCanonicalNoPositions` is rewritten to pin that; ADR-0003 §1 is annotated; Apps compare by canonical JSON only.
2. **Which code fires for a stream with a time clause that a rule inserts into depends on its use** (A2, as recommended): read by a rule or carrying `@source` → `E-GRAPH-002` at the insert target; otherwise → `E-TIME-002` at its `time`. Never both.
3. **Test files of `internal/validate` may import `internal/parser`** (A4, as recommended): `rules.json` gains `test_imports`, enforced by `archcheck`; production code stays forbidden.
4. **`E-WIN-002` is unreachable in v0.1** (as recommended): every built-in source punctuates; errors.md says so, and a unit test over a planted entry proves the check.
5. **`E-TBL-001` also covers a keyword-set operand naming a table with more than one column** (as recommended).

### Owner decisions, 2026-11-04 — S2.2b closed; S3.1's plan check

1. **S3.1 runs as S2.2b did, plus a QA break-it of the clock** (≠ the card's full loop; as recommended): QA's tests first and independent; an architecture review of the design note; the build in the main session; then one QA agent tries to break the finished clock.
2. **S3.1 is split along its card's line** (≠ R: the recommendation was one sprint, one PR): **S3.1-1** = items 1, 2 (the clock service and the metrics counters); **S3.1-2** = items 3–6 (tables and membership, the text flags, the decoder, the purity exceptions). Two sprints, two PRs, one at a time.

### Owner decisions, 2026-11-04 — S3.1-1's clock (after two architecture reviews)

The design review ([s3-1-1-design-architecture.md](../reviews/s3-1-1-design-architecture.md)) showed ADR-0004's interface, the reference executor and SEMANTICS disagreeing; the owner chose, each time, the most accurate option, then had their architectural impact reviewed ([s3-1-1-decisions-architecture.md](../reviews/s3-1-1-decisions-architecture.md)) and settled what that review raised:

1. **Walk forward in time.** When a rule's clock advances, its pending work is done in event-time order: everything at an instant before anything later; within one instant, released events, then the timers due then, then the strict deadlines (the `within` discard, the TTL purge) once the clock is past it. Results no longer depend on how far one advance jumps. Replaces ADR-0004 §2's three steps; the reference executor is corrected (it dropped stale partials only at the end of a jump).
2. **One clock per rule (per shard), shared by its keys**, so a key first seen late starts at the rule's current time (ADR-0004 §1 gave each key its own clock from −∞).
3. **Timers fire on time when every input is quiet** (SEMANTICS §2.1's pin of 2026-11-01 holds; ADR-0004 §4's "an idle stream leaves the min" goes): a quiet stream's contribution moves forward.
4. **A match completes at its last event** when every absence deadline is already behind that event; only a match waiting out an absence window completes by timer (ADR-0013 §3).
5. **A quiet stream's clock runs from its own last event time at wall-clock speed** (≠ "the wall clock minus its bound", which loses a lagging source's events after every quiet spell): its floor = last event time + wall time elapsed since that event arrived − its bound, monotone; a stream that never sent anything starts from the deployment's start time. Wall time enters only through idle ticks that queue behind the events in the runtime's intake.
6. **Fail-closed when every source is down** (product decision): silence is silence, the alert fires on time; a per-source "no data" counter shows which source went quiet.
7. **Seal the instant:** once a key's timers at an instant have fired, a later-arriving event at that instant for that key is construct-late — dropped and counted — so live and replay differ only as counted drops (SEMANTICS §2.1).
8. **Skip-past-last compares in §2.2 order** `(ts, stream id, seq)`, not by timestamp alone (SEMANTICS §6.5): a partial that began after the completed match's last event survives, however the match completed.

### Owner decisions, 2026-11-04 — S3.1-2's plan check

S3.1-2 runs S3.1's routine (owner, above). All eight as recommended:

1. **A `timestamp` attribute arrives as RFC 3339 text** (the replay log's form): any UTC offset is converted to UTC; digits finer than a millisecond are rounded down to the millisecond; a JSON number is an event error.
2. **A `duration` attribute arrives as integer milliseconds**, under an `int` attribute's rules (no fraction or exponent, the full int64 range).
3. **An `object` attribute is a JSON object or array**, kept as its original JSON text and passed through; any other JSON type is an event error.
4. **A repeated key: the last one wins** (as `encoding/json`, `JSON.parse` and Python's `json`).
5. **The Unicode tables must match exactly**: the Go toolchain's `unicode.Version` equals x/text's `norm.Version` (≠ ADR-0007 §2's "major version", annotated there). Both are 17.0.0 with Go 1.27.3 and x/text v0.43.0.
6. **That check is a Go test in `internal/text`**, run by the `unit` row (as `docsync` and `allocgate`).
7. **The decoder's fuzz target runs in a second nightly row**, `fuzz-decoder`, shaped like `fuzz-parser`.
8. **Malformed UTF-8 in a JSON string becomes U+FFFD**, as `encoding/json` does; whether it also sets `text_invisibles` is decided in S7, with the rest of the text pipeline.

### Owner decisions, 2026-11-04 — S3.1-2's design (after the architecture review)

The review ([s3-1-2-design-architecture.md](../reviews/s3-1-2-design-architecture.md): 3 blockers, 12 should-fix, 9 later) and QA's notes on the decoder raised four questions, all answered as recommended; a fifth came from QA's tests at the build ([s3-1-2-design.md](s3-1-2-design.md) v2 places every finding):

1. **An event reads the tables as they were when it arrived** (review F1): the table state at its ingest position (intake order live, log position in replay), the same for every rule and shard that evaluates it, however long it waits in the reorder buffer. A change made after it arrived never decides it. SEMANTICS §4 is annotated; how the runtime carries the snapshot through the buffer is S3.2a's design.
2. **NaN and ±Inf are fixed in this sprint** (review F3, a defect older than S3.1-2; +0.5 point): any cast or arithmetic whose float result is NaN or ±Inf is an expression error, as ADR-0005 §3 says, test first.
3. **An `optional` or `object` primary-key column becomes a deploy error with a new code, built in S3.3** (review R3/F14): a SPEC v0.1 change-log entry there. Until then `internal/table` refuses an `object` key at creation and a null key value at write.
4. **Timestamps are read by a strict RFC 3339 reader** (review F5, QA A1; refines P1): exactly the standard's grammar — lowercase `t` and `z` accepted; a leap second `:60` read as the last millisecond of that minute; a comma fraction, `+24:00` and a one-digit hour are event errors. Hand-written in `internal/event`, which does not import `time`.
5. **Fences bind production code; a narrow override lets tests build fixtures** (≠ R: the recommendation was to check fences on production files only, as purity and globals are): `rules.json` gains `test_fences`, the twin of `test_imports` — `internal/expr`'s tests may also use `table.New` and `table.Table`, which exit 4's snapshot test needs; every other fence, the model one included, still binds expr's tests.

### Owner decisions, 2026-11-04 — S3.1-2's break-it

The break-it ([s3-1-2-breakit-qa.md](../reviews/s3-1-2-breakit-qa.md): 4 blockers, 3 should-fix) found two defects older than S3.1-2 in `internal/expr`:

1. **Both are fixed in this sprint's fix round** (as recommended; +0.5 point): an out-of-range float-to-int `cast` is a runtime error, not a machine-dependent value (B4, a replay-determinism blocker), and `math:round` is exactly half-up (B6).

### Owner decisions, 2026-11-05 — S3.2a's plan check

All four as recommended; the plan check's readings are under the S3.2a card:

1. **S3.2a runs S3.1's routine** (≠ the card's full loop): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it aimed at races, deadlocks and full mailboxes.
2. **S3.2a is split along its card's line:** S3.2a-1 = items 1, 2 (the replay format layer and partitions; exits 1, 2, 4); S3.2a-2 = item 3 (the runtime core; exit 3). Two sprints, two PRs, one at a time. Why: the concurrent runtime gets its own design review and break-it, and one PR of 11.5 points would likely pass the 100-commit limit (S3.1-2: 73 commits for 8 points).
3. **S1.1c runs right after S3.3:** after the clock freeze, before the pattern engine. §3's table gains its row; it is sized at its own G0.
4. **A re-forecast now:** §3's forecast paragraph is annotated; the next stays at S3.3's G5.

### Owner decisions, 2026-11-05 — S3.2a-1's design (after the architecture review)

The review ([s3-2a-1-design-architecture.md](../reviews/s3-2a-1-design-architecture.md): 1 blocker, 8 should-fix, 4 later) and the design's own questions (O1–O6) gave seven decisions. On D1 and D3 the owner asked first which option is algorithmically correct, then chose it. [s3-2a-1-design.md](s3-2a-1-design.md) v2 places every finding.

1. **D1 — an instant is finalized only once the construct clock is past it** (review F1; as recommended after the re-ask). Its events are released, and its timers of every kind fire, only when `clock > t`. Why: a watermark W promises only that nothing before W will still arrive, so an event exactly at W can still come.
   - **Results:** an event exactly at a deadline always defeats it (SPEC §6.4); same-millisecond events from different streams always go in declared order (SEMANTICS §2.2); when a walk runs, how messages are batched and how many shards there are never change a result (ADR-0010 §2 holds by construction).
   - **The seal goes:** S3.1-1's D7 seal is no longer needed and is retired.
   - **Cost, accepted:** the newest instant waits for the next event on every input, or a quiet stream's idle tick, so a match that completes on an event on a quiet stream waits up to `idle after`. Timer-driven alerts are unaffected.
   - **Scope:** this is an older defect in S3.1-1's merged clock and the reference executor, fixed in this sprint, oracle and tests first (about +2 points).
2. **D2 — an event line carries `ts` only when its stream has no time attribute** (O1, as recommended): a `time arrival` source, or a derived output. An event-time stream's time is its attribute alone.
3. **D3 — the format defines recording records now** (O2, as recommended after the re-ask): a recording carries the deployment's start, each event's arrival time, every idle tick, and a final stop record.
   - The driver applies them as live ingest does, in place of the virtual clock and +∞.
   - A log without them is a fixture, with the virtual clock and +∞ at the end.
   - S3.2b's recorder writes them. About +1 point.
4. **D4 — an `object` value is cleaned when decoded** (O3 + review F6, as recommended): whitespace between tokens is removed, member order kept, and malformed UTF-8 is replaced with U+FFFD, as P8 does for strings. Every reader then sees one form, and a log line never breaks. A change to S3.1-2's decoder, about +0.5 point.
5. **D5 — a timer sees the tables as at the arrival whose processing fired it** (O4 + review F2, as recommended). Every message that can trigger a walk (an event, another shard's advance, a tick) carries the router's snapshot, so the shard count never changes what a timer reads.
6. **D6 — replay streams and stops at the first fault** (O6; ≠ R twice: the recommendation was to check the whole log first, with a payload fault as an event error).
   - Steps run as lines are read; the first fault stops the replay at its line, and steps already delivered stand.
   - A fault is any of: an envelope fault, a wrong-typed field, a bad table record, or a payload that fails its stream's schema.
   - Live ingest still treats a payload fault as an event error.
7. **D7 — an `object` partition key becomes a deploy error with a new code in S3.3** (O5, as recommended), beside S3.1-2's D3 table-key error. Until then `partition` refuses it when a rule is created.

### Owner decisions, 2026-11-05 — S3.2a-1's break-it

The break-it ([s3-2a-1-breakit-qa.md](../reviews/s3-2a-1-breakit-qa.md): 0 blockers, 2 should-fix, 1 later) raised one owner question:

1. **K2 — a timestamp is refused at decode when its UTC instant is outside RFC 3339's years** (as recommended). An offset could carry a timestamp outside 0000–9999 once read in UTC (`0000-01-01T00:00:00+00:01` is 31 December of year −1), and the encoder then refused it. Live ingest makes such an event an event error; a replay log makes it a fault (D6).
2. **A flaky gate test is fixed in this sprint** (as recommended; ground rule 11). T4 failed once on S1.1a-1's `test_breakit_leak` (break-it QB4) under load. The cause, proven by delaying the module by 3 s, is a race in the test's own planted module: its helper reported "running" before the test registered the kill cleanup. The helper now writes its start byte only after a go-ahead that follows the cleanups. No gate code changed.

### Owner decisions, 2026-11-05 — S3.2a-2's plan check

S3.2a-2 runs S3.1's routine and follows the S3.2a split (owner, S3.2a's plan check). Both answers as recommended:

1. **The stop-and-drain mechanism is built in S3.2a-2.** This is ADR-0012 §3's drain barrier: the shard's drain message, every outbox handed off, the sink queues flushed, repeated to a fixpoint. It sits in the runtime core, under the same design review and break-it as the rest of the concurrency code; S3.3's `AppHandle.Stop` wraps it. About 1 point moves: S3.2a-2 6 → 7, S3.3 13 → 12.
2. **One single-threaded ingest step** in `internal/runtime` serves the live router and the replay driver (review R4 of S3.2a-1's design): Seq, the table snapshot, the stream registry, then the virtual floor or the ticks. S3.2a-1's driver is refactored to call it, so live and replay cannot drift. About +0.5 point.

### Owner decisions, 2026-11-05 — S3.2a-2's break-it

The break-it ([s3-2a-2-breakit-qa.md](../reviews/s3-2a-2-breakit-qa.md): 1 blocker, 1 should-fix, 3 later) raised one question:

1. **A retried `Drain` waits for the same flush** (as recommended). A second `Drain` of an already-drained app waits for the first drain's flush markers, under its own deadline, and returns the same result. A retry after a timeout then means "finish waiting", which S3.3's `Stop` needs; it used to return `ErrDrained` at once.

### Owner decisions, 2026-11-05 — the nightly build and CI's coverage

The nightly workflow's first scheduled run failed. Its fuzz rows passed, but its summary job found no records. `actions/download-artifact` with a `pattern` puts each artifact in its own directory only when more than one matches, and the nightly has one job. This is a latent defect of S2.2a's workflow generator. Both answers as recommended:

1. **A small separate PR now** (ground rule 11, stop the line): the generator downloads each job's artifact by name into its own `records-<job>-<attempt>/` directory, test-first. The nightly is proven by a run by hand on the branch. The PR also carries S3.2a-2's close-out.
2. **The race row is pulled forward from S3.2b into that PR:**
   - `go test -race ./...` on every PR, with cgo on for that row alone (ADR-0015 §3.6, residue item 9), and its planted-race proof;
   - a nightly row that runs `internal/runtime` 20 times under `-race`.

   goleak and the second OS stay in S3.2b (its item 7 shrinks to goleak and the tripwire; item 8 is unchanged). About +1 point.

### Owner decisions, 2026-11-05 — branch protection

1. **No required status check on `main`** (owner: "lets not do branch protection"; it had been open since the roadmap review of 2026-11-04). The `protect-main` ruleset (deletion and force-push blocked, 2026-11-02) stays as it is. The planned addition of `strict-gate` as a required check (S1.1a, "Still open for the owner") is dropped: the owner merges by hand after CI, and reports main's CI.

### Owner decisions, 2026-11-05 — S3.2a-2's design (after the architecture review)

The review ([s3-2a-2-design-architecture.md](../reviews/s3-2a-2-design-architecture.md): 2 blockers, 15 should-fix, 5 later) raised three owner questions, all answered as recommended; [s3-2a-2-design.md](s3-2a-2-design.md) v2 places every finding:

1. **F14 — table writes join the intake.** A live table write enters the same queue as events, and the router applies it in order, so an event never sees a write made after it arrived, and a recording can hold every write at its place. The write call waits until the router has applied it; S3.3's `Table` API uses this path. SEMANTICS §4's "intake order live" now includes writes.
2. **F5 — a drain ends the app's input.** After its drain, `Send`, ticks and table writes for that app are refused, and its state stays readable. This matches Stop = stop sources, drain, detach (ADR-0012 §3), and a recording's stop record is always its last. S3.3's `Start` after `Stop` builds a fresh app.
3. **F10 — a body's panic resets that key**, as a purge does. Its state and timers go; its buffered events stay and start it fresh. The panic is counted, and the walk continues for every other key. `partition` recovers it, so its invariants hold (ADR-0012 §6 annotated).

### Owner decisions, 2026-11-06 — S3.2b's plan check

Seven questions, asked in plain language with the options ranked by the accuracy of what a rule detects; all seven as recommended. The readings are under the S3.2b card.

1. **A plain filter rule's alert goes out as soon as its stream accepts the event** (Q1). Before, every rule waited for its stream's clock to pass the event, which patterns need and a filter does not.
   - **Why:** a lone alert on a quiet stream could wait for the next event, hours later (example 07: a grant at 10:00, the next at 13:00, the 10:00 alert at 13:00). An accepted event is never dropped, and replay gives the same alerts as live.
   - **Cost, accepted:** a rate limit compares event times in arrival order. When two events for one key arrive out of order inside one interval, one alert can stand where the written rule gives two: with `first every 10 min`, 10:11 then 10:00 arriving give one alert.
   - **Rejected:** waiting only on rate-limited rules — exact, but late by the stream's lateness allowance, it needs `idle after` on their streams (a language change), and an event arriving late after a quiet spell is dropped; waiting always, as before.
   - The idle-key purge stays on the clock; the deploy check (TTL ≥ every rate-limit interval) means a purge never changes an alert. SEMANTICS §8 and SPEC §5.1 are annotated with S3.2b-1's design. About +0.5 point.
2. **S3.2b runs S3.1's routine and splits along its card's line** (Q2): S3.2b-1 (items 1, 2, 6, 9; about 7 points), then S3.2b-2 (items 3–5, 7, 8 and the recorder; about 6). Why: each half gets its own design review and break-it. As one sprint it would fit the 100-commit limit (PRs #10 and #11: 41 and 26 commits), so that is not the reason.
3. **`text_invisibles` can be read on every stream, and a declaration of it other than a non-optional `bool` is a deploy error, E-TYPE-001 at the declaration** (Q3, review F6 and F7 of S3.1-2). The engine's own flag is the one value read. Today an undeclared read is E-NAME-001, and a wrong-typed declaration slips through and can make a rule silently never match. A language change: SPEC's change log and an invalid example, in S3.2b-2.
4. **Exit 8's zero allocations cover every per-event stage** (the ingest step, the key, the clock, the filter, the rate limit), measured on the one-thread replay path (Q4). The live hand-off from the router to a shard gets a measured per-event budget as a tripwire, and ADR-0010 §1 gets a dated note. Rejected: reusing hand-off buffers across goroutines, the bug class ADR-0010 §4 rejects for events.
5. **The second OS of the determinism proof is macOS** (Q5): free for a public repo, a different OS and processor from Linux CI, and the toolchain already installs there.
6. **`go.uber.org/goleak` v1.3.0** (Q6; approved 2026-11-01, ADR-0015) enters `go.mod` in S3.2b-2, test-only, MIT. Its own tests' modules enter `tools/deps-allowlist.json` as "goleak's graph": testify v1.8.0 (MIT), go-spew v1.1.1 (ISC), go-difflib v1.0.0 (BSD-3), yaml.v3 v3.0.1 (MIT and Apache-2). A scratch module showed `go.mod` gains goleak alone.
7. **The tripwire's advisory ceilings** (Q7; ADR-0015 §3.11): partial-count `none` until patterns exist (S4.1); sink-depth 1,024, the sink queue's capacity; drained_pending 0, since a filter rule holds no timers. The outbox ceiling stays blocking at 65,536.

### Owner decisions, 2026-11-06 — S3.2b-1's design (after the architecture review)

The review ([s3-2b-1-design-architecture.md](../reviews/s3-2b-1-design-architecture.md): 2 blockers, 7 should-fix, 3 later, each proven with a scratch program) and QA's test material raised six questions, asked in plain language with the options ranked by accuracy. [s3-2b-1-design.md](s3-2b-1-design.md) v2 places every finding.

0. **`plan`'s tests may read `.cep` programs through the parser and the validator, and `playback`'s through the parser** (as recommended): test files only, in `tools/importcheck/rules.json`'s `test_imports`. Tests then run the real rule text, not a hand-written copy of the parser's output.
1. **D1 — a plain rule forgets a key on its own stream's time** (review F2, as recommended). Its keys purge on the stream's admission watermark (and at the drain and the end), never on the floors that idleness or the virtual clock raise.
   - **Why:** in a replay with two streams, or live after a stream goes quiet, the rule clock can already be past an accepted event; purging on it re-opened a suppression. With `first every 5 min`, grants for one account stamped 08:30 and 08:32 on a stream lagging another at 10:00 gave two alerts; the rule gives one.
   - **Cost, accepted:** on a quiet stream keys stay until its next event or the stop.
   - **Rejected:** starting the expiry from the rule clock (fewer duplicates, some remain); accepting duplicates.
2. **D2 — example 07 is limited per tenant** (review O2, as recommended): `partition by tenantId of ConsentGrants`. A tenant's alert no longer silences another tenant's for 5 minutes. Examples 05 and 09 keep showing a shared limit; the corpus gains its only partitioned plain rule. A spec-corpus change.
3. **D3 — a derived stream's `ts` declared as anything but `timestamp` is a deploy error with a code** (review F9 and O3, as recommended): E-TYPE-001 at the attribute. The engine fills `ts` with the output time, which a text or number field cannot hold. A language change.
4. **D4 — a select that can be null, bound to a non-optional output attribute, is a deploy error** (review F9; ≠ R: the recommendation was to send the alert with the field empty and warn at deploy). The author wraps it in `coalesce(...)` or declares the attribute optional. Output fields always honour their declaration, and no alert is lost at run time. A new code, E-NULL-003, and a language change; no corpus program is rejected (read 2026-11-06).
5. **D5 — D3 and D4 land in S3.2b-1** (as recommended): the planner relies on them, so it never sees those cases. S3.2b-1 grows from about 7 to about 10 points: D3 and D4 +2, D1 +0.5, the review's other fixes about +0.5.

Not an owner decision, recorded for the record: the rate anchor lives in the body's per-key state, as ADR-0006 §2's 2026-11-05 annotation (from S3.2a-1's design v2) already records; design v1 had moved it without saying so (review F5).

### Owner decisions, 2026-11-06 — S3.2b-1's review and break-it

The combined-build review ([s3-2b-1-combined-architecture.md](../reviews/s3-2b-1-combined-architecture.md): 0 blockers, 3 should-fix, 6 later) and the QA break-it ([s3-2b-1-breakit-qa.md](../reviews/s3-2b-1-breakit-qa.md): 1 blocker, 2 later) raised three questions, asked in plain language with the options ranked by accuracy:

1. **Multi-part partition keys are supported** (break-it B1; ≠ R: the recommendation was to refuse a stream keyed twice and put multi-part keys in the backlog). `partition by k of S, j of S` passed the validator and the planner kept only the last key, so different (k, j) pairs merged and alerts were lost. Each combination of key parts is its own key: its own rate limit, state and expiry; a null in any part drops the event for that rule, counted; parts may differ in type, matched position by position across streams.
   - **Syntax: grouped per stream, `(k, j) of S`** (as recommended), so a swapped pairing between two streams is visible at review. Naming one stream twice is then a deploy error pointing to that form.
   - **No limit on the number of parts** (≠ R: the recommendation was up to 4, keeping keys fixed-size and routing allocation-free). The single-part key, the common case, stays allocation-free; a multi-part key may allocate.
   - **Until then (as recommended): S3.2b-1 refuses a stream named twice with E-PART-004**, so no alert is lost in between; the rule stays once grouped keys arrive.
   - **The owner: "make sure what I decided is implemented in near future. don't drop anything."** Multi-part keys are S3.2b-2's, with their language-design step first.
2. **D3's error points at the `ts` attribute itself** (combined review C2; ≠ R: the recommendation was to keep the stream's declaration line and annotate D3). Model attributes gain a source position, and the stored model goldens are regenerated. S3.2b-2; until then the error names `ts` at the stream's declaration.
3. **The sprint's work is split into parts of about 10 points each** (the owner, answering where multi-part keys land). S3.2b-1 = what is built plus this fix round, about 10.5. S3.2b-2 = its own items (about 6) plus multi-part keys (about 2.5) plus D3's position (about 1), about 9.5.

*Id mapping for the records above (annotation, 2026-11-02):* v2's S1.1 → v3's S1.1a (runner, CI, coverage, event kernel) + S1.1b (analyzers, LAW Δ) — so "S1.1" in the coverage and parallel-run decisions above means S1.1a, and their exit lines are S1.1a's exits 3, 4 and 6; v2's S2.1 differential → S2.2a; v2's S2.2 → S2.2a (expressions) + S2.2b (validator); v2's S3.1 playback format → S3.2a; v2's S3.2 → S3.2a (runtime core) + S3.2b (replay loop, errors, arrival, flags); v2's rate limiting → S3.2b; v2's S4.1 and-groups, live quiet key and drain → S4.2.

### Owner decisions, 2026-11-06 — the docs restructure

Asked to bring the docs up to date so that old information is read only when it is needed, with links backward and forward. As recommended unless marked.

1. **The sprint plan holds only the live plan.** Closed cards and the old status lines move word for word to [history.md](history.md), the owner's decisions to this file. At close-out the closed card moves there and the status line is replaced, the old one appended to history's log.
2. **Every point-in-time document opens with a "Historical" line** naming what supersedes it and linking its siblings and the sprints before and after.
3. **Stale status is rewritten; a changed decision is annotated** (ground rule 4).
4. **The changes land as their own PR now** (≠ R: the recommendation was to ride S3.2b-2's PR), together with S3.2b-1's close-out commit; S3.2b-2 starts from `main` after it merges.
5. **Known issue S2.2a-1 is closed:** the fuzz rows are blocking since PR #13, and fuzzing nightly is the chosen schedule, not a defect.
6. **The one-module check** ([ADR-0001](../architecture/adr/ADR-0001-module-and-package-layout.md) §1) **joins S1.1c's list.**
7. **SPEC §0 names its current revision**, rev 4; the language is unchanged.
8. **Three lasting rules from retros join the sprint model** ([§10](00-sprint-model.md#10-the-quality-bar)): a design claim about a library's behaviour is proven by a scratch program before review (S3.1-2); a sprint that adds a CI workflow runs it once by hand and reads its summary (S3.2a-2); a guard test counts only once it is shown red with the line it guards removed (S3.2b-1).
9. **The docs link checker becomes a gate row in this PR** (≠ R: the recommendation was S1.1c): `doclinks`, at T1–T4, fails on any relative link or `#anchor` in a tracked Markdown file that does not resolve.

### Owner decisions, 2026-11-06 — the code graph re-indexes itself

Asked to make sure `gitnexus analyze` runs automatically when the code changes.

1. **Nothing at OS level** (≠ R: a macOS launchd job watching `.git/logs/HEAD` was proposed first; the next recommendation was git hooks in `.githooks`). No launchd, systemd or cron job.
2. **A Claude Code hook keeps the index at HEAD**, as recommended: [`gate/hooks/gitnexus_reindex.py`](../../gate/hooks/gitnexus_reindex.py), registered in `.claude/settings.json` for every prompt and for every Bash call that runs git. When HEAD differs from the commit the GitNexus registry records, it starts `gitnexus analyze --index-only` in the background. A `git pull` in the owner's own terminal is picked up at the next prompt, before the graph is used.

### Owner decisions, 2026-11-06 — S3.2b-2's plan check

The owner released the sprint the same day ("lets resume the work in current sprint"). Three questions, asked in plain language with the options ranked by the accuracy of what a rule detects and what a replay reproduces; all three as recommended. The readings are under the S3.2b card.

1. **A recording keeps a rejected live event as a "rejected" line** (Q1). Live, a payload that fails its stream's schema is dropped and counted (SEMANTICS §10). The recorder writes a line naming the stream, the arrival stamp and the reason; replay counts it and skips it, so replaying a recording gives the live run's alerts and its error counts. A format change: a record kind beside S3.2a-1's D3 records, about +0.5 point.
   - **Why:** SEMANTICS §10 lets live and replay differ only in counted construct-late drops.
   - **Rejected:** leaving it out (the same alerts, but the replay's error count lower than live's); writing the payload as received (replay stops at its line under S3.2a-1's D6, so every alert after it is not reproduced).
2. **When the recording cannot be written, detection goes on** (Q2). The recording stops at the failure; the engine counts the failure and reports where the recording broke. No alert is lost; the recording is incomplete from that point.
   - **Rejected:** pausing intake until the recording can be written (alerts late, sources blocked, possibly for hours); stopping the app (every alert after the failure lost).
3. **Grouped partition keys have one spelling** (Q3). A single key is written without brackets, `k of S`; a group is two or more different attributes in brackets, `(k, j) of S`; every stream in one clause names the same number of parts. `(k) of S`, an attribute named twice in one group, and a part-count mismatch between streams are each a deploy error whose message shows the valid form. Each gets its code, its entry in SPEC rev 5's change log and an invalid example.
   - **Why:** SPEC's "one way only" rule: one spelling is easier for a generator to write and a reviewer to read. Detections are the same either way.
   - **Rejected:** accepting `(k)` as `k` and an attribute repeated in a group as named once. A part-count mismatch is an error under either option: positions cannot be matched.

### Owner decisions, 2026-11-06 — S3.2b-2's design (after the architecture review)

The review ([s3-2b-2-design-architecture.md](../reviews/s3-2b-2-design-architecture.md): 2 blockers, 10 should-fix, 8 later, each behaviour claim proven by a scratch program; the coordinator re-ran the proofs on a fresh copy) raised three questions, and the answers raised two more. Asked in plain language with the options ranked by accuracy. [s3-2b-2-design.md](s3-2b-2-design.md) v2 places every finding.

1. **The Go entry points refuse every value a recording cannot copy exactly** (review F1; ≠ R: the recommendation refused broken values and tidied harmless ones). A typed event is refused with an error to its caller when its time field differs from its timestamp attribute, a string holds malformed UTF-8, an object's text is not valid JSON already in its compact form, or a timestamp is outside the years 0000–9999. Nothing is changed silently; a recording always replays to the live run's alerts.
2. **When the recording falls behind, it stops; detection never waits** (review F2, as recommended). The recorder writes on its own goroutine behind a bounded queue; a full queue ends the recording, counted and reported with the record where it stopped, as a write error does (S3.2b-2's Q2). **The owner added: a product setting turns recording off entirely.**
3. **A recording keeps each payload as sent**, its whitespace removed, undeclared fields included (review question C, note open point 4; ≠ R: the recommendation recorded declared fields plus counts). Replay decodes the same bytes and so counts unknown fields and flag overrides exactly. Recordings can hold fields no rule reads.
4. **Recording is off unless turned on** (≠ R: the recommendation was on unless turned off). A deployment that says nothing records nothing; the setting is the deployment's (S3.3's `Deploy`, then `cepd`).
5. **S3.2b-2 stays one sprint** (as recommended): about 12.5 points after the review's fixes and these answers, under the 13-point cap. The gate checks land with the code they guard.

Not owner decisions, recorded for the record: the tripwire keeps a ceiling per metric with an optional per-rule override, so every metric and rule pair has a ceiling as ADR-0015 §3.11 requires (review F10, question D); the review's F19 (skip ticks for apps with no idle stream) is rejected, since a caller's tick raises the stamp that later `time arrival` events take.

### Owner decisions, 2026-11-06 — S3.2b-2's review and break-it

The combined-build review ([s3-2b-2-combined-architecture.md](../reviews/s3-2b-2-combined-architecture.md): 1 blocker, 3 should-fix, 3 later) and the QA break-it ([s3-2b-2-breakit-qa.md](../reviews/s3-2b-2-breakit-qa.md): 4 blockers, 3 should-fix, 1 later; two blockers the review's too) raised two questions, asked in plain language; both as recommended. The coordinator re-ran every behaviour finding: all reproduce.

1. **Stopping an app waits for its recording only until the stop's deadline** (review C4). If the recording has not finished by then — a stuck disk — it is marked failed and the stop still reports success, every alert having gone out. Rejected: not waiting at all (the recording's status not final when the stop returns); today's behaviour (the stop fails with a timeout although detection did not).
2. **The per-rule burst gauge (`RuleStats.OutboxHigh`) is left out of the live-equals-replay check** (break-it S3). It measures what one worker holds at once — a capacity mark, like the queue high-water marks — so it depends on the number of shards; alerts and every detection counter must still match exactly, and the runaway-rule ceiling keeps working per worker. Rejected: counting it across all workers (more bookkeeping on the hot path).

### Owner decisions, 2026-11-06 — the open items after S3.2b-2

Asked for the decisions open for the owner; four questions in plain language, all answered as recommended.

1. **Two lasting rules join the sprint model** ([§10](00-sprint-model.md#10-the-quality-bar)), both from S3.2b-2's retro: a fix round's changes run under the race detector before T4 (a fix there made a data race found only at T4); a sprint that adds a CI job opens its PR as a draft to get that job's first run, and marks it ready after reading it (the gate workflow runs on pull requests only).
2. **The spec-cleanup pass runs now, before S3.3**, as sprint S3.2c: the five contradictions of known issue S2.2b-1, the two notes of the grouped-key evaluation, and integer overflow (S3.2b-1-1). Why: the spec rule writers and generators read is made consistent before more is built on it, and overflow is a correctness bug, which ground rule 10 keeps open no longer than 30 days. Rejected: inside S3.3 (at its cap); before the release (overflow open for weeks).
3. **Integer overflow is an expression error for that rule** (known issue S3.2b-1-1): as division by zero is, the event is dropped for that rule only and counted; other rules see it. No rule acts on a wrong number. Rejected: clamping to the largest value (a selected value is wrong); wrapping, documented (a huge product turns negative and the alert is missed). A language change: SPEC's change log and a test, in S3.2c.
4. **S3.2b-2's close-out goes out as a small docs PR now**, so main's status line is current, and the merged branch `s3-2b-2/integration` is deleted from GitHub.

### Owner decisions, 2026-11-06 — S3.2c's plan check

Three questions, asked in plain language; all three as recommended. The readings are under the S3.2c card.

1. **S3.2c's card is approved as written:** the language documents agree with each other and with the code, every reserved word is listed in the spec, and integer overflow never yields a wrong value. About 3 points. Out of scope: overflow in the aggregates (S8, when they exist) and timestamps past year 9999 in outputs (S5.1's sink encoding). *(Annotated 2026-11-07, owner, [S3.2c's review and break-it](#owner-decisions-2026-11-07--s32cs-review-and-break-it), 2: a timestamp **outside years 0000–9999** in an output — before year 0000 as well as past 9999.)*
2. **The routine: QA's tests first, one review, a break-it.** A one-page design note and the build; one architecture review of the design and the code together; a short QA break-it on overflow; one fix round (run under the race detector before T4); the full gate. Rejected: the full loop (a design review before the build, for one rule applied to nine operations); the lighter routine (nobody trying the overflow boundaries).
3. **The overflow rule covers every 64-bit operation:** ints, durations and timestamps alike — `+`, `-`, `*`, `/` (the smallest int divided by −1) and unary `-` on ints, `abs`, duration ± duration, duration × int, int × duration, duration / int, unary `-` on a duration, timestamp ± duration. An overflow is an expression error for that rule, counted. Rejected: ints only (a wrapped duration or timestamp can flip a comparison).

### Owner decisions, 2026-11-07 — S3.2c's review and break-it

The combined review of the design and the code ([s3-2c-combined-architecture.md](../reviews/s3-2c-combined-architecture.md): 0 blockers, 3 should-fix, 4 later; the overflow code correct, every finding in the documents) and the QA break-it ([s3-2c-breakit-qa.md](../reviews/s3-2c-breakit-qa.md): 0 blockers, 1 should-fix, 1 later) raised three questions, asked in plain language. The coordinator re-checked every finding: all hold.

1. **The spec lists exactly the arithmetic the engine does, and `timestamp − timestamp → duration` is built now** (review R2; ≠ R: the recommendation struck it from SEMANTICS and left it for later). SPEC §5.10 gains `int × duration`, `duration / int` and unary `-` (int, float, duration), which the engine already accepts, and `timestamp − timestamp → duration`, new in rev 6, checked for overflow like every other 64-bit operation. Rejected: leaving the spec behind the engine as a known issue.
2. **The S5.1 forward covers both ends: a timestamp outside years 0000–9999 in an output** (review R3; as recommended). `timestamp − duration` can land before year 0000 as easily as `timestamp + duration` past 9999, and neither is RFC 3339. Rejected: past 9999 only.
3. **The smallest int, −9223372036854775808, is a literal a rule can write** (break-it K2; ≠ R: the recommendation forwarded it to a later language sprint). A minus sign written directly before an integer literal belongs to it where the number fits only with its sign — a param default and an expression alike; the grammar is unchanged. Rejected: a known issue with `-9223372036854775807 - 1` as the expression's workaround and none for a param default.

### Owner decisions, 2026-11-07 — S3.2c's close-out

Two questions, asked in plain language; both as recommended.

1. **The close-out lands as its own small docs PR, and the merged remote branch `s3-2c/integration` is deleted.** Rejected: the close-out riding S3.3's PR (main's status line stale until then).
2. **A lasting rule: a test parses every rule fragment the language documents quote** (S3.2c's retro, from review R1: a correction wrote `-> and not` into SEMANTICS, which the grammar refuses). The rule joins the sprint model now; the test, about half a point, joins S1.1c, the gate-extras sprint after S3.3. Rejected: building it in S3.3 (a full card); not adopting it (a wrong form reaches main whenever a sprint has no review of its documents).

### Owner decisions, 2026-11-07 — S3.3's plan check

Five questions, asked in plain language; three as recommended. The readings are under the S3.3 card.

1. **S3.3 runs in two parts** (as recommended): with the runtime's per-app lifecycle (reading 1) and the forwards on the card (reading 6), it comes to about 15 points, over the 13-point cap. **S3.3-1, about 8 points:** the public facade `cep` (`Runtime`, `Config`, `Deploy`, `AppHandle` `Start`/`Stop`/`Undeploy`, `Runtime.Close` in reverse deploy order, several apps in one running engine, `Input` with `SendRow` only, the `Table` API, `BuildInfo`), the connector contracts (`io`, package `cepio`) with `io/inmemory`, the API gate (item 5), D3's and D7's deploy errors, `Deploy`'s recording setting, `Deploy` not mutating the checked model. **S3.3-2, about 7 points:** the expect sandbox, the conformance runner with E-EXP-001 and the six planted proofs, an unknown attribute in an expect block a fault, `gate/regen-baseline` and the Tier-2 baselines, the clock freeze. Between the two merges, `Deploy` does not run expect blocks. Rejected: one sprint at about 15 (over the cap); one sprint with the baselines moved to S4.1 (the corpus outputs unlocked until then).
2. **The export check and the clock freeze are gate checks of their own now** (as recommended): `apipartition` turns the gate red on any change to the public API until `api/exports.golden` is updated; a clock-golden row turns it red on any change to `internal/clock`'s exported signatures until `api/clock.golden` is updated. S1.1c's LAW Δ block lists both when it is built. Rejected: building the LAW Δ block in S3.3 (about +2 points); moving both to S1.1c (the first public API and the clock contract unguarded for a sprint, and the clock no longer freezing at S3.3).
3. **`apidiff` runs now as well as the export list** (≠ R: the recommendation kept the export list alone until the first release). `golang.org/x/exp`'s `apidiff` compares the public API against the merge base at T4 and labels each change compatible or breaking. **A new build-time dependency, approved:** `golang.org/x/exp`, pinned in `go.mod` and `tools/deps-allowlist.json` (approved in principle in ADR-0009 on 2026-11-01). Rejected: the export list alone until v0.1.0.
4. **An expect block's lists interleave by time** (as recommended; material gap M1). Within one `given` list, the written order is the arrival order, even where its times go backwards (example 07's last block). Across lists, the next event is the earliest head of any list, each list keeping its own order; a tie goes to the list written first. Rejected: list by list as written (a whole stream arriving after another, which never happens live).
5. **S3.3-1 runs the full loop with the design panel** (≠ R: the recommendation was S3.2b-2's routine, one architecture review of the design). QA's tests first and independent; a design note, reviewed by the sprint model's blind three-reviewer panel (a breaker re-implementing from the sources, a challenger measuring the alternatives, the architect with the-fool) and then QA for testability; the build in the main session; the architecture review of the combined build and a QA break-it; one fix round under the race detector; T4. At most two agents at once (ground rule 17), so the panel runs in two waves. Rejected: S3.2b-2's routine; S3.2c's lighter one.

### Owner decisions, 2026-11-07 — S3.3-1's design (after the panel)

The blind panel on the design note v1 — the breaker ([s3-3-1-design-breaker.md](../reviews/s3-3-1-design-breaker.md): 2 blockers, 10 should-fix, 2 later), the challenger ([s3-3-1-design-challenger.md](../reviews/s3-3-1-design-challenger.md): 1 blocker, 4 should-fix, 3 later), the architect with the-fool ([s3-3-1-design-architecture.md](../reviews/s3-3-1-design-architecture.md): 2 blockers, 15 should-fix, 7 later) — and QA on testability ([s3-3-1-design-qa.md](../reviews/s3-3-1-design-qa.md): 1 blocker, 10 should-fix, 4 later; a 41-row test table). The coordinator verified every finding named below against the source. Seven questions, asked in plain language with the options ordered by the accuracy of the software; six as recommended.

1. **A stopped app stays stopped; to run it again, deploy it again** (architect A5; as recommended). `Start` after `Stop` is `ErrStopped`. *Annotates F5 of S3.2a-2 (2026-11-05, "S3.3's `Start` after `Stop` builds a fresh app"): the fresh app is the caller's second `Deploy`, so its empty tables, fresh counters and new recording are explicit.* Rejected: `Start` rebuilding a fresh app under the same handle (a reset nobody can see).
2. **A table's starting rows can be given at deploy time** (architect A4; as recommended): `WithTableRows`, applied at `Start` after the recording's header and before any source opens, so the first event never reads an empty table. Rejected: filling after `Start` with the gap documented (a false alert, or a missed one, in the first milliseconds).
3. **A failed delivery is retried by the destination, never by the engine** (architect A7; as recommended): the engine's flusher counts and drops, as built since S3.2a-2; S5.1's HTTP sink retries with backoff itself. *Annotates ADR-0008 §2 and SEMANTICS §10's "retry with backoff per sink config".* Rejected: the engine's flusher retrying every destination (a slow destination holds its stream's queue; one retry rule for all).
4. **Each output's canonical text is a fresh copy the destination may keep** (challenger C3; ≠ R: the recommendation was a method writing into the destination's own buffer). `OutEvent.Canonical []byte` is allocated per output. One allocation and about 13% on the encoding step; nothing to keep by accident. Rejected: the shared buffer valid only during `Write` (a destination keeping it reads the next output's text under the old one's name).
5. **A connector named in the rule file but not yet built is refused at deploy, unless the Go program binds its own for that stream, which replaces it** (QA F9, architect P2; as recommended). The message names the connector and the sprint that builds it. Example 01's `@sink(type = 'http', …)` deploys in tests and in the owner's check by attaching an in-memory destination; S3.3-2's expect sandbox never opens connectors and ignores the annotations. *(Annotated 2026-11-08, the build: example 01 itself cannot deploy before S4.1 — its pattern body is not built — so the test carries its annotation on a plain-rule program; the owner's check uses example 09.)* Rejected: deploying with a warning (an app that looks healthy and delivers nothing); deploying silently.
6. **The batch-close signal is recorded now** (breaker S7; as recommended; the challenger, the architect and QA recommended waiting for S8). The recording format already defines the `batch_end` record (`internal/playback/reader.go`, `driver.go`); the runtime writes it from this sprint, so every recording carries the punctuations SEMANTICS §10 names, and a replay against a later group rule gives the live answer. About a quarter point. Rejected: accepting and counting the signal without recording it.
7. **`apidiff` turns the gate red only when the public list did not change with the code** (as recommended): an incompatible change with `api/exports.golden` updated in the same change passes, the report printed for the PR. Rejected: red on every incompatible change until the first release (an override per deliberate change, no added safety before a release).

**Decided as recommended by the panel, unless the owner objects** (the coordinator; each in the design note v2): one allocation per `SendRow` above `runtime.Send`'s, the values slice the engine keeps (every reviewer's blocker); the app list copy-on-write behind an atomic pointer; shards build and free their own per-app state on `mAdd`/`mRemove`, with rollback; sink queues never closed (a last marker ends the flusher); the runtime, not the facade, starts and stops a source's goroutine (`archcheck` forbids `go` in `cep`); `Drain` of a never-started app returns at once; a per-app stop quiets the ticks before cancelling the sources (D3 of S3.1-1 honoured); `AttrType` on `RowWriter` and `RowReader` now; `ConnectorFactory`/`WithConnectors` now; `Deploy(ctx, name, src string, …)` kept; functional options kept, a repeated one a deploy error; `Table.Upsert`/`Delete` variadic, `Load` a multi-row upsert; `Stats.Sinks` per binding; `ErrStarted`; the facade never imports `internal/model` (the law's `""` row drops it); `apipartition` prints with positions cleared, exported fields only, every exported method whatever its receiver, and refuses an embedded field in an exported type; `apidiff` per package with the verdict read from its report text, a missing merge base a pass that says so. **A dependency surface the owner should see:** the approved `golang.org/x/exp` tool brings `golang.org/x/mod`, `x/sync` and `x/tools` into `go.mod` (four pinned, build-time) and ten more modules into its graph, never built (`x/net`, `x/sys`, `x/telemetry`, `go-cmp`, `goldmark`, `x/tools/go/expect`, `x/tools/go/packages/packagestest`, …); all listed in `tools/deps-allowlist.json`.
