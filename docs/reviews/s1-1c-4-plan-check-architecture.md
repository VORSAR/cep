> **S1.1c-4 plan check, architecture review with the-fool (blind), 2026-11-13** — the architect reviewer on [the plan check](../sprints/s1-1c-4-plan-check.md) v1 @ a488bc9 (code `main` @ 7c65dbf). Every finding is placed in the plan check's §10.

# S1.1c-4 plan check (draft v1, a488bc9): architecture review, with the-fool

Reviewed blind and read-only. Code citations are on 7c65dbf (the branch adds only the plan document: `detect_changes` compare main shows 13 sections of `docs/sprints/s1-1c-4-plan-check.md` and no code symbols). GitNexus: `context gather`, `impact _setup_steps` (upstream: `_job`, `_strict_gate` -> `render` -> `targets` -> `ci_fresh.main`), `context _Extraction` (imported by `gate/bootstrap.py` and its test only), `impact tree_sha256`. Hindsight: not consulted, because its tools were not available in this session. Scratch: `/private/tmp/claude-501/-Users-ramith-code-cep/e3f0ee66-e9c2-42af-8977-5e85564c01ab/scratchpad/s1-1c-4-pc/arch/` (database copy `db/vulndb.zip`, sha256 3b16a8d3..., `modified` 2026-11-11T02:51:11Z; scripts `scripts/q.py`, `scripts/all.py`).

**What I re-measured and found true:**
- §0.2: offline `go list -m all` gives 23 non-main modules across the two modules.
- `go mod graph` in both modules has exactly `golang.org/x/tools@v0.52.1-0.20261106145633-d407941a339a github.com/yuin/goldmark@v1.4.13` and `... golang.org/x/net@v0.61.0`.
- `go list -e -deps -test ./...` gives participle, x/text and goleak (root), and goleak (proofs). `go list -deps tool` and the apidiff listing give x/exp, x/mod, x/sync and x/tools.
- My own range matcher over every build-list module finds only GO-2026-5320 (goldmark), GO-2026-6860 and GO-2026-6866 (x/net). stdlib and toolchain at 1.27.4 have none.
- §0.6: `go mod tidy -diff` exits 1 in both modules, with the stated diffs.
- §0.13: ci-fresh is at `gate/law.json:425-450`, `min_wants` 2. `NEEDS_MODULES` is pinned at `test_generate.py:72`.

## Blocker

**A-B1. The shared "imported set" fails open, and under Q1 it becomes the scan's red/green boundary.**
- **Evidence (code).** `rows/deps.py:75-76` lists with `go list -e -deps -test`. The row's own docstring (`rows/deps.py:8-9`) says: "a module reached only through [a package that does not build] falls from imported to graph-only, weaker, never red". That is acceptable for licence fingerprints. For a security scan under Q1 it is a missed detection: a dropped compiled module moves from "red if vulnerable" to "graph-only, counted, never red".
- **Evidence (measured).** I ran this in a scratch module requiring x/text v0.43.0, with the zip missing from a scratch GOMODCACHE, under GOPROXY=off: `go list -e -deps -test -f '{{.Module.Path}} {{.ImportPath}} err={{.Error}}' ./...` printed ` golang.org/x/text/language err=p/p.go:3:8: module lookup disabled by GOPROXY=off` (empty module path) and **exited 0**. x/text silently left the set.
- **Where it bites.** The realistic trigger is a local strict gate whose module cache holds a module's `.mod` but not its zip. `go list -m all` still succeeds there, so the module counts as graph-only. Nothing today sets a floor on the set's size: the plan never states the `vulns` row's `min_wants`.
- **Fix to the plan.**
  - The lifted function returns the set **and** the per-package load errors (`-json`, reading `Error` and `DepsErrors`, or `-f` printing them).
  - The `vulns` row is red, by name, on any load error or empty module path ("cannot compute the compiled set: <pkg>: <err>; run go mod download").
  - Set `vulns`' `min_wants` to the measured count (9 under Q1: 7 modules plus 2).
  - Add counter-example (1c): a compiled module's zip absent from the cache -> red, never "graph-only".
  - Better still, adopt A-S1's fail-closed option. Then only modules the register approves as never-built are exempt, so a load error cannot widen the exemption.

## Should-fix

**A-S1. Q1 is argued on the wrong ground and misses the strongest option.**
- **Evidence.**
  - The dependency register already holds an owner-approved, per-module classification. `tools/deps-allowlist.json`'s `scope` field reads "module graph only, never built (apidiff's transitive graph; ...)" for x/net, goldmark, go-cmp, x/sys, x/telemetry, expect, packagestest, kr/pretty and check.v1, each with `approved`, `sprint` and `ref`. The plan cites none of it.
  - Q1's line "it is exactly what Google's scanner reports" overclaims. R-V2 §2a shows `-scan module` loads only the current directory's package without tests, and `-scan package -test` misses goleak in gate/proofs. The compiled set is what govulncheck *would* load if pointed at every package, test and tool. That is a different claim.
  - Under the recommended option the graph-only findings are "a count, never red", so the three live advisories vanish from everything the owner reads.
