# S1.1c-7 design note — A2 (gate/toolchain, canary, proofs, baseline, ci, hooks)

*S1.1c-7's design note for chunk A2 (gate/toolchain, canary, proofs, baseline, ci, hooks), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes.*


## The note

S1.1c-7 design note: chunk A2 (gate/toolchain, gate/canary, gate/proofs/drivers, gate/baseline, gate/ci, gate/hooks)

### 1. Header
- **Chunk:** A2. **Findings:** 21 (G10-5, G10-14, and 19 G2s).
- **Base:** `s1-1c-7/integration` @ d5217259fa39 (checked with `git rev-parse HEAD`). The brief says the GitNexus index is fresh at that commit. I did not read the context resource again.
- **Files read** (the cited ranges in full): gate/hooks/gitnexus_reindex.py; gate/hooks/tests/test_gitnexus_reindex.py; gate/toolchain/treehash.py; tests/test_treehash.py; parity.py (:130-150, :225-262, :303-372, :438-450, :586-652); manifest.py (:134-155, :266-282); install.py (:140-160, :307-331, :411-445); vulndb_fetch.py (:199-209); toolchain tests test_breakit_s11c3.py, test_parity.py (:620-690), test_parity_handoff.py (:180-365), test_tree_digest.py (:225-275), test_archive_pins.py, test_install.py (:30-110), test_vulndb_fetch.py (:165-195); gate/canary/candidate.py (:1-50, :215-256, :370-392); canary tests test_candidate.py (:300-460), test_fix_s11c4.py (:160-185), test_f1_feeds.py (:53-77), tests/support.py; gate/canary/pin_tests.json; gate/ci/generate.py (:50-125, :316-330); tests/test_generate.py (:1-50, :180-280); gate/baseline/regen.py (:120-160, :280-330); tests/test_breakit_s3_3_2.py; tests/test_regen.py (:28-53, :280-340); gate/proofs/drivers/test_lawdelta_planted.py; gate/runner/summary.py (:45-75); scheduler.py (:445-460); stress.py (:1-80, :254-265); gate/wait.py; gate/law.json; ruff.toml; the plan's §0-§8 and §11; testing.md.
- **GitNexus calls: 5.**
  - `impact` upstream on `running` and on `start` in gitnexus_reindex.py.
  - `context` on `_lines` (treehash), `_tree_failure` (parity) and `_platform_entry` (candidate).
- **Ran in memory, read-only:** one `python3 -I -B -c` that calls `generate.render` to list which jobs download modules. No test, gate or mutate run.
- **Could not check:**
  - I ran no proof, so every expected outcome below is reasoned from the code, not observed.
  - Whether W0's `--py` supports `--repeat`.
  - gate/coverage/: no finding there, and GitNexus does not index it, so I did not open it.

### 2. Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-5 | accepted in part (EPERM sub-claim rejected) | K1c | C9 hooks K1c | pid-reuse part → S1.1c-10 | an exclusive non-blocking `fcntl.flock` on `auto-reindex.lock`, held from the pidfile check to the start |
| G10-14 | accepted | K1 | C1 toolchain K1 | – | with no argument: a usage line on stderr and exit 2, not an IndexError traceback |
| G2-5 | accepted in part | K2a | C6 baseline K2 | moving the cases into `Main` → S1.1c-10 | replace `if token in printed:` with a witness on the value-change line plus `assertNotIn(token, printed)` |
| G2-8 | accepted | K2-del | C4 canary K2-del | – | delete the tautology; test_f1_feeds covers the branch |
| G2-9 | accepted | K2a | C3 canary K2 | – | assert both exact "moved" faults |
| G2-10 | accepted | K2-del | C4 canary K2-del | – | delete the docstring-wording test (proved by an unchanged covered set) |
| G2-11 | accepted in part | K2a | C5 ci K2 | rename → S1.1c-8 | assert that the checked jobs are exactly `["lint", "proofs"]` |
| G2-12 | accepted | K2a | C5 ci K2 | – | `uses` list asserted non-empty in Canary's twin |
| G2-13 | accepted | K2e | C8 hooks K2 | – | move the main guard to the end of the file |
| G2-14 | accepted (reshaped) | K2b | C8 hooks K2 | – | the registry indexes another repository; `assert_nothing_started` |
| G2-15 | accepted | K2e (live data) | C7 proofs K2 | – | the dirty marker value is derived from the world's marker |
| G2-55 | accepted in part | K2a | C2 toolchain K2 | three renames → S1.1c-8; duplicate of test_tree_digest → S1.1c-10 | exact reasons asserted |
| G2-56 | accepted | K2a | C2 toolchain K2 | – | `reasons_of(outcome) == []` as the witness that go ran |
| G2-57 | accepted | – | – | carried in full to S1.1c-8 | rename only |
| G2-58 | accepted | – | – | carried in full to S1.1c-8 | rename only |
| G2-59 | accepted | K2a | C2 toolchain K2 | – | `errno.ELOOP` asserted |
| G2-60 | accepted | K2e (broken fixture) | C2 toolchain K2 | – | `strip="go/"`, `exec_sha256={"bin/go": "00" * 32}` |
| G2-61 | accepted | K2a | C2 toolchain K2 | – | exit 0 asserted first in three tests |
| G2-62 | accepted | – | – | carried in full: split → S1.1c-10, names → S1.1c-8 | – |
| G2-63 | accepted | K2a | C2 toolchain K2 | – | the exact missing-file fault reason is the witness |
| G2-64 | accepted | K2a | C2 toolchain K2 | – | the exact usage line asserted |

### 3. Findings

#### G10-5: the re-index hook's check-then-start (gate/hooks/gitnexus_reindex.py)

**1. Verdict: accepted in part.**

The race is real:
- `main` reads the pidfile in `running` at :122, then calls `start` at :128, with no lock between them.
- `start` writes the pidfile only after `Popen` (:99-102).
- The hook is registered for UserPromptSubmit and for Bash PostToolUse (.claude/settings.json:27, :39, grep). Two calls at once can both see "not running" and both start `analyze --force`.

