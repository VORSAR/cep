# S1.1c-6 combined build: blind architecture review (with the-fool)

> **Historical — S1.1c-6, the strict lint set: the size limits in runtime, validate, event, parser, plan and playback; merged in PR #49.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-6-plan-check.md) · [design](../sprints/s1-1c-6-design.md) · [record](../sprints/s1-1c-6-record.md) · reviews: [spike](s1-1c-6-spike.md), [plan check architecture](s1-1c-6-plan-check-architecture.md), [plan check qa](s1-1c-6-plan-check-qa.md), [design architecture](s1-1c-6-design-architecture.md), [design qa](s1-1c-6-design-qa.md), [event walkers](s1-1c-6-event-walkers-review.md), [decoder value](s1-1c-6-decoder-value-review.md), [playback](s1-1c-6-playback-review.md), [combined architecture](s1-1c-6-combined-architecture.md), [breakit qa](s1-1c-6-breakit-qa.md). **← [S1.1c-5](../sprints/s1-1c-5-record.md) · [S1.1c-7 (next)](../sprints/sprint-plan.md#status) →**

Branch `s1-1c-6/integration` @ 0f533a0, base `main` 4d2b9eb (the merge-base): 93 commits, 154 files, +21570/−4660. I only read it. My experiments ran in the scratch folder `g3-arch/`:
- detached worktrees `wt` (0f533a0) and `base` (4d2b9eb), both removed at the end;
- `tools/decls`, a go/ast lister of every package-level declaration, and `tools/nonmoved.py`, which shows a commit's lines that `git --color-moved` does not mark as moved;
- the logs.

- **GitNexus:** this session has no tool for reading the `gitnexus://repo/cep/context` resource. The index does hold symbols that commits 7565764 and af326dd added (`seedGivenTables`, `lcsLength`, `tableRow`, `recordKind`, `appendString`), and the two commits after those change no Go. So I treat the index as current for the Go code.
  - `detect_changes` (compare main): 1932 changed symbols, 155 files, 175 affected, risk "critical". Most of that is moves.
  - `check`: 0 import cycles.
  - `impact` upstream:
    - `LitValue`: 2 direct callers, `validate/resolve.go:75` and `plan/expect.go:236`.
    - `(*router).broadcast`: 3 direct callers, `tickApp`, `(*Sync).End` and `drainApp`. I read each one.
- **Hindsight:** this session has no Hindsight tools, so I searched no memory bank. The decisions I cite are in `docs/sprints/decisions.md` at 0f533a0.

## Findings

No blockers. I compared every split on the hot path and in the lifecycle that the brief names against its original, and each one holds (see "Checked and held").

### CA-S1 (should-fix): the expr → model fence now lets expr call the two new model functions

**Claim.** The owner's 2026-11-04 fence says "expr reads `model.Expr` only" (`decisions.md:286`). G1 decision 1 (`decisions.md:1131`) kept the literal merge out of expr so that this fence would stay "untouched".
- The fence is enforced by name prefix: `tools/importcheck/rules.json:34` allows expr `"Expr", "Lit", "TextOperand"` from `internal/model`, and `tools/archcheck/main.go:519` checks `strings.HasPrefix`.
- The rules note says "Lit covers Literal and the LitX constants". The sprint added two functions with that prefix, `model.LitValue` and `model.LitType` (`internal/model/literal.go:7, :29`).
- So the gate now accepts expr calling them. The rules file did not change, but in effect the fence got wider.

**Evidence.**
- Planted check in my worktree. I appended two functions to `internal/expr/nodes.go`: one calling `model.LitValue`, and a control using `model.BodyKind` and `model.BodyPlain`.
- `go build ./internal/expr` passed, and `go run ./tools/archcheck <nul-list nodes.go>` printed only:
  - `nodes.go:716: fence: internal/expr may use model.Expr*, model.Lit*, model.TextOperand* only … not model.BodyKind`
  - the same line for `model.BodyPlain`
