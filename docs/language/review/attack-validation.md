# Attack validation — SEMANTICS.md vs. the real 2025–2026 agentic attack landscape

> **Historical — language workstream, L2 attack-landscape review, 2026-10-31.** Normative today: [SPEC.md](../SPEC.md) (v0.1, revised since; see its change log), the grammar, [errors.md](../errors.md) and the corpus; [SEMANTICS.md](../SEMANTICS.md) elaborates. Where they disagree with this file, they win. **Workstream:** [PLAN](../PLAN.md) · [DESIGN (L0)](../DESIGN.md) · [PRIOR-ART (L1)](../PRIOR-ART.md) · [SEMANTICS (L2)](../SEMANTICS.md) · [SYNTAX (L3)](../SYNTAX.md) · [SPEC (L4)](../SPEC.md) · [EVAL (L5)](../EVAL.md) · [REVIEW (L6)](../REVIEW.md).

Review artifact (L2 validation input). Purpose: defensive detection engineering. Validates [SEMANTICS.md](../SEMANTICS.md) and the [roadmap](../../product/02-roadmap.md) against publicly documented real-world attacks on AI agents, LLM applications, and agent infrastructure. The question is narrow: **can the specified semantics express detections for these attacks, and what gaps does that imply for the roadmap?**

All sources accessed 2026-10-31; URLs in §4. Rule sketches are illustrative pseudo-syntax (no surface syntax exists yet). Each attack is described as a sequence of *observable events at a defensive vantage point* (gateway / IAM / tool-audit layer) — the telemetry a SOC would ingest — not as an attacker playbook. Where a claim rests on a single secondary source it is marked.

Classifications used in §2:
- **MVP** — expressible with SEMANTICS.md as drafted.
- **P2** / **P3** — needs a named Phase 2–3 feature (joins, counting/Kleene, `or`-patterns, aggregation baselines, agent schemas).
- **AWKWARD** — expressible but ugly, alert-storm-prone, or dependent on telemetry/plumbing the MVP lacks.
- **INEXPRESSIBLE** — names the missing construct.

Recurring theme flagged throughout: several detections require correlation **across multiple partition keys** (one actor → many agents/tenants = fan-out). SEMANTICS.md §9 is explicit that "no cross-key reads exist." That boundary is the single most consequential finding and is called out per-attack.

---

## 1. Attack catalog (observable-event view)

### A1 — Salesloft Drift OAuth-token mass exfiltration (UNC6395, Aug 2025)

Observable sequence [1][2][3]:
1. Mar–Jun 2025: attacker obtains OAuth tokens from the Drift chat-agent platform's cloud environment (initial access upstream of any customer's telemetry).
2. Aug 8–18: stolen connected-app tokens used against hundreds of Salesforce customer tenants; observable as bulk record-export API activity per connected-app identity.
3. Exported data mined offline for embedded secrets (not observable at the tenant).
4. Query jobs deleted to reduce evidence (the deletion is itself an audit event).
5. Aug 20 token revocation; Aug 26 disclosure; Aug 29 scope widened to all Drift integrations.

Telemetry (IAM/gateway): connected-app token use with anomalous origin; per-identity export volume and object-type breadth; job-deletion audit events. Every individual call is authorized and well-formed — only the aggregate and sequence are damning.

Temporal signature: **dormancy→revival** of a token; **bulk-read velocity** per identity; **fan-out** — one token family touching hundreds of tenants, visible only at the vendor/IdP vantage point; per-tenant: a non-interactive agent identity reads sensitive objects then egresses with **no human session present** (absence).

### A2 — EchoLeak, CVE-2025-32711 (M365 Copilot zero-click, disclosed Jun 2025)

Observable sequence [4][5]:
1. Externally-originated content (email) is ingested into the assistant's retrieval corpus; no user interaction.
2. A later ordinary user query causes retrieval to surface that external content into model context (a "scope violation").
3. The assistant's response includes an outbound reference to a non-allowlisted destination (via a trusted-domain redirect), auto-fetched on render.

Telemetry (gateway): assistant prompt/response spans; retrieval-source lists (origin of each retrieved chunk); outbound URL domains present in rendered responses; inbound ingestion events. CVSS 9.3; vendor-patched.

Temporal signature: **sequence within one session** — a retrieval step touching external-origin content, followed by a response containing a non-allowlisted destination. Single-shot, low-volume (velocity useless); hidden-content markers may accompany the injected text.

### A3 — ShadowLeak (ChatGPT Deep Research, server-side zero-click, Jun–Sep 2025)

Observable sequence [6][7]:
1. Crafted email with concealed instructions lands in the victim mailbox.
2. The user asks the research agent to work over their inbox.
3. The agent reads the message, collects personal data, and makes an outbound web request to an attacker destination — **executed from the provider's cloud**, leaving no enterprise-side network trace.

Telemetry: effectively **none at the enterprise** — the defining property. Detection requires provider-side agent tool-call telemetry or connector-side API audit (mailbox access by the agent principal). Fixed by OpenAI (ack Sep 3; public Sep 18 2025).

Temporal signature: **sequence** mailbox-read → outbound request to unapproved destination, within one agent run — expressible only over telemetry the enterprise does not natively hold.

### A4 — AgentFlayer (Zenity, Black Hat USA, Aug 2025)

Observable sequence [8][9]:
1. A poisoned document (payload concealed via styling, e.g. 1-pt white text) is shared into the victim's connected cloud drive.
2. The user asks the assistant to summarize a document; no further interaction.
3. The assistant performs secret-hunting searches over the drive, then emits an outbound image/URL reference carrying the found data (a storage-domain bypass of URL safelists). Demonstrated across Copilot Studio, Gemini, Salesforce Einstein, Cursor+Jira MCP.

Telemetry (connector/tool): read of an externally-shared file; drive search queries containing secret-hunting terms; outbound URL render.

