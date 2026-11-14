# S1.1c-5 — design note (v2 with the coordinator's two G1 rulings, 2026-11-14; after the blind architecture and QA reviews and the owner's answers; for G1)

*Step 1 of the full loop for S1.1c-5, from the answered [plan check](s1-1c-5-plan-check.md) (v2), the [owner's ten answers of 2026-11-14](decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check), the [config answer before the reviews](decisions.md#owner-decisions-2026-11-14--s11c-5s-design-before-its-reviews) and the [three answers after the reviews](decisions.md#owner-decisions-2026-11-14--s11c-5s-design-after-the-reviews). v1 was reviewed blind by architecture with the-fool ([review](../reviews/s1-1c-5-design-architecture.md), DA-*) and QA ([review](../reviews/s1-1c-5-design-qa.md), DQ-*); every finding's disposition is §11. A build-ready spec: every path, signature, data format, failure text, test and step a lane needs. Code is cited as `file:line` on `s1-1c-5/integration` @ bda6554 (unchanged at 92a47d9 for every cited code line). "M-n" marks a measurement made for this note (§0; scratch `s1-1c-5-scratch/design-exp/` and QA's `s1-1c-5-scratch/qa-design-exp/`, pinned golangci-lint v2.15.0, go1.27.4, ruff 0.17.5). "(inferred)" marks a claim not measured or not read in code. Nothing here re-decides an owner answer.*

## 0. Measured before G1

- **M-1 — `.golangci.json` cannot configure revive's two size rules.** golangci-lint v2.15.0 decodes JSON numbers as float64; revive v1.17.1 requires int64 for `argument-limit`'s argument and `file-length-limit`'s `max`. With the research's config converted to JSON, `config verify` exits 0, but `run` exits 1, `Report.Error` = `setup revive: cannot configure rule: "argument-limit": invalid value passed as argument number to the "argument-limit" rule`, and **revive reports nothing** while every other linter still reports (16 issues). The same document as **TOML** verifies, runs, and reports both (`p.go:3 argument-limit`, `p.go:501 file-length-limit`; a 500-line file and a five-parameter function are green). Answered: the config is `.golangci.toml` (owner, before the reviews, decision 1).
- **M-2 — the output channel.** With `output.show-stats = false`, no `output.formats` in the config and `--output.json.path=<absolute file>` on the command line, stdout is empty (0 bytes) and the file holds `{"Issues": [...], "Report": {"Linters": [...], "Error"?: str, "Warnings"?: [...]}}`. A revive setup failure still exits 1 with issues present: exit status alone cannot tell it from findings, so `Report.Error` must be read.
- **M-3 — forbidigo's bypasses.** With `analyze-types = true`, pattern `^(fmt\.Print.*|print|println)$` reports the builtins `print` and `println`; `^os\.(Getenv|LookupEnv|Environ)$` reports all three. `import _ "unsafe"` plus `//go:linkname` gives **no** forbidigo finding (QA's PQ-B4 reproduced). `exclude-godoc-examples = false` reports `panic` and `os.Getenv` inside `func ExampleH`.
- **M-4 — errcheck's built-in exclusions, written out.** The 25 symbols in §2.1's `exclude-functions`, with `disable-default-exclusions = true`, give exactly the findings of errcheck v1.30.0's defaults on a plant of all 25 (3 = 3, identical sets); with the defaults off and no list, 27 more are reported. Residue: a default this plant does not exercise would be missed — W0's real-tree comparison (§8, 6.3) closes it.
- **M-5 — ruff 0.17.5 reproduces every 0.17.3 hole.** The research's eight `rufftest` variants give the same results on 0.17.5; QA's three keys (`builtins`, `lint.dummy-variable-rgx = ".*"`, `[lint.pycodestyle] max-line-length = 400`) together hide F821, F841 and E501: "All checks passed!" on 0.17.5 (base: 3 errors).
- **M-6 — the tree's unclaimed Go files.** 269 tracked `.go` files, 260 outside `gate/proofs/`. Four sit under `testdata/`, which `go list ./...` never claims: `tools/apipartition/testdata/embedded/embedded.go`, `…/fixture/fixture.go`, `…/fixture/fixture_test.go`, `…/fixture_reflow/fixture.go`. No `_`/`.` directory, no build constraint, no GOOS/GOARCH file suffix, no nested `go.mod` beyond `gate/proofs/go.mod`. `tools/allowlists/generated.json` is empty. The root package holds 37 tracked `.go` files (DQ-S3).
- **M-7 — `unsafe` is already archcheck's, outside `tools/`.** `tools/archcheck/main.go:565-566` reds any `import "unsafe"` and `:575-579` any `//go:linkname`, but the `archcheck` row's `expected_scope` excludes `gate/**` and `tools/**`. The Go compiler requires `import _ "unsafe"` for `//go:linkname` (inferred from the Go spec's linkname rule), so an import check covers both.
- **M-8 — the report's two unread shapes (QA, R-8).** `Report.Warnings` = `[{"Tag": "runner/source_code", "Text": "…"}]` (a `//line` naming a missing file); `typecheck` is listed in `Report.Linters` with `Enabled: true`.
- **M-9 — `//nolint` spellings (QA, DQ-S6, qa-design-exp/m2).** With the §2.1 config golangci honours `// nolint:lll`, `//nolint: lll`, `//nolint:LLL`, `//  nolint:lll`, `//nolint:lll,`, bare `//nolint` and `//nolint:lll // x`; it ignores `//NOLINT:lll`, `/*nolint:lll*/` and `//lint:ignore lll x`.
- **M-10 — `//line` relocates a finding (QA, DQ-B2, qa-design-exp/m1).** In the non-main package `gen/g.go`, `//line ../tools/t/main.go:3` before `fmt.Println` makes golangci report `forbidigo tools/t/main.go:3 [fmtprint]`, exit 1, no `Report.Error`, no warning. A `//line` naming a missing file gives a `runner/source_code` warning. The tree holds 0 `//line` directives (`git grep`).
- **M-11 — `==` is type-blind (QA, DQ-S1).** `tomllib` documents with `arguments = [5.0]` or `tests = 1` compare `==` equal to the pinned one. With `[5.0]` golangci exits 1 with M-1's `Report.Error`.
- **M-12 — a stale report and a shared cache (QA, DQ-S2, DQ-S4).** A config error (exit 3) leaves a pre-existing `--output.json.path` file untouched. With a shared cache, the `[5.0]` run still listed revive's `argument-limit` issue from an earlier run while `Report.Error` said revive could not be set up (one comparison); with a fresh cache it listed none. `env.clean_env` drops every `GO*` name not in `GO_ALLOWED` (`gate/runner/env.py`), so `GOLANGCI_LINT_CACHE` never reaches a row unless the row sets it.
- **M-13 — the binary's main-module line (architecture, DA-S7).** In `s1-1c-5-scratch/golangci-lint.buildinfo.txt` (darwin-arm64, `CGO_ENABLED=0`, 208 `dep` lines) line 3 is `\tmod\tgithub.com/golangci/golangci-lint/v2\tv2.15.0\t`: the sum field is empty.
- **Not measured here (W0 measures, §8):** the linux-amd64 golangci-lint binary's build information (W0 fetches the pinned linux archive into scratch, checks its sha256 against `tools.json`, records the `go` line, the `mod` line and the dep count); the directive census (§4.1); the row's T0 and T1 times (§2.7); the real tree's counts under the §2.1 config; golangci v2.15.0's default-skipped directory names (DA-L5: `examples/`, `third_party/`, `Godeps/`, `builtin/`, `vendor/` — W0 plants a package under each and records which are linted); whether `Table.checkValue` and `RuleClock.fireGroup` sit on the per-event flow (DQ-S12, §7.5); whether `rows/vulns.py` already reds an exemption that matches no finding (DQ-S9, §6); the planted driver's cost and whether cases may share one run (DQ-L5, §8); `warn-unused`'s effect.

## 1. Lanes, waves, ownership (≤ 2 agents at once)

| Wave | Lane (agent) | Owns (one owner per file per wave) | Exit |
|---|---|---|---|
| G0 (done, bda6554) | coordinator | `gate/current-sprint` | — |
| W0a | coordinator (law commit, before QA) | `gate/sprints.json` and the plan's §3 table (S1.1c-5…S1.1c-8 printed, §3.4); `gate/known-issues.json` + `docs/product/known-issues.md` (the four new issues, §3.3, each `decision` the after-the-reviews section); `gate/law.json` paths (+ `.golangci.toml`, §2.6); `tools/allowlists/smells.json` is **lane B's** (it lands with `allowlists.RULES` += `smells`, PA-S13), so W0a leaves it absent | T2 green |
| W0b | QA (qa-expert) | every test module named in §8 — new files, plus the edited existing modules named there (`gate/runner/tests/test_sprintid_parts.py`, `test_suppressions.py`, `test_rows_ruff.py`, `test_aging.py`, `test_instruments_pin.py`, `test_allowlists.py`, `test_sprints_pin.py`, `gate/toolchain/tests/test_manifest.py`, `gate/canary/tests/test_candidate.py`, `test_vulndb.py`, `test_vulns_row.py`, the restored block in `test_breakit_s11c2.py`, `gate/proofs/drivers/test_suppressions_planted.py`, `test_vulns_planted.py`); `gate/proofs/drivers/test_smells_planted.py` (new); the census data file `gate/runner/testdata/directives.json`; the W0 measurements of §0's "not measured" list | each test red on the stubs for its stated reason (or, for a test green from birth, its mutation shown red); census, linux build info and the measurements recorded in the report |
| W0c | coordinator | the stubs (§2–§6 signatures, `raise NotImplementedError`), so QA's tests import | QA's modules import |
| W1 | **lane A** (python-engineer): the row | `.golangci.toml`; `gate/runner/golangci.py` (new); `gate/runner/rows/smells.py` (new); `gate/ci/generate.py` only if §2.6's check needs it | A's tests green; T2 green with `smells` **red on the real tree** (expected until W2; the lane's report quotes the counts) |
| W1 | **lane B** (python-engineer): waivers, checker, ruff pin, parts, aging | `gate/runner/pin.py` (new, W1's first commit); `gate/runner/suppressions.py`; `gate/runner/rows/suppressions.py`; `gate/runner/allowlists.py` (`RULES`); `tools/allowlists/smells.json` (created empty, rule `smells`); `gate/runner/sprintid.py` (`Order.reached_part`); `gate/runner/parts.py` (new); **`gate/runner/rows/aging.py`** (calls `parts.forwards`, §3.5); `gate/runner/tests/test_breakit_s11c2.py` (restore and its docstring's citation fix only) | B's tests green; T2 green except `smells` |
| W2 | **lane C** (go-engineer + python for the scan; one agent): the tool scan, then the whole-tree fixes | `tools.json` (`go_built`); `gate/toolchain/manifest.py`; `gate/canary/candidate.py` (carry the field); `gate/runner/vulndb.py` (`findings`' `pseudo`); `gate/runner/rows/vulns.py`; `tools/allowlists/vulns.json`; then every non-test and test `.go` file in §7.1 **outside lane D's four packages**, incl. `internal/model/canonical.go` (new) | C's tests green; `vulns` green; `smells` green on everything but the size family |
| W2 | **lane D** (go-engineer): calibration | `internal/table/**`, `internal/clock/**`, `io/inmemory/**`, `tools/shape/**` (incl. their test files for the forbidigo/errcheck fixes in them); `tools/astequal/**` (new, §7.4); its calibration record lines (§7.6) | the four packages clean for the whole set, their `smells.json` entries never written; AST-equality, characterisation and benchmarks as §7 |
| G1→T3 | coordinator | `tools/allowlists/smells.json` entries (§3.1, written from lane A's real-tree counts after W2), DEVELOPMENT.md, ADR-0015 annotations (§9), the record (incl. the G5 `astequal` re-run, §7.4) | T3, then T4 |

Order inside W2: lane C's scan commit first (it touches no Go file lane D owns), then C's whole-tree fixes. Lane C never edits a file under lane D's four directories; a whole-tree finding there is D's. **`gate/registry.json` and `gate/law.json` are the coordinator's for the whole sprint** (G1 ruling, one owner per file): every row, instrument, scope, `code` list and count in §2.5, §3.5, §6 and §8 is staged by the coordinator in W0a — including fixing an instrument already wrong today, such as `aging`'s `code` list missing `knownissues` and `sprintid` (L.1) — and any later change is requested in a lane's report and applied by the coordinator at the combine.

## 2. The `smells` row (lane A)

### 2.1 The config, `.golangci.toml`

The whole document, in this key order (the pin compares parsed values with their types, §2.2, so order and comments are free):

```toml
# The smells row's law (ADR-0015 §2; owner 2026-11-14, S1.1c-5's plan check and design). Pinned whole by gate/runner/golangci.py.
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
[[linters.exclusions.rules]]   # decision 4: tests exempt from the size family and lll
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
- `[fmtprint]` is **not** excluded in the config: decision 10 allows `fmt.Print*` in package-main tools, which a path regex cannot express. The row drops a `[fmtprint]` forbidigo issue only in a file under `tools/` whose package name, from `go list`, is `main` (§2.3 step 6b), so a non-main package under `tools/` stays red.
- `[getenv]` widens the ADR's `os.Getenv` to `LookupEnv` and `Environ` (the card's acceptance line 1 names `os.LookupEnv`; `os.Environ` is the same smell, zero uses today; kept, owner after the reviews, decision 3's context). `log.Print*` is **not** added — owner, after the reviews, decision 3: backlog.
- `warn-unused = false` is written: `true` would warn (and so red) on the `^(cmd|io)/` rule while no Getenv exists there (inferred; W0 measures).
- `panic` and `os.Getenv` stay on tests (decision 4); they are waived in code, per line (§4.2, §7.1).

### 2.2 The whole-document pin and the shared reader (`gate/runner/golangci.py`, new, lane A; the only reader of the config — lane B imports it)

The diff walker is lane B's `gate/runner/pin.py` (§5), written in W1's first commit; lane A codes against its stub.

```python
# gate/runner/pin.py (lane B)
def differences(doc: Mapping[str, object], pinned: Mapping[str, object], name: str) -> list[str]
    # [] when doc equals pinned type-strictly; else one line per path, sorted:
    #   "<name>: <path>: <got!r> ≠ pinned <want!r>", "<name>: <path>: not pinned (remove it)",
    #   "<name>: <path>: missing (pinned <want!r>)"
    # Type-strict and recursive (DQ-S1): two values are equal only when type(a) is type(b) at every level, so
    # bool ≠ int ≠ float (True ≠ 1, 5 ≠ 5.0); dicts by key set and per key; lists by length and per index — order
    # counts. <path> joins keys with "." and indexes lists ("linters.settings.revive.rules[1].arguments[0]",
    # DQ-L1), quoting a key that holds ".", "[", or a space ('"a.b"'). The walk descends into the first differing
    # list element, so a change inside revive.rules names that element's leaf, not the whole array.
```

```python
# gate/runner/golangci.py (lane A)
CONFIG = ".golangci.toml"
LINTERS = ("errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive")
SIZE_RULES = ("funlen", "gocyclo", "file-length-limit", "argument-limit")  # the size family, as smells.json names them
PINNED: dict[str, object]  # the §2.1 document as tomllib parses it, written as a Python literal

class ConfigError(ValueError): ...

def load(text: str) -> dict[str, object]                 # tomllib.loads; a TOMLDecodeError (incl. a duplicate key)
                                                         # → ConfigError(f"{CONFIG}: {e}")
def differences(doc: Mapping[str, object]) -> list[str]  # pin.differences(doc, PINNED, CONFIG)
def enabled(doc: Mapping[str, object]) -> frozenset[str] # doc["linters"]["enable"] as a set; ConfigError if absent
def rule_of(issue: Mapping[str, object]) -> str          # "funlen" | "gocyclo" | "file-length-limit" | "argument-limit"
    # | the linter name (errcheck, forbidigo, gocritic, lll, nolintlint, typecheck) — revive's rule from Text's prefix
    # "<rule>: " (M-1's texts); a revive Text with another prefix → "revive"
def read_report(path: str) -> Report                      # the JSON file; ReportError on a malformed document
class ReportError(ValueError): ...                        # a JSONDecodeError or a wrong shape, with the path
@dataclass(frozen=True) class Issue: linter: str; rule: str; text: str; file: str; line: int; column: int
@dataclass(frozen=True) class Report: issues: tuple[Issue, ...]; enabled: frozenset[str]; error: str; warnings: tuple[str, ...]
```
- `read_report` reads `Issues[*].FromLinter`, `.Text`, `.Pos.Filename` (module-relative, M-2), `.Pos.Line`, `.Pos.Column`; `Report.Linters[*]` with `Enabled: true` → `enabled` (M-8: `typecheck` is among them); `Report.Error` (string, absent = ""); `Report.Warnings[*].Text` (M-8: `[{"Tag", "Text"}]`). Unknown top-level keys are ignored; a missing or null `Issues` is an empty tuple.
- golangci.py imports `pin` only; it does not import `suppressions` (DA-S6: the package name comes from `go list`, §2.4).
- The pin reads only `CONFIG`. `PINNED` lives in code, so a config edit without the code edit is red, and both files are law paths (§2.6).

### 2.3 The row wrapper (`gate/runner/rows/smells.py`, new, lane A)

`main(argv) -> int`; argv[1] is `{files}` (the files list; at T0 the staged scope, §2.7). The row reads its tier from `os.environ["GATE_TIER"]`, as `rows/suppressions.py:59` does. Steps, all faults collected, each printed `failed: smells: <text>`:

1. `golangci.load` + `golangci.differences` → each line a fault; any fault here stops the run (no lint against a wrong config).
2. **One environment per platform** (DA-S1, DQ-S13). `PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))`. For each, `env = {**os.environ, **gomods.ENV, "GOOS": goos, "GOARCH": goarch, "GOLANGCI_LINT_CACHE": <cache>}` — the `vulns` row's shape (`rows/vulns.py:295`); `gomods.ENV` is `gate/runner/rows/gomods.py:22` (`GOPROXY=off`, `GOFLAGS=-mod=readonly`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`), imported, never copied; `os.environ` is the env the runner gave the row (it already carries `env.PINNED`'s `GOWORK=off`, `gate/runner/env.py:7`). The same dict is passed to `go list` (step 3) and `golangci-lint` (step 4) of that platform. `<cache>` (DA-L1, DQ-S4) = `os.path.join(manifest.cache_root(os.environ), "golangci-lint-cache")` — `gate/toolchain/manifest.py:76` (`VORSAR_CEP_TOOLS_CACHE`, else `~/.cache/vorsar/cep/tools`), outside the repo; created if absent; `gate/toolchain/manifest.py` joins the `smells` closure. Planted worlds pass a fresh temporary directory instead (§8 1.13).
3. **The covered count and the file map** (§2.4) per platform: `go list -e -json ./...` with that platform's env, cwd = `os.path.realpath(<repo root>)` (DQ-L2).
4. **Lint** per platform: first `os.unlink` both `<row_dir>/smells-<goos>.json` if present (DQ-S2; a missing file is fine, any other `OSError` is a fault); then `golangci-lint run --config .golangci.toml --output.json.path=<row_dir>/smells-<goos>.json <targets>` with that platform's env, cwd the realpath'd root, stdout and stderr to `<row_dir>/smells-<goos>.stdout|.stderr`. `<targets>` is `./...` at T1 and up (from the root it never enters `gate/proofs`, a nested module); at T0 §2.7's per-package list. No `--new`, `--new-from-rev`, `--new-from-patch`, `--new-from-merge-base` or `--whole-files` is ever passed (R-6 test).
5. **Exit agreement** per run: exit 0 ⇔ no issue and no `Report.Error`; exit 1 ⇔ at least one issue or a `Report.Error`; any other exit (3 = config error) → `golangci-lint (<goos>) exit <n>: <first stderr line>`; a missing report file → `golangci-lint (<goos>) wrote no report`; a `ReportError` → `golangci-lint (<goos>): <row_dir>/smells-<goos>.json: unreadable report: <error>` (DQ-S2); exit 0 with issues, or exit 1 with neither issues nor error → `golangci-lint (<goos>): exit <n> disagrees with <k> issues`.
6. **Report health**: `Report.Error` → `golangci-lint (<goos>): <error>`; each warning → `golangci-lint (<goos>) warns: <text>`; `Report.enabled != set(LINTERS) | {"typecheck"}` (M-8) → `golangci-lint (<goos>) ran <sorted> not <sorted>`; any issue with `linter == "typecheck"` → red whatever its file, never dropped.
7. **File map check** (DQ-B2): an issue whose `file` is not in that platform's `go list` file map (§2.4) → `<file>:<line>: <linter>: reported at a file no package claims under <goos> (a //line directive?): <text>`; such an issue is never dropped by step 8. (The `suppressions` row also refuses every `//line` directive, §4.2.)
8. **Merge and drop.** Merge the two runs' issues by `(file, line, column, linter, text)` (a finding on both GOOS counts once). Then, in order: (a) drop every issue whose `file` is a `generated.json` target; (b) drop a `[fmtprint]` forbidigo issue whose file is under `tools/` and whose package name in the file map is `main` (DA-S6; an external test file's name is `<Name>_test`, never `main`); (c) apply the size-family exclusions (§3.1): drop an issue with `rule_of in SIZE_RULES` whose file's directory — `posixpath.dirname(file) or "."` (DQ-S3: the root package is `.`) — equals an entry's package and whose rule equals the entry's rule, **and the entry is live**: an entry past its `expiry` drops nothing at a tier in `allowlists.RED_EXPIRED` (T4; the `smells` row has no nightly tier — the `suppressions` row judges expiry at nightly through `allowlists.check`, DA-L3); record per entry the number it dropped.
9. **Unsafe**: from step 3's `go list` data, any package whose `Imports`, `TestImports` or `XTestImports` holds `unsafe` → `<import path>: imports unsafe (banned everywhere, ADR-0015 §2 smells; //go:linkname needs it)`. No waiver (M-7).
10. **Liveness of exclusions — full-tree runs only** (DA-B1, DQ-S5): when the tier is T1 or higher (the row linted `./...`) and both platforms completed with no fault of steps 1–6, each `smells.json` size entry that dropped nothing → `tools/allowlists/smells.json: <target> waives nothing: the package is clean for <rule>; remove it`. A partial run proves nothing (the `vulns` row's `complete` rule, `rows/vulns.py:276-278`). At T0 the step is skipped and the row prints the plain line `smells: exclusions not judged at T0 (staged packages only)`.
11. Each kept issue → `<file>:<line>:<col>: <linter>[/<rule>]: <text>`, sorted. `rows.covered(<covered>)`; `rows.finish(faults)`.

Reason (green): the runner prints `covered <n>`; the row also prints one plain line (not `failed:`), pinned by a test (DQ-L4):
`smells: <n> issues dropped by <m> exclusions (<pkg> <rule> <k>, …); <d> testdata files counted as data; linux and darwin`
with the exclusions sorted by (package, rule). It keeps the debt loud in the record, and the record diffs the per-(package, rule) counts across closes (DA pre-mortem 1).

### 2.4 Covered from `go list` (lane A; `golangci.covered`)

```python
@dataclass(frozen=True) class GoFile: import_path: str; package: str   # package = go list's Name; XTestGoFiles → Name + "_test"
def go_files(listing: Sequence[Mapping[str, object]], root: str) -> tuple[dict[str, GoFile], list[str]]
    # file → GoFile for every GoFiles, CgoFiles, TestGoFiles, XTestGoFiles entry (Dir joined, made relative to
    # realpath(root); a Dir outside the root is a fault); faults: a file claimed twice in one listing; a package
    # with Error → "<import path>: <Err's first line>"
def covered(tracked: Sequence[str], linux: Mapping[str, GoFile], darwin: Mapping[str, GoFile],
            ignored_both: frozenset[str], gomods: Sequence[str]) -> tuple[int, int, list[str]]
    # → (covered, testdata_count, faults)
```
- `listing` is step 3's `go list -e -json ./...` (no `-test`, so a file sits in one package entry), decoded with `jsonstream.parse` (`gate/runner/jsonstream.py`).
- `tracked` = `git ls-files -z -- '*.go' ':(exclude)gate/proofs/**'` (the row's own call, as `rows/suppressions.py:34-39`).
- **Testdata is data, by rule** (R-1, coordinator; the `unlinted` target form is removed): a tracked `.go` file with a path segment exactly `testdata` (`"testdata" in path.split("/")[:-1]`) is accounted for as data and counted in `testdata_count` (printed in the green line). The Go toolchain never compiles a package under `testdata`, so nothing there can ship.
- Every other tracked file is **accounted for** only when it is in `linux ∪ darwin`. Red, each named: a tracked file in neither map → `<path>: no package under linux or darwin claims it (_dir/, //go:build ignore, a nested module): move it or make it build`; a file in `IgnoredGoFiles` under **both** GOOS (`ignored_both`) → `<path>: excluded by build constraints under linux and darwin`; any tracked `go.mod` other than `go.mod` and `gate/proofs/go.mod` (from `gomods`, the row's `git ls-files -z -- go.mod ':(glob)**/go.mod'`) → `<path>: a nested module is never linted`.
- `covered` = the number of tracked files accounted for, testdata included (union, so a `_linux.go` file counts once). The registry's `expected_scope` is `["git", ":(glob)**/*.go", ":(glob,exclude)gate/proofs/**"]`, so `covered == expected` (`gate/runner/verdict.py:57`) holds exactly when every file is accounted for.
- Today: 260 tracked, 256 claimed, 4 under `testdata/` (M-6) → covered 260, green line `4 testdata files counted as data`.

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
 "code": ["<the import closure of gate/runner/rows/smells.py, §2.5 rule>", ".golangci.toml"],
 "proofs": [{"path": "gate/proofs/drivers/test_smells_planted.py", "cases": 30}],
 "tiers": ["T0", "T1", "T2", "T3", "T4"], "min_wants": 1,
 "expected_scope": ["git", ":(glob)**/*.go", ":(glob,exclude)gate/proofs/**"]}
```
- **The `code` rule (DA-S5, DQ-L3), for every instrument this sprint touches:** `code` is the instrument's row module plus the transitive closure of its `gate.*` imports (repository modules only, `__init__.py` files included where imported), plus its config files. Expected today, before lanes measure: `smells` = `rows/smells.py`, `rows/__init__.py`, `golangci.py`, `pin.py`, `rows/gomods.py`, `allowlists.py`, `jsonstream.py`, `gate/toolchain/manifest.py`, and whatever those import (`sprintid.py`, `anchors.py` through `allowlists`, inferred); `aging` += `parts.py`, `git.py`, `allowlists.py`, `sprintid.py`, `knownissues.py` (and their closure); `suppressions` += `golangci.py`, `pin.py` (no longer `parts.py`, §3.5); `vulns` += `gate/toolchain/manifest.py`. The closure test (`test_instruments_pin.py :: CodeClosure.test_each_instruments_code_is_its_import_closure`, §8) derives each closure from the modules' `import`/`from … import` statements with `ast` and compares it with `code` both ways; it is the authority, and the lists above are its expected output.
- `timeout_s` 600 is a placeholder; W0 measures T2's time (two GOOS runs ≈ 2 × 17 s cold on darwin per RM, inferred; warm with the named cache) and lane A sets 4× the measured CI time, as S1.1c-4 did for `vulns`.

### 2.6 Law paths, CI

- `gate/law.json` `paths` += `{"path": ".golangci.toml", "kind": "code"}` (W0a; the `ruff.toml` precedent). A semantic LAW Δ parser is backlog (card).
- CI: job `lint` (registry `job`), generated by `gate/ci/generate.py` from the registry (`_jobs`, `:219`) — no generator change (inferred; `make ci-workflow` regenerates `.github/workflows/gate.yml`, and `ci-fresh` reds a stale one). golangci-lint is already installed by `make setup tools` (a `tools.json` pin).

### 2.7 T0 (staged mode)

At T0 the runner passes only the staged `.go` files in scope (`gate/runner/scope.py:64-74`), and `expected` is their count. The row then:
- (a) lints the **packages** of the staged files: each staged file's directory as `./<dir>` (`.` for the root), deduplicated and sorted — never `./<dir>/...`, which recurses; golangci lints whole packages. A staged file under `testdata/` contributes no target (data).
- (b) counts covered as the staged files accounted for (§2.4's rules on the staged subset; `go list` still runs `./...`, so the file map is whole).
- (c) gives the **same verdict as T1 on the staged packages** with liveness (step 10) off: steps 1–9 and 11 unchanged, so a size finding in an unstaged file of a staged package is reported too (a real finding), and an exclusion still drops its package's findings.
- The T0 time is measured in W0 on a one-file stage and a ten-package stage (warm named cache, both GOOS) and recorded (§8 6.2). Both GOOS stay at T0 (decision 10; R-6).

## 3. Exclusions and the part-level check (lane B; entries by the coordinator)

### 3.1 `tools/allowlists/smells.json` (rule `smells`, the one grammar, `gate/runner/allowlists.py:1-8`)

```json
{"schema": 1, "note": "…", "rule": "smells", "entries": [
  {"target": "internal/runtime gocyclo", "kind": "TEMP", "reason": "size family not yet cleaned; S1.1c-6 cleans it",
   "sprint": "S1.1c-5", "expiry": "2026-12-14", "issue": "S1.1c-5-1"}
]}
```
- **One target form**, checked by the `smells` row: `<package dir> <rule>` — the package's directory relative to the root (`.` for the root package `cep`), `<rule>` ∈ `golangci.SIZE_RULES`. Any other form → `smells.json: <target>: not "<package dir> <rule>"`. One entry per package per size rule (a package clean for funlen but not gocyclo has funlen enter at once). Kind TEMP only (`smells.json: <target>: a size exclusion is TEMP, with an expiry and a known issue`); `issue` names a known issue whose `forward` is a printed part (§3.2). There is no `unlinted` form (R-1: testdata is data by rule, §2.4).
- `expiry` is the grammar's (≤ 90 days ahead, `allowlists.py:173-182`); the part-level check (§3.5) is the real deadline, the expiry the grammar's backstop. The coordinator writes the entries from lane A's real-tree run after W2 (§1), one per (package, rule) with ≥ 1 finding, each forwarded per §3.3.
- "Waives nothing" is the `smells` row's step 10 (§2.3, full-tree runs only); the grammar faults are the `suppressions` row's (`allowlists.check`, `rows/suppressions.py:195-196`), unchanged; the deadline and the move check are the `aging` row's (§3.5).

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
- `reached` (by row, `:74-75`) and `passed` (`:77-87`) are unchanged; `notbuilt` keeps refusing parts (`notbuilt.py:60-61`).
- Tests extend the existing `gate/runner/tests/test_sprintid_parts.py` (DA-L2: an edited file, written by QA in W0b, turned green by lane B).

### 3.3 Known issues (coordinator, W0a; `gate/known-issues.json` + the table, `gate/runner/knownissues.py:1-14`)

One issue per later part, so each exclusion's `issue` carries its part: `S1.1c-5-1` → forward `S1.1c-6`, `S1.1c-5-2` → `S1.1c-7`, `S1.1c-5-3` → `S1.1c-8` (the size family's debt by part), and `S1.1c-5-4` (the tool scan's 15 advisories, forward `""`, §6). Each: `first_seen` `S1.1c-5`, `correctness` false, `opened` the commit's UTC day, `decision` **`docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-design-after-the-reviews`** (owner, after the reviews, decision 2 names all four as whole words, so `aging.decided`, `aging.py:32-37`, holds). The table rows' Found cells begin `S1.1c-5`. Which package goes to which part is lane D's calibration output re-cut at close (decision 2 of the plan check): at W0a every size exclusion names `S1.1c-5-1` (forward S1.1c-6) except where the plan check's order already places it (runtime and validate S1.1c-6; archcheck after the `proofs` row, S1.1c-8 — inferred from that decision's order; the coordinator confirms when writing the entries).

### 3.4 Printing the parts (`gate/sprints.json`, the plan's §3 table; coordinator, W0a)

- The plan's §3 table gains four rows **after** the open `S1.1c` row: `S1.1c-5`, `S1.1c-6`, `S1.1c-7`, `S1.1c-8` (open), and `gate/sprints.json` the same four `{"id", "closed": false}` in the same place; `compare_rows` (`sprintid.py:185-192`) and `test_sprints_pin.py` keep them equal. The `S1.1c` row stays open until S1.1c-8 closes, so `first_open_mismatch` (`:195-199`) stays green with marker `S1.1c-5` and no code change. S1.1c-1…S1.1c-4 are **not** printed.
- What printing changes for each reader: `notbuilt` — nothing (rows only). `allowlists.sprint_fault` (`:145-153`) — `sprint: S1.1c-5` resolves to its own row. `knownissues` — `first_seen`/`forward` checked as rows (unchanged code). The marker test (`test_sprints_pin.py:33`) — green (row S1.1c open). `aging` rule 2 — a forward `S1.1c-6` is now a printed row: `_forward` takes the `rows` branch (`:57-58`) and reds only when the row S1.1c-6 is **closed**.
- **`aging` rule 1 (DA-S4, R-2; owner after the reviews, decision 2).** Rule 1 counts closed rows after `first_seen`'s position (`aging.py:46`); `index` places an unprinted part at its row (`sprintid.py:65-72`). All eight ids, and when each turns red:
  - **At S1.1c-7's close** (three closed rows after S1.1c: S1.1c-5, -6, -7): `S3.3-2-1`, `S3.3-2-2`, `S3.3-2-3` (no decision) and `S1.1c-3-4` (first seen S1.1c-3, at row S1.1c, no decision). The coordinator brings these four to the owner for a decision before S1.1c-7's close (owner, decision 2).
  - **At S1.1c-8's close**: `S1.1c-5-1`, `S1.1c-5-2`, `S1.1c-5-3`, `S1.1c-5-4` would reach three closed rows; their `decision` names the after-the-reviews section, so `aging.decided` holds and they stay green.
  - Test (fixture, owner decision 2): `test_aging.py :: PrintedParts.test_at_s1_1c_7s_close_aging_names_exactly_the_undecided` — sprints with the four parts printed and S1.1c-5…-7 ticked, today's known issues plus S1.1c-5-1…4: rule 1 names exactly `S1.1c-3-4`, `S3.3-2-1`, `S3.3-2-2`, `S3.3-2-3`, and none of S1.1c-5-1…4.

### 3.5 The part-level check (`gate/runner/parts.py`, new, lane B; run by the **`aging` row**)

Placement (DA-S3; owner after the reviews, decision 1 — DA-S2's "drop the check" is rejected): the check is aging for exclusions. `gate/runner/rows/aging.py` (lane B) calls it; the `aging` row already reads `gate/known-issues.json`, `gate/sprints.json`, `gate/current-sprint` and `docs/sprints/decisions.md` and counts them. The `suppressions` row does not call it and gains no side input (R-5 and R-9 dissolve).

```python
LAST_PART = "S1.1c-8"   # decision 1 of the plan check: never past S1.1c-8
LANDING = "S1.1c-5"     # the sprint whose own branch may add size entries without a decision

@dataclass(frozen=True) class Base:            # the state at the merge base, or why there is none
    commit: str                                 # "" when git.merge_base() returned ""
    entries: Sequence[allowlists.Entry] | None  # smells.json at the base; None when absent there
    issues: Mapping[str, Mapping[str, object]] | None  # known-issues.json at the base; None when absent there

def read_base(cwd: str | None = None) -> Base
    # commit = gate/runner/git.py merge_base(cwd) (the existing function, :23-29); "" → Base("", None, None).
    # Else `git show <commit>:tools/allowlists/smells.json` and `…:gate/known-issues.json` through git.git(); a
    # non-zero exit (absent at the base) → None for that file; a parse error → ValueError naming "<commit>:<path>".

def forwards(entries: Sequence[allowlists.Entry], issues: Mapping[str, Mapping[str, object]], order: sprintid.Order,
             marker: str, tier: str, base: Base, decided: Callable[[str, str], bool]) -> tuple[list[str], list[str]]
    # → (faults, lines): faults print "failed: aging: <text>", lines print plain
```
For each `smells.json` TEMP entry with a size target:
1. Its `issue` must exist with a non-empty `forward` → else `smells.json: <target>: issue <id> has no forward part`.
2. `forward` must be a printed part (`reached_part` raises) → `smells.json: <target>: forward <f> is not a printed part of gate/sprints.json`.
3. `order.index(forward) > order.index(LAST_PART)` → `smells.json: <target>: forward <f> is past S1.1c-8 (decision 1)`.
4. `reached_part(forward, marker)` and `tier in allowlists.RED_EXPIRED` (T4, nightly) → fault `smells.json: <target>: the marker <m> has reached its part <f>: clean <package> for <rule>, or the owner moves the forward`. At T3 (the aging row's other tier) the same text is a plain line, never red — so a part's own branch is red at T4 from its G0 until the package is clean (the `notbuilt` precedent).
5. **The move check, per exclusion target** (DQ-B1; owner after the reviews, decision 1). Skipped when `base.commit == ""` (rule 6). The **effective forward** of a target is the `forward` of its entry's `issue` in the matching file (`base.entries` + `base.issues` at the base; `entries` + `issues` now). For each target now:
   - (a) present at the base: if `order.index(forward_now) > order.index(forward_base)` (later, including an entry re-pointed to a later issue, or the issue's forward moved), it is red unless the current issue's `decision` resolves through `decided(issue_id, decision)` (`aging.decided`, `aging.py:32-37`: a decisions.md section naming the id as a whole word) **and** that `decision` differs from the base issue's `decision` → `smells.json: <target>: forward moved <old> → <new> (issue <base id> → <id>) without a decision that names <id> (decision 1: only the owner moves a forward)`. Earlier or equal → silent.
   - (b) absent at the base (a new entry, or `base.entries is None`): red unless `marker == LANDING` → `smells.json: <target>: a new size exclusion after S1.1c-5: clean the package instead (decision 1)`.
   - A target present at the base and absent now (removed) → silent.
6. **No merge base** (DQ-B3): `base.commit == ""` at a tier in `allowlists.RED_EXPIRED` → fault `smells.json: no merge base with origin/main or main: forward moves not checked (decision 1)`; at T3 → plain line `aging: no merge base; forward moves not checked`. The base file absent at a resolved base is not "no base": the known-issues file absent → every target is judged by 5(b); `smells.json` absent → every target is new (5b).
- Called from `rows/aging.py` `main` after its existing rules, with the parsed lists it loaded, `os.environ["GATE_TIER"]`, `read_base()` and `aging.decided`. A `read_base` ValueError is one named fault.
- **Registry and law (lane B):** the `aging` row's `expected_scope` gains `"tools/allowlists/smells.json"` (so `GATE_COVERED` counts it, read through the same `files.nul` loop, `rows/aging.py:38-46`); its `gate/law.json` instrument `code` becomes the closure of `rows/aging.py` (+ `parts.py`, `git.py`, `allowlists.py`, `sprintid.py`, `knownissues.py`, §2.5 rule). Aging's tiers stay `T3`, `T4`, `nightly`; the countdown line shows at T3 only, which is enough (DA-S3).

## 4. Waivers (lane B)

### 4.1 The census (W0b, QA)

- **Procedure** (DQ-S6), recorded as data in `gate/runner/testdata/directives.json`: `[{"linter", "form", "placement", "honoured": bool, "source": "<module>@<version> <file>:<line>" | "none in source at <module>@<version>"}]`, which `test_suppressions.py` reads as it holds the `//nolint` tables today.
  1. For each enabled linter (`golangci.LINTERS`), at the version golangci v2.15.0 vendors (its `go.mod`), and for golangci's own processors (nolint, exclusions, `//line` handling): read the source's comment-directive parsing and record the source reference per form.
  2. Confirm each form with the pinned binary and the §2.1 config: a one-package scratch module with one finding of that linter per file and one candidate directive per file, in each placement — above the line, trailing, file-wide before `package`, block form `/* */` — and each case and spacing variant (no space, one space, two spaces, a tab, `//<TAB>`, upper case, a trailing comma, a trailing `// x`). A directive is **honoured** when its finding disappears against the control file.
  3. Done = every enabled linter has at least one row, including `"none in source at <ref>"`.
- **Known so far:**
  - `//nolint` itself (M-9): honoured `// nolint:lll`, `//nolint: lll`, `//nolint:LLL`, `//  nolint:lll`, `//nolint:lll,`, bare `//nolint`, `//nolint:lll // x`; ignored `//NOLINT:lll`, `/*nolint:lll*/`, `//lint:ignore lll x`. Every honoured spelling but the last is red through nolintlint; `//nolint:lll // x` is red only through the suppressions checker's metadata rule. Each honoured spelling is asserted red in at least one named test (2.4, planted case 18).
  - revive honours `//revive:disable-next-line[:rule]`, `//revive:disable-line[:rule]` (also trailing, with text after), `//revive:disable` … `//revive:enable`, `// revive:…` and `//<TAB>revive:…` (its regex `^//\s*revive:`), not `/* revive:… */`; gosec's `#nosec` in nine spellings incl. file-wide (gosec is not enabled, so they waive nothing today but are refused anyway, decision 7); `//gocyclo:ignore` silences gocyclo (`// gocyclo:ignore` does not); `//nolint:all`. lll skips `//go:` lines and import blocks: named residue (decision 7).
  - errcheck, funlen, gocritic, forbidigo, lll and nolintlint's own forms: W0 measures by the procedure; each honoured form becomes a `directives.json` row, and 2.1 refuses it data-driven — no code change per form beyond `go_foreign`'s matcher and `PREFILTER`.

### 4.2 `gate/runner/suppressions.py` changes

- `ENABLED` (`:21`) is removed as a constant; `rows/suppressions.py` passes `golangci.enabled(golangci.load(...))` to `check` (`:296`), so the liveness text becomes `//nolint:<linter>: <linter> is not enabled in .golangci.toml` (`:310`'s "(none is until S1.1c-5)" goes). A pin test: the set equals `golangci.LINTERS` both ways.
- `PREFILTER` (`:24`) += `"revive:"`, `"nosec"`, `"gocyclo:"`, `"line "` and each W0 form's literal.
- **A `//nolint` must trail the code it waives** (G1 ruling on the per-line rule, DA-S8): a `//nolint` on a line holding only a comment — which golangci applies to the whole next declaration, or the whole file above the package clause — is a named fault: `<file>:<line>: //nolint stands alone on its line: it would waive the whole next declaration; put it at the end of the line it waives`. The review grep of v2 is replaced by this rule.
- New recogniser `go_foreign(path, source) -> tuple[list[Directive], list[str]]` beside `go_directives` (`:152`): over `go_comments` (and block comments), each honoured census form → a `Directive(path, line, (<linter>,), "FOREIGN", {"form": <text>}, valid=False)` (counted in the LAW Δ) and a fault `<path>:<line>: <linter> honours this form; the only waiver is //nolint:<linter> // PERMANENT|TEMP … (decision 7)`. Matching follows the measured regexes: revive `^\s*revive:` after `//`; gosec `#nosec` at a comment's start after `//` or `/*` plus optional spaces, with a glued suffix; gocyclo `gocyclo:ignore` with no space after `//`.
- **`//line` refused** (DQ-B2): in `go_foreign`, a line comment starting exactly `//line ` or a block comment starting exactly `/*line ` (the compiler's forms) → fault `<path>:<line>: a //line directive moves findings between files; not allowed (S1.1c-5)`. The tree holds none (M-10).
- **Non-canonical `//nolint` spellings** (M-9): in `go_directives`, a comment that golangci reads as a nolint but is not the accepted form (`// nolint`, `//nolint: `, upper-case linter names, a trailing comma, bare `//nolint`) → fault `<path>:<line>: write //nolint:<linter> exactly (two slashes, no space, lower case, one list)`.
- `//nolint:all` and `//nolint:typecheck` → fault `write the linter: //nolint:all waives every linter` / `typecheck cannot be waived`, in `go_directives`.
- **Per line, never per function** (DA-S8): a waiver is trailing on the line it waives. A `//nolint` comment alone on its line applies to the whole next declaration or block in golangci (inferred from golangci's documented behaviour; W0's census confirms it in placement "above the line") and so would waive every pattern of that linter in a function body. Every waiver this sprint writes is per line; the coordinator's review checks it in each lane's diff (`git diff | grep -n '^+\s*//nolint'` must be empty).
- The accepted form (DEVELOPMENT.md:68-74, quoted): `//nolint:<linter>[,<linter>]` "two slashes, no space; exactly one of PERMANENT or TEMP; a `reason="…"` of at least 12 characters; PERMANENT carries `ref=<repository path>#<heading anchor>` … TEMP carries `expiry=YYYY-MM-DD`, at most 90 days ahead, and `issue=<id>` …; every suppression carries `sprint=<sprint id>`". Example the twins use: `//nolint:lll // PERMANENT reason="a URL that cannot wrap" ref=docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md#2-architecture-constraints-as-analyzers-with-teeth-reference-a-in-go sprint=S1.1c-5`.
- **A waiver on the wrong line**: nolintlint's `allow-unused = false` reports an unused `//nolint` (red through the `smells` row), and the real finding stays red (2.9, planted case 17).
- **Restore BK-S4's tests** (DQ-S7; the coordinator verified the source): `NosecIsAWaiver.test_a_nosec_is_read` and `NosecIsAWaiver.test_a_nosec_is_found_by_lawdeltas_prefilter` come back into `gate/runner/tests/test_breakit_s11c2.py` from `git show 96b76f4:gate/runner/tests/test_breakit_s11c2.py` (96b76f4 is on `main`; `fc944ef` is a pre-rebase hash), adjusted only to the new fault text. Lane B also corrects the module docstring's `fc944ef` citation to `96b76f4`.

## 5. The ruff pin (lane B; `gate/runner/rows/suppressions.py:85-112`)

- `RUFF_PINNED: dict` in `rows/suppressions.py` = `ruff.toml` as `tomllib` parses it today **minus** `lint.per-file-ignores` (its value is checked both ways against `tools/allowlists/ruff.json`, as now, `:107-112`).
- `ruff_pin` becomes: parse; take `lint.per-file-ignores` out; `pin.differences(doc, RUFF_PINNED, "ruff.toml")` (§2.2; type-strict, indexed paths) → each line a fault; then the existing both-ways check. The current deny-list checks (`extend`, top-level and `lint` ignores, `:99-106`) become redundant but stay as clearer texts.
- Re-measured on 0.17.5 (M-5): every hole the pin must close reproduces; the tests are §8's 4.1–4.4.
- Churn named: a new `py312` file now needs `per-file-target-version` **and** `RUFF_PINNED` edited (both law paths).

## 6. The tool scan (lane C)

- **`tools.json`**: an executable tool's entry may carry `"go_built": true` (only `true` is legal; `false` → `tools.json: tools[i] (<name>): "go_built": false is the default: remove it`; a non-bool → `… "go_built" must be true`). `manifest._TOOL_FIELDS` (`gate/toolchain/manifest.py:34`) admits it optionally (as `kind`, `:186`); `Tool.go_built: bool = False`; a `python-package` with it is refused. Set on `golangci-lint` only (`go` itself is scanned as the engine's toolchain); a pin test holds it there (`test_golangci_lint_is_declared_go_built`, DQ-S8). The canary's `candidate.build` copies each pinned tool dict and replaces only version/url/digests/version_line (`gate/canary/candidate.py:283-300`, inferred), so the field is carried; a test proves it.
- **`vulndb.findings`** (`gate/runner/vulndb.py:234`) gains `*, pseudo: frozenset[str] = GO_PSEUDO`; the binary scan passes `frozenset({"stdlib"})`. `pseudo` narrows only **the judgement** (`vulndb.py` ~`:276`: which pseudo-modules a finding is judged against), never the database entry's shape check (~`:271`): a `toolchain` entry stays well-formed (DA-S7). Twin test: the engine scan's default still yields `toolchain` findings.
- **`rows/vulns.py`**: for each `go_built` tool, `go version -m <tool_home>/<path_dir>/<command>` (the run's own platform's binary — CI's T4 and nightly read linux-amd64, a local T4 darwin-arm64). Parse line 1 `<file>: go<version>`; the main-module line `\tmod\t<path>\t<version>\t<sum>` where **`<sum>` may be empty** (M-13: `\tmod\tgithub.com/golangci/golangci-lint/v2\tv2.15.0\t`; split on `\t`, require ≥ 4 fields after the leading empty one, the sum optional); `\tdep\t<path>\t<version>[\t<sum>]`; `\t=>\t<path>\t<version>[\t<sum>]` replacements. Red, named: exit ≠ 0; no Go version or `go_version` raising (`:155`); no `mod` line; `mod` version ≠ the tool's pinned `version`; zero `dep` lines (vacuous). Then `findings({dep path: version (replacement applied)}, frozenset(), <go>, entries, pseudo=frozenset({"stdlib"}))`; every finding's pair is `<id> <tool name>`. `NOT_READ` (`:43-46`) drops golangci-lint. Covered = 6 + 1 per declared binary (`COVERED`, `:33`); the registry row's and the law instrument's `min_wants` **6 → 7** (DQ-S8), `expected_scope` += `tools.json`, the instrument's `code` += `gate/toolchain/manifest.py` (§2.5 rule).
- Test data: `s1-1c-5-scratch/golangci-lint.buildinfo.txt` (darwin) copied into `gate/runner/testdata/` as the darwin fixture; W0's linux record as the linux fixture.
- **Exemptions**: `tools/allowlists/vulns.json` += 15 TEMP entries `GO-2026-6599 golangci-lint` … `GO-2026-6859 golangci-lint` (research §C's list), `expiry` = commit day + 30, `issue` `S1.1c-5-4`, `sprint` `S1.1c-5`; `PAIR` (`:36`) already accepts the tool name. Renewal only by an owner decision (decision 8; LAW Δ shows it).
- **An exemption that matches no finding is red** (DQ-S9): W0 checks whether `rows/vulns.py`/`vulndb.py` already has this rule (and cites it in the report); if not, lane C adds it: after a complete scan, each `vulns.json` entry whose `<id> <module|tool>` pair matched no finding → `tools/allowlists/vulns.json: <target> waives nothing; remove it`, judged only when the scan completed (the `complete` rule, `rows/vulns.py:276-278`). Test 5.5.
- **Residue** (DA-L4): locally only the host platform's golangci-lint binary is scanned; CI's T4 and nightly scan linux's. Both come from one release build.

## 7. The whole-tree fixes (lane C) and lane D

- **7.1 Lane C's finding list** (research §B, RM counts; lane C re-runs the row first and works from its list):
  - lll — 173 production + 31 tools lines (tests exempt).
  - errcheck — `tools/archcheck/main.go:1643, 1646, 1661, 1681, 1686, 1688` and the 18 test sites (`boundary_test.go:35-42, 53, 54`; `example_test.go:24`; the five `Close` sites; `tools/apipartition/main_test.go:58, 62`).
  - gocritic appendAssign, `internal/playback/recorder.go:55, 71, 120, 140, 166, 199` (R-4, coordinator): lane C **first** writes the aliasing test `internal/playback/recorder_test.go :: TestRecordsDoNotAlias` — two records, the reused buffer changed after the first, the first record unchanged — shown red against a naive rewrite (`buf = append(other[:0], …)` with the alias broken the wrong way) in the record. Then lane C rewrites each site where that test and the playback tests stay green; a site where no rewrite keeps both green gets a per-line `//nolint:gocritic // PERMANENT reason="<the reuse idiom at this site>" ref=<the test's path>#… sprint=S1.1c-5` citing `TestRecordsDoNotAlias`. `internal/table/breakit_test.go:77`, `table_test.go:292` are lane D's.
  - forbidigo — the 15 production panics (research §B list) each `//nolint:forbidigo // PERMANENT reason="<its invariant>" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-5`, `row.go:73`'s reason naming MUST 14.
  - The canonical encoder (decision 3; DA-S8): a pure-move commit takes `internal/model/model.go:397-447` to **`internal/model/canonical.go`** (AST-checked, §7.4); a following commit adds a **per-line** waiver on each `reflect` use (about 33): `//nolint:forbidigo // PERMANENT reason="the canonical encoder walks the IR generically" ref=docs/architecture/adr/ADR-0003-<file>.md#<§2 heading anchor> sprint=S1.1c-5`. Never a function-level `//nolint` (§4.2).
  - The 28 test panics and `internal/runtime/breakit_test.go:80` (`BK_CHILD`) as per-line PERMANENT waivers (decision 4).
  - The golden-update switches (`internal/parser/golden_test.go:16`, `internal/playback/baseline_test.go:52`, `s3_2b_2_corpus_test.go:198, 291, 301`) become test flags (`var update = flag.Bool("update", false, …)` per package). Before the change lane C runs `git grep -n -E 'CEP_UPDATE_GOLDEN|CEP_BASELINE_WRITE|CEP_TRIPWIRE_FILE|CEP_DETERMINISM_PLANT' -- gate Makefile tools docs` and lists every caller in its report; each caller that sets the variable changes in the same commit to pass the flag (`go test … -args -update`), and a variable that is a child-process marker rather than an update switch (`CEP_TRIPWIRE_FILE`, `CEP_DETERMINISM_PLANT`, inferred) stays an env read with a per-line PERMANENT waiver.
- **7.2 Lane D's packages**: `internal/table` (gocyclo `New` 13, `Table.checkValue` 13; lll 1), `internal/clock` (`RuleClock.fireGroup` 11; panic `rule.go:124` waiver), `io/inmemory` (`Source.Open` 11; lll 1), `tools/shape` (`shape` 19, `shapes` 11).
- **7.3 Refactor rules**: split by concern, each new function named for what it does; no `xPart1..n`, no option struct whose only purpose is to pass > 5 values (a struct that already exists as a domain type may be passed); no behaviour change in the same commit as a move; tests are not reflowed in a refactor commit.
- **7.4 The AST-equality check** (`tools/astequal/main.go`, lane D; stdlib `go/parser`, `go/ast`, `go/token`; package main; a law path under `tools/`):
  - `astequal <before-rev> [<paths>]` reads each file at `git show <rev>:<path>` and in the work tree and parses both with `parser.ParseComments`.
  - **Canonical form** (DQ-S10): a deterministic printer walks each declaration with `ast.Inspect` and prints, per node, its Go type and **every field except `token.Pos` fields** (so `Ident.Name`, `BasicLit.Kind`/`Value`, `BinaryExpr.Op`, `UnaryExpr.Op`, `AssignStmt.Tok`, `BranchStmt.Tok`, `IncDecStmt.Tok`, `ChanType.Dir`, `GenDecl.Tok` and the rest); comments are dropped **except** `//go:` directives, which are printed with the declaration they precede (and a file's `//go:build` line with the file's package).
  - **Declaration keys**: functions by name; methods by `(<receiver type without pointer-ness and type parameters>).<name>` plus the pointer flag; `init` functions and `_` value specs numbered in source order within the package (`init#1`, `_#2`); types, consts and vars by name. A pure move across files of one package compares the package's union of keyed declarations; a declaration that changes package is unequal.
  - Exit 0 equal; 1 naming the first differing key; 2 on a parse or git error.
  - Its own Go tests (`tools/astequal/astequal_test.go`, §8 7.2) run at **T2** with the tree's Go tests (DA-S9, R-7).
  - It is not a gate row (R-7). Lane D runs it per move/reflow commit and quotes it in the record; **at G5 the coordinator re-runs `astequal <commit>^` for each lane-D move/reflow commit and quotes the exit codes in the record** (ground rule 17).
  - **Scope of the proof**: AST equality proves moves and reflows only. A split changes the AST by design; splits are proven by the characterisation tests (§7.5) and the T4 run.
- **7.5 Characterisation and performance**:
  - `tools/shape` (DQ-S11): `tools/shape/shape_test.go` first, before the first refactor commit — `TestShapeCorpusGolden` (every corpus file's shape output pinned as golden) and `TestShapeEachConstructKind` (one row per case arm of `shape`'s and `shapes`' switches). Completeness = 100 % statement coverage of `shape` and `shapes` by `go test -coverprofile` + `go tool cover -func`, quoted in the record; plus one mutation per switch arm (the arm's result changed by hand in a scratch copy) shown red in the record. The construct list is exactly the switch arms the coverage run reaches; no arm is left without a row.
  - Hot paths (DQ-S12): W0 checks whether the runtime's per-event flow reaches `Table.checkValue` or `RuleClock.fireGroup` (GitNexus `impact` upstream, or `git grep` of their callers up to `internal/runtime`). If either is reached, lane D pins `BenchmarkTableCheckValue` / `BenchmarkRuleClockFireGroup` and `TestCheckValueAllocs` / `TestFireGroupAllocs` (`testing.AllocsPerRun` at today's value) **before** the split (decision 6); a split that costs time or allocations is reverted and the function stays whole as a per-line PERMANENT waiver with the measurement as evidence.
- **7.6 The calibration record**: per package, the start and end time of each commit, findings cleared by kind, and hours per finding kind, as a table in `s1-1c-5-record.md`; the coordinator re-cuts S1.1c-6…-8 from it at close (decision 2 of the plan check).

## 8. Tests and proofs, red first (QA, W0b)

QA's finished table ([review §1](../reviews/s1-1c-5-design-qa.md)), adjusted for v2: the part-level tests are driven through `rows/aging.py` (3.2–3.4 test `parts.forwards`; 3.10 the row's call), the `unlinted` form is gone (R-1), and the dispositions' tests are added (marked v2). Module paths are relative to the repo root. "stub" = the W0c `NotImplementedError` stubs; "today" = the current code. A test green from its first run gets a mutation proof instead.

### Card line 1 — the row's rules bite (lane A turns green unless noted)

| # | Test (module :: name) | Red before code because | Green by |
|---|---|---|---|
| 1.1 | `gate/runner/tests/test_golangci.py` :: `PinnedDocument.test_the_committed_config_equals_pinned` | stub | A |
| 1.2 | `test_golangci.py` :: `PinnedDocument.test_each_hiding_edit_is_named` (subTests: `issues.new-from-rev`, `issues.new-from-patch`, `issues.whole-files`, `run.tests=false`, `max-issues-per-linter` removed, `max-same-issues` removed, `uniq-by-line=true`, `generated="lax"`, `funlen.statements=-1`, `exclude-godoc-examples` removed, `exclusions.paths=["x"]`, a fifth `exclusions.rules` entry, `run.build-tags=["x"]`, `output.formats.json.path`, `severity.default="warning"`, `errcheck.disable-default-exclusions` removed, one `exclude-functions` entry removed, `enable` minus `revive`) | stub | A |
| 1.3 | `test_golangci.py` :: `PinnedDocument.test_a_type_change_is_a_difference` — `arguments=[5.0]`, `max=500.0`, `run.tests=1`, `funlen.lines=60.0` (DQ-S1) | stub; **and Python `==` says equal (M-11)** | A |
| 1.4 | `test_golangci.py` :: `PinnedDocument.test_a_duplicate_key_is_a_config_error`, `test_an_upper_case_twin_key_is_not_pinned` (`[LINTERS]`) | stub | A |
| 1.5 | `test_golangci.py` :: `ReadReport.test_issues_error_and_warnings_are_read` (recorded JSON from qa-design-exp/m1: `Warnings=[{"Tag","Text"}]`, `Error="setup revive…"`), `test_null_issues_is_empty`, `test_a_truncated_document_is_a_report_error` | stub | A |
| 1.6 | `test_golangci.py` :: `RuleOf.test_reviews_two_size_rules_by_prefix`, `test_another_revive_prefix_is_revive` | stub | A |
| 1.7 | `gate/runner/tests/test_rows_smells.py` :: `Exit.test_exit_0_with_an_issue_is_red`, `test_exit_1_with_nothing_is_red`, `test_exit_3_is_red_with_stderr`, `test_no_report_file_is_red`, `test_a_report_left_by_an_earlier_run_is_never_read` (DQ-S2), `test_a_truncated_report_is_a_named_fault` | stub | A |
| 1.8 | `test_rows_smells.py` :: `Health.test_report_error_is_red`, `test_a_warning_is_red`, `test_enabled_short_by_one_is_red`, `test_enabled_without_typecheck_is_red`, `test_a_typecheck_issue_in_an_excluded_package_is_red` | stub | A |
| 1.9 | `test_rows_smells.py` :: `Merge.test_a_finding_on_both_goos_counts_once`, `test_a_linux_only_finding_is_kept` | stub | A |
| 1.10 | `test_rows_smells.py` :: `Drops.test_fmtprint_in_a_main_tool_is_dropped`, `test_fmtprint_in_a_non_main_tools_package_is_red`, `test_fmtprint_outside_tools_in_package_main_is_red` (cmd/), `test_fmtprint_in_a_main_tools_xtest_is_red` (v2, DA-S6: `Name + "_test"`), `test_a_generated_json_target_is_dropped`, `test_an_issue_at_a_file_no_package_claims_is_red` (DQ-B2) | stub | A |
| 1.11 | `test_rows_smells.py` :: `Env.test_go_list_and_golangci_share_one_env_per_platform` (captured env of both calls equal per platform: GOOS/GOARCH, `CGO_ENABLED=0`, `GOFLAGS=-mod=readonly`, `GOPROXY=off`, `GOTOOLCHAIN=local`, `GOWORK=off`; DA-S1, DQ-S13), `test_the_lint_cache_is_named` (`GOLANGCI_LINT_CACHE` under the tool cache root, outside the repo; DA-L1, DQ-S4), `test_the_root_is_realpathed` (v2, DQ-L2) | stub | A |
| 1.12 | `test_rows_smells.py` :: `Unsafe.test_imports_test_imports_and_xtest_imports_are_each_red` | stub | A |
| 1.13 | `gate/proofs/drivers/test_smells_planted.py` (real binary, real row, fresh `GOLANGCI_LINT_CACHE` per world; 30 cases, list below) | the row does not exist (no `smells` row id: the gate refuses the run) | A |
| 1.14 | v2 (R-6) `test_rows_smells.py` :: `Staged.test_each_staged_files_package_is_linted_once` (`./<dir>`, deduplicated, never `/...`), `Staged.test_t0_gives_t1s_verdict_on_the_staged_packages`, `Staged.test_no_new_or_whole_files_flag_at_t2_and_up` (the row's argv at every tier and `.github/workflows/gate.yml` hold no `--new*`/`--whole-files`) | stub | A |
| 1.15 | v2 (DQ-L4) `test_rows_smells.py` :: `Summary.test_the_green_line_format_is_pinned` | stub | A |

### Card line 2 — only `//nolint` waives (lane B)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 2.1 | `gate/runner/tests/test_suppressions.py` :: `Foreign.test_each_honoured_census_form_is_refused` (data-driven over `gate/runner/testdata/directives.json`, one subTest per `honoured: true` row) | `go_foreign` stub; today nothing reads `revive:`/`#nosec`/`gocyclo:` | B |
| 2.2 | `test_suppressions.py` :: `Foreign.test_each_unhonoured_form_is_not_a_fault` (the `honoured: false` rows: `/* revive:… */`, `// gocyclo:ignore`, `//NOLINT:lll`, `/*nolint:lll*/`, `//lint:ignore`) | stub | B |
| 2.3 | `test_suppressions.py` :: `Foreign.test_the_prefilter_finds_every_honoured_form` | PREFILTER lacks them today | B |
| 2.4 | `test_suppressions.py` :: `Nolint.test_a_standalone_nolint_is_refused` (above a func, above the package clause; its twin: the same waiver trailing its line passes — G1 ruling), `Nolint.test_nolint_all_is_refused`, `test_nolint_typecheck_is_refused`, `test_a_nolint_variant_golangci_honours_is_refused` (M-9's seven honoured spellings) | today `ENABLED` is empty so every one is "not enabled" — the test asserts the **specific** text | B |
| 2.5 | `test_suppressions.py` :: `Nolint.test_an_accepted_lll_waiver_passes` (the §4.2 example, verbatim) | **red today**: `ENABLED` empty makes `lll` "not enabled" | B |
| 2.6 | `test_suppressions.py` :: `Enabled.test_enabled_equals_the_config_both_ways`, `test_a_linter_not_in_the_config_is_not_enabled_text` | stub (`golangci.enabled`) | B (codes against A's stub) |
| 2.7 | `test_suppressions.py` :: `LineDirective.test_a_line_directive_is_refused` (`//line x.go:1`, `/*line x.go:1*/`) (DQ-B2) | today no such check | B |
| 2.8 | `gate/runner/tests/test_breakit_s11c2.py` :: `NosecIsAWaiver.test_a_nosec_is_read`, `NosecIsAWaiver.test_a_nosec_is_found_by_lawdeltas_prefilter` — restored from **`git show 96b76f4:`** (DQ-S7, coordinator-verified) | `go_directives`/PREFILTER do not read `#nosec` (the BK-S4 red) | B |
| 2.9 | `test_smells_planted.py` :: `test_a_waiver_on_the_wrong_line_is_red_and_the_finding_stays` (planted case 17) | row absent | A |
| 2.10 | `gate/proofs/drivers/test_suppressions_planted.py` += `test_a_revive_directive_is_red`, `test_a_nosec_is_red`, `test_a_line_directive_is_red`, `test_an_accepted_nolint_with_metadata_passes` (9 → 13) | today all four are green/unread | B |

### Card line 3 — exclusions are bounded (lane B for parts and the aging row, lane A for the row's use)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 3.1 | `gate/runner/tests/test_sprintid_parts.py` (existing, edited) :: `ReachedPart.test_the_design_table` (subTests: §3.2's six), `test_a_part_less_marker_at_the_parts_row_is_false`, `test_an_unprinted_part_raises`, `test_a_row_id_as_part_raises`, `test_parts_compare_as_numbers` (S1.1c-6 vs S1.1c-10 printed) | stub | B |
| 3.2 | `gate/runner/tests/test_parts.py` :: `test_no_issue_is_red`, `test_an_empty_forward_is_red`, `test_an_unprinted_forward_is_red`, `test_a_forward_past_s1_1c_8_is_red`, `test_reached_at_t4_is_red`, `test_reached_at_nightly_is_red`, `test_reached_at_t3_is_a_line_not_red`, `test_not_reached_is_silent` | stub | B |
| 3.3 | `test_parts.py` :: `Moves.test_a_forward_moved_without_a_decision_is_red`, `test_moved_with_a_naming_decision_is_green`, `test_moved_with_the_base_decision_is_red`, `test_moved_earlier_is_green`, **`test_an_entry_repointed_to_a_later_issue_is_red`**, **`test_a_size_entry_absent_at_the_base_is_red_after_s1_1c_5`**, `test_a_size_entry_absent_at_the_base_is_green_at_s1_1c_5`, `test_a_removed_entry_is_green` (DQ-B1; owner decision 1) | stub | B |
| 3.4 | `test_parts.py` :: `Base.test_no_merge_base_at_t4_is_red`, `test_no_merge_base_at_nightly_is_red`, `test_no_merge_base_below_t4_is_a_line`, `test_the_base_file_absent_is_judged_as_new_entries`, `ReadBase.test_a_fixture_repo_with_a_base`, `test_without_a_base`, `test_with_the_file_absent_at_the_base` (fixture git repos in a temp dir; `git.merge_base`) (DQ-B3) | stub | B |
| 3.5 | `test_rows_smells.py` :: `Exclusions.test_an_entry_drops_only_its_package_and_rule`, `test_a_subpackage_is_not_dropped`, **`test_a_root_package_entry_drops_a_root_finding`** (DQ-S3), `test_an_expired_entry_drops_nothing_at_t4`, `test_an_entry_that_drops_nothing_is_red`, `test_waives_nothing_is_not_judged_after_a_fault`, **`test_waives_nothing_is_not_judged_on_a_staged_run`** (DA-B1, DQ-S5: a T0 run with one staged clean package and an unrelated exclusion is green and prints `smells: exclusions not judged at T0 (staged packages only)`), `test_a_bad_target_form_is_red`, `test_a_permanent_size_entry_is_red`, `test_an_unlinted_target_is_a_bad_form` (v2, R-1) | stub | A |
| 3.6 | `test_rows_smells.py` :: `Covered.test_an_unclaimed_file_is_red`, `test_ignored_under_both_goos_is_red`, `test_a_nested_go_mod_is_red`, `test_a_linux_only_file_counts_once`, `test_a_testdata_file_is_counted_as_data` (R-1), `test_a_testdata_segment_must_be_whole` (`mytestdata/x.go` is not data), `test_a_file_claimed_twice_is_red`, `test_a_package_error_is_red`, `test_covered_equals_tracked_when_all_accounted` | stub | A |
| 3.7 | `test_smells_planted.py` :: planted cases 27, 28, 29 | row absent | A |
| 3.8 | `gate/runner/tests/test_sprints_pin.py` :: `test_the_four_parts_are_printed_after_s1_1c` (and `known-issues` S1.1c-5-1…-4 present with forwards S1.1c-6/-7/-8/"" and `decision` the after-the-reviews section) | W0a not yet committed | coordinator (W0a) |
| 3.9 | `gate/runner/tests/test_allowlists.py` :: `test_lane_d_packages_have_no_size_entry` (`internal/table`, `internal/clock`, `io/inmemory`, `tools/shape`) | green on an empty file — **mutation proof**: plant one entry, red | QA writes; holds through G1→T3 |
| 3.10 | v2 (DA-S3) `gate/runner/tests/test_aging.py` :: `Parts.test_the_aging_row_calls_forwards_with_its_tier_and_base`, `test_smells_json_is_counted_in_covered`, `test_a_read_base_error_is_one_named_fault` | stub | B |
| 3.11 | v2 (DA-S4, owner decision 2) `test_aging.py` :: `PrintedParts.test_at_s1_1c_7s_close_aging_names_exactly_the_undecided` (§3.4) | W0a's printing absent from the fixture's source data until written | B (fixture by QA) |

### Card line 4 — the ruff pin is closed (lane B)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 4.1 | `gate/runner/tests/test_rows_ruff.py` :: `RuffPin.test_the_committed_ruff_toml_equals_pinned` | stub `RUFF_PINNED` | B |
| 4.2 | `test_rows_ruff.py` :: `RuffPin.test_each_hiding_key_is_named` (subTests: `builtins`, `lint.dummy-variable-rgx`, `lint.pycodestyle.max-line-length`, `line-length`, `lint.select` narrowed, `lint.extend-select` removed, `per-file-target-version` entry added, `lint.flake8-bugbear.extend-immutable-calls`, `force-exclude`+`extend-exclude`, `format.exclude`) | **red today** for every key but the deny-list's (M-5) | B |
| 4.3 | `test_rows_ruff.py` :: `RuffPin.test_per_file_ignores_still_checked_both_ways`, `test_a_type_change_is_named` (`line-length = 100.0` vs pinned int) | stub | B |
| 4.4 | `gate/runner/tests/test_pin.py` :: `test_added_removed_changed_paths`, `test_type_strict` (bool/int/float distinct, recursively), `test_list_order_counts`, `test_dotted_key_quoted`, `test_a_list_path_is_indexed` (v2, DQ-L1) | new module, stub | B |

### Card line 5 — declared tools are scanned (lane C)

| # | Test | Red before because | Green by |
|---|---|---|---|
| 5.1 | `gate/toolchain/tests/test_manifest.py` :: `GoBuilt.test_true_is_accepted`, `test_false_is_refused`, `test_a_non_bool_is_refused`, `test_on_a_python_package_is_refused`, **`test_golangci_lint_is_declared_go_built`** (DQ-S8) | today the field is an unknown key (refused) | C |
| 5.2 | `gate/canary/tests/test_candidate.py` :: `test_the_candidate_carries_go_built` | stub/unknown key | C |
| 5.3 | `gate/runner/tests/test_vulndb.py` :: `Pseudo.test_a_binary_scan_never_judges_toolchain`, `test_the_engine_scan_still_judges_toolchain` (twin), `test_a_binary_scan_judges_stdlib`, `test_a_toolchain_entry_stays_well_formed_under_a_binary_scan` (v2, DA-S7) | `pseudo=` absent (TypeError) | C |
| 5.4 | `gate/runner/tests/test_vulns_row.py` :: `BuildInfo.test_exit_nonzero_is_red`, `test_no_go_line_is_red`, `test_devel_go_is_red`, `test_an_experiment_suffix_is_parsed_or_red` (`go1.27.0 X:boringcrypto`), `test_no_mod_line_is_red`, `test_an_empty_main_module_sum_is_accepted` (v2, DA-S7: the recorded `golangci-lint.buildinfo.txt`), `test_mod_version_not_the_pin_is_red`, `test_zero_deps_is_red`, `test_a_replacement_is_applied`, `test_the_pair_names_the_tool`, `test_covered_counts_each_declared_binary` (recorded `go version -m` texts: darwin from M-13's file, linux from W0's fetched archive) | stub | C |
| 5.5 | `test_vulns_row.py` :: `Exempt.test_an_exemption_matching_no_finding_is_red` (DQ-S9) | stub, or today's code if W0 finds the rule (then cited) | C |
| 5.6 | `gate/proofs/drivers/test_vulns_planted.py` += `test_a_declared_binary_with_a_stdlib_advisory_is_red`, `test_an_exemption_scoped_to_the_binary_is_green`, `test_a_toolchain_advisory_never_applies_to_a_binary`, `test_a_declared_binary_without_build_info_is_red` (31 → 35) | the scan does not exist | C |

### Law code lists (DA-S5, DQ-L3)

| # | Test | Red before because | Green by |
|---|---|---|---|
| L.1 | v2 `gate/runner/tests/test_instruments_pin.py` :: `CodeClosure.test_each_instruments_code_is_its_import_closure` (every instrument in `gate/law.json`; closure from `ast` imports of `gate.*` modules, compared both ways) | red today for `aging` (its row imports `knownissues`, `sprintid`, absent from `code`) and on the stubs for `smells`, `suppressions`, `vulns` | A (smells), B (aging, suppressions), C (vulns) — each lane its own entries; the coordinator fixes any pre-existing instrument in W0a |

### Card lines 6–7 — real tree, no behaviour change (lanes C, D; coordinator)

| # | Proof | Fails when | Owner |
|---|---|---|---|
| 6.1 | T4 run: `smells` green, `covered == expected` (260 today), the green line quotes exclusions and testdata count | any finding outside an entry | coordinator, after W2 |
| 6.2 | Record: T0 (one file; ten packages) and T1 times, darwin and CI, warm named cache | not measured | W0 then lane A |
| 6.3 | `T-A12`: errcheck defaults vs written-out list on the real tree, both counts recorded, equal | a default the plant misses | QA (W0) |
| 7.1 | T4: conformance, determinism, replay, coverage, apidiff green | any behaviour change | coordinator |
| 7.2 | `tools/astequal/astequal_test.go` (runs at T2) :: `TestReflowEqual`, `TestMovedFunctionAcrossFilesEqual`, `TestCommentEditEqual`, `TestRenamedVariableUnequal`, `TestChangedLiteralUnequal`, **`TestOperatorChangedUnequal`** (`<`→`<=`, `+`→`-`), **`TestOperandsSwappedUnequal`**, **`TestDefineVsAssignUnequal`**, **`TestBreakVsContinueUnequal`**, **`TestIncVsDecUnequal`**, **`TestStatementOrderUnequal`**, **`TestSecondInitChangedUnequal`**, **`TestSameMethodNameOtherReceiverChangedUnequal`**, **`TestBlankVarChangedUnequal`**, **`TestGoDirectiveRemovedUnequal`** (`//go:noinline`, `//go:build`), `TestImportChangedUnequal`, `TestMovedToAnotherPackageUnequal` (DQ-S10) | stub | D |
| 7.3 | `tools/shape/shape_test.go` :: `TestShapeCorpusGolden`, `TestShapeEachConstructKind` (one row per case arm of `shape`'s and `shapes`' switches); completeness = 100 % statements of `shape` and `shapes` by `go tool cover -func`, quoted in the record, before the first refactor commit; one mutation per arm shown red in the record (DQ-S11) | green from birth — mutation proof | D |
| 7.4 | `BenchmarkTableCheckValue`, `BenchmarkRuleClockFireGroup` + `TestCheckValueAllocs`/`TestFireGroupAllocs` (`testing.AllocsPerRun` pinned at today's value) **if** W0 shows either on the per-event path (DQ-S12) | an extra allocation from the split | D |
| 7.5 | Per lane-D commit: `astequal <rev>` exit 0 for move/reflow commits, re-run by the coordinator at G5 with the exit codes quoted (DA-S9, ground rule 17) | a non-pure "pure" commit | D, coordinator |
| 7.6 | v2 (R-4) `internal/playback/recorder_test.go` :: `TestRecordsDoNotAlias`, red against a naive rewrite (shown in the record) before any appendAssign site changes | a rewrite that aliases records | C |

### `test_smells_planted.py` — the 30 cases

Each one planted world (its own fresh `GOLANGCI_LINT_CACHE`), one gate run; red names the past-bound file:line and does not name the at-bound one.

1 `test_function_60_lines_green_61_red_with_comments_counted` · 2 `test_cyclomatic_10_green_11_red` · 3 `test_file_500_green_501_red` · 4 `test_params_5_green_6_red` · 5 `test_line_120_green_121_red_a_tab_one_column` · 6 `test_a_test_file_is_exempt_from_size_and_lll_not_from_panic` (proves `run.tests = true`) · 7 `test_println_print_fmt_println_red_in_a_library` · 8 `test_fmtprint_green_in_a_main_tool_red_in_a_non_main_tool` · 9 `test_a_fourth_panic_beside_three_waived_is_red` · 10 `test_reflect_red_in_l3_green_in_l2_green_in_a_test` · 11 `test_getenv_lookupenv_environ_red_green_under_cmd_and_io` · 12 `test_blank_unsafe_with_linkname_is_red` · 13 `test_unsafe_in_an_xtest_is_red` · 14 `test_findings_in_an_example_function_are_red` · 15 `test_unchecked_close_red_written_out_exclusion_green` (`rt.Close()` red, `(*bytes.Buffer).WriteString` green, `fmt.Fprintln(os.Stdout, …)` red) · 16 `test_gocritic_append_assign_is_red` · 17 `test_a_waiver_on_the_wrong_line_is_red_and_the_finding_stays` · 18 `test_nolint_spellings_golangci_honours_are_red` · 19 `test_a_typecheck_error_in_an_excluded_package_is_red_beside_an_in_scope_finding` · 20 `test_a_generated_header_not_in_generated_json_is_linted` (+ listed one dropped) · 21 `test_two_findings_on_one_line_are_both_red` (uniq-by-line) · 22 `test_fifty_one_findings_of_one_linter_are_all_red` (max-issues-per-linter) · 23 `test_a_linux_only_and_a_darwin_only_finding_are_both_red` (v2, DA-S1: the world also holds `p_nocgo.go` with `//go:build !cgo` and a finding, red under both runs — golangci lints with `CGO_ENABLED=0` as `go list` claims) · 24 `test_unclaimed_files_are_red` (`_dir/`, `//go:build ignore`, `//go:build cgo` only, nested `go.mod`; and a file under `testdata/` counted as data, not red — R-1) · 25 `test_a_hiding_config_edit_is_refused_before_lint` (`issues.new-from-rev`) · 26 `test_a_float_argument_limit_is_red` (`arguments = [5.0]`, DQ-S1) · 27 `test_an_excluded_package_hides_only_its_rule` · 28 `test_an_exclusion_that_waives_nothing_is_red` · 29 `test_a_new_package_not_clean_is_red` (v2, DA-L5: the world also plants a not-clean package under each directory name W0 finds golangci skips by default, e.g. `examples/`; red, or W0 records that none is skipped and the case plants `examples/` anyway) · 30 `test_a_line_directive_cannot_move_a_finding_into_a_main_tool` (DQ-B2).

Cost (DQ-L5): 30 worlds × 2 GOOS lints. W0 measures the driver's time and whether `teeth` admits cases sharing one run; worlds are grouped only where each case still has its own verdict.

### Counts that move (exact)

- `proofs` row `min_wants` 38 → **39** (expected_scope `gate/proofs/drivers/test_*.py`: 38 files today + `test_smells_planted.py`).
- `gate/law.json` instruments: `smells` new, `proofs: [{"path": "gate/proofs/drivers/test_smells_planted.py", "cases": 30}]`, `min_wants` 1; `suppressions` `test_suppressions_planted.py` cases 9 → **13**; `vulns` `test_vulns_planted.py` cases 31 → **35**, row and instrument `min_wants` 6 → **7** (6 + golangci-lint). `aging`'s `test_aging_planted.py` stays 5 (decision 5 of 2026-11-10: a fixture never shrinks).
- `code` lists: each the import closure (§2.5 rule, test L.1).

## 9. Docs and ADR annotations (coordinator, at G5 with the record)

- **DEVELOPMENT.md:5** — replace "**Not built yet:** the suppressions checker, `todocheck` and golangci-lint's `smells` (S1.1c-2)." with: "**Built in S1.1c-2:** the suppressions checker. **Built in S1.1c-5:** golangci-lint's `smells` row (`.golangci.toml`, pinned whole and type-strict; linux and darwin, one build environment each). **Not built yet:** `todocheck`."
- **DEVELOPMENT.md:65** — "golangci-lint's linters and `nolintlint` run in the `smells` row from S1.1c-5, configured by `.golangci.toml` (the owner, 2026-11-14, S1.1c-5's design before its reviews, 1). A `//nolint` naming a linter `.golangci.toml` does not enable is red; every other waiver form a linter honours (`//revive:…`, `#nosec`, `//gocyclo:ignore`, `//nolint:all`, a non-canonical `//nolint` spelling, …) is red, and so is any `//line` directive (S1.1c-5's plan check, 7). A waiver is trailing on the line it waives; a `//nolint` alone on its line waives a whole declaration and is not written."
- **DEVELOPMENT.md:70** — the example becomes `//nolint:lll // PERMANENT reason="a URL that cannot wrap" ref=… sprint=S1.1c-5` (gosec is not enabled).
- **DEVELOPMENT.md**, a new line after :74 — "Tests are exempt from lll, the size family and the `reflect` pattern; `panic` and `os.Getenv` stay on tests, waived per line (S1.1c-5's plan check, 4). A size exclusion in `tools/allowlists/smells.json` names a package and a rule; the `aging` row reds it at T4 when the marker reaches its issue's part, and reds a forward moved later than at the merge base unless a decisions.md section names the issue (S1.1c-5's design after the reviews, 1)."
- **ADR-0015 §2 `smells` row**: "*(Annotated 2026-11-14, S1.1c-5 ([plan check](../../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check), decisions 1, 4, 7, 9, 10; [design](../../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-design-before-its-reviews), 1; [after the reviews](../../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-design-after-the-reviews), 1–3): built as the `smells` row, T0–T4, over `.golangci.toml` pinned whole, under linux and darwin; "prod scope only" covers line ≤ 120; `fmt.Print*` allowed in package-main tools; `unsafe` checked from `go list` imports; `os.LookupEnv` and `os.Environ` beside `os.Getenv`, `log.Print*` not in the set (backlog); every waiver per line; the size limits enter by package through TEMP exclusions forwarded to S1.1c-6…-8, their deadline and forward moves checked mechanically by the `aging` row against the merge base.)*"
- **ADR-0015 §2.1 test class**: "*(Annotated 2026-11-14 (S1.1c-5's plan check, 4): tests are also exempt from lll and from the `reflect` pattern; `panic` and `os.Getenv` stay, waived in code per line. A `.go` file under `testdata/` is data: the `smells` row counts it, never lints it.)*"
- **ADR-0015 §2.2**: "*(Annotated 2026-11-14 (S1.1c-5's plan check, 1; design after the reviews, 1): the size family's debt at the `smells` row's landing is held to dated parts — TEMP exclusions per package and rule, red at T4 when the marker reaches the part, a later forward red unless a decision names it (checked against the merge base, a missing base red at T4), never past S1.1c-8 — not recorded as maxima.)*"

## 10. Risks and open points (closed at v2)

- **Q-A — closed.** The config is `.golangci.toml` ([decisions.md](decisions.md#owner-decisions-2026-11-14--s11c-5s-design-before-its-reviews), 1).
- **R-1 — closed (coordinator): a row rule, not entries.** A tracked `.go` file with a `testdata` path segment counts as data, printed in the green line's count (§2.4). QA's PERMANENT `unlinted` entries rejected: a file under testdata cannot ship. The `unlinted` target form is removed.
- **R-2 — closed (owner, after the reviews, 2).** Printing is kept. All eight ids: `S3.3-2-1`, `S3.3-2-2`, `S3.3-2-3`, `S1.1c-3-4` turn red at S1.1c-7's close — the coordinator brings them to the owner before then; `S1.1c-5-1`…`S1.1c-5-4` would reach three closed rows at S1.1c-8's close and are named by the after-the-reviews section (their `decision` points at it). Test 3.11 pins the S1.1c-7 verdict.
- **R-3 — closed (owner, after the reviews, 3).** `os.Environ` kept; `log.Print*` not added (backlog).
- **R-4 — closed (coordinator).** The aliasing test first; rewrite where it and the playback tests stay green, else a per-line PERMANENT waiver citing the test (§7.1).
- **R-5 — closed.** Dissolved by the move of the part-level check to the `aging` row (§3.5), which counts its side inputs and gains `tools/allowlists/smells.json`.
- **R-6 — closed.** Measure first in W0; T0 gives the same verdict as T1 on the staged packages with liveness off (§2.7); tests 1.14.
- **R-7 — closed.** No row; `astequal`'s own tests run at T2; the coordinator re-runs it at G5 (§7.4).
- **R-8 — closed.** Measured: `Report.Warnings` = `[{"Tag":"runner/source_code","Text":…}]`; `typecheck` in `Report.Linters` with `Enabled: true` (M-8). The function-level `//nolint` question is moot (per line, DA-S8); `warn-unused` and the linux binary's build info are W0's.
- **R-9 — closed.** The merge-base read uses `gate/runner/git.py`'s `merge_base()` and runs in the `aging` row (§3.5); no base is red at T4/nightly.
- Remaining risk, named: Go code under `gate/proofs/**` is never linted (out of scope by design); a TEMP-excluded package can grow within its exclusion until its part (no ratchet, by decision 1) — the green line's per-(package, rule) counts and the record's diff across closes keep the growth loud.

## 11. Dispositions

Severity as the reviews gave it. "Fixed" = applied in v2 at the place named.

| Finding | Severity | Disposition | Where in v2 |
|---|---|---|---|
| DA-B1 T0 "waives nothing" red | blocker | Fixed: liveness judged on full-tree runs (T1+) only; T0 prints a line; T0 test | §2.3 step 10, §2.7(c), §8 3.5 |
| DA-S1 one build env | should-fix | Fixed: `gomods.ENV` + GOOS/GOARCH (+ cache) to both `go list` and golangci; test and planted case | §2.3 step 2, §8 1.11, case 23 |
| DA-S2 drop rule 5 | should-fix | Rejected by the owner (after the reviews, decision 1: mechanical check kept); its "if kept" fixes applied (`git.merge_base()`, missing base red at T4) | §3.5 |
| DA-S3 check in the aging row | should-fix | Fixed: `parts.forwards` run by `rows/aging.py` (lane B); aging's `expected_scope` and `code` grow | §1, §3.5, §8 3.10 |
| DA-S4 all eight aging ids | should-fix | Fixed: eight ids listed; S1.1c-5-1…4's `decision` the after-the-reviews section; the other four to the owner before S1.1c-7's close; fixture test | §3.3, §3.4, §10 R-2, §8 3.11 |
| DA-S5 `code` closures | should-fix | Fixed: closure rule + closure test | §2.5, §6, §3.5, §8 L.1 |
| DA-S6 drop `package_clause` | should-fix | Fixed: `GoFile(import_path, package)` from `go list`; golangci.py no longer imports suppressions.py | §2.2, §2.3 step 8b, §2.4, §8 1.10 |
| DA-S7 empty main-module sum | should-fix | Fixed: optional sum; recorded file as test data; `pseudo` narrows the judgement only | §0 M-13, §6, §8 5.3, 5.4 |
| DA-S8 function-level nolint | should-fix | Fixed: `canonical.go` with ~33 per-line waivers; per line for every waiver | §4.2, §7.1, §9 |
| DA-S9 astequal proof | should-fix | Fixed: own tests at T2; G5 re-run by the coordinator; scope of the proof stated | §7.4, §8 7.2, 7.5 |
| DA-L1 cache unnamed | later | Fixed: row sets `GOLANGCI_LINT_CACHE` under the tool cache root; planted worlds fresh | §2.3 step 2, §8 1.11, 1.13 |
| DA-L2 `test_sprintid_parts.py` exists | later | Fixed: named as an edited file | §1, §3.2, §8 3.1 |
| DA-L3 no nightly tier for smells | later | Fixed: reworded | §2.3 step 8c |
| DA-L4 other platform's binary | later | Fixed: named residue | §6 |
| DA-L5 default-skipped dirs | later | Fixed: W0 measures; one planted case (folded into case 29, so the count stays 30) | §0, §8 case 29 |
| DA-L6 Q-A answered | later | Fixed: closed | §10 |
| DQ-B1 forward moved by re-pointing/adding | blocker | Fixed (owner decision 1): per-target effective forward vs merge base; new target red after S1.1c-5 | §3.5 rule 5, §8 3.3 |
| DQ-B2 `//line` relocation | blocker | Fixed: suppressions refuses `//line` and `/*line`; row reds an issue at an unclaimed file | §4.2, §2.3 step 7, §8 1.10, 2.7, 2.10, case 30 |
| DQ-B3 no merge base at T4 | blocker | Fixed: red at `RED_EXPIRED` tiers; line at T3 | §3.5 rule 6, §8 3.4 |
| DQ-S1 type-blind pin | should-fix | Fixed: `pin.differences` type-strict, recursive, list order counts | §2.2, §5, §8 1.3, 4.4, case 26 |
| DQ-S2 stale report | should-fix | Fixed: both files unlinked before each run; decode error a named fault | §2.3 steps 4–5, §8 1.7 |
| DQ-S3 root package `""` | should-fix | Fixed: `""` → `.` | §2.3 step 8c, §8 3.5 |
| DQ-S4 cache served stale issues | should-fix | Fixed: as DA-L1 | §2.3 step 2, §8 1.11 |
| DQ-S5 liveness at T0 | should-fix | Fixed: as DA-B1 | §2.3 step 10, §8 3.5 |
| DQ-S6 `//nolint` spellings, census completeness | should-fix | Fixed: procedure as stated; measured spellings recorded; non-canonical spellings refused | §0 M-9, §4.1, §4.2, §8 2.4, case 18 |
| DQ-S7 restore revision | should-fix | Fixed: `git show 96b76f4:` (coordinator-verified; `fc944ef` pre-rebase); docstring citation corrected by lane B | §1, §4.2, §8 2.8 |
| DQ-S8 `go_built` dropped silently | should-fix | Fixed: pin test and `vulns` `min_wants` 6 → 7 | §6, §8 5.1, counts |
| DQ-S9 exemption matching nothing | should-fix | Fixed: W0 checks; lane C adds the rule if absent | §6, §8 5.5 |
| DQ-S10 AST canonical form | should-fix | Fixed: every non-position field, receiver keys, numbered `init`/`_`, `//go:` kept | §7.4, §8 7.2 |
| DQ-S11 shape characterisation | should-fix | Fixed: 100 % statements + one mutation per arm, before the first refactor commit | §7.5, §8 7.3 |
| DQ-S12 hot path unmeasured | should-fix | Fixed: W0 checks the per-event flow; benchmarks + `AllocsPerRun` first if reached | §0, §7.5, §8 7.4 |
| DQ-S13 golangci linted both GOOS | should-fix | Fixed: as DA-S1; planted case 23 | §2.3 step 2, §8 1.11, case 23 |
| DQ-L1 indexed diff paths | later | Fixed | §2.2, §8 4.4 |
| DQ-L2 realpath root | later | Fixed | §2.3 step 3, §2.4, §8 1.11 |
| DQ-L3 `code` lists | later | Fixed: as DA-S5 | §2.5, §8 L.1 |
| DQ-L4 green line format | later | Fixed: pinned by a test | §2.3, §8 1.15 |
| DQ-L5 planted cost | later | Fixed: W0 measures cost and run-sharing | §0, §8 |
| R-1 testdata | question | Closed (coordinator): row rule, not entries; QA's `unlinted` entries rejected (a testdata file cannot ship); form removed | §2.4, §3.1, §8 3.6, case 24 |
| R-2 aging of printed parts | question | Closed (owner, after the reviews, 2) | §3.3, §3.4, §8 3.11 |
| R-3 `log.Print*`, `os.Environ` | question | Closed (owner, after the reviews, 3): `os.Environ` kept, `log.Print*` to the backlog | §2.1 |
| R-4 appendAssign | question | Closed (coordinator): aliasing test first, rewrite where green, else per-line PERMANENT citing the test | §7.1, §8 7.6 |
| R-5 suppressions' side inputs | question | Closed: dissolved by the aging-row placement | §3.5 |
| R-6 T0 | question | Closed: measure in W0; T0 = T1's verdict on staged packages, liveness off; two tests | §2.7, §8 1.14 |
| R-7 astequal not a row | question | Closed: no row; T2 tests; G5 re-run | §7.4 |
| R-8 unmeasured shapes | question | Closed: two facts recorded (M-8); rest W0 | §0, §10 |
| R-9 git in suppressions | question | Closed: `git.merge_base()` in the aging row | §3.5 |
