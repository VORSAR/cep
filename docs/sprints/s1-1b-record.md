# S1.1b — sprint record (slim)

**Sprint:** S1.1b — the analyzers engine code needs · **Routine:** slim, by the owner's decision (2026-11-04): one plan here, no reviews · **Size:** about 5 SP · **Branch:** `s1-1b/integration`

## Plan and decisions

The owner cut S1.1b to what engine code needs from its first line, and moved the rest to S1.1c ([decisions](decisions.md#owner-decisions-2026-11-04--s11b-slimmed-toward-engine-code)). The plan:
- **One Go program, standard library only:** `tools/archcheck`, reading each file's syntax with `go/parser`. No `golang.org/x/tools` dependency. It deviates from ADR-0015 §2.2's "every instrument is a `go/analysis` analyzer", and the deviation is noted here.
- **One gate row,** `archcheck` (lint, T0–T4), over every first-party `.go` file outside `gate/` and `tools/`. At T0 it sees only the staged files (the S1.1a-2 runner).
- **The import law** is `tools/importcheck/rules.json`: one flat, exhaustive list per package of the module packages it may import, from ARCHITECTURE §3 (rules 1–8 and the exception table). For packages §3 does not list, the list is §2a's layer bound. A package missing from the file is red.
- **The four checks:** imports; the clock (ADR-0015 §2.2's call family outside `internal/runtime`, `io`, `internal/playback`, `cmd`; `time` in operator packages except `internal/window/cron.go`); purity (non-test files: goroutines and channels, `sync`, `os`/`net`/`io`, `unsafe`, `//go:linkname`); globals (non-test files: error sentinels, compiled regexps, never-written lookup tables, blank vars).

## Measured results

- `go test ./tools/archcheck`: 4 tests with 27 cases, each rule red on its planted defect and quiet on its control.
- `make check` (T2): every row PASS. `archcheck` covered 6 files: S1.1a's code scanned retroactively and green.
- **A flaky test found and fixed at its root:** `test_stop_children` failed once under the full suite's load. A TERM landing inside `subprocess.Popen`'s own start-up left a forked child that nobody held. `gate/term.py` gained `popen`, which kills and reaps that child before the exception goes on; a test forces the exact interruption. The stress harness then ran the module 20/20 under CPU hogs.

## Done-done

Slim routine (owner, 2026-11-04): plan above · build, test-first · T2 green · CI green on the PR · owner merges.

## Retro

*(at close-out)*