GitNexus:
- `impact(running)`: d=1 `main` only, LOW, exact.
- `impact(start)`: d=1 `main` only, LOW, lower-bound with 5 dropped call sites. A grep finds no other caller of `gitnexus_reindex.start`; the dropped sites are other `start` names.
- Both have 12 test callers at d=3, through `World.run`.

**Rejected sub-claim, "EPERM means alive".** The follow-up runs as the hook's own user, from `Popen` at :90. So `os.kill(pid, 0)` raising EPERM proves the recorded pid now belongs to another user, and our follow-up is gone. Reading that as "not running" (:79) is correct. Reading it as alive would block re-indexing behind an unrelated process.

**Pid-reuse sub-claim (a reused pid reads as running): real, carried to S1.1c-10.**
- The clean fix retires the pidfile for a lock the follow-up inherits (`Popen(pass_fds=…)`), released by the kernel when it dies.
- That changes the detection that four existing tests rely on (`test_a_stale_pidfile_does_not_block_a_new_run`, `test_a_run_already_going_is_not_doubled`, the pidfile asserts in `assert_nothing_started`, and :134). It is a design change, not a simple fix.
- Outcome: **fixed in part**.

**2. Kind: K1c** (a logic race; plan §7.1 names G10-5). R5 does not apply: this is a Python Claude Code hook, not engine code.

**3. The change.** One production file, plus the test file.

Production, gate/hooks/gitnexus_reindex.py:
- Add `import fcntl` in the import block, sorted after `import json`.
- After `PIDFILE` (:28), add:
  `LOCKFILE = "auto-reindex.lock"  # held from the pidfile's check to the follow-up's start: two calls start one run`
- Docstring, :15. Replace `A pidfile in the index's storage keeps a second run from starting while one is going.` with:
  `A lock in the index's storage, held from the check to the start, keeps two calls at once from both starting a run, and a pidfile keeps a second run from starting while one is going.`
- `main`, from :121 to the end, becomes:

```python
    storage = entry.get("storagePath")
    if not isinstance(storage, str) or not os.path.isdir(storage):
        return 0
    gitnexus = shutil.which("gitnexus", path=environ.get("PATH"))
    if gitnexus is None:
        return 0
    try:
        with open(os.path.join(storage, LOCKFILE), "a", encoding="utf-8") as lock:
            # flock, not lockf: a POSIX record lock is the process's, so two calls in one process would share it.
            # Held by a call between its check and its start, it raises BlockingIOError: that call starts the run.
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            if not running(os.path.join(storage, PIDFILE)):
                start(gitnexus, os.path.realpath(checkout), storage, environ)
    except OSError:
        return 0
    return 0
```

- **`flock` is load-bearing.** It locks the open file description, so two `open()` calls conflict even inside one process; that is what lets the hold test below run both calls in one process. `lockf` would let both threads "acquire".
- Moving `running` after `which` changes no outcome: both paths return 0.
- No lock file is created on an early return.
- The existing tests stay green: `already_going`, because the pidfile is still read under the lock; `stale_pidfile`; and `behind_head`, whose pidfile is still written.

Test: a new class at the end of test_gitnexus_reindex.py, placed before the guard that C8 moves to the end. It needs `import queue`, `import threading` and `from unittest import mock`.

```python
class Overlap(unittest.TestCase):
    """Two hook calls at once (a prompt and a Bash call's PostToolUse) start one run between them."""

    def test_a_call_arriving_between_another_calls_check_and_its_start_starts_nothing(self):
        world = World(self, indexed="0" * 40)
        parked: queue.SimpleQueue[str] = queue.SimpleQueue()
        release: queue.SimpleQueue[str] = queue.SimpleQueue()
        starts: list[str] = []

        def holding_start(gitnexus: str, checkout: str, storage: str, environ) -> None:
            starts.append(checkout)
            if len(starts) == 1:  # the first call is parked here: after its check, before anything is started
                parked.put("parked")
                wait.get(release, "the test to release the first call's start", cap_s=30)

        self.enterContext(mock.patch.object(gitnexus_reindex, "start", holding_start))
        text = json.dumps(world.prompt())
        first = threading.Thread(
            target=gitnexus_reindex.main, args=(text, world.environ), name="first-hook-call", daemon=True
        )
        first.start()
        try:
            wait.get(parked, "the first call to reach its start", cap_s=30)
            self.assertEqual(gitnexus_reindex.main(text, world.environ), 0)
        finally:
            release.put("go")
        wait.until("the first call to return after its release", lambda: not first.is_alive(), cap_s=30)
        self.assertEqual(starts, [world.repo])
```

- `world.repo` is already a realpath (World :46-47), as `main` passes it.
- No process is spawned, because `start` is replaced. So there is no ResourceWarning, and `World.run`'s warnings filter, which is not thread-safe, is not used.

**4. Proof (K1c, plan §7.1).**
- **Red through the hold.**
  - The plant is the fix's production diff reversed, without the docstring hunk: `git diff -R <C9>^ <C9> -- gate/hooks/gitnexus_reindex.py`. Its three hunks are the import, `LOCKFILE`, and `main`'s tail back to :121-131 as at d521725. No new declaration is needed by the test.
  - Command: `mutate.py --py gate.hooks.tests.test_gitnexus_reindex.Overlap.test_a_call_arriving_between_another_calls_check_and_its_start_starts_nothing --patch <plant>` → **red**.
  - Failing line: `self.assertEqual(starts, [world.repo])`, with "Lists differ: ['…/repo', '…/repo'] != ['…/repo']". The second call's `running()` finds no pidfile, because the parked call has written none.
  - The hold makes the red deterministic, so the ledger's "k/N" is 1/1 unless `--py` supports `--repeat` (coordinator's call).
- **Green under load.** `python3.15 -I -B -X utf8 gate/runner/stress.py --runs 50 gate.hooks.tests.test_gitnexus_reindex.Overlap` → 50/50.
- **Whole module green** at C9.

**5. Commit:** C9 (gate/hooks, K1c), after C8.

