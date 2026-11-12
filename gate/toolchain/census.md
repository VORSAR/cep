# The member census of the ten pinned archives

S1.1a-1 Lane B's first commit (design note [s1-1a-gate-design.md](../../docs/sprints/s1-1a-gate-design.md) §3, "The member census"; architect DA17, QA DQ9). It records what the installer meets in every archive `tools.json` pins, and checks §3 step 4's accept and reject rules against it before any installer code exists.

**How it was taken.** On darwin-arm64 (APFS), 2026-11-02, with the pinned CPython 3.15.2 (`-I -B`). Each archive's sha256 was first checked against `tools.json`: all ten match. The census reads the tar headers with `tarfile` (names decoded as UTF-8). It extracts nothing, except the five Linux executables, whose ELF dynamic sections were parsed to settle the `exec_sha256` question for Linux (§3: "Linux is checked by Lane B's census"). "Before target" means that a symlink member comes earlier in the archive than the member it points to.

*(Annotated 2026-11-12, the Go 1.27.3 → 1.27.4 bump: the two go rows re-taken the same way, each archive's sha256 first checked against go.dev's published listing. go1.27.4 holds two more regular files than go1.27.3 and nothing else changes: no link, no special bit, no collision, the same two non-ASCII names and the same ten executables; §2–§6 hold as written.)*

## 1. Counts

