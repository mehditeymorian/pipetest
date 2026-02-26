# 03 E-commerce Checkout

## Goal

Test a checkout-like sequence from product read to cart retrieval.

## Feature coverage

- multi-step flow chaining
- response-derived flow variables
- path params from previous step values
- alias-based flow assertions

## Program

- [programs/03-ecommerce-checkout.pt](programs/03-ecommerce-checkout.pt)

## Commands

```bash
pipetest eval docs/examples/programs/03-ecommerce-checkout.pt
pipetest run docs/examples/programs/03-ecommerce-checkout.pt --report-dir ./pipetest-report
```

## Expected behavior

- `eval` exits `0`
- `run` should execute product lookup, cart creation, and cart retrieval in order
