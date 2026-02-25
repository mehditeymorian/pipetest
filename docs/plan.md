## PipeTest syntax (v0.5) — now with path params, flow overrides, flow-scoped vars, and per-flow request aliases

This is a **syntax spec**, not a grammar.

---

# 1) Path params in URLs: `/groups/:group_id/...`

### Rule

In the HTTP path, any segment like `:group_id` is replaced at runtime by the **current value of variable `group_id`** (same name), URL-encoded.

```pt
req getGroup:
  GET /groups/:group_id
  ? status == 200
```

If `group_id` is not defined when the request runs → **runtime error** (clear message: “missing variable group_id for path param”).

You can use multiple params:

```pt
req getMember:
  GET /groups/:group_id/members/:member_id
  ? status == 200
```

> You can still keep `${var}` interpolation for headers/body strings; `:param` is specifically for **path segments**.

---

# 2) Variable scoping (flow-first)

### Global variables

Defined at top-level:

```pt
let baseCurrency = "EUR"
let group_id = "g_default"
```

### Flow override variables (must be before the chain)

Flows now allow an **optional prelude** of `let ...` statements before the arrow chain. These override globals **only for that flow**.

```pt
flow "group A scenario":
  let group_id = "g_A"
  let baseCurrency = "USD"

  login -> listOrders -> getGroup

  ? getGroup.res.id == group_id
```

### Variables defined inside requests are flow-scoped

Any `let` inside a request writes into the **current flow scope**, immediately after the request completes—so later requests can use it.

```pt
req login:
  POST /auth/login
  json { email: env("EMAIL"), password: env("PASS") }
  ? status == 200
  let token = $.token    # available to subsequent requests in the same flow
```

If a request assigns a variable that already exists, it **overwrites** it in the flow.

---

# 3) Flow format: chain is `->`, assertions happen after

### Rule

A flow has three zones:

1. **Prelude**: only `let ...` (optional)
2. **Chain line**: exactly one line, with one or more request steps (single step allowed; use `->` for multi-step chains)
3. **Postlude**: assertions (and optionally reporting calls later, if you add them)

Example:

```pt
flow "happy path":
  let group_id = "g_123"

  login -> createOrder -> listOrders

  ? createOrder.status in [200, 201]
  ? listOrders.status == 200
  ? listOrders.res.items contains { id: orderId }
```

Single-step flow is also valid:

```pt
flow "smoke":
  login

  ? login.status == 200
```

---

# 4) Accessing request execution results in a flow

Inside a flow, each executed request becomes an object you can reference:

* `X.res` → **JSON response root** (since all responses are JSON)
* `X.status` → HTTP status
* `X.header["k"]` → response header
* `X.req` → the built request snapshot (method/url/headers/query/body)

Examples:

```pt
? listOrders.res.total > 0
? listOrders.status == 200
? listOrders.req.url contains "/orders"
```

Inside a request block, you still have:

* `$` as the current response JSON root
* `status` as the current HTTP status
* `header[...]` as current response headers

---

# 5) Aliasing requests inside a flow: `listOrders : orders1`

### Rule

In the chain, each step can be:

* the request name alone: `listOrders`
* or aliased: `listOrders : orders1`

That alias becomes the identifier for `.res/.req/.status` access in flow assertions.

```pt
flow "compare two lists":
  login
  -> listOrders : orders1
  -> listOrders : orders2

  ? orders1.status == 200
  ? orders2.status == 200
  ? len(orders1.res.items) >= len(orders2.res.items)
```

**Notes**

* Aliases are **per-flow**, so they don’t change the request definition.
* If you alias, you access results via the alias (`orders1.res...`), not the original request name.
* Alias collisions (same alias used twice) → compile error.

---

# 6) Hooks syntax (optional) — unchanged, but clarified

Hooks live inside a request block and are optional:

```pt
req login:
  POST /auth/login
  json { email: env("EMAIL"), password: env("PASS") }

  pre hook {
    req.header["X-Trace"] = uuid()
  }

  post hook {
    # res and $ are available here
    # you may also set flow vars here if needed
  }

  ? status == 200
  let token = $.token
```

Hook timing per request execution:

1. `pre hook {}` (if present) — can modify `req`, read flow vars
2. send request
3. bind `res`, `$`, `status`, `header[...]`
4. `post hook {}` (if present)
5. request-level `? ...` and `let ...` lines execute in order (as written)

> If you prefer assertions/lets to run before `post hook`, we can flip 4 and 5; the syntax stays the same. Pick one behavior and keep it consistent.

---

# 7) Complete example using **all** features

```pt
base "https://api.acme.com"
timeout 8s

let group_id = "g_default"

req login:
  POST /auth/login
  json { email: env("EMAIL"), password: env("PASS") }

  pre hook {
    req.header["X-Trace"] = uuid()
  }

  ? status == 200
  let token = $.token

req authed:
  auth bearer token
  header Accept = "application/json"

req createOrder(authed):
  POST /groups/:group_id/orders
  json { itemId: 42, qty: 1 }
  ? status in [200, 201]
  let orderId = $.id

req listOrders(authed):
  GET /groups/:group_id/orders
  ? status == 200

flow "group A happy path":
  let group_id = "g_A"         # overrides global group_id for this flow

  login
  -> createOrder
  -> listOrders : orders1

  ? orders1.res.items contains { id: orderId }
  ? createOrder.res.id == orderId
```

---