#### G10-14: treehash with no argument (gate/toolchain/treehash.py:27-28)

**1. Verdict: accepted.** `_lines` reads `argv[1]` at :28; with no argument that is an IndexError, outside main's `except OSError` (:42), so the result is a traceback and exit 1. GitNexus `context(_lines)`: one caller, `main`. The only starter is parity's `TREE_CHILD` (parity.py:35, grep), which always passes arguments.

**2. Kind: K1.** A Python gate tool, so R5 does not apply.

**3. The change.**
- treehash.py, after `DETAILS` (:24):

```python
USAGE = 2  # no argument: the usage line on stderr, never a traceback
USAGE_LINE = "treehash: usage: treehash.py <root> | treehash.py --details <root> [<root> ...]"
```

- `main`'s first lines, before `try:`:

```python
    if len(argv) < 2:
        print(USAGE_LINE, file=sys.stderr, flush=True)
        return USAGE
```

- Module docstring, after "An OSError prints its text on stderr and exits 3 in either form.", add: `No argument prints the usage line on stderr and exits 2.`
- Test, in test_treehash.py `TreeChild`:

```python
    def test_no_argument_prints_the_usage_line_and_exits_2(self):
        got = self.child()
        self.assertEqual((got.code, got.lines), (2, []), got.stderr)
        self.assertEqual(
            got.stderr, "treehash: usage: treehash.py <root> | treehash.py --details <root> [<root> ...]\n"
        )
```

**4. Proof (K1).**
- Plant: file `gate/toolchain/treehash.py`.
  - find `    if len(argv) < 2:\n        print(USAGE_LINE, file=sys.stderr, flush=True)\n        return USAGE\n`
  - replace `` (empty)
  - The constants are kept; the test does not use them.
- `mutate.py --py gate.toolchain.tests.test_treehash.TreeChild.test_no_argument_prints_the_usage_line_and_exits_2 --file … --find … --replace ""` → **red**.
- Failing line: the `(got.code, got.lines)` assertion, `(1, []) != (2, [])`. A subprocess exit check, so it reads red, not invalid (plan §2).
- Parity is unaffected: any exit other than 0 and 3 already reads as "tree hash failed (exit N)" (parity.py:640-642).

**5. Commit:** C1.

#### G2-5: the accept-token test (gate/baseline/tests/test_breakit_s3_3_2.py:43-49, :93)

**1. Verdict: accepted in part.**
- `if token in printed:` at :93 is never true. With `rev=self.rev` the stored rev equals SPEC's, and regen.py:320 prints the token only when `spec_rev > old[v.file].spec_rev`. So the test asserts only `code == 1`.
- The method borrowing at :43-49 is real, but moving the cases into `tr.Main` is a move/merge, carried to S1.1c-10 (Q4).

**2. Kind: K2a.**

**3. The change**, at :90-95:

```python
        token = f"{F10}:{R10}:1"
        code, printed = self.main()
        self.assertEqual(code, 1, printed)
        self.assertIn(f"value-change {F10} rule {R10} block 1 key {K1[:12]}: REFUSED", printed)  # the witness
        self.assertNotIn(token, printed)
```

This pins designed behaviour, not a choice: §8.3 ("the rev must be above the header's") and regen.py:152 and :320, which carries the comment "it can record (Q-S4)".

**4. Proof.**

```json
{"id":"G2-5","old_rev":"<lane base>","new_rev":"<this commit>","patch":"--- a/gate/baseline/regen.py\n+++ b/gate/baseline/regen.py\n@@ -151,3 +151,3 @@\n         return \"drop-change\", False\n-    if spec_rev > stored_rev and _accepted(new, accept):\n+    if _accepted(new, accept):\n         return \"spec-rev\", True\n@@ -319,3 +319,3 @@\n             print(f\"{v.cls} {v.file} rule {v.rule} block {v.ordinal} key {v.key[:12]}: {verdict}\")\n-            if v.cls == \"value-change\" and v.file in old and spec_rev > old[v.file].spec_rev:  # it can record (Q-S4)\n+            if v.cls == \"value-change\" and v.file in old:  # it can record (Q-S4)\n                 print(f\"  to accept it as a SPEC revision's change: ACCEPT {v.file}:{v.rule}:{v.ordinal}\")\n","py":"gate.baseline.tests.test_breakit_s3_3_2.BreakitMain.test_the_printed_accept_token_is_offered_only_when_it_can_record","expect":{"old":"green","new":"red"}}
{"id":"G2-5-control","control":true,"old_rev":"<lane base>","new_rev":"<this commit>","py":"gate.baseline.tests.test_breakit_s3_3_2.BreakitMain.test_the_printed_accept_token_is_offered_only_when_it_can_record","expect":{"old":"green","new":"green"}}
```

- Old is green under the plant: the token is printed, `--accept` turns it into a spec-rev, and the record exits 0. I checked that no other `spec_rev` or accept guard exists (grep of regen.py).
- New is red at `assertNotIn`.

**5. Commit:** C6.

#### G2-8: the tautological digest-mismatch test (gate/canary/tests/test_candidate.py:329-334)

**1. Verdict: accepted.**
- The fake seam raises `CanaryError` carrying the expected text, and `_platform_entry` catches only `DigestMismatch` and `OSError` (candidate.py:241-244). So the test reads back its own injected message.
- GitNexus `context(_platform_entry)`: called by `_upgraded`.
- test_f1_feeds.py:66-68 (`test_digest_mismatch_from_the_real_seam_is_named`) raises a real `DigestMismatch` and asserts the built text. That is exactly the survey's option (b), so a rewrite here would duplicate it.

**2. Kind: K2-del.**

**3. The change:** delete `Refusals.test_digest_mismatch_through_archive_is_named` (:329-334). Check whether `RUFF_LINUX` still has users; it does (`grep -n RUFF_LINUX`), so it stays.

**4. Proof:** one plant, red on the removed test at old and on the remaining one at new.

