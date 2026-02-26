# mvp-06-request-inheritance-override

title: Deterministic request inheritance with child-overrides-parent semantics

description: Ensure inherited requests materialize a single effective request shape where all inherited attributes are included and child request declarations overwrite parent attributes for HTTP line, directives, hooks, assertions, and lets according to language semantics.

files:
- internal/compiler/
- internal/runtime/
- internal/ast/
- docs/language-guide.md
- docs/rule-sets.md
- testdata/compiler/valid/
- testdata/compiler/invalid/
- testdata/runtime/valid/
- testdata/runtime/invalid/

constraints:
- Follow inheritance behavior in docs/implementation-roadmap.md and docs/language-guide.md.
- Materialization must be deterministic and preserve source spans for diagnostics where possible.
- Parent attributes are inherited first; child attributes overwrite parent values for overlapping keys/sections.
- Keep request validity constraints (single effective HTTP line, duplicate hook/body restrictions) aligned with docs/language-constraints.md.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...
- go build ./...

acceptance criteria:
- `req child(parent):` receives all inherited attributes from parent.
- Child definitions overwrite inherited parent attributes for overlapping request properties.
- Compiler/runtime behavior for inherited requests is deterministic across runs.
- Added fixtures cover merge precedence, conflict cases, and expected diagnostics.
