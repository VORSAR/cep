# S1.1c-1 — plan check (draft, 2026-11-11; the owner's answers are recorded in the sprint record, `s1-1c-1-record.md`, opened at G0)

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ 9cd4188 (the code; the docs as of `s1-1c/integration` @ 81834f6, which changed only `docs/`). Every code claim cites `file:line` on that commit; "inferred" marks a reading not proven by a run. Nothing here is an owner decision until the owner answers §11. The scope itself was decided on 2026-11-10 ([decisions](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check)); this check turns decision 3's S1.1c-1 half into lanes, seams and proofs.*

```
Sprint S1.1c-1 — the law manifest and the LAW Δ block, the current-sprint marker and the not-built expiry,
                 the dependency checks, the error-code counts · routine: full (no semantics panel) · SP ≈ 5.5 as sized · cap 2 weeks
Entry (G0)   ☐ card complete and approved ☐ dependencies merged, main green (S3.3: PR #28, main 9cd4188)
             ☐ fits the cap, lanes ≤ 8 days ☐ test material exists, or making it is an item (QA's wave 0)
             ☐ reviewed by the architect (§10) ☐ reviewed by QA (§10) ☐ answered by the owner (§11)
```
*At G0 the full done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)) is copied into the sprint record, and the G0 commit creates `gate/current-sprint` with the line `S1.1c-1` (decision 8).*

SP as sized: item 1 (marker, `tools/sprintid`, the expiry row, the marker test) 1 · item 2 (`gate/law.json`, `lawdelta`, the block in both renderers, nine semantic parsers, CI history, liveness) 3 · item 3 (`deps`: allowlist, licence, one module) 1.25 · item 4 (the counts test, SPEC §9 fixed) 0.25 — **≈ 5.5**; items 2 and 3 are inferred from the row and driver sizes of S3.3-1/S3.3-2 (`apipartition` ≈ 2, `regen-baseline` ≈ 1.5).

## 0. What the code says that the plan does not (read first)

1. **The summary has no slot for a LAW Δ block, and it is rendered from two places.** `gate/runner/summary.py:37-50` `render(lines, *, wall_ms, summed_ms, exit_code, run_dir, registry=None, image=None)` prints the row table first (`summary.py:51-66`); it is called by the local scheduler (`gate/runner/scheduler.py:511`) and by CI's aggregator (`gate/runner/summarize.py:189`). A block that is a registry row's stdout appears in neither. → §2 S3: the block is the renderer's own argument, required, fed from the `lawdelta` row's record.
2. **The one merge-base row today passes silently when it has no base.** `gate/runner/rows/apidiff.py:28-33` `merge_base()` tries `origin/main` then `main`; `apidiff.py:75-78` prints "no merge base with main: nothing compared", records covered and finishes green. Only the `lint` job checks out history (`gate/ci/generate.py:48` `NEEDS_HISTORY = frozenset({"apidiff"})`; `.github/workflows/gate.yml:15-22`). → §6 counter-examples 3 and 8; §8 risk 1: `lawdelta` fails with no base and lives in `lint`.
3. **The not-built map is Go code, not a law path, and its rule names T4.** `conformance/corpus_test.go:28-38` `notBuilt` is a `map[string]struct{ construct, sprint string }` (nine entries); `TestCorpus` (`corpus_test.go:43-81`) logs "not built until …" and never skips; the rule reads "red at that sprint's own T4" (`sprint-plan.md:100`; ADR-0015 §2 `todocheck` row as annotated). → §3 (`conformance/notbuilt.json` as data, a law path — §11 Q2); §2 S2; the row is T4-only.
4. **The plan table's "closed" mark has three forms, and the open row is a card id, not a part.** `sprint-plan.md:48-73`: `S1.1a-1 [✓](s1-1a-1-record.md)`, `S3.1 ✓ ([S3.1-1](…), [S3.1-2](…))`, `S3.2c ✓ ([record](…))`, a struck row `~~S1.1a~~` (line 47), and the open rows `S1.1c`, `S4.1`, … The first open row today is `S1.1c` (line 60); the marker will read `S1.1c-1`. → §3: the order lives in a pure package that strips `-<n>` to a row and ignores struck rows; the marker test compares rows.
5. **`gotest.py` counts packages, and a named test that never runs is red.** `gate/runner/rows/gotest.py:5-7` (GATE_COVERED = packages), `:21-34` (`missing_names`). → §4: the `conformance-expiry` row is `gotest.py -run ^TestNotBuiltExpired$ ./conformance`, `expected_scope: ["go", "./conformance"]`, `min_wants: 1`.
6. **The dependency allowlist's last entry is seven modules in one string.** `tools/deps-allowlist.json` last object: `"module": "github.com/google/go-cmp v0.6.0, github.com/yuin/goldmark v1.4.13, golang.org/x/net …"`, `"version": "as listed"`, `"license": "BSD-3-Clause / MIT (goldmark)"`. A row that compares module paths cannot read it. → §4: the coordinator normalises it into seven entries before lane C starts (a law edit, in the LAW Δ of this PR).
7. **`docsync` (d) is red on today's text.** `docs/language/SPEC.md:220` says "67 codes: 23 parse-level … 44 static"; `docs/language/errors.md` has 68 `### E-…` headings, 23 `**[parse]**` + 45 `**[static]**`; `docs/language/invalid/` holds 28 files (SPEC's "28 files" is right). The heading regex to reuse is `internal/parser/fixtext_test.go:19`. → §7 lane B: the test is red first on main's text; the SPEC line is fixed in this sprint (a law edit).
8. **Every law path ADR-0015 §1.3 names must exist, or liveness is red by construction.** §1.3 lists `tools/allowlists/*.json` and `tools/analyzers/**`, which do not exist (owner, 2026-11-04). → §3: `law.json` lists only tracked paths; a pin test asserts §1.3's *existing* paths ⊆ `law.json` and names the two absent ones as S1.1c-2's.
9. **The proof drivers have one shape, and the proofs row pins their count.** `gate/proofs/drivers/support.py:175-306` (`World`: `copy_gate`, `copy`, `tracked`, `write`, `track`, `commit`, `registry`, `run`), `:308-336` (`DriverTest`, `assertRed`); `gate/registry.json` row `proofs` has `min_wants: 21` over `:(glob)gate/proofs/drivers/test_*.py`. → §7: three new drivers; `min_wants` 21 → 24 in the registry commit that adds them.
10. **Size.** ≈ 5.5 points (above). One sprint, one PR.

