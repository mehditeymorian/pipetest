# Language constraints: parser vs semantic compiler

This matrix maps key language constraints to where they are enforced:
- **parse**: enforced while building AST from `grammar.ebnf` productions.
- **semantic pass N**: enforced after parsing in compiler passes (`Pass00`..`Pass06`).

| Constraint area | What is enforced | Grammar production reference (`grammar.ebnf`) | Enforcement stage | Error code name(s) |
|---|---|---|---|---|
| 1. Flow structure (prelude lets, exactly one chain line, postlude assertions) | Flow block shape is `let*` then exactly one chain line then assertion lines only. Non-`let` before chain and non-assert after chain are rejected. Missing/invalid chain format is rejected. | `FlowDecl`, `FlowPreludeLine`, `FlowChainLine`, `FlowAssertLine` | **parse** (shape/ordering), plus **semantic pass 5** (final hard validation) | `E_FLOW_PRELUDE_NON_LET`, `E_FLOW_MISSING_CHAIN`, `E_FLOW_CHAIN_NOT_ARROW_FORMAT`, `E_FLOW_POSTLUDE_NON_ASSERT` |
| 2. Chain requirements (at least one `->`, alias uniqueness) | Chain must contain at least one arrow transition; flow binding names (alias or request name fallback) must be unique within a flow. | `FlowChainLine`, `FlowStepRef` | **semantic pass 5** | `E_FLOW_CHAIN_NOT_ARROW_FORMAT`, `E_DUPLICATE_FLOW_BINDING` |
| 3. Request rules (single HTTP line, duplicate hook/body constraints) | Each request must declare exactly one HTTP line; duplicate `pre`/`post` hooks are forbidden; multiple body directives are forbidden. | `ReqDecl`, `ReqLine`, `HttpLine`, `HookBlock`, `Directive`, `JsonDirective` | **semantic pass 3** | `E_REQ_MISSING_HTTP_LINE`, `E_REQ_MULTIPLE_HTTP_LINES`, `E_DUPLICATE_PRE_HOOK`, `E_DUPLICATE_POST_HOOK`, `E_MULTIPLE_BODIES` |
| 4. Import constraints (no flows in imported files, cycle detection) | Import graph must be acyclic; imported files cannot define `flow` declarations. | `ImportStmt`, `FlowDecl` | **semantic pass 0** | `E_IMPORT_CYCLE`, `E_FLOW_IN_IMPORTED_FILE` |
| 5. Hook restrictions (`pre` cannot access `res`/`$`, `res` mutation forbidden) | In `pre hook`, references to `res` and `$` are forbidden; assigning into `res...` is forbidden (especially in post hook where `res` is readable but immutable). | `HookBlock`, `HookKind`, `HookStmt`, `AssignStmt`, `LValue`, `Expr` | **semantic pass 4** | `E_PRE_HOOK_REFERENCES_RES`, `E_PRE_HOOK_REFERENCES_DOLLAR`, `E_ASSIGN_TO_RES_FORBIDDEN` |

## Notes
- Some flow constraints are intentionally checked in both parser and semantic phases: parser gives immediate structural feedback; pass 5 provides authoritative compiler diagnostics.
- The pass numbering above aligns with the compiler pipeline described in project docs (`Pass00ImportGraph -> ... -> Pass06VariableAvailability`).