```json
{"id":"G2-8","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/canary/candidate.py","find":"    except DigestMismatch as e:\n        raise CanaryError(f\"{name} {platform}: the download's sha256 is not the published one\") from e\n","replace":"    except CanaryError as e:\n        raise CanaryError(f\"{name} {platform}: the download's sha256 does not match\") from e\n","old_py":"gate.canary.tests.test_candidate.Refusals.test_digest_mismatch_through_archive_is_named","new_py":"gate.canary.tests.test_f1_feeds.BuildNamesTheSeamsErrors.test_digest_mismatch_from_the_real_seam_is_named","expect":{"old":"red","new":"red"}}
{"id":"G2-8-control","control":true,"old_rev":"<lane base>","new_rev":"<this commit>","old_py":"gate.canary.tests.test_candidate.Refusals.test_digest_mismatch_through_archive_is_named","new_py":"gate.canary.tests.test_f1_feeds.BuildNamesTheSeamsErrors.test_digest_mismatch_from_the_real_seam_is_named","expect":{"old":"green","new":"green"}}
```

**5. Commit:** C4.

#### G2-9: the moved-tool test (test_candidate.py:446-454)

**1. Verdict: accepted.** `startswith("canary: go")` also matches `canary: golangci-lint…`, and "moved" is never checked. In the fixture's order (gate/canary/testdata/pinned-tools.json: python, python-floor, go, golangci-lint, ruff, lark), go is position 3 and golangci-lint position 4. candidate.py:383-386 emits one fault for each.

**2. Kind: K2a.**

**3. The change:** replace :451-454 with:

```python
        faults = self.faults()
        self.assertEqual(
            faults,
            ["canary: go moved: position 3 \u2192 4", "canary: golangci-lint moved: position 4 \u2192 3"],
        )
```

U+2192 is written as `\u2192`. Python's `str` equals the literal "→" that candidate.py:386 emits.

**4. Proof.**

```json
{"id":"G2-9","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/canary/candidate.py","find":"f\"canary: {name} moved: position","replace":"f\"canary: {name} reordered: position","py":"gate.canary.tests.test_candidate.Structural.test_tool_moved_is_named","expect":{"old":"green","new":"red"}}
{"id":"G2-9-control","control":true,"old_rev":"<lane base>","new_rev":"<this commit>","py":"gate.canary.tests.test_candidate.Structural.test_tool_moved_is_named","expect":{"old":"green","new":"green"}}
```

The find text occurs once (candidate.py:386, grep).

**5. Commit:** C3.

#### G2-10: the docstring-wording test (gate/canary/tests/test_fix_s11c4.py:170-176)

**1. Verdict: accepted.** `CandidateDoc.test_the_module_doc_says_what_touches_the_disk` asserts on `candidate.__doc__` wording only, and runs no production statement.

**2. Kind: K2-del.** No plant can be red on a remaining assertion, because no behaviour sits behind the wording. So the proof is the covered set.

**3. The change:** delete the class `CandidateDoc` (:170-176). If `candidate` is then unused in the module, drop its import (`ruff` F401 tells).

**4. Proof: the package's covered-statement set is unchanged** (plan §7.2, stdlib `trace`).
- Collect `{(file, line) for (file, line), n in trace.Trace(count=1, trace=0).results().counts.items() if file under gate/canary/ and not under gate/canary/tests/}` while running `unittest.main(module=None, argv=["x", "discover", "-s", "gate/canary/tests", "-t", "."], exit=False)` through `Trace.runfunc`.
- Take it at `<lane base>` and at `<this commit>`, two runs each. All four sets must be equal.
- The set includes G2-8's deletion, which lands in the same commit. G2-8's lines are also covered by test_f1_feeds and the other refusal tests.

**5. Commit:** C4.

#### G2-11: the go.sum restore test (gate/ci/tests/test_generate.py:189-203)

**1. Verdict: accepted in part.**
- The name contradicts the code: `go mod download all` is the form that writes go.sum (generate.py:69-74).
- The `continue` at :198 passes with nothing checked if no job downloads.
- Rendered in memory: the downloading jobs are `lint` and `proofs`, each with one restore. `test` and `strict-gate` do not download.

**2. Kind: K2a.** The rename is carried to S1.1c-8 (see §7).

**3. The change:** in :194-203:

```python
        text = generate.render([*ROWS, row("deps", "lint", ("T4",))])
        checked = []
        for job, body in jobs_of(text).items():
            with self.subTest(job):
                if "go mod download" not in body:
                    continue
                checked.append(job)
                ...  # the four existing assertions, unchanged
        self.assertEqual(checked, ["lint", "proofs"])  # the witness: the jobs that download were checked
```

**4. Proof.**

```json
{"id":"G2-11","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/ci/generate.py","find":"        *(MODULE_STEPS if modules else []),\n","replace":"        *([] if modules else []),\n","py":"gate.ci.tests.test_generate.Render.test_the_download_step_is_not_the_form_that_writes_go_sum","expect":{"old":"green","new":"red"}}
{"id":"G2-11-control","control":true,"old_rev":"<lane base>","new_rev":"<this commit>","py":"gate.ci.tests.test_generate.Render.test_the_download_step_is_not_the_form_that_writes_go_sum","expect":{"old":"green","new":"green"}}
```

The class name is to be confirmed at build: the test sits in the class that holds :183-203; use its real name.

**5. Commit:** C5.

#### G2-12: the canary's full-sha pin test (test_generate.py:270-272)

**1. Verdict: accepted.** It passes vacuously with no `uses:` lines. Its Render twin (:47-49) guards against that with `assertTrue(uses)`.

**2. Kind: K2a.**

**3. The change:**

```python
    def test_every_action_is_pinned_by_a_full_sha(self):
        uses = re.findall(r"uses: (\S+)", self.text)
        self.assertTrue(uses)
        for ref in uses:
            self.assertRegex(ref, r"^actions/(checkout|cache/restore|upload-artifact|download-artifact)@[0-9a-f]{40}$")
```

**4. Proof.**

