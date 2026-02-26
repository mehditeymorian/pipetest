# Reporting

`pipetest run` should produce deterministic, CI-friendly reports that make failures easy to triage from local runs and hosted pipelines.

## Report formats

`pipetest run` writes all standard artifacts by default:

- **JUnit XML** (`pipetest-junit.xml`)
- **JUnit compatibility alias** (`pipetest-report.xml`)
- **JSON summary** (`pipetest-report.json`)

This keeps local and CI consumption simple without extra flags.

## Mapping strategy: DSL runtime -> test report model

Use a deterministic mapping so the same program always emits stable identifiers and paths.

- **testsuite** = flow name
  - One JUnit `<testsuite>` per flow execution.
  - Suite-level counters (`tests`, `failures`, `errors`, `skipped`, `time`) are derived from all testcase rows emitted for that flow.
- **testcase** = request step and/or flow assertion
  - Emit a testcase for each executed request step (including request-level assertions).
  - Emit additional testcase rows for flow-level assertions that are not tied to a single request.
  - Recommended testcase naming format:
    - request execution row: `<stepIndex> <requestDisplayName>`
    - request assertion row: `<stepIndex> <requestDisplayName> :: assert <assertionIndex>`
    - flow assertion row: `flow :: assert <assertionIndex>`
- **failure nodes**
  - Assertion failures should emit `<failure>` nodes.
  - Runtime execution faults (HTTP transport failure, timeout, unresolved symbol at runtime, hook crash) should emit `<error>` nodes.
  - Failure/error messages should include deterministic step identifiers and source location, when available.

## Artifact paths and defaults

Default output files from `pipetest run`:

- `pipetest-junit.xml`
- `pipetest-report.xml`
- `pipetest-report.json`

Recommendations:

- Resolve paths relative to process working directory unless user passes explicit output locations.
- Overwrite previous files by default to avoid stale report confusion.
- Ensure parent directories are created when a nested output path is provided.

## CI integration snippets

### GitHub Actions

```yaml
name: pipetest

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run pipetest
        run: pipetest run program.pt

      - name: Upload report artifacts
        uses: actions/upload-artifact@v4
        with:
          name: pipetest-reports
          path: |
            pipetest-report.xml
            pipetest-report.json

      # Example ingestion with a JUnit reporter action
      - name: Publish JUnit report
        if: always()
        uses: mikepenz/action-junit-report@v5
        with:
          report_paths: pipetest-report.xml
```

### GitLab CI

```yaml
stages:
  - test

pipetest:
  stage: test
  image: alpine:latest
  script:
    - pipetest run program.pt
  artifacts:
    when: always
    paths:
      - pipetest-report.xml
      - pipetest-report.json
    reports:
      junit: pipetest-report.xml
```

## Deterministic naming for aliased flow steps

For flow steps referencing aliased requests (`reqName:alias`), generate display names that remain stable across runs:

- canonical step id: `<requestName>:<alias>`
- display label (recommended): `<stepIndex> <requestName>:<alias>`
- if no alias is present: `<stepIndex> <requestName>`

Rules:

- Preserve alias text exactly as authored (case-sensitive).
- Use the canonical id in failure/error message headers to avoid ambiguity when the same request is used multiple times in a flow.
- Include source location in failure messages using `file:line:column` when compiler metadata exists.

Recommended failure message header format:

```text
<flowName> :: <requestName>:<alias> :: <kind> @ <file>:<line>:<column>
```

Where `<kind>` is `assertion-failed` or `runtime-error`.

## Related docs

- [CLI specification](cli-spec.md)
- [Language execution model](language/execution-model.md)
- [Examples catalog](examples/README.md)
- [CI regression suite example](examples/08-ci-regression-suite.md)
