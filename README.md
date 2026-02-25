# pipetest

`pipetest` is a scripting language for testing APIs and API flows. It is a general-purpose tool intended for public use, and it does not expose an API or SDK.

`pipetest` has two primary user commands:

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


## Implementation workflow (doc-first)

To keep implementation aligned with the project specs, this repo now includes:

- `AGENTS.md` — operating rules for doc-first, milestone-based implementation
- `.codex/config.toml` — repo-scoped Codex execution defaults
- `EXECPLAN.template.md` — template used to create milestone plans from `/docs`
- `QUESTIONS.md` — capture ambiguities instead of guessing behavior

Recommended execution loop:

1. Generate `EXECPLAN.md` from `/docs/**` + `README.md`.
2. Execute one milestone at a time (M0, M1, M2, ...).
3. Run verification commands and keep the tree buildable between milestones.

## Suggested first build order

1. Lexer with INDENT/DEDENT + hook/object brace handling
2. Parser (top-level declarations + requests + flows + Pratt expressions)
3. Compiler semantic passes (imports, symbols, inheritance, flow checks, variable checks)
4. Executor (HTTP runtime, hooks, assertions, variable propagation)
5. Reporters (JSON + JUnit XML)


## CLI usage

Run static checks:

```bash
pipetest eval path/to/program.pt
```

Run flows and generate reports:

```bash
pipetest run path/to/program.pt --report-dir ./pipetest-report
```

`pipetest run` writes these artifacts in the report directory:

- `pipetest-junit.xml`
- `pipetest-report.xml`
- `pipetest-report.json`
