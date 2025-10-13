# Plan

## Scope
- Address open issues GR-01, GR-02, and GR-03 sequentially.
- Bring existing code into compliance with the repository standards (no inline comments, descriptive identifiers, constants for strings, struct-oriented design).

## Step 1 – Establish test harness (GR-01 prerequisite)
- Create dedicated integration test package under `internal/tests/integration` to exercise the public CLI surface via the `server` and `runner` packages.
- Build reusable helpers (struct-oriented) for spawning commands within tests, using table-driven scenarios and `t.TempDir()` for isolation.
- Define constants for repeated strings and exit codes used in assertions.

## Step 2 – Reproduce and guard the hang regression (GR-02)
- Write integration tests that invoke `server.RunServerPart` with trivial commands (`/bin/echo`) to capture the current hang.
- Extend tests to cover timeout enforcement and descendant termination using a helper program that spawns child processes and reports via files inside `t.TempDir()`.
- Ensure tests fail under current implementation, confirming the hang and absence of enforcement guarantees.

## Step 3 – Runner refactor to structured executor (GR-02)
- Introduce a `CommandExecutor` struct in `internal/runner` encapsulating logger, process handles, timers, and helper goroutines.
- Replace inline comments with GoDoc on exported symbols, and reorganize functionality into cohesive methods (`prepare`, `launch`, `awaitCompletion`, `enforceTimeout`), each leveraging contexts for cancellation.
- Guarantee goroutine lifecycles are tied to contexts, and ensure all helper channels are drained deterministically to resolve the hang.
- Continue supporting cgroup/macOS behaviors while simplifying Linux-specific branches via strategy methods.

## Step 4 – Server layer alignment (GR-02, GR-03)
- Update `internal/server/server.go` to use a struct-oriented API delegating to the new executor, ensuring constants for strings and improved error wrapping.
- Adjust the CLI glue in `cmd/root.go` to use descriptive constants for help/usage text, convert to struct-based command assembly, and remove inline comments while keeping behavior intact.

## Step 5 – Documentation completeness (GR-03)
- Audit all exported identifiers across packages (`cmd`, `server`, `runner`, `signals`, `cgroup`, `procscan`, `proctrack`, `util/exitcodes`) and add missing GoDoc comments or elevate existing inline comments to proper documentation blocks.
- Add a `doc.go` file where package-level context is missing or where large comment blocks should live.

## Step 6 – Finalize integration suite (GR-01)
- Expand integration tests to reach 100% code coverage thresholds by covering success path, timeout path, shell execution path, cgroup unavailability, and macOS tracker fallbacks (using stubs/mocks where OS-specific behavior cannot run on Linux).
- Introduce table-driven tests verifying CLI argument validation separately from execution, ensuring coverage for all usage errors.
- Confirm tests rely solely on exported behavior, not internal implementation details.

## Step 7 – Repository documentation and bookkeeping
- Update `README.md` (and/or create `MIGRATION.md` if required) to describe the new testing strategy and execution guarantees.
- Update `NOTES.md` to mark GR-01, GR-02, and GR-03 as completed once corresponding steps are done.
- Run `go fmt ./...`, `go vet ./...`, and the full test suite under timeouts before committing.
