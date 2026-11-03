> **Status (YYYY-MM-DD):** <added at close-out: which blockers were fixed in which PR; which later items are open in known issues>.

# S1.1a-1 break-it — the local gate and the event kernel (breaker: QA + the-fool)

**Base:** a1e4646 (`s1-1a-1/integration`). **Branch:** `s1-1a-1/breakit`, not pushed.
**Red-test commits:** e21f4ce (QB1, QB2), a1eb0ff (QB3), e31f35e (QB4), 624185b (QB5), 9ac345e (QB6).
**The five break-it modules through the shim** (command at the end): 19 tests; the only failures are the 12 new red tests (the other 7 are their positive witnesses, and they pass). **`make check` on the branch:** every row PASSes except `pytests`, which is `exit 1` and names exactly the five break-it modules; the 37 existing modules pass; `ruff` covers 107 files (102 + 5) and passes.

**Result:** 2 blockers, 2 should-fix, 6 later (3 of them without a test: an owner question, and two items whose right behaviour the design does not pin).

Method: read the design note v2.1, the record, the plan check's attack lists, and every file under `gate/` and `internal/event/` adversarially; probed each suspicion with throwaway scripts in the scratchpad before writing a red test. Real inputs: the pinned tools, real `go test`/`go vet`/`go list` on scratch modules, crafted `.tar.gz` fixtures through the real installer, the real runner (`bootstrap.py exec`) on a fresh clone at a path with spaces and unicode, SIGINT/SIGTERM sent to the real runner at four moments, two concurrent `make check` in one detached-HEAD clone, and the real `ruff.toml`. Not tried: Linux (no host), a real full disk, network failures during `make tools`, `make strict-gate` (T4) on the branch.

## Blockers

### QB1 — A tracked test module under `gate/proofs/` outside the top of `drivers/` is run by no row, and the gate passes over its failing test (false green)