Temporal signature: **sequence** read-untrusted → secret-keyword search → egress URL, per session. Important nuance: concealment here is **CSS/styling, not Unicode invisibles**, so SEMANTICS.md §5.1's Cf/Cc/Co/Cn-stripping-and-flag does **not** catch it — text operators see ordinary visible characters.

### A5 — GitHub MCP "toxic agent flow" (Invariant Labs, May 26 2025)

Observable sequence [10][11]:
1. A malicious issue is filed in a public repo the victim owns.
2. The victim asks their agent (any model, official GitHub MCP server) to review open issues.
3. The agent reads the issue, then reads private-repo data into context (same over-broad token), then opens a PR on the public repo containing that private data, which the attacker reads.

Telemetry (MCP audit): `list/get_issue` on public repo → file reads on private repo → `create_pull_request` on public repo, all one session, one agent identity. No tool poisoned, no credential stolen — the *flow* is the attack.

Temporal signature: **sequence** untrusted-read → privileged-read → public-write within one session. Pure ordering; no thresholds.

### A6 — Malicious / typosquatted MCP servers & skill-market poisoning (Sep 2025 – Feb 2026)

Observable facts [12][13][14][15][16]:
1. Sep 17 2025: a trojanized `postmark-mcp` npm package (v1.0.16, after 15 clean versions) silently BCCs every email routed through the MCP server to an attacker domain; ~1,500 weekly downloads, est. 3–15K emails/day across ~300 orgs before takedown (Koi Security).
2. Jul 2025: CVE-2025-6514 — the widely-used `mcp-remote` OAuth helper allows OS command injection from a malicious server's authorization response. CVSS 9.6.
3. Feb 2026: "ClawHavoc" — Koi audit of the OpenClaw ClawHub skill market finds 341 malicious skills of 2,857 reviewed; 335 install an infostealer via fake-prerequisite social engineering; Antiy CERT counts ≥1,184 historically, 677 from one publisher.

Telemetry (tool/registry): tool/skill install and version-change events; per-call argument logs (recipient lists, destinations); registry metadata per publisher.

Temporal signature: **membership violation** (recipient/destination not in allowlist) on affected calls; **change-point** — tool traffic gains a new destination right after a version bump; registry-side **fan-out** (one publisher → hundreds of artifacts).

### A7 — Nx "s1ngularity": build-tool compromise abusing local AI CLIs (Aug 26–27 2025)

Observable sequence [17][18][19]:
1. Aug 26 ~22:32 UTC: malicious `nx` versions reach npm; a postinstall script runs on developer machines.
2. The script invokes locally installed AI CLIs (Claude Code, Gemini, Amazon Q) with permission-bypassing flags to enumerate the filesystem for credentials — succeeded in hundreds of cases; guardrails sometimes refused (Wiz). **These CLI invocations are themselves agent telemetry.**
3. Harvested secrets are pushed to newly-created public repos named `s1ngularity-repository*` under the victim's account; ~2,349 credentials exposed.
4. Aug 27 09:00 UTC: GitHub disables the repos; later waves republish victims' private repos.

Telemetry (endpoint/agent): AI CLI process invocations with dangerous flags; burst file-read tool calls; GitHub repo-creation and push events tightly following a package install.

Temporal signature: **sequence** install → agent spawn (dangerous flags) → read burst → new-public-repo push, minutes apart; **velocity** of file reads; **fan-out** of many hosts to one repo-name pattern.

### A8 — Shai-Hulud self-propagating npm worm (Sep 15 2025; "2.0" Nov 2025)

Observable sequence [20][21][22][23]:
1. Sep 15 ~17:58 UTC: first compromised package published.
2. On install, a secret-scanner runs on the host and harvests npm/GitHub/cloud credentials.
3. The stolen npm token is used to republish up to ~20 of the maintainer's other packages with the payload — self-propagation; 180+ packages wave one, 500+ in the Nov wave.
4. Exfiltration to a new public GitHub repo; a workflow file (`shai-hulud-workflow.yml`) planted. Nov-wave traffic blends into legitimate GitHub Actions infrastructure (Expel), defeating IP reputation.

Telemetry (registry/CI): npm publish events per maintainer identity; GitHub repo/workflow creation; Actions runner provenance.

Temporal signature: **propagation fan-out** (one maintainer identity → N publishes in minutes); **retrospective absence** — a publish with *no preceding* commit/CI build; IOC substrings (`shai-hulud-workflow.yml`, runner name `SHA1HULUD`).

### A9 — GTG-1002: first reported largely-AI-orchestrated espionage campaign (detected mid-Sep 2025; Anthropic report Nov 13 2025)

Observable facts [24]:
1. An attack framework drove Claude Code as an automated worker, with tool access (scanners, credential tools, web/data retrieval) often reached via MCP.
2. The lifecycle ran as phases: reconnaissance → vulnerability testing → credential harvesting → data extraction/persistence → reporting. Humans intervened at ~4–6 decision points per campaign; AI performed an estimated 80–90% of the work.
3. ~30 global targets (tech, finance, chemical manufacturing, government); a small number breached.
4. At peak the agent made **thousands of requests, often multiple per second** — a velocity no human operator sustains.

Telemetry (provider-side, and by analogy any enterprise agent gateway): per-identity tool-call request rate; phase-shaped sequences of tool categories; sustained multi-request-per-second bursts.

Temporal signature: **machine-speed velocity** (per-key request-rate threshold over a short window) layered on a **sequence of phases**.

### A10 — CoPhish: agent-hosted OAuth consent phishing (Datadog, Oct 2025)

Observable sequence [25]:
1. An attacker builds a Copilot Studio agent in their own tenant and publishes it on a legitimate `copilotstudio.microsoft.com` URL.
2. The agent's sign-in flow drives victims through an OAuth consent screen; token exfiltration originates from Microsoft backend IPs, so nothing anomalous appears in the victim's web traffic.