- It printed nothing for the `model.LitValue` call at `:713`. Exit 1, from the control alone. I then reverted the file (`git checkout`).
- The design saw this mechanism as DA-B1 (design line 524: "passes the checker only by its `Lit` name prefix"). It narrowed the merge, but it never closed the gap.

**Consequence.** Nothing in the gate stops a later change from crossing a recorded owner fence. This is the-fool's most likely first failure below: S1.1c-7 cleans `internal/expr` (`smells.json`: file length, funlen and gocyclo, issue S1.1c-5-3), and `literalNode` (`internal/expr/nodes.go:20`) duplicates `LitValue`.

**Fix area.** `tools/importcheck/rules.json` (a law path; the owner decides). Replace the `"Lit"` prefix with `"Literal"` plus the exact `LitX` constant names that expr uses (`LitString`, `LitInt`, … at `nodes.go:22-30`), or exclude function identifiers. Add an archcheck test that plants `model.LitValue` in expr and shows it red.

### CA-S2 (should-fix): combine's planned doc rows and issue closures are not done at 0f533a0

**Claim.** The design's Combine row plans "S1.1c-5-1 and -2 closed when their last lane is in (2); the doc rows (§4, §6) (1)" (design line 29). Plan check §4 adds "ADR-0010 §3's `decode.go` if they move (PA-S11)" and the S3.3-1-3 pointer (plan check line 102). The design adds "the coordinator annotates the ADR at combine" (line 576). None of these has landed.

**Evidence.**
- `docs/architecture/adr/ADR-0010-performance-model.md:12` still calls `internal/event/decode.go` "a single-pass JSON scanner". The scanner is now in `internal/event/scanner.go`. `git diff 4d2b9eb 0f533a0 -- docs/architecture` is empty.
- `docs/product/known-issues.md:9` (S3.3-1-3) still points at "`internal/runtime/runtime.go` `iBatchEnd`".
  - `runtime.go` now holds only types (193 lines).
  - The branch is `internal/runtime/router_in.go:13` (`ingestBatchEnd`, `:44`), and the constant is at `messages.go:23`.
- `gate/known-issues.json:54-68`: S1.1c-5-1 and S1.1c-5-2 are still open with `"forward": "S1.1c-6"`. `grep -c 'S1.1c-5-1\|S1.1c-5-2' tools/allowlists/smells.json` gives 0: 91ff0d2 removed the last entries that named them.

**Consequence.**
- The docs mislead (ground rule 22).
- Anyone fixing S3.3-1-3 at S8 is sent to a file that no longer has the branch.
- At G5 the two issues read as open but forwarded to a closed sprint.

**Fix area.** The combine commits: the coordinator's files (`known-issues.json`, `known-issues.md`, the ADR annotation).

### CA-S3 (should-fix): the 100-commit cap cannot hold the planned rest without rewriting the pushed branch

**Claim.** The branch already holds 93 commits (`git rev-list --count 4d2b9eb..0f533a0`). The design's remaining items at their own maxima come to 14 (design line 32):

| Item | Commits |
|---|---|
| Two issue closures | 2 |
| Doc rows | 1 |
| Fix round | ≤ 8 |
| Record, T4 and G5 docs | 3 |

That makes up to 107, against ground rule 19's cap of 100. With no fixes at all the total is 99, which leaves one commit for the whole fix round. The design's own line 32 ("At most 98 of 100") is now wrong.

**Evidence.**
- The squash rule (design line 32) is written for lane branches *before* cherry-pick. Every lane is already on the integration branch, and `origin` has that one `s1-1c-6` head.
- Squashing now means rewriting the pushed branch. That has happened once before: `s1-1c-6/integration-unsquashed` exists.
- A rewrite changes the hashes the record cites as per-commit evidence. Record lines 70, 77 and 78 name 6f7a85a, f4ce433, 696da7f, 1c6fb06, 6ab818d, 9420a28, be08f5e, 91960f8 and others.

**Consequence.** Either the fix round is squeezed into one commit, or a late rewrite breaks the evidence trail (see CA-L1). Both are decisions for the owner.

