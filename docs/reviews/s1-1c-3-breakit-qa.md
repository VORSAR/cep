> **Historical — S1.1c-3, the documents and the clocks; merged in PR #38.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-3-plan-check.md) · [design](../sprints/s1-1c-3-design.md) · [record](../sprints/s1-1c-3-record.md) · reviews: [research fragments](s1-1c-3-research-fragments.md), [research fragments angle](s1-1c-3-research-fragments-angle.md), [research vuln canary](s1-1c-3-research-vuln-canary.md), [plan check architecture](s1-1c-3-plan-check-architecture.md), [plan check qa](s1-1c-3-plan-check-qa.md), [design architecture](s1-1c-3-design-architecture.md), [design qa](s1-1c-3-design-qa.md), [combined architecture](s1-1c-3-combined-architecture.md), [breakit qa](s1-1c-3-breakit-qa.md). **← [S1.1c-2](../sprints/s1-1c-2-record.md) · [S1.1c-4 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-3 break-it (QA, blind), 2026-11-13** — the QA breaker on the combined build at 523cc39; every finding a committed red test on branch `s1-1c-3/breakit` (bc0589d..94e0d76). Every finding is placed in [the record](../sprints/s1-1c-3-record.md).

# S1.1c-3 combined build: QA's break-it (sprint model §6 step 7)

**Counts: 2 blockers, 7 should-fix, 4 later.**

Done blind against `s1-1c-3/breakit` at /Users/ramith/code/cep-wt/s1-1c-3-breakit, cut from 523cc39. Six commits, tests only (bc0589d..94e0d76). No code was fixed. Red tests are in:
- `internal/parser/breakit_s11c3_test.go`: 11 tests, all red.
- `gate/toolchain/tests/test_breakit_s11c3.py`: 2 classes, 4 tests, all red.

Every other `internal/parser` test is still green (`go test ./internal/parser -run 'Test[^B]'` gives ok). Tools used: Go 1.27.4 and CPython 3.15.2, both pinned, and ruff 0.17.3. gofmt, go vet and ruff (format and check) are clean. Each commit passed the T0 hook (exit 0). GitNexus was not used: its graph DB was held by another process. The code was read directly.

## Verdict

1. **Not ready for G5: 2 blockers.** Both are silent passes in the sprint's own subject, and both break acceptance line 1.
   - The incident's shape gets past the check by ordinary Markdown means (BK-B1).
   - A misspelt placeholder switches off the `text`-marker check (BK-B2).
2. **The fragment checker's Markdown model is looser than CommonMark in four places:**
   - an HTML block (BK-S1);
   - an indented block with no blank line before it (BK-S2);
   - a backtick "fence" whose info string holds a backtick (BK-S3);
   - capitalised keywords (BK-S7).
3. **The register keys a span, not an occurrence** (BK-S6).
4. **Two misleading reasons users see:**
   - F3's scan names a later number in place of the first mistake (BK-S4);
   - a tree child that never hashed is reported as a damaged tool (BK-S5).
5. **Aging, QB10 and BA10's stop, cap and race paths held** (list at the end).

## Blockers

**BK-B1: The incident's shape (`-> and not`, `-> not`) gets past the check when the text that shows it is wrapped across a soft line break, has emphasis between its words, or uses a no-break space.**
- **Why it is missed:**
  - `checkIncident` runs the regexp one source line at a time.
  - `incidentPattern`'s `\s` is ASCII-only in Go's RE2.
  - Emphasis markers sit between the words.
  - Each of these still renders as `-> and not` / `-> not` to a reader.
- **What it breaks:**
  - Acceptance line 1 ("the incident … in letters … red").
  - Design §2.3 / DA-B1: "the incident shape is checked on every fence whatever its tag, every span marked or not, and every prose line".
  - ADR-0015 §3.6 residue (c): the shape "is still checked on every prose line". The check is weaker than that claim, because a prose sentence is not a source line.
