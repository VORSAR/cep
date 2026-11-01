# SiddhiQL (Siddhi 5.x) language surface — inventory

Inventory only; no evaluation. Input for the prior-art review of CEP language design.

**Sources (reproducibility):**

- Grammar: `/Users/ramith/code/siddhi/modules/siddhi-query-compiler/src/main/antlr4/io/siddhi/query/compiler/SiddhiQL.g4`, **927 lines**, siddhi repo at `98de1eb41` (2026-10-08, `v5.1.35-1-g98de1eb41`).
- Guide: **Siddhi 5.0 Streaming SQL Guide**, https://siddhi.io/en/v5.0/docs/query-guide/ , fetched 2026-10-31 (page text 168,302 chars). Guide section names below refer to this page's headings.
- Built-in catalog sampled from `/Users/ramith/code/siddhi/docs/api/5.1.33.md` (API Docs v5.1.33) for window/function signatures.

Entry format: construct · meaning · grammar rule(s) in SiddhiQL.g4 · guide heading · minimal example.

---

## 1. Application structure

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Siddhi app | One file: app annotations, then ≥1 definition, then queries/partitions | `parse`, `siddhi_app`, `execution_element` | Siddhi Application | `@app:name('X') define stream S (a int); from S select a insert into O;` |
| Definition-before-query ordering | Grammar forces all definitions before all execution elements, `;`-separated | `siddhi_app` | Siddhi Application (implied) | — |
| `@app:name` | Names the app (random UUID if absent) | `app_annotation` | Siddhi Application | `@app:name('Temperature-Analytics')` |
| `@app:statistics` | Enables metrics reporting | `app_annotation` | Statistics | `@app:statistics(reporter = 'console')` |
| `@app:playback` | Event-time playback (virtual clock) | `app_annotation` | Event Playback | `@app:playback(idle.time = '100 millisecond', increment = '2 sec')` |
| `@app:description` | App description | `app_annotation` (generic) | **not in fetched 5.0 guide** (exists in 5.1 docs) | `@app:description('...')` |
| Generic annotation | `@Name(k='v', @Nested(...))`; nestable; element = optional `key =` + **string** value | `annotation`, `annotation_element`, `property_name`, `property_value` | used throughout | `@info(name = 'query1')` |
| `@info` | Names a query (system-generated if absent) | `annotation` (generic) | Query | `@info(name = 'monitor')` |
| `@Async` | Multi-threaded async stream buffering | `annotation` (generic) | Multi-threading and Asynchronous Processing | `@Async(buffer.size='256', workers='2', batch.size.max='5')` |

## 2. Definitions

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Stream | Typed event schema | `definition_stream` | Stream | `define stream TempStream (deviceID long, roomNo int, temp double);` |
| Table | Stored, queryable event collection | `definition_table` | Table | `define table RoomTypeTable (roomNo int, type string);` |
| Named window | Shared window usable by many queries; optional emitted event type | `definition_window`, `output_event_type` | Named Window | `define window SensorWindow (name string, value float) timeBatch(1 second) output expired events;` |
| Trigger | Periodic/cron/start-up event generator; acts as a stream (`triggered_time long`) | `definition_trigger` | Trigger | `define trigger FiveMinTrigger at every 5 min;` — also `at 'start'`, `at '0 15 10 ? * MON-FRI'` |
| Named (incremental) aggregation | Pre-computed multi-granularity time-series aggregates | `definition_aggregation`, `group_by_query_selection`, `aggregation_time*` | Named Aggregation | `define aggregation TradeAgg from TradeStream select symbol, avg(price) as avgPrice group by symbol aggregate by timestamp every sec ... year;` |
| Script function | Inline UDF in a scripting language; `{...}` body is a raw balanced-brace token | `definition_function`, `function_name`, `language_name`, `function_body`, lexer `SCRIPT` | Script | `define function concatFn[javascript] return string { return data[0]+data[1]; };` |
| Attribute types | `string int long float double bool object` | `attribute_type` | Stream | `temp double` |
| Backtick identifiers | Quote any name not matching `[a-zA-Z_][a-zA-Z_0-9]*` | lexer `ID_QUOTES` | Stream | `` define stream `$test(0)` (a int); `` |

