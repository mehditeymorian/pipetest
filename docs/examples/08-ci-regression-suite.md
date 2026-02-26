# 08 CI Regression Suite

## Goal

Provide a multi-flow entrypoint suitable for CI pipelines.

## Feature coverage

- `import` usage with shared program module
- multi-flow execution in one file
- report artifact generation with `--report-dir`
- scenario suitable for branch and pull-request gates

## Program

- [programs/08-ci-regression-suite.pt](programs/08-ci-regression-suite.pt)
- shared import: [programs/shared/common.pt](programs/shared/common.pt)

## Commands

```bash
pipetest eval docs/examples/programs/08-ci-regression-suite.pt
pipetest run docs/examples/programs/08-ci-regression-suite.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` executes multiple flows and writes JUnit/JSON reports

## CI snippet

```yaml
- name: pipetest regression suite
  run: |
    pipetest eval docs/examples/programs/08-ci-regression-suite.pt
    pipetest run docs/examples/programs/08-ci-regression-suite.pt --report-dir artifacts
```
