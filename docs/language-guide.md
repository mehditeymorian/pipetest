# Language and usage guide

This guide documents how to install `pipetest`, write `.pt` programs, run checks/execution, and integrate the tool into CI pipelines.

## 1) Installation

## Prerequisites

- Go 1.24+

### Install globally with `go install`

```bash
go install github.com/mehditeymorian/pipetest/cmd/pipetest@latest
```

Confirm installation:

```bash
pipetest --help
```

### Build from repository source

```bash
git clone https://github.com/mehditeymorian/pipetest.git
cd pipetest
go build ./cmd/pipetest
./pipetest --help
```

## 2) CLI usage

`pipetest` provides three subcommands.

## `pipetest eval <program.pt>`

Runs static checks only:

- lexer/parser validation
- import loading/cycle checks
- compiler semantic checks

Does **not** execute HTTP requests.

Exit codes:

- `0`: no errors
- `1`: parse/import/semantic errors
- `2`: invalid CLI usage

Example:

```bash
pipetest eval tests/api.pt
```

## `pipetest run <program.pt>`

Compiles then executes all flows.

Responsibilities:

- performs all `eval` checks first
- executes request chains in each flow
- evaluates request and flow assertions
- writes report artifacts

Exit codes:

- `0`: all flows and assertions passed
- `1`: compile/runtime/assertion failure
- `2`: invalid CLI usage

Example:

```bash
pipetest run tests/api.pt --report-dir ./artifacts --verbose
```

Generated report files:

- `pipetest-junit.xml`
- `pipetest-report.xml` (legacy alias)
- `pipetest-report.json`

## `pipetest request <program.pt> <request-name>`

Compiles the program and executes exactly one request.

Useful for:

- debugging a single request quickly
- validating request-level assertions/hooks in isolation

Example:

```bash
pipetest request tests/api.pt login --timeout 5s
```

### Supported flags

Global/common flags:

- `--format pretty|json` (default: `pretty`)

Run/request execution flags:

- `--timeout <duration>`
- `--verbose`
- `--hide-passing-assertions`

Run-only flags:

- `--report-dir <dir>` (default: `./pipetest-report`)

## 3) Language reference

The authoritative grammar is in [`grammar.ebnf`](../grammar.ebnf). This section explains practical usage.

## Program structure

A program is a sequence of top-level statements:

- `base "..."`
- `timeout 5s`
- `import "other.pt"`
- `let name = expr`
- `req <name> ...`
- `flow "name" ...`

## Settings

```pt
base "https://api.example.com"
timeout 8s
```

- `base`: default base URL
- `timeout`: default request timeout

## Imports

```pt
import "./common/auth.pt"
```

Imports are resolved recursively from the importing file's directory.

## Variables and scoping

Top-level `let` variables are global defaults.

Flow-level `let` variables (in flow prelude) override globals for that flow.

Request-level `let` statements write values into the active flow scope, so later requests in the same flow can use them.

```pt
let group_id = "default"

req login:
  POST /auth/login
  let token = #.token

flow "scenario":
  let group_id = "g_1"
  login
```

## Request declarations

Basic request:

```pt
req listUsers:
  GET /users
  ? status == 200
```

HTTP lines accept full URLs and relative targets:

```pt
base "https://api.example.com"

req viaBase:
  GET users

req absolute:
  GET https://other.example.com/health
```

- full `http://...` / `https://...` URLs are used as-is
- non-absolute targets are combined with `base`

With inheritance:

```pt
req authed:
  auth bearer token
  header Accept = "application/json"

req listUsers(authed):
  GET /users
```

### Request line types

Inside a `req` block you can use:

- one HTTP line (required)
- directives (`json`, `header`, `query`, `auth bearer`)
- `pre hook { ... }` and `post hook { ... }`
- assertions (`? expr`)
- `let` statements

### Hooks

```pt
req login:
  POST /auth/login

  pre hook {
    req.header["X-Trace"] = uuid()
  }

  post hook {
    print("status", status)
  }
```

## Flow declarations

Flow shape:

1. optional prelude (`let ...` only)
2. exactly one chain line
3. optional postlude assertions (`? ...`)