**Fix area.** Before the fix round, the owner decides:
- which commits to squash (for example validate's ten non-hot split commits, which carry no per-commit measurement), and whether the record's citations are re-pointed in the same pass; or
- folding the closures, the doc rows and the docs commit into one.

### CA-L1 (later): evidence cited at commits that are not on the integration branch

**Claim.** The baselines behind four owner exceptions are cited at commits that exist only on local scratch branches. Those exceptions are O1, the `\u` variant, V1 and T1.

**Evidence.** `git merge-base --is-ancestor <h> 0f533a0` fails, and `git branch -a --contains` lists only local branches, for:
- `internal/event/scanner_reference_test.go:6` and `object_reference_test.go:6`: 2b86876 (`s1-1c-6/e-pre-fold`);
- `value_matches_reference_test.go:13`: 4d85258;
- `rfc3339_reference_test.go:6`: 22af094 (on `integration-unsquashed` and others);
- `decisions.md:1165`: c54fe6c;
- `decisions.md:1192-1193`: 24da7fb, 4d85258;
- `decisions.md:1204`: 880541c, dd6bb8d, c61d7ef (`s1-1c-6/p-pre-13a`);
- record lines 70 and 77: 53fb5f0, 55ab33d.

Older pages do the same (`decisions.md` cites c6deb91 and others), so the pattern predates this sprint.

**Consequence.** Merged branches are routinely deleted afterwards. Once these are gone, nobody can re-run the "before" side of the three rule-13a reviews.

**Fix area.** Archive refs for the cited baselines (pushing them is the owner's call), or cite commits that stay reachable.

### CA-L2 (later): no check protects the inlining that the redesigns' speed depends on

**Claim.** With `go build -gcflags=-m=2` at 0f533a0:

| Helper | Location | Inline cost |
|---|---|---|
| `appendObject` | `playback/encode.go:71` | 78 of budget 80 (V1's premise) |
| `isHex4` | `event/scanner.go:257` | 53 |
| `(*recQ).drainQueue` | `runtime/recorder.go:182` | 73 |
| `skipSpace` | `event/object.go:115` | 16 |
| `nonFinite` | `runtime/boundary.go:228` | 45 |

No test or gate row asserts any of these. `grep -rn 'can inline'` over `internal`, `gate/runner` and `gate/bench` finds only `escapes.py`. The pinned toolchain is go1.27.4 (`go.mod:5`), and known issue S1.1c-5-4 is about moving the toolchain.

**Consequence.** If a toolchain update changes the inliner budget, `appendObject` can stop inlining with no warning. That is the shape measured at +44.9 % on `BenchmarkAppendValue/object` (`decisions.md:1204`), and nothing fails.

**Fix area.** A cheap T2 test that builds the three packages with `-gcflags=-m` and asserts `can inline` for the named helpers. Alternatively, list them in ADR-0010 as the hot helpers that must inline.

### CA-L3 (later): the permanent differential tests more than double event's race time

**Evidence.** These are my runs on 14 cores.
- `go test -race -count=1 ./internal/event` took 28.8 s at 4d2b9eb and 64.1 s at 0f533a0.
- The race times per test were:

| Test | Race time |
|---|---|
| `TestScannerMatchesTheReference` | 44.1 s |
| `TestWalkersMatchTheReference` | 19.6 s |
| `TestValueMatchesTheReferenceOnRandomCases` | 8.3 s |

- Without race they took 5.7 s, 3.4 s and 1.1 s.
- The race row's cap is 900 s for `./...` (`gate/registry.json:817`), and CI runners have fewer cores.

**Consequence.** On CI, event may become the slowest package under race. It does not fail today.

**Fix area.** Read the race row's CI wall time at T4. If it is tight, scale the 1,000,000-input loop by `testing.Short()` or move it to the nightly job.

### CA-L4 (later): `gorun.run`'s last group kill is outside any `finally`

**Claim.** The run ends with `_killpg(proc.pid, signal.SIGKILL)` at `gate/bench/gorun.py:93`, after the outer `try` (`:73-91`).
- A stop that lands after that `try` ends but before the kill skips the kill of any grandchild left behind by a leader that exited.
- `support.py`'s `run_process`, the shape the fix commit cites, kills in a `finally:` (`gate/proofs/drivers/support.py:171`).

**Consequence.** The window is a few bytecodes, it is older than 11befad, and `go test` normally reaps its own test binary. A theoretical leak only.

**Fix area.** Move the final kill into a `finally` that runs when `proc is not None`.

## Checked and held

- **Behaviour of the hot path and the lifecycle**, read side by side, original against split, at each split commit's parent and the commit:
  - **Runtime:**
    - `ingest`, with `inputApp`, `ingestBatchEnd`, `ingestEvent`, `ingestReject`, `tickApps`, `writeTable`, `statsCut` and `ingestLifecycle`/`activate`/`beginDrain`. `&it.ev` and the record put use the same local copy, as before.
    - `take`, with `toSinks`, `ack`, `drainAck`, `statsAck` and `completeStats`. Each `continue` is now a `return` at the same point.
    - `(*shard).process`. The defer and the three control returns stay. The helpers re-read `sh.rules[app]` and `sh.arrival[app]`, but those are written only in `grow` and `install`/`uninstall` (`shard.go:35`, the mAdd and mRemove branches), so the re-reads are equivalent.
    - `route` and `sendHead`. Each 3-arm `select` moved whole, and `rt.r` is `route`'s `r` (`router.go:55-58`, `lifecycle.go:31`).
    - `broadcast(m message)`. It sets `admStream = -1` itself, its three callers build the same fields, and `adm` is 0 as before.
    - `Send`/`checkEvent`/`checkValues`/`checkValue`. Every text is byte-identical, `nonFinite` equals the old case, and the check order is the same.
    - `SendPayload`/`asDecodeError`/`recordedFault`.
    - `(*recQ).run`/`drainQueue`. One 4-arm `select`; the three defers in place.
    - `Activate`/`activationRefusal`/`startAppGoroutines`. Lock and deferred Unlock are in one function, and the Unlock still runs before Activate's deferred `leave`.
    - `Drain`/`markDraining`/`revertDrain`/`queueDrain`/`awaitMarkers`/`awaitSignal`/`awaitRecordingStop`. `Swap` comes before the enqueue, and the returns of `ErrClosed` and `ctx.Err()` are as before.
    - `awaitRuns`/`runReturn`.
    - `installAck`/`completeRemove`/`rollBackAdd`. The order of `close(a.rec.stop)`, the phase store and the reply is unchanged.
    - `Remove`/`notifyRemoved`.
    - `Close`/`shutDown`/`closeApps`/`closeApp`/`stopRuntime`/`awaitGoroutines`. `closeApp`'s returns end the loop exactly where `break` and `first = err` did.
    - `build`/`withDefaults` (the one reordering, G1 decision 6) and `validateApp` with its six checkers. The first-error order and texts are the same.
  - **Event:**
    - `appendUnescaped` with its three helpers.
    - `NewSchema`/`checkAttrs`/`checkTimeAttr`.
  - **Playback:**
    - `AppendValue` (V1: field-typed helpers, `objectError` out of line, byte-identical) and `(*Recorder).Table`/`tableRows` (T1: `"op":"` + opName + `"`, equal to `appendString("put"|"delete")`).
    - `record`/`envelope`/`recordKind`.
    - `eventRecord`, `rejectedRecord` and `tableRecord` with their helpers. `fault` returns `*LoadError`, every helper returns `error` with a literal `nil`, so there is no typed-nil hazard.
    - `diverge`, with the claim that `of[j]+1` became `i+1` verified.
    - `runBlock`. `d.Stats()` now runs after the `res` literal, but `(*Sync).Stats` is a pure read, so the change cannot be observed.
  - **Plan:** `(*body).OnEvent`/`passes`/`project` (both defers in their order), `expects`/`givens`/`thens` and `rule`/`key`/`keyAttrs`/`newBody` (`spec` is captured by copy after its last write).
  - **Parser:** `parseError` and its `failSite` helpers. The first-match order is preserved; the `->` guard falls through as the old cases did.
  - **Validate:** spot check of `selects`. `binding`'s pre-formatted text through `"%s"` matches `report`'s `Sprintf` (`validate.go:130-134`).
- **The redesigns' differential tests compare against verbatim copies of the code before the sprint.**
  - `valueReference` equals 4d2b9eb's `(*Decoder).value` byte for byte.
  - `cleanObjectReference`, `cleanLenReference` and `compactAndValidReference` equal the base functions, apart from the renamed callee.
  - `parseRFC3339Reference` and its helpers equal the base, apart from one comment.
  - `scannerReference` is the scanner *after* the E2 splits. I diffed it against base's scanner: the only differences are `at`/`atEither`, `member` (old inline code plus `colon`'s body, verbatim), `exponent` and `isDigit`, all equivalent. So the chain from base to the reference (by reading) to head (by the differential) is closed.
  - The scanner test's alphabet includes a control byte, `\xff` and a cut `\u`.
- **Exported surface and package state.** The declaration lister found 1452 declarations at base and 1696 at head.
  - New exported identifiers: only `model.LitValue` and `model.LitType` (G1 decision 1).
  - Changed or removed:
    - `(*Decoder).value`: results named, same types;
    - `(*scanner).colon`: removed (`decisions.md:1173`);
    - `digitsAt`: removed (the `parseRFC3339` re-split);
    - `plan.expect`: dissolved;
    - `broadcast`: now takes one message (`decisions.md:1156`).
  - No exported signature changed.
  - New package vars: event's `hexDigit`, `jsonSpace`, `strByte` and `strPlain`. Every use is a read, and they were decided at `decisions.md:1172, 1173, 1190`.
- **Layering.**
  - `go list` production imports, base against head: no difference. No new import edge.
  - New test-only imports (plan's internal tests use `internal/partition`, validate's use `internal/event` and `internal/expr`) are inside each package's production allowance in `rules.json`.
  - GitNexus `check`: 0 cycles.
- **Escapes, as a backstop.** The `moved to heap` lines in runtime, event, playback and plan are the same as before, except `plan.go`'s `spec`. Build time only: the closure capture moved from `rule`'s local into `newBody`'s parameter, one allocation per rule either way.
  - Package `escapes to heap` counts: runtime 384 → 350, event 81 → 82, playback 323 → 331, plan 90 → 93.
  - Playback's +8 is the inlined `appendEscape`'s seven appends counted at its inline site `encode.go:106` (the same appends as the old inline switch) and T1's op append (`decisions.md:1209`).
- **Waivers.**
  - One new PERMANENT waiver, `internal/runtime/keys.go:16` (`routeKey` kept whole). Allowed by design §2.4 step 5; "routeKey's restore stands" (`decisions.md:1174`).
  - `smells.json` only loses entries, plus the S1.1c-5-3 reason edits.
  - No refactor commit edits an existing test file, except ad69ff2 (the owner's E-GRAPH-001 fix). The later test commits all add new files, each an owner exception, except c1b4398 and 374133a, which change one file each (also decided).
- **Placement.**
  - `(*Decoder).Decode` stays in `decode.go:79` and `Parse` in `parse.go:59`.
  - No production file in the six packages passes 500 lines.
  - The design's 2026-11-18/19 annotations are present at lines 71, 668, 697, 718-719, 907, 976 and 995.
- **The gate fix.**
  - In `gorun.run`, `term.critical()` (`:75`) now sits inside the outer `try` whose `except BaseException` stops the group (`:88-91`). A TERM held during start-up is raised as the block ends, inside that `try`, and `_stop` runs only when `proc` exists.
  - `term` raises Terminated once and ignores later TERMs (`gate/term.py`), so a second TERM cannot cut `_stop` short. All four bench tools call `term.install()`.
  - `test_gorun`'s limit case blocks SIGTERM around `gorun.run`. The child inherits the mask through fork and exec (CPython skips its posix_spawn path when `cwd` or `start_new_session` is set) and unblocks only after writing both pids. The witness is `exit == -SIGTERM`, and the 20 s grace is a cap, not a race.
  - My run: `python3.15 -B -m unittest gate.bench.tests.test_gorun`, 8 tests OK.
- **Runs on 0f533a0, in my worktree:**
  - `go test -count=1 -timeout 600s ./internal/...`: 13 packages ok, exit 0.
  - `gate/bootstrap.py exec -- --tier T2 --job lint`: GATE PASSED, exit 0. All 17 rows passed, including `smells` (359 covered), `archcheck`, `suppressions` and `docsync`.
  - Not run: the T2 test job (only partly, through `go test` and `test_gorun`), T3, T4 and the path comparison against `main`.

## The-fool: pre-mortem

**Steelmanned thesis.** S1.1c-6 brings the six packages of S1.1c-5-1/-2 to 0 size findings by moving statements, not by changing behaviour. This is proven by:
- nets frozen before each split;
- mutations that count only a named failure;
- `astequal` on the pure moves;
- differential tests against verbatim copies wherever a redesign replaced a split.

Performance stays within 2 % per split and 5 % per path, with three owner-chosen redesigns and two bounded exceptions (O1's leaf cost and V1's residual). My review found no statement whose meaning changed, no reordered check that can be observed, and no change in concurrency. The thesis holds for the code.

**It merged; three weeks later it broke. The most likely causes, ranked:**

1. **(Most likely first) The expr fence is crossed with the gate green (CA-S1).**
   - S1.1c-7's job is cleaning `internal/expr` (`nodes.go` is 710 lines). `literalNode` (`nodes.go:20`) duplicates `model.LitValue` case for case.
   - A splitting agent "deduplicates" by calling `model.LitValue`. archcheck passes it (planted, above), and the owner's 2026-11-04 decision is undone without a question.
   - *Early sign:* an expr diff that adds `model.Lit` followed by an upper-case non-constant name.
   - *Mitigation:* tighten the fence now, in this sprint's fix round.
2. **The throughput ratchet.**
   - The lane path `BenchmarkSendPath/shards=1/recorded` read +3.9, +1.2, +5.0, +6.0 and +3.1 % across sessions (record line 78). It was accepted each time, because rule 13a needs a confirmed re-run.
   - S1.1c-7 splits partition and expr, which are on the same path, and measures against post-S1.1c-6 `main`. The 5 % margin starts again from zero each sprint.
   - So ~8–10 % against 4d2b9eb can arrive with every gate green. ADR-0010's stored-baseline band (20 % advisory, `ADR-0010:19`) is the only absolute check.
   - *Early sign:* T4's `main` against combined path reads 4–6 %.
   - *Mitigation:* keep 4d2b9eb's test binary as a fixed reference for S1.1c-7's path comparison, or let the owner set a budget that spans both sprints.
3. **A toolchain bump un-inlines a redesign (CA-L2).**
   - `appendObject` sits at 78 of 80. The next Go release, or S1.1c-5-4's toolchain move, can tip it, and AppendValue/object goes back toward +45 % silently.
   - *Mitigation:* the inline-assert test.
4. **The frozen references become a trap (CA-L3).**
   - The next real decoder or scanner fix reds `TestScannerMatchesTheReference`. The developer then edits the "verbatim" copy, which leaves the test proving nothing, or deletes it.
   - Meanwhile event's race time has gone from 28.8 s to 64.1 s.
   - *Mitigation:* put a retirement rule in each reference file's header ("a deliberate behaviour change replaces this file in its own commit, owner-approved"), and gate the 1M loop by `-short`.
5. **The evidence trail breaks at close-out (CA-S3, CA-L1).**
   - Fitting the fix round under 100 commits rewrites the pushed branch, so the record's per-commit verdict hashes dangle, as the lane-branch baselines already do.

**Inversion check.** What would make this merge safe three weeks out?
- the fence tightened;
- an absolute path baseline;
- an inline assertion;
- archive refs for the cited baselines;
- the combine doc rows done (CA-S2).

None of these touches engine code.
