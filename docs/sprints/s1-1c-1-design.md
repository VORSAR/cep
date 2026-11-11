# S1.1c-1 — design note (v1, 2026-11-11)

**Sprint:** [S1.1c-1](sprint-plan.md#s11c-1--the-law-manifest-and-the-law-δ-block-the-current-sprint-marker-and-the-not-built-expiry-the-dependency-checks-the-error-code-counts) · **Plan check:** [s1-1c-1-plan-check.md](s1-1c-1-plan-check.md) (v2) · **Record:** [s1-1c-1-record.md](s1-1c-1-record.md) · **Branch:** `s1-1c/integration` @ 41c2825 (the G0 law commit). *Build-ready: every path, every signature, every text the gate prints, every test by name, the steps in order. Reviewed before any lane codes (G1); the reviewers' findings are placed in §10.*

**Binding decisions, quoted:** [2026-11-10, S1.1c's scope](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check) — 3 (S1.1c-1's items), 4 ("semantic deltas per structured law file … one small parser per structured file, each with a planted-edit proof"), 7 (the full loop without the panel), 8 ("a T2 test fails when `gate/current-sprint` is not the first sprint-plan row without ✓"), 9 ("`lawdelta` … FAILS when there is no merge base — never the silent pass of the `apidiff` row"; "the block is the summary renderer's own output carried into CI's aggregated summary"). [2026-11-11, the plan check](decisions.md#owner-decisions-2026-11-11--s11c-1s-plan-check) — 1 (one `deps` row), 2 (`conformance/notbuilt.json`), 3 (`HEAD is main — read the PR run`, PASS, merge base == HEAD), 4 (fingerprint for imported, field + liveness for graph-only), 5 (the order of work), 6 (the expiry is a Python row; `gate/runner/sprintid.py`), 7 (`gate/sprints.json` pinned to the plan table), 8 (`kr/pretty`, `check.v1` approved).

## 1. What exists after G0 (read it, do not re-derive it)

`gate/current-sprint` = `S1.1c-1\n` · `gate/sprints.json` = `{"schema": 1, "note": …, "rows": [{"id": "S1.1a-1", "closed": true}, … 26 rows …]}` · `gate/law.json` = `{"schema": 1, "note": …, "paths": [{"path", "kind"[, "flag"]} × 23]}` (the kinds: `registry`, `law`, `marker`, `sprints`, `ceilings`, `code`, `workflow`, `tools`, `deps`, `edges`, `exports`, `baseline`, `notbuilt`, `doc`) · `conformance/notbuilt.json` = `{"schema": 1, "note": …, "entries": [{"file", "construct", "sprint"} × 9]}` · registry rows `lawdelta` (lint, T1–T4, `expected_scope ["git", "gate/law.json"]`, `min_wants 1`), `deps` (lint, T2–T4, `["git", "go.mod", "gate/proofs/go.mod", "tools/deps-allowlist.json"]`, `min_wants 3`), `notbuilt` (test, T4, `["git", "conformance/notbuilt.json"]`, `min_wants 1`), each running `python3.15 -I -B -X utf8 gate/runner/rows/<id>.py` — today a stub that prints `failed: <id>: not built yet (lane X)` and exits 1 · `tools/deps-allowlist.json` = 23 entries, one module each.

The gate's conventions every lane keeps: a row script starts with `rows.start()` (its row directory, `GATE_ROW_DIR`), prints `GATE_COVERED <n>` through `rows.covered(n)` once its scope is read, and ends with `rows.finish(failures)` (`failed: <line>` per failure, exit 1; exit 0 on none) — `gate/runner/rows/__init__.py:25-95`; an uncaught exception prints `internal error:` and exits 2 (`gate/runner/step.py:40-50` reads the first such line as the reason). Everything runs under the pinned CPython 3.15.2 with `-I -B -X utf8`; stdlib only; no shell. Tests are `unittest` modules under `gate/<pkg>/tests/`, run by the `pytests` row; the proof drivers under `gate/proofs/drivers/` by the `proofs` row (T4).

## 2. Lane A — the law manifest, the `lawdelta` row and the LAW Δ block

### 2.1 Files

| File | New / changed | Owner |
|---|---|---|
| `gate/runner/git.py` | new: `merge_base()`, `head()`, `ls_files(paths)`, `numstat(base, head, paths)`, `show(rev, path)` | A |
| `gate/runner/rows/apidiff.py` | changed: imports `merge_base` from `gate.runner.git`; behaviour unchanged (its silent pass stands by the owner's decision of 2026-11-07, `apidiff.py:7`) | A |
| `gate/runner/law.py` | new: the manifest reader and the pure semantic parsers | A |
| `gate/runner/rows/lawdelta.py` | replaces the stub | A |
| `gate/runner/records.py` | changed: `SCHEMA = 2`; `make(..., law: Sequence[str] = ())` | A |
| `gate/runner/scheduler.py` | changed: `_record` reads `law.txt`; `_close` computes the block and passes `law=` | A |
| `gate/runner/summary.py` | changed: `render(..., law: Sequence[str], ...)` required; prints it first | A |
| `gate/runner/summarize.py` | changed: `FIELDS["law"] = list`; `_valid` checks each element is `str`; the block from the `lint` job's `lawdelta` record | A |
| `gate/ci/generate.py` | changed: `NEEDS_HISTORY = frozenset({"apidiff", "lawdelta"})` | A |
| `gate/runner/tests/test_git.py`, `test_law.py`, `test_lawdelta.py`, `test_law_pin.py` | new | A |
| `gate/runner/tests/test_summary.py`, `test_summarize.py`, `test_main.py`, `test_records.py`; `gate/ci/tests/test_generate.py` | changed | A |
| `gate/proofs/drivers/test_lawdelta_planted.py` | from QA (W0); made green | A |

### 2.2 `gate/runner/git.py`

```python
def git(*args: str, cwd: str | None = None) -> tuple[int, str]      # subprocess.run(["git", *args]); (returncode, stdout.strip()); stdin DEVNULL; text
def merge_base() -> str                                              # "origin/main" then "main": the first `git merge-base HEAD <ref>` that succeeds, else ""
def head() -> str                                                    # `git rev-parse HEAD`
def ls_files(paths: Sequence[str]) -> list[str]                      # `git ls-files -z -- <paths>`: every tracked path under the given paths (a path ending "/" is a prefix)
def numstat(base: str, head: str, paths: Sequence[str]) -> list[tuple[str, int, int]]   # `git diff --numstat -z <base> <head> -- <paths>`: (path, added, removed); a binary file counts (0, 0)
def show(rev: str, path: str) -> str | None                          # `git show <rev>:<path>` decoded UTF-8 (errors="replace"); None when the path is absent at rev
```
`merge_base` keeps `apidiff.py:28-33`'s exact behaviour (the callers decide what "" means).

### 2.3 `gate/runner/law.py` — the manifest and the parsers

```python
KINDS = frozenset({"registry", "law", "marker", "sprints", "ceilings", "code", "workflow", "tools", "deps", "edges", "exports", "baseline", "notbuilt", "doc"})

@dataclass(frozen=True)
class Path:   path: str; kind: str; flag: str = ""       # flag only for kind "doc"
@dataclass(frozen=True)
class Manifest:   paths: tuple[Path, ...]
class ManifestError(ValueError): ...

def load_manifest(text: str) -> Manifest
    # JSON object; "schema" == 1; "paths" a non-empty list of objects with "path" (str, non-empty, no leading "/", no "..", unique), "kind" in KINDS,
    # "flag" (str) present iff kind == "doc"; anything else → ManifestError naming the entry index and the fault
def entry_for(m: Manifest, file: str) -> Path | None
    # the entry whose path equals file, or the longest entry path ending "/" that is a prefix of file; None when none
def delta(kind: str, old: str | None, new: str | None) -> str
    # the one-line semantic delta for one changed file (old/new None = absent at that side); never raises for bad content:
    # a text that does not parse yields "unparseable (old)" / "unparseable (new)"; kind "code"/"workflow" yield ""
def doc_heading(old: str | None, new: str | None) -> str
    # the heading above the first differing line: difflib.SequenceMatcher over lines; the first opcode that is not "equal"
    # gives the index in new (or old for a pure deletion); the nearest preceding line matching ^#{1,6}\s+ in that text,
    # with its "#"s and trailing "#"s stripped; "" when no heading precedes it; "(file added)" / "(file removed)" for a side that is None
def line(path: str, added: int, removed: int, one_liner: str) -> str
    # f"{path} +{added}/−{removed}" + (f" — {one_liner}" if one_liner else "")
def block(base_short: str, lines: Sequence[str]) -> list[str]
    # [] → ["LAW Δ: none"]; else [f"LAW Δ ({n} files, vs {base_short})", *lines]
```
The per-kind one-liners (each a pure function `_delta_<kind>(old, new) -> str`, joined with `; ` when several items apply; an item list longer than 6 names is written `+9 edges (a → b, c → d, …)` with the first 3 and "…"):

| kind | reads | one-liner items |
|---|---|---|
| `edges` | `tools/importcheck/rules.json`: `imports`, `test_imports`, `external`, `test_external` as edge sets `pkg → imp` (external edges as `pkg → ext:<mod>`; test edges as `pkg →test imp`); `fences`/`test_fences` as `(pkg, imp) → tuple` | `+1 edge expr → runtime`, `−1 edge …`, `fence expr ← internal/model: Expr,Lit → Expr,Lit,Pos`, `module a → b`, `note changed` |
| `registry` | both texts as raw JSON (`json.loads`, never `registry.load`); rows by `id` | `+1 row notbuilt (T4, test)`, `−1 row x`, `row unit: tiers T2,T3,T4 → T2,T3,T4,nightly`, `row proofs: min_wants 21 → 24` (every changed field, in the schema's order) |
| `exports` | lines `pkg ident kind signature…`; identity = `pkg ident`; a multi-line signature belongs to the last identity line | `+3 exports (cep.X, …)`, `−1 export (…)`, `2 signatures changed (…)` |
| `deps` | entries by `module` | `+1 module x v1.0.0 (MIT)`, `−1 module x`, `x: v1 → v2`, `x: license A → B`, `note changed` |
| `ceilings` | metrics by name; `rules` overrides | `outbox: ceiling 65536 → 1024`, `outbox: blocking true → false`, `+metric partial-count`, `−metric x`, `drained_pending[qa-eq]: 8 → 9`, `partial-count: none → 10` |
| `baseline` | `gate.baseline.regen.parse` on each side (an unparseable side → `unparseable`); blocks by `key` | `1 block re-recorded (consent/1)`, `+1 block`, `−1 block`, `source changed`, `spec_rev 10 → 11`, `format 1 → 2` |
| `notbuilt` | entries by `file` | `+1 entry x (S4.1)`, `−1 entry x`, `01-agent-kill-switch.cep: S4.1 → S4.2`, `x: construct changed` |
| `tools` | tools by `name`: `version`; any other change under a tool | `go: go1.27.3 → go1.27.4`, `ruff: hashes changed`, `+tool x`, `−tool x` |
| `marker` | the stripped text | `marker S1.1c-1 → S4.1` |
| `sprints` | rows by `id` and position | `S1.1c closed`, `S4.1 reopened`, `+1 row S4.3 at 15`, `−1 row x`, `order changed` |
| `law` | paths by `path` | `+1 path x (kind)`, `−1 path x`, `x: kind a → b`, `x: flag changed` |
| `doc` | `doc_heading` | `§9 — SPEC changed — owner decision required` (the heading text, then the entry's flag) |
| `code`, `workflow` | — | `` |

### 2.4 `gate/runner/rows/lawdelta.py` — the row

```
start
  manifest = load_manifest(read "gate/law.json")           ManifestError → failed: lawdelta: gate/law.json: <fault>; exit 1
  for each entry: tracked = ls_files([entry.path])          none → failed: lawdelta: <path> is listed in gate/law.json but git tracks nothing there; exit 1
  covered(1)                                                 (only now: the manifest is the row's scope)
  base = merge_base()                                        "" → failed: lawdelta: no merge base with main (fetch-depth 0 or a local main is needed); exit 1
  if base == head():  lines = ["LAW Δ: HEAD is main — read the PR run"]
  else:
    changes = numstat(base, head, every entry path)          [] → lines = ["LAW Δ: none"]
    for (file, added, removed) in changes, in path order:
      entry = entry_for(manifest, file)                      None → failed: lawdelta: <file> changed under a listed prefix but matches no entry (cannot happen; defensive)
      old, new = show(base, file), show(head, file)
      try: one = delta(entry.kind, old, new); doc: one = f"{doc_heading(old, new)} — {entry.flag}".lstrip(" —")
      except Exception as e: failed: lawdelta: the <kind> parser crashed on <file>: <type>: <e>; exit 1
      lines.append(line(file, added, removed, one))
    lines = block(base[:7], lines)
  write GATE_ROW_DIR/law.txt = "\n".join(lines) + "\n";  print each line;  finish([])
```
A delta is never a failure (ADR-0015 §1.3 as annotated: the row is red only when it cannot read). Uncommitted edits are not in the block: it reads `base..HEAD`; at T1 (pre-push) HEAD is the commit being pushed; the owner reads it on the PR run.

### 2.5 The record and both renderers

- `records.SCHEMA = 2`; `records.make(..., law: Sequence[str] = ())` adds `"law": list(law)` to every record.
- `scheduler._record(done, job)`: `law = _read_law(os.path.join(self._run_dir, "rows", job, done.id, "law.txt")) if done.id == "lawdelta" and done.ok else []` — `_read_law` returns the file's non-empty lines, or `[]` when the file is absent or empty.
- `scheduler._close`: `law = _law_block(self._tier, self._rows, self._finished)`:
  - the `lawdelta` row not in `self._rows` (not selected at this tier) → `[f"LAW Δ: not run — the lawdelta row is not in this registry at {tier}"]`;
  - selected, `ok` and its record's `law` non-empty → that list;
  - otherwise (fail, stopped, crashed, skipped, or an empty file) → `["LAW Δ: FAILED — see the lawdelta row"]`.
  Passed to `summary.render(lines, law=law, ...)`.
- `summary.render(lines, *, law: Sequence[str], wall_ms, summed_ms, exit_code, run_dir, registry=None, image=None)`: the output is `[*law, "", *table, …]` — the block, one blank line, then the table as today. `law` is required (no default): a caller that omits it is a `TypeError`.
- `summarize`: `FIELDS["law"] = list`; `_valid` additionally requires every element of `record["law"]` to be `str`; after the rows are judged: `law = _law_block_ci(rows, mine_lawdelta)` — the registry has no `lawdelta` row → the "not run" text with the tier; the row's record missing, malformed, duplicate, stale or `verdict != "pass"` → `FAILED — see the lawdelta row`; else the record's `law`. Passed to `render`. The existing missing-record logic already prints the row line `lint | lawdelta | FAIL | missing: no job recorded it`.
- `generate.NEEDS_HISTORY = frozenset({"apidiff", "lawdelta"})`; `test_generate.py` gains `test_every_merge_base_row_s_job_checks_out_history` iterating `NEEDS_HISTORY` by row id.

### 2.6 Lane A's tests (each red before its code)

`test_git.py` — a temp repo: `merge_base` with and without `main`; `numstat` on a two-commit repo; `show` on an absent path → None. `test_law.py` — `load_manifest` (valid; each fault); `entry_for` (exact, prefix, longest prefix, none); one test per kind with a planted pair asserting the exact one-liner (every item form in §2.3's table); `doc_heading` (a change under `## 9`, a change before any heading, added/removed file); `block` (empty, n files). `test_lawdelta.py` — the row in a temp repo with a copied manifest: each `failed:` text of §2.4; the `law.txt` content; `GATE_COVERED 1` printed after liveness. `test_records.py` — `make` carries `law`. `test_summary.py` — `render` requires `law`; the block prints first and a blank line follows. `test_summarize.py` — fixtures gain `law: []`; a record without `law` is malformed; the three CI block texts; the missing-record row line. `test_main.py` — `gate/run.py` end to end at T0, T1 and T2 in a temp git repo with a registry holding `lawdelta`: `stdout.splitlines()[0]` is the "not run" text at T0 and starts `LAW Δ` at T1/T2. `test_law_pin.py` — the §1.3 list: `gate/registry.json`, `gate/run.py`, `gate/runner/`, `gate/baseline/regen.py`, `gate/law.json`, `.github/workflows/`, `tools.json`, `tools/deps-allowlist.json`, `tools/importcheck/rules.json`, `api/exports.golden`, `api/clock.golden`, `gate/current-sprint`, `docs/language/grammar.lark`, `docs/language/grammar.ebnf`, `docs/language/SPEC.md`, `docs/language/SEMANTICS.md`, `docs/language/errors.md`, `docs/architecture/ARCHITECTURE.md`, `docs/architecture/adr/` ⊆ `law.json`'s paths; and `tools/allowlists/`, `tools/analyzers/` are untracked (so their first appearance fails this test until listed — S1.1c-2).

## 3. Lane D — the sprint grammar, the order, the marker, the expiry

### 3.1 Files

`gate/runner/sprintid.py` (new), `gate/runner/notbuilt.py` (new, the pure rule), `gate/runner/rows/notbuilt.py` (replaces the stub), `gate/runner/tests/test_sprintid.py` (from QA), `test_notbuilt.py` (new), `test_sprints_pin.py` (new); `gate/proofs/drivers/test_notbuilt_expired.py` (from QA; made green).

### 3.2 `gate/runner/sprintid.py`

The signatures of the plan check's S2, verbatim, with these rules fixed:
- `parse`: regex `^S(?P<stage>[1-9][0-9]*)(?:\.(?P<part>[1-9][0-9]*))?(?P<letter>[a-z])?(?:-(?P<n>[1-9][0-9]*))?$` over the exact text (no stripping); `ValueError(f"{text!r}: not a sprint id (S<stage>[.<part>][<letter>][-<n>])")`.
- `canonical(id)` = `f"S{stage}" + (f".{part}" if part) + letter + (f"-{n}" if n)`; `canonical(parse(x)) == x`.
- `row_of(id)` = `canonical(replace(id, n=0))`.
- `load_order(text)`: JSON; `schema == 1`; `rows` non-empty list of `{"id": str, "closed": bool}` and nothing else; each id parses; ids unique; `ValueError` names the index and the fault. `Order.rows` keeps the printed ids.
- `Order.index(text)`: `parse(text)` (its error propagates); the printed id `canonical(id)` if it is a row, else `row_of(id)` if that is a row, else `ValueError(f"{text}: not a row of gate/sprints.json")`.
- `Order.reached(entry, current)` = `index(entry) <= index(current)`.
- `Order.first_open()` = the first row with `closed == False`, else `ValueError("every row of gate/sprints.json is closed")`.
- `plan_rows(markdown)`: find the line equal to `| Sprint | Title | SP (items) | Routine | Depends on |` (pinned; absent → `ValueError("sprint-plan.md: the §3 table header was not found")`), skip the separator `|---|…`, then each line starting `| ` until a line that does not start `| `; the first cell (between the first two `|`, stripped): `~~…~~` → skipped; else the leading token up to the first space is the id (must parse); closed iff the cell contains `✓` (covers `[✓](…)` and `✓ (…)`); any cell that is not exactly `<id>`, `<id> [✓](<link>)` or `<id> ✓ (<text>)` → `ValueError(f"sprint-plan.md line {n}: unknown row shape {cell!r}")`.
- `read_marker(text)`: `text` must be one line ending in exactly one `\n` with no other whitespace → `parse(text[:-1])`; else `ValueError("gate/current-sprint: one line, one trailing newline, got …")`.

### 3.3 `gate/runner/notbuilt.py` — the rule

```python
@dataclass(frozen=True)
class Entry:  file: str; construct: str; sprint: str
def load_entries(text: str) -> tuple[Entry, ...]
    # JSON; schema == 1; "entries" a list of {"file","construct","sprint"} (all non-empty str, nothing else); files unique;
    # each sprint parses and has n == 0, else ValueError(f"conformance/notbuilt.json: entries name a row, not a part: {sprint}")
def expired(entries: Sequence[Entry], order: Order, current: ID) -> list[str]
    # for each entry whose order.reached(entry.sprint, canonical(current)): f"{file}: not built ({construct}), listed for {sprint}, which is reached by {canonical(current)}"
    # an entry whose sprint is not a row → ValueError(f"conformance/notbuilt.json: {file}: {sprint}: not a row of gate/sprints.json")
```

### 3.4 `gate/runner/rows/notbuilt.py` — the row

```
start
  marker text = read "gate/current-sprint"          OSError → failed: notbuilt: gate/current-sprint: missing; exit 1
  current = read_marker(text)                        ValueError → failed: notbuilt: <message>; exit 1
  order = load_order(read "gate/sprints.json")       ValueError → failed: notbuilt: gate/sprints.json: <message>
  order.index(canonical(current))                    ValueError → failed: notbuilt: <id>: not a row of gate/sprints.json
  entries = load_entries(read "conformance/notbuilt.json")   ValueError → failed: notbuilt: <message>
  covered(1)
  finish(expired(entries, order, current))           ValueError → failed: notbuilt: <message>
```

### 3.5 `test_sprints_pin.py` (T2, `pytests`)

`test_sprints_json_equals_the_plan_table`: `plan_rows(read docs/sprints/sprint-plan.md) == list(load_order(read gate/sprints.json).rows)`, failing with the first differing position: `gate/sprints.json row 13 is S1.1c (open); sprint-plan.md §3 row 13 is S1.1c ✓`. `test_the_marker_is_the_first_open_row`: `row_of(read_marker(read gate/current-sprint)) == order.first_open()`, failing `gate/current-sprint S4.1 is not the first open row S1.1c`. Both read the real files relative to the repository root (`gate/runner/tests` → `../../..`).

### 3.6 Lane D's tests

`test_sprintid.py` (QA's; §6 of the plan check, grammar (1)–(10)); `test_notbuilt.py` — `load_entries` faults; `expired` on a fixture order (S4.1 → three names in file order; S1.1c-1 → none; S4.2 → three; S9.2 → nine; an unknown row → the error); `test_sprints_pin.py` (above, plus a fixture-based red: a temp plan with `S1.1c ✓` against the real sprints.json → the message).

## 4. Lane B — the Go loader and the counts test

### 4.1 `conformance/corpus_test.go`

The map `notBuilt` (lines 24–38) is replaced by:
```go
// notBuiltEntry is one line of conformance/notbuilt.json (a law path): the construct a refusal names and the plan row
// that turns the file green. The expiry — red at T4 once gate/current-sprint reaches the row — is the gate's notbuilt
// row (S1.1c-1); this test only reads the list.
type notBuiltEntry struct {
	File      string `json:"file"`
	Construct string `json:"construct"`
	Sprint    string `json:"sprint"`
}

// loadNotBuilt reads conformance/notbuilt.json; a malformed file, a duplicate file name, or a sprint that names a part
// (S1.1c-2) rather than a row is fatal: the list is law and never read loosely.
func loadNotBuilt(t *testing.T) map[string]notBuiltEntry
```
`loadNotBuilt` decodes `{"schema": int, "entries": []notBuiltEntry}` with `json.Decoder.DisallowUnknownFields` off for `note` (read into a struct with `Schema int`, `Entries []notBuiltEntry`), requires `Schema == 1`, non-empty `File`/`Construct`/`Sprint`, unique files, and `Sprint` matching `^S[1-9][0-9]*(\.[1-9][0-9]*)?[a-z]?$` (no `-n`); `t.Fatalf` otherwise. `TestCorpus` uses `notBuilt := loadNotBuilt(t)` and `want, listed := notBuilt[name]` — its three refusal cases (`corpus_test.go:66-76`) unchanged. `TestNotBuiltListIsLaw` (new, T2): a fixture with `"sprint": "S1.1c-2"` is fatal through the loader (table-driven over three bad fixtures).

### 4.2 `internal/parser/errorcounts_test.go` (QA's; lane B makes it green)

`TestErrorCountsMatchTheCatalogue`: SPEC's line (the first line after `## Appendix C` matching the four-group regex of the plan check §6), errors.md's headings via the anchored regex of `fixtext_test.go:19` split by kind, `invalid/*.cep` count, every `E-[A-Z]+-[0-9]{3}` in SPEC ⊆ headings. Green after the coordinator's SPEC §9 line reads `68 codes: 23 parse-level (each with at least one counter-example file in [invalid/](invalid/); 28 files), 45 static (…)`.

## 5. Lane C — the `deps` row

### 5.1 Files

`gate/runner/deps.py` (new, pure), `gate/runner/rows/deps.py` (replaces the stub), `gate/runner/tests/test_deps.py` (new); `gate/proofs/drivers/test_deps_planted.py` (from QA; made green).

### 5.2 `gate/runner/deps.py`

```python
ALLOWED = frozenset({"Apache-2.0", "MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC"})
LICENSE_FILES = ("LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md")

@dataclass(frozen=True)
class Module:   path: str; version: str; dir: str; imported: bool     # dir "" when absent on disk
@dataclass(frozen=True)
class Entry:    module: str; version: str; scope: str; license: str; approved: str; why: str

def load_allowlist(text: str) -> tuple[Entry, ...]      # JSON; schema 1; "modules" non-empty; each field a non-empty str; modules unique; else ValueError naming the index
def spdx_parts(field: str) -> frozenset[str]            # split on " AND "; each part stripped; empty part → ValueError
def classify_license(text: str) -> str | None
    # the first match, case-insensitive, on the file's text: "Apache License" and "Version 2.0" → "Apache-2.0";
    # "Permission is hereby granted, free of charge" → "MIT"; "Redistribution and use in source and binary forms" with
    # "endorse or promote" → "BSD-3-Clause", without → "BSD-2-Clause"; "Permission to use, copy, modify, and/or distribute" → "ISC"; else None
def compare(modules: Sequence[Module], graph: frozenset[str], entries: Sequence[Entry], gomods: Sequence[str], read: Callable[[str], str | None]) -> list[str]
    # read(dir) returns the first LICENSE_FILES text found under dir, or None. Failures, each one line, in this order:
    #   gomods != ["gate/proofs/go.mod", "go.mod"] (sorted) → f"deps: tracked go.mod files: {', '.join(gomods)}; ADR-0001 §1 allows two: go.mod, gate/proofs/go.mod"
    #   a module (build list) with no entry → f"deps: {path} {version} is not in tools/deps-allowlist.json"
    #   an entry whose module is in neither the build list nor the graph → f"deps: stale entry {module}: not in the module graph of go.mod or gate/proofs/go.mod"
    #   an entry whose version != the module's version → f"deps: {module}: the allowlist says {entry.version}, the graph has {version}"
    #   an entry with a part of its license not in ALLOWED → f"deps: {module}: license {part} is not Apache-2 compatible (allowed: …)"
    #   an imported module: dir "" → f"deps: {path}: no source on disk; run: go mod download {path}";
    #     read(dir) None → f"deps: {path}: no LICENSE file in {dir}"; classify None → f"deps: {path}: unknown licence text in {dir}";
    #     classified not in spdx_parts(entry.license) → f"deps: {path}: license field {entry.license}, file reads {classified}"
```

### 5.3 `gate/runner/rows/deps.py` — the row

```
start
  env = clean + GOPROXY=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local CGO_ENABLED=0 (the pinned go is first on PATH as for every row)
  for mod in (".", "gate/proofs"):
    build[mod]  = go list -m -json all                         (cwd mod)   → Module(path, version, dir, imported=False) per object with "Version"; the main module skipped
    graph[mod]  = go mod graph                                 (cwd mod)   → every node's path (both ends of every edge), main modules skipped
    imported[mod] = go list -e -deps -test -f '{{if .Module}}{{.Module.Path}}{{end}}' ./...   (cwd mod)
  imported["."] |= go list -e -deps -f '{{if .Module}}{{.Module.Path}}{{end}}' golang.org/x/exp/cmd/apidiff   (the tool directive)
  modules = union of build lists (a path in both with different versions → failed: deps: <path>: go.mod has v1, gate/proofs/go.mod has v2 — stated, not resolved)
  mark imported; graph = union; gomods = git ls-files -z -- 'go.mod' ':(glob)**/go.mod'
  a `go` command that fails → failed: deps: go <args> failed: <first stderr line>; exit 1
  entries = load_allowlist(read tools/deps-allowlist.json)    ValueError → failed: deps: tools/deps-allowlist.json: <message>
  covered(3)
  finish(compare(modules, graph, entries, gomods, read_license))
```
`read_license(dir)` reads the first of `LICENSE_FILES` present. The `imported` set is computed from `go`, never from the allowlist's `scope` text.

### 5.4 Lane C's tests

`test_deps.py` — `load_allowlist` faults; `spdx_parts` (`MIT`, `MIT AND Apache-2.0`, a bad field); `classify_license` on the five opening texts and on GPL/unknown; `compare` for every failure text of §5.2 and the green case; the version-mismatch and compound-licence cases. The driver (QA's six cases) green; the real tree green at T2 (counter-example 7 of the plan check).

## 6. The coordinator's items

| When | What |
|---|---|
| after W0 | cherry-pick QA's drivers and `test_sprintid.py` onto `s1-1c/integration`; `proofs` `min_wants` 21 → 24 (a registry edit; `ci-fresh` unchanged); `docs/language/SPEC.md:220` → `68 codes: 23 parse-level (…; 28 files), 45 static` once the counts test is seen red (its output quoted in the record) |
| after W1 | ADR-0015 §1.3 annotated as built (`gate/law.json`'s shape; the row is `blocking: true`, red only when it cannot read; the five texts; the block carried through the record); §1.1's annotations gain `law` in the record; ADR-0015 §2's `todocheck` row and ADR-0005 §4: the expiry is the `notbuilt` row over `conformance/notbuilt.json` and `gate/sprints.json` |
| after W2 | ADR-0009 §1 and ADR-0001 §1: the `deps` row exists (what it checks, what it does not: graph-only modules by field); `docs/development/DEVELOPMENT.md` "what exists" and the gate section (the LAW Δ block, the marker, the expiry, `deps`); `.github/workflows` regenerated if the registry changed |
| open for the owner | the `tool golang.org/x/exp/cmd/apidiff` directive and its four `x/*` requires moved from `go.mod` to `gate/proofs/go.mod`, so an embedder's module graph holds only participle, x/text and goleak-as-test-only (recommended 2026-11-11, not yet decided); if yes: `apidiff.py` runs `go tool apidiff` from `gate/proofs`, the allowlist entries' `scope` lines say so, ADR-0009 §2 annotated — a coordinator item in W2 |

## 7. Build order

| Step | Lane | Lands | Red first |
|---|---|---|---|
| 1 | A | `git.py` + `apidiff.py` re-pointed | `test_git.py` |
| 2 | A | `law.py` manifest + `entry_for` + `line`/`block`; `rows/lawdelta.py` with path/±lines only (kinds yield `""`) | `test_law.py` (manifest), `test_lawdelta.py` |
| 3 | A | `records` + scheduler `law.txt` read + `render(law)` + `summarize` + the five texts | `test_records.py`, `test_summary.py`, `test_summarize.py`, `test_main.py` |
| 4 | A | the parsers, one kind per commit | `test_law.py` per kind |
| 5 | A | `test_law_pin.py`; `generate.py` | the pin; `test_generate.py` |
| 6 | A | the driver green; T2; a T4 `lint` run | `test_lawdelta_planted.py` |
| 1 | D | `parse`/`canonical`/`row_of` | QA's `test_sprintid.py` |
| 2 | D | `Order`, `load_order`, `read_marker` | the same |
| 3 | D | `plan_rows`; `test_sprints_pin.py` | the pin on a fixture plan |
| 4 | D | `notbuilt.py` rule | `test_notbuilt.py` |
| 5 | D | the row replacing the stub; the driver green; T2 | `test_notbuilt_expired.py` |
| 1 | B | the loader; the map deleted; `TestNotBuiltListIsLaw` | the fixture |
| 2 | B | the counts test green (after the SPEC line); T2 | QA's `errorcounts_test.go` |
| 1 | C | `deps.py` pure | `test_deps.py` |
| 2 | C | the row; the driver green; the real tree green; T2 | `test_deps_planted.py` |

## 8. What this design does not do

No semantic one-liner for `code`/`workflow` (path and ±lines only). No reading of uncommitted edits. No `instruments` section in `law.json` (S1.1c-2's teeth). No `todocheck` (its sprint-id reads `sprintid.py` when it lands). No change to `apidiff`'s silent pass (an owner decision). No network: `GOPROXY=off` everywhere; a missing module source is a named failure with its fix command.

## 9. Questions the reviewers should answer

- **R1** Is the record channel (§2.5) the smallest honest one, and does `summarize`'s per-element `str` check close the malformed-record hole without rejecting old artifacts for the wrong reason (the schema bump means an older attempt's records are malformed — intended: a stale attempt is red anyway)?
- **R2** `plan_rows`'s pinned header and three shapes (§3.2): what plan edit would silently pass? (A new ✓ form is an error by design; a row moved is "order changed" in the `sprints` one-liner and the pin test's first-differing-position failure.)
- **R3** The `imported` set (§5.3): is `go list -e -deps -test ./...` in the proofs module honest for a module of deliberately defective packages?
- **R4** The `doc_heading` rule (§2.3): is "the heading above the first differing line" what the owner wants to read, or the set of all changed headings?
- **R5** `reached` is `<=` by row: at S1.1c-2's G0 the marker reads `S1.1c-2`, row `S1.1c`; entries for S4.1 are not reached — correct; an entry for `S1.1c` (none exists) would be. Confirm no entry can name the current row by accident.

## 10. Review placement

| # | Reviewer | Severity | Finding | Disposition |
|---|---|---|---|---|
| | | | *(after the design reviews)* | |
