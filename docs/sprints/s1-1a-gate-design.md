# S1.1a-1 — design note: the local gate and the event kernel (v2)

*Step 1 of the full loop for [S1.1a-1](sprint-plan.md#s11a-1--the-local-gate-coverage-and-the-event-kernel), by the coordinator. **v2, 2026-11-02**, revised after the blind design review: architect + the-fool **24** findings (4 blocker / 14 should-fix / 6 later), QA **36** (10 / 21 / 5). The coordinator re-verified every blocker by probe or reasoning; §11 gives each finding's disposition, and none is rejected. It also folds in the owner's decisions of 2026-11-02 ([sprint-plan §5](sprint-plan.md#5-decisions)).*

***v2.1**, the same day: after the owner's focused re-check. A fresh agent probed v2 against the real tools and archives. It confirmed 6 of 10 claims and refuted 4, and found 3 new blockers, 11 should-fix and 5 later issues. The coordinator verified the blockers and their fixes by probe. All are fixed below and listed in §11's "Re-check" table.*

***G1 approved by the owner, 2026-11-02**, on v2.1, with whole-tree digests checked at T2 and above (§3).*

*Builds on [plan check v2](s1-1a-plan-check.md). Where this note and the plan check's §2/§5 differ, **this note wins**, and every such difference is named. S1.1a-2's parts (CI, the hooks) are designed at its own G0. **G1** = this note's focused re-check passes (owner decision) and the owner approves. Normative words: MUST, MUST NOT, SHOULD.*

**Names** (owner, 2026-11-02):
- **Module and repository:** module `github.com/vorsar/cep`, with the facade package `cep` at its root; repository `VORSAR/cep`.
- **Tool cache:** `~/.cache/vorsar/cep/tools`, overridable by `VORSAR_CEP_TOOLS_CACHE`. `XDG_CACHE_HOME` is not consulted, so the path is the same everywhere.
- **User settings:** the prefix `VORSAR_CEP_` (e.g. `VORSAR_CEP_WORKERS`).
- **Runner-to-row contract:** the names `GATE_TIER`, `GATE_ROW`, `GATE_RUN_DIR`, `GATE_ROW_DIR` and the line `GATE_COVERED` keep their `GATE_` names. They are not user settings.

---

## 1. The event kernel (P1 decided: (b+), owner 2026-11-02)

`internal/event/event.go`, package `event`. Its imports are exactly `cmp`, `errors` and `fmt`, tested by a Go test that reads the package's own imports (QA counter-example 39).

```go
type Type uint8

const (
	TInvalid Type = iota // 0: an unset slot. Never a declared attribute type, never a null (nulls keep their declared type).
	TString              // 1
	TInt                 // 2
	TFloat               // 3
	TBool                // 4
	TTimestamp           // 5
	TDuration            // 6
	TObject              // 7
)

// String: "invalid", "string", "int", "float", "bool", "timestamp", "duration", "object"; otherwise "Type(<n>)".
func (t Type) String() string

type Attr struct {
	Name     string
	Type     Type
	Optional bool
}

const ArrivalTime = -1 // TimeAttr value for `time arrival`

// Schema's fields are private: the only way to build a usable Schema is NewSchema.
// The zero Schema{} has no attributes, so TimeAttr()'s 0 can never silently name a real attribute.
type Schema struct {
	stream   string
	attrs    []Attr
	timeAttr int
}

func NewSchema(stream string, attrs []Attr, timeAttr int) (Schema, error)
func (s Schema) Stream() string
func (s Schema) NumAttrs() int
func (s Schema) Attr(i int) Attr // a copy; an out-of-range i panics like any slice index
func (s Schema) TimeAttr() int   // an attribute index, or ArrivalTime

type Value struct {
	Kind Type
	Str  string
	I    int64
	F    float64
	B    bool
	T    int64
	D    int64
	Obj  any
	Null bool
}

type Event struct {
	StreamID   int
	Ts         int64
	Seq        uint64
	Values     []Value
	Invisibles bool
	BatchID    string
}

// Compare: cmp.Compare on Ts, then StreamID, then Seq, returning at the first non-zero result.
// Exactly -1, 0 or +1. Payload fields never affect it. Both arguments MUST be non-nil.
func Compare(a, b *Event) int
```

**`NewSchema` rules.** Each rule comes from SPEC/SEMANTICS or the owner, and every error message names the rule it breaks. It **rejects**:
1. an empty stream name;
2. zero attributes (the grammar's `attr_list` requires at least one, [grammar.lark:29](../language/grammar.lark));
3. an empty attribute name;
4. a duplicate attribute name, compared case-sensitively (SPEC §2.1: "Identifiers are case-sensitive", so `a` and `A` are distinct);
5. an attribute `Type` of `TInvalid` or above `TObject`;
6. `timeAttr` not `ArrivalTime` and not a valid index;
7. `timeAttr` naming a non-`TTimestamp` attribute (owner: this is enforced here internally, and becomes a coded deploy error in S1.2's spec rev 3, item (j)).

It **accepts** an `Optional` time attribute: a null timestamp is an event-level error at ingest (SEMANTICS §1.1, §10), not a schema error. It **copies** `attrs`, so the caller mutating its slice afterwards changes nothing.

**Residue and forwards:**
- `Value{}` is detectably invalid (`Kind == TInvalid`), but nothing reads `Kind` this sprint. The consumers that fill or read slots (S3.1's decoder, S3.2a's playback loader, S2.2a's expression nodes) MUST treat `TInvalid` as an error. Their sprints carry that as a test (inferred placement; forwarded).
- `Type` numbers MUST never be encoded: S1.2's canonical encoding writes types by name (architect DA24; forwarded to S1.2).

**ARCHITECTURE §4** is annotated at G1 with this surface.

## 2. Rows by wave: what each "done" runs (architect DA1, QA DQ10)

A lane is done when its done-run exits 0. The rows exist when their subjects exist:

| When | Done-run | Rows | Who adds them |
|---|---|---|---|
| W1 (Lanes A, B) | `make check` → **the scaffold** (§7) | the scaffold's own list: parity-lite, gofmt, vet, build, Python tests, ruff | coordinator (first commit) |
| Combine-W1 | `make check` → **the real runner**; the scaffold is deleted | `parity`, `gofmt`, `vet`, `ruff`, `pyfloor`, `pytests` | coordinator, in `registry.json` at combine-W1 |
| W2 (Lanes C, D) | `make check` at the W1 tip, plus each lane's own rows run locally via a fixture registry its brief names | Lane D: the tip's rows + `unit` (QA's tables and D's code arrive together on D's branch). Lane C: the tip's rows + its instrument's tests via `pytests` | each brief names its extra rows; the coordinator adds them to `registry.json` at combine-W2 |
| Combine-W2 and after | `make check`, `combine-check`, `strict-gate` | the full registry: + `unit`, `coverage-inclusion`, `proofs` | coordinator |

QA's red tables are written **on Lane D's branch**. The integration tip never carries a red test without its fix.

## 3. The pinned tools and parity

**`tools.json`** (coordinator-owned law data), schema 1. Tools, in this order:

| Tool | Version | darwin-arm64 archive sha256 | linux-amd64 archive sha256 |
|---|---|---|---|
| `python` | CPython 3.15.2 (python-build-standalone `20261026`) | `285a6b924713bec9c1cdf442ba5ebfbafbab070060b34d607c30546b758e64a6` | `08d4ae9024a7cfbd3b70e559241afa2571cd3208d7bd5f96f88e0080dd4092aa` |
| `python-floor` | CPython 3.12.16 (same build) | `fbf4afef53c2a6b72bb741bc06d5671a9dfaf87bd84e5aa73ac9b3d0d589f14e` | `6a79e922e50006b601f29e688407659da1c2b86ebcd4e9c7254cf234be8b89de` |
| `go` | go1.27.3 | `dd3908cf20959d798c1e06ebe5b72213f0505ce01373b40ce3110975a6cd99c2` | `65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40` |
| `golangci-lint` | v2.15.0 | `d1e409481a8ca846c4bc97e781eb5cb1bd5c23353a0b3977a1cbda1b1b6447d5` | `9b7fe6a6bf6334b98decf8d86b11415ea7889afdabdd1959fdce23af2a9ebcea` |
| `ruff` | 0.17.3 | `118f7121aa838785672bf612487f755a89108bb5f44e0f3df96c5967a72614f9` | `2b0890f5a55900af741f133918c2656919bbf18c2ab7741b4ca8403a90e647a9` |

**Fields of each tool:**
- `name`, `version`, `version_argv` (relative to the tool directory), `version_line` (the exact first stdout line, with `{goos}`/`{goarch}` substituted);
- `command`: the name looked up on `PATH`, e.g. `go`, `golangci-lint`, `ruff`, `python3.15`, `python3.12`. It is versioned for the Pythons, because both `bin/` directories also hold `python3` and `pip` (re-check S3);
- `path_dir`: the directory, relative to the tool directory, that goes on `PATH`;
- per platform: `url`, `archive_sha256`, `strip` (a prefix removed from every member), and `exec_sha256`.

**`exec_sha256`** lists the executables the archive ships that the gate runs:
- **Go:** the 10 regular files under `bin/` and `pkg/tool/<os>_<arch>/`: `go`, `gofmt`, `asm`, `cgo`, `compile`, `cover`, `fix`, `link`, `preprofile`, `vet` (re-check claim 4).
- **Each Python:** `bin/python3.x`. On darwin it is static and loads no `libpython` (`otool -L`). Linux is checked by Lane B's census.
- **golangci-lint and ruff:** their single binaries.

The coordinator computes these digests from the verified archives.

**Whole-tree digests** (owner at G1, 2026-11-02, as recommended; re-check S2). Go builds some tools on demand from its pinned source into `GOCACHE` (`covdata`, `test2json`, `buildid`), and the pinned Pythons run their standard libraries. To cover both, every platform entry also pins a `tree_sha256`:
- **How it is built:** the sha256 of the lines `<kind> <path> <payload>\n` for every entry of the installed tree except `.complete`, sorted bytewise by path.
  - `kind` is `d` (directory), `f` (file), `x` (file with an execute bit) or `l` (symlink).
  - The payload is the content's sha256 for files, the link target for symlinks, and `-` for directories.
  - The coordinator computes it from the verified archive's extraction.
- **When parity checks it:** at **T2 and above**, parity recomputes it as a fifth check (`"<tool>: tree differs: run make clean-tools tools"`). At **T0/T1** only the executables are re-hashed, which keeps hooks fast.
- **Cost:** about 1 s on a T2 run (inferred); Lane B measures it and reports.

The executables that Go builds into `GOCACHE` come from that verified source and the verified compiler.

**Load validation** (exit 2): every field is present and typed, the current platform's entry exists, and `exec_sha256` is non-empty (QA DQ23).

**The member census** (architect DA17, QA DQ9) is Lane B's **first commit**, `gate/toolchain/census.md`: for all 10 archives, the count of each member type and every link with its target. The extraction rules below are checked against it before any other code.

**Bootstrap floor** (owner, 2026-11-02): the installer runs on any `python3` ≥ **3.12**. On an older interpreter it prints the fix `install Python 3.12+ (e.g. brew install python)`, then `GATE FAILED` and `exit: 2`. To keep that promise on Apple's 3.9.6, which otherwise dies with a bare `SyntaxError` (re-check S7), the bootstrap is **two files**:
- `gate/bootstrap.py`: a launcher of about 20 lines, held to **3.9 syntax** by ruff's per-file target. It checks the version and refuses with the fix text, then imports the installer.
- `gate/toolchain/install.py`: the installer, held to the **3.12** target. The `pyfloor` row (§5) runs the bootstrap's own tests under the pinned **3.12.16**, which verifies its syntax and its library use. That replaces the `ast` check, which probes showed accepts newer f-strings and star-in-index under `feature_version` (QA DQ7).

**`bootstrap.py install`.** For each tool, for the platform:
1. **Skip** if `<cache>/<name>-<version>-<archive_sha256[:12]>/.complete` holds the archive digest. If the directory exists but `.complete` is missing or wrong → exit 2, naming the directory and `make clean-tools`. It is never deleted automatically (QA counter-example 16).
2. **Download** to `<cache>/dl.<pid>.<rand>` (`urllib.request`). On an error → exit 2, naming the URL.
3. **Hash.** On a mismatch → delete the download; exit 2, naming both digests.
4. **Extract** into `<cache>/tmp.<pid>.<rand>/`, a **sibling** of the final directory (architect DA2).
   - **Accept:**
     - regular files and directories;
     - the member that **equals the strip prefix**, the tree root itself (go's `go/`, ruff's top directory; re-check claim 2);
     - parent directories created on demand: both Python archives and golangci-lint's have no directory members;
     - symlinks whose target, resolved **lexically by name** within the tree, stays inside it. Never test whether the target exists: several Python symlinks come before their targets;
     - hard links to an earlier member inside the tree.
   - **Reject** (exit 2, naming the member; nothing is written outside the temporary directory): an absolute path; a `..` component; a member outside `strip`; a link whose target escapes; a device or FIFO; any member whose path passes through an already-extracted symlink.
5. **Finalise:**
   - write `.complete` = the digest;
   - clear **all** write bits (`& ~0o222`) on every file and directory. Symlinks are skipped, never chmod-ed (re-check L1/L2; the Python archives ship group-writable modes);
   - `os.rename(tmp, final)`, same parent: re-checked to work for a read-only directory.
   - If the rename raises **any** `OSError` while `final` exists (macOS gives `EACCES` for a read-only loser), another install won the race. Verify `final`'s `.complete`, restore owner write permission on `tmp`, delete it, and continue. Any other `OSError` is re-raised (re-check claim 1, S1).
6. **Clean up:** the downloaded archive is deleted. `make clean-tools` restores owner write permission, then removes the cache.

**`bootstrap.py exec -- <args>`.** It checks the floor, then hashes the pinned 3.15.2's executables against `tools.json`. On a mismatch or a missing interpreter → `run make tools`, then `GATE FAILED`, then `exit: 2` (QA DQ20). Otherwise it calls `os.execv(P, [P, "-I", "-B", "-X", "utf8", "<repo>/gate/run.py", *args])`, with P the **absolute** path of the pinned `python3.15`, also passed as `argv[0]` so that the `run.py` guard sees the real executable on Linux too (re-check S11).

`gate/run.py` stays parseable by 3.12 and starts with a guard: unless `sys.executable` is the pinned 3.15.2 → exit 2 with `GATE FAILED`. Only then does it import the runner.

**Parity** (`gate/toolchain/parity.py`; the runner's built-in first step in every job). For each tool, in `tools.json` order, the checks below short-circuit in this order, and the tool gets **one** reason, the first that fails:
1. **not installed:** the tool's cache directory is missing → "`<tool>`: not installed: run `make tools`";
2. **path:** `shutil.which(command)` with the runner's `PATH`, after `realpath`, is not inside the tool's cache directory, also after `realpath` (on macOS `/tmp` resolves to `/private/tmp`; re-check S3) → "`<tool>`: path: `<p>` is outside the cache";
3. **version:** the first stdout line of `version_argv`, run **in the cleaned row environment of §4** (an inherited `GOTOOLCHAIN=go1.27.2` otherwise makes the cached `go` fetch another toolchain; re-check S4), ≠ `version_line` → "`<tool>`: version: want `…`, got `…`";
4. **sha256:** an `exec_sha256` entry differs → "`<tool>`: sha256 of `<file>` differs: run `make clean-tools tools`".

Every tool is checked. The parity record's reason lists all the failing tools in `tools.json` order, separated by "; " (QA counter-example 7). A binary outside the cache is never executed: the path check comes before the version check.

**`make check|combine-check|strict-gate|ci` each depend on `make tools`** (owner, 2026-11-02; architect DA15). A clean clone with an empty cache therefore installs, then runs. The dependency is **wired at combine-W1**, when Lane B's installer lands. In W1, `make check` runs the scaffold against the coordinator's hand-installed tools (re-check S8).

## 4. The registry and the runner

**A row** has plan check S1's fields. These differ from the plan check:
- **`{files}`** is replaced by the path of a NUL-separated list file, `$GATE_ROW_DIR/files.nul`, so long lists never hit the argument-length limit (architect DA21). Its sha256 goes into the record as `scope_sha256`.
- **`expected_scope` kinds:**
  - `git`: the registry stores **complete pathspecs, magic included**, and the runner adds nothing. Use `:(glob)**/*.go` and `:(glob)**/*.py`: under glob, `*` does not cross `/`, so `:(glob)*.go` lists only top-level files. Exclusions are separate pathspecs, e.g. `:(glob,exclude)gate/proofs/**`: putting `:(glob)` in front of `:(exclude)` silently drops the exclusion (re-check B1, verified by the coordinator). A registry test expands every row's scope in a fixture repository holding top-level, nested and excluded files (QA DQ8);
  - `go`: real packages only. A `go list -e -json` entry whose `ImportPath` is a relative pattern, or whose error is "no Go files" or "matched no packages", is dropped; a package with a build error is kept (architect DA3, QA DQ3);
  - `go-vet-units`: for each real package, 1 if it has any `GoFiles`, `CgoFiles` or `TestGoFiles`, plus 1 if it has `XTestGoFiles`. Probes on go1.27.3: no tests → 1; in-package tests → 1; external tests → 2; both → 2; a directory with **only** external tests → 1 (re-check claim 5).
- **`min_wants`:** for a row that is not an analyzer, the minimum covered count. `covered < min_wants` fails the row. `proofs` uses it (QA DQ4). This interprets ADR-0015 §1.1's field for non-analyzer rows; ADR-0015 is annotated at G1.

**Load validation** (exit 2, before anything runs; a table test per rule, QA DQ13):
- strict JSON: duplicate keys are refused via `object_pairs_hook`, and integers must satisfy `type(v) is int`, so `true` is not an int;
- every field is present and typed;
- `id` and `job` match `^[a-z][a-z0-9-]*$`; `id`s are unique;
- `tiers` is a non-empty subset of {T0…T4}; `timeout_s` is between 1 and 3600;
- `scope` ⊆ the classes of ADR-0015 §2.1;
- `needs` names earlier rows in the same job (the registry order is topological), and each needed row's tiers include this row's tiers (architect DA6);
- `command` is a non-empty list of strings.

**The event loop** (architect DA5/DA6, QA DQ12/DQ27). One scheduler thread waits on one `queue.SimpleQueue` for every kind of event: row ends from worker threads, timer firings, and signals. The signal handler only calls `put_nowait`.

**Scheduling:**
- `ready` = rows at the tier whose `needs` have completed, in registry order. They start while workers are free.
- When an **exclusive** row is ready, no other row starts. It starts once nothing is running, and runs alone.
- If nothing is running, nothing is ready and rows are pending, each pending row fails with "not run: dependency deadlock". Validation makes this unreachable, but it is tested.
- **The run ceiling** = the sum of the selected rows' caps + 60 s. On breach: the interrupt path, with the reason "run ceiling `<n>`s exceeded".
- The trace is written by the scheduler only.

**One step:**
1. Expand the scope and write `files.nul`. The environment:
   - **inherited**, minus every `GO*` except an allow-list (`GOCACHE`, `GOMODCACHE`, `GOPATH`, `GOPROXY`, `GOSUMDB`, `GOPRIVATE`, `GONOPROXY`, `GONOSUMDB`), minus `PYTHON*` and `GIT_*_PATHSPECS`;
   - **then set:** `PATH` (cache bins first), `GOTOOLCHAIN=local`, `GOENV=off`, `GOFLAGS=`, `GOWORK=off`, `CGO_ENABLED=0` (owner, residue item 9), and the `GATE_*` row variables;
   - `GIT_INDEX_FILE`, `GIT_DIR` and `GIT_WORK_TREE` are kept;
   - `go env -json` goes into the run directory once per run (architect DA14).
2. Spawn with `start_new_session=True`. stdin is `/dev/null`; stdout and stderr go to `GATE_ROW_DIR/stdout` and `stderr` (supersedes plan check S3b's single `log`, architect DA19).
3. Arm the watchdog: TERM to the group; after `grace_s` = 5, KILL.
4. When the leader exits, probe the group with `os.killpg(pgid, 0)`. If it is still populated, record "left running processes", then KILL the group. The leader is reaped before the probe. Residue: on Linux a zombie may still count in the group (QA DQ34; re-probed on linux-amd64 in S1.1a-2), and the pid-reuse window between reap and probe is accepted (architect DA22).

**SIGCHLD** is reset to `SIG_DFL` at start-up. A test sets it to `SIG_IGN` **in-process**, runs a row that exits 1, and expects FAIL "exit 1" (QA DQ5). `signal.SIG_IGN` is banned in ruff, except in that test file.

**Verdict precedence** (the first match gives the reason; a test per slot; architect DA7, QA DQ11):
1. not run: parity failed
2. interrupted
3. timeout after `<cap>`s
4. could not start: `<error>`
5. scope error: `<error>`
6. left running processes
7. exit `<n>`, or "killed by signal `<n>`"
8. `GATE_COVERED` missing / duplicated / malformed / 0
9. covered `<n>` ≠ expected `<m>`
10. covered `<n>` < min_wants `<k>`
11. pass

At T0 (S1.1a-2), "skip: nothing staged in scope" sits between rules 1 and 2.

**`GATE_COVERED` grammar** (QA DQ14). Stdout bytes are split on `b"\n"` only. A line starting `GATE_COVERED` must match `rb"^GATE_COVERED ([1-9][0-9]{0,8})$"` (ASCII digits, no `\r`); otherwise it is malformed.

**`VORSAR_CEP_WORKERS`:** unset → `os.process_cpu_count()`, at least 1. Set → must match `^[1-9][0-9]{0,2}$` and be ≤ 512; otherwise (including empty) → exit 2.

**Clock** (architect DA12, QA DQ15). Two injected functions:
- `wall_ms()`: UTC epoch ms, used for `run_id`, `started_ms` and `ended_ms`;
- `mono_ns()`: used for durations, watchdogs, `duration_ms` and the footer.

Tests use a **frozen** clock (everything 0) for the completion-order test, and a **scripted** per-row clock (row → duration) for the time column and the footer.

**The summary** (stdout + `summary.md`; byte-exact under an injected clock; QA DQ16, architect DA20):
- **Columns:** `job | step | verdict | time | reason`. Each column is as wide as its longest value or header.
- **Order:** rows in registry order. Parity comes first, labelled job `*` in an all-jobs local run, with one record per job, `records/<job>/parity.json` (architect DA16).
- **`time`:** a row's ms as tenths of a second, truncated.
- **Footer:** `wall <W> · summed <S> · saved <P>%`. W is the span from the run's first start to its last end on `mono_ns`. S is the sum of the rows' raw ms. Both are shown as truncated tenths and computed from raw ms. P = `(S_ms − W_ms) × 100 // S_ms` with Python's floor division, so a negative value is shown as is (1000 vs 1155 → `-16%`); P is `0` when S is 0.
- Then `GATE PASSED|GATE FAILED` and `exit: <n>`.

**Interrupts.** The first SIGINT/SIGTERM stops new starts, sends TERM to every running group, starts the grace timer, then KILLs. A second signal KILLs at once (QA counter-example 20). Unfinished rows record "interrupted". The summary is written, and the exit code is 1.

**The run directory:** `.gate/runs/<run_id>/`, with `run_id` = `YYYYMMDDTHHMMSSZ-<6 hex>` (tests pass it). Every file is written to a temporary name, then renamed.

## 5. The S1.1a-1 rows

| id | job | expected_scope | command (`gate/runner/rows/…`) | `GATE_COVERED` = |
|---|---|---|---|---|
| `gofmt` | lint | `git` `:(glob)**/*.go` | `gofmt -l` over `files.nul` in batches of at most 256. A listed file → exit 1 naming it, with the fix `gofmt -w <file>`. gofmt exit 2 → exit 1 naming the file | files in the batches gofmt completed |
| `vet` | lint | `go-vet-units` `./...` | `go vet -json ./...`. **stdout** is parsed as a stream of possibly multi-line JSON objects (`raw_decode`). go1.27.3 exits 0 even with diagnostics (probe), so **any non-empty object → exit 1**, naming `posn`. **vet's own non-zero exit → exit 1** too: a compile error prints no object and exits 1 (re-check claim 5, S5) | objects parsed |
| `unit` | test | `go` `./...` | `go test -json -count=1 -coverprofile=$GATE_ROW_DIR/cover.prof ./...` → `test.json` | distinct packages with a package-level `pass`/`fail`/`skip`. No `run` event in the whole run → exit 1, "no test ran" |
| `coverage-inclusion` | test (needs `unit`) | `go` `./internal/... . ./rule/... ./io/...` | `gate/coverage/main.py` (plan check S6) on `unit`'s artifacts. A missing artifact → exit 1, naming `unit` | packages evaluated |
| `pytests` | test | `git` `:(glob)gate/**/test_*.py` and `:(glob,exclude)gate/proofs/**` | each module in **its own subprocess** under the pinned 3.15.2 (`-I -B -X dev -W error`), through a shim (`gate/runner/pyshim.py`) that: fails any skip, `expectedFailure` or unexpected success; installs a failing `sys.unraisablehook` (leaks fail); fails a module that runs 0 tests or fails to import. **Exit 1, naming the file**, for any file under `gate/` that is: named `*_test.py`; or under `tests/` and not named `test_*.py`/`__init__.py`/`support.py`; or named `test_*.py` but not tracked by git (QA DQ8, DQ17; re-check S10) | modules that passed |
| `ruff` | lint | `git` `:(glob)**/*.py` | over `files.nul`, in batches: `ruff format --check --no-cache --config ruff.toml <files>`, then `ruff check --ignore-noqa --no-cache --config ruff.toml <files>` (owner: no inline exceptions). The explicit list stops ruff from finding untracked files (re-check S6) | the sum over batches of the summary line, matched against `^(?:(\d+) files? would be reformatted, )?(\d+) files? already formatted$` or `^(\d+) files? would be reformatted$` (ruff 0.17.3's exact forms, re-checked). `ruff check`'s own count is residue (QA DQ22) |
| `pyfloor` | lint | `git` the test modules of `gate/bootstrap.py`, `gate/toolchain/install.py` and `gate/run.py`'s guard | those tests, run under the pinned **3.12.16** with the `pytests` shim. The expected scope is the test modules only, so it can equal covered (re-check B2) | modules that passed |
| `proofs` (T4) | proofs | `git` `gate/proofs/drivers/test_*.py`; `min_wants` = the driver count in §8 | each driver in its own subprocess, through the same shim | drivers that passed |

**Teeth for the real wrappers** (QA DQ22): `gate/runner/tests/` runs each **real** wrapper with a narrowed argument (vet over 2 of 3 units; gofmt dropping its last batch of 257 files) → "covered n−1 ≠ expected n".

**`ruff.toml`** (coordinator-owned; architect DA9, QA DQ6/DQ31/DQ36):
- `target-version = "py315"`; line length 120.
- `select`: `E`, `W`, `F`, `I`, `B`, `UP`, `SIM`, `TID`, `BLE`, `S602`, `S604`, `S605`. S603, S607 and `PTH` are not selected: they flag the mandated argv calls and `os.rename`.
- `banned-api`: `time.sleep`; `os.system`; `os.wait`, `os.wait3` and `os.wait4`; `shutil.move`; `signal.SIG_IGN`.
- `per-file-ignores`, the only way to waive a rule:
  - `gate/wait.py`: TID251 (sleep);
  - the SIGCHLD test: TID251 (`SIG_IGN`);
  - the entry files: BLE001 (`main`'s catch-all).
- A top-level `[per-file-target-version]` table. ruff 0.17.3 supports it and `py315`, re-checked: `invalid-syntax … on Python 3.12` and `… on Python 3.9`.
  - `"gate/bootstrap.py" = "py39"`: the launcher, so Apple's 3.9.6 reaches the fix text (re-check S7);
  - `"gate/toolchain/install.py"`, `"gate/run.py"` and `"gate/runner/pyshim.py"` = `"py312"`: the shim also runs under 3.12.16 in `pyfloor` (re-check S10).

**The wait helper:** `gate/wait.py`, owned by Lane A, with its test. W1's Lane B MUST NOT need to wait (its tests are synchronous). `Makefile` has one owner: Lane A.

## 6. The run's exit, end to end (exit line 1)

From a fresh clone with an empty `VORSAR_CEP_TOOLS_CACHE`:
1. `make check` runs `make tools` (the bootstrap installs all five tools);
2. `exec` runs the runner at T2 (parity, `gofmt`, `vet`, `ruff`, `pyfloor`, `unit`, `coverage-inclusion`, `pytests`);
3. the run ends with `exit: 0`.

The coordinator records this run, with its output, in the sprint record (QA DQ21).

## 7. The throwaway scaffold (owner: before wave 1; its own check list; deleted at combine-W1)

`gate/scaffold.py` is written by the coordinator in the first commit and deleted in the combine-W1 commit. Before W1, the coordinator installs the five pinned tools by hand into §3's cache layout, verifying each archive's sha256.

**It runs** under the pinned 3.15.2 (`-I -B`), with SIGCHLD at its default and plan check S3a's environment as amended in §4. Each check runs in its own process group, with output to files, a timeout and a group KILL. Any exception → `exit: 2` (architect DA18, QA DQ35). **Its list, in order:**
1. parity-lite: each tool resolves inside the cache after `realpath`, and its version line matches;
2. `gofmt -l` over tracked `*.go`;
3. `go vet -json ./...`, with any non-empty object **or a non-zero vet exit** failing (re-check S5: otherwise a compile error in a `_test.go` file passes);
4. `go build ./...`;
5. each tracked `gate/**/test_*.py` module in its own subprocess, ≥ 1 test and no skip;
6. `ruff format --check`, then `ruff check --ignore-noqa`.

It prints a summary, then `GATE PASSED|FAILED` and `exit: <n>`. **It is not the gate:** no parallelism, no records, no exec-hash parity. Combine-W1's real run catches anything it misses as a loud red.

## 8. Proofs (`gate/proofs/`; architect DA4/DA11, QA DQ1/DQ4/DQ24)

`gate/proofs/` is a separate Go module, `github.com/vorsar/cep/gate/proofs` (`go 1.27.0`), holding the planted packages, plus `drivers/` (Python).

**Every driver:**
- works on **real copies**, never hard links, inside a temporary directory;
- runs the real runner at **T2** with a fixture registry, or the real instrument with the row's exact flags; never T4, so a driver cannot recurse into `proofs`;
- asserts the red verdict, its exact reason, `GATE FAILED` and `exit: 1`;
- outside the parity drivers, also asserts a **parity PASS** control in the same run.

| Driver | Planted defect (only this check can fail) | Asserted |
|---|---|---|
| `test_parity_version` | a fixture `tools.json` whose `go` `version_line` says go1.27.2. Cache, path and hashes are real | the one reason "go: version: want … got …"; the trace shows only parity; every other row "not run: parity failed" |
| `test_parity_missing` | a cache holding `python`, `python-floor`, `ruff` and `golangci-lint`, but no `go` directory | "go: not installed: run `make tools`"; exit 1 from the runner, not the bootstrap's 2 |
| `test_parity_path` | a copied cache whose `go` directory lacks `bin/go`, plus a marker-writing `go` on the caller's PATH | "go: path: … outside the cache"; the marker is absent |
| `test_parity_hash` | a copied cache with one byte flipped in `pkg/tool/<os>_<arch>/compile` | "go: sha256 of `pkg/tool/…/compile` differs"; afterwards, the **real** cache re-hashes unchanged |
| `test_coverage_below` | `internal/below`: 9 statements, 8 covered | "…/internal/below 88.88% (8/9) < 90%" |
| `test_coverage_notests` | `internal/notests`: no `_test.go` | "…/internal/notests no tests" |
| `test_pytests_vacuity` | five runs: an import error; all skipped; no tests; an untracked `test_x.py`; a tracked `runner_test.py`; plus a clean control that passes | five FAILs, each naming its file; the control passes |
| `test_ruff_sleep` | `time.sleep(0)  # noqa: TID251` in a temporary tracked file | ruff FAIL with reason "exit 1"; the row's **stdout** contains `TID251` (inline `noqa` is ignored) |
| `test_ruff_format` | a badly formatted `.py` file | ruff FAIL with reason "exit 1"; the row's stdout contains `File would be reformatted`, ruff 0.17.3's exact text (re-check B3) |
| `test_pyfloor` | in a fixture registry holding only `parity` and `pyfloor` (ruff would also flag the plant; re-check L3): a temporary `install.py` with `except ValueError, TypeError:` (3.14 syntax), and separately a call to `copy.replace` (a 3.13 API) on a tested path | `pyfloor` FAIL for each, under 3.12.16 |

The `proofs` row's `min_wants` is **10**. Deleting a driver gives covered 9 < 10 → red.

## 9. Tests and verification

- **Runner tests.** The injected clock (§4). FIFO fixtures: X opens for write then read, Y for read then write.
- **The "nothing outlives the runner" witness** (QA DQ19): each fixture process opens a named FIFO by path, writes one start byte and holds it. The test first reads one start byte per process, then waits for end-of-file.
- **One real-clock smoke test** caps only the row that is expected to time out (QA DQ28).
- **The stress harness** (`gate/runner/stress.py`, Lane A; QA DQ18): 50 runs, each child **without** `-I`/`-E`, in a scrubbed environment carrying `PYTHONHASHSEED`=0…49, with `-s -B -P -X dev -W error`, a seeded shuffle of test order across modules, and CPU hogs at 2× the cores. It prints per-test pass counts. Its own tests plant an order-dependent and a hash-dependent test, which MUST score below 50/50.
- **First-seen-red:** every acceptance test's report names its planted defect and quotes the red run.

## 10. What changes in the binding documents at G1

1. **ARCHITECTURE §4:** an annotation with §1's surface (owner (b+)).
2. **ADR-0015 §1.1:** annotations for `min_wants` on non-analyzer rows, the `go`/`go-vet-units` kinds, `{files}` as a list file, and parity's per-job records.
3. **Plan check S3b, S5 and P5:** superseded by this note where named; the plan check itself is not edited.

## 11. Dispositions (none rejected)

**Architect.**

| # | Disposition |
|---|---|
| DA1 | fixed §2 (rows by wave) |
| DA2 | fixed §3 step 4 (sibling directory) |
| DA3 | fixed §4 (`go`, `go-vet-units`) and §5 (stdout, exit 0 with diagnostics) |
| DA4 | fixed §8 |
| DA5 | fixed §4 (one event queue) |
| DA6 | fixed §4 (topological, tier subset, deadlock, ceiling) |
| DA7 | fixed §4 (precedence) |
| DA8 | owner chose (b+): §1; the timestamp rule → S1.2 (j) |
| DA9 | fixed §5 (`ruff.toml`) |
| DA10 | fixed §3 (floor 3.12 + pinned 3.12.16; the owner's choice) |
| DA11 | fixed §5 (`min_wants`, proofs excluded from `pytests`) |
| DA12 | fixed §4 (two clocks) |
| DA13 | fixed §3 (every file in `bin/` and `pkg/tool`; residue item 9) |
| DA14 | fixed §4 (env allow-list, `go env` recorded) |
| DA15 | fixed §3 (targets depend on `tools`) |
| DA16 | fixed §4 (parity records per job) |
| DA17 | fixed §3 (census first; on-demand directories, links, hard links) |
| DA18 | fixed §7 |
| DA19 | fixed §4 step 2 |
| DA20 | fixed §4 (summary) |
| DA21 | fixed §4 (`files.nul`, `scope_sha256`) |
| DA22 | fixed §4 (residue stated); SIGHUP is treated like SIGTERM |
| DA23 | **forwarded** to S1.1a-2's G0 (command paths under a T0 snapshot; `--run-id` in CI) |
| DA24 | **forwarded** to S1.2 (types encoded by name) |

**QA.**

| # | Disposition |
|---|---|
| DQ1 | fixed §8 |
| DQ2 | fixed §5 (vet) |
| DQ3 | fixed §4 (`go` kind) |
| DQ4 | fixed §5, §8 (`min_wants` 10) |
| DQ5 | fixed §4 (in-process `SIG_IGN` test; `SIG_IGN` banned) |
| DQ6 | owner (`--ignore-noqa`); fixed §5 (rules), §8 (`test_ruff_sleep`) |
| DQ7 | owner (floor 3.12, pinned 3.12.16); fixed §8 (`test_pyfloor`) |
| DQ8 | fixed §4 (`:(glob)`), §5 (misnamed tests fail) |
| DQ9 | fixed §3 (census, rules); the per-rule `file://` fixtures are Lane B's tests (plan check §6 + counter-example 15) |
| DQ10 | fixed §2 |
| DQ11 | fixed §4 |
| DQ12 | fixed §4 (queue, ceiling, second signal) |
| DQ13 | fixed §4 (strict JSON, boundaries) |
| DQ14 | fixed §4 (grammars) |
| DQ15 | fixed §4 (frozen and scripted clocks) |
| DQ16 | fixed §4 (summary pinned) |
| DQ17 | fixed §5 (`pytests` shim) |
| DQ18 | fixed §9 |
| DQ19 | fixed §9 |
| DQ20 | fixed §3 (`run.py` guard, `GATE FAILED`) |
| DQ21 | fixed §6 |
| DQ22 | fixed §5 (real-wrapper teeth; ruff-check count residue) |
| DQ23 | fixed §3 (load validation) |
| DQ24 | fixed §8 |
| DQ25 | fixed §1 (spec-settled inputs; residue: none, since the fields are private) |
| DQ26 | moot: the owner chose (b+); the consumer duty is forwarded in §1 |
| DQ27 | fixed §4 |
| DQ28 | fixed §9 |
| DQ29 | fixed §5 (wait helper, one Makefile owner) |
| DQ30 | fixed: each named rule gets a §6 case in Lane briefs (gofmt red, unit red, the three bans, WARN, a missing artifact, the install skip path, `.complete` mismatch, event imports) |
| DQ31 | fixed §5 (per-file `py312`) |
| DQ32 | fixed §1 (the `Type` constants pinned 0…7) |
| DQ33 | fixed §4 step 2 |
| DQ34 | **forwarded** to S1.1a-2 (Linux re-probe) |
| DQ35 | fixed §7 |
| DQ36 | fixed §5 (`--no-cache`; PTH not selected) |

**QA's counter-examples 1–40** are adopted into the lane briefs, amended where this note changed the design: counter-example 36 now expects the 3.12 floor message, and 11's syntax cases are replaced by `test_pyfloor`'s.

**Re-check (v2 → v2.1).** The blockers B1–B3 and the S7 fix were re-verified by the coordinator by probe. None rejected.

| # | Disposition |
|---|---|
| claim 1 / S1 | fixed §3 step 5 (any `OSError` with `final` present = lost race) |
| claim 2 | fixed §3 step 4 (root member accepted; symlinks resolved lexically by name; parents created on demand for Python too) |
| claim 4 / S2 | fixed §3 (the exact Go list; static Python). The owner decided at G1: whole-tree digests checked at T2+ |
| claim 5 / S5 | fixed §4 (`go-vet-units` formula), §5 and §7 (vet's non-zero exit fails) |
| B1 | fixed §4 (complete pathspecs, magic included; separate exclusions; a fixture-repository test), §5 |
| B2 | fixed §5 (`pyfloor` scope = test modules) |
| B3 | fixed §8 (exact ruff text, read from stdout) |
| S3 | fixed §3 (`command` and `path_dir` fields; `realpath` on both sides) |
| S4 | fixed §3 (version run in the cleaned environment) |
| S6 | fixed §5 (explicit file list; exact summary forms) |
| S7 | fixed §3 (the 3.9-syntax launcher + the 3.12 installer), §5 (per-file targets) |
| S8 | fixed §3 (`tools` dependency wired at combine-W1) |
| S9 | fixed: the record's "How it runs" row is annotated |
| S10 | fixed §5 (untracked `test_*.py` fails; the shim gets `py312`) |
| S11 | fixed §3 (absolute `argv[0]`) |
| L1, L2 | fixed §3 step 5 |
| L3 | fixed §8 |
| L4, L5 | noted: the `go` filter relies on the relative-pattern rule for missing directories; the go archive's two non-ASCII names are harmless on APFS and to be re-checked on Linux by Lane B's census |
