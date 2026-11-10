# S3.3-2 — architecture review of the combined build

> **Historical — S3.3-2, the expect sandbox, the conformance runner, the Tier-2 baselines and the clock freeze; merged in PR #28.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-08--s33-2s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-3-2-design.md) · [record](../sprints/s3-3-2-record.md) · reviews: [design: breaker](s3-3-2-design-breaker.md), [design: challenger](s3-3-2-design-challenger.md), [design: architecture](s3-3-2-design-architecture.md), [design: QA (testability)](s3-3-2-design-qa.md), **combined build: architecture**, [break-it: QA](s3-3-2-breakit-qa.md). **← [S3.3-1](../sprints/s3-3-1-record.md) · [S1.1c (next)](../sprints/sprint-plan.md#status) →**

*The agent's own write of this report was refused by the harness; the coordinator saved its final message verbatim.*

- **Range reviewed:** `4aa8c7e..1f64dc3` (31 commits, 88 files, +6218/−141), read-only checkout `/Users/ramith/code/cep-wt/s3-3-2-panel`.
- **Scratch copy:** `git archive 1f64dc3` into `/private/tmp/claude-501/-Users-ramith-code-cep/14b3d567-7a0b-4919-ab4a-e8203f9bb87a/scratchpad/review/arch/src`. Pinned Go go1.27.3, `GOTOOLCHAIN=local`. Every exit code was read from a file.

| Scratch run | What it shows | Exit |
|---|---|---|
| probe1 (module root): `given` on a derived stream | Unreachable: E-GRAPH-004 says v0.1 rules read source streams only, so X7 already refuses it | 0 |
| probe2 (module root): a source stream with a required `object` attribute | No block can be written; E-EXP-002 in all three forms (R-S2) | 0 |
| probe3 (`internal/runtime`, a scratch-only hook `zzActivateTail` between Activate's reply and its tail, `-race -count=3`) | Activate returns nil after Close has already returned `context canceled` (R-S3) | 1 (3 of 3 failed, the intended red) |
| `go test -race -count=30 -cpu 1,4 -run 'Close\|Inflight' ./internal/runtime` | The in-flight gate holds under stress | 0 |
| `go run ./tools/archcheck` on the row's scope | Import law and purity hold; `GATE_COVERED 228` | 0 |

**Code graph:** `detect_changes` (against 4aa8c7e) reports 259 changed symbols, 451 affected, risk `critical`; the index was 30 commits behind HEAD for `context`/`impact`, so callers were found with grep and confirmed in the source. **Memory:** none used.

**Counts: 0 blockers, 3 should-fix, 5 later.**

## Should-fix

### R-S1 — ADR-0005 §4 and ADR-0015's `todocheck` row describe a not-built corpus file as a skip that is never green and fails T4; the build does something else
- `ADR-0005-playback-first-gate-and-conformance.md:13`: "a case for a construct that does not exist yet is `skip: not-implemented(S<n>)` — recorded, never green, and failing T4 once its sprint closes (ADR-0015 §2 `todocheck`)"; ADR-0015 line 107 (`todocheck`) still names "a conformance `skip: not-implemented(S<closed>)`". As built, `TestCorpus` passes a not-built file with `t.Logf("not built until …")`, marked by `errors.ErrUnsupported`; the expiry moves to S1.1c (`decisions.md`, 2026-11-09, decisions 1 and 6). The sprint changed nothing under `docs/architecture/`.
- **Fix:** annotate ADR-0005 §4 in place: *(Annotated 2026-11-10, S3.3-2: a not-built file is refused with an error wrapping `errors.ErrUnsupported` and passes `TestCorpus` with a log line naming its construct and sprint; never a `t.Skip`. Its expiry is built in S1.1c — owner 2026-11-09, decisions 1 and 6.)*; the same pointer on ADR-0015's `todocheck` row; optionally ADR-0012 §3: once `Close` begins, `Add` and `Activate` are refused with `ErrClosed` (design §9.1).
- **Test:** none (docs; `doclinks` covers the links).

### R-S2 — a rule over a stream or table with a required `object` attribute cannot carry any expect block, and E-EXP-002's fix text cannot be followed
- No literal fits `object` (`internal/validate/resolve.go:56-72`); leaving it out or `null` fails X4 (`internal/validate/expects.go:96-110`); only declaring it `optional` gets out, which the Fix text does not say. SPEC §6.8: "rules shipped in packs must carry one". probe2 (stream `A (k string, p object, ts timestamp)`): `p` left out → `A.p is required; the block leaves it out`; `p: '{"x":1}'` → `A.p is object; the block gives a string value`; `p: null` → `A.p is required; the block gives it null`.
- A question, not a lane defect: X4 follows the owner's decision 2 of 2026-11-09; the panel did not raise `object`. Options, most accurate first: (a) an object literal in blocks (its JSON text — a language change); (b) X4 exempts a given `object` attribute (left out or `null` reads null, as SPEC §6.8's "omitted is null" says); (c) keep the refusal and say what to do.
- **Recommendation:** (c) now — for an `object` field the X3/X4 message ends `; a block cannot write an object: declare <S>.<a> optional`; forward (a) or (b) to the owner. **Test:** a `TestExpectMistakeMessages` row for a required `object` attribute left out, its message naming `optional`.

