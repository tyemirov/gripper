package integration_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/gripper/internal/server"
	"github.com/tyemirov/gripper/internal/util/exitcodes"
)

const (
	echoExecutablePath         = "/bin/echo"
	echoArgumentText           = "integration-echo"
	successTimeoutSeconds      = 2
	runCompletionWaitThreshold = 2 * time.Second
	fileReadRetryDelay         = 25 * time.Millisecond
	timeoutEnforcementSeconds  = 1
	descendantSleepSeconds     = 20
	pidFileName                = "child.pid"
)

type runOutcome struct {
	exitCode int
	runError error
}

type serverHarness struct {
	testingT *testing.T
}

func newServerHarness(testingT *testing.T) serverHarness {
	testingT.Helper()
	return serverHarness{testingT: testingT}
}

func (h serverHarness) invoke(timeoutSeconds int, command []string) runOutcome {
	h.testingT.Helper()
	outcomeChannel := make(chan runOutcome, 1)
	go func() {
		exitCode, runError := server.RunServerPart(timeoutSeconds, command)
		outcomeChannel <- runOutcome{exitCode: exitCode, runError: runError}
	}()

	select {
	case outcome := <-outcomeChannel:
		return outcome
	case <-time.After(runCompletionWaitThreshold):
		h.testingT.Fatalf("RunServerPart did not return within %s", runCompletionWaitThreshold)
	}

	return runOutcome{}
}

func (h serverHarness) readFile(path string) string {
	h.testingT.Helper()
	deadline := time.Now().Add(runCompletionWaitThreshold)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
		if time.Now().After(deadline) {
			h.testingT.Fatalf("failed reading %s: %v", path, err)
		}
		time.Sleep(fileReadRetryDelay)
	}
}

func (h serverHarness) parsePID(text string) int {
	h.testingT.Helper()
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		h.testingT.Fatalf("invalid pid text %q: %v", text, err)
	}
	return value
}

func (h serverHarness) assertProcessAbsent(pid int) {
	h.testingT.Helper()
	procPath := filepath.Join("/proc", strconv.Itoa(pid))
	deadline := time.Now().Add(runCompletionWaitThreshold)
	for {
		_, err := os.Stat(procPath)
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if err == nil && h.isZombie(pid) {
			return
		}
		if time.Now().After(deadline) {
			h.testingT.Fatalf("descendant pid %d still present at %s", pid, procPath)
		}
		time.Sleep(fileReadRetryDelay)
	}
}

func (h serverHarness) isZombie(pid int) bool {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return false
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return false
	}
	return fields[2] == "Z"
}

func buildDescendantHelper(t *testing.T, directory string) string {
	t.Helper()
	source := `package main

import (
"fmt"
"os"
"os/exec"
"os/signal"
"strconv"
"syscall"
)

func main() {
signal.Ignore(syscall.SIGTERM)
if len(os.Args) < 3 {
os.Exit(2)
}
pidFile := os.Args[1]
sleepSeconds, err := strconv.Atoi(os.Args[2])
if err != nil {
fmt.Fprintln(os.Stderr, "invalid sleep seconds", err)
os.Exit(2)
}
child := exec.Command("/bin/sleep", strconv.Itoa(sleepSeconds))
if err := child.Start(); err != nil {
fmt.Fprintln(os.Stderr, "start child", err)
os.Exit(2)
}
if err := os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
fmt.Fprintln(os.Stderr, "write pid", err)
child.Process.Kill()
os.Exit(2)
}
_ = child.Wait()
select {}
}
`

	sourcePath := filepath.Join(directory, "descendant_helper.go")
	if writeErr := os.WriteFile(sourcePath, []byte(source), 0o600); writeErr != nil {
		t.Fatalf("failed writing helper source: %v", writeErr)
	}
	binaryPath := filepath.Join(directory, "descendant-helper")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	buildCmd.Dir = directory
	buildCmd.Env = os.Environ()
	if output, buildErr := buildCmd.CombinedOutput(); buildErr != nil {
		t.Fatalf("build helper failed: %v (%s)", buildErr, string(output))
	}
	return binaryPath
}

func TestRunServerPartCompletesWhenCommandFinishes(t *testing.T) {
	t.Parallel()

	harness := newServerHarness(t)

	testCases := []struct {
		name          string
		command       []string
		timeoutSecond int
	}{
		{
			name:          "echo completes immediately",
			command:       []string{echoExecutablePath, echoArgumentText},
			timeoutSecond: successTimeoutSeconds,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			outcome := harness.invoke(testCase.timeoutSecond, testCase.command)

			if outcome.runError != nil {
				t.Fatalf("RunServerPart returned error: %v", outcome.runError)
			}
			if outcome.exitCode != exitcodes.ExitSuccess {
				t.Fatalf("unexpected exit code: %d", outcome.exitCode)
			}
		})
	}
}

func TestRunServerPartKillsDescendantsOnTimeout(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("descendant inspection relies on /proc")
	}
	t.Parallel()

	harness := newServerHarness(t)
	tempDir := t.TempDir()
	pidFilePath := filepath.Join(tempDir, pidFileName)

	helperBinary := buildDescendantHelper(t, tempDir)
	command := []string{helperBinary, pidFilePath, strconv.Itoa(descendantSleepSeconds)}

	outcome := harness.invoke(timeoutEnforcementSeconds, command)
	if outcome.exitCode != exitcodes.ExitTimeout {
		t.Fatalf("expected timeout exit code, received %d", outcome.exitCode)
	}

	childPIDText := harness.readFile(pidFilePath)
	childPID := harness.parsePID(childPIDText)
	harness.assertProcessAbsent(childPID)
}
