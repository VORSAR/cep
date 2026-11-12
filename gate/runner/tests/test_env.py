"""The row environment (design note §4 step 1; plan check S3a, C25)."""

import unittest

from gate.runner import env


class CleanEnv(unittest.TestCase):
    def test_sets_the_pinned_go_settings_over_the_callers(self):
        base = {"GOTOOLCHAIN": "go1.27.2", "GOENV": "/x", "GOFLAGS": "-mod=mod", "GOWORK": "/w", "CGO_ENABLED": "1"}
        got = env.clean_env(base, [])
        self.assertEqual(got["GOTOOLCHAIN"], "local")
        self.assertEqual(got["GOENV"], "off")
        self.assertEqual(got["GOFLAGS"], "")
        self.assertEqual(got["GOWORK"], "off")
        self.assertEqual(got["CGO_ENABLED"], "0")

    def test_keeps_exactly_the_go_allow_list(self):
        keep = ("GOCACHE", "GOMODCACHE", "GOPATH", "GOPROXY", "GOSUMDB", "GOPRIVATE", "GONOPROXY", "GONOSUMDB")
        got = env.clean_env({k: "v" for k in keep}, [])
        for key in keep:
            self.assertEqual(got[key], "v", key)

    def test_drops_every_other_go_variable(self):
        got = env.clean_env({"GOROOT": "/r", "GOEXPERIMENT": "x", "GODEBUG": "y", "GOBIN": "/b", "GOOS": "plan9"}, [])
        for key in ("GOROOT", "GOEXPERIMENT", "GODEBUG", "GOBIN", "GOOS"):
            self.assertNotIn(key, got)

    def test_drops_python_and_pathspec_variables(self):
        keys = ("PYTHONPATH", "PYTHONHOME", "PYTHONHASHSEED", "PYTHONSTARTUP", "GIT_LITERAL_PATHSPECS",
                "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS")  # fmt: skip  # keys as a table
        got = env.clean_env(dict.fromkeys(keys, "1"), [])
        for key in keys:
            self.assertNotIn(key, got)

    def test_drops_every_ruff_variable(self):
        # Review BA2: RUFF_OUTPUT_FORMAT=json turned a clean tree red, and RUFF_OUTPUT_FILE wrote outside the run.
        keys = ("RUFF_OUTPUT_FORMAT", "RUFF_OUTPUT_FILE", "RUFF_CACHE_DIR", "RUFF_NO_CACHE", "RUFF_TARGET_VERSION")
        got = env.clean_env(dict.fromkeys(keys, "1"), [])
        for key in keys:
            self.assertNotIn(key, got)

    def test_keeps_the_git_location_variables_and_the_rest(self):
        keys = ("GIT_INDEX_FILE", "GIT_DIR", "GIT_WORK_TREE", "HOME", "TMPDIR")
        got = env.clean_env(dict.fromkeys(keys, "/k"), [])
        for key in keys:
            self.assertEqual(got[key], "/k", key)

    def test_the_cache_bins_come_first_in_order(self):
        got = env.clean_env({"PATH": "/usr/bin:/bin"}, ["/c/go/bin", "/c/py/bin"])
        self.assertEqual(got["PATH"], "/c/go/bin:/c/py/bin:/usr/bin:/bin")

    def test_an_empty_inherited_path_adds_no_empty_entry(self):
        # An empty PATH entry means the current directory: a planted `go` in the worktree would run.
        for base in ({}, {"PATH": ""}):
            with self.subTest(base):
                self.assertEqual(env.clean_env(base, ["/c/go/bin"])["PATH"], "/c/go/bin")

    def test_the_input_is_not_changed(self):
        base = {"GOFLAGS": "x"}
        env.clean_env(base, [])
        self.assertEqual(base, {"GOFLAGS": "x"})


class RowEnv(unittest.TestCase):
    def test_adds_the_four_row_variables_over_any_inherited_ones(self):
        got = env.row_env({"HOME": "/h", "GATE_ROW": "outer"}, tier="T2", row="vet", run_dir="/r", row_dir="/r/x")
        self.assertEqual(got["GATE_TIER"], "T2")
        self.assertEqual(got["GATE_ROW"], "vet")
        self.assertEqual(got["GATE_RUN_DIR"], "/r")
        self.assertEqual(got["GATE_ROW_DIR"], "/r/x")
        self.assertEqual(got["HOME"], "/h")


if __name__ == "__main__":
    unittest.main()
