> **S1.1c-2 design review, 2026-11-12 — architecture, with the-fool.** Read blind against the design note's v1 (`s1-1c-2/integration` @ 0d6ac6c); every finding's disposition is in [the design's §9](../sprints/s1-1c-2-design.md#9-the-reviews-of-v1--every-finding-placed) and the owner's answers in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-design-after-the-reviews). Sibling: [QA (testability)](s1-1c-2-design-qa.md).

# S1.1c-2 design note v1: architecture review, with a the-fool pre-mortem

*Reviewer: architect (blind to QA). Subject: `docs/sprints/s1-1c-2-design.md` v1 at 0d6ac6c. Code read at 0d6ac6c (`git show 0d6ac6c:…` where the shared working tree had moved: QA's W0 committed dcfb3ee and 061a5c4 and had uncommitted edits during this review). Probes ran only under `arch-scratch/`, with the pinned ruff 0.17.3, golangci-lint v2.15.0, Go 1.27.3 and CPython 3.15.2. GitNexus: `context(CheckSources)` and `impact(_run_module, upstream)` agree with the code at 0d6ac6c. The index-freshness resource could not be read from this agent, so no `index.commit` is reported. Hindsight tools were not available to this agent, so no memory was used. Severity uses ground rule 15.*

## Verdict (5 lines)

1. The layering is sound and lean: pure modules, thin rows, one channel, and the law-model table. Lane C's archcheck work and the docsync design hold up (`go list -e ./...` lists all 20 packages even with an empty module cache and `GOPROXY=off`, measured).
2. Both waiver recognisers are written from expectation, not from the tools' behaviour. As specified, `py_markers` marks the decided form `# fmt: skip  # reason` as bad, because tokenize gives one COMMENT per line. `//nolint` detection misses four forms that golangci-lint honours, and `///nolint:x` gets past nolintlint too (DA-B1, DA-B2).
3. Decision 8's "red at the nightly" is not met: the suppressions row is tiered T1–T4, and the nightly runs only rows tiered `nightly` (DA-B3).
4. The instrument pin lands red on the real tree. About 8 of the 24 drivers fail the VERDICT rule (measured). docsync-b names the wrong test, in a general engine test file that decision 9 keeps out of the law and that `-cases` cannot count. docsync (c) and the keyword half of (b) have no instrument (DA-B4).
5. Totals: 4 blockers, 14 should-fix, 13 later. All are fixable in a v2 of the note, with no new scope. One finding (DA-S10) needs an owner question.

## Blockers

**DA-B1. `py_markers` cannot work as specified. Python's tokenize gives one COMMENT token per line, so the decided reason form `# fmt: skip  # <reason>` is "`# fmt: skip` followed by other text" (`fmt-skip-bad`, red), and "the next COMMENT on the same line" never exists.**
Evidence: design §2.3 ("exactly `# fmt: skip` → fmt-skip, its reason the next COMMENT on the same line"; "`# fmt: skip` followed by other text in the same comment → fmt-skip-bad") against §0.2. Under pinned 3.15.2, `tokenize.generate_tokens` on `c = [1,2,  3]  # fmt: skip  # reason here` gives one token, `'# fmt: skip  # reason here'`, for the line. A lane built to spec would turn all 40 reasoned markers red, or improvise.
Disposition: rewrite §2.3's `py_markers` as follows. Split the single COMMENT token on `#` into segments, and strip each. A segment equal to `fmt: skip` or `fmt:skip` is a skip marker. Its reason is the other segments joined, and the 12-character rule applies to it. Mirror ruff's own rule, measured with pinned ruff 0.17.3 `format --isolated`: ruff keeps `# fmt: skip  # r`, `# fmt: skip # r`, `# r  # fmt: skip`, `# noqa # fmt: skip` and `# fmt:skip`, and it drops `# fmt: skip; r` and `# FMT: SKIP`. Add the probe's table to `test_suppressions.py` as fixed rows. (DA-S4 covers the other forms ruff honours.)

**DA-B2. The `//nolint` recogniser is narrower than golangci-lint's. `// nolint:x`, `//  nolint:x`, `///nolint:x` and `// nolint` all suppress under the pinned golangci-lint, yet none of them is a "directive" under §2.3. They go unvalidated and uncounted, in engine files that are not law.**
Evidence: scratch module, pinned golangci-lint v2.15.0, `run --no-config --default=none --enable=errcheck --max-same-issues=0 --max-issues-per-linter=0`. Suppressed: `//nolint:errcheck // …`, `// nolint:errcheck`, `//  nolint:errcheck`, `///nolint:errcheck`, `//nolint errcheck`, `// nolint`, `//nolint:all`. Not suppressed: `//NOLINT:…`, `/* nolint:… */`, `/*nolint:…*/`, `//<TAB>nolint:…`, `// x nolint:…`, `// text //nolint:…`. With `--enable=nolintlint`, `// nolint:x`, `//  nolint:x`, `// nolint` and `//nolint errcheck` are reported, but **`///nolint:errcheck` is suppressed and reported by nothing**. The behaviour matches golangci's TrimLeft of `/ ` followed by `^nolint( |:|$)`. Note that the probe's first run was capped by golangci's default `max-same-issues: 3`, so any table re-measured for this must disable both caps. Design §2.3: "a comment whose text starts `nolint` (case-sensitive, no space: `//nolint`)". Today such a form passes the row, although the policy (DEVELOPMENT.md:73) says a suppression for a linter that is not enabled fails.
Disposition: in §2.3, recognise a directive exactly as golangci does: the comment text after `//`, with leading `/` and spaces stripped, matching `^nolint( |:|$)`. Then any form other than the canonical `//nolint:<l>[,<l>]` is red: `write //nolint:<linter> (no space, two slashes)`. Pin the probe's table in `test_suppressions.py`. The lawdelta prefilter (`git grep -e nolint`) already matches every form. Record the table as S1.1c-4's differential: once a golangci row exists, run the binary over it.

**DA-B3. Decision 8 is not met: no nightly run evaluates an expired dated exemption. §0.1 proves that a row can read its tier, not that the row runs at that tier.**
Evidence: `gate/registry.json`'s `suppressions` row is tiered `["T1","T2","T3","T4"]`. The rows tiered `nightly` are only `fuzz-parser`, `fuzz-decoder` and `race-stress`. `.github/workflows/nightly.yml:34` runs `--tier nightly --job nightly`, and `gate/ci/generate.py:render` builds jobs only from rows selected at that tier. Decision 8 (decisions.md, 2026-11-12) says "red at T4 and the nightly once past". On a quiet `main` an expired TEMP stays green until the next PR's T4, and the red then lands on an unrelated lane.
Disposition: §2.4 and §6 add `nightly` to the `suppressions` row's tiers (a coordinator registry edit, with the workflows regenerated so that `nightly.yml` gains a `lint` job, and pinned by the row's instrument). Add a driver case at `--tier nightly` (expired → red; the same world at T2 → green with the countdown). Correct §0.1 to "the row is scheduled at T4 and nightly".

