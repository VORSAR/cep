# S1.1c-7 design note — CD2 (validate, plan, table, expr)

*Built in W3 (2026-11-25 to 2026-11-26). Where the code differs from this note, the sprint record's W3 section says how and why ([record](s1-1c-7-record.md)).*

*S1.1c-7's design note for chunk CD2 (validate, plan, table, expr), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes.*


## The note

## S1.1c-7 design note: chunk CD2

### 1. Header
- **Chunk:** CD2. 16 findings: G10-15, G10-17, G10-22, G7a-3, G7a-6, G2-85, G2-86, G2-87, G2-88, G2-133, G2-135, G2-136, G2-137, G2-138, G2-139, G2-140.
- **Base:** HEAD is 1f4ebba, which is docs only after 37df879. `git diff --stat d521725 HEAD -- '*.go' conformance gate/runner/tests/test_notbuilt.py` prints nothing, so the index commit d521725 matches the production code.
- **Files read in full or in the cited ranges:**
  - expr: `expr.go`, `registry.go`, `runnable_test.go`, `nodes_test.go` (:1-20, :190-215).
  - plan: `plan.go`, `expect.go`, `body.go` (:70-131), `plan_test.go`, `expect_test.go`, `literalvalue_test.go`, `coverage_s3_3_2_test.go`, `internal_test.go`, `helpers_test.go`.
  - table: `table.go`, `table_test.go` (:1-175), `breakit2_test.go`.
  - validate: `units_test.go` (:1-120, :240-280), `s3_2c_breakit_test.go`, `hiding_test.go` (:1-125), `nulls.go`, `bounds.go` (:214-260), `resolve.go` (:26-150), `exprs.go` (:73-122), `names.go` (:60-85), `validate.go` (:130-135), `expects.go` (:225-243), `operands_test.go` (:108-122).
  - Coupled files: `conformance/notbuilt.json`, `conformance/corpus_test.go` (:100-160), `gate/runner/tests/test_notbuilt.py`, `gate/proofs/drivers/test_notbuilt_expired.py`, `deploy.go` (:60-90), `internal/runtime/shard.go` (:55-145), `build.go` (:35-50), `tools/importcheck/rules.json`, `.golangci.toml`, `gate/bench/mutate.py` and `load.py` headers, `tools/astequal/main.go` header, `docs/development/testing.md`.
- **GitNexus calls: 13.**
  - context: `Register`, `stream`, `TestBreakitGuardSnapshotUnderConcurrentWriters`, `nullFinding`, `givenList`, `callPlacement`, `defaults`, `zeroDurations`.
  - impact: `value`, `New` (table), `Register`, `NotRunnableError`, `Snapshot` (this one came back ambiguous).
- **What I could not check:**
  - No test or tool runs were allowed, so every expected outcome below is reasoned from the code.
  - The tools `ReadMcpResourceTool` (index freshness resource) and Hindsight are not in my tool set, so I could not read either. Freshness was established from git instead (above).
  - `mutate.py`'s `--patch`, `--subtest`, `--expect` and id rules are W0's, still to be built. I wrote entries to plan §2's contract.
  - In K2b coverage profiles, the block start column (`nulls.go:63.17`) is inferred from Go's case-clause block rule.

### 2. Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-15 | accepted | K1 | C1 expr K1 | – | `Register` refuses a nil Factory (kind ≠ PatternOnly) and MinArgs > MaxArgs |
| G10-17 | accepted | K1 | C3 plan K1 | – | "a %s literal is not a %s" → "a literal of kind %s is not of type %s"; 72 pins + 1 doc line |
| G10-22 | accepted (choice A) | K1 | C5 table K1 | – | drop the three unexported sentinels; text byte-identical; no refusal of `New` wraps a cause |
| G7a-3 | accepted | K1 (coordinator coupling commit) | C2 | – | `what`: "a text operator", "count()", "chain()"; notbuilt.json:17 and test_notbuilt.py:20 in the same commit |
| G7a-6 | accepted | K6 | C6 table K6 | – | `New`'s doc: replace "(owner D3: …S3.3's)" with the E-TBL-002 sentence |
| G2-85 | accepted | K2a | C4 plan K2 | – | derived streams assert `event.ArrivalTime`; the `-2` sentinel and the "gap G2" comment go |
| G2-86 | accepted | K2a | C4 | – | two rows: a timeless stream's own record; a field fault before a null time; doc names the step |
| G2-87 | accepted | K2e | C4 | rename → S1.1c-8; move → S1.1c-10 | `itoa` → `strconv.FormatInt` |
| G2-88 | accepted | K2a | C4 | – | `TestBuildRefusesNotBuilt` also requires a nil plan |
| G2-133 | accepted | K2a (+ load run) | C7 table K2 | rename + move → S1.1c-8 | per-table stamp row "g"; a later snapshot's stamp never falls |
| G2-135 | accepted | K2a | C8 validate K2 | rename → S1.1c-8; move as subtest → S1.1c-10 | assert exactly one E-DUR-003 |
| G2-136 | accepted | K2b | C8 | – | row rewritten over an optional attribute (`not S[note != 'x']` → E-NULL-001); old input kept as a new truthful row |
| G2-137 | accepted | K2-del | C9 validate K2-del | – | delete the row; H3 (`count() > 'a'`) is its stronger duplicate |
| G2-138 | accepted | K2a | C8 | – | row becomes a standalone subtest of the same name asserting `E-PARAM-001@<pos of "param bad">` |
| G2-139 | accepted | K2a | C8 | – | also assert `diags[0].Code == "E-NULL-002"` |
| G2-140 | accepted | K2e | C8 | – | check `MarshalCanonical`'s error at :257 |

**Ledger outcomes:**
- fixed: 13.
- fixed in part: G2-87, G2-133, G2-135.
- carried in full: none. Rejected: none. Merged: none.

### 3. Findings