## 1. Routine, shape, waves

**Full loop without the semantics panel** (owner, decision 7: the sprint changes the gate's own trust surface — the renderer, the law manifest, the expiry — so it gets the design note, the design reviews, the combined-build review and a break-it aimed at the gate; no language semantics, so no three-reviewer panel). **Waves (≤ 2 agents):** **W0** QA alone — the three proof drivers and the two red Go tests, written from this check and the card before any build (sprint model §8) · **W1** lane A (`lawdelta`, Python) + lane B (the marker, the expiry, the counts; Go) · **W2** lane C (`deps`, Python) alone, beside the coordinator's law edits. *Changes from the sprint card:* none — the card is written with this check. *One PR.*

## 2. Seams — signatures and stubs

**S1 — `gate/law.json` (lane A → lane B, lane C, the coordinator).** The law manifest's shape, created by lane A's first commit:
```json
{
  "schema": 1,
  "paths": [
    {"path": "gate/registry.json",            "kind": "registry"},
    {"path": "gate/law.json",                 "kind": "law"},
    {"path": "gate/current-sprint",           "kind": "marker"},
    {"path": "gate/tripwire.json",            "kind": "ceilings"},
    {"path": "gate/run.py",                   "kind": "code"},
    {"path": "gate/runner/",                  "kind": "code"},
    {"path": "gate/baseline/regen.py",        "kind": "code"},
    {"path": ".github/workflows/",            "kind": "workflow"},
    {"path": "tools.json",                    "kind": "tools"},
    {"path": "tools/deps-allowlist.json",     "kind": "deps"},
    {"path": "tools/importcheck/rules.json",  "kind": "edges"},
    {"path": "api/exports.golden",            "kind": "exports"},
    {"path": "api/clock.golden",              "kind": "exports"},
    {"path": "conformance/baseline/",         "kind": "baseline"},
    {"path": "conformance/notbuilt.json",     "kind": "notbuilt"},
    {"path": "docs/language/grammar.lark",    "kind": "doc", "flag": "the grammar changed — owner decision required"},
    {"path": "docs/language/grammar.ebnf",    "kind": "doc", "flag": "the grammar changed — owner decision required"},
    {"path": "docs/language/SPEC.md",         "kind": "doc", "flag": "SPEC changed — owner decision required"},
    {"path": "docs/language/SEMANTICS.md",    "kind": "doc", "flag": "SEMANTICS changed — owner decision required"},
    {"path": "docs/language/errors.md",       "kind": "doc", "flag": "the error catalogue changed — owner decision required"},
    {"path": "docs/architecture/ARCHITECTURE.md", "kind": "doc", "flag": "ARCHITECTURE changed — owner decision required"},
    {"path": "docs/architecture/adr/",        "kind": "doc", "flag": "an ADR changed — owner decision required"}
  ]
}
```
A `path` ending in `/` covers every tracked file under it. `kind` names the semantic parser (§3); `code`, `workflow` and `doc` get path and ±lines only (docs add their flag). The pin test (§7, lane A) asserts every path ADR-0015 §1.3 names that git tracks is listed, and lists the two it names that do not exist (`tools/allowlists/*.json`, `tools/analyzers/**`) as S1.1c-2's. *Stub:* lane A's first commit lands the file and a `lawdelta` that prints path and ±lines for every kind; the parsers follow kind by kind. Lane B adds nothing to the file: `conformance/notbuilt.json` and `gate/current-sprint` are listed from the start (lane A reads this check); the coordinator adds the three registry rows.

**S2 — `tools/sprintid` (lane B → the conformance tests; later `todocheck`, S1.1c-2).** A pure package: strings in, values out, no I/O.
```go
// package sprintid — the sprint-id grammar S<stage>[.<part>][<letter>][-<n>] (sprint-plan §3) and the plan table's order.
type ID struct {
	Stage  int    // S3 → 3
	Part   int    // S3.2 → 2; 0 when absent
	Letter byte   // S3.2a → 'a'; 0 when absent
	N      int    // S3.3-2 → 2; 0 when absent (the plan check's split)
}
func Parse(s string) (ID, error)           // "S3.3-2" → {3, 3, 0, 2}; "S10" → {10, 0, 0, 0}; "s3.3", "S3.3-", "S3.3-2x", "" → error
func (id ID) Row() string                  // the card id: "S3.3-2" → "S3.3"; "S1.1a-1" → "S1.1a"
func (id ID) String() string               // the canonical spelling

// Order is the plan table's rows in the order they are printed, with their closed marks.
type Order struct { /* unexported */ }
type Row struct { ID string; Closed bool } // ID is a card id ("S3.3"); Closed when the row carries ✓ in any of its forms
func ReadPlan(markdown string) (Order, error) // sprint-plan.md's §3 table: every `| S… |` row between the table's header and its first blank line; a struck row (~~S1.1a~~) is skipped; an id that fails Parse is an error; a table with no rows is an error
func (o Order) Rows() []Row
func (o Order) Index(id string) (int, error)   // the position of Parse(id).Row(); an id whose row is not in the table is an error
func (o Order) Reached(entry, current string) (bool, error) // Index(entry) <= Index(current): the entry's sprint has arrived
func (o Order) FirstOpen() (string, error)     // the first row not Closed; an error when every row is closed
```
*Stub:* lane B's first commit lands `Parse`, `Row` and the table tests (§6); `ReadPlan` follows. The conformance tests (`conformance/notbuilt_test.go`) read `../docs/sprints/sprint-plan.md` and `../gate/current-sprint` from disk and hand the text to the package. Lane B owns both.

**S3 — the LAW Δ record and the block (lane A, both ends).** The `lawdelta` row writes its block into its own record as `"law": [<line>, …]` (the record schema `gate/runner/records.py`, bumped), and `summary.render` gains a required keyword `law: Sequence[str]`, printed **before** the row table. The scheduler (`scheduler.py:511`) passes the `lawdelta` row's lines when the row ran at this tier, and `["LAW Δ: not read at this tier (T0 has no merge base)"]` at T0; `summarize` (`summarize.py:189`) reads the lines from the `lint` job's `lawdelta` record and turns a missing record into a RED row as it does for any missing record (`summarize.py:180-184` names an unknown row; the missing-record rule is ADR-0015 §1.2). A render call without `law` is a `TypeError` — the block cannot be absent.

**S4 — the `deps` row's reading of the allowlist (lane C; the coordinator normalises the file first).** Each entry is one module: `{"module": "<path>", "version": "<v>", "scope": "<text>", "license": "<spdx>", "approved": "<date>", "why": "<text>"}`; `license` ∈ {`Apache-2.0`, `MIT`, `BSD-2-Clause`, `BSD-3-Clause`, `ISC`} (ADR-0009, ground rule 20). The row reads `go list -m -json all` (the build list, offline once the module cache is warm — the `lint` job restores it) and `go mod graph` (the node set, for liveness), and each module's `LICENSE*`/`COPYING*` file under its `Dir` from `go list -m -json`.

## 3. Packages — where each new rule lives

| Rule | Package (pure, no I/O) | Extends |
|---|---|---|
| the sprint-id grammar; a part belongs to its row; the plan table's order; "reached" = at or before the current row; the first open row | `tools/sprintid` (Go, stdlib) | the grammar in `sprint-plan.md:40` (§3's numbering paragraph); nothing in code today |
| a not-built corpus file whose sprint is reached is red at T4; a missing or unparseable marker, an entry or marker naming no row, an empty map while files are refused — each fatal, never a pass | `conformance` (test `TestNotBuiltExpired`, `conformance/notbuilt_test.go`), data in `conformance/notbuilt.json` | `conformance/corpus_test.go:28-38` (the map moves to the JSON file; `TestCorpus` reads the same file through one loader, `corpus_test.go:66-76` unchanged in meaning) |
| the marker names the first open plan row | `conformance` (test `TestCurrentSprintIsTheFirstOpenRow`) over `tools/sprintid` | — |
| the LAW Δ block: per law path, +/− lines and a one-line semantic delta; `none` on an empty delta; the merge base; liveness | `gate/runner/law.py` (pure: `delta(kind, old_text, new_text) -> list[str]` per kind; `block(entries) -> list[str]`), the row `gate/runner/rows/lawdelta.py` (git only), `gate/runner/summary.py` (prints), `gate/runner/summarize.py` (carries) | `apidiff.py:28-33` (`merge_base`, moved to `gate/runner/git.py` and shared), `summary.py:37-66`, `summarize.py:150-200`, `records.py` |
| semantic deltas: `edges` (`tools/importcheck/rules.json`: ±edges by `from → to`, read as parsed JSON), `registry` (±rows by id; a changed row's changed fields), `exports` (±identifiers), `deps` (±modules, a changed version), `ceilings` (a changed ceiling by metric/rule), `baseline` (per file: blocks added / removed / re-recorded by their content key), `notbuilt` (an entry added, removed, or its sprint moved), `tools` (a tool's version changed), `marker` (`S1.1c-1 → S4.1`), `law` (±paths) | `gate/runner/law.py`, one function per kind, each over two texts | the files' own readers: `gate/runner/registry.py:150-170`, `gate/baseline/regen.py` (the content key), `tools/apipartition` (golden line format, ADR-0015 §Implementation notes) |
| a `go.mod`/build-list module not in the allowlist; a licence outside the allowed set, missing, or disagreeing with the entry; an allowlist entry for a module absent from `go mod graph`; a tracked `go.mod` other than the two | `gate/runner/deps.py` (pure: `classify_license(text) -> str | None` by fingerprint; `compare(build_list, allowlist, graph) -> list[str]`), the row `gate/runner/rows/deps.py` (`go list`, `go mod graph`, file reads) | `tools/deps-allowlist.json` (normalised), ADR-0009 §1, ADR-0001 §1 |
| SPEC §9's four counts (total, parse, static, invalid files) equal errors.md's headings and `invalid/`'s files; every code SPEC mentions is in errors.md | `internal/parser` (test `TestErrorCountsMatchTheCatalogue`, beside `fixtext_test.go`) | `fixtext_test.go:19` (the heading regex) |