## 3. Queries — input side

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Query shape | `from <input> <select?> <output-rate?> <output>` | `query`, `query_input` | Query | `from TempStream select temp insert into Out;` |
| Standard stream input | Source + handlers before/after one window | `standard_stream`, `basic_source_stream_handler(s)` | Query | `from TempStream[temp>30]#window.time(1 min)[roomNo==1]` |
| Filter | `[boolean expr]` on input; allowed pre- and post-window; optional `#` prefix | `filter` | Filter | `from TempStream[temp > 30.0]` |
| Stream function / processor | `#fn(...)` handler in the stream chain (extensions via namespace) | `stream_function`, `function_operation` | Function / Extensions | `from TempStream#log("msg")` |
| Window attachment | Exactly one `#window.<name>(...)` per input stream | `window` | Window | `from TempStream#window.length(10)` |
| Inner stream ref | `#Name` — partition-local stream | `source` (`inner='#'`) | Inner Stream | `from #AvgTempStream` |
| Fault stream ref | `!Name` — error-carrying twin stream | `source` (`fault='!'`) | Error Handling | `from !TempStream select _error insert into ErrLog;` |
| Anonymous (nested) stream | Inline sub-query as input, ends with `return` | `anonymous_stream` | **not in guide** | `from (from T select roomNo return) select roomNo insert into O;` |

## 4. Queries — projection

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Select | Attribute list, expressions with `as`, or `*` / omitted = pass-through | `query_section`, `output_attribute` | Select | `select roomNo, temp*9/5+32 as tempF` |
| Group by | Multi-key grouping | `group_by` | Group By | `group by roomNo, deviceID` |
| Having | Filter on output attributes (with or without group by) | `having` | Having | `having avgTemp > 20` |
| Order by | Multi-key ordering, `asc` (default) / `desc` | `order_by`, `order_by_reference`, `order` | Order By | `order by avgTemp desc` |
| Limit / offset | Cap/skip output events per batch; grammar takes any expression, guide says positive int | `limit`, `offset` | Limit & Offset | `limit 2 offset 1` |
| Aggregate functions (built-in, v5.1.33 core) | `and, avg, count, distinctCount, max, maxForever, min, minForever, or, stdDev, sum, unionSet` | semantic (parsed as `function_operation`) | Aggregate Function | `select avg(temp) as avgTemp` |

## 5. Queries — output side

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Insert | Emit to stream/table/window | `query_output` (INSERT) | Query / Table Insert | `insert into OutStream;` |
| Insert w/ event type | Choose which window outputs to emit | `query_output`, `output_event_type` | Event Type | `insert expired events into DelayedStream;` |
| Event types | `current events` (default) / `expired events` / `all events` | `output_event_type` | Event Type | `insert all events into X;` |
| Delete | Remove table rows matching condition | `query_output` (DELETE), `set_clause` n/a | Delete | `delete RoomTypeTable on RoomTypeTable.roomNo == roomNumber;` |
| Update | Modify table rows; optional `set`, optional `for <event type>` | `query_output` (UPDATE), `set_clause`, `set_assignment` | Update | `update RoomTbl set RoomTbl.people = RoomTbl.people + arrival on RoomTbl.roomNo == roomNo;` |
| Update or insert | Upsert into table | `query_output` (UPDATE OR INSERT) | Update or Insert | `update or insert into RoomTbl set RoomTbl.assignee = assignee on RoomTbl.roomNo == roomNo;` |
| Return | Emit back out of anonymous stream / programmatic callback | `query_output` (RETURN) | **not in guide** | `... select a return;` |

## 6. Windows (built-in catalog, API v5.1.33 "Core")

All parsed generically as `#window.<fn>(args)` (`window` → `function_operation`); names are **library**, not grammar. `<TIME>` = time constant.

