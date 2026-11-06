# Performance techniques survey — evidence for the throughput/latency architecture

> **Historical — research, 2026-11-01; evidence only.** What was adopted is [ADR-0010](../adr/ADR-0010-performance-model.md) (as annotated since); measured results live in the sprint records.

Research findings (spike output, ground rule 8). Survey of established mechanisms that high-performance middleware in Go and C++/JVM uses to maximize throughput and minimize latency, assessed against this engine's shape: single-node embeddable Go library; sharded runtime (ADR-0006) with one goroutine per shard and per-key serial execution; bounded mailboxes between ingest and shards; sinks with their own queues (ADR-0008); hot path = JSON ingest → schema validation → route by key hash → ordered buffer → pattern/window evaluation → emit. Objective: sub-millisecond hot-path latency at high throughput.

Honesty convention: **[measured]** = published numbers with method/hardware stated; **[claimed]** = vendor/author numbers without independent reproduction; **[folklore]** = widely repeated, weakly sourced or stale — do not design on it without re-measuring.

Verdict scale: **fits now** / **fits later** (after a measured need) / **does not fit** (for a single-node Go CEP engine).

All URLs accessed 2026-11-01.

---

## 1. C++/JVM low-latency middleware classics and what transfers

### 1.1 LMAX Disruptor — pre-allocated ring buffer instead of queues

A fixed-size, pre-allocated ring of event slots with sequence counters replaces linked queues. Producers claim a sequence, write into the pre-allocated slot, publish; consumers track their own sequence. No nodes are allocated per message, head and tail never contend on the same cache line, and garbage is zero in steady state.

- Effect **[measured]**: the technical paper reports unicast 1P–1C throughput of 25,998,336 ops/s (Disruptor) vs 5,339,256 (Java `ArrayBlockingQueue`) on Nehalem 2.8 GHz, and 3-stage-pipeline latency of mean 52 ns / 99% ≤ 128 ns (Disruptor) vs mean 32,757 ns / 99% ≤ 2,097,152 ns (ABQ) ([Disruptor technical paper][disruptor-paper]). Updated runs on AMD EPYC 9374F show Disruptor 4 at 160M ops/s unicast vs ABQ 20.9M.
- Cost: fixed capacity (needs a full-buffer policy), single data structure invites tight coupling, and the magic depends on memory-layout control that Java (and Go) only approximate.
- Verdict: **fits later** — our bounded channels are semantically equivalent (bounded, blocking producer); swap a channel for an in-repo SPSC ring only where a benchmark shows the channel is the bottleneck, not before (see §2.6 for measured Go gaps).

### 1.2 Single Writer Principle

Any given piece of mutable state is written by exactly one thread; others communicate by message. Removes locks, CAS retries, and cache-line ping-pong; the paper notes a single producer needs only memory barriers, no CAS ([Disruptor technical paper][disruptor-paper]; Thompson, [Single Writer Principle][single-writer]).

- Effect **[measured]**: the Disruptor latency/throughput gap above is largely this principle plus layout; Thompson's blog gives the cost ladder of contended operations (lock ≫ CAS ≫ uncontended write).
- Cost: design discipline, not code — requires routing all writes for a datum through one goroutine, which we already do.
- Verdict: **fits now** — this *is* ADR-0006 (per-key state owned by one shard goroutine, per-shard metrics counters summed on scrape). Enforce it in review: no shared atomics, no cross-shard maps, no "small harmless mutex" on the hot path.

### 1.3 Batching effect / smart batching

When a consumer wakes, it processes *everything available*, not one item: start the batch as soon as data exists, end it when the queue is empty or the batch cap is hit ("natural batching"). Synchronization, wakeup, and flush costs are amortized across the batch, so per-event cost *falls* as load rises — the opposite of queues' "J-curve".

- Effect **[measured]**: the Disruptor paper reports latency staying near-constant as load rises until memory saturates ([Disruptor technical paper][disruptor-paper]); Thompson's Smart Batching post shows batching holding latency flat under bursts where serialized individual writes degrade ([Smart Batching][smart-batching]); Fowler reports natural batching ~2x better than timeout-based batching in both best and worst cases ([Mechanical Sympathy principles][fowler-ms]).
- Cost: moderate — operator interfaces must accept a batch (slice/chunk), and tests must prove results are batch-size-invariant (a correctness trap: windows/patterns must produce identical output whether events arrive in batches of 1 or 1000).
- Verdict: **fits now** — the single highest-leverage runtime change: shard loop drains its mailbox into a chunk per wakeup and runs the chunk through validation→route and operator evaluation. No timeout needed, so no latency floor is added.

### 1.4 Wait strategies (block vs yield vs busy-spin)

How a consumer waits for work: park on a condition/channel (lowest CPU, microseconds of wakeup latency), yield-loop, or busy-spin on the sequence (lowest latency, 100% of a core). The Disruptor exposes this as a pluggable strategy; the paper notes spinning scales well because it avoids a shared lock ([Disruptor technical paper][disruptor-paper]).

- Effect **[measured in Disruptor context]**: the 52 ns mean above is with busy-spin; blocking strategies add scheduler wakeup latency (single-digit µs typical on Linux — **[folklore]** as an exact number, varies by kernel/load).
- Cost: busy-spin burns a core per consumer continuously; in Go it also fights the runtime scheduler.
- Verdict: **fits later, opt-in only** — default stays parked-on-channel (an embeddable library must not burn host cores); a config-gated "dedicated" mode (spin + `GOMAXPROCS`-aware shard count) is a plausible Phase 3 knob if µs-level wakeup cost is measured to matter against our sub-ms budget.

