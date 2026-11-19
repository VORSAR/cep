# Lane P under ground rule 13a: AppendValue, (*Recorder).Table and the replay path

Reviewer: architect-reviewer, with the-fool (pre-mortem mode, chosen by me: there is no user to ask in a sub-agent
run). Scratch only: nothing changed in any repository or worktree. Toolchain go1.27.4 darwin/arm64 (Apple M4 Pro),
golangci-lint v2.15.0. GitNexus: the index is of main, not of lane P's worktree, so it lacks the lane's helpers; I
used it only for `impact AppendValue upstream`: 4 direct callers, 8 processes
(runtime recorder run, sink adapter, trace); every option keeps every exported signature, so no caller changes.

**How I measured, and why not compare.py.** The machine was on battery and heavily loaded the whole time (load
average 24-37 on 14 cores, the coordinator's gate runs). My first compare.py run (dd6bb8d against c61d7ef) read
medians of 4,600 ns for a 480 ns benchmark with samples from 980 to 10,950 ns (`logs/R1-dd6-vs-c61.log`): unusable, so
I stopped compare.py. Instead every number below comes from the CPU's own counters (`/usr/bin/time -l`: instructions
retired and cycles elapsed of the process), each binary run at two fixed iteration counts so that start-up cancels:
per op = (count(N2) - count(N1)) / (N2 - N1), binaries interleaved over 5-7 rounds (`ctrbench.py`). Instructions per
op do not depend on load; cycles per op do not depend on clock speed or time spent descheduled, but still move with
code placement and cache sharing, so I report the minimum (and the median) and a placement control beside every
area. **The AppendValue and Table results were run twice (logs `ctr-*`, `ctr2-*`, `ctr3-*`) and agree, except V3
and T4, run once; the replay comparison ran once (5 rounds, three kinds, consistent across the kinds).** These are
indicative; the coordinator's
compare.py re-run on mains power, alone, decides.

## 1. Why each is slower (from the generated code)

**The common cause.** `event.Value` (9 fields, 88 bytes) and `plan.Table` (3 fields, 64 bytes) are larger than what
the Go compiler keeps in registers (at most 4 fields / 4 words), so a variable of either type always lives in memory.
Passing one **by value to a function that is not inlined** costs, per call: the caller loads every word into argument
registers, the callee stores every word back to its spill slots at entry, plus the call, the stack-growth check and a
frame push and pop. The whole functions never pay it. In both splits the design had asked for the one field the
helper needs (P.2: `appendFloat(dst, f float64)`, `appendTimestamp(dst, ms int64)`, `appendObject(dst, obj any)`,
`tableLine(name, opName string, ...)`); the lane passed the whole struct to keep `v.F`, `v.Obj.(string)`, `t.Name`
word for word. That deviation is the regression.

**AppendValue at 880541c** (`asm/880541c.AppendValue.t`, `asm/880541c.appendObject.t`; counters `ctr*-av-object`):
- `v` arrives in 11 registers (10 integer R3-R12 + F0; arm64 has 16 integer argument registers, so it fits), and
  both versions store all of them at entry (8 store instructions to `v+24(FP)`..`v+104(FP)`).
- The split's object case then reloads 7 registers of `v` with 6 loads (`LDP v+32`, `MOVD v+48`, `FMOVD v+56`,
  `MOVBU v+64`, `MOVD v+72`, `MOVD v+88`) and calls `appendObject` (cost 106, not inlined, 112-byte frame), which
  runs its own stack check and frame push and **stores all 11 registers of `v` again** (8 stores) before its
  2-instruction type check, then its own epilogue.
- Measured: **+26 instructions per value (93.1 -> 119.1, +27.9 %), +54 % cycles** (14.9 -> 22.9). The coordinator's
  +45 % is this.
- Float is the same mechanism (`appendFloat`, cost 114) on a 24 ns path: +25 instructions (+3.7 %), +5-6 % cycles.
- `appendTimestamp` (77) inlines: timestamp identical (2,166 instructions/op in every build).

**(*Recorder).Table at dd6bb8d** (`asm/dd6bb8d.s`, `(*Recorder).tableLine`; counters `ctr*-tb-*`):
- `tableLine(t Table, opName string, cols []event.Attr, row []event.Value)` needs 17 integer registers; arm64 has 16,
  so receiver R0, `t` R1-R8, `opName` R9-R10, `cols` R11-R13 and **`row` goes on the stack** (stored by the caller at
  `8(RSP)..24(RSP)`).
