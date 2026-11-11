"""The deps rule (S1.1c-1 lane C; design note §5.2; plan check v2 §6 "deps"; ADR-0009 §1; ADR-0001 §1), pure, on
fixtures: the allowlist's every fault, the SPDX parts, the five licence fingerprints, the `go list -m -json` stream,
the module graph's nodes, and compare's every failure text in the design's order.

Every expected text is copied from the design note by hand, never from a run of the code.
"""

import json
import unittest

from gate.runner import deps

ALLOWLIST = "tools/deps-allowlist.json"
ALLOWED_TEXT = "(allowed: Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause, ISC)"
GOMODS = ["go.mod", "gate/proofs/go.mod"]
APACHE = "                                 Apache License\n                           Version 2.0, January 2004\n"
MIT = (
    "MIT License\n\nCopyright (c) 2026 a fixture\n\nPermission is hereby granted, free of charge, to any person "
    "obtaining a copy of this software\n"
)
BSD2 = (
    "Redistribution and use in source and binary forms, with or without modification, are permitted provided "
    "that the following conditions are met:\n1. Redistributions of source code must retain the above notice.\n"
)
BSD3 = BSD2 + "3. Neither the name of the holder nor the names of its contributors may be used to endorse or promote\n"
ISC = "ISC License\n\nPermission to use, copy, modify, and/or distribute this software for any purpose\n"
GPL3 = "                    GNU GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007\n"


def entry(module: str, version: str = "v1.0.0", license: str = "MIT") -> dict:
    return {
        "module": module,
        "version": version,
        "scope": "runtime",
        "license": license,
        "approved": "2026-11-11",
        "why": "a fixture",
    }


def listing(*entries: dict, schema: int = 1, note: str | None = "a fixture") -> str:
    data: dict = {"schema": schema, "modules": list(entries)}
    if note is not None:
        data["note"] = note
    return json.dumps(data)


def E(module: str, version: str = "v1.0.0", license: str = "MIT") -> deps.Entry:
    return deps.Entry(module, version, "runtime", license, "2026-11-11", "a fixture")


def M(path: str, version: str = "v1.0.0", dir: str = "", *, imported: bool = False) -> deps.Module:
    return deps.Module(path, version, dir, imported)


def reader(files: dict[str, str]):
    return lambda dir: files.get(dir)


class LoadAllowlist(unittest.TestCase):
    def test_a_listing_loads_in_file_order(self):
        text = listing(entry("example.com/b"), entry("example.com/a", "v2.0.0", "MIT AND Apache-2.0"))
        self.assertEqual(
            deps.load_allowlist(text), (E("example.com/b"), E("example.com/a", "v2.0.0", "MIT AND Apache-2.0"))
        )

    def assertFault(self, text: str, message: str) -> None:
        with self.assertRaises(ValueError) as cm:
            deps.load_allowlist(text)
        self.assertEqual(str(cm.exception), message)

    def test_not_json(self):
        with self.assertRaises(ValueError) as cm:
            deps.load_allowlist("{")
        self.assertTrue(str(cm.exception).startswith(f"{ALLOWLIST}: "), str(cm.exception))

    def test_not_an_object(self):
        self.assertFault("[]", f"{ALLOWLIST}: not an object")

    def test_the_top_level_keys_are_exactly_three(self):
        self.assertFault(
            listing(entry("example.com/a"), note=None),
            f"{ALLOWLIST}: keys ['modules', 'schema'], missing ['note'], unexpected []",
        )
        self.assertFault(
            json.dumps({"schema": 1, "note": "x", "modules": [entry("example.com/a")], "extra": 1}),
            f"{ALLOWLIST}: keys ['extra', 'modules', 'note', 'schema'], missing [], unexpected ['extra']",
        )

    def test_schema_must_be_1(self):
        self.assertFault(listing(entry("example.com/a"), schema=2), f"{ALLOWLIST}: schema 2, not 1")

    def test_note_must_be_a_string(self):
        self.assertFault(
            json.dumps({"schema": 1, "note": 3, "modules": [entry("example.com/a")]}),
            f"{ALLOWLIST}: note is not a string",
        )

    def test_modules_must_be_a_non_empty_list(self):
        self.assertFault(json.dumps({"schema": 1, "note": "x", "modules": {}}), f"{ALLOWLIST}: modules is not a list")
        self.assertFault(listing(), f"{ALLOWLIST}: modules is empty")

    def test_an_entry_must_be_an_object_with_the_six_keys(self):
        self.assertFault(
            json.dumps({"schema": 1, "note": "x", "modules": ["a"]}), f"{ALLOWLIST}: modules[0]: not an object"
        )
        short = {k: v for k, v in entry("example.com/a").items() if k != "why"}
        self.assertFault(
            listing(entry("example.com/b"), short),
            f"{ALLOWLIST}: modules[1]: keys ['approved', 'license', 'module', 'scope', 'version'], "
            "missing ['why'], unexpected []",
        )
        self.assertFault(
            listing({**entry("example.com/a"), "licence": "MIT"}),
            f"{ALLOWLIST}: modules[0]: keys ['approved', 'licence', 'license', 'module', 'scope', 'version', 'why'],"
            " missing [], unexpected ['licence']",
        )

    def test_every_field_is_a_non_empty_string(self):
        self.assertFault(
            listing({**entry("example.com/a"), "version": ""}),
            f"{ALLOWLIST}: modules[0]: version is not a non-empty string",
        )
        self.assertFault(
            listing({**entry("example.com/a"), "approved": 20261111}),
            f"{ALLOWLIST}: modules[0]: approved is not a non-empty string",
        )

    def test_modules_are_unique(self):
        self.assertFault(
            listing(entry("example.com/a"), entry("example.com/b"), entry("example.com/a", "v2.0.0")),
            f"{ALLOWLIST}: modules[2]: duplicate module example.com/a",
        )


