# 01 Quickstart Smoke

## Goal

Validate that a basic API endpoint responds with HTTP 200.

## Feature coverage

- `base` and `timeout`
- single request declaration
- single-step flow
- request and flow assertions

## Program

- [programs/01-quickstart-smoke.pt](programs/01-quickstart-smoke.pt)

## Commands

```bash
pipetest eval docs/examples/programs/01-quickstart-smoke.pt
pipetest run docs/examples/programs/01-quickstart-smoke.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` exits `0` when `https://httpbin.org/get` is reachable and returns 200
