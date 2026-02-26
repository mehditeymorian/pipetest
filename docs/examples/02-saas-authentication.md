# 02 SaaS Authentication

## Goal

Model a login and authenticated follow-up request in a SaaS-style workflow.

## Feature coverage

- request-level `let` for token capture
- `auth bearer` directive
- request hooks
- flow alias and post-flow variable assertion

## Program

- [programs/02-saas-authentication.pt](programs/02-saas-authentication.pt)

## Commands

```bash
pipetest eval docs/examples/programs/02-saas-authentication.pt
pipetest run docs/examples/programs/02-saas-authentication.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` should show the login then authenticated request chain and pass assertions
