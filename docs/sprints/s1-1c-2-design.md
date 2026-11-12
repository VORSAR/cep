# S1.1c-2 — design note: the exemptions and the teeth (v1, 2026-11-12)

*Step 1 of the full loop for S1.1c-2 ([sprint model §6](00-sprint-model.md#6-the-full-loop)), written after G0 (`s1-1c-2/integration` @ 3bf601f) from the answered [plan check](s1-1c-2-plan-check.md) ([decisions, 2026-11-12](decisions.md#owner-decisions-2026-11-12--s11c-2s-plan-check)). A build-ready spec: every path, signature, failure text, test and step a lane needs. Reviewed by the architect with the-fool and by QA for testability before G1; no semantics panel (decision 7 of 2026-11-10). Card: [sprint plan — S1.1c-2](sprint-plan.md#s11c-2--the-exemptions-and-the-teeth). Code claims cite `file:line` on 3bf601f.*

## 0. Proven before review (sprint model §10: a library claim is proven by a scratch run)

1. **The runner gives every row its tier.** `gate/runner/env.py:29-30` `row_env` sets `GATE_TIER`, `GATE_ROW`, `GATE_RUN_DIR`, `GATE_ROW_DIR`. A row can therefore be red at T4 and nightly only (decision 8).
2. **ruff honours `# fmt: skip` followed by a second comment, and silently drops it when text follows on the same comment.** Scratch run, pinned ruff 0.17.3, `ruff format --isolated`: `x = [\n 1,2,  3]  # fmt: skip  # the matrix reads as a grid` kept its layout; `... # fmt: skip -- the matrix reads as a grid` was reformatted to `y = [1, 2, 3]`. → the reason form is `# fmt: skip  # <reason>` (§2.4), and a `# fmt: skip<anything>` comment is red.
3. **`go list -m all` selects one version for every module `go mod graph` names** but the `go`/`toolchain` pseudo-nodes ([R-V G7](../reviews/s1-1c-2-research-vuln-canary.md)), and `gate/runner/rows/deps.py:52-92` builds `modules` from `go list -m -json all` of both modules. `deps.compare` already checks the allowlist's version against the selected one for every such module (`gate/runner/deps.py:201-206`). The forward "graph-only modules get no version check" (S1.1c-1, A-L3) therefore reduces to one guard: a graph node that neither module's build list selects is red (§4.6).
4. **The T2 baseline at G0** (record): every row PASS but the three stubs, each red by name.

## 1. The law model (A holistic 3: one table, so the model is reviewable)

| Law file | Kind (LAW Δ one-liner) | Owner | Checker | Pin | Clock |
|---|---|---|---|---|---|
| `gate/registry.json` | `registry` (±rows, changed fields) | coordinator | `registry.py` on every run; `ci-fresh` | `instruments` (§4.2) | — |
| `gate/law.json` | `law` (±paths, kinds; ±instruments, changed fields) | coordinator | `lawdelta` (liveness) | `test_law_pin.py` (ADR-0015 §1.3) | — |
| `gate/current-sprint`, `gate/sprints.json` | `marker`, `sprints` | coordinator | `notbuilt` | `test_sprints_pin.py` (the plan table) | the plan's ✓ rows |
| `tools/allowlists/*.json` | **`allowlist`** (±entries by target) | coordinator | **`suppressions`** (grammar); archcheck (purity, clock, testhygiene targets); `teeth.py` (teeth); ruff pin (ruff) | — | TEMP `expiry` (calendar, red at T4 + nightly) |
| `gate/known-issues.json` | **`known-issues`** (±ids, forward, decision) | coordinator | `test_known_issues_pin.py` (T2); S1.1c-3's `aging` | the Markdown table | rows after `first_seen`; the `forward` row (S1.1c-3) |
| `tools/deps-allowlist.json` | `deps` | coordinator | `deps` | — | — |
| `ruff.toml` | `code` | coordinator | `ruff` | `ruff.json` both ways (§2.4) | — |
| `tools/importcheck/rules.json` | `edges` | coordinator | archcheck | ARCHITECTURE §3 (review) | — |
| `conformance/notbuilt.json` | `notbuilt` | coordinator | `notbuilt` (T4), `TestCorpus` | — | the marker |
| `gate/tripwire.json`, `tools.json`, goldens, baselines | as built | coordinator | their rows | — | — |
| `go.mod`, `gate/proofs/go.mod` | **`gomod`** (§4.6) | coordinator | `deps` | — | — |
| `gate/`, `tools/`, `.githooks/`, `Makefile`, corpus | `code` (grouped last) | lanes write code; coordinator data | the rows | — | — |

## 2. Lane A — the grammar, the anchors, the suppressions row, two LAW Δ kinds, the exemption line (python-engineer, W1)

### 2.1 `gate/runner/anchors.py` (new, pure; moved out of `rows/doclinks.py:25-77`)

```python
FENCE, HEADING, HTML_ANCHOR  # the three regexes, moved verbatim
def slug(heading: str) -> str                 # verbatim from doclinks.py:35-47
def prose(lines: list[str]) -> Iterator[tuple[int, str]]   # verbatim, :50-59
def anchors(text: str) -> set[str]             # verbatim, :62-74
def split_ref(ref: str) -> tuple[str, str]     # "a/b.md#x" -> ("a/b.md", "x"); "" anchor when no '#'; ValueError on "", a leading "/", "..", or more than one '#'
def resolves(ref: str, read: Callable[[str], str | None]) -> str  # "" when it resolves, else why
```
`resolves` rules, each why exact: `ref has no anchor: write <path>#<heading-anchor>`; `<path> is not a tracked Markdown file` (read returns None, or not `.md`); `<path> has no heading #<anchor>`. `doclinks.py` imports `slug`, `prose`, `anchors` from it (its behaviour unchanged; its tests unchanged and green). The short form `ADR-0015#2` fails `split_ref`'s path rule (`ADR-0015 is not a tracked Markdown file`).

### 2.2 `gate/runner/allowlists.py` (new, pure)

```python
RULES = frozenset({"purity", "clock", "teeth", "ruff", "testhygiene"})   # S1.1c-3 adds "govulncheck"
FILE_KEYS = frozenset({"schema", "note", "rule", "entries"})
@dataclass(frozen=True)
class Entry:   target: str; kind: str; reason: str; sprint: str; ref: str = ""; expiry: str = ""; issue: str = ""
@dataclass(frozen=True)
class AllowList:  name: str; rule: str; entries: tuple[Entry, ...]
def load(text: str, name: str) -> AllowList          # ValueError naming name, entry index and key
def check(lst: AllowList, *, today: date, tier: str, resolve: Callable[[str], str], order: sprintid.Order,
          current: sprintid.ID, issues: frozenset[str]) -> list[str]      # one failure line each, sorted by entry
def expiring(lst: AllowList, *, today: date, within_days: int = 14) -> list[tuple[Entry, int]]   # for the countdown
```
`load` faults (ValueError; the row prints `suppressions: tools/allowlists/<name>: <fault>`): not JSON or a duplicate key (`loads` from `law.py`); keys ≠ `FILE_KEYS`; `schema` ≠ 1; `note` not a str; `rule` not in `RULES`; the file stem ≠ `rule` (`purity.json` holds rule `purity`); `entries` not a list; an entry's keys outside `{target, kind, reason, sprint, ref, expiry, issue}` or missing one of `{target, kind, reason, sprint}`; a non-str value; two entries with one target.

`check` failures, per entry `entries[i] (<target>)`:
- `kind` not `PERMANENT`/`TEMP` → `kind must be PERMANENT or TEMP`.
- `len(reason.strip()) < 12` → `reason must be at least 12 characters`.
- PERMANENT: `ref` missing → `PERMANENT needs ref=<path>#<anchor>`; `resolve(ref)` non-empty → that why; `expiry` or `issue` present → `PERMANENT takes no expiry or issue`.
- TEMP: `expiry` missing or `issue` missing → `TEMP needs expiry=YYYY-MM-DD and issue=<known issue id>` (decision 8: both); `ref` present → `TEMP takes no ref`; `expiry` not `date.fromisoformat`-able → `expiry must be YYYY-MM-DD`; `expiry − today > 90 days` → `expiry <d> is more than 90 days ahead`; `expiry < today` and `tier in {"T4", "nightly"}` → `expired <d>` (at T0–T3 an expired entry is not red; §2.6's countdown names it); `issue not in issues` → `issue <id> is not in gate/known-issues.json`.
- `sprint`: `sprintid.parse` fails → its message; `order.index(sprint)` raises → `sprint <s> is not a row of gate/sprints.json`; `order.index(sprint) > order.index(canonical(current))` → `sprint <s> is after the current sprint <c>`; same row and `parse(sprint).n > current.n` → the same text (QA-L1: `S1.1c-9` while the marker is `S1.1c-2`).
`today` is `datetime.now(timezone.utc).date()` in the row, injected in tests; the run's tier is `os.environ["GATE_TIER"]` in the row, injected in tests.

### 2.3 `gate/runner/suppressions.py` (new, pure)

```python
ENABLED: frozenset[str] = frozenset()   # golangci linters enabled; empty until S1.1c-4 reads its .golangci.yml here
@dataclass(frozen=True)
class Directive:  line: int; linters: tuple[str, ...]; kind: str; meta: dict[str, str]; raw: str
def go_comments(source: str) -> Iterator[tuple[int, str]]     # (line, text after "//") for each line comment
def directives(source: str) -> list[Directive | tuple[int, str]]   # a parse fault is (line, why)
def check_go(path: str, source: str, *, enabled, today, tier, resolve, order, current, issues) -> list[str]
def py_markers(source: str) -> list[tuple[int, str, str]]     # (line, "noqa" | "fmt-skip" | "fmt-skip-bad" | "fmt-off", reason)
def check_py(path: str, source: str) -> list[str]
def count(go_sources: Iterable[str], py_sources: Iterable[str]) -> int   # //nolint directives + `# fmt: skip` markers
```
**`go_comments`** is a state machine over the source's characters, states `CODE`, `LINE` (after `//` to end of line), `BLOCK` (`/*` … `*/`), `STRING` (`"` with `\` escapes, ends at `"` or newline), `RAW` (`` ` `` … `` ` ``, spans lines), `RUNE` (`'` with `\` escapes). Only `LINE` comments are yielded (golangci honours `//nolint` only as a line comment). `x := "//nolint:errcheck"` yields nothing (QA-S10).
**A directive** is a comment whose text starts `nolint` (case-sensitive, no space: `//nolint`). Grammar: `nolint:<linter>[,<linter>]... // (PERMANENT|TEMP) <key>=<value>...`, values bare or double-quoted (`reason="…"` must be quoted). Faults, each `suppressions: <path>:<line>: <why>`: bare `//nolint` or `//nolint ` → `a suppression names its linter: //nolint:<linter>`; no `// PERMANENT|TEMP` part → `a suppression carries its metadata: // PERMANENT reason="…" ref=… sprint=… (DEVELOPMENT.md)`; an unknown key → `unknown key <k>`; then the same field rules as §2.2 over `reason`, `ref`, `expiry`, `issue`, `sprint`; and for each linter not in `enabled` → `//nolint:<l>: <l> is not enabled (none is until S1.1c-4)`.
**`py_markers`** reads `tokenize` COMMENT tokens (never strings): a comment `# noqa…` (case-insensitive) → `noqa`; exactly `# fmt: skip` → `fmt-skip`, its reason the next COMMENT on the same line with its `# ` stripped (§0.2); `# fmt: skip` followed by other text in the same comment → `fmt-skip-bad`; `# fmt: off`/`# fmt: on` → `fmt-off`. `check_py` failures: `noqa` → `suppressions: <path>:<line>: ruff runs --ignore-noqa: this comment waives nothing; remove it`; `fmt-skip` with a reason under 12 characters or none → `…: write its reason as a second comment: # fmt: skip  # <reason, 12+ characters>`; `fmt-skip-bad` → `…: ruff ignores a skip with text after it; write # fmt: skip  # <reason>`; `fmt-off` → `…: use # fmt: skip  # <reason> on the statement`.

### 2.4 The row `gate/runner/rows/suppressions.py` (replaces the G0 stub)

`main(argv)`: `rows.start()`; the files list `argv[1]` (the registry's `{files}`: every tracked `.go`, `tools/allowlists/*.json`, `ruff.toml`, `gate/**/*.py` — §G0's `expected_scope`); load `gate/sprints.json` (`sprintid.load_order`), `gate/current-sprint` (`read_marker`), `gate/known-issues.json` ids (a small `json` read here; the full pin is lane D's), every `tools/allowlists/*.json` (`allowlists.load`, faults red); then per file: `.go` → `check_go`, `gate/**/*.py` → `check_py`, each list → `allowlists.check`; **the ruff pin**: `tomllib` reads `ruff.toml`'s `[lint.per-file-ignores]` as `{file: [code, …]}`, `ruff.json`'s targets split `"<file>: <code>"`; each `(file, code)` in one and not the other → `suppressions: ruff.toml waives <code> for <file> without an entry in tools/allowlists/ruff.json` / `… ruff.json lists <file>: <code>, which ruff.toml does not waive`. `rows.covered(<files read>)`; `rows.finish(failures)`. No network, no git.

### 2.5 Two LAW Δ kinds (`gate/runner/law.py`; registered in `KINDS` and `PARSERS`)

- **`allowlist`** — `(loads, _delta_allowlist)`: entries keyed by `target`; per target, in new-file order then removed: added `+<target> <KIND> <sprint>`, removed `−<target>`, changed `<target>: <field> <old> → <new>` (`reason changed` for a reason); `; `-joined; suffix ` (<n> entries)`. `file added: <n> entries` for a new list. Example: `tools/allowlists/purity.json +6/−0 — +internal/x: sync.Once PERMANENT S4.1 (4 entries)`.
- **`known-issues`** — `(loads, _delta_known_issues)`: by `id`: `+<id> first seen <s>`, `−<id>`, `<id>: forward <a> → <b>`, `<id>: decision set|changed|cleared`.
After lane A lands, the coordinator re-kinds `tools/allowlists/` → `allowlist` and `gate/known-issues.json` → `known-issues` in `gate/law.json` (one commit; the `law` kind's one-liner names both).

### 2.6 The exemption line and the countdown (`gate/runner/rows/lawdelta.py`)

After the block (or `HEAD_IS_MAIN`, or `NONE`), and before the uncommitted-edits line, `lawdelta` appends:
- `exemptions: <n> in code, <m> listed (<±d> this diff)` — `n` = `suppressions.count` over the tracked `.go` and `gate/**/*.py` files at the revision; `m` = the entries in `tools/allowlists/*.json` at the revision; `d` = (n + m at HEAD) − (n + m at the merge base), printed `+0`, `+2`, `−1`. Files are read with `git.show`, found with a new `git.grep_files(rev, patterns, pathspecs) -> list[str]` (`git grep -l -e nolint -e 'fmt: skip' <rev> -- '*.go' 'gate/*.py'`), so only files holding a marker are shown; a list is counted from every `tools/allowlists/*.json` at the revision (`git.ls_tree(rev, "tools/allowlists/")`, new, both in `gate/runner/git.py`). On `HEAD_IS_MAIN`, `d` is `+0`.
- one line per entry from `allowlists.expiring(...)` at HEAD: `exemption <name>: <target> expires <date> (in <k> days)`, and for a past date `exemption <name>: <target> expired <date> (red at T4 and nightly)`.
A list that did not change prints nothing else (decision 2: quiet when unchanged). No record field, no schema move.

### 2.7 DEVELOPMENT.md (the coordinator, with lane A's landing)

The three examples become: `//nolint:gosec // PERMANENT reason="fixture generator; output is never security-relevant" ref=docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md#2-architecture-constraints-as-analyzers-with-teeth-reference-a-in-go sprint=S1.1c-4`, `//nolint:errcheck // TEMP reason="close error surfaced by the sink rework" expiry=2027-01-31 issue=S3.3-2-1 sprint=S1.1c-4`, and the `revive` line dropped (one TEMP example is enough); the rules paragraph names `ref`'s form, TEMP's `expiry` + `issue`, the 90-day bound, T4/nightly, and `# fmt: skip  # <reason>`. `test_suppressions.py` reads the policy's fenced examples and checks each parses as a valid directive with the linter injected as enabled (QA-S11).

## 3. Lane C — archcheck reads its lists; the `-tests` mode; the two waits (go-engineer, W1)

1. **`LoadAllowList(path, rule string) ([]string, error)`** in `tools/archcheck/main.go`: `encoding/json` into `struct{ Rule string; Entries []struct{ Target string } }` (unknown fields ignored: the Python row is the grammar's one validator — A-L2); `Rule != rule` → error `<path>: rule <got>, want <rule>`; returns the targets. A read or decode error is an internal error (exit 2, `internal error: <path>: …`).
2. **purity**: `syncNamed` (`main.go:79-82`) is deleted; at start-up `tools/allowlists/purity.json`'s targets `"<pkg>: <import>.<Ident>"` build the same `map[string]map[string][]string` (split at `": "`, then at the last `.`). A target that does not split → internal error naming it.
3. **clock**: the hard-coded `c.path != "internal/window/cron.go"` (`main.go:257`) becomes membership in `tools/allowlists/clock.json`'s targets `"<file>: time"` (empty now: `internal/window/cron.go` may not import time until S9.1 adds its entry — carried, A-L13).
4. **Stale targets**: after the files are read, every purity target's package directory and every clock/testhygiene target file must exist on disk → else one finding `failed: tools/allowlists/<name>.json: <target>: does not exist (a stale target)`, not counted as a file.
5. **`-tests` mode**: `go run ./tools/archcheck -tests <files.nul>` — reads the same list; for each `_test.go` file (any other file → internal error `-tests reads test files only`) it runs one check, the sleep fence: a selector on the file's local name(s) for `time` (`localNames("time")`, `main.go:236`) naming `Sleep`, `After`, `AfterFunc`, `Tick`, `NewTimer` or `NewTicker` → `failed: <file>:<line>: testhygiene: time.<Name> in a test: wait for a state, cap it with a context, yield on every pass of a poll (testing.md rule 1)`; a dot-import of `time` → `…: testhygiene: a dot-import of time hides the calls the fence reads`. Files in `tools/allowlists/testhygiene.json` (`"<file>"` targets; empty) are skipped. `GATE_COVERED <files read>`. The normal mode is unchanged.
6. **The two waits** (`io/inmemory/breakit_s3_3_1_test.go:47,54`), red-first: run `-tests` on the real tree and quote its two findings; then each `select { … case <-time.After(d): t.Fatal(…) }` becomes `ctx, cancel := context.WithTimeout(context.Background(), d); defer cancel()` and `case <-ctx.Done(): t.Fatal(…)` with the same message; the test still passes alone and under `-race -count=20`.
7. **The registry switch** (coordinator, after lane C lands): `testhygiene`'s command → `["go", "run", "./tools/archcheck", "-tests", "{files}"]`; `gate/runner/rows/testhygiene.py` deleted.

## 4. Lane D — the teeth, `docsync` (a), the forwards, BA13, the floor (python-engineer, W2)

### 4.1 `docsync` (a): `gate/runner/docsync.py` (pure) and `rows/docsync.py`

```python
def tree(architecture_md: str) -> list[str]   # the first fenced block after the "## 3." heading; ValueError when absent
def listed(tree_lines: list[str]) -> set[str] # first column (text before 2+ spaces), split on ", "; "x/" -> "x"; an item with a file extension contributes its directory ("cep.go" -> "."); items naming a path with a placeholder ("io/*") are ignored
def missing(packages: Iterable[str], listed_dirs: set[str]) -> list[str]   # sorted, "." for the root
```
The row runs `go list -e -f '{{.Dir}}' ./...` in the repository root (`rows.run_tool`, output to the row dir; `GOFLAGS=-mod=readonly`, `GOPROXY=off`), makes each directory repository-relative, and fails `docsync: <dir> is a Go package and ARCHITECTURE §3 does not list it` per missing one; `GATE_COVERED <packages>` (the registry's `expected_scope` `["go", "./..."]` counts the same packages). `go list` skips `testdata/` and the nested `gate/proofs` module. **The coordinator adds `tools/archcheck/`, `tools/apipartition/`, `tools/shape/` to §3's tree at W2's start** (Q15) and annotates `tools/analyzers/` (not built; the analyzer is `tools/archcheck/`).

### 4.2 The instrument pin: `gate/runner/teeth.py` (pure) and `gate/runner/tests/test_instruments_pin.py`

`gate/law.json` gains `"instruments"`: `law.TOP_KEYS` += `instruments`; `law.load_manifest` returns it as `Manifest.instruments: tuple[Instrument, ...]` (validation faults are `ManifestError`s naming the entry):
```python
@dataclass(frozen=True)
class Proof:       path: str; cases: int
@dataclass(frozen=True)
class Instrument:  id: str; row: str; lands_in: str; code: tuple[str, ...]; proofs: tuple[Proof, ...]
                   tiers: tuple[str, ...] = (); min_wants: int = -1; expected_scope: tuple[str, ...] = ()
                   go_test: str = ""; exempt: str = ""
```
Shapes: a **row instrument** has `tiers`, `min_wants`, `expected_scope` and no `go_test`; a **Go-test instrument** has `row: "unit"`, `go_test: "<package dir>.<TestName>"` and none of the three. Exactly one of `proofs` (non-empty) or `exempt`. `_delta_law` gains `instruments` one-liners: `+instrument <id>`, `−instrument <id>`, `instrument <id>: <field> <old> → <new>` (`cases` per proof path).

`teeth.py`:
```python
VERDICT = frozenset({"assertRed", "assertRow", "assertGateFailed", "assertParityStops", "assertParityPassed"})
RED = frozenset({"assertRed", "assertGateFailed", "assertParityStops"})   # and assertRow(..., False, ...)
def python_cases(path: str) -> tuple[int, list[str]]   # (cases unittest's loader finds, faults)
def go_cases(path: str) -> tuple[int, list[str]]       # via `go run ./tools/archcheck -cases <path>` (§4.3)
```
`python_cases` imports the driver (`importlib`, repository root on `sys.path`), counts `unittest.TestLoader().loadTestsFromModule(module).countTestCases()`, and parses the source with `ast`: each `test_*` method must call a `VERDICT` helper on `self`, directly or through one helper method of its own class or module that does (one level) → else fault `teeth: <path>::<method> asserts no gate verdict`; the module must hold at least one case reaching a `RED` helper (or `assertRow` whose fourth positional argument is the literal `False`) → else `teeth: <path> plants nothing red`. **Zero cases is a fault** (`teeth: <path> counts 0 cases`).

The pin's six tests (each with fixture manifests and registries under `gate/runner/testdata/teeth/` for its red case, then one test over the real files):
1. `test_every_row_has_one_row_instrument` — the registry's ids = the row instruments' `row`s, both ways; a duplicate → red.
2. `test_each_row_is_pinned` — `blocking`, `tiers`, `lands_in`, `min_wants`, `expected_scope` equal the instrument's (`proofs` 24 → 23 red; a scope narrowed red).
3. `test_each_go_test_exists` — `go_cases` of the package's test files lists the named test.
4. `test_code_and_proofs_are_law` — every `code`/`proofs` path tracked and covered by `law.entry_for`.
5. `test_proofs_have_their_cases` — each `Proof.cases` equals the counted cases; faults from §4.2 are red.
6. `test_exempt_names_a_teeth_entry` — an `exempt` is a target of `tools/allowlists/teeth.json` (empty today: no instrument is exempt — decision 10).

**Reach (decision 9):** a `proofs` path is a driver under `gate/proofs/drivers/`, a gate test file, or a dedicated Go test file — never a general engine test file. The instrument list as built (P5): one row instrument per registry row (32 with G0's three) with its drivers (the eight from W0 for the rows that had none) and the Go-test instruments `docsync-b` (`internal/validate.TestEveryStaticCodeIsAccountedFor`), `docsync-d` (`internal/parser.TestErrorCountsMatchTheCatalogue`), `nosleep` (`internal/runtime.TestNoSleepOrPollingInRuntime`). Lane D measures every count on the integration branch and hands the coordinator the JSON; the coordinator writes `instruments`.

### 4.3 archcheck's `-cases` mode (lane D owns `tools/archcheck/` in W2)

`go run ./tools/archcheck -cases <file_test.go>` prints, sorted: `case <TestName>` per top-level `func Test<X>(t *testing.T)`; `case <TestName>/<literal>` per `t.Run("<literal>", …)` call whose first argument is a string literal; and for a `for … range <ident>` loop whose body calls `t.Run` with a non-literal name, `case <TestName>/<ident>[<i>]` per element of `<ident>`'s composite literal when `<ident>` is declared in the same function by `:=` or `var` with a slice, array or map composite literal (QA-S3: a loop cut short changes the count); a loop over anything else → `fault <TestName>: t.Run over a table this mode cannot count`. Exit 0; `teeth.go_cases` counts the `case` lines and reports `fault` lines as faults.

### 4.4 BA13: the shim's report through a file (`gate/runner/pyshim.py`, `gate/runner/rows/__init__.py`)

`pyshim.main(argv)` takes a third argument, the report path: `argv = [prog, root, name, report]`. Before importing the module it opens `report` for writing and keeps the handle; at the end it writes the same one line it prints today (`pyshim: <name>: passed <n> tests` or `pyshim: <name>: <problems>`) to the handle and closes it, and still prints it for humans. `rows._run_module` passes `os.path.join(out_dir, f"{name}.report")` (removed first if present) and reads **only** that file: pass iff exit 0 and the file's one line is `pyshim: <name>: passed …`; no file → `exit <code>, and the shim made no report`. A module that prints the pass line and calls `os._exit(0)` leaves no report and is red (known issue BA13's own forward). The shim stays Python 3.12 (`pyfloor` runs it under 3.12.16). Both the `pytests` and `proofs` rows use `run_modules`, so both are covered. At close-out BA13 leaves known-issues.md and `gate/known-issues.json` together (the T2 pin keeps them equal).

### 4.5 The coverage floor (`gate/coverage/main.py`)

`DEFAULT_FLOOR` (`:34`) is deleted and `--floor` is required (`argparse` `required=True`; a missing one is the module's existing usage error, exit 2). The registry passes `--floor 90` since G0, so a change to the floor is a `registry` one-liner in LAW Δ. A test in `gate/coverage/tests/` pins the usage error.

### 4.6 The three forwards

- **`gomod`** (`law.py`, `(_gomod, _delta_gomod)`): `_gomod(text)` reads `module`, `go`, `toolchain`, `require` (single and block, `// indirect` kept as a flag), `replace`, `tool` lines into a dict; one-liners: `+require <m> <v>`, `−require <m>`, `<m> <v1> → <v2>`, `+replace <m> => <t>`, `+tool <p>`, `toolchain <a> → <b>`, `go <a> → <b>`; the coordinator re-kinds `go.mod` and `gate/proofs/go.mod` to `gomod`.
- **Graph-only versions** (`deps.compare`): a path in `graph` that is in no build list (`graph − set(by_path)`, today empty — §0.3) is red `deps: <path> is in the module graph and selected by neither module's build list`, beside its allowlist check; a unit case plants one.
- **Up to three headings** (`law.doc_line`): the headings of the first three hunks (deduplicated, each `§<n>` or text), then `(and <k> more)` when more hunks fall under other headings; **marker-only hunks** — a hunk whose changed lines differ only by `<!--cep…-->`/`<!--text-->` comments — are not named and are counted `+<n> markers` (A-S11). The S1.1c-1 form `(first of <k> hunks)` is replaced.

### 4.6a The dependency register gains `sprint` and `ref` (decision 17)

`tools/deps-allowlist.json`'s entries gain two required keys: `sprint` (the sprint whose approval added the module) and `ref` (the deciding section, `ref`'s form, §2.1). `deps.ENTRY_KEYS` (`gate/runner/deps.py:24`) gains both; `deps.compare` checks `sprint` with `sprintid` (at or before the marker) and `ref` with `anchors.resolves`, each fault `deps: <module>: <why>`. The coordinator writes the 25 entries' values at W2's start (each module's approval: decisions.md's dated sections and ADR-0009); the `deps` kind's one-liner names a changed `ref` or `sprint`.

### 4.7 `test_known_issues_pin.py` (T2)

The ids of `gate/known-issues.json` equal the Markdown table's, both ways; each table row's Found cell begins with its `first_seen` followed by a non-identifier character (`S1.1c-1's` counts; `S1.1c-10` does not match `S1.1c-1`); `first_seen` and a non-empty `forward` are rows or parts of `gate/sprints.json`'s order; a non-empty `decision` resolves (`anchors.resolves`) and its section — the heading's text up to the next heading of the same level — names the id as a whole word. **The `decision` form is `ref`'s form**: the G0 data's `decisions.md#…` becomes `docs/sprints/decisions.md#…` in the design commit.

## 5. QA's wave 0 for the lanes (after this note; red for the stated reason)

- **On lane A's starting branch:** `gate/runner/tests/test_anchors.py` (the moved functions, `split_ref`, `resolves` — every why of §2.1), `test_allowlists.py` (plan check §6 [2] grammar (1)–(12) with §2.2's texts), `test_suppressions.py` (`go_comments` states, the directive faults, `py_markers`, the policy's examples), `test_law.py` additions (the two kinds), `test_lawdelta.py` additions (the exemption line `+0`, `+1`, `−1`; the countdown lines; no line for an unchanged list).
- **On lane C's starting branch:** `archcheck_test.go` cases (plan check §6 [2] archcheck (1)–(8)).
- **On lane D's starting branch:** `test_teeth.py`, `test_instruments_pin.py`'s fixture cases, `test_docsync.py`, `test_known_issues_pin.py`, `gate/coverage/tests` (the required floor), `test_deps.py` (graph-only; `sprint` and `ref` on every entry), `test_law.py` (gomod, three headings, markers).
- **On integration** (T4-only, red on the stubs): `gate/proofs/drivers/test_suppressions_planted.py`, `test_testhygiene_planted.py`, `test_docsync_planted.py`, `test_shim_forged_pass.py` (BA13: a planted module prints the pass line and `os._exit(0)` → the `pytests` row red `… the shim made no report`).

## 6. Order and commits

G1 (this note reviewed; v2) → QA's W0 for the lanes → W1: lane A (`anchors.py` → `allowlists.py` → `suppressions.py` → the row → the kinds → the exemption line → its drivers green; the 40 `# fmt: skip` reasons and the inert `# noqa` in `gate/differential/shapes.py:34` are lane A's comment-only edits) ‖ lane C (the waits red-first → the lists → `-tests` → stale targets) → the coordinator: re-kinds, the `testhygiene` switch, DEVELOPMENT.md → W2: lane D (`docsync` after the §3 edit → the forwards → `-cases` → `teeth.py` + the pin over fixtures → BA13 → the floor → the measured counts) → the coordinator writes `instruments`, `proofs` `min_wants` 35, re-kinds `go.mod`s → T3 → review + break-it (a hollowed driver among its plants) → fix round → T4.

## 7. The documents at landing (the coordinator)

ADR-0015 §1.2 (registry-lint as built), §2 (the as-built instrument ids; `testhygiene` as archcheck's `-tests` mode; `suppressions` as built), §3.6 (the fence's known escapes — `<-ctx.Done()` on a short deadline, `syscall.Nanosleep`, an `exec` of `sleep`, a non-test helper file — QA-L3; the canary's silence is S1.1c-3's), §3.8 (the teeth as built), §3.10 (the one grammar as built; the deps register keeps its own schema and gains `sprint` and `ref` — §4.6a); DEVELOPMENT.md (§2.7); testing.md (no Go wait helper; the context cap; yield on every pass of a poll); ARCHITECTURE §3 (§4.1).
