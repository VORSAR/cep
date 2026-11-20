# Architecture review: `(*Decoder).value` under ground rule 13a

> **Historical — S1.1c-6, the strict lint set: the size limits in runtime, validate, event, parser, plan and playback; merged in PR #49.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-6-plan-check.md) · [design](../sprints/s1-1c-6-design.md) · [record](../sprints/s1-1c-6-record.md) · reviews: [spike](s1-1c-6-spike.md), [plan check architecture](s1-1c-6-plan-check-architecture.md), [plan check qa](s1-1c-6-plan-check-qa.md), [design architecture](s1-1c-6-design-architecture.md), [design qa](s1-1c-6-design-qa.md), [event walkers](s1-1c-6-event-walkers-review.md), [decoder value](s1-1c-6-decoder-value-review.md), [playback](s1-1c-6-playback-review.md), [combined architecture](s1-1c-6-combined-architecture.md), [breakit qa](s1-1c-6-breakit-qa.md). **← [S1.1c-5](../sprints/s1-1c-5-record.md) · [S1.1c-7 (next)](../sprints/sprint-plan.md#status) →**

Reviewer: architect-reviewer (fresh agent), 2026-11-18. Scope: `(*Decoder).value` in `internal/event/decode.go`,
lane E's worktree at `2b86876` (`value` whole), the design's split `1e48b3b` and its parent `9a11fcb`. Nothing in any
repository or worktree was changed; every prototype and log is in the scratch directory
`/private/tmp/claude-501/-Users-ramith-code-cep/e3315a83-1fa1-43ac-8afe-3d451de6f832/scratchpad/value-review/`
(called `<scratch>` below). Toolchain go1.27.4 darwin/arm64 (the pinned one), golangci-lint v2.15.0 with the repo's
`.golangci.toml`. This review recommends; the owner decides.

**Code graph.** The GitNexus index is of `main`: `(*Decoder).value` at `decode.go:130`, and the scanner's methods still
in `decode.go`. Lane E's worktree, with the scanner in `scanner.go` and the new benchmarks, is not in it; I could not
read `index.commit` (no MCP resource reader in this session).
- `impact((*Decoder).value, upstream)`: one direct caller, `(*Decoder).Decode`; at depth 2, tests and the playback
  reader (`record`, `rejectedRecord`, `eventRecord`).
- Every option keeps the signature's type, so no caller changes.

## 1. Why the split is 37 % slower

**Root cause: `Value` cannot live in registers inside a function, so every function boundary or inlined helper that
hands a `Value` over adds an 88-byte zero, write, copy and reload.**

### 1.1 The compiler fact

- `Value` has 9 fields and is 88 bytes (11 words: `Kind`, `Str` x2, `I`, `F`, `B`, `T`, `D`, `Obj` x2, `Null`).
- Go's SSA backend keeps a struct in registers only if it has at most 4 fields and is at most 4 words:
  `CanSSA` in `src/cmd/compile/internal/ssa/value.go` (`if t.NumFields() > MaxStruct { return false }`), where
  `MaxStruct = 4` (`ssa/decompose.go:412`) in the pinned toolchain.
- So every `Value` local, named or unnamed result, and inlined temporary is a stack object.
- The register ABI still passes the result in registers: R0-R9 and F0 for the `Value`, R10/R11 for the `error`
  (`args=0x48`: only the 9 argument words have stack spill slots). But each side of a call builds it in memory first:
  - the callee zeroes its result slot (5 `STP` + 1 `MOVD`), stores the fields, and loads all 11 words into registers;
  - a caller that returns the value onward stores the 11 registers to a temporary, reloads them, and copies the
    temporary into its own result slot.
- The design note's "an 11-word `Value` returned through memory" (E.2.4) names the right object, but the mechanism is
  slightly different: the `Value` *travels* in registers and is *materialised* in memory on both sides, plus a copy.

### 1.2 Generated code, today against the split

Listings: `<scratch>/value-2b86876.short`, `<scratch>/value-1e48b3b.short`, `<scratch>/stringValue-1e48b3b.short`
(full `-S` output in `<scratch>/asm-*.s`; inlining in `<scratch>/m2-*.txt`).

- **Today (`2b86876`): one `Value` slot, built in place.**
  - `value` is 3376 bytes of code, frame 464 bytes.
  - It zeroes `~r0` at entry. Each accept arm zeroes it again, stores the fields and loads the 11 result registers.
    For the string arm that is `decode.go:153`, offsets 0x2c0-0x328: `CALL decodeString`, 6 zeroing stores,
    2 field stores, register moves, `RET`.
  - **No 128-bit (`FLDPQ`) copies anywhere in the function (0).**
