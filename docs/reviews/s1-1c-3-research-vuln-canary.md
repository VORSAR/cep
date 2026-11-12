> **S1.1c-3 plan check research, 2026-11-12 — the vulnerability scan, the dependency register, the CI generator and the canary's sources, re-measured (R-V2).** A read-only research run in a scratch clone of `main` @ c87942d (Go 1.27.4), re-measuring [R-V](s1-1c-2-research-vuln-canary.md); its findings are folded into [the plan check](../sprints/s1-1c-3-plan-check.md) §0. Paths below are the session scratchpad's.

# S1.1c-3 research: govulncheck + canary, re-measured on main @ c87942d (2026-11-12)

Scratch root $S = /private/tmp/claude-501/-Users-ramith-code-cep/33e77a3e-2c9e-4bc7-ac54-01eb79c6a3d0/scratchpad/s1-1c-3-research/vuln-canary
Clone: $S/cep at c87942d. Toolchain: ~/.cache/vorsar/cep/tools/go-go1.27.4-91a44a2b8a71/bin/go ("go version go1.27.4 darwin/arm64") first on PATH, GOTOOLCHAIN=local, GOMODCACHE=$S/modcache, GOCACHE=$S/gocache, GOFLAGS empty (env in $S/env.sh). Python for gate code: the pinned python-3.15.2. "Offline" = GOPROXY=off, GOFLAGS=-mod=readonly, under `sandbox-exec -p '(version 1)(allow default)(deny network-outbound (remote ip))'` (proved: curl under it -> "Couldn't connect to server", exit 7). /Users/ramith/code/cep untouched (git status clean, HEAD c87942d). Final scratch clone state: gate/proofs go.mod/go.sum with the tool, root go.mod/go.sum with the telemetry pin, candidate allowlist entries, gate/current-sprint = S1.1c-3.

## 1. golang.org/x/vuln today

