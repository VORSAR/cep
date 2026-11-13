"""The canary tests' shared fixtures (S1.1c-4 W0c): the recorded feeds, the pins they were recorded against, and a
fake `archive` seam that writes a small archive shaped as each upstream ships it. Never the network."""

import copy
import hashlib
import json
import os
import re
import tempfile
import unittest

from gate.toolchain.tests import support

HERE = os.path.dirname(os.path.abspath(__file__))
TESTDATA = os.path.join(os.path.dirname(HERE), "testdata")
FEEDS = os.path.join(TESTDATA, "feeds")
PLATFORMS = ("darwin-arm64", "linux-amd64")
# The feed each tool reads (design §4.2): python and python-floor share python-build-standalone's.
FEED_FILE = {
    "python": "python-build-standalone.json",
    "python-floor": "python-build-standalone.json",
    "go": "go.json",
    "golangci-lint": "golangci-lint.json",
    "ruff": "ruff.json",
    "lark": "lark.json",
}


def pinned() -> dict:
    """tools.json as it stood when the feeds were recorded (2026-11-13; a copy, so a later pin move keeps these)."""
    with open(os.path.join(TESTDATA, "pinned-tools.json"), encoding="utf-8") as f:
        return json.load(f)


def feeds() -> dict[str, object]:
    """Each tool's recorded feed, parsed; python and python-floor get equal documents."""
    out = {}
    for name, file in FEED_FILE.items():
        with open(os.path.join(FEEDS, file), encoding="utf-8") as f:
            out[name] = json.load(f)
    return out


def tool(doc: dict, name: str) -> dict:
    return next(t for t in doc["tools"] if t["name"] == name)


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def gh_release(feed: list, tag: str, new_tag: str, *, rename: tuple[str, str] | None = None, **flags) -> dict:
    """A copy of the release `tag` re-tagged `new_tag`: every asset's name and url carry the new version, each
    digest made new (so a test can tell it from the original's)."""
    old = next(r for r in feed if r["tag_name"] == tag)
    new = copy.deepcopy(old)
    new["tag_name"] = new_tag
    a, b = rename or (tag, new_tag)
    for asset in new["assets"]:
        asset["name"] = asset["name"].replace(a, b)
        asset["browser_download_url"] = asset["browser_download_url"].replace(a, b).replace(f"/{tag}/", f"/{new_tag}/")
        asset["digest"] = "sha256:" + sha(asset["browser_download_url"].encode())
    new.update(flags)
    return new


def platform_of(url: str) -> str:
    if "darwin" in url:
        return "darwin-arm64"
    if "linux" in url:
        return "linux-amd64"
    return ""


def tool_of(url: str) -> str:
    name = url.rsplit("/", 1)[-1]
    if "golangci-lint" in url:
        return "golangci-lint"
    if "/astral-sh/ruff/" in url:
        return "ruff"
    if url.startswith("https://go.dev/dl/"):
        return "go"
    if name.startswith("cpython-3.12"):
        return "python-floor"
    if name.startswith("cpython-3."):
        return "python"
    if name.startswith("lark-"):
        return "lark"
    raise AssertionError(f"the fake archive has no upstream for {url}")


def strip_of(url: str) -> str:
    """The top directory each upstream's archive holds: go's go/, the Pythons' python/, the others' file stem."""
    name = url.rsplit("/", 1)[-1]
    if url.startswith("https://go.dev/dl/"):
        return "go/"
    if name.startswith("cpython-"):
        return "python/"
    return re.sub(r"\.tar\.gz$", "", name) + "/"


def member_data(url: str, rel: str) -> bytes:
    return f"{url}\n{rel}\n".encode()


class FakeArchive:
    """The `archive(url, sha256)` seam: records each call and returns a local .tar.gz shaped as the upstream's (its
    strip directory, each executable the pin lists, a nested file and a symlink). `fail` maps a url to the error
    the real seam raises for it (a digest mismatch)."""

    def __init__(self, test: unittest.TestCase, pins: dict, fail: dict[str, Exception] | None = None) -> None:
        tmp = tempfile.TemporaryDirectory(prefix="canary-archives-")
        test.addCleanup(tmp.cleanup)
        self.dir = tmp.name
        self.pins = pins
        self.fail = fail or {}
        self.calls: list[tuple[str, str]] = []

    def executables(self, url: str) -> list[str]:
        name, platform = tool_of(url), platform_of(url) or "linux-amd64"
        return sorted(tool(self.pins, name)["platforms"][platform]["exec_sha256"])

    def __call__(self, url: str, sha256: str) -> str:
        self.calls.append((url, sha256))
        if url in self.fail:
            raise self.fail[url]
        strip = strip_of(url)
        members = [support.Member(strip.rstrip("/"), "d", mode=0o755)]
        members += [
            support.Member(strip + rel, data=member_data(url, rel), mode=0o755) for rel in self.executables(url)
        ]
        members += [
            support.Member(strip + "share/doc/notes.txt", data=b"notes\n"),
            support.Member(strip + "share/notes", "l", link="doc/notes.txt"),
        ]
        path = os.path.join(self.dir, f"{len(self.calls)}.tar.gz")
        return support.write_archive(path, members)

    def urls(self) -> list[str]:
        return [url for url, _ in self.calls]
