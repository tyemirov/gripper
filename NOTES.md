# Notes

## Role

You are a staff level full stack engineer. Your task is to **re-evaluate and refactor the Prompt Bubbles repository** according to the coding standards already written in **AGENTS.md**.

## Context

* AGENTS.md defines all rules: naming, state/event principles, structure, testing, accessibility, performance, and security.
* The repo uses Alpine.js, CDN scripts only, no bundlers.
* Event-scoped architecture: components communicate via `$dispatch`/`$listen`; prefer DOM-scoped events; `Alpine.store` only for true shared domain state.
* The backend uses Go language ecosystem

## Your tasks

1. **Read AGENTS.md first** → treat it as the *authoritative style guide*.
2. **Scan the codebase** → identify violations (inline handlers, globals, duplicated strings, lack of constants, cross-component state leakage, etc.).
3. **Generate PLAN.md** → bullet list of problems and refactors needed, scoped by file. PLAN.md is a part of PR metadata. It's a transient document outlining the work on a given issue.
4. **Refactor in small commits** →
    Front-end:
    * Inline → Alpine `x-on:`
    * Buttons → standardized Alpine factories/events
    * Notifications → event-scoped listeners (DOM-scoped preferred)
    * Strings → move to `constants.js`
    * Utilities → extract into `/js/utils/`
    * Composition → normalize `/js/app.js` as Alpine composition root
    Backend:
    * Use "object-oreinted" stye of functions attached to structs
    * Prioritize data-driven solutions over imperative approach
    * Design and use shared components
5. **Tests** → Add/adjust Puppeteer tests for key flows (button → event → notification; cross-panel isolation). Prioritize end-2-end and integration tests.
6. **Docs** → Update README and MIGRATION.md with new event contracts, removed globals, and developer instructions.
7. **Timeouts**  Set a timer before running any CLI command, tests, build, git etc. If an operation takes unreasonably long without producing an output, abort it and consider a different approach. Prepend all CLI invocations with `timeout <N>s` command.

## Output requirements

* Always follow AGENTS.md rules (do not restate them, do not invent new ones).
* Output a **PLAN.md** first, then refactor step-by-step.
* Only modify necessary files.
* Descriptive identifiers, no single-letter names.
* End with a short summary of changed files and new event contracts.

**Begin by reading AGENTS.md and generating PLAN.md now.**

## Rules of engagement

Review the NOTES.md. Make a plan for autonomously fixing every item under Features, BugFixes, Improvements, Maintenance. Ensure no regressions. Ensure adding tests. Lean into integration tests. Fix every issue. Document the changes.

Fix issues one by one, working sequentially. 
1. Create a new git bracnh with descriptive name, for example `feature/LA-56-widget-defer` or `bugfix/LA-11-alpine-rehydration`. Use the taxonomy of issues as prefixes: improvement/, feature/, bugfix/, maintenace/, issue ID and a short descriptive. Respect the name limits.
2. Describe an issue through tests. 
2a. Ensure that the tests are comprehensive and failing to begin with. 
2b. Ensure AGENTS.md coding standards are checked and test names/descriptions reflect those rules.
3. Fix the issue
4. Rerun the tests
5. Repeat pp 2-4 untill the issue is fixed: 
5a. old and new comprehensive tests are passing
5b. Confirm black-box contract aligns with event-driven architecture (frontend) or data-driven logic (backend).
5c. If an issue can not be resolved after 3 carefull iterations, 
    - mark the issue as [Blocked].
    - document the reason for the bockage.
    - commit the changes into a separate branch called "blocked/<issue-id>".
    - work on the next issue from the divergence point of the previous issue.
6. Write a nice comprehensive commit message AFTER EACH issue is fixed and tested and covered with tests.
7. Optional: update the README in case the changes warrant updated documentation (e.g. have user-facing consequences)
8. Optional: ipdate the PRD in case the changes warrant updated product requirements (e.g. change product undestanding)
9. Optional: update the code examples in case the changes warrant updated code examples
10. Mark an issue as done ([X])in the NOTES.md after the issue is fixed: New and existing tests are passing without regressions
11. Commit and push the changes to the remote branch.
12. Repeat till all issues are fixed, and commits abd branches are stacked up (one starts from another).