- **The split (`1e48b3b`): for each of the 9 non-reserved attributes, `value` calls a helper that does the same build
  in its own slot, and then `value` copies the result.**
  - `value` is 1680 bytes, frame 272 bytes. The helpers are 400-576 bytes each, frames 176-288 bytes.
  - The string path (`decode.go:143`, `return stringValue(a, raw, sp.start)`):
    1. **Before the call (8 instructions):** 7 argument moves and the `CALL`.
    2. **In `stringValue` (about 24 instructions before its work starts):** the stack-bound check, frame push, 2
       argument spills and a 6-store zeroing of its own result slot. Its body then repeats today's arm.
    3. **After the return, in `value` (offsets 0x190-0x200, 29 instructions):**
       - 8 stores of the returned registers to `.autotmp_33`;
       - 8 loads back;
       - an 88-byte copy of `.autotmp_33` into `value`'s `~r0`: 2 `FLDPQ`/`FSTPQ` 32-byte pairs, a `FMOVQ` pair and a
         `MOVD` pair. The copy is dead, because the registers are already loaded, but the compiler cannot drop it
         (`~r0` is a memory object);
       - frame pop and `RET`.
  - Every arm has that 29-instruction tail: 16 `FLDPQ` in `value` (8 copy sites), against 0 today.
  - **Per attribute that is about 50 extra instructions, about 34 of them loads and stores.**
  - Inferred: the 32-byte `FLDPQ` loads read a slot written a few cycles earlier by 8 narrower stores. On most cores
    that cannot be store-forwarded, so the load waits for the stores to reach the cache.
- **Inlining (`-m=2`):**
  - `nullable` (cost 6) and `timeNote` (6) inline.
  - The eight helpers do not: `absentValue` 112, `nullValue` 114, `stringValue` 137, `intValue` 154, `floatValue` 163,
    `boolValue` 85, `timestampValue` 289, `objectValue` 144.
  - Each helper calls one non-inlinable worker (`decodeString` 146, `parseInt` 106, `strconv.ParseFloat`,
    `parseRFC3339` 707, `cleanObject` 292) and one more for its fault (`typeErr` 176, or `fmt.Sprintf`). Two calls are
    already more than the inliner's budget of 80.
  - `boolValue` (85) calls only `typeErr` and still misses, because returning a `Value` literal costs budget too.
- **The reserved, absent and null preamble:**
  - The reserved slot returns directly in both versions, so it costs nothing extra.
  - Absent and null now go through `absentValue`/`nullValue`: the same call and copy, 2 of the 10 attributes.
- **Numbers:**
  - The coordinator measured 115.9-116.1 to 159.0 ns. That is about 43 ns over 9 helper calls, about 4.8 ns per call,
    about 22 cycles (counters: +197 process cycles per op).
  - My re-measure, the split put in today's tree: +39.2 % (113.85 to 158.5 ns, p = 1.1e-05).
  - `BenchmarkDecoderValueFaults` hardly moves (+2.1 %, +2.7 %): each fault already costs about 240 ns in
    `fmt.Sprintf` and an allocation, so the extra call and copy are noise there.

### 1.3 Controls that separate the causes (indicative; see section 3 for conditions)

All against today's binary, `BenchmarkDecoderValue`, 10 ABBA pairs.

| Prototype | What it changes | Result |
|---|---|---|
| layout control | today's code shifted 192 bytes by an unused function placed before `value` (`layoutPad`) | **+1.3 %** (p = 0.12, no cost). Placement alone moves this benchmark by about 1.5 %, and up to 3.3 % on the 16-attribute one (section 3). |
| P1 | inlinable `Value`-returning helpers. `stringOf`, `boolOf` and `timestampOf` inline. `intOf` (91), `floatOf` (101) and `objectOf` (82) do not. | **+62.4 %** (p = 1.1e-05) |
| P3 | scalar readers, inlined except float and timestamp, plus an inlined `typed(a, Value{...}, want, raw, at)` that takes a `Value` | **+53.0 %** (p = 1.1e-05) |
| P2 | destination slot: one non-inlined filler per type writes `*v = Value{...}` and returns `error`. No copy on present paths, still one call per attribute, and the inlined `nullValue` still copies. | **+32.7 %** (p = 1.1e-05) |

- **Load-independent counters, the same story.** `<scratch>/tools/counters.py` reads per-process cycles and instructions
  retired from `/usr/bin/time -l`, 6 alternating rounds of `-test.benchtime=2000000x` (`<scratch>/logs/counters-*.log`).
  Process cycles count only this process's cycles, so they hold under other load: they reproduced the coordinator's
  quiet +37 % for the split while the load average was 44.
  - **The split, put in today's tree (`<scratch>/split`):** +14.3 % instructions (+452 per op, about 50 for each of the
    9 helper calls, as the listing predicts) but **+37.9 % cycles**.
  - **About 22 cycles more per helper call for about 50 instructions.** At this core's usual 6 instructions per
    cycle, 50 instructions cost about 8 cycles. So **most of the split's cost is stalls, not instructions**, which
    fits loads waiting on the stores just made (the wide copy reading the narrow stores; inferred).
  - The layout control: +1.3 % / -0.0 % cycles, +0.4 % instructions.
- **What the controls show:**
  - **P3 has fewer calls than the split and is slower.** Its frame is 1552 bytes, with 30 `FLDPQ`.
    Per string attribute it zeroes the inlined `v` (6 stores) and the inlined result (6 stores), then copies twice:
    `v` to the inlined result, and that to `~r0` (`<scratch>/value-p3.short`, 0x220-0x324).
  - **An inlined helper that takes or returns a `Value` is no cheaper than a call.** The inliner turns its parameter and
    result into stack temporaries and copies them.
  - **P2 removes the copies on present paths and keeps the call.** Its +32.7 % says an extra call level per attribute
    also costs about 3-4 ns on its own: 9 argument registers shuffled, a frame, and a stack check.
