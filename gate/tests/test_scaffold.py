"""Tests of the throwaway wave-1 scaffold's pure helpers (deleted with it at combine-W1)."""

import unittest

from gate import scaffold


class ParseJSONStream(unittest.TestCase):
    def test_counts_one_object_per_vet_unit(self):
        self.assertEqual(scaffold.parse_json_stream(b"{}\n{}\n{}\n"), [{}, {}, {}])

    def test_reads_multi_line_objects(self):
        data = b'{\n\t"ex.com/v/none": {\n\t\t"printf": [{"posn": "g.go:5:39"}]\n\t}\n}\n{}\n'
        objs = scaffold.parse_json_stream(data)
        self.assertEqual(len(objs), 2)
        self.assertEqual(objs[0]["ex.com/v/none"]["printf"][0]["posn"], "g.go:5:39")
        self.assertEqual(objs[1], {})

    def test_empty_output_is_no_objects(self):
        self.assertEqual(scaffold.parse_json_stream(b"  \n"), [])

    def test_garbage_is_an_error(self):
        with self.assertRaises(ValueError):
            scaffold.parse_json_stream(b"{} not-json")

    def test_a_non_object_is_an_error(self):
        with self.assertRaises(ValueError):
            scaffold.parse_json_stream(b"[1, 2]")


class CleanEnv(unittest.TestCase):
    def test_pins_go_and_drops_unlisted_variables(self):
        base = {
            "PATH": "/usr/bin",
            "GOFLAGS": "-mod=mod",
            "GOTOOLCHAIN": "auto",
            "GOCACHE": "/c",
            "GOEXPERIMENT": "x",
            "PYTHONPATH": "/evil",
            "GIT_LITERAL_PATHSPECS": "1",
            "GIT_INDEX_FILE": "/i",
            "HOME": "/h",
        }
        env = scaffold.clean_env(base, ["/tools/go/bin"])
        self.assertEqual(env["PATH"], "/tools/go/bin:/usr/bin")
        self.assertEqual(env["GOTOOLCHAIN"], "local")
        self.assertEqual(env["GOFLAGS"], "")
        self.assertEqual(env["CGO_ENABLED"], "0")
        self.assertEqual(env["GOCACHE"], "/c")
        self.assertEqual(env["GIT_INDEX_FILE"], "/i")
        self.assertEqual(env["HOME"], "/h")
        for gone in ("GOEXPERIMENT", "PYTHONPATH", "GIT_LITERAL_PATHSPECS"):
            self.assertNotIn(gone, env)


class ToolDirs(unittest.TestCase):
    def test_directory_is_name_version_and_digest_prefix(self):
        manifest = {"tools": [{"name": "ruff", "version": "0.17.3", "platforms": {"p": {"archive_sha256": "a" * 64}}}]}
        dirs = scaffold.tool_dirs(manifest, "p")
        self.assertTrue(dirs["ruff"].endswith("/ruff-0.17.3-aaaaaaaaaaaa"))


if __name__ == "__main__":
    unittest.main()
