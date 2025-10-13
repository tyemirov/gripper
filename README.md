# gripper

`gripper` is a small and uncompromising utility: it runs a command and guarantees that **after N seconds** the command, all its children, and all associated resources (open files, sockets, subprocesses) are terminated.

Think of it as a **grim reaper with precision timing** — reliable, cross-platform, and strict.

---

## Why `gripper`?

Most timeouts in shells, CI systems, or programming libraries are _advisory_: they may leave orphaned processes, open file descriptors, or zombie subprocesses.

`gripper` is different:

- **Hard guarantee:** after `N` seconds, the command is gone. Not “maybe gone,” not “cleaned up later.”
- **Cross-platform:**

  - **Linux:** Uses cgroup v2 `cgroup.kill` when available (atomic kill).
  - **macOS:** Uses BSD `kqueue` process tracking to follow forks/execs.

- **Fast enforcement:** grace time is internal and capped at 1 second. At most, there’s a one-second window between timeout and full termination.

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

## License

MIT License © Vadym Tyemirov
# gripper
