# S1.1c-5 plan-check research: the strict lint set, the Go tools' build info, the two forwards

Worktree: /Users/ramith/code/cep-wt/s1-1c-5-measure (detached, main d4b028f, left clean: `git status --short` empty).
Scratch: /Users/ramith/code/cep-wt/s1-1c-5-scratch/. Started 12:09, data collected by about 12:25 (+05:30).
Environment for every Go/golangci command:
`GOTOOLCHAIN=local GOFLAGS= GOWORK=off PATH=~/.cache/vorsar/cep/tools/go-go1.27.4-91a44a2b8a71/bin:$PATH GOLANGCI_LINT_CACHE=<scratch>/glcache`
(the golangci cache is kept in scratch so that no earlier cached result can leak into a run).

## A. The strict set as a golangci-lint v2 config

Config: /Users/ramith/code/cep-wt/s1-1c-5-scratch/golangci.yml (`golangci-lint config verify` exit 0).
It enables gocyclo 10, funlen 60 lines, lll 120, revive (file-length-limit 500, argument-limit 5), forbidigo (fmt.Print*, panic, reflect, unsafe, os.Getenv), errcheck, gocritic and nolintlint. `default: none`, severity error, no exclusion presets, and no per-linter or same-issue caps.

Commands:
```
golangci-lint config verify --config <scratch>/golangci.yml
cd <worktree> && golangci-lint run --config <scratch>/golangci.yml ./... > <scratch>/root.json        # exit 1, ~17 s
cd <worktree>/gate/proofs && golangci-lint run --config <scratch>/golangci.yml ./... > <scratch>/proofs.json   # exit 0, 0 issues
```
gate/proofs is a separate module (gate/proofs/go.mod). ADR-0015 §2.1 puts it outside every analyzer. I ran it for information only, and it gave 0 findings.

### Where the ADR's wording left a choice, and what I chose
1. **File ≤ 500 lines and params ≤ 5.** golangci-lint has no dedicated linter for either; only revive covers them, with `file-length-limit` and `argument-limit`. I enabled revive with just those two rules. File length counts every line (skipComments=false, skipBlankLines=false).
2. **Cyclomatic ≤ 10.** I used gocyclo, the linter §2.1 names, with `min-complexity: 10`. It reports complexity above 10, so 11 is the first value reported.
3. **funlen.** I set `lines: 60` only, because the ADR names lines. **Trap:** `statements: -1` turns funlen off completely in v2.15 (measured: 0 findings). I used `statements: 100000` instead and left `ignore-comments` at its default (true).
4. **lll.** 120 characters, `tab-width: 1` (the default).
5. **Linter selection.** `linters.default: none` plus only the named linters. v2's `standard` preset would add govet, staticcheck, ineffassign and unused; govet already has its own `vet` row.
6. **nolintlint.** Enabled with require-explanation, require-specific and allow-unused=false, because ADR §1's annotation pairs it with `smells`. It found 0 (the tree has no //nolint).
7. **forbidigo.** `analyze-types: true` with the patterns `^fmt\.Print.*$`, `^panic$`, `^reflect\..*$`, `^unsafe\..*$` and `^os\.Getenv$`. With analyze-types, every use of a reflect value's method counts (v.Kind, f.Type and so on), so the reflect figures are uses, not imports.
8. **reflect in L0–L2.** I took the layers from ARCHITECTURE.md's layer charter: L0 event, model, metrics; L1 clock; L2 pattern, window, expr, text, table, partition. An exclusion rule (`path-except '^internal/(…)/'`, text `[reflect]`) drops reflect hits everywhere else.
9. **os.Getenv outside cmd and io config paths.** There is no `cmd/`, and `io/` has no config file, so I allowed the whole `io/` tree. Only `os.Getenv` is forbidden, not os.LookupEnv or os.Environ, because the ADR names Getenv only.
10. **fmt.Print* in library code.** The config flags it everywhere and the report splits the results. All 8 hits are in `tools/apipartition/main.go`, which is `package main`. The three tools/ programs are binaries, so the library-code reading gives 0 and the everywhere reading gives 8.
11. **panic outside an allowlist.** No panic allowlist exists in tools/allowlists/, so every panic counts.
12. **errcheck and gocritic.** errcheck uses its defaults (check-type-assertions and check-blank off). gocritic uses its default checker set.
13. **Warnings are errors.** Severity defaults to error, there are no exclusion presets, and `max-issues-per-linter: 0` and `max-same-issues: 0` lift the caps.
14. **Test scope (§2.1).** `_test.go` files are exempt from funlen, gocyclo and revive (file length and params). lll, forbidigo, errcheck and gocritic still apply.
15. **tools/.** tools/ Go code gets the full set, because the tools class is gated by `smells`.
16. **Build tags.** The repository has none; I used the default tags.
17. **Paths.** `run.relative-path-mode: gomod`. v2 resolves exclusion paths relative to the config file by default, and a config kept outside the repository would then never match.
18. **Two default settings that hide findings.** Both are set explicitly in my config:
   - `issues.uniq-by-line` defaults to true and keeps one finding per line. funlen, gocyclo and revive's argument-limit all report on the `func` line, so funlen disappeared entirely (0 instead of 18) and one params finding was hidden. I set it to false.
   - `linters.exclusions.generated` defaults to `lax`. Lax treats any file whose comments mention "Code generated", "DO NOT EDIT" or "autogenerated" as generated and drops all of its findings. That silently skipped tools/archcheck/main.go (1,698 lines, unreported), tools/archcheck/archcheck_test.go and tools/archcheck/breakit_s11c2_test.go, because archcheck's own comments describe the generated-file header. I set it to `strict`, which honours only Go's canonical header and matches §2.1's policy.