class SpdxParts(unittest.TestCase):
    def test_one_part(self):
        self.assertEqual(deps.spdx_parts("MIT"), frozenset({"MIT"}))

    def test_and_splits_and_strips(self):
        self.assertEqual(deps.spdx_parts("MIT AND Apache-2.0"), frozenset({"MIT", "Apache-2.0"}))
        self.assertEqual(deps.spdx_parts(" MIT AND  Apache-2.0 "), frozenset({"MIT", "Apache-2.0"}))

    def test_an_empty_part_is_an_error(self):
        for field in ("", "MIT AND ", " AND MIT", "MIT AND  AND ISC"):
            with self.subTest(field=field), self.assertRaises(ValueError) as cm:
                deps.spdx_parts(field)
            self.assertEqual(str(cm.exception), f"license field {field!r}: empty part")


class ClassifyLicense(unittest.TestCase):
    def test_the_five_fingerprints(self):
        for text, tag in (
            (APACHE, "Apache-2.0"),
            (MIT, "MIT"),
            (BSD2, "BSD-2-Clause"),
            (BSD3, "BSD-3-Clause"),
            (ISC, "ISC"),
        ):
            with self.subTest(tag=tag):
                self.assertEqual(deps.classify_license(text), tag)

    def test_case_does_not_matter(self):
        self.assertEqual(deps.classify_license(MIT.upper()), "MIT")
        self.assertEqual(deps.classify_license(APACHE.lower()), "Apache-2.0")

    def test_gpl_and_empty_are_none(self):
        self.assertIsNone(deps.classify_license(GPL3))
        self.assertIsNone(deps.classify_license(""))

    def test_apache_needs_both_phrases(self):
        self.assertIsNone(deps.classify_license("Apache License\n"))
        self.assertIsNone(deps.classify_license("Version 2.0\n"))


class DecodeJsonStream(unittest.TestCase):
    def test_objects_back_to_back(self):
        text = '{\n\t"Path": "a",\n\t"Main": true\n}\n{\n\t"Path": "b",\n\t"Version": "v1"\n}\n'
        self.assertEqual(deps.decode_json_stream(text), [{"Path": "a", "Main": True}, {"Path": "b", "Version": "v1"}])

    def test_empty_and_blank(self):
        self.assertEqual(deps.decode_json_stream(""), [])
        self.assertEqual(deps.decode_json_stream(" \n"), [])

    def test_trailing_garbage_is_an_error(self):
        with self.assertRaises(ValueError):
            deps.decode_json_stream('{"Path": "a"} x')


