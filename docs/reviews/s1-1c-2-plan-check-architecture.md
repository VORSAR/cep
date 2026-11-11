> **S1.1c-2 plan check review, 2026-11-11 — architecture, with the-fool.** Read blind against the plan check's v1 (`s1-1c-2/integration` @ 50cf0f9; code `main` @ fd274e4); every finding's disposition is in [the plan check's §10](../sprints/s1-1c-2-plan-check.md#10-reviews-of-this-plan-check). Siblings: [architecture](s1-1c-2-plan-check-architecture.md) · [QA](s1-1c-2-plan-check-qa.md) · [product](s1-1c-2-plan-check-product.md) · research: [govulncheck and the canary](s1-1c-2-research-vuln-canary.md), [the rule fragments](s1-1c-2-research-fragments.md).

# S1.1c-2 plan check (v1, 50cf0f9): architecture review, with a the-fool pre-mortem

Reviewer: architecture (blind to QA and product). Read-only. GitNexus: index at fac4e64 (the brief's statement; the context resource could not be read from this agent). `detect_changes` (compare main) reports 25 changed doc sections, 0 code symbols and 0 processes, so the code under review is main fd274e4. Hindsight: two searches, on the summary channel and on test waits; both returned only generic pages, and nothing from them is used here.

## Verdict (5 lines)

1. The direction is right and the research is unusually good. Law data lands at G0, QA goes first, the teeth come last, and both fetched inputs follow the offline-row rule. The seams mostly hold.
2. **One blocker:** §0.4 is wrong. `io/inmemory/breakit_s3_3_1_test.go:47,54` call `time.After`, so `testhygiene` lands red on the real tree. That is the same class as the blocker at S1.1c-1's plan check.
3. The gate is drifting toward parallel mechanisms. The plan adds a second summary channel and records schema 3 where decision 4's LAW Δ kinds would carry the same facts, and those kinds are themselves missing for the new law files. There are now three expiry clocks, and TEMP `issue=` is an unverifiable waiver that never expires.
4. The grammar refactor's real risk is the error-code contract, not acceptance. `parseError` keys on participle's production names, and the two unproven productions (`ForBound`, `RateClause`) sit on exactly those keys.
5. G0 should wait until A-B1 is fixed and the owner has the missing questions (split now; one channel or two; TEMP `issue`; `testhygiene` over `gate/proofs/`). Totals: 1 blocker, 15 should-fix, 14 later.

## Findings

### Blocker

**A-B1. `testhygiene` lands red on the real tree, because §0.4's "zero calls in any Go test today" is false.**
- *Evidence.* `git grep -nE "time\.(Sleep|After|AfterFunc|Tick|NewTimer|NewTicker)\(" -- '*_test.go'` finds two calls:
  - `io/inmemory/breakit_s3_3_1_test.go:47: case <-time.After(10 * time.Second):`
  - `io/inmemory/breakit_s3_3_1_test.go:54: case <-time.After(5 * time.Second):`
- archcheck passes both today because `clock()` returns early for `io` (`tools/archcheck/main.go:253-262`, `callsAllowed`).
- The plan gives the row "an empty exception list" (§0.4), and the card's acceptance line 10 requires every new row green on the real tree. The S1.1c-1 plan check's shared blocker was this same class: the `deps` check was red on the real tree (decisions.md, "2026-11-11 — S1.1c-1's plan check").
- *Disposition.*
  - Correct §0.4.
  - Add a red-first item to lane C: run the row and see it red on these two lines, then rewrite both waits as state waits capped by `context.WithTimeout` / `<-ctx.Done()`. This is ADR-0015 §2's "every waiting test takes a ctx deadline" and keeps testing.md rule 1's "generous cap that fails loudly". The failure messages stay as they are.
  - Add §6 archcheck (8): today's tree with those lines → red naming both.
  - The alternative is a Go "one wait helper", which ADR-0015 §2 names and testing.md:5 says does not exist. Ask the owner only if lane C prefers that route. Recommendation: ctx deadlines, so there is no helper and nothing to waive.

### Should-fix

**A-S1. The grammar refactor puts the error-code contract at risk, not acceptance, and the plan's proof list does not cover that contract.**
- *Evidence.* `parseError` picks the code from participle's message text, which names productions. The `expects` helper is at `internal/parser/parse.go:149-151`; the mappings are:
  - `expects("PosElem")` → E-PAT-005 (`:182`)
  - `expects("DurationOrParam") || expects("Duration")` → E-DUR-001 (`:186`)
  - `expects("\"for\"")` → E-PAT-004 (`:188`)
  - the `clauseStart` loop over `expects("\"" + word + "\"")` → E-ORD-001 or the missing-clause codes (`:191-197`)
- R-F F3 says `RateClause` and `ForBound` "were not tested (inferred)":
  - `ForBound` wraps exactly the `'for'` literal that E-PAT-004 keys on (`grammar.go:164`).
  - `RateClause` wraps `DurationOrParam`, which E-DUR-001 keys on (`grammar.go:108`).
- The differential compares verdicts, not codes. `gate/runner/rows/differential.py:40` shows the code only inside a message.
- `impact(parseError, upstream)` reaches `Deploy` (`deploy.go`) and `tools/shape`, so this is the public deploy error path.
- The invalid corpus has one file per code (for example `e-pat-004-absence-without-for.cep`). A path that no file covers can change its code silently, for example `output first every 5` → E-DUR-001 reached through `RateClause`.
- *Disposition.*
  - §2 S3 and §6 fragments (11): "Parse unchanged" means the same verdict **and the same code and position** for every `parseError` branch.
  - QA's W0 adds a table test with one input per `expects(...)` key, reached through each wrapped clause (`for`, `output first every`, `within`, `after match`, `late events`, partition keys). Prove it can fail by renaming a production in a scratch run.
  - Lane B owns `parse.go` outright, not "only if Q14".

**A-S2. The plan builds a second summary channel and schema 3 where decision 4's LAW Δ kinds would carry the same information, and it misses those kinds.**
- *Evidence.*
  - Decision 4 of 2026-11-10: "The LAW Δ block carries a semantic delta per structured law file … one small parser per structured file, each with a planted-edit proof". ADR-0015 §1.3's own example one-liner is "clockcheck allowlist: +`internal/pattern`".
  - Plan §4 lists `tools/allowlists/` and `gate/known-issues.json` "as `code`", and §3's `law.py` row adds only `instruments` and `gomod`. The `code` kind prints one line per manifest entry, grouped last (S1.1c-1 design decision 4), so a new waiver reads as `tools/allowlists/: 1 file +6/−0`.
  - Meanwhile S2 adds `notes.txt`, `notes: list[str]`, records schema 3, a required `notes` argument to `summary.render` (GitNexus context: callers `summarize.summarize`, `scheduler.Runner._close`, and nine tests) and `NEEDS_HISTORY += suppressions`. It also creates its own risk 9 (the schema move breaking CI's aggregation).
- *Disposition.* A question for the owner, recommendation first:
  - **(a) One channel.** Add `allowlist` and `known-issues` kinds to `law.py` (lane D owns it in W3). Each prints per-entry one-liners, for example `purity.json: +internal/x: sync.Once PERMANENT ref=… S4.1 (3 entries)` and `known-issues.json: BA10 decision set`. Each gets a planted-edit case in the `lawdelta` driver.
    - The `suppressions: N (+Δ this diff)` line is computed by `lawdelta`, which already owns the merge base and its no-merge-base FAIL, through `suppressions.py`'s pure counter.
    - The vulndb date goes in the `govulncheck` row's reason.
    - Result: no `notes.txt`, no schema 3, no new `NEEDS_HISTORY` entry, and risk 9 disappears. ADR-0015 §2 already puts `API Δ` inside the LAW Δ block, so this follows the ADR's own precedent.
  - **(b) The plan's notes channel**, still adding the two kinds, because decision 4 requires them either way.

**A-S3. A TEMP entry with `issue=` is an unverifiable waiver that never expires: in effect a PERMANENT exception with no `ref`.**
- *Evidence.*
  - S1 says "`issue` is `#<n>` or an https URL (format only: the gate has no network)".
  - Decision 6 of 2026-11-10 says "No issues API". The project's issues are known-issues.md ids (`S1.1c-1-1`, `BA10`).
  - So `TEMP issue=#999` passes forever and traces to nothing.
- *Disposition.* A question for the owner. Recommendation: `issue=` must name an id in `gate/known-issues.json`. It then resolves offline, and the waiver inherits item 7's two-closed-sprints clock through that entry. DEVELOPMENT.md:75's `issue=<#id|url>` is annotated. The other option is to drop `issue` and allow `expiry` only.

**A-S4. The `ref` grammar is inconsistent across the policy, the plan's example and the resolver the plan reuses.**
- *Evidence.*
  - DEVELOPMENT.md:69's example is `ref=ADR-0015#2` (a short id and a section number).
  - The plan's S1 example is `"ref": "docs/architecture/adr/ADR-0012-…md#5"`.
  - The plan's rule says "the `doclinks` resolver", which uses GitHub slugs (`gate/runner/rows/doclinks.py:35 slug`, `:61 anchors`). A heading `### 5. …` slugs to `5-…`, so `#5` never resolves.
  - The plan also limits `ref` to "an ADR file", while DEVELOPMENT.md allows "ADR-nnnn or docs path#anchor". Most owner decisions live in decisions.md; Q11's PERMANENT `teeth.json` entries would `ref` the owner's answer there.
- *Disposition.*
  - P1 fixes one form. Recommendation: a repository path plus a GitHub-slug anchor, resolved by the same function `doclinks` uses, and allowed to point at an ADR or a decisions.md heading.
  - Annotate DEVELOPMENT.md's three examples.
  - §6 grammar gains a case: the `ADR-0015#2` short form → red naming the accepted form.

**A-S5. The shared anchor resolver has no owner and lives in a row module.**
- *Evidence.*
  - Lane A (`allowlists.check(resolve=…)`, W1) and lane E (`aging.aged(resolve=…)`, W2) both need it.
  - The resolver is `slug`, `anchors` and `Links` inside `gate/runner/rows/doclinks.py:35-107`. A pure package importing a row module inverts the layering in DEVELOPMENT.md ("A rule lives in one pure package").
  - §4 gives `rows/doclinks.py` to no lane.
- *Disposition.* In §3/§4, lane A extracts `gate/runner/anchors.py` in W1's first commit (pure: `slug`, `anchors`, `resolves(...)`) and owns `rows/doclinks.py` for that move. Lane E consumes it.

**A-S6. The `govulncheck` row's nightly job does not warm the module cache, so `go tool govulncheck` cannot build under `GOPROXY=off` on a cache miss.**
- *Evidence.*
  - `gate/ci/generate.py:52`: `NEEDS_MODULES = frozenset({"deps", "proofs"})`, with the comment at `:50-51`: "so a cold cache is never a verdict".
  - The new row is nightly-only (job `nightly`, as `fuzz-parser` is) and contains neither of those rows.
  - R-V G2 proved its offline scans only with the "cache warmed by `go mod download all`".
  - Plan S6 adds only `NEEDS_VULNDB`.
- *Disposition.* In S6, add `govulncheck` to `NEEDS_MODULES`, and have `test_generate.py` pin the nightly job's download step.

**A-S7. The database check accepts a stale database.**
- *Evidence.* S6 checks only that `index/db.json` parses and "carries a `modified` date"; nothing bounds its age. R-V G2 shows `modified` is a daily timestamp. An old zip (a reused local path, or a cached or mis-served file) scans clean against an old world.
- *Disposition.* Add to S6 and §6 govulncheck (9): `modified` more than N days before the run date (recommend 7) is a named red. Print the date in the row's reason.

**A-S8. The law manifest is itemised where ADR-0015 §1.3 says `gate/` whole, so the next new gate directory — this sprint's own `gate/canary/` — falls outside the law.**
- *Evidence.*
  - `gate/law.json` lists nine `gate/…` entries; §0.10 adds ten more by name.
  - `gate/canary/` (lane F, W3) is in no list. Pin rule (4) reaches only instruments' `code` and `proofs` paths, and the canary is a workflow, not a registry row.
  - `gate/runner/law.py:121-133` (`entry_for`) already takes an exact entry first, then the longest prefix. So `gate/` as a `code` catch-all can sit beside `gate/registry.json` (kind `registry`) and the rest.
- *Disposition.* At G0, add `gate/`, `tools/`, `.githooks/` and `Makefile` as `code` catch-alls. The specific entries keep their kinds and their own lines. Add the corpus directories as §0.10 says. The pin keeps rule (4).

**A-S9. The `instruments` schema mixes row instruments with Go-test instruments, and its rule (4) quietly makes engine test files law.**
- *Evidence.*
  - The S4 example `docsync-d` has `row: "unit"`, `lands_in: "S1.1c-1"` and no tiers.
  - Rule (1) says "the registry's row ids equal the instruments' `row` set". Rule (2) says "each pinned row … with the pinned `lands_in`". `unit` lands in S1.1a-1 (registry), so every Go-test instrument is red.
  - Rule (4) needs `internal/parser/errorcounts_test.go`, `fragments_test.go`, `fragment.go`, `internal/runtime/nosleep_test.go` and the allocgate tests under law paths. None is in `law.json`, and none is in §4's G0 list.
- *Disposition.*
  - S4 names two entry shapes:
    - a row instrument: `{id, row, tiers, lands_in, code, proofs | exempt}`;
    - a Go-test instrument: `{id, row: "unit", go_test, lands_in, code, proofs}`, checked by rules (3)–(5) only.
  - Rule (1) becomes "every registry row has exactly one row instrument".
  - §4 lists the engine test files as `code` law paths. The owner is told plainly that engine lanes will then edit law paths, and LAW Δ will show it.

**A-S10. `testhygiene` reaches into `gate/proofs/`, which ADR-0015 §2.1 puts out of scope for every analyzer.**
- *Evidence.*
  - ADR-0015 §2.1: "proofs — `gate/proofs/` … Deliberately defective code; out of scope for every analyzer".
  - The plan's P2 says "every `_test.go` but `bench/`, `gate/proofs/` included", and §6 archcheck (2) plants in `gate/proofs/internal/…_test.go`.
  - No owner decision records this. It is green today: `gate/proofs/internal/{below,control,leaky,racy}/*_test.go` hold no timer calls. But a future planted timing defect would need a waiver of the very instrument it proves.
- *Disposition.* A question for the owner. Recommendation: exclude `gate/proofs/`, as the ADR says. The other option annotates §2.1.

**A-S11. About 140 markers will bury the four substantive SEMANTICS edits in LAW Δ, which is the owner's one floor.**
- *Evidence.*
  - Q4 says about 140 markers across SPEC and SEMANTICS.
  - A doc line names at most three headings plus "(and n more)" (forward (c); today only the first, `law.py:176-193`).
  - The four corrected spans (§0.5a: SEMANTICS :98, :128, :136, :180) change language-document text, which GR-14 says needs an owner decision.
- *Disposition.*
  - In §4/§8, the markers go in one commit and the four corrections in a separate commit. The owner's check lists each correction by line, before and after.
  - Recommend that `doc_line` count hunks that only add `<!--cep…-->` comments separately, as `markers +n`, so a substantive hunk always surfaces (lane D owns `law.py`).

**A-S12. Q14 does not say which error code the incident will get, and its new file moves a pinned corpus count that the plan states wrongly.**
- *Evidence.*
  - §0.5b and §6 (11) say "the T4 differential (63 files)". The real count is 73: `git ls-files ':(glob)docs/language/examples/**/*.cep' ':(glob)docs/language/eval/generated/**/*.cep' ':(glob)docs/language/invalid/*.cep' | wc -l` → `73`. 63 is the row's `min_wants`.
  - `gate/proofs/drivers/test_differential_swap.py:53` pins `(73, 73)`. Q14's new `docs/language/invalid/` file makes it 74, so the `proofs` row goes red until that pin moves, and no lane owns the driver.
  - The natural mapping, E-PAT-005 (`parse.go:182`), changes the code a known input gets from E-ORD-001. That is a change to the diagnostic contract, under decision 7's "no language semantics change".
- *Disposition.*
  - Q14 states the code. Recommendation: E-PAT-005, an existing code, so the catalogue does not change. State that errors.md is unchanged.
  - Add a coordinator item in §4: `test_differential_swap.py:53` → `(74, 74)`, landing with the file.
  - Correct "63" to "73".

**A-S13. The card has 0.5 points of headroom against a +27% precedent, and the plan does not ask the owner whether to split before starting.**
- *Evidence.* `s1-1c-1-record.md:124`: "Card 8 (items ≈ 5.5 at the plan check, ≈ 6 at G0); as built ≈ 7". S1.1c-2 is about 12.5 against a cap of 13, and Q9's fixes "would add ≈ 1, past the cap". Ground rule 6: "Split before starting, never mid-sprint."
- *Disposition.* A new question, recommendation first. Either:
  - take this review's simplifications (A-S2 (a) removes the notes channel and schema 3; A-L13 removes the special case Q12 would add to liveness; A-S15's data-file option for ruff) and re-size at v2; or
  - split now on the card's line: items 1, 2, 4, 5, 9 against 3, 6, 7, 8. That also separates the only engine-code change (the grammar) from the gate core.

**A-S14. Q9's recommendation forwards BA13 a third time, which leaves a false green inside the mechanism that runs every proof the teeth will count.**
- *Evidence.*
  - known-issues.md:10 (BA13): "the shim's pass line shares stdout with the module under test, so a module that prints that line and then ends the process with `os._exit(0)` passes without running a test".
  - It was found at S1.1a-1, and 11 closed rows have passed since (`gate/sprints.json`).
  - This sprint builds the teeth (item 5) and ends in a break-it aimed at the gate's silent passes (decision 7 of 2026-11-10).
- *Disposition.* Re-frame Q9. Recommendation: fix BA13 here, using its own forward ("the shim writes its report to a file the wrapper names", about 0.25), and forward BA10 and QB10 with recorded decisions.

**A-S15. Q7 omits the option that best fits "one grammar".**
- *Evidence.* The project's established pattern is a data file pinned to its config or document: `rules.json` ↔ ARCHITECTURE §3, `sprints.json` ↔ the plan table, and this sprint's own `known-issues.json` ↔ known-issues.md. Q7's recommended option puts metadata in TOML comments, which is a third syntax beside the JSON lists and `//nolint`.
- *Disposition.* Add and lead with this option: `tools/allowlists/ruff.json`, in the one grammar, pinned both ways to `ruff.toml`'s `per-file-ignores` keys by the suppressions row. S1.1c-3's golangci exclusions then follow the same pattern.

### Later

- **A-L1.** `ParseFragment` lives in a production file (`fragment.go`), but only a test uses it. Put the entry points in a `_test.go` file of package `parser` (no production surface, no coverage dilution; ADR-0015 §3.6 residue 2), unless a production consumer is named. The seven productions stay in `grammar.go`.
- **A-L2.** archcheck's `DisallowUnknownFields` defines the grammar's key set a second time, in Go. Recommendation: Go reads only `rule` (which must match) and `entries[].target`, and the Python validator stays the one owner of the grammar.
- **A-L3.** An allowlist's `rule` should come from a closed set, each value read by a named checker (purity, clock, teeth, govulncheck, ruff). Otherwise a list that no checker reads never has its liveness checked.
- **A-L4.** The reason given for running aging at T4 ("so a lane's T2 is never red by the calendar") is wrong. Aging depends only on tracked files, so it is deterministic per commit. The calendar check is TEMP `expiry`, which the plan runs at T1–T4. State both choices deliberately.
  - Also, §6 aging (2) counts the first-seen row itself, which contradicts S5's "follow its first_seen row". Fix the off-by-one in P-notes.
- **A-L5.** §0.12 says "13 closed plan rows have passed since"; it is 11 (`gate/sprints.json`: 12 closed of 26, S1.1a-1 first).
- **A-L6.** The canary job should restore caches but never save them (`actions/cache/restore`), so an unpinned toolchain's artefacts never enter a cache the gate's jobs restore. Parity and the hash-keyed tool paths already limit the damage.
- **A-L7.** Raise `ci-fresh`'s `min_wants` from 2 to 3 when `canary.yml` joins its scope.
- **A-L8.** Q4's rule-like-span detector should read the lexer's keyword table (the one pinned to `grammar.lark`), never a hand-written list. Keyword changes already need a sync in cep-vscode.
- **A-L9.** Count Go `cases` by reusing `tools/archcheck` (already `go/parser`, already law), not a new one-file helper or a Python regex over Go (P5).
- **A-L10.** DEVELOPMENT.md:69-71's `sprint=S1`, `S4` and `S5` are not plan rows. Under the plan's rule (a row at or before the marker), the policy's own examples are red. Annotate them in P1.
- **A-L11.** At T0, archcheck checks a snapshot of the index. P2 should state that the allowlists are read from that same tree.
- **A-L12.** Switching the `testhygiene` registry command from its stub to `go run ./tools/archcheck -tests {files}` when lane C lands is a coordinator item that §4 does not list.
- **A-L13.** Q12's recommendation order is inverted. The leading option adds a special case to liveness for a planned file; ADR-0015 §2.2's lands-with-subject rule, and the plan's own hedge, favour dropping the entry now. Lead with the drop: S9.1 adds the entry with `internal/window/cron.go` (sprint-plan.md:295 is S9.1; §6 archcheck (7) says "S8", which is wrong).
- **A-L14.** Q8 could offer a middle option: `deps-allowlist.json` stays its own register but gains `ref` and `sprint`, checked by `allowlists.py`'s field validators. That is one validator set over two schemas, and it brings GR-10's "traces to a decision" to dependencies.

## Questions for the owner that this review adds

1. Split now, or simplify and re-size at v2? (A-S13)
2. One channel (LAW Δ kinds) or two (a notes channel and schema 3)? (A-S2)
3. Does TEMP `issue=` name a known-issue id, or is `issue` dropped? (A-S3)
4. Is `gate/proofs/` in `testhygiene`'s scope? (A-S10)

Also re-framed: Q7 (A-S15), Q9 (A-S14), Q12 (A-L13) and Q14 (A-S12).

## Holistic: what the project's history says

1. **"Green on the real tree" is again asserted, not run.** This is the third time:
   - S1.1c-1's plan check: `deps` was red on the real tree (`kr/pretty`, `check.v1`).
   - The 2026-11-10 scope review: two instruments red on landing by construction.
   - Now `testhygiene` (A-B1).

   The research runs proved `govulncheck`'s state on main, but nothing proved the other new rows. Recommendation: §0 cites a run (or a grep) for every new row's real-tree verdict before review. That costs minutes.
2. **Size history.** S1.1c-1's card carried 2.5 points of buffer (8 against items of 5.5) and built at about 7, after owner additions in the fix round. S1.1c-2 carries 0.5. The fix round's additions are the pattern, not the exception: S1.1c-1 took six law paths, the `replace` rule and the rename count there.
3. **The law model is growing sideways.** After this sprint it holds:
   - `registry.json`, `law.json` (paths and instruments), `sprints.json` and `current-sprint`, `tripwire.json`, `known-issues.json`, `notbuilt.json`;
   - `tools.json`, `deps-allowlist.json` (its own schema), `rules.json`, `tools/allowlists/*.json` (the one grammar), and ruff waivers in TOML comments;
   - records carrying `law` and `notes`;
   - three expiry clocks: sprint reached (`notbuilt`), closed-row count (aging) and calendar (TEMP `expiry`).

   The data-file ↔ Markdown pin is the one coherent pattern; the rest is accreting. Recommendation: design note P1 opens with a single "law model" table (file, kind, owner, checker, LAW Δ one-liner, pin, clock). That makes the model reviewable, and it is what the instruments pin only approximates.
4. **The S1.1a-1 forwards are 11 closed rows old.** BA10, BA13 and QB10 have pointed at "a later gate sprint" since 2026-11-03. This is that sprint. BA13 is a false green beneath the teeth (A-S14).
5. **The one-version rule couples gate tools to the engine's published module.** `go.mod` already carries `x/mod` and `x/tools` indirect pins (owner, during S1.1c-1's build). Q3 adds `x/telemetry`, and every future tool in `gate/proofs` can raise the engine's minimums again. That is acceptable now, but worth a line in Q3 and a look before S12 (the public release).
6. **Repeating precedent well.** These choices follow precedent and should stay: stubs that fail by name; law data at G0; the Go test kept on lane B's branch until it is green; the offline fetch through a generated setup step; and the `sprints.json`-style pin for known issues.

## Pre-mortem (the-fool): it is May 2027 and S1.1c-2's instruments have failed the owner

**F1 — The teeth counted methods, not bite.** (Likelihood: medium. Impact: high.)
- *Story.* By S6, a lane hollowed `test_govulncheck_planted.py`: the assertions were weakened to `assertIn("", out)`, but all 8 `test_*` methods were kept. `cases` stayed 8, the pin stayed green and LAW Δ showed `gate/proofs/drivers/: 1 file +3/−3`, grouped last. A real vulnerable module then shipped through a green nightly.
- *Root assumption.* That the number of planted cases measures a proof's strength.
- *Mitigated by the plan?* Partly. The count stops deletion, not hollowing.
- *Add.* Every sprint's break-it plants one hollowed driver. The design note states that `cases` is a floor and that strength comes from first-seen-red.

**F2 — The markers became the escape hatch.** (Likelihood: medium. Impact: high.)
- *Story.* SPEC rev 4 added 30 spans. An agent marked the awkward ones `<!--text-->`, and one of them was a new `-> and not` variant. The fragment test stayed green, because `text` is a legal marker.
- *Root assumption.* That authors classify spans honestly.
- *Mitigated by the plan?* Partly: Q4 requires every span to be marked, but any marker will do.
- *Add.* A `text` span that parses at any kind is red ("it is grammar: mark its kind"), and LAW Δ counts marker changes by kind (A-S11).

**F3 — The nightly cried wolf until the owner stopped reading it.** (Likelihood: high. Impact: medium.)
- *Story.* `-scan module` (Q5) went red on a vulnerability in a graph-only module of the gate's tool module that had no fixed version. TEMP entries were extended every month, and in March a reachable net/http finding was accepted the same way.
- *Root assumption.* That every module-level finding deserves the same handling.
- *Mitigated by the plan?* Partly (TEMP with `expiry`).
- *Add.* The row's reason separates symbol-reachable findings from module-level ones. A waiver for a reachable finding cannot be TEMP-by-expiry: it needs an issue (A-S3) and therefore an owner decision through aging.

**F4 — The canary went silent.** (Likelihood: medium. Impact: medium.)
- *Story.* A regeneration changed the cron line, which moved the notification recipient. Notifications were also email-filtered. The canary failed weekly from January to April and nobody saw it, because silence reads as green. Ruff 0.19's format change landed as a surprise inside S7's sprint and cost a day.
- *Root assumption.* That an alarm with no heartbeat will be heard.
- *Mitigated by the plan?* Only for the first run (Q10, and the owner's check of the notification setting).
- *Add.* Name the canary's silence in ADR-0015 §3.6's residue, accepted knowingly, and make "the canary's last run read" a line in every close-out.

**F5 — The waivers traced to the ADR, not to a decision.** (Likelihood: high in S1.1c-3. Impact: high.)
- *Story.* S1.1c-3's 308 findings were reduced by roughly 60 PERMANENT entries, each with `ref` to an ADR-0015 heading. Each `ref` resolves, so each passes. LAW Δ showed `tools/allowlists/: 3 files +240/−0`, and the owner approved it in one read.
- *Root assumption.* That a `ref` which resolves proves the waiver is relevant.
- *Mitigated by the plan?* No.
- *Add.* Per-entry one-liners in LAW Δ (A-S2), so each waiver is read by name. S1.1c-3's plan check caps PERMANENT additions per package, or asks the owner per group.

### Inversion: what would guarantee failure, and does it exist now?

| Guaranteed-failure condition | Present in v1? |
|---|---|
| New rows red on landing, forcing ad-hoc exceptions mid-sprint | **Yes** (A-B1) |
| Mechanisms whose output never reaches the one place the owner reads | **Partly**: the notes sit beside LAW Δ, waivers appear as `code` lines and markers swamp the SEMANTICS line (A-S2, A-S11) |
| A trust anchor that can be fooled, sitting beneath the new teeth | **Yes**: BA13 forwarded again (A-S14) |
| An overrun that forces a cut mid-sprint, against ground rule 6 | **At risk**: 0.5 points of buffer against a +27% precedent (A-S13) |
| A waiver path with no clock and no trace | **Yes**: TEMP `issue=` (A-S3) |

Every one of these has a concrete fix above, and none needs more scope than the plan already carries.

## Files

- `/Users/ramith/code/cep/docs/sprints/s1-1c-2-plan-check.md` (reviewed)
- `/Users/ramith/code/cep/io/inmemory/breakit_s3_3_1_test.go` (A-B1)
- `/Users/ramith/code/cep/internal/parser/parse.go`, `/Users/ramith/code/cep/internal/parser/grammar.go` (A-S1)
- `/Users/ramith/code/cep/gate/ci/generate.py` (A-S6)
- `/Users/ramith/code/cep/gate/runner/law.py`, `/Users/ramith/code/cep/gate/law.json` (A-S2, A-S8)
- `/Users/ramith/code/cep/gate/runner/rows/doclinks.py` (A-S4, A-S5)
- `/Users/ramith/code/cep/gate/proofs/drivers/test_differential_swap.py` (A-S12)
- `/Users/ramith/code/cep/docs/product/known-issues.md` (A-S14)
