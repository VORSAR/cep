# Development Guide — complex-events-engine

The toolchain, the gate, and the code rules every change follows. The process that runs them is [00-sprint-model.md](../sprints/00-sprint-model.md); the test rules are [testing.md](testing.md).

> **What exists (2026-11-06).** The Go module; `tools.json` (Go 1.27.3, golangci-lint v2.15.0, CPython 3.15.2 and 3.12.16, ruff 0.17.3, lark 1.3.1) with parity; the gate runner (`gate/run.py`, its rows in `gate/registry.json`, which is the tier map); `make check`, `make combine-check` and `make strict-gate`; the hooks; CI (`.github/workflows/gate.yml`, generated from the registry by `make ci-workflow`, and `nightly.yml`); the `doclinks` row, which fails on a relative link or `#anchor` in the docs that does not resolve ([owner, 2026-11-06](../sprints/decisions.md#owner-decisions-2026-11-06--the-docs-restructure)). The gate's own tooling is stdlib Python, never shell ([owner, S1.1a's plan check](../sprints/decisions.md#owner-decisions-2026-11-02--s11a-plan-check-g0)). **Built in S1.1c-1 (2026-11-11):** the LAW Δ block — `gate/law.json` names every law path, the `lawdelta` row diffs them against the merge base with `main`, and the block prints first in every summary, local and CI; the current-sprint marker (`gate/current-sprint`, set by the sprint branch's first commit) with the plan's row order in `gate/sprints.json` and the `notbuilt` row (T4) that turns red when a corpus file listed in `conformance/notbuilt.json` is still not built once its sprint is reached; the `deps` row (allowlist, licence by fingerprint, one module). **Not built yet:** the suppressions checker, `todocheck` and golangci-lint's `smells` (S1.1c-2). Run `make setup` once per clone: it turns the hooks on, and `make check` refuses until you have. The pre-commit and pre-merge-commit hooks refuse a staged file that also has unstaged changes, then run T0 in place on the staged files: parity, `gofmt`, `ruff`. The pre-push hook runs T1 on a clean HEAD and leaves any other push to CI. The hooks are a convenience; CI is the net. ~~CI's `strict-gate` job is the one check to require~~ `strict-gate` is the one CI check to read; no status check is required on `main` ([owner, 2026-11-05](../sprints/decisions.md#owner-decisions-2026-11-05--branch-protection)).

---

## Toolchain — pinned in one place

- **One manifest pins every tool** the gate depends on: the Go version (`go.mod`'s `go` and `toolchain` lines) and each gate tool at an exact version, with its checksum. One install script serves the owner's machine and CI, so both run the same binaries.
- **Parity runs first, always.** The gate's first step compares the tools it will run against the manifest. A mismatch **fails**, locally and in CI, before anything else runs: a verdict from the wrong tools means nothing.
- **No global installs.** Tools install into a cache keyed by the manifest's hash; the gate puts that directory first on `PATH` itself. *(Annotated 2026-11-03, as built: each tool has its own directory, `<name>-<version>-<archive_sha256[:12]>`, under `~/.cache/vorsar/cep/tools`, or under `VORSAR_CEP_TOOLS_CACHE` when set.)*
- **A bump is one commit**, `toolchain: bump X a→b`, that changes the manifest **and** fixes what the new version finds. A checksum is never edited to make it pass.

---

## The gate

> The gate's full mechanism catalog — architecture analyzers with planted-failure proofs, suppression liveness, freshness checks, hostility rules — is [ADR-0015](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md); the assurance strategy is [qa-strategy.md](qa-strategy.md).

The gate is a **registry**, not a script: every check is one row. **The row schema is stated once, normatively, in [ADR-0015 §1.1](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)** (id, job, tiers, blocking, timeout, needs, scope, expected_scope, min_wants, lands_in, exclusive, command). Every local tier, every `make` target and every CI job runs the same rows, so the two cannot diverge. The summary opens with the **LAW Δ block** (ADR-0015 §1.3): every change to a gate/spec/architecture law file in the diff, printed where the owner reads at merge. *(Annotated 2026-11-03: the LAW Δ block lands in ~~S1.1b~~ S1.1c ([owner, 2026-11-04](../sprints/decisions.md#owner-decisions-2026-11-04--s11b-slimmed-toward-engine-code)); until then the record lists the sprint's rule-file changes by hand.)*

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

*(As built, 2026-11-06: `race` runs at T4, as its own CI job, and nightly as `race-stress`; golangci lint, suppressions, the vulnerability scan and the licence check are S1.1c's; the conformance suite is S3.3's. `gate/registry.json` gives each row's tiers.)*

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

*The `suppressions` row checks it from S1.1c-2 (T1–T4 and nightly); golangci-lint's own linters and `nolintlint` turn on in S1.1c-4 — until then every `//nolint` is red, because no linter it could name is enabled (owner, 2026-11-12, S1.1c-2's design).*

Every lint suppression in first-party code is **accountable**, checked by the gate. The metadata rides on the same line as the directive:

```go
//nolint:gosec // PERMANENT reason="fixture generator; output is never security-relevant" ref=docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md#2-architecture-constraints-as-analyzers-with-teeth-reference-a-in-go sprint=S1.1c-2
//nolint:errcheck // TEMP reason="close error surfaced by the sink rework" expiry=2026-12-31 issue=S3.3-2-1 sprint=S1.1c-2
```

Rules (checked by the `suppressions` row, S1.1c-2): the canonical form only — `//nolint:<linter>[,<linter>]`, two slashes, no space (golangci-lint also honours `// nolint`, `///nolint` and `//nolint x`; each is red so none hides); exactly one of PERMANENT or TEMP; a `reason="…"` of at least 12 characters; **PERMANENT carries `ref=<repository path>#<heading anchor>` that resolves to a tracked document's heading — an ADR or a `docs/sprints/decisions.md` section** (a permanent exception traces to a decision, never to convenience); TEMP carries `expiry=YYYY-MM-DD`, at most 90 days ahead, and `issue=<id>` naming an entry of `gate/known-issues.json` (so it ages with that issue) — an expired one is red at T4 and the nightly, and earlier tiers show a countdown in the LAW Δ block; every suppression carries `sprint=<sprint id>`, a row or part of the sprint plan at or before the current sprint; a suppression for a linter that is not enabled fails. The same grammar governs every list under `tools/allowlists/` (ADR-0015 §3.10). A Python formatter waiver is `# fmt: skip  # <reason, 12+ characters>` on the statement (ruff ignores `# fmt: skip` followed by other text, and the gate runs ruff with `--ignore-noqa`, so `# noqa` waives nothing and is red); ruff also honours other spacings (`# fmt:  skip`, `# fmt:\tskip`), red as `ruff honours this spelling too; write # fmt: skip  # <reason>`, and the regions `# fmt: off` and `# yapf: disable` and isort's skips, red as `write # fmt: skip  # <reason> on the statement`. ruff's per-file waivers in `ruff.toml` (under `[lint]` only — `extend` and the top-level ignore tables are red) each have an entry in `tools/allowlists/ruff.json`. A generated Go file carries `// Code generated … DO NOT EDIT.` before its package clause and is listed by path in `tools/allowlists/generated.json`; either alone is red. The LAW Δ block ends with `exemptions: <n> in code, <m> listed (±d this diff)` — every waiver a tool honours counts, valid or not — and names each exemption a diff adds or removes; one whose reason or metadata changed in place is named `reason changed` or `<key> <old> → <new>`, never added. Fix the code first; suppress only a genuine, documented necessity, never to quiet the gate.

## TODO policy

*`todocheck` lands in S1.1c (owner, 2026-11-04). Until then review enforces this.*

A `TODO` carries an **owner or a sprint or issue id**: `// TODO(S3): …`, `// TODO(ramith): …`, `// TODO(#42): …`. A bare `// TODO` is not allowed. The same rule covers the synonym family `FIXME`/`HACK`/`XXX`; a named sprint must be ≤ current + 2, and a marker naming a closed sprint fails T4 (ADR-0015 §2 `todocheck`).

---

## The code graph (GitNexus)

The repository is indexed as a GitNexus knowledge graph: every symbol with its callers, callees, imports and the execution flows it is part of. The graph answers "who calls, imports or depends on X" in one call and a few hundred tokens; the same answer from reading files costs thousands and misses callers. **A claim about callers, imports or blast radius cites a graph query, not a file read.**

- **Where the index is.** Outside the tree, in the slot the GitNexus registry names for this path (`~/.gitnexus/storage/cep-*`). The `gitnexus` CLI and the MCP server resolve it from the registry, so no environment variable is needed; `.gitnexus/` is ignored as a guard against an in-repo index.
- **Freshness.** A Claude Code hook keeps the index at `HEAD` ([gitnexus_reindex.py](../../gate/hooks/gitnexus_reindex.py), registered in `.claude/settings.json`; nothing at OS level, [owner, 2026-11-06](../sprints/decisions.md#owner-decisions-2026-11-06--the-code-graph-re-indexes-itself)). On every prompt, and after every Bash call that runs git, it compares `HEAD` with the commit the GitNexus registry records; when they differ it starts a follow-up in the background (its log is `auto-reindex.log` in the index's storage) that runs `gitnexus analyze --force --index-only` (a full rebuild, about 10 s, the full-text index included) and, when that succeeds, sends TERM to every `gitnexus mcp` server whose parent is Claude Code — a server keeps the database handle it opened and answered from a stale index after every re-index; the clients start a fresh one on their next call ([owner, 2026-11-11](../sprints/decisions.md#owner-decisions-2026-11-11--the-gitnexus-hook)). A server under anything else is left alone. An agent's commit is picked up at once, a `git pull` in your own terminal at your next prompt. It skips linked worktrees and does nothing where `gitnexus` is not installed; `--index-only` never rewrites `CLAUDE.md` or the skills. The MCP resource `gitnexus://repo/cep/context` states the indexed commit. GitNexus's own after-commit note can call the index stale before the background run ends, so read the resource again a few seconds later; re-index by hand with the same command only if it is still behind.
- **Which tool for which question.** All are read-only except `rename`, which edits files.

| Question | Tool |
|---|---|
| Is the index current? | the `context` resource |
| Where does this behaviour live? | `query` (before any `grep`) |
| Who calls, imports or is called by X? | `context` |
| What breaks if X changes? | `impact` (`direction: upstream`; no callers reads as *unproven*, not safe) |
| How does A reach B? | `trace` |
| What does this branch's diff touch? | `detect_changes` (`scope: compare`, `base_ref: main`; from an agent's worktree pass its absolute path as `worktree`, or the server diffs the indexed checkout instead) |
| Import edges, package lists, anything else | `cypher`, after reading the `schema` resource |

- **What it does not know.** Markdown is a bare file node (no doc-to-code edges); a symbol added on a branch is absent until the next re-index; `.cep` example files are files, not rules. Those questions still need the file.
- **Agents** have the graph tools in their `tools:` list and a "code graph first" section in their definition; a brief names the symbols to `impact` before editing. The coordinator verifies a lane with `detect_changes` on the combined branch before reading its diff.

## Project memory (Hindsight)

A local Hindsight bank (`coding-agent::cep`) holds knowledge pages distilled from commit messages and past sessions: decisions, conventions, and why things are the way they are. It is a past record: the code, the ADRs and [decisions.md](../sprints/decisions.md) win, and a stale page is fixed by a `Correction:` document, never by trusting it.

- **The coordinator** searches the pages before writing a brief, plan, review or commit, and copies every decided value a lane relies on into the brief, quoted.
- **Agents** have only the two read-only page tools (`hindsight_search_knowledge_pages`, `hindsight_read_knowledge_page`) and a "project memory first" section in their definition: search when the brief is silent (builders) or before calling a decided choice a defect (reviewers), confirm in a tracked source, at most three searches, and report each fact used. `hindsight_reflect` is left out of their lists because its model call is too slow for a time-boxed lane.

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

**Last updated:** 2026-11-11 (S1.1c-1: the LAW Δ block, the marker and the expiry, the deps row; the apidiff tool in gate/proofs/go.mod)