- **Fix to the plan.**
  - Add an option and rank it first on accuracy: **graph-only by the register, fail-closed.** Every build-list module is read. A finding is red unless its module's register entry is classified never-built, and then it is *named* (id, module, fixed-in) in the reason and still green. The `deps` row checks that no module classified never-built appears in the compiled set, so the classification cannot go stale.
  - This needs `scope` to carry a machine-readable kind (a register format change, so ground rule 3: ask).
  - Keep "compiled set, counted" as the second option. In every option, name the graph-only ids, never only a count.
  - Replace the "exactly what Google's scanner reports" sentence.
  - Say that acceptance line 1's "any selected module" changes meaning, so the card is annotated at G0.

**A-S2. "Anything the repository uses" silently excludes the pinned Go binaries in `tools.json`.**
- **Evidence.**
  - `go version -m ~/.cache/vorsar/cep/tools/golangci-lint-v2.15.0-d1e409481a8c/golangci-lint` reads `go1.27.0` with 207 `dep` lines.
  - Against today's database, stdlib at 1.27.0 matches 15 entries: GO-2026-6599, -6600, -6603, -6604, -6605, -6607 to -6613, -6617, -6809 and -6859 (`scripts/q.py`).
  - golangci-lint is installed and parity-checked in every job today, though no row runs it yet (no `golangci` in `gate/registry.json`). S1.1c-5 makes it run on the code.
  - §0.9 equates "every gate tool" with the `tool` lines. The `vulns` reason would print "stdlib and toolchain at go1.27.4 — no known vulnerability" while the gate executes a binary built on a stdlib with 15 known entries.
- **Fix to the plan.**
  - State it in §0, then ask the owner whether the scan also reads each Go binary's build info: `go version -m` gives its Go version and modules, offline, read by the same matcher. That is red on landing today. The alternative is to record the binaries as a named residue until S1.1c-5 runs golangci-lint.
  - Also name ruff and the CPython builds as outside the Go database (a residue).

**A-S3. A network step and a bare check sit inside the shared `lint` job, and two reds have no way to unblock a PR.**
- **Evidence.**
  - `_job` (`generate.py:78-94`) runs setup steps and then the runner, and uploads with `if: always()`.
  - A failing `make vulndb` step (vuln.go.dev down, a truncated body) skips the runner. Every lint row (17 today) then reads *missing* in `strict-gate`, misattributed.
  - The planned `git diff --exit-code -- go.sum gate/proofs/go.sum` step (§2 S3) has the same effect. It is also a check outside the registry: no record, no LAW Δ line, not run by `make strict-gate`.
  - Under Q2, a database older than 30 days is red on every PR, and an outage is red too. Waivers are per finding id (`allowlists.py` entries hold `target`), so neither red has an unblock path. Ground rule 11 forbids the only other remedy, retrying.
- **Fix to the plan.**
  - Either `make vulndb` never fails the step (it records its error, and the row reports "no copy: make vulndb failed: <reason>"), or `vulns` gets its own job.
  - The go.sum-clean check becomes part of a row at T4 (e.g. `deps`), not a bare step.
  - Name the unblock path for a stale or unreachable source: for example a TEMP entry with target `vulndb:stale` (expiry and issue), or CI falling back to a cached copy no older than 7 days, the local rule.

