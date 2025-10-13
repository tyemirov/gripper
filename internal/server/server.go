package server

import (
	"fmt"
	"time"

	"github.com/temirov/gripper/internal/runner"
	"github.com/temirov/gripper/internal/util/exitcodes"
)

// RunServerPart is the "server" half of the app. It receives the post-- tail
// (the command and its arguments) and runs it under a hard timeout.
//
// Semantics:
//   - After exactly timeoutSeconds from process start, termination begins.
//   - All descendants are guaranteed killed within a fixed 1s enforcement window.
func RunServerPart(timeoutSeconds int, serverArgs []string) (int, error) {
	if timeoutSeconds <= 0 {
		return exitcodes.ExitInvalidUsage, fmt.Errorf("timeoutSeconds must be > 0")
	}
	if len(serverArgs) == 0 {
		return exitcodes.ExitInvalidUsage, fmt.Errorf("missing command after --")
	}

	options := runner.Options{
		Timeout:        time.Duration(timeoutSeconds) * time.Second,
		CommandAndArgs: serverArgs,
		// EnforceWindow is set to the fixed 1s by runner.
	}

	return runner.Run(options)
}
