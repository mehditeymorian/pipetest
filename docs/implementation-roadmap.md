# Implementation roadmap: phased delivery and module boundaries

This roadmap translates `grammar.ebnf` and the design docs into a phased build plan with explicit package ownership, acceptance criteria, and fixture strategy.

## 1) Proposed package layout and ownership

```text
cmd/pipetest
internal/lexer
internal/parser
internal/ast
internal/compiler
internal/runtime
internal/report
testdata/
```

### `cmd/pipetest`
- CLI entrypoint and command wiring (`eval`, `run`, future flags).
- Owns process exit codes and orchestration across compile/execute/report pipelines.

### `internal/lexer`
- Converts source into tokens with source spans.
- Owns layout-sensitive behavior: logical `NL`, `INDENT`, `DEDENT`, tab rejection, comment stripping, and hook brace mode.

### `internal/parser`
- Converts token stream into strongly typed AST.
- Owns parser entry points for top-level forms, request/flow blocks, hook statements, and expression parsing.

### `internal/ast`
- Defines immutable syntax tree node types and span metadata.
- Owns node contracts consumed by semantic passes and runtime planner.

### `internal/compiler` (semantic passes + IR)
- Runs ordered semantic passes against AST.
- Produces validated IR / execution plan with resolved requests, inheritance, flow chains, and variable availability facts.
- Owns diagnostic schema and stable semantic error codes.

### `internal/runtime` (HTTP executor + hooks + env/functions)
- Executes validated plans flow-by-flow and step-by-step.
- Owns hook evaluation contexts (`req`, `res`, `$`, user vars), HTTP transport integration, expression runtime, and function/env bindings.

### `internal/report` (JUnit/GitHub/GitLab output)
- Converts compile/runtime results to machine outputs (JUnit, JSON, GitHub/GitLab-friendly summaries).
- Owns per-flow and per-assertion result mapping.

---

## 2) Phase plan

## Phase 1 — lexer/token stream + indentation/hook handling

**Scope**
- Implement lexical scanner and token model.
- Support `PATH`, literals, identifiers, operators, keywords, and punctuation.
- Implement layout engine (`NL`, `INDENT`, `DEDENT`) and hook-body brace mode where indentation tokens are ignored.

**Module boundaries**
- `internal/lexer` only; no AST coupling.
- Output contract: `[]Token` + `[]LexError`.

**Acceptance criteria**
- Handles nested blocks and emits correct `INDENT`/`DEDENT` for `req`/`flow` bodies.
- Rejects tab indentation and malformed dedent stacks.
- Tracks balanced delimiters and hook braces (`pre hook { ... }`, `post hook { ... }`).
- Emits accurate source spans for all tokens and diagnostics.

**Sample fixtures (`testdata/lexer/`)**
```text
testdata/lexer/
  valid/
    basic-top-level.pt
    request-with-directives.pt
    hook-block-separators.pt
    flow-with-chain.pt
  invalid/
    tab-indentation.pt
    invalid-dedent.pt
    unterminated-string.pt
    unclosed-brace-in-hook.pt
  golden/
    request-with-directives.tokens.json
    flow-with-chain.tokens.json
```

## Phase 2 — parser + AST

**Scope**
- Build recursive-descent parser for statements/blocks and Pratt parser for expressions.
- Define AST nodes and parser entry points in `internal/ast` and `internal/parser`.

**Module boundaries**
- `internal/parser` depends on `internal/lexer` token contracts.
- `internal/ast` remains parser/runtime agnostic data model.

**Acceptance criteria**
- Parses all top-level forms (`base`, `timeout`, `import`, `let`, `req`, `flow`).
- Enforces request-line forms (`HttpLine`, directives, hooks, asserts, lets).
- Enforces flow shape (prelude lets, exactly one chain line, post-chain assertions).
- Preserves spans on every AST node for later semantic diagnostics.

**Sample fixtures (`testdata/parser/`)**
```text
testdata/parser/
  valid/
    minimal-program.pt
    request-inheritance.pt
    flow-with-aliases.pt
    hook-pre-post.pt
  invalid/
    missing-colon-after-req.pt
    malformed-flow-chain.pt
    hook-missing-rbrace.pt
    bad-expression-precedence.pt
  golden/
    minimal-program.ast.json
    flow-with-aliases.ast.json
```

## Phase 3 — semantic compiler passes and error codes

**Scope**
- Implement compiler pipeline with ordered passes and stable diagnostic codes.
- Build IR/execution plan from validated AST.

**Module boundaries**
- `internal/compiler` consumes `internal/ast` and returns plan + diagnostics.
- No HTTP execution in this phase.

**Acceptance criteria**
- Pass ordering is deterministic and short-circuits only on unrecoverable graph/parser corruption.
- Error codes are documented and stable (e.g., `E100x` import/symbol, `E200x` request semantics, `E300x` flow/vars).
- Produces execution plan with resolved inheritance and flow steps.