#### G10-15: `Registry.Register` accepts a descriptor Build cannot call
1. **Verdict: accepted.**
   - `registry.go:67-74` checks only for a duplicate name.
   - `expr.go:291` calls `d.Factory(args, n)` for every kind except PatternOnly (`:280-282`), so a nil Factory on a Scalar or Aggregate panics at Build.
   - With MinArgs > MaxArgs, `expr.go:277` refuses every call, and `arity()` (`registry.go:45-53`) prints "3 to 2 arguments".
   - GitNexus `context(Register)` finds three callers: `Builtins` (registry.go), `TestRegistryArityAndLookup` (nodes_test.go), `TestABuildErrorWithoutACodeIsETYPE001` (validate/builderror_test.go). Epistemic: exact.
   - `impact(Register, upstream)`: **risk HIGH** (25 impacted; d1 3, d2 7, d3 15; one process, plan's `rule`; modules Expr, Validate, Plan). **Warning:** `Builtins()` runs on every `Build` and `BuildRunnable` (`expr.go:95, 123`) and panics on a Register error (`registry.go:99-101`).
   - Checked: all nine built-ins pass the new rules (`registry.go:106-116`). Eight have a Factory and chain is PatternOnly. Every arity has MinArgs ≤ MaxArgs (chain is 1..maxArgs). The test callers also pass: "pick" is 1..2 with a Factory; "boom" is 0..0 with a Factory.
2. **Choice (internal, owner Q9).**
   - Candidates:
     - (A) refuse in `Register`;
     - (B) check `d.Factory == nil` in `builder.call` and return an error at Build;
     - (C) reject the finding (only built-ins register).
   - **Choice: A.** The registry is the one registration API (`registry.go:57-58`, "extensions to come"). A bad descriptor should fail where it is added, once, not at every Build of every program. B leaves a registry that holds an unusable callable.
   - Not added: MinArgs < 0. It cannot panic; `call` handles it, and only the arity text would read oddly.
3. **Kind: K1.** R5 does not apply: deploy and start time only.
   - `Register` runs from `Builtins`, which `Build`/`BuildRunnable` call at deploy (`plan.go:213`).
   - It also runs from the plan's `newBody` closure (`plan.go:263-271`), which `runtime/shard.go:70` calls inside `install`.
   - `install` is reached only on the shard loop's `mAdd` branch (`shard.go:141-143`) and at `build.go:46`, never on an event message.
4. **The exact change:**
   - `internal/expr/registry.go`, Descriptor's Factory comment (`:40-41`), new text:
     ```go
     // Factory types the built arguments and gives the node; it reports E-TYPE-001 or E-FN-001. It is nil only for a
     // pattern-only callable, whose call Build types itself; Register refuses a nil Factory for any other kind.
     ```
   - `Register` (`:66-74`) becomes:
     ```go
     // Register adds d. It refuses a descriptor Build could not call (a nil Factory for any kind but PatternOnly, or
     // MinArgs above MaxArgs) and a second registration of a name.
     func (r *Registry) Register(d Descriptor) error {
     	switch {
     	case d.Factory == nil && d.Kind != PatternOnly:
     		return fmt.Errorf("expr: %s: a nil Factory; only a %s callable has none", d.Name, PatternOnly)
     	case d.MinArgs > d.MaxArgs:
     		return fmt.Errorf("expr: %s: MinArgs %d is above MaxArgs %d", d.Name, d.MinArgs, d.MaxArgs)
     	}
     	if _, dup := r.descriptors[d.Name]; dup {
     		return fmt.Errorf("expr: %s is already registered", d.Name)
     	}
     	copy := d
     	r.descriptors[d.Name] = &copy
     	return nil
     }
     ```
   - New test in `internal/expr/nodes_test.go`, placed after `TestRegistryArityAndLookup`. It needs no new imports.
     ```go
     // TestRegisterRefusesADescriptorBuildCannotCall: Register refuses a nil Factory for a scalar or an aggregate, and
     // MinArgs above MaxArgs, registering nothing; a pattern-only callable needs no Factory (chain has none).
     func TestRegisterRefusesADescriptorBuildCannotCall(t *testing.T) {
     	first := func(args []expr.Expr, _ model.Expr) (expr.Expr, error) { return args[0], nil }
     	refused := []struct {
     		name string
     		d    expr.Descriptor
     		want string
     	}{
     		{"a scalar with no Factory", expr.Descriptor{Name: "s", Kind: expr.Scalar, MinArgs: 1, MaxArgs: 1},
     			"expr: s: a nil Factory; only a pattern-only callable has none"},
     		{"an aggregate with no Factory", expr.Descriptor{Name: "g", Kind: expr.Aggregate},
     			"expr: g: a nil Factory; only a pattern-only callable has none"},
     		{"MinArgs above MaxArgs", expr.Descriptor{Name: "m", Kind: expr.Scalar, MinArgs: 2, MaxArgs: 1, Factory: first},
     			"expr: m: MinArgs 2 is above MaxArgs 1"},
     	}
     	for _, c := range refused {
     		t.Run(c.name, func(t *testing.T) {
     			r := expr.NewRegistry()
     			if err := r.Register(c.d); err == nil || err.Error() != c.want {
     				t.Errorf("Register = %v, want %q", err, c.want)
     			}
     			if _, ok := r.Lookup(c.d.Name); ok {
     				t.Errorf("the refused descriptor %s is registered", c.d.Name)
     			}
     		})
     	}
     	r := expr.NewRegistry()
     	if err := r.Register(expr.Descriptor{Name: "p", Kind: expr.PatternOnly, MinArgs: 1, MaxArgs: 2}); err != nil {
     		t.Errorf("a pattern-only callable with no Factory: %v, want it registered", err)
     	}
     	if err := r.Register(expr.Descriptor{Name: "e", Kind: expr.Scalar, MinArgs: 1, MaxArgs: 1, Factory: first}); err != nil {
     		t.Errorf("MinArgs equal to MaxArgs: %v, want it registered", err)
     	}
     }
     ```
5. **Proof (K1):**
   - Revert plant: the inverse of the commit's production diff, `git diff <C1> <C1>~1 -- internal/expr/registry.go > g10-15.diff`.
   - Run: `python3.15 -I -B gate/bench/mutate.py --patch g10-15.diff --test TestRegisterRefusesADescriptorBuildCannotCall --pkg internal/expr`.
   - Expected: **red**, on the new test's `t.Errorf("Register = %v, want %q", …)` and `t.Errorf("the refused descriptor %s is registered", …)` lines, in all three subtests.
   - T2 green.
6. **Commit:** C1, expr K1.

#### G10-17: "a int literal is not a object"
1. **Verdict: accepted.**
   - `expect.go:239` reads `fmt.Errorf("a %s literal is not a %s", l.Kind, t)`, and pins print "a int …", "… a object" and "… a invalid" (literalvalue_test.go:39-47, :99-117).
   - GitNexus `impact(value, upstream)`: risk LOW, exact. d1: `thenRecord`, `values`, `TestValueEveryKindAndType`. d2: `givenList`, `givenTable`, `thens`. d3: `expects`, `givens`. d4: `Build` plus two internal tests. No process.
   - This is Build-time only. Through Deploy the validator refuses first (E-EXP-002, `validate/expects.go:183`), so the text is reached only from an unchecked model.
2. **Choice (text).**
   - Candidates:
     - (A) a local `article()` in plan, giving "an int literal is not an object". This would copy validate's `article` (`validate/expects.go:238-243`), because `tools/importcheck/rules.json`'s `internal/plan` list has no `internal/validate`.
     - (B) fixed-article wording: "a literal of kind int is not of type object";
     - (C) move `article` to a shared package. That is a cross-package move, deferred by Q4.
   - **Choice: B.**
     - No duplicated helper.
     - The article is correct for every kind and type, including names a vowel rule gets wrong.
     - It matches the model's own phrase: `internal/model/literal.go:5` says "a literal of kind k".
   - **New text:** `a literal of kind <kind> is not of type <type>`.
3. **Kind: K1.** Not per-event (Build only; the GitNexus upstream above). It is also an error-text-only edit, so R5's exemption is named.
4. **The exact change:**
   - `internal/plan/expect.go:239` → `return event.Value{}, fmt.Errorf("a literal of kind %s is not of type %s", l.Kind, t)`.
   - **Dry run** (`git grep -n "literal is not a"`, outside docs). Every pin is in three test files, 72 strings in all. The build agent applies the regex `a ([A-Za-z0-9()]+) literal is not a ([A-Za-z0-9()]+)` → `a literal of kind \1 is not of type \2` to:

     | File | Matches | Lines |
     |---|---|---|
     | `internal/plan/literalvalue_test.go` | 64 | every error row, :28-117 |
     | `internal/plan/expect_test.go` | 5 | :158, :163, :167, :169, :171 |
     | `internal/plan/coverage_s3_3_2_test.go` | 3 | :47, :51, :55 (`strings.Contains`) |

   - Also `literalvalue_test.go:17`, by hand: `"a <kind> literal is not a <type>"` → `"a literal of kind <kind> is not of type <type>"`.
   - The regex matches nothing in expect.go (`%s` is outside its class). The test messages `value(a %s literal, %s)` at literalvalue_test.go:137, :141, :143 are not pins and stay as they are.
   - Docs: only the closed `docs/sprints/s1-1c-6-design.md:522, :1180, :1215` quote the old text. They stay (history).
5. **Proof (K1):**
   - Run: `python3.15 -I -B gate/bench/mutate.py --file internal/plan/expect.go --find 'fmt.Errorf("a literal of kind %s is not of type %s", l.Kind, t)' --replace 'fmt.Errorf("a %s literal is not a %s", l.Kind, t)' --test TestValueEveryKindAndType`.
   - Expected: **red** at `literalvalue_test.go:143` (`t.Errorf("value(a %s literal, %s): error %v, want %q", …)`).
   - T2 green.
6. **Commit:** C3, plan K1. It lands **before** C4, whose new G2-86 row uses the new text.

#### G10-22: unexported `%w` sentinels in internal/table
1. **Verdict: accepted.**
   - `table.go:50-54` declares `errEmptyName`, `errNoAttrs` and `errNoKey`, used at `:62`, `:70` and `:84` with `%w`.
   - `errors.` appears in table.go only at `:51-53` (grep). No `errors.Is`, `errors.Unwrap` or `errors.As` on table errors anywhere (grep).
   - Every table test file is `package table_test` except `checkvalue_bench_test.go` (a benchmark), so no one can match the sentinels.
   - The other eleven refusals of `New`, `indexAttrs` and `markKey` are plain `fmt.Errorf` (`:90-114`).
   - `TestNewRejects` (table_test.go:64-98) asserts only `err != nil` and `tb == nil`.
   - GitNexus `impact(New, upstream)`: **risk CRITICAL** (60 impacted; d1 20, d2 40; processes `runBlock`, playback/expect.go, and `Deploy`; modules Runtime, Table, Playback, Expr, Conformance, Plan). **Warning:** the text is byte-identical, so no caller's message changes. The only observable difference is that `errors.Unwrap` of these three errors becomes nil, and nothing in the module unwraps them.
2. **Choice (internal, owner Q9).**
   - Candidates:
     - (A) drop the sentinels and keep each text byte-identical;
     - (B) export one `ErrInvalidTable` and wrap all fourteen refusals with it;
     - (C) reject.
   - **Choice: A.** No caller matches a table refusal. The validator refuses these declarations before `New` runs: E-NAME-001 and E-TBL-002 (`validate/names.go:61-85`). B would add an exported contract with zero users and change all fourteen refusal sites.
3. **Kind: K1.** R5:
   - `New` runs once per table at deploy (`Deploy` → plan → runtime `FromPlan`), at the start of an expect sandbox (`runBlock`), and at the start of a replay (playback `newDriver`). It never runs per event.
   - The edit touches only the three refusal branches' error construction; the success path is byte-identical. This is the named error-branch exemption.
4. **The exact change** in `internal/table/table.go`:
   - Delete `:50-54` (the `var ( errEmptyName … )` block) and the blank line after it.
   - `:62` → `return nil, errors.New("table: empty table name")`. This keeps the `errors` import. The `fmt.Errorf`-with-no-arguments form is avoided.
   - `:70` → `return nil, fmt.Errorf("table %s: no primary key", name)`.
   - `:84` → `return nil, fmt.Errorf("table %s: no attributes declared", name)`.
   - Test: `internal/table/table_test.go` imports gain `"errors"`. In `TestNewRejects`'s `t.Run` body, after the `tb != nil` check:
     ```go
     			if inner := errors.Unwrap(err); inner != nil {
     				t.Errorf("New's refusal %q wraps %v; every refusal of New is a plain error naming its fault", err, inner)
     			}
     ```
5. **Proof (K1):**
   - Plant: `git diff <C5> <C5>~1 -- internal/table/table.go > g10-22.diff`.
   - Run: `mutate.py --patch g10-22.diff --test TestNewRejects --pkg internal/table`.
   - Expected: **red** on the new `t.Errorf("New's refusal %q wraps %v; …")` line, in the subtests "an empty name", "no attributes (nil)", "no attributes (empty)", "a nil key" and "an empty key".
   - T2 green.
6. **Commit:** C5, table K1.

#### G7a-3: sprint ids in `NotRunnableError.What`
1. **Verdict: accepted.**
   - The ids are at `expr.go:249` (`"text operators (S7)"`), `:304` (`"chain (S4.1)"`) and `registry.go:188` (`"count (S8)"`).
   - They reach users through `NotRunnableError.Error()` (`expr.go:117`) and `plan/body.go:92-93` (`fmt.Errorf("%s: %w", nr.What, ErrNotBuilt)`). `deploy.go:73-74` wraps that as "cep: deploy <name>: unsupported operation: plan: rule <R>: text operators (S7): plan: this body kind is not built yet".
   - Also seen: the old text's grammar is wrong ("text operators … is not executable yet").
   - GitNexus `impact(NotRunnableError, upstream)`: risk LOW, 4 impacted, d1 1, process `rule` (plan.go).
2. **Choice (text).**
   - Candidates:
     - (A) the survey's plural phrases "text operators", "count", "chain" with "not supported in this release";
     - (B) singular noun phrases that read right in both slots ("expr: X is not executable yet" and "rule R: X: plan: …"), in the style of notbuilt.json's "a pattern body" and "a window body";
     - (C) keep the phrases and only cut the parenthesis.
   - **Choice: B: "a text operator", "count()", "chain()".**
     - It fixes the grammar.
     - "count()" and "chain()" match validate's user text (`exprs.go:113`: `"%s() is an aggregate…"`).
     - It changes only the sprint ids and their nouns. "is not executable yet" stays, because the finding's scope is sprint ids.
   - **New user-visible texts:**
     - Deploy of 05: `cep: deploy 05-text-operators.cep: unsupported operation: plan: rule <R>: a text operator: plan: this body kind is not built yet`.
     - `NotRunnableError.Error()`: `expr: a text operator is not executable yet`, `expr: count() is not executable yet`, `expr: chain() is not executable yet`.
     - `later.Eval` (`expr.go:327`): `a text operator is not executable yet`. BuildRunnable refuses these trees first, so this is not reached through Deploy.
     - The notbuilt row line on expiry: `05-text-operators.cep: not built (a text operator), listed for S7, which is reached by <marker>`.
3. **Kind: K1, in a coordinator coupling commit** (plan §0 item 5). R5: error-text-only (the `what` constants, set at Build); `later.Eval`'s code is unchanged. The named exemption applies.
4. **The exact edits for the coordinator** (one commit, C2):
   1. `internal/expr/expr.go:113-114` →
      ```go
      // NotRunnableError is an expression that types but cannot be evaluated by this release: it holds a construct whose
      // Eval always errors (What names the construct, e.g. "a text operator").
      ```
   2. `expr.go:249`: `what: "text operators (S7)"` → `what: "a text operator"`.
   3. `expr.go:304`: `what: "chain (S4.1)"` → `what: "chain()"`.
   4. `internal/expr/registry.go:188`: `what: "count (S8)"` → `what: "count()"`.
   5. `internal/expr/runnable_test.go`:
      - `:13`, `— text operators (S7), count (S8), chain (S4.1) —` → `— a text operator, count(), chain() —`.
      - `:22-23`, `"text operators (S7)"` → `"a text operator"` (twice).
      - `:24-25`, `"count (S8)"` → `"count()"` (twice).
      - `:26`, `"chain (S4.1)"` → `"chain()"`.
   6. `conformance/notbuilt.json:17`: `"construct": "text operators (S7)",` → `"construct": "a text operator",`.
   7. `gate/runner/tests/test_notbuilt.py:20`: `("05-text-operators.cep", "text operators (S7)", "S7"),` → `("05-text-operators.cep", "a text operator", "S7"),`.
   - **Dry run** (`git grep -n "text operators (S7)\|count (S8)\|chain (S4.1)\|(S7)"` outside docs). Every pin is listed above:
     - runnable_test.go:13, :22-26;
     - notbuilt.json:17, read by `TestCorpus`'s `strings.Contains` (corpus_test.go:130);
     - test_notbuilt.py:20.
     - Not affected: `gate/proofs/drivers/test_notbuilt_expired.py` copies the live notbuilt.json and asserts only the S4.1 entries' "a pattern body"; `gate/runner/tests/test_law.py:440` is "(S7)" as an entry's sprint, unrelated.
     - Docs that quote the old text are closed records and stay: history.md:337, s3-2b-1-design.md:24, s3-3-2-record.md:81.
   - **Coupling, both ways:** "a text operator" is not a substring of the old error, and "text operators (S7)" is not a substring of the new one. So `TestCorpus` reds if either side lands alone.
   - Left alone: the comments with sprint ids that are not user text (`expr.go:12-13`, `:120`; `plan.go:75-76`). They are G7, the later part.
5. **Proof (K1):**
   - Plant: `git diff <C2> <C2>~1 -- internal/expr/expr.go internal/expr/registry.go > g7a-3.diff`.
   - `mutate.py --patch g7a-3.diff --test TestBuildRunnable --pkg internal/expr` → **red** at `runnable_test.go:39`.
   - Second witness: `mutate.py --patch g7a-3.diff --test TestCorpus --pkg conformance` → **red** at `corpus_test.go:131` (subtest `05-text-operators.cep`).
   - Then T2 (the pytests row runs test_notbuilt.py) and the proofs row alone at T4: `gate/` and notbuilt.json are touched (the 2026-11-12 rule).
   - LAW Δ shows a notbuilt delta.
6. **Commit:** C2, coordinator, after C1 (both edit registry.go, in different hunks).

#### G7a-6: `New`'s stale doc
1. **Verdict: accepted.**
   - `table.go:56-59` ends "(owner D3: the deploy error for both is S3.3's)".
   - That deploy error exists: `validate/names.go:71-85` (`keyColumn`, E-TBL-002, read), and SPEC.md:79 §4.3 (grep).
2. **Kind: K6** (comment only).
3. **The exact change:** `table.go:56-59` →
   ```go
   // New validates a table declaration and returns the table with an empty current Version. It rejects an empty
   // name; no attributes; an empty or duplicate attribute name (case-sensitive); a type outside TString…TObject; an
   // empty key; and a key column that is not declared, is named twice, or is an object. An optional key column is
   // accepted; a null key value is refused when written. At deploy the validator refuses an optional or an object key
   // column (E-TBL-002, SPEC §4.3).
   ```
4. **Proof (K6):**
   - `go run ./tools/astequal <C6>~1 <C6> internal/table/table.go` → exit 0.
   - `git diff -U0 <C6>~1 <C6> -- internal/table/table.go | grep -c '// Output:'` → 0 (table.go has no Example).
   - **Jargon sweep** (plan §7.1 pattern):
     ```
     python3.15 -I -c 'import re,subprocess,sys; p=re.compile(r"\b[Ss]\d+\.\d+[a-z]?(?:-\d+)?\b|\bowner\b|\bdesign \d|\bdecision \d|break-it|\breview\b|\b[A-Z]{1,2}\d+\b|\(\s*[^)]*\d{4}-\d{2}-\d{2}"); d=subprocess.run(["git","diff","-U0",sys.argv[1]+"~1",sys.argv[1],"--","internal/table/table.go"],capture_output=True,text=True,check=True).stdout; h=[l for l in d.splitlines() if l[:1]==sys.argv[2] and l[:3]!=sys.argv[2]*3 and p.search(l)]; print("\n".join(h) or "clean"); sys.exit(1 if h else 0)' <C6> +
     ```
     - Expected on the added lines (`+`): `clean`, exit 0. "E-TBL-002" does not match `\b[A-Z]{1,2}\d+\b`.
     - The same command with `-` (the removed lines) prints `:59`'s line, hitting "owner", "D3" and "S3.3", exit 1: the red demo.
5. **Commit:** C6, table K6, after C5.

#### G2-85: the `-2` "not asserted" sentinel
1. **Verdict: accepted.**
   - `plan_test.go:21` comments "-2: not asserted (a derived stream: the design is silent, gap G2)". `:45` skips the check, and rows `:66`, `:99`, `:174`, `:175` use -2.
   - The code is not silent:
     - `plan.go:163` starts `timeAttr := event.ArrivalTime` and changes it only for Bounded/Ascending (`:165-175`).
     - `plan.go:29` documents "a derived stream: event.ArrivalTime", and `:160-161` repeats it.
     - Playback depends on it: `encode.go:223` and `recorder.go:94` write a line's `ts` when `TimeAttr() == event.ArrivalTime`. That is decision D2 of S3.2a-1 (decisions.md:373): a derived output's line carries `ts`.
   - GitNexus `context(stream)`: callers `Build` and two internal tests; accesses `ArrivalTime`, `TimeNone`.
2. **Choice (plan §0 item 8).**
   - Input: a derived stream, e.g. `stream O (k string, n int, ts timestamp);`.
   - Candidates: (a) assert `event.ArrivalTime`; (b) assert its `ts` index; (c) leave it unasserted.
   - **Choice: (a).** It is the documented behaviour and the one playback encodes against.
3. **Kind: K2a.**
4. **The exact change** in `internal/plan/plan_test.go`:
   - `:21` → `timeAttr int // the time attribute's index, or event.ArrivalTime: a `time arrival` source, or a derived stream (plan.Stream's doc)`.
   - `:45` → `if s.Schema.TimeAttr() != w.timeAttr {`.
   - `:66`, `:99`, `:174`, `:175`: the `-2` becomes `event.ArrivalTime`.
5. **Batch entries:**
   ```json
   {"id": "G2-85:07", "file": "internal/plan/plan.go", "find": "\ttimeAttr := event.ArrivalTime\n", "replace": "\ttimeAttr := event.ArrivalTime\n\tif s.Time.Kind == model.TimeNone {\n\t\tfor i, a := range s.Attrs {\n\t\t\tif a.Type == event.TTimestamp {\n\t\t\t\ttimeAttr = i\n\t\t\t}\n\t\t}\n\t}\n", "test": "TestBuildExample07", "pkg": "internal/plan", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-85:09", …same plant…, "test": "TestBuildExample09", …, "expect": {"old": "green", "new": "red"}}
   {"id": "G2-85:clauses", …same plant…, "test": "TestBuildPartitionedAndTimeClauses", …, "expect": {"old": "green", "new": "red"}}
   {"id": "G2-85:control", "control": true, "test": "TestBuildExample07", "pkg": "internal/plan", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
   - Why the plant gives old green: `NewSchema` accepts a timestamp index, `Derived` is unchanged, and Build does not read a derived stream's TimeAttr.
   - The changed code starts no goroutine (`t.Parallel` only), so no load run.
6. **Commit:** C4, plan K2.

#### G2-86: `TestBuildNamesABlocksFirstFault` lacks two adjacent pairs
1. **Verdict: accepted.**
   - GitNexus `context(givenList)`: called by `givens`; calls `fieldIndex`, then `values`.
   - `expect.go:126-129` checks the time attribute before the records (`:131-137`).
   - `values` checks the fields (`:198-208`) before the null time (`:209-211`).
   - Row `:160` has `given A [ ]` empty, so moving the check inside `givenList` stays green.
2. **Choice:** the input is the block `given S [ {k: 1} ]` on an unchecked model.
   - Candidates: the field fault first (today's code), or the null time first.
   - **Choice: the field fault**, today's order. The omitted `ts` is met only after every written field, which follows the block's written order.
   - The doc gains that step.
3. **Kind: K2a.**
4. **The exact change** in `internal/plan/expect_test.go`:
   - Doc `:149-152` →
     ```go
     // TestBuildNamesABlocksFirstFault: a block with two faults (a model the validator never saw) is refused for the one
     // met first in the block's order (design s3-3-2 §4, S1.1c-6 Q.2): every given in written order (a table's name, then
     // its rows; a stream's name, its time attribute, then its records), then the then's target, then its records, each
     // record's fields in written order, then a given stream record's time value. One row per adjacent pair of that
     // order, each wanting the earlier fault.
     ```
   - New rows, after `:161`:
     ```go
     		{"a timeless stream before its own record", "given A [ {k: 1} ] then Out [ ]", "stream A has no time attribute a block can give"},
     		{"a record's field before its own null time", "given S [ {k: 1} ] then Out [ ]", "k: a literal of kind int is not of type string"},
     ```
5. **Batch entries** (test `TestBuildNamesABlocksFirstFault`, pkg `internal/plan`; no subtests, the rows are a loop):
   ```json
   {"id": "G2-86:time-before-records", "file": "internal/plan/expect.go", "find": "\tif timeIdx < 0 {\n\t\treturn GivenList{}, fmt.Errorf(\"stream %s has no time attribute a block can give\", g.Name)\n\t}\n\tgl := GivenList{Stream: sid}\n", "replace": "\tgl := GivenList{Stream: sid}\n\tfor _, rec := range g.Records {\n\t\tif _, err := values(st.Attrs, rec, timeIdx); err != nil {\n\t\t\treturn GivenList{}, err\n\t\t}\n\t}\n\tif timeIdx < 0 {\n\t\treturn GivenList{}, fmt.Errorf(\"stream %s has no time attribute a block can give\", g.Name)\n\t}\n", "test": "TestBuildNamesABlocksFirstFault", "pkg": "internal/plan", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-86:fields-before-null-time", "file": "internal/plan/expect.go", "find": "\tfor _, f := range rec.Fields {\n\t\tidx := fieldIndex(attrs, f.Name)\n\t\tif idx < 0 {\n\t\t\treturn nil, fmt.Errorf(\"no attribute %s\", f.Name)\n\t\t}\n\t\tv, err := value(f.Value, attrs[idx].Type)\n\t\tif err != nil {\n\t\t\treturn nil, fmt.Errorf(\"%s: %w\", f.Name, err)\n\t\t}\n\t\tout[idx] = v\n\t}\n\tif timeIdx >= 0 && out[timeIdx].Null {\n\t\treturn nil, fmt.Errorf(\"the time attribute %s is null\", attrs[timeIdx].Name)\n\t}\n\treturn out, nil\n", "replace": "\tvar first error\n\tfor _, f := range rec.Fields {\n\t\tidx := fieldIndex(attrs, f.Name)\n\t\tif idx < 0 {\n\t\t\treturn nil, fmt.Errorf(\"no attribute %s\", f.Name)\n\t\t}\n\t\tv, err := value(f.Value, attrs[idx].Type)\n\t\tif err != nil {\n\t\t\tif first == nil {\n\t\t\t\tfirst = fmt.Errorf(\"%s: %w\", f.Name, err)\n\t\t\t}\n\t\t\tcontinue\n\t\t}\n\t\tout[idx] = v\n\t}\n\tif timeIdx >= 0 && out[timeIdx].Null {\n\t\treturn nil, fmt.Errorf(\"the time attribute %s is null\", attrs[timeIdx].Name)\n\t}\n\tif first != nil {\n\t\treturn nil, first\n\t}\n\treturn out, nil\n", "test": "TestBuildNamesABlocksFirstFault", "pkg": "internal/plan", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-86:control", "control": true, "test": "TestBuildNamesABlocksFirstFault", "pkg": "internal/plan", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
   - Both find texts occur exactly once. The second is pinned by `fieldIndex`; `thenRecord` uses `schemaIndex`.
   - Why the old rows stay green on each plant: every old row whose record has a field fault also gives `ts`, so the order is unchanged. A has no records in the old rows.
6. **Commit:** C4.

#### G2-87: `internal_test.go`'s `itoa`
1. **Verdict: accepted.**
   - `internal_test.go:64-73` re-implements integer formatting and returns "" for a negative. Its one use is `:52`.
   - The file holds tests of scope, state, `addSat` and the unchecked Build.
2. **Kind: K2e** for the helper. Plan §7.2's last rule (a test-internal change allowed now when it changes no outcome) is shown here:
   - On the real code both versions give "anchor/=42", green.
   - On the finding's plant (`body.go:129` `fn("anchor", "", s.anchor.At)` → `fn("anchor", "", -s.anchor.At)`) both are red at `:59`. The old message is "walk [anchor/=]" and the new one "walk [anchor/=-42]", which is now truthful.
3. **The exact change:**
   - `:52` → `walk := func(component, key string, v any) { got = append(got, component+"/"+key+"="+strconv.FormatInt(v.(int64), 10)) }`.
   - Delete `itoa` (`:64-73`).
   - Add `"strconv"` to the imports.
4. **Proof (K2e):** `git grep -n 'func itoa' -- internal/plan/internal_test.go`.
   - Old: `internal/plan/internal_test.go:64:func itoa(i int64) string {` (exit 0, red).
   - New: no output (exit 1, green).
5. **Carried:**
   - The file rename → S1.1c-8. Proposed name `internal/plan/body_internal_test.go`. The name says "body"; the assertions behind it are `scope` and `state.Walk` (body.go).
   - Moving `TestBuildRefusesAnUncheckedModel` into a new `plan_internal_test.go` is a split → S1.1c-10 (Q4).
   - Outcome: **fixed in part**.
6. **Commit:** C4.

#### G2-88: `TestBuildRefusesNotBuilt` never requires a nil plan
1. **Verdict: accepted.** `plan_test.go:197` checks only `errors.Is`. `Build` returns nil on every error path (`plan.go:84-106`).
2. **Kind: K2a.**
3. **The exact change**, `plan_test.go:197-198`:
   ```go
   		if !errors.Is(err, plan.ErrNotBuilt) || p != nil {
   			t.Errorf("Build(%s) = %v, %v; want nil and ErrNotBuilt", file, p, err)
   		}
   ```
4. **Batch entries:**
   ```json
   {"id": "G2-88", "file": "internal/plan/plan.go", "find": "\t\t\treturn nil, fmt.Errorf(\"plan: rule %s: %w\", app.Rules[i].Name, err)\n", "replace": "\t\t\treturn p, fmt.Errorf(\"plan: rule %s: %w\", app.Rules[i].Name, err)\n", "test": "TestBuildRefusesNotBuilt", "pkg": "internal/plan", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-88:control", "control": true, "test": "TestBuildRefusesNotBuilt", "pkg": "internal/plan", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
   No goroutines in the test.
5. **Commit:** C4.

#### G2-133: the ordering property is never checked
1. **Verdict: accepted.**
   - `breakit2_test.go:82-94`: `prev` is assigned and then discarded (`_ = prev`).
   - GitNexus `context`: calls only `NewSet`, `Set.Snapshot`, `Snapshot.Len`/`Table`, `Version.Get`. No order assertion.
   - `Set.Snapshot` is at `table.go:386-397`.
2. **Kind: K2a, plus a load run** (the test runs goroutines).
3. **The exact change:** replace `:53-97` with the following. The defer also stops the writers when a check fails; today's `t.Fatal` at `:88` leaves them spinning.
   ```go
   // TestBreakitGuardSnapshotUnderConcurrentWriters: writers on other goroutines (Upsert, Delete, Load) against
   // Set.Snapshot on one goroutine; run with -race. Every snapshot is internally stable: a version read twice from one
   // snapshot is the same, and a later snapshot never shows an older version of a table than an earlier one did. Each
   // table's one stamping writer puts its own rising count in row "g" with every Upsert and Load, so a table's "g"
   // never falls from one version to a later one; A's second writer upserts and deletes only row "k".
   func TestBreakitGuardSnapshotUnderConcurrentWriters(t *testing.T) {
   	a, b := bkTable(t, "A"), bkTable(t, "B")
   	set := table.NewSet([]*table.Table{a, b})
   	var wg sync.WaitGroup
   	stop := make(chan struct{})
   	defer func() { // a failed check stops the writers before the test returns
   		close(stop)
   		wg.Wait()
   	}()
   	stamp := func(tb *table.Table, w int64) func(int64) {
   		return func(i int64) {
   			switch (i + w) % 3 {
   			case 0:
   				_ = tb.Upsert([][]event.Value{bkRow("g", i), bkRow("k", i)})
   			case 1:
   				_ = tb.Delete([][]event.Value{{{Kind: event.TString, Str: "k"}}})
   			default:
   				_ = tb.Load([][]event.Value{bkRow("g", i), bkRow("k", i), bkRow("j", i)})
   			}
   		}
   	}
   	keyOnly := func(i int64) {
   		if i%2 == 0 {
   			_ = a.Upsert([][]event.Value{bkRow("k", i)})
   		} else {
   			_ = a.Delete([][]event.Value{{{Kind: event.TString, Str: "k"}}})
   		}
   	}
   	for _, write := range []func(int64){stamp(a, 0), stamp(b, 1), keyOnly} {
   		wg.Add(1)
   		go func() {
   			defer wg.Done()
   			for i := int64(0); ; i++ {
   				select {
   				case <-stop:
   					return
   				default:
   				}
   				write(i)
   			}
   		}()
   	}
   	gen := func(v *table.Version) int64 { // the stamp in row "g"; -1 before the first stamped write
   		row, ok := v.Get([]event.Value{{Kind: event.TString, Str: "g"}})
   		if !ok {
   			return -1
   		}
   		return row[1].I
   	}
   	prev := set.Snapshot()
   	for range 20000 {
   		s := set.Snapshot()
   		for id := range s.Len() {
   			v := s.Table(id)
   			if v != s.Table(id) {
   				t.Fatal("a snapshot's version changed")
   			}
   			if now, was := gen(v), gen(prev.Table(id)); now < was {
   				t.Fatalf("table %d: a later snapshot shows stamp %d after an earlier one showed %d", id, now, was)
   			}
   		}
   		prev = s
   	}
   }
   ```
4. **Batch entries:**
   ```json
   {"id": "G2-133", "file": "internal/table/table.go", "find": "\ts.last = &Snapshot{versions: versions}\n\treturn s.last\n}\n", "replace": "\ts.last = &Snapshot{versions: versions}\n\tif plantOld == nil {\n\t\tplantOld = s.last\n\t}\n\tif plantN++; plantN%2 == 0 {\n\t\treturn plantOld\n\t}\n\treturn s.last\n}\n\nvar (\n\tplantOld *Snapshot\n\tplantN   int\n)\n", "test": "TestBreakitGuardSnapshotUnderConcurrentWriters", "pkg": "internal/table", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-133:control", "control": true, "test": "TestBreakitGuardSnapshotUnderConcurrentWriters", "pkg": "internal/table", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
   - What the plant does: every second fresh build returns the first snapshot ever taken.
   - Why the old test stays green: it checks only within one snapshot.
   - Why the new test goes red: a stamp falls once the reader has seen about four changes.
   - The plant's variables are touched only by the one snapshot goroutine.
   - **Load run (green):** `python3.15 -I -B gate/bench/load.py -- -race -count=50 -run '^TestBreakitGuardSnapshotUnderConcurrentWriters$' ./internal/table` → `50/50`.
5. **Carried:** the rename to `TestSetSnapshotIsStableUnderConcurrentWriters` and the move to `set_test.go` (same package; it goes with the file rename) → S1.1c-8.
   - "Is stable" is backed by the `v != s.Table(id)` check.
   - The proposed name does **not** name the never-falls check. S1.1c-8 should cover it, e.g. "…NeverGoesBack…".
   - Outcome: **fixed in part**.
6. **Commit:** C7, table K2.

#### G2-135: any diagnostic passes
1. **Verdict: accepted.**
   - `s3_2c_breakit_test.go:30` checks only `len(diags) == 0`.
   - GitNexus `context(zeroDurations)`: caller `bounds`. `bounds.go:216-217` reports E-DUR-003 for `d.Ms < 0`.
   - For this program nothing else reports: `resolve.go:136-150` substitutes −5000, and there is no partition, so no ttl check. So exactly one diagnostic.
2. **Kind: K2a.**
3. **The exact change**, `:30-32`:
   ```go
   	if len(diags) != 1 || diags[0].Code != "E-DUR-003" || diags[0].Lint {
   		t.Fatalf("diagnostics %+v; want exactly one E-DUR-003, a negative bound refused at deploy as a zero one is", diags)
   	}
   ```
   `app` is then unused. Write the `Check` call as `_, diags := validate.Check(…)`.
4. **Batch entries:**
   ```json
   {"id": "G2-135", "file": "internal/validate/bounds.go", "find": "c.report(\"E-DUR-003\", d.Pos, \"a bound slot of rule %s is %s\"", "replace": "c.report(\"E-DUR-002\", d.Pos, \"a bound slot of rule %s is %s\"", "test": "TestBreakitANegativeDurationOverrideInABoundSlotIsRefused", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-135:control", "control": true, "test": "TestBreakitANegativeDurationOverrideInABoundSlotIsRefused", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
5. **Carried:**
   - The rename → S1.1c-8. The proposed subtest name is "a negative rate-limit override"; the assertion behind "refused" is the single E-DUR-003.
   - Moving it as a subtest into another file is a merge, and a move across packages if the target is `package validate` → S1.1c-10.
   - Outcome: **fixed in part**.
6. **Commit:** C8, validate K2.

#### G2-136: the row's input never reaches its path
1. **Verdict: accepted.**
   - `units_test.go:21` declares R as `(k string, m int, ts timestamp)`: no optional attribute and no `n`. So `:66`'s E-NAME-001 is for the unknown `n`.
   - GitNexus `context(nullFinding)`: its only caller is `nulls`. The content shows `case inAbsence:` reports E-NULL-001 (`nulls.go:63-66`) and `default:` reports E-NULL-002 (`:67-69`).
2. **Kind: K2b.**
3. **The exact change.** The program is rewritten over S's optional `note`, so the name is true. Its shape is that of operands_test.go:118, "less_inside_absence", which alone gives E-NULL-001. The old input stays as a new truthful row, so no assertion is lost and no K2-del is needed. `units_test.go:66-67` →
   ```go
   		{"!= over an optional attribute in an absence: no E-NULL-002", "rule P intent 'x' partition by k of S, k of R ttl 1 hour from every a = R and not S[note != 'x'] for 1 min within 5 min after match skip past last late events drop select a.k insert into Out;\n",
   			"E-NULL-001"},
   		{"an unknown attribute in an absence filter: one E-NAME-001", "rule P intent 'x' partition by k of S, k of R ttl 1 hour from every a = S and not R[k != 'x' and n > 1] for 1 min within 5 min after match skip past last late events drop select a.k insert into Out;\n",
   			"E-NAME-001"},
   ```
4. **Batch entries:**
   ```json
   {"id": "G2-136", "file": "internal/validate/nulls.go", "find": "c.report(\"E-NULL-001\", n.Pos, \"comparison over the optional attribute", "replace": "c.report(\"E-NULL-002\", n.Pos, \"comparison over the optional attribute", "test": "TestOneFindingPerDefect", "subtest": "!= over an optional attribute in an absence: no E-NULL-002", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-136:control", "control": true, "test": "TestOneFindingPerDefect", "subtest": "!= over an optional attribute in an absence: no E-NULL-002", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
   - **K2b coverage.** The promised branch's first statement is `nulls.go:64` (`c.report("E-NULL-001", …)`, in the block that starts at 63.17).
   - Run at each revision: `go test ./internal/validate -count=1 -run '^TestOneFindingPerDefect$/^!=_over_an_optional_attribute_in_an_absence:_no_E-NULL-002$' -covermode=count -coverprofile=<f>`, then `grep 'internal/validate/nulls.go:63\.' <f>`.
   - Expected: the count for the block covering :64 is 0 at old and > 0 at new. Quote the block's start and count.
5. **Carried:** nothing. The program was rewritten instead of renaming the row.
6. **Commit:** C8.

#### G2-137: "count() in a filter: no type error" cannot fail its second half
1. **Verdict: accepted.**
   - `S[count() > 1]` types even when it is built: `buildCount` returns an int node (`registry.go:187-189`).
   - GitNexus `context(callPlacement)`: its only caller is `placement`.
   - `hiding_test.go:60-62`, H3 (`count() > 'a'`), asserts the same code, plus that the build is skipped, plus the position.
2. **Kind: K2-del.** The row is a weaker duplicate of H3. Making it `> 'a'` would copy H3, and renaming is S1.1c-8's.
3. **The exact change:** delete `units_test.go:72` (the row).
4. **Batch entries:**
   ```json
   {"id": "G2-137", "file": "internal/validate/exprs.go", "find": "c.report(\"E-FN-001\", n.Pos, \"%s() is an aggregate", "replace": "c.report(\"E-FN-002\", n.Pos, \"%s() is an aggregate", "old_test": "TestOneFindingPerDefect", "old_subtest": "count() in a filter: no type error", "new_test": "TestOneFindingHidesAnother", "new_subtest": "H3 count() in a filter: no type finding for its comparison", "pkg": "internal/validate", "old_rev": "<parent of this commit>", "new_rev": "<this commit>", "expect": {"old": "red", "new": "red"}}
   {"id": "G2-137:control", "control": true, "old_test": "TestOneFindingPerDefect", "old_subtest": "count() in a filter: no type error", "new_test": "TestOneFindingHidesAnother", "new_subtest": "H3 count() in a filter: no type finding for its comparison", "pkg": "internal/validate", "old_rev": "<parent of this commit>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
   One assertion is removed (the codes `[E-FN-001]`), so one plant.
5. **Commit:** C9, validate K2-del.

#### G2-138: "at the declaration" is never asserted
1. **Verdict: accepted.**
   - `units_test.go:82` compares codes only.
   - GitNexus `context(defaults)`: caller `resolve`. `resolve.go:30` reports at `p.Pos`, the declaration.
2. **Kind: K2a.**
3. **The exact change:**
   - Delete row `:70` from the table.
   - After the loop (before `:87`), add the same name as a standalone subtest. This is not a rename. `codesAt` and `posOf` are from `hiding_test.go`, same package.
     ```go
     	t.Run("a bad param default: reported once, at the declaration", func(t *testing.T) {
     		t.Parallel()
     		src := base + "param bad int = 'x';\nrule P intent 'x' from S[n > ${bad}] select k insert into Out;\n"
     		_, diags := Check(parse(t, src), Options{})
     		if g, want := strings.Join(codesAt(diags), " "), "E-PARAM-001@"+posOf(t, src, "param bad", 1, 0); g != want {
     			t.Fatalf("got [%s], want [%s]", g, want)
     		}
     	})
     ```
4. **Batch entries:**
   ```json
   {"id": "G2-138", "file": "internal/validate/resolve.go", "find": "c.report(\"E-PARAM-001\", p.Pos, \"param %s is declared", "replace": "c.report(\"E-PARAM-001\", model.Pos{}, \"param %s is declared", "test": "TestOneFindingPerDefect", "subtest": "a bad param default: reported once, at the declaration", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-138:control", "control": true, "test": "TestOneFindingPerDefect", "subtest": "a bad param default: reported once, at the declaration", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
   resolve.go already imports model.
5. **Commit:** C8.

#### G2-139: only `Lint` is asserted
1. **Verdict: accepted.**
   - `units_test.go:277` checks `len(diags) != 1 || !diags[0].Lint || out == nil`.
   - `Lint` is true only for E-NULL-002 (`validate.go:133`), so a plant must change both the code and the lint rule.
2. **Kind: K2a.**
3. **The exact change**, `:277-279`:
   ```go
   	if len(diags) != 1 || diags[0].Code != "E-NULL-002" || !diags[0].Lint || out == nil {
   		t.Fatalf("got %v, app %v; want one finding, the lint E-NULL-002, and the app", diags, out != nil)
   	}
   ```
4. **Batch entries** (a patch):
   ```json
   {"id": "G2-139", "patch": "--- a/internal/validate/nulls.go\n+++ b/internal/validate/nulls.go\n@@ -67,3 +67,3 @@\n \tdefault:\n-\t\tc.report(\"E-NULL-002\", n.Pos, \"%s is optional, and `%s` is false when it is null; wrap it in coalesce(...) \"+\n+\t\tc.report(\"E-NULL-001\", n.Pos, \"%s is optional, and `%s` is false when it is null; wrap it in coalesce(...) \"+\n \t\t\t\"or test `is null` explicitly\", strings.Join(offenders, \", \"), opName(n))\n--- a/internal/validate/validate.go\n+++ b/internal/validate/validate.go\n@@ -132,3 +132,3 @@\n \t\tCode: code, Line: at.Line, Col: at.Col, Msg: fmt.Sprintf(format, args...),\n-\t\tFix: catalog[code].Fix, Lint: code == \"E-NULL-002\",\n+\t\tFix: catalog[code].Fix, Lint: code == \"E-NULL-001\" || code == \"E-NULL-002\",\n \t})\n", "test": "TestALintDoesNotRefuse", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-139:control", "control": true, "test": "TestALintDoesNotRefuse", "pkg": "internal/validate", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
   ```
5. **Commit:** C8.

#### G2-140: `MarshalCanonical`'s error is dropped
1. **Verdict: accepted.** `units_test.go:257` reads `after, _ := app.MarshalCanonical()`. A failed encoding would report "Check wrote the App it was given".
2. **Kind: K2e.** The outcome on a plant is red both before and after, with only the message differing, so K2a does not fit.
3. **The exact change**, `:257`:
   ```go
   	after, err := app.MarshalCanonical()
   	if err != nil {
   		t.Fatal(err)
   	}
   ```
4. **Proof:** `git grep -nE ',\s*_\s*:?=\s*[A-Za-z_.]*MarshalCanonical\(' -- internal/validate/units_test.go`.
   - Old: `internal/validate/units_test.go:257:	after, _ := app.MarshalCanonical()` (exit 0, red).
   - New: no output (exit 1, green).
5. **Commit:** C8.

### 4. Commit plan (ordered)

| # | Commit | Findings | Files | Proofs |
|---|---|---|---|---|
| C1 | expr K1 (lane) | G10-15 | expr/registry.go, expr/nodes_test.go | mutate `--patch` revert → red; T2 |
| C2 | coordinator coupling, K1 | G7a-3 | expr/expr.go, expr/registry.go:188, expr/runnable_test.go, conformance/notbuilt.json, gate/runner/tests/test_notbuilt.py | mutate `--patch` → red on TestBuildRunnable and TestCorpus; T2; proofs row alone at T4; LAW Δ notbuilt |
| C3 | plan K1 | G10-17 | plan/expect.go, literalvalue_test.go, expect_test.go, coverage_s3_3_2_test.go | mutate find/replace → red; T2 |
| C4 | plan K2 | G2-85, -86, -87, -88 | plan/plan_test.go, expect_test.go, internal_test.go | `--expect` entries above; G2-87's K2e grep |
| C5 | table K1 | G10-22 | table/table.go, table_test.go | mutate `--patch` → red; T2 |
| C6 | table K6 | G7a-6 | table/table.go (comment) | astequal exit 0; no `// Output:`; jargon sweep clean (red on the old line) |
| C7 | table K2 | G2-133 | table/breakit2_test.go | `--expect`; load.py 50/50 |
| C8 | validate K2 | G2-135, -136, -138, -139, -140 | validate/s3_2c_breakit_test.go, units_test.go | `--expect` (G2-136 with K2b coverage); G2-140 grep |
| C9 | validate K2-del | G2-137 | validate/units_test.go | `--expect` red/red |

- **Count:** 8 lane commits and 1 coordinator commit.
- Other CD chunks with commits of the same package × kind (expr K1, plan K1/K2, validate K2) should be merged into one commit each by the coordinator.
- C3 must precede C4. C5 and C6 can go in either order.

### 5. Choices made (each "chosen by the design note")

1. **G10-15**
   - Input: `Register(Descriptor{Kind: Scalar, Factory: nil})`, and MinArgs 2 / MaxArgs 1.
   - Candidates: refuse in Register / error in `builder.call` / reject.
   - Chosen: refuse in Register, with exact texts "expr: <name>: a nil Factory; only a pattern-only callable has none" and "expr: <name>: MinArgs <m> is above MaxArgs <n>".
   - Reason: it fails where the descriptor is added.
   - Dry run: no existing pin; the two test registrations still pass.
2. **G10-17**
   - Input: `value(LitInt, TObject)`.
   - Candidates: local `article` / fixed-article wording / shared helper (a move).
   - Chosen: "a literal of kind int is not of type object".
   - Dry run: 72 pins in 3 files (§3).
3. **G10-22**
   - Input: `table.New("", …)`, `New("T", nil, …)`, `New("T", attrs, nil)`.
   - Candidates: drop the sentinels / export `ErrInvalidTable` / reject.
   - Chosen: drop them, with the text byte-identical.
   - Dry run: `git grep` finds no pin of the three texts.
4. **G7a-3**
   - Input: the deploy of 05-text-operators.cep; `BuildRunnable` on count() and chain().
   - Candidates: survey's plural phrases plus "not supported in this release" / singular phrases / only cut the ids.
   - Chosen: "a text operator", "count()", "chain()".
   - Dry run: runnable_test.go:13, :22-26; notbuilt.json:17 via corpus_test.go:130; test_notbuilt.py:20.
5. **G2-85**
   - Input: a derived stream's TimeAttr.
   - Chosen: assert `event.ArrivalTime` (plan.go:29; D2, decisions.md:373; encode.go:223).
6. **G2-86**
   - Input: `given S [ {k: 1} ]` on an unchecked model.
   - Chosen: the field fault before the null time (today's order).
7. **G7a-6** wording: "At deploy the validator refuses an optional or an object key column (E-TBL-002, SPEC §4.3)."

### 6. Couplings and risks

- **Plan §0 item 5, not-built constructs:** touched by G7a-3, as one coordinator commit with the three-file edit above. LAW Δ shows notbuilt.json. The proofs row runs alone at T4 after it.
- **Other owners' files I need:**
  - `conformance/notbuilt.json` (coordinator);
  - `gate/runner/tests/test_notbuilt.py` (coordinator, per §0 item 5).
  - No other file outside CD's packages.
- **Law paths:** `internal/validate/docsync_test.go` and `internal/expr/docsync_test.go` are not touched. No `Register`, `Factory` or `later` reference in either (grep).
- **R5 (per-event code):** no finding is K1h.
  - G10-15: install-time only (shard.go:141-143, build.go:46).
  - G10-17: Build-time, plus the error-text exemption.
  - G10-22: deploy/start/replay-start, plus the error-branch exemption.
  - G7a-3: the error-text exemption; `later.Eval`'s code is unchanged.
  - Plants in per-event code (`Set.Snapshot`) are proofs, not commits.
- **R4 frozen benchmarks:** none touched. `table/checkvalue_bench_test.go` and `plan/body_bench_test.go` are unchanged.
- **§0 item 4 frame tables:** none in these packages.
- **Pre-mortem (the-fool):**
  1. G2-133's red needs the reader to see about four fresh builds with a rising stamp in 20,000 iterations. That is certain with parallel writers, but on a 1-CPU run the writers may not interleave. If the `--expect` reads new **green**, the lane stops and reports rather than tuning the count.
  2. G2-135 and G2-136 assert exact finding lists from reasoning (bounds.go, resolve.go; operands_test.go:118's twin). The control entries catch an extra finding at build.
  3. G7a-3: if any corpus file other than 05 stopped on count() or chain(), TestCorpus would expose it. Today none does: windows and patterns are refused first (plan.go:194-195).
  4. G10-15 is HIGH and G10-22 CRITICAL in GitNexus. The built-in table and the unchanged texts are the safety arguments, quoted above.
- **Not checked:** index freshness through the resource (git used instead); any test run; W0's final `--patch`/`--subtest`/id syntax. Entry ids use `<finding>:<tag>` suffixes; adapt them to W0's id rule.

### 7. Carried items

- **G2-87:**
  - The file rename to `internal/plan/body_internal_test.go` → S1.1c-8 (asserts `scope` and `state.Walk`).
  - Moving `TestBuildRefusesAnUncheckedModel` into a new `plan_internal_test.go` → S1.1c-10, a split under Q4.
- **G2-133:** rename `TestBreakitGuardSnapshotUnderConcurrentWriters` → `TestSetSnapshotIsStableUnderConcurrentWriters`, moved to `set_test.go` → S1.1c-8. "Is stable" is backed by the within-snapshot check; the never-falls check should also be named.
- **G2-135:**
  - The rename of `TestBreakitANegativeDurationOverrideInABoundSlotIsRefused` → S1.1c-8. Proposed subtest name "a negative rate-limit override"; "refused" is backed by exactly one E-DUR-003.
  - Its move into another file as a subtest → S1.1c-10 (merge or cross-package move, Q4).
- **No renames were made.** New names: the test `TestRegisterRefusesADescriptorBuildCannotCall`, and the rows "an unknown attribute in an absence filter: one E-NAME-001", "a timeless stream before its own record" and "a record's field before its own null time".
- **Noted for the later part (G7, not G7a):** sprint-id comments left untouched at `expr.go:12-13`, `:120`, `plan.go:75-76` and `table.go:6`.

I made no file edits, git writes or test runs, so I did not run `detect_changes`.


## Addendum: G2-134's plan half, and the coordinator's corrections

Addendum to the CD2 design note: G2-134's plan half

This adds G2-134's plan half to commit C4 (plan K2). The commit count is unchanged at 8 lane commits and 1 coordinator commit. CD1's note takes the internal/text half.

**Summary-table row:**

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G2-134's plan half | accepted | K2e | C4 plan K2 | – (CD1 has the text half) | `TestBuildCarriesExpects` gains `t.Parallel()` as its first statement |

**1. Verdict: accepted.**
- The survey finding is `docs/reviews/s1-1c-7-survey-findings.md:190`. For plan it says: "TestBuildCarriesExpects is the only test in its file without it" (testing.md:13).
- I checked this in `internal/plan/expect_test.go`. The four top-level tests start at `:35`, `:153`, `:182` and `:196`. The three at `:153`, `:182` and `:196` call `t.Parallel()` on their first line. `TestBuildCarriesExpects` at `:35` does not, and its doc gives no reason.
- It is safe to run in parallel. Its subtests ("07", "09", "widening, nulls and text_invisibles", at `:36`, `:64`, `:81`) do three things:
  - read the example files through `peExample` → `pqBuild` → `os.ReadFile`;
  - parse source through `peDecls`;
  - read package-level values that never change: `pqT0` is a `var` that is never assigned (`helpers_test.go:32`), and the other `pq*` names are `const`s (`helpers_test.go:25-34`).
- Nothing writes shared state: no `Setenv`, `Chdir` or `AllocsPerRun`, and no package-level variable is assigned (grep).
- `TestBuildExample07` and `TestBuildExample09` (`plan_test.go:61-63`, `:94-96`) already read the same files through the same helpers while running in parallel.
- So the right fix is `t.Parallel()`, not a "Not parallel:" line.

**2. Kind: K2e** (the `t.Parallel` shape, plan §7.2). It is test-only, with no production plant.

**3. The exact change.** In `internal/plan/expect_test.go`, insert one line after `:35` (`func TestBuildCarriesExpects(t *testing.T) {`):
```go
	t.Parallel()
```
- The subtests stay serial within the test, as they are now.
- This shifts the file's later line numbers by +1. G2-86's references (`:149-171`) are applied by text in the same commit, so nothing breaks.

**4. The proof (K2e).** The check lists every top-level test in the file whose first statement is not `t.Parallel()` and whose doc block has no "Not parallel:" line:
```
python3.15 -I -c 'import itertools,re,sys; L=open(sys.argv[1],encoding="utf-8").read().split("\n"); bad=[f"{sys.argv[1]}:{i+1}: {m.group(1)}" for i,l in enumerate(L) if (m:=re.match(r"func (Test\w+)\(t \*testing\.T\) \{$",l)) and L[i+1].strip()!="t.Parallel()" and not any("Not parallel:" in c for c in itertools.takewhile(lambda x:x.startswith("//"),reversed(L[:i])))]; print("\n".join(bad) or "clean"); sys.exit(1 if bad else 0)' internal/plan/expect_test.go
```
- **On the old file:** I ran this read-only scan at HEAD 1f4ebba. It printed `internal/plan/expect_test.go:35: TestBuildCarriesExpects` and exited 1, so it is **red**.
- **On the new file:** expected `clean` with exit 0, so **green**.
- **Load run:** none. The test body starts no goroutine; `t.Parallel()` is only scheduling by the runner. The T2 `-race` run of internal/plan covers it running beside its parallel siblings, which share only the read-only values above.

**5. Commit:** C4, plan K2, beside G2-85, G2-86, G2-87 and G2-88.
- C4's files are unchanged: `plan_test.go`, `expect_test.go`, `internal_test.go`.
- Add this check to C4's proof list.

**Ledger:** "G2-134's plan half": fixed (K2e) in C4. The finding's id is fully fixed only when CD1's text half lands; the coordinator records the two halves under G2-134.

Nothing else in the note changes. As before, I made no file edits, git writes or test runs; the only command was the read-only Python scan above.

*(Coordinator, 2026-11-20, after the design reviews: the serial-test marker is the repo's existing `Not parallel: <reason>` (DA-S6, widened); the check above now looks for it, and CD1's G2-73 §4 script is the one shared check for every note.)*

---

### Coordinator corrections to design-CD2.md after the pre-check (2026-11-20; evidence: precheck-1.md)

1. **G2-136's coverage line:** the block is `nulls.go:64.3,66.10`, so grep for `nulls.go:64⧵.` (⧵ = backslash). The old count is 0, as predicted. The note's `nulls.go:63.17` matches nothing.
2. **G2-87:** the failing line is `internal_test.go:60` (the Errorf), not :59. The message matches the prediction.
3. **Pre-checked:** every other CD2 old-side prediction holds (9 runs), and all controls pass.
