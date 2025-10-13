# Plan

## Scope
- Address open issue GR-08 by hardening macOS process tracking so the executor gracefully handles unsupported kqueue capabilities without emitting runtime errors.
- Preserve existing functionality for Linux execution paths and ensure fallbacks remain effective for descendant termination.
- Maintain compliance with repository standards: descriptive identifiers, constants for strings, struct-oriented design, zap error logging only, and full test coverage.

## Steps
1. **Characterize failure**
   - Add a focused test under `internal/runner` that exercises the executor with a mac tracker factory returning an `operation not supported` error to reproduce the log spam and verify current behavior.
   - Ensure the new test initially fails by asserting on the returned error until the implementation is updated.
2. **Refactor mac tracker handling**
   - Introduce a dedicated error classification in the proctrack package for unsupported tracking scenarios.
   - Update the mac tracker to detect `ENOTSUP`/`ENOSYS` from `kevent` registrations, wrap them with the new error, and close resources cleanly.
   - Teach the execution manager to treat the unsupported-tracking error as an expected fallback (no error-level log, tracker disabled).
3. **Extend tests**
   - Finalize the new test to assert that execution succeeds without emitting errors when tracking is unsupported.
   - Add coverage for tracker shutdown to ensure resources are released when fallback occurs.
4. **Documentation and bookkeeping**
   - Mark GR-08 as completed in `NOTES.md` while keeping the section header.
   - Run `timeout 120s go fmt ./...`, `timeout 120s go vet ./...`, and `timeout 120s go test ./...` before committing.
