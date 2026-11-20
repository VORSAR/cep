# S1.1c-6 combined build: QA break-it (breaker: qa-expert with the-fool)

> **Historical — S1.1c-6, the strict lint set: the size limits in runtime, validate, event, parser, plan and playback; merged in PR #49.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-6-plan-check.md) · [design](../sprints/s1-1c-6-design.md) · [record](../sprints/s1-1c-6-record.md) · reviews: [spike](s1-1c-6-spike.md), [plan check architecture](s1-1c-6-plan-check-architecture.md), [plan check qa](s1-1c-6-plan-check-qa.md), [design architecture](s1-1c-6-design-architecture.md), [design qa](s1-1c-6-design-qa.md), [event walkers](s1-1c-6-event-walkers-review.md), [decoder value](s1-1c-6-decoder-value-review.md), [playback](s1-1c-6-playback-review.md), [combined architecture](s1-1c-6-combined-architecture.md), [breakit qa](s1-1c-6-breakit-qa.md). **← [S1.1c-5](../sprints/s1-1c-5-record.md) · [S1.1c-7 (next)](../sprints/sprint-plan.md#status) →**

**Base:** 0f533a0 (`s1-1c-6/integration`), compared against `main`'s merge-base with it, 4d2b9eb.
**Branch:** `breakit/s1-1c-6` in the detached worktree
the breaker's scratch worktree, not pushed.
`main` was built side by side in a second scratch worktree (4d2b9eb).
**Red-test commits:**
- 1132906 (BK-L1)
- 816b3e9 (BK-L2)
- a6d8bc4 (BK-S1)
- 2f383e6 (BK-L2's driver installs the default SIGINT handler, so the test also holds when started with SIGINT ignored)

Each commit went through the repo's T0 hook (GATE PASSED).

**Test runs:**
- `python3.15 -B -m unittest gate.bench.tests.test_gorun gate.bench.tests.test_load gate.bench.tests.test_gorun_breakit gate.bench.tests.test_load_breakit`:
  19 tests; the only failures are the 3 new break-it tests (BK-L1 and BK-L2 errors, BK-S1 failure).
- `go test -count=1 ./...` on the base: exit 0, with 21 packages ok.

**Result:** 0 blockers, 1 should-fix, 2 later. Every engine package agrees with `main` on every input I generated. The only
differences are the ones a decision made on purpose. All three findings are in the gate's new process and measurement
tools (`gate/bench`).

Method:
- I read design §2 and §4-§9, decisions.md's S1.1c-6 sections and the two gate fixes adversarially.
- I wrote one differential harness, `internal/zzdiff` (package main, scratch, copied into both trees and never committed).
  It prints a deterministic deep rendering of each public result (`reflect`, unexported fields included, maps sorted,
  pointers followed, floats as bits). I wrote it once, built it in both trees and compared the outputs with
  `cmp`/`diff`.
- I also added scratch root-package and parser tests, copied into both trees and run there. They live only in
  the breaker's scratch harness folder.
- The runs, all on Go 1.27.4 and darwin/arm64, are listed under "Tried and held".
- **Not used:** GitNexus and Hindsight. I had no resource reader or Hindsight tool in this session, so I used running
  code instead.
- **Not tried:**
  - performance or rule 13a measurements (the coordinator's and the architecture reviewer's lane);
  - `GOOS=linux` beyond `go vet` (clean);
  - `TickEvery` live ticks, which are wall-clock driven and cannot be compared deterministically;
  - the coverage and mutation tables.

## Should-fix

### BK-S1 — load.py's red verdict names neither the test running when the test binary dies nor the cause (misleading stress evidence)

- **Where:** `gate/bench/load.py` `load()` (base 0f533a0, lines 85-107) and `tally()` (line 40).
  - `tally` counts a run only when it ends in a `pass`, `fail` or `skip` action. A test that is running when the binary
    panics (a `-timeout` panic, a goroutine panic, `os.Exit`) gets only `run` for that run, so the run is not counted.
    The test shows k/k from its earlier runs, or is not listed at all if it never finished once.
  - The tail printed on a red run is `result.stderr`'s. Under `go test -json` the panic text is an `output` event on
    stdout, and stderr is empty.
- **Scenario:** a stress run whose test binary times out or crashes.
  - Seen for real: `load.py --wall 1200 -- -race -count=4 -cpu 1,4 . ./internal/playback/ ./internal/event/` with 28
    hogs.
  - Output: 241 tests at 8/8, 59 internal/event tests at 3/3, `TestScannerMatchesTheReference: 2/2`,
    `Example_embed: 1/1`, then `verdict: below N/N: none; go test exit 1` and nothing else.
  - The cause, confirmed by re-running the same command with `-json` captured:
    - The internal/event test binary hit go test's default `-timeout 10m0s` (`panic: test timed out after 10m0s`,
      `running tests: TestScannerMatchesTheReference (2m48s)`).
    - test2json emitted a `fail` action for the package only, never for the test. load.py had printed that same test
      as `TestScannerMatchesTheReference: 2/2`, the counts of the runs it finished before the kill.
  - Why it happened:
    - Under `-race` with no load, internal/event takes 39 s on the base against 19 s on `main`.
      `TestScannerMatchesTheReference` alone takes 29 s; it is lane E's frozen-reference test with 1,000,000 random
      cases.
    - So 8 package runs under 28 hogs pass 10 minutes.
    - That is not an engine defect, but a whole-package stress run of internal/event at `-count` 3 or more under load
      needs an explicit `-timeout`.
