# S1.1c-5 design note v1 — blind QA review (testability)

> **Historical — S1.1c-5, the strict lint set: the instrument, the whole-tree rules, the first packages; merged in PR #47.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-5-plan-check.md) · [design](../sprints/s1-1c-5-design.md) · [record](../sprints/s1-1c-5-record.md) · reviews: [research lint](s1-1c-5-research-lint.md), [plan check architecture](s1-1c-5-plan-check-architecture.md), [plan check qa](s1-1c-5-plan-check-qa.md), [design architecture](s1-1c-5-design-architecture.md), [design qa](s1-1c-5-design-qa.md), [combined architecture](s1-1c-5-combined-architecture.md), [breakit qa](s1-1c-5-breakit-qa.md). **← [S1.1c-4](../sprints/s1-1c-4-record.md) · [S1.1c-6 (next)](../sprints/sprint-plan.md#status) →**

Reviewed: `docs/sprints/s1-1c-5-design.md` on `s1-1c-5/integration` @ 215bb3b (cited as design:line). Experiments: `s1-1c-5-scratch/qa-design-exp/` (m1: config types, `//line`, stale file; m2: `//nolint` spellings), pinned golangci-lint v2.15.0, go1.27.4, python 3.15.2. GitNexus not used for these points (the seams are config and gate-runner text; I read the cited lines directly). Hindsight not consulted (time box). Time box cut short by the coordinator; anything not run is marked (not verified).

## 1. The finished §8 test table

Module paths are relative to the repo root. Red-first: "stub" = the W0c `NotImplementedError` stubs, "today" = the current code. A test that is green from its first run is called out as such and gets a mutation proof instead.

### Card line 1 — the row's rules bite (lane A turns green unless noted)

| # | Test (module :: name) | Red before code because | Green by |
|---|---|---|---|
| 1.1 | `gate/runner/tests/test_golangci.py` :: `PinnedDocument.test_the_committed_config_equals_pinned` | stub | A |
| 1.2 | `test_golangci.py` :: `PinnedDocument.test_each_hiding_edit_is_named` (subTests, one per edit: `issues.new-from-rev`, `issues.new-from-patch`, `issues.whole-files`, `run.tests=false`, `max-issues-per-linter` removed, `max-same-issues` removed, `uniq-by-line=true`, `generated="lax"`, `funlen.statements=-1`, `exclude-godoc-examples` removed, `exclusions.paths=["x"]`, a fifth `exclusions.rules` entry, `run.build-tags=["x"]`, `output.formats.json.path`, `severity.default="warning"`, `errcheck.disable-default-exclusions` removed, one `exclude-functions` entry removed, `enable` minus `revive`) | stub | A |
| 1.3 | `test_golangci.py` :: `PinnedDocument.test_a_type_change_is_a_difference` — `arguments=[5.0]`, `max=500.0`, `run.tests=1`, `funlen.lines=60.0` (DQ-S1) | stub; **and Python `==` says equal (measured)** | A |
| 1.4 | `test_golangci.py` :: `PinnedDocument.test_a_duplicate_key_is_a_config_error`, `test_an_upper_case_twin_key_is_not_pinned` (`[LINTERS]`) | stub | A |
| 1.5 | `test_golangci.py` :: `ReadReport.test_issues_error_and_warnings_are_read` (recorded JSON from qa-design-exp/m1: `Warnings=[{"Tag","Text"}]`, `Error="setup revive…"`), `test_null_issues_is_empty`, `test_a_truncated_document_is_a_value_error` | stub | A |
| 1.6 | `test_golangci.py` :: `RuleOf.test_reviews_two_size_rules_by_prefix`, `test_another_revive_prefix_is_revive` | stub | A |
| 1.7 | `gate/runner/tests/test_rows_smells.py` :: `Exit.test_exit_0_with_an_issue_is_red`, `test_exit_1_with_nothing_is_red`, `test_exit_3_is_red_with_stderr`, `test_no_report_file_is_red`, `test_a_report_left_by_an_earlier_run_is_never_read` (DQ-S2), `test_a_truncated_report_is_a_named_fault` | stub | A |
| 1.8 | `test_rows_smells.py` :: `Health.test_report_error_is_red`, `test_a_warning_is_red`, `test_enabled_short_by_one_is_red`, `test_enabled_without_typecheck_is_red`, `test_a_typecheck_issue_in_an_excluded_package_is_red` | stub | A |
| 1.9 | `test_rows_smells.py` :: `Merge.test_a_finding_on_both_goos_counts_once`, `test_a_linux_only_finding_is_kept` | stub | A |
| 1.10 | `test_rows_smells.py` :: `Drops.test_fmtprint_in_a_main_tool_is_dropped`, `test_fmtprint_in_a_non_main_tools_package_is_red`, `test_fmtprint_outside_tools_in_package_main_is_red` (cmd/), `test_a_generated_json_target_is_dropped`, `test_an_issue_at_a_file_no_package_claims_is_red` (DQ-B2) | stub | A |
| 1.11 | `test_rows_smells.py` :: `Env.test_go_list_and_golangci_share_one_env_per_platform` (captured env: same GOOS/GOARCH/CGO_ENABLED=0/GOFLAGS=""/GOWORK=off), `test_the_lint_cache_is_named` (DQ-S4) | stub | A |
| 1.12 | `test_rows_smells.py` :: `Unsafe.test_imports_test_imports_and_xtest_imports_are_each_red` | stub | A |
| 1.13 | `gate/proofs/drivers/test_smells_planted.py` (real binary, real row; 30 cases, list in §1.8) | the row does not exist (no `smells` row id: the gate refuses the run) | A |

