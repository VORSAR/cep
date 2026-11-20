# S1.1c-7 — whole-code survey (read-only, `main` @ 6602ee3, 2026-11-20)

*The first step of S1.1c-7, asked by the owner on 2026-11-20. It produced findings and changed no code. The owner places each group of findings at the plan check. Every finding is listed in [s1-1c-7-survey-findings.md](s1-1c-7-survey-findings.md).*

## What was asked

The owner asked for every source file to be looked at by parallel refactoring-specialist agents, and for the work to be part of the sprint. The owner's words:

- "look at naming best practices based on the intent of the logic, where its being used, usecase, layer etc. don't put dump names period!"
- "we cannot have a test named after a sprint"
- "use gitnexus for this"

The owner's decisions on the same day:

- **What to report:** everything an agent sees as wrong, with no fixed list. Size limits were excluded, because S1.1c-8 owns them.
- **Agents:** up to six at once, for this read-only survey only (an exception to ground rule 17).
- **Placement:** each group of findings is placed at the plan check.
- **Public names:** every rename of an exported name needs the owner's yes.

## How it ran

- **Coverage:** 650 source files, about 118,000 lines. That is every tracked Go and Python file outside testdata, plus the `.lark` grammar, the Makefile and the git hook launchers.
- **First pass:** 16 batches of 4,000–10,000 lines each. A fresh `refactoring-specialist` agent read each batch under one written brief.
- **Second passes:** where a first-pass agent read files only in part (long table-driven tests, 12 gate row modules), seven second-pass agents read those 99 files line by line. They reported only what the first pass missed. Every file was read in full in the end.
- **Use sites:** every agent was required to check a name's callers, use sites and layer with GitNexus before judging it, and to mark grep-only claims. Agents made 3 to 50 GitNexus calls each.
- **Index gap:** GitNexus does not index `gate/coverage/` at all (one agent found this, and its claims there are grep-based). The index's ignore list needs checking.
- **Verification:** the coordinator checked 10 of the strongest claims against the code, and all 10 hold: B01#1, B01#2, B02#1, B03#1, B05#1, B06#2, B10#1, B10#2, B11b#1 and B15#1.

## What was found: 1,442 findings

| Group | Count | Should-fix / later | Tests / production | Where most sit |
|---|---|---|---|---|
| **G1** Gate checks that miss what they claim | 3 | 2 blockers, 1 should-fix | 0 / 3 | tools/archcheck, gate/runner |
| **G10** Behaviour defects in production code | 23 | 10 / 13 | 0 / 23 | gate/runner, gate/bench, runtime, playback |
| **G7a** Sprint and review ids in text users see | 6 | 4 / 2 | 0 / 6 | facade, runtime, expr, partition |
| **G2** Tests that do not test what they claim | 156 | 80 / 76 | 154 / 2 | gate/runner, runtime, playback, toolchain |
| **G3** Sprint, review, lane and history names in tests | 258 | 240 / 18 | 258 / 0 | playback, gate/runner, parser, runtime |
| **G4** Other weak names in tests | 234 | 96 / 138 | 234 / 0 | gate/runner, playback, runtime, parser |
| **G5** Weak names in production code (internal) | 189 | 75 / 114 | 0 / 189 | gate/runner, runtime, gate/bench, archcheck |
| **G6** Public API names (each rename needs the owner) | 41 | 21 / 20 | 0 / 41 | facade, event, parser, runtime |
| **G7** Wrong or stale comments, history ids in comments | 218 | 123 / 95 | 163 / 55 | playback, gate/runner, runtime, parser |
| **G8** Duplication and logic in the wrong place | 185 | 40 / 145 | 121 / 64 | gate/runner, playback, parser, toolchain |
| **G9** Dead code and leftover scaffolding | 45 | 15 / 30 | 29 / 16 | playback, gate/runner, parser, validate |
| **G11** Clarity, API shape and the rest | 84 | 19 / 65 | 36 / 48 | gate/runner, playback, parser, runtime |