**Sample fixtures (`testdata/compiler/`)**
```text
testdata/compiler/
  valid/
    compile-single-flow.pt
    compile-multi-request-inheritance.pt
  invalid/
    duplicate-request-name.pt
    import-cycle-a.pt
    import-cycle-b.pt
    unknown-request-in-flow.pt
    undefined-variable-in-path.pt
  golden/
    compile-single-flow.plan.json
    duplicate-request-name.errors.json
    undefined-variable-in-path.errors.json
```

## Phase 4 — runtime execution engine

**Scope**
- Implement plan executor with request materialization, hook execution, HTTP dispatch, assertion evaluation, and variable/binding propagation.

**Module boundaries**
- `internal/runtime` consumes compiler plan contracts only.
- Optional internal adapter boundary for HTTP client mocking.

**Acceptance criteria**
- Executes flow chain order exactly as compiled.
- Correctly handles pre-hook mutation of `req` and post-hook access to `res`/`$`.
- Persists request-level `let` outputs into subsequent flow scope.
- Emits typed runtime failures (transport, hook eval, assertion failures).

**Sample fixtures (`testdata/runtime/`)**
```text
testdata/runtime/
  fixtures/
    echo-server-responses.json
  valid/
    simple-get-flow.pt
    chained-binding-flow.pt
    post-hook-assertions.pt
  invalid/
    missing-path-var-at-runtime.pt
    failing-assertion.pt
    hook-runtime-error.pt
  golden/
    simple-get-flow.result.json
    failing-assertion.result.json
```

## Phase 5 — reporting + CLI integration

**Scope**
- Integrate `cmd/pipetest` with compiler/runtime.
- Emit human summary + machine reports for CI.

**Module boundaries**
- `cmd/pipetest` orchestrates.
- `internal/report` provides format-specific encoders.

**Acceptance criteria**
- `pipetest eval <file>` compiles and exits non-zero on semantic errors.
- `pipetest run <file>` compiles + executes and exits non-zero on runtime/assertion failures.
- Produces JUnit + JSON artifacts with deterministic naming and per-flow testcase mapping.
- Output works with GitHub Actions and GitLab CI ingestion patterns.

**Sample fixtures (`testdata/report/`, `testdata/cli/`)**
```text
testdata/report/
  golden/
    run-success.junit.xml
    run-success.report.json
    run-failure.junit.xml

testdata/cli/
  cases/
    eval-semantic-error.pt
    run-runtime-failure.pt
  golden/
    eval-semantic-error.stdout.txt
    run-runtime-failure.stdout.txt
```

---

## 3) Grammar-to-parser and semantics cross-reference

| Grammar construct (`grammar.ebnf`) | Parser entry point | Semantic pass name (internal/compiler) |
|---|---|---|
| `Program`, `TopStmt` | `parseProgram`, `parseTopStmt` | `Pass00ImportGraph`, `Pass01Symbols` |
| `SettingStmt` (`base`, `timeout`) | `parseSettingStmt` | `Pass01Symbols` + `Pass03RequestValidity` (global config constraints) |
| `ImportStmt` | `parseImportStmt` | `Pass00ImportGraph` |
| `LetStmt` (global/request/flow prelude/hook) | `parseLetStmt` | `Pass06VariableAvailability` + `Pass04HookRestrictions` |
| `ReqDecl`, `ReqLine` | `parseReqDecl`, `parseReqLine` | `Pass02RequestInheritance`, `Pass03RequestValidity` |
| `HttpLine`, `PathOrUrl` | `parseHttpLine` | `Pass03RequestValidity`, `Pass06VariableAvailability` (path vars) |
| `Directive` (`json`, `header`, `query`, `auth`) | `parseDirective*` | `Pass03RequestValidity`, `Pass06VariableAvailability` |
| `HookBlock`, `HookStmtList`, `AssignStmt`, `LValue` | `parseHookBlock`, `parseHookStmt`, `parseAssignStmt`, `parseLValue` | `Pass04HookRestrictions`, `Pass06VariableAvailability` |
| `FlowDecl`, `FlowPreludeLine`, `FlowChainLine`, `FlowStepRef`, `FlowAssertLine` | `parseFlowDecl`, `parseFlowChainLine`, `parseFlowStepRef` | `Pass05FlowStructureBindings`, `Pass06VariableAvailability` |
| `Expr` family (`OrExpr`..`Primary`, postfix ops) | `parseExpr` (Pratt, bp table) | `Pass06VariableAvailability` + runtime expression checks |

> Suggested compiler pass order: `Pass00ImportGraph -> Pass01Symbols -> Pass02RequestInheritance -> Pass03RequestValidity -> Pass04HookRestrictions -> Pass05FlowStructureBindings -> Pass06VariableAvailability`.

---

## 4) Recommended `testdata/` top-level structure

```text
testdata/
  lexer/
  parser/
  compiler/
  runtime/
  report/
  cli/
  shared/
    imports/
    schemas/
```

- `valid/` and `invalid/` source fixtures for each phase.
- `golden/` expected token/AST/plan/report outputs.
- `shared/imports/` supports import graph/cycle scenarios reused across compiler and CLI tests.
