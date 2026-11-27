# S1.1c-7 PR 3: architecture review

*A fresh blind reviewer (architect-reviewer, the-fool), at 4ab0d26b, before the rebase onto main at 5de6ad5. The range was cc2c2c1..4ab0d26b, W3's ten production commits, within 30 minutes. The coordinator checked each finding before any fix; the dispositions are the owner's decisions of 2026-11-27 ([PR 3's review](../sprints/decisions.md#owner-decisions-2026-11-27--pr-3s-review)) and the sprint record's Findings.*

**No blockers.**

## PR3A-1: G10-15 leaves Register's duplicate-name check untested (should-fix)

- **Where:** internal/expr/expr_test.go:495, against internal/expr/registry.go:68-76.
- **Scenario:**
  - The test registers `expr.Descriptor{Name: "abs"}`: no Kind and a nil Factory. G10-15's nil-Factory refusal fires first, so the duplicate branch is never reached.
  - Deleting the `if _, dup := …; dup {…}` block leaves every test green. No test asserts "is already registered".
- **Evidence:** the reviewer deleted the three lines; expr, validate and plan were all `ok`.
- **Coordinator's check:** the same plant, run in a throwaway worktree, gave the same result. On the base, the same test line did reach the duplicate branch.

## PR3A-2: after Undeploy, the removed app's record still holds its tables' rows (later; older than PR 3)

- **Where:** internal/runtime/transitions.go:159-161.
  - The tombstone keeps `a.app.Tables`.
  - The caller-side TableWrite check reads it past the gate (requests.go:132-135), so the router cannot nil it.
- **Scenario:** a long-lived Runtime deploys and undeploys an app with large tables, and every stored row stays reachable until Close. The owner's 2026-11-25 decision freed the recording queue only.
- **Evidence:** the reviewer's probe found, after Remove, `rec.q nil=true Loads nil=true table rows still held=3`.
- **Coordinator's check:** confirmed in the code. Remove clears `ing`, `sinks` and `Loads`, but not `Tables`.

## PR3A-3: a stale test comment on the connector refusal (later)

- coverage_s3_3_1_test.go:73 says "the default factory names the sprint of every type". The assertion at :84 checks "is not available in this release".
- The same as PR3Q-4 in the QA review.

## Answers and what held

- **Public contracts:** none broken.
  - Start's new doc matches the design's text.
  - The connector refusal still wraps `ErrConnectorNotBuilt`.
  - The godoc edits only remove references.
  - Every change a caller can see was intended by the design: the starting rows are applied in the activation's step; a recorded app's queue is sized at Add.
- **The queue's release is sound:**
  - Activated app: the router's puts happen before `close(stop)`, which happens before the goroutine's receive, then `q.q = nil`, then `close(returned)`, which Remove waits on.
  - Never-activated app: the router releases the queue itself, since no recorder goroutine ever ran.
  - Stats reads only counters under `mu`.
  - A late Drain gets ErrRemoved.
  - Close goes through Remove; on Close's give-up path the queue is not released, as the comment says.
  - `a.app.Loads = nil` has no reader off the router.
- **G10-1:**
  - A failed or cancelled Start leaves the app startable, stoppable and removable.
  - No starting-row refusal is reachable only at Start.
- **G10-22:** no caller relies on the errors' identity: playback wraps with `%w` but never unwraps; Sync panics with `%v`.
- **G10-15 and G7a-3:**
  - The nine built-ins all register.
  - No old construct name is left in code, goldens or the corpus.
- **G10-16:** the catalogue's fix text (errors.go:38) matches docs/language/errors.md:63.
- **Pre-mortem (the-fool):** the queue's release has two writers. The risks:
  - a later caller-side read of `len(a.rec.q)`;
  - a later Remove of an activated app that is not drained, which would break the guard at transitions.go:61; that is worth a sentence beside the guard;
  - a later real refusal in `applyLoads`.

  None is reachable at 4ab0d26b.

## Not reached

- the design notes' coordinator corrections, line by line;
- a load or memory measurement;
- GitNexus impact and check runs (source search was used instead);
- the test-only commits.

## Re-check of the fix round (2026-11-27)

A fresh blind reviewer read the range 5c87667..c3a795f, mainly the tables' release (`table.Set.Release`, called by the remove step), within 20 minutes. **No blockers.** Ids are RC3A-1 and RC3A-2; the dispositions are in the sprint record's Findings.

### RC3A-1: two remove-step comments give the rows-freed reason as the safety reason (later)

- **Where:** internal/runtime/transitions.go, completeRemove's "Safe: …" sentence and the drain-guard sentence.
- **The problem:** two arguments are merged:
  - why `Release` is race-free: it runs on the router, the tables' only writer and only snapshot taker, and Versions are immutable;
  - why the rows are unreachable when Remove returns: every shard dropped the app's walks at mRemove, acked first.

  A later reader could take the shards' acks as the safety argument.
- Reasoning only.

### RC3A-2: App.Tables does not say the Set must belong to one app (later)

- **Where:** internal/runtime/runtime.go:42.
- Remove now empties the Set for everyone holding it. No production path shares one: `FromPlan`, `newDriver` and `NewIngest` each build their own. The comment should say so.

### Checked and sound

- **No reader races with `Release` or sees a broken state.**
  - TableWrite's caller-side check reads only `s.tables`, `t.name` and `t.attrs`, which are written once before the app is published.
  - The router refuses later writes.
  - The shards hold only immutable Versions, dropped at mRemove.
  - Stats never reads tables; the facade's handles hold no Set; playback's Set is its own.
  - The recorder's table records are copies.
- **No Set is shared or reused:** Deploy builds fresh tables, Add runs once per Deploy, and app ids are never reused.
- **Edge cases hold:**
  - an app with no tables is skipped;
  - an empty Set is fine;
  - a rolled-back Add and a never-activated app empty tables that were already empty;
  - `clear` keeps keyKind and nkey.
- **The GC tests prove the claim:** the Set is kept alive, the weak pointer targets the stored row's own array, and one `runtime.GC()` clears it.
- **Runs:**
  - the new tests under `-race -count=5`;
  - both GC tests under `-race -count=40 -cpu 1,4`, 80 runs each, ok;
  - two mutation checks red;
  - a hostile throwaway test: four writers and one reader across Send, Drain and Remove, then 200 writes after it, under `-race -count=10`. No DATA RACE; every later write answered ErrRemoved; 0 rows.
