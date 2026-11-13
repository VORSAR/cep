> **Historical — S1.1c-4, the scans; merged in PR #40.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-13--s11c-4s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-4-plan-check.md) · [design](../sprints/s1-1c-4-design.md) · [record](../sprints/s1-1c-4-record.md) · reviews: [plan check architecture](s1-1c-4-plan-check-architecture.md), [plan check qa](s1-1c-4-plan-check-qa.md), [design architecture](s1-1c-4-design-architecture.md), [design qa](s1-1c-4-design-qa.md), [combined architecture](s1-1c-4-combined-architecture.md), [breakit qa](s1-1c-4-breakit-qa.md). **← [S1.1c-3](../sprints/s1-1c-3-record.md) · [S1.1c-5 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-4 plan check, QA review (blind), 2026-11-13** — the QA reviewer on [the plan check](../sprints/s1-1c-4-plan-check.md) v1 @ a488bc9 (code `main` @ 7c65dbf). Every finding is placed in the plan check's §10.

# S1.1c-4 plan check (draft v1, a488bc9): QA review

I reviewed the draft blind and read-only. Code citations are to 7c65dbf. My measurements use a scratch copy of today's database (`vulndb.zip` sha256 3b16a8d3…, `modified` 2026-11-11T02:51:11Z, the same bytes as §0.4's re-measure), a scratch matcher built on x/mod v0.42.0 `semver`, and x/vuln v1.9.0's source from R-V2's module cache. All of it is under `scratchpad/s1-1c-4-pc/qa/`.

Tools: GitNexus `impact` was used on `rows/deps.gather`. The GitNexus context resource, the Skill tool (`the-fool`) and Hindsight were not in this agent's toolset, so I ran the pre-mortem by hand (at the end) and did no memory searches.

**Verified as written:**
- §0.2's three entries: over the union of both build lists, a correct matcher finds GO-2026-5320 (goldmark v1.4.13), GO-2026-6860 and GO-2026-6866 (x/net v0.61.0). It finds nothing over the 7 compiled modules plus `stdlib` and `toolchain` at 1.27.4.
- §0.3's compiled set: participle, x/text, goleak, x/exp, x/mod, x/sync, x/tools.
- §0.9: one `tool` line, apidiff.
- §0.13: `TREE_CAP_S = 300` at `parity.py:38`.

## Blockers

**QA-B1 — The plan never names the `introduced: "0"` edge, and that edge decides detection on real data.**
- **Evidence.** A matcher that reads `"0"` as `v0.0.0` (the natural reading of "versions without `v`") gets real data wrong both ways:
  - **Missed detection in a compiled gate tool.** In Go semver a pseudo-version `v0.0.0-<time>-<hash>` sorts *below* `v0.0.0`. x/exp is compiled at `v0.0.0-20261106192419-47ce03d25ef0`. I planted one entry on x/exp: `introduced 0, fixed 0.0.0-20270101000000-abcdefabcdef`. The naive matcher printed nothing; the correct one printed `GO-9999-0001 golang.org/x/exp v0.0.0-20261106192419-…`.
  - **False reds on today's tree.** Over today's 23-module union the naive matcher reports 12 x/net entries and 1 x/sys entry, against the correct 2 and 0. GO-2020-0014, GO-2021-0078, GO-2022-0969 and others are `introduced 0, fixed 0.0.0-2019…`. Under Q1's other options these would be red forever.
  - **govulncheck's rule.** `"0"` is minus infinity, sorted first and never compared (x/vuln v1.9.0 `internal/semver/affects.go:58-90`: `if v1 == "0" { // -inf case. return true }` and `affected = e.Introduced == "0" || !Less(v, e.Introduced)`).
  - **What the plan has.** Its edges ("introduced − 1, introduced, fixed − 1, fixed") have no "introduced − 1" when introduced is `"0"`. §6 (15) and (16) contain no pseudo-version-below-`v0.0.0` case. Under Q1, x/net and x/sys are graph-only, so a real-tree run cannot catch the false reds. Only the table can.
