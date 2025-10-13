//go:build darwin

// Package procscan on macOS is only used as a last-chance backstop when the
// kqueue tracker is not available (or for final sweeps). It lists descendants
// via `ps` (best-effort).
package procscan

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ScannerConfig struct {
	RootPid    int
	PollPeriod time.Duration
	Stop       <-chan struct{}
	Done       chan<- struct{}
}

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

func FindDescendants(rootPid int) ([]int, error) {
	command := exec.Command("ps", "-axo", "pid,ppid")
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}

	pidToParent := make(map[int]int, 512)
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		pidValue, pidErr := strconv.Atoi(fields[0])
		parentValue, parentErr := strconv.Atoi(fields[1])
		if pidErr != nil || parentErr != nil {
			continue
		}
		pidToParent[pidValue] = parentValue
	}
	_ = command.Wait()

	queue := []int{rootPid}
	visited := map[int]bool{rootPid: true}
	descendants := make([]int, 0, 512)
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
