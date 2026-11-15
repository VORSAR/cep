# S1.1c-5 plan check v1 — blind QA review (2026-11-14)

> **Historical — S1.1c-5, the strict lint set: the instrument, the whole-tree rules, the first packages; merged in PR #47.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-5-plan-check.md) · [design](../sprints/s1-1c-5-design.md) · [record](../sprints/s1-1c-5-record.md) · reviews: [research lint](s1-1c-5-research-lint.md), [plan check architecture](s1-1c-5-plan-check-architecture.md), [plan check qa](s1-1c-5-plan-check-qa.md), [design architecture](s1-1c-5-design-architecture.md), [design qa](s1-1c-5-design-qa.md), [combined architecture](s1-1c-5-combined-architecture.md), [breakit qa](s1-1c-5-breakit-qa.md). **← [S1.1c-4](../sprints/s1-1c-4-record.md) · [S1.1c-6 (next)](../sprints/sprint-plan.md#status) →**

Reviewed read-only against `main` @ d4b028f: plan-check-v1.md and research.md, the code it relies on, and the binding decisions. Experiments ran with the pinned golangci-lint v2.15.0, Go 1.27.4 and ruff 0.17.3 in `s1-1c-5-scratch/qa-exp/` (the `m/` module, plus `ruff/` and `rev/`). I did not consult GitNexus or Hindsight: this lane is Python gate code plus tool behaviour, so I read and ran it directly. memory: none used.

Severity follows ground rule 15. For a gate instrument, a silent pass is a missed detection.

## Blockers