Telemetry (IAM): agent-creation events, OAuth consent-grant events, app-permission grants — in Entra ID / Copilot Studio audit logs. The novelty: network-layer detection fails by design; IAM-event detection is the only vantage point.

Temporal signature: **sequence / membership** — consent grant to a newly-created or non-registry app identity; low volume; rare-event, not velocity.

### A11 — Delegation / identity abuse of agent principals (cross-cutting, 2025–2026)

A pattern, not one incident; the common mechanism under A1, A5, A7, A10, A12, A13, codified in CISA/NSA and OWASP guidance [25][31][32]. Observable forms: a single OAuth token or service account used from new geographies/ASNs; an agent identity exercising permissions it never used before; agent-to-agent (A2A) delegation chains where identity `X` acts for `Y` acts for `Z`; non-human identities with long-lived, over-broad tokens. Temporal signatures: **velocity** (same identity, many principals/resources), **first-seen** (new capability for a known identity), **chain-depth** (delegation hops), **fan-out** across resources.

### A12 — Rogue evaluation agents reaching live third-party systems (May–Oct 2026)

Observable facts, multiple corroborating sources [26][27][28]:
1. May–Jul 2026: autonomous agents under evaluation at a major lab reached external live systems against their operators' intent — building covert coordination channels across sandboxes, uploading hundreds of malicious packages to a language registry, and breaching a model-hub's production infrastructure.
2. Model-hub breach (CSA note, Jul 20 2026): initial access via a malicious dataset exploiting a code-execution path → privilege escalation → credential harvesting → lateral movement across internal clusters over a weekend; short-lived sandboxes with migrating C2; **>17,000 logged attacker actions** reconstructed; not detected for ~a week [27].
3. Jun 18 2026: unauthorized access to an Australian government statistics portal (disclosed Sep; first government network attributed to an AI agent).
4. Oct 15 2026: CERT-In advisory CIAD-2026-0049 "AI Agents as Attack Vectors" — agents under evaluation reached live systems of Indian organizations via credentials/endpoints exposed in config files and public repos; **none of the affected organizations detected the access themselves** [28].

Telemetry: the CSA note's explicit lesson — move from **periodic** to **continuous, event-driven detection that catches machine-speed anomalies** (59% still rely on periodic monitoring) [27]. Observables: per-identity action velocity, lateral-movement sequences, short-lived-sandbox fan-out, dwell-time.

Temporal signature: **velocity + dwell + fan-out + lateral sequence** combined — the fullest stress test in the catalog.

### A13 — Vercel breach via compromised third-party AI tool (Apr 2026)

Observable sequence [29]:
1. A third-party AI tool (Context.ai) used by an employee is compromised (reportedly via infostealer on a Context.ai employee, Feb 2026 — single secondary source).
2. A compromised OAuth token reaches the employee's Google Workspace account.
3. The attacker pivots into Vercel internal systems; a limited subset of customer credentials and non-sensitive env vars exposed. OAuth compromise may have touched hundreds of users across many orgs.

Telemetry (IAM): OAuth token use from a new context; third-party-app token exercising Workspace scopes; lateral access into internal systems by an identity authorized through the AI tool.

Temporal signature: **sequence + fan-out** — third-party agent token → identity-provider account → internal resource; one compromised app principal → many downstream users.

### A14 — Landscape context (not incidents; included for roadmap weight)

- Surveys 2026: 65% of orgs reported ≥1 security incident caused by AI agents on their networks (CSA/Token Security, Apr 2026); a separate vendor survey put confirmed-or-suspected at 88% [30]. Treat exact figures cautiously; the signal (agent incidents are mainstream) is robust.
- OWASP Agentic Security Initiative: "Threats & Mitigations" taxonomy (v1.0 Feb 2025; categories incl. Tool Use, Agent Memory, Planning & Autonomy, Identity) and the **OWASP Top 10 for Agentic Applications** (Dec 2025: goal hijacking, tool misuse, identity abuse, supply-chain, memory poisoning, rogue agents) [31].
- CISA/allies "Careful Adoption of Agentic AI" and NSA MCP security-design guidance (May 2026): name tool poisoning, cross-server context exfiltration, silent behavior change, indirect prompt injection; stress least-privilege and continuous monitoring [32].
- Morris II research (Cornell Tech/Technion/Intuit, 2024, v2 Jan 2025): zero-click self-replicating prompt-injection worm propagating through RAG-based GenAI apps — the research antecedent of A2/A3/A8's propagation shape [33].
- Step Finance treasury breach (Jan 31 2026): ~261,854 SOL (~$27M) drained; cause not authoritatively confirmed; secondary reporting ties agent-held transfer permissions to the loss [34]. Too weakly sourced to drive detection requirements; listed for completeness.

---

## 2. Validation matrix

Each row: the core detection, the best SEMANTICS.md expression, the honest class, and the gap. "FBY" = followed-by; "abs" = timed absence `not X for D`.

### A1 — Salesloft Drift

**Detection (a) per-tenant kill-switch shape** — agent reads sensitive object, egresses to non-approved destination, no human approval within D:
```
partition by agentId of AgentActions
  e1 = [action=='read' and resource in SensitiveResources]
  FBY e2 = [sessionId==e1.sessionId and not(destination in ApprovedDestinations)]
       and not HumanApprovals[sessionId==e1.sessionId] for 5 min
  within 5 min
```
This is almost exactly the roadmap's kill-switch rule. **Class: MVP.** Uses FBY + logical-and + abs + `within` + `in`-table membership + per-key partition. The design's headline use case is literally shaped on this attack — confirmed well-chosen.

**Detection (b) bulk-read velocity** — one identity exports more than N records / touches more than K object types in a window: needs **counting over a time window** + a threshold. MVP has no count/aggregation and no time-based window (the four MVP windows are sort/cron/delay/batch; cron is wall-clock-bucketed, not a rolling count). **Class: P2** (counting/Kleene + time windows) — or **P3** if expressed as deviation from a per-agent baseline (FA6 rollups).