- At the call Table reloads its 64-byte copy of `t` (4 `LDP t-168(SP)..t-120(SP)`) and stores `row`; at entry
  `tableLine` (cost 579, 304-byte frame) stores `t` again (4 `STP .. t+32(FP)..t+80(FP)`), `opName` and `cols`, and
  reads `row+8(FP)` back for the width check.
- Measured: **+36 instructions per row (+4.5 % delete, +4.2 % put), cycles +4.7 to +8.8 % delete, +3.3 to +4.5 %
  put** (two runs). Same order as the coordinator's +6.9/+7.2 % and +5.5/+4.6 %.

**readLine at c61d7ef:** one call more per line (`rawLine`, cost 161, returning 5 registers; `trimLineEnd` inlines).
Measured: instructions per line level (-0.4 % to +0.6 % across three kinds, within the run-time's own allocation
noise), cycles +3.6 to +6.2 %. Same instructions and more cycles is the signature of placement; section 3 proves it.

## 2. Options, ranked by code health (accuracy, simplicity, the repo's rules) first

All options below:
- **byte-identical output and identical errors**: differential tests against a verbatim copy of the whole function
  (`zz_avdiff_internal_test.go`: 2,000,000 generated values against AppendValue verbatim at 546e9bc - every kind 0-9
  and 99-198, Null on and off, NaN payloads, +-Inf, -0, subnormals, MaxFloat64, random bits, timestamps at min-1/min/
  max/max+1 and MinInt64/MaxInt64, objects that are strings, nil, int, []byte, struct, pointer, bool; compared bytes,
  error text and the identity of errNonFinite/errTimeRange. `zz_tablediff_internal_test.go`: 300,000 generated calls
  against Table verbatim at 880541c - names needing escapes, duplicate attribute and key names, index -1..len, put
  and delete, 0-3 rows of 0-5 values, NaN, a writer failing on write 1-3; compared every byte written, the write
  count, error text, `errors.Is` of errNonFinite and the writer's error, and `r.buf` afterwards). Both tests catch a
  planted mutation (`<=` to `<` in the float test; `i > 0` to `i > 1` in the row loop: `mut-av2`, `mut-t3`);
- the whole playback suite passes, including every exact `AllocsPerRun` pin (`TestAppendValueAllocatesNothing`,
  `TestRecorderTableAllocatesNothingPerRow`, `TestAppendEventIntoARoomyBufferAllocatesNothing`, ... all PASS);
- no heap allocation added; in every Table option `recorder.go:161:19: append does not escape` (E4 kept: `cols` built
  in Table, passed down, never returned);
- golangci-lint (pinned, repo config): **no finding in encode.go's AppendValue group or in recorder.go** for any
  option (the only findings left are functions the base revisions had not split yet: appendString, appendEvent, the
  record decoders, Step, runBlock, diverge, sameValue). Every function cc <= 10 and <= 60 lines, no waiver.

### AppendValue (base: 546e9bc, the whole function)

| Rank | Option | Inlining (-m=2) | cc | Object: instr / cycles (min, two runs) | Float: instr / cycles | Timestamp |
|---|---|---|---|---|---|---|
| ref | whole (546e9bc) | - | 15 | 93.1 / 14.9-15.4 | 682.6 / 99.6 | 2,166 / 318.7 |
| ref | lane split (av0) | appendObject 106, appendFloat 114: no | 10 | +27.9 % / **+49 %, +54 %** | +3.7 % / +5.1 %, +6.2 % | level |
| **1** | **V1 (av6)**: the design's field-typed helpers `appendFloat(dst, f float64)`, `appendTimestamp(dst, ms int64)`, `appendObject(dst, obj any)`; appendObject's refusal built out of line in `objectError(obj any) error`, so appendObject inlines | appendObject 78 inlines, appendTimestamp 74 inlines, appendFloat 111 does not | AppendValue 10, helpers 2-3, objectError 1 | +7.5 % / av6 -3.4 % min, +2.7 % median; same code as av2/av5: +2.7 %, +3.6 %, +4.0 % | +1.5 % / av6 +3.3 %; av2/av5 +2.8 to +3.3 % | level |
| 2 | V2 (av3): the design's signatures exactly as written, the object error built in place | appendObject 104, appendFloat 111: no | as V1 | +15 % / **+17 %, +27 %** | +1.5 % / +1.9 %, +3.2 % | level |
| 3 | V3 (av4): helpers take `*event.Value` (bodies word for word, `&v` passed) | appendObject 106, appendFloat 114: no | as V1 | +12.5 % / +9 % | +1.6 % / +3.5 % | level |
| ctl | placement: whole + a 176-byte unused function before AppendValue (avpad) | - | - | 0.0 % / +4.6 %, -1.4 % | 0.0 % / +0.3 %, +0.2 % | level |

Notes:
- V1's object path compiles to the whole function's type check, length check and `memmove` instruction for
  instruction (`asm/av2.s`); the 7 extra instructions are the inlined call's results passed through a join block to
  the return (about 0.4 cycles, ~0.1 ns). Its float path is the whole function's `FABSD`/`FCMPD` test (the compiler
  folds `IsNaN || IsInf` to exactly that) plus one call whose arguments are `dst` (3 registers) and `f` (F0): +10
  instructions, ~0.7 ns on 24 ns.
