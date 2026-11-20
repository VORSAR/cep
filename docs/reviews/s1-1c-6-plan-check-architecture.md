# S1.1c-6 plan check — architecture review (blind, with the-fool's pre-mortem)

> **Historical — S1.1c-6, the strict lint set: the size limits in runtime, validate, event, parser, plan and playback; merged in PR #49.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-6-plan-check.md) · [design](../sprints/s1-1c-6-design.md) · [record](../sprints/s1-1c-6-record.md) · reviews: [spike](s1-1c-6-spike.md), [plan check architecture](s1-1c-6-plan-check-architecture.md), [plan check qa](s1-1c-6-plan-check-qa.md), [design architecture](s1-1c-6-design-architecture.md), [design qa](s1-1c-6-design-qa.md), [event walkers](s1-1c-6-event-walkers-review.md), [decoder value](s1-1c-6-decoder-value-review.md), [playback](s1-1c-6-playback-review.md), [combined architecture](s1-1c-6-combined-architecture.md), [breakit qa](s1-1c-6-breakit-qa.md). **← [S1.1c-5](../sprints/s1-1c-5-record.md) · [S1.1c-7 (next)](../sprints/sprint-plan.md#status) →**

*Of [the plan check](../sprints/s1-1c-6-plan-check.md) v1. Filed by the coordinator from the reviewer's final message, unchanged in substance. Every finding is placed in the plan check's §10.*

**Base:** `main` @ 4d2b9eb (the working tree matches it apart from docs). **Read:** the draft v1; `docs/ground-rules.md`; `docs/sprints/00-sprint-model.md` (§3, §5, §6, §10); the decisions.md sections the brief named (S1.1c-5's plan check 1, 2 and 6; S1.1c-5's close-out 1 and 2; S1.1c-6's scope 1 and 2); S1.1c-5 design §7.3–7.5; the S1.1c-5 record (lane D's numbers); `gate/runner/parts.py`, `gate/runner/aging.py`, `gate/runner/rows/aging.py`, `gate/runner/rows/smells.py`; `tools/allowlists/*.json`; `gate/law.json`; `gate/registry.json`; `gate/sprints.json`; `gate/known-issues.json`; `docs/product/known-issues.md`; `gate/runner/tests/test_aging.py:520-560`; `tools/importcheck/rules.json`; the proof drivers under `gate/proofs/drivers/`; the code at the places cited below.

**Tools:**
- **GitNexus:** Cypher for the `validateApp` definitions and for the callers of `CheckRows`, `CleanLen`, `validateApp`, `rowBound` and `lineBound`; its line numbers match 4d2b9eb.
- **Size scan:** a throwaway `go/ast` scanner in the scratchpad (cyclomatic count as gocyclo counts it, lines, parameters): 111 functions over the limits in the 12 packages, which agrees with the draft's 110 within the scanner's line-count margin. golangci-lint was not run.

## Findings

### Blockers

**PA-B1 — The plan has no proof for an approved cross-package move. Exit criterion 2 ("every move commit `astequal` exit 0") cannot hold for one.**
- `astequal` treats "a declaration that changes package" as unequal, and its own test `TestMovedToAnotherPackageUnequal` pins that (S1.1c-5 design §7.4 and §8 7.2). A move across packages also usually exports an unexported name and rewrites callers, which changes their AST too.
- The import rules are exhaustive and are law: "ARCHITECTURE §3's import rules, exhaustive (owner 2026-11-02)", `tools/importcheck/rules.json:4`. `internal/validate` may import only event, model, expr, window and metrics; `internal/runtime` may not import validate or playback; playback imports runtime. A move that needs a new import edge is an architecture change, and §2's coordinator files do not include `rules.json`.
- §2's "seam … with an owner lane and a stub" does not work: lanes branch from the integration branch and are cherry-picked only at combine, so a move from lane R's package into lane E's touches files of two lanes in two waves.
- **Fix:** every approved move is one coordinator-owned "moves" step after G1 and before the first lane, on the integration branch. Each move commit changes only the declaration's package and its callers' qualifiers, with a named proof (`astequal` extended with a declared rename map, red first, or a hand comparison plus T2). A move that needs a new `rules.json` edge is its own owner question at G1. Card exit 2 distinguishes in-package from cross-package moves.

### Should-fix