## 4. Ownership — one owner per file per wave

| Lane | Owns |
|---|---|
| **QA (W0)** | `gate/proofs/drivers/test_lawdelta_planted.py`, `test_notbuilt_expired.py`, `test_deps_planted.py` (new); `conformance/notbuilt_test.go` (the two red Go tests, which lane B then makes green — lane B owns the file from W1); `internal/parser/errorcounts_test.go` (red on today's SPEC; lane B owns it from W1) |
| **A (W1)** | `gate/law.json` (new), `gate/runner/law.py` (new), `gate/runner/git.py` (new; `merge_base` moved here, `apidiff.py` re-pointed), `gate/runner/rows/lawdelta.py` (new), `gate/runner/summary.py`, `gate/runner/summarize.py`, `gate/runner/scheduler.py` (the render call only), `gate/runner/records.py`, `gate/ci/generate.py` (`NEEDS_HISTORY`), their tests under `gate/runner/tests/` and `gate/ci/tests/`, the summarize fixture trees |
| **B (W1)** | `tools/sprintid/` (new, with tests), `conformance/notbuilt.json` (new; the map moved), `conformance/corpus_test.go` (the loader), `conformance/notbuilt_test.go`, `internal/parser/errorcounts_test.go` |
| **C (W2)** | `gate/runner/deps.py` (new), `gate/runner/rows/deps.py` (new), their tests, `gate/proofs/drivers/test_deps_planted.py` (from QA, made green) |
| **Coordinator** | `gate/registry.json` (rows `lawdelta`, `conformance-expiry`, `deps`; `proofs` `min_wants` 24), `gate/current-sprint` (the G0 commit), `tools/deps-allowlist.json` (normalised before W2), `docs/language/SPEC.md:220` (the count), `docs/sprints/00-sprint-model.md` (the G0 line), `docs/sprints/templates/plan-check.md` (the marker item), the ADR annotations (ADR-0015 §1.3 as built; ADR-0005 §4 and ADR-0015's `todocheck` row: the expiry built; ADR-0001 §1: checked; ADR-0009 §1: the row exists), `docs/development/DEVELOPMENT.md` ("what exists"), the generated workflows (`make ci-workflow`), the record |

## 5. Design notes and the panel (before code)

One build-ready design note, `s1-1c-1-design.md`, written by the coordinator after G0 (every path, full signatures, state machines, named tests, ordered steps), reviewed by the architect with the-fool and by QA for testability (decision 7). No panel. The design must settle:

- **P1** The block's exact text: one line per changed path — `<path> +<a>/−<b> — <semantic one-liner | flag>` — headed `LAW Δ (<n> files, vs <merge-base short sha>)`, or `LAW Δ: none`, or `LAW Δ: HEAD is main — read the PR run` (§11 Q3), or the FAIL reason `lawdelta: no merge base with main (fetch-depth 0 or a local main is needed)`.
- **P2** The record schema bump (`records.py`) and how `summarize` carries `law` from the `lint` job's record; the fixture tree that proves a missing `lawdelta` record is a named RED row.
- **P3** `tools/sprintid.ReadPlan`'s exact table grammar (the header row, the separator, struck rows, the three ✓ forms) and its error texts.
- **P4** The licence fingerprints (`MIT`: "Permission is hereby granted, free of charge"; `BSD-3-Clause`: "Redistribution and use in source and binary forms" + the no-endorsement clause; `BSD-2-Clause`: the same without it; `Apache-2.0`: "Apache License" + "Version 2.0"; `ISC`: "Permission to use, copy, modify, and/or distribute") and the rule for a module whose licence file is absent from the cache.
- **P5** Where the one-module check runs: inside the `deps` row (recommended: one row, three named checks, one driver with six cases) or its own row (§11 Q1).

## 6. Counter-examples (listed before coding; each becomes a test)

**LAW Δ (lane A; drivers by QA).** (1) a branch from `main` adds one edge to `tools/importcheck/rules.json` → the block's first line is `tools/importcheck/rules.json +1/−0 — +1 edge expr → runtime`. (2) no law file changed → `LAW Δ: none`, the row PASS. (3) a world with no `main` and no `origin/main` → the row FAIL, reason `lawdelta: no merge base with main …`; the summary still opens with a block (`LAW Δ: FAILED — see the lawdelta row`). (4) `law.json` lists a path git does not track → FAIL naming it (liveness). (5) a tracked path ADR-0015 §1.3 names is missing from `law.json` → the pin test red naming it. (6) `docs/language/SPEC.md` edited → `docs/language/SPEC.md +1/−1 — SPEC changed — owner decision required`. (7) a baseline block re-recorded (`conformance/baseline/07-consent-gate.jsonl`) → `… — 07-consent-gate: 1 block re-recorded`. (8) HEAD is `main` (merge base = HEAD) → `LAW Δ: HEAD is main — read the PR run`, PASS. (9) a render call without `law` → `TypeError` (unit test). (10) CI: the `lint` job's `lawdelta` record missing → a RED row `lint | lawdelta | FAIL | missing record` in the aggregated summary (fixture tree). (11) `gate/current-sprint` changed `S1.1c-1 → S4.1` → `gate/current-sprint +1/−1 — marker S1.1c-1 → S4.1`. (12) `gate/registry.json` gains a row → `… — +1 row conformance-expiry (T4, test)`.

**The expiry and the marker (lane B; the driver by QA).** (1) marker `S4.1` → `TestNotBuiltExpired` fails naming `01-agent-kill-switch.cep`, `08-session-flow.cep`, `torture/comments.cep`, each with its construct and `S4.1`; the row red. (2) marker `S1.1c-1` → pass (no entry reached). (3) marker `S4.2` → the three S4.1 entries fail (reached = at or before). (4) marker `S9.2` → every entry fails. (5) no marker file → fatal `gate/current-sprint: missing`. (6) marker `S99` → fatal `S99: not a row of the plan table`. (7) an entry `{"sprint": "S4"}` (no such row) → fatal naming the file. (8) `notbuilt.json` empty while `TestCorpus` still refuses a file → `TestCorpus` red (unchanged rule: a listed file that deploys is red; an unlisted refused file is red). (9) `Reached("S3.3-2", "S1.1c-1")` → true (row S3.3 precedes row S1.1c in the table; any numeric sort says otherwise). (10) `Reached("S10", "S9.2")` → false; `Reached("S9.2", "S10")` → true (string compare says the opposite). (11) `FirstOpen()` on today's table → `S1.1c`; marker `S1.1c-1` → `TestCurrentSprintIsTheFirstOpenRow` passes; marker `S4.1` → fails `marker S4.1 is not the first open row S1.1c`. (12) a table where every row is closed → `FirstOpen` error. (13) `Parse("s3.3")`, `Parse("S3.3-")`, `Parse("S3.3-2x")`, `Parse("")` → errors; `Parse("S1.1a-1").Row()` → `S1.1a`.

**`deps` (lane C; the driver by QA).** (1) a world's `go.mod` requires `example.com/planted v1.0.0` (a `replace` to a local directory in the world) → red `deps: example.com/planted v1.0.0 is not in tools/deps-allowlist.json`. (2) a listed module whose `LICENSE` in the cache is GPL text → red naming the module and `GPL-3.0`. (3) a listed module with no licence file → red `no LICENSE file`. (4) an entry says `MIT`, the file is BSD-3 → red `license field MIT, file reads BSD-3-Clause`. (5) an entry for a module absent from `go mod graph` → red `stale entry`. (6) a tracked `internal/x/go.mod` → red `tracked go.mod files: go.mod, gate/proofs/go.mod, internal/x/go.mod; ADR-0001 §1 allows two`. (7) the real tree after normalisation → green (the seven graph-only entries have `scope: "module graph only"` and are checked for liveness against `go mod graph` only).

**The counts (lane B).** (1) today's SPEC.md:220 (`67 … 44 static`) vs errors.md (68, 45) → red naming both numbers. (2) the fixed line (`68 … 23 parse-level … 45 static`; 28 files) → green. (3) a code `E-XYZ-999` mentioned in SPEC and absent from errors.md → red.

## 7. Test plan per lane (the rig proves the line)

| Lane | Acceptance → rig |
|---|---|
| A | (1) a planted law edit appears with its one-liner; (2) `none` on an empty delta → **driver `test_lawdelta_planted.py`** (a world with `main` and a branch, row `lawdelta` at T1) · (3) no merge base is FAIL, a listed untracked path is FAIL → **the same driver** + **`gate/runner/tests/test_lawdelta.py`** · (4) the block prints first at every tier and in CI's aggregate; a missing record is a RED row → **`test_summary.py`** (render requires `law`), **`test_summarize.py`** fixture tree · (5) each of the nine kinds yields its one-liner on a planted two-text pair → **`gate/runner/tests/test_law.py`** (pure, no git) · (6) §1.3's tracked paths ⊆ `law.json` → **`test_law_pin.py`** · (7) `lint` is the job with history for every merge-base row → **`gate/ci/tests/test_generate.py`** generalised |
| B | (1) an entry whose sprint is reached is red at T4, naming file, construct, sprint; (2) a missing/unparseable marker or an unknown row is fatal → **driver `test_notbuilt_expired.py`** (marker `S4.1` and `S1.1c-1`; tier T4, row `conformance-expiry`) + **`conformance/notbuilt_test.go`** at T2 (a fixture marker and plan in `t.TempDir()`) · (3) the order rules → **`tools/sprintid` table tests** (counter-examples 9–13) · (4) the marker is the first open row → **`TestCurrentSprintIsTheFirstOpenRow`** · (5) the counts → **`internal/parser/errorcounts_test.go`**, red on main's text first |
| C | (1)–(6) above → **driver `test_deps_planted.py`** (six cases, one world each; row `deps` at T2) + **`gate/runner/tests/test_deps.py`** (the licence fingerprints, the comparison, pure) · (7) the real tree green → the T2 run |

*Verifiers at "done":* every lane T2 (`make check`); lane A also a T4 run of the `lint` job with `lawdelta` on the integration branch, and the block read on this PR's CI summary (owner's check); lane B also `make strict-gate`'s `conformance-expiry` row green on the real tree with the marker `S1.1c-1`; every driver counted by `proofs` (`min_wants` 24) and seen red once with its instrument removed (sprint model §10).

## 8. Risks and order

1. **A silent `none`.** The `apidiff` shape (§0.2) is the obvious copy. *Mitigation:* counter-example 3 is QA's first driver case; `merge_base` moves to `gate/runner/git.py` with one caller contract: the caller decides, and `lawdelta`'s decision is FAIL.
2. **The block that is not read.** Rendered locally, not carried to CI's aggregate. *Mitigation:* S3 — the aggregate reads the record; the acceptance line is the owner reading this PR's block.
3. **The expiry at the wrong tier.** At T2 it reddens S4.1's lanes from G0 and invites relabeling. *Mitigation:* a T4-only row; `notbuilt.json` is a law path, so a relabel is a LAW Δ line (`notbuilt: 01-agent-kill-switch S4.1 → S4.2`).
4. **The marker nobody bumps.** *Mitigation:* `TestCurrentSprintIsTheFirstOpenRow` at T2, the G0 line, the plan-check template item.
5. **The plan table as machine-read input.** A renamed column or a new ✓ form breaks `ReadPlan`. *Mitigation:* `ReadPlan` pins the header row's text and fails loudly on an unknown row shape; the test reads the real plan, so the break shows at T2, not at T4.
6. **The module cache in CI.** `go list -m -json all` needs the cache; a cold job downloads (network at install time is the installer's existing exception). *Mitigation:* the row runs after the cache restore the `lint` job already does; a module with no `Dir` is a named FAIL, never a skip.
7. **Two renderers drift.** *Mitigation:* `render` is the one function; `law` is required; `test_summary.py` covers both callers' shapes.
8. **Lane B's Go in `tools/`.** `archcheck` scans `tools/` (`registry.json` row `archcheck`: scope `prod, test, tools` — to confirm in the design); `tools/sprintid` imports only `strings`, `strconv`, `errors`, `fmt`, `regexp`. *Mitigation:* stated in the brief; the lane runs T2 before claiming done.

**Order:** W0 (QA: drivers + red tests, ~0.5 day) → G1 (design reviewed) → W1 (A ‖ B) → coordinator's law edits (registry rows, allowlist normalised, SPEC line, marker exists since G0) → W2 (C) → T3 on the integration branch → combined review + break-it → fix round → T4 → owner's check (the block on this PR) → merge → close-out.

## 9. Lane briefs (skeletons; the coordinator adds paths, worktree and the report file)

**Common to all:** your own worktree and branch; the pinned toolchain first (`make tools`; the gate only runs under the pinned CPython, `gate/run.py` refuses another); write only to your owned files (§4); commit early; never push or merge; tests on the real code (never a mirror); a breaker's red test is committed; no AI attribution anywhere; never call anything an owner decision unless it is in decisions.md; a law delta you need (a registry row, an allowlist entry) goes in your report for the coordinator, never in your lane; GitNexus (`query`, `context`, `cypher`) to verify a code claim, grep/Read on an error, no retries; ≤ 3 Hindsight searches, each fact confirmed in decisions.md, an ADR or the code; a hard time box, stated in the brief; the report by Bash heredoc to the path the brief names and in full in the final message.

**Lane QA — the proofs first** (qa-expert). *Goal:* the three drivers and the two Go tests of §7, each red against today's gate. *Read:* this check §6–§7; `support.py`; `test_tripwire_planted.py` (a world with the engine), `test_api_planted.py` (a world with a branch), `test_regen_baseline.py`; `corpus_test.go`; `fixtext_test.go`. *Provides:* the rig lanes A–C make green. *Accept:* every driver fails today naming the missing row or the missing block; the Go tests fail on main's text. *Done:* each test seen red, committed red (a driver that references a row not yet in the registry fails on "unknown row" — acceptable red).

**Lane A — the law manifest and the LAW Δ block** (python-engineer). *Goal:* `gate/law.json`, `gate/runner/law.py`, `rows/lawdelta.py`, the block in both renderers, CI history, liveness, the pin test. *Stories:* counter-examples 1–12 of §6 (LAW Δ). *Read:* ADR-0015 §1.3; this check §2 S1, S3, §3; `summary.py`, `summarize.py:150-200`, `scheduler.py:500-525`, `records.py`, `apidiff.py:20-80`, `generate.py:40-60,140-160`, `registry.py:150-190`, `regen.py` (the content key), `apipartition` (the golden format). *Provides:* S1, S3. *Order:* `law.json` + path/±lines → the record + both renderers → the parsers kind by kind → liveness + pin → `generate.py`. *Accept:* §7 lane A. *Done:* T2; a T4 `lint` job run on the integration branch. *Not in this lane:* the registry rows, the workflows (the coordinator regenerates), docs.

**Lane B — the marker, the expiry, the counts** (go-engineer). *Goal:* `tools/sprintid` (S2), `conformance/notbuilt.json` + the loader, `TestNotBuiltExpired`, `TestCurrentSprintIsTheFirstOpenRow`, `TestErrorCountsMatchTheCatalogue`. *Stories:* §6 (expiry and marker) 1–13, (counts) 1–3. *Read:* `sprint-plan.md:40,47-73,100`; ADR-0005 §4 (annotated); `corpus_test.go`; `gotest.py`; `fixtext_test.go`; testing.md. *Provides:* S2. *Order:* `Parse`/`Row` + tests → `ReadPlan` + `Reached`/`FirstOpen` → `notbuilt.json` + loader (`TestCorpus` unchanged in meaning) → the two conformance tests → the counts test (red first on main's SPEC line; the coordinator fixes the line). *Accept:* §7 lane B. *Done:* T2; the `conformance-expiry` row green at T4 with marker `S1.1c-1` and red with `S4.1` (the driver). *Not in this lane:* the registry row, the SPEC edit, `gate/current-sprint` (exists since G0).

**Lane C — the dependency checks** (python-engineer). *Goal:* `gate/runner/deps.py`, `rows/deps.py`, their tests; the driver green. *Stories:* §6 (`deps`) 1–7. *Read:* ADR-0009 §1, §4; ADR-0001 §1; this check §2 S4, §5 P4–P5; `tools/deps-allowlist.json` (normalised); `apidiff.py` (the `go` invocation pattern, `CGO_ENABLED=0`, the pinned Go). *Provides:* —. *Order:* fingerprints + comparison (pure, tests) → the row → the driver green. *Accept:* §7 lane C. *Done:* T2. *Not in this lane:* the allowlist edit, the registry row.

## 10. Reviews of this plan check

*Full loop: the architect reviewer and the QA agent each read this draft blind before it goes to the owner ([§8](00-sprint-model.md#8-reviews-architecture-and-qa-at-every-stage)). Ranked on the [ground rules' scale](../ground-rules.md); each finding gets a disposition here.*

| # | Reviewer | Severity | Finding | Disposition |
|---|---|---|---|---|
| | | | *(filled after the reviews)* | |

## 11. Questions for the owner (recommendation first)

- **Q1 One `deps` row with three named checks, or three rows?** → **One row, `deps`**, three checks each with its own planted case and failure text (allowlist, licence, one-module): the three share one reading of the module graph, and one row keeps the registry honest about what runs together; the alternative, `deps` + `licence` + `gomod`, gives three summary lines and three `min_wants` for the same evidence.
- **Q2 Where does the not-built list live?** → **`conformance/notbuilt.json`, a law path**: data the expiry test, `TestCorpus` and the LAW Δ parser all read, so a relabel (`S4.1 → S4.2`) is a one-liner in the block; the alternative keeps the Go map in `corpus_test.go` and lists that file, where a relabel shows only as "+1/−1".
- **Q3 What does the block say on `main`'s own CI run?** → **`LAW Δ: HEAD is main — read the PR run`, PASS**: the merge base is HEAD, so there is no delta to show, and saying `none` would be false; the alternative diffs against `HEAD~1`, which after a rebase-merge of n commits shows one commit's delta, not the sprint's.
- **Q4 Licence by fingerprint, or by the field?** → **By fingerprint of the module's licence file, and the field must agree**: a check that compares the hand-entered field with itself checks nothing; the alternative trusts the field and only verifies the file exists.
- **Q5 The waves: W0 QA alone, then A ‖ B, then C alone?** → **Yes**: the two-agent rule, lane A's size, and lane C's dependence on the normalised allowlist; the alternative runs B ‖ C first and A alone, which delays the block that this PR's own owner's check needs.
- **Owner's check additions** (refined from the diff at the end): *by reading* — this PR's LAW Δ block on its CI summary: `gate/law.json` (new), `gate/registry.json` (+3 rows, `proofs` 21 → 24), `.github/workflows/gate.yml` (regenerated), `tools/deps-allowlist.json` (7 entries normalised), `docs/language/SPEC.md` (§9 count: flagged), the ADRs (annotations: flagged), `conformance/notbuilt.json` (new), `gate/current-sprint` (new); *by running* — `make strict-gate` with `gate/current-sprint` set to `S4.1` (the `conformance-expiry` row red naming the three files), then restored; *by hand* — the G0 line in the sprint model and the marker file reading `S1.1c-1`.
