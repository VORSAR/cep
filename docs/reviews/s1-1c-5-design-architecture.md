# S1.1c-5 design note v1 — blind architecture review (with the-fool)

> **Historical — S1.1c-5, the strict lint set: the instrument, the whole-tree rules, the first packages; merged in PR #47.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-5-plan-check.md) · [design](../sprints/s1-1c-5-design.md) · [record](../sprints/s1-1c-5-record.md) · reviews: [research lint](s1-1c-5-research-lint.md), [plan check architecture](s1-1c-5-plan-check-architecture.md), [plan check qa](s1-1c-5-plan-check-qa.md), [design architecture](s1-1c-5-design-architecture.md), [design qa](s1-1c-5-design-qa.md), [combined architecture](s1-1c-5-combined-architecture.md), [breakit qa](s1-1c-5-breakit-qa.md). **← [S1.1c-4](../sprints/s1-1c-4-record.md) · [S1.1c-6 (next)](../sprints/sprint-plan.md#status) →**

Tree: `s1-1c-5/integration` @ 215bb3b (the note cites bda6554; 215bb3b adds only docs). Every code claim was checked with grep or by reading the file on this tree. I could not read the GitNexus freshness resource from this session, so no `index.commit` is quoted. The 20-minute deadline cut the investigation short; items marked UNVERIFIED were not measured. Hindsight was not searched; the decisions were read straight from `docs/sprints/decisions.md:919-957`.

## Findings

### Blocker

**DA-B1. At T0 the row is red on every commit through step 8 ("waives nothing").**
- *Claim.* §2.7 lints only the staged files' packages. §2.3 step 8 reds every size exclusion that dropped nothing, and it is "judged only when both runs completed with no fault of steps 1–5". A T0 run completes without a fault. So every `smells.json` entry for a package that was not staged drops nothing and goes red. With the exclusions written at G1→T3, every commit's pre-commit hook would be red.
- *Evidence.* Design §2.3 step 8 (`s1-1c-5-design.md:190`) and §2.7 (`:236-238`). The vulns precedent the note cites (`rows/vulns.py:142-155`, `complete`) is about a scan that was cut short, not about a subset of packages.
- *Consequence.* The T0 hook is unusable. The likely workaround (`--no-verify`, or weakening step 8) erodes the gate. This breaks the contract that the row is T0–T4.
- *Fix to the note.* Judge step 8 only when the run linted `./...` (T1 and up). At T0, judge it only for entries whose package was linted, or skip it and print a line. Add one test: a T0 run with one staged clean package and an unrelated exclusion is green.

### Should-fix

**DA-S1. `go list` and golangci-lint run in different build contexts.**
- *Claim.* Step 2 runs `go list` with `CGO_ENABLED=0`. That is inferred from `rows/gomods.py:22`, whose `ENV` is `GOPROXY=off`, `GOFLAGS=-mod=readonly`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`. Step 3 sets only `GOOS` and `GOARCH` for golangci-lint.
- *Consequence.* On darwin, a native run defaults to `CGO_ENABLED=1`, and a linux cross run defaults to 0. A file such as `//go:build !cgo && darwin` is counted as covered by `go list` but not linted by golangci-lint: a missed detection with the covered count still equal.
- *Fix.* Both calls get the same environment: `gomods.ENV` plus `GOOS`/`GOARCH` (and `GOFLAGS`, `GOTOOLCHAIN`, `GOPROXY`). Name the cache directory too (see DA-S8). Add a planted case: a `!cgo`-tagged file with a finding is red under both runs.