**DA-B4. As specified, the instrument pin is red on the real tree on landing. The design neither measured its rule nor listed the as-built Go instruments, although plan check P5 promised "the instrument list as built with its measured counts".**
Evidence:
(a) VERDICT rule. Running §4.2's rule (a VERDICT helper on `self`, directly or one helper deep; one RED helper per module) over the 24 drivers at 0d6ac6c gives these faults: `test_hook_commit.py` (7 cases, plants nothing red; it asserts through its own `assertRefusedUnformatted`), `test_tripwire_planted.py` (3; `assertEqual(record["verdict"], "fail")`), `test_regen_baseline.py` (4), `test_leak_planted.py` (1), `test_notbuilt_expired.py` (2), `test_parity_hash.py` (1), `test_lawdelta_planted.py` (1) and `test_differential_swap.py` (plants nothing red). The scan script is at `arch-scratch/py/verdict_scan.py`.
(b) docsync-b. §4.2 names `internal/validate.TestEveryStaticCodeIsAccountedFor`, but the as-built docsync (b) is `TestDocsyncStaticCatalog` (`internal/validate/static_test.go:257` says so) plus `internal/parser/keywords_test.go:14`. docsync (c) is `internal/expr/docsync_test.go:15`, which has no instrument. The named proof file `static_test.go` is a general engine test file with 7 `Test*` functions, so decision 9 keeps it out of the law. Its `TestStatic` and `TestCorpusClean` call `t.Run` over `programs(t)` and `glob(…)` (lines 75 and 119–124), so `-cases` emits `fault … cannot count`.
(c) Test 4 needs every proof path under the law, and §4.2 never says that the dedicated Go files (`internal/runtime/nosleep_test.go`, `internal/parser/errorcounts_test.go`, …) get exact `law.json` entries.
Disposition: before G1 the coordinator runs the rule over every driver and Go proof file and puts the measured table in §4.2. Then choose one of two options, and the choice is the design's: (1) QA's W0 converts the eight drivers to the support helpers, with a `support.assertRefused` added for the hook proofs and to VERDICT; or (2) VERDICT also accepts `self.assertEqual(<record>["verdict"], "fail", …)`. Fix the Go instrument list: (b) as keywords plus static-catalog, (c), (d) and nosleep. Move `TestDocsyncStaticCatalog` and `TestDocsyncSeesPlantedDrift` into a dedicated `internal/validate/docsync_test.go` (a test-only move, owned by lane D, named in §4.2), or scope `Proof.cases` to the named test functions. List the exact `law.json` entries that the dedicated Go proof files need.