```pt
flow "checkout":
  let group_id = "g_A"

  login -> createOrder -> listOrders:orders

  ? createOrder.status in [200, 201]
  ? orders.res.total > 0
```

Single-step flows are valid:

```pt
flow "smoke":
  login
```

## Assertions and expressions

Assertions use `? <expr>`. Common operators include:

- comparison: `==`, `!=`, `<`, `<=`, `>`, `>=`
- logical: `and`, `or`, `not`
- membership/containment: `in`, `contains`
- arithmetic: `+`, `-`, `*`, `/`, `%`

Context objects:

- in request scope: `status`, `header[...]`, `$`, `#`
- in flow scope: `<step>.status`, `<step>.res`, `<step>.req`, `<step>.header[...]`

Utility functions (callable in expressions/hooks):

- `env("NAME")`: reads an environment variable
- `uuid()`: generates a random 32-char hex identifier
- `len(x)`: length of arrays/objects/strings
- `regex(pattern, value)`: returns whether `value` matches `pattern`
- `jsonpath(value, "$.path[0]")`: reads JSON fields/indices from a value
- `now()`: current UTC timestamp (`RFC3339Nano`)
- `urlencode(value)`: URL-encodes a value for query/path usage

## Path params in URLs

Named path segments like `:group_id` are substituted from variables at execution time:

```pt
req getGroup:
  GET /groups/:group_id
```

If the variable is missing, runtime emits a deterministic error.

## 4) End-to-end example

```pt
base "https://api.acme.com"
timeout 8s

let group_id = "g_default"

req login:
  POST /auth/login
  json { email: env("EMAIL"), password: env("PASS") }
  ? status == 200
  let token = #.token

req authed:
  auth bearer token
  header Accept = "application/json"

req createOrder(authed):
  POST /groups/:group_id/orders
  json { itemId: 42, qty: 1 }
  ? status in [200, 201]
  let orderId = #.id

req listOrders(authed):
  GET /groups/:group_id/orders
  ? status == 200

flow "group A happy path":
  let group_id = "g_A"

  login -> createOrder -> listOrders:orders

  ? orders.res.items contains { id: orderId }
  ? createOrder.res.id == orderId
```

Run:

```bash
pipetest eval program.pt
pipetest run program.pt --report-dir ./artifacts
```

## 5) CI integration

## GitHub Actions

```yaml
name: pipetest

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.24.x"

      - name: Install pipetest
        run: go install github.com/mehditeymorian/pipetest/cmd/pipetest@latest

      - name: Evaluate program
        run: pipetest eval tests/api.pt

      - name: Run pipetest
        if: always()
        run: pipetest run tests/api.pt --report-dir artifacts

      - name: Upload reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: pipetest-reports
          path: |
            artifacts/pipetest-junit.xml
            artifacts/pipetest-report.xml
            artifacts/pipetest-report.json

      - name: Publish JUnit summary
        if: always()
        uses: mikepenz/action-junit-report@v5
        with:
          report_paths: artifacts/pipetest-junit.xml
```

## GitLab CI

```yaml
stages:
  - test

pipetest:
  stage: test
  image: golang:1.24
  before_script:
    - go install github.com/mehditeymorian/pipetest/cmd/pipetest@latest
  script:
    - pipetest eval tests/api.pt
    - pipetest run tests/api.pt --report-dir artifacts
  artifacts:
    when: always
    paths:
      - artifacts/pipetest-junit.xml
      - artifacts/pipetest-report.xml
      - artifacts/pipetest-report.json
    reports:
      junit: artifacts/pipetest-junit.xml
```

## CI recommendations

- Always upload reports with `if: always()` / `when: always`.
- Keep `eval` before `run` to fail fast on static errors.
- Keep report paths stable so downstream tooling is deterministic.

## 6) Related docs

- CLI details: [`docs/cli-spec.md`](cli-spec.md)
- Diagnostics structure: [`docs/diagnostics.md`](diagnostics.md)
- Report mapping model: [`docs/reporting.md`](reporting.md)
- Grammar: [`grammar.ebnf`](../grammar.ebnf)
