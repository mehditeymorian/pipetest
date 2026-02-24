# mvp-05-cli-reporting

title: CLI integration and report generation

description: Implement `pipetest eval` and `pipetest run`, wire compiler/runtime, and emit CI-friendly reports.

files:
- cmd/pipetest/main.go
- internal/report/
- README.md

constraints:
- Match the command behavior and exit codes in docs/cli-spec.md.
- Generate report artifacts per docs/reporting.md with deterministic naming and mapping.
- Print diagnostics in the format defined in docs/cli-spec.md and docs/diagnostics.md.
- `pipetest run` must always write report files once execution starts.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...
- go build ./...

acceptance criteria:
- `pipetest eval <file>` compiles and exits non-zero on static errors.
- `pipetest run <file>` compiles, executes flows, and exits non-zero on runtime or assertion failures.
- Report artifacts are written with deterministic names and flow/testcase mapping.