## Should-fix

**DA-S1. "One validator for the grammar" is not one function. `allowlists.check` works on a whole list, `suppressions.check_go` re-applies "the same field rules", and `deps.compare` re-checks `sprint`/`ref` (decision 17: "checked by the same validators"). That makes three copies of the part rule, the 90-day rule and the TEMP rule.**
Evidence: design §2.2 (`check(lst: AllowList, …)`), §2.3 ("then the same field rules as §2.2"), §4.6a.
Disposition: §2.2 exports `fields(meta: Mapping[str, str], *, today, tier, resolve, order, current, issues, where: str) -> list[str]` (kind, reason, ref, expiry, issue, sprint), plus `sprint_fault` and `ref_fault`. `check`, `check_go` and `deps.compare` call these, and only these. QA's grammar table runs once against `fields`.

**DA-S2. In-code TEMP directives get no countdown. `expiring(lst: AllowList…)` covers lists only, so at T0–T3 an expired `//nolint … TEMP` is neither red nor shown, and the first sign is T4's red (decision 8: "shows a countdown at earlier tiers").**
Evidence: design §2.2 (the `expiring` signature) and §2.6 ("one line per entry from `allowlists.expiring(...)`").
Disposition: `expiring` takes `Iterable[tuple[str, Mapping]]`, holding the list entries and the directives (`path:line`). lawdelta already reads those files for `count` and prints both.

**DA-S3. The new git helpers have no error contract. `git grep` exits 1 for no match and 128 for an error, while `_raw` ignores exit codes, so a bad or missing revision reads as "no markers" and the exemption line prints a wrong number with no failure. The prefilter's patterns are also a second copy of the scanner's forms.**
Evidence: `gate/runner/git.py:10-13` (`_raw` drops the code) against §2.6's `grep_files` and `ls_tree`. `git grep -l … <rev>` prints `<rev>:<path>`, which needs stripping (measured: 24 files, 0.04 s, so cost is not the issue).
Disposition: §2.6 says exit 0 is a list, exit 1 is `[]`, and any other code is a lawdelta FAIL that names the command. The same holds for `ls_tree`. The patterns come from one `suppressions.PREFILTER`, and a test asserts that every form in the scanner's table (DA-B1, DA-B2, DA-S4) matches it.

**DA-S4. Python and ruff waivers that the design does not recognise: `# fmt:skip`, `# yapf: disable` … `# yapf: enable`, `# isort: skip`, `# isort: skip_file`, `# ruff: isort: skip_file`, and a second waiver table, `[lint.extend-per-file-ignores]`. All of them waive under the gate's own flags.**
Evidence: pinned ruff 0.17.3. `format`: `# fmt:skip` and the `# yapf: disable` region keep their layout. `check --ignore-noqa --select I`: `# isort: skip_file`, `# isort: skip` and `# ruff: isort: skip_file` pass an unsorted block, while `# ruff: noqa` is ignored (correct). With `[lint.extend-per-file-ignores] "gate/w.py" = ["TID251"]`, `time.sleep` passes. Design §2.3 knows only noqa, fmt: skip and fmt: off/on, and §2.4's pin reads only `[lint.per-file-ignores]`. These files are law (`gate/` and `ruff.toml` are `code` kinds), so each change shows as a grouped line, hence should-fix.
Disposition: §2.3 classifies every form above. Each is red except the decided `# fmt: skip  # <reason>`; `# fmt:skip` is told to write `# fmt: skip  # <reason>`. §2.4's pin reads both per-file tables, and it reds a non-empty `lint.ignore` or `lint.extend-ignore`.

