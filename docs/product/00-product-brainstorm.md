# Product Brainstorm — Go-Native Complex Event Processing Engine

**Status:** Draft v0.2 · 2026-10-31 — direction decided with product owner (§10); AI-security wedge validated by competitive research
**Scope:** Product description and feature catalog only. No architecture. Objectives, roadmap, and sprint plan come later as separate documents.
**Inputs:** Full capability sweep of the Siddhi repo (`/Users/ramith/code/siddhi`) and an Oct-2026 competitive scan of the CEP/stream-processing landscape (both saved to Hindsight memory).

---

## 1. One-line pitch

**The detection engine for the agentic enterprise:** a Go-native complex event processing core that correlates events across AI agents, humans, and data sources — "agent read a sensitive store, then called an unapproved tool within 5 minutes, with no human approval in between" — and triggers response at machine speed, embedded next to (or inside) the AI gateway as a Go library or a single static binary.

Under the hood it remains "the SQLite of CEP" — a general-purpose, embeddable temporal-pattern engine. That generality is the substrate, not the pitch.

## 2. Product description

A cloud-native **streaming and complex event processing (CEP) engine written in Go**, inspired by Siddhi. Developers declare typed event streams and write declarative rules — filters, windows, joins, aggregations, and first-class **temporal patterns** (sequence, absence, repetition, time bounds) — and the engine evaluates them continuously over live events with millisecond latency, emitting derived events to downstream systems.

It ships in two shapes from day one:

1. **An embeddable Go library** — import it, define rules, push events, receive detections in-process. No infrastructure, no network hop, no JVM.
2. **A single static binary runner** — point it at rule files, wire sources and sinks via configuration, run it on a server, in a scratch container, as a Kubernetes sidecar, or on an ARM edge gateway.

Apache-2 licensed, open source, with a pluggable extension model for connectors, functions, windows, and stores.

**Role in the portfolio (decided 2026-10-31):** this repository is not a standalone product. It is the **CEP core of the Vorsar org's main product** — an AI-agent detection & response platform that lives elsewhere; this engine powers its real-time correlation. The engine keeps general-purpose mechanics (that is what makes it testable and reusable), but every prioritization call in this document serves one mission: **detecting and responding to attacks across fleets of AI agents at machine speed.** Traditional CEP verticals (IoT/edge, embedded fraud) are not target markets; we serve them only where their needs coincide with ours. The repo keeps the name `complex-events-engine` until a requirement forces a change.

## 3. Why now — the market gap

From the Oct-2026 competitive scan (full brief in Hindsight: `market-scan-oct-2026-competitive-landscape-for-a-go-native-cep-engine`):

- **No actively maintained Go engine offers true temporal pattern matching.** The Go ecosystem has pipelines (Benthos/Bento, Redpanda Connect), messaging (Watermill), and edge SQL rules (LF Edge eKuiper), but nothing can express "A followed by B within T, with no C in between."
- **Siddhi itself is in maintenance mode** (~one release every 1–2 years), orphaning a proven design on the JVM. Esper is GPL/commercial and slowing. FlinkCEP requires a cluster and a platform team.
- **The deployment shapes where CEP is most needed now — edge gateways, telemetry collectors, sidecars, scale-to-zero services — are exactly where the JVM can't go** and where Go is the native language.

### The agentic AI inflection — why this product, now

Enterprises in 2026 run **fleets of AI agents** — client-facing copilots, internal workflow agents, coding and ops agents — fronted by AI gateways (or by nothing at all), wired to tools and data through MCP servers, acting under delegated identities, at machine speed and scale. The security model has not kept up:

- **Guardrails are per-request and per-agent.** A prompt-injection filter or policy check inspects one call from one agent in isolation. Even where guardrails exist they are rudimentary: they can block *one agent doing one thing* — they cannot see a campaign.
- **Attacks are cross-agent, cross-source, and temporal.** The sophisticated ones unfold as *sequences* spread across identities and time: poison a RAG/memory store today and trigger retrieval by a different, higher-privileged agent next week; chain delegations until a confused deputy crosses a privilege boundary; exfiltrate low-and-slow, where every individual tool call looks innocent and only the sequence is damning; fan a compromised agent identity out across MCP servers in seconds.
- **Nobody can see the whole board.** The telemetry exists — OTel GenAI semantic-convention spans, gateway logs, MCP audit events, auth logs — but it sits uncorrelated. SIEMs ingest it in batches, minutes late, with rule languages that cannot express temporal sequence or absence. By the time a human correlates, the agents have already acted.