| Window | Signature(s) | Meaning |
|---|---|---|
| `time` | `time(<INT\|LONG\|TIME> window.time)` | Sliding time |
| `timeBatch` | `timeBatch(window.time [, <INT\|LONG> start.time] [, <BOOL> stream.current.event])` | Tumbling time |
| `length` | `length(<INT> window.length)` | Sliding count |
| `lengthBatch` | `lengthBatch(<INT> window.length [, <BOOL> stream.current.event])` | Tumbling count |
| `timeLength` | `timeLength(<TIME> window.time, <INT> window.length)` | Sliding time+count |
| `session` | `session(<TIME> session.gap [, <STRING> session.key [, <TIME> allowed.latency]])` | Session per key |
| `batch` | `batch([<INT> window.length])` | Last arrived event chunk |
| `sort` | `sort(<INT> window.length, attribute [, <STRING> 'asc'\|'desc', ...])` | Top/bottom-k by value |
| `cron` | `cron(<STRING> cron.expression)` | Tumbling on cron schedule |
| `externalTime` | `externalTime(<LONG> timestamp, <TIME> window.time)` | Sliding on event-time attr |
| `externalTimeBatch` | `externalTimeBatch(<LONG> timestamp, <TIME> window.time [, start.time [, timeout]])` | Tumbling on event-time attr |
| `delay` | `delay(<TIME> window.delay)` | Emit after delay |
| `expression` | `expression(<STRING> expression)` | Sliding, retain while expr true (e.g. `'count()<=20'`) |
| `expressionBatch` | `expressionBatch(<STRING\|BOOL> expression [, include.triggering.event [, stream.current.event]])` | Tumbling on expr |
| `frequent` | `frequent(<INT> event.count [, attribute])` | **deprecated** (struck out in API doc) |
| `lossyFrequent` | `lossyFrequent(<DOUBLE> support.threshold [, error.bound [, attribute]])` | **deprecated** |

Guide "Window" table lists only: time, timeBatch, length, lengthBatch, timeLength, session, batch, sort, cron, externalTime, externalTimeBatch, delay — i.e. omits expression, expressionBatch, frequent, lossyFrequent.

## 7. Patterns & sequences

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Pattern chain | Non-contiguous "followed by" | `pattern_stream`, `pattern_source_chain`, token `FOLLOWED_BY` (`->`) | Pattern | `from e1=S1 -> e2=S2[e1.a < a] select e1.a, e2.a insert into O;` |
| `every` | Restart matching per qualifying event; also on parenthesised groups | `every_pattern_source_chain`, `every_absent_pattern_source` | Pattern | `from every e1=S1 -> e2=S2 ...` |
| Event reference | `ref=` binds matched event for later use | `standard_stateful_source`, `event` | Pattern | `e1=TempStream` |
| Logical and/or | Two (only two) stream conditions in either order / either one | `logical_stateful_source` | Logical Patterns | `from every (e1=A and e2=B) -> e3=C ...` |
| `not ... and/or` (wait-for-event absence) | Negated condition resolved when partner arrives | `logical_absent_stateful_source` | Logical Patterns | `from every (not A[x>1] and e2=B) -> ...` |
| `not X for <time>` (timed absence) | Non-occurrence within period | `basic_absent_pattern_source`, `for_time`, absent-chain rules (`absent_pattern_source_chain`, `left_/right_absent_pattern_source`) | Detecting Non-occurring Events | `from e1=A -> not B[e1.id==id] for 5 sec select e1.id insert into O;` |
| Counting quantifier | `<n>`, `<n:m>`, `<n:>`, `<:m>` repeats of one condition (guide: `<:n>` excludes n) | `pattern_collection_stateful_source`, `collect` | Counting Pattern | `from e1=TempStream[temp>40]<1:5> -> e2=Regulator ...` |
| Indexed access | `e1[0]`, `e1[3]`, `e1[last]`, `e1[last-1]`; out-of-range → `null` | `attribute_reference`, `attribute_index` | Counting Pattern / Sequence | `select e1[last].temp` |
| `within` | Time bound over the whole pattern/sequence chain (grammar: one trailing clause) | `within_time` | Pattern (`within <time gap>`) | `from every e1=A -> e2=B within 10 min ...` |
| Sequence chain | Strictly consecutive events, comma-separated | `sequence_stream`, `sequence_source_chain` | Sequence | `from every e1=S, e2=S[e1.temp<temp] select e1.temp insert into O;` |
| Kleene quantifiers | `*` (0+), `+` (1+), `?` (0/1), or `<collect>` on sequence element | `sequence_collection_stateful_source` | Sequence | `from every e1=S, e2=S[temp>e1.temp]+, e3=S[temp<e2[last].temp] ...` |
| Logical / absent sequences | `and/or/not` and `not ... for` inside sequences | `every_absent_sequence_source_chain`, `left_/right_absent_sequence_source`, `logical_*` | Logical Sequence | `from every (not A for 1 sec), e2=B ...` |

