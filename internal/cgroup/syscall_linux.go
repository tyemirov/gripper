//go:build linux

package cgroup

import (
	"os"
	"syscall"
)

func syscallKill(pid int, sig os.Signal) error {
	if signalValue, ok := sig.(syscall.Signal); ok {
		return syscall.Kill(pid, signalValue)
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
