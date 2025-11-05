package cmd_test

import (
	"os"
	"testing"
	"time"

	"github.com/tyemirov/gripper/cmd"
	"github.com/tyemirov/gripper/internal/util/exitcodes"
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

func TestExecuteBehavior(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		assertError func(*testing.T, error)
	}{
		{
			name: "short command completes",
			args: []string{cliProgramName, cliTimeoutSeconds, cliSeparatorToken, echoExecutable, echoArgument},
			assertError: func(t *testing.T, execErr error) {
				if execErr != nil {
					t.Fatalf("Execute returned error: %v", execErr)
				}
			},
		},
		{
			name: "timeout propagates exit code",
			args: []string{cliProgramName, cliTimeoutSeconds, cliSeparatorToken, sleepExecutable, sleepDurationSeconds},
			assertError: func(t *testing.T, execErr error) {
				exitError, ok := execErr.(*exitcodes.ExitError)
				if !ok {
					t.Fatalf("expected ExitError, received %v", execErr)
				}
				if exitError.Code != exitcodes.ExitTimeout {
					t.Fatalf("unexpected exit code: %d", exitError.Code)
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			originalArgs := os.Args
			os.Args = append([]string(nil), testCase.args...)
			t.Cleanup(func() { os.Args = originalArgs })

			resultChannel := make(chan error, 1)
			go func() {
				resultChannel <- cmd.Execute()
			}()

			select {
			case execErr := <-resultChannel:
				testCase.assertError(t, execErr)
			case <-time.After(executeWaitThreshold):
				t.Fatalf("Execute did not return within %s", executeWaitThreshold)
			}
		})
	}
}
