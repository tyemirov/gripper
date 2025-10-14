# gripper

`gripper` is a precise timeout enforcer: run a command and know that after **N seconds** the entire process tree is gone—children, file descriptors, sockets, everything.

Think of it as a grim reaper carrying a stopwatch: deterministic cleanup across Unix-like systems.

---

## Why `gripper`?

Most timeouts in shells, CI jobs, or libraries are only advisory and frequently leave orphaned processes or open descriptors behind.

`gripper` is different:

- **Hard cutoff:** once the deadline hits, the entire command tree is eliminated. No lingering subprocesses, no surprise cleanup tasks later.
- **Cross-platform guarantees:**
  - **Linux:** leverages cgroup v2 `cgroup.kill` for atomic, whole-tree termination.
  - **macOS:** follows forks and execs through BSD `kqueue` tracking so nothing escapes.
  - **Windows:** not supported because the necessary process-control primitives are unavailable.
- **Fast enforcement:** the internal grace period is capped at one second, so the kill happens almost immediately after timeout.

---

## Usage

```shell
gripper <seconds> -- <command> [args...]
```

### Examples

```shell
# Run "npm test" but kill it and all its children after 20s
gripper 20 -- npm test

# Run a long-running Python script with strict cutoff
gripper 60 -- python crawl.py
```

That’s it. No knobs, no optional flags — one number, one command, and a guarantee.

---

## Use in Agentic Coding

When building **agentic coding systems** — tools that launch compilers, interpreters, linters, tests, or even browsers — runaway processes are fatal.

`gripper` gives you confidence that:

- **Automated agents don’t leak:** every spawned process tree is bounded.
- **CI/CD pipelines stay predictable:** no hangs, no zombies.
- **Interactive experimentation stays safe:** try dangerous or buggy commands without fear of your system being left in a bad state.

Example integration in an agentic workflow:

```shell
# Agent decides to test code but ensures no hang beyond 15s
gripper 15 -- go test ./...
```

---

## Installation

```shell
go install github.com/temirov/gripper@latest
```

The binary `gripper` will be in your `$GOPATH/bin` or `$HOME/go/bin`.

---

## Exit Codes

- `0` — command finished successfully before timeout
- `124` — killed by `gripper` due to timeout (matches GNU `timeout`)
- `1` — runtime error in `gripper` itself

---

## Development

- Run the full validation suite locally with `go fmt ./... && go vet ./... && go test ./...`.
- Integration tests live under `internal/tests/integration` and exercise real process trees, including timeout enforcement and descendant cleanup.
- Continuous integration runs on GitHub Actions (`.github/workflows/ci.yml`) and cross-compiles binaries for Linux and macOS in addition to enforcing formatting, vetting, and tests.

---

## License

MIT License © Vadym Tyemirov
