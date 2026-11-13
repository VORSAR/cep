# The gate's entry points (S1.1a-1; design note s1-1a-gate-design.md §3–§6). Every gate target installs any
# missing pinned tools first, then hands over to the pinned CPython through the bootstrap; a system python3 >= 3.12
# only starts it, with -I so that nothing in the caller's environment can run inside it.

.PHONY: tools clean-tools setup ci-workflow check combine-check strict-gate ci regen-baseline vulndb

tools:
	python3 -I -B gate/bootstrap.py install

clean-tools:
	python3 -I -B gate/bootstrap.py clean-tools

setup:
	git config core.hooksPath .githooks

ci-workflow: tools
	python3 -I -B gate/bootstrap.py exec -- generate-ci

check: tools
	python3 -I -B gate/bootstrap.py exec -- --tier T2

combine-check: tools
	python3 -I -B gate/bootstrap.py exec -- --tier T3

strict-gate: tools
	python3 -I -B gate/bootstrap.py exec -- --tier T4

ci: strict-gate

# The vulns row's copy of the Go vulnerability database (S1.1c-4 design §2.3): one GET of vuln.go.dev/vulndb.zip,
# validated before it replaces the copy; a failure leaves the copy as it was and names itself in fetch-error.txt.
vulndb: tools
	python3 -I -B gate/bootstrap.py exec -- fetch-vulndb

# Tier 2 (design s3-3-2 §8.3): re-record conformance/baseline/ after Tier 1 passes; a refused class writes nothing.
# ACCEPT names the blocks a SPEC revision changed: ACCEPT='<file>:<rule>:<ordinal> …' or a whole '<file>'.
regen-baseline: tools
	python3 -I -B gate/baseline/regen.py --accept "$(ACCEPT)"