- **Conclusion:** for this function, any accept path that adds a non-inlined call, or that passes a `Value` through a
  helper, costs whole percent per attribute. A shape that is as fast as today must keep the `Value` in `value`'s own
  result slot and add no call on the common accept paths.

## 2. Options, ranked by code health

**Requirements** (architecture-designer step 1, from the brief): identical results and errors on every input;
cyclomatic ≤ 10 and ≤ 60 lines for every function with no waiver; no new heap allocation; at least today's speed (or
the gap stated); `value`'s signature kept; no handler table on the hot path.

**Common ground for every option below:**
- **Behaviour:** the whole `internal/event` suite passes, including:
  - the nets `TestDecodeValueEachOperand` (`decode_value_test.go`) and `value_operands_test.go`;
  - `TestBreakitDecodeEdgesAgreeWithOracle` and the other breakit tests;
  - `TestDecodeLeafAllocations` and `TestDecodeAllocationBudget`;
  - `FuzzDecode`'s seeds.
- **The differential test:** `<scratch>/zz_value_diff_test.go`, placed in each prototype as
  `internal/event/zz_value_diff_test.go`, passes.
  - The oracle is `valueToday`, today's `value` copied verbatim and renamed.
  - `TestReviewValueDiffExhaustive` runs 13 608 cases: 63 hand-picked raws × 9 types (`Type(0)` and `Type(99)`
    included, to reach the default arm) × optional or not × plain/time/reserved slot × seen or not × two offsets.
  - `TestReviewValueDiffRandom` runs 300 000 generated cases: corpus mutations, random integers and floats, random
    bytes, random roles.
  - It compares the `Value`, the `*DecodeError` field by field (`Reason`, `Attr`, `Offset`, `Msg`), and any panic text.
  - **It catches a planted fault:** five mutations of the leading option (int field swapped, float error ignored, time
    note dropped, `[` refused, `Kind` not set) each turned both tests red.
- **Allocations:** none added; allocs/op and B/op are equal in every comparison below.
- **Size:** the repo's golangci-lint reports nothing in `decode.go`. The only findings are the two scanner functions
  owned by the walkers-and-scanner redesign. No waiver.
- **The `value` signature:** the same type: `func (d *Decoder) value(b []byte, i int, sp span, closing int) (Value,
  error)`. O1-O3 name the results `(v Value, err error)`, and the name is load-bearing (see O1). Callers and tests are
  unchanged.
- **No handler table:** a `switch` of direct calls or assignments only.

### O1 (recommended, with a decision-1 exception): readers return the Go value, written into `value`'s result slot

Prototype `<scratch>/p5/internal/event/decode.go` (extract `<scratch>/p5-value.go.txt`).

```go
func (d *Decoder) value(b []byte, i int, sp span, closing int) (v Value, err error) {
	a := d.schema.attrs[i]
	if i == d.reserved {
		return Value{Kind: TBool}, nil
	}
	isTime := i == d.schema.timeAttr
	if !sp.seen {
		err = nullInto(&v, a, absentFault(a, isTime, closing))
		return v, err
	}
	raw := b[sp.start:sp.end]
	if raw[0] == 'n' {
		err = nullInto(&v, a, nullFault(a, isTime, sp.start))
		return v, err
	}
	want := "a value of a declared type"
	switch a.Type {
	case TString:
		v.Str, want = stringOf(raw)
	case TInt, TDuration:
		var n int64
		n, want = intOf(raw)
		*v.intField(a.Type) = n
	case TFloat:
		v.F, want = floatOf(raw)
	case TBool:
		v.B, want = boolOf(raw)
	case TTimestamp:
		v.T, want = timestampOf(raw)
	case TObject:
		v.Obj, want = objectOf(raw)
	}
	err = kindInto(&v, a, sp.start, want, raw)
	return v, err
}
```

- **Helpers.** Inline cost in brackets; "not inlined" marks the two that are calls. Each `xxxOf` returns its type's zero
  and a `want` text on a fault, and the texts are today's, verbatim:
  - `nullInto(v *Value, a Attr, fault error) error` (14): sets `Kind` and `Null` when `fault` is nil.
  - `absentFault` and `nullFault(a Attr, isTime bool, at int) error` (71 each): nil when `a.Optional && !isTime`, else
    `missingErr`/`nullErr`, the cold `fmt.Sprintf` and `&DecodeError` moved verbatim.
  - `kindInto(v *Value, a Attr, at int, want string, raw []byte) error` (75): `typeErr` when `want != ""`, else
    `v.Kind = a.Type`.
  - `(*Value).intField(t Type) *int64` (12): `&v.I` for `TInt`, else `&v.D`.
  - `stringOf(raw) (string, string)` (71), `intOf (int64, string)` (74), `boolOf (bool, string)` (17),
    `objectOf (any, string)` (78).
  - `floatOf (float64, string)` (97, not inlined) and `timestampOf (int64, string)` (223, not inlined; it keeps the
    `buf [64]byte`).
- **Sizes:**
  - `value`: cyclomatic 10 (1 + 3 ifs + 6 cases), 33 lines.
  - Helpers: cyclomatic 1-5, 2-11 lines.
