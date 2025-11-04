package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/tyemirov/gripper/internal/cgroup"
	"github.com/tyemirov/gripper/internal/procscan"
	"github.com/tyemirov/gripper/internal/proctrack"
	"github.com/tyemirov/gripper/internal/signals"
	"github.com/tyemirov/gripper/internal/util/exitcodes"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

const (
	linuxOSName                   = "linux"
	darwinOSName                  = "darwin"
	goroutineShutdownGrace        = 500 * time.Millisecond
	enforcementPauseDuration      = 200 * time.Millisecond
	scannerPollPeriod             = 150 * time.Millisecond
	logMessageStartFailed         = "start failed"
	logMessageCgroupCreateFailed  = "cgroup create failed; falling back"
	logMessageCgroupJoinFailed    = "failed moving pid into cgroup; falling back"
	logMessageTrackerUnavailable  = "kqueue tracker unavailable; falling back"
	logMessageTrackerStartFailed  = "kqueue tracker failed to start; continuing without it"
	logMessageTrackerCloseDelayed = "mac tracker close did not complete before deadline"
)

type timerBuilder interface {
	NewTimer(d time.Duration) *time.Timer
}

type executionEngine struct {
	loggerFactory      LoggerFactory
	timerFactory       timerBuilder
	macTrackerFactory  MacTrackerFactory
	macTrackingEnabled bool
	now                func() time.Time
	sleep              func(time.Duration)
}

type zapLoggerFactory struct{}

type systemTimerFactory struct{}

// LoggerFactory constructs zap loggers for the executor.
type LoggerFactory interface {
	Build() (*zap.Logger, error)
}

// MacTrackerFactory constructs macOS process trackers.
type MacTrackerFactory interface {
	NewTracker() (proctrack.Tracker, error)
}

type kqueueTrackerFactory struct{}

// Executor exposes the configurable execution engine used by higher layers.
type Executor struct {
	engine executionEngine
}

func newExecutionEngine() executionEngine {
	return executionEngine{
		loggerFactory:      zapLoggerFactory{},
		timerFactory:       systemTimerFactory{},
		macTrackerFactory:  kqueueTrackerFactory{},
		macTrackingEnabled: runtime.GOOS == darwinOSName,
		now:                time.Now,
		sleep:              time.Sleep,
	}
}

// ExecutorOption customizes executor construction.
type ExecutorOption func(*executionEngine)

// WithMacTrackerFactory overrides the macOS tracker factory and forces mac tracking to be enabled.
func WithMacTrackerFactory(factory MacTrackerFactory) ExecutorOption {
	return func(engine *executionEngine) {
		engine.macTrackerFactory = factory
		engine.macTrackingEnabled = true
	}
}

// WithLoggerFactory overrides the logger factory used by the executor.
func WithLoggerFactory(factory LoggerFactory) ExecutorOption {
	return func(engine *executionEngine) {
		engine.loggerFactory = factory
	}
}

// NewExecutor constructs an Executor with optional dependency overrides.
func NewExecutor(options ...ExecutorOption) Executor {
	engine := newExecutionEngine()
	for _, option := range options {
		option(&engine)
	}
	return Executor{engine: engine}
}

// Run executes the configured command under a strict timeout and returns the
// resulting exit code.
func Run(options Options) (int, error) {
	return NewExecutor().Execute(context.Background(), options)
}

// Execute launches the command and enforces the timeout using the provided
// context for cancellation.
func (engine executionEngine) Execute(ctx context.Context, options Options) (int, error) {
	if options.Timeout <= 0 {
		return exitcodes.ExitInvalidUsage, fmt.Errorf("timeout must be > 0")
	}
	if len(options.CommandAndArgs) == 0 {
		return exitcodes.ExitInvalidUsage, errors.New("command is required")
	}

	options.EnforceWindow = FixedEnforceWindow

	logger, loggerErr := engine.loggerFactory.Build()
	if loggerErr != nil {
		return exitcodes.ExitRuntimeError, fmt.Errorf("logger: %w", loggerErr)
	}
	defer logger.Sync()

	manager := executionManager{
		options:        options,
		logger:         logger,
		timerFactory:   engine.timerFactory,
		trackerFactory: engine.macTrackerFactory,
		macTracking:    engine.macTrackingEnabled,
		now:            engine.now,
		sleep:          engine.sleep,
	}

	return manager.run(ctx)
}

// Execute launches the command through the encapsulated execution engine.
func (executor Executor) Execute(ctx context.Context, options Options) (int, error) {
	return executor.engine.Execute(ctx, options)
}

func (zapLoggerFactory) Build() (*zap.Logger, error) {
	return zap.NewProduction()
}

func (systemTimerFactory) NewTimer(d time.Duration) *time.Timer {
	return time.NewTimer(d)
}

func (kqueueTrackerFactory) NewTracker() (proctrack.Tracker, error) {
	return proctrack.NewKqueueTracker()
}