## B. Counts

Total 1,386 findings: lll 1,026, forbidigo 183, gocyclo 116, errcheck 24, funlen 18, revive 11, gocritic 8, nolintlint 0.

| scope | findings | files | breakdown |
|---|---|---|---|
| production (non-test, outside tools/) | **343** | 45 | lll 173, gocyclo 90, reflect 33, funlen 16, panic 15, file>500 8, gocritic 6 (all appendAssign), params 2; fmt.Print 0, Getenv 0, unsafe 0, errcheck 0 |
| tools/ (non-test) | **74** | 3 | lll 31, gocyclo 26, fmt.Print 8 (all package main), errcheck 6, funlen 2, file>500 1 |
| production + tools | **417** | 48 | |
| tests | **969** | 143 | lll 822, reflect 93, panic 28, errcheck 18, Getenv 6, gocritic 2 (appendAssign 1, mapKey 1) |

### Comparison with 308 (what moved)
- **The 2026-11-11 config is reproduced exactly.** I exported fd274e4 with `git archive fd274e4 | tar -x` into scratch/fd274e4. Running it with uniq-by-line=true, generated=lax, **gocyclo min-complexity 11** (which allows 11, one off from "≤ 10"), no reflect rule and funlen effectively absent gives **308 non-test findings across 46 files**: lll 186, gocyclo 79, panic 15, fmt.Print 12, file>500 9, gocritic 6, params 1. The test figures match the record too (panic 27, errcheck 17, Getenv 6, gocritic 2), apart from lll at 726 against the recorded 724. Config: scratch/golangci-1111-repro.yml.
- **The same old config on d4b028f gives 288 non-test findings across 45 files.** The drop is not real. Since S1.1c-2, archcheck's comments mention the generated header, so lax mode now hides tools/archcheck/main.go.
- **ADR-exact config, fd274e4 → d4b028f** (scratch/compare.py):

| kind | prod | tools | tests |
|---|---|---|---|
| lll | 171 → 173 | 20 → 31 | 732 → 822 |
| gocyclo | 88 → 90 | 12 → 26 | – |
| funlen | 16 → 16 | 1 → 2 | – |
| reflect | 33 → 33 | – | 93 → 93 |
| panic | 15 → 15 | – | 27 → 28 |
| fmt.Print | – | 12 → 8 | – |
| errcheck | 0 → 0 | 0 → 6 | 17 → 18 |
| gocritic | 6 → 6 | – | 2 → 2 |
| file>500 | 8 → 8 | 1 → 1 | – |
| params | 2 → 2 | – | – |
| Getenv | – | – | 6 → 6 |
| **total** | 339 → 343 | 46 → 74 | 877 → 969 |

- **Engine production code barely moved (339 → 343).** The growth is:
  - tools/archcheck: 14 new functions over cyclomatic 10 — Unused 26, markDead 23, fold 21, walk 16, terminates 14, Cases 14, LoadLists 14, binding 14, cli 14, table 12, CheckTest 12, Stale 12, canCompare 12, writes 12.
  - internal/parser: firstExpected 13 and bareDuration 12.
  - Test long lines: +90.
  - No function left the list.
- **Why the ADR-exact count is higher than 308:**
  - gocyclo: 79 → 116 non-test. +21 comes from the corrected threshold and +16 from new functions.
  - funlen: +18, now visible.
  - reflect: +33 in production and +93 in tests, a rule the old config lacked.
  - tools errcheck: +6, unhidden by strict generated mode.
  - params: 2 instead of 1.