**PA-S1 — "No seams across lanes by construction" is false: the coordinator's commits on lane branches touch shared files, and the known issues span lanes.** S1.1c-5-1 covers runtime and validate (lanes R, V); -2 covers event, parser, plan, playback (E, T, X, P); -3 covers root, expr, model, partition, apipartition, archcheck (T, X, A). No single lane branch ever holds an issue's last removal. The entries are sorted and adjacent, so cherry-picks of neighbouring removals are likely to conflict (inferred). **Fix:** lane branches remove only their own `smells.json` entries; issue closure (`known-issues.json`, the table row, the `test_aging.py:544` pin) is a coordinator commit on the integration branch at combine; §2 lists the coordinator's commits on lane branches as a seam re-resolved at combine.

**PA-S2 — The lane brief's "done = T2 `make check` exit 0" cannot be reached by the lane on its own.** A cleaned package's entry is red on a full-tree run (`gate/runner/rows/smells.py:390`), and the brief forbids the lane to edit `smells.json`. **Fix:** a lane's "done" is 0 size findings on its own packages under the pinned golangci-lint (both GOOS), its packages' tests passing and every commit through T0; the coordinator then commits the removal and runs T2 and the verifiers, reading the exit line.

**PA-S3 — The proof drivers pin code in files this sprint splits, and the plan runs `proofs` only on lane A's branch and at T4.** `gate/proofs/drivers/test_fuzz_decoder_planted.py:19-20, :34` anchors on `func (d *Decoder) Decode(` in `internal/event/decode.go` ("the plant's anchor moved"); `test_fuzz_parser_planted.py:18-19` on `func Parse(` in `internal/parser/parse.go`; `test_api_planted.py:18-21` on `Start` in `apphandle.go` and `NewStreamRegistry` in the clock. These worlds copy every tracked file under `internal/`. Sprint model §10 runs the `proofs` row after a commit that changes a file a proof's world copies. **Fix:** the file map lists pinned placements that must not move; `proofs` runs after G0, on lanes E, T and P at done, and on the combined branch before T3; `gate/proofs/drivers/*.py` are the coordinator's.

**PA-S4 — §0.7 misses two instruments that name a file of lane A:** `apipartition` (`gate/law.json:488-492`) and `clockgolden` (`:514-518`), both `"code": ["tools/apipartition/main.go"]`. **Fix:** a new apipartition file joins both lists, or apipartition stays in one file (404 lines today).

**PA-S5 — The two `validateApp` functions are correctly placed; moving either breaks the import rules.** `runtime.validateApp` (`internal/runtime/runtime.go:476`) checks `runtime.App` built by `runtime.FromPlan` (`internal/runtime/sync.go:160`), called by `build` and `(*Runtime).Add`; `playback.validateApp` (`internal/playback/reader.go:151`) checks `playback.App`, called by `NewReader` and `newDriver`. `internal/validate` may not import runtime or playback. They are precondition checks between packages returning plain errors; `validate` emits a program's E-codes. **Fix:** drop the example; name real candidates.

**PA-S6 — The hot-path list mislabels a path and undercounts.** The root `row` is "the facade's RowWriter … one per SendRow, Emit, Upsert row or Delete key" (`row.go:10-12`), two allocations per row a MUST (`row.go:24`): it is the facade's live-event path, not table load. §3 lists 38 functions, not about 33.

**PA-S7 — Per-function benchmarks are the wrong net for goroutine loops, and nothing checks compounding loss on a whole path.** `route`, `(*recQ).run`, `take`, `ingest`, `(*shard).process` run on the router or shard goroutine; a per-function benchmark needs hand-built router state and does not measure the per-event cost. A 2 % tolerance per function can add up past 2 % on a path. **Fix:** one path-level benchmark per hot path with its allocation pin, compared `main` vs the combined branch at G4; per-function benchmarks for pure leaf functions only; card exit 3 gains "each hot path end to end".

**PA-S8 — Benchmarks taken while a second agent runs `make check` are contaminated.** **Fix:** samples are taken by one runner while no gate tier runs; both test binaries are built once and alternated; a sample taken under contention is discarded and re-taken, both quoted.

**PA-S9 — No exit path exists for a function that resists a safe split and is not on a hot path.** Decision 6 allows a PERMANENT waiver only for a measured hot-path cost; `LAST_PART = "S1.1c-6"` (`gate/runner/parts.py:19, :117-118`); with the marker at S1.1c-6 any surviving entry is red at T4 and nightly (`:125-130`; `allowlists.py:34`). `Drain` (23) and `Close` (20) sit next to S3.3-1-3 and S3.3-2-1. **Fix:** a risk and a question, ranked: split anyway behind extra race and stress nets; the owner moves that issue's forward to S1.1c-7 with `LAST_PART` (a gate change); a PERMANENT waiver outside decision 6.

