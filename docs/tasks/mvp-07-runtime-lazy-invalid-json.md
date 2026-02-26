# mvp-07-runtime-lazy-invalid-json

title: Lazy handling for invalid JSON responses

description: Prevent hard runtime failure on invalid JSON responses unless JSON-dependent values are accessed; request execution should continue when response JSON is malformed and only raise errors when `#`, `<binding>.res`, or other JSON access paths are evaluated. the response can be used as a whole to print even if it is not json.

files:
- internal/runtime/
- internal/diagnostics/
- docs/plan.md
- docs/rule-sets.md
- docs/diagnostics.md
- testdata/runtime/valid/
- testdata/runtime/invalid/

constraints:
- Preserve current transport/read error handling and only change invalid-JSON behavior.
- Emit clear, typed diagnostics when JSON is accessed but unavailable/invalid.
- Keep request/flow assertion semantics stable except for the lazy JSON access behavior.
- Do not regress existing successful JSON workflows.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...
- go build ./...

acceptance criteria:
- Invalid JSON response alone does not crash or immediately fail request execution.
- Accessing JSON-root dependent expressions (`#`, `.res`, jsonpath over response JSON) emits deterministic diagnostics.
- Non-JSON-dependent assertions and status/header checks still execute.
- Fixtures demonstrate both deferred-failure and no-access success scenarios.