- **Red test:** `TestBreakitIncidentShapeEvadedByRendering`, commit bc0589d. 6 subtests red:
  - prose wrapped after `and`;
  - prose wrapped after the arrow;
  - `-> *and not*`;
  - `-> not` in prose;
  - a `text`-marked span with a no-break space;
  - a `text` fence with a no-break space.
- **Failing output, quoted:**
  ```
  BK-B1: the incident's shape, rendered as one sentence, gives no -> before and/not fault: "The refused form is every a = A -> and\nnot B[x == 1] for 5 min in a rule.\n" -> []
  BK-B1: ... "The refused form is every a = A -> *and not* B[x == 1] for 5 min in a rule.\n" -> []
  BK-B1: ... "Refused: `every a = A -> and not B[x == 1] for 5 min`<!--text-->.\n" -> []
  ```
- **Proposed fix:**
  - Run the prose incident check per paragraph, not per line. Join a paragraph's lines (blockquote markers removed, code spans blanked) with a space before matching, and report the paragraph's first line.
  - Before matching prose, normalise it: strip emphasis runs (`*`, `_`, `~~`), decode `&gt;`/`&rarr;`, and map Unicode whitespace (U+00A0, U+2000–U+200B, U+202F, U+3000) to a space.
  - In the shared pattern, use `[\s\p{Zs}\x{200B}]` in place of `\s`. It is one definition (§10 C3), so the helper picks it up too.

**BK-B2: A misspelt angle placeholder lets a span of grammar tokens carry a `text` marker, and the misspelling is never named.**
- **The control:** `` `partition by <key> of <Stream> ttl <duration>`<!--text--> `` is red ("text marker on grammar tokens").
- **The same span with `<Strem>`:** green, under an inline `text` marker and in a `text` fence alike.
- **Why:** `detect()` sets `notation = true` when `bindPlaceholders` fails, and the `text` path never binds. So "unknown placeholder" is reported only under `cep-meta`/`cep-invalid`.
- **What it breaks:** acceptance line 1 lists both "a `text` marker on a span of grammar tokens" and "an unknown angle placeholder" as red. Design §2.3 says "an angle name outside the table → `unknown placeholder <name>`" without restricting it to the bound forms.
- **Note for the owner:** I read acceptance line 1 as applying to every span. If it was meant only for spans that get bound (`cep-meta`/`cep-invalid`), this drops to should-fix. The hole stays either way: a typo makes rule text green under `text`.
- **Red test:** `TestBreakitUnknownPlaceholderHiddenByTextMarker`, commit bc0589d. The control is green; 2 subtests are red.
- **Failing output, quoted:**
  ```
  BK-B2: a misspelt placeholder (inline text marker) silences the text-marker check and is never named: "Form: `partition by <key> of <Strem> ttl <duration>`<!--text-->.\n" -> []
  BK-B2: a misspelt placeholder (text fence) silences the text-marker check and is never named: "```text\npartition by <key> of <Strem> ttl <duration>\n```\n" -> []
  ```
- **Proposed fix:**
  - An angle token outside `angleNames` is never a notation token.
  - In `judgeSpan`'s unmarked/`text` path, and in `judgeFence`'s `text` lines, call `bindPlaceholders` on rule-like text and report its `unknown placeholder <name>` error.
  - Judge text-ability on the bound tokens, as the marked path already does.

## Should-fix

**BK-S1: An HTML block's rule text is never read. A `<pre>` block holding a wrong rule gives no fault.**
- **What the build does:** it refuses only a backticked span inside an HTML block (QA row (13)).
- **What the design says:** §2.3 says "indented code blocks and HTML blocks are refused by name".
- **The plant:** `<pre>\nfrom every a = A -> b = B within\n…\n</pre>` (`within` with no duration). It renders as code.
- **Red test:** `TestBreakitHTMLBlockRuleTextUnread`, commit bc0589d.
- **Failing output:** `BK-S1: an HTML <pre> block holding a wrong rule (within with no duration) gives no fault; the design refuses HTML blocks by name: "Example:\n\n<pre>\nfrom every a = A -> b = B within\n…"`
- **Proposed fix:** refuse an HTML block by name on its first line (`HTML block: write rule text in a tagged fence`), as the indented block is. The three documents hold no HTML blocks today (measured with grep: 0 `<pre`/`<div`/`<details`/`<table`), so nothing real turns red.

