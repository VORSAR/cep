# Architecture review under ground rule 13a: the object walkers and the scanner's `object` / `str`

- **Reviewer:** architect-reviewer agent, 2026-11-18, one 90-minute box; recommendations only - the owner decides.
- **Code read:** lane E's worktree `.../s1-1c-6/wt/e`, head `94c5972` (`object.go` there is exactly `c54fe6c`'s);
  generated code compared at `388a9f8` / `53fb5f0` (walkers) and `f9d2817` / `bedad36` / `1f09fe4` (scanner).
  Nothing in any repository or worktree was changed; every prototype lives in the scratch directory.
- **Toolchain:** go1.27.4 darwin/arm64 (the pinned one, `GOTOOLCHAIN=local`), the machine the coordinator measures on.
- **Status of the numbers:** indicative (other agents may have been running); the coordinator re-measures the chosen
  option alone. A layout control is included so the spread caused by code placement alone is visible.
- **Rules applied:** ground rule 13a (owner, 2026-11-18, `decisions.md` "a refactor's large slowdown triggers an
  architecture review"), decision 3 naming this area; size limits from `.golangci.toml` (gocyclo reports > 10, funlen
  60 lines, lll 120); compare.py's "costs" rule (p < 0.05 and median > 1.02x).

## Summary for the owner (plain language)

**The problem.** Two groups of small, busy functions in the event reader had to be cut into smaller pieces to meet
the size rule. Cutting them made them slower (about 10-24 % for the object cleaner, about 8 % on the scanner's
path), so under rule 13a they were put back whole and wait on this review.

**Why the cuts were slower.** Not because they did more work. This computer runs straight-line code fast and loses
a little time at every jump. In the object cleaner, the cut made the compiler lay the code out so that every
ordinary character took four extra jumps. In the scanner, each cut added a small step per object member, and on
these tiny test inputs merely moving unchanged code to another address changes the result by about 3-4 % (we
measured that). The scanner also reads every character the slow way: it re-reads its place from memory each time.

**The options, best first.**
- Object cleaner: (1) one shared "read until something must change" helper used by all three functions, with two
  small lookup tables; (2) only a lookup table for spaces; (3) the design's spare cut plus the table; (4) another
  way of writing the space test (rejected: slower).
- Scanner: (1) and (2) both give `str` a fast loop that keeps its place in a register (escapes handled inline,
  only the rare `\u` in a helper), and differ in how `object` is cut - a shared "name and colon" reader for `object`
  and `top` (cleaner) or a cut at its comma test (keeps today's calls); (3) also a table for skipping spaces (much
  faster on long payloads, slower on the tiny test inputs).

**Recommendation.**
- Object cleaner: option 1, the shared helper (prototype P7). The cleaning rules then exist once instead of three
  times, every function is far under the limit, and it was 9-39 % faster in these runs.
- Scanner: option 1, the shared "name and colon" reader plus the fast `str` (prototype S11). It removes one of
  two copies of the same rule, nothing sits at the limit, nothing measured slower, and realistic payloads were about
  a third faster. Fallback: option 2 with the table (S10), also slower nowhere.
- A question only you can answer: the tiny scanner benchmark moves 3-4 % just from code placement, more than the 2 %
  "costs" bar. Should such small readings be judged against a placement control?

**Risks.** The numbers come from a busy machine and are indicative; the coordinator re-measures alone. The tiny
scanner test is so short that code placement alone moves it by 3-4 %, above the 2 % "costs" bar. The
shared helper reverses one line of the design ("no shared walker"), which needs your agreement. A toolchain
upgrade could lay the code out differently, as happened here; the benchmarks would show it. Every option was
checked equal to today's code on millions of generated inputs, broken ones included.

## One idea that serves both areas

Both slowdowns come from the same mechanism, and so do both fixes:

- **The cost of a byte loop on this machine is its taken branches per byte, and memory round trips of its index.**
  The `isSpace` split changed no instruction count, only which side of four compares falls through (four more taken
  branches per ordinary byte). The scanner reads every string and whitespace byte through `s.i` and `s.b` in memory.
- **The fix in both is a loop over "plain" bytes that keeps its index in a register and stops only at a rare byte**:
  the walkers' `cleanRun` (with a 256-entry `strPlain` table) and the scanner's `str` with locals (with a
  `strByte` table in S10 / S11). Both make their functions smaller (cyclomatic 4-9) *and* faster.
- **Shared piece:** `jsonSpace [256]bool` (space, tab, line feed, carriage return) is the one table both files can
  use - the walkers use it now in the recommended form. Using it in the scanner's inlined `ws` was measured (S4):
  much faster on pretty-printed payloads, but slower on the tiny compact benchmark inputs in the form tried, so it is
  not part of the recommendation. A combined class table (space / plain-in-string / plain-in-scanned-string) would
  serve both files but the two "plain" sets differ (the walkers keep control bytes, the scanner rejects them and
  accepts non-ASCII), so one small table per purpose is simpler than one table with bit masks.

## Area 1: `cleanObject`, `CleanLen`, `compactAndValid`

### A1.1 Why the `isSpace` form is slower although it inlines

