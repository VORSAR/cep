> **Historical — S1.1c-4, the scans; merged in PR #40.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-13--s11c-4s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-4-plan-check.md) · [design](../sprints/s1-1c-4-design.md) · [record](../sprints/s1-1c-4-record.md) · reviews: [plan check architecture](s1-1c-4-plan-check-architecture.md), [plan check qa](s1-1c-4-plan-check-qa.md), [design architecture](s1-1c-4-design-architecture.md), [design qa](s1-1c-4-design-qa.md), [combined architecture](s1-1c-4-combined-architecture.md), [breakit qa](s1-1c-4-breakit-qa.md). **← [S1.1c-3](../sprints/s1-1c-3-record.md) · [S1.1c-5 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-4 combined build, architecture review with the-fool (blind), 2026-11-13** — the architect reviewer on `main..s1-1c-4/integration` @ 1b06251 (step 7). Every finding is placed in [the sprint record](../sprints/s1-1c-4-record.md)'s findings.

# S1.1c-4 combined-build architecture review (step 7), with the-fool

Reviewed: `git diff main..s1-1c-4/integration` at 1b06251. The branch now stands at 8a0009b, which adds one line to `docs/sprints/s1-1c-4-record.md` and changes no code, so every `file:line` below holds at both commits. The review was blind and read-only.
GitNexus: index at 1b06251 (`gitnexus status`: up to date, 960 files). `detect_changes` against main reports 1,043 changed symbols in 84 files and 57 affected processes. `check` finds 0 import cycles.
Tests: the 15 touched test modules pass under the pinned 3.15.2 (293 tests, OK, 24.9 s). Probe scripts are in `scratchpad/s1-1c-4-combined-arch/probe/`.
Hindsight: no Hindsight tools were available in this session, so I made no memory searches.

## Contracts checked and held
- **The loader's error model** (§2.1), `gomods.py:98-162`. The checks run in the designed order: a Standard package is skipped, then `Error.Err`, then no `Module`, else the package joins the compiled set. `DepsErrors` are read and keyed by their text. Lines are de-duplicated across platforms and packages, carry the ` (<goos>/<goarch> only)` suffix, and add the `module lookup disabled` hint. There is no skip list, and `tool` is read with `-deps` and no `-test`. A `Failed` is reported as `vulns: …` or `deps: …`.
- **The vulns waivers** (§2.5), `vulns.py:167-291`:
  - the target grammar is checked;
  - waivers are TEMP only, and an expired waiver is ignored;
  - an `<id> <module>` waiver silences that pair's finding and its faults;
  - a never-built finding is never "used", so it is never silenced;
  - a stale `<id> <module>` waiver is judged only when the scan completed;
  - the `vulndb` waiver silences only the 5 availability/age prefixes and is stale whenever it went unused;
  - a readable copy is always scanned;
  - "not scanned" is printed only when there is no zip.
- **The copy's atomicity** (§2.3), `vulndb_fetch.py:100-125`. The body goes to `mkstemp` in the copy's directory and is validated before `os.replace`. The stamp and `fetch-error.txt` are each written to a temp file and moved with `os.replace`. The row reads the zip once, bounded by `FETCH_CAP`, and takes the sha256 from the same bytes.
- **Tidy read as sets** (§3.1), `rows/deps.py:107-157`. Changes are attributed by file header. A leading `require ` is stripped. Directive lines are faults. go.sum lines are compared as a set. `TIDY_ALLOWED` is checked both ways. The tidy and go.sum lines run last.
- **The T4 go.sum line** (§3.2, G1-Q3), `rows/deps.py:175-187`. It runs only when `GATE_TIER == "T4"`, uses `git diff --name-only HEAD`, and a git failure becomes a named line. No row writes go.sum: grep finds no `mod download` or `-mod=mod` in `gate/runner` or `gate/toolchain`.
- **`archive_pins` against the installer's stamp order** (§4.1), `install.py:208-214, 279-288, 299-324`. `_extract` never seals. The installer still writes `.complete` and then seals, in the old order. `archive_pins` cleans up in a `finally`.
- **The canary workflow** (§4.7), `canary.yml`:
  - `permissions: contents: read`;
  - `GITHUB_TOKEN` appears on the feeds step only (pinned by `test_generate.py:274`);
  - `persist-credentials: false` on every checkout;
  - the group is `canary-${{ github.ref }}` with `cancel-in-progress: false`;
  - every cache step is `actions/cache/restore`;
  - the candidate artifact is uploaded under `if: always()`, `error.txt` included;
  - the summary job needs every job, runs under `if: always()` and reads `toJSON(needs)`;
  - a failed candidate gives `Upgrades: unknown — …` (`summary.py:44-48`).