That is, almost verbatim, the definition of complex event processing: **correlate events from many agents, humans, and data sources; detect sequences, absences, and anomalies per identity, session, and delegation chain; and respond at machine speed** — quarantine the agent, revoke the token, demand human approval — in milliseconds, inline with or beside the gateway. Form factor seals it: a correlation engine for agent traffic must live *next to or inside* the gateway — a sidecar, an embedded library — exactly where a JVM cluster cannot go and a Go library is at home.

Detections this engine expresses naturally:

- **Sensitive read → outbound call:** an agent reads from a sensitivity-labeled source, then makes any outbound tool call to an unapproved destination within 5 minutes, *with no human-approval event in between* → quarantine the session. *(pattern + absence + table join)*
- **RAG/memory grooming:** the same document is retrieved by N distinct agents that then exhibit anomalous tool use → flag the document and its ingestion path. *(partition + pattern + aggregation)*
- **Delegation abuse:** a delegation chain deeper than N hops crosses a privilege boundary within one session → block. *(sequence + reference table)*
- **Identity velocity:** an agent identity's token spend or tool-call rate exceeds its rolling 30-day profile → throttle and alert. *(incremental aggregation + filter)*
- **Gone dark:** an agent session starts and no heartbeat or completion arrives within T → investigate. *(absence detection)*
- **Semantic drift:** an agent's tool-call sequence stops matching its stated task — a deterministic pattern flags the sequence, an LLM judge confirms the intent mismatch → step-up approval. *(pattern + LLM judge, see FA14)*

Strategically this is a **vertical mission on a horizontal engine**: the engine keeps general-purpose CEP mechanics; the agentic-security mission drives all prioritization, led by a domain pack (telemetry schemas, detection rule library, gateway response integrations — see FA13). Siddhi never had a wedge and stayed a tool looking for a problem; this one is ours. Competitive validation (Oct 2026) refined the claim: Zenity *markets* multi-agent attack correlation (closed SaaS) and CrowdStrike correlates agent telemetry at SIEM latency — but **no embeddable, real-time temporal correlation engine exists at the gateway layer**, which is precisely this product. Caveat worth tracking: OTel GenAI and MCP semantic conventions are still "Development" status — correlatable but schema-volatile, so ingestion must tolerate drift, and early deployments may need us to ship instrumentation/collectors.

### Target segment (decided 2026-10-31)

**Agentic AI runtime security & observability is the mission** — cross-agent anomaly and attack detection with machine-speed response, powering the Vorsar platform.

*Engine-adjacent markets, explicitly not targets:* observability correlation, in-process fraud/risk checks, and edge/IoT monitoring all share the engine's mechanics and may adopt it organically — we welcome that adoption but build vertical content (connectors, demos, rule packs) only where an AI-security angle exists.

## 4. Target users and jobs-to-be-done

| Persona | Job-to-be-done |
|---|---|
| **Go backend engineer** (fintech, marketplace, gaming) | "When a user's events form a risky pattern, I need to know in milliseconds, inside my service, without new infrastructure." |
| **Platform / SRE engineer** | "Correlate alerts and telemetry — '5 failed logins then a success', 'no heartbeat for 2 min' — in a collector or sidecar I already run." |
| **Edge / IoT solutions engineer** | "Run condition-monitoring rules on gateway hardware, update them remotely, survive restarts without losing state." |
| **AI platform / security engineer** | "My agents act at machine speed across the enterprise. I need to see when several of them — or an agent plus a human plus a data source — together form a pattern no single guardrail can catch, and stop it automatically." |
| **Security engineer** (secondary) | "Express detection rules over event logs with sequences and absences, test them deterministically, replay historical events." |

## 5. Positioning