- **Generated code** (`<scratch>/value-p5.short`):
  - Frame 240 bytes (today 464), 1376 bytes of code.
  - String path: `CALL decodeString`, one `STP` into `v.Str`, then the shared tail: test `want`, store `Kind`, load the
    11 result registers, `RET`.
  - The only `FLDPQ` pair copies the read-only `Value{Kind: TBool}` template on the reserved path.
  - No `Value` temporary, no slot-to-slot copy. Present attributes add no call except float and timestamp, whose
    helpers return 2-3 words in registers.
- **The named result is load-bearing.** With `var v Value` and `return v, err` instead (`<scratch>/p5l`), every return
  copies `v` into `~r0` again: 6 `FLDPQ` (`<scratch>/value-p5l.short`, `decode.go:138`, `:143`, `:163`). The lane's
  comment on `value` must say so.
- **Measured:** 
  - **`BenchmarkDecoderValue`: +3.5 %, re-run +4.8 %** on a quiet machine:
    - 116.85 to 120.95 ns, p = 3.2e-05;
    - re-run 114.6 to 120.05 ns, p = 1.1e-05.
  - That is over decision 1's 2 % bar and just under rule 13a's 5 %. O1's own median is steady (120.95, 120.05);
    today's moved (116.85, 114.6).
  - `BenchmarkDecoderValueFaults`: -0.7 % (no cost).
  - Counters (process cycles, under load): +4.2 % and +6.4 % on `BenchmarkDecoderValue` (two sessions), +2.0 % on
    `BenchmarkReviewValue16`, **-0.3 % on `BenchmarkReviewDecode16`** (whole `Decode` of 16 attributes; the layout
    control moves the same benchmarks by +0.3 % and -3.3 %).
  - **Quiet `compare.py` on the realistic benchmarks:**
    - `BenchmarkReviewDecode16` **-1.7 %** (683.45 to 671.6 ns, p = 0.012): whole `Decode`, scanner included;
    - `BenchmarkReviewValue16` **+2.6 %** (224.3 to 230.15 ns, p = 0.0056).
  - **Where the 3.5-4.8 % comes from** (instructions +3.6 %, about 110 per op of 10 attributes):
    - one extra call each for float and timestamp (their readers are 97 and 223 > 80);
    - eight reloads after `decodeString`, `parseInt` and `cleanObject`. The shared tail may need `typeErr(a, at, want,
      raw)`, so `a`, `sp.start` and `raw` stay live across the worker call. The inliner merges each reader's two
      returns into one result, so the compiler cannot see that `want` is `""` on the success path
      (`<scratch>/value-p5.short`, 0x198-0x1b4).
  - **Variant P6** (`<scratch>/p6`) gives every case its own `kindInto` and `return`: 46 lines, instructions +2.5 %,
    **still +3.5 %** (114 to 118 ns, p = 1.1e-05). The merge happens inside the inlined reader, not in `value`'s tail,
    so P6 brings no gain and the shorter P5 form is preferred.
- **Pros:**
  - Meets cyclomatic ≤ 10 and ≤ 60 lines everywhere with no waiver.
  - Identical results (differential and suite).
  - No new allocation.
  - `value`'s frame halves.
  - Each reader is one small, testable job.
  - The `Value` is built in exactly one place.
  - The shape follows from the generated code, not from luck.
- **Cons:**
  - **+3.5 % and +4.8 % on `BenchmarkDecoderValue`**: over decision 1's 2 % bar, close to rule 13a's 5 %.
  - Two invariants a reader must keep: "a reader returns its type's zero on a fault" (so `v` stays `Value{}` with an
    error) and "the result stays named".
  - `intField` is a small pointer trick, a new unexported method on the API type `Value`; a package function
    `intField(v *Value, t Type) *int64` works equally.
  - Float and timestamp keep one extra (cheap) call.
  - The inline margins are thin (71-78 of 80). A toolchain that charges more would turn a reader into a call: a few
    percent, never a `Value` copy.
  - Thirteen small functions replace one.

### O2: inlinable slot fillers (`stringInto(&v, raw) (want string)`), each setting its own `Kind`

Prototype `<scratch>/p4/internal/event/decode.go`.

- **Same preamble and `typeFault` as O1.** Each filler writes `v.Kind` and its field through the pointer and returns
  `want`.
- **Cohesion is slightly better:** each type's whole accept is in one function.
- **But writing through a pointer costs inline budget:**
  - `boolInto` (24) and `stringInto` (76) inline;
  - `intInto` (96), `floatInto` (102), `objectInto` (83) and `timestampInto` (228) do not. So int, float, object and
    timestamp each keep a call, writing through the pointer (no `Value` copy).
  - The variants tried (`<scratch>/p4`, `zz_v.go` experiments: `setInt`, `intField`, early-return forms) stayed at
    83-95.
- **Sizes:** `value` cyclomatic 10, 33 lines; helpers 2-12 lines.
- **Measured:**
  - `BenchmarkDecoderValue` **+8.7 %** (113.85 to 123.7 ns, p = 1.1e-05); faults +0.6 %.
  - Counters: `BenchmarkDecoderValue` +10.2 % cycles, `BenchmarkReviewValue16` +5.1 %, `BenchmarkReviewDecode16`
    -1.3 %.
- **Pros:**
  - Same correctness and size as O1.
  - No `want` text traveling with a zero value.
- **Cons:**
  - Four of six types pay a call per attribute.
  - The `*Value` is visible in every filler.
  - Slower than O1 by construction, and it fails rule 13a (+8.7 %, at least 5 %).

