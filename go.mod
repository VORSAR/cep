module github.com/vorsar/cep

go 1.27.0

toolchain go1.27.4

require (
	github.com/alecthomas/participle/v2 v2.1.4
	go.uber.org/goleak v1.3.0
	golang.org/x/text v0.43.0
)

// The gate's apidiff tool (declared in gate/proofs/go.mod) builds with these two at these versions: one version of a
// module across the repository's modules (owner, 2026-11-11, S1.1c-1's design, 7). x/text requires both at older
// versions, so they are in every embedder's graph either way. `go mod tidy` drops these lines; the deps row then
// turns red on the two versions, which is the signal to put them back.
require (
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a // indirect
)
