# Development Guide — complex-events-engine

The toolchain, the gate, and the code rules every change follows. The process that runs them is [00-sprint-model.md](../sprints/00-sprint-model.md); the test rules are [testing.md](testing.md).

> **Not built yet (2026-10-31).** No Go module, gate, hooks or CI exist. This guide is the contract they are built to. The gate is an enabler: it lands before the first feature that needs it ([sprint model §16](../sprints/00-sprint-model.md#16-sequencing)). Command names below are the intended ones; the sprint that builds them updates this file to what exists, and the "proposed" tool choices are settled at that sprint's plan check. *(Annotated 2026-11-02: settled at S1.1a's plan check. The pins are Go 1.27.3, golangci-lint v2.15.0, CPython 3.15.2 and ruff 0.17.3. The gate's own tooling is stdlib Python, never shell (owner decision); see [sprint-plan §5](../sprints/sprint-plan.md#5-decisions).)* *(Annotated 2026-11-03, S1.1a-1: built now are the Go module, `tools.json` with parity, the gate runner (`gate/run.py`, rows in `gate/registry.json`) and the T2–T4 targets `make check`, `make combine-check` and `make strict-gate`. Not yet built: the hooks and CI (S1.1a-2), the LAW Δ block and the analyzers (S1.1b).)* *(Annotated 2026-11-04, S1.1a-2: the hooks and CI are built. Run `make setup` once per clone: it turns the hooks on, and `make check` refuses until you have. The pre-commit and pre-merge-commit hooks refuse a staged file that also has unstaged changes, then run T0 in place on the staged files: parity, `gofmt`, `ruff`. The pre-push hook runs T1 on a clean HEAD and leaves any other push to CI. CI (`.github/workflows/gate.yml`) is generated from the registry by `make ci-workflow`; its `strict-gate` job is the one check to require. The hooks are a convenience; CI is the net.)*

---

## Toolchain — pinned in one place

- **One manifest pins every tool** the gate depends on: the Go version (`go.mod`'s `go` and `toolchain` lines) and each gate tool at an exact version, with its checksum. One install script serves the owner's machine and CI, so both run the same binaries.
- **Parity runs first, always.** The gate's first step compares the tools it will run against the manifest. A mismatch **fails**, locally and in CI, before anything else runs: a verdict from the wrong tools means nothing.
- **No global installs.** Tools install into a cache keyed by the manifest's hash; the gate puts that directory first on `PATH` itself. *(Annotated 2026-11-03, as built: each tool has its own directory, `<name>-<version>-<archive_sha256[:12]>`, under `~/.cache/vorsar/cep/tools`, or under `VORSAR_CEP_TOOLS_CACHE` when set.)*
- **A bump is one commit**, `toolchain: bump X a→b`, that changes the manifest **and** fixes what the new version finds. A checksum is never edited to make it pass.

---

## The gate

> The gate's full mechanism catalog — architecture analyzers with planted-failure proofs, suppression liveness, freshness checks, hostility rules — is [ADR-0015](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md); the assurance strategy is [qa-strategy.md](qa-strategy.md).

The gate is a **registry**, not a script: every check is one row. **The row schema is stated once, normatively, in [ADR-0015 §1.1](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)** (id, job, tiers, blocking, timeout, needs, scope, expected_scope, min_wants, lands_in, exclusive, command). Every local tier, every `make` target and every CI job runs the same rows, so the two cannot diverge. The summary opens with the **LAW Δ block** (ADR-0015 §1.3): every change to a gate/spec/architecture law file in the diff, printed where the owner reads at merge. *(Annotated 2026-11-03: the LAW Δ block lands in S1.1b.)*

**Parallel by default** (owner, 2026-11-02): independent rows run at the same time, each starting as soon as the rows it `needs` finish; only rows marked `exclusive` (benchmarks, latency, stress under load) run alone. Go builds and tests run packages in parallel and share one cache; CI runs its jobs concurrently. The summary stays in registry order, so a parallel run reads exactly like a serial one ([ADR-0015 §1.1](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)).

**The four rules the gate enforces:**
1. **Collect-all.** A failing step never stops the run. Every step runs to completion; a skip is recorded with its reason, never as green.
2. **Every step has a ceiling.** Each step runs under a watchdog that kills its whole process group and names the timeout.
3. **Zero is a failure.** A step that covers zero files or zero tests fails. (`go test -run` with a misspelt pattern runs nothing and exits 0; the gate turns that into a failure.) *(Annotated 2026-11-02, owner decision: one named exception. At T0 only, a check whose staged scope is empty, such as the Go format check on a docs-only commit, records SKIP with its reason and does not fail the commit. At T1–T4, zero is still a failure.)*
4. **The exit code is the verdict**, and the summary is the table. The run prints `job | step | verdict | time | reason`, then `GATE PASSED` or `GATE FAILED`, then `exit: <code>`. *(Annotated 2026-11-03, S1.1a-1 as built: after the table come `registry: <path>` (only for a registry other than the repository's), `run: <dir>` (each row's own output is in `<dir>/rows/<job>/<id>/`) and the footer `wall … · summed … · saved …%`, then the two closing lines. A red row's reason carries its first failure line. Read the `exit:` line, not `make`'s status: `make` reports any failing recipe as 2, so its status cannot tell "gate failed" (1) from "internal error" (2) (review BA11).)*

| Tier | Command | When | Covers at least *(intent sketch — **the registry is the tier map**; per-instrument tiers are ADR-0015 §2's table, which wins on any difference)* |
|------|---------|------|------------------------------|
| T0 | pre-commit hook | each `git commit` | parity first (it runs first at **every** tier), then staged files: format (auto-fix, exact-path re-stage), the landed T0 analyzers (ADR-0015 §3.1) *(annotated 2026-11-02, owner decision: format **refuses** an unformatted staged file and prints the fix command; it never auto-fixes or re-stages)* |
| T1 | pre-push hook | each push | format check, lint, suppressions, posture guards |
| T2 | `make check` | a lane says "done" | T1 + build, `go vet`, the linter, unit tests, the conformance suite, the coverage floor |
| T3 | `make combine-check` | before pushing an integration branch | T2 + the race detector, deterministic-replay checks, the slow suites, the demo scenario |
| T4 | `make strict-gate` == `make ci` == CI | each PR, main, each tag | everything, incl. the grammar differential, the planted proofs, the vulnerability scan and the dependency licence check (Apache-2-compatible only) |

**Reading a run.** CI shows the same table as the summary of one **`strict-gate`** check, which is the one check to read. A job that did not report (cancelled, timed out, crashed) is a failure, never an absence of failures. A run on `main` is never cancelled.

**Hooks** live in version control (`.githooks/`), activated once per clone with `git config core.hooksPath .githooks`. A missing required tool blocks the commit. *(Annotated 2026-11-02, owner decision: `make setup` turns the hooks on, and `make check` refuses to run until they are on. CI turns them on too, so the rows stay identical. Lands in S1.1a-2.)* `--no-verify` is not used — that is policy, not mechanism: the hook is technically bypassable, which is why it is a convenience and the real net is CI + the owner's merge (ADR-0015 §3.1; the sprint model's "a hook is never bypassed" states the same policy).

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
//nolint:gosec // PERMANENT reason="fixture generator; output is never security-relevant" ref=ADR-0015#2 sprint=S1
//nolint:errcheck // TEMP reason="close error surfaced by S4's sink rework" issue=#42 sprint=S4
//nolint:revive // TEMP reason="exported name kept for one release" expiry=2027-03-01 sprint=S5
```

Rules: the linter name is mandatory (no bare `//nolint`; golangci's `nolintlint` with require-specific + require-explanation is the first net); exactly one of PERMANENT or TEMP; a `reason="…"` of at least 12 characters; **PERMANENT carries `ref=<ADR-nnnn or docs path#anchor>` resolving to a recorded decision — the checker verifies the target exists** (a permanent exception must trace to a decision, never to convenience); TEMP carries `expiry=YYYY-MM-DD` (fails once past) or `issue=<#id|url>`; every suppression carries `sprint=S<n>` (the sprint that introduced it); a suppression for a linter that is not enabled is dead and fails. The gate summary prints `suppressions: N (+Δ this diff)`. The same grammar governs every allowlist file (ADR-0015 §3.10). Fix the code first; suppress only a genuine, documented necessity, never to quiet the gate.

## TODO policy

A `TODO` carries an **owner or a sprint or issue id**: `// TODO(S3): …`, `// TODO(ramith): …`, `// TODO(#42): …`. A bare `// TODO` is not allowed. The same rule covers the synonym family `FIXME`/`HACK`/`XXX`; a named sprint must be ≤ current + 2, and a marker naming a closed sprint fails T4 (ADR-0015 §2 `todocheck`).

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

**Last updated:** 2026-11-01 (aligned with ADR-0015's revision after the foundation adversarial review)
