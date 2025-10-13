# Plan

## Scope
- Address issue GR-09 by splitting the current combined GitHub Actions workflow into dedicated Continuous Integration and release build pipelines.
- Ensure the CI workflow only runs verification steps (format, vet, test) and triggers on pull requests targeting `master` as well as direct pushes.
- Ensure the release workflow runs cross-platform build and checksum steps exclusively when a semantic version tag is pushed.
- Update NOTES.md to mark GR-09 complete once the workflows are separated.

## Steps
1. Inspect the existing `.github/workflows/ci.yml` to catalogue the verification and build steps that need to be redistributed.
2. Refactor `.github/workflows/ci.yml` so it retains only the lint and test job, updates triggers to the required events, and removes build responsibilities.
3. Create a new `.github/workflows/release.yml` that runs the build matrix and checksum generation when version tags (e.g. `v*`) are pushed, uploading artifacts as before.
4. Update NOTES.md to mark GR-09 as completed, keeping the section headings intact.
5. Run `timeout 120s go fmt ./...`, `timeout 120s go vet ./...`, and `timeout 120s go test ./...` to ensure the repository remains healthy.
