Below is a **complete, practical rule-set** you can implement for (A) the **lexer pre-pass** and (B) **semantic checks** (post-parse). I’m assuming your parser already builds an AST using the grammar you finalized.

---

## A) Lexer pre-pass rule-set

Your lexer will be simpler and more reliable if you split it into two layers:

1. **Pre-pass**: normalize text, strip comments safely, produce `NL`, `INDENT`, `DEDENT`, and handle **hook braces** correctly.
2. **Tokenizer**: turn characters into tokens (`IDENT`, `STRING`, `PATH`, `{`, `}`, `?`, `->`, etc.).

### A1) Text normalization

Rules:

* Normalize line endings: convert `\r\n` and `\r` → `\n`.
* Reject **TAB** characters anywhere in leading indentation (recommended: reject tabs anywhere, period).
* Keep original offsets/line/col for error reporting.

### A2) Comment stripping

Rule:

* `#` starts a comment **only when not inside a string literal**.
* Comment runs to the end of line.
* Preserve the newline (`\n`) itself.

Strings supported:

* `"`…`"` with escapes (`\"`, `\\`, `\n`, `\t`, `\uXXXX` if you want)
* raw strings using backticks `` `...` `` (no escapes)

### A3) Logical line model + `NL`

Emit a `NL` token at the end of each logical line that is not:

* inside a string literal
* inside an *expression grouping* that you decide should allow multiline (more below)

**Recommendation (keeps the language ergonomic):**

* Allow multiline **object/array literals** and parentheses expressions.
* But keep the indentation block structure deterministic.

So:

* Track `parenDepth` for `(` `)`
* Track `bracketDepth` for `[` `]`
* Track `braceDepthExpr` for `{` `}` that belong to **object literals** (not hooks)
* Track `braceDepthHook` for `{` `}` that belong to **hook bodies**

Then treat newlines as:

* **Significant** (`NL`) when: `parenDepth==0 && bracketDepth==0 && braceDepthExpr==0 && braceDepthHook==0`
* **Significant** inside **hook bodies** too (you want statements separated by newlines), so:

  * inside hook body: `braceDepthHook>0` → still emit `NL`
  * but suppress `INDENT/DEDENT` generation there (hooks are brace scoped, not indent scoped)

That means: only suppress `NL` when you’re mid-expression (paren/bracket/expr-brace), not when you’re in hook braces.

### A4) Detecting “hook braces” vs “expression braces”

You need to tag `{` as either:

* `{HOOK}` opening a hook body
* `{EXPR}` opening an object literal (JSON-ish object)

**Rule to classify `{` as HOOK:**

* If the lexer sees the token sequence:

  * `pre` `hook` `{`  OR  `post` `hook` `{`
  * (allow arbitrary whitespace and newlines between `pre` and `hook`? I’d restrict to same line for clarity)
    then that `{` increments `braceDepthHook`.

Otherwise `{` increments `braceDepthExpr`.

This keeps:

* multiline `json { ... }` workable (`{EXPR}`)
* hook bodies parsed as statement lists (`{HOOK}`)

### A5) Indentation scanning → `INDENT` / `DEDENT`

Use Python-style indentation, but only for blocks introduced by `:` (req/flow).

**Block open detection**

* When a line ends with `:` (outside strings, outside expr contexts), mark `expectIndent = true`.

**On the next non-empty, non-comment line** (and only if not inside hook braces):

* compute leading spaces count `indent`.
* compare with top of `indentStack`:

  * `indent > top` → emit `INDENT`, push `indent`
  * `indent == top` → no token
  * `indent < top` → emit one or more `DEDENT` until matches, else error if no matching indent level exists

**Special cases**

* Blank lines (empty or comment-only) do not affect indentation.
* Inside `braceDepthHook>0`, **do not emit** `INDENT/DEDENT` at all (indentation is irrelevant inside hook braces).

At EOF:

* emit enough `DEDENT` to return to indent 0, unless you’re inside a string/brace/paren/bracket (then error).

### A6) Tokenization precedence rules (important)

When scanning a line, prefer longest/most specific tokens first:

1. `->` (flow chain operator)
2. `<= >= == !=` then `< > =`
3. `:` (but note: in paths like `/groups/:group_id`, the `:` is part of `PATH`, not a token)
4. `?` assertion prefix
5. Duration literals (`10ms`, `1.5s`, `2m`, `1h`, `7d`)
6. Numbers
7. Strings (`"..."` or `` `...` ``)
8. `PATH` token:

   * captured immediately after an HTTP method
   * can be absolute (`http://...`, `https://...`) or relative (`/users`, `users`)
   * consumes until whitespace or `#` (comment start), **but allow `:` inside PATH**
9. Identifiers / keywords
10. Punctuation: `{ } [ ] ( ) , .`

### A7) PATH rules (with path params)

Tokenize the full path as a single `PATH` token, e.g.:

* `/groups/:group_id/orders/:order_id`
* `/orders`
* `orders/:id`
* `https://api.x.com/v1/orders/:id`

Semantic layer will later extract `:param` occurrences.

