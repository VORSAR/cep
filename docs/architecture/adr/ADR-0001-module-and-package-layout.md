# ADR-0001: One Go module; small public facade; internal engine packages with a fixed import DAG

## Status
Accepted (owner, 2026-11-01 — ratified with the enforcement conditions implemented in ADR-0015 and docs/development/qa-strategy.md).

## Context
The engine is embeddable-first (O3): users import a Go library whose public surface must stay small and stable from v0.1.0, because OSS users (O7) will depend on it. Development happens as coding-agent waves with one owner per file (ground rule 17), so package boundaries must be drawn where agent lanes are cut: a lane = one package wherever possible. The code rules (DEVELOPMENT.md) forbid package-level mutable state and require pure rule packages with thin adapters.

## Decision
1. Exactly one Go module, `github.com/vorsar/complex-events-engine`, rooted at the repository root. No nested modules, no multi-module workspace.
2. The public API is exactly three packages: `cep` (root: `Runtime`, `Config`, `Deploy`→`AppHandle` with `Start/Stop/Input/Table`, `InputHandler`, `Table`, the public Event view, `BuildInfo`, error types), `rule` (the generics builder incl. `rule.Param[T]`), and `io` (`Source`/`Sink`/`Mapper`/`RowWriter` SPI plus the MVP built-ins). Everything else sits under `internal/` and is invisible to importers. `cmd/cep`'s `validate` and `test` subcommands are implemented through the public Deploy pipeline (its expect-sandbox stage), not through internals — if the CLI needs something more, the facade grows deliberately (round-2 F9).
3. The package list and the eight numbered import rules in [ARCHITECTURE.md §3](../ARCHITECTURE.md) are normative. A gate check (`go list`-based, T2+) MUST fail on any violation.
4. Operator packages (`internal/pattern`, `internal/window`, `internal/expr`, `internal/text`, `internal/table`, `internal/partition`) MUST be pure: no goroutines, no I/O, no wall clock, no imports of `runtime`/`io`/each other — except the edges named in the ARCHITECTURE §3 exception table, **which is the single normative list; this sentence cites it rather than copying it, so the two cannot drift** (round-2 N5.1 as revised by the consistency review). Purity exceptions (window/cron.go's `time` types; table's `sync` snapshot swap; event's `sync.Pool`) are named with fixtures in ADR-0015 §2.
5. Binaries live under `cmd/cepd` (runner) and `cmd/cep` (validate/test/fmt CLI); they import only public packages — if a binary needs something internal, the public facade grows deliberately instead.

## Alternatives considered
- **Multi-module repo (engine + CLI separate)** — rejected: one release artifact, one version, one `go get`; split adds tag/replace ceremony with zero Phase-1 benefit.
- **Big public surface (export operator packages)** — rejected: O7 requires a stable SPI; exporting internals freezes implementation details at v0.1.0. Extensions come later through deliberate SPI (FA11), not accidental exposure.
- **`pkg/` convention instead of root package** — rejected: `cep.Runtime` at module root reads best in user code (`cep.New(...)`), matching the embeddable-first headline.

## Consequences
- Positive: agent lanes map to packages; import-DAG check makes architecture drift a red gate instead of a review argument; public API reviewable in one sitting.
- Negative: facade indirection (public `cep.Event` wraps `internal/event.Event`) costs some boilerplate; accepted for surface control.

## Implementation notes for coding agents
- Create `go.mod` with `module github.com/vorsar/complex-events-engine`, pinned `go` and `toolchain` lines (exact version chosen at S1 plan check per DEVELOPMENT.md).
- The import-DAG check: a small Go program under `tools/importcheck/` run by the gate; it reads rules from `tools/importcheck/rules.json` which MUST mirror ARCHITECTURE.md §3 verbatim (same numbering, asserted by a comment).
- Package names: the `io` public package is directory `io/` with package name `cepio` to avoid shadowing stdlib `io` in user code examples. Document this in its package comment.