Do not work on all issues at once. Work at one issue at a time sequntially.

Leave Features, BugFixes, Improvements, Maintenance sections empty when all fixes are implemented but don't delete the sections themselves.

## Issues

### Features

### Improvements

- [X] [GR-01] Prepare a full suite of integration tests with 100% coverage of the code
- [X] [GR-03] Have complete GDoc code coverage
- [X] [GR-06] There should be no logging in this program other than error reporting.
- [X] [GR-07] Add github actions to test and build an executable for various platforms. Check similar projects and adapat to this one
    ```yaml
    name: Tests

    on:
    push:
        branches:
        - master
        paths:
        - "**/*.js"
        - "**/*.html"
        - "**/*.css"
    pull_request:
        branches:
        - master
        paths:
        - "**/*.js"
        - "**/*.html"
        - "**/*.css"

    jobs:
    node-tests:
        runs-on: ubuntu-latest
        steps:
        - name: Checkout repository
            uses: actions/checkout@v4

        - name: Setup Node.js
            uses: actions/setup-node@v4
            with:
            node-version: 22

        - name: Install dependencies
            run: npm install

        - name: Install browser runtime
            run: npx puppeteer browsers install chrome

        - name: Run tests
            run: npm test
    ```
    ```yaml
    name: Release Build

    on:
    push:
        tags:
        - 'v*'

    permissions:
    contents: write

    jobs:
    release:
        runs-on: ubuntu-latest

        steps:
        - name: CheckoutCode
            uses: actions/checkout@v4

        - name: SetupGoEnvironment
            uses: actions/setup-go@v5
            with:
            go-version-file: go.mod
            check-latest: true
            cache: true

        - name: BuildCTXBinaries
            run: |
            mkdir -p dist
            GOOS=linux   GOARCH=amd64   go build -ldflags="-s -w" -o dist/ctx_linux_amd64   ./cmd/ctx
            GOOS=darwin  GOARCH=amd64   go build -ldflags="-s -w" -o dist/ctx_darwin_amd64  ./cmd/ctx
            GOOS=darwin  GOARCH=arm64   go build -ldflags="-s -w" -o dist/ctx_darwin_arm64 ./cmd/ctx
            GOOS=windows GOARCH=amd64   go build -ldflags="-s -w" -o dist/ctx_windows_amd64.exe ./cmd/ctx

        - name: GenerateChecksums
            run: |
            cd dist
            sha256sum * > checksums.txt

        - name: ExtractReleaseNotes
            run: |
            TAG_NAME=${{ github.ref_name }}
            sed -n "/## \[$TAG_NAME\]/,/\(## \[v[0-9]\)/p" CHANGELOG.md | grep -v '^## \[' > release_notes.md

        - name: CreateGitHubRelease
            uses: softprops/action-gh-release@v2
            with:
            name: "CTX release ${{ github.ref_name }}"
            body_path: release_notes.md
            files: |
                dist/ctx_linux_amd64
                dist/ctx_darwin_amd64
                dist/ctx_darwin_arm64
                dist/ctx_windows_amd64.exe
                dist/checksums.txt
            env:
            GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    ```
- [ ] [GR-09] The CI and build pipelines must be separate. Teh CI piepline must react to opening a PR against master, the build pipeline to having a version tag

### BugFixes