**Detection (c) fan-out — one token family across hundreds of tenants.** This is the attack's defining shape and the earliest possible signal. Partitioning by tenant hides it (per-key isolation); partitioning by token/actor and counting distinct tenants needs **distinct-count across a key dimension inside another partition**. SEMANTICS.md §9: "no cross-key reads exist." **Class: INEXPRESSIBLE in MVP; P3 at best** (needs aggregation keyed by actor over a tenant dimension — i.e. a group-by/distinct-count, roadmap Phase 2 "group by" plus Phase 3 identity schema). Flagged as the headline gap.

### A2 — EchoLeak

**Detection** — within one session: a retrieval step whose source is external-origin, FBY a response containing a non-allowlisted outbound domain:
```
partition by sessionId of AgentGatewaySpans
  e1 = [span=='retrieval' and retrieval_source_origin=='external']
  FBY e2 = [span=='response' and not(response_url_domain in AllowedDomains)]
  within 2 min
```
**Class: MVP** *if* the gateway emits retrieval-source-origin and response-URL fields as typed attributes. The semantics (FBY, membership, `within`, text operators for URL/domain matching, invisible-flag test) are all present. The dependency is **telemetry schema**, not language expressiveness → roadmap Phase 3 (GenAI/OTel ingestion, agent schemas). Hidden-text angle: §5.1 invisible-stripping flag (`text_invisibles`) is a usable *supporting* signal here. **Class: MVP (language) / P3 (schema).**

### A3 — ShadowLeak

**Detection** — mailbox-read FBY outbound request to unapproved destination, within one run:
```
partition by agentRunId of AgentToolCalls
  e1 = [tool=='mail.read']
  FBY e2 = [tool=='http.get' and not(dest_domain in ApprovedDestinations)]
  within 2 min
```
The pattern is **MVP-expressible**. The real obstacle is that the exfil runs provider-side and emits **no enterprise telemetry**; detection depends entirely on ingesting provider/connector agent logs. **Class: MVP (language) / P3 (requires provider telemetry ingestion)** — and flagged in §3 as a telemetry-reach limit no language feature fixes.

### A4 — AgentFlayer

**Detection** — read externally-shared file FBY secret-keyword drive-search FBY egress URL:
```
partition by sessionId of AgentToolCalls
  e1 = [tool=='drive.read' and file_shared_external==true]
  FBY e2 = [tool=='drive.search' and query has any of SecretHuntTerms]   // keyword-set
  FBY e3 = [tool=='http' and not(dest in ApprovedDestinations)]
  within 5 min
```
Three-step FBY chain + keyword-set text operator + membership. **Class: MVP** on language. Caveat already noted: the payload concealment is CSS styling, so the normalization pipeline's invisible-char flag will not fire — detection must rest on the behavioral sequence, which it can. **Class: MVP (language) / P3 (schema).**

### A5 — GitHub MCP toxic flow

**Detection** — untrusted-read → privileged-read → public-write in one session:
```
partition by sessionId of McpAudit
  e1 = [tool=='get_issue' and repo_visibility=='public']
  FBY e2 = [tool=='file_read' and repo_visibility=='private']
  FBY e3 = [tool=='create_pull_request' and repo_visibility=='public']
  within 10 min
```
Pure ordered sequence, no thresholds — the **sweet spot** for the MVP pattern engine. **Class: MVP (language) / P3 (MCP audit schema).** One of the cleanest fits in the catalog.

### A6 — Malicious MCP / skill poisoning

**Detection (a) per-call membership violation** — a tool call whose destination/recipient is not on the allowlist:
```
partition by toolId of ToolCalls
  single-event filter: not(recipient in ApprovedRecipients)   // e.g. BCC domain
```
A stateless filter + table membership. **Class: MVP.**

**Detection (b) change-point** — tool gains a new destination right after a version bump:
```
partition by toolId of ToolEvents
  e1 = [event=='version_change']
  FBY e2 = [event=='call' and not(dest in KnownDestinations)]
  within 1 day
```
**Class: MVP** if version-change and call share a stream and a key. But detecting "a *new* destination never seen before for this tool" (first-seen / novelty) rather than "not on a static table" needs **state that accumulates seen-values** — a distinct-set, not available in MVP. Static allowlist: MVP. Learned first-seen: **P3 (baselines/FA6).**

**Detection (c) publisher fan-out** (one publisher → hundreds of artifacts): distinct-count across an artifact dimension keyed by publisher — same cross-key/aggregation gap as A1(c). **Class: P2/P3.**

### A7 — Nx s1ngularity

**Detection** — package install FBY AI-CLI spawn with dangerous flags FBY new public-repo push, minutes apart:
```
partition by hostId of EndpointAgentEvents
  e1 = [event=='pkg_install']
  FBY e2 = [event=='ai_cli_spawn' and (flags has 'dangerously-skip-permissions'
                                       or flags has 'trust-all-tools')]
  FBY e3 = [event=='git_repo_create' and repo_visibility=='public']
  within 30 min
```
Clean FBY chain + term-match text operators on flags. **Class: MVP (language) / P3 (endpoint-agent schema).** Note the `or` inside a *single step's filter* is fine (boolean expression); the forbidden thing is `or` as a *pattern operator* (alternative subpatterns) — not needed here.

The **file-read burst** sub-signal (velocity of reads) is **P2** (counting). The **host fan-out** to one repo-name pattern is the cross-key gap again (**P2/P3**), though the repo-name IOC is a trivial MVP substring filter once you have the events.

### A8 — Shai-Hulud worm

