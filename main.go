// Package main provides the gripper CLI entry point.
//
// gripper runs an arbitrary command and guarantees that, after a configurable
// wall-clock timeout, the command and all of its children are terminated.
// On Linux, the strongest guarantee is provided via cgroup v2. On macOS,
// gripper uses a kqueue-based process tracker to follow forks/execs/exits
// and kill the entire tracked set upon timeout.
package main

import (
	"os"

	"github.com/temirov/gripper/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
