//go:build !darwin

// Package proctrack provides platform stubs for non-macOS builds so the
// import path github.com/temirov/gripper/internal/proctrack always resolves.
package proctrack

import "syscall"

// Tracker is kept identical to the Darwin implementation so shared code can compile.
type Tracker interface {
	StartTrackingRoot(rootProcessID int) error
	RootPid() int
	SignalAll(signalToSend syscall.Signal) error
	Close() error
}

// NewKqueueTracker is not available off macOS; return a descriptive error.
func NewKqueueTracker() (Tracker, error) {
	return nil, ErrPlatformUnsupported
}

// ErrPlatformUnsupported indicates the feature is not supported on this OS.
var ErrPlatformUnsupported = errPlatformUnsupported("proctrack: not supported on this platform")

type errPlatformUnsupported string

func (errorValue errPlatformUnsupported) Error() string { return string(errorValue) }

// FallbackDescendants is a no-op stub on non-macOS; the Linux path uses procscan instead.
func FallbackDescendants(rootProcessID int) ([]int, error) {
	return nil, nil
}
