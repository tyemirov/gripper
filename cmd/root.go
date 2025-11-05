package cmd

import (
	"errors"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tyemirov/gripper/internal/server"
	"github.com/tyemirov/gripper/internal/util/exitcodes"
)

const (
	cliUseLine            = "gripper <seconds> -- <command> [args...]"
	cliShortDescription   = "Run a command and kill it (and all descendants) after <seconds>"
	cliLongDescription    = "gripper parses <seconds>, forwards everything after -- to the server component, and enforces a 1s kill window after timeout."
	cliExampleBlock       = "  # Kill \"npm test\" and all its children after 20 seconds\n  gripper 20 -- npm test\n\n  # Kill a long-running server after 60 seconds\n  gripper 60 -- ./server --port 8080\n\n  # Running from source (note the go-run separator before your args):\n  go run ./... -- 10 -- ls -la\n"
	helpTemplate          = "{{with or .Long .Short}}{{. | trimTrailingWhitespaces}}{{end}}\n\nUSAGE\n  {{.Use}}\n\nEXAMPLES{{.Example}}\n\n"
	errMissingSeparator   = "missing separator \"--\" between <seconds> and command"
	errSecondsBeforeDash  = "seconds must precede \"--\""
	errMissingCommand     = "missing command after --"
	errSecondsNotPositive = "seconds must be a positive integer"
)

// CLI wires the Cobra command to the server service.
type CLI struct {
	service server.Service
}

// NewCLI constructs a CLI that delegates execution to the provided service.
func NewCLI(service server.Service) CLI {
	return CLI{service: service}
}

// Execute builds and runs the root Cobra command.
func Execute() error {
	cli := NewCLI(server.NewService())
	command := cli.buildRootCommand()
	return command.Execute()
}

func (cli CLI) buildRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:     cliUseLine,
		Short:   cliShortDescription,
		Long:    cliLongDescription,
		Example: cliExampleBlock,
		Args:    cobra.ArbitraryArgs,
	}

	command.RunE = cli.run
	command.SetHelpTemplate(strings.TrimLeft(helpTemplate, "\n"))
	return command
}

func (cli CLI) run(command *cobra.Command, args []string) error {
	dashIndex := command.ArgsLenAtDash()
	if dashIndex < 0 {
		return cli.usageError(errMissingSeparator)
	}
	if dashIndex != 1 {
		return cli.usageError(errSecondsBeforeDash)
	}
	if len(args) <= 1 {
		return cli.usageError(errMissingCommand)
	}

	timeoutValue, parseErr := strconv.Atoi(args[0])
	if parseErr != nil || timeoutValue <= 0 {
		return cli.usageError(errSecondsNotPositive)
	}

	serverTail := append([]string(nil), args[1:]...)
	exitCode, runErr := cli.service.Run(timeoutValue, serverTail)
	if runErr != nil && exitCode == 0 {
		return runErr
	}
	if exitCode != exitcodes.ExitSuccess {
		return &exitcodes.ExitError{Code: exitCode}
	}
	return nil
}

func (CLI) usageError(message string) error {
	return errors.New("usage: " + message + "\nTry: " + cliUseLine)
}