- **The sort's pin rule** (§4.5), `sort.py:35-61`. A record is pins only when every failure matches its pattern and `structural` is empty.
- **Law files and registry**: `vulns` matches between registry and law (T4 and nightly, `min_wants` 6, 7 `expected_scope` entries). `ci-fresh` has 3 plus `canary.yml` in both files. `proofs` has 38 in both. The deps proof has 9 cases and the vulns proof 31. `teeth.py:517-521` enforces that the fields agree.
- **Purity**: `vulndb.py`, `sort.py` and `summary.py` do no I/O.
- **Decisions**: the code contradicts none of the 2026-11-13 decisions (plan check 1–5 and (a)–(h); design G1-Q1–Q3; during-build 1–3). The narrowings I found are in CA-S1 and CA-S3.

## Findings

### CA-B1 (blocker: crash; it fails closed and gives no false green). A corrupt zip member crashes the `vulns` row with no named line and no way to waive it
- **The code:** `Copy.read` (`gate/runner/rows/vulns.py:61-74`) catches only `KeyError`, and `check_db`'s `member()` (`gate/runner/vulndb.py:293-299`) catches only `MemberTooLarge`. A zip whose central directory is intact but one member's data is bad therefore raises out of `open_copy` → `scan` → `main` and gives a traceback. The bad data can be a broken deflate stream, a bad CRC or an encrypted member, which raise `zlib.error`, `BadZipFile` or `RuntimeError`. The sha256 and stamp faults already collected are lost with it.
- **The fetch lets it in:** `_validated` (`gate/toolchain/vulndb_fetch.py:76-92`) reads only `index/db.json`. Such a body from upstream is installed with a matching stamp, and the row crashes on every run. The `vulndb` waiver cannot help with a crash.
- **Evidence:** the probe `probe/corrupt_member.py` (one `ID/` member with 2 bytes flipped, stamp consistent) prints `UNCAUGHT: error Error -3 while decompressing data: invalid code lengths set`.
- **Fix:**
  - In `Copy.read`, catch `zipfile.BadZipFile, zlib.error, EOFError, OSError, RuntimeError, NotImplementedError, ValueError` and raise a new `vulndb.MemberUnreadable(name, why)`.
  - `check_db` and `entries_of` turn it into `vulns: member <name> cannot be read: <why>`. That is an integrity fault, and it marks the scan incomplete.
  - `_validated` runs `ZipFile.testzip()` so a corrupt body never replaces a good copy.
- **Tests, red first:** a unit test or driver case with a corrupted member gives a named red and no traceback; a `test_vulndb_fetch` case with a corrupt body leaves the old zip and stamp byte-identical and writes `fetch-error.txt`.

### CA-S1 (should-fix: misleading). The canary sorts a Go patch candidate as a Break, not a pin to update
- **The stale note:** `gate/canary/pin_tests.json` holds only `test_manifest`'s pin. Its note says go moves "red this same test … and no row reads go.mod's toolchain lines". That stopped being true when lane E2b landed `gate/toolchain/tests/test_toolchain_lines.py` (owner, during the build, decision 3).
- **What reds on a go move:** with `tools.json`'s go moved, three tests in that module red:
  - `test_both_toolchain_lines_name_tools_jsons_go`;
  - `test_a_stale_toolchain_line_is_named_with_both_versions`, whose witness `assertEqual(stale_lines(repo), [])` fails on the copied candidate;
  - `test_a_missing_toolchain_line_is_named`, which hard-codes `tools.json pins go1.27.4`.
