# Documentation

This folder contains the official reference for the `pipetest` CLI and DSL.

## Table of Contents

- [Read in this order](#read-in-this-order)
- [Language documentation](#language-documentation)
- [Examples](#examples)
- [CLI and runtime references](#cli-and-runtime-references)
- [Architecture and implementation](#architecture-and-implementation)
- [Legacy compatibility](#legacy-compatibility)

## Read in this order

1. [README](../README.md) for installation and quick start.
2. [Language docs](language/README.md) to learn the DSL.
3. [Examples](examples/README.md) to copy working scenarios.
4. [CLI spec](cli-spec.md) and [reporting](reporting.md) for CI details.

## Language documentation

- [Language index](language/README.md)
- [Language specification](language/specification.md)
- [Feature reference](language/feature-reference.md)
- [Execution model](language/execution-model.md)
- [Grammar (authoritative)](../grammar.ebnf)

## Examples

- [Examples index](examples/README.md)
- [Quickstart smoke](examples/01-quickstart-smoke.md)
- [SaaS authentication](examples/02-saas-authentication.md)
- [E-commerce checkout](examples/03-ecommerce-checkout.md)
- [Payments idempotency](examples/04-payments-idempotency.md)
- [Inventory sync](examples/05-inventory-sync.md)
- [Logistics tracking](examples/06-logistics-tracking.md)
- [Negative and resilience](examples/07-negative-resilience.md)
- [CI regression suite](examples/08-ci-regression-suite.md)

## CLI and runtime references

- [CLI specification](cli-spec.md)
- [Diagnostics](diagnostics.md)
- [Reporting](reporting.md)
- [Rule sets](rule-sets.md)
- [Language constraints](language-constraints.md)

## Architecture and implementation

- [Implementation roadmap](implementation-roadmap.md)
- [Parser notes](parser.md)
- [Plan notes](plan.md)
- [Task tracker](task.md)
- [MVP tasks](tasks/)

## Legacy compatibility

- [Legacy language guide path](language-guide.md)
  - This file is kept as a compatibility page and now points to the new `docs/language/` pages.