**BK-S2: An indented code block is refused only when the line before it is blank, so one after a heading, a thematic break or a closed fence is read as prose and its wrong rule passes.**
- **Why it matters:** CommonMark starts an indented block in each of these places, because the block only cannot interrupt a paragraph. `prevBlank` is also false right after a fence's closing line.
- **Red test:** `TestBreakitIndentedCodeBlockWithoutBlankLine`, commit bc0589d, 3 subtests red. Quoted: `BK-S2: an indented code block after a heading is not refused (a wrong rule in it passes): "## Example\n    from every a = A -> b = B within\n" -> []`
- **Proposed fix:** track "may start an indented block" the way CommonMark does. It is true after a blank line, an ATX/setext heading, a thematic break, a closed fence, or the end of an HTML block. It is false only inside a paragraph or a list item's lazy continuation.

**BK-S3: A line that CommonMark does not read as a fence opens a fence in `readBlocks`, and every later line is skipped unjudged until a closing run or the end of the file.**
- **Which lines:** a backtick fence whose info string holds a backtick, such as ``` ```go `x` is how a Go fence opens. ```.
- **Effect:** a wrong `cep`-marked span written after that line gives no fault, and the census does not count it. The census pins catch only existing spans that get swallowed, not new ones.
- **Red test:** `TestBreakitBacktickInfoStringIsNotAFence`, commit bc0589d. The control is red as it should be; the plant is green. Quoted: `` BK-S3: a wrong cep-marked span after a line that is not a fence (info string with a backtick) is unjudged: "```go `x` is how a Go fence opens.\n\nThe clause `from every a = A -> b = B within`<!--cep:patternbody--> closes it.\n" ``
- **Proposed fix:** in `fenceOpen`, a backtick fence's rest of line must hold no backtick (CommonMark 4.5); otherwise the line is prose. Also report a fence still open at the end of the file (`fence never closes`), because an unclosed fence hides everything after it.
- **Note:** this is the most contrived of the seven.

**BK-S4: F3's forward scan (`bareDuration`) reports a later bare number in the statement in place of the error participle actually stopped at.**
- **Control:** `stream S (a string ts timestamp) time ts watermark bounded 1 min;` gives E-SYN-001 at 2:20 (the missing comma).
- **With `bounded 1`:** E-DUR-001 at 2:60.
- **With `… idle after 5`:** E-DUR-001 at 2:77.
- **Effect:** the first mistake is never named, and the position jumps past it.
- **Red test:** `TestBreakitBareDurationScanMasksEarlierError`, commit ff87201.
- **Failing output, quoted:**
  ```
  BK-S4: "stream S (a string ts timestamp) time ts watermark bounded 1;\n" reports E-DUR-001 at 2:60 (1 has no unit); the first mistake, the missing comma, is E-SYN-001 at 2:20 (unexpected token "ts" (expected ")" time arrival or time <attr> watermark …? ";"))
  ```
  Also seen in probes: `time ts watermarkk bounded 1;` gives E-DUR-001 at the `1`, never naming `watermarkk`.
- **Proposed fix:** take the bare number only when participle's stop is the token right after the abandoned clause. That means the stop is at or after `i+1`, with nothing but that clause between them. Or scan backwards from the stop rather than forwards. Add contract rows: a missing comma before `bounded 1`, and a misspelt `watermark` before `bounded 1`.
- **Probes that held:** the scan never crosses a `;` (an error in one statement plus a bare number in the next is reported correctly). The decision 19 and F5 branches fire right: `-> and not`, `->\n and not`, `-> /* c */ and not`, `-> not`, `-> ->`, a trailing `->`, `-> s = R -> and not …`.

