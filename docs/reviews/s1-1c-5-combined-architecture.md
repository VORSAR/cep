# S1.1c-5 combined build: blind architecture review (with the-fool)

> **Historical — S1.1c-5, the strict lint set: the instrument, the whole-tree rules, the first packages; merged in PR #47.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-5-plan-check.md) · [design](../sprints/s1-1c-5-design.md) · [record](../sprints/s1-1c-5-record.md) · reviews: [research lint](s1-1c-5-research-lint.md), [plan check architecture](s1-1c-5-plan-check-architecture.md), [plan check qa](s1-1c-5-plan-check-qa.md), [design architecture](s1-1c-5-design-architecture.md), [design qa](s1-1c-5-design-qa.md), [combined architecture](s1-1c-5-combined-architecture.md), [breakit qa](s1-1c-5-breakit-qa.md). **← [S1.1c-4](../sprints/s1-1c-4-record.md) · [S1.1c-6 (next)](../sprints/sprint-plan.md#status) →**

Branch `s1-1c-5/integration` @ c6deb91, base `main` 0fb2b53 (149 files, +9933/−700, 80 commits). I read it only. Experiments are in `s1-1c-5-scratch/arch-final-exp/`: a `--shared` clone for checking each commit, the `astequal` binary, the `strfold` checker and the two lint worlds.

- **GitNexus:** the index is at 4266f65, so it is stale. It lacks `golangci.py`, `pin.py`, `parts.py`, `rows/smells.py` and `tools/astequal`, so I confirmed every claim with grep or by running it.
  - `detect_changes` (compare 0fb2b53): 366 changed symbols, 149 files, risk "critical". Most of that is reflows.
  - `check`: 0 import cycles.
- **Hindsight:** the tools were not available in this session, so I used no memory. The decisions I cite are in `docs/sprints/decisions.md` (lines 924–1030).

## Findings

### CA-S1 (should-fix): `tools/astequal` reports "equal" when it compared nothing, and it cannot prove this sprint's moves or reflows

**Claim.** It exits 0 with no output in three cases:
- the file list arrives as one unsplit argument;
- a named path is absent at both revisions;
- the diff against the revision is empty.

In two other cases it reports a difference although the code is unchanged:
- a pure move where both files import the same package;
- a string literal split into `"a" + "b"`.

So as built it proves none of lane C's reflow commits and none of lane D's commits.

**Evidence.**
- Run in the worktree:
  - `astequal 0fb2b53 "internal/table/table.go internal/clock/clock.go"` gives exit 0;
  - `astequal 0fb2b53 internal/table/nonexistent.go` gives exit 0;
  - `astequal HEAD` gives exit 0.
- Cause: `main.go:66-79` drops a path that is absent on both sides without a word. `run` (`:33-52`) never checks that anything was compared.
- **Move false red:** run on a clone checked out at dec066a, `astequal dec066a^ internal/model/canonical.go internal/model/model.go` gives `internal/model: import "github.com/vorsar/cep/internal/event"#dup2 differs`, exit 1. Imports are keyed per package (`canon.go:81-87`).
  - A line-multiset check (both sides, `//nolint` stripped) shows the move is pure. The only differences are the comment reflow, the package clause and the import lines.
  - `TestMovedFunctionAcrossFilesEqual` passes only because its two files import disjoint packages (`astequal_test.go:56-60`).
- **String splits:** every lane C commit's production file reports a difference (for example `internal/event: func NewSchema differs`), because a `BasicLit` is not a `BinaryExpr`.
- **Lane D:** 9847faa, 3112904, 296a604 and 64a4bb9 each exit 1. They are splits, so their characterisation tests are the proof. The one exit-0 claim in a commit message (296a604, `io/inmemory/sink.go`) reproduces for real.
- **The G5 procedure is also wrong** (design §7.4, "re-run `astequal <commit>^`"). Run on the work tree, it compares `<commit>^` with HEAD, not with `<commit>`.

**Consequence.**
- A coordinator who quotes "astequal exit 0" in the record may be quoting a comparison of zero files.
- Run as designed, the G5 step gives reds that prove nothing.
- Acceptance line 7 still holds through the characterisation tests and T4, so this is not a blocker. But the instrument is misleading (ground rules 15 and 17).

