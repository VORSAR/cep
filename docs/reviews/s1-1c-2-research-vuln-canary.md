> **S1.1c-2 plan check research, 2026-11-11 — govulncheck, the vulnerability database, the canary's sources and GitHub Actions (R-V).** A read-only research run before the reviews, in a scratch clone of `s1-1c-2/integration` @ fac4e64 (code `main` @ fd274e4); its findings are folded into [the plan check](../sprints/s1-1c-2-plan-check.md) §0. Paths below are the session scratchpad's.

# S1.1c-2 research: govulncheck + canary (2026-11-11)

Scratch root: $S = /private/tmp/claude-501/-Users-ramith-code-cep/bcf47ef3-aff6-461b-85d0-fea8dd101151/scratchpad/s1-1c-2-research/vuln-canary
Toolchain: pinned go1.27.3 darwin/arm64, GOTOOLCHAIN=local, private GOMODCACHE=$S/modcache. Repo untouched (worked in `git clone` at $S/cep, HEAD fac4e64).
"Offline" runs used GOPROXY=off plus `sandbox-exec -p '(version 1)(allow default)(deny network-outbound (remote ip))'` (proved: curl under it -> "Couldn't connect", exit 7).

## G1. golang.org/x/vuln
- Latest: v1.9.0. `go list -m -versions golang.org/x/vuln` -> "... v1.8.0 v1.9.0".
- Licence: BSD-3-Clause ("Copyright 2009 The Go Authors ... Neither the name of Google LLC ...") from $S/modcache/golang.org/x/vuln@v1.9.0/LICENSE.
- In a copy of gate/proofs ($S/g1), `go get -tool golang.org/x/vuln/cmd/govulncheck@latest` -> exit 0:
  "go: upgraded golang.org/x/telemetry v0.0.0-20261009151841-97e8ea77aa2a => v0.0.0-20261010193011-f71dc92da45e"; "go: added golang.org/x/vuln v1.9.0".
- go.mod diff: the single `tool` line became a `tool ( apidiff ; govulncheck )` block (the comment moved inside it); goleak moved into the main require block; new indirect block:
  golang.org/x/sys v0.49.0 // indirect; golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect; golang.org/x/vuln v1.9.0 // indirect.
- New `go mod graph` nodes (5): github.com/google/go-cmdtest@v0.4.1-0.20220921163831-55ab3332a786 (Apache-2.0), github.com/google/renameio@v0.1.0 (Apache-2.0), golang.org/x/telemetry@v0.0.0-20261010193011-f71dc92da45e (BSD-3), golang.org/x/tools@v0.52.0 (graph-only; selected stays the pinned pseudo-version), golang.org/x/vuln@v1.9.0. x/sys v0.49.0 was already a node (now a direct indirect require). Licences read from LICENSE files in the module cache.
- `go list -m all` diff: +go-cmdtest, +renameio, +x/vuln v1.9.0, telemetry 20261009151841 -> 20261010193011.
- Cross-module skew: root `go list -m all` vs proofs+govulncheck differ in exactly one module: [('golang.org/x/telemetry', 'v0.0.0-20261009151841-97e8ea77aa2a', 'v0.0.0-20261010193011-f71dc92da45e')]. x/vuln v1.9.0's go.mod requires telemetry 20261010193011 (v1.8.0 requires older mod/sync/tools/telemetry). So the "one version across the repository's modules" rule needs root go.mod to add `golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect` (same pattern as x/mod, x/tools), or the row tolerates it.
- Builds with go1.27.3: `go build golang.org/x/vuln/cmd/govulncheck` build_exit=0; `govulncheck -version` -> "Go: go1.27.3 / Scanner: govulncheck@v1.9.0". `go version -m` on the binary: links only x/vuln, x/mod v0.42.0, x/sync v0.24.0, x/telemetry 20261010193011, x/tools (pinned pseudo). go-cmdtest/renameio are graph-only (not linked).
- `go tool govulncheck -C <dir>` from the proofs module works offline (exit 0 on the cep clone).
- Note: `govulncheck -version` with the default DB contacts vuln.go.dev (prints "DB updated: ..."); with -db file:// it does not.