### A8) Lexer error conditions (fail fast)

* Unterminated string literal
* Unterminated hook brace block
* Unterminated expr object/array/parens (if you allow multiline)
* Tabs in indentation or anywhere (recommended)
* Invalid indentation: dedent to a level that doesn’t exist in stack
* `{HOOK}` closed by `}` when not in hook mode (brace mismatch)
* `}` when both brace depths are 0

---

## B) Complete semantic check rule-set

Implement semantics as **passes** over the AST. This keeps the compiler clean and errors precise.

### Pass 0 — File graph & import rules

Inputs: entry file AST + imported file ASTs (recursively).

Checks:

1. **Import cycles** are forbidden.
2. Imported files **must not contain `flow` declarations**.
3. `import` path resolution:

   * relative paths resolved from importing file directory (consistent rule)
   * file must exist, readable, UTF-8 (or treat as bytes but consistent)
4. Duplicate imports allowed or not? Recommended: allow duplicates but de-duplicate by canonical path.

Errors:

* `E_IMPORT_CYCLE`
* `E_FLOW_IN_IMPORTED_FILE`
* `E_IMPORT_NOT_FOUND`

### Pass 1 — Symbol tables (requests, globals, flows)

Build:

* Global `ReqTable[name] -> ReqDecl`
* Global `GlobalVars[name] -> LetStmt` (optional, for “defined at top-level” checks)
* Flow table `FlowTable[name] -> FlowDecl` (only in entry)

Checks:

1. Request names are **unique across all files**.
2. Flow names are **unique in entry**.
3. Reserved keywords cannot be used as identifiers (`req`, `flow`, `let`, `import`, `pre`, `post`, `hook`, HTTP methods, `true/false/null`, etc.).

Errors:

* `E_DUPLICATE_REQ_NAME`
* `E_DUPLICATE_FLOW_NAME`
* `E_RESERVED_IDENTIFIER`

### Pass 2 — Request inheritance resolution

For each request `Child(Parent)`:
Checks:

1. Parent exists.
2. No inheritance cycles (DFS/visited states).
3. Parent chain depth within a sane limit (avoid pathological).

Then compute an **expanded request template** (for runtime):

* merged headers/query/auth/body + hooks
* apply your override rules deterministically (child wins on conflicts)

Errors:

* `E_UNKNOWN_PARENT_REQ`
* `E_INHERITANCE_CYCLE`

### Pass 3 — Validate request bodies (structure + constraints)

For each request:
Checks (strongly recommended):

1. Exactly **one HTTP line** (`METHOD PATH/URL`).
2. HTTP method is valid.
3. Hook blocks:

   * At most one `pre hook {}` per request
   * At most one `post hook {}` per request
4. Directive multiplicity:

   * If you allow only one body: only one of `json` (and in the future `body`/`form`)
5. Assertions and lets:

   * Allowed anywhere in request block, but you should define the execution order (I recommend: run hooks, then execute assertions/lets in source order).
6. In `json { ... }`:

   * must parse as your ObjectLit, keys must be `ident` or string literal
   * trailing commas allowed or not (decide here; enforce consistently)

Errors:

* `E_REQ_MISSING_HTTP_LINE`
* `E_REQ_MULTIPLE_HTTP_LINES`
* `E_DUPLICATE_PRE_HOOK`
* `E_DUPLICATE_POST_HOOK`
* `E_MULTIPLE_BODIES`

### Pass 4 — Hook semantics (read/write permissions + availability)

Hook blocks are powerful; constrain them so runtime stays predictable.

Checks:

1. **Pre hook**:

   * `res` and `$` are **not available** (error if referenced)
   * `req` is writable (limited fields)
2. **Post hook**:

   * `res` and `$` are available
   * `res` should be read-only (error if assignment targets `res...`)
3. Assignment target restrictions:

   * Allowed LValue roots: `req`, or a plain variable identifier (if you allow reassignment)
   * Disallow assignment into `res...`
4. Allowed `req` mutation targets (recommended):

   * `req.header[...]`
   * `req.query[...]`
   * `req.url` (optional; I’d disallow to keep path params deterministic)
   * `req.json` (optional; if allowed, it replaces body)

Errors:

* `E_PRE_HOOK_REFERENCES_RES`
* `E_PRE_HOOK_REFERENCES_DOLLAR`
* `E_ASSIGN_TO_RES_FORBIDDEN`
* `E_INVALID_REQ_MUTATION_TARGET`

### Pass 5 — Flow structure rules (hard constraints you asked for)

For each flow:
Checks:

1. Flow body ordering:

   * **Prelude**: only `let` statements (0+)
   * then exactly **one** chain line (single-step allowed, `->` required only for multi-step chains)
   * then **postlude**: only assertions (`? expr`) (0+)
2. Chain format:

   * every step references an existing request name
   * step aliasing `reqName : alias` optional
3. Step binding names:

   * each step produces a unique binding name:

     * if aliased: binding name = alias
     * else binding name = reqName
   * binding names must be unique within the flow