**DA-S5. The teeth guard everything but themselves. Deleting `gate/runner/tests/test_instruments_pin.py` (or `test_known_issues_pin.py`) leaves every row green, because `pytests` has `min_wants: 0` and the pin pins that 0. LAW Δ shows only `gate/: 1 file +0/−N` on a grouped line.**
Evidence: registry `pytests` (`"min_wants": 0`); `verdict.py:57-60` (covered must equal expected, and both drop together); §4.2 (no instrument covers the pin).
Disposition: list each pin test (`test_instruments_pin.py`, `test_law_pin.py`, `test_sprints_pin.py`, `test_known_issues_pin.py`) as an exact `law.json` path of kind `code`. lawdelta's liveness check then reds a deletion, and removing the entry becomes a named `law` line. Set `pytests` `min_wants` to the measured module count, as `proofs` goes 24 → 35.

**DA-S6. BA13's fix closes the accidental forge, not the one the known issue describes. A module can `open(sys.argv[3], "w")`, write the pass line and `os._exit(0)`, and in-process code can always reach the shim's handle through its frames.**
Evidence: design §4.4 (the report path is `argv[3]`; the handle is a local of `main`, which calls the loader). known-issues.md:10 calls BA13 "Adversarial only", and §4.4 removes it at close-out.
Disposition: keep the file report, which is the recorded forward and blocks the print-then-exit forge. §4.4 and ADR-0015 §3.8 should state the residue honestly: the defence against a deliberate forge is that every test module is a law path (the `gate/` catch-all). BA13 leaves known-issues with that sentence, so it is not claimed closed outright. Add a break-it case that writes `argv[3]`, expected to pass (documented residue), so nobody is surprised later.

**DA-S7. Lane C's list threading has no signature. "At start-up … build the same map" leaves `CheckSources(rules, path, files)`, the entry point the tests use, without the lists, and a relative `tools/allowlists/…` path does not resolve under `go test` (the working directory is the package directory).**
Evidence: GitNexus `context(CheckSources)`: callers `archcheck_test.go:check` and `main.go:run`. `main.go:144` is the signature, and `main.go:79-82` is the global map that §3.2 deletes.
Disposition: §3 gives `type Lists struct{ Purity map[string]map[string][]string; Clock map[string]bool; Tests map[string]bool }`, `LoadLists(root string) (*Lists, error)`, `CheckSources(rules *Rules, lists *Lists, path string, files map[string]string)`, `CheckTest(lists *Lists, path, src string) []string`, and `Cases(path, src string) (cases, faults []string)`. The tests build `Lists` explicitly.

