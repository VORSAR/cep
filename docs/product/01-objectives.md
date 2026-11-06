# Product Objectives — complex-events-engine

**Status:** v1.0 · 2026-10-31 · distilled from the product brainstorm ([00-product-brainstorm.md](00-product-brainstorm.md), decisions in its §10)
**Cadence:** quality-driven, no calendar commitments; everything below is ordered by importance, and the roadmap (02) will order it by dependency.

## Mission

Build the open-source (Apache-2), Go-native complex event processing engine that powers Vorsar's AI-agent detection & response platform: **correlate events across AI agents, humans, and data sources; detect temporal attack patterns per-request guardrails cannot see; respond at machine speed.**

## Operating model

One human developer (product owner) orchestrating **waves of coding agents**. Consequences, baked into these objectives:

- **Tests are the spec.** Agent-generated code is only as trustworthy as the gates it must pass, so the conformance suite, deterministic replay, and the rule unit-test framework (O1, O6) are not conveniences — they are the development method. They get built first and nothing merges around them.
- **Work must decompose.** The roadmap and sprint plan break features into well-specified, independently verifiable, parallelizable tasks suitable for agent-wave execution with human review at the seams.
- **Scope discipline is structural.** The core/diff/later tier system from the brainstorm is the guardrail against solo-orchestrator scope creep.

---

## O1 — A provably correct CEP core

*Why: correctness is the product (design principle 2). A security engine that mis-evaluates a pattern once loses trust permanently — and agent-wave development makes an executable definition of "correct" non-negotiable.*