```json
{"id":"G2-12","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/ci/generate.py","find":"    return f\"uses: {repo}{sub}@{sha}  # {version}\"","replace":"    return f\"uses:{repo}{sub}@{sha}  # {version}\"","py":"gate.ci.tests.test_generate.Canary.test_every_action_is_pinned_by_a_full_sha","expect":{"old":"green","new":"red"}}
{"id":"G2-12-control","control":true,"old_rev":"<lane base>","new_rev":"<this commit>","py":"gate.ci.tests.test_generate.Canary.test_every_action_is_pinned_by_a_full_sha","expect":{"old":"green","new":"green"}}
```

`targets` validates nothing (generate.py:316-325), so setUp survives the plant.

**5. Commit:** C5.

#### G2-13: the main guard mid-file (gate/hooks/tests/test_gitnexus_reindex.py:213-214)

**1. Verdict: accepted.** `Servers`, `Restart` and `FollowUp` (:233-304) are defined after the guard. A direct run therefore runs 15 tests (WhenToLook 4, Reindex 3, LeaveAlone 8) out of the module's 20.

**2. Kind: K2e** (main guard).

**3. The change:** move `if __name__ == "__main__":\n    unittest.main()` from :213-214 to the end of the file.

**4. Proof: the K2e main-guard check, run from the repo root without `-I`** (`-I` drops the cwd from `sys.path`).
- `python3.15 -B -m gate.hooks.tests.test_gitnexus_reindex` ("Ran N tests") against `python3.15 -B -m unittest gate.hooks.tests.test_gitnexus_reindex`.
- Old: 15 against 20, so red.
- New: 20 = 20 at C8, and 21 = 21 after C9.

**5. Commit:** C8.

#### G2-14: the unindexed-repository test (test_gitnexus_reindex.py:183-186)

**1. Verdict: accepted, reshaped.**
- With `indexed=None` the registry is `[]`, so `registry_entry`'s loop body (:68) never runs. With no row there is no storage, so no plausible defect can start a run. `assert_nothing_started` alone could not be shown red on any plant.
- The name promises "a repository GitNexus does not index", that is, a registry that indexes other repositories. The path-mismatch branch is the real target.

**2. Kind: K2b.**

**3. The change:**

```python
    def test_a_repository_gitnexus_does_not_index_is_left_alone(self):
        world = World(self, indexed=None)
        other = os.path.join(world.root, "other")
        os.mkdir(other)
        with open(os.path.join(world.home, "registry.json"), "w") as f:  # another repository's row, its storage ours
            json.dump([{"name": "other", "path": other, "storagePath": world.storage, "lastCommit": "0" * 40}], f)
        # A defect that started a run here would leave a follow-up writing into the world: wait for it before removal.
        self.addCleanup(lambda: os.path.exists(world.pidfile()) and world.started())
        self.assertEqual(world.run(world.prompt()), 0)
        self.assert_nothing_started(world)
```

The cleanup runs before the world's `tmp.cleanup` (LIFO). It does nothing on real code.

**4. Proof.**

```json
{"id":"G2-14","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/hooks/gitnexus_reindex.py","find":" and os.path.realpath(row[\"path\"]) == target:","replace":":","py":"gate.hooks.tests.test_gitnexus_reindex.LeaveAlone.test_a_repository_gitnexus_does_not_index_is_left_alone","expect":{"old":"green","new":"red"}}
{"id":"G2-14-control","control":true,"old_rev":"<lane base>","new_rev":"<this commit>","py":"gate.hooks.tests.test_gitnexus_reindex.LeaveAlone.test_a_repository_gitnexus_does_not_index_is_left_alone","expect":{"old":"green","new":"green"}}
```

- Under the plant, new is red at `assert_nothing_started`: the pidfile exists, written synchronously by `start` at :99-102.
- **Coverage, the K2b part.** The promised branch's first body statement is gitnexus_reindex.py:68, the `if` inside `for row in rows`. Its stdlib `trace` count, running this id alone, must go **0 → 1**.
- If C9 lands first, :68 is unchanged, so the line number stands.

**5. Commit:** C8.

#### G2-15: the hard-coded marker (gate/proofs/drivers/test_lawdelta_planted.py:189)

**1. Verdict: accepted.**
- :189 writes `"S4.1\n"`. Once the marker is S4.1 there is no dirty edit, and `block[2]` is the blank line that the summary prints after the LAW block (summary.py:58-60). So `assertEqual("", "LAW Δ: 1 law files…")` at :196 fails.
- The sibling at :157-158 already derives its value.

**2. Kind: K2e** (live-data pin; the live file is `gate/current-sprint`).

**3. The change:** replace :189 with:

```python
        old = support.read_text(os.path.join(self.world.repo, MARKER)).strip()
        self.world.write(MARKER, ("S4.2" if old == "S4.1" else "S4.1") + "\n")
```

