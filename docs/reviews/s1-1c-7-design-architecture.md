# S1.1c-7 design — architecture review

*The blind architecture review(s) of S1.1c-7's nine design notes. The reports are as the agents gave them; paths are made repo-relative. Every finding's disposition is in [the coordination record](../sprints/s1-1c-7-design-coordination.md).*


## Part one: seven notes and two addenda (ids DA-)

S1.1c-7 design notes: blind architecture review (with the-fool's pre-mortem and evidence-audit modes)

### Header

**Not reviewed: design-CD1.md and design-CD2.md.** Neither file was in the scratchpad when I reached the report (`ls`: "No such file or directory"). Both addenda (design-E1-addendum.md and design-E2-addendum.md) had arrived, and I reviewed them. Please send the CD notes.

**What I read**
- **Binding plan parts:** plan check §0 (items 4, 5, 9), §1, §2, §4, §5, §7, R1–R8 and §11. Also ground rules, CLAUDE.md, the review brief and design-coord-items.md (re-read after it changed; items 1–29).
- **Every note:** the header, summary table, commit plan, choices, couplings and risks, and carried items.
- **K1, K1c and K1h sections in full:**
  - W0 Part A (mutate.py's contract);
  - A1a: G10-7…13 (summaries) and G3-72;
  - A1b: G3-72, the rejections and K2-dels from the summary table;
  - A2: G10-5 and G10-14;
  - B: G10-18 and G10-19, plus G2-89 (BASELINE_TEST);
  - E1: G10-20, G10-21 and G7a-5;
  - E2: G10-1, G10-23, G7a-1, G7a-2 and G7a-4;
  - E1 addendum: all of it. E2 addendum: header, G2-143 and the commit plan.
- **K2 sections read:**
  - E1: G2-115, -117, -127, -131 and -132. The sha256 sample gave only G2-117 and G2-131; I added three risky ones.
  - B G2-106 and E2 G2-106, as a cross-lane pair.
- **Sample sections I did not read** (the QA half covers proof detail):
  - W0: G2-7, -147, -149, -154, -155;
  - A1a: G2-21, -23, -41, -43;
  - A1b: G2-27, -33, -35, -36, -45;
  - A2: G2-9, -11, -12, -15;
  - B: G2-96, -102, -105, -107, -109;
  - E2: G2-1, -81, -156.

**GitNexus**
- Index lastCommit is 1f4ebba = HEAD (`~/.gitnexus/storage/cep-7b08f6c21e42/meta.json`). I had no resource-reading tool, so I took freshness from meta.json.
- Calls:
  - a cypher lookup of Activate/Start;
  - `impact writeTable` upstream: LOW, exact, process `route`, so it is per-event;
  - `context newRecQ`: one caller, `newAppState`, exact.

**Memory:** no Hindsight tools were available here, so none was used.

**Checked by reading code (file:line):**
- drain.go:15-135;
- lifecycle.go:80-190;
- requests.go:37-143;
- transitions.go:76-91;
- router_in.go:100-115 and :140;
- recorder.go:55-115; app.go:75;
- apphandle.go:40-62; deploy.go:105-240; table.go:30-75;
- suppressions.py:58-62; test_suppressions_planted.py:40-44;
- rows/__init__.py:1-30; law.json instruments.

Grep-only claims are marked (grep).

### Findings

No blockers. DA-S1 comes closest; see its reason.

#### DA-S1: the queue limit for starting rows becomes a contract when a bounded fix exists (E1 addendum G10-1 runtime half, E2 G10-1)
- **Claim.** Under option B, the header and every table's starting rows are queued before the recorder goroutine starts. The addendum accepts that a total above `RecordQueue` ends the recording at once. It pins this with `TestTheHeaderAndTheStartingRowsShareTheRecordQueue` ("one over" → Failed).
- **Evidence:**
  - The queue capacity is fixed per app at creation: app.go:75 `newRecQ(app.Recorder, r.cfg.RecordQueue)`.
  - `newRecQ`'s only caller is `newAppState` (GitNexus context, exact).
  - `put` never blocks: a full queue fails the recording (recorder.go:91-110).
  - Today each table write races a recorder that is already running, so a multi-table load whose total exceeds the capacity may or may not record.
  - Under B it fails every time, and the test makes that failure the specified behaviour.
- **Why it matters.**
  - "No fix creates a new defect." Under B, a deploy whose tables each fit but together do not loses its recording deterministically.
  - The addendum rejects "an unbounded queue". But the rows are known at Add, so a bounded fix exists: size this app's queue at Add as `max(cfg.RecordQueue, 1 + Σ len(rows) + headroom)`. That is a one-line change at app.go:75 and keeps owner D2 of S3.2b-2 (a full queue still ends the recording).
  - It is S, not B, only because today's outcome is already timing-dependent.
- **Fix to the note:** E1 sizes the queue at Add, or puts both options to the owner at G1. Replace the "one over fails" row with "two tables whose rows together exceed RecordQueue record in full". Reword the App.Loads and RecordQueue docs to match.

#### DA-S2: a third agent during W1 for `table.Check` breaks ground rule 17 (coordinator item 26)
- **Claim.** The ruling has a fresh go-engineer write `table.Check` *during* W1, while lanes A1 and B run.
- **Evidence:** ground rules 17 ("≤ 2 agents at once") and R8; plan §1, where W1 = A1 + B.
- **Why it matters:** that makes three agents at once.
- **Fix:** make `table.Check` E1's first commit in W2, with E1 owning internal/table/table.go for W2 only. CD keeps it in W3, so one owner per file per wave still holds and no extra agent is needed. Otherwise schedule it in a gap between W1's hand-backs and W2's start. Either way it still takes the coupling slot.

#### DA-S3: E2's facade half is not updated for the addendum: a new Deploy refusal has no chosen text, test or dry run (E2 G10-1, coordinator item 27)
- **Claim:** with Condition A, a starting row that the table refuses now fails at `Add` inside Deploy. E2's note, written before the addendum, does not cover this path. Its addendum says only "E2-1 still depends on E1's seam".
- **Evidence:**
  - The facade's checks are `t.rows` (table.go:36-46, the RowWriter) and `runtime.CheckRows` (deploy.go:217-221). Neither applies the table's key-null rule. Example: an optional key column left null passes Deploy today, and `table.checkValue` then refuses it at Start. That is a deterministic G10-1 trigger that E2:78 misses.
  - Add's error reaches the user as `mapErr(err)` (deploy.go:109-113). It carries runtime wording ("runtime: app N: starting rows for table 0: …"), not the facade's `cep: deploy X: WithTableRows("T"): …` used at deploy.go:212 and :221.
- **Why it matters:**
  - This is user-visible behaviour and text, which plan §5 says the notes must decide, with a dry run.
  - It also makes Deploy's error wording inconsistent.
- **Fix to E2's note:**
  - choose the text: wrap at deploy.go:109, or call the table check in the facade with its own wording before Add;
  - add a facade test: a starting row with a null optional key fails Deploy with that text;
  - dry-run the existing pins;
  - correct the claim at :78.

#### DA-S4: mutate.py's worktree self-check compares the whole repository's worktree list (W0 Part A, `worktree()` step 8)
- **Claim:** step 8 raises `Internal("a worktree was left")` when `git worktree list --porcelain` differs from its value before the run.
- **Evidence:**
  - Every lane works in its own worktree of the same repository (plan §9). Each wave runs two lanes at once, and both use `--expect`.
  - `git worktree list` reads the shared common dir, so it shows the other lane's worktrees, including its throwaway mutate worktrees.
- **Why it matters:**
  - One lane's proof run fails because of the other's activity, with exit 2 and a message pointing at a worktree that is not its own.
  - An agent may then remove or prune a sibling's worktree.
- **Fix:** after the `finally`, assert only that this run's own `path` is absent from the list and that `tmp` is gone. Keep the whole-list check for counter-example #16's isolated test repository only.

#### DA-S5: B's K1 proofs are written in a shape W0's contract refuses (B G10-18, G10-19)
- **Claim:** the K1 revert plants are `--expect` entries with `old_rev = <lane base>` and expect `{old: green, new: red}`.
- **Evidence:**
  - W0's `read_batch` checks the mutation exact-once against each revision's blob (W0 A.2).
  - At the lane base, the find texts are absent: `"a %s record takes no %q key"` and `fault("%w", inPlayback{err})` arrive with B-K1.
  - Plan §7.1: a K1 proof is a revert plant on the fix commit, outcome red.
- **Why it matters:** B's batch is refused before any run. The lane would have to redesign its proof at build time, against sprint model §6's "build without re-investigating".
- **Fix:** move both entries, and the chain plant, to a plain K1 batch at the B-K1 commit, expecting red. That matches W0's own `W0-py-k1.json` and the other notes.

#### DA-S6: two new markers for "this test cannot run in parallel", against the repo's existing one (B G2-106, E2 G2-106 partition part)
- **Claim:**
  - B adds "It runs serially: …" and checks for it case-sensitively (`'runs serially'`).
  - E2 adds "// Runs serially: …" and checks for `'Runs serially:'`.
- **Evidence:** the repo already writes "Not parallel: <reason>" at 13 sites (grep). Examples: internal/playback/encode_refusals_internal_test.go:72, hot_path_allocs_internal_test.go:9, internal/clock/firegroup_bench_test.go:40. testing.md:13: "a test that cannot says why".
- **Why it matters:**
  - It creates a second and a third copy of one rule.
  - Each check ignores the other spellings, so any later gate rule meets three of them.
- **Fix:** both notes use `Not parallel: <reason>` and one shared check pattern that accepts it. Keep the reasons: B's GOMAXPROCS sentence and E2's whole-process sentence are both true.

#### DA-S7: the K1h timing does not measure the code it edits (E1 C6 G10-21, C7 G10-1)
- **Claim:** both commits time only `BenchmarkSendPath|BenchmarkSendPayloadPath`.
- **Evidence:**
  - C6 edits only BatchEnd's post-drain branch (lifecycle.go:177-183).
  - C7 moves `writeTable`'s body into `applyWrite`. `writeTable` is on the per-event Upsert/Delete path (GitNexus `impact writeTable`: process `route`).
  - internal/runtime has only three benchmarks (hotpath_bench_test.go:165, :203, :263), and none runs either edit.
- **Why it matters:** ground rule 13a's slowdown check is then vacuous for the touched path, yet the record will read "compare.py exit 0".
- **Fix:**
  - C7: add a new (not frozen) table-write benchmark, or time `writeTable`'s path some other way, parent against commit.
  - C6: say in the note that the edit sits only on the refused branch after the drain, so the Send compare checks only for collateral cost (inlining, layout).

#### DA-L1: G10-1's candidate list misses the cheapest option, and B's full cost should reach the owner (E2 G10-1)
- **Missing candidate:** a facade-only fix. Run the starting-row writes under `context.WithoutCancel` and check rows at Deploy with the table's own check.
- **B is still the better design.** It also closes:
  - the window where a concurrent Send sees the table before its starting rows;
  - the deterministic table-refusal trigger in DA-S3.
- **Why the owner should see B as a cross-package move:** it moves responsibility from the facade to the runtime (Q4 puts cross-package moves later). It now costs:
  - three commits across three agents;
  - new internal API: App.Loads, TableLoad, table.Check;
  - three new runtime texts;
  - a change in public behaviour.
- **Fix:** add the facade-only option to the candidate table with its reason for rejection. Put B to the owner at G1 with this cost.

#### DA-L2: the new G10-21 test's source-stream half adds a sixth pin to move (E1 G10-21)
- The test asserts `st[0].AfterDrain == 1`. That duplicates lifecycle_test.go:724 and :759, which are designed behaviour (MUST 11, T30).
- It adds one more pin for S3.3-1-3's fix, which E1 itself recommends, to move.
- It does not make S3.3-1-3's wrong part a contract: the window before the drain item is not asserted.
- **Fix:** drop the source half, or name it as a copy of the :724 pin.

#### DA-L3: A1 has two notes and no single commit list (A1a §4, A1b commit plan, coordinator item 12)
- Four commits in A1 touch test_suppressions.py: A1a C1, A1b C1, A1b C2 and G3-72.
- A1a leaves open whether C1 splits. gate/runner and gate/runner/rows are separate Python packages, and plan §1's K1 covers one package.
- **Fix:** the coordinator writes one ordered A1 list and decides the split now. Projected: K1 1–2, K2 2–3, K2-del 1, coupling 1, against a lane ceiling of 6.

#### DA-L4: the coupling line in the budget
- Coupling commits are now G3-72, G2-24, G7a-3 and table.Check: 4 against the plan line of 2.
- E1 sits exactly at its ceiling, with nothing spare for a fix round.
- My tally without CD's real numbers is about 84–86, under 95.
- **Fix:** re-project when the CD notes arrive.

#### DA-L5: `--expect` may be slow enough to strain the 30-minute boxes (W0 A.2 `worktree()` and `expect()`)
- Each side builds in a fresh worktree at a new path. Without -trimpath, Go keys module packages in its build cache by directory (cmd/go's action-ID rule, from memory, not run). So every side may recompile, and with 20–40 sides per lane the lanes could overrun their 30-minute boxes.
- **Fix:** reuse one worktree per revision across entries, resetting it between them. Do not use -trimpath: it changes the file paths in stack frames, which the frame tables of §0 item 4 match against.

#### DA-L6: the proofs-row rule for law paths (coordinator item 15)
- Plan §7.3 runs the proofs row when a lane touches "gate/ or a coupled file (§0 item 5)", and law paths are in that table.
- The coordinator's reasoning ("not triggered, copy_gate copies gate/ only") is true in effect but contradicts the rule's text.
- **Fix:** run it (cheap), or annotate §7.3.

#### DA-L7: record G7a-2's widening site by site (E2 G7a-2, coordinator item 24)
- The widening stays K6: astequal shows only comments changed, and it is within G7a-2's reason (rendered public godoc).
- **Fix:** list each widened site in the ledger row as "chosen by the design note", so the spot-check can match the change against the design.

#### DA-L8: G10-20's helper can make an expired-ctx Drain wait (E1 G10-20)
- After `revertUnlessQuieted`, a Drain whose ctx has already ended can wait on `runMu` behind another Drain's `awaitRuns`, which holds runMu while it waits for runs (drain.go:112 onwards).
- The wait is bounded by that Drain's own wait, and awaitRuns' own Lock already has the same exposure. The "no lock-order risk" claim holds.
- **Fix:** state the wait in the note.

### Coordinator items I disagree with

| # | Disagreement |
|---|---|
| 15 | §7.3's text calls for the proofs row on a coupled file; run it or annotate §7.3 (DA-L6) |
| 22 | The RecordQueue risk is treated as a pin; a bounded fix removes it (DA-S1) |
| 26 | A third agent in W1 breaks ground rule 17; make it E1's first W2 commit (DA-S2) |
| 27 | "Tell the owner" is not enough: E2's note needs the new Deploy refusal's text, test and dry run (DA-S3) |
| 12 | Agreed, but A1 needs one merged commit list (DA-L3) |

**Agreed, checked against the code:**
- **Item 10:** E-PART-004 is decided by SPEC (docs/language/SPEC.md:98, rev 5; validate/bounds.go:125, grep).
- **Item 9:** the corrected chain plant.
- **Items 2 and 19:** hooksPath set to /dev/null on worktree add and on commit.
- **Item 3:** the hold gives 1/1.
- **Item 14:** the leftover logic window goes to the backlog.
- **Item 13:** option (C) is sound.
  - It fixes the derived-stream variant both before and after the drain item, so a recording no longer becomes unreadable (records.go:277).
  - It leaves S3.3-1-3's source-stream window as the known issue.
  - It does not turn that window into a contract.

### Pre-mortem (the-fool), "it is week two and the sprint failed because…"

1. **G10-1 slipped across three waves.** `table.Check` landed late, E1 was at its ceiling, and E2-1 waited (DA-S2, DA-L4).
2. **W1's `--expect` runs failed with "a worktree was left".** Each lane saw the other's worktrees, and an agent deleted its sibling's worktree (DA-S4).
3. **The G1 owner later found a lost recording contract.** A deploy with two large reference tables recorded nothing (DA-S1).

**Mitigations:** the fixes above. All are note edits except DA-S1's choice of queue size.

### Evidence audit: claims that held against the code

- **G10-20:** unlocked reads at drain.go:28 and :80, the write at :119 under runMu.
- **G10-21:** the drained branch has no Derived check (lifecycle.go:177-183).
- **G10-1:**
  - apphandle.go:44-51;
  - `enqueue` checks ctx first (requests.go:57);
  - `activate`'s default branch (transitions.go:85-89).
- **G3-72:** the old texts sit at suppressions.py:61-62, test_suppressions.py:617 and :619, test_breakit_s11c2.py:359 and test_suppressions_planted.py:42-43. Both notes' new texts agree.
- **W0:**
  - `rows.SHIM_FLAGS` exists (rows/__init__.py:24);
  - gate.bench already imports gate.runner (mutate.py:35), so no new layering edge;
  - mutate.py is not a law.json instrument, so G2-24 does not interact.
- **R5:**
  - B G10-18 and E1 G7a-5 use the named text-only exemptions;
  - G10-21 and the G10-1 runtime half are K1h;
  - W0, A1 and A2 have no per-event Go code.

### Verdict

**Not ready for G1 yet**, but nothing needs redesign except one choice:
- **Note edits:** DA-S2 to DA-S7.
- **One owner-level choice:** DA-S1's queue sizing, or acceptance of the narrowed limit.
- **Before G1:** the CD1 and CD2 notes still need this review. In particular, check whether G10-22 or any other fix in internal/table changes the table texts that E1's `TestAddRefusesStartingRowsItCouldNotApply` and the `table.Check` test match with `strings.Contains`. That would be a hidden W3 coupling into internal/runtime, which no one owns in W3.

Files:
- s17/design-E1-addendum.md
- s17/design-E2.md
- s17/design-W0.md
- s17/design-B.md
- s17/design-coord-items.md
- internal/runtime/app.go
- internal/runtime/recorder.go
- deploy.go


## Part two: CD1, CD2 and the CD2 addendum

Architecture review, part two (supplement to my first report): CD1, CD2 and the CD2 addendum, plus coordinator items 30–36

### Header
- **Covered here:**
  - design-CD1.md and design-CD2.md: the header, summary table, commit plan, "Choices made", "Couplings and risks" and "Carried items";
  - every K1 and K6 section: CD1 G10-16; CD2 G10-15, G10-17, G10-22, G7a-3, G7a-6;
  - CD1's rejection G2-80;
  - the parallel-test (`t.Parallel`) findings: CD1 G2-73, G2-79 and G2-134;
  - design-CD2-addendum.md in full;
  - design-coord-items.md items 26–36 with their spot checks.
- **Part one** already reviewed the E1 and E2 addenda. Its findings DA-S1 to DA-S7 and DA-L1 to DA-L8 stand; the amendments are listed below.
- **Checked against the code (grep and read):**
  - table.go's error texts at :62–255;
  - every pin of "literal is not a";
  - every pin of "text operators (S7)", "count (S8)" and "chain (S4.1)", including docs outside sprints and reviews: none there;
  - parseerror_contract_test.go:66–74 (pecEdit and its `//nolint`);
  - the users of the pec* row tables.
- **GitNexus this pass:** no new calls; I used part one's calls and the notes' own impact results. The index is at 1f4ebba, which is HEAD.
- **Not read:** CD1's K2a/K2b/K2d sample sections (G2-65…72, -74…76, -78). Their proof detail is the QA half's.

### Amendments to part one
1. **DA-S6 widens.** The CD notes add a third way to mark a test that cannot run in parallel:
   - CD1's G2-73 and G2-134 use the repo's own convention, `Not parallel: <reason>`, and check for it. CD1 is right.
   - The CD2 addendum's check for G2-134's plan half looks for `Runs serially:`, like E2.
   - So within lane CD, the two halves of one finding are proved by two different check patterns.
   - **Fix:** B, E2 and the CD2 addendum use CD1's marker and CD1's script (design-CD1.md, G2-73 §4) as the one shared check.
2. **The hidden coupling I flagged in part one's verdict is resolved.**
   - E1's `TestAddRefusesStartingRowsItCouldNotApply` pins table texts with `strings.Contains`: "table T: row 0: 1 values for 2 attributes" (table.go:215) and `…: attribute 0 "k": a key value cannot be null` (:219 with :246).
   - CD2's G10-22 choice A keeps every table text byte-identical, and no other CD fix touches checkRow or checkValue.
   - So no W3 change can turn E1's test red. The coupling itself remains; it is worth a line in the record.
3. **DA-L4 (the budget) gets numbers.** The coupling commits are now:
   - G3-72;
   - G2-24;
   - G7a-3 (CD2's C2);
   - `table.Check`.

   That is 4 against the plan's 2. Lane CD itself is under its ceiling (DA-L9).

### New findings

#### DA-L9: lane CD is under its ceiling, not at it (coordinator items 33 and 36)
- **Claim:** the coordinator records CD's line as 14 with a ceiling of 16, and the lane as "16 = at the ceiling, no spare".
- **Evidence:**
  - Plan §1 sets CD's line at 13 (2+2+4+8+2+4+9+13 = 44), so its ceiling is 15.
  - CD1's 7 commits include the bench-input commit for G2-69, which belongs to the bench pool.
  - CD2's 9 include C2, a coordinator coupling commit, and C9, a K2-del in the shared pool.
  - Lane commits proper: CD1's parser-K1, parser-K2, event-K2, clock-K2, text-K2 and model-K2 (6), plus CD2's C1, C3, C4, C5, C6, C7 and C8 (7). That is 13, exactly the line.
- **Why it matters:** the error goes the other way from the usual. CD has 2 commits of slack for a fix round, and the pools and the coupling line carry the rest. The running total still needs the coupling line re-projected (amendment 3).
- **Fix:** count CD as 13 lane commits, plus pools (bench 1, K2-del 1) and coupling (1).

#### DA-L10: in W3, CD's lane and the coordinator both edit registry.go (CD2 G7a-3 C2, coordinator item 35)
- **Claim:** CD's lane edits `internal/expr/registry.go` in C1 (G10-15). The coordinator's C2 then edits the same file (:188), plus expr.go and runnable_test.go, all in CD's tree, while CD's lane is active.
- **Evidence:**
  - Plan §0 item 5 allows a coordinator commit "with the lane's change when both are needed".
  - CD2 orders C2 after C1.
- **Why it matters:** two writers on one file in one wave, unless the commit is assembled from the lane's branch.
- **Fix:**
  - The lane prepares C2's expr hunks on its branch.
  - The coordinator adds notbuilt.json and test_notbuilt.py to that same commit, then runs `--job proofs`.
  - State this sequencing in the lane brief.

### Architecture notes on the CD choices (no finding)
- **G10-16:** using `mk` makes the E-SYN-001 fix text come from the catalogue only: errors.md stays the single source. CD1 warns about `impact lexError`'s CRITICAL rating. The edited lines are two branches that Parse cannot reach, so the warning is justified and handled.
- **G10-15:** refusing at Register is the right layer: a bad descriptor fails once, where it is added. The note warns about the HIGH impact rating; the nine built-ins all pass the new rules (registry.go:106-116, per the note).
- **G10-17, choice B (fixed article):**
  - A local `article()` helper would copy validate's (validate/expects.go:238-243), and importcheck forbids plan → validate.
  - So B is the only option without a second copy of a rule.
  - It is Build-time text reached only from a model the validator has not checked.
- **G10-22, choice A (drop the sentinels, texts unchanged):** sound. No `errors.Is`, `As` or `Unwrap` touches table errors anywhere, and exporting a sentinel with zero users would add a contract. The new `Unwrap == nil` assertion makes "plain errors" a pinned rule. That is acceptable as the note's choice; record it as such.
- **G7a-3:**
  - The pin list is complete by grep: expr.go:249, :304, registry.go:188, runnable_test.go:13 and :22-26, notbuilt.json:17, test_notbuilt.py:20. No user doc outside the sprint and review records quotes the old texts.
  - The two-way substring argument holds, so TestCorpus reds if either side lands alone.
  - plan.go:76 and expr.go:114 still name the old constructs with sprint ids. CD2 leaves them to the later G7 work, which is consistent with the plan.
- **G2-79:**
  - It turns package-level row tables into functions of `t`. The tables have no users outside the two files (grep).
  - It removes an inline `//nolint`, which no allowlist file lists, so no coordinator file changes.
  - It meets §7.2's condition that a refactor changes no outcome.
- **R5:** no CD fix is K1h.
  - G10-15 runs at install time only.
  - G10-16, G10-17, G10-22 and G7a-3 use the named error-text or error-branch exemptions.
  - The plants in per-event code are proof-only.
- **R4:** G2-69 is its own commit, with no benchmark renamed. FUZZ_FILE (G2-78) changes in its body only.

### Coordinator items: part two

| # | View |
|---|---|
| 30 | Agree on the routing. The two halves' checks must match (amendment 1) |
| 31 | Agree |
| 32 | Agree with both rejections. G2-80: a type assertion pins Parse's documented `*Error` contract more strictly than `errors.As` would |
| 33, 36 | **Disagree:** the line is 13 and the ceiling 15; pool and coupling commits are counted separately (DA-L9) |
| 34 | **No discrepancy.** `git grep -c` gives 3 + 5 + 65 = 73 test lines. That is the note's 72 regex matches plus literalvalue_test.go:17, edited by hand (the regex cannot match `<kind>`) |
| 35 | Agree, with the sequencing in DA-L10 |
| 26 | Unchanged from part one: a third agent in W1 breaks ground rule 17 (DA-S2) |

### Verdict, both parts together
CD1, CD2 and the CD2 addendum are sound in design. Their only changes are DA-S6's shared marker for serial tests and the bookkeeping in DA-L9 and DA-L10.

The set is ready for G1 once:
- DA-S1's queue-sizing choice is settled (or the narrowed limit accepted) and put to the owner;
- the note edits DA-S2 to DA-S7 are made;
- amendment 1 is applied.

The owner should see at G1:
- DA-S1;
- DA-L1, G10-1 option B's full cost (three agents, new internal API, a public behaviour change);
- coordinator item 13's recommendation for S3.3-1-3.

Files:
- s17/design-CD1.md
- s17/design-CD2.md
- s17/design-CD2-addendum.md
- s17/design-coord-items.md
- internal/table/table.go