- **Fix.** Add to §6 and to the agreement table, each row with govulncheck's measured verdict:
  - (a) x/exp at its pinned pseudo-version against a real `introduced 0` entry → affected.
  - (b) x/net v0.61.0 against GO-2020-0014 (`fixed 0.0.0-20190125091013-…`) → not affected.
  - (c) a module at `v0.0.0-<time>-<hash>` against `introduced 0, fixed 0.1.0` → affected.
  - (d) an events list given out of order, with `"0"` not first.
- State in §2 S1: `"0"` is minus infinity and is never parsed as a version. In the driver, plant x/exp's entry as `introduced 0` so that a naive matcher fails by name.

**QA-B2 — The agreement table's measurement mode is unspecified, and every available mode either cannot measure the synthetic rows or disagrees with the plan's own matcher.**
- **Evidence (x/vuln v1.9.0 source).**
  - **Source mode** (`-scan module`) loads only the current directory's package (R-V2 §2a). So every row needs a real, downloadable version that builds under go1.27.4. Synthetic edges (an invented pseudo-version, old `+incompatible` majors that no longer build) cannot be measured.
  - **Source mode never reports `toolchain`.** Only `stdlib` is added as a module (`internal/vulncheck/packages.go:30-46`); `GoCmdModulePath` is used only to print versions (`internal/scan/util.go:46`).
  - **Query mode** (`-mode query -format json mod@ver`, `internal/scan/query.go`) does take arbitrary module@version strings, and is the one mode that gives a module verdict for a synthetic build list. But it has three problems:
    - It does **not** skip withdrawn entries. That skip lives only in `vulncheck.go:136-140`, and the plan's matcher skips them.
    - `client.byModule` builds `filtered` and then **returns `entries`** (`internal/client/client.go:266-292`). So once one entry matches, every entry that passed the index pre-filter comes back, unaffected ones included.
    - It pre-filters on `index/modules.json`'s `fixed` (`:251-255`), not on the entry's ranges.
  - Without the mode named per row, the table either gets its verdicts typed by hand (tautological: the matcher held to itself) or encodes govulncheck's bugs, which the matcher must then copy.
- **Fix.**
  - In §2 S1 and P1, name the mode for each kind of row:
    - query mode against a planted database holding **exactly one** unmodified real entry and its index lines, so the unfiltered return cannot add entries;
    - source mode for the withdrawn rows and for the `GOVERSION` conversion (go1.27.3 on `PATH`, as R-V2 did; query mode refuses `stdlib@go1.28rc1`, `parseModuleQuery`).
  - Record the three query-mode divergences as named rows where the matcher deliberately differs (withdrawn → skipped, as source mode does).
  - Say that `toolchain` rows are held to govulncheck through query mode only. Source mode cannot reach them.
  - Quote each command and the exact result in the test data.

## Should-fix

**QA-S1 — "covered = modules read + 2" cannot pass the runner, and nothing guards an empty module set.**
- **Evidence.** `verdict.decide` reds `covered N ≠ expected M` (`gate/runner/verdict.py:57-58`). `expected` is the expansion count of `expected_scope` (`step.py:78`). The only scope kinds are `git`, `go` and `go-vet-units` (`scope.py:107-117`), so no scope counts modules. This is the same mistake the aging driver flagged at S1.1c-3 (`test_aging_planted.py:11-14`).
- **The vacuity hole.** If the row instead prints a constant (as deps prints 3, `rows/deps.py:137`), then a compiled-set listing that comes back empty, through `-e`, a template or environment slip, scans only `stdlib` and `toolchain` and passes.
- **Fix.**
  - In §2 S1 and §0.13: give the row's `expected_scope` (for example `git go.mod gate/proofs/go.mod`) and a matching covered count. Or add a new scope kind as a named law change.
  - Make an empty compiled set, or one missing a main module's known edge, a named red.
  - Counter-example: the listing returns no modules → red "no module read".

