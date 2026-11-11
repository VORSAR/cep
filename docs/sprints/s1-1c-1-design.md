# S1.1c-1 — design note (v2, 2026-11-11; G1)

**Sprint:** [S1.1c-1](sprint-plan.md#s11c-1--the-law-manifest-and-the-law-δ-block-the-current-sprint-marker-and-the-not-built-expiry-the-dependency-checks-the-error-code-counts) · **Plan check:** [s1-1c-1-plan-check.md](s1-1c-1-plan-check.md) (v2) · **Record:** [s1-1c-1-record.md](s1-1c-1-record.md) · **Branch:** `s1-1c/integration`. *Build-ready: every path, every signature, every text the gate prints, every test by name, the steps in order. v1 (fef1c03) was reviewed blind by the architect with the-fool ([review](../reviews/s1-1c-1-design-architecture.md)) and by QA for testability ([review](../reviews/s1-1c-1-design-qa.md)); §10 places every finding; the owner's seven answers of 2026-11-11 ([decisions](decisions.md#owner-decisions-2026-11-11--s11c-1s-design-after-the-reviews)) are applied. v2 is the design the lanes build; a lane that finds it wrong stops and reports.*

**Binding decisions, quoted:** [2026-11-10, S1.1c's scope](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check) — 3 (S1.1c-1's items), 4 ("semantic deltas per structured law file … one small parser per structured file, each with a planted-edit proof"), 8 ("a T2 test fails when `gate/current-sprint` is not the first sprint-plan row without ✓"), 9 ("`lawdelta` … FAILS when there is no merge base — never the silent pass of the `apidiff` row"; "the block is the summary renderer's own output carried into CI's aggregated summary"). [2026-11-11, the plan check](decisions.md#owner-decisions-2026-11-11--s11c-1s-plan-check) — 1 (one `deps` row), 2 (`conformance/notbuilt.json`), 3 (`HEAD is main — read the PR run`, PASS, merge base == HEAD), 4 (fingerprint for imported, field + liveness for graph-only), 6 (the expiry is a Python row; `gate/runner/sprintid.py`), 7 (`gate/sprints.json` pinned to the plan table), 8 (`kr/pretty`, `check.v1` approved). [2026-11-11, the design](decisions.md#owner-decisions-2026-11-11--s11c-1s-design-after-the-reviews) — 1 (CI warms the module cache by a generated setup step), 2 (a sixth block line for uncommitted law edits), 3 (`§<n>` for numbered headings, else the text; `(first of <k> hunks)`), 4 (`code`/`workflow` grouped last, one line per entry), 5 (the record check requires the schema number), 6 (a `✓` outside the id cell is an error), 7 (the `tool apidiff` directive moves to `gate/proofs/go.mod`).

## 1. What exists after G0 and W0 (read it, do not re-derive it)

`gate/current-sprint` = `S1.1c-1\n` · `gate/sprints.json` = `{"schema": 1, "note": "…", "rows": [{"id": "S1.1a-1", "closed": true}, … 26 rows …]}` · `gate/law.json` = `{"schema": 1, "note": "…", "paths": [{"path", "kind"[, "flag"]} × 23]}` (kinds: `registry`, `law`, `marker`, `sprints`, `ceilings`, `code`, `workflow`, `tools`, `deps`, `edges`, `exports`, `baseline`, `notbuilt`, `doc`) · `conformance/notbuilt.json` = `{"schema": 1, "note": "…", "entries": [{"file", "construct", "sprint"} × 9]}` · `tools/deps-allowlist.json` = `{"schema": 1, "note": "…", "modules": [… 23 entries, one module each …]}` · registry rows `lawdelta` (lint, T1–T4, `expected_scope ["git", "gate/law.json"]`, `min_wants 1`), `deps` (lint, T2–T4, `["git", "go.mod", "gate/proofs/go.mod", "tools/deps-allowlist.json"]`, `min_wants 3`), `notbuilt` (test, T4, `["git", "conformance/notbuilt.json"]`, `min_wants 1`), each running `python3.15 -I -B -X utf8 gate/runner/rows/<id>.py` — today a stub printing `failed: <id>: not built yet (lane X)`, exit 1 · the `proofs` row's `min_wants` is 24 · the W0 proofs on integration: `gate/proofs/drivers/test_lawdelta_planted.py` (7 cases), `test_notbuilt_expired.py` (7), `test_deps_planted.py` (6), `internal/parser/errorcounts_test.go` (green since the SPEC §9 line reads 68 / 45); `gate/runner/tests/test_sprintid.py` on branch `s1-1c-1/qa` (lane D's first commit brings it).

**Every G0 file carries a top-level `note` (a `str`); every loader names its exact top-level key set** *(review: architect S5)*: `law.json` `{"schema", "note", "paths"}` (S1.1c-2 adds `"instruments"` by editing that one line), `sprints.json` `{"schema", "note", "rows"}`, `notbuilt.json` `{"schema", "note", "entries"}`, `deps-allowlist.json` `{"schema", "note", "modules"}`; any other key, or a missing one, is the loader's error naming it.

The gate's conventions every lane keeps: a row script starts with `rows.start()` (its row directory, `GATE_ROW_DIR`), prints `GATE_COVERED <n>` through `rows.covered(n)` once its scope is read, and ends with `rows.finish(failures)` (`failed: <line>` per failure, exit 1; exit 0 on none) — `gate/runner/rows/__init__.py:25-95`; an uncaught exception prints `internal error:` and exits 2 (`gate/runner/step.py:40-50`). Everything runs under the pinned CPython 3.15.2 with `-I -B -X utf8`; stdlib only; no shell. Tests are `unittest` modules under `gate/<pkg>/tests/`, run by the `pytests` row; a test that makes git commits runs git with `GIT_CONFIG_GLOBAL=os.devnull`, `GIT_CONFIG_NOSYSTEM=1`, a fixed identity, `commit.gpgsign=false`, `maintenance.auto=false` (the shape of `gate/proofs/drivers/support.py:36-45`) and names its branch (`git checkout -q -b main`) before the first commit — never git's default name *(review: QA-S14, QA-5 of the plan check)*. The proof drivers under `gate/proofs/drivers/` run by the `proofs` row (T4) and by hand under the pinned interpreter the moment an instrument lands.

## 2. Lane A — the law manifest, the `lawdelta` row and the LAW Δ block

### 2.1 Files

| File | New / changed |
|---|---|
| `gate/runner/git.py` | new: `git`, `merge_base`, `head`, `ls_files`, `numstat`, `show`, `dirty` |
| `gate/runner/rows/apidiff.py` | changed: imports `merge_base` from `gate.runner.git`; behaviour unchanged (its silent pass stands by the owner's decision of 2026-11-07, `apidiff.py:7`) |
| `gate/runner/law.py` | new: the manifest reader and the pure parsers |
| `gate/runner/rows/lawdelta.py` | replaces the stub |
| `gate/runner/records.py` | changed: `SCHEMA = 2`; `make(..., law: Sequence[str] = ())` |
| `gate/runner/scheduler.py` | changed: `_Done.law`; `_finish` reads `law.txt`; the pass-with-no-block rule; `_close` computes the block |
| `gate/runner/summary.py` | changed: `render(..., law, ...)` required; prints it first |
| `gate/runner/summarize.py` | changed: `FIELDS["law"]`, the schema check, `_inconsistent`, the CI block |
| `gate/ci/generate.py` | changed: `NEEDS_HISTORY`, `NEEDS_MODULES`, the module-download setup step |
| `gate/runner/tests/test_git.py`, `test_law.py`, `test_lawdelta.py`, `test_law_pin.py` | new |
| `gate/runner/tests/test_summary.py`, `test_summarize.py`, `test_main.py`, `test_records.py`, `test_scheduler.py`; `gate/ci/tests/test_generate.py` | changed |
| `gate/proofs/drivers/test_lawdelta_planted.py` | from W0; made green; one case added (the sixth line) |

### 2.2 `gate/runner/git.py`

```python
def git(*args: str, cwd: str | None = None) -> tuple[int, str]   # subprocess.run(["git", *args], stdin=DEVNULL, capture_output=True, text=True); (returncode, stdout.strip())
def merge_base() -> str                      # "origin/main" then "main": the first `git merge-base HEAD <ref>` that succeeds, else "" (apidiff.py:28-33's behaviour, verbatim)
def head() -> str                            # `git rev-parse HEAD`
def ls_files(paths: Sequence[str]) -> list[str]        # `git ls-files -z -- <paths>` split on NUL; a path ending "/" is a prefix
def numstat(base: str, head: str, paths: Sequence[str]) -> list[tuple[str, int, int]]
    # `git diff --numstat -z --no-renames <base> <head> -- <paths>`: records are `added\tremoved\tpath\0`; a binary file's "-" counts as 0;
    # --no-renames makes a renamed file a removal of the old path and an addition of the new (review: architect S3, QA-B1);
    # the list keeps git's order (bytewise by path)
def show(rev: str, path: str) -> str | None  # `git show <rev>:<path>`, UTF-8 with errors="replace"; None when absent at rev
def dirty(paths: Sequence[str]) -> int       # the number of lines of `git status --porcelain -z -- <paths>` (uncommitted edits, staged or not, under the law paths)
```

### 2.3 `gate/runner/law.py` — the manifest and the parsers

```python
KINDS = frozenset({"registry", "law", "marker", "sprints", "ceilings", "code", "workflow", "tools", "deps", "edges", "exports", "baseline", "notbuilt", "doc"})
GROUPED = frozenset({"code", "workflow"})

@dataclass(frozen=True)
class Path:     path: str; kind: str; flag: str = ""
@dataclass(frozen=True)
class Manifest: paths: tuple[Path, ...]
class ManifestError(ValueError): ...

def load_manifest(text: str) -> Manifest
    # json.loads with object_pairs_hook refusing a duplicate key (ValueError → ManifestError "duplicate key");
    # top level exactly {"schema", "note", "paths"}: schema == 1, note a str, paths a non-empty list of objects whose keys are
    # exactly {"path", "kind"} or {"path", "kind", "flag"} (flag iff kind == "doc"); path a non-empty str, no leading "/", no "..",
    # unique; kind in KINDS; else ManifestError(f"gate/law.json: paths[{i}]: {fault}") or f"gate/law.json: {fault}" for the top level
def entry_for(m: Manifest, file: str) -> Path | None
    # the entry whose path == file, else the longest entry path ending "/" that prefixes file; None when none
def delta(kind: str, old: str | None, new: str | None) -> str
    # the one-line semantic delta for one changed file; old/new None = absent at that side; NEVER raises: every _delta_<kind>
    # wraps its parsing in try/except (ValueError, KeyError, TypeError, AttributeError) and returns "unparseable (old)" or
    # "unparseable (new)" for the side that failed; both sides None is impossible (numstat never reports it); kind "code"/"workflow" → ""
def doc_line(old: str | None, new: str | None, flag: str) -> str
    # the heading above the first changed line, then the flag, then the hunk count (decision 3):
    #   hunks = the opcodes of difflib.SequenceMatcher(None, old_lines, new_lines, autojunk=False).get_opcodes() that are not "equal"
    #   the first hunk's index is i2/j2 in new (or i1 in old for a pure deletion); the nearest preceding line matching ^#{1,6}\s+(.*?)\s*#*\s*$ in that text
    #   heading → "§{n}" when the text matches ^(\d+)\. , else the text; "" when no heading precedes; "(file added)"/"(file removed)" for a None side
    #   result = " — ".join(x for x in (heading, flag) if x) + (f" (first of {len(hunks)} hunks)" if len(hunks) > 1 else "")
def line(path: str, added: int, removed: int, one_liner: str) -> str
    # f"{path} +{added}/−{removed}" + (f" — {one_liner}" if one_liner else "")
def names(items: Sequence[str]) -> str
    # the list form used by every parser: up to 6 items joined by ", "; more than 6 → the first 3 joined by ", " then ", …"
def block(base_short: str, data_lines: Sequence[str], grouped: Mapping[str, tuple[int, int, int]]) -> list[str]
    # decision 4: data_lines (every kind not in GROUPED, in git's path order) first; then one line per grouped manifest entry,
    # f"{entry}: {files} files +{a}/−{b}", in manifest order; nothing at all → ["LAW Δ: none"];
    # else [f"LAW Δ ({n} files, vs {base_short})", *data_lines, *grouped_lines] where n counts every changed file
```
The per-kind one-liners (`_delta_<kind>(old, new) -> str`; several items joined with `; `; every name list through `names`):

| kind | reads | one-liner items (exact f-strings) |
|---|---|---|
| `edges` | `tools/importcheck/rules.json` as JSON: `imports`/`test_imports`/`external`/`test_external` as sets of `f"{pkg} → {imp}"` / `f"{pkg} →test {imp}"` / `f"{pkg} → ext:{mod}"`, **keys exactly as in the file** (`internal/event`, never shortened — review QA-S6); `fences`/`test_fences` as `(pkg, imp) → tuple` | `+{n} edge{s} ({names})`, `−{n} edge{s} ({names})`, `fence {pkg} ← {imp}: {old} → {new}` (lists comma-joined), `module {a} → {b}`, `note changed` |
| `registry` | both texts as JSON (`json.loads`, `object_pairs_hook` refusing duplicates — never `registry.load`); rows by `id` | `+1 row {id} ({tiers}, {job})`, `−1 row {id}`, `row {id}: {field} {old} → {new}` per changed field in the schema's order (lists comma-joined) |
| `exports` | lines `pkg ident kind signature…`; **a line that does not start with `<pkg> ` where `<pkg>` is the previous identity's package continues the previous identity** (`api/exports.golden:2` is `}`; `:13-16` tab-indented fields); identity = `f"{pkg}.{ident}"` | `+{n} export{s} ({names})`, `−{n} export{s} ({names})`, `{n} signature{s} changed ({names})` |
| `deps` | entries by `module` | `+1 module {module} {version} ({license})`, `−1 module {module}`, `{module}: {v1} → {v2}`, `{module}: license {a} → {b}`, `note changed` |
| `ceilings` | metrics by name; `rules` overrides | `{metric}: ceiling {a} → {b}`, `{metric}: blocking {a} → {b}`, `+metric {name}`, `−metric {name}`, `{metric}[{rule}]: {a} → {b}`, `{metric}: reason changed` |
| `baseline` | `gate.baseline.regen.parse` per side (ValueError → unparseable); blocks by `key` (a sha256) | `{n} block{s} re-recorded ({names})` with a name `f"rule {rule}, block {block}"`, `+{n} block{s} ({names})`, `−{n} block{s} ({names})`, `source changed`, `spec_rev {a} → {b}`, `format {a} → {b}`; a side None → `file added: {n} blocks` / `file removed` |
| `notbuilt` | entries by `file` | `+1 entry {file} ({sprint})`, `−1 entry {file}`, `{file}: {a} → {b}`, `{file}: construct changed` |
| `tools` | `tools` by `name`: `version`; any other difference under a tool | `{name}: {a} → {b}`, `{name}: hashes changed`, `+tool {name}`, `−tool {name}` |
| `marker` | the text stripped of whitespace | `marker {a} → {b}` |
| `sprints` | rows by `id` and position | `{id} closed`, `{id} reopened`, `+1 row {id} at {i}`, `−1 row {id}`, `order changed` |
| `law` | paths by `path` | `+1 path {path} ({kind})`, `−1 path {path}`, `{path}: kind {a} → {b}`, `{path}: flag changed` |
| `doc` | `doc_line` | e.g. `§9 — SPEC changed — owner decision required (first of 2 hunks)`; `Decision — an ADR changed — owner decision required` |
| `code`, `workflow` | — | `""` (grouped by `block`) |

For every non-doc kind a side that is None reads `file added` / `file removed` (plus the kind's items where they apply) *(review: architect, build-readiness 8)*.

### 2.4 `gate/runner/rows/lawdelta.py` — the row

```
start
  manifest = load_manifest(read "gate/law.json")            ManifestError → failed: lawdelta: <message>; exit 1
  for each entry: ls_files([entry.path]) == []             → failed: lawdelta: <path> is listed in gate/law.json but git tracks nothing there; exit 1
  covered(1)                                                (the manifest is the row's scope; printed only now)
  base = merge_base()                                       "" → failed: lawdelta: no merge base with main (fetch-depth 0 or a local main is needed); exit 1
  if base == head(): lines = ["LAW Δ: HEAD is main — read the PR run"]
  else:
    changes = numstat(base, head(), [e.path for e in manifest.paths])
    data, grouped = [], {}                                  grouped: entry.path → [files, added, removed]
    for (file, a, r) in changes:
      entry = entry_for(manifest, file)                     None → failed: lawdelta: <file> changed under a listed prefix but matches no entry (defensive; §2.6 tests it in-process)
      if entry.kind in GROUPED: grouped[entry.path] += (1, a, r); continue
      old, new = show(base, file), show(head, file)
      try: one = doc_line(old, new, entry.flag) if entry.kind == "doc" else delta(entry.kind, old, new)
      except Exception as e: failed: lawdelta: the <kind> parser crashed on <file>: <type>: <e>; exit 1    (defensive; tested in-process)
      data.append(line(file, a, r, one))
    lines = block(base[:7], data, grouped)
  n = dirty([e.path for e in manifest.paths]);  if n: lines.append(f"LAW Δ: {n} law files have uncommitted edits (not shown)")     (decision 2; local tiers; in CI always 0)
  write GATE_ROW_DIR/law.txt atomically (records.write_atomic; "\n".join(lines) + "\n"; UTF-8);  print each line;  finish([])
```
A delta is never a failure (ADR-0015 §1.3 as annotated at close-out: the row is `blocking: true`, red only when it cannot read). The block reads `base..HEAD`; the sixth line names what the commits do not show.

### 2.5 The record and both renderers

- `records.SCHEMA = 2`; `records.make(..., law: Sequence[str] = ())` adds `"law": [one_line(x, None) for x in law]` to every record.
- `scheduler._Done` gains `law: tuple[str, ...] = ()`; `_finish` (after the row's process ended, before the record is written) sets it from `<run_dir>/rows/<job>/<id>/law.txt` when `done.id == "lawdelta"`: the file's non-empty lines, `()` when absent. **Pass with no block is a failure** *(review: architect S1)*: when `done.id == "lawdelta" and done.ok and not law`, `_finish` records `ok=False`, `reason="lawdelta: exit 0 but law.txt is empty or missing"`.
- `scheduler._close`: `law = _law_block(self._tier, self._job, self._rows, self._finished)`:
  - no `lawdelta` row among the selected rows → `[f"LAW Δ: not run — the lawdelta row is not in this registry at {scope}"]`, `scope` = `tier` or `f"{tier}, job {job}"` (the string `registry.select` builds, `registry.py:240`; review: architect S2);
  - selected and `ok` → `list(done.law)` (non-empty by the rule above);
  - otherwise (fail, stopped, crashed, skipped) → `["LAW Δ: FAILED — see the lawdelta row"]`.
  Passed as `summary.render(lines, law=law, ...)`.
- `summary.render(lines, *, law: Sequence[str], wall_ms, summed_ms, exit_code, run_dir, registry=None, image=None)`: output `[*law, "", *table, …]`. `law` has no default: an omitting caller is a `TypeError`.
- `summarize`: `FIELDS["law"] = list`; `_valid` additionally requires `all(type(x) is str for x in record["law"])` and **`record["schema"] == records.SCHEMA`** — a mismatch reads `malformed: schema {x}, expected {SCHEMA}` where the generic text was (decision 5; review: QA L4, architect pre-mortem 3); `_inconsistent` gains `record["id"] == "lawdelta" and record["verdict"] == "pass" and not record["law"]` → `inconsistent: pass with no LAW Δ block`; after judging: `law = _law_block_ci(rows, lines, used)` — the `lawdelta` row's job read from the registry rows (`next(r.job for r in rows if r.id == "lawdelta")`, never a literal `lint` — review: architect L4): no such row → the "not run" text with `scope = tier`; its judged line not PASS (missing, malformed, duplicate, stale, inconsistent, FAIL) → `FAILED — see the lawdelta row`; else the used record's `law`. The row line for a missing record stays `lint | lawdelta | FAIL | missing: no job recorded it` (`summarize.py:169`).
- `generate.py`: `NEEDS_HISTORY = frozenset({"apidiff", "lawdelta"})`; **`NEEDS_MODULES = frozenset({"deps"})`** (decision 1): `_setup_steps(history, modules)` appends, after `make setup tools`, `      - run: go mod download all` and `      - run: go mod download all\n        working-directory: gate/proofs` when `modules`; `_job(...)` passes `modules = any(r.id in NEEDS_MODULES for r in chosen if r.job == job)`.

### 2.6 Lane A's tests (each red before its code; the first failure line named)

- `test_git.py` (a temp repo, isolated as §1 says): `merge_base` with/without `main`; `numstat` on two files (order), on a rename (`--no-renames`: one removal, one addition), on a binary; `show` absent → None; `dirty` 0/2. First red: `ModuleNotFoundError: gate.runner.git`.
- `test_law.py`: `load_manifest` valid and each fault (duplicate key, extra top-level key, missing `note`, flag on a non-doc, `..` in a path); `entry_for`; `names` (6 and 7 items); `doc_line` (a change under `## 9. Canonical form` → `§9 — SPEC changed — owner decision required`; under `## Decision` → `Decision — …`; two hunks → `(first of 2 hunks)`; no heading; added/removed file); one test per kind with **excerpt pairs** of the real files (not whole files) covering every item form of §2.3 — `exports` includes a changed tab-indented field under `cep Config type type Config struct {` → `1 signature changed (cep.Config)`; `baseline` uses two lines copied from `conformance/baseline/07-consent-gate.cep.jsonl` with one output changed → `1 block re-recorded (rule consent, block 1)` (the rule name as the file has it); `registry` with a duplicate key → `unparseable (new)`; `block` (none; data then grouped). First red per kind: `AssertionError: '' != '+1 edge …'`.
- `test_lawdelta.py` (the row in a temp repo with the real manifest copied and its paths tracked): each `failed:` text of §2.4 — the two defensive ones **in-process** with `unittest.mock.patch("gate.runner.law.delta", side_effect=RuntimeError("planted"))` and a patched `numstat` returning an unlisted path *(review: QA-S5)*; `law.txt`'s content; `GATE_COVERED 1` after liveness; the sixth line with one dirty law file. First red: the stub's `('fail', 'lawdelta: not built yet (lane A)')`.
- `test_records.py`: `make` carries `law` (first red: `TypeError: make() got an unexpected keyword argument 'law'`). `test_scheduler.py`: `test_lawdelta_pass_without_law_txt_is_a_failure`.
- `test_summary.py`: `render` without `law` → `TypeError` (first red: `AssertionError: TypeError not raised`); the block first, a blank line, the table.
- `test_summarize.py`: fixtures gain `law: []`; `test_a_record_without_law_is_malformed` (a `rec()` with `law` deleted, as `:175-178` does for `covered`); `test_a_schema_1_record_names_the_schema` (`malformed: schema 1, expected 2`); `test_a_passing_lawdelta_record_with_no_law_is_inconsistent`; the missing-record tree *(review: QA-S13)*: registry = the four fixture rows + `row("lawdelta", tiers T1–T4, job lint)`; `records-lint-<attempt>` holds `(lint, parity)`, `(lint, gofmt)`, `(lint, ruff)` and no `(lint, lawdelta)`; expect line 0 `LAW Δ: FAILED — see the lawdelta row`, line 1 empty, and the row line `lint | lawdelta | FAIL | missing: no job recorded it`; first red: line 0 is `job | step | …`. A registry without the row → line 0 `LAW Δ: not run — the lawdelta row is not in this registry at T4`.
- `test_main.py` (end to end, `gate/run.py`) *(review: QA-S2)*: the fixture is a temp **git** repo (`git init`, `checkout -q -b main`, the gate copied, one commit; isolation as §1) with a registry of one trivial T0–T2 row plus `lawdelta`; asserts the **exact** first line: T0 → `LAW Δ: not run — the lawdelta row is not in this registry at T0`; T1 and T2 on `main` → `LAW Δ: HEAD is main — read the PR run`; on a branch `change` with one non-law commit → `LAW Δ: none`; `--job lint` at T2 with the trivial row in job `test` → the "not run" text ending `at T2, job test`. First red: line 0 is the table header.
- `test_law_pin.py`: the §1.3 list (`gate/registry.json`, `gate/run.py`, `gate/runner/`, `gate/baseline/regen.py`, `gate/law.json`, `.github/workflows/`, `tools.json`, `tools/deps-allowlist.json`, `tools/importcheck/rules.json`, `api/exports.golden`, `api/clock.golden`, `gate/current-sprint`, `docs/language/grammar.lark`, `docs/language/grammar.ebnf`, `docs/language/SPEC.md`, `docs/language/SEMANTICS.md`, `docs/language/errors.md`, `docs/architecture/ARCHITECTURE.md`, `docs/architecture/adr/`) ⊆ `law.json`'s paths, and `tools/allowlists/`, `tools/analyzers/` untracked. Green on the real file; shown red once by a scratch removal of one entry, the output quoted in the lane's report *(review: QA-S10)*.
- `gate/ci/tests/test_generate.py`: `assertEqual(generate.NEEDS_HISTORY, frozenset({"apidiff", "lawdelta"}))` (first red: `frozenset({'apidiff'}) != …`) and, per id, a rendered registry with that row in `lint` at T4 → `fetch-depth: 0` in `lint` only; `test_the_job_running_deps_downloads_modules`: a registry with `deps` in `lint` → the two `go mod download all` steps in `lint` only, the second with `working-directory: gate/proofs`.
- The driver `test_lawdelta_planted.py` (W0, 7 cases) green, plus one added case: a dirty law file at T1 → the sixth line present, the row PASS.

## 3. Lane D — the sprint grammar, the order, the marker, the expiry

### 3.1 Files

`gate/runner/sprintid.py` (new), `gate/runner/notbuilt.py` (new, the pure rule), `gate/runner/rows/notbuilt.py` (replaces the stub), `gate/runner/tests/test_sprintid.py` (QA's, from `s1-1c-1/qa` — lane D's first commit), `test_notbuilt.py` (new), `test_sprints_pin.py` (new); `gate/proofs/drivers/test_notbuilt_expired.py` (W0; made green).

### 3.2 `gate/runner/sprintid.py`

```python
@dataclass(frozen=True)
class ID:      stage: int; part: int; letter: str; n: int        # 0 / "" when absent
@dataclass(frozen=True)
class Row:     id: str; closed: bool                             # id is a printed row id ("S3.3", "S1.1a-1")
@dataclass(frozen=True)
class Order:   rows: tuple[Row, ...]
    def index(self, text: str) -> int
    def reached(self, entry: str, current: str) -> bool
    def first_open(self) -> str

GRAMMAR = re.compile(r"S(?P<stage>[1-9][0-9]*)(?:\.(?P<part>[1-9][0-9]*))?(?P<letter>[a-z])?(?:-(?P<n>[1-9][0-9]*))?")
HEADER = "| Sprint | Title | SP (items) | Routine | Depends on |"

def parse(text: str) -> ID                 # GRAMMAR.fullmatch(text) — fullmatch, never match with $ (review: architect S4); ValueError(f"{text!r}: not a sprint id (S<stage>[.<part>][<letter>][-<n>])")
def canonical(id: ID) -> str               # f"S{stage}" + (f".{part}" if part) + letter + (f"-{n}" if n); canonical(parse(x)) == x
def row_of(id: ID) -> str                  # canonical(replace(id, n=0))
def load_order(text: str) -> Order         # top level exactly {"schema", "note", "rows"}; schema == 1; rows a non-empty list of objects with keys exactly {"id", "closed"} (str parsing, bool); ids unique; ValueError(f"gate/sprints.json: rows[{i}]: {fault}") / f"gate/sprints.json: {fault}"
def plan_rows(markdown: str) -> list[Row]  # the plan's §3 table (used only by the T2 pin test)
def read_marker(text: str) -> ID           # text must be exactly one line ending in one "\n" with no other whitespace; ValueError("gate/current-sprint: one line, one trailing newline, got {text!r}")
def compare_rows(plan: Sequence[Row], order: Order) -> str     # "" when equal; else f"gate/sprints.json row {i} is {a} ({open|closed}); sprint-plan.md §3 row {i} is {b} ({open|closed})" at the first difference (a missing row on either side: "absent")
def first_open_mismatch(marker: ID, order: Order) -> str       # "" when row_of(marker) == order.first_open(); else f"gate/current-sprint {canonical(marker)} is not the first open row {order.first_open()}"
```
- `Order.index(text)`: `id = parse(text)` (its error propagates); the printed id `canonical(id)` if it is a row's id, else `row_of(id)` if that is, else `ValueError(f"{text}: not a row of gate/sprints.json")`. `reached(entry, current)` = `index(entry) <= index(current)`. `first_open()` = the first row with `closed == False`, else `ValueError("every row of gate/sprints.json is closed")`.
- `plan_rows(markdown)`: the line equal to `HEADER` — absent → the first line starting `| Sprint` gives `ValueError(f"sprint-plan.md line {n}: the §3 table header reads {line!r}, not {HEADER!r}")`, and none at all → `ValueError("sprint-plan.md: the §3 table header was not found")` *(review: QA-S1)*; the separator row skipped; then every line starting `| ` until a line that does not; cells split on `|`; **a `✓` in any cell but the first → `ValueError(f"sprint-plan.md line {n}: ✓ outside the id cell")`** (decision 6); the first cell stripped: `~~…~~` → skipped; else one of exactly `<id>`, `<id> [✓](<link>)`, `<id> ✓ (<text>)` (regexes pinned in the module) → `Row(id, closed)`, any other shape → `ValueError(f"sprint-plan.md line {n}: unknown row shape {cell!r}")`; `<id>` must parse.

### 3.3 `gate/runner/notbuilt.py` — the rule

```python
@dataclass(frozen=True)
class Entry:  file: str; construct: str; sprint: str
def load_entries(text: str) -> tuple[Entry, ...]
    # top level exactly {"schema", "note", "entries"}; schema == 1; entries a list of objects with keys exactly {"file", "construct", "sprint"}, all non-empty str; files unique;
    # each sprint parses and has n == 0, else ValueError(f"conformance/notbuilt.json: entries name a row, not a part: {sprint}"); other faults ValueError(f"conformance/notbuilt.json: entries[{i}]: {fault}")
def expired(entries: Sequence[Entry], order: Order, current: ID) -> list[str]
    # for each entry with order.reached(entry.sprint, canonical(current)): f"{file}: not built ({construct}), listed for {sprint}, which is reached by {canonical(current)}"
    # an entry whose sprint is not a row → ValueError(f"conformance/notbuilt.json: {file}: {sprint}: not a row of gate/sprints.json")
    # an entry naming the current row is reached — by design, that is the red the expiry exists for (review: architect R5, QA L5)
```

### 3.4 `gate/runner/rows/notbuilt.py` — the row

```
start
  text = read "gate/current-sprint"                 OSError → failed: notbuilt: gate/current-sprint: missing; exit 1
  current = read_marker(text)                        ValueError → failed: notbuilt: <message>
  order = load_order(read "gate/sprints.json")       ValueError → failed: notbuilt: <message>
  order.index(canonical(current))                    ValueError → failed: notbuilt: <id>: not a row of gate/sprints.json
  entries = load_entries(read "conformance/notbuilt.json")   ValueError → failed: notbuilt: <message>
  covered(1)
  finish(expired(entries, order, current))           ValueError → failed: notbuilt: <message>
```

### 3.5 `test_sprints_pin.py` (T2, `pytests`)

`test_sprints_json_equals_the_plan_table`: `compare_rows(plan_rows(read docs/sprints/sprint-plan.md), load_order(read gate/sprints.json)) == ""`, failing with the message; `test_the_marker_is_the_first_open_row`: `first_open_mismatch(read_marker(read gate/current-sprint), order) == ""`. Both read the real files (`gate/runner/tests` → `../../..`). `compare_rows` and `first_open_mismatch` are unit-tested on fixtures in `test_sprintid.py`'s module (lane D adds the cases); each pin is shown red once by a scratch edit of the real file (row 13's `closed` flipped; `S4.1\n` written to the marker), the output quoted in the lane's report *(review: QA-S10)*.

### 3.6 Lane D's tests

`test_sprintid.py` (QA's; plus the two helpers' fixture cases and the decision-6 case `| S4.1 | The pattern engine ✓ | …` → the error naming the line); `test_notbuilt.py` — `load_entries` faults (keys, a part, a duplicate file); `expired` on a fixture order (S4.1 → three names in file order; S1.1c-1 → none; S4.2 → three; S9.2 → nine; an entry for `S1.1c` with marker `S1.1c-1` → one line; an unknown row → the error). First reds: `ModuleNotFoundError: No module named 'gate.runner.sprintid'` / `'gate.runner.notbuilt'`; the driver: `('fail', 'notbuilt: not built yet (lane D)')`.

## 4. Lane B — the Go loader and the counts test

### 4.1 `conformance/corpus_test.go`

The map `notBuilt` (lines 24–38) is replaced by *(review: QA-S4, architect S5)*:
```go
// notBuiltEntry is one entry of conformance/notbuilt.json (a law path): the construct a refusal names and the plan row
// that turns the file green. The expiry — red at T4 once gate/current-sprint reaches the row — is the gate's notbuilt
// row (S1.1c-1); this test only reads the list, and never loosely.
type notBuiltEntry struct {
	File      string `json:"file"`
	Construct string `json:"construct"`
	Sprint    string `json:"sprint"`
}

type notBuiltFile struct {
	Schema  int             `json:"schema"`
	Note    string          `json:"note"`
	Entries []notBuiltEntry `json:"entries"`
}

var sprintRow = regexp.MustCompile(`^S[1-9][0-9]*(\.[1-9][0-9]*)?[a-z]?$`) // a row, never a part (no -<n>)

// parseNotBuilt is pure: the file's bytes to the map, or the first fault.
func parseNotBuilt(data []byte) (map[string]notBuiltEntry, error)
// loadNotBuilt reads conformance/notbuilt.json and fails the test on any fault.
func loadNotBuilt(t *testing.T) map[string]notBuiltEntry
```
`parseNotBuilt` decodes with `json.NewDecoder(bytes.NewReader(data))` and `DisallowUnknownFields()` into `notBuiltFile`; faults, each an error whose text the test pins: `schema %d, want 1`; `entries[%d]: file, construct and sprint must be non-empty`; `entries[%d]: duplicate file %q`; `entries[%d]: %q names a part, not a row` (fails `sprintRow`); an unknown key → the decoder's error. `TestCorpus` uses `notBuilt := loadNotBuilt(t)`; its three verdicts (`corpus_test.go:66-76`) are unchanged except the field rename `want.construct` → `want.Construct`, `want.sprint` → `want.Sprint`. `TestNotBuiltListIsLaw` (T2): a table over four bad fixtures (a part `S1.1c-2`; a duplicate file; `schema 2`; an unknown key `"comment"`) asserting each error text, and the real file parsing with nine entries. `TestSprintRowGrammar`: the valid and invalid lists of the plan check §6 grammar (2) that concern rows (no `-<n>`) against `sprintRow` *(review: QA L2)*.

### 4.2 `internal/parser/errorcounts_test.go`

Green at 0030d44 (red at 68a7921 on 67 ≠ 68 and 44 ≠ 45, quoted in the record). Lane B keeps it; nothing to build.

## 5. Lane C — the `deps` row

### 5.1 Files

`gate/runner/deps.py` (new, pure), `gate/runner/rows/deps.py` (replaces the stub), `gate/runner/tests/test_deps.py` (new); `gate/proofs/drivers/test_deps_planted.py` (W0; made green, its assertions tightened — §5.4).

### 5.2 `gate/runner/deps.py`

```python
ALLOWED = frozenset({"Apache-2.0", "MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC"})
LICENSE_FILES = ("LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md")
DROPPED_NODES = frozenset({"go", "toolchain"})           # go mod graph's version nodes (review: architect S7)

@dataclass(frozen=True)
class Module:  path: str; version: str; dir: str; imported: bool      # dir "" when absent on disk
@dataclass(frozen=True)
class Entry:   module: str; version: str; scope: str; license: str; approved: str; why: str

def load_allowlist(text: str) -> tuple[Entry, ...]    # top level exactly {"schema", "note", "modules"}; schema 1; modules non-empty; each object's keys exactly the six, all non-empty str; modules unique; ValueError(f"tools/deps-allowlist.json: modules[{i}]: {fault}")
def spdx_parts(field: str) -> frozenset[str]          # split on " AND "; each part stripped and non-empty, else ValueError
def classify_license(text: str) -> str | None         # case-insensitive, first match: "Apache License" and "Version 2.0" → Apache-2.0; "Permission is hereby granted, free of charge" → MIT;
                                                      # "Redistribution and use in source and binary forms" with "endorse or promote" → BSD-3-Clause, without → BSD-2-Clause; "Permission to use, copy, modify, and/or distribute" → ISC; else None
def decode_json_stream(text: str) -> list[dict]       # go list -m -json: objects back to back; json.JSONDecoder().raw_decode in a loop (review: architect S7)
def compare(modules: Sequence[Module], graph: frozenset[str], entries: Sequence[Entry], gomods: Sequence[str], read: Callable[[str], str | None]) -> list[str]
    # read(dir): the first LICENSE_FILES text found under dir, decoded as UTF-8 with errors="replace" (the fingerprints are ASCII — review QA L3), or None.
    # Failures, one line each, in this order:
    #   sorted(set(gomods)) != ["gate/proofs/go.mod", "go.mod"] → f"deps: tracked go.mod files: {', '.join(sorted(set(gomods)))}; ADR-0001 §1 allows two: go.mod, gate/proofs/go.mod"
    #   a module (build list) with no entry → f"deps: {path} {version} is not in tools/deps-allowlist.json"
    #   a graph node (after DROPPED_NODES and the main modules) with no entry → f"deps: {path} is in the module graph and not in tools/deps-allowlist.json"
    #   an entry whose module is in neither the build list nor the graph → f"deps: stale entry {module}: not in the module graph of go.mod or gate/proofs/go.mod"
    #   an entry whose version != the module's version → f"deps: {module}: the allowlist says {entry.version}, the graph has {version}"
    #   an entry with a part of its license not in ALLOWED → f"deps: {module}: license {part} is not Apache-2 compatible (allowed: Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause, ISC)"
    #   an imported module: dir "" → f"deps: {path}: no source on disk; run: go mod download {path}"; read None → f"deps: {path}: no LICENSE file in {dir}";
    #     classify None → f"deps: {path}: unknown licence text in {dir}"; classified not in spdx_parts(entry.license) → f"deps: {path}: license field {entry.license}, file reads {classified}"
```

### 5.3 `gate/runner/rows/deps.py` — the row

```
start
  env = the row's clean env + GOPROXY=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local CGO_ENABLED=0
  for mod in (".", "gate/proofs"):
    build[mod]    = decode_json_stream(go list -m -json all)      (cwd mod): Module(Path, Version, Dir or "", imported=False) per object with "Version"; the main module skipped
    graph[mod]    = go mod graph (cwd mod): both ends of every edge, the "@…" suffix cut, DROPPED_NODES and the main modules removed
    imported[mod] = go list -e -deps -test -f '{{if .Module}}{{.Module.Path}}{{end}}' ./...   (cwd mod)
  (after decision 7 the tool directive lives in gate/proofs/go.mod, so its modules are imported through the proofs module's `go list -e -deps` of golang.org/x/exp/cmd/apidiff, run there)
  a module in both build lists with two versions → failed: deps: <path>: go.mod has <v1>, gate/proofs/go.mod has <v2>
  a `go` command exiting non-zero → failed: deps: go <args> failed: <first stderr line>; exit 1
  gomods = git ls-files -z -- 'go.mod' ':(glob)**/go.mod'   (a set)
  entries = load_allowlist(read tools/deps-allowlist.json)   ValueError → failed: deps: <message>
  covered(3)
  finish(compare(union of builds marked imported, union of graphs, entries, gomods, read_license))
```
`-e` keeps a package that does not build in the listing with `.Error` set; its imports are read as far as they parsed, so a module reached only through a broken package falls from imported to graph-only — weaker, never red; stated here *(review: QA R3)*. CI's `lint` job downloads modules at setup (decision 1), so a cold cache is not a verdict; locally the "run: go mod download" line is the fix.

### 5.4 Lane C's tests

`test_deps.py` — every fault of `load_allowlist`; `spdx_parts`; `classify_license` on the five opening texts, GPL, empty; `decode_json_stream`; `compare` for every failure text of §5.2 in order, the version mismatch, the compound licence (`MIT AND Apache-2.0` with a file reading MIT), the graph-node-without-entry, the `go`/`toolchain` drop, and the green case. First red: `ModuleNotFoundError: gate.runner.deps`. The driver (W0, six cases) green with its assertions tightened *(review: QA-S11)*: the world's two `go.mod` copies have the `tool` directive and its requires **removed** (so no module is imported but the plant), and each case asserts `assertEqual(failed, [expected])` — the one planted line and nothing else; the world's planted module stays `../planted` beside the repository (inside the driver's temp directory), as the driver has it, because `commit()`'s `git add -A` would otherwise track a third `go.mod` (the plan check's `./planted` is superseded).

## 6. The coordinator's items

| When | What |
|---|---|
| G1 | this design committed; lane A's and lane D's worktrees cut from integration; lane D's first commit is QA's `test_sprintid.py` (cherry-picked from `s1-1c-1/qa` 2d8baaf's sibling d9ad7da) |
| after W1 | `make ci-workflow` (the `go mod download` steps and `lawdelta`'s history appear in `gate.yml`); the T3 run; ADR-0015 §1.3 annotated as built (`gate/law.json`'s shape; `blocking: true`, red only when it cannot read; the six texts; the block through the record; the schema check); §1.1's annotations gain `law` in the record; ADR-0015 §2's `todocheck` row and ADR-0005 §4: the expiry is the `notbuilt` row over `conformance/notbuilt.json` and `gate/sprints.json` |
| W2 (decision 7) | `go.mod`: the `tool golang.org/x/exp/cmd/apidiff` line and the four `// indirect` requires removed, `go mod tidy`; `gate/proofs/go.mod`: `tool golang.org/x/exp/cmd/apidiff` added, its requires, `go mod tidy` there; `go.sum` files follow; `gate/runner/rows/apidiff.py` builds the tool from the proofs module — `go -C gate/proofs build -o <row_dir>/apidiff golang.org/x/exp/cmd/apidiff` — and runs that binary from the repository root with its present arguments (the binary does not depend on the working directory; the existing `go tool apidiff` invocation is replaced; proven by a scratch run before the edit, sprint model §10); the four allowlist entries' `scope` read `build-time (go tool apidiff in gate/proofs, the T4 apidiff row only)`; ADR-0009 §2 annotated; the `apidiff` proof driver re-run |
| after W2 | ADR-0009 §1 and ADR-0001 §1: the `deps` row exists (what it checks; graph-only modules by field); `docs/development/DEVELOPMENT.md` "what exists" and the gate section (the LAW Δ block, the marker, the expiry, `deps`); the workflows regenerated if the registry changed; `gate/proofs/drivers/support.py:6`'s docstring ("drivers run at T2 and never T4") corrected — `test_notbuilt_expired.py` runs its one row at T4 *(review: QA L1)* |

## 7. Build order

| Step | Lane | Lands | Red first (the exact first failure line) |
|---|---|---|---|
| A1 | A | `git.py` (+ `apidiff.py` re-pointed) | `test_git.py`: `ModuleNotFoundError: No module named 'gate.runner.git'` |
| A2 | A | `law.py` manifest, `entry_for`, `names`, `line`, `block`; `rows/lawdelta.py` with path/±lines (every kind `""`), liveness, the merge-base rule, the sixth line, `law.txt` | `test_law.py` (manifest), `test_lawdelta.py`: `('fail', 'exit 1: lawdelta: not built yet (lane A)') != ('pass', 'covered 1')` |
| A3 | A | `records` + `_Done.law` + the pass-with-no-block rule + `render(law)` + `summarize` (fields, schema, inconsistent, the CI block) + the five texts | `test_records.py`: `TypeError: make() got an unexpected keyword argument 'law'`; `test_summary.py`: `AssertionError: TypeError not raised`; `test_summarize.py`: `'malformed' not found in …`; `test_main.py`: line 0 `'job | step | …' != 'LAW Δ: not run — …'` |
| A4 | A | the parsers and `doc_line`, one kind per commit | `test_law.py` per kind: `AssertionError: '' != '+1 edge …'` |
| A5 | A | `test_law_pin.py`; `generate.py` (`NEEDS_HISTORY`, `NEEDS_MODULES`) | the pin: a scratch red; `test_generate.py`: `frozenset({'apidiff'}) != frozenset({'apidiff', 'lawdelta'})` |
| A6 | A | the driver green (+ the sixth-line case); T2; `gate/run.py --tier T4 --job lint` on the integration branch (needs a local `main`) | the driver: `EDGE_DELTA not found in block` after the stub's fail |
| D1 | D | `parse`/`canonical`/`row_of` | QA's `test_sprintid.py`: `ModuleNotFoundError: No module named 'gate.runner.sprintid'` |
| D2 | D | `Order`, `load_order`, `read_marker` | the same module's order cases |
| D3 | D | `plan_rows` (the header error, decision 6), `compare_rows`, `first_open_mismatch`; `test_sprints_pin.py` | the fixture cases; the two scratch reds quoted |
| D4 | D | `notbuilt.py` rule | `test_notbuilt.py`: `ModuleNotFoundError: No module named 'gate.runner.notbuilt'` |
| D5 | D | the row replacing the stub; the driver green; T2 | the driver: `('fail', 'exit 1: notbuilt: not built yet (lane D)') != ('pass', 'covered 1')` |
| B1 | B | `parseNotBuilt`, `loadNotBuilt`, the map deleted, `TestNotBuiltListIsLaw`, `TestSprintRowGrammar` | the package does not compile (`undefined: loadNotBuilt`) — an existence red; the red that counts: the part fixture through a loader without the `sprintRow` rule, quoted |
| B2 | B | T2 | — (the counts test is green since 0030d44) |
| C1 | C | `deps.py` pure | `test_deps.py`: `ModuleNotFoundError: No module named 'gate.runner.deps'` |
| C2 | C | the row replacing the stub; the driver tightened and green; the real tree green; T2 | the driver: `[] != ['deps: example.com/planted v1.0.0 is not in tools/deps-allowlist.json']` after the stub's line |

## 8. What this design does not do

No semantic one-liner for `code`/`workflow` (grouped counts only). No change to `apidiff`'s silent pass (an owner decision of 2026-11-07). No `instruments` section in `law.json` (S1.1c-2's teeth). No `todocheck` (it reads `sprintid.parse`/`row_of` when it lands). No network in any row: `GOPROXY=off`; CI's setup step and the installer are the only network steps; a missing module source locally is a named failure with its fix command. The `-e` listing's weakness (§5.3) is stated, not closed.

## 9. The reviewers' questions, answered

- **R1** — yes; the hole was the pass with an empty block, closed in §2.5; the schema check is named (decision 5).
- **R2** — a `✓` outside the id cell is now an error (decision 6); a title, SP, routine or dependency edit is not the gate's business.
- **R3** — `-e` is a lower bound, stated in §5.3 and §8.
- **R4** — `§<n>` or the heading text, with the hunk count (decision 3).
- **R5** — an entry naming the current row expires by design; `test_notbuilt.py` has the case.

## 10. Review placement

| # | Reviewer | Severity | Finding | Disposition |
|---|---|---|---|---|
| B1 / QA-S12 | architect / QA | blocker / should-fix | the `deps` verdict depends on CI's module cache | decision 1: `NEEDS_MODULES` setup step (§2.5, §5.3); `test_generate.py` |
| S3 / QA-B1 | architect / QA | should-fix / blocker | a rename crashes `numstat`'s reader | fixed §2.2 (`--no-renames`), `test_git.py` |
| S1 | architect | should-fix | a `lawdelta` pass with an empty block | fixed §2.5 (the rule in `_finish`; `inconsistent` in CI); tests named |
| S2 | architect | should-fix | text (v) untrue under `--job` | fixed §2.5 (the selection scope string); `test_main.py` case |
| S4 | architect | should-fix | `$` accepts a trailing newline | fixed §3.2 (`fullmatch`) |
| S5 | architect | should-fix | the loaders would reject the `note` keys | fixed §1 (exact key sets), §4.1 (`Note` field, `DisallowUnknownFields` on) |
| S6 / QA-S7 | architect / QA | should-fix | the `doc` one-liner's text | decision 3; fixed §2.3 `doc_line` |
| S7 | architect | should-fix | the JSON stream; `go`/`toolchain` nodes; graph-node-without-entry | fixed §5.2/§5.3 |
| L1 | architect | later → decided | uncommitted law edits | decision 2; fixed §2.2 `dirty`, §2.4 |
| L2, L3, L4 | architect | later | duplicate keys; `one_line`; the `lint` literal | fixed §2.3, §2.5 |
| pre-mortem 1, 3 | architect | — | block noise; schema bumps | decisions 4 and 5 |
| build-readiness 1–13 | architect | — | the guesses | each named in §2–§5 |
| QA-S1 | QA | should-fix | the header error text vs `test_sprintid.py` | fixed §3.2 |
| QA-S2 | QA | should-fix | `test_main.py` vacuous | fixed §2.6 (a git fixture, exact lines) |
| QA-S3 | QA | should-fix | `NEEDS_HISTORY` test green early | fixed §2.6 (set equality) |
| QA-S4 | QA | should-fix | `TestNotBuiltListIsLaw` not writable | fixed §4.1 (`parseNotBuilt` pure) |
| QA-S5 | QA | should-fix | two texts no test reaches | fixed §2.6 (in-process mock tests) |
| QA-S6 | QA | should-fix | `edges` keys shortened in the example | fixed §2.3 |
| QA-S8 | QA | should-fix | `baseline` name not derivable; the file name | fixed §2.3 (`rule {rule}, block {block}`), §2.6 (the real file's lines); the plan check's path noted as superseded |
| QA-S9 | QA | should-fix | `exports` placeholders; the truncation rule | fixed §2.3 (`names`, exact f-strings, the continuation rule) |
| QA-S10 | QA | should-fix | fixture reds are of the comparison | fixed §3.2 helpers, §2.6/§3.5 scratch reds quoted |
| QA-S11 | QA | should-fix | tolerated extra reds; `../planted` | fixed §5.4 (the tool directive stripped in the world; `assertEqual` lists; the path stated) |
| QA-S13 | QA | should-fix | the missing-record fixture | fixed §2.6 |
| QA-S14 | QA | should-fix | git isolation in the new tests | fixed §1 |
| QA L1–L7 | QA | later | docstring; the two grammars; decoding; schema; R5 case; numstat order; gomods set | fixed §6, §4.1, §5.2, §2.5, §3.3, §2.6, §5.3 |
| R2 (✓ in a title cell) | QA | — | — | decision 6 |
| embedding question | owner | — | the `tool apidiff` directive | decision 7; §6 W2 |
