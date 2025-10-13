package runner_test

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/temirov/gripper/internal/proctrack"
	"github.com/temirov/gripper/internal/runner"
	"github.com/temirov/gripper/internal/util/exitcodes"
)

type executeResult struct {
	exitCode int
	err      error
}

type blockingTrackerFactory struct {
	tracker *blockingTracker
}

func newBlockingTrackerFactory() *blockingTrackerFactory {
	return &blockingTrackerFactory{}
}

func (factory *blockingTrackerFactory) NewTracker() (proctrack.Tracker, error) {
	tracker := newBlockingTracker()
	factory.tracker = tracker
	return tracker, nil
}

type blockingTracker struct {
	allowClose chan struct{}
	closeDone  chan struct{}
}

func newBlockingTracker() *blockingTracker {
	return &blockingTracker{
		allowClose: make(chan struct{}),
		closeDone:  make(chan struct{}, 1),
	}
}

func (tracker *blockingTracker) StartTrackingRoot(int) error {
	return errors.New("start failure")
}

func (tracker *blockingTracker) RootPid() int {
	return 0
}

func (tracker *blockingTracker) SignalAll(syscall.Signal) error {
	return nil
}

func (tracker *blockingTracker) Close() error {
	<-tracker.allowClose
	tracker.closeDone <- struct{}{}
	return nil
}

func TestExecuteReturnsWhenMacTrackerStartFails(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		command []string
	}{{
		name:    "mac tracker start failure does not deadlock",
		command: []string{"/bin/echo", "tracker-integration"},
	}}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			factory := newBlockingTrackerFactory()
			executor := runner.NewExecutor(runner.WithMacTrackerFactory(factory))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			options := runner.Options{
				Timeout:        time.Second,
				CommandAndArgs: append([]string(nil), testCase.command...),
			}

			resultChannel := make(chan executeResult, 1)
			go func() {
				exitCode, execErr := executor.Execute(ctx, options)
				resultChannel <- executeResult{exitCode: exitCode, err: execErr}
			}()

			select {
			case result := <-resultChannel:
				if result.err != nil {
					t.Fatalf("Execute returned error: %v", result.err)
				}
				if result.exitCode != exitcodes.ExitSuccess {
					t.Fatalf("unexpected exit code: %d", result.exitCode)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("Execute did not return within 2 seconds")
			}

			if factory.tracker == nil {
				t.Fatalf("tracker was not constructed")
			}
			close(factory.tracker.allowClose)

			select {
			case <-factory.tracker.closeDone:
			case <-time.After(time.Second):
				t.Fatalf("tracker Close was not invoked")
			}
		})
	}
}
