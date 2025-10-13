// Package exitcodes centralizes process exit codes.
//
// ExitTimeout (124) matches GNU timeout semantics.
package exitcodes

// ExitError lets Cobra propagate a specific non-zero code without printing usage.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string { return "exit requested" }

const (
	ExitSuccess      = 0   // normal completion
	ExitTimeout      = 124 // killed due to timeout
	ExitInvalidUsage = 2   // bad flags/args
	ExitRuntimeError = 1   // unexpected runtime failure
)
