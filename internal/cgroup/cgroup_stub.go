//go:build !linux

package cgroup

import "os"

func Available() (bool, string)                  { return false, "not supported on this OS" }
func CreateUnique() (string, error)              { return "", nil }
func Remove(path string) error                   { return nil }
func AddPid(path string, pid int) error          { return nil }
func SignalAll(path string, sig os.Signal) error { return nil }
func HasKillFile(path string) bool               { return false }
func Kill(path string) error                     { return nil }
