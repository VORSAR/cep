# Go vulnerability database entries

The `GO-*.json` files in this directory are unmodified copies of entries of the Go vulnerability database,
served at https://vuln.go.dev (the `ID/<id>.json` members of https://vuln.go.dev/vulndb.zip) and maintained in
https://github.com/golang/vulndb. Database entries are distributed under the terms of the Creative Commons
Attribution 4.0 International licence (CC-BY-4.0), https://creativecommons.org/licenses/by/4.0/.

Fetched on 2026-11-13 (the copy whose `index/db.json` reads `"modified":"2026-11-11T02:51:11Z"`, zip sha256
3b16a8d3ee29af03d9e8f78650c4286b9630569eb0542a93c4207e714c27ef11). They are the agreement table's real entries
(`gate/runner/tests/test_vulndb.py`; S1.1c-4 design note §5; owner 2026-11-13, S1.1c-4's plan check, decision 4).
