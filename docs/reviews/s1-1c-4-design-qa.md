> **S1.1c-4 design note, QA review for testability with the-fool (blind), 2026-11-13** — the QA reviewer on [the design note](../sprints/s1-1c-4-design.md) v1.1 @ 5746220. Every finding is placed in the note's §8.

# S1.1c-4 design note v1.1 (5746220): QA design review for testability, with the-fool

Blind and read-only. Read: the note, the five stubs, the plan check, decisions 2026-11-13, the card, testing.md, sprint model §8 and §10, and rows/deps.py, deps.py, allowlists.py, env.py, generate.py, support.py, test_deps_planted.py, test_breakit_s11c1.py. QA's W0a test_gomods.py was read through `git show s1-1c-4/qa:` and not through the worktree. GitNexus: `context gather` shows its only test caller is DepsGather. Hindsight: not available in this session, so no memory was used. Measurements are in the scratch folder s1-1c-4-design-qa/: a clone at 5746220, the pinned go1.27.4, offline.

## Blockers

**DQ-B1: when the tidy line cannot run offline, it passes `gate/proofs` as clean (fail-open), and the seam drops what it needs to tell the two cases apart.**
Evidence: in a clone with an empty `GOMODCACHE` and `GOPROXY=off`, `go mod tidy -diff` exits **1** in both modules. stdout is 0 bytes. stderr reads `go: downloading golang.org/x/exp …` and then `go: golang.org/x/exp/cmd/apidiff: module lookup disabled by GOPROXY=off`. A real difference also exits 1 (§0.3). §3.1 calls `gomods.run(..., ok=(0, 1))`, and `run` returns stdout only (`gomods.py:39-45`), so the exit code and stderr never reach the row. An empty stdout parses to two empty sets. In `gate/proofs` both sets must be empty, so the result is **green**. In the root the removed set ≠ allowance, so it reds with the wrong text ("would change", not "could not run offline"). CI restores its module cache by key prefix, so a cache that lacks a graph-only module (go-cmp, expect, packagestest) reproduces this. The fault text `go mod tidy could not run offline` is unreachable as designed. Also, `<first stderr line>` would be the misleading `go: downloading …` line.
Fix: let `run` return `(code, stdout)`, or state the rule: "an exit of 1 whose stdout has no `diff current/` header is could-not-run". Quote the last stderr line that does not start with `go: downloading`. Named test: `test_tidy_that_cannot_run_offline_is_red_not_clean`. Its fake returns exit 1, empty stdout and the measured stderr, and the test asserts `deps: go mod tidy could not run offline in gate/proofs: go: golang.org/x/exp/cmd/apidiff: module lookup disabled by GOPROXY=off`.

## Should-fix

**DQ-S1: as written, the tidy-as-sets parse misclassifies go.sum lines and ignores single-line `require` lines and other directives.**
Evidence: real output, saved as `tidy-root.txt` and `tidy-proofs.txt`. Each file's hunk opens with `diff current/go.mod tidy/go.mod`, `--- current/go.mod` and `+++ tidy/go.mod`. (1) go.sum lines such as `-golang.org/x/sync v0.24.0 h1:…` have a module path as their first field and a version as their second, so they match §3.1's go.mod pair rule. Unless lines are classified by the header, the root's removed go.mod set gains `(x/sync, v0.24.0)` ≠ allowance, and the result is red on the real tree. That pushes the builder to "fix" the allowance. (2) `gate/proofs`' diff holds `-require go.uber.org/goleak v1.3.0` / `+require go.uber.org/goleak v1.3.0`. Its first field is `require`, so the rule as written skips it, and a planted unused single-line `require x v1` (counter-example "module files" (1)) would pass. The same goes for `go`, `toolchain`, `tool`, `replace` and `exclude` lines.
Fix: (a) classify each line by the `--- current/<file>` header; (b) strip a leading `require ` before taking the pair; (c) make any other `-`/`+` line that is not blank, a comment, `require (` or `)` a fault naming the line; (d) in `gate/proofs`, where the allowance is empty, make any `-`/`+` line red. Named tests: `test_tidy_go_sum_lines_never_count_as_go_mod_pairs` (fed the measured root diff, it must be green against `TIDY_ALLOWED`), `test_tidy_single_line_require_is_read`, `test_tidy_changed_go_or_toolchain_line_is_red`.