**PA-S10 — The commit budget and the "one PR" default are not realistic, and the two-PR option is not offered.** "G0 and the design 4" leaves out the plan-check commits, the design reviews, G1's decisions, issue closures, driver and code-list commits, the record. S1.1c-5 used 96 with fewer lanes. **Fix:** count the coordinator's commits line by line; offer two PRs (sprint model §3).

**PA-S11 — Docs and known-issue rows name code locations the split moves.** `docs/product/known-issues.md:9` (S3.3-1-3) points at `runtime.go` `iBatchEnd`, handled in `ingest`; ADR-0010 §3 names `internal/event/decode.go`. **Fix:** the coordinator updates them at combine.

**PA-S12 — The `test_aging.py:540` pin is more than an id list.** It encodes "three closed rows after S1.1c": `closing = {"S1.1c-5", "S1.1c-6", "S4.1"}` (`:541`), the marker `"S4.2"` (`:548`), the test's name `test_at_s4_1s_close…` (`:528`) and its docstring (`:521-526`); aging rule 1 counts closed rows after `first_seen` (`gate/runner/aging.py:45-54`, `CLOSED_ROWS_ALLOWED = 2`). **Fix:** the G0 commit moves the ids to `["S1.1c-5","S1.1c-6","S1.1c-7"]`, `closing` to `{"S1.1c-5","S1.1c-6","S1.1c-7"}`, the marker to `"S4.1"`, renames the test, and re-checks the witness red first.

**PA-S13 — "A table of handlers" needs a guard on hot paths and against the globals check.** A map or slice of funcs costs a lookup and an indirect call per event (inferred); a package-level table is legal under `globalscheck` only if never written outside its declaration (`tools/archcheck/main.go:766-792`); a table whose handlers refer back to it is an initialisation cycle. **Fix:** no handler tables on the hot-path list; elsewhere a never-written package-level var, or a method set on an existing type.

**PA-S14 — "No exclusion left" conflicts with §7's PERMANENT `//nolint:gocyclo,funlen` waivers** (precedent `internal/table/table.go:240`). **Fix:** "no TEMP size exclusion (`smells.json` empty); PERMANENT in-code waivers only under decision 6, each citing its samples."

**PA-S15 — The size (13 SP) is not measured.** The draft says lane D's rate will not hold; the nets are 38 benchmarks plus characterisation for 59 functions; the heaviest are cyclomatic 43–59; a 30-minute box "per package" cannot cover `internal/runtime`. **Fix:** a time-boxed spike (ground rule 8: findings only) splits `ingest` or `parseRFC3339` in a scratch worktree and measures; lanes R, V and A become sub-lanes; only then is "fits the cap" ticked.

### Later

**PA-L1 — `event.CleanLen`'s one production caller is `internal/runtime/boundary.go:119` (SendPayload), not playback.**

