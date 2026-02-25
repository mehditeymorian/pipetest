# CLI specification

This document defines expected behavior for the `pipetest` command-line interface.
`pipetest` is a scripting language for testing APIs and API flows. It is a general-purpose tool intended for public use, and it does not expose an API or SDK.

## Commands

`pipetest` has two commands: `eval` for static evaluation and `run` for executing scripts.

## `pipetest eval <program.pt>`

Static analysis only.

### Responsibilities

- load entry file
- resolve imports recursively
- parse all files
- run semantic/compiler validation passes
- print diagnostics in deterministic order

### Exit codes

- `0`: no errors
- `1`: syntax or semantic/import errors
- `2`: invalid CLI usage

### Example

```bash
pipetest eval examples/happy-path.pt
```

---

## `pipetest run <program.pt>`

Compile and execute flows.

### Responsibilities

- run all `eval` checks first
- if compilation succeeds, execute flows
- evaluate request and flow assertions
- emit summary to stdout
- write CI artifacts

### Exit codes

- `0`: all flows succeeded, all assertions passed
- `1`: compilation/runtime/assertion failures
- `2`: invalid CLI usage

### Example

```bash
pipetest run examples/happy-path.pt
```

---

## Global flags (recommended)

- `--report-dir <dir>`: output directory for generated artifacts (default `./pipetest-report`)
- `--format <pretty|json>`: stdout format
- `--fail-fast`: stop all execution after first flow failure
- `--timeout <duration>`: override global timeout from file

---

## Output artifacts

When running `pipetest run`:

- `pipetest-report.json`
- `pipetest-junit.xml`

These files should always be written when execution starts, even if there are failures.

---

## Diagnostic format (recommended)

Use a stable structure:

```text
<SEVERITY> <CODE> <file>:<line>:<col> <message>
  hint: <optional remediation>
```

Example:

```text
ERROR E_FLOW_MISSING_CHAIN tests/orders.pt:42:3 flow must contain a chain line
  hint: add a chain line after flow prelude lets
```

---

## CI usage examples

### GitHub Actions

```yaml
- name: Run pipetest
  run: pipetest run tests/api.pt --report-dir artifacts

- name: Publish JUnit
  uses: actions/upload-artifact@v4
  with:
    name: pipetest-junit
    path: artifacts/pipetest-junit.xml
```

### GitLab CI

```yaml
pipetest:
  script:
    - pipetest run tests/api.pt --report-dir artifacts
  artifacts:
    when: always
    reports:
      junit: artifacts/pipetest-junit.xml
    paths:
      - artifacts/pipetest-report.json
```
