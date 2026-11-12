> **Historical — S1.1c-2, the exemptions and the teeth; merged in PR #35.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-2-plan-check.md) · [design](../sprints/s1-1c-2-design.md) · [record](../sprints/s1-1c-2-record.md) · reviews: [plan check: architecture](s1-1c-2-plan-check-architecture.md), [plan check: QA](s1-1c-2-plan-check-qa.md), [plan check: product](s1-1c-2-plan-check-product.md), [design: architecture](s1-1c-2-design-architecture.md), [design: QA (testability)](s1-1c-2-design-qa.md), [combined build: architecture](s1-1c-2-combined-architecture.md), **break-it: QA**. **← [S1.1c-1](../sprints/s1-1c-1-record.md) · [S1.1c-3 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-2 combined-build review, 2026-11-12 — QA break-it.** Read blind against the integration branch @ e7b5d89 (code vs `main` @ fd274e4); every finding's disposition is in [the record's findings](../sprints/s1-1c-2-record.md#findings) and the owner's answers in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-review-and-break-it). Sibling: [architecture](s1-1c-2-combined-architecture.md).

# S1.1c-2 combined build: QA's break-it (sprint model §6 step 7)

Branch `s1-1c-2/breakit` at /Users/ramith/code/cep-wt/s1-1c-2-breakit, cut from e7b5d89. 15 commits, tests only: ec78b11..ce362b0. No code was fixed. Red tests are in `gate/runner/tests/test_breakit_s11c2.py` (21 tests: 36 failing subtests, 2 controls green) and `tools/archcheck/breakit_s11c2_test.go` (4 tests red; every other archcheck test stays green). Tools: pinned CPython 3.15.2, Go 1.27.3, ruff 0.17.3, golangci-lint v2.15.0. I did not use GitNexus or Hindsight (no resource reader or memory tool in this session); every claim below comes from a run.

## Verdict

1. **Not ready for G5. 4 blockers.** Each is a silent pass in the build's own subject: waivers and teeth.
2. **Waivers that get through.** Ruff 0.17.3 honours `# fmt:  skip` (two or more spaces, or a tab), and the same for `fmt: off` and `yapf: disable`. The `suppressions` row sees none of these forms, and LAW Δ's count misses them too (BK-B1). Ruff also honours waivers in ruff.toml that the ruff pin never reads: top-level `ignore`, `extend-ignore`, `[per-file-ignores]`, and anything in a file pulled in by `extend` (BK-B2). Both escapes were confirmed end to end on the real rows.
3. **Proofs that are switched off or hollowed while the pin stays green.** A driver's red assertion can be swallowed, made dead or redefined as a no-op (BK-B3). A pinned proof can be kept from running while its case count still adds up (BK-B4): a Go `//go:build ignore`, a Python `load_tests`, a `del` after the class, or a class that is not a TestCase. I checked both on the real tree with `test_instruments_pin.RealTree`.
4. **7 should-fix.** Go dead verdicts, a second BA13 forge route, unused list targets, gosec's `#nosec`, a known-issue id listed twice, the forgeable generated-code header, and a pin that does not tie a proof to its row.
5. **2 later.** docsync reads data-file lines as package listings, and a ref can resolve to a heading inside an HTML comment. Most of the planted attacks were caught (list at the end).

## Blockers

