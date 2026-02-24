# mvp-02-parser-ast

title: AST and parser (recursive descent + Pratt expressions)

description: Define AST node types with spans and implement the parser to build ASTs from lexer tokens, covering top-level forms, requests, flows, hooks, directives, and expressions.

files:
- internal/ast/
- internal/parser/
- internal/parser/parser_test.go
- testdata/parser/valid/
- testdata/parser/invalid/
- testdata/parser/golden/

constraints:
- Follow grammar.ebnf and the parser strategy in docs/parser.md.
- Preserve spans on every AST node for later diagnostics.
- Enforce flow shape constraints at parse time per docs/language-constraints.md.
- Keep AST definitions immutable and parser/runtime agnostic per docs/implementation-roadmap.md.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...

acceptance criteria:
- Parses all top-level forms (base, timeout, import, let, req, flow).
- Enforces request line forms, directives, hooks, assertions, and lets.
- Enforces flow shape (prelude lets, exactly one chain line, post-chain assertions).
- Produces AST nodes with correct spans for diagnostics.
