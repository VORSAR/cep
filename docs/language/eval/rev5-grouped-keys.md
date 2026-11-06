# Rev 5 evaluation — grouped partition keys

**Pre-registered 2026-11-06, before any generation** (S3.2b-2's design note §1.6). The L5 protocol of [EVAL.md](../EVAL.md) at a smaller scale: one fresh generator agent; information diet = [SPEC.md](../SPEC.md) (rev 5 draft), [grammar.ebnf](../grammar.ebnf), [errors.md](../errors.md) and K=3 tagged examples ([01](../examples/01-agent-kill-switch.cep) pattern, [07](../examples/07-consent-gate.cep) partitioned plain rule with a rate limit, [09](../examples/09-params-and-quoting.cep) params). No example in the diet uses a grouped key: the form is learned from the spec. Output: one complete `.cep` file per task in [generated-rev5/](generated-rev5/).

## Tasks (given to the generator verbatim)

| ID | Stratum | Intent |
|---|---|---|
| G01 | plain, two-part key | Alert on every failed admin login, at most one alert per tenant-and-user pair every 10 minutes (the same user in another tenant is another pair). Streams: AdminLogins(tenantId string, userId string, ok bool, ts timestamp). |
| G02 | pattern, crossed pairing | Per pair of hosts: a connection request from host A to host B followed within 30 seconds by a reset sent from B back to A → alert. Streams: ConnRequests(src string, dst string, ts timestamp); Resets(src string, dst string, ts timestamp). Alert for every occurrence. |
| G03 | pattern, names and types differ by position | Per agent and run number: a tool call followed within 2 minutes by a policy violation for the same agent and run → alert. Streams: ToolCalls(agentId string, runNo int, tool string, ts timestamp); Violations(agent string, run int, rule string, ts timestamp). Alert for every occurrence. |
| G04 | plain, single key | Alert on every payment with risk over 0.9, at most once per merchant every 15 minutes. Streams: Payments(merchantId string, risk float, ts timestamp). |
| G05 | plain, three-part key | Alert on every denied API call, at most one alert per tenant, service and endpoint every 5 minutes. Streams: ApiCalls(tenantId string, service string, endpoint string, denied bool, ts timestamp). |

## Rubric (pre-registered)

**Parse:** `grammar_check.py grammar.lark` accepts the file. **Static:** no errors.md [static] violation on manual audit (the Go validator for rev 5 is built after G1). **Semantic:**

| ID | Must have | Static audit focus |
|---|---|---|
| G01 | `partition by (tenantId, userId) of AdminLogins`; `output first every 10 min`; filter on `ok == false` (or `not ok`) | TTL ≥ 10 min (E-TTL-002) |
| G02 | `(src, dst) of ConnRequests, (dst, src) of Resets` — the reset's parts **crossed**; a sequence within 30 sec; `every` | `idle after` on both streams (E-IDLE); after-match, late clause; E-PART-003/007 clean |
| G03 | `(agentId, runNo) of ToolCalls, (agent, run) of Violations` — string then int on both | positions typed alike (E-PART-003); TTL ≥ 2 min |
| G04 | `partition by merchantId of Payments` — **no brackets** | E-PART-005 clean |
| G05 | `(tenantId, service, endpoint) of ApiCalls` | TTL ≥ 5 min |

**Targets (as L5):** parse ≥ 97% (here: 5/5), static ≥ 90% (here: ≥ 5/5 after one repair round), one-round repair ≥ 95% of failures. A failure that points at the spec's text changes the text before G1.

## Results

*(filled after the run)*