- av2 and av5 are V1 with a `finite(f)` predicate and with appendObject's success test first; `finite` buys nothing
  (appendFloat does not inline either way, the machine code is the same), so V1 as recommended (av6) keeps
  `math.IsNaN(f) || math.IsInf(f, 0)` word for word. av6 passes the differential test, the suite, the pins and lint.
- Dropped: keeping object and float inline in AppendValue cannot reach cc 10 (null guard 2 + seven cases 7 + 1 = 10
  already); merging TInt and TDuration is not identical for a TInt value with D set; a table by kind is forbidden on
  the hot path; moving the null guard out passes `v` by value again.

### (*Recorder).Table (base: 880541c, the whole function)

| Rank | Option | cc | Delete: instr / cycles (min, two runs) | Put: instr / cycles |
|---|---|---|---|---|
| ref | whole (880541c) | 13 | 792.8 / 109.9-111.1 | 824.7 / 132.0-133.2 |
| ref | lane split (dd6bb8d) | 9 + 6 | +4.5 % / **+8.8 %, +4.7 %** | +4.2 % / +4.5 %, +3.3 % |
| **1** | **T1 (t3)**: the row loop moves whole into `(r *Recorder) tableRows(name, opName string, cols []event.Attr, rows [][]event.Value) error`, called once per Table call (`return r.tableRows(t.Name, opName, cols, rows)`); Table keeps the range check and builds the delete's key columns (E4). Plus: the op is written as `{"op":"` + opName + `","row":{` with `append` instead of `appendString(opName)`: opName is only ever the literal "put" or "delete", which need no escaping, so the bytes are identical and one non-inlined `appendString` call per row goes | Table 7, tableRows 7 | **-13.5 % / -11.6 %, -10.7 %** | **-7.5 % / -6.7 %, -8.7 %** |
| 2 | T2 (t2): T1 without the op change (every moved statement word for word) | 7 + 7 | +4.0 % / +2.5 %, +2.9 % | +3.4 % / +2.6 %, +1.7 % |
| 3 | T3 (t1): the design's signature `tableLine(name, opName string, cols, row)`, one call per row, all in registers | 9 + 6 | +3.9 % / +3.8 %, +4.6 % | +3.6 % / +3.4 %, +1.6 % |
| 4 | T4 (t4): `tableLine(t *Table, ...)`, `t := &r.app.Tables[table]` | 9 + 6 | +4.2 % / +4.9 % | +3.2 % / +4.7 % |
| ctl | placement: whole + a 176-byte unused function before Table (tpad) | - | 0.0 % / +0.7 %, +0.1 % | 0.0 % / -0.9 %, -1.2 % |

Notes:
- With one row per call (the benchmark), T2 and T3 make the same number of calls; T2 pays once per Table call
  however many rows, T3 once per row. Their residual +3-4 % instructions is the call itself (prologue, frame, the
  callee saving what it needs across its own calls).
