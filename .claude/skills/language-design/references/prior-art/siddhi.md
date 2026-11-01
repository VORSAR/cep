# Prior art: Siddhi (WSO2)

*Read when:* Siddhi is a compile target or a language the readers know (Phase 2).

## 1. Siddhi (WSO2)

Shape: keyword pipeline, SQL-flavoured, statement-terminated with `;`.

```
define stream MaterialSupplyStream (name string, amount double, supplier string);

from every (e1=MaterialSupplyStream) -> e2=MaterialSupplyStream[e1.name == e2.name and e1.amount - e2.amount > 10]
    within 10 min
select e1.name, e1.supplier, e1.amount
insert into MaterialSupplyAlertStream;

-- absence
from every (e1=MaterialSupplyStream) -> not MaterialConsumptionStream[name == e1.name and amount == e1.amount] for 15 min
select e1.name, e1.amount
insert into MaterialSupplyAlertStream;

-- counting pattern, logical pattern
from every a1 = infoStock[action == "buy"]<2:> -> b1 = StockExchangeStream[price > 70] ...
from every a1 = OrderStock1[action == "buy"] and a2 = OrderStock2[action == "buy"] -> b1 = ...
```

Other constructs: `#window.time(1 min)`, `#window.length(100)`, `#window.timeBatch`, `#window.session(gap)`; `group by`, `having`, `output every 5 sec` / `output last every`; `insert [current events | expired events | all events] into`; `partition with (symbol of StockStream) begin ... end`; `define aggregation` for incremental time-series aggregation; `@app:name`, `@source`, `@sink`, `@map` annotations for I/O; sequences (`,` strict contiguity) vs patterns (`->` relaxed).

Gets right: reads top-to-bottom as dataflow; named captures `e1=`; `within` is a first-class keyword; absence has a keyword form `not ... for <time>`; whole-application unit (streams, sources, sinks, queries) in one file; `partition` is explicit.

Gets wrong for your purpose: `every` semantics are a semantic trap (restart-per-event with parentheses changing meaning); `<2:>` counting syntax is unreadable and positional (`e1[0]`); `current/expired/all events` leaks the engine's window-expiry model into output semantics; event-time handling is bolted on (`@app:playback`, timestamp attribute conventions) rather than declared per stream; `output every` conflates trigger and rate limit; low-resource syntax, so LLM prior is weak everywhere.

Inherit: `define stream` schemas, the `from … select … insert into` spine, named captures, `within`, `not … for <time>` absence, explicit `partition`.
