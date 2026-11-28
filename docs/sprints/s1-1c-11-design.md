# S1.1c-11 — design note (the nightly nets)

*Written against `s1-1c-11/integration` @ 584a8b66 (main 9b62eedf plus the sprint's marker and record). Every code claim cites `file:line` on 584a8b66; "inferred" marks a reading not proven by a run. Callers and imports come from GitNexus `impact` (upstream, tests included) and are confirmed by `git grep`, since the index's commit could not be read from this session. The spec is [the plan check](s1-1c-11-plan-check.md) v2 (§2 S1–S5, §5 N1–N5, §6 counter-examples (1)–(24), §10) and the owner's answers in [decisions.md, 2026-11-27](decisions.md#owner-decisions-2026-11-27--s11c-11s-plan-check) (decisions 2–5 bind this note). Deviations from the plan check are listed in §7.*

## 0. What this note settles

| Section | Lane | Files |
|---|---|---|
| N1 — the nets row | A | `gate/bench/nets.py` (new), `gate/bench/tests/test_nets.py` (new), `gate/bench/mutate.py` (typed refusals and one rename), `gate/proofs/drivers/test_nets_planted.py` (new), Makefile `nets` target (coordinator) |
| N2 — the seed set | A | `gate/nets.json` (new) |
| N3 — the `nets-anchors` row | A | `gate/bench/nets.py`'s static half, `gate/proofs/drivers/test_nets_anchors_planted.py` (new) |
| N4 — budgets | C, L, A | the pins in `gate/runner/tests/test_rows_go.py` and `test_nets.py` |
| N5 — PR CI | all | the proof drivers' worlds |
| N6 — macOS and the concurrency group | M | `gate/ci/generate.py`, `gate/ci/tests/test_generate.py`; the coordinator's tier edits in `gate/registry.json` and `gate/law.json` |

---

## N1 — the nets row

### N1.1 What `gate/bench/nets.py` takes from `gate/bench/mutate.py`

No split (plan check §2 S1; RA-3). `nets.py` does `from gate.bench import mutate` and uses these names, each verified on 584a8b66:

| Name | Signature (584a8b66) | How `nets.py` uses it |
|---|---|---|
| `entry` | `entry(spec: Mapping[str, object], timeout: str, expect: bool = False) -> Entry` (`mutate.py:561`) | Builds each net's `Entry` from a plain spec (N1.3's `spec_of(net)`), `expect=False`. It runs every refusal of the mutation and the target, and computes `changes` at HEAD through `changes_of` (`:604`). A net's own `timeout` goes in as the `timeout` argument (a Go duration string, checked by `mutate.seconds` first). |
| `changes_of` | `changes_of(spec: Mapping[str, object], rev: str \| None = None) -> tuple[Change, ...]` (`:497`) | Called only through `entry` in the nightly half. The static half (N3) does not call it: it calls the pure parts it is made of (`parse_patch` `:372`, `apply` `:466`, and `occurrences`, today `_occurrences` `:286`) on the working tree; see §7 D1. |
| `Worktrees` | `class Worktrees` with `__enter__`, `__exit__`, `at(rev: str, changes: tuple[Change, ...]) -> str`, `close() -> None` (`:767-816`) | One `with mutate.Worktrees() as trees:` for the whole run. Every run is at HEAD, so there is one worktree, reset per run (`:782-785`), removed when the run ends, a SIGTERM included (`__exit__` → `close`, `:777-778`, `:799-816`). |
| `run_one` | `run_one(e: Entry, args: argparse.Namespace, scratch: str, trees: Worktrees, head: str) -> tuple[str, gorun.Result, list[str]]` (`:930-935`) | One run of a net's target: with `e.changes` (the plant run) or with `dataclasses.replace(e, changes=())` (the control run: `trees.at(head, ())` checks out clean HEAD, `:780-797`). `args` is `argparse.Namespace(race=False, cpu=None)`: `_go` reads only `args.race` and `args.cpu` (`:889-892`), `run_py` neither (`:916-927`). Returns one of `mutate.OUTCOMES` = `("red", "green", "timeout", "invalid")` (`:106`). |
| `check_compiled` | `check_compiled(entries: Sequence[Entry], race: bool, trees: Worktrees, head: str, batched: bool) -> None` (`:853`) | The nightly's compile check: once per distinct Go package, over that package's nets (one `go list` per package, cached inside the call, `:857`, `:866-867`). On `Stale` the package's nets are re-checked one by one to name the culprit (the rare path). A plant in a `.go` file the test build never compiles would read "escaped" for a `caught` net and a false pass for a `passes` net. |
| `Refused`, `Stale` (new), `Internal` | `:167`, `:171` | The loader and the judge catch `mutate.Stale` (a net verdict) apart from other `mutate.Refused` (a broken net: refused at load) and `mutate.Internal` (exit 2). |
| `seconds`, `WALL_EXTRA_S`, `OUTCOMES` | `:227`, `:102`, `:106` | Timeouts in seconds; the budget's per-run need. |

**GitNexus.** `impact(run_one)`: HIGH, 3 direct callers (`batch` `:1054`, `repeat` `:1042`, `single` `:1036`), 4 processes, all inside `mutate.py`; `nets.py` adds a fourth caller and changes none. `impact(entry)`: LOW, callers `read_batch` `:640` and `mutate` `:1095`. `impact(changes_of)`: LOW, one caller, `entry`. `impact(check_compiled)`: LOW, one caller, `mutate`. `impact(Worktrees)` is not needed: `nets.py` only constructs it. No module imports `mutate.py` today (`git grep "from gate.bench import"` lists only `gorun`, `compare`, `escapes`); its tests run it as a subprocess (`gate/bench/tests/test_mutate.py:407-414`).

### N1.2 The one change to `mutate.py`: typed refusals (and one rename, §7 D1)

Add after `class Refused` (`:167-168`):

```python
class Stale(Refused):
    """A refusal that names a net gone stale: its file, find text, hunk, package or build no longer holds the plant.
    main() prints it as any refusal (exit 2, `error: refused: <message>`); gate/bench/nets.py reads it as a verdict."""
```

Raise sites that become `raise Stale(...)`, same message text:

| Line | Function | Message (unchanged) | Its callers (GitNexus) |
|---|---|---|---|
| `:472` | `apply` | `a hunk of {path} occurs {n} times, not exactly once` | `changes_of` ← `entry` ← `read_batch`, `mutate` ← `main` (`impact(apply)`: HIGH, 3 processes, all in `mutate.py`) |
| `:483` | `base_text` (at a rev) | `{path} does not exist at {rev[:12]}` | `changes_of` (`impact(base_text)`: HIGH, same chain) |
| `:488` | `base_text` (at HEAD) | `{path} is not tracked` | `changes_of` |
| `:524` | `changes_of` | `--find occurs {count} times in {file}, not exactly once` | `entry` |
| `:586` | `entry` | `--pkg {pkg!r} is not a directory holding tracked files` | `read_batch`, `mutate` |
| `:870` | `check_compiled` | `{where}{file} is not compiled by ./{pkg}'s test build` | `mutate` |

Not `Stale`: `:492` (`differs from HEAD`, a dirty tree, an environment fault), every key, type, timeout and patch-format refusal (a malformed net: refused at load), `:830` (`go list` failed: the package does not build, read by the judge as the control's fault).

`main` catches `Refused` (`:1126-1127`), so a `Stale` prints the same `error: refused: …` line with exit 2: mutate.py's output is byte-identical. **Test (red first):** `test_mutate.py` gains `Stale.test_each_stale_refusal_is_typed` — an in-process check that each of the six sites raises `mutate.Stale` (a fake repo as the file's other tests build), red before the class exists. GitNexus `impact(Refused)` returns UNKNOWN (exception classes are not call targets); `git grep` confirms no `except` of it outside `mutate.py`.

**The rename** `_occurrences` → `occurrences` (`:286`; one caller, `changes_of` `:522`; `impact`: 1 direct caller): with the GitNexus `rename` tool, dry run first. No net's find text contains `_occurrences` or any of the six raise lines (checked on the seed set), so neither edit stales a net.

### N1.3 `gate/nets.json` — the schema and its loader

```json
{
  "schema": 1,
  "nets": [
    {"id": "g2-111", "from": "docs/sprints/s1-1c-7-plants/E1-C3.json#0",
     "file": "internal/runtime/transitions.go",
     "find": "a.removing:\n\t\tit.reply <- reply{err: ErrRemoved}",
     "replace": "a.removing:\n\t\tit.reply <- reply{err: ErrStarted}",
     "test": "TestBreakitActivateRacesRemove", "pkg": "internal/runtime", "timeout": "120s",
     "expect": "caught", "guards": "Activate on a removing app is refused with ErrRemoved, not ErrStarted (G2-111)"}
  ],
  "retired": [
    {"from": "docs/sprints/s1-1c-7-plants/E1-starvation-plants.json#13",
     "reason": "names no test: TestFullQueueWithABusyRecorderGetsNoTurn exists only in a design note"}
  ]
}
```

**A net's keys** (the loader refuses any other key):