- T1's op change is a small algorithmic improvement the 13a review is meant to find; it is the only statement in
  any option that is not a move. A variant that avoids relying on "opName needs no escaping" passes a constant
  prefix (`{"op":"put","row":{` / `{"op":"delete","row":{`) chosen next to opName in Table (5 parameters, the limit);
  I prefer the one-line comment.
- T4 reads `t.Name` through a pointer into `app.Tables`; a writer that changed `app.Tables` during the call would show
  (no realistic input does) - another reason it ranks last.
- Dropped: keeping the row body inline and moving only the width check and the error formatting out cannot reach
  cc 10 (Table's fixed part is 8, the row body 5); moving the key-column builder out means returning `cols` (E4
  broken: a heap allocation per delete); an inlined helper returning `cols` keeps 0 allocations only while the
  inliner's budget allows.

## 3. The replay-path question: placement, not code

Placement control `padB`: dd6bb8d with one unused 640-byte function placed before `Next` in reader.go (kept by a
test-only reference). It puts `Next` and `readLine` at **exactly** c61d7ef's addresses (0x1001bff90, 0x1001c03b0, from
`go tool nm`; dd6bb8d has them 848 bytes earlier) and every record decoder 848 bytes later (c61d7ef: 944). Its code
is dd6bb8d's, byte for byte.

