// Package signals forwards incoming OS signals to the managed job.
//
// On Linux with cgroup v2, signals are broadcast to all pids in cgroup.procs.
// Otherwise, signals are sent to the child's process group (-pid).
package signals

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/temirov/gripper/internal/cgroup"
)

type ForwardConfig struct {
	ChildPid    int
	UsingCgroup bool
	CgroupPath  string
	Stop        <-chan struct{}
	Done        chan<- struct{}
}

func ForwardLoop(config ForwardConfig) {
	defer close(config.Done)

	signalsChannel := make(chan os.Signal, 8)
	signal.Notify(signalsChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	defer signal.Stop(signalsChannel)

	for {
		select {
		case <-config.Stop:
			return
		case received := <-signalsChannel:
			forward(received, config)
		}
	}
}

func forward(sig os.Signal, cfg ForwardConfig) {
	if cfg.UsingCgroup && cfg.CgroupPath != "" {
		_ = cgroup.SignalAll(cfg.CgroupPath, sig)
		return
	}
	if signalValue, ok := sig.(syscall.Signal); ok {
		_ = syscall.Kill(-cfg.ChildPid, signalValue)
	} else {
		_ = syscall.Kill(-cfg.ChildPid, syscall.SIGTERM)
	}
}