- **Where:** `gate/registry.json` (`pytests` scope `:(glob)gate/**/test_*.py` + `:(glob,exclude)gate/proofs/**`; `proofs` scope `:(glob)gate/proofs/drivers/test_*.py`); `gate/runner/rows/pytests.py` `misplaced()` (lines 26–42), whose docstring promises to find "test files that would silently never run".
- **Scenario:** a developer adds `gate/proofs/tests/test_row.py` (the natural home for tests of `gate/proofs/row.py`), or `gate/proofs/test_x.py`, or `gate/proofs/drivers/more/test_x.py`. `pytests` excludes it, `proofs` does not match it (`*` does not cross `/`), and `misplaced()` flags only `*_test.py`, stray files under `tests/` and untracked `test_*.py`. Its tests never run; `pytests` PASSes `covered 1`.
- **Evidence:** `gate/runner/tests/test_breakit_rows.py`, class `QB1TestModulesNoRowRuns`: three red tests, each `AssertionError: 'covered 1' != 'exit 1'`; the witness `test_qb1_witness_the_failing_module_is_red_where_pytests_looks` shows the same failing module elsewhere turns the row `exit 1`, naming it. (Lane A's `test_proofs_are_outside_this_row` covers only a real driver, which `proofs` does run.)
- **Honest likelihood:** low to moderate: the first test anyone writes for anything in `gate/proofs/` other than a driver lands here (Lane C put `gate/proofs/row.py`'s tests in `gate/coverage/tests/test_proofs_row.py`, perhaps for this reason). When it happens, a failing test is reported green indefinitely.
- **Fix area:** `pytests.misplaced()`: flag any `test_*.py` under `gate/proofs/` that is not `gate/proofs/drivers/test_*.py` (or narrow the `pytests` exclusion to `gate/proofs/drivers/**` so such modules run at T2). Widening the `proofs` scope instead would leave them T4-only and would not turn these tests green.

### QB3 — A FIFO at a hashed path hangs parity and `bootstrap.py exec` (hang)

- **Where:** `gate/toolchain/parity.py` `_differs()` (lines 124–132) and `gate/toolchain/install.py` `exec()` (lines 315–321), both through `manifest.sha256_file()` (a plain `open()`); the whole-tree walk is safe (it hashes `S_ISREG` only).
- **Scenario:** any `exec_sha256` entry that is not the version-probed binary (e.g. go's `bin/gofmt`, `pkg/tool/…/compile`) or the gate's `bin/python3.15` is replaced by a FIFO. `open()` for reading blocks until a writer appears, which never happens. In a run, parity never returns, so the run waits for the ceiling (2340 s at T2) and reports "run ceiling exceeded" rather than the tool; `make check`'s `bootstrap.py exec` has no ceiling and hangs forever.
- **Evidence:** `gate/toolchain/tests/test_breakit_fifo.py`: `test_qb3_parity_names_a_fifo_at_a_hashed_path_instead_of_hanging` → `AssertionError: hung for 30s opening the FIFO …/go-1.0-abababababab/bin/gofmt to hash it; after release it gave [['go: sha256 of bin/gofmt differs: run make clean-tools tools']]`; `test_qb3_exec_refuses_a_fifo_interpreter_instead_of_hanging` → `hung for 30s … bin/python3.15 …; after release it gave [(2, ['python: sha256 of bin/python3.15 differs from tools.json: …'])]`. Witnesses: other bytes at the same paths are named at once.
- **Honest likelihood:** very low: it needs a FIFO planted in a read-only cache (deliberate tampering or filesystem damage). It is a blocker by the definition (a hang), and the fix is small; the owner may prefer to forward it.
- **Fix area:** `manifest.sha256_file` (or both callers): `lstat`/`fstat` and refuse anything but a regular file, or open with `O_NONBLOCK` and check `S_ISREG` on the descriptor before reading.

## Should-fix

### QB2 — Renaming one of `pyfloor`'s four test modules shrinks the row unseen; that module's subject loses its 3.12 floor check (latent missed detection)

- **Where:** `gate/registry.json`, the `pyfloor` row: `expected_scope` lists four literal paths and `min_wants` is 0 (line 124); `gate/runner/scope.py` `_git()` (`git ls-files` drops a pathspec that matches nothing, silently).
- **Scenario:** `gate/toolchain/tests/test_manifest.py` is renamed, moved or split (e.g. `test_install.py`, the largest, split into two). `pyfloor` now runs three modules, `covered 3 = expected 3`, PASS; `pytests` still runs the renamed module under 3.15 only. The next 3.13+ API in `manifest.py` (which `install.py` imports) is no longer caught. `proofs` was given `min_wants` 10 for exactly this; `pyfloor` was not.
- **Evidence:** `test_breakit_rows.py` `test_qb2_a_renamed_floor_module_does_not_pass_unseen` (uses the registry's own `pyfloor` row): `AssertionError: 'pass' != 'fail' : pyfloor passed without gate/toolchain/tests/test_manifest.py: {… 'covered': 3, 'expected': 3, 'reason': 'covered 3', 'verdict': 'pass' …}`. Witness: the four modules as registered → `covered 4`.
- **Honest likelihood:** low to moderate (renames and splits of test modules are routine); the harm is silent until the floor is next broken.
- **Fix area:** `pyfloor`'s `min_wants` = 4 in the registry (coordinator-owned), or a scope rule that every literal pathspec must match (`git ls-files --error-unmatch`). Either turns the test green.

### QB4 — A stopped row leaves its new-session descendants running forever (leak; seen for real)

- **Where:** `gate/runner/scheduler.py` `_on_watchdog`/`_kill`/`_on_stop` (lines 402–443) and `gate/runner/proc.py` `Group.signal` (lines 61–67): TERM then KILL reach the row's process group only. `gate/runner/pyshim.py` installs no SIGTERM handling, so a module's `addCleanup`s never run when it is TERMed.
- **Scenario:** the gate's own suite starts helpers in sessions of their own all the time: every in-test `Runner` spawns rows with `start_new_session=True`, and the toolchain tests' children do too. When `pytests` times out or is interrupted, each test module dies of TERM's default action before its cleanups run, and every such helper outlives the run and the gate, parented to launchd.
- **Evidence:** observed: one SIGINT to the real runner during `pytests` (a fresh clone, `make check`) left four `rowfix.py start … block` fixtures and a fake `git ls-files -z -- one.txt` running; I killed those (mine). This machine also holds **19 such orphans from earlier runs** (rowfix.py and pyshim from `s1-1a-1-lane-a`, parity fixtures from `gate-toolchain-*` temp dirs), the oldest over a day old; they are not mine and I left them. Red tests: `gate/runner/tests/test_breakit_leak.py`, `test_qb4_a_helper_in_its_own_session_is_gone_after_a_timeout` and `…_after_an_interrupt` → `AssertionError: pid 4244, started by the stopped pytests row, is still running 20s after the run ended` (each test kills its helper in cleanup). Witness: the same helper in the row's own group is gone after the timeout.
- **Honest likelihood:** high: every interrupted or timed-out `pytests` (or `proofs`) row leaks. The verdict is right ("interrupted"/"timeout"), so this is not a blocker; the orphans block forever and accumulate on a developer machine (CI runners are ephemeral). Related residue, not separately tested: a row whose own process daemonizes (`setsid`) and then exits 0 is PASS with the daemon still running; on macOS there is no subreaper, so the group probe cannot see it.
- **Fix area (owner's choice):** (a) test side: the shim turns SIGTERM into an orderly stop (raise inside the running test so its cleanups run, then exit non-zero) within the 5 s grace; or (b) runner side: snapshot the row's descendant tree (ppid chain) before sending TERM and KILL every member after the grace. Either turns both red tests green.

## Later

### QB5 — A `.tar.gz` whose tar stream is cut inside a header installs half a tool and stamps `.complete`

- **Where:** `gate/toolchain/install.py` `_Extraction.extract()` (lines 92–99): `tarfile` treats a `TruncatedHeaderError` at a non-zero offset as the end of the archive, silently.
- **Scenario:** a whole gzip stream around a tar stream that stops 200 bytes into the second header. The installer extracts the first member, stamps `.complete`, prints `demo 1.0: installed`, exits 0; every later `make tools` says "present".
- **Evidence:** `gate/toolchain/tests/test_breakit_archive.py` `test_qb5_a_tar_stream_cut_inside_a_header_is_refused` → `AssertionError: 0 != 2 : a truncated archive was published: ['demo 1.0: installed'], bin/ holds ['demo']`. Witness: the whole archive installs both members.
- **Honest likelihood:** very low: the archive's sha256 is pinned, so this needs a pin taken from a broken archive. Parity would then catch it at T2 ("tree differs") only if `tree_sha256` came from a different extractor.
- **Fix area:** after iteration, require the end-of-archive marker (two zero blocks) or that the reader consumed the whole stream; refuse otherwise, naming the URL and "check its pin".

### QB6 — Equivalent spellings walk past `ruff.toml`'s two false-green bans

- **Where:** `ruff.toml` `[lint.flake8-tidy-imports.banned-api]` (coordinator-owned law; it matches design §5's list exactly, so this is a gap in the list, not drift).
- **Scenario:** `os.waitpid(-1, 0)` and `os.waitid(os.P_ALL, 0, os.WEXITED)` reap any child exactly as the banned `os.wait` does ("it turns a failure into exit 0": `Popen.wait` then reports 0); `signal.Handlers.SIG_IGN` is `signal.SIG_IGN`. TID251 matches qualified names only, so all three pass. No instance exists in the gate today (grepped).
- **Evidence:** `gate/runner/tests/test_breakit_bans.py`: three red tests, e.g. ``AssertionError: 0 != 1 : `os.waitpid(-1, 0)` passes ruff check under the real ruff.toml``. Witness: `os.wait()`, `os.wait4(-1, 0)` and `signal.SIG_IGN` are flagged in the same world with the row's flags.
- **Honest likelihood:** low (someone reaching for `waitpid` in a test helper); the consequence is a false green.
- **Fix area:** add `os.waitpid`, `os.waitid` and `signal.Handlers.SIG_IGN` to `banned-api` (owner/coordinator: law data).

### QB7 — A Go test that skips passes `unit` and `coverage-inclusion` silently, while any Python skip fails `pytests` (owner question; no red test)

- **Where:** `gate/runner/rows/unit.py` (counts package-level ends; fails only "no test ran" over the whole run); `gate/coverage/classify.py` (fails only when every test of a package skipped).
- **Scenario / evidence (probe):** `internal/a` with `TestA` passing and `TestTheRealCheck` calling `t.Skip("flaky, skipped for now")`: `unit` → `GATE_COVERED 1`, exit 0; `coverage-inclusion` → `PASS ex.com/s/internal/a 100.00% (1/1) >= 90%`, exit 0. DEVELOPMENT rule 1 ("a skip is recorded with its reason, never as green") speaks of steps; the design is silent on per-test Go skips, and the later `skip: not-implemented(S<n>)` forms will need a policy.
- **Honest likelihood:** moderate once the engine grows; harm: a skipped Go test is invisible in the summary.
- **Fix area:** owner decision first (fail any Go skip in `unit` as the shim does; or list skips as WARN; or allow only the `skip: not-implemented(S<n>)` form). Not tested because the expected behaviour is undecided (ground rule 1).

### QB8 — Ctrl-C before the scheduler starts is reported as `internal error: KeyboardInterrupt`, exit 2 (no test: the design does not pin it)

- **Where:** `gate/runner/main.py` line 140 (`except BaseException`) around manifest load, `read_commit` (`git rev-parse`) and run-directory set-up, before `Runner.run` installs its handlers.
- **Honest likelihood:** low (a window of tens of milliseconds); harm: a user's interrupt reads as a gate bug.
- **Fix area:** catch `KeyboardInterrupt` in `main` and print an interrupt line with exit 1 (or 2) without "internal error".

### QB9 — A truncated gzip is refused as `internal error: EOFError: Compressed file ended before the end-of-stream marker was reached` (exit 2, no fix text; no test)

- **Where:** `gate/toolchain/install.py` `_install_tool`/`extract`: only `_Rejected` becomes a named refusal; `EOFError`, `tarfile.ReadError` and `zlib.error` fall to `main`'s catch-all. Exit 2 is right; the install.py docstring's "every refusal prints its fix" is not met. Same rarity as QB5 (needs a bad pin).
- **Fix area:** fold into QB5's fix: map read errors to the same "this archive cannot be installed: check its pin in tools.json" refusal.

### QB10 — Two test-module shapes can be replaced by another module at import (contrived; no test)

- **Where:** `gate/runner/rows/__init__.py` `_module_name()` and the shim's `loadTestsFromName`.
- **Scenario:** (a) `gate/a.b/test_x.py` and `gate/a/b/test_x.py` both map to `gate.a.b.test_x`: the second runs twice, counted twice, and the first never runs (covered 2 = expected 2); (b) an untracked package directory `…/test_x/` with an `__init__.py` holding a passing test shadows the tracked `…/test_x.py` (packages win at import), and `misplaced()` allows `__init__.py` under `tests/`.
- **Honest likelihood:** very low. **Fix area:** refuse dots in directory names under `gate/`, and refuse a directory named like a tracked module, in `misplaced()`.

## Tried and held

- **`GATE_COVERED` forged by tool output:** wrappers send tool output to files or echo it under `ruff> ` / `failed: `, so a forged line can only make "duplicated" (FAIL); Go rewrites unprintable characters in subtest names; the coverage instrument prefixes every line with PASS/FAIL. The counted-vs-expected and `min_wants` comparisons are right in every precedence slot (read against design §4).
- **Exit 0 but did nothing:** the shim's own report is required; `sys.exit(0)`/`os._exit(0)` at import, all-skipped, zero tests, import errors, untracked and misnamed modules are red (Lane A's and Lane C's tests). The report line can be forged by a module that prints it and then `os._exit(0)`s: adversarial only.
- **SIGCHLD:** reset in the runner, every wrapper, the shim and the coverage instrument.
- **Interrupts on the real runner** (fresh clone): SIGINT at 1 s (during parity), 6 s and 20 s, SIGTERM at 20 s, and a double SIGINT: every unfinished row "interrupted", the summary written, `GATE FAILED`, `exit: 1` within 0.3 s of the last signal; the runner's own process group was empty afterwards (QB4 is the only leak).
- **Two `make check` at once** in one clone on a detached HEAD: both PASS. **A repository at a path with spaces and unicode** (`odd path/ré pö 名/repo`, a real clone, not a worktree): `make check` PASS.
- **Unwritable run directory** (parent read-only, or the directory itself read-only): `internal error: PermissionError …`, `exit: 2`, never a pass. Full disk read for the same paths: every write is in the scheduler or a worker, whose exceptions end the run with exit 2.
- **Registry and `tools.json` edge cases:** duplicate keys, `NaN`, `true` for an int, floats, lone surrogates (exit 2 at load or at spawn), `VORSAR_CEP_WORKERS` with non-ASCII digits, relative paths with `..`/empty components, trailing-slash `exec_sha256` keys, an empty exec map. Note, not a defect: the manifest does not require `path_dir/command` to be in `exec_sha256` (true today for all five tools by data).
- **Hostile archives:** Lane B's 40 rejection fixtures, plus case and NFC/NFD folding on APFS (`O_EXCL|O_NOFOLLOW` refuses the second name), a hard link to a symlink, symlink loops, a directory where a file is expected (parity names it). Only truncation got through (QB5, QB9).
- **Tampering:** at T0/T1 parity hashes only the listed executables (accepted at G1); the rows' `gofmt` and `git` are never path-checked, so a `gofmt` planted in an earlier cache `bin/` would run at T1 — inside that accepted residue, and caught at T2+ by `tree_sha256`. The tree digest's line format is ambiguous only for root-level names containing newlines, which can hide deletions but never a changed file (and `exec_sha256` pins the binaries independently).
- **Event kernel:** 200 000 attributes with the duplicate last (rejected in 10 ms); mutating the caller's slice or a returned `Attr` changes nothing; NFC and NFD names are distinct (byte compare, as specified); `Type(255)` and `timeAttr` = `MinInt` are rejected with distinct messages; `Compare` over every combination of int64/int/uint64 extremes is a strict, antisymmetric total order under three shuffled sorts. The imports test reads every non-test `.go` file. No defect.
- **Determinism:** failure lists, verdict lines and parity reasons are sorted or in registry/`tools.json` order; `scope_sha256` is over sorted paths/import paths; `PYTHON*` is dropped from row environments and every child runs with `-I`.
- **Go scopes:** a directory whose files are all `//go:build ignore` is skipped consistently by `go list`, `go test` and `go vet`; test-only and external-test-only packages count as the formula says.
- **The proofs:** each driver asserts the exact red reason, `GATE FAILED`/`exit: 1` and a parity PASS control, and breaking each instrument (version check, path check, hash check, coverage floor, inclusion, the vacuity checks, `--ignore-noqa`, the format check, the 3.12 interpreter) would turn its driver red; apart from forwarded item (ii), no driver can pass with its instrument broken.

## Commands

    cd <worktrees>/s1-1a-1-breakit
    T=$HOME/.cache/vorsar/cep/tools
    export PATH=$T/go-go1.27.3-dd3908cf2095/bin:$T/python-3.15.2-285a6b924713/bin:$PATH
    for m in gate.runner.tests.test_breakit_rows gate.runner.tests.test_breakit_leak gate.runner.tests.test_breakit_bans \
             gate.toolchain.tests.test_breakit_fifo gate.toolchain.tests.test_breakit_archive; do
      $T/python-3.15.2-285a6b924713/bin/python3.15 -I -B -X dev -W error gate/runner/pyshim.py "$PWD" "$m"
    done

Each module ends `pyshim: <module>: failed: n failures, 0 errors` (rows 4, leak 2, bans 3, fifo 2, archive 1); the witnesses pass. `make check` on the branch: only `pytests` FAILs, naming exactly these five modules.