### O3: destination slot, one non-inlined filler per type (the design's split, writing instead of returning)

Prototype `<scratch>/p2c/internal/event/decode.go`. (`<scratch>/p2` is the first form; `<scratch>/p2b` adds the field
stores.)

- **Each design E.2.4 helper keeps its body and its error construction verbatim.** The only change is that
  `return Value{...}, nil` becomes field stores into `*v`, for example
  `stringInto(v *Value, a Attr, raw []byte, at int) error`.
- **Absent and null use the inlinable `nullInto`.**
- **It is the closest to the reviewed design:** same helper list, same mutations; only the result travels by pointer.
- **Sizes:** `value` cyclomatic 10, 31 lines; helpers 2-12 lines.
- **Measured:**
  - P2, the first form: **+32.7 %**.
  - P2c, no full-struct rewrite and no null copy: `BenchmarkDecoderValue` **+9.0 %** (113.6 to 123.8 ns,
    p = 1.1e-05); faults -0.3 %.
  - P2c counters: `BenchmarkDecoderValue` +8.1 % cycles, `BenchmarkReviewValue16` +5.1 %, `BenchmarkReviewDecode16`
    +2.3 %.
- **Pros:**
  - The simplest mechanical change from the design.
  - High cohesion: accept and fault together per type.
- **Cons:**
  - One non-inlined call per present attribute.
  - It fails rule 13a (+9.0 %, at least 5 %), so it is not a real option on this hot path unless the owner accepts the
    time.

### O4 (status quo, for comparison): keep `value` whole with the design's PERMANENT waiver

- Design E.2.4's fallback: "If decision 1 says it costs: the PERMANENT waiver on `value` (no cheaper shape keeps
  cc ≤ 10)".
- It is the fastest by definition: no change.
- But the brief requires no waiver, and O1 shows the premise "no cheaper shape keeps cc ≤ 10" does not hold.
  - If O1 is chosen, that sentence needs an annotation, not a rewrite (ground rule 4).

### Weighed and rejected

- **Move only the cold paths (absent, null, error texts) out, keeping the accept arms inline.**
  - It keeps today's speed, but `value` stays at cyclomatic 24: 28 less the two `a.Optional && !isTime` tests, with
    the type conditions still there.
  - It cannot reach 10 without moving the accept checks, which is O1.
- **Numeric against non-numeric switch.** Two non-inlined halves that each return a `Value`: the split's call and copy
  again (section 1.3), whatever the grouping.
- **Inlinable helpers that return `Value` (P1) or take one (P3).** Measured +62 % and +53 %: the inlined temporaries
  are copied (section 1.3).
- **A handler table indexed by `Type`.** Excluded by design §2.1, and it would be an indirect call returning a `Value`.
- **Shrink `Value` to at most 4 fields so it fits in registers.** An API and ARCH §4 change
  (`TestDeclarationsMatchTheSurface` in `value_test.go` pins the 9 fields), with a large blast radius. Not for this
  lane; listed only because it is the root cause.
- **Fold the reserved check into the absent arm** (the scanner never marks the reserved slot seen). It would save one
  branch, but it changes the result for `(i == reserved, sp.seen)`. No caller produces that input, but the
  differential test does, and "identical on every input" forbids it.

## 3. Indicative measurements

**Method:**
- `gate/bench/compare.py` from lane E's worktree, run under the pinned Python 3.15 with `-I -B`.
- `--pairs 10`, `--before <scratch>/bin/2b86876.test` (today's code with the review's two benchmarks added),
  `--after <scratch>/bin/<variant>.test`, `--dir <scratch>/<variant>/internal/event`.
- **Two realistic benchmarks were added in every copy** (`<scratch>/zz_review_bench_test.go`):
  - `BenchmarkReviewDecode16`: `Decode` of a 16-attribute payload: 5 strings, 3 ints and durations, 2 floats,
    2 bools, the RFC 3339 time attribute, an object, an optional null, an optional absent;
  - `BenchmarkReviewValue16`: `value` alone over its spans.
- **Logs:** `<scratch>/logs/cmp-<run>-<variant>-<bench>.log`.

**Conditions: indicative only.**
- Another agent's gate and fuzz job ran during part of these runs; the load average reached about 49.
- The runs marked "disturbed" below were taken under that load, or while I compiled; their absolute times are
  inflated.
- The coordinator must re-measure the chosen option alone (decision 1).

**Wall clock, `compare.py`, 10 ABBA pairs.** Δ is the change in median against today; "costs" is `compare.py`'s
decision-1 verdict (p < 0.05 and more than 2 %).