**PA-L2 — Law paths sit inside lane-owned directories** (`internal/runtime/nosleep_test.go`, `internal/validate/docsync_test.go`, `internal/expr/docsync_test.go`, the parser's `errorcounts_test.go`, `keywords_test.go`, `fragment*_test.go`, `parseerror_contract_test.go`; `gate/law.json:165-217`). **Fix:** read-only for lanes.

**PA-L3 — Row `S1.1c` stays open until S1.1c-7 closes;** S1.1c-6's close-out marks `S1.1c-6` only.

**PA-L4 — The file map must project file sizes after the splits;** a file moved at 480 lines can pass 500 at its last split. **Fix:** each file's projected size, at most about 450 when moved.

**PA-L5 — Q5's alternatives are not ranked by code health;** the healthiest is retiring in this sprint.

**PA-L6 — 23 test files use `AllocsPerRun`, not 22.**

## Focus 3 — the design approach and placement candidates

§5's three parts are the right skeleton but not yet build-ready. The design also needs the pinned placements (PA-S3), each file's projected size (PA-L4), for each split the order of checks it keeps down to which helper returns first, the proof for each cross-package move (PA-B1), the guard on handler tables (PA-S13) and each lane's hand-back state (PA-S2).

**Placement candidates (for the owner's decision 2):**
1. **The rule for turning a literal into a value is written three times:** `validate.fits` and `validate.as` (`internal/validate/resolve.go:57-80`), `plan.value` (`internal/plan/expect.go:181-204`, cyclomatic 18, one of the 110; its comment cites "as validate.as"), `expr.literalNode` (`internal/expr/nodes.go:20-37`). plan may not import validate; all three import `internal/model`. A candidate is one `model` function, e.g. `model.LitValue(l Literal, t event.Type) (event.Value, error)`, inside expr's `Lit` fence (`rules.json:36`). Caveat: not identical (`plan.value` accepts a string for `TObject`; `blockValue` handles object fields separately, `internal/validate/expects.go:142-150`); a merge must keep each caller's error text.
2. **The recording line's size arithmetic lives in runtime, the encoder in playback:** `rowBound` (`internal/runtime/boundary.go:65-67`), `payloadLimit`'s 256-byte envelope (`:88`), `CheckRows` (`internal/runtime/runtime.go:683`, used by `deploy.go:221`). runtime cannot import playback; `MaxLogLine` is in `internal/event/decode.go:20`. None of these is one of the 110: list it, backlog unless the owner wants it now.
3. **`tools/archcheck/main.go` holds two instruments**, `archcheck` and `testhygiene`; the test-hygiene evaluator (`CheckTest`, `Cases`, `fold`, … around `:804-1560`) is self-contained. Moving it to its own package changes the registry rows' commands, a gate-row change. Recommendation: a file split inside package main only, every new file in both instruments' `code` lists.
4. **Not candidates:** the two `validateApp` (PA-S5); `Deploy` and `bind` (`deploy.go:24, :121`), which orchestrate the facade — E-PARAM-001 parsing must happen there because only the root may import the parser, and running the expects is ADR-0005 §4's placement. `bind` splits by stage, keeping its order of checks.

## Focus 4 — the hot-path list

Correct: the live-event, evaluation, decode, recording and replay rows as listed (`cleanObject` per object value, `decode.go:196`, and per recorded payload, `boundary.go:129`; `CleanLen` per payload, `boundary.go:119`; `compactAndValid` beneath both). Mislabelled: root `(*row).run`, `(*row).put` belong to the facade's live path (PA-S6). Correctly out: `arithNode`, `plan.expect`/`values`/`value`/`rule`, `NewSchema`, `model.encode`/`encodeStruct`, `diverge`/`runBlock`/`sameValue`, `AppHandle.convert`. Borderline: `(*Trace).Step` is per replay step only when tracing. Count: 38.

## Focus 6 — Q3, Q4, Q5

**Q3.** A function stays whole only if the difference is significant (Mann–Whitney U over 10 interleaved samples per side, an exact stdlib script, p < 0.05) **and** the median moves by more than 2 %, with allocations always equal; plus PA-S8's discipline, prebuilt binaries, and the rule applied at path level at G4. Ranked: the combined rule; Mann–Whitney alone; 2 % on the median alone; overlapping ranges.

**Q4.** 100 % of statements over all tests as the floor, plus (a) unreachable statements (an invariant panic under a PERMANENT waiver, an error that cannot happen) listed with their reason, never counted as covered; (b) every control transfer the split moves across a new helper boundary (each `return`, `break`, `continue`, each counter next to an early return) gets one hand mutation shown red.

**Q5.** Keeping the machinery dormant and retiring it in S1.1c-7 is safe: rule 5(b) reds any new entry (`parts.py:166-167`) and `forwards` returns early with no entries (`:131-132`). Retirement must also cover DEVELOPMENT.md, ADR-0015 §2.2's annotation and the T3 countdown lines. The alternatives are misordered (PA-L5).

## Pre-mortem — two weeks on, the sprint failed. Why?

1. T4 red on `proofs`: lane E moved `Decode` out of `decode.go`, and nobody ran `proofs` until T4 (PA-S3).
2. Lane R stalls on `Drain` and `Close`: no safe split, no waiver path (PA-S9).
3. The benchmarks decided on noise, or 2 % per function added up to a larger loss per event (PA-S7, PA-S8).
4. Combine turned into conflict resolution over `smells.json` and issue closures; a cross-package move had no proof (PA-S1, PA-B1).
5. The 100-commit cap broke at the fix round (PA-S10).
6. Agents hit the 30-minute box on `internal/runtime` repeatedly (PA-S15).

## §0 claims checked

Correct: §0.1 (the 139, the per-package split, the 8 files, the two six-parameter functions, the sampled citations), §0.2 (the three benchmarks, lane D's numbers), §0.4, §0.5, §0.6, §0.9, §0.11, §0.13 (provided Q5 as recommended and archcheck not split into packages). Off: §0.2's 22 (23, PA-L6) and "about 33" (38, PA-S6); §0.3's `CleanLen` callers (PA-L1); §0.7 incomplete (PA-S4); §0.8 under-specified (PA-S12); §0.12's `validateApp` framing (PA-S5).