**4. Proof: the live-data check** (the scratch copy's live file changed).

```json
{"id":"G2-15","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/current-sprint","find":"S1.1c-7\n","replace":"S4.1\n","py":"gate.proofs.drivers.test_lawdelta_planted.LawDeltaPlanted.test_a_dirty_law_file_is_named_on_a_last_line_and_the_row_passes","expect":{"old":"red","new":"green"}}
{"id":"G2-15-control","control":true,"old_rev":"<lane base>","new_rev":"<this commit>","py":"gate.proofs.drivers.test_lawdelta_planted.LawDeltaPlanted.test_a_dirty_law_file_is_named_on_a_last_line_and_the_row_passes","expect":{"old":"green","new":"green"}}
```

- Old is red at :196.
- An unknown committed marker is tolerated: the sibling at :151-163 commits S4.1 and passes.
- law.json's `cases: 8` for this driver is unchanged, because no test is added.

**5. Commit:** C7.

#### G2-55: the three break-it tests with weak assertions (gate/toolchain/tests/test_breakit_s11c3.py:57, :62, :69)

**1. Verdict: accepted in part.**
- Each test asserts only that the reason does not contain "tree differs".
- GitNexus `context(_tree_failure)`: callers are `_check` and this file's `reason`. The exact reasons are at parity.py:637-642.
- The renames are carried to S1.1c-8.
- test_tree_digest.py:244-259 already pins the same three branches exactly, so the duplication is carried to S1.1c-10.

**2. Kind: K2a.**

**3. The change.** Each test's two assertions are replaced by one exact assertion:
- :59-60 → `self.assertEqual(got, "go: tree hash could not start: [Errno 24] Too many open files")`. EMFILE is 24 on macOS and Linux; `TreeStartFailed(str(e))` is at parity.py:242.
- :64-67 → `self.assertEqual(got, "go: tree hash failed (exit 1): Traceback (most recent call last):")`. The first stderr line, per parity.py:256-257. The child env is `{"PATH": os.defpath}`, so there is no colour and no dev mode.
- :73-76 → `self.assertEqual(got, "go: tree hash failed (exit -9)")`

**4. Proof:** two plants, three entries plus three controls.

```json
{"id":"G2-55-start","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/parity.py","find":"tree hash could not start: {e}","replace":"tree hash failed: {e}","py":"gate.toolchain.tests.test_breakit_s11c3.TreeChildFailureIsNotADamagedTool.test_a_child_that_cannot_start","expect":{"old":"green","new":"red"}}
{"id":"G2-55-crash","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/parity.py","find":"tree hash failed (exit {e.code}){line}","replace":"tree hash failed{line}","py":"gate.toolchain.tests.test_breakit_s11c3.TreeChildFailureIsNotADamagedTool.test_a_child_that_crashes","expect":{"old":"green","new":"red"}}
{"id":"G2-55-signal","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/parity.py","find":"tree hash failed (exit {e.code}){line}","replace":"tree hash failed{line}","py":"gate.toolchain.tests.test_breakit_s11c3.TreeChildFailureIsNotADamagedTool.test_a_child_killed_by_a_signal","expect":{"old":"green","new":"red"}}
```

The three controls have the same ids with `"control":true` and expect green/green.

**5. Commit:** C2.

#### G2-56: the telemetry test discards the outcome (gate/toolchain/tests/test_parity.py:663-671)

**1. Verdict: accepted.** The `parity.check` outcome is discarded. A probe refused before go runs would write no telemetry, and the test would pass. `check` runs path then version (parity.py:414).

**2. Kind: K2a.**

**3. The change:** replace the `parity.check(...)  # fmt: skip` statement with:

```python
        outcome = parity.check(
            go,
            self.real_cache,
            self.env,
            full_tree=False,
            spawn=parity.popen,
            scratch=self.scratch,
            run_id=RUN_ID,
            handoff_in=None,
        )
        # The witness that go ran: a probe refused before go (path, version) would leave no telemetry either.
        self.assertEqual(reasons_of(outcome), [])
```

**4. Proof.**

```json
{"id":"G2-56","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/parity.py","find":"_version_failure(tool, home, want, env, probes, timing)","replace":"f\"{tool.name}: version: not probed\"","py":"gate.toolchain.tests.test_parity.RealCache.test_a_probe_of_the_real_go_leaves_no_telemetry_in_the_world","expect":{"old":"green","new":"red"}}
```

- Plus its control (green/green).
- The find text occurs once (:414).
- The test needs the installed pinned cache, as its siblings do.

**5. Commit:** C2.

#### G2-57 and G2-58: renames only

- **G2-57 (test_parity_handoff.py:284): carried in full to S1.1c-8.**
  - The test asserts that the hand-off lists `realpath(second)` (:304) and that the link is back (:305); B then hashes for real (:307-308).
  - Proposed name: `test_the_hand_off_lists_the_tree_the_child_hashed_not_the_restored_link`.
  - "lists the tree the child hashed" ← :304. "not the restored link" ← :305-308.
- **G2-58 (test_tree_digest.py:261): carried in full to S1.1c-8.**
  - The body pins four required keywords for both `check` and `_check` (:266-275).
  - Proposed name: `test_check_requires_spawn_scratch_run_id_and_handoff_in_by_keyword`.
  - "requires … by keyword" ← the TypeError for each missing key. The `given` dict's four keys ← the names.

#### G2-59: the symlinked executable (gate/toolchain/tests/test_archive_pins.py:106-108)

**1. Verdict: accepted.** `assertRaises(OSError)` also passes on ENOENT. `sha256_file` opens with `O_NOFOLLOW` (manifest.py:272), which gives ELOOP on a link on both macOS and Linux.

**2. Kind: K2a.**

**3. The change** (add `import errno`):

```python
    def test_listed_executable_that_is_a_symlink_is_refused(self) -> None:
        with self.assertRaises(OSError) as caught:
            install.archive_pins(self.archive, "demo/", ["bin/alias"])
        self.assertEqual(caught.exception.errno, errno.ELOOP)
```

**4. Proof:** the extraction silently drops the link.

```json
{"id":"G2-59","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/install.py","find":"            os.symlink(member.linkname, target)\n","replace":"            pass\n","py":"gate.toolchain.tests.test_archive_pins.<class>.test_listed_executable_that_is_a_symlink_is_refused","expect":{"old":"green","new":"red"}}
```

- Plus its control.
- `<class>` is the class that holds :106 (fill it at build).

**5. Commit:** C2.

#### G2-60: the fixture `manifest.load` would refuse (test_breakit_s11c3.py:16-29)

**1. Verdict: accepted.** `strip="go"` fails `_strip_prefix` (manifest.py:136-137), and `exec_sha256={}` fails `_exec_map` (:152-153).

**2. Kind: K2e** (broken fixture). It changes no outcome: `_tree_failure` reads only `name` and `tree_sha256` (parity.py:630-652).

**3. The change:** `strip="go/"` and `exec_sha256={"bin/go": "00" * 32}`.

**4. Proof: the loader refuses the old fixture and accepts the new.** At each revision, from the repo root:

```
python3.15 -I -B -c "import sys; sys.path.insert(0, '.'); from gate.toolchain import manifest; from gate.toolchain.tests.test_breakit_s11c3 import a_tool; t = a_tool(); manifest._strip_prefix(t.strip, 'a_tool'); manifest._exec_map(dict(t.exec_sha256), 'a_tool'); print('accepted')"
```

- Old: a ManifestError traceback ("field 'strip' must end with '/'"), exit 1, so red.
- New: `accepted`, exit 0.

**5. Commit:** C2.

#### G2-61: three tests ignore the install's exit code (gate/toolchain/tests/test_install.py:94, :100, :105)

**1. Verdict: accepted.** Each test discards `run_main`'s result.

**2. Kind: K2a.**

**3. The change:** in each of the three tests, replace `self.run_main("install")` with:

```python
        code, lines = self.run_main("install")
        self.assertEqual(code, 0, lines)
```

**4. Proof:** the install does its work but reports failure.

```json
{"id":"G2-61","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/install.py","find":"        return install(\n            os.path.join(repo, \"tools.json\"), cache, manifest.platform_key(), out, fetch=fetch, rename=rename\n        )\n","replace":"        install(\n            os.path.join(repo, \"tools.json\"), cache, manifest.platform_key(), out, fetch=fetch, rename=rename\n        )\n        return 2\n","py":"gate.toolchain.tests.test_install.HappyPath.test_the_tool_directory_itself_is_read_only","expect":{"old":"green","new":"red"}}
```

The same entry is repeated for `test_the_cache_holds_only_the_tool_afterwards` and `test_the_installed_tree_hashes_as_written_out_by_hand`, each with its control.

**5. Commit:** C2.

#### G2-62: three scenarios run as subtests (test_parity_handoff.py:187-235, :237, :256, :269)

**Accepted; carried in full.**
- Making the three scenarios separate test methods is a split (Q4), so it goes to S1.1c-10.
- Their proposed names go to S1.1c-8:
  - `test_a_home_with_another_realpath_is_hashed_for_real` (:254: `trees == [homes["go"]]`, taken 1);
  - `test_a_repinned_tool_is_hashed_for_real` (:263-267);
  - `test_a_failed_details_child_hashes_every_candidate` (:280-282).
- **Ledger gap:** §7.5 has "carried in full to S1.1c-8" only. This finding needs "carried in full to S1.1c-10" (coordinator).

#### G2-63: the weak full-tree witness (test_parity_handoff.py:315-322)

**1. Verdict: accepted.** `covered == 0` holds for any failure. At full tree, a missing hand-off gives `f"parity: tree hand-off {handoff_in}: {e}"` (parity.py:359-363), where `e` is `strerror` "No such file or directory" (:446-447).

**2. Kind: K2a.**

**3. The change:** :319 becomes:

```python
        self.assertEqual(
            (outcome.reasons, outcome.covered), ((f"parity: tree hand-off {missing}: No such file or directory",), 0)
        )  # the witness: at full tree, the same path is read and is a fault
```

**4. Proof:** a full-tree failure that never reads the hand-off.

```json
{"id":"G2-63","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/parity.py","find":"    entries: dict[str, Entry] = {}\n    if full_tree and handoff_in is not None:\n","replace":"    entries: dict[str, Entry] = {}\n    if full_tree:\n        return Outcome(reasons=(\"parity: planted failure\",), covered=0, taken=0, taken_from=\"\")\n    if full_tree and handoff_in is not None:\n","py":"gate.toolchain.tests.test_parity_handoff.<class>.test_below_t2_the_hand_off_is_never_read","expect":{"old":"green","new":"red"}}
```

- Plus its control.
- The T1 half is untouched by the plant.

**5. Commit:** C2.

#### G2-64: the usage test checks only the exit code (gate/toolchain/tests/test_vulndb_fetch.py:185-188)

**1. Verdict: accepted.** Exit 2 is returned both for the usage line and for a FetchError (vulndb_fetch.py:199-206).

**2. Kind: K2a.**

**3. The change:**

```python
    def test_an_argument_is_a_usage_error(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = vulndb_fetch.main(["--now"], self.environ)
        self.assertEqual((code, out.getvalue()), (2, "vulndb: usage: fetch-vulndb takes no arguments, got --now\n"))
```

**4. Proof:** a usage error that prints nothing. It never reaches the network: the branch still returns before `fetch`.

```json
{"id":"G2-64","old_rev":"<lane base>","new_rev":"<this commit>","file":"gate/toolchain/vulndb_fetch.py","find":"        print(f\"vulndb: usage: fetch-vulndb takes no arguments, got {' '.join(argv)}\", flush=True)\n        return 2\n","replace":"        return 2\n","py":"gate.toolchain.tests.test_vulndb_fetch.<class>.test_an_argument_is_a_usage_error","expect":{"old":"green","new":"red"}}
```

Plus its control.

**5. Commit:** C2.

### 4. Commit plan (A2: 9 commits; plan line 8, ceiling 10)
1. **C1, gate/toolchain K1:** G10-14. Proof: `--py` revert plant, red.
2. **C2, gate/toolchain K2:** G2-55, -56, -59, -61, -63, -64 (K2a) and G2-60 (K2e). Proofs: `--expect` batch entries plus the loader one-liner.
3. **C3, gate/canary K2:** G2-9. Proof: `--expect`.
4. **C4, gate/canary K2-del:** G2-8 (plant, red/red), G2-10 (covered set, ×2).
5. **C5, gate/ci K2:** G2-11, G2-12. Proof: `--expect`.
6. **C6, gate/baseline K2:** G2-5. Proof: `--expect` with a patch.
7. **C7, gate/proofs K2:** G2-15. Proof: the live-data entry.
8. **C8, gate/hooks K2:** G2-13 (main-guard count), G2-14 (`--expect` plus the trace count 0 → 1).
9. **C9, gate/hooks K1c:** G10-5. Proof: hold red, then `stress.py` 50/50.

After the lane's hand-back: the proofs row alone at T4 (every commit here is under gate/).

### 5. Choices made (each "chosen by the design note")
- **G10-14, user text and exit code.**
  - Input: `treehash.py` with no argument.
  - Candidates: (a) a traceback, exit 1 (today); (b) exit 3; (c) a usage line on stderr, exit 2.
  - Choice: (c). Exit 3 means a damaged tool to parity (parity.py:108, :640). Exit 2 is the repo's usage exit (vulndb_fetch.py:202-203; install's refusals). Stderr, because stdout is parity's data channel.
  - Text: `treehash: usage: treehash.py <root> | treehash.py --details <root> [<root> ...]`.
  - Dry run: `git grep -n "treehash: usage"` finds nothing. The old behaviour has no text and is pinned by no test.