## G2. Offline DB
- vulndb.zip is real and documented. HEAD https://vuln.go.dev/vulndb.zip -> "HTTP/2 200", "content-type: application/zip", "content-length: 3599175", "last-modified: Wed, 11 Nov 2026 11:15:14 GMT", "cache-control: public, max-age=3600". Downloaded sha256 today 714ff02d9533ec6ca269e565b61b27cbd959aa0af89d51f318261322e875861c (changes as the DB changes; no published checksum found).
- Primary source https://go.dev/doc/security/vuln/database: "Bulk download ... a zip file containing all the index and OSV files is available at vuln.go.dev/vulndb.zip". Endpoints: /index/db.json[.gz] ({"modified"}), /index/modules.json[.gz], /index/vulns.json[.gz], /ID/$id.json[.gz]; "a site serving from a fixed file system (including a file:// URL) can implement this API".
- The zip unpacks directly (no top folder) to exactly that layout: `ID/` (4801 files, e.g. GO-2020-0001.json), `index/db.json` ({"modified":"2026-11-11T02:51:11Z"}), `index/modules.json`, `index/vulns.json`; 4804 .json files, no .gz.
- Source (x/vuln@v1.9.0 internal/client/client.go): file:// client stats the dir, uses v1 layout if `index/modules.json` exists; else falls back to a "hidden" flat-dir-of-OSV-files mode; local reads are `<endpoint>.json` (uncompressed).
- Proof on the cep clone, network blocked, GOPROXY=off, GOFLAGS=-mod=readonly, cache warmed by `go mod download all`:
  `govulncheck -db file://$S/dl/vulndb ./...` -> "No vulnerabilities found ... 1 vulnerability in modules you require, but your code doesn't appear to call" root exit=0; gate/proofs exit=0.
  `-show verbose`: "Vulnerability #1: GO-2026-6859 HTTP/1 server crash due to MaxBytesReader header race in net/http ... Found in: stdlib@go1.27.3  Fixed in: stdlib@go1.27.4".
  `-scan module` on root -> exit=3 (same GO-2026-6859). `-scan package` -> exit=0.
  `-version` with file DB -> "DB: file:///... DB updated: 2026-11-11 02:51:11 +0000 UTC" exit 0 (from index/db.json; the line is omitted when db.json is absent).

## G3. Planted vulnerability
- Fixture DB $S/g3/db: index/db.json, index/modules.json ([{"path":"example.com/planted","vulns":[{"id":"GO-0000-0001",...,"fixed":"1.0.1"}]}]), ID/GO-0000-0001.json (OSV: affected example.com/planted, SEMVER introduced 0 fixed 1.0.1, ecosystem_specific.imports[{path:"example.com/planted",symbols:["Bad"]}]). Module example.com/planted has Bad() and Good().
- `replace example.com/planted => ../planted` HIDES it: called/notcalled/fixed at symbol, package and module levels all "No vulnerabilities found." exit 0. Cause (internal/vulncheck/fetch.go:20): `if mod.Replace != nil { modPath = mod.Replace.Path }` -> DB is queried for "../planted".
- Works offline with a local file proxy: $S/g3/proxy/example.com/planted/@v/{list,v1.0.0.info,.mod,.zip,v1.0.1.*} (zip entries prefixed example.com/planted@vX/). `GOPROXY=file://$S/g3/proxy GONOSUMDB=example.com/planted go mod tidy` (network blocked) wrote go.sum; then GOPROXY=off GOFLAGS=-mod=readonly:
  | consumer | symbol | package | module |
  | called (v1.0.0, calls Bad) | 3 | 3 | 3 |
  | not called (v1.0.0, calls Good) | 0 | 3 | 3 |
  | not vulnerable (v1.0.1, calls Bad) | 0 | 0 | 0 |
  Symbol-level lines: "Vulnerability #1: GO-0000-0001", "Found in: example.com/planted@v1.0.0", "Fixed in: example.com/planted@v1.0.1", "#1: main.go:5:34: consumer.main calls planted.Bad", "Your code is affected by 1 vulnerability from 1 module."
