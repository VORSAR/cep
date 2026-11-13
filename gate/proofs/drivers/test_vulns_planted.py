"""The vulns row's planted proofs (S1.1c-4 design note v1.1 §2.4-§2.5, §5 W0b; plan check §6 "The scan"; owner
2026-11-13, S1.1c-4's plan check, decisions 1 and 2): a known vulnerability in a module a package builds, the standard
library or the toolchain is red by its id; one in a module the register marks never built is green and named; the
copy's availability, age and integrity faults are red by name; the waivers silence exactly what §2.5 says.

The world is the real tree: every tracked file, copied, with two plants the row needs before the coordinator lands
them on integration (§1's law table): the registry row of §2.5, run from a world registry, and
tools/allowlists/vulns.json, empty, rule `vulns`. Each case builds its own copy of the database, a zip of synthetic
entries (ids GO-2099-NNNN, which no real entry holds) with the index files §2.4 requires, and its own stamp.json, in
a directory the world passes as VORSAR_CEP_VULNDB; nothing is fetched. The built modules and their versions are read
from the world by the pinned go itself (`go list` over both platforms, the `tool` pattern in gate/proofs), an oracle
independent of the row's loader, so no module list or version is hard-coded.

Dates: every date is counted from today in UTC. A green copy is fetched 2 days ago and its database modified 25 days
ago, 5 days inside each limit (7 and 30), so a run across midnight UTC changes nothing. The red ages sit one day past
each limit (8 and 31): time only moves forward, so a midnight crossing ages them further and they stay red. A
waiver expires 20 days ahead (allowlists.AHEAD_MAX is 90). The row runs at T4, its first tier (§2.5).

Until lane E1 lands, every red case is red on the W0 stub's `exit 1: vulns: not built yet (S1.1c-4 lane E1)`, which
names no planted id, and every green case is red on the same line.
"""

import hashlib
import io
import json
import os
import zipfile
from datetime import UTC, datetime, timedelta

from gate.proofs.drivers import support

ROW = "vulns"  # the row its red cases land on (teeth.red_rows, BK-S7)
JOB = "lint"
TIER = "T4"
# Design §2.5's registry row, planted until the coordinator registers it; then the real row is read instead.
PLANTED_ROW = {
    "id": "vulns",
    "job": "lint",
    "tiers": ["T4", "nightly"],
    "blocking": True,
    "timeout_s": 180,
    "needs": [],
    "scope": ["tools"],
    "expected_scope": [
        "git",
        "go.mod",
        "go.sum",
        "gate/proofs/go.mod",
        "gate/proofs/go.sum",
        "tools/deps-allowlist.json",
        "tools/allowlists/vulns.json",
    ],
    "min_wants": 6,
    "lands_in": "S1.1c-4",
    "exclusive": False,
    "command": ["python3.15", "-I", "-B", "-X", "utf8", "gate/runner/rows/vulns.py"],
}
COVERED = 6  # the six files the row reads (§2.5)
WAIVERS = "tools/allowlists/vulns.json"
REGISTER = "tools/deps-allowlist.json"
KNOWN = "gate/known-issues.json"
MARKER = "gate/current-sprint"
DECISION_REF = "docs/sprints/decisions.md#owner-decisions-2026-11-13--s11c-4s-plan-check"
CANONICAL_URL = "https://vuln.go.dev/vulndb.zip"
FRESH_DAYS = 2  # the copy's age in a green world: 5 days inside COPY_MAX_DAYS
OLD_DAYS = 8  # one day past COPY_MAX_DAYS (7)
MODIFIED_DAYS = 25  # the database's age in a green world: 5 days inside SOURCE_MAX_DAYS
STALE_DAYS = 31  # one day past SOURCE_MAX_DAYS (30)
EXPIRY_DAYS = 20
PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))
GO_ENV = {"GOPROXY": "off", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "CGO_ENABLED": "0"}
NOT_IN_TREE = "example.com/not-in-the-tree"  # a clean database's one entry affects only this module
X_EXP = "golang.org/x/exp"  # built through gate/proofs' apidiff tool line
X_MOD = "golang.org/x/mod"
X_TOOLS = "golang.org/x/tools"
X_NET = "golang.org/x/net"  # in both build lists, compiled by no package (plan check §0.15)


