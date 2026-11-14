# S1.1c-5 — design note (v1 draft, 2026-11-14; for the blind architecture and QA reviews before G1)

*Step 1 of the full loop for S1.1c-5, from the answered [plan check](s1-1c-5-plan-check.md) (v2) and the [owner's ten answers of 2026-11-14](decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check). A build-ready spec: every path, signature, data format, failure text, test and step a lane needs. Code is cited as `file:line` on `s1-1c-5/integration` @ bda6554. "M-n" marks a measurement made for this note (§0; scratch `s1-1c-5-scratch/design-exp/`, pinned golangci-lint v2.15.0, go1.27.4, ruff 0.17.5). "(inferred)" marks a claim not measured or not read in code. Nothing here re-decides an owner answer; where the code makes an answer unbuildable as worded, it is a **question for the owner at G1** (§10), recommendation first.*

## 0. Measured before G1

- **M-1 — `.golangci.json` cannot configure revive's two size rules (blocks decision 10's confirm as worded).** golangci-lint v2.15.0 decodes JSON numbers as float64; revive v1.17.1 requires int64 for `argument-limit`'s argument and `file-length-limit`'s `max`. With the research's config converted to JSON, `config verify` exits 0, but `run` exits 1, `Report.Error` = `setup revive: cannot configure rule: "argument-limit": invalid value passed as argument number to the "argument-limit" rule`, and **revive reports nothing** while every other linter still reports (16 issues). The forms `[5]`, `[5.0]`, `["5"]`, `[{"max": 5}]` all fail; `file-length-limit` alone fails with `need positive int64 but got float64`. The same document as **TOML** (`golangci.toml`) verifies, runs, and reports both (`p.go:3 argument-limit`, `p.go:501 file-length-limit`; a 500-line file and a five-parameter function are green). → §10 Q-A.
- **M-2 — the output channel.** With `output.show-stats = false`, no `output.formats` in the config and `--output.json.path=<absolute file>` on the command line, stdout is empty (0 bytes) and the file holds `{"Issues": [...], "Report": {"Linters": [...], "Error"?: str, "Warnings"?: [...]}}`. A revive setup failure still exits 1 with issues present: exit status alone cannot tell it from findings, so `Report.Error` must be read.
- **M-3 — forbidigo's bypasses.** With `analyze-types = true`, pattern `^(fmt\.Print.*|print|println)$` reports the builtins `print` and `println`; `^os\.(Getenv|LookupEnv|Environ)$` reports all three. `import _ "unsafe"` plus `//go:linkname` gives **no** forbidigo finding (QA's PQ-B4 reproduced). `exclude-godoc-examples = false` reports `panic` and `os.Getenv` inside `func ExampleH` (the `[reflect]` there is dropped by the tests' reflect rule, as decided).
- **M-4 — errcheck's built-in exclusions, written out.** The 25 symbols in §2.1's `exclude-functions`, with `disable-default-exclusions = true`, give exactly the findings of errcheck v1.30.0's defaults on a plant of all 25 (3 = 3, identical sets); with the defaults off and no list, 27 more are reported (every planted site). Residue: a default this plant does not exercise would be missed — W0's real-tree comparison (§8, T-A12) closes it.
- **M-5 — ruff 0.17.5 reproduces every 0.17.3 hole.** The research's eight `rufftest` variants give the same format summaries and check results on 0.17.5 (base 3/2 errors; `exclude`, `extend-exclude`, `[lint] exclude` without force: no effect; `force-exclude`+`extend-exclude`: 1 file / passes; `force-exclude`+`[lint] exclude`: 3 / **passes**; `select = ["E","W","F"]`: 3 / **passes**; `force-exclude`+`[format] exclude`: 2 / 2 errors). QA's three keys (`builtins`, `lint.dummy-variable-rgx = ".*"`, `[lint.pycodestyle] max-line-length = 400`) together hide F821, F841 and E501: "All checks passed!" on 0.17.5 (base: 3 errors).
- **M-6 — the tree's unclaimed Go files.** 269 tracked `.go` files, 260 outside `gate/proofs/`. Four sit under `testdata/`, which `go list ./...` never claims: `tools/apipartition/testdata/embedded/embedded.go`, `…/fixture/fixture.go`, `…/fixture/fixture_test.go`, `…/fixture_reflow/fixture.go`. No `_`/`.` directory, no build constraint, no GOOS/GOARCH file suffix, no nested `go.mod` beyond `gate/proofs/go.mod`. `tools/allowlists/generated.json` is empty.
- **M-7 — `unsafe` is already archcheck's, outside `tools/`.** `tools/archcheck/main.go:565-566` reds any `import "unsafe"` and `:575-579` any `//go:linkname`, but the `archcheck` row's `expected_scope` excludes `gate/**` and `tools/**` (`gate/registry.json`, row `archcheck`). So in `tools/` neither row sees a blank `unsafe` import today. The Go compiler requires `import _ "unsafe"` for `//go:linkname` (inferred from the Go spec's linkname rule), so an import check covers both.
- **Not measured here (W0 measures, §8):** the linux-amd64 golangci-lint binary's build information (`go version -m` reads an ELF file on darwin; W0 fetches the pinned linux archive into scratch, checks its sha256 against `tools.json`, and records the `go` line, the `mod` line and the dep count); the directive census beyond revive, gosec, gocyclo and nolint (§4.1); the row's T0 and T1 times (§2.7); the real tree's counts under the §2.1 config.

## 1. Lanes, waves, ownership (≤ 2 agents at once)

| Wave | Lane (agent) | Owns (one owner per file per wave) | Exit |
|---|---|---|---|
| G0 (done, bda6554) | coordinator | `gate/current-sprint` | — |
| W0a | coordinator (law commit, before QA) | `gate/sprints.json` and the plan's §3 table (S1.1c-5…S1.1c-8 printed, §3.4); `gate/known-issues.json` + `docs/product/known-issues.md` (the four new issues, §3.3); `gate/law.json` paths (+ `.golangci.toml`, §2.6); `tools/allowlists/smells.json` (empty, rule `smells`); `allowlists.RULES` += `smells` is **lane B's** (PA-S13), so W0a leaves the file absent and lane B adds it with RULES | T2 green |
| W0b | QA (qa-expert) | every test module named in §8 (new files only, plus the restored block in `test_breakit_s11c2.py`); `gate/proofs/drivers/test_smells_planted.py`; the census data file `gate/runner/testdata/directives.json` | each test red on the stubs for its stated reason; census and the linux build info recorded in the report |
| W0c | coordinator | the stubs (§2–§6 signatures, `raise NotImplementedError`), so QA's tests import | QA's modules import |
| W1 | **lane A** (python-engineer): the row | `.golangci.toml`; `gate/runner/golangci.py` (new); `gate/runner/rows/smells.py` (new); `gate/registry.json` + `gate/law.json` instrument `smells`; `gate/ci/generate.py` only if §2.8's check needs it | A's tests green; T2 green with `smells` **red on the real tree** (expected until W2; the lane's report quotes the counts) |
| W1 | **lane B** (python-engineer): waivers, checker, ruff pin, parts | `gate/runner/suppressions.py`; `gate/runner/rows/suppressions.py`; `gate/runner/allowlists.py` (`RULES`); `gate/runner/sprintid.py` (`Order.reached_part`); `gate/runner/parts.py` (new); `gate/runner/tests/test_breakit_s11c2.py` (restore only) | B's tests green; T2 green except `smells` |
| W2 | **lane C** (go-engineer + python for the scan; one agent): the tool scan, then the whole-tree fixes | `tools.json` (`go_built`); `gate/toolchain/manifest.py`; `gate/canary/candidate.py` (carry the field); `gate/runner/vulndb.py` (`findings`' `pseudo`); `gate/runner/rows/vulns.py`; `tools/allowlists/vulns.json`; then every non-test and test `.go` file in §7.1 **outside lane D's four packages** | C's tests green; `vulns` green; `smells` green on everything but the size family |
| W2 | **lane D** (go-engineer): calibration | `internal/table/**`, `internal/clock/**`, `io/inmemory/**`, `tools/shape/**` (incl. their test files for the forbidigo/errcheck fixes in them); `tools/astequal/**` (new, §7.4); its calibration record lines (§7.6) | the four packages clean for the whole set, their `smells.json` entries never written; AST-equality and benchmarks as §7 |
| G1→T3 | coordinator | `tools/allowlists/smells.json` entries (§3.1, written from lane A's real-tree counts after W2), DEVELOPMENT.md, ADR-0015 annotations (§9), the record | T3, then T4 |

Order inside W2: lane C's scan commit first (it touches no Go file lane D owns), then C's whole-tree fixes. Lane C never edits a file under lane D's four directories; a whole-tree finding there is D's.

## 2. The `smells` row (lane A)

### 2.1 The config, `.golangci.toml` (subject to §10 Q-A; if the owner keeps JSON, revive cannot run and the file-length and params rules have no linter)

The whole document, in this key order (the pin compares parsed values, so order and comments are free):

```toml
# The smells row's law (ADR-0015 §2; owner 2026-11-14, S1.1c-5's plan check). Pinned whole by gate/runner/golangci.py.
version = "2"

[run]
relative-path-mode = "gomod"
tests = true
build-tags = []
modules-download-mode = "readonly"

[output]
show-stats = false

[issues]
max-issues-per-linter = 0
max-same-issues = 0
uniq-by-line = false
new = false
fix = false

[severity]
default = "error"

[formatters]
enable = []

[linters]
default = "none"
enable = ["errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive"]

[linters.settings.errcheck]
check-type-assertions = false
check-blank = false
disable-default-exclusions = true
exclude-functions = [
  "(*bytes.Buffer).Write", "(*bytes.Buffer).WriteByte", "(*bytes.Buffer).WriteRune", "(*bytes.Buffer).WriteString",
  "fmt.Print", "fmt.Printf", "fmt.Println",
  "fmt.Fprint(*bytes.Buffer)", "fmt.Fprintf(*bytes.Buffer)", "fmt.Fprintln(*bytes.Buffer)",
  "fmt.Fprint(*strings.Builder)", "fmt.Fprintf(*strings.Builder)", "fmt.Fprintln(*strings.Builder)",
  "fmt.Fprint(os.Stderr)", "fmt.Fprintf(os.Stderr)", "fmt.Fprintln(os.Stderr)",
  "(*io.PipeReader).CloseWithError", "(*io.PipeWriter).CloseWithError",
  "math/rand.Read", "(*math/rand.Rand).Read",
  "(*strings.Builder).Write", "(*strings.Builder).WriteByte", "(*strings.Builder).WriteRune", "(*strings.Builder).WriteString",
  "(hash.Hash).Write",
]

[linters.settings.forbidigo]
analyze-types = true
exclude-godoc-examples = false
forbid = [
  { pattern = '^(fmt\.Print.*|print|println)$', msg = "[fmtprint]" },
  { pattern = '^panic$', msg = "[panic]" },
  { pattern = '^reflect\..*$', msg = "[reflect]" },
  { pattern = '^unsafe\..*$', msg = "[unsafe]" },
  { pattern = '^os\.(Getenv|LookupEnv|Environ)$', msg = "[getenv]" },
]

[linters.settings.funlen]
lines = 60
statements = 100000      # -1 switches funlen off in v2.15 (RM); the ADR names lines only
ignore-comments = false  # decision 10: funlen counts comment lines

[linters.settings.gocritic]

[linters.settings.gocyclo]
min-complexity = 10      # reports > 10

[linters.settings.lll]
line-length = 120
tab-width = 1            # decision 10

[linters.settings.nolintlint]
require-explanation = true
require-specific = true
allow-unused = false
allow-no-explanation = []

[linters.settings.revive]
enable-all-rules = false
[[linters.settings.revive.rules]]
name = "file-length-limit"
severity = "error"
arguments = [{ max = 500, skipComments = false, skipBlankLines = false }]
[[linters.settings.revive.rules]]
name = "argument-limit"
severity = "error"
arguments = [5]

[linters.exclusions]
generated = "disable"
warn-unused = false
presets = []
paths = []
paths-except = []
[[linters.exclusions.rules]]   # §2.1 and decision 4: tests exempt from the size family and lll
path = '_test\.go$'
linters = ["funlen", "gocyclo", "lll", "revive"]
[[linters.exclusions.rules]]   # decision 4: tests exempt from the reflect pattern
path = '_test\.go$'
linters = ["forbidigo"]
text = '\[reflect\]'
[[linters.exclusions.rules]]   # ADR §2 smells: reflect only in L0–L2 (ARCHITECTURE's layer charter)
path-except = '^internal/(event|model|metrics|clock|pattern|window|expr|text|table|partition)/'
linters = ["forbidigo"]
text = '\[reflect\]'
[[linters.exclusions.rules]]   # ADR §2 smells: os.Getenv allowed in cmd and io
path = '^(cmd|io)/'
linters = ["forbidigo"]
text = '\[getenv\]'
```

Notes for lane A:
- `[fmtprint]` is **not** excluded in the config: decision 10 allows `fmt.Print*` in package-main tools, which a path regex cannot express. The row drops a `[fmtprint]` finding only in a file under `tools/` whose package clause is `main` (§2.3 step 6), so a non-main package under `tools/` stays red (QA's Q9 twin).
- The `[getenv]` pattern widens the ADR's `os.Getenv` to `LookupEnv` and `Environ` (the card's acceptance line 1 names `os.LookupEnv`; `os.Environ` is the same smell, zero uses today). `log.Print*` is not added — §10 R-3.
- `warn-unused = false` is written: `true` would warn (and so red) on the `^(cmd|io)/` rule while no Getenv exists there (inferred from golangci's documented behaviour; not measured).
- An unread key in a golangci section is silently accepted by `config verify` (inferred) — irrelevant: the whole-document pin refuses any key not written above.

### 2.2 The whole-document pin and the shared reader (`gate/runner/golangci.py`, new, lane A; the only reader of the config — lane B imports it)

```python
CONFIG = ".golangci.toml"
LINTERS = ("errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive")
SIZE_RULES = ("funlen", "gocyclo", "file-length-limit", "argument-limit")  # the size family, as smells.json names them
PINNED: dict[str, object]  # the §2.1 document as tomllib parses it, written as a Python literal

class ConfigError(ValueError): ...

def load(text: str) -> dict[str, object]                 # tomllib.loads; a TOMLDecodeError → ConfigError(f"{CONFIG}: {e}")
def differences(doc: Mapping[str, object]) -> list[str]  # [] when doc == PINNED; else one line per key path, sorted:
    #   "<path>: <got> ≠ pinned <want>", "<path>: not pinned (remove it)", "<path>: missing (pinned <want>)"
    #   <path> joins keys with ".", quoting a key that holds "." or a space; a list compares whole (order counts)
def enabled(doc: Mapping[str, object]) -> frozenset[str] # doc["linters"]["enable"] as a set; ConfigError if absent
def rule_of(issue: Mapping[str, object]) -> str          # "funlen" | "gocyclo" | "file-length-limit" | "argument-limit"
    # | the linter name (errcheck, forbidigo, gocritic, lll, nolintlint, typecheck) — revive's rule from Text's prefix
    # "<rule>: " (M-1's texts); a revive Text with another prefix → "revive"
def read_report(path: str) -> Report                      # the JSON file; ValueError on a malformed document
@dataclass(frozen=True) class Issue: linter: str; rule: str; text: str; file: str; line: int; column: int
@dataclass(frozen=True) class Report: issues: tuple[Issue, ...]; enabled: frozenset[str]; error: str; warnings: tuple[str, ...]
```
- `read_report` reads `Issues[*].FromLinter`, `.Text`, `.Pos.Filename` (module-relative, M-2), `.Pos.Line`, `.Pos.Column`; `Report.Linters[*]` with `Enabled: true` → `enabled`; `Report.Error` (string, absent = ""); `Report.Warnings[*].Text` (inferred shape `{"Tag","Text"}`; W0 plants a warning and pins it). Unknown top-level keys are ignored; a missing `Issues` (null) is an empty tuple.
- The pin reads only `CONFIG`. `PINNED` lives in code, so a config edit without the code edit is red, and both files are law paths (§2.6).

### 2.3 The row wrapper (`gate/runner/rows/smells.py`, new, lane A)

`main(argv) -> int`; argv[1] is `{files}` (the files list; at T0 the staged scope, §2.7). Steps, all faults collected, each printed `failed: smells: <text>`:
1. `golangci.load` + `golangci.differences` → each line a fault; any fault here stops the run (no lint against a wrong config).
2. **The covered count** (§2.4) under `GOOS=linux` and `GOOS=darwin` (both `GOARCH` as `gate/runner/env.py`'s pins give; `CGO_ENABLED=0`, inferred from `rows/gomods.py`'s ENV, which W1 reuses).
3. **Lint**, once per GOOS in `PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))`: `golangci-lint run --config .golangci.toml --output.json.path=<row_dir>/smells-<goos>.json ./...` with `GOOS`, `GOARCH` set, cwd the repo root, stdout and stderr to `<row_dir>/smells-<goos>.stdout|.stderr`. (`./...` from the root never enters `gate/proofs`, a nested module.)
4. **Exit agreement** per run: exit 0 ⇔ no issue and no `Report.Error`; exit 1 ⇔ at least one issue or a `Report.Error`; any other exit (3 = config error) → `golangci-lint (<goos>) exit <n>: <first stderr line>`; a missing or unreadable JSON file → `golangci-lint (<goos>) wrote no report`; exit 0 with issues, or exit 1 with neither issues nor error → `golangci-lint (<goos>): exit <n> disagrees with <k> issues`.
5. **Report health**: `Report.Error` → `golangci-lint (<goos>): <error>`; each warning → `golangci-lint (<goos>) warns: <text>`; `Report.enabled != set(LINTERS) | {"typecheck"}` (inferred that typecheck is listed enabled; W0 pins it) → `golangci-lint (<goos>) ran <sorted> not <sorted>`; any issue with `linter == "typecheck"` → red whatever its file (PQ-S3).
6. **Merge** the two runs' issues by `(file, line, column, linter, text)` (a finding on both GOOS counts once). Then, in order: (a) drop every issue whose `file` is a `generated.json` target; (b) drop a `[fmtprint]` forbidigo issue whose file is under `tools/` and whose package clause is `main` (`golangci.package_clause(source) -> str`, a scan reusing `suppressions.go_comments`' state machine to skip comments, then `package <ident>`); (c) apply the size-family exclusions (§3.1): drop an issue with `rule_of in SIZE_RULES` whose file's directory (`posixpath.dirname`) equals an entry's package and whose rule equals the entry's rule, **and the entry is live** (not expired at T4/nightly, `allowlists.RED_EXPIRED`); record each entry that dropped ≥ 1.
7. **Unsafe**: from §2.4's `go list` data, any package whose `Imports`, `TestImports` or `XTestImports` holds `unsafe` → `<import path>: imports unsafe (banned everywhere, ADR-0015 §2 smells; //go:linkname needs it)`. No waiver (M-7).
8. **Liveness of exclusions**: each `smells.json` size entry that dropped nothing → `tools/allowlists/smells.json: <target> waives nothing: the package is clean for <rule>; remove it`. Judged only when both runs completed with no fault of steps 1–5 (a partial run proves nothing; the `vulns` row's `complete` rule, `rows/vulns.py:276-278`).
9. Each kept issue → `<file>:<line>:<col>: <linter>[/<rule>]: <text>`, sorted. `rows.covered(<covered>)`; `rows.finish(faults)`.

Reason (green): the runner prints `covered <n>`; the row also prints one plain line (not `failed:`) `smells: <n> issues dropped by <m> exclusions (<pkg> <rule> <k>, …); linux and darwin` so the debt stays loud in the record (PA-B2). (inferred: the summary shows the first failure only; the record keeps stdout.)

### 2.4 Covered from `go list` (lane A; `golangci.covered`)

```python
def go_files(listing: Sequence[Mapping[str, object]], root: str) -> tuple[dict[str, str], list[str]]
    # file → import path for every GoFiles, CgoFiles, TestGoFiles, XTestGoFiles entry (Dir joined, made root-relative);
    # faults: a file claimed twice in one listing; a package with Error → "<import path>: <Err's first line>"
def covered(tracked: Sequence[str], linux: Mapping[str, str], darwin: Mapping[str, str], unlinted: frozenset[str],
            gomods: Sequence[str]) -> tuple[int, list[str]]
```
- `listing` is `go list -e -json ./...` (no `-test`, so a file sits in one package entry) under each GOOS, decoded with `jsonstream.parse` (`gate/runner/jsonstream.py`).
- `tracked` = `git ls-files -z -- '*.go' ':(exclude)gate/proofs/**'` (the row's own call, as `rows/suppressions.py:34-39`).
- A tracked file is **accounted for** when it is in `linux ∪ darwin`, or it is an `unlinted` target (§3.1's PERMANENT `unlinted <path>` entries). Exactly once: a file both linted and listed `unlinted` → `<path>: linted and listed unlinted: remove the entry`; an `unlinted` target not tracked → stale.
- Red, each named: a tracked file in neither set nor listed → `<path>: no package under linux or darwin claims it (testdata/, _dir/, //go:build ignore, a nested module): list it or move it`; a file in `IgnoredGoFiles` under **both** GOOS → the same text with `excluded by build constraints under linux and darwin`; any tracked `go.mod` other than `go.mod` and `gate/proofs/go.mod` (from `gomods`, the row's `git ls-files -z -- go.mod ':(glob)**/go.mod'`) → `<path>: a nested module is never linted`.
- `covered` = the number of tracked files accounted for (union, so a `_linux.go` file counts once). The registry's `expected_scope` is `["git", ":(glob)**/*.go", ":(glob,exclude)gate/proofs/**"]`, so `covered == expected` (`gate/runner/verdict.py:57`) holds exactly when every file is accounted for.
- Today: 260 tracked, 256 claimed, 4 `unlinted` (M-6). At G0+W0a with no entries the row is red naming the four, by design until lane A's commit lists them.

### 2.5 Registry row and `gate/law.json` instrument (copy `suppressions`' shapes, `gate/registry.json` and `gate/law.json:592-625`)

```json
{"id": "smells", "job": "lint", "tiers": ["T0", "T1", "T2", "T3", "T4"], "blocking": true, "timeout_s": 600,
 "needs": [], "scope": ["prod", "test", "tools", "generated"],
 "expected_scope": ["git", ":(glob)**/*.go", ":(glob,exclude)gate/proofs/**"], "min_wants": 1,
 "lands_in": "S1.1c-5", "exclusive": false,
 "command": ["python3.15", "-I", "-B", "-X", "utf8", "gate/runner/rows/smells.py", "{files}"]}
```
```json
{"id": "smells", "row": "smells", "lands_in": "S1.1c-5",
 "code": ["gate/runner/allowlists.py", "gate/runner/golangci.py", "gate/runner/jsonstream.py",
          "gate/runner/rows/__init__.py", "gate/runner/rows/smells.py", "gate/runner/suppressions.py", ".golangci.toml"],
 "proofs": [{"path": "gate/proofs/drivers/test_smells_planted.py", "cases": <W0's count, §8.3>}],
 "tiers": ["T0", "T1", "T2", "T3", "T4"], "min_wants": 1,
 "expected_scope": ["git", ":(glob)**/*.go", ":(glob,exclude)gate/proofs/**"]}
```
- `timeout_s` 600 is a placeholder; W0 measures T2's time (two GOOS runs ≈ 2 × 17 s cold on darwin per RM, inferred) and lane A sets 4× the measured CI time, as S1.1c-4 did for `vulns`.
- The `suppressions` instrument's `code` gains `gate/runner/golangci.py` and `gate/runner/parts.py` (it imports both, §3, §4).

### 2.6 Law paths, CI

- `gate/law.json` `paths` += `{"path": ".golangci.toml", "kind": "code"}` (W0a; the `ruff.toml` precedent, `gate/law.json` paths). A semantic LAW Δ parser is backlog (card).
- CI: job `lint` (registry `job`), generated by `gate/ci/generate.py` from the registry (`_jobs`, `:219`) — no generator change (inferred: the generator reads every row's job; `make ci-workflow` regenerates `.github/workflows/gate.yml`, and `ci-fresh` reds a stale one). golangci-lint is already installed by `make setup tools` (it is a `tools.json` pin).

### 2.7 T0 (staged mode)

At T0 the runner passes only the staged `.go` files in scope (`gate/runner/scope.py:64-74`), and `expected` is their count. The row then (a) lints the **packages** of the staged files (`golangci-lint run … <dir>/...` is wrong: it recurses; the row passes each staged file's directory as `./<dir>`, deduplicated — golangci lints whole packages, inferred), (b) counts covered as the staged files accounted for. A size finding in an unstaged file of the same package is reported too (it is a real finding). The T0 time is measured in W0 on a one-file stage and on a ten-package stage; if T0 exceeds 10 s on darwin, §10 R-6 asks the reviewers whether T0 keeps `smells` (ADR §2 says "T0 (staged) / T1").

## 3. Exclusions and the part-level check (lane B; entries by the coordinator)

### 3.1 `tools/allowlists/smells.json` (rule `smells`, the one grammar, `gate/runner/allowlists.py:1-8`)

```json
{"schema": 1, "note": "…", "rule": "smells", "entries": [
  {"target": "internal/runtime gocyclo", "kind": "TEMP", "reason": "size family not yet cleaned; S1.1c-6 cleans it",
   "sprint": "S1.1c-5", "expiry": "2026-12-14", "issue": "S1.1c-5-1"},
  {"target": "unlinted tools/apipartition/testdata/fixture/fixture.go", "kind": "PERMANENT",
   "reason": "an apipartition fixture: data under testdata/, which go list never claims",
   "sprint": "S1.1c-5", "ref": "docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md#2-architecture-constraints-as-analyzers-with-teeth-reference-a-in-go"}
]}
```
- **Targets** (checked by the `smells` row; any other form → `smells.json: <target>: not "<package dir> <rule>" or "unlinted <path>"`):
  - `<package dir> <rule>`: the package's directory relative to the root (`.` for package `cep`), `<rule>` ∈ `golangci.SIZE_RULES`. One entry per package per size rule (PQ-B2's liveness by rule: a package clean for funlen but not gocyclo has funlen enter at once). Kind TEMP only (`smells.json: <target>: a size exclusion is TEMP, with an expiry and a known issue`); `issue` names a known issue whose `forward` is a printed part (§3.2).
  - `unlinted <path>`: PERMANENT only, for a tracked `.go` file no package claims (§2.4). Whether these four belong here or the row should skip `testdata/` by rule is §10 R-1.
- `expiry` is the grammar's (≤ 90 days ahead, `allowlists.py:173-182`); the part-level check is the real deadline, the expiry the grammar's backstop. The coordinator writes the entries from lane A's real-tree run after W2 (§1), one per (package, rule) with ≥ 1 finding, each forwarded per §3.3.
- "Waives nothing" is the `smells` row's step 8 (§2.3); the grammar faults are the `suppressions` row's (`allowlists.check`, `rows/suppressions.py:195-196`), unchanged.

### 3.2 `sprintid.Order.reached_part` (lane B; `gate/runner/sprintid.py`)

```python
def reached_part(self, part: str, marker: str) -> bool:
    """Whether the marker is at or past a printed part. `part` must be a printed row id that names a part
    (S<stage>.<part><letter>-<n>); ValueError "<part>: not a printed part of gate/sprints.json" otherwise. The marker
    is placed by `index`. A later position → True; an earlier one → False; the same position → True (the marker's
    printed id is the part). A part-less marker at the part's own row (S1.1c while S1.1c-6 is printed after it)
    sits at the row's position, before every printed part → False, never an error."""
```
- Positions are the printed order (`index`, `:65-72`), so `reached_part("S1.1c-6", "S1.1c-5")` False, `("S1.1c-6", "S1.1c-6")` True, `("S1.1c-6", "S1.1c-7")` True, `("S1.1c-6", "S4.1")` True, `("S1.1c-6", "S1.1c")` False, `("S1.1c-9", …)` ValueError.
- `reached` (by row, `:74-75`) and `passed` (`:77-87`) are unchanged; `notbuilt` keeps refusing parts (`notbuilt.py:60-61`) — its entries still name rows.

### 3.3 Known issues (coordinator, W0a; `gate/known-issues.json` + the table, `gate/runner/knownissues.py:1-14`)

One issue per later part, so each exclusion's `issue` carries its part: `S1.1c-5-1` → forward `S1.1c-6`, `S1.1c-5-2` → `S1.1c-7`, `S1.1c-5-3` → `S1.1c-8` (the size family's debt by part), and `S1.1c-5-4` (the tool scan's 15 advisories, forward `""`, §6.4). Each: `first_seen` `S1.1c-5`, `correctness` false, `opened` the commit's UTC day, `decision` `docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check` **only if** that section names the id as a whole word (`aging.decided`, `aging.py:32-37`) — it does not today, so `decision` is `""` unless the owner's G1 answers name them (§10 R-2). The table rows' Found cells begin `S1.1c-5`. Which package goes to which part is lane D's calibration output re-cut at close (decision 2): at W0a every size exclusion names `S1.1c-5-1` (forward S1.1c-6) except where the plan check's order already places it (runtime and validate S1.1c-6; archcheck after the `proofs` row, S1.1c-8 — inferred from decision 2's order; the coordinator confirms at G1).

### 3.4 Printing the parts (`gate/sprints.json`, the plan's §3 table; coordinator, W0a)

- The plan's §3 table gains four rows **after** the open `S1.1c` row: `S1.1c-5`, `S1.1c-6`, `S1.1c-7`, `S1.1c-8` (open), and `gate/sprints.json` the same four `{"id", "closed": false}` in the same place; `compare_rows` (`sprintid.py:185-192`) and `test_sprints_pin.py` keep them equal. The `S1.1c` row stays open until S1.1c-8 closes, so `first_open_mismatch` (`:195-199`; it compares `row_of(marker)` = `S1.1c`) stays green with marker `S1.1c-5` and no code change. S1.1c-1…S1.1c-4 are **not** printed (they would add four closed rows to aging rule 1's count; §10 R-2).
- What printing changes for each reader: `notbuilt` — nothing (rows only; `index("S1.1c-5")` now the part's own position, after `S1.1c`, still before `S4.1`, so `reached` is unchanged for every row entry). `aging` rule 1 — each printed part that closes is a closed row (`aging.py:46`): S3.3-2-1, -2, -3 (no decision) reach 3 closed rows after S3.3 at S1.1c-7's close and turn red unless decided (§10 R-2). `aging` rule 2 — a forward `S1.1c-6` is now a printed row: `_forward` takes the `rows` branch (`:57-58`) and reds only when the row S1.1c-6 is **closed** (marked ✓), not when the marker passes it. `allowlists.sprint_fault` (`:145-153`) — `sprint: S1.1c-5` resolves to its own row. `knownissues` — `first_seen`/`forward` checked as rows (unchanged code). The marker test (`test_sprints_pin.py:33`) — green (row S1.1c open).

### 3.5 The part-level check (`gate/runner/parts.py`, new, lane B; run by the `suppressions` row)

```python
LAST_PART = "S1.1c-8"   # decision 1: never past S1.1c-8
SIZE_ISSUE = "the size family"  # used only in texts

def forwards(entries: Sequence[allowlists.Entry], issues: Mapping[str, Mapping[str, object]], order: sprintid.Order,
             marker: str, tier: str, base_issues: Mapping[str, Mapping[str, object]] | None,
             decided: Callable[[str, str], bool]) -> list[str]
```
For each `smells.json` TEMP entry with a size target (the `unlinted` ones are skipped):
1. Its `issue` must exist with a non-empty `forward` → else `smells.json: <target>: issue <id> has no forward part`.
2. `forward` must be a printed part (`reached_part` raises) → `smells.json: <target>: forward <f> is not a printed part of gate/sprints.json`.
3. `order.index(forward) > order.index(LAST_PART)` → `smells.json: <target>: forward <f> is past S1.1c-8 (decision 1)`.
4. `reached_part(forward, marker)` and `tier in allowlists.RED_EXPIRED` → `smells.json: <target>: the marker <m> has reached its part <f>: clean <package> for <rule>, or the owner moves the forward`. Below T4 the same text is printed as a plain line (the countdown's place), never red — so a part's own branch is red at T4 from its G0 until the package is clean (the `notbuilt` precedent).
5. **A forward moved without the owner:** `base_issues` is `gate/known-issues.json` at the merge base with `main` (`git show <base>:gate/known-issues.json`; the base as `lawdelta` computes it — lane B names the function in `gate/runner/git.py`, inferred). For an issue named by a size entry whose base `forward` is a printed part and whose current `forward` is later in the order: its `decision` must differ from the base's **and** `decided(id, decision)` (`aging.decided`) must hold → else `known-issues.json: <id>: forward moved <old> → <new> without a decision that names it (decision 1: only the owner moves a forward)`. A `None` base (no merge base, or the file absent there) skips rule 5 and prints `forwards: no merge base; moves not checked`.
- Called from `rows/suppressions.py` `main` after `allowlists.check` (`:195-196`), with the lists it loaded and `ctx`. `expected_scope` of `suppressions` already holds `tools/allowlists/*.json`; it gains `gate/known-issues.json`, `gate/sprints.json` and `gate/current-sprint` (the row reads them already, `rows/suppressions.py:44-47`, but never counted them; §10 R-5 on its covered count).

## 4. Waivers (lane B)

### 4.1 The census (W0b, QA)
- Procedure: for each enabled linter (`golangci.LINTERS`), a one-package scratch module with one finding of that linter per file and one candidate directive per file (above the line, trailing, file-wide before `package`, block form `/* */`, case and spacing variants: no space, one space, a tab, `//<TAB>`), run under the pinned binary with the §2.1 config; a directive is **honoured** when its finding disappears against the control file. Recorded as data in `gate/runner/testdata/directives.json`: `[{"linter", "form", "placement", "honoured": bool}]`, which `test_suppressions.py` reads as it holds the `//nolint` tables today.
- Known so far (RM, PA-B3, PQ-S6, research D(i)): revive honours `//revive:disable-next-line[:rule]`, `//revive:disable-line[:rule]` (also trailing, with text after), `//revive:disable` … `//revive:enable`, `// revive:…` and `//<TAB>revive:…` (its regex `^//\s*revive:`), not `/* revive:… */`; gosec's `#nosec` in nine spellings incl. file-wide (gosec is not enabled, so they waive nothing today but are refused anyway, decision 7); `//gocyclo:ignore` silences gocyclo (`// gocyclo:ignore` does not); `//nolint:all`. lll skips `//go:` lines and import blocks: named residue (decision 7).
- UNFINISHED: errcheck, funlen, gocritic, forbidigo, lll and nolintlint's own directive forms — W0 measures; this note adds each honoured form to §4.2's table at v2.

### 4.2 `gate/runner/suppressions.py` changes
- `ENABLED` (`:21`) is removed as a constant; `rows/suppressions.py` passes `golangci.enabled(golangci.load(...))` to `check` (`:296`), so the liveness text becomes `//nolint:<linter>: <linter> is not enabled in .golangci.toml` (`:310`'s "(none is until S1.1c-5)" goes). A pin test: the set equals `golangci.LINTERS` both ways.
- `PREFILTER` (`:24`) += `"revive:"`, `"nosec"`, `"gocyclo:"` and each W0 form's literal.
- New recogniser `go_foreign(path, source) -> tuple[list[Directive], list[str]]` beside `go_directives` (`:152`): over `go_comments` (and block comments for `#nosec`'s `/* #nosec */`), each census form honoured → a `Directive(path, line, (<linter>,), "FOREIGN", {"form": <text>}, valid=False)` (counted in the LAW Δ, W1-F3) and a fault `<path>:<line>: <linter> honours this form; the only waiver is //nolint:<linter> // PERMANENT|TEMP … (decision 7)`. Matching follows the measured regexes: revive `^\s*revive:` after `//`; gosec `#nosec` anywhere at a comment's start after `//` or `/*` plus optional spaces, with a glued suffix; gocyclo `gocyclo:ignore` with no space after `//`.
- `//nolint:all` and `//nolint:typecheck` → fault `write the linter: //nolint:all waives every linter` / `typecheck cannot be waived` (PQ-S5), in `go_directives`.
- The accepted form (DEVELOPMENT.md:68-74, quoted): `//nolint:<linter>[,<linter>]` "two slashes, no space; exactly one of PERMANENT or TEMP; a `reason="…"` of at least 12 characters; PERMANENT carries `ref=<repository path>#<heading anchor>` … TEMP carries `expiry=YYYY-MM-DD`, at most 90 days ahead, and `issue=<id>` …; every suppression carries `sprint=<sprint id>`". Example the twins use: `//nolint:lll // PERMANENT reason="a URL that cannot wrap" ref=docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md#2-architecture-constraints-as-analyzers-with-teeth-reference-a-in-go sprint=S1.1c-5`.
- **A waiver on the wrong line**: nolintlint's `allow-unused = false` reports an unused `//nolint` (red through the `smells` row), and the real finding stays red. Test T-B6.
- **Restore BK-S4's tests**: the two `NosecIsAWaiver` tests removed at fc944ef (`gate/runner/tests/test_breakit_s11c2.py:31-39`; `git show fc944ef^:gate/runner/tests/test_breakit_s11c2.py`) come back into that module as written, adjusted only to the new fault text.

## 5. The ruff pin (lane B; `gate/runner/rows/suppressions.py:85-112`)

- `RUFF_PINNED: dict` in `rows/suppressions.py` = `ruff.toml` as `tomllib` parses it today **minus** `lint.per-file-ignores` (its value is checked both ways against `tools/allowlists/ruff.json`, as now, `:107-112`).
- `ruff_pin` becomes: parse; take `lint.per-file-ignores` out; `golangci.differences`-style diff (lane B lifts the diff walker into `gate/runner/pin.py` with `differences(doc, pinned, name) -> list[str]`, shared by `golangci.differences` — lane A imports it; one owner: lane B writes `pin.py` in W1's first commit, lane A codes against its stub) → each line `ruff.toml: <path>: …`; then the existing both-ways check. The current deny-list checks (`extend`, top-level and `lint` ignores, `:99-106`) become redundant but stay as clearer texts.
- Re-measured on 0.17.5 (M-5): every hole the pin must close reproduces; the planted cases are §8's T-B9.
- Churn named: a new `py312` file now needs `per-file-target-version` **and** `RUFF_PINNED` edited (both law paths).

## 6. The tool scan (lane C)

- **`tools.json`**: an executable tool's entry may carry `"go_built": true` (only `true` is legal; `false` → `tools.json: tools[i] (<name>): "go_built": false is the default: remove it`). `manifest._TOOL_FIELDS` (`gate/toolchain/manifest.py:34`) admits it optionally (as `kind`, `:186`); `Tool.go_built: bool = False`; a `python-package` with it is refused. Set on `golangci-lint` only (`go` itself is scanned as the engine's toolchain). Tests in `gate/toolchain/tests/test_manifest.py`. The canary's `candidate.build` copies each pinned tool dict and replaces only version/url/digests/version_line (`gate/canary/candidate.py:283-300`, inferred from `structural_faults`, `:368-390`), so the field is carried; a test proves it.
- **`vulndb.findings`** (`gate/runner/vulndb.py:234`) gains `*, pseudo: frozenset[str] = GO_PSEUDO`; the binary scan passes `frozenset({"stdlib"})`. Twin test: the engine scan's default still yields `toolchain` findings (QA PQ-S9).
- **`rows/vulns.py`**: for each `go_built` tool, `go version -m <tool_home>/<path_dir>/<command>` (the run's own platform's binary — CI's T4 and nightly read linux-amd64, a local T4 darwin-arm64); parse line 1 `<file>: go<version>`, `\tmod\t<path>\t<version>\t<sum>`, `\tdep\t…`, `\t=>\t…` replacements. Red, named: exit ≠ 0; no Go version or `go_version` raising (`:155`); no `mod` line; `mod` version ≠ the tool's pinned `version`; zero `dep` lines (vacuous). Then `findings({dep path: version (replacement applied)}, frozenset(), <go>, entries, pseudo={"stdlib"})`; every finding's pair is `<id> <tool name>` (stdlib or a dep alike; the module named in the text). `NOT_READ` (`:43-46`) drops golangci-lint. Covered stays 6 + 1 per declared binary (`COVERED`, `:33`; the registry's `min_wants` and `expected_scope` += `tools.json`).
- **Exemptions**: `tools/allowlists/vulns.json` += 15 TEMP entries `GO-2026-6599 golangci-lint` … `GO-2026-6859 golangci-lint` (research §C's list), `expiry` = commit day + 30, `issue` `S1.1c-5-4`, `sprint` `S1.1c-5`; `PAIR` (`:36`) already accepts the tool name. Renewal only by an owner decision (decision 8; LAW Δ shows it).

## 7. The whole-tree fixes (lane C) and lane D

- **7.1 Lane C's finding list** (research §B, RM counts; lane C re-runs the row first and works from its list): lll — 173 production + 31 tools lines (tests exempt); errcheck — `tools/archcheck/main.go:1643, 1646, 1661, 1681, 1686, 1688` and the 18 test sites (`boundary_test.go:35-42, 53, 54`; `example_test.go:24`; the five `Close` sites; `tools/apipartition/main_test.go:58, 62`); gocritic appendAssign `internal/playback/recorder.go:55, 71, 120, 140, 166, 199` (a `//nolint:gocritic // PERMANENT` per site naming the buffer-reuse idiom, or a rewrite proven by the playback tests — §10 R-4) and `internal/table/breakit_test.go:77`, `table_test.go:292` (lane D's); forbidigo — the 15 production panics (research §B list) each `//nolint:forbidigo // PERMANENT reason="<its invariant>" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-5`, `row.go:73`'s reason naming MUST 14; the canonical encoder moved from `internal/model/model.go:397-447` to `internal/model/canonical.go` (pure move, AST-checked), its reflect uses waived per line with `ref=docs/architecture/adr/ADR-0003-…#<§2 heading>` (UNFINISHED: one waiver per reflect line is ~33 waivers; whether a function-level `//nolint` above `encode`/`encodeStruct` covers its body under golangci v2 — measured in W0 — decides the count); the 28 test panics and `internal/runtime/breakit_test.go:80` (`BK_CHILD`) as PERMANENT waivers; the golden-update switches (`internal/parser/golden_test.go:16`, `internal/playback/baseline_test.go:52`, `s3_2b_2_corpus_test.go:198, 291, 301`) become test flags (`var update = flag.Bool("update", false, …)` per package; the Makefile/driver callers that set the env vars change with them — UNFINISHED: lane C greps `CEP_UPDATE_GOLDEN|CEP_BASELINE_WRITE|CEP_TRIPWIRE_FILE|CEP_DETERMINISM_PLANT` across `gate/`, `Makefile` and drivers and lists each caller).
- **7.2 Lane D's packages**: `internal/table` (gocyclo `New` 13, `Table.checkValue` 13; lll 1), `internal/clock` (`RuleClock.fireGroup` 11; panic `rule.go:124` waiver), `io/inmemory` (`Source.Open` 11; lll 1), `tools/shape` (`shape` 19, `shapes` 11).
- **7.3 Refactor rules**: split by concern, each new function named for what it does; no `xPart1..n`, no option struct whose only purpose is to pass > 5 values (a struct that already exists as a domain type may be passed); no behaviour change in the same commit as a move; tests are not reflowed in a refactor commit.
- **7.4 The AST-equality check**: `tools/astequal/main.go` (stdlib `go/parser`, `go/ast`, `go/token`; package main; a law path under `tools/`): `astequal <before-rev> [<paths>]` reads each file at `git show <rev>:<path>` and in the work tree, parses both with comments dropped, and compares the declaration sets by name with positions zeroed (`ast.Inspect` printing each node's type and literal values into a canonical string); a pure move across files of one package compares the package's union. Exit 0 equal; 1 naming the first differing declaration. Its own tests: a reflow equal, a renamed variable unequal, a moved function equal, a changed literal unequal. Run by lane D per reflow/move commit and quoted in the record (not a gate row — §10 R-7).
- **7.5 Characterisation and performance**: `tools/shape` gets `tools/shape/shape_test.go` first — every corpus file's shape output pinned as golden, plus one case per construct kind the corpus lacks (UNFINISHED: list the kinds). Hot paths (none in lane D's four; decision 6 applies from S1.1c-6) get `Benchmark…` + `testing.AllocsPerRun` pinned before the refactor.
- **7.6 The calibration record**: per package, the start and end time of each commit, findings cleared by kind, and hours per finding kind, as a table in `s1-1c-5-record.md`; the coordinator re-cuts S1.1c-6…-8 from it at close (decision 2).

## 8. Tests and proofs, red first (QA, W0b)

UNFINISHED (names drafted; QA fixes the exact names and counts):
- **Card line 1 (rules bite)** — `gate/runner/tests/test_golangci.py`: T-A1 `differences` red per changed/added/removed key, incl. `issues.new-from-rev`, `run.tests=false`, caps removed, `uniq-by-line`, `generated="lax"`, `funlen.statements=-1`, `exclude-godoc-examples`, an extra `exclusions.paths`; T-A2 `read_report` on recorded JSON (issues, `Error`, warnings); T-A3 `rule_of` for revive's two texts. `gate/runner/tests/test_rows_smells.py`: exit agreement (exit 0 + issue, exit 1 + none, exit 3, missing file), `Report.Error`, a warning, a typecheck issue in another package, `Report.enabled` short by one, the fmtprint drop (main under tools green, non-main red), generated.json drop, merge of two GOOS. `gate/proofs/drivers/test_smells_planted.py` (real binary, scratch module copied from a fixture tree): each bound green and one past red — function 60/61 lines (comments counted), cyclomatic 10/11, file 500/501, params 5/6, line 120/121 with tabs; each forbidigo pattern red in scope and green out (reflect in L3 and in `_test.go`, Getenv under `io/`, fmtprint in a main tool); `println`, `print`, `os.LookupEnv`, `os.Environ`; `import _ "unsafe"` + `//go:linkname` red via step 7; Example function; errcheck `rt.Close()`; gocritic appendAssign; nolintlint unused; a fourth panic beside three waived ones.
- **Line 2 (only //nolint)** — `test_suppressions.py`: each census form refused (data-driven from `directives.json`), `//nolint:all`, `//nolint:typecheck`, metadata missing, the accepted twin `//nolint:lll // PERMANENT …` passing, `ENABLED` equals the config; the restored `NosecIsAWaiver` tests.
- **Line 3 (exclusions bounded)** — `test_sprintid_parts.py`: `reached_part` table (§3.2). `test_parts.py`: forward reached at T4 red / at T2 a line; unprinted part; past S1.1c-8; no forward; moved without decision; moved with decision green; no merge base. `test_rows_smells.py`: an exclusion that waives nothing; an unlisted new package red; an unclaimed file; `//go:build ignore`; nested `go.mod`; `_linux.go` counted once.
- **Line 4 (ruff pin closed)** — `test_rows_ruff.py`/`test_suppressions.py`: each of M-5's keys red, naming its path; `lint.select` narrowed red; per-file-ignores still both ways.
- **Line 5 (declared tools scanned)** — `test_vulns_row.py` and `test_vulndb.py`: a declared binary with a stdlib advisory red unless exempted; exempt `<id> golangci-lint` green; `toolchain` never for a binary; engine scan still judges `toolchain` (twin); read failure, no `mod`, wrong version, zero deps, `devel` version each red; a planted case in `test_vulns_planted.py`. `test_manifest.py`: `go_built` accepted/refused forms; canary carries it.
- **Lines 6–7** — the T4 run and the record; AST-equality runs per lane D commit.
- **Counts that move**: `proofs` `min_wants` 38 → 38 + new drivers (1, `test_smells_planted.py`); `law.json` `proofs.cases` for `suppressions` (9 → +restored), `vulns` (31 → +1 or more), `smells` (new). UNFINISHED: exact numbers after QA writes the cases (decision 5 of 2026-11-10: a fixture never shrinks).
- **T-A12 (M-4's residue)**: on the real tree, errcheck under defaults equals errcheck under the written-out list (W0 runs both and records the counts).

## 9. Docs and ADR annotations (coordinator)

UNFINISHED (sentences drafted; the coordinator finalises):
- DEVELOPMENT.md:5 — replace "**Not built yet:** the suppressions checker, `todocheck` and golangci-lint's `smells` (S1.1c-2)." with "**Built in S1.1c-2:** the suppressions checker. **Built in S1.1c-5:** golangci-lint's `smells` row (`.golangci.toml` pinned whole; linux and darwin). **Not built yet:** `todocheck`."
- DEVELOPMENT.md:65 — "golangci-lint's linters and `nolintlint` run in the `smells` row from S1.1c-5; a `//nolint` naming a linter `.golangci.toml` does not enable is red, and every other waiver form a linter honours (`//revive:…`, `#nosec`, `//gocyclo:ignore`, …) is red (owner, 2026-11-14, S1.1c-5's plan check, 7)."
- DEVELOPMENT.md:70 — the example becomes `//nolint:lll // PERMANENT reason="…" ref=… sprint=S1.1c-5` (gosec is not enabled).
- ADR-0015 §2 `smells` row: "*(Annotated 2026-11-14, S1.1c-5 ([owner](../../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check), decisions 1, 4, 7, 9, 10): built as the `smells` row, T0–T4, over `.golangci.toml` pinned whole, under linux and darwin; "prod scope only" covers line ≤ 120; `fmt.Print*` allowed in package-main tools; `unsafe` checked from `go list` imports; the size limits enter by package through TEMP exclusions forwarded to S1.1c-6…-8.)*"
- ADR-0015 §2.1 test class: "*(Annotated 2026-11-14 (decision 4): tests are also exempt from lll and from the `reflect` pattern; `panic` and `os.Getenv` stay, waived in code.)*"
- ADR-0015 §2.2: "*(Annotated 2026-11-14 (decision 1): the size family's debt at the `smells` row's landing is held to dated parts — TEMP exclusions per package and rule, red when the marker reaches the part, moved only by the owner, never past S1.1c-8 — not recorded as maxima.)*"

## 10. Risks and open points for the reviewers (none is an owner decision; each is a question)

- **Q-A (for the owner at G1; blocks lane A): the config format.** M-1: JSON cannot configure revive's file-length and params rules in golangci v2.15.0 / revive v1.17.1. Recommended: `.golangci.toml`, read by stdlib `tomllib` (no dependency; the `ruff.toml` precedent). Other: keep JSON and drop revive (no linter for file ≤ 500 and params ≤ 5 — the ADR's limits unenforced); or YAML (a new parser dependency).
- **R-1** Where the four `testdata` fixtures are accounted for: PERMANENT `unlinted` entries (this note) or a row rule skipping `testdata/` (ADR §2.1 puts testdata in the test class, which golangci cannot lint).
- **R-2** Printing S1.1c-5…-8 makes each closed part a closed row for aging rule 1: S3.3-2-1/-2/-3 (no decision) turn red at S1.1c-7's close; and the four new issues have no deciding section naming them. Should G1's answers name the ids?
- **R-3** `log.Print*` is outside the patterns (PQ-L1); `os.Environ` was added beside the card's `LookupEnv`.
- **R-4** `playback/recorder.go`'s six appendAssign: waive (the reuse idiom, aliasing-safe today) or rewrite in lane C behind the playback tests.
- **R-5** The `suppressions` row's covered count does not include the side inputs it now reads (known issues, sprints, marker, base file).
- **R-6** T0 time for two GOOS runs (W0 measures); the staged-package mapping (§2.7) is inferred.
- **R-7** The AST-equality tool is a record-only check, not a row; reviewers may want a row.
- **R-8** Not measured: Report.Warnings' JSON shape; `typecheck` in `Report.Linters`; whether a function-level `//nolint` covers reflect uses in its body; `warn-unused`'s effect; the linux binary's build info.
- **R-9** The part-level check's merge-base read needs git in the `suppressions` row (today only `ls-files`).