## 8. Joins

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Stream–stream join | Two windowed streams, `on` condition | `join_stream`, `join_source` | Join (Stream) | `from A#window.time(1 min) join B#window.length(10) on A.x==B.x select ... insert into O;` |
| Join types | `join`/`inner join`, `left outer join`, `right outer join`, `full outer join` — grammar also accepts bare `outer join` | `join` | Join (Stream) | `from A ... left outer join B ... on ...` |
| `unidirectional` | Only one side triggers the join (not both) | `join_stream` (left/right `UNIDIRECTIONAL`) | Join (Stream) | `from A#window.time(1 min) unidirectional join B#window.time(1 min) on ...` |
| Aliasing | `as ref` per side | `join_source` (AS alias) | Join (Stream) | `from Stock as S join Trade as T on S.s==T.s` |
| Stream–table join | Table side has no window; inner/left/right outer only (guide) | `join_stream` (table as source) | Join (Table) | `from TempStream join RoomTypeTable on RoomTypeTable.roomNo == TempStream.roomNo select ...` |
| Stream–named-window join | Join against defined window; incl. full outer (guide) | `join_stream` | Join (Window) | `from CheckStream as C join TwoMinTempWindow as T on T.temp > 40 select ...` |
| Stream–aggregation join | Join with `within <range>` + `per <granularity>` | `join_stream`, `within_time_range`, `per` | Join (Aggregation) | `from S join TradeAgg as T on S.symbol==T.symbol within "2014-02-15 00:00:00 +05:30", "2014-03-16 00:00:00 +05:30" per "days" select ...` |

## 9. Partitions

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Value partition | Isolated per-key query instances | `partition`, `partition_with_stream` | Partition by value | `partition with ( deviceID of TempStream ) begin from TempStream#window.length(10) select max(temp) as m insert into O; end;` |
| Range partition | Key = named condition range | `condition_ranges`, `condition_range` | Partition by range | `partition with ( roomNo >= 1030 as 'serverRoom' or roomNo < 330 as 'lobby' of TempStream ) begin ... end;` |
| `begin ... end` block | Multiple queries per partition | `partition` | Partition | — |
| Inner stream | `#Name` visible only inside the partition instance | `source` (inner `#`) | Inner Stream | `insert into #AvgTempStream` |
| `@purge` (partition) | Auto-destroy idle partition instances | `annotation` (generic) | Purge Partition | `@purge(enable='true', interval='1 sec', idle.period='15 sec') partition with (...) begin ... end;` |

## 10. Tables, stores & on-demand (store) queries

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| `@PrimaryKey` | Unique composite key | `annotation` (generic) | Primary Keys | `@PrimaryKey('symbol') define table StockTable (...);` |
| `@Index` | Secondary index(es) | `annotation` (generic) | Indexes | `@Index('roomNo') define table RoomTypeTable (...);` |
| `@store` | External backing store; types in 5.0 guide: RDBMS (MySQL/Oracle/SQL Server/PostgreSQL/DB2/H2), MongoDB | `annotation` (generic) | Store | `@store(type="rdbms", jdbc.url="...", username="...", password="...", jdbc.driver.name="...") define table RoomTypeTable (...);` |
| Table `in` membership | Boolean `<condition> in <table>` inside expressions/filters | `math_operation` alt `in_math_operation` | In | `from TempStream[ServerRoomTable.roomNo == roomNo in ServerRoomTable] insert into O;` |
| On-demand query: select | Pull query from table/window, optional `on`, group/having/order/limit | `store_query`, `store_input`, `query_section` | Store Query — (Table/Window) Select | `from RoomTypeTable on roomNo >= 10 select roomNo, type;` |
| On-demand query: aggregation select | Pull from aggregation with `within`/`per` | `store_query`, `within_time_range`, `per` | Store Query — (Aggregation) Select | `from TradeAggregation on symbol=="FB" within "2014-02-15 **:**:** +05:30" per "hours" select symbol, total;` |
| On-demand insert | `select <consts> insert into <table>` (no `from`) | `store_query` (`query_section INSERT INTO target`) | Store Query — Insert | `select 10 as roomNo, 2 as people insert into RoomOccupancyTable;` |
| On-demand delete/update/update-or-insert | Table mutation without `from` | `store_query`, `store_query_output`, `set_clause` | Store Query — Delete/Update/Update or Insert | `select 10 as roomNumber delete RoomTypeTable on RoomTypeTable.roomNo == roomNumber;` |
| Table cache | `@store(..., @cache(...))` | — | **not in fetched 5.0 guide** (added in 5.1 docs) | — |