### By package, non-test
Columns: total | lll gocyclo funlen file>500 params panic fmt.Print Getenv reflect errcheck gocritic
```
internal/validate     89 |  67 19 3 0 0 0 0 0  0 0 0
internal/runtime      59 |  35 16 3 2 1 2 0 0  0 0 0
tools/archcheck       52 |  25 19 1 1 0 0 0 0  0 6 0
internal/model        38 |   2  2 0 1 0 0 0 0 33 0 0
internal/playback     34 |   9 15 1 1 0 2 0 0  0 0 6
internal/parser       25 |  18  5 1 1 0 0 0 0  0 0 0
. (package cep)       22 |  14  5 2 0 0 1 0 0  0 0 0
internal/event        21 |   6 12 2 1 0 0 0 0  0 0 0
tools/apipartition    20 |   6  5 1 0 0 0 8 0  0 0 0
internal/partition    19 |   9  2 0 1 0 7 0 0  0 0 0
internal/plan         15 |   7  5 1 0 1 1 0 0  0 0 0
internal/expr         14 |   4  5 3 1 0 1 0 0  0 0 0
internal/table         3 |   1  2 . . . . . .  . . .
internal/clock         2 |   0  1 0 0 0 1 . .  . . .
io/inmemory            2 |   1  1 . . . . . .  . . .
tools/shape            2 |   0  2 . . . . . .  . . .
```

### By package, tests
- internal/playback 151 (lll 141, Getenv 4, panic 3, errcheck 3)
- internal/validate 111 (all lll)
- internal/runtime 103 (lll 94, panic 8, Getenv 1)
- internal/event 96 (lll 56, reflect 33, panic 7)
- package cep (.) 91 (lll 79, errcheck 11, panic 1)
- internal/expr 88 (lll 84, panic 4)
- internal/parser 86 (lll 83, panic 1, Getenv 1, errcheck 1)
- tools/archcheck 66 (all lll)
- internal/model 50 (reflect 47, lll 3)
- internal/partition 34 (lll 31, panic 3)
- internal/plan 32 (lll 31, panic 1)
- internal/clock 27 (lll 14, reflect 13)
- internal/table 17 (lll 15, gocritic 2)
- internal/text 5, tools/apipartition 5, io/inmemory 4, io 2, internal/metrics 1

### Top files
Non-test:
- 52 tools/archcheck/main.go (lll 25, gocyclo 19, errcheck 6, funlen 1, file 1)
- 38 internal/model/model.go (reflect 33, gocyclo 2, lll 2, file 1)
- 37 internal/runtime/runtime.go (lll 24, gocyclo 8, funlen 2, panic 1, params 1, file 1)
- 28 internal/validate/catalog.go (all lll)
- 20 tools/apipartition/main.go (fmt.Print 8, lll 6, gocyclo 5, funlen 1)
- 19 internal/partition/partition.go (lll 9, panic 7, gocyclo 2, file 1)
- 18 internal/event/decode.go (gocyclo 11, lll 4, funlen 2, file 1)
- 18 internal/validate/exprs.go (lll 11, gocyclo 6, funlen 1)
- 15 internal/validate/graph.go
- 14 internal/validate/bounds.go
- 12 internal/playback/reader.go
- 11 internal/parser/lower.go
- 10 deploy.go
- 10 internal/runtime/lifecycle.go
- 9 internal/expr/nodes.go
- 8 internal/plan/expect.go
- 8 internal/playback/recorder.go

Tests:
- 65 internal/validate/units_test.go
- 60 tools/archcheck/archcheck_test.go
- 59 internal/expr/overflow_test.go
- 50 internal/model/canonical_test.go
- 29 internal/event/value_test.go
- 24 internal/parser/parseerror_contract_test.go
- 24 internal/validate/outputs_test.go
- 22 internal/event/decode_test.go