- **Evidence:** `gate/bench/tests/test_load_breakit.py::DiedOutsideATest::test_a_binary_that_times_out_names_the_test_or_the_cause`.
  - Setup: a module with TestOK and a TestSlow that sleeps a minute, run with `-count=1 -timeout=3s` and 1 hog.
  - load.py exits 1, as it should, but prints only `TestOK: 1/1 under load (hogs 1)` and
    `verdict: below N/N: none; go test exit 1`.
  - TestSlow, the test that hung, appears nowhere, and no line says "timed out". The test is red today.
- **Honest likelihood:** whenever a test binary dies outside a test's own result.
  - Hangs and deadlocks under load are exactly what design §2.3.4 uses load.py to certify ("50/50 under load").
  - The exit code stays 1, so it is never a false green.
  - But the verdict says "below N/N: none" about a run that had a hung test, and the per-test counts look clean (3/3,
    2/2). A reader can blame the build or the machine, not the test, and re-running without `-json` is the only way to
    find the cause.
- **Fix area:** `gate/bench/load.py`. Red first with the test above.
  - A test with a `run` action and no result counts as a failure in `below`.
  - On a red run the tail comes from the stdout `output` events (the `panic:` and `FAIL` lines), not only from stderr.

## Later

### BK-L1 — gorun skips its group KILL when a stop lands after the leader exits (an orphaned grandchild)

- **Where:** `gate/bench/gorun.py` `run()`, base lines 88-93.
  - The group KILL for "a grandchild left in the group by a leader that exited",
    `_killpg(proc.pid, signal.SIGKILL)`, sits after the `try`/`except BaseException`, not inside it.
  - A first TERM that `term.install()` raises between `proc.wait(...)` returning and `os.killpg` leaves run as
    `Terminated` with the group alive.
  - The module docstring promises both: "a group left behind by a leader that exited is killed too. A TERM or an
    interrupt of the caller kills the child's group before the stop goes on."
- **Scenario:**
  - The leader starts a grandchild in its group and exits 0.
  - The runner's TERM to the driver arrives just after the leader is reaped.
- **Evidence:** `gate/bench/tests/test_gorun_breakit.py::StopAfterTheLeaderExits::test_a_stop_after_the_leader_exits_still_kills_its_group`.
  - The driver wraps `gorun._killpg` so that its first call (the post-wait KILL; no `_stop` ran) sends the driver its
    own TERM first. The handler raises at the next call boundary, before `os.killpg` runs.
  - The driver exits 3 (Terminated came out of run), and the grandchild is still alive after 5 s. Red.
  - Control: the same driver without the self-TERM exits 0 and the grandchild is gone (green).
