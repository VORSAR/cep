"""The deps row's proofs (S1.1c-1, plan check v2 section 6 "deps"; ADR-0009 section 1, ADR-0001 section 1): a
module in the graph that the allowlist does not list, a listed module whose licence file is unknown, missing or
disagrees with its `license` field, an allowlist entry in no graph, a third tracked go.mod, and (S1.1c-4) a wrong
licence on the module of a second `tool` line that the apidiff tool never reaches are each red, named.

One world per case. The world holds the gate, copies of go.mod, go.sum, gate/proofs/go.mod and go.sum and of
tools/deps-allowlist.json (edited per case), a package in each module, and a planted module `example.com/planted`
wired in by `replace example.com/planted => ../planted` (a directory beside the repository, so it is never a third
tracked go.mod) with `require example.com/planted v1.0.0`: a replace to a directory resolves offline with no go.sum
entry (proven by a scratch run under the pinned go, GOPROXY=off, GOFLAGS=-mod=readonly, before this driver).

The world's go.mod copies lose the `tool golang.org/x/exp/cmd/apidiff` directive (whichever module holds it:
go.mod today, gate/proofs/go.mod after decision 7 of 2026-11-11) and keep every require, so the world's module graph
is the repository's own and the real allowlist copy fits it; the design's stripping of the tool's four requires too
re-selects versions (x/text's go.mod pulls x/tools v0.51.0 with a graph of its own) that no copy of the allowlist
can list. No package of the world imports anything but the plant; the tool's four modules count as imported through
`go list -deps` of the tool, as on the real tree, and read their licence files from the module cache. Each case
asserts the row's `failed:` lines are exactly its one planted line (design note section 5.4). Each entry names its
sprint and a ref into docs/sprints/decisions.md (S1.1c-2 design section 4.6a), so the world copies that document;
the planted entries borrow the first real entry's sprint and ref.

The tidy line (S1.1c-4 design §3.1, review DQ-S2) is red in every world: the world's root has no package importing
participle, goleak or x/text, and its gate/proofs loses the tool line, so `go mod tidy` would drop requires in both.
The cases above filter it out by its prefix; the tidy case plants an unused require at the top of go.mod, the first
change in the diff's order, and asserts the root's tidy line names it. The T4 case edits go.sum after the world's
commit and is red by the `go.sum` line (design §3.2; owner, 2026-11-13, the design's decision 3).
"""

import json
import os

from gate.proofs.drivers import support

ROW = "deps"  # the row its red cases land on: teeth.red_rows ties this proof to it (BK-S7)
ALLOWLIST = "tools/deps-allowlist.json"
DECISIONS = "docs/sprints/decisions.md"
PLANTED = "example.com/planted"
VERSION = "v1.0.0"
MIT = (
    "MIT License\n\nCopyright (c) 2026 the proofs\n\nPermission is hereby granted, free of charge, to any person "
    'obtaining a copy of this software and associated documentation files (the "Software"), to deal in the '
    "Software without restriction.\n"
)
BSD3 = (
    "Copyright (c) 2026 the proofs. All rights reserved.\n\nRedistribution and use in source and binary forms, "
    "with or without modification, are permitted provided that the following conditions are met:\n\n"
    "1. Redistributions of source code must retain the above copyright notice.\n"
    "2. Redistributions in binary form must reproduce the above copyright notice.\n"
    "3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or "
    "promote products derived from this software without specific prior written permission.\n"
)
GPL3 = (
    "                    GNU GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007\n\n"
    "Everyone is permitted to copy and distribute verbatim copies of this license document.\n"
)
ENTRY = {
    "module": PLANTED,
    "version": VERSION,
    "scope": "runtime (the proof's plant)",
    "license": "MIT",
    "approved": "2026-11-11",
    "why": "the deps proof's planted module",
    "replace": "../planted",  # the replace is approved in the world (owner, 2026-11-11, the fix round)
}
PLANTED_GO = (
    "// Package planted is the proof's planted dependency.\npackage planted\n\n"
    "// Answer is its one export.\nfunc Answer() int { return 42 }\n"
)
GONE = {**ENTRY, "module": "example.com/gone", "why": "the deps proof's stale entry"}
TOOL = "golang.org/x/exp/cmd/apidiff"
TIDY = "deps: go mod tidy "  # the tidy line's prefix (design §3.1)
TIDY_PROOFS = "deps: go mod tidy would change gate/proofs: "
# S1.1c-4 design §2.1, §2.7 (plan check §6 "The register" (2)): a second `tool` line in gate/proofs/go.mod, its module
# a directory beside the repository that the apidiff tool never reaches; the licence fingerprint covers every module
# the tool lines build, so its register entry's wrong licence field is red.
SECOND_TOOL = "example.com/plantedtool"
SECOND_TOOL_ENTRY = {
    **ENTRY,
    "module": SECOND_TOOL,
    "why": "the deps proof's second tool line",
    "replace": "../../../plantedtool",  # from gate/proofs, the directory beside the repository
}