### Cyclomatic complexity per package (non-test)
"Excess" is the sum of each function's complexity above 10.
```
internal/runtime    16 functions, excess 137, 7 at >=20:
  ingest 48 (runtime.go:874), validateApp 34 (:470), Drain 23 (lifecycle.go:184), shard.process 20 (:1369), router.take 20 (:1173), Send 20 (:623)
internal/validate   19 functions, excess 121, 4 at >=20:
  selects 34 (exprs.go:247), partition 31 (bounds.go:30), graph 30 (graph.go:15), names 21 (names.go:49), givenRecord 16, before 16
tools/archcheck     19 functions, excess 113, 4 at >=20:
  purity 28 (main.go:557), Unused 26 (:249), markDead 23 (:894), fold 21 (:1030), globals 17, clock 16
internal/expr        5 functions, excess 88, 4 at >=20:
  arith.Eval 33 (nodes.go:291), builder.node 32 (expr.go:168), cast.Eval 29 (nodes.go:598), arithNode 28 (nodes.go:256), compare.Eval 16
internal/playback   15 functions, excess 83, 2 at >=20:
  diverge 29 (expect.go:106), Reader.eventRecord 23 (reader.go:461), Reader.record 17, appendString 16, appendEvent 16, sameValue 15
internal/event      12 functions, excess 83, 2 at >=20:
  parseRFC3339 43 (decode.go:812), Decoder.value 28 (decode.go:130), appendUnescaped 18, scanner.number 17, cleanObject 13, CleanLen 13
internal/parser      5 functions, excess 70:
  parseError 59 (parse.go:121), joinPragma 21 (lexer.go:126), lowerer.str 15, firstExpected 13, bareDuration 12
. (package cep)      5 functions, excess 43:
  Runtime.bind 33 (deploy.go:117), Runtime.Deploy 24 (deploy.go:24), AppHandle.convert 13, row.put 12, row.run 11
tools/apipartition   5 functions, excess 29:
  fileSurface 21, canon 21, surface 15, main 11, exportedOnly 11
internal/plan        5 functions, excess 28:
  expect 20 (expect.go:74), value 18, rule 18 (plan.go:195), values 11, body.OnEvent 11
internal/model       2: encode 13, encodeStruct 11
internal/partition   2: Rule.Offer 15, keyOf 11
internal/table       2: New 13, Table.checkValue 13
tools/shape          2: shape 19, shapes 11
internal/clock       1: RuleClock.fireGroup 11
io/inmemory          1: Source.Open 11
```

### funlen (18 functions, every one also over cyclomatic 10)
- deploy.go:24 Deploy 81, deploy.go:117 bind 93
- internal/event/decode.go:130 value 69, decode.go:812 parseRFC3339 67
- internal/expr/expr.go:168 node 85; internal/expr/nodes.go:291 Eval 100, nodes.go:598 Eval 74
- internal/parser/parse.go:121 parseError 93
- internal/plan/expect.go:74 expect 69
- internal/playback/expect.go:106 diverge 89
- internal/runtime/lifecycle.go:184 Drain 62; internal/runtime/runtime.go:874 ingest 128, runtime.go:1173 take 70
- internal/validate/bounds.go:30 partition 99, exprs.go:247 selects 66, graph.go:15 graph 73
- tools/apipartition/main.go:208 fileSurface 63; tools/archcheck/main.go:249 Unused 68

### Files over 500 lines (9, the same 9 as on 2026-11-11)
tools/archcheck/main.go 1,698; internal/runtime/runtime.go 1,536; internal/event/decode.go 931; internal/expr/nodes.go 708; internal/parser/lower.go 574; internal/partition/partition.go 529; internal/runtime/lifecycle.go 523; internal/playback/reader.go 521; internal/model/model.go 509.

### Over five parameters
internal/runtime/runtime.go:1092 `(*router).broadcast` (6); internal/plan/expect.go:74 `expect` (6).

### Every panic / fmt.Print* / os.Getenv / unsafe / reflect site
**panic, production (15):**
- internal/clock/rule.go:124
- internal/expr/registry.go:100
- internal/partition/partition.go:42, 262, 267, 269, 271, 345, 517
- internal/plan/plan.go:254
- internal/playback/driver.go:76, 91
- internal/runtime/runtime.go:1295
- internal/runtime/sync.go:169
- row.go:73 (`panic(ErrRowUsedAfterFill)`)

All but row.go's are invariant panics ("…which no input can cause", "a plan holds only checked tables"). row.go:73 is an API misuse panic.

**panic, tests (28):**
- breakit_s3_3_1_test.go:516
- internal/event/gen_test.go:186; internal/event/oracle_test.go:60, 68, 72, 84, 195, 206
- internal/expr/expr_test.go:75, 78, 82, 85
- internal/parser/parseerror_contract_test.go:70
- internal/partition/onarrival_test.go:154; internal/partition/panic_test.go:58, 75
- internal/plan/body_test.go:303
- internal/playback/breakit2_test.go:326; internal/playback/expect_test.go:301; internal/playback/s3_2b_2_corpus_test.go:94
- internal/runtime/api_test.go:129; internal/runtime/breakit_s3_2b_2_test.go:23; internal/runtime/breakit_test.go:429, 433, 440; internal/runtime/fixround_test.go:93; internal/runtime/keys_test.go:135; internal/runtime/lifecycle_test.go:533

Most are deliberate panics planted in bodies or sinks to test recovery. The rest are test-helper invariants.

**fmt.Print* (8):** tools/apipartition/main.go:53, 54, 71, 76, 77, 81, 85, 87 (package main; the `GATE_COVERED` and `failed:` lines).