**DA-S8. Lane C's landing reddens the hook proofs at T4 only. `support.copy_hooks` copies archcheck's T0 inputs but not `tools/allowlists/`, so archcheck exits 2 (`internal error: tools/allowlists/purity.json: …`, §3.1) in the `test_hook_commit` and `test_hooks_off` worlds.**
Evidence: `gate/proofs/drivers/support.py:364` at 0d6ac6c: `t0 = [*world.tracked("tools/archcheck"), "tools/importcheck/rules.json", "go.mod"]`. Proofs run only at T4, so lane C's T2 cannot see it.
Disposition: §3 step 2 names the `support.py` edit (adding `*world.tracked("tools/allowlists")`) and its owner (QA, on lane C's starting branch).

**DA-S9. The covered-count contract is ambiguous where the runner is exact. `verdict.decide` needs covered == expected, but §2.4 says `rows.covered(<files read>)` and §3.5 `GATE_COVERED <files read>`, while both rows also read `gate/sprints.json`, `gate/current-sprint`, `gate/known-issues.json`, the Markdown targets of each `ref`, and `testhygiene.json`.**
Evidence: `gate/runner/verdict.py:57-58`.
Disposition: state that covered = the number of `{files}` entries processed, each read even when skipped (exempt files and testdata included), and that the side inputs are never counted. Add a unit case for each row.

**DA-S10 (question for the owner). In-code directives are counted and never named. S1.1c-4's bulk PERMANENT `//nolint`s, in engine files that are not law, reach the owner as `(+40 this diff)` only, while plan-check risk 6's mitigation ("per-entry one-liners") covers lists only.**
Evidence: design §2.6 and plan check §8 risk 6. Decision 2 names "each changed entry one line by name" for the allowlists.
Recommendation: yes. When `d ≠ 0`, lawdelta names the added and removed directives under the exemption line (`+internal/x/y.go:12 //nolint:gosec PERMANENT S1.1c-4 ref=…#…`), capped as `law.names` caps, with `(and k more)`. Pro: the owner sees which decision each waiver cites. Con: a few more lines on the diffs that change exemptions only.

**DA-S11. The `allowlist` one-liner joins every changed entry into one `; `-separated line. Decision 2 says "each changed entry one line by name", and in S1.1c-4's bulk that becomes one unreadable line.**
Evidence: design §2.5 against decisions.md 2026-11-12, decision 2. `law.delta` returns one string, and `law.line` builds one line per file (`law.py:140-141`).
Disposition: let a kind return sub-lines (`delta` → `str | list[str]`). `block` renders the file line and then one indented line per entry, capped at 6 with `(and k more)`.

**DA-S12. Go proofs can be hollowed with the count unchanged. Python cases must reach a VERDICT helper, but `-cases` counts names only, so the two shapes give different guarantees.**
Evidence: design §4.2 (`python_cases` AST rule) against §4.3 (names only).
Disposition: `-cases` emits `fault <case>: asserts nothing` for a counted test or subtest whose body calls no `Fatal`, `Fatalf`, `Error`, `Errorf` or `FailNow` on its `*testing.T`, directly or through one same-file helper that takes it. The parameter name is read from the signature, not assumed to be `t`.

**DA-S13. Adding `instruments` to `law.TOP_KEYS` makes it required, which breaks the order of steps. Between lane D's landing and the coordinator writing the key, `load_manifest` fails, so `lawdelta` is red at T1 on the lane's branch. `_delta_law` must also read a base that has no `instruments`.**
Evidence: `law.py:97-103` (every TOP_KEY is required); design §4.2 and §6 (the coordinator writes `instruments` after W2).
Disposition: §4.2 states that lane D's branch carries `instruments: []`, a coordinator-approved data edit named in the brief (test 1 is then red by name until the real list lands), and that `_delta_law` reads `old.get("instruments", [])`.

**DA-S14. The policy-examples test carries two time bombs: `expiry=2027-01-31` turns "expired" after that date, and `issue=S3.3-2-1` stops resolving once that issue is fixed.**
Evidence: design §2.7 and QA-S11's test ("checks each parses as a valid directive").
Disposition: the test injects a fixed `today` (the note's date), tier `T2`, and `issues` containing the example's id, and asserts only the grammar and the ref resolution against the real files.

## Later

- **DA-L1.** `.githooks/pre-commit` is Python that ruff formats (ruff's expected_scope), but the suppressions row's Python scope is only `gate/**/*.py`. Add it to the row's expected_scope.
- **DA-L2.** Stale targets are checked for purity, clock and testhygiene only. Also check `ruff.json` files on disk and `teeth.json` targets that name an instrument id.
- **DA-L3.** `-cases` takes one file per `go run`, and test 3 runs it over every test file of a package (about 30 runs). Accept a NUL list in one run.
- **DA-L4.** The S1.1c-4 seam ("reads its .golangci.yml here") needs YAML, which the stdlib cannot parse, so a new dependency would be an owner question. Golangci v2 also reads `.golangci.toml` and `.json`, so recommend that format, with `linters.default: none` plus an explicit `enable` list so that `ENABLED` can be computed.
- **DA-L5.** `resolves` says "a tracked Markdown file", while §2.4 says "no git". Locally, an untracked doc satisfies a ref. Either word it as "a Markdown file in the tree" or read through git.
- **DA-L6.** `deps.compare`'s new keyword parameters (`resolve`, `order`, `current`) are not written out, although the note promises full signatures.
- **DA-L7.** Plan check P2/A-L11 ("read from the same tree archcheck checks, the index snapshot at T0") was dropped without a note. State that archcheck reads the lists from disk, and that the pre-commit guard covers staged files only.
- **DA-L8.** §4.6's marker-only hunk test should use one marker constant that S1.1c-3's marker grammar also pins, so that a later change to the marker form cannot silently stop marker hunks being counted apart.
- **DA-L9.** `teeth.py` is labelled pure but imports drivers and runs `go`. Keep it pure (counting over source text and over `-cases` output) and put the import and subprocess work in the test, so `gate/runner` never imports `gate/proofs`.
- **DA-L10.** `run_modules` has a third caller, `pyfloor` (GitNexus `impact(_run_module)`: proofs/row.py, rows/pytests.py, rows/pyfloor.py). §4.4 says "both". Name all three; the 3.12 floor still holds.
- **DA-L11.** `law.py` has its own `HEADING`, which is not fence-aware, beside the moved one in `anchors.HEADING`. The three-headings work in §4.6 should use `anchors.prose` and `HEADING`, so a `# comment` inside a fenced block in SPEC is never taken for a heading.
- **DA-L12.** Plan risk 6's note ("S1.1c-4's plan check is told to cap PERMANENT entries per package or ask per group") is not carried into the record or the backlog.
- **DA-L13.** The 90-day bound moved from "after the entry's commit" (plan §2 S1) to "after today" (§2.2) without a note. Decision 8 allows either; say which one and why.

## Pre-mortem (the-fool, "find the failure modes")

*Steelmanned thesis: v1 gives every exemption one grammar and one validator, makes every row's strength a pinned and counted fact, and keeps the owner's reading to one LAW Δ block. All of it is pure-module-first and costs nothing at T1.*

Imagine S1.1c-2 shipped, S1.1c-4 and S4.1 followed, and the instruments failed the owner. These are the most likely stories:

1. **The waiver nobody saw (most likely).** S1.1c-4's fix wave clears about 300 findings. Agents write `// nolint:gosec` (the gofmt habit of a space) and a few `///nolint:errcheck`. golangci honours them, the suppressions row sees no directive, nolintlint catches the spaced ones but not `///`, and LAW Δ reads `exemptions: 0 in code`.
   - *Root assumption:* our recogniser equals the tool's.
   - *Mitigated in v1?* No (DA-B2, DA-B1, DA-S4).
   - *Early warning:* any honoured waiver that the count misses.
   - *Fix:* recognise as each tool does, demand the canonical form, and pin a probe table.
2. **The pin landed red, so it was loosened.** Lane D finds eight drivers and two Go files faulting under the time box. The VERDICT rule is widened ad hoc, or `static_test.go` becomes law against decision 9, or `-cases` faults turn into warnings.
   - *Root assumption:* a rule written in the note holds on the tree.
   - *Mitigated?* No: P5's measured list was deferred to lane D (DA-B4).
   - *Fix:* measure in v2.
3. **Expired, and blamed on the wrong lane.** A TEMP waiver expires in a quiet week, and the owner checks scheduled runs by hand (decision 11). The nightly does not run the row, so the red appears on an agent's unrelated PR at T4, and an in-code TEMP never showed a countdown.
   - *Root assumption:* a tier label is a schedule.
   - *Mitigated?* Partly: there is a countdown for lists only (DA-B3, DA-S2).
4. **Teeth that count but do not bite (Go).** A break-it in S4.1 empties a Go proof's assertions and keeps its `t.Run` names. The count is unchanged, the pin is green, and the edit shows only as a grouped `tools/` or engine line.
   - *Root assumption:* count equals strength.
   - *Mitigated?* For Python yes; for Go no (DA-S12). The break-it's hollowed driver is Python-only as planned.
5. **Bulk exemptions arrive as one number.** S1.1c-4 adds 60 PERMANENT in-code waivers that cite `ADR-0015#…decision` generically. The owner reads `(+60 this diff)` and approves without seeing which decision each one cites.
   - *Root assumption:* naming one line per entry covered all exemptions.
   - *Mitigated?* No: the lists are named, the code is counted (DA-S10, DA-S11; owner question).

**Inversion: what in v1 would guarantee failure?**
- Writing recognisers from documentation instead of from the pinned binaries' behaviour, with no differential table (B1, B2, S4).
- Putting a strength rule in the note without measuring it on the real tree (B4).
- Letting the meta-guard live in a deletable test whose row has a floor of 0 (S5).
- Treating "the row can read GATE_TIER" as "the row runs at that tier" (B3).
- Calling a residual adversarial gap "fixed" (S6).

v1 avoids the other classic failures: there is one channel and no schema move, the layering is pure-first, archcheck stays standard-library-only, docsync is robust offline (measured), and every new law file has an owner and a kind. The fixes above are edits to the note, not new scope.

*Synthesis for v2:* add a "§0 proven" block of four measured tables:
- ruff's honoured comment forms;
- golangci's honoured `//nolint` forms, plus nolintlint's verdicts;
- the VERDICT rule over every driver;
- `-cases` over every Go proof file.

Then apply B3's tier edit and S5's exact law entries. Everything else is wording and signatures.