**Fix** (tests red first).
- Exit 2 when no file was compared, or when a named path exists at neither revision. Tests: `TestUnsplitPathListIsUsageError`, `TestPathAbsentOnBothSidesIsError`, `TestEmptyDiffIsError`.
- On exit 0, print `astequal: N files, M declarations equal`.
- Key import specs per package as a set, so identical `path+name` specs count once (`TestMoveWithSharedImportEqual`).
- Fold `+` chains of string literals before printing (`TestSplitStringLiteralEqual`). This would turn lane C's reflow claims into proofs.
- Write the G5 step as "in a scratch clone checked out at `<commit>`, run `astequal <commit>^`".
- Later: list every differing key, not only the first.

### CA-S2 (should-fix): the `smells` row's wrong-line waiver check fails open on any `nolint_filter` line it cannot read

**Claim.** `rows/smells.py`'s `silenced()` skips without a word every stderr line that carries the marker but does not match its two regexes. A checkout path with a space is enough to turn the check off.

**Evidence.**
- I used one plant in both worlds: a standalone `//nolint:lll // PERMANENT …` above a function, with a 140-column line inside the function. The run used the pinned golangci-lint v2.15.0 with `GL_DEBUG=nolint_filter` and the committed `.golangci.toml`.
  - World `…/nospace/m`: `silenced()` gives `['p.go:5:1: lll: silenced by the //nolint on line 3; …']`.
  - World `…/sp ace/m`: it gives `[]`. stderr still holds 2 `found ignored range` lines, and golangci exits 0.
- Cause: `SILENCED_AT = r" (/\S+?\.go):(\d+):(\d+) <nil> \d+ "` (`smells.py:40`), and the bare `continue`s at `:146-147` and `:150-151`.
- The planted case 17 runs in a temp world with no space in its path, so it cannot see this. After a golangci-lint upgrade, only the planted shape is re-checked.

**Consequence.**
- The row silently does less than it claims, including after a debug-format drift for any shape the plant does not cover.
- The practical exposure is low. The suppressions row's lexical `STANDALONE` refusal (`gate/runner/suppressions.py:55`, `:219`) is the first net for the standalone case, and CI paths have no spaces.

**Fix** (test red first).
- Fail closed: a stderr line containing `SILENCED` that the regexes cannot read is a fault (`unreadable nolint_filter line …; the pinned binary's debug format changed?`).
- Match the path against `realpath(root) + "/"`, not `\S+`.
- On a full-tree run, a tree with at least one in-code `//nolint` that gave zero marker lines is red. This catches a renamed debug key.
- Unit test with a root that contains a space.

### CA-S3 (should-fix): `CEP_TRIPWIRE_FILE` stays a PERMANENT-waived environment read outside decision 4's classes, and it can change a T4 verdict

**Claim.** The sprint was meant to remove environment switches from tests, except the named classes. The waiver on `internal/playback/s3_2b_2_corpus_test.go:203` gives the reason "an override of the ceilings' file, a path and not an update switch". The comment at `:201` says it is "for the gate's proofs".

**Evidence.**
- `git grep CEP_TRIPWIRE_FILE` finds no setter in `gate/`, `tools/`, `.github/`, the Makefile or `cmd/`, only docs. The tripwire proofs plant `gate/tripwire.json` in their world (s3-2b-2 design :455).
- `gate/runner/env.py:10-14` drops only `GO*`, `PYTHON*`, `RUFF_*` and `GIT_*_PATHSPECS`. So a `CEP_TRIPWIRE_FILE` in the caller's shell reaches the T4 `tripwire` row and swaps its ceilings.
- Decision 4 of the plan check: "the golden-update switches become a test flag, and the planted panics and child-process markers become PERMANENT in-code waivers". A path override is neither.
- `CEP_DETERMINISM_PLANT` (`:299`) also stays an environment read. Its value is never read by the gate, because the proofs rewrite its text as an anchor (`test_tripwire_planted.py:66-67`), and it fails safe.
- The deviation is recorded in 200d8b9's message ("stay env reads"), but not as an owner decision.

**Consequence.**
- A local strict-gate run can pass the tripwire with ceilings that CI never sees.
- A PERMANENT waiver states a use that does not exist.