**os.Getenv (6, all in tests):**
- internal/parser/golden_test.go:16 (CEP_UPDATE_GOLDEN)
- internal/playback/baseline_test.go:52 (CEP_BASELINE_WRITE)
- internal/playback/s3_2b_2_corpus_test.go:198 (CEP_TRIPWIRE_FILE), :291 (CEP_DETERMINISM_PLANT), :301 (CEP_UPDATE_GOLDEN)
- internal/runtime/breakit_test.go:80 (BK_CHILD)

**unsafe:** 0.

**reflect (126 uses):**
- internal/model/model.go: 33 uses in production, in the canonical encoder's `encode` and `encodeStruct` (reflect.Value, TypeFor, v.Kind, NumField, Field and so on).
- Tests: internal/model/canonical_test.go 47, internal/event/value_test.go 26, internal/clock/breakit_test.go 13, internal/event/schema_test.go 6, internal/event/event_extra_test.go 1. The most common are reflect.TypeFor 19, reflect.DeepEqual 17 and struct field walks.

**errcheck (24):**
- tools/archcheck/main.go:1643, 1646, 1661, 1681, 1686, 1688 (fmt.Fprintf / Fprintln results)
- boundary_test.go:35–42, 53, 54
- example_test.go:24 (rt.Close)
- `Close` unchecked: internal/parser/fragment_research_test.go:42, internal/playback/breakit_s3_2b_1_race_test.go:61, internal/playback/equivalence_test.go:227, internal/playback/s3_2b_2_corpus_test.go:154, internal/text/text_test.go:183
- os.RemoveAll unchecked: tools/apipartition/main_test.go:58, 62

**gocritic (8):**
- internal/playback/recorder.go:55, 71, 120, 140, 166, 199 appendAssign. These are the buffer-reuse idiom `b := append(r.buf[:0], …)`.
- internal/table/breakit_test.go:77 appendAssign
- internal/table/table_test.go:292 mapKey

### Candidate lanes by package (input for a split; the owner decides)
**Rates assumed (human-equivalent, deliberately conservative; use them for relative weight):**
- lll: 2 min a line
- A function over cyclomatic 10, refactored test-first behind the existing nets (funlen rides along): 11–14 → 1.5 h, 15–19 → 3 h, 20–29 → 5 h, ≥30 → 8 h
- Splitting a file: under 700 lines 2 h, under 1,000 lines 4 h, 1,000 or more 8 h
- panic 0.5 h, errcheck 0.25 h, gocritic 0.5 h, params 1 h, fmt.Print 0.25 h, Getenv 0.5 h
- reflect 0.75 h a use, only if rewritten
- 8 h a day
- Script: scratch/lanes.py

**Totals:** about 543 h ≈ 68 lane-days. About 95 h (≈ 12 d) of that is reflect work that depends on a decision, so ≈ 56 d without reflect. The project's agent lanes have historically run faster than these rates.

**Per package (non-test):**
- runtime 69 h ≈ 8.6 d (2 functions ≥30, 5 at 20–29, two file splits)
- validate 60 h ≈ 7.5 d
- tools/archcheck 57 h ≈ 7.2 d
- playback 43 h ≈ 5.4 d
- event 35 h ≈ 4.4 d
- expr 34 h ≈ 4.2 d
- model 30 h ≈ 3.7 d (25 h of it reflect)
- parser 22 h ≈ 2.7 d
- package cep 18.5 h ≈ 2.3 d
- apipartition 18 h ≈ 2.3 d
- plan 16 h ≈ 2.0 d
- partition 10 h ≈ 1.3 d
- shape 4.5 h, table 3 h, clock 2 h, io/inmemory 1.5 h

**Tests:** about 45 h ≈ 5.6 d excluding reflect, spread across every package. The reflect test work is model ≈ 35 h, event ≈ 25 h and clock ≈ 10 h.

**Candidate lanes, each ≤ 8 d at these rates:**
- **L1a runtime** — runtime.go split plus the router (ingest, take, process, Send, broadcast): ≈ 5.5 d. **L1b runtime** — lifecycle.go Drain, validateApp and the rest: ≈ 3 d. Runtime as a single lane is ≈ 8.6 d, over the 8-day cap.
- **L2 validate** ≈ 7.5 d.
- **L3 tools/archcheck** ≈ 7.2 d. tools/ is a law path, so every edit shows as a LAW Δ line, and archcheck's own tests are the gate's teeth.
- **L4 playback + plan** ≈ 7.4 d.
- **L5 event + parser** ≈ 7.1 d.
- **L6 expr + package cep** ≈ 6.5 d.
- **L7 apipartition + shape + partition, table, clock, io/inmemory** ≈ 5 d.
- **Test-file fixes** (≈ 5.6 d: 822 lll, 18 errcheck, 6 Getenv, 28 panics, 2 gocritic). These touch the same packages, so with one owner per file they fold into the package lanes rather than forming a separate lane.
- **L9 reflect (decision-dependent).** model.go's encoder (33 uses in 2 functions) and the 93 test uses: ≈ 12 d if rewritten, ≈ 0 if the owner exempts or re-scopes them.