> For **platform and security teams running fleets of AI agents**, who find that **per-request guardrails block one agent doing one thing while real attacks unfold as sequences across agents, identities, and time**, this engine is the **real-time correlation layer the gateway is missing**: temporal patterns, absence detection, and per-identity state, embeddable next to Kong/LiteLLM/OTel collectors — where SIEMs are minutes late and JVM engines cannot run.

**Defensible claim (validated by competitive research, Oct 2026):** *no embeddable, real-time temporal correlation engine exists at the AI-gateway layer.* The only cross-agent correlation on the market is closed-platform (Zenity's "Multi-Agent Attack Correlation" — SaaS, mechanism undocumented), SIEM-latency (CrowdStrike Next-Gen SIEM ingestion), or marketing-stage. Strongest proof points to cite:

1. **Salesloft Drift (Aug 2025):** a compromised AI chat agent's OAuth tokens were used to pillage hundreds of Salesforce/Google Workspace tenants over 10 days — every call individually legitimate; a textbook identity fan-out that no per-request layer caught.
2. **RSAC 2026 analyst verdict:** CrowdStrike, Cisco, and Palo Alto all shipped agentic SOC tools, yet none baselines normal agent behavior — the "agent behavioral baseline gap survived all three" (VentureBeat).
3. **MCP tool poisoning (Microsoft warning, 2026) + EchoLeak (CVE-2025-32711):** groom-then-retrieve attacks that silently exfiltrate — detectable only by cross-event temporal correlation; Cisco found 43% of tested MCP servers carried command-injection flaws.

At the engine level the horizontal positioning still holds (vs. eKuiper: true CEP; vs. Esper/Siddhi: Go-native, active, Apache-2; vs. Flink: milliseconds and megabytes) — it is simply no longer the lead story.

## 6. Product design principles

1. **Embeddable first, service second.** Every capability must work as a library call; the binary runner is a thin shell over the same engine.
2. **Correctness is the product.** Event-time semantics, out-of-order handling, and pattern-matching edge cases must be right and provably so (deterministic replay testing is a core feature, not tooling).
3. **Small and instant.** Millisecond startup, small static binary, modest memory; runs in `FROM scratch` containers and on ARM.
4. **Proven language ergonomics.** The rule language follows Siddhi's streaming-SQL design (it worked); we modernize rather than reinvent, and fix its known gaps.
5. **Batteries included, everything pluggable.** Useful out of the box (core connectors, functions, windows), with first-class extension points for the rest.
6. **Apache-2, forever.** Licensing clarity is a feature; the Benthos relicensing fallout is the cautionary tale.
7. **CEP narrows, LLM judges — and the LLM is optional.** Deterministic, cheap, always-on patterns do the filtering at machine speed; LLM judgment is an **opt-in, per-rule construct** applied where it is affordable and decisive — confirming matches, scoring intent, classifying content. Not everything needs an LLM judge: every capability of the engine works with no LLM configured at all, and the hot path never blocks on a model call.

## 7. Non-goals (v1)

- **No traditional-CEP vertical investment.** IoT/edge, fraud, and generic observability content (connectors, demos, packs) are out of scope unless they serve the AI-security mission (decided 2026-10-31).
- **No distributed cluster.** Single-node engine with excellent vertical scaling and state snapshots. Distribution hooks (partition-by-shard, handler interception) may exist, but orchestration is out of scope.
- **Not a streaming database.** No general SQL storage/query engine; tables exist to serve rules, not ad-hoc analytics.
- **No exactly-once across external systems.** At-least-once with idempotency guidance; honest documentation over marketing guarantees.
- **No graphical tooling in v1.** CLI/REPL and library DX first; editors/visualizers later.

---

## 8. Feature areas (top-down)

Twelve areas. Each feature carries a tier tag:
**[core]** = required for a credible v1 · **[diff]** = differentiator worth leading with · **[later]** = valuable, deliberately deferred.

### FA1 — Event model & streaming rule language

The foundation: how users describe events and express rules.

| Feature | Description | Tier |
|---|---|---|
| Typed event streams | `define stream` with a named schema; types: string, int, long, float, double, bool, object — plus (new vs Siddhi) first-class timestamp and decimal consideration. | core |
| Streaming SQL dialect | Declarative rule language in the Siddhi tradition: definitions + continuous queries in one deployable "app" unit. Case-insensitive keywords, comments, time constants (`1 hour 30 min`), `${VAR}` substitution. | core |
| Filters & expressions | `stream[condition]` filtering before/after windows; arithmetic, comparison, boolean operators; `is null`; membership checks against tables (`in`). | core |
| Projection & shaping | `select`, aliases, `group by`, `having`, `order by`, `limit`/`offset`. | core |
| Built-in scalar functions | Type conversion/inspection (cast, convert, instanceOf), null handling (coalesce, default, ifThenElse), math/string/time basics, UUID, eventTimestamp. | core |
| Native Go builder API | Fluent Go API that constructs the same rule model as the DSL — rules as code, type-checked at compile time where possible. Parity guaranteed. **Decided: this is the headline interface — the README leads with the library; the DSL is the shareable/ops-facing format.** | core |
| Output rate limiting | Emit `first`/`last`/`all` every N events or every time period; periodic `snapshot` output; group-aware variants. | core |
| Triggers | `define trigger` at start / every N / cron — synthetic events that drive time-based rules without input. | core |
| Multiple apps per runtime | Deploy, introspect, and remove multiple rule apps in one engine instance at runtime. | core |

### FA2 — Windowing

Bounded views over unbounded streams; the workhorse of stream processing.

| Feature | Description | Tier |
|---|---|---|
| Sliding & tumbling time windows | `time`, `timeBatch` — the two most-used windows. | core |
| Hopping window | Slide-by-step time window — a known gap in Siddhi, table stakes elsewhere; we ship it built-in. | core |
| Count windows | `length`, `lengthBatch` (last N events, batches of N). | core |
| Session window | Gap-based sessionization with session key and allowed latency for late events. | core |
| Event-time windows | `externalTime`, `externalTimeBatch` — windows driven by a timestamp attribute, not the wall clock. | core |
| Specialty windows | `timeLength`, `sort`, `cron`, `delay`, `expression` (dynamic condition-based retention). | later |
| Named shared windows | `define window` — one stateful window shared by many queries (write, read, join, query on demand). | core |
| Retraction semantics | Events carry current/expired signals so downstream aggregates stay correct as windows evict. | core |

### FA3 — Temporal patterns & sequences ⭐ flagship

The reason this product exists; the uncontested gap in the Go ecosystem.

| Feature | Description | Tier |
|---|---|---|
| Followed-by patterns | `e1=A[cond] -> e2=B[cond]` — non-contiguous temporal ordering; other events may occur between. | core |
| `every` repetition | Restart pattern matching per occurrence for continuous detection. | core |
| Logical patterns | `and` / `or` composition of pattern elements. | core |
| **Absence detection** | `not A for 10 sec`, `A and not B` — detecting what *didn't* happen (missing heartbeat, missing confirmation). Rare even among CEP engines; lead with it. | diff |
| Counting & Kleene | `<n:m>` occurrence counts in patterns; `*`, `+`, `?` quantifiers in sequences. | core |
| Strict sequences | `,` contiguous matching (no intervening events) for log/protocol analysis. | core |
| `within` time bounds | Overall time constraint on a pattern/sequence match. | core |
| Cross-event references | Conditions referencing earlier matches: `e2[price > e1.price]`, indexed access `e1[0]`, `e1[last]`. | core |
| Partitioned patterns | Pattern state automatically isolated per partition key (per user, per device). | core |
| Semantic pattern conditions | LLM and embedding predicates usable inside pattern element conditions — `e2=ToolCalls[llm:score('exfiltration-like?', payload) > 0.8]` (constructs defined in FA14). | diff |
| MATCH_RECOGNIZE surface | Optional standard-SQL-style pattern syntax as a second skin over the same engine, for SQL-literate users. | later |

### FA4 — Joins & enrichment

| Feature | Description | Tier |
|---|---|---|
| Stream–stream windowed joins | Inner / left / right / full outer, each side with its own window and filter, `on` condition. | core |
| Unidirectional joins | Only one side triggers output — the common enrich-on-arrival shape. | core |
| Stream–table joins | Enrich events against reference data (in-memory or external store). | core |
| Stream–named-window joins | Join live events against a shared window's contents. | core |
| Stream–aggregation joins | Join against time-series rollups with `within … per <granularity>`. | later |

### FA5 — Tables, state & on-demand queries

| Feature | Description | Tier |
|---|---|---|
| In-memory tables | `define table` with composite primary keys and secondary indexes (range-capable). | core |
| Stream-driven CRUD | `insert into`, `update … set … on`, `delete … on`, `update or insert` from query output. | core |
| External store SPI | Plug in Redis/Postgres/etc. as table backends; push conditions/selection down to the store where it can. | diff |
| Read-through cache | Bounded cache (FIFO/LRU/LFU) in front of store tables with preload and expiry. | later |
| On-demand queries | Synchronous request/response queries against tables, windows, and aggregations from the host app or management API — "what's the current state?" | core |

### FA6 — Incremental time-series aggregation

Pre-computed multi-granularity rollups — Siddhi's quietly loved feature. **v1 scope (decided 2026-10-31): minimal rollups only** — enough rolling aggregation (minutes/hours/days) to power FA13 behavioral baselines and velocity rules; the full sec→year framework with store persistence follows later.

| Feature | Description | Tier |
|---|---|---|
| Multi-granularity rollups | `define aggregation … every sec … year` — one definition maintains second→year rollups incrementally. We add `week` (Siddhi rejects it). | core (min/hour/day) · later (full) |
| Event-time aggregation | Aggregate by an event's own timestamp with out-of-order correction. | diff |
| Flexible retrieval | Query or join with `within <range> per <granularity>`; granularity can be a runtime expression. | diff |
| Retention & purging | Per-granularity retention with sane defaults; automatic purge. | diff |
| Persisted rollups | Back rollup tables with an external store for durability and size. | later |

### FA7 — Partitioning & parallelism

| Feature | Description | Tier |
|---|---|---|
| Value partitioning | `partition with (key of Stream)` — fully isolated rule state per key (per user, device, tenant). | core |
| Range partitioning | Expression-based partitions (`price < 100 as 'low' …`). | later |
| Partition state TTL | Evict state for idle keys (purge interval + idle period) — unbounded-cardinality safety. | core |
| Inner streams | `#stream` scoping to chain queries privately within a partition instance. | core |
| Per-stream async parallelism | Opt-in buffered async processing per stream (buffer size, workers, max batch) — Go-native equivalent of Siddhi's @Async ring buffer. | core |

### FA8 — Connectivity

Transport decoupled from format, Siddhi-style: sources/sinks + mappers.

**Boundary (decided 2026-10-31):** this repo ships the connector *framework* plus a lean built-in set. The broader adapter catalog — AI-gateway extensions, log exporters for logging systems, IAM-system plugins that emit events, database plugins, and the like — lives in **separate sibling repos in the Vorsar org**, written and versioned independently (the same model Siddhi used for its extension repos).

| Feature | Description | Tier |
|---|---|---|
| Source/sink framework | Declarative `@source`/`@sink` annotations on streams; multiple sources/sinks per stream; dynamic per-event sink options; `{{attr}}` payload templates. | core |
| Format mappers | JSON (core), CSV, text templates, Protobuf, Avro; custom attribute extraction incl. transport metadata (`trp:`-style). | core (JSON/CSV) / later (others) |
| v1 connector set | In-memory bus, stdin/stdout, file, HTTP (server + client), webhook sinks, then NATS/Kafka/gRPC. MQTT and other edge transports demoted to later — no AI-security angle (decided). | core (first 6) / diff (rest) |
| OTel-native connectivity | OTLP source and sink — the primary ingestion path for agent telemetry (GenAI semantic conventions). Must tolerate schema drift: the conventions are still "Development" status with no stabilization timeline. | core |
| Multi-destination sinks | Round-robin / broadcast / partitioned distribution with per-destination failure tracking. | later |
| Backpressure & retry | Source pause/resume; connect retry with exponential backoff; sink wait-and-retry mode. | core |

### FA9 — Reliability, persistence & error handling

| Feature | Description | Tier |
|---|---|---|
| State snapshots | Full snapshots of all stateful operators (windows, patterns, tables, aggregations, partitions) — snapshot/restore as byte blobs with revisions. | core |
| Incremental snapshots | Delta-based persistence over a base snapshot; point-in-time restore. | later |
| Staged startup | Start engine → restore state → then open sources; never consume before state is back. | core |
| Pluggable persistence stores | In-memory and filesystem built-in; SPI for S3/Redis/etc. | core |
| Per-point error policies | On error at stream/sink/table: log, wait-and-retry, route to fault stream, or capture to error store. | core |
| Fault streams | Auto-created `!stream` carrying the failed event plus error context — handle failures with the same rule language. | diff |
| Error store & replay | Pluggable store capturing mapping/transport/table failures with list/discard/replay APIs. | later |
| Delivery semantics | At-least-once end-to-end with idempotency guidance; documented honestly. | core |

### FA10 — Observability, testing & developer experience

Where we out-execute every incumbent.

| Feature | Description | Tier |
|---|---|---|
| Prometheus metrics | Throughput, latency, buffer depth, state size — per app/stream/query/table/source/sink, at off/basic/detail levels, switchable at runtime. | core |
| Playback mode | Engine clock driven by event timestamps — deterministic replay of historical events for backfill and reproducible debugging. | diff |
| Rule unit-testing framework | Given-events → expect-events testing as table-driven Go tests and a CLI runner; CI-friendly. Patterns and windows are testable deterministically via playback. | diff |
| Sandbox mode | Run an app with external connectors/stores stripped and replaced by in-memory equivalents — test business logic in isolation. | core |
| Validate-only mode | Parse + semantic-check an app without running it; errors with line/column and query context. | core |
| CLI & REPL | Run apps, validate, send test events, tail outputs, query state interactively. | core |
| OTel tracing | Optional trace of an event's path through queries — "why did this alert fire?" | later |
| Step debugger | Breakpoints at query in/out, step/continue, state inspection. | later |
| Docs generated from code | Extension metadata (params, examples) validated at build time and rendered into reference docs automatically. | later |

### FA11 — Extensibility

| Feature | Description | Tier |
|---|---|---|
| Go extension interfaces | Clean SPI for every extension type Siddhi has: function, aggregator, incremental aggregator, window, stream processor/function, source, sink, mappers, store, script. Namespaced invocation (`ns:fn`). | core |
| WASM user-defined functions | Sandboxed, polyglot UDFs — safe for multi-tenant and remotely distributed rules. Neither Siddhi nor eKuiper has this. | diff |
| Script functions | Inline functions in an embeddable scripting language via the script SPI. | later |
| Extension metadata & discovery | Self-describing extensions (name, params, examples) enabling validation, docs, and future tooling. | core |

### FA12 — Runtime & deployment modes

| Feature | Description | Tier |
|---|---|---|
| Embeddable Go library | Idiomatic Go API (contexts, channels/callbacks, no globals): create runtime → deploy apps → send events → receive outputs. The primary product. | core |
| Single static binary runner | `cee run rules/` — loads rule files + YAML config (connector credentials, named refs), runs them. Cross-compiled, scratch-image friendly, ARM64/AMD64. | core |
| Hot rule reload | Watch rule files / config for changes; deploy-undeploy without restart (state handling policy per change). | diff |
| Kubernetes-friendly service | Health/readiness endpoints, graceful drain, config-map-driven rules; operator/CRDs explicitly later. | core |
| Management API | REST/gRPC: deploy/undeploy apps, stats, on-demand queries, snapshot/restore triggers. | core |
| Scale-to-zero friendliness | Millisecond start + snapshot resume = viable in serverless/sidecar lifecycles. | diff |

### FA13 — Agentic AI detection & response pack ⭐ wedge

The vertical layer that turns the engine into "detection & response for AI agents." Everything here composes FA1–FA12 primitives — no new engine semantics required, which is exactly what makes it credible.

**Decided 2026-10-31:** the detection rule library is a **first-class, versioned product line with a maintenance commitment** — the content moat (engines get copied; curated detections compound, like Sigma rules or Falco policies). Boundary with the Vorsar platform: event schemas, detections, connectors, and response sinks live with the engine; the investigation/SOC experience lives in the platform. Pack detections declare whether they use LLM judgment (FA14) and ship deterministic-only variants where feasible, so packs stay useful in deployments with no LLM provider. Packs are **Apache-2, like the engine** (decided 2026-10-31) — monetization rests on the Vorsar platform, not the content.

| Feature | Description | Tier |
|---|---|---|
| GenAI telemetry ingestion | Source mappers for OTel GenAI semantic conventions (LLM/tool-call spans), AI-gateway logs (OpenAI-compatible proxies and major gateways), and MCP audit events — agent activity becomes typed streams out of the box. | diff |
| Agent identity & context model | Canonical event schemas keyed by agent id, session, user, tenant, and delegation chain; reference tables for agent registries, tool allowlists, and data-sensitivity labels. | diff |
| Detection rule library | Curated, versioned pack of cross-agent detections — exfiltration sequences, delegation abuse, RAG/memory grooming, identity-velocity anomalies, gone-dark absences — shipped as rule files users enable, tune, and extend. | diff |
| Response sinks | Machine-speed enforcement as ordinary sinks: gateway block/quarantine calls, token-revocation webhooks, step-up human-approval requests, incident events to SIEM/SOAR. | diff |
| Behavioral baselines | Per-agent rolling profiles (token spend, tool-call mix, destination set) via incremental aggregations, used as dynamic thresholds inside rules. **Note: this pulls part of FA6 into earlier scope than previously assumed.** | diff |
| Forensic detections | Every detection event carries the matched event chain (which events, which agents, when) for triage, audit, and replay. | diff |

### FA14 — LLM-in-the-loop detection ⭐

Deterministic rules catch what you can specify; many agentic attacks are **semantic** ("does this prompt look like grooming?", "is this sequence consistent with the agent's stated task?"). The language therefore treats LLM judgment as a first-class primitive — with the economics designed in. An LLM cannot be called on every event at machine speed, so the product pattern is **CEP narrows, LLM judges** (design principle 7): deterministic patterns pre-filter candidates on every event; LLM verdicts confirm, enrich, or dismiss the matches.

**Everything in FA14 is strictly optional.** Not every rule needs an LLM judge; users opt in per construct, per rule.

| Feature | Description | Tier |
|---|---|---|
| Optional by construction | A deployment with no LLM provider configured loses nothing from FA1–FA13; rules that use no LLM construct never touch a model; deploy-time validation (not match-time failure) catches rules that reference a judge where no provider exists. | core |
| LLM scalar functions | `llm:classify(text, labels)`, `llm:score(instruction, content)`, `llm:extract(schema, text)` — callable anywhere an expression goes: filters, projections, pattern conditions. | diff |
| Two-stage judge on matches | Attach a judge to a query/pattern: on a deterministic match, the full matched event chain plus a rule-authored verdict prompt goes to an LLM; the verdict (label, confidence, rationale) becomes attributes of the output event and gates response sinks — "quarantine only if verdict ≥ 0.8, else alert for human triage." | diff ⭐ |
| Async judgment semantics | LLM calls are slow and can fail, so the language carries per-rule timeout, fallback policy (fail-open vs. fail-closed), and provisional-then-confirmed emission — fast containment actions are never blocked by a pending judgment. | core (with any LLM feature) |
| Semantic similarity primitives | `embed(text)` plus vector similarity in filters and joins; vector-capable reference tables (known attack embeddings, approved task descriptions) enable `sim(…) > θ` checks — semantic detection at near-deterministic cost, no per-event LLM call. | diff |
| Cost & budget controls | Per-rule and per-app token/call budgets, rate limits, caching of identical judgments, and sampling ("judge 1 in N matches"); budget consumption surfaces as FA10 metrics. | core (with any LLM feature) |
| Deterministic testing of LLM rules | Judge calls are recordable fixtures, replayed in playback mode and the rule unit-test framework — rules with LLM constructs stay CI-testable; judge prompts are versioned artifacts shipped inside detection packs (FA13). | core (with any LLM feature) |
| LLM provider extension type | Providers plug in like sources/sinks: OpenAI-compatible APIs, gateway-routed (reuse the enterprise's own AI gateway and its controls), and local models for air-gapped deployments. | diff |
| Judge self-protection | The judge treats judged content strictly as data (injection-hardened prompt templates), and judge calls are themselves observable events — budgeted, logged, and correlatable by the engine's own rules. | diff |

---

## 9. Success metrics (draft)

- **North star:** weekly active engines (opt-in anonymous ping) — proxies for production embeddings.
- **DX metric:** time-to-first-detection < 15 minutes from `go get` (quickstart measured, not hoped).
- **Adoption:** module downloads, binary/Docker pulls, distinct importing repos (public GitHub).
- **Community:** contributors with merged PRs, extension packages published by third parties.
- **Quality:** pattern-semantics conformance suite passing 100%; zero known correctness bugs open > 30 days.

## 10. Decisions (settled with product owner, 2026-10-31)

1. **Primary wedge: agentic AI security.** Attack detection across agent fleets is the mission; traditional CEP use cases (IoT/edge, fraud) only where an AI angle exists.
2. **Scope: full pivot in mission, component in structure.** The Vorsar org's main product is the AI-agent detection & response platform; this repo is its CEP core engine, prioritized exclusively for that mission while keeping general-purpose mechanics.
3. **Dialect: modernized Siddhi-inspired streaming SQL.** No SiddhiQL compatibility promise; fix known quirks (hopping window, week granularity, nested-query gap).
4. **Headline interface: Go API-first.** The library leads the README/docs; the DSL is the shareable, hot-reloadable ops format.
5. **Name: keep `complex-events-engine`** until a requirement forces a change — the brand belongs to the Vorsar platform, not this component. *(Amended 2026-11-02, owner decision at S1.1a-1's design step: "package names should reflect vorsar". The Go module path is **`github.com/vorsar/cep`**, the repository is renamed `VORSAR/cep`, and the gate's local names are `~/.cache/vorsar/cep/` and the `VORSAR_CEP_` prefix. Go package names stay short (`cep`, `rule`, `cepio`, `internal/*`). The documents keep "complex-events-engine" as the project's descriptive name.)*
6. **Incremental aggregation: minimal rollups in v1** (minutes/hours/days, enough for per-agent behavioral baselines); full framework later.
7. **WASM UDFs: fast-follow after v1**, once the native Go extension SPI stabilizes.
8. **Detection rule packs: first-class versioned product line** with a maintenance commitment.
9. **LLM-in-the-loop detection is a feature area (FA14), not an afterthought** (added 2026-10-31): the rule language gets LLM judgment constructs — scalar functions, a two-stage judge over matched event chains, embedding similarity — governed by the "CEP narrows, LLM judges" principle, with async semantics, budget controls, and replayable judge fixtures so LLM rules remain deterministic in CI. **Strictly optional, per-rule opt-in** (clarified by owner, 2026-10-31): not everything needs an LLM judge — the engine and all other feature areas are fully functional with no LLM provider configured.

10. **Adapter ecosystem lives in sibling repos** in the Vorsar org — AI-gateway extensions, log exporters, IAM-system plugins, database plugins, etc., each its own repo. This engine repo ships the connector framework and a lean built-in set only (Siddhi's extension-repo model).
11. **Everything Apache-2** — the engine *and* the detection packs. Monetization rests entirely on the Vorsar platform.
12. **Public OSS project** with its own community ambitions — README/docs site/releases/community are real work items, and adoption metrics (active engines, external contributors) are first-class objectives, not vanity.
13. **v1 is quality-driven, no calendar commitment.** The roadmap is ordered by dependency, not deadline — correctness-first (per design principle 2).

### Still open (carry into the objectives doc)

- **Team capacity — resolved 2026-10-31:** one human developer (the product owner) orchestrating **waves of coding agents**. Plans are shaped for that model: work decomposed into well-specified, independently verifiable, parallelizable tasks with strong test gates; scope discipline comes from the core/diff/later tier system.
- **Competitive watch (process):** Zenity ($125M Series C, Aug 2026) is one credible sprint from making its correlation genuinely real-time; Palo Alto owns both a gateway (Portkey) and runtime security (Prisma AIRS). Monitor quarterly.

## 11. What happens next

1. Freeze this into **high-level objectives** (next doc: `01-objectives.md`).
2. Derive the **roadmap** (feature areas → releases) and **sprint plan**.
