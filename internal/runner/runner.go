// runner.go: core execution + enforcement
//
// Platform notes:
//
//	Linux: prefer cgroup v2 (cgroup.kill). Fallback: process group + /proc scan.
//	macOS: kqueue tracker when available; fallback: process group + ps sweep.
//
// Enforcement semantics:
//   - At exactly Timeout after start, we initiate termination.
//   - From that instant, we bound all enforcement to EnforceWindow (1s). We send
//     SIGTERM briefly, then SIGKILL, and perform a final sweep. We do not allow
//     post-timeout handling to exceed 1s.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/temirov/gripper/internal/cgroup"
	"github.com/temirov/gripper/internal/procscan"
	"github.com/temirov/gripper/internal/proctrack"
	"github.com/temirov/gripper/internal/signals"
	"github.com/temirov/gripper/internal/util/exitcodes"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type runContext struct {
	Logger                 *zap.Logger
	Options                Options
	ChildPid               int
	ChildExitStatusChannel chan int
	CgroupPath             string
	UsingCgroup            bool
	MacTracker             proctrack.Tracker
}

func Run(options Options) (int, error) {
	logger, loggerErr := zap.NewProduction()
	if loggerErr != nil {
		return exitcodes.ExitRuntimeError, fmt.Errorf("logger: %w", loggerErr)
	}
	defer logger.Sync()

	if options.Timeout <= 0 {
		return exitcodes.ExitInvalidUsage, fmt.Errorf("timeout must be > 0")
	}
	// Enforce non-configurable 1s window.
	options.EnforceWindow = FixedEnforceWindow

	executableName := options.CommandAndArgs[0]
	executableArgs := options.CommandAndArgs[1:]

	var execCommand *exec.Cmd
	if shouldUseShell(options.CommandAndArgs) {
		userShell := pickUserShell()
		// Build a safe command line for the shell.
		cmdline := shellJoin(options.CommandAndArgs)
		// Use login/interactive only if you *require* rc files; -lc covers aliases/functions in typical setups.
		execCommand = exec.Command(userShell, "-lc", cmdline)
	} else {
		execCommand = exec.Command(executableName, executableArgs...)
	}

	execCommand.Stdout = os.Stdout
	execCommand.Stderr = os.Stderr
	execCommand.Stdin = os.Stdin
	execCommand.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	ctx := &runContext{
		Logger:                 logger,
		Options:                options,
		ChildExitStatusChannel: make(chan int, 1),
	}

	// Linux: set up cgroup v2 if available.
	if runtime.GOOS == "linux" {
		if available, _ := cgroup.Available(); available {
			if pathValue, createErr := cgroup.CreateUnique(); createErr == nil {
				ctx.CgroupPath = pathValue
				ctx.UsingCgroup = true
				defer cgroup.Remove(pathValue)
			} else {
				logger.Warn("cgroup create failed; falling back", zap.Error(createErr))
			}
		}
	}

	// macOS: set up kqueue tracker if possible.
	if runtime.GOOS == "darwin" {
		if tracker, trackErr := proctrack.NewKqueueTracker(); trackErr == nil {
			ctx.MacTracker = tracker
			defer ctx.MacTracker.Close()
		} else {
			logger.Warn("kqueue tracker unavailable; falling back", zap.Error(trackErr))
		}
	}

	if startErr := execCommand.Start(); startErr != nil {
		ctx.Logger.Error("start failed", zap.Error(startErr))
		return exitcodes.ExitRuntimeError, startErr
	}
	ctx.ChildPid = execCommand.Process.Pid

	// Place into cgroup when used.
	if ctx.UsingCgroup {
		if moveErr := cgroup.AddPid(ctx.CgroupPath, ctx.ChildPid); moveErr != nil {
			ctx.Logger.Warn("failed moving pid into cgroup; falling back", zap.Error(moveErr))
			ctx.UsingCgroup = false
		}
	}

	// macOS tracker: start tracking root.
	if ctx.MacTracker != nil {
		if err := ctx.MacTracker.StartTrackingRoot(ctx.ChildPid); err != nil {
			ctx.Logger.Warn("kqueue tracker failed to start; continuing without it", zap.Error(err))
			ctx.MacTracker = nil
		}
	}

	// Reap routine
	go func() {
		waitErr := execCommand.Wait()
		if waitErr != nil {
			if exitError, ok := waitErr.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					ctx.ChildExitStatusChannel <- status.ExitStatus()
					return
				}
			}
			ctx.ChildExitStatusChannel <- exitcodes.ExitRuntimeError
			return
		}
		ctx.ChildExitStatusChannel <- exitcodes.ExitSuccess
	}()

	// Forward incoming signals to the job.
	signalsStop := make(chan struct{})
	signalsDone := make(chan struct{})
	go signals.ForwardLoop(signals.ForwardConfig{
		ChildPid:    ctx.ChildPid,
		UsingCgroup: ctx.UsingCgroup,
		CgroupPath:  ctx.CgroupPath,
		Stop:        signalsStop,
		Done:        signalsDone,
	})

	// Linux fallback scanner (non-cgroup)
	var scannerStop chan struct{}
	var scannerDone chan struct{}
	if runtime.GOOS == "linux" && !ctx.UsingCgroup {
		scannerStop = make(chan struct{})
		scannerDone = make(chan struct{})
		go procscan.ScannerLoop(procscan.ScannerConfig{
			RootPid:    ctx.ChildPid,
			PollPeriod: 150 * time.Millisecond,
			Stop:       scannerStop,
			Done:       scannerDone,
		})
	}

	timeoutTimer := time.NewTimer(ctx.Options.Timeout)
	defer timeoutTimer.Stop()

	finalExitCode := exitcodes.ExitRuntimeError

	select {
	case code := <-ctx.ChildExitStatusChannel:
		finalExitCode = code

	case <-timeoutTimer.C:
		// Begin fixed 1s enforcement window.
		enforcementDeadline := time.Now().Add(ctx.Options.EnforceWindow)
		ctx.Logger.Warn("timeout reached; enforcing 1s window", zap.Duration("window", ctx.Options.EnforceWindow))

		// 1) Gentle nudge: TERM, short pause (<= 200ms)
		signalTERM(ctx)
		sleepUntil(enforcementDeadline, 200*time.Millisecond)

		// 2) Hard kill path
		signalKILL(ctx)

		// 3) Stop background helpers promptly
		closeAndWait(signalsStop, signalsDone)
		if scannerStop != nil {
			closeAndWait(scannerStop, scannerDone)
		}

		// 4) Wait for the child to exit, but never exceed the 1s window.
		remaining := time.Until(enforcementDeadline)
		if remaining <= 0 {
			finalExitCode = exitcodes.ExitTimeout
		} else {
			select {
			case code := <-ctx.ChildExitStatusChannel:
				finalExitCode = code
			case <-time.After(remaining):
				finalExitCode = exitcodes.ExitTimeout
			}
		}
	}

	// Normal shutdown (if not already closed)
	closeAndWait(signalsStop, signalsDone)
	if scannerStop != nil {
		closeAndWait(scannerStop, scannerDone)
	}

	return finalExitCode, nil
}

