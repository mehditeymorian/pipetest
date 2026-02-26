# 06 Logistics Tracking

## Goal

Test shipment tracking endpoints with route variables and templates.

## Feature coverage

- path params (`:shipment_id`, `:stop_id`)
- template interpolation (`{{shipment_id}}`)
- multi-step flow with aliases
- string containment assertion on captured values

## Program

- [programs/06-logistics-tracking.pt](programs/06-logistics-tracking.pt)

## Commands

```bash
pipetest eval docs/examples/programs/06-logistics-tracking.pt
pipetest run docs/examples/programs/06-logistics-tracking.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` should pass when all tracking endpoints return HTTP 200