**QA-S2 — The planted driver has no seam for the database, so it cannot own its world, and CI's `proofs` job has no copy.**
- **Evidence.** The copy lives at `~/.cache/vorsar/cep/vulndb/` (§2 S1). A world inherits `HOME` (`support.py:184`; only `VORSAR_CEP_TOOLS_CACHE` and a few others are hidden, `:35`). Drivers "never write the shared tool cache" (`support.py:4-5`). `make vulndb` runs only in "jobs holding `vulns`" (§2 S1 CI), and the driver runs in job `proofs`.
- **Consequence.** A driver that reads the real copy is green locally and red in CI, or depends on how old the developer's copy is. A driver that writes the real copy breaks testing.md rule 3.
- **Fix.**
  - Name the seam, for example `VORSAR_CEP_VULNDB` (the precedent is `manifest.cache_root`, `manifest.py:76-82`). Add it to `support.HIDDEN`, and print the path read in the row's reason.
  - Every driver case plants its own zip and stamp, dated from the UTC today with 5-day margins (the aging precedent).
  - Counter-example: the seam unset in a world → the row reads the world's `HOME`, never the developer's.

**QA-S3 — Acceptance line 2's named proof, `test_deps.py`, cannot fail when the tool-line wiring is removed.**
- **Evidence.** `test_deps.py` imports only `allowlists`, `deps` and `sprintid` (`:12`). It tests the pure `compare` with `imported` handed in. GitNexus `impact(gather, upstream, includeTests)` shows one test caller: `test_breakit_s11c1.py:DepsGather.test_a_replace_directive_is_named`.
- **Fix.**
  - Add a `gather`-level test in that style (patching `run`, `test_breakit_s11c1.py:95-107`): a `go mod edit -json` reply with two `Tool` paths → both tools' modules are marked imported.
  - Add a case to `test_deps_planted.py`: a second `tool` line whose module is in the offline cache and reaches a module apidiff does not → a wrong licence is red, named.
  - Name both in §7 for line 2, red-proven against today's `TOOL` constant.

**QA-S4 — §6 is missing the database's real shapes and the version edge cases.** Each item below needs a named red, or an agreement row (measured today, `scratchpad/s1-1c-4-pc/qa/scripts/survey.py`):
- **`+incompatible` on the event side.** 1,527 event versions carry it (`GO-2021-0072` `2.7.0-rc.0+incompatible`; `GO-2021-0073` `2.1.1-0.20170519163204-f913f5f9c7c6+incompatible`). §6 (15) only has it on the module side.
- **stdlib pre-release introduced events.** GO-2026-6617 has `introduced: "1.27.0-0"`. Test it against `go1.28rc1` → `1.28.0-rc.1` and against a `1.28.0-0` edge.
- **An affected block with no ranges, or with only non-SEMVER ranges.** govulncheck counts these as **affected** (`affects.go:13-33`), and the matcher must agree.
- **An unknown event key** (`last_affected`, `limit`) → named red. None exist today.
- **An unparseable version on either side.** x/mod's `semver.Compare` ranks an invalid string lowest, which fails open. Make it a named red. The same goes for `GOVERSION` forms `go1.27.4 X:…` and `devel …`: x/vuln's `GoTagToSemver` returns `""` for them.
- **One entry naming two modules** (GO-2026-6617: `stdlib` and x/net) → two findings.
- **35 entries with two affected blocks for one module**, stdlib among them (GO-2023-2185). §6 (15) has one such case; take it from the real set.
- **A `replace`.** govulncheck matches at `Replace.Version` (`vulncheck.go:131-133`), and `gather` carries `Module.replace` (`rows/deps.py:70-72`). So `replace dep v1.2.0 => dep v1.0.0`, with v1.0.0 vulnerable, is red. A replace to a local directory is named.

