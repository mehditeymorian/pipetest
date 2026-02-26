# 07 Negative and Resilience

## Goal

Cover expected failure paths and transport/runtime behavior.

## Feature coverage

- explicit 404 expectation
- request timeout pressure case
- flow-level assertions for negative conditions
- runtime diagnostic visibility

## Program

- [programs/07-negative-resilience.pt](programs/07-negative-resilience.pt)

## Commands

```bash
pipetest eval docs/examples/programs/07-negative-resilience.pt
pipetest run docs/examples/programs/07-negative-resilience.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` may fail intentionally in constrained environments (for example timeout/network restrictions), which is useful for validating failure reporting