def without_tool(gomod: str) -> str:
    """A go.mod text without the apidiff tool directive; its requires stay."""
    return "".join(line for line in gomod.splitlines(keepends=True) if line.split()[:2] != ["tool", TOOL])


class DepsPlanted(support.DriverTest):
    def deps_world(self, *, license_text: str | None, entries: list[dict]) -> None:
        w = self.world
        w.copy_gate()
        for rel in ("go.mod", "go.sum", "gate/proofs/go.mod", "gate/proofs/go.sum", ALLOWLIST, DECISIONS):
            w.copy(rel)
        for rel in ("go.mod", "gate/proofs/go.mod"):
            w.write(rel, without_tool(support.read_text(os.path.join(w.repo, rel))))
        planted = self.planted = os.path.join(w.root, "planted")
        os.makedirs(planted)
        with open(os.path.join(planted, "go.mod"), "w", encoding="utf-8") as f:
            f.write(f"module {PLANTED}\n\ngo 1.27.0\n")
        with open(os.path.join(planted, "planted.go"), "w", encoding="utf-8") as f:
            f.write(PLANTED_GO)
        if license_text is not None:
            with open(os.path.join(planted, "LICENSE"), "w", encoding="utf-8") as f:
                f.write(license_text)
        w.write(
            "internal/planted/planted.go",
            f'// Package planted imports the planted module.\npackage planted\n\nimport "{PLANTED}"\n\n'
            "// Use reads it.\nfunc Use() int { return planted.Answer() }\n",
        )
        w.write(
            "gate/proofs/internal/planted/planted.go",
            "// Package planted keeps the proofs module non-empty.\npackage planted\n",
        )
        gomod = support.read_text(os.path.join(w.repo, "go.mod"))
        w.write("go.mod", f"{gomod}\nrequire {PLANTED} {VERSION}\n\nreplace {PLANTED} => ../planted\n")
        path = os.path.join(w.repo, ALLOWLIST)
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        first = data["modules"][0]
        data["modules"].extend({**e, "sprint": first["sprint"], "ref": first["ref"]} for e in entries)
        w.write(ALLOWLIST, json.dumps(data, indent=2, ensure_ascii=False) + "\n")

    def run_row(self, *, tier: str = "T2", commit: bool = True) -> support.Run:
        if commit:
            self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row("deps")), tier=tier)

    def assertRedLine(self, run: support.Run, expected: str, *, drop: str = TIDY) -> None:
        """Red, the row FAIL, and its `failed:` lines, the world's tidy lines (`drop`) aside, exactly the one planted
        line; the row's reason is its first line."""
        self.assertGateFailed(run)
        record = run.record("lint", "deps")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        out = run.row_stdout("lint", "deps")
        failed = [line.removeprefix("failed: ") for line in out.split("\n") if line.startswith("failed: ")]
        self.assertTrue(any(line.startswith(drop) for line in failed), f"the world's tidy line: {failed}")
        self.assertEqual([line for line in failed if not line.startswith(drop)], [expected], record["reason"])
        self.assertParityPassed(run, "lint")

    def test_an_unlisted_module_is_red(self):
        self.deps_world(license_text=MIT, entries=[])
        self.assertRedLine(self.run_row(), f"deps: {PLANTED} {VERSION} is not in {ALLOWLIST}")

    def test_an_unknown_licence_text_is_red(self):
        self.deps_world(license_text=GPL3, entries=[ENTRY])
        self.assertRedLine(self.run_row(), f"deps: {PLANTED}: unknown licence text in {self.planted}")

    def test_a_missing_licence_file_is_red(self):
        self.deps_world(license_text=None, entries=[ENTRY])
        self.assertRedLine(self.run_row(), f"deps: {PLANTED}: no LICENSE file in {self.planted}")

    def test_a_field_disagreeing_with_the_file_is_red(self):
        self.deps_world(license_text=BSD3, entries=[ENTRY])
        self.assertRedLine(self.run_row(), f"deps: {PLANTED}: license field MIT, file reads BSD-3-Clause")

    def test_a_stale_entry_is_red(self):
        self.deps_world(license_text=MIT, entries=[ENTRY, GONE])
        self.assertRedLine(
            self.run_row(),
            "deps: stale entry example.com/gone: not in the module graph of go.mod or gate/proofs/go.mod",
        )

    def test_a_second_tool_lines_module_has_its_licence_read(self):
        # The world's gate/proofs/go.mod gets the apidiff line back and a second tool line; the second tool's module
        # holds a BSD-3-Clause LICENSE while its entry says MIT. Red on today's rows/deps.py, which reads one TOOL
        # constant (the apidiff package), so the second tool's module stays graph-only and its licence unread.
        self.deps_world(license_text=MIT, entries=[ENTRY, SECOND_TOOL_ENTRY])
        w = self.world
        tool_dir = os.path.join(w.root, "plantedtool")
        os.makedirs(os.path.join(tool_dir, "cmd", "t"))
        for rel, text in (
            ("go.mod", f"module {SECOND_TOOL}\n\ngo 1.27.0\n"),
            ("LICENSE", BSD3),
            ("cmd/t/main.go", "// Command t is the proof's second tool.\npackage main\n\nfunc main() {}\n"),
        ):
            with open(os.path.join(tool_dir, rel), "w", encoding="utf-8") as f:
                f.write(text)
        gomod = support.read_text(os.path.join(w.repo, "gate/proofs/go.mod"))
        w.write(
            "gate/proofs/go.mod",
            f"{gomod}\ntool {TOOL}\n\ntool {SECOND_TOOL}/cmd/t\n\nrequire {SECOND_TOOL} {VERSION}\n\n"
            f"replace {SECOND_TOOL} => ../../../plantedtool\n",
        )
        self.assertRedLine(self.run_row(), f"deps: {SECOND_TOOL}: license field MIT, file reads BSD-3-Clause")

    def test_a_third_tracked_go_mod_is_red(self):
        self.deps_world(license_text=MIT, entries=[ENTRY])
        self.world.track("internal/x/go.mod", "module github.com/vorsar/cep/internal/x\n\ngo 1.27.0\n")
        self.assertRedLine(
            self.run_row(),
            "deps: tracked go.mod files: gate/proofs/go.mod, go.mod, internal/x/go.mod; "
            "ADR-0001 \u00a71 allows two: go.mod, gate/proofs/go.mod",
        )

    def test_an_unused_require_is_red_by_the_tidy_line(self):
        # The plant's require moves to the top of go.mod and no package imports it: tidy drops it, and it is the
        # first change in the root's diff (the world's other untidy requires come after it).
        self.deps_world(license_text=MIT, entries=[ENTRY])
        w = self.world
        w.write("internal/planted/planted.go", "// Package planted imports nothing.\npackage planted\n")
        gomod = support.read_text(os.path.join(w.repo, "go.mod"))
        gomod = gomod.replace(f"\nrequire {PLANTED} {VERSION}\n", "\n")
        w.write("go.mod", gomod.replace("\ntoolchain ", f"\nrequire {PLANTED} {VERSION}\n\ntoolchain ", 1))
        self.assertRedLine(
            self.run_row(), f"deps: go mod tidy would change .: go.mod: -{PLANTED} {VERSION}", drop=TIDY_PROOFS
        )

    def test_a_go_sum_edited_after_the_commit_is_red_at_T4(self):
        self.deps_world(license_text=MIT, entries=[ENTRY])
        self.world.commit()
        path = os.path.join(self.world.repo, "go.sum")
        self.world.write("go.sum", support.read_text(path) + "example.com/written v1.0.0/go.mod h1:written=\n")
        self.assertRedLine(
            self.run_row(tier="T4", commit=False),
            "deps: go.sum differs from HEAD at T4: commit it, or fix the step that wrote it",
        )
