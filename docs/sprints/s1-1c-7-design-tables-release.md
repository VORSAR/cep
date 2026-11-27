# Design note: a removed app's table rows are released at Remove (PR3A-2)

*The design note for PR3A-2's fix, written by a fresh agent on 2026-11-27 and given here unchanged. It was built as written in b5cb898, 499af5e and c3a795f, with one addition: T2 keeps the Set reachable (`runtime.KeepAlive`), so that its control cannot pass because the whole Set was collected ([the owner's decision](decisions.md#owner-decisions-2026-11-27--pr-3s-review), item 2). T4 was not built.*

Base: `s1-1c-7/integration` at 5c876674. Owner, 2026-11-27 (docs/sprints/decisions.md, "Owner decisions, 2026-11-27 — PR 3's review", item 2): "fixed in PR 3, design first ... It matches the queue's release of 2026-11-25."
GitNexus: impact/context are lower-bound here (`TableWrite`: 4 call sites dropped for receiver typing; `Table.Snapshot`: 6), so every reader below was confirmed with `git grep`.

## 0. Reproduced (the coordinator had not reproduced it yet)

A probe in a throwaway worktree (now removed): `rrRuntime` + `ldStarted(rrRecorded)` (3 starting rows), `weak.Make(&row[0])` on the stored row k1 taken from `a.app.Tables.Table(0).Snapshot().Get(k1)`, 2 Sends, Drain, Remove, `runtime.GC()` twice.
- At 5c876674: the row is still held after Remove, 3 runs of 3 (red).
- With design (a) below prototyped: freed, 3 of 3 (green). A control inside the probe showed the row is held while the app is live.
- With (a) minus its `s.last = nil` line: held again (red). The cached snapshot alone keeps every row.
- With (a) in place, `go test -race -count=1 ./internal/runtime ./internal/table .` exited 0 (log: S/s17/tables-proto-race.txt).

## 1. Every reader of an app's tables or rows

The Set is shared. `newAppState` (internal/runtime/app.go, `ing: NewIngest(app.Streams, start, app.Tables, false)`) gives the same `*table.Set` to `a.app.Tables` and to `a.ing`'s `g.set`. Rows are reachable three ways: each `Table.cur` (atomic current Version), the Set's cached `last *Snapshot` (table.go `func (s *Set) Snapshot()`), and any `*table.Snapshot` or `*table.Version` a message or rule still holds.

| # | Reader | Goroutine | What it reads | Runs after Remove returned? |
|---|---|---|---|---|
| R1 | `Runtime.TableWrite` (requests.go, the `if a.app.Tables == nil \|\| tableID < 0 ...` check and `tb := a.app.Tables.Table(tableID)`, then `CheckRows(tb.Name(), tb.Attrs(), rows)`) | caller | `a.app.Tables` (the pointer), `Set.Len`, `Set.Table` (the `tables` slice), `Table.name`, `Table.attrs`. Declarations only, never `cur` or rows. | **Yes.** A caller that passed `gate` (phase active, not draining) before Drain began can be descheduled while Drain and Remove both finish, then run the check. This is why the router cannot nil `a.app.Tables`. After the check, `r.request` reaches `router.writeTable`, which answers `ErrRemoved` on `a.ing == nil`. |
| R2 | `checkLoads` (build.go, `a.Tables.Table(l.Table)`, `tb.Check`) | caller (Add/Start) | the App being added: declarations | No. It runs before install and only on a new App. |
| R3 | `router.writeTable` → `appState.applyWrite` → `appState.write` (router_in.go, app.go `tb.Upsert`/`tb.Delete`) | router | `Table.mu`/`cur` (the only live writer: owner F14) | No. `writeTable` refuses `a.ing == nil`; `activate`→`applyLoads` refuses `a.removing`. |
| R4 | `Ingest.Event` (ingest.go `e.Tables = g.set.Snapshot()`), `router.tickApp` (`a.ing.Snapshot()`), `router.drainApp` (`a.ing.Snapshot()` in mDrain) | router | `Set.Snapshot`: `cur` of every table, writes `s.last` | No. `a.ing` is nil after `completeRemove`. Before that, `tickApps` skips `!a.activated \|\| a.drained`, events are refused for a drained app, and Remove of an activated app requires its drain complete (`remove`'s guard). So no snapshot is taken after the remove item for an activated app, and none was ever taken for a never-activated one. |
| R5 | shards: `shard.process` passes `m.tables` to `offerAll`/`advance`; `partition.Rule.Advance(tables)` sets `r.tables` and resets it to nil; `plan.body.OnEvent` sets `b.row.tables` and resets it with `defer func() { b.row = row{} }()`; buffered events keep `e.Tables` inside the rule's partition state | shard | immutable `Version`s through a `*table.Snapshot`. Never `Table.cur`. | No. `mRemove` is behind every earlier message on each shard's FIFO mailbox. `shard.uninstall` nils `sh.rules[ai]` and `sh.bodies[ai]`, which drops every buffered event and its snapshot. The ack leads to `installAck` → `completeRemove`. A message for the freed slot (shard.go "a message for a slot this shard freed") holds its snapshot only until it is processed, so it is transient. |
| R6 | Stats (stats.go) | router (mStats) / caller | `a.ing` (nil → zero rows), counters | Yes, but it never reads tables. |
| R7 | facade `cep.Table` (table.go: `Upsert`, `Delete`, `Load`), built by `tableFor` from `AppHandle.plan.Tables` | caller | It holds `rt`, `app` id, table `id`, `attrs`, `names`, `keyPos`, so no Set and no rows. Every write is `TableWrite` → `gate` → `ErrRemoved` after Undeploy (via `mapErr`). | Yes, and it gets `ErrRemoved`, as it does today. |
| R8 | `AppHandle.Table` (apphandle.go `func (a *AppHandle) Table(name string) (*Table, error)`), `bind` (deploy.go `t := tableFor(h, tid)`, rows handed to `App.Loads`) | caller | the plan's declarations; the Loads rows are dropped at Remove already (`a.app.Loads = nil`) | Yes, but it reads no rows. |
| R9 | the recorder (recorder.go): `rTable` records carry `copyRows` copies, not the table's rows | recorder | its own copies | The queue is released at its stop (owner, 2026-11-25). |
| R10 | playback / `Sync` (playback/driver.go `runtime.FromPlan(app, d.set, nil)`; sync.go `NewSync`, `Begin`, `Snapshot`) | the driver's | its own Set | It never sees Remove, so it is not affected. |
| R11 | tests that hand `FromPlan` or `App.Tables` their own Set (`pbTables`, `rrRecorded`) | test | whatever they kept | Yes. Under (a) they see empty tables after Remove (see §3). |

Nothing reads a snapshot without the router except the shards (R5, given by router messages) and the playback driver (R10, its own Set).

## 2. Candidate designs

**(a) Empty the tables, keep the Set (the queue's pattern: keep the shell a caller reads, release the content).**
- New: `func (s *Set) Release()` in internal/table/table.go. Each table publishes an empty Version, and the Set drops `s.last`. The router calls it in `completeRemove`.
- Frees: every stored row and every Version (the old `cur` and the cached `last`), once the shards let go. By R5 they already have when `completeRemove` runs. It keeps the Set, the `[]*Table`, and each Table's name, attrs, key, mutex and an empty Version: a few hundred bytes per table. The tombstone keeps the same order of metadata already (`a.app.Rules`, `a.dec`, `a.rec`).
- Synchronisation:
  - `Table.cur` is an `atomic.Pointer` whose writers are the router only (F14). `Release` runs on the router and takes `t.mu` like every writer, so it is ordered with every write.
  - `s.last` is written only by `Set.Snapshot`, which also runs only on the router, so `Release` writing it is the same goroutine.
  - R1 reads only `s.tables`, `t.name` and `t.attrs`. Those are written once, before the app is published (`r.apps.Store(&next)`; the caller's `appOf` loads it: happens-before), and never again. `Release` writes none of them, so there is no race with R1.
  - Shards: each ack of `mRemove` (channel send) happens-before `completeRemove`, so no shard walk holds a Version then. Versions are immutable, so even a straggler stays correct.
- What a caller observes after Undeploy: nothing new. `cep.Table` writes give `ErrRemoved`, as today. There is no public reader of rows. Only code holding the Set itself (R11, internal tests) sees `Len() == 0`.
- Per-event cost: none. One loop over the tables per Remove on the router, one `make(map, 0)` per table.

**(b) Stop R1 reading `a.app.Tables` past the gate, then nil it.**
- At Add, `newAppState` copies the declarations into a new field (for example `tableDecls []tableDecl{name string; attrs []event.Attr}`), which is written before the publish and never again. TableWrite's check reads that field. `completeRemove` sets `a.app.Tables = nil`.
- Frees: everything, including the Set's shell.
- Synchronisation: the field is published by `r.apps.Store`. `a.app.Tables` becomes router-only, and the router's own readers (R3, R4) are guarded by `a.ing == nil` and `a.removing`.
- What a caller observes: the same as (a).
- Per-event cost: none. But it changes `TableWrite`, which has a benchmark (internal/runtime/table_write_bench_test.go `BenchmarkTableWritePath`, `BenchmarkTableWritePathRecorded`), so it needs a `compare.py` timing on mains (K1h-adjacent).
- Weak points: (1) A row reachable from a second holder of the Set (R11) is not freed. (2) It leaves the trap that caused this defect: any future caller-side read of `a.app.Tables` past the gate is a nil dereference or a race. (3) It duplicates declarations the Set already holds.

**(b′) Move the shape check to the router (`writeTable` runs `CheckRows`).** Rejected.
- It puts per-write row checking on the router's critical path. That is a K1h on `BenchmarkTableWritePath`.
- It changes the error order a caller sees: a malformed write to a draining app gets `ErrDrained` today, but would get the row error, or the reverse.

**(c) An `atomic.Pointer[table.Set]` for `a.app.Tables`, nil'd at Remove.** Rejected.
- It costs one more atomic per TableWrite.
- A racing write would get "no table %d" instead of `ErrRemoved` unless nil is mapped to `ErrRemoved`. That is more moving parts than (a) for no more memory freed.

## 3. Chosen: (a)

- It frees the rows (all the volume) wherever the Set is held.
- It leaves the caller-side check untouched and race-free by construction.
- It adds nothing to a per-event or benchmarked path.
- It matches the 2026-11-25 shape: `a.rec` stays for callers and `a.rec.q` is released.

**No owner question on caller behaviour.** Under (a) and (b) alike, a `cep.Table` handle keeps returning `ErrRemoved` after Undeploy, and no public API reads rows. One record line instead of a question: `runtime.App.Tables` (internal) now says "Remove empties them". An internal caller that hands its own Set sees it emptied. The only such callers are tests, and no test at 5c876674 reads a Set after Remove (the full -race run with the prototype is green).

## 4. The exact change

**4.1 internal/table/table.go, after `func (s *Set) unchanged(snap *Snapshot) bool { ... }`:**
```go
// Release empties every table and drops the cached snapshot, so the rows go once no reader holds a Snapshot or
// Version taken before it; one taken before it still reads its rows (Versions are immutable). The declarations stay:
// Len, Table, Name and Attrs answer as before. Like Snapshot, one goroutine only: the tables' writer, which calls it
// at an app's removal (owner, 2026-11-27; PR 3's review PR3A-2).
func (s *Set) Release() {
	for _, t := range s.tables {
		t.clear()
	}
	s.last = nil
}

// clear publishes an empty Version.
func (t *Table) clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cur.Store(t.cur.Load().empty(0))
}
```
The package comment's purity sentence ("one sync.Mutex and one atomic.Pointer, nothing else") still holds.

**4.2 internal/runtime/transitions.go, `completeRemove`.** Replace the comment block that begins `// Only the router's own fields are dropped:` and the line `a.app.Loads = nil` with:
```go
	// Only the router's own fields are dropped: a.dec and a.rec stay, because caller goroutines read them past the
	// gate (SendPayload's decoder, Activate's and Drain's recorder) (review S4). The starting rows go too: only the
	// activation step read them, and it refuses a removing app (owner, 2026-11-25; PR 2's review PR2A-3, PR2Q-10).
	// The tables' rows go, but not their Set: TableWrite's check reads its declarations past the gate, so Release
	// empties each table and keeps them (owner, 2026-11-27; PR 3's review PR3A-2). Safe: the router is the tables'
	// only writer and their only snapshot taker, and every shard dropped the app's walks at mRemove, acked before here.
	a.phase.Store(int32(phaseRemoved))
	a.ing, a.sinks, a.quiet = nil, nil, true
	a.app.Loads = nil
	if a.app.Tables != nil {
		a.app.Tables.Release()
	}
```
(Keep the existing `// The slot keeps a tombstone ...` lines above it unchanged.)

**4.3 internal/runtime/transitions.go, `remove`: the guard sentence the review asked for.** Directly above `if a.activated && !(a.drained && a.drainRes != nil) {`:
```go
	// An activated app is removed only once its drain is complete; the remove step's releases rely on it: the
	// recorder's release of its queue at its stop (every record is queued before the stop), and the release of the
	// tables' rows (no event, tick or drain walk of the app follows its drain, so no shard is handed a snapshot after
	// mRemove).
```

**4.4 internal/runtime/requests.go, `TableWrite`.** Above `if a.app.Tables == nil || tableID < 0 ...`:
```go
	// The declarations, read past the gate: Remove keeps the Set and empties it (completeRemove), so they stay valid.
```

**4.5 internal/runtime/runtime.go, `App.Tables`'s field comment.** `// nil: no tables; the router is their only live writer (owner F14)` becomes `// nil: no tables; the router is their only live writer (owner F14), and Remove empties them` (wrap if it is over the line limit).

## 5. Tests (red first)

New tests use the stdlib `weak` package (Go 1.27.4, the pinned toolchain), so there is no new dependency. Each takes its weak pointer inside a helper, so no stack slot of the test frame keeps the row alive. Each also asserts that the row is held before the release (the control), so a vacuous pass is impossible.

**T1** internal/runtime/recorder_release_internal_test.go: `TestARemovedAppsTableRowsAreReleased`. Extend that file's header comment with "and the tables' rows (owner, 2026-11-27, PR3A-2)".
- Behaviour: once Remove has returned, no stored row of the app's tables is reachable, and the tables' declarations stay for TableWrite's check.
- Steps: `rrRuntime`; `id := ldStarted(t, ctx, r, rrRecorded(t))` (3 starting rows put at activation); `a, _ := r.appOf(id)`; `wp := trWeakRow(t, a, "k1")`. The helper takes `a.app.Tables.Table(0).Snapshot().Get(...)` and returns `weak.Make(&row[0])`. Then 2 `Send`s; `runtime.GC()`; assert `wp.Value() != nil` (control: held while live). Then `Drain`, `Remove`, `runtime.GC()`.
- Assertions:
  - `wp.Value() == nil`;
  - `a.app.Tables != nil && a.app.Tables.Len() == 1`;
  - `a.app.Tables.Table(0).Name() == "T"`;
  - `a.app.Tables.Table(0).Snapshot().Len() == 0`.
- Red at 5c876674: the row is held and Len is 3 (probe: 3 of 3).
- Plants (find → replace, expected outcome):
  - P1 transitions.go `a.app.Tables.Release()` → `_ = a.app.Tables`: red (row held, Len 3).
  - P2 transitions.go `a.app.Tables.Release()` → `a.app.Tables = nil`: red (`a.app.Tables != nil` fails). This plant pins why the Set must stay.
  - P3 table.go `s.last = nil` → `// s.last = nil`: red (row held: the cached snapshot). Confirmed in the probe.
  - P4 table.go `t.clear()` → `_ = t`: red (row held, Len 3).

**T2** internal/table/set_test.go (package table_test): `TestReleaseFreesTheRowsOnceNoEarlierSnapshotIsHeld`.
- Behaviour: after Release, a row stored before it is unreachable once no earlier Snapshot is held, including the Set's own cached one.
- Steps: a Set of two tables, with 3 rows upserted in the first. A helper takes `s.Snapshot()` (which fills the cache) and returns `weak.Make(&row[0])` for one stored row. Then `runtime.GC()` and the control `wp.Value() != nil`. Then `s.Release()`, `runtime.GC()`, and assert `wp.Value() == nil`.
- Red at HEAD: a compile error, since `Release` does not exist. Plants P3 and P4: red.

**T3** (control: the readers' tolerance) internal/table/set_test.go: `TestReleaseKeepsTheDeclarationsAndAnEarlierSnapshot`.
- Behaviour: Release changes no declaration, and a Snapshot taken before it still reads its rows.
- Steps: `before := s.Snapshot()`, then `s.Release()`.
- Assertions:
  - `s.Len() == 2`;
  - each `s.Table(i).Name()` and `Attrs()` is unchanged;
  - `s.Snapshot().Table(i).Len() == 0` for each i, and `s.Snapshot() != before`;
  - `before.Table(0).Len() == 3`, and `before.Table(0).Get(k1)` returns the row;
  - an `Upsert` after Release succeeds and `s.Snapshot().Table(0).Len() == 1` (the table stays usable, though the runtime never writes it again).
- Plant: `t.cur.Store(t.cur.Load().empty(0))` → `t.cur.Store(nil)`: red (a nil Version panics on `Len`).

**Existing control:** a `cep.Table` write after Undeploy still returns `ErrRemoved`. Find the existing test with `git grep -n ErrRemoved -- lifecycle_test.go breakit_s3_3_1_test.go`. If none asserts it for a Table write, add `TestATableWriteAfterUndeployIsRefusedRemoved` to the root lifecycle_test.go. It passes at HEAD and must still pass.

**Option, not recommended (T4):** `TestATableWritePastTheGateWhileItsAppIsRemovedIsRefusedRemoved`. It needs a `writeHold func()` seam in `Runtime` (beside `addHold`), called in TableWrite after `gate`. The test parks a write there, drains and removes the app, releases the hold, and asserts `errors.Is(err, ErrRemoved)`. With P2 it would return "no table 0" (red). It pins the race window directly. But it adds a branch to the benchmarked `TableWrite` path (needs `compare.py`), and T1's `a.app.Tables != nil` assertion already catches P2. Recommendation: leave it out.

**Race and load runs.**
- Run `go test -race` of ./internal/runtime, ./internal/table and . once (the prototype was green, exit 0).
- Also run T1–T3 with `-race -count=20`, to show the GC assertions are not flaky. A flake stops the line (rule 11); never retry.
- The `load.py` 50/50 run is **not needed**. No new cross-goroutine access is added: `Release` writes only router-owned state (`cur` under `mu`, `last`), and R1 reads only fields written before the publish.

## 6. Risks: a pre-mortem (the-fool, "find the failure modes", chosen here since no one can be asked)

Steelmanned thesis: emptying the tables in the router's remove step frees the rows with no race and no caller-visible change, because the router is the only writer and snapshot taker, the shards let go before they ack, and R1 reads only immutable declarations.

1. **A Set shared by two apps.** One Remove would empty a live app's tables, which would be a wrong detection (blocker class).
   - Today: the public path builds a fresh Set per app (deploy.go `runtime.FromPlan(p, nil, bindings)`). Only internal tests pass their own.
   - Mitigation: the `App.Tables` comment (4.5) makes the ownership explicit. Before building, the builder should confirm with `git grep -n "Tables:" -- 'internal/**/*_test.go'` and `git grep -n "FromPlan(" -- '*_test.go'` that no test hands one Set to two apps in one Runtime.
   - Not done here (time box): this is **open**.
2. **Flaky GC assertions** (a row kept alive by a register or stack slot gives a false red).
   - Mitigation: weak pointers taken in helpers, a deterministic `Len` assertion next to each, and the `-count=20` run.
   - Evidence: the probe was 3 of 3 each way.
3. **A future caller-side reader of row content past the gate** (say, a facade `Table.Get`). After Undeploy it would read an empty table rather than crash.
   - Mitigation: any such API must go through `gate` and so answers `ErrRemoved` first. The Release doc states that the declarations stay and the content goes.
4. **Someone moves `Release` earlier** (into `remove`, before the acks). It stays correct, because Versions are immutable, but it frees later.
   - Mitigation: the comment in 4.2 names the ack ordering.
5. **The rollback path.** `completeRemove` also completes `rollBackAdd`'s removal. The tables are empty there, because Loads are put only at activation, so `Release` is harmless. T1 does not cover it; a builder may add an assertion to an existing rollback test, at low value.

**K1h:** no. Nothing on a per-event or benchmarked path changes:
- `Release` runs once per Remove;
- `TableWrite` gets only a comment;
- `BenchmarkTableWritePath` is untouched unless T4's seam is adopted.

So no `compare.py` timing is needed.

## Open
- Risk 1's grep for a Set shared by two apps (the builder's first step).
- Whether the coordinator wants T4 despite its benchmark cost (recommendation: no).
- Partial check for risk 1, done at the time box: a heuristic grep of `[]runtime.App{...}` and `Tables: <set>` in the tests found no Set handed to two apps; the playback tests build one with `pbTables` per call. The grep is not exhaustive, so the builder confirms it.
