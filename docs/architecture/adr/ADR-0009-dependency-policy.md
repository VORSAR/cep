# ADR-0009: Dependency policy — stdlib first; two sanctioned golang.org/x deps; everything else in-repo; licenses gated

## Status
Accepted (owner, 2026-11-01 — ratified with the enforcement conditions implemented in ADR-0015 and docs/development/qa-strategy.md).

## Context
Ground rule 20: dependencies are few, pinned, Apache-2-compatible. O3 wants a small static binary; OSS credibility (O7) suffers from dependency sprawl; agent-written code tends to reach for deps where 200 lines of stdlib suffice. The gate's T4 runs a license check and a vulnerability scan (DEVELOPMENT.md).

## Decision
1. **Default: standard library.** An agent adding any module requires a sprint-card note + owner approval at the plan check; the gate fails on an unapproved `go.mod` change (the approved list is `tools/deps-allowlist.json`).
2. **Sanctioned (owner-ratified 2026-11-01; goleak/apidiff rows added by owner decision 2026-11-01 — foundation review B5, ≠ the in-repo-equivalents recommendation):**
   | Module | Scope | Why | License |
   |---|---|---|---|
   | `golang.org/x/tools` (goyacc; `go/analysis` + `analysistest`) | **build-time only**; generated parser committed; the gate's analyzers and their fixtures (ADR-0015 §2 — scope widened 2026-11-01) | ADR-0002: LALR with conflict reporting; CockroachDB/Vitess/PromQL precedent; ADR-0015: analyzers-with-teeth | BSD-3 |
   | `golang.org/x/text` (unicode/norm) | runtime | ADR-0007: NFC normalization; pinned Unicode tables | BSD-3 |
   | `go.uber.org/goleak` | **test-only** — the gate's goroutine-leak planted proofs (ADR-0015 §3.4) | battle-tested leak detection; owner-approved over an in-repo NumGoroutine check | MIT |
   | `golang.org/x/exp/apidiff` | **build-time only** — T4 API diff vs merge-base/last-tag (ADR-0015 §2 `apipartition`) | the Go project's own API-compatibility tool | BSD-3 |
3. **Explicitly in-repo instead of deps:** Aho-Corasick (ADR-0007), the 5-field Vixie cron parser with the SPEC-pinned dialect (ADR-0014 — `robfig/cron` rejected: dialect drift, features we must reject), Prometheus **text exposition** (`internal/metrics` writes the text format by hand; `client_golang` deferred until Phase 3+ needs histograms), `cepd` config — **decided (owner, 2026-11-01): stdlib-only JSON** (`cepd.json`); YAML revisited on platform demand.
4. CI (GitHub Actions) uses only actions pinned by commit SHA; the vulnerability scan is `govulncheck` (golang.org/x/vuln, build-time tool in the manifest, same BSD-3 family). The gate's **Python side** (grammar-differential, semantics harness) is manifest-pinned too: Python + `lark==1.3.1` with checksums in `tools.json` (round-2 F18) — test-infrastructure scope only, never the engine.
5. Test-only dependencies follow the same rule. **Exactly one is sanctioned: `go.uber.org/goleak`** (owner, 2026-11-01 — row in §2; confined to the gate's proof tests, never imported by engine tests for assertions). Otherwise stdlib `testing` suffices — no testify.
6. **Owner sign-offs (2026-11-01): goyacc, x/text and benchstat ALL APPROVED.** `golang.org/x/perf/cmd/benchstat` (build-time, BSD-3, drives the CI benchmark tripwires); fast-JSON libraries (sonic/simdjson-go) — recommended NO: JIT/assembly/platform constraints and JSON-conformance failures documented in the perf study; the in-repo schema-driven decoder captures the win dependency-free. Latency histograms are in-repo (no hdrhistogram dep).

## Alternatives considered
- **Zero deps absolutism (hand-written NFC)** — rejected: Unicode normalization is a correctness-critical, version-pinned table problem; x/text IS the Go project's own canonical answer and satisfies SEMANTICS §5.1 pinning cleanly.
- **Liberal modern Go ecosystem usage** — rejected: ground rule 20 and the licensing-clarity product principle (brainstorm principle 6).

## Consequences
- Positive: `go.sum` stays reviewable in one screen; license audit trivial; static binary stays small; supply-chain surface ≈ the Go project itself.
- Negative: we maintain small implementations (AC, cron, metrics text) — each bounded, each with conformance tests; JSON config is less ergonomic than YAML — revisit on demand.

## Implementation notes for coding agents
- Adding ANY import outside stdlib + the sanctioned table = stop and surface to the coordinator; do not vendor, do not copy code in without a license header review.
- Pin both x modules to exact versions in `go.mod`; `tools.json` records goyacc's version + checksum for the gate's parity step.
