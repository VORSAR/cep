"""The deps row's proofs (S1.1c-1, plan check v2 section 6 "deps"; ADR-0009 section 1, ADR-0001 section 1): a
module in the graph that the allowlist does not list, a listed module whose licence file is unknown, missing or
disagrees with its `license` field, an allowlist entry in no graph, and a third tracked go.mod are each red, named.

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
asserts the row's `failed:` lines are exactly its one planted line (design note section 5.4).
"""

import json
import os

from gate.proofs.drivers import support

ALLOWLIST = "tools/deps-allowlist.json"
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


def without_tool(gomod: str) -> str:
    """A go.mod text without the apidiff tool directive; its requires stay."""
    return "".join(line for line in gomod.splitlines(keepends=True) if line.split()[:2] != ["tool", TOOL])


class DepsPlanted(support.DriverTest):
    def deps_world(self, *, license_text: str | None, entries: list[dict]) -> None:
        w = self.world
        w.copy_gate()
        for rel in ("go.mod", "go.sum", "gate/proofs/go.mod", "gate/proofs/go.sum", ALLOWLIST):
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
        data["modules"].extend(entries)
        w.write(ALLOWLIST, json.dumps(data, indent=2, ensure_ascii=False) + "\n")

    def run_row(self) -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row("deps")), tier="T2")

    def assertRedLine(self, run: support.Run, expected: str) -> None:
        """Red, the row FAIL, and its `failed:` lines exactly the one planted line, which is the row's reason."""
        self.assertGateFailed(run)
        record = run.record("lint", "deps")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        out = run.row_stdout("lint", "deps")
        failed = [line.removeprefix("failed: ") for line in out.split("\n") if line.startswith("failed: ")]
        self.assertEqual(failed, [expected], record["reason"])
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

    def test_a_third_tracked_go_mod_is_red(self):
        self.deps_world(license_text=MIT, entries=[ENTRY])
        self.world.track("internal/x/go.mod", "module github.com/vorsar/cep/internal/x\n\ngo 1.27.0\n")
        self.assertRedLine(
            self.run_row(),
            "deps: tracked go.mod files: gate/proofs/go.mod, go.mod, internal/x/go.mod; "
            "ADR-0001 \u00a71 allows two: go.mod, gate/proofs/go.mod",
        )