**BK-S5: A tree child that cannot start, crashes, or is killed by a signal is reported as `go: tree differs: run make clean-tools tools`, the damaged-tool reason, whose advice cannot help.**
- **Why:** `_tree_failure` turns every `OSError` from `tree_digest` into an empty digest. That covers the spawn's own error (EMFILE, ENOMEM, a missing interpreter) and `the tree child exited <n>`. The child's stderr is written to `tree-<tool>.err` but never reaches the reason.
- **What the design says:** §3.6 / DA-S13e gives the cap its own reason for this very cause ("a slow disk is not a damaged tool: clean-tools would not help"). A child that never hashed is not a damaged tool either. These failure modes are new with BA10: before it, the hash ran in-process.
- **Red test:** `gate/toolchain/tests/test_breakit_s11c3.py::TreeChildFailureIsNotADamagedTool`, commit bb6e601, 3 tests red.
- **Failing output, quoted:**
  ```
  AssertionError: 'tree differs' unexpectedly found in 'go: tree differs: run make clean-tools tools' : BK-S5: a spawn failure (EMFILE) is reported as a damaged tool
  AssertionError: 'tree differs' unexpectedly found in 'go: tree differs: run make clean-tools tools' : BK-S5: a crashed tree child (exit 1, a traceback) is reported as a damaged tool
  AssertionError: 'tree differs' unexpectedly found in 'go: tree differs: run make clean-tools tools' : BK-S5: a tree child killed by a signal (-9) is reported as a damaged tool
  ```
- **Proposed fix:**
  - Treehash's exit 3 (an `OSError` while reading the tree, i.e. a damaged or unreadable tree) keeps `tree differs: RECOVER`.
  - A spawn `OSError` becomes `<tool>: the tree hash could not start: <err>`.
  - Any other exit or signal becomes `<tool>: the tree hash failed (exit <n>): <first stderr line>`.
  - Pin each reason.

**BK-S6: A register entry covers every occurrence of its span in the file, so a listed wrong form can be written again anywhere in that document and presented as correct.**
- **Why nothing catches it:**
  - The marker is invisible when the document is rendered.
  - The incident's shape stands inside a listed span (§10 C1).
  - No register line changes.