- **How they sort:** none of the three is in `pin_tests.json`, so `sort.py:56-60` puts them under Breaks.
- **The narrowed contract:** design §4.5 (DQ-S8) says unmeasured go, golangci-lint and lark reds are "named in PIN_TESTS as unmeasured". The file names none, so the `unmeasured` path (`sort.py:69-71`) never fires today.
- **Why it matters:** a Go patch release is the week the canary matters most, because the `vulns` row reds on stdlib advisories at the old Go.
- **Fix:**
  - Make the two witness tests read the pinned go from `tools.json` instead of `go1.27.4`.
  - Add the remaining pin, `test_both_toolchain_lines_name_tools_jsons_go`, under the `pytests` and `pyfloor` rows with its text pattern (`tools\.json pins go`), marked `unmeasured`. Alternatively, measure it with one scratch T4 run with only go swapped.
  - Add the §4.5 unmeasured entries for golangci-lint and lark, or record why none is needed.
  - Correct the note.
- **Test:** a sort over the records of a go-only candidate gives pins and no breaks.

### CA-S2 (should-fix: blocks everyone with no exemption path). Upstream format drift gives an instant red that cannot be waived, and the fetch and the row disagree on the format
- **The row is strict:** it requires `set(db) == {"modified"}` (`vulndb.py:310`) and an exact member list (`vulndb.py:302-307`). Both are integrity faults that the `vulndb` waiver never silences (`vulns.py:36-42`).
- **The fetch is lenient:** it accepts any `db.json` with a string `modified` and any set of members (`vulndb_fetch.py:90`).
- **The failure:** if vuln.go.dev adds a key to `db.json` or a new index file, `make vulndb` succeeds and replaces the good copy. The row then reds every PR and the nightly with `index/db.json is missing or malformed` (wrong text for an extra key) or `member … is not in the database's layout`. Only a gate code change can clear it.
- **Fix:** one format validation, shared by both sides. The fetch runs `check_db`'s format checks (`db.json` shape, layout, index shapes, plus `testzip`) on the downloaded body before it replaces anything. A drift then becomes a failed fetch:
  - the old copy is kept and `fetch-error.txt` names the drift;
  - on a fresh CI runner it shows as "no zip" plus "last fetch failed", which the dated `vulndb` exemption covers (decision 2);
  - locally it reds only when the copy ages past 7 days.
- **Text:** the `db.json` fault names what is wrong (unknown key or missing `modified`).
- **Note:** the exact layout came from my own DA-L3. This finding keeps the layout strict but moves its failure to the fetch, where an exemption exists.

### CA-S3 (should-fix: misleading docs). ADR-0015 and DEVELOPMENT.md are not current with what was built
- **ADR-0015 §3 item 2** (`ADR-0015…md:121`) still says "A weekly advisory canary runs latest tools and opens/updates ONE issue (never blocks; ages per §3.9)".
- **ADR-0015 §3 item 9** (`:138`) says "The weekly canary issue … are advisory rows, so they age too".
- **As built:** the canary tries patch releases on the pinned line only (G1-Q1). It writes a job summary, not an issue. It exits 1 on breaks as a check that is not required (confirm (f)). It is not a registry row and does not age.
- **G1 checklist:** the record's item "ADR-0015 annotated as built after W3" is still unticked, and W3 is done.
- **For the owner:** whether §3.9's aging still applies to canary breaks is a question no recorded decision answers. Confirm (f) makes the canary a report; it does not mention aging.
- **DEVELOPMENT.md** never mentions `make vulndb`. A local `make strict-gate` is red without a copy fetched in the last 7 days.
- **Fix:**
  - Annotate ADR-0015 §3.2 and §3.9, citing G1-Q1, G1-Q2 and confirms (c), (f), (g), and ask the owner the aging question.
  - Add one line under DEVELOPMENT.md's T4 row: run `make vulndb` before `make strict-gate`, at least weekly.

