*Review of the S1.1a-1 design note v2 (2026-11-02). That draft was revised in place and never committed, so its `GD:` line references are approximate. Every finding is dispositioned in [s1-1a-gate-design.md](../sprints/s1-1a-gate-design.md) §11.*

# S1.1a-1 design note v2: focused re-check by probe

> **Historical — S1.1a-1, the local gate, coverage and the event kernel; merged in PR #1.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-02--s11a-plan-check-g0); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1a-plan-check.md) · [design](../sprints/s1-1a-gate-design.md) · [record](../sprints/s1-1a-1-record.md) · reviews: [plan check: architecture](s1-1a-plan-check-architecture.md), [plan check: QA](s1-1a-plan-check-qa.md), [design: architecture](s1-1a-design-architecture.md), [design: QA](s1-1a-design-qa.md), **design re-check**, [build: architecture](s1-1a-1-build-architecture.md), [break-it: QA](s1-1a-1-breakit-qa.md). **← [Phase 1 decomposition review](phase1-decomposition-architecture.md) · [S1.1a-2](../sprints/s1-1a-2-record.md) →**

Target: `docs/sprints/s1-1a-gate-design.md` (v2, 2026-11-02, uncommitted). Re-checked on darwin-arm64 (Darwin 25.6.0, APFS), 2026-11-02.

Probe directory: `<scratchpad>/s1-1a/recheck-probe/`. It holds the scripts in `scripts/`, the downloads in `dl/`, and the simulated cache and test modules in `x/`. No repository file was edited.

**Downloads.** I downloaded only the five archives listed. Each sha256 matched the note's §3 table before I opened it (`shasum -a 256 -c`):

```
py315.tar.gz: OK   py312.tar.gz: OK   go.tar.gz: OK   golangci.tar.gz: OK   ruff.tar.gz: OK
```

"Inferred" marks anything I did not prove with a probe.

---

## 1. Verdicts for claims 1–10