Generated code of `CleanLen` at `388a9f8` and at `53fb5f0` (`go build -gcflags=-S`, go1.27.4, darwin/arm64, the
measuring machine's architecture; listings `cl-a.s`, `cl-b.s` in the scratch directory):

- Both are 448 bytes with the same instructions apart from branch sense and order: same registers, the same one
  bounds check (at `raw[i]`), the same loop shape, no jump table either side, `decodeRuneSlow` called the same way.
  **The only difference is which side of the four whitespace compares is the fall-through.**
- `388a9f8`, `c != ' ' && c != '\t' && c != '\n' && c != '\r'`: `CMPW $32; BEQ ws; CMPW $9; BEQ ws; CMPW $10; BEQ ws;
  CMPW $13; BEQ ws`, then the count. For an ordinary byte outside a string **none of the four branches is taken**; the
  byte costs three taken branches in all (into the out-of-string arm, to the shared `i++`, back to the loop head).
- `53fb5f0`, `!isSpace(c)` with `isSpace` inlined: the inliner's `||` chain puts each "is whitespace" block
  (`MOVD ZR, R4; JMP`) as the fall-through, so the code is `CMPW $32; BNE next; <ws block>; CMPW $9; BNE next; ...`.
  For an ordinary byte **all four branches are taken**: six taken branches per byte instead of three. `cleanObject`
  shows the same inversion (`co-388a9f8.s`, `co-53fb5f0.s`, offsets 0x388-0x3c0).
- `compactAndValid` did not slow down because there the call is `if isSpace(c) { return false }`: the whitespace side
  is the early exit, laid out away from the loop, and its code at `53fb5f0` is instruction-for-instruction the code of
  `388a9f8` (only the line numbers differ; `cv-a.s`, `cv-b.s`).
- Why taken branches cost (inferred, consistent with how wide cores fetch): a not-taken compare-and-branch is nearly
  free, while every taken branch ends a fetch block, so a byte loop's speed follows its taken branches per byte. A
  control supports it: prototype P2 (the `encoding/json` form `c <= ' ' && (c == ' ' || ...)`) executes **fewer** instructions for an ordinary byte but adds
  one taken branch in `compactAndValid`'s loop, and it costs +16 % there (+11 % `CleanLen`, +7 % `CleanObject`).

**What this decides:** a shape works when the ordinary byte runs down not-taken branches with one taken branch back
to the loop head, whatever the source looks like. A predicate whose negation guards the hot path is a bet on the
block layout. A table lookup has one test instead of four, so a flipped layout costs at most one taken branch per
byte; a loop that runs over kept bytes and stops only at a rare byte makes the rare byte the loop's exit.

### A1.2 Options, ranked by code health

Every option below passed, in its scratch copy: the whole `internal/event` test suite (the nets
`TestCleanLenIsTheStoredLength`, `TestCleanObjectEachOperand`, `TestCompactAndValidEachOperand`, the allocation pins
`compactAndValid` 0, `CleanLen` 0, `cleanObject` 7 over the six, `FuzzDecode`'s seeds), plus a differential test
written for this review (`zz_diff_test.go`): each prototype against a verbatim copy of today's three functions on
1.5 million random byte strings from a JSON-and-UTF-8-edge alphabet (each also with spare capacity past its length),
every string of up to three symbols, the six objects and the long objects; equal bool, equal string, equal count,
and a panic exactly where today's code panics. Cyclomatic numbers are gocyclo's rule (counted by a small go/ast
tool, checked against the known 13 / 13 / 9).

**Option 1 - one run walker (prototypes P3, P5, P7; recommended form P7).** A single loop, `cleanRun(raw, i, inStr)
(end int, inStr bool)`, reads the bytes the stored form keeps as they are and stops at the first it does not keep
(whitespace outside a string, a malformed byte inside one). The three functions become its callers:
`compactAndValid` is "the first run reaches the end"; `CleanLen` adds run lengths and 3 per malformed byte;
`cleanObject` appends each run in one copy and U+FFFD per malformed byte. P5 adds a 256-entry `strPlain` table
(ASCII but `"` and `\`) so a string's plain byte is one table test instead of three compares; P7 also skips a whole
whitespace run between two runs with an inlined `skipSpace` (one walker call per run of whitespace, not per byte).
- cyclomatic / lines: `cleanObject` 4 / 17, `CleanLen` 4 / 17, `compactAndValid` 1 / 4, `cleanRun` 9 / 27,
  `skipSpace` 3 / 6; no waiver. Allocations equal (pins pass).
- Health: the cleaning grammar (in a string or not, escape, quote, malformed byte) exists **once** instead of three
  times; `CleanLen == len(cleanObject)` holds by construction (both sum the same runs) rather than by a property
  test over two copies; the compact check is no longer a separate pre-pass for a non-compact object (one walk, not
  two).
- Why it is not the shared walker the design rejected (E.2.2): no per-byte callback; the walker returns per run
  and every caller's per-run work is two or three lines. The design's other objection, `compactAndValid`'s early
  exit, is the walker's natural stop.
- One subtlety: for a string cut off right after a backslash (invalid JSON, outside `CleanObject`'s contract)
  today's `cleanObject` panics on `raw[i+1]`, and `CleanLen` returns `len(raw)` if the object is otherwise compact
  but counts the missing byte if it is not. The walker returns `len(raw)+1` there and its first run is the compact
  check, so `CleanLen` gives the same two answers, and `cleanObject` slices `raw[i:end:len(raw)]` so it panics as today instead
  of reading spare capacity. The differential test pins both.

**Option 2 - a whitespace table only (P1).** `var jsonSpace = [256]bool{' ': true, '\t': true, '\n': true, '\r':
true}`; `!jsonSpace[c]` replaces the `&&` chain in `cleanObject` and `CleanLen`, `jsonSpace[c]` replaces `isSpace(c)`
in `compactAndValid`. One load and one test instead of four compares; in P1's generated code the ordinary byte takes
the same three taken branches as today, no more.
- cyclomatic: `cleanObject` 10, `CleanLen` 10, `compactAndValid` 9; lines unchanged; no waiver.
- Health: the smallest change; but three copies of the grammar stay, the two clean functions sit exactly at the
  limit (the next condition reopens this review), and a non-compact object is still walked twice.

**Option 3 - the design's margin option with the table (P4).** Option 2 plus `appendCleanRune` / `cleanRuneLen`
for the non-ASCII arm (the design's E-Q3).
- cyclomatic: `cleanObject` 8, `CleanLen` 8, the helpers 3 each; no waiver.
- Health: margin under the limit, but a fourth and fifth copy of the malformed-byte decision, and one non-inlined
  call per non-ASCII rune of a non-compact object.

**Option 4 - the `encoding/json` form of `isSpace` (P2), `c <= ' ' && (...)`.** Same cyclomatic numbers as option
2. **Rejected: it costs** (+7 to +16 %), because it adds a taken branch on the ordinary byte (A1.1).

**Considered, not prototyped (reasons):**
- SWAR 8-byte checks for the compact fast path: the walk toggles `inStr` per quote, so 8-byte words only help to
  skip string bodies; it needs word loads, a tail loop and endianness care for payloads of 30-600 bytes, where the
  table loop of option 1 already cuts `compactAndValid` by about 40 % on the long object. Disproportionate.
- `bytes.IndexByte` / `bytes.IndexAny` to skip a string body: a string must stop at `"`, `\` **and** any byte of
  0x80 or more, which no `IndexAny` set expresses; `IndexAny` also builds its ASCII set on every call, which loses
  on short strings. `IndexByte` finds one byte only.
- One pass computing the clean length that `cleanObject` sizes its buffer from: it adds a walk to save
  over-allocation; the pins count allocations, not bytes, and the time can only grow. Option 1 already makes both
  functions one walk each.

### A1.3 Indicative measurements

**How measured (both areas).** Indicative only: other agents may have been running. Each row is one run of
`gate/bench/compare.py --pairs 10` (ABBA-interleaved, `-test.cpu=1`, 1 s per sample) with **today's code as
"before"** (lane E head `94c5972`; `object.go` there is `c54fe6c`'s) and the prototype as "after", both test binaries
built from the same tree and, within each comparison, carrying the same added test files (`zz_long_bench_test.go`,
`zz_long2_bench_test.go` for the second scanner set, the differential tests and reference copies). Cells: median change; "costs" = compare.py's rule (p < 0.05 and median
more than 2 % slower), "faster" = p < 0.05 and more than 2 % faster, "-" = neither. Allocations were equal in every
row (compare.py checks them exactly). The long inputs: a realistic 617-byte pretty-printed order payload (nested
objects and arrays, escapes, non-ASCII names, numbers with fraction and exponent) and the same object compact (502
bytes); the `Long*` object benchmarks run over both, `LongScannerSpaced` / `LongScannerCompact` over one each.
**Layout control:** `base-pad` and `base-pad2` are today's code with only an unused function placed before it
(moving every hot function by 144 and 176 bytes, i.e. 16 and 48 bytes within a 64-byte line); compared against
today's code they show how much code placement alone moves each benchmark.

| prototype | CleanLen | CleanObject | CompactAndValid | LongCleanLen | LongCleanObject | LongCompactAndValid |
|---|---|---|---|---|---|---|
| P1 table only (option 2) | +4.5 % - | +0.8 % - | -1.5 % - | +1.8 % - | +0.2 % - | +0.6 % - |
| P2 guarded isSpace (option 4) | +10.8 % costs | +7.3 % costs | +16.2 % costs | +5.5 % costs | +5.5 % costs | +8.4 % costs |
| P3 run walker (option 1, plain) | -18.6 % faster | -12.8 % faster | -6.8 % faster | +1.4 % - | +3.7 % - | -15.4 % faster |
| P4 margin + table (option 3) | +3.9 % - | +3.0 % - | -1.3 % - | -1.1 % - | +0.6 % - | +0.8 % - |
| P5 run walker + strPlain (option 1) | -22.6 % faster | -13.0 % faster | -8.1 % faster | -27.1 % faster | -14.8 % faster | -42.8 % faster |
| P7 = P5 + skipSpace (option 1) | -18.3 % faster | -11.9 % faster | -9.1 % faster | -24.2 % faster | -20.9 % faster | -38.5 % faster |

Reading the table:

- **Option 1 is faster everywhere once strings get the table (P5):** `CleanLen` 120.8 -> 93.5 ns, `CleanObject`
  205 -> 178 ns, `compactAndValid` 67.8 -> 62.3 ns over the six; on the long objects `CleanLen` 1045 -> 762 ns,
  `CleanObject` 1314 -> 1120 ns, `compactAndValid` 479 -> 274 ns. The plain walker (P3) was already faster on the six
  but level on the long spaced object (one call per whitespace byte, a short copy per run); the string table and
  the whitespace-run skip address exactly that. P7 adds `skipSpace` (one walker call per whitespace run): `CleanObject` on the long objects 1247 -> 986 ns
  (-21 %, against P5's -15 %), the rest within a few points of P5 (each run moves about +-3 % between runs).
- **Option 2 (P1) and option 3 (P4) show no significant change**; both have a `CleanLen` median 4-5 % above today's
  (p = 0.35 and 0.09), worth the coordinator's quiet re-run if either is chosen.
- **Option 4 (P2) costs on every benchmark** - the control that confirms A1.1's mechanism.
- **Layout control for this area: not run** (stopped to make room for the scanner's last prototypes). The scanner's
  control (A2.3) shows code placement moving a ~90 ns benchmark by 3-4 %; option 1's gains (P7: -9 % to -39 %) are
  far beyond that, options 2 and 3's +-4 % are not distinguishable from it.

### A1.4 Recommendation, and the-fool's pre-mortem

**Recommendation: option 1, the run walker, in form P7 (`cleanRun` with the `jsonSpace` and `strPlain` tables, and `skipSpace`).** It is the only option that is both healthier than
today (one copy of the grammar instead of three, every function far under the limits, one walk instead of two for a
non-compact object) and faster on every benchmark, small and long, with equal allocations. Option 2 (the table
alone) is the fallback if the owner prefers the smallest diff: no measured cost, but no margin (10 = the limit) and
its `CleanLen` median was +4.5 % (not significant, p = 0.35), which the coordinator's quiet re-measurement should
settle.

- **Pros:** health (single source of the cleaning rules; `CleanLen == len(cleanObject)` by construction); speed
  (P7 -9 % to -39 %, P5 -8 % to -43 %); no waiver; `compactAndValid` becomes a two-line wrapper that inlines into
  `ExactObject`.
- **Cons:** it reverses the design's "no shared walker" line (E.2.2), which needs the owner's consent and an
  annotation (ground rule 4); the operand nets were written per function and must be re-pointed at `cleanRun`; two
  256-byte read-only tables are package state.

**Pre-mortem** (the-fool, "find the failure modes"; steelmanned thesis: "one run walker gives identical results to
the three loops on every input, and is faster on every realistic payload"):

1. **A behaviour difference on an input nobody generated.** Three months on, a recording replays differently because
   `CleanLen` and `cleanObject` disagree on some payload. *How:* a boundary case of the run split (a malformed byte
   right after whitespace, an escape at the end, spare capacity) handled differently. *What shows it:* the
   differential test against the verbatim old code (kept as a test-only oracle in the lane until the sprint closes)
   and `FuzzDecode`; `TestCleanLenIsTheStoredLength`. *Mitigation:* land the oracle test first (red against a planted
   difference), then the walker.
2. **A payload shape where it is slower.** A producer sends deeply indented JSON (say 8 spaces per level, half the
   bytes whitespace). P3 made one walker call per whitespace byte and was level on the long object; P7's `skipSpace`
   makes it one call per whitespace run. *What shows it:* a leaf benchmark over a heavily indented object (add one).
   *Mitigation:* keep `skipSpace`; the run walker's cost is per run, not per byte.
3. **The compiler re-arranges the code on a toolchain bump** (the failure that started this review). *What shows it:*
   the leaf benchmarks on the bump's pull request. The table forms are less exposed - one test instead of four, so a
   flipped layout costs one taken branch per byte, not four - but not immune.
4. **The speed-up is partly code placement.** The numbers here were taken with other agents running; placement
   alone moves a ~90 ns benchmark by 3-4 % (the scanner's layout control, A2.3). *What shows it:* the coordinator's
   quiet re-run; the margin (P7: -9 % to -39 %) is far above that spread.
5. **Review churn:** the reversal of E.2.2 is read as scope creep. *Mitigation:* it is a design choice inside the
   lane's own refactor, with no API change; present it to the owner as such (ground rule 1).

*Synthesis:* the recommendation holds on three conditions, each cheap: the differential oracle lands first (test
before change, ground rule 9), a heavily indented object joins the leaf benchmarks, and the coordinator's quiet re-run
confirms the gain. *Owner's turn:* whether reversing E.2.2's "no shared walker" is acceptable.

## Area 2: the scanner's `object` and `str`

### A2.1 Why `object` through `member` and `str` through `unicodeEscape` are slower

**`object` through `member`** (generated code of `bedad36` against `f9d2817`, and of today's `member`):

- Not the number of calls: per member, `f9d2817` makes `str`, `colon`, `value` from `object`; `bedad36` makes
  `member` from `object`, and `str`, `value` from `member`. Three calls each way; `member` (cost 420) and `colon`
  (cost 136) were never inlined.
- The `(key, vstart, err)` results do cost something `object` throws away: after `str` returns, `member` loads
  `s.b`'s length and capacity, makes two bounds checks for `s.b[kstart+1 : s.i-1]` and a masked pointer computation
  (12 instructions), then **stores key's three words and `vstart` to its frame before calling `value` and loads them
  back after**, and writes six result registers on every return. Its frame is 0x48 bytes against 0x28 for `colon`
  and `object`; its listing has 26 frame references against 14 for a reader without the results.
- But that is not the whole cause. Prototype S1 replaces `member` with `name` (the name and its colon only, no value,
  no key slice, one spill) and still costs **+6.1 %** on `BenchmarkScannerValue` (samples tight, p = 1.1e-05). And
  prototype S6, which keeps `object` calling `str`, `colon` and `value` itself exactly as today and only rewrites
  the separator test (two inlined `s.at` checks and a cold `listFault`), costs **+3.2 %** there (p = 0.012) and
  2.3 % more time on the long compact payload, while being level on the spaced one. So on this three-value, ~92 ns benchmark
  every edit of `object` moved it by 3-6 %: part of it is real work (an extra frame per member for `member`/`name`,
  the key and its spills for `member`, a second bounds check on the `}` path for S6), and part is placement - see
  the layout control in A2.3, which moves only the code's addresses.

**`str` through `unicodeEscape`** (`1f09fe4` against `bedad36`):

- `BenchmarkScannerValue`'s inputs contain **no `\u` escape**, so the new call is never made.
- The per-byte loop is the same instruction for instruction (load `s.i`, load `s.b`, compare `"`, `\`, `< 0x20`,
  store `s.i`, jump), except that `s` is now spilled once at entry (it must survive the possible call) and the loop
  head moved four bytes (0x24 to 0x28). The +3.1 % / +2.1 % is that one store per string plus code placement.
- Placement alone moves this benchmark by as much: today's code, unchanged but placed 176 bytes later, measures
  **+3.4 %** on `BenchmarkScannerValue` (p = 1.1e-05; A2.3). The `str` split's +3.1 % / +2.1 % is within that.

**What the generated code shows for both functions, and for the whole scanner:** every byte of a string and every
whitespace byte goes through memory: `s.i` is loaded, `s.b`'s pointer and length are loaded, `s.i` is stored, with
two taken branches per plain string byte. That, not the splits, is where the scanner's time goes on a realistic
payload; a loop that keeps the index in a register (prototype S2's `str`) cuts a 617-byte pretty-printed payload by
about a third.

**What this decides:** the shapes that work keep `str`'s index in a register, keep the simple escapes inline (a call
in the backslash arm is paid on short strings), and return nothing a caller drops. Whether `object` reaches `str`
directly or through a reader matters less than those three: with them, both cuts of `object` measured no cost.

### A2.2 Options, ranked by code health

Every scanner option passed the whole `internal/event` suite in its scratch copy (`TestScannerFaultsEachOperand`,
`TestScannerMemberFaultsAreReported`, `TestBreakitSyntaxOffsets`, the operand nets, the pins, `FuzzDecode`'s seeds)
and a differential test written for this review (`zz_scandiff_test.go`): each prototype's scanner against a
verbatim copy of today's (`refScanner`) on one million random inputs from a JSON alphabet with escape, number,
literal and control-byte pieces, every input of up to three pieces, the benchmark inputs and the 16 fault rows;
`value()` and `document()` compared on the error text, the `*DecodeError` (reason, offset, message), `s.i`,
`s.depth`, `s.most`, the closing offset, every span and `Decoded`. So every fault keeps its offset and text, and
`s.i` is left where today's code leaves it on every exit.

**The common part: `str` with a register loop.** `str` copies `s.b` and the index into locals and loops
`for i := s.i + 1; i < len(b); i++`, writing `s.i` back at each exit exactly where today's code leaves it. A plain
byte no longer goes through memory (today: load `s.i`, load `s.b`, store `s.i`, two taken branches); with the
`strByte` table (S10, S11) it is one load, one table test and one taken back-edge. The eight simple
escapes stay inline; only the `\u` check moves to `unicodeEscape()` (its loop, both faults and `s.i += 6` verbatim,
as `1f09fe4` did): `str` 11 -> 9 (33-34 lines), `unicodeEscape` 4. Variants measured: S7 moves every escape to one
`escape()` (str 6, escape 7; one call per backslash); S10 adds a `strByte [256]bool` table as the first case,
because in S8 the compiler laid the backslash case as the fall-through (a plain byte then took two taken branches;
as far as the generated code shows, Go's block layout treats a branch towards a call as unlikely, so S7's
`escape()` call had kept the backslash arm out of the way).

**Option 1 - one `name` reader for `object` and `top`, plus the register `str` (S9, or S11 with the table).** `name() (int, error)` reads
`"name"`, whitespace, `:` and whitespace from `s.i` and returns the closing quote's offset; `object` and `top` both
call `s.ws()`, `name`, then `value` themselves (`top` slices the key after `value`); `member` and `colon` are deleted.
cyclomatic: `object` 13 -> 9 (33 lines), `top` 7 -> 8, `name` 4, `str` 9, `unicodeEscape` 4.
- Health: the best structure - one reader for the member-name grammar where today there are two (`top` through
  `member`, `object` through its own quote check plus `colon`); nothing is returned that a caller throws away; no
  function at the limit.
- Cost: `name` alone (S1) measured +6.1 % on `BenchmarkScannerValue`, above the layout control's spread.
  **With the register `str` (S9) there is no cost on any benchmark:** `BenchmarkScannerValue` 92.3 -> 91.9 ns (-0.4 %, not significant),
  `BenchmarkScannerDocument` -3.9 %, long compact 656 -> 434 ns (-34 %), spaced -33 %, `BenchmarkScannerFaults`
  level. **S11** (the same plus the `strByte` table): `BenchmarkScannerValue` -2.2 %, `BenchmarkScannerDocument`
  -10.1 %, long compact 649 -> 385 ns (-41 %), spaced -35 %.

**Option 2 - `object` cut at its separator, plus the register `str` (S8, or S10 with the table).** `object` keeps
today's calls exactly (`enter`; then per member `str`, `colon`, `value`), uses `s.at('}')` and `!s.at('"')` (as `top`
and `member` already do), and writes the separator as `switch { case s.at(','): ...; case s.at('}'): ...; default:
return s.listFault("unterminated object", "want ',' or '}'") }`; `listFault(atEnd, otherwise string) error` picks
the message by `s.i >= len(s.b)`, both at `s.i` - the same offsets and texts as the old `if` plus `switch`.
cyclomatic: `object` 13 -> 10 (36 lines), `listFault` 2, `str` 9, `unicodeEscape` 4; `colon` and `member` unchanged.
- Health: smallest structural change; `listFault` can serve `array` and `top` too. But `object` sits exactly at
  10, and the two versions of the name grammar stay.
- Cost: none measured. S8 had no cost on any benchmark: `BenchmarkScannerValue`
  92.3 -> 89.5 ns (-3.1 %), `BenchmarkScannerDocument` -7.5 %, the long payloads -35 % (compact 647 -> 424 ns,
  spaced 782 -> 511 ns), `BenchmarkScannerFaults` level. S10 (the same plus the `strByte` table) is faster still on long strings:
  `BenchmarkScannerValue` -3.2 %, `BenchmarkScannerDocument` -8.5 %, long compact 665 -> 369 ns (-44 %), spaced -37 %.

**Option 3 - option 1 or 2 plus a table-driven `ws` with its index in a register (S4 measured on S2).** Fastest on
the long pretty-printed payload (-44 %), but +13.7 % on `BenchmarkScannerValue`: `ws` is inlined at a dozen sites,
the compact inputs reach it on a non-space byte, and the new form pays a table address, a load and an unconditional
store of `s.i` that the old switch did not. Not recommended in this form (a form that stores only when it moved
was not tried).

**Considered, not prototyped (reasons):**
- An inlinable `memberColon` predicate: it replaces one `if` with one `if` in `object`, so it does not lower the
  cyclomatic number by itself; in option 1 the colon is part of `name`.
- `bytes.IndexAny` to skip a string body to its next quote, backslash or control byte: the set has 34 bytes and is
  built on every call; JSON keys are mostly 1-10 bytes, where the call costs more than the loop, and the register
  loop already makes a plain byte cheap. `bytes.IndexByte` finds one byte only.
- Keeping `\u` inline while cutting `str` another way (a bounds-safe `peek` and cold fault helpers): possible at
  cyclomatic 9-10 but needs four small helpers to save one call per `\u`, which is rare in payloads.

### A2.3 Indicative measurements

Same method as A1.3. First set (long input = both long objects in one benchmark):

| prototype | ScannerValue | ScannerDocument | ScannerFaults | LongScannerValue |
|---|---|---|---|---|
| S1 name only | +6.1 % costs | +0.6 % - | +0.2 % - | +6.2 % costs |
| S2 name + register str, escape() | +3.4 % costs | -7.3 % faster | +0.9 % - | -35.4 % faster |
| S4 = S2 + table ws (option 3) | +13.7 % costs | -3.3 % faster | -0.8 % - | -43.9 % faster |

Second set (long inputs split; today's code against each):

| prototype | ScannerValue | ScannerDocument | ScannerFaults | LongScannerSpaced | LongScannerCompact |
|---|---|---|---|---|---|
| S6 separator cut, str whole | +3.2 % costs | -0.6 % - | +0.0 % - | -0.4 % - | +2.3 % costs |
| S7 = S6 + register str, escape() | +4.1 % costs | -9.2 % faster | -0.1 % - | -36.8 % faster | -39.2 % faster |
| S8 = S6 + register str, unicodeEscape() (option 2) | -3.1 % faster | -7.5 % faster | +1.9 % - | -34.6 % faster | -34.5 % faster |
| S9 = name + register str, unicodeEscape() (option 1) | -0.4 % - | -3.9 % faster | +0.9 % - | -33.3 % faster | -33.8 % faster |
| S10 = S8 + strByte table (option 2) | -3.2 % faster | -8.5 % faster | -0.9 % - | -37.4 % faster | -44.5 % faster |
| S11 = S9 + strByte table (option 1) | -2.2 % faster | -10.1 % faster | -0.3 % - | -34.8 % faster | -40.7 % faster |
| layout control base-pad | +1.8 % - | -1.0 % - | +2.0 % - | +2.7 % costs | -0.6 % - |
| layout control base-pad2 | +3.4 % costs | -2.8 % faster | +1.2 % - | +0.6 % - | -4.5 % faster |

Reading the tables:

- **The register `str` is the big lever:** every prototype with it is 33-44 % faster on both realistic payloads and
  4-10 % faster on `BenchmarkScannerDocument`, with `BenchmarkScannerFaults` level.
- **With the simple escapes kept inline, nothing costs anywhere:** S8 (separator cut) -3.1 % on
  `BenchmarkScannerValue`, S10 (the same with the table) -3.2 %, S9 (`name` reader) -0.4 %; on the long compact
  payload S10 is the fastest (-44 %), the table removing the second taken branch S8 had per plain byte.
  S11 (the `name` reader with the table) is likewise free everywhere: -2.2 % on the tiny benchmark, -10.1 % on the
  document benchmark, -41 % / -35 % on the long payloads.
- **`BenchmarkScannerValue` (three short compact values, ~92 ns) moved +3 % to +6 % with every edit of `object`**
  (S1 +6.1 %, S6 +3.2 %), and S2 / S7 did not win it back (+3.4 %, +4.1 %): their `escape()` call is paid once per run
  for the `\n` in `"xé\n"`, and the strings are one to four bytes long. **S8 keeps the simple escapes inline and
  is -3.1 % there** - the same `object` as S6, so the register `str` more than pays for the separator cut.
- **S4** shows the whitespace table's trade: -44 % on the long payload, +13.7 % on the tiny one.
- **The layout control is the key reading of this table.** `base-pad2` is today's scanner, byte-for-byte the same
  instructions, only placed 176 bytes later: it "costs" **+3.4 %** on `BenchmarkScannerValue` with p = 1.1e-05 and no
  overlapping sample (92.5 -> 95.7 ns), and is 4.5 % *faster* on the long compact payload. `base-pad` (144 bytes) moves
  `BenchmarkScannerValue` +1.8 % (p = 0.052) and "costs" +2.7 % on the long spaced payload. So **code placement alone
  moves these benchmarks by about 3-4 %, in either direction, with p-values as small as any real change**. The
  `object` and `str` splits' measured costs (+4.4 / +6.0 % and +3.1 / +2.1 %) and S2/S6/S7's +3-4 % on
  `BenchmarkScannerValue` are of the same size; only S1's +6.1 % clearly exceeds the controls. compare.py's 2 % bar
  sits below this floor for a ~92 ns benchmark: a question for the owner, not something this review can settle (see
  A2.4).

### A2.4 Recommendation, and the-fool's pre-mortem

**Recommendation: option 1 - one `name` reader for `object` and `top`, plus the register-loop `str` with only the
`\u` check out of line - in the table form S11 (`s11-excerpt.go`).** It is the healthiest structure measured (one reader for the member-name
grammar instead of two; `member` and `colon` go; `object` 9, `top` 8, `str` 9, no function at the limit; nothing
computed that a caller throws away) and it had no cost on any benchmark while making realistic payloads about a
third faster. Option 2 in its table form (S10) is the fallback: the smallest structural change, no cost anywhere
and -3.2 % on `BenchmarkScannerValue`, but `object` sits at exactly 10 and the two name readers stay.

- **Pros:** health as above; speed on every realistic input; the fault texts and offsets provably unchanged
  (differential); no waiver.
- **Cons:** `top` changes too (its cyclomatic 7 -> 8; it slices the key itself after `value`); `member`, which the
  lane added in `00bf44e`, is removed again; `name` alone was the slowest piece measured (S1 +6.1 %), so the result
  rests on the register `str` paying for it - if a later change slows `str`, `object`'s share shows again.

**A question for the owner that this area raises (not a finding against the lane):** the layout control shows
code placement alone moving `BenchmarkScannerValue` by +3.4 % with p = 1.1e-05, above compare.py's 2 % "costs" bar (the verdict decision 1 of the plan check applies). On a
~92 ns benchmark a 2-4 % "cost" therefore cannot be told apart from placement by one compare.py run. Two ways the
owner could take, if wanted: judge such a split also against two placement controls of the old code (as here), or
weigh the leaf benchmarks together with a realistic payload. The rule 13a trigger itself (the path's +8.5 % and +8.1 %) is above the controls' spread; its per-function attribution (+2 % to +6 %) is not.

**Pre-mortem** (the-fool, "find the failure modes"; steelmanned thesis: "one `name` reader for `object`
and `top`, and a register-loop `str` with only `\u` out of line, keep every fault's offset and text and make the
scanner faster on real payloads at no cost on the tiny benchmark"):

1. **A fault moves.** A later fix finds that some malformed payload now reports a different offset, and a
   recording's expected rejection breaks. *How:* `str`'s exits must leave `s.i` where today's code leaves it (each
   exit writes it), and the member reader must keep the fault order (name, `want ':'`, value). *What shows it:* the
   differential test against the verbatim old scanner (one million inputs plus every three-piece input, comparing
   `*DecodeError`, `s.i`, depth, spans), `TestScannerFaultsEachOperand`, `TestBreakitSyntaxOffsets`.
   *Mitigation:* land the differential test (as a test-only oracle) before the change, red against a planted move.
2. **The quiet re-run reverses the small-input result.** S11 read -2.2 % (S9 -0.4 %) on `BenchmarkScannerValue`, inside
   the placement spread (+-3.4 %); a quiet re-run could read +2-3 % and trip compare.py's 2 % bar while every realistic
   payload is 30-40 % faster. *What shows it:* the re-run beside a placement control. *Mitigation:* option 2's table
   form (S10, -3.2 % there, same `str`) is the fallback; or the owner's call on the bar (above).
3. **Escape-dense strings.** A producer embeds text with `\"` and `\n` every few bytes; the recommended `str` keeps
   simple escapes inline, so only `\u` pays a call. *What shows it:* a leaf benchmark over an escape-dense and a
   `\u`-dense string (add both; today's set has no `\u` on the success path at all).
4. **`name` changes `top`'s profile.** `top` (cyclomatic 7 -> 8) now slices the key after `value` and grows a frame;
   `BenchmarkScannerDocument` measured faster, but a schema with many keys could differ. *What shows it:* the
   document benchmark over a long payload with many members (add one).
5. **A toolchain bump re-arranges the scanner's blocks** - S8 against S10 shows how much one fall-through decides;
   the leaf benchmarks on the bump's pull request show it.

*Synthesis:* the recommendation holds if the differential oracle lands first, escape-dense, `\u`-dense and
many-member benchmarks join the leaf set, and the quiet re-run is read next to a placement control. *Owner's turn:*
whether `member` and `colon` may give way to one `name` reader (the lane's own `member` from `00bf44e`
removed), and whether a 2-4 % reading on a ~92 ns benchmark is judged against a placement control.

## Scratch paths (prototypes, logs, listings)

All under `/private/tmp/claude-501/-Users-ramith-code-cep/e3315a83-1fa1-43ac-8afe-3d451de6f832/scratchpad/walkers-review/`:

- Today's code plus the added tests: `base/` (`git archive 94c5972`); layout controls `base-pad/`, `base-pad2/`.
- Walker prototypes (each a full module copy; the change is in `internal/event/object.go`): `p1-table/`,
  `p2-guard/`, `p3-run/`, `p4-margin/`, `p5-runtable/`, `p6-runskip/` (P3 + skipSpace, built, not measured),
  `p7-runtable-skip/`; the recommended code alone: `p7-excerpt.go`.
- Scanner prototypes (change in `internal/event/scanner.go`): `s1-name/`, `s2-namestr/`, `s4-ws/`, `s5-strtable/`
  (S4 + a `strByte` table in `str`; built and tested, not measured), `s6-objsep/`, `s7-objsep-str/`,
  `s8-objsep-str2/`, `s9-name-str2/`, `s10-objsep-str3/`, `s11-name-str3/`; code excerpts `s11-excerpt.go` (the recommended
  `top`, `name`, `object`, `str`, `unicodeEscape`, `strByte`), `s7-excerpt.go`, `s8-excerpt.go`, `s9-excerpt-top-name.go`.
- Added test files (copied into every tree): `shared/zz_ref_test.go` (verbatim old walkers), `shared/zz_diff_test.go`
  (walker differential), `shared/zz_refscan_test.go` (verbatim old scanner as `refScanner`),
  `shared/zz_scandiff_test.go` (scanner differential), `base/internal/event/zz_long_bench_test.go` and
  `shared/zz_long2_bench_test.go` (long-payload benchmarks).
- Test binaries: `bin/` (area 1, first scanner set), `bin2/` (second scanner set). compare.py logs: `logs/area1-*.log`,
  `logs/area2-*.log`, `logs/b2-*.log` (each ends with its samples, build ids and exit code).
- Generated code: `asm-<rev or prototype>.txt` (full `-gcflags=-S`), extracted functions `cl-a.s` / `cl-b.s`
  (`CleanLen` at 388a9f8 / 53fb5f0), `co-*.s` (`cleanObject`), `cv-a.s` / `cv-b.s` (`compactAndValid`),
  `sc-*.s` (`object`, `member`, `colon` at f9d2817 / bedad36), `str-*.s` (`str` at bedad36 / 1f09fe4); inliner
  reports `m-<rev>.txt`. Tools: `tools/fn.py` (function extractor), `tools/cyclo/main.go` (gocyclo-rule counter),
  `tools/tables.py`, `tools/run-batch*.sh`.

## Code graph and memory

- GitNexus: the `cep` index predates lane E's file move (`context("CleanLen")` resolves to
  `internal/event/decode.go:251`), so `object.go` / `scanner.go` and the new symbols are not in the graph; it was used
  only to confirm `CleanLen`'s one caller, `Runtime.SendPayload` (`internal/runtime/boundary.go`). The structure
  claims above rest on the generated code and the source at the named revisions.
- memory: none (the Hindsight tools were not available in this session; no remembered decision was used).

## Size limits checked

- golangci-lint v2.15.0 with the repo's `.golangci.toml` (`--enable-only gocyclo,funlen,lll`, logs
  `logs/lint-*.txt`) was run on P7, S11 and today's code: P7's `object.go` (waivers removed) and S11's `scanner.go`
  report nothing; today's `scanner.go` reports `object` 13 and `str` 11. Findings in other files (`NewSchema`,
  `parseRFC3339`, ...) belong to other parts of the lane and are unchanged.

## Not finished in the box

- No quiet re-run of any prototype (the rule's "confirmed by a re-run" is the coordinator's step).
- `p6-runskip` and `s5-strtable` were built and pass every test but were not measured (P7 and S10 cover their
  ideas). The area-1 layout control (`bin/base-pad.test`) was stopped before it ran, to make room for S8-S11.
