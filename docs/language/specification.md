# Language Specification

This document describes the implemented `pipetest` DSL structure and rule set.

## Table of Contents

- [Source of truth](#source-of-truth)
- [Program structure](#program-structure)
- [Top-level statements](#top-level-statements)
- [Request declarations](#request-declarations)
- [Flow declarations](#flow-declarations)
- [Expressions](#expressions)
- [Lexical and layout rules](#lexical-and-layout-rules)
- [Semantic constraints](#semantic-constraints)
- [Deterministic behavior requirements](#deterministic-behavior-requirements)

## Source of truth

- Authoritative grammar: [grammar.ebnf](../../grammar.ebnf)
- Additional constraints: [../language-constraints.md](../language-constraints.md)

## Program structure

A program is an ordered sequence of top-level statements:

- `base "..."`
- `timeout <duration>`
- `import "..."`
- `let name = expr`
- `req Name:`
- `flow "name":`

## Top-level statements

### `base`

Sets the default base URL for non-absolute request targets.

### `timeout`

Sets default runtime timeout for request execution.

### `import`

Loads another `.pt` module using a path relative to the importing file.

### `let`

Defines a global variable available to flows and request evaluation.

## Request declarations

Shape:

```pt
req requestName:
  <request lines>
```

Supported request lines:
- one HTTP line: `GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS <path-or-url>`
- directives: `json`, `header`, `query`, `auth bearer`
- hooks: `pre hook { ... }`, `post hook { ... }`
- assertions: `? expr`
- request-level lets: `let name = expr`

Inheritance:

```pt
req child(parent):
  GET /v2/resource
```

Merged request lines are computed parent-first, then child overrides/extends by rule. Each request must have exactly one effective HTTP line.

## Flow declarations

Shape:

```pt
flow "name":
  let flow_var = "x"    # optional prelude lets
  reqA -> reqB:alias
  ? alias.status == 200
```

Rules:
- flow prelude can contain only `let` statements
- exactly one chain line is required
- chain can be single-step or `->` multi-step
- post-chain lines can only be assertions
- aliases are optional but must be unique per flow

## Expressions

Expression support includes:
- logical: `and`, `or`, `not`
- comparisons: `==`, `!=`, `<`, `<=`, `>`, `>=`
- membership: `in`, `contains`, `~`
- arithmetic: `+`, `-`, `*`, `/`, `%`
- field/index/call chaining: `obj.key`, `arr[0]`, `fn(x)`
- literals: string, number, bool, null, array, object

Special symbols by context:
- request scope: `status`, `header[...]`, `#`, `res`, `req`
- flow scope: `<binding>.status`, `<binding>.res`, `<binding>.req`

## Lexical and layout rules

- indentation-sensitive blocks (`INDENT` / `DEDENT`) are used for `req` and `flow`
- tab indentation is required for block indentation
- space indentation at line start in blocks is rejected
- hook blocks are brace-scoped and support statement separators
- comments begin with `#` outside of string literals
- `PATH` tokens support both absolute URLs and relative paths

## Semantic constraints

Key enforced constraints:
- imports must be acyclic
- imported files cannot declare flows
- request names must be unique
- flow names must be unique in entry file
- each request must include exactly one HTTP line
- duplicate pre/post hooks are forbidden
- multiple body directives are forbidden
- pre hook cannot reference response-only symbols
- assignment to `res` is forbidden
- flow bindings and required variables must be resolvable

## Deterministic behavior requirements

- diagnostics are sorted and deduplicated before output
- compilation succeeds only with zero static diagnostics
- runtime diagnostics are stable by flow/request context
- report artifact names are deterministic for `run`
