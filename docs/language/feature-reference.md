# Feature Reference

Practical reference for every major `pipetest` language feature.

All `.pt` block examples should be tab-indented. For copy/paste-ready programs, use [../examples/README.md](../examples/README.md).

## Table of Contents

- [Settings](#settings)
- [Imports](#imports)
- [Variables](#variables)
- [Requests](#requests)
- [Request inheritance](#request-inheritance)
- [Directives](#directives)
- [Hooks](#hooks)
- [Assertions](#assertions)
- [Flows and aliases](#flows-and-aliases)
- [Path params and templates](#path-params-and-templates)
- [Built-in functions](#built-in-functions)

## Settings

```pt
base "https://api.example.com"
timeout 8s
```

## Imports

```pt
import "./shared/common.pt"
```

Imports are resolved relative to the current file directory.

## Variables

Global variables:

```pt
let tenant_id = "tenant-42"
```

Flow prelude overrides:

```pt
flow "tenant-a":
  let tenant_id = "tenant-a"
  getTenant
```

Request-level lets write into the current flow scope after the request executes.

## Requests

```pt
req listUsers:
  GET /users
  ? status == 200
```

Request lines can include directives, hooks, assertions, and lets.

## Request inheritance

```pt
req parent:
  GET /v1/users
  header Accept = "application/json"

req child(parent):
  GET /v2/users
  ? status == 200
```

Inheritance is merged before validation/execution.

## Directives

### `json`

```pt
json { id: 1, active: true }
```

### `header`

```pt
header X-Trace = uuid()
```

### `query`

```pt
query page = 2
```

### `auth bearer`

```pt
auth bearer token
```

## Hooks

```pt
pre hook {
  req.header["X-Trace"] = uuid()
}

post hook {
  println "status={{status}}"
}
```

Hook template variables:
- `{{req}}` in pre/post hooks
- `{{status}}`, `{{res}}` in post hooks only

## Assertions

Request-level and flow-level assertions use `?`.

```pt
? status == 200
? checkout.status in [200, 201]
```

## Flows and aliases

```pt
flow "checkout":
  login -> createOrder -> listOrders:orders
  ? orders.status == 200
```

Aliases are local to the flow.

## Path params and templates

Path params use `:name` and resolve from variables at runtime:

```pt
GET /groups/:group_id/orders/:order_id
```

String templates use `{{name}}`:

```pt
GET /audit/{{group_id}}/{{order_id}}
```

## Built-in functions

Common built-ins:
- `env("NAME")`
- `uuid()`
- `len(x)`
- `regex(pattern, value)`
- `jsonpath(value, "$.a[0]")`
- `now()`
- `urlencode(value)`

See runtime semantics in [execution-model.md](execution-model.md).
