// Package cmd defines the Cobra command for gripper.
//
// Usage:
//
//	gripper <seconds> -- <command> [args...]
//
// The CLI part parses <seconds> and hands everything after `--` to the server part.
// The server part enforces the timeout and kills the entire process tree within 1s.
package cmd

import (
	"errors"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/temirov/gripper/internal/server"
	"github.com/temirov/gripper/internal/util/exitcodes"
)

var rootCommand = &cobra.Command{
	Use:   "gripper <seconds> -- <command> [args...]",
	Short: "Run a command and kill it (and all descendants) after <seconds>",
	Long: `gripper has two halves: a CLI that parses <seconds>, and a "server" part
that receives the entire tail after -- and enforces a hard timeout.
After the exact timeout is reached, ALL descendants are guaranteed terminated
within a fixed 1s enforcement window (not configurable).`,
	Example: `
  # Kill "npm test" and all its children after 20 seconds
  gripper 20 -- npm test

  # Kill a long-running server after 60 seconds
  gripper 60 -- ./server --port 8080

  # Running from source (note the go-run separator before your args):
  go run ./... -- 10 -- ls -la
`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Cobra removes the literal "--", but exposes its position via ArgsLenAtDash().
		// We treat everything BEFORE the dash as CLI part, and everything AT+AFTER as server tail.
		dashIndex := cmd.ArgsLenAtDash()
		if dashIndex < 0 {
			return usageErr(`missing separator "--" between <seconds> and command`)
		}
		if len(args) == 0 || dashIndex == 0 {
			return usageErr("seconds must precede \"--\"")
		}
		if dashIndex >= len(args) {
			return usageErr("missing command after --")
		}

		secondsText := args[0]
		timeoutSeconds, convErr := strconv.Atoi(secondsText)
		if convErr != nil || timeoutSeconds <= 0 {
			return usageErr("seconds must be a positive integer")
		}

		serverTail := args[dashIndex:] // everything after the dash position
		if len(serverTail) == 0 {
			return usageErr("missing command after --")
		}

		exitCode, runErr := server.RunServerPart(timeoutSeconds, serverTail)
		if runErr != nil && exitCode == 0 {
			return runErr
		}
		if exitCode != exitcodes.ExitSuccess {
			return &exitError{Code: exitCode}
		}
		return nil
	},
}

// Execute runs the CLI.
func Execute() error {
	// Compact help output emphasizing the universal `--` separator.
	rootCommand.SetHelpTemplate(strings.TrimLeft(`
{{with or .Long .Short}}{{. | trimTrailingWhitespaces}}{{end}}

USAGE
  {{.Use}}

EXAMPLES{{.Example}}

`, "\n"))
	return rootCommand.Execute()
}

// exitError lets Cobra propagate a specific non-zero code without printing usage.
type exitError struct{ Code int }

func (e *exitError) Error() string { return "exit requested" }

// usageErr formats concise usage errors.
func usageErr(msg string) error {
	return errors.New("usage: " + msg + "\nTry: gripper <seconds> -- <command> [args...]")
}
