> **Historical — S1.1c-4, the scans; merged in PR #40.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-13--s11c-4s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-4-plan-check.md) · [design](../sprints/s1-1c-4-design.md) · [record](../sprints/s1-1c-4-record.md) · reviews: [plan check architecture](s1-1c-4-plan-check-architecture.md), [plan check qa](s1-1c-4-plan-check-qa.md), [design architecture](s1-1c-4-design-architecture.md), [design qa](s1-1c-4-design-qa.md), [combined architecture](s1-1c-4-combined-architecture.md), [breakit qa](s1-1c-4-breakit-qa.md). **← [S1.1c-3](../sprints/s1-1c-3-record.md) · [S1.1c-5 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-4 combined build, QA break-it (blind), 2026-11-13** — QA on `s1-1c-4/integration` @ 1b06251 (step 7); each break a committed red test on `s1-1c-4/breakit` (9474d14, a851188, 389e06c). Every finding is placed in [the sprint record](../sprints/s1-1c-4-record.md)'s findings.

# S1.1c-4 step 7 break-it (QA with the-fool): report

Worktree /Users/ramith/code/cep-wt/s1-1c-4-breakit, branch s1-1c-4/breakit, base 1b06251. Three commits, six red tests, all in new files:
- 9474d14 gate/runner/tests/test_breakit_s11c4.py (test_bk_1..4)
- a851188 gate/toolchain/tests/test_breakit_s11c4.py (test_bk_5)
- 389e06c gate/canary/tests/test_breakit_s11c4.py (test_bk_6)
Each module was run alone with the pinned python3.15. Each red is quoted below. The pre-commit gate (T0) exited 0 on each commit. No new driver is needed: every break shows at unit level.

## Breaks