class GraphNodes(unittest.TestCase):
    GRAPH = (
        "github.com/vorsar/cep go@1.27.0\n"
        "github.com/vorsar/cep go.uber.org/goleak@v1.3.0\n"
        "github.com/vorsar/cep toolchain@go1.27.3\n"
        "go@1.27.0 toolchain@go1.27.0\n"
        "go.uber.org/goleak@v1.3.0 github.com/kr/pretty@v0.1.0\n"
        "go.uber.org/goleak@v1.3.0 gopkg.in/check.v1@v1.0.0-20180628173108-788fd7840127\n"
    )

    def test_both_ends_the_suffix_cut_go_and_toolchain_and_the_main_module_dropped(self):
        self.assertEqual(
            deps.graph_nodes(self.GRAPH, ["github.com/vorsar/cep"]),
            frozenset({"go.uber.org/goleak", "github.com/kr/pretty", "gopkg.in/check.v1"}),
        )

    def test_empty(self):
        self.assertEqual(deps.graph_nodes("", ["github.com/vorsar/cep"]), frozenset())


class Compare(unittest.TestCase):
    def compare(self, modules, graph, entries, gomods=GOMODS, files=None):
        return deps.compare(modules, frozenset(graph), entries, gomods, reader(files or {}))

    def test_green(self):
        modules = [M("example.com/a", dir="/d/a", imported=True), M("example.com/g")]
        entries = [E("example.com/a"), E("example.com/g", license="BSD-3-Clause")]
        self.assertEqual(self.compare(modules, ["example.com/a", "example.com/g"], entries, files={"/d/a": MIT}), [])

    def test_a_third_tracked_go_mod(self):
        self.assertEqual(
            self.compare([], [], [], gomods=["go.mod", "internal/x/go.mod", "gate/proofs/go.mod", "go.mod"]),
            [
                "deps: tracked go.mod files: gate/proofs/go.mod, go.mod, internal/x/go.mod; "
                "ADR-0001 §1 allows two: go.mod, gate/proofs/go.mod"
            ],
        )
        self.assertEqual(
            self.compare([], [], [], gomods=["go.mod"]),
            ["deps: tracked go.mod files: go.mod; ADR-0001 §1 allows two: go.mod, gate/proofs/go.mod"],
        )

    def test_a_module_without_an_entry(self):
        self.assertEqual(
            self.compare([M("example.com/planted")], ["example.com/planted"], []),
            [f"deps: example.com/planted v1.0.0 is not in {ALLOWLIST}"],
        )

    def test_a_graph_node_without_an_entry(self):
        self.assertEqual(
            self.compare([], ["golang.org/x/sys"], []),
            [f"deps: golang.org/x/sys is in the module graph and not in {ALLOWLIST}"],
        )

    def test_a_graph_node_that_is_also_a_module_is_named_once(self):
        self.assertEqual(
            self.compare([M("golang.org/x/sys", "v0.49.0")], ["golang.org/x/sys"], []),
            [f"deps: golang.org/x/sys v0.49.0 is not in {ALLOWLIST}"],
        )

    def test_a_stale_entry(self):
        self.assertEqual(
            self.compare([], [], [E("example.com/gone")]),
            ["deps: stale entry example.com/gone: not in the module graph of go.mod or gate/proofs/go.mod"],
        )

    def test_a_graph_only_entry_is_live_by_the_graph(self):
        self.assertEqual(self.compare([], ["example.com/g"], [E("example.com/g")]), [])

    def test_a_version_mismatch(self):
        self.assertEqual(
            self.compare([M("example.com/a", "v1.0.0")], ["example.com/a"], [E("example.com/a", "v1.0.1")]),
            ["deps: example.com/a: the allowlist says v1.0.1, the graph has v1.0.0"],
        )

    def test_a_licence_part_outside_the_allowed_set(self):
        self.assertEqual(
            self.compare([M("example.com/a")], ["example.com/a"], [E("example.com/a", license="GPL-3.0")]),
            [f"deps: example.com/a: license GPL-3.0 is not Apache-2 compatible {ALLOWED_TEXT}"],
        )
        self.assertEqual(
            self.compare([M("example.com/a")], ["example.com/a"], [E("example.com/a", license="MIT AND LGPL-2.1")]),
            [f"deps: example.com/a: license LGPL-2.1 is not Apache-2 compatible {ALLOWED_TEXT}"],
        )

    def test_an_empty_licence_part_is_a_line_not_a_crash(self):
        self.assertEqual(
            self.compare([M("example.com/a")], ["example.com/a"], [E("example.com/a", license="MIT AND ")]),
            ["deps: example.com/a: license field 'MIT AND ': empty part"],
        )

    def test_an_imported_module_with_no_source_on_disk(self):
        self.assertEqual(
            self.compare([M("example.com/a", imported=True)], ["example.com/a"], [E("example.com/a")]),
            ["deps: example.com/a: no source on disk; run: go mod download example.com/a"],
        )

    def test_an_imported_module_with_no_licence_file(self):
        self.assertEqual(
            self.compare([M("example.com/a", dir="/d/a", imported=True)], ["example.com/a"], [E("example.com/a")]),
            ["deps: example.com/a: no LICENSE file in /d/a"],
        )

    def test_an_imported_module_with_an_unknown_licence_text(self):
        self.assertEqual(
            self.compare(
                [M("example.com/a", dir="/d/a", imported=True)],
                ["example.com/a"],
                [E("example.com/a")],
                files={"/d/a": GPL3},
            ),
            ["deps: example.com/a: unknown licence text in /d/a"],
        )

    def test_a_field_disagreeing_with_the_file(self):
        self.assertEqual(
            self.compare(
                [M("example.com/a", dir="/d/a", imported=True)],
                ["example.com/a"],
                [E("example.com/a")],
                files={"/d/a": BSD3},
            ),
            ["deps: example.com/a: license field MIT, file reads BSD-3-Clause"],
        )

    def test_a_compound_field_agrees_when_the_file_reads_one_part(self):
        modules = [M("gopkg.in/yaml.v3", "v3.0.1", "/d/yaml", imported=True)]
        entries = [E("gopkg.in/yaml.v3", "v3.0.1", "MIT AND Apache-2.0")]
        self.assertEqual(self.compare(modules, ["gopkg.in/yaml.v3"], entries, files={"/d/yaml": MIT}), [])
        self.assertEqual(self.compare(modules, ["gopkg.in/yaml.v3"], entries, files={"/d/yaml": APACHE}), [])
        self.assertEqual(
            self.compare(modules, ["gopkg.in/yaml.v3"], entries, files={"/d/yaml": ISC}),
            ["deps: gopkg.in/yaml.v3: license field MIT AND Apache-2.0, file reads ISC"],
        )

    def test_a_graph_only_module_is_never_read_from_disk(self):
        modules = [M("example.com/g", dir="/d/g")]
        self.assertEqual(self.compare(modules, ["example.com/g"], [E("example.com/g")], files={"/d/g": GPL3}), [])

    def test_the_failures_come_in_the_designs_order_sorted_within_a_kind(self):
        modules = [
            M("example.com/z", imported=True),
            M("example.com/b", "v1.0.0", "/d/b", imported=True),
            M("example.com/a", dir="/d/a", imported=True),
            M("example.com/u"),
        ]
        graph = ["example.com/z", "example.com/b", "example.com/a", "example.com/u", "example.com/n", "example.com/m"]
        entries = [
            E("example.com/z"),
            E("example.com/b", "v1.0.1", "GPL-3.0"),
            E("example.com/a"),
            E("example.com/gone"),
        ]
        self.assertEqual(
            self.compare(modules, graph, entries, gomods=["go.mod"], files={"/d/b": GPL3}),
            [
                "deps: tracked go.mod files: go.mod; ADR-0001 §1 allows two: go.mod, gate/proofs/go.mod",
                f"deps: example.com/u v1.0.0 is not in {ALLOWLIST}",
                f"deps: example.com/m is in the module graph and not in {ALLOWLIST}",
                f"deps: example.com/n is in the module graph and not in {ALLOWLIST}",
                "deps: stale entry example.com/gone: not in the module graph of go.mod or gate/proofs/go.mod",
                "deps: example.com/b: the allowlist says v1.0.1, the graph has v1.0.0",
                f"deps: example.com/b: license GPL-3.0 is not Apache-2 compatible {ALLOWED_TEXT}",
                "deps: example.com/a: no LICENSE file in /d/a",
                "deps: example.com/b: unknown licence text in /d/b",
                "deps: example.com/z: no source on disk; run: go mod download example.com/z",
            ],
        )


if __name__ == "__main__":
    unittest.main()
