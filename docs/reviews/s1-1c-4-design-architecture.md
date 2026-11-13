> **S1.1c-4 design note, architecture review with the-fool (blind), 2026-11-13** — the architect reviewer on [the design note](../sprints/s1-1c-4-design.md) v1 @ c4837ca. Every finding is placed in the note's §8.

# S1.1c-4 design note v1 (c4837ca): architecture review, with a the-fool pre-mortem

Reviewed blind and read-only. GitNexus: the index already has the W0 stubs (a cypher query over `gomods.py`, `vulndb.py`, `candidate.py` and `install.archive_pins` returns all 7 symbols). The `gitnexus://…/context` resource could not be read because this session has no resource tool, so the index commit is not quoted. Hindsight: not used (no Hindsight tool in this session). Measurements use the pinned go1.27.4 with `GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOPROXY=off CGO_ENABLED=0`, and their scratch files are in this folder.

Findings below: 3 blockers, 14 should-fix, 7 later.

## Blockers

**DA-B1. The `gomods` seam cannot carry what the `deps` row checks today, so a lane that follows it literally loses licence and one-version detection.**
- *Evidence:*
  - The W0 stub's `gomods.Module(path, version, replace, built)` has no `dir` (`gate/runner/rows/gomods.py:17-21`).
  - `deps.compare` reads `module.dir` for every licence fingerprint (`gate/runner/deps.py:234-245`). With `dir == ""`, every imported module goes red with "no source on disk".
  - `Loaded.modules` merges both build lists into one version each, the root's first (note 2.1). But the one-version check needs both versions (`gate/runner/rows/deps.py:83-87`, `two_versions_line`).
  - Note 2.1 says that check is "unchanged", and 2.7 says "`gather` becomes a call to `gomods.load`". With this seam, neither can be true.