**What this means for capacity:** ≈ 56 d against 8-day lanes means at least 7 lanes. With at most 2 agents at once and sprints of at most 2 weeks, that does not fit one sprint at these rates.

## C. The pinned Go tools' build information

`go version -m ~/.cache/vorsar/cep/tools/golangci-lint-v2.15.0-d1e409481a8c/golangci-lint` (saved at scratch/golangci-lint.buildinfo.txt):
- Built with **go1.27.0**.
- path github.com/golangci/golangci-lint/v2/cmd/golangci-lint, mod v2.15.0.
- **207 dep lines, no replacements.**
- -trimpath, CGO_ENABLED=0, darwin/arm64, vcs.revision 25c5369a…, vcs.time 2026-10-16.
- The linux-amd64 binary is not in the local cache, so I did not read it. tools.json's version_line for the tool reads "built with go1.27.0".

Other pinned binaries built with Go: only the Go distribution itself (go and gofmt at go1.27.4), which the row already covers as stdlib/toolchain at the pinned Go. ruff (Rust), CPython (C) and lark (Python) are outside the Go database.

**The vulnerability-database copy:** ~/.cache/vorsar/cep/vulndb (path from gate/toolchain/vulndb_fetch.py:87–93 `copy_root`). The stamp says fetched 2026-11-13T20:31:13Z; the database's `modified` is 2026-11-11T02:51:11Z; it holds 4,801 entries, and `check_db` reports no faults. The copy is fresh, so I did not refetch it.

**How the row matches:** gate/runner/vulndb.py:234 `findings(modules, never_built, go_version, entries)`. It follows x/vuln's affects.go, where OSV SEMVER ranges treat introduced "0" as minus infinity, and computes earliestValidFix. It **always judges both pseudo-modules `stdlib` and `toolchain` at the given Go version** (GO_PSEUDO, :31; the loop at :274–279). gate/runner/rows/vulns.py:182 `_module_map` maps each module to its replacement when the replacement has a version.

**My script:** scratch/scripts/toolvulns.py, run with the pinned python3.15 -I. It imports gate.runner.vulndb and gate.runner.rows.vulns read-only, and calls `vulns.open_copy`, `vulns.entries_of` and `vulndb.findings(<build list>, frozenset(), "go1.27.0", entries)`.

**Result: 28 findings, 0 faults, 0 in any of the 207 third-party modules.**

15 in the standard library (the "15 expected"):

| id | package | fixed in |
|---|---|---|
| GO-2026-6599 | html/template | 1.27.2 |
| GO-2026-6600 | html/template | 1.27.2 |
| GO-2026-6603 | net/http, http2 | 1.27.2 |
| GO-2026-6604 | os (Root, Windows) | 1.27.2 |
| GO-2026-6605 | net/http, httputil | 1.27.2 |
| GO-2026-6607 | crypto/tls | 1.27.2 |
| GO-2026-6608 | net/textproto, mime/multipart | 1.27.2 |
| GO-2026-6609 | net/http | 1.27.2 |
| GO-2026-6610 | net/http, http2 | 1.27.2 |
| GO-2026-6611 | net/http, http2 | 1.27.2 |
| GO-2026-6612 | net/http, http2 | 1.27.2 |
| GO-2026-6613 | net/http | 1.27.2 |
| GO-2026-6617 | net/http, http2 | 1.27.2 |
| GO-2026-6809 | crypto/x509 | 1.27.3 |
| GO-2026-6859 | net/http | 1.27.4 |

13 in `toolchain` (cmd/go, cmd/compile, cmd/link, cmd/cgo): GO-2026-6601 and 6602 (fixed in 1.27.2), and GO-2026-6848 to 6858 (fixed in 1.27.4).

**The 13 toolchain hits do not apply to a tool binary.** They come only from `findings` judging `toolchain` at the given Go version. A binary built with go1.27.0 does not contain the go command, and govulncheck's binary mode reports only stdlib and modules. Fed the build list unchanged, the row would be red on 28 findings rather than 15.

**There is no fixed golangci-lint release.** `gh api repos/golangci/golangci-lint/releases` shows v2.15.0 (2026-10-16) as the latest; nothing has been released since. All 15 standard-library findings are fixed in go ≥ 1.27.4, so any golangci-lint release built with go 1.27.4 or later would clear them. Under decision 5, landing the check today means 15 dated exemptions in tools/allowlists/vulns.json (TEMP, each with an expiry and a known issue).

