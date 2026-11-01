# Development Guide — complex-events-engine

The toolchain, the gate, and the code rules every change follows. The process that runs them is [00-sprint-model.md](../sprints/00-sprint-model.md); the test rules are [testing.md](testing.md).

> **Not built yet (2026-10-31).** No Go module, gate, hooks or CI exist. This guide is the contract they are built to. The gate is an enabler: it lands before the first feature that needs it ([sprint model §16](../sprints/00-sprint-model.md#16-sequencing)). Command names below are the intended ones; the sprint that builds them updates this file to what exists, and the "proposed" tool choices are settled at that sprint's plan check.

---

## Toolchain — pinned in one place

- **One manifest pins every tool** the gate depends on: the Go version (`go.mod`'s `go` and `toolchain` lines) and each gate tool at an exact version, with its checksum. One install script serves the owner's machine and CI, so both run the same binaries.
- **Parity runs first, always.** The gate's first step compares the tools it will run against the manifest. A mismatch **fails**, locally and in CI, before anything else runs: a verdict from the wrong tools means nothing.
- **No global installs.** Tools install into a cache keyed by the manifest's hash; the gate puts that directory first on `PATH` itself.
- **A bump is one commit**, `toolchain: bump X a→b`, that changes the manifest **and** fixes what the new version finds. A checksum is never edited to make it pass.

---

## The gate

The gate is a **registry**, not a script: every check is one row (id, CI job, tiers, blocking or advisory, timeout, coverage rule, command). Every local tier, every `make` target and every CI job runs the same rows, so the two cannot diverge.

**The four rules the gate enforces:**
1. **Collect-all.** A failing step never stops the run. Every step runs to completion; a skip is recorded with its reason, never as green.
2. **Every step has a ceiling.** Each step runs under a watchdog that kills its whole process group and names the timeout.
3. **Zero is a failure.** A step that covers zero files or zero tests fails. (`go test -run` with a misspelt pattern runs nothing and exits 0; the gate turns that into a failure.)
4. **The exit code is the verdict**, and the summary is the table. The run prints `job | step | verdict | time | reason`, then `GATE PASSED` or `GATE FAILED`, then `exit: <code>`.

| Tier | Command | When | Covers at least *(proposed)* |
|------|---------|------|------------------------------|
| T0 | pre-commit hook | each `git commit` | staged files: format (auto-fix), fast lint |
| T1 | pre-push hook | each push | parity; format check, lint, suppressions, posture guards |
| T2 | `make check` | a lane says "done" | T1 + build, `go vet`, the linter, unit tests, the conformance suite |
| T3 | `make combine-check` | before pushing an integration branch | T2 + the race detector, deterministic-replay checks, the slow suites, the demo scenario |
| T4 | `make strict-gate` == `make ci` == CI | each PR, main, each tag | everything, incl. the vulnerability scan and the dependency licence check (Apache-2-compatible only) |

**Reading a run.** CI shows the same table as the summary of one **`strict-gate`** check, which is the one check to read. A job that did not report (cancelled, timed out, crashed) is a failure, never an absence of failures. A run on `main` is never cancelled.

**Hooks** live in version control (`.githooks/`), activated once per clone with `git config core.hooksPath .githooks`. A missing required tool blocks the commit. `--no-verify` is not used: the hook is a convenience, the real net is T4.

---

## Code rules

These follow from decided objectives; each names its source.

- **No package-level mutable state.** The engine is an embeddable library: two runtimes in one process never share state. Configuration and dependencies arrive through constructors and `context.Context` (O3: "contexts, no globals").
- **Engine time comes from the engine's clock, never the wall clock.** Event time is read from events; processing time is read through the runtime's clock, which playback mode replaces. A direct `time.Now()` in engine logic breaks deterministic replay (O1).
- **The hot path never blocks** on I/O, a network call or a model call. LLM judgment is asynchronous and optional; every capability works with no provider configured (decision: CEP narrows, LLM judges).
- **A rule lives in one pure package** (no I/O, no goroutines it does not own). Adapters (connectors, the CLI, the binary runner) stay thin and are tested against the real package, never a mirror of it.
- **Deploy-time errors carry line and column**; an unsupported construct is rejected at deploy time, never at match time (O2).
- **gofmt is not optional**: unformatted code fails T0.

*Go idiom beyond these (naming, error wrapping, generics, iterators) follows the linter's configuration, settled when the gate is built.*

---

## Suppression policy

Every lint suppression in first-party code is **accountable**, checked by the gate. The metadata rides on the same line as the directive:

```go
//nolint:gosec // PERMANENT reason="fixture generator; output is never security-relevant"
//nolint:errcheck // TEMP reason="close error surfaced by S4's sink rework" issue=#42
//nolint:revive // TEMP reason="exported name kept for one release" expiry=2027-03-01
```

Rules: the linter name is mandatory (no bare `//nolint`); exactly one of PERMANENT or TEMP; a `reason="…"` of at least 12 characters; TEMP carries `expiry=YYYY-MM-DD` (fails once past) or `issue=<#id|url>`; a suppression for a linter that is not enabled is dead and fails. Fix the code first; suppress only a genuine, documented necessity, never to quiet the gate.

## TODO policy

A `TODO` carries an **owner or a sprint or issue id**: `// TODO(S3): …`, `// TODO(ramith): …`, `// TODO(#42): …`. A bare `// TODO` is not allowed.

---

## Code review checklist (before commit)

- [ ] **Gate tier green** for the work's size (T2 for a lane), read from the `exit:` line
- [ ] **No package-level mutable state**; dependencies through constructors and `context.Context`
- [ ] **No wall-clock read in engine logic**; time through the runtime's clock
- [ ] **Hot path does not block** on I/O, network or a model
- [ ] **Each new rule in its owning package**, tested there; adapters stay thin
- [ ] **Conformance cases** for every construct added or changed; the counter-examples from the plan check are among them
- [ ] **Tests follow [testing.md](testing.md)**: no sleeps, own world, a ceiling
- [ ] **Comments explain why, not what**
- [ ] **No Claude attribution** in code, comments, commits or PR text

---

**Last updated:** 2026-10-31
