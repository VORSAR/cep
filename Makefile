# The gate's entry points (S1.1a-1; design note s1-1a-gate-design.md §3–§6). Every gate target installs any
# missing pinned tools first, then hands over to the pinned CPython through the bootstrap; a system python3 >= 3.12
# only starts it, with -I so that nothing in the caller's environment can run inside it.

.PHONY: tools clean-tools setup ci-workflow check combine-check strict-gate ci

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
