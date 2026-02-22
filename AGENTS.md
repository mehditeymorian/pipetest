# AGENTS.md (repo root)

## Mission
Implement this project strictly based on repository documentation, keeping the repo buildable at all times.

## Source of truth (in priority order)
1) /docs/** (specs, ADRs, architecture, requirements)
2) README.md (setup, run, build/test commands)
3) Go code comments + existing patterns in /cmd, /internal, /pkg
4) If something is unclear: write QUESTIONS.md and add TODOs instead of guessing.

## Working style
- Make small, reviewable batches (PR-sized diffs).
- Prefer end-to-end vertical slices over big rewrites.
- Never break `go test ./...` on main changes.
- Update docs whenever behavior changes.

## Required workflow (do not skip)
1) Read relevant docs first and list the exact files used.
2) Produce a short plan (milestones + files to change).
3) Implement milestone 1 only.
4) Run verification:
   - `go test ./...`
   - and any repo-specific commands (see below)
5) Summarize changes + how to run + how verified.

## Repo commands (edit these to match reality)
- Format: `gofmt -w .`
- Lint: `golangci-lint run ./...`
- Tests: `go test ./...`
- Build: `go build ./...`

## Go conventions
- Structure:
  - /cmd/<service>/main.go for entrypoints
  - /internal/<domain|app|infra>/... for non-public packages
  - /pkg/... only if truly public
- Errors: wrap with `%w`, prefer sentinel errors for domain cases.
- Context: pass `context.Context` through I/O boundaries.
- Logging: use a single logger abstraction; avoid global state.

## Delivery expectations per milestone
- Code + tests
- Any new config/migrations/scripts required
- Docs updates