**DQ-S2: the tidy line breaks `test_deps_planted.py`'s exact-line assertion, and no lane is told.**
Evidence: `test_deps_planted.py:75-99`. Every world deletes the `tool` directive and appends `require example.com/planted v1.0.0` plus a directory `replace`, so the world is untidy by construction. `:111-116` asserts that the `failed:` lines are **exactly** the one planted line. `deps` runs at T2 (registry), so the tidy line adds faults to all six cases. The proofs row is T4-only, so this first shows at "proofs alone".
Fix: in §3.1, state how the driver's worlds handle tidy. Either the world carries a `TIDY_ALLOWED` that matches its planted lines, or the assertion takes the tidy line out by its prefix, with a case of its own that is red on an untidy planted `require`. E2 owns the update, and it runs the `proofs` row alone in E2 (§10, the world copies `rows/deps.py`).

**DQ-S3: the T4 `go.sum` line has no committed test; it now makes local T4 red on a dirty tree, which the gate has not done before.**
Evidence: §3.2 shows it red "once … in the scratch clone", a one-off. The precedent goes the other way: `lawdelta.py:250-252` *notes* uncommitted law edits and stays green. `git.dirty` (`git.py:99`) exists but is not reused. Under the new line, a developer who runs `make strict-gate` with an uncommitted `go get` is red. That may be intended (DA-S5), but the note does not say so.
Fix: state "T4 requires go.sum committed" as intended and add it to DEVELOPMENT.md's T4 line. Add committed tests: `test_go_sum_line_only_at_T4` (T3 dirty → no line; T4 dirty → `deps: go.sum differs from HEAD at T4…`; T4 clean → none) and a `test_deps_planted.py` case at `tier="T4"` that edits go.sum after `commit()`. If `git` fails (no HEAD), name it as a fault line; it must not abort the row's other lines (§3.1's "last line" promise).

**DQ-S4: the waiver semantics have three holes a test cannot pin today.**
(a) Stale-ness when the scan was partial. §2.5 exempts only "not scanned" (no zip). Three other cases leave the `<id> <module>` entries matching nothing, so all of them read "matches nothing: remove it": `index/vulns.json` malformed (no entries read), `gomods.Failed` (no modules), and "no module read". The result is a pile of false stale lines on top of the real fault.
(b) A waiver on a never-built finding. That finding is green anyway; does matching it count as "silenced"? The note leaves it undefined.
(c) An expired waiver. §2.5 relies on the *suppressions* row's `RED_EXPIRED` (`allowlists.py:28`). It never says the *vulns* row stops silencing with it, so the vulns row could read green with `silenced: … until <past date>`.
Fix: define "judged" = loader without `Failed`, index parsed, every listed entry read; otherwise no staleness lines. State (b) one way (recommend: stale, since the mark already covers it) and say the vulns row ignores expired entries. Named tests: `test_waivers_not_judged_stale_when_the_index_is_malformed`, `test_waiver_on_never_built_finding_is_stale`, `test_expired_waiver_leaves_the_finding_red_in_vulns`.

**DQ-S5: several fault texts and rules have no named test, and some have no text at all.**
- Not in W0b's driver list or any named test: an `<id> <module>` waiver silencing a `findings` fault (DA-S1's point); `vulns: no module read` (§6 (22)); `replaced by a directory: …` (§6 (15)); `vulns: <target>: not "<id> <module>" or "vulndb"` (listed as "a malformed target" in the plan check only).
- No fault text at all: a `vulndb.zip` larger than `FETCH_CAP` at the row (§2.5 says "bounded" only); a malformed `fetch-error.txt` (unparsable `at`); a `DepsErrors` line (which `ImportStack` element is "the import path"?). Measured: the stack's last element is the *importer* (e.g. `github.com/vorsar/cep/internal/parser`), and `github.com/vorsar/cep_test` appears in no package object (the stream has `… [github.com/vorsar/cep.test]`). Read naively, a cold cache gains an extra line.
- `gate/toolchain/vulndb_fetch.py` (§2.3: validated before replacing, old copy kept, cap, fetch-error.txt, exit 2) has **no named test and no seam**: `main(argv, environ)` gives no `get` to inject, unlike `feeds.fetch(…, get)`.
Fix: give `vulndb_fetch` a `fetch(root, *, get, now)` seam and named tests: not-a-zip body → old zip and stamp unchanged, `fetch-error.txt` written, exit 2; body past `FETCH_CAP` → same; no `index/db.json` → same; success → stamp sha256 = zip's, `fetch-error.txt` removed. Name the three missing texts. Add the four row cases to W0b.