The grouping is mechanical, by each finding's category, file and wording. Where two agents described one issue from different files, both entries remain; the design step merges them.

### G1: three gate checks that miss what they claim

All three are confirmed and latent: no current code trips them.

1. **`clear(...)` counts as never written.** In the globals check (`tools/archcheck/main.go`, `written`), a package-level table emptied with `clear(...)` still passes as never written. Its twin function, `writes`, counts `clear`.
2. **A reassigned Must-built var passes.** The globals check accepts any `Must*`-built package var without asking whether it is written again. The tool's own doc says such values are "never written".
3. **The LAW Δ helpers ignore git's exit code.** `gate/runner/git.py`'s `numstat`, `ls_files`, `head` and `dirty` throw it away. If `git diff` failed, the LAW Δ block would come out empty and the row would pass.

### G10: behaviour defects (23)

These have a real effect. The should-fix ones:

- **`AppHandle.Start` can leave an app half-started.** If a table write fails, the app stays active without its sources, and a retry gets `ErrStarted`.
- **Drained input is counted unevenly.** A `BatchEnd` after the drain is counted in `AfterDrain`, while a `Send` after the drain is not.
- **A possible race on `appState.waited`.** Two concurrent Drains can read it without its lock.
- **One replay load error says the opposite of the truth.** It says a record "has no" key when that kind of record must not have the key.
- **The escape check can report a false cost.** It counts a helper's heap lines after the split but not before it.
- **`--help` becomes an "internal error"** in two bench tools.
- **Unreadable files crash rows.** The vulns row and the ruff pin crash with a traceback instead of naming the fault.
- **The re-index hook can run twice.** Two calls at once can both start a re-index (check, then start, with no lock).

The rest are later.

### G7a: text users see carries sprint ids (6)

- `@sink(type="http") arrives in S5.1`
- `text operators (S7)`
- `(break-it V1)` and `(owner D1 of S3.2b-2's design)` in runtime error messages
- `owner D7 of S3.2a-1` in a partition error
- review ids in the public godoc

## Patterns across the code

1. **Names record history, not intent.**
   - Sprint ids, review steps and lane ids appear in test files, test functions, helpers and subtests: `breakit_s3_3_1_test.go`, `TestBreakit…`, `TestCv…`, `bk`/`q2`/`pb`/`cg` prefixes, design-row ids such as `slot_3` and `H1`–`H17`.
   - Many of those helpers are the package's shared test harness, used across 10 or more files.
2. **Tests that pass without checking:**
   - checks inside a callback that may never run;
   - one-letter substring matches;
   - tests that accept either outcome;
   - inputs that never reach the path the name promises (escape tests with raw characters, a NaN test refused for its attribute count first);
   - tests placed after `if __name__ == "__main__"`.
3. **Comments that describe the past.**
   - "Red today" notes sit on bugs fixed long ago.
   - `file:line` pointers were broken by S1.1c-6's file splits.
   - Review and decision ids stand in for the rule itself, in production code too.
4. **Vague or misleading production names.**
   - Vague: `mk`, `name`, `value`, `out`, `gate`, `now`, `check`, `judge`, `show`.
   - Misleading: `Sink.Drain` returns a copy and keeps the records; `IngestFlags` returns one bool.
   - Look-alikes for different states: `isClosed`, `Closed` and `stopping`; `ErrStarted` and `ErrActive`.
5. **The same rule written several times.**
   - The select binding in validate and in plan.
   - The duplicate-key JSON check four times.
   - Tracked-file listing seven times.
   - Six recorder test doubles.
   - The tool-cache path rule three times, and the copies disagree.
6. **Logic in the wrong place.**
   - The docs fragment checker lives inside the parser's tests.
   - Runtime and partition tests live in playback.
   - The weekly canary lives in the gate runner's `main.py`.
   - Test modules are used as fixture libraries.
   - Private names are reached across packages.
7. **Dead code.** The whole `internal/metrics` package is unused. Public playback types (`Trace`, `NewReader`, `Replay`) are used only by tests. A clock flag is written and never read. A gate proof driver falls back to a frozen copy of its row.
