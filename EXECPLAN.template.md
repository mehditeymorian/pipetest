# EXECPLAN

## Docs consulted
- (fill) /docs/...
- (fill) README.md

## Assumptions (must be explicit)
- (fill)

## Open questions
- (fill) -> track in QUESTIONS.md

## Milestones
### M0: Repo hygiene / baseline
- [ ] Confirm build/test commands
- [ ] Ensure `go test ./...` is green
- [ ] Add CI for lint+test

### M1: Architecture scaffold from docs
- [ ] Create folder structure (/cmd, /internal)
- [ ] Define interfaces + boundaries (ports/adapters if applicable)

### M2: Vertical slice 1 (smallest user-facing feature)
- [ ] API/handler
- [ ] service
- [ ] persistence (in-memory or real DB per docs)
- [ ] tests (unit + one integration if possible)

### M3+: Additional slices
- [ ] Slice 2 ...
- [ ] Slice 3 ...

### Hardening
- [ ] Observability/logging
- [ ] Config management
- [ ] Migrations
- [ ] Docs updates
