//go:build linux

// Package procscan implements a lightweight descendant sweeper for Linux when
// cgroup v2 is not in use. It periodically walks /proc to find descendants of
// the root pid and sends SIGTERM as a nudge (hard kill still happens on timeout).
package procscan

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ScannerConfig configures the Linux non-cgroup scanner loop.
type ScannerConfig struct {
	RootPid    int
	PollPeriod time.Duration
	Stop       <-chan struct{}
	Done       chan<- struct{}
}

// ScannerLoop runs until Stop is closed. It sends SIGTERM to discovered descendants
// to encourage early shutdown before the hard kill path executes.
func ScannerLoop(config ScannerConfig) {
	defer close(config.Done)
	ticker := time.NewTicker(config.PollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-config.Stop:
			return
		case <-ticker.C:
			descendants, err := FindDescendants(config.RootPid)
			if err != nil {
				continue
			}
			for _, descendantPid := range descendants {
				_ = syscall.Kill(descendantPid, syscall.SIGTERM)
			}
		}
	}
}

// FindDescendants returns a best-effort list of descendant pids of rootPid using /proc.
func FindDescendants(rootPid int) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	pidToParent := make(map[int]int, 1024)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pidValue, convErr := strconv.Atoi(entry.Name())
		if convErr != nil {
			continue
		}
		statPath := filepath.Join("/proc", entry.Name(), "stat")
		data, readErr := os.ReadFile(statPath)
		if readErr != nil {
			continue
		}
		text := string(data)
		closeParenIndex := strings.LastIndex(text, ")")
		if closeParenIndex == -1 {
			continue
		}
		rest := strings.Fields(text[closeParenIndex+1:])
		if len(rest) < 2 {
			continue
		}
		parentValue, parentErr := strconv.Atoi(rest[0+1]) // ppid field
		if parentErr != nil {
			continue
		}
		pidToParent[pidValue] = parentValue
	}

	queue := []int{rootPid}
	visited := map[int]bool{rootPid: true}
	descendants := make([]int, 0, 1024)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for pidNum, parentNum := range pidToParent {
			if parentNum == current && !visited[pidNum] {
				visited[pidNum] = true
				descendants = append(descendants, pidNum)
				queue = append(queue, pidNum)
			}
		}
	}
	return descendants, nil
}