**DQ-S6: card acceptance lines 1 and 2 name proofs the plan does not supply.**
Line 1 lists "a missing, **empty, index-less** or mismatched copy". W0b's driver list has no zip, not a zip, no stamp, a sha256 mismatch and a member outside the layout, but no empty and no index-less case. Line 2's named proof is `test_deps.py`, which imports only `allowlists, deps, sprintid` (plan check §0.9) and cannot see the tool-line wiring. The note (§5, §7) discharges line 2 with `test_gomods.py` plus a `test_deps_planted.py` case.
Fix: add driver cases `empty_zip` (zero members → `index/db.json is missing or malformed`), `zero_byte_file` (→ `is not a zip`) and `no_index` (an `ID/` member only). Ask the owner at G1 to annotate line 2's proof, or state in §7 which test discharges it.

**DQ-S7: the canary's URL and `strip` substitution is wrong as written for golangci-lint, python-build-standalone and lark.**
Evidence (`tools.json`): golangci-lint's `version` is `v2.15.0`, but its `strip` is `golangci-lint-2.15.0-linux-amd64/` (no `v`). "The old version replaced by the new where it holds it" (§4.3) therefore changes nothing, and §4.4 flags the correct new `strip` as a structural break on every golangci-lint upgrade. pbs's url holds the build tag twice (`download/20261026/cpython-3.15.2%2B20261026-…`), but the pin's `version` is `3.15.2`, so a build-tag-only upgrade (today's 20261110) cannot come from "the version substituted". lark's url is a hash path (`packages/da/34/28fff3…/lark-1.3.1.tar.gz`); no substitution can produce it, and it must come from the PyPI feed's `urls[]`. lark also has no `version_argv`, so §4.7's "running `version_argv`" is undefined for it. `Candidate` (`candidate.py:23-26`) lacks §4.3's `newer_lines`, and `Upgrade(name, pinned, latest)` cannot carry `(build 20261026 → 20261110)`.
Fix: give the rule per tool: substitute `version` with and without its leading `v`/`go`, plus pbs's build tag; take lark's url and digest from the feed by filename; set lark's `version_line` to the version. Add `newer_lines` and the build pair to the stubs. Named W0c tests: `test_golangci_upgrade_strip_has_no_structural_fault`, `test_pbs_build_tag_upgrade_url_and_name`, `test_lark_url_from_feed`, `test_pbs_cpython_rc_on_line_is_not_stable`.

**DQ-S8: the canary sort works per record, and `PIN_TESTS` cannot be measured for go as planned.**
(a) §4.5 sorts "a red record whose row and failing test id …", one id. A row with a pin red *and* a real break (e.g. a unit-py row where ruff 0.17.5 also breaks another test) could be filed as a pin, hiding the break. (b) `PIN_TESTS` is "measured … under a ruff 0.17.5 candidate", yet it lists "the `go.mod` toolchain lines", which only a go upgrade produces. Today's first run carries pbs upgrades too, whose reds (hash-only) may differ in text.
Fix: send a record to `pins` only when **every** failure in it matches a pin pattern; otherwise send it to `breaks` naming the rest. Measure `PIN_TESTS` under today's full candidate (ruff + both pythons), and name the go, golangci-lint and lark pin reds as unmeasured. Named test: `test_record_with_pin_and_other_failure_is_a_break`.

**DQ-S9: on a failed candidate job, the canary loses its reason, though acceptance line 3 says "a rate limit named".**
§4.6 prints `Upgrades: unknown — the candidate job failure`. The `CanaryError` text (rate limit, digest) stays in the step log only.
Fix: the feeds and build steps write `candidate/error.txt`; the candidate's upload uses `if: always()`; the summary quotes it. Named test: `test_summary_quotes_the_candidate_error` over a recorded `NEEDS` (`candidate: failure`) plus `error.txt`.

**DQ-S10: the shape check for `index/vulns.json` is unspecified, and the real copy would fail a literal reading.**
Evidence: today's copy has 4,801 items; **19 lack `aliases`** (keys `{id, modified}`). §0.1 states items as `{id, modified, aliases}`, and §2.4 says only "likewise". Also, an `ID/` member whose id is absent from `vulns.json` is never read and never flagged, because the check runs one way only.
Fix: "a non-empty list of objects with string `id` and `modified`; `aliases` optional". Add a fault for an `ID/<id>.json` not listed in `vulns.json`, and pin it with a test over the real 19-item shape.