### Card line 2 — only `//nolint` waives (lane B)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 2.1 | `gate/runner/tests/test_suppressions.py` :: `Foreign.test_each_honoured_census_form_is_refused` (data-driven over `gate/runner/testdata/directives.json`, one subTest per `honoured: true` row) | `go_foreign` stub; today nothing reads `revive:`/`#nosec`/`gocyclo:` | B |
| 2.2 | `test_suppressions.py` :: `Foreign.test_each_unhonoured_form_is_not_a_fault` (the `honoured: false` rows: `/* revive:… */`, `// gocyclo:ignore`, `//NOLINT:lll`, `/*nolint:lll*/`, `//lint:ignore`) | stub | B |
| 2.3 | `test_suppressions.py` :: `Foreign.test_the_prefilter_finds_every_honoured_form` | PREFILTER lacks them today | B |
| 2.4 | `test_suppressions.py` :: `Nolint.test_nolint_all_is_refused`, `test_nolint_typecheck_is_refused`, `test_a_nolint_variant_golangci_honours_is_refused` (`// nolint:lll`, `//nolint: lll`, `//nolint:LLL`, `//  nolint:lll`, `//nolint:lll,`, bare `//nolint`, `//nolint:lll // x` — measured honoured, DQ-S6) | today `ENABLED` is empty so every one is "not enabled" — the test must assert the **specific** text, not just a fault | B |
| 2.5 | `test_suppressions.py` :: `Nolint.test_an_accepted_lll_waiver_passes` (the §4.2 example, verbatim) | **red today**: `ENABLED` empty makes `lll` "not enabled" | B |
| 2.6 | `test_suppressions.py` :: `Enabled.test_enabled_equals_the_config_both_ways`, `test_a_linter_not_in_the_config_is_not_enabled_text` | stub (`golangci.enabled`) | B (needs A's `golangci.py`; B codes against the stub) |
| 2.7 | `test_suppressions.py` :: `LineDirective.test_a_line_directive_is_refused` (`//line x.go:1`, `/*line x.go:1*/`) (DQ-B2) | today no such check | B |
| 2.8 | `gate/runner/tests/test_breakit_s11c2.py` :: `NosecIsAWaiver.test_a_nosec_is_read`, `NosecIsAWaiver.test_a_nosec_is_found_by_lawdeltas_prefilter` — restored from **`git show fc944ef:`**, not `fc944ef^:` (DQ-S7) | `go_directives`/PREFILTER do not read `#nosec` (the BK-S4 red) | B |
| 2.9 | `test_smells_planted.py` :: `test_a_waiver_on_the_wrong_line_is_red_and_the_finding_stays` (nolintlint unused + the real lll) | row absent | A |
| 2.10 | `gate/proofs/drivers/test_suppressions_planted.py` += `test_a_revive_directive_is_red`, `test_a_nosec_is_red`, `test_a_line_directive_is_red`, `test_an_accepted_nolint_with_metadata_passes` (9 → 13) | today all four are green/unread | B |

### Card line 3 — exclusions are bounded (lane B for parts, lane A for the row's use)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 3.1 | `gate/runner/tests/test_sprintid_parts.py` :: `ReachedPart.test_the_design_table` (subTests: §3.2's six), `test_a_part_less_marker_at_the_parts_row_is_false`, `test_an_unprinted_part_raises`, `test_a_row_id_as_part_raises`, `test_parts_compare_as_numbers` (S1.1c-6 vs S1.1c-10 printed) | stub | B |
| 3.2 | `gate/runner/tests/test_parts.py` :: `test_no_issue_is_red`, `test_an_empty_forward_is_red`, `test_an_unprinted_forward_is_red`, `test_a_forward_past_s1_1c_8_is_red`, `test_reached_at_t4_is_red`, `test_reached_at_nightly_is_red`, `test_reached_at_t2_is_a_line_not_red`, `test_not_reached_is_silent`, `test_unlinted_entries_are_skipped` | stub | B |
| 3.3 | `test_parts.py` :: `Moves.test_a_forward_moved_without_a_decision_is_red`, `test_moved_with_a_naming_decision_is_green`, `test_moved_with_the_base_decision_is_red`, `test_moved_earlier_is_green`, **`test_an_entry_repointed_to_a_later_issue_is_red`**, **`test_a_size_entry_absent_at_the_base_is_red_after_s1_1c_5`**, `test_a_removed_entry_is_green` (DQ-B1) | stub | B |
| 3.4 | `test_parts.py` :: `Base.test_no_merge_base_at_t4_is_red`, `test_no_merge_base_below_t4_is_a_line`, `test_the_base_file_absent_is_treated_as_no_base` (DQ-B3) | stub | B |
| 3.5 | `test_rows_smells.py` :: `Exclusions.test_an_entry_drops_only_its_package_and_rule`, `test_a_subpackage_is_not_dropped`, **`test_a_root_package_entry_drops_a_root_finding`** (DQ-S3), `test_an_expired_entry_drops_nothing_at_t4`, `test_an_entry_that_drops_nothing_is_red`, `test_waives_nothing_is_not_judged_after_a_fault`, **`test_waives_nothing_is_not_judged_on_a_staged_run`** (DQ-S5), `test_a_bad_target_form_is_red`, `test_a_permanent_size_entry_is_red` | stub | A |
| 3.6 | `test_rows_smells.py` :: `Covered.test_an_unclaimed_file_is_red`, `test_ignored_under_both_goos_is_red`, `test_a_nested_go_mod_is_red`, `test_a_linux_only_file_counts_once`, `test_linted_and_unlinted_is_red`, `test_a_stale_unlinted_entry_is_red`, `test_a_file_claimed_twice_is_red`, `test_a_package_error_is_red`, `test_covered_equals_tracked_when_all_accounted` | stub | A |
| 3.7 | `test_smells_planted.py` :: `test_a_new_package_not_clean_is_red`, `test_an_excluded_package_hides_only_its_rule`, `test_an_exclusion_that_waives_nothing_is_red` | row absent | A |
| 3.8 | `gate/runner/tests/test_sprints_pin.py` :: `test_the_four_parts_are_printed_after_s1_1c` (and `known-issues` S1.1c-5-1…-4 present with forwards S1.1c-6/-7/-8/"") | W0a not yet committed | coordinator (W0a) |
| 3.9 | `gate/runner/tests/test_allowlists.py` :: `test_lane_d_packages_have_no_size_entry` (`internal/table`, `internal/clock`, `io/inmemory`, `tools/shape`) — the in-scope floor, PQ-S8 | green on an empty file — **mutation proof**: plant one entry, red | QA writes; holds through G1→T3 |

### Card line 4 — the ruff pin is closed (lane B)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 4.1 | `gate/runner/tests/test_rows_ruff.py` :: `RuffPin.test_the_committed_ruff_toml_equals_pinned` | stub `RUFF_PINNED` | B |
| 4.2 | `test_rows_ruff.py` :: `RuffPin.test_each_hiding_key_is_named` (subTests: `builtins`, `lint.dummy-variable-rgx`, `lint.pycodestyle.max-line-length`, `line-length`, `lint.select` narrowed, `lint.extend-select` removed, `per-file-target-version` entry added, `lint.flake8-bugbear.extend-immutable-calls`, `force-exclude`+`extend-exclude`, `format.exclude`) | **red today** for every key but the deny-list's (measured M-5, PQ-S7) | B |
| 4.3 | `test_rows_ruff.py` :: `RuffPin.test_per_file_ignores_still_checked_both_ways`, `test_a_type_change_is_named` (`line-length = 100.0` vs pinned int) | stub | B |
| 4.4 | `gate/runner/tests/test_pin.py` :: `test_added_removed_changed_paths`, `test_type_strict`, `test_list_order_counts`, `test_dotted_key_quoted` (the shared walker, §5) | new module, stub | B |

### Card line 5 — declared tools are scanned (lane C)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 5.1 | `gate/toolchain/tests/test_manifest.py` :: `GoBuilt.test_true_is_accepted`, `test_false_is_refused`, `test_a_non_bool_is_refused`, `test_on_a_python_package_is_refused`, **`test_golangci_lint_is_declared_go_built`** (DQ-S8) | today the field is an unknown key (refused) | C |
| 5.2 | `gate/canary/tests/test_candidate.py` :: `test_the_candidate_carries_go_built` | stub/unknown key | C |
| 5.3 | `gate/runner/tests/test_vulndb.py` :: `Pseudo.test_a_binary_scan_never_judges_toolchain`, `test_the_engine_scan_still_judges_toolchain` (twin, PQ-S9), `test_a_binary_scan_judges_stdlib` | `pseudo=` absent (TypeError) | C |
| 5.4 | `gate/runner/tests/test_vulns_row.py` :: `BuildInfo.test_exit_nonzero_is_red`, `test_no_go_line_is_red`, `test_devel_go_is_red`, `test_an_experiment_suffix_is_parsed_or_red` (`go1.27.0 X:boringcrypto`), `test_no_mod_line_is_red`, `test_mod_version_not_the_pin_is_red`, `test_zero_deps_is_red`, `test_a_replacement_is_applied`, `test_the_pair_names_the_tool`, `test_covered_counts_each_declared_binary` (recorded `go version -m` texts: darwin from the cache, linux from W0's fetched archive) | stub | C |
| 5.5 | `test_vulns_row.py` :: `Exempt.test_an_exemption_matching_no_finding_is_red` (DQ-S9; not verified whether today's code has it) | (not verified) | C |
| 5.6 | `gate/proofs/drivers/test_vulns_planted.py` += `test_a_declared_binary_with_a_stdlib_advisory_is_red`, `test_an_exemption_scoped_to_the_binary_is_green`, `test_a_toolchain_advisory_never_applies_to_a_binary`, `test_a_declared_binary_without_build_info_is_red` (31 → 35) | the scan does not exist | C |

### Card lines 6–7 — real tree, no behaviour change (lanes C, D; coordinator)

| # | Proof | Fails when | Owner |
|---|---|---|---|
| 6.1 | T4 run: `smells` green, `covered == expected` (260 today), summary line quotes exclusions | any finding outside an entry | coordinator, after W2 |
| 6.2 | Record: T0 (one file; ten packages) and T1 times, darwin and CI | not measured | W0 then lane A |
| 6.3 | `T-A12`: errcheck defaults vs written-out list on the real tree, both counts recorded, equal | a default the plant misses | QA (W0) |
| 7.1 | T4: conformance, determinism, replay, coverage, apidiff green | any behaviour change | coordinator |
| 7.2 | `tools/astequal/astequal_test.go` :: `TestReflowEqual`, `TestMovedFunctionAcrossFilesEqual`, `TestCommentEditEqual`, `TestRenamedVariableUnequal`, `TestChangedLiteralUnequal`, **`TestOperatorChangedUnequal`** (`<`→`<=`, `+`→`-`), **`TestOperandsSwappedUnequal`**, **`TestDefineVsAssignUnequal`**, **`TestBreakVsContinueUnequal`**, **`TestIncVsDecUnequal`**, **`TestStatementOrderUnequal`**, **`TestSecondInitChangedUnequal`**, **`TestSameMethodNameOtherReceiverChangedUnequal`**, **`TestBlankVarChangedUnequal`**, **`TestGoDirectiveRemovedUnequal`** (`//go:noinline`, `//go:build`), `TestImportChangedUnequal`, `TestMovedToAnotherPackageUnequal` (DQ-S10) | stub | D |
| 7.3 | `tools/shape/shape_test.go` :: `TestShapeCorpusGolden`, `TestShapeEachConstructKind` (one row per case arm of `shape`'s and `shapes`' switches); completeness = 100 % statements of `shape` and `shapes` by `go tool cover -func`, quoted in the record, before the first refactor commit; one mutation per arm shown red in the record (DQ-S11) | green from birth — mutation proof | D |
| 7.4 | `BenchmarkTableCheckValue`, `BenchmarkRuleClockFireGroup` + `TestCheckValueAllocs`/`TestFireGroupAllocs` (`testing.AllocsPerRun` pinned at today's value) **if** W0 shows either on the per-event path (DQ-S12) | an extra allocation from the split | D |
| 7.5 | Per lane-D commit: `astequal <rev>` exit 0 for move/reflow commits, re-run by the coordinator at G5 (ground rule 17) | a non-pure "pure" commit | D, coordinator |

### 1.8 `test_smells_planted.py` — the 30 cases (each one planted world, one gate run, red names the past-bound file:line and does not name the at-bound one)

1 `test_function_60_lines_green_61_red_with_comments_counted` · 2 `test_cyclomatic_10_green_11_red` · 3 `test_file_500_green_501_red` · 4 `test_params_5_green_6_red` · 5 `test_line_120_green_121_red_a_tab_one_column` · 6 `test_a_test_file_is_exempt_from_size_and_lll_not_from_panic` (proves `run.tests = true`) · 7 `test_println_print_fmt_println_red_in_a_library` · 8 `test_fmtprint_green_in_a_main_tool_red_in_a_non_main_tool` · 9 `test_a_fourth_panic_beside_three_waived_is_red` · 10 `test_reflect_red_in_l3_green_in_l2_green_in_a_test` · 11 `test_getenv_lookupenv_environ_red_green_under_cmd_and_io` · 12 `test_blank_unsafe_with_linkname_is_red` · 13 `test_unsafe_in_an_xtest_is_red` · 14 `test_findings_in_an_example_function_are_red` · 15 `test_unchecked_close_red_written_out_exclusion_green` (`rt.Close()` red, `(*bytes.Buffer).WriteString` green, `fmt.Fprintln(os.Stdout, …)` red) · 16 `test_gocritic_append_assign_is_red` · 17 `test_a_waiver_on_the_wrong_line_is_red_and_the_finding_stays` · 18 `test_nolint_spellings_golangci_honours_are_red` · 19 `test_a_typecheck_error_in_an_excluded_package_is_red_beside_an_in_scope_finding` · 20 `test_a_generated_header_not_in_generated_json_is_linted` (+ listed one dropped) · 21 `test_two_findings_on_one_line_are_both_red` (uniq-by-line) · 22 `test_fifty_one_findings_of_one_linter_are_all_red` (max-issues-per-linter) · 23 `test_a_linux_only_and_a_darwin_only_finding_are_both_red` · 24 `test_unclaimed_files_are_red` (testdata/, `_dir/`, `//go:build ignore`, nested `go.mod`) · 25 `test_a_hiding_config_edit_is_refused_before_lint` (`issues.new-from-rev`) · 26 `test_a_float_argument_limit_is_red` (`arguments = [5.0]`) · 27 `test_an_excluded_package_hides_only_its_rule` · 28 `test_an_exclusion_that_waives_nothing_is_red` · 29 `test_a_new_package_not_clean_is_red` · 30 `test_a_line_directive_cannot_move_a_finding_into_a_main_tool`.

Cost: 30 worlds × 2 GOOS lints; W0 measures; if > 120 s, group worlds per class only where teeth still sees one verdict per case (not verified that teeth admits `setUpClass` sharing).

### Counts that move (exact)
- `proofs` row `min_wants` 38 → **39** (expected_scope `gate/proofs/drivers/test_*.py`: 38 files today, + `test_smells_planted.py`).
- `gate/law.json` instruments: `smells` new, `proofs: [{"path": "gate/proofs/drivers/test_smells_planted.py", "cases": 30}]`, `min_wants` 1; `suppressions` `test_suppressions_planted.py` cases 9 → **13**; `vulns` `test_vulns_planted.py` cases 31 → **35**, row and instrument `min_wants` 6 → **7** (6 + golangci-lint).
- `code` lists (later, DQ-L3): `smells` += `gate/runner/pin.py`, `gate/runner/env.py`; `suppressions` += `pin.py`, `parts.py`, `golangci.py`, `git.py`, `sprintid.py`, `aging.py`, `knownissues.py` as imported.

## 2. Findings

### Blockers

**DQ-B1 — A forward moves without the owner by re-pointing an entry's `issue`, or by adding an entry.**
- Claim (design:296): rule 5 compares, per *issue*, the base `forward` with the current one.
- Failure: base entry `internal/runtime gocyclo` issue `S1.1c-5-1` (→ S1.1c-6). A later branch edits the entry to `issue: S1.1c-5-3` (→ S1.1c-8). No issue's forward changed, so rule 5 is silent and the package's deadline moves two parts. The same applies to a new entry `internal/newpkg funlen` (issue S1.1c-5-3) added in S1.1c-6: card line 3's "a new package outside the exclusions that is not clean is red" passes, because the package is no longer outside. Decision 1 says "A forward moves only by an owner decision".
- Fix: compare **per target**: the effective forward (the entry's issue's forward) at the merge base against now. Later now → red unless decided. A target absent at the base → red unless the marker is S1.1c-5 (the landing sprint). Tests 3.3 (bold).

**DQ-B2 — `//line` moves a finding into a file the row drops.**
- Measured (qa-design-exp/m1): in the non-main package `gen/g.go`, `//line ../tools/t/main.go:3` before `fmt.Println` makes golangci report `forbidigo tools/t/main.go:3 [fmtprint]`, with exit 1, no `Report.Error` and no warning. Step 6(b) (design:188) drops it as a main-tool file, so the row is green. The same route moves a funlen/gocyclo finding into an excluded package's directory (6c) or onto a `generated.json` target (6a).
- A `//line` naming a missing file gives a `runner/source_code` warning, so that variant is red. An unindented `//line` also draws gocritic `commentFormatting` (measured), so the attack costs one `//nolint:gocritic` with metadata: a plausible waiver that launders a banned call.
- The tree has 0 `//line` directives today (`git grep`), so refusing them costs nothing.
- Fix:
  - `suppressions` refuses `//line` and `/*line` in tracked `.go` files (test 2.7, planted 2.10).
  - The row reds any issue whose file is not in the `go list` file map of the run's GOOS (test 1.10).
  - Planted case 30.

**DQ-B3 — With no merge base, the forward-move check is skipped at T4 too.**
- Claim (design:296): a `None` base "skips rule 5 and prints" a line, at every tier.
- `git.merge_base` returns "" when neither `origin/main` nor `main` resolves (gate/runner/git.py:3, :23). CI fetches full depth (gate.yml:22), but a local T4, a fork or a detached clone have no base: decision 1's guard would then pass silently at the tier that is the evidence of record.
- Fix: at tiers in `allowlists.RED_EXPIRED` (T4, nightly), no base → red. Below them → the line. Tests 3.4.

### Should-fix

**DQ-S1 — The whole-document pin is type-blind.**
- Measured: `tomllib` documents with `arguments = [5.0]` or `tests = 1` compare `==` to the pinned one (`True`).
- With `[5.0]`, golangci exits 1 with `Report.Error` "setup revive: cannot configure rule: argument-limit…", and with a fresh cache revive reports nothing. Only step 5 catches it, not the pin that claims "refuses any key not written above" (design:153).
- Fix: `pin.differences` is type-strict, recursively (`type(a) is type(b)`, so bool ≠ int ≠ float). Tests 1.3 and 4.4; planted case 26.

**DQ-S2 — A stale report file can be read.**
- Measured: a config error (exit 3) leaves a pre-existing `--output.json.path` file untouched.
- Step 4 handles exit 3, but nothing says the row deletes `<row_dir>/smells-<goos>.json` before each run. A crash or path mix-up then reads the last run's clean report.
- Fix: unlink both files before the run, and map a decode error to a named fault. Tests 1.7 (stale, truncated).

**DQ-S3 — The root package can never be excluded.**
- §3.1 names the root package `.` (design:254), but step 6(c) compares `posixpath.dirname(file)`, which is `""` for `cep.go`. The root holds 37 tracked `.go` files (`row.go`, `runtime.go`, …).
- An entry `. funlen` never drops anything: the finding stays red and the entry is "waives nothing". Loud, but it blocks the planned exclusions.
- Fix: normalise `""` → `.`. Test 3.5 (`test_a_root_package_entry_drops_a_root_finding`).

**DQ-S4 — The golangci cache is unnamed, and served stale issues in one run.**
- `env.clean_env` drops every `GO*` name not in `GO_ALLOWED` (gate/runner/env.py), so `GOLANGCI_LINT_CACHE` never reaches the row and golangci uses the user cache.
- Measured (one comparison, not repeated): with a shared cache, the `[5.0]` run still listed revive's `argument-limit` issue from the earlier run while `Report.Error` said revive could not be set up. With a fresh cache it listed none.
- The red holds through `Report.Error`, but a planted proof that asserts "finding present" can be satisfied by the cache.
- Fix: the row sets `GOLANGCI_LINT_CACHE` explicitly (a gate cache dir), and planted worlds use a fresh one. Test 1.11.

**DQ-S5 — "Waives nothing" turns red at T0 for every package not staged.**
- At T0 the row lints only the staged files' packages (design:238), so every excluded package not staged drops nothing. Step 8 (design:190) then reds each entry, and the pre-commit hook blocks.
- Fix: judge liveness only on full-tree runs (T1+). Test 3.5 (`test_waives_nothing_is_not_judged_on_a_staged_run`).

**DQ-S6 — Spellings of `//nolint` itself are missing from the census.**
- Measured (qa-design-exp/m2) with the §2.1 config:
  - golangci honours `// nolint:lll`, `//nolint: lll`, `//nolint:LLL`, `//  nolint:lll`, `//nolint:lll,`, bare `//nolint` and `//nolint:lll // x`;
  - it ignores `//NOLINT:lll`, `/*nolint:lll*/` and `//lint:ignore lll x`.
- Every honoured spelling but the last is red through nolintlint. `//nolint:lll // x` is red only through the suppressions checker's metadata rule.
- §4.1's census lists only foreign forms. Fix: add these rows to `directives.json`, and assert each honoured spelling is red in at least one named row (test 2.4, planted case 18).
- Census completeness (what W0 must measure):
  - For each enabled linter at the version golangci v2.15.0 vendors, and for golangci's own processors (nolint, exclusions, `//line`), read the source's comment-directive parsing and record `source: module@version file:line`.
  - Confirm each form with the binary, for the placements above, trailing, file-wide, block and the whitespace variants.
  - Done = every enabled linter has at least one row, including "none in source at <ref>".

**DQ-S7 — The restore cites the wrong revision.**
- design:313 says `git show fc944ef^:gate/runner/tests/test_breakit_s11c2.py`.
- Measured: `class NosecIsAWaiver` occurs 0 times at `fc944ef^`, once at `fc944ef` and 0 times at HEAD. fc944ef *added* the tests (+20 lines); the module's own docstring names fc944ef.
- A literal restore brings nothing back, and card line 2's "restored break-it tests" goes vacuous.
- Fix: `git show fc944ef:…`; the tests are `test_a_nosec_is_read` and `test_a_nosec_is_found_by_lawdeltas_prefilter` (test 2.8).

**DQ-S8 — Dropping `go_built` silently turns the tool scan off.**
- Covered is "6 + 1 per declared binary", and `expected_scope` follows `tools.json`, so removing `"go_built": true` from golangci-lint shrinks both and stays green.
- Fix: a pin test (`test_golangci_lint_is_declared_go_built`, 5.1), or `min_wants` 7 for `vulns`, or both.

**DQ-S9 — A binary exemption that matches nothing is not red (not verified).**
- `grep -n "unused|waives|matches no"` finds nothing in `gate/runner/rows/vulns.py` or `vulndb.py`.
- After an upgrade, the 15 `<id> golangci-lint` exemptions would sit inert until expiry, against the "waives nothing" principle the design applies to `smells`.
- Fix: test 5.5; if the code already has this rule, cite it.

**DQ-S10 — The AST-equality canonical form misses meaning.**
- "node type and literal values" (design:334) drops `Ident.Name`, every operator token (`BinaryExpr.Op`, `AssignStmt.Tok`, `BranchStmt.Tok`, `IncDecStmt.Tok`, `UnaryExpr.Op`) and `ChanDir`.
- Declarations "by name" collide for several `init` funcs, `_` vars and same-named methods on different receivers.
- "comments dropped" drops `//go:noinline`, `//go:build` and `//go:linkname`.
- `reflect.DeepEqual` is not an escape: `[reflect]` is red in `tools/`.
- Fix:
  - Print every non-`token.Pos` field.
  - Key methods by receiver type and name, and number `init`/`_` declarations in source order.
  - Keep `//go:` comments.
  - Tests 7.2 (bold rows). The tool is the lanes' proof of purity, so a weak tool is a misleading proof.

**DQ-S11 — `tools/shape`'s characterisation has no completeness rule.**
- design:335 leaves the construct kinds UNFINISHED, and a golden test is green from birth.
- Fix: completeness = 100 % statements of `shape` and `shapes` before the first refactor commit, with one mutation per arm shown red in the record. Test 7.3.

**DQ-S12 — "No hot path in lane D's four" is not measured.**
- `Table.checkValue` and `RuleClock.fireGroup` read as per-row and per-tick paths (inferred from their names; not checked in the graph).
- Fix: W0 checks whether the runtime's per-event flow reaches them (GitNexus `impact` upstream). If it does, the benchmark and `AllocsPerRun` pin come first (decision 6). Test 7.4.

**DQ-S13 — The planted world must prove golangci linted both GOOS, not only that `go list` claims the files.**
- Covered comes from `go list` (design:204), which proves a file is claimed, not that golangci linted it.
- `env.PINNED` gives both commands `CGO_ENABLED=0`, `GOFLAGS=""` and `GOWORK=off` (gate/runner/env.py), so they agree only if the row passes the same env to both.
- Fix: test 1.11 (the env captured from both calls is equal) and planted case 23 (a `_linux.go` and a `_darwin.go` finding are each red on any host).

### Later

- **DQ-L1:** `differences` compares a list whole (design:168), so a change inside `revive.rules` prints the whole array. Index the path (`linters.settings.revive.rules[1].arguments`).
- **DQ-L2:** under a tmp dir on macOS, `go list`'s `Dir` (`/private/var/…`) and the root (`/var/…`) differ, so every file is unclaimed. This fails loud. Use `realpath` for the root; planted worlds will show it at once.
- **DQ-L3:** the instruments' `code` lists omit the new imports (`pin.py`; for `suppressions`: `parts.py`, `golangci.py`, `git.py`, `sprintid.py`, `aging.py`, `knownissues.py`).
- **DQ-L4:** pin the green summary line's format (`smells: <n> issues dropped by <m> exclusions …`) with a test, since the owner's check reads it.
- **DQ-L5:** planted run cost: 30 worlds × 2 lints. Measure in W0 before grouping, and check whether `teeth` admits shared-run cases (not verified).

## 3. Answers to R-1 … R-9 (testability view)

- **R-1:** keep explicit PERMANENT `unlinted <path>` entries.
  - They are testable both ways: an entry that becomes claimed is red, and an untracked entry is red.
  - A blanket `testdata/` skip silently admits any production file moved under `testdata/`.
  - Tests 3.6.
- **R-2:** yes, G1's answers should name S1.1c-5-1…-4, so `aging.decided` holds.
  - Add a test now that runs aging on a fixture of the S1.1c-7 close, so S3.3-2-1/-2/-3 turning red is known in advance, not found at that close.
- **R-3:** keep `os.Environ` (planted case 11; 0 uses today). `log.Print*` is the owner's call; if added, add one planted case.
- **R-4:** waive, unless lane C first writes an aliasing test that is red against a naive rewrite: two records, mutate the buffer after the first, assert the first is unchanged. The playback tests today are single-record (PQ-S10), so a rewrite has no net.
- **R-5:** add `gate/known-issues.json`, `gate/sprints.json` and `gate/current-sprint` to `suppressions`' `expected_scope`, so deleting one is a covered mismatch.
  - The merge-base file is not in the tree; its absence is DQ-B3's rule.
- **R-6:** measure first.
  - Whatever T0 keeps must give the same verdict as T1 on the staged packages.
  - Liveness is off at T0 (DQ-S5).
  - Test `Staged.test_each_staged_files_package_is_linted_once` (`./<dir>`, no `/...`) and PQ-L2's test that no workflow passes `--new*`/`--whole-files` at T2+.
- **R-7:** no row, but not record-only.
  - Its Go tests run at T2 with the tree's Go tests.
  - The coordinator re-runs `astequal <rev>` for each lane-D move/reflow commit at G5 and quotes the exit codes (ground rule 17: an agent's claim is verified, not relayed).
- **R-8:** measured here:
  - `Report.Warnings` = `[{"Tag": "runner/source_code", "Text": "…"}]`;
  - `typecheck` is in `Report.Linters` with `Enabled: true`.
  - Still unmeasured: a function-level `//nolint` over a body, `warn-unused`, and the linux binary's build info (W0).
- **R-9:** use `gate/runner/git.py`'s `merge_base` (:23), which already exists.
  - Test it on a fixture repo with a base, without one, and with the file absent at the base.
  - No base at T4/nightly is red (DQ-B3).

## 4. Verdict

Not build-ready for its acceptance lines 1–3. Three blockers let the gate pass when it should fail: a forward moved by re-pointing an entry or adding one (DQ-B1), a `//line` relocation the row's own drops then forgive (DQ-B2, measured), and a skipped move check at T4 (DQ-B3).
Each of the three has a small fix and a named red-first test above. The §8 table is now complete: 30 planted smells cases, suppressions planted 9 → 13, vulns planted 31 → 35, `proofs` `min_wants` 38 → 39, `vulns` `min_wants` 6 → 7.
Fold in the should-fixes before G1, chiefly the type-strict pin, the stale report file, the root `.` package, liveness at T0, the restore revision, and the AST-equality canonical form.
