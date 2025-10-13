# Plan

## Scope
- Resolve remaining open issues GR-04, GR-05, GR-06, and GR-07 sequentially, ensuring each fix is introduced with failing tests first.
- Maintain compliance with repository standards (descriptive identifiers, constants for strings, struct-oriented design, zap logging only for errors).

## Status
- [x] Step 1 – Integration tests reproduce the macOS tracker deadlock before the refactor.
- [x] Step 2 – Runner and tracker shutdown flows refactored to avoid blocking channel operations.
- [x] Step 3 – CLI and integration suites stabilized with table-driven tests and deterministic timeouts.
- [x] Step 4 – Logging limited to error reporting only.
- [x] Step 5 – GitHub Actions workflow added for formatting, vetting, testing, and cross-platform builds.
- [x] Step 6 – Final documentation updates and command verifications before merge.

## Step 1 – Characterize deadlock regression (GR-04)
- Extend integration tests under `internal/tests/integration` to reproduce the deadlock reported when executing trivial commands.
- Build table-driven scenarios covering immediate command completion, timeout paths, and tracker shutdown to demonstrate the blocking behavior.
- Ensure new tests fail against the current implementation, capturing the hanging behavior.

## Step 2 – Refactor runner shutdown flow (GR-04)
- Audit `internal/runner` and related tracker code to identify blocking channel operations.
- Introduce cohesive structs with method receivers to coordinate tracker lifecycles and goroutine shutdown, ensuring contexts govern cancellation.
- Replace busy waits or blocking receives with deterministic fan-in using select statements and buffered channels to prevent deadlock.

## Step 3 – Stabilize test suite (GR-05)
- Once the runner fix is in place, update existing tests to use deterministic timeouts and assert on observable behavior rather than implementation details.
- Add missing unit or integration tests to achieve full coverage of timeout exit codes and server coordination.

## Step 4 – Logging discipline (GR-06)
- Review all packages for non-error logging; replace informational logging with structured error returns or remove unnecessary statements.
- Ensure remaining logs use zap with error level and descriptive constants.

## Step 5 – Continuous integration workflow (GR-07)
- Add GitHub Actions workflow under `.github/workflows` to run `go fmt`, `go vet`, and `go test` across supported platforms (Linux, macOS, Windows cross-build) and archive built binaries.
- Use matrix strategy with caching where applicable and ensure secrets are not required.

## Step 6 – Documentation and bookkeeping
- Update `README.md` and `NOTES.md` to reflect resolved issues and new CI workflow.
- Confirm `PLAN.md` accurately describes completed work and keep sections aligned with repository standards.
- Run `timeout 120s go fmt ./...`, `timeout 120s go vet ./...`, and `timeout 120s go test ./...` before final commit.