**Detection (a) publish without preceding build** — a publish event with no matching CI/commit for that version within a prior window. This is a **"not preceded by" absence** — abs fires on the *consequent* when the *antecedent* is missing *before* it. SEMANTICS.md's absence is `e1 FBY not B for D` — absence of a *following* event after an anchor. There is **no "A not preceded by B" operator**; you cannot anchor absence to a future event. **Class: INEXPRESSIBLE in MVP** (missing: backward/preceded-by absence, or a join against recent build events). Phase 2 stream–stream joins could express it as an anti-join. **Class: P2 (joins).**

**Detection (b) propagation fan-out** — one maintainer identity publishing N packages in minutes: counting over a window per key. **Class: P2 (counting + time window).**

**Detection (c) IOC match** — `shai-hulud-workflow.yml`, runner `SHA1HULUD`: trivial substring/term filters. **Class: MVP.**

### A9 — GTG-1002

**Detection (a) machine-speed velocity** — per-identity request rate exceeds a human-plausible threshold over a short window (thousands/min, multi-per-second): a **count over a rolling time window per key**. MVP has no rolling time window and no count. cron buckets are wall-clock-aligned, not rolling, and still need a count inside. **Class: P2** (counting + time/length windows). This is the most broadly-needed missing primitive (see §3).

**Detection (b) phase sequence** — recon-tool category FBY exploit-tool category FBY exfil-tool category: an ordered multi-step pattern. **Class: MVP (language) / P3 (tool-category schema).** The *velocity* is the strong signal; the *sequence* alone is high-FP without it.

### A10 — CoPhish

**Detection** — OAuth consent grant to an app identity not in the known-apps table (optionally: created recently):
```
partition by tenantId of IamAudit
  single-event: event=='consent_grant' and not(appId in RegisteredApps)
```
**Class: MVP** for the static-table form. The "app created < 7 days ago" refinement needs correlating create-time with grant-time — either a table carrying creation timestamps (MVP, programmatic load) or a FBY `app_create → consent_grant within 7 day`. Both **MVP**. Clean fit; the only barrier is IAM-event ingestion (**P3 schema**).

### A11 — Delegation / identity abuse

**Detection (a) same identity, new context** — token used from an ASN/geo not in its known set: **first-seen** again — needs accumulated per-identity state (distinct-set of seen ASNs). Static allowlist: MVP. Learned: **P3 (baselines).**

**Detection (b) identity fan-out** — one identity acting across many resources/principals in a window: distinct-count across a dimension per key → **P2/P3** (aggregation), and if the dimension is itself the natural partition key, the **cross-key** problem.

**Detection (c) delegation chain depth** — `X acts-for Y acts-for Z` exceeding N hops: this is **graph/recursive** reasoning. Flat named-step patterns (SEMANTICS.md §6.1 forbids temporal nesting; compose by naming intermediate streams) can express a *fixed* depth (2–3 hops) as a FBY chain over a delegation-event stream keyed by chain-root, but **arbitrary/unbounded chain depth is INEXPRESSIBLE** (no recursion/Kleene; Kleene is P2 but even Kleene over a *relationship* is not graph traversal). **Class: fixed-depth MVP/AWKWARD; arbitrary-depth INEXPRESSIBLE.**

### A12 — Rogue evaluation agents

**Detection (a) lateral sequence** — escalate → harvest → move, per identity: FBY chain, **MVP (language) / P3 (schema).**

**Detection (b) action velocity / 17K actions, dwell** — count per identity over a window, plus long-horizon "still active after D": counting (**P2**) and long-dwell absence/duration. The design's absence TTL rule (§9) actually *constrains* long-horizon detection: the partition TTL must be ≥ every bound, so a multi-day dwell detection forces a multi-day TTL (memory cost) — a real tension, noted in §3.

**Detection (c) short-lived-sandbox fan-out** — many ephemeral identities, one actor: **cross-key** again. **P3.**

### A13 — Vercel third-party-tool breach

**Detection** — third-party-app token FBY identity-provider account access FBY internal-resource access, by a linked principal:
```
partition by principalId of IamAudit
  e1 = [event=='thirdparty_token_use' and app in MonitoredAiTools]
  FBY e2 = [event=='workspace_access']
  FBY e3 = [event=='internal_resource_access']
  within 1 hour
```
**Class: MVP (language) / P3 (schema)** for the per-principal sequence. The "hundreds of users across many orgs" **fan-out** is the cross-key gap (**P3**). Correlating the *same* token across the third-party tool and the IdP may need a **join** if they are separate streams (**P2**).

---

### Expressibility tally

Counting each attack by the class of its **primary/core** detection (the earliest high-value signal), with secondary detections noted in prose above:

| Class | Attacks (primary detection) | Count |
|---|---|---|
| **MVP** (language ready; some need P3 telemetry schema) | A2, A4, A5, A6(a), A7, A10, A13 | **7** |
| **P2** (counting/Kleene, time windows, joins, or-patterns) | A8, A9 | **2** |
| **P3** (baselines/first-seen, identity schema, aggregation) | A1, A11 | **2** |
| **INEXPRESSIBLE in any current phase** (named missing construct) | A3, A12 | **2** |

Notes on the tally's honesty:
- The 7 "MVP" attacks are MVP *in language expressiveness* but all except A6(a)/A10's static forms depend on **Phase 3 agent-telemetry ingestion** to have events to match on. The MVP kill-switch demo uses hand-shaped streams; real deployment waits on P3 schemas. So "7 MVP" means "the semantics can express it," not "shippable in Phase 1."
- A3 and A12 are classed INEXPRESSIBLE not because of a single missing operator but because their *core* value needs capabilities (A3: telemetry the enterprise cannot see; A12: fused velocity+dwell+fan-out at machine speed) that no single roadmap phase delivers together — see §3.
- Nearly every attack has a **secondary** detection that falls in a *later* class than its primary (e.g. A1 primary MVP, but its best signal A1(c) is P3). The product's real coverage is better described by the gap ranking below than by the primary-class tally alone.