### R-S3 — Activate's tail can run after Close returned at its deadline
- Close step 1 (`select { case <-r.inflight.close(): case <-ctx.Done(): first = ctx.Err() }`) gives up on an Activate in flight when ctx ends, then `close(r.stopped)` and the `r.wg.Wait()` goroutine. If the router's reply was already sent, Activate's tail still runs `r.wg.Add(1); go q.run(r)` per flusher and the recorder, sets the phase and returns nil — through the facade, `Start` returns nil on a closed runtime. The late flushers exit at once (`sinkQ.run` checks `r.stopping()`), but `wg.Add` from zero after a `Wait` with nothing ordering them is `sync.WaitGroup`'s documented reuse misuse (a panic in a narrow window; not forced). Design §9.1 accepts goroutines left running on the deadline path, not a call that returns success after Close.
- **Evidence:** probe3: `Close … context canceled`, then `Activate after Close returned: <nil>`; 3 of 3, exit 1.
- **Fix:** make starting goroutines and stopping mutually exclusive — in `Close`, `r.inflight.mu.Lock(); close(r.stopped); r.inflight.mu.Unlock()`; in Activate's tail, before the first `r.wg.Add`, `r.inflight.mu.Lock(); if r.isClosed() { r.inflight.mu.Unlock(); return ErrClosed }`, start the goroutines, set the phase, unlock. The same for `Run`'s goroutines (outside the gate; Start's third step can race Close's drain the same way).
- **Test:** `TestActivateAfterCloseGaveUpIsErrClosed` (`close_gate_test.go`) with a package-test hook `activateHold func()` between the reply and the tail (the `addHold` precedent): Close with a cancelled ctx, release; Activate returns `ErrClosed`, no flusher started; `-race -count=50 -cpu 1,4`.

## Later

### R-L1 — the determinism rows no longer prove both test names ran
`determinism` and `determinism-macos` run `-run '^(TestDeterminismGolden|TestBaseline)$'`, `min_wants 1`; since b7edbef the count is packages, so a typo in one name leaves the macOS row green without running the baseline there ("no test ran" fires only when nothing matches). No existing row changes verdict (`tripwire` covers 1 test in 1 package, then and now). **Fix:** for an anchored alternation `^(A|B|…)$`, `gotest.py` reports `no test ran: <name>` for each name with no top-level `run` event. **Test:** a pytest feeding a synthetic `go test -json` stream where only `TestDeterminismGolden` runs.

### R-L2 — a Deploy during Close does all its work before being refused
`deploy.go:25` checks `r.rt.Closed()`, which reads only `r.stopped`, closed after Close's walk: a Deploy during the walk parses, validates, plans, runs every block and calls `bind` (the factories) before `Add` refuses it with `ErrClosed`. Correct result, wasted work. **Fix:** `Closed()` is also true once the gate is closing. **Test:** a counting factory, a Deploy while Close is held in its walk: `ErrClosed`, zero factory calls.