- [X] [GR-02] The program hangs when launched on a trivial command.
    ```shell
    11:49:15 tyemirov@Vadyms-MacBook-Pro:~/Development/temirov/gripper - [master] $ go run ./... 10 -- ls -la
    {"level":"warn","ts":1760381358.564859,"caller":"runner/runner.go:120","msg":"kqueue tracker failed to start; continuing without it","error":"kevent add pid 55367: operation not supported"}
    total 32
    drwxr-xr-x@  9 tyemirov  staff   288 Oct 13 11:06 .
    drwxr-xr-x@ 33 tyemirov  staff  1056 Oct 13 10:18 ..
    drwxr-xr-x@ 13 tyemirov  staff   416 Oct 13 11:07 .git
    drwxr-xr-x   3 tyemirov  staff    96 Oct 13 10:58 cmd
    -rw-r--r--@  1 tyemirov  staff   498 Oct 13 11:03 go.mod
    -rw-r--r--@  1 tyemirov  staff  2229 Oct 13 11:03 go.sum
    drwxr-xr-x   9 tyemirov  staff   288 Oct 13 11:44 internal
    -rw-r--r--   1 tyemirov  staff   547 Oct 13 10:22 main.go
    -rw-r--r--@  1 tyemirov  staff  2368 Oct 13 11:06 README.md
    ^Csignal: interrupt
    ```
- [X] [GR-04] The program is a complete custerfuck of errors
    ```shell
    14:04:53 tyemirov@Vadyms-MacBook-Pro:~/Development/temirov/gripper - [wip] $ go run ./... 10 -- ls -la
    {"level":"warn","ts":1760389551.418806,"caller":"runner/runner.go:241","msg":"kqueue tracker failed to start; continuing without it","error":"kevent add pid 48347: operation not supported"}
    fatal error: all goroutines are asleep - deadlock!

    goroutine 1 [chan receive]:
    github.com/temirov/gripper/internal/proctrack.(*KqueueTracker).Close(0xc0000a8380)
            /Users/tyemirov/Development/temirov/gripper/internal/proctrack/kqueue_darwin.go:88 +0x2c
    github.com/temirov/gripper/internal/runner.(*executionManager).configureMacTracker(0xc00009a960, 0x1?)
            /Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:242 +0x165
    github.com/temirov/gripper/internal/runner.(*executionManager).run(0xc00009a960, {0xb80b788, 0xbacbf20})
            /Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:154 +0x285
    github.com/temirov/gripper/internal/runner.executionEngine.Execute({{0xb8083f8, 0xbacbf20}, {0xb808418, 0xbacbf20}, 0xb8047b8, 0xb8047c0}, {0xb80b788, 0xbacbf20}, {0x2540be400, 0x3b9aca00, ...})
            /Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:107 +0x268
    github.com/temirov/gripper/internal/runner.Executor.Execute(...)
            /Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:112
    github.com/temirov/gripper/internal/server.Service.Run({{{{0xb8083f8, 0xbacbf20}, {0xb808418, 0xbacbf20}, 0xb8047b8, 0xb8047c0}}}, 0x0?, {0xc00007e320, 0x2, 0x2})
            /Users/tyemirov/Development/temirov/gripper/internal/server/server.go:41 +0x174
    github.com/temirov/gripper/cmd.CLI.run({{{{{...}, {...}, 0xb8047b8, 0xb8047c0}}}}, 0x0?, {0xc0000a8240, 0x3, 0x4})
            /Users/tyemirov/Development/temirov/gripper/cmd/root.go:74 +0x2b9
    github.com/spf13/cobra.(*Command).execute(0xc0000de008, {0xc000020060, 0x4, 0x4})
            /Users/tyemirov/go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:985 +0xb34
    github.com/spf13/cobra.(*Command).ExecuteC(0xc0000de008)
            /Users/tyemirov/go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:1117 +0x44f
    github.com/spf13/cobra.(*Command).Execute(...)
            /Users/tyemirov/go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:1041
    github.com/temirov/gripper/cmd.Execute()
            /Users/tyemirov/Development/temirov/gripper/cmd/root.go:39 +0x16a
    main.main()
            /Users/tyemirov/Development/temirov/gripper/main.go:12 +0x13
    exit status 2
    total 64
    drwxr-xr-x@ 12 tyemirov  staff   384 Oct 13 14:04 .
    drwxr-xr-x@ 33 tyemirov  staff  1056 Oct 13 10:18 ..
    drwxr-xr-x@ 14 tyemirov  staff   448 Oct 13 14:04 .git
    -rw-r--r--@  1 tyemirov  staff  3599 Oct 13 14:04 AGENTS.md
    drwxr-xr-x@  4 tyemirov  staff   128 Oct 13 14:04 cmd
    -rw-r--r--@  1 tyemirov  staff   498 Oct 13 14:04 go.mod
    -rw-r--r--@  1 tyemirov  staff  2229 Oct 13 14:04 go.sum
    drwxr-xr-x@ 10 tyemirov  staff   320 Oct 13 14:04 internal
    -rw-r--r--@  1 tyemirov  staff   328 Oct 13 14:04 main.go
    -rw-r--r--@  1 tyemirov  staff  5871 Oct 13 14:05 NOTES.md
    -rw-r--r--@  1 tyemirov  staff  3471 Oct 13 14:04 PLAN.md
    -rw-r--r--@  1 tyemirov  staff  2368 Oct 13 11:06 README.md
    ```