- Simpler static fixture: a vendored consumer (`go mod vendor` -> vendor/modules.txt "# example.com/planted v1.0.0" + vendor/example.com/planted/planted.go) is reported with an EMPTY module cache: exit 3 (binary), exit 3 via `go tool govulncheck -C <fixture>` from the proofs module, exit 3 with -scan module. Catch: GOFLAGS=-mod=readonly overrides vendor auto-detect -> "module lookup disabled by GOPROXY=off", exit 1; the row must not force -mod=readonly for the fixture (use -mod=vendor or no -mod).
- A flat directory holding only GO-0000-0001.json (no index/, no ID/) also works: exit 3.

## G4. Exit codes and DB failures
- Source internal/scan/errors.go: vulnerabilities found = 3, usage = 2, help = 0, other errors = 1 (cmd/govulncheck/main.go os.Exit(1)). Docs (pkg.go.dev/golang.org/x/vuln/cmd/govulncheck "Exit codes"): 0 when none found; "also exits successfully if ... -json, -format sarif, or -format openvex is provided, regardless of the number of detected vulnerabilities". Proved: planted+called with -format json/sarif/openvex -> exit 0 (json has 3 "finding" messages).
- -db file:// to a missing dir -> exit 1 "creating client: stat ...: no such file or directory".
- -db https://vuln.go.dev, network blocked -> exit 1 "fetching vulnerabilities: Get \"https://vuln.go.dev/index/modules.json.gz\": dial tcp ...: connect: operation not permitted".
- -db https://unreachable.invalid -> exit 1 "creating client: unrecognized vulndb format; see https://go.dev/security/vuln/database#api" (misleading: HEAD failure looks like a schema error).
- -db without scheme -> exit 1 "source \"/relative/no/scheme\" has unsupported scheme". Bad flag -> exit 2. Package load error -> exit 1.
- FALSE GREENS: an EMPTY directory as DB -> exit 0 "No vulnerabilities found." An index/modules.json of [] -> exit 0. A zip unpacked one level too deep -> exit 1 (unrecognized format). index present but ID file missing -> exit 1 "open ID/GO-0000-0001.json: no such file". index/db.json absent -> scan still works.

## G5. Canary "latest" sources (unauthenticated, today)
- Go: https://go.dev/dl/?mode=json -> list of {version, stable, files[]}; lists ['go1.27.4','go1.26.11']. Pick first stable; file fields: filename, os, arch, kind ("archive"), version, sha256, size. Today:
  go1.27.4.darwin-arm64.tar.gz sha256 91a44a2b8a716600ad94a4f143e156116620db8828086a6e3602291be9bd0bb4; go1.27.4.linux-amd64.tar.gz sha256 7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840. URL = https://go.dev/dl/<filename>. (Pinned 1.27.3 is behind; 1.27.4 fixes GO-2026-6859.)
- GitHub: GET https://api.github.com/repos/{owner}/{repo}/releases/latest -> tag_name, assets[].{name, browser_download_url, digest}. `digest` is "sha256:<hex>" per asset (cross-check: golangci-lint darwin-arm64 digest d1e40948...47d5 equals tools.json archive_sha256).
  golangci/golangci-lint: v2.15.0 (= pinned); assets golangci-lint-2.15.0-{darwin-arm64,linux-amd64}.tar.gz (+ golangci-lint-2.15.0-checksums.txt).
  astral-sh/ruff: 0.17.4 (pinned 0.17.3); ruff-aarch64-apple-darwin.tar.gz digest sha256:6ef8b3a7..f17af, ruff-x86_64-unknown-linux-gnu.tar.gz sha256:3ab2b34a..cb27a; also per-asset .sha256 files and sha256.sum.
  astral-sh/python-build-standalone: 20261110 (pinned 20261026); cpython-3.15.2+20261110-{aarch64-apple-darwin,x86_64-unknown-linux-gnu}-install_only.tar.gz, cpython-3.12.16+20261110-...; also SHA256SUMS; 991 assets. Asset `name` has "+", browser_download_url has "%2B" (as tools.json URLs do).