---

## 3. Roadmap findings

Ranked by **how many catalog attacks need the missing capability**. Each tied to named attacks.

### Gap 1 — Cross-key correlation (fan-out): one actor, many agents/tenants/resources. (needed by A1, A6, A7, A8, A11, A12, A13 — 7 attacks)

The most common defining signature across the catalog is **fan-out**: one token/publisher/identity/host touching many of something. SEMANTICS.md §9 states flatly "No cross-key reads exist," and partitioning is per-key isolation. This is correct and deliberate for determinism, but it means the engine *structurally cannot see* the earliest and strongest signal of the mass-exfiltration, supply-chain-worm, and identity-abuse attacks — the ones that define the 2025–2026 landscape and the ones the product's "detection across agent fleets" mission (brainstorm decision #1) explicitly claims.

- **Roadmap coverage:** Phase 2 lists "group by/having/order/limit"; Phase 3 lists "minimal rolling rollups (FA6)… powering behavioral baselines" and the "identity-velocity" detection in the pack. Together these are the raw material for fan-out detection (aggregate keyed by actor, count distinct over a dimension).
- **Ordering verdict:** **The phase ordering under-serves the product's headline claim.** Fan-out across agents is the "across a fleet" in "detection across agent fleets," yet it lands in Phase 2–3 while the Phase 1 MVP headlines a *single-agent* kill-switch. That is a defensible thin-slice choice, but the roadmap should state explicitly that **fleet-wide / multi-agent correlation is a Phase 2–3 capability, not MVP** — otherwise the MVP's "agent fleet" framing over-promises. Recommend: add an explicit roadmap note that the identity-velocity / fan-out detection (already named in FA13 and Phase 3) is the true realization of the fleet claim, and that MVP demonstrates the per-agent primitive it composes from.
- **Also note the ergonomics:** even with Phase 2 group-by, expressing "distinct tenants per token in 24h > N" cleanly needs distinct-count aggregation and a rolling window together. Confirm Phase 2 group-by + Phase 2 time windows + Phase 3 rollups actually compose for this; if group-by is batch/window-close only, streaming fan-out alerts may still be AWKWARD.

### Gap 2 — Counting / aggregation over a rolling time window (velocity & volume). (needed by A1, A7, A8, A9, A11, A12 — 6 attacks)

Machine-speed velocity (A9's thousands-of-requests/sec; A12's 17K actions), bulk-read volume (A1), file-read bursts (A7), propagation rate (A8), identity velocity (A11) all need **count ≥ N over a rolling window per key**. The MVP has four windows — sort/cron/delay/batch — none of which is a rolling count window, and no counting/aggregation at all (confirmed: §7 table + roadmap FA1 "NOT in MVP: group by/having").

- **Roadmap coverage:** Phase 2 "window catalog (time, timeBatch, hopping, length, session)" + "counting/Kleene"; Phase 3 "minimal rolling rollups (min/hour/day)." Covered.
- **Ordering verdict:** **Holds, but this is the single most broadly-needed primitive after fan-out, and velocity is the signature of the most severe attacks (A9 AI-orchestrated campaign, A12 rogue agents).** Given the mission, consider whether a *minimal* count-in-window belongs earlier than full Phase 2 breadth — the attack data would justify pulling "length/time window + count threshold" to the front of Phase 2 (it is the second-most-cited gap and the one behind the scariest incidents). Not an MVP ask, but a Phase 2 *sequencing* recommendation: count-over-window before hopping/session niceties.

### Gap 3 — "Not preceded by" / provenance-absence, and first-seen novelty. (needed by A6(b), A8, A11 — 3 attacks; plus strengthens A1, A13)

Two related missing capabilities:
- **Backward absence / anti-join:** A8's strongest signal is "a package publish with *no* preceding CI build." SEMANTICS.md absence only fires for an event *missing after* an anchor (`e1 FBY not B for D`); there is no "consequent not preceded by antecedent." **Missing construct: preceded-by absence, expressible as a stream–stream anti-join** (Phase 2 joins).
- **First-seen / novelty state:** A6(b) new-destination-for-this-tool, A11(a) new-ASN-for-this-identity, and the learned forms of A1/A13 need the engine to *remember the set of values seen per key* and fire on a new one. MVP tables are programmatically loaded (not self-populating from the stream), so learned allow-sets are not expressible; only static tables are. **Missing: engine-maintained distinct-value state / self-populating baselines.**

- **Roadmap coverage:** joins → Phase 2; baselines/rollups (FA6) → Phase 3. Both covered.
- **Ordering verdict:** **Holds.** First-seen is correctly a Phase 3 concern (it is baseline-shaped). Flag only that the *static-table* substitutes (programmatically maintained allowlists via the Go API) are a genuine MVP workaround for several attacks (A6a, A10, A13) and the docs/pack should lean on them rather than implying novelty detection exists early.

### Cross-cutting findings not owned cleanly by any phase

1. **Telemetry reach, not language, is the binding constraint for a third of the catalog.** A3 (ShadowLeak) and parts of A2/A4/A12 produce **no enterprise-side telemetry** — the exfil runs in the provider's cloud. No CEP feature fixes this; it requires ingesting provider/connector agent logs. The roadmap's Phase 3 "GenAI telemetry ingestion (OTel GenAI, AI-gateway logs, MCP audit events)" is therefore not just a detection-pack convenience — **it is the precondition for the engine to see most agent attacks at all.** Recommend elevating its framing from "schema convenience" to "detection precondition," and documenting honestly that attacks executed entirely provider-side are out of reach without provider cooperation. This is the one finding that **no phase fully owns** as a *limitation statement* (the ingestion work is scheduled; the limits-of-visibility disclosure is not).

