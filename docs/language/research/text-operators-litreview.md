# Prior art: deterministic free-form-text matching operators in rule and detection languages

Literature/prior-art review supporting the design of first-class deterministic text operators
(keyword, contains, regex) in our CEP rule language for AI-agent fleet detection. Gathered online
2026-10-31; every claim carries a numbered citation into the [references](#references). This
document records evidence only; it proposes no operator design.

Scope note on sources: operator semantics are taken from the vendors' current reference
documentation; "praise/regret" items come from postmortems, performance guides, backend-converter
issues and benchmark papers. Where a claim rests on a secondary source (community thread,
third-party reimplementation) that is flagged inline.

---

## 1. Security detection-rule languages

### 1.1 Sigma

Matching model: field-value pairs; values are matched **case-insensitively by default** with `*`
(multi-char) and `?` (single-char) wildcards; modifiers are appended to field names with `|` [1][2].

String-match modifiers (spec v2.1.0) [1][2]:

- `contains` — "Puts `*` wildcards around the values, such that the value is matched anywhere in
  the field." `startswith` / `endswith` — value expected at the beginning/end of the field
  (implemented by inserting a wildcard on one side).
- `cased` — opt-in case-sensitive matching; "Default Sigma behavior is case-insensitive matching."
  Some backends ignore `cased` (Splunk SPL and Elasticsearch are named as having no suitable
  case-sensitive operator) [1].
- `re` — regex, **case-sensitive by default** (opposite default to plain values), PCRE-style with a
  deliberately limited metacharacter set; sub-modifiers `re|i` (ignore case), `re|m` (multiline
  `^`/`$`), `re|s` (dot matches newline) [2].
- `all` — switches a value list from OR to AND; forbidden on single values [2].
- Transformation modifiers: `base64`, `base64offset` (base64 has 3 possible alignments, so
  `base64offset` is "usually preferred") [1]; `utf16le`/`utf16be`/`utf16`/`wide` (UTF-16 encodings;
  must be chained into a base64 modifier) [1][2]; `windash` — "creates all possible permutations of
  the `-`, `/`, `–` (en dash), `—` (em dash) and `―` (horizontal bar) characters" so one rule covers
  attacker flag-character swaps [2].
- Others: `exists`, `fieldref` (compare against another field), `cidr`, `lt/lte/gt/gte`, `expand` [2].

Design friction observed in the ecosystem (backend conversion costs of the case-insensitive
default):

- The pySigma Loki backend had to wrap every value in a `(?i)` regex because LogQL equality is
  case-sensitive; this is slower, and a `case_insensitive` toggle flag was added to let users trade
  correctness-to-spec for speed [3].
- The rsigma Fibratus backend notes that case-insensitive matching needs a dedicated operator
  switch, not a value wrapper, and that literal (non-wildcard) values should compile to plain string
  equality because it "evaluate[s] more efficiently" than glob matching [4].
- The spec has no normative escaping section; wildcard/escape wording is scattered through modifier
  descriptions [2] (observed gap, not a cited criticism).

### 1.2 YARA

String types [5]: text strings (double-quoted, C-style escapes, **case-sensitive by default**), hex
strings (wildcards `??`, jumps `[4-6]`, alternatives), and regexes (`/.../` with trailing `i`, `s`;
YARA's own engine — no backreferences, no capture groups, no POSIX classes).

Modifiers [5]:

- `nocase` — case-insensitive for text and regex strings; cannot combine with `xor`/`base64`.
- `wide` — "interleaves zero bytes" to emulate UTF-16LE for ASCII text only; `ascii` restores
  plain-ASCII matching alongside `wide`.
- `fullword` — the match "must not be preceded or followed by an alphanumeric character"
  (`domain` does not match `www.mydomain.com`, does match `www.my-domain.com`).
- `xor` (single-byte keyspace, optionally ranged), `base64`/`base64wide` (3 alignment permutations,
  custom alphabets), `private` (match hidden from output).
- Conditions: `any/all/none of them`, `N of (...)`, `#count`, `@offset[i]`, `!length[i]`, `at`,
  `in (range)`, `for..of` loops [5].

Measured/operational guidance (community performance guide, widely used in DFIR) [6]:

- YARA extracts ≤4-byte "atoms" from each string and searches those first; "slowing down scanning
  is caused by strings that are generating too short atoms, or none at all" — atom-less patterns
  (e.g. `/\w.*\d/`) are evaluated "at every offset of the file".
- "Avoid the `nocase` attribute if possible, because many atoms will be generated and searched
  for" (all case permutations of the atom); for one variable letter prefer `/[Pp]assword/`.
- Avoid greedy/unbounded regex (`.*`, `.{2,}`); regexes are all fed to the string engine up front
  and "do not short-circuit" even when other conditions would rule the file out.

### 1.3 Snort / Suricata

`content` matches raw bytes; "Content matches are case-sensitive by default" (Snort 3) [12];
Suricata likewise: "case-sensitive by default... exact unless `nocase`" [7]. Non-printables and
syntax characters are written in hex between pipes (`|0D|`); `;`, `\`, `"` must be escaped or
hex-encoded [7][11]. `content:!"..."` negates. Positional modifiers: `depth`, `offset` (absolute),
`distance`, `within` (relative to previous match), and Suricata `startswith`/`endswith` buffer
anchors [7]. `nocase` is per-content, applies also to hex bytes [12].

`pcre` adds full regex with `/i /s /m` flags plus Suricata-specific flags (`R` relative match,
buffer selectors). The Suricata manual is blunt: "The complexity of pcre comes with a high price
though: it has a negative influence on performance" and recommends pairing every pcre with a
content prefilter so "the content has to match first, before pcre will be checked" [7].

Multi-pattern matcher (MPM) and `fast_pattern`:

- One content per rule is entered into a shared MPM; only rules whose fast pattern fires are fully
  evaluated. Default selection (Suricata): buffer priority, then "the longest (in terms of
  character/byte length) content match", then higher byte-diversity "pattern strength"; negated
  contents are ignored [8].
- Snort 3: "The ideal fast pattern is one which, if found, is very likely to result in a rule
  match"; the default longest-content choice "is sometimes not the most unique", and bad choices
  "cause Snort to work hard with little to show for it" — hence the explicit `fast_pattern`
  override [13].
- MPM algorithm is pluggable: Aho-Corasick (`ac`, `ac-ks`) by default, Hyperscan (`hs`) "the best
  option" where available [9][10].

### 1.4 Splunk SPL

- Raw search: "By default, searches are case-insensitive" [14]. Events are tokenized at index time
  by **major breakers** (space, comma, brackets, quotes, `|`, `;`, newlines, many URL-encoded
  forms) and **minor breakers** (`.`, `_`, `=`, `/`, `:` ...) per `segmenters.conf` [15].
- `TERM(x)` matches x as a single indexed term; it "ignores minor breakers" but "the term you are
  searching for cannot contain major breakers". Documented pitfall: for `ip=127.0.0.1 - user=admin`
  the search `TERM(127.0.0.1)` fails because `=` is a minor breaker and the indexed term is
  `ip=127.0.0.1` [14].
- `CASE(x)` opts into case-sensitive matching for a term/field value [14].
- Eval functions [16]: `like(str, pattern)` — SQL `%`/`_` wildcards; `match(str, regex)` — regex,
  **unanchored** (the doc's full-match example adds `^`/`$` itself); `searchmatch("...")` — applies
  raw-search semantics to the event; `in()`, `cidrmatch()`.
- Measured: a Splunk .conf20 session reports that adding TERM() "eliminated all false positives,
  with scan_count equal to event_count" for an IP search, and that TERM appears "in less than 1% of
  all customer searches executed on Splunk Cloud" — i.e. the fast tokenized operator exists but
  users don't discover it [17]. Indexed-token mechanics (lispy, bloom filters: false positives
  possible, false negatives not) are laid out in the .conf17 "Fields, Indexed Tokens and You"
  deck [18].

### 1.5 Elastic: ES queries, EQL, KQL

- Analyzed vs exact: the reference warns "Avoid using the term query for text fields... Elasticsearch
  changes the values of text fields as part of analysis" (tokenizing, lowercasing); `match`
  analyzes the query term the same way, `term` does not [19].
- EQL (the security/sequence language): "most operators are case-sensitive"; `==`/`!=` take no
  wildcards; `:` is case-insensitive string equality **with** `*`/`?` wildcards; `like`/`regex` are
  case-sensitive with `like~`/`regex~` as case-insensitive variants [20]. Case choice is per
  operator via the `~` suffix convention.
- KQL (query bar): terms on `text` fields go through analysis ("the order of the search terms
  doesn't matter"), while `keyword` fields "must match exactly, including punctuation and case";
  `*` wildcard on values and field names; leading wildcards allowed but flagged slow and
  centrally disable-able; special characters `\():<>"*{}` escape with backslash; a quoted
  asterisk is literal [21]. No regex in KQL [21].
- Regret/response evidence: Elastic shipped a dedicated `wildcard` field type (7.9) because
  security practitioners needed grep-like partial matching that neither `text` (punctuation
  tokenization loses substrings) nor `keyword` (256-char ignore limit) served; it indexes all
  3-character n-grams, uses them to approximate, then verifies candidates to "remove false
  positives" [22]. Community threads show the complementary failure: a `text` analyzer splitting a
  token so `KCC` could not be found [22].

### 1.6 Microsoft Kusto (KQL)

The clearest tokenized-vs-substring prior art [23]:

- Terms: "each `string` value is broken into maximal sequences of alphanumeric characters, and each
  of those sequences is made into a term"; a term index holds terms of **3+ characters**.
- `has` = whole-term match using the index; `contains` = substring, falls back to a column scan:
  "`\"KustoExplorerQueryRun\" has \"Explorer\"` will return `false` while
  `\"KustoExplorerQueryRun\" contains \"Explorer\"` will return `true`".
- Full operator matrix: `==`/`=~`, `contains`/`contains_cs`, `has`/`has_cs`, `has_any`/`has_all`,
  `hasprefix`/`hassuffix` (+`_cs`), `startswith`/`endswith` (+`_cs`), `in`/`in~`, `matches regex`
  (case-sensitive, substring semantics: "LHS contains a match for RHS"). Default-insensitive
  operators each have a `_cs` variant; equality `==` and `in` are the case-sensitive defaults.
- Documented performance guidance: "when there are two operators that do the same task, use the
  case-sensitive one"; "`has` works faster than `contains`, `startswith`, or `endswith`"; terms
  under 3 chars or `contains` "revert to scanning... much slower than looking up the term in the
  term index" [23].

### 1.7 SQL standard and full-text

PostgreSQL pattern matching [24]:

- `LIKE`: `_`/`%` wildcards, **anchored to the whole string** ("pattern matching always covers the
  entire string"), `ESCAPE` clause (default backslash), case-sensitive; `ILIKE` (PG extension)
  for case-insensitive.
- `SIMILAR TO`: hybrid regex that also must match the entire string; the docs note oddities (dot is
  not a metacharacter).
- POSIX regex `~`, `~*`, `!~`, `!~*`: **unanchored** ("allowed to match anywhere within a string"),
  `(?i)` inline option.
- Security caution quoted verbatim: "regular expressions can be contrived that take arbitrary
  amounts of time and memory to process. Be wary of accepting regular-expression search patterns
  from hostile sources... `LIKE` searches, being much simpler..., are safer to use with
  possibly-hostile pattern sources" [24].

SQL Server `CONTAINS` (full-text predicate) [25]: word/phrase match over a full-text index; "the
search for characters in the word or phrase is not case-sensitive"; "punctuation is ignored"
(`"computer failure"` matches "my computer? Failure to"); prefix term `"text*"` (an unquoted `*`
is a literal); `NEAR((a,b), maxdist, match_order)`; `FORMSOF(INFLECTIONAL|THESAURUS, ...)`;
boolean `AND/OR/AND NOT`; stopwords are dropped and a stopword-only query errors. The docs also
state the performance gap: "A LIKE query against millions of rows of text data can take minutes to
return; whereas a full-text query can take only seconds or less" [25].

PostgreSQL tsquery family [26]: `to_tsquery` (full `& | ! <->` syntax, errors on malformed input),
`plainto_tsquery` (AND of words), `phraseto_tsquery` (`<->` phrases, stopword distances kept as
`<N>`), and `websearch_to_tsquery` — quoted phrases, `OR`, `-`negation, and crucially it "will
never raise syntax errors, which makes it possible to use raw user-supplied input"; all query
terms are stemmed/stopword-filtered like documents (`Rats` → `'rat'`) [26].

### 1.8 ModSecurity (WAF rule language; included as closely-related prior art)

- `@rx` is the default operator (PCRE); `@pm`/`@pmFromFile` "performs a case-insensitive match of
  the provided phrases... uses a set-based matching algorithm (Aho-Corasick), which means that it
  will match any number of keywords in parallel", and "when matching of a large number of keywords
  is needed, this operator performs much better than a regular expression" [45][46].
- Rules normalize input before matching via transformation chains (`t:lowercase` etc.); since
  2.5.0-dev2 "transformations will only be performed once for each transaction" (cached) [46].

---

## 2. Stream engines (CEP/streaming SQL)

### 2.1 Flink SQL

- `LIKE` / `SIMILAR TO` predicates exist, but "the escape character has not been supported yet" for
  either, and the docs do not state case sensitivity or define the wildcards [27].
- `REGEXP(string, regex)`: "returns TRUE if any (possibly empty) substring of string1 matches the
  **Java regular expression**" — i.e. substring semantics, Java regex dialect [27].
- `REGEXP_EXTRACT(str, regex[, group])` (group 0 = whole match), `REGEXP_REPLACE`, plus literal
  helpers `POSITION(a IN b)` (1-based, 0 if absent), `INSTR`, `LOCATE`, `UPPER`/`LOWER` [27].

### 2.2 ksqlDB

- Operators reference shows `value_expr [NOT] LIKE value_expr [ESCAPE string]` as grammar only — no
  documented wildcard set or case-sensitivity statement [28].
- Scalar functions: `REGEXP_EXTRACT`, `REGEXP_EXTRACT_ALL`, `REGEXP_REPLACE`,
  `REGEXP_SPLIT_TO_ARRAY`; literal `INSTR(string, substring[, position[, occurrence]])` (0 when
  absent); `LCASE`/`UCASE`. No string `CONTAINS` function and the docs do not name the regex
  dialect (ksqlDB is JVM-based) [29].
- Net: predicate vocabulary for text is thin and under-documented compared to the security
  languages [28][29].

### 2.3 Esper (EPL)

- `like`: SQL semantics — `_` single char, `%` any run; "SQL patterns are case-sensitive by
  default"; optional `ESCAPE`; "the default escape character is backslash" [30].
- `regexp`: "based on regular expressions implemented through the Java `java.util.regex` package"
  and — the notable semantic — it "matches the entire region against the pattern via
  `java.util.regex.Matcher.matches()`": **whole-string** regex, so substring intent requires
  `'.*Jack.*'` as the docs' own example shows [30]. (Oracle's derived EPL documentation confirms
  the LIKE semantics and the java.util.regex basis [30a].)
- Esper therefore disagrees with Flink (`REGEXP` = substring) on regex anchoring within the same
  engine family — a documented source of confusion when porting predicates [27][30].

---

## 3. Prompt-inspection practice (deterministic checks in LLM guardrails)

### 3.1 Open-source guardrail tooling

**LLM Guard (ProtectAI)** ships three purely deterministic input scanners, each advertised as
"uses built-in functions, which makes it fast" [31][32][33]:

- `BanSubstrings`: substring list with `match_type` **str** ("sought throughout the entire user
  prompt") vs **word** ("exclusively hunts for whole words"), `case_sensitive` flag, optional
  `redact` (replaces with `[REDACT]`), `contains_all` (AND vs OR over the list) [31].
- `Regex`: pattern list with `is_blocked` deciding whether patterns are "bad" or "good", and
  `match_type` SEARCH vs FULL_MATCH — i.e. explicit substring-vs-whole-string anchoring as an
  option [32].
- `InvisibleText`: detects/strips Unicode categories Cf (format), Cc (control), Co (private use),
  Cn (unassigned) and the Private Use Areas; framed as defense against "a modern form of prompt
  injection" via hidden characters [33].

**NeMo Guardrails (NVIDIA)**: the non-LLM jailbreak detection is heuristic, not keyword-based —
"length per perplexity" (input length / GPT-2-large perplexity, default threshold 89.79) and
"prefix/suffix perplexity" (default 1845.65, derived from GCG attack prompts) [34]. Measured
limits published in the 0.21.0 docs: the mean threshold "yields 31.19% of jailbreaks being
detected with a false positive rate of 7.44%"; the heuristics are English-only and "will yield
significantly more false positives on non-English text, including code" [35].

**Rebuff (ProtectAI, archived May 2025)**: four layers (heuristics, LLM detection, vector DB,
canary tokens) [36]. The heuristic layer is a concrete keyword-matching design: it generates
~8,800 candidate phrases as the Cartesian product of 11 verbs ("Ignore", "Disregard", "Bypass",
"Do not obey"...) × 8 adjectives ("previous", "prior", "above"...) × 20 objects ("instructions",
"context", "directives"...) × 5 suffixes ("and start over"...); input is normalized (lowercase,
strip punctuation/underscores, collapse whitespace) and scored by positional word overlap in a
sliding window plus a difflib character-similarity adjustment, returning a 0–1 score with the
threshold left to the caller [37]. The README disclaims: "Rebuff is still a prototype and cannot
provide 100% protection against prompt injection attacks!" [36].

**promptmap2**: attack rules are deterministic YAML (50+ rules across distraction,
prompt_stealing, jailbreak, harmful, hate, social_bias), but success evaluation is delegated to a
controller LLM judging natural-language pass/fail conditions; the README warns "weaker models may
not analyze results accurately and could lead to false positives or negatives" [38] — i.e. even
attack tooling found deterministic response-checking insufficient except for literal canaries
(its firewall mode checks the literal string "true") [38].

**Lakera PINT benchmark**: 3,007-input evaluation set that deliberately includes `hard_negatives`
— "inputs that aren't injections but contain wording that often appears in them" — to measure
false positives alongside detection; dataset kept private to limit overfitting; leaderboard scores
include Lakera Guard 98.10% (2024-06-12) and Meta Prompt Guard 90.45% [39].

### 3.2 Academic measurements of keyword/heuristic detection limits

- Liu et al., "Formalizing and Benchmarking Prompt Injection Attacks and Defenses" (USENIX Security
  2024): framework plus systematic evaluation of 5 attacks × 10 defenses (including detection-based
  defenses) over 10 LLMs and 7 tasks; positioned explicitly as the common quantitative benchmark
  because prior work was "limited to case studies" [40].
- "InjecGuard: Benchmarking and Mitigating Over-defense in Prompt Injection Guardrail Models" +
  NotInject benchmark: guard models "falsely flag benign inputs as malicious due to **trigger word
  bias**"; NotInject is 339 benign prompts deliberately enriched with injection trigger words, and
  on it state-of-the-art guards drop "close to random guessing levels (60%)" accuracy — direct
  quantitative evidence that naive keyword presence is a high-false-positive signal [41].
- Boucher et al., "Bad Characters: Imperceptible NLP Attacks": encoding-level perturbations —
  invisible characters, homoglyphs, reordering (bidi), deletion characters — degrade commercial
  and open-source NLP pipelines "with a single imperceptible encoding injection", and "with three
  injections most models can be functionally broken"; attacks worked against deployed Microsoft and
  Google systems [42]. This is the adversarial model any deterministic text matcher over prompts
  must assume.
- "ASCII smuggling" via the Unicode Tags block (U+E0000 range): invisible-to-UI instructions that
  LLMs nevertheless interpret, demonstrated against ChatGPT tooling; recommended mitigation is for
  applications to "filter out Unicode Tags code points at both prompt and response time" [43].

---

## 4. Matching-algorithm notes relevant to a Go engine

### 4.1 Multi-keyword matching

- Aho–Corasick (1975): locates all occurrences of any of a finite set of keywords in one pass;
  automaton construction time proportional to the sum of keyword lengths; original application sped
  up a bibliographic search program ~5–10× [44].
- Production uptake in exactly our problem shape: Snort/Suricata MPM prefilter (§1.3, [8][9][13]),
  ModSecurity `@pm` ("match any number of keywords in parallel... performs much better than a
  regular expression" for keyword lists) [45][46], YARA's atom engine (§1.2, [6]).
- Hyperscan (NSDI'19): decomposes regex sets into string matching + finite automata and accelerates
  both with SIMD; "improves the performance of Snort by a factor of 8.7 for a real traffic
  trace" [47]; preferred Suricata MPM where available [9][10].

### 4.2 Safe regex: RE2 / Go `regexp` vs backtracking

- RE2: "Safety is RE2's primary goal... being able to handle regular expressions from untrusted
  users without risk"; match time "linear in the length of the input string"; deliberately omits
  backreferences and lookaround — "RE2 does not support constructs for which only backtracking
  solutions are known to exist" [48].
- Go `regexp` inherits this: "The regexp implementation provided by this package is guaranteed to
  run in time linear in the size of the input... a property not guaranteed by most open source
  implementations" (RE2 syntax, no backreferences) [50].
- The asymmetry is dramatic: on `a?ⁿaⁿ` vs `aⁿ`, Perl's backtracker needs >60 seconds at n=29 where
  a Thompson NFA needs ~20 microseconds — "a million times faster" [49].
- Operational regret evidence for backtracking in a matching engine: the 2019 Cloudflare global
  outage was a WAF rule regex (critical part `.*(?:.*=.*)`) that "backtracked enormously and
  exhausted CPU"; their Lua/PCRE WAF had "no mechanism to protect against a runaway expression";
  remediations were switching to RE2/Rust regex engines with runtime guarantees, and the postmortem
  concludes "the only real solution" to catastrophic backtracking is a non-backtracking engine
  [52]. OWASP catalogs the "evil regex" patterns ((a+)+$, (a|aa)+$, overlapping
  alternation/repetition) and ReDoS CVEs [51]. PostgreSQL's own docs give the same warning for
  hostile patterns [24].

### 4.3 Unicode case folding and normalization pitfalls

- Locale-sensitive case folding is a documented security bug class: the Turkish-I problem —
  `path.StartsWith("FILE:", true, null)` returns false under Turkish culture (dotted/dotless i), so
  "someone could circumvent security measures that block access to case-insensitive URIs that begin
  with 'FILE:'"; the fix is ordinal (locale-independent) comparison [55].
- UTS #39 defines confusable (homoglyph) detection: strings are confusable iff their skeletons are
  equal (NFD → remove Default_Ignorable_Code_Points → map to prototype glyphs → NFD); defines
  mixed-script/whole-script confusables (`paypal` vs Cyrillic `pаypаl`); warns skeletons are not a
  normalization, change across Unicode versions, and that confusable checks produce false positives
  needing contextual filtering [53]. UTR #36 (security considerations) is stabilized/superseded by
  UTS #39 and UTS #55 [54].
- Invisible/format characters are an active evasion channel against text matchers in the LLM
  context: Tags-block smuggling [43], Cf/Cc/Co/Cn stripping in LLM Guard's InvisibleText scanner
  [33], and the Bad Characters attack family (invisible chars, homoglyphs, bidi reordering,
  deletion chars) measured to break deployed NLP systems [42].
- Cost of case-insensitivity inside the matcher itself: YARA's guide rates `nocase` "HIGH" cost
  (atom permutation explosion) [6]; Kusto's docs tell users case-sensitive operators are faster
  [23]; Sigma's insensitive default forces `(?i)` regex wrapping on case-sensitive backends with a
  measured usability/performance complaint [3].

---

## 5. Cross-cutting lessons (each strictly evidence-backed)

1. **Tokenized term match and substring match are different operators; conflating them burns
   users.** Kusto separates `has` (indexed whole term) from `contains` (substring scan) and
   documents both the semantic trap (`has "Explorer"` is false where `contains` is true) and the
   performance cliff (sub-3-char terms and `contains` fall back to scans) [23]. Splunk's TERM()
   fails silently when the sought text spans a major breaker (`TERM(127.0.0.1)` vs
   `ip=127.0.0.1`) [14], and Elastic explicitly warns "avoid using the term query for text
   fields" because analysis rewrites values [19]. Elastic ultimately shipped a whole new field type
   (`wildcard`, trigram-indexed + verification) because security users needed true substring
   semantics [22].

2. **Every mature detection language makes case sensitivity explicit per operator, and the
   ecosystems that picked a global default paid for it at the edges.** Defaults diverge wildly:
   Sigma insensitive [1], Splunk insensitive [14], YARA/Snort/Suricata/EQL/SQL LIKE sensitive
   [5][12][7][20][24]. Escape hatches are everywhere: Sigma `cased` [2], Splunk `CASE()` [14],
   Kusto `_cs` suffixes [23], EQL `~` suffixes [20], `nocase` [5][12], `ILIKE`/`~*` [24]. Sigma's
   insensitive default needs `(?i)` regex wrapping on case-sensitive backends (slower; toggle flag
   added) [3], and both Kusto [23] and YARA [6] document that insensitive matching costs real
   performance.

3. **Regex anchoring is a portability trap: name it in the semantics.** Whole-string regex:
   Esper `regexp` (Java `matches()`) [30], SQL `LIKE`/`SIMILAR TO` [24]. Substring regex: Flink
   `REGEXP` ("any (possibly empty) substring") [27], PostgreSQL `~` [24], Kusto `matches regex`
   ("LHS contains a match") [23], Splunk `match()` [16]. Two CEP engines (Esper, Flink) chose
   opposite semantics for the same keyword family [27][30].

4. **The winning engine architecture is a cheap multi-literal prefilter (Aho-Corasick/Hyperscan)
   gating expensive verification (regex), with the selective literal choosable by the rule
   author.** Snort/Suricata fast_pattern + MPM [8][13][9], YARA atoms [6], ModSecurity `@pm`
   ("much better than a regular expression" for keyword lists) [45][46], Hyperscan's 8.7× Snort
   speedup [47], and Suricata's rule to always pair `pcre` with a `content` prefilter [7]. The
   known pitfall is automatic prefilter selection: "the longest pattern is sometimes not the most
   unique" [13].

5. **For untrusted patterns or hot paths, use a linear-time regex engine; backtracking regex in a
   matching engine is a documented production regret.** Cloudflare's 2019 global outage came from
   one backtracking WAF regex; their stated fix was moving to RE2/Rust regex ("the only real
   solution") [52]. PostgreSQL warns identically [24]; OWASP catalogs the vulnerable pattern
   shapes [51]. Go's standard `regexp` already provides the linear-time guarantee (RE2 semantics:
   no backreferences/lookaround) [48][50][49].

6. **Pure keyword/phrase heuristics for prompt injection have measured, poor operating points —
   and the dominant failure mode is false positives on benign text containing trigger words.**
   NeMo's heuristic threshold detects 31.19% of jailbreaks at 7.44% FPR [35]; NotInject shows SOTA
   guard models falling to ~60% (near random) accuracy on benign prompts enriched with trigger
   words [41]; Rebuff's 8,800-phrase matcher ships as a "prototype [that] cannot provide 100%
   protection" [36][37]; Lakera's PINT bakes hard negatives into the benchmark precisely to price
   this in [39]. Implication for us: deterministic text operators are detection primitives whose
   rules need context (fields, sequences, correlation), not standalone classifiers.

7. **A text matcher over prompts must handle Unicode evasion in the pipeline, not in each rule:
   invisible characters, homoglyphs and locale-dependent case folding are all demonstrated
   bypasses.** Invisible Tags-block payloads reach LLMs while evading human review (mitigation:
   strip/flag the code points) [43]; LLM Guard ships a dedicated Cf/Cc/Co/Cn scanner [33]; Bad
   Characters breaks deployed systems with 1–3 imperceptible injections [42]; UTS #39 gives the
   standard confusable-skeleton algorithm and its false-positive caveats [53]; the Turkish-I
   example shows locale-aware case-insensitive compare is itself a bypass vector — fold ordinally
   [55]. WAF practice normalizes once per transaction before matching (ModSecurity transformation
   caching) [46].

8. **Transformation modifiers on values (encode/permute the keyword, not the event) are a proven
   way to keep rules readable while matching encoded variants.** Sigma `base64offset`, `utf16`+
   `base64`, `windash` (dash-variant permutations) [1][2]; YARA `xor`, `base64`, `wide` [5]. The
   same idea covers our "match keyword also in base64-encoded tool output" cases without an
   eval-time decoder.

9. **Tokenizing/stemming full-text operators are the wrong default primitive for detection rules —
   they trade precision for recall in ways rule authors can't see.** SQL Server CONTAINS ignores
   punctuation and stopwords and errors on stopword-only queries [25]; Postgres tsquery stems query
   terms (`Rats` → `rat`) [26]; Elastic/KQL analysis makes term order irrelevant on text fields
   [21]; Splunk's indexed-term model makes exact substring search impossible without TERM
   gymnastics [14][17]. Detection languages (Sigma, YARA, Snort) all chose literal
   substring/anchored semantics instead [1][5][7]. One genuinely reusable full-text idea:
   `websearch_to_tsquery` "will never raise syntax errors" on user input — a forgiving parse mode
   for operator values sourced from analysts [26].

10. **Under-specified text predicates in streaming SQL are a cautionary tale, not a model.** Flink
    ships LIKE/SIMILAR TO with "escape character has not been supported yet" and no documented case
    rules [27]; ksqlDB documents LIKE only as grammar, with no wildcard or case semantics [28].
    The security languages, by contrast, specify defaults, escaping and anchoring normatively
    [2][5][7][23] — that is the bar for our spec.

---

## References

All accessed 2026-10-31.

1. SigmaHQ docs — Field Modifiers. https://sigmahq.io/docs/basics/modifiers.html
2. Sigma specification v2.x — Modifiers Appendix. https://github.com/SigmaHQ/sigma-specification/blob/main/specification/sigma-appendix-modifiers.md
3. pySigma-backend-loki PR #53 — "Change generated LogQL queries to be case insensitive by default". https://github.com/grafana/pySigma-backend-loki/pull/53
4. rsigma Fibratus backend reference (third-party Sigma backend). https://timescale.github.io/rsigma/reference/backends/fibratus/
5. YARA documentation — Writing YARA rules. https://yara.readthedocs.io/en/stable/writingrules.html
6. Roth, F. et al. — YARA Performance Guidelines. https://github.com/Neo23x0/YARA-Performance-Guidelines
7. Suricata docs — Payload Keywords (content, nocase, pcre). https://docs.suricata.io/en/latest/rules/payload-keywords.html
8. Suricata docs — Suricata Fast Pattern Determination Explained. https://docs.suricata.io/en/latest/rules/fast-pattern-explained.html
9. Suricata docs — Tuning Considerations (mpm-algo ac/ac-ks/hs). https://docs.suricata.io/en/latest/performance/tuning-considerations.html
10. Suricata docs — Hyperscan. https://docs.suricata.io/en/latest/performance/hyperscan.html
11. Snort 3 rule docs — content. https://docs.snort.org/rules/options/payload/content
12. Snort 3 rule docs — nocase. https://docs.snort.org/rules/options/payload/nocase
13. Snort 3 rule docs — fast_pattern. https://docs.snort.org/rules/options/payload/fast_pattern
14. Splunk Search Manual 10.0 — Use CASE() and TERM() to match phrases. https://help.splunk.com/en/splunk-enterprise/search/search-manual/10.0/search-primer/use-case-and-term-to-match-phrases
15. Splunk Admin Manual 10.0 — segmenters.conf (major/minor breakers). https://help.splunk.com/en/data-management/splunk-enterprise-admin-manual/10.0/10.0.0-configuration-file-reference/segmenters.conf
16. Splunk SPL Reference 10.0 — Comparison and Conditional functions (like, match, searchmatch). https://help.splunk.com/en/splunk-enterprise/search/spl-search-reference/10.0/evaluation-functions/comparison-and-conditional-functions
17. Splunk .conf20 — PLA1089C slides (TERM scan_count/false-positive case study; <1% usage). https://conf.splunk.com/files/2020/slides/PLA1089C.pdf
18. Splunk .conf17 — Fields, Indexed Tokens and You. https://conf.splunk.com/files/2017/slides/fields-indexed-tokens-and-you.pdf
19. Elasticsearch reference — Term query ("Avoid using the term query for text fields"). https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
20. Elasticsearch reference — EQL syntax. https://www.elastic.co/guide/en/elasticsearch/reference/current/eql-syntax.html
21. Elastic reference — Kibana Query Language (KQL). https://www.elastic.co/docs/reference/query-languages/kql
22. Elastic blog — Find strings within strings faster with the new wildcard field. https://www.elastic.co/blog/find-strings-within-strings-faster-with-the-new-elasticsearch-wildcard-field
23. Microsoft Learn — Kusto string operators (terms, has vs contains, performance tips). https://learn.microsoft.com/en-us/kusto/query/datatypes-string-operators
24. PostgreSQL docs — Pattern Matching (LIKE, SIMILAR TO, POSIX regex, hostile-pattern caution). https://www.postgresql.org/docs/current/functions-matching.html
25. Microsoft Learn — CONTAINS (Transact-SQL). https://learn.microsoft.com/en-us/sql/t-sql/queries/contains-transact-sql
26. PostgreSQL docs — Controlling Text Search (to_tsquery, websearch_to_tsquery). https://www.postgresql.org/docs/current/textsearch-controls.html
27. Apache Flink 1.20 docs — System (Built-in) Functions. https://nightlies.apache.org/flink/flink-docs-release-1.20/docs/dev/table/functions/systemfunctions/
28. ksqlDB reference — Operators. https://docs.ksqldb.io/en/latest/developer-guide/ksqldb-reference/operators/
29. ksqlDB reference — Scalar functions. https://docs.ksqldb.io/en/latest/developer-guide/ksqldb-reference/scalar-functions/
30. Esper 8.9 Reference — Ch. 9 EPL Operators (9.10 Like, 9.11 Regexp). http://esper.espertech.com/release-8.9.0/reference-esper/html/epl-operator.html
    30a. Oracle EPL Reference (Esper-derived) — Operators. https://docs.oracle.com/cd/E12839_01/apirefs.1111/e14304/operators.htm
31. LLM Guard docs — BanSubstrings input scanner. https://protectai.github.io/llm-guard/input_scanners/ban_substrings/
32. LLM Guard docs — Regex input scanner. https://protectai.github.io/llm-guard/input_scanners/regex/
33. LLM Guard docs — InvisibleText input scanner. https://protectai.github.io/llm-guard/input_scanners/invisible_text/
34. NVIDIA NeMo Guardrails docs — Jailbreak Protection (heuristics, thresholds). https://docs.nvidia.com/nemo/guardrails/latest/configure-rails/guardrail-catalog/jailbreak-protection.html
35. NVIDIA NeMo Guardrails 0.21.0 docs — Jailbreak Protection (31.19% detection @ 7.44% FPR). https://docs.nvidia.com/nemo/guardrails/0.21.0/configure-rails/guardrail-catalog/jailbreak-protection.html
36. Rebuff (ProtectAI) — README (archived May 2025). https://github.com/protectai/rebuff
37. Rebuff — detect_pi_heuristics.py (phrase generation, normalization, scoring). https://github.com/protectai/rebuff/blob/main/python-sdk/rebuff/detect_pi_heuristics.py
38. promptmap2 — README (rules, controller-LLM evaluation). https://github.com/utkusen/promptmap
39. Lakera PINT benchmark — repo and leaderboard (hard_negatives; dataset composition). https://github.com/lakeraai/pint-benchmark (mirror: https://github.com/CheckPointSW/pint-benchmark)
40. Liu, Y. et al. — Formalizing and Benchmarking Prompt Injection Attacks and Defenses (USENIX Security 2024). https://arxiv.org/abs/2310.12815
41. Li, H. et al. — InjecGuard: Benchmarking and Mitigating Over-defense in Prompt Injection Guardrail Models (NotInject benchmark). https://arxiv.org/abs/2410.22770
42. Boucher, N. et al. — Bad Characters: Imperceptible NLP Attacks. https://arxiv.org/abs/2106.09898
43. Rehberger, J. (Embrace The Red) — ASCII Smuggler: Crafting Invisible Text with Unicode Tags. https://embracethered.com/blog/posts/2024/hiding-and-finding-text-with-unicode-tags/
44. Aho, A. & Corasick, M. — Efficient string matching: an aid to bibliographic search (CACM 1975). https://dl.acm.org/doi/10.1145/360825.360855
45. ModSecurity Reference Manual (v2.x) — Operators (@pm, @pmFromFile, @rx). https://github.com/SpiderLabs/ModSecurity/wiki/Reference-Manual-(v2.x)-Operators
46. SpiderLabs blog (archived at LevelBlue) — ModSecurity 2.5.0-dev2 release note (@pm introduction, transformation caching). https://www.levelblue.com/blogs/spiderlabs-blog/another-modsecurity-development-release/
47. Wang, X. et al. — Hyperscan: A Fast Multi-pattern Regex Matcher for Modern CPUs (NSDI '19). https://www.usenix.org/conference/nsdi19/presentation/wang-xiang
48. google/re2 — README "Why RE2". https://github.com/google/re2
49. Cox, R. — Regular Expression Matching Can Be Simple And Fast. https://swtch.com/~rsc/regexp/regexp1.html
50. Go standard library — package regexp (linear-time guarantee). https://pkg.go.dev/regexp
51. OWASP — Regular expression Denial of Service (ReDoS). https://owasp.org/www-community/attacks/Regular_expression_Denial_of_Service_-_ReDoS
52. Cloudflare — Details of the Cloudflare outage on July 2, 2019. https://blog.cloudflare.com/details-of-the-cloudflare-outage-on-july-2-2019/
53. Unicode — UTS #39: Unicode Security Mechanisms (confusables, skeleton, mixed-script). https://www.unicode.org/reports/tr39/
54. Unicode — UTR #36: Unicode Security Considerations (stabilized; superseded by UTS #39/#55). https://www.unicode.org/reports/tr36/
55. Microsoft Learn — Best practices for comparing strings in .NET (Turkish-I example). https://learn.microsoft.com/en-us/dotnet/standard/base-types/best-practices-strings
