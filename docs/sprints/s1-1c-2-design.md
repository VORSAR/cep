# S1.1c-2 — design note: the exemptions and the teeth (v2, 2026-11-12 — G1)

*Step 1 of the full loop for S1.1c-2 ([sprint model §6](00-sprint-model.md#6-the-full-loop)), from the answered [plan check](s1-1c-2-plan-check.md) ([decisions, 2026-11-12](decisions.md#owner-decisions-2026-11-12--s11c-2s-plan-check)). A build-ready spec: every path, signature, failure text, test and step a lane needs. v1 (0d6ac6c) was reviewed blind by the architect with the-fool ([review](../reviews/s1-1c-2-design-architecture.md): 4 blockers, 14 should-fix, 13 later) and by QA for testability ([review](../reviews/s1-1c-2-design-qa.md): 4 blockers, 10 should-fix, 5 later); the coordinator re-ran the tool claims that change the design (§0); §9 places every finding; the owner answered the two questions on 2026-11-12 ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-2s-design-after-the-reviews)). Card: [sprint plan — S1.1c-2](sprint-plan.md#s11c-2--the-exemptions-and-the-teeth). Code claims cite `file:line` on `s1-1c-2/integration` @ f8e6350.*

**The rule this version follows (both reviews' root finding):** every recogniser and every strength rule below is defined by what the pinned tools and the real tree do, measured in §0 — never by what their documentation suggests — and each measured table is pinned in a test so a tool bump that changes it is red.

## 0. Measured before G1 (sprint model §10: a library claim is proven by a scratch run)

**0.1 The tier.** Every row receives `GATE_TIER` (`gate/runner/env.py:29-30`). A tier label is not a schedule: a row runs at the nightly only if its `tiers` hold `nightly` (`gate/ci/generate.py` builds `nightly.yml` from the rows selected at that tier). → the `suppressions` row's tiers become T1–T4 **and nightly** (§2.4).

**0.2 What pinned ruff 0.17.3 honours** (`ruff format --isolated`, scratch; the architect's and QA's runs agree with the coordinator's):

| Form on the line | ruff keeps the layout? |
|---|---|
| `# fmt: skip` · `# fmt:skip` · `# fmt: skip  # r` · `# fmt: skip # r` · `# r  # fmt: skip` · `# noqa # fmt: skip` | **yes** |
| `# fmt: skip -- r` · `# fmt: skip; r` · `# FMT: SKIP` | no (silently reformatted) |
| `# yapf: disable` … `# yapf: enable` (region) | yes |
| `# isort: skip`, `# isort: skip_file`, `# ruff: isort: skip_file` | they pass an unsorted import block under `--ignore-noqa` |
| `[lint.extend-per-file-ignores]` in `ruff.toml` | waives a rule (e.g. the `time.sleep` ban) |

Python's `tokenize` yields **one** COMMENT token per line: `# fmt: skip  # reason here` is one token (coordinator's run).

**0.3 What pinned golangci-lint v2.15.0 honours** (`run --no-config --default=none --enable=errcheck --max-same-issues=0 --max-issues-per-linter=0`; the defaults' caps hide findings; the architect's table, four rows re-run by the coordinator):

| Form | Honoured (finding suppressed)? | nolintlint reports it? |
|---|---|---|
| `//nolint:errcheck // …` | yes | no (canonical) |
| `// nolint:errcheck` · `//  nolint:errcheck` · `//nolint errcheck` · `// nolint` | yes | yes |
| `///nolint:errcheck` | **yes** | **no** |
| `//nolint:all` | yes | (by config) |
| `//NOLINT:…` · `/* nolint:… */` · `/*nolint:…*/` · `//<TAB>nolint:…` · `// x nolint:…` · `// text //nolint:…` | no | — |

**0.4 The strength rule over the real drivers** (the architect's `verdict_scan.py`, re-run by the coordinator on HEAD): under v1's rule, eight of the 32 drivers fault — `test_hook_commit.py` (7 cases, its own `assertRefusedUnformatted`), `test_regen_baseline.py` (4, a tool's refusal, not a gate run), `test_tripwire_planted.py` (2 left after W0-F2: `assertEqual(record["verdict"], "fail")`), `test_notbuilt_expired.py` (2: `assertExpired` → `assertFailReason` → `assertGateFailed`, two levels), `test_leak_planted.py` (1), `test_parity_hash.py` (1, a load refusal: exit 2, no records), `test_lawdelta_planted.py` (1, a green control), `test_differential_swap.py` (plants nothing red by the rule). And QA's scratch: v1's rule stays green when a case's red assertion is swapped for `assertParityPassed` (DQ-B4).

**0.5 The Go proof files** (QA's and the architect's reads): the dedicated files are `internal/parser/errorcounts_test.go` (1 test), `internal/runtime/nosleep_test.go` (1), `internal/expr/docsync_test.go` (2: docsync (c)), `internal/parser/keywords_test.go` (4: docsync (b)'s keyword half); docsync (b)'s catalog half — `TestDocsyncStaticCatalog`, `TestDocsyncSeesPlantedDrift` — sits in `internal/validate/static_test.go` (7 tests, a general engine file). `tools/archcheck/archcheck_test.go` ranges over **inline** map literals inside `t.Run` closures (≈ `:230`, `:256`); `static_test.go:75`, `:119-124` range over function results.

**0.6 Shim callers** (DQ-S3, DA-L10): `rows._run_module` (the `pytests`, `proofs` and `pyfloor` rows), `gate/proofs/drivers/test_pyfloor.py:52-57` (direct, T4 only), `gate/runner/tests/test_pyshim.py` (direct, T2).

**0.7 Already true** (v1 §0.3–0.4): `go list -m all` selects every graph module (so the graph-only forward is one guard); T2 on the G0 commit is green but the three stubs; `go list -e ./...` lists the 20 packages offline even with an empty module cache, writing `go: downloading …` to stderr (DQ-L5).

## 1. The law model

| Law file | Kind (LAW Δ) | Checker | Pin | Clock |
|---|---|---|---|---|
| `gate/registry.json` | `registry` | `registry.py`; `ci-fresh` | `instruments` (§4.2) | — |
| `gate/law.json` | `law` (paths; instruments) | `lawdelta` (liveness) | `test_law_pin.py` | — |
| `gate/current-sprint`, `gate/sprints.json` | `marker`, `sprints` | `notbuilt` | `test_sprints_pin.py` | the plan's ✓ rows |
| `tools/allowlists/*.json` | **`allowlist`** (one line per changed entry) | **`suppressions`**; archcheck (purity, clock, testhygiene targets); the teeth pin (teeth); the ruff pin (ruff) | — | TEMP `expiry`: red at T4 and nightly |
| in-code exemptions (`//nolint`, `# fmt: skip`) | **named in the exemption section** (§2.6, owner 2026-11-12) | `suppressions` | — | TEMP `expiry` |
| `gate/known-issues.json` | **`known-issues`** | `test_known_issues_pin.py`; S1.1c-3's `aging` | the Markdown table | rows after `first_seen`; `forward` |
| `tools/deps-allowlist.json` | `deps` | `deps` (+ `sprint`, `ref`, §4.6a) | — | — |
| `ruff.toml` | `code` | `ruff`; the ruff pin | `ruff.json` both ways | — |
| `go.mod`, `gate/proofs/go.mod` | **`gomod`** | `deps` | — | — |
| the four pin tests, the dedicated Go proof files | `code`, each an **exact** entry (§4.2) | `pytests`, `unit` | `instruments` | — |
| `gate/`, `tools/`, `.githooks/`, `Makefile`, the corpus | `code` (grouped) | the rows | — | — |

## 2. Lane A — the grammar, the anchors, the suppressions row, the LAW Δ kinds, the exemption section (python-engineer, W1)

### 2.1 `gate/runner/anchors.py` (new, pure; moved from `rows/doclinks.py:25-77`)
`FENCE`, `HEADING`, `HTML_ANCHOR`, `slug`, `prose`, `anchors` move verbatim; `doclinks.py` imports them (its tests unchanged and green). New:
```python
def split_ref(ref: str) -> tuple[str, str]          # ValueError texts in §2.8
def resolves(ref: str, read: Callable[[str], str | None]) -> str   # "" or the why (§2.8)
```
`read(path)` returns the text of a **tracked** Markdown file or `None`; the row builds it from `git ls-files '*.md'` once (DA-L5: an untracked local doc never satisfies a ref).

### 2.2 `gate/runner/allowlists.py` (new, pure) — the one set of field validators
```python
RULES = frozenset({"purity", "clock", "teeth", "ruff", "testhygiene"})   # S1.1c-3 adds "govulncheck"
@dataclass(frozen=True)
class Ctx:   today: date; tier: str; order: sprintid.Order; current: sprintid.ID; issues: frozenset[str]; resolve: Callable[[str], str]
@dataclass(frozen=True)
class Entry: target: str; kind: str; reason: str; sprint: str; ref: str = ""; expiry: str = ""; issue: str = ""
@dataclass(frozen=True)
class AllowList: name: str; rule: str; entries: tuple[Entry, ...]
def load(text: str, name: str) -> AllowList                       # ValueError, texts §2.8
def fields(kind: str, reason: str, ref: str, expiry: str, issue: str, sprint: str, ctx: Ctx) -> list[str]   # the field rules, texts §2.8
def sprint_fault(sprint: str, ctx: Ctx) -> str                     # "" or why
def ref_fault(ref: str, ctx: Ctx) -> str                           # "" or why
def check(lst: AllowList, ctx: Ctx) -> list[str]                   # fields() per entry, prefixed "tools/allowlists/<name>.json: entries[<i>] (<target>): "
def expiring(items: Iterable[tuple[str, str]], today: date) -> list[tuple[str, int]]   # (label, expiry) -> (label, k), k = (expiry - today).days, for every k ≤ 14 (past ones, k < 0, all included)
```
**`fields`, `sprint_fault`, `ref_fault` are the only implementation of the field rules**; `suppressions.check_go` and `deps.compare` (§4.6a) call them (DA-S1). Rules: `kind` ∈ {PERMANENT, TEMP}; `reason.strip()` ≥ 12 characters; PERMANENT has `ref` (resolving), no `expiry`/`issue`; TEMP has `expiry` and `issue`, no `ref`; `expiry` is ISO `YYYY-MM-DD`; `0 ≤ (expiry − today).days ≤ 90` else `… more than 90 days ahead` (the bound is from the run's date, not the entry's commit — v1 changed this silently; DA-L13); an expiry before `today` is red only when `ctx.tier ∈ {"T4", "nightly"}`; `issue ∈ ctx.issues`; `sprint` parses, is a row or part of `ctx.order`, and is not after `ctx.current` (a part of the current row compares by part: `S1.1c-9` after `S1.1c-2`). Unit tests inject `Ctx` from fixtures and **never** read `gate/current-sprint`, `gate/sprints.json`, `gate/known-issues.json` or the clock (DQ-S9).

### 2.3 `gate/runner/suppressions.py` (new, pure) — recognisers measured in §0.2–0.3
```python
ENABLED: frozenset[str] = frozenset()   # S1.1c-4 sets it from its golangci config (DA-L4: a TOML or JSON config, the stdlib reads both)
PREFILTER = ("nolint", "fmt: skip", "fmt:skip", "noqa", "yapf:", "isort:", "fmt: off", "fmt: on")   # git grep -e patterns (§2.6); a test asserts every recognised form contains one
@dataclass(frozen=True)
class Directive: path: str; line: int; linters: tuple[str, ...]; kind: str; meta: dict[str, str]
def go_comments(source: str) -> Iterator[tuple[int, str]]
def go_directives(path: str, source: str) -> tuple[list[Directive], list[str]]     # (directives, faults)
def py_markers(path: str, source: str) -> tuple[list[Directive], list[str]]       # `# fmt: skip` markers as directives (linters ("fmt",), kind "MARKER", meta {"reason": …}); faults
def check(directives: Iterable[Directive], ctx: Ctx, enabled: frozenset[str]) -> list[str]
```
**`go_comments`** — states `CODE`, `LINE`, `BLOCK`, `STRING` (`"`, `\` escapes, ends at `"` or newline), `RAW` (`` ` ``, spans lines), `RUNE` (`'`, `\` escapes); yields `(line, text after the first "//")` for line comments only; one test per state, and `"//nolint:x"` inside a string yields nothing.
**A Go directive (§0.3):** strip leading `/` and spaces/no tabs from the comment text; if it then matches `^nolint( |:|$)` it is a directive golangci honours. It must be written in the canonical form — exactly `//nolint:<l>[,<l>]` (two slashes, no space, a non-empty comma list without a trailing comma) followed by ` // ` and the metadata — else red `write //nolint:<linter> (two slashes, no space)`. Text matching `^NOLINT`, block comments and `// x nolint` are not directives (golangci ignores them). Metadata: `(PERMANENT|TEMP)` then `key=value` pairs; `reason` double-quoted (a `\"` inside it is allowed; an unterminated quote is a fault); keys `reason`, `ref`, `expiry`, `issue`, `sprint`; a duplicate or unknown key is a fault. `check` applies `allowlists.fields` and then, per linter not in `enabled`, `<l> is not enabled (none is until S1.1c-4)`.
**A Python marker (§0.2):** each COMMENT token is split on `#` into stripped segments. A segment equal to `fmt: skip` or `fmt:skip` is a skip; its reason is the other segments joined by `  # ` (a `noqa` segment is not a reason), ≥ 12 characters. Red: a segment starting with `fmt: skip`/`fmt:skip` and continuing (`fmt: skip -- r`, `fmt: skip; r`) — `ruff ignores this form`; case variants (`FMT: SKIP`); `fmt: off`/`fmt: on`, `yapf: disable`/`yapf: enable`, `isort: skip`, `isort: skip_file`, `ruff: isort: skip_file` — `write # fmt: skip  # <reason> on the statement`; any `noqa` segment — `ruff runs --ignore-noqa: this comment waives nothing; remove it`. Scope: tracked `gate/**/*.py` **and** `.githooks/*` (DA-L1, DQ-L2).
**Pinned tables:** `test_suppressions.py` holds §0.2's and §0.3's tables as data and asserts each form's classification (QA's and the architect's probes); S1.1c-4 re-runs §0.3 against the binary as a differential.

### 2.4 The row `gate/runner/rows/suppressions.py` (replaces the G0 stub)
Registry: tiers **T1, T2, T3, T4, nightly** (DA-B3; the coordinator re-tiers and regenerates — `nightly.yml` gains a `lint` job). `main`: `rows.start()`; `Ctx` from `gate/sprints.json`, `gate/current-sprint`, `gate/known-issues.json` ids, `datetime.now(timezone.utc).date()`, `os.environ["GATE_TIER"]`, `resolves` over tracked Markdown; every `tools/allowlists/*.json` loaded (faults red) and checked; per `{files}` entry: `.go` → `go_directives`; `gate/**/*.py`, `.githooks/*` → `py_markers`; then `check`. **The ruff pin:** `tomllib` reads `[lint.per-file-ignores]` **and** `[lint.extend-per-file-ignores]`; each `(file, code)` ↔ a `ruff.json` target `"<file>: <code>"`, both ways; a non-empty `lint.ignore` or `lint.extend-ignore` is red. **Stale targets** of `ruff.json` (a file that does not exist) and `teeth.json` (a row id the registry lacks) are red (DA-L2). **Covered** = the number of `{files}` entries processed, skipped ones included; side inputs are never counted (DA-S9). No network, no git beyond `ls-files`.

### 2.5 The LAW Δ kinds — one line per changed entry (`law.py`)
A kind's compare may now return **several lines**: `delta()` keeps its one-liner contract for every other kind; `allowlist` and `known-issues` return `list[str]`, which `law.block` renders as the file's line followed by indented `  ` sub-lines, at most 6, then `  (and <k> more)` (DA-S11). **`allowlist`:** sub-lines `+<target> <KIND> <sprint> ref=<ref>|expiry=<d> issue=<id>`, `−<target>`, `<target>: <field> <old> → <new>` (`reason changed`); the file line ends `(<n> entries)`; a new list `file added: <n> entries` then its entries. **`known-issues`:** `+<id> first seen <s>`, `−<id>`, `<id>: forward <a> → <b>`, `<id>: decision set|changed|cleared`. The coordinator re-kinds `tools/allowlists/` and `gate/known-issues.json` when lane A lands.

### 2.6 The exemption section and the countdown (`rows/lawdelta.py`, `gate/runner/git.py`)
After the block (or `HEAD_IS_MAIN`/`NONE`), before the uncommitted-edits line:
- `exemptions: <n> in code, <m> listed (<±d> this diff)` — `n` = directives and markers (§2.3) in tracked `.go`, `gate/**/*.py`, `.githooks/*` at the revision; `m` = entries of `tools/allowlists/*.json`; `d` = (n + m at HEAD) − (n + m at the merge base), signs `+`/`−` (U+2212).
- **When `n` changed** (owner, 2026-11-12, design 1): one sub-line per added or removed in-code exemption, at most 6 then `  (and <k> more)`: `  +internal/x/y.go:12 //nolint:gosec PERMANENT S1.1c-4 ref=<ref>`, `  −gate/runner/z.py:40 # fmt: skip`. A directive is matched across revisions by `(path, linters, kind, meta)`, not by line number, so a moved line is neither added nor removed.
- **The countdown:** for every TEMP entry and TEMP directive at HEAD with `−∞ < k ≤ 14` from `allowlists.expiring`: `  exemption <where>: <target> expires <date> (in <k> days)` for `k ≥ 0`, `  exemption <where>: <target> expired <date> (red at T4 and nightly)` for `k < 0` (DA-S2). `lawdelta` takes its date from one function, `today()`, patched in tests (DQ-S9).
- **git, with an error contract** (DA-S3): `git.grep_files(rev, patterns, pathspecs) -> list[str]` (`git grep -l -e <p>… <rev> -- <specs>`; exit 0 → the paths with the `<rev>:` prefix stripped; exit 1 → `[]`; any other → `GitError` naming the command, which lawdelta turns into FAIL); `git.ls_tree(rev, prefix) -> list[str]` the same; the patterns are `suppressions.PREFILTER`.
Nothing else prints for a list or file that did not change (decision 2).

### 2.7 DEVELOPMENT.md (the coordinator, when lane A lands)
Examples become (DQ-S6, DA-S14): `//nolint:gosec // PERMANENT reason="fixture generator; output is never security-relevant" ref=docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md#2-architecture-constraints-as-analyzers-with-teeth-reference-a-in-go sprint=S1.1c-2` and `//nolint:errcheck // TEMP reason="close error surfaced by the sink rework" expiry=2026-12-31 issue=S3.3-2-1 sprint=S1.1c-2`; the rules paragraph states the canonical form, `ref`'s form, TEMP's `expiry` + `issue`, the 90-day bound, red at T4 and nightly, and `# fmt: skip  # <reason>`. `test_suppressions.py` reads these fenced examples and checks each with `Ctx(today=2026-11-12, tier="T2", order=<fixture>, current=S1.1c-2, issues={"S3.3-2-1"})` and the linter injected as enabled.

### 2.8 Exact failure texts (lane A; QA's W0 asserts them — DQ-S8)

| Where | Text (after the row's `suppressions: ` prefix where noted) |
|---|---|
| `split_ref` | `ref is empty` · `ref must be a repository path, not absolute` · `ref must not climb out with ..` · `ref has more than one #` |
| `resolves` | `ref has no anchor: write <path>#<heading-anchor>` · `<path> is not a tracked Markdown file` · `<path> has no heading #<anchor>` |
| `load` | `tools/allowlists/<name>.json: not JSON: <e>` · `…: duplicate key <k>` · `…: keys <got>, want schema, note, rule, entries` · `…: schema <s>, want 1` · `…: note is not a string` · `…: rule <r> is not one of <sorted RULES>` · `…: rule <r> but the file is <name>.json` · `…: entries is not a list` · `…: entries[<i>]: unknown key <k>` · `…: entries[<i>]: missing <k>` · `…: entries[<i>]: <k> is not a string` · `…: entries[<i>]: target <t> is listed twice` |
| `fields` | `kind must be PERMANENT or TEMP` · `reason must be at least 12 characters` · `PERMANENT needs ref=<path>#<anchor>` · `PERMANENT takes no expiry or issue` · `TEMP needs expiry=YYYY-MM-DD and issue=<known issue id>` · `TEMP takes no ref` · `expiry must be YYYY-MM-DD` · `expiry <d> is more than 90 days ahead` · `expired <d>` · `issue <id> is not in gate/known-issues.json` · `sprint <s> is not a row of gate/sprints.json` · `sprint <s> is after the current sprint <c>` |
| Go directives | `<path>:<line>: write //nolint:<linter> (two slashes, no space)` · `…: a suppression carries its metadata: // PERMANENT reason="…" ref=… sprint=…` · `…: unterminated quote` · `…: duplicate key <k>` · `…: unknown key <k>` · `…: <field text>` · `…: //nolint:<l>: <l> is not enabled (none is until S1.1c-4)` |
| Python markers | `<path>:<line>: ruff ignores this form; write # fmt: skip  # <reason>` · `…: write its reason as another comment: # fmt: skip  # <reason, 12+ characters>` · `…: write # fmt: skip  # <reason> on the statement` · `…: ruff runs --ignore-noqa: this comment waives nothing; remove it` |
| ruff pin | `ruff.toml waives <code> for <file> without an entry in tools/allowlists/ruff.json` · `tools/allowlists/ruff.json lists <file>: <code>, which ruff.toml does not waive` · `ruff.toml sets lint.<key>: write per-file waivers` |
| stale | `tools/allowlists/<name>.json: <target>: does not exist (a stale target)` |

## 3. Lane C — archcheck reads its lists; `-tests`; the two waits (go-engineer, W1)

```go
type Lists struct { Purity map[string]map[string][]string; Clock, Tests map[string]bool }
func LoadLists(root string) (*Lists, error)            // reads <root>/tools/allowlists/{purity,clock,testhygiene}.json; rule must match; a target that does not split is an error
func CheckSources(rules *Rules, lists *Lists, path string, files map[string]string) ([]string, error)   // the lists replace syncNamed and the cron.go literal
func CheckTest(lists *Lists, path, src string) []string // the sleep fence for one _test.go
func Stale(lists *Lists, root string) []string          // targets whose package dir or file is gone
```
`LoadLists` reads JSON into `struct{ Rule string; Entries []struct{ Target string } }` (unknown fields ignored: the Python row is the grammar's validator — A-L2). `main` finds `root` as the working directory (the runner runs archcheck at the repository root); tests pass `filepath.Join("..", "..")` (DA-S7). purity targets `"<pkg>: <import>.<Ident>"`; clock targets `"<file>: time"` (empty now); testhygiene targets `"<file>"` (empty). **`-tests` mode** (`go run ./tools/archcheck -tests <files.nul>`): a non-`_test.go` entry is an internal error (exit 2); each file → `CheckTest`: a selector on the file's local name(s) for `time` naming `Sleep`, `After`, `AfterFunc`, `Tick`, `NewTimer`, `NewTicker` → `failed: <file>:<line>: testhygiene: time.<Name> in a test: wait for a state, cap it with a context, yield on every pass of a poll (testing.md rule 1)`; a dot-import of time → `failed: <file>:<line>: testhygiene: a dot-import of time hides the calls the fence reads`; then `Stale` → `failed: tools/allowlists/<name>.json: <target>: does not exist (a stale target)`; `GATE_COVERED <test files read>`. **The waits** (`io/inmemory/breakit_s3_3_1_test.go:47,54`): red first — quote `-tests`' two findings on the real tree — then each becomes a `context.WithTimeout` cap with `case <-ctx.Done(): t.Fatal(<same message>)`; the test passes alone and under `-race -count=20`. **QA's one-line support edit** (DQ-S5, DA-S8): `support.copy_hooks` (`gate/proofs/drivers/support.py:364`) copies `tools/allowlists/`, so `test_hook_commit.py` and `test_hooks_off.py` stay green at T4; lane C runs both hook drivers by hand on landing. The coordinator then switches the `testhygiene` command to `["go", "run", "./tools/archcheck", "-tests", "{files}"]` and deletes the stub. *(Plan check A-L11 — archcheck reads the lists from the index snapshot at T0 — is not carried: the hook's T0 snapshot is the staged tree, and the lists are law paths whose changes LAW Δ names; recorded, DA-L7.)*

## 4. Lane D — the teeth, `docsync` (a), the forwards, BA13, the floor (python-engineer, W2)

### 4.1 `docsync` (a)
`gate/runner/docsync.py` (pure): `tree(md) -> list[str]` (the first fence after the `## 3.` heading, via `anchors.prose`), `listed(lines) -> set[str]`, `missing(packages, listed) -> list[str]`. `rows/docsync.py`: `go list -e -f '{{.Dir}}' ./...` (stderr ignored, exit code read; `GOFLAGS=-mod=readonly`, `GOPROXY=off`) → repository-relative dirs → `docsync: <dir> is a Go package and ARCHITECTURE §3 does not list it`; `GATE_COVERED <packages>`. The coordinator adds `tools/archcheck/`, `tools/apipartition/`, `tools/shape/` to §3 and annotates `tools/analyzers/` at W2's start.

### 4.2 The instrument pin — two shapes, a strength rule measured on the real drivers
`law.json` gains `"instruments"` (`law.TOP_KEYS`; `load_manifest` → `Manifest.instruments`). Lane D's branch carries `"instruments": []` as a named data edit until the coordinator writes the list, so `law.json` loads at every step (DA-S13); `_delta_law` reads an old side with `.get("instruments", [])`.
```python
@dataclass(frozen=True)
class Proof:      path: str; cases: int; test: str = ""     # test: count only this Go test function (and its subtests)
@dataclass(frozen=True)
class Instrument: id: str; row: str; lands_in: str; code: tuple[str, ...]; proofs: tuple[Proof, ...]
                  tiers: tuple[str, ...] = (); min_wants: int = -1; expected_scope: tuple[str, ...] = (); go_test: str = ""; exempt: str = ""
```
**`gate/runner/teeth.py` is pure** (DA-L9): `python_cases(source: str) -> tuple[list[str], list[str]]` (case names, faults — by `ast`, no import) and `go_cases(listing: str) -> tuple[list[str], list[str]]` (parses archcheck's `-cases` output); the pin test does the I/O (reads files; runs one `go build -o <tmp>/archcheck ./tools/archcheck` per test class, then `-cases <files...>` once — DQ-L4, DA-L3). **The strength rule** (DQ-B4, plan check §6 teeth (8)): a counted Python case must reach, in live code (not after `return`/`raise`, not under `if False`, not in a nested `def` never called), either **a red helper** — `assertRed`, `assertGateFailed`, `assertParityStops`, `assertRow` with `ok` the literal `False` (positional or keyword), or the new `support` helpers `assertRefusedAtLoad(run, reason)` (exit 2, no records), `assertHookRefused(result, reason)`, `assertRegenRefused(result, reason)` — or **a pass on the row under test** — `assertRow(..., ok=True)` whose row id is not `"parity"`. `assertParityPassed` alone never counts. Indirection: up to two levels through helpers of the driver's own module or `support`. A driver needs ≥ 1 red case; 0 cases is a fault. A counted Go case (`-cases`) must call `Fatal`, `Fatalf`, `Error`, `Errorf` or `FailNow` on its `*testing.T`, directly or through a same-file helper (DA-S12).
**QA's W0 converts the eight faulting drivers** (§0.4) to these helpers and adds the three helpers to `support.py`; the coordinator re-runs `verdict_scan` → zero faults before lane D starts.
**Go-test instruments:** `docsync-b-catalog` (`internal/validate/docsync_test.go`, new: lane D moves `TestDocsyncStaticCatalog` and `TestDocsyncSeesPlantedDrift` out of `static_test.go` unchanged), `docsync-b-keywords` (`internal/parser/keywords_test.go`), `docsync-c` (`internal/expr/docsync_test.go`), `docsync-d` (`internal/parser/errorcounts_test.go`), `nosleep` (`internal/runtime/nosleep_test.go`); each file an **exact** `law.json` entry (`code`), decision 9's "dedicated test file" (DA-B4, DQ-S2).
**`-cases`** (archcheck, owned by lane D in W2): `case <Test>`; `case <Test>/<literal>` per `t.Run("<literal>", …)`; nested `t.Run` names join with `/`; a `for … range <x>` whose body calls `t.Run` with a non-literal name counts the elements of `<x>` when `<x>` is an **inline** composite literal or an identifier bound in the same function to one (a `[:n]` slice of it counts `n` when `n` is a constant); anything else → `fault <Test>: t.Run over a table -cases cannot count` (DQ-S1). Fixtures in `archcheck_test.go`: an identifier literal, an inline literal, a function call (fault), `[:2]`, a nested `t.Run`, a case with no `Fatal`/`Error` (fault).
**The pin's tests and their exact law entries** (DA-S5): `gate/runner/tests/test_instruments_pin.py`, `test_known_issues_pin.py`, `test_law_pin.py`, `test_sprints_pin.py` are exact `law.json` entries (deleting one is a `lawdelta` liveness red and a named `law` line); `pytests` `min_wants` becomes the measured module count. Tests 1–6 as v1 (registry ↔ row instruments both ways; tiers, `blocking`, `lands_in`, `min_wants`, `expected_scope` pinned; `go_test` listed by `-cases`; code and proofs exact law paths; cases counted; `exempt` names a `teeth.json` entry). Counts are measured, never hard-coded in tests: the pin compares the manifest with the files (DQ-S10).

### 4.3 BA13 — fixed for the accidental forge; the deliberate one named (owner, 2026-11-12, design 2)
`pyshim.main(argv)`: `argv = [prog, root, name, report]` — **required**; another count exits 2 with `usage: pyshim.py <root> <dotted.module> <report-file>`. It opens `report` before importing the module, writes its one final line there, closes it, and still prints it. `rows._run_module` passes `<out_dir>/<name>.report` (removed first) and reads only that file. Every direct caller moves to three arguments in the same commit: `test_pyfloor.py:52-57` (run by hand, T4-only) and `test_pyshim.py` (DQ-S3). The residue — a module that writes the report path from `sys.argv[3]` itself and exits 0 — is named in ADR-0015 §3.6 (review-only residue: every test module is a law path, so such a module is a named LAW Δ line) and planted in `test_shim_forged_pass.py`: the stdout forge is red; the file forge is documented green with that sentence (DA-S6, DQ-S4). BA13 leaves `known-issues.md` and `gate/known-issues.json` at close-out, together.

### 4.4 The floor; 4.5 the forwards; 4.6a the deps register; 4.7 the known-issues pin
- **Floor:** `--floor` required in `gate/coverage/main.py` (the module's usage error, exit 2); the registry passes `--floor 90`.
- **`gomod`** kind: `module`, `go`, `toolchain`, `require` (single and block, `// indirect`), `replace`, `tool`; one-liners `+require <m> <v>`, `−require <m>`, `<m> <v1> → <v2>`, `+replace <m> => <t>`, `+tool <p>`, `toolchain <a> → <b>`, `go <a> → <b>`; the coordinator re-kinds both module files.
- **Graph-only guard:** `deps: <path> is in the module graph and selected by neither module's build list`.
- **Three headings:** `law.doc_line` reads headings through `anchors.prose` and `anchors.HEADING` (fences skipped — DA-L11); the first three distinct headings of non-marker hunks, then `(and <k> more)`; marker-only hunks (changed lines differing only by `<!--cep…-->`/`<!--text-->` comments; one regex constant, `law.MARKER_COMMENT`, shared with S1.1c-3 — DA-L8) are counted `+<n> markers`.
- **Deps register:** entries gain `sprint` and `ref`; `deps.compare(modules, graph, entries, gomods, read, ctx: allowlists.Ctx)` calls `allowlists.sprint_fault`/`ref_fault` (DA-L6); the coordinator writes the 25 entries' values at W2's start.
- **Known-issues pin:** as v1 §4.7, plus `S1.1c-10` not matching `S1.1c-1`, and a decision section that does not name the id → red.

## 5. QA's wave 0 for the lanes (red for the stated reason; unit tests on the owning lane's starting branch)
- **Before lane D (on integration, QA):** convert the eight drivers (§0.4); add `assertRefusedAtLoad`, `assertHookRefused`, `assertRegenRefused` to `support.py`; `copy_hooks` copies `tools/allowlists/`; the coordinator re-runs the scan → zero faults, and the `proofs` row stays PASS at T4.
- **Lane A's branch:** `test_anchors.py`, `test_allowlists.py` (every §2.8 text; the 12-character boundary green; exactly 90 days green; expired at T3 green, at T4 and nightly red), `test_suppressions.py` (one case per `go_comments` state; §0.2's and §0.3's tables; every directive fault; the policy's examples), `test_law.py` (the two kinds and their sub-lines and cap), `test_lawdelta.py` (the exemption line `+0`/`+1`/`−1`; named sub-lines; a moved directive neither added nor removed; the countdown at `k` = 14, 0, −1 with `today()` patched; nothing for an unchanged list), `test_git.py` (`grep_files`, `ls_tree`: exit 1 empty, exit 128 `GitError`).
- **Lane C's branch:** `archcheck_test.go` — purity from the list, a target that does not split, clock membership (`internal/window/cron.go` red with the empty `clock.json`, green with an entry), stale purity/clock/testhygiene targets, every banned name, a dot-import, a non-test file in `-tests` (exit 2), a listed testhygiene file skipped.
- **Lane D's branch:** `test_teeth.py` (parity-only, dead code after `return`, `if False:`, an uncalled nested def, `assertRow(ok=False)` by keyword, two-level indirection, zero cases), `test_instruments_pin.py` fixtures, `test_docsync.py` (stderr ignored), `test_known_issues_pin.py`, `gate/coverage/tests`, `test_deps.py` (graph-only; `sprint` after the marker; an unresolved `ref`), `test_law.py` (gomod incl. block require, `// indirect`, toolchain; exactly three headings; markers), `test_pyshim.py` (the third argument; the usage exit).
- **On integration** (T4-only, red on the stubs): `test_suppressions_planted.py` (dates planted ≥ 3 days from any boundary, matched as `in \d+ days`; counts asserted only as `(+<d> this diff)` — DQ-S9, DQ-S10), `test_testhygiene_planted.py`, `test_docsync_planted.py`, `test_shim_forged_pass.py`. `proofs` `min_wants` becomes the measured driver count at the end (36 if nothing else moves).

## 6. Order
G1 → QA's W0 (the driver conversions first, then the lanes' reds) → W1: lane A ‖ lane C (lane A's 40 `# fmt: skip` reasons: 11 of those lines would pass 120 characters — DQ-S7 — so lane A reflows them, quoting the red E501 run first) → the coordinator: re-kinds, the `suppressions` nightly tier and `testhygiene` switch, DEVELOPMENT.md, regenerated workflows → W2: lane D (the §3 edit and the deps refs first) → `instruments` written from lane D's measurements, `proofs` and `pytests` `min_wants` measured → T3 → review + break-it → fix round → T4. **Every G0 or law commit that moves the marker, a count or a date also runs the `proofs` row** (W0-F1's lesson).

## 7. Size
≈ 10.5 points (v1's ≈ 9, plus the measured recognisers, the in-code exemption naming, the strength rule and the eight driver conversions, the Go `-cases` table rules, the exact pin entries); the card holds 13.

## 8. The documents at landing (the coordinator)
ADR-0015 §1.2, §2, §3.6 (the fence's escapes; the shim's deliberate forge — BA13's residue), §3.8 (the teeth as built), §3.10; DEVELOPMENT.md (§2.7); testing.md; ARCHITECTURE §3; the record notes for S1.1c-4's plan check: cap PERMANENT entries per package or ask per group (plan check risk 6, DA-L12), a TOML or JSON golangci config (DA-L4), the §0.3 differential.

## 9. The reviews of v1 — every finding placed

| # | Sev. | Finding | Disposition |
|---|---|---|---|
| DA-B1, DQ-B1 | blocker | one COMMENT token per line: the `# fmt: skip  # r` rule could not be built | fixed: §0.2, §2.3 (split on `#`) |
| DQ-B2, DA-S4 | blocker / should-fix | ruff honours `# r  # fmt: skip`, `# fmt:skip`, yapf and isort forms; `extend-per-file-ignores` | fixed: §0.2 table pinned; §2.3, §2.4 |
| DA-B2, DQ-L3 | blocker | golangci honours `// nolint`, `///nolint`, `//nolint x` | fixed: §0.3 table pinned; §2.3 recogniser; canonical form required |
| DA-B3 | blocker | expired entries never checked at the nightly | fixed: §0.1, §2.4 (nightly tier) |
| DA-B4, DQ-B3, DQ-S2 | blocker | the strength rule red on 8 real drivers; Go instruments wrong, in a general file | fixed: §0.4, §4.2 (helpers, conversions in W0, dedicated `docsync_test.go`, exact entries) |
| DQ-B4, DA-S12 | blocker / should-fix | hollowing by `assertParityPassed`; Go cases without `Fatal` | fixed: §4.2's strength rule |
| DA-S1 | should-fix | three copies of the field rules | fixed: `allowlists.fields`, `sprint_fault`, `ref_fault` |
| DA-S2 | should-fix | no countdown for in-code TEMP | fixed: §2.6 |
| DA-S3 | should-fix | git helpers without an error contract | fixed: §2.6 |
| DA-S5 | should-fix | the pin can be deleted | fixed: §4.2 exact entries; `pytests` `min_wants` measured |
| DA-S6, DQ-S4 | should-fix | BA13's deliberate forge | → owner, design 2: fixed, residue named (§4.3) |
| DA-S7 | should-fix | lane C's lists had no signature | fixed: §3 |
| DA-S8, DQ-S5 | should-fix | hook worlds lack the lists | fixed: §3 (QA's `copy_hooks` edit) |
| DA-S9 | should-fix | covered wording | fixed: §2.4 |
| DA-S10 | should-fix | in-code exemptions only counted | → owner, design 1: named (§2.6) |
| DA-S11 | should-fix | one `; `-joined line | fixed: §2.5 sub-lines, cap 6 |
| DA-S13 | should-fix | `instruments` required mid-sprint | fixed: §4.2 (`[]` on lane D's branch; `.get`) |
| DA-S14, DQ-S6 | should-fix | the policy's examples fail the rules / time bombs | fixed: §2.7 |
| DQ-S1 | should-fix | `-cases` misses inline literals | fixed: §4.2 |
| DQ-S3 | should-fix | the shim's direct callers | fixed: §0.6, §4.3 |
| DQ-S7 | should-fix | 11 skip lines over 120 characters | fixed: §6 (lane A reflows, red first) |
| DQ-S8 | should-fix | inexact failure texts | fixed: §2.8 |
| DQ-S9 | should-fix | no injection for dates, order, marker | fixed: §2.2 `Ctx`, §2.6 `today()`, §5 |
| DQ-S10 | should-fix | hard-coded counts | fixed: §4.2, §5 (measured; relations only) |
| DA-L1, DQ-L2 | later | `.githooks/*` outside the scope | fixed: §2.3 |
| DA-L2 | later | stale ruff/teeth targets | fixed: §2.4 |
| DA-L3, DQ-L4 | later | one `go run` per file | fixed: §4.2 (one build, one run) |
| DA-L4 | later | S1.1c-4's config format | recorded for S1.1c-4 (§8) |
| DA-L5 | later | untracked docs satisfy a ref | fixed: §2.1 |
| DA-L6 | later | `deps.compare`'s signature | fixed: §4.6a |
| DA-L7 | later | T0 snapshot dropped silently | recorded with the reason (§3) |
| DA-L8 | later | one marker regex | fixed: `law.MARKER_COMMENT` |
| DA-L9 | later | `teeth.py` not pure | fixed: §4.2 |
| DA-L10 | later | `run_modules`' third caller | fixed: §0.6 |
| DA-L11 | later | headings inside fences | fixed: §4.5 via `anchors.prose` |
| DA-L12 | later | S1.1c-4's cap note | recorded (§8) |
| DA-L13 | later | the 90-day base moved silently | stated in §2.2 |
| DQ-L1 | later | two counter-examples are not tests | fixed: §3 (a quoted run), the driver for the scope |
| DQ-L5 | later | `go list` stderr offline | fixed: §0.7, §4.1 |

## 10. Clarifications after QA's wave 0 (the coordinator, 2026-11-12; QA's lane reds at 5ffaae7 and 9004d7b follow them)

QA marked 21 places where v2 left a choice open (`design ambiguity:` in the tests); each is settled here, QA's reading kept unless stated.
1. `split_ref` without `#` or with an empty fragment returns `(path, "")`; "has no anchor" is `resolves`' text. 2. In `ref has no anchor: write <path>#<heading-anchor>`, `<path>` is filled in, `<heading-anchor>` stays literal. 3. `resolves` never raises; `split_ref`'s texts come back as its reason. 4. `load`'s key fault is `…: keys <a, b, c>, want schema, note, rule, entries` with the present keys sorted and `, `-joined. 5. `<sorted RULES>` is `", ".join(sorted(RULES))`. 6. An unresolved PERMANENT `ref` returns `resolve`'s reason verbatim, no prefix. 7. An unparseable `sprint` is `sprint <s> is not a row of gate/sprints.json`. 8. `write //nolint:<linter> (two slashes, no space)` carries the linters as written (`write //nolint:errcheck …`); a bare `//nolint` keeps `<linter>` literal. 9. A canonical `//nolint:<l>` without ` // ` metadata gets the metadata text. 10. `# FMT: SKIP` gets `ruff ignores this form; write # fmt: skip  # <reason>`. 11. `# noqa # fmt: skip` gives both faults. 12. `PREFILTER` is case-sensitive like `git grep`; case variants are not exemptions (they are red as `ruff ignores this form` but never counted), so the PREFILTER test leaves them out. 13. For `allowlist` and `known-issues`, `delta` returns `[summary, *sub_lines]`, the summary `(<n> entries)` or `file added: <n> entries`; `block` prints `<path> +a/−b — <summary>` then the sub-lines indented two spaces, the cap of 6 applying to sub-lines. 14. Sub-lines are ordered: added (by target or id), changed, removed. 15. The exemption line follows `HEAD_IS_MAIN` too, with `+0`. 16. An added marker's sub-line is `  +<path>:<line> # fmt: skip` (the reason is not shown). 17. Countdown: for a list entry `<where>` is the list's path and `<target>` its target; for a directive `<where>` is `<path>:<line>` and `<target>` its linters `,`-joined. 18. A `GitError` makes `lawdelta` FAIL with `lawdelta: git grep failed (exit <code>)` / `lawdelta: git ls-tree failed (exit <code>)`. 19. **`expiring` returns every entry with k ≤ 14, all past ones included** (§2.6's −∞ < k; §2.2's comment corrected). 20. `CheckTest` returns findings without the `failed: ` prefix; `main` adds it, as for `CheckSources`. 21. `LoadLists` ignores the grammar's other fields; a missing list file is an error naming it (archcheck exits 2).

## 11. Clarifications for lane D (the coordinator, 2026-11-12; QA's lane D reds at f87a59d follow them)

Lane D is two lanes in one wave: **D1** (python-engineer: `teeth.py`, the pin, `knownissues.py` and its pin, `docsync`, BA13, the floor, `gomod`, the graph-only guard, three headings and markers, the deps register's `sprint` and `ref`) and **D2** (go-engineer: archcheck's `Cases` and `-cases` mode). Their one seam, fixed here:
- **`Cases(path, src string) (cases, faults []string)`** returns names without prefixes: a case `TestX` or `TestX/sub` (nested `t.Run` names joined with `/`; a range-counted table's elements `TestX/<ident>[<i>]`); a fault `TestX: <why>`. **`go run ./tools/archcheck -cases <file>...`** prints, per file in argument order and per item in `Cases` order, `<path>: case <name>` and `<path>: fault <TestX>: <why>`, then exits 0 (a file that does not parse → `<path>: fault : <parse error>`); `teeth.go_cases(listing)` groups by path. The no-verdict fault's why is `calls no Fatal, Fatalf, Error, Errorf or FailNow`; the cannot-count fault's why is `t.Run over a table -cases cannot count`. A parent test whose only failing calls are inside its `t.Run` closures counts.
QA's 22 open points: (1, 6) every fault names its path, driver case or instrument id — teeth: `teeth: <path>::<Class>.<method> asserts no gate verdict`, `teeth: <path> plants nothing red`, `teeth: <path> counts 0 cases`; the pin's texts are D1's to write, each naming the id and the field. (2) Python case names are `<Class>.<method>`. (3) `python_cases` counts a call on `self` to any name in `VERDICT`, `RED` or the three new helpers as that helper, wherever it is defined; the driver's own helpers resolve up to two levels. (4, 18) as the seam above. (5) `teeth.pin(manifest, rows, tracked, counted, teeth_targets) -> list[str]` as QA wrote it. (7) a package below a listed directory must be listed itself (exact match). (8) a first fence under `## 4.` is no fence for §3 → ValueError. (9) `go list` failing → `docsync: go list failed (exit <n>)`. (10) `gate/runner/knownissues.py`, `check(data, table, order, read) -> list[str]`, as QA wrote it. (11) `deps.Entry` gains `sprint` and `ref` as keyword fields. (12) a graph node with no entry gets both lines. (13) a document's line is `<headings>[ (and <k> more)] — <flag>[; +<n> markers]`. (14) a heading `5.2 …` stays its text (only a top-level `5. …` reads `§5`). (15) `+require <m> <v>` without `// indirect`. (16) a list field's change prints old → new joined by `, `. (17) the refusal names `gate/law.json: instruments[<i>]: proofs and exempt together`. (19) table elements as in the seam. (20) yes, it counts. (21) as the seam. (22) argparse's own text.
*(Annotated at the combine, 2026-11-12: lane D2's two readings, accepted — a table written inline is named after the loop's first named variable (`TestX/name[<i>]`), and a file that cannot be read gets the parse-error line `<path>: fault : <error>`; lane D1's — `python_cases(source, path)` takes the path so its faults name it, `go_cases` returns `{path: (cases, faults)}`, `+<n> markers` counts marker comments, the four `test_parity_*` drivers prove the runner's built-in parity step and belong to no registry row. The measured instruments (32 row, 5 Go-test) were written into `gate/law.json` with the Go proofs' counts from `-cases`; the pin over the real tree holds with no fault.)*
