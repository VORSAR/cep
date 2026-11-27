# S1.1c-7 design note — CD1 (parser, event, clock, text, model)

*Built in W3 (2026-11-25 to 2026-11-26). Where the code differs from this note, the sprint record's W3 section says how and why ([record](s1-1c-7-record.md)).*

*S1.1c-7's design note for chunk CD1 (parser, event, clock, text, model), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes. The first part's G2-69 to G2-72 sections lost their backslashes in transport; Revision 1 replaces them.*


## The note

## S1.1c-7 design note, chunk CD1 (`internal/parser`, `event`, `clock`, `text`, `model`; lane CD)

**What the note covers**
- **Findings:** 18: G10-16, G2-65 to G2-80, and G2-134.
- **Base:** `s1-1c-7/integration` at 37df879. No Go file has changed since d521725 (`git diff --stat d521725 37df879 -- '*.go'` is empty).
- **Index freshness:** I had no tool that reads MCP resources, so I read `~/.gitnexus/storage/cep-7b08f6c21e42/meta.json` instead. Its `lastCommit` is 1f4ebba, the current HEAD.
- **Files read:**
  - `internal/parser`: parseerror.go, parse.go, errors.go, lexer.go (lexAll, sliceLexer), literal.go, lower.go:280-302, errorcounts_test.go (head).
  - `internal/parser` tests: parseerror_contract_test.go:1-480, parseerror_operands_test.go, expect_pos_test.go, fragment_test.go:576-623 and :1003-1185, fuzz_test.go, vsc_known_issues_test.go:1-20 and :175-217, trivia_test.go:12-18.
  - `internal/event`: decode.go:75-290, unescape.go.
  - `internal/event` tests: breakit_test.go:1-145, breakit2_test.go:35-60, decode_bench_test.go, leaf_allocs_test.go:1-60, decode_test.go:15-160, :240-243, :334-354, :395-650, gen_test.go:1-440, oracle_test.go:50-80 and :230-275, object_test.go:78-112, s3_2b_2_alloc_test.go, fuzz_test.go.
  - `internal/clock`: rule.go:27-35, :140-235, :326-400; clock.go:72-205, :220-232; breakit_test.go:1-270 and :540-737.
  - `internal/text`: text_test.go. `internal/plan`: expect_test.go:19-40 and :150-200.
  - `internal/model`: canonical.go:40-110, canonical_test.go:1-20, :55-62, :200-225, :735-791, model.go:190-210, :267-275.
  - Couplings: `gate/law.json` (paths and the `fragments` instrument), `gate/proofs/drivers/test_fuzz_parser_planted.py`, `gate/bench/mutate.py` and `load.py` (headers), `tools/allowlists/fragments.json` (rule and counts).
  - The plan check (§0, §1, §2, §4, §5, §7, §8, §11), testing.md, and the design brief.
- **GitNexus calls: 7.**
  - `context parseError`: callers Parse, parseError (recursive), TestParseErrorContract.
  - `context lexError`: callers ParseLiteral and Parse; processes Deploy→… and tools/shape Main→….
  - `impact lexError` upstream: **CRITICAL** (170 symbols; d=1 Parse and ParseLiteral; d=2 Runtime.Deploy, tools/shape judge, tests). No per-event entry point.
  - `context ParseLiteral`: callers Runtime.Deploy and one test.
  - `context appendUnicodeEscape`: one caller, appendEscape.
  - `impact pecEdit` upstream: LOW, 4 direct (both test files and their tests).
  - `context assertKeysAscend`: one caller, TestMarshalCanonicalKeyOrder.
- **What I could not check:**
  - No runs were allowed, so every "old"/"new" outcome below is reasoned from the code. Five carry a stop rule: G2-65's thresholds, G2-66's model, G2-68's overflow, G2-72's old coverage, G2-69's alloc pin.
  - Every finding was reached. The note runs past the 40-minute box by about 10 minutes.

### Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-16 | accepted | K1 | CD-parser-K1 | – | both non-participle fallbacks go through `mk`, so they carry the catalogue's E-SYN-001 fix |
| G2-65 | accepted | K2a | CD-clock-K2 | rename → S1.1c-8 | counts each fired timer kind (the purges among them) and the same-instant ties across keys |
| G2-66 | accepted | K2a | CD-clock-K2 | – | the test double models each pending Discard's due; OnDiscard places the discard in the walk-order check |
| G2-67 | accepted | K2a | CD-clock-K2 | – | the registry model gains the drain level and a random Drain operation |
| G2-68 | accepted | K2d | CD-event-K2 | – | at most one error per payload per worker, so the buffer can never fill |
| G2-69 | accepted | K2e (bench input) | CD-event-bench | – | leafEscaped[2] becomes the two escapes `😀`; a test pins the spelling |
| G2-70 | accepted | K2b | CD-event-K2 | – | the six rows get their `\u` escapes back, plus a test that counts them |
| G2-71 | accepted | K2a | CD-event-K2 | – | invisiblesKeys' 2nd and 3rd entries become escaped spellings |
| G2-72 | accepted | K2b | CD-event-K2 | – | escapes restored in validFor[TString], escapedStrings and the (hoisted) pick list |
| G2-73 | accepted | K2e | CD-event-K2 | – | t.Parallel in 6 tests; a reason on TestDecodeAllocationBudget |
| G2-74 | accepted | K2a | CD-model-K2 | – | the loop ends on io.EOF only; any other token error is fatal |
| G2-75 | accepted | K2e | CD-parser-K2 | – | the guards check field counts before Fields[3] and Then.Fields[0] are indexed |
| G2-76 | accepted | K2e (live data) | CD-parser-K2 | – | a register read error is fatal (`t.Fatalf`) |
| G2-77 | **rejected** | – | – | – | the zero check is the only guard for a listed document with no pin |
| G2-78 | accepted | K2e | CD-parser-K2 | – | FuzzParse checks Glob's and ReadFile's errors |
| G2-79 | accepted | K2e | CD-parser-K2 | – | the five row tables become functions of `t`; `pecEdit(t, …)` fails the test on a missing text or an odd count; the panic goes |
| G2-80 | **rejected** | – | – | – | the type assertion pins Parse's documented contract more strictly than errors.As would |
| G2-134 | accepted | K2e | CD-text-K2 (text half); CD2's plan-K2 commit (plan half) | – | t.Parallel in 6 text tests and TestBuildCarriesExpects; a reason on TestIngestFlagsAllocatesNothing |

Ledger outcomes:
- **fixed:** 15 (G10-16, G2-66 to G2-76 except G2-77, G2-78, G2-79, G2-134).
- **fixed in part:** 1 (G2-65; its rename is carried to S1.1c-8).
- **rejected:** 2 (G2-77, G2-80).
- **carried in full:** none.

---

### G10-16 (K1)

1. **Verdict: accepted.**
   - Two errors are built by hand with no `Fix`:
     - parseerror.go:18: `return &Error{Name: name, Code: "E-SYN-001", Line: 1, Col: 1, Msg: err.Error()}`
     - parse.go:94 (lexError): the same line.
   - `Error()` (errors.go:17) prints `… . Fix: %s`, so on these two branches it ends in an empty "Fix: ". `mk` (parse.go:87) sets `Fix: fixes[code]`.
   - Neither branch is reachable through Parse today:
     - lexAll's errors come from participle's lexer, which raises `participle.Error`s (lexer.go:93-119, :127-225).
     - sliceLexer.Next never errors (lexer.go:236-249).
     - ParseLiteral re-wraps lexError's E-SYN-001 through `mk` (literal.go:29-33).
   - So the only observable change is on the defensive branches. The contract test already calls `parseError("t.cep", nil, errors.New("boom"))` directly (parseerror_contract_test.go:253-258) and checks Code, Line, Col and Msg, but not Fix.
   - GitNexus `impact lexError` is **CRITICAL** because of Parse's fan-out (Deploy, tools/shape). The edit replaces one line with one line in each file. It changes only the Fix field of these two unreachable branches, and keeps line numbers, so the comments' `parse.go:91-113` references stay true.
2. **Kind: K1.**
   - R5: not per-event. GitNexus upstream reaches only Runtime.Deploy and tools/shape's judge; no Send, SendPayload, Decode, router or replay entry point.
3. **Change.**
   - parseerror.go:18 and parse.go:94 each become:
     `return mk(name, "E-SYN-001", Position{Line: 1, Column: 1}, err.Error())`
   - parseerror_contract_test.go, inside subtest `"E-SYN-001/not a participle error"`, after the existing check:
     ```go
     if want := fixes["E-SYN-001"]; want == "" || e.Fix != want {
         t.Errorf("parseError's fix %q; want the catalogue's E-SYN-001 fix %q", e.Fix, want)
     }
     if got, want := e.Error(), "t.cep:1:1: E-SYN-001: boom. Fix: "+fixes["E-SYN-001"]; got != want {
         t.Errorf("parseError's text %q; want %q", got, want)
     }
     ```
   - A new sibling subtest directly after it:
     ```go
     // lexError's first branch, the same fallback (not reachable through Parse or ParseLiteral today).
     t.Run("E-SYN-001/not a participle error from the lexer", func(t *testing.T) {
         e := lexError("t.cep", "", errors.New("boom"))
         if e.Code != "E-SYN-001" || e.Line != 1 || e.Col != 1 || e.Msg != "boom" {
             t.Errorf("lexError gave %s at %d:%d %q; want E-SYN-001 at 1:1 %q", e.Code, e.Line, e.Col, e.Msg, "boom")
         }
         if want := fixes["E-SYN-001"]; want == "" || e.Fix != want {
             t.Errorf("lexError's fix %q; want the catalogue's E-SYN-001 fix %q", e.Fix, want)
         }
     })
     ```
   - Files: internal/parser/parseerror.go, parse.go, parseerror_contract_test.go (a law path, so it shows in LAW Δ).