4. If the **same request** is invoked multiple times:

   * require aliases for at least all but one, because bindings must remain unique
   * simpler: just enforce “all bindings unique”; that naturally forces aliasing

Errors:

* `E_FLOW_PRELUDE_NON_LET`
* `E_FLOW_MISSING_CHAIN`
* `E_FLOW_CHAIN_NOT_ARROW_FORMAT`
* `E_FLOW_POSTLUDE_NON_ASSERT`
* `E_UNKNOWN_REQ_IN_FLOW`
* `E_DUPLICATE_FLOW_BINDING`

### Pass 6 — Variable scope & definite-assignment checks (flow-level)

You have no conditionals/loops, so you can do a very solid static check.

Definitions:

* Global vars: top-level `let`
* Flow overrides: prelude `let` (shadow global within that flow)
* Request `let`: writes into flow scope when that request finishes

For each flow:

1. Initialize `DefinedVars` with all global lets.

2. Apply flow prelude lets (override; still “defined”).

3. Walk chain steps in order:

   * Compute **required variables** for that request invocation by scanning:

     * path params in the request path (`:group_id`)
     * template placeholders in strings (`{{group_id}}`) across path/directives/json/hook print args
     * expressions inside directives (`auth bearer token`, headers, query values, json body expressions)
     * expressions in pre hook / post hook / request assertions / request lets
   * Check: each required variable identifier is in `DefinedVars`, unless it is:

     * a built-in function (`env`, `uuid`, `len`, `jsonpath`, `regex`, etc.)
     * a special symbol (`req`, `res`, `$`, `status`, `header`)
     * a flow binding (like `orders1`, `listOrders`) is NOT available inside requests unless you explicitly allow it (I recommend: do **not** allow flow binding references inside request definitions)
   * After executing request, add variables defined by its `let` statements into `DefinedVars` (flow-scoped).

4. Validate flow postlude assertions:

   * Their identifiers may reference:

     * flow variables (`token`, `group_id`, `orderId`)
     * flow bindings (`orders1.res`, `orders1.req`)
   * Ensure referenced bindings exist (from chain).

Errors:

* `E_UNDEFINED_VARIABLE`
* `E_MISSING_PATH_PARAM_VAR` (specialized, nicer UX)
* `E_UNKNOWN_FLOW_BINDING`

### Pass 7 — Dot-access sanity checks (`X.res`, `X.req`)

Since “all responses are json”, treat:

* `X.res` as JSON root (object/array)
* `X.req` as request snapshot object

Checks:

1. In flows, `X` must be a known binding name.
2. Access forms allowed:

   * `X.res` (and then `.field` / `["field"]`)
   * `X.req` (and then `.method/.url/.header/...` — you define the shape)
   * `X.status`, `X.header[...]` (optional sugar; if supported, validate)
3. Disallow `X.res.json` (because `res` already is json root in this DSL) unless you intentionally allow it for compatibility.

Errors:

* `E_INVALID_FLOW_ACCESS` (e.g., `X.response` or `X.res.json`)
* `E_UNKNOWN_BINDING`

### Pass 8 — Expression checks (lightweight but useful)

You can keep runtime dynamic, but still catch common mistakes:

Checks (recommended):

1. Operator arity:

   * `not` unary
   * comparisons binary
2. `in` RHS should be array literal when it’s a literal; else allow dynamic.
3. `contains` RHS type sanity for literals.
4. Function calls:

   * built-in functions exist
   * optional: check arg counts for built-ins (great UX)

Errors:

* `E_UNKNOWN_FUNCTION`
* `E_BAD_ARGUMENT_COUNT`
* `E_INVALID_OPERATOR_USAGE`

---

## Implementation tips (Go-friendly)

### Data structures you’ll want

* `FileUnit{Path, AST, Imports[]}`
* `ReqDecl{Name, ParentName?, Lines[], ExpandedTemplate}`
* `FlowDecl{Name, PreludeLets[], ChainSteps[], PostAsserts[]}`
* `ChainStep{ReqName, BindingName}` where `BindingName = alias or ReqName`
* Symbol tables:

  * `map[string]*ReqDecl`
  * `map[string]*FlowDecl`
  * `map[string]VarInfo` per scope

### Required-vars extraction (static)

Write an AST walker that collects identifier references from expressions, excluding:

* built-ins (`env`, `uuid`, `len`, `jsonpath`, `regex`, …)
* special names (`req`, `res`, `$`, `status`, `header`)
* keywords

For path params and string templates:

* parse PATH string for occurrences of `/:<ident>` or `:<ident>` after `/`
* parse string literals for `{{<ident>}}` placeholders
* collect `<ident>` as required vars, except request-context template symbols allowed by hook timing (`{{req}}` in pre/post; `{{status}}`/`{{res}}` in post only)

---

## Recommended error style (makes the tool feel “real”)

Every error should include:

* file path
* line/col range
* error code + message
* a short hint

Example:

* `E_FLOW_POSTLUDE_NON_ASSERT: only assertions are allowed after the chain line in a flow. Found: let orderId = ...`

---
