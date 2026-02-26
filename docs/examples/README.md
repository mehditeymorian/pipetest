# Examples

This catalog provides runnable and product-oriented `pipetest` scenarios.

## Table of Contents

- [How to use these examples](#how-to-use-these-examples)
- [Scenario matrix](#scenario-matrix)
- [Programs directory](#programs-directory)
- [Quick commands](#quick-commands)

## How to use these examples

1. Pick a scenario page from the matrix.
2. Run `pipetest eval` on the matching program.
3. Run `pipetest run` to execute flows and generate reports.
4. Adapt endpoints, headers, and assertions for your API.

## Scenario matrix

| ID | Scenario | Product angle | Feature focus | Docs | Program |
|---|---|---|---|---|---|
| 01 | Quickstart Smoke | Generic API health | `base`, `timeout`, `req`, `flow`, assertions | [01](01-quickstart-smoke.md) | [program](programs/01-quickstart-smoke.pt) |
| 02 | SaaS Authentication | Login/session verification | flow-scoped `let`, `auth bearer`, hooks | [02](02-saas-authentication.md) | [program](programs/02-saas-authentication.pt) |
| 03 | E-commerce Checkout | Product to cart flow | chain assertions, aliases, response fields | [03](03-ecommerce-checkout.md) | [program](programs/03-ecommerce-checkout.pt) |
| 04 | Payments Idempotency | Retry-safe payment calls | headers, query, JSON body, deterministic checks | [04](04-payments-idempotency.md) | [program](programs/04-payments-idempotency.pt) |
| 05 | Inventory Sync | Cross-endpoint consistency | `jsonpath`, `len`, path params from prior step | [05](05-inventory-sync.md) | [program](programs/05-inventory-sync.pt) |
| 06 | Logistics Tracking | Shipment lifecycle | `:path_params`, templates, aliases | [06](06-logistics-tracking.md) | [program](programs/06-logistics-tracking.pt) |
| 07 | Negative and Resilience | Failure-path coverage | 404 assertions, timeout behavior, runtime diagnostics | [07](07-negative-resilience.md) | [program](programs/07-negative-resilience.pt) |
| 08 | CI Regression Suite | Team pipeline entrypoint | imports, multi-flow execution, report-dir usage | [08](08-ci-regression-suite.md) | [program](programs/08-ci-regression-suite.pt) |

## Programs directory

- Shared imports: [programs/shared/common.pt](programs/shared/common.pt)
- Scenario programs: [programs/](programs/)

## Quick commands

Evaluate one scenario:

```bash
pipetest eval docs/examples/programs/01-quickstart-smoke.pt
```

Run one scenario:

```bash
pipetest run docs/examples/programs/01-quickstart-smoke.pt --report-dir ./pipetest-report
```

Evaluate all scenario programs:

```bash
for file in docs/examples/programs/*.pt; do
  pipetest eval "$file"
done
```
