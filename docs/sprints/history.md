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
| [S3.2b](#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time) | S3.2b-1, S3.2b-2 | [S3.2b-1](s3-2b-1-record.md), [S3.2b-2](s3-2b-2-record.md) | #14, #19 |
| [S3.3](#s33--the-deploy-pipeline-and-the-conformance-runner--clock-freeze) | S3.3-1, S3.3-2 | [S3.3-1](s3-3-1-record.md), [S3.3-2](s3-3-2-record.md) | #23, #28 |
| [S1.1c-1](#s11c-1--the-law-manifest-and-the-law-δ-block-the-current-sprint-marker-and-the-not-built-expiry-the-dependency-checks-the-error-code-counts) | S1.1c-1, S1.1c's first half (S1.1c-2 open) | [record](s1-1c-1-record.md) | #31 (and #30, the recorder fix it met) |

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


### S3.2b — One runtime, live and replay: rate limits, errors, arrival time

| | |
|---|---|
| **Goal** | A plain filter rule wired through `internal/plan` runs live (sharded) and in replay (the same runtime, one shard, stepped synchronously) with identical per-key output, and rate limiting, errors, arrival time and the invisible-character flag behave exactly per SEMANTICS. |
| **Routine · size** | Full loop (concurrency; changes what rules emit) · 13 SP (items 12.5) |
| **Depends on** | S3.2a |
| **In scope** | 1. `internal/plan` (minimal): the `StreamNode` graph consuming the validated graph, the routing table, filter-rule operator chains [2]. · 2. **The replay drive loop = the runtime with one shard and a synchronous step API** (ADR-0005 §3), re-running S3.1's and S3.2a's lines through it [1.5]. · 3. **`time arrival`** (owner 2026-11-02): live stamping at ingest; replay of the recorded stamp [1]. · 4. **`text_invisibles` engine-owned** (owner 2026-11-02): always computed; a payload value ignored, overwritten and counted [0.5]. · 5. **Error semantics** (SEMANTICS §10): per-rule expression errors; the stream error policy; null key; unknown attributes skipped and counted [1.5]. · 6. The emission stage with **rate limiting** (`output first every`; anchors in `partition.Instance`; suppressed outputs counted) and the three §8 harness ports [2]. · 7. Race step + goleak proof; the metric-tripwire step (outbox ceiling blocking); `allocgate` ingest → filter [2]. · 8. The two-OS determinism proof [1]. · 9. **Test material for 07 and 09** (QA, before the engine runs): fire-path and near-miss expect blocks — 09 gains a second event inside its rate-limit interval; they run green in S3.3 [1]. |
| **Not in scope** | `cep.Deploy`, the connector contracts and the conformance runner → S3.3. |
| *Forwarded* | *(Annotated 2026-11-05, owner: item 7's race step — `go test -race ./...` on every PR, with its planted-race proof — and a nightly 20× race rerun of `internal/runtime` land before this sprint, in the nightly-fix PR; item 7 keeps goleak and the metric tripwire. From S3.2a-2: the allocation budget for ingest → shard, `time arrival` stamping (item 3), randomized live/replay equivalence (exit 1).)* |
| *Forwarded* | *(Annotated 2026-11-04, from S3.1-2's review: item 4 — `text_invisibles` has one home, `Row.Invisibles()`; `expr.Build` compiles the name to a node reading it, so a declared slot cannot go stale (F6); whether an undeclared `text_invisibles` is readable, and that a declaration must be `bool`, are validator questions for the owner (F6, F7). Item 8 — the decoder's allocation budget per event (one per non-empty string value, two per object value; F12) and ADR-0010 §1's annotation.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. The same log run live (N shards) and in replay (one shard) gives identical per-key output order — **proof:** the live/replay equivalence test. |
| | 2. Two rules on one stream: one divides by an attribute that is 0 on one event; the other still fires on that event; per-rule `errored_events` counts it; a null key, a missing non-optional attribute, an unknown attribute and a failed `cast` are each counted where SEMANTICS §10 says — **proof:** the error-semantics table test. |
| | 3. A payload without `text_invisibles` gets the computed flag; a payload saying `false` over text containing a zero-width space yields `true` and increments the override counter — **proof:** the ingest-flag test. |
| | 4. A `time arrival` stream: live events are stamped at ingest in arrival order; replaying the recorded log reproduces the output byte for byte — **proof:** the arrival-time test. |
| | 5. A second output inside the rate-limit interval is suppressed and counted; a key-less rule uses one global anchor — **proof:** the three §8 ports. |
| | 6. A planted data race, a planted goroutine leak, and a single `Advance` emitting past the outbox ceiling are each red — **proof:** three proof rows. |
| | 7. The two-OS byte diff is green and catches a planted map-iteration nondeterminism — **proof:** the T4 proof row. |
| | 8. Zero allocations per event from ingest through filter evaluation — **proof:** `allocgate`. |
| **Owner's check** | *By reading:* the error-semantics table and the live/replay equivalence scenario. |
| **Split line** | **S3.2b-1** = items 1, 2, 6, 9 (plan, replay loop, rate limits, material); **S3.2b-2** = items 3–5, 7, 8. |

*How it runs (owner, 2026-11-06, §5 "S3.2b's plan check"):* S3.1's routine — QA's tests first and independent; an architecture review of each design note; the build in the main session; a QA break-it. As since S2.2b, the plan check had no separate review pass; the reviews sit on the design notes. Split along the card's line, two sprints and two PRs, one at a time:
- **S3.2b-1** = items 1, 2, 6, 9, plus the on-arrival path for plain rules (owner Q1) → exits 1, 5; about 7 points. Exit 1 carries S3.2a-2's forward: randomized live/replay comparisons.
- **S3.2b-2** = items 3–5, 7, 8, plus the recorder (S3.2a-1's D3: "S3.2b's recorder writes them"), which exit 4 needs → exits 2, 3, 4, 6, 7, 8; about 6 points (item 7 is 1 point, its race step having landed in PR #12). *(Annotated owner, 2026-11-06, S3.2b-1's review and break-it, §5: S3.2b-2 also carries **multi-part partition keys**, written `(k, j) of S`, with no limit on the number of parts and their language-design step first (about +2.5), and **D3's error at the `ts` attribute itself**, model attributes gaining a source position (about +1): about 9.5 points. Nothing decided is dropped.)*

*The plan check's readings (coordinator, against `main` @ a5dbc2d):*
1. **There is no separate validated-graph type:** `validate.Check` returns the checked `*model.App`, which `internal/plan` consumes. `plan` may not import `runtime` (`tools/importcheck/rules.json`), so `plan` owns its description types and the runtime builds from them; `runtime.Rule`, written "as S3.2b's planner will describe it", adapts.
2. **Replay through the real runtime** (ADR-0005 §3) calls the runtime's own router step and shard step on one thread, with one shard and no goroutines (ARCHITECTURE §6: playback owns none). Today `runtime.Start` always starts goroutines, and the playback driver runs the shared ingest step but no rules.
3. **Every rule goes through partition's release buffer today**, so a plain rule's alert waits for its stream's clock; on a stream with no `idle after` (example 07) it can wait for the next event. Owner Q1 decides it.
4. **Malformed input:** a malformed row from Go code stays refused to its caller (S3.1-2's D2, the Go API boundary). A payload is decoded by schema, and its faults, unknown keys and a payload's own `text_invisibles` are counted per stream (SEMANTICS §10, item 5). Logging the cause is S5.2's item 5.
5. **Neither arrival stamping nor the invisible-character flag is computed anywhere yet**, live or in replay: the router never sets a `time arrival` event's time, and nothing calls `text.IngestFlags`. Items 3 and 4 build both.
6. **`allocgate` stays `testing.AllocsPerRun` tests run by the `unit` row** (S2.2a's decision). The decode and encode gates are S5.2's item 8; S3.1-2's decode budget (F12) and ADR-0010 §1's annotation land in S3.2b-2 with the hand-off budget (owner Q4).
7. **A derived stream's `ts` is the output timestamp,** and a `select` may widen `int` to `float` (`internal/validate`): the filter rule's projection follows both.

*S3.2b-2's plan check (coordinator, against `main` @ f32fd41; answered by the owner 2026-11-06, [decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-plan-check)):* S3.1's routine as for S3.2b-1 — QA's tests first and independent; an architecture review of the design note, in which multi-part keys get the language-design skill's full pass (semantics, syntax, evaluation); the build in the main session; a QA break-it. One sprint, one PR; about 11 points by item count (item 3 with the recorder 2, item 4 with Q3's validator rule 1, item 5 1.5, item 7 1.5, item 8 1, multi-part keys 2.5, D3's position 1, the rejected-line record kind 0.5), under the cap. Order: the model changes first (keys and the attribute position together, so the parser's goldens regenerate once), then ingest (arrival, the flag, the payload intake, the recorder), then the gate rows. The readings:
1. **No live event gets a `time arrival` stamp:** the router passes its wall stamp to the stream registry only, and the event keeps the `ts` its caller gave (`runtime.Ingest.Event`). The replay side reads recordings already (S3.2a-1's D3: start, arrival, tick, stop), but nothing writes one: `playback` encodes events only. The recorder sits on the router goroutine, the one place that sees the whole intake in order; the line encoding is `playback`'s.
2. **The invisible-character flag is computed nowhere:** `text.IngestFlags` has no caller and no expression node reads `Row.Invisibles()`; the decoder already reports a payload's own `text_invisibles`. The flag is computed once in the shared ingest step, so live and replay cannot differ.
3. **Per-rule expression errors and null keys are counted since S3.2b-1; nothing is counted per stream, and no live path takes a raw payload:** `runtime.Send` takes a typed event and refuses a malformed one (S3.1-2's D2, which stands). Item 5 adds an internal payload intake that decodes by schema and counts rejected payloads, unknown attributes and overridden flags per stream; the recorder writes a rejected payload as a "rejected" line (owner Q1).
4. **A partition key is one value end to end:** the grammar (`part_key: cname "of" cname`), the parser, `model.PartitionKey`, `plan.Rule` (one key attribute per input, one key type), the router's shard hash and `partition`'s map key and `Instance.Key`. Multi-part keys change each; the Python reference executor takes any key function, so a tuple serves and a new harness check pins "each combination is its own key". SPEC goes to rev 5.
5. **`model.Attr` has no source position;** adding one regenerates the parser's 39 stored model goldens (`internal/parser/testdata/golden`). *(Corrected 2026-11-06, while building the stub: `model.MarshalCanonical` never encodes a `Pos` field, so the position changes no golden; the multi-part key's new shape is what regenerates the goldens of partitioned programs.)*
6. **goleak is not in `go.mod`, and nothing counts one step's emissions,** so the outbox ceiling needs a runtime high-water mark as well as the T4 tripwire step, its ceilings (S3.2b's Q7) and a planted proof. The race proof landed in PR #12.
7. **Every CI job runs on `ubuntu-latest`, and the workflow generator has no per-row runner,** so the macOS job (S3.2b's Q5) needs one, with a committed replay output both systems must match byte for byte and the planted map-order proof.

### S3.2c — The spec cleanup: the language documents agree, and overflow is an error

| | |
|---|---|
| **Goal** | The language documents agree with each other and with the code, a spec-only reader finds every reserved word, and integer overflow never yields a wrong value: it is an expression error for that rule. |
| **Routine · size** | QA's tests first, one review, a break-it (owner, 2026-11-06) · 3 SP (items 2.5) *(4 SP, items 3.5, with items 4 and 5 — owner, 2026-11-07)* |
| **Depends on** | S3.2b |
| **In scope** | 1. **The five contradictions** of known issue S2.2b-1, fixed in SEMANTICS and errors.md to match SPEC and the code: §2's time-clause syntax; §2.1's and §11.10's `idle after` scope; E-SEL-001's "after the auto-filled `ts`"; §6.5's unmarked pattern; §6.4's `→ not B for D` form [0.5]. · 2. **The grouped-key evaluation's two notes:** E-IDLE-001's text says a plain rule — partitioned and rate-limited or not — needs no `idle after`; SPEC Appendix B lists every reserved word, pinned to `grammar.lark` by the keyword test [0.5]. · 3. **Integer overflow is an expression error for that rule** (owner, 2026-11-06) over every 64-bit operation — ints, durations, timestamps: checked arithmetic in `internal/expr`; a Python harness check, red first; SPEC rev 6 and SEMANTICS §10; known issues S2.2b-1 and S3.2b-1-1 closed [1.5]. *(Added 2026-11-07, owner ([decisions](decisions.md#owner-decisions-2026-11-07--s32cs-review-and-break-it)), in the fix round:)* 4. **`timestamp − timestamp` → duration**, checked for overflow; SPEC §5.10 lists every arithmetic form the engine accepts [0.5]. · 5. **The smallest int, `-9223372036854775808`, is a writable literal** [0.5]. |
| **Not in scope** | Overflow in the aggregates → S8 (they do not exist yet). A timestamp past year 9999 in an output → S5.1 (the sink's encoding). *(Annotated 2026-11-07, owner ([decisions](decisions.md#owner-decisions-2026-11-07--s32cs-review-and-break-it), 2): outside years 0000–9999.)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. Each of the five contradictions resolved in the text, and no validator verdict changed — **proof:** the owner's reading; the corpus and static-error tests unchanged and green. |
| | 2. SPEC Appendix B lists exactly `grammar.lark`'s reserved words — **proof:** the keyword test, red on a planted drift. |
| | 3. Every 64-bit operation that overflows is an expression error: the rule drops the event and counts it, and another rule on the same stream still fires — **proof:** an `expr` table test per operation at its exact boundary, a live test, the Python harness check. |
| | 4. No result that fits changes — **proof:** the same table test's in-range rows (`MaxInt64 − 1 + 1`, `MinInt64 / 1`, …). |
| | *(Added 2026-11-07, owner ([decisions](decisions.md#owner-decisions-2026-11-07--s32cs-review-and-break-it), 1 and 3), in the fix round:)* 5. `timestamp − timestamp` is a duration, exact where it fits and an expression error past either bound — **proof:** the `expr` test `TestTimestampMinusTimestampIsADuration`. · 6. `-9223372036854775808` is a writable literal in a param default and an expression, and nothing else out of range becomes writable — **proof:** QA's break-it test and the parser guards. |
| **Owner's check** | *By reading:* the SEMANTICS, SPEC and errors.md changes. *By running:* the overflow tests. |

*How it runs (owner, 2026-11-06, [decisions](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check)):* QA writes the overflow tests first, red against today's wrapping arithmetic; the coordinator writes a one-page design note and builds in the main session; one architecture review of the design and the code together; a short QA break-it on overflow; one fix round, under the race detector before T4; T4; the PR.

*The plan check's readings (coordinator, against `main` @ 53facc5):*
1. **All five contradictions stand as known issue S2.2b-1 lists them;** SPEC, errors.md and the validator agree with one another, so each fix is SEMANTICS's (and E-SEL-001's wording) catching up.
2. **E-IDLE-001's list** names a pattern with an absence, a window rule and a multi-stream rule; a plain rule is not on it, and the validator raises nothing for one — the text only has to say so.
3. **SPEC Appendix B points at `grammar.lark`'s NAME terminal** for the reserved words; `internal/parser/keywords_test.go` already pins the parser's keyword table to that terminal, so the same test can pin a list in SPEC.
4. **Every 64-bit arithmetic path wraps today:** `internal/expr`'s int `+ - * /` (the smallest int / −1 too) and unary `-`, `abs`, and the duration and timestamp kinds; only `cast` from float to int is range-checked already (S3.1-2's break-it B4).
5. **The Python reference executor has no general arithmetic;** the harness check needs a checked 64-bit helper in `docs/language/checks/executor.py`, written red first.

### S3.3 — The Deploy pipeline and the conformance runner — clock freeze

| | |
|---|---|
| **Goal** | A filter rule deployed through the public `cep.Runtime.Deploy` runs with its expect blocks executed as a deploy stage, the conformance runner provably fails on wrong expectations, and the clock contract is frozen under the gate. |
| **Routine · size** | Full loop (the public Go API) · 13 SP (items 13) |
| **Depends on** | S3.2b |
| **In scope** | 1. The public facade `cep` (minimal): `Runtime`, `Config`, `Deploy` (parse → validate → plan → expect sandbox → register), `AppHandle` `Start` / `Stop` (drain barrier to a fixpoint, `drained_pending`) / `Undeploy`, `Runtime.Close` (reverse deploy order), multiple apps per runtime, `Input` with **`SendRow` only**, the `Table` API, `BuildInfo` [4]. · 2. **The connector contracts as the leaf package `io` (package `cepio`)** — `Source`, `Sink`, `Mapper`, `RowWriter`, `EmitFunc`, `OutEvent`, `Mode`, and **the batch-close signal** (designed here; an API Δ) — plus `io/inmemory` [1.5]. · 3. **The expect sandbox in `internal/playback`**, called by `cep.Deploy` and `conformance/`: a fresh sandbox per block, `given table` seeding through table records [2]. · 4. The conformance runner `conformance/` (public API only); E-EXP-001 lifted from S2.2b's skip; the six planted wrong-expect proofs [2]. · 5. `apipartition` (`api/exports.golden`: `cep` + `io`) + `apidiff` against the merge base [1.5]. · 6. `gate/regen-baseline` + Tier-2 baselines [1.5]. · 7. `api/clock.golden` added to `gate/law.json`; **the clock contract freezes at this sprint's G5** [0.5]. |
| **Not in scope** | Patterns → S4.1. HTTP and the other connectors → S5.1, S5.2. |
| *Forwarded* | *(Annotated 2026-11-04, from S3.1-2, owner D3: an `optional` or `object` primary-key column is a deploy error with a new code, with its SPEC v0.1 change-log entry and an invalid example; owner D2: the Go API's row boundary refuses a non-finite float; break-it B5: an object value enters as its JSON text.)* *(Annotated 2026-11-05, from S3.2a-1, owner D7: an `object` partition key is a deploy error with a new code beside D3's, with its SPEC v0.1 change-log entry and an invalid example; until then `partition.NewRule` refuses it. And under D1, `Stop`'s drain uses `clock.StreamRegistry.Drain`: one past the max observed event time.)* *(Annotated 2026-11-05, from S3.2a-1's break-it: the replay reader skips an unknown attribute in an event's `attrs`, as live ingest does, so a typo in a hand-written expect block reads as null; the conformance runner should make an unknown attribute in an expect block a fault.)* *(Annotated 2026-11-05, owner at S3.2a-2's plan check: the drain barrier's mechanism moves to S3.2a-2; item 1's `Stop` wraps it. Item 1 shrinks by about 1 point (13 → 12).)* *(Annotated 2026-11-06, from S3.2b-1: the expect sandbox must define what a block's given lists mean as arrival — a list's order is its arrival order, and how several lists interleave (material gap M1; example 07's arrival-order block depends on it) — and that a block passes only when the outputs match the then-list in number as well as content (M2). `Deploy` must not mutate or reuse the checked model after `plan.Build`: a built plan compiles its bodies from it at `Start` (break-it L2).)* *(Annotated 2026-11-06, from S3.2b-2's design ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-design-after-the-architecture-review)), owner: `Deploy` has a recording setting, **off unless turned on**; when on, it attaches `playback.NewRecorder` to the app (`runtime.App.Recorder`).)* |
| **Entry criteria** | The standard ones. |
| **Exit criteria** | The done-done block, plus: |
| | 1. 07 and 09 pass their fire-path and near-miss expects through `Deploy` — **proof:** the conformance runner. |
| | 2. Each planted wrong expect — wrong value, missing output, extra output, wrong order, an output under an empty `then`, wrong timestamp — fails `Deploy` with E-EXP-001 naming the first divergence — **proof:** six proof rows. |
| | 3. Two apps in one runtime share no state; `Runtime.Close` stops them in reverse deploy order — **proof:** the multi-app isolation test. |
| | 4. Two `in` predicates on one table within one event see one snapshot while a concurrent `Table.Upsert` is held between them — **proof:** the live table-snapshot test. |
| | 5. An unlisted export turns `apipartition` red; `apidiff` reports a planted signature change — **proof:** proof rows. |
| | 6. `gate/regen-baseline` refuses to record a run that changes timestamps, ordering or counts, and records an additive one — **proof:** regen-baseline proofs. |
| | 7. A planted change to an exported `internal/clock` signature appears in the LAW Δ block — **proof:** the clock-golden proof. *(Annotated 2026-11-08, S3.3-2's plan check: `gate/law.json` and the LAW Δ block are S1.1c's ([owner, 2026-11-07](decisions.md#owner-decisions-2026-11-07--s33s-plan-check), 2); until then the `clockgolden` row (S3.3-1) turns the gate red on such a change, and its proof stands for this exit; the freeze is declared at S3.3-2's G5.)* |
| | *(Added 2026-11-08, owner ([decisions](decisions.md#owner-decisions-2026-11-08--s33-2s-plan-check), 1 and 2), at S3.3-2's plan check:)* 8. A mistake inside an expect block — an unknown stream or table, an unknown field, a wrongly typed value — fails `Deploy` with the new expect-block code at the mistake's position, before any block runs — **proof:** the validator's test programs, one per kind. · 9. A param override that changes a block's result fails `Deploy` with E-EXP-001 — **proof:** a facade test deploying 09 with `${lookback}` overridden. |
| **Owner's check** | *By running:* a 20-line Go program that embeds the engine, deploys 09 and feeds events through `SendRow`. *By reading:* `api/exports.golden` — the first public surface. |
| **Split line** | **S3.3-1** = items 1, 2, 5 (facade, contracts, API gate); **S3.3-2** = items 3, 4, 6, 7. |

*How it runs (owner, 2026-11-07, [decisions](decisions.md#owner-decisions-2026-11-07--s33s-plan-check)):* **S3.3-1** (about 8 points; record [s3-3-1-record.md](s3-3-1-record.md)): the full loop with the design panel. QA's tests first and independent, in a worktree; a design note by the coordinator; the blind three-reviewer panel (a breaker re-implementing from the sources, a challenger measuring the alternatives, the architect with the-fool), in two waves of at most two agents, then QA for testability; the owner's decisions; the build in the main session, test-first; T2, T3; the architecture review of the combined build and a QA break-it; one fix round under the race detector; T4; one PR. **S3.3-2** (about 7 points): its own plan check after S3.3-1 merges. **Binding from the plan check:** the export check and a clock-golden row are gate rows of their own now (decision 2); `apidiff` runs at T4 against the merge base, `golang.org/x/exp` a new build-time dependency (decision 3); an expect block's lists interleave by time, each list in its written order, ties to the list written first (decision 4; built in S3.3-2).

*How S3.3-2 runs (owner, 2026-11-08, [decisions](decisions.md#owner-decisions-2026-11-08--s33-2s-plan-check)):* about 7.5 points (record [s3-3-2-record.md](s3-3-2-record.md)): the full loop with the design panel, as S3.3-1. **Binding from the plan check:** a block runs with the deployment's param values and a different result refuses the deploy (decision 1, ≠ R); a mistake inside a block is its own static code, found before any block runs (decision 2: one new entry in `errors.md`, a line in SPEC's change log, no grammar change). In scope with the card's items 3, 4, 6, 7 and the forwards: known issues S3.3-1-1 (`apipartition` names a method `<Type>.<Method>`) and S3.3-1-2 (`Close` refuses new `Add`s before its snapshot). *(Annotated 2026-11-09, the design after the panel (owner, [decisions](decisions.md#owner-decisions-2026-11-09--s33-2s-design-after-the-panel)): re-sized at about 9.75 points, one sprint; "not built yet" is `errors.ErrUnsupported`; E-EXP-002 covers every kind the panel found; E-EXP-001 points at the expected record that did not come out; the baselines are keyed by content, a SPEC revision's value changes recorded only for named blocks; `optional` on a time attribute refused (E-TIME-003, SPEC rev 10); the not-built expiry moves to S1.1c. Design: [s3-3-2-design.md](s3-3-2-design.md) v2.)*

*S3.3-2's plan check readings (coordinator, against `main` @ 1fa3a5b):*
1. **Three corpus files build today:** 07 (six blocks), 09 (four) and 10 (five) parse, validate and plan; 01, 06, 08 and both torture files wait for patterns (S4.1), 02, 03, 04 for windows (S8), 05 for text operators (S7). The runner skips each unbuilt file with the sprint that builds it, and a skip is checked to be `plan.ErrNotBuilt`.
2. **Nothing runs an expect block yet, and nothing checks one:** the parser lowers blocks into `model.Rule.Expects` (`Given{Table, Name, Records}`, `Then{Name, Records}`); the validator never reads them, so a misspelt field reads as null when replayed (S3.2a-1's break-it forward; decision 2's code).
3. **Playback has the sandbox's engine:** `playback.NewPlanDriver` steps a plan over records (events, table loads, ticks) with its own tables; `Driver.End` drains as `Stop` does (one past the max event time, D1). SPEC §6.8 says a block checks *the rule's* outputs, so a sandbox runs the owning rule alone, its `given` lists as its inputs.
4. **Example 09's blocks assume `${lookback}`'s default:** under decision 1, the sandbox runs the plan built with the deployment's overrides.
5. **The clock freeze is half built:** `api/clock.golden` and the `clockgolden` row exist (S3.3-1); `gate/law.json` and the LAW Δ block are S1.1c's, so item 7 is the freeze's declaration at G5 (exit 7 annotated).
6. **Tier 2 (qa-strategy §3) has no code:** no `gate/regen-baseline`, no baseline directory; the baselines are the full canonical outputs of the 15 runnable blocks, locked; the tool re-runs Tier 1 first and classifies a diff (`additive-new-rule`, `spec-rev` record; `timestamp-drift`, `ordering-change`, `count-change` refused).
7. **Size:** the sandbox 2, the expect-block checks and code 0.75, the runner and six proofs 2, the baselines and tool 1.5, the freeze 0.25, the two known issues 0.75 — about 7.5, under the cap; no split.

*The plan check's readings (coordinator, against `main` @ a8a63ad):*
1. **The runtime has no per-app lifecycle:** `runtime.Start(cfg, apps)` takes every app up front and `Runtime.Close` stops the whole runtime (closes `stopped`, waits on the group); `Drain(app)` is per app. Item 1's `Deploy` into a running runtime, `AppHandle.Start`/`Stop`/`Undeploy` and `Close` in reverse deploy order need the runtime to register and unregister apps while it runs — the part of item 1 S3.2a-2's plan check did not foresee (about +1.5 points, hence the split).
2. **Nothing of the public surface exists:** no `cep`, `io`, `conformance` or `api/`; `tools/` holds `archcheck`, `importcheck` and `shape`, no `apipartition`; `golang.org/x/exp` is not in `go.mod`. `E-EXP-001` is in the validator's catalog marked for S3.3.
3. **The Go boundary is built:** `runtime.Send` refuses NaN and infinities (S3.1-2's D2), `recordable` refuses a value a recording cannot copy (S3.2b-2's D1), an object enters as its JSON text (B5). The facade's `SendRow` and the `Table` API land on these; the `cepio.RowWriter` boundary converts once, on the cold path.
4. **D3 and D7 are enforced below the validator only:** `partition.NewRule` refuses an object key part, and `table.New` carries no primary-key type rule; both become deploy errors with codes in the validator (SPEC change log, invalid examples), as forwarded.
5. **Playback has the pieces of the sandbox but no sandbox:** `NewPlanDriver`, `Replay`, `Reader`, `Recorder`, `Trace`; no code runs an `expect` block, interleaves `given` lists (M1) or compares outputs to a then-list in number (M2). S3.3-2's.
6. **The recording setting:** `runtime.App.Recorder` is nil unless set, so "off unless turned on" (S3.2b-2's D4) is `Deploy`'s option naming an `io.Writer`; `playback.NewRecorder` is the writer.
7. **`Deploy` must not mutate the checked model** (S3.2b-1's L2): `plan.Build` consumes the `*model.App` `validate.Check` returns; the facade keeps neither, and a built plan compiles its bodies at `Start`.

### S1.1c-1 — The law manifest and the LAW Δ block, the current-sprint marker and the not-built expiry, the dependency checks, the error-code counts

*The first half of S1.1c ([owner, 2026-11-10](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check), decision 3). S1.1c-2's card is written at its own plan check. Plan check: [s1-1c-1-plan-check.md](s1-1c-1-plan-check.md).*

| | |
|---|---|
| **Goal** | Every change to a law file is loud where the owner reads at merge — one line per file with what changed, never silently absent, never a silent "none" — and the gate knows which sprint is running: a corpus file still marked "not built" when its sprint arrives is red, a go.mod module nobody approved is red, and the language spec's own count of its error codes is checked. |
| **Routine · size** | Full loop without the semantics panel (owner, decision 7) · 8 SP (items ≈ 5.5) against the 2-week cap |
| **Depends on** | S3.3 (merged: PRs #23, #28; main 9cd4188 green) |
| **In scope** | 1. `gate/current-sprint` (created at G0 with `S1.1c-1`), the sprint-id grammar and the plan table's order as a pure package `tools/sprintid`, the not-built list as data (`conformance/notbuilt.json`, a law path), the expiry test in a **T4-only** row `conformance-expiry`, the marker test at T2 (the marker is the first open plan row), the G0 line in the sprint model and the plan-check template [1]. · 2. `gate/law.json` (every law path ADR-0015 §1.3 names that exists, with its kind and, for the language and architecture documents, its flag), the `lawdelta` row (T1–T4, the `lint` job, the merge base with `main`; **red** with no merge base), the LAW Δ block as the summary renderer's own required output — locally and in CI's aggregated summary, from the row's record — with a semantic one-liner per structured file (edges, registry rows, exports, modules, ceilings, baseline blocks, not-built entries, tool versions, the marker, the manifest itself) and the owner-decision flag on the documents; liveness (a listed path git does not track is red) and the §1.3 pin test [3]. · 3. The `deps` row (T2–T4): every module in the build list is in `tools/deps-allowlist.json` (normalised to one module per entry); each module's licence file, read from the module cache, is in the allowed set and agrees with its entry; an entry for a module no longer in the graph is red; the tracked `go.mod` files are exactly `go.mod` and `gate/proofs/go.mod` (ADR-0001 §1) [1.25]. · 4. `docsync` (d): SPEC §9's counts (codes, parse, static, invalid files) and every code it mentions agree with errors.md and `invalid/`; red first on today's text, SPEC's line fixed [0.25]. |
| **Not in scope** | The rest of S1.1c → S1.1c-2: the strict golangci set with the suppressions checker and the allowlist grammar, the `testhygiene` fence, the fragment test, `docsync` (a), registry-lint and the teeth meta-test rebuilt as-built (the `instruments` section of `law.json`), `govulncheck`, advisory aging, the weekly canary. `todocheck`'s basic form and the build-tag law: with their first subject. |
| **Entry criteria** | The standard ones ([§4](00-sprint-model.md#4-entry-and-exit-criteria)), plus: QA's three proof drivers and two red Go tests exist before the build (wave 0 of the plan check). |
| **Exit criteria** | The done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)), plus these acceptance lines: |
| | 1. A planted edit to a law file on a branch appears first in the summary as `<path> +a/−b — <one-liner>`; an empty delta prints `LAW Δ: none` — **proof:** `gate/proofs/drivers/test_lawdelta_planted.py`. |
| | 2. A run with no merge base is red naming it; a listed path git does not track is red; a render without the block is impossible (`TypeError`); a missing `lawdelta` record in CI is a named red row — **proof:** the same driver, `gate/runner/tests/test_lawdelta.py`, `test_summary.py`, the `summarize` fixture tree. |
| | 3. Each structured kind yields its one-liner on a planted pair; every tracked path ADR-0015 §1.3 names is in `law.json` — **proof:** `gate/runner/tests/test_law.py`, `test_law_pin.py`. |
| | 4. A not-built entry whose sprint row is at or before the marker's row is red at T4 naming file, construct and sprint; a missing or unparseable marker, or an id naming no plan row, is fatal — **proof:** `gate/proofs/drivers/test_notbuilt_expired.py` (marker `S4.1` → three names; `S1.1c-1` → pass), `conformance/notbuilt_test.go`, `tools/sprintid`'s table tests (S3.3 < S1.1c < S4.1 by the table; S9.2 < S10). |
| | 5. The marker is the first open plan row, else red at T2 — **proof:** `TestCurrentSprintIsTheFirstOpenRow`. |
| | 6. An unlisted module, a licence outside the allowed set or missing or disagreeing with its entry, a stale entry, a third tracked `go.mod` — each red naming it — **proof:** `gate/proofs/drivers/test_deps_planted.py` (six cases), `gate/runner/tests/test_deps.py`. |
| | 7. SPEC §9's four counts and its mentioned codes agree with errors.md and `invalid/` — **proof:** `internal/parser/errorcounts_test.go`, red first on main's "67 codes". |
| | 8. Every new row green on the real tree at T2–T4; `proofs` `min_wants` 24 — **proof:** the T4 run. |
| **Owner's check** | *By reading:* this PR's own LAW Δ block on its CI summary (law.json new, +3 registry rows, the workflow regenerated, the allowlist normalised, SPEC §9's count flagged, the ADR annotations flagged, notbuilt.json and the marker new). · *By running:* `make strict-gate` with `gate/current-sprint` set to `S4.1` — the `conformance-expiry` row red naming 01, 08 and torture/comments — then restored. · *By hand:* the G0 line in the sprint model; the marker reads `S1.1c-1`. |
| **Split line** | Not needed (≈ 5.5 points). If the plan check finds more: **S1.1c-1a** = items 1 and 4; **S1.1c-1b** = items 2 and 3. |

## Resolved owner items

*Moved from the plan's "Still open for the owner" once done.*

- *Approval of the v3 recalibration — **done** (2026-11-02); the sprint references in ADR-0005 §1, ADR-0012 §1, ADR-0015 and qa-strategy §4 are updated to v3's ids.*
- *Owner action — **done** (2026-11-02, verified via the GitHub API): ruleset `protect-main` is active on the default branch with deletion and force-push (non-fast-forward) blocked and no bypass actors. S1.1a adds the summary job as a required status check to the same ruleset once its CI has run.* *(Annotated 2026-11-05: the required status check is dropped by the owner — §5, "branch protection".)*

*Resolved 2026-11-06 ([owner](decisions.md#owner-decisions-2026-11-06--the-docs-restructure)): the three lasting rules joined the sprint model; the one-module check joined S1.1c; S2.2a-1 was closed.*

- **Three lasting-rule proposals from retros:** (1) a design claim about a library's behaviour is checked with a scratch program before review ([S3.1-2 retro](s3-1-2-record.md#retro)); (2) a new CI workflow is run once by hand in the sprint that adds it ([S3.2a-2 retro](s3-2a-2-record.md#retro)); (3) a guard test that pins wiring is proven red by a mutation before it counts ([S3.2b-1 retro](s3-2b-1-record.md#retro)).
- **The one-module check** ([ADR-0001](../architecture/adr/ADR-0001-module-and-package-layout.md) §1: the tracked `go.mod` files are exactly `go.mod` and `gate/proofs/go.mod`) has no sprint: S1.1b's slim scope did not build it and S1.1c's list does not name it.
- **Known issue S2.2a-1** looks resolved by PR #13 (the fuzz rows are blocking): close it, or keep it for the nightly-only cadence ([known issues](../product/known-issues.md)).

*Resolved 2026-11-06 ([owner](decisions.md#owner-decisions-2026-11-06--the-open-items-after-s32b-2)): the two lasting-rule proposals from S3.2b-2's retro joined the sprint model; the spec-cleanup pass was placed before S3.3 as S3.2c, with integer overflow decided as an expression error.*

- **Two lasting-rule proposals from S3.2b-2's retro:** a fix round's changes run under the race detector before T4; a sprint that adds a CI job opens its PR as a draft to get that job's first run.
- **Language questions for a spec-cleanup pass:** the five contradictions (known issue S2.2b-1) and integer overflow (S3.2b-1-1).

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
- *(Annotated 2026-11-06: S3.2b-2 merged and closed (PR #19, 35 commits) — multi-part partition keys `(k, j) of S` (SPEC rev 5), D3's error at `ts`, arrival stamping, the recorder, the ingest flag, per-stream error counts, the Go boundary's refusals, goleak, the metric tripwire and the two-OS determinism proof; S3.2b is complete → next: S3.3's plan check.)*
- *(Annotated 2026-11-06: the owner placed the spec cleanup before S3.3 as S3.2c, decided integer overflow as an expression error and adopted the two retro rules → next: S3.2c's plan check.)*
- *(Annotated 2026-11-06: S3.2c's plan check answered — the card as written, QA's tests first with one review and a break-it, overflow an expression error over every 64-bit operation → S3.2c passed G0.)*
- *(Annotated 2026-11-07: S3.2c built — SPEC rev 6, checked 64-bit arithmetic, Appendix B; T2 and T3 passed; the combined review (0 blockers) and the break-it (0 blockers) in; the owner built `timestamp − timestamp` and the smallest int's literal into the fix round → S3.2c in its fix round.)*
- *(Annotated 2026-11-07: S3.2c in its fix round — the review and the break-it found no blocker; the owner added `timestamp − timestamp` and the smallest int's literal → the fix round's verification, T4, the PR.)*
- *(Annotated 2026-11-07: S3.2c merged and closed (PR #21, 20 commits; close-out PR #22) — SPEC rev 6, overflow an expression error, `timestamp − timestamp`, the smallest int's literal, the five contradictions corrected; known issues S2.2b-1 and S3.2b-1-1 closed → next: S3.3's plan check.)*
- *(Annotated 2026-11-07: S3.3 split at its plan check — S3.3-1, the public Go API, about 8 points, passed G0; S3.3-2 (the expect sandbox, the runner, the baselines, the clock freeze) after it → next: QA's tests first and the design note, then the design panel.)*
- *(Annotated 2026-11-08: S3.3-1 merged and closed (PR #23, 45 commits; close-out PR #24) — the public Go API: `cep`, the connector contracts `io` with `io/inmemory`, several apps in one running engine, the API gate → next: S3.3-2's plan check.)*
- *(Annotated 2026-11-08: S3.3-2 passed G0 — the expect sandbox, the conformance runner, the baselines and the clock freeze, about 7.5 points, the full loop with the design panel; a block runs with the deployed param values (owner) and a mistake inside a block is its own deploy error → next: QA's tests first and the design note, then the panel.)*
- *(Annotated 2026-11-09: S1.1a-1 to S3.3-1 merged (PRs #1–#24, the last on 2026-11-08). S3.3-2 passed G1 (2026-11-09): design v2 after the blind panel, the owner's eight design decisions, re-sized at about 9.75 points; QA's first tests written red → next: QA's second lane of tests, then the build.)*
- *(Annotated 2026-11-10: Phase 1 in progress: S1.1a-1 to S3.3-2 merged (PRs #1–#28, the last on 2026-11-10): S3.3 is closed — the public Go API, the expect sandbox, the conformance runner and the Tier-2 baselines — and the clock contract is frozen (main d14814a) → next: S1.1c's plan check (the gate extras), then S4.1, the pattern engine.)*
- *(Annotated 2026-11-11: Phase 1 in progress: S1.1a-1 to S3.3-2 merged (PRs #1–#28, the last on 2026-11-10); the clock contract is frozen (main d14814a). S1.1c-1 passed G0 on 2026-11-11 (branch `s1-1c/integration` @ 41c2825: the law commit — `gate/current-sprint`, `gate/sprints.json`, `gate/law.json`, `conformance/notbuilt.json`, the rows `lawdelta`, `deps`, `notbuilt` as named stubs): the law manifest and the LAW Δ block, the current-sprint marker and the not-built expiry, the dependency checks, the error-code counts, the full loop without the panel → next: QA's proofs first (W0) and the design note (G1); then S1.1c-2 (the strict lint set with the suppressions checker, the fragment test, docsync (a), the teeth, govulncheck, advisory aging and the canary), then S4.1.)*
