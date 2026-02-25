# Diagnostics specification

This document defines the stable diagnostic contract for `pipetest`, including error code naming, required fields, output shapes, command behavior, and deterministic ordering.

## 1) Stable error code namespace

All diagnostics MUST use a stable code in one of these namespaces:

- `E_PARSE_*`: lexer/parser structure errors.
- `E_IMPORT_*`: import graph and file-loading errors.
- `E_SEM_*`: semantic validation errors detected before execution.
- `E_RUNTIME_*`: runtime execution failures while running flows/requests.
- `E_ASSERT_*`: assertion evaluation failures.

### Initial source list and finalized naming

`docs/rule-sets.md` is the initial source of diagnostic identifiers. Several existing codes are already stable and can be retained as canonical names where they already fit the namespace model (for example, `E_IMPORT_CYCLE`). Others should be finalized under the `E_SEM_*` namespace for consistency.

| Source identifier in `docs/rule-sets.md` | Final canonical code |
| --- | --- |
| `E_IMPORT_CYCLE` | `E_IMPORT_CYCLE` |
| `E_FLOW_IN_IMPORTED_FILE` | `E_IMPORT_FLOW_IN_IMPORTED_FILE` |
| `E_IMPORT_NOT_FOUND` | `E_IMPORT_NOT_FOUND` |
| `E_DUPLICATE_REQ_NAME` | `E_SEM_DUPLICATE_REQ_NAME` |
| `E_DUPLICATE_FLOW_NAME` | `E_SEM_DUPLICATE_FLOW_NAME` |
| `E_RESERVED_IDENTIFIER` | `E_SEM_RESERVED_IDENTIFIER` |
| `E_UNKNOWN_PARENT_REQ` | `E_SEM_UNKNOWN_PARENT_REQ` |
| `E_INHERITANCE_CYCLE` | `E_SEM_INHERITANCE_CYCLE` |
| `E_REQ_MISSING_HTTP_LINE` | `E_SEM_REQ_MISSING_HTTP_LINE` |
| `E_REQ_MULTIPLE_HTTP_LINES` | `E_SEM_REQ_MULTIPLE_HTTP_LINES` |
| `E_DUPLICATE_PRE_HOOK` | `E_SEM_DUPLICATE_PRE_HOOK` |
| `E_DUPLICATE_POST_HOOK` | `E_SEM_DUPLICATE_POST_HOOK` |
| `E_MULTIPLE_BODIES` | `E_SEM_MULTIPLE_BODIES` |
| `E_PRE_HOOK_REFERENCES_RES` | `E_SEM_PRE_HOOK_REFERENCES_RES` |
| `E_PRE_HOOK_REFERENCES_DOLLAR` | `E_SEM_PRE_HOOK_REFERENCES_DOLLAR` |
| `E_ASSIGN_TO_RES_FORBIDDEN` | `E_SEM_ASSIGN_TO_RES_FORBIDDEN` |
| `E_INVALID_REQ_MUTATION_TARGET` | `E_SEM_INVALID_REQ_MUTATION_TARGET` |
| `E_FLOW_PRELUDE_NON_LET` | `E_SEM_FLOW_PRELUDE_NON_LET` |
| `E_FLOW_MISSING_CHAIN` | `E_SEM_FLOW_MISSING_CHAIN` |
| `E_FLOW_CHAIN_NOT_ARROW_FORMAT` | `E_SEM_FLOW_CHAIN_NOT_ARROW_FORMAT` |
| `E_FLOW_POSTLUDE_NON_ASSERT` | `E_SEM_FLOW_POSTLUDE_NON_ASSERT` |
| `E_UNKNOWN_REQ_IN_FLOW` | `E_SEM_UNKNOWN_REQ_IN_FLOW` |
| `E_DUPLICATE_FLOW_BINDING` | `E_SEM_DUPLICATE_FLOW_BINDING` |
| `E_UNDEFINED_VARIABLE` | `E_SEM_UNDEFINED_VARIABLE` |
| `E_MISSING_PATH_PARAM_VAR` | `E_SEM_MISSING_PATH_PARAM_VAR` |
| `E_UNKNOWN_FLOW_BINDING` | `E_SEM_UNKNOWN_FLOW_BINDING` |
| `E_INVALID_FLOW_ACCESS` | `E_SEM_INVALID_FLOW_ACCESS` |
| `E_UNKNOWN_BINDING` | `E_SEM_UNKNOWN_BINDING` |
| `E_UNKNOWN_FUNCTION` | `E_SEM_UNKNOWN_FUNCTION` |
| `E_BAD_ARGUMENT_COUNT` | `E_SEM_BAD_ARGUMENT_COUNT` |
| `E_INVALID_OPERATOR_USAGE` | `E_SEM_INVALID_OPERATOR_USAGE` |

