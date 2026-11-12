module github.com/vorsar/cep/gate/proofs

go 1.27.0

toolchain go1.27.4

require go.uber.org/goleak v1.3.0

// The gate's API-diff tool, declared here so an embedder's module graph never carries it (owner, 2026-11-11, S1.1c-1's design, 7).
tool golang.org/x/exp/cmd/apidiff

require (
	golang.org/x/exp v0.0.0-20261106192419-47ce03d25ef0
	golang.org/x/mod v0.42.0
	golang.org/x/sync v0.24.0
	golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a
)