- *Fix (note and stub, before QA binds W0 tests to the seam):*
  - Add `dir: str` to `gomods.Module` (the build list's `Dir`, root first, `""` when absent).
  - Add `versions: Mapping[str, Mapping[str, str]]` to `Loaded` (module root → path → version), or `two_versions: tuple[str, ...]` built by `deps.two_versions_line`.
  - In 2.7, list which `deps.compare` inputs come from which `Loaded` field.

**DA-B2. The `archive_pins` helper, as specified, either crashes every install or breaks the installer's stamp.**
- *Evidence:*
  - Note 4.1 has one helper `_extract_sealed` (extract and seal), and says "the installer's stamp file is written after the helper and outside the hashed tree, as today".
  - Today the stamp `.complete` is written inside `tmp` before sealing, and is sealed with the tree: `install.py:275-277`, `_seal(tmp, {**extraction.modes, _STAMP: 0o444})`.
  - `_seal` clears the root's write bit (`install.py:211`), so creating `.complete` after a sealing helper fails with EACCES.
  - Writing `.complete` outside the tree breaks `_stamped(home, …)` (`install.py:224`) and `_publish`'s race rule (`:240-246`). Every later install would then report "no finished install".
  - The hash ignores the stamp by name, not by place: `manifest.tree_sha256` skips the root's `.complete` (`manifest.py:279`, `:305`).
- *Fix:*
  - The shared helper is `_extract(archive, strip, dest) -> dict[str, int]`, with no seal.
  - `_install_tool` writes `.complete`, then `_seal(tmp, {**modes, _STAMP: 0o444})`, unchanged.
  - `archive_pins` does `_extract` → `_seal(tmp, modes)` → hash → `_restore_write` → `rmtree`, all inside `try/finally`.
  - Correct the sentence to: "inside the tree, excluded from the hash by name (`manifest.py:305`)".

**DA-B3. The `vulndb` waiver's semantics are undefined in three places. Read literally, either decision 2's unblock never works or the scan silently stops.**
- *Evidence (note 2.5):*
  - (a) The liveness rule, "one that matches no finding this run → remove it", is stated for all entries, and "the `vulndb` entry … never a finding". A literal lane therefore reds every `vulndb` entry as stale, so the unblock that decision 2 grants is always red.
  - (b) "With it in force and no usable copy the row passes with `not scanned`", but "usable" is not defined. A lane may treat any silenced fault as unusable. A waiver added for the 30-day stall, or for an 8-day-old copy during an outage, would then stop the scan of a readable copy for up to the expiry window (`allowlists.AHEAD_MAX` = 90 days). That is missed detection.
  - (c) The waiver silences "every copy and database fault of 2.4", including integrity faults: the sha mismatch, a non-canonical url, members outside the layout, the member cap, and modules/ID disagreement. Decision 2 names "an outage or a stalled source", not a copy that is not the database.
  - (d) Under "not scanned", every `<id> <module>` waiver matches no finding, so they all go red as stale. The unblock then fails whenever any id waiver exists.
- *Fix:*
  - The `vulndb` entry silences only the availability and age faults: no stamp, the last fetch failed, the copy older than 7 days, the database older than 30 days.
  - Integrity faults are never silenced.
  - It is live iff it silenced at least one fault this run. Otherwise: `vulns: vulndb: no copy or database fault to exempt: remove it`.
  - When the zip opens and `check_db` reports only silenced faults, the row scans and findings stay red. The reason names the silenced faults.
  - "Not scanned" applies only when no zip can be read.
  - Under "not scanned", `<id> <module>` entries are not judged stale.
  - Each rule gets a driver case.

## Should-fix

**DA-S1. `findings` stops at the first matcher fault.** That hides every other finding, and an upstream format change has no unblock.
- *Evidence:* 2.2 says faults are "raised to the row as exceptions carrying the entry id". The stub returns `list[Finding]` (`vulndb.py:50-58`). One entry with `last_affected` makes every PR red with one line, and no waiver kind covers it: the `vulndb` entry covers 2.4 only, and `<id> <module>` covers findings only.
- *Fix:* `findings` returns `(findings, faults)` and keeps going after a bad entry or module pair. Then ask the owner at G1 whether an `<id> <module>` TEMP entry may cover an unorderable pair.

**DA-S2. The `replace` rule from the plan check was dropped.**
- *Evidence:* Plan check §2 S1 says "a `replace` with a version is matched at its replacement version … a `replace` to a directory is named", and §6 (15) has both cases. The note never says how the row turns `Loaded.modules` into `findings`' `modules` mapping. The literal `{m.path: m.version}` checks the required version, not the version that ships (the case pinned by `test_breakit_s11c1.py:86-112`).
- *Fix:* State it in 2.5:
  - The key is the required path.
  - The version is `Replace.Version` when present.
  - A directory replace gives `vulns: <path>: replaced by directory <dir>: not matched` (red, or a reason line; choose one and pin it).
  - Carry `replace_path` and `replace_version` as fields rather than re-parsing the `replace` text (see DA-L6).

**DA-S3. The loader's error model is ambiguous and noisy, and its `.test` premise is wrong.**
- *Evidence, measured:*
  - Warm cache, both modules, `linux/amd64`, `go list -e -json -deps -test ./...`: 216 and 145 packages, 0 without a module, 0 `Error`, 0 `DepsErrors`. The generated `<pkg>.test` mains carry `Module` (Main: true).
  - The skip list in 2.1 is therefore empty, and a suffix rule only adds a hole.
  - Cold cache (empty `GOMODCACHE`): 4 packages carry both `Error` ("module lookup disabled by GOPROXY=off") and no module, plus 63 `DepsErrors` entries that reduce to 3 unique (stack end, Err) pairs. Across two platforms that is about 134 lines per row, in two rows.
  - 2.1 does not say whether the "no module" and `Err` lines are exclusive. It also does not say which ImportPath a `DepsErrors` line names.
- *Fix:*
  - No skip list: a non-standard package with no module is red (pin 0).
  - Name `Error` first, and "no module" only when there is no `Error`.
  - Read `DepsErrors` only when the stack's last package is absent from the stream, naming that package.
  - De-duplicate on (package, Err) across platforms, listing the platforms.
  - Append the fix: `run go mod download in <module>`.

**DA-S4. Exit 1 means "differs" for `go mod tidy -diff` and `git diff --quiet`, and the allowance compares raw lines.**
- *Evidence:*
  - `run()` raises `Failed` on any non-zero exit (`rows/deps.py:39-41`). Both commands exit 1 to mean "differs": measured `tidy -diff` exit 1 in both modules, with empty stderr. A lane would report "could not run offline" every time.
  - The root's tidy diff also removes 5 comment lines, a blank line, `require (` and `)` (measured). An allowance of "removed lines … exactly these" is red forever, or else needs a filter the note does not define.
- *Fix:*
  - Exit 0 means clean, exit 1 with a stdout diff means it differs, anything else is `Failed`.
  - Compare the sets of `require` (path, version) pairs in the current and tidy `go.mod` (parse `-diff`'s `tidy/` side, or run `go mod edit -json` on both), and the sets of `go.sum` lines. The allowance applies to those sets.

**DA-S5. The T4 `go.sum` line reads the ambient `CI` variable.**
- *Evidence:*
  - Ground rule 13 says "same checks locally and in CI".
  - No gate code reads `CI` today (grep), and `env.clean_env` passes it through (`gate/runner/env.py:10-14`). This would be the first verdict that depends on an ambient variable.
  - A developer's shell or IDE that sets `CI=true` changes the verdict, and so does a nested world that inherits it (`support.HIDDEN` does not hide it, `support.py:36`).
- *Fix (choose one):*
  - Key the line on the run's explicit `--commit` ("the tracked `go.sum` files equal `<commit>`'s").
  - Run it at every T4 against `HEAD`.
  - Put CI-only to the owner as a named exception to rule 13.
  - Separately: add `CI` to `support.HIDDEN`.

**DA-S6. The canary's structural check files every golangci-lint and lark upgrade as a break, and the python-build-standalone upgrade is undefined.**
- *Evidence:*
  - `tools.json`'s `strip` embeds the version: `golangci-lint-2.15.0-linux-amd64/`, `lark-1.3.1/`. 4.4 allows only `version`, `version_line`, `url`, `archive_sha256`, `exec_sha256`, `tree_sha256` to differ.
  - python-build-standalone 20261110 ships the same CPython 3.15.2 and 3.12.16 (`docs/reviews/s1-1c-3-research-vuln-canary.md:144`; plan check §0.12). The plan check's owner's check expects it listed as an upgrade.
  - Under the version-only rule in 4.3 it is either not an upgrade, or its url and digest changes count as structural faults.
- *Fix:*
  - Allow `strip` to differ only when it equals the pin's `strip` with the version substituted.
  - Define the python-build-standalone upgrade by (CPython version, build tag), with the summary reading `python 3.15.2+20261026 → 3.15.2+20261110`.
  - Count a same-version rebuild as an upgrade for the structural check.

**DA-S7. Candidate binaries run in the step that holds the token, and `archive_pins` leaves no tree to run them from.**
- *Evidence:*
  - In 4.3 and 4.7, the `canary-candidate` step carries `GITHUB_TOKEN` and the version lines come "from the Linux trees". Plan check A-S5 says "never on a step that runs a candidate binary".
  - `archive_pins` deletes its temp tree (4.1), so `version_argv` has nothing to run.
- *Fix:* Two steps:
  1. `canary-feeds` (token) writes `feeds.json`.
  2. `canary-candidate` (no token) downloads, pins, extracts the Linux tree into a scratch folder, runs `version_argv`, and writes `tools.json`.

  Pin it with `test_generate.py` (the token on exactly that one step, which runs nothing but the feeds).

**DA-S8. The canary goes silently green when the candidate job fails.**
- *Evidence:* In 4.6 and 4.7, `summary` has `if: always()`, reads the records artifacts, and exits 1 only on breaks. If `candidate` fails (rate limit, `CanaryError`), every T4 job is skipped, there are no records, and the output is "Upgrades: none" with exit 0. That is plan check risk 7.
- *Fix:* Pass `${{ toJSON(needs) }}` into the summary step. A job whose result is not `success` or `failure`, or that left no records, is a named break. The candidate's own error goes in an artifact and is quoted. A test covers this.

**DA-S9. The canary narrows confirm (c).**
- *Evidence:* Decisions.md confirm (c) says "every T4 job on both systems under one candidate". Note 4.7 runs each job "on its own runner (`RUNNERS`)", which puts everything but `determinism-macos` on Linux. Its test pins that "equal to `gate.yml`'s".
- *Fix:* A two-OS matrix per job, or the narrowing put to the owner at G1 as a question.

**DA-S10. `check_db`'s signature cannot do the layout and cap checks, and a missing or corrupt zip crashes.**
- *Evidence:*
  - `check_db(read, stamp, *, zip_sha256, now)` (`vulndb.py:30-41`) has no member list, so "members outside `index/` and `ID/`" and the per-member cap over every member cannot be checked through `read(name)`.
  - 2.4 has no fault for "no `vulndb.zip`" or "not a zip" (`BadZipFile`). Both end in a traceback.
- *Fix:*
  - Add `names: Sequence[str]`.
  - `read` raises `MemberTooLarge(name)`, which `check_db` turns into the cap fault.
  - Add `vulns: no vulndb.zip at <root>: run make vulndb` and `vulns: <root>/vulndb.zip is not a zip: run make vulndb`.

**DA-S11. The copy's atomicity has gaps.**
- *Evidence:*
  - A 200 response that is not a zip (a captive portal) replaces a good local copy. "The old copy left as it was" (2.3) covers only failures.
  - The row hashes the zip by path, then opens it again. Another worktree's `make vulndb` sharing `~/.cache/vorsar/cep/vulndb` can replace it in between, so the sha checked is not that of the bytes read.
  - "`fetch-error.txt` newer than the stamp" does not say whether that means mtime or the time written in the file.
- *Fix:*
  - The fetch validates the temp file (`zipfile.is_zipfile`, and `index/db.json` parses) before `os.replace`.
  - Temp files come from `mkstemp` in `copy_root`.
  - The row opens the zip once, then hashes and reads members from the same file object.
  - "Newer" compares the parsed time in the error line with the stamp's `fetched`, and equal counts as newer. Or: a successful fetch removes the error file first, so its presence alone is red.

**DA-S12. Ownership gaps (one owner per file per wave).**
- *Evidence:*
  - `gate/runner/main.py` gets `fetch-vulndb` from E1 (W1), and needs `canary-candidate` and `canary-summary` (4.7) with no owner.
  - `gate/runner/tests/test_breakit_s11c1.py:86-112` (`DepsGather`) patches `deps_row.run` and expects `.imported` as `-f` text. It cannot stay green once the calls move to `gomods.run` with JSON, yet 2.8 says "its existing tests green" and no lane owns the file. (GitNexus `impact(gather, upstream)` lists it as gather's only test caller.)
  - The `Makefile` canary targets, if any, are not assigned.
- *Fix:*
  - Give E1 the `DepsGather` port (patch `gomods.run`, JSON streams, platform names; keep the replace assertion).
  - Give F2 (W3) `main.py`'s canary commands, after E1 has merged.
  - Add both files to the table in §1.

**DA-S13. The residue line was dropped from the reason.**
- *Evidence:* Plan check §2 S1 says "Residue, named in the row's reason: build tags beyond the two platforms' defaults, `CGO_ENABLED=1`" (QA-L1, marked "fixed"). The green reason in note 2.5 omits it.
- *Fix:* Add `not read: build tags beyond linux/amd64 and darwin/arm64 defaults; CGO_ENABLED=1` to the reason, and pin it in the driver.

**DA-S14. The cap is set before the cold CI figures exist.**
- *Evidence:* Decision 3 says "the design adds the `proofs` job's nested parity and a `stress.py` run first and raises it if they are slower". Decision 6 of S1.1c-3's review says the draft PR "reads cold parity times". Note 3.4 measures `treehash.py` locally only, and §6 has no step after the push to read the draft PR's `parity.json` on both systems and raise `SLOWEST_PARITY_S`.
- *Fix:* Add that step to §6 between the push and review, owned by E2 or the coordinator in the fix round. State that `SLOWEST_PARITY_S` mixes whole-parity CI times with single-tree local times, and that this is conservative.

## Later

- **DA-L1.** 2.6 names `deps.check`, which does not exist; it is `deps.compare` (`gate/runner/deps.py:175`).
- **DA-L2.** "In force" is undefined for waivers. State that an expired entry silences nothing: its finding is red, and `suppressions` names the expiry (`allowlists._expiry_faults` reds an expired entry only at T4 and nightly, `allowlists.py:172-182`).
- **DA-L3.** Make the layout rule an exact whitelist: `index/db.json`, `index/vulns.json`, `index/modules.json` and `ID/GO-<yyyy>-<n>.json`. A root-level member does not "begin with a directory". Today's zip has 4,804 members, none of them directories, 4,801 under `ID/` and 3 under `index/` (measured).
- **DA-L4.** `go.dev/dl/?mode=json` lists only the two newest stable lines. Once go1.29 ships, go1.27 drops out. Use `&include=all`.
- **DA-L5.** Where `go_version` comes from is unspecified in 2.5's steps. Add `go env GOVERSION` through `run()` under `ENV`, giving one fault when it cannot be ordered, not one per stdlib entry.
- **DA-L6.** `Module.replace` as `"<path> <version>"` text makes the row re-parse it. Carry `replace_path` and `replace_version` as fields.
- **DA-L7.** Two rows now run 10 `go` invocations each in the `lint` job. Measure `vulns`' 180 s timeout at E1's first green and record it.

## Pre-mortem (the-fool, done by hand): the lanes built exactly this note, and the gate now lies or blocks

1. `make tools` fails on every machine: the stamp is written after a sealing helper (DA-B2).
2. `deps` is red on every module: no `dir`, so "no source on disk". Or a lane "fixes" that by dropping licence reads (DA-B1).
3. The root's tidy line is red forever on comment lines, or reports "could not run offline" on exit 1 (DA-S4).
4. During an outage the `vulndb` waiver is itself red as stale, so every PR is blocked. Or, once it is accepted, it stops the scan of a readable copy for 90 days and later silences a sha-mismatched copy (DA-B3).
5. One upstream `last_affected` turns every PR red with a single line and no exemption path (DA-S1).
6. A module replaced to a vulnerable version is matched at its required version, and the gate is green (DA-S2).
7. CI is red and local is green on `go.sum` because of an ambient variable (DA-S5).
8. The canary is red every week for golangci-lint and lark patch releases (DA-S6), and silently green when the GitHub rate limit is spent (DA-S8).

## Verdict

Not ready for G1 as v1: three blockers. The `gomods` seam is missing the `deps` row's inputs, the `archive_pins` helper as specified breaks the installer, and the `vulndb` waiver's scope and liveness are undefined.

All three are note edits plus one stub change (`gomods.Module.dir`, `Loaded` versions, `check_db` names) before QA binds W0 tests to the seams. The should-fix items are mostly a sentence each, and DA-S9 needs the owner at G1.