type executionManager struct {
	options        Options
	logger         *zap.Logger
	timerFactory   timerBuilder
	trackerFactory MacTrackerFactory
	macTracking    bool
	now            func() time.Time
	sleep          func(time.Duration)
	command        *exec.Cmd
	childExit      chan int
	usingCgroup    bool
	cgroupPath     string
	macTracker     proctrack.Tracker
	signalRoutine  goroutineCoordinator
	scannerRoutine *goroutineCoordinator
}

func (manager *executionManager) run(ctx context.Context) (int, error) {
	if err := manager.configureCommand(); err != nil {
		return exitcodes.ExitRuntimeError, err
	}

	manager.preparePlatform()
	defer manager.cleanupPlatform()

	if err := manager.command.Start(); err != nil {
		manager.logger.Error(logMessageStartFailed, zap.Error(err))
		return exitcodes.ExitRuntimeError, err
	}

	childPid := manager.command.Process.Pid

	manager.configureCgroupMembership(childPid)
	manager.configureMacTracker(childPid)
	go manager.captureExitStatus()

	manager.launchBackgroundRoutines(childPid)
	defer manager.stopBackgroundRoutines()

	timeoutTimer := manager.timerFactory.NewTimer(manager.options.Timeout)
	defer timeoutTimer.Stop()

	select {
	case <-ctx.Done():
		manager.terminateChildImmediately(childPid)
		return exitcodes.ExitRuntimeError, ctx.Err()
	case exitCode := <-manager.childExit:
		return exitCode, nil
	case <-timeoutTimer.C:
		exitCode := manager.enforceTimeout(childPid)
		return exitCode, nil
	}
}

func (manager *executionManager) configureCommand() error {
	argv := manager.options.CommandAndArgs
	if shouldUseShell(argv) {
		shellPath := pickUserShell()
		commandLine := shellJoin(argv)
		manager.command = exec.Command(shellPath, "-lc", commandLine)
	} else {
		manager.command = exec.Command(argv[0], argv[1:]...)
	}
	manager.command.Stdout = os.Stdout
	manager.command.Stderr = os.Stderr
	manager.command.Stdin = os.Stdin
	manager.command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	manager.childExit = make(chan int, 1)
	return nil
}

func (manager *executionManager) preparePlatform() {
	if runtime.GOOS == linuxOSName {
		manager.initializeCgroup()
	}
	if manager.macTracking {
		manager.initializeMacTracker()
	}
}

func (manager *executionManager) initializeCgroup() {
	available, _ := cgroup.Available()
	if !available {
		return
	}
	pathValue, createErr := cgroup.CreateUnique()
	if createErr != nil {
		manager.logger.Error(logMessageCgroupCreateFailed, zap.Error(createErr))
		return
	}
	manager.cgroupPath = pathValue
	manager.usingCgroup = true
}

func (manager *executionManager) initializeMacTracker() {
	if manager.trackerFactory == nil {
		return
	}
	tracker, trackErr := manager.trackerFactory.NewTracker()
	if trackErr != nil {
		manager.logger.Error(logMessageTrackerUnavailable, zap.Error(trackErr))
		return
	}
	manager.macTracker = tracker
}

func (manager *executionManager) configureCgroupMembership(childPid int) {
	if !manager.usingCgroup {
		return
	}
	if addErr := cgroup.AddPid(manager.cgroupPath, childPid); addErr != nil {
		manager.logger.Error(logMessageCgroupJoinFailed, zap.Error(addErr))
		manager.usingCgroup = false
	}
}

func (manager *executionManager) configureMacTracker(childPid int) {
	if manager.macTracker == nil {
		return
	}
	if err := manager.macTracker.StartTrackingRoot(childPid); err != nil {
		if proctrack.IsTrackingUnsupported(err) {
			manager.shutdownMacTracker()
			return
		}
		manager.logger.Error(logMessageTrackerStartFailed, zap.Error(err))
		manager.shutdownMacTracker()
	}
}

func (manager *executionManager) launchBackgroundRoutines(childPid int) {
	manager.signalRoutine = newGoroutineCoordinator()
	go signals.ForwardLoop(signals.ForwardConfig{
		ChildPid:    childPid,
		UsingCgroup: manager.usingCgroup,
		CgroupPath:  manager.cgroupPath,
		Stop:        manager.signalRoutine.stop,
		Done:        manager.signalRoutine.done,
	})

	if runtime.GOOS == linuxOSName && !manager.usingCgroup {
		routine := newGoroutineCoordinator()
		manager.scannerRoutine = &routine
		go procscan.ScannerLoop(procscan.ScannerConfig{
			RootPid:    childPid,
			PollPeriod: scannerPollPeriod,
			Stop:       routine.stop,
			Done:       routine.done,
		})
	}
}