| Run | Variant | Benchmark | Today ns | Variant ns | Δ median | p | allocs | Verdict |
|---|---|---|---|---|---|---|---|---|
| r1 | layout control | `BenchmarkDecoderValue` | 112 | 113.5 | +1.3% | 0.1179 | 3/3 | no cost |
| r1 | layout control | `BenchmarkDecoderValueFaults` | 2427 | 2419.5 | -0.3% | 0.4032 | 66/66 | no cost |
| r1 | layout control | `BenchmarkReviewDecode16` | 691.95 | 700.95 | +1.3% | 0.1903 | 7/7 | no cost |
| r1 | layout control | `BenchmarkReviewValue16` | 229.9 | 229.75 | -0.1% | 0.7394 | 7/7 | no cost |
| r1 | P1 (rejected) | `BenchmarkDecoderValue` | 116.4 | 189 | +62.4% | 1.083e-05 | 3/3 | costs |
| r1 | P1 (rejected) | `BenchmarkDecoderValueFaults` | 2514 | 2599 | +3.4% | 1.083e-05 | 66/66 | costs |
| r1 | P1 (rejected) | `BenchmarkReviewDecode16` | 706.6 | 844.35 | +19.5% | 1.083e-05 | 7/7 | costs |
| r1 | P1 (rejected) | `BenchmarkReviewValue16` | 236.95 | 356.05 | +50.3% | 1.083e-05 | 7/7 | costs |
| r1 | P3 (rejected) | `BenchmarkDecoderValue` | 115.35 | 176.5 | +53.0% | 1.083e-05 | 3/3 | costs |
| r1 | P3 (rejected) | `BenchmarkDecoderValueFaults` | 2483 | 2522.5 | +1.6% | 0.000433 | 66/66 | no cost |
| r1 | P3 (rejected) | `BenchmarkReviewDecode16` | 702.3 | 809.2 | +15.2% | 0.001505 | 7/7 | costs |
| r1 | P3 (rejected) | `BenchmarkReviewValue16` | 236.85 | 330.7 | +39.6% | 0.008931 | 7/7 | costs |
| r1 | P2 (O3, first form) | `BenchmarkDecoderValue` | 116.7 | 154.85 | +32.7% | 1.083e-05 | 3/3 | costs |
| r1 | P2 (O3, first form) | `BenchmarkDecoderValueFaults` | 2490 | 2481 | -0.4% | 0.1087 | 66/66 | no cost |
| r1 (disturbed) | P2 (O3, first form) | `BenchmarkReviewDecode16` | 913.35 | 1056 | +15.6% | 0.1971 | 7/7 | no cost |
| r1 (disturbed) | P2 (O3, first form) | `BenchmarkReviewValue16` | 302.4 | 507.8 | +67.9% | 0.0156 | 7/7 | costs |
| r4 | split in today's tree | `BenchmarkDecoderValue` | 113.85 | 158.5 | +39.2% | 1.083e-05 | 3/3 | costs |
| r4 | split in today's tree | `BenchmarkDecoderValueFaults` | 2438.5 | 2495.5 | +2.3% | 1.083e-05 | 66/66 | costs |
| r4 (run 1) | **O1 (P5)** | `BenchmarkDecoderValue` | 116.85 | 120.95 | +3.5% | 3.248e-05 | 3/3 | costs |
| r4 (run 1) | **O1 (P5)** | `BenchmarkDecoderValueFaults` | 2512 | 2493.5 | -0.7% | 0.7786 | 66/66 | no cost |
| r4 (re-run) | **O1 (P5)** | `BenchmarkDecoderValue` | 114.6 | 120.05 | +4.8% | 1.083e-05 | 3/3 | costs |
| r4 (re-run) | **O1 (P5)** | `BenchmarkDecoderValueFaults` | 2445 | 2452.5 | +0.3% | 0.6301 | 66/66 | no cost |
| r4 | **O1 (P5)** | `BenchmarkReviewDecode16` | 683.45 | 671.6 | -1.7% | 0.01219 | 7/7 | no cost |
| r4 | **O1 (P5)** | `BenchmarkReviewValue16` | 224.3 | 230.15 | +2.6% | 0.005586 | 7/7 | costs |
| r4 | O1 variant P6 | `BenchmarkDecoderValue` | 114 | 118 | +3.5% | 1.083e-05 | 3/3 | costs |
| r4 | O1 variant P6 | `BenchmarkDecoderValueFaults` | 2451 | 2441.5 | -0.4% | 0.4943 | 66/66 | no cost |
| r4 | O2 (P4) | `BenchmarkDecoderValue` | 113.85 | 123.7 | +8.7% | 1.083e-05 | 3/3 | costs |
| r4 | O2 (P4) | `BenchmarkDecoderValueFaults` | 2421.5 | 2435.5 | +0.6% | 0.118 | 66/66 | no cost |
| r4 | O3 (P2c) | `BenchmarkDecoderValue` | 113.6 | 123.8 | +9.0% | 1.083e-05 | 3/3 | costs |
| r4 | O3 (P2c) | `BenchmarkDecoderValueFaults` | 2440.5 | 2433.5 | -0.3% | 0.9278 | 66/66 | no cost |

**Notes on the runs:**
- The `r1` runs ran before the other job started, except `r1-p2 Review`. That one was disturbed by my compiles and the
  job's start: today's median was 913 ns against about 690 ns when quiet. Ignore it.
- The `r4` runs ran after the load ended (load average under 3).
- O1's first quiet run was logged under the same name as its re-run, which overwrote it. Its summary lines, captured
  before the overwrite, are in `<scratch>/logs/cmp-r4-p5-BenchmarkDecoderValue-run1.summary`; the re-run is
  `...-run2.log`.
- The split was re-measured inside today's tree (`<scratch>/split`: today's `decode.go` with `1e48b3b`'s `value` and
  helpers). It reproduces the coordinator's +37 %: +39.2 %.

**Process cycles and instructions, `<scratch>/tools/counters.py`.** Taken during the other job's load; they count only
this process, and the split's +37.9 % matches the quiet result. Change against today, median of 6 alternating rounds.