- PyPI: https://pypi.org/pypi/lark/json -> info.version "1.3.1" (= pinned); urls[] with packagetype "sdist": filename lark-1.3.1.tar.gz, url https://files.pythonhosted.org/packages/da/34/.../lark-1.3.1.tar.gz, digests.sha256 b426a7a6...0d2905 (= tools.json), yanked false. info.license "MIT".
- Rate limits: response headers "x-ratelimit-limit: 60 ... x-ratelimit-resource: core"; docs (https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api): "The primary rate limit for unauthenticated requests is 60 requests per hour" (per IP; shared runner IPs) and "The rate limit for GITHUB_TOKEN is 1,000 requests per hour per repository." Canary needs 3 GitHub calls -> pass GITHUB_TOKEN.

## G6. GitHub Actions (docs only; not tested, testing needs a push)
- https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow: "To trigger the workflow_dispatch event, your workflow must be in the default branch." and "To run a workflow on a branch other than the repository's default branch, use the --ref flag. gh workflow run WORKFLOW --ref BRANCH".
- https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows (workflow_dispatch): "the 'Run workflow' button will be present if the workflow file exists on the default branch. Once a workflow has run at least once, you can dispatch it against any branch or tag via the GitHub API or GitHub CLI." So a canary file only on a branch is dispatchable only after it has run once (e.g. via its own push/pull_request trigger); unproven in practice.
- schedule (same page): "This event will only trigger a workflow run if the workflow file exists on the default branch. Scheduled workflows will only run on the default branch. In a public repository, scheduled workflows are automatically disabled when no repository activity has occurred in 60 days."
- Notifications (same page): "Notifications for scheduled workflows are sent to the user who last modified the cron syntax in the workflow file." https://docs.github.com/en/subscriptions-and-notifications/how-tos/managing-github-actions-notifications: under Settings > Notifications > System > Actions, choose "Email" and optionally "Only notify for failed workflows" (the dropdown is described starting from "Don't notify"). Whether the owner's account emails today: unproven - owner to check the setting.

## G7. go mod graph vs go list -m all (clone of cep, GOPROXY=off)
- Root: 23 graph "modules" vs 21 listed. Multi-version: golang.org/x/mod [v0.41.0, v0.42.0] -> selected v0.42.0; golang.org/x/tools [v0.51.0, v0.52.1-0.20261106145633-d407941a339a] -> selected pseudo; plus pseudo-nodes go@[1.26.0,1.27.0], toolchain@[go1.27.0,go1.27.3].
- gate/proofs: 21 vs 19. Multi-version: golang.org/x/tools [v0.51.0, pinned pseudo] -> selected pseudo; plus go@ and toolchain@.
- In graph but not in `go list -m all`: only the `go` and `toolchain` pseudo-modules. In list but not graph: none. Every selected version is a graph node.
- So yes: `go list -m all` gives exactly one selected version for every real module in `go mod graph`; the extra versions are just requirement edges MVS outvoted. A "graph-only" check can be: graph nodes minus {go, toolchain}, grouped by path, with the selected version taken from `go list -m all`.

## What this means for the plan
1. Put govulncheck in gate/proofs (x/vuln v1.9.0, BSD; adds go-cmdtest and renameio (Apache-2, graph-only) plus telemetry/sys); root go.mod must also take telemetry 20261010193011 // indirect or the one-version rule goes red.
2. The nightly row can be fully offline: download vulndb.zip in CI setup (it unpacks straight to the v1 layout), run with -db file://; the row must check the DB is real (index/modules.json present, non-empty, index/db.json modified readable), because an empty dir or `[]` index gives a silent exit 0.
3. Proof fixture: never use `replace` (it hides the module); use a vendored consumer (no -mod=readonly) or a local file GOPROXY; table of exit codes is 3/0/0 at symbol level, 3/3/0 at package and module level; parse text exit codes, not -format json (always 0).
4. Decide the scan level: default (symbol) is 0 on main today; -scan module is 3 today because go1.27.4 fixes GO-2026-6859 in net/http - the canary will see go1.27.4 now.
5. Canary sources are all unauthenticated JSON with per-asset sha256 (go.dev files[].sha256, GitHub assets[].digest, PyPI digests.sha256); use GITHUB_TOKEN (1,000/h vs 60/h); schedule runs only from main, and workflow_dispatch of a branch-only file needs one prior run - the first proving run likely needs the file on main or a push/pull_request trigger.