**Fix.**
- Delete the `CEP_TRIPWIRE_FILE` read: the proofs already plant the file. If a manual override is wanted, make it a test flag.
- For the plant, use a flag, or a `const determinismPlant = false` anchor that the two proof drivers rewrite. The drivers are law files, so the coordinator edits them.
- Otherwise, bring both to the owner as exceptions to decision 4.
- To the backlog: drop `CEP_*` names in `clean_env`. This hazard predates the sprint.

### CA-S4 (should-fix, owed at G5): the ADR and DEVELOPMENT.md updates are incomplete, and design §9's drafts are not enough

**Evidence.**
- The branch does not touch `docs/architecture/adr/ADR-0015-…`.
- DEVELOPMENT.md changed one line. `:5` still says "Not built yet: the suppressions checker, `todocheck` and golangci-lint's `smells`".

**Needed, beyond design §9's drafts:**
- **§2 `smells` row (:108):**
  - its tiers are T0–T4, not "T0 (staged) / T1";
  - "`panic` outside an allowlist" became per-line PERMANENT in-code waivers (plan check 5);
  - tests are exempt from `fmt.Print*` (W2.2).
- **§2.1:** the W2.2 exemption. The decision text itself says "ADR-0015 §2.1 is annotated".
- **§3 item 10 (:140), not in §9:**
  - it still lists "forbidigo-`panic`" as an allowlist;
  - `tools/allowlists/smells.json` joins the grammar: rule `smells`, TEMP only, with the part-level check in `aging`.
- **The tool-scan sentence in the S1.1c-4 annotation (:81), not in §9:** it is now built, and should say so:
  - tools.json's `"go_built": true` is a format change (plan check 8);
  - the scan checks `stdlib` only, never `toolchain`;
  - the 15 dated exemptions sit under S1.1c-5-4;
  - `tools.json` is counted once (W1.2).
- **DEVELOPMENT.md:** design §9's edits at `:5`, `:65` and `:70`, plus the new line after `:74`.

**Consequence.** Card acceptance line 7 is unmet until these land.

### CA-L1 (later): the "design's §2.1 document" in the test no longer matches the design

**Evidence.**
- `.golangci.toml`, `golangci.PINNED` and `test_golangci.DESIGN_TOML` are equal; I parsed all three with tomllib and compared them.
- The design note's §2.1 TOML is not equal to PINNED: `pin.differences` names 7 paths. The W2.2 `_test.go` `[fmtprint]` rule is missing from the design.
- `test_golangci.py:3` still says DESIGN_TOML is "the design note's §2.1 text, copied here by hand".
- `golangci.LINTERS` (`:20`) is a fourth copy of `linters.enable`.

**Judgement.** Three hand copies on law paths are a sound piece of friction: each edit shows in LAW Δ, and the `LINTERS` copy fails closed through the `rep.enabled` check. The stated provenance is false, though.

**Fix.**
- Annotate design §2.1 with the W2.2 rule, or re-word the docstring.
- Derive `LINTERS` from `PINNED`.

### CA-L2 (later, a question for the owner): one decision costs 26 identical waivers

**Evidence.**
- `internal/model/canonical.go` holds 26 identical PERMANENT `//nolint:forbidigo,lll` waivers in 144 lines.
- In-code `//nolint` lines went from 0 to 76 across the tree: `//nolint:forbidigo,lll` 41, `//nolint:forbidigo` 32, `//nolint:lll` 2, `//nolint:gocyclo,lll` 1.

**This is not a defect.** It follows plan-check decision 3 and the G1 ruling that every waiver goes on its own line.

**Consequence.**
- One decision dominates LAW Δ's in-code waiver count.
- Every future `reflect` line in the encoder needs another copy, which dilutes the signal of a genuinely new waiver.

**Option for the owner.** One pinned exclusion rule, `{path = "^internal/model/canonical\\.go$", linters = ["forbidigo"], text = "\\[reflect\\]"}`, reviewed once in LAW Δ.

## Checked and sound (no finding)

**Lane C's string splits keep every value.**
- I compared the folded string constants (every literal and `+` chain, multiset per commit, production files) before and after each of lane C's 12 commits, using the scratch `strfold` tool. They are equal everywhere except:
  - dec066a: one more import-path literal, the moved file's import;
  - c6deb91: `Fprintln(x)` became `printf(x + "\n")`, which gives the same output.