Naming note: 5.0 guide calls these "Store Query" (grammar rule `store_query`); 5.1 docs rename them "On-Demand Query".

## 11. Incremental (named) aggregations

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| `aggregate by ... every ...` | Aggregate on event-time attribute at multiple granularities | `definition_aggregation` | Named Aggregation | `aggregate by timestamp every sec ... year;` |
| Granularity range | `sec ... year` (inclusive range of units) | `aggregation_time_range`, `TRIPLE_DOT` | Named Aggregation | `every sec ... year` |
| Granularity list | Comma list of units | `aggregation_time_interval`, `aggregation_time_duration` | Named Aggregation | `every seconds, minutes, hours` |
| Units | `seconds minutes hours days weeks months years` | `aggregation_time_duration` | Named Aggregation | — |
| Selection restriction | select + group by only (no having/order/limit) | `group_by_query_selection` | Named Aggregation | `select symbol, avg(price) as avgPrice group by symbol` |
| `@purge` + `@retentionPeriod` | Retention per granularity | `annotation` (nested) | Named Aggregation | `@purge(enable='true', interval='10 sec', @retentionPeriod(sec='120 sec', months='all'))` |
| `@PartitionById` | Distributed (sharded) aggregation | `annotation` (generic) | Distributed Aggregation | `@PartitionById define aggregation ...` |
| `within`/`per` consumption | Range filter + output granularity on joins and store queries; `within` takes 1 wildcard string (`"2014-02-15 **:**:** +05:30"`) or 2 comma-separated start/end expressions | `within_time_range`, `per` | Join (Aggregation) / Store Query | `within "2014-02-15 00:00:00 +05:30", "2014-03-16 00:00:00 +05:30" per "days"` |

## 12. Output rate limiting

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Time-based | Emit first/last/all (default all) per interval | `output_rate`, `output_rate_type` | Output rate limiting | `output all every 10 sec` |
| Count-based | Emit first/last/all per N events | `output_rate` (`INT_LITERAL EVENTS`) | Output rate limiting | `output last every 10 events` |
| Snapshot | Periodic window-state snapshot | `output_rate` (SNAPSHOT) | Output rate limiting | `output snapshot every 5 sec` |

## 13. Sources, sinks & mappers (annotation sub-language)

All parsed by the generic `annotation` rule; keys/types are library-level.