| # | Claim | Verdict | Evidence (probe output) | Fix needed |
|---|---|---|---|---|
| 1 | §3: `os.rename` of a read-only tree from `<cache>/tmp.X/` to a sibling `<cache>/<final>` works. A loser of the install race can restore owner write without following symlinks, then `rmtree` the tree. | **CONFIRMED**, with one precision (new issue S1) | All five archives were extracted into `tmp.<pid>.<rand>/`, made read-only, then renamed. Example: `readonly: {'file': 1762, 'dir': 174, 'symlink_skipped': 9}` · `tmp root mode: 0o40555` · `rename OK -> …/python-3.15.2-285a6b924713`. The loser path, on the pinned 3.15 and on 3.12: `rename onto existing final: PermissionError errno EACCES isinstance FileExistsError: False` · `rmtree before restore: PermissionError EACCES` · `restore: {'chmod': 1935, 'symlink_skipped': 9}` · `rmtree after restore: OK` · `final intact: True`. When the target is writable and not empty, the error is different: `writable non-empty target: OSError ENOTEMPTY`, but `read-only non-empty target: PermissionError EACCES`. | The rule text: "on **any** `OSError` from the rename, if `final` exists, take the loser path; otherwise re-raise." Code that catches only `FileExistsError`, or checks only for EEXIST/ENOTEMPTY, crashes the loser with exit 2. |
| 2 | §3 step-4 rules against the real archives | **PARTLY REFUTED** | Census in §3 below. Every member of 3.15.2, 3.12.16 and golangci-lint is accepted. No archive contains an absolute path, a `..`, a hard link, a device or FIFO, an escaping link, a path through a symlink, a duplicate, a setuid bit or a case collision, so nothing is **wrongly accepted**. **Wrongly rejected:** the go and ruff archives each carry a directory member that **is** the strip prefix: `drwxr-xr-x … go/` and `drwxr-xr-x … ruff-aarch64-apple-darwin/`. `tarfile` names them `go` and `ruff-aarch64-apple-darwin`, without the slash. A literal check that the member starts with `strip` rejects them, so the first install of go and of ruff exits 2. **Ordering:** 7 symlinks in 3.15.2 and 8 in 3.12.16 come before their targets (`python/bin/idle3 -> idle3.15` is member #1 and `idle3.15` is #2), so a check based on whether the target exists rejects them. **Wording:** the Python archives also have **0** directory members: 173 parents (3.15.2) and 153 (3.12.16) are created on demand, not only golangci-lint's. | Accept a directory member equal to `strip` without its slash, as the tree root. Resolve link targets by name, within the tree as extracted so far; never check that the target exists. Fix the parenthetical about golangci-lint. |
| 3 | Read-only go1.27.3 tree: `go build`, `go vet -json` and `go test -cover` work. The read-only 3.15.2 runs `-I -B -X utf8` imports. | **CONFIRMED** | `find $GOROOT -perm -u+w` printed nothing. With `env -i`, `GOTOOLCHAIN=local GOENV=off CGO_ENABLED=0` and GOCACHE/GOMODCACHE elsewhere: `build exit 0`, `vet exit 0` (six `{}` objects), `test exit 0` with `coverage: 100.0%`, `25.0%` and `0.0%` lines. Pinned 3.15.2: `3.15.2 1 1 1 e3b0c442 OpenSSL 3.5.9`, exit 0, importing json, subprocess, hashlib, tarfile, urllib.request, signal, queue and ssl. Without `-B` it also runs, and no `.pyc` is written. 3.12.16 runs the same way. | none |
| 4 | §3 `exec_sha256` covers every tool that is run | **REFUTED** for `go test -cover` and for the Python shared library | **The Go set has 10 files:** `bin/go`, `bin/gofmt`, and `pkg/tool/darwin_arm64/{asm,cgo,compile,cover,fix,link,preprofile,vet}`. `build -x` runs only compile (buildid runs inside `go`, "# internal"). `vet -x` runs vet, compile and asm. `test -x -json -coverprofile` runs compile, vet, asm, cover, link, the `$WORK/…/*.test` binaries, and **`$GOROOT/bin/go tool covdata percent -i …`** and **`covdata textfmt`**. `covdata` is **not shipped** in pkg/tool. `go tool -n covdata` gives `$GOCACHE/ab/ab09ca55…-d/covdata`, built on demand from `GOROOT/src/cmd/covdata` and run from GOCACHE. buildid and test2json also resolve to GOCACHE. **Python:** `otool -L bin/python3.15` lists only system libraries (CoreFoundation, libSystem, libz, libedit, libncurses, libpanel, SystemConfiguration), so the binary is static. `lib/libpython3.15.dylib` (and 3.12's) ships but is **not loaded**. The stdlib `.py` files the gate actually runs are not hashed. `lib-dynload` holds only `_dbm` and `_tkinter` (3.12 adds `_crypt`). | §3 should state the residue (new issue S2). Hashing the dylib is harmless; on darwin it protects nothing. Whether the linux-amd64 binary links a `libpython…so` is UNTESTABLE here (that archive was not allowed). |
| 5 | §4 `go` kind filter, and the `go-vet-units` arithmetic | **`go` filter CONFIRMED. vet formula CONFIRMED for the shapes that compile, REFUTED in two cases** | `go list -e -json ./internal/... . ./rule/... ./io/...` exits 0 and keeps all 7 real packages, including `broken` (type error) and `syntaxerr` (syntax error in a function body), both with `Error= None`. It drops `'.'` (`no Go files in …`) and `'./rule/...'` / `'./io/...'` (`pattern ./rule/...: lstat ./rule/: no such file or directory`). Those two go only because of the relative-ImportPath rule; their error text is not "matched no packages". For an existing directory with no Go files, the output is only the stderr line `go: warning: "./rule/..." matched no packages`, with no JSON entry. **vet:** with {notests, intests, xtests, both}, 4 packages + 2 with XTestGoFiles = **6**, and vet printed 6 `{}` objects, exit 0. **Compile error:** no object on stdout; stderr `vet: internal/broken/a.go:3:23: cannot use "s" …`; **exit 1**. Full module: 7 objects against 9 expected. **External-tests-only directory** (only `package x_test` files): go list shows `GoFiles= None … XTest= ['a_test.go']`, and vet printed **1** object where the formula says **2**. Also: vet with diagnostics exits 0 (`vet exit 0`). A compile error only in a `_test.go` file gives vet exit 1 while `go build` exits 0. | The formula: units = (packages with GoFiles, CgoFiles or TestGoFiles) + (packages with XTestGoFiles). The vet wrapper and the scaffold must also fail on vet's non-zero exit (new issue S5). |
| 6 | §4: a main-thread `SimpleQueue().get()` is woken in under 0.5 s by SIGTERM (handler calls only `put_nowait`) while a worker blocks in `Popen.wait()` | **CONFIRMED** | 80 runs on the pinned 3.15.2, each in a fresh process, with the signal sent by an outside `sh`: `workers=1: trials=40 fails=0 min_ms=19.9 max_ms=50.9 woken_by=40 sig` and `workers=8: trials=40 fails=0 min_ms=18.2 max_ms=49.8 woken_by=40 sig`. The latency is an upper bound that includes `sh` start-up. | none |
| 7 | §4: on macOS, after the leader is reaped, `os.killpg(pid, 0)` succeeds while a grandchild lives and raises ProcessLookupError once the group is empty | **CONFIRMED** | `A: grandchild alive … killpg(pgid,0) OK -> group populated; after KILL+0.2s: ProcessLookupError` · `B: leader alone … ProcessLookupError (No such process)` · `C: grandchild exited 0.2 s ago … ProcessLookupError` · `D: … rc=3 … populated` · `E: leader killed by TERM … rc=-15 … populated` · `F: 50x leader alone, immediately probed: populated count = 0` | none on darwin. Linux stays forwarded (DQ34). |
| 8 | §5: ruff 0.17.3 facts | **CONFIRMED.** The "Inferred: Lane C verifies" line can go. | `ruff 0.17.3`. `target-version` type: `"py37" … "py314" \| "py315"`. **The per-file setting is a top-level table `[per-file-target-version]`** with `"gate/bootstrap.py" = "py312"` (from `ruff config per-file-target-version`). It works: `gate/bootstrap.py:4:12: invalid-syntax: Multiple exception types must be parenthesized on Python 3.12`, while the same code in a py315 file is not flagged. `ruff check --help` has `--ignore-noqa` and `-n, --no-cache`. `ruff format` accepts `--check`, `--no-cache` and `--config`; given `--ignore-noqa` it errors with `unexpected argument '--ignore-noqa' found`, but the note does not pass it to format. With `--ignore-noqa --config ruff.toml`: `gate/sleepy.py:3:1: TID251 \`time.sleep\` is banned` (inline `# noqa: TID251` ignored), `gate/ign.py:3:31: TID251 \`signal.SIG_IGN\` is banned`, and `from signal import SIG_IGN` is flagged too. Blanket `# noqa`, file-level `# ruff: noqa` and a nested `gate/sub/ruff.toml` with `select = []` are all overridden. **Without `--config`, the nested config wins**, so the flag is load-bearing. Per-file-ignores for `gate/wait.py` and the SIGCHLD test hold. No `.ruff_cache` is created. All the selected codes are valid: there is no unknown-selector error, and `XYZ999` gets `Unknown rule selector`. **Summary lines:** `7 files already formatted` · `1 file already formatted` · `1 file would be reformatted, 7 files already formatted` · `1 file would be reformatted`. An unparsable file gives **exit 2** and is **left out of** the count. The diagnostic text is `unformatted: File would be reformatted`. | See new issues B3 and S6. |
| 9 | §8 `test_pyfloor` plants | **CONFIRMED** | `3.12.16 except A, B: SyntaxError: multiple exception types must be parenthesized` · `hasattr(copy, "replace"): False` · `AttributeError: module 'copy' has no attribute 'replace'`. On 3.15.2: `except A, B: ACCEPTED` · `True` · `copy.replace(D(), x=2) -> D(x=2)`. | none. See L3 on "only this check can fail". |
| 10 | Internal consistency | **REFUTED** | Three rows cannot be green as written (B1, B2, B3). The record contradicts §7 and owner decision 3 (S9). The W1 Makefile dependency blocks Lane A's own done-run (S8). §5 and §8 disagree on untracked test modules (S10). | See §2 below. |

---

## 2. New issues

### Blockers (rule 15: wrong detection, or an acceptance line that cannot be met)

**B1. The `git` kind's `:(glob)` prefix breaks §5's `*.go` and `*.py` scopes, and silently drops exclusions.** With glob magic, `*` does not cross `/`. Probe (git 2.54.0, fixture repo):

```
git ls-files -- '*.go'            -> cep.go, internal/event/event.go
git ls-files -- ':(glob)*.go'     -> cep.go                      (internal/event/event.go missing)
git ls-files -- ':(glob)**/*.go'  -> cep.go, internal/event/event.go
git ls-files -- ':(glob)*.py'     -> (nothing)
```

What this does to the rows:
- **`gofmt`** sees only Go files at the repository root. Today there are none, so the row reports covered 0 and goes red, and exit line 1 (§6) cannot be met. Once a root `.go` file exists (the `cep` facade), every file under `internal/` goes unchecked while the row is green. That is a missed detection.
- **`ruff`** expects 0 files.
- **`pytests`** ("minus `gate/proofs/**`"): the note does not say how to write the exclusion. If an implementation puts `:(glob)` in front of an exclude pathspec, git silently drops the exclusion: `':(glob):(exclude)gate/proofs/**'`, `':(glob)!…'`, `':(glob)^…'` and `':(glob):!…'` all still list `gate/proofs/drivers/test_p.py`. The proof drivers would then run inside `pytests` at T2.

*Fix:* write the registry scopes as `**/*.go` and `**/*.py`. Add `:(glob)` only to a pathspec that has no magic yet. Write exclusions as `:(glob,exclude)gate/proofs/**`; probe: `gate/runner/tests/test_a.py, gate/test_top.py` only. Add one table test per registry scope against a fixture repository.

**B2. `pyfloor` can never be green.** Its expected scope is `gate/bootstrap.py gate/run.py` plus their test modules, which is 2 + N files. Its covered count is "modules that passed", which is N. Rule 9 ("covered n ≠ expected m") therefore always fails. pyfloor is part of exit line 1's T2 run (§6) and of combine-W1's rows. *Fix:* make the expected scope the test modules only, and state that those tests import `bootstrap.py` and `run.py`. Alternatively, count the two source files as covered once they import cleanly under 3.12.16.

**B3. `test_ruff_format` asserts text that ruff 0.17.3 never prints.** The §8 row expects ruff FAIL "would reformat". ruff 0.17.3 prints `unformatted: File would be reformatted` and `1 file would be reformatted, 7 files already formatted`. Neither contains "would reformat". The verdict reason is composed by the runner, so it is "exit 1". The driver cannot pass as written, so `proofs` (min_wants 10) cannot pass, and T4 cannot pass. *Fix:* assert the reason "exit 1", plus a row stdout that contains `File would be reformatted`, pinned to ruff 0.17.3. Also say "the row's stdout" where §8 says "the row log": §4 step 2 replaced the single log with separate stdout and stderr files. This applies to `test_ruff_sleep` as well. ruff writes its diagnostics to stdout.

### Should-fix

**S1. The install-race loser on macOS gets EACCES, not "exists"** (claim 1 probe). Two worktrees or agents running `make check` on an empty cache would crash one of them with exit 2. *Fix:* the rule wording in claim 1's row, plus a test that renames onto a read-only `final`.

**S2. Two "every executable is hashed" statements are not true.**
- `covdata` (and buildid and test2json) are built on demand from `GOROOT/src` into GOCACHE and run from there.
- On darwin, the Python binary is static: `libpython3.15.dylib` is never loaded, and the stdlib the gate runs is not hashed.

*Fix:* state the residue in §3 and in ADR-0015 §3.6: the archive hash at install plus a read-only tree protect these files, and the per-run hash does not. Plan check S5 promised "the sha256 of each executable the gate runs".

**S3. Parity's path check does not say what it looks up, and it gives false reds on macOS temp directories.**
- tools.json has no field naming the executable to find, or the directory to put on PATH. `python-floor` is not a command. Both Python `bin/` directories contain `idle3 pip pip3 pydoc3 python python3 python3-config`.
- Probe: `mkdtemp: /tmp/tmpdsqeac0y`, but `realpath: /private/tmp/tmpdsqeac0y`, and the session's `TMPDIR` is under `/var/folders`, which is also a symlink. If only the `which()` result goes through `realpath`, every driver whose cache lives in a temp directory gets "outside the cache".

*Fix:* derive the PATH entry from `dirname(version_argv[0])` and the name to find from its basename, and say so. Apply `realpath` to both sides. Rows call `python3.15` and `python3.12` by explicit name.

**S4. Parity's version probe must run in the scrubbed §4 environment.** Probe with an inherited `GOTOOLCHAIN=go1.27.2`: the cached `go version` prints `go: downloading go1.27.2 (darwin/arm64)`, and only `GOPROXY=off` stopped it. With network access, parity would download and run a toolchain from outside the cache. That breaks §3's "a binary outside the cache is never executed". *Fix:* one sentence in §3: version probes use the row environment (`GOTOOLCHAIN=local`, `GOENV=off`, and so on).

**S5. `go-vet-units` and vet's exit code** (claim 5).
- A directory holding only external tests gives 1 unit, not 2. Fix the formula as in claim 5's row.
- A compile error gives vet exit 1 with no object. The note says only "exits 0 even with diagnostics", so a wrapper might ignore the exit code. The real row is still saved by the count, so it goes red with a misleading reason.
- **The scaffold (§7 step 3) has no count check and no `go test`.** A compile error in a `_test.go` file would pass it, since `go build` exits 0 and vet prints no object. That is a false green in W1.

*Fix:* both the wrapper and the scaffold fail on vet's non-zero exit.

**S6. The `ruff` row's count does not come from the same file list as its expected scope.** Run with no paths, ruff finds files itself. It counts untracked `.py` files (probe: `tracked *.py: 7`, `8 files already formatted`). By ruff's documented default it would also count `.pyi` and `.ipynb` files and skip its default-excluded directories (inferred). Any of these makes covered differ from expected for reasons unrelated to formatting. The summary line also has three forms (claim 8). *Fix:* pass `files.nul` explicitly, in batches like gofmt; probe: two explicit files give `2 files already formatted`. Pin the green regex `^(\d+) files? already formatted$` and fail on any other form.

**S7. Nothing protects owner decision 9's promise that Apple's 3.9.6 "is refused, with the fix text".** `bootstrap.py`'s syntax is checked only at py312 (by ruff) and by running it under 3.12.16. Probe: a bootstrap that checks the version first but contains a `type` alias later. Apple `/usr/bin/python3` 3.9.6 prints `SyntaxError: invalid syntax` and exits 1, with no fix text and no `GATE FAILED`. *Fix:* give `gate/bootstrap.py` `per-file-target-version = "py39"`. ruff 0.17.3 then flags newer syntax (probe: `invalid-syntax: Cannot use \`type\` alias statement on Python 3.9`, `Cannot use \`match\` statement on Python 3.9`). Keep pyfloor proving library use at 3.12.16. Alternatively, use a tiny 3.9-syntax entry file that imports the 3.12 body.

**S8. W1's `make check` depends on a file that does not exist in W1.** §3 makes every gate target depend on `make tools`, which runs `gate/bootstrap.py install`. But `bootstrap.py` is Lane B's W1 output, on the other branch, and Lane A owns the Makefile in W1. If Lane A writes `check: tools`, its own done-run (`make check` → the scaffold) fails on its branch. *Fix:* say that in W1 `make check` runs the scaffold with no `tools` prerequisite (the tools were installed by hand, §7), and that the `tools` dependency and the real runner are wired at combine-W1. Say who edits the Makefile then, since there is one owner per file.

**S9. The sprint record contradicts the note and owner decision 3.** `s1-1a-1-record.md:74` says the scaffold "runs under the system `python3`", "runs the registry's rows", and that the first commit holds `gate/registry.json`. The note's §7 says the pinned 3.15.2, the scaffold's own list, and the registry at combine-W1 (§2). *Fix:* annotate the record at G1 (rule 4).

**S10. The `pytests` rules are incomplete.**
- §8's `test_pytests_vacuity` asserts that an untracked `test_x.py` fails, but §5's wrapper rules leave this out. Plan check §3 had "untracked module fails (QA C10)".
- The `pytests` shim is Lane A's code, but pyfloor runs it under **3.12.16**, while ruff checks it at py315. Shim code using 3.13+ features would first show up as a red pyfloor at combine-W1.

*Fix:* add the untracked rule to §5. Put the shim under `per-file-target-version = "py312"` and add it to Lane A's brief.

**S11. `bootstrap exec`'s `argv[0]` is the bare name "python3.15"** (inferred, Linux). On darwin, `sys.executable` is still the real path even with `/opt/homebrew/bin` (which holds a python3.15) first on PATH (probe: `sys.executable = '$CACHE/python-3.15.2-285a6b924713/bin/python3.15'`). On Linux, my reading of CPython's `getpath` (not probed) is that a name without a slash is looked up on PATH. `run.py`'s `sys.executable` guard could then see `''` or another python3.15, and CI would always exit 2. *Fix (free):* pass the absolute path as `argv[0]`.

### Later

- **L1.** "Never following symlinks" works on macOS (`os.chmod in os.supports_follow_symlinks: True`; the symlink and its target were unchanged). On Linux, `chmod(symlink, follow_symlinks=False)` probably raises `NotImplementedError` (inferred). Skip symlinks rather than chmod them.
- **L2.** The Python archives ship `0o664` and `0o775` modes, which are group-writable. "Remove write permission" must clear all three write bits (`0o222`), not only the owner's.
- **L3.** `test_pyfloor`'s "only this check can fail" is false when the fixture registry also holds `ruff`. ruff flags the `except A, B:` plant in `bootstrap.py` (probe). Use a pyfloor-only fixture registry.
- **L4.** State that the `go` filter relies on the relative-ImportPath rule for missing directories, so nobody "simplifies" it to matching error text.
- **L5.** The go archive has two non-ASCII names (`go/test/fixedbugs/issue27836.dir/Þfoo.go` and `Þmain.go`). They are harmless on APFS; note them in the census.

---

## 3. Archive census (claim 2)

All five sha256 values were verified. The census was read from the tar headers, not from extracted files. The rules were dry-run in `scripts/census.py`, and the real extraction is in `scripts/install_sim.py`.

| Archive | Members | Regular | Dir | Symlink | Hard link | Device/FIFO/other | Common prefix (`strip`) | Member equal to `strip` | Parents created on demand | Modes | Verdict under §3 step 4 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| CPython 3.15.2 | 1770 | 1761 | 0 | 9 | 0 | 0 | `python/` | none | 173 | 0o664 ×1741, 0o775 ×29 | all accepted. 7 symlinks come before their target, so the target check must work by name |
| CPython 3.12.16 | 1665 | 1655 | 0 | 10 | 0 | 0 | `python/` | none | 153 | 0o664 ×1604, 0o775 ×61 | all accepted. 8 symlinks come before their target |
| go1.27.3 | 17373 | 15657 | 1716 | 0 | 0 | 0 | `go/` | `go/` (dir; tarfile name `go`) | 0 | 0o644 ×15603, 0o755 ×1770 | all accepted, **except** that `go/` itself is wrongly rejected by a literal outside-strip check |
| golangci-lint v2.15.0 | 3 | 3 | 0 | 0 | 0 | 0 | `golangci-lint-2.15.0-darwin-arm64/` | none | 0 | 0o644 ×2, 0o755 ×1 | all accepted |
| ruff 0.17.3 | 2 | 1 | 1 | 0 | 0 | 0 | `ruff-aarch64-apple-darwin/` | `ruff-aarch64-apple-darwin/` (dir) | 0 | 0o755 ×2 | as go: the root dir member is wrongly rejected under a literal reading |

**Links, with targets.** Every link is a same-directory relative symlink that resolves inside the tree.
- **3.15.2:** `bin/idle3→idle3.15`, `bin/pydoc3→pydoc3.15`, `bin/python→python3.15`, `bin/python3→python3.15`, `bin/python3-config→python3.15-config`, `lib/pkgconfig/python3-embed.pc→python-3.15-embed.pc`, `lib/pkgconfig/python3.pc→python-3.15.pc`, `share/man/man1/python.1→python3.15.1`, `share/man/man1/python3.1→python3.15.1`.
- **3.12.16:** the same nine with 3.12 names, plus `bin/2to3→2to3-3.12`.
- **go, golangci-lint, ruff:** no links.

**Across all five archives:** no duplicate members, no case-insensitive collisions, no setuid, setgid or sticky bits, and no member named `.complete` at the root. The rule for hard links to an earlier member has no darwin data to exercise it. Whether the linux-amd64 archives contain any is UNTESTABLE here.

**The `exec_sha256` sets** observed on darwin:
- **go:** `bin/go`, `bin/gofmt`, `pkg/tool/darwin_arm64/{asm,cgo,compile,cover,fix,link,preprofile,vet}` (10 files).
- **3.15.2:** `bin/python3.15` (20,579,800 bytes, static) and `lib/libpython3.15.dylib` (not loaded).
- **3.12.16:** `bin/python3.12` (18,652,312 bytes, static) and `lib/libpython3.12.dylib` (not loaded).
- **golangci-lint:** `golangci-lint`. It prints `golangci-lint has version 2.15.0 built with go1.27.0 from 25c5369a on 2026-10-16T11:03:09Z` to stdout, exit 0.
- **ruff:** `ruff`.