### 1.5 Aeron design principles — garbage-free, copy-light message paths

Thompson's Aeron (HFT-grade messaging) states eight principles: garbage-free in steady state, smart batching in the message path, wait-free algorithms, non-blocking IO, *no exceptional cases in the message path*, single-writer principle, prefer unshared state, avoid unnecessary data copies ([Aeron talk, InfoQ][aeron-infoq]).

- Effect **[claimed]**: Aeron's sustained multi-million msgs/s with low jitter is attributed to these principles collectively; no single-factor measurement is published.
- Cost: "no exceptional cases on the message path" means error/slow paths are designed out-of-band — more upfront design.
- Verdict: **fits now as a checklist** — directly transferable review criteria for our hot path: zero allocations in steady state, errors counted and routed off-path (we already count-and-drop null keys off-path, ADR-0006), one copy from ingest buffer into the canonical event, none after.

### 1.6 Seastar/ScyllaDB — shard-per-core, shared-nothing

One application thread per core, each owning a slice of data; no shared-memory locking between shards, cross-shard work via explicit message passing ([Seastar shared-nothing][seastar]; [ScyllaDB architecture][scylla-arch]). Requests landing on the wrong shard are forwarded to the owner — exactly our route-by-key-hash.

- Effect **[claimed]**: ScyllaDB's order-of-magnitude-vs-Cassandra numbers are vendor marketing; the transferable, well-supported part is the *mechanism*: no locks and no cache-line bouncing between cores.
- Cost: hot keys serialize on one shard (we accepted this in ADR-0006); full Seastar also brings its own IO/memory stack, which we don't need.
- Verdict: **fits now (validation)** — our sharded runtime is this pattern in Go; Seastar is evidence the shape is right, not new work. The Go caveat: goroutines are not pinned to cores, so we get shared-nothing *logically* but not guaranteed cache residency (see §2.9).

### 1.7 DPDK-style busy polling vs interrupts (concept only)

