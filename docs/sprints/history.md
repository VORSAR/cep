# Phase 1 history — closed sprint cards and the status log

**What this is:** text that left the live [sprint plan](sprint-plan.md) once its sprints closed, moved word for word on 2026-11-06. Nothing here is current: what is built is the code and git; what is next is [sprint-plan §Status](sprint-plan.md#status); why each choice was made is in [decisions.md](decisions.md). Read a card here only to see what a closed sprint was asked to prove; its [record](README.md#records) says what it did.

In the moved text, "§5" means [decisions.md](decisions.md) and "§Status" means [the status log](#the-status-log) below.

**Adding to it:** at each close-out, move the closed card here and append the replaced status line to the log.

## Closed sprint cards

| Card | Sprints | Records | PR |
|---|---|---|---|
| [S1.1a](#s11a--the-gate-runner-ci-coverage-and-the-event-kernel) | S1.1a-1, S1.1a-2 (split at the plan check) | [S1.1a-1](s1-1a-1-record.md), [S1.1a-2](s1-1a-2-record.md) | #1, #2 |
| [S1.1b](#s11b--the-analyzers-and-the-law) | S1.1b, slim (`archcheck`; the rest moved to S1.1c) | [record](s1-1b-record.md) | #3 |
| [S1.2](#s12--the-model-kernel-the-remaining-gate-rows-and-spec-rev-3) | S1.2, cut to the model kernel and spec rev 3 | [record](s1-2-record.md) | #4 |
| [S2.1](#s21--the-parser) | S2.1 | [record](s2-1-record.md) | #5 |
| [S2.2a](#s22a--the-grammar-differential-and-the-expression-engine) | S2.2a | [record](s2-2a-record.md) | #6 |
| [S2.2b](#s22b--the-validator) | S2.2b | [record](s2-2b-record.md) | #7 |
| [S3.1](#s31--the-clock-service-tables-and-the-decoder) | S3.1-1, S3.1-2 | [S3.1-1](s3-1-1-record.md), [S3.1-2](s3-1-2-record.md) | #8, #9 |
| [S3.2a](#s32a--the-replay-format-and-the-runtime-core) | S3.2a-1, S3.2a-2 | [S3.2a-1](s3-2a-1-record.md), [S3.2a-2](s3-2a-2-record.md) | #10, #11 |

### S1.1a — The gate runner, CI, coverage and the event kernel

| | |
|---|---|
| **Goal** | A clean clone runs `make check` — pinned toolchain, parallel registry runner, deterministic summary — locally and in CI, with coverage enforced at 90% on the first Go package, `internal/event`. |
| **Routine · size** | Full loop (the event kernel fixes the null representation and the total order every later output depends on; the gate is an owner ratification condition) · 13 SP (items 12.5) |
| **Depends on** | — (first sprint; phase entry §9 holds: architecture ratified and review-hardened ✔, decomposition approved ✔) |
| **In scope** | 1. The Go module `github.com/vorsar/complex-events-engine` *(amended 2026-11-02, owner: `github.com/vorsar/cep`)*: `go.mod` with `go` and `toolchain` lines, `GOTOOLCHAIN=local` [0.5]. · 2. `tools.json` + one installer (`cache/tmp.$$`, atomic `mv` into the hash-keyed path) + the parity step, first at every tier [2]. · 3. `gate/registry.json` (row schema = ADR-0015 §1.1), `gate/run.sh` (POSIX; per-step process groups; TERM-then-KILL watchdogs; one JSON record per step; `summary.md` + `run.json`), collect-all, zero-is-failure, `expected_scope` checked against the live expansion, the `exit:` line; `make check`, `make combine-check`, `make strict-gate`; **parallel by default** (owner 2026-11-02; ADR-0015 §1.1): rows run concurrently up to a worker limit, each starting when its `needs` finish; `exclusive` rows run alone; the summary stays in registry order and prints wall time beside summed step time [4]. · 4. The `gofmt` and `go vet` rows [0.5]. · 5. `.githooks/` (T0, T1) and the CI workflow (actions pinned by SHA; rows grouped into **concurrent jobs** by their `job` field, each restoring the Go module and build cache keyed by `go.sum` + the `tools.json` hash; the summary job `needs: all`, `if: always()`, the only required check) with summary fixture trees [2]. · 6. `gate/proofs/` as a separate Go module, with this sprint's proofs [1.5]. · 7. **`internal/event`**: `Type`, `Attr`, `Schema`, `Value` (null = `Value{Null:true}` keeping the declared `Kind`), `Event`, the total-order comparator `(Ts, StreamID, Seq)` [1]. · 8. **`coverage-inclusion`** (owner 2026-11-02): every `internal/*` and public package has tests (or a reasoned allowlist entry), and ≥ **90%** statement coverage per package from T2's coverprofile, scope per ADR-0015 §2.2 (generated files excluded), no waiver within scope [1]. |
| **Not in scope** | The architecture analyzers, LAW Δ, registry-lint, suppressions → S1.1b. `internal/model` and spec rev 3 → S1.2. |
| **Entry criteria** | The standard ones, plus (answered by the owner 2026-11-02, §5): toolchain = the newest stable Go that the pinned golangci-lint supports — exact versions recorded in the plan check on day one; the Go specialist is `.claude/agents/go-engineer.md` with JetBrains' `use-modern-go` skill; every agent `model: inherit`; `main` has force-push and deletion blocked before G0 ✔ (ruleset `protect-main`, 2026-11-02), and the summary check becomes a required status check in that ruleset once this sprint's CI has run (before it merges). Hook wiring is ADR-0015 §3.1's (`core.hooksPath .githooks`). |
| **Exit criteria** | The done-done block (§13), plus: |
| | 1. A clean clone runs `make check` to `exit: 0`; CI shows the same rows — **proof:** CI `strict-gate` on the PR and the summary fixture tests classifying every planted tree. |
| | 2. A tool version that differs from `tools.json` fails before any other step and names the fix — **proof:** the parity proof. |
| | 3. The runner really runs in parallel, and safely: two independent fixture rows that each wait for the other's marker file finish only if they run at the same time (run one after the other, the watchdog kills them — red); a row never starts before its `needs` finish; an `exclusive` row never overlaps another; the summary is byte-identical whatever order rows finish in — **proof:** runner fixture tests (no sleeps — marker files and watchdog ceilings). |
| | 4. The parallel saving is measured — **proof:** the summary footer's wall time vs summed step time on the PR's CI run, recorded in the sprint record. |
| | 5. An unformatted Go file is refused at T0 — **proof:** a hook-level proof. |
| | 6. A package below 90% statement coverage, and a package with no tests, each turn the gate red naming the package and its number; `internal/event` is at or above 90% — **proof:** two planted proofs + the T2 run. |
| | 7. The event total order and the null representation are pinned — **proof:** `internal/event` table tests whose expected orderings are written by hand before the code. |
| **Owner's check** | *By running:* clone fresh, `make check`, read the summary and its timing footer; commit an unformatted file and watch the hook refuse it. *By hand:* add the required check on GitHub once CI has run. |
| **Split line** | None needed (12.5); if the plan check sizes it over, CI caching and the timing footer move to S1.1b. *(Annotated 2026-11-02: this split line could not be used: S1.1b is full, and §3's rule forbids moving work into a full neighbour. The plan check re-sized the card for the owner's Python decision at about 15 points, and the owner split it into its own two sprints: S1.1a-1 and S1.1a-2 below. This card stays as the approved original; the plan check is [s1-1a-plan-check.md](s1-1a-plan-check.md).)* |

### S1.1a-1 — The local gate, coverage and the event kernel

| | |
|---|---|
| **Goal** | A clean clone runs `make check` to `exit: 0` with the pinned toolchain, the parallel registry runner and a deterministic summary; coverage is enforced at 90% on `internal/event`; and every instrument, including the checks on the gate's own Python, is proven able to fail. |
| **Routine · size** | Full loop, no design panel (owner 2026-11-02) · 13 SP (items 11.5) |
| **Depends on** | — (first sprint) |
| **In scope** | All gate tooling is stdlib Python under CPython 3.15.2, pinned (owner 2026-11-02). 1. `go.mod` (`go 1.27.0`, `toolchain go1.27.3`) [0.5]. · 2. `tools.json` (Go 1.27.3, golangci-lint v2.15.0, CPython 3.15.2, ruff 0.17.3; archive and per-executable sha256) + `gate/bootstrap.py` (any Python ≥ 3.9 *(superseded 2026-11-02 by owner decision 9: the installer needs ≥ 3.12, verified under a pinned 3.12.16; the 3.9-syntax launcher only refuses)*; safe extraction; atomic rename into the shared cache) + parity (the hard stop; every listed executable re-hashed every run) [2.5]. · 3. `gate/registry.json` + the runner `gate/run.py` / `gate/runner/` (parallel, `needs`, `exclusive`, watchdogs over process groups, collect-all, zero-is-failure, live expected scope, records keyed by job, a summary deterministic under an injected clock, the timing footer) + `make check|combine-check|strict-gate|ci` [4]. · 4. The `gofmt`, `vet` and `unit` rows [0.5]. · 5. `internal/event` [1]. · 6. `coverage-inclusion` at 90%, classified from `go test -json` events; no waivers until S1.1b [1]. · 7. The `pytests` row, so the gate runs its own tests [0.5]. · 8. The `ruff` and `pyfloor` rows (the gate's Python formatted and linted; the bootstrap parses at 3.9 *(as built: `pyfloor` runs the floor modules' tests under 3.12.16; ruff's `py39` target holds `bootstrap.py`)*) [0.5]. · 9. `gate/proofs/`: parity, coverage ×2, `pytests`, `ruff` [1]. A coordinator-built throwaway runner scaffold serves wave 1 and is deleted when the real runner lands (owner 2026-11-02). |
| **Not in scope** | Hooks and CI → S1.1a-2. The analyzers, LAW Δ, registry-lint, the allowlist grammar → S1.1b. |
| **Entry criteria** | The standard ones; the plan check answered (owner 2026-11-02); `.claude/agents/python-engineer.md` exists. |
| **Exit criteria** | The done-done block, plus (numbering kept from S1.1a): |
| | 1. A clean clone with an empty tool cache runs `make check` to `exit: 0` — **proof:** the coordinator's fresh-clone run, recorded. |
| | 2. A tool whose version or executable hash differs from `tools.json` fails before any other row starts, naming the fix — **proof:** the parity proof (three separate cases: version, not installed, path). |
| | 3. Runner behaviour as on the S1.1a card; the summary is byte-identical whatever order rows finish in — **proof:** runner fixture tests (FIFO-held rows, an injected clock, no sleeps). |
| | 6. A package below 90% and a package with no tests each turn the gate red, naming the package and its number; `internal/event` ≥ 90% — **proof:** two planted proofs + the T2 run. |
| | 7. The event total order and the null representation are pinned — **proof:** `internal/event` tables written by QA, by hand, red before the code. |
| | 8. The gate's own tests run in the gate: a test module that is untracked, fails to import, skips everything or holds no test turns `pytests` red — **proof:** the `pytests` proof. |
| | 9. Unformatted gate Python, and a `time.sleep` outside the wait helper, are red — **proof:** the `ruff` proof. |
| **Owner's check** | *By running:* clone fresh, `make check`, read the summary and its timing footer. *By reading:* the decisions the coordinator flagged as uncertain (owner 2026-11-02). |
| **Split line** | None needed (11.5). |

### S1.1a-2 — Hooks and CI

| | |
|---|---|
| **Goal** | Every commit and push runs its tier through the same runner, `make check` refuses to run until the hooks are on, and CI runs the registry's rows as concurrent jobs with one required check. |
| **Routine · size** | Full loop · 5 SP (items 3.5) *(re-sized 2026-11-03 by the owner at G0: about 10 SP, items 5.5, then about 8 SP, items 4.5, on 2026-11-04; see the [plan check](s1-1a-2-plan-check.md) §0 and §5 decisions)* |
| **Depends on** | S1.1a-1 |
| **In scope** | 1. `.githooks/pre-commit` (T0 over an index snapshot; refuses an unformatted file and prints the fix; SKIP when nothing is staged) and `.githooks/pre-push` (T1 over the pushed tip), both Python; `make setup`; a row that makes `make check` refuse until the hooks are on (CI sets `core.hooksPath`) [1]. · 2. `gate/ci/`, which generates `.github/workflows/gate.yml` from `registry.json` (`ubuntu-latest` per the owner, actions pinned by SHA, concurrent jobs, module and build caches), plus the `ci-fresh` row [1]. · 3. `summarize` (expected rows taken from the registry) + its fixture trees [1]. · 4. The hook proof and the summary proofs [0.5]. |
| **Not in scope** | The analyzers and LAW Δ → S1.1b. |
| **Entry criteria** | The standard ones (S1.1a-1 merged). |
| **Exit criteria** | The done-done block, plus (numbering kept from S1.1a): |
| | 1. CI shows the same rows as a local `make strict-gate` — **proof:** CI `strict-gate` on the PR, the summary fixture trees, and a diff of the two row lists. |
| | 4. The parallel saving is measured — **proof:** the PR's CI timing footer, recorded. |
| | 5. An unformatted Go file is refused at T0, including under `git commit -a` and `git commit <path>` — **proof:** the hook proof. |
| | 10. `make check` with the hooks off fails, naming `make setup` — **proof:** a proof row. |
| **Owner's check** | *By running:* commit an unformatted file and watch the refusal; `make check` in a clone without `make setup`. *By hand:* add `strict-gate` as a required status check in `protect-main` once CI has run. |
| **Split line** | None needed (3.5). |

### S1.1b — The analyzers and the law

| | |
|---|---|
| **Goal** | Every architectural rule that can be checked from stage 1's code is mechanically red when broken — imports, clock use, goroutines, globals, TODOs, test hygiene, lint smells, suppressions — each analyzer proven able to fail, and every change to the gate's own rules shows in the LAW Δ block. |
| **Routine · size** | Full loop · 13 SP (items 13) |
| **Depends on** | S1.1a *(annotated 2026-11-02: now S1.1a-2, after the split)* |
| **In scope** | 1. `gate/law.json` + `lawdelta` + the LAW Δ block at the top of the summary (ADR-0015 §1.3) [2]. · 2. registry-lint with its three planted problems and the §2 instrument-id pin [1]. · 3. `importcheck` with `tools/importcheck/rules.json` encoding ARCHITECTURE §3's import lists as **exhaustive** (owner 2026-11-02) and the post-`io`-split edges; the docsync comparison of parsed edges; the edge-count ceiling [2]. · 4. `clockcheck`, incl. a synthetic fixture for the `internal/window/cron.go` exception [1]. · 5. `puritycheck` (module-wide) and `globalscheck` (const-like defined in the analyzer) [2]. · 6. `todocheck` (basic `TODO(owner\|S<n>\|#id)`) and `testhygiene` [1]. · 7. The teeth meta-test (`min_wants` per analyzer) [1]. · 8. golangci-lint strict config (`smells`, `forbidigo`, the §2.1 scope classes) + `nolintlint` [1]. · 9. The `suppressions` checker and the one allowlist grammar under `tools/allowlists/` [2]. Each analyzer ships with its `analysistest` fixture and scans S1.1a's code retroactively on landing. |
| **Not in scope** | `docsync`, the dependency/licence/vulnerability/build-tag rows and the sprint-id grammar → S1.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. A planted edit to `rules.json` appears in the LAW Δ block with its one-line semantic delta — **proof:** `lawdelta` test over a fixture repository. |
| | 2. An upward import from `internal/event` is red at T0 on the staged file — **proof:** `importcheck` fixture + a hook-level proof. |
| | 3. `time.Now()` outside the allowlist is red; the synthetic `cron.go` fixture passes `time.LoadLocation` and fails `time.Now` — **proof:** `clockcheck` fixtures. |
| | 4. A `go` statement outside the allowed packages, an `unsafe` import, and a mutable package-level `var` are each red; an error sentinel is not — **proof:** `puritycheck` and `globalscheck` fixtures. |
| | 5. A bare `//nolint`, a PERMANENT suppression without a resolvable `ref=`, and an allowlist entry naming a non-existent package each fail — **proof:** `suppressions` tests. |
| | 6. Weakening a fixture below its `min_wants`, or deleting a §2 instrument's registry row, fails; registry-lint reports its three planted problems — **proof:** the teeth meta-test + registry-lint proofs. |
| | 7. A `time.Sleep` in a test outside the wait helper is red — **proof:** `testhygiene` fixture. |
| | 8. Every analyzer's retroactive scan of S1.1a's code is green — **proof:** the T2 run. |
| **Owner's check** | *By running:* add a forbidden import on purpose and watch the commit refused. *By reading:* the LAW Δ block on the PR. |
| **Split line** | **S1.1b-1** = items 1–5 (law, imports, clock, purity, globals); **S1.1b-2** = items 6–9. |

### S1.2 — The model kernel, the remaining gate rows and spec rev 3

| | |
|---|---|
| **Goal** | `internal/model` holds the rule IR with a byte-stable canonical encoding, the remaining stage-1 gate rows are live, and spec rev 3 — the owner's language decisions of 2026-11-01 and 2026-11-02 — is applied and mechanically checked. |
| **Routine · size** | Full loop (the IR and a language revision) · 13 SP (items 11) |
| **Depends on** | S1.1b |
| **In scope** | 1. **`internal/model`**: the IR of ADR-0003 (every field named after its SPEC clause; durations and timestamps `int64` ms; `ParamRef` unresolved) and `MarshalCanonical` (sorted keys, explicit defaults, float `'g'`/−1/64, RFC3339 UTC millis, NaN/±Inf unencodable); **the canonical JSON carries no source positions** — diagnostics keep positions in a side table [3]. · 2. The dependency-allowlist check, the licence check, `govulncheck`, the build-tag law, the weekly canary [2]. · 3. The sprint-id grammar (`S<stage>[.<part>][<letter>][-<n>]` (the `-<n>` suffix only when a plan check applies a card's split line)), `gate/current-sprint`, and the skip / `deferred(P<n>, ref=…)` forms in `todocheck` [1]. · 4. `docsync` (a) one direction (every package directory that exists is listed in ARCHITECTURE §3) and `docsync` (d) error-count consistency [1]. · 5. **Spec rev 3** (language-design skill; docs lane), containing exactly: (a) the derived-stream-consumption error code (owner 2026-11-01); (b) **the cross-stream partition-key type-mismatch code** (RC8.1; owner 2026-11-02: reject at deploy — the error names both streams and both types); (c) **`late events allow` removed for v0.1** (owner 2026-11-02): `allow` is rejected wherever it appears, with E-LATE-002's text revised to "late handling per window arrives in Phase 2; use `late events drop` and set the stream's watermark bound"; example 02 changes to `late events drop`; SEMANTICS §7's cron row annotated; (d) **cron admission pinned to SPEC §6.1**: an event behind its stream's admission watermark is dropped and counted even if its cron bucket is still open — the Python reference executor's `cron_window` is aligned and a new harness check pins it, written red-first; (e) **`text_invisibles` is engine-owned** (owner 2026-11-02): the name is reserved; the engine always computes it; a payload value is ignored, overwritten and counted; a stream may declare it as `bool` for readability; (f) **keyword sets over a table operand read the table's per-event snapshot like `in`** (owner 2026-11-02); (g) `time arrival` stays (owner 2026-11-02): playback logs carry the recorded arrival stamp as `ts`; (h) E-GRAPH-001 marked unreachable in v0.1 and kept for Phase 2, with `docsync`'s 1:1 rule accepting the marking; (i) errors.md, SPEC Appendix C and the change log updated [4]. *(Added 2026-11-02 by owner decision at S1.1a-1's design review: (j) a stream's `timestamp = <attr>` must name a `timestamp`-typed attribute, as a coded deploy error with fix text [incl.]. The item total stays about 11; the plan check re-sizes it.)* |
| **Not in scope** | Expect blocks for the examples → each example's own sprint. The SPEC §9 timing-comment sentence → S6.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. `MarshalCanonical` is byte-stable and position-free — **proof:** a hand-derived canonical-JSON golden (key order, −0, 1e21, millisecond truncation, NaN/±Inf rejection) written before the code, plus a test that two models differing only in source positions encode identically. |
| | 2. An unapproved `go.mod` edit, a disallowed licence, a known-vulnerable module and a `//go:build !race` file are each red — **proof:** one planted proof each. |
| | 3. A `TODO` naming a sprint more than two rows ahead, and a skip naming the current sprint, are red — **proof:** `todocheck` fixtures driven by a fixture `gate/current-sprint`. |
| | 4. Spec rev 3 is applied: every decided code is present in errors.md with its SPEC § reference and fix text; the counts agree everywhere — **proof:** `docsync` (d) green and red on a planted count mismatch; `grammar_check` PASS; the harness passes with its new cron-admission check, first seen failing against the unaligned executor. |
| **Owner's check** | *By reading:* the rev-3 change log entry and the revised E-LATE-002 text; the canonical JSON of one model. *By hand:* the LAW Δ block (spec files changed). |
| **Split line** | **S1.2-1** = items 1–4; **S1.2-2** = item 5. |

### S2.1 — The parser

| | |
|---|---|
| **Goal** | `internal/parser` turns every corpus file into a `model.App` or into the exact catalogued parse error at the exact line and column, and keeps every comment. |
| **Routine · size** | Full loop (the language) · 13 SP (items 13) |
| **Depends on** | S1.2 (rev 3 applied) |
| **In scope** | 1. The hand-written lexer (ADR-0002 §3), positions in runes, **every comment kept as positioned trivia in a side table reachable from the parse result** (S6.2's printer needs it), with its lossless round-trip test and a comment-torture file [3]. · 2. `tools/kwgen/` → `internal/parser/keywords.go` (generated header) [1]. · 3. `cep.y` hand-derived from `grammar.lark`; `parser_gen.go` committed (generated header); the AST; lowering to `model.App` [5]. · 4. The `yyError` mapping for all [parse] codes, with line, column and fix text [2]. · 5. **Expected positions:** before the parser runs, the QA agent adds a hand-derived `@ <line>:<col>` to the first-line comment of each of the 24 files in `docs/language/invalid/` [1]. · 6. `freshness` (goyacc + kwgen; scratch-dir regeneration; header-manifest anti-forgery) [1]. *(Annotated 2026-11-04, owner: built on participle v2 — items 1, 3, 4, 5 as read for participle (the grammar as struct tags, no `cep.y`, no `parser_gen.go`); item 2 becomes a pinning test; item 6 is moot. See §5, 2026-11-04.)* |
| **Not in scope** | The grammar differential, the parser fuzz target and the nightly workflow → S2.2a. Static codes and typing → S2.2a/S2.2b. The printer → S6.2. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. All 9 examples, the generated eval files and the torture files parse; each model's canonical JSON is golden-locked — **proof:** the parser corpus test. |
| | 2. Each invalid file yields exactly its [parse] code at exactly its `@ line:col` — **proof:** the invalid-corpus table test. |
| | 3. Lexing and re-assembling every corpus, torture and comment-torture file from tokens plus trivia reproduces it byte for byte — **proof:** the lossless trivia test. |
| | 4. A stale `parser_gen.go` and a forged generated header are each red — **proof:** `freshness` proofs. *(annotated 2026-11-04: moot, no generated file; participle)* |
| **Owner's check** | *By running:* a deliberately broken rule file through the parser test helper (command named in the plan check) — read code, line, column and fix text. *By reading:* the messages for E-KW-002, E-PAT-001 and E-STR-001. |
| **Split line** | **S2.1-1** = items 1, 2, 5 (lexer, keywords, positions); **S2.1-2** = items 3, 4, 6. |

### S2.2a — The grammar differential and the expression engine

| | |
|---|---|
| **Goal** | The engine's parser is proven equivalent to the normative lark grammar over the full corpus — by verdict and by parse shape — and every expression form in SPEC §5.10 is typed, with the scalar forms compiled into executable trees. |
| **Routine · size** | Full loop (the language) · 13 SP (items 10.5) |
| **Depends on** | S2.1 |
| **In scope** | 1. The grammar-differential T4 row (ADR-0002 §5 as amended): pinned Python + `lark==1.3.1`; verdicts and canonical operator-spine S-expressions; ≥ 10 torture files in `docs/language/eval/generated/torture/`; corpus count asserted [3]. · 2. The parser fuzz target and the nightly workflow (one advisory, aging row per fuzz target) [1]. · 3. `docsync` (b) reserved words ↔ kwgen [0.5]. · 4. `internal/expr` (ADR-0011): **typing for every SPEC §5.10 form** — scalar, `in`, text operators, `chain`, `count` — exported to `validate`; **executable nodes for** scalar, arithmetic, comparison, boolean, null handling and the functions `coalesce`, `minimum`, `maximum`, `abs`, `math:round` (half-up, per Appendix F — not Go's `math.Round`), `cast`, `ifThenElse`; the `Registry` with all nine Appendix F callables (the `in` nodes land in S3.1, `chain` in S4.1, text nodes in S7, `count` in S8) [4]. · 5. The per-function evaluation table [1]. · 6. `docsync` (c) Appendix F ↔ `expr.Registry`; `allocgate` first rows [1]. · 7. The 1.1 null-strict check's scalar assertions ported [incl.]. |
| **Not in scope** | The validator → S2.2b. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The differential is green with counts at or above the known minimum, and red on a planted precedence swap in `cep.y` — **proof:** the differential row + its proof. *(annotated 2026-11-04: the planted swap goes in the participle grammar structs)* |
| | 2. Every E-TYPE-001 row of SPEC §5.10 has a table test — **proof:** `internal/expr` typing tests. |
| | 3. Every function evaluates per Appendix F: `math:round` at ±0.5 and ±1.5; `cast` success and failure per target (a counted runtime error, never a panic); integer `/` and `%`; `minimum`/`maximum` over timestamps and durations; `ifThenElse` with a null condition — **proof:** the per-function table, expected values written before the code. |
| | 4. Scalar evaluation allocates nothing except listed exclusions (`cast` to string) — **proof:** `allocgate`. |
| | 5. `docsync` (b) and (c) go red on a planted drift — **proof:** their fixtures. |
| **Owner's check** | *By reading:* one differential S-expression pair for a precedence torture file. |
| **Split line** | None needed (10.5). |

### S2.2b — The validator

| | |
|---|---|
| **Goal** | Every corpus program validates clean, and every reachable [static] code fires on its own minimal program with the catalogued fix text, line and column. |
| **Routine · size** | Full loop · 13 SP (items 11.5) |
| **Depends on** | S2.2a |
| **In scope** | 1. `internal/validate`: `catalog.go` (every [static] code with fix text, incl. rev 3's); one check per code; E-TTL-002 for pattern bounds and rate intervals; the E-NULL lints; E-REGEX-001's pre-scan [5]. · 2. Static test material in `internal/validate/testdata/` — one minimal program per reachable [static] code, **including the derived-stream and key-type codes** [1]. · 3. The connector annotation table `connectors.go` (E-NAME-001; `punctuates`; E-SINK-001; E-WIN-002) [1]. · 4. **The stream-graph checks** E-GRAPH-001/002/003, the derived-stream rejection and the cross-stream key-type check live here (ADR-0012 §1 as annotated) [1.5]. · 5. `${param}` resolution [1]. · 6. Pinned at the plan check, each with a named testdata program: whether a stream with no `@source` punctuates (E-WIN-002 for 04's `LoadedRecords`); how a `string` param becomes a list (05's `${risk_terms}`) [0.5]. · 7. Harness ports: RT-14, "9 · TTL deploy check", RT-07 (pattern and rate parts) [1]. · 8. `docsync` (b) [static] codes ↔ `catalog.go` [0.5]. |
| **Not in scope** | E-EXP-001 → S3.3. E-TXT-002 → S7. E-WIN-003 and the cron part of E-TTL-002 → S9.1. Each is a visible `skip: not-implemented(<sprint>)`. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Each reachable [static] code fires on its testdata program with its fix text, line and column; no corpus example raises any code — **proof:** the validator table test + the corpus-clean test. |
| | 2. A rule reading a derived stream, and a partition block whose key is text in one stream and a number in another, are each rejected naming what the error says — **proof:** their testdata programs. |
| | 3. `docsync` (b) goes red on a planted catalog drift — **proof:** its fixture. |
| **Owner's check** | *By reading:* the fix texts of five codes the owner picks. *By running:* one invalid program per picked code through the validator test helper. |
| **Split line** | None needed (11.5). |

### S3.1 — The clock service, tables and the decoder

| | |
|---|---|
| **Goal** | The clock service implements ADR-0004 completely — every timer kind and the discard path exercised — tables give snapshot-per-event reads, and JSON decodes by schema with full integer range, all proven by unit tests whose arithmetic matches the harness. |
| **Routine · size** | Full loop (event time, watermarks, ordering, lateness) · 13 SP (items 13) |
| **Depends on** | S2.2b |
| **In scope** | 1. `internal/clock`, all of ADR-0004, incl. **the discard path** (a partial's `within` deadline is a discard timer kind routed to `OnDiscard`, never `OnTimers`) and **a test-double operator** that schedules every timer kind and the discard kind; clock-level unit tests of admission strictness, the clamped min, per-stream virtual floors and idle exclusion, each citing its source harness check [6]. · 2. `internal/metrics` counters [0.5]. · 3. `internal/table` and the `in` / `not in` membership nodes (null-strict), completing the 1.1 check's `in` assertion [2]. · 4. `internal/text.IngestFlags` (invisible flagging only) and the Unicode-version agreement check; `BuildInfo`'s Unicode fields [1]. · 5. `internal/event/decode.go`, the schema-driven JSON decoder, with its differential test against `encoding/json` and a fuzz target written before any optimization [3]. · 6. The `puritycheck` named exceptions for `internal/table` and `internal/event` with fixtures [0.5]. |
| **Not in scope** | The replay log format and the runtime → S3.2a. Harness ports → none here. |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The clock's arithmetic matches each cited harness check — **proof:** `internal/clock` unit tests, each naming its source check. |
| | 2. Through the test-double operator, every timer kind fires in due order within (b), discards arrive only in (c), and a pending completion is never discarded — **proof:** the timer-contract test. |
| | 3. A two-key registry + `Instance` trace: one key silent, its construct clock still advances through the broadcast `WMAdvance` — **proof:** the quiet-key clock test. |
| | 4. A table write is visible to the next evaluation, never the current one — **proof:** the table snapshot test. |
| | 5. The decoder agrees with `encoding/json` on accept/reject and values, and keeps `int64` above 2^53 exact — **proof:** the differential test; fuzz nightly. |
| **Owner's check** | *By reading:* the timer-contract test's table of timer kinds and their order. |
| **Split line** | **S3.1-1** = items 1, 2 (clock); **S3.1-2** = items 3–6. |

### S3.2a — The replay format and the runtime core

| | |
|---|---|
| **Goal** | The engine's moving parts exist and hold under load: a deterministic replay-log format, per-key partitions with TTL, and sharded per-key execution with non-blocking routing. |
| **Routine · size** | Full loop (concurrency, partitioned state) · 13 SP (items 11.5) |
| **Depends on** | S3.1 |
| **In scope** | 1. `internal/playback`'s **format layer**: the log format — events; punctuation records; batch-end records `{"batch_end":"<id>","stream":"<name>"}`; table records applied at their file position — the loader, the virtual-clock `Driver`, the canonical output encoder, and a **canonical trace** (released events, timer fires, late counters) [2.5]. · 2. `internal/partition` (ADR-0006): `Instance`, `OpState`, key extraction via a compiled `expr.Expr`, the TTL purge timer and re-arm, `LastActivity`, the state registry, the timer-derivability invariant [3]. · 3. `internal/runtime`: one ingest/router goroutine; shards with bounded mailboxes, per-key serial; the non-blocking outbox handoff; the router servicing every handoff while blocked; sink queues and the runtime-owned sink flusher goroutines (calling a sink through an internal function type the facade later adapts); the key-less pseudo-key; panic recovery at the shard boundary; **an injectable idle-tick source** for the live driver [6]. |
| **Not in scope** | Rule plans, the replay drive loop and rule semantics → S3.2b. |
| *Forwarded* | *(Annotated 2026-11-04, from S3.1-2, owner D1: an event reads every table as it was at its ingest position, for every rule and shard, however long it waits in a release buffer — the runtime takes one snapshot of the app's tables per event and carries it with the event (the carrier is this sprint's design; the review recommends one opaque per-event field on `event.Event`); a timer-triggered evaluation needs a defined snapshot too (review F15: recommended, the snapshot current at the ingest position whose advance fired the timer). Exit 2 tests it. The canonical encoder must say what it emits for an object value's malformed UTF-8, which the decoder keeps verbatim (review F11).)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Loading the same log twice yields a byte-identical canonical trace, and a planted map-iteration order turns it red — **proof:** the trace test + its proof. |
| | 2. A table record changes membership from the next event onward, never for the current one — **proof:** a table + trace test. |
| | 3. Progress with a full mailbox at a shard whose handoff is pending, plus a slow sink and a bursty source — **proof:** the saturation test (test-double operators) under `-race` and a CPU hog, 50/50 passes. |
| | 4. One TTL timer per instance, re-armed on activity, never one per event — **proof:** the case guarding the Siddhi purge defect class. |
| **Owner's check** | *By reading:* the saturation scenario and its pass rate. |
| **Split line** | **S3.2a-1** = items 1, 2; **S3.2a-2** = item 3. |

*How it runs (owner, 2026-11-05, §5 "S3.2a's plan check"):* S3.1's routine — QA's tests first and independent; an architecture review of each design note; the build in the main session; a QA break-it. Split along the card's line, two sprints and two PRs, one at a time:
- **S3.2a-1** = items 1, 2 (the replay format layer, partitions) → exits 1, 2, 4; the forwarded snapshot carrier and its replay half (the loader's snapshot at each log position), the timer-evaluation rule (F15) and the encoder's malformed UTF-8 (F11).
- **S3.2a-2** = item 3 (the runtime core) → exit 3; the live router's half of the snapshot carrier, and the advance messages' (F15).

*The plan check's readings (coordinator, against `main` @ 1047512):*
1. **Key evaluation moves from item 2 to item 3's router.** ADR-0006 §4 gives it to the router, and `tools/importcheck/rules.json` lets `internal/partition` import only `event`, `clock` and `metrics`. `partition` takes the key value.
2. **Item 3's router reads a routing-table type that `internal/runtime` defines;** its tests fill it by hand, and S3.2b's planner produces it.
3. **Exit 3's `-race` run is a recorded local run** (testing.md: the pass rate goes in the PR); `go test -race` runs with `CGO_ENABLED=0` on darwin-arm64. The saturation test also runs without `-race` in the `unit` row. The gate's race row stays S3.2b's.
4. **The log format's open points** — whether an event line's `ts` repeats the time attribute in `attrs`, what a punctuation record is in v0.1, what the encoder writes for malformed UTF-8 in an object value — come to the owner as one list after S3.2a-1's design review.
5. **The snapshot carrier on `event.Event` must be a type `event` can hold without importing `table`;** S3.2a-1's design note chooses it.

*Forwarded to S3.2a-2 (2026-11-05, from S3.2a-1's design v2 §10 and owner D1, D5):* the router reads the key attribute and makes a null, wrong-type or NaN key a per-rule event error; it sets `Tables` from `table.Set.Snapshot`; an event and its own advances travel in one message; **every message that can trigger a walk carries the router's snapshot**, proven by a table-reading timer giving one result with 1 and with 4 shards; one single-threaded ingest step (Seq, snapshot, `ObserveEvent`, the virtual floor or ticks) serves the router and the playback driver; a shard may walk per message or per chunk (D1 makes them equal). *(S3.2a-2's plan check, owner 2026-11-05: the drain barrier's mechanism is built here, and so is one shared ingest step, which the playback driver is refactored to call; about 7.5 points.)*


## Resolved owner items

*Moved from the plan's "Still open for the owner" once done.*

- *Approval of the v3 recalibration — **done** (2026-11-02); the sprint references in ADR-0005 §1, ADR-0012 §1, ADR-0015 and qa-strategy §4 are updated to v3's ids.*
- *Owner action — **done** (2026-11-02, verified via the GitHub API): ruleset `protect-main` is active on the default branch with deletion and force-push (non-fast-forward) blocked and no bypass actors. S1.1a adds the summary job as a required status check to the same ruleset once its CI has run.* *(Annotated 2026-11-05: the required status check is dropped by the owner — §5, "branch protection".)*

*Resolved 2026-11-06 ([owner](decisions.md#owner-decisions-2026-11-06--the-docs-restructure)): the three lasting rules joined the sprint model; the one-module check joined S1.1c; S2.2a-1 was closed.*

- **Three lasting-rule proposals from retros:** (1) a design claim about a library's behaviour is checked with a scratch program before review ([S3.1-2 retro](s3-1-2-record.md#retro)); (2) a new CI workflow is run once by hand in the sprint that adds it ([S3.2a-2 retro](s3-2a-2-record.md#retro)); (3) a guard test that pins wiring is proven red by a mutation before it counts ([S3.2b-1 retro](s3-2b-1-record.md#retro)).
- **The one-module check** ([ADR-0001](../architecture/adr/ADR-0001-module-and-package-layout.md) §1: the tracked `go.mod` files are exactly `go.mod` and `gate/proofs/go.mod`) has no sprint: S1.1b's slim scope did not build it and S1.1c's list does not name it.
- **Known issue S2.2a-1** looks resolved by PR #13 (the fuzz rows are blocking): close it, or keep it for the nightly-only cadence ([known issues](../product/known-issues.md)).

## The status log

*The plan's one status line as it grew, 2026-11-01 to 2026-11-06, split at each annotation; the newest is last. The current line is [sprint-plan §Status](sprint-plan.md#status).*

- **Architecture baseline RATIFIED and review-hardened (2026-11-01). Phase 1 decomposition APPROVED (v2, 2026-11-02) and recalibrated the same day (v3, APPROVED: 23 sprints, every card ≤ 13 points by an item-by-item estimate, ~270 points, forecast 46–69 weeks) → next: S1.1a's plan check (G0 for the first sprint).**
- *(Annotated 2026-11-02: S1.1a's plan check is answered, and S1.1a is split into S1.1a-1 + S1.1a-2 (24 sprints) → next: S1.1a-1's design note and G1.)*
- *(Annotated 2026-11-02: S1.1a-1 passed G1 → next: the coordinator's first commit, then wave 1.)*
- *(Annotated 2026-11-03: S1.1a-1 merged and closed (G5): the local gate, coverage and the event kernel are on main → next: S1.1a-2's plan check (G0), hooks and CI.)*
- *(Annotated 2026-11-03: S1.1a-2 passed G0 → next: its design (G1), then the build in the main session.)*
- *(Annotated 2026-11-04: S1.1a-2 built; CI green on Linux; PR #2 awaits the owner's check and merge → next: S1.1b.)*
- *(Annotated 2026-11-04: S1.1a-2 merged; S1.1b slimmed by the owner to the engine-facing checks (`archcheck`); the rest → S1.1c → next: S1.2, the model kernel.)*
- *(Annotated 2026-11-04: S1.2 merged (the model kernel, spec rev 3) → S2.1, the parser, on participle v2 by the owner's decision.)*
- *(Annotated 2026-11-04: S2.1 built: every corpus file parses to a locked model, every invalid file to its exact code and position → PR for the owner → next: S2.2a.)*
- *(Annotated 2026-11-04: S2.2a built on top of S2.1's branch: the grammar differential is green over all 63 corpus files and red on a planted precedence swap; `internal/expr` types every §5.10 form and runs the scalar forms without allocating; the nightly tier, the fuzz row and the `python-package` kind are in → PR after #5 merges → next: S2.2b, the validator.)*
- *(Annotated 2026-11-04: S2.1 and S2.2a merged and closed → S2.2b passed G0 (slim with an architecture review, by the owner) → next: its design note, reviewed, then the build.)*
- *(Annotated 2026-11-04: S2.2b built: every reachable [static] code fires on QA's independent programs at its position with its fix text, the examples are clean, T4 green → PR for the owner → next: S3.1.)*
- *(Annotated 2026-11-04: S2.2b merged and closed; S3.1 split by the owner into S3.1-1 (the clock) and S3.1-2 → S3.1-1 passed G0 → next: its design note, reviewed, with QA's tests first.)*
- *(Annotated 2026-11-04: S3.1-1 merged and closed (PR #8): the clock service per the owner's D1–D8, the metrics counters, the `semantics` gate row → next: S3.1-2's plan check. Open for the owner after the roadmap review of 2026-11-04: where S1.1c (the gate extras) sits — recommended right after S3.3; a re-forecast now — eight sprints (about 80 points) took three days against the 2–3 weeks per sprint forecast; GitHub branch protection (`strict-gate` required on `main`) is still to be set.)*
- *(Annotated 2026-11-04: S3.1-2 built — tables read as at the event's arrival (owner D1), `in` / `not in`, the ingest flag on Unicode 17.0.0, the schema decoder with a strict RFC 3339 reader, the named purity exceptions; the break-it's seven findings fixed; T4 green → PR for the owner → next: S3.2a.)*
- *(Annotated 2026-11-04: S3.1-2 merged and closed (PR #9); S3.1 is complete → next: S3.2a's plan check. Still open for the owner: where S1.1c goes, a re-forecast, branch protection.)*
- *(Annotated 2026-11-05: S3.2a's plan check answered — S3.1's routine; split into S3.2a-1 (the replay format layer, partitions) and S3.2a-2 (the runtime core); S1.1c placed after S3.3; Phase 1 re-forecast at about 1–4 weeks → S3.2a-1 passed G0 → next: its design note, reviewed, with QA's tests first. Still open for the owner: branch protection.)*
- *(Annotated 2026-11-05: S3.2a-1 built: the replay format layer (fixtures and recordings, the virtual-clock driver, the canonical encoder and trace) and partitions (the TTL purge, the state registry, the timer invariant). Under owner D1 the clock finalizes an instant only once it is past it, which fixed an older defect, retired the seal, and made walk timing irrelevant. Exits 1, 2 and 4 are met; the break-it found 0 blockers and its 2 should-fix are fixed; a flaky gate test was fixed here (owner). T4 green → PR for the owner → next: S3.2a-2, the runtime core.)*
- *(Annotated 2026-11-05: S3.2a-1 merged and closed (PR #10) → next: S3.2a-2's plan check (its forwards are on the S3.2a card). Still open for the owner: branch protection, and the lasting-rule proposal of S3.1-2's retro.)*
- *(Annotated 2026-11-05: branch protection — no required status check, by the owner; the `protect-main` ruleset stays. Still open: the lasting-rule proposal.)*
- *(Annotated 2026-11-05: S3.2a-2's plan check answered — the drain barrier's mechanism and one shared ingest step are built here (about 7.5 points) → S3.2a-2 passed G0 → next: its design note, reviewed, with QA's tests first.)*
- *(Annotated 2026-11-05: S3.2a-2 built: the runtime core. It has one router, shards with bounded mailboxes and a shared non-blocking handoff, sink flushers, table writes in intake order, the drain barrier that ends an app's input, and consistent stats; the replay driver runs on the shared ingest step; a body's panic resets its key. Exit 3: saturation 50/50 under -race with a CPU hog per core. The break-it's blocker and should-fix are fixed, as is QA's re-drain defect; T4 green → PR for the owner → next: S3.2b.)*
- *(Annotated 2026-11-05: S3.2a-2 merged and closed (PR #11); S3.2a is complete. Before S3.2b: a small PR fixing the nightly build's summary and pulling the race row forward (owner) → next: S3.2b's plan check.)*
- *(Annotated 2026-11-06: the nightly fix (PR #12) and blocking fuzz rows (PR #13) merged; S3.2b's plan check answered — a plain filter rule alerts as soon as its event is accepted; S3.1's routine; split into S3.2b-1 (the plan, replay through the runtime, rate limits, the material) and S3.2b-2 (arrival time and the recorder, the invisible-character flag, error semantics, goleak and the tripwire, the two-OS proof on macOS) → S3.2b-1 passed G0 → next: its design note, reviewed, with QA's tests first.)*
- *(Annotated 2026-11-06: S3.2b-1 passed G1 — design v2 after the architecture review (2 blockers, 7 should-fix, 3 later) and QA's 34 tests, red on the stub; the owner's D1–D5: a plain rule forgets keys on its own stream's time, example 07 per tenant, two new deploy errors (a derived `ts` must be a timestamp; a nullable select into a must-have field), both here; about 10 points → next: the build in the main session, and QA's material for 07 and 09.)*
- *(Annotated 2026-11-06: S3.2b-1 built — the planner, replay through the runtime (Sync), rate limits, plain rules on arrival with keys expiring on their stream's admission (D1), two new deploy errors (D3, D4), 07 per tenant with its expect blocks; T2 and T3 green; exits 1 and 5 met. The combined review (0 blockers) and the break-it (1 blocker: two keys of one stream merged, alerts lost) were answered in one fix round, with E-PART-004 refusing a stream keyed twice. The owner chose multi-part keys `(k, j) of S` with no part limit, and D3's error at the field, both for S3.2b-2, and a split into parts of about 10 points each → next: T4, then the owner's check.)*
- *(Annotated 2026-11-06: S3.2b-1 merged and closed (PR #14, 40 commits) → next: S3.2b-2's plan check — its card's items 3–5, 7, 8 and the recorder, plus multi-part partition keys `(k, j) of S` with their language-design step and D3's error at the `ts` attribute (owner), about 9.5 points.)*