**DQ-S11: a failed `make vulndb` makes a usable copy red, which is stricter than decision 2.**
§2.5 reds `the last make vulndb failed` whenever `fetch-error.txt` is newer than the stamp, even with a 2-day-old copy that passes `check_db`. A developer offline is blocked until a fetch succeeds, or until they write a TEMP `vulndb` waiver with a known issue. Decision 2's red is the copy's age (> 7 days).
Fix: when the copy is otherwise usable, name the failed fetch in the reason and keep it green; keep it red when there is no zip. Or ask the owner. Named test: `test_failed_refetch_with_usable_copy_is_named_not_red`.

**DQ-S12: the CI-only proofs do not say what must be quoted to count.**
Fix, in §6 or §7. The **nightly on the branch** quotes the run id, the `vulns` record's PASS reason with `fetched <the run's own UTC day>`, and the `make vulndb` step outcome `success`. The **canary's first run** quotes the run id and the summary's first line (`Upgrades: ruff 0.17.3 → 0.17.5, python 3.15.2 (build 20261026 → 20261110), python-floor …`); `Pins to update` must be non-empty and equal the measured `PIN_TESTS`, and each listed break is explained. **Parity's CI times** quote the run id, each job's `parity.json` duration, and the max against `SLOWEST_PARITY_S`.

**DQ-S13: driver case (1c) would mutate the shared module cache.**
Worlds inherit `HOME` and so `GOMODCACHE` (`support.py:184`); "a built module's zip removed from the world's module cache" would delete from, or chmod, the developer's and CI's real cache (files are 0444).
Fix: (1c) sets a private `GOMODCACHE` (a copy of the needed modules, or an empty cache plus `go mod download` from a local `GOPROXY=file://`), and the note names that mechanism.

## Later

- **DQ-L1**: the ratio guard compares two constants in one file, so lowering both passes. Keep the measured table as data (`PARITY_MEASURED_S = {"<run>/<job>": s}`) and guard `SLOWEST_PARITY_S == max(...)`.
- **DQ-L2**: stale `govulncheck` naming. The card's in-scope line reads `tools/allowlists/govulncheck.json` and `allowlists.py:19`'s comment reads "S1.1c-4 adds govulncheck"; the rule and file are `vulns`.
- **DQ-L3**: §2.7's `deps.Module(…, replace=…)` leaves the string form unstated. Today it is `"<path> <version>"` or `"<path>"`, built at `rows/deps.py:71`. `gomods.Failed`'s text drops today's `deps: … failed:` prefix, so a vulns fault would carry no `vulns:` prefix. Pin both.
- **DQ-L4**: the download-step measurement should include `gomods.load` over **both** platforms in the empty cache on Linux (darwin-only deps).
- **DQ-L5**: `archive_pins`: add a test that a `_Rejected` archive leaves no temp dir (`finally` after `_seal`).

## The-fool pre-mortem: every planned test is green, and the gate still lies or blocks

1. CI restores a prefix-matched module cache missing go-cmp. Tidy cannot run, and `gate/proofs` is reported tidy (DQ-B1). S1.1c-3-1 closes on a check that never ran.
2. ruff 0.17.5 breaks a real test in the same row as the manifest pin. The canary files the row under "Pins to update", and the break ships at the next upgrade (DQ-S8).
3. Every golangci-lint patch shows a structural break, and every pbs build tag gives "Upgrades: unknown". The canary cries wolf and is ignored (DQ-S7).
4. The day `index/vulns.json` comes back malformed, the row prints the real fault plus one "remove it" per live waiver. Someone removes them, and the next good run is red (DQ-S4a).
5. A developer offline with a fresh copy, or with a dirty go.sum, is red at local T4 for no vulnerability (DQ-S11, DQ-S3).

## Verdict

v1.1 is close to build-ready for the matcher, `check_db`, the loader and `archive_pins`. Before G1 it needs one blocker fixed (the tidy line's fail-open seam) and the should-fix items placed. The heaviest of those are the tidy parse, the waiver states, the canary's substitution and sort, and what the CI-only runs must quote.
