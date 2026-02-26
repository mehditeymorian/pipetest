# Execution Model

This document explains how `pipetest` evaluates variables, requests, hooks, flows, and runtime diagnostics.

## Table of Contents

- [Compile then execute](#compile-then-execute)
- [Scope and variable lifecycle](#scope-and-variable-lifecycle)
- [Request lifecycle](#request-lifecycle)
- [Flow bindings and aliases](#flow-bindings-and-aliases)
- [Hook restrictions](#hook-restrictions)
- [Assertions](#assertions)
- [Runtime diagnostics](#runtime-diagnostics)
- [Report artifacts](#report-artifacts)

## Compile then execute

`pipetest run` and `pipetest request` always compile first:
1. load and parse modules
2. resolve imports and symbols
3. validate semantics
4. execute only if compilation has no diagnostics

## Scope and variable lifecycle

Variable availability order in each flow:
1. global `let`
2. flow prelude `let` overrides
3. request-level `let` from completed prior steps

A request can only use variables that are already defined at that step in that flow.

## Request lifecycle

For each flow step:
1. materialize path/directives/templates from current variables
2. run `pre hook` (if present)
3. dispatch HTTP request
4. bind response context (`status`, `res`, `#`, `header[...]`)
5. run `post hook` (if present)
6. evaluate request assertions and request lets in source order

## Flow bindings and aliases

Each step binds a name for flow assertions:
- default binding: request name
- alias binding: `requestName:alias`

Flow assertions can read:
- `<binding>.status`
- `<binding>.res`
- `<binding>.req`

## Hook restrictions

Semantic restrictions include:
- `pre hook` cannot reference response-only values
- assignment to `res` is forbidden
- duplicate pre/post hooks per request are invalid

## Assertions

Assertion result behavior:
- `true`: pass
- `false`: assertion diagnostic
- expression failure: runtime expression diagnostic

`--hide-passing-assertions` suppresses successful assertion lines from pretty output.

## Runtime diagnostics

Runtime failures are surfaced with stable codes (for example transport, hook, expression, missing variable, missing path param, assertion).

Diagnostics may include flow/request context fields for CI and machine processing.

See [../diagnostics.md](../diagnostics.md) for code taxonomy and output shape.

## Report artifacts

`pipetest run` writes:
- `pipetest-junit.xml`
- `pipetest-report.xml`
- `pipetest-report.json`

See [../reporting.md](../reporting.md) for mapping details.
