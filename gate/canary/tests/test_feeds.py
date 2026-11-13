"""feeds.fetch over a fake `get` (S1.1c-4 design §4.2; plan check §6 canary (5)). gate/canary/feeds.py does not exist
at W0c: every test here is red by ModuleNotFoundError until lane F1 adds it.

The seam as these tests read it (design §4.2 names `get` but not its shape — W0c report, ambiguity 1):
`get(url, headers) -> (status, response_headers, body_bytes)`, `response_headers` an email.message.Message (case-blind
like http.client's). One request per feed; python and python-floor share python-build-standalone's.
"""

import calendar
import email.message
import json
import unittest

from gate.canary.candidate import CanaryError
from gate.canary.tests import support as fs

GO = "https://go.dev/dl/?mode=json&include=all"
GH = "https://api.github.com/repos/{}/releases?per_page={}"
URLS = {
    "python": GH.format("astral-sh/python-build-standalone", 10),
    "go": GO,
    "golangci-lint": GH.format("golangci/golangci-lint", 30),
    "ruff": GH.format("astral-sh/ruff", 30),
    "lark": "https://pypi.org/pypi/lark/json",
}
RESET = calendar.timegm((2026, 11, 13, 20, 0, 0))  # 2026-11-13T20:00:00Z


def headers(**fields: str) -> email.message.Message:
    msg = email.message.Message()
    for key, value in fields.items():
        msg[key.replace("_", "-")] = value
    return msg


class FakeGet:
    def __init__(self) -> None:
        recorded = fs.feeds()
        self.answers = {
            URLS[name]: (200, headers(content_type="application/json"), json.dumps(recorded[name]).encode())
            for name in URLS
        }
        self.calls: list[tuple[str, dict]] = []

    def __call__(self, url, request_headers):
        self.calls.append((url, dict(request_headers)))
        return self.answers[url]


class Fetch(unittest.TestCase):
    def setUp(self) -> None:
        from gate.canary import feeds  # red by ModuleNotFoundError until F1 adds it

        self.feeds = feeds
        self.get = FakeGet()

    def fetch(self):
        return self.feeds.fetch(fs.pinned(), token="tok-123", get=self.get)

    def test_one_request_per_feed_and_every_tool_keyed(self):
        got = self.fetch()
        self.assertEqual(sorted(url for url, _ in self.get.calls), sorted(URLS.values()))
        self.assertEqual(set(got), {"python", "python-floor", "go", "golangci-lint", "ruff", "lark"})
        recorded = fs.feeds()
        for name in got:
            with self.subTest(tool=name):
                self.assertEqual(got[name], recorded[name])

    def test_token_on_github_requests_only(self):
        self.fetch()
        for url, sent in self.get.calls:
            with self.subTest(url=url):
                auth = {k.lower(): v for k, v in sent.items()}.get("authorization")
                if url.startswith("https://api.github.com/"):
                    self.assertEqual(auth, "Bearer tok-123")
                else:
                    self.assertIsNone(auth)

    def test_non_200_is_named(self):
        self.get.answers[GO] = (500, headers(), b"oops")
        with self.assertRaises(CanaryError) as cm:
            self.fetch()
        self.assertEqual(str(cm.exception), f"go: {GO} answered 500")

    def test_403_with_requests_left_is_a_plain_non_200(self):
        self.get.answers[URLS["ruff"]] = (403, headers(x_ratelimit_remaining="12", x_ratelimit_limit="5000"), b"{}")
        with self.assertRaises(CanaryError) as cm:
            self.fetch()
        self.assertEqual(str(cm.exception), f"ruff: {URLS['ruff']} answered 403")

    def assert_rate_limited(self, status: int) -> None:
        self.get.answers[URLS["ruff"]] = (
            status,
            headers(x_ratelimit_remaining="0", x_ratelimit_limit="5000", x_ratelimit_reset=str(RESET)),
            b'{"message": "API rate limit exceeded"}',
        )
        with self.assertRaises(CanaryError) as cm:
            self.fetch()
        text = str(cm.exception)
        self.assertTrue(text.startswith("ruff: GitHub's rate limit (5000/h) is spent until "), text)
        self.assertIn("2026-11-13", text)
        self.assertIn("20:00", text)

    def test_403_rate_limit_is_named(self):
        self.assert_rate_limited(403)

    def test_429_rate_limit_is_named(self):
        self.assert_rate_limited(429)


if __name__ == "__main__":
    unittest.main()
