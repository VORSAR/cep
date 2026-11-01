# Roadmap — complex-events-engine

**Status:** v1.0 · 2026-10-31 · derived from [01-objectives.md](01-objectives.md)
**Ordering:** by dependency, not by date (quality-driven, decision #13). Phases ship when their exit criteria pass, executed as coding-agent waves gated by tests.
**MVP strategy (owner, 2026-10-31):** pick one real use case where the CEP engine detects a problem and emits a response event that disables an agent (IAM webhook + escalation email), and build *everything required to ship that use case* — a vertical slice through every layer.

---

## Phase 1 — MVP: the "Agent Kill Switch" vertical slice

### The use case (chosen)

> **Sensitive read → unapproved egress → kill switch.**
> An enterprise AI agent reads from a sensitivity-labeled resource, then within 5 minutes makes an outbound tool call to a destination not on the approved list, **and no human-approval event arrives in between** → the engine emits a `AgentKillSwitch` event carrying the full evidence chain, which (a) calls an IAM system's webhook to disable the agent identity, and (b) sends an escalation email to the security team.

Why this one: it is the Salesloft-Drift-shaped attack (every call individually legitimate; only the sequence is damning); it requires the engine's three differentiating constructs at once — **followed-by, absence detection, `within`** — plus per-agent partitioned state; and it needs *no* windows, joins, aggregations, or LLM, which keeps the slice honest and thin. *(Amended 2026-10-31, owner: four windows — `sort`, `cron`, `delay`, `batch` — added to MVP scope; see the FA2 row below. The kill-switch rule itself still uses none.)* Alternates considered and set aside for the pack later: identity-velocity anomaly (needs baselines/FA6), gone-dark heartbeat (too trivial to headline).

### Illustrative rule (syntax not final)

```
define stream AgentActions (agentId string, sessionId string, action string,
                            resource string, destination string);
define stream HumanApprovals (agentId string, sessionId string, approvedAction string);
-- (amended 2026-10-31: agentId added — every stream consumed in a partition block
--  must carry the partition key; red-team finding RT-08)
define table SensitiveResources (resource string, label string);
define table ApprovedDestinations (destination string);

@sink(type='http', url='${IAM_WEBHOOK}', @map(type='json'))
@sink(type='email', to='${SECOPS_DL}', subject='Agent {{agentId}} disabled',
      @map(type='text'))
define stream AgentKillSwitch (agentId string, sessionId string,
                               reason string, evidence object);

partition with (agentId of AgentActions, agentId of HumanApprovals)
begin
  from every e1=AgentActions[action == 'read' and resource in SensitiveResources]
       -> e2=AgentActions[sessionId == e1.sessionId
                          and not (destination in ApprovedDestinations)]
          and not HumanApprovals[sessionId == e1.sessionId] for 5 min
       within 5 min
  select e1.agentId, e1.sessionId,
         'sensitive-read-then-unapproved-egress' as reason,
         chain(e1, e2) as evidence
  output first every 10 min
  insert into AgentKillSwitch;
end;
```

The same rule must be expressible through the Go builder API (headline interface, O2) with guaranteed parity.

### Everything required (the slice, by feature area)

| Layer | In the MVP | Explicitly NOT in the MVP |
|---|---|---|
| Event model & language (FA1) | Typed streams; filters; `in` table membership; select/insert; time constants; `${VAR}` substitution; a starter set of scalar functions; **text operators** (term match, contains/prefix/suffix, keyword-set any/all, safe regex — decided 2026-10-31); `output first every T` (alert-storm suppression); deploy-time validation with line/col errors | group by/having/order/limit; triggers; most functions; other rate-limit modes |
| Windows (FA2) *(added 2026-10-31, owner)* | Window clause + `sort`, `cron`, `delay`, `batch` windows with their emission semantics and conformance tests | Time-based catalog: tumbling/hopping/sliding/session etc. (Phase 2) |
| DSL + Go API | Parser for the subset above; Go builder with full parity for the same subset | Everything outside the subset |
| Patterns (FA3) | `every`, followed-by, logical `and`, **absence (`not … for`)**, `within`, cross-event references, evidence chain on match | Counting/Kleene, strict sequences, `or` patterns |
| Partitions (FA7) | Value partitioning by key; idle-key TTL purge | Range partitions, inner streams |
| Tables (FA5) | In-memory tables, primary key, `in` checks, programmatic load/update from Go | Secondary indexes, external stores, cache, on-demand queries |
| Connectivity (FA8) | Sources: HTTP (JSON), in-memory, stdin (replay/demo). Sinks: HTTP webhook (JSON, `{{attr}}` templates), SMTP email, log. Connect retry + backoff | Everything else (NATS/Kafka/gRPC/OTel land post-MVP; adapter catalog is sibling repos) |
| Reliability (FA9) | `@OnError` log; sink retry/backoff; honest docs: pattern state does NOT survive restart in MVP | Snapshots/restore (first item of Phase 2), fault streams, error store |
| Testing & DX (FA10/O1/O6) | **Playback mode first** (event-time clock — the test substrate); conformance tests for every shipped construct; rule unit-test harness (given→expect); quickstart measured <15 min; basic Prometheus metrics | REPL, sandbox, debugger, tracing |
| Runtime (FA12) | Embeddable Go library; minimal binary runner (rule file + YAML config); health endpoint | Hot reload, management API, k8s posture |
| Demo kit | Docker-compose scenario: fake agent traffic generator, stub IAM endpoint, mail catcher — the attack fires, the agent dies, the email arrives | — |
| Attack gallery *(added 2026-10-31, owner)* | Documented, runnable examples inspired by real cited 2025–26 incidents (from [the attack-landscape review](../language/review/attack-validation.md)): kill switch + 5–6 MVP-expressible attack shapes, each = plain-English attack story → synthetic event timeline → rule in both DSL and Go API → playback fixture run in CI as an acceptance test. Rules: synthetic telemetry only; factual, cited, defensive framing ("detects this attack shape"); each example states honestly when real deployments need Phase-3 telemetry to feed it | Examples needing P2/P3 constructs (fan-out, velocity, anti-join) — listed in the gallery as "coming", not faked |

### Honest-scope note *(added 2026-10-31, attack-landscape review)*

The MVP detects **per-agent** temporal attacks — the primitive the kill switch demonstrates. **Fleet-wide correlation (fan-out: one actor/token across many agents or tenants — the defining signature of Salesloft-Drift-class and worm-class attacks) is a Phase 2–3 capability** (group-by + rollups + identity schema), not MVP. Public MVP material must not claim cross-agent correlation yet; the identity-velocity/fan-out detections in Phase 3's pack are the true realization of the "fleet" claim.

### Exit criteria (definition of "shipped")

1. The demo scenario runs end-to-end from the public quickstart in under 15 minutes.
2. Conformance + unit tests green for every shipped construct; detection-to-kill-switch latency measured and published.
3. Docs cover the shipped subset only (no aspirational docs); tagged release `v0.1.0`, public repo.
4. The same detection authored both ways (DSL and Go API) with identical behavior, proven by one test.
5. *(added 2026-10-31, owner)* The attack gallery ships with v0.1.0: ≥5 incident-inspired documented examples, each cited, runnable via playback, and green in CI.

## Phase 2 — Correctness & durability breadth *(unblocked by: Phase 1 engine skeleton)*

**First in phase: minimal count-over-rolling-window per key** *(front-loaded 2026-10-31 — the attack-landscape review found rolling count/velocity is the primitive behind the most severe real incidents (machine-speed campaigns, worm propagation, bulk exfiltration) and the second-most-needed capability across the 2025–26 catalog)*. Then: state snapshots/restore + staged startup (kill-and-resume tests make restart semantics honest); window catalog (time, timeBatch, hopping, length, session — `sort`/`cron`/`delay`/`batch` moved into MVP 2026-10-31; `externalTime` dropped: event time is the default clock); stream–stream and stream–table joins (incl. **anti-join / "not preceded by"** — e.g. a publish with no prior CI build, the Shai-Hulud signature); counting/Kleene + strict sequences + `or`/absence combinations; group by/having/order/limit; triggers; full conformance suite breadth; sandbox mode; fault streams + error store.

## Phase 3 — Detection pack v1 & agent telemetry *(unblocked by: Phase 2 windows/joins; partially parallel with it)*

Canonical agent-event schemas (agent/session/user/tenant/delegation chain); OTel GenAI-tolerant ingestion + NATS/Kafka sources — **reframed 2026-10-31: telemetry ingestion is a detection *precondition*, not a convenience: roughly a third of documented 2025–26 agent attacks run partly or wholly provider-side and are invisible without provider/connector logs; docs must carry an honest visibility-limits statement** (fully provider-side exfiltration, e.g. ShadowLeak-class, is out of reach without provider cooperation); minimal rolling rollups (FA6: min/hour/day) powering behavioral baselines, **first-seen/novelty state (new destination/ASN/capability per identity), and rollup-backed long-horizon dwell detection (days/weeks) so long bounds don't force unbounded per-key TTL state**; detection pack v1 (Apache-2, versioned): exfil sequences, delegation abuse, RAG grooming, identity velocity/fan-out, gone-dark — each with deterministic variants and forensic chains; more response sinks (SIEM/SOAR events, step-up approval). **Named non-goal** *(2026-10-31)*: arbitrary-depth delegation-chain (graph) traversal — fixed small depths are expressible as patterns; unbounded graph reasoning belongs to the Vorsar platform, and the pack must not imply it.

## Phase 4 — LLM-in-the-loop (FA14) *(unblocked by: Phase 2 async plumbing + Phase 3 pack to attach judges to)*

`llm:` scalar functions; two-stage judge over matched chains with verdict-gated sinks; async semantics (timeouts, fail-open/closed, provisional-then-confirmed); budgets/caching/sampling; replayable judge fixtures in CI; provider extension type (OpenAI-compatible, gateway-routed, local). Optional by construction throughout.

## Phase 5 — OSS launch & extensibility *(unblocked by: SPI stability from Phases 2–4)*

Docs site with generated language reference; extension SPI frozen + contribution guide; hot rule reload; management API; REPL; adoption ping (opt-in); WASM UDFs (fast-follow decision #7); k8s posture polish. "Launch" = announcing what already works.

---

## Standing tracks (every phase)

- **Tests-are-the-spec:** no agent-wave output merges without conformance/unit gates (operating model, 01-objectives).
- **Competitive watch:** quarterly — Zenity real-time correlation, Palo Alto (Portkey + Prisma AIRS).
- **Scope guardrail:** anything not serving the current phase's exit criteria waits, per the core/diff/later tiers.

## Next

- [ ] ~~`03-sprint-plan.md`~~ [`../sprints/sprint-plan.md`](../sprints/sprint-plan.md) — Phase 1 decomposed into agent-wave batches with verification gates. *(Amended 2026-10-31: the sprint plan moved to `docs/sprints/` with the adopted sprint process, [00-sprint-model.md](../sprints/00-sprint-model.md).)*
