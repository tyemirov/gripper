package cmd_test

import (
	"os"
	"testing"
	"time"

	"github.com/temirov/gripper/cmd"
	"github.com/temirov/gripper/internal/util/exitcodes"
)

const (
	executeWaitThreshold = 2 * time.Second
	cliProgramName       = "gripper"
	cliTimeoutSeconds    = "1"
	cliSeparatorToken    = "--"
	echoExecutable       = "/bin/echo"
	echoArgument         = "cli-test"
	sleepExecutable      = "/bin/sleep"
	sleepDurationSeconds = "5"
)

func TestExecuteReturnsForShortCommand(t *testing.T) {
	t.Parallel()

	originalArgs := os.Args
	os.Args = []string{cliProgramName, cliTimeoutSeconds, cliSeparatorToken, echoExecutable, echoArgument}
	defer func() { os.Args = originalArgs }()

	resultChannel := make(chan error, 1)
	go func() {
		resultChannel <- cmd.Execute()
	}()

	select {
	case execErr := <-resultChannel:
		if execErr != nil {
			t.Fatalf("Execute returned error: %v", execErr)
		}
	case <-time.After(executeWaitThreshold):
		t.Fatalf("Execute did not return within %s", executeWaitThreshold)
	}
}

func TestExecutePropagatesTimeoutExitCode(t *testing.T) {
	t.Parallel()

	originalArgs := os.Args
	os.Args = []string{cliProgramName, cliTimeoutSeconds, cliSeparatorToken, sleepExecutable, sleepDurationSeconds}
	defer func() { os.Args = originalArgs }()

	resultChannel := make(chan error, 1)
	go func() {
		resultChannel <- cmd.Execute()
	}()

	select {
	case execErr := <-resultChannel:
		exitError, ok := execErr.(*exitcodes.ExitError)
		if !ok {
			t.Fatalf("expected ExitError, received %v", execErr)
		}
		if exitError.Code != exitcodes.ExitTimeout {
			t.Fatalf("unexpected exit code: %d", exitError.Code)
		}
	case <-time.After(executeWaitThreshold):
		t.Fatalf("Execute did not return within %s", executeWaitThreshold)
	}
}