### CA-L1 (later: coupling). A new edge from `gate/toolchain` into `gate/runner`, and the row imports the network edge
- `vulndb_fetch.py:22` imports `gate.runner.vulndb`, and `rows/vulns.py:29` imports `gate.toolchain.vulndb_fetch` for four names (`ZIP`, `STAMP`, `ERROR`, `copy_root`).
- GitNexus cypher over IMPORTS edges confirms that this is the only production import from `gate/toolchain` into `gate/runner`. It is not a module-level cycle (`check`: 0 cycles).
- **Fix:** move `ZIP`, `STAMP`, `ERROR`, `OVERRIDE` and `copy_root(environ)` into `vulndb.py`. `copy_root` stays pure over the mapping it is given. The edge then runs one way, toolchain → runner, and the verdict module never imports the fetcher.

### CA-L2 (later: cohesion). The canary's commands live in the gate's entrypoint and reach into the installer's private functions
- About 250 lines (`_version_line`, `canary_candidate`, `_failure_blocks`, `_failures`, `_job_records`, `canary_summary`) are in `gate/runner/main.py`.
- `main.py:268-285` calls `install._extract`, `install._seal`, `install._restore_write` and `install._Rejected`. GitNexus `impact` on `_extract` shows 3 direct callers, `main.py:_version_line` among them.
- **Consequences:**
  - every canary edit is an edit to the runner;
  - the canary's PR trigger `paths: ['gate/canary/**']` (`generate.py:300`) does not fire for the canary's own commands, for `render_canary`, or for `canary.yml`.
- Design §4.7 placed the commands in `main.py`, so the code follows the design. This finding is polish.
- **Fix:**
  - Move the command bodies to `gate/canary/commands.py` and dispatch them lazily, as `generate-ci` is.
  - Give `install` a public `extract_sealed(archive, strip, dest)`, used by `archive_pins` and by the version step.
  - Add `.github/workflows/canary.yml` and `gate/ci/generate.py` to the paths filter.

### CA-L3 (later: contract narrowed). The tidy line ignores the exit code
- `tidy_lines` passes `ok=(0, 1)` and discards the code (`rows/deps.py:166`). `tidy_faults` (`:141-145`) decides "could not run" from what stderr contains.
- **Probe** (`probe/tidy_probe.py`):
  - exit-1-like input with only `go: downloading` lines, or with nothing at all, returns `[]`, which reads as clean;
  - a harmless stderr warning with no diff returns `could not run`.
- Design §3.1 keys the "could not run" line on exit 1 with no `diff current/` header. Go never exits 1 silently, so the practical risk is low.
- **Fix:** have `gomods.run` return the code too (or add a variant that does). Then exit ≠ 0 with no header means could not run, and exit 0 with no header means clean, whatever stderr says.

### CA-L4 (later: implicit dependency). The `vulns` row relies on two `deps` checks that do not run in its nightly tier
- `gomods.load` keeps the root's version when both modules select a path (`gomods.py:137`, `setdefault`). The scan is complete only because `deps`' one-version line (`rows/deps.py:68-73`) reds any divergence at T2–T4.
- The row trusts `"built": false` marks, which only `deps.compare` cross-checks.
- This is safe today because the nightly runs merged `main`.
- **Fix:** both checks are cheap with `loaded` already in hand. Scan every `(path, version)` pair in `Loaded.versions`, and red a never-built mark on a module that `gomods` finds built.

### CA-L5 (later: waiver scope). The `vulndb` waiver's "no stamp" switches off integrity checks
- `"vulns: no stamp.json at "` is one of the availability prefixes (`vulns.py:38`). With a zip present but no stamp, `_stamp_faults` returns early (`vulndb.py:229-230`), so neither the sha256 nor the url check runs.
- Those are exactly the checks DA-B3 says the waiver never silences.
- **Fix:** with a zip present, treat "no stamp" as an integrity fault (the zip's origin is unknown). It is availability only when there is no zip either. The risk is low, since a CI runner always starts fresh.

### CA-L6 (later: sort precision). The sort ignores `pin_tests`' row and the record's reason
- `load_pin_tests` returns `{id: pattern}` (`sort.py:66`), so the `row` field is checked by the loader but never used. A pin test failing in an unexpected row still counts as a pin.
- When failures were read, a red record's reason is ignored (`sort.py:52-59`). A record that is "left running processes" or "killed by signal" but printed pin `failed:` lines sorts as pins.
- **Fix:** key pins by `(row, id)`. Make a red record whose reason does not start with `exit 1:` a break.

### CA-L7 (later: docstring). `candidate.py` claims to be pure but extracts archives itself
- The header says "Pure … the archives come through the injected `archive` seam". `_platform_entry` calls `install.archive_pins` directly (`candidate.py:244`), which extracts and hashes on disk.
- **Fix:** correct the docstring, or inject `archive_pins`.

### CA-L8 (later: §2.3 "on any failure"). A failed stamp write after the zip move goes unnamed
- The stamp is written outside the `try` (`vulndb_fetch.py:122`). An `OSError` after `os.replace` of the zip leaves a new zip with the old stamp, writes no `fetch-error.txt`, and escapes as `internal error` (exit 2).
- The row then reds on the sha256 mismatch (it fails closed).
- **Fix:** wrap the stamp write so `fetch-error.txt` names the failure.

### CA-L9 (later: noise). An unorderable GOVERSION gives one fault per stdlib or toolchain entry
- A `GOVERSION` such as `go1.27.4 X:…` raises inside `findings` once per stdlib or toolchain entry (`vulndb.py:194-203`), which means hundreds of lines.
- **Fix:** convert `go_version` once, before the walk, and report one line.

## The-fool pre-mortem: "it merged; a month later the gate lied or blocked everyone"
1. **Go 1.27.5 ships with security fixes.** Every PR and the nightly are red on stdlib or toolchain advisories at go1.27.4. That is decided (fail closed); the way out is a Go bump or dated `<id> stdlib` waivers. That Monday the canary files the go move under Breaks (CA-S1), so the signal to bump arrives labelled wrong.
2. **vuln.go.dev adds a `db.json` key or an index file.** `make vulndb` succeeds, then every PR is red with no exemption path until a gate code change merges (CA-S2).
3. **A corrupt member is published.** The row crashes on every run with a traceback and nothing to waive (CA-B1).
4. **vuln.go.dev is down.** Everything is red, but the dated `vulndb` waiver covers it as decided. This path is sound.
5. **The proofs row creeps toward its 600 s cap.** Locally it ran 388.2 s, with `test_vulns_planted` the longest driver at 246.6 s run serially (that run's per-module stderr). Main's CI proofs job for S1.1c-3 took 4 min 09 s (run 43417289965). Watch this job's duration on each CI run and split the driver before it nears the cap.
6. **A module marked never built starts being compiled.** `deps` reds it at T2–T4, unless only a build tag or cgo compiles it; that gap is named as residue in the reason.
7. **A developer's copy is 8 days old.** The local T4 is red. The text says to run `make vulndb`, but DEVELOPMENT.md never does (CA-S3).

## Verdict
The contracts hold across the loader, matcher, waivers, copy, tidy and go.sum lines, `archive_pins` and the canary workflow: no drift from the recorded decisions, no import cycle, registry and law consistent. Open are one crash path (CA-B1), three should-fix items (the canary mis-sorts a Go patch; upstream format drift cannot be waived; ADR-0015 and DEVELOPMENT.md are not current) and nine later items.
Fix CA-B1 and CA-S1 to CA-S3 in the fix round, each with a test that fails first. The CA-L items are coupling and polish and are safe to forward to the backlog.
