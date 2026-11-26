# cep

[![build](https://github.com/VORSAR/cep/actions/workflows/gate.yml/badge.svg?branch=main)](https://github.com/VORSAR/cep/actions/workflows/gate.yml?query=branch%3Amain)
[![coverage](https://img.shields.io/badge/coverage%20floor-90%25%20per%20package-brightgreen)](docs/development/testing.md)

cep is a complex event processing engine written in Go. You give it streams of
events and a set of rules, and it tells you when a pattern you care about shows
up. Think "this happened, then that happened within five minutes, and this other
thing never happened in between."

We're building it for one job first: catching attacks on and by AI agents. Most
guardrails look at one request at a time, but many real attacks are made of
steps that each look harmless on their own. An agent reads from a sensitive
store. A few minutes later it sends data somewhere it shouldn't. Nobody approved
it. Only the sequence gives it away, and watching for sequences over time is
exactly what a CEP engine does.

cep is the correlation core of Vorsar's detection and response platform for AI
agents. The engine itself stays general purpose, and it's Apache-2 licensed.

## Why another CEP engine

The mature CEP engines run on the JVM, which makes them awkward to put where
agent traffic actually flows: inside gateways, sidecars and collectors. Go has
stream processors, but we couldn't find a maintained one that can say "A
followed by B within T, and no C". That's the gap cep is meant to fill.

What we care about:

- **It lives inside your program.** Import the library, deploy rules, push
  events, get detections back. No server, no cluster, no network hop. A small
  static binary for people who'd rather not write Go comes later.
- **Correct before fast or feature-rich.** Event time, late events and
  out-of-order input are handled on purpose, and replay is deterministic: the
  same input gives the same output, every time. A security engine that gets a
  pattern wrong once doesn't get trusted again.
- **A rule language people already know.** It follows Siddhi's streaming SQL,
  tidied up, with its known gaps fixed. Anything cep can't run is rejected when
  you deploy it, with a line and column, rather than quietly doing the wrong
  thing.
- **LLMs are optional.** The plan is to let a model judge the few matches the
  rules have already narrowed down. Everything works without one, and the hot
  path never waits on a model call.

## A worked example: an app nobody registered

**The threat.** Consent phishing. An attacker gets someone in your company to
click "Allow" on an OAuth prompt for an app that looks legitimate but isn't.
From then on the attacker's app can read the tenant's data, and no password was
ever stolen. One thing gives it away early: the app receiving consent isn't one
you've registered.

**The rule.** Every consent grant gets checked against a table of registered
apps. A grant to anything else raises an alert.

```
language cep/0.1;

stream ConsentGrants (tenantId string, appId string optional, grantedBy string,
                      ts timestamp)
  time ts watermark bounded 1 min;

table RegisteredApps (appId string) primary key (appId);

stream ConsentAlerts (tenantId string, appId string, ts timestamp);

rule UnknownAppConsent
  intent 'A consent grant to an app identity not in the registered-apps table.'
  partition by tenantId of ConsentGrants ttl 10 min
  from ConsentGrants[coalesce(appId, 'missing') not in RegisteredApps]
  expect given table RegisteredApps [ {appId: 'crm'} ]
         given ConsentGrants [ {tenantId: 't1', appId: 'crm', grantedBy: 'alice', ts: timestamp '2026-01-01T10:00:00Z'},
                               {tenantId: 't1', appId: 'rogue', grantedBy: 'alice', ts: timestamp '2026-01-01T10:01:00Z'},
                               {tenantId: 't2', appId: 'rogue', grantedBy: 'bob', ts: timestamp '2026-01-01T10:02:00Z'},
                               {tenantId: 't1', appId: 'rogue', grantedBy: 'alice', ts: timestamp '2026-01-01T10:03:00Z'} ]
         then ConsentAlerts [ {tenantId: 't1', appId: 'rogue', ts: timestamp '2026-01-01T10:01:00Z'},
                              {tenantId: 't2', appId: 'rogue', ts: timestamp '2026-01-01T10:02:00Z'} ]
  select tenantId, coalesce(appId, 'missing') as appId
  output first every 5 min
  insert into ConsentAlerts;
```

Reading it line by line:

- `time ts watermark bounded 1 min` means the event's own timestamp is the
  clock, and events may arrive up to a minute out of order.
- `partition by tenantId` gives each tenant its own state. `ttl 10 min` lets cep
  forget a tenant that has gone quiet.
- `coalesce(appId, 'missing') not in RegisteredApps` is the check itself. A
  grant with no app id at all counts as `missing`, which isn't registered
  either, so it alerts too.
- `output first every 5 min` sends at most one alert per tenant every five
  minutes, so a burst of grants doesn't flood whoever is on call.

**What happens.** The `expect` block is a test that lives inside the rule. With
`crm` registered, these four grants come in:

| Time  | Tenant | App   | What cep does                                            |
|-------|--------|-------|----------------------------------------------------------|
| 10:00 | t1     | crm   | Nothing. `crm` is registered.                            |
| 10:01 | t1     | rogue | **Alert** for t1.                                        |
| 10:02 | t2     | rogue | **Alert** for t2. Each tenant has its own limit.         |
| 10:03 | t1     | rogue | Nothing new. t1 already got an alert in the last 5 min.  |

Two alerts come out, and they're exactly the two the test lists. cep runs every
`expect` block when you deploy the file. If the rule ever stops doing what its
tests say, the deploy is refused and the error points at the line that
disagrees.

The same rule, with six tests covering a few more corners, is
[docs/language/examples/07-consent-gate.cep](docs/language/examples/07-consent-gate.cep).
To deploy it and run its tests yourself:

```
go test ./conformance -run TestCorpus/07-consent-gate -v
```

To embed a rule in your own Go program, the shortest working example is
[example_test.go](example_test.go): it loads a rule file, deploys it, sends two
events and prints the alert that comes out.

## Where it's heading: the Agent Kill Switch

The consent check above looks at one event at a time. The rule the first
milestone is built around looks at a sequence. An agent reads something
sensitive, then sends data to a destination that isn't approved, within five
minutes, and no human approves it in between. When that happens, cep disables
the agent through an IAM webhook and emails the security team.

```
rule AgentKillSwitch
  partition by agentId of AgentActions, agentId of HumanApprovals ttl 15 min
  from every read = AgentActions[action == 'read'
                                 and resource in SensitiveResources]
    -> egress = AgentActions[sessionId == read.sessionId
                             and coalesce(destination, 'unknown') not in ApprovedDestinations]
   and not HumanApprovals[sessionId == read.sessionId] for ${kill_window}
  within ${kill_window}
  ...
```

This one doesn't run yet. cep refuses it at deploy time with "not built yet",
because pattern rules (`->`, `not ... for`, `within`) are the next piece of
work. The full file is
[docs/language/examples/01-agent-kill-switch.cep](docs/language/examples/01-agent-kill-switch.cep).

## Where it stands

Early. Phase 1 is under way and nothing has been released yet.

Working today: the Go library and version 0.1 of the rule language, covering
rules that filter and reshape single events, table lookups, per-key partitions,
rate-limited output, parameters and built-in `expect` tests. There's also an
in-memory source and sink, and playback mode, which replays recorded events on
a virtual clock.

Not built yet: pattern rules and windows, the standalone binary and CLI, the
HTTP, email and other connectors, the Go rule builder, the detection pack and
anything involving LLMs. The first release spots attacks one agent at a time.
Correlating across a whole fleet of agents comes in a later phase.

The first milestone is the Agent Kill Switch, end to end: detect the pattern,
disable the agent, email the security team, and have all of it running from a
quickstart in under 15 minutes.

The current status is kept in one place:
[the sprint plan's status line](docs/sprints/sprint-plan.md#status).

## Building and testing

You need Go 1.27 (the exact toolchain is pinned in `go.mod`) and a `python3`
of 3.12 or newer, which is only used to start the gate's own pinned tools.

```
go test ./...       # quick run of the Go tests
make check          # build, vet, lint, tests, conformance suite, coverage floor
make strict-gate    # the full gate CI runs
```

The first `make` run downloads the pinned tools it needs. Every package must
keep at least 90% statement coverage or the gate fails; that's what the
coverage badge refers to.

## Docs

[docs/README.md](docs/README.md) is the map. Good places to start are the
[objectives](docs/product/01-objectives.md), the
[roadmap](docs/product/02-roadmap.md) and the
[language spec](docs/language/SPEC.md).

## License

Apache-2.0. See [LICENSE](LICENSE).