- **Example:** SEMANTICS.md lists `→ not B for D`, quoted in the 2026-11-07 correction note. A second sentence, "An absence is written `→ not B for D`<!--cep-invalid:chainelem--> after its step.", passes.
- **Red test:** `TestBreakitListedSpanCoversEveryOccurrence`, commit 24d3ccf. The control (one occurrence) is clean. Quoted: `` BK-S6: a second occurrence of a listed wrong form (the incident's shape, recommended) gives no fault: "*(Corrected: …`→ not B for D`<!--cep-invalid:chainelem--> …)*\n\nAn absence is written `→ not B for D`<!--cep-invalid:chainelem--> after its step.\n" ``
- **Related:** `FragmentValidatorCases` reads only the first occurrence's kind, so a validator entry's second occurrence at another kind is never re-run.
- **Proposed fix:** count occurrences per entry. Either `checkMarkdown` reports `listed invalid example appears <n> times: list each` when a listed span's `found` is hit twice, or the register's target carries an occurrence count that the test checks. The validator re-run then covers each occurrence at its own kind.
- **Severity note:** should-fix because it takes a deliberate (or confused) invisible marker on a recommendation, and the doc edit shows in LAW Δ as a doc line. The owner may want it raised to blocker, since it is the incident's exact historical form.

**BK-S7: A span whose keywords are written in capitals is not rule-like, so it needs no marker and is never parsed, and the incident's shape is matched only in lowercase.**
- **Why:** the detector counts only lowercase `Keyword` tokens. `MixedKeyword` tokens do not count.
- **Effect:** `` `FROM S[x == 1] SELECT a INSERT INTO Out` `` passes, though the language refuses it (E-KW-002). `-> AND NOT` / `-> NOT` pass in spans and in prose.
- **Red test:** `TestBreakitCapitalKeywordsEscapeTheDetector`, commit f2d90d8, 3 subtests red. Quoted: ``BK-S7: the incident in capitals gives no "-> before and/not" fault: "Write `EVERY a = A -> AND NOT B[x == 1] FOR 5 MIN` for an absence.\n" -> []``
- **Proposed fix:**
  - Count `MixedKeyword` tokens toward rule-likeness. Such a span must then be listed `cep-invalid` (stage grammar, E-KW-002) or written as text with notation.
  - Make the shared incident pattern case-insensitive (`(?i)`).
  - Measure on the three documents before landing. errors.md's E-KW-002 examples are likely single-token and stay non-rule-like.

## Later

**BK-L1: `ruleWords` replaces a production name with words but keeps participle's grammar quantifier after it.**
- **What the user sees:** `unexpected token "ts" (expected ")" time arrival or time <attr> watermark …? ";")`. The design wants a message that names no grammar type (§2.2), and `TestParseErrorMessagesNameNoGrammarType` checks names, not notation.
- **Red test:** `TestBreakitMessageKeepsGrammarQuantifier`, commit ff87201. Quoted: `BK-L1: the message holds a grammar quantifier outside its quoted tokens: unexpected token "ts" (expected ")" time arrival or time <attr> watermark …? ";")`
- **Proposed fix:** in `wordsFor`, drop a `?`, `*` or `+` that follows a replaced name, or write `(optional)`. Add a contract row.

**BK-L2: After a tree hash finishes, the runner's parity `Group` still holds the reaped child's pid as its live process group.**
- **Why:** `tree_digest` rightly reaps with `proc.wait` (never `Group.reap`, which has no cap), but nothing clears `Group._pgid`, which `reap` clears under its lock.
- **Effect:** a stop that arrives after one hash and before the next spawn sends TERM to that pid's group, and `kill_left` probes and then KILLs it. The kernel may have reused that pid number.
- **Why it is not the accepted window:** the residue accepts the pid-reuse window of reap's probe, which is an instant. This window lasts until the next spawn.
- **Red test:** `FinishedTreeHashLeavesNoLiveGroup.test_the_group_forgets_a_reaped_tree_child`, commit bb6e601. White-box: it asserts `_pgid`. Quoted: `AssertionError: 18907 is not None : BK-L2: the parity group still names the reaped tree child's pid as its live group`
- **Proposed fix:** give `Group` a capped wait, `wait(proc, timeout) -> int`, which clears `_pgid` under the lock once the leader is reaped. `tree_digest` calls it when the spawner provides it, and plain `Popen.wait` for the installer's `popen`.

**BK-L3: A `cep-invalid` span is judged only at its marker's kind, so valid rule text marked invalid at a kind it cannot parse at, and listed at grammar, passes as a documented-invalid example.**
- **Example:** `within 5 min` parses at `within`. Marked `cep-invalid:expr` and listed, it is accepted, and the document then shows a valid clause as refused.
- **Red test:** `TestBreakitInvalidMarkerAtAWrongKind`, commit 24d3ccf. Quoted: `` BK-L3: a span valid at another kind, marked cep-invalid at a kind it fails at, passes as documented-invalid: "Refused: `within 5 min`<!--cep-invalid:expr-->.\n" ``
- **Why later:** the register entry shows in LAW Δ by name.
- **Proposed fix:** for a grammar-stage entry, also require that the span (bound) fails at every kind. Or at least report `parses at <kind>` when it parses at any kind, using `fragEntries` in order.

**BK-L4: The leading-comma binding (added 2026-11-13 for E-PART-002's pinned fix) is applied until nothing changes, so a doubled comma binds away and the span passes.**
- **Example:** `` `, , k of S`<!--cep-meta:partkey--> `` passes. One leading comma is the notation; a second one is a mistake.
- **Red test:** `TestBreakitDoubledLeadingCommaBindsAway`, commit 94e0d76. The control (one comma) is clean. Quoted: `` BK-L4: a doubled leading comma binds away and the span passes: "Fix: `, , k of S`<!--cep-meta:partkey-->.\n" ``
- **Proposed fix:** apply the `lead` forms once, not in the loop that runs until nothing changes.

## What held (green, confirmed by running or by reading the code against a probe)

**Incident shape (single-line forms).** Each of these gives `-> before and/not`:
- one prose line;
- a fence tagged `cep`, `cep-chain`, `text` or `go`, an untagged `~~~` fence (plus "untagged fence"), and an indented block after a blank line (plus "indented code block");
- a table cell;
- a span across a line break;
- a double-backtick span;
- a blockquote span;
- a `<div>` block line.

An HTML comment holding it is a "malformed marker" fault.

**Markers:**
- `<!--cep-frob:x-->` and `<!--cep:-->` are malformed.
- A marker separated from its span by a space gives "unmarked" plus "malformed".
- `text` on grammar tokens is red.
- `cep-invalid` on a span that parses is red ("documented as invalid, but parses").
- An unknown placeholder under `cep-meta` is red.

**Binding:**
- `partition by k of S … ttl 5 min` and a single leading comma bind as intended.
- A string whose whole content is `<…>` binds without a table check.

**The census pins:** a marker removed or changed in form moves a pinned count. A swap of two markers must still pass both judgements, so the census plus `checkMarkdown` together hold.

**parseError:** the incident's branches, F5's `-> ->` / trailing `->`, and F7's `]` position all behave as intended. F3 does not cross a statement boundary (BK-S4 is a different mistake).

**Aging** (driven through `aging.aged` with the real `gate/sprints.json`):
- **Prefix ids:** an id that is only a prefix of another id in the decision text (`BA1`/`BA10`, `S2.2b-2`/`S2.2b-20`, `QB10`/`QB10X`) is not "named", so it ages.
- **Sentence end:** `S1.1c-3-2.` is named.
- **Closed rows:** exactly two closed rows since is silent; three is red.
- **Correctness clock:** 30 days is silent; 31 is red.
- **Forwards:**
  - to a passed part: red;
  - to the same part: silent;
  - with a part-less marker: "the marker names no part";
  - to a closed row: red;
  - to an unknown sprint, or to `S1.1c-03`: named, never a crash.
- **`_entries`:** refuses odd fields (key set, types, a non-bool `correctness`, a duplicate id).

**QB10 (`rows.collisions`):**
- A dotted directory, `D/__init__.py` beside `D.py`, and a nested module under a shadowing package are each named.
- `run_modules` refuses before running anything.
- A case-only pair (`X.py`/`x.py`) is not reported. That is not silent: on macOS the import fails loudly, and on Linux both are distinct modules.

**BA10:**
- `Group.spawn` holds its lock, so a stop arriving exactly as the child spawns either refuses the spawn (`stopped before the tree hash`) or TERMs the new group. There is no in-between.
- `kill_left` with nothing spawned returns `(False, "")`.
- The cap kills the child's own group without signalling `Group`.
- `TREE_CHILD` and `TREE_CAP_S` are read at call time.
- The installer path is unchanged: `git diff main...HEAD` shows no change to `gate/toolchain/manifest.py` or `gate/bootstrap.py`.

**Gate posture:**
- **QA tables and instrument:** every QA table file and `research-spans.jsonl` is a law path and part of the `fragments` instrument (`gate/law.json` paths[46–51], instruments[37]). An edit to a QA table is therefore a LAW Δ line by file. That table-driven cases are not counted is residue (d), not filed.
- **`aging` row:** covered is the count of the files it reads, so an emptied list stays green by design.

## Not filed (no red test of code possible, or a named residue)

- **Mixed time zones for `opened`.** `opened` comes from `git log --format=%as`, which is the author's local date (the owner is at +0530). `today` comes from `rows.today()`, which is UTC. An entry added before 05:30 local with that day's date fails `knownissues.check`'s "not after today" until UTC catches up, and every 30-day clock is off by up to a day. This is a data convention in design §3.3, not code. Suggest `--date=format-local:%Y-%m-%d` with `TZ=UTC`, or document the convention.
- **Prose rule text is unparsed:** residue (c).
- **A passing mention of a known issue's id in its deciding section counts as the decision:** residue (g). A mention inside an HTML comment in a decision section (invisible to readers) would also count; noted for the owner's reading of (g).