- **G10-5, behaviour.**
  - Input: a hook call while another call sits between its check and its start.
  - Candidates: (a) block until the lock frees; (b) return at once and do nothing; (c) a lock the follow-up inherits, with the pidfile retired.
  - Choice: (b). The hook must never block Claude (docstring :13), and the lock holder starts the run.
  - (c) is better, but it is a design change that rewrites four tests' detection, so it goes to S1.1c-10.
  - No user text: the hook prints nothing.
- **G2-55, K2a or K2-del.** Strengthened as the finding proposes. Deleting the three tests as duplicates of test_tree_digest.py:244-259 would cost a separate K2-del commit, beyond the plan's line. That deletion goes to S1.1c-10 with the duplication work.
- **G2-14, a changed input.** The registry now holds another repository's row instead of being empty. Reason: no plant can be red with an empty registry, and the name promises the path-mismatch branch.
- G2-5 pins existing designed behaviour (§8.3; regen.py:320), so it is not a new choice.

### 6. Couplings and risks
- **Canary pins (gate/canary/pin_tests.json):** its five entries are test_manifest (×2) and test_toolchain_lines (×3) only. No id and no failure text changes here, so no coordinator commit is needed.
- **gate/law.json:**
  - LAW Δ shows the C5 edits (path `gate/ci/`) and C7 (`gate/proofs/drivers/`).
  - test_lawdelta_planted's `cases: 8` is unchanged.
  - `gate/baseline/regen.py` is a law path, but C6 edits only its test.
  - No proof driver's case count changes.