## D. The two forwards

### (i) gosec's `#nosec`
**gosec is not in ADR-0015 §2's strict set,** and my config does not enable it. With gosec off, `#nosec` waives nothing. DEVELOPMENT.md:70's example directive is `//nolint:gosec … sprint=S1.1c-2`, which would be red under the set as written ("gosec is not enabled").

**Measured with the pinned v2.15.0** (scratch/nosec, one case per file, `--enable gosec`, defaults):
- **Honoured** (the G401 finding disappears): `// #nosec`, `// #nosec G401`, `// #nosec G401 -- reason`, `//#nosec`, `/* #nosec */`, `// #nosecX` (a glued suffix still counts), and `// #nosec` on the line above. `// #nosec` above the package clause **silences the whole file**: nolintlint then reported the import's `//nolint:gosec` as unused. `//nolint:gosec` is honoured too.
- **Not honoured:** `// some text #nosec`, `// #NOSEC`, `// nosec`, a doc comment ending in "#nosec", and `// #nosec G101` on a G401 line (a rule list waives only the rules it names).
- **With `linters.settings.gosec.config.global.nosec: true`** (plus show-ignored), every `#nosec` form is ignored (all 14 cases reported), while `//nolint:gosec` still waives.

**What gate/runner/suppressions.py says and does today:**
- :24 `PREFILTER = ("nolint", "fmt:", "noqa", "yapf:", "isort:", "Code generated")` — no "nosec".
- :21 `ENABLED: frozenset[str] = frozenset()  # S1.1c-5 sets it from its golangci config (DA-L4)`.
- :153–154 the go_directives docstring: "The //nolint directives golangci would honour (§0.3)…".
- :310 `"…{linter} is not enabled (none is until S1.1c-5)"`.

So a `#nosec` is neither a directive nor a fault. gate/runner/tests/test_breakit_s11c2.py:31–39 records BK-S4: the red tests `NosecIsAWaiver` were taken out of the module at commit fc944ef, "to be restored there" (that is, in S1.1c-5).

**revive has the same kind of waiver, measured** (scratch/revivetest). revive is the only golangci linter that covers file ≤ 500 and params ≤ 5. Each of these removed the argument-limit finding:
- `//revive:disable-next-line:argument-limit`
- `//revive:disable-line`
- a `//revive:disable` … `//revive:enable` block
- `// revive:disable-next-line` (with a space)

Only the control case without a directive was reported. suppressions.py does not know any of these forms. I did not check whether the other linters in the set honour comment directives of their own (§F).

### (ii) The ruff.toml keys
**How the row runs ruff:** gate/runner/rows/ruff.py:23 `FLAGS = ("--no-cache", "--config", "ruff.toml", "--output-format", "full")`. It runs `ruff format --check` and then `ruff check --ignore-noqa` on an explicit file list, and its covered count is the sum of the format summaries.

**What the ruff pin reads:** gate/runner/rows/suppressions.py:85–112 `ruff_pin` turns red on top-level `extend`, on the top-level ignore/extend-ignore/per-file-ignores/extend-per-file-ignores, and on lint.ignore/extend-ignore. It checks lint per-file-ignores against tools/allowlists/ruff.json both ways. **It does not read** exclude, extend-exclude, force-exclude, lint.exclude, format.exclude or lint.select.

**What the pin tests cover:** gate/runner/tests/test_rows_ruff.py:98 (every banned API, i.e. TID251) and :130 (a BLE001 plant) run ruff with a copy of the repository's ruff.toml. Dropping "TID" or "BLE" from select would therefore turn pytests red. No test plants E, W, F, I, B, UP, SIM or S602/S604/S605.

**Experiment** with the pinned ruff 0.17.3, run exactly as the row runs it, on 3 files (one BLE001, one time.sleep TID251, one clean); scratch/rufftest/:

| ruff.toml change | format summary | check | row verdict |
|---|---|---|---|
| none | 3 formatted | 2 errors | red (correct) |
| `exclude = [2 files]` | 3 | 2 errors | no effect (explicitly passed paths bypass exclusion) |
| `extend-exclude` | 3 | 2 errors | no effect |
| `[lint] exclude` without force-exclude | 3 | 2 errors | no effect |
| `force-exclude = true` + extend-exclude | **1** | passes | red through the count (covered 1 ≠ 3) |
| `force-exclude` + `[format] exclude` | **2** | 2 errors | red through the count |
| `force-exclude` + `[lint] exclude` | 3 | **passes** | **green: a hole** |
| narrowed `lint.select = ["E","W","F"]` | 3 | **passes** | **green: a hole**, partly caught by the TID/BLE pin tests only |