**PQ-B1 — The config pin, as described, lets edits through that blind the row while every named setting stays put.**
- Claim (§2 S1): "A pin test reads the config and refuses any drift from the ADR's numbers and from the three pinned settings."
- Evidence: each edit below left the ADR numbers and the three named settings unchanged, and each still hid findings.
  - `issues.new-from-rev: HEAD` → exit 0, `0 issues` (qa-exp/m/cfg-new.yml).
  - `run.tests: false` → exit 0, `0 issues` on a package whose test file holds 3 forbidigo findings (cfg-notests.yml).
  - Dropping the two caps (defaults `max-same-issues: 3`, `max-issues-per-linter: 50`) → 3 of 4 `panic` findings reported. The new one, p.go:13, was hidden (cfg-caps.yml).
  - Untouched defaults hide findings too:
    - forbidigo `exclude-godoc-examples` defaults to true. `panic`, `reflect.TypeOf` and `os.Getenv` inside `func ExampleThing` gave 0 findings. With it set false they gave 3 (out2 against out3). The repo has `Example_embed` (example_test.go:16) and `ExampleX` (tools/archcheck/archcheck_test.go:1006).
    - errcheck's default exclusions hide unchecked `(*bytes.Buffer).WriteString` and `fmt.Fprintln(os.Stderr, …)`. With `disable-default-exclusions: true` both are reported.
  - Other keys that blind the row the same way (inferred from golangci's documented behaviour, not measured): `linters.exclusions.paths` or extra `rules`; a revive rule `exclude`; `run.build-tags`; `issues.new-from-patch`/`whole-files`; `run.issues-exit-code`; `output.formats.json.path` pointed at a file; `severity.rules` (if the row filters by severity).
- Failure: a one-line config edit turns the row green and blind. Only a LAW Δ line by file shows it, which is the same hole class as the ruff.toml forward this sprint exists to close.
- Fix:
  - Pin the whole document, not a key list. The pin test holds the canonical config as data and requires every key and every value to be equal, so any added, removed or changed key is red, naming it.
  - Set `exclude-godoc-examples: false` explicitly, and decide `errcheck.disable-default-exclusions` explicitly. That is an owner question: switching the defaults off adds findings, at least `fmt.Fprintln(os.Stderr, …)` in tools/shape/main.go:33,37, so the 417 must be re-measured.
  - Planted proofs, one each: caps removed, `new-from-rev` added, `run.tests: false`, `generated: lax`, `uniq-by-line: true`, `statements: -1`, an added `exclusions.paths`, and a finding inside an Example function.

**PQ-B2 — The forward expiry cannot be built from the "not-built expiry" as the plan says, and the obvious adaptations are wrong.**
- Claim (§1, S2): "the not-built expiry (`gate/current-sprint`) reds it if that sprint closes without it"; "The not-built expiry reads the forwards."
- Evidence:
  - `gate/runner/notbuilt.py:57-58` refuses any entry that names a part ("entries name a row, not a part"). Every planned forward (S1.1c-6, -7, -8) is a part.
  - `expired` (notbuilt.py:75) uses `Order.reached`, and `reached` works by row (sprintid.py:66-75; `index` maps S1.1c-6 to row S1.1c). So `reached("S1.1c-6", "S1.1c-5")` is True, and every forward would be red from the G0 commit.
  - The part-aware alternative, `Order.passed` (sprintid.py:77-86, used by aging.py:65), is strict. `passed("S1.1c-6", "S1.1c-6")` is False, so S1.1c-6 closes green with its package still uncleaned. It turns red only at S1.1c-7's marker commit, after the failing sprint has merged.
  - Parts are never checked against the table (sprintid.py:67 docstring, "a part is not validated by the table"). `forward: S1.1c-99` is therefore accepted and never expires until row S1.1c closes. That is an unbounded extension, visible only as a LAW Δ line.
  - notbuilt.json's grammar (file, construct, sprint) and the allowlist grammar (allowlists.py: TEMP expires by date, at most 90 days ahead, red only at T4/nightly) both differ from "`in` or `forward: S1.1c-<n>`". The scope list is "in the one grammar" by neither.
- Failure: either main is red from the G0 commit, or a package sprint closes green with its package outside the scope, and its forward can be pushed later without a check.
- Fix:
  - Specify a part-aware "reached": the marker is at or past the part, at the same row with n ≥ entry.n, or at a later row.
  - Forwards may name only printed rows. Print S1.1c-5…8 as rows of the plan table and gate/sprints.json, which the S1.1a-1/-2 precedent allows.
  - Moving a forward later, or demoting `in` to `forward`, needs `ref=` to a decisions.md heading, checked by `allowlists.ref_fault`.
  - Named T2 tests, all red first:
    - (S1.1c-6, marker S1.1c-5) green.
    - (S1.1c-6, S1.1c-6) red at T4.
    - (S1.1c-6, S1.1c-7) red.
    - Part-less marker S1.1c → a named fault.
    - `forward: S1.1c-99` → refused, not a row.
    - `in` → `forward` without a ref → red.
    - A forward moved later without a ref → red.
  - Plus a planted proof in `gate/proofs/drivers/`.

**PQ-B3 — The panic and reflect allowlists have no stated matching rule. The likely ones admit new violations.**
- Claim (Q3, Q6, §1): PERMANENT entries "for `encode` and `encodeStruct`", and "14 invariant panics … each naming its invariant".
- Evidence:
  - golangci findings carry file:line:column, not a function name (qa-exp JSON `Pos`).
  - The row is stdlib Python, which cannot parse Go.
  - A golangci `exclusions.rules` path/text rule can only match a file or line regex.
  - partition.go holds 7 panics and model.go 33 reflect uses. A file-level match admits an eighth panic or a thirty-fourth reflect use, and a line-level match breaks on every reflow.
  - It interacts with the caps (PQ-B1): with the default `max-same-issues: 3`, three allowlisted panics fill the cap and the fourth, new one is never emitted, so a row that post-filters the allowlist goes green (measured: p.go:13 hidden).
- Failure: a new `panic` or `reflect` use in an allowlisted file passes.
- Fix:
  - Allowlist targets are `<file> <func>` with an exact site count.
  - The row maps each finding to its enclosing function with a Go helper (stdlib go/ast, under tools/, a law path) and matches both ways, like `ruff_pin`: an unlisted site is red, and a listed function whose count differs is red (stale).
  - Tests:
    - A fourth panic added in a listed function → red.
    - A panic moved to an unlisted function → red.
    - An entry left after its panic is removed → red.
    - A reflect use added to model.go outside `encode`/`encodeStruct` → red.

**PQ-B4 — §6 has no case for 5 of the 8 linters or for either allowlist. A wrong row passes every planned test.**
- Evidence: §6 items 1–9 cover only funlen, gocyclo, lll, the file limit, params and the scope. Nothing covers:
  - forbidigo's five patterns;
  - the reflect L0–L2 carve-out, the Getenv `io/` carve-out or the package-main fmt carve-out (Q9);
  - errcheck, gocritic or nolintlint;
  - the panic and reflect allowlists;
  - `typecheck`.

  Measured bypasses the row as designed lets through (qa-exp/m):
  - `import _ "unsafe"` + `//go:linkname` gives no unsafe finding, because the blank import has no selector (internal/runtime/link.go).
  - `println(…)` passes the fmt.Print rule.
  - `os.LookupEnv` passes the Getenv rule.
  - Example functions (PQ-B1).
- Failure: a mistyped pattern such as `^reflect\.$`, or a carve-out regex that is too broad (`path-except` covering all of `internal/`), passes every §6 test.
- Fix: add red/green pairs:
  - each of the five patterns, red in scope and green out of scope (reflect in L3 and in a `_test.go` per Q4; Getenv under `io/`; fmt.Print in package main under tools/ and red in a non-main package);
  - errcheck on `rt.Close()`;
  - gocritic appendAssign;
  - nolintlint on an unused `//nolint`;
  - a typecheck error;
  - a `go:linkname` plant (or an owner question: does "unsafe" cover linkname?).

**PQ-B5 — The covered count cannot see what golangci linted, and some files are linted on one platform only.**
- Claim (§2 S1): "counts as covered the files linted against the scope list".
- Evidence:
  - golangci's JSON `Report` has only `Linters` (measured). No list of files or packages is emitted, so "files linted" has no source.
  - verdict.py:57 requires `covered == expected`, where expected is the live expansion of git pathspecs (scope.py). A pathspec cannot express "files of packages marked `in`".
  - Measured silent skips (qa-exp/m), each with a planted finding, none reported:
    - `//go:build ignore` (internal/runtime/ignored.go);
    - `internal/x/testdata/p`;
    - `internal/_hidden`;
    - a nested module `sub/go.mod`;
    - `only_linux.go`, skipped on darwin and reported only with `GOOS=linux` (out4). Local T2 (darwin) and CI (linux) disagree, against ground rule 13.
  - At G0 every package is forwarded (§1 wave 1), so the row gives either `GATE_COVERED 0`, which covered.py:27 makes red, or a vacuous count.
- Failure: a production `.go` file under a GOOS suffix, a build tag, `testdata/`, `_dir/` or a new nested module is never linted, and the row stays green.
- Fix:
  - covered = tracked `.go` files outside gate/proofs, each accounted for exactly once:
    - in `CompiledGoFiles`/`TestGoFiles`/`XTestGoFiles` of an in-scope package, from `go list -json` under the row's own env;
    - or in a forwarded package;
    - or in generated.json.
  - expected_scope = `:(glob)**/*.go` minus `gate/proofs/**`.
  - Any `IgnoredGoFiles`, a file in no package, or a `go.mod` other than root and gate/proofs → red, named.
  - Either refuse build constraints and GOOS/GOARCH filename suffixes outright (the build-tag law of ADR-0015 §2.2 gets its first subject), or lint under both GOOS values.
  - State what the row reports at G0, before any package is in.
  - Planted proofs: a `_linux.go` finding, a `//go:build ignore` file, a nested go.mod, and a package under testdata.

## Should-fix

**PQ-S1 — `.golangci.yml` cannot be read by stdlib Python.** Lane A's pin and lane B's `ENABLED` both "read the config". The gate has no YAML parser (`grep yaml gate/` finds none), and a new dependency needs the owner. A hand-written YAML subset would differ from viper/yaml.v3: anchors and merge keys, flow style, duplicate keys (yaml.v3 refuses those, exit 3). Fix: commit the config as `.golangci.json`; `golangci-lint config verify --config cfg.json` gave exit 0 (measured). Read it with `allowlists._pairs` so a duplicate key is refused.

**PQ-S2 — The row's reading of JSON is underspecified.**
- Measured:
  - stdout holds the JSON object followed by `\n1 issues:\n* typecheck: 1\n` (show-stats), so `json.loads(stdout)` fails.
  - A config error gives exit 3 with empty stdout.
  - `run.issues-exit-code` can make findings exit 0.
- Fix:
  - `output.show-stats: false` (measured: clean JSON), and the JSON written to a file in the row's dir.
  - Exit 0 ⇔ zero issues and exit 1 ⇔ at least one; anything else is a fault.
  - `Report.Linters` with `Enabled: true` must be exactly the 8 linters plus `typecheck`.
  - Never filter by severity or by `FromLinter`.
- Tests: exit 0 with an issue → red; exit 1 with none → red; exit 3 → red; a missing linter in `Report.Linters` → red.

**PQ-S3 — A compile error in one package hides every other package's findings.** Measured: a malformed `//go:linkname` in internal/runtime made `typecheck` the only issue in the run. The errcheck finding in an unrelated internal/table vanished (out1 against out2). If the row lints `./...` and then filters to in-scope packages, a type error in a forwarded package is dropped as out of scope, and every in-scope finding is hidden: green. Fix: a `typecheck` issue anywhere is red, whatever the scope; or lint only in-scope package patterns (exact, no `/...`). Test: plant a type error in a forwarded package next to an in-scope finding → red.

**PQ-S4 — `generated: strict` contradicts "never by golangci's heuristics" (§2 S1).** Under strict, a hand-written file given a canonical `// Code generated x. DO NOT EDIT.` header is skipped by the row. Only the suppressions row (BK-S6) reds it, and only while it is unlisted. Fix: `generated: disable`, with exclusion paths taken from generated.json only. Rewrite counter-example 7: a canonical-header file not in generated.json is linted by `smells` and red on its finding.

**PQ-S5 — The waiver and boundary tests are refusals only.**
- Items 10–12 are all "refused", and item 12 (`//nolint:gosec`) is already green today because `ENABLED` is empty (suppressions.py:21). An implementation that never sets `ENABLED` passes all three.
- Fix:
  - Add the positive case, red today: `//nolint:lll // PERMANENT reason="…" ref=… sprint=…` accepted.
  - Refuse `//nolint:all` and `//nolint:typecheck`.
  - A pin that `ENABLED` equals the config's enable list, both ways.
- Every boundary needs its green twin: 60-line function, line 120, 500-line file and five params, each green.
- Pin funlen `ignore-comments` (default true, so a 61-line function with 2 comment lines is green) and lll `tab-width`.

**PQ-S6 — The waiver census is incomplete.**
- Measured (qa-exp/rev): revive honours `//<TAB>revive:disable-next-line:…`, and `//revive:disable-line:argument-limit some reason` trailing the code. It ignores `/* revive:… */`. revive's own regex is `^//\s*revive:`.
- lll skips every line starting `//go:` (a 130-column `//go:generate` gave no finding) and whole import blocks.
- Not measured: gocritic, errcheck, funlen, gocyclo, nolintlint directive forms.
- Fix:
  - W0 runs a directive census per enabled linter, held as data the way test_suppressions.py holds the //nolint tables.
  - The revive parser matches `//` plus any whitespace, not `lstrip("/ ")`. Tests: the tab form, the trailing form, a block-enable pair, a file-wide disable.
  - Refuse the `#nosec` forms in all nine measured spellings, including the file-wide and glued forms (research D(i)).

**PQ-S7 — The ruff pin is still a deny-list with holes.**
- Measured (qa-exp/ruff), on the repo's own ruff.toml plus three keys, with `select` unchanged and no exclude key: `builtins = ["undefined_name_zz"]`, `lint.dummy-variable-rgx = ".*"` and `[lint.pycodestyle] max-line-length = 400` → "All checks passed!", exit 0. The F821, F841 and E501 findings were hidden.
- Other keys of the same kind (inferred, not measured): `line-length`, `per-file-target-version` (fewer UP findings), and plugin settings such as `flake8-bugbear.extend-immutable-calls`.
- Fix: pin ruff.toml as a whole canonical document, with per-file-ignores still checked against ruff.json both ways. A planted key outside every named list is red.
- Re-measure under whichever ruff is pinned at G0: #45 may move to 0.17.5, which is already in the cache.

**PQ-S8 — An unlisted package is not red, and counter-example 9 asserts that it is skipped.** "A package outside the scope list is not linted" passes against a row that silently ignores a new package (for example `internal/newpkg`) that is neither `in` nor forwarded. Fix: every package from `go list` (and every file per PQ-B5) is `in` or forwarded; an unlisted one is red, named. Pin the `in` set's floor in a test so a demotion is a red test, not only a LAW Δ.

**PQ-S9 — Tool-scan edge cases.**
- If "Go-built" is decided by probing `go version -m`, a declared Go tool whose read fails is skipped as if it were ruff, which contradicts counter-example 17.
- A GOPATH-built or `command-line-arguments` binary has no `mod` line and zero deps, so it passes vacuously (inferred, not measured).
- `go1.27.0 X:…` and `devel go1.28-…` version strings are unparsed (inferred, not measured).
- Counter-example 16 ("toolchain never applies to a binary") passes an implementation that also drops `toolchain` from the engine's own scan, and the Go distribution's own `go` binary is a Go-built binary.
- An exemption target `<id> stdlib` (vulns.json grammar, confirm (d)) also silences the engine's own stdlib for that id if its OSV range is later widened.
- Fix:
  - Declare the Go-built tools in tools.json.
  - Read failure, a missing `mod` line, a `mod` version other than the pin, zero deps, or an unparseable Go version → red, named.
  - Twin test for item 16: the engine scan still judges toolchain at the pinned Go.
  - Scope the exemption target to the binary (a grammar change: owner).
  - Read the linux-amd64 binary's build info in W0; the darwin build list is the only one measured.

**PQ-S10 — The refactor nets do not prove behaviour for every lane-D package.**
- tools/shape has no Go tests at all (`git ls-files tools/shape` = main.go only). Its one net is the T4 `differential` row over 63 corpus files, but §1 ends a lane at T2. Refactoring `shape` (cyclomatic 19) is unexercised at the lane's exit, and a change in an S-expression for a construct outside the corpus passes even at T4.
- The coverage floor is not a behaviour net: splitting a function can raise coverage while it changes uncovered branches.
- The appendAssign "fix" in playback/recorder.go (the `append(r.buf[:0], …)` reuse idiom) can change aliasing with byte-identical outputs in single-record tests (inferred from the idiom, not measured). That is a later sprint, but the plan should say it now.
- Fix:
  - Before each refactor, a characterisation test, or 100% statement coverage of the function to be split (`go tool cover -func`).
  - Pure-move and reflow commits proven by an AST-equality check (decls compared with positions ignored): a test that can fail. It makes "each refactor commit is pure" (§8.1) checkable.
  - A lane exits at the T3/T4 rows that cover its package: differential for shape, proofs for apipartition, race/racestress for partition.
  - Q6's `row.go:73` change is a behaviour change: its own commit, a red test first (rule 9), and an apidiff entry. It does not belong in a "behaviour-preserving" lane.

**PQ-S11 — Q1 is presented as keeping decision 1, but the forward list is recorded debt.** Decision 1 of 2026-11-10 rejects "a ratchet of today's maxima (debt recorded, not paid)". Packages forwarded across three sprints, unlinted meanwhile, are debt recorded by package. Fix: ask Q1 as an annotation of decision 1 (ground rule 4), not as "the plan does not reopen it".

**PQ-S12 — All of tools/ is a law path (gate/law.json:141), not only archcheck.** Lane D's `apipartition` and `shape` are LAW Δ lines this sprint. §0.12 and §8.2 name only archcheck. Fix: say so, and plan the owner's read.

## Later

- **PQ-L1:** `println`/`print`, `os.LookupEnv`/`os.Environ` and `log.Print*` are outside the ADR's wording. Ask the owner whether the rules mean the smell or the identifier.
- **PQ-L2:** T0 staged mode. A test, like the proof-world test 19, that no workflow or Makefile target passes `--new*`, `--whole-files` or a package subset at T2 or later.
- **PQ-L3:** Q8 says 30 days, "renewed only by an owner decision". Nothing enforces renewal beyond the LAW Δ line and aging, and an expired exemption is red only at T4/nightly (allowlists.py `RED_EXPIRED`). Say so in Q8.
- **PQ-L4:** A test that the row's env pins GOOS/GOARCH next to env.py's `PINNED`, if PQ-B5 keeps build constraints.
- **PQ-L5:** §7 "min_wants 38 → 39 or more". Count the new drivers' cases exactly at the design (decision 5 of 2026-11-10: weakening a fixture is red).

## Q1–Q10

- **Q1:** agree with the instrument first. Change: the forward mechanism per PQ-B2/S8, presented as an annotation of decision 1 (PQ-S11).
- **Q2:** agree (runtime and validate next, archcheck last); note that lane D already makes tools/ law edits (PQ-S12).
- **Q3:** agree with PERMANENT for `encode`/`encodeStruct`, only with function-precise, exact-count matching (PQ-B3).
- **Q4:** agree that tests are exempt from the reflect rule only. Keep panic and Getenv for tests, and fix the Example-function hole (PQ-B1).
- **Q5:** agree with 120 for tests, each reflow proven by the AST-equality check (PQ-S10).
- **Q6:** agree; `row.go:73` gets its own commit with a red test and an apidiff entry; the 14 entries are function-precise (PQ-B3).
- **Q7:** agree to refuse both; add the per-linter directive census and the revive tab and trailing forms (PQ-S6).
- **Q8:** agree to the dated exemptions. Change: scope them to the binary, declare the Go-built tools, and say renewal is LAW Δ only (PQ-S9, L3).
- **Q9:** agree, with a test that fmt.Print in a non-main tools/ package is red.
- **Q10:** agree to refuse outright, but pin the whole ruff.toml document, not a longer key list (PQ-S7).

## Verdict

Not ready for design. Five blockers: config drift and defaults hide findings (B1), the forward expiry cannot work as written (B2), the allowlists have no matching rule (B3), most rules have no counter-example (B4), and the covered count cannot see what was linted (B5).
Each blocker has a cheap fix: whole-document pins (golangci and ruff), a part-aware expiry over printed rows, function-precise allowlists, red/green pairs for every rule, and a covered count built from `go list` that accounts for every tracked .go file.
QA's W0 can write all of these red before code, except item 12 (green today; replace it with the positive `//nolint:lll` case).
