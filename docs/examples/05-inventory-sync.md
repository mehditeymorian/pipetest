# 05 Inventory Sync

## Goal

Check consistency between a list endpoint and an item endpoint.

## Feature coverage

- `jsonpath` extraction
- `len` checks on collection responses
- request-level lets reused as path params
- cross-step consistency assertions

## Program

- [programs/05-inventory-sync.pt](programs/05-inventory-sync.pt)

## Commands

```bash
pipetest eval docs/examples/programs/05-inventory-sync.pt
pipetest run docs/examples/programs/05-inventory-sync.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` should confirm that selected list item identifiers match item-detail response identifiers