| ReplayLine | dd6bb8d | c61d7ef | **padB (dd6bb8d's code, shifted)** | 2bed980 (lane end) | d119e9b (nets) |
|---|---|---|---|---|---|
| arrival_event cycles (min) | 1,844.9 | +6.2 % | **+5.8 %** | +1.3 % | +0.7 % |
| table_put | 2,036.8 | +5.2 % | **+6.3 %** | +0.7 % | +0.3 % |
| table_delete | 1,683.8 | +3.6 % (median +5.4 %) | **+6.6 %** (median +6.1 %) | +1.6 % | +1.6 % |
| instructions per line, against dd6bb8d | - | -0.4 % to +0.6 % | -0.4 % to +0.4 % | +1.0 % to +2.1 % | +0.2 % to +0.4 % |

Directly against each other (minimum cycles, same rounds): 2bed980 takes 4.6 %, 4.3 % and 1.9 % fewer cycles than
c61d7ef (arrival_event, table_put, table_delete), d119e9b 5.1 %, 4.6 % and 1.9 % fewer. Unchanged code moved to c61d7ef's addresses is as slow as c61d7ef; c61d7ef executes the same instructions per line as
dd6bb8d; the lane's end (2bed980) is back within 0.7-1.6 % of dd6bb8d. **c61d7ef's ReplayLine cost is code placement.
`readLine`'s split should stand**, with no change.

## 4. Indicative measurements

All in sections 1-3 (two runs each; logs `ctr-*.log`, `ctr2-*.log`, and `ctr3-*.log` for av6 itself). `BenchmarkAppendEvent` (string, int and timestamp attributes) is level for the lane, V1 and the control (3,448-3,451 instructions, cycles -1.3 % to +0.5 %). For the coordinator's re-measure with
compare.py, alone on mains power, the binaries are built in `bin/` (V1 = `av6`, T1 = `t3`, controls `avpad`, `tpad`,
`padB`); the brief's commands apply with `--dir` the matching `rev-*/internal/playback`.

Expectations to check there: V1 object about +3 % and float about +3 % against the whole function, under 13a's 5 %
but possibly over compare.py's 2 % bar; T1 clearly faster than the whole function (-7 to -12 %); the replay
placement control reproducing c61d7ef.

## 5. Recommendations, pros and cons, pre-mortem

**AppendValue: V1** (field-typed helpers as the design asked, the object refusal out of line so the hot object helper
inlines; prototype `proto-av6`).
- Pros: removes the regression (object +49-54 % -> about +3 %, float +5-6 % -> about +3 %, timestamp level); each
  helper takes exactly the data it needs and no 88-byte value crosses a call; cc 10 without a trick; only one new
  4-line function (`objectError`).
- Cons: not strictly level - float keeps one small call and object 7 instructions, about +3 % each (the floor: cc 10
  forces every refusal out of AppendValue's body, and appendFloat cannot fit the inliner's budget of 80);
  appendObject inlines at cost 78 of 80, so a future toolchain could tip it out, and it would then fall back to V2's
  +17-27 %, still well below the lane's +50 %.
- If the owner wants only moves word for word: none of the move-only forms comes near the whole function on object
  (V2 +17-27 %, V3 +9 %); V1 changes nothing but `v.F`/`v.T`/`v.Obj` to plain parameters and one error call.

**(*Recorder).Table: T1** (`proto-t3`).
- Pros: faster than the whole function (-7 to -12 % cycles, -7.5 to -13.5 % instructions); one call per Table call,
  not per row; nothing through memory; E4 untouched; both functions cc 7.
- Cons: one statement is not a move (the op written without an escape pass; correct because opName is one of two
  literals, said in a comment, covered by the exact-line nets and the differential test). If the owner wants moves
  only: T2 (+2-4 %).

**The readLine split: keep it** (section 3).

**The-fool, steelman first.** The lane's choice was defensible: moving statements word for word makes every move
checkable by a mechanical diff and keeps the mutation audit simple. The steelman fails on this evidence: the cost of
"word for word" is a 88- or 64-byte copy per value or row, and the change V1/T2 need (`v.F` -> `f`, `t.Name` ->
`name`) is mechanical too and proved identical by the differential tests.

**Pre-mortem (three weeks on, this went wrong; most likely first):**
1. *V1 reads "costs" on the coordinator's run.* Object and float about +3 %, over compare.py's 2 % bar though under
   13a's 5 %. Early sign: compare.py's `/object` and `/float` lines. Mitigation: read them beside `avpad` (placement
   moved object cycles by up to 4.6 % in one of my runs); the residual is the cc-forced floor - the alternatives are
   all slower; the owner decides whether ~3 % on two kinds is accepted. (`BenchmarkAppendEvent` is level for every
   option, but its event holds no object or float, so it says nothing about them.)
2. *appendObject stops inlining after a toolchain bump* (cost 78/80). Early sign: the inline record in the lane's
   commit / the escape-and-inline diff. Mitigation: record the cost in the commit as the lane already does; the
   fall-back is V2's cost, not the lane's.
3. *T1's op shortcut turns wrong:* someone adds an op whose name needs escaping. Mitigation: the exact-line nets
   fail at once for any new op; the comment names the assumption.
4. *The replay verdict is wrong.* Unlikely: the control reproduces c61d7ef on all three kinds with the same
   instruction count. Second-order risk is the opposite: future 2-6 % ReplayLine readings get blamed on splits. Early
   sign: instructions level, cycles up. Mitigation: read instructions per op beside cycles, as here.
5. *These counters mislead.* Cycles under heavy load include cache sharing with other jobs. Mitigation: instructions
   per op (load-independent) tell the same story in every area; the coordinator's quiet compare.py run is the
   decision.

Inversion check: the way to guarantee failure here would be to accept a "level" claim from reading the machine code
alone; each option was measured twice, beside a placement control.

## 6. For the owner, in plain words

- **The problem.** When the lane cut two long functions into smaller pieces, it handed each piece a whole large
  bundle of data instead of the one item the piece needs. Copying that bundle in and out on every value or every
  table row made object values about 50 % slower to write, and table rows 3-9 % slower. The design had asked for the
  single item; the lane passed the bundle to keep the old wording.
- **Values (recommended option 1):** give each piece only its item, and keep the object piece small enough that the
  compiler folds it back in. Object values go from +50 % to about +3 %, decimal numbers from +6 % to about +3 %,
  dates stay level. The other options (the design's pieces exactly as written; a pointer to the bundle) stay 9-27 %
  slower on objects.
- **Table rows (recommended option 1):** move the whole row loop into one piece called once per write, and stop
  re-checking the fixed words "put"/"delete" for special characters on every row. It comes out 7-12 % *faster* than
  before the split. Without that small extra change it is 2-4 % slower; the per-row options are 2-5 % slower.
- **The replay slowdown** was an accident of where code sits in memory: the old code, only shifted to the same place,
  is just as slow, and it does the same work. Keep that split as it is.
- **Risks.** My timings ran on a busy laptop on battery, so I measured work done (instructions) and processor cycles
  rather than clock time; one quiet run on mains power should confirm before you decide. The recommended value option
  still costs about 3 % on two kinds of value - the smallest any rule-abiding split reached.