- **Proofs row:** every commit is under gate/, so it runs alone at T4 after the hand-back.
- **R5, R4 and §0 item 4:** none. All of this is Python gate code, with no per-event path, no frozen benchmark and no frame table.
- **Other owners' files:** none needed. `.claude/settings.json` is untouched.
- **For the coordinator to decide:**
  - The ledger needs an outcome "carried in full to S1.1c-10" (G2-62).
  - For K1c G10-5, the hold makes red deterministic, so "k/N" is 1/1 unless `--py` supports `--repeat`.
  - G2-15's plant edits `gate/current-sprint` inside mutate's throwaway worktree. W0 must make sure committing it there runs no gate hook.
  - **The commit count is 9**, against a plan line of 8 and a ceiling of 10. It counts toward the running total of 95.
- **Pre-mortem (the-fool) on the riskiest fixes:**
  - **G10-5.** A `lockf` in the build would fail the hold test on new code, so the note says `flock` explicitly. `BlockingIOError` is an `OSError`, so the outer `except` stays a safe catch-all.
  - **G2-14.** A plant-started follow-up racing the temp directory's removal would make the red run invalid. The cleanup that waits for `world.started()` covers this.
  - **G2-5.** If regen had another accept guard, old would read red under the plant. I checked: none exists.

### 7. Carried items
- **To S1.1c-8 (renames, recorded and not made):**
  - G2-11 → `test_both_go_sums_are_restored_after_the_download_and_before_the_runner`. "restored after the download" ← `assertGreater(restore, rindex(download))`; "before the runner" ← `assertLess(restore, index("bootstrap.py exec"))`.
  - G2-55 → `test_a_child_that_cannot_start_is_not_a_damaged_tool`, `test_a_child_that_crashes_is_not_a_damaged_tool`, `test_a_child_killed_by_a_signal_is_not_a_damaged_tool`. "is not a damaged tool" ← each exact reason, which is not "tree differs".
  - G2-57 and G2-58 in full (§3).
  - G2-62's three names.
- **To S1.1c-10:**
  - G10-5's pid-reuse part: the inherited lock, with the pidfile retired.
  - G2-5's move into `tr.Main`.
  - G2-55's duplication with test_tree_digest.py:244-259.
  - G2-62's split.

**Findings not reached:** none.
---

### Coordinator corrections after the second QA pass (2026-11-20; evidence: review-design-qa2.md, coordinator's AST check)

1. **Unique ids (DQ2-S1).** W0's `--expect` refuses a duplicate id. Write each entry and control with its own id:
   - G2-55: `G2-55-start`, `G2-55-crash`, `G2-55-signal`, and controls `…-control`.
   - G2-61: `G2-61-readonly` (test_the_tool_directory_itself_is_read_only), `G2-61-cache` (test_the_cache_holds_only_the_tool_afterwards), `G2-61-hash` (test_the_installed_tree_hashes_as_written_out_by_hand), each with a `…-control`.
2. **Real class names (DQ2-S1), verified with `ast` at 1f4ebba:**
   - G2-11: `gate.ci.tests.test_generate.Scans.test_the_download_step_is_not_the_form_that_writes_go_sum`, not `Render`.
   - G2-59: `gate.toolchain.tests.test_archive_pins.ArchivePinsTest.test_listed_executable_that_is_a_symlink_is_refused`.
   - G2-63: `gate.toolchain.tests.test_parity_handoff.Reader.test_below_t2_the_hand_off_is_never_read`.
   - G2-64: `gate.toolchain.tests.test_vulndb_fetch.Main.test_an_argument_is_a_usage_error`.
3. **A deterministic deletion proof (DQ2-L3).**
   - G2-8 and G2-10 keep the whole-suite covered-set check.
   - Each deleted test's own traced set must also be ⊆ the remaining suite's set (stdlib `trace` per test id).
     - G2-10's docstring test runs no production line, so its set is empty.
     - G2-8's removed test's lines (candidate.py:240-244) are covered by test_f1_feeds's two seam tests.
4. **Ledger:** G2-62's outcome "carried in full to S1.1c-10" is accepted by check_ledger (coordinator item 1).