Compatibility guidance:

- During migration, implementations MAY emit a legacy alias in an additional field (for example, `legacy_code`) while printing only canonical code in standard text output.
- New diagnostics MUST be added directly under one of the canonical namespaces above.

## 2) Required diagnostic fields

Every diagnostic record MUST include:

- `code` (string): canonical error code.
- `message` (string): human-readable error description.
- `file` (string): source file path.
- `line` (integer, 1-based): source line.
- `column` (integer, 1-based): source column.
- `hint` (string): remediation guidance; if no meaningful remediation exists, use an empty string.

Optional fields:

- `related` (object): secondary location for cross-reference diagnostics.
  - `file` (string)
  - `line` (integer)
  - `column` (integer)
  - `message` (string, optional)

## 3) Output schema and examples

### JSON schema shape (machine-consumable)

```json
{
  "command": "eval | run",
  "ok": false,
  "diagnostics": [
    {
      "severity": "error",
      "code": "E_SEM_FLOW_MISSING_CHAIN",
      "message": "flow must contain a chain line",
      "file": "tests/orders.pt",
      "line": 42,
      "column": 3,
      "hint": "add a chain line after flow prelude lets",
      "related": {
        "file": "tests/orders.pt",
        "line": 40,
        "column": 1,
        "message": "flow declaration starts here"
      },
      "flow": "checkout_happy_path",
      "request": null
    }
  ],
  "summary": {
    "error_count": 1
  }
}
```

Notes:

- `severity` is currently `error` for all compilation/execution diagnostics.
- `flow` and `request` are optional context fields, primarily used by `run`.
- `diagnostics` MUST be sorted and deduplicated according to the policy in section 5.

### Plain text examples

Parse/semantic/import style:

```text
ERROR E_SEM_FLOW_MISSING_CHAIN tests/orders.pt:42:3 flow must contain a chain line
  hint: add a chain line after flow prelude lets
```

With related location:

```text
ERROR E_SEM_DUPLICATE_FLOW_BINDING tests/orders.pt:55:12 duplicate flow binding 'createOrder'
  related: tests/orders.pt:53:12 first binding declared here
  hint: rename one binding alias in the chain
```

Runtime/assert style:

```text
ERROR E_ASSERT_EXPECTED_TRUE tests/orders.pt:88:5 expected assertion to evaluate to true
  flow: checkout_happy_path
  request: createOrder
  hint: inspect previous response payload and assertion predicate
```

## 4) Command behavior differences

### `pipetest eval`

`eval` emits diagnostics from non-execution phases only:

- parse diagnostics (`E_PARSE_*`)
- import diagnostics (`E_IMPORT_*`)
- semantic diagnostics (`E_SEM_*`)

`eval` MUST NOT emit `E_RUNTIME_*` or `E_ASSERT_*` diagnostics.

### `pipetest run`

`run` includes all `eval` diagnostics first. If static checks pass, execution proceeds and may emit:

- runtime diagnostics (`E_RUNTIME_*`)
- assertion diagnostics (`E_ASSERT_*`)

`run` diagnostics SHOULD include per-flow context fields when available:

- `flow` (flow name)
- `request` (request name/binding, if applicable)
- optional execution identifiers (for example `attempt`, `step_index`) in JSON output

## 5) Deterministic sorting and deduping policy

To keep CI output stable across runs and environments:

1. Normalize file paths before sorting (canonical relative path from repo root, `/` separators).
2. Build a deterministic sort key:
   1. `file`
   2. `line`
   3. `column`
   4. `code`
   5. `message`
   6. `related.file` (empty string if absent)
   7. `related.line` (0 if absent)
   8. `related.column` (0 if absent)
3. Deduplicate exact duplicates using the tuple:
   - `(code, file, line, column, message, related.file, related.line, related.column)`
4. Preserve the first instance encountered for duplicates that carry extra context fields.
5. Emit diagnostics in final sorted order for both JSON and plain text output.

This policy applies equally to `eval` and `run` output.