### BK-B1 (blocker, a crash): a zip member that fails its CRC crashes the vulns row
- Test: gate.runner.tests.test_breakit_s11c4.BreakIt.test_bk_1_a_member_whose_crc_fails_is_a_named_fault_not_a_crash (9474d14)
- Red: `AssertionError: scan raised BadZipFile: Bad CRC-32 for file 'ID/GO-2099-0200.json'`
- Where: gate/runner/rows/vulns.py:61-74 `Copy.read` catches only KeyError. A CRC failure (zipfile.BadZipFile) escapes through vulndb.check_db -> open_copy -> scan -> main and ends as a traceback. A corrupt deflate stream (zlib.error), an encrypted member (RuntimeError) or an unknown method (NotImplementedError) take the same path. make vulndb does not stop it: gate/toolchain/vulndb_fetch.py:76-92 `_validated` reads only index/db.json, so a body with one corrupt ID member gets stamped with a matching sha256.
- Severity: rule 15 counts a crash as a blocker. The row does fail closed (exit non-zero), but it gives no named line, so the design's "every fault one line" does not hold.
- Fix: `Copy.read` turns BadZipFile, zlib.error, EOFError, RuntimeError and NotImplementedError into a named fault, e.g. `vulns: member <name> is corrupt: run make vulndb` (a MemberUnreadable beside MemberTooLarge, handled in check_db's `member()` and in `entries_of`). `_validated` also runs a CRC pass over every member, capped at MEMBER_CAP each, before anything is replaced.

### BK-B2 (blocker, a missed detection): a replacement whose path is also in the build list is never scanned
- Test: ...test_breakit_s11c4.BreakIt.test_bk_2_a_replacement_that_is_also_in_the_build_list_is_still_scanned (9474d14)
- Red: `AssertionError: 'vulns: GO-2099-0201 example.com/z v1.0.0: fixed in 1.5.0' not found in []`
- Where: gate/runner/rows/vulns.py:184-194 `_module_map`, and the `modules: Mapping[str, str]` argument of vulndb.findings (gate/runner/vulndb.py:163). The map keeps one version per path. With `example.com/a => example.com/z v1.0.0` beside `example.com/z v1.5.0`, a's replacement is written first (sorted by path) and z's own version then overwrites it. Go accepts this build: checkMultiplePathsUncached keys on path and version (go1.27.4 src/cmd/go/internal/modload/load.go:2036-2046). The vulnerable z v1.0.0 gets compiled and the row stays green. The repo has no such replace today.
- Fix: findings takes (path, version) pairs (or path -> set of versions). `_module_map` emits both the replacement pair and z's own pair. Each finding names its version, so a `<id> <module>` waiver still keys on the pair.

### BK-S1 (should-fix): a `withdrawn` that is not a time silently drops the entry
- Test: ...test_breakit_s11c4.BreakIt.test_bk_3_a_withdrawn_that_is_not_a_time_is_a_fault_not_a_skip (9474d14)
- Red: `AssertionError: False is not true : the malformed entry vanished without a word: []`
- Where: gate/runner/vulndb.py:181 `if entry.get("withdrawn"): continue`. Any truthy value (`true`, `"no"`, `1`) skips the entry: no finding, no fault. OSV's `withdrawn` is an RFC 3339 time, and x/vuln decodes it into *time.Time and refuses anything else. The review DA-S1 rule says an entry the row cannot judge is a named fault. Ranked should-fix, not blocker, because it needs a malformed database and the canonical one comes over TLS.
- Fix: skip only when `withdrawn` parses with `vulndb._time`. A present `withdrawn` that does not parse is a fault `(id, "", "vulns: <id>: withdrawn <v> is not an RFC 3339 time")`. `null` or absent means live.

### BK-S2 (should-fix): make vulndb loses the old copy when the stamp write fails after the zip is replaced
- Test: gate.toolchain.tests.test_breakit_s11c4.BreakIt.test_bk_5_a_stamp_write_that_fails_leaves_the_earlier_zip_and_stamp_as_they_were (a851188)
- Red: `OSError: [Errno 28] No space left on device`. It escapes `fetch` as an OSError, not a FetchError. The new zip is already in place beside the old stamp, and no fetch-error.txt is written.
- Where: gate/toolchain/vulndb_fetch.py:115-122. `os.replace(tmp, ZIP)` runs inside the try, but `_write_json(root, STAMP, ...)` runs after it and outside the failure handler. Design §2.3 says: "On any failure: ... the old copy and stamp left as they were, fetch-error.txt written ... exit 2". Instead main crashes with a traceback, and the row reds with `the zip's sha256 is not the stamp's`, an integrity fault the `vulndb` waiver cannot silence, until a later fetch succeeds. A Ctrl-C in that window gives the same result. A Ctrl-C during `get` leaves `.vulndb.zip.*.tmp` behind, because KeyboardInterrupt is not in _FAILURES (later).
- Fix: make one rename the commit point. Write the zip as `vulndb-<sha256>.zip` and the stamp naming it, so the stamp's os.replace is the only commit and the row reads the zip the stamp names. Failing that, at least put the stamp write inside the handler and on failure restore the earlier zip, kept as a hard link before the replace. Every failure writes fetch-error.txt and raises FetchError.

### BK-S3 (should-fix, the canary misleads): a record red for left-running processes is sorted as pins only
- Test: gate.canary.tests.test_breakit_s11c4.BreakIt.test_bk_6_a_record_red_for_left_running_processes_is_a_break_even_when_its_failed_lines_are_pins (389e06c)
- Red: `AssertionError: False is not true : the leak is not a break: pins ['gate.toolchain.tests.test_manifest.Load.test_the_committed_tools_json_loads_in_file_order_for_both_platforms (gate/toolchain/tests/test_manifest.py)'], breaks []`
- Where: gate/canary/sort.py:46-60 judges a red record only by `failures`. gate/runner/main.py:362-382 `_failures` reads them from the row's `failed:` lines, which rows.finish prints as the row ends. gate/runner/verdict.py:40-54 ranks a stop, a timeout, "left running processes" and a kill by signal above the exit code. So a candidate under which pytests prints only its pin red, exits 1 and leaves a child running produces the leak as the reason and the pin as the failures. The canary prints "Pins to update", exits 0 and never names the leak. A row that is stopped or killed after printing its `failed:` lines falls in the same class.
- Fix: in sort, a red record whose reason is not the exit-code kind (verdict.decide's `exit N...`) adds a break by its reason as well as sorting its failures. Or `_job_records` attaches failures only when the reason starts with `exit `.

### BK-L1 (later): the copy's age counts the stamp's own calendar day, not the UTC one
- Test: ...test_breakit_s11c4.BreakIt.test_bk_4_the_copy_age_counts_utc_calendar_days_for_a_stamp_with_an_offset (9474d14)
- Red: `AssertionError: 'vulns: the copy was fetched 8 days ago (2026-11-05), more than 7: run make vulndb' not found in []`
- Where: gate/runner/vulndb.py:248 `(now.date() - fetched.date()).days`, and :321 for `modified`. `.date()` is taken in the time's own offset. A stamp fetched at 2026-11-06T01:00:00+05:00 (2026-11-05T20:00Z) reads as 7 days old on 2026-11-13 instead of 8. Design §2.4 says "(UTC calendar days)", and _RFC3339 accepts any offset. Ranked later because make vulndb always writes `Z` and vuln.go.dev's `modified` is `Z`.
- Fix: `fetched.astimezone(UTC).date()` and `modified.astimezone(UTC).date()`, with `now` converted the same way.

## Probes that held
- Member names outside the layout (`..`, unknown names): named `not in the database's layout` (read check_db). A NUL in a name is cut off by zipfile, so the member becomes an alias of a layout name; duplicates are de-duplicated without a line (later).
- An entry whose `affected` is not a list: a `malformed` fault from both findings and check_db (read).
- Events with non-string versions, events not a list, unknown event keys: InvalidVersion or UnknownEvent faults, and the walk continues (read).
- A stamp with a naive datetime: refused by _RFC3339 (zone required), so the stamp is reported malformed (read).
- `v2.0.0-rc.1+incompatible` against `introduced 2.0.0-0, fixed 2.0.0`: affected. A pseudo-version against `introduced "0.0.0-2022..."`: affected. `go1.28rc1` becomes `1.28.0-rc.1` and is affected by `1.28.0-0..1.28.0` (all three run).
- Leading zeros in a version: InvalidVersion, a fault (fail closed) (read).
- Tidy parse on a CRLF diff: parsed to `('go.mod','-',('golang.org/x/mod','v0.42.0'))` (run).
- T4 go.sum line outside a git repo: `deps: the T4 go.sum check could not run: ...` (run). A staged but uncommitted change is caught, since `git diff HEAD` covers the index (read).
- A gate/proofs version that differs from the root's: the loader keeps the root's version for the vulns scan, but the deps row's two-versions line keeps the gate red (read). The vulns row alone is blind to the proofs version.
- Cancelled candidate job: the summary prints `canary: the candidate job ended cancelled` and exits 1; skipped T4 jobs print `no records from job` (read).
- A 200 response with HTML: is_zipfile refuses it, giving FetchError, with the old copy kept (read).

## Undone (45-minute box)
- Zip bomb by total ratio: `Copy.cache` keeps every member up to 4 MiB in memory, and nothing caps the total. One ~4 KB compressed member of 4 MiB per entry, times thousands of entries in a 64 MiB zip, means tens of GiB. Likely later or should-fix, untested.
- A huge member count: probably only slow and noisy. Untested.
- Redirects: install._fetch (urllib) follows https -> http redirects, and the stamp records CANONICAL_URL whatever the final URL was. A question for the owner, untested.
- fetch-error.txt `at` equal to `fetched`: the strict `>` hides a failure in the same second (later, untested).
- Canary feeds (two releases of one version, a pre-release above stable, a CPython rc, golangci-lint's `v` strip, lark yanked), the candidate's structural check, archive_pins, render_canary: not probed.
- pin_tests.json patterns are compiled and refused only if they match the empty text. A catastrophically backtracking pattern would hang `re.search`. The file is committed and reviewed, so later; untested.
- main._failure_blocks: a test with a docstring adds a description line, so the body is read as empty and the failure text is "". That fails safe as a break with empty text (later, untested).
- Duplicate waiver entries: not settled. My two fixtures had the wrong shape, and I did not read the loader's `seen` handling.
- GitNexus and Hindsight not consulted (time box). index.commit not read.
