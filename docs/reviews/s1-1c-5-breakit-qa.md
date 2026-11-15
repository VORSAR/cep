# S1.1c-5 combined build: QA break-it

> **Historical — S1.1c-5, the strict lint set: the instrument, the whole-tree rules, the first packages; merged in PR #47.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-5-plan-check.md) · [design](../sprints/s1-1c-5-design.md) · [record](../sprints/s1-1c-5-record.md) · reviews: [research lint](s1-1c-5-research-lint.md), [plan check architecture](s1-1c-5-plan-check-architecture.md), [plan check qa](s1-1c-5-plan-check-qa.md), [design architecture](s1-1c-5-design-architecture.md), [design qa](s1-1c-5-design-qa.md), [combined architecture](s1-1c-5-combined-architecture.md), [breakit qa](s1-1c-5-breakit-qa.md). **← [S1.1c-4](../sprints/s1-1c-4-record.md) · [S1.1c-6 (next)](../sprints/sprint-plan.md#status) →**

Branch `s1-1c-5/integration` @ c6deb91 (base `main` 0fb2b53). Read-only. Every plant was made in a copy under
`/Users/ramith/code/cep-wt/s1-1c-5-scratch/breakit/`, using the pinned golangci-lint v2.15.0, Go 1.27.4 and Python 3.15.2.
The detached worktree I added there is removed. The branch's `git status` is clean.
I used neither GitNexus nor Hindsight, to stay inside the time box. Code claims were checked by running the real tools.

## Blocker