func (manager *executionManager) stopBackgroundRoutines() {
	manager.stopRoutine(&manager.signalRoutine)
	if manager.scannerRoutine != nil {
		manager.stopRoutine(manager.scannerRoutine)
		manager.scannerRoutine = nil
	}
}

func (manager *executionManager) stopRoutine(routine *goroutineCoordinator) {
	if routine == nil || routine.stop == nil || routine.done == nil {
		return
	}
	routine.stopWithin(manager.timerFactory, goroutineShutdownGrace)
	routine.stop = nil
	routine.done = nil
}

func (manager *executionManager) captureExitStatus() {
	waitErr := manager.command.Wait()
	exitCode := exitcodes.ExitRuntimeError
	if waitErr == nil {
		exitCode = exitcodes.ExitSuccess
	} else if exitError, ok := waitErr.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}
	manager.childExit <- exitCode
}

func (manager *executionManager) enforceTimeout(childPid int) int {
	enforcementDeadline := manager.now().Add(manager.options.EnforceWindow)

	manager.sendTerminationSignal(childPid)
	manager.pauseForGrace(enforcementDeadline)
	manager.sendKillSignal(childPid)
	manager.stopBackgroundRoutines()

	remaining := time.Until(enforcementDeadline)
	if remaining > 0 {
		timer := manager.timerFactory.NewTimer(remaining)
		defer timer.Stop()

		select {
		case <-manager.childExit:
		case <-timer.C:
		}
	}
	return exitcodes.ExitTimeout
}

func (manager *executionManager) pauseForGrace(deadline time.Time) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return
	}
	if remaining < enforcementPauseDuration {
		manager.sleep(remaining)
		return
	}
	manager.sleep(enforcementPauseDuration)
}

func (manager *executionManager) sendTerminationSignal(childPid int) {
	if manager.usingCgroup {
		_ = cgroup.SignalAll(manager.cgroupPath, unix.SIGTERM)
		return
	}
	if runtime.GOOS == darwinOSName && manager.macTracker != nil {
		_ = manager.macTracker.SignalAll(syscall.SIGTERM)
		return
	}
	_ = syscall.Kill(-childPid, syscall.SIGTERM)
}

func (manager *executionManager) sendKillSignal(childPid int) {
	if manager.usingCgroup {
		if cgroup.HasKillFile(manager.cgroupPath) {
			_ = cgroup.Kill(manager.cgroupPath)
			return
		}
		_ = cgroup.SignalAll(manager.cgroupPath, unix.SIGKILL)
		return
	}
	if runtime.GOOS == darwinOSName && manager.macTracker != nil {
		_ = manager.macTracker.SignalAll(syscall.SIGKILL)
		manager.killMacDescendants(manager.macTracker.RootPid())
		return
	}
	_ = syscall.Kill(-childPid, syscall.SIGKILL)
	if runtime.GOOS == linuxOSName {
		manager.killLinuxDescendants(childPid)
		return
	}
	if runtime.GOOS == darwinOSName {
		manager.killMacDescendants(childPid)
	}
}

func (manager *executionManager) killLinuxDescendants(rootPid int) {
	descendants, err := procscan.FindDescendants(rootPid)
	if err != nil {
		return
	}
	for _, pid := range descendants {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func (manager *executionManager) killMacDescendants(rootPid int) {
	descendants, err := proctrack.FallbackDescendants(rootPid)
	if err != nil {
		return
	}
	for _, pid := range descendants {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func (manager *executionManager) cleanupPlatform() {
	manager.shutdownMacTracker()
	if manager.usingCgroup && manager.cgroupPath != "" {
		_ = cgroup.Remove(manager.cgroupPath)
	}
}

func (manager *executionManager) terminateChildImmediately(childPid int) {
	manager.sendKillSignal(childPid)
	manager.stopBackgroundRoutines()
}

func (manager *executionManager) shutdownMacTracker() {
	if manager.macTracker == nil {
		return
	}

	closeDone := make(chan struct{})
	go func(tracker proctrack.Tracker) {
		_ = tracker.Close()
		close(closeDone)
	}(manager.macTracker)

	timer := manager.timerFactory.NewTimer(goroutineShutdownGrace)
	defer timer.Stop()

	select {
	case <-closeDone:
	case <-timer.C:
		manager.logger.Error(logMessageTrackerCloseDelayed)
	}
	manager.macTracker = nil
}

type goroutineCoordinator struct {
	stop chan struct{}
	done chan struct{}
}

func newGoroutineCoordinator() goroutineCoordinator {
	return goroutineCoordinator{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (routine goroutineCoordinator) stopWithin(factory timerBuilder, limit time.Duration) {
	if routine.stop == nil || routine.done == nil {
		return
	}
	select {
	case <-routine.done:
		return
	default:
	}
	close(routine.stop)
	timer := factory.NewTimer(limit)
	defer timer.Stop()
	select {
	case <-routine.done:
	case <-timer.C:
	}
}