**BK-B1: `# fmt:` followed by extra spaces or a tab is a waiver nobody sees.**
- **Plant:** `X = [1,2,   3]  # fmt:  skip` in a gate/*.py file, with no reason. Also `# fmt:\tskip`, `#  fmt:   skip`, `# fmt:  off`/`# fmt:\toff`, `# yapf:  disable`/`# yapf:\tdisable`.
- **Measured with the pinned ruff:** every one keeps the layout. Ruff only needs whitespace of any length after `fmt:`.
- **What the gate did:** at T4 the ruff row passed and the `suppressions` row passed (exit 0, `GATE_COVERED 1`). The control, canonical `# fmt: skip` with no reason, was red with "write its reason as another comment…".
  - `py_markers` returns `([], [])` for these forms.
  - No PREFILTER pattern matches them, so the LAW Δ exemption count and its sub-lines miss them as well. The edit shows only inside the grouped `gate/` code line.
  - This breaks acceptance lines 1 and 2.
- **Red test:** `test_breakit_s11c2.FmtSkipWhitespace`, commit ec78b11 (3 tests, 14 subtests red). Quoted red: `AssertionError: ([], []) == ([], []) : a waiver ruff honours passes unseen` for `# fmt:  skip  # the table reads…`, and `AssertionError: [] is not true : a region ruff honours passes unseen` for `# yapf:\tdisable`.
- **Proposed fix:**
  - Normalise each segment the way ruff does: strip, then accept `fmt:` + any run of whitespace + `skip`/`off`/`on`, and the same for `yapf:`.
  - Make any non-canonical skip spelling red ("write # fmt: skip").
  - Add `"fmt:"` and `"yapf:"` to PREFILTER (they cover the existing `fmt: skip`/`fmt:skip`/`fmt: off`/`fmt: on` patterns).
  - Add these rows to §0.2's pinned table.

**BK-B2: ruff.toml waivers the ruff pin does not read.**
- **Plants:** in ruff.toml, a top-level `ignore = ["TID251"]`, a top-level `extend-ignore`, a top-level `[per-file-ignores]` table, or `extend = "docs/base.toml"` where that file holds `[lint.per-file-ignores]`.
- **Measured with the pinned ruff,** run the way the ruff row runs it (`check --ignore-noqa --no-cache --config ruff.toml`): each waives TID251. You get a deprecation warning on stderr and "All checks passed!", exit 0.
- **What the gate did:** the real ruff row passed a `time.sleep` with a top-level `extend-ignore` (control without it: `failed: gate/zz_bk_sleep.py: ruff check (exit 1)`). `rows/suppressions.ruff_pin` returns `[]` for all four plants.
  - This breaks acceptance 1: "a ruff waiver without its entry" is green.
  - With `extend`, the waiver file can sit outside every law path (docs/), so later edits to it get no LAW Δ line at all.
  - Narrowing `lint.select` is the same kind of global waiver; ruff.toml's LAW Δ line names it, but the pin does not.
- **Red test:** `test_breakit_s11c2.RuffPinReadsEveryWaiver`, commit 31acab6 (4 red, control green). Quoted red: `AssertionError: [] is not true : docs/base.toml's waiver, read through extend, waives TID251 for gate/x.py`.
- **Proposed fix:**
  - The pin refuses `extend`.
  - It reads top-level `per-file-ignores`/`extend-per-file-ignores` like the `[lint]` ones, or refuses them.
  - It refuses non-empty top-level `ignore`/`extend-ignore`.
  - Ruff's own stderr deprecation warning could also be made a ruff-row failure.

**BK-B3: a hollowed Python proof still counts as red.**
- **Plants:** in a driver case, keep the case count but make the red assertion unable to fail. Six ways:
  - redefine `assertRed` in the driver as a no-op;
  - `with contextlib.suppress(AssertionError): self.assertRed(...)`;
  - `try: … except AssertionError: pass` (ruff's SIM105 flags this exact form, but SIM105's own suggested rewrite is the `contextlib.suppress` form, which passes);
  - `if 0:`;
  - `for _ in ():`;
  - `if True: return` before the assertion.
- **What the gate did:** `teeth.python_cases` returns the case with no fault for all six.
  - Real tree: I cut `assertSuppressionsRed` in `test_suppressions_planted.py` down to `with __import__("contextlib").suppress(AssertionError): self.assertRed(…)`, deleting its three other assertions. `test_instruments_pin.RealTree` printed `OK`.
  - The design's live-code rule (§4.2, DQ-B4) only recognises a literal `if False` and a `return`/`raise` in the same block.
- **Red test:** `test_breakit_s11c2.TeethSeesAHollowedCase`, commit 24c47dd (6 subtests red, control green). Quoted red: `AssertionError: [] is not true : the failure swallowed by contextlib.suppress: counted as a red case`.
- **Proposed fix:**
  - A driver may not define or assign a name in RED/VERDICT.
  - A red call under `with` whose context is not a plain resource, or inside a `try` whose handler catches AssertionError/Exception/BaseException or is bare, does not count.
  - Dead code: any `if <constant>` judged by truthiness, a loop over an empty literal, and statements after a block that always returns.

**BK-B4: a pinned proof that never runs still counts.**
- **Go plant:** add `//go:build ignore` (or `// +build never`) at the top of a pinned Go proof.
  - `archcheck -cases` still lists its cases, because it parses the file and ignores build constraints.
  - Real tree, on `internal/parser/errorcounts_test.go`: RealTree printed `OK`, while `go test -list` no longer listed `TestErrorCountsMatchTheCatalogue`. The unit row sees no event, so nothing goes red.
- **Python plant:** a module-level `load_tests` that returns only the green control, `del Planted.test_red` after the class, or a case moved into a class that is not a TestCase.
  - `python_cases` counts them all.
  - Measured: pyshim reports `pyshim: pkg.test_m: passed 1 tests` (exit 0) for a module with 3 cases, 2 of them failing.
- **Red tests:**
  - `tools/archcheck/breakit_s11c2_test.go` `TestBreakitCasesBuildConstrainedFile`, commit 0b5952a. Quoted red: `--- FAIL: TestBreakitCasesBuildConstrainedFile/go:build_ignore … no fault: a proof file go test never compiles is counted`.
  - `test_breakit_s11c2.TeethCountsWhatRuns`, commits 7014d56 and ce362b0 (3 red). Quoted red: `AssertionError: [] is not true : a load_tests hook: the case is counted but never runs`.
- **Proposed fix:**
  - `Cases` faults a file with any `//go:build` or `// +build` line, or a GOOS/GOARCH filename suffix.
  - `python_cases` faults a module-level `load_tests`, `del`, `setattr` or `__test__`, and only counts classes whose base chain reaches `unittest.TestCase` / `support.GateCase`.
  - Better still, the shim could cross-check: pyshim compares the IDs it ran with the module's statically defined `test*` methods.

## Should-fix

**BK-S1: Go `-cases` has no live-code rule.**
- **Plant:** a case whose only `t.Fatal` sits under `if false {}` or in a closure that is never called.
- **What the gate did:** the case is counted as asserting. A `return` before the Fatal is caught by the vet row's unreachable check; these two are not.
- **Red test:** `TestBreakitCasesDeadVerdicts`, commit ce156c5. Quoted red: `faults []: the Fatal under if false counted as a verdict`.
- **Proposed fix:** give Go the same live-code clause the Python rule has: constant-false `if`, a closure only counts when it is called, and statements after a terminating statement.

**BK-S2: BA13 has a second deliberate forge route.**
- **Plant:** a test module that replaces `unittest.TextTestRunner.run` at import. pyshim's own `_Result`, which is looked up after the import, could be patched the same way.
- **What the gate did:** the shim itself writes `pyshim: pkg.test_m: passed 3 tests` to the report and exits 0, for a module whose only test calls `self.fail`.
  - ADR-0015 §3.6's 2026-11-12 annotation names one route (the module writes the report file).
  - It is the same residue class (every test module is a law path), but the sentence is incomplete, which is misleading.
- **Red test:** `ShimRunnerPatchForge`, commit 6aa6adf. Quoted red: `AssertionError: 0 != 1 : ('pyshim: pkg.test_m: passed 3 tests\n', …, b'…Ran 0 tests…NO TESTS RAN\n')`.
- **Proposed fix:**
  - Bind the runner and the result class before the import, and cross-check `testsRun` against `suite.countTestCases()`.
  - Or the owner widens the residue sentence to "any in-process forge by a test module".

**BK-S3: an unused list target is reported by neither mode.**
- **Plant:** a testhygiene.json target whose file exists but no longer sleeps. The same holds for a purity target whose package no longer imports the identifier, and a clock target whose file no longer imports time.
- **What the gate did:** `Stale` only checks whether the path is gone. Such an entry stays a live waiver: a sleep added back later is green, with no LAW Δ line.
- **Red test:** `TestBreakitUnusedTestHygieneTarget`, commit a24bd35. Quoted red: `-tests: exit 0, output "GATE_COVERED 1\n": an entry that waives nothing is not reported`.
- **Proposed fix:** each mode records which entries it used and reports `<list>: <target>: waives nothing (an unused target)`.

**BK-S4: gosec's `#nosec` is a waiver the grammar does not know.**
- **Measured with the pinned golangci-lint** (`--enable=gosec`): `"crypto/md5" // #nosec G501` and `md5.Sum(...) // #nosec G401` remove both findings, and they come back when the comments go.
- **What the gate did:** `go_directives` returns `([], [])`, and no PREFILTER pattern matches. No linter is enabled today; the day S1.1c-4 enables gosec, every `#nosec` already in the tree becomes a live waiver with no metadata.
- **Red test:** `NosecIsAWaiver`, commit fc944ef. Quoted red: `AssertionError: ([], []) == ([], []) : a #nosec passes unseen`.
- **Proposed fix:** S1.1c-4 adds linter-native waivers to §0.3's table and makes them red. `#nosec` is the first; `//revive:disable` and `//lint:ignore` should be measured too.

**BK-S5: a known-issue id may be listed twice.**
- **What the gate did:** `knownissues.check` keys entries by id, so a second entry with a later `first_seen` (and a matching Found cell) passes the pin. LAW Δ renders it as `S3.3-2-1: first seen S3.3 → S1.1c-2` on a file it counts as `(1 entries)` while the file holds two. S1.1c-3's aging reads `first_seen`.
- **Red test:** `KnownIssueListedTwice`, commit 99c166c. Quoted red: `AssertionError: '(1 entries)' != '(2 entries)'` and `[] is not true : an id listed twice passes`.
- **Proposed fix:** `_entries` refuses a repeated id, as `allowlists.load` already does for targets, and the known-issues kind counts raw entries.

**BK-S6: the generated-code header is a forgeable in-code exemption.**
- **What it waives:** a first line `// Code generated … DO NOT EDIT.` takes a file out of the coverage floor's numerator and denominator (`gate/coverage/main.py:114`) and out of archcheck's globals check.
- **Why it can be forged:** its anti-forgery row, `freshness`, is annotated moot. The `suppressions` row neither reads nor counts the header. archcheck's `generated()` accepts the header in any comment anywhere in the file; ADR-0015 §2.1 says the first line.
- **Origin:** this predates S1.1c-2; whether to forward it is the owner's call.
- **Red tests:** `GeneratedHeaderIsAnExemption` and `TestBreakitGeneratedHeaderAnywhere`, commit 67b6158. Quoted red: `a header after the package clause marks the file generated (ADR-0015 §2.1: the first line)`.
- **Proposed fix:**
  - A list of generated files (an allowlist entry, or a manifest the suppressions row checks).
  - archcheck reads only the comment before the package clause, as `go/ast.IsGenerated` does.

**BK-S7: the pin does not tie a proof to its instrument's row.**
- **Plant:** using the pin's own fixture, the docsync instrument borrows the lawdelta row's driver (with a matching count).
- **What the gate did:** the pin is green. Also, "a pass on the row under test" accepts `assertRow(…, ok=True)` on any row id except "parity". LAW Δ names the law.json edit; the pin does not catch it.
- **Red test:** `PinTiesAProofToItsRow`, commit cfa10bf. Quoted red: `AssertionError: [] is not true : the docsync instrument passes on the lawdelta row's driver`.
- **Proposed fix:** `python_cases` takes the instrument's row and requires a red or pass verdict on that literal row id (or the module's `ROW` constant).

## Later

**BK-L1: docsync reads a §3 data-file line as listing its directory.**
- `tools/deps-allowlist.json` lists `tools`, `api/exports.golden` lists `api`, and `tools/importcheck/rules.json` lists `tools/importcheck` (§11.7's own example). A new Go package in any of these passes.
- **Red test:** `DocsyncDataFileLines`, commit e256dc9. Quoted red: `Lists differ: [] != ['api', 'tools', 'tools/importcheck']`.
- **Proposed fix:** only a `.go` file names its directory.

**BK-L2: a ref resolves to a heading inside an HTML comment.**
- The rendered page has no such anchor, so a waiver can cite a section the reader cannot find. This is anchors code moved verbatim from doclinks.
- **Red test:** `RefToACommentedHeading`, commit 7e9fbc9. Quoted red: `'' == '' : a commented heading resolves`.
- **Proposed fix:** `anchors.prose` skips `<!-- … -->` blocks as well as fences.

## Plants the gate caught

- **Go directives:** `//nolint:errcheck,`, `// nolint`, `///nolint`, `//nolint errcheck` and a CRLF directive were all red. The forms golangci itself ignores were left alone, correctly: `/* nolint */`, `//\tnolint`, `// x nolint`, a directive inside a string or raw string, a block comment that also contains `//`.
- **Python forms:** `#fmt:skip`, `FMT: SKIP`, `fmt: skip -- r`, `noqa`/`ruff: noqa` were handled per §0.2. `# ruff: disable[...]` was measured to waive nothing under `--ignore-noqa`, so it is harmless.
- **Go `-cases`:** `[:n]` with a non-constant `n` gives a cannot-count fault. `t.Fatal` → `t.Log` gives a no-verdict fault. A `return` before the Fatal is red via vet's unreachable check.
- **Python strength rule:**
  - `assertRed` → `assertParityPassed`, a three-level helper chain, and a red helper in an `elif False:` (whose `if` is also literal `False`) all gave faults.
  - Generator and async tests are red through pyshim under `-W error`.
  - `skipTest` and `t.Skip` are red.
  - `try/except AssertionError: pass` is red through ruff SIM105, but see BK-B3.