- **Honest likelihood:** low. It needs a TERM in a window a few bytecodes wide, and a leader that exits while leaving a
  grandchild. `go test`/`go build` wait for their children.
  - It is the same class, an adjacent window, as the start-up window the owner had fixed in this sprint
    ([owner, 2026-11-19, the gate's process runner](../sprints/decisions.md#owner-decisions-2026-11-19--the-gates-process-runner)).
  - The cost is a process left running, parented to init, under a gate run.
- **Fix area:** `gorun.run`. Move the post-wait group KILL into a `finally` that holds the group whatever ends the
  `try` (the shape of `gate/proofs/drivers/support.py`'s `run_process`). Whether this goes into the sprint's fix round
  or the backlog is the owner's call, since the 2026-11-19 decision scoped only the start-up window.

### BK-L2 — a second interrupt during gorun's grace skips the group KILL

- **Where:** `gate/bench/gorun.py` `_stop()`, base lines 50-56, called from run's `except BaseException`.
  - `term.install()` ignores a second TERM, but not a SIGINT.
  - A KeyboardInterrupt raised inside `proc.wait(timeout=grace_s)` leaves `_stop` before `_killpg(..., SIGKILL)`.
- **Scenario:**
  - The coordinator presses Ctrl-C (the driver's handler TERMs the group and starts the grace).
  - The child does not die within the grace, and a second Ctrl-C arrives.
  - The group is never KILLed.
- **Evidence:** `gate/bench/tests/test_gorun_breakit.py::SecondInterruptInTheGrace::test_a_second_interrupt_during_the_grace_still_kills_the_group`.
  - The child notes its TERM in a file and lives on; its grandchild ignores TERM; the grace is 2 s.
  - The test sends the first SIGINT, waits for the child's TERM file (a state, not a sleep), then sends the second.
  - Both processes are alive 5 s after the grace would have ended. Red.
  - Control: the same test with one SIGINT is green (both gone after the grace).
  - Also reproduced by hand with a 5 s grace: `driver exit -2 child alive True grandchild alive True`. With one SIGINT:
    `child alive False grandchild alive False`.
- **Honest likelihood:** low. The children gorun runs (`go test`, `go build`, test binaries, `git`) die on TERM within
  milliseconds, so the grace rarely lasts long enough for a second Ctrl-C to matter. A child that traps or ignores TERM
  is left running.
- **Fix area:** the same as BK-L1. Send the group KILL from a `finally` in `_stop` (or mask SIGINT around the stop), so
  no second signal can cut the cleanup short.

## Tried and held

**Engine, main against the branch.** The base and `main` 4d2b9eb were built from the same harness source. Every output
was compared byte for byte.

- **Parser → validate → plan → expects: 1,141,815 programs.**
  - The inputs were the 163 `.cep` files of the repo and seeded mutants of them, of four kinds:
    - 1-3 random edits: spans deleted; tokens, keywords, escapes, odd bytes and big numbers inserted; lines duplicated,
      deleted, swapped or truncated;
    - one structural edit;
    - 2-4 word swaps inside the program;
    - the originals.
  - Compared per program:
    - `parser.Parse`'s error: type, code, line, column, message, fix and text;
    - `Reassemble() == src`, and the deep model, tokens and trivia;
    - `plan.Build` on the unchecked model;
    - `validate.Check`'s diagnostics in full, and the resolved copy;
    - `plan.Build` on the checked model, and `playback.RunExpects`' results (divergence text, position, outputs and
      counts).
  - Volume: 210,001 programs passed the parser and reached Check. 14,764 built plans and ran their expects, 26,178
    blocks in all, of which 2,068 diverged with identical text. 37 of validate's 45 catalogue codes were reached. No
    output differed and nothing panicked.
- **Param overrides:** 42,014 mutants of the 14 programs with params, each checked with 6 random override sets
  (40,686 overridden Checks, 1,586 built plans, 50,432 E-PARAM-001 findings). Identical.
- **`firstExpected` directly, 300,000 generated notations.** 3,061 differ, each of them `main` panicking where the branch
  returns `""`. That is exactly G1 decision 7; nothing else changed.
- **`parser.ParseLiteral`, 200,000 generated literals:** identical.
- **The decided validate change is real but out of the generator's reach.**
  - By hand, `rule A from X select k insert into X;` gives `main` E-NAME-001, E-GRAPH-001, E-NAME-001. The branch gives
    the two E-NAME-001s
    ([owner, 2026-11-16, lane V](../sprints/decisions.md#owner-decisions-2026-11-16--during-s11c-6s-lane-v-e-graph-001-reached-from-source)).
  - No mutant produced it. So for validate, "no difference" is weaker evidence than the counts suggest; the lane's
    mutation nets carry more weight there.
- **Event decoder: 620,000 random inputs and 168,300 byte-targeted inputs, with 4 schemas each.**
  - The schemas covered required and optional attributes, a time attribute or `time arrival`, and an object-only schema.
  - The inputs exercised:
    - every value kind, with escape-dense strings, `\u`, lone and paired surrogates and invalid UTF-8;
    - RFC 3339 forms and their single-character mutations, numbers at the int64 and float limits, durations;
    - nesting at 9,998-10,001, duplicate and escaped keys, and trailing bytes.
  - The byte-targeted set put each of the 256 bytes into 20 positions. It also put every pair (0x80-0xff, then any
    byte) into three string and key positions, and as the tail of three- and four-byte sequences.
  - Every prefix of 8 long documents was decoded (3,748 cuts). So were 5 large documents from 60 KB to 16 MB (escape-
    dense, 700,000 members, 3 MB of invalid UTF-8, 9,997-deep nesting with and without whitespace).
  - Compared: `Decode`'s values, `Decoded` and the `DecodeError` fields, plus `CleanObject`, `CleanLen` and
    `ExactObject`.
  - Identical, except the panic message on 1,425 + 57 + 45 inputs that are not valid JSON: an escape cut off at the end,
    after whitespace or malformed UTF-8 that sends `cleanObject` down its slow path. There `main` panics with
    `index out of range [n] with length n`, and the branch with `slice bounds out of range [:n+1:n]`.
  - Both panic on exactly the same inputs. The branch's own comment says so ("an escape cut off at the end panics, as it
    did", `internal/event/object.go:22`). Every production caller passes validated JSON first (runtime after `Decode`,
    validate after `json.Valid`).
  - The one panic path I could reach, `plan.Build` on an unchecked model with `o: '{ "a":"\\'`, panics on both trees.
    It is outside `Build`'s contract ("MUST be validate.Check's result"), and Check refuses it with E-EXP-002.
  - Decoding from 8 goroutines at once, reusing a junk-filled `dst`, matched the single-goroutine nil-`dst` results
    under `-race`.
- **Playback.**
  - `AppendValue` on 91 edge values matched byte for byte:
    - NaN, ±Inf, −0, 5e-324, MaxFloat64, 2^53 and 2^53+1;
    - timestamps at 0000-01-01 and 9999-12-31 and ±1 ms around each;
    - int64 limits, control characters, U+2028, invalid UTF-8;
    - non-string objects (int, nil, []byte, map), and every kind up to `TObject+2` with and without `Null`.
  - **Recordings written:** 15,000 recordings were written through `Recorder` against two programs. One had every type
    in two streams (time and arrival) and two tables (one with a two-column key). Each recording mixed `Header`,
    `Event` with and without a payload, `Rejected` (every reason, a nil fault), `Tick`, `BatchEnd`, `Table` put and
    delete (zero rows, wrong-width rows, wrong-kind values, a table out of range) and `Stop`. 10 % of them used a
    failing writer.
  - **Read back:** each recording was then read, along with 8 line- and byte-level mutants of it, by `Reader.Next` to
    its first error (the same error repeated).
  - **Replayed:** in 10,000 of the recordings, each recording and its mutants were also replayed through
    `NewPlanDriver` with every `Step` (90,000 replays).
  - **The reader at its limits:** one recording with every record kind was read at each of its 614 prefixes.
    `MaxLogLine` ±1 was read with and without the final newline.
  - All identical: call errors, bytes written, records, faults, steps, outputs and `Stats`.
- **The runtime's live path through the facade.**
  - Examples 07, 09 and 10 were deployed 150 times each, with 1 and with 4 shards. Each deployment had table rows,
    random rows (nulls, unset attributes, wrong values, malformed UTF-8, timestamps out of range), refused fills, batch
    ends, Stop, Stats, Undeploy and Close.
  - The sink outputs (sorted for 4 shards), the recordings, every error and Stats were identical. The one exception is
    the `QueueHigh` gauge, which also varies between two runs of `main`.
  - The same held with every queue at 1 (`Mailbox`, `Intake`, `SinkQueue`, `RecordQueue`), apart from the
    scheduling-dependent cut point of the recording.

**Concurrency and lifecycle.**

- `load.py --wall 1400 -- -race -count=8 -cpu 1,4 ./internal/runtime/` with 28 hogs: 179 tests, every one 16/16, exit 0.
- **Close racing Deploy:** 300 runs of three Deploy+Start+20×SendRow (one also Stop+Undeploy) against Close, under
  `-race -count=3 -cpu 1,4`. Close was always nil and no goroutine was left. The error classes were the same as `main`'s.
- **One app, everything at once:** Start, 2×Stop, Undeploy, Stats, SendRow and (one run in 3) Close, 400 runs at
  `-race -cpu 4`, twice per tree. There was no hang and no leak, Close was always nil, and both trees produced the same
  24 outcome classes. Counts moved within the run-to-run spread of `main` itself.
- **Contexts already past their deadline, or expiring at once, at each of the six lifecycle calls:** the same outcome
  distribution exactly, 38 lines.
- **A slow recording writer and a Stop deadline of 0-3 ms:** the same three outcome classes with the same counts. The
  recording is always cut on whole lines (0 invalid lines and a final newline in all 60 runs).
- The runtime's Drain, Close and Remove and their helpers, read against `main`: each select, defer and lock pair stayed
  whole. `closeApp` and `awaitMarkers` return what the inline code returned.

**The redesigns.**

- `(*Recorder).Table`: the op written raw is "put" or "delete" only. The width check, the error texts (now with `name`)
  and the stop at the first failed row are as before.
- `AppendValue`'s helpers are word for word the original branches.
- The new lookup tables (`strPlain`, `jsonSpace`, `strByte`, `hexDigit`) were exercised with every byte value in 20
  positions.
- The `Decoder` struct is unchanged, and no `sync.Pool` or `unsafe` was added.

**The gate fixes.**

- `test_gorun` under `stress.py`: 30/30, then 20/20 while `load.py`'s 28 hogs also ran (about 35 hogs on 14 cores).
- `compare.mann_whitney_p` equals a brute-force exact permutation p (midranks, ties, two-sided) on 300 random samples
  with 3-8 values per side, half of them heavily tied.
- A double TERM during the grace is ignored, as `term.install` says; only SIGINT gets through (BK-L2).

**The surface.**

- No exported identifier changed in `.`, `event`, `parser`, `validate`, `plan`, `playback` or `runtime`.
- `model` adds `LitType` and `LitValue` (lane M, G1 Q1). `validate.fits` and `as` were run over 11 literal kinds × 12
  types (132 pairs), and `plan.value` over the same pairs with 3 object texts (396). Both trees gave identical
  results.
- The two modified test files only add tests. The one exception is `internal/validate/units_test.go`'s `TestCycle`:
  its comment was rewritten and the `poisoned` line removed, as lane V's decision changed that behaviour.
- No new file carries a build tag or a GOOS suffix.

## The-fool: what would a hostile reviewer say?

**Steelmanned thesis:** S1.1c-6 split six packages and redesigned five hot functions without changing behaviour. The only
behaviour changes are the decided ones:
- the parser's whitespace-only notation;
- validate's E-GRAPH-004 and E-GRAPH-001;
- the gate's process-runner and test start-up fixes.

**Challenges (Test the evidence, then Find the failure modes):**

1. **"If anything changed, a differential against the real `main` would show it."**
   - I ran 1.14 M programs, 40k overridden checks, 792k decoder inputs, 15,000 recordings with 135k reads, 90k replays
     and 1,800 live deployments. Nothing changed except the decided changes and a panic message on a documented
     precondition violation.
   - **Conceded:** the thesis holds wherever the generator reaches.
2. **"The generator does not reach everything."**
   - The decided validate change was never hit by any mutant, and 8 of the 45 validate codes were never produced. The
     strength of "no difference" is uneven across packages.
   - **Mitigation:** a grammar-aware generator (backlog, if wanted). Meanwhile the lanes' per-split mutation nets are
     the evidence for validate's rarer paths.
3. **"A split moved code across goroutine boundaries."**
   - The runtime survived 16/16 under 28 hogs with `-race`, and its outcome distributions match `main` under races and
     expired deadlines.
   - **Conceded:** holds.
4. **"The gate fixes closed one window and left the next."**
   - Confirmed twice. The post-wait KILL (BK-L1) and a second interrupt (BK-L2) are the same class as the 2026-11-19
     fix.
5. **"The tool that certifies '50/50 under load' can hide a hung test."**
   - Confirmed (BK-S1). It came up for real on this build's own event package.

**Synthesis:** the engine's behaviour-preservation claim stands on the evidence above. The weak spot is the new `gate/bench`
process and measurement layer the sprint's evidence relies on. Fix BK-S1 before the next stress quotation. BK-L1 and
BK-L2 are one small `finally` in `gorun`; the owner decides whether they go into the fix round or the backlog.
**Next pass:** red-team `escapes.py`'s per-function grouping and `mutate.py`'s outcome classification, the remaining
evidence tools, which I did not reach.

## Verdict

0 blockers.

1 should-fix:
- BK-S1: load.py's red verdict hides the test that was running when the binary died, and the cause.

2 later items:
- BK-L1: gorun's post-wait group KILL is skipped by a stop.
- BK-L2: a second interrupt during the grace skips the KILL.

The engine (event, parser, validate, plan, playback, runtime and the facade) agreed with `main` on every generated and
malformed input, apart from the changes the owner decided.
