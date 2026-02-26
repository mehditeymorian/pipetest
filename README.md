# pipetest

`pipetest` is an open-source CLI and DSL for API testing.

It is designed for simple onboarding and professional CI usage:
- write API tests in `.pt` files
- validate statically before execution
- execute flow-based scenarios
- publish JUnit/JSON reports for pipelines

## Table of Contents

- [Why pipetest](#why-pipetest)
- [Installation](#installation)
- [Quick start](#quick-start)
- [CLI usage](#cli-usage)
- [Language and examples](#language-and-examples)
- [Documentation map](#documentation-map)
- [Examples by scenario](#examples-by-scenario)
- [CI integration](#ci-integration)
- [Development](#development)
- [Implementation workflow](#implementation-workflow)

## Why pipetest

- Language-first API tests with reusable requests and scenario flows.
- Static validation (`eval`) before runtime.
- Deterministic diagnostics and assertion reporting.
- CI-ready artifacts (`pipetest-junit.xml`, `pipetest-report.xml`, `pipetest-report.json`).

## Installation

### Option 1: Install from source

Requirements:
- Go `1.24+`

```bash
go install github.com/mehditeymorian/pipetest/cmd/pipetest@latest
pipetest --help
```

### Option 2: Build locally

```bash
git clone https://github.com/mehditeymorian/pipetest.git
cd pipetest
go build ./cmd/pipetest
./pipetest --help
```

### Option 3: Run container image

```bash
docker run --rm ghcr.io/mehditeymorian/pipetest:latest --help
```

## Quick start

Create `program.pt`:

```pt
base "https://httpbin.org"
timeout 5s

req ping:
	GET /get
	? status == 200

flow "smoke":
	ping
	? ping.status == 200
```

Important: `pipetest` request/flow blocks use tab indentation.

Run static checks:

```bash
pipetest eval program.pt
```

Run execution and write reports:

```bash
pipetest run program.pt --report-dir ./pipetest-report
```

## CLI usage

`pipetest` commands:
- `pipetest eval <program.pt>`
- `pipetest run <program.pt>`
- `pipetest request <program.pt> <request-name>`

### Exit codes

- `0`: success
- `1`: compile/runtime/assertion diagnostics
- `2`: CLI usage error

### Common flags

- `--format pretty|json` (all commands)
- `--timeout <duration>` (`run`, `request`)
- `--verbose` (`run`, `request`)
- `--hide-passing-assertions` (`run`, `request`)
- `--report-dir <dir>` (`run` only)

For complete command behavior, see [docs/cli-spec.md](docs/cli-spec.md).

## Language and examples

Start here:
- [Language documentation](docs/language/README.md)
- [Examples documentation](docs/examples/README.md)

Authoritative syntax grammar:
- [grammar.ebnf](grammar.ebnf)

Compatibility path retained:
- [docs/language-guide.md](docs/language-guide.md)

## Documentation map

- [docs/README.md](docs/README.md) - top-level docs index
- [docs/language/specification.md](docs/language/specification.md) - DSL structure and rules
- [docs/language/feature-reference.md](docs/language/feature-reference.md) - feature-by-feature usage
- [docs/language/execution-model.md](docs/language/execution-model.md) - scope, hooks, bindings, errors
- [docs/examples/README.md](docs/examples/README.md) - scenario index and run guide
- [docs/diagnostics.md](docs/diagnostics.md) - diagnostic model and output shape
- [docs/reporting.md](docs/reporting.md) - report artifacts and CI mapping

## Examples by scenario

- [01 Quickstart Smoke](docs/examples/01-quickstart-smoke.md)
- [02 SaaS Authentication](docs/examples/02-saas-authentication.md)
- [03 E-commerce Checkout](docs/examples/03-ecommerce-checkout.md)
- [04 Payments Idempotency](docs/examples/04-payments-idempotency.md)
- [05 Inventory Sync](docs/examples/05-inventory-sync.md)
- [06 Logistics Tracking](docs/examples/06-logistics-tracking.md)
- [07 Negative and Resilience](docs/examples/07-negative-resilience.md)
- [08 CI Regression Suite](docs/examples/08-ci-regression-suite.md)

## CI integration

Use `pipetest run` and upload generated artifacts:
- `pipetest-junit.xml`
- `pipetest-report.xml`
- `pipetest-report.json`

See:
- [docs/cli-spec.md](docs/cli-spec.md)
- [docs/reporting.md](docs/reporting.md)
- [docs/examples/08-ci-regression-suite.md](docs/examples/08-ci-regression-suite.md)

## Development

Repository verification commands:

```bash
gofmt -w .
golangci-lint run ./...
go test ./...
go build ./...
```

## Implementation workflow

- [AGENTS.md](AGENTS.md) - operating rules for doc-first implementation
- [.codex/config.toml](.codex/config.toml) - repo-scoped Codex defaults
- [EXECPLAN.template.md](EXECPLAN.template.md) - milestone planning template
- [QUESTIONS.md](QUESTIONS.md) - ambiguity capture