| Construct | Meaning | Guide section | Example |
|---|---|---|---|
| `@source` | Ingest via transport; types named in guide: inMemory (built-in), http, kafka, tcp, email, jms, file, cdc, prometheus | Source | `@source(type='http', receiver.url='http://0.0.0.0:8080/foo', @map(type='json')) define stream In (...);` |
| `@sink` | Publish via transport; types: inMemory, log (built-in), http, kafka, tcp, email, jms, file, prometheus | Sink | `@sink(type='http', publisher.url='...', @map(type='json')) define stream Out (...);` |
| Dynamic sink options | `{{attr}}` substitution inside option strings | Sink | `@sink(type='kafka', topic='{{roomNo}}', ...)` |
| `@map` | Payload format; default `passThrough`; types: json, xml, text, avro, binary, keyvalue, csv | Source/Sink Mapper | `@map(type='json', enclosing.element="$.portfolio")` |
| `@attributes` | Custom inbound mapping, named or positional | Source | `@attributes(symbol = "stock.company.symbol", price = "stock.price")` |
| `@payload` | Custom outbound template(s) | Sink | `@payload("""{"Symbol":"{{symbol}}"}""")` |
| `@distribution` / `@destination` | Multi-endpoint publishing: `strategy='roundRobin'` or `'partitioned', partitionKey='...'` | Sink | `@distribution(strategy='partitioned', partitionKey='country', @destination(publisher.url='...'), @destination(publisher.url='...'))` |
| `@OnError` (stream) | `action='LOG'` (default) or `'STREAM'` → fault stream `!Name` with extra `_error object` attribute | Error Handling | `@OnError(action='STREAM') define stream TempStream (...);` |
| `on.error` (sink option) | `LOG` (default) / `WAIT` (retry+backpressure) / `STREAM` (to fault stream) | Error Handling | `@sink(type='kafka', on.error='WAIT', ...)` |
| `ref` option | Named config reference in @source/@sink | **not in fetched 5.0 guide** (present in 5.1 docs) | `@source(ref='config1', ...)` |