| Tool | Platform | Members | Regular | Dir | Symlink | Hard link | Device, FIFO, other | `strip` | Member equal to `strip` | Parents created on demand | Modes (non-symlink) |
|---|---|---|---|---|---|---|---|---|---|---|---|
| python 3.15.2 | darwin-arm64 | 1770 | 1761 | 0 | 9 | 0 | 0 | `python/` | none | 173 | reg 0o664 ×1741, 0o775 ×20 |
| python 3.15.2 | linux-amd64 | 4641 | 3593 | 0 | 1048 | 0 | 0 | `python/` | none | 216 | reg 0o664 ×3572, 0o775 ×21 |
| python-floor 3.12.16 | darwin-arm64 | 1665 | 1655 | 0 | 10 | 0 | 0 | `python/` | none | 153 | reg 0o664 ×1604, 0o775 ×51 |
| python-floor 3.12.16 | linux-amd64 | 4536 | 3487 | 0 | 1049 | 0 | 0 | `python/` | none | 196 | reg 0o664 ×3435, 0o775 ×52 |
| go go1.27.4 | darwin-arm64 | 17375 | 15659 | 1716 | 0 | 0 | 0 | `go/` | `go` (dir 0o755, member #0) | 0 | dir 0o755 ×1716; reg 0o644 ×15605, 0o755 ×54 |
| go go1.27.4 | linux-amd64 | 17375 | 15659 | 1716 | 0 | 0 | 0 | `go/` | `go` (dir 0o755, member #0) | 0 | dir 0o755 ×1716; reg 0o644 ×15605, 0o755 ×54 |
| golangci-lint v2.15.0 | darwin-arm64 | 3 | 3 | 0 | 0 | 0 | 0 | `golangci-lint-2.15.0-darwin-arm64/` | none | 0 | reg 0o644 ×2, 0o755 ×1 |
| golangci-lint v2.15.0 | linux-amd64 | 3 | 3 | 0 | 0 | 0 | 0 | `golangci-lint-2.15.0-linux-amd64/` | none | 0 | reg 0o644 ×2, 0o755 ×1 |
| ruff 0.17.3 | darwin-arm64 | 2 | 1 | 1 | 0 | 0 | 0 | `ruff-aarch64-apple-darwin/` | `ruff-aarch64-apple-darwin` (dir 0o755, member #0) | 0 | dir 0o755 ×1; reg 0o755 ×1 |
| ruff 0.17.3 | linux-amd64 | 2 | 1 | 1 | 0 | 0 | 0 | `ruff-x86_64-unknown-linux-gnu/` | `ruff-x86_64-unknown-linux-gnu` (dir 0o755, member #0) | 0 | dir 0o755 ×1; reg 0o755 ×1 |

`tarfile` names a directory member without its trailing slash, so the go and ruff root members appear as `go` and `ruff-…`, not as `strip` itself.

**Across all ten archives:** no member outside `strip`; no absolute name; no empty, `.` or `..` component in any member name; no newline in a name; no duplicate member; no setuid, setgid or sticky bit; every mode carries owner read; no member named `.complete`; no member whose path passes through a symlink member; no hard link, device or FIFO.

**Non-ASCII names:** two, in both go archives: `go/test/fixedbugs/issue27836.dir/Þfoo.go` and `go/test/fixedbugs/issue27836.dir/Þmain.go` (valid UTF-8; `Þ` has no Unicode decomposition, so APFS and ext4 store the same bytes).

## 2. Links and their targets

Every link is a symlink. Every target is relative, resolves lexically inside the tree, and names a **regular file** member: no target is a directory, a symlink or absent, and no target path passes through a symlink.

**python 3.15.2, darwin-arm64** (9; 7 before their target):
- `bin/idle3 -> idle3.15` (before target)
- `bin/pydoc3 -> pydoc3.15` (before target)
- `bin/python -> python3.15` (before target)
- `bin/python3 -> python3.15` (before target)
- `bin/python3-config -> python3.15-config` (before target)
- `lib/pkgconfig/python3-embed.pc -> python-3.15-embed.pc` (after target)
- `lib/pkgconfig/python3.pc -> python-3.15.pc` (after target)
- `share/man/man1/python.1 -> python3.15.1` (before target)
- `share/man/man1/python3.1 -> python3.15.1` (before target)

**python 3.15.2, linux-amd64** (1048; 417 before their target): the nine darwin links above (same names and targets, each on the same side of its target), plus
- `lib/libpython3.15.so -> libpython3.15.so.1.0` (before target),
- and the 1038 `share/terminfo/` links listed in §6.

**python-floor 3.12.16, darwin-arm64** (10; 8 before their target):
- `bin/2to3 -> 2to3-3.12` (before target)
- `bin/idle3 -> idle3.12` (before target)
- `bin/pydoc3 -> pydoc3.12` (before target)
- `bin/python -> python3.12` (before target)
- `bin/python3 -> python3.12` (before target)
- `bin/python3-config -> python3.12-config` (before target)
- `lib/pkgconfig/python3-embed.pc -> python-3.12-embed.pc` (after target)
- `lib/pkgconfig/python3.pc -> python-3.12.pc` (after target)
- `share/man/man1/python.1 -> python3.12.1` (before target)
- `share/man/man1/python3.1 -> python3.12.1` (before target)

**python-floor 3.12.16, linux-amd64** (1049; 418 before their target): the ten darwin links above, plus
- `lib/libpython3.12.so -> libpython3.12.so.1.0` (before target),
- and the same 1038 `share/terminfo/` links as 3.15.2 (verified identical: same names, same targets), listed in §6.

Of the 1038 terminfo links, 409 come before their target. 738 point into their own directory (`name -> other`), and 300 point into a sibling directory (`share/terminfo/1/1178 -> ../a/adm1178`). Each of those 300 carries a `..` component in its **target** and resolves lexically to `share/terminfo/<x>/<name>`, inside the tree.

**go, golangci-lint, ruff:** no links, on either platform.

## 3. Names colliding case-insensitively

**Only the two linux-amd64 Python archives**, the same 33 pairs in each, all under `share/terminfo/` (directories `A`/`a`, `E`/`e`, … and files within them):

- `share/terminfo/2/2621A` / `share/terminfo/2/2621a`
- `share/terminfo/A` / `share/terminfo/a`
- `share/terminfo/E` / `share/terminfo/e`
- `share/terminfo/E/Eterm` / `share/terminfo/e/eterm`
- `share/terminfo/E/Eterm-color` / `share/terminfo/e/eterm-color`
- `share/terminfo/L` / `share/terminfo/l`
- `share/terminfo/L/LFT-PC850` / `share/terminfo/l/lft-pc850`
- `share/terminfo/M` / `share/terminfo/m`
- `share/terminfo/N` / `share/terminfo/n`
- `share/terminfo/N/NCR260VT300WPP` / `share/terminfo/n/ncr260vt300wpp`
- `share/terminfo/N/NCRVT100WPP` / `share/terminfo/n/ncrvt100wpp`
- `share/terminfo/P` / `share/terminfo/p`
- `share/terminfo/P/P12` / `share/terminfo/p/p12`
- `share/terminfo/P/P12-M` / `share/terminfo/p/p12-m`
- `share/terminfo/P/P12-M-W` / `share/terminfo/p/p12-m-w`
- `share/terminfo/P/P12-W` / `share/terminfo/p/p12-w`
- `share/terminfo/P/P14` / `share/terminfo/p/p14`
- `share/terminfo/P/P14-M` / `share/terminfo/p/p14-m`
- `share/terminfo/P/P14-M-W` / `share/terminfo/p/p14-m-w`
- `share/terminfo/P/P14-W` / `share/terminfo/p/p14-w`
- `share/terminfo/P/P4` / `share/terminfo/p/p4`
- `share/terminfo/P/P5` / `share/terminfo/p/p5`
- `share/terminfo/P/P7` / `share/terminfo/p/p7`
- `share/terminfo/P/P8` / `share/terminfo/p/p8`
- `share/terminfo/P/P8-W` / `share/terminfo/p/p8-w`
- `share/terminfo/P/P9` / `share/terminfo/p/p9`
- `share/terminfo/P/P9-8` / `share/terminfo/p/p9-8`
- `share/terminfo/P/P9-8-W` / `share/terminfo/p/p9-8-w`
- `share/terminfo/P/P9-W` / `share/terminfo/p/p9-w`
- `share/terminfo/Q` / `share/terminfo/q`
- `share/terminfo/X` / `share/terminfo/x`
- `share/terminfo/h/hp2621A` / `share/terminfo/h/hp2621a`
- `share/terminfo/h/hp70092A` / `share/terminfo/h/hp70092a`

None in the darwin archives, go, golangci-lint or ruff. These matter only when a Linux archive is extracted on a case-insensitive filesystem (macOS's default APFS). The installer installs the current platform's archives only, so it never meets them in use. A tree digest of the Linux Pythons cannot be reproduced by extracting on default macOS APFS. Use the archive members, or a case-sensitive volume.

## 4. §3 step 4's rules against this census

| Rule (design note §3 step 4) | Archives it applies to | Verdict |
|---|---|---|
| Accept regular files and directories | all | every regular file and directory member is accepted |
| Accept the member equal to `strip` (the tree root) | go ×2, ruff ×2 | needed: `go` and `ruff-…` are member #0, directories. A literal "starts with `strip`" test would reject them (re-check claim 2) |
| Create parents on demand | python ×4 (173, 216, 153, 196 directories), golangci-lint ×2 (none needed: its members sit in the root) | needed: the Python archives hold **no** directory member |
| Accept symlinks resolving lexically inside the tree, never testing the target exists | python ×4 | needed: 7, 417, 8 and 418 links come before their target; 300 terminfo targets per Linux Python use `..` and stay inside |
| Accept hard links to an earlier member inside the tree | none | no archive has a hard link; the rule is exercised only by test fixtures |
| Reject an absolute path | none | no real member is rejected |
| Reject a `..` component | none | no member **name** has one. A symlink **target** may (300 per Linux Python): the rule applies to names, and targets are judged by where they resolve |
| Reject a member outside `strip` | none | no real member is rejected |
| Reject a link whose target escapes | none | no real link escapes |
| Reject a device or FIFO | none | none present |
| Reject a member whose path passes through an extracted symlink | none | no member's path passes through a symlink |
| Clear all write bits (`& ~0o222`) | python ×4 | needed: their regular files are 0o664 and 0o775 (group-writable, re-check L2) |

**Result: under the §3 rules every member of all ten archives is accepted, and none is rejected.** Two cases the rules do not name, a duplicate member and a name with an empty or `.` component, never occur, so treating them as rejections cannot fail a real install.

## 5. `exec_sha256` against the census

- **go** (both platforms): the ten regular files `bin/go`, `bin/gofmt` and `pkg/tool/<os>_<arch>/{asm,cgo,compile,cover,fix,link,preprofile,vet}` are members, as `tools.json` lists. `bin/go` on Linux is a static ELF (no `PT_INTERP`, no `DT_NEEDED`).
- **python, python-floor, linux-amd64:** `bin/python3.15` and `bin/python3.12` are dynamic ELF executables. Their `DT_NEEDED` lists only system libraries (`libpthread.so.0`, `libdl.so.2`, `libutil.so.1`, `libm.so.6`, `librt.so.1`, `libc.so.6`, plus `libgcc_s.so.1` for 3.15.2), with `RPATH $ORIGIN/../lib`. **Neither needs `libpython3.x.so`**, so, as on darwin, the interpreter is the one executable, and `lib/libpython3.x.so*` ships but is not loaded. The only extension modules shipped are `_dbm` and `_tkinter` (plus `_crypt` in 3.12) under `lib-dynload/`; the rest of the standard library's C code is built into the executable (inferred from the listing). The standard library itself is covered only by `tree_sha256` at T2 and above (design note §3).
- **golangci-lint, ruff:** their single binaries. golangci-lint on Linux is a static ELF; ruff on Linux is dynamic against system libraries only.

## 6. The 1038 `share/terminfo/` links of both linux-amd64 Pythons

Sorted bytewise by name.

```
share/terminfo/1/1178 -> ../a/adm1178
share/terminfo/1/1730-lm -> ../d/diablo1740-lm
share/terminfo/2/2621 -> ../h/hp2621
share/terminfo/2/2621-wl -> ../h/hp2621
share/terminfo/2/2621A -> ../h/hp2621
share/terminfo/2/2621a -> ../h/hp2621
share/terminfo/3/386at -> ../a/att6386
share/terminfo/3/3b1 -> ../a/att7300
share/terminfo/4/4025ex -> ../t/tek4025ex
share/terminfo/4/4027ex -> ../t/tek4025ex
share/terminfo/4/4410-w -> ../a/att5410-w
share/terminfo/5/5051 -> ../i/ibm-pc
share/terminfo/5/5410-w -> ../a/att5410-w
share/terminfo/5/5620 -> ../a/att5620
share/terminfo/5/5630-24 -> ../a/att630-24
share/terminfo/5/5630DMD-24 -> ../a/att630-24
share/terminfo/6/6053 -> ../d/dg6053
share/terminfo/6/6053-dg -> ../d/dg6053
share/terminfo/6/605x -> ../d/dg6053
share/terminfo/6/605x-dg -> ../d/dg6053
share/terminfo/6/630-lm -> ../d/diablo1740-lm
share/terminfo/6/630MTG-24 -> ../a/att630-24
share/terminfo/7/730MTG-24 -> ../a/att730-24
share/terminfo/7/730MTG-41 -> ../a/att730-41
share/terminfo/7/730MTG-41r -> ../a/att730r-41
share/terminfo/7/730MTGr -> ../a/att730r
share/terminfo/7/730MTGr-24 -> ../a/att730r-24
share/terminfo/8/8510 -> ../c/citoh
share/terminfo/9/955-hb -> ../t/tvi955-hb
share/terminfo/9/955-w -> ../t/tvi955-w
share/terminfo/A/Apple_Terminal -> ../n/nsterm
share/terminfo/E/Eterm-color -> Eterm
share/terminfo/L/LFT-PC850 -> ../l/lft
share/terminfo/N/NCR260VT300WPP -> ../n/ncr260vt300wpp
share/terminfo/N/NCRVT100WPP -> ../n/ncrvt100wan
share/terminfo/P/P12 -> ../p/prism12
share/terminfo/P/P12-M -> ../p/prism12-m
share/terminfo/P/P12-M-W -> ../p/prism12-m-w
share/terminfo/P/P12-W -> ../p/prism12-w
share/terminfo/P/P14 -> ../p/prism14
share/terminfo/P/P14-M -> ../p/prism14-m
share/terminfo/P/P14-M-W -> ../p/prism14-m-w
share/terminfo/P/P14-W -> ../p/prism14-w
share/terminfo/P/P4 -> ../p/prism4
share/terminfo/P/P5 -> ../p/prism5
share/terminfo/P/P7 -> ../p/prism7
share/terminfo/P/P8 -> ../p/prism8
share/terminfo/P/P8-W -> ../p/prism8-w
share/terminfo/P/P9 -> ../p/prism9
share/terminfo/P/P9-8 -> ../p/prism9-8
share/terminfo/P/P9-8-W -> ../p/prism9-8-w
share/terminfo/P/P9-W -> ../p/prism9-w
share/terminfo/Q/Q306-8-pc -> ../b/bq300-8-pc
share/terminfo/Q/Q310-vip-H -> ../v/vip-H
share/terminfo/Q/Q310-vip-H-am -> ../v/vip-H
share/terminfo/Q/Q310-vip-Hw -> ../v/vip-Hw
share/terminfo/Q/Q310-vip-w -> ../v/vip-w
share/terminfo/Q/Q310-vip-w-am -> ../v/vip-w
share/terminfo/X/X-hpterm -> ../h/hpterm
share/terminfo/X/X-hpterm-color2 -> ../h/hpterm-color2
share/terminfo/a/a210 -> ampex210
share/terminfo/a/a80 -> ampex80
share/terminfo/a/a980 -> adds980
share/terminfo/a/aa4080 -> annarbor4080
share/terminfo/a/aaa-30 -> aaa
share/terminfo/a/aaa-30-s-ctxt -> aaa-s-ctxt
share/terminfo/a/aaa-30-s-rv-ct -> aaa-s-rv-ctxt
share/terminfo/a/aaa-ctxt -> aaa-30-ctxt
share/terminfo/a/aaa-rv -> aaa-30-rv
share/terminfo/a/aaa-rv-ctxt -> aaa-30-rv-ctxt
share/terminfo/a/aaa-s -> aaa-30-s
share/terminfo/a/aaa-s-rv -> aaa-30-s-rv
share/terminfo/a/aaa-unk -> aaa+unk
share/terminfo/a/adds200 -> ../r/regent60
share/terminfo/a/addsviewpoint -> ../v/viewpoint
share/terminfo/a/addsvp60 -> ../v/vp60
share/terminfo/a/adm1 -> adm1a
share/terminfo/a/aj -> aj830
share/terminfo/a/aj832 -> aj830
share/terminfo/a/alt2 -> altos2
share/terminfo/a/alt3 -> altos3
share/terminfo/a/alt4 -> altos4
share/terminfo/a/alt5 -> altos3
share/terminfo/a/alt7 -> altos7
share/terminfo/a/alt7pc -> altos7pc
share/terminfo/a/alto-heath -> alto-h19
share/terminfo/a/altoh19 -> alto-h19
share/terminfo/a/altoheath -> alto-h19
share/terminfo/a/altos-2 -> altos2
share/terminfo/a/altos-3 -> altos3
share/terminfo/a/altos-4 -> altos4
share/terminfo/a/altos-5 -> altos3
share/terminfo/a/altos5 -> altos3
share/terminfo/a/ambas -> aaa
share/terminfo/a/ambassador -> aaa
share/terminfo/a/amp219 -> ampex219
share/terminfo/a/amp219w -> ampex219w
share/terminfo/a/ampex-219 -> ampex219
share/terminfo/a/ampex-219w -> ampex219w
share/terminfo/a/ampex-232 -> ampex232
share/terminfo/a/ansi-mono -> ansi-m
share/terminfo/a/ansi43m -> ../p/pcansi-43-m
share/terminfo/a/ansi80x25 -> ../c/cons25
share/terminfo/a/ansi80x25-mono -> ../c/cons25-m
share/terminfo/a/ansi80x25-raw -> ../c/cons25w
share/terminfo/a/ansi80x30 -> ../c/cons30
share/terminfo/a/ansi80x30-mono -> ../c/cons30-m
share/terminfo/a/ansi80x43 -> ../c/cons43
share/terminfo/a/ansi80x43-mono -> ../c/cons43-m
share/terminfo/a/ansi80x50 -> ../c/cons50
share/terminfo/a/ansi80x50-mono -> ../c/cons50-m
share/terminfo/a/ansi80x60 -> ../c/cons60
share/terminfo/a/ansi80x60-mono -> ../c/cons60-m
share/terminfo/a/ansil -> ../c/cons50
share/terminfo/a/ansil-mono -> ../c/cons50-m
share/terminfo/a/ansis -> ../c/cons25
share/terminfo/a/ansis-mono -> ../c/cons25-m
share/terminfo/a/ansisysk -> ansi.sysk
share/terminfo/a/ansiterm -> ansi-generic
share/terminfo/a/ansiw -> ../c/cons25w
share/terminfo/a/ap-vm80 -> apple-vm80
share/terminfo/a/apl -> ../i/ibm-apl
share/terminfo/a/appleIIc -> appleIIgs
share/terminfo/a/appleIIe -> appleIIgs
share/terminfo/a/arm100-am -> arm100
share/terminfo/a/arm100-wam -> arm100-w
share/terminfo/a/at -> ../s/st52
share/terminfo/a/at-color -> ../s/st52-color
share/terminfo/a/at-m -> ../s/st52
share/terminfo/a/at386 -> att6386
share/terminfo/a/atari -> ../s/st52
share/terminfo/a/atari-color -> ../s/st52-color
share/terminfo/a/atari-m -> ../s/st52
share/terminfo/a/atari_st -> ../s/st52
share/terminfo/a/atari_st-color -> ../s/st52-color
share/terminfo/a/atarist-m -> ../s/st52
share/terminfo/a/att4410-w -> att5410-w
share/terminfo/a/att4410v1 -> att5410v1
share/terminfo/a/att4425 -> att5425
share/terminfo/a/att4425-nl -> att5425-nl
share/terminfo/a/att4425-w -> att5425-w
share/terminfo/a/att513 -> att500
share/terminfo/a/att5320 -> att5310
share/terminfo/a/att5410 -> att4410
share/terminfo/a/att5410v1-w -> att4410v1-w
share/terminfo/a/att5418 -> att4418
share/terminfo/a/att5418-w -> att4418-w
share/terminfo/a/att5420 -> att4415
share/terminfo/a/att5420+nl -> att4415+nl
share/terminfo/a/att5420-nl -> att4415-nl
share/terminfo/a/att5420-rv -> att4415-rv
share/terminfo/a/att5420-rv-nl -> att4415-rv-nl
share/terminfo/a/att5420-w -> att4415-w
share/terminfo/a/att5420-w-nl -> att4415-w-nl
share/terminfo/a/att5420-w-rv -> att4415-w-rv
share/terminfo/a/att5420-w-rv-n -> att4415-w-rv-n
share/terminfo/a/att5430 -> att505
share/terminfo/a/avatar1 -> avatar
share/terminfo/a/avt-rv-s -> avt-rv
share/terminfo/a/avt-s -> avt
share/terminfo/a/avt-w-rv-s -> avt-w-rv
share/terminfo/a/avt-w-s -> avt-w
share/terminfo/b/b-128 -> ../c/commodore
share/terminfo/b/basic4 -> ../m/mai
share/terminfo/b/bct510a -> ../a/att510a
share/terminfo/b/bct510d -> ../a/att510d
share/terminfo/b/bee -> beehive
share/terminfo/b/beehiveIIIm -> beehive3
share/terminfo/b/bg2.0nv -> bitgraph
share/terminfo/b/bg3.10 -> bg2.0
share/terminfo/b/bg3.10nv -> bitgraph
share/terminfo/b/bg3.10rv -> bg2.0rv
share/terminfo/b/bh3m -> beehive3
share/terminfo/b/bh4 -> beehive4
share/terminfo/b/bsdos-pc-mono -> bsdos-pc-m
share/terminfo/c/c100-1p -> ../o/oc100
share/terminfo/c/c100-4p -> c100
share/terminfo/c/c100-rv-4p -> c100-rv
share/terminfo/c/c104 -> c100
share/terminfo/c/c108-8p -> c108
share/terminfo/c/c108-rv-8p -> c108-rv
share/terminfo/c/c108-w-8p -> c108-w
share/terminfo/c/c300 -> contel300
share/terminfo/c/c301 -> contel301
share/terminfo/c/c321 -> contel301
share/terminfo/c/cci1 -> cci
share/terminfo/c/cgc2 -> cad68-2
share/terminfo/c/cgc3 -> cad68-3
share/terminfo/c/chromatics -> cg7900
share/terminfo/c/ci8510 -> citoh
share/terminfo/c/cit-80 -> cit80
share/terminfo/c/citc -> cit101
share/terminfo/c/citoh-ps -> citoh-prop
share/terminfo/c/coherent -> ../p/pc-coherent
share/terminfo/c/colorscan -> cs10
share/terminfo/c/concept -> c100
share/terminfo/c/concept-avt -> ../a/avt
share/terminfo/c/concept100 -> c100
share/terminfo/c/concept100-rv -> c100-rv
share/terminfo/c/concept108 -> c108
share/terminfo/c/concept108-4p -> c108-4p
share/terminfo/c/concept108-8p -> c108
share/terminfo/c/concept108-w-8 -> c108-w
share/terminfo/c/concept108-w8p -> c108-w
share/terminfo/c/concept108rv4p -> c108-rv-4p
share/terminfo/c/cons25-iso-m -> cons25l1-m
share/terminfo/c/cons25-iso8859 -> cons25l1
share/terminfo/c/cons25-koi8-r -> cons25r
share/terminfo/c/cons25-koi8r-m -> cons25r-m
share/terminfo/c/cons50-iso-m -> cons50l1-m
share/terminfo/c/cons50-iso8859 -> cons50l1
share/terminfo/c/cons50-koi8r -> cons50r
share/terminfo/c/cons50-koi8r-m -> cons50r-m
share/terminfo/c/cons60-iso -> cons60l1
share/terminfo/c/cons60-iso-m -> cons60l1-m
share/terminfo/c/cons60-koi8r -> cons60r
share/terminfo/c/cons60-koi8r-m -> cons60r-m
share/terminfo/c/contel320 -> contel300
share/terminfo/c/contel321 -> contel301
share/terminfo/c/contour-latest -> contour
share/terminfo/c/cops -> cops10
share/terminfo/c/cops-10 -> cops10
share/terminfo/c/crt-vt220 -> crt
share/terminfo/c/ct82 -> ../s/swtp
share/terminfo/c/cx -> color_xterm
share/terminfo/c/cx100 -> color_xterm
share/terminfo/d/d2 -> dg6053
share/terminfo/d/d2-dg -> dg6053
share/terminfo/d/d200-dg -> d200
share/terminfo/d/d214 -> d210
share/terminfo/d/d214-dg -> d210-dg
share/terminfo/d/d215 -> d211
share/terminfo/d/d215-7b -> d211-7b
share/terminfo/d/d215-dg -> d211-dg
share/terminfo/d/d216+ -> d216-unix
share/terminfo/d/d216+25 -> d216-unix-25
share/terminfo/d/d216+dg -> d216-dg
share/terminfo/d/d216e+ -> d216-unix
share/terminfo/d/d216e+dg -> d216-dg
share/terminfo/d/d216e-dg -> d216-dg
share/terminfo/d/d216e-unix -> d216-unix
share/terminfo/d/d217-dg -> d216-dg
share/terminfo/d/d230 -> d230c
share/terminfo/d/d230-dg -> d230c-dg
share/terminfo/d/d400-dg -> d400
share/terminfo/d/d411 -> d410
share/terminfo/d/d411-7b -> d410-7b
share/terminfo/d/d411-7b-w -> d410-7b-w
share/terminfo/d/d411-dg -> d410-dg
share/terminfo/d/d411-w -> d410-w
share/terminfo/d/d412+ -> d412-unix
share/terminfo/d/d412+25 -> d412-unix-25
share/terminfo/d/d412+dg -> d412-dg
share/terminfo/d/d412+s -> d412-unix-s
share/terminfo/d/d412+sr -> d412-unix-sr
share/terminfo/d/d412+w -> d412-unix-w
share/terminfo/d/d413-dg -> d412-dg
share/terminfo/d/d430-dg -> d430c-dg
share/terminfo/d/d430-dg-ccc -> d430c-dg-ccc
share/terminfo/d/d430-unix -> d430c-unix
share/terminfo/d/d430-unix-25 -> d430c-unix-25
share/terminfo/d/d430-unix-25-ccc -> d430c-unix-25-ccc
share/terminfo/d/d430-unix-ccc -> d430c-unix-ccc
share/terminfo/d/d430-unix-s -> d430c-unix-s
share/terminfo/d/d430-unix-s-ccc -> d430c-unix-s-ccc
share/terminfo/d/d430-unix-sr -> d430c-unix-sr
share/terminfo/d/d430-unix-sr-ccc -> d430c-unix-sr-ccc
share/terminfo/d/d430-unix-w -> d430c-unix-w
share/terminfo/d/d430-unix-w-ccc -> d430c-unix-w-ccc
share/terminfo/d/d450 -> d400
share/terminfo/d/d450-dg -> d400
share/terminfo/d/d460 -> d410
share/terminfo/d/d460-7b -> d410-7b
share/terminfo/d/d460-7b-w -> d410-7b-w
share/terminfo/d/d460-dg -> d410-dg
share/terminfo/d/d460-w -> d410-w
share/terminfo/d/d461 -> d410
share/terminfo/d/d461-7b -> d410-7b
share/terminfo/d/d461-7b-w -> d410-7b-w
share/terminfo/d/d461-dg -> d410-dg
share/terminfo/d/d461-w -> d410-w
share/terminfo/d/d462+ -> d412-unix
share/terminfo/d/d462+25 -> d412-unix-25
share/terminfo/d/d462+dg -> d412-dg
share/terminfo/d/d462+s -> d412-unix-s
share/terminfo/d/d462+sr -> d412-unix-sr
share/terminfo/d/d462+w -> d412-unix-w
share/terminfo/d/d462-dg -> d412-dg
share/terminfo/d/d462-unix -> d412-unix
share/terminfo/d/d462-unix-25 -> d412-unix-25
share/terminfo/d/d462-unix-s -> d412-unix-s
share/terminfo/d/d462-unix-sr -> d412-unix-sr
share/terminfo/d/d462-unix-w -> d412-unix-w
share/terminfo/d/d462e-dg -> d412-dg
share/terminfo/d/d463-dg -> d412-dg
share/terminfo/d/d463-unix -> d413-unix
share/terminfo/d/d463-unix-25 -> d413-unix-25
share/terminfo/d/d463-unix-s -> d413-unix-s
share/terminfo/d/d463-unix-sr -> d413-unix-sr
share/terminfo/d/d463-unix-w -> d413-unix-w
share/terminfo/d/d464-unix -> d414-unix
share/terminfo/d/d464-unix-25 -> d414-unix-25
share/terminfo/d/d464-unix-s -> d414-unix-s
share/terminfo/d/d464-unix-sr -> d414-unix-sr
share/terminfo/d/d464-unix-w -> d414-unix-w
share/terminfo/d/d470 -> d470c
share/terminfo/d/d470-7b -> d470c-7b
share/terminfo/d/d470-dg -> d470c-dg
share/terminfo/d/d578-dg -> d577-dg
share/terminfo/d/d80 -> ../a/ampex80
share/terminfo/d/darwin -> ../x/xnuppc
share/terminfo/d/darwin-100x37 -> ../x/xnuppc-100x37
share/terminfo/d/darwin-100x37-m -> ../x/xnuppc-100x37-m
share/terminfo/d/darwin-112x37 -> ../x/xnuppc-112x37
share/terminfo/d/darwin-112x37-m -> ../x/xnuppc-112x37-m
share/terminfo/d/darwin-128x40 -> ../x/xnuppc-128x40
share/terminfo/d/darwin-128x40-m -> ../x/xnuppc-128x40-m
share/terminfo/d/darwin-128x48 -> ../x/xnuppc-128x48
share/terminfo/d/darwin-128x48-m -> ../x/xnuppc-128x48-m
share/terminfo/d/darwin-144x48 -> ../x/xnuppc-144x48
share/terminfo/d/darwin-144x48-m -> ../x/xnuppc-144x48-m
share/terminfo/d/darwin-160x64 -> ../x/xnuppc-160x64
share/terminfo/d/darwin-160x64-m -> ../x/xnuppc-160x64-m
share/terminfo/d/darwin-200x64 -> ../x/xnuppc-200x64
share/terminfo/d/darwin-200x64-m -> ../x/xnuppc-200x64-m
share/terminfo/d/darwin-200x75 -> ../x/xnuppc-200x75
share/terminfo/d/darwin-200x75-m -> ../x/xnuppc-200x75-m
share/terminfo/d/darwin-256x96 -> ../x/xnuppc-256x96
share/terminfo/d/darwin-256x96-m -> ../x/xnuppc-256x96-m
share/terminfo/d/darwin-80x25 -> ../x/xnuppc-80x25
share/terminfo/d/darwin-80x25-m -> ../x/xnuppc-80x25-m
share/terminfo/d/darwin-80x30 -> ../x/xnuppc-80x30
share/terminfo/d/darwin-80x30-m -> ../x/xnuppc-80x30-m
share/terminfo/d/darwin-90x30 -> ../x/xnuppc-90x30
share/terminfo/d/darwin-90x30-m -> ../x/xnuppc-90x30-m
share/terminfo/d/darwin-b -> ../x/xnuppc-b
share/terminfo/d/darwin-f -> ../x/xnuppc-f
share/terminfo/d/darwin-f2 -> ../x/xnuppc-f2
share/terminfo/d/darwin-m -> ../x/xnuppc-m
share/terminfo/d/darwin-m-b -> ../x/xnuppc-m-b
share/terminfo/d/darwin-m-f -> ../x/xnuppc-m-f
share/terminfo/d/darwin-m-f2 -> ../x/xnuppc-m-f2
share/terminfo/d/datagraphix -> d132
share/terminfo/d/datamedia2500 -> dm2500
share/terminfo/d/datapoint -> dp3360
share/terminfo/d/dataspeed40 -> ../t/tty40
share/terminfo/d/dd5000 -> delta
share/terminfo/d/ddr3180 -> ddr
share/terminfo/d/dec-vt330 -> ../v/vt340
share/terminfo/d/dec-vt340 -> ../v/vt340
share/terminfo/d/dec-vt400 -> ../v/vt400
share/terminfo/d/decpro -> ../p/pro350
share/terminfo/d/decwriter -> dw2
share/terminfo/d/dg-ansi -> dg210
share/terminfo/d/dg100 -> dg6053-old
share/terminfo/d/dg605x -> dg6053
share/terminfo/d/dg6134 -> dg450
share/terminfo/d/diablo -> diablo1640
share/terminfo/d/diablo-lm -> diablo1640-lm
share/terminfo/d/diablo1640-m8 -> diablo1620-m8
share/terminfo/d/diablo1720 -> diablo1620
share/terminfo/d/diablo1730 -> diablo1640
share/terminfo/d/diablo1740 -> diablo1640
share/terminfo/d/diablo450 -> diablo1620
share/terminfo/d/diablo630 -> diablo1640
share/terminfo/d/dialogue -> ../a/ampex80
share/terminfo/d/dialogue80 -> ../a/ampex80
share/terminfo/d/dku7102 -> ../t/tws-generic
share/terminfo/d/dku7102-sna -> ../t/tws2102-sna
share/terminfo/d/dku7103-sna -> ../t/tws2103-sna
share/terminfo/d/dm1521 -> dm1520
share/terminfo/d/dmd -> ../a/att5620
share/terminfo/d/dmd-24 -> ../a/att5620-24
share/terminfo/d/dmd-34 -> ../a/att5620-34
share/terminfo/d/dmd1 -> ../a/att5620-1
share/terminfo/d/dmdt80 -> dm80
share/terminfo/d/dmdt80w -> dm80w
share/terminfo/d/ds40 -> ../t/tty40
share/terminfo/d/ds40-2 -> ../t/tty40
share/terminfo/d/dt-100 -> dt100
share/terminfo/d/dt-100w -> dt100w
share/terminfo/d/dt80 -> dm80
share/terminfo/d/dt80w -> dm80w
share/terminfo/d/dw -> dw2
share/terminfo/d/dwk-vt -> dwk
share/terminfo/e/emots -> ../g/gs6300
share/terminfo/e/envision230 -> env230
share/terminfo/e/ep4000 -> ep40
share/terminfo/e/ep4080 -> ep48
share/terminfo/e/excel64 -> excel62
share/terminfo/e/excel64-rv -> excel62-rv
share/terminfo/e/excel64-w -> excel62-w
share/terminfo/e/exec80 -> ../h/hazel
share/terminfo/f/f1720a -> f1720
share/terminfo/f/fenix -> ../p/pt100
share/terminfo/f/fenixw -> ../p/pt100w
share/terminfo/f/fixterm -> ../c/cbblit
share/terminfo/f/fortune -> fos
share/terminfo/f/freedom -> f100
share/terminfo/f/freedom-rv -> f100-rv
share/terminfo/f/freedom100 -> f100
share/terminfo/f/freedom110 -> f110
share/terminfo/f/freedom200 -> f200
share/terminfo/g/go-225 -> go225
share/terminfo/g/gs5430 -> ../a/att505
share/terminfo/g/gs5430-22 -> ../a/att505-22
share/terminfo/g/gs5430-24 -> ../a/att505-24
share/terminfo/g/gt100 -> ../i/i100
share/terminfo/g/gt100a -> ../i/i100
share/terminfo/g/guru+unk -> guru
share/terminfo/g/guru-33 -> guru
share/terminfo/g/guru-33-rv -> guru-rv
share/terminfo/g/guru-33-s -> guru-s
share/terminfo/g/guru-lp -> guru-76-lp
share/terminfo/h/h-100 -> ../z/z100
share/terminfo/h/h-100bw -> ../z/z100bw
share/terminfo/h/h100 -> ../z/z100
share/terminfo/h/h100bw -> ../z/z100bw
share/terminfo/h/h19-b -> h19
share/terminfo/h/h19-smul -> h19-us
share/terminfo/h/h19a -> h19-a
share/terminfo/h/h19g -> h19-g
share/terminfo/h/h19kermit -> h19k
share/terminfo/h/h19us -> h19-us
share/terminfo/h/h29a-kc-bc -> ../z/z29a
share/terminfo/h/h29a-kc-uc -> ../z/z29a-kc-uc
share/terminfo/h/h29a-nkc-bc -> ../z/z29a-nkc-bc
share/terminfo/h/h29a-nkc-uc -> ../z/z29a-nkc-uc
share/terminfo/h/h80 -> hazel
share/terminfo/h/he80 -> hazel
share/terminfo/h/heath -> h19
share/terminfo/h/heath-19 -> h19
share/terminfo/h/heath-ansi -> h19-a
share/terminfo/h/heathkit -> h19
share/terminfo/h/heathkit-a -> h19-a
share/terminfo/h/hft -> ../i/ibm5081
share/terminfo/h/hp -> hpgeneric
share/terminfo/h/hp2382 -> hp2382a
share/terminfo/h/hp2397 -> hp2397a
share/terminfo/h/hp2621-wl -> hp2621
share/terminfo/h/hp2621A -> hp2621
share/terminfo/h/hp2621a -> hp2621
share/terminfo/h/hp2621a-a -> hp2621-a
share/terminfo/h/hp2621k45 -> hp2621-k45
share/terminfo/h/hp2622a -> hp2622
share/terminfo/h/hp2623a -> hp2623
share/terminfo/h/hp2624a -> hp2624
share/terminfo/h/hp2624a-10p -> hp2624-10p
share/terminfo/h/hp2624b -> hp2624
share/terminfo/h/hp2624b-10p -> hp2624-10p
share/terminfo/h/hp2624b-4p -> hp2624
share/terminfo/h/hp2624b-4p-p -> hp2624b-p
share/terminfo/h/hp2626a -> hp2626
share/terminfo/h/hp2626p -> hp2626
share/terminfo/h/hp2644a -> hp2640b
share/terminfo/h/hp2645a -> hp2641a
share/terminfo/h/hp2647a -> hp2641a
share/terminfo/h/hp2648a -> hp2648
share/terminfo/h/hp45 -> hp2645
share/terminfo/h/hp700 -> hpansi
share/terminfo/h/hp70092A -> hp70092
share/terminfo/h/hp70092a -> hp70092
share/terminfo/h/hp98550a -> hp98550
share/terminfo/h/hp98550a-color -> hp98550-color
share/terminfo/h/hp98720 -> hp9837
share/terminfo/h/hp98721 -> hp9837
share/terminfo/h/hpex2 -> hp2
share/terminfo/h/htx11 -> ../z/ztx
share/terminfo/i/i3101 -> ibm3101
share/terminfo/i/i3164 -> ibm3164
share/terminfo/i/ibm3163 -> ibm3161
share/terminfo/i/ibm5051 -> ibm-pc
share/terminfo/i/ibm5154-c -> ibmega-c
share/terminfo/i/ibm6154-c -> ibmapa8c-c
share/terminfo/i/ibm8507 -> ibm8503
share/terminfo/i/ibm8513 -> ibm8512
share/terminfo/i/ibm8604 -> ibm8503
share/terminfo/i/ibmapa16 -> ../r/rtpc
share/terminfo/i/ibmapa8 -> ibmapa8c
share/terminfo/i/ibmmpel-c -> ibm5081-c
share/terminfo/i/ibmpc3r -> ../c/cons25r
share/terminfo/i/ibmpc3r-mono -> ../c/cons25r-m
share/terminfo/i/ibmx -> ibmpcx
share/terminfo/i/icl6402 -> icl6404
share/terminfo/i/intertec -> intertube
share/terminfo/i/intextii -> intext2
share/terminfo/i/ips -> ../c/citoh-prop
share/terminfo/i/ipsi -> ../d/diablo1620
share/terminfo/i/iq120 -> ../s/soroc120
share/terminfo/i/iq140 -> ../s/soroc140
share/terminfo/i/iris-ansi-net -> iris-ansi
share/terminfo/i/iris40 -> ../w/wsiris
share/terminfo/i/iterm -> iTerm.app
share/terminfo/i/iterm2 -> iTerm2.app
share/terminfo/j/jerq -> ../b/blit
share/terminfo/k/k45 -> ../h/hp2621-k45
share/terminfo/k/kaypro2 -> kaypro
share/terminfo/k/kds6402 -> ../i/icl6404
share/terminfo/k/kds7372 -> ../i/icl6404
share/terminfo/k/kds7372-w -> ../i/icl6404-w
share/terminfo/k/kon2 -> kon
share/terminfo/k/kterm-co -> kterm-color
share/terminfo/k/ktm -> ../s/synertek
share/terminfo/l/la120 -> ../d/dw3
share/terminfo/l/layer -> ../a/att5620-s
share/terminfo/l/lft-pc850 -> lft
share/terminfo/l/luna68k -> luna
share/terminfo/m/macintosh -> mac
share/terminfo/m/macterminal-w -> mac-w
share/terminfo/m/mdl110 -> ../c/cyb110
share/terminfo/m/microbee -> microb
share/terminfo/m/microterm -> ../a/act4
share/terminfo/m/microterm5 -> ../a/act5
share/terminfo/m/mime-3ax -> mime3ax
share/terminfo/m/mime1 -> mime
share/terminfo/m/mime2 -> mime
share/terminfo/m/mime2a-v -> mime2a
share/terminfo/m/mime340 -> mm340
share/terminfo/m/mimei -> mime
share/terminfo/m/mimeii -> mime
share/terminfo/m/minitel -> m2-nam
share/terminfo/m/minitel-2 -> m2-nam
share/terminfo/m/minitel-2-nam -> m2-nam
share/terminfo/m/minix-1.5 -> minix-old
share/terminfo/m/mm314 -> mime314
share/terminfo/m/mod -> modgraph48
share/terminfo/m/mod24 -> modgraph
share/terminfo/m/mouse-sun -> mterm
share/terminfo/m/mskermit227 -> msk227
share/terminfo/m/mskermit22714 -> msk22714
share/terminfo/m/mskermit227am -> msk227am
share/terminfo/m/mt-70 -> mt70
share/terminfo/n/n7900 -> ncr7900i
share/terminfo/n/nansisys -> nansi.sys
share/terminfo/n/nansisysk -> nansi.sysk
share/terminfo/n/ncr7900 -> ncr7900i
share/terminfo/n/ncrvt100pp -> ncrvt100an
share/terminfo/n/ncrvt100wpp -> ncrvt100wan
share/terminfo/n/ncsa-vt220-8 -> ncsa-m
share/terminfo/n/nd9500 -> ndr9500
share/terminfo/n/nec -> nec5520
share/terminfo/n/news -> nwp512
share/terminfo/n/news-a -> nwp512-a
share/terminfo/n/news-o -> nwp512
share/terminfo/n/news28-a -> news29
share/terminfo/n/news31 -> nwp513
share/terminfo/n/news31-a -> nwp513-a
share/terminfo/n/news31-o -> nwp513
share/terminfo/n/news33 -> nwp513-a
share/terminfo/n/news40 -> nwp512
share/terminfo/n/news40-a -> nwp512-a
share/terminfo/n/news40-o -> nwp512
share/terminfo/n/news42 -> nwp512-a
share/terminfo/n/newscbm -> nwp513
share/terminfo/n/newscbm-a -> nwp513-a
share/terminfo/n/newscbm-o -> nwp513
share/terminfo/n/newscbm33 -> nwp513-a
share/terminfo/n/nsterm-256color -> nsterm
share/terminfo/n/nsterm-7-c-s -> nsterm-c-s-7
share/terminfo/n/nsterm-7-m -> nsterm-m-7
share/terminfo/n/nsterm-7-m-s -> nsterm-m-s-7
share/terminfo/n/nsterm-7-s -> nsterm-s-7
share/terminfo/n/nsterm-acs-c -> nsterm-c-acs
share/terminfo/n/nsterm-acs-c-s -> nsterm-c-s-acs
share/terminfo/n/nsterm-acs-m -> nsterm-m-acs
share/terminfo/n/nsterm-acs-m-s -> nsterm-m-s-acs
share/terminfo/n/nsterm-acs-s -> nsterm-s-acs
share/terminfo/n/nsterm-c-7 -> nsterm-7-c
share/terminfo/n/ntconsole -> ../i/interix
share/terminfo/n/ntconsole-100 -> ../o/opennt-100
share/terminfo/n/ntconsole-100-nti -> ../o/opennt-100-nti
share/terminfo/n/ntconsole-25 -> ../i/interix
share/terminfo/n/ntconsole-25-nti -> ../i/interix-nti
share/terminfo/n/ntconsole-25-w -> ../o/opennt-w
share/terminfo/n/ntconsole-25-w-vt -> ../o/opennt-w-vt
share/terminfo/n/ntconsole-35 -> ../o/opennt-35
share/terminfo/n/ntconsole-35-nti -> ../o/opennt-35-nti
share/terminfo/n/ntconsole-35-w -> ../o/opennt-35-w
share/terminfo/n/ntconsole-50 -> ../o/opennt-50
share/terminfo/n/ntconsole-50-nti -> ../o/opennt-50-nti
share/terminfo/n/ntconsole-50-w -> ../o/opennt-50-w
share/terminfo/n/ntconsole-60 -> ../o/opennt-60
share/terminfo/n/ntconsole-60-nti -> ../o/opennt-60-nti
share/terminfo/n/ntconsole-60-w -> ../o/opennt-60-w
share/terminfo/n/ntconsole-w -> ../o/opennt-w
share/terminfo/n/ntconsole-w-vt -> ../o/opennt-w-vt
share/terminfo/n/nwe501 -> nwp513
share/terminfo/n/nwe501-a -> nwp513-a
share/terminfo/n/nwe501-o -> nwp513
share/terminfo/n/nwp-511 -> nwp511
share/terminfo/n/nwp-517 -> nwp517
share/terminfo/n/nwp-517-w -> nwp517-w
share/terminfo/n/nwp251-a -> nwp513-a
share/terminfo/n/nwp251-o -> nwp513
share/terminfo/n/nwp512-o -> nwp512
share/terminfo/n/nwp513-o -> nwp513
share/terminfo/n/nwp514 -> nwp512
share/terminfo/n/nwp514-a -> nwp512-a
share/terminfo/n/nwp514-o -> nwp512
share/terminfo/n/nwp518 -> nwp513
share/terminfo/n/nwp518-a -> nwp513-a
share/terminfo/n/nwp518-o -> nwp513
share/terminfo/n/nxterm -> ../x/xterm-color
share/terminfo/o/o31 -> ../a/adm31-old
share/terminfo/o/o4112-nd -> otek4112
share/terminfo/o/o85h -> ../a/abm85h-old
share/terminfo/o/oabm85h -> ../a/abm85h-old
share/terminfo/o/oconcept -> oc100
share/terminfo/o/ojerq -> oblit
share/terminfo/o/old-st -> ../s/simpleterm
share/terminfo/o/oldibmpc3 -> oldpc3
share/terminfo/o/opennt -> ../i/interix
share/terminfo/o/opennt-25 -> ../i/interix
share/terminfo/o/opennt-25-nti -> ../i/interix-nti
share/terminfo/o/opennt-25-w -> opennt-w
share/terminfo/o/opennt-25-w-vt -> opennt-w-vt
share/terminfo/o/opennt-nti -> ../i/interix-nti
share/terminfo/o/origibmpc3 -> origpc3
share/terminfo/o/os9LII -> ../c/coco3
share/terminfo/o/osborne1 -> osborne
share/terminfo/o/osborne1-w -> osborne-w
share/terminfo/o/otek4113 -> otek4112
share/terminfo/o/otek4114 -> otek4112
share/terminfo/p/p12 -> prism12
share/terminfo/p/p12-m -> prism12-m
share/terminfo/p/p12-m-w -> prism12-m-w
share/terminfo/p/p12-w -> prism12-w
share/terminfo/p/p14 -> prism14
share/terminfo/p/p14-m -> prism14-m
share/terminfo/p/p14-m-w -> prism14-m-w
share/terminfo/p/p14-w -> prism14-w
share/terminfo/p/p4 -> prism4
share/terminfo/p/p5 -> prism5
share/terminfo/p/p7 -> prism7
share/terminfo/p/p8 -> prism8
share/terminfo/p/p8-w -> prism8-w
share/terminfo/p/p9 -> prism9
share/terminfo/p/p9-8 -> prism9-8
share/terminfo/p/p9-8-w -> prism9-8-w
share/terminfo/p/p9-w -> prism9-w
share/terminfo/p/pc3-bold -> ../i/ibmpc3
share/terminfo/p/pc3r -> ../c/cons25r
share/terminfo/p/pc3r-m -> ../c/cons25r-m
share/terminfo/p/pc7300 -> ../a/att7300
share/terminfo/p/pcansi-mono -> pcansi-m
share/terminfo/p/pcansi25 -> pcansi-25
share/terminfo/p/pcansi25m -> pcansi-25-m
share/terminfo/p/pcansi33 -> pcansi-33
share/terminfo/p/pcansi33m -> pcansi-33-m
share/terminfo/p/pcansi43 -> pcansi-43
share/terminfo/p/pcconsole -> pccons
share/terminfo/p/pckermit12 -> pckermit
share/terminfo/p/pcz19 -> pc-coherent
share/terminfo/p/pe1100 -> ../f/fox
share/terminfo/p/pe1200 -> ../o/owl
share/terminfo/p/pe550 -> ../b/bantam
share/terminfo/p/pe6100 -> ../b/bantam
share/terminfo/p/pe6300 -> pe1251
share/terminfo/p/pe6312 -> pe1251
share/terminfo/p/pmconsole -> pmcons
share/terminfo/p/printer -> ../l/lpr
share/terminfo/p/prism8gl -> p8gl
share/terminfo/p/psterm-basic -> psterm
share/terminfo/p/psx_ansi -> ../a/ansi-nt
share/terminfo/p/pt200 -> pt100
share/terminfo/p/pt200w -> pt100w
share/terminfo/p/pt505 -> ../a/att505
share/terminfo/p/pt505-22 -> ../a/att505-22
share/terminfo/p/pt505-24 -> ../a/att505-24
share/terminfo/q/qdcons -> qdss
share/terminfo/q/qnx4 -> qnx
share/terminfo/q/qnxt4 -> qnxt
share/terminfo/q/qume -> qume5
share/terminfo/q/qvt101p -> qvt101+
share/terminfo/q/qvt108 -> qvt101
share/terminfo/q/qvt119 -> qvt119+
share/terminfo/q/qvt119-25-w -> qvt119+-25-w
share/terminfo/q/qvt119-w -> qvt119+-w
share/terminfo/q/qvt119p -> qvt119+
share/terminfo/q/qvt119p-25 -> qvt119+-25
share/terminfo/q/qvt119p-25-w -> qvt119+-25-w
share/terminfo/q/qvt119p-w -> qvt119+-w
share/terminfo/q/qvt203+ -> qvt203
share/terminfo/q/qvt203-w-am -> qvt203-w
share/terminfo/r/rebus3180 -> ../d/ddr
share/terminfo/r/regent200 -> regent60
share/terminfo/r/rxvt-color -> rxvt
share/terminfo/s/s4 -> ../a/att7300
share/terminfo/s/sb3 -> sb2
share/terminfo/s/sbobcat -> ../b/bobcat
share/terminfo/s/sc410 -> scanset
share/terminfo/s/sc415 -> scanset
share/terminfo/s/screen.linux-s -> screen.linux
share/terminfo/s/screen.minitel12-80 -> screen.minitel1b-80
share/terminfo/s/screen.minitel2-80 -> screen.minitel1b-80
share/terminfo/s/screen.xterm-new -> screen.xterm-xfree86
share/terminfo/s/securecrt -> scrt
share/terminfo/s/soroc -> soroc120
share/terminfo/s/spinwriter -> ../n/nec5520
share/terminfo/s/st52-m -> st52
share/terminfo/s/stterm -> st
share/terminfo/s/stterm-16color -> st-16color
share/terminfo/s/stterm-256color -> st-256color
share/terminfo/s/sun-cmd -> sun-c
share/terminfo/s/sun-nic -> sun-e
share/terminfo/s/sun-s-e -> sun-e-s
share/terminfo/s/sun-ss5 -> sun-cgsix
share/terminfo/s/sun1 -> sun
share/terminfo/s/sun2 -> sun
share/terminfo/s/sune -> sun-e
share/terminfo/s/superbee -> sbi
share/terminfo/s/sv80 -> ../a/att2300
share/terminfo/s/synertek380 -> synertek
share/terminfo/s/system1 -> ../i/ibm-system1
share/terminfo/t/t653x -> tandem653
share/terminfo/t/tab -> tab132
share/terminfo/t/tab132-15 -> tab132
share/terminfo/t/tek4012 -> tek
share/terminfo/t/tek4025 -> tek4024
share/terminfo/t/tek4027 -> tek4024
share/terminfo/t/tek4027-ex -> tek4025-ex
share/terminfo/t/tek4107brl -> tek4106brl
share/terminfo/t/tek4109 -> tek4107
share/terminfo/t/tek4109brl -> tek4106brl
share/terminfo/t/tek4114 -> tek4112
share/terminfo/t/teken-vt -> teken
share/terminfo/t/teleray -> t1061
share/terminfo/t/terminet -> terminet1200
share/terminfo/t/terminet300 -> terminet1200
share/terminfo/t/tgtelnet -> ../p/pilot
share/terminfo/t/ti707 -> ti703
share/terminfo/t/ti707-w -> ti703-w
share/terminfo/t/ti733 -> ti700
share/terminfo/t/ti735 -> ti700
share/terminfo/t/ti745 -> ti700
share/terminfo/t/ti800 -> ti700
share/terminfo/t/ti916-220-7 -> ti916
share/terminfo/t/ti916-220-8 -> ti916-8
share/terminfo/t/tkterm -> tt
share/terminfo/t/tn1200 -> terminet1200
share/terminfo/t/tn300 -> terminet1200
share/terminfo/t/trs80II -> trs2
share/terminfo/t/trsII -> trs2
share/terminfo/t/ts-1 -> ../f/falco
share/terminfo/t/ts-1p -> ../f/falco-p
share/terminfo/t/ts1 -> ../f/falco
share/terminfo/t/ts100-sp -> ts100
share/terminfo/t/ts1p -> ../f/falco-p
share/terminfo/t/tty35 -> tty33
share/terminfo/t/tty4420 -> ../a/att4420
share/terminfo/t/tty4424 -> ../a/att4424
share/terminfo/t/tty4424-1 -> ../a/att4424-1
share/terminfo/t/tty4424m -> ../a/att4424m
share/terminfo/t/tty4426 -> ../a/att4426
share/terminfo/t/tty5410 -> ../a/att4410
share/terminfo/t/tty5410-w -> ../a/att5410-w
share/terminfo/t/tty5410v1 -> ../a/att5410v1
share/terminfo/t/tty5410v1-w -> ../a/att4410v1-w
share/terminfo/t/tty5420 -> ../a/att4415
share/terminfo/t/tty5420+nl -> ../a/att4415+nl
share/terminfo/t/tty5420-nl -> ../a/att4415-nl
share/terminfo/t/tty5420-rv -> ../a/att4415-rv
share/terminfo/t/tty5420-rv-nl -> ../a/att4415-rv-nl
share/terminfo/t/tty5420-w -> ../a/att4415-w
share/terminfo/t/tty5420-w-nl -> ../a/att4415-w-nl
share/terminfo/t/tty5420-w-rv -> ../a/att4415-w-rv
share/terminfo/t/tty5420-w-rv-n -> ../a/att4415-w-rv-n
share/terminfo/t/tty5425 -> ../a/att5425
share/terminfo/t/tty5425-nl -> ../a/att5425-nl
share/terminfo/t/tty5425-w -> ../a/att5425-w
share/terminfo/t/tty5620 -> ../a/att5620
share/terminfo/t/tty5620-1 -> ../a/att5620-1
share/terminfo/t/tty5620-24 -> ../a/att5620-24
share/terminfo/t/tty5620-34 -> ../a/att5620-34
share/terminfo/t/tty5620-s -> ../a/att5620-s
share/terminfo/t/ttydmd -> ../a/att5620
share/terminfo/t/tvi912b-mc-2p -> tvi912b-2p-mc
share/terminfo/t/tvi912b-mc-vb -> tvi912b-vb-mc
share/terminfo/t/tvi912b-p-2p -> tvi912b-2p-p
share/terminfo/t/tvi912b-p-vb -> tvi912b-vb-p
share/terminfo/t/tvi912b-unk-2p -> tvi912b-2p-unk
share/terminfo/t/tvi912b-unk-vb -> tvi912b-vb-unk
share/terminfo/t/tvi912c -> tvi912b
share/terminfo/t/tvi912c-2p -> tvi912b-2p
share/terminfo/t/tvi912c-2p-mc -> tvi912b-2p-mc
share/terminfo/t/tvi912c-2p-p -> tvi912b-2p-p
share/terminfo/t/tvi912c-2p-unk -> tvi912b-2p-unk
share/terminfo/t/tvi912c-mc -> tvi912b-mc
share/terminfo/t/tvi912c-mc-2p -> tvi912b-2p-mc
share/terminfo/t/tvi912c-mc-vb -> tvi912b-vb-mc
share/terminfo/t/tvi912c-p -> tvi912b-p
share/terminfo/t/tvi912c-p-2p -> tvi912b-2p-p
share/terminfo/t/tvi912c-p-vb -> tvi912b-vb-p
share/terminfo/t/tvi912c-unk -> tvi912b-unk
share/terminfo/t/tvi912c-unk-2p -> tvi912b-2p-unk
share/terminfo/t/tvi912c-unk-vb -> tvi912b-vb-unk
share/terminfo/t/tvi912c-vb -> tvi912b-vb
share/terminfo/t/tvi912c-vb-mc -> tvi912b-vb-mc
share/terminfo/t/tvi912c-vb-p -> tvi912b-vb-p
share/terminfo/t/tvi912c-vb-unk -> tvi912b-vb-unk
share/terminfo/t/tvi914 -> tvi912
share/terminfo/t/tvi920 -> tvi912
share/terminfo/t/tvi920b-mc-2p -> tvi920b-2p-mc
share/terminfo/t/tvi920b-mc-vb -> tvi920b-vb-mc
share/terminfo/t/tvi920b-p-2p -> tvi920b-2p-p
share/terminfo/t/tvi920b-p-vb -> tvi920b-vb-p
share/terminfo/t/tvi920b-unk-2p -> tvi920b-2p-unk
share/terminfo/t/tvi920b-unk-vb -> tvi920b-vb-unk
share/terminfo/t/tvi920c -> tvi920b
share/terminfo/t/tvi920c-2p -> tvi920b-2p
share/terminfo/t/tvi920c-2p-mc -> tvi920b-2p-mc
share/terminfo/t/tvi920c-2p-p -> tvi920b-2p-p
share/terminfo/t/tvi920c-2p-unk -> tvi920b-2p-unk
share/terminfo/t/tvi920c-mc -> tvi920b-mc
share/terminfo/t/tvi920c-mc-2p -> tvi920b-2p-mc
share/terminfo/t/tvi920c-mc-vb -> tvi920b-vb-mc
share/terminfo/t/tvi920c-p -> tvi920b-p
share/terminfo/t/tvi920c-p-2p -> tvi920b-2p-p
share/terminfo/t/tvi920c-p-vb -> tvi920b-vb-p
share/terminfo/t/tvi920c-unk -> tvi920b-unk
share/terminfo/t/tvi920c-unk-2p -> tvi920b-2p-unk
share/terminfo/t/tvi920c-unk-vb -> tvi920b-vb-unk
share/terminfo/t/tvi920c-vb -> tvi920b-vb
share/terminfo/t/tvi920c-vb-mc -> tvi920b-vb-mc
share/terminfo/t/tvi920c-vb-p -> tvi920b-vb-p
share/terminfo/t/tvi920c-vb-unk -> tvi920b-vb-unk
share/terminfo/t/tw52-color -> tw52
share/terminfo/u/ultima2 -> ../i/ims-ansi
share/terminfo/u/ultimaII -> ../i/ims-ansi
share/terminfo/u/uniterm49 -> uniterm
share/terminfo/u/unixpc -> ../a/att7300
share/terminfo/v/v200-nam -> vt220-nam
share/terminfo/v/v320n -> vt320nam
share/terminfo/v/vapple -> ../a/apple-videx3
share/terminfo/v/vc103 -> vc303
share/terminfo/v/vc203 -> vc303
share/terminfo/v/vc403a -> vc303a
share/terminfo/v/vc414h -> vc414
share/terminfo/v/venix -> ../p/pc-venix
share/terminfo/v/viewpoint3a+ -> vp3a+
share/terminfo/v/viewpoint60 -> vp60
share/terminfo/v/viewpoint90 -> vp90
share/terminfo/v/vip7800-H -> vip-H
share/terminfo/v/vip7800-Hw -> vip-Hw
share/terminfo/v/vip7800-w -> vip-w
share/terminfo/v/visual603 -> vi603
share/terminfo/v/vitty -> ../a/att5620-s
share/terminfo/v/vk100 -> ../g/gigi
share/terminfo/v/vs100 -> ../x/xterm-24
share/terminfo/v/vs100-x10 -> ../x/x10term
share/terminfo/v/vt-61 -> vt61
share/terminfo/v/vt-utf8 -> ../m/ms-vt-utf8
share/terminfo/v/vt100+ -> ../m/ms-vt100+
share/terminfo/v/vt100-am -> vt100
share/terminfo/v/vt100-bm -> ../n/nwp512
share/terminfo/v/vt100-bm-o -> ../n/nwp512
share/terminfo/v/vt100-bot-s -> vt100-s-bot
share/terminfo/v/vt100-nam -> vt100nam
share/terminfo/v/vt100-nam-w -> vt100-w-nam
share/terminfo/v/vt100-s-top -> vt100-s
share/terminfo/v/vt100-top-s -> vt100-s
share/terminfo/v/vt100-w-am -> vt100-w
share/terminfo/v/vt100-w-nav -> vt100-nav-w
share/terminfo/v/vt200 -> vt220
share/terminfo/v/vt200-8 -> vt220-8bit
share/terminfo/v/vt200-8bit -> vt220-8bit
share/terminfo/v/vt200-old -> vt220-old
share/terminfo/v/vt200-w -> vt220-w
share/terminfo/v/vt220-8 -> vt220-8bit
share/terminfo/v/vt220-js -> vt200-js
share/terminfo/v/vt300 -> vt320
share/terminfo/v/vt300-nam -> vt320-nam
share/terminfo/v/vt300-w -> vt320-w
share/terminfo/v/vt300-w-nam -> vt320-w-nam
share/terminfo/v/vt330 -> vt340
share/terminfo/v/vt400-24 -> vt400
share/terminfo/v/vt61.5 -> vt61
share/terminfo/v/vtnt -> ../m/ms-vt100-color
share/terminfo/v/vv100 -> ../m/mvterm
share/terminfo/w/wren -> ../p/pt100
share/terminfo/w/wrenw -> ../p/pt100w
share/terminfo/w/wy-75ap -> wy75ap
share/terminfo/w/wy-99fgt -> wy99f
share/terminfo/w/wy-99fgta -> wy99fa
share/terminfo/w/wy120-wvb -> wy120-w-vb
share/terminfo/w/wy150 -> wy120
share/terminfo/w/wy150-25 -> wy120-25
share/terminfo/w/wy150-25-w -> wy120-25-w
share/terminfo/w/wy150-vb -> wy120-vb
share/terminfo/w/wy150-w -> wy120-w
share/terminfo/w/wy150-w-vb -> wy120-w-vb
share/terminfo/w/wy160-wvb -> wy160-w-vb
share/terminfo/w/wy325-42wvb -> wy325-42w-vb
share/terminfo/w/wy325-43wvb -> wy325-43w-vb
share/terminfo/w/wy325-80 -> wy325-25
share/terminfo/w/wy325-wvb -> wy325-w-vb
share/terminfo/w/wy325w-24 -> wy325-w
share/terminfo/w/wy370-101k -> wy370
share/terminfo/w/wy60-316X -> ../i/ibm3161
share/terminfo/w/wy60-AT -> ../i/ibm5151
share/terminfo/w/wy60-PC -> ../i/ibmpc
share/terminfo/w/wy60-wvb -> wy60-w-vb
share/terminfo/w/wy99fgt -> wy99f
share/terminfo/w/wy99fgta -> wy99fa
share/terminfo/w/wy99gt-wvb -> wy99gt-w-vb
share/terminfo/w/wyse-325 -> wy325-25
share/terminfo/w/wyse-75ap -> wy75ap
share/terminfo/w/wyse120 -> wy120
share/terminfo/w/wyse120-25 -> wy120-25
share/terminfo/w/wyse120-25-w -> wy120-25-w
share/terminfo/w/wyse120-vb -> wy120-vb
share/terminfo/w/wyse120-w -> wy120-w
share/terminfo/w/wyse120-wvb -> wy120-w-vb
share/terminfo/w/wyse150 -> wy120
share/terminfo/w/wyse150-25 -> wy120-25
share/terminfo/w/wyse150-25-w -> wy120-25-w
share/terminfo/w/wyse150-vb -> wy120-vb
share/terminfo/w/wyse150-w -> wy120-w
share/terminfo/w/wyse150-w-vb -> wy120-w-vb
share/terminfo/w/wyse160 -> wy160
share/terminfo/w/wyse160-25 -> wy160-25
share/terminfo/w/wyse160-25-w -> wy160-25-w
share/terminfo/w/wyse160-42 -> wy160-42
share/terminfo/w/wyse160-42-w -> wy160-42-w
share/terminfo/w/wyse160-43 -> wy160-43
share/terminfo/w/wyse160-43-w -> wy160-43-w
share/terminfo/w/wyse160-vb -> wy160-vb
share/terminfo/w/wyse160-w -> wy160-w
share/terminfo/w/wyse160-wvb -> wy160-w-vb
share/terminfo/w/wyse185 -> wy185
share/terminfo/w/wyse185-24 -> wy185-24
share/terminfo/w/wyse185-vb -> wy185-vb
share/terminfo/w/wyse185-w -> wy185-w
share/terminfo/w/wyse185-wvb -> wy185-wvb
share/terminfo/w/wyse30 -> wy30
share/terminfo/w/wyse30-mc -> wy30-mc
share/terminfo/w/wyse30-vb -> wy30-vb
share/terminfo/w/wyse325 -> wy325
share/terminfo/w/wyse325-25 -> wy325-25
share/terminfo/w/wyse325-25w -> wy325-25w
share/terminfo/w/wyse325-42 -> wy325-42
share/terminfo/w/wyse325-42w -> wy325-42w
share/terminfo/w/wyse325-43 -> wy325-43
share/terminfo/w/wyse325-43w -> wy325-43w
share/terminfo/w/wyse325-vb -> wy325-vb
share/terminfo/w/wyse325-w -> wy325-w
share/terminfo/w/wyse325-wvb -> wy325-w-vb
share/terminfo/w/wyse350 -> wy350
share/terminfo/w/wyse350-vb -> wy350-vb
share/terminfo/w/wyse350-w -> wy350-w
share/terminfo/w/wyse350-wvb -> wy350-wvb
share/terminfo/w/wyse370 -> wy370
share/terminfo/w/wyse50 -> wy50
share/terminfo/w/wyse50-mc -> wy50-mc
share/terminfo/w/wyse50-vb -> wy50-vb
share/terminfo/w/wyse50-w -> wy50-w
share/terminfo/w/wyse50-wvb -> wy50-wvb
share/terminfo/w/wyse520 -> wy520
share/terminfo/w/wyse520-24 -> wy520-24
share/terminfo/w/wyse520-36 -> wy520-36
share/terminfo/w/wyse520-36pc -> wy520-36pc
share/terminfo/w/wyse520-36w -> wy520-36w
share/terminfo/w/wyse520-36wpc -> wy520-36wpc
share/terminfo/w/wyse520-48 -> wy520-48
share/terminfo/w/wyse520-48pc -> wy520-48pc
share/terminfo/w/wyse520-48w -> wy520-48w
share/terminfo/w/wyse520-48wpc -> wy520-48wpc
share/terminfo/w/wyse520-epc -> wy520-epc
share/terminfo/w/wyse520-epc-w -> wy520-epc-w
share/terminfo/w/wyse520-p-wvb -> wy520-epc-wvb
share/terminfo/w/wyse520-pc-24 -> wy520-epc-24
share/terminfo/w/wyse520-pc-vb -> wy520-epc-vb
share/terminfo/w/wyse520-vb -> wy520-vb
share/terminfo/w/wyse520-w -> wy520-w
share/terminfo/w/wyse520-wvb -> wy520-wvb
share/terminfo/w/wyse60 -> wy60
share/terminfo/w/wyse60-25 -> wy60-25
share/terminfo/w/wyse60-25-w -> wy60-25-w
share/terminfo/w/wyse60-316X -> ../i/ibm3161
share/terminfo/w/wyse60-42 -> wy60-42
share/terminfo/w/wyse60-42-w -> wy60-42-w
share/terminfo/w/wyse60-43 -> wy60-43
share/terminfo/w/wyse60-43-w -> wy60-43-w
share/terminfo/w/wyse60-AT -> ../i/ibm5151
share/terminfo/w/wyse60-PC -> ../i/ibmpc
share/terminfo/w/wyse60-vb -> wy60-vb
share/terminfo/w/wyse60-w -> wy60-w
share/terminfo/w/wyse60-wvb -> wy60-w-vb
share/terminfo/w/wyse75 -> wy75
share/terminfo/w/wyse75-mc -> wy75-mc
share/terminfo/w/wyse75-vb -> wy75-vb
share/terminfo/w/wyse75-w -> wy75-w
share/terminfo/w/wyse75-wvb -> wy75-wvb
share/terminfo/w/wyse75ap -> wy75ap
share/terminfo/w/wyse85 -> wy85
share/terminfo/w/wyse85-8bit -> wy85-8bit
share/terminfo/w/wyse85-vb -> wy85-vb
share/terminfo/w/wyse85-w -> wy85-w
share/terminfo/w/wyse85-wvb -> wy85-wvb
share/terminfo/w/wyse99gt -> wy99gt
share/terminfo/w/wyse99gt-25 -> wy99gt-25
share/terminfo/w/wyse99gt-25-w -> wy99gt-25-w
share/terminfo/w/wyse99gt-vb -> wy99gt-vb
share/terminfo/w/wyse99gt-w -> wy99gt-w
share/terminfo/w/wyse99gt-wvb -> wy99gt-w-vb
share/terminfo/x/x1700 -> ../d/diablo1640
share/terminfo/x/x1700-lm -> ../d/diablo1740-lm
share/terminfo/x/x1720 -> xerox1720
share/terminfo/x/x1750 -> xerox1720
share/terminfo/x/x68k-ite -> x68k
share/terminfo/x/x820 -> xerox820
share/terminfo/x/xdku -> ../t/tws2103
share/terminfo/x/xenix -> ../i/ibmpcx
share/terminfo/x/xerox -> ../d/diablo1640
share/terminfo/x/xerox-lm -> ../d/diablo1640-lm
share/terminfo/x/xl83 -> ../c/cyb83
share/terminfo/x/xterm.js -> ../v/vscode
share/terminfo/x/xterms -> xterm-24
share/terminfo/x/xwsh -> ../i/iris-color
share/terminfo/z/z-100 -> z100
share/terminfo/z/z-100bw -> z100bw
share/terminfo/z/z110 -> z100
share/terminfo/z/z110bw -> z100bw
share/terminfo/z/z19 -> ../h/h19
share/terminfo/z/z29a-kc-bc -> z29a
share/terminfo/z/z29b -> z29
share/terminfo/z/z30 -> zen30
share/terminfo/z/z39a -> z39-a
share/terminfo/z/z50 -> zen50
share/terminfo/z/z8001 -> ../c/cci
share/terminfo/z/zen8001 -> ../c/cci
share/terminfo/z/zenith -> ../h/h19
share/terminfo/z/zenith29 -> z29
share/terminfo/z/zenith39-a -> z39-a
share/terminfo/z/zenith39-ansi -> z39-a
share/terminfo/z/zt-1 -> ztx
share/terminfo/z/ztx-1-a -> ztx
share/terminfo/z/ztx11 -> ztx
```