- **Allowlist fields:** blank or tab-only reasons, `２０２６-12-01`, `20261201`, `2026-W50-1`, an empty sprint, a sprint after the current one: all red. Exactly 90 days and expiry today: green.
- **TEMP expiry:** red at T4 and nightly; nightly passes `--tier nightly`.
- **Refs:** an untracked doc, a fenced heading, `#rules-2`, `#Rules`, `./docs/…`, a ref with no anchor: all red. `#rules-1` correctly resolves to the second "Rules" heading.
- **Known issues:** a TEMP `issue` whose known issue was removed is red. A known issue in the Markdown table only is red.
- **Stale targets:** a gone file or package is red in its own mode.
- **docsync:** a directory listed only in prose, a new `io/x` and a new `cmd/x` are red. A third go.mod is red via the deps row.
- **lawdelta:** a deleted pin test fails liveness ("git tracks nothing there"). No merge base is red. `git grep` exit 128 becomes a GitError and the row goes red. A moved directive is neither added nor removed. The countdown and the cap of 6 hold.
- **LAW Δ:** law.json instrument edits are named (`instrument suppressions: proofs …: 9 → …: 8`, `−instrument suppressions`).
- **pytests:** deleting a module is red at e7b5d89, because `min_wants` 82 equals the count. Note that any module added later gives the same slack for a deletion.
- **BA13:** the stdout forge is red (its driver). `pyshim`'s argument count is enforced.
- **Coverage:** `--floor` is required.

Not re-reported (already in the record): W1-F2, W1-F3, W1-F4.