| Variant | `BenchmarkDecoderValue` cycles / instr | `BenchmarkReviewValue16` cycles / instr | `BenchmarkReviewDecode16` cycles / instr |
|---|---|---|---|
| layout control | +1.3 % / -0.2 % (2nd session -0.0 % / +0.4 %) | -3.3 % / -0.0 % | +0.3 % / -0.0 % |
| split (design E.2.4) | +37.9 % / +14.3 % | +26.2 % / +12.6 % | +8.6 % / +4.6 % |
| **O1 (P5)** | +4.2 % / +4.0 % (2nd session +6.4 % / +3.6 %) | +2.0 % / +3.0 % | **-0.3 % / +1.1 %** |
| O1 variant P6 | +4.3 % / +2.5 % | +2.3 % / +1.8 % | -3.5 % / +0.6 % |
| O2 (P4) | +10.2 % / +6.1 % | +5.1 % / +4.6 % | -1.3 % / +1.6 % |
| O3 (P2c) | +8.1 % / +5.1 % | +5.1 % / +6.1 % | +2.3 % / +1.7 % |

**Reading the tables:**
- On the 10-attribute leaf benchmark, O1 costs 3.5-4.8 %; O2 and O3 cost about 9 %.
- On a realistic 16-attribute `Decode`, O1 is level: -1.7 % wall clock, -0.3 % cycles, both within the placement noise
  the layout control shows.
- Allocations are equal everywhere (3 = 3, 66 = 66, 7 = 7).

## 4. Recommendation

**O1, provided the owner accepts its measured leaf cost. Otherwise O4.**

### Why O1

- **It is the healthiest shape found that meets the structural rules:**
  - cyclomatic ≤ 10 and ≤ 60 lines everywhere, with no waiver;
  - identical behaviour (differential test and suite);
  - no new allocation;
  - the `Value` built in exactly one place;
  - the shape follows from the mechanism in section 1, not from luck.
- **It is about eight times cheaper than the design's split.** `BenchmarkDecoderValue` +3.5 % and +4.8 % against
  the split's +37 %, and level on a realistic whole-payload `Decode`.

### What the owner must weigh (ground rule 4)

- O1's +3.5 % and +4.8 % on `BenchmarkDecoderValue` are over **decision 1's 2 % bar for a split**:
  - decisions.md, plan check of 2026-11-14 (line 1095);
  - kept "as it is" on 2026-11-18, decision 3 (line 1169).
- It is under ground rule 13a's 5 %.
- As the decisions stand, a split costing more than 2 % is restored whole with its waiver, which is O4.
- **Adopting O1 therefore needs the owner's explicit exception to decision 1 for this one function.** The case for it:
  - the realistic path is level: on `BenchmarkReviewDecode16`, quiet `compare.py` -1.7 %, counters -0.3 %;
  - the leaf difference is 4-5.5 ns per 10 attributes, and the 16-attribute `Decode`, scanner included, measured no
    slower;
  - the alternative keeps a cyclomatic-28 function under a PERMANENT waiver.
- **The case against:** decision 3 was taken today with the placement evidence in hand. A 3.5-4.8 % leaf result is
  above the +1.3 % layout control measured here, and close to the 3-4 % placement shift the walkers review measured on
  a ~92 ns benchmark.
- **This review does not decide it. I recommend O1 with that exception if the coordinator's quiet re-measure stays
  under 5 %, and O4 otherwise.**

### How the lane should build O1 (build-ready)

1. **Nets commit, green on `2b86876`.** Add the differential test as `internal/event/value_diff_test.go`, from
   `<scratch>/zz_value_diff_test.go`: `valueToday` is today's `value` verbatim, the frozen oracle. It runs 13 608
   exhaustive cases and 300 000 random ones.
2. **The split commit.** Replace `decode.go` from `// value is attribute i's value` up to (not including)
   `func timeNote(` with `<scratch>/p5-value.go.txt`.
   - Give a doc comment to each helper that lacks one (`intOf`, `floatOf`, `boolOf`, `timestampOf`, `objectOf`,
     `missingErr`, `nullErr`).
   - Write `intField` as a package function `intField(v *Value, t Type) *int64` if a method on `Value` is unwanted.
3. **Decision 1 by the coordinator, alone:** `BenchmarkDecoderValue` and `BenchmarkDecoderValueFaults`, 10 pairs,
   re-run once. Add the path benchmark that contains `Decode` (the 5 % path bar, decisions.md line 1130).
4. **Mutations, each shown red by the nets:**
   - `stringOf` dropping the `'"'` test;
   - `intField` swapping `I`/`D`;
   - `floatOf` ignoring `err` (red by `1e999`);
   - `boolOf` swapping `t`/`f`;
   - `timestampOf` skipping the unescape (red by `"2026-01-01T10:00:00\u005A"`);
   - `objectOf` refusing `[`;
   - `absentFault`/`nullFault` dropping `!isTime`;
   - `kindInto` not setting `Kind`.
