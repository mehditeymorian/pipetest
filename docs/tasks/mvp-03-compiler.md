# mvp-03-compiler

title: Semantic compiler passes, diagnostics, and plan IR

description: Implement compiler passes over the AST, produce a validated execution plan, and emit diagnostics with stable canonical codes.

files:
- internal/compiler/
- internal/diagnostics/
- testdata/compiler/valid/
- testdata/compiler/invalid/
- testdata/compiler/golden/

constraints:
- Implement semantic pass rules from docs/rule-sets.md and docs/language-constraints.md.
- Use canonical diagnostic codes and output fields defined in docs/diagnostics.md.
- Maintain deterministic pass ordering and diagnostic sorting/deduping.
- No HTTP execution in this phase.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...

acceptance criteria:
- Pass ordering is deterministic and stops only on unrecoverable parse corruption.
- Canonical error codes are emitted for all semantic and import errors.
- Produces a validated execution plan with resolved inheritance and flow steps.
