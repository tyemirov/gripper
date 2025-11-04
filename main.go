// Package main provides the gripper CLI entry point.
package main

import (
	"os"

	"github.com/tyemirov/gripper/cmd"
	"github.com/tyemirov/gripper/internal/util/exitcodes"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if exitErr, ok := err.(*exitcodes.ExitError); ok {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
