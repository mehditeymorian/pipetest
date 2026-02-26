# Language Documentation

The `pipetest` language is a small DSL for request definitions, multi-step flows, assertions, and CI reporting.

## Table of Contents

- [Learning path](#learning-path)
- [Language pages](#language-pages)
- [Quick syntax snapshot](#quick-syntax-snapshot)
- [Related docs](#related-docs)

## Learning path

1. Read the [specification](specification.md) for grammar-level structure and constraints.
2. Use the [feature reference](feature-reference.md) while writing `.pt` files.
3. Review the [execution model](execution-model.md) to understand scope, hooks, and runtime behavior.
4. Copy and adapt scenarios from [examples](../examples/README.md).

## Language pages

- [Specification](specification.md)
- [Feature reference](feature-reference.md)
- [Execution model](execution-model.md)

## Quick syntax snapshot

```pt
base "https://httpbin.org"
timeout 5s

req ping:
	GET /get
	? status == 200

flow "smoke":
	ping
	? ping.status == 200
```

Block indentation in `.pt` files is tab-based.

## Related docs

- [Grammar](../../grammar.ebnf)
- [CLI spec](../cli-spec.md)
- [Diagnostics](../diagnostics.md)
- [Examples](../examples/README.md)