- [X] [GR-05] Even the tests are failing
    ```shell
    14:05:51 tyemirov@Vadyms-MacBook-Pro:~/Development/temirov/gripper - [wip] $ go fmt ./... && go vet ./... && go test ./...
    ?       github.com/temirov/gripper      [no test files]
    {"level":"warn","ts":1760389628.07854,"caller":"runner/runner.go:241","msg":"kqueue tracker failed to start; continuing without it","error":"kevent add pid 49551: operation not supported"}
    {"level":"warn","ts":1760389628.078989,"caller":"runner/runner.go:241","msg":"kqueue tracker failed to start; continuing without it","error":"kevent add pid 49552: operation not supported"}
    --- FAIL: TestExecuteReturnsForShortCommand (2.00s)
        root_test.go:41: Execute did not return within 2s
    --- FAIL: TestExecutePropagatesTimeoutExitCode (2.00s)
        root_test.go:67: Execute did not return within 2s
    FAIL
    FAIL    github.com/temirov/gripper/cmd  5.386s
    ?       github.com/temirov/gripper/internal/cgroup      [no test files]
    ?       github.com/temirov/gripper/internal/procscan    [no test files]
    ?       github.com/temirov/gripper/internal/proctrack   [no test files]
    ?       github.com/temirov/gripper/internal/runner      [no test files]
    ?       github.com/temirov/gripper/internal/server      [no test files]
    ?       github.com/temirov/gripper/internal/signals     [no test files]
    {"level":"warn","ts":1760389628.432271,"caller":"runner/runner.go:241","msg":"kqueue tracker failed to start; continuing without it","error":"kevent add pid 49553: operation not supported"}
    integration-echo
    --- FAIL: TestRunServerPartCompletesWhenCommandFinishes (2.00s)
        server_test.go:192: RunServerPart did not return within 2s
        --- FAIL: TestRunServerPartCompletesWhenCommandFinishes/echo_completes_immediately (2.00s)
    panic: test executed panic(nil) or runtime.Goexit

    goroutine 9 [running]:
    testing.tRunner.func1.2({0x1f2bc60, 0x2141f00})
            /usr/local/opt/go/libexec/src/testing/testing.go:1872 +0x237
    testing.tRunner.func1()
            /usr/local/opt/go/libexec/src/testing/testing.go:1875 +0x35b
    runtime.Goexit()
            /usr/local/opt/go/libexec/src/runtime/panic.go:615 +0x5e
    testing.(*common).FailNow(0xc000003a40)
            /usr/local/opt/go/libexec/src/testing/testing.go:1013 +0x4a
    testing.(*common).Fatalf(0xc000003a40, {0x1eaad12?, 0x1f86680?}, {0xc000106ee8?, 0xc000028630?, 0x1f3c301?})
            /usr/local/opt/go/libexec/src/testing/testing.go:1219 +0x59
    github.com/temirov/gripper/internal/tests/integration_test.serverHarness.invoke({0xc00014a1c0?}, 0x2, {0xc0000783c0, 0x2, 0x2})
            /Users/tyemirov/Development/temirov/gripper/internal/tests/integration/server_test.go:55 +0x17c
    github.com/temirov/gripper/internal/tests/integration_test.TestRunServerPartCompletesWhenCommandFinishes.func1(0xc00014a1c0)
            /Users/tyemirov/Development/temirov/gripper/internal/tests/integration/server_test.go:192 +0x4c
    testing.tRunner(0xc00014a1c0, 0xc000012b80)
            /usr/local/opt/go/libexec/src/testing/testing.go:1934 +0xea
    created by testing.(*T).Run in goroutine 7
            /usr/local/opt/go/libexec/src/testing/testing.go:1997 +0x465
    FAIL    github.com/temirov/gripper/internal/tests/integration   2.736s
    ?       github.com/temirov/gripper/internal/util/exitcodes      [no test files]
    FAIL
    ```
