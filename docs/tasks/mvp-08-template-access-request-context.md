# mvp-08-template-access-request-context

title: Template placeholders for request-context symbols (`req`, `res`, `status`)

description: Extend template interpolation support so request-context symbols like `req`, `res`, and `status` can be referenced through `{{...}}` placeholders in supported string interpolation locations, with clear compile/runtime behavior definitions.

files:
- internal/compiler/
- internal/runtime/
- docs/plan.md
- docs/language-guide.md
- docs/rule-sets.md
- docs/diagnostics.md
- testdata/compiler/valid/
- testdata/compiler/invalid/
- testdata/runtime/valid/
- testdata/runtime/invalid/

constraints:
- Align with existing `{{var}}` interpolation semantics and path `:param` rules in docs/plan.md.
- Define and enforce scope/timing rules for request-context symbols in templates (request hook scope vs flow scope).
- Preserve existing undefined-variable diagnostics for unsupported scopes.
- Keep interpolation deterministic and side-effect free.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...
- go build ./...

acceptance criteria:
- `{{status}}`, `{{req}}`, and `{{res}}` are supported where scope allows, with explicit diagnostics otherwise.
- Compiler/runtime checks correctly distinguish normal variables from request-context template symbols.
- Existing variable interpolation behavior remains backward-compatible.
- Added fixtures validate request-scope success and invalid-scope diagnostics.