4. **Proof (K1).** A plain batch run on the fix commit; every entry must read red. The two entries together are the whole inverse of the fix; no new declaration is kept.
   ```json
   [{"id":"G10-16-a","file":"internal/parser/parseerror.go","find":"return mk(name, \"E-SYN-001\", Position{Line: 1, Column: 1}, err.Error())","replace":"return &Error{Name: name, Code: \"E-SYN-001\", Line: 1, Col: 1, Msg: err.Error()}","test":"TestParseErrorContract","subtest":"E-SYN-001/not a participle error","pkg":"internal/parser"},
    {"id":"G10-16-b","file":"internal/parser/parse.go","find":"return mk(name, \"E-SYN-001\", Position{Line: 1, Column: 1}, err.Error())","replace":"return &Error{Name: name, Code: \"E-SYN-001\", Line: 1, Col: 1, Msg: err.Error()}","test":"TestParseErrorContract","subtest":"E-SYN-001/not a participle error from the lexer","pkg":"internal/parser"}]
   ```
   - Each find text occurs once in its file. The other `mk(name, "E-SYN-001", …)` calls there take `at` or `pos` and a different message.
   - Expected failing lines: (a) the new `parseError's fix ""` Errorf and the text Errorf; (b) the new subtest's `lexError's fix ""` Errorf. The build agent quotes the line numbers.
   - T2 green.
5. **Commit:** CD-parser-K1.

### G2-65 (K2a; rename carried)

1. **Verdict: accepted.**
   - The doc at breakit_test.go:724-725 promises "events, every kind of timer, discards and purges, and same-instant ties across keys". The body (:727-736) counts log lines only by their first byte (`E`, `T`, `D`).
   - The log already carries the kind and the instant: OnTimers writes `T k%d d%d K%d r%d` (:107) and OnEvent writes `E k%d s%d t%d q%d` (:77).
2. **Kind: K2a.** With a plant that hides one timer kind, the old test still passes.
3. **Change.** The body of TestBreakit_GranularityScenariosAreNonTrivial becomes:
   ```go
   counts := map[byte]int{}
   kinds := map[clock.TimerKind]int{}
   ties := 0
   for seed := uint64(0); seed < 3000; seed++ {
       log, _, _ := bkRun(t, bkGen(seed), []int64{40})
       var lastAt, lastKey int64
       have := false
       for _, l := range log {
           counts[l[0]]++
           var k, at, x, y int64
           switch l[0] {
           case 'E':
               if _, err := fmt.Sscanf(l, "E k%d s%d t%d q%d", &k, &x, &at, &y); err != nil {
                   t.Fatalf("seed %d: unreadable log line %q: %v", seed, l, err)
               }
           case 'T':
               var kind int64
               if _, err := fmt.Sscanf(l, "T k%d d%d K%d r%d", &k, &at, &kind, &y); err != nil {
                   t.Fatalf("seed %d: unreadable log line %q: %v", seed, l, err)
               }
               kinds[clock.TimerKind(kind)]++
           default:
               continue // a discard's lines carry no instant
           }
           if have && at == lastAt && k != lastKey {
               ties++
           }
           lastAt, lastKey, have = at, k, true
       }
   }
   if counts['E'] < 10000 || counts['T'] < 10000 || counts['D'] < 1000 {
       t.Fatalf("too little exercised: %v", counts)
   }
   for _, k := range []clock.TimerKind{clock.Completion, clock.CronClose, clock.DelayDue, clock.BatchClose, clock.TTLPurge} {
       if kinds[k] < 100 {
           t.Errorf("timer kind %d fired %d times across the seeds; want at least 100 (kinds %v)", k, kinds[k], kinds)
       }
   }
   if ties < 100 {
       t.Errorf("%d same-instant walk items from two different keys; want at least 100", ties)
   }
   ```
   - TTLPurge fires are the purges; discards are the existing `D` count.
   - The doc line stays as written; it is now true.
   - **Stop rule:** on the real code each kind count and `ties` should be in the thousands, by my estimate. About 19,500 events schedule one of four inclusive kinds each, plus bkGen's initial timers. If any count reads below 100, stop and report; do not lower the threshold.
4. **Proof.**
   ```json
   {"id":"G2-65","file":"internal/clock/rule.go","find":"\t\t\trc.fires = append(rc.fires, Timer{Due: t.due, Kind: t.kind, Ref: t.ref})","replace":"\t\t\tk := t.kind\n\t\t\tif k == BatchClose {\n\t\t\t\tk = DelayDue\n\t\t\t}\n\t\t\trc.fires = append(rc.fires, Timer{Due: t.due, Kind: k, Ref: t.ref})","test":"TestBreakit_GranularityScenariosAreNonTrivial","pkg":"internal/clock","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
   {"id":"G2-65-control","control":true,"test":"TestBreakit_GranularityScenariosAreNonTrivial","pkg":"internal/clock","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
   ```
   - The find text occurs once (rule.go:206).
   - With the plant, BatchClose fires are reported as DelayDue. Both are inclusive, so bkOp's checks do not fire. The old totals are unchanged, so old reads green. The new BatchClose count is 0, so new reads red.
5. **Commit:** CD-clock-K2.
   - **Carried to S1.1c-8:** the rename `TestRandomWalkScenariosExerciseEveryCallback`.
     - "exercise every callback" is backed by E ≥ 10000 (OnEvent), T ≥ 10000 with each of the five OnTimers kinds ≥ 100 (OnTimers), and D ≥ 1000 (OnDiscard).
     - "random-walk scenarios" refers to bkGen's 3000 seeds.

### G2-66 (K2a)

1. **Verdict: accepted.**
   - OnDiscard (breakit_test.go:120-128) never calls `o.at`. The walk-order check (`at`, :58-66) therefore never covers discards.
   - OnDiscard receives refs, not dues (`Callbacks.OnDiscard func(in *Instance, refs []uint64)`, clock.go:245), so the test double must remember each Discard's due. Discards are strict (clock.go:232), so their walk position is (due, strict).
2. **Kind: K2a.**
3. **Change** (breakit_test.go only):
   - Add a field to `bkOp`: `discards map[[2]uint64]int64 // each pending Discard's due by (key ordinal, ref): its place in the walk`.
   - In bkRun:181, initialise it: `discards: map[[2]uint64]int64{}`.
   - Add helpers that mirror the clock:
     ```go
     // schedule schedules a timer on in, noting a Discard's due so OnDiscard can place it in the walk.
     func (o *bkOp) schedule(in *clock.Instance, due int64, k clock.TimerKind, ref uint64) {
         in.Schedule(due, k, ref)
         if k == clock.Discard {
             o.discards[[2]uint64{in.Ordinal(), ref}] = due // a second Schedule replaces it, as the clock does
         }
     }

     // cancelDiscard cancels a pending Discard and forgets its due.
     func (o *bkOp) cancelDiscard(in *clock.Instance, ref uint64) {
         if in.Cancel(clock.Discard, ref) {
             delete(o.discards, [2]uint64{in.Ordinal(), ref})
         }
     }

     // cancelAll is the purge's reset: every pending timer of the key goes, its Discards' dues with it.
     func (o *bkOp) cancelAll(in *clock.Instance) {
         in.CancelAll()
         for k := range o.discards {
             if k[0] == in.Ordinal() {
                 delete(o.discards, k)
             }
         }
     }
     ```
   - Call sites:
     - :85 `in.Schedule(e.Ts+int64((h>>28)%3), clock.Discard, e.Seq)` → `o.schedule(in, e.Ts+int64((h>>28)%3), clock.Discard, e.Seq)`
     - :88 `in.Cancel(clock.Discard, e.Seq-1)` → `o.cancelDiscard(in, e.Seq-1)`
     - :112 `in.CancelAll()` → `o.cancelAll(in)`
     - :116 `in.Cancel(clock.Discard, uint64(tm.Due))` → `o.cancelDiscard(in, uint64(tm.Due))`
     - bkRun:188 `ins[tm.key].Schedule(tm.due, tm.kind, tm.ref)` → `op.schedule(ins[tm.key], tm.due, tm.kind, tm.ref)`
   - OnDiscard's loop becomes:
     ```go
     for _, r := range refs {
         key := [2]uint64{in.Ordinal(), r}
         if due, ok := o.discards[key]; !ok {
             o.t.Errorf("discard r%d of key %d: no pending Discard with that ref", r, in.Ordinal())
         } else {
             if due >= o.rc.Clock() && o.rc.Clock() != math.MaxInt64 { // owner D1: clock > due
                 o.t.Errorf("discard r%d due %d fired with clock %d (needs clock > due, or +∞)", r, due, o.rc.Clock())
             }
             o.at(due, bkPhaseStrict, "discard")
             delete(o.discards, key)
         }
         o.log = append(o.log, fmt.Sprintf("  r%d", r))
     }
     ```
     The log format is unchanged, so the cross-granularity log comparisons are unaffected.
   - `Instance.Ordinal()` returns uint64 (rule.go:326).
   - **Stop rule:** if the real code reads red ("no pending Discard" or "walk went backwards"), the model differs from the clock. Stop and report; do not weaken the checks.
4. **Proof.**
   ```json
   {"id":"G2-66","file":"internal/clock/clock.go","find":"func (k TimerKind) strict() bool { return k == TTLPurge || k == Discard }","replace":"func (k TimerKind) strict() bool { return k == TTLPurge }","test":"TestBreakit_D1_GranularityRandom","pkg":"internal/clock","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
   {"id":"G2-66-control","control":true,"test":"TestBreakit_D1_GranularityRandom","pkg":"internal/clock","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
   ```
   - With the plant, discards fire in the inclusive phase, out of their strict place.
   - Old reads green: the walk is still deterministic and the same at every granularity, and no old check reads a discard's position.
   - New reads red: a discard recorded at (d, strict) is followed by an inclusive item at d of a higher-ordinal key, so "walk went backwards". With 1-4 keys and dues within 0-2 of each event, this is near-certain across 3000 seeds.
5. **Commit:** CD-clock-K2.

### G2-67 (K2a)

1. **Verdict: accepted.**
   - bkStreamModel (:553-570) has no drain term, and the operation switch (:647) never calls Drain.
   - The real contribution includes it: `c := max(s.adm, s.floor, s.vfloor, s.punct, s.drain)` (clock.go:87). Drain raises every stream to `add(max admitted maxTs, 1)` when anything was admitted (clock.go:175-191).
   - The section claims "a direct model of design §1" (:551), so the model leaves out a term of the formula it claims.
2. **Kind: K2a.**
3. **Change** (breakit_test.go):
   - The struct's `floor, vfloor, punct int64` becomes `floor, vfloor, punct, drain int64`.
   - contrib: `c := max(m.floor, m.vfloor, m.punct, m.drain)`.
   - The init at :586-587 adds `drain: negInf`.
   - `switch op := r.intn(10); {` → `r.intn(11)`.
   - The punctuation branch's `default:` → `case op < 10:`.
   - A new `default:` branch:
     ```go
     default:
         trace = append(trace, "drain")
         to, seen := negInf, false
         for _, m := range ms {
             if m.hasMax {
                 to, seen = max(to, m.maxTs+1), true
             }
         }
         if seen {
             for _, m := range ms {
                 m.drain = max(m.drain, to)
             }
         }
         check(reg.Drain(nil))
     ```
   - The doc (:572-575) gains "and drains" and "drain level" in its lists.
   - The RNG stream changes, so the seeds produce new scenarios. That is acceptable for a property test.
4. **Proof.**
   ```json
   {"id":"G2-67","file":"internal/clock/clock.go","find":"to := add(m, 1)","replace":"to := m","test":"TestBreakit_RegistryMatchesTheModel","pkg":"internal/clock","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
   {"id":"G2-67-control","control":true,"test":"TestBreakit_RegistryMatchesTheModel","pkg":"internal/clock","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
   ```
   Old never calls Drain, so it reads green. New reads red with "stream %d contribution %d, model %d".
5. **Commit:** CD-clock-K2.

### G2-68 (K2d; load run)

1. **Verdict: accepted.**
   - breakit_test.go:120: `errs := make(chan string, 8*len(payloads))`.
   - A worker sends once at :130 (the shape differs), or once per mismatching value at :135, up to NumAttrs = 7 for "timed".
   - Past the buffer the senders block, `wg.Wait()` (:141) never returns, and the test hangs.
2. **Kind: K2d** (a hang inside the process).
3. **Change:** after `errs <- p` at :135, add `break`. Each worker then sends at most one message per payload, at most `len(payloads)` in total, so 8 workers never exceed the buffer.
4. **Proof.**
   ```json
   {"id":"G2-68","file":"internal/event/decode.go","find":"\t\tvals = append(vals, v)\n","replace":"\t\tif cap(dst) > 0 {\n\t\t\tv.Null = !v.Null\n\t\t}\n\t\tvals = append(vals, v)\n","test":"TestBreakitDecoderSharedConcurrently","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"timeout","new":"red"}},
   {"id":"G2-68-control","control":true,"test":"TestBreakitDecoderSharedConcurrently","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
   ```
   - The find text occurs once (decode.go:101).
   - The serial run passes a nil dst and stays unflipped. A worker reuses dst after each success, so every value of a success that follows a success mismatches: 7 sends.
   - About 33 of the ~94 payloads succeed on "timed", mostly in runs. That gives about 200 sends per worker, about 1,600 in total against a buffer of about 752. Old blocks and hits mutate's `--timeout` (default 120s): **timeout**. New reports "a concurrent Decode(…) disagrees with the serial one": **red**.
   - **Stop rule:** if old reads red rather than timeout, report it. The overflow estimate was then wrong.
   - **Load run** (goroutines): `python3.15 -I -B gate/bench/load.py -- -race -count=50 -run '^(TestBreakitDecoderSharedConcurrently|TestDecodeConcurrent|TestDecodeCases)$' ./internal/event`. Each must be 50/50. This one run also covers G2-70 and G2-73.
5. **Commit:** CD-event-K2.

### G2-69 (K2e, bench input, its own commit; R4)

1. **Verdict: accepted.**
   - decode_bench_test.go:142 is `` []byte(`😀`) ``. `od -c` shows the raw 4-byte rune, no backslash.
   - The comment at :137 promises "a surrogate pair written as two \u escapes".
   - So BenchmarkAppendUnescaped (:149-159) and the appendUnescaped pin (leaf_allocs_test.go:49-53) never reach the pair branch, unescape.go:76-79.
2. **Kind: K2e** (benchmark input).
3. **Change.**
   - :142 becomes `` []byte(`😀`), ``. As a Go interpreted string the input is `"\\ud83d\\ude00"`: 12 bytes, two six-byte escapes.
   - A new test in the same file (add `"strings"` to the imports):
     ```go
     // TestTheSurrogatePairInputIsWrittenAsTwoEscapes: leafEscaped[2] is the pair's two \u escapes as text, so
     // BenchmarkAppendUnescaped and the appendUnescaped pin reach the pair's branch, and it decodes to U+1F600.
     func TestTheSurrogatePairInputIsWrittenAsTwoEscapes(t *testing.T) {
         t.Parallel()
         in := string(leafEscaped[2])
         if in != "\\ud83d\\ude00" || strings.Count(in, `\u`) != 2 {
             t.Fatalf("leafEscaped[2] = %q; want the two escapes \\ud83d\\ude00", in)
         }
         if got := string(appendUnescaped(nil, leafEscaped[2])); got != "\U0001F600" {
             t.Fatalf("appendUnescaped(%q) = %q; want U+1F600", in, got)
         }
     }
     ```
   - Benchmark names and bodies are unchanged (R4). BenchmarkAppendUnescaped's measured work grows by one pair decode, which is why the change is its own commit.
4. **Proof (K2e):**
   - Run at the lane base and at this commit: `go test -count=1 -run '^$' -bench '^BenchmarkAppendUnescaped$' -benchtime=1x -coverprofile=<f> ./internal/event`.
   - Read the profile line for unescape.go's block that starts on line 76 after the `{` and holds :77 (`dst = utf8.AppendRune(dst, pair)`, the first statement of the pair branch).
   - Count: old **0**, new **> 0**.
   - **Stop rule:** TestDecodeLeafAllocations' appendUnescaped pin must stay 0, because the 4-byte result fits the reused [128]byte. If it moves, stop.
5. **Commit:** CD-event-bench.

### G2-70 (K2b)

1. **Verdict: accepted.** Python `ascii()` on decode_test.go shows no escape in any of the six rows:
   - :159 is `{"v":"é中A"}`, raw.
   - :160 is a raw 😀.
   - :242 is `{"ts":"2026-11-04T10:00:00Z"}`.
   - :336 is `{"user":"alice","user":"root"}`.
   - :346 is `{"v":"a"}`.
   - :353 is `{"v":"a","text_invisibles":1}`.

   None reaches appendUnicodeEscape (unescape.go:71), record's unescape (decode.go:113-115) or timestampOf's unescape (decode.go:275-277). The rows have been this way since a139dbf. TestDecodeCases (:399) runs each row as a parallel subtest.
2. **Kind: K2b.**
3. **Change** (decode_test.go, inputs only; the expected values stay as written). Each input is a raw string. Its Go interpreted form is given after it.

   | Line | Row | New input (raw string) | Interpreted | `\u` count |
   |---|---|---|---|---|
   | :159 | string/u escapes | `` `{"v":"é中A"}` `` | `"{\"v\":\"\\u00e9\\u4E2D\\u0041\"}"` | 3 |
   | :160 | string/surrogate pair | `` `{"v":"😀"}` `` | – | 2 |
   | :242 | timestamp/escaped text | `` `{"ts":"2026-11-04T10:00:00Z"}` `` | – | 1 |
   | :336 | repeat/escaped key repeats the plain one | `` `{"user":"alice","user":"root"}` `` | – | 1 |
   | :346 | unknown/escaped key matches a declared name | `` `{"v":"a"}` `` | – | 1 |
   | :353 | invisibles/escaped key | `` `{"v":"a","text_invisibles":1}` `` | – | 1 |

   - :159 expects `vStr("é中A")` = `"é中A"`. The mixed hex case covers hex4's three branches.
   - :336: the escaped key comes second, so "root" wins only if it resolves to `user`, with Unknown 0.

   A new test in decode_test.go asserts the escapes are present:
   ```go
   // TestTheEscapeRowsSpellTheirEscapes: each hand-written row named for an escape holds its \u escapes as text, so it
   // reaches appendUnicodeEscape, record's escaped-key path or timestampOf's unescape.
   func TestTheEscapeRowsSpellTheirEscapes(t *testing.T) {
       t.Parallel()
       rows := map[string]int{"string/u escapes": 3, "string/surrogate pair": 2, "timestamp/escaped text": 1,
           "repeat/escaped key repeats the plain one": 1, "unknown/escaped key matches a declared name": 1,
           "invisibles/escaped key": 1}
       for _, c := range handCases {
           if want, ok := rows[c.name]; ok {
               if got := strings.Count(c.in, `\u`); got != want {
                   t.Errorf("%s: %q holds %d \\u escapes; want %d", c.name, c.in, got, want)
               }
               delete(rows, c.name)
           }
       }
       for name := range rows {
           t.Errorf("no hand-written row %q", name)
       }
   }
   ```
   The oracle decodes keys and strings through encoding/json (oracle_test.go:53-77, :149-173), so TestDecodeOracleSelfCheck and TestDecodeConcurrent agree with the new inputs. FuzzDecode's seeds change spelling only; the event fuzz_test.go is not edited.
4. **Proof.** For each entry: `"test":"TestDecodeCases","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>"`, plus `"expect":{"old":"green","new":"red"}`.
   ```json
   {"id":"G2-70-u","file":"internal/event/unescape.go","find":"dst = utf8.AppendRune(dst, r)","replace":"dst = utf8.AppendRune(dst, utf8.RuneError)","subtest":"string/u escapes", …},
   {"id":"G2-70-pair","file":"internal/event/unescape.go","find":"dst = utf8.AppendRune(dst, pair)","replace":"dst = utf8.AppendRune(dst, utf8.RuneError)","subtest":"string/surrogate pair", …},
   {"id":"G2-70-ts","file":"internal/event/decode.go","find":"text = appendUnescaped(buf[:0], text)","replace":"text = append(buf[:0], text...)","subtest":"timestamp/escaped text", …},
   {"id":"G2-70-repeat","file":"internal/event/decode.go","find":"k = appendUnescaped(buf[:0], key)","replace":"k = append(buf[:0], key...)","subtest":"repeat/escaped key repeats the plain one", …},
   {"id":"G2-70-unknown", same plant as G2-70-repeat, "subtest":"unknown/escaped key matches a declared name", …},
   {"id":"G2-70-invisibles", same plant as G2-70-repeat, "subtest":"invisibles/escaped key", …}
   ```
   - There is one control entry per subtest (`"control":true`, the same subtest, `{"old":"green","new":"green"}`).
   - Every find text occurs once in its file (checked with grep).
   - **Coverage**, old and new: `go test -count=1 -run '^TestDecodeCases$/^<level1>$/^<level2 with spaces as _>$' -coverprofile=<f> ./internal/event`. The block's count must go 0 → > 0:

     | Row | Named first statement | Its block starts |
     |---|---|---|
     | u escapes | unescape.go:72 `r := hex4(...)` | line 71 |
     | surrogate pair | unescape.go:77 | line 76 |
     | timestamp | decode.go:276 | line 275 |
     | the three key rows | decode.go:114 | line 113 |

   - The load run is in G2-68's entry: handCases feeds TestDecodeConcurrent's four workers.
5. **Commit:** CD-event-K2.

### G2-71 (K2a)

1. **Verdict: accepted.**
   - gen_test.go:105: ``var invisiblesKeys = []string{event.InvisiblesAttr, `text_invisibles`, `text_invisibles`}``. All three are one string, and have been since a139dbf.
   - base() and attr() leave text_invisibles out (gen_test.go:206, :228), and key-escape escapes only attr()'s picks (:398-405). So no generated payload holds an escaped reserved key.
2. **Kind: K2a, not K2b.** decode.go:114 and :117 are each reached at old by other classes (key-escape; unknownKeys' `\ud83d`). The promised path is the two together, which line coverage cannot isolate, so the proof is the plant alone.
3. **Change:** :105 becomes ``var invisiblesKeys = []string{event.InvisiblesAttr, `text_invisibles`, `text_invisibles`}``. Interpreted: `"\\u0074ext_invisibles"`, `"text_invisible\\u0073"`. The assertion is the shared generator test under G2-72.
4. **Proof.**
   ```json
   {"id":"G2-71","file":"internal/event/decode.go","find":"if string(k) == InvisiblesAttr {","replace":"if string(key) == InvisiblesAttr {","test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
   {"id":"G2-71-control","control":true,"test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
   ```
   - The list's length is unchanged, so the RNG stream and the tally thresholds (decode_test.go:626-629) are unaffected.
5. **Commit:** CD-event-K2.

### G2-72 (K2b)

1. **Verdict: accepted.** `ascii()` shows raw runes where escapes belong:
   - validFor[TString] :27: `"é中"`, `"😀"`.
   - escapedStrings :95-96: `"😀"`, `"￿"`, `"￾"`, `"\U00010000"`, `"\U0010ffff"`, all raw runes.
   - The pick list in gen.value :142-143: `` `é` `` and `` `😀` `` repeat the interpreted "é" and "😀".
2. **Kind: K2b.**
   - No old input reaches the pair branch (unescape.go:77). escapedStrings' other entries are lone or broken pairs; key-escape escapes ASCII only; syntaxBad's pair is a syntax fault.
   - No old input reaches an escaped non-ASCII, non-surrogate rune at :83. The old escapes are `\u0000`, `\u001F`, `t` and control `\u00XX`.
3. **Change** (gen_test.go):
   - validFor[TString]: `` `"é中"` `` → `` `"é中"` ``; `` `"😀"` `` → `` `"😀"` ``. The interpreted raw `"\"é中😀\""` stays.
   - escapedStrings:
     - `` `"😀"` `` → `` `"😀"` ``
     - `` `"￿"` `` (raw) → `` `"￿"` `` (escape)
     - the raw `"￾"` → the escape `￾`
     - `"\U00010000"` → `` `"𐀀"` ``
     - `"\U0010ffff"` → `` `"􏿿"` ``
   - The pick list is hoisted, unchanged in length and order, to a package var, and gen.value calls `g.pick(stringPieces)`. No outcome changes.
     ```go
     var stringPieces = []string{"a", "Z", "0", " ", "é", "中", "😀", `\n`, `\"`, `\\`, `é`, `😀`, `​`, "\x7f", `\/`}
     ```
     The `` `​` `` entry is the existing raw U+200B, kept as it is.
   - A new test in gen_test.go (`slices` and `strings` are already imported):
     ```go
     // TestTheGeneratorsEscapedSpellingsHoldTheirEscapes: no list repeats a value (a lost backslash leaves a twin), every
     // escapedStrings entry holds an escape, and the escaped spellings of é中, 😀 and text_invisibles are \u text.
     func TestTheGeneratorsEscapedSpellingsHoldTheirEscapes(t *testing.T) {
         t.Parallel()
         for name, list := range map[string][]string{"invisiblesKeys": invisiblesKeys, "validFor[TString]": validFor[event.TString],
             "escapedStrings": escapedStrings, "stringPieces": stringPieces} {
             seen := map[string]bool{}
             for _, s := range list {
                 if seen[s] {
                     t.Errorf("%s holds %q twice", name, s)
                 }
                 seen[s] = true
             }
         }
         for _, s := range escapedStrings {
             if !strings.Contains(s, `\`) {
                 t.Errorf("escapedStrings holds %q, which has no escape", s)
             }
         }
         for _, s := range []string{`"é中"`, `"😀"`} {
             if !slices.Contains(validFor[event.TString], s) {
                 t.Errorf("validFor[TString] lacks %s", s)
             }
         }
         for _, s := range []string{`é`, `😀`} {
             if !slices.Contains(stringPieces, s) {
                 t.Errorf("stringPieces lacks %s", s)
             }
         }
         for _, k := range invisiblesKeys[1:] {
             if !strings.Contains(k, `\u`) {
                 t.Errorf("invisiblesKeys holds %q; want an escaped spelling of %s", k, event.InvisiblesAttr)
             }
         }
     }
     ```
4. **Proof.** For each entry: `"test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}`.
   ```json
   {"id":"G2-72-pair","file":"internal/event/unescape.go","find":"dst = utf8.AppendRune(dst, pair)","replace":"dst = utf8.AppendRune(dst, utf8.RuneError)", …},
   {"id":"G2-72-bmp","file":"internal/event/unescape.go","find":"\tdst = utf8.AppendRune(dst, r)\n","replace":"\tif r >= utf8.RuneSelf && r != utf8.RuneError {\n\t\tr = utf8.RuneError\n\t}\n\tdst = utf8.AppendRune(dst, r)\n", …}
   ```
   - The control is G2-71-control (the same test).
   - **Coverage:** `go test -count=1 -run '^TestDecodeDifferential$' -coverprofile=<f> ./internal/event`. The block at unescape.go:76 holding :77 must go 0 → > 0.
   - **Stop rule:** if the old count is > 0, record G2-72 as K2a (plants only) and say so.
5. **Commit:** CD-event-K2.

### G2-73 (K2e)

1. **Verdict: accepted.** None of these calls t.Parallel or says why (testing.md rule 3):
   - object_test.go:83 TestObjectStoredCompact and :104 TestObjectMalformedUTF8AsString;
   - breakit_test.go:72 TestBreakitDecodeEdgesAgreeWithOracle, :94 TestBreakitSyntaxOffsets, :111 TestBreakitDecoderSharedConcurrently;
   - breakit2_test.go:43 TestBreakitGuardObjectCompactRandom;
   - s3_2b_2_alloc_test.go:12 TestDecodeAllocationBudget.

   Each builds its own decoder and schema (testSchemas, NewSchema), and Decoder is safe for concurrent use.
2. **Kind: K2e.**
3. **Change:**
   - `t.Parallel()` becomes the first statement of the first six.
   - TestDecodeAllocationBudget's doc gains a last line: `// Not parallel: testing.AllocsPerRun counts the whole process's allocations.`, the wording of leaf_allocs_test.go:7-8.
   - Go runs parallel top-level tests only after the serial ones finish, so the AllocsPerRun tests are not disturbed.
4. **Proof (K2e):** a stdlib Python check, given in the commit message and the report.
   - For each `file:Test` pair, the line after `func Test…(t *testing.T) {` must be `t.Parallel()`, or the doc comment above it must contain `Not parallel:`.
   - Script:
     ```python
     import sys
     bad = []
     for spec in sys.argv[1:]:
         path, name = spec.rsplit(':', 1)
         src = open(path, encoding='utf-8').read().split('\n')
         i = next(k for k, l in enumerate(src) if l.startswith(f'func {name}('))
         j, doc = i - 1, []
         while j >= 0 and src[j].startswith('//'):
             doc.append(src[j]); j -= 1
         if src[i + 1].strip() != 't.Parallel()' and not any('Not parallel:' in d for d in doc):
             bad.append(spec)
     print(len(bad), 'without t.Parallel or a reason:', bad); sys.exit(1 if bad else 0)
     ```
   - Run on the 7 pairs: old **7**, exit 1; new **0**, exit 0.
   - The load run is in G2-68's entry.
5. **Commit:** CD-event-K2.

### G2-74 (K2a)

1. **Verdict: accepted.**
   - canonical_test.go:761-765: `tok, err := dec.Token(); if err != nil { break }`. The walk stops silently at any token error.
   - mustEncode (:55-62) does not validate the JSON. So malformed output passes "every object's keys ascend bytewise" (:217-222).
   - GitNexus: the only caller is TestMarshalCanonicalKeyOrder.
2. **Kind: K2a.**
3. **Change:** add `"errors"` and `"io"` to the imports, then:
   ```go
   tok, err := dec.Token()
   if errors.Is(err, io.EOF) {
       break
   }
   if err != nil {
       t.Fatalf("the encoding is not one JSON value: %v: %s", err, enc)
   }
   ```
4. **Proof.**
   ```json
   {"id":"G2-74","file":"internal/model/canonical.go","find":"buf.WriteByte(':')","replace":"buf.WriteByte('=')","test":"TestMarshalCanonicalKeyOrder","subtest":"every object's keys ascend bytewise","pkg":"internal/model","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
   {"id":"G2-74-control","control":true,"test":"TestMarshalCanonicalKeyOrder","subtest":"every object's keys ascend bytewise","pkg":"internal/model","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}
   ```
   - The find text occurs once (canonical.go:105).
   - Old: Token fails after the first key and breaks, with no key-order error: **green**. New: **red** (Fatalf).
5. **Commit:** CD-model-K2.

### G2-75 (K2e)

1. **Verdict: accepted.**
   - The guard at expect_pos_test.go:45 checks record counts only. :53-65 then index `Fields[0]`, `Fields[3]` and `Then.Records[0].Fields[0]`.
   - The second subtest's guard (:84) checks the given's fields but not the then's, which :101-102 index. This is the same defect, so it is fixed here too.
2. **Kind: K2e.** On a lowering plant the old test fails by panicking: `--- FAIL` lines are printed, then the panic kills the package's run. mutate reads red either way, so the proof distinguishes panic from failure.
3. **Change.**
   - :45-47 become:
     ```go
     if len(e.Givens) != 2 || len(e.Givens[0].Records) != 1 || len(e.Givens[1].Records) != 1 || len(e.Then.Records) != 1 ||
         len(e.Givens[0].Records[0].Fields) != 1 || len(e.Givens[1].Records[0].Fields) != 4 || len(e.Then.Records[0].Fields) != 3 {
         t.Fatalf("07 block 1: %+v; want two givens and a then, one record each, of 1, 4 and 3 fields", e)
     }
     ```
     The counts are example 07's lines 17-19.
   - :84 appends `|| len(e.Then.Records[0].Fields) != 1`. :85's message becomes "…, a then of one record of one field".
4. **Proof (K2e):**
   - Plant (production, via `go test -overlay`): in lower.go, `for _, f := range r.Fields {` (once, :290) → `for _, f := range r.Fields[:min(len(r.Fields), 1)] {`.
   - Run at the lane base and at this commit: `go test -count=1 -overlay <ov.json> -run '^TestExpectPositions$' ./internal/parser 2>&1`.
     - `grep -c '^panic: runtime error: index out of range'`: old **1**, new **0**.
     - `grep -c -- '--- FAIL: TestExpectPositions/07_block_1'`: **1** at both.
5. **Commit:** CD-parser-K2.

### G2-76 (K2e, live data)

1. **Verdict: accepted.**
   - fragment_test.go:1137-1139 calls `t.Errorf` on readRegister's error and carries on with no entries.
   - checkMarkdown (:576-622) then judges every listed span as unlisted. The register has 16 entries: 8 grammar, 3 lowering, 5 validator.
2. **Kind: K2e** (live data).
3. **Change:** :1138 `t.Errorf(` → `t.Fatalf(`. fragment_test.go is a law path, so the change shows in LAW Δ.
4. **Proof:**
   - In a detached worktree at each revision, change `"rule": "fragments"` (once in tools/allowlists/fragments.json) to `"rule": "fragmentz"`.
   - Run `go test -count=1 -run '^TestEveryQuotedRuleFragmentParses$' ./internal/parser`.
   - Count the output lines that match `fragment_test.go:[0-9]+:`: old **≥ 2** (the register error plus the false faults), new **exactly 1**.
5. **Commit:** CD-parser-K2.

### G2-77: rejected

- The survey's claim holds only for today's three pinned documents.
- `got, want := censusOf(md), pins[file]` (:1163) gives a zero `docCensus` to any document in `fragmentDocs` (:1101) that has no pin. Then :1164 compares 0 with 0 and stays silent, and :1167-1170 are silent if the document has no markers.
- So `got.ruleLike == 0` (:1172) is the only check that fails on a listed document with no fragments and no pin.
- **Reason recorded:** "not redundant: the sole guard for a listed document without a pin."

### G2-78 (K2e; FUZZ_FILE body only)

1. **Verdict: accepted.**
   - fuzz_test.go:18 `files, _ := filepath.Glob(pattern)` drops the error (ErrBadPattern).
   - :20 `if data, err := os.ReadFile(path); err == nil {` silently drops a seed. This is the same defect in the same loop, so it is fixed here too.
2. **Kind: K2e.**
3. **Change** (:17-24):
   ```go
   for _, pattern := range corpus {
       files, err := filepath.Glob(pattern)
       if err != nil {
           f.Fatalf("seed pattern %q: %v", pattern, err)
       }
       for _, path := range files {
           data, err := os.ReadFile(path)
           if err != nil {
               f.Fatal(err)
           }
           f.Add(string(data))
       }
   }
   ```
   - The file name and `FuzzParse` stay.
   - test_fuzz_parser_planted.py copies only `internal` and `docs/language/examples` (:30-32). The eval directories are therefore absent in its world, and Glob returns `nil, nil` for them, not an error. ReadFile reads only files Glob listed, so the driver's outcome is unchanged.
4. **Proof (K2e):** `grep -cE ', _ := filepath\.Glob|err == nil \{' internal/parser/fuzz_test.go`: old **2**, new **0**. The proofs row runs alone at T4 after the hand-back (a coupled file).
5. **Commit:** CD-parser-K2.

### G2-79 (K2e)

1. **Verdict: accepted.**
   - Five tables are built at package init with `pecEdit`:
     - pecContract (:88), pecIncidentRows (:263), pecG1Rows (:303), pecVerbatimRows (:369) in parseerror_contract_test.go;
     - peoOperandRows (parseerror_operands_test.go:15).
   - A broken row panics at :70 during package init, killing every parser test and naming no test.
   - `for i := 0; i+1 < len(pairs); i += 2` (:68) silently drops an odd last argument.
   - GitNexus `impact pecEdit`: LOW, 4 direct.
   - 122 call sites, none with an odd count today (checked by a script over both files).
2. **Kind: K2e.** The change keeps every outcome on the real code and under every production plant, because each row's text is unchanged. So plan §7.2's "changes no outcome" condition holds and the fix is made now.
3. **Change** (mechanical):
   - pecEdit becomes:
     ```go
     // pecEdit applies each (old, new) pair once, in order, to base; it fails the test, naming the pair, when base does
     // not hold an old text or the texts are odd in number (a broken row).
     func pecEdit(t testing.TB, base string, pairs ...string) string {
         t.Helper()
         if len(pairs)%2 != 0 {
             t.Fatalf("pecEdit: %d texts, not (old, new) pairs; the last is %q", len(pairs), pairs[len(pairs)-1])
         }
         for i := 0; i < len(pairs); i += 2 {
             if !strings.Contains(base, pairs[i]) {
                 t.Fatalf("pecEdit: the program does not hold %q", pairs[i])
             }
             base = strings.Replace(base, pairs[i], pairs[i+1], 1)
         }
         return base
     }
     ```
     The panic and its `//nolint:forbidigo` go, so the LAW Δ exemption count drops by one.
   - Each `var X = []pecRow{ … }` becomes `func X(t testing.TB) []pecRow {\n\treturn []pecRow{ … }\n}`, with the same name, so nothing is renamed. gofmt re-indents the rows.
   - Every `pecEdit(` call becomes `pecEdit(t, `. This includes the in-test calls at :347-348 and in TestParseErrorFirstMatchAcrossGroups.
   - The uses become `range pecContract(t)` (:249), `pecIncidentRows(t)` (:288), `pecG1Rows(t)` (:336), `pecVerbatimRows(t)` (:389), `slices.Concat(pecContract(t), pecIncidentRows(t), pecG1Rows(t), pecVerbatimRows(t))` (:464) and `peoOperandRows(t)` (operands :132).
   - Review check: `git diff -w` must show only these edits.
4. **Proof (K2e)**, three checks:
   - `grep -cE '^var [A-Za-z]+ = \[\]pecRow\{'` on the two files: old **4 + 1**, new **0 + 0**.
   - `grep -c 'panic(' internal/parser/parseerror_contract_test.go`: old **1**, new **0**.
   - `grep -ohE 'pecEdit\([a-zA-Z]+' <both files> | sort | uniq -c`: old 0 occurrences of `pecEdit(t`; new 123 = every one (122 calls plus the definition).
5. **Commit:** CD-parser-K2, after CD-parser-K1, which edits the same file.

### G2-80: rejected

- vsc_known_issues_test.go:188 and :205 use `err.(*Error)`.
- Parse's doc says "The error, if any, is a *Error" (parse.go:58). The type assertion pins exactly that. errors.As would also accept a wrapped *Error, which loosens the contract.
- The assertion cannot pass on a wrong error, so the test tests what it claims.
- **Reason recorded:** "the assertion is the stricter, documented contract; errors.As weakens it."

### G2-134 (K2e)

1. **Verdict: accepted.**
   - None of internal/text/text_test.go's seven tests calls t.Parallel: :20, :29, :40, :96, :130, :157, :226.
   - internal/plan/expect_test.go:35 TestBuildCarriesExpects is the only test of the four in its file without it. :154, :183 and :197 call it.
   - No shared mutable state: fixture() opens its own file, and peExample parses afresh.
2. **Kind: K2e.**
3. **Change:**
   - `t.Parallel()` becomes the first statement of TestUnicodeTablesAgree, TestIngestFlagsReferenceExample, TestIngestFlagsClasses, TestIngestFlagsReadsOnlyNonNullStrings, TestIngestFlagsInvalidUTF8 and TestIngestFlagsMatchesOracle.
   - TestIngestFlagsAllocatesNothing's doc (:156) gains `// Not parallel: testing.AllocsPerRun counts the whole process's allocations.`
   - TestBuildCarriesExpects gets `t.Parallel()`.
4. **Proof:** G2-73's script on the 7 text pairs plus `internal/plan/expect_test.go:TestBuildCarriesExpects`: old **8**, new **0**.
5. **Commits:**
   - CD-text-K2: the text half.
   - **The plan half lands in the plan package's K2 commit, which CD2's note owns.** The file is the same lane's, so no other owner is involved; the two notes must agree.

---

### Commit plan (CD1: 7 commits; plus the plan half of G2-134 inside CD2's plan-K2 commit)

1. **CD-parser-K1:** G10-16.
   - Proof: the K1 batch (2 entries red), each failing line quoted; T2.
2. **CD-parser-K2:** G2-75, G2-76, G2-78, G2-79.
   - Proofs: G2-75's overlay panic count; G2-76's live-data count; G2-78's grep; G2-79's three greps and its `git diff -w` check.
   - LAW Δ: fragment_test.go and parseerror_contract_test.go; exemptions −1.
   - The proofs row runs alone at T4 after the hand-back (FUZZ_FILE).
3. **CD-event-bench:** G2-69.
   - Proof: the coverage of unescape.go:76/77, 0 → > 0; the alloc pin unchanged.
4. **CD-event-K2:** G2-68, G2-70, G2-71, G2-72, G2-73.
   - Proofs: the `--expect` entries (G2-68 timeout/red; G2-70 ×6 green/red with K2b coverage; G2-71 and G2-72 ×2 green/red; their controls); G2-72's coverage; G2-73's parallel check; the load run (50/50).
5. **CD-clock-K2:** G2-65, G2-66, G2-67.
   - Proof: three `--expect` entries green/red plus three controls.
6. **CD-text-K2:** G2-134 (text).
   - Proof: the parallel check.
7. **CD-model-K2:** G2-74.
   - Proof: an `--expect` subtest entry green/red plus its control.

There is no K2-del, because G2-77 is rejected. The batch goes to `docs/sprints/s1-1c-7-plants/CD.json` with the coordinator's proof-docs commit, together with G2-73's script text.

### Choices made (for "chosen by the design note")

- **G10-16, the fallback's Fix text.**
  - Input: `parseError("t.cep", nil, errors.New("boom"))` and `lexError("t.cep", "", errors.New("boom"))`.
  - Candidates:
    - (a) `mk`, which gives errors.go:38's catalogue line, the same as every other E-SYN-001;
    - (b) `Error()` omits ". Fix: …" when Fix is empty;
    - (c) a new fix text.
  - **Choice: (a).**
    - The catalogue (errors.md, a law doc whose change needs the owner) stays the only source, verbatim. fixtext_test pins the map to it.
    - (b) changes every caller's format and gives one code two shapes.
    - (c) is new user text outside the catalogue.
  - New text: `t.cep:1:1: E-SYN-001: boom. Fix: the message names the canonical form for the construct it found.`
  - Dry run (`git grep`): no test pins the old empty fix or full text of these branches.
    - `"boom"` appears only in parseerror_contract_test.go:254-256, which checks Code, Line, Col and Msg.
    - deploy_test.go:64 compares `d.Fix` for Deploy's diagnostics, which never reach these branches.
    - `Fix ==` and `.Fix !=` pins elsewhere (lowering_test.go:61, vsc_known_issues_test.go:192, validate's docsync and static tests) are other codes.
- **Test-input spellings** (G2-69 to G2-72): the escapes in the tables above. Where the original was lost (since a139dbf), I chose escapes that reach the named branch and differ from the breakit payloads where cheap.
- **G2-65's thresholds:** ≥ 100 per kind and for ties, with a stop rule.

### Couplings and risks

- **Law paths (LAW Δ):**
  - parseerror_contract_test.go (G10-16, G2-79) and fragment_test.go (G2-76) are body edits. Both are in the `fragments` instrument's `code` list.
  - G2-79 removes a PERMANENT exemption.
- **FUZZ_FILE** internal/parser/fuzz_test.go: body only (G2-78); the name and `FuzzParse` are kept.
  - internal/event/fuzz_test.go is not edited. Its seeds include handCases, which G2-70 respells.
- **R4:** G2-69 is its own commit. No benchmark name changes. BenchmarkAppendUnescaped's measure moves, and no K1h comparison relies on it.
- **R5:** the only production change is G10-16, in the parser, at deploy time (GitNexus: Deploy and tools/shape only).
  - **Warning:** `impact lexError` reads CRITICAL by Parse's fan-out. The edited lines are two defensive branches that Parse cannot reach.
  - No event, clock or model production code changes. The plants there are plants, not commits.
- **Frame tables and line-anchored mutants:** none in these files. G10-16 keeps line counts.
- **Other owners:** none needed. The plan half of G2-134 belongs to the same lane, in CD2's plan commit.
- **Pre-mortem (the-fool), riskiest first:**
  1. G2-66's discard model differs from the clock → a false red. Stop rule; the model mirrors Schedule (replace), Cancel (only on true) and CancelAll.
  2. G2-79's mechanical edit alters a row → caught by `git diff -w` and the unchanged row outcomes. A build error is caught at once.
  3. G2-65's thresholds → stop rule.
  4. G2-68's plant does not overflow → stop rule; old reads red instead of timeout.
  5. G2-72's old coverage is not 0 → it falls back to K2a, recorded.
  6. A G2-70 row fails on the real code → it would be a real decoder defect; stop the line.

### Carried items

- **To S1.1c-8 (rename):** G2-65's `TestBreakit_GranularityScenariosAreNonTrivial` → `TestRandomWalkScenariosExerciseEveryCallback` (the assertions behind it are under G2-65 above).
- **To the later comments work (S1.1c-10), not a survey finding:** internal/model/model.go:199, `Source Source // plain and window`.
  - It is wrong today: a window body sets `Window.Source` and leaves `Body.Source` empty (lower.go:187, :205; model.go:268).
  - No fix in this chunk touches it or makes it newly false, so it is not fixed here as K6.
- **Observations, not findings, unchanged:**
  - validFor[TString]'s `"​"` (gen_test.go:28), the pick list's `​` (:143) and tsForms' `"２026-…"` (:82) hold raw runes inside backticks. They may also have been escapes, but no earlier text exists to say so.
  - As raw runes they are still valid inputs, and no survey finding names them.

**Files changed by me:** none; the work was read-only. detect_changes does not apply because nothing changed.

Scratch helper used to count call sites (not part of the build): `s17/cd1_pec.py`.


## Revision 1: G2-69 to G2-72 re-emitted byte-exact, with the reviews' corrections

### G2-69 (K2e, bench input, its own commit; R4)

**How this section is written.** ⧵ (U+29F5) stands for one ASCII backslash (byte 0x5C) in the source. Every other character is literal.

1. **Verdict: accepted.**
   - `internal/event/decode_bench_test.go:142` holds a raw 😀 between backticks: bytes `60 f0 9f 98 80 60`. There is no backslash on the line.
   - The comment at :136-138 promises "a surrogate pair written as two escapes". So BenchmarkAppendUnescaped (:149-159) and the appendUnescaped pin (`leaf_allocs_test.go:49-53`) never reach the pair branch at `unescape.go:76-79`.
   - GitNexus `context appendUnicodeEscape`: its one caller is appendEscape.

2. **Kind: K2e (a benchmark input).**

3. **Change.**
   - **Line 142.** Replace the element `` `😀` `` (bytes `60 f0 9f 98 80 60`) with `` `⧵ud83d⧵ude00` ``. The new line is:
     ```go
     	[]byte(`⧵ud83d⧵ude00`),
     ```
     - The element's bytes are `60 5c 75 64 38 33 64 5c 75 64 65 30 30 60`.
     - The input is the 12 bytes `5c 75 64 38 33 64 5c 75 64 65 30 30`: two six-byte escapes.
     - appendUnescaped decodes it to U+1F600 (UTF-8 `f0 9f 98 80`).
   - **New test**, placed directly after BenchmarkAppendUnescaped's closing brace (after :159). It adds no import, so line 142 does not move.
     ```go
     // TestTheSurrogatePairInputIsWrittenAsTwoEscapes: leafEscaped[2] is the pair's two backslash-u escapes as text (12
     // bytes), so BenchmarkAppendUnescaped and the appendUnescaped pin reach the pair's branch; it decodes to U+1F600.
     func TestTheSurrogatePairInputIsWrittenAsTwoEscapes(t *testing.T) {
     	t.Parallel()
     	if in := string(leafEscaped[2]); in != `⧵ud83d⧵ude00` {
     		t.Fatalf("leafEscaped[2] = %q; want the pair's two escapes as text", in)
     	}
     	if got := string(appendUnescaped(nil, leafEscaped[2])); got != string(rune(0x1F600)) {
     		t.Fatalf("appendUnescaped(%q) = %q; want U+1F600", leafEscaped[2], got)
     	}
     }
     ```
     - The literal `` `⧵ud83d⧵ude00` `` is the same 12 input bytes.
     - Against the old raw 😀 the first check fails. Against the new input both checks pass.
   - **Unchanged:** benchmark names and bodies (R4). BenchmarkAppendUnescaped's measured work grows by one pair decode, which is why this is its own commit.
   - **Builder instruction, before any proof:**
     1. In the two code blocks above, replace each ⧵ with an ASCII backslash.
     2. Prove the bytes with this command (it contains no backslash):
        ```
        python3.15 -I -c "import sys; f=sys.argv[1]; ls=open(f,'rb').read().splitlines(); p=bytes([92,117]); print(f, 'total', sum(l.count(p) for l in ls), *[n+':'+str(ls[int(n)-1].count(p)) for n in sys.argv[2:]])" internal/event/decode_bench_test.go 142
        ```
        It must print `total 8 142:2`. The old file prints `total 4 142:0`.
     3. Check `python3.15 -I -c "import sys; print(max(open(sys.argv[1],'rb').read().splitlines()[141]))" internal/event/decode_bench_test.go`. It must print a number below 128, meaning no raw rune is left on :142.
     4. Optionally, `sed -n 142p internal/event/decode_bench_test.go | od -c` must show the characters backslash, u, d, 8, 3, d twice.

4. **Proof (K2e).**
   - Run at the lane base and at this commit:
     ```
     go test -count=1 -run '^$' -bench '^BenchmarkAppendUnescaped$' -benchtime=1x -coverprofile=<f> ./internal/event
     ```
   - In the profile, read the line for the `unescape.go` block that starts on line 76 (after the `{`) and holds :77, `dst = utf8.AppendRune(dst, pair)`, the first statement of the pair branch. Its count must be **0** at the old revision and **> 0** at the new one.
   - **Stop rule:** TestDecodeLeafAllocations' pin "appendUnescaped over the six inputs into one [128]byte" must stay 0, since the 4-byte result fits the reused buffer. If it moves, stop and report.

5. **Commit:** CD-event-bench.

### G2-70 (K2b)

**How this section is written.** ⧵ stands for one ASCII backslash (0x5C) in the source.

1. **Verdict: accepted.** Python `ascii()` on `internal/event/decode_test.go` shows no backslash in any of the six inputs. A count of 0x5C 0x75 pairs is 0 on each line.
   - :159 `{"v":"é中A"}` is raw.
   - :160 `{"v":"😀"}` is raw.
   - :242, :336, :346 and :353 are plain ASCII with no escape.

   So the rows never reach appendUnicodeEscape (`unescape.go:71`), record's unescape (`decode.go:113-115`) or timestampOf's unescape (`decode.go:275-277`). They have been this way since a139dbf. TestDecodeCases (:399) runs each row as a parallel subtest.

2. **Kind: K2b.**

3. **Change.** Only the raw-string input of each row changes. The row name, schema and expected values stay byte-for-byte as they are.

   | Line | Row | New input (⧵ notation) | New input bytes (hex) | Decoded | ⧵u count |
   |---|---|---|---|---|---|
   | :159 | `string/u escapes` | `` `{"v":"⧵u00e9⧵u4E2D⧵u0041"}` `` | `7b 22 76 22 3a 22 5c 75 30 30 65 39 5c 75 34 45 32 44 5c 75 30 30 34 31 22 7d` (26 bytes) | v = U+00E9 U+4E2D U+0041 | 3 |
   | :160 | `string/surrogate pair` | `` `{"v":"⧵ud83d⧵ude00"}` `` | `7b 22 76 22 3a 22 5c 75 64 38 33 64 5c 75 64 65 30 30 22 7d` (20 bytes) | v = U+1F600 | 2 |
   | :242 | `timestamp/escaped text` | `` `{"ts":"2026-11-04T10⧵u003a00:00Z"}` `` | `7b 22 74 73 22 3a 22 32 30 32 36 2d 31 31 2d 30 34 54 31 30 5c 75 30 30 33 61 30 30 3a 30 30 5a 22 7d` (34 bytes) | ts = "2026-11-04T10:00:00Z" (U+003A restored) | 1 |
   | :336 | `repeat/escaped key repeats the plain one` | `` `{"user":"alice","⧵u0075ser":"root"}` `` | `7b 22 75 73 65 72 22 3a 22 61 6c 69 63 65 22 2c 22 5c 75 30 30 37 35 73 65 72 22 3a 22 72 6f 6f 74 22 7d` (35 bytes) | 2nd key = U+0075 U+0073 U+0065 U+0072 ("user") | 1 |
   | :346 | `unknown/escaped key matches a declared name` | `` `{"⧵u0076":"a"}` `` | `7b 22 5c 75 30 30 37 36 22 3a 22 61 22 7d` (14 bytes) | key = U+0076 ("v") | 1 |
   | :353 | `invisibles/escaped key` | `` `{"v":"a","text⧵u005finvisibles":1}` `` | `7b 22 76 22 3a 22 61 22 2c 22 74 65 78 74 5c 75 30 30 35 66 69 6e 76 69 73 69 62 6c 65 73 22 3a 31 7d` (34 bytes) | 2nd key = "text_invisibles" (U+005F restored) | 1 |

   - **What each row now proves:**
     - :159 expects `vStr("é中A")`, which stays as written: U+00E9 U+4E2D U+0041 as raw UTF-8 `c3 a9 e4 b8 ad 41`. The mixed hex case reaches all three branches of hex4.
     - :160 expects `vStr("😀")`, unchanged: U+1F600.
     - :336 puts the escaped key second. "root" wins only if that key resolves to `user`, and Unknown stays 0.
     - :346 is required `v`. If the escaped key did not resolve, the row would fail as a missing attribute.
     - :353 must set Reserved, with Unknown 0.
   - **The full new lines (⧵ notation):**
     ```go
     	ok("string/u escapes", "S", `{"v":"⧵u00e9⧵u4E2D⧵u0041"}`, vStr("é中A")),
     	ok("string/surrogate pair", "S", `{"v":"⧵ud83d⧵ude00"}`, vStr("😀")),
     	ok("timestamp/escaped text", "T", `{"ts":"2026-11-04T10⧵u003a00:00Z"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
     	ok("repeat/escaped key repeats the plain one", "U", `{"user":"alice","⧵u0075ser":"root"}`, vStr("root")),
     	ok("unknown/escaped key matches a declared name", "S", `{"⧵u0076":"a"}`, vStr("a")),
     	ok("invisibles/escaped key", "S", `{"v":"a","text⧵u005finvisibles":1}`, vStr("a")).withReserved(),
     ```
     These go at lines 159, 160, 242, 336, 346 and 353 in that order. For :159 and :160, edit only the input element and keep the existing `vStr(...)` bytes. Do not retype the raw runes.
   - **New test**, placed directly after TestDecodeCases' closing brace (after :436), so the row lines do not move. `strings` is already imported (:146).
     ```go
     // TestTheEscapeRowsSpellTheirEscapes: each hand-written row named for an escape holds its backslash-u escapes as
     // text, so it reaches appendUnicodeEscape, record's escaped-key path or timestampOf's unescape.
     func TestTheEscapeRowsSpellTheirEscapes(t *testing.T) {
     	t.Parallel()
     	rows := map[string]int{"string/u escapes": 3, "string/surrogate pair": 2, "timestamp/escaped text": 1,
     		"repeat/escaped key repeats the plain one": 1, "unknown/escaped key matches a declared name": 1,
     		"invisibles/escaped key": 1}
     	for _, c := range handCases {
     		if want, ok := rows[c.name]; ok {
     			if got := strings.Count(c.in, `⧵u`); got != want {
     				t.Errorf("%s: %q holds %d backslash-u escapes; want %d", c.name, c.in, got, want)
     			}
     			delete(rows, c.name)
     		}
     	}
     	for name := range rows {
     		t.Errorf("no hand-written row %q", name)
     	}
     }
     ```
     - The literal `` `⧵u` `` is the bytes `5c 75`.
     - On the old rows every count is 0, so the test reports 6 errors. On the new rows the counts are 3, 2, 1, 1, 1 and 1, and it passes.
   - **Other tests are unaffected.** The oracle decodes keys and strings through encoding/json (`oracle_test.go:53-77`, :149-173), so TestDecodeOracleSelfCheck and TestDecodeConcurrent agree with the new inputs. FuzzDecode's seeds change spelling only. `internal/event/fuzz_test.go` (FUZZ_FILE) is not edited.
   - **Builder instruction, before any proof:**
     1. In the code blocks above, replace each ⧵ with an ASCII backslash.
     2. Run:
        ```
        python3.15 -I -c "import sys; f=sys.argv[1]; ls=open(f,'rb').read().splitlines(); p=bytes([92,117]); print(f, 'total', sum(l.count(p) for l in ls), *[n+':'+str(ls[int(n)-1].count(p)) for n in sys.argv[2:]])" internal/event/decode_test.go 159 160 242 336 346 353
        ```
        It must print `total 21 159:3 160:2 242:1 336:1 346:1 353:1`. The old file prints `total 11` with every line at 0.
     3. Optionally, `sed -n 160p internal/event/decode_test.go | od -c` shows backslash-u-d-8-3-d inside the input.

4. **Proof.** Every entry carries `"test":"TestDecodeCases","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>"`. The find and replace texts contain no backslash.
   ```json
   [{"id":"G2-70-u","file":"internal/event/unescape.go","find":"dst = utf8.AppendRune(dst, r)","replace":"dst = utf8.AppendRune(dst, utf8.RuneError)","test":"TestDecodeCases","subtest":"string/u escapes","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-70-pair","file":"internal/event/unescape.go","find":"dst = utf8.AppendRune(dst, pair)","replace":"dst = utf8.AppendRune(dst, utf8.RuneError)","test":"TestDecodeCases","subtest":"string/surrogate pair","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-70-ts","file":"internal/event/decode.go","find":"text = appendUnescaped(buf[:0], text)","replace":"text = append(buf[:0], text...)","test":"TestDecodeCases","subtest":"timestamp/escaped text","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-70-repeat","file":"internal/event/decode.go","find":"k = appendUnescaped(buf[:0], key)","replace":"k = append(buf[:0], key...)","test":"TestDecodeCases","subtest":"repeat/escaped key repeats the plain one","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-70-unknown","file":"internal/event/decode.go","find":"k = appendUnescaped(buf[:0], key)","replace":"k = append(buf[:0], key...)","test":"TestDecodeCases","subtest":"unknown/escaped key matches a declared name","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-70-invisibles","file":"internal/event/decode.go","find":"k = appendUnescaped(buf[:0], key)","replace":"k = append(buf[:0], key...)","test":"TestDecodeCases","subtest":"invisibles/escaped key","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-70-u-control","control":true,"test":"TestDecodeCases","subtest":"string/u escapes","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}},
    {"id":"G2-70-pair-control","control":true,"test":"TestDecodeCases","subtest":"string/surrogate pair","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}},
    {"id":"G2-70-ts-control","control":true,"test":"TestDecodeCases","subtest":"timestamp/escaped text","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}},
    {"id":"G2-70-repeat-control","control":true,"test":"TestDecodeCases","subtest":"repeat/escaped key repeats the plain one","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}},
    {"id":"G2-70-unknown-control","control":true,"test":"TestDecodeCases","subtest":"unknown/escaped key matches a declared name","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}},
    {"id":"G2-70-invisibles-control","control":true,"test":"TestDecodeCases","subtest":"invisibles/escaped key","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}]
   ```
   - Each find text occurs exactly once in its file (checked with `grep -c`: 1 each).
   - **Why each plant reads green, then red:**
     - Old: none of the rows holds an escape, so each plant's line never runs for its subtest.
     - New, per plant:
       - u: `⧵u0041` and the others decode to U+FFFD, so the value mismatches.
       - pair: U+FFFD replaces U+1F600.
       - ts: the escape stays in the text, so RFC 3339 parsing fails and the row is rejected.
       - repeat: `⧵u0075ser` becomes an unknown key, so the result is "alice" with Unknown 1.
       - unknown: `v` is missing, so the row is rejected.
       - invisibles: Unknown 1 and Reserved false.
   - **Coverage, old and new.** Run `go test -count=1 -run '^TestDecodeCases$/^<level 1>$/^<level 2, spaces as _>$' -coverprofile=<f> ./internal/event`; for example `-run '^TestDecodeCases$/^string$/^u_escapes$'`. The named block's count must go from 0 to more than 0:

     | Row | Named first statement | Its block starts on |
     |---|---|---|
     | u escapes | `unescape.go:72`, `r := hex4(in[i+2 : i+6])` | line 71 |
     | surrogate pair | `unescape.go:77` | line 76 |
     | timestamp | `decode.go:276` | line 275 |
     | the three key rows | `decode.go:114` | line 113 |

   - **Load run.** handCases feeds TestDecodeConcurrent's four goroutines, so:
     ```
     python3.15 -I -B gate/bench/load.py -- -race -count=50 -run '^(TestBreakitDecoderSharedConcurrently|TestDecodeConcurrent|TestDecodeCases)$' ./internal/event
     ```
     Each test must report 50/50.

5. **Commit:** CD-event-K2.

### G2-71 (K2a)

**How this section is written.** ⧵ stands for one ASCII backslash (0x5C) in the source.

1. **Verdict: accepted.**
   - `internal/event/gen_test.go:105` is ``var invisiblesKeys = []string{event.InvisiblesAttr, `text_invisibles`, `text_invisibles`}``. All three entries are the same 15 ASCII bytes. The line has no backslash (count 0), and has been this way since a139dbf.
   - base() and attr() leave text_invisibles out (:206, :228). The key-escape class escapes only attr()'s picks (:398-405). So no generated payload holds an escaped reserved key.

2. **Kind: K2a, not K2b.**
   - `decode.go:114` and :117 are each reached at the old revision by other classes: key-escape, and unknownKeys' `⧵ud83d`.
   - The promised path is the two together, which line coverage cannot isolate, so the plant alone is the proof.

3. **Change.** Line 105 becomes the following. The line holds no other backslash, so replacing the whole line is safe.
   ```go
   var invisiblesKeys = []string{event.InvisiblesAttr, `⧵u0074ext_invisibles`, `text_invisible⧵u0073`}
   ```

   | Entry | ⧵ notation | Bytes (hex, 20 bytes each) | Decoded |
   |---|---|---|---|
   | 2nd | `⧵u0074ext_invisibles` | `5c 75 30 30 37 34 65 78 74 5f 69 6e 76 69 73 69 62 6c 65 73` | "text_invisibles" (U+0074 U+0065 U+0078 U+0074 U+005F U+0069 U+006E U+0076 U+0069 U+0073 U+0069 U+0062 U+006C U+0065 U+0073) |
   | 3rd | `text_invisible⧵u0073` | `74 65 78 74 5f 69 6e 76 69 73 69 62 6c 65 5c 75 30 30 37 33` | the same "text_invisibles" |

   - The list keeps its length of 3, so the RNG stream and the tally thresholds (`decode_test.go:626-629`) are unaffected.
   - The assertion that these entries hold ⧵u is the shared test TestTheGeneratorsEscapedSpellingsHoldTheirEscapes, under G2-72.
   - **Builder instruction, before any proof:** replace each ⧵ with an ASCII backslash, then run G2-72's gen_test.go byte check. Line 105 must read `105:2`.

4. **Proof.**
   ```json
   [{"id":"G2-71","file":"internal/event/decode.go","find":"if string(k) == InvisiblesAttr {","replace":"if string(key) == InvisiblesAttr {","test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-71-control","control":true,"test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}]
   ```
   - The find text occurs once (`decode.go:116`).
   - With the plant, an escaped reserved key is no longer recognised as reserved. The oracle still says Reserved.
   - Old reads green, because no payload holds an escaped reserved key.
   - New reads red. The invisibles class is about 1/28 of 7,500 payloads, and two of its three picks are now escaped.

5. **Commit:** CD-event-K2.

### G2-72 (K2b)

**How this section is written.** ⧵ stands for one ASCII backslash (0x5C) in the source. Wherever an element sits on a line with other escapes, the builder edits only the named element, matched by its old bytes, and never retypes the rest of the line.

1. **Verdict: accepted.** `ascii()` on gen_test.go shows raw runes where escapes belong:
   - **validFor[TString], :27:** the backtick elements `` `"é中"` `` (bytes `60 22 c3 a9 e4 b8 ad 22 60`) and `` `"😀"` `` (`60 22 f0 9f 98 80 22 60`).
   - **escapedStrings, :95:** `` `"😀"` `` (`60 22 f0 9f 98 80 22 60`).
   - **escapedStrings, :96:** four elements each holding one raw rune: U+FFFF (`60 22 ef bf bf 22 60`), U+FFFE (`60 22 ef bf be 22 60`), U+10000 (`60 22 f0 90 80 80 22 60`) and U+10FFFF (`60 22 f4 8f bf bf 22 60`).
   - **The pick list in gen.value, :142-143:** the backtick elements `` `é` `` (`60 c3 a9 60`) and `` `😀` `` (`60 f0 9f 98 80 60`) equal the interpreted "é" and "😀" before them.

   The current pair counts are :27 0, :95 5, :96 3, :97 8, :105 0, :142 0 and :143 0, with a file total of 29.

2. **Kind: K2b.**
   - No old input reaches the pair branch (`unescape.go:77`):
     - escapedStrings' other entries are lone or broken pairs;
     - key-escape escapes ASCII only;
     - syntaxBad's pair is a syntax fault.
   - No old input reaches `unescape.go:83` with an escaped non-ASCII rune that is not a surrogate. The old escapes are `⧵u0000`, `⧵u001F`, `⧵u0074`, `⧵u007a` and the control class's `⧵u00XX` below 0x20; lone surrogates become U+FFFD.

3. **Change** (gen_test.go).

   **(a) validFor[TString], line 27.** Two elements change; the rest of the line, including the interpreted element holding é中😀, stays untouched.

   | Old element | Old bytes | New element (⧵ notation) | New bytes | Decoded |
   |---|---|---|---|---|
   | `` `"é中"` `` | `60 22 c3 a9 e4 b8 ad 22 60` | `` `"⧵u00e9⧵u4e2d"` `` | `60 22 5c 75 30 30 65 39 5c 75 34 65 32 64 22 60` | U+00E9 U+4E2D |
   | `` `"😀"` `` | `60 22 f0 9f 98 80 22 60` | `` `"⧵ud83d⧵ude00"` `` | `60 22 5c 75 64 38 33 64 5c 75 64 65 30 30 22 60` | U+1F600 |

   **(b) escapedStrings.**

   | Line | Old element (bytes) | New element (⧵ notation) | New bytes | Decoded |
   |---|---|---|---|---|
   | :95 | `60 22 f0 9f 98 80 22 60` | `` `"⧵ud83d⧵ude00"` `` | `60 22 5c 75 64 38 33 64 5c 75 64 65 30 30 22 60` | U+1F600 |
   | :96 | `60 22 ef bf bf 22 60` | `` `"⧵uffff"` `` | `60 22 5c 75 66 66 66 66 22 60` | U+FFFF |
   | :96 | `60 22 ef bf be 22 60` | `` `"⧵ufffe"` `` | `60 22 5c 75 66 66 66 65 22 60` | U+FFFE |
   | :96 | `60 22 f0 90 80 80 22 60` | `` `"⧵ud800⧵udc00"` `` | `60 22 5c 75 64 38 30 30 5c 75 64 63 30 30 22 60` | U+10000 |
   | :96 | `60 22 f4 8f bf bf 22 60` | `` `"⧵udbff⧵udfff"` `` | `60 22 5c 75 64 62 66 66 5c 75 64 66 66 66 22 60` | U+10FFFF |

   - Each value decodes the same through encoding/json (the oracle) and through hex4 and utf16.DecodeRune (the decoder).

   **(c) The pick list, hoisted.**
   - Insert a new package var directly after line 121 (`var validWS = …`), so lines 1-121 do not move.
   - Move the slice literal from `sb.WriteString(g.pick([]string{ … }))` (old :142-143) by cut and paste, not by retyping. Then change its 11th and 12th elements.
   - The statement in gen.value becomes `sb.WriteString(g.pick(stringPieces))`.
   - The final var, in ⧵ notation:
     ```go

     // stringPieces are the pieces gen.value builds a random string value from: raw runes, and escapes written as text.
     var stringPieces = []string{"a", "Z", "0", " ", "é", "中", "😀", `⧵n`, `⧵"`, `⧵⧵`, `⧵u00e9`, `⧵ud83d⧵ude00`, <the old raw U+200B element, bytes 60 e2 80 8b 60, kept as moved>, "⧵x7f", `⧵/`}
     ```
   - Only elements 11 and 12 differ from the old literal:

     | Element | Old bytes | New (⧵ notation) | New bytes | Decoded |
     |---|---|---|---|---|
     | 11 | `60 c3 a9 60` | `` `⧵u00e9` `` | `60 5c 75 30 30 65 39 60` | U+00E9 |
     | 12 | `60 f0 9f 98 80 60` | `` `⧵ud83d⧵ude00` `` | `60 5c 75 64 38 33 64 5c 75 64 65 30 30 60` | U+1F600 |

   - Elements 8, 9, 10, 14 and 15 are the old ones moved unchanged: `⧵n`, `⧵"`, `⧵⧵`, "⧵x7f", `⧵/`. The list's length (15) and order are unchanged, so gen.value consumes the RNG exactly as before and no outcome changes.

   **(d) New test**, appended after payload() at the end of gen_test.go. `slices`, `strings` and `event` are already imported.
   ```go
   // TestTheGeneratorsEscapedSpellingsHoldTheirEscapes: no list repeats a value (a lost backslash leaves a twin), every
   // escapedStrings entry holds a backslash, and the escaped spellings of é中, 😀 and text_invisibles are written as
   // backslash-u text.
   func TestTheGeneratorsEscapedSpellingsHoldTheirEscapes(t *testing.T) {
   	t.Parallel()
   	lists := map[string][]string{"invisiblesKeys": invisiblesKeys, "validFor[TString]": validFor[event.TString],
   		"escapedStrings": escapedStrings, "stringPieces": stringPieces}
   	for name, list := range lists {
   		seen := map[string]bool{}
   		for _, s := range list {
   			if seen[s] {
   				t.Errorf("%s holds %q twice", name, s)
   			}
   			seen[s] = true
   		}
   	}
   	for _, s := range escapedStrings {
   		if !strings.Contains(s, `⧵`) {
   			t.Errorf("escapedStrings holds %q, which has no escape", s)
   		}
   	}
   	for _, s := range []string{`"⧵u00e9⧵u4e2d"`, `"⧵ud83d⧵ude00"`} {
   		if !slices.Contains(validFor[event.TString], s) {
   			t.Errorf("validFor[TString] lacks %s", s)
   		}
   	}
   	for _, s := range []string{`⧵u00e9`, `⧵ud83d⧵ude00`} {
   		if !slices.Contains(stringPieces, s) {
   			t.Errorf("stringPieces lacks %s", s)
   		}
   	}
   	for _, k := range invisiblesKeys[1:] {
   		if !strings.Contains(k, `⧵u`) {
   			t.Errorf("invisiblesKeys holds %q; want an escaped spelling of %s", k, event.InvisiblesAttr)
   		}
   	}
   }
   ```
   - `` `⧵` `` is the single byte `5c`.
   - `` `"⧵u00e9⧵u4e2d"` `` and `` `"⧵ud83d⧵ude00"` `` are exactly the new validFor elements' bytes, without the outer backticks.
   - `` `⧵u00e9` `` and `` `⧵ud83d⧵ude00` `` are the new stringPieces elements 11 and 12.

   **Re-check: the test applied to the old lists fails on each of these:**
   - **invisiblesKeys** `["text_invisibles", "text_invisibles", "text_invisibles"]`: two "holds twice" errors, and two "want an escaped spelling" errors.
   - **validFor[TString]:** no duplicate. Old `"é中"` differs from the interpreted `"é中😀"`. But both "lacks" errors fire.
   - **escapedStrings:** 5 "has no escape" errors, one for each raw entry; no duplicate.
   - **stringPieces** (the old literal): "é" (`c3 a9`) and `é` (`c3 a9`) are equal, and so are "😀" and `😀`, giving two "holds twice" errors. Both "lacks" errors fire too.

   **Re-check: the new lists pass:**
   - invisiblesKeys holds 3 distinct entries, both escaped ones containing `5c 75`.
   - validFor[TString] is distinct: `""`, `"alice"`, the simple-escapes element, `"⧵u00e9⧵u4e2d"`, `"⧵ud83d⧵ude00"`, the interpreted é中😀 element, `"text_invisibles"`, `"null"`, `"5"`, the raw U+200B element, `"{}"`, `"⧵u0000"`.
   - escapedStrings is distinct, and every entry holds `5c`:
     - new `"⧵ud83d⧵ude00"` differs from the existing `"⧵ud83d⧵⧵ude00"`;
     - new `"⧵ud800⧵udc00"` differs from the existing `"⧵udc00⧵ud800"`.
   - stringPieces is distinct: `⧵u00e9` differs from "é", `⧵ud83d⧵ude00` differs from "😀", and "⧵x7f" is the single byte 0x7F.

   **Builder instruction, before any proof:**
   1. In every code block of this section and of G2-71, replace each ⧵ with an ASCII backslash.
   2. Make the element edits by matching the old bytes in the tables above.
   3. Run, with the stringPieces declaration's line number from `grep -n 'var stringPieces' internal/event/gen_test.go`:
      ```
      python3.15 -I -c "import sys; f=sys.argv[1]; ls=open(f,'rb').read().splitlines(); p=bytes([92,117]); print(f, 'total', sum(l.count(p) for l in ls), *[n+':'+str(ls[int(n)-1].count(p)) for n in sys.argv[2:]])" internal/event/gen_test.go 27 95 96 97 105 <stringPieces line>
      ```
      After G2-71 and G2-72, which share one commit, it must print `total 54 27:4 95:7 96:9 97:8 105:2 <line>:3`. The old file prints `total 29 27:0 95:5 96:3 97:8 105:0`.
      - The total of 54 is 29, plus 4 on :27, 2 on :95, 6 on :96, 2 on :105 and 3 on the stringPieces line, plus 8 in the new test.
   4. Run `python3.15 -I -c "import sys; ls=open(sys.argv[1],'rb').read().splitlines(); print([max(ls[n-1]) for n in (95, 96, 105)])" internal/event/gen_test.go`. Each value must be below 128: no raw rune is left on those lines.

4. **Proof.** The find and replace texts contain no backslash. The bmp plant is one line: ASCII r stays r, and any r of 0x80 or more becomes U+FFFD. U+FFFD itself is unchanged.
   ```json
   [{"id":"G2-72-pair","file":"internal/event/unescape.go","find":"dst = utf8.AppendRune(dst, pair)","replace":"dst = utf8.AppendRune(dst, utf8.RuneError)","test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-72-bmp","file":"internal/event/unescape.go","find":"dst = utf8.AppendRune(dst, r)","replace":"dst = utf8.AppendRune(dst, min(r, 0x7f)+(utf8.RuneError-0x7f)*min(r>>7, 1))","test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"red"}},
    {"id":"G2-72-control","control":true,"test":"TestDecodeDifferential","pkg":"internal/event","old_rev":"<lane base>","new_rev":"<this commit>","expect":{"old":"green","new":"green"}}]
   ```
   - Each find text occurs once (`unescape.go:77` and :83).
   - **The bmp expression:** for r below 0x80, `r>>7` is 0, so the value is r. For r of 0x80 or more, it is 0x7F + 0xFF7E = 0xFFFD.
   - **Old reads green under both plants.** No old input reaches :77, nor :83 with an escaped non-ASCII rune that is not U+FFFD.
   - **New reads red.**
     - pair: `"⧵ud83d⧵ude00"`, `"⧵ud800⧵udc00"` and `"⧵udbff⧵udfff"` become U+FFFD, so the decoder disagrees with the oracle.
     - bmp: `"⧵u00e9⧵u4e2d"`, `"⧵uffff"` and `"⧵ufffe"` become U+FFFD.
   - **Coverage:** `go test -count=1 -run '^TestDecodeDifferential$' -coverprofile=<f> ./internal/event`. The `unescape.go` block starting on line 76 and holding :77 must go from **0** at the old revision to **more than 0** at the new one.
   - **Stop rule:** if the old count is more than 0, record G2-72 as K2a (plants only) and say so in the record.

5. **Commit:** CD-event-K2, together with G2-68, G2-70, G2-71 and G2-73.

---

Coordinator-only note, not part of design-CD1.md: other sections of my first note held backslashes that may have been decoded the same way.
- **JSON escapes ⧵t, ⧵n and ⧵":** the batch entries of G10-16, G2-65 and G2-68.
- **The function-header text in G2-79.**
- **Regex escapes:** G2-78's grep and G2-79's greps.

They are worth checking in design-CD1.md.
---

### Coordinator corrections after the pre-check (2026-11-20; evidence: precheck-1.md)

1. **Coverage blocks start at their first statement** (Go's coverprofile), not at the `if` line. Read these blocks:
   - G2-70: `unescape.go:72.…` (u escapes), `unescape.go:77.…` (pair), `decode.go:276.…` (timestamp), `decode.go:114.…` (the three key rows).
   - G2-69 and G2-72: the block `unescape.go:77.5,79.1`.

   At the old revision, under TestDecodeDifferential, `unescape.go:76.4,76.78` is already 1 (the `if pair :=` line). Only the :77 block is 0, so the K2b claims hold. **Do not read the :76 block**: it would fire G2-72's stop rule falsely.
2. **G2-79's grep:** the regex needs digits. Use `^var [A-Za-z0-9]+ = ⧵[⧵]pecRow⧵{` (⧵ = backslash). Old counts are 4 + 1 (contract test :88, :263, :303, :369; operands file :15). The note's `[A-Za-z]+` misses `pecG1Rows`.
3. **Pre-checked:**
   - Every JSON entry parses, and its find occurs exactly once (no lost backslashes outside G2-69..72).
   - G2-65's new body is green on the real code and red under the plant.
   - G2-68 old reads timeout (122 s).
   - G2-69's input keeps TestDecodeLeafAllocations at 0.
4. **G2-134's counts span two commits (DQ2-L5).**
   - At CD-text-K2: run the shared check over the 7 internal/text pairs. Old 7, new 0.
   - At CD2's C4: the plan pair. Old 1, new 0 (design-CD2-addendum.md).