**DA-S2. Rule 5 of the part-level check (merge-base read, R-9) repeats the LAW Δ, is weaker than it, and is inconsistent with lawdelta.**
- *Evidence 1.* `gate/known-issues.json` and `tools/allowlists/` are already law paths (`gate/law.json` `paths`). Any forward move therefore appears in the LAW Δ, which the owner reads, and only the owner merges.
- *Evidence 2.* Rule 5 accepts any `decision` that `aging.decided` resolves (`aging.py:32-37`). `docs/sprints/decisions.md` is not a law path, so the mechanical check can be satisfied by anyone who writes a section naming the id.
- *Evidence 3.* A `None` base "prints … moves not checked" (design `:296`). The lawdelta row reds the same condition (`rows/lawdelta.py:29`, `NO_BASE`; owner decision 9 of 2026-11-10: never silent).
- *Evidence 4.* §6 itself relies on the LAW Δ alone for decision 8's owner-only renewal (`:327`). The two owner-only edits are treated differently.
- *Fix (preferred).* Drop rule 5 and the git read from the `suppressions` row. State that the owner-only move is held by the LAW Δ, as decision 8 is.
- *If kept.* Use `git.merge_base()` (it exists, `gate/runner/git.py:23-29`; the note's "inferred" can go), and make a missing base red at T4, as lawdelta does.

**DA-S3. The part-level check sits in the wrong row; R-5 is a symptom of that.**
- *Claim.* `parts.forwards` is aging for exclusions. The `aging` row already reads `gate/known-issues.json`, `gate/sprints.json`, `gate/current-sprint` and `docs/sprints/decisions.md`, and already counts them in its `expected_scope` (registry row `aging`, tiers T3/T4/nightly). `parts.py` imports `aging.decided` anyway.
- *Consequence.* Hosting the check in `suppressions` widens that row's inputs (R-5) and adds git (R-9). It also spreads one exclusion's lifecycle over three rows: `smells` applies it and judges "waives nothing", `suppressions` checks the grammar, the part and the move, and `aging` keeps the issue clocks.
- *Fix.* Call `parts.forwards` from `rows/aging.py`, adding `tools/allowlists/smells.json` to aging's `expected_scope` and law `code`. Give `rows/aging.py` to lane B. R-5 then disappears. The below-T4 "countdown line" shows at T3 only, which is enough.

**DA-S4. R-2 understates the aging effect of printing S1.1c-5…-8.**
- *Claim.* Rule 1 counts the closed rows after `first_seen`'s position (`aging.py:46`). `index` places an unprinted part at its row (`sprintid.py:65-72`). The open issues today are S3.3-2-1, -2 and -3 (no decision), plus **S1.1c-3-4** (first seen S1.1c-3, which sits at row S1.1c, before the printed parts, with no decision).
- *When each turns red.*
  - All four turn red at S1.1c-7's close.
  - The four new S1.1c-5-n issues (first seen S1.1c-5, `decision` "") reach three closed rows at S1.1c-8's close.
  - S1.1c-5-4 (the tool scan, forward "") may well still be open then, so it turns red too.
- *Consequence.* Predictable red on main's nightly at a close-out. Ground rule 11 makes that stop-the-line.
- *Is printing still right?* Yes. Each part is a sprint (one PR, ≤ 2 weeks), so counting parts as rows is the honest clock, with S1.1a-1/-2 as the precedent. The plan check also decided the printing (`s1-1c-5-plan-check.md:86`).
- *Fix.* R-2's list names all eight ids (the four above and S1.1c-5-1…4). The G1 decision section names S1.1c-5-1…4 as whole words, so the `decided` check (`aging.decided`, `aging.py:32-37`) holds. The note recommends that the owner decides S3.3-2-1/2/3 and S1.1c-3-4 before S1.1c-7's close. Add a test: with the four parts printed and S1.1c-5…-7 ticked, rule 1 names exactly the undecided ids.

**DA-S5. The instruments' law `code` lists miss modules they will import.**
- *Evidence.* Today: `suppressions` code = allowlists, anchors, rows/__init__, rows/suppressions, sprintid, suppressions. `vulns` code = allowlists, deps, rows/__init__, rows/gomods, rows/vulns, vulndb, toolchain/vulndb_fetch (`gate/law.json` instruments).
- *What the note adds and what it leaves out.*
  - `smells`' code (§2.5, `:222-223`) omits `gate/runner/pin.py` (which `golangci.differences` is built on, §5) and `gate/runner/rows/gomods.py` (its ENV is reused).
  - `suppressions` gains golangci.py and parts.py (`:229`) but not pin.py, aging.py, knownissues.py or git.py.
  - `vulns` will read `go_built` through `gate/toolchain/manifest.py`, which is not in its code.
- *Fix.* List each instrument's full import closure, and add a test that derives the closure from the imports and compares it with `code`.

**DA-S6. The `package_clause` scanner is a second Go parser where `go list` already gives the answer.**
- *Claim.* Step 6(b) adds `golangci.package_clause`, which reuses `suppressions.go_comments`. The row already holds §2.4's `go list` listing, whose `Name` gives each file's package. For `XTestGoFiles` the package is `Name + "_test"`.
- *Fix.* Map each file to `(import path, package name)` in `go_files` and drop `package_clause`. That also removes the import from golangci.py to suppressions.py, which crosses lanes.

**DA-S7. The tool scan's parser must accept the main module's empty sum.**
- *Evidence.* Line 3 of the recorded build info (`s1-1c-5-scratch/golangci-lint.buildinfo.txt`) is `\tmod\tgithub.com/golangci/golangci-lint/v2\tv2.15.0\t`: the sum field is empty. §6 (`:326`) writes the line as `\tmod\t<path>\t<version>\t<sum>`.
- *Consequence.* A strict four-field parse breaks every run. A lax parse needs a test.
- *Fix.* Name the empty-sum form in the note, and use the recorded file as T-C fixture data (208 lines with `dep`; build `CGO_ENABLED=0`, `GOOS=darwin`).
- *Lane note.* The new `pseudo=` must narrow the judgement at `vulndb.py:276` only, not the DB entry's shape check at `:271`. A `toolchain` entry in the database stays well-formed. (UNVERIFIED beyond reading those two lines.)

**DA-S8. A function-level `//nolint:forbidigo` over the canonical encoder (R-8, §7.1) would waive every pattern in the body.**
- *Claim.* nolint names a linter, not a pattern. A `//nolint:forbidigo` above `encode` or `encodeStruct` would also silence any later `panic`, `os.Getenv` or `fmt.Print` inside the function. That is the waiver widening that decision 7 refuses in spirit.
- *Fix.* Put the waivers per line, about 33, made honest by moving the encoder to `canonical.go` (decision 3). Alternatively, allow a function-level waiver only if the row proves that the waived body's findings are all `[reflect]`, which is over-built. Recommend per line.

**DA-S9. The card's acceptance line 7 names "the T4 run" as proof, but `astequal` is record-only (R-7).**
- *Fix.* Keep it out of the rows. At G5 the coordinator re-runs `astequal <c>^` for each move or reflow commit of lane D, the cheapest proving run, and quotes the exit codes in the record. Say plainly that AST equality proves moves and reflows only. Splits are proven by the characterisation tests.

### Later

- **DA-L1.** The golangci-lint cache directory is unnamed. If the runner gives the row a fresh HOME or `row_dir`, every run is cold (about 2 × 17 s at T0; R-6). Pin `GOLANGCI_LINT_CACHE` to the tool cache, and run the planted driver with an empty cache.
- **DA-L2.** W0b owns "new files only", but `gate/runner/tests/test_sprintid_parts.py` already exists. Name it as an edited file with its owner.
- **DA-L3.** Step 6(c) speaks of expiry "at T4/nightly", but the `smells` row has no nightly tier (§2.5). The `suppressions` row judges expiry at nightly, so nothing is lost. Reword.
- **DA-L4.** §6 scans only the run's own platform binary, so the other platform's golangci-lint is never read. Both come from one release build, but name it as residue.
- **DA-L5.** Possible default directory skips in golangci-lint v2 (`examples/`, `third_party/`, …): add one planted package under such a name to `test_smells_planted.py`. UNVERIFIED whether v2 still skips any.
- **DA-L6.** §10 Q-A is answered (`decisions.md:953-957`). Mark it closed in v2 of the note.

## Answers to R-1 … R-9

- **R-1.** Use a row rule, not entries. A tracked `.go` file with a `testdata` path segment is accounted as data and counted in the green line. The Go toolchain never compiles `testdata`, so nothing there can ship, and no hole opens. This removes the `unlinted` target form, its PERMANENT grammar, its stale check and its "linted and listed" check. Every other unclaimed file stays red.
- **R-2.** Yes, name them. The list must hold all eight ids: S3.3-2-1/2/3, S1.1c-3-4 and S1.1c-5-1…4 (DA-S4). Keep the printing.
- **R-3.** Leave `log.Print*` out, as ADR-0015 §2 does, and put it on the backlog. Keep `os.Environ`: it is the same smell, has zero uses today, and adds no churn.
- **R-4.** Rewrite rather than waive if every site is the `a = append(b[:0], …)` reuse idiom. The rewrite is a two-statement form that keeps the alias and needs no waiver. Prove it with the playback tests and lane C's characterisation run. Six PERMANENT waivers for style are waiver inflation.
- **R-5.** The question goes away if the check moves to the aging row (DA-S3). Otherwise, yes: count the inputs.
- **R-6.** Keep both GOOS at T0, as decided, with a warm named cache (DA-L1) and DA-B1 fixed. Measure before asking anything. The staged-package mapping (`./<dir>` per staged directory) is right, because golangci-lint lints whole packages.
- **R-7.** Record-only is right for a one-sprint calibration tool. Add the G5 re-run (DA-S9).
- **R-8.** W0 pins each of the listed items. For the function-level `//nolint`: whatever W0 measures, choose per-line waivers (DA-S8).
- **R-9.** Drop it, because the LAW Δ is the control (DA-S2). If the owner keeps it, a missing base is red at T4, like lawdelta.

## Pre-mortem (the-fool): "S1.1c-5 merged green; six weeks later a finding the set should catch reached main. How?"

1. **Most likely: inside a TEMP-excluded package.** Someone wrote a new 150-line, cyclomatic-18 function in `internal/runtime` while its exclusion was live. Per-package exclusions have no ratchet, by owner decision 1 ("not recorded as maxima"). Mitigation within the decision: print the per-(package, rule) dropped counts in the row's green line, and have the record diff them across closes, so growth is loud.
2. **A widened waiver.** A function-level `//nolint:forbidigo` on the encoder swallowed a new `panic` (DA-S8).
3. **A build-context gap.** A cgo-tagged file was counted as covered but never linted (DA-S1).
4. **The T0 hook bypassed.** The DA-B1 false red trained people to commit with `--no-verify`. T1 still catches it, so this costs time, not a merge.
5. **Out of scope by design.** Go code under `gate/proofs/**` is never linted. Accept and name it.

## Verdict

The seams deliver the ten decisions and the TOML answer. Ownership is one per file, and there is no import cycle (golangci→pin, rows/suppressions→golangci/parts, parts→sprintid/allowlists/aging).

There is one blocker, DA-B1: at T0, step 8 reds every unstaged exclusion.

The rest is simplification and closing gaps:
- DA-S2/S3: move the part-level check to the aging row and drop the merge-base read.
- DA-S1: one build environment for `go list` and golangci-lint.
- DA-S4: the full aging id list.
- DA-S5: complete the `code` lists.