5. **Comments that carry the speed:**
   - on `value`: "the result is named and filled in place; no helper on this path takes or returns a `Value`
     (`Value` exceeds the compiler's 4-field SSA limit, so each one is a stack copy: architecture review, section 1)";
   - on the readers: "returns its type's zero on a fault".
6. **Design note §6 E.2.4:** annotate it, without rewriting (ground rule 4): its premise "no cheaper shape keeps
   cc ≤ 10" no longer holds.

### Pros and cons

- **Pros:**
  - The size rule is met without a waiver.
  - Every function is small and single-purpose.
  - The `Value` is built in one place.
  - Absent and null lose the call level the design gave them.
  - The differential net stays as a permanent guard.
- **Cons:**
  - +3.5 % and +4.8 % on the leaf benchmark: it needs the decision-1 exception, and it sits close to rule 13a's 5 %.
  - Two invariants (zero on fault, named result) carry speed and correctness; only comments and nets protect them.
  - Thin inline margins (71-78 of 80).
  - Float and timestamp keep one call each.

### Pre-mortem (the-fool, mode "Find the failure modes"; the brief set the mode)

- **Steelmanned thesis:** O1 removes `value`'s 28-branch body without the per-attribute `Value` copy that sank every
  earlier split. A differential test that catches planted faults proves it identical. It costs a few percent on a leaf
  micro-benchmark and nothing measurable on a real decode.
- **Failure 1, most likely: a later edit brings a `Value` back through a helper.** Six months on, someone "tidies"
  `kindInto` into `(Value, error)`, or turns the named result into a local (`<scratch>/p5l` shows the copy coming
  back), and the benchmark loses 5-60 %.
  - Early warning: `BenchmarkDecoderValue` in decision 1's gate.
  - Mitigation: the comment above, and the benchmark staying in the gate. An inline check on the six inlined helpers
    (`-m=2` in the escapes or lint gate) is later severity.
- **Failure 2, correctness: a reader returns a non-zero value with a fault.** Example: `intOf` returning `parseInt`'s
  partial `n`. On a fault `value` then returns a non-zero `Value` with its error.
  - `Decode` drops values on error, so production would not see it, but direct callers and tests would.
  - Early warning: the differential test, which compares the `Value` with `DeepEqual`.
  - Mitigation: keep that test permanently.
- **Failure 3, toolchain: an upgrade changes inline costs.** `objectOf` (78) or `kindInto` (75) crosses 80 and becomes
  a call: a few percent, unseen until measured.
  - Mitigation: a toolchain bump re-runs the benchmarks. Record the margins in the E.2.4 annotation.
- **Failure 4, measurement: the verdict is placement, either way.** The layout control here moved the benchmark by
  +1.3 %, and the walkers review saw 3-4 %. O1's +3.5 % and +4.8 % (P6 also +3.5 %) could be partly placement, and the
  coordinator's re-run could land on either side of 5 %.
  - Mitigation: the coordinator's alone re-run decides; claim "about +4-5 % on the leaf", never "level".
- **Failure 5, readability: indirection.** A reader of `value` must open `kindInto` to see where a type fault is
  raised, and the `want` texts sit in six readers.
  - Mitigation: the comments. Each text stays next to the check that produces it, as today.
- **Inversion check (what would guarantee failure):**
  - passing a `Value` by value through any helper on the accept path;
  - adding a call on the string, int, bool, object, absent or null path;
  - dropping the differential net.
  - O1 avoids all three. The residual risk is drift, guarded by the benchmark and the net.
- **Synthesis:** adopt O1 with the comments and the permanent net, if the owner accepts about 4-5 % on the leaf
  benchmark. If not, or if the quiet re-run lands at 5 % or more, keep O4 with its PERMANENT waiver citing this review.
  O2 and O3 are slower by construction and should not be built.
- **Next step:** if the owner wants it, a second pass in the "Test the evidence" mode on the coordinator's quiet
  numbers.

## 5. For the owner, in plain words

- **The problem.** The code that reads each field of an incoming event is too big for our size rule. The planned way
  of cutting it into nine parts made it about 37 % slower, so under your 5 % rule it was put back whole.
- **Why the plan was slow.** One field's value is a fairly large bundle of 11 pieces. Go cannot keep a bundle that big
  in the processor's fast registers inside a function. So whenever one part hands the bundle to another, the computer
  writes it out to memory and copies it, and the copy then has to wait. The planned cut did that once for every field.
- **The options, best first:**
  1. **Small helpers return only the plain value** (a number, a text, true or false), and the main part writes the
     bundle once, in place. Same results, meets the size rule with no permanent exception. In my tests it is about
     4-5 % slower on the narrow field-reading benchmark and level on reading a whole realistic event.
  2. **Small helpers fill the bundle directly.** Also correct, but four of the six field kinds still pay an extra step:
     about 9 % slower.
  3. **The planned parts, changed to fill the bundle instead of handing it back.** Closest to the plan, but 9-33 %
     slower.
  4. **Keep the code whole, with a permanent written exception.** No slowdown and no work, but the size rule stays
     waived for this function.
- **My recommendation.** Option 1, plus a permanent test that checks the new code against an exact copy of today's
  code on over 300 000 inputs. But option 1's 4-5 % is above the 2 % limit you kept for each split today, and close
  to your 5 % rule. So choosing it means making an exception for this one function. If you would rather not,
  option 4 is the right fallback.
- **The risks:**
  - The speed relies on two habits a future programmer could break without noticing: "never hand the bundle between
    parts" and "a helper returns nothing on an error". Comments, the comparison test and the speed check in the gate
    guard them.
  - Part of my timing was taken while another job loaded the machine. The coordinator must time option 1 again on a
    quiet machine before you decide.
