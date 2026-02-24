# mvp-04-runtime

title: Runtime execution engine

description: Execute compiled plans flow-by-flow with HTTP dispatch, hooks, assertions, and variable propagation.

files:
- internal/runtime/
- testdata/runtime/fixtures/
- testdata/runtime/valid/
- testdata/runtime/invalid/
- testdata/runtime/golden/

constraints:
- Follow hook timing and variable scoping rules in docs/plan.md.
- Execute flow chains in order and propagate flow-scoped variables between steps.
- Emit typed runtime failures for transport, hook evaluation, and assertion failures.
- Respect timeout and base settings from the program and CLI overrides.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...

acceptance criteria:
- Executes flow chain order exactly as compiled.
- Pre hooks can mutate req, post hooks can read res and $.
- Request-level lets persist into subsequent flow scope.
- Runtime errors are reported with clear diagnostics.