| Key | Required | Value |
|---|---|---|
| `id` | yes | one printable token, unique across `nets` (mutate's own rule, `:634-635`); the batch entry's `id`, else `<batch stem>-<index>` (e.g. `W0-1-2`), else the `.diff` file's stem |
| `from` | yes | `docs/sprints/<sprint>-plants/<batch>.json#<0-based index>` or `docs/sprints/<sprint>-plants/<name>.diff`; names an existing entry; unique across `nets` and `retired` together |
| `file`, `find`, `replace` | the triple, or `patch` | as mutate's batch keys; `patch` holds the diff's text (`:35-37`) |
| `test` (+ optional `subtest`) and optional `pkg`, or `py` | one target | the **new** side's target: `new_test`/`new_subtest`/`new_py` when the batch entry has them, else `test`/`subtest`/`py` (as `mutate._side` reads them, `:544-548`); `pkg` without the leading `./` |
| `timeout` | yes | a Go duration string (`"120s"`); N4.3 sets it |
| `expect` | yes | `"caught"` (the test must fail with the plant) or `"passes"` (a tolerance plant: the test must pass with it) |
| `guards` | yes | one line, no control character: what the test guards, in words (the batch's `note`, or the record's) |

**A retirement's keys:** exactly `from` and `reason`; `reason` is non-empty, one line.

**`spec_of(net) -> dict[str, str]`**: the net minus `id`, `from`, `timeout`, `expect`, `guards` — exactly mutate's plain keys (`PLAIN_KEYS`, `:111`), so `mutate.entry(spec_of(net), net["timeout"])` checks it as mutate does.

**`load(path: str) -> Nets`** (pure, no git, no build) refuses, each as `nets.json: <message>` and exit 2 of either row:

1. not UTF-8 JSON; top level not an object with exactly `schema` (= 1), `nets` (a list), `retired` (a list);
2. a net or retirement not an object; an unknown or missing key (`net <id>: unknown keys [...]`, `net <id>: missing [...]`);
3. a non-string value where a string is due; `expect` not `caught`/`passes`; `timeout` that `mutate.seconds(value, "timeout")` refuses;
4. both or neither of the triple and `patch`; both or neither of `test` and `py`; `subtest` or `pkg` with `py` (mutate's rules, run through `mutate.target(test, subtest, py)` `:305`);
5. two nets with one `id` (`net <id>: the id is not unique`) — counter-example (12);
6. one `from` twice across `nets` and `retired` (`<from>: named twice`);
7. a retirement with an empty or multi-line `reason` (`retired <from>: no reason`) — counter-example (17);
8. `guards` empty or multi-line.

```python
@dataclasses.dataclass(frozen=True)
class Net:
    id: str
    source: str            # "from"
    spec: dict[str, str]   # spec_of(net)
    timeout: str
    expect: str            # "caught" | "passes"
    guards: str

@dataclasses.dataclass(frozen=True)
class Nets:
    nets: tuple[Net, ...]
    retired: tuple[tuple[str, str], ...]   # (from, reason)

class LoadError(Exception): ...
def load(path: str = "gate/nets.json") -> Nets: ...
```

### N1.4 The judge — states and exact lines

Pure function first, runner second: `judge(control: str | None, plant: str | None, expect: str, stale: str = "") -> Verdict`, where `control` and `plant` are mutate outcomes (`red`, `green`, `timeout`, `invalid`) or `None` (not run), and `Verdict = (word, passed: bool)`. The runner composes the line.

**A target** is `(pkg, test, subtest)` for Go, `py` for Python. The nets are grouped by target, in `nets.json` order of each target's first net; the targets run in that order. One control per distinct target (225 on the seed set, N2.4).

| Step | Outcome (mutate's) | Verdict for every net of the target | Pass? | Line printed (and kept for `failed:` when red) |
|---|---|---|---|---|
| 0. Load | `LoadError` | — | exit 2 | `error: refused: nets.json: <message>` |
| 1. Entry | `mutate.Stale` from `entry` | `stale` | no | `net <id> stale: <Stale message>` |
| | other `mutate.Refused` | — (a broken net, exit 2) | exit 2 | `error: refused: net <id>: <message>` |
| 2. Compile check (Go, once per package) | `mutate.Stale` | `stale` | no | `net <id> stale: <file> is not compiled by ./<pkg>'s test build` |
| 3. Control (no plant) | `green` | judge its nets (step 4) | — | `control <target>: green (<wall> s)` |
| | `red` | `unjudged` | no | `net <id> unjudged: <target> fails without its plant` |
| | `timeout` | `unjudged` | no | `net <id> unjudged: <target> ran past <timeout> without its plant` |
| | `invalid` | `unjudged` | no | `net <id> unjudged: <target> is invalid without its plant (no run, a skip, a build failure, or a subtest probe not counting one)` |
| 4. Plant run, `expect: caught` | `red` | `caught` | yes | `net <id> caught (<wall> s)` |
| | `green` | `escaped` | no | `net <id> escaped: <target> passes with its plant from <from>` |
| 4. Plant run, `expect: passes` | `green` | `passes` | yes | `net <id> passes (<wall> s)` |
| | `red` | `tolerance failed` | no | `net <id> tolerance failed: <target> fails with its plant from <from>` |
| 4. Plant run, either | `timeout` | `timeout` | no | `net <id> timeout: <target> ran past <timeout> with its plant` |
| | `invalid` | `invalid` | no | `net <id> invalid: <target> is invalid with its plant (a build or import failure, a skip, or no run)` |
| Budget | the next run does not fit | `not run` | no | `net <id> not run: the budget ran out (<left> s left, <need> s needed)` |

`<target>` prints as `./<pkg> <test>[/<subtest>]` or `<py>`. For an `invalid` or `unjudged` line, mutate's `details(outcome, result, notes)` (`:955-957`: the stderr and stdout tails, then the notes) is printed indented under the line, never into `failed:`. A subtest target is judged on that subtest alone: `run_go` probes first and needs exactly one run (`:876-880`), `judge_go` reads the subtest's own actions (`:714-722`), so a sibling's failure is never "caught" — counter-example (5). Python red needs exactly one failure and `Ran 1` (`:741-753`), so an import error is `invalid` — (8). A skip reads `invalid` in both languages (`:715`, `:752`) — (4).

**The run's lines, in order:**

```
nets: <L> live, <R> retired, head <40-hex>, budget <B> s
go: <go version>                                   # mutate.record (:960-967)
python: <version>
control ./internal/runtime TestBreakitActivateRacesRemove: green (4.1 s)
net g2-111 caught (3.9 s)
...
nets: caught <a>, passes <b>, escaped <c>, tolerance failed <d>, unjudged <e>, stale <f>, timeout <g>, invalid <h>, not run <i>; wall <w> s
GATE_COVERED <a + b + c + d + e + f + g + h>
failed: net <id> escaped: ...                      # rows.finish (gate/runner/rows/__init__.py:149-153)
```

Exit 0 only when every net is `caught` or `passes` (`rows.finish` returns 1 on any `failed:` line). `GATE_COVERED` counts the nets judged (every verdict but `not run`); 0 only if every net was not run.

**Verdicts as they come.** Each verdict line is printed and flushed at once and appended to `$GATE_ROW_DIR/verdicts.txt` (opened with `buffering=1`), so a watchdog kill keeps every earlier verdict — counter-example (11). `failed:` lines are printed only by `rows.finish` at the end; a killed run's record shows the scheduler's timeout and its row directory holds `verdicts.txt`.

### N1.5 The loop, the budget and SIGTERM

```python
def main(argv: list[str]) -> int:
    row_dir = rows.start()                    # SIGCHLD default + term.install() (gate/runner/rows/__init__.py:33-37)
    mode, budget_s = _args(argv[1:])          # "--judge --budget <s>" | "--anchors"
    ...
def judge_all(nets: Nets, budget_s: float, out: Callable[[str], None]) -> list[str]:
    deadline = time.monotonic() + budget_s
    head = git rev-parse HEAD
    with tempfile.TemporaryDirectory(prefix="nets-") as scratch, mutate.Worktrees() as trees:
        entries = {net.id: mutate.entry(net.spec, net.timeout)  (Stale → stale verdict)}
        compile check per Go package (N1.1)
        for target, group in targets(entries):            # group = this target's nets, in nets.json order
            need = _need(max timeout of group)            # the control's timeout: the group's largest
            if time.monotonic() + need > deadline: every remaining net "not run"; break
            control = mutate.run_one(dataclasses.replace(group[0], changes=()), ARGS, scratch, trees, head)
            for e in group:
                if control != "green": verdict unjudged; continue
                need = _need(e.timeout)
                if time.monotonic() + need > deadline: e and every later net "not run"; stop both loops
                plant = mutate.run_one(e, ARGS, scratch, trees, head)
                verdict = judge(control, plant, expect)
```

`_need(timeout)` = `mutate.seconds(timeout) + mutate.WALL_EXTRA_S` (120 s, the Go wall clock's own margin, `:895`) + 30 s for the worktree reset and the shim; Python runs need less (`:925`), and the same formula keeps one rule. `ARGS = argparse.Namespace(race=False, cpu=None)`. Every run is sequential: one go or Python test process at a time, one worktree — the bounded exception the owner granted to testing.md rule 2 (decision 5; `docs/development/testing.md:12`). `test_nets.py` pins it: a fake `run_one` counts concurrent calls (at most one) and `Worktrees` constructions (exactly one per run).

**SIGTERM.** `rows.start()` installs `term`'s handler (`gate/term.py:32-44`): the first SIGTERM raises `Terminated` in the main thread; the `with mutate.Worktrees()` block's `__exit__` removes the worktree (`mutate.py:777-778`, `:799-816`); the scheduler's SIGKILL follows only after its grace (`gate/runner/scheduler.py:491-498`). The verdicts written so far stay in stdout and `verdicts.txt`. The budget makes this the rare path: `--budget` is below `timeout_s` (N1.6).

### N1.6 The registry row, `make nets`, the instrument and the proof

```json
{"id": "nets", "job": "nets", "tiers": ["nightly"], "blocking": true, "timeout_s": 3600, "needs": [],
 "scope": ["tools"], "expected_scope": ["git", "gate/nets.json"], "min_wants": 1, "lands_in": "S1.1c-11",
 "exclusive": false,
 "command": ["python3.15", "-I", "-B", "-X", "utf8", "gate/bench/nets.py", "--judge", "--budget", "3300"]}
```

`job: "nets"`, a job of its own (plan check §2 S4); `timeout_s` 3600 is `TIMEOUT_MAX_S` (`gate/runner/registry.py:19`). `--budget 3300` leaves 300 s for setup, the summary lines and cleanup. **Pin** `test_nets.py::Budget.test_the_nets_rows_budget_and_its_largest_need_fit_its_timeout_s`: reads the real row through `registry.load`, asserts `budget + 300 <= timeout_s` and `budget >= _need(largest net timeout)` (a row whose one net cannot fit is red at T2, not at night). `expected_scope` mirrors `lawdelta`'s `["git", "gate/law.json"]`, `min_wants 1` (`gate/registry.json`, row `lawdelta`).

**Makefile** (coordinator): `nets: tools` / `\tpython3 -I -B gate/bootstrap.py exec -- --tier nightly --job nets`, as `check` runs `--tier T2` (`Makefile:19-20`). Rename pull requests run it before merge (decision 2).

**Instrument** in `gate/law.json`: `{"id": "nets", "row": "nets", "lands_in": "S1.1c-11", "code": <the import closure of gate/bench/nets.py>, "proofs": [{"path": "gate/proofs/drivers/test_nets_planted.py", "cases": 5}], "tiers": ["nightly"], "min_wants": 1, "expected_scope": ["git", "gate/nets.json"]}`. The closure is computed, not typed: `gate/runner/tests/test_instruments_pin.py`'s `CodeClosure` names it (`:218-295`); it includes `gate/bench/mutate.py`, `gate/bench/gorun.py`, `gate/runner/jsonstream.py`, `gate/runner/rows/__init__.py`, `gate/runner/stress.py`, `gate/term.py` (mutate's imports, `mutate.py:98-100`) and theirs. Lane L changes `stress.py` in W2, so the coordinator applies the closure at the wave's end (plan check §4).

**Proof driver** `gate/proofs/drivers/test_nets_planted.py` (one world each, a tiny Go package and a `gate/nets.json` of one or two nets written by the driver; `support.registry_row("nets")` with `--budget` lowered in the copied row where a case needs it):

| Case | World | Asserts |
|---|---|---|
| `test_an_escaped_net_is_red_naming_it` | a net whose plant the test does not see | red, reason holds `net n1 escaped: ./p TestX passes with its plant from ...` |
| `test_a_test_red_without_its_plant_leaves_its_nets_unjudged` | the test fails at HEAD | red `net n1 unjudged: ./p TestX fails without its plant`; never `caught` |
| `test_a_stale_net_is_red_and_the_others_still_run` | one find text absent, one live net | red `net n1 stale: --find occurs 0 times ...`; `net n2 caught` in the row's log |
| `test_a_tolerance_net_that_fails_is_red` | `expect: passes`, the test fails with the plant | red `net n1 tolerance failed: ...` |
| `test_the_budget_running_out_names_the_rest_not_run` | `--budget` below the second net's need | red `net n2 not run: the budget ran out ...`; `net n1 caught` kept in `verdicts.txt` |

Each case's wall is measured and budgeted in N5.

---

## N2 — seeding rules and their result

### N2.1 The input

`docs/sprints/s1-1c-7-plants/` (the only plants directory: `ls -d docs/sprints/*-plants`) holds 75 JSON files; `survey-index.json`, `naming-prototype.json` and `commit-map.json` are not plants. The other 72 hold **413 entries: 296 plants (256 find texts, 40 diffs) and 117 controls**, plus **four standalone `.diff` plants** — 417 entries in all. Their designated tests, not in the files, come from the fix commits: `E1-G10-20-revert.diff` and `-2.diff` → `TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained` (aeec09c; `internal/runtime/lifecycle_races_internal_test.go:221`); `E1-G7a-5-revert.diff` → `TestBoundaryRefusalsStateTheirRuleWithNoProcessReference` (4ee80f0; `internal/runtime/boundary_checks_internal_test.go:359`); `PR2Q-11-check-before-lock.diff` → `gate.hooks.tests.test_gitnexus_reindex.CheckUnderLock.test_a_call_parked_in_its_check_and_a_whole_second_call_start_one_run_between_them` (4dc13d9; `gate/hooks/tests/test_gitnexus_reindex.py:358`).

### N2.2 The rules, in order (the first that applies decides)

1. **Control** (`"control": true`, no mutation): folded into the judge. It is never listed in `nets.json`; the judge's control step runs its test whenever a live net names that test. The inflow check (N3) skips controls by rule.
2. **Caught only some of the time** → retired, `reason: "caught only some of the time (<the record's words>): covered by the race or load rows"`. starve-3 (recorded "may be red only some of the time"), both G10-20 race diffs (7 of 50 and 3 of 50 under `--race --repeat 50 --load`, `docs/sprints/s1-1c-7-record.md:303`, `:382`). No batch was recorded under `--race` or `--cpu` otherwise (`git grep -- --race docs/sprints/s1-1c-7-*`), so the schema needs no race, cpu or repeat key.
3. **Names no test** → retired: starve-5 (`TestFullQueueWithABusyRecorderGetsNoTurn` is declared in no `_test.go` file).
4. **Plants into a file the gate edits every sprint** (`gate/current-sprint`, `gate/registry.json`, `gate/law.json`) → retired, `reason: "plants into <file>, a file the gate edits every sprint"`.
5. **The code it plants into was removed** (not moved) → retired, `reason: "plants into code removed at <commit> (<what>)"`. starve-2 ×5 and starve-4 ×2 plant into `q.waiting`, the flag dropped at 86aea25 ([2026-11-22, the starvation fix](decisions.md#owner-decisions-2026-11-22--the-starvation-fix-under-load)); `git grep "waiting\.\(Store\|Load\)" -- internal` is empty.
6. **A duplicate** (the same mutation and the same target as an earlier-kept net) → retired, `reason: "the same plant and test as net <id>"`. starve-1 duplicates starve2-a (the later batch, the current proof).
7. **Does not apply at HEAD** (a `Stale` from `entry`, or the target not declared) → **stale: lane A re-anchors it** (shown caught, or passing for a tolerance plant, by one `mutate.py` single run on the re-anchored plant) **or retires it with a reason**, before `nets.json` is committed.
8. **Otherwise a live net**: `expect: "passes"` when the batch entry's `expect.new` is `"green"` (the tolerance plants), else `"caught"` (`expect.new: "red"`, or a plain batch, where only red counts, `mutate.py:70`). Its target is the new side's (N1.3). Its `timeout` is the entry's (normalised to a duration string), else `"120s"` (mutate's default, `:1008`), until N4.3 replaces it with the measured value.

### N2.3 The throwaway classifier (not committed)

A stdlib Python script, run with `python3.15 -I -B <script> <worktree> <out.json>` from the worktree at 584a8b66, importing `gate.bench.mutate`: for each of the 417 entries it applies N2.2's rules in order, calls `mutate.changes_of(spec)` for the anchor (the `Refused` message kept), checks the Go test by the regex `^func (Test[A-Za-z0-9_]*)\(` over the package directory's `*_test.go` files and a Python id by N3's AST walk, groups duplicates by (file, find, replace, patch, target), and writes one row per entry. It ran in 18.9 s. Lane A re-runs the same rules as `nets.py --seed` is **not** proposed: seeding is once, and N3's inflow check keeps it whole afterwards.

### N2.4 The result, at 584a8b66

| Class | Count |
|---|---|
| Controls, folded into the judge | 117 |
| **Live nets** | **264** — 261 `caught`, 3 `passes`; 148 Go, 116 Python; 26 with a `subtest`; 11 from per-side targets (new side) |
| Retired, with a reason (N2.5) | 16 |
| Stale, for lane A to re-anchor or retire (N2.6) | 20 |
| **Total** | **417** (413 + 4 diffs) |

Anchors at HEAD over the 296 batch plants: 267 apply; 21 find texts occur zero times, 1 twice, 7 diffs refused — **29 no longer apply** (the plan check's 28 at 9b62eedf, plus G2-15, whose find text in `gate/current-sprint` went with the sprint marker, 643423bb). Of the four `.diff` files, `E1-G10-20-revert.diff` does not apply (its `revertUnlessQuieted` is gone, `s1-1c-7-record.md:382`); the other three do.

The live set's distinct targets: **225** (116 Go, among 98 distinct top-level Go tests, and 109 Python ids) — the control runs per night. Timeouts carried: 221 none (→ `120s`), 20 at `180s`, 17 at `300s`, 7 at `120s`. Live nets on files this sprint edits: `gate/bench/mutate.py` 55 (all on `gate.bench.tests.test_mutate`; none touches N1.2's lines), `gate/ci/generate.py` 2 (lane M), `gate/bench/load.py` 2 (lane L), `gate/runner/rows/race.py` 1 (lane C). Lane A seeds against the integration head after W1 and runs `nets-anchors` after each wave's merge-in; the coordinator re-anchors any net a W2 lane stales at the wave's end. 9 controls name a test no live net names (their plants are retired or stale): `test_breakit_s11c2.RuffPinReadsEveryWaiver…`, `test_known_issues_fields.Fields…`, `test_proc.Group…`, `test_lawdelta_planted…`, `TestDrainMarksAStoppedRecordingIncompleteAtItsDeadline`, `TestCviSourceRefusalsAndContexts`, `test_instruments_pin.CodeClosure…` (×2), `TestTestsModeRefusesANonTestFile`; nothing runs them, as nothing is planted on them.

**The five tolerance plants:** G2-47 (`A1-3-G2-47.json#0`, `ruff.toml`) → `passes`; G2-97 (`B-K2b.json#2`, a 25 ms sink slowdown, `TestARecordingThatFallsBehindStops`) → `passes`; G2-151-live (`W0-go.json#4`, `tools/allowlists/purity.json`, `TestListsPurity`) → `passes`; G2-15 (`A2-C7.json#0`, `gate/current-sprint`) → retired by rule 4; g2-117 (`E1-C2.json#4`, a 400 ms drain sleep) → stale (its diff no longer places in `drain.go`): lane A re-anchors it as `passes` or retires it. The QA review ran three of the five tolerance plants with starve-3 at 9b62eedf (`red 1, green 3`, RQ-1); which three it ran is not named, so that the three live `passes` nets are green with their plants is inferred until the first nightly.

**Not finished (deadline):** no `mutate.py` check run was made on the re-anchoring candidates or the `passes` nets; the per-file line numbers `generate.py:152-166` and `summarize.py:196-199` in N6.4 are read from the source but not re-counted (inferred to ±2 lines); the proofs row's per-driver wall report in N5 item 2 is inferred to exist. Lane A confirms each before building.

### N2.5 The retirements (16)

| `from` (under `docs/sprints/s1-1c-7-plants/`) | id | reason |
|---|---|---|
| `E1-starvation-plants.json#10` | starve-3 | caught only some of the time ("may be red only some of the time"): covered by the race or load rows |
| `E1-G10-20-revert.diff` | — | a race plant caught 7 of 50 under `--race --repeat 50 --load` (s1-1c-7-record.md:303): covered by the race or load rows |
| `E1-G10-20-revert-2.diff` | — | a race plant caught 3 of 50 under `--race --repeat 50 --load` (s1-1c-7-record.md:382): covered by the race or load rows |
| `E1-starvation-plants.json#13` | starve-5 | names no test: `TestFullQueueWithABusyRecorderGetsNoTurn` exists only in a design note |
| `A2-C7.json#0` | G2-15 | plants into `gate/current-sprint`, a file the gate edits every sprint |
| `G2-24.json#0` | G2-24 | plants into `gate/law.json`, a file the gate edits every sprint |
| `PR2Q-13.json#0` | PR2Q-13-hooks-row-names-no-py | plants into `gate/registry.json`, a file the gate edits every sprint |
| `PR2Q-13.json#1` | PR2Q-13-ruff-rekeyed-gofmt-twice | plants into `gate/law.json`, a file the gate edits every sprint |
| `E1-starvation-plants.json#0` | starve-1 | the same plant and test as net starve2-a |
| `E1-starvation-plants.json#5`–`#9` | starve-2, -TableWrite, -BatchEnd, -Tick, -late_Send | plants into code removed at 86aea25 (the `waiting` flag dropped) |
| `E1-starvation-plants.json#11`, `#12` | starve-4-TableWrite, starve-4 | plants into code removed at 86aea25 (the `waiting` flag dropped) |

### N2.6 The stale nets for lane A (20)

| `from` | id | why at 584a8b66 | target |
|---|---|---|---|
| `A1-1.json#8` | G10-10 | a hunk of `gate/runner/rows/race.py` places 0 times (lane C edits race.py in W1: re-anchor after W1) | `gate.runner.tests.test_rows_go.Race.test_a_repeat_that_is_not_a_number_is_a_usage_failure` |
| `CD2-C5.json#0` | g10-22-revert | a hunk of `internal/table/table.go` places 0 times | `TestNewRejects` |
| `E1-C2.json#4` | g2-117 | a hunk of `internal/runtime/drain.go` places 0 times; a tolerance plant (`passes`) | `TestDrainMarksAStoppedRecordingIncompleteAtItsDeadline` |
| `E1-drain-plants.json#2`–`#5` | drain-p2-t1, -t2, -t4, -active | find text 0 times in `internal/runtime/drain.go` | four Drain tests (the batch names them) |
| `E2-1.json#0` | G10-1-revert | a hunk of `apphandle.go` places 0 times (also `deploy.go`) | `TestAStartWhoseContextEndsMidwayLeavesTheAppStartable` |
| `W0-1.json#2`–`#5` | W0-1-2 … W0-1-5 | find text 0 times in `gate/bench/mutate.py` | `gate.bench.tests.test_mutate.Patch…`, `PyMode…` |
| `W0-1.json#9` | W0-1-9 | find text **twice** in `gate/bench/mutate.py` | `gate.bench.tests.test_mutate.PyMode.test_a_python_patch_is_applied_in_the_copy` |
| `W0-2.json#0`, `#1`, `#4` | W0-2-0, -1, -4 | find text 0 times in `gate/bench/mutate.py` | `test_mutate.Subtests…` |
| `W0-2.json#2`, `#6`, `#7` | W0-2-2, -6, -7 | a hunk of `gate/bench/mutate.py` places 0 times | `test_mutate.Subtests…` |
| `W0-3.json#1` | W0-3-1 | find text 0 times in `gate/bench/mutate.py` | `test_mutate.Refusals.test_a_bad_entry_timeout_is_refused_before_any_build` |

Every one of their targets is still declared. Each re-anchored net is shown red (or green for `passes`) by one `mutate.py` single run, its line in the record; a plant whose defect can no longer be written is retired, naming the commit that removed it.

---

## N3 — the T2 `nets-anchors` row

`gate/bench/nets.py --anchors`: static, no builds, no worktree, no `go` command. Its checks, each a `failed:` line naming the net:

| # | Check | How | Line |
|---|---|---|---|
| 1 | `nets.json` loads | `load()` (N1.3) | `error: refused: nets.json: <message>` (exit 2) |
| 2 | each net's files are tracked | one `git ls-files -z` → a set; for a patch, every path `mutate.parse_patch` names | `net <id> stale: <file> is not tracked` |
| 3 | the find text occurs exactly once, or every hunk places | the working tree's text (UTF-8, `surrogateescape`, as `base_text` reads it, `mutate.py:493-494`); `mutate.occurrences(text, find) == 1`; for a patch, `mutate.apply(path, text, hunks)` per path, catching `mutate.Stale` | `net <id> stale: --find occurs <n> times in <file>, not exactly once` / `net <id> stale: a hunk of <file> occurs <n> times, not exactly once` |
| 4 | a Go test is declared | every `*_test.go` directly in the package directory (`pkg`, else the first file's directory, as `entry` does, `:607-610`); the regex `^func (Test[A-Za-z0-9_]*)\(` per line (`re.M`), a top-level function only, since a method has a receiver before its name | `net <id> stale: <test> is declared in no _test.go file of ./<pkg>` |
| 5 | a Python id resolves | split at `.`; the longest prefix that is a file `<a>/<b>/<c>.py` is the module; the rest is `Class` or `Class.method`; `ast.parse` the module; the class is a top-level `ClassDef`; the method is a `FunctionDef` in it or in a base class named by a plain `Name` and defined in the same module (followed transitively) | `net <id> stale: <py> does not resolve` |
| 6 | the inflow | every entry with a mutation in every `docs/sprints/*-plants/*.json` (skipping files whose top level is not a list, which takes out `survey-index.json`, `naming-prototype.json` and `commit-map.json`; verified: all 72 plant batches are lists), and every `docs/sprints/*-plants/*.diff`, is one net's `from` or one retirement's `from` | `plants <from>: neither a net nor retired` |
| 7 | every `from` names an existing entry | the batch's index exists, or the `.diff` file exists | `net <id>: from <from> names no plant` |
| 8 | a `from` pointing at a control | refused | `net <id>: from <from> is a control, not a plant` |

What it cannot see, by design: whether a plant still compiles, whether a test still catches it, a build-tag-excluded test file (`//go:build`) declaring the test (inferred: rare; the nightly's control reads it `invalid`). Those are the nightly's.

**Measured on the real set** (584a8b66, the owner's machine, 14 CPUs): checks 2–3 over all 300 plants through `parse_patch`, `apply` and the occurrence count on the working tree: **0.05 s**, plus 0.29 s to import `mutate`; the same through `mutate.changes_of` took 18.9 s (three `git` processes per file, `base_text` `:487-492`) — hence §7 D1. Checks 4–6 are file reads and `ast.parse`: inferred well under a second. A **pin** in `test_nets.py` runs the real `--anchors` over the committed `nets.json` and asserts it exits 0 within 10 s.

**Registry row:**

```json
{"id": "nets-anchors", "job": "lint", "tiers": ["T2", "T3", "T4"], "blocking": true, "timeout_s": 60, "needs": [],
 "scope": ["tools"], "expected_scope": ["git", "gate/nets.json"], "min_wants": 1, "lands_in": "S1.1c-11",
 "exclusive": false,
 "command": ["python3.15", "-I", "-B", "-X", "utf8", "gate/bench/nets.py", "--anchors"]}
```

It prints `nets-anchors: <L> nets, <R> retired, <P> plants, <F> stale; <w> s`, then `GATE_COVERED <L>`, then `failed:` lines. **Instrument:** `{"id": "nets-anchors", "row": "nets-anchors", "lands_in": "S1.1c-11", "code": <closure>, "proofs": [{"path": "gate/proofs/drivers/test_nets_anchors_planted.py", "cases": 4}], "tiers": ["T2", "T3", "T4"], "min_wants": 1, "expected_scope": ["git", "gate/nets.json"]}`.

**Proof driver** `test_nets_anchors_planted.py` — counter-examples (13)–(17):

| Case | Plant in the world | Asserts |
|---|---|---|
| `test_an_edit_under_a_nets_find_text_is_red_at_t2` | the world's commit edits the net's find text | red `net n1 stale: --find occurs 0 times in ...` |
| `test_a_renamed_test_is_red` | the world renames `TestX` | red `net n1 stale: TestX is declared in no _test.go file of ./p` |
| `test_a_plant_neither_a_net_nor_retired_is_red` | a plants batch entry added (or a net deleted without a retirement) | red `plants <from>: neither a net nor retired` |
| `test_a_retirement_without_a_reason_is_refused` | `"reason": ""` | red, `error: refused: nets.json: retired <from>: no reason` |

Unit tests in `test_nets.py` cover the rest: a replace-only identifier (a rename of a name only the `replace` text holds never stales the net: the anchor is the find text), a patch net's file renamed, two nets with one id, a `subtest` with `py`.

---

## N4 — budgets

### N4.1 The method, for every new nightly row

1. Each new row lands with a provisional go `-timeout` and `timeout_s` from the owner's machine.
2. Three dispatched nightlies on `s1-1c-11/integration` (the burn-in, decision 6) record each row's wall time and, for go rows, go's own per-package time from `test.json`. The record lists them per run id.
3. The go `-timeout` is **at least three times the slowest of the CI runs**, rounded up to a whole minute; `timeout_s` must hold `-timeout + 60 s` (cmd/go's kill margin) `+ 120 s` (build and report headroom) and stay `<= 3600` (`gate/runner/registry.py:19`). A row that cannot fit is split, never given a bigger cap.
4. Each row gets a **pin** in the shape of `RaceStressBudget` (`gate/runner/tests/test_rows_go.py:212-226`): the real row read through `registry.load`, `timeout_min * 60 + GO_KILL_MARGIN_S + STRESS_HEADROOM_S <= row.timeout_s` — counter-example (18). Race-stress per package moves from the module constant `STRESS_TIMEOUT_MIN` (`gate/runner/rows/race.py:25`) to a per-row `--timeout=<m>m` argument parsed beside `--repeat=` (`:33-39`), the pin reading it from the row's command (lane C).

| Row (lane) | Measured on the owner's machine (inferred until the burn-in) | Rule |
|---|---|---|
| race-stress per package: `internal/playback`, `internal/partition`, root, `io/inmemory` (C) | per package, by lane C | the method; one row each, `job` of its own |
| `-cpu 1,4` over `internal/...` (C) | about twice `unit`'s go time (two passes) | the method; the row passes `-timeout` explicitly (today `unit` runs go's default 10 m per package) |
| load (L) | `TestLiveEqualsReplay` took about 78 s a run under 28 hogs (`s1-1c-7-record.md:304-308`) | its count small (plan check §5 N4); `load.py --wall` set explicitly, `<= timeout_s - 120` (its default 3600 equals `TIMEOUT_MAX_S`, `gate/bench/load.py:106`) |
| stress (L) | per listed Python id | `stress.py`'s run count per id; the row's `timeout_s` from the burn-in |
| nets (A) | N4.3 | N1.6's `--budget` pin |
| the macOS test job (M) | the T4 test job's rows on darwin | its rows keep their `timeout_s`; the job's cap is the generator's sum (`gate/ci/generate.py:219-230`) |

### N4.2 `load.py`'s wall

Every load row's command passes `--wall <s>` with `s <= timeout_s - 120`; a pin asserts it per load row (lane L), so `load.py` stops its hogs and reports before the scheduler's SIGTERM.

### N4.3 The nets row's per-net and per-control timeouts

The row prints each run's wall (`(<wall> s)` on every `control` and `net` line). After the three burn-in nightlies, lane A sets each net's `timeout` to **max(60 s, 3 × the slowest of its three plant walls)**, rounded up to 30 s, and a control's timeout is the largest of its nets' (N1.5). A net whose slowest wall is above 600 s is reported to the coordinator before its timeout is set (G2-92, `TestLiveEqualsReplay`, carries 300 s today). The sum of all `_need` values is printed as `need <s> s` on the first line beside the budget; the plan check's local extrapolation is 10–15 minutes for the whole set (inferred), well inside 3300 s.

---

## N5 — PR CI

Every proof driver runs in the T4 `proofs` row (`gate/registry.json`, row `proofs`: job `proofs`, 600 s, `exclusive: true`), drivers in parallel one per core (`gate/runner/rows/__init__.py:130-146`). Decision 4: the small growth is accepted, each piece measured, no CPU hogs in the proofs world.

1. **Before.** Lane C's first commit records the `proofs` row's wall on the integration branch's PR CI run (the record's "before" line, the run id).
2. **Per driver budget.** Each new driver case: at most 30 s on the owner's machine, asserted by nothing but measured once per PR from the row's per-driver report and listed in the record. The nets driver's worlds hold one tiny Go package and at most two nets, so a case is two or three `go test` runs.
3. **No hogs.** `load.py` drivers pass `--hogs 0` (`gate/bench/load.py:104`, `:114` accepts 0); `stress.py` drivers pass `--no-hogs` (`gate/runner/stress.py:263`). `stress.hog_count()` is twice the CPU count (`gate/runner/stress.py:60-61`); none starts inside the proofs world.
4. **Deterministic once-in-N plants.** The load and stress drivers' planted failures read a counter file in the world's temporary directory: run k increments it and fails exactly when the count reaches N (e.g. the 3rd of 5), so the row's k/N verdict is the same on every machine — counter-example (21). Never a random draw.
5. **After.** Each PR's last commit records the `proofs` row's wall again; exit criterion 9 compares the two. The T2 `nets-anchors` row adds under a second (N3).

---

## N6 — the macOS test job (S3) and the concurrency group (S4)

### N6.1 `gate/ci/generate.py` (lane M)

```python
# A job's runner when it is not ubuntu-latest, by (tier, job): the determinism proof's second OS at T4 (S3.2b's item
# 8; owner Q5: macOS) and the test job at the nightly tier (owner, 2026-11-27, S1.1c-11's plan check, decision 3).
RUNNERS: dict[tuple[str, str], str] = {
    ("T4", "determinism-macos"): "macos-latest",
    ("nightly", "test"): "macos-latest",
}


def runner(tier: str, job: str) -> str:
    return RUNNERS.get((tier, job), "ubuntu-latest")
```

`_job` (`gate/ci/generate.py:124-150`) prints `f"    runs-on: {runner(tier, job)}"` in place of `RUNNERS.get(job, 'ubuntu-latest')` (`:138`); `_job` already takes `tier`. The canary renders the T4 jobs with `TIER` (`render_canary`, `_jobs(chosen, TIER, canary=True)`), so its `test` job stays on ubuntu and its `determinism-macos` on macOS, as today. `RUNNERS` has no other reader (`git grep RUNNERS`: `:44`, `:138`).

**The concurrency group** (`render`, `:205-206`): `["  group: nightly-${{ github.ref }}", "  cancel-in-progress: false"]`. A dispatched branch run then neither waits for nor replaces main's 03:00 run (RA-9).

### N6.2 Registry and law tiers (coordinator)

All nine `test` job rows gain `"nightly"` at the end of `tiers`: `unit`, `coverage-inclusion`, `pytests`, `semantics`, `conformance`, `baseline` (`["T2", "T3", "T4", "nightly"]`) and `differential`, `notbuilt`, `tripwire` (`["T4", "nightly"]`). Each row's instrument in `gate/law.json` repeats the same `tiers` (`teeth.pin` compares `tiers`, `gate/runner/teeth.py:484`, `:521-524`). Mixed PR and nightly tiers exist today (`vulns`: `["T4", "nightly"]`). Records stay `(test, <row>)`, so the summary, the instruments' `row`, and the drivers are unchanged. The nightly `test` job's cap follows the generator's rule: the nine `timeout_s` sum to 2,640 s, so `max(10, ceil((2640 × 1.2 + 60) / 60) + 10)` = 64 minutes (`:221-226`).

### N6.3 `gate/ci/tests/test_generate.py` cases (red first) — counter-examples (23), (24)

1. `test_the_nightly_test_job_runs_on_macos_and_the_t4_test_job_on_linux` — render the real registry at `nightly` and at `T4`; the `test:` job's `runs-on` is `macos-latest` and `ubuntu-latest` respectively, with the same nine rows selected (the nightly run line holds `--tier nightly --job test`).
2. `test_runners_are_keyed_by_tier_and_job` — a registry with a row of job `j` at T4 and nightly and `RUNNERS` patched with `("nightly", "j")`: only the nightly workflow's `j` runs on macOS.
3. The existing `test_the_macos_job_runs_on_macos_and_every_other_on_linux` (`:129`) is rewritten for the (tier, job) key, every job but `determinism-macos` on ubuntu at T4.
4. `test_the_nightly_concurrency_group_carries_the_ref` — the nightly's `concurrency:` block is `group: nightly-${{ github.ref }}`, `cancel-in-progress: false`; the gate's block unchanged.

### N6.4 A missing macOS record

The summary's expected set is each selected row's `(job, id)` plus each job's parity row (`gate/runner/summarize.py:176-180`). When the macOS job uploads nothing (a runner that never started, a cancelled job), its download step is `continue-on-error: true` (`generate.py:152-166`) and leaves no directory; every `(test, <row>)` and `(test, parity)` then reads `FAIL`, `missing: no job recorded it` (`summarize.py:196-199`), and strict-gate is red. A record from another attempt reads `stale: only attempt <n> ran it` (`:197-199`). **Test** in the summary's tests (lane M, with the coordinator's leave, since `summarize.py` is not changed): a nightly registry with the test rows and no `records-test-*` directory → each test row `FAIL missing`.

### N6.5 The one-time darwin demonstration (recorded, never committed)

1. Lane M, after its generator change is on `s1-1c-11/integration`: a scratch branch `s1-1c-11/darwin-demo` off it, one commit adding `internal/playback/darwin_demo_test.go`: `func TestDarwinDemo(t *testing.T) { if runtime.GOOS == "darwin" { t.Fatal("planted: fails on darwin only") } }`.
2. The coordinator pushes the scratch branch (ground rule 18: agents never push) and runs `gh workflow run nightly.yml --ref s1-1c-11/darwin-demo`.
3. Expected: the nightly `test` job red, `unit` naming `TestDarwinDemo`; every ubuntu job green. The run id, its URL and the `unit` line go into the record.
4. The scratch branch is deleted, local and remote; the test file never reaches the integration branch. (The owner's machine is darwin, so the plant would turn local T2 red: it is never run locally.)

---

## 7. Deviations from the plan check, and why

- **D1 — the anchor check reads the working tree, not `changes_of`.** The plan check (§2 S2, §3) names `mutate.changes_of` for "find once or hunks place". Measured on the set: 18.9 s through `changes_of` (git processes per file) against 0.05 s through the same pure parts it calls on the working tree; `changes_of` also refuses a file that differs from HEAD (`mutate.py:491-492`), so a developer's uncommitted edit would read as a refusal instead of as the stale net it makes. To call the same occurrence count without importing a private name, `_occurrences` becomes `occurrences` — a second, mechanical change to `mutate.py` beside the typed refusals (`impact`: one caller). The nightly still goes through `entry` → `changes_of`, so both halves share one rule.
- **D2 — a control that is `invalid` reads `unjudged`, not `stale`.** The plan check's table splits "selects no test" (stale) from "fails" (unjudged); mutate's `invalid` covers no run, a skip, a build failure and a probe miscount (`mutate.py:62-70`) and the judge must not parse output. A test that is declared but selects nothing is the static half's `stale` at T2 (N3 check 4); at night every non-green control is `unjudged`, with mutate's tails printed under it.
- **D3 — seven more retirements than the plan check names.** Rule 5 (code removed at 86aea25: starve-2 ×5, starve-4 ×2) and rule 6 (starve-1, a duplicate). Both follow from evidence (`git grep` on `waiting`; identical plants), not from preference; lane A may still re-anchor any of them if the reviewers disagree.
- **D4 — 29 plants no longer apply, not 28.** G2-15's find text left `gate/current-sprint` with this sprint's marker (643423bb); it is retired by rule 4 either way.
- **D5 — the inflow check skips controls by rule** rather than listing 117 retirements: a control has no plant, and decision 2 speaks of plants.

## 8. Open questions for the owner

- **Q1 — The seven plants into the removed `waiting` flag (D3).** **(a, recommended) Retire them, naming 86aea25:** their defect can no longer be written, and the same test keeps four live plants (starve2-a…d). (b) Lane A writes new plants for the same test in their place: more nets on one test, no new defect class.
- **Q2 — `_occurrences` made public (D1).** **(a, recommended) Rename it to `occurrences`:** one rule for both halves, the T2 check under a second. (b) Keep mutate.py to the typed refusals and let the static half call `changes_of`: about 19 s at T2 on every PR, and a dirty file reads as a refusal.

*Nothing else in this note needs an owner's word; every other choice follows the plan check v2 and decisions 2–5.*
