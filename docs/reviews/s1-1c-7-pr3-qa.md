# S1.1c-7 PR 3: QA review

*Two fresh blind reviewers (qa-expert), at 4ab0d26b, before the rebase onto main at 5de6ad5, by plants, each within 30 minutes. The first half covered the root package, partition, io/inmemory, tools and the recording queue's release; the second covered the CD packages. The coordinator re-ran each plant that a finding rests on; the dispositions are the owner's decisions of 2026-11-27 ([PR 3's review](../sprints/decisions.md#owner-decisions-2026-11-27--pr-3s-review)) and the sprint record's Findings.*

**No blockers in either half.**

## The first half (PR3Q-1 to PR3Q-5)

### PR3Q-1: only a table's first starting row could reach the engine, and no test notices (should-fix)

- **Where:** deploy.go:226.
- **Plant:** `Rows: rows` → `Rows: rows[:1]`. Green on TestBreakitRecordingWithBatchesTablesAndASourceReplays, and on the whole root package.
- **Why the tests miss it:** every visibility test passes one row per table, and the replay test compares two runs that both read the shortened table.
- **Coordinator:** re-run in its own throwaway worktree: green; the root package ok.

### PR3Q-2: the @source half of G7a-1's text is not pinned (should-fix)

- **Where:** connectors.go:28.
- **Plant:** `kind, title(kind))` → `"sink", "Sink")`, so an @source refusal tells the user to bind a sink. Green on TestCvConnectorFactory and on the whole root package.
- **Coordinator:** re-run: green; the root package ok.

### PR3Q-3: TestEmbeddedInterfaceIsRefused claims the golden stays untouched and never reads it (should-fix)

- **Where:** tools/apipartition/main_test.go:189-190.
- **Plant:** a refused run writes the golden. Green on that test; only the sibling TestEmbeddedFieldIsRefused goes red.
- **Coordinator:** re-run: green.

### PR3Q-4: TestCvConnectorFactory's doc still says the default factory names each type's sprint (should-fix)

- **Where:** coverage_s3_3_1_test.go:73. The architecture review's PR3A-3 names the same line.

### PR3Q-5: the midway-cancel Start test never takes its failure branch (later)

- **Where:** lifecycle_test.go:446.
- The test is a revert guard and works as one. But Start's doc half "an error before the activation leaves the app as it was, and Start may be called again" has no test.

### Noted, no finding

- A plant releasing the queue before the stop's drain is caught elsewhere in the package.
- A plant releasing a live app's queue is caught only under `-race` (T4's race job).
- A release on the Close path was not checked.

### Flake runs

- Every new or changed test in scope: `-race -count=5`, ok.
- No sleeps and no unbounded spins.

## The second half (PR3Q-20 to PR3Q-23)

### PR3Q-20: no test pins table.New's three refusal texts (should-fix)

- **Where:** internal/table/table.go:57, :65 and :79.
- **Why it matters:** "byte-identical texts" was G10-22's safety argument, and TestNewRejects checks only the error, the nil table and `Unwrap`.
- **Plants:** each text changed. Green; the parser, table and playback packages ok.
- **Coordinator:** re-run green. With all four parser and table plants at once, parser, table, playback, runtime and the root were all ok.

### PR3Q-21: lexError's fallback name is not pinned (should-fix; the branch is unreachable through Parse)

- **Where:** internal/parser/parse.go:94.
- **Plant:** `mk(name, …)` → `mk("", …)`. Green; the parser package ok.
- **Coordinator:** re-run: green.

### PR3Q-22: Register's MinArgs > MaxArgs refusal is pinned for a scalar only (should-fix)

- **Where:** internal/expr/registry.go:69-73.
- **Plants:**
  - a pattern-only descriptor skips every check;
  - the arity check applies to scalars only.

  Both green; expr, validate and plan ok.
- **Coordinator:** re-run: green, both.

### PR3Q-23: test_notbuilt.py's ENTRIES is a hand copy that nothing compares with conformance/notbuilt.json (later)

- The JSON itself is pinned by TestCorpus.

### Checked and confirmed

- G2-137's H3 overlaps the deleted row and is stronger.
- G2-133's stamp check catches lost updates.
- G2-66, G2-67, G2-69, and G10-15's two refusals are pinned.
- G2-79's rows are unchanged.
- G2-65's per-fire count; G2-68's bounded sends; G7a-6's claim.

### Flake runs

- **The table break-it:** `-race -count=5`, and at `-cpu 1`.
- **The event package:** its concurrent tests.
- **The clock package:** its break-its.
- **The packages that gained `t.Parallel`:** whole, under `-race`.

All ok.

## Re-check of the fix round (2026-11-27)

A fresh blind reviewer read the range 5c87667..c3a795f by plants, within 20 minutes. **No blockers.**
- Every one of the 11 earlier findings is now pinned: all 14 of its test-gap plants went red on the named test.
- For the tables' release, these plants all went red: dropping the `Release` call, dropping `s.last = nil`, a `clear` that republishes the same Version, and a `Release` that also drops the declarations. Dropping the nil guard panics the runtime package.
- Ids are RC3Q-1 and RC3Q-2; the dispositions are in the sprint record's Findings.

### RC3Q-1: `Set.Release` emptying every table is pinned only for table 0 (should-fix)

- **Where:** internal/table/table.go, Release's loop.
  - Both table tests build two tables but put rows only in T0.
  - The runtime test's app has one table.
- **Plant:** the loop runs over `s.tables[:min(1, len(s.tables))]`. Green on both Release tests, and the table, runtime and root packages are ok.
- **Coordinator:** re-run in a throwaway worktree: the three packages ok.

### RC3Q-2: "the table stays writable" is asserted by a count only (later)

- **Where:** internal/table/set_test.go, TestReleaseKeepsTheDeclarationsAndAnEarlierSnapshot.
- **Plant:** `clear` stores a Version without keyKind and nkey. Green: the Upsert still succeeds and the count is 1.
- **Coordinator:** re-run: green.

### Flake

The three GC tests under `-race -count=10`, and again at `-cpu 1`: ok.