**A-S4. The canary's rendering will drift and can cancel the gate.**
- **Evidence.**
  - §2 S4 renders "from a fixed list of the T4 jobs". A T4 job added later (S1.1c-5's lint set) is then never run under the candidate, and the canary reports a green it never tested.
  - `render`'s non-nightly branch (`generate.py:148-152`) sets `group: gate-${{ github.ref }}` with `cancel-in-progress` on pull requests. A canary built from those lines shares the gate's concurrency group on any PR touching `gate/canary/**`, this sprint's own PR included, so one cancels the other, `strict-gate` included.
  - "Restored, never saved" needs `actions/cache/restore`. `_setup_steps` uses `actions/cache`, which saves in its post step (`generate.py:62-72`, keys carry `github.sha`). decisions.md:222 allows four actions, `cache` among them.
- **Fix to the plan.**
  - Take the canary's jobs from `registry.select(rows, "T4")`, with a test that they equal gate.yml's jobs.
  - Give the canary its own concurrency group (`canary-${{ github.ref }}`), pinned by a test that it differs from gate.yml's.
  - Name `actions/cache/restore@<the same pinned SHA>` and state that it falls under decision 3's `cache`, or ask the owner.

**A-S5. The candidate handoff and the token's reach are unnamed.**
- **Evidence.**
  - §2 S4 says nothing of how one candidate `tools.json` reaches every T4 job. Computed per job, two jobs can see different "latest" releases if one lands mid-run.
  - The darwin entries' tree and exec hashes come from a Linux runner (R-V2 §6 proved cross-extraction for ruff).
  - `GITHUB_TOKEN` "read for the feeds" is not confined. The T4 jobs execute freshly downloaded, unvetted upstream binaries.
- **Fix to the plan.** One Linux job computes the candidate once and uploads it as an artifact, and every T4 job downloads that exact file. The token is set as `env` on the feeds step only, never on steps that run candidate tools. State both in P4.

**A-S6. Generated workflows have no owner while `generate.py` changes hands.**
- **Evidence.**
  - §4 gives `generate.py` to E (W1) and then F (W2), but `make ci-workflow`'s outputs and ci-fresh's registry/law entries to the coordinator only after W2.
  - `ci_fresh` compares the tracked workflows with `generate.targets()`, a link GitNexus confirms (`impact _setup_steps`).
  - So every lane run after E's `NEEDS_MODULES` and cache-key change, or after the W0 commit adds `vulns` to the registry (which changes the lint job in gate.yml and nightly.yml), has ci-fresh red. Neither lane can finish green.
- **Fix to the plan.** The commit that changes the registry or `generate.py` regenerates the workflows. W0's commit carries gate.yml and nightly.yml. E's W1 carries them too. F's W2 carries `canary.yml` together with the registry and law ci-fresh scope and `min_wants` 3, in one commit. Otherwise state the expected red, with its exact text, in each brief.

**A-S7. The compiled set depends on the host platform.**
- **Evidence.** `go list -deps` evaluates build tags for the host's GOOS/GOARCH (`rows/deps.py:27` fixes only CGO). A module imported only under `_linux.go` files is compiled in CI but graph-only on a Mac, so one commit, day and database give different verdicts. Decision 5 accepted day and network dependence, not platform dependence. Measured today: `GOOS=linux GOARCH=amd64` gives the same set as darwin, so this is latent, not red.
- **Fix to the plan.** Take the union over both supported platforms (`manifest.SUPPORTED_PLATFORMS`): two offline listings per module. Add a counter-example with a planted linux-only import.

**A-S8. Q3's ranking weighs a rare event above a stop-the-line one, and its figures are not "cold".**
- **Evidence.** The recommended 60 s saves four minutes on a stuck read, a hung filesystem that has never been seen. The cost is a false red, which reddens a whole job and is a flake under ground rule 11, sized from 4 macOS samples (max 10.36 s). The CI trees are restored by `actions/cache` just before parity, so the page cache is likely warm. `parity.py:31-37` and S1.1c-3-3 asked for *cold* figures.
- **Fix to the plan.** Call the figures "CI, tree restored from cache", not cold. State the false-red risk in Q3. Either rank 120 s first, or recommend 60 s only after the draft PR's runs add macOS samples.

**A-S9. The `deps` row taking tidy fits the subject, but as planned a tidy failure would mask the row's other verdicts.**
- **Evidence.**
  - `run()` raises `Failed` and `main` returns that one line (`rows/deps.py:39-41`, `:128-129`), so any failing command aborts the allowlist, licence and one-version checks.
  - `go mod tidy` loads tests of dependencies. gate/proofs's tidy wants go.sum lines for go-cmp v0.6.0, expect and packagestest (measured with `go mod tidy -diff`), so offline it needs graph-only zips a fresh local cache may lack. A long-green row would turn red there with its real checks unread.
  - "Exactly go.mod:13-20's block" as diff text is brittle to neighbouring edits.
- **Fix to the plan.** Run tidy last and report its failure as one more named line. P3 measures offline tidy in a cold *local* cache as well as in CI. Express the root's allowance semantically: the commented block's two requirements, plus the go.sum lines of x/mod, x/sync and x/tools.

**A-S10. The canary's red is not placed against ground rule 11.**
- **Evidence.** The first runs will be red or carry "pins to update" (ruff 0.17.5 and python-build-standalone 20261110 per §0.12). The weekly run on `main` is then a red workflow on `main`, and ground rule 11 says "Stop the line for red main". The plan never says whether a canary red counts.
- **Fix to the plan.** Add confirm (f) for the owner: the canary is a report, never a required check and never a stop-the-line red. Ground rule 11 applies to gate.yml and nightly.yml.

## Later

- **A-L1.** `archive_pins` re-composes the installer's private steps (`_Extraction`, the stamp, `_seal`, `_restore_write`; `install.py:251-288`) a second time.
  - Extract one private helper that both `_install_tool` and `archive_pins` call, so the two cannot drift.
  - Counter-example (11), "over a recorded archive reproduces a committed pin", needs a real tool archive in testdata (tens of MB) or the network in T2. Use an equivalence test over a small synthetic tar.gz instead (`archive_pins` equals the installer's tree), with the one-off ruff 0.17.3 reproduction quoted in F's report.
- **A-L2.** `vulndb_fetch.py` sits in `gate/runner/`, while the gate's only network edge today is `gate/toolchain/install.py` (urllib, `DOWNLOAD_TIMEOUT_S`, temp file plus rename).
  - Put the fetcher beside it and reuse those.
  - Add a fitness function: ruff `banned-api` on `urllib.request` and `http.client`, with per-file ignores for exactly install.py, the fetcher and `gate/canary/feeds.py`.
- **A-L3.** The waiver list is named `govulncheck`, a tool the repository will not use (x/vuln not added). Rename the rule and file to `vulns`; that is card wording, not a recorded decision. Also name where a PERMANENT entry in that list is refused: the generic grammar accepts both kinds.
- **A-L4.** `rows/deps.py:112` carries an inline `datetime.now(UTC).date()`. BA9's standing rule moves it to `rows.today()` when the file changes, which lane E does. Add it to E's brief.
- **A-L5.** Stale and hostile-input details:
  - S1.1c-3's [4] counter-example (11), "a zip unpacked one level deep", is meaningless once the zip is read in place. Replace it with "members under a top folder -> red".
  - `check_db` should also verify the zip's sha256 against the stamp.
  - The per-member cap must count decompressed bytes read, not the header's declared size.
- **A-L6.** The SP line says "≈ 7.5–8.7 built, inside the card", but 8.7 exceeds the card's 8.
- **A-L7.** For S2, `go list -e -deps tool` (the `tool` meta-pattern, measured offline in gate/proofs) is simpler than `go mod edit -json` plus a loop.
- **A-L8.** At G0, annotate the card's acceptance line 1. Its "week-old database" becomes "a copy fetched more than 7 days ago, or a database unchanged for 30" under Q2, and its "selected" changes under Q1.

## The-fool: pre-mortem ("two days from now S1.1c-4 failed or shipped a gate that lies — why?")

Steelman: the plan reuses decided mechanisms, measured the as-decided scan red before G0, and keeps the network at thin edges. The plausible failure stories, ranked by likelihood times impact:

1. **The green that was not checked** (A-B1, A-S7). A local T4 on a laptop whose module cache lost a zip prints `vulns: ... 6 modules built, 17 graph-only — no known vulnerability`. The dropped module is the one with the new advisory. Nobody notices, because "graph-only" is a count. *Early sign:* the built count moves without a go.mod change. *Mitigation:* load errors are red; `min_wants` is set at 9; fail-closed by the register.
2. **The sprint's own PR fights itself** (A-S4, A-S6). The draft PR touches `gate/canary/**`. The canary and the gate share `gate-<ref>` with cancel-in-progress, so `strict-gate` shows cancelled. Meanwhile each lane's ci-fresh was red and was waved through as "expected". *Mitigation:* a separate concurrency group, regeneration owned per commit, tests pinning both.
3. **vuln.go.dev has a bad hour on merge day** (A-S3). The lint job dies at `make vulndb`, and 17 rows read missing. The owner's check reads a wall of "missing", and ground rule 11 forbids a retry. *Mitigation:* the fetch never kills the job; the row names the failure; an unblock path exists.
4. **The scan tells the truth about the wrong thing** (A-S1, A-S2). Three months later S1.1c-5 runs golangci-lint on every PR. It is built on go1.27.0 with 15 stdlib entries, while `vulns` prints "no known vulnerability" every night. *Mitigation:* the owner decides now; the reason line names exactly what was read.
5. **The canary is always red, so it is read as noise or as red main** (A-S10, A-S4). After the first run (ruff 0.17.5) it stays red weekly. Either it is ignored, or the coordinator stops the line on it. A T4 job added in S1.1c-5 is never exercised by the fixed job list. *Mitigation:* confirm (f); jobs taken from the registry.

## Verdict

The plan's evidence holds (re-measured: the three graph-only findings, the compiled set, the tidy diffs, the counts), and the scope is one coherent change. Fix A-B1 before the owner reads Q1, and rework Q1's framing to include the register's fail-closed option; the should-fix items are seam and ownership fixes the design note can carry.