ruff.toml is a law path (test_law_pin.py:38), so any edit shows as a LAW Δ line. The ruff 0.17.5 upgrade (another PR, not merged) was not measured.

## E. Stale references
Code, tests and user-visible messages already say S1.1c-5 for the lint set: suppressions.py:21 and :310; test_suppressions.py:308 and :321; gate/proofs/drivers/test_suppressions_planted.py:38; vulns.py:44 (NOT_READ: "golangci-lint (Go build info, from S1.1c-5)"); test_vulns_planted.py:611; test_breakit_s11c2.py:31–39.

Every remaining "S1.1c-4" in code means the scans sprint and is correct: the canary, vulns, deps, Makefile:30, law.json:711, registry.json:595. gate/current-sprint reads S1.1c-4, the marker S1.1c-5's first commit changes.

Stale doc lines:
- **docs/development/DEVELOPMENT.md:5** ("What exists (2026-11-06)"): "**Not built yet:** the suppressions checker, `todocheck` and golangci-lint's `smells` (S1.1c-2)." The suppressions checker was built in S1.1c-2, and smells is S1.1c-5.
- **docs/development/DEVELOPMENT.md:70**: the example `//nolint:gosec` names a linter the strict set does not enable.
- **ADR-0015:108**: the `smells` row's lands_in column still says "S1", with no annotation in the row itself. The general annotation at :81 moves it to S1.1c.
- History (not to be rewritten): docs/sprints/s1-1c-2-record.md:79, :93 and :131 say "S1.1c-4". decisions.md:774 already carries the S1.1c-5 annotation.

No row named `smells` exists in gate/registry.json. golangci-lint is pinned and parity-checked but no row runs it.

## What this means for the plan check (facts only; the owner decides)
1. **The 308 figure came from a config that undercounts:**
   - The gocyclo threshold was one off (it allowed 11).
   - uniq-by-line hid funlen and one params finding.
   - lax generated mode hid tools/archcheck/main.go.
   - There was no reflect rule.

   The ADR-exact count on d4b028f is **417 non-test findings in 48 files** (343 engine + 74 tools) and **969 in tests**.
2. **Engine production code is flat since fd274e4 (339 → 343).** The growth is tools/archcheck (gocyclo 12 → 26) and test long lines (+90).
3. **Three settings must be pinned whatever config lands,** each with a planted proof: uniq-by-line=false, generated=strict, and a funlen statements value other than −1. A config kept outside the repository also needs relative-path-mode.
4. **reflect in L0–L2 is the largest block that depends on a decision.** It is 33 production uses (model.go's canonical encoder, 2 functions) and 93 test uses in L0/L1 tests, which §2.1's forbidigo covers. It costs ≈ 12 lane-days if rewritten.
5. **panic:** 15 production invariant panics need the forbidigo-panic allowlist that ADR §3.10 names but tools/allowlists/ does not have. The 28 test panics are mostly planted on purpose.
6. **fmt.Print* "library code"** gives 0 if package main is excluded, 8 (all in tools/apipartition) if not.
7. **Waivers the suppressions checker does not know:** gosec's `#nosec` (including a whole-file form), live only if gosec is enabled; and revive's `//revive:disable…` directives, live as soon as revive is enabled for the file-length and params limits.
8. **vulns on golangci-lint:**
   - 15 standard-library findings and no third-party ones; there is no fixed golangci-lint release.
   - The current matcher, applied to a tool's build list as-is, adds 13 `toolchain` findings that do not apply to a binary.
9. **ruff holes:**
   - force-exclude plus `[lint] exclude`, or a narrowed `lint.select`, pass silently.
   - exclude and extend-exclude alone change nothing for this row.
   - The row's count catches force-exclude and format exclusions.
10. **Effort at the stated conservative rates** is ≈ 56 lane-days without reflect and ≈ 68 with it. runtime alone is ≈ 8.6 d, over the 8-day lane cap. The total exceeds one 2-week sprint with at most 2 agents at these rates.

## F. Not measured
- The linux-amd64 golangci-lint binary's build info (not in the local cache).
- T0's staged-file mode.
- Whether linters other than gosec and revive honour comment directives of their own.
- golangci formatters and `--fix`.
- ruff 0.17.5 behaviour.

Scratch artefacts: golangci.yml (the ADR-exact config), golangci-1111-repro.yml (reproduces 308), root.json, proofs.json, fd274e4.json, repro-*.json, analysis.txt, analyze.py, compare.py, lanes.py, scripts/toolvulns.py, golangci-lint.buildinfo.txt, nosec/, revivetest/, rufftest/.
