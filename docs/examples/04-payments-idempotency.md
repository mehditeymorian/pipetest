# 04 Payments Idempotency

## Goal

Validate retry-safe payment calls that share an idempotency key.

## Feature coverage

- global variables
- `header`, `query`, and `json` directives
- deterministic value checks across two requests
- aliased steps for readable assertions

## Program

- [programs/04-payments-idempotency.pt](programs/04-payments-idempotency.pt)

## Commands

```bash
pipetest eval docs/examples/programs/04-payments-idempotency.pt
pipetest run docs/examples/programs/04-payments-idempotency.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` should pass when both requests echo the same payment identifier