func closeAndWait(stop chan struct{}, done chan struct{}) {
	if stop == nil || done == nil {
		return
	}
	// If it's already done, don't touch the stop channel.
	select {
	case <-done:
		return
	default:
	}

	// Signal the goroutine to stop.
	close(stop)

	// Wait for it to acknowledge, but don't block forever (avoid rare races where
	// the goroutine hasn't started yet or is stuck in a syscall/select corner case).
	select {
	case <-done:
		return
	case <-time.After(500 * time.Millisecond):
		// Best-effort shutdown; proceed to exit to avoid hangs.
		return
	}
}

func signalTERM(ctx *runContext) {
	if ctx.UsingCgroup {
		_ = cgroup.SignalAll(ctx.CgroupPath, unix.SIGTERM)
		return
	}
	if runtime.GOOS == "darwin" && ctx.MacTracker != nil {
		_ = ctx.MacTracker.SignalAll(syscall.SIGTERM)
		return
	}
	_ = syscall.Kill(-ctx.ChildPid, syscall.SIGTERM)
}

func signalKILL(ctx *runContext) {
	if ctx.UsingCgroup {
		if cgroup.HasKillFile(ctx.CgroupPath) {
			_ = cgroup.Kill(ctx.CgroupPath)
			return
		}
		_ = cgroup.SignalAll(ctx.CgroupPath, unix.SIGKILL)
		return
	}
	if runtime.GOOS == "darwin" && ctx.MacTracker != nil {
		_ = ctx.MacTracker.SignalAll(syscall.SIGKILL)
		// Final sweep as belt-and-suspenders.
		if descendants, _ := proctrack.FallbackDescendants(ctx.MacTracker.RootPid()); len(descendants) > 0 {
			for _, pid := range descendants {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
		return
	}
	// Generic fallback: kill entire process group, then sweep.
	_ = syscall.Kill(-ctx.ChildPid, syscall.SIGKILL)
	if runtime.GOOS == "linux" {
		if descendants, err := procscan.FindDescendants(ctx.ChildPid); err == nil {
			for _, pid := range descendants {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	} else if runtime.GOOS == "darwin" {
		if descendants, err := proctrack.FallbackDescendants(ctx.ChildPid); err == nil {
			for _, pid := range descendants {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
}

func sleepUntil(deadline time.Time, maxPause time.Duration) {
	if maxPause <= 0 {
		return
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return
	}
	if maxPause > remaining {
		time.Sleep(remaining)
		return
	}
	time.Sleep(maxPause)
}
