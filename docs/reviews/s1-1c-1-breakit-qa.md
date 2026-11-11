> **S1.1c-1 combined build, 2026-11-11 — QA's break-it.** Read against `s1-1c/integration` @ d039fbe (the sprint's diff from `main` 9cd4188); every finding's disposition is in [the record's findings table](../sprints/s1-1c-1-record.md#findings) and the owner's answers in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-11--s11c-1s-fix-round). Sibling: [architecture](s1-1c-1-combined-architecture.md). Recommendations only.

# S1.1c-1 QA break-it report

Worktree /Users/ramith/code/cep-wt/s1-1c-1-breakit, branch s1-1c-1/breakit from s1-1c/integration @ d039fbe.
Commit: 63350ce test(gate): QA break-it for S1.1c-1 - five red tests (gate/runner/tests/test_breakit_s11c1.py).
Run: python3.15 -B -X utf8 -m unittest gate.runner.tests.test_breakit_s11c1 -> 5 failures, 0 errors.
GitNexus: not used (local augment reported the graph DB held by another process; code read directly).
Hindsight: 0 searches (no design choice was questioned; every finding is against the design's own text).

## Red tests (all in gate/runner/tests/test_breakit_s11c1.py)

1. SHOULD-FIX (misleading) - DocLine.test_a_changed_heading_names_its_own_section and
   DocLine.test_a_new_section_appended_names_the_new_section.
   Assertion: `'§1 — SPEC changed — owner decision required' != '§2 — SPEC changed — owner decision required'` (both).
   Cause: gate/runner/law.py:174-176 `_heading_above(new_lines, j1)` scans strictly before j1; when the first
   changed line IS a heading (a renamed section, a new section appended), the block names the section above it.
   Fix: for the new side, if `new_lines[j1]` matches HEADING use it (scan `lines[:j1+1]`); same for the old side on
   a pure deletion whose first deleted line is a heading.

2. SHOULD-FIX (silent pass on a mis-written manifest) - LawDeltaRow.test_a_directory_entry_without_a_slash_is_not_live.
   Assertion: `0 != 1 : GATE_COVERED 1\nLAW Δ: HEAD is main — read the PR run`.
   Cause: gate/runner/rows/lawdelta.py:56 liveness is `git ls-files -- <path>` non-empty, which a directory
   satisfies; gate/runner/law.py:120-132 `entry_for` only treats a path ending "/" as a prefix, so nothing under
   `gate/runner` (no slash) ever matches; on main the row passes having checked nothing there, on a branch with a
   change there it fails with the defensive "changed under a listed prefix but matches no entry" (a red, but the
   wrong text). The real manifest is correct and test_law_pin pins its entries, so this is a boundary, not a live bug.
   Fix: liveness for a non-prefix entry is `ls_files([path]) == [path]` (exactly the file); the failure text
   "is listed in gate/law.json but git tracks nothing there" fits as is, or "is a directory; list it as <path>/".

3. SHOULD-FIX (missed detection with a workaround) - DepsGather.test_a_replace_directive_is_named.
   Assertion: `[] != ['deps: example.com/dep: replaced in go.mod by example.com/dep v1.0.0']`.
   Cause: gate/runner/rows/deps.py:67-68 reads `obj["Version"]` (the require line) and drops `Replace`; `go mod
   graph` also shows the require version. A `replace` to another version, another module path or a local directory
   therefore matches the allowlist's version and passes. The licence fingerprint reads the replacement's Dir (good),
   so a licence mismatch on an imported module is still caught. Sharper point for the owner: go.mod and
   gate/proofs/go.mod are not in gate/law.json, so the LAW Δ block does not show a replace or a require change
   either; the deps row is the only guard and it reads the require side. Workaround: review go.mod in the PR.
   Fix: in `gather`, an object with `Replace` is a failed line naming the replacement (path, version or Dir); or
   carry it on `Module` and let `compare` demand an allowlist field for it. Note the W0 driver
   gate/proofs/drivers/test_deps_planted.py plants its module through `replace example.com/planted => ../planted`,
   so the fix changes that world's expected lines (or its allowlist entry must name the replacement).
   Question for the owner: should go.mod and gate/proofs/go.mod be law paths (kind code, grouped)?

4. LATER (misleading count) - LawDeltaRow.test_a_staged_rename_under_the_law_paths_is_one_edit.
   Assertion: `2 != 1`.
   Cause: gate/runner/git.py:71-74 counts NUL records of `git status --porcelain -z`; a staged rename emits two
   (new path, old path), so one rename reads "2 law files have uncommitted edits". By git's default an untracked
   file under a law path is also counted (not verified in the box).
   Fix: parse the records (skip the record following an `R`/`C` status) and decide whether `??` counts.

## Probes that found a weakness without a test (no red test written; owner-accepted or polish)

- LATER: deps.classify_license (gate/runner/deps.py:97) fingerprints a GPL text that cites "the Apache License,
  Version 2.0" as Apache-2.0 (probe: `gpl-cites-apache -> Apache-2.0`); a first-match substring fingerprint is the
  design's decision 4, so forwarded as a question: check GPL/LGPL/AGPL/MPL negatives first, or require the phrase
  near the top of the file.
- LATER: `_delta_registry` says nothing for a top-level change (`schema` 1 -> 2 gives `gate/registry.json +1/−1`
  with no one-liner); two rows with one `id` are diffed as one row (last wins). `doc_line` for an EOL-only change
  prints the flag alone (acceptable).
- LATER: `Order.index("S1.1c-2")` is S1.1c's index (12) and `reached("S1.1c-2", "S1.1c-1")` is True; the part guard
  lives only in load_entries (pinned by design). Rows S1.1a-1/S1.1a-2 can never be named by an entry (load_entries
  refuses n != 0) although they are rows of gate/sprints.json; both closed, so harmless today.

## Tried and held (one line each)

- Every real law file parses on both sides: delta(kind, t, t) == "" for all 11 data kinds; a planted edit per kind
  yields the named one-liner (registry `row lawdelta: timeout_s 60 → 61`; exports multi-line `1 signature changed
  (cep.Config)`; clock.golden `clock.BatchClose`; ceilings; tools (+tool/−tool on a rename); sprints `S1.1c closed`;
  notbuilt `S4.1 → S4.2`; law `kind registry → code`; edges `+1 edge internal/event → internal/x`).
- A two-word exports line -> `unparseable (new)`, no crash. A 2-part line continuation (`}`, tab) is read.
- Marker: CRLF, BOM, two lines, no trailing newline, empty -> each its one-line error. Ids S3.3-2x, S03.3, S3.3-0,
  s3.3, S3.3-1a refused; S3.3a-1 accepted.
- Plan table: a ✓ in the Depends-on cell -> "line 4: ✓ outside the id cell"; `S4.1 ✓ ([record](x.md))` accepted.
- entries: [] -> expired() is [] (a pass, covered 1) - by design, TestCorpus guards the list.
- Duplicate keys: law.loads refuses them in every JSON kind (registry included).
- Code read, not run: read_text decodes with errors="replace", so a BOM or Latin-1 LICENSE does not crash;
  LICENCE or a LICENSE in a subdirectory is a named red ("no LICENSE file in"), never silent; a tracked
  testdata/go.mod would be a named third module (ADR-0001 text); `spdx_parts("MIT AND ")` is a named red;
  go/toolchain graph nodes dropped; a law.txt of blank lines is NO_BLOCK (a fail); the scheduler's _law_block
  reads _finished for every selected row (_not_run fills it), so no KeyError on a stopped run.
- T0 hook on the commit: parity, gofmt, archcheck, ruff all green on the branch.

## Not tried in the box

- The record channel under a watchdog TERM mid-diff; summarize with two lawdelta records in two jobs; a unicode
  or space path and a binary file under a law prefix end to end; the pins removed from go.mod in a planted world
  (the designed red); the deps row's cap on a slow `go list`; concurrency of two rows running `git status`
  (read-only, index refresh is opportunistic - no failure expected). The Go loader tests were run last (result in
  the final message).
