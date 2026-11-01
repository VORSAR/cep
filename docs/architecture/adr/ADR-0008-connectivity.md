# ADR-0008: Connectivity — small Source/Sink/Mapper SPI, MVP built-ins, at-least-once, sinks own their queues

## Status
Accepted (owner, 2026-11-01 — ratified with the enforcement conditions implemented in ADR-0015 and docs/development/qa-strategy.md).

## Context
Roadmap Phase 1 slice (FA8): sources http (JSON), in-memory, stdin (replay/demo); sinks http webhook (JSON, `{{attr}}` templates), SMTP email, log; connect retry + backoff. The wider adapter catalog lives in sibling repos (decision #10), so the SPI must be public and small but is NOT frozen for third parties until Phase 5 (O7). SEMANTICS §8/§10: append-only sinks reject changelog producers at deploy (E-SINK-001); at-least-once toward sinks, stated honestly; a sink failure never loses rule state. DEVELOPMENT.md: the hot path never blocks on I/O. Batch windows need source punctuations (SPEC §6.3, E-WIN-002).

## Decision
1. Public SPI in package `io` (dir `io/`, package name `cepio`):
   ```go
   type Source interface {
       Open(ctx context.Context, emit EmitFunc) error // blocks until ctx done; emit is threadsafe
       // EmitFunc: func(fill func(RowWriter) error, batchID string) error — the engine allocates the row,
       // calls fill with the writer, validates, ingests. Mappers are fill-functions over bytes.
   }
   // The PUBLIC value boundary (round-2 N3 — internal/event.Value must never cross the facade):
   // the engine hands the mapper a schema-bound RowWriter; the mapper writes typed values
   // directly into engine-owned storage. Third parties implement Source/Mapper with ONLY
   // public types; zero-alloc preserved (no intermediate map, no reflection, int64-safe).
   type RowWriter interface {
       SetString(attr int, v string); SetInt(attr int, v int64); SetFloat(attr int, v float64)
       SetBool(attr int, v bool); SetTimestamp(attr int, unixMs int64)
       SetDuration(attr int, ms int64); SetObject(attr int, v any) // all seven schema types crossable (round-3 R1)
       SetNull(attr int)
       AttrIndex(name string) int // -1 if unknown; schema-driven mappers resolve once and cache
   }
   // batch identity travels as EmitFunc's batchID string — no separate handle type (round-3 R3)
   type Sink interface {
       Accepts() Mode                      // Append or Changelog (E-SINK-001 checked at deploy)
       Write(ctx context.Context, e cepio.OutEvent) error // called from the sink's own goroutine only
   }
   type Mapper interface { Decode(b []byte, w RowWriter) error }  // json, text in MVP
   ```
   Annotation vocabulary (`@source`/`@sink`/`@map` types and keys) is a closed table in `internal/validate/connectors.go`; unknown names = E-NAME-001.
2. **Sinks own their failure domain (revised 2026-11-01 — the blocking party is the ROUTER, never a shard; see ADR-0006 §5 for the deadlock-freedom argument):** each sink binding gets one flusher goroutine + one bounded queue, written only by the router. The flusher drains its queue in batches and **coalesces writes** (one marshal+write per drained batch where the protocol allows — perf study §1/§2 amortization), with per-event fallback for SMTP. Retry with exponential backoff + cap; after max retries: log + `sink_failures` counter (MVP; fault streams are Phase 2).
3. **At-least-once, stated:** delivery may duplicate on retry; the docs and the demo say so (no exactly-once claims — non-goal). The `{{attr}}` template renderer and JSON encoder live in `io/render.go`, shared by http and smtp sinks.
4. **Sources:** http source = one listener, POST JSON per event or NDJSON batch (each POST body = one source batch → `batchID` = request id, giving batch windows their punctuations — a source that cannot batch passes an empty batchID and `batch()` over it is E-WIN-002 at deploy, per the connector table's `punctuates: bool` column); stdin source = NDJSON, one batch per `--batch-lines` or per EOF (demo/replay use); inmemory source/sink = the Go API's programmatic feed (also what tests use).
5. **Config:** `cmd/cepd` reads rule files + a config file supplying `${param}` overrides and connector settings; the library user passes the same via `cep.Config`. **DECIDED (owner, 2026-11-01): JSON** (`cepd.json`, stdlib-only); YAML revisited on platform demand; roadmap amended. Secrets arrive via params/env, never inline in rule files (ground rule 20 posture).

## Alternatives considered
- **Shared global sink pool** — rejected: one slow webhook must not starve another sink; per-binding queues isolate failure domains and are trivially understandable for agent implementers.
- **Freezing the SPI now for third parties** — rejected: O7 schedules SPI freeze at Phase 5 after real built-ins exercise it; until then the package documents "experimental, may change in minor versions".
- **Exactly-once via dedup store** — rejected: non-goal (brainstorm §7); honest at-least-once plus idempotency guidance.

## Consequences
- Positive: hot path provably non-blocking (lint: no `io` imports in operators, ADR-0001); batch punctuation has a concrete source; sibling-repo adapters get a worked example (http) to copy.
- Negative: per-sink queues add memory per binding — bounded and configurable; duplicates possible on retry — documented.

## Implementation notes for coding agents
- SMTP sink in MVP: plain `net/smtp` with STARTTLS, one message per event, `{{attr}}` templates in subject/body; no HTML, no attachments; the demo's mail-catcher validates.
- OutEvent carries the canonical encoding from ADR-0003/0005 plus the target stream name; the log sink prints exactly the canonical JSON (so demo output diffs against expect fixtures).
- Every built-in ships with a conformance-style integration test using its own listener on port 0 and `t.TempDir()` (testing.md rule 3).