### R-L3 — a Start overlapping the start of Close returns the wrong error
`apphandle.go:42-62`: Activate is inside the gate, `TableWrite` and `Run` are not; if Close begins after Activate left the gate, Start's next step is refused with `ErrDrained`/`ErrRemoved` → `cep.ErrStopped`/`ErrUndeployed`. **Fix:** in `Start`, a step after Activate that fails while the runtime is closing returns `ErrClosed`. **Test:** a facade test holding Start between Activate and its table write (the R-S3 hook).

### R-L4 — a divergence text prints an out-of-range timestamp as `0`
`canon` (`internal/playback/expect.go`) falls back to `fmt.Sprintf("%v", v.F)` for any value `AppendValue` refuses; for a timestamp outside 0000–9999 that prints the unused float `0`. **Fix:** the `%v` fallback for `TFloat` only; a timestamp prints `<ms> ms (outside the years 0000-9999)`. **Test:** a `TestDivergenceTexts` row with a body emitting `ts = math.MaxInt64`.

### R-L5 — SPEC wording and line endings
(a) SPEC §6.8 rev 10 says E-EXP-001 "points at the expected record that did not come out, or at `then`"; the code also points at `expect` when the rule panicked (design §5.4 case 1). Fix: add it. (b) No `.gitattributes`: the baseline header's `source_sha256` hashes the corpus bytes, so a CRLF checkout turns `TestBaseline` red (CI runs Linux and macOS only). Fix: `*.cep text eol=lf`, `conformance/baseline/*.jsonl text eol=lf`; a gate check on `git check-attr eol`.

## Checked and holding
- **The in-flight gate (§9.1):** `enter` and `close` share one mutex; `leave` is deferred past Add's reply and Activate's tail; Close drains by `a.started`; a second `Close` is unchanged; joined Remove errors keep `errors.Is` through the facade's `mapErr` (`ErrRemoved` filtered; `ErrClosed` impossible in the walk); no new deadlock (with a deadline Close returns at it; with `context.Background()` behind a blocked sink it hangs, as before). The seams `removeFault`, `addHold` are written only by `close_gate_test.go`, read in `Remove` and `router.add`, set before the call they affect and ordered by the intake channel: acceptable as documented test-only fields.
- **The sandbox (§5):** a fresh `plan.App` per block, the owning rule alone; tables first through `Sync.TableWrite`, errors returned; lists merged by head `Ts`, ties to the earlier list, written order kept; `End()` to +∞; `Late` summed over the given streams, the rest from `Rules[0][0]`; §5.3's equality incl. −0.0; the alignment as §5.4 (its pairing valid for any match relation, so "nearest" is never empty while outputs remain); positions at the then-record `{`, `then`, `expect`; deterministic (one goroutine, its one map for membership); nothing live (the sandbox runtime is built, never started; no sinks, no recorder); archcheck exit 0.
- **Deploy (§6):** the stages in order (parse, overrides, validate; `plan.Build` with `ErrNotBuilt` beside `errors.ErrUnsupported`; `RunExpects`, one E-EXP-001 per failing block; the `ctx.Err()` check; `bind`, `Add`); L2 holds (`nomodel_test` passes).
- **E-EXP-002:** X1–X10 at §3.1's positions; a section failing X1, X7, X8 or X9 not checked further; `readsTable` walks every expression slot; a derived stream cannot be given (probe1); E-TIME-003's second reason after the type check; SPEC rev 10 matches the code but R-L5(a).
- **The gate:** rows `conformance`, `baseline`; `regen.py` runs Tier 1 first, writes nothing on any refusal, re-runs itself under the pinned CPython (`parity` in `TIER1_ROWS` is the runner's per-job check: harmless); baseline bytes stable across OSes (Go's shortest float text, UTC timestamps, positions out of the key); `proofs` min_wants 21 = the 21 drivers; `apipartition`'s `funcIdent` strips `*`, type parameters and parentheses; both goldens change 74 lines each way, format only.
- **Coupling:** `plan.Rule.Target` read only by the sandbox, set from the compiled body's target; `plan.App.Expects` not carried by `FromPlan`; `runtime.NewSync`'s only caller is `playback.newDriver` (offline replay and the sandbox), never a live path.