### BK-B1: any tracked Markdown heading counts as the owner's decision to move a forward (rule 5 of the part-level check)
- **Claim.** Design §3.5 5(a) says a later forward is red unless the issue's `decision` resolves to "a decisions.md
  section naming the id". `parts.forwards` is handed `aging.decided` with `rows/aging.py`'s `read`, and that reader
  serves any tracked `.md` file (`gate/runner/rows/aging.py:65-71`; `gate/runner/aging.py:32-37`, "a heading of
  tracked Markdown"). So a heading in the sprint record, the design note or a review file "decides" a move. Agents
  write all of those.
- **Plant.** In a copy: Base = the branch's own smells.json and known-issues.json, treated as already merged; marker
  S1.1c-6; tier T4. The plant moves S1.1c-5-2's forward from S1.1c-7 to S1.1c-8.
  - Control: the move with the decision left unchanged is red:
    `aging: smells.json: internal/event file-length-limit: forward moved S1.1c-7 → S1.1c-8 (issue S1.1c-5-2 → S1.1c-5-2) without a decision that names S1.1c-5-2 (decision 1: only the owner moves a forward)`
  - The plant also sets the decision to `docs/sprints/s1-1c-5-record.md#decisions-at-the-gates`. That section names
    S1.1c-5-2 on line 41. Result: green, with no fault for any of the 12 internal/event and other S1.1c-5-2 entries.
- **Failure scenario.** In S1.1c-6, a lane moves an exclusion's deadline by one or two sprints without the owner. It
  edits known-issues.json and points `decision` at a heading the sprint's own docs already have, or at one it adds.
  T4 is green. This is the same class as S1.1c-2's CA-S3, which was fixed only for PERMANENT refs
  (`allowlists.ref_fault` limits them to an ADR or docs/sprints/decisions.md). The cap still holds: no forward can
  pass S1.1c-8. Rule 1 of the aging row shares the same reader; that part is older than this sprint.
- **Fix, red first.** Add `gate/runner/tests/test_parts.py::test_a_forward_moved_with_a_decision_outside_decisions_md_is_red`.
  Its decision is a heading of `docs/sprints/s1-1c-5-record.md` that names the id; it is red today. Add the witness
  `..._in_decisions_md_is_green`. Then the `decided` passed to `forwards` accepts only `docs/sprints/decisions.md#…`,
  as the design says, or also an ADR if the owner prefers `ref_fault`'s set. Ask the owner whether rule 1 should be
  narrowed the same way.

## Should-fix

### BK-S1: the smells row's wrong-line waiver check fails open, silently
- **Claim.** `silenced()` (`gate/runner/rows/smells.py:141-162`) skips every `found ignored range` line it cannot
  parse (`continue` at :147-148 and :152-153). It cannot parse two kinds of real output from the pinned binary:
  - (a) A repository root whose path contains a space. `SILENCED_AT` needs `(/\S+?\.go)`.
  - (b) An issue with a non-nil `LineRange`. Revive sets one, which covers argument-limit and file-length-limit, and
    the line then prints `0x…`, not `<nil>`.
- **Plant (a), the real row end to end.** The same tree in two places, with one plant appended to
  internal/clock/clock.go:
  `//nolint:forbidigo // planted: waiver on the line above` sitting alone above `panic("planted")`.
  Both runs used `GATE_TIER=T1 python3 -I -B -X utf8 gate/runner/rows/smells.py`.
  - In `.../breakit/wt`:
    `failed: smells: internal/clock/clock.go:251:2: forbidigo: silenced by the //nolint on line 250; a //nolint waives only its own line`, then exit 1.
  - In `.../breakit/sp ace/cep`: `GATE_COVERED 269`, the green summary line, and exit 0.
- **Plant (b).** A module with `//nolint:revive // planted…` alone above `func F(a, b, c, d, e, f int)`.
  golangci-lint exits 0 with `Issues: []`. Stderr has
  `found ignored range for issue &{revive argument-limit: … a.go:6:1 0x53efba923460 0 …}: {[revive] map[] {4 6} …}`.
  `silenced()` returns only the forbidigo plant and never the revive one.
- **Failure scenario.** Today the suppressions row's standalone refusal still catches both planted forms. I confirmed
  `internal/clock/clock.go:250: //nolint stands alone on its line…`. So this is the backstop failing, not lost
  detection, and CI paths have no spaces. But the row's green line claims a check it did not make. A change in the
  debug format after a tool upgrade would also go silent for every linter except forbidigo; only planted case 17
  guards that, and only for forbidigo.
- **Fix, red first.** Add these to the smells row tests, using stderr fixtures copied from the pinned binary:
  - `test_a_wrong_line_waiver_under_a_root_with_a_space_is_red`
  - `test_a_wrong_line_revive_waiver_is_red` (the `0x…` LineRange form)
  - `test_an_unparseable_silenced_line_is_a_fault`

  Then find Pos by the literal `root + "/"` prefix with `(?:<nil>|0x[0-9a-f]+)`. A line that contains `SILENCED` and
  does not parse becomes a fault, never a `continue`.

### BK-S2: `tools/astequal` says "equal" for changes in behaviour
- **Claim.** Keys are `<package dir>: <decl>`. Build context that lives in the file name (`_test.go`, the
  `_<GOOS>`/`_<GOARCH>` suffixes) and the package name (`p` against `p_test` in one directory) are not part of the key
  or the text. Paths named on the command line but absent on both sides are dropped.
- **Plants** (a scratch git repo, `.../breakit/aerepo`):
  - `func F` (public API) moved from p/a.go into p/a_test.go: `exit 0`. F leaves the production build.
  - `git mv p/h_linux.go p/h_windows.go`: `exit 0` with no paths, and `exit 0` with both paths named. H is no longer
    built on linux.
  - Control: swapping the bodies of H in h_linux.go and h_darwin.go gives `astequal: p: func H differs`, exit 1.
  - The known case is confirmed: `astequal main "internal/model/canonical.go internal/model/model.go"` gives exit 0.
    So does `astequal main internal/model/nope.go`, a typo'd path.
  - Correctly exit 2: a revision that does not exist, a non-Go file (`README.md:1:1: illegal character`), and an
    empty path `""`.
- **Failure scenario.** At G5 the coordinator quotes `astequal <commit>^` exit codes as evidence that a commit was a
  pure move or reflow (design §7.4, §8 7.5). A move into a test file, a rename that changes the platform, or a
  mistyped path list gives a quoted "exit 0" for a commit that changed what builds.
- **Fix, red first.** Add to `tools/astequal/astequal_test.go`:
  - `TestMovedIntoTestFileUnequal`
  - `TestMovedIntoExternalTestPackageUnequal`
  - `TestGOOSSuffixRenameUnequal`
  - `TestNamedPathAbsentOnBothSidesIsExit2`, which covers both the unsplit list and the typo

  Then key each declaration by (dir, package name, `_test`, filename GOOS/GOARCH, `//go:build`), or put that file
  context into its text. A named path that exists on neither side is a usage error.

### BK-S3: `astequal` reports the lane's own pure move dec066a as unequal
- **Claim.** Imports are keyed per package and numbered `#dup<n>`. A move into a new file that needs an import the old
  file keeps adds `#dup2`, so the result is "differs".
- **Plant.** Work tree at dec066a, `astequal dec066a^` gives
  `astequal: internal/model: import "github.com/vorsar/cep/internal/event"#dup2 differs`, exit 1. A copy that prints
  every differing key lists that key and nothing else, so the move is pure apart from the import.
- **Failure scenario.** The G5 re-run quotes exit 1 for a pure move. That gives a reason to dismiss astequal's exit
  codes in general, which is the opposite of what the evidence is for. `TestMovedFunctionAcrossFilesEqual` does not
  cover a moved function that needs an import.
- **Fix, red first.** Add `TestMoveIntoNewFileSharingAnImportEqual`, in dec066a's shape. Compare imports as a set per
  (package dir, package name, local name).

## Later

- **BK-L1.** The wrong-line message names the comment group's first line, not the directive's line. In the revive and
  forbidigo plant the directive is on line 9 and the message says "silenced by the //nolint on line 8", which is the
  doc comment. Name the directive's line from the source.
- **BK-L2.** `astequal <rev>` compares `<rev>` with the work tree. The G5 step "re-run `astequal <commit>^` for each
  lane-D commit" is only right with the work tree checked out at `<commit>`, which is how I ran it. At the
  integration tip it would compare against every later commit. Either add an `<after-rev>` form or write the checkout
  into §7.4.
- **BK-L3 (a question for the owner, not a defect).** A size exclusion is package-wide with no count cap. A new
  over-long function added to an excluded package, for example `.` or `internal/event`, is dropped silently until
  that package's forward part. The only trace is the per-(package, rule) count in the green line, which the design
  says to compare between closes (DA pre-mortem 1). Today the counts are 139 issues across 31 exclusions. Is a ratchet
  (a count per entry) wanted?

## Attacks that held
- **Config pin.**
  - The whole document is compared type-strictly. Any extra key is red; that includes case variants such as
    `[Issues]`, which viper would merge.
  - golangci-lint runs with `--config .golangci.toml`, so no other config file is read.
  - `uniq-by-line = false` and `max-issues-per-linter`/`max-same-issues = 0` are pinned.
  - `GOFLAGS`, `CGO_ENABLED`, `GOPROXY` and `GOTOOLCHAIN` are overridden by `gomods.ENV`.
- **No directory is skipped by default.** `third_party/`, `builtin/`, `Godeps/`, `examples/`, `node_modules/` and
  `vendorx/` were all reported by the pinned binary.
- **Waivers.**
  - `//nolint:lll` on a short line is red through nolintlint: `directive //nolint:lll … is unused for linter "lll"`.
  - A trailing `//nolint` followed by a comment line in the same group does not silence the next line (the range
    stays at the directive's line).
  - A standalone `//nolint` is refused by the suppressions row.
  - The comment lexer `go_all_comments` matches `go/parser` exactly on 12 tricky literal forms: escaped quotes,
    `'"'`, `"/*"`, a multi-line raw string, a block comment holding a quote, and `'\\'`.
- **Generated, ruff and vulns.**
  - A generated.json target without the marker is red (stale).
  - ruff runs with `--config ruff.toml --ignore-noqa`, so neither hierarchical configs nor `# noqa` can waive.
  - Removing golangci-lint's `go_built` is red at T2: `test_golangci_lint_is_declared_go_built`.
- **Lane D's packages.** table, clock, inmemory and shape hold no smells.json entry.
- **Engine refactors.**
  - recorder.go: `b := r.buf[:0]; b = append(b, X)` is equivalent to `append(r.buf[:0], X)` at all six sites.
  - regen.py passes `-args -baseline-write=<dir>`, and no code still reads `CEP_UPDATE_GOLDEN` or
    `CEP_BASELINE_WRITE`.
  - archcheck's `report` keeps the first write error, so a failed write is exit 2 on all three paths.
  - The message strings split into two parts in archcheck have the same value as before.
  - 12be398 gives astequal exit 0.
- **Not reached in the time box.** Two concurrent `--allow-parallel-runners` runs sharing one cache; T0's staged-only
  run beyond reading the code; the census row by row; `go version -m` edge cases beyond reading the code.

## Verdict
1 blocker: BK-B1. A forward can be moved with a "decision" taken from any tracked Markdown, which reopens the CA-S3 class in the part-level check.
3 should-fix: BK-S1, the wrong-line check failing open (a path with a space, revive); BK-S2, astequal saying "equal" for moves into tests and platform renames; BK-S3, a false "unequal" on dec066a. There are 3 later items.
The smells row's core verdict held under every config, skip-directory and waiver plant; the gaps are in the checks behind it and in astequal's evidence.
