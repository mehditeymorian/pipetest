# Implementation roadmap: parser, compiler, executor

This plan translates the existing grammar and design docs into an executable implementation backlog.

## 1) Deliverables

Build a `pipetest` CLI with:

- `pipetest eval program.pt`
- `pipetest run program.pt`

Core components:

1. **Lexer** (layout-aware tokenization)
2. **Parser** (AST generation)
3. **Compiler** (multi-pass semantic analysis + execution plan)
4. **Executor** (HTTP runtime + assertions + variable scopes)
5. **Reporters** (machine-friendly CI output)

---

## 2) Architecture overview

```text
source .pt file(s)
   -> import resolver
   -> lexer
   -> parser (AST)
   -> compiler semantic passes
   -> validated execution plan
   -> runtime executor
   -> result model
   -> reporters (stdout summary + JSON + JUnit)
```

Suggested package structure:

```text
internal/
  lexer/
  parser/
  ast/
  compiler/
    imports/
    symbols/
    inheritance/
    flowcheck/
    varcheck/
  runtime/
    httpclient/
    hooks/
    assert/
  report/
cmd/pipetest/
```

---

## 3) Parser plan

### 3.1 Lexer milestones

Implement a deterministic lexer aligned to `grammar.ebnf` and `docs/rule-sets.md`:

- newline normalization (`\n`)
- comment stripping outside strings
- `INDENT`/`DEDENT` generation
- `PATH` token support with `:path_param`
- hook brace tracking (`pre hook {}`, `post hook {}`)
- expression grouping depth (`()`, `[]`, `{}`)

Errors to ship immediately:

- invalid dedent
- tab indentation usage
- unterminated strings
- brace/paren mismatch

### 3.2 Parser milestones

Use recursive descent for statements and Pratt for expressions.

1. Parse top-level declarations: `base`, `timeout`, `import`, `let`, `req`, `flow`
2. Parse request body lines:
   - HTTP line
   - directives (`json`, `header`, `query`, `auth bearer`)
   - hooks
   - request assertions and lets
3. Parse flow with strict zones:
   - prelude (`let` only)
   - one chain line (`->` required)
   - postlude (assertions only)
4. Parse postfix expressions (call, index, field)

Parser output: AST with source spans for all nodes.

---

## 4) Compiler plan (semantic analysis)

Implement semantic checks as ordered passes to maximize error quality.

### Pass 0 — import graph

- resolve relative imports
- detect missing files and cycles
- disallow flows in imported files

### Pass 1 — symbol tables

- collect requests globally
- collect flow names in entry file
- enforce uniqueness and reserved-name constraints

### Pass 2 — request inheritance

- validate parent existence
- detect cycles
- build expanded request templates (child overrides parent)

### Pass 3 — request validity

- single HTTP line per request
- duplicate hook constraints
- body/directive multiplicity constraints

### Pass 4 — hook restrictions

- pre-hook cannot reference `res` or `$`
- post-hook cannot assign `res`
- enforce allowed lvalue targets

### Pass 5 — flow structure + bindings

- ensure chain exists and uses `->`
- ensure all referenced requests exist
- ensure binding uniqueness (alias collisions)

### Pass 6 — variable availability

Per-flow definite-assignment analysis:

- seed with global lets
- apply flow prelude lets
- validate required vars per request (expressions + path params)
- add vars from request-level lets after each step
- validate flow assertions (vars + bindings)

Compiler output: validated `ExecutionPlan` with resolved requests and flow steps.

---

## 5) Executor plan

### 5.1 Runtime model

For each flow execution:

- `FlowScope` map for variables
- `Bindings` map (step binding -> result snapshot)
- request context (`req` mutable draft, `res/$/status/header` after HTTP)

### 5.2 Request execution sequence

For each step:

1. materialize request template + inherited fields
2. evaluate path params and directive expressions
3. run pre-hook
4. send HTTP request
5. bind response context
6. run post-hook
7. execute request assertions and lets in source order
8. save step binding result for flow assertions

### 5.3 Runtime failures

Return typed runtime errors:

- missing path variable
- HTTP/transport failure
- assertion failure
- hook evaluation failure

Continue-on-failure policy (recommended default): fail current flow fast; continue next flow.

---

## 6) CLI behavior plan

### `pipetest eval program.pt`

- compile only
- print diagnostics
- non-zero exit if any diagnostics of severity error

### `pipetest run program.pt`

- compile + execute
- print human summary
- write machine report artifacts (default directory configurable)
- non-zero exit if compile errors, runtime errors, or assertion failures

---

## 7) CI/CD reporting plan

Generate at least two formats:

1. **JSON** (`pipetest-report.json`) for custom tooling
2. **JUnit XML** (`pipetest-junit.xml`) for CI test ingestion

Recommended mapping:

- each flow -> testsuite
- each request assertion + flow assertion -> testcase
- assertion/runtime failures -> testcase failure/error

GitHub Actions and GitLab CI can both ingest JUnit XML.

---

## 8) Iteration plan

### Phase 1 (MVP compile)

- lexer/parser
- AST + diagnostics
- `eval` command functional

### Phase 2 (MVP run)

- compiler passes
- basic executor (no retries)
- assertions and variable propagation
- JSON report

### Phase 3 (CI-grade)

- JUnit output
- improved diagnostics with source spans and hints
- stable exit code matrix and docs

---

## 9) Definition of done

- `pipetest eval` validates syntax + imports + semantics with precise diagnostics
- `pipetest run` executes flows and evaluates all assertions
- report artifacts produced and usable in GitHub Actions / GitLab CI
- docs include CLI spec, report schema, and examples