**QA-S5 — The plan never defines what a waiver matches, so one id can silence a second module.**
- **Evidence.** §2 S1 says "a finding is red unless `tools/allowlists/govulncheck.json` lists it". A finding's key is not given. Ids span modules (GO-2026-6617 names `stdlib` and x/net), so a waiver keyed by id alone quietly waives the second module.
- **Fix.**
  - `target` = `<id> <module>`, with the version in the reason.
  - Counter-examples: a waiver for (GO-X, stdlib) while GO-X also hits a compiled module → that module red; a target that does not parse → red; one waiver per finding.

**QA-S6 — The stamp's `sha256` and `url` are written but never checked.**
- **Evidence.** §2 S1's `check_db` checks the stamp's presence, form, future date and age. It does not compare the stamp with the zip. A refreshed stamp beside an old zip (a hand edit, or a half-written pair from `make vulndb`) then reads as fresh.
- **Fix.**
  - `check_db` also checks: `sha256(zip) == stamp.sha256`; `url` is the canonical one; `modified` is not after `fetched` and not in the future.
  - One red counter-example for each.

**QA-S7 — The day boundaries and their seam are under-specified.**
- **Evidence.** §6 (10) tests 8 days and 31 days only. "Fetched" is a timestamp; `today` is a date (`rows/__init__.py:27`).
- **Fix.**
  - State the rule as UTC calendar days.
  - Unit tests with `today` injected: 7 → green, 8 → red; 30 → green, 31 → red; a fetch at 23:59:59Z and at 00:00:01Z on the same boundary; a stamp one second in the future.
  - Driver cases keep 5-day margins.

**QA-S8 — A failed fetch in CI hides every lint row.**
- **Evidence.** §2 S1 runs `make vulndb` as a step before the runner. A failed step ends the job before `gate/bootstrap.py exec` runs (`generate.py:79-89`). Every lint row then reads "missing" in `strict-gate` (`generate.py:93-96`), and the cause sits only in a step log. That works against testing.md rule 4 ("shows every failure in one summary").
- **Fix.**
  - The vulndb step gets `continue-on-error: true`.
  - The row reports "no copy: make vulndb", with the fetch's first error line kept beside the stamp.
  - `test_generate.py` pins this. Counter-example: fetch exit 1 → the other lint rows still run.

**QA-S9 — The canary can file a real break under "pins to update", and the hash clause can never fire for the pin test it targets.**
- **Evidence.** `test_the_committed_tools_json_loads_in_file_order_for_both_platforms` (`gate/toolchain/tests/test_manifest.py:121-160`) asserts (name, version, command, path_dir) first, and the archive-prefix homes second.
  - Under any upgrade the first assertion fails, so the hash assertion is never reached.
  - Every upgrade also moves the archive prefixes, so "the same test failing on a hash" is not a usable signal.
  - A candidate-builder bug (order, `path_dir`, command) fails that same first assertion. If the sort keys on the test id it files the bug as a pin update; parsing unittest's diff text instead is brittle.
  - Some expected reds are not tests at all (the go.mod toolchain lines, `census.md`, possibly `lawdelta` on a changed `tools.json`).
- **Fix.**
  - P4's table is **measured**: one local T4 run under a ruff 0.17.5 candidate, with every red row and test listed with its text.
  - The sort accepts a pin red only after a structural diff of the candidate `tools.json` against the pinned one shows only version, url, digests and version_line moved, for tools in the upgrade list.
  - Counter-example: a candidate with one tool's `path_dir` changed → a break, named.

**QA-S10 — The canary says "gate green" for tables it never re-measures.**
- **Evidence.** `test_suppressions.py:3-4`: "§0.3's golangci-lint table and §0.2's ruff table are held here as data". Nothing runs the candidate tool against them. A ruff or golangci-lint release that honours a new waiver form stays green under the candidate, and the upgrade would then let unseen waivers through the `suppressions` row.
- **Fix.**
  - A canary test feeds each table form to the candidate binary and compares the result with the table.
  - Failing that, the summary says "suppression tables not re-measured" for each ruff or golangci-lint upgrade.

