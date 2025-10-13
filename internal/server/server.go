package server

import (
	"context"
	"errors"
	"time"

	"github.com/temirov/gripper/internal/runner"
	"github.com/temirov/gripper/internal/util/exitcodes"
)

const (
	errTimeoutNotPositive = "timeoutSeconds must be > 0"
	errCommandMissing     = "missing command after --"
)

// Service executes commands using the runner package while applying additional validation.
type Service struct {
	commandExecutor runner.Executor
}

// NewService constructs a Service with the default executor.
func NewService() Service {
	return Service{commandExecutor: runner.NewExecutor()}
}

// Run launches the command defined by the server tail under the provided timeout.
func (service Service) Run(timeoutSeconds int, serverArgs []string) (int, error) {
	if timeoutSeconds <= 0 {
		return exitcodes.ExitInvalidUsage, errors.New(errTimeoutNotPositive)
	}
	if len(serverArgs) == 0 {
		return exitcodes.ExitInvalidUsage, errors.New(errCommandMissing)
	}

	options := runner.Options{
		Timeout:        time.Duration(timeoutSeconds) * time.Second,
		CommandAndArgs: serverArgs,
	}

	return service.commandExecutor.Execute(context.Background(), options)
}

// RunServerPart delegates to the default Service instance for backwards compatibility.
func RunServerPart(timeoutSeconds int, serverArgs []string) (int, error) {
	return NewService().Run(timeoutSeconds, serverArgs)
}