- Latest: v1.9.0 (unchanged). `go list -m -versions golang.org/x/vuln` -> "... v1.7.0 v1.8.0 v1.9.0"; `@latest` Time "2026-10-11T03:39:53Z", GoVersion "1.26.0".
- Licence: BSD-3-Clause ($S/modcache/golang.org/x/vuln@v1.9.0/LICENSE: "Copyright 2009 The Go Authors. / Redistribution and use in source and binary forms..."; the deps row's classifier would read it as BSD-3-Clause).
- `cd gate/proofs && go get -tool golang.org/x/vuln/cmd/govulncheck@v1.9.0` from a pristine tree -> exit 0:
  "go: upgraded golang.org/x/telemetry v0.0.0-20261009151841-97e8ea77aa2a => v0.0.0-20261010193011-f71dc92da45e" / "go: added golang.org/x/vuln v1.9.0". Only gate/proofs/go.mod (+16/-4) and gate/proofs/go.sum (+6) change; the root is untouched.
  go.mod diff:
  ```
  -require go.uber.org/goleak v1.3.0
  -
  -// The gate's API-diff tool, declared here so an embedder's module graph never carries it (owner, 2026-11-11, S1.1c-1's design, 7).
  -tool golang.org/x/exp/cmd/apidiff
  +tool (
  +	// The gate's API-diff tool, declared here so an embedder's module graph never carries it (owner, 2026-11-11, S1.1c-1's design, 7).
  +	golang.org/x/exp/cmd/apidiff
  +	golang.org/x/vuln/cmd/govulncheck
  +)
   require (
  +	go.uber.org/goleak v1.3.0
   	golang.org/x/exp v0.0.0-20261106192419-47ce03d25ef0
   	... (mod, sync, tools unchanged)
   )
  +
  +require (
  +	golang.org/x/sys v0.49.0 // indirect
  +	golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect
  +	golang.org/x/vuln v1.9.0 // indirect
  +)
  ```
  go.sum +6: x/sys v0.49.0 (h1 + /go.mod), x/telemetry 20261010193011-f71dc92da45e (h1:P6NGsugec4oASJ+yKMNgTndeo0h2CbixZLvQIe9h5R4= + /go.mod h1:GNu+nb+E5axfGmP+Uu3ZVQu5lE/Xk42MjZOHWUD94/o=), x/vuln v1.9.0 (h1:12VaqnI9kJfqwRoPptxoqsUhQtdfrSbdksT7X/KY/sc= + /go.mod h1:E/Frwm0xp49fTyBW+6CQHdkvobknbwUM49yEpMGUauw=).
- New `go mod graph` nodes (5, none removed): github.com/google/go-cmdtest@v0.4.1-0.20220921163831-55ab3332a786 (Apache-2.0), github.com/google/renameio@v0.1.0 (Apache-2.0), golang.org/x/telemetry@v0.0.0-20261010193011-f71dc92da45e (BSD-3), golang.org/x/tools@v0.52.0 (BSD-3; a graph-only version - selected stays the pinned pseudo-version), golang.org/x/vuln@v1.9.0 (BSD-3). Licences from the LICENSE files in $S/modcache (go-cmdtest and renameio files are byte-identical, sha256 prefix cfc7749b96f63bd3; the four x/ files share prefix 911f8f5782931320). x/sys v0.49.0 was already a node.
- `go list -m all` (proofs) diff: +go-cmdtest, +renameio, +x/vuln v1.9.0, telemetry 20261009151841 -> 20261010193011.
- Cross-module skew (root `go list -m all` vs proofs with the tool): exactly one module: `('golang.org/x/telemetry', 'v0.0.0-20261009151841-97e8ea77aa2a', 'v0.0.0-20261010193011-f71dc92da45e')`. Cause: x/vuln v1.9.0's go.mod requires `golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e` (also go-cmdtest, go-cmp v0.6.0, mod v0.42.0, sync v0.24.0, tools v0.52.0, packagestest v0.1.1-deprecated; indirect renameio v0.1.0, sys v0.49.0, expect v0.1.1-deprecated).
- **The needed root pin is still `golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect`.** Proved two ways:
  - Minimal: one go.mod line in the existing indirect block + one go.sum line (`golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e/go.mod h1:GNu+nb+E5axfGmP+Uu3ZVQu5lE/Xk42MjZOHWUD94/o=`). Under GOFLAGS=-mod=readonly: `go list -m all` exit 0, `go build ./...` exit 0, `go vet ./...` exit 0, `go mod verify` "all modules verified"; skew after pin: `[]`. Without the go.sum line: "missing go.sum entry for go.mod file", exit 1.
  - `go get golang.org/x/telemetry@v0.0.0-20261010193011-f71dc92da45e` in the root writes 3 go.mod lines (x/sync v0.24.0, x/sys v0.49.0, x/telemetry, all `// indirect`) and 4 go.sum lines (sys h1+go.mod, telemetry h1+go.mod). Also skew `[]`.
- Neither module is `go mod tidy`-clean on pristine c87942d (`go mod tidy -diff` exit 1 in both: the root's tidy drops the comment-pinned mod/tools block; proofs' tidy marks exp/mod/sync/tools `// indirect` and adds go-cmp/expect/packagestest go.sum lines). The gate does not run tidy (no "tidy" in gate/runner, gate/ci, gate/registry.json). With the tool, tidy would additionally add go-cmdtest and renameio go.sum lines - not needed by any command run here.
- Build: `go build golang.org/x/vuln/cmd/govulncheck` exit 0 with go1.27.4; `go version -m`: links x/vuln v1.9.0, x/mod v0.42.0, x/sync v0.24.0, x/telemetry 20261010193011, x/tools (pinned pseudo). go-cmdtest/renameio not linked.
- Side observation: `go mod download all` (the CI warming step, generate.py:53) itself writes 14 lines into the root go.sum (go-cmp, kr/pretty, goldmark, x/net, x/sys, x/telemetry 20261009, gopkg.in/check.v1) - a dirty go.sum in the CI checkout after that step. Pre-existing, not caused by the tool.

## 2. The real-tree verdict

- `curl -sS -D headers.txt -o vulndb.zip https://vuln.go.dev/vulndb.zip` (11:46:12 UTC) -> "HTTP/2 200", "content-length: 3599175", "last-modified: Thu, 12 Nov 2026 11:15:42 GMT", "etag: \"855c49c13011d8149630eecf6c813fb9\"", "cache-control: public, max-age=3600", content-type application/zip.
- sha256 `714ff02d9533ec6ca269e565b61b27cbd959aa0af89d51f318261322e875861c` - **identical to 2026-11-11's** despite the new last-modified (re-upload, same bytes). Unpacked (no top folder): ID/ 4801 files (newest GO-2026-6917), index/{db.json,modules.json,vulns.json}, 4804 .json, 0 .gz. `index/db.json` = `{"modified":"2026-11-11T02:51:11Z"}` (unchanged). modules.json: 1457 modules. stdlib's newest: GO-2026-6859 fixed 1.27.4. x/text's newest: GO-2026-6629 fixed 0.41.0 (pinned v0.43.0).
- `govulncheck -db file://… -version` (offline) -> "Go: go1.27.4 / Scanner: govulncheck@v1.9.0 / DB: file:///… / DB updated: 2026-11-11 02:51:11 +0000 UTC", exit 0.
- Real DB, offline, every result (script $S/scripts/cover.sh):
  ```
  [cep]    -scan module            -> exit=0  "No vulnerabilities found."
  [cep]    -scan module -test      -> exit=0
  [cep]    -scan package ./...     -> exit=0
  [cep]    -scan package -test ./... -> exit=0
  [cep]    -scan symbol ./...      -> exit=0  (default level)
  [cep]    -scan symbol -test ./... -> exit=0
  [cep]    -scan package all       -> exit=0
  [proofs] -scan module            -> exit=1  "govulncheck: loading packages: There are errors with the provided package patterns: -: no Go files in …/cep/gate/proofs"
  [proofs] -scan package ./...     -> exit=0
  [proofs] -scan package -test ./... -> exit=0
  [proofs] -scan package tool      -> exit=0
  [proofs] -scan package all       -> exit=0
  [proofs] -scan symbol tool       -> exit=0
  go tool govulncheck -db … -scan module -C $S/cep   (run from gate/proofs) -> exit 0
  go tool govulncheck -db … -scan module -C $S/cep/gate/proofs              -> exit 1 (same "no Go files")
  ```
  No finding id anywhere. `-scan module -show verbose` on the root: "The package pattern matched the following root package: github.com/vorsar/cep / Govulncheck scanned the following 3 modules and the go1.27.4 standard library: github.com/vorsar/cep, github.com/alecthomas/participle/v2@v2.1.4, golang.org/x/text@v0.43.0 / No vulnerabilities found."
- Stdlib-at-pinned-Go proof: same binary, same DB, root `-scan module` with the cached go1.27.3 first on PATH -> "Vulnerability #1: GO-2026-6859 … Found in: stdlib@go1.27.3  Fixed in: stdlib@go1.27.4", exit 3. The stdlib version is `go env GOVERSION` of the `go` on PATH (x/vuln internal/scan/run.go:87-96), not go.mod's toolchain line - the row must run under the pinned Go.
- **Would the new row be red on landing today?** No finding exists: every well-formed invocation on both modules exits 0. It would be red (exit 1, an error, not a finding) only if it ran `-scan module` inside gate/proofs, whose root directory has no Go files.

### 2a. NEW: what `-scan module` actually covers in v1.9.0 (proved with a planted DB)
- `-scan module` refuses package patterns: `govulncheck -scan module ./...` -> "patterns are not accepted for module only scanning", exit 2 (internal/scan/flags.go:116-117). (A zsh run passing "-scan module" as one word also exits 2 - usage.)
- With no patterns it calls `packages.Load(cfg)` with none, i.e. the package in the current directory only, no tests (internal/scan/source.go:23-48; internal/vulncheck/packages.go:199-227), and always adds the stdlib module at GOVERSION (packages.go:30-46). It is **not** `go list -m all`.
- Planted DB ($S/planted-db, from $S/scripts/plantdb.py): one module-wide entry per module, introduced 0, no fix (stdlib: fixed 1.27.5, imports only net/smtp, which nothing imports). Ids: 0001 stdlib, 0002 x/text, 0003 participle, 0004 goleak, 0005 x/tools, 0006 x/mod, 0007 x/telemetry, 0008 x/vuln, 0009 x/exp, 0010 go-cmdtest, 0011 x/sync, 0012 alecthomas/repr, 0013 x/sys. Results (all exit 3 except where noted):
  ```
  [cep]    -scan module              -> 0001 0002 0003
  [cep]    -scan module -test        -> 0001 0002 0003 0004
  [cep]    -scan package ./...       -> 0002 0003            (no stdlib: net/smtp not imported)
  [cep]    -scan package -test ./... -> 0002 0003 0004
  [cep]    -scan symbol ./...        -> 0002 0003
  [cep]    -scan symbol -test ./...  -> 0002 0003 0004
  [cep]    -scan package all         -> 0002 0003 0004
  [proofs] -scan module              -> exit 1 (no Go files)
  [proofs/internal/leaky] -scan module -> 0001
  [proofs] -scan package ./...       -> exit 0, none
  [proofs] -scan package -test ./... -> exit 0, none   (goleak MISSED, see below)
  [proofs] -scan package tool        -> 0005 0006 0007 0008 0009 0011
  [proofs] -scan package all         -> 0004 0005 0006 0007 0008 0009 0011
  [proofs] -scan symbol tool         -> 0005 0006 0007 0008 0009 0011
  ```
  Never reported by any mode: 0010 go-cmdtest, 0012 repr, 0013 x/sys - graph-only (never compiled; `go list -deps tool` in proofs = exp, mod, sync, telemetry, tools, vuln; root `-deps -test ./...` = participle, text, goleak).
  So the root `-scan module` alone covers stdlib + participle + x/text; it misses goleak (tests) and every gate/proofs tool module. In the measured runs, root `-scan module -test` plus proofs `-scan package all` covered every compiled module and the stdlib (inferred: that union is one way to meet decision 12's "anything the repository uses"; a design choice, not made here).
  Today root `.` and `./...` reach the same modules (participle, x/text), though 6 of the root's 20 packages are not reached from `.` - a future dependency imported only by one of those would be missed by `-scan module` (inferred from the load rule).
- NEW: `-test` at package level misses a module imported only by an in-package test file: gate/proofs/internal/leaky/leaky_test.go is `package leaky` and imports goleak (`go list` TestImports = [go.uber.org/goleak testing]); govulncheck `-scan package -test ./...` there lists "leaky.test" as a root but scans only "1 modules and the go1.27.4 standard library". Root's goleak imports are all in external `_test` packages (main_test.go `package cep_test`, io/inmemory, internal/runtime, internal/playback) and are found. Likely cause (inferred from source): AddPackages dedupes by PkgPath (internal/vulncheck/packages.go:142-147, "TODO: check duplicates are okay?"), so the test variant of `leaky` is dropped. `-scan package all` in proofs does find goleak.
- NEW: an OSV entry without `database_specific.url` panics the text output ("nil pointer dereference" at internal/scan/text.go:270), exit 2. Planted fixtures must carry it (the real DB's entries do).

## 3. The dependency register (tools/deps-allowlist.json, gate/runner/deps.py, gate/runner/rows/deps.py)

- Entry keys, exactly (deps.py:30): `module, version, scope, license, approved, why, sprint, ref`, all non-empty strings (deps.py:89-91); optional `replace` (deps.py:84-88); top keys `schema` (=1), `note`, `modules` non-empty, modules unique (deps.py:29, 66-107). 23 entries today; x/telemetry is listed at v0.0.0-20261009151841-97e8ea77aa2a, sprint S3.3-1, ref …#owner-decisions-2026-11-07--s33s-plan-check (allowlist lines 185-194).
- `sprint` must be a row of gate/sprints.json and not after gate/current-sprint (allowlists.py:143-154); `ref` must be `docs/architecture/adr/…` or `docs/sprints/decisions.md#…` and resolve in tracked Markdown (allowlists.py:157-171; rows/deps.py:95-118). The plan-check heading's anchor is `docs/sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-plan-check` (decisions.md:80 index link).
- What the row checks (deps.py:175-251): tracked go.mod set is exactly the two (:189-193); every build-list module has an entry (:196-198); every graph node (path only - versions are cut, :165-172) has an entry (:199-201) and is selected by a build list (:202-203); stale entries (:204-206); replace approval (:207-214); entry version == selected version (:215-220); SPDX parts within Apache-2.0/MIT/BSD-2/BSD-3/ISC (:25, :221-229); licence-file fingerprint only for imported modules (:230-245); sprint/ref (:246-250). One version across modules: rows/deps.py:83-87 -> deps.two_versions_line (deps.py:126-131). The row: GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, CGO_ENABLED=0 (rows/deps.py:27); imported = `go list -e -deps -test ./...` in each module (:75-76) plus `go list -e -deps <TOOL>` with `TOOL = "golang.org/x/exp/cmd/apidiff"` only (:25, :77-82); Module records take the root's version first (`setdefault`, :88-91).
- Graph-only versions (x/tools@v0.52.0) never trip anything: only paths are compared.
- The real row, run in the clone (pinned python, GATE_ROW_DIR set, $S/scripts/depsrow.sh):
  ```
  pristine c87942d                 -> GATE_COVERED 3, exit 0
  tool added (proofs only)         -> exit 1
    failed: deps: golang.org/x/telemetry: go.mod has v0.0.0-20261009151841-97e8ea77aa2a, gate/proofs/go.mod has v0.0.0-20261010193011-f71dc92da45e; one version across both modules: restore the pins in go.mod (see its comment)
    failed: deps: github.com/google/go-cmdtest v0.4.1-0.20220921163831-55ab3332a786 is not in tools/deps-allowlist.json
    failed: deps: github.com/google/renameio v0.1.0 is not in tools/deps-allowlist.json
    failed: deps: golang.org/x/vuln v1.9.0 is not in tools/deps-allowlist.json
  + root pin (1 go.mod + 1 go.sum line) -> exit 1: the same three "not in" lines plus
    failed: deps: golang.org/x/telemetry: the allowlist says v0.0.0-20261009151841-97e8ea77aa2a, the graph has v0.0.0-20261010193011-f71dc92da45e
  + entries (x/vuln v1.9.0 BSD-3-Clause; go-cmdtest and renameio Apache-2.0 "module graph only"; telemetry version bumped; sprint "S1.1c-3", ref …#owner-decisions-2026-11-12--s11c-2s-plan-check)
      with gate/current-sprint = S1.1c-2 (as on main) -> exit 1:
        failed: deps: github.com/google/go-cmdtest: sprint S1.1c-3 is after the current sprint S1.1c-2   (same for renameio, x/vuln)
      with gate/current-sprint = S1.1c-3 -> GATE_COVERED 3, exit 0
  ```
  Note: before the root pin, the telemetry allowlist mismatch is masked (the root's version is the one compared, rows/deps.py:91).
- Needed entries: golang.org/x/vuln v1.9.0 (BSD-3-Clause), github.com/google/go-cmdtest v0.4.1-0.20220921163831-55ab3332a786 (Apache-2.0), github.com/google/renameio v0.1.0 (Apache-2.0); x/telemetry's `version` -> v0.0.0-20261010193011-f71dc92da45e (whether its sprint/ref also move is a choice). x/sys, x/tools, x/mod, x/sync already listed at the selected versions. No other module is new.
- Gap (proved): with `TOOL` = apidiff only, x/vuln and x/telemetry are not "imported", so their licence files are never fingerprinted. Planting `license: "MIT"` on x/vuln -> exit 0 (missed); planting `MIT` on x/exp -> "failed: deps: golang.org/x/exp: license field MIT, file reads BSD-3-Clause", exit 1. `go list -e -deps -f … golang.org/x/vuln/cmd/govulncheck` in proofs yields mod, sync, telemetry, tools, vuln - adding govulncheck to TOOL (e.g. a tuple) would cover them (inferred; not run).

## 4. CI generator (gate/ci/generate.py on c87942d)

- `WORKFLOWS = {"T4": WORKFLOW, "nightly": ".github/workflows/nightly.yml"}` (:24); `targets()` emits one workflow per tier in WORKFLOWS that has rows (:167-173); `main()` writes them (:176-190).
- `NEEDS_MODULES = frozenset({"deps", "proofs"})` (:52); `MODULE_DOWNLOAD = "      - run: go mod download all"` (:53), emitted twice, the second with `working-directory: gate/proofs`, after `make setup tools` (:73-74). Pinned whole by test_generate.py:69-86 (`assertEqual(generate.NEEDS_MODULES, frozenset({"deps", "proofs"}))` at :72). A govulncheck row offline (GOPROXY=off) needs its id in NEEDS_MODULES, else its job has no download step (inferred).
- Module-cache key lines (:66-72): path `~/.cache/go-build` + `~/go/pkg/mod`; `key: go-${{ runner.os }}-${{ hashFiles('go.mod', 'go.sum') }}-${{ github.sha }}`, `restore-keys: go-${{ runner.os }}-${{ hashFiles('go.mod', 'go.sum') }}-`. Only the root go.mod/go.sum are hashed: a gate/proofs-only change (adding or bumping x/vuln) does not change the key; the root telemetry pin does, once. Tools cache key: `tools-${{ runner.os }}-${{ hashFiles('tools.json') }}` (:65).
- Nightly: `render(rows, "nightly")` (:131-164): rows with "nightly" in tiers grouped by `job`; triggers `schedule: - cron: '0 3 * * *'` + `workflow_dispatch:` (:140-144); concurrency `group: nightly`, `cancel-in-progress: false` (:148-152); `permissions: contents: read` (:145-146); env CGO_ENABLED '0'; each job cap = max(10, ceil((Σtimeout_s·1.2+60)/60)+10) minutes (:158-162); then `strict-gate` (:114-128) summarizing `--tier nightly`. Current .github/workflows/nightly.yml: jobs `lint` (suppressions, 13 min), `nightly` (fuzz-parser 900 s, fuzz-decoder 900 s, race-stress 1800 s; 83 min), `strict-gate`. Registry tiers are fixed at `TIERS = ("T0", "T1", "T2", "T3", "T4", "nightly")` (gate/runner/registry.py:14); `select()` refuses zero rows (registry.py:234-242).
- A third workflow (inferred from the code; not built or run): render() is two-way (`nightly = tier == "nightly"`, :135), so canary.yml from the registry needs a new tier in registry.TIERS, a WORKFLOWS entry, and a third trigger/concurrency branch in render() (`schedule` weekly + `workflow_dispatch` + `pull_request: paths: ['gate/canary/**']` per decision 18; test_the_posture asserts `"paths:"` absent only from gate.yml, test_generate.py:155). The other route is a renderer outside the tier system. The canary also needs network and, for the GitHub API at 60/h unauthenticated, a token env (open question).
- ci-fresh: registry row gate/registry.json:240-262 - `"expected_scope": ["git", ".github/workflows/gate.yml", ".github/workflows/nightly.yml"]` (:254-258), `"min_wants": 2` (:259), job lint, T1-T4. Its law instrument gate/law.json:401-427 pins the same (`"min_wants": 2` :422, expected_scope :423-426; code generate.py, registry.py, rows/__init__.py, rows/ci_fresh.py; proof gate/proofs/drivers/test_cifresh_planted.py, 2 cases). law.py:77: "A row instrument pins its row's tiers, min_wants and expected_scope"; teeth.py:504 checks registry <-> instruments both ways. The row (gate/runner/rows/ci_fresh.py:16-37) compares the tracked files of its scope with `generate.targets()`; a target missing from the scope prints "<path> is not tracked: run make ci-workflow and git add it" (:32-34), and covered = len(tracked) (:36). So a generated canary.yml needs registry.json:254-259 and law.json:422-426 to list it with min_wants 3 (inferred from code). `.github/workflows/` (kind workflow) and `gate/` (kind code) are already whole in law.json's `paths`, so gate/canary/ and canary.yml are law files on arrival.

## 5. Canary sources today (unauthenticated; GITHUB_TOKEN/GH_TOKEN unset; curl's default user agent)

| Tool | Pinned (tools.json) | Latest feed | sha256 from feed |
|---|---|---|---|
| go | go1.27.4 | https://go.dev/dl/?mode=json: [('go1.27.4', True), ('go1.26.11', True)] -> go1.27.4 (= pin) | files[].sha256: darwin-arm64 91a44a2b8a716600ad94a4f143e156116620db8828086a6e3602291be9bd0bb4, linux-amd64 7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840 (= pins); file keys arch, filename, kind, os, sha256, size, version |
| golangci-lint | v2.15.0 | releases/latest tag_name v2.15.0, published 2026-10-16T11:33:37Z (= pin) | assets[].digest present on 78/78; darwin-arm64 sha256:d1e409481a8c…47d5, linux-amd64 sha256:9b7fe6a6bf63…bcea (= pins) |
| ruff | 0.17.3 | 0.17.4, published 2026-11-05T18:53:24Z (behind) | digest on 40/40; ruff-aarch64-apple-darwin.tar.gz sha256:6ef8b3a7fd5294f5e8384ceb29784a6f5657da0853824312b1d2cef53aef17af; ruff-x86_64-unknown-linux-gnu.tar.gz sha256:3ab2b34aeacdbc1ea5c529cdbda3e05792633d587228123fd09f3b5a0facb27a; also per-asset .sha256 files |
| python / python-floor | 3.15.2+20261026 / 3.12.16+20261026 | python-build-standalone 20261110, published 2026-11-11T01:51:17Z (behind; same CPython versions 3.15.2 and 3.12.16) | digest on 991/991; 3.15.2 darwin sha256:6767ca67…cbab, linux sha256:182e7adf…ac18; 3.12.16 darwin sha256:e956e60b…0b8f, linux sha256:c2f87d4b…2e99; also SHA256SUMS. Asset `name` has "+", browser_download_url "%2B" (as tools.json) |
| lark | 1.3.1 | https://pypi.org/pypi/lark/json info.version 1.3.1 (= pin), info.license 'MIT', license_expression None | sdist lark-1.3.1.tar.gz digests.sha256 b426a7a6d6d53189d318f2b6236ab5d6429eaf09259f1ca33eb716eed10d2905 (= pin), yanked False; a wheel is also listed |

- The python-build-standalone release carries CPython 3.11.18, 3.12.16, 3.13.18, 3.14.10, 3.15.2 and **3.16.0a2**: "latest" must be chosen within the pinned minor line and skip pre-releases.
- GitHub asset keys: browser_download_url, content_type, created_at, digest, download_count, id, label, name, node_id, size, state, updated_at, uploader, url. Each release prerelease=False, draft=False.
- Rate-limit headers (api.github.com only; go.dev and PyPI sent none): "x-ratelimit-limit: 60", "x-ratelimit-resource: core", remaining 48 / 47 / 46 (used 12 / 13 / 14) on the three calls, reset 1794486114 - this IP had already spent 11 of 60 this hour before the run.

## 6. What a candidate tools.json needs

- Schema (gate/toolchain/manifest.py): top `{schema: 1, tools: [...]}`; executable keys `command, name, path_dir, platforms, version, version_argv, version_line` (+ optional `kind`) (:35); python-package keys `kind, module, name, path_dir, platforms, version, version_line` (:36); per-platform keys `archive_sha256, exec_sha256, strip, tree_sha256, url` (:37); digests `[0-9a-f]{64}` (:32); platforms ⊆ ("darwin-arm64", "linux-amd64") (:20). Validated by `load()` (:212-229) via `_tool` (:183-209) and `_entry` (:157-165). Cache home `<name>-<version>-<archive_sha256[:12]>` (`tool_home`, :232-234).
- From the feeds (no download): `version`; `url` (go.dev/dl/<filename>, GitHub browser_download_url, PyPI url); `archive_sha256` (go files[].sha256, GitHub digest minus "sha256:", PyPI digests.sha256). `strip` follows the archive's naming (ruff-<target>/, golangci-lint-<ver>-<plat>/, python/, go/, lark-<ver>/) - derivable from the name, confirmed only by extraction (inferred).
- Only by downloading and extracting with the installer's own code: `tree_sha256` and `exec_sha256` (which files are listed is the current entry's set; their content hashes need the tree). `version_line` needs running the tool (`version_argv`, first stdout line, parity.py:145-166; go's is a template `go version go1.27.4 {goos}/{goarch}`; golangci-lint's carries commit and build date: "golangci-lint has version 2.15.0 built with go1.27.0 from 25c5369a on 2026-10-16T11:03:09Z"; lark's is `lark.__version__`). It is per tool, not per platform, so a Linux runner can take it from the linux binary (inferred).
- Method proved ($S/scripts/pins.py, pinned python 3.15.2 -I, repo code from the clone): `install._Extraction(strip, out).extract(archive)` then `install._seal(out, x.modes)` then `manifest.tree_sha256(out)` and `manifest.sha256_file(out/rel)`. It reproduced both committed ruff 0.17.3 pins exactly, including the linux archive extracted on darwin (darwin exec b768f341…, tree b275c168…; linux exec 6761c67c…, tree 9079ac52…). **`_seal` is required**: without it the linux tree hash came out 85a9ae53… (wrong), because the archive's modes (the `x` kind) are applied by _seal. ruff 0.17.4 candidates: darwin archive 6ef8b3a7… (= GitHub digest), exec 80f10a353f138c90ceb4886f11b26e6e7eae9a7add9d38c6ad771dcaaab7ca4b, tree 83757bd8a31a5133282b63be2382746c1913468b5a2e3c91ce258706844367fe; linux archive 3ab2b34a… (= digest), exec 4ef33997b4bb7c5f141527e90b2da0abe918108362ca1b9825c4d13adce0e347, tree 7948f9bca54868c4b32e87503973555395ceecdaa427fe622fba539aa09544b1. Sealed trees need `_restore_write` before removal.
- Functions a canary would reuse: manifest.load (:212), manifest.tool_home (:232), manifest.sha256_file (:251-266), manifest.tree_sha256 (:275-284); install._fetch (:59-61; urllib, 60 s read timeout), install._Extraction (class :92, extract :104-118 - the hostile-archive checks), install._seal (:207-211), install._restore_write (:214-222); the end-to-end shape is install._install_tool (:251-288: fetch -> sha256 check -> extract -> stamp -> seal -> publish). Parity's checks a candidate must pass: parity._hash_failure (:179-183), parity._tree_failure (:186-192), parity._version_failure/_probe (:145-166). These are private (underscore) names (a design point, inferred).
- Beyond tools.json, the Go bump c87942d also touched: go.mod and gate/proofs/go.mod toolchain lines, gate/toolchain/tests/test_manifest.py (:121-160, versions and archive prefixes typed by hand, e.g. "91a44a2b8a71", "7226736c2339"), gate/toolchain/census.md (go rows re-taken), gate/proofs/drivers/test_parity_path.py and test_parity_version.py (version strings), DEVELOPMENT.md, ADR-0015 §3.2. A candidate tools.json alone would leave test_manifest's pin test red (inferred).

## What changed since the 2026-11-11 report
1. Go pinned at go1.27.4: GO-2026-6859 no longer applies; the root `-scan module` is exit 0 (was 3). Go is no longer a canary upgrade.
2. vulndb.zip bytes unchanged (same sha256 714ff02d…, db.json modified still 2026-11-11T02:51:11Z); only the HTTP last-modified moved (2026-11-12 11:15:42 GMT); etag 855c49c1….
3. NEW: `-scan module` refuses package patterns (exit 2), loads only the package in the current directory (no tests) plus the stdlib, and fails with exit 1 in gate/proofs ("no Go files"). It is not a scan of `go list -m all`. Planted coverage matrix in §2a.
4. NEW: `-scan package -test` misses goleak in gate/proofs (in-package test import); `-scan package all` finds it.
5. NEW: an OSV entry lacking `database_specific.url` panics govulncheck's text output (exit 2).
6. NEW precision: proofs go.sum +6 lines; the root pin needs 1 go.mod + 1 go.sum line (or `go get`'s 3 + 4); both pass readonly list/build/vet/verify.
7. NEW: the deps row's exact red lines for each step, the sprint-after-marker fault until gate/current-sprint is S1.1c-3, and the licence-file gap for the tool's modules (`TOOL` is apidiff only).
8. Feeds: same versions as 2026-11-11 (ruff 0.17.4 and pbs 20261110 still ahead of pins; golangci-lint and lark equal); NEW: pbs carries 3.16.0a2; go.dev also lists go1.26.11 stable.
9. NEW: candidate-pin method reproduced the committed ruff pins; `_seal` is part of the method.
10. NEW: neither module is tidy on main (pre-existing; the gate does not run tidy). `go mod download all` dirties the root go.sum (pre-existing).

Scripts and data: $S/scripts/{plantdb.py,cover.sh,depsrow.sh,addentries.py,feeds.py,pins.py}, $S/dl/vulndb, $S/planted-db, $S/feeds, $S/rowdirs.