Kernel-bypass packet frameworks poll NIC queues continuously instead of taking interrupts: polling wastes cycles when idle but processes arrivals near-instantly; interrupt paths add syscall/copy latency ([Metronome paper on DPDK polling][metronome]; [DPDK Poll Mode Driver guide][dpdk-pmd]). Hybrids (Linux NAPI, Metronome's sleep&wake) switch modes by load.

- Effect **[measured]**: Metronome quantifies the trade-off (a polling core is 100% busy regardless of load).
- Cost: a dedicated core per poller; irrelevant complexity for a userspace, network-light library.
- Verdict: **does not fit** as infrastructure — we are not doing packet IO. The *concept* survives as §1.4's opt-in spin mode and as "drain greedily while work exists" (§1.3).

---

## 2. Go-specific high-performance engineering from real systems

### 2.1 NATS — hand-rolled zero-allocation protocol parser

NATS replaced regex-based protocol parsing with a hand-written, zero-allocation byte parser, avoided `defer` on the fast path, kept allocation near zero, and used byte-slice-keyed trie routing to avoid `[]byte`→`string` conversions ([GopherAcademy: NATS, high performance messaging in Go][nats-gopheracademy]; [Collison's 2012 design notes][collison-gist]).

- Effect **[measured, dated]**: ~2M msgs/s through the system on a 2012 laptop ([Collison gist][collison-gist]); the official `nats-bench` tutorial shows 7.67M msgs/s single-publisher with 16-byte messages on 2016-era setup ([natsbench docs][natsbench]). Current "millions/sec" marketing figures lack a published method — treat as **[claimed]**.
- Cost: hand-rolled parsers are real code to own and test (NATS's is a few hundred lines of state machine).
- Verdict: **fits now** — the strongest Go-native precedent for §3.5 (hand-rolled schema-driven JSON decode) and for keeping `defer`, reflection, and maps off the per-event path.

### 2.2 NATS — write coalescing (flusher goroutine)

A dedicated flusher goroutine coalesces many logical sends into few `write()` syscalls on the socket; senders signal it via a buffered channel ([GopherCon 2017 NATS client talk][nats-client-talk]). Same amortization as §1.3 applied to output.

- Effect **[claimed]**: central to NATS client throughput; no isolated public number.
- Cost: small — one goroutine and a buffer per sink; must bound the buffer (we already do: sinks own bounded queues).
- Verdict: **fits now** — our sink workers should drain their queue and write batches (one marshal buffer, one write), never one syscall per emitted event.

### 2.3 VictoriaMetrics — `sync.Pool` discipline and its pitfalls

VictoriaMetrics uses `sync.Pool` heavily (118 uses) but with rules: pool only for CPU-bound work (objects held across I/O get GC-cleared mid-flight and stolen across Ps), use *leveled* pools (array of 17 pools by size class) for variable-size buffers, and cap pooled capacity (they stop at 2^18 B — bigger gives no benefit and bloats memory) ([VictoriaMetrics: sync.Pool for CPU-bound operations][vm-syncpool]).

- Effect **[measured]**: Tyler Treat measured concurrent struct allocation 337 ns/op vs 65.5 ns/op pooled (~5x) ([So You Wanna Go Fast?][bravenewgeek]); VictoriaMetrics reports "significantly reduced allocations" without a single headline number.
- Cost: `sync.Pool` is drained at GC (also noted by [Cloudflare's 2013 predecessor post][cloudflare-recycle], now historical); pooled objects create aliasing hazards (§4.2); per-P stealing degrades under imbalance.
- Verdict: **fits now, buffers first** — pool marshal/parse scratch buffers (leveled, capped) on the CPU-bound hot path; pooling *events* themselves is §4.2 and needs the aliasing rules.

### 2.4 Go GC tuning — GOGC, GOMEMLIMIT (soft memory limit)

GOGC sets how much new heap may accumulate relative to live heap before a cycle (default 100); GOMEMLIMIT (Go ≥ 1.19) is a runtime-wide *soft* memory ceiling. The official guide's pattern for throughput-sensitive services: raise GOGC (fewer cycles, more CPU for work) and set GOMEMLIMIT as the safety net, leaving headroom below the container limit ([Go GC guide][gc-guide]; [runtime env docs][runtime-docs]). Heap ballast is the pre-1.19 hack and is obsolete.

- Effect **[measured per workload]**: GC cycles scale ~1/GOGC; the guide shows the cost model and warns death-spiral behavior if GOMEMLIMIT is set with no headroom.
- Cost: *process-global* knobs. A library that sets them reconfigures its host application.
- Verdict: **fits now for `cepd`, hands-off in the library** — the embedded engine must never call `SetGCPercent`/`SetMemoryLimit`; ship tuning guidance (and defaults in `cepd`) instead. The deeper fix is §2.5: allocate so little that GC tuning barely matters.

### 2.5 Escape analysis and allocation-free API design

The compiler stack-allocates values it can prove don't escape; APIs determine escape. Allocation-free Go APIs take caller-provided buffers (`AppendX(dst []byte, ...) []byte` à la `strconv.AppendInt`), return values not pointers, and avoid interfaces/closures on hot paths. Verify with `go build -gcflags=-m` and `testing.B.ReportAllocs` ([Dave Cheney, High Performance Go Workshop][cheney-workshop]; [Go GC guide][gc-guide]).

- Effect **[measured]**: heap alloc 62.3 ns/op + GC debt vs stack 11.6 ns/op in Treat's benchmarks ([So You Wanna Go Fast?][bravenewgeek]); more importantly, zero-alloc steady state removes GC pause/assist jitter from tail latency entirely — the mechanism behind Aeron's principle 1 and NATS §2.1.
- Cost: API rigidity (buffer-passing signatures), and escape analysis is version-fragile — guard with alloc-count benchmarks in the gate, not with faith.
- Verdict: **fits now** — a design rule for every hot-path package boundary (event struct, decoder, operator eval, sink marshal): "0 allocs/event in steady state" is testable and should be a gate tripwire.

### 2.6 Channels vs lock-free rings in Go — measured, not folklore

Go channels are a mutex-protected ring with scheduler integration. Measured comparisons: Treat found SPSC ring 80.9 ns/op vs channel 512 ns/op at GOMAXPROCS=1 (~6x) and 182 vs 542 ns/op at GOMAXPROCS=8 (~3x), but channels *beat* naive lock-free rings under some contention patterns, and his single-thread contention numbers were questioned by commenters ([So You Wanna Go Fast?][bravenewgeek]); an independent write-up reproduces the ~6x/~3x shape ([WeTest: make your Go program run faster][wetest]). "Channels are slow" as a blanket claim is **[folklore]** — a few hundred ns per op is noise next to a multi-µs JSON parse, and the gap shrinks with §1.3 batching (one channel op per *batch*).
- Cost of rings: hand-rolled lock-free code in Go is hard to get right (Treat found a bug in his own Ctrie) and loses channel/scheduler integration (select, blocking semantics we rely on for backpressure).
- Verdict: **channels fit now; rings fit later** — keep bounded channels as mailboxes (they implement our blocking backpressure for free); revisit per-mailbox only if the profiler shows channel ops dominating after batching.

### 2.7 Goroutine scheduling costs — keep the topology fixed

Goroutine switches are user-space and cheap relative to thread switches, but published figures disagree wildly: ~150 ns (academic yield benchmark, [Cforall thesis][cforall]), ~54 ns (Go 1.22 yield microbenchmark, [huizhou92][goroutine-switch]), ~1.4–1.7 µs for channel-roundtrip-style switches in older posts. Treat all as **[order-of-magnitude only]**. The design consequence is stable regardless: don't spawn per-event or per-key goroutines; keep a small fixed set ([Vyukov, Scalable Go Scheduler design doc][go-sched]).

- Cost: none for us — ADR-0006 already rejected goroutine-per-key for exactly this reason.
- Verdict: **fits now (already designed)** — fixed topology: N ingest + `shards` + sink workers; nothing dynamic on the hot path.

### 2.8 False sharing and cache-line padding in Go

Independent variables written by different goroutines but sharing a 64-byte cache line contend as if shared ([Disruptor technical paper][disruptor-paper] explains the mechanism). In Go: pad hot per-shard/per-P structs (e.g., `golang.org/x/sys/cpu.CacheLinePad` or trailing `[64]byte` arrays), and don't pack per-shard counters into one array of `uint64`.

- Effect **[measured, weakly]**: Treat: 156→132 ns/op SPSC (~15%) and 27,763→17,860 ns/op at 100P/100C (~36%) with padding ([So You Wanna Go Fast?][bravenewgeek]); blog claims of 20x ([besthub example][false-sharing-besthub]) lack published methodology — treat the magnitude as **[folklore]**, the mechanism as solid.
- Cost: trivial (struct layout + a comment); wasted bytes.
- Verdict: **fits now, cheaply** — lay out per-shard state and per-shard metrics so shards never write the same cache line; verify with `-cpu=1,2,4,8` benchmarks rather than trusting the blogs.

### 2.9 `runtime.LockOSThread` and CPU affinity

Pins a goroutine to its OS thread (needed for thread-local C APIs; usable with OS-level affinity to approximate Seastar-style core pinning) ([runtime docs][runtime-docs]). Go gives no portable core-pinning, so shard-to-core residency is best-effort.

- Effect: no credible published Go-middleware win from pinning alone; **[folklore]** territory.
- Cost: a locked goroutine monopolizes a thread; combined with channels it can add scheduler overhead ([golang-nuts thread][lockosthread-nuts]).
- Verdict: **fits later, opt-in with §1.4's dedicated mode only** — not in the default embeddable configuration.

### 2.10 Tailscale/Cloudflare precedents

Tailscale got >10 Gb/s through `wireguard-go` via batching at the syscall boundary (UDP generic segmentation/receive offload — many packets per syscall) plus checksum work ([Tailscale: Surpassing 10Gb/s][tailscale]) — the same amortization lesson as §1.3/§2.2 at a different layer. Cloudflare's 2013 buffer-recycling post is historical (superseded by `sync.Pool`, per its own banner) but documents why a GC'd language middleware pools buffers ([Cloudflare][cloudflare-recycle]). Vitess demonstrates the standing Go pattern of multiplexing many logical clients onto small pooled resource sets ([Vitess overview][vitess]).

- Verdict: **supporting evidence** — batching-at-boundaries and bounded pooling are the two recurring Go wins; no new mechanism beyond §1.3/§2.2/§2.3.

---

## 3. JSON/ingest parsing — the typical bottleneck

### 3.1 simdjson (the paper)

Two-stage SIMD parsing: stage 1 finds structural characters and validates UTF-8 with vector instructions across 64-byte blocks (branch-free); stage 2 builds a "tape" (flat index) without materializing a DOM of heap objects. First standard-compliant parser above gigabytes/second on one commodity core, using ≤¼ the instructions of RapidJSON ([Langdale & Lemire, arXiv:1902.08318][simdjson-paper]).

- Effect **[measured]**: multi-GB/s on one core (paper, with full methodology).
- Cost: C++, SIMD intrinsics, per-ISA code paths.
- Verdict: **does not fit as a dependency; fits as a design source** — the *tape* idea (index first, materialize never) informs our decoder (§3.5).

### 3.2 simdjson-go (MinIO port)

Go port using AVX2 assembly; validating.

- Effect **[claimed by maintainers]**: ~40–60% of C++ simdjson's speed; e.g., citm_catalog benchmark 1565 MB/s, ~15.6x over `encoding/json` ([minio/simdjson-go][simdjson-go]).
- Cost: amd64-only (requires AVX2+CLMUL), large assembly surface, slow release cadence (last listed release 2023); DataDog's fork is archived.
- Verdict: **does not fit** — ADR-0009 (stdlib first) + portability + maintenance risk outweigh a parse-stage win we can mostly capture schema-natively (§3.5).

### 3.3 bytedance/sonic

JIT-compiles schema-specialized codecs at runtime (via golang-asm) plus SIMD scanning ([bytedance/sonic][sonic]).

- Effect **[claimed vs measured]**: README claims large multiples over `encoding/json`; an independent benchmarker (author of rival `sonnet`, so interested) measured ~2x typical, found sonic ignores invalid UTF-8 and fails 43 JSONTestSuite cases vs 24 for `encoding/json` ([r/golang benchmark thread][sonic-reddit]).
- Cost: runtime JIT + `unsafe` + assembly; supported Go versions are a pinned range (e.g., 1.17–1.23 at the time checked) — every Go upgrade becomes a dependency event; first-call JIT latency needs `Pretouch`. Maximal tension with ADR-0009 and the "supply-chain ≈ the Go project" stance.
- Verdict: **does not fit** — correctness-first engine cannot take a parser that relaxes UTF-8/standards conformance, and the dependency policy excludes it anyway. Same reasoning, softer, for `goccy/go-json` (143 JSONTestSuite failures in the same comparison) and `json-iterator/go` (maintenance largely stalled).

### 3.4 The anti-pattern: `encoding/json` into `map[string]any`

Decoding to `map[string]any` allocates a map, boxed values, and strings per field, then forces reflection-style access. This is the slowest common path and the one our schema makes unnecessary.

- Effect **[measured]**: reflection-based JSON vs generated code: 9,362 vs 5,839 ns/op unmarshal in Treat's run ([So You Wanna Go Fast?][bravenewgeek]); simdjson's instruction-count analysis attributes most parser cost to materialization and branching ([simdjson paper][simdjson-paper]).
- Verdict: **does not fit** — forbidden on the hot path from day one.

### 3.5 Hand-rolled schema-driven decoding (the recommendation)

We know the schema before the first event arrives. Compile each stream's schema into a field table (expected keys, types, target offsets in the canonical event), then run a single-pass byte scanner over the JSON: match keys (length+byte compare, no string alloc), parse scalars directly into the event's typed slots, validate as you parse (one pass does decode+schema validation), skip unknown fields without materializing them. This is NATS's parser discipline (§2.1) applied to JSON, with simdjson's "don't build a DOM" insight (§3.1), in pure Go.

- Effect **[inferred from measured parts]**: eliminates the map/boxing allocations (§3.4) and the separate validation pass; pure-Go schema-specialized decoders (goccy, easyjson-style codegen) demonstrate 2–5x over `encoding/json` without assembly — our in-repo equivalent targets that band. Must be verified by our own gate benchmarks; no one has measured *our* decoder.
- Cost: an in-repo component (likely < 1k lines + conformance tests against `encoding/json` on valid inputs and JSONTestSuite-style corpora); must handle escapes/UTF-8/numbers with full standards conformance — this is where the hand-rolled approach earns or loses its keep.
- Verdict: **fits now** — the single biggest hot-path win available inside ADR-0009; differential-test it against `encoding/json` (same accept/reject + same values) to keep the correctness bar.

---

## 4. Event-processing-specific techniques

### 4.1 Batching/chunking through operator pipelines

Stream engines pass *chunks* of events between operators, not single events. Siddhi's `ComplexEventChunk` is a linked chunk handed to each `Processor.process(chunk)` ([siddhi-core source][siddhi-chunk]; maintainers confirm events that arrive together are processed together, [StackOverflow answer][siddhi-so]). Flink buffers records into network buffers with a flush timeout: default 100 ms; lowering it trades throughput for latency because "per-buffer overhead is significantly higher than per-record overhead"; timeout 0 = flush per record ([Flink network stack deep dive][flink-deepdive]; [Flink network buffer tuning][flink-tuning]). The Disruptor gets the same effect implicitly (§1.3) *without a timeout*, because consumers drain to the published cursor.

- Effect **[measured]**: Flink's own docs/deep-dive document the throughput/latency curve vs buffer timeout; Disruptor paper documents flat latency under load via batch draining.
- Cost: chunk-shaped operator APIs; a timeout-based design (Flink's) adds a latency floor — a drain-based design (Disruptor's) doesn't, which matters for a sub-ms target.
- Verdict: **fits now, drain-based** — mailbox drain (§1.3) feeds operator evaluation in chunks; explicitly *no* flush timer on the hot path. Tests assert batch-size invariance of outputs.

### 4.2 Object pooling of events — and the aliasing hazard

Reusing event objects avoids per-event allocation (§2.3), but a pooled event returned to the pool while a window, pattern partial-match, or sink still references it is a use-after-return bug: state silently mutates, producing wrong detections that may differ run-to-run. VictoriaMetrics' rules (§2.3) handle buffers; *events with multi-owner lifetimes* are the dangerous case.

- Effect: §2.3's ~5x allocation-cost numbers **[measured]**; the hazard is documented practice, not speculation (every pooling writeup carries the caveat; `sync.Pool` docs require that callers no longer reference returned objects).
- Cost: needs an ownership rule. Two safe designs: (a) events are immutable after ingest and *not pooled* while any window can retain them — pool only scratch buffers; (b) reference-counted events — complexity and a correctness risk our ground rules (11: stop-the-line for non-deterministic replay) price very high.
- Verdict: **fits later, restricted** — pool ingest/marshal buffers now (§2.3); do *not* pool canonical events until a measured GC problem exists *and* window retention semantics give a provable release point. Determinism beats allocation count.

### 4.3 Hierarchical timing wheels for timers/TTL/window expiry

Varghese & Lauck's wheels give O(1) start/stop/expire for huge timer populations: a ring of buckets per resolution tier, overflow cascading to coarser wheels ([Hashed and Hierarchical Timing Wheels, SOSP '87; summary][timing-wheels]; Kafka rebuilt its purgatory on them for O(1) insert/delete at tens of thousands of outstanding timers, [Confluent][kafka-purgatory]). Go's `time.Timer` is a per-timer heap entry — fine for thousands, costly for one timer per (rule, key) instance at high cardinality.

- Effect **[measured]**: Kafka's redesign numbers (purgatory redesign proposal) and the paper's complexity analysis; Go-side, heap-timer churn at high cardinality shows up as `runtime.siftdownTimer` in profiles — **[folklore]** until we profile it ourselves.
- Cost: a small in-repo wheel (a few hundred lines) driven by the clock service (ADR-0004); resolution becomes a config decision; playback must drive the wheel from the virtual clock.
- Verdict: **fits now (design the timer facade for it), build when cardinality demands** — route all TTL/window timers through one per-shard timer facility from the start so the wheel can replace stdlib timers without touching operators.

### 4.4 String interning for repeated keys/stream names

Stream names, rule IDs, and field keys repeat per event; interning stores one canonical copy and compares by pointer/handle. Go 1.23's stdlib `unique` package provides this with GC-integrated cleanup — `Handle[T]` comparison is pointer-cheap ([Go blog: New unique package][unique-blog]). VictoriaMetrics reports up to 3x memory reduction interning metric metadata, works best for read-heavy/low-variety strings, and warns interning high-cardinality values *increases* memory and CPU ([VictoriaMetrics: strings interning][vm-interning]).

- Effect **[measured]**: VictoriaMetrics' 3x memory figure on their workload; use Go ≥ 1.23.2 (earlier 1.23 retained original strings).
- Cost: near-zero (stdlib); the discipline is choosing *what* to intern: names and schema keys yes, event payload values no.
- Verdict: **fits now** — intern stream/rule/field identifiers at ingest/compile time; never intern unbounded user values. Better still, our schema-driven decoder (§3.5) resolves field keys to table indices, which out-performs interning on the hot path.

---

## 5. Measurement discipline

### 5.1 Coordinated omission (Gil Tene)

Load generators that wait for a response before sending the next request silently drop exactly the samples that occur *during* a stall, understating high percentiles by orders of magnitude ([How NOT to Measure Latency, InfoQ][tene-talk]; [mechanical-sympathy thread][co-thread]). The fix: issue load on a fixed schedule and measure from *intended* send time, or apply HdrHistogram's expected-interval correction.

- Effect **[measured/demonstrated]**: Tene's talks demonstrate multi-order-of-magnitude p99.9 errors in common tools.
- Cost: the load harness must be open-loop (scheduled arrivals), slightly more complex than a closed loop.
- Verdict: **fits now** — our throughput/latency gate benchmarks must be open-loop from day one, or every sub-ms claim we make is fiction.

### 5.2 HdrHistogram and percentiles done right

Constant-time recording into log-linear buckets with bounded relative error; report the full percentile spectrum (p50/p90/p99/p99.9/max), never means; max is a first-class number ([HdrHistogram][hdrhistogram]). A Go port exists ([HdrHistogram/hdrhistogram-go][hdr-go]), but the data structure is small enough to implement in-repo under ADR-0009.

- Verdict: **fits now** — in-repo log-linear histogram for the bench harness and runtime latency metrics (our hand-written Prometheus text exposition already anticipates histograms).

### 5.3 Go benchmarking traps and `testing.B.Loop`

Classic traps: the compiler dead-code-eliminates a benchmark body whose result is unused, inlines-and-folds constants, or the loop measures harness overhead. Go 1.24's `b.Loop()` makes the compiler keep loop bodies (inlining into the loop is disallowed) and excludes setup from timing ([Go blog: testing.B.Loop][bloop]). Pre-1.24 style needs sinks/`KeepAlive`. Also: benchmark on quiet machines, fixed CPU counts (`-cpu`), and report allocs (`-benchmem`).

- Verdict: **fits now** — standardize on `b.Loop()` + `ReportAllocs`; a gate check greps for benchmarks without sinks on older patterns.

### 5.4 benchstat and continuous benchmarking tripwires

`benchstat` compares benchmark runs with proper statistics (needs ≥ 10 runs via `-count`, reports deltas with confidence; refuses to call noise a regression) ([golang.org/x/perf/cmd/benchstat][benchstat]). CI pattern: store a baseline, run `-count=10` on the perf-critical packages, fail the gate on statistically significant regression beyond a threshold (e.g., >5% on hot-path ns/op or any alloc-count increase).

- Cost: benchmark runtime in CI (minutes) and noise management (dedicated runner or higher thresholds); `x/perf` is a build-time tool — same family as our sanctioned goyacc (needs owner sign-off per ADR-0009).
- Verdict: **fits now** — this is the "tests are the spec" ground rule applied to performance: alloc-count and latency tripwires make every later optimization provable and every regression loud.

---

## 6. Top 10 techniques ranked by expected impact for this engine

| # | Technique | Source basis | Verdict | Expected impact | Guardrail |
|---|---|---|---|---|---|
| 1 | Schema-driven, allocation-free JSON decode (single pass, no `map[string]any`, no reflection) | §3.1, §3.5; NATS §2.1 | fits now | Largest single hot-path win; parse typically dominates ingest (simdjson: most parser cost is materialization) | Differential-test vs `encoding/json`; full RFC 8259 conformance |
| 2 | Zero allocations per event in steady state (escape-analysis-driven APIs, buffer passing) | §2.5; Aeron p.1 | fits now | Removes GC assist/pause jitter from tail latency; the sub-ms p99.9 enabler | `ReportAllocs` tripwire: 0 allocs/event in gate |
| 3 | Natural batching: drain mailbox per wakeup, chunk through operators (no flush timer) | §1.3, §4.1 | fits now | Amortizes channel ops/wakeups; latency flat under burst (Disruptor-measured) | Tests prove output is batch-size-invariant |
| 4 | Coalesced sink writes (flusher per sink, batch marshal + write) | §2.2; Tailscale §2.10 | fits now | Syscall/flush amortization; keeps shards decoupled from sink I/O | Bounded sink queues (already ADR-0008) |
| 5 | Open-loop benchmark harness + HdrHistogram percentiles + benchstat CI tripwires | §5.1–5.4 | fits now | Meta-impact: makes items 1–4 provable and regressions loud; prevents fictitious sub-ms claims | In-repo histogram; `x/perf` needs owner sign-off |
| 6 | Single-writer/shared-nothing enforcement (per-shard state+counters, no shared atomics, padded layout) | §1.2, §1.6, §2.8 | fits now (already designed) | Preserves the architecture's headroom; cheap insurance against contention creep | Review rule + `-cpu` scaling benchmarks |
| 7 | Leveled, capped `sync.Pool` for scratch buffers (CPU-bound path only) | §2.3 | fits now | ~5x on buffer alloc cost; keeps steady-state allocs at 0 where buffers are unavoidable | VM rules: leveled, capped, never across I/O waits |
| 8 | Per-shard hierarchical timing wheel behind a timer facade | §4.3 | facade now, wheel when measured | O(1) timers at high (rule,key) cardinality; stdlib timer heap is the known cliff | Driven by clock service; playback uses virtual clock |
| 9 | Interning of stream/rule/field identifiers (`unique`, Go ≥ 1.23.2) + schema key→index resolution | §4.4, §3.5 | fits now | Memory + compare-cost reduction on metadata (VM: up to 3x memory on their workload) | Never intern unbounded payload values |
| 10 | Opt-in dedicated mode: spin-wait shards / `LockOSThread` | §1.4, §2.9 | fits later | Shaves scheduler-wakeup µs off p99 for dedicated deployments | Never default: embeddable library must not burn host cores |

Explicitly rejected for this engine: sonic / simdjson-go / goccy as dependencies (§3.2–3.3 — dependency policy, conformance, portability); DPDK-style infrastructure (§1.7); event-object pooling until ownership semantics make it provably safe (§4.2); goroutine-per-key (§2.7, already rejected in ADR-0006).

---

## References (all accessed 2026-11-01)

1. Thompson, Farley, Barker, Gee, Stewart — *Disruptor: High performance alternative to bounded queues* (LMAX technical paper): <https://lmax-exchange.github.io/disruptor/disruptor.html>
2. Thompson — *Single Writer Principle*: <https://mechanical-sympathy.blogspot.com/2011/09/single-writer-principle.html>
3. Thompson — *Smart Batching*: <https://mechanical-sympathy.blogspot.com/2011/10/smart-batching.html>
4. Fowler — *Principles of Mechanical Sympathy*: <https://martinfowler.com/articles/mechanical-sympathy-principles.html>
5. Thompson — *Aeron: Open-source high-performance messaging* (InfoQ talk): <https://www.infoq.com/presentations/aeron/>
6. Seastar — *Shared-nothing design*: <https://seastar.io/shared-nothing/>
7. ScyllaDB — *Architecture*: <https://www.scylladb.com/technology/>
8. DPDK — *Poll Mode Driver* (programmer's guide): <https://doc.dpdk.org/guides/prog_guide/poll_mode_drv.html>
9. Faltelli et al. — *Metronome: adaptive and precise intermittent packet retrieval in DPDK*: <https://arxiv.org/pdf/2103.13263>
10. GopherAcademy — *NATS: High performance cloud native messaging in Go*: <https://blog.gopheracademy.com/advent-2015/nats-high-performance-cloud-native-messaging-written-in-go/>
11. Collison — 2012 NATS/messaging design + benchmark notes (gist): <https://gist.github.com/derekcollison/4227635>
12. NATS docs — *nats-bench tutorial*: <https://docs.nats.io/using-nats/nats-tools/nats_cli/natsbench>
13. NATS — *Writing networking clients in Go* (GopherCon 2017 slides): <https://www.slideshare.net/nats_io/writing-networking-clients-in-go-gophercon-2017-talk>
14. VictoriaMetrics — *sync.Pool for CPU-bound operations*: <https://victoriametrics.com/blog/tsdb-performance-techniques-sync-pool/>
15. VictoriaMetrics — *Strings interning*: <https://victoriametrics.com/blog/tsdb-performance-techniques-strings-interning/>
16. Go team — *A Guide to the Go Garbage Collector*: <https://go.dev/doc/gc-guide>
17. Go `runtime` package docs (GOGC, GOMEMLIMIT, LockOSThread): <https://pkg.go.dev/runtime>
18. Cheney — *High Performance Go Workshop* (GopherCon 2019 edition): <https://dave.cheney.net/high-performance-go-workshop/gophercon-2019.html>
19. Treat — *So You Wanna Go Fast?* (measured Go benchmarks; 2016 — defer numbers predate Go 1.13 open-coded defers): <https://bravenewgeek.com/so-you-wanna-go-fast/>
20. WeTest — *How to make your Go program run faster* (ring vs channel): <https://www.wetest.net/blog/how-to-make-your-go-program-run-faster-861.html>
21. Vyukov — *Scalable Go Scheduler Design Doc*: <https://golang.org/s/go11sched>
22. huizhou92 — *Performance analysis of goroutine switching*: <https://huizhou92.com/p/performance-analysis-of-goroutine-switching/>
23. Delisle — Cforall thesis context-switch measurements: <https://cforall.uwaterloo.ca/trac/export/2db79e570c15d201eae79cb4c2e21c234c6c3524/doc/proposals/concurrency/text/results.tex>
24. golang-nuts — LockOSThread + channel performance thread: <https://groups.google.com/g/golang-nuts/c/WqVEThZnQkM>
25. Tailscale — *Surpassing 10Gb/s over Tailscale*: <https://tailscale.com/blog/more-throughput>
26. Cloudflare — *Recycling memory buffers in Go* (historical; superseded by sync.Pool): <https://blog.cloudflare.com/recycling-memory-buffers-in-go>
27. Vitess — *What is Vitess* (connection pooling): <https://vitess.io/docs/overview/whatisvitess>
28. Langdale, Lemire — *Parsing Gigabytes of JSON per Second* (simdjson), arXiv:1902.08318: <https://arxiv.org/abs/1902.08318>
29. minio/simdjson-go: <https://github.com/minio/simdjson-go>
30. bytedance/sonic: <https://github.com/bytedance/sonic>
31. r/golang — independent JSON codec benchmarks + JSONTestSuite conformance comparison (sonnet author): <https://www.reddit.com/r/golang/comments/16u956j/the_latest_json_encodedecode_benchmarks_and/>
32. Siddhi — `ComplexEventChunk` source: <https://github.com/siddhi-io/siddhi/blob/master/modules/siddhi-core/src/main/java/io/siddhi/core/event/ComplexEventChunk.java>
33. StackOverflow — *Why do all events come in bulk* (Siddhi maintainer answer): <https://stackoverflow.com/questions/31541385/why-do-all-events-come-in-bulk-rather-than-one-by-one>
34. Apache Flink — *A Deep-Dive into Flink's Network Stack*: <https://flink.apache.org/2019/06/05/a-deep-dive-into-flinks-network-stack/>
35. Apache Flink — *Network Buffer Tuning* (buffer timeout): <https://nightlies.apache.org/flink/flink-docs-release-1.14/docs/deployment/memory/network_mem_tuning/>
36. Varghese, Lauck — *Hashed and Hierarchical Timing Wheels* (SOSP '87; The Morning Paper summary): <https://blog.acolyer.org/2015/11/23/hashed-and-hierarchical-timing-wheels/>
37. Confluent — *Apache Kafka, Purgatory, and Hierarchical Timing Wheels*: <https://www.confluent.io/blog/apache-kafka-purgatory-hierarchical-timing-wheels/>
38. Knyszek — *New unique package* (Go blog): <https://go.dev/blog/unique>
39. Tene — *How NOT to Measure Latency* (InfoQ): <https://www.infoq.com/presentations/latency-pitfalls/>
40. mechanical-sympathy — *Coordinated Omission* thread (Tene): <https://groups.google.com/g/mechanical-sympathy/c/icNZJejUHfE>
41. HdrHistogram: <http://hdrhistogram.org/> ; Go port: <https://github.com/HdrHistogram/hdrhistogram-go>
42. Go blog — *More predictable benchmarking with testing.B.Loop*: <https://go.dev/blog/testing-b-loop>
43. benchstat (golang.org/x/perf): <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>

[disruptor-paper]: https://lmax-exchange.github.io/disruptor/disruptor.html
[single-writer]: https://mechanical-sympathy.blogspot.com/2011/09/single-writer-principle.html
[smart-batching]: https://mechanical-sympathy.blogspot.com/2011/10/smart-batching.html
[fowler-ms]: https://martinfowler.com/articles/mechanical-sympathy-principles.html
[aeron-infoq]: https://www.infoq.com/presentations/aeron/
[seastar]: https://seastar.io/shared-nothing/
[scylla-arch]: https://www.scylladb.com/technology/
[dpdk-pmd]: https://doc.dpdk.org/guides/prog_guide/poll_mode_drv.html
[metronome]: https://arxiv.org/pdf/2103.13263
[nats-gopheracademy]: https://blog.gopheracademy.com/advent-2015/nats-high-performance-cloud-native-messaging-written-in-go/
[collison-gist]: https://gist.github.com/derekcollison/4227635
[natsbench]: https://docs.nats.io/using-nats/nats-tools/nats_cli/natsbench
[nats-client-talk]: https://www.slideshare.net/nats_io/writing-networking-clients-in-go-gophercon-2017-talk
[vm-syncpool]: https://victoriametrics.com/blog/tsdb-performance-techniques-sync-pool/
[vm-interning]: https://victoriametrics.com/blog/tsdb-performance-techniques-strings-interning/
[gc-guide]: https://go.dev/doc/gc-guide
[runtime-docs]: https://pkg.go.dev/runtime
[cheney-workshop]: https://dave.cheney.net/high-performance-go-workshop/gophercon-2019.html
[bravenewgeek]: https://bravenewgeek.com/so-you-wanna-go-fast/
[wetest]: https://www.wetest.net/blog/how-to-make-your-go-program-run-faster-861.html
[go-sched]: https://golang.org/s/go11sched
[goroutine-switch]: https://huizhou92.com/p/performance-analysis-of-goroutine-switching/
[cforall]: https://cforall.uwaterloo.ca/trac/export/2db79e570c15d201eae79cb4c2e21c234c6c3524/doc/proposals/concurrency/text/results.tex
[lockosthread-nuts]: https://groups.google.com/g/golang-nuts/c/WqVEThZnQkM
[tailscale]: https://tailscale.com/blog/more-throughput
[cloudflare-recycle]: https://blog.cloudflare.com/recycling-memory-buffers-in-go
[vitess]: https://vitess.io/docs/overview/whatisvitess
[simdjson-paper]: https://arxiv.org/abs/1902.08318
[simdjson-go]: https://github.com/minio/simdjson-go
[sonic]: https://github.com/bytedance/sonic
[sonic-reddit]: https://www.reddit.com/r/golang/comments/16u956j/the_latest_json_encodedecode_benchmarks_and/
[siddhi-chunk]: https://github.com/siddhi-io/siddhi/blob/master/modules/siddhi-core/src/main/java/io/siddhi/core/event/ComplexEventChunk.java
[siddhi-so]: https://stackoverflow.com/questions/31541385/why-do-all-events-come-in-bulk-rather-than-one-by-one
[flink-deepdive]: https://flink.apache.org/2019/06/05/a-deep-dive-into-flinks-network-stack/
[flink-tuning]: https://nightlies.apache.org/flink/flink-docs-release-1.14/docs/deployment/memory/network_mem_tuning/
[timing-wheels]: https://blog.acolyer.org/2015/11/23/hashed-and-hierarchical-timing-wheels/
[kafka-purgatory]: https://www.confluent.io/blog/apache-kafka-purgatory-hierarchical-timing-wheels/
[unique-blog]: https://go.dev/blog/unique
[tene-talk]: https://www.infoq.com/presentations/latency-pitfalls/
[co-thread]: https://groups.google.com/g/mechanical-sympathy/c/icNZJejUHfE
[hdrhistogram]: http://hdrhistogram.org/
[hdr-go]: https://github.com/HdrHistogram/hdrhistogram-go
[bloop]: https://go.dev/blog/testing-b-loop
[benchstat]: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
[false-sharing-besthub]: https://besthub.dev/articles/understanding-false-sharing-and-cache-padding-in-go-0345a989edd2