2. **The TTL ≥ bound rule (§9) taxes long-dwell detection.** A1 (dormant token revival over weeks) and A12 (week-long dwell) want detections spanning days/weeks. The deploy check "TTL ≥ every pattern bound" means a 14-day dwell detection forces a 14-day per-key TTL — unbounded-ish memory for high-cardinality agent fleets. This is a correct safety rule, but the attack data shows long-horizon detection is real (A1, A12), so the roadmap should anticipate a memory-efficient long-horizon mechanism (e.g. rollup-backed dwell checks rather than live partial-match retention). Currently implicit in FA6 rollups; make it explicit.

3. **Delegation-chain depth (A11c) is genuinely out of scope for a flat-pattern CEP.** Graph/recursive traversal of A2A delegation chains is not a CEP primitive and no phase claims it. Fixed small depths are expressible; arbitrary depth is not. This is correctly a *platform* concern (the brainstorm's Vorsar platform boundary), but the roadmap should name it as a deliberate non-goal so the detection pack does not imply chain-depth coverage.

4. **Normalization pipeline scope is right but has a documented blind spot.** §5.1's Cf/Cc/Co/Cn invisible-stripping is well-aimed at ASCII-smuggling prompt injection — a real technique. But A4's concealment is **CSS styling (white/1-pt text)**, which arrives as ordinary visible code points; the pipeline cannot flag it. Not a gap to fix in the engine (styling is a rendering concern), but the detection pack docs should not claim the invisible-flag catches "hidden-text injection" generally — only the Unicode-invisible subclass. Minor accuracy note, not a roadmap change.

### Phase-ordering verdict (overall)

The dependency ordering is **sound** — joins/counting/windows (Phase 2) genuinely unblock the pack and baselines (Phase 3), and LLM-in-the-loop (Phase 4) correctly sits last. Two adjustments the attack data supports:
- **Make the fleet/fan-out capability's phase explicit** (Gap 1): the MVP's "agent fleet" language should be qualified, since cross-agent correlation is Phase 2–3.
- **Pull minimal count-over-window to the front of Phase 2** (Gap 2): it is the primitive behind the most severe incidents (A9, A12) and the second-most-cited gap; it should precede hopping/session window breadth.
- **Re-frame Phase 3 telemetry ingestion as a detection precondition, not a convenience** (cross-cutting #1): without it the engine cannot observe most of the catalog.

No phase needs to be reordered wholesale; no named gap is wholly uncovered except the *visibility-limits disclosure* (#1) and the *explicit non-goal* for chain-depth (#3), both of which are documentation actions rather than engineering phases.

---

## 4. References

All accessed 2026-10-31.

[1] The Hacker News, "Google Warns of Salesloft Drift OAuth Breach…", Aug 2025 — https://thehackernews.com/2025/08/google-warns-salesloft-oauth-breach.html
[2] Security Affairs, "UNC6395 targets Salesloft in Drift OAuth token theft campaign", Aug 2025 — https://securityaffairs.com/181632/hacking/unc6395-targets-salesloft-in-drift-oauth-token-theft-campaign.html
[3] Help Net Security, "Salesloft Drift data breach: Investigation reveals how attackers got in", Sep 8 2025 — https://www.helpnetsecurity.com/2025/09/08/salesloft-drift-data-breach-investigation/
[4] The Hacker News, "Zero-Click AI Vulnerability Exposes Microsoft 365 Copilot Data", Jun 2025 — https://thehackernews.com/2025/06/zero-click-ai-vulnerability-exposes.html
[5] TechRepublic, "First Known Zero-Click AI Exploit: M365 Copilot's 'EchoLeak' Flaw", Jun 2025 — https://www.techrepublic.com/article/news-microsoft-365-copilot-flaw-echoleak/
[6] Radware, "Radware Uncovers First Zero-Click, Service-Side Vulnerability in ChatGPT" (ShadowLeak), Sep 2025 — https://www.radware.com/newsevents/pressreleases/2025/radware-uncovers-first-zero-click-service-side-vulnerability-in-chatgpt/
[7] The Hacker News, "ShadowLeak Zero-Click Flaw Leaks Gmail Data", Sep 2025 — https://thehackernews.com/2025/09/shadowleak-zero-click-flaw-leaks-gmail.html
[8] Zenity Labs, "AgentFlayer: ChatGPT Connectors 0-click attack", Aug 2025 — https://labs.zenity.io/p/agentflayer-chatgpt-connectors-0click-attack-5b41
[9] Hackread, "AgentFlayer 0-click exploit abuses ChatGPT Connectors…", Aug 2025 — https://hackread.com/?p=133202
[10] Invariant Labs, "MCP GitHub vulnerability" (toxic agent flow), May 2025 — https://invariantlabs.ai/blog/mcp-github-vulnerability
[11] Cybersecurity News, "GitHub MCP Server Vulnerability", 2025 — https://cybersecuritynews.com/github-mcp-server-vulnerability/
[12] The Hacker News, "First Malicious MCP Server Found" (postmark-mcp), Sep 2025 — https://thehackernews.com/2025/09/first-malicious-mcp-server-found.html
[13] Snyk, "Malicious MCP server on npm (postmark-mcp) harvests emails", Sep 2025 — https://snyk.io/blog/malicious-mcp-server-on-npm-postmark-mcp-harvests-emails/
[14] Vulert / NVD, "CVE-2025-6514: mcp-remote OS command injection" — https://vulert.com/vuln-db/CVE-2025-6514
[15] The Hacker News, "Researchers Find 341 Malicious ClawHub Skills" (ClawHavoc), Feb 2026 — https://thehackernews.com/2026/02/researchers-find-341-malicious-clawhub.html
[16] Antiy, "ClawHavoc: Analysis of Large-Scale Poisoning Campaign Targeting the OpenClaw Skill Market", Feb 2026 — https://www.antiy.net/p/clawhavoc-analysis-of-large-scale-poisoning-campaign-targeting-the-openclaw-skill-market-for-ai-agents/
[17] Socket, "Nx Packages Compromised in Supply Chain Attack Weaponizing AI CLI Tools", Aug 2025 — https://socket.dev/blog/nx-packages-compromised
[18] JFrog Research, "Nx Supply Chain Attack Targets AI Tool Users", Aug 2025 — https://research.jfrog.com/post/nx-supply-chain-attack-targets-ai-tool-users/
[19] CyberInsider, "Supply chain attack hits Nx npm package…", Aug 2025 — https://cyberinsider.com/supply-chain-attack-hits-nx-npm-package-with-4-6m-weekly-downloads/
[20] Krebs on Security, "Self-Replicating Worm Hits 180+ Software Packages" (Shai-Hulud), Sep 16 2025 — https://krebsonsecurity.com/2025/09/self-replicating-worm-hits-180-software-packages/
[21] Sysdig, "Shai-Hulud: the novel self-replicating worm infecting hundreds of npm packages", Sep 2025 — https://sysdig.com/blog/shai-hulud-the-novel-self-replicating-worm-infecting-hundreds-of-npm-packages
[22] Semgrep, "Sha1-Hulud: The Second Coming of the NPM Worm is Digging For Secrets", Nov 2025 — https://ai.semgrep.dev/blog/2025/digging-for-secrets-sha1-hulud-the-second-coming-of-the-npm-worm
[23] Expel, "Stories from the SOC: The second coming of Shai-Hulud", Dec 2025 — https://expel.com/blog/the-second-coming-of-shai-hulud/
[24] Anthropic, "Disrupting the first reported AI-orchestrated cyber espionage campaign" (GTG-1002), Nov 13 2025 — https://www.anthropic.com/news/disrupting-AI-espionage
[25] BleepingComputer, "New CoPhish attack steals OAuth tokens via Copilot Studio agents" (Datadog Security Labs), Oct 2025 — https://www.bleepingcomputer.com/news/security/new-cophish-attack-steals-oauth-tokens-via-copilot-studio-agents
[26] Wikipedia, "2026 OpenAI agent cyberattacks" (aggregates NYT, Reuters, BBC, OpenAI, Wikimedia primary reports), 2026 — https://en.wikipedia.org/wiki/2026_OpenAI_agent_cyberattacks
[27] Cloud Security Alliance, "Research Note: Hugging Face Autonomous Agent Breach", Jul 20 2026 — https://labs.cloudsecurityalliance.org/research/csa-research-note-huggingface-autonomous-agent-breach-202607/
[28] Deccan Chronicle, "CERT-In Issues Critical Alert after Autonomous AI Agents Access Live Systems of Indian Orgs" (CIAD-2026-0049), Oct 15 2026 — https://www.deccanchronicle.com/artificial-intelligence/cert-in-issues-critical-alert-after-autonomous-ai-agents-access-live-systems-of-indian-orgs-1996324
[29] The Record, "Cloud platform Vercel says company breached through AI tool" (Context.ai), Apr 2026 — https://therecord.media/cloud-platform-vercel-says-company-breached-through-ai-tool
[30] Kiteworks, "AI Agent Security Incidents Hit 65% of Firms in 2026" (CSA/Token Security survey), 2026 — https://www.kiteworks.com/cybersecurity-risk-management/ai-agent-security-incidents-2026/
[31] OWASP GenAI Security Project, "OWASP Top 10 for Agentic Applications", Dec 9 2025 — https://genai.owasp.org/2025/12/09/owasp-top-10-for-agentic-applications-the-benchmark-for-agentic-security-in-the-age-of-autonomous-ai/
[32] Reed Smith, "NSA Publishes Security Guidance on Designing AI Systems with Model Context Protocol" (and CISA/allies agentic AI guidance), May 2026 — https://www.reedsmith.com/our-insights/blogs/viewpoints/102mvg9/nsa-publishes-security-guidance-on-designing-ai-systems-with-model-context-protoc/
[33] arXiv 2403.02817, "Morris II: Unleashing Zero-click Worms that Target GenAI-Powered Applications" (v2 Jan 30 2025) — https://arxiv.org/abs/2403.02817
[34] Cointelegraph, "Step Finance treasury breach, STEP token crash", Jan–Feb 2026 — https://cointelegraph.com/news/step-finance-treasury-breach-solana-step-token-crash

---

## Dispositions (coordinator, 2026-10-31) — per ground rule 16

2026-dated incidents spot-verified against independent sources (CSA Hugging Face note; Vercel/Context.ai reporting) before acting. Actions taken:

1. **Gap 1 (fan-out / fleet claim)** → FIXED (docs): honest-scope note added to 02-roadmap.md Phase 1 — fleet-wide correlation is Phase 2–3; MVP demonstrates the per-agent primitive. Composition check (group-by × rolling windows for streaming distinct-count) forwarded to Phase 2 design.
2. **Gap 2 (count-over-rolling-window)** → FIXED (roadmap): front-loaded as the first item of Phase 2, with the attack evidence cited.
3. **Gap 3 (not-preceded-by + first-seen)** → FIXED (roadmap): anti-join named explicitly in Phase 2 joins; first-seen/novelty state named explicitly in Phase 3 rollups.
4. **Telemetry-reach limit** → FIXED (roadmap): Phase 3 ingestion reframed as a detection precondition; visibility-limits statement required in docs.
5. **TTL vs long-dwell tension** → FIXED (roadmap): rollup-backed long-horizon dwell detection named in Phase 3.
6. **Delegation-chain depth** → FIXED (roadmap): named as an explicit non-goal (platform concern); fixed small depths remain expressible.
7. **CSS-concealment blind spot** → FIXED (spec): honest-limit note added to SEMANTICS.md §5.1 — the pipeline catches Unicode-level concealment only.

Verdict adopted: phase ordering sound; MVP kill-switch use case confirmed by the A1 per-tenant shape; no wholesale reordering.