**QA-S11 — Q1 misstates what Google's scanner reports, and the card's acceptance lines need restating under Q1 and Q2.**
- **Evidence.** Q1 says the scan "is exactly what Google's scanner reports". Source mode never reports `toolchain` (QA-B2), and its default level is symbol, not module. Q1 also narrows the card's line 1 ("any selected module") and its goal ("every module the repository uses"). Q2 changes "week-old database" (`sprint-plan.md:112`, `:119`). The plan lists no card edit.
- **Fix.**
  - Correct Q1's sentence.
  - Under Q1 and Q2 as recommended, reword line 1: "any compiled module, the standard library or the toolchain …; a graph-only module is counted, not red; a copy fetched over 7 days ago, or a database unchanged for 30 days, is red".
  - Make the card edit a G0 item.

**QA-S12 — Q3's figures are not the worst case the cap meets.**
- **Evidence.** §0.5 reads each job's top-level parity right after a tool-cache restore, when the files were just written and sit in the page cache. That is warm, not cold. It also leaves out the `proofs` job, where each driver's nested gate re-hashes go's 15,660-file tree while other drivers do the same (inferred), and the stress harness under CPU hogs. A 60 s cap that flakes there stops the line.
- **Fix.**
  - Add to Q3's table: the nested parity durations of one `proofs` run, and one run under `gate/runner/stress.py`.
  - The pin test is a ratio guard over the cited data (for example cap ≥ 5 × the slowest cited run), red-proven by lowering the cap.

## Later

- **QA-L1 — The compiled set's boundaries.** The compiled set is host GOOS/GOARCH, default tags, `CGO_ENABLED=0`. Measured today, it is identical for darwin/linux/windows × cgo 0/1. `-e` drops a broken package's imports (`rows/deps.py:8-9`). Name both as the scan's residue in P1.
- **QA-L2 — Which index the matcher reads.** Today `index/modules.json` and `ID/` agree both ways (0/0 measured). govulncheck looks up through `modules.json`; the plan's `check_db` only checks that it is non-empty. State which one the matcher reads. Counter-example: an `ID/` entry naming a module that `modules.json` omits.
- **QA-L3 — The tidy check in CI's state.** P3 should measure it in a scratch clone with an empty `GOMODCACHE`: the CI download form, then `GOPROXY=off go mod tidy -diff` in both modules. Proofs' tidy wants go.sum lines for go-cmp v0.6.0 and `x/tools/go/expect` and `packagestest` (§0.6), which a cold cache may lack. That would make the deps row red in CI and green locally.
- **QA-L4 — The download-step check is never shown red.** The `test_generate.py` pin and the `git diff --exit-code` step each need to be shown red once: the old form in that scratch clone (exit 1 quoted) before the draft PR.
- **QA-L5 — The graph-only count.** Add a unit test: a module graph-only in one list and compiled in the other is counted once, as built. (1b)'s count is asserted in the reason.

## Pre-mortem (the-fool's question, run by hand)

**Green on a vulnerable build:**
- the `"0"` edge on x/exp's pseudo-version (B1);
- a `replace` matched at the require version (S4);
- an id-only waiver covering a second module (S5);
- an old zip with a fresh stamp (S6);
- an empty compiled set still covered (S1);
- an invalid version comparing lowest (S4);
- a module compiled only under a tag, OS or cgo (L1).

**Red for a week on a clean build:**
- the naive `"0"` reading, giving 11 extra x/net and x/sys entries under Q1's other options (B1);
- a vuln.go.dev outage aborting the lint job (S8);
- a driver reading a copy that CI's proofs job never fetched (S2);
- a 60 s cap under the proofs job's contention (S12);
- query-mode verdicts that drag the matcher into govulncheck's own bugs (B2).

## Verdict

The plan's direction holds, but acceptance line 1's two proofs are not yet able to fail for the right reason: the `"0"` and pseudo-version edge is missing (B1) and the agreement table has no sound way to measure govulncheck (B2); line 2's proof cannot see the wiring (S3).
Fix B1, B2 and the twelve should-fix items in v2 (mostly named counter-examples, two seams and one acceptance-line rewrite) and the plan is testable as written.