- [ ] [GR-08] Fix errors: write a test to verify the correctness. if we don't need kqueue on MacOs we shouldnt have used it, if we do, it should ahve worked.
    ```
    15:08:46 tyemirov@Vadyms-MacBook-Pro:~/Development/temirov/gripper - [wip] $ go run ./... 10 -- ls -la
    {"level":"error","ts":1760393333.292462,"caller":"runner/runner.go:276","msg":"kqueue tracker failed to start; continuing without it","error":"kevent add pid 78720: operation not supported","stacktrace":"github.com/temirov/gripper/internal/runner.(*executionManager).configureMacTracker\n\t/Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:276\ngithub.com/temirov/gripper/internal/runner.(*executionManager).run\n\t/Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:187\ngithub.com/temirov/gripper/internal/runner.executionEngine.Execute\n\t/Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:134\ngithub.com/temirov/gripper/internal/runner.Executor.Execute\n\t/Users/tyemirov/Development/temirov/gripper/internal/runner/runner.go:139\ngithub.com/temirov/gripper/internal/server.Service.Run\n\t/Users/tyemirov/Development/temirov/gripper/internal/server/server.go:41\ngithub.com/temirov/gripper/cmd.CLI.run\n\t/Users/tyemirov/Development/temirov/gripper/cmd/root.go:74\ngithub.com/spf13/cobra.(*Command).execute\n\t/Users/tyemirov/go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:985\ngithub.com/spf13/cobra.(*Command).ExecuteC\n\t/Users/tyemirov/go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:1117\ngithub.com/spf13/cobra.(*Command).Execute\n\t/Users/tyemirov/go/pkg/mod/github.com/spf13/cobra@v1.8.1/command.go:1041\ngithub.com/temirov/gripper/cmd.Execute\n\t/Users/tyemirov/Development/temirov/gripper/cmd/root.go:39\nmain.main\n\t/Users/tyemirov/Development/temirov/gripper/main.go:12\nruntime.main\n\t/usr/local/opt/go/libexec/src/runtime/proc.go:285"}
    total 80
    drwxr-xr-x@ 13 tyemirov  staff    416 Oct 13 15:08 .
    drwxr-xr-x@ 33 tyemirov  staff   1056 Oct 13 10:18 ..
    drwxr-xr-x@ 14 tyemirov  staff    448 Oct 13 15:08 .git
    drwxr-xr-x@  3 tyemirov  staff     96 Oct 13 15:08 .github
    -rw-r--r--@  1 tyemirov  staff   3599 Oct 13 14:04 AGENTS.md
    drwxr-xr-x@  4 tyemirov  staff    128 Oct 13 15:08 cmd
    -rw-r--r--@  1 tyemirov  staff    498 Oct 13 14:04 go.mod
    -rw-r--r--@  1 tyemirov  staff   2229 Oct 13 14:04 go.sum
    drwxr-xr-x@ 10 tyemirov  staff    320 Oct 13 14:04 internal
    -rw-r--r--@  1 tyemirov  staff    328 Oct 13 14:04 main.go
    -rw-r--r--@  1 tyemirov  staff  15495 Oct 13 15:08 NOTES.md
    -rw-r--r--@  1 tyemirov  staff   2972 Oct 13 15:08 PLAN.md
    -rw-r--r--@  1 tyemirov  staff   2945 Oct 13 15:08 README.md
    ```

### Maintenance