## 14. Expressions & literals

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| int / long | digits; `L`/`l` suffix for long | `signed_int_value`, `INT_LITERAL`, `LONG_LITERAL` | Value | `123`, `-750l` |
| float / double | `F`/`f` suffix float; `D`/`d` optional for double; exponents | `FLOAT_LITERAL`, `DOUBLE_LITERAL` | Value | `123.0f`, `-75.0e-10D` |
| bool | `true`/`false`, case-insensitive | `bool_value` | Value | `TRUE` |
| string | `'...'`, `"..."`, `"""..."""` (triple allows quotes/multiline) | `string_value`, `STRING_LITERAL` | Value | `"""{"a":1}"""` |
| time constant | `(<int> <unit>)+` in descending unit order → long ms; units: `year(s) month(s) week(s) day(s) hour(s) minute(s)/min second(s)/sec millisecond(s)/millisec`, all case-insensitive, no bare `ms`; no `and` in syntax (guide prose "1 hour and 25 minutes" is informal) | `time_value`, `year_value`…`millisecond_value`, lexer `YEARS`…`MILLISECONDS` | Value (Time) | `1 hour 25 min` |
| Arithmetic | `* / %`, `+ -` | `math_operation` (#multiplication/#addition) | Select (precedence list) | `temp*9/5+32` |
| Comparison | `> >= < <=`, `== !=` | `math_operation` (#greaterthan…/#equality) | Select | `volume != 100` |
| Logical | `and or not`, parentheses | `math_operation` (#and/#or/#not) | Select | `not (price<605 and volume>100)` |
| Null check | `<attr|streamref|function()> is null` | `null_check`, `stream_reference` | Logical Patterns (`e1[last] is null` etc.) | `e2 is null` |
| `in` | Membership of condition in a table | `math_operation` #in_math_operation | In | `roomNo == 10 in ServerRoomTable` |
| Precedence (guide) | `()` > `IS NULL` > `NOT` > `* / %` > `+ -` > `< <= > >=` > `== !=` > `IN` > `AND` > `OR` | `math_operation` alternative order | Select | — |
| Attribute reference | `attr`, `Stream.attr`, `alias.attr`, `e1[3].attr`, `#Inner.attr`, `!Fault.attr` | `attribute_reference`, `attribute_index` | Select / Pattern | `e1[last-1].temp` |
| Function call | `(ns:)?name(args…)`; `*` as arg allowed | `function_operation`, `attribute_list` | Function | `str:concat(a, b)` |
| Built-in functions (core v5.1.33) | `UUID cast coalesce convert createSet currentTimeMillis default eventTimestamp ifThenElse instanceOfBoolean/Double/Float/Integer/Long/String maximum minimum sizeOfSet pol2Cart(stream fn) log(stream processor)` | semantic | Function | `ifThenElse(temp>30, 'hot', 'ok')` |
| Keywords as identifiers | Most keywords usable as names (contextual) | `name : id\|keyword`, `keyword` | — | `define stream join (...)` parses |
| Case sensitivity | Keywords case-insensitive (letter fragments); identifiers case-sensitive | lexer fragments `A`–`Z` | — | `SELECT`/`select` |
| Comments | `--` to EOL; `/* ... */` block (hidden channel) | lexer `SINGLE_LINE_COMMENT`, `MULTILINE_COMMENT` | **not documented** (used in guide examples) | `-- comment` |
| Env substitution | No `${...}`/`$` substitution token exists in grammar; none in guide | — | — | — |

## 15. Scripts & extensions

| Construct | Meaning | Grammar rule(s) | Guide section | Example |
|---|---|---|---|---|
| Namespaced call | `<namespace>:<function>(…)` selects an extension | `function_operation` (`function_namespace ':'`) | Extensions | `from In[price>=20]#window.foo:unique(symbol) select ...` |
| Extension types | Function, Aggregate Function, Window, Stream Function, Stream Processor, Sink, Source, Store, Script, Source Mapper, Sink Mapper | — | Extension Types | `math:sin(x)`, `custom:std(x)`, `#custom:pol2cart(theta,rho)` |
| Script body | `{ ... }` with nested braces/strings/`//` comments lexed opaquely | lexer `SCRIPT`, `SCRIPT_ATOM` | Script | `define function f[javascript] return string { ... };` |

## 16. Grammar ↔ guide mismatches (flagged)

**In the grammar, NOT in the (5.0) guide:**

1. `anonymous_stream` — nested inline queries as query input, and the `return` output clause (`query_output` RETURN). Entirely undocumented.
2. Bare `outer join` (without left/right/full) is a valid `join` alternative.
3. Optional `#` before a filter (`filter : '#'? '[' expr ']'`).
4. `RAW` is a lexer token whose only parser use is the `keyword` escape rule (keywords-as-identifiers, SiddhiQL.g4:622); no construct consumes it — dead surface. *(Corrected after verification 2026-10-31.)*
5. Comment syntax (`--`, `/* */`) is defined in the lexer; the guide uses `--` in examples but never specifies comments.
6. `within`/`per` are grammatically allowed on **any** join (`join_stream`) and on store-query input (`store_input`); the guide documents them only for aggregation joins/queries.
7. `limit`/`offset` accept arbitrary expressions in the grammar; the guide states positive integers.
8. Trigger time: grammar accepts any string after `at` (`string_value`); `'start'` vs cron expression is semantic, not syntactic.
9. Annotation values are restricted to string literals (`property_value : string_value`) — numbers/booleans in annotations are always quoted strings.
10. Pattern `within` attaches once, at the end of the whole pattern/sequence chain (`pattern_stream`); the guide's logical-pattern syntax sketch shows `(within <time gap>)?` per stage, which the grammar does not accept.
11. Grammar's `every` may wrap parenthesised absent-pattern groups (`EVERY? '(' ... ')'`) in forms the guide never illustrates.

**In the guide, NOT in the grammar (library-level semantics riding on generic syntax):**

1. All window/function/aggregator names (time, timeBatch, sum, avg, …) — parsed as generic `function_operation`; the catalog lives in siddhi-core/extensions.
2. All annotation vocabularies: `@source/@sink/@map/@attributes/@payload/@distribution/@destination/@store/@PrimaryKey/@Index/@purge/@retentionPeriod/@PartitionById/@Async/@OnError/@info` ride the one generic `annotation` rule; names and keys are unvalidated by the parser.
3. `{{attribute}}` dynamic sink properties, cron expressions, `within` wildcard timestamps (`"2014-02-15 **:**:** +05:30"`), `per "days"` granularity strings — all opaque string literals to the grammar.
4. `on.error` sink option values (LOG/WAIT/STREAM) and `@OnError` actions (LOG/STREAM) — semantic.
5. Fault-stream auto-schema (base attributes + `_error object`) — runtime behavior, not grammar.
6. Restrictions stated only in prose: one window per stream per query, `unidirectional` not on both sides, logical patterns limited to two streams, joins require windows on streams.
7. Guide artifacts: aggregate-function table mislabels `min` as "max"; `@app:description`, `@source(ref=…)`, and table caching (`@cache`) appear in 5.1 docs but not in this 5.0 guide page.
