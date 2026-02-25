# pipetest

`pipetest` is an open-source CLI and DSL for API testing.

It lets you define API requests, connect them into multi-step flows, add assertions, and run everything in CI with JUnit/JSON reports.

## Why `pipetest`

- **Language-first API tests**: define reusable requests and scenario flows in `.pt` files.
- **Static validation before execution**: catch parser, import, and semantic errors with `pipetest eval`.
- **Flow execution with diagnostics**: run all flows (or one request) and get deterministic output.
- **CI-ready artifacts**: emits JUnit XML + JSON reports compatible with GitHub Actions and GitLab CI.

## Installation

### Option 1: Install from source with Go

Requirements:

- Go 1.24+

Install:

```bash
go install github.com/mehditeymorian/pipetest/cmd/pipetest@latest
```

Then verify:

```bash
pipetest --help
```

### Option 2: Build locally from this repository

```bash
git clone https://github.com/mehditeymorian/pipetest.git
cd pipetest
go build ./cmd/pipetest
./pipetest --help
```

### Option 3: Pull container image from GitHub Container Registry

Use the published GHCR image:

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

Run static checks only:

```bash
pipetest eval program.pt
```

Run execution + reports:

```bash
pipetest run program.pt --report-dir ./pipetest-report
```

## CLI commands

`pipetest` currently supports three commands:

- `pipetest eval <program.pt>`
- `pipetest run <program.pt>`
- `pipetest request <program.pt> <request-name>`

### `pipetest eval`

Performs non-runtime validation only:

- syntax and parser checks
- import resolution and cycle checks
- semantic validation

Exit codes:

- `0` = no diagnostics
- `1` = static diagnostics found
- `2` = CLI usage error

### `pipetest run`

Compiles and executes all flows in the program:

- runs the same checks as `eval` first
- executes request chains and assertions
- writes report artifacts to `--report-dir` (default `./pipetest-report`)

Exit codes:

- `0` = all flows successful
- `1` = compile/runtime/assertion failures
- `2` = CLI usage error

Artifacts written by `run`:

- `pipetest-junit.xml`
- `pipetest-report.xml` (legacy compatibility alias)
- `pipetest-report.json`

### `pipetest request`

Compiles the program, selects one named request, and executes only that request as a synthetic one-step flow.

Exit codes:

- `0` = request run successful
- `1` = compile/runtime/assertion failures
- `2` = CLI usage error (including unknown request name)

### Common flags

- `--format pretty|json` (all commands)
- `--timeout <duration>` (`run`, `request`)
- `--verbose` (`run`, `request`)
- `--hide-passing-assertions` (`run`, `request`)
- `--report-dir <dir>` (`run` only)

## Language overview

A `.pt` file typically includes:

- settings (`base`, `timeout`)
- imports (`import "other.pt"`)
- variables (`let x = ...`)
- request declarations (`req Name:`)
- flow declarations (`flow "name":`)

Key features:

- request inheritance (`req child(parent):`)
- path parameters (`/groups/:group_id`)
- optional `pre hook {}` and `post hook {}` blocks in requests
- request-level and flow-level assertions (`? expr`)
- flow step aliases (`reqName:alias`)

For a complete language/usage reference, see:

- [`docs/language-guide.md`](docs/language-guide.md)
- [`grammar.ebnf`](grammar.ebnf)

## CI integration

For complete GitHub Actions and GitLab CI examples, see:

- [`docs/language-guide.md#ci-integration`](docs/language-guide.md#ci-integration)
- [`docs/reporting.md`](docs/reporting.md)

This repository includes two GitHub Actions workflows:

- `.github/workflows/ci.yml`
  - runs `go test ./...`
  - runs `go build ./...`
  - runs `golangci-lint run ./...`
  - triggers on pull requests and all branch pushes
- `.github/workflows/container-image.yml`
  - builds a Docker image from `Dockerfile`
  - smoke-tests the image on pull requests
  - publishes tagged images to `ghcr.io/<owner>/pipetest` on `main` and version tags

## Development

Repository verification commands:

```bash
gofmt -w .
golangci-lint run ./...
go test ./...
go build ./...
```

## Documentation index

- `grammar.ebnf` — authoritative language grammar
- `docs/language-guide.md` — complete language and usage guide (installation, examples, CI)
- `docs/plan.md` — product-level behavior and examples
- `docs/language-constraints.md` — language constraints and non-goals
- `docs/cli-spec.md` — detailed command behavior
- `docs/diagnostics.md` — diagnostics taxonomy and output rules
- `docs/reporting.md` — report mapping and CI artifact strategy
- `docs/rule-sets.md` — lexer/semantic validation rules
- `docs/implementation-roadmap.md` — implementation milestones

## Implementation workflow (doc-first)

To keep implementation aligned with project specs, this repo includes:

- `AGENTS.md` — operating rules for doc-first, milestone-based implementation
- `.codex/config.toml` — repo-scoped Codex execution defaults
- `EXECPLAN.template.md` — template used to create milestone plans from `/docs`
- `QUESTIONS.md` — capture ambiguities instead of guessing behavior
