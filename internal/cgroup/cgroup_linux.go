//go:build linux

// Package cgroup provides minimal cgroup v2 helpers for Linux.
//
// When available, gripper creates a unique cgroup, adds the launched pid,
// and on timeout either writes to cgroup.kill (preferred) or signals all pids
// listed in cgroup.procs.
package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// basePath is the standard cgroup v2 mount.
const basePath = "/sys/fs/cgroup"

// Available reports whether cgroup v2 controllers file is present.
func Available() (bool, string) {
	controllersPath := filepath.Join(basePath, "cgroup.controllers")
	_, statErr := os.Stat(controllersPath)
	if statErr != nil {
		return false, "cgroup.controllers not found (cgroup v2 likely unavailable)"
	}
	return true, "cgroup v2 present"
}

// CreateUnique creates a uniquely named leaf cgroup and returns its path.
func CreateUnique() (string, error) {
	currentTimestampNanos := time.Now().UnixNano()
	directoryName := fmt.Sprintf("gripper-%d-%d", os.Getpid(), currentTimestampNanos)
	fullPath := filepath.Join(basePath, directoryName)
	if createDirectoryErr := os.Mkdir(fullPath, 0o755); createDirectoryErr != nil {
		return "", createDirectoryErr
	}
	return fullPath, nil
}

// Remove attempts to move remaining pids to the parent and remove the cgroup.
func Remove(cgroupPath string) error {
	parentProcsPath := filepath.Join(basePath, "cgroup.procs")
	ourProcsPath := filepath.Join(cgroupPath, "cgroup.procs")

	if fileData, readFileErr := os.ReadFile(ourProcsPath); readFileErr == nil {
		for _, processIDText := range splitFieldsIntoTokens(string(fileData)) {
			_ = os.WriteFile(parentProcsPath, []byte(processIDText+"\n"), 0o644)
		}
	}
	_ = os.Remove(cgroupPath)
	return nil
}

// AddPid writes a pid into cgroup.procs.
func AddPid(cgroupPath string, processID int) error {
	targetFilePath := filepath.Join(cgroupPath, "cgroup.procs")
	lineToWrite := strconv.Itoa(processID) + "\n"
	return os.WriteFile(targetFilePath, []byte(lineToWrite), 0o644)
}

// SignalAll sends a signal to all pids listed in cgroup.procs.
func SignalAll(cgroupPath string, signalToSend os.Signal) error {
	targetFilePath := filepath.Join(cgroupPath, "cgroup.procs")
	fileData, readFileErr := os.ReadFile(targetFilePath)
	if readFileErr != nil {
		return readFileErr
	}
	for _, processIDText := range splitFieldsIntoTokens(string(fileData)) {
		if processIDValue, convertErr := strconv.Atoi(processIDText); convertErr == nil {
			_ = syscallKill(processIDValue, signalToSend)
		}
	}
	return nil
}

// HasKillFile reports whether cgroup.kill exists.
func HasKillFile(cgroupPath string) bool {
	killFilePath := filepath.Join(cgroupPath, "cgroup.kill")
	_, statErr := os.Stat(killFilePath)
	return statErr == nil
}

// Kill writes "1\n" to cgroup.kill to kill all processes atomically.
func Kill(cgroupPath string) error {
	killFilePath := filepath.Join(cgroupPath, "cgroup.kill")
	return os.WriteFile(killFilePath, []byte("1\n"), 0o644)
}

// splitFieldsIntoTokens splits a whitespace-separated string into individual tokens.
// It trims spaces, tabs, and newlines automatically using the Go standard library.
//
// Example:
//
//	rawInput: "123\n456  789"
//	output:   []string{"123", "456", "789"}
func splitFieldsIntoTokens(rawInput string) []string {
	return strings.Fields(rawInput)
}