def stamp_time(moment: datetime) -> str:
    return moment.strftime("%Y-%m-%dT%H:%M:%SZ")


def next_patch(version: str) -> str:
    """A database version just above `version` (a build-list `v…` or a go tag): its core with the patch plus one.
    A pseudo-version v0.0.0-<time>-<hash> sorts below 0.0.0, so 0.0.1 is above it too."""
    core = version.removeprefix("go").removeprefix("v").split("-", 1)[0].split("+", 1)[0]
    parts = [int(p) for p in core.split(".")] + [0, 0]
    return f"{parts[0]}.{parts[1]}.{parts[2] + 1}"


def osv(id: str, *affected: tuple[str, list[dict]], modified: str) -> dict:
    """One OSV entry in the database's shape (§0.1): each affected block a module and its SEMVER events."""
    return {
        "schema_version": "1.3.1",
        "id": id,
        "modified": modified,
        "published": modified,
        "aliases": [],
        "summary": f"A planted advisory ({id}) for the vulns proof",
        "details": "Synthetic: no real advisory holds this id.",
        "affected": [
            {
                "package": {"name": module, "ecosystem": "Go"},
                "ranges": [{"type": "SEMVER", "events": events}],
                "ecosystem_specific": {"imports": [{"path": module}]},
            }
            for module, events in affected
        ],
        "references": [],
        "database_specific": {"url": f"https://pkg.go.dev/vuln/{id}", "review_status": "REVIEWED"},
    }


def fixed_at(fixed: str) -> list[dict]:
    return [{"introduced": "0"}, {"fixed": fixed}]


def database(entries: list[dict], *, modified: str, extra: dict[str, bytes] | None = None) -> bytes:
    """A zip in the database's layout (§2.4): index/db.json, index/modules.json and index/vulns.json agreeing with
    the ID/<id>.json members, plus any `extra` member."""
    modules: dict[str, list[dict]] = {}
    for e in entries:
        for block in e["affected"]:
            item = {"id": e["id"], "modified": e["modified"]}
            fixes = [ev["fixed"] for r in block["ranges"] for ev in r["events"] if "fixed" in ev]
            if fixes:
                item["fixed"] = fixes[-1]
            modules.setdefault(block["package"]["name"], []).append(item)
    members = {
        "index/db.json": {"modified": modified},
        "index/modules.json": [{"path": p, "vulns": v} for p, v in sorted(modules.items())],
        "index/vulns.json": [{"id": e["id"], "modified": e["modified"], "aliases": []} for e in entries],
        **{f"ID/{e['id']}.json": e for e in entries},
    }
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as z:
        for name, value in members.items():
            z.writestr(name, json.dumps(value, indent=1).encode("utf-8"))
        for name, data in (extra or {}).items():
            z.writestr(name, data)
    return buf.getvalue()


def zip_of(members: dict[str, bytes]) -> bytes:
    """A zip holding exactly these members (none: an empty zip)."""
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as z:
        for name, data in members.items():
            z.writestr(name, data)
    return buf.getvalue()


def go_binary() -> str:
    go = support.tool("go")
    return os.path.join(support.home(support.real_cache(), "go"), go.path_dir, go.command)


