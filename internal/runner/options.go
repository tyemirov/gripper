// Package runner orchestrates launching the target process and enforcing
// timeout semantics with a fixed 1s enforcement window after timeout.
package runner

import "time"

// FixedEnforceWindow is the non-configurable window after timeout within which
// gripper guarantees that all descendant processes are terminated.
const FixedEnforceWindow = 1 * time.Second

// Options configures a single run invocation.
type Options struct {
	Timeout        time.Duration // hard wall-clock timeout from launch
	EnforceWindow  time.Duration // fixed 1s window (always FixedEnforceWindow)
	CommandAndArgs []string      // command to execute + args
}
