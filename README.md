# pipetest

`pipetest` is a DSL-driven API test tool with two primary user commands:

- `pipetest eval program.pt`
- `pipetest run program.pt`

This repository currently contains the language grammar and design notes. The next implementation milestone is to deliver the lexer/parser, compiler (semantic analyzer + planner), and executor/runtime.

## Documentation index

- `grammar.ebnf` — authoritative language grammar
- `docs/plan.md` — product-level language behavior and examples
- `docs/parser.md` — parser strategy notes (recursive descent + Pratt)
- `docs/rule-sets.md` — lexer and semantic validation rules
- `docs/implementation-roadmap.md` — concrete implementation plan for parser/compiler/executor
- `docs/cli-spec.md` — command behavior for `eval` and `run`
- `docs/reporting.md` — report format defaults, mapping strategy, and CI integration snippets

## Target commands

### `pipetest eval program.pt`
Performs static checks only:

- syntax validation
- import resolution/cycle checks
- semantic validation (flow structure, symbols, hooks, variable availability)

No HTTP requests are executed.

### `pipetest run program.pt`
Compiles and executes flows:

- runs request chains in each flow
- evaluates request-level and flow-level assertions
- emits CI/CD compatible report output for GitHub Actions and GitLab CI

## Suggested first build order

1. Lexer with INDENT/DEDENT + hook/object brace handling
2. Parser (top-level declarations + requests + flows + Pratt expressions)
3. Compiler semantic passes (imports, symbols, inheritance, flow checks, variable checks)
4. Executor (HTTP runtime, hooks, assertions, variable propagation)
5. Reporters (JSON + JUnit XML)