Outcomes:
- Event-time and processing-time semantics with out-of-order tolerance, implemented and documented honestly (what is guaranteed, what is not).
- A **pattern-semantics conformance suite** — goldens for windows, patterns, sequences, absence, joins, partitions (informed by Siddhi's observed behavior where sensible) — at 100% pass in CI, from the first operator onward.
- **Deterministic replay (playback mode) built before the operators it tests**, so every stateful feature lands with reproducible, event-time-driven tests.
- Snapshot/restore proven by kill-and-resume tests: state survives restarts without event loss within documented semantics.
- Zero known correctness bugs open longer than 30 days.

Feature areas: FA1, FA2, FA3, FA9, FA10(playback).

## O2 — The temporal pattern language Go doesn't have

*Why: the uncontested gap — no maintained Go engine expresses "A followed by B within T, and no C." This is the reason the engine exists.*

Outcomes:
- Modernized Siddhi-inspired streaming SQL dialect: filters, projections, group/having/order/limit, windows, joins, patterns (`->`, `every`, counting, logical and/or), **absence detection** (`not … for …`), strict sequences with Kleene quantifiers, `within`, cross-event references, output rate limiting, triggers.
- **Go builder API as the headline interface** with guaranteed parity — both author the same rule model.
- Siddhi's known gaps fixed: hopping window, `week` granularity, honest rejection (deploy-time, with line/col) of unsupported constructs.
- Per-key partitioning with isolated state and idle-key TTL — unbounded identity cardinality (agents, sessions, users) is safe by default.

Feature areas: FA1, FA2, FA3, FA4, FA7.

## O3 — Embeddable-first form factor

*Why: the correlation layer must live where agent traffic lives — inside gateways, sidecars, collectors — exactly where JVM engines cannot go.*

Outcomes:
- Idiomatic Go library (contexts, no globals): create runtime → deploy apps → send events → receive detections, in-process, sub-millisecond hot path.
- Single static binary runner: rule files + YAML config, millisecond startup, scratch-image and ARM64 friendly, health/readiness endpoints, hot rule reload.
- Scale-to-zero viability: fast start plus snapshot resume.

Feature areas: FA12, FA9.

## O4 — Agent-fleet detection readiness

*Why: the mission. The engine must turn agent telemetry into detections and responses out of the box.*

Outcomes:
- Ingestion that tolerates schema drift in OTel GenAI / MCP conventions; connector framework + lean built-in set (in-memory, stdio, file, HTTP, webhooks; then NATS/Kafka/gRPC) — the wider adapter catalog lives in sibling Vorsar repos.
- Canonical agent-event schemas keyed by agent/session/user/tenant/delegation chain; reference tables for registries, allowlists, sensitivity labels.
- Response sinks: gateway block/quarantine calls, revocation webhooks, step-up approval, SIEM/SOAR events.
- **Detection pack v1 (Apache-2, versioned, maintained):** exfiltration sequences, delegation abuse, RAG grooming, identity velocity, gone-dark — each with forensic event chains, LLM-optional variants.
- Behavioral baselines via minimal rolling rollups (minutes/hours/days).

Feature areas: FA5, FA6(minimal), FA8, FA13.

## O5 — Optional LLM-in-the-loop judgment

*Why: many agentic attacks are semantic; "CEP narrows, LLM judges" — and the LLM is strictly optional (design principle 7).*

Outcomes:
- `llm:` scalar functions and the two-stage judge over matched event chains, with verdict-gated response sinks.
- Async semantics (timeout, fail-open/fail-closed, provisional-then-confirmed), budgets/caching/sampling, replayable judge fixtures for CI.
- Provider extension type: OpenAI-compatible, gateway-routed, local models.
- **Zero-LLM deployments lose nothing** — validated at deploy time, never at match time.

Feature areas: FA14, FA11.

## O6 — Testing & developer experience that converts

*Why: time-to-first-detection is the adoption lever, and the same tooling is our own agent-wave verification harness.*

Outcomes:
- Measured quickstart: `go get` → first detection firing in under 15 minutes.
- Rule unit-test framework (given events → expect events) as table-driven Go tests and a CLI runner; sandbox mode; validate-only with line/col errors; CLI + REPL.
- Prometheus metrics (throughput, latency, buffers, state size) switchable at runtime.

Feature areas: FA10.

## O7 — A healthy public OSS project

*Why: decided posture — the engine's public credibility feeds the platform's.*

Outcomes:
- Docs site with the language reference generated from extension metadata; versioned releases with changelogs; contribution guide; examples gallery led by agentic-security detections.
- Adoption instrumented (opt-in anonymous ping → weekly active engines); external contributors and third-party extensions exist within the first year of public release.
- Extension SPI stable enough to invite community connectors (prerequisite for WASM fast-follow).

Feature areas: FA11, cross-cutting.

---

## Objective → feature-area map

| | O1 correctness | O2 language | O3 embeddable | O4 detection | O5 LLM | O6 DX | O7 OSS |
|---|---|---|---|---|---|---|---|
| FA1 core/language | ● | ● | | | | | |
| FA2 windows | ● | ● | | | | | |
| FA3 patterns | ● | ● | | | | | |
| FA4 joins | | ● | | | | | |
| FA5 tables/state | | | | ● | | | |
| FA6 aggregations (minimal) | | | | ● | | | |
| FA7 partitions | | ● | | ● | | | |
| FA8 connectivity | | | | ● | | | |
| FA9 reliability | ● | | ● | | | | |
| FA10 testing/observability | ● | | | | | ● | |
| FA11 extensibility | | | | | ● | | ● |
| FA12 runtime modes | | | ● | | | | |
| FA13 detection pack | | | | ● | | | ● |
| FA14 LLM-in-the-loop | | | | | ● | | |

## Non-goals (inherited from the brainstorm, §7)

No traditional-CEP vertical investment (IoT/edge, fraud, generic observability content) without an AI angle · no distributed cluster in v1 · not a streaming database · no exactly-once-across-external-systems claims · no graphical tooling in v1 · adapter catalog lives in sibling repos, not here.

## Risks being managed

1. **Correctness burden** (O1 exists because of it) — mitigated by conformance-first development and deterministic replay.
2. **Competitors closing the gap** — Zenity making correlation real-time; Palo Alto owning gateway + runtime security. Quarterly watch; our moats are embeddability, the language, and Apache-2.
3. **Telemetry immaturity** — OTel GenAI/MCP conventions unstable; mitigated by drift-tolerant ingestion here and the sibling adapter repos.
4. **Solo-orchestrator scope creep** — mitigated by the tier system and this document: anything not serving O1–O7 waits.

## Next

- [x] `02-roadmap.md` — dependency-ordered phases (what unblocks what), no dates. **MVP strategy (owner):** one vertical-slice use case — detect, then disable the agent via IAM webhook + escalation email — building everything required to ship it.
- [x] ~~`03-sprint-plan.md`~~ [`../sprints/sprint-plan.md`](../sprints/sprint-plan.md) *(done: Phase 1's decomposition approved 2026-11-02; progress is its [§Status](../sprints/sprint-plan.md#status))* — agent-wave execution plan: decomposed, verifiable task batches with human review gates. *(Amended 2026-10-31: moved to `docs/sprints/` with the adopted sprint process, [00-sprint-model.md](../sprints/00-sprint-model.md).)*