- 64a4bb9 (shape, lane D) changes its literals by design, and is characterised by `corpus.golden`.

**Engine refactors.**
- `recorder.go`: `b := append(r.buf[:0], …)` became `b := r.buf[:0]; b = append(b, …)`. That is the same semantics, and `TestRecordsDoNotAlias` was red first.
- `archcheck`: exit 2 on a failed write is red in a blocking row.
- The golden and baseline switches are now test flags, and `regen.py`'s `-args -baseline-write=` follows them. No stale `CEP_UPDATE_GOLDEN` or `CEP_BASELINE_WRITE` reference is left outside history docs.
- The plant anchor still matches once after its waiver was appended.

**Public API.**
- Nothing under `api/` changed, and `apipartition` was green at T3 for `cep`, `io` and `io/inmemory`.
- `apidiff` runs at T4.

**Contracts.**
- **The `smells` row:**
  - the config is pinned whole before any lint;
  - it uses one environment per GOOS and removes a stale report;
  - it checks the exit code against the report, and checks `Report.Error`, `Warnings` and the enabled set;
  - a typecheck finding, or a finding at a file no package claims, is never dropped;
  - a size rule's liveness is checked only on a clean full-tree run;
  - covered is counted from `go list`, with testdata counted as data;
  - build-ignored files and nested modules are red.
- **The part-level check (`parts.py`):** its rules are implemented, and it fails closed when the base has no `known-issues.json`. `decisions.md` is a law path.
- **`vulns`:** tools.json is counted once, and binaries are checked against `stdlib` only.
- **`--allow-parallel-runners` (W1.1):** sound. golangci-lint's cache uses the Go build-cache format, which is built for concurrent use, and each planted world has a fresh cache.

**Layering.**
- New edges: `golangci→pin`, `parts→{allowlists, git, knownissues, sprintid}`, `rows/suppressions→golangci`. The last is the single reader of the config, as designed.
- No cycle.
- Each law `code` list matches its row's import closure, held by the L.1 test.

## Pre-mortem (the-fool): "S1.1c-5 merged; a month later the lint set let a real smell into main. How?"

1. **The most likely: through an exclusion, by design.**
   - There are 31 TEMP (package, rule) exclusions, the root package's `funlen` and `gocyclo` among them. A new 150-line, complexity-25 function lands in the root package or `internal/runtime` during S1.1c-6. The row is green, and the per-entry count in the green line rises from N to N+1, but nobody diffs it.
   - Decision 1 rejected maxima, so this is accepted residue (design §10).
   - Early warning: the counts per (package, rule) in the summary rise between closes.
   - Cheap mitigation, if the owner wants it: the aging row already reads the merge base, so it could print the delta per entry against the base in LAW Δ. That makes the growth loud, not red.
2. **Through a wrong-line waiver after a tool upgrade, or under a checkout path with a space.** That is CA-S2. The lexical refusal still catches the standalone form, so the miss needs a widened range golangci adds that the lexer cannot see. Unlikely, but silent.
3. **Through a "pure reflow" quoted as proven.** That is CA-S1. Someone runs `astequal` with a list quoted as one argument, gets 0, and the record says "AST-equal". A behaviour change rides in a style commit. The gate still runs conformance and determinism, so a semantic change is likely caught, but not a message or ordering change.
4. **Through an environment-dependent verdict.** That is CA-S3. A local T4 run passes the tripwire with an override file. It is the same failure class the getenv ban exists for.

## Verdict

1. The combined build matches the design and the recorded decisions. Every deviation is recorded: tools.json counted once, `--allow-parallel-runners`, the `fmtprint` test exemption, waivers naming `lll`. Lane C's reflows provably keep every string value. I found no blocker.
2. Four should-fix items before G5: CA-S1 (`astequal`'s empty passes and false reds), CA-S2 (fail closed in `silenced()`), CA-S3 (remove or decide the two environment reads) and CA-S4 (the ADR and DEVELOPMENT.md updates, beyond §9's drafts).
3. Two later items, CA-L1 and CA-L2. No drift in layering or ownership. I make no apidiff claim beyond `apipartition` at T3.
