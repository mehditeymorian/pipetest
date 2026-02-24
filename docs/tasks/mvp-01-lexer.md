# mvp-01-lexer

title: Lexer with layout and hook handling

description: Implement the lexer with NL/INDENT/DEDENT, hook brace classification, PATH tokens, and precise spans, producing tokens plus lex errors.

files:
- internal/lexer/token.go
- internal/lexer/lexer.go
- internal/lexer/errors.go
- internal/lexer/lexer_test.go
- testdata/lexer/valid/
- testdata/lexer/invalid/
- testdata/lexer/golden/

constraints:
- Implement the pre-pass and tokenization rules in docs/rule-sets.md (A1 through A8).
- Follow the lexer guidance in docs/parser.md and grammar.ebnf.
- Reject tabs in indentation, and treat hook braces as brace-scoped with no INDENT/DEDENT inside.
- Do not introduce AST dependencies in internal/lexer.

test:
- gofmt -w .
- golangci-lint run ./...
- go test ./...

acceptance criteria:
- Handles nested blocks and emits correct INDENT/DEDENT for req and flow bodies.
- Rejects tab indentation and malformed dedent stacks with clear lex errors.
- Tracks balanced delimiters and hook brace mode correctly.
- Emits accurate spans for all tokens and diagnostics.
