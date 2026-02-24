# mvp-00-scaffold

title: Project scaffolding and module setup

description: Initialize the Go module and create the base package layout so later MVP phases can compile and run.

files:
- go.mod
- go.sum
- cmd/pipetest/main.go
- internal/
- testdata/

constraints:
- Follow the package layout in docs/implementation-roadmap.md.
- Keep the repository buildable between tasks.
- If the module path is unclear, record the decision in QUESTIONS.md and add TODOs instead of guessing.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...
- go build ./...

acceptance criteria:
- Go module initialized with a documented module path.
- Base directories exist for cmd, internal, and testdata.
- `go build ./...` succeeds.
- `go test ./...` succeeds.
