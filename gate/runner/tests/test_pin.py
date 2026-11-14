"""gate/runner/pin.py (S1.1c-5 design §2.2, §5, §8 4.4; DQ-S1, DQ-L1): the type-strict whole-document diff behind the
.golangci.toml pin and the ruff pin. Every expected line is the design's text, written by hand."""

import unittest

from gate.runner import pin

PINNED = {
    "run": {"tests": True, "timeout": "5m"},
    "linters": {"enable": ["errcheck", "lll"], "settings": {"revive": {"rules": [{"name": "a"}, {"name": "b"}]}}},
}


def _copy(value):
    if isinstance(value, dict):
        return {k: _copy(v) for k, v in value.items()}
    if isinstance(value, list):
        return [_copy(v) for v in value]
    return value


class Differences(unittest.TestCase):
    def test_equal_is_empty(self):
        self.assertEqual(pin.differences(_copy(PINNED), PINNED, "x.toml"), [])

    def test_added_removed_changed_paths(self):
        doc = _copy(PINNED)
        doc["run"]["timeout"] = "1m"
        del doc["run"]["tests"]
        doc["issues"] = {"new-from-rev": "HEAD"}
        self.assertEqual(
            pin.differences(doc, PINNED, "x.toml"),
            [
                "x.toml: issues: not pinned (remove it)",
                "x.toml: run.tests: missing (pinned True)",
                "x.toml: run.timeout: '1m' ≠ pinned '5m'",
            ],
        )

    def test_type_strict(self):
        # Python's == says True == 1 and 5 == 5.0 (M-11); the pin must not.
        for got, want in ((1, True), (True, 1), (5.0, 5), (5, 5.0), (0, False)):
            with self.subTest(got=got, want=want):
                self.assertEqual(
                    pin.differences({"a": {"b": [got]}}, {"a": {"b": [want]}}, "x.toml"),
                    [f"x.toml: a.b[0]: {got!r} ≠ pinned {want!r}"],
                )

    def test_list_order_counts(self):
        doc = _copy(PINNED)
        doc["linters"]["enable"] = ["lll", "errcheck"]
        self.assertEqual(
            pin.differences(doc, PINNED, "x.toml"),
            [
                "x.toml: linters.enable[0]: 'lll' ≠ pinned 'errcheck'",
                "x.toml: linters.enable[1]: 'errcheck' ≠ pinned 'lll'",
            ],
        )

    def test_dotted_key_quoted(self):
        self.assertEqual(
            pin.differences({"a": {"b.c": 1, "d e": 2, "f[0]": 3}}, {"a": {}}, "x.toml"),
            [
                'x.toml: a."b.c": not pinned (remove it)',
                'x.toml: a."d e": not pinned (remove it)',
                'x.toml: a."f[0]": not pinned (remove it)',
            ],
        )

    def test_a_list_path_is_indexed(self):
        doc = _copy(PINNED)
        doc["linters"]["settings"]["revive"]["rules"][1]["name"] = "c"
        self.assertEqual(
            pin.differences(doc, PINNED, "x.toml"),
            ["x.toml: linters.settings.revive.rules[1].name: 'c' ≠ pinned 'b'"],
        )


if __name__ == "__main__":
    unittest.main()