class VulnsPlanted(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        w = self.world
        for rel in w.tracked("."):
            w.copy(rel)
        w.write(
            WAIVERS,
            json.dumps(
                {"schema": 1, "note": "the vulns row's dated exemptions (planted)", "rule": "vulns", "entries": []},
                indent=2,
            )
            + "\n",
        )
        self.now = datetime.now(UTC)
        self.today = self.now.date()
        self.copy_root = os.path.join(w.root, "vulndb")
        os.mkdir(self.copy_root)
        w.env["VORSAR_CEP_VULNDB"] = self.copy_root
        self.marker = support.read_text(os.path.join(w.repo, MARKER)).strip()
        known = json.loads(support.read_text(os.path.join(w.repo, KNOWN)))
        self.issue = next(e["id"] for e in known["entries"] if not e["forward"])
        self.expiry = (self.today + timedelta(days=EXPIRY_DAYS)).isoformat()
        self.go_version = self.go(".", ["env", "GOVERSION"]).strip()
        self.assertTrue(self.marker and self.issue and self.go_version.startswith("go1."))

    # ---------------------------------------------------------------------------------------------------- the world

    def go(self, cwd: str, args: list[str], **extra: str) -> str:
        out, err = os.path.join(self.world.root, "go.stdout"), os.path.join(self.world.root, "go.stderr")
        env = {**self.world.env, **GO_ENV, **extra}
        argv = [go_binary(), *args]
        code = support.run_process(
            argv, cwd=os.path.join(self.world.repo, cwd), env=env, out=out, err=err, cap_s=support.RUN_CAP_S
        )
        if code != 0:
            raise AssertionError(f"go {' '.join(args)} in {cwd}: exit {code}: {support.read_text(err)}")
        return support.read_text(out)

    def versions(self) -> dict[str, str]:
        """path -> version over both build lists, the root's first (§2.1), the main modules aside."""
        found: dict[str, str] = {}
        for mod in ("gate/proofs", "."):
            for line in self.go(
                mod, ["list", "-m", "-f", "{{if not .Main}}{{.Path}} {{.Version}}{{end}}", "all"]
            ).split("\n"):
                if line.strip():
                    path, version = line.split()
                    found[path] = version
        return found

    def built(self) -> dict[str, str]:
        """Every module a package, a test or a tool line compiles on either platform, by the pinned go itself."""
        template = "{{if .Module}}{{if not .Module.Main}}{{.Module.Path}}{{end}}{{end}}"
        paths: set[str] = set()
        for goos, goarch in PLATFORMS:
            for mod, patterns in ((".", ["./..."]), ("gate/proofs", ["./...", "tool"])):
                listing = self.go(
                    mod, ["list", "-e", "-deps", "-test", "-f", template, *patterns], GOOS=goos, GOARCH=goarch
                )
                paths |= set(listing.split())
        versions = self.versions()
        self.assertTrue({X_EXP, X_MOD, X_TOOLS} <= paths and X_NET not in paths and X_NET in versions, paths)
        return {path: versions[path] for path in sorted(paths)}

    def modified(self, days: int = MODIFIED_DAYS) -> str:
        return stamp_time(self.now - timedelta(days=days))

    def plant_copy(
        self,
        zip_bytes: bytes | None,
        *,
        stamp: bool = True,
        fetched_days: int = FRESH_DAYS,
        sha256: str | None = None,
    ) -> None:
        """vulndb.zip and stamp.json in the world's copy root, as make vulndb leaves them."""
        if zip_bytes is not None:
            with open(os.path.join(self.copy_root, "vulndb.zip"), "wb") as f:
                f.write(zip_bytes)
        if stamp:
            digest = sha256 or hashlib.sha256(zip_bytes or b"").hexdigest()
            fetched = stamp_time(self.now - timedelta(days=fetched_days))
            with open(os.path.join(self.copy_root, "stamp.json"), "w", encoding="utf-8") as f:
                json.dump({"fetched": fetched, "sha256": digest, "url": CANONICAL_URL}, f)

    def clean_db(self, *, modified_days: int = MODIFIED_DAYS, extra: dict[str, bytes] | None = None) -> bytes:
        """A database with one entry, in no module of the tree."""
        entry = osv("GO-2099-0100", (NOT_IN_TREE, fixed_at("1.0.1")), modified=self.modified(modified_days))
        return database([entry], modified=self.modified(modified_days), extra=extra)

    def waive(self, target: str, *, kind: str = "TEMP") -> None:
        entry = {
            "target": target,
            "kind": kind,
            "reason": "a planted exemption of the vulns proof",
            "sprint": self.marker,
        }
        if kind == "TEMP":
            entry.update(expiry=self.expiry, issue=self.issue)
        else:
            entry["ref"] = DECISION_REF
        path = os.path.join(self.world.repo, WAIVERS)
        data = json.loads(support.read_text(path))
        data["entries"].append(entry)
        self.world.write(WAIVERS, json.dumps(data, indent=2) + "\n")

    def mark_never_built(self, module: str) -> None:
        """The register's `"built": false` on one entry (§2.6; decision 1)."""
        path = os.path.join(self.world.repo, REGISTER)
        data = json.loads(support.read_text(path))
        found = [e for e in data["modules"] if e["module"] == module]
        self.assertEqual(len(found), 1, module)
        found[0]["built"] = False
        self.world.write(REGISTER, json.dumps(data, indent=2, ensure_ascii=False) + "\n")

    def module_cache_without(self, module: str, version: str) -> str:
        """A module cache in this world mirroring the real one by links, with `module`'s extracted tree and its zip
        (and .ziphash) absent: go then cannot load its packages offline (GOPROXY=off)."""
        real = os.path.realpath(self.go(".", ["env", "GOMODCACHE"]).strip())
        dest = os.path.join(self.world.root, "modcache")
        download = f"cache/download/{module}/@v/{version}"
        omit = {f"{module}@{version}", f"{download}.zip", f"{download}.ziphash"}
        for rel in omit:
            self.assertTrue(os.path.lexists(os.path.join(real, rel)), f"the real module cache lacks {rel}")
        needed = {"/".join(p.split("/")[:i]) for p in omit for i in range(1, p.count("/") + 1)}

        def fill(rel_dir: str) -> None:
            os.mkdir(os.path.join(dest, rel_dir))
            for name in sorted(os.listdir(os.path.join(real, rel_dir))):
                rel = f"{rel_dir}/{name}" if rel_dir else name
                if rel in omit:
                    continue
                if rel in needed:
                    fill(rel)
                else:
                    os.symlink(os.path.join(real, rel), os.path.join(dest, rel))

        fill("")
        return dest

    def run_row(self) -> support.Run:
        self.world.commit()
        try:
            row = support.registry_row(ROW)
        except AssertionError:
            row = PLANTED_ROW  # not yet registered (§1: the coordinator's W0 commit)
        return self.world.run(registry=self.world.registry(row), tier=TIER)

    def failed_lines(self, run: support.Run) -> list[str]:
        out = run.row_stdout(JOB, ROW)
        return [line.removeprefix("failed: ") for line in out.split("\n") if line.startswith("failed: ")]

    # ----------------------------------------------------------------------------------------------- the verdicts

    def assertRedLines(self, run: support.Run, expected: list[str], *, exactly: bool = False) -> list[str]:
        """Red; the row FAIL; each expected line among its `failed:` lines (all of them, with `exactly`); parity
        passed in the same run."""
        self.assertGateFailed(run)
        record = run.record(JOB, ROW)
        self.assertEqual(record["verdict"], "fail", run.stdout)
        failed = self.failed_lines(run)
        if exactly:
            self.assertEqual(sorted(failed), sorted(expected), record["reason"])
        for line in expected:
            self.assertIn(line, failed, record["reason"])
        self.assertParityPassed(run, JOB)
        return failed

    def assertRedMatching(self, run: support.Run, prefix: str, suffix: str) -> None:
        """Red with one `failed:` line from `prefix` to `suffix` (a count in between that the run's clock sets)."""
        self.assertGateFailed(run)
        self.assertEqual(run.record(JOB, ROW)["verdict"], "fail", run.stdout)
        failed = self.failed_lines(run)
        self.assertTrue(any(f.startswith(prefix) and f.endswith(suffix) for f in failed), (prefix, suffix, failed))
        self.assertParityPassed(run, JOB)

    def assertScannedGreen(self, run: support.Run, *needles: str) -> None:
        """Green at covered 6, the gate passed, and each needle in the row's own output (its reason, §2.5)."""
        self.assertRow(run, JOB, "vulns", True, f"covered {COVERED}")
        self.assertParityPassed(run, JOB)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)
        out = run.row_stdout(JOB, ROW)
        for needle in needles:
            self.assertIn(needle, out)

    # ------------------------------------------------------------------------------------------- the findings

    def test_a_planted_entry_per_built_module_and_stdlib_and_toolchain_is_red_by_id(self):
        # (1): each built module, stdlib and toolchain at the pinned go, each its own id, each red by name.
        targets = {**self.built(), "stdlib": self.go_version, "toolchain": self.go_version}
        entries, expected = [], []
        for n, (module, version) in enumerate(sorted(targets.items()), start=1):
            id, fixed = f"GO-2099-{n:04d}", next_patch(version)
            entries.append(osv(id, (module, fixed_at(fixed)), modified=self.modified()))
            expected.append(f"vulns: {id} {module} {version}: fixed in {fixed}")
        self.plant_copy(database(entries, modified=self.modified()))
        self.assertRedLines(self.run_row(), expected, exactly=True)

    def test_a_finding_in_a_module_marked_never_built_is_green_and_named(self):
        # (1b), decision 1: x/net is in both build lists and compiled by nothing; the register marks it.
        version = self.versions()[X_NET]
        fixed = next_patch(version)
        entry = osv("GO-2099-0201", (X_NET, fixed_at(fixed)), modified=self.modified())
        self.plant_copy(database([entry], modified=self.modified()))
        self.mark_never_built(X_NET)
        self.assertScannedGreen(self.run_row(), f"never built: GO-2099-0201 {X_NET} {version} fixed {fixed}")

    def test_a_built_module_missing_from_the_module_cache_is_red(self):
        # (1c), review A-B1: x/exp (the apidiff tool's module) absent from the world's module cache; the loader names
        # the package it could not load, never dropping the module from the compiled set.
        version = self.versions()[X_EXP]
        self.world.env["GOMODCACHE"] = self.module_cache_without(X_EXP, version)
        self.plant_copy(self.clean_db())
        # Measured (pinned go1.27.4, this cache, GOPROXY=off): `go list -e -json -deps tool` exits 0 and gives the tool
        # package with no Module and Error.Err "module lookup disabled by GOPROXY=off" (darwin/arm64; a lookup is no
        # platform's, so linux/amd64 gives the same text and the line carries no platform).
        line = (
            f"gomods: {X_EXP}/cmd/apidiff (gate/proofs): module lookup disabled by GOPROXY=off: "
            "run go mod download in gate/proofs, then re-run"
        )
        self.assertRedLines(self.run_row(), [line])

    def test_x_exp_introduced_0_with_no_fix_is_red(self):
        # (1f): every version of x/exp affected, no fix yet.
        version = self.versions()[X_EXP]
        entry = osv("GO-2099-0301", (X_EXP, [{"introduced": "0"}]), modified=self.modified())
        self.plant_copy(database([entry], modified=self.modified()))
        self.assertRedLines(self.run_row(), [f"vulns: GO-2099-0301 {X_EXP} {version}: no fix yet"], exactly=True)

    # ---------------------------------------------------------------------------------------------- the copy

    def test_no_zip_is_red(self):
        self.plant_copy(None, stamp=False)
        self.assertRedLines(self.run_row(), [f"vulns: no vulndb.zip at {self.copy_root}: run make vulndb"])

    def test_a_copy_that_is_not_a_zip_is_red(self):
        self.plant_copy(b"<html>a captive portal</html>\n")
        self.assertRedLines(self.run_row(), [f"vulns: {self.copy_root}/vulndb.zip is not a zip: run make vulndb"])

    def test_no_stamp_is_red(self):
        self.plant_copy(self.clean_db(), stamp=False)
        self.assertRedLines(self.run_row(), [f"vulns: no stamp.json at {self.copy_root}: run make vulndb"])

    def test_a_copy_fetched_8_days_ago_is_red(self):
        day = (self.now - timedelta(days=OLD_DAYS)).date().isoformat()
        self.plant_copy(self.clean_db(), fetched_days=OLD_DAYS)
        self.assertRedMatching(
            self.run_row(), "vulns: the copy was fetched ", f" days ago ({day}), more than 7: run make vulndb"
        )

    def test_a_database_unchanged_for_31_days_is_red(self):
        day = (self.now - timedelta(days=STALE_DAYS)).date().isoformat()
        self.plant_copy(self.clean_db(modified_days=STALE_DAYS))
        self.assertRedMatching(
            self.run_row(),
            f"vulns: the database has not changed since {day} (",
            " days, more than 30): check vuln.go.dev, or add a dated vulndb exemption naming a known issue",
        )

    def test_a_sha256_mismatch_is_red(self):
        self.plant_copy(self.clean_db(), sha256="0" * 64)
        self.assertRedLines(self.run_row(), ["vulns: the zip's sha256 is not the stamp's: run make vulndb"])

    def test_a_member_outside_the_layout_is_red(self):
        self.plant_copy(self.clean_db(extra={"ID/README.txt": b"not an entry\n"}))
        self.assertRedLines(self.run_row(), ["vulns: member ID/README.txt is not in the database's layout"])

    def plant_fetch_error(self, days: int, error: str = "https://vuln.go.dev/vulndb.zip answered 503") -> str:
        at = stamp_time(self.now - timedelta(days=days))
        with open(os.path.join(self.copy_root, "fetch-error.txt"), "w", encoding="utf-8") as f:
            json.dump({"at": at, "error": error}, f)
        return at

    def test_a_failed_fetch_newer_than_the_stamp_with_no_zip_is_red_with_its_text(self):
        # §2.5 (v2; reviews DA-S11, DQ-S11): "newer" is the times inside the files; with no usable zip it is red.
        self.plant_copy(None)  # the stamp of an earlier fetch (2 days old), its zip gone
        at = self.plant_fetch_error(1)
        self.assertRedLines(
            self.run_row(), [f"vulns: the last make vulndb failed at {at}: https://vuln.go.dev/vulndb.zip answered 503"]
        )

    def test_a_failed_refetch_with_a_usable_copy_is_green_and_named(self):
        # §2.5 (v2; review DQ-S11): decision 2 makes the copy's age the red; a failed re-fetch beside a 2-day-old copy
        # is named in the reason.
        self.plant_copy(self.clean_db())
        at = self.plant_fetch_error(1)
        self.assertScannedGreen(
            self.run_row(), f"last fetch failed at {at}: https://vuln.go.dev/vulndb.zip answered 503"
        )

    def test_a_fetch_error_older_than_the_stamp_is_green_and_not_named(self):
        self.plant_copy(self.clean_db())
        self.plant_fetch_error(FRESH_DAYS + 1)
        run = self.run_row()
        self.assertScannedGreen(run)
        self.assertNotIn("last fetch failed", run.row_stdout(JOB, ROW))

    def test_an_empty_zip_is_red_index_missing(self):
        # Card line 1's empty copy (review DQ-S6).
        self.plant_copy(zip_of({}))
        self.assertRedLines(self.run_row(), ["vulns: index/db.json is missing or malformed"])

    def test_a_zip_holding_only_an_entry_is_red_index_missing(self):
        # Card line 1's index-less copy (review DQ-S6).
        entry = osv("GO-2099-0601", (NOT_IN_TREE, fixed_at("1.0.1")), modified=self.modified())
        self.plant_copy(zip_of({"ID/GO-2099-0601.json": json.dumps(entry).encode()}))
        self.assertRedLines(self.run_row(), ["vulns: index/db.json is missing or malformed"])

    def test_a_zero_byte_copy_is_red_not_a_zip(self):
        self.plant_copy(b"")
        self.assertRedLines(self.run_row(), [f"vulns: {self.copy_root}/vulndb.zip is not a zip: run make vulndb"])

    # ------------------------------------------------------------------------------------------- the waivers

    def test_an_id_module_waiver_silences_that_pair_only(self):
        # One id in two modules; the waiver names one pair: the other stays red, and nothing else fails.
        versions = self.versions()
        entry = osv(
            "GO-2099-0401",
            (X_MOD, fixed_at(next_patch(versions[X_MOD]))),
            (X_TOOLS, fixed_at(next_patch(versions[X_TOOLS]))),
            modified=self.modified(),
        )
        self.plant_copy(database([entry], modified=self.modified()))
        self.waive(f"GO-2099-0401 {X_MOD}")
        expected = f"vulns: GO-2099-0401 {X_TOOLS} {versions[X_TOOLS]}: fixed in {next_patch(versions[X_TOOLS])}"
        self.assertRedLines(self.run_row(), [expected], exactly=True)

    def test_an_id_module_waiver_silences_that_pairs_findings_fault(self):
        # §2.5 (review DA-S1): an unknown range event in one entry for one module is that pair's fault; the waiver
        # silences it and is named; the same fault in another entry stays red.
        events = [{"introduced": "0"}, {"last_affected": "9.9.9"}]
        entries = [
            osv("GO-2099-0501", (X_MOD, events), modified=self.modified()),
            osv("GO-2099-0502", (X_TOOLS, events), modified=self.modified()),
        ]
        self.plant_copy(database(entries, modified=self.modified()))
        self.waive(f"GO-2099-0501 {X_MOD}")
        self.assertRedLines(self.run_row(), ["vulns: GO-2099-0502: unknown range event last_affected"], exactly=True)

    def test_an_id_module_waiver_silencing_a_fault_is_green_and_named(self):
        events = [{"introduced": "0"}, {"last_affected": "9.9.9"}]
        entry = osv("GO-2099-0503", (X_MOD, events), modified=self.modified())
        self.plant_copy(database([entry], modified=self.modified()))
        self.waive(f"GO-2099-0503 {X_MOD}")
        self.assertScannedGreen(self.run_row(), f"silenced: GO-2099-0503 {X_MOD} until {self.expiry} ({self.issue})")

    def test_a_malformed_waiver_target_is_red(self):
        self.plant_copy(self.clean_db())
        self.waive("GO-2099-0600")  # an id with no module
        self.assertRedLines(self.run_row(), ['vulns: GO-2099-0600: not "<id> <module>" or "vulndb"'], exactly=True)

    def test_an_expired_waiver_silences_nothing_and_is_not_stale(self):
        # §2.5 (review DQ-S4c): the row treats an expired entry as absent; the suppressions row reds it.
        version = self.versions()[X_MOD]
        entry = osv("GO-2099-0604", (X_MOD, fixed_at(next_patch(version))), modified=self.modified())
        self.plant_copy(database([entry], modified=self.modified()))
        self.expiry = (self.today - timedelta(days=1)).isoformat()
        self.waive(f"GO-2099-0604 {X_MOD}")
        self.assertRedLines(
            self.run_row(), [f"vulns: GO-2099-0604 {X_MOD} {version}: fixed in {next_patch(version)}"], exactly=True
        )

    def test_waivers_are_not_judged_stale_when_the_index_is_malformed(self):
        # §2.5 (review DQ-S4a): staleness only on a completed scan.
        members = {
            "index/db.json": json.dumps({"modified": self.modified()}).encode(),
            "index/modules.json": json.dumps([{"path": NOT_IN_TREE, "vulns": []}]).encode(),
            "index/vulns.json": b"{}",
        }
        self.plant_copy(zip_of(members))
        self.waive(f"GO-2099-0605 {X_MOD}")
        failed = self.assertRedLines(self.run_row(), ["vulns: index/vulns.json is missing, malformed or empty"])
        self.assertFalse([line for line in failed if "matches nothing" in line], failed)

    def test_a_directory_replacement_is_not_matched_and_is_named(self):
        # §2.2 (review DA-S2): a replace to a local directory is not matched, and the reason names it. x/net, in both
        # build lists and compiled by nothing, is replaced in the root by a go.mod-only directory; an entry naming its
        # build-list version would be red without the replace.
        version = self.versions()[X_NET]
        self.world.write("third_party/xnet/go.mod", f"module {X_NET}\n\ngo 1.27\n")
        gomod = support.read_text(os.path.join(self.world.repo, "go.mod"))
        self.world.write("go.mod", gomod + f"\nreplace {X_NET} => ./third_party/xnet\n")
        entry = osv("GO-2099-0606", (X_NET, fixed_at(next_patch(version))), modified=self.modified())
        self.plant_copy(database([entry], modified=self.modified()))
        self.assertScannedGreen(self.run_row(), f"replaced by a directory: {X_NET} → ./third_party/xnet")

    def test_a_waiver_matching_nothing_is_red_as_stale(self):
        self.plant_copy(self.clean_db())
        self.waive(f"GO-2099-0402 {X_MOD}")
        self.assertRedLines(self.run_row(), [f"vulns: GO-2099-0402 {X_MOD} matches nothing: remove it"], exactly=True)

    def test_a_permanent_waiver_is_red_and_silences_nothing(self):
        # Review A-L3: a vulnerability exemption is TEMP; a refused entry silences nothing, so its finding stays red.
        version = self.versions()[X_MOD]
        entry = osv("GO-2099-0403", (X_MOD, fixed_at(next_patch(version))), modified=self.modified())
        self.plant_copy(database([entry], modified=self.modified()))
        self.waive(f"GO-2099-0403 {X_MOD}", kind="PERMANENT")
        self.assertRedLines(
            self.run_row(),
            [
                f"vulns: GO-2099-0403 {X_MOD}: a vulnerability exemption is TEMP, with an expiry and a known issue",
                f"vulns: GO-2099-0403 {X_MOD} {version}: fixed in {next_patch(version)}",
            ],
        )

    def test_the_vulndb_waiver_silences_an_old_copy_but_not_its_findings(self):
        # Decision 2, review DA-B3: a readable copy is always scanned, whatever its age.
        version = self.versions()[X_MOD]
        entry = osv("GO-2099-0404", (X_MOD, fixed_at(next_patch(version))), modified=self.modified(MODIFIED_DAYS))
        self.plant_copy(database([entry], modified=self.modified(MODIFIED_DAYS)), fetched_days=OLD_DAYS)
        self.waive("vulndb")
        expected = f"vulns: GO-2099-0404 {X_MOD} {version}: fixed in {next_patch(version)}"
        self.assertRedLines(self.run_row(), [expected], exactly=True)

    def test_the_vulndb_waiver_with_no_zip_is_green_not_scanned(self):
        self.plant_copy(None, stamp=False)
        self.waive("vulndb")
        self.assertScannedGreen(
            self.run_row(),
            "not scanned: ",
            f"vulns: no vulndb.zip at {self.copy_root}: run make vulndb",
            f"exempted by vulndb until {self.expiry} ({self.issue})",
        )

    def test_the_vulndb_waiver_never_silences_a_sha256_mismatch(self):
        self.plant_copy(self.clean_db(), sha256="0" * 64)
        self.waive("vulndb")
        self.assertRedLines(self.run_row(), ["vulns: the zip's sha256 is not the stamp's: run make vulndb"])

    def test_a_vulndb_waiver_that_silenced_nothing_is_red_as_stale(self):
        self.plant_copy(self.clean_db())
        self.waive("vulndb")
        self.assertRedLines(self.run_row(), ["vulns: vulndb matches nothing: remove it"], exactly=True)

    # --------------------------------------------------------------------------------------------- the real tree

    def test_the_real_tree_with_a_clean_database_is_green(self):
        # (23): the real tree's modules, stdlib and toolchain against a clean database: green, covered 6.
        self.plant_copy(self.clean_db())
        modified_day = (self.now - timedelta(days=MODIFIED_DAYS)).date().isoformat()
        fetched_day = (self.now - timedelta(days=FRESH_DAYS)).date().isoformat()
        self.assertScannedGreen(
            self.run_row(),
            f"vulndb {modified_day} (fetched {fetched_day}, {self.copy_root}), 1 entries; ",
            f"stdlib and toolchain at {self.go_version}",
            "not read: golangci-lint (Go build info, from S1.1c-5); ruff, CPython (outside the Go database)",
        )
